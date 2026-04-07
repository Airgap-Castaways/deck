package stepmeta

import (
	"fmt"
	"io/fs"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
)

type Definition struct {
	Kind        string
	Family      string
	FamilyTitle string
	Category    string
	Group       string
	GroupOrder  int
	DocsPage    string
	DocsOrder   int
	Visibility  string
	Roles       []string
	Outputs     []string
	SchemaFile  string
	SchemaPatch func(root map[string]any)
	Summary     string
	WhenToUse   string
	Example     string
	Notes       []string
	Ask         AskMetadata
}

type AskMetadata struct {
	Capabilities             []string
	ContractHints            ContractHints
	Builders                 []AuthoringBuilder
	MatchSignals             []string
	KeyFields                []string
	ValidationHints          []ValidationHint
	ConstrainedLiteralFields []ConstrainedLiteralField
	QualityRules             []QualityRule
	AntiSignals              []string
}

type AuthoringBuilder struct {
	ID                   string
	Phase                string
	DefaultStepID        string
	Summary              string
	RequiresCapabilities []string
	Bindings             []AuthoringBinding
}

type AuthoringBinding struct {
	Path     string
	From     string
	Semantic string
	Required bool
}

type ContractHints struct {
	ProducesArtifacts   []string
	ConsumesArtifacts   []string
	PublishesState      []string
	ConsumesState       []string
	RoleSensitive       bool
	VerificationRelated bool
}

type ValidationHint struct {
	ErrorContains string
	Fix           string
}

type ConstrainedLiteralField struct {
	Path          string
	AllowedValues []string
	Guidance      string
}

type QualityRule struct {
	Trigger string
	Message string
	Level   string
}

type FieldDoc struct {
	Path        string
	Description string
	Example     string
	Required    bool
	Hidden      bool
	Source      SourceRef
}

type Docs struct {
	Summary   string
	WhenToUse string
	Example   string
	Notes     []string
	Fields    []FieldDoc
	Source    SourceRef
}

type Schema struct {
	SpecType any
	Patch    func(root map[string]any)
	Source   SourceRef
}

type SourceRef struct {
	File string
	Line int
}

type Entry struct {
	Definition Definition
	TypeName   string
	Docs       Docs
	Schema     Schema
}

var (
	mu         sync.RWMutex
	entries    = map[string]registeredDef{}
	stepspecFS fs.FS
)

type registeredDef struct {
	Definition Definition
	TypeName   string
	Type       reflect.Type
	Schema     registeredSchema
}

type registeredSchema struct {
	Patch  func(root map[string]any)
	Source SourceRef
}

func MustRegister[T any](def Definition) struct{} {
	kind := strings.TrimSpace(def.Kind)
	if kind == "" {
		panic("stepmeta: kind is required")
	}
	typeName := typeNameFor[T]()
	if typeName == "" {
		panic(fmt.Sprintf("stepmeta: could not resolve type name for %s", kind))
	}
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil {
		panic(fmt.Sprintf("stepmeta: could not resolve reflect type for %s", kind))
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("stepmeta: registered type for %s must be a struct", kind))
	}
	mu.Lock()
	defer mu.Unlock()
	if _, exists := entries[kind]; exists {
		panic(fmt.Sprintf("stepmeta: duplicate registration for %s", kind))
	}
	registered := registeredDef{Definition: def, TypeName: typeName, Type: t}
	if def.SchemaPatch != nil {
		file, line := callerSource()
		registered.Schema = registeredSchema{Patch: def.SchemaPatch, Source: SourceRef{File: file, Line: line}}
	}
	entries[kind] = registered
	return struct{}{}
}

func RegisteredKinds() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(entries))
	for kind := range entries {
		out = append(out, kind)
	}
	sort.Strings(out)
	return out
}

func Lookup(kind string) (Entry, bool, error) {
	mu.RLock()
	registered, ok := entries[strings.TrimSpace(kind)]
	mu.RUnlock()
	if !ok {
		return Entry{}, false, nil
	}
	typeName := registered.TypeName
	docs, err := buildDocs(registered.Type)
	if err != nil {
		return Entry{}, true, err
	}
	entry := Entry{Definition: cloneDefinition(registered.Definition), TypeName: typeName, Docs: docs}
	if registered.Schema.Patch != nil {
		entry.Schema = Schema{SpecType: reflect.New(registered.Type).Interface(), Patch: registered.Schema.Patch, Source: registered.Schema.Source}
	}
	entry.Docs = mergeDefinitionDocs(entry.Definition, entry.Docs)
	if err := validateEntry(entry); err != nil {
		return Entry{}, true, err
	}
	return entry, true, nil
}

func typeNameFor[T any]() string {
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil {
		return ""
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}

func cloneDefinition(def Definition) Definition {
	cloned := def
	cloned.Category = strings.TrimSpace(def.Category)
	cloned.Roles = append([]string(nil), def.Roles...)
	cloned.Outputs = append([]string(nil), def.Outputs...)
	cloned.Notes = append([]string(nil), def.Notes...)
	cloned.Ask.Capabilities = append([]string(nil), def.Ask.Capabilities...)
	cloned.Ask.ContractHints.ProducesArtifacts = append([]string(nil), def.Ask.ContractHints.ProducesArtifacts...)
	cloned.Ask.ContractHints.ConsumesArtifacts = append([]string(nil), def.Ask.ContractHints.ConsumesArtifacts...)
	cloned.Ask.ContractHints.PublishesState = append([]string(nil), def.Ask.ContractHints.PublishesState...)
	cloned.Ask.ContractHints.ConsumesState = append([]string(nil), def.Ask.ContractHints.ConsumesState...)
	if len(def.Ask.Builders) > 0 {
		cloned.Ask.Builders = make([]AuthoringBuilder, len(def.Ask.Builders))
		copy(cloned.Ask.Builders, def.Ask.Builders)
		for i := range cloned.Ask.Builders {
			cloned.Ask.Builders[i].RequiresCapabilities = append([]string(nil), def.Ask.Builders[i].RequiresCapabilities...)
			if len(def.Ask.Builders[i].Bindings) > 0 {
				cloned.Ask.Builders[i].Bindings = make([]AuthoringBinding, len(def.Ask.Builders[i].Bindings))
				copy(cloned.Ask.Builders[i].Bindings, def.Ask.Builders[i].Bindings)
			}
		}
	}
	cloned.Ask.MatchSignals = append([]string(nil), def.Ask.MatchSignals...)
	cloned.Ask.KeyFields = append([]string(nil), def.Ask.KeyFields...)
	cloned.Ask.AntiSignals = append([]string(nil), def.Ask.AntiSignals...)
	cloned.Ask.ValidationHints = append([]ValidationHint(nil), def.Ask.ValidationHints...)
	cloned.Ask.QualityRules = append([]QualityRule(nil), def.Ask.QualityRules...)
	if len(def.Ask.ConstrainedLiteralFields) > 0 {
		cloned.Ask.ConstrainedLiteralFields = make([]ConstrainedLiteralField, len(def.Ask.ConstrainedLiteralFields))
		copy(cloned.Ask.ConstrainedLiteralFields, def.Ask.ConstrainedLiteralFields)
		for i := range cloned.Ask.ConstrainedLiteralFields {
			cloned.Ask.ConstrainedLiteralFields[i].AllowedValues = append([]string(nil), def.Ask.ConstrainedLiteralFields[i].AllowedValues...)
		}
	}
	return cloned
}

func mergeDefinitionDocs(def Definition, docs Docs) Docs {
	merged := docs
	if strings.TrimSpace(def.Summary) != "" {
		merged.Summary = strings.TrimSpace(def.Summary)
	}
	if strings.TrimSpace(def.WhenToUse) != "" {
		merged.WhenToUse = strings.TrimSpace(def.WhenToUse)
	}
	if strings.TrimSpace(def.Example) != "" {
		merged.Example = strings.TrimSpace(def.Example)
	}
	if len(def.Notes) > 0 {
		merged.Notes = append([]string(nil), def.Notes...)
	}
	return merged
}

func RegisterSourceFS(source fs.FS) struct{} {
	if source == nil {
		panic("stepmeta: source fs is nil")
	}
	mu.Lock()
	defer mu.Unlock()
	stepspecFS = source
	return struct{}{}
}

func callerSource() (string, int) {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return "", 0
	}
	return file, line
}
