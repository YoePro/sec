# P27 Sec MLIR Dialect Amendment — LLVM Handoff

- **Status:** Implementation specification; not implementation evidence
- **Created:** 2026-10-20
- **Last updated:** 2026-10-20
- **Revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/mlir/dialect-versions/sec_mlir_dialect_package27.md` (installed 2026-10-20)
- **Replaces:** None; additive amendment. Preserve all existing paragraph references.
- **Package:** `SEC-MLIR-P27`

---

## § 1. Version control

**No unconditional Sec dialect schema bump is required by P27.** The currently checked-in specification is v12 and current default generated schema is v11; explicit v12 selection is supported as of 2026-10-19. P17–P26 planned schemas v13–v22 are not installed merely because P27 exists. Schema **v23 is reserved/conditional** only if a new serialized Sec op/type/attr is actually needed after predecessor reconciliation.

## § 2. No redundant handoff Sec ops

Do not add `sec.llvm_bridge`, `sec.llvm_call`, `sec.llvm_return`, or a duplicated LLVM dialect merely to mark a pipeline stage. Ordinary standard MLIR operations are used after their Sec obligations are fully discharged.

## § 3. Finalization legality

`sec-finalize-for-llvm` rejects remaining unresolved `sec.*` ops/types/semantic attrs. Existing `sec.unreachable`, checked failure endpoints and source-specific operations must have been lowered or rejected with capability diagnostics. Preserve debug metadata only through a declared compatible representation.

## § 4. Typed capability verification

Capability IDs and plan keys are compiler-owned readiness/verification metadata, not ordinary Sec values. Prefer established pass interfaces rather than adding a new serialized dialect family. If implementation requires a serialized attribute, it must be narrowly specified, parsed/printed/verified, assigned an exact new schema version and regression tested.

## § 5. Verification passes

Use existing `sec-check-lowering-support`, `sec-finalize-for-llvm` and standard MLIR verifiers. Add/extend `--sec-verify-llvm-handoff` only when no equivalent existing verification exists. Reject illegal LLVM attrs, missing layout/ABI plans, undischarged references and unsupported profiles.

## § 6. Acceptance

The dialect parser/verifier retains v11 emission regressions and explicit v12 acceptance. Higher schema cases are conditional on their actual installed predecessors. This amendment alone is not evidence of a v23 module.
