package install

import "github.com/Airgap-Castaways/deck/internal/workflowexec"

func evaluateWhen(expr string, vars map[string]any, runtime map[string]any) (bool, error) {
	return evaluateWhenWithContext(expr, vars, runtime, nil)
}

func evaluateWhenWithContext(expr string, vars map[string]any, runtime map[string]any, context map[string]any) (bool, error) {
	return workflowexec.EvaluateWhenWithContext(expr, vars, runtime, context, errCodeConditionEval)
}

func EvaluateWhen(expr string, vars map[string]any, runtime map[string]any) (bool, error) {
	return evaluateWhen(expr, vars, runtime)
}

func EvaluateWhenWithContext(expr string, vars map[string]any, runtime map[string]any, context map[string]any) (bool, error) {
	return evaluateWhenWithContext(expr, vars, runtime, context)
}
