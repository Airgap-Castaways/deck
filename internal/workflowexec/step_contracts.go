package workflowexec

import "strings"

func normalizeStepKey(key StepTypeKey) StepTypeKey {
	return StepTypeKey{APIVersion: strings.TrimSpace(key.APIVersion), Kind: strings.TrimSpace(key.Kind)}
}

func StepSchemaFileForKey(key StepTypeKey) (string, bool, error) {
	def, ok, err := BuiltInTypeDefinitionForKey(normalizeStepKey(key))
	if err != nil {
		return "", false, err
	}
	if !ok || def.Step.SchemaFile == "" {
		return "", false, nil
	}
	return def.Step.SchemaFile, true, nil
}

func StepKinds() ([]string, error) {
	defs, err := StepDefinitions()
	if err != nil {
		return nil, err
	}
	kinds := make([]string, 0, len(defs))
	for _, def := range defs {
		kinds = append(kinds, def.Kind)
	}
	return kinds, nil
}

func StepAllowedForRoleForKey(role string, key StepTypeKey) (bool, error) {
	def, ok, err := BuiltInTypeDefinitionForKey(normalizeStepKey(key))
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	return containsString(def.Step.Roles, role), nil
}

func StepHasOutputForKey(key StepTypeKey, output string) (bool, error) {
	def, ok, err := BuiltInTypeDefinitionForKey(normalizeStepKey(key))
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	return containsString(def.Step.Outputs, output), nil
}

func StepKindsForRole(role string) []string {
	defs, err := StepDefinitions()
	if err != nil {
		return nil
	}
	out := make([]string, 0)
	for _, def := range defs {
		if containsString(def.Roles, role) {
			out = append(out, def.Kind)
		}
	}
	return out
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
