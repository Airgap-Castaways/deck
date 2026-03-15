package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSiteReleaseImportList(t *testing.T) {
	root := t.TempDir()
	bundlePath := filepath.Join(t.TempDir(), "site-release.tar")
	writeSiteReleaseBundleTarFixture(t, bundlePath)

	out, err := runWithCapturedStdout([]string{"site", "release", "import", "--root", root, "--id", "release-1", "--bundle", bundlePath})
	if err != nil {
		t.Fatalf("site release import failed: %v", err)
	}
	if !strings.Contains(out, "site release import: ok") {
		t.Fatalf("unexpected import output: %q", out)
	}

	listOut, err := runWithCapturedStdout([]string{"site", "release", "list", "--root", root})
	if err != nil {
		t.Fatalf("site release list failed: %v", err)
	}
	if !strings.Contains(listOut, "release-1") {
		t.Fatalf("expected imported release in list output, got %q", listOut)
	}
}

func TestSiteSessionCreateClose(t *testing.T) {
	root := t.TempDir()
	bundlePath := filepath.Join(t.TempDir(), "site-release.tar")
	writeSiteReleaseBundleTarFixture(t, bundlePath)

	if _, err := runWithCapturedStdout([]string{"site", "release", "import", "--root", root, "--id", "release-1", "--bundle", bundlePath}); err != nil {
		t.Fatalf("site release import failed: %v", err)
	}

	createOut, err := runWithCapturedStdout([]string{"site", "session", "create", "--root", root, "--id", "session-1", "--release", "release-1"})
	if err != nil {
		t.Fatalf("site session create failed: %v", err)
	}
	if !strings.Contains(createOut, "site session create: ok") {
		t.Fatalf("unexpected create output: %q", createOut)
	}

	closeOut, err := runWithCapturedStdout([]string{"site", "session", "close", "--root", root, "--id", "session-1"})
	if err != nil {
		t.Fatalf("site session close failed: %v", err)
	}
	if !strings.Contains(closeOut, "status=closed") {
		t.Fatalf("expected closed status output, got %q", closeOut)
	}
}

func TestSiteAssignRoleNode(t *testing.T) {
	root := t.TempDir()
	bundlePath := filepath.Join(t.TempDir(), "site-release.tar")
	writeSiteReleaseBundleTarFixture(t, bundlePath)

	if _, err := runWithCapturedStdout([]string{"site", "release", "import", "--root", root, "--id", "release-1", "--bundle", bundlePath}); err != nil {
		t.Fatalf("site release import failed: %v", err)
	}
	if _, err := runWithCapturedStdout([]string{"site", "session", "create", "--root", root, "--id", "session-1", "--release", "release-1"}); err != nil {
		t.Fatalf("site session create failed: %v", err)
	}

	roleOut, err := runWithCapturedStdout([]string{"site", "assign", "role", "--root", root, "--session", "session-1", "--assignment", "assign-role", "--role", "apply", "--workflow", "workflows/scenarios/apply.yaml"})
	if err != nil {
		t.Fatalf("site assign role failed: %v", err)
	}
	if !strings.Contains(roleOut, "site assign role: ok") {
		t.Fatalf("unexpected role assignment output: %q", roleOut)
	}

	nodeOut, err := runWithCapturedStdout([]string{"site", "assign", "node", "--root", root, "--session", "session-1", "--assignment", "assign-node", "--node", "node-1", "--role", "apply", "--workflow", "workflows/scenarios/apply.yaml"})
	if err != nil {
		t.Fatalf("site assign node failed: %v", err)
	}
	if !strings.Contains(nodeOut, "site assign node: ok") {
		t.Fatalf("unexpected node assignment output: %q", nodeOut)
	}

	statusOut, err := runWithCapturedStdout([]string{"site", "status", "--root", root})
	if err != nil {
		t.Fatalf("site status failed: %v", err)
	}
	if !strings.Contains(statusOut, "node node-1") || !strings.Contains(statusOut, "apply=not-run") {
		t.Fatalf("unexpected site status output: %q", statusOut)
	}
}

func TestSiteAssignRejectsUnknownSession(t *testing.T) {
	root := t.TempDir()
	bundlePath := filepath.Join(t.TempDir(), "site-release.tar")
	writeSiteReleaseBundleTarFixture(t, bundlePath)

	if _, err := runWithCapturedStdout([]string{"site", "release", "import", "--root", root, "--id", "release-1", "--bundle", bundlePath}); err != nil {
		t.Fatalf("site release import failed: %v", err)
	}

	_, err := runWithCapturedStdout([]string{"site", "assign", "role", "--root", root, "--session", "missing-session", "--assignment", "assign-role", "--role", "apply", "--workflow", "workflows/scenarios/apply.yaml"})
	if err == nil {
		t.Fatalf("expected unknown session error")
	}
	if !strings.Contains(err.Error(), "session \"missing-session\" not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSiteReleaseImportRejectsInvalidBundle(t *testing.T) {
	root := t.TempDir()
	invalidPath := filepath.Join(t.TempDir(), "invalid.tar")
	if err := os.WriteFile(invalidPath, []byte("not-a-tar"), 0o644); err != nil {
		t.Fatalf("write invalid bundle file: %v", err)
	}

	_, err := runWithCapturedStdout([]string{"site", "release", "import", "--root", root, "--id", "release-1", "--bundle", invalidPath})
	if err == nil {
		t.Fatalf("expected invalid bundle rejection")
	}
	if !strings.Contains(err.Error(), "site release import") {
		t.Fatalf("expected site release import error context, got %v", err)
	}
}
