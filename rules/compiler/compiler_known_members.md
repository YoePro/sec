# Compiler-Known Members

- **Status:** Normative
- **Created:** 2026-08-05
- **Last updated:** 2026-11-07 (stable chapter, subsection, and paragraph numbering)
- **Document revision:** 1
- **Sec language version:** 0.1
- **Canonical path:** `rules/compiler/compiler_known_members.md`
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`
- **Implementation governance:** `governance/core.yaml` (`compiler.compiler-known-surfaces`)
- **Related rulebooks:** `rules/library/core-library.md`, `rules/types/types.md`, `rules/collections/collections.md`, `rules/memory/reference_model.md`, `rules/memory/layout.md`, `rules/memory/allocation.md`, `rules/memory/arena.md`, `rules/memory/unsafe.md`, `rules/analysis/effect_analysis.md`, `rules/semantic_ir.md`, `rules/platform/ffi.md`, `rules/tooling/diagnostics.md`, `rules/tooling/lsp.md`

---

## § 1 Status

**§ 1(1)** This document is the canonical rulebook for compiler-known functions, methods,
properties, associated members, and internal operations in Sec 0.1.

**§ 1(2)** It defines:

- what compiler-known means;
- the boundary between compiler semantics, core declarations, core helper
  implementations, standard-library APIs, and target helpers;
- the canonical member registry;
- name resolution and overload resolution;
- member availability on built-in, named, related, generic, reference, array,
  slice, string, pointer, Arena, and target-specific types;
- representation-sensitive properties;
- compile-time type and layout queries;
- string and character materialization members;
- Semantic IR requirements;
- Sec MLIR lowering requirements;
- effects, ownership, borrowing, allocation, and unsafe behavior;
- diagnostics;
- formatter and LSP behavior;
- implementation governance and tests.

**§ 1(3)** All normative compiler-known-member semantics belong to this document.

**§ 1(4)** `core-library.md` owns ordinary core APIs and helper implementations.

**§ 1(5)** `stdlib.md` owns optional higher-level library APIs.

**§ 1(6)** `properties.md` owns ordinary user-defined property syntax and behavior.

**§ 1(7)** `rules/declarations/impl.md` owns ordinary user-defined implementation blocks.

**§ 1(8)** `types.md`, `collections.md`, `reference_model.md`, `unsafe.md`,
`allocation.md`, `arena.md`, `layout.md`, `effect_analysis.md`, and
`semantic_ir.md` own their respective semantic domains.

**§ 1(9)** This document consumes those rules and determines when the compiler must know a
member independently of an ordinary source declaration.

---

## § 2 Purpose

**§ 2(1)** Some operations cannot be resolved or implemented correctly as ordinary Sec
source alone.

**§ 2(2)** They require compiler knowledge of one or more of:

```text
exact built-in type identity;
physical representation;
addressability;
storage origin;
reference provenance;
array extent;
slice bounds;
string representation;
target layout;
alignment;
ABI;
volatile behavior;
allocation context;
temporary lifetime;
ownership transfer;
Semantic IR operation kind;
target-specific lowering.
```

**§ 2(3)** Examples include:

```sec
value.Ptr
value.Len
len(value)
value.SizeOf
int32.SizeOf
value.ToString()
"test string".ToByteArray()
"test string".ToCharArray()
"test string".ToRuneArray()
myCharArray[start..<end].ToString()
```

**§ 2(4)** The compiler must know that these operations exist and must know their canonical
semantics.

**§ 2(5)** Their physical implementation may still use:

```text
inline lowering;
a Sec MLIR operation;
a core helper;
a target helper;
a platform helper;
a privileged core impl;
ordinary user code where overriding is permitted.
```

---

## § 3 Core principle

```text
Compiler-known describes semantic authority.

It does not require every operation to lower to one machine instruction.

It does not require every operation to be absent from core source.

It does not permit the compiler to bypass ordinary ownership, effect, lifetime,
allocation, or unsafe rules.
```

---

## § 4 Normative ownership

**§ 4(1)** A compiler-known member has exactly one normative semantic owner.

**§ 4(2)** This document owns:

```text
existence;
canonical spelling;
eligible receiver or target types;
lookup category;
whether overriding is permitted;
required semantic facts;
required IR representation;
required diagnostics.
```

**§ 4(3)** Another rulebook may own detailed domain semantics.

**§ 4(4)** Examples:

```text
Ptr:
    this document owns availability and resolution
    reference_model.md and unsafe.md own pointer validity and unsafe obligations

Len:
    this document owns availability and result type
    collections.md owns array and slice length semantics
    string rules own string byte-length semantics

SizeOf:
    this document owns the member
    layout.md owns physical size

ToRuneArray:
    this document owns required availability and conversion category
    string and allocation rules own representation and allocation behavior
```

**§ 4(5)** Other documents must refer to this rulebook rather than create a second
compiler-known-member registry.

---

## § 5 Compiler-known categories

**§ 5(1)** A member registry entry has one of these semantic categories.

**§ 5(2)** The registry's canonical authority categories are:

```text
CompilerProvidedFallbackMember
AuthoritativeCompilerSemanticProperty
CompilerKnownOperation
PrivilegedCoreMember
OrdinaryUserMember
```

**§ 5(3)** These are compiler-semantic categories, not Sec enum declarations. A
compiler-known Sec declaration must be specified in complete canonical syntax
(module, name, generic parameters, variants/members, parameter and result/error
types, modifiers, visibility, availability, and target dependence as
applicable), or explicitly cross-reference the owner that does so. A
compiler-only request, diagnostic, dependency record, or IR structure must
instead be marked as not a Sec declaration and fully specified as compiler
semantics.

**§ 5(4)** Compiler-facing versus user-facing is an authority boundary independent of
source spelling. Real `_` and `__` declarations retain the ordinary Sec
visibility rules; compiler metadata neither needs those spellings nor gains
source visibility from them.

### § 5.1 Intrinsic property

**§ 5.1(1)** A property whose value is derived directly from compiler-known representation or
semantic metadata.

**§ 5.1(2)** Examples:

```text
Ptr
Len
numeric Min
numeric Max
numeric Bits
float Epsilon
float Infinity
float NaN
decimal Scale
```

### § 5.2 Intrinsic operation

**§ 5.2(1)** A method whose semantics require explicit compiler representation.

**§ 5.2(2)** Examples:

```text
SizeOf
RawPtr.Read()
RawPtr.Write()
RawPtr.Offset()
Arena.Reset()
Arena.Release()
```

### § 5.3 Compiler-known core method

**§ 5.3(1)** A method whose existence and contract are known by the compiler, but whose
ordinary implementation may be supplied by privileged core source.

**§ 5.3(2)** Examples:

```text
ToString()
ToByteArray()
ToCharArray()
ToRuneArray()
Fill(value)
```

**§ 5.3(3)** Only methods explicitly listed as compiler-known belong to this category.

**§ 5.3(4)** An ordinary core method does not become compiler-known merely because it is
implemented in core.

### § 5.4 Compiler-known associated function

**§ 5.4(1)** An associated function available without ordinary user imports and attached to
a compiler-known type.

**§ 5.4(2)** Examples:

```text
string.FromRuneArray(...)
Arena.FromBuffer(...)
Arena.WithCapacity(...)
```

### § 5.5 Compiler-known global function

**§ 5.5(1)** A compiler-owned function resolved without an import.

**§ 5.5(2)** Required in Sec 0.1:

```sec
len(value)
```

### § 5.6 Internal intrinsic operation

**§ 5.6(1)** A compiler operation with no required public source spelling.

**§ 5.6(2)** Examples may include:

```text
AlignOf(T);
materialize addressable storage;
construct a slice descriptor;
extract an ArenaDomain;
validate generation;
obtain target ABI classification.
```

**§ 5.6(3)** Internal intrinsic operations are not automatically callable by user code.

### § 5.7 Privileged internal core helper

**§ 5.7(1)** A representation-sensitive implementation helper may instead be an ordinary
private core function whose name begins with `__`.

**§ 5.7(2)** Such a helper may contain a small direct MLIR/LLVM implementation when the
supported privileged-core syntax and contracts permit it. The need for
low-level implementation does not by itself make the helper compiler-known.

**§ 5.7(3)** Use compiler-known identity only when the compiler must own the operation's
semantic identity, availability, resolution, analysis facts, or lowering
contract independently of the core source declaration.

**§ 5.7(4)** An internal core helper remains subject to its declared ownership, lifetime,
effect, unsafe, and target contracts. Direct low-level implementation does not
bypass those contracts.

---

## § 6 Compiler-known does not mean globally visible syntax

**§ 6(1)** The parser uses ordinary syntax for public compiler-known members.

**§ 6(2)** Examples:

```sec
value.Ptr
value.Len
value.SizeOf
string.FromRuneArray(runes)
```

**§ 6(3)** The parser must not introduce special AST node kinds based only on the spelling.

**§ 6(4)** The parser creates ordinary:

```text
member access;
member call;
associated member access;
associated call;
global call.
```

**§ 6(5)** Semantic analysis resolves the expression to a compiler-known registry entry.

**§ 6(6)** Semantic IR then records the intrinsic semantic operation explicitly where
required.

---

## § 7 Registry

**§ 7(1)** The compiler must maintain one canonical compiler-known-member registry.

**§ 7(2)** The registry must not be distributed as unrelated string comparisons across
Sema, LSP, lowering, and backend code.

**§ 7(3)** A conceptual entry contains:

```go
type CompilerKnownMember struct {
    ID                 CompilerKnownMemberID
    CanonicalName      string
    LegacyNames        []string
    Kind               CompilerKnownMemberKind
    ReceiverPattern    TypePattern
    Parameters         []ParameterPattern
    Result             ResultPattern
    IsUnsafe           bool
    IsOverridable      bool
    IsInheritable      bool
    RequiresAddress    bool
    RequiresLayout     bool
    RequiresAllocation bool
    SemanticOperation  SemanticOperationKind
}
```

**§ 7(4)** This structure is illustrative.

**§ 7(5)** All implementation code is written in English.

**§ 7(6)** The real registry may use generated tables, typed declarations, or another
stable representation.

---

## § 8 Stable member identity

**§ 8(1)** Every compiler-known member has a stable semantic ID.

**§ 8(2)** The ID is not derived solely from its display name.

**§ 8(3)** The ID must distinguish overloads and receiver categories.

**§ 8(4)** Examples:

```text
CKM-PTR-VALUE
CKM-LEN-STRING
CKM-LEN-ARRAY
CKM-LEN-SLICE
CKF-LEN
CKM-SIZEOF-VALUE
CKM-SIZEOF-TYPE
CKM-TOSTRING-STRING
CKM-TOSTRING-BYTE-SEQUENCE
CKM-TOSTRING-CHAR-SEQUENCE
CKM-TOSTRING-RUNE-SEQUENCE
CKM-STRING-TOBYTEARRAY
CKM-STRING-TOCHARARRAY
CKM-STRING-TORUNEARRAY
```

**§ 8(5)** Exact numbering belongs to the compiler registry.

---

## § 9 Source naming convention

**§ 9(1)** Sec public methods and public compiler-known properties use the canonical member
casing selected by this rulebook.

**§ 9(2)** For the members defined here:

```text
Ptr
Len
IsEmpty
Capacity on list, map and set
Reserve on dynamic list, map and set
WithCapacity on dynamic list, map and set
SizeOf
Fill
ToString
ToByteArray
ToCharArray
ToRuneArray
```

**§ 9(3)** The global function remains:

```text
len
```

**§ 9(4)** There is no public `Length` alias in Sec 0.1.

**§ 9(5)** Rationale:

```text
Ptr and Len form one compact low-level naming pair;
Length would duplicate Len;
duplicate aliases increase language, formatter, LSP, test, and documentation
surface;
the global len function has a distinct signed-index purpose and existing
bootstrap usage.
```

---

## § 10 Legacy lowercase member spellings

**§ 10(1)** The repository currently contains lowercase intrinsic spellings:

```text
.ptr
.len
.scale
```

**§ 10(2)** The canonical Sec 0.1 public spellings are:

```text
.Ptr
.Len
.Scale
```

**§ 10(3)** `.scale` / `.Scale` is the `int32` instance property of a `decimal` value,
with an infallible getter and a fallible `try set` reporting `PrecisionError`
(`core-library.md` § 10.1; MD-082, 2026-10-18; MD-066, 2026-10-28, which
replaces the former `uint8` type). Both spellings resolve to the one identity
`CKM-DECIMAL-SCALE`. `decimal128` has the same property with its own single
identity (MD-068). New examples, generated declarations and hover use
`.Scale`.

**§ 10(4)** The read-only value predicates of `core-library.md` §§ 11.2 and 10.4
(MD-068 § 9(2)) are compiler-known properties of the mathematical value,
independent of the representation scale, with one identity per type:
`IsInteger`, `IsZero`, `IsPositive` and `IsNegative` of type `bool` and
`Sign` of type `int` are `CKM-DECIMAL-ISINTEGER`, `CKM-DECIMAL-ISZERO`,
`CKM-DECIMAL-ISPOSITIVE`, `CKM-DECIMAL-ISNEGATIVE` and `CKM-DECIMAL-SIGN`
on `decimal` and the corresponding `CKM-DECIMAL128-*` identities on
`decimal128`. They have no setters and no legacy spellings.

**§ 10(5)** The decimal core methods of `core-library.md` §§ 11.2 and 10.4 (MD-066 § 9,
MD-068 § 9(2)) are compiler-known methods with one identity per type,
`CKM-DECIMAL-<NAME>` and `CKM-DECIMAL128-<NAME>`: `Normalize()`, `Floor()`,
`Ceiling()`, `Truncate()` and `Round()` return the receiver type;
`Min(other)` and `Max(other)` take and return the receiver type;
`Abs()` returns `Result[T, OverflowError]`; `Clamp(minimum, maximum)`
returns `Result[T, RangeError]`; `Rescale(scale: int32)` and
`RoundTo(scale: int32[, mode: RoundingMode])` return
`Result[T, PrecisionError]`. The conversions of `core-library.md` § 10.6
(MD-068 § 10) are compiler-known methods with the same identity scheme:
`ToInt()`, `ToInt32()`, `ToInt64()`, `ToInt128()`, `ToFloat()`,
`ToFloat32()` and `ToFloat64()`, each with an optional `mode: RoundingMode`
argument and the result `Result[target, ConversionError]`. Calls check arity
and argument types; the error types resolve to the loaded core declarations.

**§ 10(6)** Lowercase spellings are classified as migration spellings.

**§ 10(7)** They are not separate semantic members.

**§ 10(8)** The compiler may temporarily accept them during bootstrap migration.

**§ 10(9)** When accepted, they resolve to the same stable member ID as the canonical
spelling.

**§ 10(10)** A compatibility diagnostic should state:

```text
information: lowercase compiler-known member spelling is deprecated

use:
    value.Ptr
```

**§ 10(11)** or:

```text
use:
    value.Len
```

**§ 10(12)** The formatter may rewrite legacy lowercase spellings only when compatibility
mode is enabled and semantic resolution is unambiguous.

**§ 10(13)** The final `docs-v0.1.0` normative examples must use canonical spellings.

---

## § 11 Reserved member names

**§ 11(1)** The following universal compiler-known member names cannot be redeclared with a
different meaning on a type where the universal member applies:

```text
Ptr
SizeOf
```

**§ 11(2)** `Len`, `IsEmpty`, and `Fill` are reserved on the compiler-known collection
receiver categories where their canonical properties or operation apply.

**§ 11(3)** The global function name `len` is compiler-owned and cannot be redeclared or
overloaded by user code.

**§ 11(4)** `ToString` is different:

```text
category: CompilerProvidedFallbackMember
canonical shape: fn ToString() Result[string, StringError]
exact user replacement: permitted on eligible user-owned nominal types
```

**§ 11(5)** An exact user-owned `ToString() Result[string, StringError]` replaces the
fallback without becoming an ambiguous overload. A differently shaped overload does not replace it.
`SizeOf`, by contrast, is an `AuthoritativeCompilerSemanticProperty`; user
code cannot override it where the canonical property applies.

**§ 11(6)** String conversion-array methods are reserved on built-in `string`.

**§ 11(7)** **Register hardware operations (MD-077, MD-113, 2026-10-30; `rules/corrections/applied/MD-113-shadow-knowledge-across-calls-decision-20261030.md` § 10).** On a `register[N]` type the names `Read`, `Write` and `IsShadowKnown` are reserved in the combined `impl` surface (`rules/declarations/registers.md` § 12.7). They are registry members with stable identities, reachable only inside the owning `impl T` on a `HardwareInstance[T]` receiver and never overridable or inheritable:

| ID | Source form | Result | Hardware effect | Software effect |
|---|---|---|---|---|
| `CKM-REGISTER-READ` | `self.Read()` | detached `T` | one requested observation, with documented read side effects | establishes observed knowledge per the observation's postconditions |
| `CKM-REGISTER-WRITE` | `self.Write()` | `void` | commits pending intents; no pending intent means no I/O | updates knowledge per the commit postconditions |
| `CKM-REGISTER-WRITE-VALUE` | `self.Write(value)` | `void` | explicit whole-register write under a verified plan | per its postconditions |
| `CKM-REGISTER-IS-SHADOW-KNOWN` | `self.IsShadowKnown(T.Field[.Nested...])` | `bool` | never | reads compiler-managed per-instance shadow availability only |

**§ 11(8)** `IsShadowKnown` has category `CompilerKnownOperation` with one static `RegisterFieldSelector` operand rooted in `T`; it is not unsafe, needs no allocation and maps to the Semantic IR operation `register.hw.shadow_known` (`rules/compiler/semantic_ir.md` § 22(9)). The selector is resolved by Sema to the canonical `RegisterTypeID` and complete `RegisterFieldID` path before IR construction and has no public type. Sema, LSP completion, hover, definition, reserved-name diagnostics and IR all consume these registry entries; no second LSP-owned intrinsic namespace or string comparison may define them.

**§ 11(9)** User-defined types may use the same method names for their own ordinary
semantics unless another interface contract restricts them.

---

## § 12 Lookup order

**§ 12(1)** Member lookup follows this conceptual order:

```text
1. exact user-defined member on the nominal type, when permitted;
2. exact compiler-known member for the exact semantic type;
3. exact privileged core member for the exact built-in type;
4. eligible compiler/core member inherited from a related underlying type;
5. interface or generic-constraint member;
6. no member.
```

**§ 12(2)** This order must not allow a user module to override representation-sensitive
members on compiler-owned built-in types.

**§ 12(3)** Ordinary user modules may not add `impl` blocks to compiler-owned built-in
types.

**§ 12(4)** Privileged core may do so through the canonical core loading mechanism.

---

## § 13 Built-in type member lookup

**§ 13(1)** Compiler-owned built-in types participate in ordinary member lookup.

**§ 13(2)** The compiler must expose registry entries through the same semantic member model
used by:

```text
completion;
hover;
go to definition;
overload resolution;
generic constraints;
interface conformance;
diagnostics.
```

**§ 13(3)** LSP must not maintain a separate hard-coded list.

---

## § 14 Named and related types

**§ 14(1)** A named type remains nominally distinct from its underlying type.

**§ 14(2)** A compiler-known or core member may be inherited only when:

```text
the operation remains representation-compatible;
nominal identity is not erased;
contracts are not bypassed;
unit semantics are not bypassed;
no implicit conversion is introduced;
parameter and result substitution remains valid;
unsafe obligations remain unchanged.
```

**§ 14(3)** Examples that are normally eligible:

```sec
type CustomerID uint64
let text := customerID.ToString()
let bytes := customerID.SizeOf
```

**§ 14(4)** Examples that require special care:

```text
Ptr:
    may be available when the named value is addressable
    result is RawPtr[CustomerID], not RawPtr[uint64]

SizeOf:
    uses the named type layout
    normally equal to the underlying representation size

Len:
    available only when the named type remains a sequence type under canonical
    named-type rules

ToString:
    explicit nominal implementation takes precedence
```

**§ 14(5)** A `distinct` type may restrict inherited behavior according to the distinct-type
rulebook.

---

## § 15 Generic lookup

**§ 15(1)** Compiler-known members participate in generic checking only when the generic
contract guarantees their availability.

**§ 15(2)** The compiler must not assume that every unconstrained `T` has:

```text
Len;
Ptr.
```

**§ 15(3)** `ToString()` is the exception (MD-122, decided 2026-11-01): every generic parameter,
pack element and interface value holds an ordinary value, so the fallback exists for
every concrete argument and the template may call it. It returns
`Result[string, StringError]`. The call resolves per specialization to an exact user
`ToString`, or else to the fallback of the concrete type; a constraint's `ToString`
requirement takes precedence. Because the concrete type is unknown in the template,
the call counts as an unknown callee, so `@noPanic` and `@noAlloc` cannot rest on it.
This grants no other member and no implicit formatting interface. Interpolation
holes keep their own formatting contract rule.

**§ 15(4)** `SizeOf` is available only when `T` is statically sized for the active
CompilationPlan.

**§ 15(5)** `Ptr` additionally requires an addressable value expression.

**§ 15(6)** A future interface may express printable or sequence behavior.

**§ 15(7)** This rulebook does not invent that interface syntax.

---

## § 16 Effects and compiler-known members

**§ 16(1)** Compiler-known members remain subject to effect analysis.

**§ 16(2)** A compiler-known member may be:

```text
effect-free;
MayAllocate;
MayPanic;
MayAccessVolatile;
MayMutateExternalState;
unsafe;
ordered through Arena effects.
```

**§ 16(3)** The compiler must not classify an operation as pure merely because it is
intrinsic.

**§ 16(4)** Examples:

```text
Len:
    no allocation
    no mutation
    no consumption

SizeOf:
    no allocation
    result derived from layout
    receiver-expression effects remain when value syntax is used

Ptr:
    unsafe
    no ownership transfer
    may expose volatile address semantics

ToByteArray:
    materializing
    may allocate
    requires allocation context where defined

Arena.Alloc:
    MayAllocate
    ArenaAllocate

_now:
    MayUseNondeterministicInput
    requires UTCWallClock
```

---

## § 17 Semantic IR representation

**§ 17(1)** A compiler-known operation receives an explicit Semantic IR operation or stable
intrinsic-call identity when its semantics matter after Sema.

**§ 17(2)** The compiler must not lower all compiler-known members as generic unresolved
calls.

**§ 17(3)** Required explicit distinctions include:

```text
GetPointer
GetLength
SizeOfType
SizeOfValue
StringIdentity
StringToByteArray
StringToCharArray
StringToRuneArray
ByteSequenceToString
RuneSequenceToString
CharSequenceToString
UTCWallClockRead
Arena operations
RawPtr operations
```

**§ 17(4)** Exact operation names are implementation-defined.

**§ 17(5)** The semantic distinctions are required.

---

## § 18 Sec MLIR path

**§ 18(1)** The intended lowering path is:

```text
Sec source
    ↓
ordinary AST member expression
    ↓
resolved compiler-known member
    ↓
explicit Sec Semantic IR operation
    ↓
high-level Sec MLIR operation
    ↓
standard MLIR dialects
    ↓
LLVM dialect and target code
```

**§ 18(2)** A compiler-known operation may lower to a core/helper call when that preserves
the semantic operation and all analysis facts.

---

## § 19 Compiler-known helper calls

**§ 19(1)** A helper implementation does not own semantics.

**§ 19(2)** Example:

```text
semantic operation:
    StringToRuneArray

selected implementation:
    core string decoder helper
```

**§ 19(3)** The call graph records the helper when it is a real callable.

**§ 19(4)** Effect analysis includes helper effects.

**§ 19(5)** Inlining or replacing the helper does not change the source member contract.

---

## § 20 Internal core string-slice helper

**§ 20(1)** The checked core string API may use the following private helper:

```sec
fn __StringSliceUnchecked(value: string, start: uint, end: uint) string
```

**§ 20(2)** This is an implementation detail of `core/string.sec`, not a required
compiler-known global function. Its leading `__` gives it source-file-private
visibility under `names_scopes_visibility.md`; it must not be exposed by
ordinary completion or public API discovery.

**§ 20(3)** The helper creates a `string` value for the byte interval `[start, end)` of
`value`. Its result is a string, not an array slice, safe reference, or
source-level borrow. Its low-level body may form a string descriptor over the
existing backing storage and must preserve the string storage and lifetime
rules.

**§ 20(4)** The helper performs no bounds validation, allocation, or string-content
validation. Its caller must establish:

```text
start <= end
end <= value.SizeOf
```

**§ 20(5)** Public core operations such as checked substring extraction own those checks
and their error results. Code outside the declaring core source file cannot use
the helper to bypass them.

**§ 20(6)** The intended representation-level operation is:

```text
result.data = value.data + start
result.len  = end - start
```

**§ 20(7)** This helper does not require a compiler-known registry entry or a
name-specific backend call-emission case merely because its implementation is
low-level. A compiler-known identity remains permitted only if separate
semantic or analysis requirements make compiler ownership necessary.

---

## § 21 `StringView` and string ranges (MD-126)

**§ 21(1)** Decided 2026-11-05 (`rules/corrections/applied/MD-126-borrowed-string-ranges-rune-iteration-applied-20261105.md`).

**§ 21(2)** `StringView` is a compiler-known type identity with a fixed public member set,
listed in full in `core-library.md` § 5.8: `Len`, `SizeOf`, `IsEmpty`,
`GetRune`, `RuneAt`, `GetByte`, `ByteAt`, `CharAt`, `StartsWith(StringView)`
and `ToString()`. Sema and the LSP must take these entries from the core
registry and must not fabricate others; no `string` member is inherited by
name.

**§ 21(3)** The string range `text[a..<b]` (and every other § 8.4 range form) over a
`string` or `StringView` source is a compiler-known operation identity, not a
member call. Its resolved facts are: source kind, normalized rune range,
result type `StringView`, source-backed borrow origin, no allocation, no
mutation, and the ordinary `panic.bounds`/`slice-range` or `try`
`RangeError` failure edge.

**§ 21(4)** Public `string.Slice(start, end)` is removed from the string member registry.
For a former `text.Slice(a, b)` call, diagnostics suggest `text[a..<b]` or
`try text[a..<b]` and explain that the result is borrowed; they do not
rewrite or materialize automatically.

**§ 21(5)** The private `__StringSliceUnchecked` helper above stays a core-internal
**owned `string` descriptor operation**. It is not the public borrow
constructor and keeps its own migration status.

---

## § 22 Private core UTC wall-clock intrinsic

**§ 22(1)** The compiler-known identifier `_now` is a privileged core-only value intrinsic
with semantic result type `datetime`. It is available only while compiling
loader-proven `sec/core`; it has no public declaration, is not importable, and
must not appear in ordinary lookup, completion, or public API documentation
outside trusted core.

**§ 22(2)** The same core-only exposure boundary applies to other compiler-provided
function or value spellings beginning with `_`: these are internal core
services, not public Sec APIs. An LSP synthetic-definition request by registry
ID alone must not expose such a service; navigation must originate from a
semantically authorized core use. Ordinary source declarations keep their
module-internal or private visibility under
`rules/foundations/names_scopes_visibility.md` §§12.2–12.3.

**§ 22(3)** Its stable registry contract is:

```text
canonical name: _now
availability: loader-proven sec/core only
result type: datetime
unsafe: false
effect: MayUseNondeterministicInput
required target capability: UTCWallClock
public visibility: none
```

**§ 22(4)** Each evaluation performs one UTC wall-clock read. The compiler preserves that
read as an explicit Semantic IR operation or stable intrinsic identity. It must
not constant-fold the read, substitute compiler-host time, build time, the Unix
epoch, or another sentinel, or merge distinct reads through common-subexpression
elimination.

**§ 22(5)** The intrinsic is invalid in every `SemanticCompileTimeRequiredContext`,
including static initializers. The public temporal API and projection semantics
are defined by `rules/types/temporal.md`.

---

## § 23 `Ptr`

### § 23.1 Canonical spelling

```sec
value.Ptr
```

**§ 23.1(1)** `Ptr` is an unsafe read-only compiler-known property.

**§ 23.1(2)** Conceptual form:

```sec
unsafe property Ptr: RawPtr[T] {
    get
}
```

**§ 23.1(3)** The exact result type depends on the receiver category.

---

## § 24 `Ptr` on addressable scalar values

**§ 24(1)** For an addressable value of exact semantic type `T`:

```sec
unsafe {
    let pointer := value.Ptr
}
```

**§ 24(2)** the result is:

```text
RawPtr[T]
```

**§ 24(3)** The pointer refers to the current storage representation of that value.

**§ 24(4)** The source expression must be addressable.

---

## § 25 `Ptr` on strings

**§ 25(1)** For `string`:

```sec
unsafe {
    let pointer := text.Ptr
}
```

**§ 25(2)** the result is:

```text
RawPtr[byte]
```

**§ 25(3)** It refers to the first encoded storage byte of the string view or representation
selected by the active CompilationPlan.

**§ 25(4)** `text.SizeOf` is the number of accessible encoded bytes.
`text.Len` counts decoded Unicode scalar values.

**§ 25(5)** `Ptr` does not imply a trailing zero byte.

**§ 25(6)** A foreign API requiring a terminator needs an explicit materializing operation.

**§ 25(7)** The source string remains immutable.

**§ 25(8)** Obtaining `RawPtr[byte]` does not grant semantic permission to modify immutable
string storage.

**§ 25(9)** An unsafe write through the pointer must satisfy all immutability, ownership,
representation, and FFI obligations.

---

## § 26 `Ptr` on arrays

**§ 26(1)** For an addressable array of `T`:

```sec
unsafe {
    let pointer := values.Ptr
}
```

**§ 26(2)** the result is:

```text
RawPtr[T]
```

**§ 26(3)** It refers to the first element storage when the array has at least one element.

**§ 26(4)** The array must have stable addressable storage.

**§ 26(5)** The pointer does not contain the array length.

**§ 26(6)** The same property is available on owning dynamic arrays `T[]` and `list[T]`.
It refers to their current contiguous element backing. It does not expose
capacity, transfer ownership, or stabilize storage across a mutating operation.

**§ 26(7)** `map[K, V]` and `set[T]` do not have compiler-known `Ptr`: their physical
storage is not a public contiguous sequence contract.

---

## § 27 `Ptr` on slices

**§ 27(1)** For a shared or mutable slice:

```sec
unsafe {
    let pointer := view.Ptr
}
```

**§ 27(2)** the result is:

```text
RawPtr[T]
```

**§ 27(3)** It refers to the first represented element when `view.Len > 0`.

**§ 27(4)** The pointer does not carry:

```text
bounds;
lifetime;
generation;
ownership;
mutability;
slice length.
```

**§ 27(5)** These remain properties of the safe slice and compiler analysis.

---

## § 28 Empty `Ptr`

**§ 28(1)** For an empty string, array, or slice:

```text
Len == 0;
Ptr must not be dereferenced;
the physical pointer value may be null, a sentinel, a base pointer, or another
target-valid representation.
```

**§ 28(2)** No source program may depend on one universal empty-pointer bit pattern.

**§ 28(3)** Pointer equality on empty views does not establish shared storage identity.

---

## § 29 `Ptr` addressability

**§ 29(1)** Normally addressable:

```text
local storage;
materialized parameters;
fields;
array elements with stable storage;
slice elements with stable storage;
static storage;
addressed storage;
compiler-approved string storage;
materialized owning arrays;
safe slice views.
```

**§ 29(2)** Normally not addressable:

```text
pure arithmetic temporaries;
values existing only as SSA values;
unmaterialized conversions;
unmaterialized function results;
compile-time-only values;
types without an address-space representation.
```

**§ 29(3)** The compiler must not silently allocate or extend a lifetime merely to satisfy
`Ptr`.

**§ 29(4)** A target may materialize a value only when ordinary language semantics already
require storage.

---

## § 30 `Ptr` unsafe obligations

**§ 30(1)** Accessing `Ptr` requires explicit unsafe context.

**§ 30(2)** `Ptr`:

```text
does not transfer ownership;
does not extend lifetime;
does not pin storage;
does not create a safe reference;
does not preserve slice bounds;
does not guarantee non-nullness for empty storage;
does not prevent relocation by the true owner;
does not keep an ArenaDomain alive;
does not retain a foreign-call buffer;
preserves volatile/address-space semantics where applicable.
```

**§ 30(3)** Using the pointer after the source storage becomes invalid is unsafe and
invalid.

---

## § 31 `Ptr` and Arena storage

**§ 31(1)** A pointer obtained from an Arena-backed value is valid only while:

```text
the ArenaDomain remains live;
the allocation epoch remains valid;
the allocation remains within bounds;
the storage is not invalidated by Reset or Release;
the FFI or unsafe operation obeys the retention contract.
```

**§ 31(2)** `RawPtr[T]` does not block Arena Reset or Release by itself.

**§ 31(3)** Code retaining the raw pointer must establish an explicit dependency through
the foreign/unsafe contract.

---

## § 32 `Ptr` and volatile storage

**§ 32(1)** When the source storage is volatile or addressed, pointer access preserves the
address-space and volatile obligations.

**§ 32(2)** The raw pointer does not convert volatile storage into ordinary memory.

**§ 32(3)** Loads and stores must lower according to the target's volatile rules.

---

## § 33 `Len`

### § 33.1 Canonical spelling

```sec
value.Len
```

**§ 33.1(1)** `Len` is a read-only compiler-known property.

**§ 33.1(2)** The result type is:

```text
uint
```

**§ 33.1(3)** There is no `Length` alias in Sec 0.1.

---

## § 34 `Len` and value-form `SizeOf` on strings

**§ 34(1)** For `string`, both are read-only properties returning `uint` (MD-058,
2026-10-12, `rules/corrections/applied/md058-collection-capacity-reservation-correction-20261012.md`):

```sec
let runeCount := text.Len
let byteCount := text.SizeOf
```

**§ 34(2)** `Len` is the number of decoded Unicode scalar values. It counts neither
grapheme clusters nor display columns and performs no normalization. The
value-form `SizeOf` is the encoded UTF-8 payload byte count. Embedded U+0000
counts as one rune and one byte; non-BMP scalars count as one rune and four
bytes. No trailing terminator, unused capacity, descriptor or allocation header
contributes to either count.

**§ 34(3)** For example, `"éΩ"` has `Len == 2` and `SizeOf == 4`, and `"Hej π"` has
`Len == 5` and `SizeOf == 6`. Pointer-based byte views, byte offsets and foreign
lengths use `SizeOf`. No public `ByteLen`, `RuneLen` or `CharLen` alias exists;
an internal helper for efficient scalar counting is not a public member.
Whether every `string` is valid UTF-8 remains open (MD-050).
The string contracts in `rules/types/contracts.md` use these same units.
The compatibility lowercase `.len` follows `.Len` and therefore counts runes.

---

## § 35 `Len` on fixed arrays

**§ 35(1)** For a fixed array `T[N]`:

```sec
let count := values.Len
```

**§ 35(2)** the result is the compile-time constant:

```text
N
```

**§ 35(3)** The result type is `uint`.

---

## § 36 `Len` on owning dynamic arrays

**§ 36(1)** For an owning dynamic array or sequence `T[]`:

```sec
let count := values.Len
```

**§ 36(2)** the result is the current element count.

**§ 36(3)** It is not the allocated capacity unless the collection rulebook explicitly
defines both values as equal.

**§ 36(4)** No implicit `Capacity` property is created by this rulebook.

---

## § 37 `Len` on slices

**§ 37(1)** For `ref T[]` and `ref mut T[]`:

```sec
let count := view.Len
```

**§ 37(2)** the result is the number of elements represented by the view.

**§ 37(3)** It is a runtime value unless statically proven.

**§ 37(4)** It is not the byte size.

**§ 37(5)** The represented byte payload may be computed from layout only when safe checked
multiplication is valid:

```text
view.Len * T.SizeOf
```

---

## § 38 `Len` on library collections

**§ 38(1)** For `list[T]`, `map[K, V]`, and `set[T]`, `Len` is the logical element or entry
count. It is never capacity or backing-storage size.

---

## § 39 `Len` evaluation

**§ 39(1)** `Len`:

```text
evaluates the receiver exactly once;
does not allocate;
does not consume the receiver;
does not mutate the receiver;
does not read sequence elements;
may be folded when statically known.
```

**§ 39(2)** For a volatile sequence descriptor, reading descriptor metadata follows the
volatile rulebook when the descriptor itself is volatile.

---

## § 40 `IsEmpty`

**§ 40(1)** `IsEmpty` is a read-only compiler-known property on fixed arrays, owning
dynamic arrays, slices, `list[T]`, `map[K, V]`, and `set[T]`:

```sec
if values.IsEmpty {
    return
}
```

**§ 40(2)** It is equivalent to `values.Len == 0`, evaluates the receiver exactly once,
and does not allocate, consume, or inspect elements. `IsEmpty()` is not a
second callable spelling.

---

## § 41 Global `len`

**§ 41(1)** Sec 0.1 retains the compiler-known global function:

```sec
len(value)
```

**§ 41(2)** Required conceptual overloads:

```sec
fn len(value: string) uint

fn len[T](value: T[]) uint

fn len[T](value: ref T[]) uint

fn len[T](value: ref mut T[]) uint
```

**§ 41(3)** The compiler infers `T`.

**§ 41(4)** `len` returns `uint` (MD-074/MD-088, 2026-10-17; `core-library.md` § 2.4).

**§ 41(5)** A length is a nonnegative count; signed offsets use an explicit checked
conversion.

**§ 41(6)** Example:

```sec
// position: uint
if position >= len(input) {
    return 0r
}
```

**§ 41(7)** `len(value)` and `value.Len` represent the same logical count for the same
value and have the same result type:

```text
value.Len:
    uint

len(value):
    uint
```

**§ 41(8)** No representability proof, saturation, wrapping, rejection or conversion to
`int` happens inside `len`; the exact unsigned value is preserved through
Semantic IR, Sec MLIR and LLVM. `len(values) - 1` on an empty sequence is
checked unsigned underflow, not a bounds failure.

---

## § 42 `len` restrictions

**§ 42(1)** `len`:

```text
evaluates its argument exactly once;
does not allocate;
does not consume;
does not mutate;
cannot be redeclared;
cannot be user-overloaded.
```

**§ 42(2)** It accepts only canonical compiler-known sequence categories.

**§ 42(3)** An arbitrary user type with a method named `Len` is not automatically accepted
by global `len`.

**§ 42(4)** A future interface-based overload requires a separate rule.

---

## § 43 `SizeOf`

**§ 43(1)** `SizeOf` is a compiler-known layout query.

**§ 43(2)** Required forms:

```sec
let valueSize := value.SizeOf
let typeSize := TypeName.SizeOf
```

**§ 43(3)** The result type is:

```text
uint
```

**§ 43(4)** The associated type property returns physical storage size for one value of the
type. The instance property follows the category rules below. There is no
canonical global `SizeOf(TypeName)` form in Sec 0.1.

**§ 43(5)** The registry shapes are equivalent to:

```sec
property SizeOf: uint {
    get
}

static property SizeOf: uint {
    get
}
```

**§ 43(6)** Both are authoritative compiler semantic properties and cannot be replaced by
user members on receiver categories where they apply.

**§ 43(7)** The semantic size comes from `layout.md`.

---

## § 44 Value-form `SizeOf`

**§ 44(1)** Example:

```sec
let size := myInt.SizeOf
```

**§ 44(2)** The size is determined from the exact semantic type of the receiver.

**§ 44(3)** The receiver expression follows ordinary evaluation rules and is evaluated
exactly once.

**§ 44(4)** Its stored value is not inspected to determine the result.

**§ 44(5)** Therefore:

```sec
let size := MakeValue().SizeOf
```

**§ 44(6)** still evaluates `MakeValue()` and preserves its effects.

**§ 44(7)** The compiler may eliminate receiver materialization only when normal
effect/ownership analysis permits it.

---

## § 45 Type-form `SizeOf`

**§ 45(1)** Example:

```sec
let size := int64.SizeOf
```

**§ 45(2)** The type form has no value receiver.

**§ 45(3)** It is a compile-time constant for one concrete CompilationPlan when the type has
complete layout.

**§ 45(4)** Generic type form:

```sec
let size := T.SizeOf
```

**§ 45(5)** is valid only when the generic contract guarantees that `T` is sized and has
complete layout at specialization.

---

## § 46 `SizeOf` meaning by category

### § 46.1 Scalar value

**§ 46.1(1)** Returns the physical storage size of the scalar type.

### § 46.2 Struct, enum, union, register, and named type

**§ 46.2(1)** Returns the complete physical size including layout-required padding.

**§ 46.2(2)** It uses the exact nominal type layout.

### § 46.3 Fixed array

**§ 46.3(1)** Returns the contiguous element payload size, including layout-required element
stride.

### § 46.4 Owning dynamic array

**§ 46.4(1)** Returns `value.Len * T.SizeOf`: the initialized element payload bytes. It does
not return descriptor size or reserved capacity.

### § 46.5 Slice

**§ 46.5(1)** Returns `value.Len * T.SizeOf`: the represented payload bytes. It does not
return the slice/reference descriptor size.

### § 46.6 List

**§ 46.6(1)** Returns `value.Len * T.SizeOf`: the initialized contiguous payload bytes. It
does not include capacity, allocation headers, or the owner descriptor.

### § 46.7 String

**§ 46.7(1)** The value form `text.SizeOf` returns the encoded UTF-8 payload bytes of the
string value (MD-058); it does not return `string.Len` and does not include a
terminator, unused capacity, descriptor or allocation header. The type form
`string.SizeOf` remains the physical layout size of a string value descriptor.

### § 46.8 Map and set

**§ 46.8(1)** There is no general payload `SizeOf` contract for `map` or `set` values; their
`Capacity` (`collections.md` §§ 14.3, 15.2) is no byte size.

### § 46.9 Safe reference

**§ 46.9(1)** Returns the physical representation size selected by the active profile.

**§ 46.9(2)** The logical reference model remains unchanged.

### § 46.10 `RawPtr[T]`

**§ 46.10(1)** Returns the target raw-pointer representation size.

### § 46.11 Arena

**§ 46.11(1)** Returns the physical Arena owner/descriptor size for the concrete
CompilationPlan after layout selection.

**§ 46.11(2)** It does not include owned backing capacity.

---

## § 47 Unsized and incomplete types

**§ 47(1)** `SizeOf` is invalid when:

```text
the type is unsized;
layout is incomplete;
layout depends on unresolved target selection;
the type has no physical runtime representation;
the value exists only at compile time.
```

**§ 47(2)** The diagnostic must identify the missing layout fact.

**§ 47(3)** `void.SizeOf` is invalid unless a later layout rule explicitly assigns a
representation.

---

## § 48 `SizeOf` and target plans

**§ 48(1)** `SizeOf` is evaluated per concrete `CompilationPlan`.

**§ 48(2)** The same source type may have different size on:

```text
linux/amd64;
linux/arm32;
bare-metal/cortex-m4;
another ABI/profile.
```

**§ 48(3)** A multi-output build evaluates and verifies the expression separately for every
output.

**§ 48(4)** Cross-plan tooling may report differing values.

**§ 48(5)** It must not merge them into one runtime value.

---

## § 49 `SizeOf` effects

**§ 49(1)** The layout result itself:

```text
does not allocate;
does not mutate;
does not consume;
does not access the stored value;
does not require unsafe.
```

**§ 49(2)** Value-form receiver evaluation may still carry effects.

**§ 49(3)** Type-form has no receiver effects.

---

## § 50 Contextual `fill`

**§ 50(1)** `fill` is a compiler-known contextual construction form, not an ordinary
globally overloadable function. Its expected destination type selects one of
these forms:

```sec
let fixed: T[N] := fill(value)
let owned: T[] := try fill(value, count)
let text: string := try fill(fragment, count)
```

**§ 50(2)** The fixed-array form initializes exactly `N` elements. The owning-array form
allocates and initializes exactly `count` elements and may fail according to
the active allocation context. The string form produces `count` repetitions of
the supplied fragment according to the string encoding rules. The value
expression is evaluated once and each stored element is copied from that value,
so element type `T` must be copyable unless a later rule defines a distinct
factory form.

**§ 50(3)** A mutable slice also has the compiler-known operation:

```sec
view.Fill(value)
```

**§ 50(4)** It overwrites every represented element in index order. `T` must be copyable;
the receiver must be `ref mut T[]`; the operation neither reallocates nor
changes `Len`. Shared slices have no `Fill`.

**§ 50(5)** The registry must give contextual `fill` and mutable-slice `Fill` stable,
distinct semantic identities so Sema, LSP, Semantic IR, and lowering do not
infer them from spelling alone.

---

## § 51 `AlignOf`

**§ 51(1)** The compiler requires an internal layout query equivalent to:

```text
AlignOf(T)
```

**§ 51(2)** for allocation, ABI, fields, arrays, and Arena lowering.

**§ 51(3)** Sec 0.1 does not require a public `AlignOf()` member.

**§ 51(4)** A public member may be added only through an explicit rulebook update.

**§ 51(5)** Internal `AlignOf` must not be exposed accidentally through LSP completion.

---

## § 52 `ToString()`

**§ 52(1)** `ToString()` is the universal compiler-provided fallback member on every Sec
type that can produce an ordinary value:

```sec
fn ToString() Result[string, StringError]
```

### § 52.1 Uniform `ToString` result

**§ 52.1(1)** Decision 2026-10-03 (resolves MD-035): every `ToString` member returns exactly
`Result[string, StringError]`. This covers:

```text
the compiler-provided fallback;
privileged core declarations;
the exact user-owned replacement;
every user-declared or core-declared overload, such as ToString(format);
every interface requirement named ToString.
```

**§ 52.1(2)** Materializing text may fail to allocate, and a format overload may receive an
invalid format; both failures use the one text failure family `StringError`
(`rules/library/core-library.md` § 15.1) through its `Allocation` and `Format`
variants, so every `ToString`, with or without arguments, has the same result
type. A failure is never hidden behind fallback text. Because users may implement their own `ToString`, the contract
is enforced on the declaration: a declared `ToString` with any other result type
is rejected with the mandatory diagnostic `S1100`
(`members.tostring-signature`) at its declared result. Callers handle the
result like any other `Result`, normally with `try`. A `ToString` that cannot
fail, such as `string.ToString()`, still returns `Ok(value)` through the same
result type.

**§ 52.1(3)** Each concrete type family must define or cross-reference its canonical fallback
formatting semantics before that family's support is implementation-complete.

**§ 52.1(4)** The compiler knows:

```text
whether the member exists;
the selected overload;
the result type;
whether the operation is identity, scalar encoding, numeric formatting, or
sequence materialization;
effects;
allocation-context requirement;
Semantic IR operation kind.
```

**§ 52.1(5)** Core may provide helper implementations.

**§ 52.1(6)** Target code may provide optimized helpers.

---

## § 53 Required built-in `ToString()` surface

**§ 53(1)** Sec 0.1 requires no-argument `ToString()` on:

```text
every concrete type that can produce an ordinary value.
```

**§ 53(2)** This is a compiler-provided fallback surface, not a claim that every type has
the same formatting behavior. Scalar, sequence, collection, and aggregate
families retain their distinct canonical representations. Generic parameters,
pack elements and interface values have the fallback too ("Generic lookup").

**§ 53(3)** Numeric formatting overloads may also include:

```sec
value.ToString(format)
```

**§ 53(4)** Such an overload also returns `Result[string, StringError]`. The exact
format grammar belongs to the formatting rulebook.

---

## § 54 `string.ToString()`

**§ 54(1)** For `string`:

```sec
let result := try text.ToString()
```

**§ 54(2)** the result is `Ok` with the same semantic string value; it never fails.

**§ 54(3)** It:

```text
does not allocate;
does not copy encoded bytes;
does not consume the string;
does not mutate the string;
preserves storage identity where the string model exposes it;
is included for generic formatting consistency.
```

**§ 54(4)** The compiler may fold the operation to identity.

---

## § 55 Boolean `ToString()`

**§ 55(1)** `bool.ToString()` produces the canonical text representation defined by core.

**§ 55(2)** The canonical English forms for Sec 0.1 are:

```text
true
false
```

**§ 55(3)** The operation is culture-independent.

**§ 55(4)** Locale-aware formatting belongs to a higher library.

---

## § 56 Numeric `ToString()`

**§ 56(1)** No-argument numeric `ToString()` produces the canonical culture-independent Sec
representation of the value.

**§ 56(2)** The representation must:

```text
round-trip through the matching parser where the numeric grammar supports it;
not depend on process locale;
not insert grouping separators;
use canonical sign and exponent syntax;
preserve enough precision for the type's canonical round-trip rule.
```

**§ 56(3)** Exact floating-point and decimal formatting algorithms belong to the formatting
and numeric rulebooks.

**§ 56(4)** The operation may lower to core helpers.

**§ 56(5)** The explicit conversion `string(numericValue)` resolves to this same operation
(MD-051, 2026-10-17): identical default formatting and the identical result
`Result[string, StringError]`, with no format argument and no implicit
application (`types.md`, "Numeric-to-text conversion"). It is not the
array/slice-to-string constructor cast rejected below.

**§ 56(6)** This relation targets primitive `string` only. Named string types do not
inherit it (MD-093): `Name(number)` is unavailable. Prefer the explicit sequence
`try Name(number.ToString())`, with `StringError` for materialization and any
independent `ConversionError.Contract` validation under the ordinary `try`
boundary (`types.md`, "Numeric-to-text conversion").

---

## § 57 `char.ToString()` and `rune.ToString()`

**§ 57(1)** `char.ToString()` materializes the canonical string representation of that
`char`.

**§ 57(2)** `rune.ToString()` encodes exactly one valid Unicode scalar value into a string.

**§ 57(3)** A `rune` is already guaranteed to be a valid Unicode scalar under the type
rules.

**§ 57(4)** The operations:

```text
do not consume the scalar;
do not mutate the scalar;
may require string materialization;
use the active allocation/string-storage policy;
must preserve the exact scalar value.
```

**§ 57(5)** The canonical source surface is the no-argument form returning
`Result[string, StringError]`.

**§ 57(6)** A CompilationPlan that cannot provide the required materialization strategy must
reject the operation rather than silently select an unrelated heap.

---

## § 58 User-defined `ToString()`

**§ 58(1)** A user-defined nominal type may define:

```sec
impl Customer {
    fn ToString() Result[string, StringError] {
        // ...
    }
}
```

**§ 58(2)** The method is an ordinary user implementation.

**§ 58(3)** It is not converted into an intrinsic merely because its name is `ToString`.

**§ 58(4)** The explicit nominal implementation takes precedence over inherited
underlying-type `ToString()`.

**§ 58(5)** The compiler still records its callable effects normally.

**§ 58(6)** An interface may later require `ToString()`.

**§ 58(7)** That interface does not change the compiler-known built-in registry.

---

## § 59 Byte, char, and rune sequence `ToString()`

**§ 59(1)** Arrays and slices whose element type is exactly `byte` provide:

```sec
value.ToString()
```

**§ 59(2)** Arrays and slices whose element type is exactly `rune` provide:

```sec
value.ToString()
```

**§ 59(3)** Arrays and slices whose element type is exactly `char` also provide:

```sec
value.ToString()
```

**§ 59(4)** Examples:

```sec
let runes: rune[2] := ['A', 'B']
let first := runes.ToString()

let chars := try "test string".ToCharArray()
let second := chars[0..<4].ToString()
```

**§ 59(5)** For these three text-element families, the operation:

```text
uses only the represented elements;
does not search for a terminator;
does not consume or mutate the source;
materializes one string;
preserves element order;
does not exist as a general array formatting operation.
```

**§ 59(6)** For a byte sequence the text-materialization list above does **not** apply:
`ToString()` never interprets byte contents as text (MD-091, 2026-10-18). It is
the general representational operation. For the dynamic `byte[]` its result is
the type summary `byte[]`, whether the bytes look like valid UTF-8 or not:

```sec
// The content is not UTF-8-decoded by ToString().
let description := try bytes.ToString() // "byte[]"
```

**§ 59(7)** Fixed byte arrays and borrowed byte views keep their collection/fallback
formatting contracts and never decode merely because their element type is
`byte`; no `ToString()` overload advertises itself as a byte-to-text decoder.
The signature stays the universal `Result[string, StringError]`. Decoding
bytes is the standard-library operation `str.FromByteArray` /
`str.FromByteArrayLossy` (`core-library.md` § 5.2). Char and rune sequence
semantics are unchanged.

---

## § 60 Char-sequence semantics

**§ 60(1)** `char` sequence conversion follows the canonical `char` model in `types.md`
and the string rulebook.

**§ 60(2)** Each `char` is converted according to the same semantics as
`char.ToString()`.

**§ 60(3)** The operation is not implicitly defined as:

```text
UTF-8 byte reinterpretation;
UTF-16 code-unit reinterpretation;
grapheme segmentation;
zero-terminated character scanning.
```

**§ 60(4)** This rulebook requires consistency between:

```text
char.ToString();
char-array/slice ToString();
string char iteration where defined.
```

**§ 60(5)** `string.ToCharArray()` is not part of that group: it is the byte-preserving
counterpart of `ToByteArray()` (MD-073), and char-sequence `ToString()` does not
reinterpret a `char` sequence as UTF-8 because of where it came from.

---

## § 61 Rune-sequence semantics

**§ 61(1)** Each `rune` is a Unicode scalar value.

**§ 61(2)** Rune array/slice `ToString()` encodes the represented scalar sequence in order.

**§ 61(3)** It does not:

```text
normalize Unicode;
change case;
replace valid scalars;
append a terminator.
```

**§ 61(4)** The result represents exactly the input scalar sequence.

---

## § 62 Materialization and allocation context

**§ 62(1)** A materializing `ToString()` operation may require allocation or another
string-storage provider.

**§ 62(2)** The compiler records:

```text
MayAllocate where applicable;
RequiresAllocationContext where applicable;
exact source operation;
selected provider/helper.
```

**§ 62(3)** This rulebook does not permit hidden fallback to an unrelated heap.

**§ 62(4)** The exact failure surface and owning string representation must remain
synchronized with:

```text
allocation.md;
arena.md;
string memory rules;
core-library.md.
```

**§ 62(5)** The no-argument source signatures remain the canonical signatures already
required by the core surface.

**§ 62(6)** A profile incapable of satisfying the operation rejects its use.

---

## § 63 `ToByteArray()`

**§ 63(1)** Required on `string`:

```sec
fn ToByteArray() Result[byte[], CollectionError]

let bytes := try text.ToByteArray()
```

**§ 63(2)** It materializes an owning `byte[]` and reports unavailable or insufficient
result storage as `CollectionError` (MD-072).

**§ 63(3)** The result contains exactly the encoded bytes represented by the string.

**§ 63(4)** It:

```text
preserves byte order;
copies or otherwise produces independent owning array storage;
does not append a zero terminator;
does not include unused capacity;
does not expose the string's immutable backing as mutable aliases;
does not consume or mutate the source.
```

**§ 63(5)** For an empty string it returns an empty owning array.

**§ 63(6)** The result length equals the encoded payload byte count (MD-058):

```text
text.SizeOf
```

**§ 63(7)** when `text.SizeOf` is representable as the owning-array element count. For
example, the array produced by `"Hej π".ToByteArray()` has `Len == 6`.

---

## § 64 `ToByteArray()` and encoding

**§ 64(1)** `ToByteArray()` is not a text transcoder.

**§ 64(2)** It returns the string's canonical encoded byte sequence: well-formed UTF-8,
the only encoding a `string` has (`types.md`, "Valid UTF-8 invariant").

**§ 64(3)** Low-level FFI code requiring another encoding uses an explicit transcoding API.

---

## § 65 `ToByteArray()` allocation

**§ 65(1)** `ToByteArray()` is materializing.

**§ 65(2)** It may:

```text
MayAllocate;
RequireAllocationContext;
use Arena-backed owning array storage;
lower to a bulk copy;
be folded for compile-time constants when ownership remains correct.
```

**§ 65(3)** The returned array is independently owned.

**§ 65(4)** Mutating it does not mutate the source string.

---

## § 66 `ToCharArray()`

**§ 66(1)** Required on `string`:

```sec
fn ToCharArray() Result[char[], CollectionError]

let chars := try "test string".ToCharArray()
```

**§ 66(2)** It materializes an owning `char[]` and reports unavailable or insufficient
result storage as `CollectionError`, exactly as `ToByteArray()` does (MD-072).

**§ 66(3)** `ToCharArray()` is byte-preserving (MD-073, 2026-10-31): the result contains one
`char` for each byte of the string's UTF-8 encoding, in byte order, with the
identical numeric value. It is conceptually `ToByteArray()` followed by an exact
per-element `char(byte)` conversion. It does not iterate Unicode scalars and
performs no `rune`-to-`char` representability check, so no scalar is rejected,
truncated or replaced: a scalar encoded in several UTF-8 bytes contributes
several `char` elements.

```sec
let ascii := try "ABC".ToCharArray()  // [0x41t, 0x42t, 0x43t]
let accent := try "é".ToCharArray()    // [0xC3t, 0xA9t]
let omega := try "Ω".ToCharArray()     // [0xCEt, 0xA9t]
```

**§ 66(4)** The result has exactly `text.SizeOf` elements, never `text.Len`; element `i`
equals `text.ByteAt(i)`. All bytes, including zero and continuation bytes, are
preserved.

**§ 66(5)** The operation:

```text
preserves byte order;
does not append a terminator;
does not expose string backing as mutable aliases;
does not consume or mutate the source;
returns an empty array for an empty string;
does not encode into a locale, ANSI or code-page encoding;
introduces no ConversionError, StringError or Unicode-range failure.
```

**§ 66(6)** `"é"` yields the UTF-8 bytes `C3 A9`, not the Latin-1 byte `E9`. Producing a
code-page encoding requires an explicit codec.

**§ 66(7)** Failure publishes no partly formed array and leaves the source intact. An
implementation may reuse the byte-array materialization path and need not
allocate a separate intermediate `byte[]`, but no source-visible implicit
`byte[]` to `char[]` conversion or unsound storage retyping exists.

---

## § 67 `ToRuneArray()`

**§ 67(1)** Required on `string`:

```sec
fn ToRuneArray() Result[rune[], CollectionError]

let runes := try "test string".ToRuneArray()
```

**§ 67(2)** It materializes an owning `rune[]` and reports unavailable or insufficient
result storage as `CollectionError` (MD-072).

**§ 67(3)** The result contains one `rune` for every Unicode scalar represented by the
string, in source order.

**§ 67(4)** It:

```text
does not include encoded continuation bytes as separate runes;
does not append a terminating rune;
does not normalize;
does not change case;
does not consume or mutate the source;
returns an empty array for an empty string.
```

**§ 67(5)** Every string is valid UTF-8 (MD-050), so decoding is valid for every string:
the result has exactly `text.Len` elements and no `InvalidUtf8` path exists.

---

## § 68 Materializing array independence

**§ 68(1)** Arrays returned by:

```text
ToByteArray();
ToCharArray();
ToRuneArray();
```

**§ 68(2)** are owning arrays.

**§ 68(3)** They do not borrow from the source string.

**§ 68(4)** The source string may be destroyed or moved according to normal ownership rules
without invalidating the returned array.

**§ 68(5)** The returned array may be mutated without mutating the source.

**§ 68(6)** An implementation may share immutable storage internally only when it preserves
these observable ownership and mutation semantics, including copy-on-write
requirements where applicable.

**§ 68(7)** No mandatory copy-on-write runtime is introduced.

---

## § 69 Round-trip expectations

### § 69.1 Bytes

**§ 69.1(1)** The standard-library `str.FromByteArray` accepts the canonical encoded bytes of
every string, so decoding the array produced by `text.ToByteArray()` (borrowed
with `ref`) reconstructs equivalent string content; only storage failure can
occur (`StringError.Allocation`). The round trip never uses `byte[].ToString()`,
which is a type summary (MD-091).

### § 69.2 Runes

**§ 69.2(1)** Encoding the array produced by `text.ToRuneArray()` with
`string.FromRuneArray` (borrowed with `ref`) reconstructs equivalent scalar
content.

### § 69.3 Chars

**§ 69.3(1)** No round-trip invariant `text.ToCharArray().ToString() == text` holds for
non-ASCII text (MD-073). `ToCharArray()` preserves UTF-8 bytes, while
char-sequence `ToString()` keeps its own per-`char` interpretation (see
"Char-sequence semantics") and is not changed into a UTF-8 byte decoder to make
the round trip succeed. For ASCII content (`U+0000..U+007F`) one element per
scalar is produced and the round trip reconstructs the text. Decoding a byte
sequence back to text uses `str.FromByteArray` on `ToByteArray()` output.

---

## § 70 No duplicate iterator names

**§ 70(1)** Materializing conversions are:

```text
ToByteArray
ToCharArray
ToRuneArray
```

**§ 70(2)** Names such as:

```text
Bytes
Chars
Runes
```

**§ 70(3)** must not be aliases for the materializing operations.

**§ 70(4)** They may later denote:

```text
iterators;
views;
non-owning traversal;
direct language iteration.
```

**§ 70(5)** Such APIs require distinct semantics.

---

## § 71 Associated string constructors

**§ 71(1)** The compiler-known/core surface includes:

```sec
fn FromRuneArray(value: ref rune[]) Result[string, StringError]
```

**§ 71(2)** It borrows its input and materializes an independent string (MD-072,
`core-library.md` § 5.2). There is no compiler-known or core
`string.FromByteArray`: byte decoding is the standard-library
`str.FromByteArray` (MD-091, 2026-10-18), and `string.FromByteArray(...)` is
diagnosed with a pointer to it.

**§ 71(3)** A `FromCharArray` constructor may be added when the canonical char/string model
is finalized.

**§ 71(4)** These are associated functions, not constructor casts.

**§ 71(5)** This is invalid as a replacement:

```sec
string(value)
```

**§ 71(6)** when converting arrays or slices to string.

**§ 71(7)** The compiler resolves the associated function through the built-in member
registry and privileged core declaration model.

---

## § 72 Internal core string materialization

**§ 72(1)** `str.FromByteArray`'s strict decoding contract below is owned by
`sec/stdlib/str/stdlib_str.md` (MD-091, 2026-10-18). The compiler provides one
narrow capability that turns a validated byte sequence into a string:

```text
canonical name: _stringFromUtf8
registry ID: CKF-STRING-FROM-UTF8
operand: one borrowed contiguous byte sequence (ref byte[], ref byte[N], or a byte slice)
result type: Result[string, StringError]
unsafe: false (it validates strict UTF-8 itself)
availability: loader-proven sec/core only (never stdlib; owner decision 2026-10-18)
public visibility: none (no completion, no public documentation, no core declaration)
```

**§ 72(2)** Only trusted core knows it; core may allocate only inside `ToString()`, so
core uses it from `ToString` implementations (the core byte formatters in
`sec/core/byte.sec`). It borrows its operand and never consumes, adopts,
mutates or aliases it; it returns `StringError.InvalidUtf8(offset)` for
malformed input and `StringError.Allocation` for storage failure. Authority is
loader provenance (`SourceCore`), never path or module spelling. It is not a
public decode path and is not callable as `string.FromByteArray`,
`string(bytes)` or an implicit conversion. How standard-library code
(`str.FromByteArray`, `str.Builder.Finish`, io, url) obtains a string from
validated bytes is open as MD-109. Semantic IR and lowering of the operation
belong to the string-materialization work of P23 and are not implemented
yet.

### § 72.1 Strict decoding contract

**§ 72.1(1)** `str.FromByteArray` decodes the borrowed input as strict UTF-8, the only
canonical string encoding (MD-050).

**§ 72.1(2)** It does not assume zero termination.

**§ 72.1(3)** It uses the represented array or slice bounds.

**§ 72.1(4)** Malformed input returns `Err(StringError.InvalidUtf8(offset))` with the byte
offset of the leading byte of the first malformed or truncated sequence; it
never replaces, truncates, skips, panics or yields a partial string. Storage
failure returns `StringError.Allocation`. The input is never consumed, mutated,
adopted or aliased (`core-library.md` § 5.2).

---

## § 73 `FromRuneArray`

**§ 73(1)** `FromRuneArray` encodes the borrowed Unicode scalar sequence as canonical
UTF-8 and reports storage failure as `StringError.Allocation`.

**§ 73(2)** Every `rune` is already a valid scalar, so safe input cannot produce
`InvalidCodePoint`.

**§ 73(3)** The operation does not:

```text
read beyond the represented bounds;
look for 0r as a terminator;
normalize;
change case.
```

---

## § 74 Fundamental numeric associated properties

**§ 74(1)** The canonical registry also includes representation-sensitive associated
properties already required by the core surface.

### § 74.1 Integer types

**§ 74.1(1)** For each concrete signed and unsigned integer type:

```text
Min
Max
Bits
```

**§ 74.1(2)** `Bits` is the semantic bit width.

**§ 74.1(3)** `Min` and `Max` are compile-time constants of the exact type.

### § 74.2 Floating-point types

**§ 74.2(1)** Where defined:

```text
Min
Max
Epsilon
Infinity
NegativeInfinity
NaN
```

**§ 74.2(2)** The exact values and representability follow numeric rules.

### § 74.3 Decimal types

**§ 74.3(1)** Representation-sensitive members such as:

```text
Scale
```

**§ 74.3(2)** are compiler-known only when the decimal rulebook defines them as stable public
semantics.

**§ 74.3(3)** `decimal.Scale` and `decimal128.Scale` are such members (MD-066/MD-068,
2026-10-28): each has one canonical identity of type `int32` with getter and
fallible setter semantics, and the lowercase `scale` is only a migration
alias of that identity. The registry must not keep the former `uint8` or
`int` result type. The value predicates, scale/rounding methods and
conversions of `core-library.md` §§ 10.2 and 10.4 are the compiler-known
identities listed under "Legacy lowercase member spellings" above; the three
`ToString` overloads (§ 10.7) are declared in the trusted core sources
`sec/core/decimal.sec` and `sec/core/decimal128.sec` and resolve through
ordinary core member lookup. Both types have identical signatures; tooling
must not advertise a `decimal128` member that has no implementation or
gate-aware diagnostic.

**§ 74.3(4)** Internal coefficient layout is not exposed merely because the compiler knows
it.

---

## § 75 `RawPtr[T]` compiler-known members

**§ 75(1)** The registry includes required raw-pointer operations such as:

```sec
unsafe pointer.Read()
unsafe pointer.Write(value)
unsafe pointer.VolatileRead()
unsafe pointer.VolatileWrite(value)
unsafe pointer.Offset(elements)
unsafe pointer.AddBytes(bytes)
unsafe pointer.Difference(other)
unsafe {
    let observer := pointer.AsRef
    let editor := otherPointer.AsRefMut
}
```

**§ 75(2)** `AsRef: ref T` and `AsRefMut: ref mut T` (`CKM-RAWPTR-AS-REF`, `CKM-RAWPTR-AS-REF-MUT`) are the only raw-to-safe reference spellings (MD-070; `raw_pointers.md` § 19A). They exist only on a typed `RawPtr[T]`, never on `RawPtr[void]`, `T`, `ref T` or `ref mut T`, and they carry the caller obligations of `raw_pointers.md` § 19(2)–(3). The reverse direction is the existing unsafe property `.Ptr`. Applied through a safe reference (`r.Ptr`, or `(*r).Ptr` per MD-063) it addresses the referent's storage (`raw_pointers.md` § 18(7)–(9)).

**§ 75(3)** Ordinary raw access and pointer arithmetic are owned by `unsafe.md` and
`reference_model.md`. `VolatileRead` and `VolatileWrite` are distinct observable
physical-storage effects owned by `rules/platform/volatile.md`; they must not be
implemented as aliases for ordinary `Read` and `Write`.

**§ 75(4)** They remain explicit Semantic IR operations until pointer provenance and ABI
lowering are complete.

---

## § 76 Arena compiler-known members

**§ 76(1)** `Arena` members are compiler-known because they affect:

```text
ArenaDomain identity;
ownership;
validity epoch;
ordered effects;
allocation context;
task/thread dependencies;
MLIR lowering.
```

**§ 76(2)** Required entries include:

```text
Arena.FromBuffer
Arena.WithCapacity
Arena.Growable where enabled
Arena.New
Arena.Alloc
Arena.Reset
Arena.Release
```

**§ 76(3)** Their detailed semantics are owned by `arena.md`.

---

## § 77 Core declarations

**§ 77(1)** A compiler-known core method may have a privileged core declaration.

**§ 77(2)** Example:

```sec
impl string {
    fn ToString() Result[string, StringError] {
        return Ok(self)
    }
}
```

**§ 77(3)** The declaration provides:

```text
ordinary source visibility;
documentation;
callable body where needed;
overload shape;
LSP definition location.
```

**§ 77(4)** The compiler-known registry provides:

```text
stable semantic identity;
built-in eligibility;
intrinsic classification;
required analysis facts;
fallback when body is unavailable;
target/helper lowering.
```

**§ 77(5)** The two must agree.

**§ 77(6)** A mismatch is a compiler/core build error.

---

## § 78 Registry and core validation

**§ 78(1)** During compiler/core bootstrap, validate:

```text
canonical name;
receiver type;
associated versus instance kind;
parameters;
result type;
unsafe requirement;
mutating or consuming receiver behavior;
effect declaration where present;
overload set.
```

**§ 78(2)** The compiler must reject a privileged core declaration that contradicts the
registry.

**§ 78(3)** The registry must not silently rewrite an incompatible core declaration.

---

## § 79 Core helper absence

**§ 79(1)** A compiler-known intrinsic may remain usable when no ordinary core body exists,
provided the selected CompilationPlan has a valid intrinsic lowering.

**§ 79(2)** A compiler-known core method requiring a helper body is unavailable when neither:

```text
a valid core implementation;
nor
a valid intrinsic/target lowering
```

**§ 79(3)** exists.

**§ 79(4)** The diagnostic identifies the missing implementation capability.

---

## § 80 User shadowing

**§ 80(1)** An ordinary local variable, field, or method name may textually match a
compiler-known name in a scope where no compiler-known receiver pattern applies.

**§ 80(2)** Example:

```sec
type Report struct {
    Len: int
}
```

**§ 80(3)** Whether this field is legal follows ordinary naming and property rules.

**§ 80(4)** It must not alter array, slice, or string `Len` semantics.

**§ 80(5)** Universal member conflicts on eligible types are rejected.

---

## § 81 Overload resolution

**§ 81(1)** Compiler-known methods participate in ordinary overload resolution.

**§ 81(2)** Examples:

```sec
value.ToString()
value.ToString("N2")
```

**§ 81(3)** are distinguished by parameters, not return type alone.

**§ 81(4)** Compiler-known candidates carry stable IDs through overload resolution.

**§ 81(5)** Diagnostics must identify whether a candidate came from:

```text
compiler intrinsic;
privileged core;
user impl;
related-type inheritance;
interface contract.
```

---

## § 82 Addressability diagnostics

**§ 82(1)** Invalid:

```sec
unsafe {
    let pointer := (a + b).Ptr
}
```

**§ 82(2)** when the temporary has no materialized stable storage.

**§ 82(3)** Diagnostic:

```text
error: Ptr requires an addressable value

the expression exists only as a temporary SSA value

help:
    store the value in a local binding before obtaining its pointer
```

**§ 82(4)** The compiler must not materialize storage solely to make the expression legal.

---

## § 83 `Len` diagnostics

**§ 83(1)** Invalid:

```sec
let count := number.Len
```

**§ 83(2)** Diagnostic:

```text
error: type `int` has no compiler-known Len property

Len is available on:
    string
    arrays
    owning sequences
    shared slices
    mutable slices
```

**§ 83(3)** A user-defined ordinary `Len` method/property may still exist on its own nominal
type.

---

## § 84 `SizeOf` diagnostics

**§ 84(1)** Invalid:

```sec
let size := UnsizedType.SizeOf
```

**§ 84(2)** Diagnostic:

```text
error: SizeOf requires a complete sized layout

type:
    UnsizedType

missing:
    concrete layout for the active CompilationPlan
```

**§ 84(3)** For multi-output builds, report the failing plan.

---

## § 85 String-conversion diagnostics

**§ 85(1)** Diagnostics must distinguish:

```text
member unavailable;
allocation context unavailable;
target materialization unsupported;
invalid string encoding;
char representation mismatch;
result ownership unsupported;
missing core/helper implementation.
```

**§ 85(2)** Example:

```text
error: ToRuneArray requires a materializing string allocation context

call:
    text.ToRuneArray()

active profile:
    baremetal/cortex-m4/noalloc

help:
    provide an explicit Arena-backed conversion API
    or use non-materializing rune iteration
```

**§ 85(3)** Exact help depends on the available APIs.

---

## § 86 Legacy spelling diagnostics

**§ 86(1)** When legacy compatibility is enabled:

```sec
value.ptr
value.len
```

**§ 86(2)** resolve to canonical members and produce configurable information.

**§ 86(3)** When compatibility is disabled, they are ordinary unresolved member names.

**§ 86(4)** The diagnostic should not suggest both `Len` and `Length`.

**§ 86(5)** Only the canonical spelling is suggested.

---

## § 87 Compile-time evaluation

**§ 87(1)** Compiler-known members may be compile-time evaluable when all required inputs
are compile-time known and the operation is permitted by CTFE.

**§ 87(2)** Always or normally CTFE-compatible:

```text
fixed-array Len;
type-form SizeOf;
numeric Min;
numeric Max;
numeric Bits;
layout constants.
```

**§ 87(3)** Conditionally CTFE-compatible:

```text
value-form SizeOf after preserving receiver effects;
string Len for compile-time string;
ToString for compile-time scalar;
string materialization conversions when CTFE storage rules support owning
results.
```

**§ 87(4)** Not normal CTFE:

```text
Ptr to runtime storage;
RawPtr dereference;
Arena allocation;
target-provided runtime helpers;
volatile operations.
```

**§ 87(5)** Compile-time evaluation does not change source semantics.

---

## § 88 Allocation-context integration

**§ 88(1)** Materializing compiler-known methods record whether they:

```text
MayAllocate;
RequireAllocationContext;
accept an explicit Arena through another overload;
can be compile-time materialized;
are unavailable in a no-allocation profile.
```

**§ 88(2)** The compiler-known registry must not embed one universal heap provider.

**§ 88(3)** Allocation origin is resolved through `allocation.md` and `arena.md`.

---

## § 89 Ownership and borrow integration

### § 89.1 Result projections

**§ 89.1(1)** The canonical registry includes these compiler-known `Result[T, E]` members:

```text
Ok()   -> Option[T]       consuming receiver
Err()  -> Option[E]       consuming receiver
OkRef  -> Option[ref T]   shared non-consuming property
ErrRef -> Option[ref E]   shared non-consuming property
```

**§ 89.1(2)** Consuming projections transfer the retained active payload without hidden
clone and destroy the non-retained payload exactly once. Sema rejects a
projection that might abandon a non-discardable obligation. Borrowed properties
create a shared payload borrow only when active and cannot outlive the Result.
Registry entries expose stable IDs, exact receiver/result types, receiver mode,
effects, and normative source so compiler and LSP use identical facts.

**§ 89.1(3)** Registry entries specify whether the receiver is:

```text
shared;
mutably borrowed;
consumed;
addressability-only;
compile-time type-only.
```

**§ 89.1(4)** Examples:

```text
Len:
    shared, non-consuming

SizeOf value form:
    ordinary evaluation, non-consuming

Ptr:
    non-consuming, addressability required, unsafe

ToByteArray:
    shared source, independent owning result

Arena.Release:
    consuming receiver
```

**§ 89.1(5)** Sema and Semantic IR use these facts.

---

## § 90 Call-graph integration

**§ 90(1)** A compiler-known member may be:

```text
semantically atomic;
lowered to a real helper call;
implemented by privileged core;
target-dispatched;
open through a user-defined ToString implementation.
```

**§ 90(2)** The call graph records actual callable helpers when they matter.

**§ 90(3)** It may represent a fully atomic intrinsic through a summary only when this is
sound for:

```text
effects;
stack;
allocation;
panic;
unsafe provenance;
task/thread behavior.
```

---

## § 91 LSP

**§ 91(1)** The LSP consumes the canonical compiler registry.

**§ 91(2)** It must provide:

```text
completion;
signature help;
hover;
definition or synthetic definition;
overload information;
legacy-name migration;
effect and unsafe information;
target/profile availability;
named-type inheritance source;
lowering category in analysis mode.
```

**§ 91(3)** Examples:

```text
Len
    compiler-known property
    result: uint
    string meaning: Unicode scalar count; the value-form SizeOf is the UTF-8 byte count

Ptr
    unsafe compiler-known property
    result: RawPtr[byte]
    does not extend lifetime

SizeOf
    compiler-known property or global type query
    result for linux/amd64: 16 bytes
```

### § 91.1 User-facing symbol names

**§ 91.1(1)** Ordinary tooling presentation uses the resolved public source name and
concrete receiver type, for example `list[Customer].Append` or `string.Len`.
The stable `CKM-`, `CKF-` and `CKV-` identities remain semantic and transport
facts; they must not substitute for a user-facing name. Stable diagnostic
codes are unaffected.

**§ 91.1(2)** The shared compiler registry supplies these display facts. Consumers must not
maintain a second ID-to-label inventory or recover a name by decoding an ID.
Receiver type arguments and the member signature come from the already
resolved registry entry. A descriptive receiver pattern is not a concrete
receiver type and must not be presented as one.

**§ 91.1(3)** Global functions and value intrinsics use their canonical source spelling.
Presentation never grants access to an internal or unavailable operation;
visibility, effects, unsafe requirements and target/profile availability still
come from the original resolved semantic entry.

**§ 91.1(4)** If a public name or signature is unavailable, tooling uses an explicit,
readable fallback rather than an implementation ID or an invented source
signature. An unavailable contextual parameter list does not establish a
zero-argument function. A context-dependent result remains distinguishable
from a known `void` result.

---

## § 92 Synthetic definitions

**§ 92(1)** When no ordinary core declaration exists, LSP may navigate to a synthetic
read-only definition generated from the registry.

**§ 92(2)** The synthetic definition must state:

```text
compiler-known;
normative rulebook section;
receiver pattern;
signature;
effects;
unsafe requirement;
target restrictions.
```

**§ 92(3)** It must not pretend that generated text is user source.

**§ 92(4)** Synthetic navigation uses a friendly public member/receiver name for the
editor-visible document basename and generated member label. Stable registry
identities remain transport data, rather than visible filenames or definition
text. Unknown public names or signatures use explicit readable fallbacks;
tooling must not recover a name by decoding an identity.

**§ 92(5)** The `sec-compiler-known:` scheme and `sec/compilerKnownDefinition` content
request serve generated read-only documents outside the workspace. Generated
text starts with a comment stating that it is synthetic registry output, not
editable or compilable source. It retains category, normative rule, receiver,
effects, unsafe requirements, target restrictions and caller obligations.
Navigation places the cursor on the resolved signature, or an explicit
signature-unavailable comment when that fact is absent.

**§ 92(6)** Receiver-instantiated signatures preserve concrete nominal types while
method-local generic parameters remain generic. URI transport retains stable
operation identity and an instance discriminator for receiver-specific or
call-context-specific metadata. Distinct instances must not overwrite each
other's content. The bounded presentation cache requires renewed navigation
for an evicted instance; a cold instance request must never fall back to an
unrelated or generic definition. Equivalent editor URI escaping or query order
must not change the resolved instance.

**§ 92(7)** Legacy ID-only requests may retain public generic registry recovery. They do
not supply core authority: private functions and value intrinsics require
navigation from an authorized resolved use, regardless of whether the content
request uses a friendly filename or a legacy ID basename.
`CompilerKnownDocumentationForType` generates derived documentation through the
same friendly synthetic-definition renderer.

---

## § 93 Formatter

**§ 93(1)** The formatter:

```text
preserves canonical member casing;
may migrate legacy ptr/len spelling under explicit migration mode;
does not rewrite ordinary user members merely by name;
preserves unsafe blocks;
does not convert len(value) into value.Len or the reverse;
does not change SizeOf value/type form.
```

**§ 93(2)** `len(value)` and `value.Len` have different result types and are not formatter
style aliases.

---

## § 94 Documentation generation

**§ 94(1)** Manuals and API references may be generated from the registry plus privileged
core declarations.

**§ 94(2)** Generated documentation must distinguish:

```text
compiler intrinsic;
compiler-known core;
ordinary core;
standard library;
user implementation.
```

**§ 94(3)** Derived documentation is not normative.

---

## § 95 Separate compilation

**§ 95(1)** Module metadata records compiler-known usage through stable IDs.

**§ 95(2)** It must not serialize only display strings.

**§ 95(3)** Metadata may include:

```text
member ID;
resolved overload;
receiver type;
result type;
unsafe requirement;
effects;
allocation-context requirement;
target/layout dependency;
helper symbol where required.
```

**§ 95(4)** A registry-version mismatch invalidates incompatible module metadata.

---

## § 96 Incremental compilation

**§ 96(1)** Changes to these facts invalidate affected semantic and lowering results:

```text
registry entry;
canonical name;
legacy name;
receiver pattern;
layout;
string representation;
array/slice representation;
target ABI;
effect summary;
allocation-context requirement;
core helper declaration;
target helper availability.
```

**§ 96(2)** Unrelated registry entries should retain stable IDs.

---

## § 97 Required registry inventory

**§ 97(1)** Sec 0.1 must include at least the following public entries.

### § 97.1 Universal or representation-sensitive

```text
Ptr
SizeOf
```

### § 97.2 Sequence length

```text
Len
IsEmpty
```

### § 97.3 Collection initialization and mutation

```text
fill
Fill
Reverse
Append
Insert
RemoveAt
Remove
Clear
Contains
ContainsKey
IndexOf
Sort
SortBy
Add
Union
Intersection
Difference
SymmetricDifference
len
```

**§ 97.3(1)** Availability and result types are receiver-specific and are defined by
`collections.md`. Listing a name here does not make it universal: for example,
`Ptr` is absent on map/set, `Fill` is limited to mutable slices, and dynamic
arrays expose no `Capacity`.

### § 97.4 Collection capacity members

**§ 97.4(1)** MD-058 (2026-10-12, `rules/corrections/applied/md058-collection-capacity-reservation-correction-20261012.md`) adds these registry entries:

```text
CKM-LIST-CAPACITY      property Capacity: uint                 list[T], list[T, Capacity]
CKM-MAP-CAPACITY       property Capacity: uint                 map[K, V], map[K, V, Capacity]
CKM-SET-CAPACITY       property Capacity: uint                 set[T], set[T, Capacity]
CKM-LIST-RESERVE       fn Reserve(additional: uint) Result[void, CollectionError]
CKM-MAP-RESERVE        fn Reserve(additional: uint) Result[void, CollectionError]
CKM-SET-RESERVE        fn Reserve(additional: uint) Result[void, CollectionError]
CKM-LIST-WITHCAPACITY  static fn WithCapacity(capacity: uint) Result[list[T], CollectionError]
CKM-MAP-WITHCAPACITY   static fn WithCapacity(capacity: uint) Result[map[K, V], CollectionError]
CKM-SET-WITHCAPACITY   static fn WithCapacity(capacity: uint) Result[set[T], CollectionError]
```

**§ 97.4(2)** `Reserve` is a structurally mutating instance method of a mutable dynamic
receiver; it may allocate through the active allocation context, may relocate
backing storage and is transactional. `WithCapacity` is an associated factory
that returns a new empty owner and allocates only when its capacity is nonzero.
Bounded variants, `T[]`, fixed arrays and slices have neither member, and no
`Capacity()` method exists. Their full contracts are `collections.md`
§§ 13.12, 14.8 and 15.5.

### § 97.5 String and formatting

```text
ToString
ToByteArray
ToCharArray
ToRuneArray
string.FromRuneArray
_stringFromUtf8 (internal, core only)
byte-sequence ToString
char-sequence ToString
rune-sequence ToString
```

### § 97.6 Numeric representation

```text
Min
Max
Bits
Epsilon where applicable
Infinity where applicable
NegativeInfinity where applicable
NaN where applicable
Scale where canonically defined
```

### § 97.7 Raw pointers

```text
Read
Write
VolatileRead
VolatileWrite
Offset
AddBytes
Difference
```

### § 97.8 Arena

```text
FromBuffer
WithCapacity
Growable where enabled
New
Alloc
Reset
Release
```

### § 97.9 Iteration protocol

```text
CKM-ITERATOR-NEXT
    source contract: explicit implements Iterator[T]
    concrete operation: mut Next() Option[T]
    dispatch: statically resolved
    runtime support: none
```

**§ 97.9(1)** The `Iterator[T]` interface and `for` behavior are defined by
`rules/control-flow/flowcontrol_for.md`, section 37. This registry identity lets
Semantic IR, lowering, and tooling consume the exact Sema-selected method. A
member named `Next` without explicit conformance never acquires this identity.

### § 97.10 Unit metadata members

**§ 97.10(1)** Read-only value members of a named unit `<U>` (MD-062, 2026-10-27; types,
defaults and diagnostics in `rules/types/units.md`, "Unit metadata value
types"):

```text
Name        string           implicit
Category    UnitCategory     implicit
LongName    string
Symbol      string
BaseUnit    bool
Status      UnitStatus
Dimension   UnitDimension
Kind        concrete enum type of the value
Scale       decimal
System      concrete enum type of the value
Transform   UnitTransform
Offset      decimal
Origin      concrete enum type of the value
LogBase     decimal
LogFactor   decimal
Reference   decimal or decimal<V>
```

**§ 97.10(2)** Each has a stable registry ID; legacy migration spellings of metadata keys
do not become separate entries. These are value members only: `<U>.Name(...)`
still resolves an ordinary unit function. Structural unit expressions have
none of them.

### § 97.11 `UnitDimension`

```text
AxisCount        read-only property, uint
IsDimensionless  read-only property, bool
==, !=           compiler-defined equality (rules/foundations/operators.md)
```

**§ 97.11(1)** No other members, constructors or operators exist
(`rules/library/core-library.md` § 27.5).

**§ 97.11(2)** Other compiler-known entries may exist when owned by their canonical
rulebooks.

**§ 97.11(3)** The registry is not limited to this minimum list.

---

## § 98 Excluded aliases in Sec 0.1

**§ 98(1)** The following are not canonical aliases:

```text
Length
Pointer
Data
DataPtr
ByteCount
Bytes as materializing alias
Chars as materializing alias
Runes as materializing alias
sizeof global operator
```

**§ 98(2)** A later rulebook may add a distinct operation when its semantics are genuinely
different.

---

## § 99 Required source tests

**§ 99(1)** Create or update:

```text
compiler_known_ptr_valid.sec
compiler_known_ptr_invalid.sec
compiler_known_len_valid.sec
compiler_known_len_invalid.sec
compiler_known_global_len_valid.sec
compiler_known_global_len_invalid.sec
compiler_known_sizeof_valid.sec
compiler_known_sizeof_invalid.sec
compiler_known_tostring_valid.sec
compiler_known_tostring_invalid.sec
compiler_known_string_to_byte_array_valid.sec
compiler_known_string_to_byte_array_invalid.sec
compiler_known_string_to_char_array_valid.sec
compiler_known_string_to_char_array_invalid.sec
compiler_known_string_to_rune_array_valid.sec
compiler_known_string_to_rune_array_invalid.sec
compiler_known_char_sequence_to_string_valid.sec
compiler_known_rune_sequence_to_string_valid.sec
compiler_known_byte_sequence_to_string_invalid.sec
compiler_known_named_type_valid.sec
compiler_known_named_type_invalid.sec
compiler_known_legacy_spelling_valid.sec
compiler_known_legacy_spelling_invalid.sec
compiler_known_allocation_context_invalid.sec
compiler_known_target_invalid.sec
```

---

## § 100 Registry tests

**§ 100(1)** Test:

```text
stable IDs;
canonical names;
legacy aliases;
receiver patterns;
overloads;
unsafe flags;
allocation-context flags;
inheritable flags;
overridable flags;
core declaration matching;
target availability.
```

---

## § 101 `Ptr` tests

**§ 101(1)** Test:

```text
addressable scalar;
local;
field;
array;
slice;
string;
static;
Arena-backed storage;
volatile/addressed storage;
empty array;
empty slice;
empty string;
temporary rejection;
lifetime non-extension;
unsafe requirement;
legacy ptr migration.
```

---

## § 102 `Len` and `len` tests

**§ 102(1)** Test:

```text
string Len and value-form SizeOf with distinct units, and no ByteLen/RuneLen/CharLen alias;
fixed-array compile-time length;
owning sequence runtime length;
shared slice;
mutable slice;
Len result uint;
len result int;
argument evaluated once;
large-length representability;
invalid receiver;
user redeclaration rejection;
legacy len migration.
```

---

## § 103 `SizeOf` tests

**§ 103(1)** Test:

```text
scalar;
named type;
struct padding;
enum;
union;
register;
fixed array;
owning array live payload bytes;
slice represented payload bytes;
list live payload bytes;
string descriptor;
safe reference profile;
RawPtr target width;
Arena descriptor;
type form;
value form receiver effects;
generic specialization;
incomplete type rejection;
multi-target differing result.
```

---

## § 104 String materialization tests

**§ 104(1)** Test:

```text
string identity ToString;
bool ToString;
numeric ToString;
char ToString;
rune ToString;
byte array ToString;
byte slice ToString;
char array ToString;
char slice ToString;
rune array ToString;
rune slice ToString;
ordinary aggregate and collection fallback ToString;
selected slice bounds;
no terminator scan;
empty input;
ToByteArray independence;
ToCharArray independence;
ToRuneArray independence;
round trip;
strict UTF-8 decoding failures with exact first-invalid byte offsets
    (0xFF, leading continuation bytes, overlong, truncated 2/3/4-byte,
    surrogates, above U+10FFFF, near buffer ends) leaving the input unchanged;
rune-position search, string ranges (MD-126) and rune access versus byte access
    (IndexOf, IndexOf(start), LastIndexOf, text[a..<b], GetRune, RuneAt, GetByte)
    against one shared rune-index oracle;
Split, SplitToArray and SplitOnce with multibyte separators;
materialization storage failure as CollectionError / StringError.Allocation;
allocation-context requirement;
noalloc profile rejection;
compile-time constant folding.
```

---

## § 105 Semantic IR tests

**§ 105(1)** Golden tests must verify explicit operations for:

```text
GetPointer;
GetLength;
SizeOfType;
SizeOfValue;
StringIdentity;
StringToByteArray;
StringToCharArray;
StringToRuneArray;
CharSequenceToString;
RuneSequenceToString;
RawPtr operations;
Arena operations.
```

**§ 105(2)** The tests must verify source locations and stable member IDs.

---

## § 106 MLIR tests

**§ 106(1)** Test:

```text
high-level Sec intrinsic operation printing;
type verification;
layout attributes;
effect interfaces;
unsafe provenance;
allocation-context operands;
helper-call lowering;
constant folding;
target-specific SizeOf;
Ptr lowering;
Len lowering;
string conversion lowering;
absence of premature raw-pointer lowering.
```

---

## § 107 Diagnostic tests

**§ 107(1)** Verify:

```text
stable diagnostic ID;
canonical spelling suggestion;
no Length suggestion;
unsafe requirement;
addressability cause;
layout cause;
allocation-context cause;
target/profile cause;
core/helper mismatch;
named-type inheritance explanation;
complete source path.
```

---

## § 108 Implementation plan

### § 108.1 Phase 1 — Registry foundation

1. Add stable `CompilerKnownMemberID`.
2. Add one canonical registry.
3. Replace scattered string checks gradually.
4. Add canonical and legacy spellings.
5. Add synthetic declaration metadata.
6. Validate registry uniqueness.

### § 108.2 Phase 2 — Unified member resolution

1. Resolve compiler-known and core members through one member system.
2. Preserve origin category.
3. Integrate overload resolution.
4. Integrate named-type inheritance.
5. Integrate generic constraints.
6. Reject illegal user overrides.

### § 108.3 Phase 3 — Core validation

1. Load privileged core declarations.
2. Match them to registry entries.
3. Validate signatures.
4. Validate unsafe and ownership behavior.
5. Validate effect declarations.
6. Report compiler/core mismatch.

### § 108.4 Phase 4 — Fundamental members

**§ 108.4(1)** Implement in this order:

```text
Len;
global len;
Ptr;
SizeOf;
string identity ToString;
byte/char/rune sequence ToString;
ToByteArray;
ToCharArray;
ToRuneArray;
numeric representation properties;
remaining ToString implementations.
```

### § 108.5 Phase 5 — Semantic IR

1. Add explicit intrinsic operations.
2. Preserve stable member IDs.
3. Preserve effects.
4. Preserve allocation context.
5. Preserve unsafe provenance.
6. Preserve target/layout dependencies.

### § 108.6 Phase 6 — Sec MLIR

1. Add high-level operations.
2. Add verifiers.
3. Add constant folding.
4. Add effect interfaces.
5. Add helper-call lowering.
6. Add target specialization.
7. Lower to standard MLIR and LLVM.

### § 108.7 Phase 7 — Tooling

1. LSP completion.
2. Synthetic definitions.
3. Hover and effects.
4. Legacy migration.
5. Formatter support.
6. Generated API documentation.

### § 108.8 Phase 8 — Migration

1. Replace `.ptr` with `.Ptr` in normative examples.
2. Replace `.len` with `.Len` in normative examples.
3. Retain `len(...)` where it is used; it now returns `uint`, the same as
   `.Len`, and signed offsets need an explicit checked conversion.
4. Update core and stdlib source.
5. Update bootstrap source.
6. Remove legacy compatibility after the migration gate.

---

## § 109 Implementation graph nodes

**§ 109(1)** Recommended nodes include:

```text
CKM-REGISTRY
CKM-STABLE-IDS
CKM-UNIFIED-LOOKUP
CKM-CORE-VALIDATION
CKM-LEGACY-MIGRATION
CKM-PTR
CKM-LEN-PROPERTY
CKM-LEN-FUNCTION
CKM-SIZEOF
CKM-TOSTRING
CKM-BYTE-SEQUENCE-TOSTRING
CKM-CHAR-SEQUENCE-TOSTRING
CKM-RUNE-SEQUENCE-TOSTRING
CKM-STRING-TOBYTEARRAY
CKM-STRING-TOCHARARRAY
CKM-STRING-TORUNEARRAY
CKM-NUMERIC-PROPERTIES
CKM-RAWPTR
CKM-ARENA
CKM-SEMANTIC-IR
CKM-MLIR
CKM-DIAGNOSTICS
CKM-LSP
CKM-FORMATTER
CKM-TESTS
```

**§ 109(2)** Codex must not implement the complete compiler-known-member model as one
undifferentiated task.

---

## § 110 Current repository synchronization

**§ 110(1)** The repository currently contains partial direct Sema handling and partial core
stubs for several members.

**§ 110(2)** The implementation inventory must verify current code before assigning status.

**§ 110(3)** Known partial areas include:

```text
lowercase ptr resolution;
lowercase len properties;
global len recognition;
string ToString;
rune ToString stub;
string ToRuneArray stub;
byte/char/rune-sequence ToString recognition;
Arena semantic members;
RawPtr type and operations;
numeric core declarations.
```

**§ 110(4)** Implemented frontend foundation:

```text
one typed Sema-owned compiler-known member registry;
stable CKM identities for the currently exposed entries;
canonical Ptr and Len lookup with lowercase migration spellings;
global len recognition;
value-form and type-form SizeOf type resolution;
universal fallback and byte/char/rune-sequence ToString type resolution;
string ToByteArray, ToCharArray, and ToRuneArray type resolution;
string.FromRuneArray type resolution; internal _stringFromUtf8 authority (core only);
integer Min, Max, and Bits and floating representation property lookup;
RawPtr Read, Write, VolatileRead, VolatileWrite, Offset, AddBytes, and Difference validation;
stable CKM-RAWPTR-VOLATILE-READ and CKM-RAWPTR-VOLATILE-WRITE identities;
unsafe, non-void, argument-type, and explicit volatile-effect validation;
Arena constructor and instance-member validation;
compiler-known Iterator[T] identity and CKM-ITERATOR-NEXT Sema resolution;
LSP value and static member completion sourced from the same registry;
canonical compiler-known member TextMate highlighting.
```

**§ 110(5)** Known architectural work includes:

```text
canonical casing migration;
complete allocation-aware materialization;
explicit Semantic IR operations;
Sec MLIR operations;
target/profile validation;
complete diagnostics.
```

**§ 110(6)** This section is informative implementation status.

**§ 110(7)** It does not weaken the normative rules.

---

## § 111 Required synchronization

**§ 111(1)** This rulebook must remain synchronized with:

```text
allocation.md
arena.md
collections.md
attributes.md
call_graph.md
compiler_analysis.md
compiler_pipeline.md
core-library.md
default_values.md
diagnostics.md
effect_analysis.md
platform/ffi.md
formatting.md
declarations/generics.md
impl.md
layout.md
lsp.md
ownership.md
properties.md
reference_model.md
semantic_ir.md
stdlib.md
string memory rules
rules/platform/target_profiles.md
rules/platform/volatile.md
types.md
unsafe.md
```

**§ 111(2)** When a listed file does not yet exist, its future rulebook must consume the
canonical member definitions here.

---

## § 112 Rulebook ownership summary

```text
compiler_known_members.md
    owns registry, availability, resolution, spelling, and intrinsic identity

core-library.md
    owns ordinary core declarations and helper bodies

properties.md
    owns user-defined property syntax

impl.md
    owns user-defined methods and implementations

layout.md
    owns physical size and alignment

collections.md
    owns array and slice bounds and sequence semantics

reference_model.md
    owns reference and pointer validity

unsafe.md
    owns unsafe obligations

allocation.md and arena.md
    own allocation-context and storage allocation

effect_analysis.md
    owns inferred and declared effects

semantic_ir.md
    owns the general IR framework

lsp.md
    owns tooling presentation, not semantic truth
```

---

## § 113 Shaped types

**§ 113(1)** The canonical registry recognizes these shaped receiver families:

```text
vector[T, N]
matrix[T, Rows, Columns]
tensor[T, Dims...]
tensor[T, Shape[Rank]]
ref tensor_view[T, Rank]
ref mut tensor_view[T, Rank]
```

**§ 113(2)** It also recognizes `Shape[Rank]`, `Strides[Rank]`, `TensorLayout[Rank]`,
`Axes[Rank]`, `AxisList[Count]`, `MemorySpace`, `StorageRequest`,
`ShapedStorageRequest[Rank]`, `StorageError` and `ShapeError` as
compiler-known supporting identities. Under the § 21 A/B rule (MD-016 closure,
2026-10-21) the metadata types, `StorageRequest`, `ShapedStorageRequest`,
`StorageError` and `ShapeError` are **A** (real declarations in
`shaped-types.md` §§ 4.1, 6.0, 7.1, 11, 15.3, 29.3, 29.5); `MemorySpace` is
public but its compiler-private representation is **B**; the shaped families
are compiler-known type forms (**B** for their representation) with the exact
`init` overloads of `shaped-types.md` § 12; `MemorySpaceKind`, storage origin,
invalidation domains and layout metadata are **B** and not public types.

**§ 113(3)** Where meaningful for the receiver, the read-only properties are `Rank`,
`Shape`, `Len`, `Strides`, `Layout`, `MemorySpace`, `IsContiguous`, and
`SizeOf`. `Ptr` is additionally unsafe and requires addressability. These are
properties rather than zero-argument methods. Shaped `Len` is the total logical
scalar element count.

**§ 113(4)** On a shaped instance, `SizeOf` is the represented logical payload byte count,
conceptually `value.Len * element payload size`. It is not descriptor size,
storage span, or unique backing-byte count. Associated `Type.SizeOf` remains
the physical complete-type layout query.

**§ 113(5)** The registry provides stable semantic identities for `Reshape`, `ToShape`,
`Materialize`, `TransferTo`, `Relayout`, `Permute`, `Transpose`, `BroadcastTo`,
`Dot`, `Outer`, `Magnitude`, `Normalize`, `Cross`, and `Contract`. The `x`
operator remains compiler-known shaped algebraic multiplication.

**§ 113(6)** Ownership and effect facts distinguish borrowed non-allocating `Reshape`,
consuming non-allocating `ToShape`, new-owner `Materialize`, source-preserving
`TransferTo`, and potentially storage-producing `Relayout`.

**§ 113(7)** `ToShape` is a **static** method of each owning family whose first parameter is
the consuming `-> source` (`tensor[T, Shape[Rank]].ToShape(<-source,
newShape)`); the instance form is rejected. `Materialize`, `TransferTo` and
`Relayout` fail with `StorageError` and require a legal duplication of `T`.
For each concrete receiver the registry records stable IDs, receiver
patterns, rank/type substitutions, argument names and ownership modes, result
and rank conditions, error channels, effects, addressability, borrow
provenance and target availability, and LSP synthesizes the exact specialized
signature from them (MD-016 closure § 14).

**§ 113(8)** Associated `Rank`, `Shape`, and `Len` exist only when the complete value is a
static type fact. Runtime-shaped tensors and tensor views do not synthesize
type-level runtime `Shape` or `Len` values.

**§ 113(9)** Compiler, Sema, completion, hover, definition lookup, overload resolution, and
diagnostics consume this same registry. No LSP-local shaped member inventory is
permitted.

### § 113.1 Shaped metadata families (MD-016 part 1)

**§ 113.1(1)** The five shaped metadata families are compiler-known nominal types whose exact
declarations are owned by `shaped-types.md` §§ 4.1, 6.0, 7.1 and 15.3
(`rules/corrections/applied/md016-shaped-metadata-declarations-correction-20261018.md`). Their registry entries are exactly:

| Family | Members | Read-only indexed expression | Construction |
| --- | --- | --- | --- |
| `Shape[Rank]` | `Rank: uint` get-only property | `shape[axis: uint] -> uint` | `init(extents: uint[Rank])`, infallible; literal `Shape[Rank] { ... }` |
| `Strides[Rank]` | `Rank: uint` get-only property | `strides[axis: uint] -> int` | `init(strides: int[Rank])`, infallible; literal `Strides[Rank] { ... }` |
| `TensorLayout[Rank]` | enum variants `DenseRowMajor`, `DenseColumnMajor`, `Strided` | none | variants only; no `init` |
| `Axes[Rank]` | `Rank: uint` get-only property | `axes[position: uint] -> uint` | `init(axes: uint[Rank]) ShapeError`; statically valid literal |
| `AxisList[Count]` | `Count: uint` get-only property | `axisList[position: uint] -> uint` | `init(axes: uint[Count]) ShapeError`; statically valid literal |

**§ 113.1(2)** Rank and Count are compile-time non-negative integer arguments of these type
forms only. The indexed expressions are compiler-known read-only operations
with ordinary bounds semantics, not public methods. No other constructor,
iterator, mutable accessor, cast, pointer member, public field, `ToString`,
`.Len` or shaped-payload `.SizeOf` exists on these descriptors. `ShapeError`
resolves through the ordinary nominal registry with exactly the nine
cases of `shaped-types.md` § 29.3 — since the MD-016 closure (2026-10-21)
an error **union** whose ninth case is `InitError(StorageError)`; `InvalidStep`
was removed (step validity uses `RangeError`). Hover and signature help show the exact
declarations, structured comments and, for `Axes`/`AxisList`,
`constructs: Axes[Rank]` / `construction error: ShapeError` rather than a
`Result` return type; go-to-definition may open a generated read-only
definition.

---

## § 114 Final canonical summary

**§ 114(1)** Compiler-known means that the compiler owns the semantic existence and identity
of an operation even when core or target code supplies its implementation.

**§ 114(2)** Compiler-known members use ordinary source syntax and ordinary AST member
expressions.

**§ 114(3)** Sema resolves them through one canonical typed registry.

**§ 114(4)** The canonical public low-level properties are `Ptr` and `Len`.

**§ 114(5)** `Length` is not an alias.

**§ 114(6)** Existing lowercase `.ptr` and `.len` are migration spellings and resolve to the
same member IDs only during compatibility migration.

**§ 114(7)** The global compiler-known `len(value)` remains and, like `value.Len`, returns
`uint` (MD-074/MD-088).

**§ 114(8)** `Ptr` is unsafe, requires addressable storage, returns `RawPtr`, and does not
extend lifetime, preserve bounds, transfer ownership, or keep Arena storage
alive.

**§ 114(9)** `SizeOf` is available in value and type form for complete sized layouts and
returns target-specific physical storage size in bytes.

**§ 114(10)** Value-form receiver effects remain observable.

**§ 114(11)** String `Len` counts Unicode scalar values; value-form `SizeOf` is the encoded UTF-8 byte length (MD-058).

**§ 114(12)** Array and slice `Len` is element count.

**§ 114(13)** `ToString() Result[string, StringError]` is a universal
compiler-provided fallback on every Sec type that can produce an ordinary
value; eligible user-owned nominal types may replace it with the exact
canonical shape, and every `ToString` declaration returns that result type.

**§ 114(14)** `string.ToString()` is identity.

**§ 114(15)** `byte`, `char`, and `rune` arrays and slices provide materializing `ToString()`
over the represented range without terminator scanning. Byte sequences preserve
their encoded bytes, char sequences follow the canonical char conversion, and
rune sequences encode each Unicode scalar, including multibyte scalars.

**§ 114(16)** Strings provide materializing `ToByteArray()`, `ToCharArray()`, and
`ToRuneArray()`.

**§ 114(17)** The returned arrays are independently owned, preserve order, contain no implicit
terminator, and do not alias mutable string storage.

**§ 114(18)** `ToByteArray()` returns canonical encoded bytes.

**§ 114(19)** `ToRuneArray()` returns Unicode scalar values.

**§ 114(20)** `ToCharArray()` returns `Result[char[], CollectionError]` with one `char` per
UTF-8 byte and the same numeric values as `ToByteArray()` (MD-073); it performs
no rune-range check and no code-page encoding.

**§ 114(21)** Materializing operations preserve allocation-context and effect facts and never
silently fall back to an unrelated heap.

**§ 114(22)** Compiler-known operations remain explicit in Semantic IR and Sec MLIR until
representation, effects, ownership, allocation, and target lowering are
complete.

**§ 114(23)** The registry is the single source for compiler, core validation, LSP,
formatter, documentation generation, separate compilation, and diagnostics.

**§ 114(24)** No compiler-known operation bypasses ownership, lifetime, allocation, unsafe,
effect, or target-profile rules.

**§ 114(25)** As-property naming (owner clarification, 2026-11-06): As* conversions are properties, never methods; functional getters use Get*(). RawPtr.AsRef and RawPtr.AsRefMut are unsafe read-only properties. Their stable identities and safety contracts do not change. See `rules/corrections/applied/md070-as-property-naming-correction-20261106.md`.
