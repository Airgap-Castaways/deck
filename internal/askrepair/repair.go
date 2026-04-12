package askrepair

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcatalog"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askdiagnostic"
	"github.com/Airgap-Castaways/deck/internal/askir"
	"github.com/Airgap-Castaways/deck/internal/stepmeta"
	"github.com/Airgap-Castaways/deck/internal/stepspec"
	"github.com/Airgap-Castaways/deck/internal/structurededit"
	"github.com/Airgap-Castaways/deck/internal/validate"

	"gopkg.in/yaml.v3"
)

func TryAutoRepair(root string, files []askcontract.GeneratedFile, diags []askdiagnostic.Diagnostic, repairPaths []string) ([]askcontract.GeneratedFile, []string, bool, error) {
	return TryAutoRepairWithProgram(root, files, diags, repairPaths, askcontract.AuthoringProgram{})
}

func TryAutoRepairWithProgram(root string, files []askcontract.GeneratedFile, diags []askdiagnostic.Diagnostic, repairPaths []string, program askcontract.AuthoringProgram) ([]askcontract.GeneratedFile, []string, bool, error) {
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
		if !repairPathAllowed(path, repairPaths) {
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
			if edits, extraNotes, handled := repairInvalidFieldMigration(diag, files); handled {
				editDocs[path] = append(editDocs[path], edits...)
				notes = append(notes, extraNotes...)
				continue
			}
			editDocs[path] = append(editDocs[path], askcontract.StructuredEditAction{Op: "delete", RawPath: rawPath})
			notes = append(notes, fmt.Sprintf("removed unsupported field %s in %s", rawPath, path))
		case "fill-field":
			rawPath := repairRawPath(diag)
			value, ok := defaultFillValue(diag, files, program)
			if rawPath == "" || !ok {
				continue
			}
			editDocs[path] = append(editDocs[path], askcontract.StructuredEditAction{Op: "set", RawPath: rawPath, Value: value})
			notes = append(notes, fmt.Sprintf("filled %s in %s", rawPath, path))
		case "fix-literal":
			rawPath := repairRawPath(diag)
			if rawPath == "" {
				continue
			}
			if edits, extraNotes, handled := repairLiteralMigration(diag, files); handled {
				editDocs[path] = append(editDocs[path], edits...)
				notes = append(notes, extraNotes...)
				continue
			}
			value := ""
			if len(diag.Allowed) > 0 {
				value = strings.TrimSpace(diag.Allowed[0])
			}
			if preferred, ok := defaultLiteralValue(diag, files, program); ok && strings.TrimSpace(preferred) != "" {
				value = preferred
			}
			if value == "" {
				if preferred, ok := defaultFillValue(diag, files, program); ok {
					editDocs[path] = append(editDocs[path], askcontract.StructuredEditAction{Op: "set", RawPath: rawPath, Value: preferred})
					notes = append(notes, fmt.Sprintf("restored schema-valid literal %s in %s", rawPath, path))
				}
				continue
			}
			editDocs[path] = append(editDocs[path], askcontract.StructuredEditAction{Op: "set", RawPath: rawPath, Value: value})
			notes = append(notes, fmt.Sprintf("set constrained literal %s in %s", rawPath, path))
		case "review-diagnostic":
			if edits, extraNotes, handled := repairReviewDiagnostic(diag); handled {
				editDocs[path] = append(editDocs[path], edits...)
				notes = append(notes, extraNotes...)
			}
		}
	}
	if len(editDocs) == 0 && len(replaceDocs) == 0 {
		return files, nil, false, nil
	}
	repaired := append([]askcontract.GeneratedFile(nil), files...)
	index := map[string]int{}
	for i, file := range repaired {
		index[filepath.ToSlash(strings.TrimSpace(file.Path))] = i
	}
	for path, doc := range replaceDocs {
		content, err := renderRepairDocument(doc)
		if err != nil {
			return files, nil, false, err
		}
		if idx, ok := index[path]; ok {
			repaired[idx] = askcontract.GeneratedFile{Path: path, Content: content}
		} else {
			index[path] = len(repaired)
			repaired = append(repaired, askcontract.GeneratedFile{Path: path, Content: content})
		}
	}
	for path, edits := range editDocs {
		if _, replaced := replaceDocs[path]; replaced || len(edits) == 0 {
			continue
		}
		idx, ok := index[path]
		if !ok {
			continue
		}
		raw := []byte(repaired[idx].Content)
		doc, _ := askir.ParseDocument(path, raw)
		structEdits := make([]stepspec.StructuredEdit, 0, len(edits))
		for _, edit := range edits {
			resolvedPath := askir.ResolveStructuredEditPath(edit.RawPath, doc)
			structEdits = append(structEdits, stepspec.StructuredEdit{Op: edit.Op, RawPath: resolvedPath, Value: edit.Value})
		}
		applied, err := structurededit.Apply(structurededit.FormatYAML, raw, structEdits)
		if err != nil {
			return files, nil, false, fmt.Errorf("apply repair edits to %s: %w", path, err)
		}
		repaired[idx] = askcontract.GeneratedFile{Path: path, Content: normalizeRenderedContent(applied)}
	}
	return repaired, dedupeStrings(notes), true, nil
}

func repairPathAllowed(path string, repairPaths []string) bool {
	if len(repairPaths) == 0 {
		return true
	}
	path = filepath.ToSlash(strings.TrimSpace(path))
	for _, allowed := range repairPaths {
		if filepath.ToSlash(strings.TrimSpace(allowed)) == path {
			return true
		}
	}
	return false
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
	path = strings.TrimPrefix(path, "(root).")
	if strings.HasPrefix(path, strings.TrimSpace(diag.StepKind)+".") {
		path = strings.TrimPrefix(path, strings.TrimSpace(diag.StepKind)+".")
	}
	path = strings.TrimPrefix(path, ".")
	return path
}

func defaultFillValue(diag askdiagnostic.Diagnostic, files []askcontract.GeneratedFile, program askcontract.AuthoringProgram) (any, bool) {
	path := normalizeRepairPath(diag)
	switch {
	case path == "version":
		return validate.SupportedWorkflowVersion(), true
	case strings.HasSuffix(path, ".id"):
		return defaultStepID(diag), true
	case strings.HasSuffix(path, ".spec"):
		return map[string]any{}, true
	}
	if value, ok := bindingDefaultValue(strings.TrimSpace(diag.StepKind), path, program); ok {
		return value, true
	}
	if field, ok := askcatalog.Current().LookupField(strings.TrimSpace(diag.StepKind), path); ok {
		if parsed, ok := parseFieldDefault(field); ok {
			return parsed, true
		}
	}
	if strings.TrimSpace(diag.StepKind) == "CheckHost" && path == "spec.checks" {
		return []string{"os", "arch", "swap"}, true
	}
	return nil, false
}

func defaultLiteralValue(diag askdiagnostic.Diagnostic, files []askcontract.GeneratedFile, program askcontract.AuthoringProgram) (string, bool) {
	path := normalizeRepairPath(diag)
	if value, ok := bindingDefaultValue(strings.TrimSpace(diag.StepKind), path, program); ok {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return text, true
		}
	}
	if field, ok := askcatalog.Current().LookupField(strings.TrimSpace(diag.StepKind), path); ok {
		if len(field.Enum) > 0 {
			return strings.TrimSpace(field.Enum[0]), true
		}
	}
	return "", false
}

func bindingDefaultValue(kind string, path string, program askcontract.AuthoringProgram) (any, bool) {
	step, ok := askcatalog.Current().LookupStep(kind)
	if !ok {
		return nil, false
	}
	for _, builder := range step.Builders {
		for _, binding := range builder.Bindings {
			if strings.TrimSpace(binding.Path) != strings.TrimSpace(path) {
				continue
			}
			if value, ok := bindingSourceValue(strings.TrimSpace(binding.From), program); ok {
				return value, true
			}
		}
	}
	return nil, false
}

func bindingSourceValue(source string, program askcontract.AuthoringProgram) (any, bool) {
	switch {
	case strings.HasPrefix(source, "program:"):
		return program.Value(strings.TrimPrefix(source, "program:"))
	case strings.HasPrefix(source, "derive:"):
		return deriveRepairValue(strings.TrimPrefix(source, "derive:"), program)
	case strings.HasPrefix(source, "const:"):
		return parseConst(strings.TrimPrefix(source, "const:")), true
	default:
		return nil, false
	}
}

func deriveRepairValue(name string, program askcontract.AuthoringProgram) (any, bool) {
	switch strings.TrimSpace(name) {
	case "platform.repoType":
		if strings.EqualFold(strings.TrimSpace(program.Platform.Family), "debian") {
			return "deb-flat", true
		}
		return "rpm", true
	case "platform.backendImage":
		if strings.EqualFold(strings.TrimSpace(program.Platform.Family), "debian") {
			return "ubuntu:22.04", true
		}
		return "rockylinux:9", true
	case "artifacts.packageOutputDir":
		family := strings.ToLower(strings.TrimSpace(program.Platform.Family))
		release := strings.TrimSpace(program.Platform.Release)
		repoType := strings.ToLower(strings.TrimSpace(program.Platform.RepoType))
		if family == "debian" || repoType == "deb-flat" {
			if release == "" {
				return "packages/", true
			}
			return filepath.ToSlash(filepath.Join("packages", "deb", release)), true
		}
		if release == "" {
			return "packages/", true
		}
		return filepath.ToSlash(filepath.Join("packages", "rpm", release)), true
	case "artifacts.imageOutputDir":
		return "images/control-plane", true
	case "cluster.joinFile":
		return "/tmp/deck/join.txt", true
	case "cluster.podCIDR":
		return "10.244.0.0/16", true
	case "verification.expectedReadyCount":
		if program.Verification.ExpectedReadyCount > 0 {
			return program.Verification.ExpectedReadyCount, true
		}
		if program.Verification.ExpectedNodeCount > 0 {
			return program.Verification.ExpectedNodeCount, true
		}
	case "verification.expectedControlPlaneReady":
		if program.Verification.ExpectedControlPlaneReady > 0 {
			return program.Verification.ExpectedControlPlaneReady, true
		}
		if program.Cluster.ControlPlaneCount > 0 {
			return program.Cluster.ControlPlaneCount, true
		}
		return 1, true
	case "verification.interval":
		return "5s", true
	case "verification.timeout":
		if program.Verification.ExpectedNodeCount > 1 {
			return "10m", true
		}
		return "5m", true
	}
	return nil, false
}

func defaultStepID(diag askdiagnostic.Diagnostic) string {
	kind := strings.ToLower(strings.TrimSpace(diag.StepKind))
	if kind == "" {
		kind = "step"
	}
	kind = strings.ReplaceAll(kind, " ", "-")
	kind = strings.ReplaceAll(kind, "_", "-")
	return kind + "-step"
}

func repairReviewDiagnostic(diag askdiagnostic.Diagnostic) ([]askcontract.StructuredEditAction, []string, bool) {
	rawPath := repairRawPath(diag)
	if rawPath == "" {
		return nil, nil, false
	}
	if strings.TrimSpace(diag.StepKind) == "Command" && rawPath == "spec.command" && strings.Contains(strings.ToLower(strings.TrimSpace(diag.Message)), "invalid type") {
		value := strings.TrimSpace(diag.Actual)
		if value == "" {
			value = "true"
		}
		return []askcontract.StructuredEditAction{{Op: "set", RawPath: rawPath, Value: []string{value}}}, []string{fmt.Sprintf("normalized %s to a command array in %s", rawPath, diagnosticFile(diag))}, true
	}
	return nil, nil, false
}

func parseFieldDefault(field askcatalog.Field) (any, bool) {
	value := strings.TrimSpace(field.Default)
	if value == "" {
		return nil, false
	}
	switch field.Type {
	case "boolean":
		return value == "true", true
	case "integer":
		if n, err := strconv.Atoi(value); err == nil {
			return n, true
		}
	}
	return value, true
}

func parseConst(value string) any {
	value = strings.TrimSpace(value)
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}
	if n, err := strconv.Atoi(value); err == nil {
		return n
	}
	return value
}

func repairLiteralMigration(diag askdiagnostic.Diagnostic, files []askcontract.GeneratedFile) ([]askcontract.StructuredEditAction, []string, bool) {
	path := repairRawPath(diag)
	if !strings.HasSuffix(path, ".kind") {
		return nil, nil, false
	}
	actual := strings.TrimSpace(diag.Actual)
	switch strings.ToLower(actual) {
	case "apt", "yum", "dnf", "apk", "package", "packages":
		pkgValue, ok := invalidFieldAny(files, diag, "package")
		if !ok {
			pkgValue, ok = invalidFieldAny(files, diag, "name")
		}
		if !ok {
			return []askcontract.StructuredEditAction{{Op: "set", RawPath: path, Value: "InstallPackage"}}, []string{fmt.Sprintf("normalized %s step kind to InstallPackage in %s", actual, diagnosticFile(diag))}, true
		}
		packages := normalizePackageValue(pkgValue)
		stepRoot := strings.TrimSuffix(path, ".kind")
		edits := []askcontract.StructuredEditAction{{Op: "set", RawPath: path, Value: "InstallPackage"}, {Op: "set", RawPath: stepRoot + ".spec.packages", Value: packages}, {Op: "delete", RawPath: stepRoot + ".spec.package"}, {Op: "delete", RawPath: stepRoot + ".spec.name"}}
		if _, hasState := invalidFieldAny(files, diag, "state"); hasState {
			edits = append(edits, askcontract.StructuredEditAction{Op: "delete", RawPath: stepRoot + ".spec.state"})
		}
		return edits, []string{fmt.Sprintf("migrated %s package step to InstallPackage in %s", actual, diagnosticFile(diag))}, true
	}
	return nil, nil, false
}

func repairInvalidFieldMigration(diag askdiagnostic.Diagnostic, files []askcontract.GeneratedFile) ([]askcontract.StructuredEditAction, []string, bool) {
	path := repairRawPath(diag)
	if strings.TrimSpace(diag.StepKind) == "InstallPackage" && path == "steps."+strings.TrimSpace(diag.StepID)+".spec.sourcePath" {
		value, ok := invalidFieldValue(files, diag, "sourcePath")
		if !ok || strings.TrimSpace(value) == "" {
			return nil, nil, false
		}
		return []askcontract.StructuredEditAction{{Op: "set", RawPath: "steps." + strings.TrimSpace(diag.StepID) + ".spec.source.type", Value: "local-repo"}, {Op: "set", RawPath: "steps." + strings.TrimSpace(diag.StepID) + ".spec.source.path", Value: value}, {Op: "delete", RawPath: path}}, []string{fmt.Sprintf("moved InstallPackage sourcePath into source.path in %s", diagnosticFile(diag))}, true
	}
	if strings.TrimSpace(diag.StepKind) == "Command" && strings.HasSuffix(path, ".command") {
		value, ok := invalidFieldAny(files, diag, "command")
		if !ok {
			value = strings.TrimSpace(diag.Actual)
		}
		commandValue := normalizeCommandValue(value)
		stepRoot := strings.TrimSuffix(path, ".command")
		return []askcontract.StructuredEditAction{{Op: "set", RawPath: stepRoot + ".spec.command", Value: commandValue}, {Op: "delete", RawPath: path}}, []string{fmt.Sprintf("moved Command.command into spec.command in %s", diagnosticFile(diag))}, true
	}
	if strings.HasSuffix(path, ".kind") {
		actual := strings.TrimSpace(diag.Actual)
		for _, candidate := range stepmeta.RegisteredKinds() {
			if strings.EqualFold(candidate, actual) {
				return []askcontract.StructuredEditAction{{Op: "set", RawPath: path, Value: candidate}}, []string{fmt.Sprintf("normalized step kind casing in %s", diagnosticFile(diag))}, true
			}
		}
	}
	return nil, nil, false
}

func invalidFieldAny(files []askcontract.GeneratedFile, diag askdiagnostic.Diagnostic, key string) (any, bool) {
	file := diagnosticFile(diag)
	stepID := strings.TrimSpace(diag.StepID)
	for _, candidate := range files {
		if filepath.ToSlash(strings.TrimSpace(candidate.Path)) != file || candidate.Delete {
			continue
		}
		doc, err := askir.ParseDocument(candidate.Path, []byte(candidate.Content))
		if err != nil || doc.Workflow == nil {
			continue
		}
		steps := workflowSteps(*doc.Workflow)
		for i, step := range steps {
			if stepID != "" && strings.TrimSpace(step.ID) != stepID {
				continue
			}
			if stepID == "" && i > 0 {
				continue
			}
			value, ok := step.Spec[key]
			if ok {
				return value, true
			}
		}
	}
	return nil, false
}

func normalizePackageValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, strings.TrimSpace(fmt.Sprint(item)))
		}
		if len(out) > 0 {
			return out
		}
	case string:
		if strings.TrimSpace(typed) != "" {
			return []string{strings.TrimSpace(typed)}
		}
	default:
		if text := strings.TrimSpace(fmt.Sprint(typed)); text != "" && text != "<nil>" {
			return []string{text}
		}
	}
	return []string{"package"}
}

func normalizeCommandValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, strings.TrimSpace(fmt.Sprint(item)))
		}
		if len(out) > 0 {
			return out
		}
	case string:
		if strings.TrimSpace(typed) != "" {
			return []string{strings.TrimSpace(typed)}
		}
	case bool:
		return []string{fmt.Sprint(typed)}
	default:
		if text := strings.TrimSpace(fmt.Sprint(typed)); text != "" && text != "<nil>" {
			return []string{text}
		}
	}
	return []string{"true"}
}

func invalidFieldValue(files []askcontract.GeneratedFile, diag askdiagnostic.Diagnostic, key string) (string, bool) {
	file := diagnosticFile(diag)
	stepID := strings.TrimSpace(diag.StepID)
	if file == "" || stepID == "" {
		return "", false
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
			value, ok := step.Spec[key].(string)
			return strings.TrimSpace(value), ok
		}
	}
	return "", false
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
	base := sanitizeName(prefix) + "-" + current
	if seen[base] == 0 {
		seen[base] = 1
		return base, true
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s-%d", base, suffix)
		if seen[candidate] == 0 {
			seen[candidate] = 1
			return candidate, true
		}
	}
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

func renderRepairDocument(doc askcontract.GeneratedDocument) (string, error) {
	if doc.Workflow == nil {
		return "", fmt.Errorf("renderRepairDocument: document %s has no workflow content", doc.Path)
	}
	raw, err := yaml.Marshal(doc.Workflow)
	if err != nil {
		return "", fmt.Errorf("render repair document %s: %w", doc.Path, err)
	}
	return normalizeRenderedContent(raw), nil
}

func normalizeRenderedContent(raw []byte) string {
	trimmed := strings.TrimRight(string(raw), "\n")
	if trimmed == "" {
		return ""
	}
	return trimmed + "\n"
}
