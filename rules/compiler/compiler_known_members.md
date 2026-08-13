# Compiler-Known Members

## Status

This document is the canonical rulebook for compiler-known functions, methods,
properties, associated members, and internal operations in Sec 0.1.

It defines:

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

All normative compiler-known-member semantics belong to this document.

`core-library.md` owns ordinary core APIs and helper implementations.

`stdlib.md` owns optional higher-level library APIs.

`properties.md` owns ordinary user-defined property syntax and behavior.

`rules/declarations/impl.md` owns ordinary user-defined implementation blocks.

`types.md`, `collections.md`, `reference_model.md`, `unsafe.md`,
`allocation.txt`, `arena.md`, `layout.md`, `effect_analysis.md`, and
`semantic_ir.txt` own their respective semantic domains.

This document consumes those rules and determines when the compiler must know a
member independently of an ordinary source declaration.

---

# Purpose

Some operations cannot be resolved or implemented correctly as ordinary Sec
source alone.

They require compiler knowledge of one or more of:

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

Examples include:

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

The compiler must know that these operations exist and must know their canonical
semantics.

Their physical implementation may still use:

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

# Core principle

```text
Compiler-known describes semantic authority.

It does not require every operation to lower to one machine instruction.

It does not require every operation to be absent from core source.

It does not permit the compiler to bypass ordinary ownership, effect, lifetime,
allocation, or unsafe rules.
```

---

# Normative ownership

A compiler-known member has exactly one normative semantic owner.

This document owns:

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

Another rulebook may own detailed domain semantics.

Examples:

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

Other documents must refer to this rulebook rather than create a second
compiler-known-member registry.

---

# Compiler-known categories

A member registry entry has one of these semantic categories.

## Intrinsic property

A property whose value is derived directly from compiler-known representation or
semantic metadata.

Examples:

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

## Intrinsic operation

A method whose semantics require explicit compiler representation.

Examples:

```text
SizeOf
RawPtr.Read()
RawPtr.Write()
RawPtr.Offset()
Arena.Reset()
Arena.Release()
```

## Compiler-known core method

A method whose existence and contract are known by the compiler, but whose
ordinary implementation may be supplied by privileged core source.

Examples:

```text
ToString()
ToByteArray()
ToCharArray()
ToRuneArray()
Fill(value)
```

Only methods explicitly listed as compiler-known belong to this category.

An ordinary core method does not become compiler-known merely because it is
implemented in core.

## Compiler-known associated function

An associated function available without ordinary user imports and attached to
a compiler-known type.

Examples:

```text
string.FromByteArray(...)
string.FromRuneArray(...)
Arena.FromBuffer(...)
Arena.WithCapacity(...)
```

## Compiler-known global function

A compiler-owned function resolved without an import.

Required in Sec 0.1:

```sec
len(value)
```

## Internal intrinsic operation

A compiler operation with no required public source spelling.

Examples may include:

```text
AlignOf(T);
materialize addressable storage;
construct a slice descriptor;
extract an ArenaDomain;
validate generation;
obtain target ABI classification.
```

Internal intrinsic operations are not automatically callable by user code.

---

# Compiler-known does not mean globally visible syntax

The parser uses ordinary syntax for public compiler-known members.

Examples:

```sec
value.Ptr
value.Len
value.SizeOf
string.FromRuneArray(runes)
```

The parser must not introduce special AST node kinds based only on the spelling.

The parser creates ordinary:

```text
member access;
member call;
associated member access;
associated call;
global call.
```

Semantic analysis resolves the expression to a compiler-known registry entry.

Semantic IR then records the intrinsic semantic operation explicitly where
required.

---

# Registry

The compiler must maintain one canonical compiler-known-member registry.

The registry must not be distributed as unrelated string comparisons across
Sema, LSP, lowering, and backend code.

A conceptual entry contains:

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

This structure is illustrative.

All implementation code is written in English.

The real registry may use generated tables, typed declarations, or another
stable representation.

---

# Stable member identity

Every compiler-known member has a stable semantic ID.

The ID is not derived solely from its display name.

The ID must distinguish overloads and receiver categories.

Examples:

```text
CKM-PTR-VALUE
CKM-LEN-STRING
CKM-LEN-ARRAY
CKM-LEN-SLICE
CKF-LEN
CKM-SIZEOF-VALUE
CKM-SIZEOF-TYPE
CKM-TOSTRING-STRING
CKM-TOSTRING-RUNE-SEQUENCE
CKM-STRING-TOBYTEARRAY
CKM-STRING-TOCHARARRAY
CKM-STRING-TORUNEARRAY
```

Exact numbering belongs to the compiler registry.

---

# Source naming convention

Sec public methods and public compiler-known properties use the canonical member
casing selected by this rulebook.

For the members defined here:

```text
Ptr
Len
IsEmpty
Capacity on list
SizeOf
Fill
ToString
ToByteArray
ToCharArray
ToRuneArray
```

The global function remains:

```text
len
```

There is no public `Length` alias in Sec 0.1.

Rationale:

```text
Ptr and Len form one compact low-level naming pair;
Length would duplicate Len;
duplicate aliases increase language, formatter, LSP, test, and documentation
surface;
the global len function has a distinct signed-index purpose and existing
bootstrap usage.
```

---

# Legacy lowercase member spellings

The repository currently contains lowercase intrinsic spellings:

```text
.ptr
.len
```

The canonical Sec 0.1 public spellings are:

```text
.Ptr
.Len
```

Lowercase spellings are classified as migration spellings.

They are not separate semantic members.

The compiler may temporarily accept them during bootstrap migration.

When accepted, they resolve to the same stable member ID as the canonical
spelling.

A compatibility diagnostic should state:

```text
information: lowercase compiler-known member spelling is deprecated

use:
    value.Ptr
```

or:

```text
use:
    value.Len
```

The formatter may rewrite legacy lowercase spellings only when compatibility
mode is enabled and semantic resolution is unambiguous.

The final `docs-v0.1.0` normative examples must use canonical spellings.

---

# Reserved member names

The following universal compiler-known member names cannot be redeclared with a
different meaning on a type where the universal member applies:

```text
Ptr
SizeOf
```

`Len`, `IsEmpty`, and `Fill` are reserved on the compiler-known collection
receiver categories where their canonical properties or operation apply.

The global function name `len` is compiler-owned and cannot be redeclared or
overloaded by user code.

`ToString` is different:

```text
built-in and related types may receive compiler/core-defined ToString;
user-defined nominal types may define their own ToString implementation;
an explicit user implementation takes precedence over eligible inherited
underlying-type behavior.
```

String conversion-array methods are reserved on built-in `string`.

User-defined types may use the same method names for their own ordinary
semantics unless another interface contract restricts them.

---

# Lookup order

Member lookup follows this conceptual order:

```text
1. exact user-defined member on the nominal type, when permitted;
2. exact compiler-known member for the exact semantic type;
3. exact privileged core member for the exact built-in type;
4. eligible compiler/core member inherited from a related underlying type;
5. interface or generic-constraint member;
6. no member.
```

This order must not allow a user module to override representation-sensitive
members on compiler-owned built-in types.

Ordinary user modules may not add `impl` blocks to compiler-owned built-in
types.

Privileged core may do so through the canonical core loading mechanism.

---

# Built-in type member lookup

Compiler-owned built-in types participate in ordinary member lookup.

The compiler must expose registry entries through the same semantic member model
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

LSP must not maintain a separate hard-coded list.

---

# Named and related types

A named type remains nominally distinct from its underlying type.

A compiler-known or core member may be inherited only when:

```text
the operation remains representation-compatible;
nominal identity is not erased;
contracts are not bypassed;
unit semantics are not bypassed;
no implicit conversion is introduced;
parameter and result substitution remains valid;
unsafe obligations remain unchanged.
```

Examples that are normally eligible:

```sec
type CustomerID uint64
let text := customerID.ToString()
let bytes := customerID.SizeOf
```

Examples that require special care:

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

A `distinct` type may restrict inherited behavior according to the distinct-type
rulebook.

---

# Generic lookup

Compiler-known members participate in generic checking only when the generic
contract guarantees their availability.

The compiler must not assume that every unconstrained `T` has:

```text
Len;
ToString;
Ptr.
```

`SizeOf` is available only when `T` is statically sized for the active
CompilationPlan.

`Ptr` additionally requires an addressable value expression.

A future interface may express printable or sequence behavior.

This rulebook does not invent that interface syntax.

---

# Effects and compiler-known members

Compiler-known members remain subject to effect analysis.

A compiler-known member may be:

```text
effect-free;
MayAllocate;
MayPanic;
MayAccessVolatile;
MayMutateExternalState;
unsafe;
ordered through Arena effects.
```

The compiler must not classify an operation as pure merely because it is
intrinsic.

Examples:

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
```

---

# Semantic IR representation

A compiler-known operation receives an explicit Semantic IR operation or stable
intrinsic-call identity when its semantics matter after Sema.

The compiler must not lower all compiler-known members as generic unresolved
calls.

Required explicit distinctions include:

```text
GetPointer
GetLength
GetSignedLength
SizeOfType
SizeOfValue
StringIdentity
StringToByteArray
StringToCharArray
StringToRuneArray
RuneSequenceToString
CharSequenceToString
Arena operations
RawPtr operations
```

Exact operation names are implementation-defined.

The semantic distinctions are required.

---

# Sec MLIR path

The intended lowering path is:

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

A compiler-known operation may lower to a core/helper call when that preserves
the semantic operation and all analysis facts.

---

# Compiler-known helper calls

A helper implementation does not own semantics.

Example:

```text
semantic operation:
    StringToRuneArray

selected implementation:
    core string decoder helper
```

The call graph records the helper when it is a real callable.

Effect analysis includes helper effects.

Inlining or replacing the helper does not change the source member contract.

---

# `Ptr`

## Canonical spelling

```sec
value.Ptr
```

`Ptr` is an unsafe read-only compiler-known property.

Conceptual form:

```sec
unsafe property Ptr: RawPtr[T] {
    get
}
```

The exact result type depends on the receiver category.

---

# `Ptr` on addressable scalar values

For an addressable value of exact semantic type `T`:

```sec
unsafe {
    let pointer := value.Ptr
}
```

the result is:

```text
RawPtr[T]
```

The pointer refers to the current storage representation of that value.

The source expression must be addressable.

---

# `Ptr` on strings

For `string`:

```sec
unsafe {
    let pointer := text.Ptr
}
```

the result is:

```text
RawPtr[byte]
```

It refers to the first encoded storage byte of the string view or representation
selected by the active CompilationPlan.

`text.Len` is the number of accessible encoded bytes.

`Ptr` does not imply a trailing zero byte.

A foreign API requiring a terminator needs an explicit materializing operation.

The source string remains immutable.

Obtaining `RawPtr[byte]` does not grant semantic permission to modify immutable
string storage.

An unsafe write through the pointer must satisfy all immutability, ownership,
representation, and FFI obligations.

---

# `Ptr` on arrays

For an addressable array of `T`:

```sec
unsafe {
    let pointer := values.Ptr
}
```

the result is:

```text
RawPtr[T]
```

It refers to the first element storage when the array has at least one element.

The array must have stable addressable storage.

The pointer does not contain the array length.

The same property is available on owning dynamic arrays `T[]` and `list[T]`.
It refers to their current contiguous element backing. It does not expose
capacity, transfer ownership, or stabilize storage across a mutating operation.

`map[K, V]` and `set[T]` do not have compiler-known `Ptr`: their physical
storage is not a public contiguous sequence contract.

---

# `Ptr` on slices

For a shared or mutable slice:

```sec
unsafe {
    let pointer := view.Ptr
}
```

the result is:

```text
RawPtr[T]
```

It refers to the first represented element when `view.Len > 0`.

The pointer does not carry:

```text
bounds;
lifetime;
generation;
ownership;
mutability;
slice length.
```

These remain properties of the safe slice and compiler analysis.

---

# Empty `Ptr`

For an empty string, array, or slice:

```text
Len == 0;
Ptr must not be dereferenced;
the physical pointer value may be null, a sentinel, a base pointer, or another
target-valid representation.
```

No source program may depend on one universal empty-pointer bit pattern.

Pointer equality on empty views does not establish shared storage identity.

---

# `Ptr` addressability

Normally addressable:

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

Normally not addressable:

```text
pure arithmetic temporaries;
values existing only as SSA values;
unmaterialized conversions;
unmaterialized function results;
compile-time-only values;
types without an address-space representation.
```

The compiler must not silently allocate or extend a lifetime merely to satisfy
`Ptr`.

A target may materialize a value only when ordinary language semantics already
require storage.

---

# `Ptr` unsafe obligations

Accessing `Ptr` requires explicit unsafe context.

`Ptr`:

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

Using the pointer after the source storage becomes invalid is unsafe and
invalid.

---

# `Ptr` and Arena storage

A pointer obtained from an Arena-backed value is valid only while:

```text
the ArenaDomain remains live;
the allocation epoch remains valid;
the allocation remains within bounds;
the storage is not invalidated by Reset or Release;
the FFI or unsafe operation obeys the retention contract.
```

`RawPtr[T]` does not block Arena Reset or Release by itself.

Code retaining the raw pointer must establish an explicit dependency through
the foreign/unsafe contract.

---

# `Ptr` and volatile storage

When the source storage is volatile or addressed, pointer access preserves the
address-space and volatile obligations.

The raw pointer does not convert volatile storage into ordinary memory.

Loads and stores must lower according to the target's volatile rules.

---

# `Len`

## Canonical spelling

```sec
value.Len
```

`Len` is a read-only compiler-known property.

The result type is:

```text
uint
```

There is no `Length` alias in Sec 0.1.

---

# `Len` on strings

For `string`:

```sec
let byteCount := text.Len
```

`Len` is the number of encoded bytes accessible through the string's byte view.

It is not:

```text
rune count;
char count;
grapheme-cluster count;
display-column width.
```

String iteration and conversion members provide higher-level text semantics.

---

# `Len` on fixed arrays

For a fixed array `T[N]`:

```sec
let count := values.Len
```

the result is the compile-time constant:

```text
N
```

The result type is `uint`.

---

# `Len` on owning dynamic arrays

For an owning dynamic array or sequence `T[]`:

```sec
let count := values.Len
```

the result is the current element count.

It is not the allocated capacity unless the collection rulebook explicitly
defines both values as equal.

No implicit `Capacity` property is created by this rulebook.

---

# `Len` on slices

For `ref T[]` and `ref mut T[]`:

```sec
let count := view.Len
```

the result is the number of elements represented by the view.

It is a runtime value unless statically proven.

It is not the byte size.

The represented byte payload may be computed from layout only when safe checked
multiplication is valid:

```text
view.Len * SizeOf(T)
```

---

# `Len` on library collections

For `list[T]`, `map[K, V]`, and `set[T]`, `Len` is the logical element or entry
count. It is never capacity or backing-storage size.

---

# `Len` evaluation

`Len`:

```text
evaluates the receiver exactly once;
does not allocate;
does not consume the receiver;
does not mutate the receiver;
does not read sequence elements;
may be folded when statically known.
```

For a volatile sequence descriptor, reading descriptor metadata follows the
volatile rulebook when the descriptor itself is volatile.

---

# `IsEmpty`

`IsEmpty` is a read-only compiler-known property on fixed arrays, owning
dynamic arrays, slices, `list[T]`, `map[K, V]`, and `set[T]`:

```sec
if values.IsEmpty {
    return
}
```

It is equivalent to `values.Len == 0`, evaluates the receiver exactly once,
and does not allocate, consume, or inspect elements. `IsEmpty()` is not a
second callable spelling.

---

# Global `len`

Sec 0.1 retains the compiler-known global function:

```sec
len(value)
```

Required conceptual overloads:

```sec
fn len(value: string) int

fn len[T](value: T[]) int

fn len[T](value: ref T[]) int

fn len[T](value: ref mut T[]) int
```

The compiler infers `T`.

`len` returns `int`.

Its purpose is signed index and offset arithmetic.

Example:

```sec
let index := position + offset

if index < 0 || index >= len(input) {
    return 0r
}
```

`len(value)` and `value.Len` represent the same logical count for the same
value, with different result types:

```text
value.Len:
    uint

len(value):
    int
```

The compiler performs a checked representability proof where the sequence length
might exceed `int.max`.

A CompilationPlan that permits a sequence larger than `int.max` must:

```text
reject len(value) when representability cannot be proven;
or
define a checked Result form in a later rulebook.
```

Sec 0.1 does not silently wrap.

---

# `len` restrictions

`len`:

```text
evaluates its argument exactly once;
does not allocate;
does not consume;
does not mutate;
cannot be redeclared;
cannot be user-overloaded.
```

It accepts only canonical compiler-known sequence categories.

An arbitrary user type with a method named `Len` is not automatically accepted
by global `len`.

A future interface-based overload requires a separate rule.

---

# `SizeOf`

`SizeOf` is a compiler-known layout query.

Required forms:

```sec
let valueSize := value.SizeOf
let typeSize := TypeName.SizeOf
let queriedSize := SizeOf(TypeName)
```

The result type is:

```text
uint
```

The associated type property and the global type query return physical storage
size for one value of the type. The instance property follows the category
rules below.

The semantic size comes from `layout.md`.

---

# Value-form `SizeOf`

Example:

```sec
let size := myInt.SizeOf
```

The size is determined from the exact semantic type of the receiver.

The receiver expression follows ordinary evaluation rules and is evaluated
exactly once.

Its stored value is not inspected to determine the result.

Therefore:

```sec
let size := MakeValue().SizeOf
```

still evaluates `MakeValue()` and preserves its effects.

The compiler may eliminate receiver materialization only when normal
effect/ownership analysis permits it.

---

# Type-form `SizeOf`

Example:

```sec
let size := int64.SizeOf
```

The type form has no value receiver.

It is a compile-time constant for one concrete CompilationPlan when the type has
complete layout.

Generic type form:

```sec
let size := T.SizeOf
```

is valid only when the generic contract guarantees that `T` is sized and has
complete layout at specialization.

---

# `SizeOf` meaning by category

## Scalar value

Returns the physical storage size of the scalar type.

## Struct, enum, union, register, and named type

Returns the complete physical size including layout-required padding.

It uses the exact nominal type layout.

## Fixed array

Returns the contiguous element payload size, including layout-required element
stride.

## Owning dynamic array

Returns `value.Len * SizeOf(T)`: the initialized element payload bytes. It does
not return descriptor size or reserved capacity.

## Slice

Returns `value.Len * SizeOf(T)`: the represented payload bytes. It does not
return the slice/reference descriptor size.

## List

Returns `value.Len * SizeOf(T)`: the initialized contiguous payload bytes. It
does not include capacity, allocation headers, or the owner descriptor.

## String

Returns the physical size of the string value or descriptor.

It does not return `string.Len`.

## Safe reference

Returns the physical representation size selected by the active profile.

The logical reference model remains unchanged.

## `RawPtr[T]`

Returns the target raw-pointer representation size.

## Arena

Returns the physical Arena owner/descriptor size for the concrete
CompilationPlan after layout selection.

It does not include owned backing capacity.

---

# Unsized and incomplete types

`SizeOf` is invalid when:

```text
the type is unsized;
layout is incomplete;
layout depends on unresolved target selection;
the type has no physical runtime representation;
the value exists only at compile time.
```

The diagnostic must identify the missing layout fact.

`void.SizeOf` is invalid unless a later layout rule explicitly assigns a
representation.

---

# `SizeOf` and target plans

`SizeOf` is evaluated per concrete `CompilationPlan`.

The same source type may have different size on:

```text
linux/amd64;
linux/arm32;
bare-metal/cortex-m4;
another ABI/profile.
```

A multi-output build evaluates and verifies the expression separately for every
output.

Cross-plan tooling may report differing values.

It must not merge them into one runtime value.

---

# `SizeOf` effects

The layout result itself:

```text
does not allocate;
does not mutate;
does not consume;
does not access the stored value;
does not require unsafe.
```

Value-form receiver evaluation may still carry effects.

Type-form has no receiver effects.

---

# Contextual `fill`

`fill` is a compiler-known contextual construction form, not an ordinary
globally overloadable function. Its expected destination type selects one of
these forms:

```sec
let fixed: T[N] := fill(value)
let owned: T[] := try fill(value, count)
let text: string := try fill(fragment, count)
```

The fixed-array form initializes exactly `N` elements. The owning-array form
allocates and initializes exactly `count` elements and may fail according to
the active allocation context. The string form produces `count` repetitions of
the supplied fragment according to the string encoding rules. The value
expression is evaluated once and each stored element is copied from that value,
so element type `T` must be copyable unless a later rule defines a distinct
factory form.

A mutable slice also has the compiler-known operation:

```sec
view.Fill(value)
```

It overwrites every represented element in index order. `T` must be copyable;
the receiver must be `ref mut T[]`; the operation neither reallocates nor
changes `Len`. Shared slices have no `Fill`.

The registry must give contextual `fill` and mutable-slice `Fill` stable,
distinct semantic identities so Sema, LSP, Semantic IR, and lowering do not
infer them from spelling alone.

---

# `AlignOf`

The compiler requires an internal layout query equivalent to:

```text
AlignOf(T)
```

for allocation, ABI, fields, arrays, and Arena lowering.

Sec 0.1 does not require a public `AlignOf()` member.

A public member may be added only through an explicit rulebook update.

Internal `AlignOf` must not be exposed accidentally through LSP completion.

---

# `ToString()`

`ToString()` is a required compiler-known core method on fundamental printable
types.

The compiler knows:

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

Core may provide helper implementations.

Target code may provide optimized helpers.

---

# Required built-in `ToString()` surface

Sec 0.1 requires no-argument `ToString()` on:

```text
string;
bool;
signed integers;
unsigned integers;
byte;
binary floating-point types;
decimal types;
char;
rune;
eligible named types related to these types.
```

Numeric formatting overloads may also include:

```sec
value.ToString(format)
```

The exact format grammar belongs to the formatting rulebook.

---

# `string.ToString()`

For `string`:

```sec
let result := text.ToString()
```

the result is the same semantic string value.

It:

```text
does not allocate;
does not copy encoded bytes;
does not consume the string;
does not mutate the string;
preserves storage identity where the string model exposes it;
is included for generic formatting consistency.
```

The compiler may fold the operation to identity.

---

# Boolean `ToString()`

`bool.ToString()` produces the canonical text representation defined by core.

The canonical English forms for Sec 0.1 are:

```text
true
false
```

The operation is culture-independent.

Locale-aware formatting belongs to a higher library.

---

# Numeric `ToString()`

No-argument numeric `ToString()` produces the canonical culture-independent Sec
representation of the value.

The representation must:

```text
round-trip through the matching parser where the numeric grammar supports it;
not depend on process locale;
not insert grouping separators;
use canonical sign and exponent syntax;
preserve enough precision for the type's canonical round-trip rule.
```

Exact floating-point and decimal formatting algorithms belong to the formatting
and numeric rulebooks.

The operation may lower to core helpers.

---

# `char.ToString()` and `rune.ToString()`

`char.ToString()` materializes the canonical string representation of that
`char`.

`rune.ToString()` encodes exactly one valid Unicode scalar value into a string.

A `rune` is already guaranteed to be a valid Unicode scalar under the type
rules.

The operations:

```text
do not consume the scalar;
do not mutate the scalar;
may require string materialization;
use the active allocation/string-storage policy;
must preserve the exact scalar value.
```

The canonical source surface retains the existing no-argument form returning
`string`.

A CompilationPlan that cannot provide the required materialization strategy must
reject the operation rather than silently select an unrelated heap.

---

# User-defined `ToString()`

A user-defined nominal type may define:

```sec
impl Customer {
    fn ToString() string {
        // ...
    }
}
```

The method is an ordinary user implementation.

It is not converted into an intrinsic merely because its name is `ToString`.

The explicit nominal implementation takes precedence over inherited
underlying-type `ToString()`.

The compiler still records its callable effects normally.

An interface may later require `ToString()`.

That interface does not change the compiler-known built-in registry.

---

# Rune and char sequence `ToString()`

Arrays and slices whose element type is exactly `rune` provide:

```sec
value.ToString()
```

Arrays and slices whose element type is exactly `char` also provide:

```sec
value.ToString()
```

Examples:

```sec
let runes: rune[2] := ['A', 'B']
let first := runes.ToString()

let chars := "test string".ToCharArray()
let second := chars[0..<4].ToString()
```

The operation:

```text
uses only the represented elements;
does not search for a terminator;
does not consume or mutate the source;
materializes one string;
preserves element order;
does not exist as a general array formatting operation.
```

`byte[]` and byte slices do not automatically receive `ToString()` because byte
data does not by itself establish encoding.

Use:

```sec
string.FromByteArray(bytes)
```

or another explicit decoder.

---

# Char-sequence semantics

`char` sequence conversion follows the canonical `char` model in `types.md`
and the string rulebook.

Each `char` is converted according to the same semantics as
`char.ToString()`.

The operation is not implicitly defined as:

```text
UTF-8 byte reinterpretation;
UTF-16 code-unit reinterpretation;
grapheme segmentation;
zero-terminated character scanning.
```

If the final `char` model restricts representable text, the compiler must
diagnose or use the canonical conversion behavior defined there.

This rulebook requires consistency between:

```text
string.ToCharArray();
char.ToString();
char-array/slice ToString();
string char iteration where defined.
```

---

# Rune-sequence semantics

Each `rune` is a Unicode scalar value.

Rune array/slice `ToString()` encodes the represented scalar sequence in order.

It does not:

```text
normalize Unicode;
change case;
replace valid scalars;
append a terminator.
```

The result represents exactly the input scalar sequence.

---

# Materialization and allocation context

A materializing `ToString()` operation may require allocation or another
string-storage provider.

The compiler records:

```text
MayAllocate where applicable;
RequiresAllocationContext where applicable;
exact source operation;
selected provider/helper.
```

This rulebook does not permit hidden fallback to an unrelated heap.

The exact failure surface and owning string representation must remain
synchronized with:

```text
allocation.txt;
arena.md;
string memory rules;
core-library.md.
```

The no-argument source signatures remain the canonical signatures already
required by the core surface.

A profile incapable of satisfying the operation rejects its use.

---

# `ToByteArray()`

Required on `string`:

```sec
let bytes := text.ToByteArray()
```

It materializes an owning `byte[]`.

The result contains exactly the encoded bytes represented by the string.

It:

```text
preserves byte order;
copies or otherwise produces independent owning array storage;
does not append a zero terminator;
does not include unused capacity;
does not expose the string's immutable backing as mutable aliases;
does not consume or mutate the source.
```

For an empty string it returns an empty owning array.

The result length equals:

```text
text.Len
```

when `text.Len` is representable as the owning-array element count.

---

# `ToByteArray()` and encoding

`ToByteArray()` is not a text transcoder.

It returns the string's canonical encoded byte sequence.

The string rulebook must define that encoding before documentation freeze.

Low-level FFI code requiring another encoding uses an explicit transcoding API.

---

# `ToByteArray()` allocation

`ToByteArray()` is materializing.

It may:

```text
MayAllocate;
RequireAllocationContext;
use Arena-backed owning array storage;
lower to a bulk copy;
be folded for compile-time constants when ownership remains correct.
```

The returned array is independently owned.

Mutating it does not mutate the source string.

---

# `ToCharArray()`

Required on `string`:

```sec
let chars := "test string".ToCharArray()
```

It materializes an owning `char[]`.

The produced elements follow the canonical `char` interpretation of the string.

The operation:

```text
preserves logical order;
does not append a terminator;
does not expose string backing as mutable aliases;
does not consume or mutate the source;
returns an empty array for an empty string.
```

The result count is the number of canonical `char` elements, not necessarily the
encoded byte length.

The exact relationship between `char`, Unicode scalar values, and string
iteration is owned by the type and string rulebooks.

This document requires all related operations to agree.

---

# `ToRuneArray()`

Required on `string`:

```sec
let runes := "test string".ToRuneArray()
```

It materializes an owning `rune[]`.

The result contains one `rune` for every Unicode scalar represented by the
string, in source order.

It:

```text
does not include encoded continuation bytes as separate runes;
does not append a terminating rune;
does not normalize;
does not change case;
does not consume or mutate the source;
returns an empty array for an empty string.
```

When strings are guaranteed valid under the canonical string model, conversion
is semantically valid for every string.

If strings may contain invalid encoded byte sequences, the string rulebook must
define a Result/error or replacement policy.

This rulebook does not silently choose one.

---

# Materializing array independence

Arrays returned by:

```text
ToByteArray();
ToCharArray();
ToRuneArray();
```

are owning arrays.

They do not borrow from the source string.

The source string may be destroyed or moved according to normal ownership rules
without invalidating the returned array.

The returned array may be mutated without mutating the source.

An implementation may share immutable storage internally only when it preserves
these observable ownership and mutation semantics, including copy-on-write
requirements where applicable.

No mandatory copy-on-write runtime is introduced.

---

# Round-trip expectations

## Bytes

Where `string.FromByteArray` accepts the canonical encoded bytes:

```text
string.FromByteArray(text.ToByteArray())
```

must reconstruct equivalent string content.

The exact error surface belongs to the string constructor rule.

## Runes

```text
string.FromRuneArray(text.ToRuneArray())
```

must reconstruct equivalent scalar content.

## Chars

```text
text.ToCharArray().ToString()
```

must reconstruct equivalent string content when every string value is fully
representable by the canonical `char` model.

When that condition does not hold, the char/string rulebook must define the
failure or exclusion explicitly.

---

# No duplicate iterator names

Materializing conversions are:

```text
ToByteArray
ToCharArray
ToRuneArray
```

Names such as:

```text
Bytes
Chars
Runes
```

must not be aliases for the materializing operations.

They may later denote:

```text
iterators;
views;
non-owning traversal;
direct language iteration.
```

Such APIs require distinct semantics.

---

# Associated string constructors

The compiler-known/core surface includes:

```sec
string.FromByteArray(value)
string.FromRuneArray(value)
```

A `FromCharArray` constructor may be added when the canonical char/string model
is finalized.

These are associated functions, not constructor casts.

This is invalid as a replacement:

```sec
string(value)
```

when converting arrays or slices to string.

The compiler resolves the associated function through the built-in member
registry and privileged core declaration model.

---

# `FromByteArray`

`FromByteArray` interprets the input according to the canonical string encoding.

It does not assume zero termination.

It uses the represented array or slice bounds.

The exact valid input forms, ownership, allocation, and error behavior belong to
the string rulebook and core declaration.

---

# `FromRuneArray`

`FromRuneArray` encodes the represented Unicode scalar sequence.

Every `rune` is already a valid scalar.

The operation does not:

```text
read beyond the represented bounds;
look for 0r as a terminator;
normalize;
change case.
```

---

# Fundamental numeric associated properties

The canonical registry also includes representation-sensitive associated
properties already required by the core surface.

## Integer types

For each concrete signed and unsigned integer type:

```text
Min
Max
Bits
```

`Bits` is the semantic bit width.

`Min` and `Max` are compile-time constants of the exact type.

## Floating-point types

Where defined:

```text
Min
Max
Epsilon
Infinity
NegativeInfinity
NaN
```

The exact values and representability follow numeric rules.

## Decimal types

Representation-sensitive members such as:

```text
Scale
```

are compiler-known only when the decimal rulebook defines them as stable public
semantics.

Internal coefficient layout is not exposed merely because the compiler knows
it.

---

# `RawPtr[T]` compiler-known methods

The registry includes required raw-pointer operations such as:

```sec
unsafe pointer.Read()
unsafe pointer.Write(value)
unsafe pointer.Offset(elements)
unsafe pointer.AddBytes(bytes)
unsafe pointer.Difference(other)
```

Their semantics are owned by `unsafe.md` and `reference_model.md`.

They remain explicit Semantic IR operations until pointer provenance and ABI
lowering are complete.

---

# Arena compiler-known members

`Arena` members are compiler-known because they affect:

```text
ArenaDomain identity;
ownership;
validity epoch;
ordered effects;
allocation context;
task/thread dependencies;
MLIR lowering.
```

Required entries include:

```text
Arena.FromBuffer
Arena.WithCapacity
Arena.Growable where enabled
Arena.New
Arena.Alloc
Arena.Reset
Arena.Release
```

Their detailed semantics are owned by `arena.md`.

---

# Core declarations

A compiler-known core method may have a privileged core declaration.

Example:

```sec
impl string {
    fn ToString() string {
        return self
    }
}
```

The declaration provides:

```text
ordinary source visibility;
documentation;
callable body where needed;
overload shape;
LSP definition location.
```

The compiler-known registry provides:

```text
stable semantic identity;
built-in eligibility;
intrinsic classification;
required analysis facts;
fallback when body is unavailable;
target/helper lowering.
```

The two must agree.

A mismatch is a compiler/core build error.

---

# Registry and core validation

During compiler/core bootstrap, validate:

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

The compiler must reject a privileged core declaration that contradicts the
registry.

The registry must not silently rewrite an incompatible core declaration.

---

# Core helper absence

A compiler-known intrinsic may remain usable when no ordinary core body exists,
provided the selected CompilationPlan has a valid intrinsic lowering.

A compiler-known core method requiring a helper body is unavailable when neither:

```text
a valid core implementation;
nor
a valid intrinsic/target lowering
```

exists.

The diagnostic identifies the missing implementation capability.

---

# User shadowing

An ordinary local variable, field, or method name may textually match a
compiler-known name in a scope where no compiler-known receiver pattern applies.

Example:

```sec
type Report struct {
    Len: int
}
```

Whether this field is legal follows ordinary naming and property rules.

It must not alter array, slice, or string `Len` semantics.

Universal member conflicts on eligible types are rejected.

---

# Overload resolution

Compiler-known methods participate in ordinary overload resolution.

Examples:

```sec
value.ToString()
value.ToString("N2")
```

are distinguished by parameters, not return type alone.

Compiler-known candidates carry stable IDs through overload resolution.

Diagnostics must identify whether a candidate came from:

```text
compiler intrinsic;
privileged core;
user impl;
related-type inheritance;
interface contract.
```

---

# Addressability diagnostics

Invalid:

```sec
unsafe {
    let pointer := (a + b).Ptr
}
```

when the temporary has no materialized stable storage.

Diagnostic:

```text
error: Ptr requires an addressable value

the expression exists only as a temporary SSA value

help:
    store the value in a local binding before obtaining its pointer
```

The compiler must not materialize storage solely to make the expression legal.

---

# `Len` diagnostics

Invalid:

```sec
let count := number.Len
```

Diagnostic:

```text
error: type `int` has no compiler-known Len property

Len is available on:
    string
    arrays
    owning sequences
    shared slices
    mutable slices
```

A user-defined ordinary `Len` method/property may still exist on its own nominal
type.

---

# `SizeOf` diagnostics

Invalid:

```sec
let size := UnsizedType.SizeOf
```

Diagnostic:

```text
error: SizeOf requires a complete sized layout

type:
    UnsizedType

missing:
    concrete layout for the active CompilationPlan
```

For multi-output builds, report the failing plan.

---

# String-conversion diagnostics

Diagnostics must distinguish:

```text
member unavailable;
allocation context unavailable;
target materialization unsupported;
invalid string encoding;
char representation mismatch;
result ownership unsupported;
missing core/helper implementation.
```

Example:

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

Exact help depends on the available APIs.

---

# Legacy spelling diagnostics

When legacy compatibility is enabled:

```sec
value.ptr
value.len
```

resolve to canonical members and produce configurable information.

When compatibility is disabled, they are ordinary unresolved member names.

The diagnostic should not suggest both `Len` and `Length`.

Only the canonical spelling is suggested.

---

# Compile-time evaluation

Compiler-known members may be compile-time evaluable when all required inputs
are compile-time known and the operation is permitted by CTFE.

Always or normally CTFE-compatible:

```text
fixed-array Len;
type-form SizeOf;
numeric Min;
numeric Max;
numeric Bits;
layout constants.
```

Conditionally CTFE-compatible:

```text
value-form SizeOf after preserving receiver effects;
string Len for compile-time string;
ToString for compile-time scalar;
string materialization conversions when CTFE storage rules support owning
results.
```

Not normal CTFE:

```text
Ptr to runtime storage;
RawPtr dereference;
Arena allocation;
target-provided runtime helpers;
volatile operations.
```

Compile-time evaluation does not change source semantics.

---

# Allocation-context integration

Materializing compiler-known methods record whether they:

```text
MayAllocate;
RequireAllocationContext;
accept an explicit Arena through another overload;
can be compile-time materialized;
are unavailable in a no-allocation profile.
```

The compiler-known registry must not embed one universal heap provider.

Allocation origin is resolved through `allocation.txt` and `arena.md`.

---

# Ownership and borrow integration

Registry entries specify whether the receiver is:

```text
shared;
mutably borrowed;
consumed;
addressability-only;
compile-time type-only.
```

Examples:

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

Sema and Semantic IR use these facts.

---

# Call-graph integration

A compiler-known member may be:

```text
semantically atomic;
lowered to a real helper call;
implemented by privileged core;
target-dispatched;
open through a user-defined ToString implementation.
```

The call graph records actual callable helpers when they matter.

It may represent a fully atomic intrinsic through a summary only when this is
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

# LSP

The LSP consumes the canonical compiler registry.

It must provide:

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

Examples:

```text
Len
    compiler-known property
    result: uint
    string meaning: encoded byte length

Ptr
    unsafe compiler-known property
    result: RawPtr[byte]
    does not extend lifetime

SizeOf
    compiler-known property or global type query
    result for linux/amd64: 16 bytes
```

---

# Synthetic definitions

When no ordinary core declaration exists, LSP may navigate to a synthetic
read-only definition generated from the registry.

The synthetic definition must state:

```text
compiler-known;
normative rulebook section;
receiver pattern;
signature;
effects;
unsafe requirement;
target restrictions.
```

It must not pretend that generated text is user source.

---

# Formatter

The formatter:

```text
preserves canonical member casing;
may migrate legacy ptr/len spelling under explicit migration mode;
does not rewrite ordinary user members merely by name;
preserves unsafe blocks;
does not convert len(value) into value.Len or the reverse;
does not change SizeOf value/type form.
```

`len(value)` and `value.Len` have different result types and are not formatter
style aliases.

---

# Documentation generation

Manuals and API references may be generated from the registry plus privileged
core declarations.

Generated documentation must distinguish:

```text
compiler intrinsic;
compiler-known core;
ordinary core;
standard library;
user implementation.
```

Derived documentation is not normative.

---

# Separate compilation

Module metadata records compiler-known usage through stable IDs.

It must not serialize only display strings.

Metadata may include:

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

A registry-version mismatch invalidates incompatible module metadata.

---

# Incremental compilation

Changes to these facts invalidate affected semantic and lowering results:

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

Unrelated registry entries should retain stable IDs.

---

# Required registry inventory

Sec 0.1 must include at least the following public entries.

## Universal or representation-sensitive

```text
Ptr
SizeOf
```

## Sequence length

```text
Len
IsEmpty
```

## Collection initialization and mutation

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

Availability and result types are receiver-specific and are defined by
`collections.md`. Listing a name here does not make it universal: for example,
`Ptr` is absent on map/set, `Fill` is limited to mutable slices, and dynamic
arrays expose no `Capacity`.

## String and formatting

```text
ToString
ToByteArray
ToCharArray
ToRuneArray
string.FromByteArray
string.FromRuneArray
char-sequence ToString
rune-sequence ToString
```

## Numeric representation

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

## Raw pointers

```text
Read
Write
Offset
AddBytes
Difference
```

## Arena

```text
FromBuffer
WithCapacity
Growable where enabled
New
Alloc
Reset
Release
```

Other compiler-known entries may exist when owned by their canonical
rulebooks.

The registry is not limited to this minimum list.

---

# Excluded aliases in Sec 0.1

The following are not canonical aliases:

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

A later rulebook may add a distinct operation when its semantics are genuinely
different.

---

# Required source tests

Create or update:

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

# Registry tests

Test:

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

# `Ptr` tests

Test:

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

# `Len` and `len` tests

Test:

```text
string byte length;
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

# `SizeOf` tests

Test:

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

# String materialization tests

Test:

```text
string identity ToString;
bool ToString;
numeric ToString;
char ToString;
rune ToString;
char array ToString;
char slice ToString;
rune array ToString;
rune slice ToString;
byte array ToString rejection;
selected slice bounds;
no terminator scan;
empty input;
ToByteArray independence;
ToCharArray independence;
ToRuneArray independence;
round trip;
allocation-context requirement;
noalloc profile rejection;
compile-time constant folding.
```

---

# Semantic IR tests

Golden tests must verify explicit operations for:

```text
GetPointer;
GetLength;
GetSignedLength;
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

The tests must verify source locations and stable member IDs.

---

# MLIR tests

Test:

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

# Diagnostic tests

Verify:

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

# Implementation plan

## Phase 1 — Registry foundation

1. Add stable `CompilerKnownMemberID`.
2. Add one canonical registry.
3. Replace scattered string checks gradually.
4. Add canonical and legacy spellings.
5. Add synthetic declaration metadata.
6. Validate registry uniqueness.

## Phase 2 — Unified member resolution

1. Resolve compiler-known and core members through one member system.
2. Preserve origin category.
3. Integrate overload resolution.
4. Integrate named-type inheritance.
5. Integrate generic constraints.
6. Reject illegal user overrides.

## Phase 3 — Core validation

1. Load privileged core declarations.
2. Match them to registry entries.
3. Validate signatures.
4. Validate unsafe and ownership behavior.
5. Validate effect declarations.
6. Report compiler/core mismatch.

## Phase 4 — Fundamental members

Implement in this order:

```text
Len;
global len;
Ptr;
SizeOf;
string identity ToString;
char/rune sequence ToString;
ToByteArray;
ToCharArray;
ToRuneArray;
numeric representation properties;
remaining ToString implementations.
```

## Phase 5 — Semantic IR

1. Add explicit intrinsic operations.
2. Preserve stable member IDs.
3. Preserve effects.
4. Preserve allocation context.
5. Preserve unsafe provenance.
6. Preserve target/layout dependencies.

## Phase 6 — Sec MLIR

1. Add high-level operations.
2. Add verifiers.
3. Add constant folding.
4. Add effect interfaces.
5. Add helper-call lowering.
6. Add target specialization.
7. Lower to standard MLIR and LLVM.

## Phase 7 — Tooling

1. LSP completion.
2. Synthetic definitions.
3. Hover and effects.
4. Legacy migration.
5. Formatter support.
6. Generated API documentation.

## Phase 8 — Migration

1. Replace `.ptr` with `.Ptr` in normative examples.
2. Replace `.len` with `.Len` in normative examples.
3. Retain `len(...)` where signed length is intended.
4. Update core and stdlib source.
5. Update bootstrap source.
6. Remove legacy compatibility after the migration gate.

---

# Implementation graph nodes

Recommended nodes include:

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

Codex must not implement the complete compiler-known-member model as one
undifferentiated task.

---

# Current repository synchronization

The repository currently contains partial direct Sema handling and partial core
stubs for several members.

The implementation inventory must verify current code before assigning status.

Known partial areas include:

```text
lowercase ptr resolution;
lowercase len properties;
global len recognition;
string ToString;
rune ToString stub;
string ToRuneArray stub;
rune-sequence ToString recognition;
Arena semantic members;
RawPtr type and operations;
numeric core declarations.
```

Implemented frontend foundation:

```text
one typed Sema-owned compiler-known member registry;
stable CKM identities for the currently exposed entries;
canonical Ptr and Len lookup with lowercase migration spellings;
global len recognition;
value-form and type-form SizeOf type resolution;
fundamental and char/rune-sequence ToString type resolution;
string ToByteArray, ToCharArray, and ToRuneArray type resolution;
string.FromByteArray and string.FromRuneArray type resolution;
integer Min, Max, and Bits and floating representation property lookup;
RawPtr Read, Write, Offset, AddBytes, and Difference validation;
Arena constructor and instance-member validation;
LSP value and static member completion sourced from the same registry;
canonical compiler-known member TextMate highlighting.
```

Known architectural work includes:

```text
canonical casing migration;
complete allocation-aware materialization;
explicit Semantic IR operations;
Sec MLIR operations;
target/profile validation;
complete diagnostics.
```

This section is informative implementation status.

It does not weaken the normative rules.

---

# Required synchronization

This rulebook must remain synchronized with:

```text
allocation.txt
arena.md
collections.md
attributes.md
call_graph.md
compiler_analysis.txt
compiler_pipeline.txt
core-library.md
default_values.md
diagnostics.txt
effect_analysis.md
ffi.txt
formatting.md
generics.txt
impl.md
layout.md
lsp.md
ownership.md
properties.md
reference_model.md
semantic_ir.txt
stdlib.md
string memory rules
target_profiles.md
types.md
unsafe.md
```

When a listed file does not yet exist, its future rulebook must consume the
canonical member definitions here.

---

# Rulebook ownership summary

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

allocation.txt and arena.md
    own allocation-context and storage allocation

effect_analysis.md
    owns inferred and declared effects

semantic_ir.txt
    owns the general IR framework

lsp.md
    owns tooling presentation, not semantic truth
```

---

# Shaped types

The canonical registry recognizes these shaped receiver families:

```text
vector[T, N]
matrix[T, Rows, Columns]
tensor[T, Dims...]
tensor[T, Shape[Rank]]
ref tensor_view[T, Rank]
ref mut tensor_view[T, Rank]
```

It also recognizes `Shape[Rank]`, `Strides[Rank]`, `TensorLayout[Rank]`,
`Axes[Rank]`, `AxisList[Count]`, `MemorySpace`, `StorageRequest`, and
`ShapedStorageRequest[Rank]` as compiler-known supporting identities.

Where meaningful for the receiver, the read-only properties are `Rank`,
`Shape`, `Len`, `Strides`, `Layout`, `MemorySpace`, `IsContiguous`, and
`SizeOf`. `Ptr` is additionally unsafe and requires addressability. These are
properties rather than zero-argument methods. Shaped `Len` is the total logical
scalar element count.

On a shaped instance, `SizeOf` is the represented logical payload byte count,
conceptually `value.Len * element payload size`. It is not descriptor size,
storage span, or unique backing-byte count. Associated `Type.SizeOf` and global
`SizeOf(Type)` remain physical layout queries.

The registry provides stable semantic identities for `Reshape`, `ToShape`,
`Materialize`, `TransferTo`, `Relayout`, `Permute`, `Transpose`, `BroadcastTo`,
`Dot`, `Outer`, `Magnitude`, `Normalize`, `Cross`, and `Contract`. The `x`
operator remains compiler-known shaped algebraic multiplication.

Ownership and effect facts distinguish borrowed non-allocating `Reshape`,
consuming non-allocating `ToShape`, new-owner `Materialize`, source-preserving
`TransferTo`, and potentially storage-producing `Relayout`.

Associated `Rank`, `Shape`, and `Len` exist only when the complete value is a
static type fact. Runtime-shaped tensors and tensor views do not synthesize
type-level runtime `Shape` or `Len` values.

Compiler, Sema, completion, hover, definition lookup, overload resolution, and
diagnostics consume this same registry. No LSP-local shaped member inventory is
permitted.

---

# Final canonical summary

Compiler-known means that the compiler owns the semantic existence and identity
of an operation even when core or target code supplies its implementation.

Compiler-known members use ordinary source syntax and ordinary AST member
expressions.

Sema resolves them through one canonical typed registry.

The canonical public low-level properties are `Ptr` and `Len`.

`Length` is not an alias.

Existing lowercase `.ptr` and `.len` are migration spellings and resolve to the
same member IDs only during compatibility migration.

The global compiler-known `len(value)` remains and returns `int` for signed index
arithmetic.

`value.Len` returns `uint`.

`Ptr` is unsafe, requires addressable storage, returns `RawPtr`, and does not
extend lifetime, preserve bounds, transfer ownership, or keep Arena storage
alive.

`SizeOf` is available in value and type form for complete sized layouts and
returns target-specific physical storage size in bytes.

Value-form receiver effects remain observable.

String `Len` is encoded byte length.

Array and slice `Len` is element count.

`ToString()` is compiler-known on fundamental printable types.

`string.ToString()` is identity.

`char` and `rune` arrays and slices provide materializing `ToString()` over the
represented range without terminator scanning.

Strings provide materializing `ToByteArray()`, `ToCharArray()`, and
`ToRuneArray()`.

The returned arrays are independently owned, preserve order, contain no implicit
terminator, and do not alias mutable string storage.

`ToByteArray()` returns canonical encoded bytes.

`ToRuneArray()` returns Unicode scalar values.

`ToCharArray()` follows the canonical Sec char model and must remain consistent
with char iteration and char-sequence `ToString()`.

Materializing operations preserve allocation-context and effect facts and never
silently fall back to an unrelated heap.

Compiler-known operations remain explicit in Semantic IR and Sec MLIR until
representation, effects, ownership, allocation, and target lowering are
complete.

The registry is the single source for compiler, core validation, LSP,
formatter, documentation generation, separate compilation, and diagnostics.

No compiler-known operation bypasses ownership, lifetime, allocation, unsafe,
effect, or target-profile rules.
