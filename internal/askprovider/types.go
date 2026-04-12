package askprovider

import "github.com/Airgap-Castaways/deck/internal/askcontract"

// Re-export provider types from askcontract for backward compatibility.

type Request = askcontract.ProviderRequest
type ToolDefinition = askcontract.ProviderToolDefinition
type ToolCall = askcontract.ProviderToolCall
type Response = askcontract.ProviderResponse
type Client = askcontract.ProviderClient
