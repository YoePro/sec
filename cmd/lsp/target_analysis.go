package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	lspserver "sec/internal/lsp/server"
	platformtarget "sec/internal/platform/target"
	"sec/internal/sema"
)

// targetAnalysisSnapshot retains detached compiler facts, never a shared
// mutable Analyzer. Both target diagnostics and conformance lenses consume it.
type targetAnalysisSnapshot struct {
	Errors       []sema.Error
	Warnings     []sema.Error
	Conformances []sema.InterfaceConformanceFact
	key          string
}

type targetAnalysisFlight struct {
	done chan struct{}
	data []byte
	err  error
}

// targetSnapshotCache bounds completed snapshots to 32 entries / 8 MiB. The
// serialized private values are immutable; decoding gives each reader its own
// nested diagnostics and conformance slices. In-flight requests share one Sema
// computation without introducing another worker pool.
// Rules: rules/tooling/lsp.md — "Multi-target diagnostics", "Document snapshots",
// "Interface conformance code lens"; rules/compiler/compiler_analysis.md — §55.
type targetSnapshotCache struct {
	mu           sync.Mutex
	entries      map[string][]byte
	order        []string
	bytes        int
	flights      map[string]*targetAnalysisFlight
	computations uint64
}

var sharedTargetAnalyses targetSnapshotCache

// load coalesces one immutable key and never caches failed computations.
// Waiters are released even on panic; a subsequent request may retry.
// Rules: rules/tooling/lsp.md — "Responsiveness model", "Document snapshots".
func (c *targetSnapshotCache) load(key string, compute func() ([]byte, error)) (data []byte, err error) {
	c.mu.Lock()
	if data, ok := c.entries[key]; ok {
		c.mu.Unlock()
		return data, nil
	}
	if flight := c.flights[key]; flight != nil {
		c.mu.Unlock()
		<-flight.done
		return flight.data, flight.err
	}
	if c.flights == nil {
		c.flights = map[string]*targetAnalysisFlight{}
	}
	flight := &targetAnalysisFlight{done: make(chan struct{})}
	c.flights[key] = flight
	c.computations++
	c.mu.Unlock()
	defer func() {
		if failed := recover(); failed != nil {
			data, err = nil, fmt.Errorf("target analysis failed: %v", failed)
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if err == nil && len(data) <= 8<<20 {
			for len(c.order) >= 32 || c.bytes+len(data) > 8<<20 {
				oldest := c.order[0]
				c.bytes -= len(c.entries[oldest])
				delete(c.entries, oldest)
				c.order = c.order[1:]
			}
			if c.entries == nil {
				c.entries = map[string][]byte{}
			}
			c.entries[key] = data
			c.order = append(c.order, key)
			c.bytes += len(data)
		}
		flight.data, flight.err = data, err
		delete(c.flights, key)
		close(flight.done)
	}()
	return compute()
}

// analysisForTarget assembles the same target-selected source/import closure
// for both consumers. The fingerprint includes the complete prepared AST with
// token locations, import failures, manifest snapshot, explicit selection,
// effective target/depth and server epoch. Source or configuration changes
// therefore cannot reuse earlier facts, even for direct calls outside the loop.
// Parsing remains outside the cache; only the expensive Sema run is shared.
// Rules: rules/tooling/lsp.md — "Target-aware analysis", "Multi-target diagnostics",
// "Interface conformance code lens"; rules/compiler/compiler_analysis.md — §55.
func analysisForTarget(uri, text string, overlay sourceOverlay, target platformtarget.Target) (targetAnalysisSnapshot, error) {
	path := pathFromURI(uri)
	program := parseProgramForLSP(uri, text)
	if program == nil {
		return targetAnalysisSnapshot{}, fmt.Errorf("cannot parse target source")
	}
	lspserver.AssembleModuleForTarget(program, path, overlay.Sources, target)
	resolveCoreSources(program, path, overlay)
	imports := resolveSourceImportsForTarget(program, map[string]bool{}, path, target, overlay)
	analyzer := newLSPAnalyzerWithOverlay(uri, program, overlay)
	var planKey any
	if definition, found := platformtarget.Find(target); found {
		if plan, err := definition.ScalarPlan(); err == nil {
			planKey = plan
			analyzer = sema.NewAnalyzerWithScalarPlanAndDepth(plan, sema.AnalysisInteractive)
		}
	}
	analysisCache.Lock()
	epoch := analysisCache.epoch
	analysisCache.Unlock()
	_, manifest, _ := lspProjectManifest(path, overlay)
	hash := sha256.New()
	err := json.NewEncoder(hash).Encode([]any{uri, text, target, planKey, analyzer.AnalysisDepth(),
		manifest, overlay.Targets, epoch, program, imports})
	if err != nil {
		return targetAnalysisSnapshot{}, err
	}
	key := hex.EncodeToString(hash.Sum(nil))
	data, err := sharedTargetAnalyses.load(key, func() ([]byte, error) {
		errors := append(imports, analyzer.Analyze(program)...)
		return json.Marshal(targetAnalysisSnapshot{Errors: errors, Warnings: analyzer.Warnings(), Conformances: analyzer.InterfaceConformances()})
	})
	if err != nil {
		return targetAnalysisSnapshot{}, err
	}
	var snapshot targetAnalysisSnapshot
	err = json.Unmarshal(data, &snapshot)
	snapshot.key = key
	return snapshot, err
}
