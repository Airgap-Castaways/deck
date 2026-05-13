package askpolicy

import (
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askdefaults"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

type ContractGraph struct {
	Artifacts        []askcontract.ArtifactContract
	SharedState      []askcontract.SharedStateContract
	RoleExecution    askcontract.RoleExecutionModel
	Verification     askcontract.VerificationStrategy
	ApplyAssumptions []string
}

type RequirementLike struct {
	Connectivity   string
	NeedsPrepare   bool
	ArtifactKinds  []string
	ScenarioIntent []string
	RequiredFiles  []string
	EntryScenario  string
}

func BuildContractGraph(facts Facts, req RequirementLike, workspace askretrieve.WorkspaceSummary) ContractGraph {
	graph := ContractGraph{}
	consumerPath := strings.TrimSpace(req.EntryScenario)
	if consumerPath == "" {
		consumerPath = workspacepaths.CanonicalApplyWorkflow
	}
	if req.NeedsPrepare || len(req.ArtifactKinds) > 0 || workspace.HasPrepare {
		for _, kind := range req.ArtifactKinds {
			switch strings.TrimSpace(strings.ToLower(kind)) {
			case "package":
				graph.Artifacts = append(graph.Artifacts, askcontract.ArtifactContract{Kind: "package", ProducerPath: workspacepaths.CanonicalPrepareWorkflow, ConsumerPath: consumerPath, Description: "prepare downloads package content and apply installs it from a local repository path"})
			case "image":
				graph.Artifacts = append(graph.Artifacts, askcontract.ArtifactContract{Kind: "image", ProducerPath: workspacepaths.CanonicalPrepareWorkflow, ConsumerPath: consumerPath, Description: "prepare downloads container images and apply loads them from a local image bundle path"})
			case "repository-mirror":
				graph.Artifacts = append(graph.Artifacts, askcontract.ArtifactContract{Kind: "repository-setup", ProducerPath: workspacepaths.CanonicalPrepareWorkflow, ConsumerPath: consumerPath, Description: "prepare stages repository content and apply configures the node to consume it locally"})
			}
		}
	}
	if facts.MultiRoleRequested || facts.Topology == "multi-node" || facts.Topology == "ha" || facts.NodeCount > 1 {
		graph.SharedState = append(graph.SharedState, askcontract.SharedStateContract{Name: "join-file", ProducerPath: askdefaults.JoinFile, ConsumerPaths: []string{askdefaults.JoinFile}, AvailabilityModel: "published-for-worker-consumption", Description: "bootstrap flow publishes join data before join flows consume it"})
		graph.RoleExecution = askcontract.RoleExecutionModel{RoleSelector: "vars.role", ControlPlaneFlow: "preflight -> runtime setup -> bootstrap", WorkerFlow: "preflight -> runtime setup -> join", PerNodeInvocation: true}
	}
	graph.Verification = askcontract.VerificationStrategy{BootstrapPhase: "bootstrap", FinalPhase: "verify-cluster", FinalVerificationRole: "control-plane"}
	if facts.NodeCount > 0 {
		graph.Verification.ExpectedNodeCount = facts.NodeCount
	}
	if facts.Topology == "ha" && facts.ControlPlaneCount > 0 {
		graph.Verification.ExpectedControlPlaneReady = facts.ControlPlaneCount
	} else if facts.MultiRoleRequested || facts.Topology == "multi-node" || facts.Topology == "ha" {
		graph.Verification.ExpectedControlPlaneReady = 1
	}
	if facts.Topology == "single-node" {
		graph.Verification.ExpectedNodeCount = 1
		graph.Verification.ExpectedControlPlaneReady = 1
	}
	if strings.TrimSpace(facts.Connectivity) == "offline" {
		graph.ApplyAssumptions = append(graph.ApplyAssumptions, "apply consumes local artifacts and avoids live downloads")
	}
	return graph
}

func (g ContractGraph) ExecutionModel() askcontract.ExecutionModel {
	return askcontract.ExecutionModel{
		ArtifactContracts:    append([]askcontract.ArtifactContract(nil), g.Artifacts...),
		SharedStateContracts: append([]askcontract.SharedStateContract(nil), g.SharedState...),
		RoleExecution:        g.RoleExecution,
		Verification:         g.Verification,
		ApplyAssumptions:     append([]string(nil), g.ApplyAssumptions...),
	}
}
