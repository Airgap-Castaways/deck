package askdraft

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcatalog"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

type Candidate struct {
	ID                string
	Path              string
	Phase             string
	StepKind          string
	Summary           string
	RequiredOverrides []string
	OptionalOverrides []string
}

func Candidates(plan askcontract.PlanResponse, brief askcontract.AuthoringBrief) []Candidate {
	paths := candidateTargetPaths(plan)
	items := []Candidate{}
	for _, path := range paths {
		items = append(items, matchedCandidatesForPath(path, brief.RequiredCapabilities)...)
	}
	return dedupeCandidates(items)
}

func PromptBlock(plan askcontract.PlanResponse, brief askcontract.AuthoringBrief) string {
	candidates := Candidates(plan, brief)
	if len(candidates) == 0 {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString("Draft builder candidates:\n")
	b.WriteString("- Select builder ids under selection.targets[].builders and set only documented override keys; do not author raw step.spec payloads.\n")
	b.WriteString("- Code assembles the workflow documents from these candidates, fills defaults from the authoring program and source-of-truth metadata, and rejects unsupported override keys.\n")
	paths := candidateTargetPaths(plan)
	for _, path := range paths {
		matched := matchedCandidatesForPath(path, brief.RequiredCapabilities)
		if len(matched) == 0 {
			supportedIDs := askcatalog.Current().BuilderIDsForPath(path)
			if len(supportedIDs) == 0 {
				b.WriteString("- path: ")
				b.WriteString(path)
				b.WriteString(" has no registered draft builders.\n")
				continue
			}
			b.WriteString("- path: ")
			b.WriteString(path)
			b.WriteString(" has no capability-matched draft builders under the current plan. If you must generate this path, stay inside these supported builder ids: ")
			b.WriteString(strings.Join(supportedIDs, ", "))
			b.WriteString("\n")
			continue
		}
		b.WriteString("- path: ")
		b.WriteString(path)
		b.WriteString(" supported builder ids: ")
		b.WriteString(strings.Join(candidateIDs(matched), ", "))
		b.WriteString("\n")
		for _, candidate := range matched {
			b.WriteString("- id: ")
			b.WriteString(candidate.ID)
			b.WriteString(" path=")
			b.WriteString(candidate.Path)
			b.WriteString(" phase=")
			b.WriteString(candidate.Phase)
			b.WriteString(" step=")
			b.WriteString(candidate.StepKind)
			b.WriteString(" summary=")
			b.WriteString(candidate.Summary)
			b.WriteString("\n")
			if len(candidate.RequiredOverrides) > 0 {
				b.WriteString("  - required overrides: ")
				b.WriteString(strings.Join(candidate.RequiredOverrides, ", "))
				b.WriteString("\n")
			}
			if len(candidate.OptionalOverrides) > 0 {
				b.WriteString("  - optional overrides: ")
				b.WriteString(strings.Join(candidate.OptionalOverrides, ", "))
				b.WriteString("\n")
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func Compile(selection askcontract.DraftSelection) ([]askcontract.GeneratedDocument, error) {
	return CompileWithProgram(askcontract.AuthoringProgram{}, selection)
}

func CompileWithProgram(program askcontract.AuthoringProgram, selection askcontract.DraftSelection) ([]askcontract.GeneratedDocument, error) {
	catalog := askcatalog.Current()
	documents := make([]askcontract.GeneratedDocument, 0, len(selection.Targets)+1)
	selectionVars := cloneMap(selection.Vars)
	for _, target := range selection.Targets {
		if documentKind(target.Path, target.Kind) == "vars" {
			selectionVars = mergedVars(selectionVars, target.Vars)
		}
	}
	for _, target := range selection.Targets {
		path := filepath.ToSlash(strings.TrimSpace(target.Path))
		if path == "" {
			continue
		}
		kind := documentKind(path, target.Kind)
		switch kind {
		case "vars":
			documents = append(documents, askcontract.GeneratedDocument{Path: path, Kind: "vars", Vars: cloneMap(target.Vars)})
		case "component":
			documents = append(documents, askcontract.GeneratedDocument{Path: path, Kind: "component", Component: &askcontract.ComponentDocument{Steps: append([]askcontract.WorkflowStep(nil), target.Steps...)}})
		default:
			if len(target.Builders) == 0 {
				continue
			}
			workflow, err := buildWorkflowTarget(catalog, path, target, program, mergedVars(selectionVars, target.Vars))
			if err != nil {
				return nil, err
			}
			documents = append(documents, askcontract.GeneratedDocument{Path: path, Kind: "workflow", Workflow: workflow})
		}
	}
	if len(selection.Vars) > 0 && !hasVarsTarget(selection.Targets) {
		documents = append(documents, askcontract.GeneratedDocument{Path: workspacepaths.CanonicalVarsWorkflow, Kind: "vars", Vars: cloneMap(selection.Vars)})
	}
	documents = ensureRoleVarInDraftDocuments(documents)
	if len(documents) == 0 {
		return nil, fmt.Errorf("draft builder selection did not produce any documents")
	}
	return documents, nil
}

func ensureRoleVarInDraftDocuments(documents []askcontract.GeneratedDocument) []askcontract.GeneratedDocument {
	if !draftDocumentsReferenceRoleVar(documents) {
		return documents
	}
	hasVarsDoc := false
	for i := range documents {
		if documentKind(documents[i].Path, documents[i].Kind) != "vars" {
			continue
		}
		hasVarsDoc = true
		if documents[i].Vars == nil {
			documents[i].Vars = map[string]any{}
		}
		if _, ok := documents[i].Vars["role"]; !ok {
			documents[i].Vars["role"] = "control-plane"
		}
	}
	if !hasVarsDoc {
		documents = append(documents, askcontract.GeneratedDocument{Path: workspacepaths.CanonicalVarsWorkflow, Kind: "vars", Vars: map[string]any{"role": "control-plane"}})
	}
	return documents
}

func draftDocumentsReferenceRoleVar(documents []askcontract.GeneratedDocument) bool {
	for _, doc := range documents {
		if doc.Workflow == nil {
			continue
		}
		if workflowReferencesRoleVar(*doc.Workflow) {
			return true
		}
	}
	return false
}

func workflowReferencesRoleVar(workflow askcontract.WorkflowDocument) bool {
	for _, step := range workflow.Steps {
		if strings.Contains(strings.TrimSpace(step.When), "vars.role") {
			return true
		}
	}
	for _, phase := range workflow.Phases {
		for _, step := range phase.Steps {
			if strings.Contains(strings.TrimSpace(step.When), "vars.role") {
				return true
			}
		}
	}
	return false
}

func buildWorkflowTarget(catalog askcatalog.Catalog, path string, target askcontract.DraftTargetSelection, program askcontract.AuthoringProgram, variables map[string]any) (*askcontract.WorkflowDocument, error) {
	workflow := &askcontract.WorkflowDocument{Version: "v1alpha1", Vars: cloneMap(target.Vars)}
	phaseIndex := map[string]int{}
	usedStepIDs := map[string]int{}
	for _, selected := range target.Builders {
		builder, ok := catalog.LookupBuilder(strings.TrimSpace(selected.ID))
		if !ok {
			return nil, fmt.Errorf("unsupported draft builder %q for %s", selected.ID, path)
		}
		overrides := normalizeBuilderOverrides(builder, selected.Overrides)
		if err := validateOverrideKeys(builder, overrides); err != nil {
			return nil, err
		}
		phase, step, err := buildStep(builder, overrides, program, variables)
		if err != nil {
			return nil, err
		}
		step.ID = nextDraftStepID(step.ID, phase, step.When, usedStepIDs)
		idx, ok := phaseIndex[phase]
		if !ok {
			phaseIndex[phase] = len(workflow.Phases)
			workflow.Phases = append(workflow.Phases, askcontract.WorkflowPhase{Name: phase})
			idx = len(workflow.Phases) - 1
		}
		workflow.Phases[idx].Steps = append(workflow.Phases[idx].Steps, step)
	}
	return workflow, nil
}

func nextDraftStepID(base string, phase string, when string, used map[string]int) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "step"
	}
	if used == nil {
		return base
	}
	if used[base] == 0 {
		used[base] = 1
		return base
	}
	suffixes := []string{}
	if role := roleSuffixFromWhen(when); role != "" {
		suffixes = append(suffixes, role)
	}
	if cleanPhase := sanitizeStepID(phase); cleanPhase != "" && cleanPhase != "main" {
		suffixes = append(suffixes, cleanPhase)
	}
	for _, suffix := range suffixes {
		candidate := base + "-" + suffix
		if used[candidate] == 0 {
			used[candidate] = 1
			used[base]++
			return candidate
		}
	}
	count := used[base] + 1
	used[base] = count
	for {
		candidate := fmt.Sprintf("%s-%d", base, count)
		if used[candidate] == 0 {
			used[candidate] = 1
			return candidate
		}
		count++
		used[base] = count
	}
}

func roleSuffixFromWhen(when string) string {
	when = strings.TrimSpace(strings.ToLower(when))
	for _, token := range []string{"control-plane", "worker", "prepare", "apply"} {
		if strings.Contains(when, token) {
			return sanitizeStepID(token)
		}
	}
	return ""
}

func buildStep(builder askcatalog.Builder, overrides map[string]any, program askcontract.AuthoringProgram, variables map[string]any) (string, askcontract.WorkflowStep, error) {
	resolved, err := resolveBindings(builder, overrides, program, variables)
	if err != nil {
		return "", askcontract.WorkflowStep{}, err
	}
	step := askcontract.WorkflowStep{ID: firstNonEmpty(builder.DefaultStepID, sanitizeStepID(builder.ID)), Kind: builder.StepKind, Spec: map[string]any{}}
	for _, path := range resolved.order {
		value := resolved.values[path]
		switch path {
		case "when":
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "nil" && text != "<nil>" && !strings.Contains(text, "vars.nil") {
				step.When = text
			}
		case "id":
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" {
				step.ID = text
			}
		default:
			if !strings.HasPrefix(path, "spec.") {
				continue
			}
			setPath(step.Spec, strings.TrimPrefix(path, "spec."), value)
		}
	}
	return firstNonEmpty(builder.Phase, "main"), step, nil
}

type bindingResolution struct {
	values map[string]any
	order  []string
}

func resolveBindings(builder askcatalog.Builder, overrides map[string]any, program askcontract.AuthoringProgram, variables map[string]any) (bindingResolution, error) {
	values := map[string]any{}
	order := []string{}
	required := map[string]bool{}
	seenPath := map[string]bool{}
	for _, binding := range builder.Bindings {
		path := strings.TrimSpace(binding.Path)
		if path == "" {
			continue
		}
		if !seenPath[path] {
			seenPath[path] = true
			order = append(order, path)
		}
		if binding.Required {
			required[path] = true
		}
		if _, ok := values[path]; ok {
			continue
		}
		if value, ok := resolveBindingValue(strings.TrimSpace(binding.From), overrides, program, variables); ok {
			values[path] = canonicalizeBindingValue(binding, value, program)
		}
	}
	for path := range required {
		if _, ok := values[path]; !ok {
			return bindingResolution{}, fmt.Errorf("draft builder %s requires %s", builder.ID, path)
		}
	}
	return bindingResolution{values: values, order: order}, nil
}

func resolveBindingValue(source string, overrides map[string]any, program askcontract.AuthoringProgram, variables map[string]any) (any, bool) {
	switch {
	case strings.HasPrefix(source, "override:"):
		key := strings.TrimPrefix(source, "override:")
		value, ok := overrides[key]
		if !ok || value == nil {
			return nil, false
		}
		if strings.TrimSpace(key) == "whenRole" {
			if normalized, ok := normalizeWhenRoleOverride(value, program); ok {
				return normalized, true
			}
			return nil, false
		}
		return normalizeOverrideValue(value, variables), true
	case strings.HasPrefix(source, "program:"):
		return program.Value(strings.TrimPrefix(source, "program:"))
	case strings.HasPrefix(source, "derive:"):
		return deriveValue(strings.TrimPrefix(source, "derive:"), overrides, program)
	case strings.HasPrefix(source, "const:"):
		return parseConst(strings.TrimPrefix(source, "const:")), true
	default:
		return nil, false
	}
}

func normalizeWhenRoleOverride(value any, program askcontract.AuthoringProgram) (any, bool) {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" || text == "nil" {
		return nil, false
	}
	if strings.Contains(text, "{{") || strings.Contains(text, "${{") {
		return nil, false
	}
	if strings.Contains(text, "==") || strings.HasPrefix(text, "vars.") || strings.HasPrefix(text, "runtime.") {
		return text, true
	}
	return roleWhen(program, text)
}

func deriveValue(name string, overrides map[string]any, program askcontract.AuthoringProgram) (any, bool) {
	switch strings.TrimSpace(name) {
	case "platform.repoType":
		family := firstNonEmpty(stringOverride(overrides, "distroFamily"), stringValue(program, "platform.family"))
		if strings.EqualFold(family, "debian") {
			return "deb-flat", true
		}
		return "rpm", true
	case "platform.backendImage":
		family := firstNonEmpty(stringOverride(overrides, "distroFamily"), stringValue(program, "platform.family"))
		if strings.EqualFold(family, "debian") {
			return "ubuntu:22.04", true
		}
		return "rockylinux:9", true
	case "artifacts.packageOutputDir":
		family := firstNonEmpty(stringOverride(overrides, "distroFamily"), stringValue(program, "platform.family"))
		release := firstNonEmpty(stringOverride(overrides, "distroRelease"), stringValue(program, "platform.release"))
		repoType := firstNonEmpty(stringOverride(overrides, "repoType"), stringValue(program, "platform.repoType"))
		if strings.EqualFold(family, "debian") || strings.EqualFold(repoType, "deb-flat") {
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
	case "cluster.kubernetesVersion":
		return "stable", true
	case "cluster.criSocket":
		return "unix:///run/containerd/containerd.sock", true
	case "cluster.roleWhen.control-plane":
		return roleWhen(program, "control-plane")
	case "cluster.roleWhen.worker":
		return roleWhen(program, "worker")
	case "verification.expectedReadyCount":
		if value := intOverride(overrides, "readyCount"); value > 0 {
			return value, true
		}
		if value, ok := intValue(program, "verification.expectedReadyCount"); ok {
			return value, true
		}
		if value := intOverride(overrides, "nodeCount"); value > 0 {
			return value, true
		}
		if value, ok := intValue(program, "verification.expectedNodeCount"); ok {
			return value, true
		}
	case "verification.expectedControlPlaneReady":
		if value := intOverride(overrides, "controlPlaneReady"); value > 0 {
			return value, true
		}
		if value, ok := intValue(program, "verification.expectedControlPlaneReady"); ok {
			return value, true
		}
		if count, ok := intValue(program, "cluster.controlPlaneCount"); ok && count > 0 {
			return count, true
		}
		return 1, true
	case "verification.interval":
		return "5s", true
	case "verification.timeout":
		if count, ok := intValue(program, "verification.expectedNodeCount"); ok && count > 1 {
			return "10m", true
		}
		if count := intOverride(overrides, "nodeCount"); count > 1 {
			return "10m", true
		}
		return "5m", true
	case "verification.roleWhen":
		role := stringValue(program, "verification.finalVerificationRole")
		if strings.TrimSpace(role) == "" || strings.TrimSpace(role) == "local" {
			return "", false
		}
		return roleWhen(program, role)
	}
	return nil, false
}

func roleWhen(program askcontract.AuthoringProgram, role string) (any, bool) {
	selector := stringValue(program, "cluster.roleSelector")
	controlPlaneCount, _ := intValue(program, "cluster.controlPlaneCount")
	workerCount, _ := intValue(program, "cluster.workerCount")
	if selector == "" || selector == "nil" || selector == "<nil>" || controlPlaneCount+workerCount <= 1 {
		return "", false
	}
	return `vars.` + selector + ` == "` + strings.TrimSpace(role) + `"`, true
}

func stringValue(program askcontract.AuthoringProgram, path string) string {
	if value, ok := program.Value(path); ok {
		if text, ok := value.(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func intValue(program askcontract.AuthoringProgram, path string) (int, bool) {
	value, ok := program.Value(path)
	if !ok || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return typed, typed > 0
	case int64:
		return int(typed), typed > 0
	case float64:
		return int(typed), typed > 0
	default:
		return 0, false
	}
}

func stringOverride(overrides map[string]any, key string) string {
	if len(overrides) == 0 {
		return ""
	}
	value, ok := overrides[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func intOverride(overrides map[string]any, key string) int {
	if len(overrides) == 0 {
		return 0
	}
	value, ok := overrides[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(typed))
		return n
	default:
		n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(value)))
		return n
	}
}

func validateOverrideKeys(builder askcatalog.Builder, overrides map[string]any) error {
	allowed := map[string]bool{}
	for _, key := range builder.OverrideKeys() {
		allowed[key] = true
	}
	for key := range overrides {
		clean := strings.TrimSpace(key)
		if !allowed[clean] && !deprecatedOverrideAllowed(builder.ID, clean) {
			return fmt.Errorf("draft builder %s does not support override %q", builder.ID, key)
		}
	}
	return nil
}

func normalizeBuilderOverrides(builder askcatalog.Builder, overrides map[string]any) map[string]any {
	if len(overrides) == 0 {
		return nil
	}
	normalized := map[string]any{}
	for key, value := range overrides {
		clean := strings.TrimSpace(key)
		if clean == "" || value == nil || ignoredBuilderOverrideKey(clean) {
			continue
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			continue
		}
		if alias := builderOverrideAlias(builder.ID, clean); alias != "" {
			clean = alias
		}
		normalized[clean] = value
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func ignoredBuilderOverrideKey(key string) bool {
	switch strings.TrimSpace(key) {
	case "phase", "step", "stepKind", "summary", "action", "path", "kind":
		return true
	default:
		return false
	}
}

func builderOverrideAlias(builderID string, key string) string {
	key = strings.TrimSpace(key)
	if strings.TrimSpace(builderID) == "apply.init-kubeadm" && key == "outputJoinFile" {
		return "joinFile"
	}
	return ""
}

func deprecatedOverrideAllowed(builderID string, key string) bool {
	for _, item := range []string{"backend", "distro", "id", "kind", "repo", "source", "spec", "when"} {
		if strings.TrimSpace(item) == strings.TrimSpace(key) {
			return true
		}
	}
	allowed := map[string][]string{
		"prepare.download-package":       {"backendImage", "backendRuntime", "distroFamily", "distroRelease", "outputDir", "packages", "repoType"},
		"prepare.download-image":         {"backendEngine", "images", "outputDir"},
		"apply.install-package":          {"packages", "sourcePath"},
		"apply.load-image":               {"images", "runtime", "sourceDir"},
		"apply.init-kubeadm":             {"criSocket", "imageRepository", "joinFile", "kubernetesVersion", "podCIDR", "whenRole"},
		"apply.join-kubeadm":             {"joinFile", "whenRole"},
		"apply.check-cluster":            {"controlPlaneReady", "interval", "nodeCount", "readyCount", "timeout", "whenRole"},
		"apply.check-kubernetes-cluster": {"controlPlaneReady", "interval", "nodeCount", "readyCount", "timeout", "whenRole"},
	}
	for _, item := range allowed[strings.TrimSpace(builderID)] {
		if strings.TrimSpace(item) == strings.TrimSpace(key) {
			return true
		}
	}
	return false
}

func candidateTargetPaths(plan askcontract.PlanResponse) []string {
	allowed := map[string]bool{}
	for _, path := range plan.AuthoringBrief.TargetPaths {
		clean := filepath.ToSlash(strings.TrimSpace(path))
		if clean != "" {
			allowed[clean] = true
		}
	}
	for _, file := range plan.Files {
		clean := filepath.ToSlash(strings.TrimSpace(file.Path))
		if clean != "" {
			allowed[clean] = true
		}
	}
	out := make([]string, 0, len(allowed))
	for path := range allowed {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func targetRole(path string) string {
	builders := askcatalog.Current().BuildersForPath(path)
	if len(builders) == 0 {
		return ""
	}
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == workspacepaths.CanonicalPrepareWorkflow {
		return "prepare"
	}
	return "apply"
}

func builderAllowed(builder askcatalog.Builder, capabilities []string) bool {
	if len(builder.RequiresCapabilities) == 0 {
		return true
	}
	for _, capability := range builder.RequiresCapabilities {
		if containsString(capabilities, capability) {
			return true
		}
	}
	return false
}

func matchedCandidatesForPath(path string, capabilities []string) []Candidate {
	catalog := askcatalog.Current()
	role := targetRole(path)
	if role == "" {
		return nil
	}
	items := []Candidate{}
	for _, step := range catalog.StepKinds() {
		if !containsString(step.AllowedRoles, role) {
			continue
		}
		for _, builder := range step.Builders {
			if !builderAllowed(builder, capabilities) {
				continue
			}
			items = append(items, Candidate{
				ID:                builder.ID,
				Path:              path,
				Phase:             builder.Phase,
				StepKind:          builder.StepKind,
				Summary:           firstNonEmpty(builder.Summary, step.Summary),
				RequiredOverrides: builder.RequiredOverrideKeys(),
				OptionalOverrides: builder.OptionalOverrideKeys(),
			})
		}
	}
	return dedupeCandidates(items)
}

func candidateIDs(candidates []Candidate) []string {
	if len(candidates) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		id := strings.TrimSpace(candidate.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func normalizeOverrideValue(value any, variables map[string]any) any {
	switch typed := value.(type) {
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				out = append(out, text)
			}
		}
		if len(out) > 0 {
			return out
		}
		return nil
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(item)
			if text != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		if resolved, ok := resolveVarReference(typed, variables); ok {
			return resolved
		}
		return strings.TrimSpace(typed)
	default:
		return value
	}
}

func canonicalizeBindingValue(binding askcatalog.Binding, value any, program askcontract.AuthoringProgram) any {
	semantic := strings.TrimSpace(binding.Semantic)
	text, isString := value.(string)
	if !isString {
		return value
	}
	text = strings.TrimSpace(text)
	switch semantic {
	case "package-output-dir":
		if strings.HasPrefix(text, "packages/") || text == "packages/" {
			return text
		}
		if canonical, ok := deriveValue("artifacts.packageOutputDir", nil, program); ok {
			return canonical
		}
	case "image-output-dir":
		if strings.HasPrefix(text, "images/") || text == "images/" {
			return text
		}
		if canonical, ok := deriveValue("artifacts.imageOutputDir", nil, program); ok {
			return canonical
		}
	case "package-repo-type":
		if text == "rpm" || text == "deb-flat" {
			return text
		}
		if canonical, ok := deriveValue("platform.repoType", nil, program); ok {
			return canonical
		}
	}
	return value
}

func resolveVarReference(value string, variables map[string]any) (any, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "{{") || !strings.HasSuffix(value, "}}") {
		return nil, false
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "{{"), "}}"))
	inner = strings.TrimSpace(strings.TrimPrefix(inner, "."))
	if !strings.HasPrefix(inner, "vars.") {
		return nil, false
	}
	key := strings.TrimSpace(strings.TrimPrefix(inner, "vars."))
	resolved, ok := variables[key]
	if !ok {
		return nil, false
	}
	return cloneValue(resolved), true
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

func setPath(root map[string]any, path string, value any) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	current := root
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == len(parts)-1 {
			current[part] = value
			return
		}
		next, _ := current[part].(map[string]any)
		if next == nil {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
}

func sanitizeStepID(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, ".", "-")
	value = strings.ReplaceAll(value, "_", "-")
	if value == "" {
		return "step"
	}
	return value
}

func documentKind(path string, kind string) string {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == workspacepaths.CanonicalVarsWorkflow {
		return "vars"
	}
	if strings.HasPrefix(clean, "workflows/components/") {
		return "component"
	}
	if strings.TrimSpace(kind) != "" {
		return strings.ToLower(strings.TrimSpace(kind))
	}
	return "workflow"
}

func hasVarsTarget(targets []askcontract.DraftTargetSelection) bool {
	for _, target := range targets {
		if documentKind(target.Path, target.Kind) == "vars" {
			return true
		}
	}
	return false
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = cloneValue(value)
	}
	return out
}

func mergedVars(base map[string]any, overlay map[string]any) map[string]any {
	out := cloneMap(base)
	if out == nil {
		out = map[string]any{}
	}
	for key, value := range overlay {
		out[key] = cloneValue(value)
	}
	return out
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, cloneValue(item))
		}
		return out
	case []string:
		out := make([]string, 0, len(typed))
		out = append(out, typed...)
		return out
	default:
		return value
	}
}

func containsString(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func dedupeCandidates(items []Candidate) []Candidate {
	seen := map[string]bool{}
	out := make([]Candidate, 0, len(items))
	for _, item := range items {
		key := item.ID + "|" + item.Path
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].ID < out[j].ID
		}
		return out[i].Path < out[j].Path
	})
	return out
}
