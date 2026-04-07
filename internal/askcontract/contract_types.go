package askcontract

import (
	"encoding/json"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/workflowissues"
)

var flexStringObjectKeys = []string{"question", "text", "message", "title", "label"}

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
	for _, key := range flexStringObjectKeys {
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
