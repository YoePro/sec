package sema

import (
	"os"
	"strings"
	"testing"
)

// Canonical collections retain their element/index and key/value binding rules.
// Rules: rules/control-flow/flowcontrol_for.md — §§13–21.
func TestForCompilerKnownCollectionLoopBindings(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/for_canonical_collections_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

// Invalid binding counts retain canonical set/map/sequential diagnostics.
// Rules: rules/control-flow/flowcontrol_for.md — §§14,20–21.
func TestForCompilerKnownCollectionLoopBindingErrors(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/for_canonical_collection_bindings_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	expected := []string{
		"set iteration supports one loop binding, got 2 at 5:6",
		"map iteration requires key and value bindings, got 1 at 10:6",
		"map iteration requires key and value bindings, got 3 at 15:6",
		"sequential iteration supports one or two loop bindings, got 3 at 20:6",
	}
	assertSemaErrors(t, errors, expected)
}

// Historical collection aliases and Next/Len-shaped user types cannot establish
// iteration. The frontend rejects them before publishing an iterator plan.
// Rules: rules/control-flow/flowcontrol_for.md — §13 and §37.
func TestForRejectsLegacyCollectionAndMethodNameDiscovery(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/for_legacy_collections_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	if len(errors) != 7 {
		t.Fatalf("got %d diagnostics, want 7: %v", len(errors), errors)
	}
	for _, diagnostic := range errors {
		if !strings.Contains(diagnostic.Message, "is not iterable") {
			t.Fatalf("unexpected diagnostic: %v", diagnostic)
		}
	}
	if len(analyzer.resolvedForIterations) != 0 {
		t.Fatalf("rejected loops published iterator plans: %#v", analyzer.resolvedForIterations)
	}
}

// Compiler identity and resolved arity are required in addition to spelling,
// including for the privileged Iterator interface itself.
// Rules: rules/control-flow/flowcontrol_for.md — §13 and §37.
func TestForCategoriesRequireCanonicalCompilerIdentity(t *testing.T) {
	integer := Type{Name: "int", Kind: IntType}
	for _, name := range []string{"list", "set", "map", "vector"} {
		canonical := Type{Name: name, Kind: StructType, Intrinsic: true, TypeArgs: []Type{integer}}
		if name == "map" {
			canonical.TypeArgs = append(canonical.TypeArgs, integer)
		}
		if name == "vector" {
			canonical.ConstArgs = []int64{3}
		}
		if !isForCollectionFamily(canonical) {
			t.Errorf("canonical %s rejected", name)
		}
		impostor := canonical
		impostor.Intrinsic = false
		if isForCollectionFamily(impostor) {
			t.Errorf("ordinary type named %s accepted", name)
		}
		wrongKind := canonical
		wrongKind.Kind = InterfaceType
		if isForCollectionFamily(wrongKind) {
			t.Errorf("wrong kind named %s accepted", name)
		}
		wrongArity := canonical
		wrongArity.TypeArgs = nil
		if isForCollectionFamily(wrongArity) {
			t.Errorf("wrong arity named %s accepted", name)
		}
	}
	for _, name := range []string{"Vec", "Set", "Map"} {
		if isForCollectionFamily(Type{Name: name, Kind: StructType, Intrinsic: true, TypeArgs: []Type{integer, integer}}) {
			t.Errorf("legacy category %s accepted", name)
		}
	}
	analyzer := NewAnalyzer()
	conformance := Type{Name: "Iterator", Kind: InterfaceType, Intrinsic: true, TypeArgs: []Type{integer}}
	source := Type{Name: "Counter", Kind: StructType, Implements: []Type{conformance}}
	if _, _, _, ok := analyzer.compilerKnownIterator(source); !ok {
		t.Fatal("canonical Iterator conformance rejected")
	}
	source.Implements[0].Intrinsic = false
	if _, _, _, ok := analyzer.compilerKnownIterator(source); ok {
		t.Fatal("ordinary interface named Iterator granted iteration")
	}
}
