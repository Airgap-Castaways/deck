package stepspec

type KubeadmInit struct {
	OutputJoinFile        string   `json:"outputJoinFile"`
	SkipIfAdminConfExists *bool    `json:"skipIfAdminConfExists"`
	CriSocket             string   `json:"criSocket"`
	KubernetesVersion     string   `json:"kubernetesVersion"`
	ConfigFile            string   `json:"configFile"`
	ConfigTemplate        string   `json:"configTemplate"`
	PodNetworkCIDR        string   `json:"podNetworkCIDR"`
	AdvertiseAddress      string   `json:"advertiseAddress"`
	IgnorePreflightErrors []string `json:"ignorePreflightErrors"`
	ExtraArgs             []string `json:"extraArgs"`
	Timeout               string   `json:"timeout"`
}

type KubeadmJoin struct {
	JoinFile       string   `json:"joinFile"`
	ConfigFile     string   `json:"configFile"`
	AsControlPlane bool     `json:"asControlPlane"`
	ExtraArgs      []string `json:"extraArgs"`
	Timeout        string   `json:"timeout"`
}

type KubeadmReset struct {
	Force                       bool     `json:"force"`
	IgnoreErrors                bool     `json:"ignoreErrors"`
	StopKubelet                 *bool    `json:"stopKubelet"`
	CriSocket                   string   `json:"criSocket"`
	ExtraArgs                   []string `json:"extraArgs"`
	RemovePaths                 []string `json:"removePaths"`
	RemoveFiles                 []string `json:"removeFiles"`
	CleanupContainers           []string `json:"cleanupContainers"`
	RestartRuntimeManageService string   `json:"restartRuntimeService"`
	WaitForRuntimeService       bool     `json:"waitForRuntimeService"`
	WaitForRuntimeReady         bool     `json:"waitForRuntimeReady"`
	WaitForMissingManifestsGlob string   `json:"waitForMissingManifestsGlob"`
	StopKubeletAfterReset       bool     `json:"stopKubeletAfterReset"`
	VerifyContainersAbsent      []string `json:"verifyContainersAbsent"`
	ReportFile                  string   `json:"reportFile"`
	ReportResetReason           string   `json:"reportResetReason"`
	Timeout                     string   `json:"timeout"`
}

type KubeadmUpgrade struct {
	KubernetesVersion     string   `json:"kubernetesVersion"`
	IgnorePreflightErrors []string `json:"ignorePreflightErrors"`
	ExtraArgs             []string `json:"extraArgs"`
	RestartKubelet        *bool    `json:"restartKubelet"`
	KubeletService        string   `json:"kubeletService"`
	Timeout               string   `json:"timeout"`
}
