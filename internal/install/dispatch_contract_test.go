package install

import (
	"testing"

	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func TestInstallStepHandlersCoverApplyKinds(t *testing.T) {
	if _, err := workflowexec.StepRoleHandlers("apply", installStepHandlers); err != nil {
		t.Fatalf("install step handler registration: %v", err)
	}
}
