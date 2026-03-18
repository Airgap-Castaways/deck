package schemadoc

import (
	"strings"
	"testing"

	"github.com/taedi90/deck/internal/workflowexec"
)

func TestToolMetadataCoversStepKinds(t *testing.T) {
	for _, kind := range workflowexec.StepKinds() {
		if _, ok := toolMetadata[kind]; !ok {
			t.Fatalf("missing tool metadata for kind %s", kind)
		}
	}
}

func TestSharedRegisterExamplesUseGenericOutputs(t *testing.T) {
	for name, example := range map[string]string{
		"common register":   commonFieldDocs["register"].Example,
		"workflow register": WorkflowMeta().FieldDocs["steps[].register"].Example,
	} {
		if strings.Contains(example, "joinCommand") || strings.Contains(example, "joinCmd") {
			t.Fatalf("%s example should not reference kubeadm-specific outputs: %q", name, example)
		}
	}
}

func TestRemovedFieldsStayOutOfPublicMetadata(t *testing.T) {
	checks := []struct {
		kind  string
		field string
	}{
		{kind: "File", field: "spec.owner"},
		{kind: "File", field: "spec.group"},
		{kind: "Wait", field: "spec.state"},
	}
	for _, tc := range checks {
		meta, ok := toolMetadata[tc.kind]
		if !ok {
			t.Fatalf("missing tool metadata for kind %s", tc.kind)
		}
		if _, exists := meta.FieldDocs[tc.field]; exists {
			t.Fatalf("field %s should not appear in %s metadata", tc.field, tc.kind)
		}
	}
}

func TestActionMetadataCoversActionContracts(t *testing.T) {
	for _, kind := range workflowexec.StepKinds() {
		contract, ok := workflowexec.StepContractForKind(kind)
		if !ok || len(contract.Actions) == 0 {
			continue
		}
		meta := toolMetadata[kind]
		for action := range contract.Actions {
			if _, ok := meta.ActionNotes[action]; !ok {
				t.Fatalf("missing action note for %s.%s", kind, action)
			}
			if _, ok := meta.ActionExamples[action]; !ok {
				t.Fatalf("missing action example for %s.%s", kind, action)
			}
		}
	}
}
