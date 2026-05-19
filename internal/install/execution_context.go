package install

import (
	"strings"

	"github.com/Airgap-Castaways/deck/internal/workflowcontext"
)

type ExecutionContext struct {
	BundleRoot string
	StatePath  string
	Context    workflowcontext.Context
	kubeadm    kubeadmExecutor
}

func (c ExecutionContext) RenderContext() map[string]any {
	if strings.TrimSpace(c.Context.Command) != "" {
		return c.Context.RenderMap()
	}
	return map[string]any{
		"bundleRoot": strings.TrimSpace(c.BundleRoot),
		"stateFile":  strings.TrimSpace(c.StatePath),
	}
}
