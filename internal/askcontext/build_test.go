package askcontext

import (
	"os"
	"strings"
	"testing"

	"github.com/taedi90/deck/internal/schemadoc"
	"github.com/taedi90/deck/internal/workflowexec"
)

func TestManifestIncludesAllStepKinds(t *testing.T) {
	manifest := Current()
	seen := map[string]bool{}
	for _, step := range manifest.StepKinds {
		seen[step.Kind] = true
	}
	for _, kind := range workflowexec.StepKinds() {
		if !seen[kind] {
			t.Fatalf("missing step kind in manifest: %s", kind)
		}
	}
}

func TestDocsReferenceCLIIncludesCoreAskContext(t *testing.T) {
	raw, err := os.ReadFile("/home/opencode/workspace/deck/docs/reference/cli.md")
	if err != nil {
		t.Fatalf("read docs: %v", err)
	}
	text := string(raw)
	for _, want := range []string{"workflows/components/", "workflows/vars.yaml", "prepare` is the online or collection-oriented side", "Prefer typed step kinds for common host changes."} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected docs to contain %q", want)
		}
	}
}

func TestManifestWorkflowRulesMatchSchemaDoc(t *testing.T) {
	manifest := Current()
	meta := schemadoc.WorkflowMeta()
	for _, note := range meta.Notes {
		if !contains(manifest.Workflow.Notes, note) {
			t.Fatalf("missing workflow note %q", note)
		}
	}
}

func TestPromptBlocksIncludeCoreAuthoringGuidance(t *testing.T) {
	blocks := []string{
		GlobalAuthoringBlock(),
		WorkspaceTopologyBlock(),
		RoleGuidanceBlock(),
		ComponentGuidanceBlock(),
		VarsGuidanceBlock(),
		CLIHintsBlock(),
	}
	joined := strings.Join(blocks, "\n")
	for _, want := range []string{"workflows/components/", "workflows/vars.yaml", "prepare", "apply", "Prefer typed steps over Command"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in prompt blocks, got %q", want, joined)
		}
	}
}

func TestRelevantStepKindsMatchesDockerRequest(t *testing.T) {
	relevant := RelevantStepKinds("install docker on rocky9 and enable service")
	if len(relevant) == 0 {
		t.Fatalf("expected relevant steps")
	}
	joined := make([]string, 0, len(relevant))
	for _, step := range relevant {
		joined = append(joined, step.Kind)
	}
	if !contains(joined, "Packages") {
		t.Fatalf("expected Packages in relevant steps, got %v", joined)
	}
}

func TestDocBlocksExposeAskContext(t *testing.T) {
	if got := AuthoringDocBlock(); !strings.Contains(got, "workflows/components/") || !strings.Contains(got, "workflows/vars.yaml") {
		t.Fatalf("unexpected authoring doc block: %q", got)
	}
	if got := CLIDocBlock(); !strings.Contains(got, "deck ask") || !strings.Contains(got, ".deck/plan/") {
		t.Fatalf("unexpected cli doc block: %q", got)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
