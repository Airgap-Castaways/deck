package stepspec

// Poll and verify Kubernetes cluster health on the local node.
// @deck.when Use this for typed bootstrap and upgrade verification instead of ad-hoc kubectl shell loops.
// @deck.example
// kind: CheckKubernetesCluster
// spec:
//
//	interval: 5s
//	nodes:
//	  total: 1
//	  ready: 1
//	  controlPlaneReady: 1
//	reports:
//	  nodesPath: /tmp/deck/reports/bootstrap-nodes.txt
type ClusterCheck struct {
	// Kubeconfig path used for kubectl-based checks.
	// @deck.example /etc/kubernetes/admin.conf
	Kubeconfig string `json:"kubeconfig"`
	// Duration between poll attempts while waiting for cluster state to converge.
	// @deck.example 5s
	Interval string `json:"interval"`
	// Optional delay before the first poll attempt.
	// @deck.example 10s
	InitialDelay string `json:"initialDelay"`
	// Maximum total duration to keep polling before the step fails.
	// @deck.example 10m
	Timeout string `json:"timeout"`
	// Optional checks for cluster node count and readiness.
	// @deck.example {total:1,ready:1,controlPlaneReady:1}
	Nodes ClusterCheckNodes `json:"nodes"`
	// Optional checks for Kubernetes component versions.
	// @deck.example {server:v1.31.0,kubelet:v1.31.0}
	Versions ClusterCheckVersions `json:"versions"`
	// Optional checks for kube-system pod readiness.
	// @deck.example {readyNames:[etcd-control-plane]}
	KubeSystem ClusterCheckKubeSystem `json:"kubeSystem"`
	// Optional file-content assertions evaluated on every poll attempt.
	// @deck.example [{path:/etc/containerd/config.toml,contains:[registry.k8s.io/pause:3.10]}]
	FileChecks []ClusterCheckFileCheck `json:"fileAssertions"`
	// Optional paths for writing node and cluster state reports.
	// @deck.example {nodesPath:/tmp/deck/reports/bootstrap-nodes.txt}
	Reports ClusterCheckReports `json:"reports"`
}

type ClusterCheckNodes struct {
	// Expected total node count returned by `kubectl get nodes`.
	// @deck.example 1
	Total *int `json:"total"`
	// Expected count of Ready nodes.
	// @deck.example 1
	Ready *int `json:"ready"`
	// Expected count of Ready control-plane nodes.
	// @deck.example 1
	ControlPlaneReady *int `json:"controlPlaneReady"`
}

type ClusterCheckVersions struct {
	// Target Kubernetes version written into the optional version report file.
	// @deck.example v1.31.0
	TargetVersion string `json:"targetVersion"`
	// Expected API server version from `kubectl version -o json`.
	// @deck.example v1.31.0
	Server string `json:"server"`
	// Expected kubelet version for the selected node.
	// @deck.example v1.31.0
	Kubelet string `json:"kubelet"`
	// Expected local kubeadm version from `kubeadm version -o short`.
	// @deck.example v1.31.0
	Kubeadm string `json:"kubeadm"`
	// Node name used when reading kubelet version.
	// @deck.example control-plane
	NodeName string `json:"nodeName"`
	// Optional report file that records target, server, kubelet, and kubeadm versions.
	// @deck.example /tmp/deck/reports/upgrade-version.txt
	ReportPath string `json:"reportPath"`
}

type ClusterCheckKubeSystem struct {
	// Exact kube-system pod names that must be present and fully Ready.
	// @deck.example [etcd-control-plane,kube-apiserver-control-plane]
	ReadyNames []string `json:"readyNames"`
	// Pod-name prefixes for which at least one matching Ready pod must exist.
	// @deck.example [kube-proxy-]
	ReadyPrefixes []string `json:"readyPrefixes"`
	// Prefix-based readiness requirements with minimum Ready pod counts.
	// @deck.example [{prefix:coredns-,minReady:2}]
	ReadyPrefixMinimums []ClusterCheckReadyPrefixMinimum `json:"readyPrefixMinimums"`
	// Optional text report path for `kubectl get pods -n kube-system`.
	// @deck.example /tmp/deck/reports/kube-system-pods.txt
	ReportPath string `json:"reportPath"`
	// Optional JSON report path for `kubectl get pods -n kube-system -o json`.
	// @deck.example /tmp/deck/reports/kube-system-pods.json
	JSONReportPath string `json:"jsonReportPath"`
}

type ClusterCheckReadyPrefixMinimum struct {
	// Pod-name prefix to match inside `kube-system`.
	// @deck.example coredns-
	Prefix string `json:"prefix"`
	// Minimum number of matching Ready pods required for the prefix.
	// @deck.example 2
	MinReady int `json:"minReady"`
}

type ClusterCheckFileCheck struct {
	// Path of the local file whose content should be checked.
	// @deck.example /etc/containerd/config.toml
	Path string `json:"path"`
	// Strings that must all be present in the file content.
	// @deck.example [registry.k8s.io/pause:3.10]
	Contains []string `json:"contains"`
}

type ClusterCheckReports struct {
	// Optional report file path for `kubectl get nodes` output.
	// @deck.example /tmp/deck/reports/bootstrap-nodes.txt
	NodesPath string `json:"nodesPath"`
	// Optional second node report path for shared cluster node reports.
	// @deck.example /tmp/deck/reports/cluster-nodes.txt
	ClusterNodesPath string `json:"clusterNodesPath"`
}
