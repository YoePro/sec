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
	"CKM-SIZEOF-VALUE":      {Signature: "property SizeOf: uint", Documentation: "Byte size of the receiver's represented payload: for a string its encoded UTF-8 bytes (Len counts Unicode scalars), for contiguous sequences and lists Len times the element stride; never capacity, descriptor or allocator metadata.", Rule: ruleCompilerKnownMembers + " — Value-form `SizeOf`", Receiver: "any sized value", Target: "the result depends on the active target plan's layout"},
	"CKM-SIZEOF-TYPE":       {Signature: "static property SizeOf: uint", Documentation: "Byte size of the type's representation.", Rule: ruleCompilerKnownMembers + " — Type-form `SizeOf`", Receiver: "any sized type, written Type.SizeOf", Target: "the result depends on the active target plan's layout"},
	"CKM-LEN-VARIADIC-PACK": {Signature: "property Len: uint", Documentation: "Number of arguments in the variadic pack.", Rule: "rules/declarations/functions.md — section 30", Receiver: "variadic parameter pack", Target: noTargetRestriction},

	"CKM-DYNAMIC-ARRAY-APPEND":   {Signature: "fn Append(value: T) Result[void, CollectionError]", Documentation: "Appends value at the end, growing storage through the active allocation context; may relocate elements.", Rule: ruleCollections + " — 6.7 `Append`", Receiver: "owning dynamic array T[] (mutable)", Target: noTargetRestriction},
	"CKM-DYNAMIC-ARRAY-CLEAR":    {Signature: "fn Clear() void", Documentation: "Destroys every element and sets Len to 0; capacity is retained.", Rule: ruleCollections + " — 6.8 `Clear`", Receiver: "owning dynamic array T[] (mutable)", Target: noTargetRestriction},
	"CKM-DYNAMIC-ARRAY-REMOVEAT": {Signature: "fn RemoveAt(index: uint) Option[T]", Documentation: "Removes and returns the element at index, shifting later elements down; None when index is out of range.", Rule: ruleCollections + " — 6.9 `RemoveAt`", Receiver: "owning dynamic array T[] (mutable)", Target: noTargetRestriction},
	"CKM-MUTABLE-SLICE-REVERSE":  {Signature: "fn Reverse() void", Documentation: "Reverses the elements in place.", Rule: ruleCollections + " — 7.8 `Reverse`", Receiver: "mutable slice or array view", Target: noTargetRestriction},
	"CKM-MUTABLE-SLICE-FILL":     {Signature: "fn Fill(value: T) void", Documentation: "Assigns value to every element in place; T must be copyable.", Rule: ruleCollections + " — 7.9 Slice `Fill`", Receiver: "mutable slice or array view", Target: noTargetRestriction},

	"CKM-LIST-CAPACITY":     {Signature: "property Capacity: uint", Documentation: "Number of elements the list can hold without growing; never the live length.", Rule: ruleCollections + " — 13.4 List properties", Receiver: "list[T]", Target: noTargetRestriction},
	"CKM-LIST-RESERVE":      {Signature: "fn Reserve(additional: uint) Result[void, CollectionError]", Documentation: "Guarantees Capacity >= Len + additional; a no-op when capacity already suffices. Transactional: on failure the list is unchanged.", Rule: ruleCollections + " — 13.12 Capacity reservation", Receiver: "dynamic list[T] (mutable)", Target: noTargetRestriction},
	"CKM-LIST-WITHCAPACITY": {Signature: "static fn WithCapacity(capacity: uint) Result[list[T], CollectionError]", Documentation: "Creates an empty list with Capacity >= capacity; no element is constructed.", Rule: ruleCollections + " — 13.12 Capacity reservation", Receiver: "dynamic list[T], written list[T].WithCapacity", Target: noTargetRestriction},
	"CKM-MAP-CAPACITY":      {Signature: "property Capacity: uint", Documentation: "Number of entries the map can hold without growing; for a bounded map its declared maximum. Never buckets or bytes.", Rule: ruleCollections + " — 14.8 Capacity reservation", Receiver: "map[K, V] or map[K, V, Capacity]", Target: noTargetRestriction},
	"CKM-MAP-RESERVE":       {Signature: "fn Reserve(additional: uint) Result[void, CollectionError]", Documentation: "Guarantees Capacity >= Len + additional entries; a no-op when capacity already suffices. Transactional, including rehash.", Rule: ruleCollections + " — 14.8 Capacity reservation", Receiver: "dynamic map[K, V] (mutable)", Target: noTargetRestriction},
	"CKM-MAP-WITHCAPACITY":  {Signature: "static fn WithCapacity(capacity: uint) Result[map[K, V], CollectionError]", Documentation: "Creates an empty map with Capacity >= capacity entries; no key or value is constructed.", Rule: ruleCollections + " — 14.8 Capacity reservation", Receiver: "dynamic map[K, V], written map[K, V].WithCapacity", Target: noTargetRestriction},
	"CKM-SET-CAPACITY":      {Signature: "property Capacity: uint", Documentation: "Number of distinct elements the set can hold without growing; for a bounded set its declared maximum. Never buckets or bytes.", Rule: ruleCollections + " — 15.5 Capacity reservation", Receiver: "set[T] or set[T, Capacity]", Target: noTargetRestriction},
	"CKM-SET-RESERVE":       {Signature: "fn Reserve(additional: uint) Result[void, CollectionError]", Documentation: "Guarantees Capacity >= Len + additional elements; a no-op when capacity already suffices. Transactional, including rehash.", Rule: ruleCollections + " — 15.5 Capacity reservation", Receiver: "dynamic set[T] (mutable)", Target: noTargetRestriction},
	"CKM-SET-WITHCAPACITY":  {Signature: "static fn WithCapacity(capacity: uint) Result[set[T], CollectionError]", Documentation: "Creates an empty set with Capacity >= capacity elements; no element is constructed.", Rule: ruleCollections + " — 15.5 Capacity reservation", Receiver: "dynamic set[T], written set[T].WithCapacity", Target: noTargetRestriction},
	"CKM-LIST-APPEND":       {Signature: "fn Append(value: T) Result[void, CollectionError]", Documentation: "Appends value at the end, growing storage through the active allocation context.", Rule: ruleCollections + " — 13.5 Core list methods", Receiver: "list[T] (mutable)", Target: noTargetRestriction},
	"CKM-LIST-INSERT":       {Signature: "fn Insert(index: uint, value: T) Result[bool, CollectionError]", Documentation: "Inserts value at index, shifting later elements up; Ok(false) when index is past the end.", Rule: ruleCollections + " — 13.10 `Insert`", Receiver: "list[T] (mutable)", Target: noTargetRestriction},
	"CKM-LIST-REMOVEAT":     {Signature: "fn RemoveAt(index: uint) Option[T]", Documentation: "Removes and returns the element at index, shifting later elements down; None when index is out of range.", Rule: ruleCollections + " — 13.6 `RemoveAt`", Receiver: "list[T] (mutable)", Target: noTargetRestriction},
	"CKM-LIST-REMOVE":       {Signature: "fn Remove(value: T) bool", Documentation: "Removes the first element equal to value; false when none is found.", Rule: ruleCollections + " — 13.7 `Remove`", Receiver: "list[T] (mutable), T equatable", Target: noTargetRestriction},
	"CKM-LIST-CLEAR":        {Signature: "fn Clear() void", Documentation: "Destroys every element and sets Len to 0; capacity is retained.", Rule: ruleCollections + " — 13.5 Core list methods", Receiver: "list[T] (mutable)", Target: noTargetRestriction},
	"CKM-LIST-CONTAINS":     {Signature: "fn Contains(value: T) bool", Documentation: "Reports whether an element equal to value exists.", Rule: ruleCollections + " — 13.8 `Contains`", Receiver: "list[T], T equatable", Target: noTargetRestriction},
	"CKM-LIST-INDEXOF":      {Signature: "fn IndexOf(value: T) Option[uint]", Documentation: "Index of the first element equal to value, or None.", Rule: ruleCollections + " — 13.9 `IndexOf`", Receiver: "list[T], T equatable", Target: noTargetRestriction},
	"CKM-LIST-REVERSE":      {Signature: "fn Reverse() void", Documentation: "Reverses the elements in place.", Rule: ruleCollections + " — 13.5 Core list methods", Receiver: "list[T] (mutable)", Target: noTargetRestriction},
	"CKM-LIST-SORT":         {Signature: "fn Sort() void", Documentation: "Sorts the elements in place by their natural ordering.", Rule: ruleCollections + " — 13.11 Sorting", Receiver: "list[T] (mutable), T ordered", Target: noTargetRestriction},
	"CKM-LIST-SORTBY":       {Signature: "fn SortBy(compare: fn(left: ref T, right: ref T) int) void", Documentation: "Sorts the elements in place with the comparison function.", Rule: ruleCollections + " — 13.11 Sorting", Receiver: "list[T] (mutable)", Target: noTargetRestriction},

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

	"CKM-STRING-TOBYTEARRAY":   {Signature: "fn ToByteArray() Result[byte[], CollectionError]", Documentation: "New owning array with exactly SizeOf encoded UTF-8 bytes (byte units, no terminator); never aliases the string. Allocates through the active context; CollectionError when result storage is unavailable.", Rule: ruleCompilerKnownMembers + " — `ToByteArray()`", Receiver: "string", Target: noTargetRestriction},
	"CKM-STRING-TOCHARARRAY":   {Signature: "fn ToCharArray() char[]", Documentation: "New owning array with the string's chars; allocates through the active context.", Rule: ruleCompilerKnownMembers + " — `ToCharArray()`", Receiver: "string", Target: noTargetRestriction},
	"CKM-STRING-TORUNEARRAY":   {Signature: "fn ToRuneArray() Result[rune[], CollectionError]", Documentation: "New owning array with exactly Len Unicode scalar values in order; never aliases the string. Allocates through the active context; CollectionError when result storage is unavailable.", Rule: ruleCompilerKnownMembers + " — `ToRuneArray()`", Receiver: "string", Target: noTargetRestriction},
	"CKM-STRING-FROMBYTEARRAY": {Signature: "static fn FromByteArray(value: ref byte[]) Result[string, StringError]", Documentation: "Decodes the borrowed bytes as strict UTF-8 into an independent string; StringError.InvalidUtf8(byteOffset) names the first malformed sequence, StringError.Allocation reports storage failure. The bytes are not changed.", Rule: ruleCompilerKnownMembers + " — `FromByteArray`", Receiver: "string (type-qualified)", Target: noTargetRestriction},
	"CKM-STRING-FROMRUNEARRAY": {Signature: "static fn FromRuneArray(value: ref rune[]) Result[string, StringError]", Documentation: "Encodes the borrowed Unicode scalars as canonical UTF-8 into an independent string; StringError.Allocation reports storage failure.", Rule: ruleCompilerKnownMembers + " — `FromRuneArray`", Receiver: "string (type-qualified)", Target: noTargetRestriction},

	"CKF-ATOMIC-FENCE": {Signature: "fn Fence(order: MemoryOrder) void", Documentation: "Establishes an explicit memory-ordering fence for the current execution context; it does not itself Load or Store an Atomic[T] and adds no modification. The order is mandatory (no default), must be known at compile time, and is one of Acquire, Release, AcqRel or SeqCst; Relaxed is invalid.", Rule: "rules/concurrency/atomics.md — § 26 Exact fence surface, § 40 Fences", Receiver: "core atomic namespace, written atomic.Fence(order)", Target: "the selected CompilationPlan realizes the fence; target capability is checked separately from order validity"},

	"CKM-NUMERIC-MIN":   {Signature: "static property Min: T", Documentation: "Smallest finite value of the numeric type.", Rule: ruleCompilerKnownMembers + " — Fundamental numeric associated properties", Receiver: "numeric type (type-qualified)", Target: "int and uint follow the target pointer width"},
	"CKM-NUMERIC-MAX":   {Signature: "static property Max: T", Documentation: "Largest finite value of the numeric type.", Rule: ruleCompilerKnownMembers + " — Fundamental numeric associated properties", Receiver: "numeric type (type-qualified)", Target: "int and uint follow the target pointer width"},
	"CKM-NUMERIC-BITS":  {Signature: "static property Bits: uint", Documentation: "Width of the numeric representation in bits.", Rule: ruleCompilerKnownMembers + " — Fundamental numeric associated properties", Receiver: "numeric type (type-qualified)", Target: "int, uint, and plain float follow the target pointer width"},
	"CKM-DECIMAL-SCALE": {Signature: "property Scale: uint8", Documentation: "Semantic scale (number of fractional decimal digits) of this decimal value (core-library.md § 10.1). Operator result scales await MD-066.", Rule: ruleCompilerKnownMembers + " — Decimal types", Receiver: "decimal value", Target: noTargetRestriction},

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
	if member.BoundReceiver != "" {
		fmt.Fprintf(&text, "// Instance:   %s\n", member.BoundReceiver)
	}
	fmt.Fprintf(&text, "// Effects:    %s\n", effects)
	fmt.Fprintf(&text, "// Unsafe:     %s\n", unsafe)
	fmt.Fprintf(&text, "// Target:     %s\n", orEmpty(member.TargetRestriction, noTargetRestriction))
	if len(member.LegacyNames) > 0 {
		fmt.Fprintf(&text, "// Legacy:     %s (migrate to %s)\n", strings.Join(member.LegacyNames, ", "), member.Name)
	}
	if member.StructuralMutation {
		text.WriteString("// Structural: changes the receiver's element count or storage\n")
	}
	if len(member.CallerObligations) > 0 {
		text.WriteString("// Caller obligations (unsafe does not prove these):\n")
		for _, obligation := range member.CallerObligations {
			fmt.Fprintf(&text, "// - %s\n", obligation)
		}
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

// CompilerKnownSymbolAt exposes resolved members, interface requirements and
// private value intrinsics to tooling through the same presentation model.
// Untrusted value uses have no fact and cannot acquire a synthetic identity.
// Rules: rules/compiler/compiler_known_members.md — LSP, Private core UTC wall-clock intrinsic;
// rules/types/temporal.md — §3.
func (a *Analyzer) CompilerKnownSymbolAt(file string, line int, column int) (CompilerKnownMember, bool) {
	if member, ok := a.CompilerKnownMemberAt(file, line, column); ok {
		return member, true
	}
	value, ok := a.CompilerKnownValueAt(file, line, column)
	if !ok {
		return CompilerKnownMember{}, false
	}
	return compilerKnownValuePresentation(value), true
}

// compilerKnownValuePresentation projects canonical value metadata for tooling.
// Rules: rules/compiler/compiler_known_members.md — LSP, Private core UTC wall-clock intrinsic;
// rules/types/temporal.md — §3.
func compilerKnownValuePresentation(value CompilerKnownValue) CompilerKnownMember {
	return CompilerKnownMember{
		ID: value.ID, Name: value.Name, Kind: CompilerKnownValueExpression,
		Result: value.Result, Signature: value.Name + ": " + typeDisplayName(value.Result),
		Effects: value.Effects, Category: CompilerKnownOperation,
		Rule:              "rules/types/temporal.md — §3; rules/compiler/compiler_known_members.md — Private core UTC wall-clock intrinsic",
		Receiver:          "loader-proven core source only",
		TargetRestriction: value.RequiredCapability + " must be Supported and Enabled in the frozen CompilationPlan",
		Documentation:     "One UTC wall-clock read per evaluation; unavailable in compile-time evaluation and static initialization.",
	}
}
