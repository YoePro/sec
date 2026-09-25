package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func TestCompilerKnownFundamentalMembers(t *testing.T) {
	input := `
module main

fn Test(text: string, runes: rune[2], ptr: RawPtr[int]) void {
	let canonicalLength: uint := text.Len
	let migrationLength: uint := text.len
	let valueSize: uint := text.SizeOf
	let typeSize: uint := int32.SizeOf
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

// TestCompilerKnownUniversalToStringAndTextSequences verifies both layers of
// the canonical fallback: every concrete value has ToString, while byte,
// char, and rune sequences retain distinct materialization identities.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "ToString()"
//   - rules/compiler/compiler_known_members.md — "Byte, char, and rune sequence ToString()"
//   - rules/collections/collections.md — §4 "ToString"
func TestCompilerKnownUniversalToStringAndTextSequences(t *testing.T) {
	input := `
module main

type Packet struct { value: int }
type Plain struct { value: int }
type Formatted struct { value: int }

impl Packet {
	fn ToString() string {
		return self.value.ToString()
	}
}

impl Formatted {
	fn ToString(format: string) string {
		return format
	}
}

fn Bytes(high: byte, low: byte) string {
	let bytes: byte[] := [high, low]
	return bytes.ToString()
}

fn Chars(chars: ref char[]) string {
	return chars.ToString()
}

fn Runes(runes: rune[]) string {
	return runes.ToString()
}

fn TypeFallback(values: int[]) string {
	return values.ToString()
}

fn ObjectFallback(value: Plain) string {
	return value.ToString()
}

fn OverloadDoesNotReplaceFallback(value: Formatted) string {
	return value.ToString()
}

fn ExplicitOverloadRemainsCallable(value: Formatted) string {
	return value.ToString("custom")
}

fn UserReplacement(packet: Packet) string {
	return packet.ToString()
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)

	sequenceIDs := []struct {
		element Type
		want    string
	}{
		{element: builtinTypes()["byte"], want: "CKM-TOSTRING-BYTE-SEQUENCE"},
		{element: builtinTypes()["char"], want: "CKM-TOSTRING-CHAR-SEQUENCE"},
		{element: builtinTypes()["rune"], want: "CKM-TOSTRING-RUNE-SEQUENCE"},
		{element: builtinTypes()["int"], want: "CKM-TOSTRING-VALUE"},
	}
	for _, test := range sequenceIDs {
		typ := NewDynamicArrayType(test.element)
		member, ok := compilerKnownMember(typ, "ToString", false)
		if !ok || member.Kind != CompilerKnownMethod || member.Result.Kind != StringType || member.ID != test.want {
			t.Fatalf("%s[].ToString = %+v, %v; want %s returning string", test.element.Name, member, ok, test.want)
		}
	}
}

// A trusted core primitive may add an overload without losing the universal
// parameterless fallback. The overload's enum argument must go through normal
// method resolution rather than the compiler-known numeric string-format path.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "Lookup order"
//   - rules/compiler/compiler_known_members.md — "User-defined ToString()"
func TestCompilerKnownPrimitiveToStringOverloadUsesOrdinaryMethodResolution(t *testing.T) {
	const sourceFile = "sec/core/byte.sec"
	input := `module core

impl byte {
	enum ByteStringFormat {
		Decimal,
		Hexadecimal,
	}

	fn ToString(format: byte.ByteStringFormat) string {
		return "formatted"
	}
}

fn Format(value: byte) string {
	return value.ToString(byte.ByteStringFormat.Hexadecimal)
}

fn Default(value: byte) string {
	return value.ToString()
}
`
	parsed := parser.New(lexer.NewWithFile(input, sourceFile)).Parse()
	if parsed.HasErrors {
		t.Fatalf("parser errors: %+v", parsed.Diagnostics)
	}
	parsed.Program.SourceProvenance = map[string]ast.SourceProvenance{sourceFile: ast.SourceCore}
	assertSemaErrors(t, NewAnalyzer().Analyze(parsed.Program), nil)
}

func TestCompilerKnownRawPointerVolatileAccess(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Access(pointer: RawPtr[int], value: int) int {
	unsafe {
		pointer.VolatileWrite(value)
		return pointer.VolatileRead()
	}
}
`)
	assertSemaErrors(t, errors, nil)

	wantRegistry := map[string]EffectKind{
		"CKM-RAWPTR-VOLATILE-READ":  EffectVolatileRead,
		"CKM-RAWPTR-VOLATILE-WRITE": EffectVolatileWrite,
	}
	rawPointer := Type{Kind: RawPtrType, Name: "RawPtr", TypeArgs: []Type{builtinTypes()["int"]}}
	for _, member := range CompilerKnownMembersForType(rawPointer, false) {
		effect, wanted := wantRegistry[member.ID]
		if !wanted {
			continue
		}
		if !member.Unsafe || len(member.Effects) != 1 || member.Effects[0] != effect {
			t.Fatalf("volatile registry member %s = %+v", member.ID, member)
		}
		delete(wantRegistry, member.ID)
	}
	if len(wantRegistry) != 0 {
		t.Fatalf("volatile registry entries missing: %v", wantRegistry)
	}

	id := callGraphNodeIDByName(t, analyzer.CallGraph(), "Access")
	effects := analyzer.CallGraph().EffectSummary(id).DirectEffects
	if len(effects) != 2 || effects[0].Kind != EffectVolatileWrite || effects[1].Kind != EffectVolatileRead {
		t.Fatalf("volatile effects = %+v", effects)
	}
}

func TestCompilerKnownRawPointerVolatileAccessRejectsUnsafeVoidAndWrongValue(t *testing.T) {
	input := `
module main

fn Test(pointer: RawPtr[int], opaque: RawPtr[void]) void {
	let outside := pointer.VolatileRead()
	unsafe {
		let noValue := opaque.VolatileRead()
		opaque.VolatileWrite(1)
		pointer.VolatileWrite("wrong")
	}
}
`
	errors := analyzeSourceRaw(t, input)
	for _, fragment := range []string{
		"RawPtr.VolatileRead requires unsafe",
		"RawPtr[void].VolatileRead cannot materialize a value",
		"RawPtr[void].VolatileWrite cannot consume a value",
		"RawPtr.VolatileWrite value must be int, got string",
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

// TestCompilerKnownRequestCancel verifies the symmetric, non-consuming
// cancellation request surface on owning task and thread handles.
//
// Rules:
//   - rules/concurrency/cancellation.md — § 5 "Symmetric task/thread cancellation surface"
//   - rules/concurrency/cancellation.md — § 6(2)–(5) "Relevant Task[T] cancellation surface"
//   - rules/concurrency/cancellation.md — § 7(2)–(4) "Relevant Thread[T] cancellation surface"
func TestCompilerKnownRequestCancel(t *testing.T) {
	input := `
module main

fn Request(taskHandle: Task[int], threadHandle: Thread[int]) void {
	taskHandle.RequestCancel()
	threadHandle.RequestCancel()
	detach taskHandle discard
	detach threadHandle discard
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)

	for _, typeName := range []string{"Task", "Thread"} {
		typ := Type{Name: typeName, Kind: StructType, TypeArgs: []Type{builtinTypes()["int"]}}
		member, ok := compilerKnownMember(typ, "RequestCancel", false)
		if !ok {
			t.Fatalf("%s[int].RequestCancel is not registered", typeName)
		}
		if member.Kind != CompilerKnownMethod || member.Result.Kind != VoidType || member.Signature != "fn RequestCancel() void" {
			t.Fatalf("%s[int].RequestCancel = %+v", typeName, member)
		}
	}
}

// TestCompilerKnownRequestCancelRejectsArguments keeps RequestCancel's exact
// zero-argument source signature synchronized for both handle families.
//
// Rules:
//   - rules/concurrency/cancellation.md — § 6(2) "Relevant Task[T] cancellation surface"
//   - rules/concurrency/cancellation.md — § 7(2) "Relevant Thread[T] cancellation surface"
func TestCompilerKnownRequestCancelRejectsArguments(t *testing.T) {
	input := `
module main

fn Invalid(taskHandle: ref Task[int], threadHandle: ref Thread[int]) void {
	taskHandle.RequestCancel(true)
	threadHandle.RequestCancel(false)
}
`
	errors := analyzeSourceRaw(t, input)
	for _, fragment := range []string{
		"Task[int].RequestCancel expects 0 arguments, got 1",
		"Thread[int].RequestCancel expects 0 arguments, got 1",
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

func TestCompilerKnownCollectionPropertiesAndMethods(t *testing.T) {
	input := `
module main

fn Test(values: int[], view: ref mut int[], users: list[int], entries: map[int, string], members: set[int]) Result[void, CollectionError] {
	let arrayLength: uint := values.Len
	let empty: bool := values.IsEmpty
	let bytes: uint := view.SizeOf
	let listLength: uint := users.Len
	let mapLength: uint := entries.Len
	let setLength: uint := members.Len
	let signedListLength: int := len(users)
	let signedMapLength: int := len(entries)
	let signedSetLength: int := len(members)
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

// TestCompilerKnownStaticShapedFacts verifies that statically determined Rank
// and Len facts are properties with uint results for shaped values.
//
// Rules:
//   - rules/collections/shaped-types.md — § 3.1–3.5 "Shaped type families"
//   - rules/collections/shaped-types.md — § 5 "Rank, Shape, and Len"
//   - rules/corrections/applied/compiler_known_members-shaped-correction-20260813.md — "Required read-only shaped properties"
func TestCompilerKnownStaticShapedFacts(t *testing.T) {
	input := `
module main

fn Inspect(v: vector[int, 4], m: matrix[int, 3, 4], t: tensor[int, 2, 3, 4], view: ref tensor_view[int, 3]) void {
	let vectorRank: uint := v.Rank
	let vectorLen: uint := v.Len
	let matrixRank: uint := m.Rank
	let matrixLen: uint := m.Len
	let tensorRank: uint := t.Rank
	let tensorLen: uint := t.Len
	let viewRank: uint := view.Rank
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)

	matrix := builtinTypes()["matrix"]
	matrix.TypeArgs = []Type{builtinTypes()["int"]}
	matrix.ConstArgs = []int64{3, 4}
	for _, static := range []bool{false, true} {
		rank, ok := compilerKnownMember(matrix, "Rank", static)
		if !ok || rank.Kind != CompilerKnownProperty || rank.Result.Kind != UintType || !strings.Contains(rank.Documentation, "2") {
			t.Fatalf("matrix Rank (static=%v) = %+v, %v", static, rank, ok)
		}
		length, ok := compilerKnownMember(matrix, "Len", static)
		if !ok || length.Kind != CompilerKnownProperty || length.Result.Kind != UintType || !strings.Contains(length.Documentation, "12") {
			t.Fatalf("matrix Len (static=%v) = %+v, %v", static, length, ok)
		}
	}

	view := builtinTypes()["tensor_view"]
	view.TypeArgs = []Type{builtinTypes()["int"]}
	view.ConstArgs = []int64{3}
	if _, ok := compilerKnownMember(view, "Rank", true); !ok {
		t.Fatal("tensor_view type must expose its statically known Rank")
	}
	if _, ok := compilerKnownMember(view, "Len", true); ok {
		t.Fatal("tensor_view type must not synthesize a runtime Len")
	}
}

func TestCompilerKnownAppendAcceptsTypedStructLiteralUnderTry(t *testing.T) {
	input := `
module main

type Message struct {
	Message: string,
	ID: string,
	OtherData: string,
}

fn Add(messages: Message[]) Result[void, CollectionError] {
	try messages.Append(Message {
		Message: "abc",
		ID: "P1230",
		OtherData: "asdgh",
	})
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

fn Test(users: list[int], entries: map[int, string], members: set[int], other: set[int]) Result[void, CollectionError] {
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

	let added: bool := try members.Add(1)
	let setRemoved: bool := members.Remove(1)
	let setContains: bool := members.Contains(1)
	let combined: set[int] := try members.Union(other)
	let common: set[int] := try members.Intersection(other)
	let difference: set[int] := try members.Difference(other)
	let symmetric: set[int] := try members.SymmetricDifference(other)
	members.Clear()

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

// rules/compiler/compiler_known_members.md "Named and related types" allows a
// named string representation to use eligible privileged-core string members
// while retaining its nominal type.
func TestNamedStringInheritsPrivilegedCoreMembers(t *testing.T) {
	const sourceFile = "/tmp/sec-test/sec/core/string.sec"
	input := `module main

impl string {
    property ByteLen: uint {
        get {
            return self.Len
        }
    }

    fn IndexOf(value: string) Option[uint] {
        return None
    }
}

type Priority uint8
type HeaderValue string
type Override string

impl HeaderValue {
    fn IsValid() bool {
        return self.ByteLen != 0u
    }
}

impl Override {
    property ByteLen: bool {
        get {
            return true
        }
    }
}

enum StructuredFieldError error {
    AllocationFailed
}

fn Parse(value: HeaderValue) Result[Priority, StructuredFieldError] {
    let bytes: uint := value.ByteLen
    let n: Option[uint] := value.IndexOf("=")
    if n is None {
        return Err(StructuredFieldError.AllocationFailed)
    }
    return Err(StructuredFieldError.AllocationFailed)
}

fn CheckExactProperty(value: Override) bool {
    return value.ByteLen
}
`
	l := lexer.NewWithFile(input, sourceFile)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	program.SourceProvenance = map[string]ast.SourceProvenance{sourceFile: ast.SourceCore}
	assertSemaErrors(t, NewAnalyzer().Analyze(program), nil)
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
	let removedGlobalSize := SizeOf(int32)
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
		"unknown function or type SizeOf",
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
		"len":                    "CKF-LEN",
		"fill":                   "CKF-FILL",
		"__StringSliceUnchecked": "CKF-STRING-SLICE-UNCHECKED",
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
	if _, exists := compilerKnownFunction("SizeOf"); exists {
		t.Fatal("removed global SizeOf must not remain in the compiler-known function registry")
	}
}

func TestCompilerKnownStringSliceIsInternalAndTyped(t *testing.T) {
	known, ok := compilerKnownFunction("__StringSliceUnchecked")
	if !ok {
		t.Fatal("missing compiler-known string slice operation")
	}
	if !known.Internal {
		t.Fatal("compiler-known string slice operation must be internal")
	}
	if known.OwnerFile != "sec/core/string.sec" {
		t.Fatalf("compiler-known string slice owner = %q", known.OwnerFile)
	}
	if known.Result.Kind != StringType || len(known.Parameters) != 3 {
		t.Fatalf("unexpected string slice signature: %#v", known)
	}
	want := []TypeKind{StringType, UintType, UintType}
	for index, parameter := range known.Parameters {
		if parameter.Type.Kind != want[index] {
			t.Fatalf("parameter %d has kind %s, want %s", index, parameter.Type.Kind, want[index])
		}
	}
}

// TestRemovedGlobalSizeOfMayBeDeclaredAsOrdinaryFunction verifies that only
// the instance and associated SizeOf properties retain compiler authority.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "SizeOf"
//   - rules/corrections/applied/compiler-known-fundamentals-cross-rulebook-correction-20260907.md — §§ 12 and 22.3
func TestRemovedGlobalSizeOfMayBeDeclaredAsOrdinaryFunction(t *testing.T) {
	input := `
module main

fn SizeOf(value: int) uint {
	return 0u
}

fn UseUserSizeOf() uint {
	return SizeOf(1)
}

fn fill(value: int) int {
	return value
}
`
	errors := analyzeSourceRaw(t, input)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "function fill is compiler-known and cannot be declared") {
		t.Fatalf("errors = %v, want only compiler-known fill redeclaration", errors)
	}
	for _, err := range errors {
		if strings.Contains(err.Message, "function SizeOf is compiler-known") {
			t.Fatalf("removed global SizeOf remains reserved: %v", errors)
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
