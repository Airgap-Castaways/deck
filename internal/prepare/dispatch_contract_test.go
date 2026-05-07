package prepare

import (
	"testing"

	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func TestPrepareStepHandlersCoverPrepareKinds(t *testing.T) {
	if _, err := workflowexec.StepRoleHandlers("prepare", prepareStepHandlers); err != nil {
		t.Fatalf("prepare step handler registration: %v", err)
	}
}
