package askir

import (
	"fmt"
	"strconv"
	"strings"
	"text/template/parse"

	"gopkg.in/yaml.v3"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

type workflowRender struct {
	Version string         `yaml:"version"`
	Vars    map[string]any `yaml:"vars,omitempty"`
	Phases  []phaseRender  `yaml:"phases,omitempty"`
	Steps   []stepRender   `yaml:"steps,omitempty"`
}

type phaseRender struct {
	Name           string         `yaml:"name"`
	MaxParallelism int            `yaml:"maxParallelism,omitempty"`
	Imports        []importRender `yaml:"imports,omitempty"`
	Steps          []stepRender   `yaml:"steps,omitempty"`
}

type importRender struct {
	Path string `yaml:"path"`
	When string `yaml:"when,omitempty"`
}

type stepRender struct {
	ID            string            `yaml:"id"`
	APIVersion    string            `yaml:"apiVersion,omitempty"`
	Kind          string            `yaml:"kind"`
	Metadata      map[string]any    `yaml:"metadata,omitempty"`
	When          string            `yaml:"when,omitempty"`
	ParallelGroup string            `yaml:"parallelGroup,omitempty"`
	Register      map[string]string `yaml:"register,omitempty"`
	Retry         int               `yaml:"retry,omitempty"`
	Timeout       string            `yaml:"timeout,omitempty"`
	Spec          map[string]any    `yaml:"spec"`
}

type componentRender struct {
	Steps []stepRender `yaml:"steps"`
}

func renderDocument(path string, doc askcontract.GeneratedDocument) (string, error) {
	switch documentKind(path, doc) {
	case "workflow":
		if doc.Workflow == nil {
			return "", fmt.Errorf("document %s is missing workflow content", path)
		}
		doc.Workflow = normalizeWorkflowDocument(doc.Workflow)
		return renderYAML(workflowFromIR(*doc.Workflow))
	case "component":
		if doc.Component == nil {
			return "", fmt.Errorf("document %s is missing component content", path)
		}
		doc.Component = normalizeComponentDocument(doc.Component)
		return renderYAML(componentFromIR(*doc.Component))
	case "vars":
		if doc.Vars == nil {
			return "", fmt.Errorf("document %s is missing vars content", path)
		}
		return renderYAML(unwrapVarsDocument(normalizeMapValues(doc.Vars)))
	default:
		return "", fmt.Errorf("document %s uses unsupported kind %q", path, doc.Kind)
	}
}

func normalizeWorkflowDocument(doc *askcontract.WorkflowDocument) *askcontract.WorkflowDocument {
	if doc == nil {
		return nil
	}
	copyDoc := *doc
	copyDoc.Vars = unwrapVarsDocument(normalizeMapValues(doc.Vars))
	copyDoc.Phases = make([]askcontract.WorkflowPhase, 0, len(doc.Phases))
	for _, phase := range doc.Phases {
		phaseCopy := phase
		phaseCopy.Steps = normalizeSteps(phase.Steps)
		copyDoc.Phases = append(copyDoc.Phases, phaseCopy)
	}
	copyDoc.Steps = normalizeSteps(doc.Steps)
	return &copyDoc
}

func normalizeComponentDocument(doc *askcontract.ComponentDocument) *askcontract.ComponentDocument {
	if doc == nil {
		return nil
	}
	copyDoc := *doc
	copyDoc.Steps = normalizeSteps(doc.Steps)
	return &copyDoc
}

func normalizeSteps(items []askcontract.WorkflowStep) []askcontract.WorkflowStep {
	out := make([]askcontract.WorkflowStep, 0, len(items))
	for _, item := range items {
		step := item
		step.When = normalizeTemplateAliases(step.When)
		step.Timeout = normalizeTemplateAliases(step.Timeout)
		step.Metadata = normalizeMapValues(step.Metadata)
		step.Spec = normalizeMapValues(step.Spec)
		out = append(out, step)
	}
	return out
}

func normalizeMapValues(values map[string]any) map[string]any {
	if len(values) == 0 {
		return values
	}
	out := map[string]any{}
	for key, value := range values {
		out[key] = normalizeValue(value)
	}
	return out
}

func normalizeSliceValues(values []any) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, normalizeValue(value))
	}
	return out
}

func normalizeValue(value any) any {
	switch typed := value.(type) {
	case string:
		return normalizeTemplateAliases(typed)
	case map[string]any:
		return normalizeMapValues(typed)
	case []any:
		return normalizeSliceValues(typed)
	default:
		return value
	}
}

func unwrapVarsDocument(values map[string]any) map[string]any {
	if len(values) != 1 {
		return values
	}
	nested, ok := values["vars"].(map[string]any)
	if !ok {
		return values
	}
	return normalizeMapValues(nested)
}

func normalizeTemplateAliases(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	var out strings.Builder
	for i := 0; i < len(input); {
		start, end, ok := nextTemplateAction(input, i)
		if !ok {
			out.WriteString(input[i:])
			break
		}
		out.WriteString(input[i:start])
		action := input[start : end+2]
		if normalized, ok := normalizeTemplateAction(action); ok {
			out.WriteString(normalized)
		} else {
			out.WriteString(action)
		}
		i = end + 2
	}
	return out.String()
}

type templateAliasSelector struct {
	Value   string
	Bracket bool
}

var templateAliasParseFuncs = []map[string]any{{
	"index":   true,
	"runtime": true,
	"vars":    true,
}}

func normalizeTemplateAction(action string) (string, bool) {
	trimmed := strings.TrimSpace(action)
	if trimmed == "" {
		return action, false
	}
	bodyStart := 2
	if strings.HasPrefix(trimmed, "${{") {
		bodyStart = 3
	}
	if !strings.HasSuffix(trimmed, "}}") || len(trimmed) < bodyStart+2 {
		return action, false
	}
	body := strings.TrimSpace(trimmed[bodyStart : len(trimmed)-2])
	canonical, ok := canonicalTemplateActionBody(body)
	if !ok {
		return action, false
	}
	normalized := "{{ " + canonical + " }}"
	return normalized, normalized != action
}

func canonicalTemplateActionBody(body string) (string, bool) {
	if action, ok := parseTemplateAction(body); ok {
		if canonical, ok := canonicalBodyFromParsedAction(action); ok {
			return canonical, true
		}
	}
	path, ok := parseTemplateAliasPath(body)
	if !ok {
		return "", false
	}
	if _, ok := parseTemplateAction(path.parseableExpression()); !ok {
		return "", false
	}
	return path.canonicalBody(), true
}

func parseTemplateAction(body string) (*parse.ActionNode, bool) {
	trees, err := parse.Parse("alias", "{{ "+body+" }}", "", "", templateAliasParseFuncs...)
	if err != nil {
		return nil, false
	}
	tree := trees["alias"]
	if tree == nil || tree.Root == nil || len(tree.Root.Nodes) != 1 {
		return nil, false
	}
	action, ok := tree.Root.Nodes[0].(*parse.ActionNode)
	return action, ok
}

func canonicalBodyFromParsedAction(action *parse.ActionNode) (string, bool) {
	if action == nil || action.Pipe == nil || len(action.Pipe.Decl) > 0 || len(action.Pipe.Cmds) != 1 {
		return "", false
	}
	cmd := action.Pipe.Cmds[0]
	if cmd == nil || len(cmd.Args) == 0 {
		return "", false
	}
	if len(cmd.Args) == 1 {
		namespace, selectors, ok := selectorsFromDirectArg(cmd.Args[0])
		if !ok {
			return "", false
		}
		return canonicalAliasBody(namespace, selectors), true
	}
	ident, ok := cmd.Args[0].(*parse.IdentifierNode)
	if !ok || ident.Ident != "index" {
		return "", false
	}
	namespace, selectors, ok := selectorsFromIndexArgs(cmd.Args[1:])
	if !ok {
		return "", false
	}
	return canonicalAliasBody(namespace, selectors), true
}

func selectorsFromDirectArg(arg parse.Node) (string, []templateAliasSelector, bool) {
	switch typed := arg.(type) {
	case *parse.FieldNode:
		if len(typed.Ident) < 2 || !isTemplateAliasNamespace(typed.Ident[0]) {
			return "", nil, false
		}
		return typed.Ident[0], fieldSelectors(typed.Ident[1:]), true
	case *parse.ChainNode:
		ident, ok := typed.Node.(*parse.IdentifierNode)
		if !ok || !isTemplateAliasNamespace(ident.Ident) || len(typed.Field) == 0 {
			return "", nil, false
		}
		return ident.Ident, fieldSelectors(typed.Field), true
	default:
		return "", nil, false
	}
}

func selectorsFromIndexArgs(args []parse.Node) (string, []templateAliasSelector, bool) {
	if len(args) < 2 {
		return "", nil, false
	}
	namespace, selectors, ok := selectorsFromIndexRoot(args[0])
	if !ok {
		return "", nil, false
	}
	for _, arg := range args[1:] {
		switch typed := arg.(type) {
		case *parse.StringNode:
			if isTemplateAliasSegment(typed.Text) {
				selectors = append(selectors, templateAliasSelector{Value: typed.Text})
				continue
			}
			selectors = append(selectors, templateAliasSelector{Value: strconv.Quote(typed.Text), Bracket: true})
		case *parse.NumberNode:
			if !typed.IsInt {
				return "", nil, false
			}
			selectors = append(selectors, templateAliasSelector{Value: strconv.FormatInt(typed.Int64, 10), Bracket: true})
		default:
			return "", nil, false
		}
	}
	return namespace, selectors, true
}

func selectorsFromIndexRoot(arg parse.Node) (string, []templateAliasSelector, bool) {
	switch typed := arg.(type) {
	case *parse.FieldNode:
		if len(typed.Ident) == 0 || !isTemplateAliasNamespace(typed.Ident[0]) {
			return "", nil, false
		}
		return typed.Ident[0], fieldSelectors(typed.Ident[1:]), true
	case *parse.ChainNode:
		ident, ok := typed.Node.(*parse.IdentifierNode)
		if !ok || !isTemplateAliasNamespace(ident.Ident) {
			return "", nil, false
		}
		return ident.Ident, fieldSelectors(typed.Field), true
	default:
		return "", nil, false
	}
}

func fieldSelectors(parts []string) []templateAliasSelector {
	selectors := make([]templateAliasSelector, 0, len(parts))
	for _, part := range parts {
		selectors = append(selectors, templateAliasSelector{Value: part})
	}
	return selectors
}

type templateAliasPath struct {
	Namespace string
	Selectors []templateAliasSelector
}

func parseTemplateAliasPath(body string) (templateAliasPath, bool) {
	body = strings.TrimSpace(body)
	if body == "" {
		return templateAliasPath{}, false
	}
	i := 0
	if body[i] == '.' {
		i++
	}
	namespace := ""
	switch {
	case strings.HasPrefix(body[i:], "vars"):
		namespace = "vars"
	case strings.HasPrefix(body[i:], "runtime"):
		namespace = "runtime"
	default:
		return templateAliasPath{}, false
	}
	i += len(namespace)
	if i >= len(body) || body[i] != '.' {
		return templateAliasPath{}, false
	}
	selectors := []templateAliasSelector{}
	for i < len(body) {
		if body[i] != '.' {
			return templateAliasPath{}, false
		}
		i++
		segmentLen := templateAliasSegmentLen(body[i:])
		if segmentLen == 0 {
			return templateAliasPath{}, false
		}
		selectors = append(selectors, templateAliasSelector{Value: body[i : i+segmentLen]})
		i += segmentLen
		for i < len(body) && body[i] == '[' {
			end := strings.IndexByte(body[i+1:], ']')
			if end < 0 {
				return templateAliasPath{}, false
			}
			selector := strings.TrimSpace(body[i+1 : i+1+end])
			if selector == "" {
				return templateAliasPath{}, false
			}
			selectors = append(selectors, templateAliasSelector{Value: selector, Bracket: true})
			i += end + 2
		}
	}
	if len(selectors) == 0 {
		return templateAliasPath{}, false
	}
	return templateAliasPath{Namespace: namespace, Selectors: selectors}, true
}

func (path templateAliasPath) canonicalBody() string {
	return canonicalAliasBody(path.Namespace, path.Selectors)
}

func (path templateAliasPath) parseableExpression() string {
	parts := []string{"index", "." + path.Namespace}
	for _, selector := range path.Selectors {
		if selector.Bracket {
			parts = append(parts, templateAliasBracketParseToken(selector.Value))
			continue
		}
		parts = append(parts, strconv.Quote(selector.Value))
	}
	return strings.Join(parts, " ")
}

func canonicalAliasBody(namespace string, selectors []templateAliasSelector) string {
	if usesCanonicalDotPath(selectors) {
		var out strings.Builder
		out.WriteByte('.')
		out.WriteString(namespace)
		for _, selector := range selectors {
			out.WriteByte('.')
			out.WriteString(selector.Value)
		}
		return out.String()
	}
	parts := []string{"index", "." + namespace}
	for _, selector := range selectors {
		parts = append(parts, templateAliasSelectorToken(selector))
	}
	return strings.Join(parts, " ")
}

func usesCanonicalDotPath(selectors []templateAliasSelector) bool {
	if len(selectors) == 0 {
		return false
	}
	for _, selector := range selectors {
		if selector.Bracket || !isCanonicalDotIdentifier(selector.Value) {
			return false
		}
	}
	return true
}

func templateAliasSelectorToken(selector templateAliasSelector) string {
	if selector.Bracket {
		return templateAliasBracketParseToken(selector.Value)
	}
	return strconv.Quote(selector.Value)
}

func isCanonicalDotIdentifier(value string) bool {
	if value == "" || !isTemplateAliasDotIdentStart(value[0]) {
		return false
	}
	for i := 1; i < len(value); i++ {
		if !isTemplateAliasDotIdentPart(value[i]) {
			return false
		}
	}
	return true
}

func templateAliasBracketParseToken(selector string) string {
	if isDecimalSelector(selector) || isQuotedTemplateAliasSelector(selector) {
		return selector
	}
	return strconv.Quote(selector)
}

func isTemplateAliasNamespace(value string) bool {
	return value == "vars" || value == "runtime"
}

func isTemplateAliasSegment(value string) bool {
	return templateAliasSegmentLen(value) == len(value)
}

func templateAliasSegmentLen(segment string) int {
	if segment == "" || !isTemplateAliasSegmentStart(segment[0]) {
		return 0
	}
	i := 1
	for i < len(segment) && isTemplateAliasSegmentPart(segment[i]) {
		i++
	}
	return i
}

func isTemplateAliasSegmentStart(ch byte) bool {
	return ch == '_' || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

func isTemplateAliasSegmentPart(ch byte) bool {
	return isTemplateAliasSegmentStart(ch) || ch == '-' || (ch >= '0' && ch <= '9')
}

func isTemplateAliasDotIdentStart(ch byte) bool {
	return ch == '_' || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

func isTemplateAliasDotIdentPart(ch byte) bool {
	return isTemplateAliasDotIdentStart(ch) || (ch >= '0' && ch <= '9')
}

func isQuotedTemplateAliasSelector(selector string) bool {
	if len(selector) < 2 {
		return false
	}
	quote := selector[0]
	return (quote == '\'' || quote == '"' || quote == '`') && selector[len(selector)-1] == quote
}

func isDecimalSelector(selector string) bool {
	for i := 0; i < len(selector); i++ {
		if selector[i] < '0' || selector[i] > '9' {
			return false
		}
	}
	return selector != ""
}

func nextTemplateAction(input string, offset int) (int, int, bool) {
	for i := offset; i < len(input)-1; i++ {
		if input[i] == '{' && input[i+1] == '{' {
			end := strings.Index(input[i+2:], "}}")
			if end < 0 {
				return 0, 0, false
			}
			return i, i + 2 + end, true
		}
		if i < len(input)-2 && input[i] == '$' && input[i+1] == '{' && input[i+2] == '{' {
			end := strings.Index(input[i+3:], "}}")
			if end < 0 {
				return 0, 0, false
			}
			return i, i + 3 + end, true
		}
	}
	return 0, 0, false
}

func workflowFromIR(doc askcontract.WorkflowDocument) workflowRender {
	out := workflowRender{Version: strings.TrimSpace(doc.Version)}
	if len(doc.Vars) > 0 {
		out.Vars = doc.Vars
	}
	if len(doc.Phases) > 0 {
		out.Phases = make([]phaseRender, 0, len(doc.Phases))
		for _, phase := range doc.Phases {
			out.Phases = append(out.Phases, phaseRender{
				Name:           phase.Name,
				MaxParallelism: phase.MaxParallelism,
				Imports:        importsFromIR(phase.Imports),
				Steps:          stepsFromIR(phase.Steps),
			})
		}
	}
	if len(doc.Steps) > 0 {
		out.Steps = stepsFromIR(doc.Steps)
	}
	return out
}

func componentFromIR(doc askcontract.ComponentDocument) componentRender {
	return componentRender{Steps: stepsFromIR(doc.Steps)}
}

func importsFromIR(items []askcontract.PhaseImport) []importRender {
	out := make([]importRender, 0, len(items))
	for _, item := range items {
		out = append(out, importRender{Path: item.Path, When: item.When})
	}
	return out
}

func stepsFromIR(items []askcontract.WorkflowStep) []stepRender {
	out := make([]stepRender, 0, len(items))
	for _, item := range items {
		out = append(out, stepRender{
			ID:            item.ID,
			APIVersion:    item.APIVersion,
			Kind:          item.Kind,
			Metadata:      item.Metadata,
			When:          item.When,
			ParallelGroup: item.ParallelGroup,
			Register:      item.Register,
			Retry:         item.Retry,
			Timeout:       item.Timeout,
			Spec:          item.Spec,
		})
	}
	return out
}

func renderYAML(doc any) (string, error) {
	raw, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshal document yaml: %w", err)
	}
	raw, err = quoteWholeValueTemplateScalars(raw)
	if err != nil {
		return "", err
	}
	return normalizeRenderedContent(raw), nil
}

func normalizeRenderedContent(raw []byte) string {
	trimmed := strings.TrimRight(string(raw), "\n")
	if trimmed == "" {
		return ""
	}
	return trimmed + "\n"
}

func quoteWholeValueTemplateScalars(raw []byte) ([]byte, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("parse rendered yaml for template quoting: %w", err)
	}
	markWholeValueTemplateScalars(&node)
	var out strings.Builder
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&node); err != nil {
		_ = encoder.Close()
		return nil, fmt.Errorf("encode rendered yaml with quoted templates: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close rendered yaml encoder: %w", err)
	}
	return []byte(out.String()), nil
}

func markWholeValueTemplateScalars(node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind == yaml.ScalarNode {
		if expr, ok := wholeValueTemplate(node.Value); ok {
			node.Value = expr
			node.Tag = "!!str"
			node.Style = yaml.DoubleQuotedStyle
		}
		return
	}
	for i := range node.Content {
		markWholeValueTemplateScalars(node.Content[i])
	}
}

func wholeValueTemplate(value string) (string, bool) {
	normalized := normalizeTemplateAliases(strings.TrimSpace(value))
	if normalized == "" {
		return "", false
	}
	start, end, ok := nextTemplateAction(normalized, 0)
	if !ok || start != 0 || end+2 != len(normalized) {
		return "", false
	}
	body := strings.TrimSpace(normalized[2 : len(normalized)-2])
	if _, ok := canonicalTemplateActionBody(body); ok {
		return normalized, true
	}
	return "", false
}
