# Sec Core Library

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
intrinsic functions.

Examples:

```sec
value.ptr
text.len
array.len
slice.len
len(text)
len(array)
len(slice)
```

The public syntax behaves like ordinary member access.

The implementation is compiler-defined.

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

## 1.3 No duplicate wrapper type

A separate `String` type must not be introduced only to provide methods or
associated functions for `string`.

The built-in type itself acts as both:

- the value type,
- the namespace for associated functions.

Examples:

```sec
let text := string.FromByteArray(bytes)
let empty := text.IsEmpty()
```

The implementation must exist only once.

## 1.4 Compiler-known `len`

`len` is a compiler-known core function. It uses ordinary call syntax and is
available without an import:

```sec
fn len(value: string) int
fn len[T](value: T[]) int
fn len[T](value: ref T[]) int
fn len[T](value: ref mut T[]) int
```

The compiler infers `T` from the argument. `len` returns `int` in Sec 0.1 so
that it composes directly with indexes, offsets and positions without an
integer conversion:

```sec
let index := self.pos + offset
if index < 0 || index >= len(self.input) {
    return 0r
}
```

`len` accepts only `string`, an owning array/sequence, or a shared or mutable
slice reference. It evaluates its argument once, does not allocate and does
not consume the argument. Its name is compiler-owned and cannot be redeclared
or overloaded by user code.

The existing intrinsic member properties `string.len`, `array.len` and
`slice.len` remain `uint` in Sec 0.1. Compiler and standard-library code that
performs signed index arithmetic should use the `len(...)` function.

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
string.FromByteArray(bytes)
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
    fn FromByteArray(value: byte[]) string {
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

`string` is a built-in type.

The compiler defines:

- string literal construction,
- string representation,
- string lifetime rules,
- equality and comparison primitives when needed,
- byte length,
- raw byte pointer access,
- indexing and slicing primitives where supported.

## 5.1 Intrinsic members

```sec
property len: uint {
    get
}

unsafe property ptr: RawPtr[byte] {
    get
}
```

`len` is the byte length unless a later language rule explicitly changes it.

The compiler-known `len(text)` function returns that same byte length as an
`int` for index arithmetic.

## 5.2 Required associated functions

```sec
impl string {
    fn FromByteArray(value: byte[]) string
    fn FromRuneArray(value: rune[]) string
}
```

`rune` arrays and rune slice views additionally provide a compiler-known
materializing method:

```sec
let runes: rune[2] := ['A', 'B']
let text := runes.ToString()
```

`ToString()` is available only when the element type is exactly `rune`. It
accepts no arguments, returns `string`, does not mutate or consume the source,
and has the same allocation and encoding behavior as `string.FromRuneArray`.
It is not a general `array.ToString()` formatting operation.

These are the minimum required constructors.

Their ownership and allocation behavior must be defined by the string memory
model.

If construction can fail, the signatures must use `Result`.

Possible final signatures include:

```sec
fn FromByteArray(value: byte[]) Result[string, EncodingError]
fn FromRuneArray(value: rune[]) Result[string, EncodingError]
```

The final choice depends on whether invalid encoding is accepted, replaced or
rejected.

## 5.3 Required instance methods

```sec
impl string {
    fn ToString() string

    fn IsEmpty() bool

    fn Compare(other: string) int
    fn StartsWith(prefix: string) bool
    fn EndsWith(suffix: string) bool
    fn Contains(value: string) bool

    fn IndexOf(value: string) Option[uint]
    fn LastIndexOf(value: string) Option[uint]

    fn Trim() string
    fn TrimStart() string
    fn TrimEnd() string

    fn Split(separator: string) StringSplitIterator

    fn ToByteArray() byte[]
    fn ToRuneArray() rune[]
}
```

This is a minimum surface, not a complete string API.

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
    fn ToString() string {
        return self
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
    fn ToString() string
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

```sec
type property min: T {
    get
}

type property max: T {
    get
}

type property bits: uint {
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
    fn ToString() string
    fn ToString(format: string) Result[string, FormatError]

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
type property min: T {
    get
}

type property max: T {
    get
}

type property bits: uint {
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
    fn ToString() string
    fn ToString(format: string) Result[string, FormatError]

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
type property min: T {
    get
}

type property max: T {
    get
}

type property epsilon: T {
    get
}

type property infinity: T {
    get
}

type property negativeInfinity: T {
    get
}

type property nan: T {
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
    fn ToString() string
    fn ToString(format: string) Result[string, FormatError]

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

property scale: uint8 {
    get
}
```

The internal coefficient representation must not be exposed as a public member
unless the representation becomes a permanent language guarantee.

## 10.2 Required methods

```sec
impl decimal {
    fn ToString() string
    fn ToString(format: string) Result[string, FormatError]

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

---

# 11. Character and rune

The exact distinction between `char` and `rune` is defined by the type rules.

Both are built-in types and support `.ptr` when addressable.

## 11.1 Character

```sec
impl char {
    fn ToString() string

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
    fn ToString() string

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
    fn ToString() string
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

## 13.2 Required shared-slice methods

```sec
impl ref T[] {
    fn ToString() string
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
fn ToString() string
```

Numeric types also provide:

```sec
fn ToString(format: string) Result[string, FormatError]
```

The format-string overload allows concise formatting rules.

Format strings must be validated at compile time when the argument is a
compile-time literal and the format grammar is known.

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
the same value.

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
CustomerID.ToString() -> string
```

Not automatically inherited:

- operations whose parameter uses the raw underlying type,
- operations whose result would silently become the raw underlying type,
- operations that violate a range or unit contract,
- operations that would permit invalid cross-domain arithmetic.

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

enum RangeError {
    InvalidRange
    StartAfterEnd
    OutOfBounds
}

enum IndexError {
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
  returns `int`, is shown by LSP global completion and rejects user
  redeclaration.
- `string.len`, array `.len` and slice `.len` are intrinsic members returning
  `uint`.
- `rune` arrays and rune slice views provide compiler-known `ToString()` text
  materialization, including fixed arrays such as `rune[2]`.
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
- Compiler intrinsic types are registered in semantic-analysis metadata and
  remain compiler-owned rather than declared in core source. This includes
  fundamental builtins (`bool`, integer/float/decimal types, `string`, `void`,
  `never`) and compiler-known generic/runtime-facing types such as `RawPtr[T]`,
  `Option[T]`, `Result[T, E]`, `Task[T]`, `Mutex[T]`, `MutexGuard[T]`,
  `Atomic[T]`, `CompareExchangeResult[T]`, `Event[T]`,
  `EventStorage[T, Capacity]`, `Subscription`, `EventSubscribeResult`, `Arena`
  and `AllocationError`.
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
- `StringSplitIterator` exists as a minimal core type so `string.Split` can
  have a real return type, but iterator behavior is not implemented.

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

- The rulebook specifies `FromByteArray(value: byte[])`,
  `FromRuneArray(value: rune[])`, `ToByteArray() byte[]` and
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

# 24. Design summary

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
