package main

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
	"sync"

	lspserver "sec/internal/lsp/server"
	platformtarget "sec/internal/platform/target"
	"sec/internal/sema"
)

// crossTargetResult holds, for one document text, the diagnostics of every
// target the document's module is built for.
type crossTargetResult struct {
	textHash  string
	active    string
	targets   []string
	perTarget map[string][]diagnostic
}

type crossTargetStore struct {
	mu         sync.Mutex
	results    map[string]crossTargetResult
	generation map[string]uint64
}

func newCrossTargetStore() *crossTargetStore {
	return &crossTargetStore{results: map[string]crossTargetResult{}, generation: map[string]uint64{}}
}

func documentTextHash(text string) string {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(text))
	return strconv.FormatUint(hash.Sum64(), 16)
}

// scheduleCrossTargetDiagnostics analyzes a target-independent document once
// per project variant or target of its module in the background and republishes the module's
// diagnostics when done. It runs on open and save only, because one pass
// analyzes the module for every target. Configuration/target switches also
// reschedule these analyses with the new immutable request selection.
//
// Rules:
//   - rules/tooling/lsp.md — "Multi-target diagnostics", "Target status" (per-target diagnostics)
//   - rules/platform/platform_model.md — diagnostics retain Target identity
func (s *server) scheduleCrossTargetDiagnostics(uri string) {
	if s == nil || s.documentSnapshots == nil {
		return
	}
	snapshot, ok := s.documentSnapshots.Snapshot(uri)
	if !ok {
		return
	}
	if s.crossTarget == nil {
		s.crossTarget = newCrossTargetStore()
	}
	store := s.crossTarget
	store.mu.Lock()
	store.generation[uri]++
	generation := store.generation[uri]
	store.mu.Unlock()
	overlay := s.sourceOverlay()
	go func() {
		result, ok := computeCrossTargetDiagnostics(uri, snapshot.Text, overlay)
		store.mu.Lock()
		if store.generation[uri] != generation {
			store.mu.Unlock()
			return
		}
		if ok {
			store.results[uri] = result
		} else {
			delete(store.results, uri)
		}
		store.mu.Unlock()
		if ok {
			s.scheduleModuleDiagnostics(uri)
		}
	}()
}

// crossTargetResultFor returns the stored result when it still belongs to
// text.
func (s *server) crossTargetResultFor(uri string, text string) (crossTargetResult, bool) {
	if s == nil || s.crossTarget == nil {
		return crossTargetResult{}, false
	}
	s.crossTarget.mu.Lock()
	defer s.crossTarget.mu.Unlock()
	result, ok := s.crossTarget.results[uri]
	if !ok || result.textHash != documentTextHash(text) {
		return crossTargetResult{}, false
	}
	return result, true
}

// computeCrossTargetDiagnostics analyzes a document without its own `#target`
// for every declared project variant, or targets selected by platform files
// when no project variants apply. It reports false
// when the document is target-specific or its module has fewer than two
// targets, where the ordinary active-target diagnostics are complete.
func computeCrossTargetDiagnostics(uri string, text string, overlay sourceOverlay) (result crossTargetResult, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	path := pathFromURI(uri)
	active := parseProgramForLSP(uri, text)
	if active == nil || path == "" {
		return crossTargetResult{}, false
	}
	if _, directed := lspserver.ProgramTarget(active); directed {
		return crossTargetResult{}, false
	}
	if options := lspDiagnosticTargetOptions(path, overlay); len(options) >= 2 {
		result = crossTargetResult{textHash: documentTextHash(text), active: lspActiveTarget(active, path, overlay).String(), perTarget: map[string][]diagnostic{}}
		for _, option := range options {
			name := option.diagnosticName()
			result.targets = append(result.targets, name)
			target := platformtarget.Target{OS: option.OS, Arch: option.Arch}
			result.perTarget[name] = semanticDiagnosticsForTarget(uri, text, overlay, target)
			if option.Active {
				result.active = name
			}
		}
		return result, true
	}
	targets, _ := moduleTargets(path, lspserver.ProgramModule(active), text, overlay, active)
	if len(targets) < 2 {
		return crossTargetResult{}, false
	}
	result = crossTargetResult{
		textHash:  documentTextHash(text),
		active:    lspActiveTarget(active, path, overlay).String(),
		perTarget: map[string][]diagnostic{},
	}
	for _, target := range targets {
		result.targets = append(result.targets, target.String())
		result.perTarget[target.String()] = semanticDiagnosticsForTarget(uri, text, overlay, target)
	}
	return result, true
}

// semanticDiagnosticsForTarget runs Sema for one target and keeps the
// diagnostics that belong to the document.
func semanticDiagnosticsForTarget(uri string, text string, overlay sourceOverlay, target platformtarget.Target) []diagnostic {
	path := pathFromURI(uri)
	parsed := parseProgramForLSP(uri, text)
	if parsed == nil {
		return nil
	}
	lspserver.AssembleModuleForTarget(parsed, path, overlay.Sources, target)
	resolveCoreSources(parsed, path, overlay)
	errors := resolveSourceImportsForTarget(parsed, map[string]bool{}, path, target, overlay)
	analyzer := newLSPAnalyzerWithOverlay(uri, parsed, overlay)
	if definition, found := platformtarget.Find(target); found {
		if plan, err := definition.ScalarPlan(); err == nil {
			analyzer = sema.NewAnalyzerWithScalarPlanAndDepth(plan, sema.AnalysisInteractive)
		}
	}
	errors = append(errors, analyzer.Analyze(parsed)...)
	out := []diagnostic{}
	for _, err := range errors {
		if diagnosticBelongsToSource(err, path) {
			out = append(out, semaDiagnosticWithSources(err, 1, uri, text, overlay))
		}
	}
	for _, warning := range analyzer.Warnings() {
		if diagnosticBelongsToSource(warning, path) {
			out = append(out, semaDiagnosticWithSources(warning, 2, uri, text, overlay))
		}
	}
	return out
}

func crossTargetKey(item diagnostic) string {
	message := item.Message
	if index := strings.Index(message, "\n"); index >= 0 {
		message = message[:index]
	}
	return fmt.Sprintf("%s|%d:%d|%s", item.Code, item.Range.Start.Line, item.Range.Start.Character, message)
}

// mergeCrossTargetDiagnostics records applicability: a diagnostic on every
// target is kept unchanged; one on only some targets states them; and one
// absent on the active target is added with its targets named first.
//
// Rules:
//   - rules/tooling/lsp.md — "Multi-target diagnostics" (all targets / one target / ...)
func mergeCrossTargetDiagnostics(active []diagnostic, result crossTargetResult) []diagnostic {
	applies := map[string][]string{}
	examples := map[string]diagnostic{}
	order := []string{}
	for _, target := range result.targets {
		for _, item := range result.perTarget[target] {
			key := crossTargetKey(item)
			if _, seen := examples[key]; !seen {
				examples[key] = item
				order = append(order, key)
			}
			if len(applies[key]) == 0 || applies[key][len(applies[key])-1] != target {
				applies[key] = append(applies[key], target)
			}
		}
	}
	total := len(result.targets)
	scope := func(targets []string) string {
		return fmt.Sprintf("applies to %d of %d targets: %s", len(targets), total, strings.Join(targets, ", "))
	}
	merged := make([]diagnostic, 0, len(active))
	present := map[string]bool{}
	for _, item := range active {
		key := crossTargetKey(item)
		present[key] = true
		if targets, known := applies[key]; known && len(targets) < total {
			item.Message += "\n\n" + scope(targets)
		}
		merged = append(merged, item)
	}
	for _, key := range order {
		if present[key] {
			continue
		}
		targets := applies[key]
		if len(targets) == total {
			// On every target but not reported by the active analysis, for
			// example with another scalar plan; the active result stands.
			continue
		}
		item := examples[key]
		item.Message = "[" + strings.Join(targets, ", ") + "] " + item.Message + "\n\n" + scope(targets)
		merged = append(merged, item)
	}
	sort.SliceStable(merged, func(i, j int) bool {
		return comparePosition(merged[i].Range.Start, merged[j].Range.Start) < 0
	})
	return merged
}
