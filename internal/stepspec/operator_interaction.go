package stepspec

// Print an operator-facing message during workflow execution.
// @deck.when Use this to explain a manual checkpoint, show rendered context, or make an apply/prepare workflow self-describing without shell commands.
// @deck.note `Message` is intended for human-readable operator output, not machine-readable workflow data.
// @deck.note Do not use `Message` to display secrets.
// @deck.example
// kind: Message
// spec:
//
//	level: info
//	message: |
//	  Node setup is starting.
//	  role: {{ .vars.role }}
type Message struct {
	// Message text to print. Templates are rendered before the step runs and YAML block scalars support multi-line text.
	// @deck.example Node setup is starting.
	Message string `json:"message"`
	// Message severity used as a human-readable prefix for warnings and errors. Defaults to `info`.
	// @deck.example info
	Level string `json:"level"`
	// Output stream for the message. Defaults to `stdout`.
	// @deck.example stdout
	Stream string `json:"stream"`
}

// Ask the local operator for a yes/no decision.
// @deck.when Use this as an explicit operator gate before a destructive or environment-specific action, or register the decision for later branching.
// @deck.note `Confirm` records no implicit global value. Use `register` when later steps need the decision.
// @deck.note In non-interactive mode, `Confirm` requires `spec.default`.
// @deck.example
// kind: Confirm
// register:
//
//	doReset: confirmed
//
// spec:
//
//	message: Reset existing cluster state?
//	default: false
//	onNo: continue
type Confirm struct {
	// Prompt shown to the local operator.
	// @deck.example Continue with kubeadm init?
	Message string `json:"message"`
	// Default answer used for empty input and required in non-interactive mode.
	// @deck.example false
	Default *bool `json:"default"`
	// Behavior when the operator answers no. Defaults to `fail`.
	// @deck.example continue
	OnNo string `json:"onNo"`
}

// Ask the local operator for a string value.
// @deck.when Use this for local operator-provided values that are only known at apply time and should feed later steps through `register`.
// @deck.note Non-secret input can use `spec.default` in non-interactive mode.
// @deck.note `spec.secret: true` disables terminal echo when possible, keeps the value out of persisted state, and re-prompts when an incomplete run must resume without the value.
// @deck.example
// kind: Input
// register:
//
//	nodeIP: value
//
// spec:
//
//	message: Node advertise address
//	required: true
type Input struct {
	// Prompt shown to the local operator.
	// @deck.example Node advertise address
	Message string `json:"message"`
	// Default value used for empty input and non-interactive mode.
	// @deck.example 192.0.2.10
	Default *string `json:"default"`
	// Require a non-empty value. Defaults to `true`.
	// @deck.example true
	Required *bool `json:"required"`
	// Treat the value as secret. Secret values require `register`, are available to later steps in memory, and are not persisted in state.
	// @deck.example false
	Secret bool `json:"secret"`
}
