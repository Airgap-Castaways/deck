package askdraft

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
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

type sharedContext struct {
	PackageList       []string
	ImageList         []string
	DistroFamily      string
	DistroRelease     string
	RepoType          string
	PackageOutputDir  string
	ImageOutputDir    string
	JoinFile          string
	PodCIDR           string
	NodeCount         int
	ControlPlaneReady int
	RoleSelectorVar   string
}

func Candidates(plan askcontract.PlanResponse, brief askcontract.AuthoringBrief) []Candidate {
	items := []Candidate{}
	allowed := map[string]bool{}
	for _, path := range append([]string{}, plan.AuthoringBrief.TargetPaths...) {
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
	if allowed["workflows/prepare.yaml"] {
		if hasCapability(brief.RequiredCapabilities, "prepare-artifacts") || hasCapability(brief.RequiredCapabilities, "package-staging") {
			items = append(items, Candidate{ID: "prepare.download-package", Path: "workflows/prepare.yaml", Phase: "packages", StepKind: "DownloadPackage", Summary: "Stage offline packages with typed package download schema.", RequiredOverrides: []string{"packages", "distroFamily", "distroRelease"}, OptionalOverrides: []string{"repoType", "backendRuntime", "backendImage", "outputDir", "timeout"}})
		}
		if hasCapability(brief.RequiredCapabilities, "image-staging") {
			items = append(items, Candidate{ID: "prepare.download-image", Path: "workflows/prepare.yaml", Phase: "images", StepKind: "DownloadImage", Summary: "Stage offline image archives with typed image download schema.", RequiredOverrides: []string{"images"}, OptionalOverrides: []string{"outputDir", "backendEngine", "timeout"}})
		}
	}
	for path := range allowed {
		if path != "workflows/prepare.yaml" && !strings.HasPrefix(path, "workflows/scenarios/") {
			continue
		}
		if hasCapability(brief.RequiredCapabilities, "package-staging") || hasCapability(brief.RequiredCapabilities, "prepare-artifacts") {
			items = append(items, Candidate{ID: "apply.install-package", Path: path, Phase: "install-packages", StepKind: "InstallPackage", Summary: "Install packages from the local offline repository output.", RequiredOverrides: []string{}, OptionalOverrides: []string{"packages", "sourcePath", "timeout"}})
		}
		if hasCapability(brief.RequiredCapabilities, "image-staging") {
			items = append(items, Candidate{ID: "apply.load-image", Path: path, Phase: "load-images", StepKind: "LoadImage", Summary: "Load prepared image archives into the local runtime.", RequiredOverrides: []string{}, OptionalOverrides: []string{"images", "sourceDir", "runtime", "timeout"}})
		}
		if hasCapability(brief.RequiredCapabilities, "kubeadm-bootstrap") {
			items = append(items, Candidate{ID: "apply.init-kubeadm", Path: path, Phase: "bootstrap", StepKind: "InitKubeadm", Summary: "Bootstrap a control-plane node with typed kubeadm init fields.", RequiredOverrides: []string{}, OptionalOverrides: []string{"joinFile", "podCIDR", "kubernetesVersion", "criSocket", "whenRole", "timeout"}})
		}
		if hasCapability(brief.RequiredCapabilities, "kubeadm-join") {
			items = append(items, Candidate{ID: "apply.join-kubeadm", Path: path, Phase: "join", StepKind: "JoinKubeadm", Summary: "Join worker nodes with a typed kubeadm join step.", RequiredOverrides: []string{}, OptionalOverrides: []string{"joinFile", "whenRole", "timeout"}})
		}
		if hasCapability(brief.RequiredCapabilities, "cluster-verification") {
			items = append(items, Candidate{ID: "apply.check-cluster", Path: path, Phase: "verify", StepKind: "CheckCluster", Summary: "Verify cluster readiness with typed node expectations.", RequiredOverrides: []string{}, OptionalOverrides: []string{"nodeCount", "readyCount", "controlPlaneReady", "interval", "timeout", "whenRole"}})
		}
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
	b.WriteString("- Code assembles the workflow documents from these candidates, fills safe defaults, and rejects unsupported override keys.\n")
	for _, candidate := range candidates {
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
	return strings.TrimSpace(b.String())
}

func Compile(selection askcontract.DraftSelection) ([]askcontract.GeneratedDocument, error) {
	ctx := inferSharedContext(selection)
	documents := make([]askcontract.GeneratedDocument, 0, len(selection.Targets)+1)
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
			workflow, err := buildWorkflowTarget(path, target, ctx)
			if err != nil {
				return nil, err
			}
			documents = append(documents, askcontract.GeneratedDocument{Path: path, Kind: "workflow", Workflow: workflow})
		}
	}
	if len(selection.Vars) > 0 && !hasVarsTarget(selection.Targets) {
		documents = append(documents, askcontract.GeneratedDocument{Path: "workflows/vars.yaml", Kind: "vars", Vars: cloneMap(selection.Vars)})
	}
	if len(documents) == 0 {
		return nil, fmt.Errorf("draft builder selection did not produce any documents")
	}
	return documents, nil
}

func buildWorkflowTarget(path string, target askcontract.DraftTargetSelection, ctx sharedContext) (*askcontract.WorkflowDocument, error) {
	workflow := &askcontract.WorkflowDocument{Version: "v1alpha1", Vars: cloneMap(target.Vars)}
	phaseIndex := map[string]int{}
	for _, selected := range target.Builders {
		phase, step, err := buildStep(path, selected, ctx)
		if err != nil {
			return nil, err
		}
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

func buildStep(path string, selected askcontract.DraftBuilderSelection, ctx sharedContext) (string, askcontract.WorkflowStep, error) {
	path = filepath.ToSlash(strings.TrimSpace(path))
	overrides := selected.Overrides
	if err := validateOverrideKeys(strings.TrimSpace(selected.ID), overrides); err != nil {
		return "", askcontract.WorkflowStep{}, err
	}
	switch strings.TrimSpace(selected.ID) {
	case "prepare.download-package":
		packages := firstStringList(ctx.PackageList, stringListOverride(overrides, "packages"))
		if len(packages) == 0 {
			return "", askcontract.WorkflowStep{}, fmt.Errorf("builder %s requires override packages", selected.ID)
		}
		family := firstString(stringOverride(overrides, "distroFamily"), ctx.DistroFamily)
		release := firstString(stringOverride(overrides, "distroRelease"), ctx.DistroRelease)
		if family == "" || release == "" {
			return "", askcontract.WorkflowStep{}, fmt.Errorf("builder %s requires overrides distroFamily and distroRelease", selected.ID)
		}
		repoType := firstString(stringOverride(overrides, "repoType"), ctx.RepoType, defaultRepoType(family))
		outputDir := firstString(stringOverride(overrides, "outputDir"), ctx.PackageOutputDir, defaultPackageOutputDir(family, release, repoType))
		step := map[string]any{
			"packages": packages,
			"distro": map[string]any{
				"family":  family,
				"release": release,
			},
			"repo": map[string]any{
				"type":     repoType,
				"generate": true,
			},
			"backend": map[string]any{
				"mode":    "container",
				"runtime": firstString(stringOverride(overrides, "backendRuntime"), "auto"),
				"image":   firstString(stringOverride(overrides, "backendImage"), defaultBackendImage(family, release)),
			},
			"outputDir": outputDir,
			"timeout":   firstString(stringOverride(overrides, "timeout"), "30m"),
		}
		return "packages", workflowStep("prepare-download-packages", "DownloadPackage", "", step), nil
	case "prepare.download-image":
		images := firstStringList(ctx.ImageList, stringListOverride(overrides, "images"))
		if len(images) == 0 {
			return "", askcontract.WorkflowStep{}, fmt.Errorf("builder %s requires override images", selected.ID)
		}
		step := map[string]any{
			"images": images,
			"backend": map[string]any{
				"engine": firstString(stringOverride(overrides, "backendEngine"), "go-containerregistry"),
			},
			"outputDir": firstString(stringOverride(overrides, "outputDir"), ctx.ImageOutputDir, "images/control-plane"),
			"timeout":   firstString(stringOverride(overrides, "timeout"), "10m"),
		}
		return "images", workflowStep("prepare-download-images", "DownloadImage", "", step), nil
	case "apply.install-package":
		step := map[string]any{
			"packages": firstStringList(stringListOverride(overrides, "packages"), ctx.PackageList),
			"source": map[string]any{
				"type": "local-repo",
				"path": firstString(stringOverride(overrides, "sourcePath"), ctx.PackageOutputDir, defaultPackageOutputDir(ctx.DistroFamily, ctx.DistroRelease, defaultRepoType(ctx.DistroFamily))),
			},
			"timeout": firstString(stringOverride(overrides, "timeout"), "20m"),
		}
		return "install-packages", workflowStep("apply-install-packages", "InstallPackage", "", step), nil
	case "apply.load-image":
		step := map[string]any{
			"images":    firstStringList(stringListOverride(overrides, "images"), ctx.ImageList),
			"sourceDir": firstString(stringOverride(overrides, "sourceDir"), ctx.ImageOutputDir, "images/control-plane"),
			"runtime":   firstString(stringOverride(overrides, "runtime"), "ctr"),
			"timeout":   firstString(stringOverride(overrides, "timeout"), "10m"),
		}
		return "load-images", workflowStep("apply-load-images", "LoadImage", "", step), nil
	case "apply.init-kubeadm":
		joinFile := firstString(stringOverride(overrides, "joinFile"), ctx.JoinFile, "/tmp/deck/join.txt")
		step := map[string]any{
			"outputJoinFile": joinFile,
			"podNetworkCIDR": firstString(stringOverride(overrides, "podCIDR"), ctx.PodCIDR, "10.244.0.0/16"),
			"timeout":        firstString(stringOverride(overrides, "timeout"), "20m"),
		}
		if value := stringOverride(overrides, "kubernetesVersion"); value != "" {
			step["kubernetesVersion"] = value
		}
		if value := stringOverride(overrides, "criSocket"); value != "" {
			step["criSocket"] = value
		}
		return "bootstrap", workflowStep("apply-init-control-plane", "InitKubeadm", firstString(stringOverride(overrides, "whenRole"), defaultRoleWhen(ctx, "control-plane")), step), nil
	case "apply.join-kubeadm":
		step := map[string]any{
			"joinFile": firstString(stringOverride(overrides, "joinFile"), ctx.JoinFile, "/tmp/deck/join.txt"),
			"timeout":  firstString(stringOverride(overrides, "timeout"), "15m"),
		}
		return "join", workflowStep("apply-join-worker", "JoinKubeadm", firstString(stringOverride(overrides, "whenRole"), defaultRoleWhen(ctx, "worker")), step), nil
	case "apply.check-cluster":
		total := firstInt(intOverride(overrides, "nodeCount"), ctx.NodeCount, 1)
		ready := firstInt(intOverride(overrides, "readyCount"), total)
		controlPlaneReady := firstInt(intOverride(overrides, "controlPlaneReady"), ctx.ControlPlaneReady, 1)
		step := map[string]any{
			"interval": firstString(stringOverride(overrides, "interval"), "5s"),
			"timeout":  firstString(stringOverride(overrides, "timeout"), defaultCheckTimeout(total)),
			"nodes": map[string]any{
				"total":             total,
				"ready":             ready,
				"controlPlaneReady": controlPlaneReady,
			},
		}
		return "verify", workflowStep("apply-check-cluster", "CheckCluster", firstString(stringOverride(overrides, "whenRole"), defaultRoleWhen(ctx, "control-plane")), step), nil
	default:
		return "", askcontract.WorkflowStep{}, fmt.Errorf("unsupported draft builder %q for %s", selected.ID, path)
	}
}

func validateOverrideKeys(builderID string, overrides map[string]any) error {
	allowed := allowedOverrideKeys(builderID)
	for key := range overrides {
		if !allowed[strings.TrimSpace(key)] {
			return fmt.Errorf("draft builder %s does not support override %q", builderID, key)
		}
	}
	return nil
}

func allowedOverrideKeys(builderID string) map[string]bool {
	keys := map[string][]string{
		"prepare.download-package": {"packages", "distroFamily", "distroRelease", "repoType", "backendRuntime", "backendImage", "outputDir", "timeout"},
		"prepare.download-image":   {"images", "outputDir", "backendEngine", "timeout"},
		"apply.install-package":    {"packages", "sourcePath", "timeout"},
		"apply.load-image":         {"images", "sourceDir", "runtime", "timeout"},
		"apply.init-kubeadm":       {"joinFile", "podCIDR", "kubernetesVersion", "criSocket", "whenRole", "timeout"},
		"apply.join-kubeadm":       {"joinFile", "whenRole", "timeout"},
		"apply.check-cluster":      {"nodeCount", "readyCount", "controlPlaneReady", "interval", "timeout", "whenRole"},
	}
	allowed := map[string]bool{}
	for _, key := range keys[builderID] {
		allowed[key] = true
	}
	return allowed
}

func inferSharedContext(selection askcontract.DraftSelection) sharedContext {
	ctx := sharedContext{RoleSelectorVar: "role"}
	for _, target := range selection.Targets {
		for _, builder := range target.Builders {
			switch builder.ID {
			case "prepare.download-package":
				ctx.PackageList = firstStringList(ctx.PackageList, stringListOverride(builder.Overrides, "packages"))
				ctx.DistroFamily = firstString(ctx.DistroFamily, stringOverride(builder.Overrides, "distroFamily"))
				ctx.DistroRelease = firstString(ctx.DistroRelease, stringOverride(builder.Overrides, "distroRelease"))
				ctx.RepoType = firstString(ctx.RepoType, stringOverride(builder.Overrides, "repoType"))
				ctx.PackageOutputDir = firstString(ctx.PackageOutputDir, stringOverride(builder.Overrides, "outputDir"))
			case "prepare.download-image":
				ctx.ImageList = firstStringList(ctx.ImageList, stringListOverride(builder.Overrides, "images"))
				ctx.ImageOutputDir = firstString(ctx.ImageOutputDir, stringOverride(builder.Overrides, "outputDir"))
			case "apply.init-kubeadm":
				ctx.JoinFile = firstString(ctx.JoinFile, stringOverride(builder.Overrides, "joinFile"))
				ctx.PodCIDR = firstString(ctx.PodCIDR, stringOverride(builder.Overrides, "podCIDR"))
			case "apply.join-kubeadm":
				ctx.JoinFile = firstString(ctx.JoinFile, stringOverride(builder.Overrides, "joinFile"))
			case "apply.check-cluster":
				ctx.NodeCount = firstInt(ctx.NodeCount, intOverride(builder.Overrides, "nodeCount"))
				ctx.ControlPlaneReady = firstInt(ctx.ControlPlaneReady, intOverride(builder.Overrides, "controlPlaneReady"))
			}
		}
	}
	return ctx
}

func workflowStep(id string, kind string, when string, spec any) askcontract.WorkflowStep {
	step := askcontract.WorkflowStep{ID: id, Kind: kind, Spec: specMap(spec)}
	if strings.TrimSpace(when) != "" {
		step.When = strings.TrimSpace(when)
	}
	return step
}

func specMap(value any) map[string]any {
	raw, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func documentKind(path string, kind string) string {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if strings.TrimSpace(kind) != "" {
		return strings.ToLower(strings.TrimSpace(kind))
	}
	if clean == "workflows/vars.yaml" {
		return "vars"
	}
	if strings.HasPrefix(clean, "workflows/components/") {
		return "component"
	}
	return "workflow"
}

func defaultRepoType(family string) string {
	switch strings.ToLower(strings.TrimSpace(family)) {
	case "debian":
		return "deb-flat"
	default:
		return "rpm"
	}
}

func defaultPackageOutputDir(family string, release string, repoType string) string {
	family = strings.ToLower(strings.TrimSpace(family))
	release = strings.TrimSpace(release)
	repoType = strings.ToLower(strings.TrimSpace(repoType))
	if release == "" {
		return "packages/"
	}
	if repoType == "deb-flat" || family == "debian" {
		return filepath.ToSlash(filepath.Join("packages", "deb", release))
	}
	return filepath.ToSlash(filepath.Join("packages", "rpm", release))
}

func defaultBackendImage(family string, release string) string {
	family = strings.ToLower(strings.TrimSpace(family))
	release = strings.TrimSpace(release)
	if family == "debian" {
		return "ubuntu:22.04"
	}
	if release == "9" || strings.Contains(strings.ToLower(release), "rhel9") || strings.Contains(strings.ToLower(release), "rocky9") {
		return "rockylinux:9"
	}
	return "rockylinux:9"
}

func defaultRoleWhen(ctx sharedContext, role string) string {
	if ctx.NodeCount <= 1 {
		return ""
	}
	return fmt.Sprintf("vars.%s == %q", ctx.RoleSelectorVar, role)
}

func defaultCheckTimeout(total int) string {
	if total > 1 {
		return "10m"
	}
	return "5m"
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstStringList(values ...[]string) []string {
	for _, value := range values {
		if len(value) > 0 {
			return append([]string(nil), value...)
		}
	}
	return nil
}

func firstInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func stringOverride(overrides map[string]any, key string) string {
	if len(overrides) == 0 {
		return ""
	}
	value, ok := overrides[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func stringListOverride(overrides map[string]any, key string) []string {
	if len(overrides) == 0 {
		return nil
	}
	value, ok := overrides[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return dedupeStrings(typed)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				out = append(out, text)
			}
		}
		return dedupeStrings(out)
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" {
			return nil
		}
		return []string{text}
	}
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
		out[key] = value
	}
	return out
}

func hasCapability(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(want) {
			return true
		}
	}
	return false
}

func dedupeCandidates(items []Candidate) []Candidate {
	seen := map[string]bool{}
	out := make([]Candidate, 0, len(items))
	for _, item := range items {
		if seen[item.ID+"|"+item.Path] {
			continue
		}
		seen[item.ID+"|"+item.Path] = true
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
