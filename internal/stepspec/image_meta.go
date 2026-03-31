package stepspec

import "github.com/Airgap-Castaways/deck/internal/stepmeta"

var _ = stepmeta.MustRegister[DownloadImage](stepmeta.Definition{
	Kind:        "DownloadImage",
	Family:      "image",
	FamilyTitle: "Image",
	DocsPage:    "image",
	DocsOrder:   10,
	Visibility:  "public",
	Roles:       []string{"prepare"},
	Outputs:     []string{"artifacts"},
	SchemaFile:  "image.download.schema.json",
	SchemaPatch: stepmeta.PatchDownloadImageToolSchema,
	Ask: stepmeta.AskMetadata{
		Capabilities:             []string{"prepare-artifacts", "image-staging"},
		ContractHints:            stepmeta.ContractHints{ProducesArtifacts: []string{"image"}},
		MatchSignals:             []string{"air-gapped", "image", "images", "registry", "mirror", "offline", "prepare"},
		KeyFields:                []string{"spec.images", "spec.auth", "spec.backend", "spec.outputDir"},
		ValidationHints:          []stepmeta.ValidationHint{{ErrorContains: "spec.backend.engine must be one of", Fix: "Keep spec.backend.engine as the literal value `go-containerregistry`; do not replace it with a vars template."}, {ErrorContains: "is not supported for role prepare", Fix: "For prepare-time image collection, use DownloadImage instead of Command so the step matches the prepare role."}},
		ConstrainedLiteralFields: []stepmeta.ConstrainedLiteralField{{Path: "spec.backend.engine", AllowedValues: []string{"go-containerregistry"}, Guidance: "Keep spec.backend.engine as a literal enum, not a vars template."}},
	},
})

var _ = stepmeta.MustRegister[LoadImage](stepmeta.Definition{
	Kind:        "LoadImage",
	Family:      "image",
	FamilyTitle: "Image",
	DocsPage:    "image",
	DocsOrder:   20,
	Visibility:  "public",
	Roles:       []string{"apply"},
	SchemaFile:  "image.load.schema.json",
	SchemaPatch: stepmeta.PatchImageLoadToolSchema,
	Ask: stepmeta.AskMetadata{
		Capabilities:             []string{"image-staging", "apply-artifact-consumer", "kubeadm-bootstrap"},
		ContractHints:            stepmeta.ContractHints{ConsumesArtifacts: []string{"image"}},
		MatchSignals:             []string{"air-gapped", "image", "images", "archive", "containerd", "docker", "offline"},
		KeyFields:                []string{"spec.images", "spec.sourceDir", "spec.runtime", "spec.command"},
		ConstrainedLiteralFields: []stepmeta.ConstrainedLiteralField{{Path: "spec.runtime", AllowedValues: []string{"auto", "ctr", "docker", "podman"}, Guidance: "Keep spec.runtime as a literal enum, not a vars template."}},
	},
})

var _ = stepmeta.MustRegister[VerifyImage](stepmeta.Definition{
	Kind:        "VerifyImage",
	Family:      "image",
	FamilyTitle: "Image",
	DocsPage:    "image",
	DocsOrder:   30,
	Visibility:  "public",
	Roles:       []string{"apply"},
	SchemaFile:  "image.verify.schema.json",
	SchemaPatch: stepmeta.PatchVerifyImageToolSchema,
})
