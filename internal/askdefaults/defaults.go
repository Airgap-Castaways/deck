package askdefaults

import (
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

const (
	ImageOutputDir                = "images/control-plane"
	JoinFile                      = "/tmp/deck/join.txt"
	PodCIDR                       = "10.244.0.0/16"
	CRISocket                     = "unix:///run/containerd/containerd.sock"
	VerificationInterval          = "5s"
	SingleNodeVerificationTimeout = "5m"
	MultiNodeVerificationTimeout  = "10m"
	DefaultKubernetesVersion      = "1.30.0"
)

func RepoType(family string) string {
	if strings.EqualFold(strings.TrimSpace(family), "debian") {
		return "deb-flat"
	}
	return "rpm"
}

func BackendImage(family string) string {
	if strings.EqualFold(strings.TrimSpace(family), "debian") {
		return "ubuntu:22.04"
	}
	return "rockylinux:9"
}

func PackageOutputDir(family string, release string, repoType string) string {
	family = strings.ToLower(strings.TrimSpace(family))
	release = strings.TrimSpace(release)
	repoType = strings.ToLower(strings.TrimSpace(repoType))
	if release == "" {
		return "packages/"
	}
	if repoType == "deb-flat" || family == "debian" {
		return filepath.ToSlash(filepath.Join("packages", "deb", release))
	}
	return filepath.ToSlash(filepath.Join("packages", "rpm", release))
}

func KubeadmPackages() []string {
	return []string{"kubeadm", "kubelet", "kubectl", "cri-tools", "containerd"}
}

func KubeadmImages(version string) []string {
	version = strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if version == "" || strings.EqualFold(version, "stable") {
		version = DefaultKubernetesVersion
	}
	return []string{
		"registry.k8s.io/kube-apiserver:v" + version,
		"registry.k8s.io/kube-controller-manager:v" + version,
		"registry.k8s.io/kube-scheduler:v" + version,
		"registry.k8s.io/kube-proxy:v" + version,
		"registry.k8s.io/pause:3.9",
	}
}

func VerificationTimeout(expectedNodeCount int) string {
	if expectedNodeCount > 1 {
		return MultiNodeVerificationTimeout
	}
	return SingleNodeVerificationTimeout
}

func ExpectedReadyCount(program askcontract.AuthoringProgram) (int, bool) {
	if program.Verification.ExpectedReadyCount > 0 {
		return program.Verification.ExpectedReadyCount, true
	}
	if program.Verification.ExpectedNodeCount > 0 {
		return program.Verification.ExpectedNodeCount, true
	}
	return 0, false
}

func ExpectedControlPlaneReady(program askcontract.AuthoringProgram) int {
	if program.Verification.ExpectedControlPlaneReady > 0 {
		return program.Verification.ExpectedControlPlaneReady
	}
	if program.Cluster.ControlPlaneCount > 0 {
		return program.Cluster.ControlPlaneCount
	}
	return 1
}
