# Correction — Await v2 cross-rulebook synchronization

- **Status:** Applied
- **Created:** 2026-09-16
- **Last updated:** 2026-09-21
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `main-reviewed-2026-09-16`
- **Primary owning rulebook:** `rules/concurrency/await.md`
- **Implementation governance:** `governance/concurrency_await.yaml`
- **Classification:** Normative synchronization of decided await/task-cancellation semantics

---

## 1. Canonical await rule

Synchronize all rulebooks, compiler-known declarations, Sema, LSP, tests, Semantic IR, lowering, and runtime code to:

```sec
await task
```

where `task` owns:

```sec
Task[T]
```

and the await expression always has:

```sec
TaskOutcome[T]
```

Await consumes the owning `Task[T]` handle.

Do not narrow the source type to `T` or `Result[T, TaskError]`.

---

## 2. Freeze `TaskError`

Update `rules/concurrency/tasks.md`, the canonical core task declaration, task governance, frontend identities, LSP, and tests so `TaskError` is exactly:

```sec
enum TaskError error {
    // The already-created task execution mechanism could not obtain
    // memory required to continue or resume execution.
    OutOfMemory

    // A runtime/executor resource required after successful task
    // creation was exhausted.
    ResourceLimit

    // The executor/runtime responsible for an already-created task
    // became unavailable before the task reached another terminal outcome.
    ExecutorUnavailable

    // A target/runtime execution failure occurred after creation and
    // is not represented by another TaskError variant.
    NativeFailure
}
```

Remove wording that says the variant inventory is still open.

Remove governance work items saying a core `TaskError` declaration is blocked on an open inventory.

Do not add `InvalidConfiguration`, `ExecutionError`, or `RuntimeError` to Sec 0.1 `TaskError`.

`InvalidConfiguration` remains a task-creation category under `TaskSpawnError`.

---

## 3. Canonical `TaskOutcome[T]`

Keep the exact type:

```sec
type TaskOutcome[T] union {
    // The task function returned normally with its declared result T.
    Completed(T)

    // Cooperative task cancellation committed terminally.
    Cancelled

    // A panic escaped the task boundary under the selected recoverable
    // task-boundary panic policy.
    Panicked(PanicInfo)

    // The already-created task failed at the execution/runtime layer.
    Failed(TaskError)
}
```

Comments documenting variants must appear before the declarations they document.

Apply the same documentation ordering rule to relevant examples/declarations in `tasks.md`, `cancellation.md`, `await.md`, canonical core task declarations, and generated API documentation.

Do not leave post-variant comments such as:

```sec
Completed(T)
// description
```

---

## 4. `PanicInfo`

Do not redefine `PanicInfo`.

Use the exact canonical type owned by `rules/errors/panic.md`.

Ensure await/task hover and navigation resolve to that single source-visible/compiler-known identity.

---

## 5. Replace legacy cancellation spelling

Replace legacy await examples such as:

```sec
task.cancel()
```

with:

```sec
task.RequestCancel()
```

`RequestCancel()` is idempotent, does not consume `Task[T]`, does not imply immediate termination, and does not predetermine the terminal `TaskOutcome[T]`.

Do not reintroduce lowercase `cancel()` as a compatibility spelling through this correction.

---

## 6. Await lifecycle ownership

After entering:

```sec
await worker
```

the source `worker` binding is consumed.

The await operation internally owns the task lifecycle until one of two resolutions:

```text
await commit
caller-cancellation cleanup
```

Do not keep the source binding conditionally available.

Do not restore it after caller cancellation.

---

## 7. Await commit point

Define await commit as:

```text
the awaited task is terminal,
the await operation has acquired TaskOutcome[T],
and ownership of that outcome is ready to transfer
to the waiting source continuation
```

A scheduler wakeup, status observation, or the child merely entering a terminal runtime state before the await acquires the outcome is not by itself source-level await commit.

Completion and caller cancellation race against this semantic commit boundary.

---

## 8. Completion wins

If await commit wins before current-execution cancellation commits:

- `TaskOutcome[T]` transfers to source code;
- the owning task handle remains consumed;
- later caller cancellation cannot undo the await;
- later terminal caller cancellation cleans up the already-owned outcome normally.

---

## 9. Caller cancellation wins before await commit

If current-execution cancellation commits before await commit:

1. The await expression produces no source-visible `TaskOutcome[T]`.
2. The await operation retains internal ownership of the consumed task handle/lifecycle.
3. The runtime requests cooperative cancellation of the awaited task.
4. Await cleanup waits/drives the awaited task until one terminal outcome exists.
5. The terminal `TaskOutcome[T]` is acquired internally.
6. The complete outcome/payload is destroyed/discarded exactly once.
7. Only then may terminal cancellation of the waiting execution complete.

This is not implicit detach.

This is not handle restoration.

This is structured cleanup of an already-consumed lifecycle capability.

---

## 10. Child cancellation may delay caller cancellation

Because cancellation is cooperative, the child may not stop immediately.

The waiting execution's terminal cancellation may therefore be delayed until the child terminates.

Do not add a runtime shortcut that silently detaches the child.

Deadlock/starvation/structured-concurrency analysis must account for the possibility that the lifecycle dependency persists after caller cancellation has won the await race.

---

## 11. Synchronize `cancellation.md`

Update the generic await/join wording in `rules/concurrency/cancellation.md`.

The current generic rule says that when caller cancellation wins before await/join commit, lifecycle/result state must remain valid according to the owning rule.

For **await**, add the concrete cross-reference:

```text
await.md owns the pre-commit caller-cancellation cleanup:
retain lifecycle ownership, RequestCancel the awaited task,
wait for terminal state, destroy the terminal outcome,
then complete caller cancellation.
```

Do not automatically apply the consuming-await cleanup to `join`; join retains its own owning-handle semantics.

Also update `TaskOutcome[T]` example comments to appear before each variant.

---

## 12. Synchronize `tasks.md`

Update `tasks.md` to:

- freeze the exact `TaskError` inventory;
- remove open-inventory clauses;
- update task governance accordingly;
- document exact await commit/caller-cancellation behavior by cross-reference to `await.md`;
- preserve `await Task[T] -> TaskOutcome[T]`;
- preserve `Completed(Err(E))`;
- preserve task-handle consumption by await;
- preserve `RequestCancel()` naming;
- place declaration comments before variants/members they document.

`tasks.md` remains the type/lifecycle owner; `await.md` owns the exact consuming wait/cancellation-commit operation.

---

## 13. Synchronize `blocking.md`

Keep `await Task[T]` as potentially waiting.

Add/clarify that pre-commit caller cancellation does not necessarily end the underlying lifecycle wait immediately.

After caller cancellation wins, await cleanup may continue waiting for the child to become terminal.

A backend must therefore not assume:

```text
caller cancellation => await dependency instantly disappears
```

Physical worker blocking/suspension remains target/profile controlled.

---

## 14. Synchronize structured concurrency

Ensure structured-concurrency text is compatible with:

```text
consuming await + caller cancellation
    does not detach child
    retains lifecycle obligation
    requests child cancellation
    waits through child cleanup
```

Borrowed child state must remain valid until child terminal cleanup completes.

---

## 15. Result ownership

Successful await:

```text
Completed(T)
    transfers T

Cancelled
    transfers no T

Panicked(PanicInfo)
    transfers/copies PanicInfo normally

Failed(TaskError)
    transfers TaskError
```

Caller-cancelled pre-commit await:

```text
no source-visible TaskOutcome[T]
terminal child outcome is owned and destroyed internally
```

Never destroy a move-only `Completed(T)` payload both in the runtime and in source code.

---

## 16. Borrowing

Update await/borrowing analysis so caller cancellation does not shorten child-held borrow lifetimes unsafely.

If the awaited task holds a valid borrow into waiter-owned state, that state must remain alive until the child reaches terminal state and releases it.

The pre-commit cancellation cleanup is part of that lifetime.

Reject code where this cannot be proven.

---

## 17. Mutex guards

Keep the existing prohibition on a live `MutexGuard[T]` crossing an invalid await suspension.

Diagnostics should identify guard acquisition, live guard, await expression, and a suggested scope/release point.

No mutex API change is introduced here.

---

## 18. Semantic IR

Update Semantic IR requirements to preserve, at minimum:

```text
concrete Task[T]
concrete TaskOutcome[T]
consumed lifecycle ownership
await commit boundary
terminal outcome category
payload ownership
caller cancellation registration/state
completion-vs-cancellation first-commit race
child RequestCancel on pre-commit caller cancellation
internal lifecycle retention
internal terminal-outcome destruction
suspension/resumption continuation
live borrows
completion synchronization
source provenance
target/profile requirements
```

Do not make the legacy literal opcode list from the old `await.md` normative.

Concrete IR naming belongs only to `rules/compiler/semantic_ir.md`.

---

## 19. Lowering and runtime

Lowering/runtime must not:

- translate await into a plain function call;
- return bare `T`;
- flatten nested `Result`;
- lose the consumed lifecycle handle during cancellation races;
- produce both source outcome and pre-commit caller-cancel path;
- implicitly detach the child;
- restore the source task handle;
- terminate caller cancellation before required child lifecycle cleanup completes.

Already-terminal task fast paths are permitted if semantics remain identical.

---

## 20. LSP and tooling

Hover on:

```sec
await worker
```

must always report:

```sec
TaskOutcome[T]
```

with concrete `T`.

Hover/navigation for `TaskError` must show exactly:

```text
OutOfMemory
ResourceLimit
ExecutorUnavailable
NativeFailure
```

Hover/navigation for `PanicInfo` must resolve to the canonical panic declaration.

Code actions/examples must use `RequestCancel()`, not legacy `cancel()`.

---

## 21. `language-rulebook-status.md`

After the correction is applied, mark `concurrency/await.md` as synchronized/written revision 2.0.

Suggested summary:

```text
Revision 2.0 defines invariant TaskOutcome typing, exact TaskError,
consuming lifecycle ownership, await commit semantics, and structured
pre-commit caller-cancellation cleanup without implicit detach.
```

---

## 22. Governance

Populate:

```text
governance/concurrency_await.yaml
```

with the `concurrency.await-v2` integration.

Update:

```text
governance/concurrency_task.yaml
```

only for task-owned changes, especially the now-frozen `TaskError` inventory.

Do not duplicate `concurrency.await-v2` into `concurrency_task.yaml`.

Do not create `implementation-status-await.yaml`.

Do not append a duplicate integration to root `implementation-status.yaml`.

After synchronization is complete, move this correction to:

```text
rules/corrections/applied/
```

and replace `corrections_pending` with the applied-correction reference.

---

## 23. Required tests

Add/synchronize tests for:

- invariant `await Task[T] -> TaskOutcome[T]`;
- nested Result preservation;
- exact `TaskError` variants;
- handle consumption;
- repeated await;
- observer/thread/process rejection;
- await commit versus caller-cancellation race;
- child `RequestCancel()` on pre-commit caller cancellation;
- lifecycle retention until terminal child state;
- exact internal terminal-outcome destruction;
- no implicit detach;
- no handle restoration;
- child ignoring cancellation delays waiter terminal cancellation;
- borrow lifetime preservation through cleanup;
- mutex guard prohibition;
- defer/CTE prohibition;
- completion memory synchronization;
- LSP hover/navigation;
- Semantic IR ownership verification.
