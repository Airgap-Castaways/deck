package applycli

import "strings"

type StepSelection struct {
	SelectedStep string
	FromStep     string
	ToStep       string
}

func (s StepSelection) Normalize() StepSelection {
	return StepSelection{
		SelectedStep: strings.TrimSpace(s.SelectedStep),
		FromStep:     strings.TrimSpace(s.FromStep),
		ToStep:       strings.TrimSpace(s.ToStep),
	}
}

func (s StepSelection) IsZero() bool {
	normalized := s.Normalize()
	return normalized.SelectedStep == "" && normalized.FromStep == "" && normalized.ToStep == ""
}

func (s StepSelection) Summary() string {
	normalized := s.Normalize()
	if normalized.SelectedStep != "" {
		return "step=" + normalized.SelectedStep
	}
	parts := make([]string, 0, 2)
	if normalized.FromStep != "" {
		parts = append(parts, "from="+normalized.FromStep)
	}
	if normalized.ToStep != "" {
		parts = append(parts, "to="+normalized.ToStep)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ",")
}
