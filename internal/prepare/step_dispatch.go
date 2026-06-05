package prepare

import (
	"context"
	"fmt"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/errcode"
	"github.com/Airgap-Castaways/deck/internal/hostcheck"
	"github.com/Airgap-Castaways/deck/internal/operatorio"
	"github.com/Airgap-Castaways/deck/internal/stepmeta"
	"github.com/Airgap-Castaways/deck/internal/stepspec"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

type prepareStepHandler func(ctx context.Context, runner CommandRunner, bundleRoot string, step config.Step, rendered map[string]any, inputVars map[string]string, opts RunOptions) ([]string, map[string]any, error)

var prepareStepHandlers = workflowexec.MustStepRoleHandlers("prepare", map[string]prepareStepHandler{
	"CheckHost":       prepareCheckHost,
	"DownloadFile":    prepareDownloadFile,
	"DownloadImage":   prepareDownloadImage,
	"DownloadPackage": prepareDownloadPackage,
	"Message":         prepareMessage,
})

func runPrepareRenderedStepWithKey(ctx context.Context, runner CommandRunner, bundleRoot string, step config.Step, rendered map[string]any, key workflowexec.StepTypeKey, inputVars map[string]string, opts RunOptions) ([]string, map[string]any, error) {
	kind := step.Kind
	handler, ok, err := workflowexec.StepRoleHandlerForKey("prepare", prepareStepHandlers, key)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, errcode.Newf(errCodePrepareKindUnsupported, "unsupported step kind %s", kind)
	}
	files, outputs, err := handler(ctx, runner, bundleRoot, step, rendered, inputVars, opts)
	if err != nil {
		return nil, nil, err
	}
	projected, err := stepmeta.ProjectRuntimeOutputsForKind(kind, rendered, outputs, stepmeta.RuntimeOutputOptions{})
	if err != nil {
		return nil, nil, err
	}
	return files, projected, nil
}

func prepareDownloadFile(ctx context.Context, _ CommandRunner, bundleRoot string, _ config.Step, rendered map[string]any, _ map[string]string, opts RunOptions) ([]string, map[string]any, error) {
	files, err := runDownloadFiles(ctx, bundleRoot, rendered, opts)
	if err != nil {
		return nil, nil, err
	}
	return files, map[string]any{"artifacts": files}, nil
}

func prepareDownloadPackage(ctx context.Context, runner CommandRunner, bundleRoot string, step config.Step, rendered map[string]any, inputVars map[string]string, opts RunOptions) ([]string, map[string]any, error) {
	files, err := runDownloadPackage(ctx, runner, bundleRoot, step, rendered, inputVars, "packages", opts)
	if err != nil {
		return nil, nil, err
	}
	return files, map[string]any{"artifacts": files}, nil
}

func prepareDownloadImage(ctx context.Context, runner CommandRunner, bundleRoot string, _ config.Step, rendered map[string]any, _ map[string]string, opts RunOptions) ([]string, map[string]any, error) {
	files, err := runDownloadImage(ctx, runner, bundleRoot, rendered, opts)
	if err != nil {
		return nil, nil, err
	}
	return files, map[string]any{"artifacts": files}, nil
}

func prepareCheckHost(_ context.Context, runner CommandRunner, _ string, _ config.Step, rendered map[string]any, _ map[string]string, opts RunOptions) ([]string, map[string]any, error) {
	decoded, err := workflowexec.DecodeSpec[stepspec.CheckHost](rendered)
	if err != nil {
		return nil, nil, fmt.Errorf("decode checks spec: %w", err)
	}
	deps := resolveCheckHostRuntime(opts)
	outputs, err := hostcheck.Run(decoded, runner, hostcheck.Runtime{
		ReadHostFile:  deps.readHostFile,
		CurrentGOOS:   deps.currentGOOS,
		CurrentGOARCH: deps.currentGOARCH,
	}, errCodePrepareCheckHostFailed)
	if err != nil {
		return nil, nil, err
	}
	return nil, outputs, nil
}

func prepareMessage(_ context.Context, _ CommandRunner, _ string, _ config.Step, rendered map[string]any, _ map[string]string, opts RunOptions) ([]string, map[string]any, error) {
	decoded, err := workflowexec.DecodeSpec[stepspec.Message](rendered)
	if err != nil {
		return nil, nil, fmt.Errorf("decode message spec: %w", err)
	}
	interaction := opts.Interaction
	if interaction == nil {
		interaction = operatorio.Default()
	}
	if err := runPrepareMessage(decoded, interaction); err != nil {
		return nil, nil, err
	}
	return nil, nil, nil
}

func runPrepareMessage(spec stepspec.Message, interaction operatorio.Interface) error {
	level := strings.TrimSpace(spec.Level)
	if level == "" {
		level = "info"
	}
	stream := strings.TrimSpace(spec.Stream)
	if stream == "" {
		stream = "stdout"
	}
	if err := interaction.Message(level, spec.Message, stream); err != nil {
		return fmt.Errorf("message failed: %w", err)
	}
	return nil
}
