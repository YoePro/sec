package sema

import (
	"math/big"
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestSynchronizationResourceRelationships verifies all four identity states,
// exact projections, opaque snapshot scopes, and conservative alias joins.
// Rules: rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource identity".
func TestSynchronizationResourceRelationships(t *testing.T) {
	semantics := SynchronizationResourceSemantics{ContractID: "sec.Mutex", Capacity: big.NewInt(1), Reentrancy: ResourceNonReentrant}
	root := Place{Root: "lock", RootID: 1, RootKind: PlaceRootLocal}
	other := root
	other.RootID = 2 // The same spelling is not the same resource.
	renamed := root
	renamed.Root = "entirelyDifferentSpelling"
	unnamed := root
	unnamed.Root = "" // RootID is authoritative even without presentation.
	missingID := root
	missingID.RootID = 0
	deref := root
	deref.RootKind = PlaceRootDeref
	deref.Projections = []PlaceProjection{{Kind: PlaceDereference}}
	otherDeref := deref
	otherDeref.RootID = 2
	field := func(name string) Place {
		p := root
		p.Projections = []PlaceProjection{{Kind: PlaceField, Name: name}}
		return p
	}
	index := func(value string) Place {
		p := root
		n, ok := new(big.Int).SetString(value, 10)
		if !ok {
			t.Fatal(value)
		}
		p.Projections = []PlaceProjection{{Kind: PlaceIndex, ConstantIndex: n}}
		return p
	}
	dynamic := root
	dynamic.Projections = []PlaceProjection{{Kind: PlaceIndex, DynamicIndex: true}}
	property := field("Lock")
	property.Projections[0].Kind = PlaceProperty
	slice := root
	slice.Projections = []PlaceProjection{{Kind: PlaceSlice}}
	invalidProjection := root
	invalidProjection.Projections = []PlaceProjection{{Kind: "unrecognized"}}
	unknownRoot := root
	unknownRoot.RootKind = ""
	ambiguous := root
	ambiguous.AmbiguousProvenance = true
	alternatives := root
	alternatives.AlternativeOrigins = []Place{other}
	disjointAlternatives := other
	disjointAlternatives.AlternativeOrigins = []Place{{RootID: 3, RootKind: PlaceRootLocal}}
	missingAlternative := root
	missingAlternative.AlternativeOrigins = []Place{missingID}
	unresolvedRef := root
	unresolvedRef.Type.Kind = ReferenceType
	conflictingRole := root
	conflictingRole.RootKind = PlaceRootStatic
	tests := []struct {
		name        string
		left, right Place
		want        SynchronizationResourceRelationship
	}{
		{"same", root, root, ResourceSame},
		{"rename", root, renamed, ResourceSame},
		{"unnamed", root, unnamed, ResourceSame},
		{"same spelling distinct bindings", root, other, ResourceDisjoint},
		{"no name fallback", missingID, missingID, ResourceUnknown},
		{"different ref parameters", deref, otherDeref, ResourceMayAlias},
		{"ref may denote owned resource", root, otherDeref, ResourceMayAlias},
		{"same ref", deref, deref, ResourceSame},
		{"different stored fields", field("First"), field("Second"), ResourceDisjoint},
		{"same stored field", field("First"), field("First"), ResourceSame},
		{"strict prefix", root, field("First"), ResourceMayAlias},
		{"wide exact same", index("9223372036854775808"), index("9223372036854775808"), ResourceSame},
		{"wide exact distinct", index("9223372036854775808"), index("9223372036854775809"), ResourceDisjoint},
		{"dynamic versus exact", dynamic, index("0"), ResourceMayAlias},
		{"dynamic versus dynamic", dynamic, dynamic, ResourceMayAlias},
		{"getter", property, property, ResourceUnknown},
		{"unnormalized slice", slice, slice, ResourceUnknown},
		{"unsupported projection", invalidProjection, root, ResourceUnknown},
		{"missing root role", unknownRoot, root, ResourceUnknown},
		{"ambiguous provenance", ambiguous, root, ResourceUnknown},
		{"mixed alternatives", alternatives, root, ResourceMayAlias},
		{"all alternatives disjoint", disjointAlternatives, root, ResourceDisjoint},
		{"unknown alternative dominates", missingAlternative, root, ResourceUnknown},
		{"reference holder is not referent", unresolvedRef, unresolvedRef, ResourceUnknown},
		{"inconsistent root role", root, conflictingRole, ResourceUnknown},
	}
	snapshot := NewSynchronizationResourceSnapshot()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left := snapshot.Identify(NewSynchronizationResourceFact(tt.left, semantics))
			right := snapshot.Identify(NewSynchronizationResourceFact(tt.right, semantics))
			for _, pair := range [][2]SynchronizationResourceIdentity{{left, right}, {right, left}} {
				if got := SynchronizationRelationship(pair[0], pair[1]); got != tt.want {
					t.Fatalf("relationship = %s, want %s", got, tt.want)
				}
			}
		})
	}
	identity := snapshot.Identify(NewSynchronizationResourceFact(root, semantics))
	otherState := NewSynchronizationResourceSnapshot().Identify(NewSynchronizationResourceFact(root, semantics))
	if SynchronizationRelationship(identity, otherState) != ResourceUnknown || SynchronizationRelationship(identity, SynchronizationResourceIdentity{}) != ResourceUnknown {
		t.Fatal("an unrelated snapshot or zero value proved resource identity")
	}
	var nilSnapshot *SynchronizationResourceSnapshot
	if SynchronizationRelationship(nilSnapshot.Identify(NewSynchronizationResourceFact(root, semantics)), identity) != ResourceUnknown {
		t.Fatal("nil snapshot proved identity")
	}
}

// TestSynchronizationResourceContractsAndSnapshots verifies resource semantics
// are not inferred as mutex defaults and all published provenance is detached.
// Rules: rules/analysis/deadlock_analysis.md — "Resource capacity", "Reentrant resources";
// rules/compiler/compiler_analysis.md — immutable analysis results.
func TestSynchronizationResourceContractsAndSnapshots(t *testing.T) {
	place := Place{RootID: 1, RootKind: PlaceRootLocal, Projections: []PlaceProjection{{Kind: PlaceIndex, ConstantIndex: big.NewInt(4)}},
		Type:               Type{TypeArgs: []Type{{Name: "Original"}}},
		AlternativeOrigins: []Place{{RootID: 1, RootKind: PlaceRootLocal, Projections: []PlaceProjection{{Kind: PlaceIndex, ConstantIndex: big.NewInt(4)}}}},
	}
	contract := SynchronizationResourceSemantics{ContractID: "foreign.counting-semaphore", Capacity: big.NewInt(8), Reentrancy: ResourceReentrant}
	fact := NewSynchronizationResourceFact(place, contract)
	place.Projections[0].ConstantIndex.SetInt64(9)
	place.AlternativeOrigins[0].Projections[0].ConstantIndex.SetInt64(9)
	place.Type.TypeArgs[0].Name = "Changed"
	contract.Capacity.SetInt64(1)
	got := fact.Place()
	if got.Projections[0].ConstantIndex.Int64() != 4 || got.AlternativeOrigins[0].Projections[0].ConstantIndex.Int64() != 4 || got.Type.TypeArgs[0].Name != "Original" || fact.Semantics().Capacity.Int64() != 8 {
		t.Fatal("producer data aliases published facts")
	}
	got.Projections[0].ConstantIndex.SetInt64(7)
	got.Type.TypeArgs[0].Name = "Changed again"
	publishedContract := fact.Semantics()
	publishedContract.Capacity.SetInt64(2)
	if fact.Place().Projections[0].ConstantIndex.Int64() != 4 || fact.Place().Type.TypeArgs[0].Name != "Original" || fact.Semantics().Capacity.Int64() != 8 {
		t.Fatal("consumer data aliases published facts")
	}
	snapshot := NewSynchronizationResourceSnapshot()
	identity := snapshot.Identify(fact)
	if SynchronizationRelationship(identity, identity) != ResourceSame {
		t.Fatal("identical contracted resource lost identity")
	}
	for _, changed := range []SynchronizationResourceSemantics{
		{}, {ContractID: "different"},
		{ContractID: "foreign.counting-semaphore", Capacity: big.NewInt(1), Reentrancy: ResourceReentrant},
		{ContractID: "foreign.counting-semaphore", Capacity: big.NewInt(8), Reentrancy: ResourceNonReentrant},
		{ContractID: "foreign.counting-semaphore", Capacity: big.NewInt(0), Reentrancy: ResourceReentrant},
	} {
		if SynchronizationRelationship(identity, snapshot.Identify(NewSynchronizationResourceFact(fact.Place(), changed))) != ResourceUnknown {
			t.Fatal("incompatible resource semantics proved identity", changed)
		}
	}
	unknownCapacity := NewSynchronizationResourceFact(fact.Place(), SynchronizationResourceSemantics{ContractID: "foreign.resource"})
	if unknownCapacity.Semantics().Capacity != nil || unknownCapacity.Semantics().Reentrancy != "" {
		t.Fatal("unknown resource semantics became mutex defaults")
	}
	unknownIdentity := snapshot.Identify(unknownCapacity)
	if SynchronizationRelationship(unknownIdentity, unknownIdentity) != ResourceSame {
		t.Fatal("unknown capacity should not erase proven identity")
	}
}

// TestSynchronizationResourceSourceFacts exercises canonical Place/referent
// producers on real Sec source, depth independence, immutable facts, and reset.
// Rules: rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource identity";
// rules/concurrency/mutex.md — §§1(2), 5(3–4).
func TestSynchronizationResourceSourceFacts(t *testing.T) {
	file := "../../testdata/sema/synchronization_resource_identity_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithDepth(depth)
		assertSemaErrors(t, a.Analyze(program), nil)
		facts := map[string]SynchronizationResourceFact{}
		expressions := map[string]ast.Expression{}
		var referents []SynchronizationResourceFact
		var oldExpression ast.Expression
		walkASTValue(reflect.ValueOf(program), func(node any) {
			if identifier, ok := node.(*ast.Identifier); ok {
				if fact, found := a.SynchronizationResourceOf(identifier); found && identifier.Value == "first" && fact.Place().RootKind == PlaceRootLocal {
					referents = append(referents, fact)
				}
			}
			let, ok := node.(*ast.LetStatement)
			if !ok {
				return
			}
			prefix, ok := let.Value.(*ast.RefExpression)
			if !ok {
				return
			}
			if fact, found := a.SynchronizationResourceOf(prefix.Value); found {
				facts[let.Name.Value] = fact
				expressions[let.Name.Value] = prefix.Value
				oldExpression = prefix.Value
			}
		})
		for _, name := range []string{"first", "alias", "second"} {
			fact, found := facts[name]
			if !found || fact.Place().RootID == 0 || fact.Semantics().ContractID != "sec.Mutex" || fact.Semantics().Capacity.Cmp(big.NewInt(1)) != 0 || fact.Semantics().Reentrancy != ResourceNonReentrant {
				t.Fatalf("missing canonical mutex fact %s: %+v", name, fact)
			}
		}
		// The three observations share unchanged owned state in this fixture;
		// qualify them in one snapshot after that ownership fact is established.
		snapshot := NewSynchronizationResourceSnapshot()
		first, alias, second := snapshot.Identify(facts["first"]), snapshot.Identify(facts["alias"]), snapshot.Identify(facts["second"])
		if SynchronizationRelationship(first, alias) != ResourceSame || SynchronizationRelationship(first, second) != ResourceDisjoint {
			t.Fatal("canonical stored-field provenance was not preserved")
		}
		if len(referents) == 0 {
			t.Fatal("reference-argument resource provenance was not published")
		}
		for _, referent := range referents {
			if SynchronizationRelationship(first, snapshot.Identify(referent)) != ResourceSame {
				t.Fatal("reference binding identity replaced canonical referent identity")
			}
		}
		published := facts["first"].Place()
		published.Projections[0].Name = "corrupted"
		if fresh, _ := a.SynchronizationResourceOf(expressions["first"]); fresh.Place().Projections[0].Name == "corrupted" {
			t.Fatal("source query aliases internal provenance")
		}
		oldFact, _ := a.SynchronizationResourceOf(oldExpression)
		assertSemaErrors(t, a.Analyze(program), nil)
		freshFact, _ := a.SynchronizationResourceOf(oldExpression)
		if SynchronizationRelationship(snapshot.Identify(oldFact), snapshot.Identify(freshFact)) != ResourceUnknown {
			t.Fatal("reused root IDs from a new analysis proved resource identity")
		}
		if len(a.synchronizationResources) == 0 {
			t.Fatal("reanalyzing lost resource facts")
		}
		empty := parser.New(lexer.New("module empty\nfn Empty() void {}\n")).ParseProgram()
		assertSemaErrors(t, a.Analyze(empty), nil)
		if _, found := a.SynchronizationResourceOf(oldExpression); found {
			t.Fatal("resource facts survived Analyzer reset")
		}
	}
	var nilAnalyzer *Analyzer
	if _, found := nilAnalyzer.SynchronizationResourceOf(nil); found {
		t.Fatal("nil analyzer returned a resource")
	}
}
