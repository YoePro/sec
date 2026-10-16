# Sec MLIR Dialect — Schema v17 Decimal Amendment

- **Status:** Proposed schema v17 amendment (not yet active)
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical repository path (proposed):** `rules/mlir/dialect-versions/sec_mlir_dialect_v17.md`
- **Package:** `SEC-MLIR-P21`
- **Reviewed repository:** `YoePro/sec`, `main`, October 16, 2026
- **Repository SHA:** Not independently verifiable through the accessible GitHub view; record exact SHA at implementation time.
- **Predecessor:** `SEC-MLIR-P20` (schema 16 specification)

## § 1. Version and prerequisites

Schema 17 follows schema 16 from P20; currently canonical `rules/mlir/sec_mlir_dialect.md` still advertises schema 11. Before changing the current canonical schema, merge/verify intervening packages and retain old numbered clauses and snapshots. Compiler-generated v17 module declares `sec.dialect_version = 17 : i32` only after P21 passes.

## § 2. Reused decimal types

Reuse existing `!sec.decimal` and `!sec.decimal128` type identities from earlier schema. Do not redeclare or represent them as `f64`, as IEEE 754 decimal128, or as `!llvm.struct` within the high-level dialect.

## § 3. Failure reason type (D2-gated)

Proposed `!sec.decimal_failure_reason` enum values: `none`, `overflow`, `division-by-zero`, `precision-loss`, `scale-overflow`, `precision-exhausted`. The single error mapping op may exist only after DecimalArithmeticError is approved. Failed flag and reason must agree.

## § 4. Required operations

```text
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
sec.decimal.error_from_reason
```

The final op is D2-gated; core method op availability is gated by D3 for decimal128.

## § 5. Decimal constant

`sec.decimal.constant` has no operands and returns one `!sec.decimal` or `!sec.decimal128`. Typed attributes carry exact base-10 coefficient in unbounded canonical signed-decimal textual form, semantic scale, original literal spelling and proof that the value is representable in chosen target. Verifier rejects overflow and invalid scale. No host-float conversion.

## § 6. Unary/binary checked

Unary kinds `plus`, `neg`; binary kinds `add`, `sub`, `mul`, `div`, `rem`. Checked binary result shape: `(decimal candidate, i1 failed, !sec.decimal_failure_reason reason)` **after D2**. Operator attrs include immutable Sema-resolved type, scale-policy identity D1, source operation/provenance, and resolved failure error ID. No implicit widening between decimal kinds.

## § 7. Comparison

`sec.decimal.cmp` has predicates `eq`, `ne`, `lt`, `le`, `gt`, `ge`; returns `i1`. Exact comparison ignores representational scale for numeric order; it must not lower to `arith.cmpf` or compare raw composite fields.

## § 8. Scale/core methods

`sec.decimal.scale` returns the type from canonical core declaration (`uint8` for documented decimal), *not the physical i32*. `sec.decimal.core` contains a closed compiler-known ID and exact result/error signature from the accepted registry. Method spelling alone is not its semantic identity.

## § 9. RoundTo/Rescale

Use separate total checked operations for the documented `decimal` signatures, output candidate/error `PrecisionError`; require accepted D1 details. No new public implicit rounding parameters. Explicit typed conversions reuse the existing `RoundingMode` enum and one-rounding contract.

## § 10. Numeric conversion

Conversion ops carry concrete source/target types, checked/exact classification, explicit optional mode, P20 NumericConversionError source identity, and result guard. Do not create `DecimalConversionError` as another unrelated family. Source `float` width is resolved by active CompilationPlan (MD-014), not the old P20 fixed-f64 assumption.

## § 11. Parsing and verification

Required parse/print/verify round-trips, type/scale validity, no implicit signless erosion, reason/failed parity, fallible candidate guard, error nominal identity, source provenance, and exact numeric equivalence. Provide negative fixtures for missing mode, unsupported methods, unapproved D1/D2 IDs, and wrong 128-bit values.

## § 12. No physical layout in schema v17

Physical component layout remains owned by `rules/memory/layout.md`. These ops are high-level; no direct LLVM lowering, no silent helper ABI, no guessed struct offsets, and no backend-chosen rescaling.
