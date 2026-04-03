package askcontext

import "testing"

func TestBuildStepKindsProjectsContractHints(t *testing.T) {
	var downloadImage, installPackage, initKubeadm, joinKubeadm, clusterCheck StepKindContext
	for _, step := range Current().StepKinds {
		switch step.Kind {
		case "DownloadImage":
			downloadImage = step
		case "InstallPackage":
			installPackage = step
		case "InitKubeadm":
			initKubeadm = step
		case "JoinKubeadm":
			joinKubeadm = step
		case "CheckKubernetesCluster":
			clusterCheck = step
		}
	}
	if len(downloadImage.ProducesArtifacts) == 0 || downloadImage.ProducesArtifacts[0] != "image" {
		t.Fatalf("expected image production hint, got %#v", downloadImage)
	}
	if len(installPackage.ConsumesArtifacts) == 0 || installPackage.ConsumesArtifacts[0] != "package" {
		t.Fatalf("expected package consume hint, got %#v", installPackage)
	}
	if len(initKubeadm.PublishesState) == 0 || initKubeadm.PublishesState[0] != "join-file" {
		t.Fatalf("expected join publication hint, got %#v", initKubeadm)
	}
	if len(joinKubeadm.ConsumesState) == 0 || joinKubeadm.ConsumesState[0] != "join-file" {
		t.Fatalf("expected join consumption hint, got %#v", joinKubeadm)
	}
	if !clusterCheck.VerificationRelated {
		t.Fatalf("expected verification hint, got %#v", clusterCheck)
	}
}
