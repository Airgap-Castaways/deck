package workflowrefs

import (
	"fmt"
	"sort"
	"strings"
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

func TemplateReferences(input string) []Reference {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil
	}
	refs := newReferenceSet()
	collectDirectTemplateActionRefs(trimmed, refs)
	tmpl, err := template.New("refs").Parse(normalizeTemplateAliasesForParse(trimmed))
	if err == nil {
		collectTemplateNodeRefs(tmpl.Root, refs)
	}
	return refs.sorted()
}

func ValueTemplateReferences(value any) []Reference {
	refs := newReferenceSet()
	collectValueTemplateRefs(value, refs)
	return refs.sorted()
}

func WhenReferences(expr string) ([]Reference, error) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return nil, nil
	}
	env, err := cel.NewEnv(
		cel.Variable(NamespaceVars, cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable(NamespaceRuntime, cel.MapType(cel.StringType, cel.DynType)),
	)
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

func collectValueTemplateRefs(value any, refs *referenceSet) {
	switch typed := value.(type) {
	case string:
		for _, ref := range TemplateReferences(typed) {
			refs.add(ref.Namespace, ref.Path)
		}
	case map[string]any:
		for _, item := range typed {
			collectValueTemplateRefs(item, refs)
		}
	case []any:
		for _, item := range typed {
			collectValueTemplateRefs(item, refs)
		}
	}
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
		out.WriteString(normalizeTemplateActionBody(body))
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
		body := input[start+2 : end]
		if namespace, path, ok := namespacePath(body); ok {
			refs.add(namespace, path)
		}
		i = end + 2
	}
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

func normalizeTemplateActionBody(body string) string {
	trimmed := strings.TrimSpace(body)
	namespace, path, ok := namespacePath(trimmed)
	if !ok {
		return body
	}
	return " ." + namespace + "." + path + " "
}

func namespacePath(input string) (string, string, bool) {
	trimmed := strings.TrimSpace(input)
	trimmed = strings.TrimPrefix(trimmed, ".")
	for _, namespace := range []string{NamespaceVars, NamespaceRuntime} {
		prefix := namespace + "."
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		path := strings.TrimPrefix(trimmed, prefix)
		if validReferencePath(path) {
			return namespace, path, true
		}
	}
	return "", "", false
}

func validReferencePath(path string) bool {
	if path == "" {
		return false
	}
	for i := 0; i < len(path); {
		if !isReferenceIdentStart(path[i]) {
			return false
		}
		i++
		for i < len(path) && isReferenceIdentPart(path[i]) {
			i++
		}
		for i < len(path) && path[i] == '[' {
			end := strings.IndexByte(path[i+1:], ']')
			if end < 0 {
				return false
			}
			i += end + 2
		}
		if i == len(path) {
			return true
		}
		if path[i] != '.' {
			return false
		}
		i++
		if i == len(path) {
			return false
		}
	}
	return true
}

func isReferenceIdentStart(ch byte) bool {
	return ch == '_' || ch == '-' || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

func isReferenceIdentPart(ch byte) bool {
	return isReferenceIdentStart(ch) || (ch >= '0' && ch <= '9')
}
