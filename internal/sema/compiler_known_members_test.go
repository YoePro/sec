package sema

import (
	"strings"
	"testing"
)

func TestCompilerKnownFundamentalMembers(t *testing.T) {
	input := `
module main

fn Test(text: string, runes: rune[2], ptr: RawPtr[int]) void {
	let canonicalLength: uint := text.Len
	let migrationLength: uint := text.len
	let valueSize: uint := text.SizeOf
	let typeSize: uint := int32.SizeOf
	let globalTypeSize: uint := SizeOf(int32)
	let minimum: int32 := int32.Min
	let maximum: int32 := int32.Max
	let bits: uint := int32.Bits
	let decimalScale: int := decimal.Scale
	let formatted: string := true.ToString()
	let bytes: byte[] := text.ToByteArray()
	let chars: char[] := text.ToCharArray()
	let decoded: rune[] := text.ToRuneArray()
	let joined: string := runes.ToString()
	let fromRunes: string := string.FromRuneArray(runes)
	let fromBytes: string := string.FromByteArray(bytes)
	unsafe {
		let address: RawPtr[byte] := text.Ptr
		let value: int := ptr.Read()
		ptr.Write(value)
	}
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)
}

func TestCompilerKnownCollectionPropertiesAndMethods(t *testing.T) {
	input := `
module main

fn Test(values: int[], view: ref mut int[], users: list[int], entries: map[int, string], unique: set[int]) Result[void, CollectionError] {
	let arrayLength: uint := values.Len
	let empty: bool := values.IsEmpty
	let bytes: uint := view.SizeOf
	let listLength: uint := users.Len
	let mapLength: uint := entries.Len
	let setLength: uint := unique.Len
	let signedListLength: int := len(users)
	let signedMapLength: int := len(entries)
	let signedSetLength: int := len(unique)
	view.Fill(3)
	view.Reverse()
	try values.Append(4)
	let removed: Option[int] := values.RemoveAt(0)
	values.Clear()
	unsafe {
		let data: RawPtr[int] := values.Ptr
	}
	return Ok()
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)
}

func TestCompilerKnownContextualFill(t *testing.T) {
	input := `
module main

fn Test() Result[void, CollectionError] {
	let fixed: int[3] := fill(7)
	let owned: int[] := try fill(7, 3)
	let text: string := try fill("=", 4)
	discard fixed
	discard owned
	discard text
	return Ok()
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)
}

func TestCompilerKnownLibraryCollectionMethods(t *testing.T) {
	input := `
module main

fn Compare(left: int, right: int) int {
	return left - right
}

fn Test(users: list[int], entries: map[int, string], unique: set[int], other: set[int]) Result[void, CollectionError] {
	let capacity: uint := users.Capacity
	try users.Append(1)
	let inserted: bool := try users.Insert(0, 2)
	let removed: Option[int] := users.RemoveAt(0)
	let removedValue: bool := users.Remove(2)
	let contains: bool := users.Contains(3)
	let index: Option[uint] := users.IndexOf(3)
	users.Reverse()
	users.Sort()
	users.SortBy(Compare)

	let entry: Option[string] := entries.Remove(1)
	let hasKey: bool := entries.ContainsKey(1)
	entries.Clear()

	let added: bool := try unique.Add(1)
	let setRemoved: bool := unique.Remove(1)
	let setContains: bool := unique.Contains(1)
	let combined: set[int] := try unique.Union(other)
	let common: set[int] := try unique.Intersection(other)
	let difference: set[int] := try unique.Difference(other)
	let symmetric: set[int] := try unique.SymmetricDifference(other)
	unique.Clear()

	discard capacity
	discard inserted
	discard removed
	discard removedValue
	discard contains
	discard index
	discard entry
	discard hasKey
	discard added
	discard setRemoved
	discard setContains
	discard combined
	discard common
	discard difference
	discard symmetric
	return Ok()
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)
}

func TestCompilerKnownArenaMembers(t *testing.T) {
	input := `
module main

fn Test(buffer: ref mut byte[]) void {
	let mut borrowed := Arena.FromBuffer(buffer)
	let one := borrowed.New[int]()
	let many := borrowed.Alloc[int](2u)
	borrowed.Reset()
	borrowed.Release()

	let fixed := Arena.WithCapacity(1024u)
	let growable := Arena.Growable(1024u)
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)
}

func TestCompilerKnownMemberValidation(t *testing.T) {
	input := `
module main

fn Test(text: string, ptr: RawPtr[int], shared: ref byte[], number: int) void {
	let badLength := number.Len
	let badConversion := text.ToRuneArray(1)
	let unsafeRead := ptr.Read()
	let badArena := Arena.FromBuffer(shared)
}
`
	errors := analyzeSourceRaw(t, input)
	if len(errors) != 4 {
		t.Fatalf("wrong sema error count. got=%d want=4 errors=%v", len(errors), errors)
	}
}

func TestCompilerKnownCanonicalPropertyAndCollectionRestrictions(t *testing.T) {
	input := `
module main

fn Test(text: string, entries: map[int, string]) void {
	let calledValueSize := text.SizeOf()
	let calledTypeSize := int32.SizeOf()
	let contextless := fill(1)
	unsafe {
		let mapPointer := entries.Ptr
	}
}
`
	errors := analyzeSourceRaw(t, input)
	for _, fragment := range []string{
		"unknown function or type text.SizeOf",
		"unknown function or type int32.SizeOf",
		"fill requires an explicit array or string target type",
		"unknown member Ptr on map[int, string]",
	} {
		found := false
		for _, err := range errors {
			if strings.Contains(err.Message, fragment) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing error containing %q; errors=%v", fragment, errors)
		}
	}
}

func TestCompilerKnownRegistryHasStableRequiredIDs(t *testing.T) {
	stringMembers := CompilerKnownMembersForType(builtinTypes()["string"], false)
	want := map[string]bool{
		"CKM-PTR-VALUE":          false,
		"CKM-LEN-STRING":         false,
		"CKM-SIZEOF-VALUE":       false,
		"CKM-TOSTRING-STRING":    false,
		"CKM-STRING-TOBYTEARRAY": false,
		"CKM-STRING-TOCHARARRAY": false,
		"CKM-STRING-TORUNEARRAY": false,
	}
	for _, member := range stringMembers {
		if _, exists := want[member.ID]; exists {
			want[member.ID] = true
		}
	}
	for id, found := range want {
		if !found {
			t.Fatalf("compiler-known registry is missing %s", id)
		}
	}
}

func TestCompilerKnownGlobalRegistryHasStableRequiredIDs(t *testing.T) {
	want := map[string]string{
		"len":    "CKF-LEN",
		"SizeOf": "CKF-SIZEOF-TYPE",
		"fill":   "CKF-FILL",
	}
	for _, function := range CompilerKnownFunctions() {
		if id, ok := want[function.Name]; ok {
			if function.ID != id {
				t.Fatalf("compiler-known function %s ID = %q, want %q", function.Name, function.ID, id)
			}
			delete(want, function.Name)
		}
	}
	if len(want) != 0 {
		t.Fatalf("compiler-known global registry is missing %v", want)
	}
}

func TestCompilerKnownGlobalFunctionsCannotBeRedeclared(t *testing.T) {
	input := `
module main

fn SizeOf(value: int) uint {
	return 0u
}

fn fill(value: int) int {
	return value
}
`
	errors := analyzeSourceRaw(t, input)
	for _, name := range []string{"SizeOf", "fill"} {
		found := false
		for _, err := range errors {
			if strings.Contains(err.Message, "function "+name+" is compiler-known and cannot be declared") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing redeclaration error for %s; errors=%v", name, errors)
		}
	}
}

func TestCompilerKnownRegistryCoversCanonicalCollectionSurface(t *testing.T) {
	intType := builtinTypes()["int"]
	dynamic := compilerKnownDynamicArray(intType)
	dynamicWant := map[string]bool{
		"CKM-LEN-ARRAY":              false,
		"CKM-ISEMPTY-ARRAY":          false,
		"CKM-PTR-VALUE":              false,
		"CKM-SIZEOF-VALUE":           false,
		"CKM-DYNAMIC-ARRAY-APPEND":   false,
		"CKM-DYNAMIC-ARRAY-CLEAR":    false,
		"CKM-DYNAMIC-ARRAY-REMOVEAT": false,
	}
	for _, member := range CompilerKnownMembersForType(dynamic, false) {
		if _, ok := dynamicWant[member.ID]; ok {
			dynamicWant[member.ID] = true
		}
		if member.Name == "SizeOf" && member.Kind != CompilerKnownProperty {
			t.Fatalf("value SizeOf kind = %q, want property", member.Kind)
		}
	}
	for id, found := range dynamicWant {
		if !found {
			t.Fatalf("dynamic-array registry is missing %s", id)
		}
	}

	mapType := builtinTypes()["map"]
	mapType.TypeArgs = []Type{intType, builtinTypes()["string"]}
	for _, member := range CompilerKnownMembersForType(mapType, false) {
		if member.Name == "Ptr" {
			t.Fatal("map must not expose compiler-known Ptr")
		}
	}

	for _, member := range CompilerKnownMembersForType(builtinTypes()["int32"], true) {
		if member.Name == "SizeOf" && member.Kind != CompilerKnownProperty {
			t.Fatalf("type SizeOf kind = %q, want property", member.Kind)
		}
	}
}
