package askprovider

import "github.com/Airgap-Castaways/deck/internal/askcontract"

// Re-export provider defaults from askcontract for backward compatibility.

const (
	DefaultProvider = askcontract.DefaultProvider
	DefaultModel    = askcontract.DefaultModel

	OpenAIBaseURL       = askcontract.OpenAIBaseURL
	OpenRouterBaseURL   = askcontract.OpenRouterBaseURL
	GeminiOpenAIBaseURL = askcontract.GeminiOpenAIBaseURL
)

var (
	NormalizeProvider      = askcontract.NormalizeProvider
	ProviderDefaultModel   = askcontract.ProviderDefaultModel
	ProviderDefaultEndpoint = askcontract.ProviderDefaultEndpoint
)
