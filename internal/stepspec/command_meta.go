package stepspec

import "github.com/Airgap-Castaways/deck/internal/stepmeta"

var _ = stepmeta.MustRegister[Command](stepmeta.Definition{
	Kind:        "Command",
	Family:      "command",
	FamilyTitle: "Command",
	Group:       "advanced",
	GroupOrder:  10,
	DocsPage:    "command",
	DocsOrder:   10,
	Visibility:  "public",
	Roles:       []string{"apply"},
	SchemaFile:  "command.schema.json",
	SchemaPatch: stepmeta.PatchCommandToolSchema,
	Summary:     "Run a narrowly scoped host command when no typed step models the action.",
	WhenToUse:   "Use this only as an escape hatch for vendor tools, custom probes, or one-off local commands that do not map cleanly to an existing typed step.",
	Example:     "kind: Command\nspec:\n  command: [/opt/vendor/bin/node-health, --quick]\n  timeout: 30s",
	Notes: []string{
		"Do not use `Command` for service management, directory creation, file copy, sysctl changes, swap control, kernel modules, archive extraction, or symlink creation when the typed steps already cover those actions.",
		"Prefer a direct command vector over `sh -c` unless shell syntax is truly required.",
		"Keep command steps small, explicit, and bounded with `spec.timeout`.",
	},
	Ask: stepmeta.AskMetadata{
		Capabilities: []string{"escape-hatch"},
		MatchSignals: []string{"shell", "command", "script", "escape hatch"},
		KeyFields:    []string{"spec.command", "spec.env", "spec.sudo", "spec.timeout"},
		QualityRules: []stepmeta.QualityRule{{Trigger: "typed-preferred", Message: "Use Command only as an escape hatch. Prefer a typed step when one clearly matches the requested host action.", Level: "advisory"}},
		AntiSignals:  []string{"typed", "typed steps", "where possible"},
	},
})

var _ = stepmeta.MustRegister[CheckHost](stepmeta.Definition{
	Kind:        "CheckHost",
	Family:      "host-check",
	FamilyTitle: "HostCheck",
	Group:       "host-prep",
	GroupOrder:  10,
	DocsPage:    "host-check",
	DocsOrder:   10,
	Visibility:  "public",
	Roles:       []string{"apply", "prepare"},
	Outputs:     []string{"passed", "failedChecks"},
	SchemaFile:  "host-check.schema.json",
	SchemaPatch: stepmeta.PatchCheckHostToolSchema,
	Summary:     "Validate host suitability checks on the current node.",
	WhenToUse:   "Use this near the start of apply workflows, or optional prepare preflight, to fail early when host prerequisites are not met. Host facts remain available through runtime.host without register.",
	Example:     "kind: CheckHost\nspec:\n  checks: [os, arch, swap]\n  failFast: true",
	Notes:       []string{"Use `CheckHost` for host suitability validation instead of inventing custom preflight objects or shell-only probes when the built-in checks cover the requirement."},
	Ask: stepmeta.AskMetadata{
		Capabilities:    []string{"kubeadm-bootstrap", "host-preflight"},
		MatchSignals:    []string{"host", "preflight", "rhel", "rocky", "ubuntu", "air-gapped", "single-node"},
		KeyFields:       []string{"spec.checks", "spec.binaries", "spec.failFast"},
		ValidationHints: []stepmeta.ValidationHint{{ErrorContains: "checkhost", Fix: "For CheckHost, use spec.checks as a YAML string array like [os, arch, swap]."}, {ErrorContains: "checks is required", Fix: "CheckHost requires spec.checks. Example: spec: {checks: [os, arch, swap]}."}, {ErrorContains: "additional property os is not allowed", Fix: "Do not use spec.os for CheckHost; put named checks under spec.checks instead."}, {ErrorContains: "spec.checks.0: invalid type", Fix: "Each CheckHost spec.checks item must be a plain string such as os or arch, not an object."}},
	},
})
