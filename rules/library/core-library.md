# Sec Core Library

Compiler-known Sec declarations backed by `core` remain real source-visible
declarations and must be specified in complete canonical syntax. Their
compiler-known identity and core declaration resolve to one semantic symbol.
Compiler-internal structures are not Sec declarations and do not use `_` or
`__` as a substitute for an explicit authority boundary. Real core helpers
with those names retain ordinary Sec visibility semantics. The canonical
registry and replacement policy are owned by
`rules/compiler/compiler_known_members.md`.

## Purpose

The Sec core library defines the minimum language-level functionality that is
available without importing the standard library.

The core library is not a complete utility library.

It provides:

- fundamental behavior for built-in types,
- compiler-known intrinsic members,
- ordinary core methods implemented in Sec,
- standard error types required by language operations,
- the bridge between built-in types and normal AST/Sema member resolution.

The standard library may add higher-level collection types and algorithms,
formatting facilities, text processing, operating-system integration, and
platform APIs.

First-class language collections and shaped types are implemented by the
compiler and core library.

The core library must remain:

- small,
- deterministic,
- target-independent in semantics,
- usable without a runtime,
- usable on hosted, freestanding and embedded targets,
- free from hidden allocation unless explicitly stated,
- independent of garbage collection,
- available before ordinary imports are resolved.

This document defines a minimum required surface. It is not intended to be the
complete final list of core members.

---

# 1. Core and compiler responsibilities

The compiler and the core library have separate responsibilities.

## 1.1 Compiler-owned functionality

The compiler owns operations that cannot be implemented correctly as ordinary
Sec code because they require knowledge of:

- value representation,
- addressability,
- storage class,
- target layout,
- alignment,
- temporary lifetime,
- volatility,
- register lowering,
- raw pointer creation,
- array and slice layout,
- string literal layout,
- exact builtin type identity.

Compiler-owned operations are exposed as intrinsic members or internal
intrinsic functions when the compiler must own their semantic identity,
availability, analysis facts, or lowering contract independently of core
source.

Examples:

```sec
value.ptr
text.len
array.len
slice.Len
len(text)
len(array)
len(slice)
```

The public syntax behaves like ordinary member access.

The implementation is compiler-defined.

Representation-sensitive physical implementation alone does not require
compiler-owned semantic identity. A small private `__...` helper in privileged
core source may contain direct MLIR/LLVM where the supported low-level syntax
and contracts permit it.

## 1.2 Core-owned functionality

The core library implements ordinary behavior using privileged `impl` blocks on
built-in types.

Example:

```sec
impl string {
    fn IsEmpty() bool {
        return self.len == 0
    }
}
```

Ordinary user modules may not add `impl` blocks to compiler-owned built-in
types.

The core library may do so because it is compiled as a trusted language module.

A `_...` function declared in trusted core is internal to core; using it requires
trusted core source as well as its ordinary module visibility. A user source
that spells its module `core`, or imitates a core file path, does not acquire
that access. `__...` functions additionally retain their source-file-private
owner boundary. Ordinary user-declared underscore functions keep the usual
module-internal/private visibility rules.


Core may also keep narrow representation-sensitive behavior in private
`__...` helpers. Such a helper may use a direct MLIR/LLVM body when ordinary Sec
cannot express the operation, while the public checked operation remains
ordinary core code. Low-level implementation does not exempt the helper from
ownership, lifetime, effect, unsafe, target, or cleanup validation.

## 1.3 No duplicate wrapper type

A separate `String` type must not be introduced only to provide methods or
associated functions for `string`.

The built-in type itself acts as both:

- the value type,
- the namespace for associated functions.

Examples:

```sec
let text := try string.FromRuneArray(ref runes)
let empty := text.IsEmpty()
```

The implementation must exist only once.

## 1.4 Compiler-known `len`

`len` is a compiler-known core function. It uses ordinary call syntax and is
available without an import:

```sec
/**
 * Returns the Unicode scalar count of a valid UTF-8 string.
 *
 * Returns:
 *   The nonnegative scalar count, as uint.
 */
fn len(value: string) uint

/**
 * Returns the element count of an owning sequence.
 *
 * Parameters:
 *   - value: The array to inspect; no ownership is consumed.
 *
 * Returns:
 *   The nonnegative element count, as uint.
 */
fn len[T](value: T[]) uint

/**
 * Returns the element count of a shared array view.
 *
 * Parameters:
 *   - value: A non-consuming shared array view.
 *
 * Returns:
 *   The nonnegative element count, as uint.
 */
fn len[T](value: ref T[]) uint

/**
 * Returns the element count of a mutable array view.
 *
 * Parameters:
 *   - value: A non-consuming mutable array view.
 *
 * Returns:
 *   The nonnegative element count, as uint.
 */
fn len[T](value: ref mut T[]) uint
```

The compiler infers `T` from the argument. A length is a nonnegative count, so
`len` returns `uint` and denotes exactly the same logical count as `.Len`
(MD-074/MD-088, 2026-10-17,
`rules/corrections/applied/md051-md074-md088-length-numeric-string-correction-20261017.md`).
`len(string)` counts Unicode scalars, never encoded bytes; `string.SizeOf`
remains the UTF-8 payload-byte count.

1. A legitimate `uint` length is never reduced, saturated, wrapped, rejected or
   converted to `int` because `len` was called. Compiler, Semantic IR, Sec
   MLIR, LLVM and other backends preserve the unsigned width and the exact
   value; a target may still enforce its own independently defined
   storage/object-size limits.
2. No implicit `uint` ↔ `int` conversion exists. Code that needs a signed
   offset uses an explicit checked conversion (`try int(len(values))`), which
   fails when the length does not fit.
3. A length and an index are distinct: signed index and offset arithmetic and
   its bounds checks are unchanged; this revision does not broaden the legal
   element index domain or add a panic or error type.
4. `len(values) - 1` on an empty `values` is checked unsigned underflow under
   `rules/foundations/operators.md`; it fails before any indexing and never
   wraps to `uint.Max`. Indexing with a signed `-1` is a separate bounds
   violation. The two are not interchangeable.

```sec
if values.Len > 0 {
    let last := values[len(values) - 1]
    Process(last)
}
```

`len` accepts only `string`, an owning array/sequence, or a shared or mutable
slice reference. It evaluates its argument once, does not allocate and does
not consume the argument. Its name is compiler-owned and cannot be redeclared
or overloaded by user code.

The intrinsic member properties `.Len` (with the migration spelling `.len`) on
strings, arrays and slices are also `uint`; `len(value)` and `value.Len` always
have the same type and value.

## 1.5 Private representation helpers

A private core helper is distinct from a compiler-known intrinsic:

```text
private core helper
    core owns its declaration and implementation
    low-level body is permitted where defined
    no compiler-known identity is implied

compiler-known operation
    compiler owns semantic identity or analysis/lowering contract
    implementation may still delegate to core
```

For checked string substring extraction, the preferred boundary is:

```text
public string.Slice
    validates start and end
    returns Result[string, RangeError]

private __StringSliceUnchecked
    constructs the result string descriptor
    performs no check, allocation, or content validation
```

The helper's intended representation operation is `data + start` with length
`end - start`. Its exact body must use the supported privileged-core low-level
syntax and preserve the declared string lifetime and ownership contract.

---

# 2. Built-in member model

Built-in types participate in ordinary member lookup.

A member may be:

- a compiler intrinsic property,
- a compiler intrinsic method,
- a core-defined instance method,
- a core-defined associated function,
- a core-defined property when later supported.

The parser must not encode a separate call syntax for built-in members.

These expressions use the normal AST forms:

```sec
text.len
text.Trim()
string.FromRuneArray(ref runes)
number.ToString()
```

They are parsed as ordinary member access and call expressions.

Semantic analysis determines the resolved member kind.

---

# 3. `self`

An instance method always has an implicit `self`.

The source declaration does not repeat a receiver parameter.

Valid:

```sec
impl string {
    fn IsEmpty() bool {
        return self.len == 0
    }
}
```

Invalid boilerplate:

```sec
impl string {
    fn IsEmpty(self: string) bool {
    }
}
```

The compiler derives receiver behavior from:

- the method kind,
- whether the method mutates the instance,
- whether it consumes the instance,
- the resolved type,
- ownership and borrowing rules.

Future syntax may explicitly mark mutating or consuming instance methods if
needed, but ordinary instance methods always have `self`.

Associated functions do not have `self`.

Example:

```sec
impl string {
    fn FromRuneArray(value: ref rune[]) Result[string, StringError] {
    }
}
```

---

# 4. Universal pointer member

## 4.1 Syntax

Every addressable value supports:

```sec
value.ptr
```

The result type is:

```sec
RawPtr[T]
```

where `T` is the resolved value type.

For `string`, the pointer refers to its byte storage:

```sec
text.ptr
```

with type:

```sec
RawPtr[byte]
```

## 4.2 Unsafe requirement

Accessing `.ptr` always requires an unsafe context.

Example:

```sec
unsafe {
    let pointer := value.ptr
}
```

This applies even when the pointer is used for:

- FFI,
- system calls,
- hardware access,
- allocator implementation,
- platform code.

## 4.3 Addressability

`.ptr` is valid only for an addressable expression.

Normally addressable:

- local storage,
- parameters with storage,
- fields,
- array elements when their location is stable,
- slice elements when their location is stable,
- static storage,
- addressed register storage,
- compiler-approved string storage.

Normally not addressable:

- pure temporary arithmetic results,
- values that exist only as SSA values,
- temporary conversions,
- temporary function results without materialized storage.

The compiler must not silently allocate or extend a lifetime merely to provide
`.ptr`.

## 4.4 Semantics

`.ptr`:

- does not transfer ownership,
- does not extend lifetime,
- does not create a safe reference,
- does not guarantee non-nullness beyond the source representation,
- does not guarantee continued validity,
- preserves volatile behavior when the source storage is volatile.

---

# 5. String

`string` is a built-in type. Every `string` is valid UTF-8 text, never a
binary buffer (`types.md`, "Valid UTF-8 invariant", MD-050).

The compiler defines:

- string literal construction,
- string representation,
- string lifetime rules,
- equality and comparison primitives when needed,
- Unicode-scalar length (`Len`) and encoded byte length (value-form `SizeOf`),
- raw byte pointer access,
- indexing and slicing primitives where supported.

## 5.1 Intrinsic members

```sec
property Len: uint { get }
property SizeOf: uint { get }

unsafe property ptr: RawPtr[byte] {
    get
}
```

`Len` counts Unicode scalar values and the value-form `SizeOf` counts encoded
UTF-8 bytes (MD-058); no public `ByteLen`, `RuneLen` or `CharLen` alias exists.
Both are compiler-owned, read-only properties; privileged core declarations
cannot change their units. Byte pointer arithmetic, byte offsets and byte slices
explicitly use `SizeOf`.

The compatibility lowercase `.len` and compiler-known `len(text)` count runes;
`len(text)` retains its `int` result. This supersedes the earlier byte-count
meaning (string-length-contracts-correction-20261009).

`Len` and `SizeOf` are independent facts that Sema, Semantic IR, Sec MLIR, LSP
and runtime helpers carry separately.

## 5.2 Required associated functions

```sec
impl string {
    fn FromRuneArray(value: ref rune[]) Result[string, StringError]
}
```

It is a compiler-known associated function supplied by trusted core (MD-072,
2026-10-17). It **borrows** the input array: it is neither consumed nor
mutated, and the resulting string stays valid after the array is moved,
modified or destroyed. It never adopts or aliases the array's mutable backing;
a sharing optimization must still produce an independently owned, immutable
result. No consuming zero-copy overload exists in Sec 0.1.

**Byte decoding is not a core API (MD-091, 2026-10-18).** Public conversion
from a `byte[]` sequence to a `string` is owned by `stdlib/str`, because
materializing the string allocates or needs explicit result storage. Core has
no `string.FromByteArray`, not even as an alias, and there is no `string(bytes)`
cast and no implicit `byte[]` → `string` conversion. The public APIs are
(`sec/stdlib/str/stdlib_str.md`, `sec/stdlib/str/collect.sec`):

```sec
// Decode a borrowed byte sequence as strictly validated UTF-8.
fn FromByteArray(value: ref byte[]) Result[string, StringError]

// Decode a borrowed byte sequence while replacing malformed UTF-8.
fn FromByteArrayLossy(value: ref byte[]) Result[string, CollectionError]
```

`str.FromByteArray` decodes its entire represented input as strict UTF-8 —
never as Latin-1, ASCII with extended bytes or a platform default. Malformed
input returns `Err(StringError.InvalidUtf8(offset))`, where `offset` is the
zero-based byte offset of the leading byte of the first malformed or truncated
sequence (or of the first immediately invalid byte). It does not replace,
truncate, skip, panic or return a partially decoded string. The internal
materialization capability `_stringFromUtf8` is known only to trusted core
(`compiler_known_members.md`, "Internal core string materialization"); how the
standard library materializes validated bytes is open as MD-109.

`FromRuneArray` encodes the scalars to canonical UTF-8 in order. Safe `rune`
values are always valid scalars, so ordinary input cannot produce
`InvalidCodePoint`; an invalid rune fabricated by broken privileged code is an
invariant violation.

`FromRuneArray` may allocate through the active permitted allocation context
and reports storage failure as `StringError.Allocation(AllocationError)`. It
does not append a terminator, normalize or change case. Failure leaves the borrowed source
unchanged and exposes no partially formed string or leaked storage.

`byte`, `char`, and `rune` arrays and slice views additionally provide a
compiler-known materializing method:

```sec
let runes: rune[2] := ['A', 'B']
let text := try runes.ToString()
```

For those three element types, `ToString()` accepts no arguments, has the
universal result `Result[string, StringError]`, and does not mutate or consume
the source. **A byte sequence's `ToString()` never interprets its contents as
text (MD-091):** it is the general representational operation, not decoding.
For the dynamic `byte[]` its result is the type summary `byte[]`
(`let description := try bytes.ToString() // "byte[]"`) whatever the bytes
contain; fixed byte arrays and borrowed byte views keep their collection
fallback formatting and likewise never decode. Producing the summary still
uses the ordinary `ToString` storage/error contract. Decode bytes with
`str.FromByteArray` or `str.FromByteArrayLossy`; char sequences apply
`char.ToString()` element semantics (char-domain details: MD-073); rune
sequences encode each Unicode scalar as `string.FromRuneArray` does. Other
concrete values retain the universal compiler-provided fallback described by
`compiler_known_members.md`, but do not use these text-sequence semantics.

## 5.3 Required instance methods

```sec
impl string {
    fn ToString() Result[string, StringError]

    fn IsEmpty() bool

    fn Compare(other: string) int
    fn StartsWith(prefix: string) bool
    fn EndsWith(suffix: string) bool
    fn Contains(value: string) bool

    fn IndexOf(value: string) Option[uint]
    fn IndexOf(value: string, start: uint) Option[uint]
    fn LastIndexOf(value: string) Option[uint]
    fn Slice(start: uint, end: uint) Result[string, RangeError]

    fn GetRune(index: uint) Result[rune, IndexError]
    fn RuneAt(index: uint) rune
    fn GetByte(index: uint) Result[byte, IndexError]
    fn ByteAt(index: uint) byte
    fn CharAt(index: uint) char

    fn Trim() string
    fn TrimStart() string
    fn TrimEnd() string

    fn Split(separator: string) StringSplitIterator

    fn ToByteArray() Result[byte[], CollectionError]
    fn ToRuneArray() Result[rune[], CollectionError]
}
```

This is a minimum surface, not a complete string API. The position units of
the search, slicing and access members are defined in § 5.7. Every public
declaration in trusted core carries a structured documentation comment stating
its position unit, errors and allocation.

`ToByteArray` and `ToRuneArray` (MD-072) materialize owning arrays whose storage
is independent of the source string; callers may mutate or move them without
altering the string, and no mutable alias to the string's UTF-8 storage exists.
`ToByteArray` returns exactly `self.SizeOf` bytes — the UTF-8 encoding already
represented, with no transcoding, terminator or unused capacity. `ToRuneArray`
returns exactly `self.Len` scalar values in order. Neither has an
`InvalidUtf8` path, because the source is valid. Storage failure uses the
existing `CollectionError` family, consistent with array allocation and growth;
a failed materialization is never reported as a successful empty array.

## 5.4 Bytes and runes

The following names must not duplicate `ToByteArray` or `ToRuneArray`:

```sec
Bytes()
Runes()
```

If added later, they must have distinct non-materializing semantics.

Possible meanings:

```sec
fn Bytes() StringByteIterator
fn Runes() StringRuneIterator
```

or direct language iteration may make them unnecessary.

Materializing conversions use:

```sec
ToByteArray()
ToRuneArray()
```

Iteration must use iterator semantics or direct `for` support.

## 5.5 ToString

`string.ToString()` is an identity operation.

It is included for consistency with generic formatting and type interfaces.

```sec
impl string {
    fn ToString() Result[string, StringError] {
        return Ok(self)
    }
}
```

## 5.6 Allocation visibility

Methods that allocate must make failure and allocation policy explicit.

A method must not silently allocate merely because its name resembles a common
operation in another language.

Operations such as these may require later allocating variants:

```sec
Replace
ToUpper
ToLower
Join
Copy
```

They are not mandatory in the minimum core surface.

## 5.7 Rune positions and byte access (MD-071)

The following members accept and return **zero-based Unicode-scalar (rune)
indexes**, not UTF-8 byte offsets:

```text
IndexOf(value)            IndexOf(value, start)     LastIndexOf(value)
Slice(start, end)         GetRune(index)            RuneAt(index)
```

1. Every index in `0..self.Len` denotes a boundary between complete scalars.
2. `Slice(start, end)` selects the half-open rune range `[start, end)` and never
   cuts a scalar. A reversed range returns `RangeError.StartAfterEnd` and an
   end beyond `self.Len` returns `RangeError.OutOfBounds`; no scalar-boundary
   error exists for this API.
3. `IndexOf(value, start)` searches from rune index `start` and returns the rune
   index of the first exact match. A `start` beyond `self.Len` returns `None`.
   An empty `value` returns `Some(start)` when `start <= self.Len`.
4. `IndexOf(value)` equals `IndexOf(value, 0)`. `LastIndexOf(value)` returns the
   rune index of the last exact match; an empty `value` returns
   `Some(self.Len)`.
5. Searches are ordinal Unicode-scalar sequence searches with no
   normalization, case folding or locale rules.
6. `GetRune` returns `IndexError.OutOfBounds` and `RuneAt` panics when `index`
   is not below `self.Len`.

The separately named byte-access members `GetByte(index)`, `ByteAt(index)` and
`CharAt(index)` use **UTF-8 byte indexes** below `self.SizeOf` and are
documented as access to encoded bytes. No writable string byte reference
exists.

`CharAt(index)` is a permanent public convenience alias (MD-092, decided
2026-10-27; `rules/corrections/MD-092-string-CharAt-byte-alias-correction.md`).
For every index it behaves exactly as `char(self.ByteAt(index))`:
- **Result.** It returns one encoded byte as a `char` with value `0..255`. It does not decode a scalar, inspect adjacent bytes or normalize, and its result never converts implicitly to `rune`.
- **Index domain.** Every byte index below `self.SizeOf` is valid, continuation bytes included. `self.Len` is not the bound.
- **Out of range.** An index at or beyond `SizeOf` has exactly `ByteAt`'s panic. There is no second bounds policy and no `Result`.
- **Behavior.** It is read-only, does not allocate and has no special ownership or lifetime behavior.
- **Delegation.** `ByteAt` remains the single authority for byte retrieval, and `CharAt` delegates to it.
- **Callers.** The member is neither deprecated nor scheduled for removal, and calling it raises no diagnostic. Callers that scan encoded ASCII bytes may use it, while callers that scan scalars use the rune members.

Implementations may scan UTF-8 bytes internally, including byte-based
substring matching and lazy iterators that keep byte and rune counters without
allocating, but must translate every externally visible position to rune
coordinates. Private byte-offset helpers such as `__StringSliceUnchecked`
receive only verified byte coordinates on scalar boundaries.

These rules apply to `string` only; array, `vector`, list, binary-view and
`char[]` positions keep the index semantics of their own types.

Hover and diagnostics name the unit of every `uint` position: `IndexOf`
returns rune indexes and `GetByte` takes a byte index.

---

# 6. Boolean

## 6.1 Intrinsic member

```sec
unsafe property ptr: RawPtr[bool] {
    get
}
```

## 6.2 Required method

```sec
impl bool {
    fn ToString() Result[string, StringError]
}
```

Logical negation remains an operator:

```sec
!value
```

No `Not()` method is required.

---

# 7. Signed integers

This section applies to:

```text
int
int8
int16
int32
int64
int128
int256
```

## 7.1 Intrinsic associated members

Spelling (decided 2026-10-26): numeric associated properties use the canonical
PascalCase spellings of `compiler_known_members.md` ("Fundamental numeric
associated properties"): `Min`, `Max`, `Bits` here and in § 8.1, and `Min`,
`Max`, `Epsilon`, `Infinity`, `NegativeInfinity`, `NaN` in § 9.1. No lowercase
migration spelling is accepted.

```sec
type property Min: T {
    get
}

type property Max: T {
    get
}

type property Bits: uint {
    get
}
```

Usage:

```sec
let smallest := int32.min
let largest := int32.max
let width := int32.bits
```

## 7.2 Intrinsic instance member

```sec
unsafe property ptr: RawPtr[T] {
    get
}
```

## 7.3 Required methods

```sec
impl T {
    fn ToString() Result[string, StringError]
    fn ToString(format: string) Result[string, StringError]

    fn Abs() Result[T, OverflowError]

    fn Min(other: T) T
    fn Max(other: T) T
    fn Clamp(minimum: T, maximum: T) Result[T, RangeError]

    fn IsEven() bool
    fn IsOdd() bool

    fn CountOnes() uint
    fn CountZeros() uint
    fn LeadingZeros() uint
    fn TrailingZeros() uint

    fn RotateLeft(amount: uint) T
    fn RotateRight(amount: uint) T

    fn SwapBytes() T
}
```

The actual declarations are generated or instantiated for every signed integer
type.

`Abs()` is fallible because the smallest signed value may not have a positive
representation in the same type.

---

# 8. Unsigned integers and byte

This section applies to:

```text
uint
uint8
uint16
uint32
uint64
uint128
uint256
byte
```

## 8.1 Intrinsic associated members

```sec
type property Min: T {
    get
}

type property Max: T {
    get
}

type property Bits: uint {
    get
}
```

## 8.2 Intrinsic instance member

```sec
unsafe property ptr: RawPtr[T] {
    get
}
```

## 8.3 Required methods

```sec
impl T {
    fn ToString() Result[string, StringError]
    fn ToString(format: string) Result[string, StringError]

    fn Min(other: T) T
    fn Max(other: T) T
    fn Clamp(minimum: T, maximum: T) Result[T, RangeError]

    fn IsEven() bool
    fn IsOdd() bool

    fn CountOnes() uint
    fn CountZeros() uint
    fn LeadingZeros() uint
    fn TrailingZeros() uint

    fn RotateLeft(amount: uint) T
    fn RotateRight(amount: uint) T

    fn SwapBytes() T
}
```

Unsigned types do not require `Abs()`.

---

# 9. Floating-point types

This section applies to:

```text
float
float32
float64
```

## 9.1 Intrinsic associated members

```sec
type property Min: T {
    get
}

type property Max: T {
    get
}

type property Epsilon: T {
    get
}

type property Infinity: T {
    get
}

type property NegativeInfinity: T {
    get
}

type property NaN: T {
    get
}
```

## 9.2 Intrinsic instance member

```sec
unsafe property ptr: RawPtr[T] {
    get
}
```

## 9.3 Required methods

```sec
impl T {
    fn ToString() Result[string, StringError]
    fn ToString(format: string) Result[string, StringError]

    fn Abs() T
    fn Min(other: T) T
    fn Max(other: T) T
    fn Clamp(minimum: T, maximum: T) Result[T, RangeError]

    fn IsFinite() bool
    fn IsInfinite() bool
    fn IsNaN() bool
    fn IsNormal() bool
    fn IsSubnormal() bool

    fn Floor() T
    fn Ceiling() T
    fn Truncate() T
    fn Round() T

    fn SquareRoot() Result[T, MathError]
}
```

Advanced mathematics such as trigonometric, logarithmic and exponential
functions belongs in a higher-level math module unless later promoted into
core.

---

# 10. Decimal

## 10.1 Intrinsic members

```sec
unsafe property ptr: RawPtr[decimal] {
    get
}

property Scale: uint8 {
    get
}
```

`Scale` is the canonical spelling of the read-only `uint8` instance property
of a `decimal` value; it is not a static `decimal.Scale` on the type
namespace. The lowercase `scale` is a temporary migration spelling resolving
to the same member (`compiler_known_members.md`, "Legacy lowercase member
spellings"). It does not exist on `decimal128` until MD-068 is decided
(MD-082, 2026-10-18).

The internal coefficient representation must not be exposed as a public member
unless the representation becomes a permanent language guarantee.

## 10.2 Required methods

```sec
impl decimal {
    fn ToString() Result[string, StringError]
    fn ToString(format: string) Result[string, StringError]

    fn Abs() Result[decimal, OverflowError]

    fn Min(other: decimal) decimal
    fn Max(other: decimal) decimal
    fn Clamp(
        minimum: decimal,
        maximum: decimal,
    ) Result[decimal, RangeError]

    fn Floor() decimal
    fn Ceiling() decimal
    fn Truncate() decimal
    fn Round() decimal

    fn RoundTo(scale: uint8) Result[decimal, PrecisionError]
    fn Rescale(scale: uint8) Result[decimal, PrecisionError]

    fn IsInteger() bool
    fn IsZero() bool
    fn Sign() int
}
```

## 10.3 Rounding modes

Numeric core declares the one rounding-policy enumeration
(`sec/core/rounding.sec`, MD-015, 2026-10-13):

```sec
enum RoundingMode {
    NearestEven
    NearestAwayFromZero
    TowardZero
    AwayFromZero
    Ceil
    Floor
}
```

`RoundingMode` belongs to core's numeric API, not to `core/error.sec`, and is
available without importing `stdlib/math`. An explicit conversion that accepts
a rounding policy takes it as a `RoundingMode` argument and applies it exactly
once to the exact mathematical result; no mode is ever selected implicitly.
Numeric and unit conversions (`T(value, mode)`, `T<U>(value, mode)`) are
defined in `rules/types/units.md` "Explicit rounding" (MD-047, 2026-10-13) and
the time-quantity conversion `duration(q, mode)` in
`rules/types/temporal.md` § 5.6. An ordinary `math.Round()` belongs to
`stdlib/math` and consumes this enum; its overload family is specified by the
`stdlib/math` rulebook, not here, and no other rounding enum exists.

---

# 11. Character and rune

The exact distinction between `char` and `rune` is defined by the type rules.

Both are built-in types and support `.ptr` when addressable.

## 11.1 Character

```sec
impl char {
    fn ToString() Result[string, StringError]

    fn IsDigit() bool
    fn IsLetter() bool
    fn IsLetterOrDigit() bool
    fn IsWhitespace() bool
    fn IsControl() bool
}
```

## 11.2 Rune

```sec
impl rune {
    fn ToString() Result[string, StringError]

    fn IsDigit() bool
    fn IsLetter() bool
    fn IsLetterOrDigit() bool
    fn IsWhitespace() bool
    fn IsControl() bool

    fn IsAscii() bool
    fn Utf8Length() uint
}
```

Simple one-to-one case conversion may later be added.

Full Unicode case conversion belongs on strings because one rune may map to
multiple runes.

---

# 12. Arrays

This section applies to fixed arrays using the final Sec postfix array syntax.

## 12.1 Compiler-known properties

```sec
property Len: uint {
    get
}

property IsEmpty: bool {
    get
}

unsafe property Ptr: RawPtr[T] {
    get
}

property SizeOf: uint {
    get
}
```

## 12.2 Required methods

```sec
impl T[N] {
    fn ToString() Result[string, StringError]
}
```

Borrowed views are formed with the explicit reference-slicing syntax from
`collections.md`. `AsSlice`, `AsMutableSlice`, `First`, and `Last` are not
universal array members.

Methods requiring equality, ordering, copying or allocation may be added through
generic constraints later.

Examples:

```sec
Contains
IndexOf
Sort
Map
Filter
```

They are not mandatory in the minimum core surface.

---

# 13. Slices

Slices are non-owning views.

## 13.1 Compiler-known properties

```sec
property Len: uint {
    get
}

property IsEmpty: bool {
    get
}

unsafe property Ptr: RawPtr[T] {
    get
}

property SizeOf: uint {
    get
}
```

The `.Len` property and global `len(slice)` are the same exact, non-consuming
`uint` element count (§ 1.4). No signed-length observation or implicit narrowing
is introduced. Shared slices are copyable, mutable slices move-only; both are
trivially destructible and own no elements. Their explicit range construction,
canonical `RangeError` precedence and `IndexError.OutOfBounds` element checks
are specified in `rules/collections/collections.md` §§ 7.1–7.6. Dynamic
reference validity is checked first; spatial `try` handling never catches
`panic.invalid-reference-generation`. Slice `==`/`!=` remain invalid.

## 13.2 Required shared-slice methods

```sec
impl ref T[] {
    fn ToString() Result[string, StringError]
}
```

## 13.3 Required mutable-slice methods

```sec
impl ref mut T[] {
    fn Reverse() void
    fn Fill(value: T) void where T: Copy
}
```

Sub-slices are formed with explicit reference slicing. The core library does
not add `Slice`, iterator-helper, `First`, or `Last` members. Iteration uses the
language's ordinary collection iteration protocol.

`Contains`, `IndexOf`, and similar conveniences are not privileged core
members; they require ordinary library APIs and the corresponding generic
constraints.

---

# 14. RawPtr

`RawPtr[T]` is compiler-known and unsafe.

Minimum operations:

```sec
impl RawPtr[T] {
    unsafe fn Read() T
    unsafe fn Write(value: T) void

    unsafe fn Offset(elements: int) RawPtr[T]
    unsafe fn AddBytes(bytes: int) RawPtr[byte]

    unsafe fn Difference(other: RawPtr[T]) int
}
```

Null-related members are added only when null semantics are finalized.

Raw pointer operations do not create ownership.

---

# 15. ToString and formatting

`ToString()` is part of the minimum core surface for fundamental printable
types.

Required:

```sec
fn ToString() Result[string, StringError]
```

Numeric types also provide:

```sec
fn ToString(format: string) Result[string, StringError]
```

Every `ToString`, including each overload, returns exactly
`Result[string, StringError]` (decision 2026-10-03, MD-035; see
`rules/compiler/compiler_known_members.md` "Uniform `ToString` result").

`.ToString()` is the preferred numeric-to-text spelling. The explicit
conversion `string(numericValue)` is the same operation as the numeric type's
no-argument `ToString()` — same default formatting, same
`Result[string, StringError]` result and failure — and has no format overload
(`types.md`, "Numeric-to-text conversion", MD-051, 2026-10-17).

This primitive `string(number)` convenience is not inherited by named string
types (MD-093, 2026-10-26). For `type Name string`, `Name(number)` is invalid,
including under `try`. Prefer `try Name(number.ToString())`: first materialize
text with its `StringError` channel, then perform the ordinary nominal
conversion and any independently fallible `ConversionError.Contract` checks.
`Name(try number.ToString())` is valid only when the outer conversion is
infallible or is separately protected. General `try` boundaries and declared
error-channel compatibility remain defined by `errorhandling.md` §§ 11–14.

Higher-level formatting with heterogeneous arguments builds on the general
heterogeneous native variadic type packs (`rules/declarations/functions.md`
§ 38A, MD-117); core defines no formatting-only argument rule, and the public
`fmt` surface is decided separately (MD-118).

## 15.1 `StringError`

`StringError` is the one compiler-known failure family of text. Every
`ToString`, every runtime string concatenation, and every runtime interpolation
fails with it, so all text-producing operations share one error channel:

```sec
type StringError union error {
    Allocation(AllocationError),
    Format(FormatError),
    InvalidUtf8(uint),
    InvalidCodePoint,
    OutOfBounds,
}

enum FormatError error {
    InvalidFormat,
    UnsupportedFormat,
    InvalidSpecifier,
    InvalidPrecision,
}
```

`Allocation` carries the `AllocationError` of a failed text materialization.
`Format` carries the `FormatError` of a format overload such as
`ToString(format)` whose format is invalid or unsupported at run time.
`InvalidUtf8`, `InvalidCodePoint`, and `OutOfBounds` are the text decoding
failures of string conversions (`string.FromRuneArray`, the stdlib
`str.FromByteArray`, UTF-8 decoding). `InvalidUtf8(offset)` carries the
zero-based byte offset of the leading byte of the first malformed or truncated
UTF-8 sequence; strict decoding reports it instead of producing a malformed
string (§ 5.2, MD-050). Safe `FromRuneArray` input cannot produce
`InvalidCodePoint`. `StringError` and `FormatError` are
compiler-known identities; trusted core supplies their source declarations and
ordinary code cannot redeclare them.

The format-string overload allows concise formatting rules.

Format strings must be validated at compile time when the argument is a
compile-time literal and the format grammar is known. A format that is known
only at run time and is invalid or unsupported fails with
`StringError.Format`.

A higher-level strongly typed formatting API may later be added.

Examples may include:

```sec
value.ToString("D")
value.ToString("X")
value.ToString("N2")
value.ToString("E4")
```

The exact format grammar is defined in a separate formatting rule.

The presence of `ToString()` on `string` is intentional even though it returns
`Ok` with the same value.

---

# 16. Named and related types

A named type remains semantically distinct from its underlying type.

Example:

```sec
type Speed decimal
type CustomerID uint64
```

Core behavior may be made available to related named types when the operation is
valid and preserves the named type's semantics.

## 16.1 Related-type inheritance rule

A named type may receive a builtin/core member from its underlying type when all
of the following are true:

- the operation is representation-compatible,
- the operation does not erase nominal identity,
- the operation does not bypass contracts,
- the operation does not bypass unit semantics,
- the operation does not introduce an implicit conversion,
- the result type can be correctly substituted.

Examples:

```sec
Speed.IsZero() -> bool
Speed.Abs() -> Result[Speed, OverflowError]
Speed.Min(other: Speed) -> Speed
CustomerID.ToString() -> Result[string, StringError]
```

Not automatically inherited:

- operations whose parameter uses the raw underlying type,
- operations whose result would silently become the raw underlying type,
- operations that violate a range or unit contract,
- operations that would permit invalid cross-domain arithmetic.

In particular, primitive numeric-to-text conversion into `string` does not
authorize direct conversion into a named string type. Materialization and
nominal validation remain explicit, ordered operations (§ 15; MD-093).

## 16.2 Sema substitution

When resolving an inherited core member:

- `Self` is substituted with the named type,
- parameters using the underlying type as the receiver domain are substituted
  with the named type when the operation is type-preserving,
- return types are substituted with the named type when valid,
- non-type-preserving members retain their declared result type,
- contract checks remain active,
- unit algebra remains active.

The compiler must not treat this as an implicit cast.

---

# 17. Core loading and compilation

The core library is conceptually present in every compilation.

It does not require an explicit import.

## 17.1 Compilation order

Recommended order:

1. Register compiler builtin types.
2. Register compiler intrinsic members.
3. Load and parse the core module.
4. Register privileged `impl` blocks for built-in types.
5. Register core error types.
6. Register core generic types such as `Option` and `Result`.
7. Complete core declaration resolution.
8. Load user modules.
9. Perform ordinary semantic analysis.
10. Instantiate related-type members as required.

Core declarations must be available before user function bodies are checked.

## 17.2 No source injection into user files

The compiler must not prepend core source text to user source files.

The core module is loaded as a separate compiler-owned module.

This preserves:

- source positions,
- diagnostics,
- module boundaries,
- incremental compilation,
- tooling behavior,
- AST integrity.

---

# 18. AST integration

## 18.1 Ordinary AST nodes

Core source uses the same AST nodes as ordinary Sec source:

- `ImplDeclaration`,
- `FunctionDeclaration`,
- `PropertyDeclaration`,
- `TypeDeclaration`,
- `EnumDeclaration`,
- `UnionDeclaration`,
- `InterfaceDeclaration`.

Builtin member use also uses ordinary expressions:

- `MemberExpression`,
- `CallExpression`,
- `AssignmentExpression`,
- `IndexExpression`.

The parser must not create separate syntax nodes for each core method.

## 18.2 Intrinsic declarations

Compiler intrinsics should be represented in the declaration model even when
they have no ordinary source body.

Suggested semantic declaration metadata:

```text
BuiltinMember
    Name
    TargetType
    MemberKind
    ReceiverMode
    Parameters
    ReturnType
    IntrinsicID
    UnsafeRequired
    AddressableRequired
    TypeLevel
```

Possible member kinds:

```text
IntrinsicProperty
IntrinsicMethod
CoreMethod
CoreAssociatedFunction
```

Possible intrinsic IDs:

```text
StringLen
ValuePtr
ArrayLen
SliceLen
IntegerMin
IntegerMax
IntegerBits
FloatNaN
FloatInfinity
```

AST may hold synthetic declaration nodes, but intrinsic identity must be carried
into Semantic IR.

## 18.3 Synthetic source positions

Compiler-generated intrinsic declarations must use a distinct synthetic source
location.

Diagnostics should still point primarily to the user's member access or call.

Core source declarations should point to the actual core source file when
compiler diagnostics refer to their implementation.

---

# 19. Semantic analysis integration

## 19.1 Unified member table

Sema should use one member-resolution system for:

- user-defined methods,
- core methods,
- intrinsic properties,
- intrinsic methods,
- associated functions,
- properties,
- fields,
- register fields.

Suggested structure:

```text
TypeMembers
    Fields
    Properties
    Methods
    AssociatedFunctions
    Intrinsics
```

A lookup for:

```sec
value.Member
```

must consider the resolved type and its related-type rules.

A lookup for:

```sec
Type.Member
```

must consider type-level members and associated functions.

## 19.2 Core registry

The semantic analyzer should maintain a core registry:

```text
CoreRegistry
    BuiltinTypes
    BuiltinMembers
    CoreImpls
    CoreErrors
    RelatedTypeRules
    IntrinsicDefinitions
```

The registry is initialized once per compilation context.

## 19.3 Related type lookup

Member resolution order for a named type should be:

1. Direct members declared on the named type.
2. Direct privileged core members declared for that exact type.
3. Eligible related members from the underlying builtin type.
4. Interface-provided callable surface where applicable.
5. Failure diagnostic.

Direct members must be able to override eligible inherited core behavior where
the language permits overriding.

The compiler must reject ambiguous member surfaces.

## 19.4 Overload resolution

Core methods participate in ordinary overload resolution.

Return type alone must not distinguish overloads.

Example:

```sec
value.ToString()
value.ToString("N2")
```

are distinguished by parameter count and parameter types.

## 19.5 Unsafe checking

Intrinsic metadata must carry whether unsafe context is required.

For:

```sec
value.ptr
```

Sema must check:

- current unsafe context,
- addressability,
- source type,
- resulting `RawPtr` type,
- volatility,
- target restrictions where applicable.

## 19.6 Lowering

After Sema resolves a member, Semantic IR must explicitly record the operation.

Examples:

```text
CoreCall
IntrinsicRead
IntrinsicAddressOf
StringLength
ArrayLength
SliceLength
IntegerIntrinsic
```

LLVM or MLIR must not infer high-level core semantics from source syntax.

---

# 20. Standard core errors

Core defines errors that are required by fundamental language operations and do
not naturally belong to an optional standard-library module.

These errors are always available.

The exact representation may use enums, unions or named error types according to
the final error-type rules.

## 20.1 Required minimum errors

```sec
enum OverflowError {
    Overflow
}

enum DivisionByZeroError {
    DivisionByZero
}

enum ArithmeticError {
    Overflow
    DivisionByZero
    InvalidShift
}

enum EnumValueError {
    UndeclaredValue
    OutOfRange
}

enum RangeError {
    InvalidRange
    StartAfterEnd
    OutOfBounds
}

enum IndexError error {
    OutOfBounds
}

enum PrecisionError {
    PrecisionLoss
    ScaleOverflow
    PrecisionExhausted
}

enum FormatError {
    InvalidFormat
    UnsupportedFormat
    InvalidSpecifier
    InvalidPrecision
}

enum EncodingError {
    InvalidEncoding
    InvalidUtf8
    InvalidCodePoint
}

enum MathError {
    DomainError
    Overflow
    Underflow
    NotFinite
}

enum AllocationError {
    OutOfMemory
    Unsupported
    InvalidSize
    InvalidAlignment
}
```

This is a minimum list.

Additional core errors may be introduced when required by language-level
features.

## 20.2 Core versus standard library errors

An error belongs in core when:

- a builtin operator may produce it,
- a builtin type method requires it,
- allocation semantics require it,
- fundamental indexing or slicing requires it,
- conversion or formatting of builtin types requires it,
- the compiler must recognize it without importing a module.

An error belongs in the standard library or a higher module when it describes:

- files,
- sockets,
- processes,
- operating-system APIs,
- networking,
- parsing of a specific file format,
- database access,
- protocols,
- application domains.

Examples not belonging in core:

```text
IOError
FileNotFoundError
SocketError
HttpError
JsonError
DatabaseError
```

## 20.3 Error identity

Core errors are normal named Sec types.

They must:

- participate in `Result`,
- participate in `match`,
- preserve exact type identity,
- require explicit conversion or mapping to another error type,
- be registered before user semantic analysis.

The compiler must not treat all errors as one universal runtime error value.

## 20.4 No mandatory runtime

Core errors must not require:

- exception objects,
- stack unwinding,
- heap allocation,
- reflection,
- runtime type lookup,
- a global error registry.

They are ordinary statically known values.

---

# 21. Standard core generic types

The following fundamental types should be available from core:

```sec
Option[T]
Result[T, E]
```

They are language-level or core-level generic types and must be available
without standard-library imports.

Their parser and semantic behavior may be compiler-assisted, but their type
identity and member surface should be visible through the core declaration
model.

Possible minimum members:

```sec
impl Option[T] {
    fn IsSome() bool
    fn IsNone() bool
}

impl Result[T, E] {
    fn IsOk() bool
    fn IsErr() bool
}
```

Extraction, transformation and iterator-style helpers may be added later.

---

# 22. Restrictions on core

Core must not:

- depend on the standard library,
- depend on operating-system services,
- silently allocate without an explicit rule,
- require global initialization,
- require global destruction,
- require garbage collection,
- expose compiler-private representation accidentally,
- bypass ownership or borrowing,
- make unsafe operations safe merely by wrapping them,
- duplicate language operators with unnecessary methods,
- introduce a second public type merely to decorate a builtin type.

Core may use:

- privileged impl declarations,
- compiler intrinsics,
- core-only internal declarations,
- target-independent low-level helpers,
- explicitly target-selected implementations where semantics remain identical.

---

# 23. Initial implementation strategy

Recommended implementation phases:

## Phase 1

- core module loading,
- intrinsic member registry,
- privileged `impl` on built-in types,
- `string.len`,
- universal unsafe `.ptr`,
- `ToString()` for fundamental types,
- standard core errors,
- `Option` and `Result` registration.

### Current implementation status

Implemented:

- `sec/core/*.sec` is parsed as a compiler-owned core library by the compiler
  before ordinary import resolution and user semantic analysis.
- The LSP loads the same `sec/core/*.sec` sources before diagnostics and
  completion semantic analysis.
- Core source is loaded as separate parsed source files with their original
  token file paths. It is not prepended as text to user files.
- Privileged `impl` blocks on compiler-known built-in/core types are allowed
  only from files under `sec/core`.
- Ordinary user files still may not add `impl` blocks to compiler-owned
  built-in types such as `string`.
- Core impl bodies on built-in targets are semantically analyzed.
- Compiler-known core function `len(...)` is semantically available without
  imports for strings, owning arrays/sequences and `ref`/`ref mut` slices. It
  returns `uint`, is shown by LSP global completion and rejects user
  redeclaration.
- `string.Len`, array `.Len` and slice `.Len` are intrinsic members returning
  `uint`.
- `byte`, `char`, and `rune` arrays and slice views provide compiler-known
  `ToString()` text materialization, including fixed arrays such as `rune[2]`.
- Impl-method receiver mutability is inferred through evaluated expressions,
  including method calls used as `let` and grouped-`let` initializers, control
  conditions, assignment values, and nested expression operands.
- Universal `.ptr` member resolution exists for addressable values and returns
  `RawPtr[T]`.
- `string.ptr` returns `RawPtr[byte]`.
- `.ptr` access requires an unsafe context.
- The initial addressability check for `.ptr` accepts identifiers, member
  expressions, index expressions and compiler-approved string literals, and
  rejects pure temporary expressions.
- Core errors currently defined in core source:
  `OverflowError`, `DivisionByZeroError`, `RangeError`, `IndexError`,
  `PrecisionError`, `FormatError`, `EncodingError` and `MathError`.
- `AllocationError` remains compiler-known and has the minimum core values
  `OutOfMemory`, `Unsupported`, `InvalidSize` and `InvalidAlignment`.
- `ArithmeticError` is compiler-known, always available and has the exact
  values `Overflow`, `DivisionByZero` and `InvalidShift`. It is used by naked
  `try` over checked integer arithmetic and does not implicitly convert to
  another error or an error union.
- `EnumValueError` is compiler-known, always available and has the exact values
  `UndeclaredValue` and `OutOfRange`. It is used by checked runtime
  integer-to-enum conversion.
- Compiler intrinsic types are registered in semantic-analysis metadata and
  remain compiler-owned rather than declared in core source. This includes
  fundamental builtins (`bool`, integer/float/decimal types, `string`, `void`,
  `never`) and compiler-known generic/runtime-facing types such as `RawPtr[T]`,
  `Option[T]`, `Result[T, E]`, `Task[T]`, `Mutex[T]`, `MutexGuard[T]`,
  `Atomic[T]`, `CompareExchangeResult[T]`, `Event[T]`,
  `EventStorage[T, Capacity]`, `Subscription`, `EventSubscribeResult`, `Arena`
  `AllocationError` and `EnumValueError`.
- `RawPtr[void]` is supported as an extern C/FFI pointer type.
- Minimum core source currently includes `bool.ToString()`, selected `int`
  and `uint` methods, a semantically valid subset of `string` methods
  including `ToString()`, `IsEmpty()` and `ToRuneArray()`, plus the minimum
  `rune` method declarations used by bootstrap lexing: `ToString()`, ASCII
  classification methods, `IsAscii()` and `Utf8Length()`.

Partially implemented:

- Intrinsic members are implemented directly in semantic analysis rather than
  through a complete unified `BuiltinMember` declaration table.
- `len(...)` is recognized directly by semantic analysis. Its distinct
  Semantic IR and backend lowering are pending.
- `.ptr` lowering still relies on existing backend member emission and is not
  yet represented as a distinct Semantic IR intrinsic operation.
- `string.ToRuneArray()`, `rune.ToString()` and `rune.Utf8Length()` are
  semantically available deterministic stubs. They do not yet materialize
  source data or provide runtime conversion behavior.
- `rune.IsDigit()`, `IsLetter()`, `IsLetterOrDigit()`, `IsWhitespace()` and
  `IsControl()` currently classify ASCII only. Full Unicode classification is
  still pending.
- Other core method bodies that require allocation, string scanning, Unicode,
  formatting, iterators or owned slices remain deterministic stubs.
- `StringSplitIterator` explicitly implements the compiler-known
  `Iterator[string]` contract. Frontend `for part in text.Split(separator)`
  resolution is implemented; Semantic IR and backend lowering of the resolved
  static `Next()` loop remain pending in `frontend.for-loops-v2`.

Pending:

- Full intrinsic member registry and unified type-member table.
- Type-level numeric members such as `int.min`, `int.max` and `int.bits`.
- Complete integer, unsigned integer, float, decimal, char, rune, array,
  slice and raw-pointer method surfaces.
- Related named-type member inheritance and substitution.
- Compile-time format string validation.
- Complete backend/IR representation for core intrinsics.
- Allocation-aware materialization and runtime lowering for
  `string.ToByteArray()` and `string.ToRuneArray()`.

Current intentional deviation from the desired final surface:

- The rulebook specifies `FromRuneArray(value: rune[])`, `ToByteArray() byte[]` and
  `ToRuneArray() rune[]`. The current core source retains ref-based
  constructors while `ToRuneArray() rune[]` is available as a semantic stub.
  Final ownership, allocation and runtime materialization semantics remain
  pending.

## Phase 2

- integer core methods,
- floating-point classification and rounding,
- decimal methods,
- string search and trim,
- byte/rune array conversion,
- array and slice minimum members.

## Phase 3

- formatting grammar,
- string splitting iterator,
- Unicode-aware iteration,
- related named-type member substitution,
- generic constrained collection methods.

## Phase 4

- optimized intrinsic lowering,
- target-specific intrinsic selection,
- compile-time format validation,
- complete Unicode case and category support where desired.

---

# 24. Shaped compiler/core boundary

`vector`, `matrix`, `tensor`, and `tensor_view` are compiler-known first-class
language type families. Their intrinsic properties and operations exist because
the compiler-owned registry and `shaped-types.md` define them; they do not
require ordinary privileged core or standard-library `impl` declarations.

The compiler-owned shaped surface includes `Rank`, `Shape`, `Len`, `Strides`,
`Layout`, `MemorySpace`, `IsContiguous`, `Ptr`, `SizeOf`, `Reshape`, `ToShape`,
`Materialize`, `TransferTo`, `Relayout`, `Permute`, `Transpose`, `BroadcastTo`,
`Dot`, `Outer`, `Magnitude`, `Normalize`, `Cross`, `Contract`, and contextual
operator `x` semantics.

Core and standard-library source may provide implementation helpers and
additive numerical algorithms, but cannot redefine these semantics. Ordinary
modules cannot monkey-patch compiler-owned shaped types with privileged global
`impl` blocks.

The shaped metadata types `Shape`, `Strides`, `TensorLayout`, `Axes` and
`AxisList` and their `ShapeError` are compiler-known declarations owned by
`shaped-types.md` (MD-016 part 1); their existence never depends on core stub
helpers, and `ShapeError` has exactly one canonical declaration — an error
union with `InitError(StorageError)` since the MD-016 closure (2026-10-21),
next to the shared `StorageError` enum and the `StorageRequest` /
`ShapedStorageRequest[Rank]` structs of `shaped-types.md` §§ 11, 29.

Legacy lowercase `.len` and `.ptr` examples do not override canonical `.Len`
and `.Ptr` spellings or the existing compatibility migration.

---

# 25. Design summary

The compiler owns representation-sensitive primitives.

The core library owns ordinary behavior.

Built-in types may have privileged core `impl` blocks without introducing
wrapper types.

`self` is implicit in all instance methods.

`.ptr` exists for addressable values and always requires unsafe.

`string` has intrinsic `len` and `ptr`, materializing
`ToByteArray`/`ToRuneArray`, and a minimum ordinary method surface.

`ToString()` exists on fundamental printable types, including `string`.

Named types may use eligible related core members without losing nominal
identity or bypassing contracts.

AST uses ordinary declarations and expressions.

Sema loads a core registry before user modules and resolves core, intrinsic and
user members through one unified member system.

Core contains standard errors required by language-level operations.

This rule defines the minimum required core surface, not the complete final core
library.

# 26. Compiler-known testing context

`testing` is a compiler-known namespace available only while analyzing a
`TestCompilationPlan`. It is not a hidden standard-library import and cannot be
obtained in production source by importing or declaring an unrelated module
named `testing`.

The Sec 0.1 surface—`Pass`, `Fail`, `Skip`, `Log`, `Expect`, `Require`,
`ExpectEqual`, `RequireEqual`, `Run`, and `TestResult`—is defined by
`rules/tooling/testing.md`. The compiler owns the test-context identity and
boundary effects while ordinary value, equality, error, ownership, and cleanup
semantics remain governed by their canonical rulebooks.

# 27. Unit metadata types (MD-062)

The value types of unit metadata reads (`rules/types/units.md`, "Unit
metadata value types") are public `sec/core` declarations
(`rules/corrections/md062-unit-metadata-value-types-correction-20261027.md`
§§ 3, 7, 12(1)–12(2)). They resolve as ordinary nominal types with ordinary
enum equality, `match` and constant evaluation; the compiler registers exactly
these identities and creates no parallel synthetic symbols for them. Each
variant's documentation comment precedes the variant.

## 27.1 `UnitKind`

Kind of quantity. The catalog grows only through deliberate additions to this
declaration; a `Kind:` value may also be a member of any declared user enum.

```sec
enum UnitKind {
    // Kind has not been supplied or otherwise established.
    Unknown

    // A declared miscellaneous quantity without a narrower standard kind.
    Other

    // SI base quantities.
    Length
    Mass
    Time
    ElectricCurrent
    ThermodynamicTemperature
    AmountOfSubstance
    LuminousIntensity

    // General quantities.
    Count
    Ratio
    Probability
    MonetaryAmount
    InformationAmount
    InformationRate

    // Geometry.
    Area
    Volume
    PlaneAngle
    SolidAngle

    // Motion.
    Speed
    Velocity
    Acceleration
    Jerk
    RotationalFrequency
    AngularVelocity
    AngularAcceleration

    // Mechanics.
    Force
    Pressure
    Stress
    MassDensity
    Energy
    Work
    Heat
    Torque
    Power
    ApparentPower
    ReactivePower

    // Time-dependent quantities.
    Frequency
    MassFlowRate
    VolumeFlowRate

    // Electricity and magnetism.
    ElectricCharge
    ElectricPotential
    ElectromotiveForce
    ElectricFieldStrength
    ElectricalResistance
    ElectricalConductance
    ElectricalImpedance
    ElectricalReactance
    Capacitance
    Inductance
    MagneticFieldStrength
    MagneticFlux
    MagneticFluxDensity

    // Thermodynamics.
    Entropy
    HeatCapacity
    SpecificHeatCapacity
    ThermalConductivity

    // Chemistry.
    Concentration
    AmountConcentration
    CatalyticActivity

    // Photometry.
    LuminousFlux
    Illuminance
    Luminance

    // Radiation.
    Radioactivity
    AbsorbedDose
    DoseEquivalent
}
```

## 27.2 `UnitSystem`

Descriptive system metadata. A shared system never proves convertibility.

```sec
enum UnitSystem {
    // No system was explicitly supplied.
    Unspecified

    // An explicitly miscellaneous system.
    Other

    // International System of Units.
    SI

    // Centimetre-gram-second and its electromagnetic variants.
    CGS
    CGSGaussian
    CGSElectrostatic
    CGSElectromagnetic

    // International Electrotechnical Commission conventions.
    IEC

    // Customary and specialist systems.
    Imperial
    USCustomary
    Nautical
    Astronomical
    Natural
    Planck
    Atomic
    Currency
    Domain

    // Historical systems and regional/period measurements.
    Historical
}
```

## 27.3 `UnitOrigin`

`Zero` and `One` are declared coordinate conventions, not proof of a shared
physical datum.

```sec
enum UnitOrigin {
    // No physical/coordinate origin was supplied.
    None

    // A declared zero-origin coordinate convention.
    Zero

    // A declared one-origin coordinate convention.
    One

    // Absolute zero for compatible thermodynamic point quantities.
    AbsoluteZero
}
```

## 27.4 `UnitTransform`, `UnitCategory` and `UnitStatus`

These three are closed: metadata must use their members, never a user enum.
`UnitCategory` is the type of the implicit `<U>.Category`; it adds no
`Category:` metadata key. Historical provenance is `UnitSystem.Historical`,
not a status.

```sec
enum UnitTransform {
    Linear
    Affine
    Logarithmic
}

enum UnitCategory {
    Physical
    Currency
    Information
    Ratio
    Other
}

enum UnitStatus {
    Active
    Deprecated
    Obsolete
}
```

## 27.5 `UnitDimension`

`UnitDimension` is a compiler-provided opaque nominal value type in `sec/core`,
not a user-writable struct. Its complete public Sec 0.1 surface is:

| Member / operation | Signature | Meaning |
| --- | --- | --- |
| `AxisCount` | read-only property `AxisCount: uint` | number of non-zero axes |
| `IsDimensionless` | read-only property `IsDimensionless: bool` | the normalized axis set is empty |
| `==` | `(UnitDimension, UnitDimension) -> bool` | exact normalized-dimension equality |
| `!=` | `(UnitDimension, UnitDimension) -> bool` | negation of `==` |

It has no constructor, literal, default value, setter, index, iterator,
ordering, arithmetic, implicit conversion or special `ToString`. Its private
representation is not published as source, and no illustrative constructor
bodies are declared. Equality semantics, axis identity and runtime
requirements are defined in `rules/types/units.md`, "Unit metadata value
types" (7).
