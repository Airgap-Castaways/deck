package install

import "strings"

type ExecutionContext struct {
	BundleRoot string
	StatePath  string
	kubeadm    kubeadmExecutor
}

func (c ExecutionContext) RenderContext() map[string]any {
	return map[string]any{
		"bundleRoot": strings.TrimSpace(c.BundleRoot),
		"stateFile":  strings.TrimSpace(c.StatePath),
	}
}
