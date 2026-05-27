package install

import (
	"context"
	"fmt"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/errcode"
	"github.com/Airgap-Castaways/deck/internal/operatorio"
	"github.com/Airgap-Castaways/deck/internal/stepspec"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func installMessage(_ context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, execCtx ExecutionContext) (map[string]any, error) {
	decoded, err := workflowexec.DecodeSpec[stepspec.Message](effectiveSpec)
	if err != nil {
		return nil, fmt.Errorf("decode message spec: %w", err)
	}
	return nil, runMessage(decoded, execCtx.Interaction, execCtx.SecretValues)
}

func installConfirm(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, execCtx ExecutionContext) (map[string]any, error) {
	decoded, err := workflowexec.DecodeSpec[stepspec.Confirm](effectiveSpec)
	if err != nil {
		return nil, fmt.Errorf("decode confirm spec: %w", err)
	}
	confirmed, err := runConfirm(ctx, decoded, execCtx.Interaction)
	if err != nil {
		return nil, err
	}
	return map[string]any{"confirmed": confirmed}, nil
}

func installInput(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, execCtx ExecutionContext) (map[string]any, error) {
	decoded, err := workflowexec.DecodeSpec[stepspec.Input](effectiveSpec)
	if err != nil {
		return nil, fmt.Errorf("decode input spec: %w", err)
	}
	value, err := runInput(ctx, decoded, execCtx.Interaction)
	if err != nil {
		return nil, err
	}
	return map[string]any{"value": value}, nil
}

func runMessage(spec stepspec.Message, interaction operatorio.Interface, secretValues []string) error {
	if interaction == nil {
		interaction = operatorio.Default()
	}
	if containsSecretValue(spec.Message, secretValues) {
		return errcode.Newf(errCodeInstallInteractionUnsupported, "Message cannot render a secret runtime value")
	}
	level := strings.TrimSpace(spec.Level)
	if level == "" {
		level = "info"
	}
	stream := strings.TrimSpace(spec.Stream)
	if stream == "" {
		stream = "stdout"
	}
	if err := interaction.Message(level, spec.Message, stream); err != nil {
		return errcode.New(errCodeInstallInteraction, err)
	}
	return nil
}

func containsSecretValue(text string, secretValues []string) bool {
	for _, secret := range secretValues {
		if secret != "" && strings.Contains(text, secret) {
			return true
		}
	}
	return false
}

func runConfirm(ctx context.Context, spec stepspec.Confirm, interaction operatorio.Interface) (bool, error) {
	if interaction == nil {
		interaction = operatorio.Default()
	}
	confirmed, err := interaction.Confirm(ctx, spec.Message, spec.Default)
	if err != nil {
		return false, errcode.New(errCodeInstallInteraction, err)
	}
	onNo := strings.TrimSpace(spec.OnNo)
	if onNo == "" {
		onNo = "fail"
	}
	if !confirmed && onNo == "fail" {
		return false, errcode.Newf(errCodeInstallInteraction, "operator answered no")
	}
	return confirmed, nil
}

func runInput(ctx context.Context, spec stepspec.Input, interaction operatorio.Interface) (string, error) {
	if interaction == nil {
		interaction = operatorio.Default()
	}
	required := true
	if spec.Required != nil {
		required = *spec.Required
	}
	defaultValue := ""
	hasDefault := false
	if spec.Default != nil {
		defaultValue = *spec.Default
		hasDefault = true
	}
	value, err := interaction.Input(ctx, spec.Message, operatorio.InputOptions{Default: defaultValue, HasDefault: hasDefault, Required: required, Secret: spec.Secret})
	if err != nil {
		return "", errcode.New(errCodeInstallInteraction, err)
	}
	return value, nil
}
