package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func computeStateKey(workflowBytes []byte, effectiveVars map[string]any) string {
	normalizedWorkflow := normalizeWorkflowBytes(workflowBytes)
	varLines := renderEffectiveVars(effectiveVars)

	h := sha256.New()
	_, _ = h.Write(normalizedWorkflow)
	_, _ = h.Write([]byte("\n--vars--\n"))
	_, _ = h.Write([]byte(varLines))
	return hex.EncodeToString(h.Sum(nil))
}

func normalizeWorkflowBytes(workflowBytes []byte) []byte {
	if len(workflowBytes) == 0 {
		return nil
	}
	return []byte(strings.ReplaceAll(string(workflowBytes), "\r\n", "\n"))
}

func computeWorkflowSHA256(workflowBytes []byte) string {
	normalizedWorkflow := normalizeWorkflowBytes(workflowBytes)
	h := sha256.Sum256(normalizedWorkflow)
	return hex.EncodeToString(h[:])
}

func renderEffectiveVars(effectiveVars map[string]any) string {
	if len(effectiveVars) == 0 {
		return ""
	}
	keys := make([]string, 0, len(effectiveVars))
	for key := range effectiveVars {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, key := range keys {
		b.WriteString(key)
		b.WriteString("=")
		b.WriteString(stableVarValue(effectiveVars[key]))
		b.WriteString("\n")
	}
	return b.String()
}

func stableVarValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(encoded)
}
