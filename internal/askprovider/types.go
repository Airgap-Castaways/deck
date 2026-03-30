package askprovider

import (
	"context"
	"encoding/json"
	"time"
)

type Request struct {
	Kind               string
	Provider           string
	Model              string
	APIKey             string
	OAuthToken         string
	AccountID          string
	Endpoint           string
	SystemPrompt       string
	Prompt             string
	ResponseSchema     json.RawMessage
	ResponseSchemaName string
	MaxRetries         int
	Timeout            time.Duration
}

type Response struct {
	Content string
}

type Client interface {
	Generate(ctx context.Context, req Request) (Response, error)
}
