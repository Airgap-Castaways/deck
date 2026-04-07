package prepare

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/Airgap-Castaways/deck/internal/stepspec"
)

func TestParseImageRegistryAuth(t *testing.T) {
	auth, err := parseImageRegistryAuth(stepspec.DownloadImage{
		Auth: []stepspec.ImageAuth{{
			Registry: "registry.example.com",
			Basic: stepspec.ImageAuthBasic{
				Username: "robot",
				Password: "secret",
			},
		}},
	})
	if err != nil {
		t.Fatalf("parseImageRegistryAuth failed: %v", err)
	}
	entry, ok := auth["registry.example.com"]
	if !ok {
		t.Fatalf("expected registry entry, got %v", auth)
	}
	if entry.username != "robot" || entry.password != "secret" {
		t.Fatalf("unexpected auth entry: %+v", entry)
	}
}

func TestParseImageRegistryAuthRejectsDuplicateRegistry(t *testing.T) {
	_, err := parseImageRegistryAuth(stepspec.DownloadImage{
		Auth: []stepspec.ImageAuth{
			{Registry: "registry.example.com", Basic: stepspec.ImageAuthBasic{Username: "a", Password: "b"}},
			{Registry: "registry.example.com", Basic: stepspec.ImageAuthBasic{Username: "c", Password: "d"}},
		},
	})
	if err == nil {
		t.Fatalf("expected duplicate registry error")
	}
	if want := "duplicate registry entries"; err != nil && !strings.Contains(err.Error(), want) {
		t.Fatalf("expected %q in error, got %v", want, err)
	}
}

func TestImageAuthKeychainUsesExplicitRegistryCredentials(t *testing.T) {
	ref, err := name.ParseReference("registry.example.com/team/app:1.0")
	if err != nil {
		t.Fatalf("parse reference: %v", err)
	}
	keychain := imageRegistryAuthMap{
		"registry.example.com": {registry: "registry.example.com", username: "robot", password: "secret"},
	}.keychain()
	auth, err := keychain.Resolve(ref.Context())
	if err != nil {
		t.Fatalf("resolve auth: %v", err)
	}
	config, err := auth.Authorization()
	if err != nil {
		t.Fatalf("authorization: %v", err)
	}
	if config.Username != "robot" || config.Password != "secret" {
		t.Fatalf("unexpected auth config: %+v", config)
	}
}

func TestImageAuthKeychainFallsBackWhenRegistryNotConfigured(t *testing.T) {
	ref, err := name.ParseReference("registry.k8s.io/pause:3.9")
	if err != nil {
		t.Fatalf("parse reference: %v", err)
	}
	keychain := imageRegistryAuthMap{
		"registry.example.com": {registry: "registry.example.com", username: "robot", password: "secret"},
	}.keychain()
	auth, err := keychain.Resolve(ref.Context())
	if err != nil {
		t.Fatalf("resolve auth: %v", err)
	}
	config, err := auth.Authorization()
	if err != nil {
		t.Fatalf("authorization: %v", err)
	}
	if config.Username == "robot" {
		t.Fatalf("expected fallback auth, got explicit credentials: %+v", config)
	}
}

func TestLoadImageArtifactMetaTreatsUnreadableFileAsCacheMiss(t *testing.T) {
	bundle := t.TempDir()
	dir := "images"
	path := imageMetaFileAbs(bundle, dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir image meta dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"images":["registry.k8s.io/pause:3.9"],"files":["images/pause.tar"]}`), 0o000); err != nil {
		t.Fatalf("write image meta: %v", err)
	}

	_, ok, err := loadImageArtifactMeta(bundle, dir)
	if err != nil {
		t.Fatalf("expected unreadable metadata to be treated as cache miss, got %v", err)
	}
	if ok {
		t.Fatal("expected unreadable metadata cache miss")
	}
}

func TestLoadImageArtifactMetaTreatsMalformedJSONAsCacheMiss(t *testing.T) {
	bundle := t.TempDir()
	dir := "images"
	path := imageMetaFileAbs(bundle, dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir image meta dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{not-json`), 0o644); err != nil {
		t.Fatalf("write image meta: %v", err)
	}

	_, ok, err := loadImageArtifactMeta(bundle, dir)
	if err != nil {
		t.Fatalf("expected malformed metadata to be treated as cache miss, got %v", err)
	}
	if ok {
		t.Fatal("expected malformed metadata cache miss")
	}
}
