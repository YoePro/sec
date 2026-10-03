package sema

import (
	"fmt"
	"strings"
)

// compilerKnownSyntheticInfo is the registry metadata a synthetic read-only
// definition presents for a compiler-known member that has no ordinary core
// source declaration.
type compilerKnownSyntheticInfo struct {
	Signature     string
	Documentation string
	Rule          string
	Receiver      string
	Target        string
}

const (
	ruleCompilerKnownMembers = "rules/compiler/compiler_known_members.md"
	ruleCollections          = "rules/collections/collections.md"
	ruleRawPointers          = "rules/memory/raw_pointers.md"
	ruleArena                = "rules/memory/arena.md"
	ruleThreads              = "rules/concurrency/threads.md"
	noTargetRestriction      = "none; available on every target plan"
)

// compilerKnownSyntheticCatalog completes the registry entries by stable ID.
// Families whose IDs depend on the receiver (Len, IsEmpty, ToString) are keyed
// by their ID prefix in compilerKnownSyntheticFamilies.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "Synthetic definitions", "LSP", "Required registry inventory"
var compilerKnownSyntheticCatalog = map[string]compilerKnownSyntheticInfo{
	"CKM-PTR-VALUE":         {Signature: "unsafe property Ptr: RawPtr[T]", Documentation: "Address of the receiver's first element or byte; reading it requires unsafe and does not extend the receiver's lifetime.", Rule: ruleCompilerKnownMembers + " — `Ptr`", Receiver: "addressable scalar, string, T[N], T[], or slice", Target: noTargetRestriction},
	"CKM-SIZEOF-VALUE":      {Signature: "property SizeOf: uint", Documentation: "Byte size of the receiver's value representation; for owning collections the payload bytes, not the header.", Rule: ruleCompilerKnownMembers + " — Value-form `SizeOf`", Receiver: "any sized value", Target: "the result depends on the active target plan's layout"},
	"CKM-SIZEOF-TYPE":       {Signature: "static property SizeOf: uint", Documentation: "Byte size of the type's representation.", Rule: ruleCompilerKnownMembers + " — Type-form `SizeOf`", Receiver: "any sized type, written Type.SizeOf", Target: "the result depends on the active target plan's layout"},
	"CKM-LEN-VARIADIC-PACK": {Signature: "property Len: uint", Documentation: "Number of arguments in the variadic pack.", Rule: "rules/declarations/functions.md — section 30", Receiver: "variadic parameter pack", Target: noTargetRestriction},

	"CKM-DYNAMIC-ARRAY-APPEND":   {Signature: "fn Append(value: T) Result[void, CollectionError]", Documentation: "Appends value at the end, growing storage through the active allocation context; may relocate elements.", Rule: ruleCollections + " — 6.7 `Append`", Receiver: "owning dynamic array T[] (mutable)", Target: noTargetRestriction},
	"CKM-DYNAMIC-ARRAY-CLEAR":    {Signature: "fn Clear() void", Documentation: "Destroys every element and sets Len to 0; capacity is retained.", Rule: ruleCollections + " — 6.8 `Clear`", Receiver: "owning dynamic array T[] (mutable)", Target: noTargetRestriction},
	"CKM-DYNAMIC-ARRAY-REMOVEAT": {Signature: "fn RemoveAt(index: uint) Option[T]", Documentation: "Removes and returns the element at index, shifting later elements down; None when index is out of range.", Rule: ruleCollections + " — 6.9 `RemoveAt`", Receiver: "owning dynamic array T[] (mutable)", Target: noTargetRestriction},
	"CKM-MUTABLE-SLICE-REVERSE":  {Signature: "fn Reverse() void", Documentation: "Reverses the elements in place.", Rule: ruleCollections + " — 7.8 `Reverse`", Receiver: "mutable slice or array view", Target: noTargetRestriction},
	"CKM-MUTABLE-SLICE-FILL":     {Signature: "fn Fill(value: T) void", Documentation: "Assigns value to every element in place; T must be copyable.", Rule: ruleCollections + " — 7.9 Slice `Fill`", Receiver: "mutable slice or array view", Target: noTargetRestriction},

	"CKM-LIST-CAPACITY": {Signature: "property Capacity: uint", Documentation: "Number of elements the list can hold without growing; never the live length.", Rule: ruleCollections + " — 13.4 List properties", Receiver: "list[T]", Target: noTargetRestriction},
	"CKM-LIST-APPEND":   {Signature: "fn Append(value: T) Result[void, CollectionError]", Documentation: "Appends value at the end, growing storage through the active allocation context.", Rule: ruleCollections + " — 13.5 Core list methods", Receiver: "list[T] (mutable)", Target: noTargetRestriction},
	"CKM-LIST-INSERT":   {Signature: "fn Insert(index: uint, value: T) Result[bool, CollectionError]", Documentation: "Inserts value at index, shifting later elements up; Ok(false) when index is past the end.", Rule: ruleCollections + " — 13.10 `Insert`", Receiver: "list[T] (mutable)", Target: noTargetRestriction},
	"CKM-LIST-REMOVEAT": {Signature: "fn RemoveAt(index: uint) Option[T]", Documentation: "Removes and returns the element at index, shifting later elements down; None when index is out of range.", Rule: ruleCollections + " — 13.6 `RemoveAt`", Receiver: "list[T] (mutable)", Target: noTargetRestriction},
	"CKM-LIST-REMOVE":   {Signature: "fn Remove(value: T) bool", Documentation: "Removes the first element equal to value; false when none is found.", Rule: ruleCollections + " — 13.7 `Remove`", Receiver: "list[T] (mutable), T equatable", Target: noTargetRestriction},
	"CKM-LIST-CLEAR":    {Signature: "fn Clear() void", Documentation: "Destroys every element and sets Len to 0; capacity is retained.", Rule: ruleCollections + " — 13.5 Core list methods", Receiver: "list[T] (mutable)", Target: noTargetRestriction},
	"CKM-LIST-CONTAINS": {Signature: "fn Contains(value: T) bool", Documentation: "Reports whether an element equal to value exists.", Rule: ruleCollections + " — 13.8 `Contains`", Receiver: "list[T], T equatable", Target: noTargetRestriction},
	"CKM-LIST-INDEXOF":  {Signature: "fn IndexOf(value: T) Option[uint]", Documentation: "Index of the first element equal to value, or None.", Rule: ruleCollections + " — 13.9 `IndexOf`", Receiver: "list[T], T equatable", Target: noTargetRestriction},
	"CKM-LIST-REVERSE":  {Signature: "fn Reverse() void", Documentation: "Reverses the elements in place.", Rule: ruleCollections + " — 13.5 Core list methods", Receiver: "list[T] (mutable)", Target: noTargetRestriction},
	"CKM-LIST-SORT":     {Signature: "fn Sort() void", Documentation: "Sorts the elements in place by their natural ordering.", Rule: ruleCollections + " — 13.11 Sorting", Receiver: "list[T] (mutable), T ordered", Target: noTargetRestriction},
	"CKM-LIST-SORTBY":   {Signature: "fn SortBy(compare: fn(left: ref T, right: ref T) int) void", Documentation: "Sorts the elements in place with the comparison function.", Rule: ruleCollections + " — 13.11 Sorting", Receiver: "list[T] (mutable)", Target: noTargetRestriction},

	"CKM-MAP-REMOVE":      {Signature: "fn Remove(key: K) Option[V]", Documentation: "Removes the entry for key and returns its value, or None.", Rule: ruleCollections + " — 14.7 Core map operations", Receiver: "map[K, V] (mutable)", Target: noTargetRestriction},
	"CKM-MAP-CONTAINSKEY": {Signature: "fn ContainsKey(key: K) bool", Documentation: "Reports whether an entry for key exists.", Rule: ruleCollections + " — 14.7 Core map operations", Receiver: "map[K, V]", Target: noTargetRestriction},
	"CKM-MAP-CLEAR":       {Signature: "fn Clear() void", Documentation: "Removes and destroys every entry.", Rule: ruleCollections + " — 14.7 Core map operations", Receiver: "map[K, V] (mutable)", Target: noTargetRestriction},

	"CKM-SET-ADD":                  {Signature: "fn Add(value: T) Result[bool, CollectionError]", Documentation: "Adds value; Ok(false) when an equal element already exists.", Rule: ruleCollections + " — 15.3 Core set methods", Receiver: "set[T] (mutable)", Target: noTargetRestriction},
	"CKM-SET-REMOVE":               {Signature: "fn Remove(value: T) bool", Documentation: "Removes value; false when it is absent.", Rule: ruleCollections + " — 15.3 Core set methods", Receiver: "set[T] (mutable)", Target: noTargetRestriction},
	"CKM-SET-CONTAINS":             {Signature: "fn Contains(value: T) bool", Documentation: "Reports whether value is an element.", Rule: ruleCollections + " — 15.3 Core set methods", Receiver: "set[T]", Target: noTargetRestriction},
	"CKM-SET-CLEAR":                {Signature: "fn Clear() void", Documentation: "Removes and destroys every element.", Rule: ruleCollections + " — 15.3 Core set methods", Receiver: "set[T] (mutable)", Target: noTargetRestriction},
	"CKM-SET-UNION":                {Signature: "fn Union(other: ref set[T]) Result[set[T], CollectionError]", Documentation: "New set with the elements of both sets.", Rule: ruleCollections + " — 15.3 Core set methods", Receiver: "set[T]", Target: noTargetRestriction},
	"CKM-SET-INTERSECTION":         {Signature: "fn Intersection(other: ref set[T]) Result[set[T], CollectionError]", Documentation: "New set with the elements present in both sets.", Rule: ruleCollections + " — 15.3 Core set methods", Receiver: "set[T]", Target: noTargetRestriction},
	"CKM-SET-DIFFERENCE":           {Signature: "fn Difference(other: ref set[T]) Result[set[T], CollectionError]", Documentation: "New set with the elements of this set that are not in other.", Rule: ruleCollections + " — 15.3 Core set methods", Receiver: "set[T]", Target: noTargetRestriction},
	"CKM-SET-SYMMETRIC-DIFFERENCE": {Signature: "fn SymmetricDifference(other: ref set[T]) Result[set[T], CollectionError]", Documentation: "New set with the elements in exactly one of the two sets.", Rule: ruleCollections + " — 15.3 Core set methods", Receiver: "set[T]", Target: noTargetRestriction},

	"CKM-STRING-TOBYTEARRAY":   {Signature: "fn ToByteArray() byte[]", Documentation: "New owning array with the string's UTF-8 bytes; allocates through the active context.", Rule: ruleCompilerKnownMembers + " — `ToByteArray()`", Receiver: "string", Target: noTargetRestriction},
	"CKM-STRING-TOCHARARRAY":   {Signature: "fn ToCharArray() char[]", Documentation: "New owning array with the string's chars; allocates through the active context.", Rule: ruleCompilerKnownMembers + " — `ToCharArray()`", Receiver: "string", Target: noTargetRestriction},
	"CKM-STRING-TORUNEARRAY":   {Signature: "fn ToRuneArray() rune[]", Documentation: "New owning array with the string's Unicode scalar values; allocates through the active context.", Rule: ruleCompilerKnownMembers + " — `ToRuneArray()`", Receiver: "string", Target: noTargetRestriction},
	"CKM-STRING-FROMBYTEARRAY": {Signature: "static fn FromByteArray(bytes: ref byte[]) Result[string, StringError]", Documentation: "New string from UTF-8 bytes, validating the encoding.", Rule: ruleCompilerKnownMembers + " — `FromByteArray`", Receiver: "string (type-qualified)", Target: noTargetRestriction},
	"CKM-STRING-FROMRUNEARRAY": {Signature: "static fn FromRuneArray(runes: ref rune[]) Result[string, StringError]", Documentation: "New string from Unicode scalar values.", Rule: ruleCompilerKnownMembers + " — `FromRuneArray`", Receiver: "string (type-qualified)", Target: noTargetRestriction},

	"CKM-NUMERIC-MIN":   {Signature: "static property Min: T", Documentation: "Smallest finite value of the numeric type.", Rule: ruleCompilerKnownMembers + " — Fundamental numeric associated properties", Receiver: "numeric type (type-qualified)", Target: "int and uint follow the target pointer width"},
	"CKM-NUMERIC-MAX":   {Signature: "static property Max: T", Documentation: "Largest finite value of the numeric type.", Rule: ruleCompilerKnownMembers + " — Fundamental numeric associated properties", Receiver: "numeric type (type-qualified)", Target: "int and uint follow the target pointer width"},
	"CKM-NUMERIC-BITS":  {Signature: "static property Bits: uint", Documentation: "Width of the numeric representation in bits.", Rule: ruleCompilerKnownMembers + " — Fundamental numeric associated properties", Receiver: "numeric type (type-qualified)", Target: "int, uint, and plain float follow the target pointer width"},
	"CKM-DECIMAL-SCALE": {Signature: "static property Scale: uint", Documentation: "Number of fractional decimal digits of the decimal type.", Rule: ruleCompilerKnownMembers + " — Decimal types", Receiver: "decimal type (type-qualified)", Target: noTargetRestriction},

	"CKM-RAWPTR-READ":           {Signature: "unsafe fn Read() T", Documentation: "Reads the pointee.", Rule: ruleRawPointers, Receiver: "RawPtr[T]", Target: noTargetRestriction},
	"CKM-RAWPTR-WRITE":          {Signature: "unsafe fn Write(value: T) void", Documentation: "Writes value to the pointee.", Rule: ruleRawPointers, Receiver: "RawPtr[T]", Target: noTargetRestriction},
	"CKM-RAWPTR-VOLATILE-READ":  {Signature: "unsafe fn VolatileRead() T", Documentation: "Observable physical-storage read that the optimizer may not remove, merge, or reorder.", Rule: "rules/platform/volatile.md", Receiver: "RawPtr[T] to a validated physical region", Target: "requires a target region with validated volatile access"},
	"CKM-RAWPTR-VOLATILE-WRITE": {Signature: "unsafe fn VolatileWrite(value: T) void", Documentation: "Observable physical-storage write that the optimizer may not remove, merge, or reorder.", Rule: "rules/platform/volatile.md", Receiver: "RawPtr[T] to a validated physical region", Target: "requires a target region with validated volatile access"},
	"CKM-RAWPTR-OFFSET":         {Signature: "fn Offset(count: int) RawPtr[T]", Documentation: "Pointer moved by count elements.", Rule: ruleRawPointers, Receiver: "typed RawPtr[T]", Target: noTargetRestriction},
	"CKM-RAWPTR-ADDBYTES":       {Signature: "fn AddBytes(count: int) RawPtr[byte]", Documentation: "Pointer moved by count bytes.", Rule: ruleRawPointers, Receiver: "RawPtr[byte]", Target: noTargetRestriction},
	"CKM-RAWPTR-DIFFERENCE":     {Signature: "fn Difference(other: RawPtr[T]) int", Documentation: "Distance in elements between two pointers into the same allocation.", Rule: ruleRawPointers, Receiver: "typed RawPtr[T]", Target: noTargetRestriction},

	"CKM-ARENA-NEW":          {Signature: "fn New[T]() Result[ref mut T, AllocationError]", Documentation: "Allocates one default-initialized T in the arena.", Rule: ruleArena, Receiver: "mutable Arena", Target: noTargetRestriction},
	"CKM-ARENA-ALLOC":        {Signature: "fn Alloc[T](count: uint) Result[ref mut T[], AllocationError]", Documentation: "Allocates count default-initialized elements in the arena.", Rule: ruleArena, Receiver: "mutable Arena", Target: noTargetRestriction},
	"CKM-ARENA-RESET":        {Signature: "fn Reset() void", Documentation: "Releases every allocation of the current generation for reuse; earlier references become invalid.", Rule: ruleArena, Receiver: "mutable Arena", Target: noTargetRestriction},
	"CKM-ARENA-RELEASE":      {Signature: "fn Release() void", Documentation: "Returns the arena's backing storage.", Rule: ruleArena, Receiver: "mutable Arena", Target: noTargetRestriction},
	"CKM-ARENA-FROMBUFFER":   {Signature: "static fn FromBuffer(buffer: ref mut byte[]) Arena", Documentation: "Arena over caller-provided storage; never allocates.", Rule: ruleArena, Receiver: "Arena (type-qualified)", Target: noTargetRestriction},
	"CKM-ARENA-WITHCAPACITY": {Signature: "static fn WithCapacity(bytes: uint) Result[Arena, AllocationError]", Documentation: "Arena with a fixed-capacity backing allocation.", Rule: ruleArena, Receiver: "Arena (type-qualified)", Target: noTargetRestriction},
	"CKM-ARENA-GROWABLE":     {Signature: "static fn Growable() Arena", Documentation: "Arena that grows its backing storage on demand.", Rule: ruleArena, Receiver: "Arena (type-qualified)", Target: "requires a hosted allocation context"},

	"CKM-THREAD-START":   {Rule: ruleThreads, Receiver: "Thread", Target: "requires a target plan that provides operating-system threads"},
	"CKM-THREAD-OBSERVE": {Rule: ruleThreads, Receiver: "Thread", Target: "requires a target plan that provides operating-system threads"},
}

// compilerKnownSyntheticFamilies covers receiver-dependent IDs by prefix.
var compilerKnownSyntheticFamilies = []struct {
	prefix string
	info   compilerKnownSyntheticInfo
}{
	{"CKM-LEN-", compilerKnownSyntheticInfo{Signature: "property Len: uint", Documentation: "Number of live elements; for string the encoded byte length.", Rule: ruleCompilerKnownMembers + " — `Len`", Receiver: "string, T[N], T[], slice, list, map, or set", Target: noTargetRestriction}},
	{"CKM-ISEMPTY-", compilerKnownSyntheticInfo{Signature: "property IsEmpty: bool", Documentation: "True when Len is 0.", Rule: ruleCompilerKnownMembers + " — `IsEmpty`", Receiver: "T[N], T[], slice, list, map, or set", Target: noTargetRestriction}},
	{"CKM-TOSTRING-", compilerKnownSyntheticInfo{Signature: "fn ToString() Result[string, StringError]", Documentation: "Text form of the receiver; fails with StringError on allocation or format failure.", Rule: ruleCompilerKnownMembers + " — `ToString()`", Receiver: "any value with a ToString surface", Target: noTargetRestriction}},
	{"CKM-RESULT-", compilerKnownSyntheticInfo{Rule: "rules/errors/errorhandling.md — Result projections", Receiver: "Result[T, E]", Target: noTargetRestriction}},
}

// withSyntheticMetadata fills the presentation metadata of a registry entry
// that its builder left empty; existing signatures and documentation win.
func withSyntheticMetadata(member CompilerKnownMember) CompilerKnownMember {
	info, ok := compilerKnownSyntheticCatalog[member.ID]
	if !ok {
		for _, family := range compilerKnownSyntheticFamilies {
			if strings.HasPrefix(member.ID, family.prefix) {
				info, ok = family.info, true
				break
			}
		}
	}
	if !ok {
		return member
	}
	if member.Signature == "" {
		member.Signature = info.Signature
	}
	if member.Documentation == "" {
		member.Documentation = info.Documentation
	}
	if member.Rule == "" {
		member.Rule = info.Rule
	}
	if member.Receiver == "" {
		member.Receiver = info.Receiver
	}
	if member.TargetRestriction == "" {
		member.TargetRestriction = info.Target
	}
	return member
}

// CompilerKnownSyntheticDefinition renders the read-only synthetic definition
// of a compiler-known member: a comment block stating that it is
// compiler-known, its normative rulebook section, receiver pattern,
// signature, effects, unsafe requirement, and target restrictions, followed
// by the signature line. It is generated text, never user source.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "Synthetic definitions"
//   - rules/library/core-library.md — 18.3 Synthetic source positions
func CompilerKnownSyntheticDefinition(member CompilerKnownMember) string {
	member = withSyntheticMetadata(member)
	signature := member.Signature
	if signature == "" {
		signature = fmt.Sprintf("%s %s", member.Kind, member.Name)
	}
	effects := "none"
	if len(member.Effects) > 0 {
		names := make([]string, 0, len(member.Effects))
		for _, effect := range member.Effects {
			names = append(names, string(effect))
		}
		effects = strings.Join(names, ", ")
	}
	unsafe := "no"
	if member.Unsafe {
		unsafe = "yes; use only inside unsafe"
	}
	orEmpty := func(value string, fallback string) string {
		if value == "" {
			return fallback
		}
		return value
	}
	var text strings.Builder
	text.WriteString("// Synthetic read-only definition generated from the compiler-known member\n")
	text.WriteString("// registry. This is not source code and cannot be edited or compiled.\n")
	text.WriteString("//\n")
	fmt.Fprintf(&text, "// Member:     %s (%s)\n", member.Name, member.ID)
	fmt.Fprintf(&text, "// Category:   compiler-known %s", member.Kind)
	if member.Category != "" {
		fmt.Fprintf(&text, ", %s", member.Category)
	}
	text.WriteString("\n")
	fmt.Fprintf(&text, "// Rule:       %s\n", orEmpty(member.Rule, ruleCompilerKnownMembers))
	fmt.Fprintf(&text, "// Receiver:   %s\n", orEmpty(member.Receiver, "see the rulebook section"))
	fmt.Fprintf(&text, "// Effects:    %s\n", effects)
	fmt.Fprintf(&text, "// Unsafe:     %s\n", unsafe)
	fmt.Fprintf(&text, "// Target:     %s\n", orEmpty(member.TargetRestriction, noTargetRestriction))
	if len(member.LegacyNames) > 0 {
		fmt.Fprintf(&text, "// Legacy:     %s (migrate to %s)\n", strings.Join(member.LegacyNames, ", "), member.Name)
	}
	if member.StructuralMutation {
		text.WriteString("// Structural: changes the receiver's element count or storage\n")
	}
	if member.Documentation != "" {
		text.WriteString("//\n")
		for _, line := range strings.Split(member.Documentation, "\n") {
			fmt.Fprintf(&text, "// %s\n", line)
		}
	}
	text.WriteString("\n")
	text.WriteString(signature)
	text.WriteString("\n")
	return text.String()
}

// CompilerKnownSyntheticSignatureLine is the zero-based line of the signature
// in CompilerKnownSyntheticDefinition, where navigation places the cursor.
func CompilerKnownSyntheticSignatureLine(text string) int {
	return strings.Count(strings.TrimRight(text, "\n"), "\n")
}
