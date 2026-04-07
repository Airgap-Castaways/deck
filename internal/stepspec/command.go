package stepspec

// Run a narrowly scoped host command when no typed step models the action.
// @deck.when Use this only as an escape hatch for vendor tools, custom probes, or one-off local commands that do not map cleanly to an existing typed step.
// @deck.note Do not use `Command` for service management, directory creation, file copy, sysctl changes, swap control, kernel modules, archive extraction, or symlink creation when the typed steps already cover those actions.
// @deck.note Prefer a direct command vector over `sh -c` unless shell syntax is truly required.
// @deck.note Keep command steps small, explicit, and bounded with `spec.timeout`.
// @deck.example
// kind: Command
// spec:
//
//	command: [/opt/vendor/bin/node-health, --quick]
//	timeout: 30s
type Command struct {
	// Command vector to execute. The first element is the binary and the rest are arguments.
	// @deck.example [systemctl,restart,containerd]
	Command []string `json:"command"`
	// Additional environment variables passed to the command process as key-value pairs.
	// @deck.example {KUBECONFIG:/etc/kubernetes/admin.conf}
	Env map[string]string `json:"env"`
	// Prepend `sudo` before the command vector. Defaults to `false`.
	// @deck.example false
	Sudo bool `json:"sudo"`
	// Maximum duration for the command before it is killed. Overrides the step-level `timeout`.
	// @deck.example 30s
	Timeout string `json:"timeout"`
}

type CheckHost struct {
	// Named checks to run against the local host.
	// @deck.example [os,arch,swap]
	Checks []string `json:"checks"`
	// Binary names to verify are present in `PATH`.
	// @deck.example [kubeadm,kubelet,kubectl]
	Binaries []string `json:"binaries"`
	// Stop on the first failing check rather than running all checks.
	// @deck.example true
	FailFast *bool `json:"failFast"`
}
