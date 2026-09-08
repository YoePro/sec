package main

import (
	"path/filepath"
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
	lspserver "sec/internal/lsp/server"
	"sec/internal/parser"
)

// analyzeDiagnosticBatch runs Sema once for each represented directory/module
// and routes diagnostics back to their source documents. Parser diagnostics and
// recovery suppression remain local to the document that produced them.
// Rules: rules/tooling/lsp.md — "Responsiveness model", "Recovery nodes";
// rules/projects/modules.md — "Source directory and module membership".
func analyzeDiagnosticBatch(snapshots []lspserver.Snapshot, overlay sourceOverlay) map[string][]diagnostic {
	results := make(map[string][]diagnostic, len(snapshots))
	type document struct {
		snapshot lspserver.Snapshot
		program  *ast.Program
		recovery []parser.RecoveryEvent
	}
	groups := map[string][]document{}
	for _, snapshot := range snapshots {
		path := pathFromURI(snapshot.URI)
		parsed := parser.New(lexer.NewWithFile(snapshot.Text, path)).Parse()
		results[snapshot.URI] = []diagnostic{}
		for _, value := range parsed.Diagnostics {
			results[snapshot.URI] = append(results[snapshot.URI], structuredParserDiagnostic(value))
		}
		if parsed.Fatal {
			continue
		}
		module := programModulePath(parsed.Program)
		key := normalizedSourcePath(filepath.Dir(path)) + "\x00" + module
		if module == "" {
			key = snapshot.URI
		}
		groups[key] = append(groups[key], document{snapshot, parsed.Program, parsed.Recovery})
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		docs := groups[key]
		sort.Slice(docs, func(i, j int) bool { return docs[i].snapshot.URI < docs[j].snapshot.URI })
		first := docs[0]
		prepareProgramForLSP(first.program, pathFromURI(first.snapshot.URI), overlay)
		analyzer := newLSPAnalyzer(first.snapshot.URI)
		errors := analyzer.Analyze(first.program)
		for _, doc := range docs {
			path := pathFromURI(doc.snapshot.URI)
			for _, err := range errors {
				if diagnosticBelongsToSource(err, path) && !semanticDiagnosticComesFromRecovery(err, doc.recovery) {
					results[doc.snapshot.URI] = append(results[doc.snapshot.URI], semaDiagnostic(err, 1))
				}
			}
			for _, warning := range analyzer.Warnings() {
				if diagnosticBelongsToSource(warning, path) {
					results[doc.snapshot.URI] = append(results[doc.snapshot.URI], semaDiagnostic(warning, 2))
				}
			}
		}
	}
	return results
}

// publishDiagnosticBatch serializes diagnostic work and discards an obsolete
// module job before analysis or publication. A snapshot change in any open file
// also invalidates the result, since that file may be an imported dependency.
// Rules: rules/tooling/lsp.md — "Responsiveness model", "Document synchronization".
func (s *server) publishDiagnosticBatch(dir string, generation uint64) error {
	s.diagnosticWorkMu.Lock()
	defer s.diagnosticWorkMu.Unlock()
	current := func() bool {
		return !s.diagnosticsStopped && s.diagnosticGeneration[dir] == generation
	}
	s.timerMu.Lock()
	valid := current()
	s.timerMu.Unlock()
	if !valid {
		return nil
	}
	all := s.documentSnapshots.Snapshots()
	selected := []lspserver.Snapshot{}
	overlay := sourceOverlay{}
	for _, snapshot := range all {
		path := pathFromURI(snapshot.URI)
		overlay[normalizedSourcePath(path)] = snapshot.Text
		if normalizedSourcePath(filepath.Dir(path)) == dir {
			selected = append(selected, snapshot)
		}
	}
	if len(selected) == 0 {
		return nil
	}
	results := analyzeDiagnosticBatch(selected, overlay)
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	if !current() {
		return nil
	}
	latest := s.documentSnapshots.Snapshots()
	if len(latest) != len(all) {
		return nil
	}
	for _, snapshot := range all {
		now, ok := s.documentSnapshots.Snapshot(snapshot.URI)
		if !ok || now != snapshot {
			return nil
		}
	}
	for _, snapshot := range selected {
		if err := s.notify("textDocument/publishDiagnostics", map[string]any{
			"uri": snapshot.URI, "version": snapshot.Version, "diagnostics": results[snapshot.URI],
		}); err != nil {
			return err
		}
	}
	return nil
}
