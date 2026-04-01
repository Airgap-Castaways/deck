package askintent

import "testing"

func TestClassifyClarifyForLowInfoPrompt(t *testing.T) {
	decision := Classify(Input{Prompt: "test"})
	if decision.Route != RouteClarify {
		t.Fatalf("expected clarify route, got %s", decision.Route)
	}
	if decision.AllowGeneration {
		t.Fatalf("clarify route must not allow generation")
	}
	if !IsHardOverride(decision) {
		t.Fatalf("expected low-info clarify to be a hard override")
	}
}

func TestClassifyUsesHardOverrideFlags(t *testing.T) {
	if decision := Classify(Input{Prompt: "review workflow", ReviewFlag: true}); decision.Route != RouteReview || !IsHardOverride(decision) {
		t.Fatalf("expected review hard override, got %#v", decision)
	}
	if decision := Classify(Input{Prompt: "create workflow", CreateFlag: true}); decision.Route != RouteDraft || !decision.AllowGeneration || !IsHardOverride(decision) {
		t.Fatalf("expected create hard override, got %#v", decision)
	}
	if decision := Classify(Input{Prompt: "edit workflow", EditFlag: true}); decision.Route != RouteRefine || !decision.AllowGeneration || !IsHardOverride(decision) {
		t.Fatalf("expected edit hard override, got %#v", decision)
	}
}

func TestClassifyDefersNormalPromptsToClassifier(t *testing.T) {
	decision := Classify(Input{Prompt: "Explain how workflows/scenarios/worker-join.yaml works", HasWorkflowTree: true})
	if decision.Route != RouteClarify {
		t.Fatalf("expected classifier handoff placeholder, got %#v", decision)
	}
	if decision.Reason != "classifier required" {
		t.Fatalf("expected classifier-required reason, got %#v", decision)
	}
	if IsHardOverride(decision) {
		t.Fatalf("expected classifier-required decision not to be a hard override")
	}
}

func TestInferTargetKeepsPrepareAndApplyAsWorkspaceScope(t *testing.T) {
	target := inferTarget("create prepare and apply workflows for kubeadm")
	if target.Kind != "workspace" {
		t.Fatalf("expected workspace target, got %#v", target)
	}
}
