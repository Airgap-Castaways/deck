package install

import (
	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/stepmeta"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func stepOutputs(kind string, rendered map[string]any) (map[string]any, error) {
	return stepmeta.ProjectRuntimeOutputsForKind(kind, rendered, nil, stepmeta.RuntimeOutputOptions{})
}

func applyRegisterWithSecrets(step config.Step, phase string, rendered map[string]any, outputs map[string]any, runtimeVars map[string]any, runtimeSecrets map[string]RuntimeSecret) error {
	merged, err := stepmeta.ProjectRuntimeOutputsForKind(step.Kind, rendered, outputs, stepmeta.RuntimeOutputOptions{})
	if err != nil {
		return err
	}
	if err := workflowexec.ApplyRegister(step, merged, runtimeVars, errCodeRegisterOutputMissing); err != nil {
		return err
	}
	secretOutputs := secretOutputKeys(step, rendered)
	for runtimeKey, outputKey := range step.Register {
		if secretOutputs[outputKey] {
			runtimeSecrets[runtimeKey] = RuntimeSecret{Phase: phase, StepID: step.ID, Output: outputKey}
			continue
		}
		delete(runtimeSecrets, runtimeKey)
	}
	return nil
}
