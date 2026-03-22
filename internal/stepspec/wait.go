package stepspec

type Wait struct {
	Interval     string   `json:"interval"`
	PollInterval string   `json:"pollInterval"`
	InitialDelay string   `json:"initialDelay"`
	Path         string   `json:"path"`
	Paths        []string `json:"paths"`
	Glob         string   `json:"glob"`
	Type         string   `json:"type"`
	NonEmpty     bool     `json:"nonEmpty"`
	Name         string   `json:"name"`
	Command      []string `json:"command"`
	Address      string   `json:"address"`
	Port         string   `json:"port"`
	Timeout      string   `json:"timeout"`
}
