# Correction: P16 slice Semantic IR and schema-12 installation

- **Status:** Applied 2026-10-19
- **Created:** 2026-10-19
- **Document revision:** 1
- **Sec version:** 0.1
- **Work item:** P16-05
- **Source authority:** `rules/mlir/semantic-ir/sec_semantic_ir_slice_v1.md`,
  `rules/mlir/dialect-versions/sec_mlir_dialect_v12.md`,
  `rules/mlir/lowering-versions/sec_mlir_lowering_v12.md`,
  `rules/corrections/applied/md051-md074-md088-length-numeric-string-correction-20261017.md`
  §§ 1–3 and 6, `rules/mlir/packages/sec-mlir-dialect_package15.md` § 28
  (revised 2026-10-03), `rules/memory/storage.md` §§ 14 and 24,
  `rules/memory/reference_model.md` § 26
- **Targets:** `rules/compiler/semantic_ir.md`,
  `rules/mlir/sec_mlir_dialect.md`, `rules/mlir/sec_mlir_lowering.md`

## Problem

The supplied P16 representation contracts had not been installed in the
consolidated rulebooks. Their historical text also proposed a signed
`SliceLengthInt` / `sec.slice.len_int`, used `.len` and retained the withdrawn
relocation-class model. Later accepted decisions already supersede these
assumptions. Installing the inputs verbatim would restore obsolete semantics.

## Applied integration

1. `semantic_ir.md` § 126.1–§ 126.26 installs every slice amendment section;
   § 16(6) links the existing array/slice overview to it. Element type and
   shared/mutable authority define slice identity. Runtime length, exact or
   dynamic range, origins, storage, borrow/lifetime and validity facts survive
   separately. Place operations, guard/failure ordering, trivial element
   ownership, returned origins, deterministic printing and physical-layout
   exclusions remain explicit.
2. `semantic_ir.md` § 106(5) distinguishes document revision 2.0 from
   executable Semantic IR schema 1. P16 requires no executable version bump.
   Sec MLIR schema and lowering specification version 12 are separate again.
3. `sec_mlir_dialect.md` v12.1–v12.33 and `sec_mlir_lowering.md`
   v12.1–v12.31 install the complete corresponding inputs with stable
   version-prefixed headings, preserving preceding versions.
4. Both `.Len` and `len(slice)` use the same unsigned length observation:
   `SliceLength -> uint`, emitted as `sec.slice.len -> !sec.uint`. There is
   no admitted `SliceLengthInt` / `sec.slice.len_int`, signed
   representability gate or implicit narrowing. An explicit checked signed
   conversion is independent. These are the already accepted MD-074/MD-088
   semantics, not a new length policy.
5. Slice facts reuse canonical address stability and a separate pin
   dependency, not `RelocationClass`. Parameter/call/returned-origin and
   optional metadata serialization remains subject to MD-084. This correction
   does not confirm the implementation's unresolved concrete spellings.
6. Schema 12 inherits schema-11 compatibility, including the early admissions
   already recorded in the v11 snapshot § 41. That section is also installed
   as consolidated v11.41; no new operation is admitted by this synchronization.
7. The three inputs retain their historical clauses with integration notices
   directing readers to the corrected consolidated contracts. Pre-existing
   section numbers and the original version history remain available.

## Completion boundary

Installation completes P16-05 only. It does not complete source-rule
synchronization (P16-03/P16-04), predecessor acceptance (P16-07), snapshot
capture (P16-01), remaining slice verification/production, physical lowering
or P16 acceptance. Specification installation does not promote the compiler's
schema-11 default. Governance records partial schema-12 support separately.

`language-rulebook-status.md`, `package16-todo.md` and the sole owning
`lowering.sec-mlir-package16` integration are synchronized with this boundary.
