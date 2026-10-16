# P20/P21 Compatibility Note — ConversionError after MD-012

- **Status:** Required correction to prior package proposals; already normative, not a decision request
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/normative-sync/sec_p20_p21_conversion_compatibility_package22.md`
- **Replaces:** None; additive package amendment only
- **Package:** `SEC-MLIR-P22`

---

## § 1. Conflict

An August P20 package proposed introducing `NumericConversionError`. The current October repository has **already resolved conversion errors** through MD-012, including concrete canonical declarations in `sec/core/error.sec`. A duplicate error family is incorrect.

## § 2. Required mapping

```text
Old proposed NumericConversionError.OutOfRange    -> ConversionError.OutOfRange
Old proposed NumericConversionError.PrecisionLoss -> ConversionError.PrecisionLoss
Old proposed NumericConversionError.NotFinite     -> ConversionError.NonFinite
```

This mapping is for **runtime conversions only**. Follow full canonical conversion semantics for `ConversionError.InvalidScalar`, `.ScaleOverflow`, or `.Contract(ContractError)` where applicable. Do not change the meaning of arithmetic errors or core `PrecisionError` merely because conversion uses the same underlying number.

## § 3. Correct source error declarations

The core file already declares:

```text
ContractKind enum
ContractError union error: Violation { Kind, DeclarationIndex }
ConversionError union error:
    OutOfRange
    InvalidScalar
    PrecisionLoss
    ScaleOverflow
    NonFinite
    Contract(ContractError)
```

Do not generate duplicates, reorder public variants, replace the union with an enum, or change original error identity.

## § 4. Correct previous plans

Review and correct P20 numeric checked conversion plan, Result type, local try handlers, error mapping ops and tests. Review P21 decimal/integer/float conversion plan similarly; **do not decide its arithmetic error or scale policy here**. Preserve P20's other October corrections, including target-dependent `float` width from MD-014.

## § 5. Implementation status

These are specification synchronization requirements. They are not proof P20/P21 were implemented, and they are not new unresolved D# issues.
