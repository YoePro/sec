package sema

import (
	"strings"
	"testing"
)

// Derived signatures retain nominal and nested receiver types; documentation
// and synthetic navigation consume exactly the same registry rendering.
// Rules: rules/compiler/compiler_known_members.md — "Named and related types",
// "Synthetic definitions", "Documentation generation".
func TestCompilerKnownReceiverInstantiatedPresentation(t *testing.T) {
	intType, stringType := builtinTypes()["int"], builtinTypes()["string"]
	customer := Type{Name: "Customer", Kind: StructType, Named: true}
	list := builtinTypes()["list"]
	list.TypeArgs = []Type{customer}
	mapping := builtinTypes()["map"]
	mapping.TypeArgs = []Type{stringType, list}
	set := builtinTypes()["set"]
	set.TypeArgs = []Type{customer}
	pointer := builtinTypes()["RawPtr"]
	pointer.TypeArgs = []Type{customer}
	slice := compilerKnownSharedReference(Type{Kind: SliceType, Element: &customer})
	slice.ReferenceMutable = true
	for _, test := range []struct {
		typ             Type
		name, signature string
		static          bool
	}{
		{list, "Append", "fn Append(value: Customer) Result[void, CollectionError]", false},
		{list, "RemoveAt", "fn RemoveAt(index: uint) Option[Customer]", false},
		{list, "WithCapacity", "static fn WithCapacity(capacity: uint) Result[list[Customer], CollectionError]", true},
		{mapping, "Remove", "fn Remove(key: string) Option[list[Customer]]", false},
		{mapping, "WithCapacity", "static fn WithCapacity(capacity: uint) Result[map[string, list[Customer]], CollectionError]", true},
		{set, "Add", "fn Add(value: Customer) Result[bool, CollectionError]", false},
		{NewDynamicArrayType(customer), "Append", "fn Append(value: Customer) Result[void, CollectionError]", false},
		{slice, "Fill", "fn Fill(value: Customer) void", false},
		{pointer, "Read", "unsafe fn Read() Customer", false},
		{stringType, "Ptr", "unsafe property Ptr: RawPtr[byte]", false},
		{compilerKnownResult(intType, builtinTypes()["CollectionError"]), "Err", "fn Err() Option[CollectionError]", false},
		{builtinTypes()["int32"], "Max", "static property Max: int32", true},
		{builtinTypes()["Arena"], "New", "fn New[T]() Result[ref mut T, AllocationError]", false},
	} {
		member, ok := compilerKnownMember(test.typ, test.name, test.static)
		if !ok || member.Signature != test.signature || member.BoundReceiver == "" {
			t.Fatalf("%s.%s: member = %+v, want %q", typeDisplayName(test.typ), test.name, member, test.signature)
		}
		document := CompilerKnownDocumentationForType(test.typ, test.static)
		if !strings.Contains(document, CompilerKnownSyntheticDefinition(member)) {
			t.Fatalf("documentation differs from synthetic definition for %s", member.ID)
		}
	}
}
