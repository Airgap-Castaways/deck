package install

import (
	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/stepmeta"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func stepOutputs(kind string, rendered map[string]any) (map[string]any, error) {
	return stepmeta.ProjectRuntimeOutputsForKind(kind, rendered, nil, stepmeta.RuntimeOutputOptions{})
}

func applyRegister(step config.Step, rendered map[string]any, outputs map[string]any, runtimeVars map[string]any) error {
	merged, err := stepmeta.ProjectRuntimeOutputsForKind(step.Kind, rendered, outputs, stepmeta.RuntimeOutputOptions{})
	if err != nil {
		return err
	}
	return workflowexec.ApplyRegister(step, merged, runtimeVars, errCodeRegisterOutputMissing)
}
