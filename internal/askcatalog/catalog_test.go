package askcatalog

import "testing"

func TestCurrentProjectsSourceOfTruthForPackageBuilder(t *testing.T) {
	catalog := Current()
	if _, ok := catalog.LookupStep("DownloadPackage"); !ok {
		t.Fatalf("expected DownloadPackage step in catalog")
	}
	field, ok := catalog.LookupField("DownloadPackage", "spec.backend.mode")
	if !ok || len(field.Enum) != 1 || field.Enum[0] != "container" || !field.ConstrainedLiteral {
		t.Fatalf("expected constrained backend mode field from source-of-truth, got %#v", field)
	}
	builder, ok := catalog.LookupBuilder("prepare.download-package")
	if !ok || builder.StepKind != "DownloadPackage" || len(builder.Bindings) == 0 {
		t.Fatalf("expected projected package builder metadata, got %#v", builder)
	}
}

func TestAllowedGeneratedPathUsesWorkspaceAuthoringRules(t *testing.T) {
	if !AllowedGeneratedPath("workflows/scenarios/apply.yaml") {
		t.Fatalf("expected scenario path to be allowed")
	}
	if AllowedGeneratedPath("../secrets.txt") {
		t.Fatalf("expected traversal path to be rejected")
	}
}
