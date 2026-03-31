package askrepair

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askdiagnostic"
	"github.com/Airgap-Castaways/deck/internal/askir"
)

func TryAutoRepair(root string, files []askcontract.GeneratedFile, diags []askdiagnostic.Diagnostic, repairPaths []string) ([]askcontract.GeneratedFile, []string, bool, error) {
	_ = repairPaths
	if len(files) == 0 || len(diags) == 0 {
		return files, nil, false, nil
	}
	editDocs := map[string][]askcontract.StructuredEditAction{}
	replaceDocs := map[string]askcontract.GeneratedDocument{}
	notes := []string{}
	for _, diag := range diags {
		path := diagnosticFile(diag)
		if path == "" {
			continue
		}
		if _, replaced := replaceDocs[path]; replaced {
			continue
		}
		switch strings.TrimSpace(diag.RepairOp) {
		case "rename-step":
			replacement, note, ok := renameDuplicateStepIDs(path, files)
			if !ok {
				continue
			}
			replaceDocs[path] = replacement
			notes = append(notes, note)
		case "remove-field":
			rawPath := repairRawPath(diag)
			if rawPath == "" {
				continue
			}
			editDocs[path] = append(editDocs[path], askcontract.StructuredEditAction{Op: "delete", RawPath: rawPath})
			notes = append(notes, fmt.Sprintf("removed unsupported field %s in %s", rawPath, path))
		case "fill-field":
			rawPath := repairRawPath(diag)
			value, ok := defaultFillValue(diag, files)
			if rawPath == "" || !ok {
				continue
			}
			editDocs[path] = append(editDocs[path], askcontract.StructuredEditAction{Op: "set", RawPath: rawPath, Value: value})
			notes = append(notes, fmt.Sprintf("filled %s in %s", rawPath, path))
		case "fix-literal":
			rawPath := repairRawPath(diag)
			if rawPath == "" || len(diag.Allowed) == 0 {
				continue
			}
			editDocs[path] = append(editDocs[path], askcontract.StructuredEditAction{Op: "set", RawPath: rawPath, Value: strings.TrimSpace(diag.Allowed[0])})
			notes = append(notes, fmt.Sprintf("set constrained literal %s in %s", rawPath, path))
		}
	}
	if len(editDocs) == 0 && len(replaceDocs) == 0 {
		return files, nil, false, nil
	}
	documents := make([]askcontract.GeneratedDocument, 0, len(replaceDocs)+len(editDocs))
	for _, doc := range replaceDocs {
		documents = append(documents, doc)
	}
	for path, edits := range editDocs {
		if _, replaced := replaceDocs[path]; replaced || len(edits) == 0 {
			continue
		}
		documents = append(documents, askcontract.GeneratedDocument{Path: path, Action: "edit", Edits: edits})
	}
	repaired, err := askir.MaterializeWithBase(root, files, askcontract.GenerationResponse{Documents: documents})
	if err != nil {
		return files, nil, false, err
	}
	return repaired, dedupeStrings(notes), true, nil
}

func diagnosticFile(diag askdiagnostic.Diagnostic) string {
	for _, value := range []string{diag.File, diag.Path} {
		clean := filepath.ToSlash(strings.TrimSpace(value))
		if strings.HasPrefix(clean, "workflows/") {
			if idx := strings.Index(clean, ":"); idx > 0 {
				clean = strings.TrimSpace(clean[:idx])
			}
			return clean
		}
	}
	return ""
}

func repairRawPath(diag askdiagnostic.Diagnostic) string {
	path := normalizeRepairPath(diag)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "steps[") || strings.HasPrefix(path, "phases[") || strings.HasPrefix(path, "vars.") {
		return path
	}
	if strings.TrimSpace(diag.StepID) != "" {
		return "steps." + strings.TrimSpace(diag.StepID) + "." + path
	}
	return path
}

func normalizeRepairPath(diag askdiagnostic.Diagnostic) string {
	path := strings.TrimSpace(diag.Path)
	if strings.HasPrefix(path, strings.TrimSpace(diag.StepKind)+".") {
		path = strings.TrimPrefix(path, strings.TrimSpace(diag.StepKind)+".")
	}
	path = strings.TrimPrefix(path, ".")
	return path
}

func defaultFillValue(diag askdiagnostic.Diagnostic, files []askcontract.GeneratedFile) (any, bool) {
	path := normalizeRepairPath(diag)
	switch strings.TrimSpace(diag.StepKind) {
	case "CheckHost":
		if path == "spec.checks" {
			return []string{"os", "arch", "swap"}, true
		}
	case "InitKubeadm":
		if path == "spec.outputJoinFile" {
			return "/tmp/deck/join.txt", true
		}
	case "JoinKubeadm":
		if path == "spec.joinFile" {
			return "/tmp/deck/join.txt", true
		}
	case "CheckCluster":
		if path == "spec.interval" {
			return "5s", true
		}
		if path == "spec.nodes.total" {
			return 1, true
		}
		if path == "spec.nodes.ready" {
			return 1, true
		}
		if path == "spec.nodes.controlPlaneReady" {
			return 1, true
		}
	case "InstallPackage":
		if path == "spec.source.type" {
			return "local-repo", true
		}
		if path == "spec.source.path" {
			return "packages/", true
		}
	case "DownloadPackage":
		if path == "spec.repo.type" {
			family, _ := downloadPackageDistro(files, diag)
			if strings.EqualFold(family, "debian") {
				return "deb-flat", true
			}
			return "rpm", true
		}
		if path == "spec.backend.mode" {
			return "container", true
		}
		if path == "spec.backend.runtime" {
			return "auto", true
		}
		if path == "spec.backend.image" {
			family, release := downloadPackageDistro(files, diag)
			if strings.EqualFold(family, "debian") {
				return "ubuntu:22.04", true
			}
			if strings.Contains(strings.ToLower(release), "9") {
				return "rockylinux:9", true
			}
			return "rockylinux:9", true
		}
		if path == "spec.outputDir" {
			family, release := downloadPackageDistro(files, diag)
			if strings.EqualFold(family, "debian") && release != "" {
				return filepath.ToSlash(filepath.Join("packages", "deb", release)), true
			}
			if release != "" {
				return filepath.ToSlash(filepath.Join("packages", "rpm", release)), true
			}
			return "packages/", true
		}
	}
	return nil, false
}

func downloadPackageDistro(files []askcontract.GeneratedFile, diag askdiagnostic.Diagnostic) (string, string) {
	file := diagnosticFile(diag)
	stepID := strings.TrimSpace(diag.StepID)
	if file == "" || stepID == "" {
		return "", ""
	}
	for _, candidate := range files {
		if filepath.ToSlash(strings.TrimSpace(candidate.Path)) != file || candidate.Delete {
			continue
		}
		doc, err := askir.ParseDocument(candidate.Path, []byte(candidate.Content))
		if err != nil || doc.Workflow == nil {
			continue
		}
		for _, step := range workflowSteps(*doc.Workflow) {
			if strings.TrimSpace(step.ID) != stepID {
				continue
			}
			distro, _ := step.Spec["distro"].(map[string]any)
			family, _ := distro["family"].(string)
			release, _ := distro["release"].(string)
			return strings.TrimSpace(family), strings.TrimSpace(release)
		}
	}
	return "", ""
}

func renameDuplicateStepIDs(path string, files []askcontract.GeneratedFile) (askcontract.GeneratedDocument, string, bool) {
	for _, candidate := range files {
		if filepath.ToSlash(strings.TrimSpace(candidate.Path)) != path || candidate.Delete {
			continue
		}
		doc, err := askir.ParseDocument(candidate.Path, []byte(candidate.Content))
		if err != nil || doc.Workflow == nil {
			return askcontract.GeneratedDocument{}, "", false
		}
		workflow := *doc.Workflow
		seen := map[string]int{}
		renamed := false
		for i := range workflow.Steps {
			newID, changed := uniqueStepID(workflow.Steps[i].ID, "step", seen)
			workflow.Steps[i].ID = newID
			renamed = renamed || changed
		}
		for i := range workflow.Phases {
			phaseLabel := sanitizeName(workflow.Phases[i].Name)
			for j := range workflow.Phases[i].Steps {
				newID, changed := uniqueStepID(workflow.Phases[i].Steps[j].ID, phaseLabel, seen)
				workflow.Phases[i].Steps[j].ID = newID
				renamed = renamed || changed
			}
		}
		if !renamed {
			return askcontract.GeneratedDocument{}, "", false
		}
		return askcontract.GeneratedDocument{Path: path, Kind: doc.Kind, Action: "replace", Workflow: &workflow}, "renamed duplicate step ids in " + path, true
	}
	return askcontract.GeneratedDocument{}, "", false
}

func uniqueStepID(current string, prefix string, seen map[string]int) (string, bool) {
	current = strings.TrimSpace(current)
	if current == "" {
		return current, false
	}
	count := seen[current]
	seen[current] = count + 1
	if count == 0 {
		return current, false
	}
	return sanitizeName(prefix) + "-" + current, true
}

func workflowSteps(doc askcontract.WorkflowDocument) []askcontract.WorkflowStep {
	out := append([]askcontract.WorkflowStep(nil), doc.Steps...)
	for _, phase := range doc.Phases {
		out = append(out, phase.Steps...)
	}
	return out
}

func sanitizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "step"
	}
	return value
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
