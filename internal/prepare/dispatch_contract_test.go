package prepare

import (
	"testing"

	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func TestPrepareStepHandlersCoverPrepareKinds(t *testing.T) {
	defs, err := workflowexec.StepDefinitions()
	if err != nil {
		t.Fatalf("StepDefinitions: %v", err)
	}
	for _, def := range defs {
		if !contains(def.Roles, "prepare") {
			continue
		}
		if _, ok := prepareStepHandlers[def.Kind]; !ok {
			t.Fatalf("missing prepare handler for prepare kind %s", def.Kind)
		}
	}
}
