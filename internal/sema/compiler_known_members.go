package sema

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

type CompilerKnownMemberKind string

const (
	CompilerKnownProperty           CompilerKnownMemberKind = "property"
	CompilerKnownMethod             CompilerKnownMemberKind = "method"
	CompilerKnownAssociatedFunction CompilerKnownMemberKind = "associated-function"
)

type CompilerKnownMember struct {
	ID            string
	Name          string
	LegacyNames   []string
	Kind          CompilerKnownMemberKind
	Result        Type
	Signature     string
	Documentation string
	Unsafe        bool
	Effects       []EffectKind
	// StructuralMutation is an operation-contract fact consumed by parameter
	// usage analysis; it is not inferred from the source-level member name.
	StructuralMutation bool
	// Category is the semantic-authority classification that determines
	// replacement and conflict policy.
	Category CompilerKnownMemberCategory
	// Rule, Receiver, and TargetRestriction are presentation metadata for
	// hover and synthetic definitions (compiler_known_synthetic.go).
	Rule              string
	Receiver          string
	TargetRestriction string
}

// CompilerKnownMemberCategory classifies a compiler-known member by semantic
// authority. The labels are compiler registry categories, not Sec enums.
//
// Rules:
//   - rules/corrections/applied/compiler-known-fundamentals-cross-rulebook-correction-20260907.md — § 6 "Compiler-known fundamental categories", § 22 "Registry synchronization"
type CompilerKnownMemberCategory string

const (
	CompilerProvidedFallbackMember        CompilerKnownMemberCategory = "CompilerProvidedFallbackMember"
	AuthoritativeCompilerSemanticProperty CompilerKnownMemberCategory = "AuthoritativeCompilerSemanticProperty"
	CompilerKnownOperation                CompilerKnownMemberCategory = "CompilerKnownOperation"
	PrivilegedCoreMember                  CompilerKnownMemberCategory = "PrivilegedCoreMember"
	OrdinaryUserMember                    CompilerKnownMemberCategory = "OrdinaryUserMember"
)

// UserReplacementPermitted reports whether an exact user-owned member may
// replace this compiler-known member on an eligible user-owned type.
//
// Rules:
//   - rules/corrections/applied/compiler-known-fundamentals-cross-rulebook-correction-20260907.md — §§ 7, 9, 11, 22, 23
func (category CompilerKnownMemberCategory) UserReplacementPermitted() bool {
	return category == CompilerProvidedFallbackMember || category == OrdinaryUserMember
}

// classifyCompilerKnownMember assigns the § 6 category: the universal
// ToString() is the compiler-provided fallback, compiler-known properties
// expose authoritative semantic facts (layout, length, emptiness, shaped and
// numeric facts), and compiler-known methods and associated functions are
// compiler-known operations.
//
// Rules:
//   - rules/corrections/applied/compiler-known-fundamentals-cross-rulebook-correction-20260907.md — §§ 6–8, 11, 22
func classifyCompilerKnownMember(member CompilerKnownMember) CompilerKnownMember {
	if member.Category != "" {
		return member
	}
	switch {
	case member.Name == "ToString" && member.Kind == CompilerKnownMethod && strings.HasPrefix(member.ID, "CKM-TOSTRING"):
		member.Category = CompilerProvidedFallbackMember
	case member.Kind == CompilerKnownProperty:
		member.Category = AuthoritativeCompilerSemanticProperty
	default:
		member.Category = CompilerKnownOperation
	}
	return member
}

type CompilerKnownFunction struct {
	ID         string
	Name       string
	Parameters []FunctionParameter
	Result     Type
	Internal   bool
	OwnerFile  string
}

// CompilerKnownValue describes a compiler-owned value expression whose
// semantics cannot be supplied by an ordinary source declaration.
type CompilerKnownValue struct {
	ID                 string
	Name               string
	Result             Type
	Internal           bool
	Effects            []EffectKind
	RequiredCapability string
}

// CompilerKnownValues is the canonical catalog for compiler-owned value
// intrinsics. Values in this catalog are not installed as ordinary symbols;
// Sema applies their authority and visibility rules at each use site.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "Private core UTC wall-clock intrinsic"
//   - rules/types/temporal.md — §3 "UTC wall-clock access"
func CompilerKnownValues() []CompilerKnownValue {
	return []CompilerKnownValue{
		{
			ID:                 "CKV-TEMPORAL-NOW",
			Name:               "_now",
			Result:             builtinType("datetime"),
			Internal:           true,
			Effects:            []EffectKind{EffectMayUseNondeterministicInput},
			RequiredCapability: "UTCWallClock",
		},
	}
}

// compilerKnownValue resolves the stable registry entry for a compiler-owned
// value spelling without exposing it through ordinary symbols.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "Private core UTC wall-clock intrinsic"
func compilerKnownValue(name string) (CompilerKnownValue, bool) {
	for _, value := range CompilerKnownValues() {
		if value.Name == name {
			return value, true
		}
	}
	return CompilerKnownValue{}, false
}

// isCompilerKnownValueName reserves compiler-owned value identities against
// conflicting source declarations.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "Private core UTC wall-clock intrinsic"
func isCompilerKnownValueName(name string) bool {
	_, ok := compilerKnownValue(name)
	return ok
}

// CompilerKnownFunctions is the canonical catalog used to reserve and expose
// compiler-owned global functions. Their detailed contextual validation stays
// in Sema, while LSP observes the same registered Function values.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "Global `len`", "Contextual `fill`", and "Internal core string-slice helper"
//   - rules/corrections/applied/compiler-known-fundamentals-cross-rulebook-correction-20260907.md — §§ 12 and 22.3 remove global SizeOf(TypeName)
func CompilerKnownFunctions() []CompilerKnownFunction {
	types := sharedBuiltinTypes()
	return []CompilerKnownFunction{
		{ID: "CKF-LEN", Name: "len", Result: types["int"]},
		{ID: "CKF-FILL", Name: "fill", Result: Type{Kind: InvalidType}},
		{
			ID:        "CKF-STRING-SLICE-UNCHECKED",
			Name:      "__StringSliceUnchecked",
			OwnerFile: "sec/core/string.sec",
			Parameters: []FunctionParameter{
				{Name: "value", Type: types["string"]},
				{Name: "start", Type: types["uint"]},
				{Name: "end", Type: types["uint"]},
			},
			Result:   types["string"],
			Internal: true,
		},
	}
}

func compilerKnownFunction(name string) (CompilerKnownFunction, bool) {
	for _, function := range CompilerKnownFunctions() {
		if function.Name == name {
			return function, true
		}
	}
	return CompilerKnownFunction{}, false
}

func CompilerKnownMembersForType(typ Type, static bool) []CompilerKnownMember {
	members := compilerKnownValueMembers(typ)
	if static {
		members = compilerKnownStaticMembers(typ)
	}
	for index := range members {
		members[index] = withSyntheticMetadata(classifyCompilerKnownMember(members[index]))
	}
	return members
}

// compilerKnownValueMembers builds the canonical registry view for value
// receivers, including operation-contract facts consumed by semantic analyses.
// Structural mutation is attached to the registry entry so downstream passes
// do not recreate member semantics from spellings.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "Registry"
//   - rules/compiler/compiler_known_members.md — "Stable member identity"
//   - rules/analysis/parameter_usage_analysis.md — "Structural collection operations"
func compilerKnownValueMembers(typ Type) []CompilerKnownMember {
	members := []CompilerKnownMember{}
	uintType := builtinType("uint")
	boolType := builtinType("bool")

	members = append(members, compilerKnownShapedFactMembers(typ, false)...)

	if compilerKnownPointerReceiver(typ) {
		members = append(members, CompilerKnownMember{ID: "CKM-PTR-VALUE", Name: "Ptr", LegacyNames: []string{"ptr"}, Kind: CompilerKnownProperty, Result: compilerKnownRawPointerResult(typ), Unsafe: true})
	}
	if compilerKnownValueSizeReceiver(typ) {
		members = append(members, CompilerKnownMember{ID: "CKM-SIZEOF-VALUE", Name: "SizeOf", Kind: CompilerKnownProperty, Result: uintType})
	}
	if dereferenceType(typ).Kind == VariadicPackType {
		// rules/declarations/functions.md section 30 exposes Len on a native
		// pack without implying array/slice methods, contiguity, or a pointer.
		members = append(members, CompilerKnownMember{ID: "CKM-LEN-VARIADIC-PACK", Name: "Len", LegacyNames: []string{"len"}, Kind: CompilerKnownProperty, Result: uintType})
	} else if compilerKnownSequenceType(typ) {
		members = append(members, CompilerKnownMember{ID: compilerKnownLenID(typ), Name: "Len", LegacyNames: []string{"len"}, Kind: CompilerKnownProperty, Result: uintType})
		if dereferenceType(typ).Kind != StringType {
			members = append(members, CompilerKnownMember{ID: compilerKnownIsEmptyID(typ), Name: "IsEmpty", Kind: CompilerKnownProperty, Result: boolType})
		}
	}
	if compilerKnownToStringReceiver(typ) {
		members = append(members, CompilerKnownMember{ID: compilerKnownToStringID(typ), Name: "ToString", Kind: CompilerKnownMethod, Result: toStringResultType()})
	}
	sequence := dereferenceType(typ)
	if sequence.Kind == ResultType && len(sequence.TypeArgs) == 2 {
		okRef := compilerKnownSharedReference(sequence.TypeArgs[0])
		errRef := compilerKnownSharedReference(sequence.TypeArgs[1])
		members = append(members,
			CompilerKnownMember{
				ID:            "CKM-RESULT-OK",
				Name:          "Ok",
				Kind:          CompilerKnownMethod,
				Result:        compilerKnownOption(sequence.TypeArgs[0]),
				Signature:     "fn Ok() Option[T]",
				Documentation: "Consumes an owned Result and returns its success payload as an Option.",
			},
			CompilerKnownMember{
				ID:            "CKM-RESULT-ERR",
				Name:          "Err",
				Kind:          CompilerKnownMethod,
				Result:        compilerKnownOption(sequence.TypeArgs[1]),
				Signature:     "fn Err() Option[E]",
				Documentation: "Consumes an owned Result and returns its error payload as an Option.",
			},
			CompilerKnownMember{
				ID:            "CKM-RESULT-OK-REF",
				Name:          "OkRef",
				Kind:          CompilerKnownProperty,
				Result:        compilerKnownOption(okRef),
				Signature:     "property OkRef: Option[ref T]",
				Documentation: "Borrows the success payload without consuming the Result.",
			},
			CompilerKnownMember{
				ID:            "CKM-RESULT-ERR-REF",
				Name:          "ErrRef",
				Kind:          CompilerKnownProperty,
				Result:        compilerKnownOption(errRef),
				Signature:     "property ErrRef: Option[ref E]",
				Documentation: "Borrows the error payload without consuming the Result.",
			},
		)
	}
	members = append(members, compilerKnownThreadLocalMembers(sequence)...)
	if sequence.Kind == ArrayType && arrayShapeOf(sequence) == ArrayShapeDynamic && sequence.Element != nil {
		members = append(members,
			CompilerKnownMember{ID: "CKM-DYNAMIC-ARRAY-APPEND", Name: "Append", Kind: CompilerKnownMethod, Result: compilerKnownResult(builtinType("void"), builtinType("CollectionError")), StructuralMutation: true},
			CompilerKnownMember{ID: "CKM-DYNAMIC-ARRAY-CLEAR", Name: "Clear", Kind: CompilerKnownMethod, Result: builtinType("void"), StructuralMutation: true},
			CompilerKnownMember{ID: "CKM-DYNAMIC-ARRAY-REMOVEAT", Name: "RemoveAt", Kind: CompilerKnownMethod, Result: compilerKnownOption(*sequence.Element), StructuralMutation: true},
		)
	}
	if compilerKnownMutableSlice(typ) {
		members = append(members,
			CompilerKnownMember{ID: "CKM-MUTABLE-SLICE-REVERSE", Name: "Reverse", Kind: CompilerKnownMethod, Result: builtinType("void")},
			CompilerKnownMember{ID: "CKM-MUTABLE-SLICE-FILL", Name: "Fill", Kind: CompilerKnownMethod, Result: builtinType("void")},
		)
	}
	if sequence.Name == "list" && len(sequence.TypeArgs) == 1 {
		element := sequence.TypeArgs[0]
		members = append(members,
			CompilerKnownMember{ID: "CKM-LIST-CAPACITY", Name: "Capacity", Kind: CompilerKnownProperty, Result: uintType},
			CompilerKnownMember{ID: "CKM-LIST-APPEND", Name: "Append", Kind: CompilerKnownMethod, Result: compilerKnownResult(builtinType("void"), builtinType("CollectionError")), StructuralMutation: true},
			CompilerKnownMember{ID: "CKM-LIST-INSERT", Name: "Insert", Kind: CompilerKnownMethod, Result: compilerKnownResult(boolType, builtinType("CollectionError")), StructuralMutation: true},
			CompilerKnownMember{ID: "CKM-LIST-REMOVEAT", Name: "RemoveAt", Kind: CompilerKnownMethod, Result: compilerKnownOption(element), StructuralMutation: true},
			CompilerKnownMember{ID: "CKM-LIST-REMOVE", Name: "Remove", Kind: CompilerKnownMethod, Result: boolType, StructuralMutation: true},
			CompilerKnownMember{ID: "CKM-LIST-CLEAR", Name: "Clear", Kind: CompilerKnownMethod, Result: builtinType("void"), StructuralMutation: true},
			CompilerKnownMember{ID: "CKM-LIST-CONTAINS", Name: "Contains", Kind: CompilerKnownMethod, Result: boolType},
			CompilerKnownMember{ID: "CKM-LIST-INDEXOF", Name: "IndexOf", Kind: CompilerKnownMethod, Result: compilerKnownOption(uintType)},
			CompilerKnownMember{ID: "CKM-LIST-REVERSE", Name: "Reverse", Kind: CompilerKnownMethod, Result: builtinType("void")},
			CompilerKnownMember{ID: "CKM-LIST-SORT", Name: "Sort", Kind: CompilerKnownMethod, Result: builtinType("void")},
			CompilerKnownMember{ID: "CKM-LIST-SORTBY", Name: "SortBy", Kind: CompilerKnownMethod, Result: builtinType("void")},
		)
	}
	if sequence.Name == "map" && len(sequence.TypeArgs) == 2 {
		members = append(members,
			CompilerKnownMember{ID: "CKM-MAP-REMOVE", Name: "Remove", Kind: CompilerKnownMethod, Result: compilerKnownOption(sequence.TypeArgs[1]), StructuralMutation: true},
			CompilerKnownMember{ID: "CKM-MAP-CONTAINSKEY", Name: "ContainsKey", Kind: CompilerKnownMethod, Result: boolType},
			CompilerKnownMember{ID: "CKM-MAP-CLEAR", Name: "Clear", Kind: CompilerKnownMethod, Result: builtinType("void"), StructuralMutation: true},
		)
	}
	if sequence.Name == "set" && len(sequence.TypeArgs) == 1 {
		members = append(members,
			CompilerKnownMember{ID: "CKM-SET-ADD", Name: "Add", Kind: CompilerKnownMethod, Result: compilerKnownResult(boolType, builtinType("CollectionError")), StructuralMutation: true},
			CompilerKnownMember{ID: "CKM-SET-REMOVE", Name: "Remove", Kind: CompilerKnownMethod, Result: boolType, StructuralMutation: true},
			CompilerKnownMember{ID: "CKM-SET-CONTAINS", Name: "Contains", Kind: CompilerKnownMethod, Result: boolType},
			CompilerKnownMember{ID: "CKM-SET-CLEAR", Name: "Clear", Kind: CompilerKnownMethod, Result: builtinType("void"), StructuralMutation: true},
			CompilerKnownMember{ID: "CKM-SET-UNION", Name: "Union", Kind: CompilerKnownMethod, Result: compilerKnownResult(sequence, builtinType("CollectionError"))},
			CompilerKnownMember{ID: "CKM-SET-INTERSECTION", Name: "Intersection", Kind: CompilerKnownMethod, Result: compilerKnownResult(sequence, builtinType("CollectionError"))},
			CompilerKnownMember{ID: "CKM-SET-DIFFERENCE", Name: "Difference", Kind: CompilerKnownMethod, Result: compilerKnownResult(sequence, builtinType("CollectionError"))},
			CompilerKnownMember{ID: "CKM-SET-SYMMETRIC-DIFFERENCE", Name: "SymmetricDifference", Kind: CompilerKnownMethod, Result: compilerKnownResult(sequence, builtinType("CollectionError"))},
		)
	}
	if typ.Kind == StringType {
		members = append(members,
			CompilerKnownMember{ID: "CKM-STRING-TOBYTEARRAY", Name: "ToByteArray", Kind: CompilerKnownMethod, Result: compilerKnownDynamicArray(builtinType("byte"))},
			CompilerKnownMember{ID: "CKM-STRING-TOCHARARRAY", Name: "ToCharArray", Kind: CompilerKnownMethod, Result: compilerKnownDynamicArray(builtinType("char"))},
			CompilerKnownMember{ID: "CKM-STRING-TORUNEARRAY", Name: "ToRuneArray", Kind: CompilerKnownMethod, Result: compilerKnownDynamicArray(builtinType("rune"))},
		)
	}
	if typ.Kind == RawPtrType {
		members = append(members,
			CompilerKnownMember{ID: "CKM-RAWPTR-READ", Name: "Read", Kind: CompilerKnownMethod, Result: compilerKnownRawPointerElement(typ), Unsafe: true},
			CompilerKnownMember{ID: "CKM-RAWPTR-WRITE", Name: "Write", Kind: CompilerKnownMethod, Result: builtinType("void"), Unsafe: true},
			// rules/platform/volatile.md sections 9-11: volatile access is a
			// distinct unsafe, effectful operation, not an alias for Read/Write.
			CompilerKnownMember{ID: "CKM-RAWPTR-VOLATILE-READ", Name: "VolatileRead", Kind: CompilerKnownMethod, Result: compilerKnownRawPointerElement(typ), Unsafe: true, Effects: []EffectKind{EffectVolatileRead}},
			CompilerKnownMember{ID: "CKM-RAWPTR-VOLATILE-WRITE", Name: "VolatileWrite", Kind: CompilerKnownMethod, Result: builtinType("void"), Unsafe: true, Effects: []EffectKind{EffectVolatileWrite}},
			CompilerKnownMember{ID: "CKM-RAWPTR-OFFSET", Name: "Offset", Kind: CompilerKnownMethod, Result: typ, Unsafe: true},
			CompilerKnownMember{ID: "CKM-RAWPTR-ADDBYTES", Name: "AddBytes", Kind: CompilerKnownMethod, Result: typ, Unsafe: true},
			CompilerKnownMember{ID: "CKM-RAWPTR-DIFFERENCE", Name: "Difference", Kind: CompilerKnownMethod, Result: builtinType("int"), Unsafe: true},
		)
	}
	if typ.Name == "Arena" {
		members = append(members,
			CompilerKnownMember{ID: "CKM-ARENA-NEW", Name: "New", Kind: CompilerKnownMethod},
			CompilerKnownMember{ID: "CKM-ARENA-ALLOC", Name: "Alloc", Kind: CompilerKnownMethod},
			CompilerKnownMember{ID: "CKM-ARENA-RESET", Name: "Reset", Kind: CompilerKnownMethod, Result: builtinType("void")},
			CompilerKnownMember{ID: "CKM-ARENA-RELEASE", Name: "Release", Kind: CompilerKnownMethod, Result: builtinType("void")},
		)
	}
	members = append(members, compilerKnownCancellationMembers(sequence)...)
	members = append(members, compilerKnownThreadMembers(sequence)...)
	return members
}

// compilerKnownThreadLocalMembers exposes the canonical v2 access surface.
// Runtime initialization, thread-bound provenance, and conflicting-borrow
// enforcement remain separate semantic/lowering responsibilities; this catalog
// owns the exact source-visible member identities and their concrete result
// types for Sema and LSP.
//
// Rules:
//   - rules/concurrency/thread_local.md — §4 "Exact ThreadLocal[T] declaration"
//   - rules/concurrency/thread_local.md — §§15–17 "Borrow, BorrowMut, and Replace"
//   - rules/concurrency/thread_local.md — §64 "Completion and navigation"
func compilerKnownThreadLocalMembers(typ Type) []CompilerKnownMember {
	if typ.Name != "ThreadLocal" || len(typ.TypeArgs) != 1 {
		return nil
	}
	payload := typ.TypeArgs[0]
	payloadName := typeDisplayName(payload)
	return []CompilerKnownMember{
		{
			ID:            "CKM-THREADLOCAL-BORROW",
			Name:          "Borrow",
			Kind:          CompilerKnownMethod,
			Result:        compilerKnownSharedReference(payload),
			Signature:     "fn Borrow() ref " + payloadName,
			Documentation: "Borrows the current physical thread's lazily initialized value.",
		},
		{
			ID:            "CKM-THREADLOCAL-BORROW-MUT",
			Name:          "BorrowMut",
			Kind:          CompilerKnownMethod,
			Result:        compilerKnownMutableReference(payload),
			Signature:     "fn BorrowMut() ref mut " + payloadName,
			Documentation: "Mutably borrows the current physical thread's lazily initialized value.",
		},
		{
			ID:            "CKM-THREADLOCAL-REPLACE",
			Name:          "Replace",
			Kind:          CompilerKnownMethod,
			Result:        payload,
			Signature:     "fn Replace(value: " + payloadName + ") " + payloadName,
			Documentation: "Replaces the current physical thread's value and returns the previous value.",
		},
	}
}

// compilerKnownCancellationMembers exposes the symmetric cooperative
// cancellation request on owning task and thread handles. Requesting
// cancellation neither consumes the handle nor resolves its lifecycle duty.
//
// Rules:
//   - rules/concurrency/cancellation.md — § 5 "Symmetric task/thread cancellation surface"
//   - rules/concurrency/cancellation.md — § 6(2)–(5) "Relevant Task[T] cancellation surface"
//   - rules/concurrency/cancellation.md — § 7(2)–(4) "Relevant Thread[T] cancellation surface"
func compilerKnownCancellationMembers(typ Type) []CompilerKnownMember {
	if (typ.Name != "Task" && typ.Name != "Thread") || len(typ.TypeArgs) != 1 {
		return nil
	}
	identity := strings.ToUpper(typ.Name)
	return []CompilerKnownMember{{
		ID:            "CKM-" + identity + "-REQUEST-CANCEL",
		Name:          "RequestCancel",
		Kind:          CompilerKnownMethod,
		Result:        builtinType("void"),
		Signature:     "fn RequestCancel() void",
		Documentation: "Requests cooperative cancellation without consuming the owning handle. The request is idempotent and has no effect after terminal completion.",
	}}
}

// compilerKnownThreadMembers exposes the exact CamelCase thread-v2 surface
// that needs no further analysis: identity, logical name, and status on the
// owning Thread[T] and on the copyable ThreadObserver[T], plus observer
// creation and deferred start on the owner. Value, Panic, and Termination wait
// for terminal-availability analysis and Platform for target-resolved
// ThreadPlatform declarations, so they are deliberately absent rather than
// unchecked. Join and detach remain language operations.
//
// Rules:
//   - rules/concurrency/threads.md — § 26 "Exact public Thread[T] surface", §§ 27–29, § 40 "ThreadObserver[T]", § 41 "Creating an observer"
func compilerKnownThreadMembers(typ Type) []CompilerKnownMember {
	if (typ.Name != "Thread" && typ.Name != "ThreadObserver") || len(typ.TypeArgs) != 1 {
		return nil
	}
	identity := strings.ToUpper(typ.Name)
	members := []CompilerKnownMember{
		{
			ID: "CKM-" + identity + "-ID", Name: "ID", Kind: CompilerKnownProperty, Result: builtinType("ThreadID"),
			Signature:     "property ID: ThreadID",
			Documentation: "Sec thread identity; preserved across moves and join, and never reused for a later thread.",
		},
		{
			ID: "CKM-" + identity + "-NAME", Name: "Name", Kind: CompilerKnownProperty, Result: builtinType("string"),
			Signature:     "property Name: string",
			Documentation: "Immutable logical thread name used for observation and diagnostics.",
		},
		{
			ID: "CKM-" + identity + "-STATUS", Name: "Status", Kind: CompilerKnownProperty, Result: builtinType("ThreadStatus"),
			Signature:     "property Status: ThreadStatus",
			Documentation: "Current lifecycle status: Created, Running, Completed, Cancelled, Panicked, or Terminated.",
		},
	}
	if typ.Name == "Thread" {
		observer := builtinType("ThreadObserver")
		observer.TypeArgs = []Type{typ.TypeArgs[0]}
		members = append(members,
			CompilerKnownMember{
				ID: "CKM-THREAD-OBSERVE", Name: "Observe", Kind: CompilerKnownMethod, Result: observer,
				Signature:     "fn Observe() " + typeDisplayName(observer),
				Documentation: "Creates a copyable, non-owning, metadata-only observer; infallible and does not affect lifecycle ownership.",
			},
			CompilerKnownMember{
				ID: "CKM-THREAD-START", Name: "Start", Kind: CompilerKnownMethod,
				Result:        compilerKnownResult(builtinType("void"), builtinType("ThreadStartError")),
				Signature:     "fn Start() Result[void, ThreadStartError]",
				Documentation: "Starts a thread created with Deferred start; a thread that is not in Created state fails with ThreadStartError.InvalidState.",
			},
		)
	}
	return members
}

func compilerKnownStaticMembers(typ Type) []CompilerKnownMember {
	members := compilerKnownShapedFactMembers(typ, true)
	if compilerKnownSizedType(typ) {
		members = append(members, CompilerKnownMember{ID: "CKM-SIZEOF-TYPE", Name: "SizeOf", Kind: CompilerKnownProperty, Result: builtinType("uint")})
	}
	if isIntegerType(typ) {
		members = append(members,
			CompilerKnownMember{ID: "CKM-NUMERIC-MIN", Name: "Min", Kind: CompilerKnownProperty, Result: typ},
			CompilerKnownMember{ID: "CKM-NUMERIC-MAX", Name: "Max", Kind: CompilerKnownProperty, Result: typ},
			CompilerKnownMember{ID: "CKM-NUMERIC-BITS", Name: "Bits", Kind: CompilerKnownProperty, Result: builtinType("uint")},
		)
	}
	if typ.Kind == FloatType {
		for _, name := range []string{"Min", "Max", "Epsilon", "Infinity", "NegativeInfinity", "NaN"} {
			members = append(members, CompilerKnownMember{ID: "CKM-FLOAT-" + strings.ToUpper(name), Name: name, Kind: CompilerKnownProperty, Result: typ})
		}
	}
	if typ.Kind == DecimalType {
		members = append(members, CompilerKnownMember{ID: "CKM-DECIMAL-SCALE", Name: "Scale", Kind: CompilerKnownProperty, Result: builtinType("int")})
	}
	if typ.Kind == StringType {
		members = append(members,
			CompilerKnownMember{ID: "CKM-STRING-FROMBYTEARRAY", Name: "FromByteArray", Kind: CompilerKnownAssociatedFunction, Result: typ},
			CompilerKnownMember{ID: "CKM-STRING-FROMRUNEARRAY", Name: "FromRuneArray", Kind: CompilerKnownAssociatedFunction, Result: typ},
		)
	}
	if typ.Name == "Arena" {
		members = append(members,
			CompilerKnownMember{ID: "CKM-ARENA-FROMBUFFER", Name: "FromBuffer", Kind: CompilerKnownAssociatedFunction, Result: typ},
			CompilerKnownMember{ID: "CKM-ARENA-WITHCAPACITY", Name: "WithCapacity", Kind: CompilerKnownAssociatedFunction},
			CompilerKnownMember{ID: "CKM-ARENA-GROWABLE", Name: "Growable", Kind: CompilerKnownAssociatedFunction},
		)
	}
	return members
}

// compilerKnownShapedFactMembers exposes the read-only Rank, Shape, and Len
// facts that are completely determined by a shaped receiver's static type.
// Keeping these entries in the canonical registry makes Sema and LSP consume
// one identity.
//
// Runtime-shaped tensor and tensor_view Shape/Len are deliberately absent
// until their runtime shape semantics exist; tensor_view Rank remains
// type-known.
//
// Rules:
//   - rules/collections/shaped-types.md — § 3.1–3.5 "Shaped type families"
//   - rules/collections/shaped-types.md — § 5 "Rank, Shape, and Len"
//   - rules/collections/shaped-types.md — § 5.1 "Type-level access"
//   - rules/collections/shaped-types.md — §§ 6–6.1 "Strides"
//   - rules/collections/shaped-types.md — § 8 "Contiguity"
//   - rules/corrections/applied/compiler_known_members-shaped-correction-20260813.md — "Required read-only shaped properties" and "Type-level properties"
func compilerKnownShapedFactMembers(typ Type, static bool) []CompilerKnownMember {
	typ = dereferenceType(typ)
	rank, length, shaped := compilerKnownStaticShapedFacts(typ)
	if !shaped {
		return nil
	}

	uintType := builtinType("uint")
	members := []CompilerKnownMember{{
		ID:            "CKM-SHAPED-RANK",
		Name:          "Rank",
		Kind:          CompilerKnownProperty,
		Result:        uintType,
		Signature:     "property Rank: uint",
		Documentation: "Compile-time-known shaped rank: " + rank + ".",
	}}
	if length != "" {
		shapeType := builtinType("Shape")
		shapeType.ConstArgs = []int64{int64(len(typ.ConstArgs))}
		members = append(members, CompilerKnownMember{
			ID:            "CKM-SHAPED-SHAPE",
			Name:          "Shape",
			Kind:          CompilerKnownProperty,
			Result:        shapeType,
			Signature:     "property Shape: " + typeDisplayName(shapeType),
			Documentation: "Compile-time-known shaped extents: " + shapedStaticShape(typ) + ".",
		})
		if !static {
			stridesType := builtinType("Strides")
			stridesType.ConstArgs = []int64{int64(len(typ.ConstArgs))}
			members = append(members, CompilerKnownMember{
				ID:            "CKM-SHAPED-STRIDES",
				Name:          "Strides",
				Kind:          CompilerKnownProperty,
				Result:        stridesType,
				Signature:     "property Strides: " + typeDisplayName(stridesType),
				Documentation: "Compile-time-known canonical dense row-major element strides: " + shapedStaticDenseStrides(typ) + ".",
			})
			members = append(members, CompilerKnownMember{
				ID:            "CKM-SHAPED-IS-CONTIGUOUS",
				Name:          "IsContiguous",
				Kind:          CompilerKnownProperty,
				Result:        builtinType("bool"),
				Signature:     "property IsContiguous: bool",
				Documentation: "Compile-time-known true for a canonical dense owning shaped value.",
			})
		}
		members = append(members, CompilerKnownMember{
			ID:            "CKM-SHAPED-LEN",
			Name:          "Len",
			Kind:          CompilerKnownProperty,
			Result:        uintType,
			Signature:     "property Len: uint",
			Documentation: "Compile-time-known total logical scalar element count: " + length + ".",
		})
	}
	return members
}

// shapedStaticDenseStrides derives canonical row-major element strides using
// arbitrary precision so tooling never observes host-integer overflow.
//
// Rules:
//   - rules/collections/shaped-types.md — § 6 "Strides"
//   - rules/collections/shaped-types.md — § 6.1 "Canonical dense row-major strides"
func shapedStaticDenseStrides(typ Type) string {
	strides := make([]string, len(typ.ConstArgs))
	stride := big.NewInt(1)
	for index := len(typ.ConstArgs) - 1; index >= 0; index-- {
		strides[index] = stride.String()
		stride.Mul(stride, big.NewInt(typ.ConstArgs[index]))
	}
	return "[" + strings.Join(strides, ", ") + "]"
}

// shapedStaticShape renders the exact ordered extents encoded by a statically
// shaped owning type for compiler-owned tooling facts.
//
// Rules:
//   - rules/collections/shaped-types.md — § 3.1–3.3 "Shaped type families"
//   - rules/collections/shaped-types.md — § 5 "Rank, Shape, and Len"
func shapedStaticShape(typ Type) string {
	extents := make([]string, 0, len(typ.ConstArgs))
	for _, extent := range typ.ConstArgs {
		extents = append(extents, strconv.FormatInt(extent, 10))
	}
	return "[" + strings.Join(extents, ", ") + "]"
}

// compilerKnownStaticShapedFacts derives only facts fully encoded by the
// canonical shaped type arguments. The empty length marks a rank-only family.
//
// Rules:
//   - rules/collections/shaped-types.md — § 3.1–3.5 "Shaped type families"
//   - rules/collections/shaped-types.md — § 5 "Rank, Shape, and Len"
func compilerKnownStaticShapedFacts(typ Type) (rank string, length string, ok bool) {
	if len(typ.TypeArgs) != 1 {
		return "", "", false
	}
	switch typ.Name {
	case "vector":
		if len(typ.ConstArgs) != 1 {
			return "", "", false
		}
		return "1", shapedStaticElementCount(typ), true
	case "matrix":
		if len(typ.ConstArgs) != 2 {
			return "", "", false
		}
		return "2", shapedStaticElementCount(typ), true
	case "tensor":
		if len(typ.ConstArgs) == 0 {
			return "", "", false
		}
		return strconv.FormatInt(int64(len(typ.ConstArgs)), 10), shapedStaticElementCount(typ), true
	case "tensor_view":
		if len(typ.ConstArgs) != 1 || typ.ConstArgs[0] < 0 {
			return "", "", false
		}
		return strconv.FormatInt(typ.ConstArgs[0], 10), "", true
	default:
		return "", "", false
	}
}

// shapedStaticElementCount returns the exact Len encoded by a statically
// shaped owning type, retaining arbitrary precision for registry callers.
//
// Rules:
//   - rules/collections/shaped-types.md — § 3.1–3.3 "Shaped type families"
//   - rules/collections/shaped-types.md — § 5 "Rank, Shape, and Len"
func shapedStaticElementCount(typ Type) string {
	if typ.StaticElementCount != nil {
		return typ.StaticElementCount.String()
	}
	product := big.NewInt(1)
	for _, extent := range typ.ConstArgs {
		product.Mul(product, big.NewInt(extent))
	}
	return product.String()
}

func compilerKnownMember(typ Type, name string, static bool) (CompilerKnownMember, bool) {
	for _, member := range CompilerKnownMembersForType(typ, static) {
		if member.Name == name {
			return member, true
		}
		for _, legacy := range member.LegacyNames {
			if legacy == name {
				return member, true
			}
		}
	}
	return CompilerKnownMember{}, false
}

func compilerKnownSequenceType(typ Type) bool {
	sequence := dereferenceType(typ)
	if sequence.Kind == StringType || sequence.Kind == ArrayType || sequence.Kind == SliceType {
		return true
	}
	return compilerKnownCollectionName(sequence.Name)
}

// compilerKnownToStringReceiver exposes the universal fallback on every
// concrete value receiver. Generic and interface declarations still need a
// contract that guarantees ToString; the fallback is selected only after a
// concrete runtime value type is known.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "ToString()"
//   - rules/compiler/compiler_known_members.md — "User-defined ToString()"
//   - rules/compiler/compiler_known_members.md — "Generic lookup"
func compilerKnownToStringReceiver(typ Type) bool {
	value := dereferenceType(typ)
	switch value.Kind {
	case InvalidType, VoidType, NeverType, GenericType, InterfaceType, VariadicPackType:
		return false
	default:
		return true
	}
}

// compilerKnownInterpolationFallbackReceiver is narrower than direct
// ToString lookup. Interpolation remains an explicitly governed formatting
// context and does not opt every universal type-oriented fallback into
// data-revealing formatting.
//
// Rules:
//   - rules/foundations/operators.md — "Interpolation and formatting"
//   - rules/compiler/compiler_known_members.md — "Generic lookup"
func compilerKnownInterpolationFallbackReceiver(typ Type) bool {
	value := dereferenceType(typ)
	if value.Kind == BoolType || value.Kind == StringType || value.Kind == CharType || value.Kind == RuneType || isNumericType(value) {
		return true
	}
	if value.Kind == ArrayType || value.Kind == SliceType {
		return value.Element != nil && (value.Element.Kind == CharType || value.Element.Kind == RuneType)
	}
	return compilerKnownCollectionName(value.Name)
}

func compilerKnownCollectionName(name string) bool {
	switch name {
	case "list", "map", "set":
		return true
	default:
		return false
	}
}

func compilerKnownPointerReceiver(typ Type) bool {
	sequence := dereferenceType(typ)
	return sequence.Name != "map" && sequence.Name != "set" && compilerKnownSizedType(typ)
}

func compilerKnownValueSizeReceiver(typ Type) bool {
	sequence := dereferenceType(typ)
	return sequence.Name != "map" && sequence.Name != "set" && compilerKnownSizedType(typ)
}

func compilerKnownMutableSlice(typ Type) bool {
	return typ.Kind == ReferenceType && typ.ReferenceMutable && typ.Element != nil && typ.Element.Kind == SliceType
}

func compilerKnownSizedType(typ Type) bool {
	switch typ.Kind {
	case InvalidType, VoidType, NeverType, GenericType, InterfaceType, VariadicPackType:
		return false
	default:
		return true
	}
}

func compilerKnownRawPointerResult(typ Type) Type {
	element := typ
	if typ.Kind == StringType {
		element = builtinType("byte")
	} else {
		sequence := dereferenceType(typ)
		if (sequence.Kind == ArrayType || sequence.Kind == SliceType) && sequence.Element != nil {
			element = *sequence.Element
		} else if sequence.Name == "list" && len(sequence.TypeArgs) == 1 {
			element = sequence.TypeArgs[0]
		}
	}
	return rawPointerType(element)
}

func compilerKnownRawPointerElement(typ Type) Type {
	if len(typ.TypeArgs) == 1 {
		return typ.TypeArgs[0]
	}
	return Type{Kind: InvalidType}
}

func compilerKnownDynamicArray(element Type) Type {
	return NewDynamicArrayType(element)
}

func compilerKnownLenID(typ Type) string {
	typ = dereferenceType(typ)
	if typ.Kind == StringType {
		return "CKM-LEN-STRING"
	}
	if typ.Kind == ArrayType {
		return "CKM-LEN-ARRAY"
	}
	if compilerKnownCollectionName(typ.Name) {
		return "CKM-LEN-" + strings.ToUpper(typ.Name)
	}
	return "CKM-LEN-SLICE"
}

func compilerKnownIsEmptyID(typ Type) string {
	typ = dereferenceType(typ)
	if typ.Kind == ArrayType {
		return "CKM-ISEMPTY-ARRAY"
	}
	if typ.Kind == SliceType {
		return "CKM-ISEMPTY-SLICE"
	}
	return "CKM-ISEMPTY-" + strings.ToUpper(typ.Name)
}

// compilerKnownToStringID retains the selected fallback family after reference
// auto-dereference so later compiler clients receive one stable semantic ID.
//
// Rules: rules/compiler/compiler_known_members.md — "ToString()".
func compilerKnownToStringID(typ Type) string {
	sequence := dereferenceType(typ)
	if (sequence.Kind == ArrayType || sequence.Kind == SliceType) && sequence.Element != nil {
		switch {
		case sequence.Element.Name == "byte":
			return "CKM-TOSTRING-BYTE-SEQUENCE"
		case sequence.Element.Kind == CharType:
			return "CKM-TOSTRING-CHAR-SEQUENCE"
		case sequence.Element.Kind == RuneType:
			return "CKM-TOSTRING-RUNE-SEQUENCE"
		}
		return "CKM-TOSTRING-VALUE"
	}
	switch sequence.Kind {
	case StringType:
		return "CKM-TOSTRING-STRING"
	case BoolType:
		return "CKM-TOSTRING-BOOL"
	case IntType:
		return "CKM-TOSTRING-SIGNED-INTEGER"
	case UintType:
		return "CKM-TOSTRING-UNSIGNED-INTEGER"
	case FloatType:
		return "CKM-TOSTRING-FLOAT"
	case DecimalType:
		return "CKM-TOSTRING-DECIMAL"
	case CharType:
		return "CKM-TOSTRING-CHAR"
	case RuneType:
		return "CKM-TOSTRING-RUNE"
	default:
		return "CKM-TOSTRING-VALUE"
	}
}

func compilerKnownOption(value Type) Type {
	typ := builtinType("Option")
	typ.TypeArgs = []Type{value}
	return typ
}

// compilerKnownSharedReference constructs the immutable payload reference used
// by Result.OkRef/ErrRef without assigning receiver provenance prematurely;
// Sema attaches that provenance at the concrete member-access site.
//
// Rules:
//   - rules/errors/errorhandling.md — §6.2 "Non-consuming borrowed projections"
func compilerKnownSharedReference(value Type) Type {
	return Type{Name: referenceTypeName(value, false), Kind: ReferenceType, Element: &value}
}

func compilerKnownMutableReference(value Type) Type {
	return Type{Name: referenceTypeName(value, true), Kind: ReferenceType, Element: &value, ReferenceMutable: true}
}

func compilerKnownResult(value Type, err Type) Type {
	return Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{value, err}}
}

// rejectAuthoritativeMemberReplacement rejects a user impl member that would
// replace an authoritative compiler-known semantic property applicable to the
// impl target, such as `SizeOf`, the layout truth of the active
// CompilationPlan. Fallback members such as `ToString()` remain replaceable,
// and loader-proven trusted core keeps its privileged implementing members.
//
// Rules:
//   - rules/corrections/applied/compiler-known-fundamentals-cross-rulebook-correction-20260907.md — §§ 22, 23(3), 25(1)–(2), 26
//   - rules/compiler/compile_time_evaluation.md — § 16 "SizeOf"
//   - rules/compiler/compiler_known_members.md — authoritative semantic properties
func (a *Analyzer) rejectAuthoritativeMemberReplacement(targetName string, target Type, member ast.ImplMember) bool {
	name, token, ok := implMemberIdentity(member)
	if !ok {
		return false
	}
	// § 20: loader-proven trusted core may provide the privileged core
	// declaration that implements a compiler-known surface.
	if a.isTrustedCoreSourceToken(token) {
		return false
	}
	for _, static := range []bool{false, true} {
		known, found := compilerKnownMember(target, name, static)
		if !found || known.Category != AuthoritativeCompilerSemanticProperty {
			continue
		}
		a.addErrorAtTokenWithMetadata(
			token,
			diagnostics.AuthoritativeMemberReplacement,
			fmt.Sprintf("remove the member; %s.%s is provided by the compiler for this type", targetName, known.Name),
			"%s is an authoritative compiler-known property for %s and cannot be replaced by a user-defined member",
			known.Name,
			targetName,
		)
		return true
	}
	return false
}

// implMemberIdentity returns the declared name and name token of an impl
// member that introduces a named member.
func implMemberIdentity(member ast.ImplMember) (string, lexer.Token, bool) {
	switch member := member.(type) {
	case *ast.FunctionDeclaration:
		if member.Name != nil {
			return member.Name.Value, member.Name.Token, true
		}
	case *ast.PropertyDeclaration:
		if member.Name != nil {
			return member.Name.Value, member.Name.Token, true
		}
	case *ast.LetStatement:
		if member.Name != nil {
			return member.Name.Value, member.Name.Token, true
		}
	case *ast.EventDeclaration:
		if member.Name != nil {
			return member.Name.Value, member.Name.Token, true
		}
	}
	return "", lexer.Token{}, false
}

// ShapedFacts is the tooling presentation of the shaped facts Sema resolved
// for a type: rank, extents, total element count, whether the shape is known
// statically, and canonical dense contiguity. Empty strings mark facts that
// are not statically known, such as the extents of a tensor_view.
type ShapedFacts struct {
	Family       string
	Rank         string
	Shape        string
	Len          string
	Strides      string
	StaticShape  bool
	IsContiguous bool
}

// ShapedFactsOf exposes the compiler-known shaped facts of typ to tooling. It
// derives every value from the same registry functions that publish the
// Rank, Shape, Len, Strides, and IsContiguous members, so the LSP never keeps
// an independent shaped table.
//
// Rules:
//   - rules/tooling/lsp.md — "Shaped values"
//   - rules/collections/shaped-types.md — § 5 "Rank, Shape, and Len", § 6 "Strides", § 8 "Contiguity"
func ShapedFactsOf(typ Type) (ShapedFacts, bool) {
	typ = dereferenceType(typ)
	rank, length, ok := compilerKnownStaticShapedFacts(typ)
	if !ok {
		return ShapedFacts{}, false
	}
	facts := ShapedFacts{Family: typ.Name, Rank: rank, Len: length}
	if length != "" {
		facts.StaticShape = true
		facts.Shape = shapedStaticShape(typ)
		facts.Strides = shapedStaticDenseStrides(typ)
		facts.IsContiguous = true
	}
	return facts, true
}
