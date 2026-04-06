package schemadoc

import (
	"fmt"
	"sort"
)

type GroupMetadata struct {
	Key          string
	Title        string
	Summary      string
	WhenToUse    string
	Order        int
	TypicalFlows []GroupFlow
	SeeAlso      []string
}

type GroupFlow struct {
	Title string
	Kinds []string
	Note  string
}

var groupMetadata = map[string]GroupMetadata{
	"host-prep": {
		Key:          "host-prep",
		Title:        "Host Prep",
		Summary:      "Prepare a node so later runtime and Kubernetes steps can succeed predictably.",
		WhenToUse:    "Start here for host suitability checks and low-level node prerequisites such as swap, kernel modules, and sysctl settings.",
		Order:        10,
		TypicalFlows: []GroupFlow{{Title: "Preflight a node", Kinds: []string{"CheckHost", "Swap", "KernelModule", "Sysctl"}}},
		SeeAlso:      []string{"runtime-services", "kubernetes-lifecycle"},
	},
	"artifact-staging": {
		Key:          "artifact-staging",
		Title:        "Artifact Staging",
		Summary:      "Collect offline-ready files, packages, and images during prepare.",
		WhenToUse:    "Use this group during prepare when apply must avoid remote downloads and consume pre-staged bundle content.",
		Order:        20,
		TypicalFlows: []GroupFlow{{Title: "Stage offline packages", Kinds: []string{"DownloadPackage"}, Note: "Use Package Management during apply to consume the staged repository output."}, {Title: "Stage offline images", Kinds: []string{"DownloadImage"}, Note: "Use Runtime and Services during apply to load staged image archives into the local runtime."}, {Title: "Fetch bundle files", Kinds: []string{"DownloadFile"}}},
		SeeAlso:      []string{"package-management", "kubernetes-lifecycle", "filesystem-content"},
	},
	"filesystem-content": {
		Key:          "filesystem-content",
		Title:        "Filesystem and Content",
		Summary:      "Create directories, write files, edit structured documents, extract archives, and arrange paths on the node.",
		WhenToUse:    "Use this group for direct node-side filesystem mutations and content management during apply.",
		Order:        30,
		TypicalFlows: []GroupFlow{{Title: "Lay down config files", Kinds: []string{"EnsureDirectory", "WriteFile", "EditYAML"}}, {Title: "Expand prepared content", Kinds: []string{"ExtractArchive", "CreateSymlink"}}},
		SeeAlso:      []string{"runtime-services", "artifact-staging"},
	},
	"package-management": {
		Key:          "package-management",
		Title:        "Package Management",
		Summary:      "Configure local repositories, refresh package metadata, and install packages during apply.",
		WhenToUse:    "Use this group when a node must install packages from mirrored or staged package content without relying on online repositories.",
		Order:        40,
		TypicalFlows: []GroupFlow{{Title: "Offline package install", Kinds: []string{"ConfigureRepository", "RefreshRepository", "InstallPackage"}, Note: "This flow often starts with DownloadPackage in Artifact Staging."}},
		SeeAlso:      []string{"artifact-staging", "runtime-services"},
	},
	"runtime-services": {
		Key:          "runtime-services",
		Title:        "Runtime and Services",
		Summary:      "Configure container runtimes, load or verify local images, and manage systemd units and services on the node.",
		WhenToUse:    "Use this group when runtime settings, local image availability, or service state changes must take effect locally during apply.",
		Order:        50,
		TypicalFlows: []GroupFlow{{Title: "Configure containerd", Kinds: []string{"WriteContainerdConfig", "WriteContainerdRegistryHosts", "ManageService"}}, {Title: "Load offline images", Kinds: []string{"LoadImage", "VerifyImage"}, Note: "Use Artifact Staging during prepare to collect image archives before apply loads them into the local runtime."}, {Title: "Install or override a unit", Kinds: []string{"WriteSystemdUnit", "ManageService"}}},
		SeeAlso:      []string{"waits-polling", "filesystem-content", "host-prep"},
	},
	"kubernetes-lifecycle": {
		Key:          "kubernetes-lifecycle",
		Title:        "Kubernetes Lifecycle",
		Summary:      "Bootstrap or join kubeadm nodes and verify Kubernetes-specific cluster state.",
		WhenToUse:    "Use this group for kubeadm bootstrap, join, reset, upgrade, and Kubernetes cluster verification steps.",
		Order:        60,
		TypicalFlows: []GroupFlow{{Title: "Bootstrap a control plane", Kinds: []string{"InitKubeadm", "CheckKubernetesCluster"}, Note: "Offline control-plane flows usually load required images earlier through Runtime and Services."}, {Title: "Join workers", Kinds: []string{"JoinKubeadm", "CheckKubernetesCluster"}, Note: "Final cluster verification usually runs on the control-plane role after workers have joined."}},
		SeeAlso:      []string{"host-prep", "package-management", "waits-polling", "artifact-staging", "runtime-services"},
	},
	"waits-polling": {
		Key:          "waits-polling",
		Title:        "Waits and Polling",
		Summary:      "Wait for files, commands, services, and ports to converge before dependent steps continue.",
		WhenToUse:    "Use this group when later steps must wait on a specific local condition instead of assuming immediate convergence.",
		Order:        70,
		TypicalFlows: []GroupFlow{{Title: "Wait for service readiness", Kinds: []string{"WaitForService", "WaitForTCPPort"}}, {Title: "Wait for generated files", Kinds: []string{"WaitForFile", "WaitForCommand"}}},
		SeeAlso:      []string{"runtime-services", "kubernetes-lifecycle"},
	},
	"advanced": {
		Key:          "advanced",
		Title:        "Advanced",
		Summary:      "Use escape-hatch steps only when no typed step clearly matches the requested action.",
		WhenToUse:    "Start with typed groups first. Use this group only when the built-in typed steps do not fit the required host action.",
		Order:        80,
		TypicalFlows: []GroupFlow{{Title: "Fallback path", Kinds: []string{"Command"}, Note: "Prefer a typed step whenever one clearly fits."}},
	},
}

func GroupMeta(key string) (GroupMetadata, bool) {
	meta, ok := groupMetadata[key]
	return meta, ok
}

func MustGroupMeta(key string) GroupMetadata {
	meta, ok := GroupMeta(key)
	if !ok {
		panic(fmt.Sprintf("missing schemadoc group metadata for %s", key))
	}
	return meta
}

func GroupMetas() []GroupMetadata {
	out := make([]GroupMetadata, 0, len(groupMetadata))
	for _, meta := range groupMetadata {
		out = append(out, meta)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Key < out[j].Key
	})
	return out
}
