# P27 Sec MLIR Lowering — LLVM Handoff Version 23 (planned)

- **Status:** Implementation specification; not implementation evidence
- **Created:** 2026-10-20
- **Last updated:** 2026-10-20
- **Revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/mlir/lowering-versions/sec_mlir_lowering_v23.md` (installed 2026-10-20 at the established location instead of the proposed `rules/mlir/lowering-versions/sec_mlir_lowering_package27.md`)
- **Replaces:** None; additive amendment. Preserve all existing paragraph references.
- **Package:** `SEC-MLIR-P27`

---

## § 1. Version status

This is **planned lowering specification v23**, following P26's planned v22. It is NOT a claim that the installed lowering rulebook, default emission, or LLVM conversion implements v23. Installed lowering specification currently reads v12, with default schema-v11 emission.

## § 2. Pipeline

```text
verified Semantic IR
 -> verified mixed Sec/standard MLIR
 -> legalize supported semantics to standard MLIR
 -> sec-check-lowering-support (existing capability authority)
 -> sec-finalize-for-llvm (no undischarged Sec semantics)
 -> registered standard-to-LLVM-dialect conversions
 -> LLVM dialect verifier
 -> matching-build MLIR to LLVM IR translation
 -> LLVM IR verifier
 -> matching-build llc (object)
 -> target system C driver (link only)
```

Pass placement may be adjusted to the existing installed pipeline as long as all gates run before their obligations would be erased.

## § 3. S0 scalar/CFG admission

Admit only concrete scalar integer/bool constants, verified integer operations, comparisons, branches and already-supported direct internal scalar calls, together with correctly lowered failure endpoints. No aggregate ABI inference.

## § 4. LLVM type conversion

Use standard registered MLIR-to-LLVM type conversion for already physical scalar/CFG types. Do not choose physical aggregate/reference/string layout from LLVM types. Target `int`/`uint` widths follow `CompilationPlan`, not the host.

## § 5. Function and branch conversion

Use the existing `func`/`cf` to LLVM conversion passes. Functions and call sites must match the temporary internal scalar convention established by the current single track, explicitly gated by MD-101 until P25 replaces it. Verify all block argument, result and return types.

## § 6. Checked failure and panic

Materialize the selected known failure endpoint before LLVM. Preserve distinguishable failure reasons for future policy/reporting. Current hosted trap strategy is interim MD-101 behavior, not an eternal Sec language mandate.

## § 7. LLVM attrs and UB avoidance

Derive attributes only from canonical proof/plan. No unproved `nsw`, `nuw`, `noalias`, `nonnull`, `dereferenceable`, alignment, `inbounds` GEP or value `undef`/poison used to replace Sec checked failures. Keep safety guards dominant.

## § 8. Toolchain and object

Use matching `mlir-translate`/LLVM IR toolset/`llc`. Validate tool identities and target triple/data layout. `llc -filetype=obj` produces the object; system C driver links that object and does not compile textual LLVM IR.

## § 9. Capability failures

Before LLVM emission, reject unsupported SemIR op families, missing P24 layout, P25 CallABIPlan, P26 reference discharge, P17 cleanup or incompatible target features. Emit `[unsupported-lowering]` with capability and stage, never fall back to a retired backend.

## § 10. Integrated test boundaries

Test emitted LLVM dialect, LLVM IR, target object and native host exit separately; test schema v11 default/v12 explicit acceptance; test negative unsupported feature and unsupported target; test O0/O2 semantic equivalence and stale-cache cold parity.

## § 11. No force-completion of planned packages

A scalar/CFG LLVM S0 pipeline is useful even while P17–P26 are incomplete. LLVM conversion for reference/descriptor/ABI/foreign ops is enabled only after those predecessors are executable. Do not mark P25/P26 tasks complete from one scalar test.

## § 12. Completion

Complete P27's S0 scope after focused/negative/lit/Go tests, verified LLVM IR, matching-build `llc`, host object+link test and correct no-fallback capability diagnostics. Record not-run and blocked honestly.
