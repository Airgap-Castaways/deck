package stepspec

type ClusterCheck struct {
	Kubeconfig   string                  `json:"kubeconfig"`
	Interval     string                  `json:"interval"`
	InitialDelay string                  `json:"initialDelay"`
	Timeout      string                  `json:"timeout"`
	Nodes        ClusterCheckNodes       `json:"nodes"`
	Versions     ClusterCheckVersions    `json:"versions"`
	KubeSystem   ClusterCheckKubeSystem  `json:"kubeSystem"`
	FileChecks   []ClusterCheckFileCheck `json:"fileAssertions"`
	Reports      ClusterCheckReports     `json:"reports"`
}

type ClusterCheckNodes struct {
	Total             *int `json:"total"`
	Ready             *int `json:"ready"`
	ControlPlaneReady *int `json:"controlPlaneReady"`
}

type ClusterCheckVersions struct {
	TargetVersion string `json:"targetVersion"`
	Server        string `json:"server"`
	Kubelet       string `json:"kubelet"`
	Kubeadm       string `json:"kubeadm"`
	NodeName      string `json:"nodeName"`
	ReportPath    string `json:"reportPath"`
}

type ClusterCheckKubeSystem struct {
	ReadyNames          []string                         `json:"readyNames"`
	ReadyPrefixes       []string                         `json:"readyPrefixes"`
	ReadyPrefixMinimums []ClusterCheckReadyPrefixMinimum `json:"readyPrefixMinimums"`
	ReportPath          string                           `json:"reportPath"`
	JSONReportPath      string                           `json:"jsonReportPath"`
}

type ClusterCheckReadyPrefixMinimum struct {
	Prefix   string `json:"prefix"`
	MinReady int    `json:"minReady"`
}

type ClusterCheckFileCheck struct {
	Path     string   `json:"path"`
	Contains []string `json:"contains"`
}

type ClusterCheckReports struct {
	NodesPath        string `json:"nodesPath"`
	ClusterNodesPath string `json:"clusterNodesPath"`
}
