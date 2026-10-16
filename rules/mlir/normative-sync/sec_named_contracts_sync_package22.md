# Package 22 Normative Synchronization — Named Contracts and Conversion Errors

- **Status:** Normative sync proposal; preserves already locked decisions
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/normative-sync/sec_named_contracts_sync_package22.md`
- **Replaces:** None; additive package amendment only
- **Package:** `SEC-MLIR-P22`

---

## § 1. Do not replace contracts.md

`rules/types/contracts.md` revision 2.1 (2026-10-09) is the canonical named-contract rulebook. This file states **implementation synchronization**, not a competing complete copy. Apply focused amendments to `rules/compiler/semantic_ir.md`, `rules/mlir/sec_mlir_dialect.md`, `rules/mlir/sec_mlir_lowering.md`, relevant frontend plan documentation, and governance without renumbering old normative paragraphs.

## § 2. Already locked source ownership

Contracts belong only to named types. Contract/default operand expressions requiring compile-time facts execute through canonical semantic CTE. Named type identity is preserved after validated construction; variable-level/field-level inline contracts remain invalid.

## § 3. Existing public conversion error

MD-012 (2026-10-08) is authoritative. `sec/core/error.sec` owns `ContractKind`, `ContractError` and `ConversionError` with exact members. The compiler must reuse them; no new declarations are authorized. `ConversionError.Contract(ContractError.Violation { Kind, DeclarationIndex })` represents the first failed declared contract after intrinsic conversion succeeds. Intrinsic failures take precedence.

## § 4. Ordinal and provenance

`DeclarationIndex` is zero-based within the *declaring named-type contract list*. Resolved inherited contract checks retain their defining declaration identity in compiler metadata; public error payload does not gain extra fields. Source locations provide diagnostics. Runtime evaluation follows canonical source/effective contract order and stops on first failure.

## § 5. Runtime checking

One evaluation of source expression, intrinsic conversion first, ordered contract checks second, final nominal construction last. Static proven cases may elide checks without changing first failure. Fallible assignment commits only after total success. No hidden panic or GC. `try` uses normal Result and local-handler rules.

## § 6. String units

String `Len` is number of Unicode scalar values, not UTF-8 byte length or grapheme clusters. Value-form string `SizeOf` is number of UTF-8 bytes, not descriptor layout and not terminator-inclusive. String byte contracts are independent of rune contracts; do not restore `.ByteLen` or `.RuneLen`. Invalid UTF-8 handling remains existing MD-050 if still open in current governance.

## § 7. Collection contracts

`unique` requires equality-comparable direct elements and compares them semantically (not pointer/hash identity, not recursive uniqueness). Length constraints count direct logical elements. Implementation readiness follows actual per-family Semantic IR support; backend cannot silently approve an unsupported source value.

## § 8. Regex remains MD-010

The `regex` contract syntax is parsed and retained. Without an accepted dedicated regex rulebook, no frontend or backend may claim successful semantic validation. Do not create P22-D1 merely to rename MD-010. `require` is not Sec 0.1 contract grammar.

## § 9. Cross-package correction

P20's old proposal to introduce `NumericConversionError` is incompatible with MD-012. Synchronize that proposal to `ConversionError` and map source-specific numeric failures to its exact members. The proposed P21 arithmetic-error design questions stay separate from conversion. Floating `float` width must continue to follow October CompilationPlan rules, not August hardcoded binary64.

## § 10. Governance

Reuse `governance/types.yaml` entry `frontend.type-contracts-v2` and existing stable status IDs. Preserve evidence, bugs and currently implemented legacy behavior; add deltas only. Blockers are documented explicitly and tested as expected unsupported diagnostics.
