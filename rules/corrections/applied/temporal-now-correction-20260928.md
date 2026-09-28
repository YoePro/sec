# Correction — Temporal wall-clock access

- **Status:** Applied
- **Applied:** 2026-09-28
- **Created:** 2026-09-28
- **Sec language version:** 0.1
- **Primary owning rulebook:** `rules/types/temporal.md`
- **Affected rulebooks:** `rules/types/types.md`, `rules/types/default_values.md`, `rules/compiler/compiler_known_members.md`, `rules/platform/platform_model.md`
- **Archived location:** `rules/corrections/applied/temporal-now-correction-20260928.md`

## 1. Purpose

This correction defines the Sec 0.1 wall-clock boundary required by the core
temporal API.

It introduces no new temporal type and no new effect kind.

The public API is implemented by trusted `sec/core` source. The compiler exposes
only the private core intrinsic required to connect that source to target
wall-clock acquisition.

## 2. `rules/types/temporal.md`

Revise the document to revision 1.1 and replace the statement that no `Now`
operation exists with the following rules.

### 2.1 Public wall-clock properties

Sec 0.1 defines these type-level properties:

```sec
impl datetime {
    static property Now: datetime {
        get
    }
}

impl date {
    static property Today: date {
        get
    }
}

impl time {
    static property Now: time {
        get
    }
}
```

The properties are ordinary public core declarations with language-defined
semantics.

`datetime.Now` returns the current UTC wall-clock date and time.

`date.Today` returns the UTC calendar date corresponding to one wall-clock read.

`time.Now` returns the UTC time-of-day corresponding to one wall-clock read.

Sec 0.1 does not apply a local time zone, UTC offset, daylight-saving rule, or
other civil-time conversion to these properties.

The three properties are infallible at the Sec API level and return `datetime`,
`date`, and `time` respectively. They do not return `Result`.

Each property evaluation performs its own wall-clock read. Separate evaluations
need not describe the same instant.

### 2.2 Private core intrinsic `_now`

The compiler-known identifier:

```text
_now
```

is a privileged core-only value intrinsic with semantic type:

```sec
datetime
```

Evaluating `_now` reads the current UTC wall clock and produces a valid
`datetime` value using the canonical temporal representation.

`_now`:

- is available only while compiling loader-proven `sec/core` source;
- has no public declaration;
- is not importable;
- is not visible through ordinary name lookup, completion, or public API
  documentation outside trusted core;
- is invalid in ordinary user or standard-library source;
- exists only to connect the core temporal implementation to compiler and target
  semantics.

The public `datetime.Now`, `date.Today`, and `time.Now` properties remain the
source-level API. `_now` is not a fourth public temporal operation.

`date.Today` and `time.Now` are semantically projections of a single `_now`
wall-clock read performed for that property evaluation.

## 3. `rules/compiler/compiler_known_members.md`

Register `_now` as a stable privileged core-only compiler-known intrinsic.

The registry entry must define at least:

```text
canonical name: _now
availability: loader-proven sec/core only
result type: datetime
unsafe: false
effect: MayUseNondeterministicInput
required target capability: UTCWallClock
public visibility: none
```

The compiler must preserve the wall-clock read as an explicit Semantic IR
operation or stable intrinsic identity.

A `_now` read must not be:

- constant-folded;
- replaced by compiler-host time;
- replaced by build time;
- replaced by the Unix epoch or another sentinel;
- merged with another `_now` read by common-subexpression elimination.

No new effect kind is introduced. Clock input already belongs to
`MayUseNondeterministicInput`.

## 4. `rules/platform/platform_model.md`

Add the canonical target capability:

```text
UTCWallClock
```

`UTCWallClock` means that the selected target and active `CompilationPlan` can
provide the non-fallible Sec wall-clock operation required by `_now`.

A reachable wall-clock read requires:

```text
Supported + Enabled
```

`Unsupported`, `Disabled`, and an unresolved required capability are compile-time
or CompilationPlan errors according to the existing capability-state rules.

The mere presence of the core property declarations does not require
`UTCWallClock`. The capability becomes a program requirement when a reachable
operation uses `_now`.

A target without `UTCWallClock` may therefore use `date`, `time`, `datetime`, and
`duration` as ordinary values while rejecting code that requires current
wall-clock acquisition.

## 5. `rules/types/default_values.md`

Replace the undecided temporal-default wording with:

```text
date      NonDefaultable
time      NonDefaultable
datetime  NonDefaultable
duration  NonDefaultable
```

Their backing representations do not create implicit defaults.

In particular:

```text
datetime.Now is not a default value.
date.Today is not a default value.
time.Now is not a default value.
```

Default initialization must never read a clock.

## 6. `rules/types/types.md`

Synchronize the temporal-type summary with `rules/types/temporal.md`.

`date`, `time`, `datetime`, and `duration` remain compiler-known nominal value
types. Their representation and wall-clock semantics are owned by
`rules/types/temporal.md`.

Any wording that all temporal operations remain undefined must be narrowed so it
does not contradict the Sec 0.1 `Now` and `Today` properties defined by that
rulebook.

## 7. Compile-time evaluation

No new compile-time-evaluation mechanism is introduced.

The existing prohibition on ambient current time applies to `_now`,
`datetime.Now`, `date.Today`, and `time.Now`.

They are invalid in every `SemanticCompileTimeRequiredContext`, including static
initializers.

Sec 0.1 has no injected clock for CTE or static initialization.

Deterministic injected-clock evaluation is not planned as part of this
correction. If such a feature is ever required, it needs a separate future
normative revision defining its inputs, ownership, reproducibility, and
`CompilationPlan` semantics.

## 8. Superseding rule

The canonical Sec 0.1 rule is:

> Trusted `sec/core` implements `datetime.Now`, `date.Today`, and `time.Now` as
> static UTC wall-clock properties. The compiler provides the private
> core-only `_now: datetime` intrinsic, which requires the `UTCWallClock`
> target capability and carries `MayUseNondeterministicInput`. Temporal types
> remain non-defaultable, current time is unavailable to compile-time
> evaluation, and `_now` is never part of the public Sec API.
