# MD-048 — Affine and logarithmic unit conversion: normative completion

- **Document class:** Normative rulebook correction / implementation contract
- **Status:** Ready for rulebook integration and review; **not yet merged or implemented**
- **Date:** 2026-11-19
- **Document revision:** 1
- **Applies to:** Sec 0.1 unit conversions; future binary float formats follow the same format-parametric rules when introduced
- **Primary owner:** `rules/types/units.md`
- **Required sync:** `rules/foundations/grammar.md`, `rules/compiler/semantic_ir.md`, `sec/core/error.sec`, `rules/library/core-library.md`, compiler diagnostics, governance, tests
- **Existing decisions preserved:** MD-012 (`ConversionError`), MD-040 (unit calls), MD-047 (single final rounding), MD-062 (public metadata projection)
- **Evidence:** conversation decisions through 2026-11-19; `MD-048-rekonstruktion-2026-11-18.md` rev. 14; `MD-048-reference-typed-expression-correction-2026-11-18.md`; `MD-048-numeric-strategies-error-bounds-correction-2026-11-18.md` rev. 12
- **Repository baseline:** `YoePro/sec` main inspected 2026-11-19, latest checked commit `cc987fc3bbab8229f45227e3e2be47839d67e282`; current `rules/types/units.md` and `missing-decisions.yaml` still require MD-048 integration.

## § 1. Scope and normative integration

**§ 1(1).** This document completes the *unit-transform conversion* semantics of MD-048. The rules below are normative **proposed insertions/corrections** to the existing unit rulebook, not a new competing unit model. Retain every existing section/paragraph ID in `units.md` (§§ 1–50); integrate subordinate subsections or append new numbered paragraphs. A unit declaration is not an implementation claim.

**§ 1(2).** The conversion contract consists of (i) statically valid source and destination unit relationships, (ii) one mathematically defined mapping from source coordinate to destination coordinate, (iii) independent domain/range validation, (iv) an effective numerical strategy and explicit rounding permission, and (v) the final target-carrier result. The compiler may optimize the mapping only if it preserves the chosen contract.

**§ 1(3).** The general `FastMath` hardware-policy system, FFI, processor state, tasks, threads, and global FP-environment isolation do not belong to MD-048. The separate file `MD-048-side-note-numeric-environment-0.1x-2026-11-19.md` retains previously approved principles for Sec 0.1.1 or later. This file does not create new compiler-wide strategy syntax.

**§ 1(4).** The existing restrictions on nominal unit identity, category, dimension, Kind, declared conversion functions, point/difference role, implicit conversion, `try`, and named-type contracts continue to apply. A numerical approximation policy never manufactures unit compatibility.

## § 2. Add to `units.md` § 21: affine conversion

**§ 21.1(1) — Exact affine mapping.** For source affine coordinate `a`, source metadata `S_s`, `O_s`, destination metadata `S_t`, `O_t`, and a common canonical point coordinate `c`:

```text
c = a * S_s + O_s
b = (c - O_t) / S_t
  = (a * S_s + O_s - O_t) / S_t
```

A source or destination with `UnitTransform.Linear` uses its linear formula (`O = 0` for coordinate mapping), **only** when point/difference roles and origin compatibility permit it. Intermediate arithmetic above denotes one exact mathematical expression, not mandatory physical storage operations.

**§ 21.1(2) — Nonzero and signed Scale.** Affine `Scale` is any finite, nonzero compile-time real scale accepted by the unit metadata numeric contract. Negative values are valid and reverse coordinate direction; zero is invalid at declaration time. The implementation must correctly reverse interval endpoints and comparison/range proof ordering when `Scale < 0`. This extends § 17.7 without weakening point algebra.

**§ 21.1(3) — Origin and point identity.** Matching dimensions and numerical scales alone do not establish that two point origins are compatible. `Origin` is a semantic datum and `Offset` a coordinate displacement; one must not be inferred from the other. A conversion that cannot establish compatible point origins at compile time is rejected unless a separately declared and validated explicit relationship supplies it. `UnitOrigin.None` is not evidence of compatibility.

**§ 21.1(4) — Difference path.** The difference corresponding to an affine point uses the scale but **not** the point offset:

```text
difference_canonical = difference_source * S_s
difference_target = difference_canonical / S_t
```

A difference shall never gain or lose a point offset merely because it is represented using the same unit spelling as a point. The point algebra in § 23 remains unchanged. Two compatible points may be subtracted; two points may not be added.

**§ 21.1(5) — Result errors.** A known invalid scale, inconsistent origin, or invalid unit relationship is a compile-time diagnostic. A mathematically valid conversion whose target carrier cannot represent the result follows §§ 29–30 and § 22.5 below; it is not silently clamped, wrapped, or rounded twice.

## § 3. Add to `units.md` § 22: logarithmic transform and inverse

**§ 22.1(1) — Parameters.** For a logarithmic unit define: source logarithmic coordinate `l`; logarithm base `B` (`LogBase`); logarithmic factor `F` (`LogFactor`); optional explicitly declared linear reference quantity `R` (`Reference`). `B` must be a positive finite real other than 1; `F` must be finite and nonzero; a declared `R` must be a positive finite compatible *linear* quantity. Invalid constant metadata is a declaration error.

**§ 22.1(2) — Level with Reference.** If `Reference` is present, the corresponding linear quantity `q` is:

```text
q = R * B^(l / F)
l = F * log_B(q / R)
```

The inverse is defined for finite real `q > 0`, within compatible source/destination quantity domains. `q == 0` and `q < 0` have **no finite real inverse** under this model. A negative logarithmic coordinate `l` is entirely valid; it does not imply a negative `q`.

**§ 22.1(3) — Ratio without Reference.** Without `Reference`, the unit denotes a *logarithmic ratio coordinate* and maps to a positive, dimensionless linear ratio `r`:

```text
r = B^(l / F)
l = F * log_B(r)
```

This is a *mathematical ratio mapping*, **not** an implicit metadata initializer `Reference: 1`. Omitted `Reference` and explicit `Reference: 1` remain distinguishable metadata and semantic declarations. The compiler may not invent a unit-dependent physical reference.

**§ 22.1(4) — Base and factor signs.** `0 < B < 1` is permitted. A negative `F` is permitted. Both preserve invertibility of the logarithmic mapping; monotonic direction depends on `ln(B) / F`. Division by zero in `F`, base 1, nonpositive base, and nonpositive reference are declaration errors. No accidental `abs(q)` transformation is allowed for a negative underlying input.

**§ 22.1(5) — Natural logarithm.** The semantics use the mathematical constant `B`, not a host approximation to it. An expression for `LogBase` must be supported by the existing compile-time constant rules. This correction introduces **no new literal, built-in `e`, symbolic irrational constant expression, or implicit approximation** solely to spell a natural base. When existing expression facilities cannot supply an exact intended base, the unit declaration cannot claim that exact base by writing a rounded substitute.

### § 22.2 Typed Reference and underlying quantity domain

**§ 22.2(1).** `Reference:` accepts an ordinary typed compile-time numeric quantity expression, including a carrier-qualified constructor. Sec introduces **no** `Reference[T]:` syntax.

```sec
unit dBm ratio

impl <dBm> {
    Dimension: []
    Kind: UnitKind.Ratio
    Transform: UnitTransform.Logarithmic
    LogBase: 10
    LogFactor: 10
    Reference: decimal128<W>(0.001)
}
```

**§ 22.2(2).** `Reference: 0.001<W>` is also valid; the ordinary default numeric carrier for a quantity literal is `decimal` where the unit has not specified another carrier. Explicit `decimal128<W>` selects the initializer's carrier, **not** the later result's carrier. Typed reference initializer expressions are not metadata-specific grammar; they use the normal Sec expression/type rules and must be evaluable at compile time.

**§ 22.2(3).** Analyze and retain separately: (a) the *logarithmic coordinate unit* (`dBm` may have `Dimension: []`, `Kind: UnitKind.Ratio`), and (b) the *linear reference domain* (`W` has its own physical dimension and Kind). The coordinate unit's dimensionless representation must not erase the referenced physical dimension. Two referenced logarithmic levels can convert via a fixed relationship only when their resolved linear reference domains are compatible; a log ratio without a reference does not thereby become a physical power level.

**§ 22.2(4).** Compatibility of references requires resolved unit identity/dimensional compatibility, compatible known physical Kind, compatible role (ordinary positive linear quantity, not an affine point), and a known exact canonical relationship. Different known Kinds may not be bridged by a guessed scale or a mere equality of dimensions. Explicit declared conversion functions and their existing authority remain available as specified in § 31.

**§ 22.2(5).** The compiler retains the reference's normalized mathematical value, source type, carrier, original unit identity, resolved dimension, Kind, and positive-domain proof. The metadata initializer **may be typed differently** from both the logarithmic coordinate and conversion result. Conversion arithmetic must not use an inexact public metadata projection in place of these semantic facts.

**§ 22.2(6) — Public metadata reconciliation.** `units.md` § 18 exposes `<U>.Reference` through its existing public projection (`decimal` or `decimal<V>`), whereas the declaration can have a wider initializer carrier (e.g. `decimal128<W>`). The public read is **not** the exact internal reference value. If the existing projection cannot represent an accepted initializer under § 18's numeric-read contract, the public read is diagnosed, rather than invalidating the unit declaration or silently changing conversion semantics. An absent `Reference` is a compile-time error **when read**, not a fabricated `1`; for a valid reference-free logarithmic ratio, omission alone is **not** a declaration error. Revise only § 18(15)'s current conflicting sentence claiming *every* logarithmic unit requires a reference.

### § 22.3 Explicit conversion paths

**§ 22.3(1) — Linear/affine to logarithmic.** Normalize the source's compatible physical *linear quantity* (or dimensionless ratio) to canonical value `q`, then evaluate the destination's inverse logarithmic mapping. A point cannot be silently used as a free quantity. Require `q > 0` before taking the logarithm; reject a conversion with an unsatisfied domain. The destination metadata determines the reference domain and logarithmic coordinate.

**§ 22.3(2) — Logarithmic to linear/affine.** Apply the source forward mapping to the compatible positive canonical quantity or ratio, then convert to the destination linear representation. A conversion into a destination point still requires an independently valid point-origin relationship and must not be inferred from a physical matching dimension.

**§ 22.3(3) — Logarithmic to logarithmic.** For two referenced *levels* with source `(B_s,F_s,R_s)` and destination `(B_t,F_t,R_t)`, assuming `R_s` and `R_t` are expressed in the **same exact canonical linear quantity**, the mathematical target coordinate is:

```text
l_t = F_t * [ ln(R_s / R_t) + (l_s / F_s) * ln(B_s) ] / ln(B_t)
```

This is exactly equivalent to `q = R_s * B_s^(l_s/F_s)` followed by `l_t = F_t * log_(B_t)(q/R_t)` for all finite real in-domain values. The implementation should compose the expression directly or use an equivalent stable method; it must **not** require materialization of a `q` which can overflow an intermediate carrier when the final `l_t` is finite and representable.

**§ 22.3(4) — Ratio-to-ratio.** For two *reference-free* logarithmic ratios sharing a compatible underlying ratio domain:

```text
l_t = (F_t / F_s) * (ln(B_s) / ln(B_t)) * l_s
```

This simplification must preserve exact mathematical meaning and the chosen numerical policy. The compiler shall not mix referenced levels and reference-free ratios as if `Reference: 1` had been declared, unless an independently valid explicit conversion supplies the missing physical reference/domain relationship.

**§ 22.3(5) — Example.** With `dBm` reference `0.001<W>` and `dBW` reference `1<W>`, both base 10 and factor 10, an in-domain source coordinate of `30 dBm` converts to `0 dBW` (exact mathematical result). A log-level conversion may be representable exactly even when a naive intermediate `pow()` is not; final representability decides success, not an incidental evaluation order.

**§ 22.3(6) — Conversion resolution.** Constructor-style explicit conversion and carrier-qualified conversion follow §§ 30–32. Do not add an implicit log conversion merely because a numerical transform exists. The explicit destination conversion function remains authoritative when called (§ 31(6)). No ordinary addition/multiplication of logarithmic coordinates is introduced here; § 22(7)–(8) remains in force.

### § 22.4 Conversion-plan and proof invariants

**§ 22.4(1).** Analysis must construct a specialized transform plan preserving: source/target unit IDs and role; transform kinds; canonical dimension and Kind; origin compatibility; exact Scale/Offset facts; source/destination base/factor; reference presence and reference physical domains; normalized reference values; source/target numeric carriers; effective strategy; explicit rounding mode, if present; known domain and representability proofs; and compile-time/runtime error classification.

**§ 22.4(2).** A plan is rejected before lowering if source/target relationship or compile-time metadata is invalid. Do not silently turn an invalid or unresolved logarithmic plan into a scale-only plan; do not retain only source text for `Reference` as if it were a validated semantic constant.

**§ 22.4(3).** Lowering may algebraically fold and compose conversions and may use target-specific optimized operations, but cannot alter their unit semantics, cause multiple independently rounded intermediate conversions, or bypass errors/required checks. Source expression evaluation order and the rights of an explicitly called conversion function remain governed by the original rulebooks.

### § 22.5 Exactness, rounding, strategies and failure

**§ 22.5(1) — Mathematical reference.** The mathematical result `x` is the real-number result of the *entire* valid unit transform expressed in the destination's coordinate system, before target-carrier rounding. Exact intermediate rational arithmetic is retained where applicable. Transcendental log/exp steps are specified by the exact mathematical functions; a host or target libm is not implicitly an exact oracle.

**§ 22.5(2) — Strict.** `Strict` is the default. An implicit conversion requires exact representability and all usual implicit-unit conditions. An explicit conversion without rounding/approximation permission also requires exact representability. When a conversion is permitted an explicit `RoundingMode`, the returned representable result must be the mathematical value rounded **once at the final destination carrier** in the requested direction; all intermediate operations must be sufficiently accurate to prove that final result. A compiler may use arbitrary precision, algebraic proofs, or certified routines; it may not substitute a merely approximate `log()`/`exp()` and claim correctly rounded Strict semantics. If an implementation cannot establish the required result for a particular target/path, it may take a different conforming implementation or diagnose the unsupported guarantee; it may not silently weaken it.

**§ 22.5(3) — One rounding.** In accord with MD-047, `RoundingMode` never means “round the source, round a linear intermediate, then round the destination.” Mathematical cancellations and exact conversions must remain eligible for exact final results (e.g. `30 dBm` to `0 dBW`). Any internal rounding is implementation detail whose combined effect must not change the externally required final result.

**§ 22.5(4) — BoundedApproximation.** An explicitly selected bounded strategy admits an implementation only if its **delivered** finite numeric result `y` satisfies its stated bound against exact final target-coordinate `x`, inclusive of every internal approximation and the final carrier representation. It does not authorize unit or domain violations, NaN/infinity substitution, or undisclosed target-range failures. If the guarantee cannot be established, fall back to a conforming implementation; reject a proven-impossible constant requirement; otherwise report a checked precision failure on paths for which the implementation cannot establish the promise. The bounded strategy does not itself grant an arbitrary extra rounding at the conversion boundary.

**§ 22.5(5) — Absolute/relative metric.** For nonnegative absolute error allowance `A` in the destination *difference* coordinate and nonnegative dimensionless relative allowance `R`, the user-approved combined rule is:

```text
x = exact mathematical destination coordinate
y = delivered destination coordinate
absolute_error = |y - x|
allowed_error = max(A, R * |x|)
accept iff absolute_error <= allowed_error
```

With an affine point, `A` is a difference (no offset); with a logarithmic level or ratio, `A` is an interval in the destination's **logarithmic coordinate**, not a physical linear power error. For standalone relative tolerance at `x = 0`, only exact zero is accepted unless an explicit absolute allowance is also present; no divide-by-zero evaluation is necessary. `A = 0` or `R = 0` demands the corresponding exact target-coordinate result.

**§ 22.5(6) — ULP metric.** For a finite real exact coordinate `x`, ULP refers to the resolved **destination binary floating format** (not intermediate format and not metadata initializer carrier). For a finite nonrepresentable `x`, local ULP is the gap between its consecutive representable neighbors; for an exactly representable `x`, use the *smaller positive gap* to adjacent distinct finite values. At zero/subnormal magnitudes use the smallest positive subnormal step. Signed zero has zero numerical distance to zero but other signed-zero rules remain independent. NaN, infinity, and out-of-finite-range values do not satisfy a finite ULP bound. The format-parametric rule covers binary32/binary64 and future float16/bfloat16 **once supported**, including their actual exponent and significand parameters. This is a conservative precise boundary convention; the source syntax for ULP limits belongs to the common numeric-policy interface.

**§ 22.5(7) — Multiple bounds.** The combined Absolute+Relative envelope in § 22.5(5) is normative. A ULP allowance independently specifies an eligible bound; when more than one independently specified metric is active for the same result, **each supplied independent metric must be satisfied** unless the common numeric-policy interface expressly defines a single combined form. This is not permission to silently sum separate error budgets.

**§ 22.5(8) — FastMath.** `FastMath` is an explicit opt-in without a universal numerical error guarantee. It may select faster approximation strategies for eligible binary float unit conversions, but never silently bypasses transform/domain/Kind/range/named-type checks or changes the meanings of a separately supplied `RoundingMode`. Selecting `FastMath` alone does **not** assert absence of NaN/infinity and does **not** enable FTZ or DAZ. `nnan`, `ninf`, blanket LLVM `fast`, FTZ, and DAZ require their separate approved permissions or compiler proof. Do not silently reinterpret `decimal`, `decimal128`, or future exact `rational` using binary fast-math.

**§ 22.5(9) — Active strategy.** For a unit conversion, the active policy is the nearest explicitly selected lexical-block policy, otherwise the function policy, otherwise the project default, otherwise `Strict`. Callees do not inherit strategy dynamically. A local `Strict` selection overrides a broader `FastMath` default. These are required semantic precedence rules; `@math(...)`, `absolute:`, `math = ...`, and other historical examples are **not ratified grammar** and must not be treated as implemented source syntax merely by appearance here. The shared policy-owner must specify grammar separately, without changing the unit conversion contract.

**§ 22.5(10) — Target selection.** The compiler normally selects a conforming conversion implementation automatically. An explicit `Allow` target preference may fall back to another conforming implementation, but must report unavailability and the chosen route with actionable advice. An unsatisfied explicit `Demand` is a compile-time diagnostic; a named target-feature demand (e.g. AVX2) requires that feature in the declared compilation target, not a runtime fallback. A named capability is not a demand to emit a particular instruction; exact instruction choice belongs to explicitly target-gated intrinsics, specified outside this unit rulebook. FTZ and DAZ are independent explicit preferences; neither is implicit in `FastMath` and neither bypasses a bounded-error promise.

**§ 22.5(11) — Failure categories.** The compiler distinguishes:

| Situation | Constant/proven-invalid at compilation | Runtime-dependent checked conversion |
| --- | --- | --- |
| Invalid unit/Kind/origin relationship or invalid metadata | Compile-time error | Not dynamically repairable; compile-time error |
| Logarithm of nonpositive linear input / invalid logarithmic domain | Compile-time error | `ConversionError.InvalidDomain` (new variant required) |
| Otherwise valid result outside destination carrier range | Compile-time error | `ConversionError.OutOfRange` |
| Result not exactly representable without a permitted rounding/approximation, or certified bound cannot be met | Compile-time error when provable | `ConversionError.PrecisionLoss` |
| Nonfinite floating input incompatible with conversion domain | Compile-time error when provable | Existing `ConversionError.NonFinite` where applicable; `InvalidDomain` for a finite but logarithmically invalid real input |

A conversion's required `try` behavior continues to follow § 46 and existing effect/failure analysis: a provably total conversion does not become fallible merely because a mode was specified.

**§ 22.5(12) — Core error synchronization.** The current `sec/core/error.sec` `ConversionError` contains `OutOfRange`, `InvalidScalar`, `PrecisionLoss`, `ScaleOverflow`, `NonFinite`, `Contract(...)`, **but no `InvalidDomain`**. MD-048 therefore requires adding a documented `InvalidDomain` variant to that same public union (not a duplicate error family). This is a targeted cross-book correction mandated by the agreed log-domain distinction. `MathError.DomainError` remains the math-library error and is not silently substituted for the checked *conversion* error.

**§ 22.5(13) — Precision versus measurement.** A sensor's measurement uncertainty is **not** the same as a conversion's approximation error tolerance. MD-048 measures only numeric conversion error in destination coordinates. The compiler shall not infer an allowable bound from the physical accuracy of a sensor or measurement.

## § 4. Required in-place changes to existing `units.md` paragraphs

**§ 4(1).** **Keep** the numbers `§ 17.7(2)`, `§ 18(13)`, `§ 18(15)`, `§ 22(2)–(6)`, `§ 30(4)`, and `§ 44(2)`. Change only their conflicting wording, as follows:

| Existing paragraph | Normative reconciliation without renumbering |
| --- | --- |
| `§ 17.7(2)` | Replace “Negative scales on affine units remain under MD-048” with the signed nonzero affine rule in new `§ 21.1(2)`. |
| `§ 18(13)` | Clarify that public `<U>.Reference` is a `decimal`/`decimal<V>` *projection*, potentially distinct from the initializer's carrier; missing `Reference` has no public value. Do not alter the 16-member surface. |
| `§ 18(15)` | Replace “a logarithmic declaration missing any of them is a declaration error” with “a logarithmic declaration missing `LogBase` or `LogFactor` is a declaration error; `Reference` may be omitted only for a reference-free ratio declaration; reading an absent `Reference` is an error.” |
| `§ 22(2)` | Read the metadata list as `LogBase`, `LogFactor`, and `Reference` **when present**; `Reference` absence has defined ratio semantics rather than a fabricated default. |
| `§ 22(3)–(5)` | Point to `§ 22.1` equations and `§ 22.2` reference/domain rules; eliminate ambiguity over the position of `LogFactor`. |
| `§ 22(6)` | Keep existing `Reference: 1` example valid as a declared dimensionless reference. Add a separate example with *no* `Reference` for reference-free log ratio semantics. |
| `§ 30(4)` | Link to `§ 22.5` for mathematical exactness and one final rounding after any logarithmic/affine transform; do not silently round intermediate `pow`/`log` results. |
| `§ 44(2)` | `LogBase` and `LogFactor` required; `Reference` required for a referenced level, optional for a reference-free ratio; validate positivity and domain compatibility where declared. |

**§ 4(2).** The compatibility revision in `§ 18(13)` does **not** automatically create a `decimal128` public metadata member type. That would change the existing MD-062 read surface and requires its own decision; internal exact unit conversion facts are not derived from the public projection.

## § 5. Grammar, Semantic IR and lowering sync (implementation obligations)

**§ 5(1) — Grammar.** Extend `rules/foundations/grammar.md` § 26 with a documented semantic production for `Reference:` whose value is a complete ordinary typed compile-time expression (e.g. `decimal128<W>(0.001)`). `TokensUntilLineEnd` remains a parser integration detail, not a substitute for typed expression validation. Do not implement `Reference[T]:`. Preserve existing metadata name spelling and declarations.

**§ 5(2) — Sema.** Resolve `Reference` as a compile-time typed quantity, normalize into the linear canonical domain, record *presence versus omission*, and validate positivity, Kind, dimension, and source unit. Resolve logarithmic conversion availability and numeric errors independently from syntactic call shape. Keep named-unit resolution and explicit user-defined conversion precedence intact.

**§ 5(3) — Semantic IR.** A conversion plan must carry the facts enumerated in § 22.4(1), including specialized affine/log transforms and effective conversion policy. Exact normalized mathematical constants are semantic values; public `decimal` projections and source-text strings cannot serve as plan identity. Failure/effect information and source positions must be preserved.

**§ 5(4) — MLIR/backend.** Lower exact affine composition and logarithmic transforms without forcing premature float intermediates or emitting two final-rounding operations. Backends may choose platform-specific instructions and certified numerical libraries, but the declared target, Strict contract, bounded-result guarantee, and explicit FastMath permissions constrain every lowering. Extend conformance coverage independently of whether every optimization is initially implemented.

**§ 5(5) — Diagnostics and LSP.** A diagnostic should identify: source/destination units; failing condition; relevant `Reference` dimension/Kind; demanded target feature, if any; whether the invalid value was compile-time-known; and a viable correction (provide an explicit reference, use compatible unit, choose representable carrier, supply a valid rounding mode, or loosen an explicit nonessential hardware preference). No generic “requires specialized conversion operation” for a compatible supported logarithmic relationship.

## § 6. Conformance matrix: required positive and negative cases

| ID | Test / requirement | Expected |
| --- | --- | --- |
| A01 | Affine point `a`, `S=1`, `O=273.15` converted to compatible linear canonical point | Mathematically `a + 273.15`, one final carrier rounding |
| A02 | Affine point with negative `Scale`; round-trip points | Invertible, direction reversal and bounds correct |
| A03 | Difference associated with an affine unit | Scale applied; offset never applied |
| A04 | Two incompatible declared point origins | Reject before lowering |
| A05 | Affine `Scale: 0` | Declaration error |
| L01 | `B=10`, `F=10`, `Reference: 0.001<W>`; `l=30` | Underlying canonical power is `1<W>` |
| L02 | Above level converted to `B=10`, `F=10`, `Reference: 1<W>` | Destination log coordinate exactly `0` |
| L03 | Reference-free ratio `B=10`, `F=10`, `l=20` | Linear ratio `100`; no reference metadata fabricated |
| L04 | Ratio with omitted `Reference`, reading `<U>.Reference` | Compile-time metadata read error |
| L05 | `Reference: decimal128<W>(0.001)` | Accepted typed compile-time initializer; conversion result carrier independent |
| L06 | `Reference[T]: ...` | Reject unsupported metadata syntax |
| L07 | Referenced power level to unrelated time/length reference | Reject dimension/Kind incompatibility |
| L08 | `B=1`, `B<=0`, `F=0`, `R<=0` | Declaration errors |
| L09 | `B=0.5` and/or `F<0` | Valid; correct monotonic sign and inverse |
| L10 | Negative finite logarithmic coordinate | Valid, maps to positive linear quantity |
| L11 | Inverse log with source `q=0` or `q<0` | InvalidDomain (static diagnostic or runtime checked failure) |
| L12 | Two referenced log levels with differing references | Direct composed path; no overflowing mandatory linear intermediate |
| L13 | Referenced log level mixed with reference-free ratio | Reject absent independently validated explicit relationship |
| L14 | Negative linear amplitude passed to logarithm | Reject domain; do not apply `abs` |
| P01 | Strict log/exp path with exact result representable | Exact target result, no premature rounding |
| P02 | Strict path with explicit rounding | Exactly once to requested final target carrier |
| P03 | Strict path requiring inexact result with no permission | PrecisionLoss or compile-time error |
| P04 | Bounded Absolute+Relative | Enforce `|y-x| <= max(A, R*|x|)` in target coordinates |
| P05 | Bounded ULP for binary formats | Uses target format including subnormals and exponent boundaries |
| P06 | `float16`/`bfloat16` integration once supported | Same format-parametric tests; no premature implementation claim |
| P07 | Bounded allowed error smaller than unavoidable representational error | Reject known impossibility or checked PrecisionLoss |
| P08 | FastMath alone | Does not inject `nnan`/`ninf`, FTZ or DAZ; unit/domain checks remain |
| P09 | Project FastMath + explicit function/block Strict | Local Strict wins; callee does not inherit caller mode |
| T01 | Explicit `Allow` unavailable on declared target | Conforming alternative + informative diagnostic |
| T02 | Explicit hardware `Demand AVX2` on non-AVX2 target | Compile-time error with alternatives, never silent fallback |

## § 7. Ownership, closure and excluded design work

**§ 7(1).** These rules close the MD-048 *design* question for affine/logarithmic unit transforms, subject to integration of the necessary core error variant and editorial reconciliation with extant §§ 18/22/44. **Do not mark the implementation complete** until Sema, Semantic IR, lowering, diagnostics, and conformance tests have corresponding verified work. `missing-decisions.yaml` must distinguish `decided` (normative rules integrated) from implementation status.

**§ 7(2).** Cross-cutting syntax for strategy selection, platform intrinsic catalogs, global FTZ/DAZ execution-state isolation, task/thread/FFI semantics, and the implementation of hardware dispatch are **not** part of MD-048's completion criterion. Preserve those confirmed discussions in their existing side note and assign them to the relevant general rulebooks.

**§ 7(3).** This correction was drafted against the checked repository rules and recorded decisions; it does not assert any commit, code modification, or successful test that has not actually occurred. Before syncing, the integrator must compare the current repository HEAD and resolve later conflicting rules rather than overwriting them.

## § 8. Decision trace and honest provenance

**§ 8(1) — Explicitly approved in the conversation.** Typed `Reference: decimal128<W>(0.001)` instead of `Reference[T]`; three strategies with Strict default; project/function/block precedence; callee independence; Absolute+Relative+ULP in 0.1; combined tolerance envelope and defined absolute error; format-parametric ULP including future half formats; FastMath without implicit NaN/Inf assumptions; separately selected FTZ and DAZ; automatic-first plus Allow/Demand; missing named target feature = compile-time error; instruction-level request = explicit intrinsic; defer generic hardware-state isolation.

**§ 8(2) — In existing normative rules.** Linear/affine formulas, point/difference algebra, explicit conversion and Kind rules, named-unit/metadata identity, MD-047 rounding/`try`, and the current core `ConversionError` variants.

**§ 8(3) — Mathematically derived or editorially resolved for implementability.** Log→log composition, strict positivity domain, direct formula simplification, reference-domain facts, exclusion of optional physical reference from reference-free ratio, conservative local ULP spacing at representable exponent boundaries, zero handling for standalone Relative, and fail-closed bounded paths. Some source detail was only available as historical assistant recaps rather than primary user-acceptance turns; these are **documented consequences/interpretations**, not falsely attributed fresh user votes.

**§ 8(4) — No invented grammar.** Activation and error-bound source spellings remain owned by future common numeric-policy specifications. Nothing in this correction authorizes a parser to recognize `@math`, `absolute:`, `Reference[T]`, target-demand keywords, or other unratified tokens. MD-048 can establish the semantics *consumed* by a unit conversion without creating global grammar prematurely.
