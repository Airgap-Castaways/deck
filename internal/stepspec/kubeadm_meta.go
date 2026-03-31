package stepspec

import "github.com/Airgap-Castaways/deck/internal/stepmeta"

var _ = stepmeta.MustRegister[KubeadmInit](stepmeta.Definition{
	Kind:        "InitKubeadm",
	Family:      "kubeadm",
	FamilyTitle: "Kubeadm",
	DocsPage:    "kubeadm",
	DocsOrder:   10,
	Visibility:  "public",
	Roles:       []string{"apply"},
	Outputs:     []string{"joinFile"},
	SchemaFile:  "kubeadm.init.schema.json",
	SchemaPatch: stepmeta.PatchInitKubeadmToolSchema,
	Ask: stepmeta.AskMetadata{
		Capabilities: []string{"kubeadm-bootstrap"},
		ContractHints: stepmeta.ContractHints{
			PublishesState: []string{"join-file"},
			RoleSensitive:  true,
		},
		MatchSignals: []string{"kubeadm", "bootstrap", "init", "control-plane", "cluster init"},
		KeyFields:    []string{"spec.outputJoinFile", "spec.configFile", "spec.kubernetesVersion", "spec.advertiseAddress", "spec.podNetworkCIDR"},
	},
})

var _ = stepmeta.MustRegister[KubeadmJoin](stepmeta.Definition{
	Kind:        "JoinKubeadm",
	Family:      "kubeadm",
	FamilyTitle: "Kubeadm",
	DocsPage:    "kubeadm",
	DocsOrder:   20,
	Visibility:  "public",
	Roles:       []string{"apply"},
	SchemaFile:  "kubeadm.join.schema.json",
	SchemaPatch: stepmeta.PatchJoinKubeadmToolSchema,
	Ask: stepmeta.AskMetadata{
		Capabilities: []string{"kubeadm-join"},
		ContractHints: stepmeta.ContractHints{
			ConsumesState: []string{"join-file"},
			RoleSensitive: true,
		},
		MatchSignals: []string{"kubeadm", "join", "worker", "add node"},
		KeyFields:    []string{"spec.joinFile", "spec.configFile", "spec.asControlPlane", "spec.extraArgs"},
	},
})

var _ = stepmeta.MustRegister[KubeadmReset](stepmeta.Definition{
	Kind:        "ResetKubeadm",
	Family:      "kubeadm",
	FamilyTitle: "Kubeadm",
	DocsPage:    "kubeadm",
	DocsOrder:   30,
	Visibility:  "public",
	Roles:       []string{"apply"},
	SchemaFile:  "kubeadm.reset.schema.json",
	SchemaPatch: stepmeta.PatchResetKubeadmToolSchema,
})

var _ = stepmeta.MustRegister[KubeadmUpgrade](stepmeta.Definition{
	Kind:        "UpgradeKubeadm",
	Family:      "kubeadm",
	FamilyTitle: "Kubeadm",
	DocsPage:    "kubeadm",
	DocsOrder:   40,
	Visibility:  "public",
	Roles:       []string{"apply"},
	SchemaFile:  "kubeadm.upgrade.schema.json",
	SchemaPatch: stepmeta.PatchUpgradeKubeadmToolSchema,
	Ask: stepmeta.AskMetadata{
		Capabilities: []string{"kubeadm-bootstrap"},
		ContractHints: stepmeta.ContractHints{
			RoleSensitive: true,
		},
		MatchSignals: []string{"kubeadm", "upgrade", "control-plane"},
		KeyFields:    []string{"spec.kubernetesVersion", "spec.ignorePreflightErrors", "spec.restartKubelet", "spec.kubeletService"},
	},
})
