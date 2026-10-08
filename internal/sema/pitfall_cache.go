package sema

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"sync"

	"sec/internal/ast"
)

// PitfallCacheScope identifies compatible module, plan and semantic-rule inputs.
// Rules: rules/analysis/pitfall_analysis.md — "Incremental analysis";
// rules/compiler/incremental_compilation.md — §§4-9,16.
type PitfallCacheScope struct{ Module, CompilationPlan, CompilerModel string }

// PitfallBodyID identifies a semantic body's source/provenance occurrence.
// Source movement invalidates diagnostic locations even if its text is equal.
// Rules: rules/compiler/incremental_compilation.md — §§5,8(3),15.
type PitfallBodyID struct {
	File         string
	Line, Column int
}

// PitfallBodyIdentity derives the exact body anchor from canonical syntax.
// Rules: rules/analysis/pitfall_analysis.md — "Incremental analysis".
func PitfallBodyIdentity(body *ast.BlockStatement) PitfallBodyID {
	if body == nil {
		return PitfallBodyID{}
	}
	return PitfallBodyID{body.Token.File, body.Token.Line, body.Token.Column}
}

// PitfallDependency revisions belong to canonical prerequisite producers,
// including transitive summaries, set-valued and negative dependencies.
// An unchanged callable name or source timestamp is not a fact fingerprint.
// Rules: rules/compiler/incremental_compilation.md — §§11-16;
// rules/analysis/pitfall_analysis.md — "Registry and dependencies".
type PitfallDependency struct{ Kind, Identity, Fingerprint string }

// PitfallBodyRevision attests a complete canonical fact dependency closure.
// Hosts must include every relevant producer dependency and active required
// fact kind. Incomplete/unknown input disables reuse for this body; omitted
// bodies are analyzed cold. This is a compiler API, not Sec source syntax.
// Rules: rules/compiler/incremental_compilation.md — §§7,11-14;
// rules/analysis/pitfall_analysis.md — "Incremental analysis".
type PitfallBodyRevision struct {
	Body         PitfallBodyID
	Complete     bool
	Dependencies []PitfallDependency
}

// PitfallCacheUsage separates avoided rule walks from ordinary semantic work.
// Mandatory producers and whole-unit budget admission always execute anew.
// Rules: rules/analysis/pitfall_analysis.md — "Incremental analysis", "Analysis states".
type PitfallCacheUsage struct{ Hits, Misses, Stored, Unavailable, Superseded int }

type pitfallCacheKey struct {
	Scope PitfallCacheScope
	Body  PitfallBodyID
}
type pitfallFactKey struct {
	Scope          PitfallCacheScope
	Kind, Identity string
}
type pitfallBodyPayload struct {
	Findings []PitfallFinding
	Foreign  []ResolvedForeignBufferExtent
}
type pitfallCacheEntry struct {
	Fingerprint  [32]byte
	Dependencies []PitfallDependency
	Payload      pitfallBodyPayload
}
type pitfallCacheLease struct {
	Owner      *PitfallCache
	Key        pitfallCacheKey
	Generation uint64
	Entry      pitfallCacheEntry
}

// PitfallCache retains immutable, complete body results in memory. Its zero
// value is usable; unrelated bodies/scopes survive dependency invalidation.
// Rules: rules/analysis/pitfall_analysis.md — "Incremental analysis";
// rules/compiler/incremental_compilation.md — §§7-10,29-34.
type PitfallCache struct {
	mu         sync.Mutex
	entries    map[pitfallCacheKey]pitfallCacheEntry
	pending    map[pitfallCacheKey]pitfallCacheLease
	observed   map[pitfallFactKey]string
	generation uint64
}

type pitfallCacheRun struct {
	Cache  *PitfallCache
	Scope  PitfallCacheScope
	Bodies map[PitfallBodyID]PitfallBodyRevision
	Leases []pitfallCacheLease
	Usage  PitfallCacheUsage
}

// AnalyzeWithPitfallCache runs fresh canonical Sema, then reuses only admitted
// optional body searches with complete compatibility evidence. Successful
// latest-generation body computations publish transactionally after Analyze;
// failed analysis never publishes. Cache configuration errors are returned
// separately and fall back to cold Sema without changing Sec validity.
// Hosts own complete prerequisite revision production and cache lifetime.
// Rules: rules/compiler/incremental_compilation.md — §§2,7,29-34;
// rules/analysis/pitfall_analysis.md — "Incremental analysis", "Diagnostic ownership and coalescing".
func (a *Analyzer) AnalyzeWithPitfallCache(program *ast.Program, cache *PitfallCache, scope PitfallCacheScope, revisions []PitfallBodyRevision) ([]Error, PitfallCacheUsage, error) {
	run, cacheError := preparePitfallCacheRun(cache, scope, revisions)
	previous := a.pitfallCacheRun
	a.pitfallCacheRun = run
	defer func() { a.pitfallCacheRun = previous }()
	semanticErrors := a.Analyze(program)
	if run == nil {
		return semanticErrors, PitfallCacheUsage{}, cacheError
	}
	for _, lease := range run.Leases {
		if len(semanticErrors) == 0 && run.Cache.publish(lease) {
			run.Usage.Stored++
		} else {
			run.Cache.discard(lease)
			run.Usage.Superseded++
		}
	}
	return semanticErrors, run.Usage, cacheError
}

// PitfallRequiredFactKinds exposes the active registry's producer requirements
// in deterministic order, so hosts can verify completeness without a second
// tooling-owned pitfall registry. Additional actual dependencies remain required.
// Rules: rules/analysis/pitfall_analysis.md — "Registry and dependencies";
// rules/compiler/incremental_compilation.md — §§11-14.
func PitfallRequiredFactKinds(depth AnalysisDepth) []string {
	kinds := map[string]bool{}
	for _, rule := range pitfallRuleRegistry {
		if analysisDepthAtLeast(depth, rule.MinimumDepth) {
			for _, fact := range rule.RequiredFacts {
				kinds[fact] = true
			}
		}
	}
	result := make([]string, 0, len(kinds))
	for kind := range kinds {
		result = append(result, kind)
	}
	sort.Strings(result)
	return result
}

// preparePitfallCacheRun detaches host inputs and rejects contradictory
// revisions or duplicate producer identities; absent knowledge stays cold.
// Rules: rules/compiler/incremental_compilation.md — §§7,12,15,17.
func preparePitfallCacheRun(cache *PitfallCache, scope PitfallCacheScope, revisions []PitfallBodyRevision) (*pitfallCacheRun, error) {
	if cache == nil {
		return nil, nil
	}
	if scope.Module == "" || scope.CompilationPlan == "" || scope.CompilerModel == "" {
		return nil, errors.New("pitfall cache requires module, CompilationPlan and compiler-model identities")
	}
	run := &pitfallCacheRun{Cache: cache, Scope: scope, Bodies: map[PitfallBodyID]PitfallBodyRevision{}}
	observed := map[[2]string]string{}
	for _, revision := range revisions {
		if revision.Body.File == "" || revision.Body.Line <= 0 || revision.Body.Column <= 0 {
			return nil, errors.New("pitfall cache requires an exact source body identity")
		}
		if _, exists := run.Bodies[revision.Body]; exists {
			return nil, errors.New("duplicate pitfall body revision")
		}
		revision.Dependencies = append([]PitfallDependency(nil), revision.Dependencies...)
		seen := map[[2]string]bool{}
		for _, dep := range revision.Dependencies {
			key := [2]string{dep.Kind, dep.Identity}
			if dep.Kind == "" || dep.Identity == "" || dep.Fingerprint == "" || seen[key] {
				return nil, errors.New("invalid or duplicate pitfall fact dependency")
			}
			if old, exists := observed[key]; exists && old != dep.Fingerprint {
				return nil, errors.New("contradictory pitfall fact revisions in one snapshot")
			}
			seen[key] = true
			observed[key] = dep.Fingerprint
		}
		sort.Slice(revision.Dependencies, func(i, j int) bool {
			left, right := revision.Dependencies[i], revision.Dependencies[j]
			if left.Kind != right.Kind {
				return left.Kind < right.Kind
			}
			return left.Identity < right.Identity
		})
		run.Bodies[revision.Body] = revision
	}
	return run, nil
}

// clonePitfallPayload detaches cached findings, suppression/action slices and
// nested foreign-contract/Place storage from both publication and consumption.
// Rules: rules/compiler/incremental_compilation.md — §§3,29-34;
// rules/analysis/pitfall_analysis.md — "Determinism".
func clonePitfallPayload(payload pitfallBodyPayload) pitfallBodyPayload {
	result := pitfallBodyPayload{Findings: make([]PitfallFinding, len(payload.Findings)), Foreign: cloneForeignBufferExtents(payload.Foreign)}
	for i, finding := range payload.Findings {
		result.Findings[i] = clonePitfallFinding(finding)
	}
	return result
}

// bodyFingerprint binds exact syntax/provenance, plan scalars, depth, budget,
// fact revisions, registry metadata and the implementation's cache contract.
// Rules: rules/compiler/incremental_compilation.md — §§8-9,14-17;
// rules/analysis/pitfall_analysis.md — "Incremental analysis".
func (b *pitfallBuilder) bodyFingerprint(body *ast.BlockStatement, revision PitfallBodyRevision) ([32]byte, bool) {
	kinds := map[string]bool{}
	for _, dep := range revision.Dependencies {
		kinds[dep.Kind] = true
	}
	if !revision.Complete || !pitfallCacheSyntaxComplete(reflect.ValueOf(body)) {
		return [32]byte{}, false
	}
	for _, kind := range PitfallRequiredFactKinds(b.analyzer.analysisDepth) {
		if !kinds[kind] {
			return [32]byte{}, false
		}
	}
	input := struct {
		Contract     string
		Body         *ast.BlockStatement
		Scope        PitfallCacheScope
		Depth        AnalysisDepth
		Budget       AnalysisBudget
		NativeBits   uint16
		Float        Type
		Rules        []PitfallRuleDefinition
		Dependencies []PitfallDependency
	}{"pitfall-body-cache-v1", body, b.analyzer.pitfallCacheRun.Scope, b.analyzer.analysisDepth, b.analyzer.analysisBudget, b.analyzer.targetUintWidthBits, b.analyzer.types["float"], pitfallRuleRegistry, revision.Dependencies}
	encoded, err := json.Marshal(input)
	if err != nil {
		return [32]byte{}, false
	}
	return sha256.Sum256(encoded), true
}

// pitfallCacheSyntaxComplete refuses recovered syntax even when parser-owned
// errors are intentionally not repeated by Sema. Recovery is tooling input,
// never a complete cacheable semantic body.
// Rules: rules/compiler/parser_recovery.md — "Recovery nodes";
// rules/compiler/incremental_compilation.md — §§2,7;
// rules/analysis/pitfall_analysis.md — "Analysis states".
func pitfallCacheSyntaxComplete(value reflect.Value) bool {
	if !value.IsValid() {
		return true
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return true
		}
		if value.CanInterface() {
			switch value.Interface().(type) {
			case *ast.RecoveryInfo, *ast.InvalidExpression, *ast.InvalidStatement, *ast.InvalidPattern:
				return false
			}
		}
		return pitfallCacheSyntaxComplete(value.Elem())
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if !pitfallCacheSyntaxComplete(value.Field(i)) {
				return false
			}
		}
	case reflect.Map:
		for _, key := range value.MapKeys() {
			if !pitfallCacheSyntaxComplete(value.MapIndex(key)) {
				return false
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if !pitfallCacheSyntaxComplete(value.Index(i)) {
				return false
			}
		}
	}
	return true
}

// lookupOrBegin validates observed prerequisite revisions and creates a unique
// publication lease on misses. A changed fact evicts every dependent body,
// including pending work, while leaving unrelated scope/body units untouched.
// Rules: rules/compiler/incremental_compilation.md — §§7-8,29-34;
// rules/analysis/pitfall_analysis.md — "Incremental analysis".
func (cache *PitfallCache) lookupOrBegin(key pitfallCacheKey, fingerprint [32]byte, dependencies []PitfallDependency) (pitfallBodyPayload, bool, pitfallCacheLease) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.entries == nil {
		cache.entries = map[pitfallCacheKey]pitfallCacheEntry{}
		cache.pending = map[pitfallCacheKey]pitfallCacheLease{}
		cache.observed = map[pitfallFactKey]string{}
	}
	for _, dep := range dependencies {
		fact := pitfallFactKey{key.Scope, dep.Kind, dep.Identity}
		if old, exists := cache.observed[fact]; exists && old != dep.Fingerprint {
			cache.invalidateLocked(&key.Scope, dep.Kind, dep.Identity)
		}
		cache.observed[fact] = dep.Fingerprint
	}
	if entry, exists := cache.entries[key]; exists && entry.Fingerprint == fingerprint {
		return clonePitfallPayload(entry.Payload), true, pitfallCacheLease{}
	}
	delete(cache.entries, key)
	cache.generation++
	lease := pitfallCacheLease{Owner: cache, Key: key, Generation: cache.generation, Entry: pitfallCacheEntry{Fingerprint: fingerprint, Dependencies: append([]PitfallDependency(nil), dependencies...)}}
	cache.pending[key] = lease
	return pitfallBodyPayload{}, false, lease
}

// publish commits one complete body result only if its lease and dependency
// revisions are still current. The lease cannot publish a second time.
// Rules: rules/compiler/incremental_compilation.md — §§29-34.
func (cache *PitfallCache) publish(lease pitfallCacheLease) bool {
	if lease.Owner != cache {
		return false
	}
	for _, finding := range lease.Entry.Payload.Findings {
		if finding.State != PitfallStateFinding && finding.State != PitfallStateSuppressed {
			return false
		}
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	pending, exists := cache.pending[lease.Key]
	if !exists || pending.Generation != lease.Generation {
		return false
	}
	for _, dep := range lease.Entry.Dependencies {
		if cache.observed[pitfallFactKey{lease.Key.Scope, dep.Kind, dep.Identity}] != dep.Fingerprint {
			return false
		}
	}
	lease.Entry.Payload = clonePitfallPayload(lease.Entry.Payload)
	lease.Entry.Dependencies = append([]PitfallDependency(nil), lease.Entry.Dependencies...)
	cache.entries[lease.Key] = lease.Entry
	delete(cache.pending, lease.Key)
	return true
}

// discard cancels only the current lease, preserving any newer computation.
// Rules: rules/compiler/incremental_compilation.md — §§29-34.
func (cache *PitfallCache) discard(lease pitfallCacheLease) {
	if lease.Owner != cache {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if pending, exists := cache.pending[lease.Key]; exists && pending.Generation == lease.Generation {
		delete(cache.pending, lease.Key)
	}
}

// Invalidate evicts every completed or pending body using a changed canonical
// fact identity, across plans/models. It returns a deterministic affected-unit
// count; unrelated bodies remain reusable. Revisions are reobserved on demand.
// Rules: rules/analysis/pitfall_analysis.md — "Incremental analysis";
// rules/compiler/incremental_compilation.md — §§8,19,29.
func (cache *PitfallCache) Invalidate(kind, identity string) int {
	if cache == nil {
		return 0
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.invalidateLocked(nil, kind, identity)
}

// invalidateLocked removes precisely the cached/pending dependency consumers.
// Rules: rules/compiler/incremental_compilation.md — §§8,19,29.
func (cache *PitfallCache) invalidateLocked(scope *PitfallCacheScope, kind, identity string) int {
	affected := map[pitfallCacheKey]bool{}
	depends := func(key pitfallCacheKey, deps []PitfallDependency) bool {
		if scope != nil && key.Scope != *scope {
			return false
		}
		for _, dep := range deps {
			if dep.Kind == kind && dep.Identity == identity {
				return true
			}
		}
		return false
	}
	for key, entry := range cache.entries {
		if depends(key, entry.Dependencies) {
			delete(cache.entries, key)
			affected[key] = true
		}
	}
	for key, lease := range cache.pending {
		if depends(key, lease.Entry.Dependencies) {
			delete(cache.pending, key)
			affected[key] = true
		}
	}
	return len(affected)
}

// walkPitfallBody consumes a complete cached optional body search only after
// fresh canonical Sema and whole-body budget admission. Mandatory owners and
// incomplete prerequisite closures always take the ordinary walk. Cached
// findings pass the current safety/publication gate and current counters.
// Rules: rules/analysis/pitfall_analysis.md — "Incremental analysis", "Analysis states",
// "Diagnostic ownership and coalescing", "Fix safety";
// rules/compiler/incremental_compilation.md — §§2,7,29-34.
func (b *pitfallBuilder) walkPitfallBody(body *ast.BlockStatement) {
	run := b.analyzer.pitfallCacheRun
	if run == nil {
		b.walkBlock(body)
		return
	}
	revision, known := run.Bodies[PitfallBodyIdentity(body)]
	if !known || len(b.analyzer.errors) != 0 {
		run.Usage.Unavailable++
		b.walkBlock(body)
		return
	}
	fingerprint, compatible := b.bodyFingerprint(body, revision)
	if !compatible {
		run.Usage.Unavailable++
		b.walkBlock(body)
		return
	}
	key := pitfallCacheKey{run.Scope, revision.Body}
	payload, hit, lease := run.Cache.lookupOrBegin(key, fingerprint, revision.Dependencies)
	if hit {
		run.Usage.Hits++
		for _, finding := range payload.Findings {
			b.add(finding)
		}
		b.result.foreignExtentInputs = append(b.result.foreignExtentInputs, payload.Foreign...)
		return
	}
	run.Usage.Misses++
	findingsStart, foreignStart := len(b.result.results), len(b.result.foreignExtentInputs)
	b.walkBlock(body)
	payload = pitfallBodyPayload{Findings: b.result.results[findingsStart:], Foreign: b.result.foreignExtentInputs[foreignStart:]}
	// Owner-enrichment occurrences may carry a distinct canonical diagnostic
	// root. Recompute those explanations instead of persisting an owner link.
	for _, finding := range payload.Findings {
		if finding.DiagnosticID != "" || finding.Classification == PitfallProvenInvalid {
			run.Cache.discard(lease)
			run.Usage.Unavailable++
			return
		}
	}
	lease.Entry.Payload = clonePitfallPayload(payload)
	run.Leases = append(run.Leases, lease)
}
