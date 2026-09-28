# Temporal values

- **Status:** Normative, phase 1 representation and wall-clock contract
- **Created:** 2026-09-28
- **Last updated:** 2026-09-28
- **Document revision:** 1.1
- **Sec language version:** 0.1
- **Canonical path:** `rules/types/temporal.md`
- **Implementation governance:** `governance/types.yaml`

## Purpose

This rulebook defines the initial storage and trusted-core boundary for Sec's
compiler-known temporal values and the Sec 0.1 UTC wall-clock boundary. It does
not otherwise define calendar arithmetic, parsing, formatting, or time zones.

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

Beyond the three UTC wall-clock properties in §3, this revision defines no
public temporal constructor, component property, formatter, parser, arithmetic
operation, comparison operation, serialization format, time zone, offset, or
default value. Core may add an operation only when a later revision defines its
semantic contract and validation behavior.

The UTC wall-clock properties above do not define a monotonic clock.
`Instant.Now` and its acquisition, comparison, and arithmetic semantics remain
deferred and must not be inferred from `_now`.

Calendar-relative quantities such as months and years are not `duration`
components. A later revision may define anchored decomposition on `date` or
`datetime`, or introduce a separate calendar-period value.
