package workflowcontract

import "maps"

type FieldDoc struct {
	Description string
	Example     string
}

type ToolMetadata struct {
	Kind      string
	Category  string
	Summary   string
	WhenToUse string
	Example   string
	FieldDocs map[string]FieldDoc
	Notes     []string
}

type SchemaMetadata struct {
	GeneratorName string
	SpecType      any
	Patch         func(root map[string]any)
}

func defaultToolMetadata(def StepDefinition) ToolMetadata {
	return ToolMetadata{
		Kind:      def.Kind,
		Category:  def.Category,
		Summary:   def.Summary,
		WhenToUse: def.WhenToUse,
	}
}

func defaultSchemaMetadata(def StepDefinition) (SchemaMetadata, error) {
	return SchemaMetadata{GeneratorName: def.ToolSchemaGenerator}, nil
}

type BuiltInTypeDefinition struct {
	Key    StepTypeKey
	Step   StepDefinition
	Docs   ToolMetadata
	Schema SchemaMetadata
}

func BuiltInTypeDefinitions() ([]BuiltInTypeDefinition, error) {
	return BuiltInTypeDefinitionsWith(defaultToolMetadata, defaultSchemaMetadata)
}

func BuiltInTypeDefinitionsWith(toolBuilder func(StepDefinition) ToolMetadata, schemaBuilder func(StepDefinition) (SchemaMetadata, error)) ([]BuiltInTypeDefinition, error) {
	defs, err := StepDefinitions()
	if err != nil {
		return nil, err
	}
	out := make([]BuiltInTypeDefinition, 0, len(defs))
	if toolBuilder == nil {
		toolBuilder = defaultToolMetadata
	}
	if schemaBuilder == nil {
		schemaBuilder = defaultSchemaMetadata
	}
	for _, def := range defs {
		key := StepTypeKey{APIVersion: def.APIVersion, Kind: def.Kind}
		schema, err := schemaBuilder(def)
		if err != nil {
			return nil, err
		}
		out = append(out, BuiltInTypeDefinition{
			Key:    key,
			Step:   def,
			Docs:   cloneToolMetadata(toolBuilder(def)),
			Schema: schema,
		})
	}
	return out, nil
}

func BuiltInTypeDefinitionForKey(key StepTypeKey) (BuiltInTypeDefinition, bool, error) {
	return BuiltInTypeDefinitionForKeyWith(key, defaultToolMetadata, defaultSchemaMetadata)
}

func BuiltInTypeDefinitionForKeyWith(key StepTypeKey, toolBuilder func(StepDefinition) ToolMetadata, schemaBuilder func(StepDefinition) (SchemaMetadata, error)) (BuiltInTypeDefinition, bool, error) {
	def, ok, err := StepDefinitionForKey(key)
	if err != nil {
		return BuiltInTypeDefinition{}, false, err
	}
	if !ok {
		return BuiltInTypeDefinition{}, false, nil
	}
	if toolBuilder == nil {
		toolBuilder = defaultToolMetadata
	}
	if schemaBuilder == nil {
		schemaBuilder = defaultSchemaMetadata
	}
	schema, err := schemaBuilder(def)
	if err != nil {
		return BuiltInTypeDefinition{}, false, err
	}
	return BuiltInTypeDefinition{Key: key, Step: def, Docs: cloneToolMetadata(toolBuilder(def)), Schema: schema}, true, nil
}

func cloneToolMetadata(meta ToolMetadata) ToolMetadata {
	cloned := meta
	if meta.FieldDocs != nil {
		cloned.FieldDocs = maps.Clone(meta.FieldDocs)
	}
	cloned.Notes = append([]string(nil), meta.Notes...)
	return cloned
}
