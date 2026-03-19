package workflowexec

import "sort"

type StepDefinition struct {
	Kind       string
	SchemaFile string
	Visibility string
	Category   string
	Roles      []string
	Outputs    []string
	Actions    []StepActionDefinition
}

type StepActionDefinition struct {
	Name    string
	Roles   []string
	Outputs []string
	Fields  []string
}

func StepDefinitions() []StepDefinition {
	defs := []StepDefinition{
		stepDefWithCategory("Checks", "checks.schema.json", "public", "prepare", []string{"prepare"}, []string{"passed", "failedChecks"}),
		stepDefWithCategory("Artifacts", "artifacts.schema.json", "public", "apply", []string{"apply"}, nil),
		stepDefWithCategory("Packages", "packages.schema.json", "public", "packages", []string{"prepare", "apply"}, nil,
			actionDefWithFields("download", []string{"prepare"}, []string{"artifacts"}, []string{"action", "packages", "distro", "repo", "backend", "output"}),
			actionDefWithFields("install", []string{"apply"}, nil, []string{"action", "packages", "source", "restrictToRepos", "excludeRepos"}),
		),
		stepDefWithCategory("Directory", "directory.schema.json", "public", "filesystem", []string{"apply"}, []string{"path"}),
		stepDefWithCategory("Symlink", "symlink.schema.json", "public", "filesystem", []string{"apply"}, []string{"path"}),
		stepDefWithCategory("SystemdUnit", "systemd-unit.schema.json", "public", "system", []string{"apply"}, []string{"path"}),
		stepDefWithCategory("Containerd", "containerd.schema.json", "public", "runtime", []string{"apply"}, []string{"path"}),
		stepDefWithCategory("PackageCache", "package-cache.schema.json", "public", "packages", []string{"apply"}, nil),
		stepDefWithCategory("Swap", "swap.schema.json", "public", "system", []string{"apply"}, nil),
		stepDefWithCategory("KernelModule", "kernel-module.schema.json", "public", "system", []string{"apply"}, []string{"name", "names"}),
		stepDefWithCategory("Command", "command.schema.json", "advanced", "advanced", []string{"apply"}, nil),
		stepDefWithCategory("Service", "service.schema.json", "public", "system", []string{"apply"}, []string{"name", "names"}),
		stepDefWithCategory("Sysctl", "sysctl.schema.json", "public", "system", []string{"apply"}, nil),
		stepDefWithCategory("File", "file.schema.json", "public", "filesystem", []string{"prepare", "apply"}, nil,
			actionDefWithFields("download", []string{"prepare", "apply"}, []string{"path", "artifacts"}, []string{"action", "source", "fetch", "output"}),
			actionDefWithFields("write", []string{"apply"}, []string{"path"}, []string{"action", "path", "content", "contentFromTemplate", "mode"}),
			actionDefWithFields("copy", []string{"apply"}, []string{"dest"}, []string{"action", "src", "dest", "mode"}),
			actionDefWithFields("edit", []string{"apply"}, []string{"path"}, []string{"action", "path", "backup", "edits", "mode"}),
		),
		stepDefWithCategory("Repository", "repository.schema.json", "public", "packages", []string{"apply"}, nil,
			actionDefWithFields("configure", []string{"apply"}, []string{"path"}, []string{"action", "format", "path", "mode", "replaceExisting", "disableExisting", "backupPaths", "cleanupPaths", "refreshCache", "repositories", "timeout"}),
		),
		stepDefWithCategory("Image", "image.schema.json", "public", "containers", []string{"prepare", "apply"}, nil,
			actionDefWithFields("download", []string{"prepare"}, []string{"artifacts"}, []string{"action", "images", "auth", "backend", "output"}),
			actionDefWithFields("verify", []string{"apply"}, nil, []string{"action", "images", "command"}),
		),
		stepDefWithCategory("Wait", "wait.schema.json", "public", "control-flow", []string{"apply"}, nil,
			actionDefWithFields("serviceActive", []string{"apply"}, nil, []string{"action", "interval", "initialDelay", "name", "timeout", "pollInterval"}),
			actionDefWithFields("commandSuccess", []string{"apply"}, nil, []string{"action", "interval", "initialDelay", "command", "timeout", "pollInterval"}),
			actionDefWithFields("fileExists", []string{"apply"}, nil, []string{"action", "interval", "initialDelay", "path", "type", "nonEmpty", "timeout", "pollInterval"}),
			actionDefWithFields("fileAbsent", []string{"apply"}, nil, []string{"action", "interval", "initialDelay", "path", "type", "timeout", "pollInterval"}),
			actionDefWithFields("tcpPortClosed", []string{"apply"}, nil, []string{"action", "interval", "initialDelay", "address", "port", "timeout", "pollInterval"}),
			actionDefWithFields("tcpPortOpen", []string{"apply"}, nil, []string{"action", "interval", "initialDelay", "address", "port", "timeout", "pollInterval"}),
		),
		stepDefWithCategory("Kubeadm", "kubeadm.schema.json", "public", "kubernetes", []string{"apply"}, nil,
			actionDefWithFields("init", []string{"apply"}, []string{"joinFile"}, []string{"action", "configFile", "configTemplate", "pullImages", "outputJoinFile", "kubernetesVersion", "advertiseAddress", "podNetworkCIDR", "criSocket", "ignorePreflightErrors", "extraArgs", "skipIfAdminConfExists"}),
			actionDefWithFields("join", []string{"apply"}, nil, []string{"action", "configFile", "joinFile", "asControlPlane", "extraArgs"}),
			actionDefWithFields("reset", []string{"apply"}, nil, []string{"action", "force", "ignoreErrors", "stopKubelet", "criSocket", "extraArgs", "removePaths", "removeFiles", "cleanupContainers", "restartRuntimeService"}),
		),
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Kind < defs[j].Kind })
	return defs
}

func stepDefWithCategory(kind, schemaFile, visibility, category string, roles, outputs []string, actions ...StepActionDefinition) StepDefinition {
	def := StepDefinition{
		Kind:       kind,
		SchemaFile: schemaFile,
		Visibility: visibility,
		Category:   category,
		Roles:      append([]string(nil), roles...),
		Outputs:    append([]string(nil), outputs...),
		Actions:    append([]StepActionDefinition(nil), actions...),
	}
	sort.Strings(def.Roles)
	sort.Strings(def.Outputs)
	sort.Slice(def.Actions, func(i, j int) bool { return def.Actions[i].Name < def.Actions[j].Name })
	return def
}

func actionDefWithFields(name string, roles, outputs, fields []string) StepActionDefinition {
	def := StepActionDefinition{Name: name, Roles: append([]string(nil), roles...), Outputs: append([]string(nil), outputs...), Fields: append([]string(nil), fields...)}
	sort.Strings(def.Roles)
	sort.Strings(def.Outputs)
	sort.Strings(def.Fields)
	return def
}

func StepDefinitionForKind(kind string) (StepDefinition, bool) {
	for _, def := range StepDefinitions() {
		if def.Kind == kind {
			return def, true
		}
	}
	return StepDefinition{}, false
}
