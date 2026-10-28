# SEC-MLIR Package 21 — implementation TODO

Package: `SEC-MLIR-P21` — Exact Decimal Semantic Operations  
Primary rulebook: `rules/mlir/packages/sec-mlir-dialect_package21.md`  
Package contract: `rules/mlir/packages/sec-mlir-dialect_package21.yaml`

Normative inputs:

- `rules/mlir/normative-sync/sec_decimal_numeric_sync_package21.md`
- `rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md`
- `rules/mlir/normative-sync/sec_p20_p21_conversion_compatibility_package22.md`
- `rules/mlir/semantic-ir/sec_semantic_ir_decimal_package21.md`
- `rules/mlir/dialect-versions/sec_mlir_dialect_v17.md`
- `rules/mlir/lowering-versions/sec_mlir_lowering_v17.md`

This is an implementation plan, not a completion report or a language
decision. It follows P20 and preserves the ownership of all incomplete P15–P20
work. D1–D3 are normative gates: their dependent implementation must remain
blocked until an approved rulebook decision exists.

## Inventory snapshot — 2026-10-18

Audited HEAD:

```text
831f6e1
```

The working tree has 137 changed entries. This inventory includes those changes
as well as HEAD; capture their exact intended snapshot and untracked inventory
before acceptance work starts.

| Area | Current implementation | P21 consequence |
|---|---|---|
| Active types | `decimal` and `decimal128` are active Sema and Semantic IR kinds; wide integer types remain active. | Preserve both decimal kinds end-to-end; do not demote `decimal128` to planned or emulate either through binary float. |
| Exact literals | Sema parses decimal values with `big.Int` coefficients and width checks; Semantic IR retains `DecimalConstant` coefficient, scale and lexeme. | Credit exact constant infrastructure, then make publication immutable and complete bounded exponent/scale diagnostics. |
| Scale surface | Decimal `Scale` is a read-only `uint8` compiler-known property, with a regression test; `decimal128` members remain withheld. | The old package-YAML `int` bug is already corrected. Since MD-066/MD-068 (2026-10-28) the canonical surface is `Scale: int32` with getter and `try set` on both `decimal` and `decimal128`; the `uint8` property is now a divergence to correct (P21-17, P21-19). |
| Existing semantics | Sema resolves decimal arithmetic, truncation remainder, comparisons, unit carriers, defaults and some exact constant evaluation. | These are frontend facts, not proof of approved checked arithmetic, D1 scale policy, D2 error CFG, schema 17, or native lowering. |
| Existing IR/emission | `TypeDecimal`, `TypeDecimal128` and `const.decimal` emit high-level types and constants. | Reuse them; add P21 operations only from immutable resolved plans. |
| Core string surface | `sec/core/decimal.sec` declares fallible `ToString() Result[string, StringError]`, but its body is a stub. | Retain the correct signature; string materialization remains outside P21. |
| Track and schema | The one native track is authoritative; the current canonical schema is still below P21's target schema 17. | P21 cannot claim v17 emission until P20 schema-16 and intermediate package work is reconciled. |

Primary implementation anchors:

```text
internal/sema/constants.go
internal/sema/analyzer_inference.go
internal/sema/compiler_known_members.go
internal/sema/constant/numeric.go
internal/sema/rounded_conversion.go
internal/ir/semantic/model.go
internal/ir/semantic/builder.go
internal/ir/semantic/verifier.go
internal/lowering/secmlir/emitter.go
sec/core/decimal.sec
sec/core/rounding.sec
```

## Authority, corrections and decision gates

1. **Exact public conversion error:** the P20/P21 compatibility note maps old
   `NumericConversionError` references to the existing public
   `ConversionError`: `OutOfRange`, `PrecisionLoss` and `NonFinite` as
   applicable. Do not declare a duplicate numeric-conversion error type.
2. **Target-sized float:** MD-014 makes plain `float` binary32 on 32-bit plans
   and binary64 on 64-bit plans. Decimal/float conversions must consume the
   selected plan rather than assuming f64.
3. **Rounding:** reuse the six existing `RoundingMode` cases. A legal explicit
   mode is applied once to the exact mathematical value after unit conversion;
   no implicit rounding or binary-float intermediate is permitted. The one
   exception is decimal division (MD-066 § 5), which rounds an inexact
   quotient once with nearest-even.
4. **D1 (MD-066) — scale/canonicalization:** decided 2026-10-28
   (`rules/corrections/md066-md067-md068-decimal-correction-20261028.md`).
   The public scale is `int32` with a fallible exact setter; operations
   select the exact scale closest to their preferred scale; division alone
   rounds (nearest-even, finest admissible scale); a provably zero decimal
   divisor is a compile-time error. Owners: `types.md` "Decimal scale",
   `operators.md` "Decimal arithmetic", `core-library.md` §§ 10.1, 10.5.
5. **D2 (MD-067) — arithmetic error:** decided 2026-10-28. No
   `DecimalArithmeticError`; `core::ArithmeticError` gains `ScaleOverflow`
   and `PrecisionExhausted`, chosen by the most-specific-applicable-error
   rule (`errorhandling.md` § 3.4, `core-library.md` § 20.1).
6. **D3 (MD-068) — decimal128 core members:** decided 2026-10-28.
   `decimal128` mirrors the complete `decimal` member family
   (`core-library.md` § 10.4). `Parse()` and `ToUInt*` stay deferred.
7. **One compilation track:** P21 widens only Sec MLIR → standard MLIR → LLVM.
   The historical legacy-path requirement is not permission to restore a
   parallel backend.

## Completion boundary

P21 completes exact `decimal` and `decimal128` only after approved D1–D3
semantics are represented through immutable Sema plans, Semantic IR, verified
schema-17 Sec MLIR and proof-preserving checked lowering. Arithmetic and
comparison use exact integer/rational semantics; physical materialization is
separate and never selects binary float, host `int64`, hidden allocation, a
mandatory runtime or ABI layout by accident.

P21 excludes string formatting/materialization, decimal parsing from runtime
text, advanced math, unapproved decimal128 API, physical ABI/layout decisions,
new allocation policy, and any decimal behavior whose normative gate remains
open.

## A. Establish an executable and synchronized baseline

- [ ] P21-01 — Record the exact working snapshot, untracked inventory,
  predecessor matrix (P15–P20), Go/MLIR/Clang versions and `SEC_MLIR_BIN`.
- [ ] P21-02 — Run relevant predecessor Go, vet, MLIR-verifier and native-track
  controls; distinguish pre-existing failures from P21 work.
- [ ] P21-03 — Reconcile all P21 inputs with current canonical types,
  operators, layout, core-library, errors, runtime checks, units, Semantic IR,
  dialect and lowering rulebooks. Preserve version history rather than
  overwriting canonical rulebooks with package snapshots.
- [ ] P21-04 — Update P21 text and planning to the current `ConversionError`,
  MD-014 float width, MD-047 rounding contract and one-track architecture.
- [x] P21-05 — Confirm D1–D3 in `missing-decisions.yaml`; update their owner
  entries if evidence has changed, but do not implement gated semantics before
  a normative decision is applied. Done 2026-10-28: MD-066/MD-067/MD-068 are
  `decided-implementation-pending` and synchronized into their owning
  rulebooks; implementation follows the correction's § 13(3) step order.
- [ ] P21-06 — Create or update the sole `lowering.sec-mlir-package21`
  integration in `governance/lowering.yaml` with exact gates, dependencies,
  evidence, partial state and deferrals; validate with `yamlstatus`.

## B. Credit the implemented decimal foundation

- [x] P21-07 — Keep `decimal`, `decimal128`, `int128`, `int256`, `uint128`
  and `uint256` active in Sema and Semantic IR with distinct type identity and
  no binary-float fallback.

- [x] P21-08 — Parse and retain exact decimal literal coefficients, scale and
  lexeme using arbitrary-precision integers; validate decimal and decimal128
  coefficient widths without an `int64` gate.

  Current anchors: `internal/sema/constants.go`, `internal/sema/analyzer_test.go`,
  `internal/ir/semantic/model.go`, and `internal/ir/semantic/builder.go`.

- [x] P21-09 — Represent and verify high-level `const.decimal` values and emit
  `!sec.decimal` / `!sec.decimal128` constants without converting them to
  binary floating values.

  Current anchors: `internal/ir/semantic/verifier.go` and
  `internal/lowering/secmlir/emitter.go`. This is constant support only, not
  full decimal operation lowering.

- [x] P21-10 — Expose decimal `Scale` as the authoritative read-only `uint8`
  property; withhold decimal128 members until D3. The core `ToString` signature
  is fallible and correct, while its string-producing body remains outside P21.
  *Superseded 2026-10-28 (MD-066):* the property must become `int32` with a
  transactional `try set`; that change is tracked by P21-17.

  Current anchors: `internal/sema/compiler_known_members.go`,
  `internal/sema/decimal_scale_member_test.go`, and `sec/core/decimal.sec`.

- [x] P21-11 — Resolve existing frontend decimal arithmetic, exact constant
  evaluation, mathematical comparison, truncation remainder, unit carriers and
  nominal constraints without implicit decimal/float promotion.

  This credit does not approve a result-scale policy, operator failure contract
  or lowering surface.

  Correction 2026-10-18: the truncation remainder was in fact unresolved —
  `decimal % decimal` typed silently as invalid — and decimal mixed with a
  non-decimal numeric family returned invalid without a diagnostic. Both are
  fixed in `inferDecimalInfixExpression` (`TestFloatOperatorsAndLiteralShaping`).

## C. Complete safe infrastructure before gated arithmetic

- [ ] P21-12 — Publish immutable `ResolvedDecimalLiteralPlan` and
  `DecimalRepresentationPlan`: cloned coefficient, source lexeme/radix/exponent,
  target decimal kind, bounded intermediate scale/exponent, source location and
  selected layout reference (§§7–8, 25–27).
- [ ] P21-13 — Audit literal grammar and context shaping for unsuffixed,
  `m`-suffixed, exponent and base-prefixed forms; add bounded deterministic
  diagnostics for extreme input without `strings.Repeat`, host-width overflow
  or float parsing (§§7–8).

  Verified 2026-10-28: the unsuffixed host-float64 parse gate is removed.
  Exact large/tiny decimal literals reach Semantic IR; resolved binary literal
  ranges are checked in Sema after context shaping with mandatory S1175.
  The broader extreme-input/exponent audit remains part of P21-13.
- [ ] P21-14 — Correct all remaining stale decimal assumptions in package YAML,
  comments and governance: old int64 literal limits, old `Scale: int`, old
  NumericConversionError, fixed-f64 plain float and the retired legacy track.
- [ ] P21-15 — Publish a detached resolved decimal-conversion plan for the
  independently specified conversion directions. Include source/target,
  target-plan float format, exact mathematical proof/guard, optional resolved
  RoundingMode, canonical `ConversionError` mapping and unit/contract facts
  (§§9–10, 23–26).
- [ ] P21-16 — Implement only conversion infrastructure that current rules
  fully determine: exact source-value preservation, no binary-float decimal
  evaluation, exact integer-domain checks and one-rounding-after-unit-transform
  behavior. Keep scale-/error-policy-dependent paths blocked on D1/D2.

## D. Normative-decision-gated semantic work

- [ ] P21-17 — **D1 decided 2026-10-28 (MD-066); unblocked.** Implement the
  preferred-scale/nearest-exact-scale policy, the `int32` public scale with
  its transactional `try set`, zero behavior, literal scale, rounded division
  and provably-zero-divisor rejection for all plans (§§13, 16–17, 21–22).
  Partial 2026-10-28: Sema `Scale: int32` (decimal and decimal128 identities)
  with a checked `try set` reporting `PrecisionError`; Semantic IR
  `DecimalConstant.Scale` and the `sec.const.decimal` verifier accept negative
  scales (lit `md066-decimal-signed-scale.mlir`). Getter/setter lowering fails
  closed. Literal scale done 2026-10-28 (signed spelled scale, shared
  `constant.DecimalLiteral`/`FitDecimal`, width diagnostic). Operator scale
  plans, rounded division and the zero-divisor rule remain; the concrete list
  is in governance `lowering.sec-mlir-package21`.
- [ ] P21-18 — **D2 decided 2026-10-28 (MD-067); unblocked.** Map decimal
  failures onto `core::ArithmeticError` (adding `ScaleOverflow` and
  `PrecisionExhausted`) by the most-specific-applicable-error rule, and into
  Result/try/panic behavior. No `DecimalArithmeticError`, no `PrecisionError`
  for operators, no inferred union (§§14, 18–19).
- [ ] P21-19 — **D3 decided 2026-10-28 (MD-068); unblocked.** Add the
  decimal128 core declarations mirroring `decimal` (`core-library.md` § 10.4),
  compiler-known IDs, synthetic navigation, LSP presentation and tests
  (§§11, 15, 21).
- [ ] P21-20 — After D1/D2, publish immutable `ResolvedDecimalOperatorPlan`
  and `ResolvedDecimalMemberPlan` carrying exact operands/type, approved scale
  policy, error identity, source order, copy/effect facts and guard status.
- [ ] P21-21 — After D1/D2, implement unary and binary checked decimal
  operations with arbitrary-precision add/sub/mul/div/rem algorithms, exact
  rational division termination checks, truncation remainder, no implicit
  rounding outside division, and MD-066 § 5 nearest-even rounding of an
  inexact quotient (§§16–19).
- [ ] P21-22 — Implement exact numeric comparison independent of stored scale;
  lower only the already accepted decimal core methods. `Abs`, `Clamp`,
  `RoundTo` and `Rescale` preserve their individually declared error contracts
  and remain D1-gated where required (§§20–22).

## E. Semantic IR, schema 17 and verified emission

- [ ] P21-23 — Add Semantic IR decimal literal/operator/member/conversion
  plans and the P21 operation family from the approved facts only. Big integers
  are cloned on publication; builder code must never reparse AST text (§§25–28).
- [ ] P21-24 — Represent checked candidates, failure flag and reason such that
  candidates are success-only; preserve evaluation order and existing Result,
  try, ownership and cleanup semantics (§§18–19, 28).
- [ ] P21-25 — Install schema-17 only after schema-16 and predecessor work is
  reconciled. Reuse existing decimal type identities and add only missing P21
  operations, attributes, parsing, printing and registration (§29).
- [ ] P21-26 — Add decimal value, guard and conversion verifiers. Reject bad
  coefficient/scale, stale/unapproved D1/D2 IDs, wrong error identity, float
  intermediates, loss of source provenance, unsafe candidate consumption and
  unproved physical narrowing (§§27–29, 33).
- [ ] P21-27 — Extend the Go emitter from verified Semantic IR facts only;
  preserve exact coefficient, semantic scale, source kind and plan metadata.

## F. Lowering boundary

- [ ] P21-28 — Add `--sec-lower-decimal-core` as a proof-preserving,
  high-level lowering pass. It may use checked exact integer/scaling sequences
  or a versioned compiler helper but must not choose policy, allocate secretly,
  use signed-overflow UB or use binary float (§§30–32).
- [ ] P21-29 — Lower comparison mathematically rather than by raw component
  layout; lower conversions from exact integer/binary-rational values using the
  selected target float format and the single resolved rounding mode (§§20, 23,
  30–31).
- [ ] P21-30 — Add target/profile capability diagnostics when bounded exact
  arithmetic or its verified helper is unavailable. Do not claim full LLVM,
  ABI or physical-layout completion in P21 (§§30–33).
- [ ] P21-31 — Register supported P21 constructs with
  `sec-check-lowering-support`; incomplete and D1/D2/D3-blocked operations
  must fail as explicit normative-decision or unsupported-lowering boundaries.

## G. Required test matrix

- [ ] P21-32 — Test all literal forms, coefficient boundaries for i64/i128,
  extreme exponent/scale diagnostics, direct decimal128 shaping and no
  binary-float intermediate (§§7–8, 35).
- [ ] P21-33 — After D1/D2, test add/sub/mul/div/rem, terminating and
  nonterminating quotients, zero divisor, negative remainder, numeric equality
  across scales and named/unit-bearing cases (§§16–20, 36).
- [ ] P21-34 — Test decimal core members, `Scale: int32` with `try set`, error contracts,
  all rounding modes and any approved decimal128 parity surface (§§21–22, 37).
- [ ] P21-35 — Test conversions through all active integer widths, 32/64-bit
  plain float, fixed float widths, nonfinite values, exactness, rounding,
  ConversionError mapping, unit transforms and named contracts (§§9–10, 23,
  38).
- [ ] P21-36 — Add Semantic IR/MLIR negative tests for invalid scale,
  coefficient, policy/reason, candidate use, missing guard/provenance and
  physical narrowing; add differential tests against an arbitrary-precision
  evaluator (§§27–33, 39).
- [ ] P21-37 — Run focused Go tests, `go vet`, dialect/lowering verification,
  MLIR lit tests, 32/64 target-plan checks and the full suite. Record exact
  commands and results in governance (§§35–40).

## H. Acceptance, governance and handoff

- [ ] P21-38 — Reconcile acceptance gate A with current implementation and
  corrections; never treat a package file, parser support or constant support
  as full decimal lowering (§41).
- [ ] P21-39 — Do not begin acceptance gate B until D1, D2 and D3 are each
  normatively approved and synchronized into their owning rulebooks (§42).
  Approval and synchronization were completed 2026-10-28; gate B now waits on
  implementation evidence only.
- [ ] P21-40 — Update governance, language-rulebook status, missing decisions,
  package status and P20 compatibility evidence with exact implemented,
  blocked, deferred and unsupported states.
- [ ] P21-41 — Produce the required implementation report: snapshot,
  corrections, decision status, plans, exact representation, verifier/lowering
  evidence, test results, deviations and the bounded next-package handoff
  (§43–44).

## Recommended execution order

1. A: capture the baseline and resolve P20/P21 compatibility drift.
2. B/C: complete exact literal and conversion infrastructure that requires no
   unapproved scale/error/API policy.
3. Stop at D1–D3 where required; obtain normative corrections before operator,
   error-CFG or decimal128-member implementation.
4. E/F: after decisions and P20 schema prerequisites, install schema 17 and
   widen the single native track.
5. G/H: verify every stage with focused and end-to-end evidence before moving
   an item to complete.
