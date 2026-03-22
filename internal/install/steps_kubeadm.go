package install

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/taedi90/deck/internal/filemode"
	"github.com/taedi90/deck/internal/fsutil"
	"github.com/taedi90/deck/internal/hostfs"
	"github.com/taedi90/deck/internal/workflowexec"
)

type kubeadmResetSpec struct {
	Force                       bool     `json:"force"`
	IgnoreErrors                bool     `json:"ignoreErrors"`
	StopKubelet                 *bool    `json:"stopKubelet"`
	CriSocket                   string   `json:"criSocket"`
	ExtraArgs                   []string `json:"extraArgs"`
	RemovePaths                 []string `json:"removePaths"`
	RemoveFiles                 []string `json:"removeFiles"`
	CleanupContainers           []string `json:"cleanupContainers"`
	RestartRuntimeManageService string   `json:"restartRuntimeService"`
	WaitForRuntimeService       bool     `json:"waitForRuntimeService"`
	WaitForRuntimeReady         bool     `json:"waitForRuntimeReady"`
	WaitForMissingManifestsGlob string   `json:"waitForMissingManifestsGlob"`
	StopKubeletAfterReset       bool     `json:"stopKubeletAfterReset"`
	VerifyContainersAbsent      []string `json:"verifyContainersAbsent"`
	ReportFile                  string   `json:"reportFile"`
	ReportResetReason           string   `json:"reportResetReason"`
	Timeout                     string   `json:"timeout"`
}

type kubeadmInitSpec struct {
	OutputJoinFile        string   `json:"outputJoinFile"`
	SkipIfAdminConfExists *bool    `json:"skipIfAdminConfExists"`
	CriSocket             string   `json:"criSocket"`
	KubernetesVersion     string   `json:"kubernetesVersion"`
	ConfigFile            string   `json:"configFile"`
	ConfigTemplate        string   `json:"configTemplate"`
	PodNetworkCIDR        string   `json:"podNetworkCIDR"`
	AdvertiseAddress      string   `json:"advertiseAddress"`
	PullImages            bool     `json:"pullImages"`
	IgnorePreflightErrors []string `json:"ignorePreflightErrors"`
	ExtraArgs             []string `json:"extraArgs"`
	Timeout               string   `json:"timeout"`
}

var kubeadmAdminConfPath = "/etc/kubernetes/admin.conf"

var (
	kubeadmInitExecutor    = runInitKubeadmReal
	kubeadmJoinExecutor    = runJoinKubeadmReal
	kubeadmResetExecutor   = runResetKubeadmReal
	kubeadmUpgradeExecutor = runUpgradeKubeadmReal
)

type kubeadmJoinSpec struct {
	JoinFile       string   `json:"joinFile"`
	ConfigFile     string   `json:"configFile"`
	AsControlPlane bool     `json:"asControlPlane"`
	ExtraArgs      []string `json:"extraArgs"`
	Timeout        string   `json:"timeout"`
}

type kubeadmUpgradeSpec struct {
	KubernetesVersion     string   `json:"kubernetesVersion"`
	IgnorePreflightErrors []string `json:"ignorePreflightErrors"`
	ExtraArgs             []string `json:"extraArgs"`
	RestartKubelet        *bool    `json:"restartKubelet"`
	KubeletService        string   `json:"kubeletService"`
	Timeout               string   `json:"timeout"`
}

func runInitKubeadm(ctx context.Context, spec map[string]any) error {
	decoded, err := workflowexec.DecodeSpec[kubeadmInitSpec](spec)
	if err != nil {
		return fmt.Errorf("decode InitKubeadm spec: %w", err)
	}
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}
	return kubeadmInitExecutor(ctx, decoded)
}

func runJoinKubeadm(ctx context.Context, spec map[string]any) error {
	decoded, err := workflowexec.DecodeSpec[kubeadmJoinSpec](spec)
	if err != nil {
		return fmt.Errorf("decode JoinKubeadm spec: %w", err)
	}
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}
	return kubeadmJoinExecutor(ctx, decoded)
}

func runUpgradeKubeadm(ctx context.Context, spec map[string]any) error {
	decoded, err := workflowexec.DecodeSpec[kubeadmUpgradeSpec](spec)
	if err != nil {
		return fmt.Errorf("decode UpgradeKubeadm spec: %w", err)
	}
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}
	return kubeadmUpgradeExecutor(ctx, decoded)
}

func runInitKubeadmReal(parent context.Context, spec kubeadmInitSpec) error {
	joinFile := strings.TrimSpace(spec.OutputJoinFile)
	if joinFile == "" {
		return fmt.Errorf("%s: InitKubeadm requires outputJoinFile", errCodeInstallInitJoinMissing)
	}
	if shouldSkipInitKubeadm(spec) {
		return nil
	}
	timeout := parseStepTimeout(spec.Timeout, 10*time.Minute)
	criSocket := strings.TrimSpace(spec.CriSocket)
	kubernetesVersion := strings.TrimSpace(spec.KubernetesVersion)
	configFile := strings.TrimSpace(spec.ConfigFile)
	configTemplate := strings.TrimSpace(spec.ConfigTemplate)

	advertiseAddress, err := resolveKubeadmAdvertiseAddress(parent, spec, configTemplate, timeout)
	if err != nil {
		return fmt.Errorf("%s: %w", errCodeInstallInitFailed, err)
	}

	if configTemplate != "" {
		if configFile == "" {
			return fmt.Errorf("%s: configTemplate requires configFile", errCodeInstallInitFailed)
		}
		configBody := configTemplate
		if strings.EqualFold(configTemplate, "default") {
			configBody = renderDefaultInitKubeadmConfig(
				advertiseAddress,
				kubernetesVersion,
				strings.TrimSpace(spec.PodNetworkCIDR),
				criSocket,
			)
		}
		if !strings.HasSuffix(configBody, "\n") {
			configBody += "\n"
		}
		if err := filemode.WritePrivateFile(configFile, []byte(configBody)); err != nil {
			return err
		}
	}

	if spec.PullImages {
		pullArgs := []string{"config", "images", "pull"}
		if kubernetesVersion != "" {
			pullArgs = append(pullArgs, "--kubernetes-version", kubernetesVersion)
		}
		if criSocket != "" {
			pullArgs = append(pullArgs, "--cri-socket", criSocket)
		}
		if err := runTimedCommandWithContext(parent, "kubeadm", pullArgs, timeout); err != nil {
			if errors.Is(err, ErrStepCommandTimeout) {
				return fmt.Errorf("%s: kubeadm config images pull timed out: %w", errCodeInstallInitFailed, err)
			}
			return fmt.Errorf("%s: kubeadm config images pull failed: %w", errCodeInstallInitFailed, err)
		}
	}

	args := []string{"init"}
	if configFile != "" {
		args = append(args, "--config", configFile)
	} else {
		if advertiseAddress != "" {
			args = append(args, "--apiserver-advertise-address", advertiseAddress)
		}
		if podCIDR := strings.TrimSpace(spec.PodNetworkCIDR); podCIDR != "" {
			args = append(args, "--pod-network-cidr", podCIDR)
		}
		if criSocket != "" {
			args = append(args, "--cri-socket", criSocket)
		}
		if kubernetesVersion != "" {
			args = append(args, "--kubernetes-version", kubernetesVersion)
		}
	}
	if ignore := trimmedStringSlice(spec.IgnorePreflightErrors); len(ignore) > 0 {
		args = append(args, "--ignore-preflight-errors", strings.Join(ignore, ","))
	}
	if extra := trimmedStringSlice(spec.ExtraArgs); len(extra) > 0 {
		args = append(args, extra...)
	}

	if err := runTimedCommandWithContext(parent, "kubeadm", args, timeout); err != nil {
		if errors.Is(err, ErrStepCommandTimeout) {
			return fmt.Errorf("%s: kubeadm init timed out: %w", errCodeInstallInitFailed, err)
		}
		return fmt.Errorf("%s: kubeadm init failed: %w", errCodeInstallInitFailed, err)
	}

	joinArgs := []string{"token", "create", "--print-join-command"}
	joinOut, err := runCommandOutputWithContext(parent, append([]string{"kubeadm"}, joinArgs...), timeout)
	if err != nil {
		if errors.Is(err, ErrStepCommandTimeout) {
			return fmt.Errorf("%s: kubeadm token create timed out", errCodeInstallInitFailed)
		}
		return fmt.Errorf("%s: kubeadm token create failed: %w", errCodeInstallInitFailed, err)
	}
	joinCmd := strings.TrimSpace(joinOut)
	if joinCmd == "" {
		return fmt.Errorf("%s: empty kubeadm join command output", errCodeInstallInitFailed)
	}

	return filemode.WritePrivateFile(joinFile, []byte(joinCmd+"\n"))
}

func shouldSkipInitKubeadm(spec kubeadmInitSpec) bool {
	skip := true
	if spec.SkipIfAdminConfExists != nil {
		skip = *spec.SkipIfAdminConfExists
	}
	if !skip {
		return false
	}
	_, err := os.Stat(kubeadmAdminConfPath)
	return err == nil
}

func resolveKubeadmAdvertiseAddress(ctx context.Context, spec kubeadmInitSpec, configTemplate string, timeout time.Duration) (string, error) {
	advertiseAddress := strings.TrimSpace(spec.AdvertiseAddress)
	if strings.EqualFold(advertiseAddress, "auto") || (advertiseAddress == "" && strings.EqualFold(configTemplate, "default")) {
		resolved, err := detectKubeadmAdvertiseAddress(ctx, timeout)
		if err != nil {
			return "", fmt.Errorf("failed to detect node IPv4 for kubeadm init: %w", err)
		}
		return resolved, nil
	}
	return advertiseAddress, nil
}

func detectKubeadmAdvertiseAddress(ctx context.Context, timeout time.Duration) (string, error) {
	routeOut, routeErr := runCommandOutputWithContext(ctx, []string{"ip", "-4", "route", "get", "1.1.1.1"}, timeout)
	if routeErr == nil {
		if routeSrc := parseRouteSourceIPv4(routeOut); routeSrc != "" {
			return routeSrc, nil
		}
	}
	if routeErr != nil && (errors.Is(routeErr, context.Canceled) || errors.Is(routeErr, context.DeadlineExceeded)) {
		return "", routeErr
	}

	addrOut, addrErr := runCommandOutputWithContext(ctx, []string{"ip", "-4", "-o", "addr", "show", "scope", "global"}, timeout)
	if addrErr == nil {
		if globalIP := parseFirstGlobalIPv4(addrOut); globalIP != "" {
			return globalIP, nil
		}
	}
	if addrErr != nil && (errors.Is(addrErr, context.Canceled) || errors.Is(addrErr, context.DeadlineExceeded)) {
		return "", addrErr
	}

	return "", errors.New("no default-route source IPv4 and no global IPv4 found")
}

func parseRouteSourceIPv4(routeOut string) string {
	fields := strings.Fields(routeOut)
	for i := 0; i < len(fields)-1; i++ {
		if fields[i] != "src" {
			continue
		}
		if parsed := net.ParseIP(fields[i+1]); parsed != nil && parsed.To4() != nil {
			return fields[i+1]
		}
	}
	return ""
}

func parseFirstGlobalIPv4(addrOut string) string {
	for _, line := range strings.Split(addrOut, "\n") {
		for _, token := range strings.Fields(line) {
			if !strings.Contains(token, "/") {
				continue
			}
			ip := strings.SplitN(token, "/", 2)[0]
			if parsed := net.ParseIP(ip); parsed != nil && parsed.To4() != nil {
				return ip
			}
		}
	}
	return ""
}

func renderDefaultInitKubeadmConfig(advertiseAddress, kubernetesVersion, podSubnet, criSocket string) string {
	b := strings.Builder{}
	b.WriteString("apiVersion: kubeadm.k8s.io/v1beta3\n")
	b.WriteString("kind: InitConfiguration\n")
	b.WriteString("localAPIEndpoint:\n")
	_, _ = fmt.Fprintf(&b, "  advertiseAddress: %s\n", advertiseAddress)
	b.WriteString("  bindPort: 6443\n")
	if criSocket != "" {
		b.WriteString("nodeRegistration:\n")
		_, _ = fmt.Fprintf(&b, "  criSocket: %s\n", criSocket)
	}
	b.WriteString("---\n")
	b.WriteString("apiVersion: kubeadm.k8s.io/v1beta3\n")
	b.WriteString("kind: ClusterConfiguration\n")
	if kubernetesVersion != "" {
		_, _ = fmt.Fprintf(&b, "kubernetesVersion: %s\n", kubernetesVersion)
	}
	if podSubnet != "" {
		b.WriteString("networking:\n")
		_, _ = fmt.Fprintf(&b, "  podSubnet: %s\n", podSubnet)
	}
	return b.String()
}

func runJoinKubeadmReal(ctx context.Context, spec kubeadmJoinSpec) error {
	joinFile := strings.TrimSpace(spec.JoinFile)
	configFile := strings.TrimSpace(spec.ConfigFile)
	if joinFile != "" && configFile != "" {
		return fmt.Errorf("%s: JoinKubeadm accepts joinFile or configFile, not both", errCodeInstallJoinInputConflict)
	}
	if joinFile == "" && configFile == "" {
		return fmt.Errorf("%s: JoinKubeadm requires joinFile or configFile", errCodeInstallJoinPathMissing)
	}

	args := []string{"kubeadm", "join"}
	if configFile != "" {
		if _, err := os.Stat(configFile); err != nil {
			return fmt.Errorf("%s: config file not found: %w", errCodeInstallJoinFileMissing, err)
		}
		args = append(args, "--config", configFile)
	} else {
		raw, err := fsutil.ReadFile(joinFile)
		if err != nil {
			return fmt.Errorf("%s: join file not found: %w", errCodeInstallJoinFileMissing, err)
		}
		joinCommand := strings.TrimSpace(string(raw))
		if joinCommand == "" {
			return fmt.Errorf("%s: join command is empty", errCodeInstallJoinCmdMissing)
		}
		args = strings.Fields(joinCommand)
		if len(args) == 0 || args[0] != "kubeadm" {
			return fmt.Errorf("%s: join command must start with kubeadm", errCodeInstallJoinCmdInvalid)
		}
	}
	if spec.AsControlPlane && !stringSliceContains(args[1:], "--control-plane") {
		args = append(args, "--control-plane")
	}
	if extra := trimmedStringSlice(spec.ExtraArgs); len(extra) > 0 {
		args = append(args, extra...)
	}

	if err := runTimedCommandWithContext(ctx, args[0], args[1:], parseStepTimeout(spec.Timeout, 5*time.Minute)); err != nil {
		if errors.Is(err, ErrStepCommandTimeout) {
			return fmt.Errorf("%s: kubeadm join timed out: %w", errCodeInstallJoinFailed, err)
		}
		return fmt.Errorf("%s: kubeadm join failed: %w", errCodeInstallJoinFailed, err)
	}
	return nil
}

func runResetKubeadm(ctx context.Context, spec map[string]any) error {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}

	decoded, err := workflowexec.DecodeSpec[kubeadmResetSpec](spec)
	if err != nil {
		return fmt.Errorf("decode ResetKubeadm spec: %w", err)
	}
	return kubeadmResetExecutor(ctx, decoded)
}

func runResetKubeadmReal(ctx context.Context, decoded kubeadmResetSpec) error {
	stopKubelet := true
	if decoded.StopKubelet != nil {
		stopKubelet = *decoded.StopKubelet
	}
	if stopKubelet {
		_ = runTimedCommandWithContext(ctx, "systemctl", []string{"stop", "kubelet"}, parseStepTimeout(decoded.Timeout, 2*time.Minute))
	}

	kubeadmArgs := []string{"reset"}
	if decoded.Force {
		kubeadmArgs = append(kubeadmArgs, "--force")
	}
	if strings.TrimSpace(decoded.CriSocket) != "" {
		kubeadmArgs = append(kubeadmArgs, "--cri-socket", strings.TrimSpace(decoded.CriSocket))
	}
	if extra := trimmedStringSlice(decoded.ExtraArgs); len(extra) > 0 {
		kubeadmArgs = append(kubeadmArgs, extra...)
	}

	resetErr := runTimedCommandWithContext(ctx, "kubeadm", kubeadmArgs, parseStepTimeout(decoded.Timeout, 10*time.Minute))
	if resetErr != nil && !decoded.IgnoreErrors {
		if errors.Is(resetErr, ErrStepCommandTimeout) {
			return fmt.Errorf("%s: kubeadm reset timed out: %w", errCodeInstallResetFailed, resetErr)
		}
		return fmt.Errorf("%s: kubeadm reset failed: %w", errCodeInstallResetFailed, resetErr)
	}

	if err := removeResetPaths(decoded.RemovePaths); err != nil {
		return fmt.Errorf("%s: remove reset paths: %w", errCodeInstallResetFailed, err)
	}
	if err := removeResetFiles(decoded.RemoveFiles); err != nil {
		return fmt.Errorf("%s: remove reset files: %w", errCodeInstallResetFailed, err)
	}

	cleanupContainers := trimmedStringSlice(decoded.CleanupContainers)
	for _, name := range cleanupContainers {
		if err := cleanupContainerByName(ctx, name, strings.TrimSpace(decoded.CriSocket), parseStepTimeout(decoded.Timeout, 2*time.Minute)); err != nil {
			return fmt.Errorf("%s: cleanup stale container %s: %w", errCodeInstallResetFailed, name, err)
		}
	}

	restartRuntime := strings.TrimSpace(decoded.RestartRuntimeManageService)
	if restartRuntime != "" {
		if err := runTimedCommandWithContext(ctx, "systemctl", []string{"restart", restartRuntime}, parseStepTimeout(decoded.Timeout, 2*time.Minute)); err != nil {
			if errors.Is(err, ErrStepCommandTimeout) {
				return fmt.Errorf("%s: restart runtime service %s timed out: %w", errCodeInstallResetFailed, restartRuntime, err)
			}
			return fmt.Errorf("%s: restart runtime service %s failed: %w", errCodeInstallResetFailed, restartRuntime, err)
		}
	}
	if decoded.WaitForRuntimeService && restartRuntime != "" {
		if err := runWaitDecoded(ctx, "WaitForService", waitSpec{Name: restartRuntime}, parseStepTimeout(decoded.Timeout, 2*time.Minute)); err != nil {
			return fmt.Errorf("%s: runtime service did not become active: %w", errCodeInstallResetFailed, err)
		}
	}
	if decoded.WaitForRuntimeReady {
		command := []string{"crictl"}
		if socket := strings.TrimSpace(decoded.CriSocket); socket != "" {
			command = append(command, "--runtime-endpoint", socket)
		}
		command = append(command, "info")
		if err := runWaitDecoded(ctx, "WaitForCommand", waitSpec{Command: command}, parseStepTimeout(decoded.Timeout, 2*time.Minute)); err != nil {
			return fmt.Errorf("%s: runtime did not become ready: %w", errCodeInstallResetFailed, err)
		}
	}
	if glob := strings.TrimSpace(decoded.WaitForMissingManifestsGlob); glob != "" {
		if err := runWaitDecoded(ctx, "WaitForMissingFile", waitSpec{Glob: glob}, parseStepTimeout(decoded.Timeout, 2*time.Minute)); err != nil {
			return fmt.Errorf("%s: manifests did not disappear: %w", errCodeInstallResetFailed, err)
		}
	}
	for _, name := range trimmedStringSlice(decoded.VerifyContainersAbsent) {
		present, err := kubeadmContainerPresent(ctx, name, strings.TrimSpace(decoded.CriSocket), parseStepTimeout(decoded.Timeout, 2*time.Minute))
		if err != nil {
			return fmt.Errorf("%s: verify stale container %s: %w", errCodeInstallResetFailed, name, err)
		}
		if present {
			return fmt.Errorf("%s: stale container still present: %s", errCodeInstallResetFailed, name)
		}
	}
	if decoded.StopKubeletAfterReset {
		_ = runTimedCommandWithContext(ctx, "systemctl", []string{"stop", "kubelet"}, parseStepTimeout(decoded.Timeout, 2*time.Minute))
	}
	if reportPath := strings.TrimSpace(decoded.ReportFile); reportPath != "" {
		if err := writeResetReport(ctx, decoded, reportPath); err != nil {
			return fmt.Errorf("%s: write reset report: %w", errCodeInstallResetFailed, err)
		}
	}

	return nil
}

func runUpgradeKubeadmReal(ctx context.Context, spec kubeadmUpgradeSpec) error {
	version := strings.TrimSpace(spec.KubernetesVersion)
	if version == "" {
		return fmt.Errorf("%s: UpgradeKubeadm requires kubernetesVersion", errCodeInstallUpgradeFailed)
	}
	timeout := parseStepTimeout(spec.Timeout, 20*time.Minute)
	args := []string{"upgrade", "apply", "-y", version}
	if ignore := trimmedStringSlice(spec.IgnorePreflightErrors); len(ignore) > 0 {
		args = append(args, "--ignore-preflight-errors", strings.Join(ignore, ","))
	}
	if extra := trimmedStringSlice(spec.ExtraArgs); len(extra) > 0 {
		args = append(args, extra...)
	}
	if err := runTimedCommandWithContext(ctx, "kubeadm", args, timeout); err != nil {
		if errors.Is(err, ErrStepCommandTimeout) {
			return fmt.Errorf("%s: kubeadm upgrade timed out: %w", errCodeInstallUpgradeFailed, err)
		}
		return fmt.Errorf("%s: kubeadm upgrade failed: %w", errCodeInstallUpgradeFailed, err)
	}
	restartKubelet := true
	if spec.RestartKubelet != nil {
		restartKubelet = *spec.RestartKubelet
	}
	if !restartKubelet {
		return nil
	}
	service := strings.TrimSpace(spec.KubeletService)
	if service == "" {
		service = "kubelet"
	}
	if err := runTimedCommandWithContext(ctx, "systemctl", []string{"restart", service}, parseStepTimeout(spec.Timeout, 2*time.Minute)); err != nil {
		if errors.Is(err, ErrStepCommandTimeout) {
			return fmt.Errorf("%s: restart service %s timed out: %w", errCodeInstallUpgradeFailed, service, err)
		}
		return fmt.Errorf("%s: restart service %s failed: %w", errCodeInstallUpgradeFailed, service, err)
	}
	return nil
}

func removeResetPaths(paths []string) error {
	for _, path := range trimmedStringSlice(paths) {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func removeResetFiles(paths []string) error {
	for _, path := range trimmedStringSlice(paths) {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func cleanupContainerByName(ctx context.Context, name, criSocket string, timeout time.Duration) error {
	listArgs := []string{}
	if criSocket != "" {
		listArgs = append(listArgs, "--runtime-endpoint", criSocket)
	}
	listArgs = append(listArgs, "ps", "-a", "--name", name, "-q")

	out, err := runCommandOutputWithContext(ctx, append([]string{"crictl"}, listArgs...), timeout)
	if err != nil {
		return err
	}

	ids := strings.Fields(strings.TrimSpace(out))
	if len(ids) == 0 {
		return nil
	}

	rmArgs := []string{}
	if criSocket != "" {
		rmArgs = append(rmArgs, "--runtime-endpoint", criSocket)
	}
	rmArgs = append(rmArgs, "rm", "-f")
	rmArgs = append(rmArgs, ids...)
	return runTimedCommandWithContext(ctx, "crictl", rmArgs, timeout)
}

func kubeadmContainerPresent(ctx context.Context, name, criSocket string, timeout time.Duration) (bool, error) {
	listArgs := []string{}
	if criSocket != "" {
		listArgs = append(listArgs, "--runtime-endpoint", criSocket)
	}
	listArgs = append(listArgs, "ps", "-a", "--name", name, "-q")
	out, err := runCommandOutputWithContext(ctx, append([]string{"crictl"}, listArgs...), timeout)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func writeResetReport(ctx context.Context, decoded kubeadmResetSpec, reportPath string) error {
	ref, err := hostfs.NewHostPath(reportPath)
	if err != nil {
		return err
	}
	if err := ref.EnsureParentDir(filemode.PublishedArtifact); err != nil {
		return err
	}
	manifests := "absent"
	if glob := strings.TrimSpace(decoded.WaitForMissingManifestsGlob); glob != "" {
		matches, err := filepath.Glob(glob)
		if err != nil {
			return err
		}
		if len(matches) > 0 {
			manifests = "present"
		}
	}
	stale := "absent"
	for _, name := range trimmedStringSlice(decoded.VerifyContainersAbsent) {
		present, err := kubeadmContainerPresent(ctx, name, strings.TrimSpace(decoded.CriSocket), 2*time.Minute)
		if err != nil {
			return err
		}
		if present {
			stale = "present"
			break
		}
	}
	kubeletConfig := "absent"
	if info, err := os.Stat("/var/lib/kubelet/config.yaml"); err == nil && info.Size() > 0 {
		kubeletConfig = "present"
	}
	kubeletService := "inactive"
	if err := runTimedCommandWithContext(ctx, "systemctl", []string{"is-active", "--quiet", "kubelet"}, 10*time.Second); err == nil {
		kubeletService = "active"
	}
	content := strings.Join([]string{
		"resetReason=" + strings.TrimSpace(decoded.ReportResetReason),
		"kubeadmReset=ok",
		"manifests=" + manifests,
		"staleControlPlaneContainers=" + stale,
		"containerd=active",
		"kubeletConfig=" + kubeletConfig,
		"kubeletService=" + kubeletService,
		"",
	}, "\n")
	return ref.WriteFile([]byte(content), filemode.PublishedArtifact)
}

func trimmedStringSlice(items []string) []string {
	trimmed := make([]string, 0, len(items))
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v != "" {
			trimmed = append(trimmed, v)
		}
	}
	return trimmed
}

func stringSliceContains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
