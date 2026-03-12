package workflowexec

import "strings"

type HostFactsReader func(string) ([]byte, error)

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

	return map[string]any{
		"os": map[string]any{
			"name":      osName,
			"id":        osID,
			"family":    osFamily,
			"version":   osVersion,
			"versionId": osVersionID,
			"release":   osVersionID,
			"idLike":    osLike,
		},
		"arch": arch,
		"kernel": map[string]any{
			"release": kernelRelease,
		},
	}
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
