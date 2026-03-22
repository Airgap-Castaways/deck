package workflowexec

type BuiltInTypeDefinition struct {
	Key      StepTypeKey
	Step     StepDefinition
	Contract StepContract
}

func BuiltInTypeDefinitions() []BuiltInTypeDefinition {
	defs := StepDefinitions()
	out := make([]BuiltInTypeDefinition, 0, len(defs))
	for _, def := range defs {
		key := StepTypeKey{APIVersion: def.APIVersion, Kind: def.Kind}
		contract, _ := StepContractForKey(key)
		out = append(out, BuiltInTypeDefinition{
			Key:      key,
			Step:     def,
			Contract: contract,
		})
	}
	return out
}

func BuiltInTypeDefinitionForKey(key StepTypeKey) (BuiltInTypeDefinition, bool) {
	def, ok := StepDefinitionForKey(key)
	if !ok {
		return BuiltInTypeDefinition{}, false
	}
	contract, _ := StepContractForKey(key)
	return BuiltInTypeDefinition{Key: key, Step: def, Contract: contract}, true
}
