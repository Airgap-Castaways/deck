package askpolicy

import (
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func PlanNeedsClarification(plan askcontract.PlanResponse) bool {
	for _, item := range plan.Clarifications {
		if item.BlocksGeneration && strings.TrimSpace(item.Answer) == "" {
			return true
		}
	}
	return false
}

func NormalizePlanNotes(existingBlockers []string, existingQuestions []string, clarifications []askcontract.PlanClarification) ([]string, []string) {
	clarificationText := map[string]bool{}
	for _, item := range clarifications {
		if text := strings.TrimSpace(item.Question); text != "" {
			clarificationText[text] = true
		}
	}
	blockers := []string{}
	for _, item := range existingBlockers {
		text := strings.TrimSpace(item)
		if text == "" || clarificationText[text] {
			continue
		}
		blockers = append(blockers, text)
	}
	questions := []string{}
	for _, item := range existingQuestions {
		text := strings.TrimSpace(item)
		if text == "" || clarificationText[text] {
			continue
		}
		questions = append(questions, text)
	}
	return dedupeStrings(blockers), dedupeStrings(questions)
}
