package install

import (
	"testing"

	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func TestInstallStepHandlersCoverApplyKinds(t *testing.T) {
	defs, err := workflowexec.StepDefinitions()
	if err != nil {
		t.Fatalf("StepDefinitions: %v", err)
	}
	for _, def := range defs {
		if !contains(def.Roles, "apply") {
			continue
		}
		if _, ok := installStepHandlers[def.Kind]; !ok {
			t.Fatalf("missing install handler for apply kind %s", def.Kind)
		}
	}
}
