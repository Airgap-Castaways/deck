package askprovider

import (
	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

// Re-export provider types from askcontract for backward compatibility.
type (
	Request        = askcontract.ProviderRequest
	ToolDefinition = askcontract.ProviderToolDefinition
	ToolCall       = askcontract.ProviderToolCall
	Response       = askcontract.ProviderResponse
	Client         = askcontract.ProviderClient
)
