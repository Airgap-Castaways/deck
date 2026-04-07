package askcatalog

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/schemafacts"
	"github.com/Airgap-Castaways/deck/internal/stepmeta"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

type Catalog struct {
	Workflow   WorkflowRules
	Workspace  WorkspaceRules
	Policy     PolicyRules
	Modes      []ModeRules
	Components ComponentRules
	Vars       VarsRules
	Steps      map[string]Step
	ordered    []string
}

type WorkflowRules struct {
	Summary          string
	SupportedVersion string
	TopLevelModes    []string
	SupportedModes   []string
	RequiredFields   []string
	ImportRule       string
	PhaseRules       []string
	StepRules        []string
	PhaseExample     string
	StepsExample     string
	InvariantNotes   []string
	SourceRefs       []string
}

type WorkspaceRules struct {
	WorkflowRoot      string
	ScenarioDir       string
	ComponentDir      string
	VarsPath          string
	AllowedPaths      []string
	CanonicalPrepare  string
	CanonicalApply    string
	GeneratedPathNote string
	SourceRefs        []string
}

type PolicyRules struct {
	AssumeOfflineByDefault bool
	PreferTypedSteps       bool
	PrepareArtifactKinds   []string
	ForbiddenApplyActions  []string
	VarsAdvisory           []string
	ComponentAdvisory      []string
	SourceRefs             []string
}

type ModeRules struct {
	Mode        string
	Summary     string
	WhenToUse   string
	Prefer      []string
	Avoid       []string
	OutputFiles []string
	SourceRefs  []string
}

type ComponentRules struct {
	Summary         string
	ImportRule      string
	ReuseRule       string
	LocationRule    string
	FragmentRule    string
	ImportExample   string
	FragmentExample string
	AllowedRootKeys []string
	SourceRefs      []string
}

type VarsRules struct {
	Path        string
	Summary     string
	PreferFor   []string
	AvoidFor    []string
	ExampleKeys []string
	SourceRefs  []string
}

type Step struct {
	Kind                     string
	Category                 string
	Group                    string
	GroupTitle               string
	Summary                  string
	WhenToUse                string
	SchemaFile               string
	AllowedRoles             []string
	Outputs                  []string
	Capabilities             []string
	Contract                 ContractBindings
	MatchSignals             []string
	AntiSignals              []string
	ValidationHints          []stepmeta.ValidationHint
	ConstrainedLiteralFields []stepmeta.ConstrainedLiteralField
	QualityRules             []stepmeta.QualityRule
	KeyFields                []string
	Fields                   map[string]Field
	RuleSummaries            []string
	Builders                 []Builder
	SourceRefs               []string
}

type ContractBindings struct {
	ProducesArtifacts   []string
	ConsumesArtifacts   []string
	PublishesState      []string
	ConsumesState       []string
	RoleSensitive       bool
	VerificationRelated bool
}

type Field struct {
	Path               string
	Type               string
	Requirement        schemafacts.RequirementLevel
	Default            string
	Pattern            string
	Enum               []string
	Description        string
	Example            string
	ConstrainedLiteral bool
	Guidance           string
	SourceRef          string
}

type Builder struct {
	ID                   string
	StepKind             string
	Phase                string
	DefaultStepID        string
	Summary              string
	RequiresCapabilities []string
	Bindings             []Binding
	SourceRefs           []string
}

type Binding struct {
	Path     string
	From     string
	Semantic string
	Required bool
}

func (c Catalog) StepKinds() []Step {
	out := make([]Step, 0, len(c.ordered))
	for _, kind := range c.ordered {
		if step, ok := c.Steps[kind]; ok {
			out = append(out, step)
		}
	}
	return out
}

func (c Catalog) LookupStep(kind string) (Step, bool) {
	for _, candidate := range stepAliasCandidates(kind) {
		if step, ok := c.Steps[candidate]; ok {
			return step, true
		}
	}
	return Step{}, false
}

func (c Catalog) LookupField(kind string, path string) (Field, bool) {
	step, ok := c.LookupStep(kind)
	if !ok {
		return Field{}, false
	}
	field, ok := step.Fields[path]
	return field, ok
}

func (c Catalog) LookupBuilder(id string) (Builder, bool) {
	for _, step := range c.StepKinds() {
		for _, builder := range step.Builders {
			for _, candidate := range builderAliasCandidates(id) {
				if builder.ID == candidate {
					return builder, true
				}
			}
		}
	}
	return Builder{}, false
}

func (c Catalog) BuildersForPath(path string) []Builder {
	role := roleForPath(path)
	if role == "" {
		return nil
	}
	out := []Builder{}
	for _, step := range c.StepKinds() {
		if !containsRole(step.AllowedRoles, role) {
			continue
		}
		out = append(out, step.Builders...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (c Catalog) BuilderIDsForPath(path string) []string {
	builders := c.BuildersForPath(path)
	if len(builders) == 0 {
		return nil
	}
	out := make([]string, 0, len(builders))
	seen := map[string]bool{}
	for _, builder := range builders {
		id := strings.TrimSpace(builder.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func (b Builder) OverrideKeys() []string {
	keys := []string{}
	seen := map[string]bool{}
	for _, binding := range b.Bindings {
		const prefix = "override:"
		if len(binding.From) <= len(prefix) || binding.From[:len(prefix)] != prefix {
			continue
		}
		key := binding.From[len(prefix):]
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (b Builder) RequiredOverrideKeys() []string {
	keys := []string{}
	seen := map[string]bool{}
	for _, binding := range b.Bindings {
		const prefix = "override:"
		if len(binding.From) <= len(prefix) || binding.From[:len(prefix)] != prefix || !binding.Required {
			continue
		}
		key := binding.From[len(prefix):]
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (b Builder) OptionalOverrideKeys() []string {
	required := map[string]bool{}
	for _, key := range b.RequiredOverrideKeys() {
		required[key] = true
	}
	optional := []string{}
	for _, key := range b.OverrideKeys() {
		if !required[key] {
			optional = append(optional, key)
		}
	}
	return optional
}

func roleForPath(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	switch {
	case path == workspacepaths.CanonicalPrepareWorkflow:
		return "prepare"
	case workspacepaths.IsScenarioAuthoringPath(path):
		return "apply"
	default:
		return ""
	}
}

func containsRole(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}

func stepAliasCandidates(kind string) []string {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return nil
	}
	switch kind {
	case "CheckCluster":
		return []string{"CheckCluster", "CheckKubernetesCluster"}
	case "CheckKubernetesCluster":
		return []string{"CheckKubernetesCluster", "CheckCluster"}
	default:
		return []string{kind}
	}
}

func builderAliasCandidates(id string) []string {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	switch id {
	case "apply.check-cluster":
		return []string{"apply.check-cluster", "apply.check-kubernetes-cluster"}
	case "apply.check-kubernetes-cluster":
		return []string{"apply.check-kubernetes-cluster", "apply.check-cluster"}
	default:
		return []string{id}
	}
}
