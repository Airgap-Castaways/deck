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
		spec   map[string]any
		output []string
	}{
		{name: "directory path", kind: "Directory", spec: map[string]any{"path": "/tmp/example"}, output: []string{"path"}},
		{name: "file write path", kind: "File", spec: map[string]any{"action": "write", "path": "/tmp/example"}, output: []string{"path"}},
		{name: "file copy dest", kind: "File", spec: map[string]any{"action": "copy", "dest": "/tmp/copied"}, output: []string{"dest"}},
		{name: "file download outputs", kind: "File", spec: map[string]any{"action": "download", "source": map[string]any{"url": "https://example.invalid/payload.txt"}}, output: []string{"path", "artifacts"}},
		{name: "repository path", kind: "Repository", spec: map[string]any{"action": "configure", "path": "/etc/apt/sources.list.d/offline.list"}, output: []string{"path"}},
		{name: "service name", kind: "Service", spec: map[string]any{"name": "containerd"}, output: []string{"name"}},
		{name: "service names", kind: "Service", spec: map[string]any{"names": []any{"containerd", "kubelet"}}, output: []string{"names"}},
		{name: "kernel module name", kind: "KernelModule", spec: map[string]any{"name": "overlay"}, output: []string{"name"}},
		{name: "kernel module names", kind: "KernelModule", spec: map[string]any{"names": []any{"overlay", "br_netfilter"}}, output: []string{"names"}},
		{name: "kubeadm join file", kind: "Kubeadm", spec: map[string]any{"action": "init", "outputJoinFile": joinPath}, output: []string{"joinFile"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			outputs := stepOutputs(tc.kind, tc.spec)
			for _, key := range tc.output {
				if _, ok := outputs[key]; !ok {
					t.Fatalf("expected runtime output %q for %s", key, tc.kind)
				}
				if !workflowexec.StepHasOutput(tc.kind, tc.spec, key) {
					t.Fatalf("contract missing output %q for %s", key, tc.kind)
				}
			}
		})
	}
}

func writePrivateTestFile(path string, content []byte) error {
	return filemode.WritePrivateFile(path, content)
}
