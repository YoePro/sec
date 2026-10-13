package main

import (
	"sync"

	"sec/internal/ast"
	"sec/internal/sema"
)

// analysisCache shares one complete document analysis between the read-only
// requests an editor sends for the same document state (semantic tokens,
// inlay hints, code lenses, highlights, hover, ...), so a burst after one edit
// costs one analysis instead of one per request.
//
// Validity is conservative: the server advances the epoch for every message
// that is not a read-only analysis request (document synchronization, saves,
// configuration, target selection, workspace changes), so an entry is reused
// only between two such messages and only for the same document text and
// overlay presence and effective analysis depth. Budget changes cannot reuse
// results at an incompatible depth, even before a configuration notification.
// The cache is enabled only while the server loop runs;
// direct calls (tests, tooling) always analyze afresh.
//
// Concurrency: read-only requests run on worker goroutines. Each entry has its
// own lock, held from the start of its analysis until the caller releases it,
// so one document state is analyzed once even when several workers ask for it
// at the same time, and an analyzer (which is not safe for concurrent use) is
// used by one worker at a time. Callers treat the program and analyzer as
// read-only and call release when they no longer use them.
//
// Rules:
//   - rules/tooling/lsp.md — "Responsiveness model", "Snapshots"
var analysisCache = struct {
	sync.Mutex
	enabled bool
	epoch   uint64
	entries map[string]*cachedAnalysis
}{}

type cachedAnalysis struct {
	mu         sync.Mutex
	ready      bool
	epoch      uint64
	text       string
	hasOverlay bool
	depth      sema.AnalysisDepth
	program    *ast.Program
	analyzer   *sema.Analyzer
}

// enableAnalysisCache turns the shared analysis cache on for the server loop.
func enableAnalysisCache() {
	analysisCache.Lock()
	analysisCache.enabled = true
	analysisCache.entries = map[string]*cachedAnalysis{}
	analysisCache.Unlock()
}

// disableAnalysisCache turns the cache off and drops every entry when the
// server loop ends.
func disableAnalysisCache() {
	analysisCache.Lock()
	analysisCache.enabled = false
	analysisCache.entries = nil
	analysisCache.Unlock()
}

// advanceAnalysisEpoch invalidates every cached analysis. Workers that still
// hold an entry finish with it; new requests analyze the new state.
func advanceAnalysisEpoch() {
	analysisCache.Lock()
	analysisCache.epoch++
	if analysisCache.enabled {
		analysisCache.entries = map[string]*cachedAnalysis{}
	}
	analysisCache.Unlock()
}

// cachedLSPAnalysis returns the prepared and analyzed program of a document,
// reusing the analysis of the same document state within the current epoch,
// and the function that releases it. parsed is the caller's freshly parsed
// program, used when this call performs the analysis.
// Rules: rules/tooling/lsp.md — Configuration, Snapshots;
// rules/compiler/compiler_analysis.md — §§55(2),60(5).
func cachedLSPAnalysis(uri string, text string, parsed *ast.Program, overlay sourceOverlay) (*ast.Program, *sema.Analyzer, func()) {
	hasOverlay := len(overlay.Sources) > 0 || len(overlay.Targets) > 0
	depth := lspAnalysisDepthWithOverlay(pathFromURI(uri), overlay)
	analysisCache.Lock()
	if !analysisCache.enabled {
		analysisCache.Unlock()
		program, analyzer := analyzeLSPDocumentAtDepth(uri, parsed, overlay, depth)
		return program, analyzer, func() {}
	}
	key := uri
	entry, ok := analysisCache.entries[key]
	if ok && (entry.epoch != analysisCache.epoch || entry.text != text || entry.hasOverlay != hasOverlay || entry.depth != depth) {
		ok = false
	}
	if !ok {
		entry = &cachedAnalysis{epoch: analysisCache.epoch, text: text, hasOverlay: hasOverlay, depth: depth}
		entry.mu.Lock()
		analysisCache.entries[key] = entry
		analysisCache.Unlock()
		completed := false
		defer func() {
			if !completed {
				// A panicking analysis must not leave waiters blocked.
				analysisCache.Lock()
				if analysisCache.entries[key] == entry {
					delete(analysisCache.entries, key)
				}
				analysisCache.Unlock()
				entry.mu.Unlock()
			}
		}()
		entry.program, entry.analyzer = analyzeLSPDocumentAtDepth(uri, parsed, overlay, depth)
		entry.ready = true
		completed = true
		return entry.program, entry.analyzer, entry.mu.Unlock
	}
	analysisCache.Unlock()
	entry.mu.Lock()
	if !entry.ready {
		// The analysis this request waited for failed; analyze privately.
		entry.mu.Unlock()
		program, analyzer := analyzeLSPDocumentAtDepth(uri, parsed, overlay, depth)
		return program, analyzer, func() {}
	}
	return entry.program, entry.analyzer, entry.mu.Unlock
}

// analyzeLSPDocument selects the request's project budget and runs shared Sema.
// Rules: rules/tooling/lsp.md — Configuration, Snapshots;
// rules/compiler/compiler_analysis.md — §§60(1–5),62(1–3).
func analyzeLSPDocument(uri string, parsed *ast.Program, overlay sourceOverlay) (*ast.Program, *sema.Analyzer) {
	return analyzeLSPDocumentAtDepth(uri, parsed, overlay, lspAnalysisDepthWithOverlay(pathFromURI(uri), overlay))
}

// analyzeLSPDocumentAtDepth retains the exact budget selected for the request
// throughout compilation and cache publication.
// Rules: rules/tooling/lsp.md — Configuration, Snapshots;
// rules/compiler/compiler_analysis.md — §§55(2),60(1–5),62(1–3).
func analyzeLSPDocumentAtDepth(uri string, parsed *ast.Program, overlay sourceOverlay, depth sema.AnalysisDepth) (*ast.Program, *sema.Analyzer) {
	prepareProgramForLSP(parsed, pathFromURI(uri), overlay)
	analyzer := newLSPAnalyzerForInputsAtDepth(uri, parsed, overlay, depth)
	analyzer.Analyze(parsed)
	return parsed, analyzer
}
