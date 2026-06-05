package stepspec

import "github.com/Airgap-Castaways/deck/internal/stepmeta"

var _ = stepmeta.MustRegister[WriteContainerdConfig](stepmeta.Definition{
	Kind:        "WriteContainerdConfig",
	Family:      "containerd",
	FamilyTitle: "Containerd",
	Group:       "container-runtime",
	GroupOrder:  10,
	DocsPage:    "containerd",
	DocsOrder:   10,
	Visibility:  "public",
	Roles:       []string{"apply"},
	Outputs:     []string{"path"},
	SchemaFile:  "containerd.config.schema.json",
	SchemaPatch: stepmeta.PatchWriteContainerdConfigToolSchema,
	Parallel:    parallelTargetPaths("spec.path"),
	Ask: stepmeta.AskMetadata{
		Capabilities:    []string{"container-runtime"},
		MatchSignals:    []string{"containerd", "config.toml", "SystemdCgroup", "registry config"},
		KeyFields:       []string{"spec.path", "spec.versionPolicy", "spec.rawSettings", "spec.rawSettings[].op", "spec.rawSettings[].key", "spec.rawSettings[].rawPath", "spec.rawSettings[].value"},
		ValidationHints: []stepmeta.ValidationHint{{ErrorContains: "spec.rawSettings", Fix: "For WriteContainerdConfig, rawSettings entries using set, appendUnique, or replaceList require value; delete does not."}},
	},
})

var _ = stepmeta.MustRegister[WriteContainerdRegistryHosts](stepmeta.Definition{Kind: "WriteContainerdRegistryHosts", Family: "containerd", FamilyTitle: "Containerd", Group: "container-runtime", GroupOrder: 20, DocsPage: "containerd", DocsOrder: 20, Visibility: "public", Roles: []string{"apply"}, Outputs: []string{"path"}, SchemaFile: "containerd.registry-hosts.schema.json", SchemaPatch: stepmeta.PatchWriteContainerdRegistryHostsToolSchema, Parallel: parallelTargetPaths("spec.path")})
