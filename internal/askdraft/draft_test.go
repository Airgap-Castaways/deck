package askdraft

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func TestCompileBuildsWorkflowDocumentsFromBuilderSelections(t *testing.T) {
	selection := askcontract.DraftSelection{
		Targets: []askcontract.DraftTargetSelection{
			{
				Path: "workflows/prepare.yaml",
				Builders: []askcontract.DraftBuilderSelection{{
					ID: "prepare.download-package",
					Overrides: map[string]any{
						"packages":      []any{"kubeadm", "kubelet", "kubectl"},
						"distroFamily":  "rhel",
						"distroRelease": "9",
					},
				}},
			},
			{
				Path:     "workflows/scenarios/apply.yaml",
				Builders: []askcontract.DraftBuilderSelection{{ID: "apply.init-kubeadm", Overrides: map[string]any{"joinFile": "/tmp/deck/join.txt"}}, {ID: "apply.check-cluster", Overrides: map[string]any{"nodeCount": 1}}},
			},
		},
		Vars: map[string]any{"role": "control-plane"},
	}
	docs, err := Compile(selection)
	if err != nil {
		t.Fatalf("compile builder selection: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected prepare, apply, and vars docs, got %#v", docs)
	}
	byPath := map[string]askcontract.GeneratedDocument{}
	for _, doc := range docs {
		byPath[doc.Path] = doc
	}
	if byPath["workflows/prepare.yaml"].Workflow == nil || byPath["workflows/scenarios/apply.yaml"].Workflow == nil || byPath["workflows/vars.yaml"].Vars == nil {
		t.Fatalf("expected compiled workflow and vars documents, got %#v", byPath)
	}
	prepare := byPath["workflows/prepare.yaml"].Workflow
	if len(prepare.Phases) != 1 || prepare.Phases[0].Steps[0].Kind != "DownloadPackage" {
		t.Fatalf("expected typed prepare builder output, got %#v", prepare)
	}
	apply := byPath["workflows/scenarios/apply.yaml"].Workflow
	joinedKinds := []string{}
	for _, phase := range apply.Phases {
		for _, step := range phase.Steps {
			joinedKinds = append(joinedKinds, step.Kind)
		}
	}
	if !strings.Contains(strings.Join(joinedKinds, ","), "InitKubeadm") || !strings.Contains(strings.Join(joinedKinds, ","), "CheckCluster") {
		t.Fatalf("expected kubeadm builder steps, got %#v", apply)
	}
}
