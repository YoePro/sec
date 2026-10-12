package main

import (
	"path/filepath"
	"sort"

	"sec/internal/ast"
)

// diagnosticSourceDependencies records files in the compiler-assembled source
// universe, including transitive imports and trusted library provenance. It
// tracks canonical loader results rather than guessing dependencies from names.
// Rules: rules/errors/panic.md — §28(8); rules/memory/allocation.md — §29(5);
// rules/tooling/lsp.md — "Snapshots", "Incremental analysis".
func diagnosticSourceDependencies(program *ast.Program, source string) map[string]bool {
	result := map[string]bool{normalizedSourcePath(source): true}
	for file := range program.SourceProvenance {
		if file != "" {
			result[normalizedSourcePath(file)] = true
		}
	}
	for _, statement := range program.Statements {
		if token, known := lspStatementTokenForSource(statement); known && token.File != "" {
			result[normalizedSourcePath(token.File)] = true
		}
	}
	return result
}

// scheduleSourceDiagnostics invalidates the edited module and every open module
// whose published semantic facts include that source. Before first publication,
// unknown dependency coverage conservatively refreshes open modules. Closing an
// overlay also refreshes dependents so disk contents replace unsaved contracts.
// Newly added siblings also change an imported module's declaration/target set,
// even though their paths could not occur in the previously loaded closure.
// Rules: rules/errors/panic.md — §28(8); rules/memory/allocation.md — §29(5);
// rules/compiler/incremental_compilation.md — §§11–13,44;
// rules/tooling/lsp.md — "Document synchronization", "Incremental analysis".
func (s *server) scheduleSourceDiagnostics(uri string) {
	if s == nil || s.documentSnapshots == nil {
		return
	}
	source := normalizedSourcePath(pathFromURI(uri))
	changedDir := normalizedSourcePath(filepath.Dir(source))
	representatives := map[string]string{changedDir: uri}
	snapshots := s.documentSnapshots.Snapshots()
	s.timerMu.Lock()
	for _, snapshot := range snapshots {
		dir := normalizedSourcePath(filepath.Dir(pathFromURI(snapshot.URI)))
		dependencies, known := s.diagnosticDependencies[dir]
		if !known || sourceDependencyAffected(dependencies, source) {
			representatives[dir] = snapshot.URI
		}
	}
	s.timerMu.Unlock()
	affected := map[string]bool{}
	for dir := range representatives {
		affected[dir] = true
	}
	s.invalidateCrossTargetDirectories(affected)
	dirs := make([]string, 0, len(representatives))
	for dir := range representatives {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		s.scheduleModuleDiagnostics(representatives[dir])
	}
}

// sourceDependencyAffected treats source membership as a set-valued dependency.
// Any sibling of a loaded source can change the module's overloads, contracts,
// or implementations. Using the compiler-loaded directories is a conservative
// module granularity; unrelated directories do not invalidate a known closure.
// Rules: rules/compiler/incremental_compilation.md — §§11(2)–(3),13,44;
// rules/errors/panic.md — §28(8); rules/projects/modules.md — "Source directory and module membership".
func sourceDependencyAffected(dependencies map[string]bool, source string) bool {
	if dependencies[source] {
		return true
	}
	dir := filepath.Dir(source)
	for file := range dependencies {
		if filepath.Dir(file) == dir {
			return true
		}
	}
	return false
}
