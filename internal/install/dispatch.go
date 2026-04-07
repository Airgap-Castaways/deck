package install

import (
	"context"
	"fmt"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/errcode"
	"github.com/Airgap-Castaways/deck/internal/executil"
	"github.com/Airgap-Castaways/deck/internal/hostcheck"
	"github.com/Airgap-Castaways/deck/internal/stepspec"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

type installLookPathRunner struct{}

func (installLookPathRunner) LookPath(file string) (string, error) {
	return executil.LookPathWorkflowBinary(file)
}

type installStepHandler func(ctx context.Context, step config.Step, rendered map[string]any, effectiveSpec map[string]any, execCtx ExecutionContext) (map[string]any, error)

var installStepHandlers = map[string]installStepHandler{
	"CheckHost":                    installCheckHost,
	"CheckKubernetesCluster":       installCheckKubernetesCluster,
	"Command":                      installCommand,
	"ConfigureRepository":          installConfigureRepository,
	"CopyFile":                     installCopyFile,
	"CreateSymlink":                installCreateSymlink,
	"EditFile":                     installEditFile,
	"EditJSON":                     installEditJSON,
	"EditTOML":                     installEditTOML,
	"EditYAML":                     installEditYAML,
	"EnsureDirectory":              installEnsureDirectory,
	"ExtractArchive":               installExtractArchive,
	"InitKubeadm":                  installInitKubeadm,
	"InstallPackage":               installPackages,
	"JoinKubeadm":                  installJoinKubeadm,
	"KernelModule":                 installKernelModule,
	"LoadImage":                    installLoadImage,
	"ManageService":                installManageService,
	"RefreshRepository":            installRefreshRepository,
	"ResetKubeadm":                 installResetKubeadm,
	"Swap":                         installSwap,
	"Sysctl":                       installSysctl,
	"UpgradeKubeadm":               installUpgradeKubeadm,
	"VerifyImage":                  installVerifyImage,
	"WaitForCommand":               installWait,
	"WaitForFile":                  installWait,
	"WaitForMissingFile":           installWait,
	"WaitForMissingTCPPort":        installWait,
	"WaitForService":               installWait,
	"WaitForTCPPort":               installWait,
	"WriteContainerdConfig":        installWriteContainerdConfig,
	"WriteContainerdRegistryHosts": installWriteContainerdRegistryHosts,
	"WriteFile":                    installWriteFile,
	"WriteSystemdUnit":             installWriteSystemdUnit,
}

func executeWorkflowStep(ctx context.Context, step config.Step, rendered map[string]any, key workflowexec.StepTypeKey, execCtx ExecutionContext) (map[string]any, error) {
	kind := step.Kind
	effectiveSpec := specWithStepTimeout(rendered, step.Timeout)
	allowed, err := workflowexec.StepAllowedForRoleForKey("apply", key)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, errcode.Newf(errCodeInstallKindUnsupported, "unsupported step kind %s", kind)
	}
	handler, ok := installStepHandlers[kind]
	if !ok {
		return nil, errcode.Newf(errCodeInstallKindUnsupported, "unsupported step kind %s", kind)
	}
	return handler(ctx, step, rendered, effectiveSpec, execCtx)
}

func specWithStepTimeout(rendered map[string]any, stepTimeout string) map[string]any {
	trimmed := strings.TrimSpace(stepTimeout)
	if trimmed == "" {
		return rendered
	}
	cloned := make(map[string]any, len(rendered)+1)
	for key, value := range rendered {
		cloned[key] = value
	}
	cloned["timeout"] = trimmed
	return cloned
}

func installPackages(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runInstallPackages(ctx, effectiveSpec)
}

func installWriteFile(_ context.Context, _ config.Step, rendered map[string]any, _ map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runWriteFile(rendered)
}

func installCopyFile(ctx context.Context, _ config.Step, rendered map[string]any, _ map[string]any, execCtx ExecutionContext) (map[string]any, error) {
	return nil, runCopyFile(ctx, execCtx.BundleRoot, rendered)
}

func installEditFile(_ context.Context, _ config.Step, rendered map[string]any, _ map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runEditFile(rendered)
}

func installEditTOML(_ context.Context, _ config.Step, rendered map[string]any, _ map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runEditTOML(rendered)
}

func installEditYAML(_ context.Context, _ config.Step, rendered map[string]any, _ map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runEditYAML(rendered)
}

func installEditJSON(_ context.Context, _ config.Step, rendered map[string]any, _ map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runEditJSON(rendered)
}

func installExtractArchive(ctx context.Context, _ config.Step, rendered map[string]any, _ map[string]any, execCtx ExecutionContext) (map[string]any, error) {
	return nil, runExtractArchive(ctx, execCtx.BundleRoot, rendered)
}

func installSysctl(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runSysctl(ctx, effectiveSpec)
}

func installManageService(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runManageService(ctx, effectiveSpec)
}

func installEnsureDirectory(_ context.Context, _ config.Step, rendered map[string]any, _ map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runEnsureDir(rendered)
}

func installCreateSymlink(_ context.Context, _ config.Step, rendered map[string]any, _ map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runCreateSymlink(rendered)
}

func installWriteSystemdUnit(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runWriteSystemdUnit(ctx, effectiveSpec)
}

func installConfigureRepository(ctx context.Context, _ config.Step, rendered map[string]any, _ map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runRepoConfig(ctx, rendered)
}

func installRefreshRepository(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runRefreshRepository(ctx, effectiveSpec)
}

func installWriteContainerdConfig(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runWriteContainerdConfig(ctx, effectiveSpec)
}

func installWriteContainerdRegistryHosts(_ context.Context, _ config.Step, rendered map[string]any, _ map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runWriteContainerdRegistryHosts(rendered)
}

func installSwap(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runSwap(ctx, effectiveSpec)
}

func installKernelModule(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runKernelModule(ctx, effectiveSpec)
}

func installCommand(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, _ ExecutionContext) (map[string]any, error) {
	decoded, err := workflowexec.DecodeSpec[stepspec.Command](effectiveSpec)
	if err != nil {
		return nil, fmt.Errorf("decode command spec: %w", err)
	}
	return nil, runCommandDecoded(ctx, decoded)
}

func installLoadImage(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, execCtx ExecutionContext) (map[string]any, error) {
	return nil, runLoadImage(ctx, execCtx.BundleRoot, effectiveSpec)
}

func installVerifyImage(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runVerifyImages(ctx, effectiveSpec)
}

func installInitKubeadm(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, execCtx ExecutionContext) (map[string]any, error) {
	return nil, runInitKubeadm(ctx, effectiveSpec, execCtx.kubeadm)
}

func installJoinKubeadm(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, execCtx ExecutionContext) (map[string]any, error) {
	return nil, runJoinKubeadm(ctx, effectiveSpec, execCtx.kubeadm)
}

func installResetKubeadm(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, execCtx ExecutionContext) (map[string]any, error) {
	return nil, runResetKubeadm(ctx, effectiveSpec, execCtx.kubeadm)
}

func installUpgradeKubeadm(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, execCtx ExecutionContext) (map[string]any, error) {
	return nil, runUpgradeKubeadm(ctx, effectiveSpec, execCtx.kubeadm)
}

func installCheckKubernetesCluster(ctx context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, _ ExecutionContext) (map[string]any, error) {
	return nil, runCheckKubernetesCluster(ctx, effectiveSpec)
}

func installCheckHost(_ context.Context, _ config.Step, _ map[string]any, effectiveSpec map[string]any, _ ExecutionContext) (map[string]any, error) {
	decoded, err := workflowexec.DecodeSpec[stepspec.CheckHost](effectiveSpec)
	if err != nil {
		return nil, fmt.Errorf("decode check host spec: %w", err)
	}
	return hostcheck.Run(decoded, installLookPathRunner{}, hostcheck.DefaultRuntime(), errCodeInstallCheckHostFailed)
}

func installWait(ctx context.Context, step config.Step, _ map[string]any, effectiveSpec map[string]any, _ ExecutionContext) (map[string]any, error) {
	decoded, err := workflowexec.DecodeSpec[stepspec.Wait](effectiveSpec)
	if err != nil {
		return nil, fmt.Errorf("decode wait spec: %w", err)
	}
	return nil, runWaitDecoded(ctx, step.Kind, decoded, commandTimeout(effectiveSpec))
}
