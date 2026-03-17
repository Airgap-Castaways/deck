package buildinfo

import (
	"strconv"
	"strings"
)

const Name = "deck"

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
	Variant = "core"
	Dirty   = "false"
)

type Info struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Variant string `json:"variant"`
	Dirty   bool   `json:"dirty"`
}

func Current() Info {
	return Info{
		Name:    Name,
		Version: normalizedValue(Version, "dev"),
		Commit:  normalizedValue(Commit, "unknown"),
		Date:    normalizedValue(Date, "unknown"),
		Variant: normalizedValue(Variant, "core"),
		Dirty:   parseDirty(Dirty),
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
