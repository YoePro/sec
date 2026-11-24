# MD-129 — Task creation with an existing parent cancellation request

- **Language version:** Sec 0.1 (retained in Sec 0.1.1; no new source syntax)
- **Document revision:** 1.0
- **Date:** 2026-11-24
- **Status:** Owner-approved design; normative integration pending; implementation not verified
- **Decision:** MD-129 — `spawn-already-cancelled-parent`
- **Governance owner:** `concurrency.spawn-v1`
- **Repository baseline:** `YoePro/sec`, main `3ed6048c0b9e25b28030a49c1f7cbe62421cd5e1`
- **Related decisions:** MD-128 (transactional spawn ownership), MD-138 (callable execution requirements)
- **Primary rulebooks:** `rules/concurrency/spawn.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/tasks.md`, `rules/concurrency/concurrency.md`
- **Related rulebooks:** `rules/concurrency/threads.md`, `rules/concurrency/processes.md`, `rules/concurrency/await.md`, `rules/concurrency/scheduling.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/compiler/semantic_ir.md`

## § 1. Approved decision and scope

§ 1(1) An existing cooperative cancellation request on a parent task is not, by itself, a reason for `spawn` or `spawn task` to fail. Creation remains fallible for its normal creation-error reasons.

§ 1(2) A successfully created child task with a structured parent-child cancellation relationship receives the parent's existing cancellation request. A task without that relationship does not inherit a cancellation request merely because some execution entity created it.

§ 1(3) The existing `detach` operation breaks the structured parent-child cancellation relationship for future propagation. It does not revoke a cancellation request that the child has already received.

§ 1(4) Preserve the distinction between logical task cancellation, physical-thread cancellation, explicitly passed `Context` cancellation and process termination. No automatic cross-kind inheritance is introduced.

§ 1(5) This correction changes no public error type, source grammar or cancellation API. It resolves the deferred semantic choice in `spawn.md` § 24(3) and `cancellation.md` § 55 for Sec 0.1 and later compatible versions.

## § 2. Logical parenthood and cancellation linkage

§ 2(1) Determine the actual structured task relationship under `spawn.md` § 9, `tasks.md` § 8 and `concurrency.md` § 19. Cancellation inheritance applies to a real parent-child cancellation link, not to lexical nesting, handle ownership alone, the identity of a physical worker, or the fact that a callable uses task-related operations.

§ 2(2) A task spawned from ordinary program entry or a native thread without a current logical task has no fictitious task parent. No implicit root `TaskContext` may be invented for inheritance.

§ 2(3) Parent and child retain separate logical task identities and separate request state. The inherited request sets the child's own cooperative request state; it does not alias the parent's flag or merge their identities.

§ 2(4) This decision does not introduce a new detached-at-creation syntax. A child that initially has a parent is subject to that relationship until the existing detach operation commits. A later detach must not be interpreted as evidence that the task was parentless at creation.

§ 2(5) Ownership and cancellation relationship are distinct semantic facts. Moving or returning a handle continues to transfer the lifecycle obligation according to the existing rules. This correction does not define a general reparenting operation or change relationship semantics for handle transfer beyond the approved detach behavior.

## § 3. Creation with an existing cancellation request

§ 3(1) If the parent has a committed cancellation request when a linked child task is successfully established, the child must begin with its own cancellation request pending. This includes a request made before the spawn operation began.

§ 3(2) The inherited request must be established before the new task is allowed to observe its own cancellation state or execute a cancellation point. A child must not temporarily observe `Task.Current().CancelRequested == false` merely because initialization or publication of the inherited request was delayed.

§ 3(3) An already-requested parent may create a child successfully when all other language, target and runtime creation requirements are met. The raw result remains `Result[Task[T], TaskSpawnError]`; success produces an owning `Task[T]`. No `Cancelled` creation-error variant or cancellation-specific spawn failure is introduced.

§ 3(4) Successful creation does not imply that the child has already reached a terminal outcome. The child may observe the pending request and perform ordinary cooperative cancellation, reach an existing cancellation point, return normally before terminal cancellation commits, panic or fail according to the existing task outcome model. Creation itself must not fabricate `TaskOutcome.Cancelled`.

§ 3(5) A parent with a pending request is not the same as a parent whose execution has already terminated. This rule applies to otherwise legal reachable creation operations. It does not authorize execution of source code after terminal cancellation, bypass cleanup restrictions, or change when `cancel` terminates the current entity.

§ 3(6) If the task has no parent-child cancellation link, an existing request on its creator does not set the child's request state. Independently delivered requests and explicitly shared `Context` state continue to follow their owning rules.

## § 4. Detach and retained cancellation state

§ 4(1) The existing detach operation consumes the task handle and transfers lifecycle responsibility according to `tasks.md` § 22. In addition to its existing lifecycle effect, a successful detach breaks the structured parent-child cancellation propagation link for that task.

§ 4(2) Cancellation requests committed to the child before the detach boundary remain pending afterward, if the child is still in an applicable nonterminal state. Detach must not clear, undo, shield or replace an already received cancellation request.

§ 4(3) After detach commits, a later cancellation request on the former parent is not propagated to the child through the broken link. Independently valid cancellation mechanisms, including separately shared `Context` relationships, retain their own contracts.

§ 4(4) The following schematic sequence assumes that the task was created with a structured cancellation link and that the parent already had a pending request:

```sec
let worker := try spawn Work()
detach worker
```

On successful creation, the child receives the request before it can run. Detach then breaks the future link; it does not undo the request. The result is the same whether the child first runs before or after detach. The child's eventual outcome still follows its cooperative behavior and normal scheduling.

§ 4(5) For a task whose normal return value requires explicit disposal, the existing `detach worker discard` form remains required. Detach does not dispose of an unhandled spawn creation error, permit invalid escaping borrows, or relax transferability checks.

§ 4(6) Detach does not establish completion synchronization or guarantee immediate termination. The existing detached-task lifecycle, resource cleanup and shutdown contracts remain unchanged.

## § 5. Concurrent requests, creation and detachment

§ 5(1) Creation of a structured cancellation link and publication of its initial request state must be coordinated with concurrent parent cancellation. There must be no lost request caused by checking the parent before publishing the link and then failing to observe a request that arrives between those operations.

§ 5(2) A parent request committed before successful link establishment must be reflected in the child's initial request state. If the parent request commits after link establishment while the link remains active, the existing propagation obligation applies. Failed creation publishes no lasting child relationship.

§ 5(3) Detach and propagation must have a well-defined logical ordering. A cancellation request received while the link was active cannot be revoked because detach wins a later scheduling race. A request committed by the former parent after link removal must not reach the child through that removed link.

§ 5(4) The implementation must coordinate cancellation publication and link removal so that a request owed by a still-active relationship is not silently lost merely because propagation work was queued when detach occurred. The implementation may synchronize publication, record the owed request, or use another equivalent mechanism. This is a required consequence of preserving committed requests and breaking only future propagation, not a prescribed locking or atomic implementation.

§ 5(5) Already-terminal task outcomes remain immutable under the existing cancellation rules. Neither delayed request propagation nor detach may replace an established outcome.

§ 5(6) Cancellation state and relationship transitions must be data-race-free. Apply `cancellation.md` § 18 and `concurrency_memory_model.md`. Cancellation publication does not become a general memory-publication mechanism for unrelated application data.

## § 6. Symmetry and independence of execution kinds

The existing creation syntax remains symmetric. Cancellation behavior follows the actual execution-kind contract rather than inventing a common cancellation identity.

| Selected creation / relationship | Existing creator request | Child behavior on successful creation |
|---|---|---|
| `spawn` / `spawn task`, linked logical task parent | Task cancellation requested | Child's own task cancellation request pending at entry |
| `spawn` / `spawn task`, no parent cancellation link | Request on creator, if applicable | No inherited task request |
| `spawn thread` from a task | Task cancellation requested | No inherited task cancellation on physical thread |
| `spawn process` from a task | Task cancellation requested | No implicit process termination |
| Any applicable form with explicit `Context` | Context already cancelled or deadline exceeded | Observe/handle the Context through its own existing contract; do not manufacture execution cancellation |

§ 6(1) `Thread.Current().CancelRequested` continues to observe the current physical thread identity. A parent task's pending request is not a physical-thread request. This correction does not define any new thread-to-thread inheritance policy.

§ 6(2) Process creation and termination remain independent under `processes.md` § 29. A task request does not implicitly terminate, detach or otherwise resolve the lifecycle of a process.

§ 6(3) `Context`, `ContextSource`, and their derived branches remain separate from current-execution cancellation. This correction neither adds a `Context` transfer facility for processes nor changes existing transferability requirements.

§ 6(4) No new execution-context requirement is introduced. Existing inferred requirements, and the explicit Sec 0.1.1 callable requirement syntax of MD-138 when integrated, must still be satisfied by the selected child execution context.

## § 7. Ownership, errors and cleanup

§ 7(1) MD-128 supplies the common ownership-commit rule. On a genuine task creation failure, pending direct source transfers are cancelled; on success, ownership commits exactly once, even when the child begins with cancellation requested.

§ 7(2) A subsequent cooperative cancellation of the successfully created child is not grounds for rolling back the creation commit. Child-owned inputs follow ordinary destruction and cleanup contracts, including applicable `defer` behavior. No moved value is restored to the parent merely because the child cancels before useful work begins.

§ 7(3) A successful creation always establishes its normal handle lifecycle obligation. A pending cancellation request is not a substitute for await, join, transfer or detach. An already requested child may continue to execute until it commits a terminal outcome.

§ 7(4) Existing await caller-cancellation cleanup remains unchanged (`await.md` §§ 17–24, 52). A request made by that cleanup is an independently defined cancellation operation on the internally owned handle, not a new rule for parentless tasks or detached children.

§ 7(5) This correction does not make `spawn` a new cancellation point, modify shutdown policy, or prescribe a scheduling algorithm. Existing cancellation-point rules determine whether and when the current execution can commit terminal cancellation.

## § 8. Frontend, runtime and Semantic IR obligations

§ 8(1) Sema must preserve the distinction between a real current task context, a physical-thread context and the absence of a logical task parent. Existing lifetime/ownership checks for creation and detach remain mandatory. No statically known parent cancellation request may be treated as an invalid creation configuration solely because it is pending.

§ 8(2) Semantic IR and runtime contracts must retain the creation kind, structured cancellation relationship, initialization/publication of the child's own request state, relationship-removal operation on detach, creation outcome and lifecycle ownership. The representation may use runtime state where the request is not known statically; the compiler must not guess its value.

§ 8(3) Runtime creation and scheduling must prevent observation of an incorrect initial state. Runtime cancellation and detach must coordinate relationship publication, request delivery and link removal as required by § 5.

§ 8(4) Verification and lowering must not replace logical task identity with a physical worker identity, introduce implicit detach, clear pending cancellation on detach, or encode an existing request as a creation error. Exact low-level operation names and representations remain owned by the relevant compiler/runtime specifications.

§ 8(5) Tooling must describe the distinction between request pending, successful creation and terminal cancellation. It must not imply that detaching an already-requested child resets its state.

## § 9. Normative integration map

Preserve existing section numbers. Insert stable subordinate paragraphs where needed; use a shared owning contract rather than maintaining divergent copies of the same rules.

| Rulebook | Required amendment |
|---|---|
| `spawn.md` §§ 6, 8–9, 15, 24–26 | Replace § 24(3)'s explicit deferral with the decided initialization rule; cross-reference detach semantics and preserve analysis/IR facts. |
| `cancellation.md` §§ 2, 17–18, 32, 55 | Extend § 55 with the canonical linked-child initialization, detach and race-ordering rules; retain cooperative and terminal-outcome distinctions. |
| `tasks.md` §§ 4–5, 8, 22 | Clarify that pending parent cancellation is not a creation error; define future cancellation-link removal at successful detach and retention of already received requests. |
| `concurrency.md` §§ 19, 21 | Cross-reference structured cancellation linkage, detachment and cooperative cleanup; preserve ownership obligations. |
| `threads.md` § 46 and `processes.md` § 29 | Preserve independent execution identities and existing cancellation/termination contracts; add cross-references only where necessary. |
| `await.md` §§ 17–24, 52 | Preserve existing caller-cancellation cleanup; distinguish its explicit request from relationship propagation. |
| Scheduling, concurrency memory model and Semantic IR rulebooks | Carry the necessary state-publication, relationship and commit facts; do not introduce a second cancellation system. |

Update the owning `concurrency.spawn-v1` governance fragment and cross-links to the cancellation and task owners. Resolve MD-129 in `missing-decisions.yaml` according to the repository's closure convention after normative integration. Record implementation status independently. Integrate MD-128 first or together with this correction. This document supersedes the MD-129 proposal in the earlier combined six-decision draft; it does not approve that draft's other proposals.

## § 10. Required conformance and regression coverage

Use deterministic runtime or scheduler fixtures to establish the relevant request, creation and detach ordering. Test observable source semantics and runtime invariants, not a particular scheduling implementation. Do not treat missing implementation support as passing conformance.

| ID | Scenario | Expected result |
|---|---|---|
| T01 | Linked child created after parent request is committed | Successful creation if resources permit; child starts requested |
| T02 | Parent not requested at linked creation | No inherited request at entry |
| T03 | Parent request arrives during preparation, before creation commit | Child's first possible observation is true |
| T04 | Parent request arrives after link establishment, while linked | Normal propagation; no lost request |
| T05 | Child scheduled immediately after successful creation | Correct initial state before first observation/checkpoint |
| T06 | Child scheduled only after creator detaches it | Existing request remains pending |
| T07 | Child runs before detach, observes true | Detach does not reset the request |
| T08 | Child detached before any parent request | Later former-parent request not propagated through old link |
| T09 | Parent request and detach race | Outcome consistent with logical ordering; no received request revoked |
| T10 | Propagation work queued before detach | Request owed by the active relationship is not silently lost |
| T11 | Child already terminal when propagation/detach occurs | Terminal outcome unchanged; ordinary lifecycle resolution |
| T12 | Pending parent request and actual resource creation failure | Normal `TaskSpawnError`; MD-128 rollback; no published child link |
| T13 | Linked child created with moved owned resource and inherited request | Creation commits transfer; later cancellation cleans child-owned value |
| T14 | Child observes request and executes `cancel` | Normal cancelled outcome and deterministic cleanup |
| T15 | Requested child returns normally before terminal cancellation | Normal completed outcome; no fabricated cancellation |
| T16 | Requested child panics or otherwise fails after creation | Existing terminal classification; no reclassification as creation error |
| T17 | Task spawned from ordinary `main` without a logical parent | No fictitious task parent or inherited request |
| T18 | Task spawned from ordinary physical-thread context without a logical parent | No task-parent inheritance |
| T19 | `spawn thread` from cancellation-requested task | New thread does not inherit task cancellation identity/request |
| T20 | `spawn process` from cancellation-requested task | No implicit termination or lifecycle discharge |
| T21 | Task has no parent link but receives an independently valid explicit request | Explicit request retains normal semantics |
| T22 | Cancelled shared `Context` passed to an otherwise unrequested task | Context-aware operation sees its context error; no fabricated task request |
| T23 | Detach task using independently shared Context | Detach does not break the independent Context relationship |
| T24 | Detach value-returning task without required discard intent | Existing detach diagnostic |
| T25 | Detach task with invalid escaping borrow | Existing lifetime/transferability diagnostic |
| T26 | Await caller-cancellation cleanup consumes child handle | Existing request/wait/disposal discipline; no implicit detach |
| T27 | Successful creation followed by unresolved handle abandonment | Existing lifecycle diagnostic; cancellation is not a waiver |
| T28 | Invalid IR clears request at detach or substitutes worker identity | Verification rejects invalid representation |

## § 11. Source anchors and limits

- `missing-decisions.yaml`, MD-129: explicitly defers success-versus-error behavior for an already-requested parent.
- `rules/concurrency/spawn.md` §§ 6, 8–9, 15, 24–26: creation, parenthood, detach and cancellation relationship facts.
- `rules/concurrency/cancellation.md` §§ 2, 14–18, 32, 41, 44–45, 55: cooperative request/commit model, distinct identities, memory semantics and Context independence.
- `rules/concurrency/tasks.md` §§ 4–5, 8, 22 and `concurrency.md` §§ 19, 21: task creation errors, structured relationships and detach.
- `rules/concurrency/threads.md` §§ 46–48 and `processes.md` § 29: independent thread/process control.
- `rules/concurrency/await.md` §§ 17–24, 52: existing cancellation cleanup and lifecycle resolution.

The owner explicitly approved successful creation with an inherited request for linked children, no inheritance without that relationship, and detach removing future propagation without revoking a received request. The logical ordering and publication requirements make those decisions race-safe; they do not prescribe a runtime mechanism or introduce new cancellation points. No repository modification or implementation/conformance run is claimed.
