package askretrieve

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askstate"
)

func TestRetrieveOmitsTypedStepProseChunksForAllRoutes(t *testing.T) {
	prompt := "create an air-gapped rhel9 single-node kubeadm workflow using typed steps where possible"
	draft := Retrieve(askintent.RouteDraft, prompt, askintent.Target{}, WorkspaceSummary{}, askstate.Context{}, nil)
	review := Retrieve(askintent.RouteReview, prompt, askintent.Target{}, WorkspaceSummary{}, askstate.Context{}, nil)
	draftChunk := findChunk(draft, "typed-steps-draft")
	reviewChunk := findChunk(review, "typed-steps-review")
	if draftChunk != nil || reviewChunk != nil {
		t.Fatalf("expected route-specific typed-step chunks, got draft=%v review=%v", draft.Chunks, review.Chunks)
	}
	for _, chunk := range review.Chunks {
		if chunk.Source == "askcontext" && chunk.Label == "typed-steps" {
			t.Fatalf("expected review retrieval to omit typed-step prose chunks, got %#v", review.Chunks)
		}
	}
}

func TestRetrievePrefersRepositoryExamplesOverTypedGuidanceForDraft(t *testing.T) {
	result := Retrieve(askintent.RouteDraft, "create an air-gapped rhel9 single-node kubeadm workflow using typed steps where possible", askintent.Target{}, WorkspaceSummary{}, askstate.Context{}, nil)
	if findChunk(result, "typed-steps-draft") != nil {
		t.Fatalf("expected draft retrieval to omit typed-step guidance chunk, got %#v", result.Chunks)
	}
	foundExample := false
	for _, chunk := range result.Chunks {
		if chunk.Source == "example" {
			foundExample = true
			break
		}
	}
	if !foundExample {
		t.Fatalf("expected draft retrieval to include repository examples, got %#v", result.Chunks)
	}
}

func TestRetrieveIncludesStructuredMCPEvidenceChunk(t *testing.T) {
	result := Retrieve(askintent.RouteDraft, "create an air-gapped package workflow", askintent.Target{}, WorkspaceSummary{}, askstate.Context{}, []Chunk{{ID: "mcp-doc", Source: "mcp", Label: "context7:search", Topic: "mcp:context7:search", Content: "Typed MCP evidence JSON:\n{\n  \"artifactKinds\": [\"package\"],\n  \"offlineHints\": [\"Treat gathered installation artifacts as offline bundle inputs for prepare before apply.\"]\n}\n\nSource excerpt:\nDownload rpm packages before offline installation.", Score: 70, Evidence: &EvidenceSummary{ArtifactKinds: []string{"package"}, OfflineHints: []string{"Treat gathered installation artifacts as offline bundle inputs for prepare before apply."}}}})
	var found bool
	for _, chunk := range result.Chunks {
		if chunk.Source == "mcp" && strings.Contains(chunk.Content, "Typed MCP evidence JSON:") {
			if chunk.Evidence == nil || len(chunk.Evidence.ArtifactKinds) == 0 || chunk.Evidence.ArtifactKinds[0] != "package" {
				t.Fatalf("expected typed evidence summary, got %#v", chunk)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected structured mcp evidence chunk, got %#v", result.Chunks)
	}
}

func TestRetrieveIncludesLocalFactChunks(t *testing.T) {
	result := Retrieve(askintent.RouteDraft, "create kubeadm workflow using builder selection", askintent.Target{}, WorkspaceSummary{}, askstate.Context{}, nil)
	found := false
	for _, chunk := range result.Chunks {
		if chunk.Source == "local-facts" {
			found = true
			if !strings.Contains(chunk.Content, "Local facts:") {
				t.Fatalf("expected local facts prefix, got %q", chunk.Content)
			}
			if chunk.ID == "local-facts-stepspec" {
				if chunk.Label != "stepspec-facts" {
					t.Fatalf("expected fact-oriented stepspec label, got %#v", chunk)
				}
				if strings.Contains(chunk.Content, "candidate step kind") {
					t.Fatalf("expected stepspec chunk to avoid ranking language, got %q", chunk.Content)
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected local-facts chunks, got %#v", result.Chunks)
	}
}

func TestRetrieveRepoBehaviorExplainPrefersLocalFactsOverWorkspace(t *testing.T) {
	workspace := WorkspaceSummary{Files: []WorkspaceFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps: []\n"}}}
	result := Retrieve(askintent.RouteExplain, "Explain how InitKubeadm and CheckKubernetesCluster are assembled for ask draft generation in this repo", askintent.Target{Kind: "workspace"}, workspace, askstate.Context{}, nil)
	if len(result.Chunks) == 0 {
		t.Fatalf("expected retrieval chunks")
	}
	firstLocal := -1
	firstWorkspace := -1
	for i, chunk := range result.Chunks {
		if firstLocal == -1 && chunk.Source == "local-facts" {
			firstLocal = i
		}
		if firstWorkspace == -1 && chunk.Source == "workspace" {
			firstWorkspace = i
		}
	}
	if firstLocal == -1 {
		t.Fatalf("expected local facts for repo-behavior explain, got %#v", result.Chunks)
	}
	if firstWorkspace != -1 && firstLocal > firstWorkspace {
		t.Fatalf("expected local facts before workspace chunks for repo-behavior explain, got %#v", result.Chunks)
	}
	if chunk := findChunk(result, "local-facts-askpolicy"); chunk == nil || !strings.Contains(chunk.Content, "askpolicy") {
		t.Fatalf("expected askpolicy local facts for repo-behavior explain, got %#v", result.Chunks)
	}
}

func TestRetrieveDraftUsesAuthoringFactAssemblyWithoutAskContextOrState(t *testing.T) {
	result := Retrieve(
		askintent.RouteDraft,
		"create an air-gapped kubeadm prepare and apply workflow with worker join",
		askintent.Target{},
		WorkspaceSummary{},
		askstate.Context{LastLint: "old lint output"},
		[]Chunk{{ID: "mcp-doc", Source: "mcp", Label: "web-search:kubernetes.io", Topic: "mcp:web-search:kubernetes.io", Content: "Typed MCP evidence JSON:\n{}"}},
	)
	for _, chunk := range result.Chunks {
		if chunk.Source == "askcontext" {
			t.Fatalf("expected draft retrieval to avoid generic askcontext blocks, got %#v", result.Chunks)
		}
		if chunk.Source == "state" {
			t.Fatalf("expected draft retrieval to omit state facts, got %#v", result.Chunks)
		}
	}
	if len(filterChunksBySource(result, "example")) > 2 {
		t.Fatalf("expected draft retrieval to cap examples per fact group, got %#v", result.Chunks)
	}
	if len(filterChunksBySource(result, "mcp")) == 0 {
		t.Fatalf("expected draft retrieval to retain external evidence facts, got %#v", result.Chunks)
	}
}

func TestRetrieveQuestionIncludesInformationalFactAssembly(t *testing.T) {
	result := Retrieve(askintent.RouteQuestion, "what does the workflow do", askintent.Target{}, WorkspaceSummary{}, askstate.Context{}, nil)
	chunk := findChunk(result, "workflow-meta")
	if chunk == nil || chunk.Source != "askcontext" {
		t.Fatalf("expected question retrieval to include informational workflow facts, got %#v", result.Chunks)
	}
	if len(filterChunksBySource(result, "example")) != 0 {
		t.Fatalf("expected question retrieval to omit authoring examples, got %#v", result.Chunks)
	}
}

func TestBuildChunkTextSeparatesLocalFactsAndExternalEvidence(t *testing.T) {
	text := BuildChunkText(RetrievalResult{Chunks: []Chunk{{ID: "facts-1", Source: "local-facts", Label: "source-of-truth-stepmeta", Topic: "local-facts:stepmeta", Content: "Local facts:\n- path: internal/stepmeta/registry.go"}, {ID: "mcp-1", Source: "mcp", Label: "web-search:kubernetes.io", Topic: "mcp:web-search:kubernetes.io", Content: "Typed MCP evidence JSON:\n{}"}, {ID: "workspace-1", Source: "workspace", Label: "workflows/scenarios/apply.yaml", Topic: "workspace:workflows/scenarios/apply.yaml", Content: "version: v1alpha1"}}})
	for _, want := range []string{"Local facts:", "External evidence:", "Retrieved context:", "[chunk:facts-1,source:local-facts", "[chunk:mcp-1,source:mcp", "[chunk:workspace-1,source:workspace"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in grouped chunk text, got %q", want, text)
		}
	}
}

func TestRetrieveAddsReferenceExamplesForComplexAuthoringPrompt(t *testing.T) {
	result := Retrieve(askintent.RouteDraft, "create an air-gapped kubeadm prepare and apply workflow with worker join", askintent.Target{}, WorkspaceSummary{}, askstate.Context{}, nil)
	found := false
	for _, chunk := range result.Chunks {
		if chunk.Source == "example" {
			found = true
			if !strings.Contains(chunk.Content, "Reference example:") {
				t.Fatalf("expected example chunk wrapper, got %q", chunk.Content)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected example reference chunks for complex authoring prompt, got %#v", result.Chunks)
	}
}

func TestRouteBudgetExpandsForComplexAuthoringPrompt(t *testing.T) {
	baseBytes, baseChunks := routeBudget(askintent.RouteDraft, "create workflow")
	complexBytes, complexChunks := routeBudget(askintent.RouteDraft, "create an air-gapped kubeadm prepare and apply workflow with worker join")
	if complexBytes <= baseBytes || complexChunks <= baseChunks {
		t.Fatalf("expected complex budget expansion, got base=(%d,%d) complex=(%d,%d)", baseBytes, baseChunks, complexBytes, complexChunks)
	}
}

func TestRetrieveReservesExamplesWithoutTypedPromptHintsForComplexAuthoringPrompt(t *testing.T) {
	result := Retrieve(askintent.RouteDraft, "create an air-gapped rhel9 3-node kubeadm prepare and apply workflow with worker join", askintent.Target{}, WorkspaceSummary{}, askstate.Context{}, nil)
	exampleCount := 0
	for _, chunk := range result.Chunks {
		if chunk.Source == "example" {
			exampleCount++
		}
		if chunk.Source == "askcontext" && (chunk.Label == "typed-steps" || chunk.Label == "step-composition") {
			t.Fatalf("expected complex draft retrieval to omit typed-step prose chunks, got %#v", result.Chunks)
		}
	}
	if exampleCount == 0 {
		t.Fatalf("expected reserved examples for complex authoring prompt, got %#v", result.Chunks)
	}
}

func TestCompressChunkContentDoesNotTrimYAML(t *testing.T) {
	content := strings.Join([]string{
		"version: v1alpha1",
		"phases:",
		"  - name: preflight",
		"    steps:",
		"      - id: check-host",
		"        kind: CheckHost",
		"  - name: bootstrap",
		"    steps:",
		"      - id: init-cluster",
		"        kind: InitKubeadm",
		"      - id: publish-join",
		"        kind: CopyFile",
		"  - name: workers",
		"    steps:",
		"      - id: join-worker",
		"        kind: JoinKubeadm",
		"  - name: verify",
		"    steps:",
		"      - id: check-kubernetes-cluster",
		"        kind: CheckKubernetesCluster",
	}, "\n")
	compressed := compressChunkContent("multi-node kubeadm worker join handoff", "workflows/scenarios/apply.yaml", content, 80)
	if compressed != content {
		t.Fatalf("expected yaml content to remain uncompressed, got %q", compressed)
	}
	if shouldCompressChunk("workflows/scenarios/apply.yaml", content) {
		t.Fatalf("expected yaml chunk to skip compression")
	}
}

func TestExampleChunkScorePrefersRepoNativeCurrentExamples(t *testing.T) {
	prompt := "create an air-gapped kubeadm prepare and apply workflow with worker join"
	docsGuide := exampleChunkScore(prompt, "docs/guides/examples/offline-k8s-worker.yaml", "version: v1alpha1\napiVersion: deck/v1alpha1\nkind: JoinKubeadm")
	repoNative := exampleChunkScore(prompt, "test/workflows/scenarios/worker-join.yaml", "version: v1alpha1\nkind: JoinKubeadm")
	if repoNative <= docsGuide {
		t.Fatalf("expected repo-native workflow example to outrank docs guide example, got repo=%d docs=%d", repoNative, docsGuide)
	}
}

func TestExampleChunkAllowedKeepsCurrentCanonicalSources(t *testing.T) {
	if !exampleChunkAllowed("docs/guides/examples/offline-k8s-worker.yaml", "version: v1alpha1\napiVersion: deck/v1alpha1\nkind: JoinKubeadm") {
		t.Fatalf("expected current docs guide example to remain eligible")
	}
	if !exampleChunkAllowed("test/workflows/scenarios/worker-join.yaml", "version: v1alpha1\nkind: JoinKubeadm") {
		t.Fatalf("expected repo-native example to remain eligible")
	}
	if exampleChunkAllowed("docs/examples/offline-k8s-worker.yaml", "version: v1alpha1\nkind: JoinKubeadm") {
		t.Fatalf("expected non-canonical example path to be filtered out")
	}
}

func TestExampleChunkScorePenalizesIrrelevantUpgradeExamples(t *testing.T) {
	prompt := "create an air-gapped kubeadm prepare and apply workflow with worker join"
	upgrade := exampleChunkScore(prompt, "test/workflows/scenarios/upgrade.yaml", "version: v1alpha1\nkind: UpgradeKubeadm")
	join := exampleChunkScore(prompt, "test/workflows/scenarios/worker-join.yaml", "version: v1alpha1\nkind: JoinKubeadm")
	if join <= upgrade {
		t.Fatalf("expected join-focused example to outrank upgrade example, got join=%d upgrade=%d", join, upgrade)
	}
}

func findChunk(result RetrievalResult, id string) *Chunk {
	for i := range result.Chunks {
		if result.Chunks[i].ID == id {
			return &result.Chunks[i]
		}
	}
	return nil
}

func filterChunksBySource(result RetrievalResult, source string) []Chunk {
	chunks := make([]Chunk, 0)
	for _, chunk := range result.Chunks {
		if chunk.Source == source {
			chunks = append(chunks, chunk)
		}
	}
	return chunks
}
