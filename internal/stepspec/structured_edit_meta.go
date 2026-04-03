package stepspec

import "github.com/Airgap-Castaways/deck/internal/stepmeta"

var _ = stepmeta.MustRegister[EditTOML](stepmeta.Definition{Kind: "EditTOML", Family: "file", FamilyTitle: "File", Group: "filesystem-content", GroupOrder: 60, DocsPage: "file", DocsOrder: 50, Visibility: "public", Roles: []string{"apply"}, Outputs: []string{"path"}, SchemaFile: "file.edit-toml.schema.json", SchemaPatch: stepmeta.PatchEditTOMLToolSchema})

var _ = stepmeta.MustRegister[EditYAML](stepmeta.Definition{Kind: "EditYAML", Family: "file", FamilyTitle: "File", Group: "filesystem-content", GroupOrder: 70, DocsPage: "file", DocsOrder: 60, Visibility: "public", Roles: []string{"apply"}, Outputs: []string{"path"}, SchemaFile: "file.edit-yaml.schema.json", SchemaPatch: stepmeta.PatchEditYAMLToolSchema, Ask: stepmeta.AskMetadata{MatchSignals: []string{"yaml", "edit", "patch", "config"}}})

var _ = stepmeta.MustRegister[EditJSON](stepmeta.Definition{Kind: "EditJSON", Family: "file", FamilyTitle: "File", Group: "filesystem-content", GroupOrder: 80, DocsPage: "file", DocsOrder: 70, Visibility: "public", Roles: []string{"apply"}, Outputs: []string{"path"}, SchemaFile: "file.edit-json.schema.json", SchemaPatch: stepmeta.PatchEditJSONToolSchema})
