package main

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"sec/internal/project"
)

type workspaceSourceSelection struct {
	roots  []project.SourceScope
	scopes map[string]project.SourceScope
	failed map[string]bool
}

// newWorkspaceSourceSelection resolves editor folders through the shared
// project model. Opening a project subdirectory selects the whole governing
// project; an invalid manifest cannot enable a standalone-folder fallback.
// Rules: rules/tooling/lsp.md — Project loading, Workspace symbols;
// rules/projects/projects.md — §§4,36(1–3),49.
func newWorkspaceSourceSelection(roots []string) *workspaceSourceSelection {
	selection := &workspaceSourceSelection{scopes: map[string]project.SourceScope{}, failed: map[string]bool{}}
	seen := map[string]bool{}
	for _, root := range roots {
		if root == "" {
			continue
		}
		scope, ok := selection.scope(normalizedSourcePath(root))
		if ok && !seen[scope.Root] {
			seen[scope.Root] = true
			selection.roots = append(selection.roots, scope)
		}
	}
	return selection
}

// scope memoizes immutable project discovery only within the current request.
// Rules: rules/tooling/lsp.md — Document snapshots, Project loading.
func (selection *workspaceSourceSelection) scope(directory string) (project.SourceScope, bool) {
	if scope, ok := selection.scopes[directory]; ok {
		return scope, true
	}
	if selection.failed[directory] {
		return project.SourceScope{}, false
	}
	scope, err := project.DiscoverSourceScope(directory)
	if err != nil {
		selection.failed[directory] = true
		return project.SourceScope{}, false
	}
	selection.scopes[directory] = scope
	return scope, true
}

// contains uses the same manifest boundary and source-location facts for query
// and lazy resolve, including independent projects below an umbrella folder.
// Nested manifests inside a selected project do not create valid subprojects.
// Rules: rules/projects/projects.md — §§4(1–3),13–15,36;
// rules/tooling/lsp.md — Workspace symbols.
func (selection *workspaceSourceSelection) contains(path string) bool {
	if filepath.Ext(path) != ".sec" {
		return false
	}
	for _, root := range selection.roots {
		if !root.Contains(path) || workspaceSourceExcluded(root.Root, path) {
			continue
		}
		owner, ok := selection.scope(filepath.Dir(path))
		if !ok || !owner.Contains(path) {
			continue
		}
		if root.Manifest == "" || root.Manifest == owner.Manifest {
			return true
		}
	}
	return false
}

// workspaceSourceExcluded preserves editor exclusions below the effective
// source root; reserved project storage is owned by project.SourceScope.
// Rules: rules/tooling/lsp.md — Workspace symbols.
func workspaceSourceExcluded(root, path string) bool {
	relative, err := filepath.Rel(root, filepath.Dir(path))
	if err != nil {
		return true
	}
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part != "." && (strings.HasPrefix(part, ".") || part == "testdata" || part == "node_modules") {
			return true
		}
	}
	return false
}

// workspaceSourcePaths enumerates manifest-governed source roots plus explicit
// open documents. Disk traversal only enumerates candidates: the shared project
// model decides root identity and admissible source locations. Each request
// rediscovers manifests so additions, edits and removal affect cached symbols.
// Rules: rules/tooling/lsp.md — Workspace symbols, Project loading;
// rules/projects/projects.md — §§13–15,36,49.
func workspaceSourcePaths(roots []string, overlay sourceOverlay) []string {
	selection := newWorkspaceSourceSelection(roots)
	seen := map[string]bool{}
	paths := []string{}
	add := func(path string) {
		path = normalizedSourcePath(path)
		if path != "" && filepath.Ext(path) == ".sec" && !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	for _, root := range selection.roots {
		_ = filepath.WalkDir(root.Root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				if path != root.Root && workspaceSourceExcluded(root.Root, filepath.Join(path, "source.sec")) {
					return fs.SkipDir
				}
				owner, ok := selection.scope(path)
				if !ok || (owner.Manifest != "" && path == filepath.Join(owner.Root, "bin")) {
					return fs.SkipDir
				}
				return nil
			}
			if selection.contains(path) {
				add(path)
			}
			return nil
		})
	}
	for path := range overlay.Sources {
		add(path)
	}
	sort.Strings(paths)
	return paths
}

// workspaceSymbolSourceInScope rechecks the same source selection without a
// recursive directory walk, preserving explicit open-file inclusion.
// Rules: rules/tooling/lsp.md — Workspace symbols, Project loading.
func workspaceSymbolSourceInScope(path string, roots []string, overlay sourceOverlay) bool {
	if filepath.Ext(path) != ".sec" {
		return false
	}
	if _, open := overlay.Sources[path]; open {
		return true
	}
	return newWorkspaceSourceSelection(roots).contains(path)
}
