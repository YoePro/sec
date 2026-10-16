# Sec MLIR Dialect — Planned Schema v19

- **Status:** Normative additive amendment (subject to existing MD decisions)
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/sec_mlir_dialect.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P23`

---

## § 1. Version

Target schema `19`, only after predecessor `18` exists. Compiler-generated module marker `sec.dialect_version = 19 : i32` after implementation, not now.

## § 2. String type

Reuse predecessor semantic string type if present; otherwise register one high-level `!sec.string`. Do not duplicate physical string descriptors or alias `!sec.slice<byte>`.

## § 3. Shared attributes

Typed attributes/facts: `sec.string_encoding_proof`, `sec.string_backing_origin`, `sec.string_storage_identity`, `sec.string_byte_extent`, `sec.string_lifetime_dependency`, `sec.string_allocation_context`, `sec.string_scalar_count_proof`, `sec.source_member_id`. Use proper stable metadata not raw address.

## § 4. Literal

`sec.string.literal`: one immutable `!sec.string`, exact UTF-8 byte attribute, source/provenance; no runtime effects.

## § 5. Lengths

`sec.string.rune_len`: string -> `!sec.uint`; `sec.string.byte_len`: string -> `!sec.uint`; `sec.string.signed_rune_len`: string -> `!sec.int` with representability proof.

## § 6. Comparison

`sec.string.compare`: (string,string)->!sec.int; `sec.string.equals`: (string,string)->i1; `starts_with`, `ends_with`: (string,string)->i1; bounded content no allocation.

## § 7. Search

`sec.string.find`: returns `Option[uint]` or P11 Option semantic variant; `sec.offset_unit` required. Public emission gated MD-071 (P23-D1) until unit selected.

## § 8. Subrange

`sec.string.subrange_unchecked`: (string,start,end)->string; only compiler/core private call, proof of bounds and lifetime, byte-unit attrs; no allocation.

## § 9. Concat checked

`sec.string.concat_checked`: piece values/segments + selected `!sec.alloc_context`, returns candidate `!sec.string`, `i1 failed`, canonical `AllocationError`; exact one-semantic-materialization invariant and failure/success dominance.

## § 10. Owning-array materialization

`sec.string.to_array_checked` typed kind byte/char/rune to owning `!sec.dynamic_array<T>`; `sec.string.from_sequence_checked` converts compatible bounded source sequence; exact candidate/error depends on approved public plan and MD-050, not invented source signature.

## § 11. Identity

`sec.string.identity`: preserves immutable string and returns normal `Result[string,StringError]` semantics through P11 Result builder; may optimize away content copy.

## § 12. Rune step

`sec.string.rune_next` (optional) operates on valid UTF-8 string and internal byte cursor with explicit bounds, returns rune + advanced cursor + done; no unguarded read. Full `for` CFG remains dependent on for-loop integration.

## § 13. Validation gate

Optional `sec.string.utf8_validate` is introduced only after MD-050 confirms when input validation is required and exact failure. A byte-sequence validation helper may exist internally without claiming all strings valid.

## § 14. Verification flags

`--sec-verify-strings` checks values/units/provenance. `--sec-verify-string-materialization` checks selected context, single publication, typed AllocationError CFG, lifetime and candidate dominance.

## § 15. No physical lowering

No string LLVM tuple layout, FFI ABI, universal heap, new runtime/copy-on-write or guaranteed pointer shape in schema v19.
