# Select

- **Status:** Normative
- **Created:** 2026-09-23
- **Last updated:** 2026-09-23
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/select.md`
- **Replaces:** Earlier legacy revision at the same canonical path
- **Repository baseline reviewed:** `main-reviewed-2026-09-23`
- **Implementation governance:** `governance/concurrency_select.yaml`
- **Related rulebooks:** `rules/concurrency/channels.md`, `rules/concurrency/await.md`, `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/concurrency/processes.md`, `rules/concurrency/ipc.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/blocking.md`, `rules/concurrency/mutex.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/concurrency/structured_concurrency.md`, `rules/compiler/semantic_ir.md`, `rules/platform/target_profiles.md`, `rules/tooling/lsp.md`

---

## § 1. Purpose and authority

**Governance tags:** `concurrency.select-v2`

§ 1(1) `select` waits on a finite set of compiler-recognized concurrent operations and commits exactly one branch.

§ 1(2) `select` is not a general conditional expression and is not a mechanism for polling arbitrary values.

§ 1(3) This rulebook owns `select` syntax, branch preparation, readiness, source-order priority, commit, `default`, `after`, cancellation interaction, ownership across candidate branches, the Sec 0.1 set of selectable operation categories, and select-specific compiler/runtime/tooling obligations.

§ 1(4) The operation-owning rulebook remains authoritative for each selected operation's return type, ownership effects, lifecycle effects, synchronization, errors, and target restrictions.

§ 1(5) Mutable implementation status belongs in `governance/concurrency_select.yaml`.

---

## § 2. Core distinction

**Governance tags:** `concurrency.select-v2`

§ 2(1) The core control-flow distinction is:

```text
if
    chooses from boolean control flow

switch
    chooses from values or conditions

match
    chooses from patterns/variants

select
    chooses from compiler-known concurrent operations that may become ready
```

§ 2(2) A normal function or property does not become selectable merely because it is fast, slow, blocking, asynchronous internally, or named like a wait operation.

§ 2(3) Selectability is a normative property of a canonical operation.

---

## § 3. Core syntax

**Governance tags:** `concurrency.select-v2`, `frontend.select-v2`

§ 3(1) The canonical form is:

```sec
select {
    first := operationA => {
        HandleA(first)
    }

    second := operationB => {
        HandleB(second)
    }
}
```

§ 3(2) A branch may omit a binding when its selected operation returns `void` or when ordinary discard rules allow the result to be ignored.

§ 3(3) `default` and `after` are select-language branches and are not ordinary function calls.

§ 3(4) A `select` executes one branch at most once per execution of the statement.

---

## § 4. Exactly one branch

**Governance tags:** `concurrency.select-v2`

§ 4(1) If one or more branches are ready, exactly one branch commits.

§ 4(2) A non-selected branch does not commit.

§ 4(3) After the selected branch body finishes, control continues after the `select` unless ordinary branch control flow exits elsewhere.

§ 4(4) A blocking select with no ready branch waits until at least one branch becomes ready, current-execution cancellation wins, or another specified control-flow termination condition applies.

---

## § 5. Source-order priority

**Governance tags:** `concurrency.select-v2`

§ 5(1) Ready branches are selected by source order.

§ 5(2) If multiple branches are ready at the same semantic selection point, the earliest ready branch wins.

```sec
select {
    critical := criticalRx.Receive() => {
        HandleCritical(critical)
    }

    normal := normalRx.Receive() => {
        HandleNormal(normal)
    }
}
```

§ 5(3) If both receives are ready, `criticalRx.Receive()` wins.

§ 5(4) The compiler/runtime must not introduce hidden round-robin or random fairness that overrides source order.

§ 5(5) Source-order priority is visible program semantics.

---

## § 6. Starvation

**Governance tags:** `concurrency.select-v2`

§ 6(1) A later branch may starve if an earlier branch remains continuously ready.

§ 6(2) This follows from the explicit source-order priority rule.

§ 6(3) Sec 0.1 provides no implicit fairness mode for `select`.

---

## § 7. Preparation, readiness, commit

**Governance tags:** `concurrency.select-v2`, `semantic-ir.select-v2`

§ 7(1) Every ordinary selectable branch has three semantic phases:

```text
prepare
readiness
commit
```

§ 7(2) Preparation evaluates/stages the operation's branch operands exactly once for this select execution.

§ 7(3) Readiness determines whether the operation can semantically complete without further waiting.

§ 7(4) Commit performs the operation and applies its ownership/state/synchronization effects.

§ 7(5) Preparation is not commit.

§ 7(6) Readiness is not commit.

§ 7(7) Only the selected branch may commit.

---

## § 8. Non-destructive readiness

**Governance tags:** `concurrency.select-v2`

§ 8(1) Readiness checks must be non-destructive.

§ 8(2) Readiness must not move an application-owned payload irreversibly, dequeue/enqueue a message, acquire a mutex, consume a semaphore permit, reap a process, consume a task handle or join capability, create a revocation ticket, mutate a user buffer as a committed pipe read, write pipe bytes, increment operation statistics, or invoke arbitrary user callbacks.

§ 8(3) Internal runtime registration/bookkeeping is permitted when it is observationally reversible and has no source-visible operation effect before commit.

---

## § 9. Prepared ownership

**Governance tags:** `concurrency.select-v2`, `analysis.transferability`

§ 9(1) A candidate operation may require a move-only source value if the operation would consume that value on commit.

§ 9(2) The select context may hold conditional/prepared ownership while waiting.

§ 9(3) Prepared ownership is not final ownership transfer to the operation.

§ 9(4) If another branch wins, ownership associated only with a non-selected branch returns to the source control-flow state when execution continues normally.

§ 9(5) If current execution terminates through cancellation, still-owned prepared values are cleaned up through ordinary cancellation/destruction rules.

§ 9(6) Sema and Semantic IR must represent this conditional ownership explicitly.

---

## § 10. Branch result typing

**Governance tags:** `concurrency.select-v2`, `frontend.select-v2`

§ 10(1) A selected ordinary operation produces exactly its normal declared result type.

§ 10(2) `select` does not widen or narrow that type.

```sec
select {
    result := tx.Send(<-message) => {
        HandleSend(<-result)
    }

    after 1<s> => {
        HandleTimeout(message)
    }
}
```

§ 10(3) Here `result` has exactly `ChannelSendResult[T]` for the concrete channel message type `T`.

---

## § 11. Complete Sec 0.1 selectable categories

**Governance tags:** `concurrency.select-v2`

§ 11(1) Sec 0.1 recognizes the following operation categories as selectable when their owning rulebook and selected target support them:

```text
ordinary in-process channel:
    Sender[T].Send(...)
    Sender[T].SendRevocable(...)
    Receiver[T].Receive()

task:
    await Task[T]
    join Task[T]

thread:
    join Thread[T]

process:
    join Process[T]
    join Command
    ProcessObserver.Wait()

pipe:
    PipeReader.Read(...)
    PipeWriter.Write(...)

typed IPC:
    IPCSender[T].Send(...)
    IPCReceiver[T].Receive()

process-shared synchronization:
    IPCMutex.Lock()
    IPCSemaphore.Acquire()

select language:
    after duration
    default
```

§ 11(2) This list is closed for Sec 0.1 except where another already-canonical rulebook explicitly defines an additional selectable operation and is synchronized with this book.

§ 11(3) Library or user code cannot create a selectable operation by convention alone.

---

## § 12. Operations explicitly not selectable

**Governance tags:** `concurrency.select-v2`

§ 12(1) The following are not primitive selectable operations in Sec 0.1:

```text
Sender[T].TrySend(...)
Receiver[T].TryReceive()
MessageTicket[T].Revoke()
Sender[T].Share()
Receiver[T].Discard()
Sender[T].Statistics()
Receiver[T].Statistics()
endpoint Close operations

IPCMutex.TryLock()
IPCSemaphore.TryAcquire()
IPCSemaphore.Release()

PipeReader.ReadExact(...)
PipeWriter.WriteAll(...)

Task.Current().CancelRequested
Thread.Current().CancelRequested

ordinary properties
ordinary arbitrary function calls
```

§ 12(2) A nonblocking `Try*` operation is already an immediate ordinary operation and gains no useful waiting semantics from primitive `select`.

§ 12(3) A boolean cancellation-request property is observation, not a readiness source.

---

## § 13. Why channel `Try*` is excluded

**Governance tags:** `concurrency.select-v2`, `concurrency.channels-v2`

§ 13(1) `TrySend()` always returns immediately with `ChannelTrySendResult[T]`, whose exact variants are `Sent`, `WouldBlock(T)`, and `Closed(T)`.

§ 13(2) `TryReceive()` always returns immediately with `ChannelTryReceiveResult[T]`, whose exact variants are `Received(T)`, `Empty`, and `Closed`.

§ 13(3) Treating these as selectable would make the branch continuously immediate rather than a true waiting candidate.

§ 13(4) Programs needing nonblocking polling call these operations normally outside primitive select semantics.

---

## § 14. Ordinary channel receive

**Governance tags:** `concurrency.select-v2`, `concurrency.channels-v2`

§ 14(1) The selectable receive operation is:

```sec
fn Receive() Option[T]
```

§ 14(2) The branch is ready when a deliverable message is available or the send side is closed and drained.

§ 14(3) If selected, the branch receives exactly `Option[T]`.

§ 14(4) `Some(message)` transfers message ownership; `None` means permanently closed and drained.

§ 14(5) A non-selected receive removes no message and advances no committed receive state.

---

## § 15. Ordinary channel send

**Governance tags:** `concurrency.select-v2`, `concurrency.channels-v2`

§ 15(1) The selectable blocking send operations are:

```sec
fn Send(message: T) ChannelSendResult[T]
fn Send(message: T, lifetime: duration) ChannelSendResult[T]
```

§ 15(2) The lifetime overload requires the channel's `Expiration` capability.

§ 15(3) A send branch is ready when it can immediately return `Sent` or `Closed(T)`.

§ 15(4) A full open buffered channel is not ready.

§ 15(5) An open rendezvous channel without a matching receiver is not ready.

---

## § 16. Non-selected channel send

**Governance tags:** `concurrency.select-v2`, `concurrency.channels-v2`

§ 16(1) A non-selected send branch must not transfer ownership, enqueue, rendezvous, destroy the message, start post-commit expiration, increment channel statistics, or alter sender/channel lifecycle state.

§ 16(2) If another branch wins and normal execution continues, the prepared message remains available to source code.

§ 16(3) This rule applies equally to move-only `T`.

---

## § 17. Revocable channel send

**Governance tags:** `concurrency.select-v2`, `concurrency.channels-v2`

§ 17(1) The selectable operations are:

```sec
fn SendRevocable(message: T) ChannelRevocableSendResult[T]
fn SendRevocable(
    message: T,
    lifetime: duration
) ChannelRevocableSendResult[T]
```

§ 17(2) The exact selected result is:

```sec
type ChannelRevocableSendResult[T] union {
    // The revocable send committed.
    Accepted(MessageTicket[T])

    // RX closed before commit; ownership returns.
    Closed(T)
}
```

§ 17(3) A non-selected revocable send creates no `MessageTicket[T]` and starts no message lifetime.

§ 17(4) `MessageTicket[T].Revoke()` is not selectable.

---

## § 18. Task await branch

**Governance tags:** `concurrency.select-v2`, `concurrency.await-v2`

§ 18(1) `await Task[T]` is selectable.

```sec
select {
    outcome := await first => {
        HandleFirst(<-outcome)
    }

    outcome := await second => {
        HandleSecond(<-outcome)
    }
}
```

§ 18(2) An await branch is ready when the task has a terminal outcome that the select operation can commit/acquire without further waiting.

§ 18(3) If selected, the branch performs canonical await commit and yields exactly `TaskOutcome[T]`.

§ 18(4) The selected await consumes that `Task[T]` handle.

§ 18(5) A non-selected await does not consume, cancel, detach, join, or take the task outcome.

---

## § 19. Await branch and select cancellation

**Governance tags:** `concurrency.select-v2`, `concurrency.await-v2`, `concurrency.cancellation-v2`

§ 19(1) Before branch commit, the select owns only prepared conditional access to an await candidate; it has not performed consuming await.

§ 19(2) If current-execution cancellation wins while no branch has committed, no await branch commits.

§ 19(3) The task handles remain source-owned lifecycle values subject to surrounding cancellation/structured-cleanup rules.

§ 19(4) If an await branch commits first, `await.md` becomes authoritative from the commit point onward.

---

## § 20. Task join branch

**Governance tags:** `concurrency.select-v2`, `concurrency.tasks-v2`

§ 20(1) `join Task[T]` is selectable in Sec 0.1.

§ 20(2) The branch is ready when the task is terminal and its outstanding join capability can commit without further waiting.

§ 20(3) Selected task join establishes task-completion synchronization, consumes the one-shot join capability, preserves the `Task[T]` handle for terminal inspection, and does not transfer `TaskOutcome[T]`.

§ 20(4) A non-selected task join consumes no join capability and changes no task lifecycle state.

§ 20(5) Await and join remain distinct selectable operations.

---

## § 21. Thread join branch

**Governance tags:** `concurrency.select-v2`, `concurrency.thread-v2`

§ 21(1) `join Thread[T]` is selectable in Sec 0.1.

§ 21(2) The branch is ready when the thread is terminal and its join capability can commit without further waiting.

§ 21(3) Selected thread join establishes thread-completion synchronization, consumes the one-shot join capability, releases join-owned native resources and any explicit-storage borrow when cleanup permits, preserves terminal handle state, and preserves any unconsumed normal result.

§ 21(4) The source `Thread[T]` binding remains available after selected join for terminal inspection/value access according to `threads.md`.

§ 21(5) A non-selected thread join consumes no join capability, releases no join-owned resource or storage borrow, unlocks no terminal payload, and creates no completion synchronization edge.

---

## § 22. Process join branch

**Governance tags:** `concurrency.select-v2`, `concurrency.process-v1`

§ 22(1) `join Process[T]` is selectable.

§ 22(2) The branch is ready when the complete process terminal outcome is known and owning join can commit without further waiting.

§ 22(3) If selected, process join performs the same canonical lifecycle resolution as ordinary process join, including required native collection/reaping.

§ 22(4) The resolved owner remains available for terminal inspection and any still-owned normal result.

§ 22(5) A non-selected process join does not join, reap, detach, terminate, unlock terminal payloads, or consume join capability.

---

## § 23. Command join branch

**Governance tags:** `concurrency.select-v2`, `concurrency.process-v1`

§ 23(1) `join Command` is selectable when the `Command` owns an established started process and has an available join capability.

§ 23(2) Readiness/commit follows canonical `Command` process lifecycle rules.

§ 23(3) A non-selected command join does not reap, detach, terminate, or otherwise alter command/process lifecycle.

---

## § 24. Unified join rule

**Governance tags:** `concurrency.select-v2`

§ 24(1) Across task, thread, process, and command, selectable join follows one common source-level model:

```text
readiness
    execution is terminal and join can commit immediately

selected commit
    consume the one-shot join capability
    establish completion synchronization
    preserve the handle value according to its owning rulebook

non-selected
    consume nothing
    perform no lifecycle resolution
```

§ 24(2) The exact resources released/unlocked by join remain execution-kind-specific.

§ 24(3) Unified syntax does not imply identical terminal status types or one generic JoinResult.

---

## § 25. Process observer wait

**Governance tags:** `concurrency.select-v2`, `concurrency.process-v1`

§ 25(1) `ProcessObserver.Wait()` is selectable because the process rulebook defines the exact public waiting operation:

```sec
fn Wait() ProcessStatus
```

§ 25(2) Its branch is ready when the observed process status is terminal.

§ 25(3) If selected, it returns exactly `ProcessStatus` and does not join, reap, detach, terminate, consume an owner join capability, or unlock owner-only payloads.

§ 25(4) Observer wait remains repeatable according to the process rulebook.

---

## § 26. Task/thread observer boundary

**Governance tags:** `concurrency.select-v2`, `concurrency.tasks-v2`, `concurrency.thread-v2`

§ 26(1) Sec 0.1 select does not invent a hidden wait operation for `TaskObserver` or `ThreadObserver`.

§ 26(2) An owning book saying that an observer may conceptually participate in completion observation/select is insufficient without an exact public selectable operation.

§ 26(3) Task/thread observer selection becomes available only when the owning rulebook defines and synchronizes an exact operation surface.

§ 26(4) `ProcessObserver.Wait()` is unaffected because its exact public `Wait()` surface already exists.

---

## § 27. Pipe read/write

**Governance tags:** `concurrency.select-v2`, `concurrency.ipc-v1`

§ 27(1) Primitive selectable pipe operations are:

```sec
fn Read(buffer: ref mut byte[]) Result[uint, IOError]
fn Write(data: ref byte[]) Result[uint, IOError]
```

§ 27(2) A non-selected `Read` consumes no bytes and does not modify the user buffer as a committed read.

§ 27(3) A non-selected `Write` commits zero bytes.

§ 27(4) `ReadExact()` and `WriteAll()` are composed operations and are not primitive selectable operations in Sec 0.1.

---

## § 28. Typed IPC send/receive

**Governance tags:** `concurrency.select-v2`, `concurrency.ipc-v1`

§ 28(1) The selectable typed IPC operations are:

```sec
fn Send(message: T) Result[void, IPCError]
fn Receive() Result[Option[T], IPCError]
```

on their canonical `IPCSender[T]` and `IPCReceiver[T]` owners.

§ 28(2) A selected operation yields its ordinary exact result type.

§ 28(3) A non-selected IPC send commits/materializes/transfers nothing.

§ 28(4) A non-selected IPC receive consumes/materializes no message.

§ 28(5) Ownership on selected pre-commit IPC failure follows IPC's transactional rollback contract.

---

## § 29. IPC synchronization select

**Governance tags:** `concurrency.select-v2`, `concurrency.ipc-v1`

§ 29(1) The selectable synchronization operations are:

```sec
fn Lock() Result[IPCMutexGuard, IPCSyncError]
fn Acquire() Result[void, IPCSyncError]
```

on `IPCMutex` and `IPCSemaphore` respectively.

§ 29(2) A non-selected mutex branch acquires no mutex and creates no guard.

§ 29(3) A non-selected semaphore branch consumes no permit.

§ 29(4) `IPCMutex.TryLock()` and `IPCSemaphore.TryAcquire()` are not selectable because they are already nonblocking.

§ 29(5) `IPCSemaphore.Release()` is not a waiting select candidate.

---

## § 30. `after` branch

**Governance tags:** `concurrency.select-v2`, `frontend.temporal-duration`

§ 30(1) The canonical timeout branch is:

```sec
after duration => {
    HandleTimeout()
}
```

§ 30(2) The expression must be type-compatible with canonical `duration`.

§ 30(3) Compatible unit syntax may materialize a duration:

```sec
after 100<ms> => {
    HandleTimeout()
}
```

§ 30(4) The timeout interval starts when select prepares that branch.

§ 30(5) Timing uses a monotonic time source; wall-clock adjustments do not alter the deadline.

---

## § 31. Zero duration

**Governance tags:** `concurrency.select-v2`

§ 31(1) A zero-duration `after` branch is immediately ready.

§ 31(2) It remains subject to source-order priority.

---

## § 32. Target timer requirement

**Governance tags:** `concurrency.select-v2`, `compiler.platform-model`

§ 32(1) A select containing positive-duration `after` requires a selected target/profile capable of implementing canonical monotonic timeout.

§ 32(2) A statically unsupported target produces a compile-time diagnostic.

§ 32(3) The runtime must not silently substitute wall-clock time.

§ 32(4) This rulebook does not create a runtime `SelectError` for statically unsupported timer capability.

---

## § 33. `default` branch

**Governance tags:** `concurrency.select-v2`

§ 33(1) A select may contain one `default` branch.

§ 33(2) `default` is selected only if no earlier ordinary/`after` branch is ready at the selection point.

§ 33(3) A select with `default` does not wait.

§ 33(4) `default` performs no readiness registration.

---

## § 34. Default placement and reachability

**Governance tags:** `concurrency.select-v2`, `frontend.select-v2`

§ 34(1) `default` must be the final branch and at most one `default` is permitted.

§ 34(2) A non-final default is a compile-time error.

§ 34(3) A positive-duration `after` followed by final `default` is unreachable because default wins immediately whenever no earlier branch is ready.

§ 34(4) Such dead timeout branches must be diagnosed.

§ 34(5) A zero-duration `after` before final `default` makes that later default unreachable by source-order readiness.

---

## § 35. Current-execution cancellation

**Governance tags:** `concurrency.select-v2`, `concurrency.cancellation-v2`

§ 35(1) A potentially waiting `select` is a current-execution cancellation point.

§ 35(2) If cancellation commits before any branch commit, no select branch commits.

§ 35(3) Prepared but uncommitted state is rolled back/released/cleaned according to owning semantics and ordinary cancellation destruction.

§ 35(4) Cancellation does not select a hidden branch.

§ 35(5) Cancellation after branch commit cannot retroactively undo the committed operation.

---

## § 36. `CancelRequested` is not selectable

**Governance tags:** `concurrency.select-v2`, `concurrency.cancellation-v2`

§ 36(1) The canonical observations:

```sec
Task.Current().CancelRequested
Thread.Current().CancelRequested
```

have type `bool` and are ordinary properties.

§ 36(2) They are not selectable operations.

§ 36(3) This form is invalid:

```sec
select {
    message := rx.Receive() => {
        Handle(message)
    }

    Task.Current().CancelRequested => {
        cancel
    }
}
```

§ 36(4) Explicit request observation uses ordinary control flow:

```sec
if Task.Current().CancelRequested {
    cancel
}
```

§ 36(5) Waiting select already responds to current-execution cancellation through § 35.

---

## § 37. Cancellation versus branch commit

**Governance tags:** `concurrency.select-v2`, `concurrency.cancellation-v2`

§ 37(1) Current-execution cancellation and branch commit participate in a first-commit race.

§ 37(2) If a branch commits first, its ordinary operation semantics apply.

§ 37(3) If cancellation commits first, no branch operation commits.

§ 37(4) The runtime must not both commit an operation and execute pre-commit select-cancellation rollback for the same selection attempt.

§ 37(5) The runtime must not lose conditionally owned values in this race.

---

## § 38. Branch bindings

**Governance tags:** `concurrency.select-v2`, `frontend.select-v2`

§ 38(1) A result binding has the operation's exact normal result type.

§ 38(2) Bindings follow ordinary lexical scope, mutability, ownership, destructuring, pattern, and shadowing rules.

§ 38(3) A branch-local binding does not escape automatically.

§ 38(4) A move-only selected result is owned by that branch body unless moved onward.

---

## § 39. Ownership merge after select

**Governance tags:** `concurrency.select-v2`, `analysis.transferability`

§ 39(1) Sema must merge ownership availability across all reachable selected branches.

§ 39(2) If one reachable branch consumes a value and another preserves it, later unconditional use is invalid unless ordinary control-flow analysis proves availability.

§ 39(3) Diagnostics should identify the branch that may consume the value.

---

## § 40. Definite initialization

**Governance tags:** `concurrency.select-v2`, `frontend.select-v2`

§ 40(1) A variable is definitely initialized after select only when every normal continuing branch assigns it.

§ 40(2) Cancellation/non-local control flow follows ordinary definite-initialization reachability rules.

---

## § 41. Same exclusive resource in several branches

**Governance tags:** `concurrency.select-v2`, `analysis.borrowing`

§ 41(1) The same move-only/exclusive operation capability must not be used as two independent candidates in one select unless its owning rule explicitly supports such multiplexing.

§ 41(2) One unique `Receiver[T]` does not represent two independent receive candidates.

§ 41(3) Equivalent conflicts apply to one owning task await handle, one one-shot join capability, and incompatible move-only message preparation.

---

## § 42. Evaluation of branch operands

**Governance tags:** `concurrency.select-v2`, `frontend.select-v2`

§ 42(1) Expressions needed to prepare branch operations are evaluated exactly once, in source order, when entering select, subject to compiler-defined deferred operand forms such as `await` and `join` syntax.

§ 42(2) They are not repeatedly re-evaluated while waiting.

§ 42(3) Preparation expressions obey ordinary side-effect ordering.

§ 42(4) Arbitrary user side effects must not be executed repeatedly as readiness probes.

---

## § 43. Closed and terminal outcomes are ready

**Governance tags:** `concurrency.select-v2`

§ 43(1) An operation is ready when its ordinary semantics can complete immediately with a terminal, closed, or error result.

§ 43(2) Examples include closed/drained channel receive, send to closed RX, already-terminal await, already-terminal valid join, and terminal `ProcessObserver.Wait()`.

§ 43(3) Readiness means immediate completion, not successful/desirable completion.

§ 43(4) A ready error/closed outcome is not skipped merely because a later branch might succeed.

---

## § 44. Loops and nested select

**Governance tags:** `concurrency.select-v2`

§ 44(1) Every loop iteration creates a new select preparation/readiness attempt.

§ 44(2) Non-selected reservations from a previous iteration do not survive implicitly.

§ 44(3) A selected branch may contain another select.

§ 44(4) Nested select receives no implicit readiness reservation from the outer select.

---

## § 45. Branch control flow

**Governance tags:** `concurrency.select-v2`

§ 45(1) A selected branch body may use ordinary control flow including `return`, `break`, `continue`, error propagation, `cancel`, nested select, and ordinary blocks.

§ 45(2) Branch control flow does not cause another branch from the same select to commit.

---

## § 46. Guards across select

**Governance tags:** `concurrency.select-v2`, `concurrency.mutex-v2`

§ 46(1) A live `MutexGuard[T]` must not cross a potentially waiting select.

§ 46(2) Sec 0.1 may conservatively reject select with a live guard even if all candidates happen to be immediately ready at runtime.

§ 46(3) The same general rule applies to other execution-affine guards such as `IPCMutexGuard`.

Suggested diagnostic:

```text
mutex guard state remains active across select
```

---

## § 47. Borrows across select

**Governance tags:** `concurrency.select-v2`, `analysis.borrowing`

§ 47(1) Every prepared borrow must remain valid until a branch commits and ordinary branch semantics take over, another branch commits and the prepared borrow is released, or select cancellation cleanup releases it.

§ 47(2) Conflicting simultaneous branch preparations are invalid even though at most one branch later commits.

§ 47(3) Suspension/migration must not invalidate borrowed storage.

---

## § 48. Waiting and deadlock analysis

**Governance tags:** `concurrency.select-v2`, `sema.deadlock-analysis`, `concurrency.blocking-v1`

§ 48(1) A select without `default` may wait.

§ 48(2) Its wait dependencies are the union of prepared selectable operation dependencies.

§ 48(3) `default` makes select non-waiting.

§ 48(4) Deadlock/starvation analysis must preserve source-order priority and execution-kind lifecycle dependencies.

---

## § 49. Memory synchronization

**Governance tags:** `concurrency.select-v2`, `concurrency.memory-model-v2`

§ 49(1) Select itself creates no generic synchronization edge merely by choosing a branch.

§ 49(2) The selected operation contributes its ordinary synchronization semantics.

§ 49(3) Non-selected operations create no commit synchronization edge.

§ 49(4) Readiness observation alone is not application-memory synchronization unless the owning operation explicitly says otherwise.

---

## § 50. Task and thread execution contexts

**Governance tags:** `concurrency.select-v2`, `concurrency.scheduling-v1`

§ 50(1) `select` may execute in a logical task or physical thread when every candidate operation is valid in that context.

§ 50(2) In a task, waiting select should suspend/park logically when the selected runtime can do so.

§ 50(3) In a physical thread, waiting select may park/block the physical thread.

§ 50(4) Source-level readiness, source-order priority, commit, ownership, and cancellation semantics are identical.

---

## § 51. No arbitrary selectable interface

**Governance tags:** `concurrency.select-v2`, `frontend.select-v2`

§ 51(1) Sec 0.1 defines no user-implementable `Selectable`, `Awaitable`, `Pollable`, or readiness interface for primitive select.

§ 51(2) User-defined methods cannot opt into primitive select by matching a method name/signature.

§ 51(3) Future extensibility requires explicit normative design for readiness registration, commit, rollback, ownership, cancellation, and tooling.

---

## § 52. Target and CompilationPlan requirements

**Governance tags:** `concurrency.select-v2`, `compiler.platform-model`

§ 52(1) The selected immutable `CompilationPlan` determines whether the complete prepared operation set can be multiplexed conformingly.

§ 52(2) Relevant facts may include task suspension, thread parking/wait sets, monotonic timers, channel readiness registration, task/thread/process completion integration, IPC/pipe readiness, synchronization-object readiness, cancellation wakeup, and bounded/no-allocation runtime constraints.

§ 52(3) Compiler-host capabilities must not substitute for target facts.

§ 52(4) Unsupported statically known select constructions are compile-time errors.

---

## § 53. Allocation

**Governance tags:** `concurrency.select-v2`, `allocation.general-v2`

§ 53(1) Select execution must not hide unbounded dynamic allocation.

§ 53(2) Compiler/runtime wait-set state may use statically/fixed/provisioned storage where required by the selected profile.

§ 53(3) Sec 0.1 select syntax has no public `SelectError` result.

§ 53(4) A no-allocation target may accept only select forms whose runtime registration/storage can be satisfied conformingly.

---

## § 54. Semantic analysis

**Governance tags:** `frontend.select-v2`, `analysis.transferability`

§ 54(1) Sema must determine for every branch the selectable operation identity, exact return type, resource identities, one-time preparation effects, candidate ownership, readiness kind, commit effects, cancellation rollback behavior, synchronization effect, and target requirements.

§ 54(2) Sema must reject ordinary calls/properties that are not canonical selectable operations.

§ 54(3) Sema must reject `TrySend`, `TryReceive`, `TryLock`, `TryAcquire`, and `CancelRequested` as primitive selectable operations.

§ 54(4) Sema must accept task/thread/process/Command `join` according to D3 and the owning lifecycle rules.

§ 54(5) Sema must preserve exact branch result types rather than inventing a generic select wrapper.

---

## § 55. Semantic IR

**Governance tags:** `semantic-ir.select-v2`

§ 55(1) Semantic IR must preserve select explicitly enough that lowering never reconstructs semantics from arbitrary calls.

§ 55(2) Required facts include source-ordered branch list, operation identity/kind, exact operand/result types, prepared values and conditional ownership, resource identity, readiness registration/state, selected branch identity, atomic commit state, rollback/non-selected release path, `after` deadline, `default` presence, cancellation registration, operation-specific synchronization, source provenance, and target capability requirements.

§ 55(3) IR verification must ensure at most one branch commits.

§ 55(4) IR verification must reject double ownership transfer, consumed non-selected join capability, non-selected task await consumption, non-selected message/ticket transfer, non-selected guard/permit acquisition, and contradictory selected/cancelled commit state.

§ 55(5) Concrete IR opcode names remain owned by `semantic_ir.md`.

---

## § 56. Lowering

**Governance tags:** `lowering.select-v2`, `compiler.platform-model`

§ 56(1) Lowering consumes validated select Semantic IR and the selected `CompilationPlan`.

§ 56(2) Lowering may use runtime wait sets, scheduler registrations, OS/native multiplexing, event-loop registration, generated fast-path readiness tests, or conforming combinations.

§ 56(3) Lowering must preserve source-order priority, non-destructive readiness, exactly-one commit, conditional ownership, exact operation result types, cancellation-versus-commit atomicity, operation-specific synchronization, and non-selected no-effect semantics.

§ 56(4) Backend convenience must not convert excluded `Try*` operations or boolean properties into selectable primitives.

---

## § 57. Runtime obligations

**Governance tags:** `runtime.select-v2`

§ 57(1) The runtime must be able to withdraw/release non-selected readiness registrations without committing their operations.

§ 57(2) A readiness notification may be stale by commit time; the runtime must revalidate/commit atomically according to the operation contract.

§ 57(3) Native notification order must not override source-order priority.

§ 57(4) Runtime cleanup releases all select-local registration state after selection/cancellation.

---

## § 58. LSP hover

**Governance tags:** `tooling.select-v2`

§ 58(1) Hover on a branch operation must show its exact ordinary return type.

Examples:

```text
tx.Send(...)                 -> ChannelSendResult[T]
tx.SendRevocable(...)        -> ChannelRevocableSendResult[T]
rx.Receive()                 -> Option[T]
await task                   -> TaskOutcome[T]
observer.Wait()              -> ProcessStatus
IPCMutex.Lock()              -> Result[IPCMutexGuard, IPCSyncError]
IPCSemaphore.Acquire()       -> Result[void, IPCSyncError]
```

§ 58(2) Hover must not describe `TrySend`, `TryReceive`, or `CancelRequested` as selectable.

§ 58(3) Hover/docs for task/thread/process/Command join must state that selected join consumes join capability while preserving handle semantics defined by the owner.

---

## § 59. Completion and navigation

**Governance tags:** `tooling.select-v2`

§ 59(1) Completion in select candidate position should prefer canonical selectable operations valid for current type/context.

§ 59(2) Completion must not propose `TrySend`, `TryReceive`, `TryLock`, `TryAcquire`, or `CancelRequested` as primitive waiting branches.

§ 59(3) Navigation resolves selected operations to their ordinary canonical declaration/keyword rule rather than duplicate select-only APIs.

---

## § 60. Diagnostics

**Governance tags:** `tooling.select-v2`, `frontend.select-v2`

§ 60(1) Suggested diagnostics include:

```text
operation is not selectable: Sender[Message].TrySend
```

```text
operation is not selectable: Receiver[Message].TryReceive
```

```text
CancelRequested is a bool property, not a selectable operation
```

```text
default branch must be last in select
```

```text
default makes this timeout branch unreachable
```

```text
receiver rx is used by more than one branch in the same select
```

```text
mutex guard state remains active across select
```

§ 60(2) Diagnostics should identify the operation owner and, where useful, suggest ordinary nonblocking use outside select.

---

## § 61. Canonical examples

**Governance tags:** `concurrency.select-v2`

§ 61(1) Channel receive with timeout:

```sec
select {
    message := rx.Receive() => {
        match message {
            Some(value) => {
                Handle(<-value)
            }

            None => {
                HandleClosed()
            }
        }
    }

    after 100<ms> => {
        HandleTimeout()
    }
}
```

§ 61(2) Channel send retains message ownership when timeout wins:

```sec
let message := BuildMessage()

select {
    result := tx.Send(<-message) => {
        match result {
            Sent => {
            }

            Closed(returned) => {
                Recover(<-returned)
            }
        }
    }

    after 100<ms> => {
        Use(message)
    }
}
```

§ 61(3) Unified task/thread join shape:

```sec
select {
    join task => {
        HandleTask(task.Status)
    }

    join thread => {
        HandleThread(thread.Status)
    }

    after 1<s> => {
        HandleTimeout()
    }
}
```

---

## § 62. Restrictions

**Governance tags:** `concurrency.select-v2`

§ 62(1) Sec 0.1 select must not commit more than one branch; choose ready branches randomly/fairly instead of source order; commit a non-selected operation; repeatedly evaluate user operands while waiting; treat ordinary calls as selectable automatically; treat nonblocking `Try*` operations as primitive waits; treat `CancelRequested` as selectable; consume non-selected task/join capability; move non-selected channel/IPC payloads; create non-selected tickets; acquire non-selected guard/permit; reap non-selected processes; mutate non-selected pipe read buffers; write non-selected pipe bytes; bypass ownership/borrow/lifecycle rules; expose backend notification order as branch priority; or create a general hidden user-defined selectable protocol.

---

## § 63. Explicitly absent Sec 0.1 forms

**Governance tags:** `concurrency.select-v2`

§ 63(1) Sec 0.1 does not define random/fair select, round-robin select, priority values separate from source order, selectable TrySend/TryReceive, selectable TryLock/TryAcquire, selectable CancelRequested properties, a general Selectable interface, user-defined readiness callbacks, automatic TaskObserver/ThreadObserver selection without exact public wait operations, or a select-wide generic Result/Error wrapper.

---

## § 64. Conformance scenarios

**Governance tags:** `concurrency.select-v2`

§ 64(1) Conformance tests must include at least:

- first ready branch wins by source order;
- continuously ready early branch can starve a later branch;
- exactly one branch commits;
- non-selected branch has no operation effect;
- branch operands evaluate exactly once;
- channel Receive closed/drained readiness;
- channel Send ready/blocked/Closed semantics;
- revocable ticket only on selected commit;
- TrySend and TryReceive rejected as selectable;
- MessageTicket.Revoke rejected as selectable;
- await selected consumes task and yields TaskOutcome[T];
- non-selected await preserves Task[T];
- task join selectable and consumes join capability only;
- thread join selectable and consumes join capability only;
- process and Command join selectable;
- ProcessObserver.Wait selectable;
- TaskObserver/ThreadObserver select rejected without exact operation;
- pipe Read/Write selectable and ReadExact/WriteAll rejected;
- IPC send/receive selectable;
- IPCMutex.Lock/IPCSemaphore.Acquire selectable and Try variants rejected;
- monotonic/zero `after`;
- default final/unique/reachability;
- cancellation winning pre-commit commits no branch;
- CancelRequested rejected as selectable;
- conditional move ownership and ownership merge;
- duplicate exclusive candidate rejected;
- live guard across waiting select rejected;
- native notification order cannot override source order;
- LSP excludes non-selectable Try*/CancelRequested;
- Semantic IR verifies exactly-one commit.

---

## § 65. Cross-rulebook synchronization

**Governance tags:** `concurrency.select-v2`

§ 65(1) `channels.md` states unconditionally that `TrySend` and `TryReceive` are not selectable.

§ 65(2) `await.md` remains authoritative after a selected await branch reaches await commit.

§ 65(3) `tasks.md` states that task `join` is selectable while preserving the task handle.

§ 65(4) `threads.md` defines thread join as normative Sec 0.1 selectable join.

§ 65(5) `processes.md` already owns process/Command join and `ProcessObserver.Wait()` details; this rulebook preserves them.

§ 65(6) Task/thread observer prose defines no select participation without an exact public operation, so no hidden operation is implied.

§ 65(7) `cancellation.md` defines `CancelRequested` properties as ordinary boolean observations, not selectable operations.

§ 65(8) `ipc.md` retains its existing distinction between blocking selectable primitives and nonblocking Try operations.

§ 65(9) Concrete Semantic IR vocabulary remains owned by `semantic_ir.md`.

---

## § 66. Governance

**Governance tags:** `concurrency.select-v2`, `frontend.select-v2`, `semantic-ir.select-v2`, `lowering.select-v2`, `runtime.select-v2`, `tooling.select-v2`, `analysis.transferability`, `analysis.borrowing`, `sema.deadlock-analysis`, `compiler.platform-model`

§ 66(1) `governance/concurrency_select.yaml` is the sole canonical implementation-status owner for select expressions, cases, and selection semantics.

§ 66(2) Operation-specific implementation work remains cross-linked to owning channel/task/thread/process/IPC governance fragments rather than duplicated as select-owned type/API definitions.

§ 66(3) Root `implementation-status.yaml` is not a second canonical ledger.

§ 66(4) Cross-rulebook synchronization required by this revision was tracked by the accompanying select-v2 correction.

§ 66(5) After synchronization is applied, the correction belongs under `rules/corrections/applied/`.
