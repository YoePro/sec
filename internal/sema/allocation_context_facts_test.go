package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestAllocationContextFacts checks canonical selection across moves, distinct
// equally named Arena owners, implicit materialization, constructor backing,
// all synchronous paths, deferred work, recursion and spawn boundaries.
// Rules: rules/memory/allocation.md — §§5,6,17,29(1),(3); rules/memory/arena.md — §4.2.
func TestAllocationContextFacts(t *testing.T) {
	file := "../../testdata/sema/allocation_context_facts_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	graph := a.CallGraph()
	contexts := func(name string) []AllocationContextFact {
		return a.AllocationContexts(callGraphNodeIDByName(t, graph, name))
	}
	render := contexts("Render")
	if len(render) != 1 || !render[0].ContextKnown || !render[0].Context.Available || render[0].Context.Profile != "hosted" || render[0].Context.Origin != StorageOriginArena || render[0].SelectedDomain != "" {
		t.Fatal(render)
	}
	if interpolation := contexts("Interpolate"); len(interpolation) != 1 || interpolation[0].Context != render[0].Context {
		t.Fatal(interpolation)
	}
	if len(contexts("Folded")) != 0 {
		t.Fatal("folded text acquired a runtime context")
	}
	alloc := contexts("Allocate")
	if len(alloc) != 2 || alloc[0].SelectedDomain == "" || alloc[0].SelectedDomain != alloc[1].SelectedDomain || alloc[0].Arena != "mem" || alloc[1].Arena != "moved" || alloc[0].ContextKnown || alloc[1].CreatedDomain != "" {
		t.Fatal(alloc)
	}
	other := contexts("OtherDomain")
	if len(other) != 1 || other[0].Arena != "mem" || other[0].SelectedDomain == "" || other[0].SelectedDomain == alloc[0].SelectedDomain {
		t.Fatal("lexical names conflated distinct domains", other, alloc)
	}
	created := contexts("Created")
	if len(created) != 1 || created[0].CreatedDomain == "" || created[0].SelectedDomain != "" || created[0].ContextKnown {
		t.Fatal("created Arena was used as its backing context", created)
	}
	for _, name := range []string{"Forward", "Recursive", "Deferred"} {
		facts := contexts(name)
		if len(facts) != 1 || facts[0].Context != render[0].Context || len(facts[0].Path) < 2 {
			t.Fatal(name, facts)
		}
	}
	combined := contexts("Combined")
	if len(combined) != 4 {
		t.Fatal("not all transitive context sites retained", combined)
	}
	unresolved := 0
	for _, fact := range combined {
		if !fact.ContextKnown && fact.SelectedDomain == "" {
			unresolved++
		}
	}
	if unresolved != 1 {
		t.Fatal("foreign context was guessed", combined)
	}
	if len(contexts("Spawner")) != 0 {
		t.Fatal("spawned worker context leaked into caller")
	}
	before := a.AllocationContextFacts()
	for _, fact := range combined {
		fact.Path[0] = "corrupted"
	}
	combined[0].SelectedDomain = "corrupted"
	if !reflect.DeepEqual(before, a.AllocationContextFacts()) {
		t.Fatal("mutable context snapshots")
	}
	if len(a.AllocationContexts("missing")) != 0 {
		t.Fatal("invented missing callable context")
	}
	fresh := NewAnalyzer()
	if errs := fresh.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	if !reflect.DeepEqual(before, fresh.AllocationContextFacts()) {
		t.Fatal("unstable fresh context projection")
	}
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	// Compiler metadata identifies abstract creation sites deterministically,
	// independently of whether the analyzer instance is fresh or reused. These
	// are not runtime allocation identities or permission to reuse old live facts.
	// Rules: compiler/compiler.md §71; compiler_pipeline.md §76;
	// memory/arena.md §§4.2,44.
	if !reflect.DeepEqual(before, a.AllocationContextFacts()) {
		t.Fatal("unstable reused-analyzer context projection")
	}
	// The projection must not rerun inference or alter semantic result types.
	for expression, typ := range a.expressionTypes {
		if _, ok := expression.(*ast.CallExpression); ok && typ.Kind == ResultType {
			if actual, _ := a.ResolvedTypeOf(expression); !reflect.DeepEqual(typ, actual) {
				t.Fatal("type mutation")
			}
		}
	}
}

// TestAllocationContextProfiles checks target-resolved availability (including
// rejected source), leaving profile semantics and failure diagnostics unchanged.
// Rules: rules/memory/allocation.md — §§17(7)-(9),22(4),29(1),(3).
func TestAllocationContextProfiles(t *testing.T) {
	file := "../../testdata/sema/allocation_context_facts_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"hosted", "embedded-arena", "noalloc", "freestanding"} {
		t.Run(profile, func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(string(data), file))
			program := p.ParseProgram()
			a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "baremetal", TargetArch: "cortex-m3", Profile: profile, PointerWidthBits: 32})
			errs := a.Analyze(program)
			available := profile == "hosted" || profile == "embedded-arena"
			if available && len(errs) != 0 {
				t.Fatal(errs)
			}
			if !available && len(errs) != 2 {
				t.Fatal("expected missing-context errors on both runtime strings", errs)
			}
			facts := a.AllocationContexts(callGraphNodeIDByName(t, a.CallGraph(), "Render"))
			if len(facts) != 1 || !facts[0].ContextKnown || facts[0].Context.Profile != profile || facts[0].Context.Available != available {
				t.Fatal(facts)
			}
			description := AllocationContextDescription(facts[0])
			if !strings.Contains(description, profile) || !available && !strings.Contains(description, "unavailable") {
				t.Fatal(description)
			}
		})
	}
}

// TestAllocationContextMissingAndAmbiguousEvidence ensures that source labels
// and inconsistent cloned-call metadata cannot manufacture a selected domain.
// Rules: rules/memory/allocation.md — §§5(4),6(4),24(6),29(1),(3).
func TestAllocationContextMissingAndAmbiguousEvidence(t *testing.T) {
	file := "../../testdata/sema/allocation_context_facts_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	a := NewAnalyzer()
	if errs := a.Analyze(p.ParseProgram()); len(errs) != 0 {
		t.Fatal(errs)
	}
	original := a.expressionTypes
	a.expressionTypes = nil
	for _, fact := range a.AllocationContextFacts() {
		if fact.SelectedDomain != "" || fact.CreatedDomain != "" {
			t.Fatal("domain guessed without canonical type", fact)
		}
	}
	a.expressionTypes = original
	var location sourceTokenKey
	for expression, typ := range original {
		call, ok := expression.(*ast.CallExpression)
		if !ok || typ.Kind != ResultType || len(typ.TypeArgs) == 0 || typ.TypeArgs[0].Kind != ReferenceType || typ.TypeArgs[0].ReferenceOriginStorage != StorageOriginArena {
			continue
		}
		copyCall := *call
		copyType := typ
		copyType.TypeArgs = append([]Type(nil), typ.TypeArgs...)
		copyType.TypeArgs[0].ReferenceOriginName = "conflicting-domain"
		original[&copyCall] = copyType
		location = sourceTokenLocation(callCalleeDefinitionToken(call))
		break
	}
	if location == (sourceTokenKey{}) {
		t.Fatal("missing represented explicit allocation")
	}
	baseline := a.AllocationContextFacts()
	for iteration := 0; iteration < 20; iteration++ {
		actual := a.AllocationContextFacts()
		if !reflect.DeepEqual(baseline, actual) {
			t.Fatal("map traversal changed ambiguous selection")
		}
		found := false
		for _, fact := range actual {
			if sourceTokenLocation(fact.Source) == location {
				found = true
				if fact.SelectedDomain != "" || fact.ContextKnown {
					t.Fatal("conflicting domain chosen", fact)
				}
			}
		}
		if !found {
			t.Fatal("unknown site disappeared")
		}
	}
}
