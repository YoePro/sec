# Types

- **Status:** Normative
- **Created:** 2026-08-12
- **Last updated:** 2026-08-24
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/types/types.md`
- **Replaces:** `rules/types/types.txt`

## Purpose

This rulebook defines the central type model of Sec.

It is the canonical entry point for:

- compiler-known type families;
- type identity;
- type naming classes;
- variable declaration forms;
- literal family selection;
- contextual literal typing;
- assignability;
- explicit conversion;
- named types;
- constrained named types;
- defaultability at the type-system boundary;
- the relationship between type semantics and ownership, references, collections, units, and error handling.

Specialized rulebooks define the detailed semantics of individual type families.

This rulebook replaces the previous `types.txt`.

Implementation status does not belong in this rulebook. It is maintained separately.

---

# Core principles

Sec is statically typed.

Every expression that survives semantic analysis has a resolved type.

Type semantics are defined by Sec and are independent of the compiler backend.

A physical representation does not define type identity.

Two types may use the same representation while remaining semantically different.

The compiler must not infer source-language type semantics from LLVM, MLIR, ABI layout, machine width, or another backend representation.

---

# Type categories

Sec distinguishes several broad type categories.

The categories are semantic. They do not require one common runtime representation.

| Category | Examples |
|---|---|
| special types | `any`, `void` |
| logical and text scalar types | `bool`, `byte`, `char`, `rune`, `string` |
| signed integer types | `int`, `int8`, `int16`, `int32`, `int64`, `int128`, `int256` |
| unsigned integer types | `uint`, `uint8`, `uint16`, `uint32`, `uint64`, `uint128`, `uint256` |
| binary floating-point types | `float`, `float32`, `float64` |
| exact decimal types | `decimal`, `decimal128` |
| temporal types | `date`, `time`, `datetime`, `duration` |
| fixed sequences | `T[N]` |
| owning unsized sequences | `T[]` |
| borrowed slices | `ref T[]`, `ref mut T[]` |
| safe references | `ref T`, `ref mut T` |
| raw addresses | `RawPtr[T]` |
| callable types | `fn(T, U) R`, `mut fn() R`, `-> fn() R` |
| first-class collections | `list`, `map`, `set` |
| first-class shaped types | `vector`, `matrix`, `tensor`, `tensor_view` |
| hardware layout forms | `bit`, `bit[N]`, `register[N]` |
| nominal core types | `Result[T, E]`, `Option[T]`, `Task[T]`, `Thread[T]`, `Shape[Rank]`, `Strides[Rank]`, `TensorLayout[Rank]`, `MemorySpace` |
| user-defined nominal types | named scalar types, structs, enums, unions, interfaces, units, registers |

This table is an overview.

The detailed rules remain in the specialized rulebooks.

---

# Compiler-known lowercase types

Fundamental language types and first-class compiler-known type constructors use lowercase names.

They require no import.

The canonical compiler-known lowercase set includes:

```text
any
bool
byte
char
error
rune
string
void

int
int8
int16
int32
int64
int128
int256

uint
uint8
uint16
uint32
uint64
uint128
uint256

float
float32
float64

decimal
decimal128

date
time
datetime
duration

bit
register

list
map
set

vector
matrix
tensor
tensor_view
```

These names are language-level names rather than standard-library aliases.

A compiler-known lowercase type constructor may have type arguments, compile-time value arguments, or both.

Examples:

```sec
list[int]
map[string, User]
set[UserID]

vector[float64, 3]
matrix[float32, 4, 4]
tensor[float32, 3, 224, 224]
tensor_view[float32, 3]

bit[8]
register[32]
```

The exact parameter rules are defined by the owning rulebook.

---

# Lowercase and uppercase naming

Compiler-known fundamental types and compiler-known fundamental type constructors use lowercase names.

Named and nominal types use uppercase names.

Examples:

```sec
int
string
list[int]
matrix[float32, 4, 4]
```

compared with:

```sec
Percent
User
Result[int, IOError]
Option[string]
Task[Response]
Shape[3]
TensorLayout[2]
```

User-defined named types follow the normal named-type naming rules.

Uppercase nominal core types remain predeclared compiler/core symbols rather than lowercase language constructors.

Important examples include:

```text
Result
Option
Task
Thread
Shape
Strides
TensorLayout
MemorySpace
RawPtr
```

They are not made lowercase merely because the compiler knows about them.

---

# Contextual `set`

`set` is a compiler-known lowercase collection type constructor in type context:

```sec
let values: set[int]
```

It is also the property setter spelling inside property grammar:

```sec
property Value: int {
    set value {
        _value = value
    }
}
```

Outside those contexts, `set` may be an ordinary identifier:

```sec
let set := 10
```

The lexer must not require separate spellings for collection `set`, property `set`, and an ordinary identifier.

The parser resolves the role from context.

---

# Predeclared nominal core types

Some types are available without an ordinary user import while retaining uppercase nominal names.

Examples include:

```sec
Result[T, E]
Option[T]

Task[T]
Thread[T]

Shape[Rank]
Strides[Rank]
TensorLayout[Rank]
MemorySpace

RawPtr[T]
```

Being compiler-known does not make these types structural.

Their identity and behavior remain nominal or otherwise defined by their owning rulebooks.

---

# Special types

## `void`

`void` represents the absence of a returned value.

It is used as a function result type:

```sec
fn Notify(message: string) void {
}
```

`void` is not an ordinary storable value type.

Ordinary variables and stored fields cannot have type `void`.

Special compiler-known forms may use `void` as a type argument where the owning rule explicitly permits it.

Example:

```sec
RawPtr[void]
Result[void, IOError]
```

The owning type defines what `void` means in that position.

## `any`

`any` is a compiler-known general-value type.

It does not disable static type checking.

Using `any` must not create implicit unchecked conversion, hidden reflection, or unrestricted dynamic member access.

Operations that recover or narrow a concrete type from `any` require defined type-safe semantics.

The complete representation and narrowing model for `any` must remain independent of backend implementation.

Until a more specialized rulebook defines additional behavior, `any` must not be treated as permission for implicit conversion between unrelated concrete types.

---

# Logical and text scalar types

## `bool`

`bool` has exactly two logical values:

```sec
true
false
```

Conditions require `bool`.

Numeric values do not implicitly become `bool`.

Explicit numeric-to-boolean conversion is permitted:

```sec
let ready := bool(value)
```

The result is:

```text
zero       -> false
non-zero   -> true
```

This conversion is explicit and infallible.

## `byte`

`byte` is the compiler-known byte scalar type.

It represents one byte-sized unsigned value.

Byte-oriented storage and FFI may use `byte` where the semantic meaning is raw byte data.

Detailed ABI identity relative to fixed-width integer types is defined by the ABI and layout rules and must not be inferred merely from equal representation size.

## `char`

`char` is Sec's character scalar type.

A character literal is `char` by default when no stronger context selects another compatible scalar type:

```sec
let ch := 'A'
```

The numeric family suffix for `char` is:

```text
t
```

Examples:

```sec
let zero := 0t
let a := 65t
let hexA := 0x41t
```

The canonical explicit zero `char` literal is:

```sec
0t
```

## `rune`

`rune` is Sec's Unicode scalar type.

The numeric family suffix is:

```text
r
```

Examples:

```sec
let zero := 0r
let omega := 0x03A9r
let face := 0x1F600r
```

The canonical explicit zero `rune` literal is:

```sec
0r
```

A character literal may be shaped to `rune` by context:

```sec
let r: rune := 'A'
```

`char` and `rune` remain distinct types.

Conversions between them follow explicit conversion and representability rules.

## `string`

`string` is Sec's compiler-known string type.

A string is not an implicit array of `char`, `rune`, or `byte`.

Conversions between strings and arrays, slices, bytes, or runes use explicitly defined core/library operations.

Array slice syntax does not apply directly to `string`. Checked substring
extraction is provided by the public core string API. Core may implement it
with a source-file-private unchecked helper as defined by
`compiler_known_members.md`; that helper is not part of the public string API
and does not become compiler-known solely because its body is low-level.

String representation is not defined by this rulebook as an ABI-stable struct.

---

# Integer types

Sec provides signed and unsigned integer families.

## Signed

```text
int
int8
int16
int32
int64
int128
int256
```

## Unsigned

```text
uint
uint8
uint16
uint32
uint64
uint128
uint256
```

The explicit-width types have the width named by the type.

The large integer types:

```text
int128
int256
uint128
uint256
```

are active Sec language types.

They must not be described as future or merely planned language types.

Implementation coverage is tracked outside this rulebook.

## `int` and `uint`

`int` and `uint` are target-sized integer types.

Their physical width is selected by the target profile.

Code that requires an exact ABI width must use a fixed-width type.

Examples:

```sec
int32
uint64
int128
```

Target selection must not change signedness or ordinary integer semantics.

---

# Binary floating-point types

Sec provides:

```text
float
float32
float64
```

`float32` and `float64` request explicit binary floating-point widths.

`float` is the general target/compiler-known binary floating-point type.

Code that requires an exact external representation must use an explicit-width type.

The numeric family suffix for binary floating point is:

```text
g
```

Examples:

```sec
let value := 1.5g
let whole := 10g
let hexadecimalValue := 0x10g
```

A `g` suffix selects the binary floating-point family.

It does not select a fixed width.

Context may select an explicit width:

```sec
let small: float32 := 1.5g
let wide: float64 := 1.5g
```

---

# Exact decimal types

Sec provides:

```text
decimal
decimal128
```

Both belong to the exact base-10 decimal family.

`decimal` is the default exact decimal type for an unsuffixed fractional or exponent literal when no stronger context selects another type.

Example:

```sec
let value := 3.14
```

The inferred type is:

```text
decimal
```

Binary floating point must be requested by type context or the `g` family suffix:

```sec
let binary: float64 := 3.14
let binaryLiteral := 3.14g
```

The numeric family suffix for exact decimal is:

```text
m
```

Examples:

```sec
let whole := 10m
let exact := 3.14m
let scientific := 1.25e-3m
let hexadecimalValue := 0x10m
```

The suffix selects the decimal family rather than a fixed width.

Context may select `decimal128`:

```sec
let wide: decimal128 := 8m
```

A decimal literal must be interpreted from its exact source spelling.

It must not first pass through binary floating point.

This rulebook does not define decimal storage layout, helper functions, MLIR representation, or LLVM representation.

Those are implementation and lowering concerns.

`decimal32` and `decimal64` are not part of the canonical Sec type set defined by this rulebook.

---

# Temporal types

Sec provides four compiler-known temporal value types:

```text
date
time
datetime
duration
```

They require no import.

Their basic meanings are:

```text
date
    a calendar date

time
    a time of day

datetime
    a combined date and time value

duration
    an elapsed amount of time
```

These are first-class Sec types.

They are not aliases for integers or strings.

They do not implicitly convert to or from numeric types.

Detailed rules for:

- calendars;
- time zones;
- offsets;
- UTC relationships;
- precision;
- formatting;
- parsing;
- arithmetic;
- comparison;
- literals;
- defaults;
- serialization;

belong in the temporal rulebook.

Until those rules are defined, this rulebook establishes the types and their identity but does not invent additional temporal semantics.

---

# Numeric literal family suffixes

Sec uses explicit one-letter lowercase suffixes when the programmer wants a literal to communicate its scalar family directly.

The canonical suffixes are:

```text
i    signed integer family
u    unsigned integer family
g    binary floating-point family
m    exact decimal family
t    char scalar type
r    rune scalar type
```

Suffixes are:

- lowercase;
- case-sensitive;
- adjacent to the literal;
- part of the literal spelling.

Examples:

```sec
8i
8u
8g
8m
65t
65r
```

The family suffix does not normally select a fixed numeric width.

Examples:

```sec
let small: int8 := 8i
let wide: int256 := 8i

let smallFloat: float32 := 8g
let wideFloat: float64 := 8g

let exact: decimal128 := 8m
```

`t` and `r` select the exact scalar types `char` and `rune`.

---

# Base-prefixed literals and suffixes

Integer-form literals may use binary, octal, decimal, or hexadecimal spelling.

The scalar family suffix remains unambiguous because the canonical suffix letters are not hexadecimal digits.

Examples:

```sec
0b1000001t
0o101t
0x41t

0x41i
0x41u
0x41g
0x41m
0x41r
```

For `g` and `m`, a base-prefixed integer-form literal denotes that integer value represented in the selected scalar family.

Example:

```sec
0x10g
```

denotes the numeric value sixteen in the binary floating-point family.

It is not C-style hexadecimal floating-point syntax.

Fractional and scientific-exponent forms use decimal-form source syntax and accept only:

```text
g
m
```

Examples:

```sec
1.5g
1.5m
1e3g
1e3m
```

A `t` or `r` suffix is valid only on integer-form literals.

---

# Breaking suffix migration

The canonical suffix set changed during the Sec 0.1 design process.

The old spellings:

```text
c    old char suffix
d    old decimal suffix
f    old binary-float suffix
```

are replaced by:

```text
t    char
m    decimal
g    binary float
```

The canonical migration is:

```text
c -> t
d -> m
f -> g
```

Examples:

```sec
65c        // old
65t        // canonical

3.14d      // old
3.14m      // canonical

3.14f      // old
3.14g      // canonical
```

This is a breaking lexical change.

The compiler implementation, bootstrap lexer, formatter, tests, examples, manuals, diagnostics, and generated knowledge documents must be synchronized.

Where the old spelling can be recognized unambiguously, the compiler should issue a focused migration diagnostic rather than a generic malformed-literal diagnostic.

Examples:

```text
literal suffix 'c' was replaced by 't' for char
literal suffix 'd' was replaced by 'm' for decimal
literal suffix 'f' was replaced by 'g' for binary float
```

No compatibility mode is part of the language rule.

The old spellings are not alternate canonical syntax.

---

# Unsuffixed literal inference

Literal tokens preserve exact source information until semantic context resolves their type.

## Integer-form literal

An unsuffixed integer-form literal defaults to:

```text
int
```

when no stronger type context exists.

Example:

```sec
let value := 10
```

`value` has type `int`.

## Fractional or exponent literal

An unsuffixed fractional or scientific-exponent literal defaults to:

```text
decimal
```

Example:

```sec
let value := 3.14
let scientific := 1e3
```

Both are exact decimal-family values unless context selects another compatible type.

## Character literal

A character literal defaults to:

```text
char
```

but may be shaped to `rune` by compatible context.

Example:

```sec
let ch := 'A'
let r: rune := 'A'
```

## Context shaping

An untyped literal may be shaped directly by declaration or expression context.

Examples:

```sec
let small: int8 := 10
let wide: uint256 := 10
let ratio: float64 := 3.14
let exact: decimal128 := 3.14
let percent: Percent := 50
```

The compiler validates representability and contracts against the target type.

Context shaping of an untyped literal is not implicit conversion of an already typed runtime value.

---

# Variable declarations

Variables are immutable by default.

The normal inferred forms are:

```sec
let a := 9
let mut a := 9
```

An explicit type may be supplied:

```sec
let a: int := 9
let mut a: int := 9
```

A mutable declaration may omit the initializer only when the declared type is defaultable:

```sec
let mut a: int
```

An immutable declaration requires an initializer:

```sec
let a: int
```

is invalid.

The compiler does not create an undefined immutable value.

---

# Multiple `let` declarators

A `let` declaration may contain multiple declarators.

Examples:

```sec
let a := 9, b := "hello", c := true
let mut a := 9, b := 10, c := 11
```

Each declarator is checked independently.

With inferred declarations, each declarator receives the type of its own initializer.

Example:

```sec
let a := 9, b := "hello", c := true
```

declares:

```text
a : int
b : string
c : bool
```

---

# Type-first declarations

Sec also supports type-first declarations.

A single declaration is valid:

```sec
int mut: a
```

This is as valid as:

```sec
let mut a: int
```

The type-first form is not restricted to multiple declarations.

Multiple declarations are also valid:

```sec
int mut: a, b, c
```

An immutable type-first declaration requires an initializer:

```sec
int: a := 1
```

Multiple immutable values may be declared:

```sec
int: a := 1, b := 2, c := 3
```

A mutable type-first declarator may omit its initializer only when the type is defaultable:

```sec
int mut: a
```

For a non-defaultable type, an initializer remains required.

The explicit type applies to every declarator in the statement.

---

# Parenthesized immutable type-first groups

Sec supports a parenthesized immutable type-first declaration group.

Canonical syntax:

```sec
int (
    a := 1,
    b := 2,
    c := 3,
)
```

This form declares immutable typed values.

It does not construct an aggregate value.

The parentheses intentionally distinguish a declaration group from value construction and block syntax.

Rules:

- the type appears once before the group;
- the group uses `(` and `)`;
- every entry requires an initializer;
- every entry has the declared type;
- a trailing comma is allowed;
- the group is immutable;
- this form does not use `{` and `}`.

For example:

```sec
TokenType (
    ILLEGAL := "ILLEGAL",
    EOF := "EOF",
    IDENT := "IDENT",
)
```

is a declaration group, not construction of a `TokenType` value.

---

# Type identity

Type identity is stronger than representation identity.

Two values do not have the same type merely because their storage representation is equal.

## Named types

A user-defined named type is distinct from its underlying type.

Example:

```sec
type Percent int range 0..100
type Age int range 0..130
```

The following types are distinct:

```text
Percent
Age
int
```

Invalid:

```sec
let age: Age := 20
let percent: Percent := age
```

No implicit conversion is performed merely because both types use integer semantics.

Named types may be generic:

```sec
type ID[T] int
type Wrapped[T] T
```

Concrete instantiations preserve nominal identity. `ID[User]` and
`ID[Product]` are distinct types even when their substituted representation is
identical. After substitution, ordinary named-type conversion, contracts,
defaultability, copy/move behavior, layout, interface conformance, and ABI rules
apply to the concrete type.

## Generic and parameterized types

Type arguments participate in type identity.

Examples:

```text
list[int]
list[string]

Pair[int, string]
Pair[string, int]

Shape[2]
Shape[3]
```

These are distinct types or distinct concrete instantiations as defined by their owning type families.

## Arrays

Array length participates in type identity:

```text
int[4]
int[5]
```

are distinct.

## References

Reference mode participates in type identity:

```text
ref T
ref mut T
```

are distinct reference types.

## Function types

Callable capability, ordered parameter types and ownership modes, variadic
shape, and return type participate in function-type identity.

Example:

```text
fn(int) bool
fn(int) int
mut fn() int
-> fn() Resource
fn(ref Buffer) void
fn(ref mut Buffer) void
fn(-> Buffer) void
fn(...int) int
```

are distinct.

---

# Assignability

Assignment requires semantic compatibility between source and target.

The compiler must not use equal physical representation as evidence of assignability.

A value of an exact named type may be assigned to another location of that exact type subject to mutability, ownership, and borrowing rules.

Example:

```sec
let a: Percent := 20
let mut b: Percent := 10

b = a
```

This assignment does not recreate the `Percent` contract obligation.

`a` is already a valid `Percent`.

The assignment remains subject to normal copy/move rules.

---

# Untyped literals versus typed values

Untyped literals may be shaped by context.

Typed runtime values are not implicitly changed into unrelated named or numeric types.

Valid:

```sec
let p: Percent := 90
```

Invalid:

```sec
let value: int := 90
let p: Percent := value
```

Use explicit conversion:

```sec
let p := try Percent(value)
```

when the conversion may fail.

This distinction is fundamental:

```text
literal shaping
    resolves an untyped literal in target context

conversion
    changes an already typed value into another type
```

They are not the same operation.

---

# Explicit conversions

The general conversion spelling is:

```sec
TargetType(value)
```

Examples:

```sec
int32(value)
decimal(value)
bool(value)
char(value)
rune(value)
Percent(value)
```

Conversions are checked.

They must not silently truncate, wrap, discard precision, violate a contract, or create an invalid scalar value unless a separately named lossy operation explicitly defines that behavior.

A conversion that may fail requires `try`.

Examples:

```sec
let small := try int32(largeValue)
let r := try rune(codePoint)
let p := try Percent(raw)
```

When the compiler proves a conversion valid from compile-time information, no runtime validation is emitted.

The language semantics remain checked.

Lossy, wrapping, saturating, or otherwise intentionally non-preserving conversion semantics require separately defined explicit operations.

---

# Named types and contracts

A named type retains nominal identity.

Example:

```sec
type Percent int range 0..100
```

The contract belongs to the type.

It does not belong to one variable.

A compile-time literal may be shaped directly to the named type when it satisfies every contract:

```sec
let p: Percent := 50
```

This is invalid:

```sec
let p: Percent := 101
```

Runtime construction may be fallible:

```sec
let p := try Percent(raw)
```

The detailed contract vocabulary is defined by `contracts.md` and `runtime_checks.md`.

---

# Assignment of constrained named types

An already valid value of a constrained named type may be assigned to the same type without revalidating the contract merely because storage changes.

Example:

```sec
let source: Percent := 91
let mut target: Percent := 10

target = source
```

The value `source` is already a valid `Percent`.

The compiler must not insert redundant contract failure semantics for the plain same-type assignment.

This is different from an operation that computes a new constrained value.

---

# Value-producing mutation of constrained named types

A compound assignment computes a new value.

For a constrained named type, the new value may violate the contract even when every input value is individually valid.

Example:

```sec
let mut percentage: Percent := 91
let additional: Percent := 10
```

The mathematical result of:

```sec
percentage += additional
```

would be:

```text
101
```

which is not a valid `Percent`.

Therefore compound assignment to a constrained named type is a fallible value-producing mutation and uses the canonical `try` assignment form:

```sec
try percentage += additional {
    Err(error) => {
        Handle(error)
    }
}
```

The same principle applies to other operations that produce a new value for constrained storage.

The compiler may eliminate a runtime check when proof establishes that failure is impossible, but it must preserve the language's fallibility and contract semantics.

---

# Defaultability

Every type is classified as either:

```text
Defaultable
NonDefaultable
```

A defaultable type has one compiler-resolved valid default value.

A non-defaultable type cannot be used without an initializer in a context that requires default initialization.

Example:

```sec
int mut: value
```

is valid because `int` is defaultable.

For named and aggregate types, defaultability is resolved according to `default_values.md`.

The compiler must never treat undefined storage as a default value.

This rulebook does not duplicate the complete default-selection algorithm.

---

# Arrays, owning sequences, and slices

Sec uses postfix sequence syntax.

Fixed array:

```sec
T[N]
```

Owning unsized sequence:

```sec
T[]
```

Borrowed slice:

```sec
ref T[]
ref mut T[]
```

Examples:

```sec
int[4]
byte[256]

Packet[]

ref byte[]
ref mut int[]
```

Array length is part of type identity.

Slices are non-owning references to sequence storage.

The complete rules for layout, indexing, slicing, ownership, defaultability,
and bounds are defined by `collections.md`.

---

# Safe references

Safe reference forms are:

```sec
ref T
ref mut T
```

A reference does not own the referenced value.

`ref T` provides shared access.

`ref mut T` provides exclusive mutable access.

Sec does not expose lifetime parameters in ordinary source syntax.

Reference validity, provenance, generations, storage epochs, borrowing, and escape behavior are defined by the reference and lifetime rulebooks.

The physical representation of a safe reference is not fixed by this rulebook.

---

# Raw pointers

Unchecked foreign or low-level addresses use:

```sec
RawPtr[T]
```

Example:

```sec
let address: RawPtr[byte]
let opaque: RawPtr[void]
```

`RawPtr[T]` is not a safe reference.

It does not imply ownership.

It does not extend lifetime.

Unsafe conversion or dereference rules are defined by `raw_pointers` and FFI rulebooks.

---

# Function types

Function values have a statically known function type.

Syntax:

```sec
fn(ParameterType, ParameterType) ReturnType
```

Examples:

```sec
fn(int) bool
fn(int, int) int
fn(string) void
```

Parameter names are not part of function-type identity.

Callable capability and parameter consumption are distinct. For example,
`-> fn(-> Resource) Handle` consumes both the callable environment and its
argument. Parameter names and individual capture contents are not part of
source-level callable type identity, although captures determine capability,
copy/move classification, and lifetime.

Lambda, closure, capture, call, and ownership rules are defined by the function and closure rulebooks.

---

# First-class collections

The compiler-known first-class collection constructors are:

```sec
list[T]
list[T, Capacity]

map[K, V]
map[K, V, Capacity]

set[T]
set[T, Capacity]
```

They require no import.

They are not aliases for standard-library generic types.

Their semantics benefit from compiler knowledge, including capacity, allocation, ownership, indexing/lookup, analysis, and lowering.

More specialized data structures remain nominal library types.

Examples:

```sec
LinkedList[T]
OrderedMap[K, V]
OrderedSet[T]
```

Detailed collection semantics are defined by `collections.md`.

---

# First-class shaped types

The compiler-known shaped type constructors are:

```sec
vector[T, N]
matrix[T, Rows, Columns]
tensor[T, Dimensions...]
tensor[T, Shape[Rank]]
tensor_view[T, Rank]
```

They require no import.

Supporting nominal types include:

```sec
Shape[Rank]
Strides[Rank]
TensorLayout[Rank]
Axes[Rank]
AxisList[Count]
MemorySpace
StorageRequest
ShapedStorageRequest[Rank]
```

The lowercase shaped constructors are fundamental compiler-known type forms.

The uppercase supporting types remain nominal core types.

`tensor[T, Dimensions...]` is an owning tensor whose rank and extents are
compile-time-known. `tensor[T, Shape[Rank]]` is an owning runtime-shaped tensor:
its rank is compile-time-known while its extents are runtime values. It must not
be reinterpreted as `tensor_view[T, Rank]`, which remains a compiler-known
non-owning view type form. Safe source-level views use `ref tensor_view[T, Rank]`
or `ref mut tensor_view[T, Rank]`.

The supporting shaped types are nominal semantic types, not aliases for
ordinary arrays or lists.

Physical layout is not inferred from the source spelling unless the shaped-type rulebook explicitly makes layout observable.

Detailed shape, stride, layout, memory-space, view, and matrix-operation rules are defined by `shaped-types.md`.

---

# Hardware layout types

Sec has compiler-known hardware-oriented forms:

```sec
bit
bit[N]
register[N]
```

These are language forms rather than standard-library generics.

They participate in compile-time layout semantics.

Example:

```sec
type Control register[8] {
    Enabled: bit
    Mode: bit[3]
    _: bit[4]
}
```

Detailed register and bit-field semantics are defined by the register rulebook.

---

# Units

Units are semantic quantity identities. A unit is not itself a numeric
representation; a unit-bearing numeric type combines a numeric carrier with a
unit semantic descriptor.

Examples:

```sec
unit Meter physical
unit Second physical
unit Item uint other

let exact: decimal<Meter> := 10
let measured: float64<Meter> := 10
let count: <Item> := 4
```

The optional declaration carrier is only the default used by `<NamedUnit>`; it
is not part of unit identity. A compound unit-only expression such as `<m/s>`
defaults to `decimal`.

The complete unit declaration, structural algebra, Kind, transform, conversion,
and system rules are defined by `units.md`.

Named unit types remain distinct according to their unit semantics.

---

# Core generic result and option types

`Result[T, E]` and `Option[T]` are nominal core generic types.

They are not special parser spellings for ordinary type references.

`Result[T, E]` participates in explicit typed error handling. `E` must be the
compiler-known `error` root or a concrete Sec type declared as an error type.
`Result[T, string]` is invalid.

For a concrete error type, implicit widening `ConcreteError -> error` is valid
and preserves concrete type identity, active variant, payload, ownership, and
destruction obligations. Implicit `error -> ConcreteError` narrowing and
cross-concrete error conversion remain invalid. This relation is error-specific
and does not introduce general inheritance.

`Option[T]` represents explicit optionality.

Safe references are non-null.

Optional reference semantics therefore use an explicit optional type rather than nullable `ref`.

Detailed behavior is defined by the error-handling, union, and reference rulebooks.

---

# Type inference

Type inference resolves types from explicit language information.

It must not guess between semantically distinct types.

Inference may use:

- literal kind;
- target type;
- declaration context;
- function parameters;
- generic constraints and arguments;
- expected result type where defined;
- operator semantics;
- pattern context.

Inference must preserve named type identity.

The compiler must reject ambiguous inference rather than silently choose an unrelated type.

---

# No implicit conversion

Sec does not use broad implicit conversion between already typed values.

Examples:

```sec
let age: Age := 20
let percent: Percent := age
```

is invalid.

Likewise, binary float, decimal, integer, char, rune, and named numeric values do not silently change type merely because a machine representation could support the conversion.

Use:

- literal context;
- explicit conversion;
- explicit unit conversion;
- a named lossy operation;
- another operation defined by the owning rulebook.

---

# Type semantics and ownership

Type compatibility does not decide ownership transfer by itself.

The compiler separately determines whether an operation:

- copies;
- moves;
- borrows;
- mutates;
- destroys;
- reuses an SSA value.

Copyability and move-only classification are semantic properties of the resolved type.

Detailed rules are defined by ownership, copy/move, borrowing, lifetime, and destruction rulebooks.

Backend loads and stores must not be used to infer source-level copy or move semantics.

---

# Must-use and discardability

`must-use` and `discardability` are distinct semantic properties of a resolved
type or concrete type instance.

```text
must-use
    an unnamed produced value may not disappear through implicit discard

discardable
    explicit terminal discard is legal
```

A type may therefore be must-use while remaining explicitly discardable.

Canonical examples include:

```text
Result[int, IOError]
    must-use
    discardable

Thread[int] while unresolved
    must-use
    non-discardable

Result[Thread[int], SpawnError]
    must-use
    non-discardable while Ok may contain an unresolved Thread[int]

Option[int]
    not automatically must-use
    discardable
```

Discardability is recursive through concrete aggregate and variant payload
structure. It is not derived merely from copy classification or physical
representation.

Compiler-known core types may carry these properties before a general
user-defined must-use declaration mechanism exists. The source declaration
mechanism for user-defined must-use types remains owned by the attribute and
type-design rules.

Detailed terminal consumption and implicit call-result behavior is defined by
`rules/control-flow/discard.md`.

---

# Type semantics and representation

A source type may have different physical representations under different targets or profiles when the owning rule permits this.

Examples include:

- `int` and `uint` width;
- safe-reference metadata;
- collection storage;
- interface values;
- closure environments;
- shaped-value materialization.

Representation changes must not change source semantics.

Types that cross an ABI boundary require separately defined ABI-compatible representation.

The type system must not expose LLVM or MLIR implementation details as source-language truth.

---

# Diagnostics

Type diagnostics should identify:

- the expected type;
- the actual type;
- the source expression;
- the relevant named type or contract;
- the failed conversion or assignability rule;
- generic arguments where relevant;
- array or shaped dimensions where relevant.

For a breaking literal suffix migration, diagnostics should identify the old and new spelling directly.

Examples:

```text
literal suffix 'c' was replaced by 't' for char
literal suffix 'd' was replaced by 'm' for decimal
literal suffix 'f' was replaced by 'g' for binary float
```

A type diagnostic should explain semantic incompatibility rather than report only backend representation differences.

---

# Related rulebooks

Detailed rules are owned by the corresponding canonical rulebooks.

Important adjacent areas include:

```text
lexical_structure.md
grammar.md
contracts.md
runtime_checks.md
default_values.md
types/units.md
collections.md
shaped-types.md
reference_model.md
ownership.md
copy_move.md
borrowing.txt
lifetime_analysis.md
raw_pointers.txt
functions.md
declarations/lambda-functions.md
declarations/generics.md
declarations/interfaces.md
errorhandling.md
declarations/registers.md
platform/fixed-address-bindings.md
platform/ffi.md
control-flow/discard.md
storage.md
```

When an older filename still uses `.txt`, the next rewritten canonical rulebook must use `.md`.

---

# Design summary

The Sec type model follows these principles:

```text
type identity is semantic, not representational

untyped literals may be shaped by context

typed runtime values are not implicitly converted

named types remain nominal

contracts belong to types

plain same-type assignment preserves an already valid constrained value

value-producing constrained mutation must preserve or validate the contract

fundamental compiler-known type constructors use lowercase names

nominal types use uppercase names

first-class compiler-known types require no import

backend representation does not define source semantics
```
