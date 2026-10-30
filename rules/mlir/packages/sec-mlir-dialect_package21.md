# Sec MLIR Program — Package 21: Exact Decimal Semantic Operations

- **Status:** Implementation package — normative decision gates D1–D3 decided 2026-10-28 (MD-066/MD-067/MD-068, `rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md`); implementation pending
- **Created:** 2026-10-16
- **Last updated:** 2026-10-28 (D1–D3 decisions synchronized; superseded recommendations marked)
- **Document revision:** 1.1
- **Sec language version:** 0.1
- **Canonical repository path (proposed):** `rules/mlir/packages/sec-mlir-dialect_package21.md`
- **Package:** `SEC-MLIR-P21`
- **Reviewed repository:** `YoePro/sec`, `main`, October 16, 2026
- **Repository SHA:** Not independently verifiable through the accessible GitHub view; record exact SHA at implementation time.
- **Predecessor:** `SEC-MLIR-P20` (schema 16 specification)

**Governance:** Do not claim that writing this package implements any compiler feature. Normative rulebooks are authoritative; governance records implementation state.

**Important:** The October language rules supersede incompatible August assumptions; the included P20 audit is mandatory.

## § 1. Purpose and execution order

P21 is a bounded implementation package for exact `decimal` and `decimal128` through Sema → canonical Semantic IR → high-level Sec MLIR → checked exact integer/decimal helper lowering. Execute **one numbered implementation stage at a time**, add focused positive/negative tests, run the appropriate focused test suite, and report failure before continuing. Do not combine unrelated stages or fill specification gaps with compiler guesses.

**Stages:** (1) baseline and corrections; (2) exact decimal constant/representation; (3) resolved operator plans; (4) arithmetic and failure CFG; (5) comparison/core operations; (6) conversion/rounding; (7) schema 17 dialect/verifiers; (8) lowering/verifiers; (9) regression/target matrix/governance.

## § 2. Canonical source hierarchy

Use current repository paths rather than August paths:

```text
rules/types/types.md
rules/foundations/operators.md
rules/memory/layout.md
rules/library/core-library.md
rules/compiler/compiler_known_members.md
rules/compiler/semantic_ir.md
rules/errors/errorhandling.md
rules/errors/runtime_checks.md
rules/types/units.md
rules/mlir/sec_mlir.md
rules/mlir/sec_mlir_dialect.md
rules/mlir/sec_mlir_lowering.md
rules/mlir/packages/
rules/mlir/normative-sync/
rules/mlir/semantic-ir/
rules/mlir/dialect-versions/
rules/mlir/lowering-versions/
governance/
```

Do not overwrite the canonical `sec_mlir_dialect.md` or `sec_mlir_lowering.md` with an old snapshot. Apply revisions preserving existing numbered paragraph identities. Place old snapshots under version-history directories according to repository governance.

## § 3. Observed implementation facts

October `main` includes `internal/sema/constants.go` with `decimalLiteralValue` using a signed `int64` coefficient and `uint8` scale, and a public compiler-known Decimal `Scale` entry returning `int` (`internal/sema/compiler_known_members.go`). Both are incompatible with full wide-decimal support / the canonical `decimal.scale: uint8` core declaration (canonical since 2026-10-28: `Scale: int32` with `try set`, MD-066). The existing `sec/core/decimal.sec` exposes an unfinished `ToString() string` stub, contrary to the canonical fallible `Result[string,StringError]` core surface. Treat as identified bugs, not as alternate language rules.

`sec/core/rounding.sec` already defines the six-case `RoundingMode`. The actual canonical Sec MLIR dialect and lowering rulebooks still identify schema/lowering **11**; uploaded P20 packages specify **16**. P21's **17** is the *next package target*, not a claim that schema 16 is currently emitted.

## § 4. Active language types

`decimal` and `decimal128` are both **ACTIVE builtin scalar types**. Neither is an alias for binary float, and both are trivially copyable / trivially destructible. Wide `int128`, `int256`, `uint128`, `uint256` remain active throughout conversions. For precision/representation restrictions, report a stage-specific limitation; never downgrade a source type to “planned.”

## § 5. Fixed representation boundary

The canonical native component representations are:

```text
decimal    : { coefficient: signed i64,  scale: signed i32 }
decimal128 : { coefficient: signed i128, scale: signed i32 }
```

Those **physical** types are not an invitation to expose coefficient access. P21 keeps a distinct high-level `!sec.decimal` / `!sec.decimal128` throughout semantic lowering and uses canonical plan-resolved composite layout only at its dedicated physical representation boundary. No IEEE decimal128/Densely Packed Decimal representation and no float emulation.

## § 6. Semantic numeric value

Represent the exact mathematical value conceptually as `coefficient × 10^(-scale)`. Static mathematical evaluation uses arbitrary-precision integers/rationals and bounded resource budgets. Numeric equality and ordering must compare mathematical values, not the bitwise fields or stored scale. A public `scale` observation is a separate semantic concern (see D1). Do not pick a canonicalization policy by accident.

## § 7. Literal grammar

The `m` suffix chooses the exact decimal family, not a fixed width. Unsuffixed fractional/scientific literal without a stronger context defaults to `decimal`; context may shape directly to `decimal128`:

```sec
let a := 3.14
let b := 1.25e-3m
let wide: decimal128 := 8m
let sixteen := 0x10m
```

Preserve source lexeme, radix, exponent, sign, and resolved target. Parse directly to exact coefficient/exponent; never via binary float or host `float64`. Integers in base-prefixed `m` forms denote exact integer values, not hexadecimal floating syntax.

## § 8. Literal precision

For static decimal literals, implement representation checks using arbitrary-precision coefficient, including 128-bit target coefficient. No frontend `strconv.ParseInt(..., 64)` may be the gate for `decimal128`. Oversized source exponent must produce bounded deterministic diagnostics, not unbounded `strings.Repeat` allocation or host-int overflow. Keep negative-zero source spelling provenance if used for diagnostics; the integer coefficient/scale value itself has mathematical zero.

## § 9. Existing numeric conversion framework

P20 introduces `NumericConversionError` (`OutOfRange`, `PrecisionLoss`, `NotFinite`), `NumericConvertExact`, `NumericConvertChecked`, and `try`/handler integration. Reuse that family for **typed-value conversions**, but check its actual core declaration/status before implementation. Do **not** assume package-file presence means production support. Do not create another generic `ConversionError` silently.

## § 10. October rounding contract

The single canonical `RoundingMode` is in `sec/core/rounding.sec`: `NearestEven`, `NearestAwayFromZero`, `TowardZero`, `AwayFromZero`, `Ceil`, `Floor`. Exact typed conversions use `T(value)` and explicitly rounded conversions use `T(value, mode)` when defined. Apply mode once to the **exact mathematical value**; no hidden rounding, binary-float intermediate, unit transform after rounding, or second rounding. Existing named-type/unit checks still apply.

## § 11. Existing public decimal core

The verified `rules/library/core-library.md` §10 specifies for `decimal`: unsafe `.ptr`, instance `.scale: uint8`, fallible `Abs() -> Result[decimal,OverflowError]`, `Min/Max`, `Clamp(...) -> Result[decimal,RangeError]`, `Floor/Ceiling/Truncate/Round`, `RoundTo(scale: uint8) -> Result[decimal,PrecisionError]`, `Rescale(scale: uint8) -> Result[decimal,PrecisionError]`, `IsInteger`, `IsZero`, `Sign`, and `ToString` overloads returning `Result[string,StringError]`.

There is **no separately complete `decimal128` member listing** in that section. P21 must not silently fabricate the public `decimal128` method signatures; the shared operator and conversion semantics are covered, and decimal128 member parity is explicitly D3.

*Updated 2026-10-28 (MD-066/MD-068):* `core-library.md` §§ 10.1–10.7 now define the complete surface: `Scale: int32` with getter and `try set` reporting `PrecisionError`; properties `IsInteger`, `IsZero`, `IsPositive`, `IsNegative`, `Sign`; `RoundTo(int32)`, `RoundTo(int32, RoundingMode)`, `Rescale(int32)`, `Normalize()`; `ToInt*`/`ToFloat*`; three `ToString` overloads; and the mirrored `decimal128` inventory in § 10.4. The `uint8` signatures above are historical.

## § 12. Decisions requiring normative resolution

The October rules **do not yet specify** (D1) observable result-scale selection/canonicalization for every operator; (D2) one exact error type for decimal operators that can fail by both arithmetic and precision causes; or (D3) an exact complete decimal128 public method surface. The companion synchronization presents concrete recommendations for approval. Do not mark these already decided, invent member shapes, or implement P21 operator behavior requiring them before the rules are accepted. Independent literal/type/representation/checked-conversion infrastructure may proceed.

*Resolved 2026-10-28:* all three gates are decided by `rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md` and synchronized into their canonical owners. §§ 13–15 below keep the former recommendations only as history, each followed by the decision that supersedes it. Implementation remains pending and follows the correction's § 13(3) step order.

## § 13. D1 recommended scale policy (decided 2026-10-28, MD-066)

**Decision (supersedes the recommendation below):** the public scale is `int32` over its whole signed interval, negative scales included. Each operation has a preferred scale (add/sub/div/rem: maximum input scale; multiply: the sum), and a nonzero exact result takes the exact scale closest to it within the real asymmetric coefficient domain; exact zero keeps the preferred scale clamped to `int32`. Division alone may round: an inexact quotient rounds once with nearest-even at the finest admissible scale, keeping its whole-number part. A provably zero decimal divisor is a compile-time error. Owners: `types.md` "Decimal scale"; `operators.md` "Decimal arithmetic".

Former recommended rule set (superseded):

- Preserve source literal scale when representable; normalize leading exponent by exact powers of ten only.
- For add/sub/remainder, prefer `max(left.scale, right.scale)`; for multiply, prefer the sum of operand scales.
- Use arbitrary-precision intermediates, then remove *only exact trailing coefficient zeros* when necessary to fit coefficient/scale. Preserve the preferred scale when it fits; scale must not be altered casually because `.scale` is observable.
- For exact division, prefer at least the maximum input scale and choose the **smallest** representable nonnegative result scale at or above that preference. If preference is impossible, consider exact trailing-zero reduction under the chosen canonical rule, not rounding.
- Define canonical zero scale per operation explicitly: recommended preserve the preferred scale if valid; literal zero preserves source scale.
- Fix public `scale` range at `0..255` only if confirmed consistent with the `uint8` surface; physical i32 scale is not automatically a larger language domain.

The companion sync marks every item PROPOSED rather than settled.

## § 14. D2 recommended decimal operator error (decided 2026-10-28, MD-067)

**Decision (supersedes the proposal below):** no `DecimalArithmeticError`. Decimal operators reuse `core::ArithmeticError`, which gains exactly `ScaleOverflow` and `PrecisionExhausted` (five variants). The most specific applicable variant is chosen by mathematical cause, not check order; a rounded quotient is no failure; `PrecisionLoss` is not an operator error. Owners: `core-library.md` § 20.1; `errorhandling.md` § 3.4; `operators.md` "Decimal arithmetic failures".

Former proposed single core enum (rejected):

```sec
enum DecimalArithmeticError error {
    Overflow
    DivisionByZero
    PrecisionLoss
    ScaleOverflow
    PrecisionExhausted
}
```

A single explicit error identity supports `try left / right` even when failures have multiple categories; no inferred union. Core methods retain their already declared `OverflowError` / `PrecisionError` signatures. Until D2 is approved, do not substitute `ArithmeticError.Overflow` for precision loss or create implicit wrappers. Error priority proposal: division-by-zero → scale impossibility → coefficient overflow → nonterminating/precision loss, with deterministic tests.

## § 15. D3 recommended decimal128 parity (decided 2026-10-28, MD-068)

**Decision:** `decimal128` mirrors the complete `decimal` member family, including `Scale: int32` with `try set` (`core-library.md` § 10.4). `Parse()` and `ToUInt*` are deferred for both types.

Former recommendation: `decimal128` core methods mirror the proven decimal methods, replacing `decimal` in return/argument type positions. `scale` remains `uint8` only if D1 confirms its range; `Abs()` and precision-specific method error types retain the narrower existing types. Copy method comments and stable identities into `sec/core/decimal128.sec` and compiler-known member registry **after** approval. This proposal does not make a file already exist.

## § 16. Arithmetic operators in scope

Once D1/D2 are fixed, implement unary `+`, unary `-`, `+`, `-`, `*`, `/`, `%`, and Sema-resolved compound forms. No numeric promotion between typed `decimal` and `decimal128` is implicit. Untyped literal shaping may supply the other operand of a resolved decimal type. Check left-to-right evaluation exactly once. The operation's nominal result type is Sema-resolved, not chosen by lowering.

## § 17. Exact arithmetic algorithms

Use arbitrary-precision integer/rational computation **before** rounding to target representation. Add/sub align powers of ten; multiply coefficients and accumulate scales; division computes reduced rational quotient and detects whether the reduced denominator contains primes other than 2 or 5; remainder uses `q=trunc(a/b)` and `r=a-q*b` with integer/rational exactness, never the rounded quotient. Validation of representability follows the approved D1 policy: an inexact quotient is rounded once (nearest-even, finest admissible scale) and succeeds, while `+`, `-`, `*`, `%` stay exact or fail. Never implement `%` by a nearest-integer remainder helper. Do not use i64/i128 overflow as intermediate control flow.

## § 18. Checked decimal failures

Every dynamic arithmetic operation carries a total checked high-level result and explicit reason. On ordinary failure follow the existing deterministic panic/check endpoint; on fallible `try` failure construct the *approved* exact operator error type. Constant-provable failure follows the current warnings/diagnostics policy of its operation and must **not** silently be rewritten into a runtime success or rounded value. *MD-066 (2026-10-28):* a provably zero decimal divisor is a compile-time error even under `try`, and compilation stops; a successfully rounded quotient is not a failure and sets no failure flag. Do not assume the old August statement “every constant error is a fatal compile-time diagnostic”: read the current `operators.md` and runtime-check policy.

## § 19. Result and handler integration

Reuse P9/P10/P11 typed error/Result and P12 match-CFG mechanisms. No new general handler dialect. Only the success candidate may be consumed; error reason is consumed on the failure path. Naked propagation requires an exact compatible error type. Any application error mapping remains an explicit source handler. Ownership/cleanup across both paths reuses P17.

## § 20. Comparisons and ordering

Implement `==`, `!=`, `<`, `<=`, `>`, `>=` using exact rational/decimal numeric comparison. Scale alone does not change equality (`1.2` equals `1.20`). No conversion through `f32`, `f64`, `arith.cmpf`, or target host decimal environment. Comparable named/unit-bearing decimal values retain their source nominal/unit conversion and contract semantics.

## § 21. Decimal core methods

Implement already-declared `decimal` methods through compiler-known resolved IDs. `.Scale` returns exactly `int32` (D1, MD-066), not `int` or `uint8`, and has a transactional fallible setter. `Abs()` checks minimum representable signed coefficient and retains `OverflowError` (not the proposed operator error). `Min`, `Max`, `Clamp` operate on numeric order, not physical scale. `IsInteger`, `IsZero`, and `Sign` are total and allocation-free. `Floor`, `Ceiling`, `Truncate`, and `Round` must use exact coefficient/scale arithmetic. `RoundTo`/`Rescale` preserve their declared `PrecisionError` and require D1's exact scale/rounding contract before full lowering.

## § 22. RoundTo versus Rescale proposal

Recommended: `Rescale(target)` changes representation **exactly** or returns `PrecisionError.PrecisionLoss` / `PrecisionExhausted` / `ScaleOverflow` according to the approved rule; it never rounds. `RoundTo(target)` is an explicitly named rounding operation using `RoundingMode.NearestEven` unless a different policy is approved; rounding occurs once on exact value. `Round()` to integer uses nearest-even; `Floor/Ceiling/Truncate` use named directions. Do not silently add `RoundingMode` parameters to those **already exact public signatures** without a separate API decision. *Decided 2026-10-28 (MD-066 § 8):* `Rescale` is exact and never rounds; `RoundTo(scale)` uses nearest-even and the approved `RoundTo(scale, mode)` overload applies the given mode once; `Floor`/`Ceiling`/`Truncate`/`Round` stay parameterless.

## § 23. Explicit numeric conversions

Implement checked exact `decimal ↔ decimal128`, `int/uint ↔ decimal` for all active widths, `decimal ↔ binary float`, and `decimal ↔ bool` subject to existing source-valid conversion directions. When a mode argument is legal, compute exact rational first and apply the chosen mode once. `decimal ↔ float` may require high-precision rational evaluation (decompose binary significand/exponent exactly), never binary-float fallback for decimal evaluation. No implicit typed promotion. Conversion errors use P20 `NumericConversionError`, with `NotFinite` for nonfinite binary-float sources.

## § 24. Units and named constraints

Decimal carrier values may be unit-bearing (`decimal<Unit>`) and may underlie constrained nominal types. P21 consumes Sema's resolved unit transformation and numeric conversion plan; it does not reimplement dimension/Kind checking. Source `T<U>(value, mode)` performs exact mathematical unit conversion then one rounding. Named type construction must still validate its contract after conversion. Nothing in P21 invents an implicit unit or contract bypass.

## § 25. Representation and identity

Semantic types remain distinct `DecimalType` and `Decimal128Type`; compiler-known member IDs are exact and LSP-visible. A storage-backed decimal value is safe only when coefficient/scale satisfy accepted invariants. An unsafe/FFI produced invalid raw scale must be validated before it enters ordinary safe decimal operations. Physical layout follows `rules/memory/layout.md` under CompilationPlan; do not hardcode composite offsets, target alignment or ABI pass classes.

## § 26. Sema read-only facts

Introduce/reuse `ResolvedDecimalLiteralPlan`, `ResolvedDecimalOperatorPlan`, `ResolvedDecimalConversionPlan`, `ResolvedDecimalMemberPlan` and `DecimalRepresentationPlan` as immutable, read-only queries. Each carries source kind, resolved target, exact constant/lexeme, approved result-scale policy ID, allowed error type, proof/guard classification, source locations and target/layout reference. The Semantic IR builder must not reparses AST tokens or recompute the type/scale policy.

## § 27. Constant representation

Recommended implementation-only exact constant:

```go
type ExactDecimalConstant struct {
    Coefficient *big.Int
    Scale       int64
    Lexeme      string
    TargetKind  string
}
```

Treat fields as immutable on publication (clone all `big.Int`). Scale may temporarily be wider during exact parsing; the final representability check uses the **approved semantic scale domain**. Never interpret an arbitrary `big.Int` as a direct i128 before range validation.

## § 28. Decimal checked operation family

Semantic IR:

```text
DecimalConstOp
DecimalUnaryOp
DecimalBinaryCheckedOp
DecimalCompareOp
DecimalScaleOp
DecimalCoreOp
DecimalRoundToCheckedOp
DecimalRescaleCheckedOp
DecimalConvertExactOp
DecimalConvertCheckedOp
DecimalErrorFromReasonOp
```

`DecimalBinaryCheckedOp` / checked conversions follow `(candidate, failed:i1, reason)` with a fixed reason domain only **after D2**. Candidate must not be consumed on failure. Avoid physically constructing the `Result` for every checked arithmetic operation: reuse existing guard CFG and Result/try handlers.

## § 29. Dialect schema 17

Schema 17 **extends** the P20 schema-16 package and does not claim the canonical repository dialect already emits it:

```text
!sec.decimal
!sec.decimal128
!sec.decimal_failure_reason          // only after D2
sec.decimal.constant
sec.decimal.unary
sec.decimal.binary_checked
sec.decimal.cmp
sec.decimal.scale
sec.decimal.core
sec.decimal.round_to_checked
sec.decimal.rescale_checked
sec.decimal.convert_exact
sec.decimal.convert_checked
sec.decimal.error_from_reason       // only after D2
```

Types `!sec.decimal` and `!sec.decimal128` already appear in earlier dialect design; **do not redefine them**. This list is a v17 surface delta: verify existing declarations and add only missing ops/attrs.

## § 30. High-level lowering boundary

P21 lowers exact decimal arithmetic through a total, checked, target-independent integer/scaling plan or a verified helper contract. Physical representation is the canonical `{i64,i32}` / `{i128,i32}` once materialized; intermediate precision may be much wider than coefficient size. No conversion to `arith.addf` / `math.*f`, `double`, `long double`, or host printf. No dependence on undefined integer overflow, poison, `nsw/nuw` assumptions, or accidental integer truncation.

## § 31. Decimal physical materialization is separate

Introduce `--sec-lower-decimal-core` as a verified **intermediate** lowering pass that preserves semantic error/scale proofs. If required integer precision or fixed helper implementations are unavailable on a target, leave verified high-level decimal op for a later approved pass or fail with target capability diagnostic. Do not claim full LLVM completion. Layout materialization must preserve signed coefficient, scale, source scalar kind, and normal ABI/layout guards.

## § 32. Helper contract

Any decimal runtime helper is a **compiler-controlled, versioned, pure/fallible numerical helper**, not a mandatory global runtime. It must accept explicit exact types/target/layout flags, return a typed checked outcome, never allocate silently, never inspect ambient locale/rounding mode, and be independently differential-tested against an arbitrary-precision reference evaluator. It must not choose a scale or error reason omitted by Sema.

## § 33. Verification passes

Register:

```text
--sec-verify-decimal-values
--sec-verify-decimal-guards
--sec-verify-decimal-conversions
--sec-lower-decimal-core
```

Existing P9/P10/P11 guard/Result verifiers remain. Verifiers reject unknown policy IDs, out-of-range constant coefficients/scales, mismatched error nominal identities, unwarranted rounding, unguarded result candidate, wrong success/failure polarity, invalid operation over nominal/unit types, and loss of `decimal128` identity.

## § 34. Effects and ownership

Decimal arithmetic, comparisons and explicit conversion are intrinsically allocation-free, with checked error-control effects only. Scalar decimal values are `CopyTrivial` and trivially destructible. Formatting/`ToString` requires the separate StringError/allocation materialization contract and is **not implemented** by P21. No hidden allocation context argument is introduced into pure decimal arithmetic.

## § 35. Test: literal and lexical

Positive: unsuffixed `3.14`; `.5m`; `1e3m`; `0x10m`; `decimal128` literal > i64; max/min signed i64/i128 coefficient; scale 0, 1, 255 when allowed; zeros/trailing zeros; underscores; negative literals. Negative: coefficient just beyond width, scale outside approved domain, huge exponent without blowup, old `d` suffix migration diagnostic, binary-float intermediate detection. Test exact lexeme → constant → IR round-trip.

## § 36. Test: arithmetic and comparison

Operator matrix on decimal and decimal128: +, -, *, /, %, unary ±, compound forms; zero divisor; signed negative remainders; terminating vs nonterminating quotient (`1/2`, `1/4`, `1/3`); different operand scales; same numeric value different scales; coefficient i64/i128 boundaries; sum/product preferred-scale overflow; scale overflow; no automatic rounding outside division; rounded division (`1/3`, `2/3`, `1000000000/3`) and exact `1/8` with scale 3; constant/dynamic parity. Failure classification follows D2 (MD-067): `ScaleOverflow`, `PrecisionExhausted`, `Overflow`, `DivisionByZero`; see the DEC-01–DEC-22 matrix in the correction § 14.

## § 37. Test: core decimal

`.Scale` exact `int32` property with transactional `try set`; numeric min/max/Clamp; Abs of signed minimum coefficient; Floor/Ceiling/Truncate/Round for signs/ties; RoundTo/Rescale with exact, inexact and impossible requested scales; IsZero/IsInteger/Sign; no spontaneous ToString materialization. Test `decimal128` public member parity only after D3 is accepted.

## § 38. Test: conversion

All active int/uint widths, including 128/256; signed extrema; narrowing decimal128→decimal; exact decimal↔float32/float64 with NaN/Infinity error; negative zero as applicable; exact fraction vs explicit rounding modes; mode argument `NearestEven`, `NearestAwayFromZero`, `TowardZero`, `AwayFromZero`, `Ceil`, `Floor`; half ties both signs; no automatic rounding; units and named contract postcheck. P20 float target width on 32/64 must follow MD-014 rather than older P20 hardcoded f64 assumption.

## § 39. Test: CFG, semantic/MLIR negative

Positive canonical `(candidate, failed, reason)` failure/try/local-handler CFG; fail path does not use candidate; one evaluation each operand; no hidden Result union; no implicit decimal→binary-float; no unverified i64/i128 narrowing; no duplicate/missing operation registration; P20 schema 16 regression. Negative MLIR: swapped branch polarity, invalid scalar type, unsupported scale, cross-kind operation, fabricated policy, missing source provenance, use-on-failure, reason/flag mismatch, helper mismatch.

## § 40. Test: target, profiles and regression

Run 32-bit and 64-bit CompilationPlans; validate target layout offsets/alignment; ensure decimal128 cannot be truncated by host or direct-LLVM path; check noalloc/bare-metal pure operations; test large constants against CPU/time/memory budgets. Run exact installed `check-sec-mlir`/CMake and `go test ./...` commands, capture full versions and outputs. Do not claim tests passed unless actually executed. Keep legacy lowering operational or give explicit unsupported diagnostics.

## § 41. Acceptance gate A: already canonical

Completion requirements independent of D1–D3:

```text
[ ] current repo SHA recorded by implementer
[ ] canonical new-path rulebooks read
[ ] P20 schema-16 prerequisites verified or missing items reported
[ ] decimal and decimal128 active semantic types retained
[ ] arbitrary-precision parsing/constant representation
[ ] i64 and i128 representability checks
[ ] no binary-float intermediate
[ ] immutable resolved Sema plans
[ ] P20 checked conversion mechanism reused
[ ] 6-case RoundingMode reused
[ ] exact numeric comparison
[ ] verified high-level decimal constant/type printer
[ ] public decimal scale type bug corrected after D1 domain confirmed
[ ] legacy decimal ToString stub diagnosed, not adopted as norm
[ ] source and MLIR regression tests
[ ] governance fragments updated without duplicating historical entries
```

## § 42. Acceptance gate B: normative approvals

Only after D1–D3 have been explicitly accepted and synchronized:

```text
[ ] exact arithmetic scale algorithm implemented
[ ] source-visible .scale policy and target domain locked
[ ] ArithmeticError ScaleOverflow/PrecisionExhausted declared and mapped (D2: no DecimalArithmeticError)
[ ] checked decimal arithmetic and reasons implemented
[ ] RoundTo/Rescale result scale and error behavior implemented
[ ] decimal128 exact public core members decided and registered
[ ] schema 17 types/ops/pass/verifiers installed
[ ] checked helper/back-end boundary tested
[ ] no implicit fallback or source API inventing
```

An unresolved gate is recorded as **blocked by normative decision**, not as “implemented.”

## § 43. Implementation report contract

Report, in order: repository HEAD SHA; dated source inventory; P20 compatibility status; D1–D3 approval IDs; files touched; original and fixed Sema constant implementation; i128 coverage; exact arithmetic strategy; scale policy and results; error type/source declarations; core member registry parity; conversion plan and rounding-mode integration; IR/dialect version; MLIR round trips; compiler passes; guard verifiers; target plan matrix; tests/run commands and actual outputs; governance changes; remaining unsupported stages; next-package recommendation. One line per test stage plus concise failures is sufficient; never present checklist intentions as test results.

## § 44. Beyond P21

P22 should be selected using the current October backlog, not August package projections. Good candidates remain contracts/runtime checks and physical aggregate/decimal-layout lowering, but do not decide a future package's exact scope from this proposal alone.
