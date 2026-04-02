package preparecli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/buildinfo"
	"github.com/Airgap-Castaways/deck/internal/filemode"
	"github.com/Airgap-Castaways/deck/internal/fsutil"
	"github.com/Airgap-Castaways/deck/internal/httpfetch"
	"github.com/Airgap-Castaways/deck/internal/userdirs"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

var runtimeBinaryDownloadHTTPClient = httpfetch.Client(0)

const (
	binarySourceAuto    = "auto"
	binarySourceLocal   = "local"
	binarySourceRelease = "release"
)

type runtimeBinaryTarget struct {
	OS   string
	Arch string
}

var supportedRuntimeBinaryTargets = []runtimeBinaryTarget{
	{OS: "linux", Arch: "amd64"},
	{OS: "linux", Arch: "arm64"},
	{OS: "darwin", Arch: "amd64"},
	{OS: "darwin", Arch: "arm64"},
}

type runtimeBinaryDeps struct {
	currentGOOS   func() string
	currentGOARCH func() string
	readFile      func(string) ([]byte, error)
	osExecutable  func() (string, error)
	latestRelease func(ctx context.Context) (string, error)
	fetchRelease  func(ctx context.Context, version string, target runtimeBinaryTarget) ([]byte, error)
	cacheRoot     func() (string, error)
}

func defaultRuntimeBinaryDeps() runtimeBinaryDeps {
	return runtimeBinaryDeps{
		currentGOOS:   func() string { return runtime.GOOS },
		currentGOARCH: func() string { return runtime.GOARCH },
		readFile:      fsutil.ReadFile,
		osExecutable:  os.Executable,
		latestRelease: fetchLatestReleaseVersion,
		fetchRelease:  fetchReleaseRuntimeBinary,
		cacheRoot:     userdirs.CacheRoot,
	}
}

func stageRuntimeBinariesWithContext(ctx context.Context, preparedRootAbs string, opts Options) error {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}
	deps := opts.runtimeBinaryDeps
	if deps.currentGOOS == nil || deps.currentGOARCH == nil || deps.readFile == nil || deps.osExecutable == nil || deps.latestRelease == nil || deps.fetchRelease == nil || deps.cacheRoot == nil {
		deps = defaultRuntimeBinaryDeps()
	}
	source, err := resolveBinarySource(opts, deps)
	if err != nil {
		return err
	}
	releaseVersion, err := resolveRuntimeBinaryReleaseVersion(ctx, opts, deps, source)
	if err != nil {
		return err
	}
	targets, err := resolveBinaryTargets(opts, source, deps)
	if err != nil {
		return err
	}
	for _, target := range targets {
		raw, err := loadRuntimeBinary(ctx, opts, deps, source, releaseVersion, target)
		if err != nil {
			return err
		}
		relPath := filepath.Join(workspacepaths.PreparedBinRoot, target.OS, target.Arch, "deck")
		if err := writeBytes(filepath.Join(preparedRootAbs, relPath), raw, 0o755); err != nil {
			return err
		}
		if err := emitDiagnostic(opts, 2, "deck: prepare runtimeBinary=%s source=%s\n", filepath.ToSlash(filepath.Join(preparedRootAbs, relPath)), source); err != nil {
			return err
		}
	}
	return nil
}

func dryRunRuntimeBinaryWrites(preparedRootAbs string, opts Options) ([]string, error) {
	deps := opts.runtimeBinaryDeps
	if deps.currentGOOS == nil || deps.currentGOARCH == nil || deps.readFile == nil || deps.osExecutable == nil || deps.latestRelease == nil || deps.fetchRelease == nil || deps.cacheRoot == nil {
		deps = defaultRuntimeBinaryDeps()
	}
	source, err := resolveBinarySource(opts, deps)
	if err != nil {
		return nil, err
	}
	targets, err := resolveBinaryTargets(opts, source, deps)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(targets))
	for _, target := range targets {
		paths = append(paths, filepath.Join(preparedRootAbs, workspacepaths.PreparedBinRoot, target.OS, target.Arch, "deck"))
	}
	return paths, nil
}

func resolveBinarySource(opts Options, deps runtimeBinaryDeps) (string, error) {
	requested := strings.ToLower(strings.TrimSpace(opts.BinarySource))
	if requested == "" {
		requested = binarySourceAuto
	}
	switch requested {
	case binarySourceAuto:
		if buildinfo.Current().Version == "dev" {
			if strings.TrimSpace(opts.BinaryDir) != "" {
				return binarySourceLocal, nil
			}
			return binarySourceRelease, nil
		}
		return binarySourceRelease, nil
	case binarySourceLocal, binarySourceRelease:
		return requested, nil
	default:
		return "", fmt.Errorf("--bundle-binary-source must be auto, local, or release")
	}
}

func resolveBinaryTargets(opts Options, source string, deps runtimeBinaryDeps) ([]runtimeBinaryTarget, error) {
	baseTargets := opts.Binaries
	if len(baseTargets) == 0 {
		if source == binarySourceLocal && strings.TrimSpace(opts.BinaryDir) == "" {
			return []runtimeBinaryTarget{{OS: deps.currentGOOS(), Arch: deps.currentGOARCH()}}, nil
		}
		baseTargets = make([]string, 0, len(supportedRuntimeBinaryTargets))
		for _, target := range supportedRuntimeBinaryTargets {
			baseTargets = append(baseTargets, target.OS+"/"+target.Arch)
		}
	}
	targets, err := parseRuntimeBinaryTargets(baseTargets)
	if err != nil {
		return nil, err
	}
	excludes, err := parseRuntimeBinaryTargets(opts.BinaryExcludes)
	if err != nil {
		return nil, fmt.Errorf("parse --bundle-binary-exclude: %w", err)
	}
	if len(excludes) == 0 {
		return targets, nil
	}
	excluded := make(map[string]bool, len(excludes))
	for _, target := range excludes {
		excluded[target.OS+"/"+target.Arch] = true
	}
	filtered := make([]runtimeBinaryTarget, 0, len(targets))
	for _, target := range targets {
		if excluded[target.OS+"/"+target.Arch] {
			continue
		}
		filtered = append(filtered, target)
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no runtime binaries selected after applying --bundle-binary-exclude")
	}
	return filtered, nil
}

func parseRuntimeBinaryTargets(rawTargets []string) ([]runtimeBinaryTarget, error) {
	seen := map[string]bool{}
	targets := make([]runtimeBinaryTarget, 0, len(rawTargets))
	for _, raw := range rawTargets {
		target, err := parseRuntimeBinaryTarget(raw)
		if err != nil {
			return nil, err
		}
		key := target.OS + "/" + target.Arch
		if seen[key] {
			continue
		}
		seen[key] = true
		targets = append(targets, target)
	}
	return targets, nil
}

func parseRuntimeBinaryTarget(raw string) (runtimeBinaryTarget, error) {
	parts := strings.Split(strings.TrimSpace(raw), "/")
	if len(parts) != 2 {
		return runtimeBinaryTarget{}, fmt.Errorf("--bundle-binary must use os/arch")
	}
	osVal := strings.ToLower(strings.TrimSpace(parts[0]))
	archVal := strings.ToLower(strings.TrimSpace(parts[1]))
	if (osVal != "linux" && osVal != "darwin") || (archVal != "amd64" && archVal != "arm64") {
		return runtimeBinaryTarget{}, fmt.Errorf("unsupported bundle binary target %s", raw)
	}
	return runtimeBinaryTarget{OS: osVal, Arch: archVal}, nil
}

func resolveRuntimeBinaryReleaseVersion(ctx context.Context, opts Options, deps runtimeBinaryDeps, source string) (string, error) {
	if source != binarySourceRelease {
		return "", nil
	}
	version := strings.TrimSpace(opts.BinaryVer)
	if version != "" {
		return version, nil
	}
	version = buildinfo.Current().Version
	if version != "" && version != "dev" {
		return version, nil
	}
	version, err := deps.latestRelease(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve latest deck release: %w", err)
	}
	version = strings.TrimSpace(version)
	if version == "" {
		return "", fmt.Errorf("resolve latest deck release: empty version")
	}
	return version, nil
}

func loadRuntimeBinary(ctx context.Context, opts Options, deps runtimeBinaryDeps, source string, releaseVersion string, target runtimeBinaryTarget) ([]byte, error) {
	switch source {
	case binarySourceLocal:
		return loadLocalRuntimeBinary(opts, deps, target)
	case binarySourceRelease:
		if strings.TrimSpace(releaseVersion) == "" {
			return nil, fmt.Errorf("release version is required")
		}
		return loadCachedReleaseRuntimeBinary(ctx, deps, releaseVersion, target)
	default:
		return nil, fmt.Errorf("unsupported binary source %s", source)
	}
}

func loadCachedReleaseRuntimeBinary(ctx context.Context, deps runtimeBinaryDeps, version string, target runtimeBinaryTarget) ([]byte, error) {
	cachePath, err := runtimeBinaryCachePath(deps, version, target)
	if err != nil {
		return nil, err
	}
	raw, err := deps.readFile(cachePath)
	if err == nil {
		return raw, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read runtime binary cache %s: %w", cachePath, err)
	}
	raw, err = deps.fetchRelease(ctx, version, target)
	if err != nil {
		return nil, err
	}
	if err := writeBytesAtomically(cachePath, raw, 0o755); err != nil {
		return nil, fmt.Errorf("cache runtime binary %s: %w", cachePath, err)
	}
	return raw, nil
}

func runtimeBinaryCachePath(deps runtimeBinaryDeps, version string, target runtimeBinaryTarget) (string, error) {
	cacheRoot, err := deps.cacheRoot()
	if err != nil {
		return "", err
	}
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if v == "" {
		return "", fmt.Errorf("release version is required")
	}
	return filepath.Join(cacheRoot, "artifacts", "runtime-binary", "v"+v, target.OS, target.Arch, "deck"), nil
}

func writeBytesAtomically(path string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), filemode.ArtifactDirMode); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	base := filepath.Base(path)
	if base == "" || base == "." || base == string(filepath.Separator) {
		base = "runtime-binary"
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+base+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	published := false
	defer func() {
		_ = tmp.Close()
		if !published {
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("publish temp file: %w", err)
	}
	published = true
	return nil
}

func loadLocalRuntimeBinary(opts Options, deps runtimeBinaryDeps, target runtimeBinaryTarget) ([]byte, error) {
	dir := strings.TrimSpace(opts.BinaryDir)
	if dir == "" {
		if target.OS != deps.currentGOOS() || target.Arch != deps.currentGOARCH() {
			return nil, fmt.Errorf("--bundle-binary-source=local without --bundle-binary-dir only supports the current host target %s/%s", deps.currentGOOS(), deps.currentGOARCH())
		}
		execPath, err := deps.osExecutable()
		if err != nil {
			return nil, fmt.Errorf("resolve deck binary path: %w", err)
		}
		return deps.readFile(execPath)
	}
	path, err := resolveLocalRuntimeBinaryPath(dir, target)
	if err != nil {
		return nil, err
	}
	return deps.readFile(path)
}

func resolveLocalRuntimeBinaryPath(dir string, target runtimeBinaryTarget) (string, error) {
	base := strings.TrimSpace(dir)
	candidates := []string{
		filepath.Join(base, fmt.Sprintf("deck-%s-%s", target.OS, target.Arch)),
		filepath.Join(base, fmt.Sprintf("deck_%s_%s", target.OS, target.Arch)),
		filepath.Join(base, target.OS, target.Arch, "deck"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("local runtime binary not found for %s/%s under %s", target.OS, target.Arch, base)
}

func fetchReleaseRuntimeBinary(ctx context.Context, version string, target runtimeBinaryTarget) ([]byte, error) {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if v == "" {
		return nil, fmt.Errorf("release version is required")
	}

	url := fmt.Sprintf("https://github.com/Airgap-Castaways/deck/releases/download/v%s/deck_%s_%s_%s.tar.gz", v, v, target.OS, target.Arch)
	return downloadArchiveDeckBinary(ctx, url)
}

func fetchLatestReleaseVersion(ctx context.Context) (string, error) {
	return fetchLatestReleaseVersionFromURL(ctx, "https://github.com/Airgap-Castaways/deck/releases/latest")
}

func fetchLatestReleaseVersionFromURL(ctx context.Context, rawURL string) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("context is nil")
	}
	client := *runtimeBinaryDownloadHTTPClient
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	parsed, err := urlpkgParseHTTPS(rawURL)
	if err != nil {
		return "", fmt.Errorf("resolve latest release URL: %w", err)
	}
	resp, err := httpfetch.Do(ctx, &client, http.MethodGet, parsed.String(), nil, "resolve latest release")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently && resp.StatusCode != http.StatusTemporaryRedirect && resp.StatusCode != http.StatusPermanentRedirect {
		return "", fmt.Errorf("resolve latest release: unexpected status %d", resp.StatusCode)
	}
	location := strings.TrimSpace(resp.Header.Get("Location"))
	if location == "" {
		return "", fmt.Errorf("resolve latest release: missing redirect location")
	}
	redirectURL, err := urlpkgParseHTTPS(location)
	if err != nil {
		return "", fmt.Errorf("resolve latest release redirect: %w", err)
	}
	version := pathBase(redirectURL.Path)
	if version == "" || version == "latest" {
		return "", fmt.Errorf("resolve latest release: invalid redirect target %s", redirectURL.String())
	}
	return version, nil
}

func downloadArchiveDeckBinary(ctx context.Context, url string) ([]byte, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}
	parsed, err := urlpkgParseHTTPS(url)
	if err != nil {
		return nil, err
	}
	resp, err := httpfetch.Do(ctx, runtimeBinaryDownloadHTTPClient, http.MethodGet, parsed.String(), nil, "download release archive "+parsed.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download release archive %s: unexpected status %d", parsed.String(), resp.StatusCode)
	}
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read release archive %s: %w", parsed.String(), err)
	}
	defer func() { _ = gzr.Close() }()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read release archive %s: %w", parsed.String(), err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if pathBase(hdr.Name) != "deck" {
			continue
		}
		return io.ReadAll(tr)
	}
	return nil, fmt.Errorf("release archive %s does not contain deck", parsed.String())
}

func urlpkgParseHTTPS(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("parse release archive URL %s: %w", raw, err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, fmt.Errorf("release archive URL must use http or https: %s", raw)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("release archive URL host is required: %s", raw)
	}
	return parsed, nil
}

func pathBase(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}
