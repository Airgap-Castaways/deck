package config

type Workflow struct {
	Version string         `yaml:"version"`
	Imports []string       `yaml:"imports"`
	Vars    map[string]any `yaml:"vars"`
	Context Context        `yaml:"context"`
	Phases  []Phase        `yaml:"phases"`
}

type Context struct {
	BundleRoot string `yaml:"bundleRoot"`
	StateFile  string `yaml:"stateFile"`
}

type Phase struct {
	Name  string `yaml:"name"`
	Steps []Step `yaml:"steps"`
}

type Step struct {
	ID         string            `yaml:"id"`
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   map[string]any    `yaml:"metadata"`
	When       string            `yaml:"when"`
	Register   map[string]string `yaml:"register"`
	Retry      int               `yaml:"retry"`
	Timeout    string            `yaml:"timeout"`
	Spec       map[string]any    `yaml:"spec"`
}
