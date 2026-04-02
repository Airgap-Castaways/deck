package askir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/stepspec"
)

func TestMaterializeWorkflowDocumentRendersYAML(t *testing.T) {
	root := t.TempDir()
	files, err := Materialize(root, askcontract.GenerationResponse{
		Documents: []askcontract.GeneratedDocument{{
			Path: "workflows/scenarios/apply.yaml",
			Kind: "workflow",
			Workflow: &askcontract.WorkflowDocument{
				Version: "v1alpha1",
				Steps: []askcontract.WorkflowStep{{
					ID:   "run",
					Kind: "Command",
					Spec: map[string]any{"command": []any{"true"}},
				}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("materialize workflow document: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one materialized file, got %#v", files)
	}
	for _, want := range []string{"version: v1alpha1", "kind: Command", "spec:"} {
		if !strings.Contains(files[0].Content, want) {
			t.Fatalf("expected %q in rendered workflow, got %q", want, files[0].Content)
		}
	}
}

func TestMaterializeRefineEditsAppliesStructuredChanges(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	content := "version: v1alpha1\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [true]\n"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	files, err := Materialize(root, askcontract.GenerationResponse{
		Documents: []askcontract.GeneratedDocument{{
			Path:   "workflows/scenarios/apply.yaml",
			Action: "edit",
			Edits:  []askcontract.StructuredEditAction{{Op: "set", RawPath: "steps.0.timeout", Value: "5m"}},
		}},
	})
	if err != nil {
		t.Fatalf("materialize refine edit: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one edited file, got %#v", files)
	}
	if !strings.Contains(files[0].Content, "timeout: 5m") {
		t.Fatalf("expected structured edit to be applied, got %q", files[0].Content)
	}
}

func TestMaterializeRefineBracketPathEdit(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	content := "version: v1alpha1\nsteps:\n  - id: first\n    kind: Command\n    spec:\n      command: [true]\n  - id: second\n    kind: Command\n    spec:\n      command: [true]\n  - id: third\n    kind: Command\n    spec:\n      command: [true]\n  - id: fourth\n    kind: Command\n    spec:\n      command: [true]\n"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	files, err := Materialize(root, askcontract.GenerationResponse{
		Documents: []askcontract.GeneratedDocument{{
			Path:   "workflows/scenarios/apply.yaml",
			Action: "edit",
			Edits:  []askcontract.StructuredEditAction{{Op: "set", RawPath: "steps[3].timeout", Value: "10m"}},
		}},
	})
	if err != nil {
		t.Fatalf("materialize bracket path edit: %v", err)
	}
	if !strings.Contains(files[0].Content, "id: fourth") || !strings.Contains(files[0].Content, "timeout: 10m") {
		t.Fatalf("expected bracket path edit to update fourth step, got %q", files[0].Content)
	}
}

func TestMaterializeRefineExtractVarTransform(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	varsPath := filepath.Join(root, "workflows", "vars.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	content := "version: v1alpha1\nsteps:\n  - id: init\n    kind: InitKubeadm\n    spec:\n      podNetworkCIDR: 10.244.0.0/16\n"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.WriteFile(varsPath, []byte("clusterName: demo\n"), 0o600); err != nil {
		t.Fatalf("write vars: %v", err)
	}
	files, err := Materialize(root, askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{
		Path:   "workflows/scenarios/apply.yaml",
		Action: "edit",
		Transforms: []askcontract.RefineTransformAction{{
			Type:     "extract-var",
			RawPath:  "steps[0].spec.podNetworkCIDR",
			VarName:  "podCIDR",
			VarsPath: "workflows/vars.yaml",
			Value:    "10.244.0.0/16",
		}},
	}}})
	if err != nil {
		t.Fatalf("materialize refine transform: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected scenario and vars files, got %#v", files)
	}
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}
	if !strings.Contains(byPath["workflows/scenarios/apply.yaml"], "{{ .vars.podCIDR }}") {
		t.Fatalf("expected scenario to reference extracted var, got %q", byPath["workflows/scenarios/apply.yaml"])
	}
	if !strings.Contains(byPath["workflows/vars.yaml"], "podCIDR: 10.244.0.0/16") {
		t.Fatalf("expected vars file to include extracted value, got %q", byPath["workflows/vars.yaml"])
	}
}

func TestMaterializeRefineExtractVarTransformsAccumulateSharedVarsFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	varsPath := filepath.Join(root, "workflows", "vars.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	content := "version: v1alpha1\nsteps:\n  - id: init\n    kind: InitKubeadm\n    spec:\n      podNetworkCIDR: 10.244.0.0/16\n      kubernetesVersion: v1.30.1\n"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.WriteFile(varsPath, []byte("clusterName: demo\n"), 0o600); err != nil {
		t.Fatalf("write vars: %v", err)
	}
	files, err := Materialize(root, askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{
		Path:   "workflows/scenarios/apply.yaml",
		Action: "edit",
		Transforms: []askcontract.RefineTransformAction{{
			Type:     "extract-var",
			RawPath:  "steps[0].spec.podNetworkCIDR",
			VarName:  "podCIDR",
			VarsPath: "workflows/vars.yaml",
			Value:    "10.244.0.0/16",
		}, {
			Type:     "extract-var",
			RawPath:  "steps[0].spec.kubernetesVersion",
			VarName:  "kubernetesVersion",
			VarsPath: "workflows/vars.yaml",
			Value:    "v1.30.1",
		}},
	}}})
	if err != nil {
		t.Fatalf("materialize repeated vars transforms: %v", err)
	}
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}
	varsContent := byPath["workflows/vars.yaml"]
	for _, want := range []string{"podCIDR: 10.244.0.0/16", "kubernetesVersion: v1.30.1", "clusterName: demo"} {
		if !strings.Contains(varsContent, want) {
			t.Fatalf("expected %q in vars file, got %q", want, varsContent)
		}
	}
}

func TestMaterializeRefineSetFieldTransform(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	content := "version: v1alpha1\nsteps:\n  - id: init\n    kind: InitKubeadm\n    spec:\n      podNetworkCIDR: 10.244.0.0/16\n"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	files, err := Materialize(root, askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{
		Path:   "workflows/scenarios/apply.yaml",
		Action: "edit",
		Transforms: []askcontract.RefineTransformAction{{
			Type:    "set-field",
			RawPath: "steps[0].spec.podNetworkCIDR",
			Value:   "10.250.0.0/16",
		}},
	}}})
	if err != nil {
		t.Fatalf("materialize set-field transform: %v", err)
	}
	if len(files) != 1 || !strings.Contains(files[0].Content, "10.250.0.0/16") {
		t.Fatalf("expected set-field transform to update scenario, got %#v", files)
	}
}

func TestMaterializeRefineCandidateSetFieldTransform(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	content := "version: v1alpha1\nsteps:\n  - id: init\n    kind: InitKubeadm\n    spec:\n      podNetworkCIDR: 10.244.0.0/16\n"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	files, err := Materialize(root, askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{
		Path:   "workflows/scenarios/apply.yaml",
		Action: "edit",
		Transforms: []askcontract.RefineTransformAction{{
			Type:      "set-field",
			Candidate: "set-field|workflows/scenarios/apply.yaml|steps[0].spec.podNetworkCIDR",
			Value:     "10.250.0.0/16",
		}},
	}}})
	if err != nil {
		t.Fatalf("materialize candidate set-field transform: %v", err)
	}
	if len(files) != 1 || !strings.Contains(files[0].Content, "10.250.0.0/16") {
		t.Fatalf("expected candidate-driven set-field transform to update scenario, got %#v", files)
	}
}

func TestMaterializeRefineCandidateExtractVarTransform(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	varsPath := filepath.Join(root, "workflows", "vars.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	content := "version: v1alpha1\nsteps:\n  - id: init\n    kind: InitKubeadm\n    spec:\n      kubernetesVersion: 1.35.1\n"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.WriteFile(varsPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write vars: %v", err)
	}
	files, err := Materialize(root, askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{
		Path:   "workflows/scenarios/apply.yaml",
		Action: "edit",
		Transforms: []askcontract.RefineTransformAction{{
			Type:      "extract-var",
			Candidate: "extract-var|workflows/scenarios/apply.yaml|steps[0].spec.kubernetesVersion",
			VarName:   "kubernetesVersion",
			VarsPath:  "workflows/vars.yaml",
		}},
	}, {
		Path:   "workflows/vars.yaml",
		Action: "edit",
		Transforms: []askcontract.RefineTransformAction{{
			Type:    "set-field",
			RawPath: "kubernetesVersion",
			Value:   "1.35.1",
		}},
	}}})
	if err != nil {
		t.Fatalf("materialize candidate extract-var transform: %v", err)
	}
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}
	if !strings.Contains(byPath["workflows/scenarios/apply.yaml"], "{{ .vars.kubernetesVersion }}") {
		t.Fatalf("expected scenario to reference extracted candidate var, got %q", byPath["workflows/scenarios/apply.yaml"])
	}
	if !strings.Contains(byPath["workflows/vars.yaml"], "kubernetesVersion: 1.35.1") {
		t.Fatalf("expected vars file to be derived from candidate value, got %q", byPath["workflows/vars.yaml"])
	}
}

func TestMaterializeRemovesEmptyWorkflowVarsBlockAfterDelete(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	content := "version: v1alpha1\nvars:\n  kubernetesVersion: 1.35.1\nsteps:\n  - id: init\n    kind: InitKubeadm\n    spec:\n      kubernetesVersion: 1.35.1\n"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	files, err := Materialize(root, askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{
		Path:   "workflows/scenarios/apply.yaml",
		Action: "edit",
		Transforms: []askcontract.RefineTransformAction{{
			Type:    "delete-field",
			RawPath: "vars.kubernetesVersion",
		}},
	}}})
	if err != nil {
		t.Fatalf("materialize delete last vars key: %v", err)
	}
	if len(files) != 1 || strings.Contains(files[0].Content, "vars: {}") || strings.Contains(files[0].Content, "vars:\n") {
		t.Fatalf("expected empty vars block to be removed, got %#v", files)
	}
}

func TestApplyStructuredEditsSetsTemplateStringForKubernetesVersion(t *testing.T) {
	raw := []byte("version: v1alpha1\nsteps:\n  - id: init\n    kind: InitKubeadm\n    spec:\n      kubernetesVersion: 1.35.1\n")
	updated, err := applyStructuredEdits(raw, []stepspec.StructuredEdit{{Op: "set", RawPath: "/steps/0/spec/kubernetesVersion", Value: "{{ .vars.kubernetesVersion }}"}})
	if err != nil {
		t.Fatalf("apply structured edits: %v", err)
	}
	if !strings.Contains(string(updated), "{{ .vars.kubernetesVersion }}") {
		t.Fatalf("expected template string in updated yaml, got %q", string(updated))
	}
}

func TestMaterializeRefineVarsEditCreatesMissingVarsFile(t *testing.T) {
	root := t.TempDir()
	files, err := Materialize(root, askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{
		Path:   "workflows/vars.yaml",
		Action: "edit",
		Transforms: []askcontract.RefineTransformAction{{
			Type:  "set-field",
			Path:  "vars.joinFilePath",
			Value: "/tmp/deck/join.txt",
		}},
	}}})
	if err != nil {
		t.Fatalf("materialize vars edit: %v", err)
	}
	if len(files) != 1 || !strings.Contains(files[0].Content, "joinFilePath: /tmp/deck/join.txt") {
		t.Fatalf("expected missing vars file to be created via edit transform, got %#v", files)
	}
}

func TestMaterializeRefineVarsEditNormalizesWrappedVarsPath(t *testing.T) {
	root := t.TempDir()
	files, err := Materialize(root, askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{
		Path:   "workflows/vars.yaml",
		Action: "edit",
		Transforms: []askcontract.RefineTransformAction{{
			Type:  "set-field",
			Path:  "vars.joinFilePath",
			Value: "/tmp/deck/join.txt",
		}},
	}}})
	if err != nil {
		t.Fatalf("materialize wrapped vars path edit: %v", err)
	}
	if len(files) != 1 || strings.Contains(files[0].Content, "vars:") || !strings.Contains(files[0].Content, "joinFilePath: /tmp/deck/join.txt") {
		t.Fatalf("expected vars edit to normalize wrapper path, got %#v", files)
	}
}

func TestMaterializeRefineExtractVarQuotesCanonicalTemplate(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	content := "version: v1alpha1\nsteps:\n  - id: init\n    kind: InitKubeadm\n    spec:\n      kubernetesVersion: v1.35.1\n"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	files, err := Materialize(root, askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{
		Path:   "workflows/scenarios/apply.yaml",
		Action: "edit",
		Transforms: []askcontract.RefineTransformAction{{
			Type:     "extract-var",
			RawPath:  "steps[0].spec.kubernetesVersion",
			VarName:  "kubernetesVersion",
			VarsPath: "workflows/vars.yaml",
			Value:    "v1.35.1",
		}},
	}}})
	if err != nil {
		t.Fatalf("materialize extract-var quoting: %v", err)
	}
	if len(files) != 2 || (!strings.Contains(files[0].Content, `kubernetesVersion: "{{ .vars.kubernetesVersion }}"`) && !strings.Contains(files[0].Content, `kubernetesVersion: '{{ .vars.kubernetesVersion }}'`)) {
		t.Fatalf("expected canonical quoted vars template, got %#v", files)
	}
}

func TestMaterializeSkipsUnsupportedUnknownExtractVarCandidate(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	content := "version: v1alpha1\nphases:\n  - name: verify\n    steps:\n      - id: apply-check-cluster\n        kind: CheckCluster\n        spec:\n          interval: 5s\n          nodes:\n            total: 1\n            ready: 1\n            controlPlaneReady: 1\n          timeout: 10m\n"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	files, err := Materialize(root, askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{
		Path:   "workflows/scenarios/apply.yaml",
		Action: "edit",
		Transforms: []askcontract.RefineTransformAction{{
			Candidate: "extract-var|workflows/scenarios/apply.yaml|phases[0].steps[0].spec.interval",
		}},
	}}})
	if err != nil {
		t.Fatalf("materialize with unsupported extract-var candidate: %v", err)
	}
	if len(files) != 1 || !strings.Contains(files[0].Content, "interval: 5s") {
		t.Fatalf("expected original workflow content to remain after skipping unsupported candidate, got %#v", files)
	}
}

func TestMaterializePrunesUnusedVarsCompanionWrites(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	content := "version: v1alpha1\nvars:\n  kubernetesVersion: 1.35.1\nphases:\n  - name: bootstrap\n    steps:\n      - id: init\n        kind: InitKubeadm\n        spec:\n          kubernetesVersion: 1.35.1\n"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	files, err := Materialize(root, askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{
		{
			Path:   "workflows/scenarios/apply.yaml",
			Action: "edit",
			Transforms: []askcontract.RefineTransformAction{{
				Type:     "extract-var",
				RawPath:  "phases[0].steps[0].spec.kubernetesVersion",
				VarName:  "kubernetesVersion",
				VarsPath: "workflows/vars.yaml",
				Value:    "1.35.1",
			}},
		},
		{
			Path:   "workflows/vars.yaml",
			Action: "edit",
			Transforms: []askcontract.RefineTransformAction{
				{Type: "set-field", RawPath: "kubernetesVersion", Value: "1.35.1"},
				{Type: "set-field", RawPath: "joinFile", Value: "/tmp/deck/join.txt"},
			},
		},
	}})
	if err != nil {
		t.Fatalf("materialize with vars pruning: %v", err)
	}
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}
	if strings.Contains(byPath["workflows/vars.yaml"], "joinFile:") || !strings.Contains(byPath["workflows/vars.yaml"], "kubernetesVersion: 1.35.1") {
		t.Fatalf("expected unused vars companion writes to be pruned, got %#v", byPath)
	}
}

func TestMaterializeRefineDeleteFieldTransform(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	content := "version: v1alpha1\nsteps:\n  - id: init\n    kind: InitKubeadm\n    spec:\n      podNetworkCIDR: 10.244.0.0/16\n      ignorePreflightErrors: [Swap]\n"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	files, err := Materialize(root, askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{
		Path:   "workflows/scenarios/apply.yaml",
		Action: "edit",
		Transforms: []askcontract.RefineTransformAction{{
			Type:    "delete-field",
			RawPath: "steps[0].spec.ignorePreflightErrors",
		}},
	}}})
	if err != nil {
		t.Fatalf("materialize delete-field transform: %v", err)
	}
	if len(files) != 1 || strings.Contains(files[0].Content, "ignorePreflightErrors") {
		t.Fatalf("expected delete-field transform to remove field, got %#v", files)
	}
}

func TestMaterializeRefineExtractComponentTransform(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	content := "version: v1alpha1\nphases:\n  - name: bootstrap\n    steps:\n      - id: init\n        kind: InitKubeadm\n        spec:\n          podNetworkCIDR: 10.244.0.0/16\n"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	files, err := Materialize(root, askcontract.GenerationResponse{Documents: []askcontract.GeneratedDocument{{
		Path:   "workflows/scenarios/apply.yaml",
		Action: "edit",
		Transforms: []askcontract.RefineTransformAction{{
			Type:    "extract-component",
			RawPath: "phases.bootstrap",
			Path:    "workflows/components/bootstrap.yaml",
		}},
	}}})
	if err != nil {
		t.Fatalf("materialize extract-component transform: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected scenario and component files, got %#v", files)
	}
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}
	if !strings.Contains(byPath["workflows/scenarios/apply.yaml"], "imports:") || !strings.Contains(byPath["workflows/scenarios/apply.yaml"], "path: bootstrap.yaml") {
		t.Fatalf("expected scenario to import extracted component, got %q", byPath["workflows/scenarios/apply.yaml"])
	}
	if strings.Contains(byPath["workflows/scenarios/apply.yaml"], "kind: InitKubeadm") {
		t.Fatalf("expected inline steps removed from scenario, got %q", byPath["workflows/scenarios/apply.yaml"])
	}
	if !strings.Contains(byPath["workflows/components/bootstrap.yaml"], "kind: InitKubeadm") {
		t.Fatalf("expected component to contain extracted steps, got %q", byPath["workflows/components/bootstrap.yaml"])
	}
}

func TestMaterializeDeleteDocument(t *testing.T) {
	files, err := Materialize(t.TempDir(), askcontract.GenerationResponse{
		Documents: []askcontract.GeneratedDocument{{Path: "workflows/components/old.yaml", Action: "delete"}},
	})
	if err != nil {
		t.Fatalf("materialize delete: %v", err)
	}
	if len(files) != 1 || !files[0].Delete {
		t.Fatalf("expected delete file, got %#v", files)
	}
}

func TestMaterializeCompilesBuilderSelection(t *testing.T) {
	files, err := Materialize(t.TempDir(), askcontract.GenerationResponse{Program: &askcontract.AuthoringProgram{Cluster: askcontract.ProgramCluster{JoinFile: "/tmp/deck/join.txt", ControlPlaneCount: 1}, Verification: askcontract.ProgramVerification{ExpectedNodeCount: 1, ExpectedReadyCount: 1, ExpectedControlPlaneReady: 1}}, Selection: &askcontract.DraftSelection{Targets: []askcontract.DraftTargetSelection{{Path: "workflows/scenarios/apply.yaml", Kind: "workflow", Builders: []askcontract.DraftBuilderSelection{{ID: "apply.init-kubeadm", Overrides: map[string]any{}}, {ID: "apply.check-cluster", Overrides: map[string]any{}}}}}}})
	if err != nil {
		t.Fatalf("materialize builder selection: %v", err)
	}
	if len(files) != 1 || !strings.Contains(files[0].Content, "kind: InitKubeadm") || !strings.Contains(files[0].Content, "kind: CheckCluster") {
		t.Fatalf("expected builder selection to render typed workflow content, got %#v", files)
	}
}

func TestMaterializeWithBasePreservesUntouchedFiles(t *testing.T) {
	base := []askcontract.GeneratedFile{{Path: "workflows/vars.yaml", Content: "role: control-plane\n"}}
	files, err := MaterializeWithBase(t.TempDir(), base, askcontract.GenerationResponse{
		Documents: []askcontract.GeneratedDocument{{
			Path: "workflows/scenarios/apply.yaml",
			Kind: "workflow",
			Workflow: &askcontract.WorkflowDocument{
				Version: "v1alpha1",
				Steps:   []askcontract.WorkflowStep{{ID: "run", Kind: "Command", Spec: map[string]any{"command": []any{"true"}}}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("materialize with base: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected untouched base file plus new file, got %#v", files)
	}
	if files[0].Path != "workflows/vars.yaml" || files[0].Content != base[0].Content {
		t.Fatalf("expected untouched base content to be preserved, got %#v", files)
	}
}

func TestParseDocumentWorkflow(t *testing.T) {
	doc, err := ParseDocument("workflows/scenarios/apply.yaml", []byte("version: v1alpha1\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [true]\n"))
	if err != nil {
		t.Fatalf("parse workflow document: %v", err)
	}
	if doc.Workflow == nil || doc.Workflow.Version != "v1alpha1" || len(doc.Workflow.Steps) != 1 {
		t.Fatalf("unexpected parsed workflow: %#v", doc)
	}
}

func TestParseDocumentVars(t *testing.T) {
	doc, err := ParseDocument("workflows/vars.yaml", []byte("role: control-plane\nversion: v1.30.1\n"))
	if err != nil {
		t.Fatalf("parse vars document: %v", err)
	}
	if doc.Vars == nil || doc.Vars["role"] != "control-plane" {
		t.Fatalf("unexpected vars document: %#v", doc)
	}
}

func TestResolveStructuredEditPathFindsPhaseStepByID(t *testing.T) {
	doc := askcontract.GeneratedDocument{Path: "workflows/scenarios/apply.yaml", Kind: "workflow", Workflow: &askcontract.WorkflowDocument{Version: "v1alpha1", Phases: []askcontract.WorkflowPhase{{Name: "apply", Steps: []askcontract.WorkflowStep{{ID: "apply-runtime-ready", Kind: "WaitForService", Spec: map[string]any{"timeout": "5m"}}}}}}}
	path := resolveStructuredEditPath("steps.apply-runtime-ready.spec.timeout", doc)
	if path != "/phases/0/steps/0/spec/timeout" {
		t.Fatalf("unexpected resolved path: %q", path)
	}
}
