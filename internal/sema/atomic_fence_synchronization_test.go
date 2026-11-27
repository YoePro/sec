package sema

import (
	"math/rand"
	"os"
	"reflect"
	"sort"
	"sync"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// fenceSynchronizationFixture obtains canonical facts from real source rather
// than manufacturing operation names, orders or storage identities.
// Rules: rules/concurrency/concurrency_memory_model.md §§24–33,52.
func fenceSynchronizationFixture(t *testing.T) (*Analyzer, []*ast.CallExpression) {
	t.Helper()
	data, err := os.ReadFile("../../testdata/sema/atomic_fence_synchronization.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "atomic_fence_synchronization.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	assertSemaErrors(t, a.Analyze(program), nil)
	calls := []*ast.CallExpression{}
	for call := range a.resolvedAtomicFences {
		calls = append(calls, call)
	}
	for call := range a.resolvedAtomicMemoryOperations {
		calls = append(calls, call)
	}
	sort.Slice(calls, func(i, j int) bool { return calls[i].Token.ByteStart < calls[j].Token.ByteStart })
	if len(calls) != 14 {
		t.Fatalf("calls: %d", len(calls))
	}
	return a, calls
}

// TestAtomicFenceSynchronizationPatterns covers every endpoint order, all three
// patterns, strict execution-local sequencing and exact observed communication.
// Rules: rules/concurrency/concurrency_memory_model.md §§29–33,55(13).
func TestAtomicFenceSynchronizationPatterns(t *testing.T) {
	a, calls := fenceSynchronizationFixture(t)
	for _, release := range []int{0, 2, 3} {
		for _, acquire := range []int{1, 2, 3} {
			events := []AtomicExecutionEvent{
				{ID: "release", Context: "producer", Sequence: 1, Call: calls[release]},
				{ID: "write", Context: "producer", Sequence: 2, Call: calls[4]},
				{ID: "read", Context: "consumer", Sequence: 1, Call: calls[7], ReadsFrom: "write"},
				{ID: "acquire", Context: "consumer", Sequence: 2, Call: calls[acquire]},
			}
			want := []AtomicFenceSynchronization{{"release", "acquire", "write", "write", "read"}, {"release", "read", "write", "write", "read"}}
			got, err := a.ResolveAtomicFenceSynchronization(events, NewSynchronizationResourceSnapshot())
			if err != nil || !reflect.DeepEqual(got.Edges, want) || len(got.UnresolvedReads) != 0 {
				t.Fatalf("%d/%d: %+v %v", release, acquire, got, err)
			}
			// Changing the actual read outcome to Relaxed removes only fence/atomic.
			events[2].Call = calls[6]
			got, err = a.ResolveAtomicFenceSynchronization(events, NewSynchronizationResourceSnapshot())
			if err != nil || !reflect.DeepEqual(got.Edges, want[:1]) {
				t.Fatalf("fence/fence %+v %v", got, err)
			}
			// No fences: a release Store and acquire Load belong to the separate
			// atomic/atomic rule, not this fence query.
			events[1].Call, events[2].Call = calls[5], calls[7]
			got, err = a.ResolveAtomicFenceSynchronization(events[1:3], NewSynchronizationResourceSnapshot())
			if err != nil || len(got.Edges) != 0 {
				t.Fatal(got, err)
			}
			events[2].Call = calls[6]
			got, err = a.ResolveAtomicFenceSynchronization(events[1:], NewSynchronizationResourceSnapshot())
			if err != nil || !reflect.DeepEqual(got.Edges, []AtomicFenceSynchronization{{"write", "acquire", "write", "write", "read"}}) {
				t.Fatalf("atomic/fence %+v %v", got, err)
			}
		}
	}
}

// TestAtomicFenceSynchronizationChains retains all release heads, rejects a
// Store break and respects independent CAS outcomes and missing predecessors.
// Rules: rules/concurrency/concurrency_memory_model.md §§17–18,24–31.
func TestAtomicFenceSynchronizationChains(t *testing.T) {
	a, calls := fenceSynchronizationFixture(t)
	events := []AtomicExecutionEvent{
		{ID: "head", Context: "producer", Sequence: 1, Call: calls[5]},
		{ID: "rmw", Context: "middle", Sequence: 1, Call: calls[9], ReadsFrom: "head"},
		{ID: "tail", Context: "last", Sequence: 1, Call: calls[8], ReadsFrom: "rmw"},
		{ID: "read", Context: "consumer", Sequence: 1, Call: calls[6], ReadsFrom: "tail"},
		{ID: "fence", Context: "consumer", Sequence: 2, Call: calls[1]},
	}
	state := NewSynchronizationResourceSnapshot()
	got, err := a.ResolveAtomicFenceSynchronization(events, state)
	want := []AtomicFenceSynchronization{{"head", "fence", "head", "tail", "read"}, {"rmw", "fence", "rmw", "tail", "read"}}
	if err != nil || !reflect.DeepEqual(got.Edges, want) {
		t.Fatal(got, err)
	}
	// A release fence before a relaxed carrier also survives contiguous RMWs.
	fenced := append([]AtomicExecutionEvent(nil), events...)
	fenced[0].Call, fenced[0].Sequence = calls[4], 2
	fenced = append(fenced, AtomicExecutionEvent{ID: "release", Context: "producer", Sequence: 1, Call: calls[0]})
	got, err = a.ResolveAtomicFenceSynchronization(fenced, state)
	fenceWant := []AtomicFenceSynchronization{{"release", "fence", "head", "tail", "read"}, {"rmw", "fence", "rmw", "tail", "read"}}
	if err != nil || !reflect.DeepEqual(got.Edges, fenceWant) {
		t.Fatal(got, err)
	}
	events[2].Call, events[2].ReadsFrom = calls[4], ""
	got, err = a.ResolveAtomicFenceSynchronization(events, state)
	if err != nil || len(got.Edges) != 0 {
		t.Fatalf("Store did not break chain: %+v %v", got, err)
	}
	events[2].Call, events[2].ReadsFrom = calls[10], "rmw"
	got, err = a.ResolveAtomicFenceSynchronization(events, state)
	if err != nil || !reflect.DeepEqual(got.Edges, want) {
		t.Fatalf("successful CAS: %+v %v", got, err)
	}
	events[2].Path = 1 // failed CAS reads but does not modify
	if got, err = a.ResolveAtomicFenceSynchronization(events, state); err == nil || len(got.Edges) != 0 {
		t.Fatal("failed CAS was a modification", got, err)
	}
	events[2].Path, events[2].ReadsFrom = 0, ""
	got, err = a.ResolveAtomicFenceSynchronization(events, state)
	if err != nil || len(got.Edges) != 0 || !reflect.DeepEqual(got.UnresolvedReads, []string{"read", "tail"}) {
		t.Fatal(got, err)
	}
	// Failed acquire CAS itself can carry publication to a later acquire fence.
	events = []AtomicExecutionEvent{events[0], {ID: "failed", Context: "consumer", Sequence: 1, Call: calls[10], Path: 1, ReadsFrom: "head"}, events[4]}
	got, err = a.ResolveAtomicFenceSynchronization(events, state)
	if err != nil || !reflect.DeepEqual(got.Edges, []AtomicFenceSynchronization{{"head", "fence", "head", "head", "failed"}}) {
		t.Fatal(got, err)
	}
}

// TestAtomicFenceSynchronizationUncertainty never infers communication from
// source order, alias possibility, unrelated storage or a missing ownership state.
// Rules: rules/concurrency/concurrency_memory_model.md §§28–33,52,55(13).
func TestAtomicFenceSynchronizationUncertainty(t *testing.T) {
	a, calls := fenceSynchronizationFixture(t)
	base := []AtomicExecutionEvent{
		{ID: "release", Context: "producer", Sequence: 1, Call: calls[0]},
		{ID: "write", Context: "producer", Sequence: 2, Call: calls[4]},
		{ID: "read", Context: "consumer", Sequence: 1, Call: calls[7], ReadsFrom: "write"},
		{ID: "acquire", Context: "consumer", Sequence: 2, Call: calls[1]},
	}
	cases := []struct {
		name       string
		change     func([]AtomicExecutionEvent)
		state      *SynchronizationResourceSnapshot
		unresolved bool
	}{
		{"no communication", func(e []AtomicExecutionEvent) { e[2].ReadsFrom = "" }, NewSynchronizationResourceSnapshot(), true},
		{"may alias", func(e []AtomicExecutionEvent) { e[2].Call = calls[11] }, NewSynchronizationResourceSnapshot(), true},
		{"no state", func(e []AtomicExecutionEvent) {}, nil, true},
		{"late release fence", func(e []AtomicExecutionEvent) { e[0].Sequence = 3 }, NewSynchronizationResourceSnapshot(), false},
		{"unrelated execution", func(e []AtomicExecutionEvent) { e[0].Context = "unrelated" }, NewSynchronizationResourceSnapshot(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			events := append([]AtomicExecutionEvent(nil), base...)
			tc.change(events)
			got, err := a.ResolveAtomicFenceSynchronization(events, tc.state)
			if err != nil || len(got.Edges) != 0 || (len(got.UnresolvedReads) > 0) != tc.unresolved {
				t.Fatal(got, err)
			}
		})
	}
	// An early acquire fence cannot acquire a later read's publication.
	events := append([]AtomicExecutionEvent(nil), base...)
	events[2].Call = calls[6]
	events[3].Sequence = 1
	events[2].Sequence = 2
	got, err := a.ResolveAtomicFenceSynchronization(events, NewSynchronizationResourceSnapshot())
	if err != nil || len(got.Edges) != 0 {
		t.Fatal(got, err)
	}
	// Proven disjoint fields make the proposed read-from witness invalid.
	events[1].Call = calls[12]
	events[2].Call = calls[13]
	if got, err = a.ResolveAtomicFenceSynchronization(events, NewSynchronizationResourceSnapshot()); err == nil || len(got.Edges) != 0 {
		t.Fatal(got, err)
	}
}

// TestAtomicFenceSynchronizationValidation rejects malformed witnesses without
// partially publishing edges, including foreign snapshots and impossible chains.
// Rules: rules/concurrency/concurrency_memory_model.md §§16–18,24–33,52.
func TestAtomicFenceSynchronizationValidation(t *testing.T) {
	a, calls := fenceSynchronizationFixture(t)
	base := []AtomicExecutionEvent{
		{ID: "write", Context: "producer", Sequence: 1, Call: calls[5]},
		{ID: "read", Context: "consumer", Sequence: 1, Call: calls[6], ReadsFrom: "write"},
		{ID: "fence", Context: "consumer", Sequence: 2, Call: calls[1]},
	}
	cases := []func([]AtomicExecutionEvent){
		func(e []AtomicExecutionEvent) { e[0].ID = "" },
		func(e []AtomicExecutionEvent) { e[0].Context = "" },
		func(e []AtomicExecutionEvent) { e[0].Sequence = 0 },
		func(e []AtomicExecutionEvent) { e[1].ID = "write" },
		func(e []AtomicExecutionEvent) { e[2].Sequence = 1 },
		func(e []AtomicExecutionEvent) { e[0].Call = nil },
		func(e []AtomicExecutionEvent) { e[1].Path = -1 },
		func(e []AtomicExecutionEvent) { e[1].Path = 3 },
		func(e []AtomicExecutionEvent) { e[2].Path = 1 },
		func(e []AtomicExecutionEvent) { e[2].ReadsFrom = "write" },
		func(e []AtomicExecutionEvent) { e[0].ReadsFrom = "read" },
		func(e []AtomicExecutionEvent) { e[1].ReadsFrom = "absent" },
		func(e []AtomicExecutionEvent) { e[1].ReadsFrom = "fence" },
		func(e []AtomicExecutionEvent) { e[1].Context = "producer"; e[1].Sequence = 2; e[0].Sequence = 3 },
		func(e []AtomicExecutionEvent) { e[0].Call = calls[8]; e[1].Call = calls[8]; e[0].ReadsFrom = "read" },
	}
	for n, change := range cases {
		events := append([]AtomicExecutionEvent(nil), base...)
		change(events)
		got, err := a.ResolveAtomicFenceSynchronization(events, NewSynchronizationResourceSnapshot())
		if err == nil || len(got.Edges) != 0 {
			t.Fatalf("case %d: %+v %v", n, got, err)
		}
	}
	events := append(append([]AtomicExecutionEvent(nil), base...), AtomicExecutionEvent{ID: "fork", Context: "other", Sequence: 1, Call: calls[8], ReadsFrom: "write"})
	events[1].Call = calls[8]
	if got, err := a.ResolveAtomicFenceSynchronization(events, NewSynchronizationResourceSnapshot()); err == nil || len(got.Edges) != 0 {
		t.Fatal(got, err)
	}
	// Stale AST identities cannot borrow authority from a later analysis.
	old := append([]AtomicExecutionEvent(nil), base...)
	clean := parser.New(lexer.New("module main\nfn Clean() void {}\n")).ParseProgram()
	assertSemaErrors(t, a.Analyze(clean), nil)
	if got, err := a.ResolveAtomicFenceSynchronization(old, NewSynchronizationResourceSnapshot()); err == nil || len(got.Edges) != 0 {
		t.Fatal(got, err)
	}
	if _, err := a.ResolveAtomicFenceSynchronization(old[2:], NewSynchronizationResourceSnapshot()); err == nil {
		t.Fatal("stale fence survived reanalysis")
	}
}

// TestAtomicFenceSynchronizationRequiresCanonicalSources excludes invalid,
// unreachable and shadowed calls and rejects AST identities from other snapshots.
// Rules: rules/concurrency/atomics.md §§26,40,45;
// rules/concurrency/concurrency_memory_model.md §§29–33,52.
func TestAtomicFenceSynchronizationRequiresCanonicalSources(t *testing.T) {
	for _, name := range []string{"atomic_fence_invalid.sec", "atomic_fence_shadowed_valid.sec", "atomic_fence_effects_unreachable_invalid.sec"} {
		data, err := os.ReadFile("../../testdata/sema/" + name)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		a.Analyze(program)
		count := 0
		if err := walk.Inspect(program, func(node any) error {
			call, ok := node.(*ast.CallExpression)
			if !ok || callExpressionName(call) != "atomic.Fence" {
				return nil
			}
			count++
			events := []AtomicExecutionEvent{{ID: "candidate", Context: "execution", Sequence: 1, Call: call}}
			if got, err := a.ResolveAtomicFenceSynchronization(events, NewSynchronizationResourceSnapshot()); err == nil || len(got.Edges) != 0 {
				t.Fatalf("%s published unauthorized fence: %+v %v", name, got, err)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			t.Fatal("fixture had no fence candidate", name)
		}
	}
	a, calls := fenceSynchronizationFixture(t)
	_, foreign := fenceSynchronizationFixture(t)
	for _, call := range []*ast.CallExpression{foreign[0], foreign[4]} {
		if _, err := a.ResolveAtomicFenceSynchronization([]AtomicExecutionEvent{{ID: "foreign", Context: "execution", Sequence: 1, Call: call}}, NewSynchronizationResourceSnapshot()); err == nil {
			t.Fatal("accepted foreign AST identity")
		}
	}
	var absent *Analyzer
	if _, err := absent.ResolveAtomicFenceSynchronization([]AtomicExecutionEvent{{ID: "event", Context: "execution", Sequence: 1, Call: calls[0]}}, nil); err == nil {
		t.Fatal("accepted absent analysis")
	}
}

// TestAtomicFenceSynchronizationDeterminism verifies witness permutation and
// detached concurrent queries; the result contains no mutable analyzer data.
// Rules: rules/concurrency/concurrency_memory_model.md §§29–33,52.
func TestAtomicFenceSynchronizationDeterminism(t *testing.T) {
	a, calls := fenceSynchronizationFixture(t)
	events := []AtomicExecutionEvent{
		{ID: "release", Context: "producer", Sequence: 1, Call: calls[0]},
		{ID: "write", Context: "producer", Sequence: 2, Call: calls[4]},
		{ID: "read", Context: "consumer", Sequence: 1, Call: calls[7], ReadsFrom: "write"},
		{ID: "acquire", Context: "consumer", Sequence: 2, Call: calls[1]},
	}
	// Prime a snapshot before parallel readers; queries do not mutate it.
	state := NewSynchronizationResourceSnapshot()
	want, err := a.ResolveAtomicFenceSynchronization(events, state)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(17))
	for n := 0; n < 50; n++ {
		rng.Shuffle(len(events), func(i, j int) { events[i], events[j] = events[j], events[i] })
		got, err := a.ResolveAtomicFenceSynchronization(events, state)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatal(got, err)
		}
	}
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				got, err := a.ResolveAtomicFenceSynchronization(events, state)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Error(got, err)
					return
				}
				got.Edges[0].From = "mutated"
			}
		}()
	}
	wg.Wait()
}
