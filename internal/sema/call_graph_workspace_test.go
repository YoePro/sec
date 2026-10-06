package sema

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Workspace graph reuse must respect source/contract/selection dependencies,
// plan/model scope and latest-generation publication, independently of IDs.
// Rules: rules/analysis/call_graph.md — "Incremental compilation", "Incremental tests";
// rules/compiler/incremental_compilation.md — §§29–34, 83–86.
func TestCallGraphWorkspaceIndex(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/call_graph_workspace_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	const file = "workspace.sec"
	parsed := parser.New(lexer.NewWithFile(string(data), file))
	program := parsed.ParseProgram()
	if errs := parsed.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	analyzer := NewAnalyzer()
	if errs := analyzer.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	graph := analyzer.CallGraph()
	deps := []CallGraphDependency{{"source", file, "source-v1"}, {"source-set", "main-membership", "membership-v1"}, {"contract", "imported-operation", "contract-v1"}}
	scope := CallGraphScope{"main", "linux-amd64", "sema-model-v1"}
	other := CallGraphScope{"main", "linux-arm64", "sema-model-v1"}
	expected, err := graph.ForCompilationPlan(scope)
	if err != nil {
		t.Fatal(err)
	}
	var index WorkspaceCallGraphIndex
	indexedErrors, committed, cacheError := NewAnalyzer().AnalyzeIndexed(program, &index, scope, deps)
	if len(indexedErrors) != 0 || !committed || cacheError != nil {
		t.Fatal("indexed compiler analysis failed", indexedErrors, committed, cacheError)
	}
	publish := func(scope CallGraphScope, deps []CallGraphDependency) {
		t.Helper()
		lease, err := index.Begin(scope, deps)
		if err != nil {
			t.Fatal(err)
		}
		ok, err := index.Publish(lease, graph)
		if err != nil || !ok {
			t.Fatal("publication", ok, err)
		}
	}
	publish(scope, deps)
	publish(other, deps)
	snapshot, ok := index.Snapshot(scope, deps)
	if !ok || !reflect.DeepEqual(expected, snapshot) {
		t.Fatal("snapshot mismatch", ok)
	}
	snapshot.sites[0].Targets[0] = "mutated"
	unchanged, ok := index.Snapshot(scope, deps)
	if !ok || !reflect.DeepEqual(expected, unchanged) {
		t.Fatal("index facts not detached")
	}
	reversed := append([]CallGraphDependency(nil), deps...)
	reversed[0], reversed[2] = reversed[2], reversed[0]
	if _, ok := index.Snapshot(scope, reversed); !ok {
		t.Fatal("dependency order affects validity")
	}
	for _, wrong := range []CallGraphScope{{"main", "missing-plan", "sema-model-v1"}, {"main", "linux-amd64", "sema-model-v2"}} {
		if _, ok := index.Snapshot(wrong, deps); ok {
			t.Fatal("cross-context reuse")
		}
	}
	mutatedDeps := append([]CallGraphDependency(nil), deps...)
	mutatedDeps[0].Fingerprint = "source-v2"
	if _, ok := index.Snapshot(scope, mutatedDeps); ok {
		t.Fatal("same IDs authorized stale source reuse")
	}
	before, _ := index.Checkpoint()
	again, _ := index.Checkpoint()
	if !bytes.Equal(before, again) {
		t.Fatal("checkpoint not deterministic")
	}
	current := map[CallGraphScope][]CallGraphDependency{scope: deps, other: deps}
	restored, err := RestoreWorkspaceCallGraphIndex(before, current)
	if err != nil {
		t.Fatal(err)
	}
	roundtrip, ok := restored.Snapshot(scope, deps)
	if !ok || !reflect.DeepEqual(expected, roundtrip) {
		t.Fatal("checkpoint changed canonical facts")
	}
	mainID := callGraphNodeIDByName(t, expected, "main")
	if !reflect.DeepEqual(expected.DomainSummary(mainID), roundtrip.DomainSummary(mainID)) || !reflect.DeepEqual(expected.Roots(), roundtrip.Roots()) {
		t.Fatal("derived effects/reachability changed after restore")
	}
	for _, node := range expected.Nodes() {
		if !reflect.DeepEqual(expected.SameStackSCC(node.ID), roundtrip.SameStackSCC(node.ID)) {
			t.Fatal("recursive component changed")
		}
	}
	stale, err := RestoreWorkspaceCallGraphIndex(before, map[CallGraphScope][]CallGraphDependency{scope: mutatedDeps, other: deps})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := stale.Snapshot(scope, mutatedDeps); ok {
		t.Fatal("stale persisted dependency reused")
	}
	if _, ok := stale.Snapshot(other, deps); !ok {
		t.Fatal("unaffected plan not restored")
	}
	// New work invalidates the old scope immediately; later completion cannot
	// overwrite the current generation or reuse a consumed publication lease.
	old, err := index.Begin(scope, deps)
	if err != nil {
		t.Fatal(err)
	}
	latest, err := index.Begin(scope, deps)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := index.Publish(old, graph); err != nil || ok {
		t.Fatal("obsolete completion accepted")
	}
	if ok, err := index.Publish(latest, graph); err != nil || !ok {
		t.Fatal("latest completion rejected", err)
	}
	if ok, err := index.Publish(latest, graph); err != nil || ok {
		t.Fatal("lease published twice")
	}
	// Invalidation also reaches pending work and all transitive/SCC facts of
	// dependent graph units, while an unrelated module unit remains reusable.
	unrelated := CallGraphScope{"isolated", "linux-amd64", "sema-model-v1"}
	unrelatedDeps := []CallGraphDependency{{"source", file, "source-v1"}, {"source-set", "isolated-membership", "membership-v1"}}
	publish(unrelated, unrelatedDeps)
	pending, err := index.Begin(scope, deps)
	if err != nil {
		t.Fatal(err)
	}
	affected := index.Invalidate("contract", "imported-operation")
	if len(affected) != 2 {
		t.Fatal("external contract dependents not invalidated", affected)
	}
	if ok, err := index.Publish(pending, graph); err != nil || ok {
		t.Fatal("invalidated pending analysis published")
	}
	if _, ok := index.Snapshot(other, deps); ok {
		t.Fatal("transitive dependent graph remained current")
	}
	if _, ok := index.Snapshot(unrelated, unrelatedDeps); !ok {
		t.Fatal("unrelated graph evicted")
	}
	publish(scope, deps)
	publish(other, deps)
	if affected := index.Invalidate("source-set", "main-membership"); len(affected) != 2 {
		t.Fatal("negative/set membership dependency not invalidated", affected)
	}
	publish(scope, deps)
	publish(other, deps)
	changed, err := index.Begin(scope, mutatedDeps)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := index.Snapshot(other, deps); ok {
		t.Fatal("newly observed source change left old plan current")
	}
	if ok, err := index.Publish(changed, graph); err != nil || !ok {
		t.Fatal("new source generation rejected", err)
	}
	detachedDeps := append([]CallGraphDependency(nil), mutatedDeps...)
	lease, err := index.Begin(scope, detachedDeps)
	if err != nil {
		t.Fatal(err)
	}
	detachedDeps[0].Fingerprint = "mutated"
	if ok, err := index.Publish(lease, graph); err != nil || !ok {
		t.Fatal("dependency slice aliases lease")
	}
	if _, ok := index.Snapshot(scope, mutatedDeps); !ok {
		t.Fatal("detached lease used mutated dependencies")
	}
	// Wrong-owner and malformed source/target facts never enter the index.
	var alien WorkspaceCallGraphIndex
	if _, err := alien.Publish(lease, graph); err == nil {
		t.Fatal("foreign lease accepted")
	}
	invalid := graph.clone()
	invalid.sites[0].Targets = []CallableID{"missing"}
	fresh, err := index.Begin(scope, mutatedDeps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := index.Publish(fresh, invalid); err == nil {
		t.Fatal("missing target accepted")
	}
	if _, ok := index.Snapshot(scope, mutatedDeps); ok {
		t.Fatal("invalid intermediate snapshot retained old proof")
	}
	if _, err := index.Begin(CallGraphScope{}, deps); err == nil {
		t.Fatal("missing plan/model scope accepted")
	}
	duplicate := append(append([]CallGraphDependency(nil), deps...), deps[0])
	if _, err := index.Begin(scope, duplicate); err == nil {
		t.Fatal("duplicate fingerprint accepted")
	}
	for _, broken := range [][]byte{before[:len(before)/2], append(append([]byte(nil), before...), []byte(" {}")...)} {
		if _, err := RestoreWorkspaceCallGraphIndex(broken, current); err == nil {
			t.Fatal("partial or trailing checkpoint accepted")
		}
	}
	var envelope graphCheckpoint
	if err := json.Unmarshal(before, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.Version++
	bad, _ := json.Marshal(envelope)
	if _, err := RestoreWorkspaceCallGraphIndex(bad, current); err == nil {
		t.Fatal("wrong schema accepted")
	}
	envelope.Version--
	envelope.SHA256 = "corrupt"
	bad, _ = json.Marshal(envelope)
	if _, err := RestoreWorkspaceCallGraphIndex(bad, current); err == nil {
		t.Fatal("integrity mismatch accepted")
	}
	// Structural checks apply even to a decodable, checksummed payload.
	envelope = graphCheckpoint{}
	_ = json.Unmarshal(before, &envelope)
	var records []graphCheckpointEntry
	_ = json.Unmarshal(envelope.Payload, &records)
	records[0].Graph.Sites[0].Targets = []CallableID{"missing"}
	envelope.Payload, _ = json.Marshal(records)
	digest := sha256.Sum256(envelope.Payload)
	envelope.SHA256 = hex.EncodeToString(digest[:])
	bad, _ = json.Marshal(envelope)
	if _, err := RestoreWorkspaceCallGraphIndex(bad, current); err == nil {
		t.Fatal("malformed graph payload accepted")
	}
	// Exercise concurrent readers and invalidation/publication under the race
	// detector; deterministic lease checks above establish the actual outcomes.
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 20; j++ {
				_, _ = index.Snapshot(scope, mutatedDeps)
				_, _ = index.Checkpoint()
			}
		}()
	}
	for j := 0; j < 20; j++ {
		publish(scope, mutatedDeps)
		index.Invalidate("source-set", "main-membership")
	}
	workers.Wait()
	// An invalid intermediate source must discard its lease and cannot poison
	// later corrected analysis or erase the owning language error.
	invalidData, err := os.ReadFile("../../testdata/sema/call_graph_workspace_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	invalidProgram := parser.New(lexer.NewWithFile(string(invalidData), file)).ParseProgram()
	indexedErrors, committed, cacheError = NewAnalyzer().AnalyzeIndexed(invalidProgram, &index, scope, mutatedDeps)
	if len(indexedErrors) == 0 || committed || cacheError != nil {
		t.Fatal("invalid source entered graph cache", indexedErrors, committed, cacheError)
	}
	if _, ok := index.Snapshot(scope, mutatedDeps); ok {
		t.Fatal("old graph claimed current after invalid analysis")
	}
	indexedErrors, committed, cacheError = NewAnalyzer().AnalyzeIndexed(program, &index, scope, deps)
	if len(indexedErrors) != 0 || !committed || cacheError != nil {
		t.Fatal("corrected source failed indexed analysis", indexedErrors, committed, cacheError)
	}
	// A real callee-body edit preserves declaration identity but changes the
	// transitive panic fact of its callers. The warm index must match cold Sema.
	revisedSource := strings.Replace(string(data), "return value + 1", "return value", 1)
	revisedProgram := parser.New(lexer.NewWithFile(revisedSource, file)).ParseProgram()
	revisedDeps := append([]CallGraphDependency(nil), deps...)
	revisedDeps[0].Fingerprint = "source-v3"
	cold := NewAnalyzer()
	if errors := cold.Analyze(revisedProgram); len(errors) != 0 {
		t.Fatal(errors)
	}
	coldGraph, err := cold.CallGraph().ForCompilationPlan(scope)
	if err != nil {
		t.Fatal(err)
	}
	if coldGraph.EffectSummary(mainID).MayPanic {
		t.Fatal("revised callee unexpectedly retains panic")
	}
	indexedErrors, committed, cacheError = NewAnalyzer().AnalyzeIndexed(revisedProgram, &index, scope, revisedDeps)
	if len(indexedErrors) != 0 || !committed || cacheError != nil {
		t.Fatal("edited callee failed indexed analysis", indexedErrors, committed, cacheError)
	}
	warm, ok := index.Snapshot(scope, revisedDeps)
	if !ok || !reflect.DeepEqual(coldGraph, warm) || warm.EffectSummary(mainID).MayPanic {
		t.Fatal("warm changed-callee graph differs from cold Sema")
	}
}
