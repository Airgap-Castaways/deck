package install

import (
	"path/filepath"
	"testing"

	"github.com/taedi90/deck/internal/filemode"
	"github.com/taedi90/deck/internal/workflowexec"
)

func TestStepOutputsCoverApplyContracts(t *testing.T) {
	tmp := t.TempDir()
	joinPath := filepath.Join(tmp, "join.txt")
	if err := writePrivateTestFile(joinPath, []byte("kubeadm join fake\n")); err != nil {
		t.Fatalf("write join file: %v", err)
	}

	tests := []struct {
		name   string
		kind   string
		action string
		spec   map[string]any
		output []string
	}{
		{name: "directory path", kind: "Directory", spec: map[string]any{"path": "/tmp/example"}, output: []string{"path"}},
		{name: "symlink path", kind: "Symlink", spec: map[string]any{"path": "/usr/local/bin/kubectl", "target": "/opt/bin/kubectl"}, output: []string{"path"}},
		{name: "systemd unit path", kind: "SystemdUnit", spec: map[string]any{"path": "/etc/systemd/system/kubelet.service"}, output: []string{"path"}},
		{name: "containerd path", kind: "Containerd", spec: map[string]any{"path": "/etc/containerd/config.toml"}, output: []string{"path"}},
		{name: "file write path", kind: "File", action: "write", spec: map[string]any{"action": "write", "path": "/tmp/example"}, output: []string{"path"}},
		{name: "file copy dest", kind: "File", action: "copy", spec: map[string]any{"action": "copy", "dest": "/tmp/copied"}, output: []string{"dest"}},
		{name: "file edit path", kind: "File", action: "edit", spec: map[string]any{"action": "edit", "path": "/tmp/edited"}, output: []string{"path"}},
		{name: "file download outputs", kind: "File", action: "download", spec: map[string]any{"action": "download", "source": map[string]any{"url": "https://example.invalid/payload.txt"}}, output: []string{"path", "artifacts"}},
		{name: "repository path", kind: "Repository", action: "configure", spec: map[string]any{"action": "configure", "path": "/etc/apt/sources.list.d/offline.list"}, output: []string{"path"}},
		{name: "service name", kind: "Service", spec: map[string]any{"name": "containerd"}, output: []string{"name"}},
		{name: "service names", kind: "Service", spec: map[string]any{"names": []any{"containerd", "kubelet"}}, output: []string{"names"}},
		{name: "kernel module name", kind: "KernelModule", spec: map[string]any{"name": "overlay"}, output: []string{"name"}},
		{name: "kernel module names", kind: "KernelModule", spec: map[string]any{"names": []any{"overlay", "br_netfilter"}}, output: []string{"names"}},
		{name: "kubeadm join file", kind: "Kubeadm", action: "init", spec: map[string]any{"action": "init", "outputJoinFile": joinPath}, output: []string{"joinFile"}},
	}
	covered := map[string]bool{}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			outputs := stepOutputs(tc.kind, tc.spec)
			for _, key := range tc.output {
				covered[coverageKey(tc.kind, tc.action, key)] = true
				if _, ok := outputs[key]; !ok {
					t.Fatalf("expected runtime output %q for %s", key, tc.kind)
				}
				if !workflowexec.StepHasOutput(tc.kind, tc.spec, key) {
					t.Fatalf("contract missing output %q for %s", key, tc.kind)
				}
			}
		})
	}

	for _, def := range workflowexec.StepDefinitions() {
		if len(def.Actions) == 0 {
			if !contains(def.Roles, "apply") {
				continue
			}
			for _, key := range def.Outputs {
				if !covered[coverageKey(def.Kind, "", key)] {
					t.Fatalf("missing apply output coverage for %s output %s", def.Kind, key)
				}
			}
			continue
		}
		for _, action := range def.Actions {
			if !contains(action.Roles, "apply") {
				continue
			}
			for _, key := range action.Outputs {
				if !covered[coverageKey(def.Kind, action.Name, key)] {
					t.Fatalf("missing apply output coverage for %s.%s output %s", def.Kind, action.Name, key)
				}
			}
		}
	}
}

func writePrivateTestFile(path string, content []byte) error {
	return filemode.WritePrivateFile(path, content)
}

func coverageKey(kind, action, output string) string {
	return kind + ":" + action + ":" + output
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
