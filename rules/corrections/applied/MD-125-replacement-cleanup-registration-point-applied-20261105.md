# MD-125 — Replacement preserves an owned Place's cleanup registration position

- **Decision ID:** MD-125 — `replacement-cleanup-registration-point`
- **Document kind:** Approved normative correction and implementation handoff
- **Status:** **APPROVED — normatively integrated 2026-11-05** (destruction.md §§ 6.1(4)–(5), 13.2(5)–(8), 13.3(5); semantic_ir.md § 40(7)–(10); defer.md § 9; P17 sync texts reconciled; P17-03 done). **Implementation pending** (governance `frontend.destruction`, `lowering.sec-mlir-package17`).
- **Created / approved:** 2026-11-05
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Reviewed repository:** `YoePro/sec`, `main` at `0cdacb04fb3757a26569d51c21a5ae445f0facaf` (verified 2026-11-05)
- **Decision:** MD-125 option **B**. Successful **replacement** of an available owned Place preserves that Place's existing cleanup position. Successful **reinitialization** of an unavailable Place establishes a new cleanup position.
- **Normative owners:** `rules/memory/destruction.md`, `rules/control-flow/defer.md`, `rules/compiler/semantic_ir.md`
- **Contradicting predecessor specifications:** `rules/mlir/normative-sync/sec_ownership_destruction_sync_v1.md` §§ 1 and 3; `rules/mlir/semantic-ir/sec_semantic_ir_ownership_destruction_v1.md` § 11; `rules/corrections/p17-ownership-destruction-rules-synchronization.md` proposals A and D
- **Implementation owner:** `package17-todo.md` (P17-03 and the existing P17 cleanup implementation work); `governance/lifetime.yaml` (`frontend.destruction`); `governance/lowering.yaml` (`lowering.sec-mlir-package17`)
- **Decision-register owner:** `missing-decisions.yaml`, existing MD-125 entry

> **Integration instruction.** MD-125 settles a source-language choice; it does not certify the current implementation. Apply the numbered normative insertions below without renumbering existing paragraphs. Correct the contradictory *P17 synchronization texts* explicitly. Reuse existing P17 checklist steps and governance integration IDs; do not create duplicate owners or claim implementation completion from the decision alone. Implement one change, test it, and then continue to the next.

---

## § 1. Purpose, scope and settled choice

**§ 1(1).** This correction resolves the previously open MD-125 question about the relative execution order of a registered `defer` and destruction of an owned value that is subsequently replaced in the same binding/Place.

**§ 1(2).** The canonical model remains the common LIFO cleanup ordering established by `destruction.md` § 6.2 and `defer.md` § 7. **Replacement does not append a new cleanup obligation to that order.** It changes the currently owned value associated with an already-active Place cleanup obligation.

**§ 1(3).** A replacement destroys the preceding owned value exactly once, in accordance with `destruction.md` § 13.2. Only after successful preparation and permitted commitment does the new value become owned by the destination. This immediate destruction is distinct from the destination's eventual scope/function-exit cleanup.

**§ 1(4).** A reinitialization after unavailability is distinct: the previous ownership obligation has ended, so successfully reinitializing the mutable Place establishes a *new* cleanup registration at that reinitialization point (`destruction.md` §§ 6.1(2), 13.1(3)).

**§ 1(5).** The decision does **not** alter `defer` into a snapshot mechanism. A registered deferred block reads the current contents of its captured binding/Place when the block actually executes (`defer.md` § 9; `semantic_ir.md` § 44). No implicit copy of the prior owned value is made at `defer` registration.

**§ 1(6).** The rule applies to owned Places that already have a live automatic-destruction obligation, including values with nontrivial destruction. It does not change ordinary value assignment, the order of independent `defer` registrations, custom `free`, temporary lifetimes, borrowing restrictions, partial-move legality, or panic behavior. Conditional availability is handled per path (§ 6).

## § 2. Canonical example and observable order

**§ 2(1).** Consider the following code (the helpers are illustrative; `Handle` owns a resource with deterministic destruction):

```sec
fn Example() void {
    let mut handle := Make(1)

    defer {
        Log(handle.Id)
    }

    handle = Make(2)
}
```

**§ 2(2).** The observable semantic sequence is:

1. `Make(1)` completes; `handle` owns the first value, and its eventual cleanup obligation is registered.
2. The `defer` block is registered, *without evaluating* `Log(handle.Id)`.
3. The replacement's right-hand side, `Make(2)`, is prepared under the existing transactional-assignment rules.
4. The first value is destroyed exactly once when the replacement commits.
5. The second value becomes the current value of `handle`; the *existing* cleanup registration retains its original ordering position.
6. At normal function exit, the registered `defer` executes, evaluates `handle.Id` and logs `2`.
7. The second value is destroyed exactly once by `handle`'s retained cleanup obligation.

**§ 2(3).** This function is **valid**, provided the usual type, borrowing, ownership, effect and lifetime requirements hold. Rejecting it merely because the old owner was replaced would contradict the decided `defer` semantics.

**§ 2(4).** If several successful replacements occur while the Place remains available, each old value is destroyed at its replacement point; the final current value is destroyed when the original active obligation becomes eligible. Replacement neither duplicates the obligation nor moves it behind intervening `defer` registrations.

## § 3. Normative insertion — `rules/memory/destruction.md`

Insert the following new paragraphs **after § 6.1(3)**, preserving every existing number:

> **§ 6.1(4)** A successful replacement of an **Available** owned Place changes which owned value its already-active cleanup obligation will destroy, but preserves the obligation's original source-semantic registration position in the common cleanup order. Replacement is not a second registration. The old value's destruction at replacement commit is a distinct immediate action.
>
> **§ 6.1(5)** Successful reinitialization of an **Unavailable** mutable Place establishes a new cleanup obligation at the reinitialization point. It never restores a canceled registration's former ordering position. On a `ConditionallyAvailable` path, the distinction is resolved according to whether a live prior obligation exists on that path.

Insert the following new paragraphs **after § 13.2(4)**:

> **§ 13.2(5)** On successful replacement of an Available owned Place, the new owned value **inherits the destination's existing active cleanup obligation and its position** in the unified cleanup order. The previous value is destroyed exactly once before the replacement becomes committed. No additional final cleanup obligation is registered for the same Place by the replacement.
>
> **§ 13.2(6)** A deferred block that refers to the destination Place, and was validly registered before replacement, observes the destination's current value at deferred execution. Replacement of that value does not itself invalidate the deferred read or cause the new value to be destroyed before a defer that was registered after the destination's original cleanup obligation.
>
> **§ 13.2(7)** Failure during preparation must leave the old available value and its active cleanup obligation, including its ordering position, unchanged. Successful commitment transfers the prepared value into the existing obligation only after all prerequisite checks and required old-value cleanup have completed under the language's existing failure and destruction rules.
>
> **§ 13.2(8)** Repeated replacement preserves the same active cleanup position until the obligation ends through a terminal ownership action or the Place's eligible lifetime end. A later legal reinitialization after unavailability is governed by § 13.1(3) and § 6.1(5), not by this replacement rule.

Insert the following new paragraph **after § 13.3(4)**:

> **§ 13.3(5)** For a `ConditionallyAvailable` destination, a successful assignment preserves the pre-existing active cleanup position **on paths where an owned value remains available**, and establishes a new cleanup position **on paths where the Place is unavailable**. The compiler must preserve this distinction in its ownership and cleanup facts; no path may destroy a value twice or read an unavailable value.

**Rationale:** These insertions preserve §§ 6.2, 6.3, 13.1 and 13.2(1)–(4) unchanged while making replacement/reinitialization ordering explicit. The *value incarnation* changes on replacement; the *live Place obligation's registration position* does not.

## § 4. Normative insertion — `rules/compiler/semantic_ir.md`

Insert the following new paragraphs **after § 40(6)**:

> **§ 40(7)** Successful replacement of an Available owned Place destroys the old owned incarnation according to the resolved destruction plan and associates the prepared new value with the **same active cleanup registration identity/ordering position** as the destination. It does **not** create a later-ordered final cleanup registration.
>
> **§ 40(8)** Successful reinitialization of an Unavailable mutable Place establishes a new cleanup registration identity and position. An ownership action that terminates or transfers the existing responsibility cancels that obligation under the ordinary move/discard/return rules.
>
> **§ 40(9)** A conditional replacement/reinitialization retains path-sensitive information about whether a prior active obligation is inherited or a new obligation is registered. Semantic IR cleanup plans, verifier checks and lowering must preserve the canonical execution order without double-destruction, stale cleanup entries or use-after-destruction at a later `defer`.
>
> **§ 40(10)** Source-level `defer` capture retains the binding/Place identity and performs its ordinary read when the deferred body runs (§ 44; `defer.md` § 9). The current value after a valid replacement must still be available to a deferred read that is ordered before that Place's inherited final cleanup. The optimizer may not move that cleanup ahead of the deferred read.

**IR implementation note (not new source syntax):** Existing `ResolvedOwnedCleanupAssignment`, cleanup action IDs, registrations and producer operations must distinguish **new value incarnation** from **new cleanup registration**. A per-assignment source-site action ID may identify the immediate replacement/destruction operation; it may *not* silently stand in for a newly appended owner cleanup obligation on the available-replacement path. Preserve detached Sema facts and verifiable order rather than patching the operation printer to make the output appear correct.

## § 5. Normative clarification — `rules/control-flow/defer.md`

Keep § 9's existing example and statement that a deferred block reads the value at execution. Add the following paragraph **after the example in § 9**, without changing the section number:

> The deferred block references the captured binding/Place identity, not a snapshot of the value stored there when `defer` was registered. If an available owned value in that Place is validly replaced before deferred execution, the block reads the **replacement value**, and the Place's inherited final cleanup still runs in its established position after any earlier-ordered deferred read. Existing borrowing and pending-use restrictions continue to apply; `defer` does not authorize an invalid move, an unavailable read, or an escaped borrow.

This is a clarification of existing § 9, **not** a separate capture mechanism or change in `defer` syntax.

## § 6. Reinitialization, paths, errors and boundaries

**§ 6(1).** A successful assignment to an Unavailable mutable Place is reinitialization. Its new registration is source-ordered at that reinitialization point. Do not treat this as ordinary replacement just because the same binding name appeared earlier.

**§ 6(2).** A whole-Place replacement of a still-owned value preserves the active slot even when the new value has a different runtime resource identity. The old value's destructor runs at replacement commitment; the final destructor runs later for the replacement value. This distinction is required for correctness on both 32- and 64-bit targets and is independent of physical stack layout.

**§ 6(3).** On paths where the destination is only conditionally available, preserve both legal cases with explicit path-sensitive facts. Neither unconditional reuse nor unconditional new registration is permitted unless the compiler has proved the corresponding path condition. Existing policies that reject dynamic bookkeeping may still require explicit convergence; MD-125 does not relax those restrictions.

**§ 6(4).** If the right-hand side or required preparation fails before replacement commitment, the destination retains the old value and its existing cleanup registration. Cleanup of successfully prepared temporaries follows their own original lifetime rules. No replacement registration is committed on that failure edge.

**§ 6(5).** Borrowing and `defer` still constrain which replacements are legal. A live reference into the old value cannot be silently retargeted to the new value merely because the Place cleanup obligation is preserved. An invalid borrowed subobject, an unavailable deferred use or a forbidden transfer remains a diagnostic under the existing ownership and borrow rules.

**§ 6(6).** This correction does not decide the separate P17 boundary concerning a new owned binding created in each loop iteration and read by a `defer`, or the loop-storage identity problem described in the P17 synchronization audit. Those cases retain their existing status and must not be treated as fixed merely because MD-125 is closed.

## § 7. Required reconciliation of contradictory P17 texts

These texts currently state **option A** and must be corrected when integrating MD-125; simply inserting §§ 3–5 above leaves contradictory normative authorities.

| Document | Existing conflicting statement | Required action |
|---|---|---|
| `rules/mlir/normative-sync/sec_ownership_destruction_sync_v1.md` § 1 | Lists "successful replacement creating a new owned value" as a new cleanup *registration event*. | Distinguish immediate replacement/destruction from registration: **no new final obligation for available replacement**; new registration for previously unavailable Place only. |
| Same file § 3 | States "The new incarnation's cleanup registration occurs at the replacement point." | Replace with the inherited-active-obligation rule; preserve old-value destruction and new-incarnation distinction. |
| `rules/mlir/semantic-ir/sec_semantic_ir_ownership_destruction_v1.md` § 11 | States "The new owned incarnation is registered at the replacement point." | Clarify that the new incarnation becomes the value covered by the **pre-existing active** cleanup obligation. |
| `rules/corrections/p17-ownership-destruction-rules-synchronization.md`, proposals A and D | Proposes creating new final cleanup registration at replacement and treating the illustrative defer case as invalid. | Mark proposals **A and D superseded by MD-125** and point to the approved insertions. Preserve already-applied proposals B, C and E and their paragraph numbers. |
| Same P17 audit, open MD-125 example | Describes the owner decision as undecided and identifies the case as a lowering boundary. | Record **approved option B**, expected execution order and implementation remaining. Do not erase the historical finding or falsely assert the boundary is implemented. |
| `package17-todo.md`, P17-03 | Says A/D remain open until MD-125 is decided. | Record that the owner decision is resolved. Mark P17-03 complete **only after** its rulebook and sync reconciliation checks have passed. |

**§ 7(1).** The correction supersedes only the conflicting replacement-registration assertions; it does not rescind unified LIFO order, deterministic old-value destruction, successful-only registration, move cancellation, reinitialization rules, defer execution-time reads, or existing P17 boundary evidence.

**§ 7(2).** Do not retroactively rewrite unrelated MDs. The P17 synchronization documents are the known contradictory normative inputs and must be explicitly reconciled with this correction.

## § 8. Exact implementation inventory and change ownership

The following are **review/change targets**, not claims that every file definitely requires a code edit. Verify each implementation step against current HEAD before applying it.

| Layer | File / existing owner | Required check or change |
|---|---|---|
| Sema plan | `internal/sema/cleanup_assignments.go` | `prepareOwnedCleanupAssignment` currently gives every accepted assignment an action identity derived from the assignment site. Distinguish replacement's **inherited active registration** from reinitialization's **new registration**, including Conditional/Partial cases. Preserve successful-commit publication. |
| Sema registration | `internal/sema/cleanup_registrations.go` | Keep one active final obligation for an available replacement; record reinit separately. Ensure metadata preserves correct scope, Place and original LIFO position. |
| Sema identities | `internal/sema/cleanup_identities.go` | Distinguish source action identity, current owned incarnation and the destination cleanup obligation's stable ordering identity; avoid collisions and duplicate final cleanup. |
| Sema cancellation | `internal/sema/cleanup_cancellations.go` | Confirm genuine terminal transfer/discard cancels the active obligation and available replacement does not. |
| Sema defer | `internal/sema/analyzer_defer.go`, `internal/sema/defer_plans.go` | The valid defer-after-initialization / before-replacement example must not be rejected solely due to replacement. Retain proper invalid-move and borrow diagnostics. |
| Sema tests | `internal/sema/cleanup_registrations_test.go`, `internal/sema/cleanup_transitions_test.go`, `internal/sema/destruction_test.go` | Test preserved slot, reinit new slot, repeated replacement, error path, conditional availability and pending defer uses. |
| Semantic IR producer | `internal/ir/semantic/cleanup_producers.go` | Review `cleanup.track-owned`, cancellation and replacement event emission. The current source-site registration/ordinal scheme must not append a later final owner cleanup for an available replacement. |
| Semantic IR state/verifier | `internal/ir/semantic/cleanup_ops.go`, `internal/ir/semantic/verifier.go` and related P17 producer tests | Preserve inherited action ordering, exact-once cleanup, path distinction and valid deferred reads. |
| P17 regression | `testdata/semantic_ir/package17_variants/defer_reads_replaced.sec` | Existing concrete reproducer: it must become a **valid accepted case** when the required lowering is implemented, not a permanent invalid-language test. |
| P17 plan | `package17-todo.md` | Reconcile P17-03 and the still-open cleanup producer, registration and lowering tasks with the approved semantics. |
| P17 dialect/lowering | `rules/mlir/packages/sec-mlir-dialect_package17.md`, its package YAML, applicable Sec MLIR source and verifier tests | Reconcile the operation/cleanup plan contracts with inherited registration. Add verified source → Semantic IR → Sec MLIR acceptance before declaring the compiler-side boundary closed. |
| Tooling/diagnostics | Existing compiler diagnostics and LSP tests | A valid replacement/defer program must not receive an invented ownership error. Invalid moved/unavailable reads still require a useful diagnostic naming the pending deferred use. |

**§ 8(1).** Do not create a parallel cleanup stack, new ownership engine, or extra runtime registration solely to implement this decision. The implementation is an adjustment to existing P17 Sema facts and one canonical cleanup plan.

**§ 8(2).** The correction specifies the source-observable semantics. Compiler implementation may represent inherited obligations by stable registration IDs, forwarded payloads or another verified scheme, provided order, lifetime, exact-once destruction and failure behavior are identical.

## § 9. Required conformance and regression tests

**§ 9(1).** Add or update focused tests for the following independent cases:

1. **Valid replacement followed by defer read:** first owned value destroyed at replacement; defer sees second value; second destroyed afterward.
2. **Repeated replacement:** each preceding value destroyed once; only the last value is eligible for the retained final cleanup.
3. **Reinitialization after unavailability:** no destruction of absent old value; new cleanup obligation registered at reinitialization (while respecting pending deferred uses).
4. **Conditional ownership:** reuse prior active obligation on Available paths and create a new registration on Unavailable paths; verify joins.
5. **Preparation failure:** the first value and original final obligation remain intact; no premature destruction, no newly committed final obligation.
6. **Interleaved cleanup:** independent owned values and multiple `defer` registrations retain canonical unified LIFO ordering around replacement.
7. **Borrowing:** illegal overlapping references or pending borrowed uses are still rejected; no forged reference to an old object after replacement.
8. **Scope and exit edges:** ordinary fallthrough, explicit return, error propagation and applicable branch exits destroy every still-owned value exactly once in the correct order.
9. **Existing P17 boundary fixture:** `defer_reads_replaced.sec` transitions from the documented lowering boundary to a positive execution/IR test only when verified implementation exists.

**§ 9(2).** Required testing layers: Sema valid/invalid tests, semantic ownership/cleanup fact tests, Semantic IR producer and verifier tests, Sec MLIR verification/lowering tests, a native execution test when supported by the implemented track, and LSP diagnostic regression where appropriate. Record actual command results; the presence of this correction is not evidence of passing tests.

**§ 9(3).** Test both standard and exceptional control-flow edges where supported. Do not use an implementation's current "unsupported" boundary as evidence of language-level invalidity.

## § 10. Governance reconciliation — amend, do not duplicate

**§ 10(1).** In `missing-decisions.yaml`, keep **MD-125** as the historical decision ID, update the description to record **approved option B (2026-11-05)**, and change its decision status according to that ledger's established decided/pending-integration convention. Do not delete the record until canonical-rulebook integration and governance synchronization satisfy the ledger's removal policy. Do not register an MD-125 replacement under a new MD number.

**§ 10(2).** Update the existing `frontend.destruction` integration in `governance/lifetime.yaml`: add the rulebook authority and an **implementation-pending** item for inherited cleanup ordering, together with the related tests. Amend stale `implemented` descriptions that presently imply every assignment registers a later-ordered cleanup; preserve unrelated implemented evidence.

**§ 10(3).** Update the existing `lowering.sec-mlir-package17` integration in `governance/lowering.yaml`: reconcile the plan/producer/verifier work, link the P17 test fixture and identify the existing unsatisfied cleanup-lowering gate. Do not add `lowering.md125` or another independent integration entry with the same responsibility.

**§ 10(4).** Reconcile `package17-todo.md` **P17-03** (source-rule synchronization), plus the existing relevant cleanup implementation steps. Preserve their IDs and completion history. If normative integration finishes before lowering, record the distinction precisely: **MD-125 decided and source rules synchronized; implementation still partial**.

**§ 10(5).** Update `language-rulebook-status.md` only to reflect verified source-rule integration. No implementation claim is justified until its corresponding Sema, IR, dialect/lowering and tests are actually verified.

## § 11. Acceptance conditions and status transition

**§ 11(1).** The **normative correction** is integrated only when:

- `destruction.md` has §§ 6.1(4)–(5), 13.2(5)–(8) and 13.3(5), retaining all prior numbers;
- `semantic_ir.md` has § 40(7)–(10), retaining §§ 40(1)–(6);
- `defer.md` § 9 explicitly clarifies a replacement's execution-time value without creating a snapshot rule;
- contradictory P17 sync and Semantic IR amendment claims are corrected or marked explicitly superseded by this MD;
- the existing MD-125 ledger, P17-03 checklist, governance owners and rulebook status are reconciled with no duplicate IDs.

**§ 11(2).** The **implementation** is complete only with verified Sema facts, IR operations, dialect/lowering behavior, source-accepted valid defer/replacement fixture, correct owner cleanup order, negative borrowing/availability diagnostics and passing focused regression coverage. Any still-unsupported compilation track must remain openly recorded.

**§ 11(3).** A revision of this correction must preserve all existing paragraph IDs (§ 1(1) through § 11(3)). Add subordinate or subsequent numbered paragraphs without renumbering references already used by code, tests or governance.

---

## § 12. Decision summary

**MD-125 is APPROVED (2026-11-05):**

- **Replacement (Available):** destroy the old owned value at successful replacement commitment; reuse the Place's existing final cleanup registration and **retain its LIFO position**.
- **Reinitialization (Unavailable):** no old destruction; create a **new** cleanup registration at successful reinitialization.
- **`defer`:** evaluate its body at execution, reading the current value in the captured Place; replacement does **not** take a registration-time snapshot or invalidate an otherwise valid deferred read.
- **Conditional availability:** preserve the appropriate per-path case rather than inventing one universal cleanup action.
- **Implementation:** P17 remains responsible for bringing existing Sema/IR/MLIR cleanup code and tests into compliance; no independent implementation owner is added.
