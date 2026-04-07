package validate

import (
	"github.com/Airgap-Castaways/deck/internal/workflowcontract"
	"github.com/Airgap-Castaways/deck/internal/workflowissues"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

var workflowTopLevelModes = []string{"phases", "steps"}

const (
	workflowSupportedVersion = workflowcontract.SupportedWorkflowVersion
	workflowImportRule       = "Imports are only supported under phases[].imports and resolve from " + workspacepaths.CanonicalComponentsDir + "/."
)

func SupportedWorkflowRoles() []string {
	return []string{"prepare", "apply"}
}

func SupportedWorkflowVersion() string {
	return workflowSupportedVersion
}

func WorkflowTopLevelModes() []string {
	out := make([]string, len(workflowTopLevelModes))
	copy(out, workflowTopLevelModes)
	return out
}

func WorkflowImportRule() string {
	return workflowImportRule
}

func WorkflowInvariantNotes() []string {
	return []string{
		"A workflow must define at least one of phases or steps.",
		"A workflow cannot define both top-level phases and top-level steps at the same time.",
		workflowissues.MustSpec(workflowissues.CodeDuplicateStepID).Details,
		workflowImportRule,
		"Workflow mode is determined by command context or file location, not by an in-file role field.",
		"Command is an escape hatch. Prefer typed steps for service, filesystem, archive, sysctl, swap, kernel-module, and symlink actions when deck already models them.",
		"Each step still validates against its own kind-specific schema after the top-level workflow schema passes.",
	}
}
