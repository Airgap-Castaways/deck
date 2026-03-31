package askcli

import (
	"fmt"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
)

func validatePrimaryAuthoringContract(route askintent.Route, gen askcontract.GenerationResponse, attempt int) error {
	if attempt > 1 || legacyAuthoringFallbackEnabled() {
		return nil
	}
	switch route {
	case askintent.RouteDraft:
		return validatePrimaryDraftContract(gen)
	case askintent.RouteRefine:
		return validatePrimaryRefineContract(gen)
	default:
		return nil
	}
}

func legacyAuthoringFallbackEnabled() bool {
	return askFeatureEnabled("DECK_ASK_ENABLE_LEGACY_AUTHORING_FALLBACK")
}

func validatePrimaryDraftContract(gen askcontract.GenerationResponse) error {
	if gen.Selection == nil {
		return fmt.Errorf("draft primary path requires builder selection under selection.targets[].builders; legacy free-form document authoring is disabled")
	}
	if !askcontract.SelectionUsesBuilders(*gen.Selection) {
		return fmt.Errorf("draft primary path requires selection.targets[].builders; inline selection.steps/phases fallback is disabled")
	}
	if len(gen.Documents) > 0 {
		return fmt.Errorf("draft primary path must not return documents directly; return builder selection only")
	}
	for _, target := range gen.Selection.Targets {
		if len(target.Steps) > 0 || len(target.Phases) > 0 {
			return fmt.Errorf("draft primary path must not return inline step specs or phases inside selection targets")
		}
	}
	return nil
}

func validatePrimaryRefineContract(gen askcontract.GenerationResponse) error {
	if len(gen.Documents) == 0 {
		return fmt.Errorf("refine primary path requires edit documents with transform candidate ids")
	}
	for _, doc := range gen.Documents {
		action := strings.ToLower(strings.TrimSpace(doc.Action))
		if action == "" {
			action = inferredContractAction(doc)
		}
		switch action {
		case "preserve", "delete":
			continue
		case "edit":
			if len(doc.Edits) > 0 {
				return fmt.Errorf("refine primary path must not use raw structured edits; select transform candidate ids instead")
			}
			if len(doc.Transforms) == 0 {
				return fmt.Errorf("refine primary path edit documents must include transform candidate ids")
			}
			for _, transform := range doc.Transforms {
				if strings.TrimSpace(transform.Candidate) == "" {
					return fmt.Errorf("refine primary path must use transform candidate ids instead of raw transform payloads")
				}
			}
		default:
			return fmt.Errorf("refine primary path does not allow %s document actions; full-document rewriting is fallback-only", action)
		}
	}
	return nil
}

func inferredContractAction(doc askcontract.GeneratedDocument) string {
	if len(doc.Edits) > 0 || len(doc.Transforms) > 0 {
		return "edit"
	}
	if doc.Workflow != nil || doc.Component != nil || doc.Vars != nil {
		return "replace"
	}
	return "preserve"
}
