package workflowcontext

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	CommandApply   = "apply"
	CommandPrepare = "prepare"

	SourceFilesystem = "filesystem"
	SourceServer     = "server"
)

type Context struct {
	Command  string
	Workflow Workflow
	Paths    Paths
}

type Workflow struct {
	Source   string
	Path     string
	Scenario string
}

type Paths struct {
	BundleRoot string
	OutputRoot string
	StateFile  string
}

type FieldDefinition struct {
	Path                      string
	Type                      string
	Prepare                   bool
	Apply                     bool
	Description               string
	IncludeInStateFingerprint bool
}

type contextFieldDefinition struct {
	FieldDefinition
	Value func(Context) any
}

var contextFieldDefinitions = []contextFieldDefinition{
	{
		FieldDefinition: FieldDefinition{Path: "context.command", Type: "string", Prepare: true, Apply: true, Description: "Current command, `prepare` or `apply`.", IncludeInStateFingerprint: true},
		Value:           func(c Context) any { return strings.TrimSpace(c.Command) },
	},
	{
		FieldDefinition: FieldDefinition{Path: "context.workflow.source", Type: "string", Prepare: true, Apply: true, Description: "Workflow source, `filesystem` or `server`.", IncludeInStateFingerprint: true},
		Value:           func(c Context) any { return strings.TrimSpace(c.Workflow.Source) },
	},
	{
		FieldDefinition: FieldDefinition{Path: "context.workflow.isServer", Type: "boolean", Prepare: true, Apply: true, Description: "Boolean convenience value derived from `context.workflow.source == \"server\"`."},
		Value:           func(c Context) any { return strings.TrimSpace(c.Workflow.Source) == SourceServer },
	},
	{
		FieldDefinition: FieldDefinition{Path: "context.workflow.path", Type: "string", Prepare: true, Apply: true, Description: "Resolved workflow file path or URL.", IncludeInStateFingerprint: true},
		Value:           func(c Context) any { return strings.TrimSpace(c.Workflow.Path) },
	},
	{
		FieldDefinition: FieldDefinition{Path: "context.workflow.scenario", Type: "string", Apply: true, Description: "Scenario name when apply resolved a scenario.", IncludeInStateFingerprint: true},
		Value:           func(c Context) any { return strings.TrimSpace(c.Workflow.Scenario) },
	},
	{
		FieldDefinition: FieldDefinition{Path: "context.paths.bundleRoot", Type: "string", Prepare: true, Apply: true, Description: "Prepared output root during prepare; selected bundle root during apply.", IncludeInStateFingerprint: true},
		Value:           func(c Context) any { return strings.TrimSpace(c.Paths.BundleRoot) },
	},
	{
		FieldDefinition: FieldDefinition{Path: "context.paths.outputRoot", Type: "string", Prepare: true, Description: "Prepared output root.", IncludeInStateFingerprint: true},
		Value:           func(c Context) any { return strings.TrimSpace(c.Paths.OutputRoot) },
	},
	{
		FieldDefinition: FieldDefinition{Path: "context.paths.stateFile", Type: "string", Apply: true, Description: "Apply state file path."},
		Value:           func(c Context) any { return strings.TrimSpace(c.Paths.StateFile) },
	},
}

func FieldDefinitions() []FieldDefinition {
	out := make([]FieldDefinition, len(contextFieldDefinitions))
	for i, def := range contextFieldDefinitions {
		out[i] = def.FieldDefinition
	}
	return out
}

func (c Context) RenderMap() map[string]any {
	out := map[string]any{}
	for _, def := range contextFieldDefinitions {
		setDottedPath(out, strings.TrimPrefix(def.Path, "context."), def.Value(c))
	}
	out["bundleRoot"] = strings.TrimSpace(c.Paths.BundleRoot)
	out["stateFile"] = strings.TrimSpace(c.Paths.StateFile)
	return out
}

func (c Context) StateFingerprint() string {
	payload := c.stateFingerprintPayload()
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf("%v", payload)
	}
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

func (c Context) stateFingerprintPayload() map[string]any {
	payload := map[string]any{}
	for _, def := range contextFieldDefinitions {
		if !def.IncludeInStateFingerprint {
			continue
		}
		setDottedPath(payload, strings.TrimPrefix(def.Path, "context."), def.Value(c))
	}
	return payload
}

func setDottedPath(root map[string]any, path string, value any) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	if len(parts) == 0 {
		return
	}
	current := root
	for _, part := range parts[:len(parts)-1] {
		if part == "" {
			return
		}
		next, _ := current[part].(map[string]any)
		if next == nil {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	if last := parts[len(parts)-1]; last != "" {
		current[last] = value
	}
}

func SourceForWorkflowPath(workflowPath string) string {
	trimmed := strings.TrimSpace(workflowPath)
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return SourceServer
	}
	return SourceFilesystem
}

func AbsolutePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return trimmed
	}
	return abs
}
