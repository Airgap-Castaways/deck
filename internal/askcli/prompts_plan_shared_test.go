package askcli

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func TestAuthoringProgramPromptBlockIncludesArtifactDownloadContract(t *testing.T) {
	prompt := authoringProgramPromptBlock(askcontract.AuthoringProgram{
		Platform: askcontract.ProgramPlatform{
			Family:       "debian",
			Release:      "jammy",
			RepoType:     "deb-flat",
			BackendImage: "ubuntu:22.04",
		},
		Artifacts: askcontract.ProgramArtifacts{
			Packages:         []string{"kubeadm"},
			PackageOutputDir: "packages/deb/jammy",
			Images:           []string{"registry.k8s.io/kube-apiserver:v1.31.0"},
			ImageOutputDir:   "images/kubernetes",
		},
	})

	for _, want := range []string{
		"- platform.family: debian",
		"- platform.release: jammy",
		"- platform.repoType: deb-flat",
		"- platform.backendImage: ubuntu:22.04",
		"- artifacts.packages: kubeadm",
		"- artifacts.packageOutputDir: packages/deb/jammy",
		"- artifacts.images: registry.k8s.io/kube-apiserver:v1.31.0",
		"- artifacts.imageOutputDir: images/kubernetes",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", want, prompt)
		}
	}
}
