package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Airgap-Castaways/deck/internal/errcode"
	"github.com/Airgap-Castaways/deck/internal/executil"
	"github.com/Airgap-Castaways/deck/internal/stepspec"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func aptNonInteractiveEnv() map[string]string {
	return map[string]string{
		"DEBIAN_FRONTEND":          "noninteractive",
		"APT_LISTCHANGES_FRONTEND": "none",
		"NEEDRESTART_MODE":         "l",
		"NEEDRESTART_SUSPEND":      "1",
	}
}

func dnfNonInteractiveEnv() map[string]string {
	return map[string]string{
		"TERM": "dumb",
	}
}

func runAptGetInstall(ctx context.Context, args []string, timeout time.Duration) error {
	return runTimedCommandSpecWithContext(ctx, append([]string{"apt-get"}, args...), aptNonInteractiveEnv(), false, timeout, os.Stdout, os.Stderr)
}

func runDnfInstall(ctx context.Context, args []string, timeout time.Duration) error {
	cmdArgs := []string{"dnf", "-q", "--setopt=color=never"}
	cmdArgs = append(cmdArgs, args...)
	return runTimedCommandSpecWithContext(ctx, cmdArgs, dnfNonInteractiveEnv(), false, timeout, os.Stdout, os.Stderr)
}

func runInstallPackages(ctx context.Context, spec map[string]any) error {
	return runInstallPackagesForKind(ctx, "InstallPackage", spec)
}

type packageInstallRequest struct {
	Kind            string
	Manager         string
	Source          *stepspec.InstallPackageSource
	Packages        []string
	RestrictToRepos []string
	ExcludeRepos    []string
	Apt             stepspec.InstallAptPackageOptions
	Dnf             stepspec.InstallDnfPackageOptions
	Timeout         string
}

func runInstallPackagesForKind(ctx context.Context, kind string, spec map[string]any) error {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}

	req, err := normalizePackageInstallRequest(kind, spec)
	if err != nil {
		return err
	}
	if len(req.Packages) == 0 {
		return errcode.Newf(errCodeInstallPackagesRequired, "InstallPackages requires packages")
	}

	sourcePath, err := validatePackageInstallSource(req.Source)
	if err != nil {
		return err
	}

	manager, err := resolvePackageInstallManager(req)
	if err != nil {
		return err
	}
	if err := validatePackageInstallManagerBinary(manager); err != nil {
		return err
	}

	args := []string{"install", "-y"}
	policy := buildPackageRepoPolicy(req.RestrictToRepos, req.ExcludeRepos)
	cleanup := func() {}
	if manager == "apt" {
		repoArgs, repoCleanup, err := aptRepoArgs(policy)
		if err != nil {
			return errcode.New(errCodeInstallPkgSourceInvalid, err)
		}
		if repoCleanup != nil {
			cleanup = repoCleanup
		}
		args = append(args, repoArgs...)
	} else {
		args = append(args, dnfRepoArgs(policy)...)
	}
	defer cleanup()

	optionArgs, err := installPackageOptionArgs(manager, req)
	if err != nil {
		return err
	}
	args = append(args, optionArgs...)

	if sourcePath != "" {
		ext := ".rpm"
		if manager == "apt" {
			ext = ".deb"
		}
		artifacts, err := collectPackageArtifact(sourcePath, ext)
		if err != nil {
			return errcode.New(errCodeInstallPkgSourceInvalid, err)
		}
		args = append(args, artifacts...)
	} else {
		args = append(args, req.Packages...)
	}

	var installErr error
	if manager == "apt" {
		installErr = runAptGetInstall(ctx, args, parseStepTimeout(req.Timeout, 10*time.Minute))
	} else {
		installErr = runDnfInstall(ctx, args, parseStepTimeout(req.Timeout, 10*time.Minute))
	}
	if installErr != nil {
		if errors.Is(installErr, ErrStepCommandTimeout) || errors.Is(installErr, context.DeadlineExceeded) {
			return errcode.New(errCodeInstallPkgFailed, fmt.Errorf("package installation timed out: %w", installErr))
		}
		return errcode.New(errCodeInstallPkgFailed, fmt.Errorf("package installation failed: %w", installErr))
	}
	return nil
}

func normalizePackageInstallRequest(kind string, spec map[string]any) (packageInstallRequest, error) {
	switch strings.TrimSpace(kind) {
	case "", "InstallPackage":
		decoded, err := workflowexec.DecodeSpec[stepspec.InstallPackage](spec)
		if err != nil {
			return packageInstallRequest{}, fmt.Errorf("decode InstallPackage spec: %w", err)
		}
		manager := strings.TrimSpace(string(decoded.Manager))
		if manager == "" {
			manager = "auto"
		}
		_, hasApt := spec["apt"]
		_, hasDnf := spec["dnf"]
		if (hasApt || hasDnf) && manager == "auto" {
			return packageInstallRequest{}, errcode.Newf(errCodeInstallPkgOptionInvalid, "InstallPackage requires explicit manager apt or dnf when apt or dnf options are set")
		}
		if manager == "apt" && hasDnf {
			return packageInstallRequest{}, errcode.Newf(errCodeInstallPkgOptionInvalid, "dnf options require manager dnf")
		}
		if manager == "dnf" && hasApt {
			return packageInstallRequest{}, errcode.Newf(errCodeInstallPkgOptionInvalid, "apt options require manager apt")
		}
		return packageInstallRequest{Kind: "InstallPackage", Manager: manager, Source: decoded.Source, Packages: decoded.Packages, RestrictToRepos: decoded.RestrictToRepos, ExcludeRepos: decoded.ExcludeRepos, Apt: decoded.Apt, Dnf: decoded.Dnf, Timeout: decoded.Timeout}, nil
	case "InstallAptPackage":
		if _, ok := spec["manager"]; ok {
			return packageInstallRequest{}, errcode.Newf(errCodeInstallPkgOptionInvalid, "InstallAptPackage does not accept manager")
		}
		if _, ok := spec["dnf"]; ok {
			return packageInstallRequest{}, errcode.Newf(errCodeInstallPkgOptionInvalid, "dnf options require InstallDnfPackage")
		}
		decoded, err := workflowexec.DecodeSpec[stepspec.InstallAptPackage](spec)
		if err != nil {
			return packageInstallRequest{}, fmt.Errorf("decode InstallAptPackage spec: %w", err)
		}
		return packageInstallRequest{Kind: "InstallAptPackage", Manager: "apt", Source: decoded.Source, Packages: decoded.Packages, RestrictToRepos: decoded.RestrictToRepos, ExcludeRepos: decoded.ExcludeRepos, Apt: decoded.Apt, Timeout: decoded.Timeout}, nil
	case "InstallDnfPackage":
		if _, ok := spec["manager"]; ok {
			return packageInstallRequest{}, errcode.Newf(errCodeInstallPkgOptionInvalid, "InstallDnfPackage does not accept manager")
		}
		if _, ok := spec["apt"]; ok {
			return packageInstallRequest{}, errcode.Newf(errCodeInstallPkgOptionInvalid, "apt options require InstallAptPackage")
		}
		decoded, err := workflowexec.DecodeSpec[stepspec.InstallDnfPackage](spec)
		if err != nil {
			return packageInstallRequest{}, fmt.Errorf("decode InstallDnfPackage spec: %w", err)
		}
		return packageInstallRequest{Kind: "InstallDnfPackage", Manager: "dnf", Source: decoded.Source, Packages: decoded.Packages, RestrictToRepos: decoded.RestrictToRepos, ExcludeRepos: decoded.ExcludeRepos, Dnf: decoded.Dnf, Timeout: decoded.Timeout}, nil
	default:
		return packageInstallRequest{}, errcode.Newf(errCodeInstallKindUnsupported, "unsupported package install kind %s", kind)
	}
}

func validatePackageInstallSource(source *stepspec.InstallPackageSource) (string, error) {
	if source == nil {
		return "", nil
	}
	typeVal := strings.TrimSpace(source.Type)
	if typeVal != "" && typeVal != "local-repo" {
		return "", errcode.Newf(errCodeInstallPkgSourceInvalid, "unsupported source type %q", typeVal)
	}
	path := strings.TrimSpace(source.Path)
	if path == "" {
		return "", nil
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", errcode.Newf(errCodeInstallPkgSourceInvalid, "source path must be an existing directory: %s", path)
	}
	return path, nil
}

func resolvePackageInstallManager(req packageInstallRequest) (string, error) {
	manager := strings.TrimSpace(req.Manager)
	if manager == "" {
		manager = "auto"
	}
	switch manager {
	case "apt", "dnf":
		return validatePackageInstallManagerForHost(manager)
	case "auto":
		facts := repoConfigDetectHostFacts()
		osFacts, _ := facts["os"].(map[string]any)
		family := strings.ToLower(strings.TrimSpace(stringValue(osFacts, "family")))
		switch family {
		case "debian":
			return "apt", nil
		case "rhel":
			return "dnf", nil
		default:
			return "", errcode.Newf(errCodeInstallPkgMgrMissing, "unable to resolve package manager from host family %q", family)
		}
	default:
		return "", errcode.Newf(errCodeInstallPkgOptionInvalid, "InstallPackage manager must be one of auto, apt, dnf")
	}
}

func validatePackageInstallManagerForHost(manager string) (string, error) {
	facts := repoConfigDetectHostFacts()
	osFacts, _ := facts["os"].(map[string]any)
	family := strings.ToLower(strings.TrimSpace(stringValue(osFacts, "family")))
	if family == "" {
		return manager, nil
	}
	if manager == "apt" && family != "debian" {
		return "", errcode.Newf(errCodeInstallPkgMgrMissing, "apt package install requested on host family %q", family)
	}
	if manager == "dnf" && family != "rhel" {
		return "", errcode.Newf(errCodeInstallPkgMgrMissing, "dnf package install requested on host family %q", family)
	}
	return manager, nil
}

func validatePackageInstallManagerBinary(manager string) error {
	switch manager {
	case "apt":
		if _, err := executil.LookPathAptGet(); err != nil {
			return errcode.Newf(errCodeInstallPkgMgrMissing, "apt-get not found")
		}
	case "dnf":
		if _, err := executil.LookPathDnf(); err != nil {
			return errcode.Newf(errCodeInstallPkgMgrMissing, "dnf not found")
		}
	default:
		return errcode.Newf(errCodeInstallPkgMgrMissing, "unsupported package manager %q", manager)
	}
	return nil
}

func installPackageOptionArgs(manager string, spec packageInstallRequest) ([]string, error) {
	switch manager {
	case "apt":
		return aptInstallPackageOptionArgs(spec.Apt), nil
	case "dnf":
		return dnfInstallPackageOptionArgs(spec.Dnf), nil
	default:
		return nil, errcode.Newf(errCodeInstallPkgMgrMissing, "unsupported package manager %q", manager)
	}
}

func aptInstallPackageOptionArgs(options stepspec.InstallAptPackageOptions) []string {
	args := make([]string, 0)
	if options.FixBroken {
		args = append(args, "--fix-broken")
	}
	if options.InstallRecommends != nil {
		value := "no"
		if *options.InstallRecommends {
			value = "yes"
		}
		args = append(args, "-o", "APT::Install-Recommends="+value)
	}
	for _, option := range options.DPKGOptions {
		option = strings.TrimSpace(option)
		if option != "" {
			args = append(args, "-o", "Dpkg::Options::=--"+option)
		}
	}
	if release := strings.TrimSpace(options.DefaultRelease); release != "" {
		args = append(args, "-t", release)
	}
	if options.AllowDowngrade {
		args = append(args, "--allow-downgrades")
	}
	if options.FailOnAutoremove {
		args = append(args, "--no-remove")
	}
	return args
}

func dnfInstallPackageOptionArgs(options stepspec.InstallDnfPackageOptions) []string {
	args := make([]string, 0)
	if options.SkipBroken {
		args = append(args, "--skip-broken")
	}
	if options.AllowErasing {
		args = append(args, "--allowerasing")
	}
	if options.InstallWeakDeps != nil {
		value := "False"
		if *options.InstallWeakDeps {
			value = "True"
		}
		args = append(args, "--setopt=install_weak_deps="+value)
	}
	if options.DisableGPGCheck {
		args = append(args, "--nogpgcheck")
	}
	if options.Best != nil {
		if *options.Best {
			args = append(args, "--best")
		} else {
			args = append(args, "--nobest")
		}
	}
	if options.CacheOnly {
		args = append(args, "--cacheonly")
	}
	for _, pattern := range options.ExcludePackages {
		pattern = strings.TrimSpace(pattern)
		if pattern != "" {
			args = append(args, "--exclude="+pattern)
		}
	}
	return args
}

func collectPackageArtifact(root, ext string) ([]string, error) {
	artifacts := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), strings.ToLower(ext)) {
			artifacts = append(artifacts, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(artifacts) == 0 {
		return nil, fmt.Errorf("no %s artifacts found under %s", ext, root)
	}
	sort.Strings(artifacts)
	return artifacts, nil
}
