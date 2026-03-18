package workflowexec

import "sort"

type StepDefinition struct {
	Kind       string
	SchemaFile string
	Roles      []string
	Outputs    []string
	Actions    []StepActionDefinition
}

type StepActionDefinition struct {
	Name    string
	Roles   []string
	Outputs []string
}

func StepDefinitions() []StepDefinition {
	defs := []StepDefinition{
		stepDef("Checks", "checks.schema.json", []string{"prepare"}, []string{"passed", "failedChecks"}),
		stepDef("Artifacts", "artifacts.schema.json", []string{"apply"}, nil),
		stepDef("Packages", "packages.schema.json", []string{"prepare", "apply"}, nil,
			actionDef("download", []string{"prepare"}, []string{"artifacts"}),
			actionDef("install", []string{"apply"}, nil),
		),
		stepDef("Directory", "directory.schema.json", []string{"apply"}, []string{"path"}),
		stepDef("Symlink", "symlink.schema.json", []string{"apply"}, []string{"path"}),
		stepDef("SystemdUnit", "systemd-unit.schema.json", []string{"apply"}, []string{"path"}),
		stepDef("Containerd", "containerd.schema.json", []string{"apply"}, []string{"path"}),
		stepDef("PackageCache", "package-cache.schema.json", []string{"apply"}, nil),
		stepDef("Swap", "swap.schema.json", []string{"apply"}, nil),
		stepDef("KernelModule", "kernel-module.schema.json", []string{"apply"}, []string{"name", "names"}),
		stepDef("Command", "command.schema.json", []string{"apply"}, nil),
		stepDef("Service", "service.schema.json", []string{"apply"}, []string{"name", "names"}),
		stepDef("Sysctl", "sysctl.schema.json", []string{"apply"}, nil),
		stepDef("File", "file.schema.json", []string{"prepare", "apply"}, nil,
			actionDef("download", []string{"prepare", "apply"}, []string{"path", "artifacts"}),
			actionDef("write", []string{"apply"}, []string{"path"}),
			actionDef("copy", []string{"apply"}, []string{"dest"}),
			actionDef("edit", []string{"apply"}, []string{"path"}),
		),
		stepDef("Repository", "repository.schema.json", []string{"apply"}, nil,
			actionDef("configure", []string{"apply"}, []string{"path"}),
		),
		stepDef("Image", "image.schema.json", []string{"prepare", "apply"}, nil,
			actionDef("download", []string{"prepare"}, []string{"artifacts"}),
			actionDef("verify", []string{"apply"}, nil),
		),
		stepDef("Wait", "wait.schema.json", []string{"apply"}, nil,
			actionDef("serviceActive", []string{"apply"}, nil),
			actionDef("commandSuccess", []string{"apply"}, nil),
			actionDef("fileExists", []string{"apply"}, nil),
			actionDef("fileAbsent", []string{"apply"}, nil),
			actionDef("tcpPortClosed", []string{"apply"}, nil),
			actionDef("tcpPortOpen", []string{"apply"}, nil),
		),
		stepDef("Kubeadm", "kubeadm.schema.json", []string{"apply"}, nil,
			actionDef("init", []string{"apply"}, []string{"joinFile"}),
			actionDef("join", []string{"apply"}, nil),
			actionDef("reset", []string{"apply"}, nil),
		),
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Kind < defs[j].Kind })
	return defs
}

func stepDef(kind, schemaFile string, roles, outputs []string, actions ...StepActionDefinition) StepDefinition {
	def := StepDefinition{
		Kind:       kind,
		SchemaFile: schemaFile,
		Roles:      append([]string(nil), roles...),
		Outputs:    append([]string(nil), outputs...),
		Actions:    append([]StepActionDefinition(nil), actions...),
	}
	sort.Strings(def.Roles)
	sort.Strings(def.Outputs)
	sort.Slice(def.Actions, func(i, j int) bool { return def.Actions[i].Name < def.Actions[j].Name })
	return def
}

func actionDef(name string, roles, outputs []string) StepActionDefinition {
	def := StepActionDefinition{Name: name, Roles: append([]string(nil), roles...), Outputs: append([]string(nil), outputs...)}
	sort.Strings(def.Roles)
	sort.Strings(def.Outputs)
	return def
}
