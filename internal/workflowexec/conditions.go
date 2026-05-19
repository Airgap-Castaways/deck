package workflowexec

import (
	"fmt"

	"github.com/Airgap-Castaways/deck/internal/workflowexpr"
)

func EvaluateWhen(expr string, vars map[string]any, runtime map[string]any, errCode string) (bool, error) {
	return EvaluateWhenWithContext(expr, vars, runtime, nil, errCode)
}

func EvaluateWhenWithContext(expr string, vars map[string]any, runtime map[string]any, context map[string]any, errCode string) (bool, error) {
	result, err := workflowexpr.EvaluateWhen(expr, workflowexpr.Inputs{Vars: vars, Runtime: runtime, Context: context})
	if err != nil {
		return false, fmt.Errorf("%s: %w", errCode, err)
	}
	return result, nil
}
