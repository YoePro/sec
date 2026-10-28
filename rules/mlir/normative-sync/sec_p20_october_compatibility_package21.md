# P21 Compatibility Audit — October Changes Affecting P20

- **Status:** Required compatibility audit (not a separate language decision)
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical repository path (proposed):** `rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md`
- **Package:** `SEC-MLIR-P21`
- **Reviewed repository:** `YoePro/sec`, `main`, October 16, 2026
- **Repository SHA:** Not independently verifiable through the accessible GitHub view; record exact SHA at implementation time.
- **Predecessor:** `SEC-MLIR-P20` (schema 16 specification)

## § 1. Why the compatibility audit exists

August Package 20 remains a historical implementation package, but early October/October 13 language updates supersede some of its assumptions. P21 must not accidentally reintroduce P20's stale assumptions through decimal↔float conversions.

## § 2. MD-014 target-sized float correction

`rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` §6 explicitly states `float` follows platform bitness: **f32 on 32-bit, f64 on 64-bit**. The P20 package says `float` always has binary64 semantics. Fix P20's Sema scalar plan, `!sec.float` resolution, exact-conversion paths, tests and docs using MD-014 as authority. Do not create an independent default-float width parameter.

## § 3. MD-047 rounding-mode correction

`rules/types/units.md` “Explicit rounding” and `sec/core/rounding.sec` already provide six modes and `T(value, mode)` typed conversion syntax. P21 must extend existing conversion plans to carry explicit mode; P20's old exact-only checked conversion rule is still the no-mode default. Do not implicitly apply nearest-even to a typed numeric conversion.

## § 4. Core error declaration reconciliation

P20 declares `NumericConversionError` in its sync proposal, but accessible `sec/core/error.sec` has no such enum. Verify current live checkout before re-adding; reconcile exact core error declarations/registry and their comments, without duplicated nominal error types or unacknowledged aliases. `PrecisionError` already has three variants and is not a substitute for an unspecified operator error.

## § 5. Dialect version and rulebook-path reconciliation

GitHub `rules/mlir/packages/` contains P20 (v16 package). Canonical `rules/mlir/sec_mlir_dialect.md` and `sec_mlir_lowering.md` presently declare **v11**, with v12–v16 package amendments kept separately. Avoid treating the package manifest as proof that dialect v16 is active. Merge package amendments in order and preserve stable headings/compatibility snapshots.

## § 6. Audit acceptance

```text
[ ] P20 float native-width correction applied
[ ] target 32/64 tests
[ ] RoundingMode conversion-mode tests
[ ] NumericConversionError core identity synchronized
[ ] source-visible .scale int-vs-uint8 bug identified/fixed in P21 (target type is now int32 with try set, MD-066 2026-10-28)
[ ] dialect v16 installed before claiming v17 emission
[ ] current repository SHA recorded and evidence preserved
```

Audit is not a replacement for the canonical rulebooks; changes belong in their owning books and governance fragments.
