package workflowexec

import "strings"

type StepContract struct {
	SchemaFile string
	Roles      map[string]bool
	Outputs    map[string]bool
}

func stepContracts() map[StepTypeKey]StepContract {
	contracts := make(map[StepTypeKey]StepContract, len(StepDefinitions()))
	for _, def := range StepDefinitions() {
		contracts[StepTypeKey{APIVersion: def.APIVersion, Kind: def.Kind}] = StepContract{
			SchemaFile: def.SchemaFile,
			Roles:      setOf(def.Roles...),
			Outputs:    setOf(def.Outputs...),
		}
	}
	return contracts
}

func normalizeStepKey(key StepTypeKey) StepTypeKey {
	return StepTypeKey{APIVersion: strings.TrimSpace(key.APIVersion), Kind: strings.TrimSpace(key.Kind)}
}

func StepSchemaFileForKey(key StepTypeKey) (string, bool) {
	contract, ok := stepContracts()[normalizeStepKey(key)]
	if !ok || contract.SchemaFile == "" {
		return "", false
	}
	return contract.SchemaFile, true
}

func StepContractForKey(key StepTypeKey) (StepContract, bool) {
	contract, ok := stepContracts()[normalizeStepKey(key)]
	return contract, ok
}

func StepKinds() []string {
	defs := StepDefinitions()
	kinds := make([]string, 0, len(defs))
	for _, def := range defs {
		kinds = append(kinds, def.Kind)
	}
	return kinds
}

func StepAllowedForRoleForKey(role string, key StepTypeKey) bool {
	contract, ok := StepContractForKey(key)
	if !ok {
		return false
	}
	return contract.Roles[role]
}

func StepHasOutputForKey(key StepTypeKey, output string) bool {
	contract, ok := StepContractForKey(key)
	if !ok {
		return false
	}
	return contract.Outputs[output]
}

func setOf(values ...string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}
