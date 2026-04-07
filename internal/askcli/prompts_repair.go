package askcli

import (
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askdiagnostic"
)

func documentRepairSystemPrompt(brief askcontract.AuthoringBrief, plan askcontract.PlanResponse) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask document repair assistant. Return strict JSON only using the document generation response shape.\n")
	b.WriteString("JSON shape: {\"summary\":string,\"review\":[]string,\"documents\":[{\"path\":string,\"kind\":string,\"action\":string,\"workflow\":object?,\"component\":object?,\"vars\":object?,\"edits\":[]object?}]}. documents must contain at least one revised document.\n")
	b.WriteString("Refine repair may use structured transforms when a code-owned operation is more reliable than open-ended edits. Supported transforms: {type: extract-var, candidate: string, varName?: string, varsPath?: string, value: any}, {type: set-field, candidate: string, value: any}, {type: delete-field, candidate: string}, {type: extract-component, candidate: string, path?: string}. Use rawPath only when no candidate id exists.\n")
	b.WriteString("Repair document structure and schema issues with the smallest possible edits. Do not redesign the workflow unless a validator message explicitly requires it.\n")
	b.WriteString("Keep preserve-if-valid documents byte-for-byte identical after rendering. Revise only documents implicated by the parse or schema error when possible.\n")
	b.WriteString("Return only revised documents when possible; unchanged rendered files will be preserved by the caller.\n")
	b.WriteString("Every rendered workflow file must stay standalone-valid and preserve existing structure unless the validator requires a targeted change.\n")
	b.WriteString(authoringBriefPromptBlock(brief))
	b.WriteString("\n")
	b.WriteString(authoringProgramPromptBlock(plan.AuthoringProgram))
	b.WriteString("\n")
	b.WriteString(executionModelPromptBlock(plan.ExecutionModel))
	b.WriteString("\n")
	return b.String()
}

func documentRepairUserPrompt(prevFiles []askcontract.GeneratedFile, validation string, diags []askdiagnostic.Diagnostic, repairPaths []string) string {
	b := &strings.Builder{}
	b.WriteString("Repair these generated documents without redesigning them. Return only the revised documents if possible.\n")
	b.WriteString("Do not introduce new step kinds, new workflow files, or new execution contracts unless the validator error explicitly requires them.\n")
	b.WriteString("Focus only on the affected file paths named by the validator.\n")
	if len(diags) == 0 {
		b.WriteString("Validator summary:\n")
		b.WriteString(summarizeValidationError(validation))
		b.WriteString("\nRaw validator error:\n")
		b.WriteString(strings.TrimSpace(validation))
		b.WriteString("\n")
	} else {
		b.WriteString("Structured validator findings:\n")
		for _, diag := range diags {
			b.WriteString("- ")
			b.WriteString(strings.TrimSpace(diag.Message))
			if strings.TrimSpace(diag.Path) != "" {
				b.WriteString(" path=")
				b.WriteString(strings.TrimSpace(diag.Path))
			}
			if strings.TrimSpace(diag.RepairOp) != "" {
				b.WriteString(" op=")
				b.WriteString(strings.TrimSpace(diag.RepairOp))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString(askdiagnostic.RepairPromptBlock(diags))
	b.WriteString("\n")
	b.WriteString(repairOperationPromptBlock(diags))
	b.WriteString("\n")
	b.WriteString(documentStructureRepairPromptBlock(prevFiles, validation, repairPaths))
	b.WriteString("\n")
	b.WriteString(targetedRepairPromptBlock(prevFiles, diags, repairPaths))
	return strings.TrimSpace(b.String())
}

func repairOperationPromptBlock(diags []askdiagnostic.Diagnostic) string {
	ops := map[string][]string{}
	for _, diag := range diags {
		op := strings.TrimSpace(diag.RepairOp)
		if op == "" {
			continue
		}
		detail := strings.TrimSpace(diag.Path)
		if detail == "" {
			detail = strings.TrimSpace(diag.Message)
		}
		if strings.TrimSpace(diag.StepKind) != "" {
			detail = strings.TrimSpace(diag.StepKind) + " " + detail
		}
		ops[op] = append(ops[op], detail)
	}
	if len(ops) == 0 {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString("Suggested repair operations:\n")
	for _, op := range repairOperationOrder {
		items := dedupe(ops[op])
		if len(items) == 0 {
			continue
		}
		b.WriteString("- ")
		b.WriteString(op)
		b.WriteString(": ")
		b.WriteString(strings.Join(items, ", "))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
