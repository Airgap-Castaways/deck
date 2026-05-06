package workflowexec

import (
	"fmt"
	"sort"
	"strings"
)

func MustStepRoleHandlers[T any](role string, handlers map[string]T) map[string]T {
	registered, err := StepRoleHandlers(role, handlers)
	if err != nil {
		panic(err)
	}
	return registered
}

func StepRoleHandlers[T any](role string, handlers map[string]T) (map[string]T, error) {
	role = strings.TrimSpace(role)
	if role == "" {
		return nil, fmt.Errorf("step role is required")
	}
	defs, err := StepDefinitions()
	if err != nil {
		return nil, err
	}
	required := map[string]struct{}{}
	rolesByKind := map[string][]string{}
	for _, def := range defs {
		rolesByKind[def.Kind] = def.Roles
		if containsString(def.Roles, role) {
			required[def.Kind] = struct{}{}
		}
	}
	if len(required) == 0 {
		return nil, fmt.Errorf("no step definitions registered for role %q", role)
	}

	registered := make(map[string]T, len(handlers))
	for rawKind, handler := range handlers {
		kind := strings.TrimSpace(rawKind)
		if kind == "" {
			return nil, fmt.Errorf("step handler kind is required for role %q", role)
		}
		roles, ok := rolesByKind[kind]
		if !ok {
			return nil, fmt.Errorf("step handler for role %q references unknown kind %q", role, kind)
		}
		if !containsString(roles, role) {
			return nil, fmt.Errorf("step handler for kind %q is registered for role %q, but metadata roles are %s", kind, role, strings.Join(roles, ","))
		}
		if _, exists := registered[kind]; exists {
			return nil, fmt.Errorf("duplicate step handler for role %q and kind %q", role, kind)
		}
		registered[kind] = handler
	}

	missing := make([]string, 0)
	for kind := range required {
		if _, ok := registered[kind]; !ok {
			missing = append(missing, kind)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("missing step handlers for role %q: %s", role, strings.Join(missing, ", "))
	}
	return registered, nil
}

func StepRoleHandlerForKey[T any](role string, handlers map[string]T, key StepTypeKey) (T, bool, error) {
	var zero T
	normalized := normalizeStepKey(key)
	allowed, err := StepAllowedForRoleForKey(role, normalized)
	if err != nil {
		return zero, false, err
	}
	if !allowed {
		return zero, false, nil
	}
	handler, ok := handlers[normalized.Kind]
	return handler, ok, nil
}
