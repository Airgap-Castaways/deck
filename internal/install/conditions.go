package install

import "github.com/taedi90/deck/internal/workflowexec"

func evaluateWhen(expr string, vars map[string]any, runtime map[string]any, ctx map[string]any) (bool, error) {
	return workflowexec.EvaluateWhen(expr, vars, runtime, ctx, errCodeConditionEval)
}

func EvaluateWhen(expr string, vars map[string]any, runtime map[string]any, ctx map[string]any) (bool, error) {
	return evaluateWhen(expr, vars, runtime, ctx)
}
