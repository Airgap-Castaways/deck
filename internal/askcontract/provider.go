package askcontract

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// Provider types — LLM provider abstraction layer.

type ProviderRequest struct {
	Kind                     string
	Provider                 string
	Model                    string
	APIKey                   string
	OAuthToken               string
	AccountID                string
	Endpoint                 string
	SystemPrompt             string
	Prompt                   string
	ResponseSchema           json.RawMessage
	ResponseSchemaName       string
	Tools                    []ProviderToolDefinition
	ToolChoiceRequired       bool
	DisableParallelToolCalls bool
	MaxRetries               int
	Timeout                  time.Duration
}

type ProviderToolDefinition struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type ProviderToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

type ProviderResponse struct {
	Content   string
	ToolCalls []ProviderToolCall
}

type ProviderClient interface {
	Generate(ctx context.Context, req ProviderRequest) (ProviderResponse, error)
}

// Provider configuration defaults.

const (
	DefaultProvider = "openai"
	DefaultModel    = "gpt-5.3-codex-spark"

	OpenAIBaseURL       = "https://api.openai.com/v1"
	OpenRouterBaseURL   = "https://openrouter.ai/api/v1"
	GeminiOpenAIBaseURL = "https://generativelanguage.googleapis.com/v1beta/openai/"
)

func NormalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func ProviderDefaultModel(provider string) string {
	switch NormalizeProvider(provider) {
	case "gemini", "google", "google-openai":
		return "gemini-2.5-flash"
	default:
		return DefaultModel
	}
}

func ProviderDefaultEndpoint(provider string) string {
	switch NormalizeProvider(provider) {
	case "gemini", "google", "google-openai":
		return GeminiOpenAIBaseURL
	case "openrouter":
		return OpenRouterBaseURL
	default:
		return OpenAIBaseURL
	}
}
