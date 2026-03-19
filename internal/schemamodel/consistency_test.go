package schemamodel

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/taedi90/deck/schemas"
)

func TestFileSchemaModelMatchesToolSchema(t *testing.T) {
	schema := loadToolSchemaMap(t, "file.schema.json")
	assertStructFieldsPresent(t, reflect.TypeOf(FileStepSpec{}), schemaAtPath(t, schema, "properties.spec.properties"))
	assertStructFieldsPresent(t, reflect.TypeOf(FileEditRule{}), schemaAtPath(t, schema, "properties.spec.properties.edits.items.properties"))
	assertStructFieldsPresent(t, reflect.TypeOf(FileSource{}), schemaAtPath(t, schema, "properties.spec.properties.source.properties"))
	assertStructFieldsPresent(t, reflect.TypeOf(FileBundleRef{}), schemaAtPath(t, schema, "properties.spec.properties.source.properties.bundle.properties"))
	assertStructFieldsPresent(t, reflect.TypeOf(FileOutputTarget{}), schemaAtPath(t, schema, "properties.spec.properties.output.properties"))
}

func TestWaitSchemaModelMatchesToolSchema(t *testing.T) {
	schema := loadToolSchemaMap(t, "wait.schema.json")
	assertStructFieldsPresent(t, reflect.TypeOf(WaitStepSpec{}), schemaAtPath(t, schema, "properties.spec.properties"))
}

func loadToolSchemaMap(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := schemas.ToolSchema(name)
	if err != nil {
		t.Fatalf("ToolSchema(%q): %v", name, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal schema %q: %v", name, err)
	}
	return out
}

func schemaAtPath(t *testing.T, root map[string]any, path string) map[string]any {
	t.Helper()
	current := any(root)
	for _, segment := range strings.Split(path, ".") {
		next, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("schema path %q missing segment %q", path, segment)
		}
		current, ok = next[segment]
		if !ok {
			t.Fatalf("schema path %q missing segment %q", path, segment)
		}
	}
	props, ok := current.(map[string]any)
	if !ok {
		t.Fatalf("schema path %q did not resolve to properties map", path)
	}
	return props
}

func assertStructFieldsPresent(t *testing.T, typ reflect.Type, properties map[string]any) {
	t.Helper()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		jsonTag := field.Tag.Get("json")
		name := strings.TrimSpace(strings.Split(jsonTag, ",")[0])
		if name == "" || name == "-" {
			continue
		}
		if _, ok := properties[name]; !ok {
			t.Fatalf("schema missing field %s for type %s", name, typ.Name())
		}
	}
}
