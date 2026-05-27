package install

import (
	"context"
	"slices"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/config"
)

type secretValuesContextKey struct{}

func withSecretValues(ctx context.Context, secrets []string) context.Context {
	if len(secrets) == 0 {
		return ctx
	}
	return context.WithValue(ctx, secretValuesContextKey{}, append([]string(nil), secrets...))
}

func secretValuesFromContext(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	values, _ := ctx.Value(secretValuesContextKey{}).([]string)
	return append([]string(nil), values...)
}

func secretOutputKeys(step config.Step, rendered map[string]any) map[string]bool {
	if step.Kind != "Input" || !boolSpecValue(rendered, "secret") {
		return nil
	}
	return map[string]bool{"value": true}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func maskError(err error, secrets []string) error {
	if err == nil {
		return nil
	}
	masked := maskSecrets(err.Error(), secrets)
	if masked == err.Error() {
		return err
	}
	return &maskedError{err: err, masked: masked}
}

type maskedError struct {
	err    error
	masked string
}

func (e *maskedError) Error() string {
	if e == nil {
		return ""
	}
	return e.masked
}

func (e *maskedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func boolSpecValue(values map[string]any, key string) bool {
	raw, ok := values[key]
	if !ok {
		return false
	}
	value, _ := raw.(bool)
	return value
}

func cloneRuntimeSecrets(input map[string]RuntimeSecret) map[string]RuntimeSecret {
	out := make(map[string]RuntimeSecret, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func publicRuntimeVars(runtimeVars map[string]any, runtimeSecrets map[string]RuntimeSecret) map[string]any {
	out := make(map[string]any, len(runtimeVars))
	for key, value := range runtimeVars {
		if _, secret := runtimeSecrets[key]; secret {
			continue
		}
		out[key] = value
	}
	return out
}

func runtimeSecretValues(runtimeVars map[string]any, runtimeSecrets map[string]RuntimeSecret) []string {
	values := make([]string, 0, len(runtimeSecrets))
	for key := range runtimeSecrets {
		value, ok := runtimeVars[key].(string)
		if ok && value != "" {
			values = append(values, value)
		}
	}
	slices.SortFunc(values, func(a, b string) int { return len(b) - len(a) })
	return values
}

func maskSecrets(text string, secrets []string) string {
	masked := text
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		masked = strings.ReplaceAll(masked, secret, "***")
	}
	return masked
}

func sanitizedState(st *State, runtimeVars map[string]any, runtimeSecrets map[string]RuntimeSecret, preserveCompletedPhases bool) *State {
	if st == nil {
		return nil
	}
	out := *st
	secrets := cloneRuntimeSecrets(runtimeSecrets)
	out.RuntimeVars = publicRuntimeVars(runtimeVars, secrets)
	out.RuntimeSecrets = secrets
	out.Error = maskSecrets(out.Error, runtimeSecretValues(runtimeVars, secrets))
	if !preserveCompletedPhases {
		out.CompletedPhases = completedPhasesForState(out.CompletedPhases, secrets)
	} else {
		out.CompletedPhases = append([]string(nil), out.CompletedPhases...)
	}
	return &out
}

func completedPhasesForState(completed []string, runtimeSecrets map[string]RuntimeSecret) []string {
	if len(completed) == 0 || len(runtimeSecrets) == 0 {
		return append([]string(nil), completed...)
	}
	cut := len(completed)
	for _, secret := range runtimeSecrets {
		phase := strings.TrimSpace(secret.Phase)
		if phase == "" {
			continue
		}
		for idx, completedPhase := range completed {
			if completedPhase == phase && idx < cut {
				cut = idx
			}
		}
	}
	return append([]string(nil), completed[:cut]...)
}

func resetCompletedPhasesForMissingSecrets(st *State) {
	if st == nil || len(st.RuntimeSecrets) == 0 || len(st.CompletedPhases) == 0 {
		return
	}
	if st.Phase == "completed" && st.FailedPhase == "" {
		return
	}
	missingSecrets := map[string]RuntimeSecret{}
	for key, secret := range st.RuntimeSecrets {
		if _, ok := st.RuntimeVars[key]; !ok {
			missingSecrets[key] = secret
		}
	}
	if len(missingSecrets) == 0 {
		return
	}
	st.CompletedPhases = completedPhasesForState(st.CompletedPhases, missingSecrets)
	st.Phase = ""
	st.FailedPhase = ""
	st.Error = ""
}
