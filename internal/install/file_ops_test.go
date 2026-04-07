package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestEditFileBackup_DefaultEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.conf")
	if err := os.WriteFile(path, []byte("mode=old\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	spec := map[string]any{
		"path":  path,
		"edits": []any{map[string]any{"match": "mode=old", "with": "mode=new"}},
	}
	if err := runEditFile(spec); err != nil {
		t.Fatalf("runEditFile failed: %v", err)
	}

	backups, err := listEditFileBackups(path)
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(backups))
	}

	raw, err := os.ReadFile(filepath.Join(filepath.Dir(path), backups[0]))
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(raw) != "mode=old\n" {
		t.Fatalf("unexpected backup content: %q", string(raw))
	}
}

func TestEditFileBackup_OptOutDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.conf")
	if err := os.WriteFile(path, []byte("mode=old\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	spec := map[string]any{
		"path":   path,
		"backup": false,
		"edits":  []any{map[string]any{"match": "mode=old", "with": "mode=new"}},
	}
	if err := runEditFile(spec); err != nil {
		t.Fatalf("runEditFile failed: %v", err)
	}

	backups, err := listEditFileBackups(path)
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if len(backups) != 0 {
		t.Fatalf("expected no backup files, got %d", len(backups))
	}
}

func TestEditFileBackup_RetentionKeepsLatestTen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.conf")
	if err := os.WriteFile(path, []byte("mode=old\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	oldest := filepath.Base(path) + ".bak-20240101T000000Z"
	oldestPath := filepath.Join(dir, oldest)
	if err := os.WriteFile(oldestPath, []byte("oldest"), 0o644); err != nil {
		t.Fatalf("write oldest backup: %v", err)
	}
	if err := os.Chtimes(oldestPath, time.Unix(1, 0), time.Unix(1, 0)); err != nil {
		t.Fatalf("chtimes oldest: %v", err)
	}

	for i := 1; i < 10; i++ {
		name := fmt.Sprintf("%s.bak-20240101T0000%02dZ", filepath.Base(path), i)
		backupPath := filepath.Join(dir, name)
		if err := os.WriteFile(backupPath, []byte("existing"), 0o644); err != nil {
			t.Fatalf("write existing backup %d: %v", i, err)
		}
		stamp := time.Unix(int64(100+i), 0)
		if err := os.Chtimes(backupPath, stamp, stamp); err != nil {
			t.Fatalf("chtimes existing backup %d: %v", i, err)
		}
	}

	spec := map[string]any{
		"path":  path,
		"edits": []any{map[string]any{"match": "mode=old", "with": "mode=new"}},
	}
	if err := runEditFile(spec); err != nil {
		t.Fatalf("runEditFile failed: %v", err)
	}

	backups, err := listEditFileBackups(path)
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if len(backups) != 10 {
		t.Fatalf("expected 10 backup files, got %d", len(backups))
	}
	if _, err := os.Stat(oldestPath); !os.IsNotExist(err) {
		t.Fatalf("expected oldest backup to be removed, err=%v", err)
	}
}

func TestEditFileBackup_NameFormatAndCollisionSuffix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.conf")
	if err := os.WriteFile(path, []byte("mode=old\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	basePattern := regexp.MustCompile(`^target\.conf\.bak-\d{8}T\d{6}Z$`)
	collisionPattern := regexp.MustCompile(`^target\.conf\.bak-\d{8}T\d{6}Z-[0-9a-f]{8}$`)

	for i := 0; i < 20; i++ {
		first, err := createEditFileBackup(path, []byte("old"))
		if err != nil {
			t.Fatalf("create first backup: %v", err)
		}
		second, err := createEditFileBackup(path, []byte("old"))
		if err != nil {
			t.Fatalf("create second backup: %v", err)
		}

		firstName := filepath.Base(first)
		if !basePattern.MatchString(firstName) {
			t.Fatalf("unexpected first backup name format: %q", firstName)
		}

		secondName := filepath.Base(second)
		if strings.HasPrefix(secondName, firstName+"-") {
			if !collisionPattern.MatchString(secondName) {
				t.Fatalf("unexpected collision backup name format: %q", secondName)
			}
			return
		}

		if err := os.Remove(first); err != nil {
			t.Fatalf("remove first backup retry %d: %v", i, err)
		}
		if err := os.Remove(second); err != nil {
			t.Fatalf("remove second backup retry %d: %v", i, err)
		}
	}

	t.Fatalf("expected at least one same-second collision with suffix")
}

func TestEditFileBackup_CreateFailureIncludesBackupPath(t *testing.T) {
	dir := t.TempDir()
	readOnlyDir := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0o755); err != nil {
		t.Fatalf("mkdir readonly dir: %v", err)
	}
	path := filepath.Join(readOnlyDir, "target.conf")
	if err := os.WriteFile(path, []byte("mode=old\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	if err := os.Chmod(readOnlyDir, 0o555); err != nil {
		t.Fatalf("chmod readonly dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(readOnlyDir, 0o755)
	})

	err := runEditFile(map[string]any{
		"path":  path,
		"edits": []any{map[string]any{"match": "mode=old", "with": "mode=new"}},
	})
	if err == nil {
		t.Fatalf("expected backup creation failure")
	}

	if !strings.Contains(err.Error(), path+".bak-") {
		t.Fatalf("expected error to include backup path prefix %q, got %v", path+".bak-", err)
	}
}

func TestEditFileSupportsAppendOperation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.conf")
	if err := os.WriteFile(path, []byte("alpha\nalpha\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	err := runEditFile(map[string]any{
		"path": path,
		"edits": []any{
			map[string]any{"match": "alpha", "replaceWith": "-beta", "op": "append"},
		},
	})
	if err != nil {
		t.Fatalf("runEditFile failed: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(raw) != "alpha-beta\nalpha-beta\n" {
		t.Fatalf("unexpected edited content: %q", string(raw))
	}
}

func TestFileModeAppliesToCopyAndEdit(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dest := filepath.Join(dir, "dest.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := runCopyFile(context.Background(), "", map[string]any{"source": map[string]any{"path": src}, "path": dest, "mode": "0600"}); err != nil {
		t.Fatalf("runCopyFile failed: %v", err)
	}
	if info, err := os.Stat(dest); err != nil {
		t.Fatalf("stat dest: %v", err)
	} else if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected dest mode 0600, got %o", info.Mode().Perm())
	}

	if err := runEditFile(map[string]any{
		"path":  dest,
		"mode":  "0640",
		"edits": []any{map[string]any{"match": "hello", "with": "deck", "op": "replace"}},
	}); err != nil {
		t.Fatalf("runEditFile failed: %v", err)
	}
	if info, err := os.Stat(dest); err != nil {
		t.Fatalf("stat edited dest: %v", err)
	} else if info.Mode().Perm() != 0o640 {
		t.Fatalf("expected edited dest mode 0640, got %o", info.Mode().Perm())
	}
}

func TestCopyFileReadsBundleSourceFromBundleRoot(t *testing.T) {
	dir := t.TempDir()
	bundleRoot := filepath.Join(dir, "bundle")
	dest := filepath.Join(dir, "dest.txt")
	sourcePath := filepath.Join(bundleRoot, "files", "bin", "tool")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("mkdir bundle source: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("bundle-copy"), 0o644); err != nil {
		t.Fatalf("write bundle source: %v", err)
	}

	if err := runCopyFile(context.Background(), bundleRoot, map[string]any{
		"source": map[string]any{"bundle": map[string]any{"root": "files", "path": "bin/tool"}},
		"path":   dest,
	}); err != nil {
		t.Fatalf("runCopyFile failed: %v", err)
	}

	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(raw) != "bundle-copy" {
		t.Fatalf("unexpected copied content: %q", string(raw))
	}
}

func TestExtractArchiveReadsBundleSourceFromBundleRoot(t *testing.T) {
	dir := t.TempDir()
	bundleRoot := filepath.Join(dir, "bundle")
	destDir := filepath.Join(dir, "out")
	archivePath := filepath.Join(bundleRoot, "files", "archives", "tool.tar.gz")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatalf("mkdir archive dir: %v", err)
	}
	if err := writeTestTarGz(archivePath, map[string]string{"bin/tool": "extracted"}); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	if err := runExtractArchive(context.Background(), bundleRoot, map[string]any{
		"source": map[string]any{"bundle": map[string]any{"root": "files", "path": "archives/tool.tar.gz"}},
		"path":   destDir,
	}); err != nil {
		t.Fatalf("runExtractArchive failed: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(destDir, "bin", "tool"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(raw) != "extracted" {
		t.Fatalf("unexpected extracted content: %q", string(raw))
	}
}

func TestLoadImageReadsArchivesFromBundleRoot(t *testing.T) {
	dir := t.TempDir()
	bundleRoot := filepath.Join(dir, "bundle")
	archivePath := filepath.Join(bundleRoot, "images", "control-plane", sanitizeImageArchiveName("registry.k8s.io/kube-apiserver:v1.30.1")+".tar")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatalf("mkdir image dir: %v", err)
	}
	if err := os.WriteFile(archivePath, []byte("image"), 0o644); err != nil {
		t.Fatalf("write image archive: %v", err)
	}

	if err := runLoadImage(context.Background(), bundleRoot, map[string]any{
		"images":    []string{"registry.k8s.io/kube-apiserver:v1.30.1"},
		"sourceDir": "images/control-plane",
		"command":   []string{"/bin/sh", "-c", "test -f {archive}"},
	}); err != nil {
		t.Fatalf("runLoadImage failed: %v", err)
	}
}
