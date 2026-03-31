package stepspec

import "github.com/Airgap-Castaways/deck/internal/stepmeta"

var _ = stepmeta.MustRegister[ClusterCheck](stepmeta.Definition{
	Kind:        "CheckCluster",
	Family:      "cluster-check",
	FamilyTitle: "ClusterCheck",
	DocsPage:    "cluster-check",
	DocsOrder:   10,
	Visibility:  "public",
	Roles:       []string{"apply"},
	SchemaFile:  "cluster-check.schema.json",
	SchemaPatch: stepmeta.PatchCheckClusterToolSchema,
	Ask: stepmeta.AskMetadata{
		Capabilities:  []string{"cluster-verification", "kubeadm-bootstrap"},
		ContractHints: stepmeta.ContractHints{VerificationRelated: true, RoleSensitive: true},
		Builders: []stepmeta.AuthoringBuilder{{
			ID:                   "apply.check-cluster",
			Phase:                "verify",
			DefaultStepID:        "apply-check-cluster",
			Summary:              "Verify cluster readiness with typed node expectations.",
			RequiresCapabilities: []string{"cluster-verification"},
			Bindings: []stepmeta.AuthoringBinding{
				{Path: "spec.interval", From: "override:interval"},
				{Path: "spec.interval", From: "program:verification.interval"},
				{Path: "spec.interval", From: "derive:verification.interval"},
				{Path: "spec.timeout", From: "override:timeout"},
				{Path: "spec.timeout", From: "program:verification.timeout"},
				{Path: "spec.timeout", From: "derive:verification.timeout"},
				{Path: "spec.nodes.total", From: "override:nodeCount"},
				{Path: "spec.nodes.total", From: "program:verification.expectedNodeCount", Required: true},
				{Path: "spec.nodes.ready", From: "override:readyCount"},
				{Path: "spec.nodes.ready", From: "program:verification.expectedReadyCount"},
				{Path: "spec.nodes.ready", From: "derive:verification.expectedReadyCount", Required: true},
				{Path: "spec.nodes.controlPlaneReady", From: "override:controlPlaneReady"},
				{Path: "spec.nodes.controlPlaneReady", From: "program:verification.expectedControlPlaneReady"},
				{Path: "spec.nodes.controlPlaneReady", From: "derive:verification.expectedControlPlaneReady", Required: true},
				{Path: "when", From: "override:whenRole"},
				{Path: "when", From: "program:verification.roleWhen"},
				{Path: "when", From: "derive:verification.roleWhen"},
			},
		}},
		MatchSignals:             []string{"kubernetes", "kubeadm", "cluster", "verify", "health", "ready"},
		KeyFields:                []string{"spec.nodes", "spec.versions", "spec.kubeSystem", "spec.reports"},
		ValidationHints:          []stepmeta.ValidationHint{{ErrorContains: "spec.interval: does not match pattern", Fix: "Keep CheckCluster spec.interval as a literal duration such as 5s; do not replace it with a vars template."}},
		ConstrainedLiteralFields: []stepmeta.ConstrainedLiteralField{{Path: "spec.interval", Guidance: "Keep spec.interval as a literal duration such as 5s or 30s, not a vars template."}},
	},
})
