package askcontext

type Manifest struct {
	CLI        CLIContext
	Topology   WorkspaceTopology
	Workflow   WorkflowRules
	Roles      []RoleGuidance
	Components ComponentGuidance
	Vars       VarsGuidance
	StepKinds  []StepKindContext
}

type CLIContext struct {
	Command             string
	PlanSubcommand      string
	TopLevelDescription string
	ImportantFlags      []CLIFlag
	Examples            []string
}

type CLIFlag struct {
	Name        string
	Description string
}

type WorkspaceTopology struct {
	WorkflowRoot      string
	ScenarioDir       string
	ComponentDir      string
	VarsPath          string
	AllowedPaths      []string
	CanonicalPrepare  string
	CanonicalApply    string
	GeneratedPathNote string
}

type WorkflowRules struct {
	Summary string
	Notes   []string
}

type RoleGuidance struct {
	Role        string
	Summary     string
	WhenToUse   string
	Prefer      []string
	Avoid       []string
	OutputFiles []string
}

type ComponentGuidance struct {
	Summary      string
	ImportRule   string
	ReuseRule    string
	LocationRule string
}

type VarsGuidance struct {
	Path        string
	Summary     string
	PreferFor   []string
	AvoidFor    []string
	ExampleKeys []string
}

type StepKindContext struct {
	Kind         string
	Category     string
	Summary      string
	WhenToUse    string
	SchemaFile   string
	AllowedRoles []string
	Actions      []string
	Outputs      []string
	Notes        []string
}
