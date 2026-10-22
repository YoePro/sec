# SEC-MLIR Package 26 — implementation TODO

Package: `SEC-MLIR-P26` — Validated Physical Reference and Descriptor Lowering  
Primary rulebook: `rules/mlir/packages/sec-mlir-dialect_package26.md`  
Package contract: `rules/mlir/packages/sec-mlir-dialect_package26.yaml`

Normative inputs:

- `rules/mlir/normative-sync/sec_reference_physical_sync_package26.md`
- `rules/mlir/semantic-ir/sec_semantic_ir_reference_physical_package26.md`
- `rules/mlir/dialect-versions/sec_mlir_dialect_v22.md`
- `rules/mlir/lowering-versions/sec_mlir_lowering_v22.md`
- `rules/mlir/packages/sec-mlir-dialect_package26_decision-index.md`
- `rules/memory/reference_model.md` (§§ 30–39 epochs and generations, §§ 70–79
  Semantic IR, lowering, check elimination, panic, diagnostics, tooling)

This is an implementation plan, not a completion report. It was installed
2026-10-20 from `rules/package26-todo-proposal.md` and validated against the
live checkout: item IDs P26-01..P26-27 are the proposal's and stay stable;
P26-28 onward were added during validation. Execute one item at a time, run
its focused positive and negative test immediately and record commands and
output before continuing.

## Inventory snapshot — 2026-10-20

The proposal's basis (`831f6e1`, 145 dirty entries, 2026-10-18) is stale: the
working tree now also contains the P16 schema-12 work of 2026-10-19 and the
P20 float subset. Record the live HEAD and dirty state again before P26-01 is
checked.

| Area | Current implementation | P26 consequence |
|---|---|---|
| Schema | Default emission is schema 11; since 2026-10-19 `sec-mlir-opt` also accepts schema 12 (P16 slices) and the emitter can select it explicitly. Schemas 13–21 (P17–P25) are not installed. | v22 stays planned; P26 work that needs schema-level operations waits for the chain 13–21. |
| Reference semantics | P15 Sema publishes Place, direct-reference, borrow, origin and validation facts; Semantic IR has place/reference operations including `ref.is-valid` and `fail.reference-generation`. | Consume these facts; never add a second borrow checker. |
| Stale-reference failure | Sema records the `InvalidReferenceGeneration` panic effect (`recordReferenceValidationEffect`); the dialect has `sec.ref.is_valid` and `sec.fail.reference_generation`; `sec-lower-failure-endpoints` already turns failure terminators into a trap. | P26-11 reuses this path; `sec-check-lowering-support` does not yet admit `sec.ref.is_valid` / `sec.fail.reference_generation`, so the native track rejects them today. |
| Reference policy | `layout.ResolvedReferencePolicy` defaults to a 64-bit logical epoch on 32- and 64-bit targets and rejects a shorter width without a retirement proof. | Credit for P26-09's width rule; non-wrap/exhaustion lowering remains. |
| Physical layout | P24 `ResolvedLayout`, `LayoutID` and `sec.repr.*` do not exist yet. | Every item that names a LayoutID or `sec.repr.*` op is blocked on P24, not merely on schema numbers. |
| Call ABI | P25 `CallABIPlan` does not exist; extern calls are rejected by the native track. | P26-22/23 stay blocked on P25. |
| Storage transitions | P18/P19 `StorageEstablishDomain`/`AdvanceEpoch`/`EndDomain`/`Reclaim` operations are not implemented. | P26-10, P26-15 and P26-16 are blocked on P18/P19. |

Primary implementation anchors (verified to exist 2026-10-20):

```text
internal/layout/reference_policy.go
internal/layout/scalar_plan.go
internal/platform/target/target.go
internal/sema/place.go
internal/sema/reference_plans.go
internal/ir/semantic/place_reference.go
internal/ir/semantic/model.go
internal/ir/semantic/verifier.go
internal/lowering/secmlir/emitter.go
mlir/include/sec/Dialect/Sec/SecOps.td
mlir/lib/Conversion/SecToLLVM/SecToLLVM.cpp
governance/memory.yaml
```

## Authority and dependency constraints

1. **Decisions:** MD-070 (`RawPtr` ↔ safe reference spelling), MD-019
   (explicit layout spelling), MD-073 and MD-109 (string/char materialization)
   and MD-108 (borrowed interface view representation, book § 27) are open
   gates. MD-061, MD-050, MD-071, MD-072 and MD-004 are applied and not
   reopened.
2. **New decisions follow the project practice:** a simple choice is asked of
   the owner directly; a substantial fork is recorded as a new `MD-###` entry
   in `missing-decisions.yaml` (book § 4's standalone `P26-D#.md` files are
   replaced by this practice; no duplicate IDs).
3. **Panic identity:** a stale ordinary direct reference is
   `panic.invalid-reference-generation` / `InvalidReferenceGeneration`
   (`reference_model.md` § 76(2)) — never a `Result`, never caught by `try`.
4. **Epoch width:** 64-bit logical epoch on 32-bit general-purpose targets
   (`reference_model.md` § 33); no truncation, tearing or wrap.
5. **One track only:** Sema → Semantic IR → Sec dialect MLIR → standard MLIR →
   LLVM dialect → LLVM IR → `llc` object (same LLVM build) → clang link. P26
   lowers to standard MLIR; LLVM-dialect convergence is P27.
6. **Governance:** progress is recorded in the existing IDs
   `semantic-ir.reference-model`, `lowering.reference-model`,
   `platform.reference-model`, `tooling.reference-model` and
   `lowering.references` in `governance/memory.yaml`; no new parallel ID.

## Completion boundary

As defined by the package book § 58 (acceptance checklist) and § 10 of the
lowering-v22 book. P26 does not define public stable/weak handle APIs,
MD-070 syntax, a runtime borrow checker, a mandatory generation manager, GC,
a general heap allocator, final LLVM/ABI encoding or interface-view layout.

## A. Baseline and dependencies

- [ ] P26-01 — Record live `git rev-parse HEAD`, `git status --short`, toolchain
  versions and current generated schema (default 11; 12 selectable).
- [ ] P26-02 — Inventory P15–P25 implementation truth; read corrected local
  package plans and `package25-todo.md`. Note: P24 and P25 have no executable
  core yet.
- [ ] P26-03 — Compare current `missing-decisions.yaml` and corrections; reuse
  existing governance IDs (constraint 6).
- [ ] P26-04 — Run and record existing focused reference/layout/ABI and full
  Go/MLIR baseline tests.

## B. Canonical reference physical plans

- [ ] P26-05 — Reuse P24 LayoutID/reference policy and define immutable
  read-only reference physical plan references (book § 8). **Blocked on P24**
  for LayoutID; the reference-policy half can start from
  `layout.ResolvedReferencePolicy`.
- [ ] P26-06 — Construct guarantee-specific proof/check/target discharge facts
  and a complete cache key (book §§ 7, 12, 37).
- [ ] P26-07 — Implement fully proven address-only direct-ref vertical slice
  with positive and negative tests (book § 13).
- [ ] P26-08 — Implement target-selected checked-epoch direct-ref vertical
  slice with checked failure and tests (book §§ 17–18).

## C. Validity and invalidation

- [ ] P26-09 — Preserve 64-bit logical epoch on 32-bit plans; add
  non-wrap/retirement controls (book §§ 11, 19).

  Partial credit: `layout.ResolvedReferencePolicy` already defaults to 64 bits
  and requires a proof for a shorter width; non-wrap and exhaustion lowering
  remain.

- [ ] P26-10 — Reuse P18/P19 storage transitions to invalidate checked
  dependencies (book § 20). **Blocked on P18/P19.**
- [ ] P26-11 — Implement stale ordinary reference panic
  `InvalidReferenceGeneration` and bounds distinction (book §§ 18, 47).

  Existing path to reuse, not to reimplement: Sema effect
  (`recordReferenceValidationEffect`), Semantic IR `ref.is-valid` /
  `fail.reference-generation`, dialect `sec.ref.is_valid` /
  `sec.fail.reference_generation`, and the trap endpoint in
  `sec-lower-failure-endpoints`. Missing: admission to
  `sec-check-lowering-support` and lowering of `sec.ref.is_valid` (P26-28).

- [ ] P26-12 — Ensure check success dominates access and invalidating
  operations prevent unsound hoisting (book §§ 32, 46).
- [ ] P26-13 — Validate concurrency protocol/non-tearing only when supported
  by selected plan (book § 31).

## D. Physical views and descriptors

- [ ] P26-14 — Lower selected P16 shared/mutable slice descriptor with
  owner/epoch/range facts (book § 22). Depends on P16 completion and P24.
- [ ] P26-15 — Preserve P18 owned array backing and P17 cleanup without
  descriptor ownership duplication (book §§ 23–24). **Blocked on P17/P18.**
- [ ] P26-16 — Lower P19 Arena-backed refs/slices and reset/release boundary
  only on ready substrate (book § 26). **Blocked on P19.**
- [ ] P26-17 — Lower selected P23 string materialization/view descriptor
  without inventing materialization API (book § 25). Blocked on P23; MD-109
  stays a separate gate.
- [ ] P26-18 — Validate zero-sized elements, zero-length slices and wrong
  address-space rejection (book §§ 15–16, 30).

## E. Schema v22 and ABI integration

- [ ] P26-19 — Install typed schema-v22 plan/proof attrs/ops only after
  executable predecessor checks (schemas 13–21).
- [ ] P26-20 — Register physical reference, descriptor and discharge verifiers
  with malformed IR tests (book §§ 42–45).
- [ ] P26-21 — Register representation/validity/selected-descriptor lowering
  passes and standard-MLIR tests (book § 41).
- [ ] P26-22 — Consume P25 exact CallABIPlan at native call boundaries only if
  ABI substrate is executable (book § 35). **Blocked on P25.**
- [ ] P26-23 — Test verified foreign call-bounded ref adapter; reject public
  MD-070 conversions until decided (book §§ 29, 36). **Blocked on P25.**
  Note 2026-10-22: MD-070 was decided and applied on 2026-10-21. The public spellings are the unsafe compiler-known `RawPtr[T].AsRef()` / `AsRefMut()` (`raw_pointers.md` § 18A), and their Semantic IR operation is still a capability boundary. The "reject public conversions" part of this item therefore becomes: lower them only once that operation exists with its proof facts (MD-070 § 6).

## F. Regression and handoff

- [ ] P26-24 — Test 32/64-bit, hosted/hardened/bare-metal/unsupported
  profiles, and no-mandatory-runtime cases (book § 49).
- [ ] P26-25 — Mutate cached layout/plan/epoch/ABI metadata and compare
  independently verified cold results (book §§ 37, 46).
- [ ] P26-26 — Run full Go/vet/CMake/sec-mlir-opt/llvm-lit checks where
  installed; record not-run honestly.
- [ ] P26-27 — Update existing governance fragments (constraint 6) and create
  the implementation report of book § 59 with exact evidence.

## G. Added during validation (2026-10-20)

- [ ] P26-28 — Admit `sec.ref.is_valid` and `sec.fail.reference_generation`
  to `sec-check-lowering-support` once a selected reference representation can
  lower the validity test; until then keep the explicit unsupported-lowering
  diagnostic. Test the trap endpoint for a stale Arena-backed reference.
- [ ] P26-29 — Register the reference diagnostics of book § 48 /
  `reference_model.md` § 77 (missing guarantee, origin, invalidating event,
  selected plan) through the diagnostics registry; separate malformed source
  from unsupported target representation and imported-IR invariant failures.
- [ ] P26-30 — Keep LSP/`sec analyse` presentation of validation checks and
  elided checks consistent with the same facts (`reference_model.md` § 79).
- [ ] P26-31 — Keep MD-108 (interface view representation) as an explicit
  capability gap for interface/closure descriptors (book § 27); do not choose a
  data-plus-table layout in P26.

## Recommended execution order

1. A: capture the live baseline and the real P15–P25 capability matrix.
2. B/C: plan records, static proof taxonomy, the 64-bit epoch rule and the
   existing stale-reference path (P26-05 policy half, P26-06, P26-09, P26-11,
   P26-28) — these do not need P24.
3. D/E: only as P16–P19, P23, P24 and P25 become executable; each blocked item
   keeps an explicit diagnostic.
4. F/G: verifier negatives, profiles, caches, diagnostics, tooling and
   governance with recorded commands.
