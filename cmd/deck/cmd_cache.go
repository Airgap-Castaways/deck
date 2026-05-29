package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	ctrllogs "github.com/Airgap-Castaways/deck/internal/logs"
	"github.com/Airgap-Castaways/deck/internal/userdirs"
)

type cacheEntry struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	ModTime   string `json:"mod_time"`
}

func executeCacheList(env *cliEnv, output string) (err error) {
	started := time.Now().UTC()
	entryCount := 0
	defer func() {
		status := "ok"
		if err != nil {
			status = "failed"
		}
		_ = env.verboseCLIEvent(1, ctrllogs.CLIEvent{Component: "cache", Event: "list_completed", Attrs: map[string]any{"status": status, "duration_ms": time.Since(started).Milliseconds(), "entries": entryCount}})
	}()
	resolvedOutput, err := resolveOutputFormat(output)
	if err != nil {
		return err
	}

	root, err := defaultDeckCacheRoot()
	if err != nil {
		return err
	}
	if err := env.verboseCLIEvent(1, ctrllogs.CLIEvent{Component: "cache", Event: "list_requested", Attrs: map[string]any{"root": root, "output": strings.TrimSpace(output)}}); err != nil {
		return err
	}
	entries, err := listCacheEntries(root)
	if err != nil {
		return err
	}
	entryCount = len(entries)
	if err := env.verboseCLIEvent(1, ctrllogs.CLIEvent{Component: "cache", Event: "list_loaded", Attrs: map[string]any{"entries": len(entries)}}); err != nil {
		return err
	}
	if err := env.verboseCLIEvent(2, ctrllogs.CLIEvent{Level: "debug", Component: "cache", Event: "list_summary", Attrs: cacheEntriesSummary(entries)}); err != nil {
		return err
	}
	for idx, entry := range entries {
		if err := env.verboseCLIEvent(3, ctrllogs.CLIEvent{Level: "debug", Component: "cache", Event: "list_entry", Attrs: map[string]any{"entry_index": idx + 1, "entry_count": len(entries), "path": entry.Path, "size_bytes": entry.SizeBytes, "mod_time": entry.ModTime}}); err != nil {
			return err
		}
	}
	if resolvedOutput == "json" {
		enc := env.stdoutJSONEncoder()
		return enc.Encode(entries)
	}
	for _, e := range entries {
		if err := env.stdoutPrintf("%s\t%d\t%s\n", e.Path, e.SizeBytes, e.ModTime); err != nil {
			return err
		}
	}
	return nil
}

func executeCacheClean(env *cliEnv, olderThan string, dryRun bool) (err error) {
	started := time.Now().UTC()
	matchCount := 0
	deletedCount := 0
	defer func() {
		status := "ok"
		if err != nil {
			status = "failed"
		}
		_ = env.verboseCLIEvent(1, ctrllogs.CLIEvent{Component: "cache", Event: "clean_completed", Attrs: map[string]any{"status": status, "duration_ms": time.Since(started).Milliseconds(), "matches": matchCount, "deleted": deletedCount, "dry_run": dryRun}})
	}()
	root, err := defaultDeckCacheRoot()
	if err != nil {
		return err
	}
	if err := env.verboseCLIEvent(1, ctrllogs.CLIEvent{Component: "cache", Event: "clean_requested", Attrs: map[string]any{"root": root, "older_than": strings.TrimSpace(olderThan), "dry_run": dryRun}}); err != nil {
		return err
	}
	cutoff, hasCutoff, err := parseOlderThan(olderThan)
	if err != nil {
		_ = env.verboseCLIEvent(1, ctrllogs.CLIEvent{Level: "error", Component: "cache", Event: "clean_parse_failed", Attrs: map[string]any{"older_than": strings.TrimSpace(olderThan), "error": err, "suggestion": "use a duration such as 24h, 7d, or 30d"}})
		return err
	}
	plan, err := computeCacheCleanPlan(root, cutoff, hasCutoff)
	if err != nil {
		return err
	}
	matchCount = len(plan)
	if err := env.verboseCLIEvent(1, ctrllogs.CLIEvent{Component: "cache", Event: "clean_planned", Attrs: map[string]any{"matches": len(plan)}}); err != nil {
		return err
	}
	if err := env.verboseCLIEvent(2, ctrllogs.CLIEvent{Level: "debug", Component: "cache", Event: "clean_cutoff", Attrs: map[string]any{"has_cutoff": hasCutoff, "cutoff": formatCacheCutoff(cutoff)}}); err != nil {
		return err
	}
	for _, p := range plan {
		if err := env.verboseCLIEvent(2, ctrllogs.CLIEvent{Component: "cache", Event: "clean_match", Attrs: map[string]any{"path": p}}); err != nil {
			return err
		}
		if info, statErr := os.Stat(p); statErr == nil {
			if err := env.verboseCLIEvent(3, ctrllogs.CLIEvent{Level: "debug", Component: "cache", Event: "clean_match_stat", Attrs: map[string]any{"path": p, "is_dir": info.IsDir(), "size_bytes": info.Size(), "mod_time": info.ModTime().UTC().Format(time.RFC3339)}}); err != nil {
				return err
			}
		}
		if err := env.stdoutPrintln(p); err != nil {
			return err
		}
	}
	if dryRun {
		return nil
	}
	for _, p := range plan {
		if err := os.RemoveAll(p); err != nil {
			return fmt.Errorf("delete %s: %w", p, err)
		}
		deletedCount++
	}
	return nil
}

func cacheEntriesSummary(entries []cacheEntry) map[string]any {
	totalBytes := int64(0)
	oldest := ""
	newest := ""
	for _, entry := range entries {
		totalBytes += entry.SizeBytes
		if oldest == "" || entry.ModTime < oldest {
			oldest = entry.ModTime
		}
		if newest == "" || entry.ModTime > newest {
			newest = entry.ModTime
		}
	}
	return map[string]any{"entries": len(entries), "total_bytes": totalBytes, "oldest": displayValueOrDash(oldest), "newest": displayValueOrDash(newest)}
}

func formatCacheCutoff(cutoff time.Time) string {
	if cutoff.IsZero() {
		return "-"
	}
	return cutoff.UTC().Format(time.RFC3339)
}

func defaultDeckCacheRoot() (string, error) {
	root, err := userdirs.CacheRoot()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(root); err == nil {
		return root, nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("stat cache root: %w", err)
	}
	legacyRoot, found, err := resolveLegacyDeckCacheRoot()
	if err != nil {
		return "", err
	}
	if found {
		return legacyRoot, nil
	}
	return root, nil
}

func listCacheEntries(root string) ([]cacheEntry, error) {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return []cacheEntry{}, nil
		}
		return nil, fmt.Errorf("stat cache root: %w", err)
	}
	entries := []cacheEntry{}
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entries = append(entries, cacheEntry{
			Path:      filepath.ToSlash(rel),
			SizeBytes: info.Size(),
			ModTime:   info.ModTime().UTC().Format(time.RFC3339),
		})
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk cache root: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func parseOlderThan(raw string) (time.Time, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, false, nil
	}
	var dur time.Duration
	if strings.HasSuffix(trimmed, "d") {
		n := strings.TrimSuffix(trimmed, "d")
		days, err := strconv.ParseInt(n, 10, 64)
		if err != nil || days < 0 {
			return time.Time{}, false, fmt.Errorf("invalid --older-than: %s", trimmed)
		}
		dur = time.Duration(days) * 24 * time.Hour
	} else {
		parsed, err := time.ParseDuration(trimmed)
		if err != nil || parsed < 0 {
			return time.Time{}, false, fmt.Errorf("invalid --older-than: %s", trimmed)
		}
		dur = parsed
	}
	return time.Now().Add(-dur), true, nil
}

func computeCacheCleanPlan(root string, cutoff time.Time, hasCutoff bool) ([]string, error) {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("stat cache root: %w", err)
	}
	plan := []string{}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read cache root: %w", err)
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		if !hasCutoff {
			plan = append(plan, path)
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if info.ModTime().Before(cutoff) {
			plan = append(plan, path)
		}
	}
	sort.Strings(plan)
	return plan, nil
}
