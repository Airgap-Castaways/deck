package stepmeta_test

import (
	"testing"

	"github.com/Airgap-Castaways/deck/internal/stepmeta"
	_ "github.com/Airgap-Castaways/deck/internal/stepspec"
)

func TestProjectRuntimeOutputsForKindUsesArtifactsForDownloadFileAliases(t *testing.T) {
	outputs, err := stepmeta.ProjectRuntimeOutputsForKind("DownloadFile", map[string]any{"source": map[string]any{"path": "/tmp/payload.txt"}}, map[string]any{"artifacts": []string{"files/payload.txt"}}, stepmeta.RuntimeOutputOptions{})
	if err != nil {
		t.Fatalf("ProjectRuntimeOutputsForKind(DownloadFile): %v", err)
	}
	if got := outputs["outputPath"]; got != "files/payload.txt" {
		t.Fatalf("expected outputPath from artifacts, got %#v", got)
	}
	paths, ok := outputs["outputPaths"].([]string)
	if !ok || len(paths) != 1 || paths[0] != "files/payload.txt" {
		t.Fatalf("expected outputPaths from artifacts, got %#v", outputs["outputPaths"])
	}
}

func TestProjectRuntimeOutputsForKindUsesIndexFreeDotProjectionOnlyWhenSafe(t *testing.T) {
	outputs, err := stepmeta.ProjectRuntimeOutputsForKind("ManageService", map[string]any{"name": "containerd"}, nil, stepmeta.RuntimeOutputOptions{})
	if err != nil {
		t.Fatalf("ProjectRuntimeOutputsForKind(ManageService): %v", err)
	}
	if got := outputs["name"]; got != "containerd" {
		t.Fatalf("expected projected service name, got %#v", got)
	}
}

func TestProjectRuntimeOutputsForKindRespectsJoinFileExistence(t *testing.T) {
	outputs, err := stepmeta.ProjectRuntimeOutputsForKind("InitKubeadm", map[string]any{"outputJoinFile": "/tmp/join.txt"}, nil, stepmeta.RuntimeOutputOptions{FileExists: func(path string) bool {
		return path == "/tmp/join.txt"
	}})
	if err != nil {
		t.Fatalf("ProjectRuntimeOutputsForKind(InitKubeadm): %v", err)
	}
	if got := outputs["joinFile"]; got != "/tmp/join.txt" {
		t.Fatalf("expected joinFile output, got %#v", got)
	}
}

func TestProjectRuntimeOutputsForKindDoesNotInventArtifactsForOtherFamilies(t *testing.T) {
	outputs, err := stepmeta.ProjectRuntimeOutputsForKind("DownloadImage", map[string]any{"images": []any{"registry.k8s.io/pause:3.9"}}, nil, stepmeta.RuntimeOutputOptions{})
	if err != nil {
		t.Fatalf("ProjectRuntimeOutputsForKind(DownloadImage): %v", err)
	}
	if _, ok := outputs["artifacts"]; ok {
		t.Fatalf("did not expect synthetic artifacts output, got %#v", outputs["artifacts"])
	}
}
