package stepmeta

var (
	PatchRefreshRepositoryToolSchema   = patchRefreshRepositoryToolSchema
	PatchDownloadPackageToolSchema     = patchDownloadPackageToolSchema
	PatchInstallPackageToolSchema      = patchInstallPackageToolSchema
	PatchInstallAptPackageToolSchema   = patchInstallAptPackageToolSchema
	PatchInstallDnfPackageToolSchema   = patchInstallDnfPackageToolSchema
	PatchConfigureRepositoryToolSchema = patchConfigureRepositoryToolSchema
)

func patchRefreshRepositoryToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec := specMap(root)
	delete(propertyMap(spec), "timeout")
	properties := propertyMap(spec)
	setMap(properties, "manager", enumStringSchema("auto", "apt", "dnf"))
	setMap(properties, "clean", map[string]any{"type": "boolean"})
	setMap(properties, "update", map[string]any{"type": "boolean"})
	setMap(properties, "restrictToRepos", stringArraySchema(0, true))
	setMap(properties, "excludeRepos", stringArraySchema(0, true))
	spec["anyOf"] = []any{
		map[string]any{"properties": map[string]any{"clean": map[string]any{"const": true}}, "required": []any{"clean"}},
		map[string]any{"properties": map[string]any{"update": map[string]any{"const": true}}, "required": []any{"update"}},
	}
	setMap(props, "spec", spec)
}

func patchDownloadPackageToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec := specMap(root)
	delete(propertyMap(spec), "timeout")
	properties := propertyMap(spec)
	setMap(properties, "packages", stringArraySchema(1, false))
	distro := propertyMap(spec)["distro"].(map[string]any)
	setMap(propertyMap(distro), "family", enumStringSchema("debian", "rhel"))
	setMap(propertyMap(distro), "release", minLenStringSchema())
	distro["required"] = []any{"family", "release"}
	repo := propertyMap(spec)["repo"].(map[string]any)
	repoProps := propertyMap(repo)
	setMap(repoProps, "type", enumStringSchema("deb-flat", "rpm"))
	setMap(repoProps, "generate", map[string]any{"type": "boolean"})
	setMap(repoProps, "pkgsDir", minLenStringSchema())
	repo["required"] = []any{"type"}
	if items, ok := repoProps["modules"].(map[string]any); ok {
		if itemMap, ok := items["items"].(map[string]any); ok {
			itemMap["required"] = []any{"name", "stream"}
			itemProps := propertyMap(itemMap)
			setMap(itemProps, "name", minLenStringSchema())
			setMap(itemProps, "stream", minLenStringSchema())
		}
	}
	backend := propertyMap(spec)["backend"].(map[string]any)
	backendProps := propertyMap(backend)
	setMap(backendProps, "mode", enumStringSchema("container"))
	setMap(backendProps, "runtime", enumStringSchema("auto", "docker", "podman"))
	setMap(backendProps, "image", minLenStringSchema())
	backend["required"] = []any{"mode", "image"}
	setMap(properties, "outputDir", minLenStringSchema())
	spec["required"] = []any{"packages", "distro", "repo", "backend"}
	spec["allOf"] = []any{
		map[string]any{
			"if":   map[string]any{"properties": map[string]any{"distro": map[string]any{"properties": map[string]any{"family": map[string]any{"const": "debian"}}, "required": []any{"family"}}}, "required": []any{"distro"}},
			"then": map[string]any{"properties": map[string]any{"repo": map[string]any{"properties": map[string]any{"type": map[string]any{"const": "deb-flat"}}}}},
		},
		map[string]any{
			"if":   map[string]any{"properties": map[string]any{"distro": map[string]any{"properties": map[string]any{"family": map[string]any{"const": "rhel"}}, "required": []any{"family"}}}, "required": []any{"distro"}},
			"then": map[string]any{"properties": map[string]any{"repo": map[string]any{"properties": map[string]any{"type": map[string]any{"const": "rpm"}}}}},
		},
	}
	setMap(props, "spec", spec)
}

func patchInstallPackageToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec := specMap(root)
	delete(propertyMap(spec), "timeout")
	properties := propertyMap(spec)
	patchInstallPackageCommonProperties(properties)
	setMap(properties, "manager", enumStringSchema("auto", "apt", "dnf"))
	patchInstallPackageAptProperties(properties)
	patchInstallPackageDnfProperties(properties)
	spec["required"] = []any{"packages"}
	spec["allOf"] = []any{
		map[string]any{"if": map[string]any{"required": []any{"apt"}}, "then": map[string]any{"required": []any{"manager"}, "properties": map[string]any{"manager": map[string]any{"const": "apt"}}}},
		map[string]any{"if": map[string]any{"required": []any{"dnf"}}, "then": map[string]any{"required": []any{"manager"}, "properties": map[string]any{"manager": map[string]any{"const": "dnf"}}}},
		map[string]any{"if": map[string]any{"properties": map[string]any{"manager": map[string]any{"const": "apt"}}, "required": []any{"manager"}}, "then": map[string]any{"not": map[string]any{"required": []any{"dnf"}}}},
		map[string]any{"if": map[string]any{"properties": map[string]any{"manager": map[string]any{"const": "dnf"}}, "required": []any{"manager"}}, "then": map[string]any{"not": map[string]any{"required": []any{"apt"}}}},
	}
	setMap(props, "spec", spec)
}

func patchInstallAptPackageToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec := specMap(root)
	delete(propertyMap(spec), "timeout")
	properties := propertyMap(spec)
	patchInstallPackageCommonProperties(properties)
	patchInstallPackageAptProperties(properties)
	spec["required"] = []any{"packages"}
	setMap(props, "spec", spec)
}

func patchInstallDnfPackageToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec := specMap(root)
	delete(propertyMap(spec), "timeout")
	properties := propertyMap(spec)
	patchInstallPackageCommonProperties(properties)
	patchInstallPackageDnfProperties(properties)
	spec["required"] = []any{"packages"}
	setMap(props, "spec", spec)
}

func patchInstallPackageCommonProperties(properties map[string]any) {
	if source, ok := properties["source"].(map[string]any); ok {
		source["required"] = []any{"type", "path"}
		sourceProps := propertyMap(source)
		setMap(sourceProps, "type", map[string]any{"const": "local-repo"})
		setMap(sourceProps, "path", minLenStringSchema())
	}
	setMap(properties, "packages", stringArraySchema(1, false))
	setMap(properties, "restrictToRepos", stringArraySchema(0, true))
	setMap(properties, "excludeRepos", stringArraySchema(0, true))
}

func patchInstallPackageAptProperties(properties map[string]any) {
	apt, ok := properties["apt"].(map[string]any)
	if !ok {
		return
	}
	aptProps := propertyMap(apt)
	setMap(aptProps, "fixBroken", map[string]any{"type": "boolean"})
	setMap(aptProps, "installRecommends", map[string]any{"type": "boolean"})
	setMap(aptProps, "dpkgOptions", stringArraySchema(0, true))
	setMap(aptProps, "defaultRelease", minLenStringSchema())
	setMap(aptProps, "allowDowngrade", map[string]any{"type": "boolean"})
	setMap(aptProps, "failOnAutoremove", map[string]any{"type": "boolean"})
}

func patchInstallPackageDnfProperties(properties map[string]any) {
	dnf, ok := properties["dnf"].(map[string]any)
	if !ok {
		return
	}
	dnfProps := propertyMap(dnf)
	setMap(dnfProps, "skipBroken", map[string]any{"type": "boolean"})
	setMap(dnfProps, "allowErasing", map[string]any{"type": "boolean"})
	setMap(dnfProps, "installWeakDeps", map[string]any{"type": "boolean"})
	setMap(dnfProps, "disableGpgCheck", map[string]any{"type": "boolean"})
	setMap(dnfProps, "best", map[string]any{"type": "boolean"})
	setMap(dnfProps, "cacheOnly", map[string]any{"type": "boolean"})
	setMap(dnfProps, "excludePackages", stringArraySchema(0, true))
}

func patchConfigureRepositoryToolSchema(root map[string]any) {
	props := propertyMap(root)
	spec := specMap(root)
	properties := propertyMap(spec)
	setMap(properties, "format", enumStringSchema("auto", "deb", "rpm"))
	setMap(properties, "path", map[string]any{"type": "string"})
	setMap(properties, "mode", modeSchema())
	setMap(properties, "replaceExisting", map[string]any{"type": "boolean"})
	setMap(properties, "disableExisting", map[string]any{"type": "boolean"})
	setMap(properties, "backupPaths", stringArraySchema(0, false))
	setMap(properties, "cleanupPaths", stringArraySchema(0, false))
	if repos, ok := properties["repositories"].(map[string]any); ok {
		repos["type"] = "array"
		repos["minItems"] = 1
		if items, ok := repos["items"].(map[string]any); ok {
			itemProps := propertyMap(items)
			items["anyOf"] = []any{
				map[string]any{"required": []any{"id"}},
				map[string]any{"required": []any{"baseurl"}},
			}
			setMap(itemProps, "id", minLenStringSchema())
			setMap(itemProps, "name", minLenStringSchema())
			setMap(itemProps, "baseurl", minLenStringSchema())
			setMap(itemProps, "enabled", map[string]any{"type": "boolean"})
			setMap(itemProps, "gpgcheck", map[string]any{"type": "boolean"})
			setMap(itemProps, "gpgkey", minLenStringSchema())
			setMap(itemProps, "trusted", map[string]any{"type": "boolean"})
			setMap(itemProps, "suite", minLenStringSchema())
			setMap(itemProps, "component", minLenStringSchema())
			setMap(itemProps, "type", enumStringSchema("deb", "deb-src"))
			setMap(itemProps, "extra", map[string]any{"type": "object", "additionalProperties": map[string]any{"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "boolean"}, map[string]any{"type": "integer"}, map[string]any{"type": "number"}}}})
		}
	}
	spec["required"] = []any{"repositories"}
	spec["allOf"] = []any{
		map[string]any{
			"if":   map[string]any{"properties": map[string]any{"format": map[string]any{"const": "deb"}}, "required": []any{"format"}},
			"then": map[string]any{"properties": map[string]any{"repositories": map[string]any{"items": map[string]any{"required": []any{"baseurl"}}}}},
		},
		map[string]any{
			"if":   map[string]any{"properties": map[string]any{"format": map[string]any{"const": "rpm"}}, "required": []any{"format"}},
			"then": map[string]any{"properties": map[string]any{"repositories": map[string]any{"items": map[string]any{"required": []any{"id"}}}}},
		},
	}
	setMap(props, "spec", spec)
}
