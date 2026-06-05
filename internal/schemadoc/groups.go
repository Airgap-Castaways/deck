package schemadoc

import (
	"fmt"
	"sort"
)

type GroupMetadata struct {
	Key          string
	Title        string
	Aliases      []string
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
		Aliases:      []string{"host prerequisites", "node prerequisites", "preflight"},
		Summary:      "Prepare a node so later runtime and Kubernetes steps can succeed predictably.",
		WhenToUse:    "Start here for host suitability checks and low-level node prerequisites such as swap, kernel modules, and sysctl settings.",
		Order:        10,
		TypicalFlows: []GroupFlow{{Title: "Preflight a node", Kinds: []string{"CheckHost", "Swap", "KernelModule", "Sysctl"}}},
		SeeAlso:      []string{"container-runtime", "kubernetes-lifecycle"},
	},
	"files-archives": {
		Key:          "files-archives",
		Title:        "Files and Archives",
		Aliases:      []string{"filesystem content", "file management", "artifact staging", "offline artifacts"},
		Summary:      "Fetch, create, copy, extract, edit, and link files used by prepare or apply workflows.",
		WhenToUse:    "Use this group for file artifacts, node-side file placement, archive extraction, and structured or textual file edits.",
		Order:        20,
		TypicalFlows: []GroupFlow{{Title: "Stage a prepared file", Kinds: []string{"DownloadFile"}}, {Title: "Place or edit node files", Kinds: []string{"EnsureDirectory", "WriteFile", "CopyFile", "EditYAML"}}, {Title: "Expand prepared content", Kinds: []string{"ExtractArchive", "CreateSymlink"}}},
		SeeAlso:      []string{"packages-repositories", "container-images", "services-systemd"},
	},
	"packages-repositories": {
		Key:          "packages-repositories",
		Title:        "Packages and Repositories",
		Aliases:      []string{"package management", "package staging", "artifact staging", "offline packages", "repositories"},
		Summary:      "Stage packages, configure repositories, refresh metadata, and install packages.",
		WhenToUse:    "Use this group for package workflows that start with prepare-time package staging and finish with apply-time repository setup and installation.",
		Order:        30,
		TypicalFlows: []GroupFlow{{Title: "Stage packages during prepare", Kinds: []string{"DownloadPackage"}}, {Title: "Install from offline repositories during apply", Kinds: []string{"ConfigureRepository", "RefreshRepository", "InstallPackage"}}},
		SeeAlso:      []string{"container-images", "host-prep", "kubernetes-lifecycle"},
	},
	"container-images": {
		Key:          "container-images",
		Title:        "Container Images",
		Aliases:      []string{"image staging", "artifact staging", "offline images", "registry images"},
		Summary:      "Stage container images during prepare, then load or verify them on nodes during apply.",
		WhenToUse:    "Use this group when image archives must be prepared for air-gapped apply runs or verified in the local container runtime.",
		Order:        40,
		TypicalFlows: []GroupFlow{{Title: "Stage offline images", Kinds: []string{"DownloadImage"}}, {Title: "Load prepared images", Kinds: []string{"LoadImage", "VerifyImage"}}},
		SeeAlso:      []string{"container-runtime", "kubernetes-lifecycle", "files-archives"},
	},
	"container-runtime": {
		Key:          "container-runtime",
		Title:        "Container Runtime",
		Aliases:      []string{"runtime services", "containerd", "registry hosts", "runtime config"},
		Summary:      "Configure containerd and registry host resolution on the node.",
		WhenToUse:    "Use this group when container runtime configuration, registry mirrors, or trust policy must be managed during apply.",
		Order:        50,
		TypicalFlows: []GroupFlow{{Title: "Configure containerd", Kinds: []string{"WriteContainerdConfig", "WriteContainerdRegistryHosts"}}, {Title: "Restart the runtime after config changes", Kinds: []string{"ManageService"}, Note: "Use Services and Systemd for the service lifecycle action."}},
		SeeAlso:      []string{"container-images", "services-systemd", "host-prep"},
	},
	"services-systemd": {
		Key:          "services-systemd",
		Title:        "Services and Systemd",
		Aliases:      []string{"runtime services", "service lifecycle", "systemd", "units"},
		Summary:      "Write systemd units and manage local service lifecycle actions.",
		WhenToUse:    "Use this group when workflows need to install unit files, reload systemd, or start, stop, restart, reload, enable, or disable services.",
		Order:        60,
		TypicalFlows: []GroupFlow{{Title: "Install or override a unit", Kinds: []string{"WriteSystemdUnit", "ManageService"}}, {Title: "Apply service state", Kinds: []string{"ManageService"}}},
		SeeAlso:      []string{"container-runtime", "waits-polling", "files-archives"},
	},
	"kubernetes-lifecycle": {
		Key:          "kubernetes-lifecycle",
		Title:        "Kubernetes Lifecycle",
		Aliases:      []string{"kubeadm", "cluster lifecycle", "bootstrap", "join"},
		Summary:      "Bootstrap or join kubeadm nodes and verify Kubernetes-specific cluster state.",
		WhenToUse:    "Use this group for kubeadm bootstrap, join, reset, upgrade, and Kubernetes cluster verification steps.",
		Order:        70,
		TypicalFlows: []GroupFlow{{Title: "Bootstrap a control plane", Kinds: []string{"InitKubeadm", "CheckKubernetesCluster"}, Note: "Offline control-plane flows usually stage packages and images before kubeadm runs."}, {Title: "Join workers", Kinds: []string{"JoinKubeadm", "CheckKubernetesCluster"}, Note: "Final cluster verification usually runs on the control-plane role after workers have joined."}},
		SeeAlso:      []string{"host-prep", "packages-repositories", "container-images", "waits-polling"},
	},
	"waits-polling": {
		Key:          "waits-polling",
		Title:        "Waits and Polling",
		Aliases:      []string{"waits", "polling", "readiness", "gates"},
		Summary:      "Wait for files, commands, services, and ports to converge before dependent steps continue.",
		WhenToUse:    "Use this group when later steps must wait on a specific local condition instead of assuming immediate convergence.",
		Order:        80,
		TypicalFlows: []GroupFlow{{Title: "Wait for service readiness", Kinds: []string{"WaitForService", "WaitForTCPPort"}}, {Title: "Wait for generated files", Kinds: []string{"WaitForFile", "WaitForCommand"}}},
		SeeAlso:      []string{"services-systemd", "kubernetes-lifecycle"},
	},
	"operator-interaction": {
		Key:          "operator-interaction",
		Title:        "Operator Interaction",
		Aliases:      []string{"operator gates", "manual input", "human approval", "prompts"},
		Summary:      "Print operator-facing messages and collect explicit local operator decisions or values.",
		WhenToUse:    "Use this group when a workflow needs a clear local checkpoint or an apply-time value that should flow through register outputs.",
		Order:        90,
		TypicalFlows: []GroupFlow{{Title: "Gate a local action", Kinds: []string{"Message", "Confirm"}}, {Title: "Collect an apply-time value", Kinds: []string{"Input"}}},
		SeeAlso:      []string{"waits-polling", "advanced"},
	},
	"advanced": {
		Key:          "advanced",
		Title:        "Advanced",
		Aliases:      []string{"command", "escape hatch", "custom command"},
		Summary:      "Use escape-hatch steps only when no typed step clearly matches the requested action.",
		WhenToUse:    "Start with typed groups first. Use the advanced group only for vendor tools, custom probes, or one-off local commands that deck does not model directly.",
		Order:        100,
		TypicalFlows: []GroupFlow{{Title: "Fallback path", Kinds: []string{"Command"}, Note: "Do not reimplement service, file, archive, sysctl, swap, kernel-module, or symlink actions with shell when typed steps already exist."}},
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
