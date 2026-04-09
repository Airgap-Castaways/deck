package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2ELayoutContracts(t *testing.T) {
	root := testProjectRoot(t)
	runnerPath := filepath.Join(root, "test", "e2e", "vagrant", "run-scenario.sh")
	renderPath := filepath.Join(root, "test", "e2e", "vagrant", "render-workflows.sh")
	scenarioHelperPath := filepath.Join(root, "test", "e2e", "vagrant", "run-scenario-vm-scenario.sh")
	bootstrapVerifyPath := filepath.Join(root, "test", "workflows", "scenarios", "control-plane-bootstrap-verify.yaml")
	workerJoinVerifyPath := filepath.Join(root, "test", "workflows", "scenarios", "worker-join-cluster-verify.yaml")
	nodeResetVerifyPath := filepath.Join(root, "test", "workflows", "scenarios", "node-reset-worker-verify.yaml")
	legacyVerifyDir := filepath.Join(root, "test", "workflows", "verifications")
	if _, err := os.Stat(runnerPath); err != nil {
		t.Fatalf("stat canonical runner: %v", err)
	}
	if _, err := os.Stat(renderPath); err != nil {
		t.Fatalf("stat workflow renderer: %v", err)
	}
	if _, err := os.Stat(scenarioHelperPath); err != nil {
		t.Fatalf("stat scenario helper: %v", err)
	}
	for _, path := range []string{bootstrapVerifyPath, workerJoinVerifyPath, nodeResetVerifyPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("stat explicit verify workflow %s: %v", path, err)
		}
	}
	if _, err := os.Stat(legacyVerifyDir); !os.IsNotExist(err) {
		t.Fatalf("expected legacy verification tree to be removed, got err=%v", err)
	}
	requireScriptHelpContainsAll(t, runnerPath, "--scenario", "--fresh", "--fresh-cache", "--art-dir")

	layoutContractCmd := "ROOT_DIR='" + root + "'; DECK_VAGRANT_SCENARIO=k8s-worker-join; DECK_VAGRANT_RUN_ID=contract-run; DECK_VAGRANT_CACHE_KEY=contract-cache; source '" + filepath.Join(root, "test", "e2e", "vagrant", "common.sh") + "'; parse_args --art-dir test/tmp/e2e-layout-contract-run; test \"${ART_DIR_REL}\" = test/tmp/e2e-layout-contract-run; test \"${CHECKPOINT_DIR}\" = \"${ROOT_DIR}/test/tmp/e2e-layout-contract-run/checkpoints\"; test \"${RUN_LOG_DIR}\" = \"${ROOT_DIR}/test/tmp/e2e-layout-contract-run/logs\"; test \"${RUN_REPORT_DIR}\" = \"${ROOT_DIR}/test/tmp/e2e-layout-contract-run/reports\"; test \"${RUN_BUNDLE_SOURCE_FILE}\" = \"${ROOT_DIR}/test/tmp/e2e-layout-contract-run/bundle-source.txt\"; test \"${DECK_VAGRANT_CONTROL_PLANE_IP}\" = 192.168.57.10; refresh_layout_contracts; test \"${PREPARED_BUNDLE_REL}\" = test/artifacts/cache/bundles/shared/contract-cache/bundle; test \"${PREPARED_BUNDLE_TAR_REL}\" = test/artifacts/cache/bundles/shared/contract-cache/prepared-bundle.tar; test \"${PREPARED_BUNDLE_WORK_REL}\" = test/artifacts/cache/staging/shared/contract-cache; test \"${CONTROL_PLANE_RSYNC_STAGE_REL}\" = test/artifacts/cache/vagrant/shared/contract-cache/control-plane-rsync-root; test \"${WORKER_RSYNC_STAGE_REL}\" = test/artifacts/cache/vagrant/shared/contract-cache/worker-rsync-root"
	runBashScript(t, root, layoutContractCmd)

	rsyncContractCmd := "ROOT_DIR='" + root + "'; DECK_VAGRANT_SCENARIO=k8s-worker-join; DECK_VAGRANT_RUN_ID=contract-run; DECK_VAGRANT_CACHE_KEY=contract-cache; DECK_VAGRANT_VM_SCENARIO_SCRIPT='" + filepath.Join(root, "test", "e2e", "vagrant", "run-scenario-vm.sh") + "'; DECK_VAGRANT_VM_DISPATCHER_SCRIPT='" + filepath.Join(root, "test", "e2e", "vagrant", "run-scenario-vm.sh") + "'; source '" + filepath.Join(root, "test", "e2e", "vagrant", "common.sh") + "'; refresh_layout_contracts; mkdir -p \"$(dirname \"${PREPARED_BUNDLE_TAR_ABS}\")\" \"$(dirname \"${PREPARED_BUNDLE_STAMP}\")\"; printf 'tarball' > \"${PREPARED_BUNDLE_TAR_ABS}\"; printf 'contract-cache\\n' > \"${PREPARED_BUNDLE_STAMP}\"; prepare_rsync_stage_roots; test -f \"${CONTROL_PLANE_RSYNC_STAGE_ABS}/${PREPARED_BUNDLE_TAR_REL}\"; test ! -e \"${WORKER_RSYNC_STAGE_ABS}/${PREPARED_BUNDLE_TAR_REL}\"; test -f \"${CONTROL_PLANE_RSYNC_STAGE_ABS}/test/e2e/vagrant/run-scenario-vm.sh\"; test -f \"${WORKER_RSYNC_STAGE_ABS}/test/e2e/vagrant/run-scenario-vm.sh\"; test -f \"${WORKER_RSYNC_STAGE_ABS}/test/e2e/vagrant/run-scenario-vm-scenario.sh\"; test ! -e \"${WORKER_RSYNC_STAGE_ABS}/test/e2e/scenario-hooks\"; test ! -e \"${WORKER_RSYNC_STAGE_ABS}/test/e2e/scenario-meta\""
	runBashScript(t, root, rsyncContractCmd)

	tmp := t.TempDir()
	bootstrapContractCmd := "ROOT_DIR='" + root + "'; DECK_VAGRANT_SCENARIO=k8s-control-plane-bootstrap; DECK_VAGRANT_RUN_ID=run-a; DECK_VAGRANT_CACHE_KEY=cache-a; source '" + filepath.Join(root, "test", "e2e", "vagrant", "common.sh") + "'; ART_DIR_ABS='" + filepath.Join(tmp, "bootstrap") + "'; SERVER_URL=http://127.0.0.1:18080; DECK_VAGRANT_PROVIDER=libvirt; RUN_STARTED_AT=2026-01-01T00:00:00Z; mkdir -p \"${ART_DIR_ABS}/reports\"; printf 'nodes\\n' > \"${ART_DIR_ABS}/reports/cluster-nodes.txt\"; validate_collected_artifacts; write_result_contract; test -f \"${ART_DIR_ABS}/pass.txt\"; python3 - <<'PY' \"${ART_DIR_ABS}/result.json\"\nimport json\nimport sys\npath = sys.argv[1]\nwith open(path, 'r', encoding='utf-8') as fp:\n    data = json.load(fp)\nassert data['scenario'] == 'k8s-control-plane-bootstrap'\nassert data['result'] == 'PASS'\nevidence = data['evidence']\nassert evidence['clusterNodes'] == 'reports/cluster-nodes.txt'\nassert 'workerApply' not in evidence\nassert 'workerReset' not in evidence\nPY"
	runBashScript(t, root, bootstrapContractCmd)

	nodeResetContractCmd := "ROOT_DIR='" + root + "'; DECK_VAGRANT_SCENARIO=k8s-node-reset; DECK_VAGRANT_RUN_ID=run-b; DECK_VAGRANT_CACHE_KEY=cache-b; source '" + filepath.Join(root, "test", "e2e", "vagrant", "common.sh") + "'; ART_DIR_ABS='" + filepath.Join(tmp, "node-reset") + "'; SERVER_URL=http://127.0.0.1:18080; DECK_VAGRANT_PROVIDER=libvirt; RUN_STARTED_AT=2026-01-01T00:00:00Z; mkdir -p \"${ART_DIR_ABS}/reports\"; printf 'nodes\\n' > \"${ART_DIR_ABS}/reports/cluster-nodes.txt\"; printf 'pods\\n' > \"${ART_DIR_ABS}/reports/kube-system-pods.txt\"; printf 'ok\\n' > \"${ART_DIR_ABS}/reports/worker-apply.txt\"; printf 'ok\\n' > \"${ART_DIR_ABS}/reports/worker-2-apply.txt\"; printf 'ok\\n' > \"${ART_DIR_ABS}/reports/worker-rejoin.txt\"; printf 'kubeadmReset=ok\\ncontainerd=active\\nkubeletService=active\\n' > \"${ART_DIR_ABS}/reports/reset-state.txt\"; printf 'kubeletServiceAfterRejoin=active\\nkubeletConfigAfterRejoin=present\\n' > \"${ART_DIR_ABS}/reports/rejoin-kubelet.txt\"; validate_collected_artifacts; write_result_contract; test -f \"${ART_DIR_ABS}/pass.txt\"; python3 - <<'PY' \"${ART_DIR_ABS}/result.json\"\nimport json\nimport sys\npath = sys.argv[1]\nwith open(path, 'r', encoding='utf-8') as fp:\n    data = json.load(fp)\nassert data['scenario'] == 'k8s-node-reset'\nassert data['result'] == 'PASS'\nevidence = data['evidence']\nassert evidence['workerApply'] == 'reports/worker-apply.txt'\nassert evidence['worker2Apply'] == 'reports/worker-2-apply.txt'\nassert evidence['workerReset'] == 'reports/reset-state.txt'\nassert evidence['workerRejoin'] == 'reports/worker-rejoin.txt'\nassert evidence['resetState'] == 'reports/reset-state.txt'\nassert evidence['kubeletRejoinState'] == 'reports/rejoin-kubelet.txt'\nPY"
	runBashScript(t, root, nodeResetContractCmd)

	upgradeContractCmd := "ROOT_DIR='" + root + "'; DECK_VAGRANT_SCENARIO=k8s-upgrade; DECK_VAGRANT_RUN_ID=run-c; DECK_VAGRANT_CACHE_KEY=cache-c; source '" + filepath.Join(root, "test", "e2e", "vagrant", "common.sh") + "'; ART_DIR_ABS='" + filepath.Join(tmp, "upgrade") + "'; SERVER_URL=http://127.0.0.1:18080; DECK_VAGRANT_PROVIDER=libvirt; RUN_STARTED_AT=2026-01-01T00:00:00Z; mkdir -p \"${ART_DIR_ABS}/reports\"; printf 'nodes\\n' > \"${ART_DIR_ABS}/reports/cluster-nodes.txt\"; printf 'pods\\n' > \"${ART_DIR_ABS}/reports/kube-system-pods.txt\"; printf 'upgrade\\n' > \"${ART_DIR_ABS}/reports/upgrade-version.txt\"; printf 'upgrade-nodes\\n' > \"${ART_DIR_ABS}/reports/upgrade-nodes.txt\"; validate_collected_artifacts; write_result_contract; test -f \"${ART_DIR_ABS}/pass.txt\"; python3 - <<'PY' \"${ART_DIR_ABS}/result.json\"\nimport json\nimport sys\npath = sys.argv[1]\nwith open(path, 'r', encoding='utf-8') as fp:\n    data = json.load(fp)\nassert data['scenario'] == 'k8s-upgrade'\nassert data['result'] == 'PASS'\nevidence = data['evidence']\nassert evidence['upgradeVersion'] == 'reports/upgrade-version.txt'\nassert evidence['upgradeNodes'] == 'reports/upgrade-nodes.txt'\nassert 'workerApply' not in evidence\nPY"
	runBashScript(t, root, upgradeContractCmd)

	hostMetadataCmd := "ROOT_DIR='" + root + "'; DECK_VAGRANT_SCENARIO=k8s-control-plane-bootstrap; source '" + filepath.Join(root, "test", "e2e", "vagrant", "common.sh") + "'; load_scenario_metadata; test \"${SCENARIO_METADATA_LOADED}\" = 1; test \"${SCENARIO_METADATA_NODES}\" = control-plane; test \"${SCENARIO_METADATA_USES_WORKERS}\" = 0; test \"${SCENARIO_METADATA_VERIFY_STAGE_DEFAULT}\" = bootstrap"
	runBashScript(t, root, hostMetadataCmd)

	upgradeMetadataCmd := "ROOT_DIR='" + root + "'; DECK_VAGRANT_SCENARIO=k8s-upgrade; source '" + filepath.Join(root, "test", "e2e", "vagrant", "common.sh") + "'; load_scenario_metadata; test \"${SCENARIO_METADATA_LOADED}\" = 1; test \"${SCENARIO_METADATA_KUBERNETES_VERSION}\" = v1.30.1; test \"${SCENARIO_METADATA_UPGRADE_KUBERNETES_VERSION}\" = v1.31.0"
	runBashScript(t, root, upgradeMetadataCmd)

	guestHelperCmd := "ROOT_DIR='" + root + "'; source '" + scenarioHelperPath + "'; declare -F apply_offline_guard >/dev/null"
	runBashScript(t, root, guestHelperCmd)

	manifestActionsCmd := "python3 '" + filepath.Join(root, "test", "e2e", "vagrant", "scenario-manifest.py") + "' '" + root + "' k8s-worker-join actions verify cluster | grep 'scenarios/worker-join-cluster-verify.yaml' >/dev/null"
	runBashScript(t, root, manifestActionsCmd)

	renderDir := filepath.Join(tmp, "rendered")
	cmd := exec.Command("bash", filepath.Join(root, "test", "e2e", "vagrant", "render-workflows.sh"), root, renderDir)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("render prepared bundle workflows contract check failed: %v\n%s", err, string(out))
	}
	applyContent, err := os.ReadFile(filepath.Join(renderDir, "scenarios", "control-plane-bootstrap.yaml"))
	if err != nil {
		t.Fatalf("read rendered scenario workflow: %v", err)
	}
	if !strings.Contains(string(applyContent), "bootstrap.yaml") {
		t.Fatalf("expected rendered scenario workflow to keep canonical imports, got:\n%s", string(applyContent))
	}
	upgradeContent, err := os.ReadFile(filepath.Join(renderDir, "scenarios", "upgrade.yaml"))
	if err != nil {
		t.Fatalf("read rendered upgrade workflow: %v", err)
	}
	if !strings.Contains(string(upgradeContent), "upgrade-control-plane") {
		t.Fatalf("expected rendered upgrade workflow to keep upgrade steps, got:\n%s", string(upgradeContent))
	}
}

func testProjectRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, ".."))
}
