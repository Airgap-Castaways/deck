package askretrieve

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcatalog"
	_ "github.com/Airgap-Castaways/deck/internal/stepspec"
)

func TestBuildStepspecSummaryUsesCanonicalCatalogFacts(t *testing.T) {
	builderID, kind := currentGroundingClusterCheckFact()
	summary := buildStepspecSummary(t.TempDir(), strings.ToLower("Explain HelperKind InlineKind apply.init-kubeadm "+builderID))
	for _, want := range []string{
		"- observed typed step kinds:",
		"- observed ask builders:",
		"- step fact: " + kind + " builders=" + builderID,
		"- step fact: InitKubeadm builders=apply.init-kubeadm",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in summary, got %q", want, summary)
		}
	}
	if strings.Contains(summary, "candidate step kind") {
		t.Fatalf("expected fact-only wording, got %q", summary)
	}
}

func currentGroundingClusterCheckFact() (string, string) {
	for _, step := range askcatalog.Current().StepKinds() {
		if !strings.Contains(step.Kind, "Check") || !strings.Contains(step.Kind, "Cluster") {
			continue
		}
		for _, builder := range step.Builders {
			if strings.Contains(builder.ID, "check") && strings.Contains(builder.ID, "cluster") {
				return builder.ID, step.Kind
			}
		}
	}
	return "apply.check-kubernetes-cluster", "CheckKubernetesCluster"
}

func TestBuildStepmetaSummaryUsesCanonicalBuilderAndValidationCounts(t *testing.T) {
	summary := buildStepmetaSummary(t.TempDir())
	for _, want := range []string{
		"- registered builder metadata blocks observed:",
		"- validation hints observed:",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in stepmeta summary, got %q", want, summary)
		}
	}
}

func TestBuildStepspecSummaryFallsBackToObservedFactsWithoutRankingLanguage(t *testing.T) {
	summary := buildStepspecSummary(t.TempDir(), strings.ToLower("Explain repository metadata"))
	if !strings.Contains(summary, "- observed step fact:") {
		t.Fatalf("expected observed fact fallback, got %q", summary)
	}
	if strings.Contains(summary, "candidate step kind") {
		t.Fatalf("expected ranking language to be removed, got %q", summary)
	}
}
