package prepare

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/taedi90/deck/internal/config"
)

type PlanDiagnostics struct {
	ArtifactGroups []ArtifactGroupDiagnostic
	CachePlan      PackCachePlan
}

type ArtifactGroupDiagnostic struct {
	Kind        string
	Name        string
	Jobs        int
	Parallelism int
	Retry       int
}

func InspectPlan(wf *config.Workflow, bundleRoot string, opts RunOptions) (PlanDiagnostics, error) {
	if wf == nil {
		return PlanDiagnostics{}, fmt.Errorf("workflow is nil")
	}
	diagnostics := PlanDiagnostics{}
	groups, err := planArtifactJobGroups(wf, bundleRoot, opts)
	if err != nil {
		return diagnostics, err
	}
	diagnostics.ArtifactGroups = make([]ArtifactGroupDiagnostic, 0, len(groups))
	for _, group := range groups {
		diagnostics.ArtifactGroups = append(diagnostics.ArtifactGroups, ArtifactGroupDiagnostic{
			Kind:        strings.TrimSpace(group.Kind),
			Name:        strings.TrimSpace(group.Name),
			Jobs:        len(group.Jobs),
			Parallelism: group.Execution.Parallelism,
			Retry:       group.Execution.Retry,
		})
	}
	if strings.TrimSpace(wf.Role) != "prepare" {
		return diagnostics, nil
	}
	prepareSteps, err := prepareExecutionSteps(wf)
	if err != nil {
		return diagnostics, err
	}
	workflowSHA := strings.TrimSpace(wf.WorkflowSHA256)
	if workflowSHA == "" {
		fallbackBytes, err := json.Marshal(wf)
		if err != nil {
			return diagnostics, fmt.Errorf("encode workflow for prepare cache: %w", err)
		}
		workflowSHA = computeWorkflowSHA256(fallbackBytes)
	}
	statePath, err := defaultPackCacheStatePath(workflowSHA)
	if err != nil {
		return diagnostics, fmt.Errorf("resolve prepare cache state path: %w", err)
	}
	prevState, err := loadPackCacheState(statePath)
	if err != nil {
		return diagnostics, err
	}
	workflowBytes, err := json.Marshal(wf)
	if err != nil {
		return diagnostics, fmt.Errorf("encode workflow for prepare cache plan: %w", err)
	}
	diagnostics.CachePlan = ComputePackCachePlan(prevState, workflowBytes, wf.Vars, prepareSteps)
	diagnostics.CachePlan.WorkflowSHA256 = workflowSHA
	return diagnostics, nil
}
