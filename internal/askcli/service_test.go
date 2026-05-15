package askcli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askconfig"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askpolicy"
	"github.com/Airgap-Castaways/deck/internal/askprovider"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/askstate"
	"github.com/Airgap-Castaways/deck/internal/workflowissues"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

type stubClient struct {
	responses         []string
	providerResponses []askprovider.Response
	calls             int
	prompts           []askprovider.Request
}

type flushBuffer struct {
	bytes.Buffer
	flushes int
}

func (b *flushBuffer) Flush() error {
	b.flushes++
	return nil
}

func (s *stubClient) Generate(_ context.Context, req askprovider.Request) (askprovider.Response, error) {
	s.prompts = append(s.prompts, req)
	defer func() { s.calls++ }()
	if len(s.providerResponses) > 0 {
		idx := s.calls
		if idx >= len(s.providerResponses) {
			idx = len(s.providerResponses) - 1
		}
		return s.providerResponses[idx], nil
	}
	if len(s.responses) == 0 {
		return askprovider.Response{}, errors.New("no stub response configured")
	}
	idx := s.calls
	if idx >= len(s.responses) {
		idx = len(s.responses) - 1
	}
	return askprovider.Response{Content: s.responses[idx]}, nil
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
			"running CheckKubernetesCluster in both control-plane and worker flows is not realistic",
		},
		MissingContracts: []string{"join-file publication contract"},
	})
	if len(critic.Blocking) != 0 || len(critic.MissingContracts) != 0 {
		t.Fatalf("expected recoverable issues to downgrade, got %#v", critic)
	}
	joined := strings.Join(critic.Advisory, "\n")
	for _, want := range []string{"CheckKubernetesCluster"} {
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
			"Execution model / workflows/scenarios/apply.yaml: Final CheckKubernetesCluster on the control-plane can race worker joins because apply is described as per-node role-based execution with no barrier or wait contract",
			"workflows/prepare.yaml -> workflows/scenarios/apply.yaml: The offline image artifact contract is underspecified. The plan does not define the produced image format/bundle layout or the exact path apply LoadImage consumes",
		},
		MissingContracts: []string{
			"Execution model: Synchronization contract between worker joins and the final control-plane CheckKubernetesCluster step.",
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
		{Code: workflowissues.CodeWeakVerificationStaging, Severity: workflowissues.SeverityAdvisory, Message: "final CheckKubernetesCluster should wait for worker join completion", Recoverable: true},
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
		"Execution model / workflows/scenarios/apply.yaml: Final CheckKubernetesCluster on the control-plane can race worker joins because apply is described as per-node role-based execution with no barrier or wait contract",
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
	critic := askcontract.PlanCriticResponse{Findings: []askcontract.PlanCriticFinding{{Code: workflowissues.CodeRoleCardinalityGap, Severity: workflowissues.SeverityMissingContract, Message: "single-node role cardinality is not explicit"}, {Code: workflowissues.CodeWeakVerificationStaging, Severity: workflowissues.SeverityAdvisory, Message: "single CheckKubernetesCluster-only flow is weakly staged"}}}
	if hasFatalPlanReviewIssues(plan, critic) {
		t.Fatalf("expected simple single-node verification plan to continue past recoverable critic findings")
	}
}

func TestBuildPlanWithReviewFallsBackOnPlannerFailure(t *testing.T) {
	client := &stubClient{responses: []string{"not-json"}}
	prompt := "Create a minimal single-node apply-only kubeadm workflow using InitKubeadm and CheckKubernetesCluster"
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

func TestBuildPlanWithReviewBuildsViableArtifactFallbackPlan(t *testing.T) {
	client := &stubClient{responses: []string{"not-json"}}
	prompt := "Create an offline RHEL 9 kubeadm workflow for exactly 2 nodes: 1 control-plane and 1 worker. Generate both workflows/prepare.yaml and workflows/scenarios/apply.yaml. In prepare, stage kubeadm kubelet kubectl cri-tools containerd packages and Kubernetes control-plane images using typed steps only. In apply, use vars.role with allowed values control-plane and worker, bootstrap the control-plane with InitKubeadm writing /tmp/deck/join.txt, join the worker with JoinKubeadm using that same file, and run final CheckKubernetesCluster only on the control-plane expecting total 2 nodes and controlPlaneReady 1. Do not use remote downloads during apply."
	req := askpolicy.BuildScenarioRequirements(prompt, askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft})
	plan, _, usedFallback, err := buildPlanWithReview(context.Background(), client, askconfigSettings{provider: "openai", model: "gpt-5.4", apiKey: "test-key"}, askintent.Decision{Route: askintent.RouteDraft}, askretrieve.RetrievalResult{}, prompt, askretrieve.WorkspaceSummary{}, req, newAskLogger(io.Discard, "trace"))
	if err != nil {
		t.Fatalf("expected viable artifact fallback plan, got %v", err)
	}
	if !usedFallback {
		t.Fatalf("expected fallback path")
	}
	if len(plan.AuthoringProgram.Artifacts.Packages) == 0 || len(plan.AuthoringProgram.Artifacts.Images) == 0 {
		t.Fatalf("expected artifact fallback plan to seed package/image staging, got %#v", plan.AuthoringProgram.Artifacts)
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
	summary, answer := localExplain(askretrieve.WorkspaceSummary{}, "Explain how InitKubeadm and CheckKubernetesCluster are assembled for ask draft generation in this repo", askintent.Target{Kind: "workspace"})
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

func TestExplainAndReviewSystemPromptsIncludeParsedWorkflowSummaries(t *testing.T) {
	retrieval := askretrieve.RetrievalResult{Chunks: []askretrieve.Chunk{{Label: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nphases:\n  - name: bootstrap\n    steps:\n      - id: run\n        kind: Command\n        spec:\n          command: [true]\n"}}}
	for _, prompt := range []string{buildInfoSystemPrompt(askintent.RouteExplain, askintent.Target{Kind: "scenario", Path: "workflows/scenarios/apply.yaml"}, retrieval, askretrieve.WorkspaceSummary{}), buildInfoSystemPrompt(askintent.RouteReview, askintent.Target{Kind: "scenario", Path: "workflows/scenarios/apply.yaml"}, retrieval, askretrieve.WorkspaceSummary{})} {
		for _, want := range []string{"Parsed workflow summaries:", "workflows/scenarios/apply.yaml [workflow] phases=1 top-level-steps=0"} {
			if !strings.Contains(prompt, want) {
				t.Fatalf("expected %q in info prompt, got %q", want, prompt)
			}
		}
	}
}

func TestExplainSystemPromptUsesCodePathModeForRepoBehavior(t *testing.T) {
	retrieval := askretrieve.RetrievalResult{Chunks: []askretrieve.Chunk{{ID: "local-facts-stepmeta", Source: "local-facts", Label: "source-of-truth-stepmeta", Content: "Local facts:\n- path: internal/stepmeta/registry.go"}, {ID: "local-facts-askdraft", Source: "local-facts", Label: "askdraft-compiler", Content: "Local facts:\n- file: internal/askdraft/draft.go\n- function: CompileWithProgram"}, {ID: "workspace-apply", Source: "workspace", Label: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps: []\n"}}}
	prompt := buildInfoSystemPrompt(askintent.RouteExplain, askintent.Target{Kind: "workspace"}, retrieval, askretrieve.WorkspaceSummary{})
	for _, want := range []string{"explaining how this repository assembles workflow behavior", "internal/stepmeta", "internal/stepspec", "registry/metadata -> builder selection -> binding resolution -> workflow document compilation"} {
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
	prompt := buildInfoSystemPrompt(askintent.RouteReview, askintent.Target{Kind: "scenario", Path: "workflows/scenarios/apply.yaml"}, askretrieve.RetrievalResult{}, workspace)
	for _, want := range []string{"Structured validation issues:", "sourceDir", "Additional property sourceDir is not allowed"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in review prompt, got %q", want, prompt)
		}
	}
}

func TestReviewSystemPromptIgnoresComponentAndVarsValidationIssues(t *testing.T) {
	workspace := askretrieve.WorkspaceSummary{Files: []askretrieve.WorkspaceFile{
		{Path: "workflows/components/k8s/runtime.yaml", Content: "steps:\n  - id: install\n    kind: InstallPackage\n    spec:\n      packages: \"{{ .vars.runtimePackages }}\"\n"},
		{Path: "workflows/vars.yaml", Content: "runtimePackages: [containerd]\n"},
	}}
	prompt := buildInfoSystemPrompt(askintent.RouteReview, askintent.Target{Kind: "workspace"}, askretrieve.RetrievalResult{}, workspace)
	if strings.Contains(prompt, "Structured validation issues:") {
		t.Fatalf("expected review prompt to avoid context-free component and vars validation issues, got %q", prompt)
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
	json := `{"version":1,"request":"create a 3-node offline kubeadm workflow with prepare and apply phases","intent":"draft","complexity":"complex","blockers":[],"targetOutcome":"generate files","assumptions":[],"openQuestions":[],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"entry"}],"validationChecklist":["lint"]}`
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

func TestApplyPlanAnswersPreservesNonCodeOwnedClarificationIDs(t *testing.T) {
	root := t.TempDir()
	plan := askcontract.PlanResponse{
		Version:       1,
		Request:       "create 3-node kubeadm workflow",
		Intent:        "draft",
		Complexity:    "complex",
		TargetOutcome: "generate files",
		Files: []askcontract.PlanFile{
			{Path: "workflows/prepare.yaml", Kind: "workflow", Action: "create", Purpose: "prepare"},
			{Path: "workflows/scenarios/apply.yaml", Kind: "scenario", Action: "create", Purpose: "apply"},
		},
		Clarifications: []askcontract.PlanClarification{
			{ID: "artifact-publish-model", Question: "How should prepared offline artifacts be made available during apply?", Kind: "choice", Options: []string{"local-http-server", "portable-bundle"}, RecommendedDefault: "local-http-server"},
			{ID: "topology.roleModel", Question: "pick role model", Kind: "enum", Options: []string{"1cp-2workers", "3cp-ha"}, BlocksGeneration: true},
			{ID: "kubernetes-version", Question: "Which Kubernetes version should be installed?", Kind: "string", RecommendedDefault: "v1.30.0"},
		},
	}

	updated, err := applyPlanAnswers(plan, []string{"topology.roleModel=1cp-2workers"})
	if err != nil {
		t.Fatalf("apply plan answers: %v", err)
	}
	ids := map[string]bool{}
	for _, item := range updated.Clarifications {
		if item.ID == "" {
			t.Fatalf("expected clarification ids to be preserved, got %#v", updated.Clarifications)
		}
		ids[item.ID] = true
	}
	for _, want := range []string{"artifact-publish-model", "topology.roleModel", "kubernetes-version"} {
		if !ids[want] {
			t.Fatalf("expected clarification %q after normalization, got %#v", want, updated.Clarifications)
		}
	}
	planMD := renderPlanMarkdown(updated, ".deck/plan/latest.md")
	if _, _, err := savePlanArtifact(root, Options{PlanDir: ".deck/plan"}, updated, planMD); err != nil {
		t.Fatalf("save plan artifact: %v", err)
	}
	loaded, _, err := loadPlanArtifact(root, ".deck/plan/latest.json")
	if err != nil {
		t.Fatalf("load plan artifact: %v", err)
	}
	loadedIDs := map[string]bool{}
	for _, item := range loaded.Clarifications {
		loadedIDs[item.ID] = true
	}
	for _, want := range []string{"artifact-publish-model", "topology.roleModel", "kubernetes-version"} {
		if !loadedIDs[want] {
			t.Fatalf("expected saved clarification %q, got %#v", want, loaded.Clarifications)
		}
	}
}

func TestExecutePlanResumeWithAnswersUsesSavedPlanRoute(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "test-key")
	root := t.TempDir()
	planDir := filepath.Join(root, ".deck", "plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	json := `{"version":1,"request":"create workflow","intent":"draft","complexity":"complex","blockers":[],"targetOutcome":"generate files","assumptions":[],"openQuestions":[],"clarifications":[{"id":"topology.roleModel","question":"The request implies multiple nodes, but the role model is unclear. Which node role layout should the plan use?","kind":"enum","options":["1cp-2workers","3cp-ha"],"blocksGeneration":true}],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/prepare.yaml","kind":"workflow","action":"create","purpose":"prepare"},{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"entry"}],"validationChecklist":["lint"]}`
	if err := os.WriteFile(filepath.Join(planDir, "latest.json"), []byte(json), 0o600); err != nil {
		t.Fatalf("write json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "latest.md"), []byte("md"), 0o600); err != nil {
		t.Fatalf("write md: %v", err)
	}
	stdout := &bytes.Buffer{}
	if err := Execute(context.Background(), Options{Root: root, FromPath: ".deck/plan/latest.json", Answers: []string{"topology.roleModel=1cp-2workers"}, PlanOnly: true, Stdout: stdout, Stderr: io.Discard}, &stubClient{}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "updated plan artifact") {
		t.Fatalf("expected updated plan artifact output, got %q", stdout.String())
	}
	stored, _, err := loadPlanArtifact(root, ".deck/plan/latest.json")
	if err != nil {
		t.Fatalf("load plan artifact: %v", err)
	}
	answered := false
	for _, item := range stored.Clarifications {
		if strings.TrimSpace(item.Answer) == "1cp-2workers" {
			answered = true
			break
		}
	}
	if !answered {
		t.Fatalf("expected saved plan to retain answered clarification, got %#v", stored.Clarifications)
	}
	if stored.Intent != "draft" {
		t.Fatalf("expected saved plan route to stay draft, got %#v", stored)
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
	if !strings.Contains(stdout.String(), "authoring needs clarification") {
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
				"mcp: skipped until requested in authoring tool loop",
			},
			avoid: []string{"mcp:web-search initialize failed:"},
		},
		{
			name:    "enabled-via-env",
			enabled: true,
			want: []string{
				"mcp: skipped until requested in authoring tool loop",
			},
			avoid: []string{"mcp:web-search initialize failed:"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.enabled {
				t.Setenv("DECK_ASK_ENABLE_AUGMENT", "1")
			}
			root := t.TempDir()
			writeLatestPlanArtifact(t, root)
			client := &stubClient{providerResponses: agentWriteLintFinishResponses(t, askcontract.GeneratedFile{Path: workspacepaths.CanonicalApplyWorkflow, Content: waitForHostsWorkflow("5s")})}
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
	if !strings.Contains(stdout.String(), "authoring clarification stopped") {
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
	client := &stubClient{providerResponses: agentWriteLintFinishResponses(t, askcontract.GeneratedFile{Path: workspacepaths.CanonicalApplyWorkflow, Content: waitForHostsWorkflow("5s")})}
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

func TestValidateGenerationAllowsPreserveOnlyRefineNoop(t *testing.T) {
	gen := askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{Path: "workflows/scenarios/worker-join.yaml", Action: "preserve"}}}
	plan := askcontract.PlanResponse{Intent: "refine", Files: []askcontract.PlanFile{{Path: "workflows/scenarios/worker-join.yaml", Action: "update"}}, AuthoringBrief: askcontract.AuthoringBrief{TargetPaths: []string{"workflows/scenarios/worker-join.yaml"}}}
	summary, critic, err := validateGeneration(context.Background(), t.TempDir(), gen, nil, askintent.Decision{Route: askintent.RouteRefine}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{})
	if err != nil {
		t.Fatalf("expected preserve-only refine noop to validate, got %v", err)
	}
	if summary == "" || len(critic.Blocking) != 0 {
		t.Fatalf("expected noop validation success, got summary=%q critic=%#v", summary, critic)
	}
}

func TestValidateGenerationRefineAllowsUntouchedExistingComponentImports(t *testing.T) {
	root := t.TempDir()
	componentDir := filepath.Join(root, "workflows", "components", "k8s")
	scenarioDir := filepath.Join(root, "workflows", "scenarios")
	if err := os.MkdirAll(componentDir, 0o755); err != nil {
		t.Fatalf("mkdir components: %v", err)
	}
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatalf("mkdir scenarios: %v", err)
	}
	if err := os.WriteFile(filepath.Join(componentDir, "prereq.yaml"), []byte("steps:\n  - id: prereq\n    kind: CheckHost\n    spec:\n      checks: [os]\n"), 0o644); err != nil {
		t.Fatalf("write component: %v", err)
	}
	files := []askcontract.GeneratedFile{
		{Path: "workflows/scenarios/control-plane-bootstrap.yaml", Content: "version: v1alpha1\nphases:\n  - name: host-prereqs\n    imports:\n      - path: k8s/prereq.yaml\n  - name: verify\n    steps:\n      - id: bootstrap-report\n        kind: CheckKubernetesCluster\n        spec:\n          interval: 5s\n          timeout: 10m\n          nodes:\n            total: 1\n            ready: 1\n            controlPlaneReady: 1\n"},
		{Path: "workflows/vars.yaml", Content: "clusterName: bootstrap-root\n"},
	}
	gen := testMaterialized("refine", files)
	plan := askcontract.PlanResponse{Intent: "refine", Files: []askcontract.PlanFile{{Path: "workflows/scenarios/control-plane-bootstrap.yaml", Action: "update"}, {Path: "workflows/vars.yaml", Action: "update"}}, AuthoringBrief: askcontract.AuthoringBrief{TargetPaths: []string{"workflows/scenarios/control-plane-bootstrap.yaml", "workflows/vars.yaml"}}}
	summary, critic, err := validateGeneration(context.Background(), root, gen, files, askintent.Decision{Route: askintent.RouteRefine}, plan, plan.AuthoringBrief, askretrieve.RetrievalResult{})
	if err != nil {
		t.Fatalf("expected refine validation with existing imports to pass, got err=%v critic=%#v", err, critic)
	}
	if summary == "" || len(critic.Blocking) != 0 {
		t.Fatalf("expected successful refine validation, got summary=%q critic=%#v", summary, critic)
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

func TestSemanticCriticRefineDoesNotRequireUntouchedImportedComponentsToBeRegenerated(t *testing.T) {
	gen := testMaterialized("", []askcontract.GeneratedFile{{Path: "workflows/scenarios/control-plane-bootstrap.yaml", Content: "version: v1alpha1\nphases:\n  - name: host-prereqs\n    imports:\n      - path: k8s/prereq.yaml\n  - name: verify\n    steps:\n      - id: bootstrap-report\n        kind: CheckKubernetesCluster\n        spec:\n          interval: 5s\n          timeout: 10m\n          nodes:\n            total: 1\n            ready: 1\n            controlPlaneReady: 1\n"}, {Path: "workflows/vars.yaml", Content: "clusterName: bootstrap-root\n"}})
	critic := semanticCritic(gen, askintent.Decision{Route: askintent.RouteRefine}, askcontract.PlanResponse{}, askcontract.AuthoringBrief{}, askretrieve.RetrievalResult{})
	if strings.Contains(strings.Join(critic.Blocking, "\n"), "scenario imports missing component") {
		t.Fatalf("expected refine semantic critic not to require untouched imported components, got %#v", critic)
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
	if len(critic.Advisory) == 0 || !strings.Contains(strings.Join(critic.Advisory, "\n"), "prepare") {
		t.Fatalf("expected prepare advisory finding, got %#v", critic)
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
	content := "version: v1alpha1\nsteps:\n  - id: check\n    kind: CheckHost\n    spec:\n      checks: [os]\n  - id: verify\n    kind: CheckKubernetesCluster\n    spec:\n      checks: [nodes_ready]\n"
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
