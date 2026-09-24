# Correction — Select v2 cross-rulebook synchronization

- **Status:** Applied
- **Created:** 2026-09-23
- **Last updated:** 2026-09-24
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `main-reviewed-2026-09-23`
- **Primary owning rulebook:** `rules/concurrency/select.md`
- **Implementation governance:** `governance/concurrency_select.yaml`
- **Classification:** Normative synchronization of select readiness/commit and selectable-operation decisions

---

## 1. Canonical selectable-operation rule

Synchronize compiler recognition, Sema, LSP, Semantic IR, lowering, runtime wait registration, examples, and tests to the closed Sec 0.1 selectable-operation set defined by `rules/concurrency/select.md`.

An operation is not selectable merely because it blocks, waits, polls, returns quickly, or has a suggestive name.

Primitive select participation requires canonical readiness and commit semantics.

---

## 2. Remove channel `Try*` from select

The following are ordinary nonblocking operations and are **not** selectable:

```sec
Sender[T].TrySend(...)
Receiver[T].TryReceive()
```

Remove them from any select operand registry, parser/Sema special case, LSP completion list, tests, examples, and documentation claiming they can be primitive select branches.

Do not convert them into an always-ready select branch.

Do not use them as an implicit `default`.

---

## 3. Correct `channels.md`

The current channels v2 text includes a conditional clause equivalent to:

```text
a selected TrySend branch yields ChannelTrySendResult[T]
if TrySend is accepted as a selectable immediate operand
```

Remove that possibility.

Canonical rule:

```text
TrySend and TryReceive are never primitive selectable operations in Sec 0.1.
```

Keep their exact ordinary-call result types unchanged.

---

## 4. Keep blocking channel operations selectable

Keep selectable:

```sec
Sender[T].Send(...)
Sender[T].SendRevocable(...)
Receiver[T].Receive()
```

A selected operation yields its ordinary exact result type.

A non-selected channel branch moves no message, enqueues/dequeues nothing, creates no ticket, starts no committed expiration lifetime, increments no statistics, and changes no terminal message/channel state.

---

## 5. `MessageTicket.Revoke()` is not selectable

Do not add primitive readiness registration for:

```sec
MessageTicket[T].Revoke()
```

It is an ordinary non-waiting ticket operation.

Likewise, do not make `Share`, `Discard`, `Statistics`, or endpoint `Close` primitive select candidates.

---

## 6. Remove `CancelRequested` select branch

Remove old conceptual forms using:

```sec
Task.Current().CancelRequested
Thread.Current().CancelRequested
```

as select branches.

These are ordinary `bool` properties, not waitable/selectable operations.

Explicit request observation uses ordinary control flow:

```sec
if Task.Current().CancelRequested {
    cancel
}
```

A waiting select already participates in current-execution cancellation without a hidden cancellation branch.

---

## 7. Cancellation race

For a waiting select:

```text
branch commit wins first
    selected operation commits normally

current-execution cancellation wins first
    no branch commits
    prepared branch state is rolled back/released/cleaned
```

Do not surface cancellation as an ordinary branch result.

Do not commit one operation and simultaneously run pre-commit select-cancellation rollback.

---

## 8. Task `await` in select

Keep `await Task[T]` selectable.

Before branch commit, preparation does not consume the `Task[T]`.

A non-selected await leaves the task handle unchanged.

If select cancellation wins before branch commit, merely prepared await does not invoke direct-await D2 cleanup; the still-owned task lifecycle remains subject to surrounding cancellation/structured cleanup.

If the await branch commits, it consumes `Task[T]`, yields `TaskOutcome[T]`, and `await.md` owns semantics from that commit point onward.

---

## 9. Task `join` is selectable

Update `tasks.md`, frontend recognition, LSP, tests, IR, and runtime so:

```sec
join task
```

is selectable in Sec 0.1.

Readiness:

```text
task is terminal
and its outstanding join capability can commit immediately
```

Selected commit:

```text
consume one-shot join capability
establish task-completion synchronization
preserve Task[T] handle for terminal inspection
do not transfer TaskOutcome[T]
```

Non-selected:

```text
consume no join capability
change no lifecycle state
```

---

## 10. Thread `join` is selectable

Replace legacy/planned wording.

`join Thread[T]` is normative selectable Sec 0.1 behavior.

Selected thread join performs the same ordinary join semantics already owned by `threads.md`: completion synchronization, join-capability consumption, release of join-owned native resources, preserved terminal handle, and preserved unconsumed normal result.

Non-selected join does none of these.

---

## 11. Process and Command join

Preserve existing process-book decisions:

```sec
join Process[T]
join Command
```

are selectable.

Selected join performs normal process/Command lifecycle join.

Non-selected join does not reap, detach, terminate, consume join capability, or unlock/consume terminal payloads.

---

## 12. Unified join model

The source-level select rule is intentionally uniform:

```text
Task[T]
Thread[T]
Process[T]
Command
```

all support selectable `join` when a valid one-shot join capability exists.

The exact resource/lifecycle consequences remain owned by each execution-kind rulebook.

Do not invent one universal `JoinResult` type.

---

## 13. `ProcessObserver.Wait()`

Keep exact existing process observer support:

```sec
fn Wait() ProcessStatus
```

`ProcessObserver.Wait()` is selectable and repeatable.

Selecting it observes terminal `ProcessStatus` and does not resolve owner join/detach obligations or reap/unlock owner payloads.

---

## 14. Task/thread observer wording

Current task/thread books may say their observer can conceptually participate in completion observation/select without defining an exact public wait operation.

Do not synthesize a hidden observer-select method.

For Sec 0.1, TaskObserver/ThreadObserver do not become selectable until their owning books define an exact canonical public operation and synchronize it with select.

This does not affect `ProcessObserver.Wait()`.

---

## 15. IPC blocking versus Try operations

Preserve IPC's existing distinction.

Selectable:

```sec
PipeReader.Read(...)
PipeWriter.Write(...)
IPCSender[T].Send(...)
IPCReceiver[T].Receive()
IPCMutex.Lock()
IPCSemaphore.Acquire()
```

Not primitive selectable:

```sec
PipeReader.ReadExact(...)
PipeWriter.WriteAll(...)
IPCMutex.TryLock()
IPCSemaphore.TryAcquire()
IPCSemaphore.Release()
```

No new IPC API is introduced.

---

## 16. Exact result types

Selected branches bind the operation's exact ordinary result type.

```text
Sender[T].Send
    ChannelSendResult[T]

Sender[T].SendRevocable
    ChannelRevocableSendResult[T]

Receiver[T].Receive
    Option[T]

await Task[T]
    TaskOutcome[T]

ProcessObserver.Wait
    ProcessStatus

PipeReader.Read
    Result[uint, IOError]

PipeWriter.Write
    Result[uint, IOError]

IPCSender[T].Send
    Result[void, IPCError]

IPCReceiver[T].Receive
    Result[Option[T], IPCError]

IPCMutex.Lock
    Result[IPCMutexGuard, IPCSyncError]

IPCSemaphore.Acquire
    Result[void, IPCSyncError]
```

Do not introduce `SelectResult`, `SelectError`, or a generic branch wrapper.

---

## 17. Source-order priority

Keep source-order priority normative.

If multiple branches are ready at one selection point, the first ready branch wins.

Native event-notification order must not override source order.

No random, fair, or round-robin selection is introduced.

---

## 18. Preparation and conditional ownership

Implement explicit phases:

```text
prepare
readiness
commit
```

Operation arguments are prepared once, not repeatedly re-evaluated while waiting.

Move-only values staged for potentially consuming branches are conditionally owned.

If another branch wins during normal execution, ownership returns to the continuing source path.

If select is cancelled before commit, still-owned values are destroyed/resolved through ordinary cancellation cleanup.

Only selected commit performs ownership transfer.

---

## 19. Timeout syntax

Use canonical duration/unit forms:

```sec
after 100<ms> => {
}
```

rather than legacy `100ms`/`1s` examples.

`after` uses monotonic time.

Zero duration is immediately ready but remains source-order prioritized.

Unsupported timer capability is a compile-time target/profile diagnostic rather than runtime `SelectError`.

---

## 20. Default

Keep exactly one optional final `default`.

A select with default never waits.

Diagnose combinations where a positive timeout before final default is unreachable because default necessarily wins whenever no earlier operation is ready.

Apply ordinary source-order reachability to zero-duration after/default combinations.

---

## 21. Guard and borrow analysis

Keep live `MutexGuard[T]` and `IPCMutexGuard` restrictions across potentially waiting select.

Branch preparation borrows must remain simultaneously valid.

Reject conflicting preparations even though only one branch later commits.

Model selected-await task lifecycle ownership only at commit, not during mere readiness preparation.

---

## 22. Semantic IR

Represent select explicitly with at least:

```text
source-ordered candidates
canonical operation identity/kind
exact operand/result types
one-time prepared values
conditional ownership
resource identity
readiness registration/state
selected branch identity
atomic commit state
non-selected rollback/release
after deadline
default presence
current-execution cancellation state
operation synchronization
source provenance
target requirements
```

IR verification must reject more than one committed branch, ownership transfer by a non-selected branch, consumed non-selected join capability, non-selected task await consumption, non-selected ticket creation, non-selected guard/permit acquisition, or simultaneous pre-commit cancellation and selected commit.

Concrete opcode names remain owned by `semantic_ir.md`.

---

## 23. LSP/tooling

Update completion so selectable candidate suggestions include only canonical valid operations for the current type/context.

Do not suggest these as primitive select candidates:

```text
TrySend
TryReceive
TryLock
TryAcquire
CancelRequested
```

Hover on branch bindings shows exact operation result types.

Navigation resolves to the owning ordinary operation/type declaration rather than duplicate select-only declarations.

---

## 24. Governance

Populate:

```text
governance/concurrency_select.yaml
```

with `concurrency.select-v2`.

Cross-link related channel/task/thread/process/IPC work rather than duplicating those integrations.

Do not create `implementation-status-select.yaml`.

Do not append a duplicate integration to root `implementation-status.yaml`.

---

## 25. `language-rulebook-status.md`

After synchronization, mark `concurrency/select.md` as synchronized revision 2.0.

Suggested summary:

```text
Revision 2.0 defines source-order prepare/readiness/commit semantics,
exact selectable operation categories, unified Task/Thread/Process/Command
join selection, and excludes nonblocking Try* and CancelRequested branches.
```

---

## 26. Required tests

Add/update tests for source-order priority, starvation visibility, exactly-one commit, non-selected no-effect, one-time preparation, conditional ownership, TrySend/TryReceive rejection, CancelRequested rejection, channel send/receive/revocable exact result types, await selected/non-selected ownership, selectable task/thread/process/Command join, ProcessObserver.Wait select, no hidden task/thread observer wait, IPC primitive select set, IPC Try rejection, monotonic/zero timeout, default reachability, cancellation-versus-commit race, guard/borrow conflicts, LSP completion/hover, and IR exactly-one-commit verification.
