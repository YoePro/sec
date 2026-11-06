package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"

	"sec/internal/ast"
	"sec/internal/sema"
)

// analysisCache shares one complete document analysis between the read-only
// requests an editor sends for the same document state (semantic tokens,
// inlay hints, code lenses, highlights, hover, ...), so a burst after one edit
// costs one analysis instead of one per request.
//
// Validity is content based: an entry is reused while its identity is
// unchanged — the document text, the text of every open document (any of
// which may be an import), the selected targets, the effective analysis depth
// and the server generation. The generation advances on the messages that
// change state outside that identity (configuration, workspace folders,
// watched files on disk, initialization), so unopened imports, core sources
// on disk and settings cannot go stale. Diagnostics of a module group with a
// single open document share the same entry, so one document state is
// analyzed once for diagnostics and requests together.
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
	overlayKey string
	depth      sema.AnalysisDepth
	program    *ast.Program
	analyzer   *sema.Analyzer
	// importErrors are the module assembly/import errors and semanticErrors
	// what Analyze returned, retained for the diagnostics that share the
	// entry (rules/tooling/lsp.md — "Snapshots").
	importErrors   []sema.Error
	semanticErrors []sema.Error
}

// analysisResult is one shared, read-only analysis of a document state.
type analysisResult struct {
	program        *ast.Program
	analyzer       *sema.Analyzer
	importErrors   []sema.Error
	semanticErrors []sema.Error
}

// overlayIdentity fingerprints every input of an analysis outside the
// document text: the open documents' text and the selected targets.
func overlayIdentity(overlay sourceOverlay) string {
	hash := sha256.New()
	paths := make([]string, 0, len(overlay.Sources))
	for path := range overlay.Sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		fmt.Fprintf(hash, "%d:%s%d:%s", len(path), path, len(overlay.Sources[path]), overlay.Sources[path])
	}
	keys := make([]string, 0, len(overlay.Targets))
	for key := range overlay.Targets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(hash, "target %q=%#v;", key, overlay.Targets[key])
	}
	return hex.EncodeToString(hash.Sum(nil))
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

// advanceAnalysisEpoch invalidates every cached analysis after a change that
// the content identity does not capture (configuration, workspace, files on
// disk). Workers that still hold an entry finish with it; new requests
// analyze the new state.
func advanceAnalysisEpoch() {
	analysisCache.Lock()
	analysisCache.epoch++
	if analysisCache.enabled {
		analysisCache.entries = map[string]*cachedAnalysis{}
	}
	analysisCache.Unlock()
}

// cachedLSPAnalysis returns the prepared and analyzed program of a document,
// reusing the analysis of the same document state, and the function that
// releases it. parsed is the caller's freshly parsed program, used when this
// call performs the analysis.
// Rules: rules/tooling/lsp.md — Configuration, Snapshots;
// rules/compiler/compiler_analysis.md — §§55(2),60(5).
func cachedLSPAnalysis(uri string, text string, parsed *ast.Program, overlay sourceOverlay) (*ast.Program, *sema.Analyzer, func()) {
	result, release := cachedLSPAnalysisResult(uri, text, parsed, overlay)
	return result.program, result.analyzer, release
}

// cachedLSPAnalysisResult is cachedLSPAnalysis with the analysis errors, for
// diagnostics that share the entry.
func cachedLSPAnalysisResult(uri string, text string, parsed *ast.Program, overlay sourceOverlay) (analysisResult, func()) {
	overlayKey := overlayIdentity(overlay)
	depth := lspAnalysisDepthWithOverlay(pathFromURI(uri), overlay)
	analysisCache.Lock()
	if !analysisCache.enabled {
		analysisCache.Unlock()
		return analyzeLSPDocumentResult(uri, parsed, overlay, depth), func() {}
	}
	key := uri
	entry, ok := analysisCache.entries[key]
	if ok && (entry.epoch != analysisCache.epoch || entry.text != text || entry.overlayKey != overlayKey || entry.depth != depth) {
		ok = false
	}
	if !ok {
		entry = &cachedAnalysis{epoch: analysisCache.epoch, text: text, overlayKey: overlayKey, depth: depth}
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
		result := analyzeLSPDocumentResult(uri, parsed, overlay, depth)
		entry.program, entry.analyzer = result.program, result.analyzer
		entry.importErrors, entry.semanticErrors = result.importErrors, result.semanticErrors
		entry.ready = true
		completed = true
		return result, entry.mu.Unlock
	}
	analysisCache.Unlock()
	entry.mu.Lock()
	if !entry.ready {
		// The analysis this request waited for failed; analyze privately.
		entry.mu.Unlock()
		return analyzeLSPDocumentResult(uri, parsed, overlay, depth), func() {}
	}
	return analysisResult{program: entry.program, analyzer: entry.analyzer, importErrors: entry.importErrors, semanticErrors: entry.semanticErrors}, entry.mu.Unlock
}

// forgetCachedAnalysis drops a closed document's entry.
func forgetCachedAnalysis(uri string) {
	analysisCache.Lock()
	if analysisCache.entries != nil {
		delete(analysisCache.entries, uri)
	}
	analysisCache.Unlock()
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
	result := analyzeLSPDocumentResult(uri, parsed, overlay, depth)
	return result.program, result.analyzer
}

// analyzeLSPDocumentResult assembles and analyzes a document exactly as its
// single-document diagnostics batch does, keeping both error sets.
// Rules: rules/tooling/lsp.md — "Snapshots"; rules/analysis/pitfall_analysis.md — "Analysis states".
func analyzeLSPDocumentResult(uri string, parsed *ast.Program, overlay sourceOverlay, depth sema.AnalysisDepth) analysisResult {
	importErrors := prepareProgramForLSP(parsed, pathFromURI(uri), overlay)
	analyzer := newLSPAnalyzerForInputsAtDepth(uri, parsed, overlay, depth)
	analyzer.SetPitfallSourcePriority([]string{pathFromURI(uri)})
	semanticErrors := analyzer.Analyze(parsed)
	return analysisResult{program: parsed, analyzer: analyzer, importErrors: importErrors, semanticErrors: semanticErrors}
}

// analysisInputsChangeOutsideIdentity reports a message that may change an
// analysis input the cache identity does not fingerprint: settings, workspace
// folders, files on disk (unopened imports, core, manifests) or a command such
// as target selection. Document synchronization changes the fingerprinted
// text and needs no epoch.
func analysisInputsChangeOutsideIdentity(method string) bool {
	return stateMutatingMethod(method) || method == "workspace/executeCommand"
}
