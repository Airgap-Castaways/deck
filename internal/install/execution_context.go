package install

import (
	"strings"

	"github.com/Airgap-Castaways/deck/internal/operatorio"
	"github.com/Airgap-Castaways/deck/internal/workflowcontext"
)

type ExecutionContext struct {
	BundleRoot   string
	StatePath    string
	Context      workflowcontext.Context
	Interaction  operatorio.Interface
	SecretValues []string
	kubeadm      kubeadmExecutor
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
