package buildinfo

import (
	"strconv"
	"strings"
)

const Name = "deck"

var (
	Repository = "https://github.com/Airgap-Castaways/deck"
	Version    = "dev"
	Commit     = "unknown"
	Date       = "unknown"
	Dirty      = "false"
)

type Info struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Commit     string `json:"commit"`
	Date       string `json:"date"`
	Dirty      bool   `json:"dirty"`
	Repository string `json:"repository"`
}

func Current() Info {
	return Info{
		Name:       Name,
		Version:    normalizedValue(Version, "dev"),
		Commit:     normalizedValue(Commit, "unknown"),
		Date:       normalizedValue(Date, "unknown"),
		Dirty:      parseDirty(Dirty),
		Repository: normalizedValue(Repository, "unknown"),
	}
}

func Summary() string {
	info := Current()
	return info.Name + " " + info.Version
}

func normalizedValue(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func parseDirty(value string) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	return parsed
}
