package preparecli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/buildinfo"
)

func TestRunStagesLocalRuntimeBinariesFromDirectory(t *testing.T) {
	root := prepareWorkspaceForRuntimeTests(t)
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "deck-linux-amd64"), []byte("linux-amd64"), 0o755); err != nil {
		t.Fatalf("write amd64 binary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "deck-linux-arm64"), []byte("linux-arm64"), 0o755); err != nil {
		t.Fatalf("write arm64 binary: %v", err)
	}
	var stdout bytes.Buffer

	if err := Run(context.Background(), Options{
		PreparedRoot: filepath.Join(root, "outputs"),
		BinarySource: binarySourceLocal,
		BinaryDir:    binDir,
		Binaries:     []string{"linux/amd64", "linux/arm64"},
		Stdout:       &stdout,
	}); err != nil {
		t.Fatalf("prepare run failed: %v", err)
	}
	for rel, want := range map[string]string{
		filepath.Join("outputs", "bin", "linux", "amd64", "deck"): "linux-amd64",
		filepath.Join("outputs", "bin", "linux", "arm64", "deck"): "linux-arm64",
	} {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if string(raw) != want {
			t.Fatalf("unexpected %s content: %q", rel, string(raw))
		}
	}
	launcherRaw, err := os.ReadFile(filepath.Join(root, "deck"))
	if err != nil {
		t.Fatalf("read launcher: %v", err)
	}
	if !strings.Contains(string(launcherRaw), "outputs/bin/") {
		t.Fatalf("expected launcher to reference outputs/bin, got %q", string(launcherRaw))
	}
}

func TestRunStagesReleaseRuntimeBinariesWithDefaultTargets(t *testing.T) {
	root := prepareWorkspaceForRuntimeTests(t)
	var got []string
	fetcher := func(_ context.Context, version string, target runtimeBinaryTarget) ([]byte, error) {
		got = append(got, version+":"+target.OS+"/"+target.Arch)
		return []byte(target.OS + "-" + target.Arch), nil
	}
	if err := Run(context.Background(), Options{
		PreparedRoot: filepath.Join(root, "outputs"),
		BinarySource: binarySourceRelease,
		BinaryVer:    "v1.2.3",
		runtimeBinaryDeps: runtimeBinaryDeps{
			currentGOOS:   func() string { return runtime.GOOS },
			currentGOARCH: func() string { return runtime.GOARCH },
			readFile:      os.ReadFile,
			osExecutable:  os.Executable,
			fetchRelease:  fetcher,
		},
	}); err != nil {
		t.Fatalf("prepare run failed: %v", err)
	}
	want := []string{"v1.2.3:linux/amd64", "v1.2.3:linux/arm64", "v1.2.3:darwin/amd64", "v1.2.3:darwin/arm64"}
	if len(got) != len(want) {
		t.Fatalf("unexpected fetched target count: got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected fetched targets: got %#v want %#v", got, want)
		}
	}
	for _, rel := range []string{
		filepath.Join("outputs", "bin", "linux", "amd64", "deck"),
		filepath.Join("outputs", "bin", "linux", "arm64", "deck"),
		filepath.Join("outputs", "bin", "darwin", "amd64", "deck"),
		filepath.Join("outputs", "bin", "darwin", "arm64", "deck"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("expected runtime binary %s: %v", rel, err)
		}
	}
}

func TestRunStagesLocalRuntimeBinariesWithDefaultTargetsFromDirectory(t *testing.T) {
	root := prepareWorkspaceForRuntimeTests(t)
	binDir := t.TempDir()
	for name, body := range map[string]string{
		"deck-linux-amd64":  "linux-amd64",
		"deck-linux-arm64":  "linux-arm64",
		"deck-darwin-amd64": "darwin-amd64",
		"deck-darwin-arm64": "darwin-arm64",
	} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(body), 0o755); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	if err := Run(context.Background(), Options{
		PreparedRoot: filepath.Join(root, "outputs"),
		BinarySource: binarySourceLocal,
		BinaryDir:    binDir,
	}); err != nil {
		t.Fatalf("prepare run failed: %v", err)
	}

	for rel, want := range map[string]string{
		filepath.Join("outputs", "bin", "linux", "amd64", "deck"):  "linux-amd64",
		filepath.Join("outputs", "bin", "linux", "arm64", "deck"):  "linux-arm64",
		filepath.Join("outputs", "bin", "darwin", "amd64", "deck"): "darwin-amd64",
		filepath.Join("outputs", "bin", "darwin", "arm64", "deck"): "darwin-arm64",
	} {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if string(raw) != want {
			t.Fatalf("unexpected %s content: %q", rel, string(raw))
		}
	}
}

func TestRunLocalSourceWithoutDirRejectsImplicitAllTargets(t *testing.T) {
	root := prepareWorkspaceForRuntimeTests(t)
	err := Run(context.Background(), Options{
		PreparedRoot: filepath.Join(root, "outputs"),
		BinarySource: binarySourceLocal,
		runtimeBinaryDeps: runtimeBinaryDeps{
			currentGOOS:   func() string { return "darwin" },
			currentGOARCH: func() string { return "arm64" },
			readFile:      os.ReadFile,
			osExecutable:  os.Executable,
			fetchRelease:  fetchReleaseRuntimeBinary,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "default runtime bundle includes all supported platforms") {
		t.Fatalf("expected implicit all-targets error, got %v", err)
	}
}

func TestRunLocalSourceWithoutDirRejectsForeignTarget(t *testing.T) {
	root := prepareWorkspaceForRuntimeTests(t)
	err := Run(context.Background(), Options{
		PreparedRoot: filepath.Join(root, "outputs"),
		BinarySource: binarySourceLocal,
		Binaries:     []string{"linux/amd64"},
		runtimeBinaryDeps: runtimeBinaryDeps{
			currentGOOS:   func() string { return "darwin" },
			currentGOARCH: func() string { return "arm64" },
			readFile:      os.ReadFile,
			osExecutable:  os.Executable,
			fetchRelease:  fetchReleaseRuntimeBinary,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "only supports the current host target darwin/arm64") {
		t.Fatalf("expected foreign target error, got %v", err)
	}
}

func TestResolveBinaryTargetsAppliesExcludes(t *testing.T) {
	targets, err := resolveBinaryTargets(Options{BinaryExcludes: []string{"darwin/amd64", "darwin/arm64"}}, binarySourceRelease, defaultRuntimeBinaryDeps())
	if err != nil {
		t.Fatalf("resolve binary targets: %v", err)
	}
	got := make([]string, 0, len(targets))
	for _, target := range targets {
		got = append(got, target.OS+"/"+target.Arch)
	}
	want := []string{"linux/amd64", "linux/arm64"}
	if len(got) != len(want) {
		t.Fatalf("unexpected targets: got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected targets: got %#v want %#v", got, want)
		}
	}
}

func TestResolveBinaryTargetsRejectsEmptyAfterExclude(t *testing.T) {
	_, err := resolveBinaryTargets(Options{Binaries: []string{"linux/amd64"}, BinaryExcludes: []string{"linux/amd64"}}, binarySourceRelease, defaultRuntimeBinaryDeps())
	if err == nil || !strings.Contains(err.Error(), "no runtime binaries selected") {
		t.Fatalf("expected empty-target error, got %v", err)
	}
}

func TestResolveBinarySourceAutoUsesReleaseForImplicitAllTargetsOnDev(t *testing.T) {
	oldVersion := buildinfo.Version
	buildinfo.Version = "dev"
	t.Cleanup(func() { buildinfo.Version = oldVersion })

	source, err := resolveBinarySource(Options{}, defaultRuntimeBinaryDeps())
	if err != nil {
		t.Fatalf("resolve binary source: %v", err)
	}
	if source != binarySourceRelease {
		t.Fatalf("unexpected source: got %q want %q", source, binarySourceRelease)
	}
}

func TestResolveBinarySourceAutoKeepsLocalWhenTargetsAreExplicitOnDev(t *testing.T) {
	oldVersion := buildinfo.Version
	buildinfo.Version = "dev"
	t.Cleanup(func() { buildinfo.Version = oldVersion })

	source, err := resolveBinarySource(Options{Binaries: []string{"linux/amd64"}}, defaultRuntimeBinaryDeps())
	if err != nil {
		t.Fatalf("resolve binary source: %v", err)
	}
	if source != binarySourceLocal {
		t.Fatalf("unexpected source: got %q want %q", source, binarySourceLocal)
	}
}

func TestResolveLocalRuntimeBinaryPathSupportsStructuredLayout(t *testing.T) {
	binDir := t.TempDir()
	path := filepath.Join(binDir, "linux", "arm64", "deck")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir structured bin dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("deck"), 0o755); err != nil {
		t.Fatalf("write structured binary: %v", err)
	}
	resolved, err := resolveLocalRuntimeBinaryPath(binDir, runtimeBinaryTarget{OS: "linux", Arch: "arm64"})
	if err != nil {
		t.Fatalf("resolve local runtime path: %v", err)
	}
	if resolved != path {
		t.Fatalf("unexpected runtime path: got %q want %q", resolved, path)
	}
}

func TestDownloadArchiveDeckBinaryReadsDeckEntry(t *testing.T) {
	archive := buildTestDeckArchive(t, map[string]string{
		"README.md":     "readme",
		"deck":          "deck-binary",
		"docs/extra.md": "ignored",
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer srv.Close()
	raw, err := downloadArchiveDeckBinary(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("download archive deck binary: %v", err)
	}
	if string(raw) != "deck-binary" {
		t.Fatalf("unexpected deck binary contents: %q", string(raw))
	}
}

func TestDownloadArchiveDeckBinaryRejectsMissingDeckEntry(t *testing.T) {
	archive := buildTestDeckArchive(t, map[string]string{"README.md": "readme"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer srv.Close()
	_, err := downloadArchiveDeckBinary(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "does not contain deck") {
		t.Fatalf("expected missing deck error, got %v", err)
	}
}

func prepareWorkspaceForRuntimeTests(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	seedDir := filepath.Join(root, "seed", "files")
	if err := os.MkdirAll(filepath.Join(root, "workflows", "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatalf("mkdir seed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "workflows", "prepare.yaml"), []byte("version: v1alpha1\nphases:\n  - name: prepare\n    steps:\n      - id: seed\n        kind: DownloadFile\n        spec:\n          source:\n            path: files/source.bin\n          fetch:\n            sources:\n              - type: local\n                path: "+filepath.ToSlash(filepath.Join(root, "seed"))+"\n          outputPath: files/source.bin\n"), 0o644); err != nil {
		t.Fatalf("write prepare workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "workflows", "scenarios", "apply.yaml"), []byte("version: v1alpha1\nsteps: []\n"), 0o644); err != nil {
		t.Fatalf("write apply workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "source.bin"), []byte("seed"), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}
	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalCWD) })
	return root
}

func buildTestDeckArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var compressed bytes.Buffer
	gzw := gzip.NewWriter(&compressed)
	tw := tar.NewWriter(gzw)
	for name, body := range files {
		raw := []byte(body)
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(raw))}); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write(raw); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return compressed.Bytes()
}
