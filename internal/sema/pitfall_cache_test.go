package sema

import (
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// pitfallCacheFixture parses fresh source and provides explicit producer
// revisions for the controlled test snapshot; these are not source annotations.
// Rules: rules/analysis/pitfall_analysis.md — "Required incremental and LSP tests";
// rules/compiler/incremental_compilation.md — §§7,11-16.
func pitfallCacheFixture(t *testing.T, source string, depth AnalysisDepth) (*ast.Program, []PitfallBodyRevision) {
	t.Helper()
	file := "../../testdata/sema/pitfall_cache_valid.sec"
	p := parser.New(lexer.NewWithFile(source, file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	revisions := []PitfallBodyRevision{}
	for _, stmt := range program.Statements {
		fn, ok := stmt.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		rev := PitfallBodyRevision{Body: PitfallBodyIdentity(fn.Body), Complete: true}
		for _, kind := range PitfallRequiredFactKinds(depth) {
			rev.Dependencies = append(rev.Dependencies, PitfallDependency{kind, fn.Name.Value + "/" + kind, "canonical-v1"})
		}
		revisions = append(revisions, rev)
	}
	return program, revisions
}

// pitfallCacheSource reads the source fixture shared by cold and warm checks.
// Rules: rules/analysis/pitfall_analysis.md — "Required incremental and LSP tests".
func pitfallCacheSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/sema/pitfall_cache_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// checkPitfallCacheRun requires warm/cold equality of validity, exact finding
// evidence/actions/suppression, evaluations, coverage and consumed FFI facts.
// Rules: rules/compiler/incremental_compilation.md — §§1-2,72;
// rules/analysis/pitfall_analysis.md — "Determinism", "Incremental analysis".
func checkPitfallCacheRun(t *testing.T, a *Analyzer, cache *PitfallCache, scope PitfallCacheScope, source string) PitfallCacheUsage {
	t.Helper()
	program, revisions := pitfallCacheFixture(t, source, a.analysisDepth)
	actual, usage, err := a.AnalyzeWithPitfallCache(program, cache, scope, revisions)
	if err != nil || len(actual) != 0 {
		t.Fatal(actual, usage, err)
	}
	coldProgram, _ := pitfallCacheFixture(t, source, a.analysisDepth)
	cold := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: a.targetUintWidthBits}, a.analysisDepth)
	cold.analysisBudget = a.analysisBudget
	expected := cold.Analyze(coldProgram)
	if !reflect.DeepEqual(actual, expected) || !reflect.DeepEqual(a.PitfallAnalysis(), cold.PitfallAnalysis()) {
		t.Fatal("warm/cold mismatch", actual, expected, a.PitfallAnalysis(), cold.PitfallAnalysis())
	}
	return usage
}

// TestPitfallCacheBodyReuse invalidates an edited bound or guard without
// rerunning unrelated bodies, and replays both findings and NoFinding bodies.
// Rules: rules/analysis/pitfall_analysis.md — "Required incremental and LSP tests";
// rules/compiler/incremental_compilation.md — §§7-8,18.
func TestPitfallCacheBodyReuse(t *testing.T) {
	source := pitfallCacheSource(t)
	scope := PitfallCacheScope{"pitfall_cache", "plan64", "sema-model-v1"}
	var cache PitfallCache
	a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: 64}, AnalysisStandard)
	if usage := checkPitfallCacheRun(t, a, &cache, scope, source); usage.Misses != 4 || usage.Stored != 4 || usage.Hits != 0 {
		t.Fatal(usage)
	}
	if usage := checkPitfallCacheRun(t, a, &cache, scope, source); usage.Hits != 4 || usage.Misses != 0 || usage.Stored != 0 {
		t.Fatal(usage)
	}
	// Equal-width edits retain unrelated exact source/provenance coordinates.
	edited := strings.Replace(source, "flag == false", "flag == true ", 1)
	if usage := checkPitfallCacheRun(t, a, &cache, scope, edited); usage.Hits != 3 || usage.Misses != 1 || usage.Stored != 1 {
		t.Fatal(usage)
	}
	edited = strings.Replace(edited, "0u..<items.Len", "1u..<items.Len", 1)
	if usage := checkPitfallCacheRun(t, a, &cache, scope, edited); usage.Hits != 3 || usage.Misses != 1 {
		t.Fatal(usage)
	}
	skipped := false
	for _, f := range a.PitfallAnalysis().Findings() {
		skipped = skipped || f.Rule == PitfallSkippedFirstElement
	}
	if !skipped {
		t.Fatal("edited loop finding absent")
	}
	edited = strings.Replace(edited, "items.Len > 0u", "flag          ", 1)
	if usage := checkPitfallCacheRun(t, a, &cache, scope, edited); usage.Hits != 3 || usage.Misses != 1 {
		t.Fatal(usage)
	}
	final := false
	for _, f := range a.PitfallAnalysis().Findings() {
		final = final || f.Rule == PitfallFinalElementNeedsNonEmpty
	}
	if !final {
		t.Fatal("removed guard finding absent")
	}
	edited = strings.Replace(edited, "flag          ", "items.Len > 0u", 1)
	if usage := checkPitfallCacheRun(t, a, &cache, scope, edited); usage.Hits != 3 || usage.Misses != 1 {
		t.Fatal(usage)
	}
	for _, f := range a.PitfallAnalysis().Findings() {
		if f.Rule == PitfallFinalElementNeedsNonEmpty {
			t.Fatal("valid guard kept stale finding", f)
		}
	}
	// Ordinary Analyze never inherits an indexed request's context.
	p, _ := pitfallCacheFixture(t, source, AnalysisStandard)
	a.Analyze(p)
	if a.pitfallCacheRun != nil {
		t.Fatal("cache context leaked")
	}
}

// TestPitfallCacheDependenciesAndScopes checks producer revision invalidation,
// explicit eviction, plan/model/module separation, depth and budget gates.
// Rules: rules/compiler/incremental_compilation.md — §§7-9,14,29-34;
// rules/analysis/pitfall_analysis.md — "Incremental analysis", "Analysis states".
func TestPitfallCacheDependenciesAndScopes(t *testing.T) {
	source := pitfallCacheSource(t)
	scope := PitfallCacheScope{"pitfall_cache", "plan64", "v1"}
	var cache PitfallCache
	a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: 64}, AnalysisStandard)
	checkPitfallCacheRun(t, a, &cache, scope, source)
	p, revs := pitfallCacheFixture(t, source, AnalysisStandard)
	// Two consumers of the same canonical exported summary; changing its
	// revision must evict both, while the other two bodies remain reusable.
	for _, i := range []int{0, 1} {
		revs[i].Dependencies = append(revs[i].Dependencies, PitfallDependency{"callee-summary", "exported/Observe", "v1"})
	}
	_, u, e := a.AnalyzeWithPitfallCache(p, &cache, scope, revs)
	if e != nil || u.Hits != 2 || u.Misses != 2 {
		t.Fatal(u, e)
	}
	for _, i := range []int{0, 1} {
		revs[i].Dependencies[len(revs[i].Dependencies)-1].Fingerprint = "v2"
	}
	_, u, e = a.AnalyzeWithPitfallCache(p, &cache, scope, revs)
	if e != nil || u.Hits != 2 || u.Misses != 2 {
		t.Fatal(u, e)
	}
	if affected := cache.Invalidate("callee-summary", "exported/Observe"); affected != 2 {
		t.Fatal(affected)
	}
	_, u, e = a.AnalyzeWithPitfallCache(p, &cache, scope, revs)
	if e != nil || u.Hits != 2 || u.Stored != 2 {
		t.Fatal(u, e)
	}
	if affected := cache.Invalidate("callee-summary", "unrelated"); affected != 0 {
		t.Fatal(affected)
	}
	for _, other := range []PitfallCacheScope{{"other", "plan64", "v1"}, {"pitfall_cache", "plan32", "v1"}, {"pitfall_cache", "plan64", "v2"}} {
		if usage := checkPitfallCacheRun(t, a, &cache, other, source); usage.Misses != 4 || usage.Stored != 4 {
			t.Fatal(other, usage)
		}
	}
	deeper := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: 64}, AnalysisDeep)
	if usage := checkPitfallCacheRun(t, deeper, &cache, scope, source); usage.Hits != 0 || usage.Misses != 4 {
		t.Fatal(usage)
	}
	deeper.SetPitfallBudget(0, 0)
	if usage := checkPitfallCacheRun(t, deeper, &cache, scope, source); usage.Hits != 0 || usage.Misses != 0 || usage.Stored != 0 {
		t.Fatal("budget bypass", usage)
	}
	actual32 := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: 32}, AnalysisStandard)
	if usage := checkPitfallCacheRun(t, actual32, &cache, scope, source); usage.Hits != 0 || usage.Misses != 4 {
		t.Fatal("native plan bits reused", usage)
	}
}

// TestPitfallCacheFailClosedAndDetachment refuses incomplete prerequisite
// closures, source errors and malformed configuration, and detaches revisions
// and output evidence from stored state.
// Rules: rules/compiler/incremental_compilation.md — §§2-3,7,29-34;
// rules/analysis/pitfall_analysis.md — "Analysis states", "Determinism".
func TestPitfallCacheFailClosedAndDetachment(t *testing.T) {
	source := pitfallCacheSource(t)
	scope := PitfallCacheScope{"pitfall_cache", "plan64", "v1"}
	var cache PitfallCache
	a := NewAnalyzerWithDepth(AnalysisStandard)
	p, revs := pitfallCacheFixture(t, source, AnalysisStandard)
	_, u, e := a.AnalyzeWithPitfallCache(p, &cache, scope, revs)
	if e != nil || u.Stored != 4 {
		t.Fatal(u, e)
	}
	// A host that cannot provide complete current prerequisites gets no hits.
	incomplete := append([]PitfallBodyRevision(nil), revs...)
	incomplete[0].Complete = false
	incomplete[1].Dependencies = nil
	incomplete[2].Dependencies = append([]PitfallDependency(nil), revs[2].Dependencies[1:]...)
	_, u, e = a.AnalyzeWithPitfallCache(p, &cache, scope, incomplete)
	if e != nil || u.Hits != 1 || u.Unavailable != 3 || u.Stored != 0 {
		t.Fatal(u, e)
	}
	// Detach setup input before publication and output after a cache hit.
	run, e := preparePitfallCacheRun(&cache, scope, revs)
	if e != nil {
		t.Fatal(e)
	}
	old := run.Bodies[revs[0].Body].Dependencies[0].Fingerprint
	revs[0].Dependencies[0].Fingerprint = "caller mutation"
	if run.Bodies[revs[0].Body].Dependencies[0].Fingerprint != old {
		t.Fatal("input storage shared")
	}
	p, revs = pitfallCacheFixture(t, source, AnalysisStandard)
	_, u, e = a.AnalyzeWithPitfallCache(p, &cache, scope, revs)
	if e != nil || u.Hits != 4 {
		t.Fatal(u, e)
	}
	snapshot := a.PitfallAnalysis()
	before := snapshot.Results()
	snapshot.results[0].EvidenceFor[0].Fact = "mutated snapshot"
	snapshot.results[0].Actions[0].Title = "mutated snapshot"
	_, u, e = a.AnalyzeWithPitfallCache(p, &cache, scope, revs)
	if e != nil || u.Hits != 4 || !reflect.DeepEqual(before, a.PitfallAnalysis().Results()) {
		t.Fatal("output storage shared", u, e)
	}
	malformed := append([]PitfallBodyRevision(nil), revs...)
	malformed[0].Dependencies = append(append([]PitfallDependency(nil), revs[0].Dependencies...), revs[0].Dependencies[0])
	actual, u, e := a.AnalyzeWithPitfallCache(p, &cache, scope, malformed)
	if e == nil || len(actual) != 0 || u != (PitfallCacheUsage{}) || !reflect.DeepEqual(before, a.PitfallAnalysis().Results()) {
		t.Fatal("cache error became source error", actual, u, e)
	}
	invalidScope := scope
	invalidScope.CompilerModel = ""
	if _, _, e = a.AnalyzeWithPitfallCache(p, &cache, invalidScope, revs); e == nil {
		t.Fatal("incomplete scope accepted")
	}
	invalidData, err := os.ReadFile("../../testdata/sema/pitfall_diagnostic_ownership_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	invalid := parser.New(lexer.NewWithFile(string(invalidData), "invalid.sec")).ParseProgram()
	cold := NewAnalyzer()
	expected := cold.Analyze(invalid)
	actual, u, e = a.AnalyzeWithPitfallCache(invalid, &cache, scope, nil)
	if e != nil || len(actual) == 0 || !reflect.DeepEqual(actual, expected) || u.Stored != 0 || u.Hits != 0 {
		t.Fatal(actual, u, e)
	}
	if !reflect.DeepEqual(a.PitfallAnalysis().Results(), cold.PitfallAnalysis().Results()) {
		t.Fatal("owning diagnostics changed under cache")
	}
	body := p.Statements[1].(*ast.FunctionDeclaration).Body
	recovered := *body
	recovered.Statements = []ast.Statement{&ast.InvalidStatement{Token: body.Token}}
	if pitfallCacheSyntaxComplete(reflect.ValueOf(&recovered)) {
		t.Fatal("recovery treated as complete")
	}
}

// TestPitfallCachePublication rejects invalidated, superseded, foreign and
// repeated publication leases while preserving unrelated pending units.
// Rules: rules/compiler/incremental_compilation.md — §§29-34;
// rules/analysis/pitfall_analysis.md — "Incremental analysis".
func TestPitfallCachePublication(t *testing.T) {
	scope := PitfallCacheScope{"module", "plan", "model"}
	key := pitfallCacheKey{scope, PitfallBodyID{"source.sec", 1, 1}}
	other := pitfallCacheKey{scope, PitfallBodyID{"source.sec", 2, 1}}
	deps := []PitfallDependency{{"summary", "callee", "v1"}}
	var cache, foreign PitfallCache
	_, _, old := cache.lookupOrBegin(key, [32]byte{1}, deps)
	_, _, newer := cache.lookupOrBegin(key, [32]byte{2}, deps)
	if cache.publish(old) {
		t.Fatal("obsolete lease published")
	}
	cache.discard(old)
	if !cache.publish(newer) || cache.publish(newer) {
		t.Fatal("latest lease not single-publication")
	}
	_, hit, _ := cache.lookupOrBegin(key, [32]byte{2}, deps)
	if !hit {
		t.Fatal("committed result unavailable")
	}
	_, _, old = cache.lookupOrBegin(key, [32]byte{3}, deps)
	_, _, unrelated := cache.lookupOrBegin(other, [32]byte{1}, []PitfallDependency{{"summary", "other-callee", "v1"}})
	if affected := cache.Invalidate("summary", "callee"); affected != 1 {
		t.Fatal(affected)
	}
	if cache.publish(old) || !cache.publish(unrelated) {
		t.Fatal("invalidation missed pending work or evicted unrelated")
	}
	_, _, newer = cache.lookupOrBegin(key, [32]byte{4}, deps)
	_, _, alien := foreign.lookupOrBegin(key, [32]byte{4}, deps)
	if cache.publish(alien) {
		t.Fatal("foreign lease published")
	}
	deps[0].Fingerprint = "v2"
	_, _, changed := cache.lookupOrBegin(key, [32]byte{5}, deps)
	if cache.publish(newer) || !cache.publish(changed) {
		t.Fatal("changed producer revision did not obsolete lease")
	}
	_, _, unknown := cache.lookupOrBegin(key, [32]byte{6}, deps)
	unknown.Entry.Payload.Findings = []PitfallFinding{{State: PitfallStateNotEvaluated}}
	if cache.publish(unknown) {
		t.Fatal("NotEvaluated cached as a complete result")
	}
}

// TestPitfallCacheForeignPayloadIsolation exercises actual FFI findings and
// deep contract/Place snapshots through repeated cache consumption.
// Rules: rules/analysis/pitfall_analysis.md — "Incremental analysis", "FFI pitfall analysis";
// rules/compiler/incremental_compilation.md — §§3,29-34.
func TestPitfallCacheForeignPayloadIsolation(t *testing.T) {
	a, program := foreignUnitFixture(t, AnalysisStandard, 64)
	revisions := []PitfallBodyRevision{}
	for _, stmt := range program.Statements {
		fn, ok := stmt.(*ast.FunctionDeclaration)
		if !ok || fn.Body == nil {
			continue
		}
		revision := PitfallBodyRevision{Body: PitfallBodyIdentity(fn.Body), Complete: true}
		for _, kind := range PitfallRequiredFactKinds(AnalysisStandard) {
			revision.Dependencies = append(revision.Dependencies, PitfallDependency{kind, fn.Name.Value + "/" + kind, "v1"})
		}
		revisions = append(revisions, revision)
	}
	scope := PitfallCacheScope{"foreign_units", "plan64", "model"}
	var cache PitfallCache
	before := a.PitfallAnalysis()
	_, u, e := a.AnalyzeWithPitfallCache(program, &cache, scope, revisions)
	if e != nil || u.Stored != len(revisions) {
		t.Fatal(u, e)
	}
	_, u, e = a.AnalyzeWithPitfallCache(program, &cache, scope, revisions)
	if e != nil || u.Hits != len(revisions) || !reflect.DeepEqual(before, a.PitfallAnalysis()) {
		t.Fatal(u, e, "FFI cache mismatch")
	}
	snapshot := a.PitfallAnalysis()
	if len(snapshot.foreignExtentInputs) == 0 {
		t.Fatal("no canonical FFI input")
	}
	for i := range snapshot.foreignExtentInputs {
		snapshot.foreignExtentInputs[i].ExtentUnit = ForeignExtentElements
		if snapshot.foreignExtentInputs[i].PointerOrigin != nil {
			snapshot.foreignExtentInputs[i].PointerOrigin.Root = "mutated origin"
		}
	}
	_, u, e = a.AnalyzeWithPitfallCache(program, &cache, scope, revisions)
	if e != nil || u.Hits != len(revisions) || !reflect.DeepEqual(before, a.PitfallAnalysis()) {
		t.Fatal(u, e, "FFI cache shares snapshot storage")
	}
	// Removing trusted contracts is a changed prerequisite, not a cached
	// positive or negative contract fact. Recompute all affected bodies.
	a.SetForeignBufferExtentContracts(nil)
	for i := range revisions {
		for j := range revisions[i].Dependencies {
			if strings.Contains(revisions[i].Dependencies[j].Kind, "foreign") {
				revisions[i].Dependencies[j].Fingerprint = "metadata-removed"
			}
		}
	}
	_, u, e = a.AnalyzeWithPitfallCache(program, &cache, scope, revisions)
	if e != nil || u.Hits != 0 || len(a.PitfallAnalysis().Findings()) != 0 || len(a.PitfallAnalysis().ForeignExtentInputs()) != 0 {
		t.Fatal(u, e, a.PitfallAnalysis())
	}
}

// TestPitfallCacheConcurrentSnapshots shares cached immutable payloads between
// independent analyzers while keeping each canonical semantic request local.
// Rules: rules/compiler/incremental_compilation.md — §§21,23,29-34;
// rules/analysis/pitfall_analysis.md — "Determinism".
func TestPitfallCacheConcurrentSnapshots(t *testing.T) {
	source := pitfallCacheSource(t)
	scope := PitfallCacheScope{"pitfall_cache", "plan64", "model"}
	var cache PitfallCache
	warm := NewAnalyzerWithDepth(AnalysisStandard)
	checkPitfallCacheRun(t, warm, &cache, scope, source)
	expected := warm.PitfallAnalysis()
	type result struct {
		Errors     []Error
		Usage      PitfallCacheUsage
		CacheError error
		Findings   *PitfallAnalysis
	}
	results := make(chan result, 4)
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		p, revisions := pitfallCacheFixture(t, source, AnalysisStandard)
		workers.Add(1)
		go func(program *ast.Program, revs []PitfallBodyRevision) {
			defer workers.Done()
			a := NewAnalyzerWithDepth(AnalysisStandard)
			errors, usage, err := a.AnalyzeWithPitfallCache(program, &cache, scope, revs)
			results <- result{errors, usage, err, a.PitfallAnalysis()}
		}(p, revisions)
	}
	workers.Wait()
	close(results)
	for actual := range results {
		if len(actual.Errors) != 0 || actual.CacheError != nil || actual.Usage.Hits != 4 || !reflect.DeepEqual(expected, actual.Findings) {
			t.Fatal(actual)
		}
	}
}
