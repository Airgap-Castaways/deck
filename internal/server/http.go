package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type auditLogger struct {
	mu sync.Mutex
	f  *os.File

	path         string
	maxSizeBytes int64
	maxFiles     int
}

type auditLoggerOptions struct {
	maxSizeBytes int64
	maxFiles     int
}

const (
	auditSchemaVersion = 1
	auditSourceServer  = "server"
	auditEventRequest  = "http_request"

	auditEventRegistrySeed = "registry_seed"

	defaultAuditMaxSizeMB = 50
	defaultAuditMaxFiles  = 10
)

func newAuditLogger(root string, opts auditLoggerOptions) (*auditLogger, error) {
	logPath := filepath.Join(root, ".deck", "logs", "server-audit.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, fmt.Errorf("create audit log directory: %w", err)
	}
	if opts.maxSizeBytes <= 0 {
		opts.maxSizeBytes = int64(defaultAuditMaxSizeMB) * 1024 * 1024
	}
	if opts.maxFiles <= 0 {
		opts.maxFiles = defaultAuditMaxFiles
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open audit log file: %w", err)
	}
	return &auditLogger{f: f, path: logPath, maxSizeBytes: opts.maxSizeBytes, maxFiles: opts.maxFiles}, nil
}

func (a *auditLogger) Write(entry map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.shouldRotateLocked() {
		_ = a.rotateLocked()
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_, _ = a.f.Write(append(raw, '\n'))
}

func (a *auditLogger) shouldRotateLocked() bool {
	if a.maxSizeBytes <= 0 {
		return false
	}
	info, err := a.f.Stat()
	if err != nil {
		return false
	}
	return info.Size() > a.maxSizeBytes
}

func (a *auditLogger) rotateLocked() error {
	if err := a.f.Close(); err != nil {
		return err
	}
	var firstErr error

	oldestPath := fmt.Sprintf("%s.%d", a.path, a.maxFiles)
	if err := os.Remove(oldestPath); err != nil && !os.IsNotExist(err) {
		firstErr = err
	}
	if firstErr == nil {
		for i := a.maxFiles - 1; i >= 1; i-- {
			src := fmt.Sprintf("%s.%d", a.path, i)
			dst := fmt.Sprintf("%s.%d", a.path, i+1)
			if err := os.Rename(src, dst); err != nil && !os.IsNotExist(err) {
				firstErr = err
				break
			}
		}
	}
	if firstErr == nil {
		if err := os.Rename(a.path, fmt.Sprintf("%s.1", a.path)); err != nil && !os.IsNotExist(err) {
			firstErr = err
		}
	}
	f, err := os.OpenFile(a.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		if firstErr != nil {
			return fmt.Errorf("%v; reopen audit log: %w", firstErr, err)
		}
		return err
	}
	a.f = f
	return firstErr
}

func buildServerAuditRecord(ts time.Time, eventType, level, message string) map[string]any {
	return map[string]any{
		"ts":             ts.UTC().Format(time.RFC3339Nano),
		"schema_version": auditSchemaVersion,
		"source":         auditSourceServer,
		"event_type":     eventType,
		"level":          level,
		"message":        message,
	}
}

func addExtra(entry map[string]any, extra map[string]any) {
	if len(extra) == 0 {
		return
	}
	entry["extra"] = extra
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

type HandlerOptions struct {
	ReportMax      int
	AuditMaxSizeMB int
	AuditMaxFiles  int
}

type serverHandler struct {
	rootAbs string
	logger  *auditLogger
	base    http.Handler
}

type registryCatalogEntry struct {
	repoTag string
	repo    string
	tag     string
	tarPath string
}

type registryResolvedImage struct {
	repo        string
	tag         string
	tarPath     string
	image       v1.Image
	manifest    *v1.Manifest
	rawManifest []byte
	digest      v1.Hash
}

type registryManifestRequest struct {
	repo string
	ref  string
}

type registryBlobRequest struct {
	repo   string
	digest string
}

func NewHandler(root string, opts HandlerOptions) (http.Handler, error) {
	resolvedRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve server root: %w", err)
	}

	auditMaxSizeMB := opts.AuditMaxSizeMB
	if auditMaxSizeMB <= 0 {
		auditMaxSizeMB = defaultAuditMaxSizeMB
	}
	auditMaxFiles := opts.AuditMaxFiles
	if auditMaxFiles <= 0 {
		auditMaxFiles = defaultAuditMaxFiles
	}

	logger, err := newAuditLogger(resolvedRoot, auditLoggerOptions{maxSizeBytes: int64(auditMaxSizeMB) * 1024 * 1024, maxFiles: auditMaxFiles})
	if err != nil {
		return nil, err
	}

	h := &serverHandler{rootAbs: resolvedRoot, logger: logger}
	h.base = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/healthz":
			h.handleHealthz(w, r)
		case r.URL.Path == "/v2" || r.URL.Path == "/v2/" || strings.HasPrefix(r.URL.Path, "/v2/"):
			h.handleRegistry(w, r)
		case strings.HasPrefix(r.URL.Path, "/files/") || strings.HasPrefix(r.URL.Path, "/packages/") || strings.HasPrefix(r.URL.Path, "/images/") || strings.HasPrefix(r.URL.Path, "/workflows/"):
			h.handleStatic(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now().UTC()
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		h.base.ServeHTTP(rw, r)
		level := "info"
		if rw.status >= http.StatusInternalServerError {
			level = "error"
		} else if rw.status >= http.StatusBadRequest {
			level = "warn"
		}
		entry := buildServerAuditRecord(start, auditEventRequest, level, "http request handled")
		addExtra(entry, map[string]any{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      rw.status,
			"remote_addr": r.RemoteAddr,
			"duration_ms": time.Since(start).Milliseconds(),
		})
		logger.Write(entry)
	}), nil
}

func (h *serverHandler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *serverHandler) handleRegistry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	if r.URL.Path == "/v2" || r.URL.Path == "/v2/" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if req, ok := parseRegistryManifestRequest(r.URL.Path); ok {
		h.handleRegistryManifest(w, r, req)
		return
	}
	if req, ok := parseRegistryBlobRequest(r.URL.Path); ok {
		h.handleRegistryBlob(w, r, req)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func parseRegistryManifestRequest(urlPath string) (registryManifestRequest, bool) {
	const token = "/manifests/"
	if !strings.HasPrefix(urlPath, "/v2/") || !strings.Contains(urlPath, token) {
		return registryManifestRequest{}, false
	}
	rest := strings.TrimPrefix(urlPath, "/v2/")
	parts := strings.SplitN(rest, token, 2)
	if len(parts) != 2 {
		return registryManifestRequest{}, false
	}
	repo := strings.Trim(parts[0], "/")
	ref := strings.TrimSpace(parts[1])
	if repo == "" || ref == "" || strings.Contains(repo, "..") {
		return registryManifestRequest{}, false
	}
	return registryManifestRequest{repo: repo, ref: ref}, true
}

func parseRegistryBlobRequest(urlPath string) (registryBlobRequest, bool) {
	const token = "/blobs/"
	if !strings.HasPrefix(urlPath, "/v2/") || !strings.Contains(urlPath, token) {
		return registryBlobRequest{}, false
	}
	rest := strings.TrimPrefix(urlPath, "/v2/")
	parts := strings.SplitN(rest, token, 2)
	if len(parts) != 2 {
		return registryBlobRequest{}, false
	}
	repo := strings.Trim(parts[0], "/")
	digest := strings.TrimSpace(parts[1])
	if repo == "" || digest == "" || strings.Contains(repo, "..") {
		return registryBlobRequest{}, false
	}
	return registryBlobRequest{repo: repo, digest: digest}, true
}

func (h *serverHandler) handleRegistryManifest(w http.ResponseWriter, r *http.Request, req registryManifestRequest) {
	resolved, err := h.resolveRegistryImage(req.repo, req.ref)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	mediaType := string(types.DockerManifestSchema2)
	if resolved.manifest != nil && strings.TrimSpace(string(resolved.manifest.MediaType)) != "" {
		mediaType = string(resolved.manifest.MediaType)
	}
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Docker-Content-Digest", resolved.digest.String())
	w.Header().Set("Content-Length", strconv.Itoa(len(resolved.rawManifest)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resolved.rawManifest)
}

func (h *serverHandler) handleRegistryBlob(w http.ResponseWriter, r *http.Request, req registryBlobRequest) {
	resolved, err := h.resolveRegistryImage(req.repo, req.digest)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	body, contentType, found, err := resolveRegistryBlobContent(resolved, req.digest)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Docker-Content-Digest", req.digest)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *serverHandler) resolveRegistryImage(repo, ref string) (*registryResolvedImage, error) {
	entries, err := h.scanRegistryCatalog()
	if err != nil {
		return nil, err
	}
	candidates := make([]registryCatalogEntry, 0)
	for _, entry := range entries {
		if entry.repo == repo {
			candidates = append(candidates, entry)
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("repo not found: %s", repo)
	}
	if !strings.HasPrefix(ref, "sha256:") {
		for _, entry := range candidates {
			if entry.tag == ref {
				return loadRegistryResolvedImage(entry)
			}
		}
		return nil, fmt.Errorf("tag not found: %s:%s", repo, ref)
	}
	for _, entry := range candidates {
		resolved, loadErr := loadRegistryResolvedImage(entry)
		if loadErr != nil {
			continue
		}
		if resolved.digest.String() == ref {
			return resolved, nil
		}
	}
	for _, entry := range candidates {
		resolved, loadErr := loadRegistryResolvedImage(entry)
		if loadErr != nil {
			continue
		}
		if manifestContainsDigest(resolved.manifest, ref) {
			return resolved, nil
		}
	}
	return nil, fmt.Errorf("digest not found: %s@%s", repo, ref)
}

func (h *serverHandler) scanRegistryCatalog() ([]registryCatalogEntry, error) {
	imagesRoot := filepath.Join(h.rootAbs, "images")
	if _, err := os.Stat(imagesRoot); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	entries := make([]registryCatalogEntry, 0)
	err := filepath.WalkDir(imagesRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || strings.ToLower(filepath.Ext(path)) != ".tar" {
			return nil
		}
		manifest, err := tarball.LoadManifest(func() (io.ReadCloser, error) { return os.Open(path) })
		if err != nil {
			return nil
		}
		for _, descriptor := range manifest {
			for _, repoTag := range descriptor.RepoTags {
				tag, err := name.NewTag(repoTag, name.WeakValidation)
				if err != nil {
					continue
				}
				aliases := registryRepositoryAliases(tag.Repository.Name())
				for _, alias := range aliases {
					entries = append(entries, registryCatalogEntry{
						repoTag: repoTag,
						repo:    alias,
						tag:     tag.TagStr(),
						tarPath: path,
					})
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].repo == entries[j].repo {
			if entries[i].tag == entries[j].tag {
				return entries[i].tarPath < entries[j].tarPath
			}
			return entries[i].tag < entries[j].tag
		}
		return entries[i].repo < entries[j].repo
	})
	return entries, nil
}

func registryRepositoryAliases(repo string) []string {
	trimmed := strings.TrimSpace(strings.Trim(repo, "/"))
	if trimmed == "" {
		return nil
	}
	aliases := []string{trimmed}
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) == 2 && looksLikeRegistryDomain(parts[0]) {
		aliases = append(aliases, parts[1])
	}
	return dedupeStrings(aliases)
}

func looksLikeRegistryDomain(v string) bool {
	return strings.Contains(v, ".") || strings.Contains(v, ":") || v == "localhost"
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

func loadRegistryResolvedImage(entry registryCatalogEntry) (*registryResolvedImage, error) {
	tag, err := name.NewTag(entry.repoTag, name.WeakValidation)
	if err != nil {
		return nil, err
	}
	imageObj, err := tarball.ImageFromPath(entry.tarPath, &tag)
	if err != nil {
		return nil, err
	}
	rawManifest, err := imageObj.RawManifest()
	if err != nil {
		return nil, err
	}
	manifest, err := imageObj.Manifest()
	if err != nil {
		return nil, err
	}
	digest, err := imageObj.Digest()
	if err != nil {
		return nil, err
	}
	return &registryResolvedImage{
		repo:        entry.repo,
		tag:         entry.tag,
		tarPath:     entry.tarPath,
		image:       imageObj,
		manifest:    manifest,
		rawManifest: rawManifest,
		digest:      digest,
	}, nil
}

func manifestContainsDigest(manifest *v1.Manifest, digest string) bool {
	if manifest == nil {
		return false
	}
	if manifest.Config.Digest.String() == digest {
		return true
	}
	for _, layer := range manifest.Layers {
		if layer.Digest.String() == digest {
			return true
		}
	}
	return false
}

func resolveRegistryBlobContent(resolved *registryResolvedImage, digest string) ([]byte, string, bool, error) {
	if resolved == nil || resolved.manifest == nil {
		return nil, "", false, nil
	}
	if resolved.manifest.Config.Digest.String() == digest {
		body, err := resolved.image.RawConfigFile()
		if err != nil {
			return nil, "", false, err
		}
		mediaType := strings.TrimSpace(string(resolved.manifest.Config.MediaType))
		if mediaType == "" {
			mediaType = "application/octet-stream"
		}
		return body, mediaType, true, nil
	}
	layers, err := resolved.image.Layers()
	if err != nil {
		return nil, "", false, err
	}
	for idx, layer := range layers {
		layerDigest, err := layer.Digest()
		if err != nil {
			return nil, "", false, err
		}
		if layerDigest.String() != digest {
			continue
		}
		body, err := readLayerCompressed(layer)
		if err != nil {
			return nil, "", false, err
		}
		mediaType := "application/octet-stream"
		if idx < len(resolved.manifest.Layers) && strings.TrimSpace(string(resolved.manifest.Layers[idx].MediaType)) != "" {
			mediaType = string(resolved.manifest.Layers[idx].MediaType)
		}
		return body, mediaType, true, nil
	}
	return nil, "", false, nil
}

func readLayerCompressed(layer v1.Layer) ([]byte, error) {
	rc, err := layer.Compressed()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	buf := bytes.Buffer{}
	if _, err := io.Copy(&buf, rc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (h *serverHandler) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	category, relPath, targetPath, status := h.resolveCategoryPath(r.URL.Path)
	if status != 0 {
		w.WriteHeader(status)
		return
	}

	if r.Method == http.MethodPut {
		if err := writeFileAtomic(targetPath, r.Body); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	hash := sha256.Sum256(data)
	etag := fmt.Sprintf("\"sha256:%s\"", hex.EncodeToString(hash[:]))
	setStaticHeaders(w.Header(), category, relPath, etag, len(data), data)

	if matchETag(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func setStaticHeaders(h http.Header, category, relPath, etag string, size int, data []byte) {
	h.Set("Accept-Ranges", "bytes")
	h.Set("Cache-Control", "no-store")
	h.Set("ETag", etag)
	h.Set("Content-Length", strconv.Itoa(size))
	h.Set("Content-Type", http.DetectContentType(data))
	if category == "workflows" && relPath == "index.json" {
		h.Set("Content-Type", "application/json")
	}
}

func matchETag(rawHeader, expected string) bool {
	rawHeader = strings.TrimSpace(rawHeader)
	if rawHeader == "" {
		return false
	}
	if rawHeader == "*" {
		return true
	}
	for _, token := range strings.Split(rawHeader, ",") {
		if strings.TrimSpace(token) == expected {
			return true
		}
	}
	return false
}

func (h *serverHandler) resolveCategoryPath(urlPath string) (string, string, string, int) {
	category := ""
	switch {
	case strings.HasPrefix(urlPath, "/files/"):
		category = "files"
	case strings.HasPrefix(urlPath, "/packages/"):
		category = "packages"
	case strings.HasPrefix(urlPath, "/images/"):
		category = "images"
	case strings.HasPrefix(urlPath, "/workflows/"):
		category = "workflows"
	default:
		return "", "", "", http.StatusNotFound
	}

	relPath := strings.TrimPrefix(urlPath, "/"+category+"/")
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return "", "", "", http.StatusNotFound
	}
	if strings.Contains(relPath, "\\") {
		return "", "", "", http.StatusForbidden
	}
	for _, segment := range strings.Split(relPath, "/") {
		if segment == ".." {
			return "", "", "", http.StatusForbidden
		}
	}
	cleanRel := strings.TrimPrefix(path.Clean("/"+relPath), "/")
	if cleanRel == "." || cleanRel == "" {
		return "", "", "", http.StatusNotFound
	}
	if cleanRel == ".deck" || strings.HasPrefix(cleanRel, ".deck/") || strings.Contains(cleanRel, "/.deck/") {
		return "", "", "", http.StatusForbidden
	}

	targetPath := filepath.Join(h.rootAbs, category, filepath.FromSlash(cleanRel))
	resolvedTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return "", "", "", http.StatusForbidden
	}
	rootPrefix := h.rootAbs + string(os.PathSeparator)
	if resolvedTarget != h.rootAbs && !strings.HasPrefix(resolvedTarget, rootPrefix) {
		return "", "", "", http.StatusForbidden
	}

	return category, cleanRel, resolvedTarget, 0
}

func writeFileAtomic(targetPath string, src io.Reader) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := io.Copy(tmpFile, src); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}
	return nil
}
