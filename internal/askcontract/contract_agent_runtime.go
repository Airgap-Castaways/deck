package askcontract

import (
	"encoding/json"
	"fmt"
	"strings"
)

type AgentTurnResponse struct {
	Summary       string              `json:"summary"`
	Review        []string            `json:"review"`
	ToolCalls     []AgentToolCall     `json:"toolCalls,omitempty"`
	Finish        *AgentFinish        `json:"finish,omitempty"`
	Clarification *AgentClarification `json:"clarification,omitempty"`
}

type AgentToolCall struct {
	Name    string   `json:"name"`
	Path    string   `json:"path,omitempty"`
	Paths   []string `json:"paths,omitempty"`
	Query   string   `json:"query,omitempty"`
	Content string   `json:"content,omitempty"`
	Include []string `json:"include,omitempty"`
	Intent  string   `json:"intent,omitempty"`
}

type AgentFinish struct {
	Reason string `json:"reason,omitempty"`
}

type AgentClarification struct {
	Question string `json:"question"`
	Reason   string `json:"reason,omitempty"`
}

func AgentTurnResponseSchema() json.RawMessage {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
			"review":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"toolCalls": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":      map[string]any{"type": "string"},
						"tool":      map[string]any{"type": "string"},
						"path":      map[string]any{"type": "string"},
						"paths":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"query":     map[string]any{"type": "string"},
						"content":   map[string]any{"type": "string"},
						"include":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"intent":    map[string]any{"type": "string"},
						"args":      map[string]any{"type": "object"},
						"arguments": map[string]any{"type": "object"},
					},
				},
			},
			"finish": map[string]any{
				"oneOf": []any{
					map[string]any{"type": "object"},
					map[string]any{"type": "string"},
					map[string]any{"type": "boolean"},
				},
			},
			"clarification": map[string]any{
				"oneOf": []any{
					map[string]any{"type": "object"},
					map[string]any{"type": "string"},
				},
			},
		},
		"additionalProperties": true,
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		panic(err)
	}
	return raw
}

func ParseAgentTurn(raw string) (AgentTurnResponse, error) {
	cleaned := clean(raw)
	if cleaned == "" {
		return AgentTurnResponse{}, fmt.Errorf("agent turn response is empty")
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(cleaned), &root); err != nil {
		return AgentTurnResponse{}, fmt.Errorf("parse agent turn response: %w", err)
	}
	resp := AgentTurnResponse{}
	resp.Summary = firstString(root["summary"], root["message"], root["result"], "agent turn")
	resp.Review = stringList(root["review"])
	if len(resp.Review) == 0 {
		resp.Review = stringList(root["notes"])
	}
	if calls, ok, err := parseAgentToolCalls(root); err != nil {
		return AgentTurnResponse{}, err
	} else if ok {
		resp.ToolCalls = calls
	}
	if finish, ok := parseAgentFinish(root); ok {
		resp.Finish = finish
	}
	if clarification, ok := parseAgentClarification(root); ok {
		resp.Clarification = clarification
	}
	modeCount := 0
	if len(resp.ToolCalls) > 0 {
		modeCount++
	}
	if resp.Finish != nil {
		modeCount++
	}
	if resp.Clarification != nil {
		modeCount++
	}
	if modeCount != 1 {
		return AgentTurnResponse{}, fmt.Errorf("agent turn response must choose exactly one of toolCalls, finish, or clarification")
	}
	return resp, nil
}

func parseAgentToolCalls(root map[string]any) ([]AgentToolCall, bool, error) {
	candidates := []any{root["toolCalls"], root["tools"], root["calls"]}
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		rawCalls, ok := candidate.([]any)
		if !ok {
			return nil, false, fmt.Errorf("agent toolCalls must be an array")
		}
		calls := make([]AgentToolCall, 0, len(rawCalls))
		for _, rawCall := range rawCalls {
			callMap, ok := rawCall.(map[string]any)
			if !ok {
				return nil, false, fmt.Errorf("agent tool call must be an object")
			}
			call, err := parseAgentToolCall(callMap)
			if err != nil {
				return nil, false, err
			}
			calls = append(calls, call)
		}
		return calls, true, nil
	}
	if toolName := firstString(root["tool"], root["name"]); toolName != "" {
		call, err := parseAgentToolCall(root)
		if err != nil {
			return nil, false, err
		}
		return []AgentToolCall{call}, true, nil
	}
	return nil, false, nil
}

func parseAgentToolCall(raw map[string]any) (AgentToolCall, error) {
	args := nestedMap(raw, "args", "arguments")
	call := AgentToolCall{
		Name:    normalizeAgentToolName(firstString(raw["name"], raw["tool"], args["name"], args["tool"])),
		Path:    strings.TrimSpace(firstString(raw["path"], args["path"], args["file"], args["target"])),
		Paths:   mergeStringLists(raw["paths"], args["paths"], args["files"]),
		Query:   strings.TrimSpace(firstString(raw["query"], args["query"], args["term"], args["search"])),
		Content: strings.TrimSpace(firstString(raw["content"], args["content"], args["text"], args["body"])),
		Include: mergeStringLists(raw["include"], raw["includes"], args["include"], args["includes"], args["glob"]),
		Intent:  strings.TrimSpace(firstString(raw["intent"], args["intent"])),
	}
	if err := validateAgentToolCall(call); err != nil {
		return AgentToolCall{}, err
	}
	return call, nil
}

func parseAgentFinish(root map[string]any) (*AgentFinish, bool) {
	value, ok := root["finish"]
	if !ok {
		return nil, false
	}
	finish := &AgentFinish{}
	switch typed := value.(type) {
	case string:
		finish.Reason = strings.TrimSpace(typed)
	case bool:
		if !typed {
			return nil, false
		}
	case map[string]any:
		finish.Reason = firstString(typed["reason"], typed["message"], typed["result"], typed["status"])
	}
	finish.Reason = strings.TrimSpace(finish.Reason)
	return finish, true
}

func parseAgentClarification(root map[string]any) (*AgentClarification, bool) {
	if value, ok := root["clarification"]; ok {
		switch typed := value.(type) {
		case string:
			question := strings.TrimSpace(typed)
			if question == "" {
				return nil, false
			}
			return &AgentClarification{Question: question}, true
		case map[string]any:
			question := strings.TrimSpace(firstString(typed["question"], typed["message"], typed["prompt"]))
			if question == "" {
				return nil, false
			}
			return &AgentClarification{Question: question, Reason: strings.TrimSpace(firstString(typed["reason"]))}, true
		}
	}
	question := strings.TrimSpace(firstString(root["question"], root["prompt"]))
	if question == "" {
		return nil, false
	}
	return &AgentClarification{Question: question, Reason: strings.TrimSpace(firstString(root["reason"]))}, true
}

func normalizeAgentToolName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "file-search", "file_search":
		return "file_search"
	case "file-read", "file_read":
		return "file_read"
	case "file-write", "file_write":
		return "file_write"
	case "deck-init", "deck_init":
		return "deck_init"
	case "deck-lint", "deck_lint":
		return "deck_lint"
	case "mcp-web-search", "mcp_web_search":
		return "mcp_web_search"
	default:
		return strings.ToLower(strings.TrimSpace(name))
	}
}

func validateAgentToolCall(call AgentToolCall) error {
	switch call.Name {
	case "file_search":
		if call.Query == "" && call.Path == "" && len(call.Paths) == 0 {
			return fmt.Errorf("agent tool file_search requires query or path")
		}
	case "file_read":
		if call.Path == "" && len(call.Paths) == 0 {
			return fmt.Errorf("agent tool file_read requires path or paths")
		}
	case "file_write":
		if call.Path == "" {
			return fmt.Errorf("agent tool file_write requires path")
		}
		if call.Content == "" {
			return fmt.Errorf("agent tool file_write requires content")
		}
	case "deck_init":
	case "deck_lint":
	case "mcp_web_search":
		if call.Name == "mcp_web_search" && call.Query == "" {
			return fmt.Errorf("agent tool mcp_web_search requires query")
		}
	default:
		return fmt.Errorf("unsupported agent tool %q", call.Name)
	}
	return nil
}

func firstString(values ...any) string {
	for _, value := range values {
		typed, ok := value.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(typed)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringList(value any) []string {
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if typed, ok := item.(string); ok {
			trimmed := strings.TrimSpace(typed)
			if trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	return out
}

func mergeStringLists(values ...any) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		for _, item := range stringList(value) {
			if !seen[item] {
				seen[item] = true
				out = append(out, item)
			}
		}
		if typed, ok := value.(string); ok {
			trimmed := strings.TrimSpace(typed)
			if trimmed != "" && !seen[trimmed] {
				seen[trimmed] = true
				out = append(out, trimmed)
			}
		}
	}
	return out
}

func nestedMap(raw map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			if typed, ok := value.(map[string]any); ok {
				return typed
			}
		}
	}
	return map[string]any{}
}
