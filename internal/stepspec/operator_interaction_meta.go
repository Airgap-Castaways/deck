package stepspec

import "github.com/Airgap-Castaways/deck/internal/stepmeta"

var _ = stepmeta.MustRegister[Message](stepmeta.Definition{
	Kind:        "Message",
	Family:      "operator-interaction",
	FamilyTitle: "OperatorInteraction",
	Category:    "control-flow",
	Group:       "operator-interaction",
	GroupOrder:  10,
	DocsPage:    "operator-interaction",
	DocsOrder:   10,
	Visibility:  "public",
	Roles:       []string{"apply", "prepare"},
	SchemaFile:  "message.schema.json",
	SchemaPatch: stepmeta.PatchMessageToolSchema,
	Summary:     "Print an operator-facing message during workflow execution.",
	WhenToUse:   "Use this to explain a manual checkpoint, show rendered context, or make an apply/prepare workflow self-describing without shell commands.",
	Example:     "kind: Message\nspec:\n  level: info\n  message: |\n    Node setup is starting.\n    role: {{ .vars.role }}",
	Notes: []string{
		"`Message` is intended for human-readable operator output, not machine-readable workflow data.",
		"Do not use `Message` to display secrets.",
	},
})

var _ = stepmeta.MustRegister[Confirm](stepmeta.Definition{
	Kind:        "Confirm",
	Family:      "operator-interaction",
	FamilyTitle: "OperatorInteraction",
	Category:    "control-flow",
	Group:       "operator-interaction",
	GroupOrder:  20,
	DocsPage:    "operator-interaction",
	DocsOrder:   20,
	Visibility:  "public",
	Roles:       []string{"apply"},
	Outputs:     []string{"confirmed"},
	SchemaFile:  "confirm.schema.json",
	SchemaPatch: stepmeta.PatchConfirmToolSchema,
	Summary:     "Ask the local operator for a yes/no decision.",
	WhenToUse:   "Use this as an explicit operator gate before a destructive or environment-specific action, or register the decision for later branching.",
	Example:     "kind: Confirm\nregister:\n  doReset: confirmed\nspec:\n  message: Reset existing cluster state?\n  default: false\n  onNo: continue",
	Notes: []string{
		"`Confirm` records no implicit global value. Use `register` when later steps need the decision.",
		"In non-interactive mode, `Confirm` requires `spec.default`.",
	},
})

var _ = stepmeta.MustRegister[Input](stepmeta.Definition{
	Kind:        "Input",
	Family:      "operator-interaction",
	FamilyTitle: "OperatorInteraction",
	Category:    "control-flow",
	Group:       "operator-interaction",
	GroupOrder:  30,
	DocsPage:    "operator-interaction",
	DocsOrder:   30,
	Visibility:  "public",
	Roles:       []string{"apply"},
	Outputs:     []string{"value"},
	SchemaFile:  "input.schema.json",
	SchemaPatch: stepmeta.PatchInputToolSchema,
	Summary:     "Ask the local operator for a string value.",
	WhenToUse:   "Use this for local operator-provided values that are only known at apply time and should feed later steps through `register`.",
	Example:     "kind: Input\nregister:\n  nodeIP: value\nspec:\n  message: Node advertise address\n  required: true",
	Notes: []string{
		"Non-secret input can use `spec.default` in non-interactive mode.",
		"`spec.secret: true` disables terminal echo when possible, keeps the value out of persisted state, and re-prompts when an incomplete run must resume without the value.",
	},
})
