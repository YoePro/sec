# Enums

**Status:** Canonical normative rulebook
**Language version:** Sec 0.1
**Document revision:** 1.0
**Created:** 2026-08-13
**Last updated:** 2026-08-13
**Repository baseline reviewed:** `ae227c1`
**Replaces:** the previous enum rulebook revision

## 1. Purpose

This rulebook defines Sec enum declarations, enum domains, underlying representations,
member initialization, `iota`, defaults, aliases, conversions, matching, hardware-backed
enums, semantic analysis, and lowering requirements.

Implementation status does not belong in this rulebook. It is tracked separately.

---

## 2. Core model

An enum is a nominal value type with named members and an integer-compatible underlying
representation.

Sec distinguishes two semantic enum domains:

1. **ordinary enums are closed**;
2. **bit-backed hardware enums are open over their complete bit width**.

The distinction is semantic and must not be inferred merely from machine representation.

---

## 3. Declaration forms

Canonical forms include:

```sec
enum Color {
    RED,
    GREEN,
    BLUE,
}
```

```sec
enum Status int {
    UNKNOWN,
    ACTIVE,
    DISABLED,
}
```

The type-declaration form is also valid:

```sec
type Operation enum {
    ADD,
    SUBTRACT,
}
```

The default underlying type of an ordinary enum is `int`.

An explicit ordinary underlying type must be an integer type.

Hardware-backed enums may use `bit` or `bit[N]`:

```sec
enum AlertPinMode bit[2] {
    ALERT_DISABLED = 0b00,
    COMPARATOR = 0b01,
    INTERRUPT = 0b10,
}
```

The parser may also accept a colon before a bit underlying:

```sec
enum AlertPinMode: bit[2] {
    ALERT_DISABLED = 0b00,
    COMPARATOR = 0b01,
    INTERRUPT = 0b10,
}
```

`bit` is shorthand for `bit[1]` in enum underlying position.

Bit-backed enum widths are `1..256` unless a target rule imposes a narrower supported
implementation range.

An enum must declare at least one member.

---

## 4. Enum members

Enum members are named typed constants in the enum namespace.

```sec
enum Direction {
    NORTH,
    EAST,
    SOUTH,
    WEST,
}

let direction: Direction := Direction.NORTH
```

Member names must be unique within one enum.

Different enums may reuse the same member names.

Members may be separated by commas or line breaks where the canonical grammar permits it.
A trailing comma is allowed.

An enum member has the enum type, not the underlying integer type.

---

## 5. Member initializer syntax

An explicit initializer uses `=`:

```sec
enum HttpStatus int {
    OK = 200,
    NOT_FOUND = 404,
}
```

The initializer must be an integer constant expression representable by the enum's
underlying representation.

`Member: expression` is not canonical enum initializer syntax. A parser may accept it only
as recovery syntax so diagnostics or the formatter can rewrite it to `=`.

---

## 6. `iota`

`iota` is a compile-time integer constant available only while evaluating enum member
initializers.

For each enum declaration:

- the first member has `iota == 0`;
- `iota` increases by one for every following declared member;
- explicit numeric values do not reset or modify `iota`;
- aliases do not modify `iota` progression;
- each enum has an independent `iota` sequence;
- `iota` has the enum's underlying integer type during constant evaluation.

`iota` is not visible outside enum initializer evaluation.

### 6.1 First omitted initializer

If the first member omits its initializer, its implicit initializer expression is `iota`.

Therefore:

```sec
enum Direction {
    NORTH,
    EAST,
    SOUTH,
    WEST,
}
```

is semantically equivalent to:

```sec
enum Direction {
    NORTH = iota,
    EAST,
    SOUTH,
    WEST,
}
```

and resolves to:

```text
NORTH = 0
EAST  = 1
SOUTH = 2
WEST  = 3
```

### 6.2 Omitted initializer repeats the preceding initializer expression

After the first member, an omitted initializer repeats the preceding explicit or implicit
initializer expression and evaluates that expression using the current member's `iota`.

This follows the Go-style `iota` model.

Example:

```sec
enum X int {
    A = iota,      // 0
    B,             // 1
    C,             // 2
    D = 10,        // 10
    E,             // 10: repeats the expression "10"
}
```

An explicit numeric jump does not change `iota` and does not create an implicit
"previous value + 1" sequence.

To jump and then continue from the new sequence, express the jump through `iota`:

```sec
enum X int {
    A = iota,      // 0
    B,             // 1
    C,             // 2
    D = iota + 7,  // 10
    E,             // 11
    F,             // 12
}
```

Programmers normally write `iota` only when establishing or changing the initializer
expression. Repeating `iota` on every member is unnecessary.

Example:

```sec
enum Permission uint {
    NONE = 0,
    READ = 1 << iota,
    WRITE,
    EXECUTE,
}
```

resolves to:

```text
NONE    = 0
READ    = 2
WRITE   = 4
EXECUTE = 8
```

All repeated initializer expressions are checked for overflow and representability after
substitution of the current `iota` value.

---

## 7. Numeric aliases

Multiple member names may have the same numeric value.

```sec
enum ResultCode int {
    SUCCESS = 0,
    OK = 0,
    NOT_FOUND = 404,
    MISSING = 404,
}
```

Duplicate numeric values are valid aliases and must not be rejected.

Alias names remain distinct declared names, but values with the same numeric representation
belong to the same runtime value class for equality and pattern coverage.

---

## 8. Enum defaults

Every valid non-empty enum is defaultable.

The semantic default is selected by member declaration, not by raw underlying zero.

### 8.1 Implicit default

If no member is explicitly marked as default, the first declared member is the enum's
default.

```sec
enum Status int {
    UNKNOWN = 10,
    ACTIVE = 20,
}
```

The default is:

```sec
Status.UNKNOWN
```

The value `0` has no special default status.

### 8.2 Explicit default member

Exactly one member may be marked `default`.

The marker appears after the member name and before an optional initializer:

```sec
enum ConnectionState {
    CONNECTING,
    CONNECTED,
    DISCONNECTED default,
}
```

With an explicit initializer:

```sec
enum ConnectionState int {
    CONNECTING = 10,
    CONNECTED = 20,
    DISCONNECTED default = 30,
}
```

The explicit member overrides the first-member rule.

`default` does not change:

- the member's initializer expression;
- `iota`;
- omitted-initializer repetition;
- alias semantics.

Example:

```sec
enum Mode int {
    OFF = iota,
    IDLE default,
    ACTIVE,
}
```

resolves to:

```text
OFF    = 0
IDLE   = 1
ACTIVE = 2
```

and the semantic default is `Mode.IDLE`.

An alias may be the explicit default member:

```sec
enum ResultCode int {
    SUCCESS = 0,
    OK default = 0,
    FAILED = 1,
}
```

The declared default member is `ResultCode.OK` even though `SUCCESS` has the same runtime
numeric value.

### 8.3 Default initialization

A mutable enum declaration without an initializer uses the enum's semantic default.

```sec
enum Color {
    RED,
    GREEN,
    BLUE,
}

let mut color: Color
```

is semantically equivalent to:

```sec
let mut color: Color := Color.RED
```

This is default initialization, not uninitialized storage.

An immutable binding still requires an explicit initializer:

```sec
let color: Color
```

is invalid under the normal immutable-binding rule.

---

## 9. Ordinary enums are closed

An ordinary enum's semantic value domain consists only of the unique numeric values named by
its declared members.

Example:

```sec
enum Color int {
    RED = 1,
    GREEN = 2,
    BLUE = 3,
}
```

Valid `Color` values are the declared runtime value classes `1`, `2`, and `3`.

The fact that the underlying integer representation can represent other integers does not
make those integers valid `Color` values.

No ordinary enum value may exist outside the declared domain through safe language
semantics.

---

## 10. Bit-backed hardware enums are open

A bit-backed enum is open over the complete representable domain of its bit width.

Example:

```sec
enum AlertPinMode bit[2] {
    ALERT_DISABLED = 0b00,
    COMPARATOR = 0b01,
    INTERRUPT = 0b10,
}
```

The declared members name three semantic encodings, but all values representable by
`bit[2]` remain valid runtime `AlertPinMode` values.

Therefore `0b11` is a valid raw `AlertPinMode` value even though no declared member names it.

A `bit[N]` enum does **not** need to declare all `2^N` possible values.

This open-domain rule exists because hardware can expose reserved, undocumented, future, or
otherwise undeclared bit patterns that Sec must preserve without trapping or fabricating a
member name.

### 10.1 Hardware best practice

Code that consumes an open hardware enum should normally handle undeclared values explicitly.

Canonical example:

```sec
match mode {
    AlertPinMode.ALERT_DISABLED => HandleDisabled()
    AlertPinMode.COMPARATOR => HandleComparator()
    AlertPinMode.INTERRUPT => HandleInterrupt()
    _ => HandleUnknownMode()
}
```

Tooling may warn when code over an open bit-backed enum assumes that declared members are the
complete runtime domain.

A diagnostic should explain that hardware may produce an undeclared encoding and should
suggest an explicit fallback where appropriate.

---

## 11. Register fields and bit-backed enums

A named register field may use a bit-backed enum when the enum width exactly matches the
field width required by the register layout.

```sec
enum AlertPinMode bit[2] {
    ALERT_DISABLED = 0b00,
    COMPARATOR = 0b01,
    INTERRUPT = 0b10,
}

type AlertControl register[8] {
    Mode: AlertPinMode,
    _: bit[6],
}
```

Reading the register field preserves the complete raw `bit[N]` pattern.

An undeclared but representable pattern does not cause a trap and does not make the enum
value invalid.

Ordinary closed enums are not register-field types unless another dedicated hardware rule
explicitly permits them.

Reserved register fields named `_` continue to use `bit` or `bit[N]`.

---

## 12. Conversions

Enums do not implicitly convert to or from integers.

Different enum types do not implicitly convert to each other even when their representations
are identical.

### 12.1 Enum to integer

Explicit enum-to-integer conversion uses normal conversion syntax:

```sec
let raw := int(Color.RED)
```

The numeric value represented by the enum is converted under the normal checked integer
conversion rules.

If the target integer type cannot represent every possible runtime value and the actual value
is not statically known, the normal fallible conversion rules apply.

### 12.2 Integer constant to ordinary closed enum

A compile-time integer constant may convert to a closed ordinary enum only when its numeric
value matches a declared runtime value class.

```sec
enum Color int {
    RED = 1,
    GREEN = 2,
    BLUE = 3,
}

let color := Color(2)
```

is valid and produces `Color.GREEN`'s runtime value class.

This is a compile-time error:

```sec
let color := Color(9)
```

because `9` is not a declared `Color` value.

### 12.3 Runtime integer to ordinary closed enum

A runtime integer-to-ordinary-enum conversion is checked.

When Sema cannot prove that the input is always one of the declared numeric value classes,
`try` is required:

```sec
let color := try Color(raw)
```

The canonical error family is:

```text
EnumValueError.UndeclaredValue
EnumValueError.OutOfRange
```

`OutOfRange` means the input cannot be represented by the enum's underlying representation.

`UndeclaredValue` means the input is representable by the underlying representation but is
not a declared value of the closed ordinary enum.

If compile-time proof establishes that failure is impossible, no runtime check and no
failure-handling syntax are required.

### 12.4 Integer to open bit-backed enum

For an open `bit[N]` enum, any integer value representable by the N-bit unsigned domain is a
valid enum value, whether or not a member names it.

Constant conversion within the bit width is valid:

```sec
enum DeviceMode bit[2] {
    OFF = 0b00,
    ACTIVE = 0b01,
    SLEEP = 0b10,
}

let rawMode := DeviceMode(0b11)
```

The resulting value is valid but unnamed.

A constant outside the bit width is a compile-time error.

A runtime conversion requires `try` unless Sema proves that the source range fits completely
within `0..(2^N - 1)`:

```sec
let mode := try DeviceMode(raw)
```

The runtime failure is `EnumValueError.OutOfRange`.

An undeclared but in-range bit pattern never produces `UndeclaredValue` for an open bit-backed
enum.

### 12.5 Enum to enum

Conversion between different enum types is never implicit.

When required, code converts explicitly through an integer representation and the target
enum's checked conversion semantics:

```sec
let raw := int(SourceMode.ACTIVE)
let target := try TargetMode(raw)
```

The target enum determines whether the numeric value is valid.

---

## 13. Assignment, equality, and arithmetic

Assignment between values of the same enum type follows ordinary value assignment rules.

No implicit assignment exists between:

- an enum and an integer;
- two different enum types.

Equality and inequality are defined between values of the same enum type:

```sec
if color == Color.RED {
    HandleRed()
}
```

Equality compares the semantic numeric enum value. Aliases with the same numeric value are
equal.

Direct arithmetic and ordering are not defined on enum values.

Convert explicitly to an appropriate integer type when numeric arithmetic is genuinely
intended.

---

## 14. `match` and exhaustiveness

Enum patterns match semantic numeric enum values.

Aliases sharing a numeric value cover the same runtime value class.

### 14.1 Ordinary closed enum

For an ordinary closed enum, a `match` is exhaustive when every unique declared runtime
value class is covered by an unguarded pattern, or when an unguarded catch-all is present.

Example:

```sec
enum Direction {
    NORTH,
    EAST,
    SOUTH,
    WEST,
}

match direction {
    Direction.NORTH => MoveNorth()
    Direction.EAST => MoveEast()
    Direction.SOUTH => MoveSouth()
    Direction.WEST => MoveWest()
}
```

No underlying integer values outside those declared members belong to the ordinary enum's
semantic domain.

### 14.2 Open bit-backed enum

For an open `bit[N]` enum, the complete semantic domain contains all `2^N` bit patterns.

Listing only the declared members is not exhaustive when undeclared bit patterns remain.

```sec
match mode {
    AlertPinMode.ALERT_DISABLED => HandleDisabled()
    AlertPinMode.COMPARATOR => HandleComparator()
    AlertPinMode.INTERRUPT => HandleInterrupt()
    _ => HandleUnknownMode()
}
```

A catch-all is not mathematically required only when the unguarded patterns already cover the
complete bit domain.

For hardware-facing code, an explicit fallback remains recommended whenever reserved or
unknown encodings are possible.

---

## 15. `switch`

An enum may be used as a `switch` subject under the ordinary value-comparison rules.

`switch` does not acquire enum-specific exhaustiveness merely because the subject is an enum.

Use `match` when exhaustive enum branching is intended.

---

## 16. Nested enums

Enums may be declared inside an owning `impl` where nested declarations are permitted.

```sec
impl Vehicle {
    enum FuelType {
        PETROL,
        DIESEL,
        ELECTRIC,
    }
}
```

Outside the owning `impl`, the enum type is referenced through the owner:

```sec
Vehicle.FuelType
Vehicle.FuelType.DIESEL
```

Inside `impl Vehicle`, the shorter nested name may be used according to the normal nested-name
scope rules:

```sec
FuelType
FuelType.DIESEL
```

Module-level enum members are accessed through their enum namespace:

```sec
Color.RED
```

---

## 17. Sema requirements

Sema must:

- register every enum as a distinct nominal type;
- resolve the underlying type;
- use `int` when an ordinary underlying type is omitted;
- reject invalid ordinary underlying types;
- preserve exact `bit[N]` width for hardware enums;
- reject bit widths outside the canonical supported language range;
- reject empty enums;
- reject duplicate member names;
- allow duplicate numeric values as aliases;
- evaluate explicit enum initializer expressions at compile time;
- make `iota` available only during enum initializer evaluation;
- assign the current zero-based member index to `iota`;
- treat a missing first initializer as implicit `iota`;
- make later omitted initializers repeat the preceding initializer expression;
- evaluate repeated expressions using the current member's `iota`;
- detect overflow and underlying-representation violations;
- resolve exactly one enum default member;
- reject more than one explicit `default` marker;
- select the first member when no explicit default exists;
- default-initialize mutable enum declarations that omit an initializer;
- distinguish closed ordinary enum domains from open bit-backed domains;
- preserve undeclared in-range bit-backed values;
- validate checked integer-to-enum conversions;
- require `try` when a runtime conversion may fail;
- preserve normal proof-based elimination of impossible runtime checks;
- reject implicit integer/enum and cross-enum conversion;
- validate ordinary-enum `match` coverage against declared value classes;
- validate open bit-enum `match` coverage against the complete bit domain;
- treat numeric aliases as one pattern-coverage class;
- preserve enum nominal identity through lowering metadata.

---

## 18. Diagnostics and tooling

Diagnostics should distinguish at least:

```text
duplicate enum member name
invalid enum underlying type
empty enum declaration
enum initializer is not a constant expression
enum initializer is not representable by the underlying type
multiple enum members marked default
iota is only available inside enum member initialization
integer value is not a declared member of closed enum
integer value does not fit bit-backed enum width
fallible enum conversion requires try
non-exhaustive match for closed enum
non-exhaustive match for open bit-backed enum; undeclared hardware encodings remain possible
```

The LSP should expose in hover information whether an enum is:

```text
closed ordinary enum
open bit-backed enum
```

For an open bit-backed enum, hover or diagnostics should make clear that undeclared bit
patterns are valid runtime values.

When a `match` over an open hardware enum omits an unknown-value fallback while undeclared
bit patterns remain possible, tooling may provide a focused warning or suggestion.

---

## 19. Lowering requirements

An enum lowers through its declared underlying representation while preserving source-level
nominal identity in compiler metadata where needed.

An ordinary enum's closed semantic domain must remain a Sema-level invariant even when its
machine representation can encode additional bit patterns.

A `bit[N]` enum lowers to an exact N-bit unsigned/signless machine representation as required
by the target lowering rules.

Open bit-backed enum reads must preserve all representable N-bit patterns.

Member constants, default resolution, `iota`, repeated initializer expressions, aliases, and
checked constant conversions are resolved before backend constant emission.

Checked runtime conversions must lower to explicit validation and failure control flow when
Sema cannot prove success.

No enum lookup table or runtime reflection facility is implied by this rulebook.

Detailed MLIR operation and lowering contracts belong to `rules/mlir/`.

---

## 20. Canonical summary

```text
ordinary enum
    closed domain
    valid runtime values are declared numeric value classes only

bit-backed enum
    open domain
    every value representable by bit[N] is valid
    declared members name known encodings

member initialization
    first omitted initializer -> implicit iota
    later omitted initializer -> repeat preceding initializer expression
    evaluate repeated expression using current iota

explicit numeric jump
    does not reset iota
    does not imply previous-value-plus-one continuation

default
    first declared member unless one member is marked `default`
    marker syntax: MEMBER default [= expression]
    raw numeric zero has no special default priority

iota
    zero-based declaration position
    independent of resolved numeric values

ordinary integer -> closed enum
    constant: must name a declared value class
    runtime: checked, `try` when failure is possible

integer -> bit-backed enum
    any in-range bit pattern is valid
    runtime: `try` only when width fit is not provable

match
    ordinary enum: declared value classes define exhaustive domain
    bit-backed enum: complete bit[N] domain defines exhaustive domain
    hardware-facing code should handle unknown/reserved encodings
```
