package askcatalog

import (
	"sort"

	"github.com/Airgap-Castaways/deck/internal/schemafacts"
	"github.com/Airgap-Castaways/deck/internal/stepmeta"
)

type Catalog struct {
	Workflow  WorkflowRules
	Workspace WorkspaceRules
	Steps     map[string]Step
	ordered   []string
}

type WorkflowRules struct {
	SupportedVersion string
	TopLevelModes    []string
	SupportedModes   []string
	RequiredFields   []string
	ImportRule       string
	InvariantNotes   []string
}

type WorkspaceRules struct {
	WorkflowRoot     string
	ScenarioDir      string
	ComponentDir     string
	VarsPath         string
	AllowedPaths     []string
	CanonicalPrepare string
	CanonicalApply   string
}

type Step struct {
	Kind                     string
	Category                 string
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
}

type Builder struct {
	ID                   string
	StepKind             string
	Phase                string
	DefaultStepID        string
	Summary              string
	RequiresCapabilities []string
	Bindings             []Binding
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
	step, ok := c.Steps[kind]
	return step, ok
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
			if builder.ID == id {
				return builder, true
			}
		}
	}
	return Builder{}, false
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
