package askprovider

import (
	"context"
	"encoding/json"
	"time"
)

type Request struct {
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
	Tools                    []ToolDefinition
	ToolChoiceRequired       bool
	DisableParallelToolCalls bool
	MaxRetries               int
	Timeout                  time.Duration
}

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

type Response struct {
	Content   string
	ToolCalls []ToolCall
}

type Client interface {
	Generate(ctx context.Context, req Request) (Response, error)
}
