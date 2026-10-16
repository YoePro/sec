# Sec MLIR Dialect — Schema v18 (Named Contracts)

- **Status:** Target schema specification; dependent on v17 being installed
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/dialect-versions/sec_mlir_dialect_v18.md`
- **Replaces:** None; additive package amendment only
- **Package:** `SEC-MLIR-P22`

---

## § 1. Version prerequisite

This is the delta from **v17 to v18**. Source `rules/mlir/sec_mlir_dialect.md` may still describe an earlier *implemented* schema; do not promote v18 without actual v17 predecessor compatibility. Compiler-generated v18 modules shall carry:

```mlir
sec.dialect_version = 18 : i32
```

## § 2. Reused types

Use existing `!sec.named<...>` / `!sec.distinct<...>` as appropriate to canonical named type; P11 ordinary enum and union types for `ContractKind`, `ContractError`, `ConversionError`; existing `!sec.result` for checked flow. **No** `!sec.contract_error`, `!sec.conversion_error`, or duplicate source-facing error wrapper.

## § 3. Minimal new operation: `sec.contract.check`

Input: a semantically valid **underlying** typed value plus resolved plan information (no AST). Output: `i1` meaning **contract accepted**. Required metadata: `sec.contract_kind`, `sec.contract_origin_type_id`, `sec.contract_declaration_index`, `sec.contract_predicate_plan_id`, `sec.contract_proof`, `sec.source_location`. Optional typed comparison operands come from CTE plan and are immutable. Reject unknown predicate kinds/unsupported validation families.

## § 4. Nominal construction

Introduce `sec.contract.validated_named` only if existing named wrap/construct op cannot express the success-precondition. Input underlying T; output canonical nominal T. Require provenance that all intrinsic/contract guards dominate the operation, or a concrete static proof for a complete known value. No fallible source declaration is implicit in this op itself.

## § 5. Error constructors

Prefer existing `sec.enum.constant` plus `sec.union.construct` to form actual `ContractError.Violation` and `ConversionError.Contract`. A dedicated `sec.contract.violation` shorthand is **not required**; if added it must canonicalize exactly to those values and may not define a new public error type.

## § 6. Guard structure

Compiler-generated CFG:

```text
intrinsic-check
    failed -> ConversionError intrinsic variant
    success -> contract-0.check
contract-N.check
    false -> Violation(Kind, DeclarationIndex) -> ConversionError.Contract
    true -> next or nominal construct
```

Use standard `cf.cond_br`/`cf.br` and typed block arguments. Existing Result/try handler verifiers remain authoritative.

## § 7. Predicate families

Concrete lowering may reduce `range`, parity/divisibility and `finite` to Arith predicates; string rune/byte length and collection logical count to their canonical compiler-known observations. `unique` uses only a Sema-resolved equality/iteration plan. Regex must not carry `proof=validated` before MD-010 semantic rulebook exists.

## § 8. Readiness and provenance

Source Kind, declaration ordinal, declaring TypeID and plan identity are compiler metadata; user-visible error payload remains only `Kind` and `DeclarationIndex`. No backend may manufacture a plan by scanning source-level type spelling.

## § 9. Verifier pass

`--sec-verify-named-contracts` rejects invalid nominal construction, wrong order/payload, a guard not dominating its error branch, unproven suppression, wrong core error identity, unsupported/regex success, wrong source value, and early destination commit.

## § 10. Parse/print and backward compatibility

All new ops and attrs require deterministic custom/generic parse-print and verification. Old schema v1–v17 regression inputs remain valid under declared compatibility policy. Unknown dialect versions fail closed. Do not alter existing numbered schema paragraphs; append v18 section in canonical rulebook.

## § 11. No runtime or physical layout

No mandatory object metadata or contract runtime table; no nominal data field; no LLVM exception/contract trap. The contract is validated semantically; representation may be erased only when proof is retained.
