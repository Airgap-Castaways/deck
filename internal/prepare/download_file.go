package prepare

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/errcode"
	"github.com/Airgap-Castaways/deck/internal/fetch"
	"github.com/Airgap-Castaways/deck/internal/filemode"
	"github.com/Airgap-Castaways/deck/internal/fsutil"
	"github.com/Airgap-Castaways/deck/internal/httpfetch"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

var artifactDownloadHTTPClient = httpfetch.Client(0)

const downloadFileSidecarVersion = 1

type downloadFileSidecarMetadata struct {
	Version       int    `json:"version"`
	URL           string `json:"url"`
	LocalSHA256   string `json:"localSHA256"`
	ETag          string `json:"etag,omitempty"`
	LastModified  string `json:"lastModified,omitempty"`
	ContentLength int64  `json:"contentLength,omitempty"`
}

type downloadFileRemoteCheck struct {
	response *http.Response
	metadata downloadFileSidecarMetadata
}

type prepareDownloadFileBundleSpec struct {
	Root string `json:"root"`
	Path string `json:"path"`
}

type prepareDownloadFileSourceSpec struct {
	URL    string                         `json:"url"`
	Path   string                         `json:"path"`
	SHA256 string                         `json:"sha256"`
	Bundle *prepareDownloadFileBundleSpec `json:"bundle"`
}

type prepareFileFetchSourceSpec struct {
	Type string `json:"type"`
	Path string `json:"path"`
	URL  string `json:"url"`
}

type prepareFileFetchSpec struct {
	OfflineOnly bool                         `json:"offlineOnly"`
	Sources     []prepareFileFetchSourceSpec `json:"sources"`
}

type prepareDownloadFileSpec struct {
	Items      []prepareDownloadFileItemSpec `json:"items"`
	Source     prepareDownloadFileSourceSpec `json:"source"`
	Fetch      prepareFileFetchSpec          `json:"fetch"`
	OutputPath string                        `json:"outputPath"`
	Mode       string                        `json:"mode"`
}

type prepareDownloadFileItemSpec struct {
	Source     prepareDownloadFileSourceSpec `json:"source"`
	Fetch      prepareFileFetchSpec          `json:"fetch"`
	OutputPath string                        `json:"outputPath"`
	Mode       string                        `json:"mode"`
}

func runDownloadFiles(ctx context.Context, bundleRoot string, spec map[string]any, opts RunOptions) ([]string, error) {
	decoded, err := workflowexec.DecodeSpec[prepareDownloadFileSpec](spec)
	if err != nil {
		return nil, fmt.Errorf("decode prepare File spec: %w", err)
	}
	items := normalizePrepareDownloadFileItems(decoded)
	if len(items) == 0 {
		return nil, fmt.Errorf("file action download requires source.path or source.url")
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		artifact, err := runDownloadFileItem(ctx, bundleRoot, item, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, artifact)
	}
	return out, nil
}

func runDownloadFileItem(ctx context.Context, bundleRoot string, decoded prepareDownloadFileItemSpec, opts RunOptions) (string, error) {
	bundleRef := decoded.Source.Bundle
	if bundleRef != nil {
		root := strings.TrimSpace(bundleRef.Root)
		refPath := strings.TrimSpace(bundleRef.Path)
		if root == "" || refPath == "" {
			return "", fmt.Errorf("DownloadFile bundle source requires root and path")
		}
		decoded.Source.Path = filepath.ToSlash(filepath.Join(root, refPath))
		decoded.Source.Bundle = nil
		if bundleRoot != "" {
			decoded.Fetch.Sources = append([]prepareFileFetchSourceSpec{{Type: "bundle", Path: bundleRoot}}, decoded.Fetch.Sources...)
		}
	}
	url := strings.TrimSpace(decoded.Source.URL)
	sourcePath := strings.TrimSpace(decoded.Source.Path)
	expectedSHA := strings.ToLower(strings.TrimSpace(decoded.Source.SHA256))
	offlineOnly := decoded.Fetch.OfflineOnly
	outPath := strings.TrimSpace(decoded.OutputPath)
	if strings.TrimSpace(outPath) == "" {
		outPath = filepath.ToSlash(filepath.Join("files", inferDownloadFileName(sourcePath, url)))
	}
	validatedPath, err := ensurePreparedPathUnderRoot("DownloadFile", "outputPath", outPath, "files")
	if err != nil {
		return "", err
	}
	outPath = validatedPath
	if strings.TrimSpace(sourcePath) == "" && strings.TrimSpace(url) == "" {
		return "", fmt.Errorf("file action download requires source.path or source.url")
	}

	target := filepath.Join(bundleRoot, outPath)
	if err := filemode.EnsureParentDir(target, filemode.PublishedArtifact); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	reuse, remoteCheck, err := canReuseDownloadFile(ctx, decoded, target, opts)
	if err != nil {
		return "", err
	}
	if remoteCheck != nil && remoteCheck.response != nil {
		respBody := remoteCheck.response.Body
		defer func() { _ = respBody.Close() }()
	}
	if reuse {
		return outPath, nil
	}

	f, tempPath, err := createDownloadTempFile(target)
	if err != nil {
		return "", fmt.Errorf("create output file: %w", err)
	}
	published := false
	defer func() {
		_ = f.Close()
		if !published {
			_ = os.Remove(tempPath)
		}
	}()

	var sidecar *downloadFileSidecarMetadata
	if sourcePath != "" {
		raw, err := resolveSourceBytesFromSpec(ctx, decoded, sourcePath)
		if err == nil {
			if _, err := f.Write(raw); err != nil {
				return "", fmt.Errorf("write output file: %w", err)
			}
		} else {
			if url == "" {
				return "", err
			}
			if offlineOnly {
				return "", errcode.Newf(errCodePrepareOfflinePolicyBlock, "source.url fallback blocked by offline policy")
			}
			if _, err := f.Seek(0, 0); err != nil {
				return "", fmt.Errorf("reset output file cursor: %w", err)
			}
			if err := f.Truncate(0); err != nil {
				return "", fmt.Errorf("truncate output file: %w", err)
			}
			meta, err := downloadURLToFile(ctx, f, url, nil)
			if err != nil {
				return "", err
			}
			sidecar = &meta
		}
	} else {
		if offlineOnly {
			return "", errcode.Newf(errCodePrepareOfflinePolicyBlock, "source.url blocked by offline policy")
		}
		if remoteCheck != nil {
			meta, err := writePreparedDownloadResponse(f, remoteCheck)
			if err != nil {
				return "", err
			}
			remoteCheck = nil
			sidecar = &meta
		} else {
			meta, err := downloadURLToFile(ctx, f, url, nil)
			if err != nil {
				return "", err
			}
			sidecar = &meta
		}
	}

	var targetMode os.FileMode = filemode.ArtifactFileMode
	if modeRaw := strings.TrimSpace(decoded.Mode); modeRaw != "" {
		modeVal, err := strconv.ParseUint(modeRaw, 8, 32)
		if err != nil {
			return "", fmt.Errorf("invalid mode: %w", err)
		}
		targetMode = os.FileMode(modeVal)
	}
	if err := f.Chmod(targetMode); err != nil {
		return "", fmt.Errorf("apply mode: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close output file: %w", err)
	}
	if expectedSHA != "" {
		if err := verifyFileSHA256(tempPath, expectedSHA); err != nil {
			return "", err
		}
	}
	if err := os.Rename(tempPath, target); err != nil {
		return "", fmt.Errorf("publish output file: %w", err)
	}
	published = true
	if sidecar != nil {
		if expectedSHA != "" {
			sidecar.LocalSHA256 = expectedSHA
		} else {
			sidecar.LocalSHA256, err = fileSHA256(target)
			if err != nil {
				return "", fmt.Errorf("compute downloaded file checksum: %w", err)
			}
		}
		if err := writeDownloadFileSidecar(target, *sidecar); err != nil {
			return "", err
		}
	} else if err := removeDownloadFileSidecar(target); err != nil {
		return "", err
	}

	return outPath, nil
}

func createDownloadTempFile(target string) (*os.File, string, error) {
	dir := filepath.Dir(target)
	base := filepath.Base(target)
	if base == "" || base == "." || base == string(filepath.Separator) {
		base = "download"
	}
	pattern := "." + base + ".tmp-*"
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, "", err
	}
	return f, f.Name(), nil
}

func downloadURLToFile(ctx context.Context, target *os.File, url string, headers http.Header) (downloadFileSidecarMetadata, error) {
	check, err := openPreparedDownloadRequest(ctx, url, headers)
	if err != nil {
		return downloadFileSidecarMetadata{}, err
	}
	return writePreparedDownloadResponse(target, check)
}

func openPreparedDownloadRequest(ctx context.Context, url string, headers http.Header) (*downloadFileRemoteCheck, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	client := artifactDownloadHTTPClient
	if client == nil {
		client = httpfetch.Client(httpfetch.DefaultTimeout)
	}
	resp, err := client.Do(req) //nolint:bodyclose // Response body ownership transfers to the caller.
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	return &downloadFileRemoteCheck{
		response: resp,
		metadata: downloadFileSidecarMetadata{
			Version:       downloadFileSidecarVersion,
			URL:           strings.TrimSpace(url),
			ETag:          strings.TrimSpace(resp.Header.Get("ETag")),
			LastModified:  strings.TrimSpace(resp.Header.Get("Last-Modified")),
			ContentLength: resp.ContentLength,
		},
	}, nil
}

func writePreparedDownloadResponse(target *os.File, check *downloadFileRemoteCheck) (downloadFileSidecarMetadata, error) {
	if check == nil || check.response == nil {
		return downloadFileSidecarMetadata{}, fmt.Errorf("download response is nil")
	}
	if check.response.StatusCode < 200 || check.response.StatusCode >= 300 {
		return downloadFileSidecarMetadata{}, fmt.Errorf("download %s: unexpected status %d", check.metadata.URL, check.response.StatusCode)
	}
	if _, err := io.Copy(target, check.response.Body); err != nil {
		return downloadFileSidecarMetadata{}, fmt.Errorf("write output file: %w", err)
	}
	return check.metadata, nil
}

func resolveSourceBytes(ctx context.Context, spec map[string]any, sourcePath string) ([]byte, error) {
	decoded, err := workflowexec.DecodeSpec[prepareDownloadFileSpec](spec)
	if err != nil {
		return nil, fmt.Errorf("decode prepare File spec: %w", err)
	}
	items := normalizePrepareDownloadFileItems(decoded)
	if len(items) == 0 {
		items = []prepareDownloadFileItemSpec{{Fetch: decoded.Fetch}}
	}
	if len(items) != 1 {
		return nil, fmt.Errorf("resolveSourceBytes expects a single DownloadFile item")
	}
	return resolveSourceBytesFromSpec(ctx, items[0], sourcePath)
}

func resolveSourceBytesFromSpec(ctx context.Context, spec prepareDownloadFileItemSpec, sourcePath string) ([]byte, error) {
	if len(spec.Fetch.Sources) > 0 {
		sources := make([]fetch.SourceConfig, 0, len(spec.Fetch.Sources))
		for _, source := range spec.Fetch.Sources {
			sources = append(sources, fetch.SourceConfig{Type: source.Type, Path: source.Path, URL: source.URL})
		}
		if len(sources) == 0 {
			return nil, errcode.Newf(errCodeArtifactSourceNotFound, "source.path %s not found in configured fetch sources", sourcePath)
		}
		raw, err := fetch.ResolveBytes(ctx, sourcePath, sources, fetch.ResolveOptions{OfflineOnly: spec.Fetch.OfflineOnly})
		if err == nil {
			return raw, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if ctx != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errcode.Newf(errCodeArtifactSourceNotFound, "source.path %s not found in configured fetch sources", sourcePath)
	}

	raw, err := fsutil.ReadFile(sourcePath)
	if err == nil {
		return raw, nil
	}
	return nil, errcode.Newf(errCodeArtifactSourceNotFound, "source.path %s not found", sourcePath)
}

func verifyFileSHA256(path, expected string) error {
	actual, err := fileSHA256(path)
	if err != nil {
		return fmt.Errorf("read downloaded file for checksum: %w", err)
	}
	if !strings.EqualFold(actual, expected) {
		return errcode.Newf(errCodePrepareChecksumMismatch, "expected %s got %s", expected, actual)
	}
	return nil
}

func inferDownloadFileName(sourcePath, sourceURL string) string {
	if strings.TrimSpace(sourcePath) != "" {
		base := filepath.Base(filepath.FromSlash(strings.TrimSpace(sourcePath)))
		if base != "" && base != "." && base != string(filepath.Separator) {
			return base
		}
	}
	if strings.TrimSpace(sourceURL) != "" {
		trimmed := strings.TrimSpace(sourceURL)
		if idx := strings.Index(trimmed, "?"); idx >= 0 {
			trimmed = trimmed[:idx]
		}
		base := filepath.Base(filepath.FromSlash(trimmed))
		if base != "" && base != "." && base != string(filepath.Separator) {
			return base
		}
	}
	return "downloaded.bin"
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // Hashing an already-selected local artifact path.
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func canReuseDownloadFile(ctx context.Context, spec prepareDownloadFileItemSpec, target string, opts RunOptions) (bool, *downloadFileRemoteCheck, error) {
	if opts.ForceRedownload {
		return false, nil, nil
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	if info.Size() == 0 {
		return false, nil, nil
	}

	expectedSHA := strings.ToLower(strings.TrimSpace(spec.Source.SHA256))
	if expectedSHA != "" {
		if err := verifyFileSHA256(target, expectedSHA); err == nil {
			return true, nil, nil
		}
		return false, nil, nil
	}

	sourcePath := strings.TrimSpace(spec.Source.Path)
	if sourcePath == "" {
		url := strings.TrimSpace(spec.Source.URL)
		if url == "" {
			return false, nil, nil
		}
		meta, ok := readDownloadFileSidecar(target)
		if !ok || !strings.EqualFold(meta.URL, url) || strings.TrimSpace(meta.LocalSHA256) == "" {
			return false, nil, nil
		}
		targetSHA, err := fileSHA256(target)
		if err != nil {
			return false, nil, err
		}
		if !strings.EqualFold(targetSHA, meta.LocalSHA256) {
			return false, nil, nil
		}
		if spec.Fetch.OfflineOnly || (strings.TrimSpace(meta.ETag) == "" && strings.TrimSpace(meta.LastModified) == "") {
			return true, nil, nil
		}
		headers := http.Header{}
		if meta.ETag != "" {
			headers.Set("If-None-Match", meta.ETag)
		}
		if meta.LastModified != "" {
			headers.Set("If-Modified-Since", meta.LastModified)
		}
		check, err := openPreparedDownloadRequest(ctx, url, headers)
		if err != nil {
			return false, nil, err
		}
		if check.response.StatusCode == http.StatusNotModified {
			_ = check.response.Body.Close()
			return true, nil, nil
		}
		if remoteMetadataMatches(meta, check.metadata) {
			_ = check.response.Body.Close()
			return true, nil, nil
		}
		return false, check, nil
	}
	raw, resolveErr := resolveSourceBytesFromSpec(ctx, spec, sourcePath)
	if resolveErr != nil {
		return false, nil, resolveErr
	}
	targetSHA, err := fileSHA256(target)
	if err != nil {
		return false, nil, err
	}
	sum := sha256.Sum256(raw)
	return strings.EqualFold(targetSHA, hex.EncodeToString(sum[:])), nil, nil
}

func downloadFileSidecarPath(target string) string {
	return filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".download.json")
}

func readDownloadFileSidecar(target string) (downloadFileSidecarMetadata, bool) {
	raw, err := os.ReadFile(downloadFileSidecarPath(target))
	if err != nil {
		return downloadFileSidecarMetadata{}, false
	}
	var meta downloadFileSidecarMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return downloadFileSidecarMetadata{}, false
	}
	if meta.Version != downloadFileSidecarVersion {
		return downloadFileSidecarMetadata{}, false
	}
	meta.URL = strings.TrimSpace(meta.URL)
	meta.LocalSHA256 = strings.ToLower(strings.TrimSpace(meta.LocalSHA256))
	meta.ETag = strings.TrimSpace(meta.ETag)
	meta.LastModified = strings.TrimSpace(meta.LastModified)
	return meta, true
}

func writeDownloadFileSidecar(target string, meta downloadFileSidecarMetadata) error {
	meta.Version = downloadFileSidecarVersion
	meta.URL = strings.TrimSpace(meta.URL)
	meta.LocalSHA256 = strings.ToLower(strings.TrimSpace(meta.LocalSHA256))
	meta.ETag = strings.TrimSpace(meta.ETag)
	meta.LastModified = strings.TrimSpace(meta.LastModified)
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("encode download metadata sidecar: %w", err)
	}
	path := downloadFileSidecarPath(target)
	if err := filemode.EnsureParentDir(path, filemode.PublishedArtifact); err != nil {
		return fmt.Errorf("create download metadata directory: %w", err)
	}
	f, tempPath, err := createDownloadTempFile(path)
	if err != nil {
		return fmt.Errorf("create download metadata sidecar: %w", err)
	}
	published := false
	defer func() {
		_ = f.Close()
		if !published {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := f.Write(append(raw, '\n')); err != nil {
		return fmt.Errorf("write download metadata sidecar: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close download metadata sidecar: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("publish download metadata sidecar: %w", err)
	}
	published = true
	return nil
}

func removeDownloadFileSidecar(target string) error {
	path := downloadFileSidecarPath(target)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove download metadata sidecar: %w", err)
	}
	return nil
}

func remoteMetadataMatches(previous, current downloadFileSidecarMetadata) bool {
	if previous.ETag != "" && current.ETag != previous.ETag {
		return false
	}
	if previous.LastModified != "" && current.LastModified != previous.LastModified {
		return false
	}
	if previous.ContentLength > 0 && current.ContentLength > 0 && previous.ContentLength != current.ContentLength {
		return false
	}
	return previous.ETag != "" || previous.LastModified != ""
}

func normalizePrepareDownloadFileItems(spec prepareDownloadFileSpec) []prepareDownloadFileItemSpec {
	if len(spec.Items) > 0 {
		return spec.Items
	}
	if strings.TrimSpace(spec.Source.URL) == "" && strings.TrimSpace(spec.Source.Path) == "" && spec.Source.Bundle == nil {
		return nil
	}
	return []prepareDownloadFileItemSpec{{
		Source:     spec.Source,
		Fetch:      spec.Fetch,
		OutputPath: spec.OutputPath,
		Mode:       spec.Mode,
	}}
}
