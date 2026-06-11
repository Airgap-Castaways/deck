package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/errcode"
)

const (
	fakeAptEnvScriptTemplate = "#!/usr/bin/env bash\nset -euo pipefail\necho \"$*\" > \"%s\"\nprintf 'DEBIAN_FRONTEND=%%s\nAPT_LISTCHANGES_FRONTEND=%%s\nNEEDRESTART_MODE=%%s\nNEEDRESTART_SUSPEND=%%s\n' \"${DEBIAN_FRONTEND:-}\" \"${APT_LISTCHANGES_FRONTEND:-}\" \"${NEEDRESTART_MODE:-}\" \"${NEEDRESTART_SUSPEND:-}\" > \"%s\"\nexit 0\n"
	fakeArgsScriptTemplate   = "#!/bin/sh\nprintf '%%s\n' \"$*\" > \"%s\"\nexit 0\n"
)

func verifyAptEnv(t *testing.T, envMarker string) {
	t.Helper()
	envRaw, err := os.ReadFile(envMarker)
	if err != nil {
		t.Fatalf("read env marker: %v", err)
	}
	envText := string(envRaw)
	for _, want := range []string{
		"DEBIAN_FRONTEND=noninteractive",
		"APT_LISTCHANGES_FRONTEND=none",
		"NEEDRESTART_MODE=l",
		"NEEDRESTART_SUSPEND=1",
	} {
		if !strings.Contains(envText, want) {
			t.Fatalf("expected apt env %q, got %q", want, envText)
		}
	}
}

func setPackageInstallHostFamily(t *testing.T, family string) {
	t.Helper()
	origDetect := repoConfigDetectHostFacts
	t.Cleanup(func() { repoConfigDetectHostFacts = origDetect })
	repoConfigDetectHostFacts = func() map[string]any {
		return map[string]any{"os": map[string]any{"family": family}}
	}
}

func TestRun_PackagesExecutesPackageManager(t *testing.T) {
	setPackageInstallHostFamily(t, "debian")
	dir := t.TempDir()
	bundle := filepath.Join(dir, "bundle")
	statePath := filepath.Join(dir, "state", "state.json")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	artifact := filepath.Join(bundle, "files", "a.txt")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		t.Fatalf("mkdir files: %v", err)
	}
	if err := os.WriteFile(artifact, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writeManifestForTest(bundle, "files/a.txt", []byte("ok")); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	marker := filepath.Join(dir, "apt-invoked.txt")
	envMarker := filepath.Join(dir, "apt-env.txt")
	fakeApt := filepath.Join(binDir, "apt-get")
	script := fmt.Sprintf(fakeAptEnvScriptTemplate, marker, envMarker)
	if err := os.WriteFile(fakeApt, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake apt-get: %v", err)
	}

	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, originalPath))

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "install-pkgs",
				Kind: "InstallPackage",
				Spec: map[string]any{"packages": []any{"containerd", "kubelet"}},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
		t.Fatalf("expected install packages success, got %v", err)
	}

	raw, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	args := strings.TrimSpace(string(raw))
	if !strings.Contains(args, "install -y containerd kubelet") {
		t.Fatalf("unexpected apt-get args: %q", args)
	}
	verifyAptEnv(t, envMarker)
}

func TestRun_PackagesSourcePathValidation(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "bundle")
	statePath := filepath.Join(dir, "state", "state.json")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	artifact := filepath.Join(bundle, "files", "a.txt")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		t.Fatalf("mkdir files: %v", err)
	}
	if err := os.WriteFile(artifact, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writeManifestForTest(bundle, "files/a.txt", []byte("ok")); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "install-pkgs",
				Kind: "InstallPackage",
				Spec: map[string]any{
					"packages": []any{"containerd"},
					"source":   map[string]any{"type": "local-repo", "path": filepath.Join(dir, "missing")},
				},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected source validation error")
	}
	if !errcode.Is(err, errCodeInstallPkgSourceInvalid) {
		t.Fatalf("expected typed code %s, got %v", errCodeInstallPkgSourceInvalid, err)
	}
	if !strings.Contains(err.Error(), "E_INSTALL_PACKAGES_SOURCE_INVALID") {
		t.Fatalf("expected E_INSTALL_PACKAGES_SOURCE_INVALID, got %v", err)
	}
}

func TestRun_InstallPackagesFromLocalRepo(t *testing.T) {
	setPackageInstallHostFamily(t, "debian")
	dir := t.TempDir()
	bundle := filepath.Join(dir, "bundle")
	statePath := filepath.Join(dir, "state", "state.json")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	artifact := filepath.Join(bundle, "files", "a.txt")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		t.Fatalf("mkdir files: %v", err)
	}
	if err := os.WriteFile(artifact, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writeManifestForTest(bundle, "files/a.txt", []byte("ok")); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	repoDir := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	debA := filepath.Join(repoDir, "containerd_1.0.0_amd64.deb")
	debB := filepath.Join(repoDir, "kubelet_1.30.1_amd64.deb")
	if err := os.WriteFile(debA, []byte("deb-a"), 0o644); err != nil {
		t.Fatalf("write debA: %v", err)
	}
	if err := os.WriteFile(debB, []byte("deb-b"), 0o644); err != nil {
		t.Fatalf("write debB: %v", err)
	}

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	marker := filepath.Join(dir, "apt-local-invoked.txt")
	envMarker := filepath.Join(dir, "apt-local-env.txt")
	fakeApt := filepath.Join(binDir, "apt-get")
	script := fmt.Sprintf(fakeAptEnvScriptTemplate, marker, envMarker)
	if err := os.WriteFile(fakeApt, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake apt-get: %v", err)
	}

	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, originalPath))

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "install-pkgs",
				Kind: "InstallPackage",
				Spec: map[string]any{
					"packages": []any{"containerd", "kubelet"},
					"source":   map[string]any{"type": "local-repo", "path": repoDir},
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
		t.Fatalf("expected local repo install success, got %v", err)
	}

	raw, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	args := strings.TrimSpace(string(raw))
	if !strings.Contains(args, "install -y") {
		t.Fatalf("unexpected apt-get args: %q", args)
	}
	if !strings.Contains(args, debA) || !strings.Contains(args, debB) {
		t.Fatalf("local deb artifacts were not passed to apt-get: %q", args)
	}
	verifyAptEnv(t, envMarker)
}

func TestRunInstallPackages_AptFixBrokenOption(t *testing.T) {
	setPackageInstallHostFamily(t, "debian")
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	marker := filepath.Join(dir, "apt-args.txt")
	fakeApt := filepath.Join(binDir, "apt-get")
	if err := os.WriteFile(fakeApt, []byte(fmt.Sprintf(fakeArgsScriptTemplate, marker)), 0o755); err != nil {
		t.Fatalf("write fake apt-get: %v", err)
	}
	t.Setenv("PATH", binDir)

	if err := runInstallPackages(context.Background(), map[string]any{"manager": "apt", "packages": []any{"containerd"}, "apt": map[string]any{"fixBroken": true}}); err != nil {
		t.Fatalf("expected install packages success, got %v", err)
	}
	raw, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	args := strings.TrimSpace(string(raw))
	if args != "install -y --fix-broken containerd" {
		t.Fatalf("unexpected apt-get args: %q", args)
	}
}

func TestRunInstallPackages_DnfSkipBrokenOption(t *testing.T) {
	setPackageInstallHostFamily(t, "rhel")
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	marker := filepath.Join(dir, "dnf-args.txt")
	fakeDnf := filepath.Join(binDir, "dnf")
	if err := os.WriteFile(fakeDnf, []byte(fmt.Sprintf(fakeArgsScriptTemplate, marker)), 0o755); err != nil {
		t.Fatalf("write fake dnf: %v", err)
	}
	t.Setenv("PATH", binDir)

	if err := runInstallPackagesForKind(context.Background(), "InstallDnfPackage", map[string]any{"packages": []any{"containerd"}, "dnf": map[string]any{"skipBroken": true}}); err != nil {
		t.Fatalf("expected install packages success, got %v", err)
	}
	raw, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	args := strings.TrimSpace(string(raw))
	if args != "-q --setopt=color=never install -y --skip-broken containerd" {
		t.Fatalf("unexpected dnf args: %q", args)
	}
}

func TestRunInstallPackages_AutoUsesHostFamilyOverBinaryPriority(t *testing.T) {
	setPackageInstallHostFamily(t, "rhel")
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	aptMarker := filepath.Join(dir, "apt-args.txt")
	dnfMarker := filepath.Join(dir, "dnf-args.txt")
	if err := os.WriteFile(filepath.Join(binDir, "apt-get"), []byte(fmt.Sprintf(fakeArgsScriptTemplate, aptMarker)), 0o755); err != nil {
		t.Fatalf("write fake apt-get: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "dnf"), []byte(fmt.Sprintf(fakeArgsScriptTemplate, dnfMarker)), 0o755); err != nil {
		t.Fatalf("write fake dnf: %v", err)
	}
	t.Setenv("PATH", binDir)

	if err := runInstallPackages(context.Background(), map[string]any{"packages": []any{"containerd"}}); err != nil {
		t.Fatalf("expected install packages success, got %v", err)
	}
	if _, err := os.Stat(aptMarker); !os.IsNotExist(err) {
		t.Fatalf("expected apt-get not to be invoked, stat err: %v", err)
	}
	raw, err := os.ReadFile(dnfMarker)
	if err != nil {
		t.Fatalf("read dnf marker: %v", err)
	}
	if args := strings.TrimSpace(string(raw)); args != "-q --setopt=color=never install -y containerd" {
		t.Fatalf("unexpected dnf args: %q", args)
	}
}

func TestRunInstallPackages_DnfLocalArtifactUsesRepoPolicy(t *testing.T) {
	setPackageInstallHostFamily(t, "rhel")
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	rpm := filepath.Join(repoDir, "containerd-1.0.0.x86_64.rpm")
	if err := os.WriteFile(rpm, []byte("rpm"), 0o644); err != nil {
		t.Fatalf("write rpm: %v", err)
	}
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	marker := filepath.Join(dir, "dnf-local-args.txt")
	if err := os.WriteFile(filepath.Join(binDir, "dnf"), []byte(fmt.Sprintf(fakeArgsScriptTemplate, marker)), 0o755); err != nil {
		t.Fatalf("write fake dnf: %v", err)
	}
	t.Setenv("PATH", binDir)

	if err := runInstallPackages(context.Background(), map[string]any{
		"manager":         "dnf",
		"packages":        []any{"containerd"},
		"source":          map[string]any{"type": "local-repo", "path": repoDir},
		"restrictToRepos": []any{"offline"},
		"excludeRepos":    []any{"updates"},
	}); err != nil {
		t.Fatalf("expected install packages success, got %v", err)
	}
	raw, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	args := strings.TrimSpace(string(raw))
	for _, want := range []string{"--disablerepo=*", "--enablerepo=offline", "--disablerepo=updates", rpm} {
		if !strings.Contains(args, want) {
			t.Fatalf("expected %q in dnf args %q", want, args)
		}
	}
}

func TestRunInstallPackages_AptLocalArtifactUsesRepoPolicy(t *testing.T) {
	setPackageInstallHostFamily(t, "debian")
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	deb := filepath.Join(repoDir, "containerd_1.0.0_amd64.deb")
	if err := os.WriteFile(deb, []byte("deb"), 0o644); err != nil {
		t.Fatalf("write deb: %v", err)
	}
	sourceList := filepath.Join(dir, "offline.list")
	if err := os.WriteFile(sourceList, []byte("deb [trusted=yes] file:/repo ./\n"), 0o644); err != nil {
		t.Fatalf("write source list: %v", err)
	}
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	marker := filepath.Join(dir, "apt-local-policy-args.txt")
	if err := os.WriteFile(filepath.Join(binDir, "apt-get"), []byte(fmt.Sprintf(fakeArgsScriptTemplate, marker)), 0o755); err != nil {
		t.Fatalf("write fake apt-get: %v", err)
	}
	t.Setenv("PATH", binDir)

	if err := runInstallPackages(context.Background(), map[string]any{
		"manager":         "apt",
		"packages":        []any{"containerd"},
		"source":          map[string]any{"type": "local-repo", "path": repoDir},
		"restrictToRepos": []any{sourceList},
	}); err != nil {
		t.Fatalf("expected install packages success, got %v", err)
	}
	raw, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	args := strings.TrimSpace(string(raw))
	for _, want := range []string{"Dir::Etc::sourcelist=", "Dir::Etc::sourceparts=", deb} {
		if !strings.Contains(args, want) {
			t.Fatalf("expected %q in apt args %q", want, args)
		}
	}
}

func TestRunInstallPackages_MissingSelectedManagerBinary(t *testing.T) {
	setPackageInstallHostFamily(t, "rhel")
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	t.Setenv("PATH", binDir)

	err := runInstallPackages(context.Background(), map[string]any{"manager": "dnf", "packages": []any{"containerd"}})
	if err == nil || !errcode.Is(err, errCodeInstallPkgMgrMissing) || !strings.Contains(err.Error(), "dnf not found") {
		t.Fatalf("expected missing dnf error, got %v", err)
	}
}

func TestRunInstallPackages_RejectsAmbiguousManagerSpecificOptions(t *testing.T) {
	setPackageInstallHostFamily(t, "debian")
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	fakeApt := filepath.Join(binDir, "apt-get")
	if err := os.WriteFile(fakeApt, []byte(fmt.Sprintf(fakeArgsScriptTemplate, filepath.Join(dir, "apt-args.txt"))), 0o755); err != nil {
		t.Fatalf("write fake apt-get: %v", err)
	}
	t.Setenv("PATH", binDir)

	err := runInstallPackages(context.Background(), map[string]any{"packages": []any{"containerd"}, "apt": map[string]any{"fixBroken": true}})
	if err == nil || !strings.Contains(err.Error(), "requires explicit manager") {
		t.Fatalf("expected explicit manager rejection, got %v", err)
	}

	err = runInstallPackages(context.Background(), map[string]any{"manager": "apt", "packages": []any{"containerd"}, "dnf": map[string]any{"skipBroken": true}})
	if err == nil || !strings.Contains(err.Error(), "dnf options require manager dnf") {
		t.Fatalf("expected manager/block mismatch rejection, got %v", err)
	}
}

func TestRunInstallPackages_ExplicitKindsRejectForeignOptions(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		spec    map[string]any
		message string
	}{
		{
			name:    "apt rejects manager",
			kind:    "InstallAptPackage",
			spec:    map[string]any{"manager": "apt", "packages": []any{"containerd"}},
			message: "does not accept manager",
		},
		{
			name:    "apt rejects dnf options",
			kind:    "InstallAptPackage",
			spec:    map[string]any{"packages": []any{"containerd"}, "dnf": map[string]any{"skipBroken": true}},
			message: "dnf options require InstallDnfPackage",
		},
		{
			name:    "dnf rejects manager",
			kind:    "InstallDnfPackage",
			spec:    map[string]any{"manager": "dnf", "packages": []any{"containerd"}},
			message: "does not accept manager",
		},
		{
			name:    "dnf rejects apt options",
			kind:    "InstallDnfPackage",
			spec:    map[string]any{"packages": []any{"containerd"}, "apt": map[string]any{"fixBroken": true}},
			message: "apt options require InstallAptPackage",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := runInstallPackagesForKind(context.Background(), tc.kind, tc.spec)
			if err == nil || !errcode.Is(err, errCodeInstallPkgOptionInvalid) || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("expected invalid option error containing %q, got %v", tc.message, err)
			}
		})
	}
}

func TestRun_PackagesTimeoutUsesTimeoutClassification(t *testing.T) {
	setPackageInstallHostFamily(t, "debian")
	dir := t.TempDir()
	bundle := filepath.Join(dir, "bundle")
	statePath := filepath.Join(dir, "state", "state.json")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	artifact := filepath.Join(bundle, "files", "a.txt")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		t.Fatalf("mkdir files: %v", err)
	}
	if err := os.WriteFile(artifact, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writeManifestForTest(bundle, "files/a.txt", []byte("ok")); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	fakeApt := filepath.Join(binDir, "apt-get")
	script := "#!/usr/bin/env bash\nset -euo pipefail\nsleep 1\n"
	if err := os.WriteFile(fakeApt, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake apt-get: %v", err)
	}
	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, originalPath))

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:      "install-pkgs",
				Kind:    "InstallPackage",
				Timeout: "20ms",
				Spec:    map[string]any{"packages": []any{"containerd"}},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected install packages timeout")
	}
	if !strings.Contains(err.Error(), errCodeInstallPkgFailed) {
		t.Fatalf("expected package install error code, got %v", err)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout classification, got %v", err)
	}
	if strings.Contains(err.Error(), "package installation failed") {
		t.Fatalf("expected timeout path instead of generic failure, got %v", err)
	}
}
