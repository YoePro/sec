# Types

- **Status:** Normative
- **Created:** 2026-08-12
- **Last updated:** 2026-11-08 (stable chapter, subsection, and paragraph numbering); 2026-10-28 ("Contextual and self-inferred `Some(value)`", MD-102); 2026-10-28 ("Decimal scale", MD-066/MD-067/MD-068)
- **Document revision:** 2.6
- **Sec language version:** 0.1
- **Canonical path:** `rules/types/types.md`
- **Repository baseline reviewed:** `50655d3bc5d2e2596935dc5609cb1db1387e0cec`
- **Implementation governance:** `governance/types.yaml`
- **Replaces:** `rules/types/types.txt`

---

## § 1 Purpose

**§ 1(1)** This rulebook defines the central type model of Sec.

**§ 1(2)** It is the canonical entry point for:

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

**§ 1(3)** Specialized rulebooks define the detailed semantics of individual type families.

**§ 1(4)** This rulebook replaces the previous `types.txt`.

**§ 1(5)** Implementation status does not belong in this rulebook. It is maintained separately.

---

## § 2 Core principles

**§ 2(1)** Sec is statically typed.

**§ 2(2)** Every expression that survives semantic analysis has a resolved type.

**§ 2(3)** Type semantics are defined by Sec and are independent of the compiler backend.

**§ 2(4)** A physical representation does not define type identity.

**§ 2(5)** Two types may use the same representation while remaining semantically different.

**§ 2(6)** The compiler must not infer source-language type semantics from LLVM, MLIR, ABI layout, machine width, or another backend representation.

---

## § 3 Type categories

**§ 3(1)** Sec distinguishes several broad type categories.

**§ 3(2)** The categories are semantic. They do not require one common runtime representation.

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
| compiler-known interfaces | `Iterator[T]` |
| user-defined nominal types | named scalar types, structs, enums, unions, interfaces, units, registers |

**§ 3(3)** This table is an overview.

**§ 3(4)** The detailed rules remain in the specialized rulebooks.

---

## § 4 Compiler-known lowercase types

**§ 4(1)** Fundamental language types and first-class compiler-known type constructors use lowercase names.

**§ 4(2)** They require no import.

**§ 4(3)** The canonical compiler-known lowercase set includes:

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

**§ 4(4)** These names are language-level names rather than standard-library aliases.

**§ 4(5)** A compiler-known lowercase type constructor may have type arguments, compile-time value arguments, or both.

**§ 4(6)** Examples:

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

**§ 4(7)** The exact parameter rules are defined by the owning rulebook.

---

## § 5 Lowercase and uppercase naming

**§ 5(1)** Compiler-known fundamental types and compiler-known fundamental type constructors use lowercase names.

**§ 5(2)** Named and nominal types use uppercase names.

**§ 5(3)** Examples:

```sec
int
string
list[int]
matrix[float32, 4, 4]
```

**§ 5(4)** compared with:

```sec
Percent
User
Result[int, IOError]
Option[string]
Task[Response]
Shape[3]
TensorLayout[2]
```

**§ 5(5)** User-defined named types follow the normal named-type naming rules.

**§ 5(6)** Uppercase nominal core types remain predeclared compiler/core symbols rather than lowercase language constructors.

**§ 5(7)** Important examples include:

```text
Result
Option
Task
Thread
Atomic
MemoryOrder
CompareExchangeResult
Shape
Strides
TensorLayout
MemorySpace
RawPtr
```

**§ 5(8)** They are not made lowercase merely because the compiler knows about them.

**§ 5(9)** `Atomic[T]`, `MemoryOrder`, and `CompareExchangeResult[T]` are canonical
nominal core types. Exact atomic type eligibility and operation semantics belong
to `rules/concurrency/atomics.md`.

---

## § 6 Contextual `set`

**§ 6(1)** `set` is a compiler-known lowercase collection type constructor in type context:

```sec
let values: set[int]
```

**§ 6(2)** It is also the property setter spelling inside property grammar:

```sec
property Value: int {
    set value {
        _value = value
    }
}
```

**§ 6(3)** Outside those contexts, `set` may be an ordinary identifier:

```sec
let set := 10
```

**§ 6(4)** The lexer must not require separate spellings for collection `set`, property `set`, and an ordinary identifier.

**§ 6(5)** The parser resolves the role from context.

---

## § 7 Predeclared uppercase compiler and core types

**§ 7(1)** Some types and interfaces are available without an ordinary user import while
retaining uppercase names.

**§ 7(2)** Examples include:

```sec
Result[T, E]
Option[T]
Iterator[T]

Task[T]
Thread[T]

Shape[Rank]
Strides[Rank]
TensorLayout[Rank]
MemorySpace

RawPtr[T]
```

**§ 7(3)** Being compiler-known does not make these types structural.

**§ 7(4)** Their identity and behavior remain nominal or otherwise defined by their owning rulebooks.

**§ 7(5)** `Iterator[T]` is a compiler-known interface rather than a storable collection
type. Its explicit conformance, yielded type, and static `for` resolution are
defined by `rules/control-flow/flowcontrol_for.md`. Compiler knowledge of the
interface does not create a runtime interface representation.

---

## § 8 Special types

### § 8.1 `void`

**§ 8.1(1)** `void` represents the absence of a returned value.

**§ 8.1(2)** It is used as a function result type:

```sec
fn Notify(message: string) void {
}
```

**§ 8.1(3)** `void` is not an ordinary storable value type.

**§ 8.1(4)** Ordinary variables and stored fields cannot have type `void`.

**§ 8.1(5)** Special compiler-known forms may use `void` as a type argument where the owning rule explicitly permits it.

**§ 8.1(6)** Example:

```sec
RawPtr[void]
Result[void, IOError]
```

**§ 8.1(7)** The owning type defines what `void` means in that position.

### § 8.2 `any`

**§ 8.2(1)** `any` is a compiler-known general-value type.

**§ 8.2(2)** It does not disable static type checking.

**§ 8.2(3)** Using `any` must not create implicit unchecked conversion, hidden reflection, or unrestricted dynamic member access.

**§ 8.2(4)** Operations that recover or narrow a concrete type from `any` require defined type-safe semantics.

**§ 8.2(5)** The complete representation and narrowing model for `any` must remain independent of backend implementation.

**§ 8.2(6)** Until a more specialized rulebook defines additional behavior, `any` must not be treated as permission for implicit conversion between unrelated concrete types.

---

## § 9 Logical and text scalar types

### § 9.1 `bool`

**§ 9.1(1)** `bool` has exactly two logical values:

```sec
true
false
```

**§ 9.1(2)** Conditions require `bool`.

**§ 9.1(3)** Numeric values do not implicitly become `bool`.

**§ 9.1(4)** Explicit numeric-to-boolean conversion is permitted:

```sec
let ready := bool(value)
```

**§ 9.1(5)** The result is:

```text
zero       -> false
non-zero   -> true
```

**§ 9.1(6)** This conversion is explicit and infallible.

### § 9.2 `byte`

**§ 9.2(1)** `byte` is the compiler-known byte scalar type.

**§ 9.2(2)** It represents one unsigned 8-bit data byte with the complete value domain `0..255`.

**§ 9.2(3)** Byte-oriented storage and FFI may use `byte` where the semantic meaning is raw byte data.

**§ 9.2(4)** `byte` and `char` are distinct Sec types with potentially different members and intended uses, even though both use an identical 8-bit representation. Equal machine representation does not erase type identity: neither is implicitly assignable to the other and neither substitutes for the other in overload resolution.

**§ 9.2(5)** `byte(charValue)` and `char(byteValue)` are explicit, total, value-preserving conversions over `0..255`. The compiler may implement them without a machine instruction.

```sec
let character: char := 65t
let raw: byte := byte(character)
let restored: char := char(raw)
```

**§ 9.2(6)** Detailed ABI identity relative to fixed-width integer types is defined by the ABI and layout rules and must not be inferred merely from equal representation size.

### § 9.3 `char`

**§ 9.3(1)** `char` is Sec's character-oriented 8-bit scalar type with the complete value domain `0..255`.

**§ 9.3(2)** A `char` value is not inherently UTF-8 encoded. Its raw value can participate in byte-oriented encodings, including Latin-1, only when an explicitly chosen encoding interprets it. No ambient locale or encoding is inferred from a `char` value.

**§ 9.3(3)** `char` ordering is unsigned numeric order `0..255`; it is not Unicode, locale, or alphabetic collation.

**§ 9.3(4)** A single-quoted character literal is `rune` by default (see "Character literal"). An explicitly expected `char` may context-shape a single-quoted literal only when its decoded Unicode scalar value is in `0..255`. This literal shaping is not an implicit `rune`-to-`char` conversion of a previously typed expression:

```sec
let letter: char := 'a'   // contextual char literal, value 97
let accent: char := 'é'   // contextual char literal, value 233
let invalid: char := 'Ω'  // compile-time error: U+03A9 does not fit char
```

**§ 9.3(5)** An unsuffixed integer literal is not implicitly shaped to `char`. Character intent is expressed with the `t` suffix, a single-quoted literal in a valid `char` context, or an explicit conversion. This is a deliberate exception to integer-literal contextual shaping for integer-family types:

```sec
let correct: char := 65t
let alsoCorrect: char := char(65)
let incorrect: char := 65  // compile-time error: use 65t or char(65)
```

**§ 9.3(6)** The numeric family suffix for `char` is:

```text
t
```

**§ 9.3(7)** Examples:

```sec
let zero := 0t
let a := 65t
let hexA := 0x41t
```

**§ 9.3(8)** The canonical explicit zero `char` literal is:

```sec
0t
```

**§ 9.3(9)** A `t`-suffixed literal must be in `0..255`. A `t` literal above 255 is invalid even when it is a valid Unicode scalar value:

```sec
let maximum: char := 255t
let invalid: char := 256t   // compile-time error
```

### § 9.4 `rune`

**§ 9.4(1)** `rune` is Sec's Unicode scalar type. Its domain is `U+0000..U+10FFFF` excluding the surrogate values `U+D800..U+DFFF`. It is semantically distinct from `char` and `byte`, and `rune` values order by Unicode scalar numeric order.

**§ 9.4(2)** A single-quoted character literal is `rune` by default, independently of its scalar's numeric value. A default `rune` literal does not become `char` merely because its value is in `0..255`.

**§ 9.4(3)** The numeric family suffix is:

```text
r
```

**§ 9.4(4)** Examples:

```sec
let zero := 0r
let omega := 0x03A9r
let face := 0x1F600r
```

**§ 9.4(5)** The canonical explicit zero `rune` literal is:

```sec
0r
```

**§ 9.4(6)** Examples:

```sec
let letter := 'a'   // rune, U+0061
let pi := 'π'       // rune, U+03C0
let omega: rune := 'Ω'
```

**§ 9.4(7)** `char` and `rune` remain distinct types.

**§ 9.4(8)** Conversions between them follow explicit conversion and representability rules. A runtime `rune`-to-`char` conversion requires a range check when it cannot be proven statically; a compiler-known out-of-range constant is rejected at compile time.

### § 9.5 `string`

**§ 9.5(1)** `string` is Sec's compiler-known string type.

**§ 9.5(2)** A string is not an implicit array of `char`, `rune`, or `byte`.

**§ 9.5(3)** `Len` counts decoded Unicode scalar values without normalization. The
value-form `SizeOf` counts encoded UTF-8 payload bytes (MD-058). Both counts
exclude any external terminator. For example, `"éΩ"` has `Len == 2` and
`SizeOf == 4`. String length contracts use the units defined by
`rules/types/contracts.md` revision 2.1.

### Valid UTF-8 invariant (MD-050)

**§ 9.5(4)** Every `string` value in safe Sec code is a sequence of Unicode scalar values
encoded as well-formed canonical UTF-8. Invalid leading bytes, malformed or
truncated continuation sequences, overlong encodings, encoded surrogates and
scalar values beyond U+10FFFF never occur in a `string`.

**§ 9.5(5)** The invariant is enforced at every text-creation boundary: string literals,
binary decoding, string-returning runtime and FFI helpers, and every
compiler-known path that constructs or materializes a `string`. `unsafe` does
not authorize ordinary code to present malformed bytes as a `string`;
privileged or FFI code that bypasses validation carries an explicit trusted
obligation to establish the invariant before exposing the value.

**§ 9.5(6)** NUL (U+0000), control characters, combining marks and non-ASCII scalars are
valid content. No zero termination is implied. UTF-8 validity does not imply a
normalization form, collation, case folding, grapheme segmentation or any
protocol-specific textual policy.

**§ 9.5(7)** `string` is text, not a generic binary buffer. Arbitrary or malformed bytes
are valid binary data (`byte[]`, byte slices and other binary storage) and
never become a `string` without successful decoding: malformed UTF-8 received
from a socket, request or file is an ordinary recoverable input condition, not
a panic, assertion or memory fault. UTF-8 is the only canonical encoding;
ASCII, Latin-1, Windows code pages and UTF-16 do not become UTF-8 by
reinterpretation or guessing, and no encoding is inferred automatically.

**§ 9.5(8)** `byte`, `char` and `rune` remain distinct: a `char` (0..255) and a `char[]`
may hold values outside ASCII without being ASCII, UTF-8 or Latin-1 text, and
a `rune` is always a Unicode scalar value.

**§ 9.5(9)** `string.ToCharArray()` is byte-preserving (MD-073, 2026-10-31): it yields one
`char` per byte of the UTF-8 encoding with the identical numeric value, so
`"Ω".ToCharArray()` yields `0xCEt, 0xA9t`. It is not a `rune`-to-`char`
conversion and needs no range check. It does not make `char` Unicode text,
Latin-1 or an implicit alias of `byte`, and it does not change the `char`
domain, literal rules or the valid UTF-8 invariant (`core-library.md` § 5.3).

**§ 9.5(10)** Public string positions of the core search, slicing and rune-access members
are rune indexes; byte-access members use UTF-8 byte indexes and say so
(`core-library.md` § 5.7, MD-071).

**§ 9.5(11)** Conversions between strings and arrays, slices, bytes, or runes use explicitly defined core/library operations.

**§ 9.5(12)** String bracket ranges (MD-126, decided 2026-11-05, `rules/corrections/applied/MD-126-borrowed-string-ranges-rune-iteration-applied-20261105.md`). The range
forms `text[a..<b]`, `text[a..b]`, `text[a..]`, `text[..b]`, `text[..<b]` and
`text[..]` apply to a `string` or `StringView` source and create a borrowed
`StringView` **without** a preceding `ref`. Endpoints are zero-based rune
indexes relative to the source. Normalization, evaluation order and failure
precedence follow `collections.md` § 7.10. This is a source-kind-specific
exception: array and sequence slicing still requires `ref values[...]`, and no
other type gains implicit slice creation. A standalone `text[index]` is not
given meaning by this rule. Public `string.Slice(start, end)` is removed by
MD-126; `try text[a..<b]` is its non-panicking replacement.

**§ 9.5(13)** Core may implement owned substring helpers with a source-file-private
unchecked helper as defined by `compiler_known_members.md`; that helper is not
part of the public string API, is not the bracket-range constructor, and does
not become compiler-known solely because its body is low-level.

**§ 9.5(14)** `StringView` (MD-126) is a predeclared, compiler-known, read-only, non-owning
bounded view over valid UTF-8 text. It is distinct from `string`, from
`ref string` (a reference to a complete `string` Place) and from
`ref byte[]`/`ref rune[]`; user code cannot redeclare it. A view carries the
provenance and lifetime obligation of its originating string backing and may
not outlive it. Copying a view copies the borrow holder only, never text, and
never extends the backing lifetime; destroying a view is trivial and does not
destroy the source. Because its endpoints are scalar boundaries, a view is
always valid UTF-8; UTF-8 is neither normalized nor transcoded. `Len` counts
runes and `SizeOf` counts UTF-8 bytes within the view, and every member index
is view-relative. No implicit conversion from `StringView` to `string` exists,
and a `StringView` argument is not accepted by a `string` parameter without a
declared overload; explicit materialization is `ToString()`. No `ref mut`
form of a view exists. Its representation is implementation-defined and no
FFI/ABI-visible layout is selected. The source-visible member surface is
exactly `core-library.md` § 5.8.

**§ 9.5(15)** String representation is not defined by this rulebook as an ABI-stable struct.

---

## § 10 Integer types

**§ 10(1)** Sec provides signed and unsigned integer families.

### § 10.1 Signed

```text
int
int8
int16
int32
int64
int128
int256
```

### § 10.2 Unsigned

```text
uint
uint8
uint16
uint32
uint64
uint128
uint256
```

**§ 10.2(1)** The explicit-width types have the width named by the type.

**§ 10.2(2)** The large integer types:

```text
int128
int256
uint128
uint256
```

**§ 10.2(3)** are active Sec language types.

**§ 10.2(4)** They must not be described as future or merely planned language types.

**§ 10.2(5)** Implementation coverage is tracked outside this rulebook.

### § 10.3 `int` and `uint`

**§ 10.3(1)** `int` and `uint` are target-sized integer types.

**§ 10.3(2)** Their physical width is selected by the target profile.

**§ 10.3(3)** Code that requires an exact ABI width must use a fixed-width type.

**§ 10.3(4)** Examples:

```sec
int32
uint64
int128
```

**§ 10.3(5)** Target selection must not change signedness or ordinary integer semantics.

---

## § 11 Binary floating-point types

**§ 11(1)** Sec provides:

```text
float
float32
float64
```

**§ 11(2)** `float32` and `float64` request explicit binary floating-point widths.

**§ 11(3)** `float` is platform-sized on exactly the same principle as `int` and `uint`: its
width comes from the selected Sec platform's canonical width fact, the same fact
that resolves native integer width.

```text
32-bit platform: int and uint are 32-bit; float has float32 width and semantics
64-bit platform: int and uint are 64-bit; float has float64 width and semantics
```

**§ 11(4)** There is no separately selectable default float width and no independent float
width policy. Host compiler width, backend preference, LLVM defaults, or foreign
ABI convenience never override the selected platform. Every rule that depends on
the representable values of `float`, including range-derived defaults, uses the
platform-selected width. This rule fixes width only; it does not change the
conversion or type-identity rules between `float` and the explicit-width
spellings (MD-014; `rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` § 6).

**§ 11(5)** Code that requires an exact external representation or a fixed width must use an
explicit-width type, `float32` or `float64`.

**§ 11(6)** The numeric family suffix for binary floating point is:

```text
g
```

**§ 11(7)** Examples:

```sec
let value := 1.5g
let whole := 10g
let hexadecimalValue := 0x10g
```

**§ 11(8)** A `g` suffix selects the binary floating-point family.

**§ 11(9)** It does not select a fixed width.

**§ 11(10)** Context may select an explicit width:

```sec
let small: float32 := 1.5g
let wide: float64 := 1.5g
```

---

## § 12 Exact decimal types

**§ 12(1)** Sec provides:

```text
decimal
decimal128
```

**§ 12(2)** Both belong to the exact base-10 decimal family.

**§ 12(3)** `decimal` is the default exact decimal type for an unsuffixed fractional or exponent literal when no stronger context selects another type.

**§ 12(4)** Example:

```sec
let value := 3.14
```

**§ 12(5)** The inferred type is:

```text
decimal
```

**§ 12(6)** Binary floating point must be requested by type context or the `g` family suffix:

```sec
let binary: float64 := 3.14
let binaryLiteral := 3.14g
```

**§ 12(7)** The numeric family suffix for exact decimal is:

```text
m
```

**§ 12(8)** Examples:

```sec
let whole := 10m
let exact := 3.14m
let scientific := 1.25e-3m
let hexadecimalValue := 0x10m
```

**§ 12(9)** The suffix selects the decimal family rather than a fixed width.

**§ 12(10)** Context may select `decimal128`:

```sec
let wide: decimal128 := 8m
```

**§ 12(11)** A decimal literal must be interpreted from its exact source spelling.

**§ 12(12)** It must not first pass through binary floating point.

### § 12.1 Decimal scale

**§ 12.1(1)** The value of a decimal is `coefficient × 10^(-scale)` (MD-066, 2026-10-28). The coefficient is a signed 64-bit integer for `decimal` and a signed 128-bit integer for `decimal128`; neither width is target-sized. The scale is a signed `int32`, and its whole interval `int32.Min ... int32.Max` is valid, including negative scales. The observable scale is the `Scale` instance property (`rules/library/core-library.md` § 10.1).

**§ 12.1(2)** A decimal literal keeps the scale implied by its exact source spelling when that scale is representable; `1.20m` has scale 2. When it is not, the literal takes the closest exact legal scale, or is a compile-time error when none exists. A literal zero keeps its source scale when valid. Exponent validation stays bounded and never materializes an arbitrarily large `10^N`.

**§ 12.1(3)** Arithmetic result scales, decimal division rounding and decimal arithmetic errors are defined in `rules/foundations/operators.md` "Decimal arithmetic". Equality and ordering compare mathematical values, so `1.2m == 1.20m`.

**§ 12.1(4)** `decimal128` is an independent type, not an alias of `decimal`, a binary float or a silent promotion target. Its public core API mirrors `decimal` (`rules/library/core-library.md` § 10.4).

**§ 12.1(5)** Exact fractions (`rational`) and complex numbers are separate future topics and do not change the `decimal`/`decimal128` contract.

**§ 12.1(6)** This rulebook does not define decimal helper functions, MLIR representation, or LLVM representation; the storage layout is owned by `rules/memory/layout.md` § 19.

**§ 12.1(7)** Those are implementation and lowering concerns.

**§ 12.1(8)** `decimal32` and `decimal64` are not part of the canonical Sec type set defined by this rulebook.

---

## § 13 Temporal types

**§ 13(1)** Sec provides four compiler-known temporal value types:

```text
date
time
datetime
duration
```

**§ 13(2)** They require no import.

**§ 13(3)** Their basic meanings are:

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

**§ 13(4)** These are first-class Sec types.

**§ 13(5)** They are not aliases for integers or strings.

**§ 13(6)** They do not implicitly convert to or from numeric types.

**§ 13(7)** A unit-bearing time quantity such as `5<s>` is not a `duration`; it converts
to one only through the exact conversion of `rules/types/temporal.md` § 5
(explicit `duration(q)`, or a proved contextual conversion at a `duration`
declaration or argument; MD-015, 2026-10-13).

**§ 13(8)** Detailed rules for:

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

**§ 13(9)** belong in the temporal rulebook.

**§ 13(10)** The representation and trusted-core boundary are defined by
[`temporal.md`](temporal.md). That rulebook establishes the fixed phase-1 value
representations and the UTC wall-clock properties `datetime.Now`, `date.Today`,
and `time.Now` through a private core-only `_now` intrinsic. It does not make
calendar arithmetic, formatting, parsing, local-time conversion, or monotonic
clock acquisition available.

**§ 13(11)** This rulebook establishes the types and their identity but does not invent
temporal semantics beyond those owned by `temporal.md`.

---

## § 14 Numeric literal family suffixes

**§ 14(1)** Sec uses explicit one-letter lowercase suffixes when the programmer wants a literal to communicate its scalar family directly.

**§ 14(2)** The canonical suffixes are:

```text
i    signed integer family
u    unsigned integer family
g    binary floating-point family
m    exact decimal family
t    char scalar type
r    rune scalar type
```

**§ 14(3)** Suffixes are:

- lowercase;
- case-sensitive;
- adjacent to the literal;
- part of the literal spelling.

**§ 14(4)** Examples:

```sec
8i
8u
8g
8m
65t
65r
```

**§ 14(5)** The family suffix does not normally select a fixed numeric width.

**§ 14(6)** Examples:

```sec
let small: int8 := 8i
let wide: int256 := 8i

let smallFloat: float32 := 8g
let wideFloat: float64 := 8g

let exact: decimal128 := 8m
```

**§ 14(7)** `t` and `r` select the exact scalar types `char` and `rune`. A `t` literal must be in `0..255`; an `r` literal must be a Unicode scalar value (`0..U+10FFFF` excluding `U+D800..U+DFFF`).

---

## § 15 Base-prefixed literals and suffixes

**§ 15(1)** Integer-form literals may use binary, octal, decimal, or hexadecimal spelling.

**§ 15(2)** The scalar family suffix remains unambiguous because the canonical suffix letters are not hexadecimal digits.

**§ 15(3)** Examples:

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

**§ 15(4)** For `g` and `m`, a base-prefixed integer-form literal denotes that integer value represented in the selected scalar family.

**§ 15(5)** Example:

```sec
0x10g
```

**§ 15(6)** denotes the numeric value sixteen in the binary floating-point family.

**§ 15(7)** It is not C-style hexadecimal floating-point syntax.

**§ 15(8)** Fractional and scientific-exponent forms use decimal-form source syntax and accept only:

```text
g
m
```

**§ 15(9)** Examples:

```sec
1.5g
1.5m
1e3g
1e3m
```

**§ 15(10)** A `t` or `r` suffix is valid only on integer-form literals.

---

## § 16 Breaking suffix migration

**§ 16(1)** The canonical suffix set changed during the Sec 0.1 design process.

**§ 16(2)** The old spellings:

```text
c    old char suffix
d    old decimal suffix
f    old binary-float suffix
```

**§ 16(3)** are replaced by:

```text
t    char
m    decimal
g    binary float
```

**§ 16(4)** The canonical migration is:

```text
c -> t
d -> m
f -> g
```

**§ 16(5)** Examples:

```sec
65c        // old
65t        // canonical

3.14d      // old
3.14m      // canonical

3.14f      // old
3.14g      // canonical
```

**§ 16(6)** This is a breaking lexical change.

**§ 16(7)** The compiler implementation, bootstrap lexer, formatter, tests, examples, manuals, diagnostics, and generated knowledge documents must be synchronized.

**§ 16(8)** Where the old spelling can be recognized unambiguously, the compiler should issue a focused migration diagnostic rather than a generic malformed-literal diagnostic.

**§ 16(9)** Examples:

```text
literal suffix 'c' was replaced by 't' for char
literal suffix 'd' was replaced by 'm' for decimal
literal suffix 'f' was replaced by 'g' for binary float
```

**§ 16(10)** No compatibility mode is part of the language rule.

**§ 16(11)** The old spellings are not alternate canonical syntax.

---

## § 17 Unsuffixed literal inference

**§ 17(1)** Literal tokens preserve exact source information until semantic context resolves their type.

### § 17.1 Integer-form literal

**§ 17.1(1)** An unsuffixed integer-form literal defaults to:

```text
int
```

**§ 17.1(2)** when no stronger type context exists.

**§ 17.1(3)** Example:

```sec
let value := 10
```

**§ 17.1(4)** `value` has type `int`.

### § 17.2 Fractional or exponent literal

**§ 17.2(1)** An unsuffixed fractional or scientific-exponent literal defaults to:

```text
decimal
```

**§ 17.2(2)** Example:

```sec
let value := 3.14
let scientific := 1e3
```

**§ 17.2(3)** Both are exact decimal-family values unless context selects another compatible type.

### § 17.3 Character literal

**§ 17.3(1)** A character literal defaults to:

```text
rune
```

**§ 17.3(2)** independently of its scalar's numeric value. An explicitly expected `char` may shape it only when the decoded scalar is in `0..255`.

**§ 17.3(3)** Example:

```sec
let r := 'A'          // rune
let pi := 'π'         // rune
let ch: char := 'A'   // contextual char literal
```

### § 17.4 Context shaping

**§ 17.4(1)** An untyped literal may be shaped directly by declaration or expression context.

**§ 17.4(2)** Examples:

```sec
let small: int8 := 10
let wide: uint256 := 10
let ratio: float64 := 3.14
let exact: decimal128 := 3.14
let percent: Percent := 50
```

**§ 17.4(3)** The compiler validates representability and contracts against the target type.

**§ 17.4(4)** Context shaping of an untyped literal is not implicit conversion of an already typed runtime value.

### § 17.5 Contextual None and empty-literal arguments

**§ 17.5(1)** A bare `None` has no independently inferable `Option[T]` type, and an empty collection literal `[]` needs an expected collection type. Typed declarations, assignments and returns supply that type, and so do call arguments and constructor/initializer arguments (MD-027, 2026-10-12, `rules/corrections/applied/md027-md033-md036-md054-contextual-literals-formatter-configuration-correction-20261012.md`):

```sec
fn AcceptOption(value: Option[int]) void {
}

fn AcceptItems(values: int[]) void {
}

AcceptOption(None)
AcceptItems([])
```

**§ 17.5(2)** When an argument position has one unambiguous expected parameter type, `None` is typed as that `Option[T]` and `[]` as that empty collection type. This is not an implicit conversion between distinct `Option` or collection types.

**§ 17.5(3)** Overload resolution considers all supplied input arguments together: arguments with independently known types eliminate inapplicable overloads before the contextual type of `None` or `[]` is determined, wherever the contextual argument appears. If the remaining viable overloads give a contextual argument incompatible types, the call is ambiguous; the compiler never chooses by declaration order, arbitrary defaulting, the expected result type or an unannounced conversion. The diagnostic names the conflicting candidate parameter types and explains that an explicitly typed argument or binding resolves the ambiguity:

```sec
fn Store(value: Option[int]) void {
}

fn Store(value: Option[string]) void {
}

let number: Option[int] := None
Store(number)  // Valid: Option[int] determines the overload.
Store(None)    // Error: ambiguous overload.
```

**§ 17.5(4)** The same rules apply to ordinary calls, methods, `init(...)` and `new` construction; no new syntax or dynamic overload dispatch is introduced.

### § 17.6 Contextual and self-inferred `Some(value)`

**§ 17.6(1)** `Some(value)` is the canonical one-payload Option constructor when it resolves
to the compiler-known `Option[T].Some`, not to a visible ordinary callable named
`Some` (MD-102, 2026-10-28,
`rules/corrections/applied/md029-md042-md102-pattern-reachability-some-correction-20261028.md`
§§ 11–14). It is valid wherever ordinary type resolution establishes its
`Option[T]` type, and a valid use never produces `S1157`.

**§ 17.6(2)** An expected `Option[T]` — a typed declaration, assignment, return, call or
constructor argument — wins as the context in which the payload is validated
and shaped:

```sec
fn AcceptOption(value: Option[int]) void {
}

AcceptOption(Some(42))
let number: Option[int64] := Some(42)
```

**§ 17.6(3)** Unlike `None`, the payload carries type information. In overload resolution it
eliminates candidates exactly like an ordinary argument: a candidate expecting
`Option[U]` is viable only if the payload can legally initialize `U` under the
existing contextual-literal, compatibility and explicit-conversion rules. No
candidate is chosen merely because its parameter is an `Option`, and there is
no `Some`-specific precedence; ordinary literal shaping and ranking decide, or
the call is ambiguous with both candidate types named:

```sec
fn Store(value: Option[int]) void {
}

fn Store(value: Option[string]) void {
}

Store(Some(42)) // Selects the Option[int] overload.
Store(None)     // Ambiguous unless other arguments/context resolve it.
```

**§ 17.6(4)** Without an expected type, `Some(value)` whose payload has an independently
inferable type `T` has the nominal type `Option[T]`. This is a positive
inference rule, not an implicit optional conversion of unrelated expressions,
and it does not bypass literal representability, nominal identity, contracts
or special type restrictions:

```sec
let number := Some(42)     // Option[int].
let name := Some("Alice")  // Option[string].
let nested := Some(Some(1)) // Option[Option[int]].
```

**§ 17.6(5)** A standalone `None` still has no type, and an uncontextualized `Some(None)`
lacks the inner `Option[T]` type; both are diagnosed unless an expected type
supplies every unknown nested constructor:

```sec
let invalid := None                         // Error: Option type cannot be inferred.
let nested: Option[Option[int]] := Some(None) // Valid.
```

**§ 17.6(6)** Nested `Some(Some(value))` typing does not introduce nested match patterns.
Typing examines payload types and static constraints only; the payload is
evaluated exactly once in the ordinary evaluation order, and a failed overload
choice evaluates nothing. Ownership, borrowing and lifetime rules stay binding:
no implicit move from a reusable non-copyable Place, invented copy, concealed
reborrow, lifetime extension or escape of a short-lived reference.

**§ 17.6(7)** `None` is spelled without parentheses. `None()` is invalid Option
construction and keeps the `S1157` guidance that `None` takes no arguments and
is not a function. A contextual `None`, `Some(...)`, `[]` or `null` passed to a
parameter outside its accepted type family is a type error.

---

## § 18 Variable declarations

**§ 18(1)** Variables are immutable by default.

**§ 18(2)** The normal inferred forms are:

```sec
let a := 9
let mut a := 9
```

**§ 18(3)** An explicit type may be supplied:

```sec
let a: int := 9
let mut a: int := 9
```

**§ 18(4)** A mutable declaration may omit the initializer only when the declared type is defaultable:

```sec
let mut a: int
```

**§ 18(5)** An immutable declaration requires an initializer:

```sec
let a: int
```

**§ 18(6)** is invalid.

**§ 18(7)** The compiler does not create an undefined immutable value.

---

## § 19 Multiple `let` declarators

**§ 19(1)** A `let` declaration may contain multiple declarators.

**§ 19(2)** Examples:

```sec
let a := 9, b := "hello", c := true
let mut a := 9, b := 10, c := 11
```

**§ 19(3)** Each declarator is checked independently.

**§ 19(4)** With inferred declarations, each declarator receives the type of its own initializer.

**§ 19(5)** Example:

```sec
let a := 9, b := "hello", c := true
```

**§ 19(6)** declares:

```text
a : int
b : string
c : bool
```

---

## § 20 Type-first declarations

**§ 20(1)** Sec also supports type-first declarations.

**§ 20(2)** A single declaration is valid:

```sec
int mut: a
```

**§ 20(3)** This is as valid as:

```sec
let mut a: int
```

**§ 20(4)** The type-first form is not restricted to multiple declarations.

**§ 20(5)** Multiple declarations are also valid:

```sec
int mut: a, b, c
```

**§ 20(6)** An immutable type-first declaration requires an initializer:

```sec
int: a := 1
```

**§ 20(7)** Multiple immutable values may be declared:

```sec
int: a := 1, b := 2, c := 3
```

**§ 20(8)** A mutable type-first declarator may omit its initializer only when the type is defaultable:

```sec
int mut: a
```

**§ 20(9)** For a non-defaultable type, an initializer remains required.

**§ 20(10)** The explicit type applies to every declarator in the statement.

---

## § 21 Parenthesized immutable type-first groups

**§ 21(1)** Sec supports a parenthesized immutable type-first declaration group.

**§ 21(2)** Canonical syntax:

```sec
int (
    a := 1,
    b := 2,
    c := 3,
)
```

**§ 21(3)** This form declares immutable typed values.

**§ 21(4)** It does not construct an aggregate value.

**§ 21(5)** The parentheses intentionally distinguish a declaration group from value construction and block syntax.

**§ 21(6)** Rules:

- the type appears once before the group;
- the group uses `(` and `)`;
- every entry requires an initializer;
- every entry has the declared type;
- a trailing comma is allowed;
- the group is immutable;
- this form does not use `{` and `}`.

**§ 21(7)** For example:

```sec
TokenType (
    ILLEGAL := "ILLEGAL",
    EOF := "EOF",
    IDENT := "IDENT",
)
```

**§ 21(8)** is a declaration group, not construction of a `TokenType` value.

---

## § 22 Type identity

**§ 22(1)** Type identity is stronger than representation identity.

**§ 22(2)** Two values do not have the same type merely because their storage representation is equal.

### § 22.1 Named types

**§ 22.1(1)** A user-defined named type is distinct from its underlying type.

**§ 22.1(2)** Example:

```sec
type Percent int range 0..100
type Age int range 0..130
```

**§ 22.1(3)** The following types are distinct:

```text
Percent
Age
int
```

**§ 22.1(4)** Invalid:

```sec
let age: Age := 20
let percent: Percent := age
```

**§ 22.1(5)** No implicit conversion is performed merely because both types use integer semantics.

**§ 22.1(6)** A named type is declared as `type Name TypeReference` with exactly one
underlying type. Chained named types remain distinct: after `type B int` and
`type A B`, the types `int`, `B`, and `A` are all distinct. Forms naming several
underlying types, such as `type A int string`, and the legacy `=` forms
`type Name = ExistingType` and `type Name = First Second` are invalid Sec 0.1
syntax (`rules/foundations/grammar.md`; `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 3).

**§ 22.1(7)** Named types may be generic:

```sec
type ID[T] int
type Wrapped[T] T
```

**§ 22.1(8)** Concrete instantiations preserve nominal identity. `ID[User]` and
`ID[Product]` are distinct types even when their substituted representation is
identical. After substitution, ordinary named-type conversion, contracts,
defaultability, copy/move behavior, layout, interface conformance, and ABI rules
apply to the concrete type.

### § 22.2 Generic and parameterized types

**§ 22.2(1)** Type arguments participate in type identity.

**§ 22.2(2)** Examples:

```text
list[int]
list[string]

Pair[int, string]
Pair[string, int]

Shape[2]
Shape[3]
```

**§ 22.2(3)** These are distinct types or distinct concrete instantiations as defined by their owning type families.

### § 22.3 Arrays

**§ 22.3(1)** Array length participates in type identity:

```text
int[4]
int[5]
```

**§ 22.3(2)** are distinct.

### § 22.4 References

**§ 22.4(1)** Reference mode participates in type identity:

```text
ref T
ref mut T
```

**§ 22.4(2)** are distinct reference types.

### § 22.5 Function types

**§ 22.5(1)** Callable capability, ordered parameter types and ownership modes, variadic
shape, and return type participate in function-type identity.

**§ 22.5(2)** Example:

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

**§ 22.5(3)** are distinct.

---

## § 23 Assignability

**§ 23(1)** Assignment requires semantic compatibility between source and target.

**§ 23(2)** The compiler must not use equal physical representation as evidence of assignability.

**§ 23(3)** A value of an exact named type may be assigned to another location of that exact type subject to mutability, ownership, and borrowing rules.

**§ 23(4)** Example:

```sec
let a: Percent := 20
let mut b: Percent := 10

b = a
```

**§ 23(5)** This assignment does not recreate the `Percent` contract obligation.

**§ 23(6)** `a` is already a valid `Percent`.

**§ 23(7)** The assignment remains subject to normal copy/move rules.

---

## § 24 Untyped literals versus typed values

**§ 24(1)** Untyped literals may be shaped by context.

**§ 24(2)** Typed runtime values are not implicitly changed into unrelated named or numeric types.

**§ 24(3)** Valid:

```sec
let p: Percent := 90
```

**§ 24(4)** Invalid:

```sec
let value: int := 90
let p: Percent := value
```

**§ 24(5)** Use explicit conversion:

```sec
let p := try Percent(value)
```

**§ 24(6)** when the conversion may fail.

**§ 24(7)** This distinction is fundamental:

```text
literal shaping
    resolves an untyped literal in target context

conversion
    changes an already typed value into another type
```

**§ 24(8)** They are not the same operation.

---

## § 25 Explicit conversions

**§ 25(1)** The general conversion spelling is:

```sec
TargetType(value)
```

**§ 25(2)** Examples:

```sec
int32(value)
decimal(value)
bool(value)
char(value)
rune(value)
Percent(value)
```

**§ 25(3)** Conversions are checked.

**§ 25(4)** They must not silently truncate, wrap, discard precision, violate a contract, or create an invalid scalar value unless a separately named lossy operation explicitly defines that behavior.

**§ 25(5)** A conversion that may fail requires `try`. A missing conversion relation is
a compile-time type error; `try` does not create a relation. In particular,
explicit `string -> Name` conversion for `type Name string` does not imply a
direct `numeric -> Name` conversion (MD-093).

**§ 25(6)** Examples:

```sec
let small := try int32(largeValue)
let r := try rune(codePoint)
let p := try Percent(raw)
```

**§ 25(7)** When the compiler proves a conversion valid from compile-time information, no runtime validation is emitted.

**§ 25(8)** The language semantics remain checked.

**§ 25(9)** Conversion failure has three distinct semantic layers that must never collapse
into one undifferentiated conversion error:

```text
no conversion relation
    compile-time type diagnostic; no runtime Result error type represents it

intrinsic target-domain failure
    the value cannot form a valid value of the target primitive/scalar type:
    overflow or non-representability, forbidden precision loss, an invalid
    Unicode scalar value, or another validity failure the target type defines

declared contract failure
    the value is valid for the underlying target domain but violates a
    contract of the constrained named type
```

**§ 25(10)** The intrinsic range of a primitive such as `uint8` is a property of the type,
not an implicit `range` contract. A conversion into a constrained named type
converts into the underlying target domain first, stops on intrinsic failure,
and only then evaluates the declared contracts in canonical source order
(`rules/types/contracts.md`, "Conversion failure layers"). A compile-time-known
invalid value is diagnosed at compile time.

**§ 25(11)** A runtime-dependent checked conversion that can fail exposes the core error
type `ConversionError` (declared in `sec/core/error.sec`). Intrinsic failures use
its `OutOfRange`, `InvalidScalar`, `PrecisionLoss`, `ScaleOverflow`, or
`NonFinite` variants; a declared contract failure uses
`ConversionError.Contract(ContractError.Violation { ... })` as defined in
`rules/types/contracts.md`, "Public conversion and contract errors". Defining
this channel never demotes a proven compile-time error into a runtime result
(MD-012; `rules/corrections/applied/md012-md022-conversion-errors-ffi-literals-correction-20261008.md` § 4).

**§ 25(12)** Lossy, wrapping, saturating, or otherwise intentionally non-preserving conversion semantics require separately defined explicit operations.

### § 25.1 Exact integer conversion proof

**§ 25.1(1)** Decided 2026-10-18 (MD-104). A built-in integer-to-integer conversion is
*exact* — lowered as Semantic IR `numeric.convert-exact` /
`sec.numeric.convert_exact` with no runtime error edge
(`rules/compiler/semantic_ir.md` § 124) — only when Sema has proved and
retained that every value the source may hold is representable in the target
under the **selected CompilationPlan**:

- a compile-time-known constant is proved by its exact value; an unrelated
  literal elsewhere proves nothing about a runtime value;
- a non-enum source is proved by its complete type domain;
- an ordinary **closed** integer-backed enum is proved by its set of declared
  value classes — not the whole carrier range; aliases with duplicate values
  do not enlarge it (`rules/declarations/enums.md` § 9, § 12.1);
- an **open** `bit[N]` enum has the domain `0 ..= 2^N - 1`; its named cases
  never bound it, so `bit[8]` converts exactly to `uint8`, `uint16` or
  `int16` but not to `int8`;
- an external C or other ABI-defined open enum uses its complete ABI raw
  domain for the active target; when that domain is unresolved no exactness
  proof is manufactured from declared cases. (Sec 0.1 has no such foreign
  enum surface yet.)

**§ 25.1(2)** The proof is made for the active plan (Option A), not for an intersection of
all targets: `int` and `uint` take that plan's resolved width, never the
host's, an assumed `int64` or an unresolved layout. So `int(value)` with
`value: uint32` is exact where `int` is 64-bit signed and is not exact where
it is 32-bit; there the conversion is a checked conversion and failure must be
handled. The same source may therefore compile on one target and need a
checked conversion on another; this is target-dependent portability, not
relaxed safety. Without the needed plan facts the compiler proves only what
holds at every supported width and otherwise fails closed. Proofs record the
plan they were made for; a proof made for another plan — including one
from a cache or incremental reuse — is never reused but recomputed, and
Semantic IR rejects a proof that does not match the module's plan.

**§ 25.1(3)** Direct enum-to-carrier conversion is an identity only when the declared
carrier and target agree; any other width or signedness needs the proof
above. The closed-enum proof relies on the closed-domain invariant: a raw
integer is never accepted as an arbitrary closed-enum value, and raw foreign
values that could violate it are validated or represented by an open/raw
type.

**§ 25.1(4)** A conversion that is not proven exact is a checked conversion whose public
failure family is `ConversionError` (`OutOfRange` for an integer domain,
MD-012); P20's provisional `NumericConversionError` names no public type.
Where the track cannot lower checked conversions yet it rejects them with an
unsupported-lowering diagnostic; it never truncates silently. MD-056's
remaining frontend `try` consistency question is not decided here.

### § 25.2 Numeric-to-text conversion (MD-051)

**§ 25.2(1)** `.ToString()` is the preferred spelling of numeric-to-text materialization. An
explicit `string(numericValue)` conversion is also permitted for every numeric
value (`int`/`uint` families including `byte`, `float`, `decimal`, and named
numeric types whose conversion contracts allow the route). It is exactly the
no-argument `ToString()` of that numeric type under another spelling: the same
default culture-independent formatting and the same fallible result
`Result[string, StringError]`, with the same allocation and error obligations.

```sec
let number := 123

let preferred := try number.ToString()
let explicit := try string(number)
```

**§ 25.2(2)** Both yield the same text. `string(numericValue)` is not a code-point
conversion, has no format argument (format-specific presentation stays
`.ToString(format)`) and is never applied implicitly: `let text: string :=
number` remains invalid. The distinct `char` and `rune` conversions are
unchanged (`rules/corrections/applied/md051-md074-md088-length-numeric-string-correction-20261017.md` § 7).
`byte` is the `uint8` family member here (owner decision 2026-10-17): `string(b)`
equals `b.ToString()` and yields decimal text, never a character.

**§ 25.2(3)** The primitive formatting conversion is not inherited by named string types
(MD-093, 2026-10-26). Given `type Name string`, both `Name(number)` and
`try Name(number)` are invalid for numeric primitives and named numeric values.
No implicit formatting step or `Result[Name, ...]` interpretation is permitted.

**§ 25.2(4)** Materialize text explicitly, preferably with `try Name(number.ToString())`.
The outer `try` protects the entire expression under `errorhandling.md`
§§ 11–14: materialization may fail with `StringError`, and subsequent target
contract validation may independently fail with `ConversionError.Contract`.
The first failure in evaluation order stops evaluation. Every unhandled
failure must fit the enclosing declared error channel; no error union or
mapping is inferred. The existing common `error` channel can accept both.

**§ 25.2(5)** `Name(try number.ToString())` is also valid when conversion of the resulting
string to `Name` is infallible. An inner-only `try` does not protect a separately
fallible outer named-type conversion. Existing strings still use `Name(text)`
with ordinary nominal identity and contract validation. No new `try` grammar,
conversion API or error family is introduced.

---

## § 26 Named types and contracts

**§ 26(1)** A named type retains nominal identity.

**§ 26(2)** For a named string type, explicit conversion from an already materialized
`string` preserves that nominal identity and applies its contracts. This is
distinct from numeric text formatting: `Name(text)` may be a valid conversion,
while `Name(number)` has no relation, even under `try` (MD-093).

**§ 26(3)** Example:

```sec
type Percent int range 0..100
```

**§ 26(4)** The contract belongs to the type.

**§ 26(5)** It does not belong to one variable.

**§ 26(6)** A compile-time literal may be shaped directly to the named type when it satisfies every contract:

```sec
let p: Percent := 50
```

**§ 26(7)** This is invalid:

```sec
let p: Percent := 101
```

**§ 26(8)** Runtime construction may be fallible:

```sec
let p := try Percent(raw)
```

**§ 26(9)** The detailed contract vocabulary is defined by `contracts.md` and `runtime_checks.md`.

---

## § 27 Assignment of constrained named types

**§ 27(1)** An already valid value of a constrained named type may be assigned to the same type without revalidating the contract merely because storage changes.

**§ 27(2)** Example:

```sec
let source: Percent := 91
let mut target: Percent := 10

target = source
```

**§ 27(3)** The value `source` is already a valid `Percent`.

**§ 27(4)** The compiler must not insert redundant contract failure semantics for the plain same-type assignment.

**§ 27(5)** This is different from an operation that computes a new constrained value.

---

## § 28 Value-producing mutation of constrained named types

**§ 28(1)** A compound assignment computes a new value.

**§ 28(2)** For a constrained named type, the new value may violate the contract even when every input value is individually valid.

**§ 28(3)** Example:

```sec
let mut percentage: Percent := 91
let additional: Percent := 10
```

**§ 28(4)** The mathematical result of:

```sec
percentage += additional
```

**§ 28(5)** would be:

```text
101
```

**§ 28(6)** which is not a valid `Percent`.

**§ 28(7)** Therefore compound assignment to a constrained named type is a fallible value-producing mutation and uses the canonical `try` assignment form:

```sec
try percentage += additional {
    Err(error) => {
        Handle(error)
    }
}
```

**§ 28(8)** The same principle applies to other operations that produce a new value for constrained storage.

**§ 28(9)** The compiler may eliminate a runtime check when proof establishes that failure is impossible, but it must preserve the language's fallibility and contract semantics.

---

## § 29 Defaultability

**§ 29(1)** Every type is classified as either:

```text
Defaultable
NonDefaultable
```

**§ 29(2)** A defaultable type has one compiler-resolved valid default value.

**§ 29(3)** A non-defaultable type cannot be used without an initializer in a context that requires default initialization.

**§ 29(4)** Example:

```sec
int mut: value
```

**§ 29(5)** is valid because `int` is defaultable.

**§ 29(6)** For named and aggregate types, defaultability is resolved according to `default_values.md`.

**§ 29(7)** The compiler must never treat undefined storage as a default value.

**§ 29(8)** This rulebook does not duplicate the complete default-selection algorithm.

---

## § 30 Arrays, owning sequences, and slices

**§ 30(1)** Sec uses postfix sequence syntax.

**§ 30(2)** Fixed array:

```sec
T[N]
```

**§ 30(3)** Owning unsized sequence:

```sec
T[]
```

**§ 30(4)** Borrowed slice:

```sec
ref T[]
ref mut T[]
```

**§ 30(5)** Examples:

```sec
int[4]
byte[256]

Packet[]

ref byte[]
ref mut int[]
```

**§ 30(6)** Array length is part of type identity.

**§ 30(7)** Slices are non-owning references to sequence storage.

**§ 30(8)** The complete rules for layout, indexing, slicing, ownership, defaultability,
and bounds are defined by `collections.md`.

---

## § 31 Safe references

**§ 31(1)** Safe reference forms are:

```sec
ref T
ref mut T
```

**§ 31(2)** A reference does not own the referenced value.

**§ 31(3)** `ref T` provides shared access.

**§ 31(4)** `ref mut T` provides exclusive mutable access.

**§ 31(5)** Sec does not expose lifetime parameters in ordinary source syntax.

**§ 31(6)** Reference validity, provenance, generations, storage epochs, borrowing, and escape behavior are defined by the reference and lifetime rulebooks.

**§ 31(7)** The physical representation of a safe reference is not fixed by this rulebook.

---

## § 32 Raw pointers

**§ 32(1)** Unchecked foreign or low-level addresses use:

```sec
RawPtr[T]
```

**§ 32(2)** Example:

```sec
let address: RawPtr[byte]
let opaque: RawPtr[void]
```

**§ 32(3)** `RawPtr[T]` is not a safe reference.

**§ 32(4)** It does not imply ownership.

**§ 32(5)** It does not extend lifetime.

**§ 32(6)** Unsafe conversion or dereference rules are defined by `raw_pointers` and FFI rulebooks.

---

## § 33 Function types

**§ 33(1)** Function values have a statically known function type.

**§ 33(2)** Syntax:

```sec
fn(ParameterType, ParameterType) ReturnType
```

**§ 33(3)** Examples:

```sec
fn(int) bool
fn(int, int) int
fn(string) void
```

**§ 33(4)** Parameter names are not part of function-type identity.

**§ 33(5)** Callable capability and parameter consumption are distinct. For example,
`-> fn(-> Resource) Handle` consumes both the callable environment and its
argument. Parameter names and individual capture contents are not part of
source-level callable type identity, although captures determine capability,
copy/move classification, and lifetime.

**§ 33(6)** Lambda, closure, capture, call, and ownership rules are defined by the function and closure rulebooks.

---

## § 34 First-class collections

**§ 34(1)** The compiler-known first-class collection constructors are:

```sec
list[T]
list[T, Capacity]

map[K, V]
map[K, V, Capacity]

set[T]
set[T, Capacity]
```

**§ 34(2)** They require no import.

**§ 34(3)** They are not aliases for standard-library generic types.

**§ 34(4)** Their semantics benefit from compiler knowledge, including capacity, allocation, ownership, indexing/lookup, analysis, and lowering.

**§ 34(5)** More specialized data structures remain nominal library types.

**§ 34(6)** Examples:

```sec
LinkedList[T]
OrderedMap[K, V]
OrderedSet[T]
```

**§ 34(7)** Detailed collection semantics are defined by `collections.md`.

---

## § 35 First-class shaped types

**§ 35(1)** The compiler-known shaped type constructors are:

```sec
vector[T, N]
matrix[T, Rows, Columns]
tensor[T, Dimensions...]
tensor[T, Shape[Rank]]
tensor_view[T, Rank]
```

**§ 35(2)** They require no import.

**§ 35(3)** Supporting nominal types include:

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

**§ 35(4)** The lowercase shaped constructors are fundamental compiler-known type forms.

**§ 35(5)** The uppercase supporting types remain nominal core types.

**§ 35(6)** `tensor[T, Dimensions...]` is an owning tensor whose rank and extents are
compile-time-known. `tensor[T, Shape[Rank]]` is an owning runtime-shaped tensor:
its rank is compile-time-known while its extents are runtime values. It must not
be reinterpreted as `tensor_view[T, Rank]`, which remains a compiler-known
non-owning view type form. Safe source-level views use `ref tensor_view[T, Rank]`
or `ref mut tensor_view[T, Rank]`.

**§ 35(7)** The supporting shaped types are nominal semantic types, not aliases for
ordinary arrays or lists.

**§ 35(8)** Physical layout is not inferred from the source spelling unless the shaped-type rulebook explicitly makes layout observable.

**§ 35(9)** Detailed shape, stride, layout, memory-space, view, and matrix-operation rules are defined by `shaped-types.md`.

---

## § 36 Hardware layout types

**§ 36(1)** Sec has compiler-known hardware-oriented forms:

```sec
bit
bit[N]
register[N]
```

**§ 36(2)** These are language forms rather than standard-library generics.

**§ 36(3)** They participate in compile-time layout semantics.

**§ 36(4)** Example:

```sec
type Control register[8] {
    Enabled: bit
    Mode: bit[3]
    _: bit[4]
}
```

**§ 36(5)** Detailed register and bit-field semantics are defined by the register rulebook.

---

## § 37 Units

**§ 37(1)** Units are semantic quantity identities. A unit is not itself a numeric
representation; a unit-bearing numeric type combines a numeric carrier with a
unit semantic descriptor.

**§ 37(2)** Examples:

```sec
unit Meter physical
unit Second physical
unit Item uint other

let exact: decimal<Meter> := 10
let measured: float64<Meter> := 10
let count: <Item> := 4
```

**§ 37(3)** The optional declaration carrier is only the default used by `<NamedUnit>`; it
is not part of unit identity. A compound unit-only expression such as `<m/s>`
defaults to `decimal`.

**§ 37(4)** The complete unit declaration, structural algebra, Kind, transform, conversion,
and system rules are defined by `units.md`.

**§ 37(5)** Unit metadata reads `<U>.Member` have the typed results defined in
`units.md`, "Unit metadata value types" (MD-062, 2026-10-27). They introduce
the core types `UnitKind`, `UnitSystem`, `UnitOrigin`, `UnitTransform`,
`UnitCategory` and `UnitStatus` (ordinary closed enums) and `UnitDimension`
(an opaque, immutable, copyable, non-defaultable compiler-known value type
with only `AxisCount`, `IsDimensionless`, `==` and `!=`), declared in
`rules/library/core-library.md` § 27. None of them makes types first-class
values or adds reflection.

**§ 37(6)** Named unit types remain distinct according to their unit semantics.

---

## § 38 Core generic result and option types

**§ 38(1)** `Result[T, E]` and `Option[T]` are nominal core generic types.

**§ 38(2)** They are not special parser spellings for ordinary type references.

**§ 38(3)** `Result[T, E]` participates in explicit typed error handling. `E` must be the
compiler-known `error` root, a concrete Sec type declared as an error type, or
a generic parameter constrained by `error` (`rules/declarations/generics.md`
§ 15A, MD-120). `Result[T, string]` is invalid.

**§ 38(4)** For a concrete error type, implicit widening `ConcreteError -> error` is valid
and preserves concrete type identity, active variant, payload, ownership, and
destruction obligations. Implicit `error -> ConcreteError` narrowing and
cross-concrete error conversion remain invalid. This relation is error-specific
and does not introduce general inheritance.

**§ 38(5)** `Option[T]` represents explicit optionality.

**§ 38(6)** Safe references are non-null.

**§ 38(7)** Optional reference semantics therefore use an explicit optional type rather than nullable `ref`.

**§ 38(8)** Detailed behavior is defined by the error-handling, union, and reference rulebooks.

---

## § 39 Type inference

**§ 39(1)** Type inference resolves types from explicit language information.

**§ 39(2)** It must not guess between semantically distinct types.

**§ 39(3)** Inference may use:

- literal kind;
- target type;
- declaration context;
- function parameters;
- generic constraints and arguments;
- expected result type where defined;
- operator semantics;
- pattern context.

**§ 39(4)** Inference must preserve named type identity.

**§ 39(5)** The compiler must reject ambiguous inference rather than silently choose an unrelated type.

---

## § 40 Explicit low-bit extraction

**§ 40(1)** The compiler-known expression

```sec
bit[N](value)
```

**§ 40(2)** extracts the `N` least-significant bits of an integer value (MD-056, 2026-10-12 and 2026-10-13, `rules/corrections/applied/md040-md046-md047-md056-units-register-bits-rounding-correction-20261013.md` § 3, which supersedes `rules/corrections/applied/md046-md056-bit-extraction-correction-20261012.md`). It is a deliberate, visibly lossy bit-pattern extraction, distinct from the value-preserving checked conversion `TargetType(value)`; it is not a new keyword, a general cast, a core function, a standalone `bit[N]` type or a `uintN` family. MD-077 (2026-10-22, `rules/corrections/applied/md077-register-semantic-ir-final-decision-20261022.md` § 6): a register *field* declared `bit[N]` (`N >= 2`) reads as the same unsigned carrier with a Sema-proven interval `0..2^N-1`; a `bit` field reads as `bool`, whereas `bit[1](value)` still yields the carrier. These are different constructs.

**§ 40(3)** Semantics: with bit 0 as the least-significant bit of the source's fixed-width two's-complement representation, the result is

```text
bit[N](x) = x mod 2^N
```

**§ 40(4)** where `mod` is the unique non-negative remainder in `0..2^N-1`, also for negative `x`. Extraction works on the numeric bit pattern, not on bytes in memory, and is independent of endianness, register field allocation order, access width and C ABI. It is pure, infallible and allocation-free and needs neither `try` nor `unsafe`.

**§ 40(5)** `N` is a compile-time integer constant in `1..256`. It must not exceed the source's resolved representation width; a narrower source is never zero- or sign-extended. The width of `int` and `uint` comes from the active CompilationPlan and is diagnosed, never guessed, when unavailable.

**§ 40(6)** The result type is always unsigned and chosen from `N` alone:

| `N` | Result type |
|---|---|
| 1–8 | `uint8` |
| 9–16 | `uint16` |
| 17–32 | `uint32` |
| 33–64 | `uint64` |
| 65–128 | `uint128` |
| 129–256 | `uint256` |

**§ 40(7)** The compiler records the proven interval `0 <= result <= 2^N - 1`, which a later checked conversion may use to prove that it cannot fail. `bit[N](value)` returns `uint8`, never `byte` or `char`, for `N` in `1..8`.

**§ 40(8)** Accepted sources are the primitive signed and unsigned integer types, including target-sized `int` and `uint` with a resolved width, the eight-bit `byte` and `char` (read as their numeric representation, not as text), and — the MD-046 exception — unit-bearing integer quantities such as `uint16<rpm>`. Named types, constrained types, enums, registers, pointers, foreign `c::` scalars, strings, runes, booleans and floating values are rejected; they need their own explicit conversion first, and extraction never strips a contract.

**§ 40(9)** The result never inherits a unit: `bit[N](speed)` is a unitless unsigned value even when `speed` has a unit. The optional suffix `bit[N](value)<U>` (written with no space before `<`) gives the result the unit `U`: it labels a unitless source, which is the programmer's assertion at a hardware or protocol boundary, or keeps the source's own unit. It never changes a unit — `bit[12](speed)<Hz>` for a `speed` in rpm is an error; use the declared conversion `<Hz>(speed)`. The numeric bits are the same in both forms; no scale, offset, logarithm or rounding applies (MD-046, 2026-10-13, `rules/corrections/applied/md040-md046-md047-md056-units-register-bits-rounding-correction-20261013.md` § 4).

```sec
let word: uint32 := 0xABCD1234
let low12 := bit[12](word)      // uint16, 0x234

let negative: int64 := -1
let low8 := bit[8](negative)    // uint8, 255

let wide: uint16 := 0xABCD
let truncated := bit[8](wide)   // uint8, 0xCD: explicit truncation
let checked := try uint8(wide)  // fails: 0xABCD does not fit uint8
```

**§ 40(10)** Register conversions (MD-110, 2026-10-25): `bit[N](x)` remains the only deliberate low-bit extraction. The exact detached register construction `T(x)` and the register-to-integer interpretation `IntegerType(register)` are specified in `rules/declarations/registers.md` § 12.3. Their fit rule is bit-pattern representability, so a signed `-1` is a valid all-ones pattern, and a `bit[N](x)` result converts to `register[N]` exactly, without a check.

---

## § 41 No implicit conversion

**§ 41(1)** Sec does not use broad implicit conversion between already typed values.

**§ 41(2)** Examples:

```sec
let age: Age := 20
let percent: Percent := age
```

**§ 41(3)** is invalid.

**§ 41(4)** Likewise, binary float, decimal, integer, char, rune, and named numeric values do not silently change type merely because a machine representation could support the conversion.

**§ 41(5)** Use:

- literal context;
- explicit conversion;
- explicit unit conversion;
- a named lossy operation;
- another operation defined by the owning rulebook.

---

## § 42 Type semantics and ownership

**§ 42(1)** Type compatibility does not decide ownership transfer by itself.

**§ 42(2)** The compiler separately determines whether an operation:

- copies;
- moves;
- borrows;
- mutates;
- destroys;
- reuses an SSA value.

**§ 42(3)** Copyability and move-only classification are semantic properties of the resolved type.

**§ 42(4)** Detailed rules are defined by ownership, copy/move, borrowing, lifetime, and destruction rulebooks.

**§ 42(5)** Backend loads and stores must not be used to infer source-level copy or move semantics.

---

## § 43 Must-use and discardability

**§ 43(1)** `must-use` and `discardability` are distinct semantic properties of a resolved
type or concrete type instance.

```text
must-use
    an unnamed produced value may not disappear through implicit discard

discardable
    explicit terminal discard is legal
```

**§ 43(2)** A type may therefore be must-use while remaining explicitly discardable.

**§ 43(3)** Canonical examples include:

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

**§ 43(4)** Discardability is recursive through concrete aggregate and variant payload
structure. It is not derived merely from copy classification or physical
representation.

**§ 43(5)** Compiler-known core types may carry these properties before a general
user-defined must-use declaration mechanism exists. The source declaration
mechanism for user-defined must-use types remains owned by the attribute and
type-design rules.

**§ 43(6)** Detailed terminal consumption and implicit call-result behavior is defined by
`rules/control-flow/discard.md`.

---

## § 44 Type semantics and representation

**§ 44(1)** A source type may have different physical representations under different targets or profiles when the owning rule permits this.

**§ 44(2)** Examples include:

- `int` and `uint` width;
- safe-reference metadata;
- collection storage;
- interface values;
- closure environments;
- shaped-value materialization.

**§ 44(3)** Representation changes must not change source semantics.

**§ 44(4)** Types that cross an ABI boundary require separately defined ABI-compatible representation.

**§ 44(5)** The type system must not expose LLVM or MLIR implementation details as source-language truth.

---

## § 45 Diagnostics

**§ 45(1)** Type diagnostics should identify:

- the expected type;
- the actual type;
- the source expression;
- the relevant named type or contract;
- the failed conversion or assignability rule;
- generic arguments where relevant;
- array or shaped dimensions where relevant.

**§ 45(2)** For a breaking literal suffix migration, diagnostics should identify the old and new spelling directly.

**§ 45(3)** Examples:

```text
literal suffix 'c' was replaced by 't' for char
literal suffix 'd' was replaced by 'm' for decimal
literal suffix 'f' was replaced by 'g' for binary float
```

**§ 45(4)** A type diagnostic should explain semantic incompatibility rather than report only backend representation differences.

---

## § 46 Related rulebooks

**§ 46(1)** Detailed rules are owned by the corresponding canonical rulebooks.

**§ 46(2)** Important adjacent areas include:

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
borrowing.md
lifetime_analysis.md
raw_pointers.md
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

**§ 46(3)** When an older filename still uses `.txt`, the next rewritten canonical rulebook must use `.md`.

---

## § 47 Design summary

**§ 47(1)** The Sec type model follows these principles:

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
