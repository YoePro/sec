package main

import (
	"encoding/json"
	"path/filepath"
	"strings"

	lspserver "sec/internal/lsp/server"
)

// workspaceSymbol carries the LSP 3.17+ lazy location and round-trip data.
// Rules: rules/tooling/lsp.md — "Workspace symbols", "Protocol transport and types".
type workspaceSymbol struct {
	Name          string                      `json:"name"`
	Kind          int                         `json:"kind"`
	Tags          []int                       `json:"tags,omitempty"`
	ContainerName string                      `json:"containerName,omitempty"`
	Location      workspaceSymbolLocation     `json:"location"`
	Data          *workspaceSymbolResolveData `json:"data,omitempty"`
}

type workspaceSymbolLocation struct {
	URI   string    `json:"uri"`
	Range *lspRange `json:"range,omitempty"`
}

type workspaceSymbolResolveData struct {
	Version int    `json:"version"`
	Key     string `json:"key"`
	Entry   int    `json:"entry"`
}

// unresolvedSymbols uses cached declaration provenance to defer only the range
// clients advertised as resolvable. Equal names and overloads stay distinct by
// entry and snapshot; no second semantic analysis or name lookup is introduced.
// Rules: rules/tooling/lsp.md — "Workspace symbols", "Document snapshots".
func (index *workspaceSymbolIndex) unresolvedSymbols(symbols []symbolInformation) []workspaceSymbol {
	result := make([]workspaceSymbol, 0, len(symbols))
	index.mu.Lock()
	defer index.mu.Unlock()
	for _, symbol := range symbols {
		rng := symbol.Location.Range
		item := workspaceSymbol{Name: symbol.Name, Kind: symbol.Kind, ContainerName: symbol.ContainerName,
			Location: workspaceSymbolLocation{URI: symbol.Location.URI, Range: &rng}}
		summary := index.files[normalizedSourcePath(pathFromURI(symbol.Location.URI))]
		for i, entry := range summary.entries {
			if entry.name == symbol.Name && entry.kind == symbol.Kind && entry.rng == rng {
				item.Location.Range = nil
				item.Data = &workspaceSymbolResolveData{Version: 1, Key: summary.fingerprint, Entry: i}
				break
			}
		}
		result = append(result, item)
	}
	return result
}

// resolveWorkspaceSymbol validates the original declaration against the current
// workspace and source snapshot. It never reads an unindexed client-supplied
// file or relocates a stale overload by matching only its name.
// Rules: rules/tooling/lsp.md — "Workspace symbols", "Document snapshots";
// rules/foundations/names_scopes_visibility.md — §12 visibility prefixes.
func (index *workspaceSymbolIndex) resolveWorkspaceSymbol(symbol workspaceSymbol, roots []string, overlay sourceOverlay) (workspaceSymbol, int, string) {
	data := symbol.Data
	if index == nil || data == nil || data.Version != 1 || data.Key == "" || data.Entry < 0 {
		return symbol, -32602, "invalid workspace symbol resolve data"
	}
	path := normalizedSourcePath(pathFromURI(symbol.Location.URI))
	if path == "" || uriFromPath(path) != symbol.Location.URI || !workspaceSymbolSourceInScope(path, roots, overlay) {
		return symbol, -32602, "workspace symbol is outside the current workspace"
	}
	index.mu.Lock()
	_, known := index.files[path]
	index.mu.Unlock()
	if !known {
		return symbol, -32602, "workspace symbol has no declaration summary"
	}
	summary, ok := index.summary(path, overlay)
	if !ok || summary.fingerprint != data.Key {
		return symbol, -32801, "workspace symbol source changed; repeat workspace/symbol"
	}
	// Resolution reads only the selected source. Content validation also catches
	// edits which preserve the disk summary cache's size and modification time.
	text, err := lspserver.ReadSource(path, overlay.Sources)
	if err != nil || sourceContentFingerprint(string(text)) != data.Key {
		index.mu.Lock()
		if cached, exists := index.files[path]; exists && cached.key == summary.key {
			delete(index.files, path)
		}
		index.mu.Unlock()
		return symbol, -32801, "workspace symbol source changed; repeat workspace/symbol"
	}
	if data.Entry >= len(summary.entries) {
		return symbol, -32602, "invalid workspace symbol declaration"
	}
	entry := summary.entries[data.Entry]
	container := entry.container
	if isCoreSourcePath(path) {
		container += coreReadOnlyContainerSuffix
	}
	if entry.name != symbol.Name || entry.kind != symbol.Kind || container != symbol.ContainerName ||
		(symbol.Location.Range != nil && *symbol.Location.Range != entry.rng) {
		return symbol, -32602, "workspace symbol does not match its declaration"
	}
	switch visibilityPrefix(entry.name) {
	case "":
	case "_":
		visible := false
		for openPath := range overlay.Sources {
			if filepath.Ext(openPath) != ".sec" {
				continue
			}
			if open, ok := index.summary(openPath, overlay); ok && open.module == summary.module && open.module != "" {
				visible = true
				break
			}
		}
		if !visible {
			return symbol, -32602, "workspace symbol is no longer visible"
		}
	default:
		if _, open := overlay.Sources[path]; !open {
			return symbol, -32602, "workspace symbol is no longer visible"
		}
	}
	symbol.Location.Range = &entry.rng
	return symbol, 0, ""
}

// workspaceSymbolSourceInScope applies the same roots, exclusions and open-file
// inclusion as workspaceSourcePaths without traversing the workspace on resolve.
// Rules: rules/tooling/lsp.md — "Workspace symbols", "Project loading".
func workspaceSymbolSourceInScope(path string, roots []string, overlay sourceOverlay) bool {
	if filepath.Ext(path) != ".sec" {
		return false
	}
	if _, open := overlay.Sources[path]; open {
		return true
	}
	for _, root := range roots {
		if root == "" {
			continue
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			continue
		}
		parts := strings.Split(relative, string(filepath.Separator))
		excluded := false
		for _, part := range parts[:len(parts)-1] {
			if strings.HasPrefix(part, ".") || part == "testdata" || part == "node_modules" {
				excluded = true
				break
			}
		}
		if !excluded {
			return true
		}
	}
	return false
}

// handleWorkspaceSymbolResolve returns the exact cached declaration range or a
// protocol error for malformed, stale, or no-longer-visible items.
// Rules: rules/tooling/lsp.md — "Workspace symbols", "Protocol transport and types".
func (s *server) handleWorkspaceSymbolResolve(message rpcMessage) error {
	var symbol workspaceSymbol
	if err := json.Unmarshal(message.Params, &symbol); err != nil {
		return s.respondError(message.ID, -32602, "invalid workspace symbol")
	}
	resolved, code, reason := s.workspaceSymbols.resolveWorkspaceSymbol(symbol, s.workspaceRoots, s.sourceOverlay())
	if code != 0 {
		return s.respondError(message.ID, code, reason)
	}
	return s.respond(message.ID, resolved)
}
