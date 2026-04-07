package prepare

import (
	"context"
	"fmt"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/errcode"
	"github.com/Airgap-Castaways/deck/internal/hostcheck"
	"github.com/Airgap-Castaways/deck/internal/stepmeta"
	"github.com/Airgap-Castaways/deck/internal/stepspec"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

type prepareStepHandler func(ctx context.Context, runner CommandRunner, bundleRoot string, step config.Step, rendered map[string]any, inputVars map[string]string, opts RunOptions) ([]string, map[string]any, error)

var prepareStepHandlers = map[string]prepareStepHandler{
	"CheckHost":       prepareCheckHost,
	"DownloadFile":    prepareDownloadFile,
	"DownloadImage":   prepareDownloadImage,
	"DownloadPackage": prepareDownloadPackage,
}

func runPrepareRenderedStepWithKey(ctx context.Context, runner CommandRunner, bundleRoot string, step config.Step, rendered map[string]any, key workflowexec.StepTypeKey, inputVars map[string]string, opts RunOptions) ([]string, map[string]any, error) {
	kind := step.Kind
	allowed, err := workflowexec.StepAllowedForRoleForKey("prepare", key)
	if err != nil {
		return nil, nil, err
	}
	if !allowed {
		return nil, nil, errcode.Newf(errCodePrepareKindUnsupported, "unsupported step kind %s", kind)
	}
	handler, ok := prepareStepHandlers[kind]
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
