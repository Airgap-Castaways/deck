package workflowexec

import (
	"encoding/json"
	"fmt"
)

func DecodeSpec[T any](spec map[string]any) (T, error) {
	var out T
	raw, err := json.Marshal(spec)
	if err != nil {
		return out, fmt.Errorf("marshal spec: %w", err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("decode spec: %w", err)
	}
	return out, nil
}
