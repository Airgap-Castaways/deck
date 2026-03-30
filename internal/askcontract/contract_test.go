package askcontract

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/workflowissues"
)

func TestParseInfoFallback(t *testing.T) {
	resp := ParseInfo("plain text answer")
	if resp.Answer == "" {
		t.Fatalf("expected fallback answer")
	}
}

func TestParseGeneration(t *testing.T) {
	raw := `{"summary":"ok","documents":[{"path":"workflows/scenarios/apply.yaml","kind":"workflow","workflow":{"version":"v1alpha1","steps":[{"id":"run","kind":"Command","spec":{"command":["true"]}}]}}]}`
	resp, err := ParseGeneration(raw)
	if err != nil {
		t.Fatalf("parse generation: %v", err)
	}
	if len(resp.Documents) != 1 {
		t.Fatalf("expected one document, got %d", len(resp.Documents))
	}
}

func TestParseGenerationDocuments(t *testing.T) {
	raw := `{"summary":"ok","documents":[{"path":"workflows/scenarios/apply.yaml","kind":"workflow","workflow":{"version":"v1alpha1","steps":[{"id":"run","kind":"Command","spec":{"command":["true"]}}]}}]}`
	resp, err := ParseGeneration(raw)
	if err != nil {
		t.Fatalf("parse generation documents: %v", err)
	}
	if len(resp.Documents) != 1 || resp.Documents[0].Workflow == nil {
		t.Fatalf("expected one workflow document, got %#v", resp.Documents)
	}
	if resp.Documents[0].Workflow.Version != "v1alpha1" {
		t.Fatalf("unexpected workflow document: %#v", resp.Documents[0])
	}
}

func TestParseGenerationRejectsEmptyPayload(t *testing.T) {
	if _, err := ParseGeneration(`{"summary":"ok","review":[]}`); err == nil {
		t.Fatalf("expected parse error for empty generation payload")
	}
}

func TestParseGenerationRejectsLegacyFilesPayload(t *testing.T) {
	if _, err := ParseGeneration(`{"summary":"ok","files":[{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\n"}]}`); err == nil || !strings.Contains(err.Error(), "documents") {
		t.Fatalf("expected legacy files payload to be rejected, got %v", err)
	}
}

func TestParseGenerationNormalizesEditAliases(t *testing.T) {
	resp, err := ParseGeneration(`{"summary":"ok","documents":[{"path":"workflows/scenarios/apply.yaml","kind":"workflow","action":"revise","edits":[{"op":"replace","stepId":"wait-step","path":"spec.timeout","value":"10m"}]}]}`)
	if err != nil {
		t.Fatalf("parse generation aliases: %v", err)
	}
	edit := resp.Documents[0].Edits[0]
	if edit.Op != "set" || edit.RawPath != "steps.wait-step.spec.timeout" {
		t.Fatalf("unexpected normalized edit: %#v", edit)
	}
}

func TestParseGenerationNormalizesTargetAliasEdit(t *testing.T) {
	resp, err := ParseGeneration(`{"summary":"ok","documents":[{"path":"workflows/prepare.yaml","kind":"workflow","action":"revise","edits":[{"op":"add","target":{"kind":"step","id":"prepare-download-packages","field":"spec.backend.image"},"value":"alpine:3.20"}]}]}`)
	if err != nil {
		t.Fatalf("parse generation target alias: %v", err)
	}
	edit := resp.Documents[0].Edits[0]
	if edit.Op != "insert" || edit.RawPath != "steps.prepare-download-packages.spec.backend.image" {
		t.Fatalf("unexpected normalized target edit: %#v", edit)
	}
}

func TestParseGenerationNormalizesPatchActionAndTargetStepID(t *testing.T) {
	resp, err := ParseGeneration(`{"summary":"ok","documents":[{"path":"workflows/prepare.yaml","kind":"workflow","action":"patch","edits":[{"op":"set","targetStepId":"prepare-download-packages","path":"spec.backend.image","value":"alpine:3.20"}]}]}`)
	if err != nil {
		t.Fatalf("parse generation patch action: %v", err)
	}
	if resp.Documents[0].Action != "edit" {
		t.Fatalf("expected patch action to normalize to edit, got %#v", resp.Documents[0])
	}
	if resp.Documents[0].Edits[0].RawPath != "steps.prepare-download-packages.spec.backend.image" {
		t.Fatalf("unexpected normalized targetStepId edit: %#v", resp.Documents[0].Edits[0])
	}
}

func TestParseGenerationNormalizesRemoveAlias(t *testing.T) {
	resp, err := ParseGeneration(`{"summary":"ok","documents":[{"path":"workflows/prepare.yaml","kind":"workflow","action":"revise","edits":[{"op":"remove","stepId":"prepare-download-packages","path":"spec.repo.path"}]}]}`)
	if err != nil {
		t.Fatalf("parse generation remove alias: %v", err)
	}
	if resp.Documents[0].Edits[0].Op != "delete" {
		t.Fatalf("expected delete op, got %#v", resp.Documents[0].Edits[0])
	}
}

func TestParseGenerationNormalizesBracketPath(t *testing.T) {
	resp, err := ParseGeneration(`{"summary":"ok","documents":[{"path":"workflows/scenarios/apply.yaml","kind":"workflow","action":"edit","edits":[{"op":"replace","path":"steps[3].spec.timeout","value":"10m"}]}]}`)
	if err != nil {
		t.Fatalf("parse generation bracket path: %v", err)
	}
	if resp.Documents[0].Edits[0].RawPath != "steps.3.spec.timeout" {
		t.Fatalf("expected normalized bracket path, got %#v", resp.Documents[0].Edits[0])
	}
}

func TestParseGenerationTreatsReviseWithWorkflowAsReplace(t *testing.T) {
	resp, err := ParseGeneration(`{"summary":"ok","documents":[{"path":"workflows/prepare.yaml","kind":"workflow","action":"revise","workflow":{"version":"v1alpha1","steps":[]}}]}`)
	if err != nil {
		t.Fatalf("parse generation revise workflow: %v", err)
	}
	if resp.Documents[0].Action != "replace" {
		t.Fatalf("expected revise workflow action to normalize to replace, got %#v", resp.Documents[0])
	}
}

func TestParseClassification(t *testing.T) {
	raw := `{"route":"explain","confidence":0.82,"reason":"analysis request","target":{"kind":"scenario","path":"workflows/scenarios/apply.yaml","name":"apply"},"generationAllowed":false}`
	resp, err := ParseClassification(raw)
	if err != nil {
		t.Fatalf("parse classification: %v", err)
	}
	if resp.Route != "explain" {
		t.Fatalf("unexpected route: %s", resp.Route)
	}
	if resp.Target.Path != "workflows/scenarios/apply.yaml" {
		t.Fatalf("unexpected target: %#v", resp.Target)
	}
}

func TestParsePlan(t *testing.T) {
	raw := `{"version":1,"request":"create workflow","intent":"draft","complexity":"complex","authoringBrief":{"modeIntent":"apply-only"},"executionModel":{"artifactContracts":[{"kind":"package","producerPath":"workflows/prepare.yaml","consumerPath":"workflows/scenarios/apply.yaml","description":"offline package flow"}],"roleExecution":{"roleSelector":"vars.role","controlPlaneFlow":"bootstrap","workerFlow":"join","perNodeInvocation":true},"verification":{"bootstrapPhase":"bootstrap-control-plane","finalPhase":"verify-cluster","finalVerificationRole":"control-plane","expectedNodeCount":3,"expectedControlPlaneReady":1},"applyAssumptions":["apply consumes local artifacts"]},"offlineAssumption":"offline","needsPrepare":false,"artifactKinds":[],"blockers":[],"targetOutcome":"generate files","assumptions":["use v1alpha1"],"openQuestions":[],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"entry"}],"validationChecklist":["lint"]}`
	resp, err := ParsePlan(raw)
	if err != nil {
		t.Fatalf("parse plan: %v", err)
	}
	if resp.Intent != "draft" || len(resp.Files) != 1 || resp.OfflineAssumption != "offline" {
		t.Fatalf("unexpected plan: %#v", resp)
	}
	if len(resp.ExecutionModel.ArtifactContracts) != 1 || resp.ExecutionModel.RoleExecution.RoleSelector != "vars.role" || resp.ExecutionModel.Verification.ExpectedNodeCount != 3 || resp.ExecutionModel.Verification.FinalVerificationRole != "control-plane" {
		t.Fatalf("expected execution model to parse, got %#v", resp.ExecutionModel)
	}
}

func TestParsePlanInfersOfflineAndArtifacts(t *testing.T) {
	resp, err := ParsePlan(`{"version":1,"request":"create an air-gapped package and image workflow","intent":"draft","complexity":"complex","blockers":[],"targetOutcome":"generate files","assumptions":[],"openQuestions":[],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/prepare.yaml","kind":"scenario","action":"create","purpose":"prepare"},{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"entry"}],"validationChecklist":["lint"]}`)
	if err != nil {
		t.Fatalf("parse plan: %v", err)
	}
	if resp.OfflineAssumption != "" || resp.NeedsPrepare || len(resp.ArtifactKinds) != 0 {
		t.Fatalf("expected inferred offline prepare plan, got %#v", resp)
	}
}

func TestParsePlanMissingRequiredFields(t *testing.T) {
	if _, err := ParsePlan(`{"version":1,"intent":"draft","files":[]}`); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestParsePlanRejectsDescriptiveEntryScenario(t *testing.T) {
	_, err := ParsePlan(`{"version":1,"request":"create workflow","intent":"draft","complexity":"complex","blockers":[],"targetOutcome":"generate files","assumptions":[],"openQuestions":[],"entryScenario":"the apply scenario for this workspace","files":[{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"entry"}],"validationChecklist":["lint"]}`)
	if err == nil {
		t.Fatalf("expected semantic entryScenario error")
	}
	if err != nil && err.Error() == "" {
		t.Fatalf("expected non-empty error")
	}
}

func TestParsePlanRejectsEntryScenarioMissingFromFiles(t *testing.T) {
	_, err := ParsePlan(`{"version":1,"request":"create workflow","intent":"draft","complexity":"complex","blockers":[],"targetOutcome":"generate files","assumptions":[],"openQuestions":[],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/components/base.yaml","kind":"component","action":"create","purpose":"support"}],"validationChecklist":["lint"]}`)
	if err == nil {
		t.Fatalf("expected entryScenario planned file mismatch")
	}
}

func TestParsePlanRepairsTrailingCommas(t *testing.T) {
	resp, err := ParsePlan(`{"version":1,"request":"create workflow","intent":"draft","complexity":"complex","blockers":[],"targetOutcome":"generate files","assumptions":[],"openQuestions":[],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"entry",}],"validationChecklist":["lint",],}`)
	if err != nil {
		t.Fatalf("expected trailing-comma repair, got %v", err)
	}
	if len(resp.Files) != 1 || resp.Files[0].Path != "workflows/scenarios/apply.yaml" {
		t.Fatalf("unexpected repaired plan: %#v", resp)
	}
}

func TestParsePostProcess(t *testing.T) {
	resp, err := ParsePostProcess(`{"summary":"refine apply verification","blocking":[],"advisory":["extract-vars"],"upgradeCandidates":["extract-vars","preserve-inline"],"reviseFiles":["workflows/scenarios/apply.yaml"],"preserveFiles":["workflows/prepare.yaml"],"requiredEdits":["gate final verification"],"verificationExpectations":["lint stays green"],"suggestedFixes":["move shared path into vars"]}`)
	if err != nil {
		t.Fatalf("parse post-process: %v", err)
	}
	if resp.Summary == "" || len(resp.RequiredEdits) != 1 || len(resp.VerificationExpectations) != 1 {
		t.Fatalf("unexpected post-process response: %#v", resp)
	}
}

func TestParsePlanCriticWithStructuredFindings(t *testing.T) {
	resp, err := ParsePlanCritic(`{"summary":"critic summary","blocking":["role selector missing"],"advisory":[],"missingContracts":[],"suggestedFixes":["add vars.role selector"],"findings":[{"code":"missing_role_selector","severity":"blocking","message":"role selector missing","path":"executionModel.roleExecution.roleSelector"},{"code":"ambiguous_join_contract","severity":"advisory","message":"join handoff should be explicit","recoverable":true}]}`)
	if err != nil {
		t.Fatalf("parse plan critic: %v", err)
	}
	if len(resp.Findings) != 2 {
		t.Fatalf("expected findings to parse, got %#v", resp)
	}
	if resp.Findings[0].Code != workflowissues.CodeMissingRoleSelector || resp.Findings[1].Severity != workflowissues.SeverityAdvisory {
		t.Fatalf("unexpected findings: %#v", resp.Findings)
	}
}

func TestParsePlanCriticRejectsInvalidFindingSeverity(t *testing.T) {
	_, err := ParsePlanCritic(`{"summary":"critic summary","blocking":[],"advisory":[],"missingContracts":[],"suggestedFixes":[],"findings":[{"code":"missing_role_selector","severity":"warn","message":"role selector missing"}]}`)
	if err == nil || !strings.Contains(err.Error(), "invalid severity") {
		t.Fatalf("expected invalid severity error, got %v", err)
	}
}
