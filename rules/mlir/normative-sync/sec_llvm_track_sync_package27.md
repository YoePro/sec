# P27 Normative Synchronization — LLVM Track and Schema v12

- **Status:** Implementation specification; not implementation evidence
- **Created:** 2026-10-20
- **Last updated:** 2026-10-20
- **Revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/mlir/normative-sync/sec_llvm_track_sync_package27.md` (installed 2026-10-20 at the established location instead of the proposed `rules/mlir/sync/sec_llvm_track_sync_package27.md`)
- **Replaces:** None; additive amendment. Preserve all existing paragraph references.
- **Package:** `SEC-MLIR-P27`

---

## § 1. Authoritative implementation status

As of 2026-10-20, current checked-in `rules/mlir/sec_mlir_dialect.md` and `rules/mlir/sec_mlir_lowering.md` specify **v12**. Schema 12 was installed 2026-10-19 and is accepted when explicitly selected; compiler emission defaults to **v11**. P16 full slice emission/acceptance is incomplete. Amend P26 historical text that said the *current specification* was v11; do not change source semantics or overwrite the installed v12 text.

## § 2. Single compilation track

MD-101 and `rules/compiler/compiler_pipeline.md` §45(6) are authoritative: Semantic IR → Sec/standard MLIR → LLVM dialect → LLVM IR → matching-build `llc` object → system C driver link. Legacy direct-LLVM and AST-to-LLVM-dialect paths are retired. Do not add fallback.

## § 3. Existing capability gates

Reuse `sec-check-lowering-support` and `sec-finalize-for-llvm`; extend only to make stage readiness and remaining Sec semantics fail closed. A parser/IR schema admission does not prove LLVM lowering.

## § 4. Existing toolchain selection

Respect `--mlir-bin`, `SEC_MLIR_BIN`, configured build directory and PATH according to the installed pipeline contract. Verify exact executable versions. LLVM IR does not cross into mismatched `llc` or C compiler frontend.

## § 5. Decision dependencies

MD-101 is already tracking provisional trap/entry/internal scalar ABI details. Do not convert these into a new decision ID. MD-019, MD-021, MD-045 and MD-070 remain separate. Public `missing-decisions.yaml` can lag local modifications.

## § 6. ABI and layout boundary

Only existing verified scalar internal conventions may cross P27 S0. Aggregates and foreign calls require canonical P24/P25 plans; references/descriptors require P26. Do not infer ABI from machine types.

## § 7. Panic boundary

Do not permanently lock `trap + unreachable` as the universal panic policy from MD-101's provisional implementation proposal. Preserve cause and CompilationPlan policy until a concrete endpoint is proven.

## § 8. No source changes

No new Sec source syntax or types are introduced. All content is compiler architecture/lowering-level. Normal language safety and checked error semantics remain unchanged.
