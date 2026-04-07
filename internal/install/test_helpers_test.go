package install

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/stepspec"
)

type stubKubeadmExecutor struct {
	initFn    func(stepspec.KubeadmInit) error
	joinFn    func(stepspec.KubeadmJoin) error
	resetFn   func(stepspec.KubeadmReset) error
	upgradeFn func(stepspec.KubeadmUpgrade) error
}

func (s stubKubeadmExecutor) Init(_ context.Context, spec stepspec.KubeadmInit) error {
	if s.initFn == nil {
		return fmt.Errorf("unexpected kubeadm init: %+v", spec)
	}
	return s.initFn(spec)
}

func (s stubKubeadmExecutor) Join(_ context.Context, spec stepspec.KubeadmJoin) error {
	if s.joinFn == nil {
		return fmt.Errorf("unexpected kubeadm join: %+v", spec)
	}
	return s.joinFn(spec)
}

func (s stubKubeadmExecutor) Reset(_ context.Context, spec stepspec.KubeadmReset) error {
	if s.resetFn == nil {
		return fmt.Errorf("unexpected kubeadm reset: %+v", spec)
	}
	return s.resetFn(spec)
}

func (s stubKubeadmExecutor) Upgrade(_ context.Context, spec stepspec.KubeadmUpgrade) error {
	if s.upgradeFn == nil {
		return fmt.Errorf("unexpected kubeadm upgrade: %+v", spec)
	}
	return s.upgradeFn(spec)
}

func useStubInitJoinKubeadm() kubeadmExecutor {
	return stubKubeadmExecutor{
		initFn: runInitKubeadmStub,
		joinFn: runJoinKubeadmStub,
	}
}

func useStubResetKubeadm() kubeadmExecutor {
	return stubKubeadmExecutor{resetFn: runResetKubeadmStub}
}

func useStubUpgradeKubeadm() kubeadmExecutor {
	return stubKubeadmExecutor{upgradeFn: runUpgradeKubeadmStub}
}

func useStubCheckKubernetesCluster(t *testing.T) {
	t.Helper()
	origCheckKubernetesCluster := checkClusterExecutor
	t.Cleanup(func() {
		checkClusterExecutor = origCheckKubernetesCluster
	})
	checkClusterExecutor = func(_ context.Context, spec stepspec.ClusterCheck) error {
		return runCheckKubernetesClusterStub(spec)
	}
}

func nilContextForInstallTest() context.Context { return nil }

func listEditFileBackups(path string) ([]string, error) {
	dir := filepath.Dir(path)
	prefix := filepath.Base(path) + ".bak-"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	backups := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), prefix) {
			backups = append(backups, entry.Name())
		}
	}
	return backups, nil
}

func writeManifestForTest(bundleRoot, relPath string, content []byte) error {
	sum := sha256.Sum256(content)
	entry := map[string]any{
		"path":   relPath,
		"sha256": hex.EncodeToString(sum[:]),
		"size":   len(content),
	}
	manifest := map[string]any{"entries": []any{entry}}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(bundleRoot, ".deck", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(manifestPath, raw, 0o644)
}
