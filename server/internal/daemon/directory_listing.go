package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type directoryEntry struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Hidden bool   `json:"hidden"`
}

func listRuntimeDirectory(provider, path string) (resolved string, entries []directoryEntry, err error) {
	entries = []directoryEntry{}
	if path == "" {
		roots, _, err := localSkillRootsForProvider(provider)
		if err != nil {
			return "", nil, err
		}
		for _, root := range roots {
			absolute, err := filepath.Abs(root.path)
			if err != nil {
				return "", nil, err
			}
			info, err := os.Stat(absolute)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return "", nil, err
			}
			if info.IsDir() {
				entries = append(entries, directoryEntry{Name: relativizeHomePath(absolute), Path: absolute, Kind: "folder", Hidden: strings.HasPrefix(filepath.Base(absolute), ".")})
			}
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
		return "", entries, nil
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", nil, fmt.Errorf("resolve user home: %w", err)
		}
		path = home + strings.TrimPrefix(path, "~")
	}
	if !filepath.IsAbs(path) {
		return "", nil, fmt.Errorf("path must be absolute")
	}
	resolved = filepath.Clean(path)
	info, err := os.Stat(resolved)
	if os.IsNotExist(err) {
		return resolved, nil, fmt.Errorf("path does not exist: %s", resolved)
	}
	if err != nil {
		return resolved, nil, err
	}
	if !info.IsDir() {
		return resolved, nil, fmt.Errorf("not a folder: %s", resolved)
	}
	children, err := os.ReadDir(resolved)
	if err != nil {
		return resolved, nil, err
	}
	entries = make([]directoryEntry, 0, len(children))
	for _, child := range children {
		childPath := filepath.Join(resolved, child.Name())
		kind := "file"
		info, err := os.Stat(childPath)
		if err != nil && !os.IsNotExist(err) {
			return resolved, nil, err
		}
		if err == nil && info.IsDir() {
			kind = "folder"
		}
		entries = append(entries, directoryEntry{Name: child.Name(), Path: childPath, Kind: kind, Hidden: strings.HasPrefix(child.Name(), ".")})
	}
	// os.ReadDir returns entries sorted by name.
	return resolved, entries, nil
}

func (d *Daemon) handleDirectoryListing(ctx context.Context, rt Runtime, requestID, path string) {
	d.logger.Info("runtime directory listing requested", "runtime_id", rt.ID, "request_id", requestID, "provider", rt.Provider)
	resolved, entries, err := listRuntimeDirectory(rt.Provider, path)
	payload := map[string]any{"status": "completed", "path": resolved, "entries": entries}
	if err != nil {
		payload = map[string]any{"status": "failed", "error": err.Error()}
	}
	d.reportRuntimeResultWithRetry(ctx, "directory_listing", rt.ID, requestID, func(ctx context.Context) error {
		return d.client.ReportDirectoryListingResult(ctx, rt.ID, requestID, payload)
	})
}
