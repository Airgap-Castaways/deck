package stepspec

// Wait bridges convergence gaps between steps.
// @deck.note Keep waits specific so failures identify exactly which dependency did not become ready within the timeout.
// @deck.note Use `initialDelay` when a service emits a transient non-active state immediately after being started.
type Wait struct {
	// Duration between poll attempts.
	// @deck.example 2s
	Interval string `json:"interval"`
	// Deprecated alias for `interval`. Prefer `interval`.
	// @deck.example 2s
	PollInterval string `json:"pollInterval"`
	// Duration to wait before the first poll attempt.
	// @deck.example 1s
	InitialDelay string `json:"initialDelay"`
	// Filesystem path to check.
	// @deck.example /etc/kubernetes/admin.conf
	Path string `json:"path"`
	// List of paths that must all be absent before the step succeeds.
	// @deck.example [/etc/kubernetes/manifests/a.yaml,/etc/kubernetes/manifests/b.yaml]
	Paths []string `json:"paths"`
	// Glob pattern that must resolve to zero matches before the step succeeds.
	// @deck.example /etc/kubernetes/manifests/*.yaml
	Glob string `json:"glob"`
	// Filesystem entry type restriction for path checks.
	// @deck.example file
	Type string `json:"type"`
	// Require the matched file to have non-zero size.
	// @deck.example true
	NonEmpty bool `json:"nonEmpty"`
	// Service name to check.
	// @deck.example containerd
	Name string `json:"name"`
	// Command vector to run on each poll attempt.
	// @deck.example [test,-f,/etc/kubernetes/admin.conf]
	Command []string `json:"command"`
	// Host or IP address for TCP port checks.
	// @deck.example 127.0.0.1
	Address string `json:"address"`
	// TCP port number to check.
	// @deck.example 6443
	Port string `json:"port"`
	// Maximum total duration to wait before the step fails.
	// @deck.example 5m
	Timeout string `json:"timeout"`
}
