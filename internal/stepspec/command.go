package stepspec

// Run an explicit command as an escape hatch.
// @deck.when Use this only when no typed step expresses the change clearly enough.
// @deck.note Prefer a typed step kind over `Command` whenever one is available.
// @deck.note Use `spec.timeout` to bound commands that may hang instead of relying only on the outer step timeout.
// @deck.example
// kind: Command
// spec:
//
//	command: [systemctl, status, containerd]
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
