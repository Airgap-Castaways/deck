package main

import (
	"testing"

	"github.com/Airgap-Castaways/deck/internal/schemadoc"
)

func TestGroupMetadataIncludesKubernetesLifecycle(t *testing.T) {
	meta := schemadoc.MustGroupMeta("kubernetes-lifecycle")
	if meta.Title != "Kubernetes Lifecycle" {
		t.Fatalf("unexpected group title: %q", meta.Title)
	}
	if len(meta.TypicalFlows) == 0 {
		t.Fatal("expected typical flows for kubernetes lifecycle")
	}
}
