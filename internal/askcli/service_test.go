package askcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askconfig"
	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askdiagnostic"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askknowledge"
	"github.com/Airgap-Castaways/deck/internal/askpolicy"
	"github.com/Airgap-Castaways/deck/internal/askprovider"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/askscaffold"
	"github.com/Airgap-Castaways/deck/internal/askstate"
	"github.com/Airgap-Castaways/deck/internal/schemadoc"
	"github.com/Airgap-Castaways/deck/internal/testutil/legacygen"
	"github.com/Airgap-Castaways/deck/internal/workflowcontract"
	"github.com/Airgap-Castaways/deck/internal/workflowissues"
	"github.com/Airgap-Castaways/deck/schemas"
)

type stubClient struct {
	responses []string
	calls     int
	prompts   []askprovider.Request
}

type flushBuffer struct {
	bytes.Buffer
	flushes int
}

func enableLegacyAuthoringFallback(t *testing.T) {
	t.Helper()
	t.Setenv("DECK_ASK_ENABLE_LEGACY_AUTHORING_FALLBACK", "1")
}

func (b *flushBuffer) Flush() error {
	b.flushes++
	return nil
}

func (s *stubClient) Generate(_ context.Context, req askprovider.Request) (askprovider.Response, error) {
	s.prompts = append(s.prompts, req)
	defer func() { s.calls++ }()
	if len(s.responses) == 0 {
		return askprovider.Response{}, errors.New("no stub response configured")
	}
	idx := s.calls
	if idx >= len(s.responses) {
		idx = len(s.responses) - 1
	}
	content := s.responses[idx]
	if strings.Contains(strings.TrimSpace(content), `"files"`) && isGenerationLikeKind(req.Kind) {
		content = legacygen.ToDocuments(content)
	}
	return askprovider.Response{Content: content}, nil
}

func isGenerationLikeKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "generate", "generate-fast", "postprocess-edit":
		return true
	default:
		return false
	}
}

func testMaterialized(summary string, files []askcontract.GeneratedFile) askcontract.GenerationResponse {
	return askcontract.GenerationResponse{Summary: summary, Files: files}
}

func TestClassifyWithLLMRetriesMalformedJSON(t *testing.T) {
	client := &stubClient{responses: []string{
		"not-json",
		`{"route":"explain","confidence":0.9,"reason":"analyze existing scenario","target":{"kind":"scenario","path":"workflows/scenarios/apply.yaml","name":"apply"},"generationAllowed":false}`,
	}}
	decision, err := classifyWithLLM(
		context.Background(),
		client,
		askconfig.EffectiveSettings{Settings: askconfig.Settings{Provider: "openai", Model: "gpt-5.4", APIKey: "test-key"}},
		classifierSystemPrompt(),
		classifierUserPrompt("explain apply", false, askretrieve.WorkspaceSummary{HasWorkflowTree: true}),
		newAskLogger(io.Discard, "trace"),
	)
	if err != nil {
		t.Fatalf("classify with llm: %v", err)
	}
	if client.calls != 2 {
		t.Fatalf("expected retry on malformed classifier json, got %d calls", client.calls)
	}
	if decision.Route != askintent.RouteExplain || decision.Target.Path != "workflows/scenarios/apply.yaml" {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestGenerateWithValidationStopsOnRouteMismatch(t *testing.T) {
	client := &stubClient{responses: []string{
		`{"summary":"wrong route","review":[],"files":[]}`,
		`{"summary":"should not retry","review":[],"files":[{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\n"}]}`,
	}}
	_, _, _, _, _, _, err := generateWithValidation(context.Background(), client, askprovider.Request{Kind: "generate", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key"}, t.TempDir(), 2, newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteDraft}, askcontract.PlanResponse{}, askcontract.AuthoringBrief{}, askretrieve.RetrievalResult{}, askcontract.PlanCriticResponse{})
	if err == nil {
		t.Fatalf("expected generation failure")
	}
	if !strings.Contains(err.Error(), "without repair") {
		t.Fatalf("expected non-repairable termination, got %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("expected non-repairable failure to stop after one call, got %d", client.calls)
	}
}

func TestClassifyWithLLMReturnsSemanticErrorForInvalidRoute(t *testing.T) {
	client := &stubClient{responses: []string{`{"route":"mystery","confidence":0.9,"reason":"bad route","target":{"kind":"workspace"},"generationAllowed":false}`}}
	_, err := classifyWithLLM(context.Background(), client, askconfig.EffectiveSettings{Settings: askconfig.Settings{Provider: "openai", Model: "gpt-5.4", APIKey: "test-key"}}, classifierSystemPrompt(), classifierUserPrompt("do something", false, askretrieve.WorkspaceSummary{}), newAskLogger(io.Discard, "trace"))
	if err == nil {
		t.Fatalf("expected semantic classifier error")
	}
	var cErr classifierError
	if !errors.As(err, &cErr) || cErr.kind != classifierErrorSemantic {
		t.Fatalf("expected semantic classifier error, got %v", err)
	}
}

func TestClassifyWithLLMReturnsSemanticErrorForLowConfidence(t *testing.T) {
	client := &stubClient{responses: []string{`{"route":"explain","confidence":0.2,"reason":"unclear","target":{"kind":"workspace"},"generationAllowed":false}`}}
	_, err := classifyWithLLM(context.Background(), client, askconfig.EffectiveSettings{Settings: askconfig.Settings{Provider: "openai", Model: "gpt-5.4", APIKey: "test-key"}}, classifierSystemPrompt(), classifierUserPrompt("do something", false, askretrieve.WorkspaceSummary{}), newAskLogger(io.Discard, "trace"))
	if err == nil {
		t.Fatalf("expected low-confidence semantic classifier error")
	}
	var cErr classifierError
	if !errors.As(err, &cErr) || cErr.kind != classifierErrorSemantic {
		t.Fatalf("expected semantic classifier error, got %v", err)
	}
}

func TestBuildPlanWithReviewRetriesOnPlanCriticBlocking(t *testing.T) {
	client := &stubClient{responses: []string{
		`{"version":1,"request":"create 3-node kubeadm workflow","intent":"draft","complexity":"complex","authoringBrief":{"routeIntent":"draft","targetScope":"workspace","targetPaths":["workflows/prepare.yaml","workflows/scenarios/apply.yaml"],"modeIntent":"prepare+apply","connectivity":"offline","completenessTarget":"complete","topology":"multi-node","nodeCount":3,"requiredCapabilities":["prepare-artifacts","kubeadm-bootstrap","kubeadm-join"]},"authoringProgram":{"platform":{"family":"rhel","release":"9"},"artifacts":{"packages":["kubeadm","kubelet","kubectl"]},"cluster":{"joinFile":"/tmp/deck/join.txt"},"verification":{"expectedNodeCount":3}},"executionModel":{"artifactContracts":[{"kind":"package","producerPath":"workflows/prepare.yaml","consumerPath":"workflows/scenarios/apply.yaml","description":"offline package flow"}],"roleExecution":{"roleSelector":"vars.role","controlPlaneFlow":"bootstrap","workerFlow":"join","perNodeInvocation":true},"verification":{"bootstrapPhase":"bootstrap-control-plane","finalPhase":"verify-cluster","expectedNodeCount":3,"expectedControlPlaneReady":1},"applyAssumptions":["apply consumes local artifacts"]},"offlineAssumption":"offline","needsPrepare":true,"artifactKinds":["package"],"blockers":[],"targetOutcome":"generate files","assumptions":[],"openQuestions":[],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/prepare.yaml","kind":"workflow","action":"create","purpose":"prepare"},{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"apply"}],"validationChecklist":["lint"]}`,
		`{"summary":"plan is not viable yet","blocking":["multi-role request has no viable role selector or branching model"],"advisory":[],"missingContracts":[],"suggestedFixes":["Add executionModel.roleExecution.roleSelector for control-plane and worker branching"],"findings":[{"code":"missing_role_selector","severity":"blocking","message":"multi-role request has no viable role selector or branching model","path":"executionModel.roleExecution.roleSelector"}]}`,
		`{"version":1,"request":"create 3-node kubeadm workflow","intent":"draft","complexity":"complex","authoringBrief":{"routeIntent":"draft","targetScope":"workspace","targetPaths":["workflows/prepare.yaml","workflows/scenarios/apply.yaml"],"modeIntent":"prepare+apply","connectivity":"offline","completenessTarget":"complete","topology":"multi-node","nodeCount":3,"requiredCapabilities":["prepare-artifacts","kubeadm-bootstrap","kubeadm-join"]},"authoringProgram":{"platform":{"family":"rhel","release":"9"},"artifacts":{"packages":["kubeadm","kubelet","kubectl"]},"cluster":{"joinFile":"/tmp/deck/join.txt"},"verification":{"expectedNodeCount":3}},"executionModel":{"artifactContracts":[{"kind":"package","producerPath":"workflows/prepare.yaml","consumerPath":"workflows/scenarios/apply.yaml","description":"offline package flow"}],"sharedStateContracts":[{"name":"join-file","producerPath":"/tmp/deck/join.txt","consumerPaths":["/tmp/deck/join.txt"],"availabilityModel":"published-for-worker-consumption","description":"publish join file for workers"}],"roleExecution":{"roleSelector":"vars.role","controlPlaneFlow":"bootstrap","workerFlow":"join","perNodeInvocation":true},"verification":{"bootstrapPhase":"bootstrap-control-plane","finalPhase":"verify-cluster","expectedNodeCount":3,"expectedControlPlaneReady":1},"applyAssumptions":["apply consumes local artifacts"]},"offlineAssumption":"offline","needsPrepare":true,"artifactKinds":["package"],"blockers":[],"targetOutcome":"generate files","assumptions":[],"openQuestions":[],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/prepare.yaml","kind":"workflow","action":"create","purpose":"prepare"},{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"apply"}],"validationChecklist":["lint"]}`,
		`{"summary":"plan is ready","blocking":[],"advisory":["role-aware execution is explicit"],"missingContracts":[],"suggestedFixes":[]}`,
	}}
	plan, critic, usedFallback, err := buildPlanWithReview(context.Background(), client, askconfigSettings{provider: "openai", model: "gpt-5.4", apiKey: "test-key"}, askintent.Decision{Route: askintent.RouteDraft}, askretrieve.RetrievalResult{}, "create an air-gapped rhel9 3-node kubeadm workflow", askretrieve.WorkspaceSummary{}, askpolicy.BuildScenarioRequirements("create an air-gapped rhel9 3-node kubeadm workflow", askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft}), newAskLogger(io.Discard, "trace"))
	if err != nil {
		t.Fatalf("buildPlanWithReview: %v", err)
	}
	if usedFallback {
		t.Fatalf("expected reviewed plan without fallback")
	}
	if client.calls != 4 {
		t.Fatalf("expected two plan attempts and two critic attempts, got %d calls", client.calls)
	}
	if len(plan.ExecutionModel.SharedStateContracts) != 1 {
		t.Fatalf("expected second plan to include shared-state contract, got %#v", plan.ExecutionModel)
	}
	if len(critic.Blocking) != 0 || critic.Summary != "plan is ready" {
		t.Fatalf("expected final non-blocking critic result, got %#v", critic)
	}
	if len(client.prompts) < 3 || (!strings.Contains(client.prompts[2].Prompt, "role selector") && !strings.Contains(client.prompts[2].Prompt, "Required plan updates before generation")) {
		t.Fatalf("expected replanning prompt to include plan critic findings, got %#v", client.prompts)
	}
}

func TestNormalizePlanCriticDowngradesRecoverableIssues(t *testing.T) {
	plan := askcontract.PlanResponse{ExecutionModel: askcontract.ExecutionModel{
		ArtifactContracts:    []askcontract.ArtifactContract{{Kind: "package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}},
		SharedStateContracts: []askcontract.SharedStateContract{{Name: "join-file", ProducerPath: "/tmp/deck/join.txt", ConsumerPaths: []string{"/tmp/deck/join.txt"}, AvailabilityModel: "published-for-worker-consumption"}},
		RoleExecution:        askcontract.RoleExecutionModel{RoleSelector: "vars.role"},
		Verification:         askcontract.VerificationStrategy{FinalVerificationRole: "control-plane"},
	}}
	critic := normalizePlanCritic(plan, askcontract.PlanCriticResponse{
		Blocking: []string{
			"artifact consumers should bind to explicit artifact contracts",
			"running CheckCluster in both control-plane and worker flows is not realistic",
		},
		MissingContracts: []string{"join-file publication contract"},
	})
	if len(critic.Blocking) != 0 || len(critic.MissingContracts) != 0 {
		t.Fatalf("expected recoverable issues to downgrade, got %#v", critic)
	}
	joined := strings.Join(critic.Advisory, "\n")
	for _, want := range []string{"CheckCluster"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in advisory, got %#v", want, critic)
		}
	}
	for _, avoid := range []string{"artifact consumers should bind"} {
		if strings.Contains(joined, avoid) {
			t.Fatalf("expected normalized plan to suppress %q, got %#v", avoid, critic)
		}
	}
	if !strings.Contains(joined, "join-file publication contract") {
		t.Fatalf("expected join publication advisory to remain without a fully normalized join handoff, got %#v", critic)
	}
}

func TestNormalizePlanCriticPrefersStructuredFindingCodes(t *testing.T) {
	plan := askcontract.PlanResponse{}
	critic := normalizePlanCritic(plan, askcontract.PlanCriticResponse{
		Findings: []askcontract.PlanCriticFinding{
			{Code: workflowissues.CodeMissingRoleSelector, Severity: workflowissues.SeverityBlocking, Message: "role selector missing"},
			{Code: workflowissues.CodeAmbiguousJoinContract, Severity: workflowissues.SeverityMissingContract, Message: "join publication path should be explicit", Recoverable: true},
		},
	})
	if len(critic.Blocking) != 1 || critic.Blocking[0] != "role selector missing" {
		t.Fatalf("expected fatal finding to remain blocking, got %#v", critic)
	}
	if len(critic.MissingContracts) != 0 {
		t.Fatalf("expected recoverable missing contract to downgrade, got %#v", critic)
	}
	if len(critic.Advisory) != 1 || critic.Advisory[0] != "join publication path should be explicit" {
		t.Fatalf("expected recoverable finding to become advisory, got %#v", critic)
	}
}

func TestNormalizePlanCriticDowngradesRecoverableChecksumAndCardinalityRequests(t *testing.T) {
	plan := askcontract.PlanResponse{ExecutionModel: askcontract.ExecutionModel{
		ArtifactContracts:    []askcontract.ArtifactContract{{Kind: "package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}, {Kind: "image", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}},
		SharedStateContracts: []askcontract.SharedStateContract{{Name: "join-file", ProducerPath: "/tmp/deck/join.txt", ConsumerPaths: []string{"/tmp/deck/join.txt"}, AvailabilityModel: "published-for-worker-consumption"}},
		RoleExecution:        askcontract.RoleExecutionModel{RoleSelector: "vars.role"},
		Verification:         askcontract.VerificationStrategy{FinalVerificationRole: "control-plane", ExpectedNodeCount: 3, ExpectedControlPlaneReady: 1},
	}}
	critic := normalizePlanCritic(plan, askcontract.PlanCriticResponse{
		Blocking:         []string{"the plan defines two join-state paths and should use a single, canonical worker-consumed join contract"},
		MissingContracts: []string{"vars.artifacts.images.checksum contract object", "role cardinality contract for vars"},
	})
	if len(critic.Blocking) != 0 || len(critic.MissingContracts) != 0 {
		t.Fatalf("expected recoverable checksum/cardinality issues to downgrade, got %#v", critic)
	}
	joined := strings.Join(critic.Advisory, "\n")
	for _, want := range []string{"single, canonical worker-consumed join contract", "checksum contract object", "role cardinality contract"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in advisory, got %#v", want, critic)
		}
	}
}

func TestNormalizePlanCriticDowngradesGpt54OperationalCompletenessLanguage(t *testing.T) {
	plan := askcontract.PlanResponse{ExecutionModel: askcontract.ExecutionModel{
		ArtifactContracts:    []askcontract.ArtifactContract{{Kind: "package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}, {Kind: "image", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}},
		SharedStateContracts: []askcontract.SharedStateContract{{Name: "join-file", ProducerPath: "workflows/scenarios/apply.yaml", ConsumerPaths: []string{"workflows/scenarios/apply.yaml"}, AvailabilityModel: "published-for-worker-consumption"}},
		RoleExecution:        askcontract.RoleExecutionModel{RoleSelector: "vars.role"},
		Verification:         askcontract.VerificationStrategy{FinalVerificationRole: "control-plane", ExpectedNodeCount: 3, ExpectedControlPlaneReady: 1},
	}}
	critic := normalizePlanCritic(plan, askcontract.PlanCriticResponse{
		Blocking: []string{
			"workflows/scenarios/apply.yaml: The join handoff is not executable as written. reachable through the prepared local file-serving path or equivalent offline shared path is too vague for a shared-state contract",
			"Execution model / workflows/scenarios/apply.yaml: Final CheckCluster on the control-plane can race worker joins because apply is described as per-node role-based execution with no barrier or wait contract",
			"workflows/prepare.yaml -> workflows/scenarios/apply.yaml: The offline image artifact contract is underspecified. The plan does not define the produced image format/bundle layout or the exact path apply LoadImage consumes",
		},
		MissingContracts: []string{
			"Execution model: Synchronization contract between worker joins and the final control-plane CheckCluster step.",
			"Execution model / topology: Role cardinality contract for 3 nodes = 1 control-plane + 2 workers",
		},
	})
	if len(critic.Blocking) != 0 || len(critic.MissingContracts) != 0 {
		t.Fatalf("expected operational-completeness gaps to downgrade, got %#v", critic)
	}
	joined := strings.Join(critic.Advisory, "\n")
	for _, want := range []string{"join handoff is not executable as written", "can race worker joins", "Synchronization contract", "Role cardinality contract"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in advisory, got %#v", want, critic)
		}
	}
	if strings.Contains(joined, "image artifact contract is underspecified") {
		t.Fatalf("expected canonical prepare/apply artifact contracts to suppress image-contract noise, got %#v", critic)
	}
}

func TestNormalizePlanCriticSuppressesArtifactContractGapWhenCanonicalContractsExist(t *testing.T) {
	plan := askcontract.PlanResponse{
		ArtifactKinds: []string{"package", "image"},
		NeedsPrepare:  true,
		EntryScenario: "workflows/scenarios/apply.yaml",
		ExecutionModel: askcontract.ExecutionModel{
			ArtifactContracts: []askcontract.ArtifactContract{{Kind: "package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}, {Kind: "image", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}},
		},
	}
	critic := normalizePlanCritic(plan, askcontract.PlanCriticResponse{Findings: []askcontract.PlanCriticFinding{
		{Code: workflowissues.CodeArtifactContractGap, Severity: workflowissues.SeverityMissingContract, Message: "artifact contract names should be explicit", Recoverable: true},
		{Code: workflowissues.CodeMissingArtifactConsumer, Severity: workflowissues.SeverityAdvisory, Message: "apply consumer bindings are underspecified", Recoverable: true},
	}})
	if len(critic.Blocking) != 0 || len(critic.MissingContracts) != 0 || len(critic.Advisory) != 0 {
		t.Fatalf("expected canonical prepare/apply contracts to suppress artifact-gap noise, got %#v", critic)
	}
}

func TestNormalizePlanCriticSuppressesJoinNoiseWhenPublishedJoinContractExists(t *testing.T) {
	plan := askcontract.PlanResponse{
		AuthoringBrief:   askcontract.AuthoringBrief{Topology: "multi-node", NodeCount: 2, RequiredCapabilities: []string{"kubeadm-bootstrap", "kubeadm-join"}},
		AuthoringProgram: askcontract.AuthoringProgram{Cluster: askcontract.ProgramCluster{JoinFile: "/tmp/deck/join.txt"}},
		ExecutionModel: askcontract.ExecutionModel{
			SharedStateContracts: []askcontract.SharedStateContract{{Name: "join-file", ProducerPath: "/tmp/deck/join.txt", ConsumerPaths: []string{"/tmp/deck/join.txt"}, AvailabilityModel: "published-for-worker-consumption"}},
			RoleExecution:        askcontract.RoleExecutionModel{RoleSelector: "vars.role", PerNodeInvocation: true, WorkerFlow: "join"},
		},
	}
	critic := normalizePlanCritic(plan, askcontract.PlanCriticResponse{Findings: []askcontract.PlanCriticFinding{
		{Code: workflowissues.CodeAmbiguousJoinContract, Severity: workflowissues.SeverityAdvisory, Message: "join publication should be more explicit", Recoverable: true},
		{Code: workflowissues.CodeWorkerJoinFanoutGap, Severity: workflowissues.SeverityAdvisory, Message: "worker joins should wait on join publication", Recoverable: true},
	}})
	if len(critic.Blocking) != 0 || len(critic.MissingContracts) != 0 || len(critic.Advisory) != 0 {
		t.Fatalf("expected canonical published join handoff to suppress join-contract noise, got %#v", critic)
	}
}

func TestNormalizePlanCriticSuppressesRoleAndVerificationNoiseWhenCountsAreExplicit(t *testing.T) {
	plan := askcontract.PlanResponse{
		AuthoringBrief:   askcontract.AuthoringBrief{Topology: "multi-node", NodeCount: 2, RequiredCapabilities: []string{"kubeadm-bootstrap", "kubeadm-join", "cluster-verification"}},
		AuthoringProgram: askcontract.AuthoringProgram{Cluster: askcontract.ProgramCluster{JoinFile: "/tmp/deck/join.txt", ControlPlaneCount: 1, WorkerCount: 1}},
		ExecutionModel: askcontract.ExecutionModel{
			SharedStateContracts: []askcontract.SharedStateContract{{Name: "join-file", ProducerPath: "/tmp/deck/join.txt", ConsumerPaths: []string{"/tmp/deck/join.txt"}, AvailabilityModel: "published-for-worker-consumption"}},
			RoleExecution:        askcontract.RoleExecutionModel{RoleSelector: "vars.role", PerNodeInvocation: true, WorkerFlow: "join", ControlPlaneFlow: "bootstrap"},
			Verification:         askcontract.VerificationStrategy{FinalVerificationRole: "control-plane", ExpectedNodeCount: 2, ExpectedControlPlaneReady: 1},
		},
	}
	critic := normalizePlanCritic(plan, askcontract.PlanCriticResponse{Findings: []askcontract.PlanCriticFinding{
		{Code: workflowissues.CodeRoleCardinalityGap, Severity: workflowissues.SeverityAdvisory, Message: "role counts should be more explicit", Recoverable: true},
		{Code: workflowissues.CodeWeakVerificationStaging, Severity: workflowissues.SeverityAdvisory, Message: "final CheckCluster should wait for worker join completion", Recoverable: true},
	}})
	if len(critic.Blocking) != 0 || len(critic.MissingContracts) != 0 || len(critic.Advisory) != 0 {
		t.Fatalf("expected explicit role counts and final verification contract to suppress residual review noise, got %#v", critic)
	}
}

func TestHasFatalPlanReviewIssuesOnlyForNonViablePlans(t *testing.T) {
	viable := askcontract.PlanResponse{
		AuthoringBrief: askcontract.AuthoringBrief{ModeIntent: "prepare+apply", Topology: "multi-node"},
		EntryScenario:  "workflows/scenarios/apply.yaml",
		Files:          []askcontract.PlanFile{{Path: "workflows/prepare.yaml"}, {Path: "workflows/scenarios/apply.yaml"}},
		ExecutionModel: askcontract.ExecutionModel{RoleExecution: askcontract.RoleExecutionModel{RoleSelector: "vars.role"}, ArtifactContracts: []askcontract.ArtifactContract{{Kind: "package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}}},
		NeedsPrepare:   true,
	}
	critic := askcontract.PlanCriticResponse{Advisory: []string{"join publication path could be more explicit"}, MissingContracts: []string{"topology cardinality vars contract"}}
	if hasFatalPlanReviewIssues(viable, critic) {
		t.Fatalf("expected recoverable review issues to proceed to generation")
	}
	fatalPlan := viable
	fatalPlan.EntryScenario = ""
	if !hasFatalPlanReviewIssues(fatalPlan, askcontract.PlanCriticResponse{}) {
		t.Fatalf("expected missing entry scenario to remain fatal")
	}
}

func TestHasFatalPlanReviewIssuesAllowsRecoverableMissingContracts(t *testing.T) {
	plan := askcontract.PlanResponse{
		AuthoringBrief: askcontract.AuthoringBrief{ModeIntent: "prepare+apply", Topology: "multi-node", RequiredCapabilities: []string{"kubeadm-join"}},
		EntryScenario:  "workflows/scenarios/apply.yaml",
		Files:          []askcontract.PlanFile{{Path: "workflows/prepare.yaml"}, {Path: "workflows/scenarios/apply.yaml"}},
		NeedsPrepare:   true,
	}
	critic := askcontract.PlanCriticResponse{MissingContracts: []string{"join publication contract", "topology cardinality contract"}}
	if hasFatalPlanReviewIssues(plan, critic) {
		t.Fatalf("expected recoverable missing contracts to proceed to generation")
	}
}

func TestHasFatalPlanReviewIssuesRequiresSharedStateContractGraph(t *testing.T) {
	plan := askcontract.PlanResponse{
		Request:           "create three-node kubeadm workflow with workers",
		OfflineAssumption: "offline",
		AuthoringBrief:    askcontract.AuthoringBrief{ModeIntent: "prepare+apply", Topology: "multi-node", RequiredCapabilities: []string{"kubeadm-bootstrap", "kubeadm-join"}},
		EntryScenario:     "workflows/scenarios/apply.yaml",
		Files:             []askcontract.PlanFile{{Path: "workflows/prepare.yaml"}, {Path: "workflows/scenarios/apply.yaml"}},
		ExecutionModel:    askcontract.ExecutionModel{ArtifactContracts: []askcontract.ArtifactContract{{Kind: "package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}}},
		NeedsPrepare:      true,
		ArtifactKinds:     []string{"package"},
	}
	if !hasFatalPlanReviewIssues(plan, askcontract.PlanCriticResponse{}) {
		t.Fatalf("expected missing shared-state graph to be fatal")
	}
}

func TestHasFatalPlanReviewIssuesTreatsPlannerBlockersAsFatal(t *testing.T) {
	plan := askcontract.PlanResponse{
		AuthoringBrief: askcontract.AuthoringBrief{ModeIntent: "prepare+apply", Topology: "multi-node"},
		EntryScenario:  "workflows/scenarios/apply.yaml",
		Files:          []askcontract.PlanFile{{Path: "workflows/prepare.yaml"}, {Path: "workflows/scenarios/apply.yaml"}},
		ExecutionModel: askcontract.ExecutionModel{RoleExecution: askcontract.RoleExecutionModel{RoleSelector: "vars.role"}, ArtifactContracts: []askcontract.ArtifactContract{{Kind: "package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}}},
		NeedsPrepare:   true,
		Blockers:       []string{"join publication path is still underspecified", "final verification placement could be stronger"},
	}
	if !hasFatalPlanReviewIssues(plan, askcontract.PlanCriticResponse{}) {
		t.Fatalf("expected explicit planner blockers to stop generation")
	}
}

func TestHasFatalPlanReviewIssuesTreatsSinglePlannerBlockerAsFatal(t *testing.T) {
	plan := askcontract.PlanResponse{
		AuthoringBrief: askcontract.AuthoringBrief{ModeIntent: "prepare+apply", Topology: "multi-node"},
		EntryScenario:  "workflows/scenarios/apply.yaml",
		Files:          []askcontract.PlanFile{{Path: "workflows/prepare.yaml"}, {Path: "workflows/scenarios/apply.yaml"}},
		ExecutionModel: askcontract.ExecutionModel{RoleExecution: askcontract.RoleExecutionModel{RoleSelector: "vars.role"}, ArtifactContracts: []askcontract.ArtifactContract{{Kind: "package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}}},
		NeedsPrepare:   true,
		Blockers:       []string{"no viable role selector is available for the worker/control-plane branching model"},
	}
	if !hasFatalPlanReviewIssues(plan, askcontract.PlanCriticResponse{}) {
		t.Fatalf("expected planner blocker prose to stop generation once marked as blocker")
	}
}

func TestHasFatalPlanReviewIssuesIgnoresRecoverableGpt54PlanCriticBlockers(t *testing.T) {
	plan := askcontract.PlanResponse{
		AuthoringBrief: askcontract.AuthoringBrief{ModeIntent: "prepare+apply", Topology: "multi-node", RequiredCapabilities: []string{"kubeadm-join", "cluster-verification"}},
		EntryScenario:  "workflows/scenarios/apply.yaml",
		Files:          []askcontract.PlanFile{{Path: "workflows/prepare.yaml"}, {Path: "workflows/scenarios/apply.yaml"}, {Path: "workflows/vars.yaml"}},
		ExecutionModel: askcontract.ExecutionModel{
			ArtifactContracts:    []askcontract.ArtifactContract{{Kind: "package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}, {Kind: "image", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}},
			SharedStateContracts: []askcontract.SharedStateContract{{Name: "join-file", ProducerPath: "workflows/scenarios/apply.yaml", ConsumerPaths: []string{"workflows/scenarios/apply.yaml"}, AvailabilityModel: "published-for-worker-consumption"}},
			RoleExecution:        askcontract.RoleExecutionModel{RoleSelector: "vars.role"},
			Verification:         askcontract.VerificationStrategy{FinalVerificationRole: "control-plane", ExpectedNodeCount: 3, ExpectedControlPlaneReady: 1},
		},
		NeedsPrepare: true,
	}
	critic := normalizePlanCritic(plan, askcontract.PlanCriticResponse{Blocking: []string{
		"workflows/scenarios/apply.yaml join handoff: the join artifact path is named, but the contract does not specify whether the published file contains a full reusable kubeadm join command",
		"workflows/vars.yaml + apply role logic: the plan says role selector is vars.topology.nodeRole, but it does not define how each running node resolves to one topology entry",
		"Execution model / workflows/scenarios/apply.yaml: Final CheckCluster on the control-plane can race worker joins because apply is described as per-node role-based execution with no barrier or wait contract",
	}})
	if hasFatalPlanReviewIssues(plan, critic) {
		t.Fatalf("expected recoverable gpt-5.4 blocker language to proceed to generation")
	}
}

func TestHasFatalPlanReviewIssuesBlocksStructuredExecutionContractGaps(t *testing.T) {
	plan := askcontract.PlanResponse{
		Intent:         "draft",
		AuthoringBrief: askcontract.AuthoringBrief{ModeIntent: "prepare+apply", Topology: "multi-node", RequiredCapabilities: []string{"kubeadm-bootstrap", "kubeadm-join", "cluster-verification"}},
		EntryScenario:  "workflows/scenarios/apply.yaml",
		Files:          []askcontract.PlanFile{{Path: "workflows/prepare.yaml"}, {Path: "workflows/scenarios/apply.yaml"}},
		ExecutionModel: askcontract.ExecutionModel{ArtifactContracts: []askcontract.ArtifactContract{{Kind: "package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}}, RoleExecution: askcontract.RoleExecutionModel{RoleSelector: "vars.role"}},
		NeedsPrepare:   true,
	}
	critic := askcontract.PlanCriticResponse{Findings: []askcontract.PlanCriticFinding{{Code: workflowissues.CodeAmbiguousJoinContract, Severity: workflowissues.SeverityMissingContract, Message: "join publication and consumption are still ambiguous"}, {Code: workflowissues.CodeWeakVerificationStaging, Severity: workflowissues.SeverityAdvisory, Message: "final verification can race worker joins"}}}
	if !hasFatalPlanReviewIssues(plan, critic) {
		t.Fatalf("expected structured execution-contract findings to block generation")
	}
}

func TestHasFatalPlanReviewIssuesAllowsRecoverableExecutionContractGapsForRefine(t *testing.T) {
	plan := askcontract.PlanResponse{
		Intent:         "refine",
		AuthoringBrief: askcontract.AuthoringBrief{RouteIntent: "refine", ModeIntent: "apply-only", Topology: "multi-node", NodeCount: 2, AnchorPaths: []string{"workflows/scenarios/apply.yaml"}},
		EntryScenario:  "workflows/scenarios/apply.yaml",
		Files:          []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml"}, {Path: "workflows/vars.yaml"}},
		ExecutionModel: askcontract.ExecutionModel{RoleExecution: askcontract.RoleExecutionModel{RoleSelector: "runtime.host.role", WorkerFlow: "join", PerNodeInvocation: true}},
	}
	critic := askcontract.PlanCriticResponse{Findings: []askcontract.PlanCriticFinding{
		{Code: workflowissues.CodeAmbiguousJoinContract, Severity: workflowissues.SeverityMissingContract, Message: "join publication and consumption are still ambiguous", Recoverable: true},
		{Code: workflowissues.CodeWeakVerificationStaging, Severity: workflowissues.SeverityAdvisory, Message: "final verification can race worker joins", Recoverable: true},
	}}
	if hasFatalPlanReviewIssues(plan, critic) {
		t.Fatalf("expected recoverable refine review findings to proceed to generation")
	}
}

func TestHasFatalPlanReviewIssuesAllowsSimpleSingleNodeVerificationPlan(t *testing.T) {
	plan := askcontract.PlanResponse{
		Intent:         "draft",
		AuthoringBrief: askcontract.AuthoringBrief{ModeIntent: "apply-only", Topology: "single-node", NodeCount: 1, RequiredCapabilities: []string{"cluster-verification"}},
		EntryScenario:  "workflows/scenarios/apply.yaml",
		Files:          []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml"}},
		ExecutionModel: askcontract.ExecutionModel{Verification: askcontract.VerificationStrategy{ExpectedNodeCount: 1, ExpectedControlPlaneReady: 1, FinalVerificationRole: "local"}},
	}
	critic := askcontract.PlanCriticResponse{Findings: []askcontract.PlanCriticFinding{{Code: workflowissues.CodeRoleCardinalityGap, Severity: workflowissues.SeverityMissingContract, Message: "single-node role cardinality is not explicit"}, {Code: workflowissues.CodeWeakVerificationStaging, Severity: workflowissues.SeverityAdvisory, Message: "single CheckCluster-only flow is weakly staged"}}}
	if hasFatalPlanReviewIssues(plan, critic) {
		t.Fatalf("expected simple single-node verification plan to continue past recoverable critic findings")
	}
}

func TestBuildPlanWithReviewFallsBackOnPlannerFailure(t *testing.T) {
	client := &stubClient{responses: []string{"not-json"}}
	prompt := "Create a minimal single-node apply-only kubeadm workflow using InitKubeadm and CheckCluster"
	req := askpolicy.BuildScenarioRequirements(prompt, askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft})
	plan, critic, usedFallback, err := buildPlanWithReview(context.Background(), client, askconfigSettings{provider: "openai", model: "gpt-5.4", apiKey: "test-key"}, askintent.Decision{Route: askintent.RouteDraft}, askretrieve.RetrievalResult{}, prompt, askretrieve.WorkspaceSummary{}, req, newAskLogger(io.Discard, "trace"))
	if err != nil {
		t.Fatalf("buildPlanWithReview fallback: %v", err)
	}
	if !usedFallback {
		t.Fatalf("expected fallback path")
	}
	if len(critic.Blocking) != 0 {
		t.Fatalf("expected empty plan critic on fallback, got %#v", critic)
	}
	if plan.ExecutionModel.RoleExecution.RoleSelector != "" {
		t.Fatalf("expected fallback execution model defaults, got %#v", plan.ExecutionModel)
	}
	if plan.ExecutionModel.Verification.ExpectedNodeCount != 1 || plan.ExecutionModel.Verification.FinalVerificationRole != "control-plane" {
		t.Fatalf("expected fallback single-node verification defaults, got %#v", plan.ExecutionModel)
	}
}

func TestBuildPlanWithReviewStopsEarlyWhenFallbackPlanIsNotViable(t *testing.T) {
	client := &stubClient{responses: []string{"not-json"}}
	prompt := "Create an offline RHEL 9 kubeadm workflow for exactly 2 nodes: 1 control-plane and 1 worker. Generate both workflows/prepare.yaml and workflows/scenarios/apply.yaml. In prepare, stage kubeadm kubelet kubectl cri-tools containerd packages and Kubernetes control-plane images using typed steps only. In apply, use vars.role with allowed values control-plane and worker, bootstrap the control-plane with InitKubeadm writing /tmp/deck/join.txt, join the worker with JoinKubeadm using that same file, and run final CheckCluster only on the control-plane expecting total 2 nodes and controlPlaneReady 1. Do not use remote downloads during apply."
	req := askpolicy.BuildScenarioRequirements(prompt, askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft})
	_, _, usedFallback, err := buildPlanWithReview(context.Background(), client, askconfigSettings{provider: "openai", model: "gpt-5.4", apiKey: "test-key"}, askintent.Decision{Route: askintent.RouteDraft}, askretrieve.RetrievalResult{}, prompt, askretrieve.WorkspaceSummary{}, req, newAskLogger(io.Discard, "trace"))
	if err == nil {
		t.Fatalf("expected invalid artifact fallback plan to stop early")
	}
	if usedFallback {
		t.Fatalf("expected invalid fallback plan to return an error, not succeed")
	}
	if !strings.Contains(err.Error(), "fallback plan is not viable") || !strings.Contains(err.Error(), "artifacts.packages") {
		t.Fatalf("expected early artifact viability error, got %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("expected planner failure to stop before critic/generation, got %d calls", client.calls)
	}
}

func TestBuildPlanWithReviewNormalizesRefineFallbackScope(t *testing.T) {
	client := &stubClient{responses: []string{"not-json"}}
	prompt := "refactor workflows/scenarios/apply.yaml to use workflows/vars.yaml for repeated local values"
	workspace := askretrieve.WorkspaceSummary{HasWorkflowTree: true, HasApply: true, Files: []askretrieve.WorkspaceFile{{Path: "workflows/scenarios/apply.yaml"}, {Path: "workflows/vars.yaml"}}}
	decision := askintent.Decision{Route: askintent.RouteRefine, Target: askintent.Target{Kind: "scenario", Path: "workflows/scenarios/apply.yaml"}}
	req := askpolicy.BuildScenarioRequirements(prompt, askretrieve.RetrievalResult{}, workspace, decision)
	plan, critic, usedFallback, err := buildPlanWithReview(context.Background(), client, askconfigSettings{provider: "openai", model: "gpt-5.4", apiKey: "test-key"}, decision, askretrieve.RetrievalResult{}, prompt, workspace, req, newAskLogger(io.Discard, "trace"))
	if err != nil {
		t.Fatalf("buildPlanWithReview refine fallback: %v", err)
	}
	if !usedFallback {
		t.Fatalf("expected refine fallback path")
	}
	if len(critic.Blocking) != 0 {
		t.Fatalf("expected empty plan critic on fallback, got %#v", critic)
	}
	if strings.Join(plan.AuthoringBrief.AnchorPaths, ",") != "workflows/scenarios/apply.yaml" {
		t.Fatalf("expected refine fallback anchor path, got %#v", plan.AuthoringBrief)
	}
	if !strings.Contains(strings.Join(plan.AuthoringBrief.AllowedCompanionPaths, ","), "workflows/vars.yaml") {
		t.Fatalf("expected refine fallback vars companion path, got %#v", plan.AuthoringBrief)
	}
	joinedTargets := strings.Join(plan.AuthoringBrief.TargetPaths, ",")
	if !strings.Contains(joinedTargets, "workflows/scenarios/apply.yaml") || !strings.Contains(joinedTargets, "workflows/vars.yaml") {
		t.Fatalf("expected refine fallback target paths, got %#v", plan.AuthoringBrief)
	}
}

func TestBuildPlanWithReviewRecoversPartialPlannerResponse(t *testing.T) {
	client := &stubClient{
		responses: []string{
			`{"version":1,"request":"Create an offline RHEL 9 kubeadm workflow for exactly 2 nodes: 1 control-plane and 1 worker","intent":"draft","authoringProgram":{"platform":{"family":"rhel","release":"9"},"artifacts":{"packages":["kubeadm","kubelet","kubectl","cri-tools","containerd"],"images":["registry.k8s.io/kube-apiserver:v1.30.0"]},"cluster":{"joinFile":"/tmp/deck/join.txt","roleSelector":"vars.role","controlPlaneCount":1,"workerCount":1},"verification":{"expectedNodeCount":2,"expectedReadyCount":2,"expectedControlPlaneReady":1}},"openQuestions":[{"question":"Which repository mirror should prepare use?"},{"text":"Should apply default to role-based execution?"}],"entryScenario":"Execute prepare first, then apply","files":[{"path":"workflows/prepare.yaml","kind":"workflow","action":"create","purpose":"prepare"},{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"apply"},{"path":"workflows/vars.yaml","kind":"vars","action":"create","purpose":"vars"}],"validationChecklist":["lint"]}`,
			`{"summary":"plan critic skipped","blocking":[],"advisory":[],"missingContracts":[],"suggestedFixes":[]}`,
		},
	}
	req := askpolicy.BuildScenarioRequirements("Create an offline RHEL 9 kubeadm workflow for exactly 2 nodes: 1 control-plane and 1 worker", askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft})
	plan, critic, usedFallback, err := buildPlanWithReview(context.Background(), client, askconfigSettings{provider: "openai", model: "gpt-5.4", apiKey: "test-key"}, askintent.Decision{Route: askintent.RouteDraft}, askretrieve.RetrievalResult{}, "Create an offline RHEL 9 kubeadm workflow for exactly 2 nodes: 1 control-plane and 1 worker", askretrieve.WorkspaceSummary{}, req, newAskLogger(io.Discard, "trace"))
	if err != nil {
		t.Fatalf("buildPlanWithReview recovery: %v", err)
	}
	if usedFallback {
		t.Fatalf("expected partial planner response to recover without fallback")
	}
	if plan.EntryScenario != "workflows/scenarios/apply.yaml" {
		t.Fatalf("expected normalized entry scenario, got %#v", plan.EntryScenario)
	}
	if len(plan.AuthoringProgram.Artifacts.Packages) != 5 || len(plan.AuthoringProgram.Artifacts.Images) != 1 {
		t.Fatalf("expected recovered artifact program, got %#v", plan.AuthoringProgram)
	}
	joinedQuestions := strings.Join(plan.OpenQuestions, "\n")
	for _, want := range []string{"Which repository mirror should prepare use?", "Should apply default to role-based execution?"} {
		if !strings.Contains(joinedQuestions, want) {
			t.Fatalf("expected malformed openQuestions objects to recover %q, got %#v", want, plan.OpenQuestions)
		}
	}
	if len(critic.Blocking) != 0 {
		t.Fatalf("expected non-blocking critic result, got %#v", critic)
	}
}

func TestNormalizeArtifactKindsDropsPlannerNoise(t *testing.T) {
	kinds := askpolicy.NormalizeArtifactKinds([]string{"workflow", "scenario", "image", "vars", "package"})
	if strings.Join(kinds, ",") != "image,package" {
		t.Fatalf("unexpected normalized artifact kinds: %v", kinds)
	}
}

func TestGenerateWithValidationRetriesParseFailure(t *testing.T) {
	enableLegacyAuthoringFallback(t)
	client := &stubClient{responses: []string{
		`not-json`,
		`{"summary":"ok","review":[],"files":[{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [\"true\"]\n"}]}`,
	}}
	_, files, _, _, _, retries, err := generateWithValidation(context.Background(), client, askprovider.Request{Kind: "generate", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key", Prompt: "generate"}, t.TempDir(), 2, newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteDraft}, askcontract.PlanResponse{}, askcontract.AuthoringBrief{}, askretrieve.RetrievalResult{}, askcontract.PlanCriticResponse{})
	if err != nil {
		t.Fatalf("expected parse retry success: %v", err)
	}
	if retries != 1 || len(files) != 1 {
		t.Fatalf("unexpected result: retries=%d files=%d", retries, len(files))
	}
}

func TestGenerateWithValidationRepairsSemanticFailure(t *testing.T) {
	enableLegacyAuthoringFallback(t)
	client := &stubClient{responses: []string{
		`{"summary":"missing vars","review":[],"files":[{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [\"true\"]\n"}]}`,
		`{"summary":"ok","review":[],"files":[{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [\"true\"]\n"},{"path":"workflows/vars.yaml","content":"{}\n"}]}`,
	}}
	plan := askcontract.PlanResponse{Files: []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "create"}, {Path: "workflows/vars.yaml", Action: "create"}}, ValidationChecklist: []string{"vars are defined"}}
	_, files, _, _, _, retries, err := generateWithValidation(context.Background(), client, askprovider.Request{Kind: "generate", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key", Prompt: "generate"}, t.TempDir(), 2, newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteDraft}, plan, askcontract.AuthoringBrief{}, askretrieve.RetrievalResult{}, askcontract.PlanCriticResponse{})
	if err != nil {
		t.Fatalf("expected semantic repair success: %v", err)
	}
	if retries != 1 || len(files) != 2 {
		t.Fatalf("unexpected result: retries=%d files=%d", retries, len(files))
	}
}

func TestGenerateWithValidationRepairsKubeadmStyleCheckHostFailure(t *testing.T) {
	enableLegacyAuthoringFallback(t)
	client := &stubClient{responses: []string{
		`{"summary":"invalid kubeadm draft","review":[],"files":[{"path":"workflows/prepare.yaml","content":"version: v1alpha1\nsteps:\n  - id: preflight\n    kind: CheckHost\n    spec:\n      os:\n        type: rhel\n        version: \"9\"\n"},{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [\"true\"]\n"}]}`,
		`{"summary":"repaired kubeadm draft","review":[],"files":[{"path":"workflows/prepare.yaml","content":"version: v1alpha1\nsteps:\n  - id: preflight\n    kind: CheckHost\n    spec:\n      checks: [os, arch, swap]\n      failFast: true\n"},{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\nsteps:\n  - id: bootstrap\n    kind: UpgradeKubeadm\n    spec:\n      kubernetesVersion: v1.31.0\n  - id: verify-cluster\n    kind: CheckCluster\n    spec:\n      interval: 5s\n      nodes:\n        total: 1\n        ready: 1\n        controlPlaneReady: 1\n"}]}`,
	}}
	plan := askcontract.PlanResponse{
		Request:             "create an air-gapped rhel9 single-node kubeadm workflow using typed steps where possible",
		TargetOutcome:       "Generate a prepare and apply workflow for kubeadm",
		Files:               []askcontract.PlanFile{{Path: "workflows/prepare.yaml", Action: "create"}, {Path: "workflows/scenarios/apply.yaml", Action: "create"}},
		ValidationChecklist: []string{"Typed steps should be used where applicable"},
	}
	_, files, _, _, _, retries, err := generateWithValidation(context.Background(), client, askprovider.Request{Kind: "generate", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key", Prompt: plan.Request}, t.TempDir(), 2, newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteDraft}, plan, askcontract.AuthoringBrief{}, askretrieve.RetrievalResult{}, askcontract.PlanCriticResponse{})
	if err != nil {
		t.Fatalf("expected kubeadm-style repair success: %v", err)
	}
	if retries != 1 || len(files) != 2 {
		t.Fatalf("unexpected result: retries=%d files=%d", retries, len(files))
	}
}

func TestGenerateWithValidationUsesAutomaticRepairBeforeModelRetry(t *testing.T) {
	enableLegacyAuthoringFallback(t)
	client := &stubClient{responses: []string{`{"summary":"invalid init draft","review":[],"files":[{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\nsteps:\n  - id: init\n    kind: InitKubeadm\n    spec:\n      podNetworkCIDR: 10.244.0.0/16\n"}]}`}}
	plan := askcontract.PlanResponse{Request: "create kubeadm workflow", Files: []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "create"}}, AuthoringBrief: askcontract.AuthoringBrief{RouteIntent: "draft", TargetScope: "scenario", TargetPaths: []string{"workflows/scenarios/apply.yaml"}, ModeIntent: "apply-only"}}
	_, files, _, _, _, retries, err := generateWithValidation(context.Background(), client, askprovider.Request{Kind: "generate-fast", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key", Prompt: plan.Request}, t.TempDir(), 2, newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteDraft}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{}, askcontract.PlanCriticResponse{})
	if err != nil {
		t.Fatalf("expected automatic repair success: %v", err)
	}
	if retries != 1 || client.calls != 1 {
		t.Fatalf("expected one generation call plus automatic repair, got retries=%d calls=%d", retries, client.calls)
	}
	if len(files) != 1 || !strings.Contains(files[0].Content, "outputJoinFile: /tmp/deck/join.txt") {
		t.Fatalf("expected repaired init output join file, got %#v", files)
	}
}

func TestGenerateWithValidationRetryPromptIncludesRawValidatorErrorAndRepairGuidance(t *testing.T) {
	enableLegacyAuthoringFallback(t)
	client := &stubClient{responses: []string{
		`{"summary":"invalid kubeadm draft","review":[],"files":[{"path":"workflows/prepare.yaml","content":"version: v1alpha1\nsteps:\n  - id: preflight\n    kind: CheckHost\n    spec:\n      os:\n        type: rhel\n        version: \"9\"\n"}]}`,
		`{"summary":"repaired kubeadm draft","review":[],"files":[{"path":"workflows/prepare.yaml","content":"version: v1alpha1\nsteps:\n  - id: preflight\n    kind: CheckHost\n    spec:\n      checks: [os, arch, swap]\n      failFast: true\n"},{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\nsteps:\n  - id: bootstrap\n    kind: UpgradeKubeadm\n    spec:\n      kubernetesVersion: v1.31.0\n  - id: verify-cluster\n    kind: CheckCluster\n    spec:\n      interval: 5s\n      nodes:\n        total: 1\n        ready: 1\n        controlPlaneReady: 1\n"}]}`,
	}}
	plan := askcontract.PlanResponse{
		Request:       "create an air-gapped rhel9 single-node kubeadm workflow using typed steps where possible",
		TargetOutcome: "Generate a prepare workflow for kubeadm",
		Files:         []askcontract.PlanFile{{Path: "workflows/prepare.yaml", Action: "create"}, {Path: "workflows/scenarios/apply.yaml", Action: "create"}},
	}
	_, _, _, _, _, _, err := generateWithValidation(context.Background(), client, askprovider.Request{Kind: "generate", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key", Prompt: plan.Request}, t.TempDir(), 2, newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteDraft}, plan, askcontract.AuthoringBrief{}, askretrieve.RetrievalResult{}, askcontract.PlanCriticResponse{Blocking: []string{"artifact contract is too vague"}, MissingContracts: []string{"package producer path contract"}})
	if err != nil {
		t.Fatalf("expected repair success: %v", err)
	}
	if len(client.prompts) != 2 {
		t.Fatalf("expected two generate calls, got %d", len(client.prompts))
	}
	retryPrompt := client.prompts[1].Prompt
	for _, want := range []string{"Structured validator findings:", "CheckHost", "spec.checks", "spec.os", "Structured diagnostics JSON:", "Suggested repair operations:", "fill-field", "remove-field", "Targeted repair mode:", "Affected files to revise first:", "path: workflows/prepare.yaml [revise]", "Preserve unchanged files when they are already valid", "package producer path contract"} {
		if !strings.Contains(retryPrompt, want) {
			t.Fatalf("expected %q in retry prompt, got %q", want, retryPrompt)
		}
	}
	for _, avoid := range []string{"Previously generated files:", "kind: CheckHost\n    spec:\n      os:", "version: v1alpha1\nsteps:"} {
		if strings.Contains(retryPrompt, avoid) {
			t.Fatalf("expected retry prompt to avoid raw previous file exemplar %q, got %q", avoid, retryPrompt)
		}
	}
}

func TestGenerateWithValidationRetryPromptIncludesDuplicateStepIDRepairGuidance(t *testing.T) {
	enableLegacyAuthoringFallback(t)
	client := &stubClient{responses: []string{
		`{"summary":"duplicate ids","review":[],"files":[{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\nphases:\n  - name: control-plane\n    steps:\n      - id: preflight-host\n        kind: CheckHost\n        spec:\n          checks: [os, arch, swap]\n  - name: worker\n    steps:\n      - id: preflight-host\n        kind: CheckHost\n        spec:\n          checks: [os, arch, swap]\n"}]}`,
		`{"summary":"repaired ids","review":[],"files":[{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\nphases:\n  - name: control-plane\n    steps:\n      - id: control-plane-preflight-host\n        kind: CheckHost\n        spec:\n          checks: [os, arch, swap]\n  - name: worker\n    steps:\n      - id: worker-preflight-host\n        kind: CheckHost\n        spec:\n          checks: [os, arch, swap]\n"}]}`,
	}}
	plan := askcontract.PlanResponse{Request: "create a multi-phase workflow", Files: []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "create"}}}
	_, _, _, _, _, _, err := generateWithValidation(context.Background(), client, askprovider.Request{Kind: "generate-fast", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key", Prompt: plan.Request}, t.TempDir(), 2, newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteDraft}, plan, askcontract.AuthoringBrief{}, askretrieve.RetrievalResult{}, askcontract.PlanCriticResponse{})
	if err != nil {
		t.Fatalf("expected duplicate-id repair success: %v", err)
	}
	if len(client.prompts) != 2 {
		t.Fatalf("expected two generate calls, got %d", len(client.prompts))
	}
	retryPrompt := client.prompts[1].Prompt
	for _, want := range []string{string(workflowissues.CodeDuplicateStepID), "Duplicate step id repair", "control-plane-preflight-host", "worker-preflight-host", "Return structured documents, not raw file payloads."} {
		if !strings.Contains(retryPrompt, want) {
			t.Fatalf("expected %q in duplicate-id retry prompt, got %q", want, retryPrompt)
		}
	}
}

func TestGenerateWithValidationRejectsLegacyFilesOnlyYamlRepairPayload(t *testing.T) {
	client := &stubClient{responses: []string{
		`{"summary":"broken yaml","review":[],"files":[{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\nphases:\n  - name: broken\n    steps:\n      - id: run\n        kind: Command\n        spec:\n          command: [\"true\"\n"},{"path":"workflows/vars.yaml","content":"role: control-plane\n"}]}`,
	}}
	plan := askcontract.PlanResponse{Request: "repair yaml structure", Files: []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "create"}, {Path: "workflows/vars.yaml", Action: "create"}}}
	_, _, _, _, _, _, err := generateWithValidation(context.Background(), client, askprovider.Request{Kind: "generate", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key", Prompt: plan.Request}, t.TempDir(), 2, newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteDraft}, plan, askcontract.AuthoringBrief{}, askretrieve.RetrievalResult{}, askcontract.PlanCriticResponse{})
	if err == nil || !strings.Contains(err.Error(), "documents") {
		t.Fatalf("expected legacy files-only payload to fail, got %v", err)
	}
	if len(client.prompts) != 1 {
		t.Fatalf("expected files-only payload to stop without repair, got %d prompts", len(client.prompts))
	}
}

func TestCurrentWorkspaceDocumentSummariesParsesWorkflowAndVars(t *testing.T) {
	workspace := askretrieve.WorkspaceSummary{Files: []askretrieve.WorkspaceFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [true]\n"}, {Path: "workflows/vars.yaml", Content: "role: control-plane\n"}}}
	summaries := currentWorkspaceDocumentSummaries(workspace)
	joined := strings.Join(summaries, "\n")
	for _, want := range []string{"workflows/scenarios/apply.yaml [workflow]", "workflows/vars.yaml [vars]"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in document summaries, got %q", want, joined)
		}
	}
}

func TestGenerateWithValidationMergesPartialValidationRepairResponse(t *testing.T) {
	enableLegacyAuthoringFallback(t)
	client := &stubClient{responses: []string{
		`{"summary":"draft","review":[],"files":[{"path":"workflows/prepare.yaml","content":"version: v1alpha1\nphases:\n  - name: collect\n    steps:\n      - id: collect\n        kind: DownloadPackage\n        spec:\n          packages: [kubeadm]\n          distro:\n            family: rhel\n            release: rhel9\n          repo:\n            type: rpm\n          outputDir: /tmp/bad\n"},{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\nsteps:\n  - id: init\n    kind: InitKubeadm\n    spec:\n      outputJoinFile: /tmp/join.sh\n  - id: verify\n    kind: CheckCluster\n    spec:\n      interval: 5s\n      nodes:\n        total: 3\n        ready: 1\n        controlPlaneReady: 1\n"}]}`,
		`{"summary":"patched prepare only","review":["repaired prepare output root"],"files":[{"path":"workflows/prepare.yaml","content":"version: v1alpha1\nphases:\n  - name: collect\n    steps:\n      - id: collect\n        kind: DownloadPackage\n        spec:\n          packages: [kubeadm]\n          distro:\n            family: rhel\n            release: rhel9\n          repo:\n            type: rpm\n          outputDir: packages/\n"}]}`,
	}}
	plan := askcontract.PlanResponse{
		Request:        "create an air-gapped 3-node kubeadm prepare and apply workflow",
		Files:          []askcontract.PlanFile{{Path: "workflows/prepare.yaml", Action: "create"}, {Path: "workflows/scenarios/apply.yaml", Action: "create"}},
		AuthoringBrief: askcontract.AuthoringBrief{RouteIntent: "draft", TargetScope: "workspace", ModeIntent: "prepare+apply", Topology: "multi-node", NodeCount: 3},
	}
	brief := plan.AuthoringBrief
	_, files, _, _, _, retries, err := generateWithValidation(context.Background(), client, askprovider.Request{Kind: "generate-fast", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key", Prompt: plan.Request}, t.TempDir(), 2, newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteDraft}, plan, brief, askretrieve.RetrievalResult{}, askcontract.PlanCriticResponse{})
	if err != nil {
		t.Fatalf("expected merged validation repair success: %v", err)
	}
	if retries != 1 || len(files) != 2 {
		t.Fatalf("expected merged result with preserved apply file, got retries=%d files=%d", retries, len(files))
	}
	joined := files[0].Content + "\n" + files[1].Content
	if !strings.Contains(joined, "outputDir: packages/") || !strings.Contains(joined, "kind: InitKubeadm") {
		t.Fatalf("expected prepare patch plus preserved apply file, got %#v", files)
	}
}

func TestGenerateWithValidationRepairsOnJudgeBlocking(t *testing.T) {
	enableLegacyAuthoringFallback(t)
	client := &stubClient{responses: []string{
		`{"summary":"draft one","review":[],"files":[{"path":"workflows/prepare.yaml","content":"version: v1alpha1\nsteps:\n  - id: collect\n    kind: DownloadPackage\n    spec:\n      packages: [kubeadm]\n      repo:\n        type: rpm\n      distro:\n        family: rhel\n        release: rocky9\n"},{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\nsteps:\n  - id: install\n    kind: InstallPackage\n    spec:\n      packages: [kubeadm]\n      source:\n        type: local-repo\n        path: /tmp/packages\n  - id: init\n    kind: InitKubeadm\n    spec:\n      outputJoinFile: /tmp/join.sh\n  - id: join\n    kind: JoinKubeadm\n    spec:\n      joinFile: /tmp/join.sh\n  - id: verify\n    kind: CheckCluster\n    spec:\n      interval: 5s\n      nodes:\n        total: 3\n        ready: 3\n        controlPlaneReady: 1\n"}]}`,
		`{"summary":"platform mismatch","blocking":["request asked for rhel9 but prepare uses rocky9"],"advisory":[],"missingCapabilities":[],"suggestedFixes":["Use an rhel9-compatible distro.release value instead of rocky9 in DownloadPackage"]}`,
		`{"summary":"draft two","review":[],"files":[{"path":"workflows/prepare.yaml","content":"version: v1alpha1\nsteps:\n  - id: collect\n    kind: DownloadPackage\n    spec:\n      packages: [kubeadm]\n      repo:\n        type: rpm\n      distro:\n        family: rhel\n        release: \"9\"\n"},{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\nsteps:\n  - id: install\n    kind: InstallPackage\n    spec:\n      packages: [kubeadm]\n      source:\n        type: local-repo\n        path: /tmp/packages\n  - id: init\n    kind: InitKubeadm\n    spec:\n      outputJoinFile: /tmp/join.sh\n  - id: join\n    kind: JoinKubeadm\n    spec:\n      joinFile: /tmp/join.sh\n  - id: verify\n    kind: CheckCluster\n    spec:\n      interval: 5s\n      nodes:\n        total: 3\n        ready: 3\n        controlPlaneReady: 1\n"}]}`,
		`{"summary":"looks good","blocking":[],"advisory":["prepare now matches the requested rhel9 platform"],"missingCapabilities":[],"suggestedFixes":[]}`,
	}}
	plan := askcontract.PlanResponse{Request: "create an air-gapped 3-node kubeadm prepare and apply workflow", AuthoringBrief: askcontract.AuthoringBrief{RouteIntent: "draft", TargetScope: "workspace", ModeIntent: "prepare+apply", Topology: "multi-node", NodeCount: 3, RequiredCapabilities: []string{"prepare-artifacts", "kubeadm-bootstrap", "kubeadm-join"}}}
	brief := plan.AuthoringBrief
	_, files, _, critic, _, retries, err := generateWithValidation(context.Background(), client, askprovider.Request{Kind: "generate", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key", Prompt: plan.Request}, t.TempDir(), 2, newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteDraft}, plan, brief, askretrieve.RetrievalResult{}, askcontract.PlanCriticResponse{})
	if err != nil {
		t.Fatalf("expected judge-driven repair success: %v", err)
	}
	if retries != 1 {
		t.Fatalf("expected one retry, got %d", retries)
	}
	if len(files) != 2 || client.calls != 4 {
		t.Fatalf("unexpected result files=%d calls=%d", len(files), client.calls)
	}
	joined := strings.Join(critic.Advisory, "\n")
	if !strings.Contains(joined, "prepare now matches the requested rhel9 platform") {
		t.Fatalf("expected judge advisory to be preserved, got %#v", critic)
	}
	if len(client.prompts) < 3 || !strings.Contains(client.prompts[2].Prompt, "semantic judge requested revision") {
		t.Fatalf("expected retry prompt to include judge revision request, got %#v", client.prompts)
	}
}

func TestGenerateWithValidationKeepsFinalJudgeBlockingAsAdvisory(t *testing.T) {
	enableLegacyAuthoringFallback(t)
	client := &stubClient{responses: []string{
		`{"summary":"draft","review":[],"files":[{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\nsteps:\n  - id: init\n    kind: InitKubeadm\n    spec:\n      outputJoinFile: /tmp/join.sh\n  - id: verify\n    kind: CheckCluster\n    spec:\n      interval: 5s\n      nodes:\n        total: 3\n        ready: 1\n        controlPlaneReady: 1\n"}]}`,
		`{"summary":"still thin","blocking":["requested worker join behavior is still missing"],"advisory":[],"missingCapabilities":["kubeadm-join"],"suggestedFixes":["Add worker join steps"]}`,
	}}
	plan := askcontract.PlanResponse{Request: "create an air-gapped 3-node kubeadm workflow", AuthoringBrief: askcontract.AuthoringBrief{RouteIntent: "draft", TargetScope: "workspace", ModeIntent: "apply-only", Topology: "multi-node", NodeCount: 3, RequiredCapabilities: []string{"kubeadm-bootstrap", "kubeadm-join"}}}
	_, _, _, critic, _, retries, err := generateWithValidation(context.Background(), client, askprovider.Request{Kind: "generate", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key", Prompt: plan.Request}, t.TempDir(), 1, newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteDraft}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{}, askcontract.PlanCriticResponse{})
	if err != nil {
		t.Fatalf("expected final judge blocking to stay advisory, got %v", err)
	}
	if retries != 0 {
		t.Fatalf("expected no retries, got %d", retries)
	}
	joined := strings.Join(critic.Advisory, "\n")
	for _, want := range []string{"requested worker join behavior is still missing", "kubeadm-join"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in final advisory, got %#v", want, critic)
		}
	}
}

func TestGenerateWithValidationSucceedsForSpecificOfflineTwoNodePrompt(t *testing.T) {
	enableLegacyAuthoringFallback(t)
	prompt := "Create an offline RHEL 9 kubeadm workflow for exactly 2 nodes: 1 control-plane and 1 worker. Generate both workflows/prepare.yaml and workflows/scenarios/apply.yaml. In prepare, stage kubeadm kubelet kubectl cri-tools containerd packages and Kubernetes control-plane images using typed steps only. In apply, use vars.role with allowed values control-plane and worker, bootstrap the control-plane with InitKubeadm writing /tmp/deck/join.txt, join the worker with JoinKubeadm using that same file, and run final CheckCluster only on the control-plane expecting total 2 nodes and controlPlaneReady 1. Do not use remote downloads during apply. Use workflows/vars.yaml if values repeat, and use workflows/components/ only if needed."
	client := &stubClient{responses: []string{`{"summary":"specific two-node draft","review":[],"files":[{"path":"workflows/prepare.yaml","content":"version: v1alpha1\nsteps:\n  - id: prepare-download-kubernetes-packages\n    kind: DownloadPackage\n    spec:\n      packages: [kubeadm, kubelet, kubectl, cri-tools, containerd]\n      distro:\n        family: rhel\n        release: \"9\"\n      repo:\n        type: rpm\n        generate: true\n      backend:\n        mode: container\n        runtime: auto\n        image: rockylinux:9\n      outputDir: packages/rpm/rhel9\n  - id: prepare-download-control-plane-images\n    kind: DownloadImage\n    spec:\n      images: [registry.k8s.io/kube-apiserver:v1.30.0, registry.k8s.io/kube-controller-manager:v1.30.0, registry.k8s.io/kube-scheduler:v1.30.0, registry.k8s.io/etcd:3.5.12-0, registry.k8s.io/pause:3.9]\n      outputDir: images/control-plane\n"},{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\nphases:\n  - name: install-packages\n    steps:\n      - id: apply-install-kubernetes-packages\n        kind: InstallPackage\n        spec:\n          packages: [kubeadm, kubelet, kubectl, cri-tools, containerd]\n          source:\n            type: local-repo\n            path: packages/rpm/rhel9\n  - name: bootstrap\n    steps:\n      - id: apply-init-control-plane\n        when: vars.role == \"control-plane\"\n        kind: InitKubeadm\n        spec:\n          outputJoinFile: \"{{ .vars.joinFile }}\"\n          podNetworkCIDR: \"{{ .vars.podCIDR }}\"\n  - name: join\n    steps:\n      - id: apply-join-worker\n        when: vars.role == \"worker\"\n        kind: JoinKubeadm\n        spec:\n          joinFile: \"{{ .vars.joinFile }}\"\n  - name: verify\n    steps:\n      - id: apply-verify-cluster\n        when: vars.role == \"control-plane\"\n        kind: CheckCluster\n        spec:\n          timeout: 10m\n          interval: 5s\n          nodes:\n            total: 2\n            ready: 2\n            controlPlaneReady: 1\n          reports:\n            nodesPath: /tmp/deck/reports/two-node-cluster.txt\n"},{"path":"workflows/vars.yaml","content":"clusterName: two-node-offline\nrole: control-plane\njoinFile: /tmp/deck/join.txt\npodCIDR: 10.244.0.0/16\n"}]}`}}
	plan := askcontract.PlanResponse{
		Request:       prompt,
		NeedsPrepare:  true,
		ArtifactKinds: []string{"package", "image"},
		AuthoringBrief: askcontract.AuthoringBrief{
			RouteIntent:          "draft offline two-node kubeadm prepare+apply workflows",
			TargetScope:          "workspace",
			ModeIntent:           "prepare+apply",
			Connectivity:         "offline",
			CompletenessTarget:   "complete",
			Topology:             "multi-node",
			NodeCount:            2,
			RequiredCapabilities: []string{"prepare-artifacts", "package-staging", "image-staging", "kubeadm-bootstrap", "kubeadm-join", "cluster-verification"},
		},
		ExecutionModel: askcontract.ExecutionModel{
			ArtifactContracts:    []askcontract.ArtifactContract{{Kind: "package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}, {Kind: "image", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}},
			SharedStateContracts: []askcontract.SharedStateContract{{Name: "join-file", ProducerPath: "/tmp/deck/join.txt", ConsumerPaths: []string{"/tmp/deck/join.txt"}, AvailabilityModel: "local-only"}},
			RoleExecution:        askcontract.RoleExecutionModel{RoleSelector: "vars.role", PerNodeInvocation: true, ControlPlaneFlow: "install -> bootstrap -> verify", WorkerFlow: "install -> join"},
			Verification:         askcontract.VerificationStrategy{FinalVerificationRole: "control-plane", ExpectedNodeCount: 2, ExpectedControlPlaneReady: 1},
			ApplyAssumptions:     []string{"apply consumes local artifacts and avoids remote downloads"},
		},
	}
	_, files, _, critic, _, retries, err := generateWithValidation(context.Background(), client, askprovider.Request{Kind: "generate", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key", Prompt: prompt}, t.TempDir(), 2, newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteDraft}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{}, askcontract.PlanCriticResponse{})
	if err != nil {
		t.Fatalf("generateWithValidation: %v", err)
	}
	if retries != 0 {
		t.Fatalf("expected first-pass success for specific prompt, got %d retries", retries)
	}
	if len(critic.Blocking) != 0 {
		t.Fatalf("expected no blocking critic issues, got %#v", critic)
	}
	if len(files) != 3 {
		t.Fatalf("expected prepare/apply/vars files, got %#v", files)
	}
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}
	for _, want := range []string{"workflows/prepare.yaml", "workflows/scenarios/apply.yaml", "workflows/vars.yaml"} {
		if _, ok := byPath[want]; !ok {
			t.Fatalf("expected generated file %q, got %#v", want, byPath)
		}
	}
	if !strings.Contains(byPath["workflows/prepare.yaml"], "kind: DownloadPackage") || !strings.Contains(byPath["workflows/prepare.yaml"], "kind: DownloadImage") {
		t.Fatalf("expected typed prepare staging steps, got %q", byPath["workflows/prepare.yaml"])
	}
	if !strings.Contains(byPath["workflows/scenarios/apply.yaml"], "kind: CheckCluster") || !strings.Contains(byPath["workflows/scenarios/apply.yaml"], "total: 2") {
		t.Fatalf("expected explicit final verification contract, got %q", byPath["workflows/scenarios/apply.yaml"])
	}
}

func TestSemanticCriticUsesExecutionModelForRoleAndJoinContracts(t *testing.T) {
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nphases:\n  - name: bootstrap\n    steps:\n      - id: init\n        kind: InitKubeadm\n        spec:\n          outputJoinFile: /tmp/join.sh\n"}})
	plan := askcontract.PlanResponse{
		Request:        "create 3-node kubeadm workflow",
		AuthoringBrief: askcontract.AuthoringBrief{RouteIntent: "draft", TargetScope: "workspace", ModeIntent: "prepare+apply", Topology: "multi-node", NodeCount: 3},
		ExecutionModel: askcontract.ExecutionModel{
			ArtifactContracts:    []askcontract.ArtifactContract{{Kind: "package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}},
			SharedStateContracts: []askcontract.SharedStateContract{{Name: "join-file", ProducerPath: "/tmp/deck/join.txt", ConsumerPaths: []string{"/tmp/deck/join.txt"}, AvailabilityModel: "published-for-worker-consumption"}},
			RoleExecution:        askcontract.RoleExecutionModel{RoleSelector: "vars.role", PerNodeInvocation: true},
			Verification:         askcontract.VerificationStrategy{ExpectedNodeCount: 3, ExpectedControlPlaneReady: 1},
		},
	}
	critic := semanticCritic(gen, askintent.Decision{Route: askintent.RouteDraft}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{})
	joinedBlocking := strings.Join(critic.Blocking, "\n")
	if !strings.Contains(joinedBlocking, "artifact producer required by execution model") {
		t.Fatalf("expected artifact producer failure to stay blocking, got %#v", critic)
	}
	joinedAdvisory := strings.Join(critic.Advisory, "\n")
	for _, want := range []string{"role-aware per-node invocation via vars.role"} {
		if !strings.Contains(joinedAdvisory, want) {
			t.Fatalf("expected %q in execution-model critic output, got %#v", want, critic)
		}
	}
}

func TestSemanticCriticBlocksWorkerOnlyFinalVerificationAndMissingJoinPublish(t *testing.T) {
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/prepare.yaml", Content: "version: v1alpha1\nsteps:\n  - id: collect\n    kind: DownloadPackage\n    spec:\n      packages: [kubeadm]\n      distro:\n        family: rhel\n        release: \"9\"\n      repo:\n        type: rpm\n      outputDir: /tmp/packages\n"}, {Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nphases:\n  - name: bootstrap\n    steps:\n      - id: init\n        when: .vars.role == \"control-plane\"\n        kind: InitKubeadm\n        spec:\n          outputJoinFile: /tmp/deck/join.txt\n  - name: join\n    steps:\n      - id: worker-join\n        when: .vars.role == \"worker\"\n        kind: JoinKubeadm\n        spec:\n          joinFile: /tmp/deck/join.txt\n  - name: verify\n    steps:\n      - id: verify-final\n        when: .vars.role == \"worker\"\n        kind: CheckCluster\n        spec:\n          interval: 5s\n          nodes:\n            total: 3\n            ready: 3\n            controlPlaneReady: 3\n"}})
	plan := askcontract.PlanResponse{Request: "create 3-node kubeadm workflow", AuthoringBrief: askcontract.AuthoringBrief{RouteIntent: "draft", TargetScope: "workspace", ModeIntent: "prepare+apply", Topology: "multi-node", NodeCount: 3}, ExecutionModel: askcontract.ExecutionModel{ArtifactContracts: []askcontract.ArtifactContract{{Kind: "package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}}, SharedStateContracts: []askcontract.SharedStateContract{{Name: "join-file", ProducerPath: "/tmp/deck/server-root/files/cluster/join.txt", ConsumerPaths: []string{"/tmp/deck/server-root/files/cluster/join.txt"}, AvailabilityModel: "published-for-worker-consumption"}}, RoleExecution: askcontract.RoleExecutionModel{RoleSelector: "vars.role", PerNodeInvocation: true}, Verification: askcontract.VerificationStrategy{FinalVerificationRole: "control-plane", ExpectedNodeCount: 3, ExpectedControlPlaneReady: 1}}}
	critic := semanticCritic(gen, askintent.Decision{Route: askintent.RouteDraft}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{})
	if len(critic.Blocking) != 0 {
		t.Fatalf("expected recoverable design issues to stay advisory, got %#v", critic)
	}
	joined := strings.Join(critic.Advisory, "\n")
	for _, want := range []string{"published shared-state availability", "expected control-plane role", "expected nodes=3 controlPlaneReady=1"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in execution-model critic output, got %#v", want, critic)
		}
	}
}

func TestMaybePostProcessGenerationAppliesOperationalRepairOnly(t *testing.T) {
	client := &stubClient{responses: []string{
		`{"summary":"final verification should be gated to control-plane and inline structure is acceptable","blocking":["final cluster verification should run only on the control-plane role for this draft"],"advisory":["preserve inline structure for now"],"upgradeCandidates":["preserve-inline"],"reviseFiles":["workflows/scenarios/apply.yaml"],"preserveFiles":["workflows/prepare.yaml","workflows/vars.yaml"],"suggestedFixes":["Gate the final CheckCluster step with .vars.role == \"control-plane\""]}`,
		`{"summary":"post-processed draft","review":[],"files":[{"path":"workflows/prepare.yaml","content":"version: v1alpha1\nphases:\n  - name: collect\n    steps:\n      - id: packages\n        kind: DownloadPackage\n        spec:\n          packages: [kubeadm]\n          distro:\n            family: rhel\n            release: \"9\"\n          repo:\n            type: rpm\n          outputDir: packages/kubernetes\n"},{"path":"workflows/scenarios/apply.yaml","content":"version: v1alpha1\nphases:\n  - name: bootstrap\n    steps:\n      - id: init\n        when: .vars.role == \"control-plane\"\n        kind: InitKubeadm\n        spec:\n          outputJoinFile: /tmp/deck/join.txt\n      - id: join\n        when: .vars.role == \"worker\"\n        kind: JoinKubeadm\n        spec:\n          joinFile: /tmp/deck/join.txt\n      - id: verify\n        when: .vars.role == \"control-plane\"\n        kind: CheckCluster\n        spec:\n          interval: 5s\n          nodes:\n            total: 3\n            ready: 3\n            controlPlaneReady: 1\n"},{"path":"workflows/vars.yaml","content":"role: control-plane\n"}]}`,
		`{"summary":"post-processed workflow now has control-plane scoped final verification","blocking":[],"advisory":["inline structure is fine for now"],"missingCapabilities":[],"suggestedFixes":[]}`,
	}}
	plan := askcontract.PlanResponse{Request: "create 3-node kubeadm workflow", AuthoringBrief: askcontract.AuthoringBrief{ModeIntent: "prepare+apply", CompletenessTarget: "complete", Topology: "multi-node"}, ExecutionModel: askcontract.ExecutionModel{ArtifactContracts: []askcontract.ArtifactContract{{Kind: "package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}}, SharedStateContracts: []askcontract.SharedStateContract{{Name: "join-file", ProducerPath: "/tmp/deck/join.txt", ConsumerPaths: []string{"/tmp/deck/join.txt"}, AvailabilityModel: "local-only"}}, RoleExecution: askcontract.RoleExecutionModel{RoleSelector: "vars.role", PerNodeInvocation: true}, Verification: askcontract.VerificationStrategy{ExpectedNodeCount: 3, ExpectedControlPlaneReady: 1}}}
	brief := plan.AuthoringBrief
	gen := testMaterialized("draft", []askcontract.GeneratedFile{{Path: "workflows/prepare.yaml", Content: "version: v1alpha1\nphases:\n  - name: collect\n    steps:\n      - id: packages\n        kind: DownloadPackage\n        spec:\n          packages: [kubeadm]\n          distro:\n            family: rhel\n            release: \"9\"\n          repo:\n            type: rpm\n          outputDir: packages/kubernetes\n"}, {Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nphases:\n  - name: bootstrap\n    steps:\n      - id: init\n        when: .vars.role == \"control-plane\"\n        kind: InitKubeadm\n        spec:\n          outputJoinFile: /tmp/deck/join.txt\n      - id: join\n        when: .vars.role == \"worker\"\n        kind: JoinKubeadm\n        spec:\n          joinFile: /tmp/deck/join.txt\n      - id: verify\n        kind: CheckCluster\n        spec:\n          interval: 5s\n          nodes:\n            total: 3\n            ready: 3\n            controlPlaneReady: 1\n"}, {Path: "workflows/vars.yaml", Content: "role: control-plane\n"}})
	judge := askcontract.JudgeResponse{Summary: "usable but final verification should not run on workers", Advisory: []string{"final verification placement should be control-plane only"}}
	summary, err := maybePostProcessGeneration(context.Background(), client, askprovider.Request{Kind: "generate", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key"}, t.TempDir(), newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteDraft}, plan, brief, askretrieve.RetrievalResult{}, gen, gen.Files, "lint ok (1 workflows)", askcontract.CriticResponse{}, judge, askcontract.PlanCriticResponse{Advisory: []string{"shared-state publish path should stay explicit"}})
	if err != nil {
		t.Fatalf("maybePostProcessGeneration: %v", err)
	}
	if !summary.Applied {
		t.Fatalf("expected post-processing edit to apply")
	}
	if !strings.Contains(strings.Join(summary.Notes, "\n"), "preserve-inline") {
		t.Fatalf("expected preserve-inline advisory in notes, got %#v", summary.Notes)
	}
	if !strings.Contains(summary.Files[1].Content, "when: .vars.role == \"control-plane\"") {
		t.Fatalf("expected final verification to be gated after post-process, got %q", summary.Files[1].Content)
	}
}

func TestMaybePostProcessGenerationSkipsStructuralCleanupOnlyAdvice(t *testing.T) {
	client := &stubClient{responses: []string{
		`{"summary":"draft is operationally sound; only optional cleanup remains","blocking":[],"advisory":["extract-vars"],"upgradeCandidates":["extract-vars","preserve-inline"],"reviseFiles":[],"preserveFiles":["workflows/prepare.yaml","workflows/scenarios/apply.yaml","workflows/vars.yaml"],"suggestedFixes":[]}`,
	}}
	plan := askcontract.PlanResponse{Request: "create 3-node kubeadm workflow", AuthoringBrief: askcontract.AuthoringBrief{ModeIntent: "prepare+apply", CompletenessTarget: "complete", Topology: "multi-node"}}
	gen := testMaterialized("draft", []askcontract.GeneratedFile{{Path: "workflows/prepare.yaml", Content: "version: v1alpha1\nsteps:\n  - id: collect\n    kind: DownloadPackage\n    spec:\n      packages: [kubeadm]\n      distro:\n        family: rhel\n        release: \"9\"\n      repo:\n        type: rpm\n"}, {Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: verify\n    when: .vars.role == \"control-plane\"\n    kind: CheckCluster\n    spec:\n      interval: 5s\n      nodes:\n        total: 3\n        ready: 3\n        controlPlaneReady: 1\n"}})
	judge := askcontract.JudgeResponse{Summary: "mostly good", Advisory: []string{"worker join and verification are acceptable"}}
	summary, err := maybePostProcessGeneration(context.Background(), client, askprovider.Request{Kind: "generate", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key"}, t.TempDir(), newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteDraft}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{}, gen, gen.Files, "lint ok (1 workflows)", askcontract.CriticResponse{}, judge, askcontract.PlanCriticResponse{})
	if err != nil {
		t.Fatalf("maybePostProcessGeneration skip structural: %v", err)
	}
	if summary.Applied {
		t.Fatalf("expected no post-process edit for structural-only advice")
	}
	if !strings.Contains(strings.Join(summary.Notes, "\n"), "extract-vars") {
		t.Fatalf("expected advisory note without cleanup edit, got %#v", summary.Notes)
	}
}

func TestShouldAutoPostProcessStillRunsForPrepareApplyOperationalReview(t *testing.T) {
	plan := askcontract.PlanResponse{AuthoringBrief: askcontract.AuthoringBrief{ModeIntent: "prepare+apply", CompletenessTarget: "complete"}}
	files := []askcontract.GeneratedFile{{Path: "workflows/prepare.yaml", Content: "version: v1alpha1\nsteps: []\n"}, {Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps: []\n"}}
	if !shouldAutoPostProcess(plan, plan.AuthoringBrief, askcontract.CriticResponse{}, askcontract.JudgeResponse{}, files) {
		t.Fatalf("expected prepare+apply drafts to keep the operational post-process review")
	}
}

func TestStructuralWorkflowSummaryIncludesWhenConditions(t *testing.T) {
	doc := askcontract.WorkflowDocument{
		Steps: []askcontract.WorkflowStep{{ID: "verify", Kind: "CheckCluster", When: "vars.role == \"control-plane\"", Spec: map[string]any{"nodes": map[string]any{"total": 1}}}},
	}
	summary := structuralWorkflowSummary(doc)
	for _, want := range []string{"CheckCluster", "vars.role == \"control-plane\""} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in summary, got %q", want, summary)
		}
	}
}

func TestEnrichPostProcessFindingsPreservesRenderedFilesWithoutCleanupHints(t *testing.T) {
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/prepare.yaml", Content: "version: v1alpha1\nsteps:\n  - id: fetch\n    kind: DownloadPackage\n    spec:\n      outputDir: /srv/offline/kubernetes\n"}, {Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: install\n    kind: InstallPackage\n    spec:\n      source:\n        type: local-repo\n        path: /srv/offline/kubernetes\n"}})
	findings := enrichPostProcessFindings(askcontract.PostProcessResponse{}, gen.Files)
	if len(findings.Advisory) != 0 || len(findings.UpgradeCandidates) != 0 {
		t.Fatalf("expected no structural cleanup hints, got %#v", findings)
	}
	for _, path := range []string{"workflows/prepare.yaml", "workflows/scenarios/apply.yaml"} {
		if !containsTrimmed(findings.PreserveFiles, path) {
			t.Fatalf("expected %s to remain preserved, got %#v", path, findings)
		}
	}
}

func TestSummarizeValidationErrorHighlightsWorkflowSkeletonFixes(t *testing.T) {
	summary := summarizeValidationError("E_SCHEMA_INVALID: (root): version is required; steps.0: id is required; steps.1: id is required")
	for _, want := range []string{"Schema validation failure", "version: v1alpha1", "id` field"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in summary, got %q", want, summary)
		}
	}
}

func TestSummarizeValidationErrorRejectsPhaseIDs(t *testing.T) {
	summary := summarizeValidationError("E_SCHEMA_INVALID: (root): version is required; phases.0: Additional property id is not allowed; phases.1: Additional property id is not allowed")
	for _, want := range []string{"Remove `id` from phases", "Phase objects support `name`, `steps`, `imports`, and optional `maxParallelism` only", "version: v1alpha1"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in summary, got %q", want, summary)
		}
	}
}

func TestSummarizeValidationErrorHighlightsYAMLShapeFixes(t *testing.T) {
	summary := summarizeValidationError("parse yaml: yaml: line 10: did not find expected node content")
	for _, want := range []string{"YAML parse failure", "template", "valid YAML structure"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in summary, got %q", want, summary)
		}
	}
}

func TestLocalExplainDescribesScenarioStructure(t *testing.T) {
	workspace := askretrieve.WorkspaceSummary{
		Files: []askretrieve.WorkspaceFile{
			{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nphases:\n  - name: bootstrap\n    imports:\n      - path: bootstrap.yaml\n  - name: verify\n    steps:\n      - id: report\n        kind: Command\n        spec:\n          command: [bash, -lc, \"true\"]\n"},
			{Path: "workflows/components/bootstrap.yaml", Content: "steps:\n  - id: step-one\n    kind: InitKubeadm\n    spec:\n"},
		},
	}
	summary, answer := localExplain(workspace, "explain apply", askintent.Target{Kind: "scenario", Path: "workflows/scenarios/apply.yaml", Name: "apply"})
	if summary == "" {
		t.Fatalf("expected explain summary")
	}
	for _, want := range []string{"version \"v1alpha1\"", "bootstrap, verify", "bootstrap.yaml", "Command x1", "Related component available: workflows/components/bootstrap.yaml"} {
		if !strings.Contains(answer, want) {
			t.Fatalf("expected %q in answer, got %q", want, answer)
		}
	}
}

func TestLocalExplainUsesRepoBehaviorFallbackForAssemblyQuestion(t *testing.T) {
	summary, answer := localExplain(askretrieve.WorkspaceSummary{}, "Explain how InitKubeadm and CheckCluster are assembled for ask draft generation in this repo", askintent.Target{Kind: "workspace"})
	if summary != "Repository explanation" {
		t.Fatalf("expected repository explanation summary, got %q", summary)
	}
	for _, want := range []string{"internal/stepmeta/registry.go", "internal/stepspec/*_meta.go", "internal/askdraft/draft.go", "CompileWithProgram"} {
		if !strings.Contains(answer, want) {
			t.Fatalf("expected %q in repo-behavior fallback, got %q", want, answer)
		}
	}
}

func TestAskLoggerDebugAndTrace(t *testing.T) {
	var buf flushBuffer
	root := t.TempDir()
	logger := newAskLogger(&buf, "trace", root)
	logger.debug("command", "command", `deck ask "explain apply"`)
	logger.prompt("explain", "system text", "user text")
	logger.response("explain", `{"summary":"ok"}`)
	logText := buf.String()
	for _, want := range []string{"component=ask event=command", `command="deck ask \"explain apply\""`, "event=prompt", `content="system text"`, `content="user text"`, "event=response", `{\"summary\":\"ok\"}`, "path=.deck/ask/runs/"} {
		if !strings.Contains(logText, want) {
			t.Fatalf("expected %q in log output, got %q", want, logText)
		}
	}
	entries, err := filepath.Glob(filepath.Join(root, ".deck", "ask", "runs", "*", "prompts", "*.txt"))
	if err != nil {
		t.Fatalf("glob prompt artifacts: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 prompt artifacts, got %d", len(entries))
	}
	responses, err := filepath.Glob(filepath.Join(root, ".deck", "ask", "runs", "*", "responses", "*.json"))
	if err != nil {
		t.Fatalf("glob response artifacts: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response artifact, got %d", len(responses))
	}
	if buf.flushes == 0 {
		t.Fatalf("expected logger to flush output")
	}
}

func TestGenerationSystemPromptIncludesAskContextBlocks(t *testing.T) {
	req := askpolicy.ScenarioRequirements{Connectivity: "offline", RequiredFiles: []string{"workflows/scenarios/apply.yaml"}}
	scaffold := askscaffold.Build(req, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft}, askcontract.PlanResponse{}, askknowledge.Current())
	retrieval := askretrieve.RetrievalResult{Chunks: []askretrieve.Chunk{{ID: "facts-1", Source: "local-facts", Label: "source-of-truth-stepmeta", Topic: "local-facts:stepmeta", Content: "Local facts:\n- path: internal/stepmeta/registry.go", Score: 91}, {ID: "example-1", Source: "example", Label: "test/workflows/scenarios/kubeadm.yaml", Content: "Reference example:\n- path: test/workflows/scenarios/kubeadm.yaml\nversion: v1alpha1\nsteps:\n  - id: init\n    kind: InitKubeadm\n", Score: 90}, {ID: "workspace-apply", Source: "workspace", Label: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: join\n    kind: JoinKubeadm\n", Score: 80}, {ID: "mcp-1", Source: "mcp", Label: "web-search:kubernetes.io", Topic: "mcp:web-search:kubernetes.io", Content: "Typed MCP evidence JSON:\n{}", Score: 75, Evidence: &askretrieve.EvidenceSummary{Title: "Installing kubeadm", Domain: "kubernetes.io", DomainCategory: "official-docs", Freshness: "external-docs", Official: true, TrustLevel: "high"}}, {ID: "typed-steps-draft", Source: "askcontext", Topic: askcontext.TopicTypedSteps, Label: "typed-steps", Content: "typed guidance", Score: 70}}}
	prompt := generationSystemPrompt(askintent.RouteDraft, askintent.Target{Kind: "workspace"}, "create an air-gapped 3-node kubeadm workflow", retrieval, req, askcontract.PlanResponse{}, askcontract.AuthoringBrief{ModeIntent: "prepare+apply", Connectivity: "offline", CompletenessTarget: "starter", Topology: "multi-node", RequiredCapabilities: []string{"kubeadm-join", "cluster-verification"}}, askcontract.ExecutionModel{ArtifactContracts: []askcontract.ArtifactContract{{Kind: "package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml", Description: "offline package flow"}}, SharedStateContracts: []askcontract.SharedStateContract{{Name: "join-file", ProducerPath: "/tmp/deck/join.txt", ConsumerPaths: []string{"/tmp/deck/join.txt"}, AvailabilityModel: "published-for-worker-consumption"}}, RoleExecution: askcontract.RoleExecutionModel{RoleSelector: "vars.role", ControlPlaneFlow: "bootstrap", WorkerFlow: "join", PerNodeInvocation: true}, Verification: askcontract.VerificationStrategy{FinalVerificationRole: "control-plane", ExpectedNodeCount: 3, ExpectedControlPlaneReady: 1}, ApplyAssumptions: []string{"apply consumes local artifacts"}}, scaffold)
	for _, want := range []string{"Workflow source-of-truth:", "Authoring policy from deck metadata:", "Validated scaffold:", "Return structured workflow documents, not final YAML text.", "JSON shape: {\"summary\":string,\"review\":[]string,\"selection\":", "Evidence boundaries:", "Local facts are authoritative for deck workflow validity", "External evidence is only for upstream product behavior", "Do not let external docs override local deck workflow truth", "external source: Installing kubeadm [domain=kubernetes.io, category=official-docs, freshness=external-docs, official=true, trust=high]"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in generation prompt, got %q", want, prompt)
		}
	}
	if !strings.Contains(prompt, "Primary repository context follows.") || !strings.Contains(prompt, "Reference example:") || !strings.Contains(prompt, "JoinKubeadm") {
		t.Fatalf("expected repository context to appear in generation prompt, got %q", prompt)
	}
	if strings.Index(prompt, "Primary repository context follows.") > strings.Index(prompt, "Workflow source-of-truth:") {
		t.Fatalf("expected repository context before abstract policy blocks, got %q", prompt)
	}
	for _, want := range []string{"Normalized authoring brief:", "mode intent: prepare+apply", "connectivity: offline", "completeness target: starter", "Normalized execution model:", "artifact package: workflows/prepare.yaml -> workflows/scenarios/apply.yaml", "shared state join-file:", "role selector: vars.role", "verification expected nodes: 3", "verification final role: control-plane", "Keep document structure schema-focused:"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in generation prompt, got %q", want, prompt)
		}
	}
	for _, avoid := range []string{"Workspace topology:", "Prepare/apply guidance:", "Components and imports:", "Variables guidance:", "Relevant CLI usage:", "typed guidance", "Candidate typed steps you may choose from:", "Step composition guidance:"} {
		if strings.Contains(prompt, avoid) {
			t.Fatalf("expected generation prompt to avoid duplicated context block %q, got %q", avoid, prompt)
		}
	}
}

func TestAskRequestTimeoutScalesGenerationByIterationsAndPromptSize(t *testing.T) {
	small := askRequestTimeout("generate", 1, "sys", "user")
	large := askRequestTimeout("generate", 5, strings.Repeat("s", 12000), strings.Repeat("u", 12000))
	if large <= small {
		t.Fatalf("expected larger generation timeout for more iterations and prompt bytes, got small=%s large=%s", small, large)
	}
	if askRequestTimeout("classify", 1, "sys", "user") >= small {
		t.Fatalf("expected classify timeout to stay below generation timeout")
	}
}

func TestGenerationRetrievalPromptBlockSkipsProjectContextAndCapsExamples(t *testing.T) {
	retrieval := askretrieve.RetrievalResult{Chunks: []askretrieve.Chunk{
		{ID: "example-1", Source: "example", Label: "a", Content: "Reference example:\n- path: a\n", Score: 100},
		{ID: "example-2", Source: "example", Label: "b", Content: "Reference example:\n- path: b\n", Score: 99},
		{ID: "example-3", Source: "example", Label: "c", Content: "Reference example:\n- path: c\n", Score: 98},
		{ID: "project", Source: "project", Label: "project-context", Content: "Project context:\nvery long", Score: 97},
	}}
	block := generationRetrievalPromptBlock(retrieval)
	if strings.Contains(block, "Project context:") {
		t.Fatalf("expected project context to be skipped, got %q", block)
	}
	if strings.Count(block, "Reference example:") != 2 {
		t.Fatalf("expected exactly two examples in generation retrieval block, got %q", block)
	}
}

func TestGenerationRetrievalPromptBlockSeparatesLocalFactsAndExternalEvidence(t *testing.T) {
	block := generationRetrievalPromptBlock(askretrieve.RetrievalResult{Chunks: []askretrieve.Chunk{{ID: "facts-1", Source: "local-facts", Label: "source-of-truth-stepmeta", Topic: "local-facts:stepmeta", Content: "Local facts:\n- path: internal/stepmeta/registry.go", Score: 80}, {ID: "mcp-1", Source: "mcp", Label: "web-search:kubernetes.io", Topic: "mcp:web-search:kubernetes.io", Content: "Typed MCP evidence JSON:\n{}", Score: 70}, {ID: "workspace-1", Source: "workspace", Label: "workflows/scenarios/apply.yaml", Topic: "workspace:workflows/scenarios/apply.yaml", Content: "version: v1alpha1", Score: 60}}})
	for _, want := range []string{"Local facts:", "External evidence:", "Retrieved context:"} {
		if !strings.Contains(block, want) {
			t.Fatalf("expected %q in retrieval prompt block, got %q", want, block)
		}
	}
	if strings.Index(block, "Local facts:") > strings.Index(block, "External evidence:") {
		t.Fatalf("expected local facts before external evidence, got %q", block)
	}
}

func TestGenerationSystemPromptUsesStructuredDocumentResponseShape(t *testing.T) {
	req := askpolicy.ScenarioRequirements{Connectivity: "unspecified", RequiredFiles: []string{"workflows/prepare.yaml"}, NeedsPrepare: true}
	scaffold := askscaffold.Build(req, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft, Target: askintent.Target{Kind: "scenario", Path: "workflows/prepare.yaml"}}, askcontract.PlanResponse{}, askknowledge.Current())
	prompt := generationSystemPrompt(askintent.RouteDraft, askintent.Target{Kind: "scenario", Path: "workflows/prepare.yaml"}, "create a prepare workflow using DownloadFile and default output location", askretrieve.RetrievalResult{}, req, askcontract.PlanResponse{}, askcontract.AuthoringBrief{ModeIntent: "prepare-only", CompletenessTarget: "complete"}, askcontract.ExecutionModel{}, scaffold)
	for _, want := range []string{"JSON shape: {\"summary\":string,\"review\":[]string,\"selection\":", "Return structured workflow documents, not final YAML text.", "Keep document structure schema-focused:"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in generation prompt, got %q", want, prompt)
		}
	}
}

func TestGenerationSystemPromptOmitsTypedStepCandidateBlocks(t *testing.T) {
	req := askpolicy.ScenarioRequirements{Connectivity: "offline", RequiredFiles: []string{"workflows/scenarios/apply.yaml"}}
	scaffold := askscaffold.Build(req, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft}, askcontract.PlanResponse{}, askknowledge.Current())
	prompt := generationSystemPrompt(askintent.RouteDraft, askintent.Target{Kind: "workspace"}, "create an air-gapped rhel9 single-node kubeadm workflow", askretrieve.RetrievalResult{}, req, askcontract.PlanResponse{}, askcontract.AuthoringBrief{ModeIntent: "apply-only", Connectivity: "offline", CompletenessTarget: "starter", Topology: "single-node", RequiredCapabilities: []string{"kubeadm-bootstrap", "cluster-verification"}}, askcontract.ExecutionModel{}, scaffold)
	for _, avoid := range []string{"Candidate typed steps you may choose from:", "These are hints, not required selections."} {
		if strings.Contains(prompt, avoid) {
			t.Fatalf("expected %q to be omitted from generation prompt, got %q", avoid, prompt)
		}
	}
	for _, want := range []string{"Relevant typed-step schemas:", "spec.checks [required]:", "constrained: spec.runtime"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in generation prompt, got %q", want, prompt)
		}
	}
}

func TestGenerationSystemPromptCarriesWorkflowStepIDUniquenessRule(t *testing.T) {
	req := askpolicy.ScenarioRequirements{Connectivity: "offline", RequiredFiles: []string{"workflows/prepare.yaml", "workflows/scenarios/apply.yaml"}, NeedsPrepare: true}
	scaffold := askscaffold.Build(req, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft}, askcontract.PlanResponse{}, askknowledge.Current())
	prompt := generationSystemPrompt(askintent.RouteDraft, askintent.Target{Kind: "workspace"}, "create a 3-node workflow", askretrieve.RetrievalResult{}, req, askcontract.PlanResponse{}, askcontract.AuthoringBrief{ModeIntent: "prepare+apply", CompletenessTarget: "complete"}, askcontract.ExecutionModel{}, scaffold)
	for _, want := range []string{"Every step id must be unique across top-level steps and steps nested under phases.", "Rename duplicate step ids with role- or phase-specific prefixes instead of reusing the same id."} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in generation prompt, got %q", want, prompt)
		}
	}
}

func TestGenerationSystemPromptPrefersInlineFirstForComplexWorkspaceDrafts(t *testing.T) {
	req := askpolicy.ScenarioRequirements{Connectivity: "offline", RequiredFiles: []string{"workflows/prepare.yaml", "workflows/scenarios/apply.yaml"}, NeedsPrepare: true}
	scaffold := askscaffold.Build(req, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft}, askcontract.PlanResponse{}, askknowledge.Current())
	prompt := generationSystemPrompt(askintent.RouteDraft, askintent.Target{Kind: "workspace"}, "create an air-gapped 3-node kubeadm workflow", askretrieve.RetrievalResult{}, req, askcontract.PlanResponse{}, askcontract.AuthoringBrief{ModeIntent: "prepare+apply", Connectivity: "offline", CompletenessTarget: "complete", Topology: "multi-node"}, askcontract.ExecutionModel{}, scaffold)
	for _, want := range []string{"prefer a first schema-valid inline result", "extract `workflows/components/` only when reuse is explicit"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in generation prompt, got %q", want, prompt)
		}
	}
}

func TestGenerationSystemPromptPrefersSelectionForDrafts(t *testing.T) {
	req := askpolicy.ScenarioRequirements{Connectivity: "offline", RequiredFiles: []string{"workflows/scenarios/apply.yaml"}}
	scaffold := askscaffold.Build(req, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft}, askcontract.PlanResponse{}, askknowledge.Current())
	prompt := generationSystemPrompt(askintent.RouteDraft, askintent.Target{Kind: "workspace"}, "create a simple apply workflow", askretrieve.RetrievalResult{}, req, askcontract.PlanResponse{}, askcontract.AuthoringBrief{ModeIntent: "apply-only", CompletenessTarget: "starter"}, askcontract.ExecutionModel{}, scaffold)
	for _, want := range []string{"selection.targets[].builders", "Do not author arbitrary typed step specs on the primary draft path", "selection.targets"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in draft selection prompt, got %q", want, prompt)
		}
	}
}

func TestGenerationSystemPromptUsesRefineDocumentActions(t *testing.T) {
	req := askpolicy.ScenarioRequirements{Connectivity: "offline", AcceptanceLevel: "refine", RequiredFiles: []string{"workflows/scenarios/apply.yaml"}}
	workspace := askretrieve.WorkspaceSummary{HasWorkflowTree: true, Files: []askretrieve.WorkspaceFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [true]\n"}}}
	scaffold := askscaffold.Build(req, workspace, askintent.Decision{Route: askintent.RouteRefine}, askcontract.PlanResponse{}, askknowledge.Current())
	plan := askcontract.PlanResponse{AuthoringBrief: askcontract.AuthoringBrief{TargetPaths: []string{"workflows/scenarios/apply.yaml"}, TargetScope: "scenario"}}
	systemPrompt := generationSystemPrompt(askintent.RouteRefine, askintent.Target{Kind: "workspace"}, "refine the apply workflow to add a timeout", askretrieve.RetrievalResult{}, req, plan, askcontract.AuthoringBrief{ModeIntent: "apply-only", CompletenessTarget: "refine", TargetPaths: []string{"workflows/scenarios/apply.yaml"}, TargetScope: "scenario"}, askcontract.ExecutionModel{}, scaffold)
	userPrompt := generationUserPrompt(workspace, askstate.Context{}, "refine the apply workflow to add a timeout", "", askintent.RouteRefine, plan)
	for _, want := range []string{"actions preserve|delete|edit", "Return structured workflow documents, not final YAML text.", "Refine transform contract:", "Approved target paths: workflows/scenarios/apply.yaml"} {
		if !strings.Contains(systemPrompt, want) {
			t.Fatalf("expected %q in refine system prompt, got %q", want, systemPrompt)
		}
	}
	for _, want := range []string{"Current parsed workspace documents:", "workflows/scenarios/apply.yaml [workflow]", "Clarified refine target paths:"} {
		if !strings.Contains(userPrompt, want) {
			t.Fatalf("expected %q in refine user prompt, got %q", want, userPrompt)
		}
	}
}

func TestGenerationSystemPromptAddsVarsTransformHintsForRefine(t *testing.T) {
	req := askpolicy.ScenarioRequirements{AcceptanceLevel: "refine", RequiredFiles: []string{"workflows/scenarios/control-plane-bootstrap.yaml", "workflows/vars.yaml"}}
	workspace := askretrieve.WorkspaceSummary{HasWorkflowTree: true}
	plan := askcontract.PlanResponse{
		VarsRecommendation: []string{"Use workflows/vars.yaml for repeated values."},
		AuthoringBrief:     askcontract.AuthoringBrief{TargetScope: "workspace", TargetPaths: []string{"workflows/scenarios/control-plane-bootstrap.yaml", "workflows/vars.yaml"}, ModeIntent: "apply-only", CompletenessTarget: "refine"},
	}
	scaffold := askscaffold.Build(req, workspace, askintent.Decision{Route: askintent.RouteRefine}, plan, askknowledge.Current())
	prompt := generationSystemPrompt(askintent.RouteRefine, askintent.Target{Kind: "scenario", Path: "workflows/scenarios/control-plane-bootstrap.yaml"}, "refactor workflows/scenarios/control-plane-bootstrap.yaml to use workflows/vars.yaml for repeated values", askretrieve.RetrievalResult{}, req, plan, plan.AuthoringBrief, askcontract.ExecutionModel{}, scaffold)
	for _, want := range []string{"Refine transform contract:", "Approved target paths: workflows/scenarios/control-plane-bootstrap.yaml, workflows/vars.yaml", "update the scenario file and vars file together as one transform", "For `extract-var`, put the variable key in `varName`.", "Only extract values into workflows/vars.yaml when they are explicitly recommended or genuinely repeated.", "vars advisory:"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in refine transform prompt, got %q", want, prompt)
		}
	}
}

func TestParseGenerationAllowsRefineTransforms(t *testing.T) {
	resp, err := askcontract.ParseGeneration(`{"summary":"vars transform","review":[],"documents":[{"path":"workflows/scenarios/apply.yaml","action":"edit","transforms":[{"type":"extract-var","rawPath":"steps[0].spec.podNetworkCIDR","varName":"podCIDR","varsPath":"workflows/vars.yaml","value":"10.244.0.0/16"}]}]}`)
	if err != nil {
		t.Fatalf("parse generation: %v", err)
	}
	if len(resp.Documents) != 1 || len(resp.Documents[0].Transforms) != 1 {
		t.Fatalf("expected parsed transform, got %#v", resp.Documents)
	}
	if resp.Documents[0].Transforms[0].Type != "extract-var" {
		t.Fatalf("expected normalized transform type, got %#v", resp.Documents[0].Transforms[0])
	}
}

func TestParseGenerationNormalizesFieldTransforms(t *testing.T) {
	resp, err := askcontract.ParseGeneration(`{"summary":"field transforms","review":[],"documents":[{"path":"workflows/scenarios/apply.yaml","action":"edit","transforms":[{"type":"update-field","rawPath":"steps[0].timeout","value":"10m"},{"type":"remove-field","rawPath":"steps[0].spec.ignorePreflightErrors"}]}]}`)
	if err != nil {
		t.Fatalf("parse generation: %v", err)
	}
	if len(resp.Documents) != 1 || len(resp.Documents[0].Transforms) != 2 {
		t.Fatalf("expected parsed transforms, got %#v", resp.Documents)
	}
	if resp.Documents[0].Transforms[0].Type != "set-field" || resp.Documents[0].Transforms[1].Type != "delete-field" {
		t.Fatalf("expected normalized field transforms, got %#v", resp.Documents[0].Transforms)
	}
}

func TestParseGenerationNormalizesComponentTransform(t *testing.T) {
	resp, err := askcontract.ParseGeneration(`{"summary":"component transform","review":[],"documents":[{"path":"workflows/scenarios/apply.yaml","action":"edit","transforms":[{"type":"extract_component","rawPath":"phases.bootstrap","path":"workflows/components/bootstrap.yaml"}]}]}`)
	if err != nil {
		t.Fatalf("parse generation: %v", err)
	}
	if len(resp.Documents) != 1 || len(resp.Documents[0].Transforms) != 1 {
		t.Fatalf("expected parsed component transform, got %#v", resp.Documents)
	}
	if resp.Documents[0].Transforms[0].Type != "extract-component" {
		t.Fatalf("expected normalized component transform, got %#v", resp.Documents[0].Transforms[0])
	}
}

func TestParseGenerationCompilesDraftSelection(t *testing.T) {
	resp, err := askcontract.ParseGeneration(`{"summary":"selection draft","review":[],"selection":{"patterns":["apply-only"],"targets":[{"path":"workflows/scenarios/apply.yaml","kind":"workflow","steps":[{"id":"run","kind":"Command","spec":{"command":["true"]}}]},{"path":"workflows/vars.yaml","kind":"vars","vars":{"role":"control-plane"}}]}}`)
	if err != nil {
		t.Fatalf("parse generation: %v", err)
	}
	if len(resp.Documents) != 2 {
		t.Fatalf("expected compiled documents from selection, got %#v", resp.Documents)
	}
	if resp.Documents[0].Workflow == nil || resp.Documents[1].Vars == nil {
		t.Fatalf("expected compiled workflow and vars docs, got %#v", resp.Documents)
	}
}

func TestParseGenerationKeepsBuilderSelectionsDeferred(t *testing.T) {
	resp, err := askcontract.ParseGeneration(`{"summary":"builder draft","review":[],"selection":{"targets":[{"path":"workflows/scenarios/apply.yaml","kind":"workflow","builders":[{"id":"apply.init-kubeadm","overrides":{"joinFile":"/tmp/deck/join.txt"}}]}]}}`)
	if err != nil {
		t.Fatalf("parse generation builder selection: %v", err)
	}
	if len(resp.Documents) != 0 || resp.Selection == nil || len(resp.Selection.Targets) != 1 || len(resp.Selection.Targets[0].Builders) != 1 {
		t.Fatalf("expected builder selection to remain deferred for materialization, got %#v", resp)
	}
}

func TestGenerateWithValidationRejectsLegacyDraftDocumentsByDefault(t *testing.T) {
	client := &stubClient{responses: []string{`{"summary":"legacy draft","review":[],"documents":[{"path":"workflows/scenarios/apply.yaml","kind":"workflow","workflow":{"version":"v1alpha1","steps":[{"id":"run","kind":"Command","spec":{"command":["true"]}}]}}]}`}}
	_, _, _, _, _, _, err := generateWithValidation(context.Background(), client, askprovider.Request{Kind: "generate-fast", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key", Prompt: "create workflow"}, t.TempDir(), 1, newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteDraft}, askcontract.PlanResponse{}, askcontract.AuthoringBrief{}, askretrieve.RetrievalResult{}, askcontract.PlanCriticResponse{})
	if err == nil || !strings.Contains(err.Error(), "builder selection") {
		t.Fatalf("expected legacy draft documents to be rejected, got %v", err)
	}
}

func TestGenerateWithValidationRejectsLegacyRefineReplaceByDefault(t *testing.T) {
	client := &stubClient{responses: []string{`{"summary":"legacy refine","review":[],"documents":[{"path":"workflows/scenarios/apply.yaml","action":"replace","workflow":{"version":"v1alpha1","steps":[{"id":"run","kind":"Command","spec":{"command":["true"]}}]}}]}`}}
	_, _, _, _, _, _, err := generateWithValidation(context.Background(), client, askprovider.Request{Kind: "generate-fast", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key", Prompt: "refine workflow"}, t.TempDir(), 1, newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteRefine}, askcontract.PlanResponse{}, askcontract.AuthoringBrief{}, askretrieve.RetrievalResult{}, askcontract.PlanCriticResponse{})
	if err == nil || !strings.Contains(err.Error(), "refine primary path") {
		t.Fatalf("expected legacy refine replace output to be rejected, got %v", err)
	}
}

func TestGenerateWithValidationAllowsLegacyDraftFallbackWhenEnabled(t *testing.T) {
	enableLegacyAuthoringFallback(t)
	client := &stubClient{responses: []string{`{"summary":"legacy draft","review":[],"documents":[{"path":"workflows/scenarios/apply.yaml","kind":"workflow","workflow":{"version":"v1alpha1","steps":[{"id":"run","kind":"Command","spec":{"command":["true"]}}]}}]}`}}
	_, files, _, _, _, _, err := generateWithValidation(context.Background(), client, askprovider.Request{Kind: "generate-fast", Provider: "openai", Model: "gpt-5.4", APIKey: "test-key", Prompt: "create workflow"}, t.TempDir(), 1, newAskLogger(io.Discard, "trace"), askintent.Decision{Route: askintent.RouteDraft}, askcontract.PlanResponse{}, askcontract.AuthoringBrief{}, askretrieve.RetrievalResult{}, askcontract.PlanCriticResponse{})
	if err != nil || len(files) != 1 {
		t.Fatalf("expected legacy draft fallback to succeed when enabled, got files=%#v err=%v", files, err)
	}
}

func TestValidatePrimaryAuthoringContractRejectsDraftDocumentsOnRetry(t *testing.T) {
	err := validatePrimaryAuthoringContract(askintent.RouteDraft, askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{Path: "workflows/scenarios/apply.yaml", Kind: "workflow", Workflow: &askcontract.WorkflowDocument{Version: "v1alpha1"}}}}, 2)
	if err == nil || !strings.Contains(err.Error(), "builder selection") {
		t.Fatalf("expected draft primary contract to reject direct documents on retry, got %v", err)
	}
}

func TestDraftSelectionRetryPromptAvoidsDocumentRepairMode(t *testing.T) {
	prompt := draftSelectionRetryPrompt("create offline workflow", "schema invalid", []askdiagnostic.Diagnostic{{Message: "spec.runtime must be one of auto, ctr, docker, podman"}})
	for _, want := range []string{"Return draft selection only", "Do not return documents", "spec.runtime must be one of"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in draft retry prompt, got %q", want, prompt)
		}
	}
}

func TestValidatePrimaryRefineContractAllowsRawVarsCompanionTransform(t *testing.T) {
	err := validatePrimaryRefineContract(askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{Path: "workflows/scenarios/apply.yaml", Action: "edit", Transforms: []askcontract.RefineTransformAction{{Type: "set-field", Candidate: "set-field|workflows/scenarios/apply.yaml|steps[0].spec.outputJoinFile", Value: "{{ .vars.joinFilePath }}"}}}, {Path: "workflows/vars.yaml", Action: "edit", Transforms: []askcontract.RefineTransformAction{{Type: "set-field", Path: "vars.joinFilePath", Value: "/tmp/deck/join.txt"}}}}})
	if err != nil {
		t.Fatalf("expected raw vars companion transform to be allowed, got %v", err)
	}
}

func TestValidatePrimaryRefineContractAllowsRawScenarioTransform(t *testing.T) {
	err := validatePrimaryRefineContract(askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{Path: "workflows/scenarios/apply.yaml", Action: "edit", Transforms: []askcontract.RefineTransformAction{{Type: "set-field", RawPath: "steps[0].timeout", Value: "10m"}}}}})
	if err != nil {
		t.Fatalf("expected raw scenario transform to be allowed, got %v", err)
	}
}

func TestValidatePrimaryRefineContractRejectsRawTransformWithoutTargetPath(t *testing.T) {
	err := validatePrimaryRefineContract(askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{Path: "workflows/scenarios/apply.yaml", Action: "edit", Transforms: []askcontract.RefineTransformAction{{Type: "set-field", Value: "10m"}}}}})
	if err == nil || !strings.Contains(err.Error(), "explicit target path") {
		t.Fatalf("expected raw transform without target path to be rejected, got %v", err)
	}
}

func TestValidatePrimaryRefineContractRejectsExtractComponentWithoutSourceRawPath(t *testing.T) {
	err := validatePrimaryRefineContract(askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{Path: "workflows/scenarios/apply.yaml", Action: "edit", Transforms: []askcontract.RefineTransformAction{{Type: "extract-component", Path: "workflows/components/bootstrap.yaml"}}}}})
	if err == nil || !strings.Contains(err.Error(), "explicit target path") {
		t.Fatalf("expected extract-component without raw path to be rejected, got %v", err)
	}
}

func TestGenerationSystemPromptAddsComponentTransformHintsForRefine(t *testing.T) {
	req := askpolicy.ScenarioRequirements{AcceptanceLevel: "refine", RequiredFiles: []string{"workflows/scenarios/control-plane-bootstrap.yaml", "workflows/components/bootstrap.yaml"}}
	workspace := askretrieve.WorkspaceSummary{HasWorkflowTree: true}
	plan := askcontract.PlanResponse{ComponentRecommendation: []string{"Consider workflows/components/ for reusable repeated logic across phases or scenarios."}, AuthoringBrief: askcontract.AuthoringBrief{TargetScope: "workspace", TargetPaths: []string{"workflows/scenarios/control-plane-bootstrap.yaml", "workflows/components/bootstrap.yaml"}, ModeIntent: "apply-only", CompletenessTarget: "refine"}}
	scaffold := askscaffold.Build(req, workspace, askintent.Decision{Route: askintent.RouteRefine}, plan, askknowledge.Current())
	prompt := generationSystemPrompt(askintent.RouteRefine, askintent.Target{Kind: "scenario", Path: "workflows/scenarios/control-plane-bootstrap.yaml"}, "extract reusable bootstrap logic into workflows/components/", askretrieve.RetrievalResult{}, req, plan, plan.AuthoringBrief, askcontract.ExecutionModel{}, scaffold)
	for _, want := range []string{"extract-component", "component advisory:", "moving inline phase steps into workflows/components/ while preserving the scenario phase layout"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in component transform prompt, got %q", want, prompt)
		}
	}
}

func TestExplainAndReviewSystemPromptsIncludeParsedWorkflowSummaries(t *testing.T) {
	retrieval := askretrieve.RetrievalResult{Chunks: []askretrieve.Chunk{{Label: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nphases:\n  - name: bootstrap\n    steps:\n      - id: run\n        kind: Command\n        spec:\n          command: [true]\n"}}}
	for _, prompt := range []string{explainSystemPrompt(askintent.Target{Kind: "scenario", Path: "workflows/scenarios/apply.yaml"}, retrieval), reviewSystemPrompt(askintent.Target{Kind: "scenario", Path: "workflows/scenarios/apply.yaml"}, retrieval, askretrieve.WorkspaceSummary{})} {
		for _, want := range []string{"Parsed workflow summaries:", "workflows/scenarios/apply.yaml [workflow] phases=1 top-level-steps=0"} {
			if !strings.Contains(prompt, want) {
				t.Fatalf("expected %q in info prompt, got %q", want, prompt)
			}
		}
	}
}

func TestExplainSystemPromptUsesCodePathModeForRepoBehavior(t *testing.T) {
	retrieval := askretrieve.RetrievalResult{Chunks: []askretrieve.Chunk{{ID: "local-facts-stepmeta", Source: "local-facts", Label: "source-of-truth-stepmeta", Content: "Local facts:\n- path: internal/stepmeta/registry.go"}, {ID: "local-facts-askdraft", Source: "local-facts", Label: "askdraft-compiler", Content: "Local facts:\n- file: internal/askdraft/draft.go\n- function: CompileWithProgram"}, {ID: "workspace-apply", Source: "workspace", Label: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps: []\n"}}}
	prompt := explainSystemPrompt(askintent.Target{Kind: "workspace"}, retrieval)
	for _, want := range []string{"explaining how this repository assembles workflow behavior", "internal/stepmeta", "internal/stepspec", "internal/askdraft", "registry/metadata -> builder selection -> binding resolution -> workflow document compilation"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in repo-behavior explain prompt, got %q", want, prompt)
		}
	}
	if strings.Contains(prompt, "Parsed workflow summaries:") {
		t.Fatalf("expected repo-behavior explain prompt to avoid workspace-summary framing, got %q", prompt)
	}
}

func TestReviewSystemPromptIncludesStructuredValidationIssues(t *testing.T) {
	workspace := askretrieve.WorkspaceSummary{Files: []askretrieve.WorkspaceFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: install\n    kind: InstallPackage\n    spec:\n      packages: [kubeadm]\n      sourceDir: /tmp/packages\n"}}}
	prompt := reviewSystemPrompt(askintent.Target{Kind: "scenario", Path: "workflows/scenarios/apply.yaml"}, askretrieve.RetrievalResult{}, workspace)
	for _, want := range []string{"Structured validation issues:", "sourceDir", "Additional property sourceDir is not allowed"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in review prompt, got %q", want, prompt)
		}
	}
}

func TestPromptAndDocsShareSchemaRuleSummaryForDownloadFile(t *testing.T) {
	page := testSchemaDocFamilyPageInput(t, "file")
	rendered := string(schemadoc.RenderToolPage(page))
	req := askpolicy.ScenarioRequirements{Connectivity: "unspecified", RequiredFiles: []string{"workflows/prepare.yaml"}, NeedsPrepare: true}
	scaffold := askscaffold.Build(req, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft, Target: askintent.Target{Kind: "scenario", Path: "workflows/prepare.yaml"}}, askcontract.PlanResponse{}, askknowledge.Current())
	prompt := generationSystemPrompt(askintent.RouteDraft, askintent.Target{Kind: "scenario", Path: "workflows/prepare.yaml"}, "create a prepare workflow using DownloadFile", askretrieve.RetrievalResult{}, req, askcontract.PlanResponse{}, askcontract.AuthoringBrief{ModeIntent: "prepare-only", CompletenessTarget: "complete"}, askcontract.ExecutionModel{}, scaffold)
	rule := "At least one of `spec.source` or `spec.items` must be set."
	if !strings.Contains(rendered, rule) {
		t.Fatalf("expected docs to include %q, got %q", rule, rendered)
	}
	if !strings.Contains(prompt, rule) {
		t.Fatalf("expected prompt to include schema-derived %q, got %q", rule, prompt)
	}
}

func testSchemaDocFamilyPageInput(t *testing.T, family string) schemadoc.PageInput {
	t.Helper()
	defs := workflowcontract.StepDefinitions()
	page := schemadoc.PageInput{Family: family}
	for _, def := range defs {
		if def.Family != family || def.Visibility != "public" {
			continue
		}
		raw, err := schemas.ToolSchema(def.SchemaFile)
		if err != nil {
			t.Fatalf("ToolSchema(%q): %v", def.SchemaFile, err)
		}
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("unmarshal tool schema %q: %v", def.SchemaFile, err)
		}
		properties, _ := schema["properties"].(map[string]any)
		spec, _ := properties["spec"].(map[string]any)
		if page.PageSlug == "" {
			page.PageSlug = def.DocsPage
			page.Title = schemadoc.DisplayFamilyTitle(def.Family, "")
			page.Summary = "Reference for the `" + page.Title + "` family of typed workflow steps."
		}
		page.Variants = append(page.Variants, schemadoc.VariantInput{
			Kind:        def.Kind,
			Title:       def.FamilyTitle,
			Description: def.Summary,
			SchemaPath:  filepath.ToSlash(filepath.Join("schemas", "tools", def.SchemaFile)),
			Schema:      schema,
			Meta:        schemadoc.ToolMetaForDefinition(def),
			Spec:        spec,
			Outputs:     append([]string(nil), def.Outputs...),
			DocsOrder:   def.DocsOrder,
		})
	}
	if page.PageSlug == "" {
		t.Fatalf("missing test page for family %s", family)
	}
	return page
}

func TestAppendPlanAdvisoryPromptCarriesRecoverableReviewIntoGeneration(t *testing.T) {
	base := generationUserPrompt(askretrieve.WorkspaceSummary{}, askstate.Context{}, "create a 3-node kubeadm workflow", "", askintent.RouteDraft, askcontract.PlanResponse{})
	plan := askcontract.PlanResponse{Blockers: []string{"join publication path is still underspecified"}, OpenQuestions: []string{"artifact checksum naming can be refined later"}}
	critic := askcontract.PlanCriticResponse{Advisory: []string{"final verification should stay on control-plane"}, MissingContracts: []string{"role cardinality contract"}, SuggestedFixes: []string{"publish join state explicitly before worker JoinKubeadm"}}
	prompt := appendPlanAdvisoryPrompt(base, plan, critic)
	for _, want := range []string{"Plan review carry-forward:", "generate the best viable draft", "artifact checksum naming can be refined later", "recoverable missing contract: role cardinality contract", "plan suggested fix: publish join state explicitly before worker JoinKubeadm"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in generation prompt, got %q", want, prompt)
		}
	}
	if strings.Contains(prompt, "join publication path is still underspecified") {
		t.Fatalf("expected fatal blockers to stay out of generation carry-forward, got %q", prompt)
	}
}

func TestRequiredFixesForValidationFlagsTemplatedCollections(t *testing.T) {
	fixes := requiredFixesForValidation("parse yaml: yaml: invalid map key: map[string]interface {}{\".vars.dockerPackages\":interface {}(nil)}")
	if len(fixes) < 2 {
		t.Fatalf("expected extra required fixes, got %v", fixes)
	}
	joined := strings.Join(fixes, "\n")
	if !strings.Contains(joined, "whole-value template expressions") {
		t.Fatalf("unexpected templated collection fix: %v", fixes)
	}
}

func TestRequiredFixesForValidationIncludesCheckHostRepairHint(t *testing.T) {
	fixes := requiredFixesForValidation("E_SCHEMA_INVALID: step check-rhel9-host (CheckHost): spec: checks is required; spec: Additional property os is not allowed")
	joined := strings.Join(fixes, "\n")
	for _, want := range []string{"spec.checks", "spec.os"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in fixes, got %v", want, fixes)
		}
	}
}

func TestLoadRequestTextReadsWorkspaceFile(t *testing.T) {
	root := t.TempDir()
	requestPath := filepath.Join(root, "request.md")
	if err := os.WriteFile(requestPath, []byte("extra details\n"), 0o600); err != nil {
		t.Fatalf("write request file: %v", err)
	}
	text, source, err := loadRequestText(root, "base prompt", "request.md")
	if err != nil {
		t.Fatalf("load request text: %v", err)
	}
	if source != "file" {
		t.Fatalf("unexpected source: %s", source)
	}
	if !strings.Contains(text, "base prompt") || !strings.Contains(text, "extra details") {
		t.Fatalf("unexpected request text: %q", text)
	}
}

func TestLoadRequestTextRejectsEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "request.md")
	if err := os.WriteFile(outside, []byte("secret\n"), 0o600); err != nil {
		t.Fatalf("write outside request file: %v", err)
	}
	_, _, err := loadRequestText(root, "", outside)
	if err == nil {
		t.Fatalf("expected escape rejection")
	}
	if !strings.Contains(err.Error(), "resolve ask request file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRequestTextPrefersPlanJSON(t *testing.T) {
	root := t.TempDir()
	planDir := filepath.Join(root, ".deck", "plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	mdPath := filepath.Join(planDir, "sample.md")
	jsonPath := filepath.Join(planDir, "sample.json")
	if err := os.WriteFile(mdPath, []byte("freeform markdown"), 0o600); err != nil {
		t.Fatalf("write md: %v", err)
	}
	json := `{"version":1,"request":"create workflow","intent":"draft","complexity":"complex","blockers":[],"targetOutcome":"generate files","assumptions":[],"openQuestions":[],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"entry"}],"validationChecklist":["lint"]}`
	if err := os.WriteFile(jsonPath, []byte(json), 0o600); err != nil {
		t.Fatalf("write json: %v", err)
	}
	text, source, err := loadRequestText(root, "", ".deck/plan/sample.md")
	if err != nil {
		t.Fatalf("load request text: %v", err)
	}
	if source != "plan-json" {
		t.Fatalf("expected plan-json source, got %s", source)
	}
	if !strings.Contains(text, "Plan request") || !strings.Contains(text, "workflows/scenarios/apply.yaml") {
		t.Fatalf("expected plan-derived request text, got %q", text)
	}
}

func TestLoadRequestTextFallsBackToPlanMarkdown(t *testing.T) {
	root := t.TempDir()
	planDir := filepath.Join(root, ".deck", "plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "sample.md"), []byte("freeform markdown"), 0o600); err != nil {
		t.Fatalf("write markdown: %v", err)
	}
	text, source, err := loadRequestText(root, "", ".deck/plan/sample.md")
	if err != nil {
		t.Fatalf("load request text: %v", err)
	}
	if source != "plan-markdown" {
		t.Fatalf("expected plan-markdown source, got %s", source)
	}
	if text != "freeform markdown" {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestLoadPlanArtifactFromMarkdownResolvesJSON(t *testing.T) {
	root := t.TempDir()
	planDir := filepath.Join(root, ".deck", "plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "sample.md"), []byte("markdown"), 0o600); err != nil {
		t.Fatalf("write markdown: %v", err)
	}
	json := `{"version":1,"request":"create workflow","intent":"draft","complexity":"complex","blockers":[],"targetOutcome":"generate files","assumptions":[],"openQuestions":[],"clarifications":[{"id":"topology.kind","question":"pick topology","kind":"enum","options":["single-node","multi-node"],"blocksGeneration":true}],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"entry"}],"validationChecklist":["lint"]}`
	if err := os.WriteFile(filepath.Join(planDir, "sample.json"), []byte(json), 0o600); err != nil {
		t.Fatalf("write json: %v", err)
	}
	plan, relJSON, err := loadPlanArtifact(root, ".deck/plan/sample.md")
	if err != nil {
		t.Fatalf("load plan artifact: %v", err)
	}
	if relJSON != ".deck/plan/sample.json" {
		t.Fatalf("unexpected json path: %s", relJSON)
	}
	if len(plan.Clarifications) != 1 || plan.Clarifications[0].ID != "topology.kind" {
		t.Fatalf("unexpected clarifications: %#v", plan.Clarifications)
	}
}

func TestApplyPlanAnswersNormalizesClarificationState(t *testing.T) {
	plan := askcontract.PlanResponse{
		Version:        1,
		Request:        "create 3-node kubeadm workflow",
		Intent:         "draft",
		Complexity:     "complex",
		TargetOutcome:  "generate files",
		Files:          []askcontract.PlanFile{{Path: "workflows/prepare.yaml", Kind: "workflow", Action: "create", Purpose: "prepare"}, {Path: "workflows/scenarios/apply.yaml", Kind: "scenario", Action: "create", Purpose: "apply"}},
		Clarifications: []askcontract.PlanClarification{{ID: "topology.roleModel", Question: "pick role model", Kind: "enum", Options: []string{"1cp-2workers", "3cp-ha"}, BlocksGeneration: true}},
	}
	updated, err := applyPlanAnswers(plan, []string{"topology.roleModel=1cp-2workers"})
	if err != nil {
		t.Fatalf("apply plan answers: %v", err)
	}
	answered := ""
	for _, item := range updated.Clarifications {
		if item.ID == "topology.roleModel" {
			answered = item.Answer
			break
		}
	}
	if answered != "1cp-2workers" {
		t.Fatalf("expected stored answer, got %#v", updated.Clarifications)
	}
	if updated.AuthoringBrief.NodeCount != 3 || updated.ExecutionModel.RoleExecution.RoleSelector != "vars.role" {
		t.Fatalf("expected normalized role-aware plan, got %#v %#v", updated.AuthoringBrief, updated.ExecutionModel)
	}
}

func TestExecutePlanResumeStopsWhenClarificationsRemain(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "test-key")
	root := t.TempDir()
	planDir := filepath.Join(root, ".deck", "plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	json := `{"version":1,"request":"create workflow","intent":"draft","complexity":"complex","blockers":[],"targetOutcome":"generate files","assumptions":[],"openQuestions":[],"clarifications":[{"id":"topology.kind","question":"pick topology","kind":"enum","options":["single-node","multi-node"],"blocksGeneration":true}],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"entry"}],"validationChecklist":["lint"]}`
	if err := os.WriteFile(filepath.Join(planDir, "latest.json"), []byte(json), 0o600); err != nil {
		t.Fatalf("write json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "latest.md"), []byte("md"), 0o600); err != nil {
		t.Fatalf("write md: %v", err)
	}
	stdout := &bytes.Buffer{}
	client := &stubClient{responses: []string{}}
	if err := Execute(context.Background(), Options{Root: root, Prompt: "implement this plan", FromPath: ".deck/plan/latest.md", Stdin: strings.NewReader(""), Stdout: stdout, Stderr: io.Discard}, client); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "saved plan still requires clarification") {
		t.Fatalf("expected clarification stop, got %q", stdout.String())
	}
}

func TestExecuteAuthoringMCPAugmentationIsGatedByEnv(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "test-key")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	if err := askconfig.SaveStored(askconfig.Settings{MCP: askconfig.MCP{Enabled: true, Servers: []askconfig.MCPServer{{Name: "web-server", RunCommand: "/bin/sh", Args: []string{"-c", "exit 0"}}}}}); err != nil {
		t.Fatalf("save stored config: %v", err)
	}
	tests := []struct {
		name    string
		enabled bool
		want    []string
		avoid   []string
	}{
		{
			name:    "disabled-by-default",
			enabled: false,
			want: []string{
				"mcp: disabled for default local pipeline",
			},
			avoid: []string{"mcp:web-search initialize failed:"},
		},
		{
			name:    "enabled-via-env",
			enabled: true,
			want: []string{
				"mcp:web-search initialize failed:",
				"transport closed",
			},
			avoid: []string{"mcp: disabled for default local pipeline"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.enabled {
				t.Setenv("DECK_ASK_ENABLE_AUGMENT", "1")
			}
			root := t.TempDir()
			writeLatestPlanArtifact(t, root)
			client := &stubClient{responses: []string{`{"summary":"generated starter workflows","review":[],"selection":{"targets":[{"path":"workflows/scenarios/apply.yaml","kind":"workflow","builders":[{"id":"apply.check-cluster","overrides":{"nodeCount":1}}]}]}}`}}
			if err := Execute(context.Background(), Options{Root: root, Prompt: "implement this plan", FromPath: ".deck/plan/latest.md", Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: io.Discard}, client); err != nil {
				t.Fatalf("execute: %v", err)
			}
			state, err := askstate.Load(root)
			if err != nil {
				t.Fatalf("load ask state: %v", err)
			}
			joined := strings.Join(state.LastAugmentEvents, "\n")
			for _, want := range tc.want {
				if !strings.Contains(joined, want) {
					t.Fatalf("expected augment event %q, got %#v", want, state.LastAugmentEvents)
				}
			}
			for _, avoid := range tc.avoid {
				if strings.Contains(joined, avoid) {
					t.Fatalf("did not expect augment event %q, got %#v", avoid, state.LastAugmentEvents)
				}
			}
		})
	}
}

func TestExecutePlanResumeInteractiveClarificationCanQuit(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "test-key")
	root := t.TempDir()
	planDir := filepath.Join(root, ".deck", "plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	json := `{"version":1,"request":"create workflow","intent":"draft","complexity":"complex","blockers":[],"targetOutcome":"generate files","assumptions":[],"openQuestions":[],"clarifications":[{"id":"topology.kind","question":"pick topology","kind":"enum","options":["single-node","multi-node"],"recommendedDefault":"single-node","blocksGeneration":true}],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"entry"}],"validationChecklist":["lint"]}`
	if err := os.WriteFile(filepath.Join(planDir, "latest.json"), []byte(json), 0o600); err != nil {
		t.Fatalf("write json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "latest.md"), []byte("md"), 0o600); err != nil {
		t.Fatalf("write md: %v", err)
	}
	originalProbe := interactiveSessionProbe
	interactiveSessionProbe = func(io.Reader, io.Writer) bool { return true }
	defer func() { interactiveSessionProbe = originalProbe }()
	stdout := &bytes.Buffer{}
	client := &stubClient{responses: []string{}}
	if err := Execute(context.Background(), Options{Root: root, Prompt: "implement this plan", FromPath: ".deck/plan/latest.md", Stdin: strings.NewReader("q\n"), Stdout: stdout, Stderr: io.Discard}, client); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "saved plan after interactive clarification exit") {
		t.Fatalf("expected interactive quit output, got %q", stdout.String())
	}
	stored, _, err := loadPlanArtifact(root, ".deck/plan/latest.json")
	if err != nil {
		t.Fatalf("load plan artifact: %v", err)
	}
	if stored.Clarifications[0].Answer != "" {
		t.Fatalf("expected unanswered clarification after quit, got %#v", stored.Clarifications)
	}
}

func TestExecutePlanResumeInteractiveClarificationCanContinue(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "test-key")
	root := t.TempDir()
	planDir := filepath.Join(root, ".deck", "plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	json := `{"version":1,"request":"create workflow","intent":"draft","complexity":"complex","blockers":[],"targetOutcome":"generate files","assumptions":[],"openQuestions":[],"clarifications":[{"id":"topology.kind","question":"pick topology","kind":"enum","options":["single-node","multi-node"],"recommendedDefault":"single-node","blocksGeneration":true}],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"entry"}],"validationChecklist":["lint"]}`
	if err := os.WriteFile(filepath.Join(planDir, "latest.json"), []byte(json), 0o600); err != nil {
		t.Fatalf("write json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "latest.md"), []byte("md"), 0o600); err != nil {
		t.Fatalf("write md: %v", err)
	}
	originalProbe := interactiveSessionProbe
	interactiveSessionProbe = func(io.Reader, io.Writer) bool { return true }
	defer func() { interactiveSessionProbe = originalProbe }()
	stdout := &bytes.Buffer{}
	client := &stubClient{responses: []string{`{"summary":"generated starter workflows","review":[],"selection":{"targets":[{"path":"workflows/scenarios/apply.yaml","kind":"workflow","builders":[{"id":"apply.check-cluster","overrides":{"nodeCount":1}}]}]}}`}}
	if err := Execute(context.Background(), Options{Root: root, Prompt: "implement this plan", FromPath: ".deck/plan/latest.md", Stdin: strings.NewReader("2\n"), Stdout: stdout, Stderr: io.Discard}, client); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "ask write: ok") || !strings.Contains(stdout.String(), "wrote:") {
		t.Fatalf("expected generation write after answering clarification, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "workflows/scenarios/apply.yaml") {
		t.Fatalf("expected generated scenario output after clarification, got %q", stdout.String())
	}
}

func TestValidateGenerationBlocksFilesOutsideClarifiedPlanTargets(t *testing.T) {
	gen := askcontract.GenerationResponse{}
	files := []askcontract.GeneratedFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: wait-runtime\n    kind: WaitForFile\n    spec:\n      path: /etc/containerd/config.toml\n      interval: 1s\n      timeout: 5s\n"}, {Path: "workflows/components/unplanned.yaml", Content: "steps:\n  - id: helper\n    kind: WaitForFile\n    spec:\n      path: /tmp/helper\n      interval: 1s\n      timeout: 5s\n"}}
	plan := askcontract.PlanResponse{Intent: "draft", EntryScenario: "workflows/scenarios/apply.yaml", Files: []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "create"}}, AuthoringBrief: askcontract.AuthoringBrief{TargetScope: "workspace", TargetPaths: []string{"workflows/scenarios/apply.yaml"}}}
	_, critic, err := validateGeneration(context.Background(), t.TempDir(), gen, files, askintent.Decision{Route: askintent.RouteDraft}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{})
	if err == nil {
		t.Fatalf("expected plan contract validation error")
	}
	if !strings.Contains(strings.Join(critic.Blocking, "\n"), "outside the clarified plan target paths") {
		t.Fatalf("expected plan target blocking, got %#v", critic)
	}
	if !containsTrimmed(critic.CoverageGaps, "workflows/components/unplanned.yaml") {
		t.Fatalf("expected coverage gap for unplanned file, got %#v", critic)
	}
}

func TestRestrictRepairTargetsToPlanFiltersUnplannedPaths(t *testing.T) {
	plan := askcontract.PlanResponse{AuthoringBrief: askcontract.AuthoringBrief{TargetPaths: []string{"workflows/prepare.yaml", "workflows/scenarios/apply.yaml"}}}
	paths := restrictRepairTargetsToPlan([]string{"workflows/prepare.yaml", "workflows/components/helper.yaml"}, plan)
	if len(paths) != 1 || paths[0] != "workflows/prepare.yaml" {
		t.Fatalf("expected only planned repair target, got %#v", paths)
	}
}

func TestValidateSemanticGenerationRefineRejectsUnplannedFile(t *testing.T) {
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [\"true\"]\n"}, {Path: "workflows/components/new.yaml", Content: "steps: []\n"}})
	plan := askcontract.PlanResponse{Files: []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "update"}}}
	err := validateSemanticGeneration(gen, askintent.Decision{Route: askintent.RouteRefine}, plan)
	if err == nil {
		t.Fatalf("expected refine semantic validation failure")
	}
}

func TestValidateSemanticGenerationDoesNotRequireVarsFromChecklistMentionOnly(t *testing.T) {
	gen := testMaterialized("draft", []askcontract.GeneratedFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: bootstrap\n    kind: InitKubeadm\n    spec:\n      outputJoinFile: /tmp/deck/join.txt\n"}})
	plan := askcontract.PlanResponse{
		EntryScenario: "workflows/scenarios/apply.yaml",
		Files:         []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "create"}},
		AuthoringBrief: askcontract.AuthoringBrief{
			TargetPaths: []string{"workflows/scenarios/apply.yaml"},
		},
		ValidationChecklist: []string{
			"Do not add prepare workflow, components, or vars unless later requested.",
		},
	}
	if err := validateSemanticGeneration(gen, askintent.Decision{Route: askintent.RouteDraft}, plan); err != nil {
		t.Fatalf("expected checklist mention alone not to require vars file, got %v", err)
	}
}

func TestSemanticCriticWarnsWhenTypedStepsRequestedButOnlyCommandUsed(t *testing.T) {
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [\"true\"]\n"}})
	plan := askcontract.PlanResponse{
		Request:             "create an air-gapped rhel9 single-node kubeadm workflow using typed steps where possible",
		TargetOutcome:       "Generate typed-step focused workflows",
		ValidationChecklist: []string{"Typed steps should be used where applicable"},
	}
	critic := semanticCritic(gen, askintent.Decision{Route: askintent.RouteRefine}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{})
	joined := strings.Join(critic.Advisory, "\n")
	if !strings.Contains(joined, "Prefer") && !strings.Contains(joined, "typed") {
		t.Fatalf("expected typed-step advisory, got %#v", critic)
	}
}

func TestSemanticCriticBlocksOfflineApplyWithDownloads(t *testing.T) {
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: fetch\n    kind: Command\n    spec:\n      command: [\"curl\",\"-L\",\"https://example.invalid/pkg.rpm\"]\n"}})
	plan := askcontract.PlanResponse{Request: "create a package installation workflow", OfflineAssumption: "offline"}
	critic := semanticCritic(gen, askintent.Decision{Route: askintent.RouteDraft}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{})
	if len(critic.Blocking) == 0 {
		t.Fatalf("expected offline apply blocking finding, got %#v", critic)
	}
}

func TestSemanticCriticRequiresPrepareForArtifactPlan(t *testing.T) {
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: install\n    kind: InstallPackage\n    spec:\n      packages: [kubeadm]\n"}})
	plan := askcontract.PlanResponse{Request: "create an air-gapped package workflow", OfflineAssumption: "offline", NeedsPrepare: true, ArtifactKinds: []string{"package"}}
	critic := semanticCritic(gen, askintent.Decision{Route: askintent.RouteDraft}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{})
	if len(critic.Blocking) == 0 || !strings.Contains(strings.Join(critic.Blocking, "\n"), "prepare") {
		t.Fatalf("expected prepare blocking finding, got %#v", critic)
	}
}

func TestSemanticCriticKeepsVarsAndComponentsAsAdvisory(t *testing.T) {
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: install\n    kind: InstallPackage\n    spec:\n      packages: [kubeadm]\n"}})
	plan := askcontract.PlanResponse{
		Request:                 "refine the workflow to reuse repeated local values",
		VarsRecommendation:      []string{"Use workflows/vars.yaml for repeated package, image, path, or version values."},
		ComponentRecommendation: []string{"Consider workflows/components/ for reusable repeated logic across phases or scenarios."},
	}
	critic := semanticCritic(gen, askintent.Decision{Route: askintent.RouteDraft}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{})
	if len(critic.Blocking) != 0 {
		t.Fatalf("expected only advisory findings, got %#v", critic)
	}
	joined := strings.Join(critic.Advisory, "\n")
	for _, want := range []string{"vars.yaml", "components/"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q advisory, got %#v", want, critic)
		}
	}
}

func TestSemanticCriticDetectsRepeatedValuesForVarsAdvisory(t *testing.T) {
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/prepare.yaml", Content: "version: v1alpha1\nsteps:\n  - id: download\n    kind: DownloadPackage\n    spec:\n      packages: [kubeadm]\n      outputDir: packages/kubernetes\n"}, {Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: install\n    kind: InstallPackage\n    spec:\n      packages: [kubeadm]\n      source:\n        type: local-repo\n        path: packages/kubernetes\n"}})
	critic := semanticCritic(gen, askintent.Decision{Route: askintent.RouteDraft}, askcontract.PlanResponse{}, askcontract.AuthoringBrief{}, askretrieve.RetrievalResult{})
	joined := strings.Join(critic.Advisory, "\n")
	if !strings.Contains(joined, "workflows/vars.yaml") {
		t.Fatalf("expected vars advisory from repeated values, got %#v", critic)
	}
}

func TestSemanticCriticDetectsRepeatedStepSequenceForComponentsAdvisory(t *testing.T) {
	content := "version: v1alpha1\nsteps:\n  - id: check\n    kind: CheckHost\n    spec:\n      checks: [os]\n  - id: verify\n    kind: CheckCluster\n    spec:\n      checks: [nodes_ready]\n"
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/prepare.yaml", Content: content}, {Path: "workflows/scenarios/apply.yaml", Content: content}})
	critic := semanticCritic(gen, askintent.Decision{Route: askintent.RouteDraft}, askcontract.PlanResponse{}, askcontract.AuthoringBrief{}, askretrieve.RetrievalResult{})
	joined := strings.Join(critic.Advisory, "\n")
	if !strings.Contains(joined, "workflows/components/") {
		t.Fatalf("expected component advisory from repeated sequence, got %#v", critic)
	}
}

func TestSemanticCriticBlocksVarsTemplateInConstrainedLiteralField(t *testing.T) {
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/prepare.yaml", Content: "version: v1alpha1\nsteps:\n  - id: prepare-download-kubernetes-packages\n    kind: DownloadPackage\n    spec:\n      packages: [kubeadm]\n      distro:\n        family: rhel\n        release: rocky9\n      repo:\n        type: rpm\n      backend:\n        mode: container\n        runtime: '{{ .vars.packageBackendRuntime }}'\n        image: rockylinux:9\n"}})
	critic := semanticCritic(gen, askintent.Decision{Route: askintent.RouteDraft}, askcontract.PlanResponse{}, askcontract.AuthoringBrief{}, askretrieve.RetrievalResult{})
	joined := strings.Join(critic.Blocking, "\n")
	if !strings.Contains(joined, "spec.backend.runtime") {
		t.Fatalf("expected constrained field violation, got %#v", critic)
	}
}

func TestSemanticCriticBlocksPrepareCommandForImageCollection(t *testing.T) {
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/prepare.yaml", Content: "version: v1alpha1\nsteps:\n  - id: pull-images\n    kind: Command\n    spec:\n      command: [\"bash\",\"-lc\",\"docker pull registry.k8s.io/kube-apiserver:v1.31.0 && docker save registry.k8s.io/kube-apiserver:v1.31.0 -o images/control-plane/apiserver.tar\"]\n"}})
	plan := askcontract.PlanResponse{ArtifactKinds: []string{"image"}, NeedsPrepare: true}
	critic := semanticCritic(gen, askintent.Decision{Route: askintent.RouteDraft}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{})
	joined := strings.Join(critic.Blocking, "\n")
	if !strings.Contains(joined, "typed prepare step") {
		t.Fatalf("expected prepare Command artifact blocking, got %#v", critic)
	}
}

func TestSemanticCriticBlocksWhenAuthoringBriefLosesPrepareApplyScope(t *testing.T) {
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [\"true\"]\n"}})
	plan := askcontract.PlanResponse{Request: "create prepare and apply workflows", AuthoringBrief: askcontract.AuthoringBrief{TargetScope: "workspace", ModeIntent: "prepare+apply", TargetPaths: []string{"workflows/prepare.yaml", "workflows/scenarios/apply.yaml"}}}
	critic := semanticCritic(gen, askintent.Decision{Route: askintent.RouteDraft}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{})
	joined := strings.Join(critic.Blocking, "\n")
	for _, want := range []string{"workflows/prepare.yaml", "prepare and apply"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in blocking findings, got %#v", want, critic)
		}
	}
}

func TestSemanticCriticTreatsMissingComponentTargetsAsAdvisoryForWorkspaceDrafts(t *testing.T) {
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/prepare.yaml", Content: "version: v1alpha1\nsteps:\n  - id: prepare-download\n    kind: DownloadPackage\n    spec:\n      packages: [kubeadm]\n"}, {Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: apply-install\n    kind: InstallPackage\n    spec:\n      packages: [kubeadm]\n      source:\n        type: local-repo\n        path: /tmp/deck/packages\n"}})
	plan := askcontract.PlanResponse{AuthoringBrief: askcontract.AuthoringBrief{TargetScope: "workspace", ModeIntent: "prepare+apply", TargetPaths: []string{"workflows/prepare.yaml", "workflows/scenarios/apply.yaml", "workflows/components/bootstrap/control-plane.yaml"}}}
	critic := semanticCritic(gen, askintent.Decision{Route: askintent.RouteDraft}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{})
	if strings.Contains(strings.Join(critic.Blocking, "\n"), "workflows/components/bootstrap/control-plane.yaml") {
		t.Fatalf("expected missing component target to avoid blocking, got %#v", critic)
	}
	if !strings.Contains(strings.Join(critic.Advisory, "\n"), "workflows/components/bootstrap/control-plane.yaml") {
		t.Fatalf("expected advisory for missing component target, got %#v", critic)
	}
}

func TestSemanticCriticTreatsMissingExecutionModelComponentsAsAdvisoryForWorkspaceDrafts(t *testing.T) {
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/prepare.yaml", Content: "version: v1alpha1\nsteps:\n  - id: prepare-download-images\n    kind: DownloadImage\n    spec:\n      images: [registry.k8s.io/kube-apiserver:v1.30.1]\n"}, {Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: apply-load-images\n    kind: LoadImage\n    spec:\n      runtime: ctr\n      images: [registry.k8s.io/kube-apiserver:v1.30.1]\n"}})
	plan := askcontract.PlanResponse{AuthoringBrief: askcontract.AuthoringBrief{TargetScope: "workspace", ModeIntent: "prepare+apply"}, ExecutionModel: askcontract.ExecutionModel{ArtifactContracts: []askcontract.ArtifactContract{{Kind: "image", ProducerPath: "workflows/components/prepare/images.yaml", ConsumerPath: "workflows/components/bootstrap/control-plane.yaml"}}}}
	critic := semanticCritic(gen, askintent.Decision{Route: askintent.RouteDraft}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{})
	joinedBlocking := strings.Join(critic.Blocking, "\n")
	for _, avoid := range []string{"workflows/components/prepare/images.yaml", "workflows/components/bootstrap/control-plane.yaml"} {
		if strings.Contains(joinedBlocking, avoid) {
			t.Fatalf("expected missing execution-model component target %q to avoid blocking, got %#v", avoid, critic)
		}
	}
	joinedAdvisory := strings.Join(critic.Advisory, "\n")
	for _, want := range []string{"workflows/components/prepare/images.yaml", "workflows/components/bootstrap/control-plane.yaml"} {
		if !strings.Contains(joinedAdvisory, want) {
			t.Fatalf("expected advisory for missing execution-model component target %q, got %#v", want, critic)
		}
	}
}

func TestSemanticCriticBlocksIncompleteKubeadmScenario(t *testing.T) {
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: preflight\n    kind: CheckHost\n    spec:\n      checks: [os, arch, swap]\n"}})
	plan := askcontract.PlanResponse{Request: "create an air-gapped rhel9 single-node kubeadm workflow", OfflineAssumption: "offline"}
	req := askpolicy.ScenarioRequirements{AcceptanceLevel: "refine", Connectivity: "offline", ScenarioIntent: []string{"kubeadm"}}
	eval := askpolicy.EvaluateGeneration(req, plan, gen)
	found := false
	for _, finding := range eval.Findings {
		if strings.Contains(finding.Message, "scenario intent") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected kubeadm scenario fidelity blocking, got %#v", eval)
	}
}

func TestRepoMapChunkIncludesImportsModeAndKinds(t *testing.T) {
	workspace := askretrieve.WorkspaceSummary{Files: []askretrieve.WorkspaceFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nphases:\n  - name: bootstrap\n    imports:\n      - path: bootstrap.yaml\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [\"true\"]\n"}}}
	chunk := repoMapChunk(workspace)
	for _, want := range []string{"imports=bootstrap.yaml", "steps=Command"} {
		if !strings.Contains(chunk.Content, want) {
			t.Fatalf("expected %q in repo map chunk, got %q", want, chunk.Content)
		}
	}
}

func TestPlanWorkspaceChunksIncludeImportedComponents(t *testing.T) {
	workspace := askretrieve.WorkspaceSummary{Files: []askretrieve.WorkspaceFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nphases:\n  - name: bootstrap\n    imports:\n      - path: bootstrap.yaml\n"}, {Path: "workflows/components/bootstrap.yaml", Content: "steps:\n  - id: run\n    kind: Command\n    spec:\n      command: [\"true\"]\n"}}}
	plan := askcontract.PlanResponse{Files: []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "update"}}}
	chunks := planWorkspaceChunks(plan, workspace)
	if len(chunks) < 2 {
		t.Fatalf("expected planned scenario and imported component chunks, got %d", len(chunks))
	}
}

func writeLatestPlanArtifact(t *testing.T, root string) {
	t.Helper()
	planDir := filepath.Join(root, ".deck", "plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	json := `{"version":1,"request":"create workflow","intent":"draft","complexity":"complex","authoringProgram":{"verification":{"expectedNodeCount":1,"expectedReadyCount":1,"expectedControlPlaneReady":1}},"blockers":[],"targetOutcome":"generate files","assumptions":[],"openQuestions":[],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"entry"}],"validationChecklist":["lint"]}`
	if err := os.WriteFile(filepath.Join(planDir, "latest.json"), []byte(json), 0o600); err != nil {
		t.Fatalf("write json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "latest.md"), []byte("md"), 0o600); err != nil {
		t.Fatalf("write md: %v", err)
	}
}
