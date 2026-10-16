# P21 Normative Synchronization — Decimal, Precision and October Corrections

- **Status:** Proposed normative amendment — D1–D3 require acceptance
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical repository path (proposed):** `rules/mlir/normative-sync/sec_decimal_numeric_sync_package21.md`
- **Package:** `SEC-MLIR-P21`
- **Reviewed repository:** `YoePro/sec`, `main`, October 16, 2026
- **Repository SHA:** Not independently verifiable through the accessible GitHub view; record exact SHA at implementation time.
- **Predecessor:** `SEC-MLIR-P20` (schema 16 specification)

## § 1. Sources and hierarchy

Reviewed `rules/types/types.md`, `rules/foundations/operators.md`, `rules/memory/layout.md`, `rules/library/core-library.md`, `rules/types/units.md`, `rules/errors/errorhandling.md`, `rules/compiler/compiler_known_members.md`, `rules/compiler/semantic_ir.md`, and current Sema/core files on the October `main`. This is a **proposed amendment**, not an automatic override of existing accepted decisions.

## § 2. Confirmed decimal family

`decimal`/`decimal128` are active exact decimal types. Unsuffixed fraction/exponent defaults to decimal; `m` selects family, not width; direct literal shaping is exact, no binary-float conversion. Layout components: `{i64,i32}` / `{i128,i32}`. Both retain distinct scalar identity.

## § 3. Confirmed core members

`decimal.scale: uint8`, `Abs()->Result[decimal,OverflowError]`, `RoundTo(uint8)->Result[decimal,PrecisionError]`, `Rescale(uint8)->Result[decimal,PrecisionError]`, and the other §10 members remain canonical. Compiler registry's `Scale: int` is a bug. Core's `decimal.ToString() string` stub is inconsistent with `Result[string,StringError]`. Do not silently use either implementation as language authority.

## § 4. Confirmed rounding and errors

Only `RoundingMode` from `sec/core/rounding.sec` is canonical. An explicit `T(value, mode)` conversion rounds once after exact mathematics. Existing `PrecisionError` variants are exactly `PrecisionLoss`, `ScaleOverflow`, `PrecisionExhausted`. `ArithmeticError` exists for generic checked arithmetic; it must not be silently widened or repurposed to hide precision errors. P20's `NumericConversionError` remains the requested checked-conversion type, subject to declaration synchronization.

## § 5. D1 — public decimal scale policy (PROPOSED, requires decision)

**Unresolved:** Current rules do not lock post-operator scale selection, exact normalization policy, or the range that the physical i32 scale may validly contain relative to the public `uint8` property.

**Recommended:** Treat source-valid scale as `0..255`; retain literal scale where representable; for add/sub/remainder prefer max input scale, for multiply prefer the sum, and strip only exact coefficient trailing zeros when required for representability; do not round. For division choose a representable exact scale deterministically using the algorithm in the implementation package. Zero preserves preferred operation scale when valid. Always expose the resulting value's semantic scale as `uint8`.

**Approval required before:** operator-scale-sensitive results, `.scale` correction implementation beyond type, RoundTo/Rescale final behavior, runtime value validation. Record decision ID in rulebook and governance.

## § 6. D2 — decimal operator error type (PROPOSED, requires decision)

**Unresolved:** One decimal operator may fail both arithmetically and by precision/scale; current rulebooks demand one exact error type for `try` and do not identify a canonical combined type.

**Recommended new core declaration (requires explicit approval):**

```sec
enum DecimalArithmeticError error {
    Overflow
    DivisionByZero
    PrecisionLoss
    ScaleOverflow
    PrecisionExhausted
}
```

Fallible naked decimal arithmetic propagates exactly this type. Existing `OverflowError` and `PrecisionError` method contracts stay unchanged. No inferred error unions or arbitrary wrapping. Recommend deterministic failure priority: zero divisor, impossible scale, coefficient overflow, precision loss. Confirm variant set and priority before coding.

## § 7. D3 — decimal128 public members (PROPOSED, requires decision)

Core §10 documents the full `decimal` surface but not the complete `decimal128` public surface. Recommend an exact mirrored `impl decimal128` member set (appropriate `decimal128` result/argument types), including `scale: uint8`, fallible `Abs`, `RoundTo`, and `Rescale`, with comments preceding each public declaration. **Do not invent or advertise those APIs before approval.** Ordinary active `decimal128` scalar operators/conversions are unaffected.

## § 8. Exact literal and arithmetic intermediate

The builder must retain exact coefficient/exponent/scale source facts and use arbitrary-precision intermediate arithmetic. No Sema `int64` truncation for `decimal128`. A permitted operation returns an exact mathematical decimal only when a representable coefficient/scale pair exists; unchecked approximation is not permitted. `1/3` is not a finite base-10 quotient; its failure is not “out of memory” or an implicit rounded success.

## § 9. October float compatibility notice

The October MD-014 correction makes **plain `float` target-sized** (f32 on a 32-bit platform, f64 on a 64-bit platform), superseding P20's hardcoded f64 statement. Fix P20 dependent float/decimal conversion plans and test target matrix; do not modify unrelated already accepted rounding policy. Track this correction separately as part of the P20 follow-up, not as a new decimal language feature.

## § 10. Safe application order

Merge approved D1–D3 into their owning canonical books without renumbering existing section/paragraph IDs. Update compiler-known member metadata and core declarations, then the Semantic IR rulebook, dialect snapshot, lowering snapshot, and only then the implementation package/YAML status. Do not overwrite canonical schema 11 directly with v17 without satisfying and integrating intermediate package amendments. Preserve legacy YAML history and avoid duplicate governance IDs.
