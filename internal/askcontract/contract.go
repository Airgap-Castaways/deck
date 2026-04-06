package askcontract

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/structuredpath"
	"github.com/Airgap-Castaways/deck/internal/workflowissues"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

type flexStrings []string

func (f *flexStrings) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*f = nil
		return nil
	}
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		single = strings.TrimSpace(single)
		if single == "" {
			*f = nil
		} else {
			*f = []string{single}
		}
		return nil
	}
	var singleObject map[string]any
	if err := json.Unmarshal(data, &singleObject); err == nil {
		if extracted, ok := flexStringFromObject(singleObject); ok {
			*f = []string{extracted}
			return nil
		}
	}
	var many []string
	if err := json.Unmarshal(data, &many); err != nil {
		var items []any
		if err := json.Unmarshal(data, &items); err != nil {
			return err
		}
		out := make([]string, 0, len(items))
		for _, item := range items {
			switch typed := item.(type) {
			case string:
				typed = strings.TrimSpace(typed)
				if typed != "" {
					out = append(out, typed)
				}
			case map[string]any:
				if extracted, ok := flexStringFromObject(typed); ok {
					out = append(out, extracted)
				}
			}
		}
		*f = out
		return nil
	}
	out := make([]string, 0, len(many))
	for _, item := range many {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	*f = out
	return nil
}

func flexStringFromObject(value map[string]any) (string, bool) {
	for _, key := range []string{"question", "text", "message", "title", "label"} {
		raw, ok := value[key]
		if !ok {
			continue
		}
		text, ok := raw.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text != "" {
			return text, true
		}
	}
	return "", false
}

type GeneratedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Delete  bool   `json:"delete,omitempty"`
}

type GeneratedDocument struct {
	Path       string                  `json:"path"`
	Kind       string                  `json:"kind,omitempty"`
	Action     string                  `json:"action,omitempty"`
	Workflow   *WorkflowDocument       `json:"workflow,omitempty"`
	Component  *ComponentDocument      `json:"component,omitempty"`
	Vars       map[string]any          `json:"vars,omitempty"`
	Edits      []StructuredEditAction  `json:"edits,omitempty"`
	Transforms []RefineTransformAction `json:"transforms,omitempty"`
}

type WorkflowDocument struct {
	Version string          `json:"version"`
	Vars    map[string]any  `json:"vars,omitempty"`
	Phases  []WorkflowPhase `json:"phases,omitempty"`
	Steps   []WorkflowStep  `json:"steps,omitempty"`
}

type WorkflowPhase struct {
	Name           string         `json:"name"`
	MaxParallelism int            `json:"maxParallelism,omitempty"`
	Imports        []PhaseImport  `json:"imports,omitempty"`
	Steps          []WorkflowStep `json:"steps,omitempty"`
}

type PhaseImport struct {
	Path string `json:"path"`
	When string `json:"when,omitempty"`
}

func (p *PhaseImport) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*p = PhaseImport{}
		return nil
	}
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*p = PhaseImport{Path: strings.TrimSpace(single)}
		return nil
	}
	type alias PhaseImport
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*p = PhaseImport(value)
	p.Path = strings.TrimSpace(p.Path)
	p.When = strings.TrimSpace(p.When)
	return nil
}

type WorkflowStep struct {
	ID            string            `json:"id"`
	APIVersion    string            `json:"apiVersion,omitempty"`
	Kind          string            `json:"kind"`
	Metadata      map[string]any    `json:"metadata,omitempty"`
	When          string            `json:"when,omitempty"`
	ParallelGroup string            `json:"parallelGroup,omitempty"`
	Register      map[string]string `json:"register,omitempty"`
	Retry         int               `json:"retry,omitempty"`
	Timeout       string            `json:"timeout,omitempty"`
	Spec          map[string]any    `json:"spec"`
}

type ComponentDocument struct {
	Steps []WorkflowStep `json:"steps"`
}

type StructuredEditAction struct {
	Op           string         `json:"op"`
	RawPath      string         `json:"rawPath"`
	Path         string         `json:"path,omitempty"`
	StepID       string         `json:"stepId,omitempty"`
	TargetStepID string         `json:"targetStepId,omitempty"`
	Target       map[string]any `json:"target,omitempty"`
	Value        any            `json:"value,omitempty"`
}

type RefineTransformAction struct {
	Type      string `json:"type"`
	Candidate string `json:"candidate,omitempty"`
	RawPath   string `json:"rawPath,omitempty"`
	VarName   string `json:"varName,omitempty"`
	VarsPath  string `json:"varsPath,omitempty"`
	Path      string `json:"path,omitempty"`
	Value     any    `json:"value,omitempty"`
}

type GenerationResponse struct {
	Summary   string              `json:"summary"`
	Review    []string            `json:"review"`
	Files     []GeneratedFile     `json:"-"`
	Documents []GeneratedDocument `json:"documents,omitempty"`
	Selection *DraftSelection     `json:"selection,omitempty"`
	Program   *AuthoringProgram   `json:"-"`
}

type DraftSelection struct {
	Patterns []string               `json:"patterns,omitempty"`
	Targets  []DraftTargetSelection `json:"targets,omitempty"`
	Vars     map[string]any         `json:"vars,omitempty"`
}

type DraftBuilderSelection struct {
	ID        string         `json:"id"`
	Overrides map[string]any `json:"overrides,omitempty"`
}

type DraftTargetSelection struct {
	Path     string                  `json:"path"`
	Kind     string                  `json:"kind,omitempty"`
	Builders []DraftBuilderSelection `json:"builders,omitempty"`
	Phases   []DraftPhaseSelection   `json:"phases,omitempty"`
	Steps    []WorkflowStep          `json:"steps,omitempty"`
	Vars     map[string]any          `json:"vars,omitempty"`
}

type DraftPhaseSelection struct {
	Name    string         `json:"name"`
	Imports []PhaseImport  `json:"imports,omitempty"`
	Steps   []WorkflowStep `json:"steps,omitempty"`
}

type PlanFile struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Action  string `json:"action"`
	Purpose string `json:"purpose"`
}

type PlanResponse struct {
	Version                 int                 `json:"version"`
	Request                 string              `json:"request"`
	Intent                  string              `json:"intent"`
	Complexity              string              `json:"complexity"`
	AuthoringBrief          AuthoringBrief      `json:"authoringBrief,omitempty"`
	AuthoringProgram        AuthoringProgram    `json:"authoringProgram,omitempty"`
	ExecutionModel          ExecutionModel      `json:"executionModel,omitempty"`
	OfflineAssumption       string              `json:"offlineAssumption,omitempty"`
	NeedsPrepare            bool                `json:"needsPrepare,omitempty"`
	ArtifactKinds           []string            `json:"artifactKinds,omitempty"`
	VarsRecommendation      []string            `json:"varsRecommendation,omitempty"`
	ComponentRecommendation []string            `json:"componentRecommendation,omitempty"`
	Blockers                []string            `json:"blockers"`
	TargetOutcome           string              `json:"targetOutcome"`
	Assumptions             []string            `json:"assumptions"`
	OpenQuestions           []string            `json:"openQuestions"`
	Clarifications          []PlanClarification `json:"clarifications,omitempty"`
	EntryScenario           string              `json:"entryScenario"`
	Files                   []PlanFile          `json:"files"`
	ValidationChecklist     []string            `json:"validationChecklist"`
}

type PlanClarification struct {
	ID                 string   `json:"id"`
	Question           string   `json:"question"`
	Kind               string   `json:"kind,omitempty"`
	Reason             string   `json:"reason,omitempty"`
	Decision           string   `json:"decision,omitempty"`
	Options            []string `json:"options,omitempty"`
	RecommendedDefault string   `json:"recommendedDefault,omitempty"`
	Answer             string   `json:"answer,omitempty"`
	BlocksGeneration   bool     `json:"blocksGeneration,omitempty"`
	Affects            []string `json:"affects,omitempty"`
}

type AuthoringBrief struct {
	RouteIntent              string   `json:"routeIntent,omitempty"`
	TargetScope              string   `json:"targetScope,omitempty"`
	TargetPaths              []string `json:"targetPaths,omitempty"`
	AnchorPaths              []string `json:"anchorPaths,omitempty"`
	AllowedCompanionPaths    []string `json:"allowedCompanionPaths,omitempty"`
	DisallowedExpansionPaths []string `json:"disallowedExpansionPaths,omitempty"`
	ModeIntent               string   `json:"modeIntent,omitempty"`
	Connectivity             string   `json:"connectivity,omitempty"`
	CompletenessTarget       string   `json:"completenessTarget,omitempty"`
	Topology                 string   `json:"topology,omitempty"`
	NodeCount                int      `json:"nodeCount,omitempty"`
	PlatformFamily           string   `json:"platformFamily,omitempty"`
	EscapeHatchMode          string   `json:"escapeHatchMode,omitempty"`
	RequiredCapabilities     []string `json:"requiredCapabilities,omitempty"`
}

type AuthoringProgram struct {
	Platform     ProgramPlatform     `json:"platform,omitempty"`
	Artifacts    ProgramArtifacts    `json:"artifacts,omitempty"`
	Cluster      ProgramCluster      `json:"cluster,omitempty"`
	Verification ProgramVerification `json:"verification,omitempty"`
}

type ProgramPlatform struct {
	Family       string `json:"family,omitempty"`
	Release      string `json:"release,omitempty"`
	RepoType     string `json:"repoType,omitempty"`
	BackendImage string `json:"backendImage,omitempty"`
}

type ProgramArtifacts struct {
	Packages         []string `json:"packages,omitempty"`
	Images           []string `json:"images,omitempty"`
	PackageOutputDir string   `json:"packageOutputDir,omitempty"`
	ImageOutputDir   string   `json:"imageOutputDir,omitempty"`
}

type ProgramCluster struct {
	JoinFile          string `json:"joinFile,omitempty"`
	PodCIDR           string `json:"podCIDR,omitempty"`
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
	CriSocket         string `json:"criSocket,omitempty"`
	RoleSelector      string `json:"roleSelector,omitempty"`
	ControlPlaneCount int    `json:"controlPlaneCount,omitempty"`
	WorkerCount       int    `json:"workerCount,omitempty"`
}

type ProgramVerification struct {
	ExpectedNodeCount         int    `json:"expectedNodeCount,omitempty"`
	ExpectedReadyCount        int    `json:"expectedReadyCount,omitempty"`
	ExpectedControlPlaneReady int    `json:"expectedControlPlaneReady,omitempty"`
	FinalVerificationRole     string `json:"finalVerificationRole,omitempty"`
	Interval                  string `json:"interval,omitempty"`
	Timeout                   string `json:"timeout,omitempty"`
}

type planResponseLoose struct {
	Version                 int                   `json:"version"`
	Request                 string                `json:"request"`
	Intent                  string                `json:"intent"`
	Complexity              string                `json:"complexity"`
	AuthoringBrief          authoringBriefLoose   `json:"authoringBrief,omitempty"`
	AuthoringProgram        authoringProgramLoose `json:"authoringProgram,omitempty"`
	ExecutionModel          executionModelLoose   `json:"executionModel,omitempty"`
	OfflineAssumption       string                `json:"offlineAssumption,omitempty"`
	NeedsPrepare            bool                  `json:"needsPrepare,omitempty"`
	ArtifactKinds           flexStrings           `json:"artifactKinds,omitempty"`
	VarsRecommendation      flexStrings           `json:"varsRecommendation,omitempty"`
	ComponentRecommendation flexStrings           `json:"componentRecommendation,omitempty"`
	Blockers                flexStrings           `json:"blockers"`
	TargetOutcome           string                `json:"targetOutcome"`
	Assumptions             flexStrings           `json:"assumptions"`
	OpenQuestions           flexStrings           `json:"openQuestions"`
	Clarifications          []planClarification   `json:"clarifications,omitempty"`
	EntryScenario           string                `json:"entryScenario"`
	Files                   []PlanFile            `json:"files"`
	ValidationChecklist     flexStrings           `json:"validationChecklist"`
}

type authoringBriefLoose struct {
	RouteIntent              string      `json:"routeIntent,omitempty"`
	TargetScope              string      `json:"targetScope,omitempty"`
	TargetPaths              flexStrings `json:"targetPaths,omitempty"`
	AnchorPaths              flexStrings `json:"anchorPaths,omitempty"`
	AllowedCompanionPaths    flexStrings `json:"allowedCompanionPaths,omitempty"`
	DisallowedExpansionPaths flexStrings `json:"disallowedExpansionPaths,omitempty"`
	ModeIntent               string      `json:"modeIntent,omitempty"`
	Connectivity             string      `json:"connectivity,omitempty"`
	CompletenessTarget       string      `json:"completenessTarget,omitempty"`
	Topology                 string      `json:"topology,omitempty"`
	NodeCount                int         `json:"nodeCount,omitempty"`
	PlatformFamily           string      `json:"platformFamily,omitempty"`
	EscapeHatchMode          string      `json:"escapeHatchMode,omitempty"`
	RequiredCapabilities     flexStrings `json:"requiredCapabilities,omitempty"`
}

type authoringProgramLoose struct {
	Platform     ProgramPlatform       `json:"platform,omitempty"`
	Artifacts    programArtifactsLoose `json:"artifacts,omitempty"`
	Cluster      ProgramCluster        `json:"cluster,omitempty"`
	Verification ProgramVerification   `json:"verification,omitempty"`
}

type programArtifactsLoose struct {
	Packages         flexStrings `json:"packages,omitempty"`
	Images           flexStrings `json:"images,omitempty"`
	PackageOutputDir string      `json:"packageOutputDir,omitempty"`
	ImageOutputDir   string      `json:"imageOutputDir,omitempty"`
}

type executionModelLoose struct {
	ArtifactContracts    []ArtifactContract         `json:"artifactContracts,omitempty"`
	SharedStateContracts []sharedStateContractLoose `json:"sharedStateContracts,omitempty"`
	RoleExecution        RoleExecutionModel         `json:"roleExecution,omitempty"`
	Verification         VerificationStrategy       `json:"verification,omitempty"`
	ApplyAssumptions     flexStrings                `json:"applyAssumptions,omitempty"`
}

type sharedStateContractLoose struct {
	Name              string      `json:"name,omitempty"`
	ProducerPath      string      `json:"producerPath,omitempty"`
	ConsumerPaths     flexStrings `json:"consumerPaths,omitempty"`
	AvailabilityModel string      `json:"availabilityModel,omitempty"`
	Description       string      `json:"description,omitempty"`
}

type planClarification struct {
	ID                 string      `json:"id"`
	Question           string      `json:"question"`
	Kind               string      `json:"kind,omitempty"`
	Reason             string      `json:"reason,omitempty"`
	Decision           string      `json:"decision,omitempty"`
	Options            flexStrings `json:"options,omitempty"`
	RecommendedDefault string      `json:"recommendedDefault,omitempty"`
	Answer             string      `json:"answer,omitempty"`
	BlocksGeneration   bool        `json:"blocksGeneration,omitempty"`
	Affects            flexStrings `json:"affects,omitempty"`
}

func (p AuthoringProgram) Value(path string) (any, bool) {
	switch strings.TrimSpace(path) {
	case "platform.family":
		return strings.TrimSpace(p.Platform.Family), strings.TrimSpace(p.Platform.Family) != ""
	case "platform.release":
		return strings.TrimSpace(p.Platform.Release), strings.TrimSpace(p.Platform.Release) != ""
	case "platform.repoType":
		return strings.TrimSpace(p.Platform.RepoType), strings.TrimSpace(p.Platform.RepoType) != ""
	case "platform.backendImage":
		return strings.TrimSpace(p.Platform.BackendImage), strings.TrimSpace(p.Platform.BackendImage) != ""
	case "artifacts.packages":
		return append([]string(nil), p.Artifacts.Packages...), len(p.Artifacts.Packages) > 0
	case "artifacts.images":
		return append([]string(nil), p.Artifacts.Images...), len(p.Artifacts.Images) > 0
	case "artifacts.packageOutputDir":
		return strings.TrimSpace(p.Artifacts.PackageOutputDir), strings.TrimSpace(p.Artifacts.PackageOutputDir) != ""
	case "artifacts.imageOutputDir":
		return strings.TrimSpace(p.Artifacts.ImageOutputDir), strings.TrimSpace(p.Artifacts.ImageOutputDir) != ""
	case "cluster.joinFile":
		return strings.TrimSpace(p.Cluster.JoinFile), strings.TrimSpace(p.Cluster.JoinFile) != ""
	case "cluster.podCIDR":
		return strings.TrimSpace(p.Cluster.PodCIDR), strings.TrimSpace(p.Cluster.PodCIDR) != ""
	case "cluster.kubernetesVersion":
		return strings.TrimSpace(p.Cluster.KubernetesVersion), strings.TrimSpace(p.Cluster.KubernetesVersion) != ""
	case "cluster.criSocket":
		return strings.TrimSpace(p.Cluster.CriSocket), strings.TrimSpace(p.Cluster.CriSocket) != ""
	case "cluster.roleSelector":
		return strings.TrimSpace(p.Cluster.RoleSelector), strings.TrimSpace(p.Cluster.RoleSelector) != ""
	case "cluster.controlPlaneCount":
		return p.Cluster.ControlPlaneCount, p.Cluster.ControlPlaneCount > 0
	case "cluster.workerCount":
		return p.Cluster.WorkerCount, p.Cluster.WorkerCount > 0
	case "verification.expectedNodeCount":
		return p.Verification.ExpectedNodeCount, p.Verification.ExpectedNodeCount > 0
	case "verification.expectedReadyCount":
		return p.Verification.ExpectedReadyCount, p.Verification.ExpectedReadyCount > 0
	case "verification.expectedControlPlaneReady":
		return p.Verification.ExpectedControlPlaneReady, p.Verification.ExpectedControlPlaneReady > 0
	case "verification.finalVerificationRole":
		return strings.TrimSpace(p.Verification.FinalVerificationRole), strings.TrimSpace(p.Verification.FinalVerificationRole) != ""
	case "verification.interval":
		return strings.TrimSpace(p.Verification.Interval), strings.TrimSpace(p.Verification.Interval) != ""
	case "verification.timeout":
		return strings.TrimSpace(p.Verification.Timeout), strings.TrimSpace(p.Verification.Timeout) != ""
	case "cluster.roleWhen.control-plane":
		return roleWhenExpression(p.Cluster.RoleSelector, p.Cluster.ControlPlaneCount, p.Cluster.WorkerCount, "control-plane")
	case "cluster.roleWhen.worker":
		return roleWhenExpression(p.Cluster.RoleSelector, p.Cluster.ControlPlaneCount, p.Cluster.WorkerCount, "worker")
	case "verification.roleWhen":
		role := strings.TrimSpace(p.Verification.FinalVerificationRole)
		if role == "" || role == "local" {
			return "", false
		}
		return roleWhenExpression(p.Cluster.RoleSelector, p.Cluster.ControlPlaneCount, p.Cluster.WorkerCount, role)
	default:
		return nil, false
	}
}

func roleWhenExpression(selector string, controlPlaneCount int, workerCount int, role string) (string, bool) {
	selector = strings.TrimSpace(selector)
	if selector == "" || selector == "nil" || selector == "<nil>" || (controlPlaneCount+workerCount) <= 1 {
		return "", false
	}
	if strings.TrimSpace(role) == "" {
		return "", false
	}
	return "vars." + selector + ` == "` + strings.TrimSpace(role) + `"`, true
}

type ExecutionModel struct {
	ArtifactContracts    []ArtifactContract    `json:"artifactContracts,omitempty"`
	SharedStateContracts []SharedStateContract `json:"sharedStateContracts,omitempty"`
	RoleExecution        RoleExecutionModel    `json:"roleExecution,omitempty"`
	Verification         VerificationStrategy  `json:"verification,omitempty"`
	ApplyAssumptions     []string              `json:"applyAssumptions,omitempty"`
}

type ArtifactContract struct {
	Kind         string `json:"kind,omitempty"`
	ProducerPath string `json:"producerPath,omitempty"`
	ConsumerPath string `json:"consumerPath,omitempty"`
	Description  string `json:"description,omitempty"`
}

type SharedStateContract struct {
	Name              string   `json:"name,omitempty"`
	ProducerPath      string   `json:"producerPath,omitempty"`
	ConsumerPaths     []string `json:"consumerPaths,omitempty"`
	AvailabilityModel string   `json:"availabilityModel,omitempty"`
	Description       string   `json:"description,omitempty"`
}

type RoleExecutionModel struct {
	RoleSelector      string `json:"roleSelector,omitempty"`
	ControlPlaneFlow  string `json:"controlPlaneFlow,omitempty"`
	WorkerFlow        string `json:"workerFlow,omitempty"`
	PerNodeInvocation bool   `json:"perNodeInvocation,omitempty"`
}

type VerificationStrategy struct {
	BootstrapPhase            string `json:"bootstrapPhase,omitempty"`
	FinalPhase                string `json:"finalPhase,omitempty"`
	FinalVerificationRole     string `json:"finalVerificationRole,omitempty"`
	ExpectedNodeCount         int    `json:"expectedNodeCount,omitempty"`
	ExpectedControlPlaneReady int    `json:"expectedControlPlaneReady,omitempty"`
}

type CriticResponse struct {
	Blocking       []string `json:"blocking"`
	Advisory       []string `json:"advisory"`
	MissingFiles   []string `json:"missingFiles"`
	InvalidImports []string `json:"invalidImports"`
	CoverageGaps   []string `json:"coverageGaps"`
	RequiredFixes  []string `json:"requiredFixes"`
}

type JudgeResponse struct {
	Summary             string   `json:"summary"`
	Blocking            []string `json:"blocking"`
	Advisory            []string `json:"advisory"`
	MissingCapabilities []string `json:"missingCapabilities"`
	SuggestedFixes      []string `json:"suggestedFixes"`
}

type PlanCriticResponse struct {
	Summary          string              `json:"summary"`
	Blocking         []string            `json:"blocking"`
	Advisory         []string            `json:"advisory"`
	MissingContracts []string            `json:"missingContracts"`
	SuggestedFixes   []string            `json:"suggestedFixes"`
	Findings         []PlanCriticFinding `json:"findings,omitempty"`
}

type PlanCriticFinding struct {
	Code        workflowissues.Code     `json:"code"`
	Severity    workflowissues.Severity `json:"severity"`
	Message     string                  `json:"message"`
	Path        string                  `json:"path,omitempty"`
	Recoverable bool                    `json:"recoverable,omitempty"`
}

type PostProcessResponse struct {
	Summary                  string   `json:"summary"`
	Blocking                 []string `json:"blocking"`
	Advisory                 []string `json:"advisory"`
	UpgradeCandidates        []string `json:"upgradeCandidates"`
	ReviseFiles              []string `json:"reviseFiles"`
	PreserveFiles            []string `json:"preserveFiles"`
	RequiredEdits            []string `json:"requiredEdits"`
	VerificationExpectations []string `json:"verificationExpectations"`
	SuggestedFixes           []string `json:"suggestedFixes"`
}

func GenerationResponseSchema() json.RawMessage {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"summary", "review"},
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
			"review":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"selection": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"patterns": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"vars":     openObjectSchema(),
					"targets": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"required":             []string{"path"},
							"properties": map[string]any{
								"path": map[string]any{"type": "string"},
								"kind": map[string]any{"type": "string"},
								"builders": map[string]any{
									"type": "array",
									"items": map[string]any{
										"type":                 "object",
										"additionalProperties": false,
										"required":             []string{"id"},
										"properties": map[string]any{
											"id":        map[string]any{"type": "string"},
											"overrides": openObjectSchema(),
										},
									},
								},
								"steps":  openObjectSchema(),
								"phases": openObjectSchema(),
								"vars":   openObjectSchema(),
							},
						},
					},
				},
			},
			"documents": map[string]any{
				"type":     "array",
				"minItems": 1,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"path"},
					"properties": map[string]any{
						"path":      map[string]any{"type": "string"},
						"kind":      map[string]any{"type": "string"},
						"action":    map[string]any{"type": "string"},
						"workflow":  openObjectSchema(),
						"component": openObjectSchema(),
						"vars":      openObjectSchema(),
						"transforms": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type":                 "object",
								"additionalProperties": false,
								"required":             []string{"type"},
								"properties": map[string]any{
									"type":      map[string]any{"type": "string"},
									"candidate": map[string]any{"type": "string"},
									"rawPath":   map[string]any{"type": "string"},
									"varName":   map[string]any{"type": "string"},
									"varsPath":  map[string]any{"type": "string"},
									"path":      map[string]any{"type": "string"},
									"value":     map[string]any{},
								},
							},
						},
						"edits": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type":                 "object",
								"additionalProperties": false,
								"required":             []string{"op", "rawPath"},
								"properties": map[string]any{
									"op":      map[string]any{"type": "string"},
									"rawPath": map[string]any{"type": "string"},
									"value":   map[string]any{},
								},
							},
						},
					},
				},
			},
		},
		"anyOf": []any{
			map[string]any{"required": []string{"documents"}},
			map[string]any{"required": []string{"selection"}},
		},
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		panic(err)
	}
	return raw
}

func openObjectSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": true}
}

type InfoResponse struct {
	Summary         string   `json:"summary"`
	Answer          string   `json:"answer"`
	Suggestions     []string `json:"suggestions"`
	Findings        []string `json:"findings"`
	SuggestedChange []string `json:"suggestedChanges"`
}

type ClassificationResponse struct {
	Route             string               `json:"route"`
	Confidence        float64              `json:"confidence"`
	Reason            string               `json:"reason"`
	Target            ClassificationTarget `json:"target"`
	GenerationAllowed *bool                `json:"generationAllowed,omitempty"`
	ReviewStyle       string               `json:"reviewStyle,omitempty"`
}

type ClassificationTarget struct {
	Kind string `json:"kind,omitempty"`
	Path string `json:"path,omitempty"`
	Name string `json:"name,omitempty"`
}

type EvidencePlan struct {
	Decision                string           `json:"decision"`
	Reason                  string           `json:"reason,omitempty"`
	FreshnessSensitive      bool             `json:"freshnessSensitive,omitempty"`
	InstallEvidence         bool             `json:"installEvidence,omitempty"`
	CompatibilityEvidence   bool             `json:"compatibilityEvidence,omitempty"`
	TroubleshootingEvidence bool             `json:"troubleshootingEvidence,omitempty"`
	Entities                []EvidenceEntity `json:"entities,omitempty"`
}

type EvidenceEntity struct {
	Name string `json:"name"`
	Kind string `json:"kind,omitempty"`
}

func ParseGeneration(raw string) (GenerationResponse, error) {
	cleaned := clean(raw)
	if cleaned == "" {
		return GenerationResponse{}, fmt.Errorf("model returned empty response")
	}
	var resp GenerationResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return GenerationResponse{}, fmt.Errorf("parse generation response: %w", err)
	}
	if strings.TrimSpace(resp.Summary) == "" {
		resp.Summary = "No summary provided."
	}
	for i := range resp.Documents {
		resp.Documents[i].Path = strings.TrimSpace(resp.Documents[i].Path)
		resp.Documents[i].Kind = normalizeDocumentKind(resp.Documents[i].Kind)
		resp.Documents[i].Action = normalizeDocumentAction(resp.Documents[i].Action)
		if resp.Documents[i].Action == "edit" && len(resp.Documents[i].Edits) == 0 && len(resp.Documents[i].Transforms) == 0 && (resp.Documents[i].Workflow != nil || resp.Documents[i].Component != nil || resp.Documents[i].Vars != nil) {
			resp.Documents[i].Action = "replace"
		}
		if strings.EqualFold(resp.Documents[i].Kind, "vars") && resp.Documents[i].Vars == nil {
			resp.Documents[i].Vars = map[string]any{}
		}
		for j := range resp.Documents[i].Edits {
			resp.Documents[i].Edits[j].Op = normalizeEditOp(resp.Documents[i].Edits[j].Op)
			resp.Documents[i].Edits[j].RawPath = normalizeEditPath(resp.Documents[i].Edits[j].RawPath, resp.Documents[i].Edits[j].Path, firstNonEmpty(resp.Documents[i].Edits[j].StepID, resp.Documents[i].Edits[j].TargetStepID), resp.Documents[i].Edits[j].Target)
			resp.Documents[i].Edits[j].RawPath = normalizeVarsDocumentRawPath(resp.Documents[i].Path, resp.Documents[i].Edits[j].RawPath)
			resp.Documents[i].Edits[j].Path = ""
			resp.Documents[i].Edits[j].StepID = ""
			resp.Documents[i].Edits[j].TargetStepID = ""
			resp.Documents[i].Edits[j].Target = nil
		}
		for j := range resp.Documents[i].Transforms {
			resp.Documents[i].Transforms[j].Type = normalizeTransformType(resp.Documents[i].Transforms[j].Type)
			resp.Documents[i].Transforms[j].Candidate = strings.TrimSpace(resp.Documents[i].Transforms[j].Candidate)
			resp.Documents[i].Transforms[j].RawPath = strings.TrimSpace(resp.Documents[i].Transforms[j].RawPath)
			resp.Documents[i].Transforms[j].VarName = strings.TrimSpace(resp.Documents[i].Transforms[j].VarName)
			resp.Documents[i].Transforms[j].VarsPath = strings.TrimSpace(resp.Documents[i].Transforms[j].VarsPath)
			normalizeExtractVarShape(&resp.Documents[i].Transforms[j])
			resp.Documents[i].Transforms[j].Path = strings.TrimSpace(resp.Documents[i].Transforms[j].Path)
			if resp.Documents[i].Transforms[j].RawPath == "" && resp.Documents[i].Transforms[j].Type != "extract-component" {
				resp.Documents[i].Transforms[j].RawPath = resp.Documents[i].Transforms[j].Path
			}
			resp.Documents[i].Transforms[j].RawPath = normalizeVarsDocumentRawPath(resp.Documents[i].Path, resp.Documents[i].Transforms[j].RawPath)
			resp.Documents[i].Transforms[j].Path = normalizeVarsDocumentRawPath(resp.Documents[i].Path, resp.Documents[i].Transforms[j].Path)
		}
	}
	if resp.Selection != nil {
		for i := range resp.Selection.Patterns {
			resp.Selection.Patterns[i] = strings.TrimSpace(resp.Selection.Patterns[i])
		}
		for i := range resp.Selection.Targets {
			resp.Selection.Targets[i].Path = strings.TrimSpace(resp.Selection.Targets[i].Path)
			resp.Selection.Targets[i].Kind = normalizeDocumentKind(resp.Selection.Targets[i].Kind)
			for j := range resp.Selection.Targets[i].Builders {
				resp.Selection.Targets[i].Builders[j].ID = strings.TrimSpace(resp.Selection.Targets[i].Builders[j].ID)
			}
		}
	}
	if len(resp.Documents) == 0 && resp.Selection == nil {
		return GenerationResponse{}, fmt.Errorf("generation response did not include documents or selection")
	}
	if len(resp.Documents) == 0 && resp.Selection != nil && !SelectionUsesBuilders(*resp.Selection) {
		resp.Documents = compileDraftSelection(*resp.Selection)
	}
	if len(resp.Documents) > 0 {
		if err := validateGeneratedDocuments(resp.Documents); err != nil {
			return GenerationResponse{}, err
		}
	}
	return resp, nil
}

func ParseEvidencePlan(raw string) (EvidencePlan, error) {
	cleaned := clean(raw)
	if cleaned == "" {
		return EvidencePlan{}, fmt.Errorf("model returned empty response")
	}
	var plan EvidencePlan
	if err := json.Unmarshal([]byte(cleaned), &plan); err != nil {
		return EvidencePlan{}, fmt.Errorf("parse evidence plan: %w", err)
	}
	plan.Decision = normalizeEvidenceDecision(plan.Decision)
	if plan.Decision == "" {
		return EvidencePlan{}, fmt.Errorf("evidence plan is missing decision")
	}
	plan.Reason = strings.TrimSpace(plan.Reason)
	seen := map[string]bool{}
	normalized := make([]EvidenceEntity, 0, len(plan.Entities))
	for _, entity := range plan.Entities {
		entity.Name = strings.TrimSpace(entity.Name)
		entity.Kind = strings.TrimSpace(entity.Kind)
		if entity.Name == "" {
			continue
		}
		key := strings.ToLower(entity.Name) + "::" + strings.ToLower(entity.Kind)
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, entity)
	}
	plan.Entities = normalized
	return plan, nil
}

func CompileDraftSelection(selection DraftSelection) []GeneratedDocument {
	return compileDraftSelection(selection)
}

func SelectionUsesBuilders(selection DraftSelection) bool {
	for _, target := range selection.Targets {
		if len(target.Builders) > 0 {
			return true
		}
	}
	return false
}

func validateGeneratedDocuments(documents []GeneratedDocument) error {
	for _, doc := range documents {
		if strings.TrimSpace(doc.Path) == "" {
			return fmt.Errorf("generated document path is empty")
		}
		action := strings.TrimSpace(doc.Action)
		if action == "" {
			action = inferredDocumentAction(doc)
		}
		switch action {
		case "preserve":
			continue
		case "delete":
			if doc.Workflow != nil || doc.Component != nil || doc.Vars != nil || len(doc.Edits) > 0 {
				return fmt.Errorf("generated document %s delete action must not include content or edits", doc.Path)
			}
		case "edit":
			if len(doc.Edits) == 0 && len(doc.Transforms) == 0 && (doc.Workflow != nil || doc.Component != nil || doc.Vars != nil) {
				action = "replace"
			}
			if action == "edit" && len(doc.Edits) == 0 && len(doc.Transforms) == 0 {
				return fmt.Errorf("generated document %s edit action must include edits or transforms", doc.Path)
			}
			if action == "edit" && (doc.Workflow != nil || doc.Component != nil || doc.Vars != nil) && len(doc.Transforms) == 0 {
				return fmt.Errorf("generated document %s edit action must not include replacement content without transforms", doc.Path)
			}
			if action != "replace" {
				continue
			}
			fallthrough
		case "replace", "create":
			if err := validateDocumentPayload(doc); err != nil {
				return err
			}
		default:
			return fmt.Errorf("generated document %s uses unsupported action %q", doc.Path, action)
		}
	}
	return nil
}

func validateDocumentPayload(doc GeneratedDocument) error {
	count := 0
	if doc.Workflow != nil {
		count++
	}
	if doc.Component != nil {
		count++
	}
	if doc.Vars != nil {
		count++
	}
	if count != 1 {
		return fmt.Errorf("generated document %s must include exactly one of workflow, component, or vars", doc.Path)
	}
	return nil
}

func inferredDocumentAction(doc GeneratedDocument) string {
	if len(doc.Edits) > 0 {
		return "edit"
	}
	if doc.Workflow != nil || doc.Component != nil || doc.Vars != nil {
		return "replace"
	}
	return "preserve"
}

func compileDraftSelection(selection DraftSelection) []GeneratedDocument {
	documents := make([]GeneratedDocument, 0, len(selection.Targets)+1)
	for _, target := range selection.Targets {
		path := strings.TrimSpace(target.Path)
		if path == "" {
			continue
		}
		kind := normalizeDocumentKind(target.Kind)
		switch kind {
		case "vars":
			documents = append(documents, GeneratedDocument{Path: path, Kind: "vars", Vars: cloneMap(target.Vars)})
		case "component":
			documents = append(documents, GeneratedDocument{Path: path, Kind: "component", Component: &ComponentDocument{Steps: append([]WorkflowStep(nil), target.Steps...)}})
		default:
			workflow := &WorkflowDocument{Version: "v1alpha1", Vars: cloneMap(target.Vars), Steps: append([]WorkflowStep(nil), target.Steps...)}
			if len(target.Phases) > 0 {
				workflow.Phases = make([]WorkflowPhase, 0, len(target.Phases))
				for _, phase := range target.Phases {
					workflow.Phases = append(workflow.Phases, WorkflowPhase{Name: strings.TrimSpace(phase.Name), Imports: append([]PhaseImport(nil), phase.Imports...), Steps: append([]WorkflowStep(nil), phase.Steps...)})
				}
			}
			documents = append(documents, GeneratedDocument{Path: path, Kind: "workflow", Workflow: workflow})
		}
	}
	if len(selection.Vars) > 0 && !selectionHasVarsTarget(selection) {
		documents = append(documents, GeneratedDocument{Path: "workflows/vars.yaml", Kind: "vars", Vars: cloneMap(selection.Vars)})
	}
	return documents
}

func selectionHasVarsTarget(selection DraftSelection) bool {
	for _, target := range selection.Targets {
		if normalizeDocumentKind(target.Kind) == "vars" || strings.TrimSpace(target.Path) == "workflows/vars.yaml" {
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

func normalizeDocumentKind(kind string) string {
	trimmed := strings.ToLower(strings.TrimSpace(kind))
	switch trimmed {
	case "scenario":
		return "workflow"
	default:
		return trimmed
	}
}

func normalizeEditOp(op string) string {
	trimmed := strings.ToLower(strings.TrimSpace(op))
	if trimmed == "add" {
		return "insert"
	}
	if trimmed == "remove" {
		return "delete"
	}
	if trimmed == "replace" {
		return "set"
	}
	return trimmed
}

func normalizeDocumentAction(action string) string {
	trimmed := strings.ToLower(strings.TrimSpace(action))
	switch trimmed {
	case "update", "revise":
		return "edit"
	case "patch":
		return "edit"
	case "noop", "skip":
		return "preserve"
	default:
		return trimmed
	}
}

func normalizeTransformType(kind string) string {
	trimmed := strings.ToLower(strings.TrimSpace(kind))
	switch trimmed {
	case "extract_var", "extract-vars", "extractvar":
		return "extract-var"
	case "set_field", "set-field", "update-field", "update_field":
		return "set-field"
	case "delete_field", "delete-field", "remove-field", "remove_field":
		return "delete-field"
	case "extract_component", "extract-component", "extractcomponent":
		return "extract-component"
	default:
		return trimmed
	}
}

func normalizeEditPath(rawPath string, alias string, stepID string, target map[string]any) string {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		path = strings.TrimSpace(alias)
	}
	if strings.TrimSpace(stepID) == "" && len(target) > 0 {
		if id, ok := target["id"].(string); ok {
			stepID = id
		}
		if path == "" {
			if field, ok := target["field"].(string); ok {
				path = field
			}
		}
	}
	if strings.TrimSpace(stepID) != "" && path != "" {
		path = "steps." + strings.TrimSpace(stepID) + "." + strings.TrimPrefix(path, ".")
	}
	if strings.TrimSpace(path) == "" {
		return ""
	}
	canonical, err := structuredpath.Canonicalize(path)
	if err != nil {
		return path
	}
	return canonical
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func ParseInfo(raw string) InfoResponse {
	cleaned := clean(raw)
	if cleaned == "" {
		return InfoResponse{Summary: "No response returned.", Answer: ""}
	}
	var resp InfoResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		trimmed := strings.TrimSpace(raw)
		return InfoResponse{Summary: "Answer", Answer: trimmed}
	}
	if strings.TrimSpace(resp.Summary) == "" {
		resp.Summary = "Answer"
	}
	if strings.TrimSpace(resp.Answer) == "" {
		resp.Answer = strings.TrimSpace(raw)
	}
	return resp
}

func ParseClassification(raw string) (ClassificationResponse, error) {
	cleaned := clean(raw)
	if cleaned == "" {
		return ClassificationResponse{}, fmt.Errorf("classification response is empty")
	}
	var resp ClassificationResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return ClassificationResponse{}, fmt.Errorf("parse classification response: %w", err)
	}
	resp.Route = strings.TrimSpace(resp.Route)
	resp.Reason = strings.TrimSpace(resp.Reason)
	resp.Target.Kind = strings.TrimSpace(resp.Target.Kind)
	resp.Target.Path = strings.TrimSpace(resp.Target.Path)
	resp.Target.Name = strings.TrimSpace(resp.Target.Name)
	if resp.Confidence < 0 {
		resp.Confidence = 0
	}
	if resp.Confidence > 1 {
		resp.Confidence = 1
	}
	return resp, nil
}

func normalizeEvidenceDecision(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "required", "require", "must":
		return "required"
	case "optional", "recommended", "prefer":
		return "optional"
	case "unnecessary", "none", "skip", "not-needed":
		return "unnecessary"
	default:
		return ""
	}
}

func ParsePlan(raw string) (PlanResponse, error) {
	return parsePlan(raw, true)
}

func ParsePlanPartial(raw string) (PlanResponse, error) {
	return parsePlan(raw, false)
}

func parsePlan(raw string, validate bool) (PlanResponse, error) {
	cleaned := clean(raw)
	if cleaned == "" {
		return PlanResponse{}, fmt.Errorf("plan response is empty")
	}
	if !validate {
		resp, err := parseLoosePlan(cleaned)
		if err == nil {
			normalizePlanResponse(&resp)
			return resp, nil
		}
	}
	var resp PlanResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		repaired := repairLooseJSON(cleaned)
		if repaired == cleaned || json.Unmarshal([]byte(repaired), &resp) != nil {
			loose, looseErr := parseLoosePlan(cleaned)
			if looseErr != nil {
				return PlanResponse{}, fmt.Errorf("parse plan response: %w", err)
			}
			resp = loose
		}
	}
	normalizePlanResponse(&resp)
	if !validate {
		return resp, nil
	}
	return validatePlanResponse(resp)
}

func normalizePlanResponse(resp *PlanResponse) {
	if resp == nil {
		return
	}
	if resp.Version == 0 {
		resp.Version = 1
	}
	resp.Request = strings.TrimSpace(resp.Request)
	resp.Intent = strings.TrimSpace(resp.Intent)
	resp.Complexity = strings.TrimSpace(resp.Complexity)
	resp.AuthoringBrief.RouteIntent = strings.TrimSpace(resp.AuthoringBrief.RouteIntent)
	resp.AuthoringBrief.TargetScope = strings.TrimSpace(resp.AuthoringBrief.TargetScope)
	resp.AuthoringBrief.PlatformFamily = strings.TrimSpace(resp.AuthoringBrief.PlatformFamily)
	resp.AuthoringBrief.EscapeHatchMode = strings.TrimSpace(resp.AuthoringBrief.EscapeHatchMode)
	resp.AuthoringBrief.ModeIntent = strings.TrimSpace(resp.AuthoringBrief.ModeIntent)
	resp.AuthoringBrief.Connectivity = strings.TrimSpace(resp.AuthoringBrief.Connectivity)
	resp.AuthoringBrief.CompletenessTarget = strings.TrimSpace(resp.AuthoringBrief.CompletenessTarget)
	resp.AuthoringBrief.Topology = strings.TrimSpace(resp.AuthoringBrief.Topology)
	resp.ExecutionModel.RoleExecution.RoleSelector = strings.TrimSpace(resp.ExecutionModel.RoleExecution.RoleSelector)
	resp.ExecutionModel.RoleExecution.ControlPlaneFlow = strings.TrimSpace(resp.ExecutionModel.RoleExecution.ControlPlaneFlow)
	resp.ExecutionModel.RoleExecution.WorkerFlow = strings.TrimSpace(resp.ExecutionModel.RoleExecution.WorkerFlow)
	resp.ExecutionModel.Verification.BootstrapPhase = strings.TrimSpace(resp.ExecutionModel.Verification.BootstrapPhase)
	resp.ExecutionModel.Verification.FinalPhase = strings.TrimSpace(resp.ExecutionModel.Verification.FinalPhase)
	resp.ExecutionModel.Verification.FinalVerificationRole = strings.TrimSpace(resp.ExecutionModel.Verification.FinalVerificationRole)
	resp.AuthoringProgram.Platform.Family = strings.TrimSpace(resp.AuthoringProgram.Platform.Family)
	resp.AuthoringProgram.Platform.Release = strings.TrimSpace(resp.AuthoringProgram.Platform.Release)
	resp.AuthoringProgram.Platform.RepoType = strings.TrimSpace(resp.AuthoringProgram.Platform.RepoType)
	resp.AuthoringProgram.Platform.BackendImage = strings.TrimSpace(resp.AuthoringProgram.Platform.BackendImage)
	resp.AuthoringProgram.Artifacts.PackageOutputDir = strings.TrimSpace(resp.AuthoringProgram.Artifacts.PackageOutputDir)
	resp.AuthoringProgram.Artifacts.ImageOutputDir = strings.TrimSpace(resp.AuthoringProgram.Artifacts.ImageOutputDir)
	resp.AuthoringProgram.Cluster.JoinFile = strings.TrimSpace(resp.AuthoringProgram.Cluster.JoinFile)
	resp.AuthoringProgram.Cluster.PodCIDR = strings.TrimSpace(resp.AuthoringProgram.Cluster.PodCIDR)
	resp.AuthoringProgram.Cluster.KubernetesVersion = strings.TrimSpace(resp.AuthoringProgram.Cluster.KubernetesVersion)
	resp.AuthoringProgram.Cluster.CriSocket = strings.TrimSpace(resp.AuthoringProgram.Cluster.CriSocket)
	resp.AuthoringProgram.Cluster.RoleSelector = strings.TrimSpace(resp.AuthoringProgram.Cluster.RoleSelector)
	resp.AuthoringProgram.Verification.FinalVerificationRole = strings.TrimSpace(resp.AuthoringProgram.Verification.FinalVerificationRole)
	resp.AuthoringProgram.Verification.Interval = strings.TrimSpace(resp.AuthoringProgram.Verification.Interval)
	resp.AuthoringProgram.Verification.Timeout = strings.TrimSpace(resp.AuthoringProgram.Verification.Timeout)
	resp.OfflineAssumption = strings.TrimSpace(resp.OfflineAssumption)
	resp.TargetOutcome = strings.TrimSpace(resp.TargetOutcome)
	resp.EntryScenario = strings.TrimSpace(resp.EntryScenario)
	for i := range resp.Clarifications {
		resp.Clarifications[i].ID = strings.TrimSpace(resp.Clarifications[i].ID)
		resp.Clarifications[i].Question = strings.TrimSpace(resp.Clarifications[i].Question)
		resp.Clarifications[i].Kind = strings.TrimSpace(resp.Clarifications[i].Kind)
		resp.Clarifications[i].Reason = strings.TrimSpace(resp.Clarifications[i].Reason)
		resp.Clarifications[i].Decision = strings.TrimSpace(resp.Clarifications[i].Decision)
		resp.Clarifications[i].RecommendedDefault = strings.TrimSpace(resp.Clarifications[i].RecommendedDefault)
		resp.Clarifications[i].Answer = strings.TrimSpace(resp.Clarifications[i].Answer)
		for j := range resp.Clarifications[i].Options {
			resp.Clarifications[i].Options[j] = strings.TrimSpace(resp.Clarifications[i].Options[j])
		}
		for j := range resp.Clarifications[i].Affects {
			resp.Clarifications[i].Affects[j] = strings.TrimSpace(resp.Clarifications[i].Affects[j])
		}
	}
	for i := range resp.AuthoringBrief.TargetPaths {
		resp.AuthoringBrief.TargetPaths[i] = strings.TrimSpace(resp.AuthoringBrief.TargetPaths[i])
	}
	for i := range resp.AuthoringBrief.AnchorPaths {
		resp.AuthoringBrief.AnchorPaths[i] = strings.TrimSpace(resp.AuthoringBrief.AnchorPaths[i])
	}
	for i := range resp.AuthoringBrief.AllowedCompanionPaths {
		resp.AuthoringBrief.AllowedCompanionPaths[i] = strings.TrimSpace(resp.AuthoringBrief.AllowedCompanionPaths[i])
	}
	for i := range resp.AuthoringBrief.DisallowedExpansionPaths {
		resp.AuthoringBrief.DisallowedExpansionPaths[i] = strings.TrimSpace(resp.AuthoringBrief.DisallowedExpansionPaths[i])
	}
	for i := range resp.AuthoringBrief.RequiredCapabilities {
		resp.AuthoringBrief.RequiredCapabilities[i] = strings.TrimSpace(resp.AuthoringBrief.RequiredCapabilities[i])
	}
	for i := range resp.AuthoringProgram.Artifacts.Packages {
		resp.AuthoringProgram.Artifacts.Packages[i] = strings.TrimSpace(resp.AuthoringProgram.Artifacts.Packages[i])
	}
	for i := range resp.AuthoringProgram.Artifacts.Images {
		resp.AuthoringProgram.Artifacts.Images[i] = strings.TrimSpace(resp.AuthoringProgram.Artifacts.Images[i])
	}
	for i := range resp.ExecutionModel.ArtifactContracts {
		resp.ExecutionModel.ArtifactContracts[i].Kind = strings.TrimSpace(resp.ExecutionModel.ArtifactContracts[i].Kind)
		resp.ExecutionModel.ArtifactContracts[i].ProducerPath = strings.TrimSpace(resp.ExecutionModel.ArtifactContracts[i].ProducerPath)
		resp.ExecutionModel.ArtifactContracts[i].ConsumerPath = strings.TrimSpace(resp.ExecutionModel.ArtifactContracts[i].ConsumerPath)
		resp.ExecutionModel.ArtifactContracts[i].Description = strings.TrimSpace(resp.ExecutionModel.ArtifactContracts[i].Description)
	}
	for i := range resp.ExecutionModel.SharedStateContracts {
		resp.ExecutionModel.SharedStateContracts[i].Name = strings.TrimSpace(resp.ExecutionModel.SharedStateContracts[i].Name)
		resp.ExecutionModel.SharedStateContracts[i].ProducerPath = strings.TrimSpace(resp.ExecutionModel.SharedStateContracts[i].ProducerPath)
		resp.ExecutionModel.SharedStateContracts[i].AvailabilityModel = strings.TrimSpace(resp.ExecutionModel.SharedStateContracts[i].AvailabilityModel)
		resp.ExecutionModel.SharedStateContracts[i].Description = strings.TrimSpace(resp.ExecutionModel.SharedStateContracts[i].Description)
		for j := range resp.ExecutionModel.SharedStateContracts[i].ConsumerPaths {
			resp.ExecutionModel.SharedStateContracts[i].ConsumerPaths[j] = strings.TrimSpace(resp.ExecutionModel.SharedStateContracts[i].ConsumerPaths[j])
		}
	}
	for i := range resp.ExecutionModel.ApplyAssumptions {
		resp.ExecutionModel.ApplyAssumptions[i] = strings.TrimSpace(resp.ExecutionModel.ApplyAssumptions[i])
	}
	for i := range resp.Blockers {
		resp.Blockers[i] = strings.TrimSpace(resp.Blockers[i])
	}
	for i := range resp.Assumptions {
		resp.Assumptions[i] = strings.TrimSpace(resp.Assumptions[i])
	}
	for i := range resp.OpenQuestions {
		resp.OpenQuestions[i] = strings.TrimSpace(resp.OpenQuestions[i])
	}
	for i := range resp.ValidationChecklist {
		resp.ValidationChecklist[i] = strings.TrimSpace(resp.ValidationChecklist[i])
	}
	for i := range resp.ArtifactKinds {
		resp.ArtifactKinds[i] = strings.TrimSpace(resp.ArtifactKinds[i])
	}
	for i := range resp.VarsRecommendation {
		resp.VarsRecommendation[i] = strings.TrimSpace(resp.VarsRecommendation[i])
	}
	for i := range resp.ComponentRecommendation {
		resp.ComponentRecommendation[i] = strings.TrimSpace(resp.ComponentRecommendation[i])
	}
}

func validatePlanResponse(resp PlanResponse) (PlanResponse, error) {
	if resp.Request == "" {
		return PlanResponse{}, fmt.Errorf("plan response is missing request")
	}
	if resp.Intent == "" {
		return PlanResponse{}, fmt.Errorf("plan response is missing intent")
	}
	if len(resp.Files) == 0 {
		return PlanResponse{}, fmt.Errorf("plan response is missing files")
	}
	seenClarifications := map[string]bool{}
	for _, item := range resp.Clarifications {
		if item.ID == "" {
			return PlanResponse{}, fmt.Errorf("plan response has clarification with empty id")
		}
		if item.Question == "" {
			return PlanResponse{}, fmt.Errorf("plan response clarification %q is missing question", item.ID)
		}
		if seenClarifications[item.ID] {
			return PlanResponse{}, fmt.Errorf("plan response has duplicate clarification id %q", item.ID)
		}
		seenClarifications[item.ID] = true
	}
	for i := range resp.Files {
		resp.Files[i].Path = strings.TrimSpace(resp.Files[i].Path)
		resp.Files[i].Kind = strings.TrimSpace(resp.Files[i].Kind)
		resp.Files[i].Action = strings.TrimSpace(resp.Files[i].Action)
		resp.Files[i].Purpose = strings.TrimSpace(resp.Files[i].Purpose)
		if resp.Files[i].Path == "" {
			return PlanResponse{}, fmt.Errorf("plan response has file with empty path")
		}
		if resp.Files[i].Action == "" {
			resp.Files[i].Action = "create"
		}
		switch resp.Files[i].Action {
		case "modify", "update", "create-or-modify", "create-or-update":
			if strings.HasPrefix(resp.Files[i].Path, "workflows/") {
				resp.Files[i].Action = "update"
			}
		case "create":
			// keep as-is
		}
		if !workspacepaths.IsAllowedAuthoringPath(resp.Files[i].Path) {
			return PlanResponse{}, fmt.Errorf("plan response has file outside allowed ask paths: %s", resp.Files[i].Path)
		}
	}
	if resp.EntryScenario != "" {
		if resolved := resolvePlannedEntryScenario(resp.EntryScenario, resp.Files); resolved != "" {
			resp.EntryScenario = resolved
		}
		if !workspacepaths.IsAllowedAuthoringPath(resp.EntryScenario) || !strings.HasPrefix(resp.EntryScenario, "workflows/scenarios/") {
			return PlanResponse{}, fmt.Errorf("plan response entryScenario must be a scenario path under workflows/scenarios/: %s", resp.EntryScenario)
		}
		matched := false
		for _, file := range resp.Files {
			if file.Path == resp.EntryScenario {
				matched = true
				break
			}
		}
		if !matched {
			return PlanResponse{}, fmt.Errorf("plan response entryScenario must match a planned file: %s", resp.EntryScenario)
		}
	}
	return resp, nil
}

func parseLoosePlan(cleaned string) (PlanResponse, error) {
	var loose planResponseLoose
	if err := json.Unmarshal([]byte(cleaned), &loose); err != nil {
		repaired := repairLooseJSON(cleaned)
		if repaired == cleaned || json.Unmarshal([]byte(repaired), &loose) != nil {
			return PlanResponse{}, err
		}
	}
	resp := PlanResponse{
		Version:                 loose.Version,
		Request:                 loose.Request,
		Intent:                  loose.Intent,
		Complexity:              loose.Complexity,
		AuthoringBrief:          loose.AuthoringBrief.toStrict(),
		AuthoringProgram:        loose.AuthoringProgram.toStrict(),
		ExecutionModel:          loose.ExecutionModel.toStrict(),
		OfflineAssumption:       loose.OfflineAssumption,
		NeedsPrepare:            loose.NeedsPrepare,
		ArtifactKinds:           []string(loose.ArtifactKinds),
		VarsRecommendation:      []string(loose.VarsRecommendation),
		ComponentRecommendation: []string(loose.ComponentRecommendation),
		Blockers:                []string(loose.Blockers),
		TargetOutcome:           loose.TargetOutcome,
		Assumptions:             []string(loose.Assumptions),
		OpenQuestions:           []string(loose.OpenQuestions),
		EntryScenario:           loose.EntryScenario,
		Files:                   loose.Files,
		ValidationChecklist:     []string(loose.ValidationChecklist),
	}
	for _, item := range loose.Clarifications {
		resp.Clarifications = append(resp.Clarifications, item.toStrict())
	}
	return resp, nil
}

func (b authoringBriefLoose) toStrict() AuthoringBrief {
	return AuthoringBrief{
		RouteIntent:              b.RouteIntent,
		TargetScope:              b.TargetScope,
		TargetPaths:              []string(b.TargetPaths),
		AnchorPaths:              []string(b.AnchorPaths),
		AllowedCompanionPaths:    []string(b.AllowedCompanionPaths),
		DisallowedExpansionPaths: []string(b.DisallowedExpansionPaths),
		ModeIntent:               b.ModeIntent,
		Connectivity:             b.Connectivity,
		CompletenessTarget:       b.CompletenessTarget,
		Topology:                 b.Topology,
		NodeCount:                b.NodeCount,
		PlatformFamily:           b.PlatformFamily,
		EscapeHatchMode:          b.EscapeHatchMode,
		RequiredCapabilities:     []string(b.RequiredCapabilities),
	}
}

func (p authoringProgramLoose) toStrict() AuthoringProgram {
	return AuthoringProgram{
		Platform: p.Platform,
		Artifacts: ProgramArtifacts{
			Packages:         []string(p.Artifacts.Packages),
			Images:           []string(p.Artifacts.Images),
			PackageOutputDir: p.Artifacts.PackageOutputDir,
			ImageOutputDir:   p.Artifacts.ImageOutputDir,
		},
		Cluster:      p.Cluster,
		Verification: p.Verification,
	}
}

func (m executionModelLoose) toStrict() ExecutionModel {
	out := ExecutionModel{
		ArtifactContracts: append([]ArtifactContract(nil), m.ArtifactContracts...),
		RoleExecution:     m.RoleExecution,
		Verification:      m.Verification,
		ApplyAssumptions:  []string(m.ApplyAssumptions),
	}
	for _, item := range m.SharedStateContracts {
		out.SharedStateContracts = append(out.SharedStateContracts, SharedStateContract{
			Name:              item.Name,
			ProducerPath:      item.ProducerPath,
			ConsumerPaths:     []string(item.ConsumerPaths),
			AvailabilityModel: item.AvailabilityModel,
			Description:       item.Description,
		})
	}
	return out
}

func (c planClarification) toStrict() PlanClarification {
	return PlanClarification{
		ID:                 c.ID,
		Question:           c.Question,
		Kind:               c.Kind,
		Reason:             c.Reason,
		Decision:           c.Decision,
		Options:            []string(c.Options),
		RecommendedDefault: c.RecommendedDefault,
		Answer:             c.Answer,
		BlocksGeneration:   c.BlocksGeneration,
		Affects:            []string(c.Affects),
	}
}

func resolvePlannedEntryScenario(entry string, files []PlanFile) string {
	entry = filepath.ToSlash(strings.TrimSpace(entry))
	if strings.HasPrefix(entry, "workflows/scenarios/") {
		return entry
	}
	matches := []string{}
	for _, file := range files {
		path := filepath.ToSlash(strings.TrimSpace(file.Path))
		if !strings.HasPrefix(path, "workflows/scenarios/") {
			continue
		}
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if entry == path || entry == filepath.Base(path) || entry == base {
			matches = append(matches, path)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}

func normalizeVarsDocumentRawPath(docPath string, rawPath string) string {
	if filepath.ToSlash(strings.TrimSpace(docPath)) != filepath.ToSlash(filepath.Join(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowVarsRel)) {
		return strings.TrimSpace(rawPath)
	}
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "vars" {
		return ""
	}
	if strings.HasPrefix(rawPath, "vars.") {
		return strings.TrimPrefix(rawPath, "vars.")
	}
	return rawPath
}

func normalizeExtractVarShape(transform *RefineTransformAction) {
	if transform == nil || strings.TrimSpace(transform.Type) != "extract-var" {
		return
	}
	varsPath := strings.TrimSpace(transform.VarsPath)
	if varsPath == "" || strings.HasPrefix(filepath.ToSlash(varsPath), workspacepaths.WorkflowRootDir+"/") {
		return
	}
	if strings.TrimSpace(transform.VarName) == "" {
		transform.VarName = varsPath
	}
	transform.VarsPath = ""
}

func ParseJudge(raw string) (JudgeResponse, error) {
	cleaned := clean(raw)
	if cleaned == "" {
		return JudgeResponse{}, fmt.Errorf("judge response is empty")
	}
	var resp JudgeResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return JudgeResponse{}, fmt.Errorf("parse judge response: %w", err)
	}
	resp.Summary = strings.TrimSpace(resp.Summary)
	for i := range resp.Blocking {
		resp.Blocking[i] = strings.TrimSpace(resp.Blocking[i])
	}
	for i := range resp.Advisory {
		resp.Advisory[i] = strings.TrimSpace(resp.Advisory[i])
	}
	for i := range resp.MissingCapabilities {
		resp.MissingCapabilities[i] = strings.TrimSpace(resp.MissingCapabilities[i])
	}
	for i := range resp.SuggestedFixes {
		resp.SuggestedFixes[i] = strings.TrimSpace(resp.SuggestedFixes[i])
	}
	return resp, nil
}

func ParsePlanCritic(raw string) (PlanCriticResponse, error) {
	cleaned := clean(raw)
	if cleaned == "" {
		return PlanCriticResponse{}, fmt.Errorf("plan critic response is empty")
	}
	var resp PlanCriticResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return PlanCriticResponse{}, fmt.Errorf("parse plan critic response: %w", err)
	}
	resp.Summary = strings.TrimSpace(resp.Summary)
	for i := range resp.Blocking {
		resp.Blocking[i] = strings.TrimSpace(resp.Blocking[i])
	}
	for i := range resp.Advisory {
		resp.Advisory[i] = strings.TrimSpace(resp.Advisory[i])
	}
	for i := range resp.MissingContracts {
		resp.MissingContracts[i] = strings.TrimSpace(resp.MissingContracts[i])
	}
	for i := range resp.SuggestedFixes {
		resp.SuggestedFixes[i] = strings.TrimSpace(resp.SuggestedFixes[i])
	}
	for i := range resp.Findings {
		resp.Findings[i].Code = workflowissues.Code(strings.TrimSpace(string(resp.Findings[i].Code)))
		resp.Findings[i].Severity = workflowissues.Severity(strings.TrimSpace(string(resp.Findings[i].Severity)))
		resp.Findings[i].Message = strings.TrimSpace(resp.Findings[i].Message)
		resp.Findings[i].Path = strings.TrimSpace(resp.Findings[i].Path)
		if resp.Findings[i].Code == "" {
			return PlanCriticResponse{}, fmt.Errorf("plan critic finding is missing code")
		}
		if !workflowissues.IsSupportedCriticCode(resp.Findings[i].Code) {
			return PlanCriticResponse{}, fmt.Errorf("plan critic finding %q uses unsupported code", resp.Findings[i].Code)
		}
		if resp.Findings[i].Message == "" {
			return PlanCriticResponse{}, fmt.Errorf("plan critic finding %q is missing message", resp.Findings[i].Code)
		}
		switch resp.Findings[i].Severity {
		case workflowissues.SeverityBlocking, workflowissues.SeverityAdvisory, workflowissues.SeverityMissingContract:
			// ok
		case "":
			resp.Findings[i].Severity = workflowissues.SeverityAdvisory
		default:
			return PlanCriticResponse{}, fmt.Errorf("plan critic finding %q has invalid severity %q", resp.Findings[i].Code, resp.Findings[i].Severity)
		}
	}
	return resp, nil
}

func ParsePostProcess(raw string) (PostProcessResponse, error) {
	cleaned := clean(raw)
	if cleaned == "" {
		return PostProcessResponse{}, fmt.Errorf("post-process response is empty")
	}
	var resp PostProcessResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return PostProcessResponse{}, fmt.Errorf("parse post-process response: %w", err)
	}
	resp.Summary = strings.TrimSpace(resp.Summary)
	for i := range resp.Blocking {
		resp.Blocking[i] = strings.TrimSpace(resp.Blocking[i])
	}
	for i := range resp.Advisory {
		resp.Advisory[i] = strings.TrimSpace(resp.Advisory[i])
	}
	for i := range resp.UpgradeCandidates {
		resp.UpgradeCandidates[i] = strings.TrimSpace(resp.UpgradeCandidates[i])
	}
	for i := range resp.ReviseFiles {
		resp.ReviseFiles[i] = strings.TrimSpace(resp.ReviseFiles[i])
	}
	for i := range resp.PreserveFiles {
		resp.PreserveFiles[i] = strings.TrimSpace(resp.PreserveFiles[i])
	}
	for i := range resp.RequiredEdits {
		resp.RequiredEdits[i] = strings.TrimSpace(resp.RequiredEdits[i])
	}
	for i := range resp.VerificationExpectations {
		resp.VerificationExpectations[i] = strings.TrimSpace(resp.VerificationExpectations[i])
	}
	for i := range resp.SuggestedFixes {
		resp.SuggestedFixes[i] = strings.TrimSpace(resp.SuggestedFixes[i])
	}
	return resp, nil
}

func clean(response string) string {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start >= 0 && end > start {
		response = response[start : end+1]
	}
	return strings.TrimSpace(response)
}

func repairLooseJSON(response string) string {
	response = strings.ReplaceAll(response, ",]", "]")
	response = strings.ReplaceAll(response, ", }", " }")
	response = strings.ReplaceAll(response, ",}", "}")
	response = strings.ReplaceAll(response, ",\n]", "\n]")
	response = strings.ReplaceAll(response, ",\n}", "\n}")
	return response
}
