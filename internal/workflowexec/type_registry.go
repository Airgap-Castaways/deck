package workflowexec

import "github.com/Airgap-Castaways/deck/internal/workflowcontract"

type (
	// These aliases keep schema/documentation callers on the workflowexec facade
	// when they need runtime step keys and contract projections together.
	FieldDoc              = workflowcontract.FieldDoc
	ToolMetadata          = workflowcontract.ToolMetadata
	SchemaMetadata        = workflowcontract.SchemaMetadata
	BuiltInTypeDefinition = workflowcontract.BuiltInTypeDefinition
)

func BuiltInTypeDefinitions() ([]BuiltInTypeDefinition, error) {
	return workflowcontract.BuiltInTypeDefinitions()
}

func BuiltInTypeDefinitionsWith(toolBuilder func(StepDefinition) ToolMetadata, schemaBuilder func(StepDefinition) (SchemaMetadata, error)) ([]BuiltInTypeDefinition, error) {
	var tb func(workflowcontract.StepDefinition) workflowcontract.ToolMetadata
	if toolBuilder != nil {
		tb = func(def workflowcontract.StepDefinition) workflowcontract.ToolMetadata { return toolBuilder(def) }
	}
	var sb func(workflowcontract.StepDefinition) (workflowcontract.SchemaMetadata, error)
	if schemaBuilder != nil {
		sb = func(def workflowcontract.StepDefinition) (workflowcontract.SchemaMetadata, error) {
			return schemaBuilder(def)
		}
	}
	return workflowcontract.BuiltInTypeDefinitionsWith(tb, sb)
}

func BuiltInTypeDefinitionForKey(key StepTypeKey) (BuiltInTypeDefinition, bool, error) {
	return workflowcontract.BuiltInTypeDefinitionForKey(workflowcontract.StepTypeKey(key))
}

func BuiltInTypeDefinitionForKeyWith(key StepTypeKey, toolBuilder func(StepDefinition) ToolMetadata, schemaBuilder func(StepDefinition) (SchemaMetadata, error)) (BuiltInTypeDefinition, bool, error) {
	var tb func(workflowcontract.StepDefinition) workflowcontract.ToolMetadata
	if toolBuilder != nil {
		tb = func(def workflowcontract.StepDefinition) workflowcontract.ToolMetadata { return toolBuilder(def) }
	}
	var sb func(workflowcontract.StepDefinition) (workflowcontract.SchemaMetadata, error)
	if schemaBuilder != nil {
		sb = func(def workflowcontract.StepDefinition) (workflowcontract.SchemaMetadata, error) {
			return schemaBuilder(def)
		}
	}
	return workflowcontract.BuiltInTypeDefinitionForKeyWith(workflowcontract.StepTypeKey(key), tb, sb)
}
