# Temporal values

- **Status:** Normative, phase 1 representation and wall-clock contract
- **Created:** 2026-09-28
- **Last updated:** 2026-10-13
- **Document revision:** 1.2
- **Sec language version:** 0.1
- **Canonical path:** `rules/types/temporal.md`
- **Implementation governance:** `governance/types.yaml`

## Purpose

This rulebook defines the initial storage and trusted-core boundary for Sec's
compiler-known temporal values, the Sec 0.1 UTC wall-clock boundary, and the
conversion of time quantities to `duration` (§5). It does not otherwise define
calendar arithmetic, parsing, formatting, or time zones.

## 1. Compiler-known temporal identities

`date`, `time`, `datetime`, and `duration` are compiler-known nominal value
types. They require no import, are distinct from every numeric and string
type, and do not acquire an implicit default value from their storage.

Only loader-proven `sec/core` source may refine one of these identities with a
source declaration or an `impl` block. The compiler-known identity remains
intrinsic after that refinement. Ordinary source may neither redeclare the
type nor add an `impl` block.

The trusted declaration exposes its backing fields only as module-internal
core implementation detail. Those fields are not part of the public temporal
API and cannot be read, written, or supplied by a module outside `core`.

## 2. Phase-1 value representations

The canonical phase-1 representations are:

| Type | Representation | Invariant |
|---|---|---|
| `duration` | signed `int64` total nanoseconds | A duration is one signed fixed elapsed amount. It has no years or months. |
| `date` | signed `int32` epoch-day count from 1970-01-01 | Negative values represent dates before the epoch. |
| `time` | `uint64` nanoseconds since midnight | The value is in `[0, 86_400_000_000_000)`. |
| `datetime` | signed `int32` epoch days plus `uint64` nanoseconds since midnight | Its time-of-day part satisfies the `time` invariant. |

The selected primitive widths are part of this phase-1 representation contract.
They do not turn any temporal type into a numeric alias and they do not permit
numeric conversion.

Trusted core declares the backing shape with ordinary module-internal fields:

```sec
module core

type duration struct {
    _nanoseconds: int64,
}

type date struct {
    _epochDays: int32,
}

type time struct {
    _nanosecondsSinceMidnight: uint64,
}

type datetime struct {
    _epochDays: int32,
    _nanosecondsSinceMidnight: uint64,
}
```

Core code constructs one of these backing structs with ordinary struct-literal
syntax, for example `date { _epochDays: epochDays }`. `new date { ... }` is not
Sec construction syntax. A core implementation may read its own `_` fields;
callers outside `core` must use future public operations instead.

`duration` is signed because subtraction and negation can produce a value
before zero. `date` is signed because a calendar value can precede the epoch.
`time` is unsigned because it is an offset inside one day and never negative.

## 3. UTC wall-clock access

Sec 0.1 defines these public type-level properties in trusted core:

```sec
impl datetime {
    static property Now: datetime { get }
}

impl date {
    static property Today: date { get }
}

impl time {
    static property Now: time { get }
}
```

`datetime.Now` returns the current UTC wall-clock date and time. `date.Today`
returns the UTC calendar date projected from one wall-clock read, and
`time.Now` returns the UTC time-of-day projected from one wall-clock read. No
local time zone, UTC offset, daylight-saving rule, or other civil-time
conversion is applied.

The three properties are infallible at the Sec API level and return
`datetime`, `date`, and `time` respectively rather than `Result`. Each property
evaluation performs its own wall-clock read; separate evaluations need not
describe the same instant.

The compiler-known identifier `_now` is a privileged core-only value intrinsic
with semantic type `datetime`. Evaluating it reads the current UTC wall clock
and produces a valid canonical `datetime`. It is available only in
loader-proven `sec/core`, has no public declaration, is not importable, and is
not visible through ordinary lookup, completion, or public documentation
outside trusted core. `date.Today` and `time.Now` each project from one `_now`
read performed for that property evaluation.

`_now` requires the `UTCWallClock` target capability in state
`Supported + Enabled` and carries the existing
`MayUseNondeterministicInput` effect. The read must remain an explicit Semantic
IR operation or stable intrinsic identity. It must not be constant-folded,
replaced with compiler-host or build time, replaced with a sentinel, or merged
with another read.

`_now`, `datetime.Now`, `date.Today`, and `time.Now` are invalid in every
`SemanticCompileTimeRequiredContext`, including static initializers. Sec 0.1
defines no deterministic injected clock for compile-time evaluation.

The presence of the core declarations alone does not require `UTCWallClock`;
the capability becomes a program requirement only when a reachable operation
uses `_now`.

## 4. Deferred surface

Beyond the three UTC wall-clock properties in §3 and the conversion of time
quantities to `duration` in §5, this revision defines no public temporal
constructor, component property, formatter, parser, arithmetic operation,
comparison operation, serialization format, time zone, offset, or default
value. Core may add an operation only when a later revision defines its
semantic contract and validation behavior. §5.8 fixes the overflow behavior
any `duration` arithmetic must have.

The UTC wall-clock properties above do not define a monotonic clock.
`Instant.Now` and its acquisition, comparison, and arithmetic semantics remain
deferred and must not be inferred from `_now`.

Calendar-relative quantities such as months and years are not `duration`
components. A later revision may define anchored decomposition on `date` or
`datetime`, or introduce a separate calendar-period value.

## 5. Conversion of time quantities to `duration`

Decided by MD-015 (2026-10-13,
`rules/corrections/applied/md015-time-unit-duration-conversion-correction-20261013.md`).

### 5.1 Time quantities are not durations

**§ 5.1(1)** A numeric quantity whose unit has the dimension of time (see
`rules/types/units.md`, "Quantity literals") is **not** automatically a
`duration`. Conversion to `duration` happens only where it is expressly
requested (`duration(q)`) or where an expected `duration` type provides
context (§ 5.5). Without such context the expression stays a unit-bearing
numeric quantity and takes part in ordinary unit algebra:

```sec
let elapsed := 5<s>                  // unit-bearing numeric quantity
let converted := duration(elapsed)   // explicitly requests duration
let velocity := 100<m> / elapsed     // length/time; elapsed is not a duration
```

**§ 5.1(2)** The conversion is defined once, here. Mutex, Channel, Select,
Task, Context and every other API with a `duration` parameter use it
unchanged and must not invent their own time-unit parsing, unit-name tables,
rounding, or timeout overloads.

### 5.2 Core time units

**§ 5.2(1)** The time units `s`, `ms`, `us` and `ns` belong to the
compiler-known core unit environment and are available without importing the
standard library. They are declared once, in `sec/core/baseunits.sec`:

```sec
unit s physical

impl s {
    LongName: "Second"
    Symbol: "s"
    Kind: time
    Dimension: [time^1]
    Scale: 1
    System: SI
}
```

and likewise `ms` (`Scale: 1 / 1000`, LongName `"Millisecond"`), `us`
(`Scale: 1 / 1000000`, `"Microsecond"`) and `ns` (`Scale: 1 / 1000000000`,
`"Nanosecond"`). All four denote linear time differences, not absolute
calendar or monotonic-clock points. The MD-015 correction writes these
declarations in the `impl <Unit>` spelling of MD-040; MD-040 is still open in
`missing-decisions.yaml`, so the core catalog keeps the current `impl Unit`
spelling until it is integrated.

**§ 5.2(2)** Neither the compiler nor any API may infer that a unit with a
convenient short name is a time quantity. Only the resolved dimension
(`time^1`), Kind (`time` or unknown), linear transform and exact compile-time
scale count. Other time units, including user-defined ones, take part through
the same rules.

### 5.3 `duration` domain

**§ 5.3(1)** `duration` is a signed 64-bit count of nanoseconds (§ 2), range
`-9223372036854775808 ..= 9223372036854775807`. Negative, zero and positive
durations are all valid values:

```sec
let before := duration(-5<s>)
let none := duration(0<ns>)
let after := duration(5<s>)
```

**§ 5.3(2)** An API may impose its own contract, for example a non-negative
timeout. Such a contract is owned by that API's rulebook; it never changes the
domain of `duration` and never silently clamps an argument.

### 5.4 Exact conversion

**§ 5.4(1)** For a time quantity `q`, the exact mathematical value in
seconds is computed through the resolved linear unit transform and multiplied
by `1_000_000_000`:

```text
exactNanoseconds = exactQuantityInSeconds * 1000000000
```

No target-dependent `float32`/`float64` approximation and no intermediate
rounding takes place.

**§ 5.4(2)** Without an explicit rounding mode the conversion succeeds only
when the exact result is an integer inside the `duration` range. There is no
implicit truncation, saturation, wraparound, hidden precision loss or sign
change. Positive and negative quantities follow the same rules.

**§ 5.4(3)** An incompatible dimension, a point (rather than difference)
role, an unresolved or runtime-supplied scale, or a non-linear transform is
never accepted as `duration`; it is an ordinary unit/conversion error.
Specialized affine and logarithmic conversions belong to MD-048.

### 5.5 Explicit and contextual forms

**§ 5.5(1)** The explicit conversion `duration(q)`, initialization of a
declared `duration`, and a call argument for a `duration` parameter are valid
when the conversion is proved exact and in range:

```sec
let explicitValue := duration(5<s>)
let declaredValue: duration := 500<ms>

fn Wait(d: duration) void {
}

Wait(500<ms>)
```

**§ 5.5(2)** A contextual conversion (declaration or argument) is accepted
only when the compiler proves it for the actual value: a compile-time constant
quantity, or every value of a statically known bounded integer domain whose
scale is a whole number of nanoseconds (for example `int16<ms>`).

**§ 5.5(3)** A value whose exactness or range cannot be proved is never
silently converted at a `duration` boundary. The program uses the explicit
conversion, which is then a checked conversion handled by the ordinary
`try`/`Result` rules:

```sec
fn CheckedTimeout(source: decimal<ms>) Result[duration, ConversionError] {
    let timeout := try duration(source)
    return Ok(timeout)
}
```

**§ 5.5(4)** Overload selection still depends on input parameters only
(MD-027). Contextual conversion of a time quantity never resolves an
otherwise ambiguous call and never consults the expected result type.

### 5.6 Explicit rounding

**§ 5.6(1)** When the exact nanosecond count is fractional the default
conversion is invalid. The program may pass a `RoundingMode` as a second
argument:

```sec
let a := duration(500<ms>)                      // 500_000_000 ns
let b := duration(0.5<ns>, RoundingMode.Ceil)   // +1 ns
let c := duration(-0.5<ns>, RoundingMode.Floor) // -1 ns
let invalid := duration(0.5<ns>)                // error: half a nanosecond
```

**§ 5.6(2)** `RoundingMode` is declared once by numeric core
(`sec/core/rounding.sec`), not by `stdlib/math`:

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

Each mode has its mathematical meaning, including for negative values. The
declaration is the one given by the MD-015 correction; MD-047, which owns the
rounding policy of general unit conversions (its source spelling there and
its checked failure), remains open and must reuse this enum rather than
declare another.

**§ 5.6(3)** Rounding happens exactly once, after the full mathematical
unit conversion and before the range check, on the exact nanosecond count —
never on the source quantity or an intermediate carrier. A rounding mode never
permits overflow, an incompatible unit, an undefined scale or any unrelated
conversion error, and no rounding mode is ever chosen implicitly.

### 5.7 Errors

**§ 5.7(1)** A statically provable invalid conversion — inexact without a
rounding mode, an invalid unit relationship, or a result outside the range —
is a compile-time error (S1153 `temporal.invalid-duration-conversion`), as is
an unprovable contextual conversion.

**§ 5.7(2)** A dynamic explicit conversion that can fail is a checked
conversion: it requires `try` (S1139) and fails with the core
`ConversionError` — `ConversionError.OutOfRange` for a result outside the
range and `ConversionError.PrecisionLoss` for a fractional nanosecond count
without a rounding mode.

### 5.8 Duration arithmetic overflow

**§ 5.8(1)** Arithmetic on `duration` values, where defined, follows Sec's
ordinary checked-overflow rules. No duration-specific wrapping, clamping,
saturation or silent loss exists:

```sec
let span := duration(5<s>)
let offset := duration(-2<s>)
let combined := span + offset   // 3 seconds; ordinary checked arithmetic
```

**§ 5.8(2)** An implementation must not introduce an overflowing intermediate
multiplication when the mathematical conversion is representable, and must
not accept an overflowing final result. A backend may use a provably correct
wider intermediate or equivalent checked arithmetic.

**§ 5.8(3)** A target or profile that cannot implement these checks and
results rejects the operation at compile time instead of changing its
temporal meaning.

### 5.9 Concurrency boundaries

**§ 5.9(1)** Concurrency and context APIs keep their canonical `duration`
parameters; there are no unit-specific overloads. `Mutex.Lock(250<ms>)`,
`Context.WithTimeout(5<s>)`, Channel and Select timeouts, and every other
`duration` boundary use the conversion plans and diagnostics of this section,
and resolve unit names through the canonical unit environment.

**§ 5.9(2)** Negative-input policy, zero-timeout behavior, cancellation,
timeout commit ordering and the target timebase belong to each API's own
rulebook. Nothing here declares every negative timeout valid, and nothing
globally forbids negative `duration` values.

