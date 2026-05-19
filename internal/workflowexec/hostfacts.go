package workflowexec

import (
	"strings"

	"github.com/Airgap-Castaways/deck/internal/maputil"
)

type HostFactsReader func(string) ([]byte, error)

type RuntimeFieldDefinition struct {
	Path        string
	Type        string
	Description string
}

type hostFactSource struct {
	osName        string
	arch          string
	osID          string
	osFamily      string
	osVersion     string
	osVersionID   string
	osIDLike      string
	kernelRelease string
}

type runtimeHostFieldDefinition struct {
	RuntimeFieldDefinition
	Value func(hostFactSource) any
}

var runtimeHostFieldDefinitions = []runtimeHostFieldDefinition{
	{
		RuntimeFieldDefinition: RuntimeFieldDefinition{Path: "runtime.host.os.name", Type: "string", Description: "Operating system name reported by the Go runtime."},
		Value:                  func(src hostFactSource) any { return src.osName },
	},
	{
		RuntimeFieldDefinition: RuntimeFieldDefinition{Path: "runtime.host.os.id", Type: "string", Description: "Distribution ID from `/etc/os-release` `ID`, lowercased."},
		Value:                  func(src hostFactSource) any { return src.osID },
	},
	{
		RuntimeFieldDefinition: RuntimeFieldDefinition{Path: "runtime.host.os.family", Type: "string", Description: "Inferred distribution family such as `debian` or `rhel`, or empty when unknown."},
		Value:                  func(src hostFactSource) any { return src.osFamily },
	},
	{
		RuntimeFieldDefinition: RuntimeFieldDefinition{Path: "runtime.host.os.version", Type: "string", Description: "Distribution version from `/etc/os-release` `VERSION`."},
		Value:                  func(src hostFactSource) any { return src.osVersion },
	},
	{
		RuntimeFieldDefinition: RuntimeFieldDefinition{Path: "runtime.host.os.versionId", Type: "string", Description: "Distribution version ID from `/etc/os-release` `VERSION_ID`."},
		Value:                  func(src hostFactSource) any { return src.osVersionID },
	},
	{
		RuntimeFieldDefinition: RuntimeFieldDefinition{Path: "runtime.host.os.release", Type: "string", Description: "Alias of `runtime.host.os.versionId` retained for existing workflows."},
		Value:                  func(src hostFactSource) any { return src.osVersionID },
	},
	{
		RuntimeFieldDefinition: RuntimeFieldDefinition{Path: "runtime.host.os.idLike", Type: "string", Description: "Distribution compatibility IDs from `/etc/os-release` `ID_LIKE`, lowercased."},
		Value:                  func(src hostFactSource) any { return src.osIDLike },
	},
	{
		RuntimeFieldDefinition: RuntimeFieldDefinition{Path: "runtime.host.arch", Type: "string", Description: "Normalized host architecture such as `amd64` or `arm64`."},
		Value:                  func(src hostFactSource) any { return src.arch },
	},
	{
		RuntimeFieldDefinition: RuntimeFieldDefinition{Path: "runtime.host.kernel.release", Type: "string", Description: "Kernel release from `/proc/sys/kernel/osrelease`."},
		Value:                  func(src hostFactSource) any { return src.kernelRelease },
	},
}

func RuntimeHostFieldDefinitions() []RuntimeFieldDefinition {
	out := make([]RuntimeFieldDefinition, len(runtimeHostFieldDefinitions))
	for i, def := range runtimeHostFieldDefinitions {
		out[i] = def.RuntimeFieldDefinition
	}
	return out
}

func DetectHostFacts(goos string, goarch string, readFile HostFactsReader) map[string]any {
	osName := strings.TrimSpace(goos)
	arch := normalizeHostArch(strings.TrimSpace(goarch))
	osRelease := parseOSReleaseVars(readFile)
	osID := strings.ToLower(strings.TrimSpace(osRelease["ID"]))
	osVersion := strings.TrimSpace(osRelease["VERSION"])
	osVersionID := strings.TrimSpace(osRelease["VERSION_ID"])
	osLike := strings.ToLower(strings.TrimSpace(osRelease["ID_LIKE"]))
	osFamily := inferOSFamily(osID, osLike)
	kernelRelease := readKernelRelease(readFile)

	source := hostFactSource{
		osName:        osName,
		arch:          arch,
		osID:          osID,
		osFamily:      osFamily,
		osVersion:     osVersion,
		osVersionID:   osVersionID,
		osIDLike:      osLike,
		kernelRelease: kernelRelease,
	}
	out := map[string]any{}
	for _, def := range runtimeHostFieldDefinitions {
		maputil.SetDottedPath(out, strings.TrimPrefix(def.Path, "runtime.host."), def.Value(source))
	}
	return out
}

func normalizeHostArch(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	default:
		return strings.ToLower(strings.TrimSpace(v))
	}
}

func parseOSReleaseVars(readFile HostFactsReader) map[string]string {
	raw, err := readFile("/etc/os-release")
	if err != nil {
		return map[string]string{}
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		if key != "" {
			out[key] = value
		}
	}
	return out
}

func inferOSFamily(id string, idLike string) string {
	candidate := strings.ToLower(strings.TrimSpace(id + " " + idLike))
	if candidate == "" {
		return ""
	}
	for _, token := range strings.Fields(candidate) {
		switch token {
		case "debian", "ubuntu":
			return "debian"
		case "rhel", "centos", "rocky", "almalinux", "fedora", "ol", "amzn":
			return "rhel"
		}
	}
	return ""
}

func readKernelRelease(readFile HostFactsReader) string {
	raw, err := readFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}
