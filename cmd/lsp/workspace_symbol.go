package main

import (
	"encoding/json"
	"hash/fnv"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"sec/internal/ast"
	lspserver "sec/internal/lsp/server"
)

type workspaceSymbolParams struct {
	Query string `json:"query"`
}

type workspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

type initializeParams struct {
	RootURI               string            `json:"rootUri"`
	RootPath              string            `json:"rootPath"`
	WorkspaceFolders      []workspaceFolder `json:"workspaceFolders"`
	InitializationOptions json.RawMessage   `json:"initializationOptions"`
}

type didChangeWorkspaceFoldersParams struct {
	Event struct {
		Added   []workspaceFolder `json:"added"`
		Removed []workspaceFolder `json:"removed"`
	} `json:"event"`
}

// symbolInformation is the LSP SymbolInformation answer of workspace/symbol.
type symbolInformation struct {
	Name          string   `json:"name"`
	Kind          int      `json:"kind"`
	Location      location `json:"location"`
	ContainerName string   `json:"containerName,omitempty"`
}

// workspaceSymbolEntry is one declaration of a cached per-file summary.
type workspaceSymbolEntry struct {
	name      string
	kind      int
	container string
	rng       lspRange
}

type workspaceSymbolFile struct {
	key     string
	module  string
	entries []workspaceSymbolEntry
}

// workspaceSymbolIndex caches per-file declaration summaries keyed by the
// file's on-disk identity (size and modification time) or by the hash of the
// open-document snapshot, so a workspace/symbol request only re-parses files
// that changed since the previous request.
type workspaceSymbolIndex struct {
	mu    sync.Mutex
	files map[string]workspaceSymbolFile
}

func newWorkspaceSymbolIndex() *workspaceSymbolIndex {
	return &workspaceSymbolIndex{files: map[string]workspaceSymbolFile{}}
}

// maxWorkspaceSymbols bounds one answer; clients refine with a longer query.
const maxWorkspaceSymbols = 2000

// coreReadOnlyContainerSuffix marks read-only trusted core declarations.
const coreReadOnlyContainerSuffix = " · core (read-only)"

// workspaceSymbolsForQuery answers workspace/symbol: the declarations of
// every Sec source file under the workspace roots and every open document,
// projected from the same declaration facts as textDocument/documentSymbol,
// filtered by a case-insensitive subsequence match and ordered
// deterministically by match quality, then name, container, file, and line.
//
// Visibility follows the editor context, since the request carries no
// document: a module-internal name (`_`) is offered only when an open
// document belongs to its module, and a private name (`__`) only when its
// own source file is open. Read-only trusted core declarations are offered
// after workspace declarations and marked in their container name.
//
// Rules:
//   - rules/tooling/lsp.md — "Workspace symbols"
//   - rules/foundations/names_scopes_visibility.md — §12 visibility prefixes
//   - rules/projects/modules.md — "Source directory and module membership"
func workspaceSymbolsForQuery(index *workspaceSymbolIndex, roots []string, query string, overlay sourceOverlay) []symbolInformation {
	if index == nil {
		index = newWorkspaceSymbolIndex()
	}
	paths := workspaceSourcePaths(roots, overlay)
	openModules := map[string]bool{}
	summaries := make(map[string]workspaceSymbolFile, len(paths))
	for _, path := range paths {
		summary, ok := index.summary(path, overlay)
		if !ok {
			continue
		}
		summaries[path] = summary
		if _, open := overlay.Sources[path]; open && summary.module != "" {
			openModules[summary.module] = true
		}
	}

	type rankedSymbol struct {
		symbolInformation
		score int
		core  bool
		path  string
	}
	ranked := []rankedSymbol{}
	for _, path := range paths {
		summary, ok := summaries[path]
		if !ok {
			continue
		}
		_, open := overlay.Sources[path]
		core := isCoreSourcePath(path)
		for _, entry := range summary.entries {
			switch visibilityPrefix(entry.name) {
			case "":
			case "_":
				if !openModules[summary.module] {
					continue
				}
			default:
				if !open {
					continue
				}
			}
			score, matched := workspaceSymbolMatchScore(entry.name, query)
			if !matched {
				continue
			}
			container := entry.container
			if core {
				container += coreReadOnlyContainerSuffix
			}
			ranked = append(ranked, rankedSymbol{
				symbolInformation: symbolInformation{
					Name:          entry.name,
					Kind:          entry.kind,
					Location:      location{URI: uriFromPath(path), Range: entry.rng},
					ContainerName: container,
				},
				score: score,
				core:  core,
				path:  path,
			})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if a.score != b.score {
			return a.score < b.score
		}
		if a.core != b.core {
			return !a.core
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.ContainerName != b.ContainerName {
			return a.ContainerName < b.ContainerName
		}
		if a.path != b.path {
			return a.path < b.path
		}
		return a.Location.Range.Start.Line < b.Location.Range.Start.Line
	})
	if len(ranked) > maxWorkspaceSymbols {
		ranked = ranked[:maxWorkspaceSymbols]
	}
	symbols := make([]symbolInformation, 0, len(ranked))
	for _, symbol := range ranked {
		symbols = append(symbols, symbol.symbolInformation)
	}
	return symbols
}

// workspaceSymbolMatchScore matches query case-insensitively as a
// subsequence of name; lower scores rank first: exact, prefix, substring,
// then scattered subsequence.
func workspaceSymbolMatchScore(name string, query string) (int, bool) {
	lowerName := strings.ToLower(name)
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	switch {
	case lowerQuery == "":
		return 3, true
	case lowerName == lowerQuery:
		return 0, true
	case strings.HasPrefix(lowerName, lowerQuery):
		return 1, true
	case strings.Contains(lowerName, lowerQuery):
		return 2, true
	}
	remaining := []rune(lowerQuery)
	for _, r := range lowerName {
		if len(remaining) > 0 && r == remaining[0] {
			remaining = remaining[1:]
		}
	}
	return 3, len(remaining) == 0
}

// workspaceSourcePaths lists the normalized .sec paths under the roots plus
// every open document, sorted. Hidden directories, testdata fixtures, and
// node_modules are not workspace source.
func workspaceSourcePaths(roots []string, overlay sourceOverlay) []string {
	seen := map[string]bool{}
	paths := []string{}
	add := func(path string) {
		path = normalizedSourcePath(path)
		if path == "" || filepath.Ext(path) != ".sec" || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}
	for _, root := range roots {
		if root == "" {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				if entry != nil && entry.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				name := entry.Name()
				if path != root && (strings.HasPrefix(name, ".") || name == "testdata" || name == "node_modules") {
					return fs.SkipDir
				}
				return nil
			}
			add(path)
			return nil
		})
	}
	for path := range overlay.Sources {
		add(path)
	}
	sort.Strings(paths)
	return paths
}

// summary returns the cached declaration summary of path, rebuilding it when
// the open snapshot or the on-disk file changed.
func (index *workspaceSymbolIndex) summary(path string, overlay sourceOverlay) (workspaceSymbolFile, bool) {
	var key string
	var text string
	if snapshot, open := overlay.Sources[path]; open {
		hash := fnv.New64a()
		_, _ = hash.Write([]byte(snapshot))
		key = "open:" + strconv.FormatUint(hash.Sum64(), 16)
		text = snapshot
	} else {
		info, err := os.Stat(path)
		if err != nil {
			return workspaceSymbolFile{}, false
		}
		key = "disk:" + strconv.FormatInt(info.Size(), 10) + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
	}
	index.mu.Lock()
	cached, ok := index.files[path]
	index.mu.Unlock()
	if ok && cached.key == key {
		return cached, true
	}
	if text == "" {
		data, err := lspserver.ReadSource(path, overlay.Sources)
		if err != nil {
			return workspaceSymbolFile{}, false
		}
		text = string(data)
	}
	summary := workspaceSymbolSummary(uriFromPath(path), text)
	summary.key = key
	index.mu.Lock()
	index.files[path] = summary
	index.mu.Unlock()
	return summary, true
}

// workspaceSymbolSummary flattens the document-symbol outline of one source
// into named declarations with their container: the module for top-level
// declarations, and the owning type for members, fields, and variants. Impl
// blocks contribute their members under the implemented type; module
// headers, let groups, init, and free are not symbols of their own.
func workspaceSymbolSummary(uri string, text string) workspaceSymbolFile {
	summary := workspaceSymbolFile{}
	program := parseProgramForLSP(uri, text)
	if program == nil {
		return summary
	}
	summary.module = programModulePath(program)
	add := func(symbol documentSymbol, container string) {
		summary.entries = append(summary.entries, workspaceSymbolEntry{
			name:      symbol.Name,
			kind:      symbol.Kind,
			container: container,
			rng:       symbol.SelectionRange,
		})
	}
	var addMembers func(children []documentSymbol, container string)
	addMembers = func(children []documentSymbol, container string) {
		for _, child := range children {
			if child.Name == "init" || child.Name == "free" {
				continue
			}
			add(child, container)
			addMembers(child.Children, child.Name)
		}
	}
	for _, stmt := range program.Statements {
		switch stmt := stmt.(type) {
		case *ast.ModuleStatement:
			continue
		case *ast.ImplStatement:
			if stmt == nil || stmt.Target == nil {
				continue
			}
			for _, member := range stmt.Members {
				if child, ok := documentSymbolForImplMember(text, member); ok {
					addMembers([]documentSymbol{child}, stmt.Target.Name)
				}
			}
			continue
		case *ast.LetGroupStatement:
			if symbol, ok := documentSymbolForStatement(text, stmt); ok {
				addMembers(symbol.Children, summary.module)
			}
			continue
		}
		if symbol, ok := documentSymbolForStatement(text, stmt); ok {
			add(symbol, summary.module)
			addMembers(symbol.Children, symbol.Name)
		}
	}
	return summary
}

// workspaceRootsFromInitialize reads the workspace folders, falling back to
// the root URI or root path of older clients.
func workspaceRootsFromInitialize(params initializeParams) []string {
	roots := []string{}
	for _, folder := range params.WorkspaceFolders {
		if path := pathFromURI(folder.URI); path != "" {
			roots = append(roots, normalizedSourcePath(path))
		}
	}
	if len(roots) == 0 {
		if path := pathFromURI(params.RootURI); path != "" {
			roots = append(roots, normalizedSourcePath(path))
		} else if params.RootPath != "" {
			roots = append(roots, normalizedSourcePath(params.RootPath))
		}
	}
	return roots
}

// changedWorkspaceRoots applies a workspace-folder change event.
func changedWorkspaceRoots(roots []string, params didChangeWorkspaceFoldersParams) []string {
	removed := map[string]bool{}
	for _, folder := range params.Event.Removed {
		removed[normalizedSourcePath(pathFromURI(folder.URI))] = true
	}
	updated := []string{}
	seen := map[string]bool{}
	for _, root := range roots {
		if !removed[root] && !seen[root] {
			seen[root] = true
			updated = append(updated, root)
		}
	}
	for _, folder := range params.Event.Added {
		root := normalizedSourcePath(pathFromURI(folder.URI))
		if root != "" && !seen[root] {
			seen[root] = true
			updated = append(updated, root)
		}
	}
	return updated
}

func isCoreSourcePath(path string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	return strings.Contains(path, "/sec/core/") || strings.HasPrefix(path, "sec/core/")
}
