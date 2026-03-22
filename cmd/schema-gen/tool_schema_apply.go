package main

import "github.com/taedi90/deck/internal/stepspec"

func generateCommandToolSchema() map[string]any {
	root := stepEnvelopeSchema("Command", "CommandStep", "Escape hatch for commands that are not yet covered by typed steps.", "public")
	patchCommandToolSchema(root)
	return root
}

func patchCommandToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.Command{})
	if err != nil {
		panic(err)
	}
	properties := propertyMap(spec)
	setMap(properties, "command", stringArraySchema(1, false))
	setMap(properties, "env", map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}})
	setMap(properties, "sudo", map[string]any{"type": "boolean", "default": false})
	setMap(properties, "timeout", durationStringSchema())
	spec["required"] = []any{"command"}
	setMap(props, "spec", spec)
}

func generateWriteContainerdConfigToolSchema() map[string]any {
	root := stepEnvelopeSchema("WriteContainerdConfig", "WriteContainerdConfigStep", "Writes the containerd config.toml file on the node.", "public")
	patchWriteContainerdConfigToolSchema(root)
	return root
}

func patchWriteContainerdConfigToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.WriteContainerdConfig{})
	if err != nil {
		panic(err)
	}
	delete(propertyMap(spec), "timeout")
	properties := propertyMap(spec)
	setMap(properties, "path", minLenStringSchema())
	setMap(properties, "configPath", minLenStringSchema())
	setMap(properties, "systemdCgroup", map[string]any{"type": "boolean"})
	setMap(properties, "createDefault", map[string]any{"type": "boolean", "default": true})
	setMap(props, "spec", spec)
}

func generateWriteContainerdRegistryHostsToolSchema() map[string]any {
	root := stepEnvelopeSchema("WriteContainerdRegistryHosts", "WriteContainerdRegistryHostsStep", "Writes containerd registry host configuration for mirrors and trust policy.", "public")
	patchWriteContainerdRegistryHostsToolSchema(root)
	return root
}

func patchWriteContainerdRegistryHostsToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.WriteContainerdRegistryHosts{})
	if err != nil {
		panic(err)
	}
	properties := propertyMap(spec)
	setMap(properties, "path", minLenStringSchema())
	if registryHosts, ok := properties["registryHosts"].(map[string]any); ok {
		registryHosts["minItems"] = 1
		if items, ok := registryHosts["items"].(map[string]any); ok {
			items["required"] = []any{"registry", "server", "host", "capabilities", "skipVerify"}
			itemProps := propertyMap(items)
			setMap(itemProps, "registry", minLenStringSchema())
			setMap(itemProps, "server", minLenStringSchema())
			setMap(itemProps, "host", minLenStringSchema())
			setMap(itemProps, "capabilities", stringArraySchema(1, true))
			setMap(itemProps, "skipVerify", map[string]any{"type": "boolean"})
		}
	}
	spec["required"] = []any{"path", "registryHosts"}
	setMap(props, "spec", spec)
}

func generateEnsureDirectoryToolSchema() map[string]any {
	root := stepEnvelopeSchema("EnsureDirectory", "EnsureDirectoryStep", "Ensures a directory exists on the local node.", "public")
	patchEnsureDirectoryToolSchema(root)
	return root
}

func patchEnsureDirectoryToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.EnsureDirectory{})
	if err != nil {
		panic(err)
	}
	properties := propertyMap(spec)
	setMap(properties, "path", minLenStringSchema())
	setMap(properties, "mode", modeSchema())
	spec["required"] = []any{"path"}
	setMap(props, "spec", spec)
}

func generateDownloadImageToolSchema() map[string]any {
	root := stepEnvelopeSchema("DownloadImage", "DownloadImageStep", "Downloads images into bundle output storage.", "public")
	patchDownloadImageToolSchema(root)
	return root
}

func patchDownloadImageToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.DownloadImage{})
	if err != nil {
		panic(err)
	}
	delete(propertyMap(spec), "timeout")
	spec["required"] = []any{"images"}
	properties := propertyMap(spec)
	setMap(properties, "images", stringArraySchema(1, false))
	setMap(properties, "auth", imageAuthSchema())
	setMap(properties, "backend", imageBackendSchema())
	setMap(properties, "outputDir", minLenStringSchema())
	setMap(props, "spec", spec)
}

func generateImageLoadToolSchema() map[string]any {
	root := stepEnvelopeSchema("LoadImage", "LoadImageStep", "Loads prepared image archives into the local container runtime.", "public")
	patchImageLoadToolSchema(root)
	return root
}

func patchImageLoadToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.LoadImage{})
	if err != nil {
		panic(err)
	}
	delete(propertyMap(spec), "timeout")
	spec["required"] = []any{"images"}
	properties := propertyMap(spec)
	setMap(properties, "images", stringArraySchema(1, false))
	setMap(properties, "sourceDir", minLenStringSchema())
	setMap(properties, "runtime", enumStringSchema("auto", "ctr", "docker", "podman"))
	setMap(properties, "command", stringArraySchema(1, false))
	setMap(props, "spec", spec)
}

func generateVerifyImageToolSchema() map[string]any {
	root := stepEnvelopeSchema("VerifyImage", "VerifyImageStep", "Verifies that required images already exist on the node.", "public")
	patchVerifyImageToolSchema(root)
	return root
}

func patchVerifyImageToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.VerifyImage{})
	if err != nil {
		panic(err)
	}
	delete(propertyMap(spec), "timeout")
	spec["required"] = []any{"images"}
	properties := propertyMap(spec)
	setMap(properties, "images", stringArraySchema(1, false))
	setMap(properties, "command", stringArraySchema(1, false))
	setMap(props, "spec", spec)
}

func imageAuthSchema() map[string]any {
	return map[string]any{
		"type":     "array",
		"minItems": 1,
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []any{"registry", "basic"},
			"properties": map[string]any{
				"registry": minLenStringSchema(),
				"basic": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []any{"username", "password"},
					"properties": map[string]any{
						"username": map[string]any{"type": "string"},
						"password": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}

func imageBackendSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"engine": enumStringSchema("go-containerregistry"),
		},
	}
}

func generateCheckHostToolSchema() map[string]any {
	root := stepEnvelopeSchema("CheckHost", "CheckHostStep", "Runs host checks before prepare execution.", "public")
	patchCheckHostToolSchema(root)
	return root
}

func patchCheckHostToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.CheckHost{})
	if err != nil {
		panic(err)
	}
	properties := propertyMap(spec)
	setMap(properties, "checks", map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string", "enum": []any{"os", "arch", "kernelModules", "swap", "binaries"}}})
	setMap(properties, "binaries", stringArraySchema(0, false))
	setMap(properties, "failFast", map[string]any{"type": "boolean", "default": true})
	spec["required"] = []any{"checks"}
	setMap(props, "spec", spec)
}

func generateKernelModuleToolSchema() map[string]any {
	root := stepEnvelopeSchema("KernelModule", "KernelModuleStep", "Loads and persists required kernel modules on the local node.", "public")
	patchKernelModuleToolSchema(root)
	return root
}

func patchKernelModuleToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.KernelModule{})
	if err != nil {
		panic(err)
	}
	delete(propertyMap(spec), "timeout")
	properties := propertyMap(spec)
	setMap(properties, "name", minLenStringSchema())
	setMap(properties, "names", stringArraySchema(1, true))
	setMap(properties, "load", map[string]any{"type": "boolean", "default": true})
	setMap(properties, "persist", map[string]any{"type": "boolean", "default": true})
	setMap(properties, "persistFile", map[string]any{"type": "string"})
	spec["oneOf"] = []any{
		map[string]any{"required": []any{"name"}, "not": map[string]any{"required": []any{"names"}}},
		map[string]any{"required": []any{"names"}, "not": map[string]any{"required": []any{"name"}}},
	}
	setMap(props, "spec", spec)
}

func generateInitKubeadmToolSchema() map[string]any {
	root := stepEnvelopeSchema("InitKubeadm", "InitKubeadmStep", "Runs kubeadm init and writes a join command file.", "public")
	patchInitKubeadmToolSchema(root)
	return root
}

func patchInitKubeadmToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.KubeadmInit{})
	if err != nil {
		panic(err)
	}
	delete(propertyMap(spec), "timeout")
	spec["required"] = []any{"outputJoinFile"}
	properties := propertyMap(spec)
	setMap(properties, "outputJoinFile", minLenStringSchema())
	setMap(properties, "skipIfAdminConfExists", map[string]any{"type": "boolean", "default": true})
	setMap(props, "spec", spec)
}

func generateJoinKubeadmToolSchema() map[string]any {
	root := stepEnvelopeSchema("JoinKubeadm", "JoinKubeadmStep", "Runs kubeadm join.", "public")
	patchJoinKubeadmToolSchema(root)
	return root
}

func patchJoinKubeadmToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.KubeadmJoin{})
	if err != nil {
		panic(err)
	}
	delete(propertyMap(spec), "timeout")
	spec["oneOf"] = []any{
		map[string]any{"required": []any{"joinFile"}},
		map[string]any{"required": []any{"configFile"}},
	}
	properties := propertyMap(spec)
	setMap(properties, "joinFile", minLenStringSchema())
	setMap(properties, "configFile", minLenStringSchema())
	setMap(properties, "asControlPlane", map[string]any{"type": "boolean", "default": false})
	setMap(props, "spec", spec)
}

func generateResetKubeadmToolSchema() map[string]any {
	root := stepEnvelopeSchema("ResetKubeadm", "ResetKubeadmStep", "Runs kubeadm reset and optional cleanup steps.", "public")
	patchResetKubeadmToolSchema(root)
	return root
}

func patchResetKubeadmToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.KubeadmReset{})
	if err != nil {
		panic(err)
	}
	delete(propertyMap(spec), "timeout")
	properties := propertyMap(spec)
	setMap(properties, "force", map[string]any{"type": "boolean", "default": false})
	setMap(properties, "ignoreErrors", map[string]any{"type": "boolean", "default": false})
	setMap(properties, "stopKubelet", map[string]any{"type": "boolean", "default": true})
	setMap(props, "spec", spec)
}

func generateUpgradeKubeadmToolSchema() map[string]any {
	root := stepEnvelopeSchema("UpgradeKubeadm", "UpgradeKubeadmStep", "Runs kubeadm upgrade apply and optionally restarts kubelet.", "public")
	patchUpgradeKubeadmToolSchema(root)
	return root
}

func patchUpgradeKubeadmToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.KubeadmUpgrade{})
	if err != nil {
		panic(err)
	}
	delete(propertyMap(spec), "timeout")
	spec["required"] = []any{"kubernetesVersion"}
	properties := propertyMap(spec)
	setMap(properties, "kubernetesVersion", minLenStringSchema())
	setMap(properties, "restartKubelet", map[string]any{"type": "boolean", "default": true})
	setMap(properties, "kubeletService", minLenStringSchema())
	setMap(props, "spec", spec)
}

func generateCheckClusterToolSchema() map[string]any {
	root := stepEnvelopeSchema("CheckCluster", "CheckClusterStep", "Polls and verifies Kubernetes cluster state on the local node.", "public")
	patchCheckClusterToolSchema(root)
	return root
}

func patchCheckClusterToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.ClusterCheck{})
	if err != nil {
		panic(err)
	}
	properties := propertyMap(spec)
	setMap(properties, "kubeconfig", minLenStringSchema())
	setMap(properties, "interval", durationStringSchema())
	setMap(properties, "initialDelay", durationStringSchema())
	setMap(properties, "timeout", durationStringSchema())

	nodes := propertyMap(spec)["nodes"].(map[string]any)
	nodeProps := propertyMap(nodes)
	setMap(nodeProps, "total", map[string]any{"type": "integer", "minimum": 0})
	setMap(nodeProps, "ready", map[string]any{"type": "integer", "minimum": 0})
	setMap(nodeProps, "controlPlaneReady", map[string]any{"type": "integer", "minimum": 0})

	versions := propertyMap(spec)["versions"].(map[string]any)
	versionProps := propertyMap(versions)
	setMap(versionProps, "targetVersion", minLenStringSchema())
	setMap(versionProps, "server", minLenStringSchema())
	setMap(versionProps, "kubelet", minLenStringSchema())
	setMap(versionProps, "kubeadm", minLenStringSchema())
	setMap(versionProps, "nodeName", minLenStringSchema())
	setMap(versionProps, "reportPath", minLenStringSchema())

	kubeSystem := propertyMap(spec)["kubeSystem"].(map[string]any)
	kubeSystemProps := propertyMap(kubeSystem)
	setMap(kubeSystemProps, "readyNames", stringArraySchema(0, false))
	setMap(kubeSystemProps, "readyPrefixes", stringArraySchema(0, false))
	setMap(kubeSystemProps, "reportPath", minLenStringSchema())
	setMap(kubeSystemProps, "jsonReportPath", minLenStringSchema())
	readyPrefixMinimums, _ := kubeSystemProps["readyPrefixMinimums"].(map[string]any)
	if items, ok := readyPrefixMinimums["items"].(map[string]any); ok {
		items["required"] = []any{"prefix", "minReady"}
		itemProps := propertyMap(items)
		setMap(itemProps, "prefix", minLenStringSchema())
		setMap(itemProps, "minReady", map[string]any{"type": "integer", "minimum": 1})
	}

	fileAssertions, _ := propertyMap(spec)["fileAssertions"].(map[string]any)
	if items, ok := fileAssertions["items"].(map[string]any); ok {
		items["required"] = []any{"path", "contains"}
		itemProps := propertyMap(items)
		setMap(itemProps, "path", minLenStringSchema())
		setMap(itemProps, "contains", stringArraySchema(1, false))
	}

	reports := propertyMap(spec)["reports"].(map[string]any)
	reportProps := propertyMap(reports)
	setMap(reportProps, "nodesPath", minLenStringSchema())
	setMap(reportProps, "clusterNodesPath", minLenStringSchema())

	setMap(props, "spec", spec)
}
