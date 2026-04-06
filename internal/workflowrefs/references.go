package workflowrefs

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"text/template"
	"text/template/parse"

	"github.com/google/cel-go/cel"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

const (
	NamespaceVars    = "vars"
	NamespaceRuntime = "runtime"
)

type Reference struct {
	Namespace string
	Path      string
	Root      string
}

var (
	whenEnvOnce sync.Once
	whenEnvInst *cel.Env
	errWhenEnv  error
)

func TemplateReferences(input string) ([]Reference, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, nil
	}
	refs := newReferenceSet()
	collectDirectTemplateActionRefs(trimmed, refs)
	tmpl, err := template.New("refs").Parse(normalizeTemplateAliasesForParse(trimmed))
	if err != nil {
		return nil, fmt.Errorf("parse template references: %w", err)
	}
	collectTemplateNodeRefs(tmpl.Root, refs)
	return refs.sorted(), nil
}

func ValueTemplateReferences(value any) ([]Reference, error) {
	refs := newReferenceSet()
	if err := collectValueTemplateRefs(value, refs); err != nil {
		return nil, err
	}
	return refs.sorted(), nil
}

func WhenReferences(expr string) ([]Reference, error) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return nil, nil
	}
	env, err := whenEnv()
	if err != nil {
		return nil, fmt.Errorf("create CEL env: %w", err)
	}
	ast, issues := env.Parse(trimmed)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}
	parsed, err := cel.AstToParsedExpr(ast)
	if err != nil {
		return nil, fmt.Errorf("convert CEL ast: %w", err)
	}
	refs := newReferenceSet()
	collectCELRefs(parsed.GetExpr(), refs)
	return refs.sorted(), nil
}

func whenEnv() (*cel.Env, error) {
	whenEnvOnce.Do(func() {
		whenEnvInst, errWhenEnv = cel.NewEnv(
			cel.Variable(NamespaceVars, cel.MapType(cel.StringType, cel.DynType)),
			cel.Variable(NamespaceRuntime, cel.MapType(cel.StringType, cel.DynType)),
		)
	})
	return whenEnvInst, errWhenEnv
}

type referenceSet struct {
	items map[string]Reference
}

func newReferenceSet() *referenceSet {
	return &referenceSet{items: map[string]Reference{}}
}

func (s *referenceSet) add(namespace, path string) {
	namespace = strings.TrimSpace(namespace)
	path = strings.TrimSpace(path)
	if namespace == "" || path == "" {
		return
	}
	root := path
	if idx := strings.IndexAny(path, ".["); idx > 0 {
		root = path[:idx]
	}
	if root == "" {
		return
	}
	key := namespace + "\n" + path
	s.items[key] = Reference{Namespace: namespace, Path: path, Root: root}
	rootKey := namespace + "\n" + root
	s.items[rootKey] = Reference{Namespace: namespace, Path: root, Root: root}
}

func (s *referenceSet) sorted() []Reference {
	if len(s.items) == 0 {
		return nil
	}
	out := make([]Reference, 0, len(s.items))
	for _, ref := range s.items {
		out = append(out, ref)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Root < out[j].Root
	})
	return out
}

func collectValueTemplateRefs(value any, refs *referenceSet) error {
	switch typed := value.(type) {
	case string:
		found, err := TemplateReferences(typed)
		if err != nil {
			return err
		}
		for _, ref := range found {
			refs.add(ref.Namespace, ref.Path)
		}
	case map[string]any:
		for _, item := range typed {
			if err := collectValueTemplateRefs(item, refs); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range typed {
			if err := collectValueTemplateRefs(item, refs); err != nil {
				return err
			}
		}
	}
	return nil
}

func collectTemplateNodeRefs(node parse.Node, refs *referenceSet) {
	switch typed := node.(type) {
	case *parse.ListNode:
		if typed == nil {
			return
		}
		for _, child := range typed.Nodes {
			collectTemplateNodeRefs(child, refs)
		}
	case *parse.ActionNode:
		collectTemplatePipeRefs(typed.Pipe, refs)
	case *parse.IfNode:
		collectTemplatePipeRefs(typed.Pipe, refs)
		collectTemplateNodeRefs(typed.List, refs)
		collectTemplateNodeRefs(typed.ElseList, refs)
	case *parse.RangeNode:
		collectTemplatePipeRefs(typed.Pipe, refs)
		collectTemplateNodeRefs(typed.List, refs)
		collectTemplateNodeRefs(typed.ElseList, refs)
	case *parse.WithNode:
		collectTemplatePipeRefs(typed.Pipe, refs)
		collectTemplateNodeRefs(typed.List, refs)
		collectTemplateNodeRefs(typed.ElseList, refs)
	case *parse.TemplateNode:
		collectTemplatePipeRefs(typed.Pipe, refs)
	}
}

func collectTemplatePipeRefs(pipe *parse.PipeNode, refs *referenceSet) {
	if pipe == nil {
		return
	}
	for _, cmd := range pipe.Cmds {
		collectTemplateCommandRefs(cmd, refs)
	}
}

func collectTemplateCommandRefs(cmd *parse.CommandNode, refs *referenceSet) {
	if cmd == nil {
		return
	}
	for _, arg := range cmd.Args {
		collectTemplateArgRefs(arg, refs)
	}
}

func collectTemplateArgRefs(node parse.Node, refs *referenceSet) {
	switch typed := node.(type) {
	case *parse.FieldNode:
		if len(typed.Ident) >= 2 && isNamespace(typed.Ident[0]) {
			refs.add(typed.Ident[0], strings.Join(typed.Ident[1:], "."))
		}
	case *parse.VariableNode:
		if len(typed.Ident) >= 3 && typed.Ident[0] == "$" && isNamespace(typed.Ident[1]) {
			refs.add(typed.Ident[1], strings.Join(typed.Ident[2:], "."))
		}
	case *parse.ChainNode:
		collectTemplateArgRefs(typed.Node, refs)
	case *parse.PipeNode:
		collectTemplatePipeRefs(typed, refs)
	}
}

func collectCELRefs(expr *exprpb.Expr, refs *referenceSet) {
	if expr == nil {
		return
	}
	switch typed := expr.ExprKind.(type) {
	case *exprpb.Expr_SelectExpr:
		if namespace, path, ok := celSelectPath(typed.SelectExpr); ok {
			refs.add(namespace, path)
		}
		collectCELRefs(typed.SelectExpr.Operand, refs)
	case *exprpb.Expr_CallExpr:
		if typed.CallExpr.Target != nil {
			collectCELRefs(typed.CallExpr.Target, refs)
		}
		for _, arg := range typed.CallExpr.Args {
			collectCELRefs(arg, refs)
		}
	case *exprpb.Expr_ListExpr:
		for _, elem := range typed.ListExpr.Elements {
			collectCELRefs(elem, refs)
		}
	case *exprpb.Expr_StructExpr:
		for _, entry := range typed.StructExpr.Entries {
			collectCELRefs(entry.GetValue(), refs)
			collectCELRefs(entry.GetMapKey(), refs)
		}
	case *exprpb.Expr_ComprehensionExpr:
		collectCELRefs(typed.ComprehensionExpr.IterRange, refs)
		collectCELRefs(typed.ComprehensionExpr.AccuInit, refs)
		collectCELRefs(typed.ComprehensionExpr.LoopCondition, refs)
		collectCELRefs(typed.ComprehensionExpr.LoopStep, refs)
		collectCELRefs(typed.ComprehensionExpr.Result, refs)
	}
}

func celSelectPath(sel *exprpb.Expr_Select) (string, string, bool) {
	if sel == nil {
		return "", "", false
	}
	parts := []string{sel.Field}
	current := sel.Operand
	for current != nil {
		switch typed := current.ExprKind.(type) {
		case *exprpb.Expr_SelectExpr:
			parts = append([]string{typed.SelectExpr.Field}, parts...)
			current = typed.SelectExpr.Operand
		case *exprpb.Expr_IdentExpr:
			if !isNamespace(typed.IdentExpr.Name) {
				return "", "", false
			}
			return typed.IdentExpr.Name, strings.Join(parts, "."), true
		default:
			return "", "", false
		}
	}
	return "", "", false
}

func isNamespace(name string) bool {
	return name == NamespaceVars || name == NamespaceRuntime
}

func normalizeTemplateAliasesForParse(input string) string {
	var out strings.Builder
	for i := 0; i < len(input); {
		start, end, ok := nextTemplateAction(input, i)
		if !ok {
			out.WriteString(input[i:])
			break
		}
		prefixEnd := start
		if start > i && input[start-1] == '$' {
			prefixEnd = start - 1
		}
		out.WriteString(input[i:prefixEnd])
		body := input[start+2 : end]
		out.WriteString("{{")
		out.WriteString(rewriteTemplateActionBody(body))
		out.WriteString("}}")
		i = end + 2
	}
	return out.String()
}

func collectDirectTemplateActionRefs(input string, refs *referenceSet) {
	for i := 0; i < len(input); {
		start, end, ok := nextTemplateAction(input, i)
		if !ok {
			return
		}
		collectTemplateActionBodyRefs(input[start+2:end], refs)
		i = end + 2
	}
}

func collectTemplateActionBodyRefs(body string, refs *referenceSet) {
	forEachTemplateActionReference(body, func(_ int, _ int, namespace, path string, _ bool) {
		refs.add(namespace, path)
	})
}

func rewriteTemplateActionBody(body string) string {
	var out strings.Builder
	last := 0
	forEachTemplateActionReference(body, func(start, end int, namespace, path string, hasDot bool) {
		out.WriteString(body[last:start])
		rewritten := normalizedTemplateReference(namespace, path, hasDot)
		if rewritten == "" {
			if !hasDot {
				out.WriteByte('.')
			}
			out.WriteString(body[start:end])
		} else {
			out.WriteString(rewritten)
		}
		last = end
	})
	if last == 0 {
		return body
	}
	out.WriteString(body[last:])
	return out.String()
}

func normalizedTemplateReference(namespace, path string, hasDot bool) string {
	if strings.Contains(path, "[") {
		selectors, ok := referencePathSelectors(path)
		if !ok {
			return ""
		}
		parts := make([]string, 0, len(selectors)+2)
		parts = append(parts, "index", "."+namespace)
		parts = append(parts, selectors...)
		return strings.Join(parts, " ")
	}
	if hasDot {
		return "." + namespace + "." + path
	}
	return "." + namespace + "." + path
}

func forEachTemplateActionReference(body string, visit func(start, end int, namespace, path string, hasDot bool)) {
	for i := 0; i < len(body); {
		if quote, end, ok := quotedTemplateString(body, i); ok {
			i = end
			_ = quote
			continue
		}
		namespace, path, end, hasDot, ok := templateActionReferenceAt(body, i)
		if !ok {
			i++
			continue
		}
		visit(i, end, namespace, path, hasDot)
		i = end
	}
}

func quotedTemplateString(body string, start int) (byte, int, bool) {
	if start >= len(body) {
		return 0, 0, false
	}
	quote := body[start]
	if quote != '\'' && quote != '"' && quote != '`' {
		return 0, 0, false
	}
	for i := start + 1; i < len(body); i++ {
		if quote != '`' && body[i] == '\\' {
			i++
			continue
		}
		if body[i] == quote {
			return quote, i + 1, true
		}
	}
	return quote, len(body), true
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
			return i + 1, i + 3 + end, true
		}
	}
	return 0, 0, false
}

func templateActionReferenceAt(body string, start int) (string, string, int, bool, bool) {
	if start > 0 && !isTemplateReferenceBoundary(body[start-1]) {
		return "", "", 0, false, false
	}
	hasDot := false
	pos := start
	if body[pos] == '.' {
		hasDot = true
		pos++
		if start > 0 && !isTemplateReferenceBoundary(body[start-1]) {
			return "", "", 0, false, false
		}
	}
	namespace := ""
	switch {
	case strings.HasPrefix(body[pos:], NamespaceVars+"."):
		namespace = NamespaceVars
	case strings.HasPrefix(body[pos:], NamespaceRuntime+"."):
		namespace = NamespaceRuntime
	default:
		return "", "", 0, false, false
	}
	pos += len(namespace) + 1
	pathLen := referencePathLen(body[pos:])
	if pathLen == 0 {
		return "", "", 0, false, false
	}
	return namespace, body[pos : pos+pathLen], pos + pathLen, hasDot, true
}

func isTemplateReferenceBoundary(ch byte) bool {
	return !isReferenceIdentPart(ch) && ch != '.' && ch != ']'
}

func referencePathLen(path string) int {
	if path == "" {
		return 0
	}
	i := 0
	for {
		segmentLen := referenceSegmentLen(path[i:])
		if segmentLen == 0 {
			return 0
		}
		i += segmentLen
		for i < len(path) && path[i] == '[' {
			end := strings.IndexByte(path[i+1:], ']')
			if end < 0 {
				return 0
			}
			i += end + 2
		}
		if i >= len(path) || path[i] != '.' {
			break
		}
		i++
		if i >= len(path) {
			return 0
		}
	}
	return i
}

func referencePathSelectors(path string) ([]string, bool) {
	if path == "" {
		return nil, false
	}
	selectors := []string{}
	i := 0
	for {
		segmentLen := referenceSegmentLen(path[i:])
		if segmentLen == 0 {
			return nil, false
		}
		selectors = append(selectors, fmt.Sprintf("%q", path[i:i+segmentLen]))
		i += segmentLen
		for i < len(path) && path[i] == '[' {
			end := strings.IndexByte(path[i+1:], ']')
			if end < 0 {
				return nil, false
			}
			selector := strings.TrimSpace(path[i+1 : i+1+end])
			if selector == "" {
				return nil, false
			}
			if isQuotedTemplateSelector(selector) || isDecimalSelector(selector) {
				selectors = append(selectors, selector)
			} else {
				selectors = append(selectors, fmt.Sprintf("%q", selector))
			}
			i += end + 2
		}
		if i >= len(path) {
			break
		}
		if path[i] != '.' {
			return nil, false
		}
		i++
		if i >= len(path) {
			return nil, false
		}
	}
	return selectors, true
}

func isQuotedTemplateSelector(selector string) bool {
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

func referenceSegmentLen(segment string) int {
	if segment == "" || !isReferenceIdentStart(segment[0]) {
		return 0
	}
	i := 1
	for i < len(segment) && isReferenceIdentPart(segment[i]) {
		i++
	}
	return i
}

func isReferenceIdentStart(ch byte) bool {
	return ch == '_' || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

func isReferenceIdentPart(ch byte) bool {
	return isReferenceIdentStart(ch) || ch == '-' || (ch >= '0' && ch <= '9')
}
