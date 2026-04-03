package stepspec

import "github.com/Airgap-Castaways/deck/internal/stepmeta"

var _ = stepmeta.MustRegister[DownloadFile](stepmeta.Definition{
	Kind:        "DownloadFile",
	Family:      "file",
	FamilyTitle: "File",
	Group:       "artifact-staging",
	GroupOrder:  10,
	DocsPage:    "file",
	DocsOrder:   10,
	Visibility:  "public",
	Roles:       []string{"prepare"},
	Outputs:     []string{"outputPath", "outputPaths", "artifacts"},
	SchemaFile:  "file.download.schema.json",
	SchemaPatch: stepmeta.PatchDownloadFileToolSchema,
	Notes: []string{
		"`outputPath` must stay under the prepared `files/` root.",
		"Omit `outputPath` unless later steps need a stable custom location inside `files/`.",
	},
	Ask: stepmeta.AskMetadata{KeyFields: []string{"spec.source", "spec.fetch", "spec.mode"}},
})

var _ = stepmeta.MustRegister[WriteFile](stepmeta.Definition{
	Kind:        "WriteFile",
	Family:      "file",
	FamilyTitle: "File",
	Group:       "filesystem-content",
	GroupOrder:  20,
	DocsPage:    "file",
	DocsOrder:   20,
	Visibility:  "public",
	Roles:       []string{"apply"},
	Outputs:     []string{"path"},
	SchemaFile:  "file.write.schema.json",
	SchemaPatch: stepmeta.PatchWriteFileToolSchema,
	Ask:         stepmeta.AskMetadata{MatchSignals: []string{"write", "file", "config", "motd", "content"}, KeyFields: []string{"spec.path", "spec.content", "spec.template", "spec.mode"}},
})

var _ = stepmeta.MustRegister[CopyFile](stepmeta.Definition{
	Kind:        "CopyFile",
	Family:      "file",
	FamilyTitle: "File",
	Group:       "filesystem-content",
	GroupOrder:  30,
	DocsPage:    "file",
	DocsOrder:   30,
	Visibility:  "public",
	Roles:       []string{"apply"},
	Outputs:     []string{"path"},
	SchemaFile:  "file.copy.schema.json",
	SchemaPatch: stepmeta.PatchCopyFileToolSchema,
	Ask:         stepmeta.AskMetadata{KeyFields: []string{"spec.source", "spec.path", "spec.mode"}},
})

var _ = stepmeta.MustRegister[ExtractArchive](stepmeta.Definition{
	Kind:        "ExtractArchive",
	Family:      "file",
	FamilyTitle: "File",
	Group:       "filesystem-content",
	GroupOrder:  40,
	DocsPage:    "file",
	DocsOrder:   80,
	Visibility:  "public",
	Roles:       []string{"apply"},
	Outputs:     []string{"path"},
	SchemaFile:  "file.extract-archive.schema.json",
	SchemaPatch: stepmeta.PatchExtractArchiveToolSchema,
})

var _ = stepmeta.MustRegister[EditFile](stepmeta.Definition{
	Kind:        "EditFile",
	Family:      "file",
	FamilyTitle: "File",
	Group:       "filesystem-content",
	GroupOrder:  50,
	DocsPage:    "file",
	DocsOrder:   40,
	Visibility:  "public",
	Roles:       []string{"apply"},
	Outputs:     []string{"path"},
	SchemaFile:  "file.edit.schema.json",
	SchemaPatch: stepmeta.PatchEditFileToolSchema,
	Ask:         stepmeta.AskMetadata{KeyFields: []string{"spec.path", "spec.edits", "spec.backup", "spec.mode"}},
})
