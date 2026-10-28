package sema

import (
	"reflect"
	"strings"
	"testing"
)

// Display facts consume receiver-resolved members from the actual registry,
// including nominal/nested receivers and method-local generic parameters.
// Rules: rules/compiler/compiler_known_members.md — "LSP", "Named and related types".
func TestCompilerKnownMemberDisplayFacts(t *testing.T) {
	types := builtinTypes()
	customer := Type{Name: "Customer", Kind: StructType, Named: true}
	list := types["list"]
	list.TypeArgs = []Type{customer}
	mapping := types["map"]
	mapping.TypeArgs = []Type{types["string"], list}

	pointer := types["RawPtr"]
	pointer.TypeArgs = []Type{customer}
	base := types["string"]
	slug := base
	slug.Name = "Slug"
	slug.Named = true
	slug.NamedBase = &base
	for _, tc := range []struct {
		receiver               Type
		name, label, signature string
		static                 bool
	}{
		{list, "Append", "list[Customer].Append", "fn Append(value: Customer) Result[void, CollectionError]", false},
		{mapping, "WithCapacity", "map[string, list[Customer]].WithCapacity", "static fn WithCapacity(capacity: uint) Result[map[string, list[Customer]], CollectionError]", true},
		{types["string"], "Len", "string.Len", "property Len: uint", false},
		{types["Arena"], "New", "Arena.New", "fn New[T]() Result[ref mut T, AllocationError]", false},
		{pointer, "Read", "RawPtr[Customer].Read", "unsafe fn Read() Customer", false},
		{slug, "Len", "Slug.Len", "property Len: uint", false},
	} {
		member, ok := compilerKnownMember(tc.receiver, tc.name, tc.static)
		if !ok {
			t.Fatal(tc.name)
		}
		before := member
		facts := member.DisplayFacts()
		if facts.Label != tc.label || facts.Signature != tc.signature || facts.Name != tc.name || facts.Receiver != typeDisplayName(tc.receiver) {
			t.Fatalf("%s: %+v", tc.name, facts)
		}
		if !reflect.DeepEqual(before, member) {
			t.Fatal("display query mutated semantic metadata")
		}
	}
}

// All three registry families expose source names without ID decoding; missing
// contextual signatures remain unavailable rather than inventing zero arity.
// Rules: rules/compiler/compiler_known_members.md — "Stable member identity", "LSP".
func TestCompilerKnownFunctionAndValueDisplayFacts(t *testing.T) {
	for _, function := range CompilerKnownFunctions() {
		facts := function.DisplayFacts()
		if facts.Name != function.Name || facts.Label != function.Name || facts.Kind != "function" {
			t.Fatalf("%s: %+v", function.ID, facts)
		}
		if function.Parameters == nil && facts.Signature != "" {
			t.Fatal("invented contextual signature", facts)
		}
		if function.Name == "_stringFromUtf8" && facts.Signature != "fn _stringFromUtf8(value: ref byte[]) Result[string, StringError]" {
			t.Fatalf("lost concrete signature: %+v", facts)
		}
	}
	for _, value := range CompilerKnownValues() {
		before := value
		facts := value.DisplayFacts()
		if facts.Name != value.Name || facts.Label != value.Name || facts.Kind != "value" || facts.Signature != value.Name+": "+typeDisplayName(value.Result) {
			t.Fatalf("%s: %+v", value.ID, facts)
		}
		if !reflect.DeepEqual(before, value) {
			t.Fatal("presentation changed intrinsic visibility/effects")
		}

	}
	zero := CompilerKnownFunction{Name: "Zero", Parameters: []FunctionParameter{}, Result: builtinTypes()["void"]}
	if got := zero.DisplayFacts().Signature; got != "fn Zero() void" {
		t.Fatal(got)
	}
}

// Incomplete/transport-only entries remain human-readable and never expose an
// identity as a source name. Receiver patterns are not concrete receiver facts.
// Rules: rules/compiler/compiler_known_members.md — "LSP", "Synthetic definitions".
func TestCompilerKnownDisplayFactsUnavailable(t *testing.T) {
	for _, member := range []CompilerKnownMember{
		{},
		{ID: "CKM-NOT-REGISTERED", Name: "CKM-NOT-REGISTERED", Kind: CompilerKnownMethod, Receiver: "any receiver", Signature: "method CKM-NOT-REGISTERED"},
		{Name: "Value", Kind: CompilerKnownProperty, Result: builtinTypes()["int"]},
	} {
		facts := member.DisplayFacts()
		for _, value := range []string{facts.Name, facts.Label, facts.Signature, facts.Receiver, facts.ResultType} {
			if strings.Contains(value, "CKM-") || strings.Contains(value, "CKF-") || strings.Contains(value, "CKV-") {
				t.Fatal(facts)
			}
		}
		if facts.Label == "" || facts.Receiver != "" {
			t.Fatal("unavailable receiver invented", facts)
		}
	}
	if facts := (CompilerKnownMember{}).DisplayFacts(); facts.Label != "compiler-known symbol" || facts.ResultType != "context-dependent" || facts.Signature != "" {
		t.Fatal(facts)
	}
}

// Missing names/parameter types do not authorize declarations; a known unsafe
// property keeps its unsafe qualifier even when only fallback metadata exists.
// Rules: rules/compiler/compiler_known_members.md — "LSP"; rules/memory/raw_pointers.md — §35.
func TestCompilerKnownDisplayFactFallbacksPreserveContracts(t *testing.T) {
	functions := []CompilerKnownFunction{
		{ID: "CKF-UNKNOWN", Name: "CKF-UNKNOWN"},
		{Name: "Unknown", Parameters: []FunctionParameter{{Name: "value"}}, Result: builtinTypes()["void"]},
	}
	for _, function := range functions {
		facts := function.DisplayFacts()
		if facts.Label == "" || facts.Signature != "" || strings.Contains(facts.Label, "CKF-") {
			t.Fatal(facts)
		}
	}
	if facts := (CompilerKnownValue{ID: "CKV-UNKNOWN", Name: "CKV-UNKNOWN"}).DisplayFacts(); facts.Label != "compiler-known value" || facts.Signature != "" {
		t.Fatal(facts)
	}
	unsafe := CompilerKnownMember{Name: "Pointer", Kind: CompilerKnownProperty, Result: builtinTypes()["uint"], Unsafe: true}
	if got := unsafe.DisplayFacts().Signature; got != "unsafe property Pointer: uint" {
		t.Fatal(got)
	}
}
