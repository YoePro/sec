package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/lexer"
)

// Root identity follows the declaration rather than the spelling: two
// bindings with the same name are different roots, one declaration keeps one
// identity, identities are allocated deterministically in first-use order,
// and Places without an identity fall back to the display name.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §10 "Stable place root identity", §15, §138 "Required Sema place tests"
func TestPlaceRootIdentityFollowsDeclarations(t *testing.T) {
	analyzer := NewAnalyzer()
	first := lexer.Token{Lexeme: "value", Line: 3, Column: 9}
	sibling := lexer.Token{Lexeme: "value", Line: 7, Column: 9}
	if id := analyzer.placeRootID(first); id != 1 {
		t.Fatalf("first identity = %d, want 1", id)
	}
	if id := analyzer.placeRootID(sibling); id != 2 {
		t.Fatalf("second identity = %d, want 2", id)
	}
	if id := analyzer.placeRootID(first); id != 1 {
		t.Fatalf("repeated identity = %d, want the stable 1", id)
	}
	if id := analyzer.placeRootID(lexer.Token{}); id != 0 {
		t.Fatalf("a declaration without a source location gets identity %d, want none", id)
	}

	left := Place{Root: "value", RootID: 1}
	right := Place{Root: "value", RootID: 2}
	if got := Relationship(left, right); got != PlaceDisjoint {
		t.Fatalf("same spelling, different declarations = %s, want disjoint", got)
	}
	if got := Relationship(left, Place{Root: "value", RootID: 1, Projections: []PlaceProjection{relationshipField("X")}}); got != PlaceContains {
		t.Fatalf("same declaration = %s, want contains", got)
	}
	if got := Relationship(left, Place{Root: "value"}); got != PlaceSame {
		t.Fatalf("missing identity falls back to the name, got %s", got)
	}
}

// The root kind classifies the binding role of the root (revised § 10).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §10 (revised 2026-10-03)
func TestPlaceRootKindClassifiesBindingRole(t *testing.T) {
	analyzer := NewAnalyzer()
	token := func(line int) lexer.Token { return lexer.Token{Line: line, Column: 1} }
	analyzer.symbols = map[string]Symbol{
		"count":   {Name: "count", Token: token(1), Local: true, Parameter: true, Storage: StorageOriginAutomatic},
		"total":   {Name: "total", Token: token(2), Local: true, Storage: StorageOriginAutomatic},
		"self":    {Name: "self", Token: token(3), Local: true, Parameter: true},
		"Size":    {Name: "Size", Token: token(4), ImplicitMember: true},
		"Limit":   {Name: "Limit", Token: token(5), Storage: StorageOriginStatic},
		"counter": {Name: "counter", Token: token(6), Local: true, Storage: StorageOriginThreadLocal},
	}
	for name, want := range map[string]PlaceRootKind{
		"count": PlaceRootParameter, "total": PlaceRootLocal, "self": PlaceRootReceiver,
		"Size": PlaceRootReceiver, "Limit": PlaceRootStatic, "counter": PlaceRootStatic,
	} {
		place, ok := analyzer.rootPlace(name)
		if !ok || place.RootKind != want || place.RootID == 0 {
			t.Errorf("%s: kind %s, identity %d, want %s with an identity", name, place.RootKind, place.RootID, want)
		}
	}
}

// Correctness decisions consult the canonical Relationship query; the legacy
// boolean PlacesOverlap stays a compatibility adapter that only its own
// definition and tests mention.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §16 "Legacy PlacesOverlap"
func TestLegacyPlacesOverlapHasNoCorrectnessCallers(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for number, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "PlacesOverlap(") && !strings.HasPrefix(line, "func PlacesOverlap(") {
				t.Errorf("%s:%d calls the legacy PlacesOverlap; use Relationship or placesMayOverlap", file, number+1)
			}
		}
	}
}

// Interprocedural parameter-usage summaries build their Places with the same
// compiler-owned root identity Sema assigns to the parameter declaration, and
// a generic body keeps one root identity per declaration across every
// instantiation (P15-10).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §10 "Stable place root identity"
//   - rules/compiler/semantic_ir.md — § 10(7)
func TestPlaceRootIdentityCoversSummariesAndGenericInstantiation(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

fn Touch(values: ref mut int[4], other: ref mut int[4]) void {
    values[1] = 2
    other[1] = 3
}

fn Twice[T](value: T) T {
    let copy :<- value
    return copy
}

fn Use() int {
    discard Twice[bool](true)
    return Twice[int](4)
}
`)
	assertSemaErrors(t, errors, nil)
	summary := parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "Touch")
	ids := map[PlaceRootID]string{}
	for _, parameter := range summary.Parameters {
		if len(parameter.Uses) == 0 {
			t.Fatalf("%s has no recorded uses", parameter.Name)
		}
		for _, use := range parameter.Uses {
			place := use.Place
			if place.RootID == 0 || place.RootKind != PlaceRootParameter || place.RootToken != parameter.Declaration {
				t.Fatalf("%s use Place root = %d/%s/%v, want the parameter declaration identity", parameter.Name, place.RootID, place.RootKind, place.RootToken)
			}
			if place.RootID != analyzer.placeRootID(parameter.Declaration) {
				t.Fatalf("%s summary identity %d differs from Sema's %d", parameter.Name, place.RootID, analyzer.placeRootID(parameter.Declaration))
			}
			if previous, seen := ids[place.RootID]; seen && previous != parameter.Name {
				t.Fatalf("parameters %s and %s share root identity %d", previous, parameter.Name, place.RootID)
			}
			ids[place.RootID] = parameter.Name
		}
	}
	copies := 0
	for key := range analyzer.placeRootIDs {
		if key.Line == 9 && key.Column == 9 {
			copies++
		}
	}
	if copies != 1 {
		t.Fatalf("generic local has %d root identities across instantiations, want 1", copies)
	}
}
