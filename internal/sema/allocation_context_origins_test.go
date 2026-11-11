package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Domain provenance is the compiler's creation/owner source, including through
// owner moves and transitive calls; ambient/unknown context gets no fake origin.
// Rules: rules/memory/allocation.md §§5,6,29;
// rules/memory/arena.md §§4.2,4.3,67(4); rules/compiler/compiler.md §71.
func TestAllocationContextOrigins(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/allocation_context_facts_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a := NewAnalyzer()
	analyze := func(text string) {
		p := parser.New(lexer.NewWithFile(text, "allocation-context.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
	}
	analyze(string(data))
	allocate := a.AllocationContexts(callGraphNodeIDByName(t, a.CallGraph(), "Allocate"))
	if len(allocate) != 2 || allocate[0].SelectedDomainOrigin.Line != 11 || allocate[0].SelectedDomainOrigin.File != "allocation-context.sec" || allocate[0].SelectedDomainOrigin != allocate[1].SelectedDomainOrigin || allocate[1].Arena != "moved" {
		t.Fatal("move replaced context origin", allocate)
	}
	other := a.AllocationContexts(callGraphNodeIDByName(t, a.CallGraph(), "OtherDomain"))
	if len(other) != 1 || other[0].SelectedDomainOrigin.Line != 25 || other[0].SelectedDomainOrigin == allocate[0].SelectedDomainOrigin {
		t.Fatal("same-spelled receivers conflated origins", other)
	}
	created := a.AllocationContexts(callGraphNodeIDByName(t, a.CallGraph(), "Created"))
	if len(created) != 1 || created[0].CreatedDomainOrigin.Line != 30 || created[0].SelectedDomainOrigin.Line != 0 || !strings.Contains(AllocationContextDescription(created[0]), "backing allocation context unresolved") {
		t.Fatal("created origin mistaken for backing", created)
	}
	origins := map[string]lexer.Token{}
	for _, identity := range a.GeneratedIdentities() {
		origins[identity.ID] = identity.Source
	}
	for _, fact := range a.AllocationContextFacts() {
		if fact.SelectedDomainOrigin != origins[fact.SelectedDomain] || fact.CreatedDomainOrigin != origins[fact.CreatedDomain] {
			t.Fatal("noncanonical provenance", fact)
		}
		if fact.ContextKnown && (fact.SelectedDomainOrigin.Line != 0 || fact.CreatedDomainOrigin.Line != 0) {
			t.Fatal("invented implicit source origin", fact)
		}
	}
	combined := a.AllocationContexts(callGraphNodeIDByName(t, a.CallGraph(), "Combined"))
	selected := 0
	for _, fact := range combined {
		if fact.SelectedDomain != "" {
			selected++
			if fact.SelectedDomainOrigin != allocate[0].SelectedDomainOrigin || len(fact.Path) < 2 {
				t.Fatal("lost transitive origin", fact)
			}
		}
	}
	if selected != 2 {
		t.Fatal(combined)
	}
	unknown := allocate[0]
	unknown.SelectedDomainOrigin = lexer.Token{}
	if !strings.Contains(AllocationContextDescription(unknown), "allocation context origin unresolved") {
		t.Fatal("missing provenance guessed", unknown)
	}
	// A subsequent snapshot must refresh coordinates without mutating old facts.
	analyze("\n" + string(data))
	updated := a.AllocationContexts(callGraphNodeIDByName(t, a.CallGraph(), "Allocate"))
	if updated[0].SelectedDomainOrigin.Line != 12 || allocate[0].SelectedDomainOrigin.Line != 11 || updated[0].SelectedDomain != allocate[0].SelectedDomain {
		t.Fatal("stale source origin or unstable identity", updated, allocate)
	}
}
