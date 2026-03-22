package stepspec

type ImageAuthBasic struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ImageAuth struct {
	Registry string         `json:"registry"`
	Basic    ImageAuthBasic `json:"basic"`
}

type ImageBackend struct {
	Engine string `json:"engine"`
}

type DownloadImage struct {
	Images    []string     `json:"images"`
	Auth      []ImageAuth  `json:"auth"`
	Backend   ImageBackend `json:"backend"`
	OutputDir string       `json:"outputDir"`
	Timeout   string       `json:"timeout"`
}

type LoadImage struct {
	Images    []string `json:"images"`
	SourceDir string   `json:"sourceDir"`
	Runtime   string   `json:"runtime"`
	Command   []string `json:"command"`
	Timeout   string   `json:"timeout"`
}

type VerifyImage struct {
	Images  []string `json:"images"`
	Command []string `json:"command"`
	Timeout string   `json:"timeout"`
}
