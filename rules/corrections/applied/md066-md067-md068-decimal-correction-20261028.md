# MD-066 / MD-067 / MD-068 — Decimal Scale, Arithmetic Failure and Public Core API Correction

- **Status:** Applied 2026-10-30 — normatively synchronized; § 13(3) steps 1-4 (rules, core/API, frontend/CTE) implemented with the frontend part of the § 14 matrix (operator scale plans, nearest-even division, truncating remainder, zero-divisor diagnostics, ArithmeticError classification, Scale, the complete decimal/decimal128 member family, § 14(2) negatives). Steps 5-8 (Semantic IR decimal operation plans, schema-17 Sec MLIR and lowering, runtime formatting/conversions, the DEC-22 differential matrix) remain ordinary P21 work in governance lowering.sec-mlir-package21; Semantic IR rejects decimal arithmetic explicitly until then.
- **Created:** 2026-10-28
- **Last updated:** 2026-10-28
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline:** `YoePro/sec`, `main`, `7be87775a851be60a692a50010d1ab368550b7d1` (verified 2026-10-28).
- **Decisions resolved:** `MD-066` (`SEC-MLIR-P21-D1`), `MD-067` (`SEC-MLIR-P21-D2`), `MD-068` (`SEC-MLIR-P21-D3`).
- **Proposed canonical correction path:** `rules/corrections/pending/md066-md067-md068-decimal-correction-20261028.md`.
- **Normative owners:** `rules/types/types.md`, `rules/foundations/operators.md`, `rules/library/core-library.md`, `rules/errors/errorhandling.md`, `rules/errors/runtime_checks.md`, `rules/errors/panic.md` (where affected), `rules/compiler/compiler_known_members.md`, `rules/compiler/semantic_ir.md`, and the P21 normative synchronization, Semantic IR, Sec MLIR and lowering specifications.
- **Related authority:** `rules/memory/layout.md` §19; `rules/declarations/properties.md` §6; `rules/types/units.md` (MD-047); `sec/core/rounding.sec`; `sec/core/error.sec`; `rules/compiler/compile_time_evaluation.md`; `rules/control-flow/flowcontrol_match.md`.
- **Implementation tracking:** Reconcile the **existing** P21 package, `package21-todo.md`, `rules/mlir/packages/sec-mlir-dialect_package21.yaml`, relevant existing entries in `governance/types.yaml`, `governance/sema.yaml`, `governance/lowering.yaml` and other owning governance files. Do not create parallel IDs for already tracked work.
- **Supersedes:** The **proposed, unapproved** D1–D3 recommendations in `rules/mlir/normative-sync/sec_decimal_numeric_sync_package21.md` §§5–7 and the incompatible P21 package/snapshot assumptions. This correction does **not** silently supersede unrelated locked rules.
- **Change discipline:** Preserve existing numbered paragraphs and stable references. Amend in place or add stable new paragraphs; never renumber cited sections. A design decision is not proof of implementation.

> **MANDATORY IMPLEMENTATION HANDOFF:** Work **one numbered implementation step at a time**, perform its focused checks, fix failures and report the result before starting the next step. Check the live repository `main` HEAD before editing and re-read source files only when their sync identifier changes. Do not mark MD-066/067/068 *implemented* merely because this correction is accepted or the governance decision gate is opened. Do not silently advance a Sec MLIR schema or claim P15–P20 dependencies are complete.

---

## § 1. Authority, intent and decision boundary

**§ 1(1).** This correction records the Sec owner's accepted public semantics for the built-in `decimal` and `decimal128` families, their observable representation scale, arithmetic error classification and mirrored core APIs. It closes the *design questions* in MD-066, MD-067 and MD-068; synchronization, tests and implementation remain independent work.

**§ 1(2).** Sec decimal arithmetic must be useful for ordinary application code. In particular, a nonterminating decimal quotient such as `1m / 3m` is **not** an arithmetic failure merely because it cannot be represented exactly. Division is the sole ordinary decimal arithmetic operator in this correction allowed to approximate its mathematical result automatically. Its rounding is deterministic and specified by §5.

**§ 1(3).** Addition, subtraction, multiplication, unary arithmetic and remainder continue to require exact mathematical results within the decimal carrier representation. Their exactness requirement must not be weakened by the new division rule. Decimal comparison remains exact and numeric rather than representational.

**§ 1(4).** `decimal128` is an independent built-in type, not an alias, binary float, or silent promotion target for `decimal`. Its **public core API and semantics mirror `decimal`**, substituting `decimal128` for receiver, operand and result types where appropriate.

**§ 1(5).** Ordinary numeric/unit conversion rules from MD-047 remain exact unless an approved conversion API explicitly requests or specifies rounding. The automatic rounding in **decimal division** is a narrowly owned arithmetic rule; it does not authorize lossy implicit casts, lossy unit conversions, or default rounding in other operators.

**§ 1(6).** `rational` (exact fractions) and complex-number support are future, separately owned design topics. They do not change the public `decimal`/`decimal128` contract under this correction and must not be synthesized by P21 as an implementation shortcut.

---

## § 2. Storage and public scale domain (MD-066)

**§ 2(1).** Retain the existing *fixed*, target-independent physical component representation in `rules/memory/layout.md` §19:

```text
decimal    = { signed i64 coefficient, signed i32 scale }
decimal128 = { signed i128 coefficient, signed i32 scale }
```

The mathematical value is `coefficient × 10^(-scale)`. `int`, `uint` and plain `float` may be target-sized under their own rules; **neither decimal coefficient width is target-sized**.

**§ 2(2).** The public scale has Sec type **`int32`**, with the entire signed interval `int32.Min ... int32.Max`. Negative scales are legal. The prior public `uint8`/`0..255` recommendations are rejected. A compiler-known `int` scale and the old `uint8` scale are both invalid once this correction is applied.

**§ 2(3).** `.Scale` is a canonical **instance** property, not a static property of the type, and not a method. It supports infallible `get` and **fallible `try set`**. The setter has the established Sec property grammar and error contract:

```sec
// Observes or exactly changes the decimal scale without changing its value.
property Scale: int32 {
    get
    try set requestedScale PrecisionError
}
```

This block is a **public accessor contract**; real core/privileged definitions supply valid bodies or compiler-known operations. Do not emit a bodyless ordinary `impl` as a purported completed implementation.

**§ 2(4).** A successful `try value.Scale = requestedScale` leaves the *mathematical value* unchanged and commits exactly the requested scale. The coefficient is updated accordingly. If exact rescaling is impossible, assignment fails with `PrecisionError` and **neither coefficient nor scale changes**. The receiver must denote a mutable, assignable Place. A read-only `let` may read `.Scale` but may not set it. The setter never rounds.

**§ 2(5).** `.Scale` has **one canonical semantic identity** with getter and setter. Continue to resolve legacy lowercase `.scale`, where the existing migration policy allows it, as an alias of that same identity; do not introduce a second property or make `.scale` a separate field. New source and generated documentation use `Scale`.

**§ 2(6).** `MinScale` and `MaxScale` in the scale-selection algorithm below are **internal mathematical boundaries for a particular result value**. They are **not** new public static or instance properties of `decimal`/`decimal128`. The carrier's fixed scale domain is already expressed by `int32.Min`/`int32.Max`.

**§ 2(7).** The internal coefficient is not public merely because the representation is specified. Do not publish `Coefficient` or representation constructors/accessors under this correction. The universal unsafe `.ptr` obeys existing pointer/addressability rules and is not an API to bypass the scalar invariant.

---

## § 3. Preferred scale for exact operations (MD-066)

**§ 3(1).** Evaluate each operation's mathematical result with sufficient exact precision to decide representability before selecting its final coefficient and scale. Do not use host binary floats, fixed host `int64` intermediates, or overflow-driven arithmetic as the semantic algorithm. Resource-bounded exact reasoning is allowed only when it preserves deterministic behavior/diagnostics.

**§ 3(2).** The preferred result scale `P` is calculated with sufficiently wide signed arithmetic, without wrapping the public `int32` scale:

| Ordinary operation | Preferred scale `P` |
|---|---|
| `a + b` | `max(a.Scale, b.Scale)` |
| `a - b` | `max(a.Scale, b.Scale)` |
| `a * b` | `a.Scale + b.Scale` |
| `a / b` | `max(a.Scale, b.Scale)` |
| `a % b` | `max(a.Scale, b.Scale)` |

**§ 3(3).** For a **nonzero, exactly representable** mathematical result `V`, let `S_min(V)` and `S_max(V)` bound the contiguous integer interval of signed `int32` scales at which a signed `i64`/`i128` **integer** coefficient exactly represents `V`. The selected scale is the closest member of that interval to `P`:

```text
resultScale = clamp(P, S_min(V), S_max(V))
```

Here `clamp` is a mathematical operation, **not** a call to `decimal.Clamp()`. For exact representable nonzero `V` the interval is contiguous and the selected scale is unique. If `P` lies inside the interval, it is retained unchanged; do not strip coefficient trailing zeros gratuitously.

**§ 3(4).** A different exact scale may be selected when the preferred scale cannot be represented. This is intentional and observable through `.Scale`. Do not reject an operation solely because its **preferred** coefficient overflows if another valid exact scale exists.

**§ 3(5).** Exact scale selection must use the **real asymmetric signed coefficient domain**, not a hypothetical symmetric absolute maximum. `-2^63` and `-2^127` are valid negative coefficients despite the absence of positive counterparts in the same signed width.

**§ 3(6).** For the exact mathematical result **zero**, its coefficient is zero. If `P` lies within the signed `int32` domain, preserve `P`; otherwise clamp `P` to `int32.Min` or `int32.Max` as appropriate. Zero must not fail merely because an intermediate preferred scale (notably a sum during multiplication) lies outside `int32`.

**§ 3(7).** Decimal literals preserve the scale implied by their exact source spelling when representable. If that scale is not representable, use the closest **exact** legal scale, if one exists; otherwise report the appropriate compile-time diagnostic. Never first decode a decimal literal through `float32`/`float64`. A literal zero keeps its source-preferred scale when valid. Exponent validation must remain bounded rather than materializing `10^N` of arbitrary size.

**§ 3(8).** Exact arithmetic must not change to an approximate operation just because the exact result cannot fit: `+`, `-`, `*`, `%` and unary arithmetic fail using §7 when no exact representable coefficient/scale pair exists. Division has the distinct policy in §5.

---

## § 4. Exact arithmetic, remainder and comparisons

**§ 4(1).** Addition/subtraction align exact powers of ten, multiplication uses exact coefficient product and scale addition, and all intermediate representability checks occur **after** exact mathematical evaluation. No operation may silently widen the result type from `decimal` to `decimal128` or convert it to binary floating point.

**§ 4(2).** `%` remains exact **truncation-based remainder**:

```text
q = trunc(a / b)                 // mathematical integer, toward zero
r = a - q * b                   // mathematically exact
```

This is **not** the rounded public `decimal` quotient from §5 and must never obtain `q` by first evaluating an approximate `a / b`. `%` by zero follows §6. An exact remainder that cannot be represented follows §7.

**§ 4(3).** Equality and ordering compare mathematical values: e.g. `1.2m == 1.20m`. Representation scale is not part of equality and must not affect comparison ordering. Comparison must not lower through binary floating point.

**§ 4(4).** Existing compound assignments retain their ordinary checked operation and assignment semantics; they use the same numerical policy as the corresponding binary operator, with exactly-once operand evaluation and transactional target updates where required by the owning assignment rules.

---

## § 5. Decimal division: maximum useful precision with automatic rounding

**§ 5(1).** The mathematical quotient is computed exactly (conceptually as an unbounded rational). If it has an **exact representation** in the selected decimal type, **use §3's preferred-scale/nearest-exact-scale algorithm**. Exact finite quotients must not be rounded unnecessarily or padded to fill all available coefficient digits.

**§ 5(2).** If **no exact representation** of the quotient exists within the selected coefficient/scale domain, division may approximate it automatically. Choose the **finest available signed `int32` scale** whose correctly rounded coefficient fits the signed `i64`/`i128` domain and respects the whole-number-part representability requirement of §5(4). This applies both to nonterminating quotients and finite quotients whose exact representation exceeds the available coefficient precision.

**§ 5(3).** Automatic division rounding uses **`RoundingMode.NearestEven`**. Round **once** from the exact mathematical quotient at the selected scale. At an exact midpoint, choose the candidate with an even final coefficient. If rounding carries into a nonrepresentable coefficient, try the next coarser admissible scale; never wrap, saturate, use a host floating-point intermediate or perform double rounding.

**§ 5(4).** The quotient's **whole-number magnitude** takes precedence over fractional precision. Do not coarsen the scale so far that significant whole-number digits are silently discarded merely to obtain an in-range coefficient. The *ordinary carry caused by correctly rounding the fractional part* (for example a value immediately below `1` rounding to `1`) is permitted and is not itself a whole-number coverage failure. If the quotient's whole-number digits cannot be retained by any admissible representation, apart from such a legitimate rounding carry, division fails with the most specific applicable `ArithmeticError` under §7. A zero whole-number part is handled according to available fractional scale, including valid underflow to rounded zero when no nonzero candidate is representable.

**§ 5(5).** When the selected mathematical result is exactly zero, use the §3(6) zero/preferred-scale policy. When a **nonzero** quotient rounds to zero because its magnitude is below the finest representable quantum, that is an **approximated division result**, not an exact zero quotient: retain the actual selected rounding scale and do not reapply the exact-zero preferred-scale policy.

**§ 5(6).** Representative `decimal` results under the signed 64-bit coefficient constraint (assuming each shown result is stored with its selected scale):

| Operation | Value | Result scale | Reason |
|---|---|---:|---|
| `1m / 3m` | `0.3333333333333333333` | 19 | Nonterminating; maximal precision. |
| `2m / 3m` | `0.6666666666666666667` | 19 | Nonterminating; nearest-even rounding. |
| `1000000000m / 3m` | `333333333.3333333333` | 10 | Nine whole digits; remaining coefficient capacity is fractional. |
| `1m / 8m` | `0.125` | 3 | Exactly representable; no unnecessary padding. |

**§ 5(7).** `decimal128` follows exactly the same algorithm with a signed 128-bit coefficient, providing greater available precision without introducing a target-dependent default. Conformance compares results against an independent exact-rational reference evaluator.

**§ 5(8).** A rounded division is an **ordinary successful arithmetic value**, not `PrecisionLoss`, and needs no `try` merely because the quotient is inexact. `try` remains relevant for actual checked failure paths such as a runtime zero divisor or an unrepresentable whole-number part.

**§ 5(9).** This is a specifically approved exception to old P21 wording such as “`1/3` is an error”, “division is exact or fails”, “no implicit rounding”, and “nonterminating division must produce `precision-loss`”. Remove or supersede those statements at their canonical owners; do not leave competing normative meanings in the same revision.

---

## § 6. Division/remainder by zero: compile time versus runtime

**§ 6(1).** When the compiler can **prove** that the divisor of decimal division or decimal remainder is zero, the expression is a **compile-time error**. This applies even if the source uses `try`. Compilation of the affected program must **stop rather than continue into successful code generation or publishing an executable artifact**. The compiler may retain its ordinary diagnostic-collection policy only insofar as it does not resume a successful compilation. Provide a specific explanatory diagnostic with the source location and a repair hint.

**§ 6(2).** If zero cannot be proven or disproven at compile time, the generated program checks the runtime divisor. A zero divisor in the **ordinary** operation reaches the existing non-returning **panic/runtime safety endpoint** with the semantic reason `DivisionByZero`; it is not a resumable exception and does not continue the failed stack.

**§ 6(3).** The corresponding **fallible** expression `try a / b` or `try a % b` turns that dynamic failure into **`ArithmeticError.DivisionByZero`**, through existing typed `Result`/handler propagation, with no panic for that handled failure point. Other `try`/panic effects in operands or handler bodies remain governed by their own rules.

**§ 6(4).** Constant-zero rejection is an explicitly approved decimal-language rule. Reconcile it with `rules/errors/runtime_checks.md` and CTE policy without incorrectly asserting that *every other* provably invalid runtime check everywhere in Sec is automatically a compile error. The narrow decision here concerns a provably zero **decimal divisor**.

**§ 6(5).** Proofs that a divisor is nonzero may remove redundant runtime checks. The compiler must not assume nonzero merely because code uses ordinary division or because a named constant could not be evaluated in an optional CTE context.

---

## § 7. ArithmeticError and the most-specific-error rule (MD-067)

**§ 7(1).** **Do not introduce `DecimalArithmeticError`.** Decimal operators reuse the existing canonical compiler-known **`ArithmeticError`** and its exact nominal identity. Retain its three original variants and append precisely the two newly approved specific variants:

```sec
// Failures of checked built-in arithmetic operations.
enum ArithmeticError error {
    // Arithmetic overflow not described by a more specific applicable variant.
    Overflow

    // Division or remainder was attempted with a zero divisor.
    DivisionByZero

    // A shift count is invalid for the selected integer type.
    InvalidShift

    // No valid signed decimal scale can represent the mathematical result.
    ScaleOverflow

    // Exact decimal digits exceed the selected coefficient's capacity.
    PrecisionExhausted
}
```

**§ 7(2).** The general Sec error-selection principle is **MOST SPECIFIC APPLICABLE ERROR**: an operation reports the narrowest existing variant that **accurately** describes its failure; a generic variant is used only when no more specific applicable variant is defined in the operation's selected public error type. Selection must be deterministic and independent of the order of low-level checks, backend target or optimization. Do **not** invent an implicit conversion between distinct concrete error types or a hierarchy among enum variants.

**§ 7(3).** The applicable decimal operator mapping is:

| Semantic failure | Public error |
|---|---|
| Runtime decimal division/remainder by zero under `try` | `ArithmeticError.DivisionByZero` |
| A finite exact result requires a signed scale outside `int32` and coefficient precision otherwise suffices | `ArithmeticError.ScaleOverflow` |
| A required **exact** result needs more significant coefficient digits than signed `i64`/`i128` can hold at any allowable exact scale | `ArithmeticError.PrecisionExhausted` |
| Other unrepresentable arithmetic outcome for which no narrower variant accurately applies | `ArithmeticError.Overflow` |
| Inexact decimal quotient successfully rounded under §5 | **No failure** |

**§ 7(4).** For an exactly representable finite-decimal mathematical value, classify failure by its minimal exact integer coefficient after stripping all exact factors of ten **for classification only** (this does not license changing an otherwise valid preferred output scale): if the minimal coefficient cannot fit the actual signed coefficient range, `PrecisionExhausted`; otherwise, if no legal signed `int32` scale represents the value, `ScaleOverflow`. A remaining unclassifiable representability failure uses `Overflow`. This ordering reflects mathematical causes, **not** implementation check order. Runtime zero-divisor failure is detected before attempting a quotient or remainder.

**§ 7(5).** For an **approximate division** that cannot produce any §5-compliant coefficient/scale candidate preserving the required whole-number part, classify the actual failed constraint by the same most-specific rule. `ScaleOverflow` is for the scale domain; `PrecisionExhausted` is for insufficient coefficient digits; `Overflow` remains the generic fallback. An ordinary nonterminating quotient is **never** `PrecisionExhausted` or `PrecisionLoss` merely for being nonterminating.

**§ 7(6).** `ArithmeticError.Overflow` is **not** a supervariant and does not match `PrecisionExhausted` or `ScaleOverflow` implicitly. A source author who wants all remaining arithmetic errors handles them explicitly or with `Err(_)`/the established binding forms. Exhaustive `match` over the expanded enum must account for the new members; existing exhaustive matches without coverage will require updating. This is an intentional source-visible enum evolution.

**§ 7(7).** Keep `ArithmeticError` an **enum**, not a `union error` with payloads. No per-error operand strings, desired-precision payloads, implicit allocation, or new exceptions are added. Compiler diagnostics and internal failure reasons may be richer than the public value.

**§ 7(8).** Do **not** change `OverflowError`, `DivisionByZeroError`, `RangeError`, `PrecisionError` or `ConversionError` into aliases of `ArithmeticError`. Their existing typed APIs retain distinct nominal errors. In particular, `decimal.Abs()` keeps `OverflowError`; `Clamp()` keeps `RangeError`; decimal scale/rounding operations keep `PrecisionError`; numeric conversions keep `ConversionError`.

**§ 7(9).** The existing integer `ArithmeticError` cases continue working. Do not emit decimal-specific failures for operations whose type cannot produce them; nevertheless all variants belong to the one **closed enum** and participate in the ordinary exhaustive-match rules. Core compiler-known registration, Semantic IR enum metadata, MLIR reason-to-error mapping, diagnostics, LSP and tests must use the **same canonical variant set and identity**.

---

## § 8. Explicit scale and rounding operations

**§ 8(1).** Preserve the existing core enum `RoundingMode` and **only** its already approved six variants: `NearestEven`, `NearestAwayFromZero`, `TowardZero`, `AwayFromZero`, `Ceil`, `Floor`. Do not add another rounding enum.

**§ 8(2).** `Rescale(scale: int32)` returns a **new** decimal value with exactly the requested scale and unchanged mathematical value, or a `PrecisionError`. It **never** rounds. `value.Scale = scale` has the same exact rescaling criterion but commits to a mutable receiver transactionally.

**§ 8(3).** `RoundTo(scale: int32)` and `RoundTo(scale: int32, mode: RoundingMode)` return a **new** value with **exactly** the requested scale, or a `PrecisionError`. The one-argument form uses `NearestEven`. The selected mode rounds **once** from the original mathematical value; the result does not mutate the receiver.

**§ 8(4).** `PrecisionError` retains exactly its current members and contracts:

| Variant | Applies when |
|---|---|
| `PrecisionLoss` | An operation requiring **exact** scaling would need a non-integer coefficient at the requested scale. |
| `PrecisionExhausted` | The required (exact or once-rounded, as applicable) coefficient at the **explicitly requested scale** cannot fit in the signed coefficient width. |
| `ScaleOverflow` | An operation requires a representational scale outside the signed `int32` domain. A valid explicit `int32` target scale alone does not trigger this case. |

**§ 8(5).** `RoundTo` does not report `PrecisionLoss` merely because it rounds; rounding is explicitly requested. `Rescale` and the `Scale` setter prefer `PrecisionLoss` for a non-integral desired coefficient and `PrecisionExhausted` when the desired exact integral coefficient would overflow. Runtime failure leaves the setter's original receiver untouched. Zero can be rescaled or rounded to every legal `int32` target scale.

**§ 8(6).** `Floor()`, `Ceiling()`, `Truncate()` and `Round()` remain **methods** with no arguments because they transform values rather than observe an intrinsic property. `Round()` uses nearest-even. They return the same decimal type directly and choose a legal exact representation of their whole-number mathematical result, preferring scale zero and falling back to the nearest valid exact scale if necessary.

**§ 8(7).** `Normalize()` is a new, **infallible**, nonmutating method returning the same decimal type. Remove exact trailing coefficient zeros while decreasing the signed scale, stopping at `int32.Min`; do not change the mathematical value. `Normalize()` on any zero returns coefficient zero and scale zero. Normalization is **not** automatically applied to an otherwise valid result that should preserve its preferred or explicitly requested scale.

---

## § 9. Complete approved public member surface (MD-068)

**§ 9(1).** The following lists are **normative public signatures**, not runnable placeholder bodies. In core source, declarations must have valid implementation bodies or the approved compiler-known intrinsic boundary. Keep source/LSP documentation comments **immediately before every public declaration/member**, including union/enum variants where applicable; do not move comments after the member they describe.

**§ 9(2).** `decimal` has exactly the following approved core member family under this correction (plus the language-wide unsafe `.ptr` intrinsic and any independently canonical universal intrinsic members):

```sec
impl decimal {
    // Observes the signed decimal scale or changes it exactly.
    property Scale: int32 {
        get
        try set requestedScale PrecisionError
    }

    // Reports whether the mathematical value is integral.
    property IsInteger: bool { get }

    // Reports whether the mathematical value is zero.
    property IsZero: bool { get }

    // Reports whether the mathematical value is greater than zero.
    property IsPositive: bool { get }

    // Reports whether the mathematical value is less than zero.
    property IsNegative: bool { get }

    // Returns -1, 0, or 1 according to the mathematical sign.
    property Sign: int { get }

    // Formats the invariant, scale-preserving round-trip representation.
    fn ToString() Result[string, StringError]

    // Formats using an explicit invariant format code.
    fn ToString(format: string) Result[string, StringError]

    // Formats using explicit decimal and grouping separators.
    fn ToString(format: string, decimalSeparator: string, groupSeparator: string) Result[string, StringError]

    // Returns the mathematical absolute value, or an overflow error.
    fn Abs() Result[decimal, OverflowError]

    // Returns the lesser numeric operand, retaining this receiver on a tie.
    fn Min(other: decimal) decimal

    // Returns the greater numeric operand, retaining this receiver on a tie.
    fn Max(other: decimal) decimal

    // Restricts the value to an inclusive, validated numeric interval.
    fn Clamp(minimum: decimal, maximum: decimal) Result[decimal, RangeError]

    // Returns the greatest mathematical integer not exceeding this value.
    fn Floor() decimal

    // Returns the least mathematical integer not below this value.
    fn Ceiling() decimal

    // Returns the whole-number value truncated toward zero.
    fn Truncate() decimal

    // Rounds to a mathematical integer using nearest-even.
    fn Round() decimal

    // Rounds to the exact requested output scale using nearest-even.
    fn RoundTo(scale: int32) Result[decimal, PrecisionError]

    // Rounds once to the exact requested scale using the given mode.
    fn RoundTo(scale: int32, mode: RoundingMode) Result[decimal, PrecisionError]

    // Changes output scale exactly, never rounding.
    fn Rescale(scale: int32) Result[decimal, PrecisionError]

    // Removes exact trailing coefficient zeros without changing the value.
    fn Normalize() decimal

    // Converts to target-sized int without inexact rounding.
    fn ToInt() Result[int, ConversionError]

    // Converts to target-sized int using an explicit rounding policy.
    fn ToInt(mode: RoundingMode) Result[int, ConversionError]

    // Converts to fixed-width int32 without inexact rounding.
    fn ToInt32() Result[int32, ConversionError]

    // Converts to fixed-width int32 using the requested rounding policy.
    fn ToInt32(mode: RoundingMode) Result[int32, ConversionError]

    // Converts to fixed-width int64 without inexact rounding.
    fn ToInt64() Result[int64, ConversionError]

    // Converts to fixed-width int64 using the requested rounding policy.
    fn ToInt64(mode: RoundingMode) Result[int64, ConversionError]

    // Converts to fixed-width int128 without inexact rounding.
    fn ToInt128() Result[int128, ConversionError]

    // Converts to fixed-width int128 using the requested rounding policy.
    fn ToInt128(mode: RoundingMode) Result[int128, ConversionError]

    // Converts to target-sized binary float using nearest-even.
    fn ToFloat() Result[float, ConversionError]

    // Converts to target-sized binary float using an explicit rounding mode.
    fn ToFloat(mode: RoundingMode) Result[float, ConversionError]

    // Converts to binary float32 using nearest-even.
    fn ToFloat32() Result[float32, ConversionError]

    // Converts to binary float32 using an explicit rounding mode.
    fn ToFloat32(mode: RoundingMode) Result[float32, ConversionError]

    // Converts to binary float64 using nearest-even.
    fn ToFloat64() Result[float64, ConversionError]

    // Converts to binary float64 using an explicit rounding mode.
    fn ToFloat64(mode: RoundingMode) Result[float64, ConversionError]
}
```

**§ 9(3).** `decimal128` has the **same public members and overloads** with all corresponding receiver/operand/result uses of `decimal` replaced with `decimal128`. The full public signature inventory, rather than an unspecified “same as decimal” placeholder, is:

```sec
impl decimal128 {
    // Observes the signed decimal scale or changes it exactly.
    property Scale: int32 {
        get
        try set requestedScale PrecisionError
    }

    // Reports whether the mathematical value is integral.
    property IsInteger: bool { get }

    // Reports whether the mathematical value is zero.
    property IsZero: bool { get }

    // Reports whether the mathematical value is greater than zero.
    property IsPositive: bool { get }

    // Reports whether the mathematical value is less than zero.
    property IsNegative: bool { get }

    // Returns -1, 0, or 1 according to the mathematical sign.
    property Sign: int { get }

    // Formats the invariant, scale-preserving round-trip representation.
    fn ToString() Result[string, StringError]

    // Formats using an explicit invariant format code.
    fn ToString(format: string) Result[string, StringError]

    // Formats using explicit decimal and grouping separators.
    fn ToString(format: string, decimalSeparator: string, groupSeparator: string) Result[string, StringError]

    // Returns the mathematical absolute value, or an overflow error.
    fn Abs() Result[decimal128, OverflowError]

    // Returns the lesser numeric operand, retaining this receiver on a tie.
    fn Min(other: decimal128) decimal128

    // Returns the greater numeric operand, retaining this receiver on a tie.
    fn Max(other: decimal128) decimal128

    // Restricts the value to an inclusive, validated numeric interval.
    fn Clamp(minimum: decimal128, maximum: decimal128) Result[decimal128, RangeError]

    // Returns the greatest mathematical integer not exceeding this value.
    fn Floor() decimal128

    // Returns the least mathematical integer not below this value.
    fn Ceiling() decimal128

    // Returns the whole-number value truncated toward zero.
    fn Truncate() decimal128

    // Rounds to a mathematical integer using nearest-even.
    fn Round() decimal128

    // Rounds to the exact requested output scale using nearest-even.
    fn RoundTo(scale: int32) Result[decimal128, PrecisionError]

    // Rounds once to the exact requested scale using the given mode.
    fn RoundTo(scale: int32, mode: RoundingMode) Result[decimal128, PrecisionError]

    // Changes output scale exactly, never rounding.
    fn Rescale(scale: int32) Result[decimal128, PrecisionError]

    // Removes exact trailing coefficient zeros without changing the value.
    fn Normalize() decimal128

    // Converts to target-sized int without inexact rounding.
    fn ToInt() Result[int, ConversionError]

    // Converts to target-sized int using an explicit rounding policy.
    fn ToInt(mode: RoundingMode) Result[int, ConversionError]

    // Converts to fixed-width int32 without inexact rounding.
    fn ToInt32() Result[int32, ConversionError]

    // Converts to fixed-width int32 using the requested rounding policy.
    fn ToInt32(mode: RoundingMode) Result[int32, ConversionError]

    // Converts to fixed-width int64 without inexact rounding.
    fn ToInt64() Result[int64, ConversionError]

    // Converts to fixed-width int64 using the requested rounding policy.
    fn ToInt64(mode: RoundingMode) Result[int64, ConversionError]

    // Converts to fixed-width int128 without inexact rounding.
    fn ToInt128() Result[int128, ConversionError]

    // Converts to fixed-width int128 using the requested rounding policy.
    fn ToInt128(mode: RoundingMode) Result[int128, ConversionError]

    // Converts to target-sized binary float using nearest-even.
    fn ToFloat() Result[float, ConversionError]

    // Converts to target-sized binary float using an explicit rounding mode.
    fn ToFloat(mode: RoundingMode) Result[float, ConversionError]

    // Converts to binary float32 using nearest-even.
    fn ToFloat32() Result[float32, ConversionError]

    // Converts to binary float32 using an explicit rounding mode.
    fn ToFloat32(mode: RoundingMode) Result[float32, ConversionError]

    // Converts to binary float64 using nearest-even.
    fn ToFloat64() Result[float64, ConversionError]

    // Converts to binary float64 using an explicit rounding mode.
    fn ToFloat64(mode: RoundingMode) Result[float64, ConversionError]
}
```

**§ 9(4).** The universal unsafe pointer getter continues to exist for addressable values of both types under the ordinary intrinsic `.ptr` rules. This correction does **not** introduce a type-specific public coefficient pointer, mutable raw coefficient property, or a second `Ptr` implementation. The actual compiler-known registry remains the owner of intrinsic member identity and case-migration aliases.

**§ 9(5).** `IsInteger`, `IsZero`, `IsPositive`, `IsNegative`, `Sign` and `Scale` are **properties**, read without parentheses. A query of value state that requires no parameter is normally a property; a transformation is normally a method even when it has no arguments (`Normalize()`, `Round()`, `Abs()`, etc.). Property getter computations do not by themselves imply a setter.

**§ 9(6).** `Min(other)` and `Max(other)` compare numeric value and return one operand's **unchanged representation**. On mathematical equality, **the receiver wins**. `Clamp(minimum, maximum)` uses an inclusive interval, returns `RangeError.InvalidRange` for `minimum > maximum`, returns the receiver unchanged inside the range **including equality at a bound**, otherwise returns the selected bound with its original coefficient/scale. Two numerically equal bounds are valid.

**§ 9(7).** `Abs()` preserves the original scale and returns `Result[..., OverflowError]` because the minimum negative coefficient may have no positive counterpart in the signed coefficient domain. It does not silently change scale to evade an error. `Min`/`Max` are infallible, and `Clamp` is fallible **only** under its established range contract.

---

## § 10. Conversion and formatting contracts

**§ 10(1).** Use **`ToInt*` and `ToFloat*` exclusively** for these decimal convenience conversions; no redundant `AsInt`, `AsInt32`, `AsInt64`, `AsFloat` or `AsFloat*` public aliases are introduced by this correction. A `To*` method may accept optional rounding policy parameters; a public `As*` read-only property cannot accept parameters.

**§ 10(2).** `ToInt()` returns target-sized `int` (32 bits on a 32-bit target, 64 bits on a 64-bit target); `ToInt32`, `ToInt64` and `ToInt128` have fixed-width results. Without a `RoundingMode`, these methods require an **exact integral value**, or return the appropriate `ConversionError` (`PrecisionLoss` or `OutOfRange`). With a mode, compute the integer by one rounding of the exact mathematical input and then range-check against the actual target width; range testing occurs **after** rounding.

**§ 10(3).** `ToFloat()` returns target-sized binary `float` according to MD-014, while `ToFloat32` and `ToFloat64` have fixed binary widths. These methods use nearest-even by default or the explicit `RoundingMode` overload. Compute the conversion from the exact decimal rational, not from a truncated intermediate or a host float. Unrepresentable target values fail through existing `ConversionError` rather than silently selecting infinity. Underflow, if representable as rounded zero or a subnormal under the selected mode, follows that mode's binary target rules.

**§ 10(4).** These methods do **not** redefine the existing explicit `TargetType(value)` / `TargetType(value, mode)` conversion grammar. Maintain its canonical exactness and error rules, and reconcile any separate syntax-level conversion family with these convenience methods explicitly; do not infer an implicit cast merely from a `To*` member.

**§ 10(5).** Retain exactly the approved three `ToString` overloads in §9. Their formatting is deterministic and independent of host locale:

| Format | Meaning |
|---|---|
| `R` (also default `ToString()`) | Invariant, exact, **scale-preserving** round-trip representation. Use exponent notation when required to avoid pathological size; restoring the value must restore `.Scale`. |
| `G` | General, compact numeric representation; redundant decimal trailing zeros need not be preserved. |
| `F<n>` | Fixed number of fractional digits for display. |
| `N<n>` | Fixed digits with grouping of three whole-part digits. |
| `E<n>` | Scientific notation with the specified display precision. |

**§ 10(6).** `ToString(format, decimalSeparator, groupSeparator)` uses explicit `string` separators, including a possibly empty grouping separator. Examples:

```sec
let value := 1234567.50m
let us := try value.ToString("N2", ".", ",")
let eu := try value.ToString("N2", ",", ".")
let spaced := try value.ToString("N2", ",", " ")
// us     == "1,234,567.50"
// eu     == "1.234.567,50"
// spaced == "1 234 567,50"
```

**§ 10(7).** Formatting with a requested fractional precision may round the **rendered display text** using nearest-even; it never mutates the decimal value or changes `.Scale`. Other explicit display rounding policies may be obtained by applying `RoundTo()` first. `R` never loses numeric or scale information. Unsupported or invalid format strings follow the existing `StringError`/`FormatError` contract.

**§ 10(8).** `string(numericValue)` continues to use the exact same invariant default formatting and `Result[string, StringError]` contract as `numericValue.ToString()`, per MD-051. No host locale, hidden allocation exception or divergent `decimal128` formatting rule is introduced.

**§ 10(9).** The owner has expressed **interest in `Parse()`** for both decimal types, but its precise static/member location, public signatures, error identity, parser grammar and allocation behavior were **not** approved in the MD-066/067/068 discussion. `Parse()` is expressly **deferred to a separate API decision** and is **not** to be fabricated as an executable public core requirement under this correction. Likewise, unsigned `ToUInt*` overloads are deferred; do not infer them from the `ToInt*` family. The `R` contract can be documented independently of claiming that a new `Parse()` implementation already exists.

---

## § 11. Diagnostic/example contract for Scale and RoundTo

**§ 11(1).** This example illustrates the approved transactional `Scale` setter and the difference between exact rescaling and chosen rounding. It also shows partial `try` handling: errors other than `PrecisionLoss` continue through the ordinary propagation channel, and the handler's inner `try` is independent.

```sec
fn SetPreferredScale(value: decimal) Result[decimal, PrecisionError] {
    let mut result := value

    try result.Scale = 2 {
        Err(PrecisionError.PrecisionLoss) => {
            result = try result.RoundTo(2)
        }
    }

    return Ok(result)
}
```

**§ 11(2).** `try result.Scale = 2` succeeds exactly for e.g. `12.50` but fails with `PrecisionError.PrecisionLoss` for `12.53`. On that failure, `result` remains `12.53` until the explicit `RoundTo(2)` succeeds and its returned value is assigned. An `Err` from `RoundTo` is not caught by the outer `try`'s handler. Do not document the incorrect assumption that `RoundTo()` mutates its receiver.

**§ 11(3).** Because the `Scale` setter is fallible, a call without `try` is invalid. Because its error type is `PrecisionError`, the setter is not declared as an infallible `set`, and an ordinary getter cannot fail in Sec 0.1.

---

## § 12. Semantic IR, Sec MLIR, compiler-known and lowering synchronization

**§ 12(1).** `rules/compiler/compiler_known_members.md` and `internal/sema/compiler_known_members.go` must publish one canonical `decimal.Scale` / `decimal128.Scale` identity of type `int32`, including getter and fallible setter semantics. Correct legacy `uint8`/`int` metadata, LSP hover/completion and historical aliases without duplicating the member. Ensure property setter lowering has the established checked transaction behavior.

**§ 12(2).** All `decimal128` methods from §9 must resolve through core/normal member lookup with the same actual signatures as `decimal`; do not advertise them solely in an LSP list without implementation or gate-aware diagnostic support. Add `sec/core/decimal128.sec` (currently absent at the reviewed HEAD) as appropriate to the trusted core source organization. The current `sec/core/decimal.sec` contains a `ToString()` stub; retain its fallible signature and distinguish implementation TODO from normative behavior.

**§ 12(3).** Sema's immutable resolved decimal plans must carry: concrete decimal type/width, exact operands and mathematical result basis, preferred scale, selected actual scale, exact vs **rounded-division** classification, rounding mode, divisor-zero proof classification, full specific failure reason, typed error identity, source provenance and `try`/panic effects. The builder may not reconstruct these decisions differently at later stages.

**§ 12(4).** Semantic IR must preserve the chosen scale and approximation status; a nonterminating quotient is a successful value when §5 permits. MLIR verifiers and lowering must reject a representation that silently uses float, loses the chosen scale, re-rounds the quotient, fails to guard an error candidate, or turns `ArithmeticError` into a second nominal decimal error type.

**§ 12(5).** The current P21 provisional `!sec.decimal_failure_reason` mechanism may continue to be a **compiler-internal** exact failure-reason domain; it does **not** imply a source-level `DecimalArithmeticError`. If v17 remains the intended successor schema, update its mapping op, reason enumerants, verifier, enum identity and tests to construct **`ArithmeticError`** with five canonical public variants. A dedicated decimal internal reason can distinguish precision versus scale failures before mapping, but **nonterminating or rounded-success division must not set the failure flag**. Do not claim a schema-17 implementation before resolving actual predecessor schemas/implementation gates.

**§ 12(6).** Synchronize the affected P21 documents: `rules/mlir/packages/sec-mlir-dialect_package21.md`, `rules/mlir/packages/sec-mlir-dialect_package21.yaml`, `rules/mlir/normative-sync/sec_decimal_numeric_sync_package21.md`, `rules/mlir/semantic-ir/sec_semantic_ir_decimal_package21.md`, `rules/mlir/dialect-versions/sec_mlir_dialect_v17.md`, `rules/mlir/lowering-versions/sec_mlir_lowering_v17.md`, `package21-todo.md`, and related canonical `sec_mlir*` books. Replace contradictory unapproved assumptions in place; do not simply append a second, conflicting policy.

**§ 12(7).** The generic builtin arithmetic error mapping (currently `Overflow`, `DivisionByZero`, `InvalidShift`) must grow to include **`ScaleOverflow`, `PrecisionExhausted`** everywhere the complete canonical enum is registered, verified, materialized or matched. Reuse the existing `core::ArithmeticError` identity and canonical enum path. Do not add a parallel core enum definition or change it to union layout. Update numeric/error effect analysis, error pattern exhaustiveness, any compiler-generated rejection/diagnostics, and targeted regression tests.

**§ 12(8).** The revised decimal division exception must not accidentally alter the existing exactness policy of integer operations, checked unit conversions, fixed `TargetType(value)` conversions, or non-division decimal operators. Explicitly test this separation across Sema, CTE, Semantic IR and lowering.

---

## § 13. Governance and implementation sequence

**§ 13(1).** Update `missing-decisions.yaml` entries **MD-066, MD-067 and MD-068 in place** from `open` to the project's canonical *design decided / implementation pending* state after the accepted correction is installed. Preserve their IDs, descriptions/history and reference the correction. Do not mark implementation complete merely on document acceptance.

**§ 13(2).** Update the **existing** P21 D1/D2/D3 decision gate entries in the package YAML to reference this accepted decision and release only the associated *normative* blocks. Retain all actual implementation dependencies and unfinished P15–P20 work. No duplicate `SEC-MLIR-P21-D#` or new parallel governance series is authorized.

**§ 13(3).** The implementation agent must execute, test and report **one numbered step at a time**:

1. **Baseline/inventory:** verify `main` HEAD, current schema gates, current code/Tests and governance ownership; record discrepancies without altering semantics.
2. **Canonical rules:** synchronize types/operators/runtime checks/core/rounding/properties/error-selection rules and P21 normative sync, preserving stable section identifiers.
3. **Core/API:** update `ArithmeticError` enum metadata and trusted core declarations; implement/plan public scale getter/try-set, precision and decimal128 parity, preserving existing core symbols.
4. **Frontend/CTE:** validate literal scales, exact operator scale plans, division rounding, provable zero-divisor diagnostics, comparison/remainder, errors, target-width independence.
5. **Semantic IR:** carry immutable scale, exactness/rounded status and precise error CFG plans; verify success/failed branches and typed `try`.
6. **Sec MLIR and lowering:** revise schema-gated decimal operations, the canonical error mapping, verifier proofs and deterministic helper/lowering semantics, without skipping schema prerequisites.
7. **Core formatting/conversions:** implement signatures and precise rounding/formats within their correctly owned core/std boundaries; do not mark stubbed routines complete.
8. **Conformance/governance closeout:** execute independent differential tests and the target plan matrix; update existing YAML entries and package TODOs with **observed** results, leaving unimplemented items remaining.

**§ 13(4).** A step is not complete until focused tests pass or the agent reports the exact blocking capability and leaves unsupported paths explicitly rejected. Do not batch all steps into one unchecked change. A design-approved file is never a license to fabricate implementation status or bypass error handling, ownership, schema, or target capability checks.

---

## § 14. Required conformance and regression matrix

**§ 14(1).** Minimum decimal tests, independently verified against exact integer/rational reference mathematics, shall cover:

| ID | Required test class | Expected invariant |
|---|---|---|
| DEC-01 | `decimal` and `decimal128` fixed layout on 32/64-bit target plans | `{i64,i32}` / `{i128,i32}`; `.Scale: int32` in both plans. |
| DEC-02 | Negative/zero/positive and `int32`-boundary scales | No `uint8` restriction, no host-width overflow, bounded diagnostics. |
| DEC-03 | Exact literal scale and nonzero/zero normalization | Source scale preserved when valid; zero `Normalize()` gets scale zero. |
| DEC-04 | `+`, `-`, `*`, `%` preferred-scale and exact fallback | Closest exact scale, never approximation. |
| DEC-05 | Exact `1/8` division | `.Scale == 3`, value `0.125`, without artificial digit padding. |
| DEC-06 | Repeating `1/3`, `2/3`, `1000000000/3` | Nearest-even at finest scale; no `PrecisionLoss`. |
| DEC-07 | Decimal finite quotient needing rounding despite termination | Automatically rounds when exact result exceeds coefficient precision and whole-part condition holds. |
| DEC-08 | Signed coefficient minima and rounding carries | No symmetric-absolute-max bug, double rounding, wrap or saturation. |
| DEC-09 | Division underflow to zero | Distinguish rounded nonzero quotient from exact zero quotient/scale. |
| DEC-10 | Provably zero decimal divisor, with and without `try` | Compile-time diagnostic; compilation fails. |
| DEC-11 | Runtime zero divisor, ordinary and `try` | Panic endpoint vs `ArithmeticError.DivisionByZero`. |
| DEC-12 | `%` with repeating mathematical quotient | Uses exact truncated quotient, not approximate division result. |
| DEC-13 | `Scale` getter/try setter and `Rescale` | Exact success; on error receiver unchanged; expected `PrecisionError`. |
| DEC-14 | `RoundTo` one/two arguments | Exact requested scale or error; selected rounding mode exactly once. |
| DEC-15 | `Abs` at min negative coefficient | `OverflowError`, not arithmetic enum error; scale preserved on success. |
| DEC-16 | `Min`, `Max`, `Clamp` and equal numeric values with different scales | Numeric ordering; receiver wins ties; clamp retains selected existing representation. |
| DEC-17 | `IsInteger`, `IsZero`, `IsPositive`, `IsNegative`, `Sign` | Property access without `()`; proper bool/sign results; no mutation. |
| DEC-18 | `ToInt*`/`ToFloat*` default and explicit rounding | Exact integer by default; nearest-even float by default; target-sized vs fixed-width correct. |
| DEC-19 | `ToString` three overloads and `R/G/F/N/E` | `R` retains scale; explicit separators; display rounding does not mutate. |
| DEC-20 | `ArithmeticError` exhaustive and wildcard handlers | Five variants; `Err(_)` covers remaining; no implicit variant hierarchy. |
| DEC-21 | Error specificity | Scale vs coefficient precision vs generic overflow are classified deterministically. |
| DEC-22 | Full Sema → Semantic IR → Sec MLIR → lowering differential matrix | Same values/scale/errors across 32/64-bit plans and supported targets; no binary-float decimal backend. |

**§ 14(2).** Add precise negative tests for unsupported old `As*` member names, `decimal128` missing-member regression, attempted infallible `.Scale` assignment, wrong signed scale type, invalid rounding mode, bad formatting, source-level duplicate error types and wrong P21 failure reason/nominal error mapping.

**§ 14(3).** Include const/CTE and runtime-equivalent positive cases, and confirm **compile-time zero-divisor errors do not become successful `Result` values**. An ordinary dynamic zero divisor must not be optimized away or accidentally converted into automatic rounded division.

**§ 14(4).** Compare exact `decimal` and `decimal128` results using arbitrary precision, and implement precision tests without platform `float64` oracle rounding. Verify the **actual** produced `.Scale`, not just printed numeric equality.

---

## § 15. Separate core consistency finding — not a new decimal decision

**§ 15(1).** The repository currently disagrees about integer query members:

| Location | Observed spelling/form at reviewed HEAD |
|---|---|
| `sec/core/int.sec` | `property IsEven`, `IsOdd`, `IsPositive`, `IsNegative` |
| `sec/core/uint.sec` | `fn IsEven()`, `fn IsOdd()` |
| `rules/library/core-library.md` §§7–8 | `fn IsEven()`, `fn IsOdd()` |

**§ 15(2).** Record this as a **distinct follow-up/core-audit inconsistency**, not as an automatic MD-066/067/068 implementation task and not as a fresh duplicate governance ID. Review comparable predicates across numeric types before changing the existing integer/unsigned core rules. The guiding API principle is: intrinsic value-state queries without arguments are normally read-only **properties**; transformations are normally **methods**, even when parameterless. Keep symmetry where the mathematical meaning is valid, not merely to make type member lists identical.

**§ 15(3).** No public `IsEven`/`IsOdd` members are added to `decimal` by this correction. Their meaning is not defined for arbitrary fractional values. The `IsPositive`/`IsNegative` properties **are** approved for `decimal`/`decimal128`.

---

## § 16. Boundaries and completion criteria

**§ 16(1).** The design decisions MD-066, MD-067 and MD-068 are **closed by owner approval**, but their implementation remains pending. The corrected files must not falsely claim P21, P15 or schema/lowering v17 has been implemented or accepted.

**§ 16(2).** `Parse()` is **acknowledged but deferred** pending a separate exact API/error/parser decision. `ToUInt*` remains deferred. General `rational` and `complex` design is outside this correction. The unrelated MD-049 decimal-range default decision is **not** silently resolved by the signed scale decisions here.

**§ 16(3).** Source rules, compiler-known members, Sema, CTE, Semantic IR, Sec MLIR, lowering, LSP, diagnostics, core declarations, tests and governance are synchronized only when all their actual contracts and observed implementation state agree. Preserve previous rulebook paragraph identity and existing stable governance/package IDs.

**§ 16(4).** This correction is the accepted source for **what to implement**. The canonical owning rulebooks become authoritative only after they are actually amended; the implementation and tracked status become complete only after verified code and tests. No replacement of old text with an unimplemented claim is permitted.
