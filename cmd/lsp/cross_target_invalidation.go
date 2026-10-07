package main

import "path/filepath"

// invalidateCrossTargetDirectories removes cached per-plan diagnostics and
// supersedes in-flight work when its source closure or project plan changes.
// Active-plan reanalysis must never merge allocation facts from the old closure.
// Cross-target recomputation retains its existing open/save scheduling policy.
// Rules: rules/memory/allocation.md — §29(5); rules/tooling/lsp.md — "Snapshots", "Multi-target diagnostics".
func (s *server) invalidateCrossTargetDirectories(dirs map[string]bool) {
	if s == nil || s.crossTarget == nil {
		return
	}
	store := s.crossTarget
	store.mu.Lock()
	defer store.mu.Unlock()
	uris := map[string]bool{}
	for uri := range store.generation {
		uris[uri] = true
	}
	for uri := range store.results {
		uris[uri] = true
	}
	for uri := range uris {
		if dirs[normalizedSourcePath(filepath.Dir(pathFromURI(uri)))] {
			store.generation[uri]++
			delete(store.results, uri)
		}
	}
}
