# MD-128 — Transactional ownership at the spawn creation boundary

- **Language version:** Sec 0.1 (retained in Sec 0.1.1; no new source syntax)
- **Document revision:** 1.0
- **Date:** 2026-11-24
- **Status:** Owner-approved design; normative integration pending; implementation not verified
- **Decision:** MD-128 — `spawn-creation-failure-ownership`
- **Governance owner:** `concurrency.spawn-v1`
- **Repository baseline:** `YoePro/sec`, main `3ed6048c0b9e25b28030a49c1f7cbe62421cd5e1`
- **Related decisions:** MD-129 (cancellation at task creation), MD-133 (spawn-block return types), MD-138 (callable execution requirements)
- **Primary rulebooks:** `rules/concurrency/spawn.md`, `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/concurrency/processes.md`, `rules/memory/ownership.md`
- **Related rulebooks:** `rules/memory/copy_move.md`, `rules/memory/borrowing.md`, `rules/memory/destruction.md`, `rules/memory/transferability.md`, `rules/declarations/lambda-functions.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/errors/errorhandling.md`, `rules/compiler/semantic_ir.md`

## § 1. Approved decision and scope

§ 1(1) The three concurrency creation forms shall have the same transactional ownership semantics: `spawn task`, `spawn thread`, and `spawn process`. Unqualified `spawn` retains its equivalence to `spawn task`.

§ 1(2) The existing process-startup contract in `processes.md` § 8.6 is the normative precedent. Successful creation commits the ownership transfers belonging to that spawn operation. Failed creation preserves ownership of the reusable caller-owned sources whose transfers were prepared but not committed.

§ 1(3) The purpose of this symmetry is to reduce cognitive load. A programmer shall not need to learn different failure-path move rules for the three execution kinds. Kind-specific execution, transferability, process materialization, scheduling, configuration, and lifecycle contracts remain intact.

§ 1(4) This correction neither changes public creation-error types nor introduces new grammar. It resolves the open task/thread commit rule as a Sec 0.1 semantic correction, applicable also to Sec 0.1.1.

## § 2. The transaction boundary

§ 2(1) A spawn operation has one logical creation transaction, covering its own pending transfers of reusable source Places, together with the preparation of owned values, borrowed inputs, receiver/environment state and required creation resources. This is a semantic transaction, not a requirement for a particular runtime transaction mechanism.

§ 2(2) The selected callable, arguments, receiver, captures and configuration are resolved and evaluated in the creating execution context in the ordinary prescribed evaluation order. Apply the existing explicit-move, pending reservation, copy and borrow rules. A named source requiring consumption uses the applicable `<-` syntax. Fresh temporaries do not acquire a synthetic explicit-move requirement.

§ 2(3) A pending transfer of a reusable source does not yet change its ownership to the child. Its reservation prevents overlapping conflicting accesses according to `ownership.md` §§ 11–14. Reservation is not permission to destroy, duplicate or independently use the source.

§ 2(4) All transfers belonging to the creation transaction commit exactly once at successful creation. The source Places that were consumed become unavailable; the new execution owns the corresponding inputs. Independently permitted copies remain copies. Borrows remain borrows, with the appropriate boundary-specific lifetime and exclusivity obligations.

§ 2(5) Successful creation establishes the source-visible owning handle and the corresponding valid execution identity. A child may begin executing before its creator receives or examines the `Ok` result, but never before the logical creation commit and the required input publication. No child user code may access a pending, incompletely prepared or uncommitted transferred input.

§ 2(6) Creation commit is distinct from callable entry, callable completion, and result observation. It is not delayed until `await`, `join`, or a deferred thread's `Start()`.

| Execution kind | Raw result type | Ownership commit boundary |
|---|---|---|
| `spawn` / `spawn task` | `Result[Task[T], TaskSpawnError]` | Successful task creation |
| `spawn thread` | `Result[Thread[T], ThreadSpawnError]` | Successful thread creation, including deferred creation |
| `spawn process` | `Result[Process[T], ProcessSpawnError]` | Existing successful process-startup commit |

## § 3. Failed creation

§ 3(1) Returning the creation result `Err(...)` means creation did not commit. All pending transfers of reusable caller-owned source Places belonging to this transaction are cancelled. Those sources remain owned and available to the caller, subject to the ordinary effects and ownership state established independently before or during argument evaluation.

§ 3(2) The failed transaction releases its own pending reservations and any temporary borrow obligations that would otherwise have belonged to the newly created execution. It must not release unrelated loans or invalidate pre-existing independent borrows.

§ 3(3) Creation-only temporary resources and caller-owned temporary argument values or environments must receive ordinary failure-path cleanup. No uncommitted reusable source may be destroyed merely because the creation attempt failed. No second owner or duplicate destructor may be created by rollback.

§ 3(4) If a native execution object, runtime control structure or bootstrap child was provisionally created before commit, the implementation must prevent user execution with uncommitted inputs and clean up the provisional state. A reported creation failure must not leave a live unowned child, hidden lifecycle obligation, unreleased creation resource or competing owner. For process bootstrap, retain `processes.md` § 8.8 in full.

§ 3(5) Creation failure is not a post-commit execution failure. After successful creation, panic, cancellation, native execution failure, a normal returned `Err(E)`, or a failure to start an already-created deferred thread does not roll back the earlier spawn ownership commit. Those events follow the owning lifecycle and outcome contracts.

§ 3(6) Unsupported target capabilities, statically invalid configuration, illegal ownership transfers and invalid borrows remain compile-time diagnostics where the owning rules require them. The transaction does not convert static errors into runtime spawn errors.

## § 4. Independently completed operations are not rolled back

§ 4(1) Only pending transfers belonging to the current spawn creation transaction are reversible under § 3. Independently completed nested calls, earlier moves, mutations, I/O and other observable evaluation effects are not undone.

§ 4(2) Construction of a closure environment follows ordinary closure rules. When an independently completed closure construction has already consumed a capture, later failed spawn creation does not restore that capture to its original binding. If the spawn consumes the reusable closure value itself, its pending transfer is subject to the same transaction as any other reusable owned input.

§ 4(3) Apply the same distinction to receiver evaluation and configuration construction. A resource already moved into an independently constructed owner is not restored to its earlier source. A pending direct transfer of that owner into the new execution is cancelled on failed creation. Temporary owners are cleaned up according to their actual ownership state. Existing `ThreadConfig` failure-cleanup rules remain applicable to configuration state actually owned by the failed creation machinery; they must not destroy a reusable source whose transfer never committed.

§ 4(4) If evaluation of a later argument fails before creation is attempted, the existing outer-call preparation and failure-cleanup rules apply. No successful spawn commit has occurred. Failure of argument evaluation must not be relabelled as a spawn creation error.

## § 5. Source-level ownership paths

§ 5(1) The following is a schematic ownership example. Assume `resource` is a valid reusable move-only input, `Work` accepts it by value, the selected boundary permits its transfer, and `UseResource` and `HandleOutcome` have compatible declarations. The example is not a new API declaration.

```sec
match spawn task Work(<-resource) {
    Ok(worker) => {
        // resource is unavailable here.
        let outcome := await worker
        HandleOutcome(outcome)
    }
    Err(error) => {
        // resource remains owned and available here.
        HandleCreationFailure(error)
        UseResource(ref resource)
    }
}
```

§ 5(2) Replacing the task creation with valid thread or process creation changes the handle, creation error and lifecycle resolution operation, but not the conditional availability of the transferred source.

§ 5(3) In the ordinary `try` propagation form:

```sec
let worker := try spawn Work(<-resource)
```

successful continuation has an unavailable `resource` Place and an owning task handle. On a propagated creation failure, `resource` remains owned by its original owner and is subject to ordinary cleanup on that path. No synthetic value is returned to restore the source.

§ 5(4) When control flow merges after success and failure, normal ownership-state merging applies. A source consumed on success and retained on failure is not unconditionally available merely because one predecessor retained it. There is no new implicit reinitialization or duplication operation.

§ 5(5) An independently constructed closure demonstrates the nested boundary:

```sec
let callback := capture(<-resource) fn() void {
    UseResource(ref resource)
}
// resource has already been consumed by closure construction.
// A later failed spawn of callback cannot restore resource.
```

The eventual spawn must obey the ordinary callable ownership and transferability requirements. No new syntax for transferring a callable is introduced by this correction.

## § 6. Cancellation and other execution boundaries

§ 6(1) MD-129 governs cancellation inheritance and the initial request state of a successfully created child task. A cancellation request already present in the parent is not by itself a creation error. A genuine creation error still follows this correction's rollback rules.

§ 6(2) If existing cancellation rules prevent completion of an operation before its creation commit, cleanup follows the uncommitted path; after commit, the owning handle and committed inputs follow ordinary lifecycle and cancellation cleanup. This paragraph does not designate `spawn` as a new cancellation point or specify any new interruptible creation operation.

§ 6(3) In deferred thread creation, ownership transfers at successful creation of the thread handle. A later `Start()` error does not retroactively return the inputs to the creator. Existing deferred-thread state, resource ownership and cleanup contracts govern that failure.

## § 7. Frontend, Semantic IR and runtime obligations

§ 7(1) Sema must distinguish prepared transfers, committed transfers and independently completed moves. On each creation-result path it must establish the correct Place availability, owner identity, active loans and lifecycle obligation. Match, try handlers, propagated errors and ordinary control-flow joins must preserve these facts.

§ 7(2) Semantic IR must retain the selected execution kind, prepared input Places, actual transfer/copy/borrow modes, creation commit boundary, success and failure paths, cleanup obligations, source provenance and the resulting handle owner. No particular opcode spelling or representation is mandated here.

§ 7(3) IR verification must reject a success path that leaves a consumed source available, a failure path that loses an uncommitted reusable source, duplicate destruction, user entry before input commit, or a reported creation error with an unresolved child lifecycle.

§ 7(4) Lowering and runtime creation machinery must implement the same transaction for each supported kind, respecting its memory-publication and transferability rules. Process adapters and provisional native threads may use different mechanisms without changing the source contract. A runtime strategy incapable of satisfying the transaction is not a valid implementation of that spawn form.

§ 7(5) Diagnostics and LSP ownership information must distinguish a move pending on a creation result from an independently committed move. Diagnostics must not suggest that creation `Err` destroyed a directly prepared reusable source or restored an earlier consumed capture.

## § 8. Normative integration map

Apply this correction without renumbering existing rulebook sections or duplicating the same normative contract under competing owners.

| Rulebook | Required amendment |
|---|---|
| `spawn.md` §§ 6, 10, 11 | Extend § 10(6)'s transaction to every kind; replace § 10(8)'s explicit deferral of a common rollback rule. Qualify ownership descriptions in § 11 by successful creation. Preserve publication and execution-kind distinctions. |
| `spawn.md` §§ 12–13, 16, 25–26 | Align pending/committed borrows, closure and receiver transfers, source-path analysis and Semantic IR with the common creation transaction. |
| `tasks.md` §§ 4–5 | State successful creation commits transfers, failed creation cancels pending transfers; keep the exact public creation-error set and post-creation outcome distinction. |
| `threads.md` §§ 15–20 | Make § 20(5) precise; reconcile § 20(6) with the actual ownership of reusable configurations versus temporaries. Preserve creation/start distinction. |
| `processes.md` §§ 8.6–8.8 | Preserve the existing transaction and bootstrap rollback; cross-reference the shared principle if needed. |
| `ownership.md` § 14 and § 14.2 | Add a narrow spawn creation-commit specialization; ordinary synchronous calls retain their existing call-entry commit. |
| `semantic_ir.md` and relevant lowering/runtime rulebooks | Cross-reference the required creation-commit, rollback and cleanup facts, preserving canonical operation ownership. |

Update the owning `concurrency.spawn-v1` governance fragment and cross-links to task/thread/process and ownership work. Resolve MD-128 in `missing-decisions.yaml` according to the repository's current closure convention after normative integration. Record implementation status separately from design approval. This document supersedes the MD-128 proposal in the earlier combined six-decision draft; it does not approve that draft's other proposals.

## § 9. Required conformance and regression coverage

Test vectors describe observable requirements rather than prescribe a particular test harness. Inject creation failures deterministically where real resource exhaustion would be unreliable. Run the common cases for all supported execution kinds, with appropriate transferability and lifecycle fixtures. An unsupported implementation must not be marked conforming merely because a test is skipped.

| ID | Scenario | Expected result |
|---|---|---|
| T01 | Successful task spawn consumes a named move-only input | Source unavailable; exactly one transferred owner |
| T02 | Task creation fails before commit | Source available in `Err` arm; no child entry |
| T03 | Successful physical-thread creation | Same ownership result as T01 |
| T04 | Thread creation fails before commit | Same ownership result as T02 |
| T05 | Successful process startup | Existing process transaction preserved |
| T06 | Process bootstrap fails after native child creation, before commit | Source retained; provisional child reaped; no user entry |
| T07 | Default `spawn` versus explicit `spawn task` | Identical ownership semantics |
| T08 | Multiple direct move inputs; creation fails | All still-pending transfers cancelled; each source retained |
| T09 | Multiple direct move inputs; creation succeeds | All commit; no partial caller ownership |
| T10 | Conflicting use of a reserved source during argument evaluation | Existing reservation/conflict diagnostic |
| T11 | Later argument evaluation fails before creation | Pending outer transfers cancelled; original evaluation error preserved |
| T12 | Earlier nested consuming call succeeded before failure | Its committed ownership transfer is not reversed |
| T13 | Earlier observable mutation or I/O occurred | Effect not reversed by creation failure |
| T14 | Earlier closure construction consumed a capture | Capture does not return to original binding |
| T15 | Failed pending transfer of a reusable closure owner | Closure owner retained; its existing capture state preserved |
| T16 | Failed creation with fresh temporary input | Temporary destroyed exactly once; no source restoration fabricated |
| T17 | Copyable input passed with ordinary copying | Original retained on both paths; prepared copy cleaned up on failure |
| T18 | Explicit consuming parameter receives copyable reusable input | Consuming transfer commits only on success |
| T19 | Failed creation with pending borrow | Transaction-owned loan released; unrelated loans unchanged |
| T20 | Successful creation with retained borrow | Existing lifetime/exclusivity obligation preserved until its canonical end |
| T21 | `try spawn` propagates creation failure | Still-owned source follows ordinary cleanup, exactly once |
| T22 | Merge of success and failure paths without reinitializing source | No unconditional availability of conditionally moved source |
| T23 | Successful deferred thread creation; later `Start()` failure | No rollback of successful creation ownership |
| T24 | Creation failure with independently constructed configuration | Pending transfer cancelled; prior configuration construction not reversed |
| T25 | Child runs immediately after creation commit | All inputs valid; no race with ownership publication |
| T26 | Child panics, cancels, fails, or returns `Err(E)` after commit | Creation remains successful; no source restoration |
| T27 | Parent already cancellation-requested and genuine creation failure | MD-128 rollback; MD-129 does not suppress real errors |
| T28 | Illegal process borrow or unsupported execution kind | Canonical compile-time diagnostic; no runtime rollback workaround |
| T29 | Malformed IR exposes both source and child owners on success | Verification fails |
| T30 | Malformed IR reports creation failure with child user execution | Verification fails |

## § 10. Source anchors and limits

- `missing-decisions.yaml`, MD-128: records the unresolved task/thread ownership boundary and the existing process precedent.
- `rules/concurrency/processes.md` §§ 8.6–8.8: transactional startup and provisional-child cleanup.
- `rules/concurrency/spawn.md` §§ 6, 9–16, 24–26: creation, evaluation, transfer, borrowing and analysis.
- `rules/concurrency/tasks.md` §§ 4–5 and `threads.md` §§ 15–20: fallible creation and thread start/commit boundaries.
- `rules/memory/ownership.md` §§ 11–14: ordinary call preparation, reservation and call-entry commit.

The owner approved a common transaction for all three kinds and the exclusion of independently completed moves from rollback. The additional analysis, cleanup and test requirements are consequences of that decision and the existing ownership/lifecycle contracts. No repository modification, runtime fault-injection run or compiler implementation verification is claimed by this document.
