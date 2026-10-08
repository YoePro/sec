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
// and effective target, and routes diagnostics back to their source documents. Parser diagnostics and
// recovery suppression remain local to the document that produced them.
// Rules: rules/tooling/lsp.md — "Responsiveness model", "Recovery nodes";
// rules/projects/modules.md — "Source directory and module membership";
// rules/platform/platform_model.md — source selection by target.
func analyzeDiagnosticBatch(snapshots []lspserver.Snapshot, overlay sourceOverlay) map[string][]diagnostic {
	return analyzeDiagnosticBatchWithPolicy(snapshots, overlay, pitfallDiagnosticSettings{})
}

// analyzeDiagnosticBatchWithPolicy applies one immutable optional diagnostic policy
// to a compiler batch. Rules: rules/analysis/pitfall_analysis.md — "LSP configuration reload".
func analyzeDiagnosticBatchWithPolicy(snapshots []lspserver.Snapshot, overlay sourceOverlay, policy pitfallDiagnosticSettings) map[string][]diagnostic {
	return analyzeDiagnosticBatchWithDependencies(snapshots, overlay, policy, nil)
}

// analyzeDiagnosticBatchWithDependencies returns compiler source dependencies alongside current diagnostics.
// Rules: rules/memory/allocation.md — §29(5); rules/tooling/lsp.md — "Snapshots", "Incremental analysis".
func analyzeDiagnosticBatchWithDependencies(snapshots []lspserver.Snapshot, overlay sourceOverlay, policy pitfallDiagnosticSettings, dependencies map[string]map[string]bool) map[string][]diagnostic {
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
			results[snapshot.URI] = append(results[snapshot.URI], structuredParserDiagnostic(value, snapshot.Text))
		}
		if parsed.Fatal {
			continue
		}
		module := programModulePath(parsed.Program)
		// One Sema run serves every document of the same directory, module,
		// and effective target. The module is assembled for the first
		// document's target, so platform files for different targets (two
		// `#target` files of one module) must not share a run: the second
		// would be excluded from the assembly and receive no diagnostics.
		key := normalizedSourcePath(filepath.Dir(path)) + "\x00" + module + "\x00" + lspActiveTarget(parsed.Program, path, overlay).String()
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
		importErrors := prepareProgramForLSP(first.program, pathFromURI(first.snapshot.URI), overlay)
		if dependencies != nil {
			for _, doc := range docs {
				if len(importErrors) == 0 {
					dependencies[doc.snapshot.URI] = diagnosticSourceDependencies(first.program, pathFromURI(doc.snapshot.URI))
				} else {
					dependencies[doc.snapshot.URI] = nil // unresolved source closure requires conservative refresh
				}
			}
		}
		analyzer := newLSPAnalyzerWithOverlay(first.snapshot.URI, first.program, overlay)
		paths := make([]string, 0, len(docs))
		for _, doc := range docs {
			paths = append(paths, pathFromURI(doc.snapshot.URI))
		}
		analyzer.SetPitfallSourcePriority(paths)
		errors := append(importErrors, analyzer.Analyze(first.program)...)
		errors = append(errors, lspProgramEntryErrors(analyzer, first.program, pathFromURI(first.snapshot.URI), first.snapshot.Text, overlay)...)
		for _, doc := range docs {
			path := pathFromURI(doc.snapshot.URI)
			for _, err := range errors {
				if diagnosticBelongsToSource(err, path) && !semanticDiagnosticComesFromRecovery(err, doc.recovery, doc.snapshot.Text) {
					results[doc.snapshot.URI] = append(results[doc.snapshot.URI], semaDiagnosticWithSources(err, 1, doc.snapshot.URI, doc.snapshot.Text, overlay))
				}
			}
			results[doc.snapshot.URI] = append(results[doc.snapshot.URI], pitfallDiagnostics(analyzer, doc.snapshot.URI, doc.snapshot.Text, overlay, policy)...)
			for _, warning := range analyzer.Warnings() {
				if diagnosticBelongsToSource(warning, path) {
					results[doc.snapshot.URI] = append(results[doc.snapshot.URI], semaDiagnosticWithSources(warning, 2, doc.snapshot.URI, doc.snapshot.Text, overlay))
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
	overlay := s.sourceOverlay()
	for _, snapshot := range all {
		path := pathFromURI(snapshot.URI)
		overlay.Sources[normalizedSourcePath(path)] = snapshot.Text
		if normalizedSourcePath(filepath.Dir(path)) == dir {
			selected = append(selected, snapshot)
		}
	}
	if len(selected) == 0 {
		return nil
	}
	dependencies := map[string]map[string]bool{}
	results := analyzeDiagnosticBatchWithDependencies(selected, overlay, s.pitfallSettingsSnapshot(), dependencies)
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	if !current() {
		return nil
	}
	latest := s.documentSnapshots.Snapshots()
	if len(latest) != len(all) {
		s.scheduleDiagnosticJobLocked(dir)
		return nil
	}
	for _, snapshot := range all {
		now, ok := s.documentSnapshots.Snapshot(snapshot.URI)
		if !ok || now != snapshot {
			s.scheduleDiagnosticJobLocked(dir)
			return nil
		}
	}
	if s.diagnosticDependencies == nil {
		s.diagnosticDependencies = map[string]map[string]bool{}
	}
	files := map[string]bool{}
	complete := true
	for _, snapshot := range selected {
		if dependencies[snapshot.URI] == nil {
			complete = false
		}
		for file := range dependencies[snapshot.URI] {
			files[file] = true
		}
	}
	if complete {
		s.diagnosticDependencies[dir] = files
	} else {
		delete(s.diagnosticDependencies, dir)
	}
	for _, snapshot := range selected {
		published := results[snapshot.URI]
		if crossTarget, ok := s.crossTargetResultFor(snapshot.URI, snapshot.Text); ok {
			published = mergeCrossTargetDiagnostics(published, crossTarget)
		}
		published = parameterInsightDiagnostics(published, s.parameterInsight)
		if err := s.notify("textDocument/publishDiagnostics", map[string]any{
			"uri": snapshot.URI, "version": snapshot.Version, "diagnostics": published,
		}); err != nil {
			return err
		}
	}
	return nil
}
