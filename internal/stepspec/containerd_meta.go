package stepspec

import "github.com/Airgap-Castaways/deck/internal/stepmeta"

var _ = stepmeta.MustRegister[WriteContainerdConfig](stepmeta.Definition{Kind: "WriteContainerdConfig", Family: "containerd", FamilyTitle: "Containerd", DocsPage: "containerd", DocsOrder: 10, Visibility: "public", Roles: []string{"apply"}, Outputs: []string{"path"}, SchemaFile: "containerd.config.schema.json", SchemaPatch: stepmeta.PatchWriteContainerdConfigToolSchema})

var _ = stepmeta.MustRegister[WriteContainerdRegistryHosts](stepmeta.Definition{Kind: "WriteContainerdRegistryHosts", Family: "containerd", FamilyTitle: "Containerd", DocsPage: "containerd", DocsOrder: 20, Visibility: "public", Roles: []string{"apply"}, Outputs: []string{"path"}, SchemaFile: "containerd.registry-hosts.schema.json", SchemaPatch: stepmeta.PatchWriteContainerdRegistryHostsToolSchema})
