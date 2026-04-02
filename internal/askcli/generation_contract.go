package askcli

import (
	"fmt"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
)

func validatePrimaryAuthoringContract(route askintent.Route, gen askcontract.GenerationResponse, attempt int) error {
	if route == askintent.RouteDraft {
		if legacyAuthoringFallbackEnabled() {
			return nil
		}
		return validatePrimaryDraftContract(gen)
	}
	if attempt > 1 || legacyAuthoringFallbackEnabled() {
		return nil
	}
	switch route {
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
		return fmt.Errorf("refine primary path requires edit documents with code-owned transforms")
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
				return fmt.Errorf("refine primary path edit documents must include transforms")
			}
			for _, transform := range doc.Transforms {
				if strings.TrimSpace(transform.Candidate) == "" && !refineAllowsRawTransform(doc, transform) {
					return fmt.Errorf("refine primary path raw transforms must use a supported code-owned transform with an explicit target path")
				}
			}
		default:
			return fmt.Errorf("refine primary path does not allow %s document actions; full-document rewriting is fallback-only", action)
		}
	}
	return nil
}

func refineAllowsRawTransform(doc askcontract.GeneratedDocument, transform askcontract.RefineTransformAction) bool {
	typeName := strings.TrimSpace(transform.Type)
	rawPath := strings.TrimSpace(transform.RawPath)
	if rawPath == "" {
		rawPath = strings.TrimSpace(transform.Path)
	}
	switch typeName {
	case "set-field", "delete-field":
		return rawPath != ""
	case "extract-var":
		return rawPath != "" && strings.TrimSpace(transform.VarName) != ""
	case "extract-component":
		if strings.TrimSpace(doc.Path) == "workflows/vars.yaml" {
			return false
		}
		return strings.TrimSpace(transform.RawPath) != "" && strings.TrimSpace(transform.Path) != ""
	default:
		return false
	}
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
