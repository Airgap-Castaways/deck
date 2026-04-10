package askcli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askdiagnostic"
	"github.com/Airgap-Castaways/deck/internal/askprovider"
	"github.com/Airgap-Castaways/deck/internal/askstate"
	"github.com/Airgap-Castaways/deck/internal/stepmeta"
	"github.com/Airgap-Castaways/deck/internal/validate"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
	deckschemas "github.com/Airgap-Castaways/deck/schemas"
)

const (
	authorToolFinish        = "author_finish"
	authorToolClarification = "author_request_clarification"
)

type authorToolCall struct {
	Name       string
	Path       string
	Paths      []string
	Pattern    string
	Query      string
	Glob       string
	Content    string
	Offset     int
	Limit      int
	OldString  string
	NewString  string
	ReplaceAll bool
	Topic      string
	Kind       string
	Intent     string
}

type authorFinishCall struct {
	Summary string `json:"summary,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type authorClarificationCall struct {
	Question string `json:"question"`
	Reason   string `json:"reason,omitempty"`
}

type authorSchemaReadCall struct {
	Topic string `json:"topic,omitempty"`
	Kind  string `json:"kind,omitempty"`
}

type authorReadResult struct {
	Path         string `json:"path"`
	Exists       bool   `json:"exists"`
	Source       string `json:"source,omitempty"`
	Content      string `json:"content,omitempty"`
	StartLine    int    `json:"startLine,omitempty"`
	EndLine      int    `json:"endLine,omitempty"`
	TotalLines   int    `json:"totalLines,omitempty"`
	Truncated    bool   `json:"truncated,omitempty"`
	WasCandidate bool   `json:"wasCandidate,omitempty"`
}

type authorGrepMatch struct {
	Path      string `json:"path"`
	Line      int    `json:"line,omitempty"`
	Match     string `json:"match,omitempty"`
	Snippet   string `json:"snippet,omitempty"`
	Source    string `json:"source,omitempty"`
	Candidate bool   `json:"candidate,omitempty"`
}

type authorReadSnapshot struct {
	Content     string
	Exists      bool
	Candidate   bool
	Offset      int
	Limit       int
	WasFullRead bool
}

func authoringToolDefinitions(session *authoringAgentSession) []askprovider.ToolDefinition {
	defs := make([]askprovider.ToolDefinition, 0, len(session.availableTools)+2)
	for _, name := range activeAuthoringTools(session) {
		defs = append(defs, askprovider.ToolDefinition{
			Name:        name,
			Description: authorToolDescription(name),
			Parameters:  authorToolParameters(name),
		})
	}
	defs = append(defs,
		askprovider.ToolDefinition{
			Name:        authorToolFinish,
			Description: "Finish the current authoring session when candidate files and verifier state satisfy the request.",
			Parameters: mustJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"summary": nullableStringSchema(),
					"reason":  nullableStringSchema(),
				},
				"required":             []string{"summary", "reason"},
				"additionalProperties": false,
			}),
		},
		askprovider.ToolDefinition{
			Name:        authorToolClarification,
			Description: "Ask one targeted clarification question only when the request is blocked on missing information that cannot be safely inferred.",
			Parameters: mustJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question": nullableStringSchema(),
					"reason":   nullableStringSchema(),
				},
				"required":             []string{"question", "reason"},
				"additionalProperties": false,
			}),
		},
	)
	return defs
}

func authorToolDescription(name string) string {
	switch name {
	case "read":
		return "Read approved files or current candidate files. Supports offset and limit for ranged reads."
	case "glob":
		return "Find files by name pattern in the current workspace and candidate state."
	case "grep":
		return "Search file contents in the current workspace and candidate state with a regex pattern."
	case "file_write":
		return "Create or replace an approved workflow file in candidate state."
	case "file_edit":
		return "Edit an approved file by exact string replacement after reading it first."
	case "validate":
		return "Validate the current candidate state and return structured deck diagnostics."
	case "schema":
		return "Read repo-owned workflow rules or exact typed step schema/example data."
	case "init":
		return "Prepare the default workspace scaffold for an empty workflow workspace."
	case "web_search":
		return "Look up optional external evidence when policy allows it."
	default:
		return "Authoring tool"
	}
}

func authorToolParameters(name string) json.RawMessage {
	switch name {
	case "read":
		return mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":   nullableStringSchema(),
				"paths":  nullableStringArraySchema(),
				"offset": nullableIntegerSchema(),
				"limit":  nullableIntegerSchema(),
			},
			"required":             []string{"path", "paths", "offset", "limit"},
			"additionalProperties": false,
		})
	case "glob":
		return mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": nullableStringSchema(),
				"path":    nullableStringSchema(),
			},
			"required":             []string{"pattern", "path"},
			"additionalProperties": false,
		})
	case "grep":
		return mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": nullableStringSchema(),
				"path":    nullableStringSchema(),
				"glob":    nullableStringSchema(),
				"limit":   nullableIntegerSchema(),
			},
			"required":             []string{"pattern", "path", "glob", "limit"},
			"additionalProperties": false,
		})
	case "file_write":
		return mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string"},
				"content": map[string]any{"type": "string"},
			},
			"required":             []string{"path", "content"},
			"additionalProperties": false,
		})
	case "file_edit":
		return mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":        map[string]any{"type": "string"},
				"old_string":  map[string]any{"type": "string"},
				"new_string":  map[string]any{"type": "string"},
				"replace_all": nullableBooleanSchema(),
			},
			"required":             []string{"path", "old_string", "new_string", "replace_all"},
			"additionalProperties": false,
		})
	case "validate", "init":
		return mustJSON(map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false})
	case "schema":
		return mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"topic": nullableStringSchema(),
				"kind":  nullableStringSchema(),
			},
			"required":             []string{"topic", "kind"},
			"additionalProperties": false,
		})
	case "web_search":
		return mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":  nullableStringSchema(),
				"intent": nullableStringSchema(),
			},
			"required":             []string{"query", "intent"},
			"additionalProperties": false,
		})
	default:
		return mustJSON(map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false})
	}
}

func nullableStringSchema() map[string]any {
	return map[string]any{"type": []string{"string", "null"}}
}

func nullableStringArraySchema() map[string]any {
	return map[string]any{"type": []string{"array", "null"}, "items": map[string]any{"type": "string"}}
}

func nullableIntegerSchema() map[string]any {
	return map[string]any{"type": []string{"integer", "null"}}
}

func nullableBooleanSchema() map[string]any {
	return map[string]any{"type": []string{"boolean", "null"}}
}

func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}

func normalizeAuthorToolName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "read", "file_read", "file-read":
		return "read"
	case "glob":
		return "glob"
	case "grep", "file_search", "file-search":
		return "grep"
	case "file_write", "file-write", "write":
		return "file_write"
	case "file_edit", "file-edit", "edit":
		return "file_edit"
	case "validate", "deck_lint", "deck-lint":
		return "validate"
	case "schema", "deck_schema_read", "deck-schema-read":
		return "schema"
	case "init", "deck_init", "deck-init":
		return "init"
	case "web_search", "mcp_web_search", "mcp-web-search":
		return "web_search"
	default:
		return strings.TrimSpace(name)
	}
}

func parseAuthorToolCall(call askprovider.ToolCall) (authorToolCall, error) {
	name := normalizeAuthorToolName(call.Name)
	if name == "" {
		return authorToolCall{}, fmt.Errorf("provider tool call missing name")
	}
	parsed := map[string]any{}
	if len(call.Arguments) > 0 {
		if err := json.Unmarshal(call.Arguments, &parsed); err != nil {
			return authorToolCall{}, fmt.Errorf("parse tool arguments for %s: %w", name, err)
		}
	}
	toolCall := authorToolCall{
		Name:      name,
		Path:      stringArg(parsed, "path"),
		Paths:     stringArrayArg(parsed, "paths"),
		Pattern:   firstNonEmptyString(stringArg(parsed, "pattern"), stringArg(parsed, "query")),
		Query:     firstNonEmptyString(stringArg(parsed, "query"), stringArg(parsed, "pattern")),
		Glob:      stringArg(parsed, "glob"),
		Content:   stringArg(parsed, "content"),
		Offset:    intArg(parsed, "offset"),
		Limit:     intArg(parsed, "limit"),
		OldString: stringArg(parsed, "old_string"),
		NewString: stringArg(parsed, "new_string"),
		Topic:     firstNonEmptyString(stringArg(parsed, "topic"), stringArg(parsed, "target")),
		Kind:      stringArg(parsed, "kind"),
		Intent:    stringArg(parsed, "intent"),
	}
	if value, ok := parsed["replace_all"].(bool); ok {
		toolCall.ReplaceAll = value
	}
	if err := validateAuthorToolCall(toolCall); err != nil {
		return authorToolCall{}, err
	}
	return toolCall, nil
}

func validateAuthorToolCall(call authorToolCall) error {
	switch call.Name {
	case "read":
		if strings.TrimSpace(call.Path) == "" && len(call.Paths) == 0 {
			return fmt.Errorf("author tool read requires path or paths")
		}
	case "glob":
		if strings.TrimSpace(call.Pattern) == "" {
			return fmt.Errorf("author tool glob requires pattern")
		}
	case "grep":
		if strings.TrimSpace(call.Pattern) == "" {
			return fmt.Errorf("author tool grep requires pattern")
		}
	case "file_write":
		if strings.TrimSpace(call.Path) == "" {
			return fmt.Errorf("author tool file_write requires path")
		}
		if call.Content == "" {
			return fmt.Errorf("author tool file_write requires content")
		}
	case "file_edit":
		if strings.TrimSpace(call.Path) == "" {
			return fmt.Errorf("author tool file_edit requires path")
		}
		if call.OldString == "" && call.NewString == "" {
			return fmt.Errorf("author tool file_edit requires old_string or new_string")
		}
	case "validate", "init":
	case "schema":
		if strings.TrimSpace(call.Topic) == "" && strings.TrimSpace(call.Kind) == "" {
			return nil
		}
	case "web_search":
		if strings.TrimSpace(call.Query) == "" {
			return fmt.Errorf("author tool web_search requires query")
		}
	default:
		return fmt.Errorf("unsupported author tool %q", call.Name)
	}
	return nil
}

func parseAuthorFinishTool(call askprovider.ToolCall) (authorFinishCall, error) {
	finish := authorFinishCall{}
	if len(call.Arguments) == 0 {
		return finish, nil
	}
	if err := json.Unmarshal(call.Arguments, &finish); err != nil {
		return authorFinishCall{}, fmt.Errorf("parse finish tool arguments: %w", err)
	}
	finish.Summary = strings.TrimSpace(finish.Summary)
	finish.Reason = strings.TrimSpace(finish.Reason)
	return finish, nil
}

func parseAuthorClarificationTool(call askprovider.ToolCall) (authorClarificationCall, error) {
	clarify := authorClarificationCall{}
	if err := json.Unmarshal(call.Arguments, &clarify); err != nil {
		return authorClarificationCall{}, fmt.Errorf("parse clarification tool arguments: %w", err)
	}
	clarify.Question = strings.TrimSpace(clarify.Question)
	clarify.Reason = strings.TrimSpace(clarify.Reason)
	if clarify.Question == "" {
		return authorClarificationCall{}, fmt.Errorf("clarification tool requires question")
	}
	return clarify, nil
}

func renderProviderResponse(resp askprovider.Response) string {
	if len(resp.ToolCalls) == 0 {
		return strings.TrimSpace(resp.Content)
	}
	payload := map[string]any{"toolCalls": []map[string]any{}}
	calls := make([]map[string]any, 0, len(resp.ToolCalls))
	for _, call := range resp.ToolCalls {
		item := map[string]any{"id": call.ID, "name": normalizeAuthorToolName(call.Name)}
		if len(call.Arguments) > 0 {
			var parsed any
			if err := json.Unmarshal(call.Arguments, &parsed); err == nil {
				item["arguments"] = parsed
			} else {
				item["arguments"] = strings.TrimSpace(string(call.Arguments))
			}
		}
		calls = append(calls, item)
	}
	payload["toolCalls"] = calls
	if strings.TrimSpace(resp.Content) != "" {
		payload["content"] = strings.TrimSpace(resp.Content)
	}
	return renderAgentPayload(payload)
}

func authoringToolCatalogSummary(session *authoringAgentSession) string {
	items := append([]string(nil), activeAuthoringTools(session)...)
	items = append(items, authorToolFinish, authorToolClarification)
	return strings.Join(items, ", ")
}

func workspaceSummaryForPrompt(session *authoringAgentSession) string {
	if !session.workspace.HasWorkflowTree {
		return "none yet"
	}
	paths := make([]string, 0, len(session.workspace.Files))
	for _, file := range session.workspace.Files {
		paths = append(paths, file.Path)
	}
	return strings.Join(paths, ", ")
}

func activeAuthoringTools(session *authoringAgentSession) []string {
	if session == nil {
		return nil
	}
	tools := append([]string(nil), session.availableTools...)
	if session.decision.Route == "draft" && !session.workspace.HasWorkflowTree {
		if session.verificationFailure > 0 {
			return []string{"read", "file_edit", "file_write", "validate", "schema"}
		}
		if len(session.candidateByPath) == 0 {
			if schemaLoopCount(session.toolEvents) >= 2 {
				return []string{"file_write", "init", "validate"}
			}
			return []string{"read", "file_write", "init", "validate", "schema", "web_search"}
		}
	}
	return tools
}

func buildSchemaReadPayload(call authorSchemaReadCall) (map[string]any, error) {
	topic := normalizeSchemaTopic(call.Topic)
	kind := strings.TrimSpace(call.Kind)
	workflowPayload := map[string]any{
		"ok":               true,
		"topic":            "workflow",
		"supportedVersion": validate.SupportedWorkflowVersion(),
		"topLevelModes":    validate.WorkflowTopLevelModes(),
		"supportedModes":   validate.SupportedWorkflowRoles(),
		"importRule":       validate.WorkflowImportRule(),
		"invariantNotes":   validate.WorkflowInvariantNotes(),
		"example": strings.Join([]string{
			"version: " + validate.SupportedWorkflowVersion(),
			"steps:",
			"  - id: example",
			"    kind: Command",
			"    spec:",
			"      command: [\"true\"]",
		}, "\n"),
	}
	if topic != "step" {
		if kind != "" {
			workflowPayload["note"] = fmt.Sprintf("topic %q maps to workflow rules; ignore kind %q unless you need an exact step schema", strings.TrimSpace(call.Topic), kind)
		}
		return workflowPayload, nil
	}
	if kind == "" {
		return map[string]any{
			"ok":              true,
			"topic":           "step",
			"note":            "step schema requests require an exact registered step kind",
			"knownKinds":      stepmeta.RegisteredKinds(),
			"workflowExample": workflowPayload["example"],
		}, nil
	}
	entry, ok, err := stepmeta.Lookup(kind)
	if err != nil {
		return nil, err
	}
	if !ok {
		return map[string]any{
			"ok":              true,
			"topic":           "step",
			"kind":            kind,
			"knownKinds":      stepmeta.RegisteredKinds(),
			"suggestedKinds":  suggestStepKinds(kind),
			"note":            fmt.Sprintf("%s is not an exact registered step kind; use one of the suggested or known kinds", kind),
			"workflowExample": workflowPayload["example"],
		}, nil
	}
	payload := map[string]any{
		"ok":                       true,
		"topic":                    "step",
		"kind":                     entry.Definition.Kind,
		"summary":                  strings.TrimSpace(entry.Docs.Summary),
		"whenToUse":                strings.TrimSpace(entry.Docs.WhenToUse),
		"example":                  strings.TrimSpace(entry.Docs.Example),
		"schemaFile":               strings.TrimSpace(entry.Definition.SchemaFile),
		"keyFields":                entry.Definition.Ask.KeyFields,
		"validationHints":          entry.Definition.Ask.ValidationHints,
		"constrainedLiteralFields": entry.Definition.Ask.ConstrainedLiteralFields,
	}
	if rawSchema, err := deckschemas.ToolSchema(entry.Definition.SchemaFile); err == nil {
		var schema any
		if json.Unmarshal(rawSchema, &schema) == nil {
			payload["schema"] = schema
		}
	}
	return payload, nil
}

func schemaLoopCount(events []askstate.AgentToolEvent) int {
	count := 0
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.Name != "schema" {
			if event.Name == "file_write" || event.Name == "validate" || event.Name == "file_edit" || event.Name == "read" {
				break
			}
			continue
		}
		count++
	}
	return count
}

func normalizeSchemaTopic(topic string) string {
	switch strings.ToLower(strings.TrimSpace(topic)) {
	case "", "workflow", "workflows", "scenario", "scenarios", "draft", "authoring":
		return "workflow"
	case "step", "steps", "builder", "builders", "kind":
		return "step"
	default:
		return "workflow"
	}
}

func suggestStepKinds(kind string) []string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		return nil
	}
	out := []string{}
	for _, candidate := range stepmeta.RegisteredKinds() {
		lower := strings.ToLower(candidate)
		if strings.Contains(lower, kind) || strings.Contains(kind, lower) || strings.HasPrefix(lower, kind) {
			out = append(out, candidate)
		}
	}
	if len(out) == 0 {
		for _, candidate := range stepmeta.RegisteredKinds() {
			lower := strings.ToLower(candidate)
			if strings.HasPrefix(lower, string(kind[0])) {
				out = append(out, candidate)
				if len(out) >= 8 {
					break
				}
			}
		}
	}
	return out
}

func buildLintRepairContext(diags []askdiagnostic.Diagnostic) map[string]any {
	context := map[string]any{
		"workflow": map[string]any{
			"supportedVersion": validate.SupportedWorkflowVersion(),
			"topLevelModes":    validate.WorkflowTopLevelModes(),
			"supportedModes":   validate.SupportedWorkflowRoles(),
		},
	}
	stepKinds := dedupeStepKinds(diags)
	if len(stepKinds) == 0 {
		return context
	}
	steps := make([]map[string]any, 0, len(stepKinds))
	for _, kind := range stepKinds {
		entry, ok, err := stepmeta.Lookup(kind)
		if err != nil || !ok {
			continue
		}
		item := map[string]any{
			"kind":                entry.Definition.Kind,
			"example":             strings.TrimSpace(entry.Docs.Example),
			"summary":             strings.TrimSpace(entry.Docs.Summary),
			"whenToUse":           strings.TrimSpace(entry.Docs.WhenToUse),
			"schemaFile":          strings.TrimSpace(entry.Definition.SchemaFile),
			"keyFields":           append([]string(nil), entry.Definition.Ask.KeyFields...),
			"validationHints":     append([]stepmeta.ValidationHint(nil), entry.Definition.Ask.ValidationHints...),
			"constrainedLiterals": append([]stepmeta.ConstrainedLiteralField(nil), entry.Definition.Ask.ConstrainedLiteralFields...),
		}
		if rawSchema, err := deckschemas.ToolSchema(entry.Definition.SchemaFile); err == nil {
			var schema any
			if json.Unmarshal(rawSchema, &schema) == nil {
				item["schema"] = schema
			}
		}
		steps = append(steps, item)
	}
	if len(steps) > 0 {
		context["steps"] = steps
	}
	return context
}

func dedupeStepKinds(diags []askdiagnostic.Diagnostic) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, diag := range diags {
		kind := strings.TrimSpace(diag.StepKind)
		if kind == "" || seen[kind] {
			continue
		}
		seen[kind] = true
		out = append(out, kind)
	}
	return out
}

func approvedPathsSummary(paths []string) string {
	if len(paths) == 0 {
		return workspacepaths.CanonicalApplyWorkflow
	}
	return strings.Join(paths, ", ")
}

func authorToolPaths(call authorToolCall) []string {
	paths := []string{}
	if strings.TrimSpace(call.Path) != "" {
		paths = append(paths, strings.TrimSpace(call.Path))
	}
	for _, path := range call.Paths {
		clean := strings.TrimSpace(path)
		if clean != "" {
			paths = append(paths, clean)
		}
	}
	return dedupe(paths)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func stringArg(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func stringArrayArg(raw map[string]any, key string) []string {
	value, ok := raw[key]
	if !ok {
		return nil
	}
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		text, ok := item.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func intArg(raw map[string]any, key string) int {
	value, ok := raw[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
}

func matchesGlob(path string, pattern string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	path = filepath.ToSlash(strings.TrimSpace(path))
	if pattern == "" {
		return true
	}
	if matched, err := filepath.Match(pattern, path); err == nil && matched {
		return true
	}
	basePattern := filepath.Base(pattern)
	if basePattern != "" && basePattern != pattern {
		if matched, err := filepath.Match(basePattern, filepath.Base(path)); err == nil && matched {
			return true
		}
	}
	collapsed := strings.ReplaceAll(pattern, "**/", "")
	if collapsed != pattern {
		if matched, err := filepath.Match(collapsed, path); err == nil && matched {
			return true
		}
		if matched, err := filepath.Match(collapsed, filepath.Base(path)); err == nil && matched {
			return true
		}
	}
	return strings.Contains(path, strings.Trim(pattern, "*"))
}
