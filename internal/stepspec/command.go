package stepspec

type Command struct {
	Command []string          `json:"command"`
	Env     map[string]string `json:"env"`
	Sudo    bool              `json:"sudo"`
	Timeout string            `json:"timeout"`
}

type CheckHost struct {
	Checks   []string `json:"checks"`
	Binaries []string `json:"binaries"`
	FailFast *bool    `json:"failFast"`
}
