package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func writeRegistryTestImage(t *testing.T, root, ref string) {
	t.Helper()
	tag, err := name.NewTag(ref, name.WeakValidation)
	if err != nil {
		t.Fatalf("name.NewTag %s: %v", ref, err)
	}
	img, err := random.Image(256, 1)
	if err != nil {
		t.Fatalf("random.Image %s: %v", ref, err)
	}
	safe := strings.NewReplacer("/", "_", ":", "_").Replace(ref)
	tarPath := filepath.Join(root, "outputs", "images", safe+".tar")
	if err := tarball.WriteToFile(tarPath, tag, img); err != nil {
		t.Fatalf("tarball.WriteToFile %s: %v", ref, err)
	}
}

func registryStatus(t *testing.T, h http.Handler, path string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr.Code
}

func registryCatalogRepos(t *testing.T, h http.Handler) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v2/_catalog", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("catalog status = %d, want 200", rr.Code)
	}
	var payload struct {
		Repositories []string `json:"repositories"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	return payload.Repositories
}

// Two source images whose repository names differ only by registry domain
// (quay.io/team/app and example.com/team/app) both strip to the alias
// "team/app". The alias is ambiguous and must not silently resolve to an
// arbitrary one; both canonical repositories must still work.
func TestRegistryAmbiguousAliasIsRejected(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "outputs", "images"), 0o755); err != nil {
		t.Fatalf("mkdir images: %v", err)
	}
	writeRegistryTestImage(t, root, "quay.io/team/app:v1")
	writeRegistryTestImage(t, root, "example.com/team/app:v2")

	h, err := NewHandler(root, HandlerOptions{})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	repos := registryCatalogRepos(t, h)
	if !containsString(repos, "quay.io/team/app") {
		t.Fatalf("catalog missing canonical quay.io/team/app: %v", repos)
	}
	if !containsString(repos, "example.com/team/app") {
		t.Fatalf("catalog missing canonical example.com/team/app: %v", repos)
	}
	if containsString(repos, "team/app") {
		t.Fatalf("ambiguous alias team/app must be hidden from catalog: %v", repos)
	}

	if got := registryStatus(t, h, "/v2/team/app/tags/list"); got != http.StatusNotFound {
		t.Fatalf("ambiguous alias tags status = %d, want 404", got)
	}
	if got := registryStatus(t, h, "/v2/team/app/manifests/v1"); got != http.StatusNotFound {
		t.Fatalf("ambiguous alias manifest status = %d, want 404", got)
	}

	if got := registryStatus(t, h, "/v2/quay.io/team/app/manifests/v1"); got != http.StatusOK {
		t.Fatalf("canonical quay.io/team/app manifest status = %d, want 200", got)
	}
	if got := registryStatus(t, h, "/v2/example.com/team/app/manifests/v2"); got != http.StatusOK {
		t.Fatalf("canonical example.com/team/app manifest status = %d, want 200", got)
	}

	// The rejected alias access must be recorded in the audit log with the
	// colliding canonical repositories for operator diagnosis.
	auditRaw, err := os.ReadFile(filepath.Join(root, ".deck", "logs", "server-audit.log"))
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	audit := string(auditRaw)
	if !strings.Contains(audit, auditEventRegistryAliasCollision) {
		t.Fatalf("audit log missing collision event: %s", audit)
	}
	for _, want := range []string{"quay.io/team/app", "example.com/team/app"} {
		if !strings.Contains(audit, want) {
			t.Fatalf("audit log missing colliding repo %q: %s", want, audit)
		}
	}
}

func TestSelectRepoEntries(t *testing.T) {
	entries := []registryCatalogEntry{
		{repo: "quay.io/team/app", canonicalRepo: "quay.io/team/app", tag: "v1", tarPath: "a"},
		{repo: "team/app", canonicalRepo: "quay.io/team/app", tag: "v1", tarPath: "a"},
		{repo: "example.com/team/app", canonicalRepo: "example.com/team/app", tag: "v2", tarPath: "b"},
		{repo: "team/app", canonicalRepo: "example.com/team/app", tag: "v2", tarPath: "b"},
		{repo: "quay.io/solo/app", canonicalRepo: "quay.io/solo/app", tag: "v3", tarPath: "c"},
		{repo: "solo/app", canonicalRepo: "quay.io/solo/app", tag: "v3", tarPath: "c"},
	}

	t.Run("canonical repo served", func(t *testing.T) {
		sel, err := selectRepoEntries(entries, "quay.io/team/app")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sel) != 1 || sel[0].canonicalRepo != "quay.io/team/app" {
			t.Fatalf("unexpected selection: %+v", sel)
		}
	})

	t.Run("ambiguous alias rejected", func(t *testing.T) {
		sel, err := selectRepoEntries(entries, "team/app")
		if sel != nil {
			t.Fatalf("expected no entries for ambiguous alias, got %+v", sel)
		}
		var collErr *registryAliasCollisionError
		if !errors.As(err, &collErr) {
			t.Fatalf("expected *registryAliasCollisionError, got %v", err)
		}
		if collErr.alias != "team/app" {
			t.Fatalf("collision alias = %q, want team/app", collErr.alias)
		}
		if len(collErr.canonicalRepos) != 2 ||
			collErr.canonicalRepos[0] != "example.com/team/app" ||
			collErr.canonicalRepos[1] != "quay.io/team/app" {
			t.Fatalf("collision canonicalRepos = %v, want sorted [example.com/team/app quay.io/team/app]", collErr.canonicalRepos)
		}
	})

	t.Run("unambiguous alias served", func(t *testing.T) {
		sel, err := selectRepoEntries(entries, "solo/app")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sel) != 1 || sel[0].canonicalRepo != "quay.io/solo/app" {
			t.Fatalf("unexpected selection: %+v", sel)
		}
	})

	t.Run("unknown repo returns nothing", func(t *testing.T) {
		sel, err := selectRepoEntries(entries, "does/not-exist")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sel != nil {
			t.Fatalf("expected nil selection, got %+v", sel)
		}
	})

	t.Run("prefer canonical over colliding alias", func(t *testing.T) {
		mixed := []registryCatalogEntry{
			{repo: "foo/bar", canonicalRepo: "foo/bar", tag: "v1", tarPath: "x"},
			{repo: "foo/bar", canonicalRepo: "quay.io/foo/bar", tag: "v9", tarPath: "y"},
		}
		sel, err := selectRepoEntries(mixed, "foo/bar")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sel) != 1 || sel[0].canonicalRepo != "foo/bar" || sel[0].tag != "v1" {
			t.Fatalf("expected only the canonical foo/bar entry, got %+v", sel)
		}
	})
}
