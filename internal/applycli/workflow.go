package applycli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/taedi90/deck/internal/config"
)

type workflowStepRef struct {
	phaseName string
	step      config.Step
}

func BuildPrefetchWorkflow(wf *config.Workflow) *config.Workflow {
	if wf == nil {
		return &config.Workflow{}
	}
	prefetchSteps := make([]config.Step, 0)
	for _, phase := range wf.Phases {
		for _, step := range phase.Steps {
			if step.Kind == "DownloadFile" {
				prefetchSteps = append(prefetchSteps, step)
			}
		}
	}
	if len(prefetchSteps) == 0 {
		return &config.Workflow{}
	}
	return &config.Workflow{
		Version:        wf.Version,
		Vars:           wf.Vars,
		Phases:         []config.Phase{{Name: "prefetch", Steps: prefetchSteps}},
		StateKey:       wf.StateKey,
		WorkflowSHA256: wf.WorkflowSHA256,
	}
}

func BuildExecutionWorkflow(wf *config.Workflow, phaseName string, stepSelection StepSelection) (*config.Workflow, error) {
	if wf == nil {
		return nil, errors.New("workflow is nil")
	}
	phases, err := selectWorkflowPhases(wf, phaseName)
	if err != nil {
		return nil, err
	}
	selectedSteps, err := selectWorkflowSteps(phases, stepSelection.Normalize())
	if err != nil {
		return nil, err
	}
	return &config.Workflow{
		Version:        wf.Version,
		Vars:           wf.Vars,
		Phases:         selectedSteps,
		StateKey:       wf.StateKey,
		WorkflowSHA256: wf.WorkflowSHA256,
	}, nil
}

func selectWorkflowPhases(wf *config.Workflow, phaseName string) ([]config.Phase, error) {
	if strings.TrimSpace(phaseName) == "" {
		phases := make([]config.Phase, len(wf.Phases))
		copy(phases, wf.Phases)
		return phases, nil
	}
	selectedPhase, found := findWorkflowPhaseByName(wf, phaseName)
	if !found {
		return nil, fmt.Errorf("%s phase not found", phaseName)
	}
	return []config.Phase{{Name: selectedPhase.Name, Steps: selectedPhase.Steps}}, nil
}

func selectWorkflowSteps(phases []config.Phase, selection StepSelection) ([]config.Phase, error) {
	selection = selection.Normalize()
	if selection.IsZero() {
		selectedPhases := make([]config.Phase, len(phases))
		copy(selectedPhases, phases)
		return selectedPhases, nil
	}
	ordered := make([]workflowStepRef, 0)
	for _, phase := range phases {
		for _, step := range phase.Steps {
			ordered = append(ordered, workflowStepRef{phaseName: phase.Name, step: step})
		}
	}
	if len(ordered) == 0 {
		return nil, errors.New("no steps found")
	}
	start, end, err := resolveStepRange(ordered, selection)
	if err != nil {
		return nil, err
	}
	selected := make([]config.Phase, 0, len(phases))
	orderedIndex := 0
	for _, phase := range phases {
		phaseSteps := make([]config.Step, 0)
		for _, step := range phase.Steps {
			if orderedIndex >= start && orderedIndex <= end {
				phaseSteps = append(phaseSteps, step)
			}
			orderedIndex++
		}
		if len(phaseSteps) > 0 {
			selected = append(selected, config.Phase{Name: phase.Name, Steps: phaseSteps})
		}
	}
	if len(selected) == 0 {
		return nil, errors.New("step selector matched no steps")
	}
	return selected, nil
}

func resolveStepRange(ordered []workflowStepRef, selection StepSelection) (int, int, error) {
	if selection.SelectedStep != "" {
		index := stepIndexByID(ordered, selection.SelectedStep)
		if index == -1 {
			return 0, 0, fmt.Errorf("step %s not found", selection.SelectedStep)
		}
		return index, index, nil
	}
	start := 0
	if selection.FromStep != "" {
		start = stepIndexByID(ordered, selection.FromStep)
		if start == -1 {
			return 0, 0, fmt.Errorf("step %s not found", selection.FromStep)
		}
	}
	end := len(ordered) - 1
	if selection.ToStep != "" {
		end = stepIndexByID(ordered, selection.ToStep)
		if end == -1 {
			return 0, 0, fmt.Errorf("step %s not found", selection.ToStep)
		}
	}
	if start > end {
		return 0, 0, fmt.Errorf("step range is invalid: from-step %s occurs after to-step %s", selection.FromStep, selection.ToStep)
	}
	return start, end, nil
}

func stepIndexByID(ordered []workflowStepRef, id string) int {
	for i, ref := range ordered {
		if ref.step.ID == id {
			return i
		}
	}
	return -1
}

func findWorkflowPhaseByName(wf *config.Workflow, name string) (config.Phase, bool) {
	for _, phase := range wf.Phases {
		if phase.Name == name {
			return phase, true
		}
	}
	return config.Phase{}, false
}
