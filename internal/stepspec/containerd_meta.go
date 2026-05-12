package stepspec

import "github.com/Airgap-Castaways/deck/internal/stepmeta"

var _ = stepmeta.MustRegister[WriteContainerdConfig](stepmeta.Definition{Kind: "WriteContainerdConfig", Family: "containerd", FamilyTitle: "Containerd", Group: "runtime-services", GroupOrder: 10, DocsPage: "containerd", DocsOrder: 10, Visibility: "public", Roles: []string{"apply"}, Outputs: []string{"path"}, SchemaFile: "containerd.config.schema.json", SchemaPatch: stepmeta.PatchWriteContainerdConfigToolSchema, Parallel: parallelTargetPaths("spec.path")})

var _ = stepmeta.MustRegister[WriteContainerdRegistryHosts](stepmeta.Definition{Kind: "WriteContainerdRegistryHosts", Family: "containerd", FamilyTitle: "Containerd", Group: "runtime-services", GroupOrder: 20, DocsPage: "containerd", DocsOrder: 20, Visibility: "public", Roles: []string{"apply"}, Outputs: []string{"path"}, SchemaFile: "containerd.registry-hosts.schema.json", SchemaPatch: stepmeta.PatchWriteContainerdRegistryHostsToolSchema, Parallel: parallelTargetPaths("spec.path")})
