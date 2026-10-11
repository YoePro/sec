# Correction: SEC-MLIR-P15 place and direct-reference synchronization

- **Status:** Applied 2026-10-11
- **Created:** 2026-10-11
- **Last updated:** 2026-10-11
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Source authority:** `rules/mlir/normative-sync/sec_reference_sync_v1.md`,
  `rules/mlir/semantic-ir/sec_semantic_ir_place_reference_v1.md`,
  `rules/mlir/dialect-versions/sec_mlir_dialect_v10.md`,
  `rules/mlir/dialect-versions/sec_mlir_dialect_v11.md`,
  `rules/mlir/lowering-versions/sec_mlir_lowering_v10.md`,
  `rules/mlir/lowering-versions/sec_mlir_lowering_v11.md`
- **Targets:** `rules/errors/runtime_checks.md`, `rules/memory/reference_model.md`,
  `rules/compiler/semantic_ir.md`, `rules/mlir/sec_mlir_dialect.md`,
  `rules/mlir/sec_mlir_lowering.md`

## Problem

Package 15 (P15-04, P15-05) requires its normative synchronization and
amendments to be present in the canonical rulebooks. The canonical rulebooks
still contained or lacked:

- `runtime_checks.md` listed "reference generation validation -> ReferenceError"
  as an ordinary fallible operation, implying `try` recovery of a stale ordinary
  direct reference;
- `reference_model.md` § 76(2) only asked the stale-reference panic reason to be
  "distinguished or equivalent" without naming the registered reason;
- `semantic_ir.md` v2 described Places and references conceptually but lacked
  the P15 Place, direct-reference, validity, verifier and printer requirements;
- `sec_mlir_dialect.md` and `sec_mlir_lowering.md` still described schema and
  lowering version 9, although schema 10 (P14) is implemented and schema 11 is
  the P15 target.

## Correction

- `runtime_checks.md`: reference-generation validation removed from the
  fallible-operation table and replaced by its three outcomes (proven: no check;
  ordinary direct reference: check plus `panic.invalid-reference-generation`;
  stable/weak handle: explicit `Option`/`Result`), no `try` recovery, partial
  safety mechanism, no mandatory runtime metadata.
- `reference_model.md` § 76(2): the stable reason is the registered
  `InvalidReferenceGeneration` reason (`panic.invalid-reference-generation`).
- `semantic_ir.md`: §§ 10(5)–10(11), 45(5)–45(13), 46(6)–46(9), 48(6)–48(7),
  96(5) and 108(5) integrate the Place/direct-reference amendment into the
  relevant v2 sections (§ 121(5)); the relationship domain is
  `PotentiallyOverlapping`, matching the implemented `PlaceRelationship`.
- `sec_mlir_dialect.md` and `sec_mlir_lowering.md`: current version 11, version
  history and status extended for v10 and v11, and the v10/v11 sections installed
  as versioned parts. The compiler keeps emitting schema 10 until P15 completes
  schema-11 emission (P15-50); schemas 1-10 remain valid regression inputs.

`references.md`, `panic.md` § 13 and the reference-model epoch-width rules
(§§ 33-34) already matched the synchronization and were not changed.
