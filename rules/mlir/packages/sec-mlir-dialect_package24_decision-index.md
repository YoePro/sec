# P24 Decision Index — Existing Gates and New-Decision Policy

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec-mlir-dialect_package24_decision-index.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P24`

---

## § 1. No newly discovered language decision

P24 found **no genuinely new normative source-language decision** beyond existing tracked gates. Therefore there are **zero new P24-D# files**. Do not generate artificial decisions to fill the numbering series.

## § 2. Existing MD-019

Explicit source syntax for packing/align/field offsets. The layout service can accept already-resolved constraints now; new source spelling remains blocked pending MD-019.

## § 3. Existing MD-071 (P23-D1)

Public string offsets/rune vs byte boundaries: no assumption from physical string descriptor choice.

## § 4. Existing MD-072 (P23-D2)

String/array materializing constructor signatures/ownership/error contracts: no implicit ABI/descriptor inference.

## § 5. Existing MD-073 (P23-D3)

Char domain conversion policy: no byte-level truncation shortcut.

## § 6. Existing MD-050 and P21

Invalid UTF-8 remains MD-050. Decimal precision/scale/API decision status P21-D1–D3 must be checked against mapped MD numbers (MD-066, MD-067, MD-068); no re-decision. October MD-012/MD-014 override obsolete P20 text.

## § 7. Decision file policy going forward

If implementation reveals a **new** unresolved fork not covered by above MD entries, add individual `P24-D1.md`, then `P24-D2.md`, etc., with normative evidence and explicit owner approval gate; never silently change layout semantics. The absence of a new file today is intentional.
