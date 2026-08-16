# Layout

## Status

This document is the canonical layout rulebook for Sec 0.1.

It defines:

- semantic layout;
- native layout;
- explicit layout contracts;
- complete and incomplete layout;
- size;
- alignment;
- stride;
- padding;
- field placement;
- fixed-array representation;
- struct representation;
- enum representation;
- tagged-union representation;
- register storage width;
- descriptor layout requirements;
- representation validity;
- zero-sized types;
- recursive layout;
- generic layout;
- packing;
- explicit alignment;
- explicit field offsets;
- endianness;
- representation observability;
- layout compatibility;
- layout queries;
- layout stability;
- Semantic IR requirements;
- lowering requirements;
- diagnostics;
- tests;
- implementation migration requirements.

This rulebook does not introduce final source syntax for explicit layout.

In particular, this document does not by itself introduce:

- a `layout` keyword;
- a `packed` keyword;
- an `align` keyword;
- an `offset` keyword;
- an `endian` keyword;
- a general representation-reinterpretation expression;
- C layout by default;
- a stable serialization format;
- a stable cross-target native layout;
- a general safe byte-view of arbitrary values.

Future source syntax may use attributes or compiler-known declarations.

Any such syntax must preserve the semantics defined here.

Adjacent rulebooks retain responsibility for their specialized areas:

```text
storage.md
    storage origin, storage lifetime, backing storage, relocation,
    invalidation, reclamation, memory spaces, and pinning

types.md
    semantic type identity, scalar families, named types, and conversions

struct.md
    struct declarations, fields, literals, properties, and field tags

collections.md
    fixed arrays, owning dynamic arrays, slice references, indexing, and slicing

enums.md
    enum declarations, values, and underlying types

unions.md
    union declarations, variants, construction, matching, and active payloads

declarations/registers.md
    register type and bit layout

platform/fixed-address-bindings.md
    addressed hardware access

compiler_known_members.md
    source-facing compiler-known layout queries such as `T.SizeOf` and `SizeOf(T)`

reference_model.md
    safe-reference guarantees and profile-selected reference representation

allocation.txt
    allocation operations and allocation failure

destruction.txt
    destruction and cleanup ordering

platform/ffi.md
    foreign ABI compatibility and foreign representation contracts

abi.md
    function calling conventions, parameter passing, and return classification

target_profiles.md
    target data layout, supported memory spaces, and target constraints

semantic_ir.txt
    complete Semantic IR requirements

effect_analysis.md
    effects and observable evaluation behavior
```

This rulebook is normative for the physical representation of materialized Sec
values within storage.

---

# Current implementation status

The current compiler contains several layout-related foundations.

## Implemented

The current compiler implements:

- semantic scalar type identities;
- fixed-width integer families through 256 bits in lexer, parser, AST, and
  semantic analysis;
- `decimal` and `decimal128` semantic types;
- MLIR representations:
  - `decimal` as `{ i64, i32 }`;
  - `decimal128` as `{ i128, i32 }`;
- named types that preserve nominal identity separately from their underlying
  representation;
- fixed-array syntax using `T[N]`;
- compile-time fixed-array lengths;
- zero-length fixed arrays;
- fixed arrays with no hidden pointer, length, capacity, or allocator field;
- owning dynamic array syntax using `T[]`;
- slice-reference syntax using `ref T[]` and `ref mut T[]`;
- struct declarations and semantic field order;
- MLIR struct lowering in declaration field order;
- enum underlying types;
- bit-backed enum width metadata;
- fieldless enum lowering to the selected underlying integer representation;
- union declaration-order variant indices;
- the conceptual union representation of tag plus largest payload storage;
- register declarations with exact semantic bit widths;
- compiler-known `SizeOf` semantics for complete layouts;
- target-plan-sensitive `SizeOf` evaluation;
- descriptor-size semantics that exclude separate backing storage;
- semantic metadata for fields, enum representation, union representation,
  register width, ABI requirements, and explicit layout attributes;
- rejection of selected direct recursive by-value union layouts;
- checked fixed-array length and total-layout arithmetic in semantic analysis.

## Partially implemented

The current implementation is partial in these areas:

- physical struct layout is currently delegated substantially to MLIR/LLVM data
  layout rather than one complete shared Sec layout phase;
- field offsets, aggregate alignment, padding ranges, and element stride are not
  represented uniformly in Semantic IR;
- `int` and `uint` are not yet lowered consistently as pointer-sized integers
  across every compiler path;
- default enum `int` lowering still uses `i32` in the existing enum backend path;
- the legacy direct LLVM decimal representation differs from the canonical MLIR
  representation;
- `T[]` is treated semantically as an owning descriptor, while selected older
  array-rule wording still calls a bare `T[]` unsized;
- complete generic aggregate layout is available only after current
  monomorphization paths instantiate the relevant type;
- union layout requirements are known conceptually but are not represented as
  one canonical resolved layout object;
- register bit widths are validated, but target-specific storage alignment and
  access-unit validation remain incomplete;
- reference and slice representations are selected by current lowering paths,
  but are not described by one canonical layout record;
- FFI rejects several non-compatible types, but complete explicit foreign-layout
  validation is not implemented;
- padding initialization and padding-leak diagnostics are incomplete;
- representation-validity metadata is incomplete;
- partial initialization is not tracked comprehensively at field, element, and
  payload granularity.

## Not implemented yet

The compiler has not yet completed:

- one shared layout-resolution phase for every concrete `CompilationPlan`;
- canonical `ResolvedLayout` objects;
- canonical `ResolvedFieldLayout` objects;
- canonical padding-range representation;
- complete target-aware alignment calculation;
- complete field-offset calculation;
- complete element-stride calculation;
- complete union tag selection and payload placement;
- explicit layout-contract validation;
- explicit aggregate alignment contracts;
- explicit total-size contracts;
- explicit field-offset contracts;
- packing semantics;
- misaligned-field reference rejection;
- target-supported unaligned access lowering;
- explicit endianness contracts;
- representation-validity classification;
- zero-initialization validity checks;
- complete recursive by-value layout diagnostics;
- generic layout caching keyed by concrete type identity and compilation plan;
- `AlignOf`;
- `StrideOf`;
- `FieldOffset`;
- native-layout stability classification;
- target-stable layout contracts;
- contract-stable layout contracts;
- layout-specific diagnostics and tests required by this rulebook;
- complete synchronization of adjacent rulebooks with this canonical model.

The current implementation facts do not override the semantics defined below.

---

# Purpose

Layout describes how a materialized value is represented within storage.

Layout answers questions such as:

```text
How many bytes does the value occupy?

What alignment does the value require?

Where does each stored field begin?

What stride separates adjacent array elements?

Where is a union tag stored?

How large must union payload storage be?

Which byte ranges are padding?

Which bit patterns represent valid values?

Which layout properties are stable?
```

Layout does not determine:

- who owns the value;
- where the storage came from;
- how long the storage remains valid;
- whether the storage may relocate;
- how the storage is allocated;
- which function registers carry a parameter;
- whether a value may cross an FFI boundary;
- how a value is serialized;
- whether a reference is valid.

Those concerns belong to their respective rulebooks.

---

# Core principles

The canonical layout principles are:

```text
Layout is resolved for one concrete CompilationPlan.

Sec distinguishes semantic layout, native layout, and explicit layout.

Materialized struct fields retain declaration order.

Native padding is not semantic data.

Physical layout similarity does not imply type compatibility.

Storage layout and callable ABI are separate concerns.

A value must have complete sized layout before it may be stored by value.

Size and alignment do not imply that every bit pattern is a valid value.

Object initialization is separate from storage layout.

Direct recursive by-value layout is invalid.

Generic layout is resolved per concrete instantiation.

Backends consume Sec-resolved layout and must not redefine it independently.
```

---

# CompilationPlan

Layout is resolved for one concrete `CompilationPlan`.

A layout-relevant compilation plan includes at least:

```text
target architecture;
target pointer width;
target data layout;
target endianness;
selected ABI;
target profile;
supported scalar alignments;
supported aggregate alignments;
supported unaligned accesses;
compiler layout rules;
explicit layout contracts;
concrete generic arguments;
selected reference representation;
selected descriptor representation.
```

The same semantic Sec type may have different native layout under different
plans.

Examples include:

```text
linux/amd64;
linux/arm32;
bare-metal/cortex-m4;
an MMIO-specific target profile;
a hardened profile with generation-bearing references;
a constrained profile with statically proven thin references.
```

A multi-output build resolves and validates layout independently for every
output plan.

One plan's resolved size or alignment must not be reused as another plan's
layout.

---

# Layout levels

Sec distinguishes three layout levels.

## Semantic layout

Semantic layout defines target-independent language guarantees.

Examples:

```text
a struct contains its stored fields;

stored struct fields retain declaration order;

T[N] contains exactly N inline elements;

fixed-array elements occur in increasing index order;

an enum has one declared underlying representation;

a union has exactly one active variant;

a union variant has one stable declaration-order index;

a register has exactly its declared semantic bit width;

named types retain nominal identity;

properties and methods do not add stored fields.
```

Semantic layout does not necessarily define:

```text
byte size;
byte alignment;
field offsets;
padding;
descriptor field order;
reference metadata;
call ABI classification.
```

## Native layout

Native layout is the compiler-selected physical representation for one concrete
`CompilationPlan`.

Native layout includes, where relevant:

```text
size;
alignment;
field offsets;
padding ranges;
element stride;
tag type;
tag offset;
payload offset;
payload size;
payload alignment;
descriptor representation;
reference representation;
validity metadata representation.
```

Native layout may differ across plans.

Native layout is not automatically stable across compiler versions.

## Explicit layout

Explicit layout is a representation whose selected properties are fixed by an
explicit layout contract.

A contract may lock one or more of:

```text
field order;
field offsets;
field alignment;
aggregate alignment;
packing;
total size;
endianness;
tag representation;
payload placement.
```

Only properties stated by the contract become contract guarantees.

Properties not fixed by the contract continue to use native layout rules.

An explicit layout contract:

- does not change nominal type identity;
- does not automatically make a type FFI-compatible;
- does not automatically make a type serializable;
- does not automatically make raw byte reinterpretation safe;
- does not override ownership, storage, lifetime, or destruction rules.

---

# Layout stability

Every resolved layout has one stability class.

```text
CompilationLocal
TargetStable
ContractStable
```

## CompilationLocal

The layout is selected for the active compilation.

It is valid for that compilation output.

It is not guaranteed to remain identical across:

- compiler versions;
- target profiles;
- selected ABIs;
- reference profiles;
- descriptor strategies;
- generic instantiations.

Ordinary native layout is `CompilationLocal` unless a stronger rule applies.

## TargetStable

The target profile explicitly guarantees selected layout properties.

The guarantee must identify:

```text
the target profile;
the profile version;
the guaranteed properties;
the applicable type categories.
```

Target stability does not imply stability on another target or profile.

## ContractStable

An explicit layout contract guarantees the selected properties independently of
ordinary native layout choices.

The guarantee remains limited to the properties named by the contract.

A contract may still be invalid for a target that cannot represent or access the
required layout.

---

# Terminology

## Size

The number of addressable bytes occupied by one complete value representation.

Size includes layout-required tail padding.

Size excludes separate backing storage unless the type's canonical layout embeds
that backing directly.

## Alignment

The required byte alignment for the beginning of a materialized value.

An address satisfies alignment `A` when it is a valid target address for a
value requiring alignment `A`.

## Natural alignment

The alignment selected by the native layout rules for the type under the active
plan.

## Explicit alignment

An alignment requirement introduced by an explicit layout contract.

## Field offset

The byte distance from the start of an aggregate to the start of one stored
field.

## Stride

The byte distance between corresponding starts of adjacent elements in an array
or sequence representation.

Stride must preserve the required alignment of every element.

## Padding

A byte range reserved by layout but not occupied by semantic field or element
data.

Padding may occur:

- before a field;
- between fields;
- after the final field;
- between a tag and payload;
- at the end of an aggregate;
- between array element payloads when stride exceeds element size.

## Representation validity

The rules that determine whether one materialized bit pattern represents a
valid live value of a type.

## Complete layout

A layout for which every required physical property is known for the active
plan.

## Sized layout

A complete layout with a finite compile-time size and alignment.

## Incomplete layout

A layout whose physical properties cannot yet be resolved.

## Unsized payload

Runtime-sized backing or payload storage that is not itself one by-value Sec
object.

An owning descriptor such as `T[]` may be sized even when its separately owned
element sequence is runtime-sized.

---

# Complete sized layout

A type has complete sized layout when the compiler knows at least:

```text
size;
alignment;
physical representation;
contained field offsets or element stride;
destruction-relevant subobjects;
representation-validity class.
```

A type without complete sized layout must not appear directly by value in:

- a stored struct field;
- a fixed array element;
- a union payload;
- a local storage slot;
- a static storage slot;
- an owning aggregate payload;
- another position requiring complete physical layout.

A reference, descriptor, handle, or owning indirection may contain or identify a
runtime-sized payload while remaining sized itself.

---

# Types without ordinary by-value storage

## `void`

`void` has no ordinary runtime value representation.

It has no addressable storage layout.

`void.SizeOf` is invalid.

## Internal `never`

An internal `never` type represents non-returning control flow.

It has no ordinary source-level runtime value representation.

It must not be used as a stored field or fixed-array element.

## Compile-time-only entities

Types, modules, interfaces used only as declarations, generic parameters before
specialization, and other compile-time-only entities do not automatically have
runtime layout.

---

# Scalar layout

## Fixed-width signed integers

The following sizes are exact:

```text
int8       1 byte
int16      2 bytes
int32      4 bytes
int64      8 bytes
int128    16 bytes
int256    32 bytes
```

## Fixed-width unsigned integers

The following sizes are exact:

```text
uint8      1 byte
uint16     2 bytes
uint32     4 bytes
uint64     8 bytes
uint128   16 bytes
uint256   32 bytes
```

Their natural alignment is selected by the active target data layout.

A target may align a wide integer to less than its size.

The compiler must use the plan's actual alignment rather than assume:

```text
alignment == size
```

## `int` and `uint`

`int` and `uint` use the canonical pointer-sized integer width of the active
`CompilationPlan`.

Examples:

```text
32-bit plan:
    int  = 32-bit signed integer
    uint = 32-bit unsigned integer

64-bit plan:
    int  = 64-bit signed integer
    uint = 64-bit unsigned integer
```

Their size matches the plan's canonical pointer size.

Their alignment follows the target data layout for that width.

The pointer-sized rule is semantic and must be applied consistently across:

- semantic analysis;
- enum underlying layout;
- constant range checks;
- MLIR;
- LLVM;
- `SizeOf`;
- ABI validation;
- FFI validation.

## `byte`

`byte` has the same native physical layout as `uint8`.

```text
size:      1 byte
alignment: 1 byte
```

`byte` remains its canonical Sec scalar type.

Physical identity with `uint8` does not create implicit conversion rules beyond
those defined by the type system.

## `bool`

An addressable `bool` occupies one byte and has alignment one.

Only the canonical false and true representations are valid in safe materialized
storage.

The compiler may use a narrower SSA or register representation when:

- the value is not materialized in addressable storage;
- the observable semantics remain unchanged;
- conversion to and from the canonical stored representation is preserved.

A backend's temporary `i1` representation does not change the one-byte stored
layout.

## `char`

`char` occupies one byte and has alignment one.

Its valid bit patterns are those defined by the canonical Sec `char` rule.

## `rune`

`rune` uses a 32-bit scalar representation.

```text
size:      4 bytes
alignment: target alignment for uint32
```

Only valid Unicode scalar values are valid `rune` representations in safe code.

Surrogate values and values outside the Unicode scalar range are invalid.

## Floating-point values

```text
float32
    IEEE-754 binary32
    size: 4 bytes

float64
    IEEE-754 binary64
    size: 8 bytes

float
    same semantic numeric width and native physical layout as float64
```

Alignment follows the target data layout.

The language may retain separate canonical type names even when native layout is
identical.

## `decimal`

The canonical native component representation is:

```text
coefficient: int64
scale:       int32
```

The aggregate uses ordinary native struct placement for those two components.

The final size and alignment are therefore computed from the target alignment of
`int64` and `int32`.

## `decimal128`

The canonical native component representation is:

```text
coefficient: int128
scale:       int32
```

The aggregate uses ordinary native struct placement for those two components.

The target may add padding after `scale` to satisfy aggregate alignment.

## Raw pointers

`RawPtr[T]` uses the target raw-pointer size and alignment for the applicable
address space.

The pointee type does not change pointer size.

A target-specific address space may use another pointer layout only when that
space is part of the concrete compilation plan.

Raw-pointer layout does not imply:

- ownership;
- valid provenance;
- valid pointee representation;
- bounds;
- non-nullness;
- lifetime.

---

# Named types, contracts, and units

A named type uses the same native physical layout as its underlying type unless
an explicit future layout contract states otherwise.

Examples:

```sec
type CustomerID uint64
type ProductID uint64
type Percent int range 0..100
type Meter decimal<m>
```

The following do not add hidden per-value storage:

- nominal identity;
- range contracts;
- unit metadata;
- regex contracts;
- length contracts;
- semantic dimensions;
- compiler-known members;
- methods;
- properties;
- impl blocks;
- interfaces implemented by the type.

Contract checks affect construction and mutation semantics.

They do not insert a hidden contract tag into every value.

Two named types may have identical physical layout and still be distinct,
incompatible Sec types.

---

# Struct layout

## Field order

Stored struct fields retain declaration order in every materialized complete
struct representation.

The compiler may insert padding.

It must not reorder ordinary stored fields.

Example:

```sec
type Example struct {
    first: byte
    second: uint32
    third: byte
}
```

The physical order is:

```text
first
padding when required
second
third
tail padding when required
```

The compiler must not place `second` first merely to reduce padding.

## Native struct algorithm

For a non-packed native struct:

```text
currentOffset = 0
maximumAlignment = 1

for each stored field in declaration order:
    fieldAlignment = AlignOf(fieldType)
    fieldSize = SizeOf(fieldType)

    fieldOffset = RoundUp(currentOffset, fieldAlignment)
    currentOffset = CheckedAdd(fieldOffset, fieldSize)
    maximumAlignment = Max(maximumAlignment, fieldAlignment)

structAlignment = maximumAlignment
structSize = RoundUp(currentOffset, structAlignment)
```

All arithmetic is checked.

An overflow is a compile-time layout error.

## Stored members

Only stored fields participate in struct storage layout.

The following do not add stored bytes:

- properties;
- methods;
- nested type declarations;
- nested enum declarations;
- nested union declarations;
- nested unit declarations;
- constants;
- interface conformance;
- field tags;
- documentation;
- visibility metadata;
- compiler annotations that do not explicitly define representation.

## Tail padding

Tail padding is included in `SizeOf`.

It ensures that adjacent values in an array or aggregate can satisfy the
struct's alignment.

Tail padding is not semantic field data.

## Nested structs

A stored nested struct field uses the complete size and alignment of its field
type.

Nested layout is resolved before the containing struct layout.

## Generic structs

A generic struct declaration does not have one universal concrete native layout.

Each concrete instantiation has its own layout.

Example:

```sec
type Pair[A, B] struct {
    first: A
    second: B
}
```

These are resolved independently:

```text
Pair[int32, byte]
Pair[decimal, string]
Pair[CustomerID, ProductID]
```

## Empty structs

An empty struct has:

```text
size:      0
alignment: 1
```

An empty struct is a valid zero-sized type.

## Zero-sized fields

A zero-sized stored field:

- has an object lifetime;
- has nominal field identity;
- may share its numeric address with another zero-sized field or surrounding
  storage;
- does not force positive aggregate size by itself.

Semantic field identity must not be derived solely from a numeric address.

## Materialization and optimization

The compiler may eliminate a whole struct or individual field storage when:

- the representation is not observed;
- ownership and destruction remain correct;
- debug requirements permit it;
- all source semantics are preserved.

When a complete struct representation is materialized, it must follow the
resolved layout.

---

# Padding

Padding is not a Sec value.

Padding:

- has no semantic type;
- has no field identity;
- has no object lifetime;
- is not initialized merely because surrounding fields are initialized;
- need not contain deterministic bytes;
- may change during copy, move, reconstruction, optimization, or lowering;
- must not affect semantic equality;
- must not affect semantic hashing;
- must not be included implicitly in ordinary serialization;
- must not be read through safe field access.

A compiler may choose to initialize padding for security or target reasons.

Such initialization does not make padding semantic data.

When native representation bytes leave the trusted execution context through:

- FFI;
- a file;
- a socket;
- shared memory;
- device transfer;
- another process;
- a persistent binary format;

padding must be:

- omitted;
- explicitly defined by a layout contract;
- or deterministically initialized before exposure.

This prevents disclosure of stale or uninitialized bytes.

---

# Fixed-array layout

Sec fixed arrays use postfix syntax:

```sec
T[N]
```

`N` is part of the type.

A fixed array contains exactly `N` inline elements.

It contains no hidden:

- pointer;
- runtime length;
- capacity;
- allocator;
- backing-storage owner;
- generation field.

## Element alignment and stride

For `T[N]`:

```text
elementAlignment = AlignOf(T)
elementSize = SizeOf(T)
elementStride = RoundUp(elementSize, elementAlignment)

arrayAlignment = elementAlignment
arraySize = CheckedMultiply(N, elementStride)
```

The active target may define equivalent layout where the computed stride
preserves the same element alignment and array semantics.

## Contiguity

Elements occur in increasing index order.

The address of element `i + 1` is one element stride after element `i` when the
array is materialized.

Contiguity means contiguous array element slots.

It does not require:

```text
StrideOf(T) == semantic payload bytes of T
```

because one element slot may contain tail padding.

## Zero-length arrays

`T[0]` is valid.

It has:

```text
size:      0
alignment: AlignOf(T)
stride:    StrideOf(T)
```

A zero-length array contains no live `T` elements.

## Zero-sized elements

When `SizeOf(T) == 0`, `StrideOf(T)` may be zero.

A `T[N]` array may therefore have zero total size even when `N` is positive.

Array element identity is derived from:

```text
array identity + index
```

not solely from numeric address.

The compiler must preserve bounds, ownership, initialization, and destruction
semantics for zero-sized elements.

## Layout overflow

The compiler must reject:

- element stride overflow;
- `N * stride` overflow;
- array sizes exceeding the target's representable object size;
- arrays whose alignment cannot be represented by the selected target.

---

# Owning dynamic array layout

`T[]` is an owning dynamic array or sequence value.

It is a sized descriptor type.

Its runtime-known element sequence is separate backing storage unless a concrete
canonical representation explicitly embeds some or all backing capacity.

`T[]` may appear by value in:

- variables;
- fields;
- parameters;
- return values;
- unions;
- other sized aggregates.

`SizeOf(T[])` returns the descriptor size for the active plan.

It does not return:

```text
length * SizeOf(T)
```

This is the type-layout query. For an owning-array instance, `values.SizeOf`
reports `values.Len * stride(T)` payload bytes and does not reveal descriptor
layout or unused capacity.

and does not include separate backing allocation.

The exact native descriptor representation is profile-selected.

It must preserve all required semantics, which may include:

```text
backing-storage identity;
current data address or handle;
length;
capacity;
allocator or allocation context;
ownership state;
generation or epoch dependency;
memory-space identity.
```

Not every representation must store every conceptual field physically.

A field may be omitted when equivalent semantics are proven by static analysis
or represented elsewhere.

A bare `T[]` is not an unsized by-value type.

The runtime-sized element sequence is the unsized payload managed by the owning
descriptor.

---

# Slice and reference layout

## Slice references

```sec
ref T[]
ref mut T[]
```

are sized safe-reference descriptors.

Their backing element sequence is not included in the internal descriptor-layout
query or type-form `SizeOf(ref T[])`. The public instance property
`view.SizeOf` instead reports represented payload bytes as defined by
`compiler_known_members.md`.

A valid representation must preserve:

```text
storage identity;
element type;
length;
bounded range;
access authority;
provenance;
lifetime or epoch dependency;
address-space compatibility.
```

The active profile may use:

- address plus length;
- handle plus length;
- address, length, and expected epoch;
- another representation preserving the same guarantees.

## `ref T`

A safe `ref T` representation is profile-selected.

It may be:

- a direct address;
- an address plus expected generation;
- an indirect stable handle;
- a target capability;
- another representation preserving the reference model.

Its physical size is therefore plan-dependent.

## `ref mut T`

A mutable safe reference may use the same physical shape as `ref T`.

Its exclusive authority remains semantic even when no extra runtime bit is
required.

## Empty slices

An empty slice may use a null-like hidden base representation only when:

- length is zero;
- the base is never dereferenced;
- no safe `ref T` is reconstructed from the base alone;
- every operation respects the empty range.

This does not make safe references nullable.

---

# String layout

`string` is a sized value or descriptor.

Its exact native representation is selected by the active plan and canonical
string model.

It may include or depend on:

```text
data address or handle;
length;
capacity when owned;
ownership form;
allocation context;
generation dependency;
encoding contract.
```

`SizeOf(string)` returns the value or descriptor size.

It does not return the number of characters, runes, or bytes in the string
payload.

String payload storage is counted separately unless the canonical representation
explicitly embeds it.

Native string bytes are not a general ABI or serialization contract.

---

# Enum layout

A fieldless enum uses exactly the layout of its resolved underlying integer
type.

```text
enum size      = underlying size
enum alignment = underlying alignment
```

The enum adds no hidden:

- tag;
- name pointer;
- lookup table pointer;
- metadata pointer;
- validity bitmap.

Enum nominal identity remains compiler metadata.

## Default underlying type

When no underlying type is declared, the underlying type is `int`.

Because `int` is pointer-sized, the default fieldless enum native layout is
pointer-sized for the active plan.

The compiler must not hard-code default enum lowering to `i32`.

## Explicit underlying types

An enum with explicit underlying type uses exactly that type's layout.

Example:

```sec
enum Status uint16 {
    Idle = 0
    Ready = 1
}
```

has the physical layout of `uint16`.

## Bit-backed enums

A `bit[N]`-backed enum has exactly `N` semantic representation bits.

It is used where register or hardware layout permits that exact bit width.

When materialized as ordinary addressable standalone storage, the target profile
must define:

- the containing storage unit;
- alignment;
- access width;
- legal load and store lowering.

Within a register field, the enum occupies exactly `N` bits.

## Enum representation validity

An enum representation is valid when its underlying value satisfies the enum
validity rule.

Unless another enum rule explicitly allows unknown values, only declared enum
values are valid safe enum representations.

Numeric aliases remain valid because multiple names may share one declared
underlying value.

---

# Tagged-union layout

A Sec union is a tagged union.

Sec 0.1 does not use:

- niche optimization;
- untagged layout;
- C-union layout;
- user-controlled native union layout;
- implicit nullable-pointer representation;
- payload-only representation.

## Variant indices

Each variant receives a stable zero-based internal index in declaration order.

The index is compiler metadata.

It is not a source-level integer value.

## Tag type

The native tag uses the smallest byte-based unsigned integer type that can
represent every variant index.

```text
1 through 256 variants:
    uint8

257 through 65,536 variants:
    uint16

65,537 through 4,294,967,296 variants:
    uint32

larger:
    uint64 when the target and compiler support the required layout,
    otherwise a compile-time error
```

The upper bounds include every zero-based index representable by the selected
type.

## Payload requirements

For every variant:

```text
variantPayloadSize
variantPayloadAlignment
```

are resolved from the complete payload type.

An empty variant has:

```text
payload size:      0
payload alignment: 1
```

A struct-like variant payload uses ordinary struct layout for its named payload
fields.

The union payload requirements are:

```text
payloadSize = Max(all variant payload sizes)
payloadAlignment = Max(1, all variant payload alignments)
```

## Native union algorithm

```text
tagSize = SizeOf(tagType)
tagAlignment = AlignOf(tagType)

payloadOffset = RoundUp(tagSize, payloadAlignment)

unionAlignment = Max(tagAlignment, payloadAlignment)

rawSize = CheckedAdd(payloadOffset, payloadSize)
unionSize = RoundUp(rawSize, unionAlignment)
```

The tag begins at offset zero in the Sec 0.1 native union layout.

Padding may occur:

- after the tag;
- within a struct-like payload;
- after the payload storage.

## Active payload

Exactly one variant is active.

Only the active payload has object lifetime.

Inactive payload storage:

- is not a live value;
- must not be read;
- must not be borrowed as its payload type;
- must not be destroyed as an inactive payload;
- may contain stale bytes.

Changing variants must:

1. evaluate the new payload;
2. preserve failure atomicity where required;
3. end the old payload lifetime;
4. write the new payload;
5. write or commit the new tag according to a lowering that never exposes an
   invalid safe union state.

## Empty unions

Empty unions remain invalid.

## Recursive unions

A union payload must have complete finite by-value layout.

Direct recursive by-value storage is invalid.

Recursion through a finite-sized indirection descriptor is allowed.

---

# Register layout

A register type declares an exact semantic bit layout.

Example:

```sec
type Status register[32] {
    Ready: bit
    Error: bit
    _: bit[30]
}
```

The declared register width is exact.

Named fields and reserved fields together must occupy exactly that width.

## Semantic storage width

For `register[N]`:

```text
semantic bits = N
minimum addressable storage bytes = Ceil(N / 8)
```

No compiler-generated padding may be inserted between register fields.

Reserved bits remain part of the representation.

## Physical access unit

The target profile defines:

- legal physical access width;
- required alignment;
- bit numbering;
- byte order;
- volatile access instructions;
- whether sub-byte or non-power-of-two storage is directly addressable.

A target may require a physical access unit wider than the minimum byte count.

This does not change:

- semantic register width;
- field bit offsets;
- reserved-bit semantics.

## Impl blocks

Methods, properties, interfaces, and impl metadata never add storage to a
register value.

A 32-bit register remains a 32-bit semantic register even when behavior is
defined in an impl block.

---

# Function, closure, interface, and resource descriptors

The following categories have sized plan-selected native layouts when used as
runtime values:

```text
function values;
capturing closures;
interface values;
Arena owner values;
collection descriptors;
mapping descriptors;
resource wrappers;
stable handles;
weak handles.
```

Their source-level semantics do not require one universal field representation.

A resolved layout must identify their physical size and alignment for the active
plan before they are stored by value.

## Function values

A non-capturing function value may lower to one code pointer or equivalent
target callable identity.

A general callable representation may require:

```text
code identity;
environment identity or pointer.
```

The presence of a closure environment does not change the source function type.

## Capturing closures

A closure environment has its own aggregate layout.

Captured values occur in a compiler-defined environment order that must be
deterministic for the active compilation.

The environment layout is not a source-observable stable layout unless an
explicit contract later states otherwise.

## Interface values

An interface value may require:

```text
concrete value address or owner;
dispatch metadata;
ownership or reference mode;
generation or provenance metadata.
```

The layout is plan-selected.

Owning and borrowed interface values may have different canonical
representations if their semantic types distinguish them.

## Resource wrappers

A resource wrapper's layout follows its stored fields and explicit contracts.

The existence of a custom `free` operation does not add a hidden runtime
destructor pointer unless a separate dynamic-dispatch contract requires it.

---

# Recursive layout

The compiler builds a layout dependency graph.

An edge is a by-value layout dependency when one type directly contains another
type's complete representation.

Examples:

```text
struct field by value;
fixed-array element;
union payload;
closure environment capture by value;
embedded descriptor field;
```

## Invalid direct recursion

```sec
type Node struct {
    next: Node
}
```

is invalid because calculating `Node` requires calculating `Node` again without
finite indirection.

## Invalid indirect by-value cycle

```sec
type First struct {
    second: Second
}

type Second struct {
    first: First
}
```

is also invalid.

## Valid recursion through indirection

Recursion is valid through a type with complete finite descriptor layout.

Examples may include:

```sec
type Node struct {
    next: Option[Box[Node]]
}
```

or an equivalent owning/reference indirection defined by the language.

The compiler must distinguish:

- by-value dependency;
- reference dependency;
- raw-pointer dependency;
- owning-indirection descriptor dependency;
- runtime backing dependency.

Only by-value cycles make layout infinite.

## Diagnostic cycle

A recursive-layout diagnostic must show the relevant cycle.

Example shape:

```text
error: recursive by-value layout has no finite size

Node
  field next: Node
  depends on Node again

help:
    introduce a reference or another finite-sized indirection
```

---

# Generic layout

A generic declaration may remain layout-incomplete until concrete type arguments
are known.

Example:

```sec
type Pair[A, B] struct {
    first: A
    second: B
}
```

The declaration defines a layout template.

Each monomorphized instance resolves a concrete layout.

## Instance identity

The layout cache key must include:

```text
canonical generic declaration identity;
ordered concrete generic arguments;
active CompilationPlan identity;
relevant explicit layout contract identity.
```

The same key must produce one canonical resolved layout.

Different concrete argument lists may produce different:

- size;
- alignment;
- field offsets;
- padding;
- copy classification;
- destruction requirements.

## Generic queries

A generic body may use a layout query on `T` only when:

- the operation is resolved during concrete specialization;
- or an applicable generic constraint guarantees complete layout.

The compiler must not invent one placeholder numeric size for an unresolved
generic type.

## Recursive generic layout

The compiler must detect recursive instantiation that expands without finite
indirection.

The diagnostic should identify both:

- the generic instantiation chain;
- the by-value field or payload causing the expansion.

---

# Explicit layout contracts

Explicit layout semantics may be introduced through attributes or another
compiler-known declaration form.

This rulebook fixes their meaning without fixing final surface syntax.

## Contract properties

A contract may specify:

```text
field order;
field offsets;
field alignment;
aggregate alignment;
packing;
total size;
endianness;
tag representation;
payload placement.
```

Every specified property is validated.

An unsupported or contradictory contract is a compile-time error.

## Full placement versus partial placement

Sec 0.1 supports one of:

- ordinary compiler-selected placement;
- a complete explicit placement contract.

Partial explicit field offsets mixed with automatic placement are not part of
the initial rule.

A future extension may define deterministic mixed placement.

Until then, a type that uses explicit field offsets must define all stored field
offsets required by the contract.

## Explicit field offsets

An explicit field offset:

- is measured in bytes from aggregate start;
- must be non-negative;
- must be compile-time known;
- must not overflow target layout arithmetic;
- must place the complete field within explicit total size when total size is
  fixed;
- must not overlap another stored field;
- must satisfy natural field alignment unless packing explicitly permits
  misalignment.

## Explicit total size

An explicit total size must:

- contain every stored field;
- contain every required tag and payload range;
- satisfy aggregate alignment unless the contract explicitly defines a foreign
  representation with another rule;
- be representable by the target object-size model.

A size smaller than the required occupied range is invalid.

A larger size introduces explicitly contracted trailing padding.

## Explicit aggregate alignment

An explicit aggregate alignment normally may increase natural alignment.

It must not reduce alignment below the largest required field alignment unless
packing explicitly permits misaligned fields.

The requested alignment must be supported by the target or emulated by an
explicit storage mechanism defined elsewhere.

## Contract validation

Contract validation occurs after:

- all field types are resolved;
- concrete generic arguments are known;
- the active target data layout is selected.

The compiler must report both requested and computed values when validation
fails.

---

# Packing

Packing is a representation contract.

It is not a normal optimization.

Packing may reduce or eliminate compiler-inserted padding.

Packing must not reorder fields.

## Misaligned fields

Packing may place a field at an address that does not satisfy the field type's
natural alignment.

Such a field is marked `IsMisaligned` in resolved layout.

## Access to misaligned fields

A value read from a misaligned field is permitted only when the selected target
and lowering can preserve Sec semantics using:

- a supported unaligned load;
- bytewise reconstruction;
- a compiler-known packed-field operation;
- another verified target operation.

A write follows the corresponding safe strategy.

If no valid lowering exists, the access is a compile-time error for that target.

## References to misaligned fields

A normal `ref T` or `ref mut T` must not be created directly to a misaligned
field because such references guarantee correct alignment for `T`.

Invalid conceptual operation:

```sec
let fieldRef := ref packedValue.field
```

when `field` is not aligned for its type.

A compiler-known packed-field proxy or explicit unsafe raw access may be defined
separately.

## Atomic and volatile access

Packing does not imply that a misaligned field supports:

- atomic access;
- volatile access of the declared width;
- lock-free access;
- hardware register access.

The target and applicable rulebook must validate those requirements separately.

---

# Endianness

Endianness is a materialized representation property.

It does not change the semantic numeric value.

Canonical representation modes are:

```text
NativeEndian
LittleEndian
BigEndian
```

## Ordinary values

Ordinary native Sec scalar storage uses target-native endianness.

## Explicit endian fields

An explicitly endian-encoded scalar field is decoded when read and encoded when
written.

Arithmetic operates on the decoded semantic value.

Example conceptual behavior:

```text
stored little-endian uint32 bytes
    -> decode
semantic uint32
    -> arithmetic
semantic uint32
    -> encode
stored little-endian uint32 bytes
```

## Aggregate endianness

A struct-wide endian contract does not mean reversing all aggregate bytes.

It applies to explicitly supported scalar fields according to the contract.

Padding, field order, references, descriptors, resources, and nested aggregates
are not transformed by blindly reversing the full object byte sequence.

## Unsupported fields

The following must not receive ordinary numeric endian encoding without a
specific representation contract:

- safe references;
- raw pointers;
- function values;
- interface values;
- owning descriptors;
- resource handles;
- closures;
- opaque foreign values.

## Registers

Register byte order and bit numbering remain governed by register and target
contracts.

A generic aggregate-endianness rule must not override a register's hardware
layout.

---

# Representation validity

Size and alignment are not sufficient to establish a valid live value.

Every layout has one representation-validity class.

```text
AllBitPatternsValid
RestrictedBitPatterns
ActiveVariantDependent
Opaque
```

## `AllBitPatternsValid`

Every bit pattern of the occupied semantic representation is valid.

Typical examples include unsigned and signed fixed-width integers.

Padding is excluded from this statement because padding is not semantic data.

## `RestrictedBitPatterns`

Only selected bit patterns represent valid values.

Examples include:

- `bool`;
- `rune`;
- enums when unknown underlying values are not permitted;
- constrained compiler-known scalar encodings.

## `ActiveVariantDependent`

Validity depends on a discriminant and the active payload.

Tagged unions use this class.

A valid union requires:

- a valid tag;
- the corresponding payload representation to be valid;
- no assumption that inactive payload bytes represent live values.

## `Opaque`

Ordinary Sec code may not construct or validate the representation from raw
bytes.

Examples may include:

- references;
- interface values;
- resource wrappers with invariants;
- closure values;
- foreign opaque values;
- target capabilities.

Only compiler-known operations, validated wrappers, or unsafe code may establish
such a value.

---

# Raw storage and initialization

Correct size and alignment create suitable storage.

They do not create a live object.

Object lifetime begins only after a valid representation is constructed.

## Uninitialized storage

Typed uninitialized storage:

- reserves correctly sized and aligned slots;
- does not contain live objects;
- must not be read as `T`;
- must not be borrowed as `ref T`;
- must not be destroyed as `T`.

Construction starts the object lifetime of the initialized slot.

Destruction ends that object lifetime.

## Partial initialization

Aggregate initialization may be partial during construction.

The compiler must track initialized state for relevant:

- struct fields;
- array elements;
- union payload;
- closure captures;
- compiler-generated aggregate components.

On early failure, only successfully initialized subobjects are destroyed.

## Zero initialization

Filling storage with zero bytes creates a valid `T` only when the type's
representation contract defines the all-zero representation as a valid
initialized value.

There is no universal rule that every Sec type is valid after bytewise zeroing.

Examples requiring type-specific validation include:

- references;
- non-zero handles;
- enums without a zero variant;
- unions;
- resource wrappers;
- opaque descriptors;
- runes when additional invariants apply.

Default construction and zero-byte initialization are separate operations.

## Padding initialization

Initializing every semantic field does not necessarily initialize padding.

A layout contract may require deterministic padding for:

- FFI;
- shared memory;
- persistent format;
- device transfer;
- security.

Otherwise padding remains non-semantic.

---

# Representation observability

Safe Sec code may observe only representation facts exposed by a rule or
explicit contract.

Normally observable layout facts include:

- `SizeOf`;
- compiler-known alignment queries when exposed;
- compiler-known stride queries when exposed;
- explicit field offsets;
- explicit alignment;
- explicit total size;
- explicit endianness;
- register bits;
- verified foreign or hardware layout.

Safe Sec code must not assume:

- unspecified native field offsets;
- padding byte values;
- one universal reference shape;
- one universal string descriptor shape;
- one universal interface representation;
- one universal closure representation;
- stable native layout across compiler versions;
- C compatibility of ordinary structs;
- serialization compatibility of native object bytes.

## Equality

Semantic equality compares semantic values.

It must not compare native padding.

A type without semantic equality does not gain equality merely because its bytes
can be compared.

## Hashing

Semantic hashing hashes semantic value components.

It must not include unspecified padding.

## Serialization

Ordinary serialization operates on semantic fields and explicit format rules.

It must not dump native object bytes by default.

## Debuggers

Debug information may expose native offsets for the compiled program.

This does not promote those offsets to a source-language stability guarantee.

---

# Layout compatibility

Physical similarity does not create compatibility.

Two types may have identical:

- size;
- alignment;
- field count;
- field offsets;
- field types;
- tag shape;

and still be distinct, incompatible Sec types.

Layout compatibility exists only when an applicable contract explicitly defines
it.

Possible uses include:

- FFI validation;
- compiler-known representation conversion;
- validated memory mapping;
- explicit unsafe reinterpretation.

Layout compatibility does not create:

- implicit conversion;
- structural typing;
- automatic aliasing permission;
- ownership transfer;
- destruction compatibility;
- permission to use `ref A` as `ref B`.

---

# Reinterpretation

Safe Sec has no general representation reinterpret cast.

Treating existing bytes as another type requires an explicit unsafe boundary
unless a compiler-known checked operation provides equivalent validation.

A reinterpretation must validate or assert:

```text
size compatibility;
alignment compatibility;
field or scalar layout;
representation validity;
object lifetime;
initialization;
ownership;
aliasing;
provenance;
destruction responsibility;
memory-space compatibility;
endianness;
active union state when relevant.
```

Reinterpretation must not create:

- two owners of one resource;
- two independent destruction responsibilities;
- a safe reference with false alignment;
- a live object over already-live incompatible storage;
- a reference whose provenance does not authorize access.

Equal size alone is never sufficient.

---

# Layout queries

The compiler must support semantic layout queries for:

```text
SizeOf
AlignOf
StrideOf
FieldOffset
```

`SizeOf` already has compiler-known source forms defined by
`compiler_known_members.md`.

The final source spellings of the remaining queries may be defined there or by
another compiler-known rule.

This document defines their semantics.

## Result type

Every successful layout query returns:

```text
uint
```

for the active `CompilationPlan`.

The value is compile-time known for one concrete plan and complete layout.

## `SizeOf`

`SizeOf(T)` means the complete physical storage size of one materialized `T`,
including required padding.

For descriptors, it returns descriptor size and excludes separate backing
storage.

For fixed arrays, it includes complete element stride.

For union values, it includes tag, payload storage, and padding.

For registers, it returns `Ceil(registerBitWidth / 8)` bytes. Target-required
physical access width remains a separate access constraint and does not increase
the semantic register storage size.

## `AlignOf`

`AlignOf(T)` returns the required alignment of a materialized `T`.

## `StrideOf`

`StrideOf(T)` returns the byte distance required between adjacent materialized
`T` elements in an array representation.

Normally:

```text
StrideOf(T) = RoundUp(SizeOf(T), AlignOf(T))
```

A target-specific explicit layout may provide an equivalent validated stride.

## `FieldOffset`

`FieldOffset(T, field)` returns the resolved byte offset of one stored field.

It is valid only for:

- stored struct fields;
- stored closure-environment fields when compiler tooling requests them;
- explicit stored descriptor fields where the representation is intentionally
  queryable;
- another compiler-known stored aggregate member.

It is invalid for:

- properties;
- methods;
- constants;
- enum members;
- computed members;
- arbitrary interface requirements;
- semantic members without stored representation.

## Value-form evaluation

When a query uses a value receiver, receiver evaluation follows normal Sec
evaluation rules.

The value itself is not inspected to calculate layout.

Effects of producing the receiver are preserved.

## Invalid queries

A layout query is invalid when:

- the type is incomplete;
- the type is unresolved;
- the type has no runtime representation;
- the type remains unspecialized;
- the target plan is unresolved;
- the requested field is not stored;
- layout arithmetic overflowed;
- the selected target cannot represent the layout.

The diagnostic must identify the missing or invalid layout fact.

---

# Layout resolution phase

The compiler uses one shared layout-resolution phase.

Canonical order:

```text
1. Resolve semantic type identities.
2. Resolve concrete generic instances.
3. Select the target and complete CompilationPlan.
4. Build by-value layout dependencies.
5. Detect illegal recursive layout cycles.
6. Resolve scalar layouts.
7. Resolve compiler-known descriptor and reference layouts.
8. Resolve arrays and aggregates.
9. Resolve enum and union representations.
10. Resolve register addressable storage requirements.
11. Validate explicit layout contracts.
12. Validate representation-validity requirements.
13. Validate target access requirements.
14. Cache canonical resolved layouts.
15. Publish resolved layouts to plan-resolved Semantic IR.
16. Lower through MLIR and target backends.
```

A target-independent semantic phase may retain unresolved layout requirements.

Before any layout-sensitive backend lowering, every required concrete layout must
be resolved.

---

# Resolved layout model

The compiler must be able to represent at least:

```text
ResolvedLayout {
    TypeIdentity
    CompilationPlanIdentity
    Size
    Alignment
    StabilityClass
    RepresentationValidity
    Fields
    PaddingRanges
    ElementStride
    ArrayLength
    TagLayout
    PayloadLayout
    RegisterBitWidth
    ExplicitContract
}
```

Not every field applies to every type.

## Field layout

```text
ResolvedFieldLayout {
    FieldIdentity
    Offset
    Size
    Alignment
    IsMisaligned
    Endianness
    PaddingBefore
}
```

## Tag layout

```text
ResolvedTagLayout {
    Type
    Offset
    Size
    Alignment
    VariantIndices
}
```

## Payload layout

```text
ResolvedPayloadLayout {
    Offset
    Size
    Alignment
    Variants
}
```

## Padding range

```text
PaddingRange {
    Start
    Length
    ContractDefined
}
```

These names are conceptual.

Compiler implementation types may use other names only when the same facts are
preserved.

---

# Semantic IR requirements

Plan-resolved Semantic IR must retain or reference every resolved layout required
for later lowering.

Semantic IR must preserve:

- semantic type identity;
- resolved physical layout identity;
- field identities;
- field offsets where materialized;
- alignment requirements;
- array stride;
- union tag and payload rules;
- representation validity;
- explicit layout contracts;
- source locations for user-declared layout requirements.

Object operations remain semantic operations.

Examples:

```text
construct struct;
extract field;
borrow field;
update field;
construct union variant;
test union tag;
extract active payload;
construct array;
index array;
construct object in typed storage;
end object lifetime.
```

The backend must not infer source ownership or object lifetime merely from loads
and stores.

---

# Backend lowering

MLIR, LLVM, and other backends consume the resolved Sec layout.

A backend must not independently:

- reorder Sec struct fields;
- choose another enum underlying representation;
- select another union tag scheme;
- perform niche optimization in Sec 0.1;
- change explicit offsets;
- remove required explicit padding;
- change explicit endianness;
- make misaligned references appear aligned;
- reinterpret descriptors as C-compatible;
- change zero-sized identity semantics;
- change object representation validity.

A backend may optimize physical operations when:

- the observable layout contract is preserved;
- safe-reference guarantees remain valid;
- object lifetime remains correct;
- padding is not exposed;
- explicit layout remains exact;
- debug and FFI requirements are preserved.

---

# ABI boundary

Storage layout and callable ABI are separate.

`layout.md` defines how a value exists in storage.

`abi.md` defines how a value is passed or returned by a function.

A value may have complete storage layout while the ABI:

- passes it in registers;
- splits it into components;
- passes it indirectly;
- returns it through caller-provided storage;
- applies platform-specific classification.

ABI lowering must preserve the semantic value and its storage layout when
materialized.

---

# FFI boundary

Ordinary native Sec layout is not automatically foreign-compatible.

A type crossing an FFI boundary directly requires:

- an explicit foreign-compatible representation contract;
- a supported target ABI;
- compatible field types;
- compatible alignment;
- compatible padding;
- compatible enum or union representation;
- compatible ownership and lifetime rules.

The compiler must not infer FFI compatibility merely from coincidental size and
offset equality.

Strings, owning arrays, slices, references, interfaces, closures, and other
descriptors require explicit foreign representations or wrappers.

`extern "C"` structs and unions consume the active C ABI layout model.
Bitfield allocation units, ordering, padding, and zero-width effects are
ABI-owned and do not reuse register bit-order rules. `C::flex[T]` contributes no
descriptor and is valid only as the final stored field; the containing
structure's size excludes runtime trailing elements. Incomplete foreign types
have no by-value layout.

---

# Memory spaces

Memory-space identities and access contracts are defined by `storage.md`.

Layout consumes those contracts.

A memory space may affect:

- pointer representation;
- legal alignment;
- legal load/store widths;
- unaligned access support;
- atomic support;
- volatile requirements;
- address-space-specific descriptor layout.

Layout must not redefine memory-space ownership or lifetime.

Numerically equal addresses in different memory spaces do not imply compatible
layout access.

---

# Diagnostics

Layout diagnostics must use stable diagnostic identities.

Required diagnostic categories include at least:

```text
layout.incomplete-type
layout.unsized-by-value
layout.recursive-by-value
layout.size-overflow
layout.invalid-alignment
layout.overlapping-fields
layout.field-out-of-bounds
layout.misaligned-field-reference
layout.invalid-explicit-size
layout.unsupported-packed-access
layout.invalid-endian-field
layout.invalid-representation
layout.unstable-representation-use
layout.unsupported-target-layout
layout.invalid-layout-query
```

## Diagnostic content

A diagnostic should identify, when relevant:

- semantic type;
- concrete generic instantiation;
- target and compilation plan;
- computed size;
- computed alignment;
- requested size;
- requested alignment;
- field name;
- field offset;
- field occupied range;
- conflicting field range;
- recursion cycle;
- unsupported access width;
- representation-validity requirement;
- explicit contract source location.

## Example: recursive layout

```text
error: recursive by-value layout has no finite size

type:
    Node

cycle:
    Node
      field next: Node
      -> Node

help:
    introduce a reference or another finite-sized indirection
```

## Example: invalid alignment

```text
error: explicit alignment cannot be satisfied

type:
    Packet

requested alignment:
    64

target:
    bare-metal/cortex-m4

maximum supported alignment:
    16
```

## Example: overlapping fields

```text
error: explicit layout fields overlap

Header.length:
    offset 0
    size   4
    range  0..<4

Header.flags:
    offset 2
    size   4
    range  2..<6
```

## Example: misaligned reference

```text
error: cannot create ref uint32 to misaligned packed field

field:
    Packet.value

field offset:
    1

required alignment:
    4

help:
    read the field by value or use a compiler-known packed-field operation
```

## Example: invalid representation

```text
error: raw bytes do not establish a valid Rune value

reason:
    the bit pattern is not a Unicode scalar value
```

---

# LSP requirements

The language server should expose resolved layout information when available.

Useful information includes:

- size;
- alignment;
- field offsets;
- element stride;
- padding;
- union tag type;
- union payload size;
- layout stability;
- explicit contract;
- target plan.

The LSP must distinguish:

- semantic source guarantees;
- current native layout;
- explicit stable contract.

Quick fixes may suggest:

- introducing indirection for recursive layout;
- increasing explicit size;
- correcting explicit alignment;
- correcting field offsets;
- removing unsafe packing;
- reading a packed field by value rather than reference;
- adding an explicit foreign wrapper.

The LSP must not automatically insert unsafe reinterpretation.

---

# Required tests

The layout test suite must cover at least the following.

## Scalar tests

- every fixed-width integer size;
- target-specific alignment of wide integers;
- pointer-sized `int`;
- pointer-sized `uint`;
- `byte`;
- stored `bool`;
- `char`;
- `rune`;
- `float32`;
- `float64`;
- `float`;
- `decimal`;
- `decimal128`;
- raw pointers.

## Struct tests

- declaration-order fields;
- leading inter-field padding;
- tail padding;
- nested structs;
- empty structs;
- zero-sized fields;
- properties adding no storage;
- methods adding no storage;
- field tags adding no storage;
- generic struct instantiations;
- layout overflow.

## Array tests

- `T[0]`;
- `T[1]`;
- ordinary arrays;
- aligned element stride;
- arrays of padded structs;
- arrays of zero-sized elements;
- nested arrays;
- total-size overflow;
- owning `T[]` descriptor size;
- slice descriptor size.

## Enum tests

- default underlying `int`;
- target-sized default enum layout;
- explicit `uint8`;
- explicit `int64`;
- bit-backed enum fields;
- enum aliases;
- enum nominal identity with identical layout.

## Union tests

- one empty variant;
- multiple empty variants;
- payload variants;
- struct-like payloads;
- tag-width thresholds;
- payload alignment;
- tail padding;
- active-payload validity;
- no niche optimization;
- recursive by-value rejection;
- valid indirect recursion.

## Register tests

- widths divisible by eight;
- widths not divisible by eight;
- exact field-bit totals;
- reserved bits;
- bit-backed enum fields;
- target access-unit validation;
- impl members adding no storage.

## Explicit-layout tests

- complete explicit field offsets;
- explicit total size;
- increased alignment;
- invalid reduced alignment;
- packed fields;
- misaligned reads;
- misaligned reference rejection;
- unsupported packed access;
- overlapping fields;
- out-of-bounds fields;
- little-endian scalar fields;
- big-endian scalar fields;
- invalid endian annotation on pointer-like fields.

## Representation-validity tests

- valid and invalid bool representations;
- valid and invalid runes;
- valid and invalid enum values;
- valid and invalid union tags;
- raw storage without initialization;
- zero initialization of valid and invalid types;
- opaque descriptor rejection.

## Generic and plan tests

- same generic instance under one plan;
- different generic arguments;
- same type under 32-bit and 64-bit plans;
- reference-profile layout differences;
- multi-output build verification;
- layout cache isolation by plan.

## Query tests

- `SizeOf`;
- `AlignOf`;
- `StrideOf`;
- `FieldOffset`;
- incomplete-layout rejection;
- non-stored-member rejection;
- plan-specific results;
- value-form receiver effects.

---

# Implementation requirements

The compiler implementation should proceed in this order.

## Phase 1: canonical data model

Add canonical compiler structures for:

- `ResolvedLayout`;
- `ResolvedFieldLayout`;
- padding ranges;
- tag layout;
- payload layout;
- representation validity;
- layout stability;
- explicit contracts.

## Phase 2: scalar consistency

Make scalar layout consistent across:

- semantic analysis;
- MLIR;
- direct LLVM paths that remain;
- enum lowering;
- `SizeOf`;
- FFI validation.

In particular:

- make `int` and `uint` pointer-sized for the active plan;
- remove hard-coded default enum `i32` lowering;
- use `{ i64, i32 }` as canonical `decimal` layout;
- use `{ i128, i32 }` as canonical `decimal128` layout.

## Phase 3: aggregates

Implement shared layout calculation for:

- structs;
- fixed arrays;
- enums;
- unions;
- registers;
- descriptor types.

## Phase 4: dependency analysis

Implement:

- by-value layout dependency graph;
- recursive-cycle detection;
- generic instantiation layout resolution;
- canonical layout caching.

## Phase 5: explicit contracts

Implement validation for:

- full explicit field offsets;
- explicit alignment;
- explicit total size;
- packing;
- endianness.

Surface syntax may be added through the attributes rulebook.

## Phase 6: queries and diagnostics

Implement:

- `AlignOf`;
- `StrideOf`;
- `FieldOffset`;
- full diagnostic categories;
- LSP layout display.

## Phase 7: backend consumption

Update MLIR and remaining backend paths to consume canonical resolved layout.

Remove competing layout calculations where possible.

## Phase 8: synchronization

Synchronize at least:

```text
types.md
collections.md
struct.md
enums.md
unions.md
declarations/registers.md
platform/fixed-address-bindings.md
compiler_known_members.md
reference_model.md
platform/ffi.md
semantic_ir.txt
storage.md
language-rulebook-status.md
rules_implementations.txt
```

---

# Required synchronization decisions

The following existing statements must be updated when this rulebook is
integrated.

## Pointer-sized `int` and `uint` migration

Any implementation path treating `int` or `uint` as universally 32-bit must be
changed.

They are pointer-sized for the active `CompilationPlan`.

## Default enum lowering

The existing hard-coded rule:

```text
default enum int lowers to i32
```

must be replaced.

A default enum uses the active plan's `int` layout.

## `T[]`

A bare `T[]` is a sized owning descriptor value.

Older wording that calls bare `T[]` itself unsized must be corrected.

The runtime-sized element backing is separate from the descriptor.

## Decimal representation

The canonical layout is:

```text
decimal:
    { i64, i32 }

decimal128:
    { i128, i32 }
```

Legacy direct LLVM representations must not remain normative.

## Semantic IR

Target-independent Semantic IR may retain unresolved layout requirements.

Before layout-sensitive MLIR or backend lowering, every required concrete layout
must be attached or referenced through the canonical resolved layout model.

---

# Design summary

```text
Layout is per concrete CompilationPlan.

Semantic layout defines target-independent language guarantees.

Native layout defines one plan-selected physical representation.

Explicit layout locks only the properties named by its contract.

Stored struct fields retain declaration order.

Native padding is not semantic data.

Fixed-width scalar sizes are exact.

int and uint are pointer-sized.

Stored bool uses one byte.

char uses one byte.

rune uses 32 bits.

decimal uses coefficient int64 plus scale int32.

decimal128 uses coefficient int128 plus scale int32.

Named types, contracts, units, methods, and properties add no hidden per-value
storage.

Empty structs may have size zero and alignment one.

Fixed arrays use T[N], inline contiguous element slots, and no hidden
descriptor.

T[] is a sized owning descriptor whose separate backing storage is excluded from
the internal/type descriptor-size query; instance `values.SizeOf` is the live
payload byte extent.

Slices and safe references use profile-selected sized representations.

Enums use exactly their resolved underlying representation.

Default enum underlying int is pointer-sized.

Sec unions use an explicit declaration-order tag and largest-payload storage.

Sec 0.1 performs no niche optimization.

Registers retain exact semantic bit layout.

Packing may create misaligned fields.

A normal safe reference must not point directly to a misaligned field.

Endianness describes stored representation, not semantic numeric value.

Size and alignment do not establish representation validity.

Raw storage does not contain a live object until valid initialization completes.

Zero bytes are not a universal default object representation.

Native object bytes are not a stable serialization format.

Physical similarity does not create type or layout compatibility.

General reinterpretation requires unsafe.

Layout queries are plan-specific compile-time operations returning uint.

Direct recursive by-value layout is invalid.

Generic layout is resolved per concrete monomorphized instance.

One shared compiler layout phase resolves physical layout.

Backends consume the resolved Sec layout and must not redefine it.
```
