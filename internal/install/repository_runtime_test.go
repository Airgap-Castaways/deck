package install

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Airgap-Castaways/deck/internal/config"
)

func TestRepoConfigStep(t *testing.T) {
	t.Run("rpm with explicit path", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "repo", "offline.repo")
		spec := map[string]any{
			"format": "rpm",
			"path":   target,
			"repositories": []any{map[string]any{
				"id":       "offline-base",
				"name":     "offline-base",
				"baseurl":  "file:///srv/repo",
				"enabled":  true,
				"gpgcheck": false,
			}},
		}
		if err := runRepoConfig(context.Background(), spec); err != nil {
			t.Fatalf("runRepoConfig failed: %v", err)
		}
		raw, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		got := string(raw)
		if !strings.Contains(got, "[offline-base]") || !strings.Contains(got, "baseurl=file:///srv/repo") {
			t.Fatalf("unexpected repo config: %q", got)
		}
	})

	t.Run("deb list rendering", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "repo", "offline.list")
		spec := map[string]any{
			"format": "deb",
			"path":   target,
			"repositories": []any{map[string]any{
				"baseurl":   "http://repo.local/apt/bookworm",
				"trusted":   true,
				"suite":     "./",
				"component": "main",
			}},
		}
		if err := runRepoConfig(context.Background(), spec); err != nil {
			t.Fatalf("runRepoConfig failed: %v", err)
		}
		raw, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		if strings.TrimSpace(string(raw)) != "deb [trusted=yes] http://repo.local/apt/bookworm ./ main" {
			t.Fatalf("unexpected apt repo config: %q", string(raw))
		}
	})

	t.Run("auto format uses host family and default path", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "repo", "default.list")
		origDetect := repoConfigDetectHostFacts
		origDefaultPath := repoConfigDefaultPathFunc
		t.Cleanup(func() {
			repoConfigDetectHostFacts = origDetect
			repoConfigDefaultPathFunc = origDefaultPath
		})
		repoConfigDetectHostFacts = func() map[string]any {
			return map[string]any{"os": map[string]any{"family": "debian"}}
		}
		repoConfigDefaultPathFunc = func(format string) string {
			if format != "deb" {
				t.Fatalf("expected deb format, got %s", format)
			}
			return target
		}

		spec := map[string]any{
			"format": "auto",
			"repositories": []any{map[string]any{
				"baseurl": "http://repo.local/apt/bookworm",
			}},
		}
		if err := runRepoConfig(context.Background(), spec); err != nil {
			t.Fatalf("runRepoConfig failed: %v", err)
		}
		raw, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		if strings.TrimSpace(string(raw)) != "deb http://repo.local/apt/bookworm ./" {
			t.Fatalf("unexpected apt auto-rendered config: %q", string(raw))
		}
	})

	t.Run("cleanup and backup paths", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "repo", "offline.repo")
		legacyA := filepath.Join(dir, "legacy-a.repo")
		legacyB := filepath.Join(dir, "legacy-b.repo")
		if err := os.WriteFile(legacyA, []byte("[a]\nenabled=1\n"), 0o644); err != nil {
			t.Fatalf("write legacyA: %v", err)
		}
		if err := os.WriteFile(legacyB, []byte("[b]\nenabled=1\n"), 0o644); err != nil {
			t.Fatalf("write legacyB: %v", err)
		}

		spec := map[string]any{
			"format":       "rpm",
			"path":         target,
			"cleanupPaths": []any{filepath.Join(dir, "legacy-*.repo")},
			"backupPaths":  []any{filepath.Join(dir, "legacy-*.repo")},
			"repositories": []any{map[string]any{
				"id":      "offline",
				"baseurl": "file:///srv/repo",
			}},
		}
		if err := runRepoConfig(context.Background(), spec); err != nil {
			t.Fatalf("runRepoConfig failed: %v", err)
		}
		if _, err := os.Stat(legacyA + ".deck.bak"); err != nil {
			t.Fatalf("missing backup for legacyA: %v", err)
		}
		if _, err := os.Stat(legacyB + ".deck.bak"); err != nil {
			t.Fatalf("missing backup for legacyB: %v", err)
		}
		if _, err := os.Stat(legacyA); !os.IsNotExist(err) {
			t.Fatalf("expected legacyA removed, err=%v", err)
		}
		if _, err := os.Stat(legacyB); !os.IsNotExist(err) {
			t.Fatalf("expected legacyB removed, err=%v", err)
		}
	})

	t.Run("disable existing rpm repositories", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "offline.repo")
		existing := filepath.Join(dir, "legacy.repo")
		if err := os.WriteFile(existing, []byte("[legacy]\nname=legacy\nenabled=1\n"), 0o644); err != nil {
			t.Fatalf("write legacy repo: %v", err)
		}

		spec := map[string]any{
			"format":          "rpm",
			"path":            target,
			"disableExisting": true,
			"backupPaths":     []any{existing},
			"repositories": []any{map[string]any{
				"id":      "offline",
				"baseurl": "file:///srv/repo",
			}},
		}
		if err := runRepoConfig(context.Background(), spec); err != nil {
			t.Fatalf("runRepoConfig failed: %v", err)
		}

		raw, err := os.ReadFile(existing)
		if err != nil {
			t.Fatalf("read legacy repo: %v", err)
		}
		if !strings.Contains(string(raw), "enabled=0") {
			t.Fatalf("expected existing repo to be disabled, got %q", string(raw))
		}
	})

	t.Run("disable existing deb source paths", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "offline.list")
		existing := filepath.Join(dir, "legacy.list")
		if err := os.WriteFile(existing, []byte("deb http://legacy.local stable main\n"), 0o644); err != nil {
			t.Fatalf("write legacy apt source: %v", err)
		}

		spec := map[string]any{
			"format":          "deb",
			"path":            target,
			"disableExisting": true,
			"backupPaths":     []any{existing},
			"repositories": []any{map[string]any{
				"baseurl": "http://repo.local/apt/bookworm",
			}},
		}
		if err := runRepoConfig(context.Background(), spec); err != nil {
			t.Fatalf("runRepoConfig failed: %v", err)
		}

		if _, err := os.Stat(existing + ".deck.bak"); err != nil {
			t.Fatalf("missing backup for existing apt source: %v", err)
		}
		if _, err := os.Stat(existing); !os.IsNotExist(err) {
			t.Fatalf("expected existing apt source removed, err=%v", err)
		}

		raw, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read new apt source: %v", err)
		}
		if strings.TrimSpace(string(raw)) != "deb http://repo.local/apt/bookworm ./" {
			t.Fatalf("unexpected apt repo config: %q", string(raw))
		}
	})

	t.Run("repository configure does not refresh cache inline", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "repo", "offline.list")
		origRun := repoConfigRunTimedCommand
		t.Cleanup(func() { repoConfigRunTimedCommand = origRun })
		calls := make([]string, 0)
		repoConfigRunTimedCommand = func(_ context.Context, name string, args []string, timeout time.Duration) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			if timeout < time.Second {
				t.Fatalf("unexpected timeout: %s", timeout)
			}
			return nil
		}

		spec := map[string]any{
			"format": "deb",
			"path":   target,
			"repositories": []any{map[string]any{
				"baseurl": "http://repo.local/apt/bookworm",
			}},
		}
		if err := runRepoConfig(context.Background(), spec); err != nil {
			t.Fatalf("runRepoConfig failed: %v", err)
		}
		if len(calls) != 0 {
			t.Fatalf("expected no refresh commands during configure: %#v", calls)
		}
	})

	t.Run("repository configure only writes repo files", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "repo", "offline.list")
		origRun := repoConfigRunTimedCommand
		t.Cleanup(func() { repoConfigRunTimedCommand = origRun })
		calls := make([]string, 0)
		repoConfigRunTimedCommand = func(_ context.Context, name string, args []string, timeout time.Duration) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		}

		spec := map[string]any{
			"format": "deb",
			"path":   target,
			"repositories": []any{map[string]any{
				"baseurl": "http://repo.local/apt/bookworm",
			}},
		}
		if err := runRepoConfig(context.Background(), spec); err != nil {
			t.Fatalf("runRepoConfig failed: %v", err)
		}
		if len(calls) != 0 {
			t.Fatalf("expected no refresh command sequence: %#v", calls)
		}
	})
}

func TestRefreshRepositoryStep(t *testing.T) {
	t.Run("apt clean only", func(t *testing.T) {
		calls := make([]string, 0)
		runner := func(name string, args []string, timeout time.Duration) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		}

		spec := map[string]any{"manager": "apt", "clean": true}
		if err := runRefreshRepositoryWithRunner(spec, runner); err != nil {
			t.Fatalf("runRefreshRepository failed: %v", err)
		}

		if len(calls) != 1 || calls[0] != "apt-get clean" {
			t.Fatalf("unexpected apt clean commands: %#v", calls)
		}
	})

	t.Run("apt clean plus update ordering", func(t *testing.T) {
		calls := make([]string, 0)
		runner := func(name string, args []string, timeout time.Duration) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		}

		spec := map[string]any{"manager": "apt", "clean": true, "update": true}
		if err := runRefreshRepositoryWithRunner(spec, runner); err != nil {
			t.Fatalf("runRefreshRepository failed: %v", err)
		}

		if len(calls) != 2 || calls[0] != "apt-get clean" || calls[1] != "apt-get update" {
			t.Fatalf("unexpected apt clean/update sequence: %#v", calls)
		}
	})

	t.Run("dnf clean only", func(t *testing.T) {
		calls := make([]string, 0)
		runner := func(name string, args []string, timeout time.Duration) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		}

		spec := map[string]any{"manager": "dnf", "clean": true}
		if err := runRefreshRepositoryWithRunner(spec, runner); err != nil {
			t.Fatalf("runRefreshRepository failed: %v", err)
		}

		if len(calls) != 1 || calls[0] != "dnf clean all" {
			t.Fatalf("unexpected dnf clean commands: %#v", calls)
		}
	})

	t.Run("dnf update uses makecache behavior", func(t *testing.T) {
		calls := make([]string, 0)
		runner := func(name string, args []string, timeout time.Duration) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		}

		spec := map[string]any{"manager": "dnf", "update": true}
		if err := runRefreshRepositoryWithRunner(spec, runner); err != nil {
			t.Fatalf("runRefreshRepository failed: %v", err)
		}

		if len(calls) != 1 || calls[0] != "dnf makecache -y" {
			t.Fatalf("expected makecache call, got %#v", calls)
		}
		for _, call := range calls {
			if strings.Contains(call, "dnf update") || strings.Contains(call, "dnf install") {
				t.Fatalf("dnf update must not perform package upgrade/install, got %q", call)
			}
		}
	})

	t.Run("auto manager resolves using host facts", func(t *testing.T) {
		origDetect := repoConfigDetectHostFacts
		t.Cleanup(func() { repoConfigDetectHostFacts = origDetect })

		calls := make([]string, 0)
		runner := func(name string, args []string, timeout time.Duration) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		}
		repoConfigDetectHostFacts = func() map[string]any {
			return map[string]any{"os": map[string]any{"family": "rhel"}}
		}

		spec := map[string]any{"manager": "auto", "update": true}
		if err := runRefreshRepositoryWithRunner(spec, runner); err != nil {
			t.Fatalf("runRefreshRepository failed: %v", err)
		}

		if len(calls) != 1 || calls[0] != "dnf makecache -y" {
			t.Fatalf("expected auto/rhel to resolve to dnf makecache, got %#v", calls)
		}
	})
}

func TestRun_RepositoryRequiresExplicitAction(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state", "state.json")
	target := filepath.Join(dir, "offline.list")
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "repo-config",
				Kind: "ConfigureRepository",
				Spec: map[string]any{
					"format":       "deb",
					"path":         target,
					"repositories": []any{map[string]any{"id": "offline", "baseurl": "http://repo.local/debian"}},
				},
			}},
		}},
	}
	if err := Run(context.Background(), wf, RunOptions{StatePath: statePath}); err != nil {
		t.Fatalf("expected repository step to run, got %v", err)
	}
}
