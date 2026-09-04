# Await

## Purpose

`await` waits for a task to finish and resolves its outcome.

It is the normal point where task execution ends from the owner's perspective
and the task result returns to ordinary synchronous control flow.

`await` consumes the owning `Task[T]` handle.

`await` is task-specific in v0.1.

Raw `Thread[T]` and process handles are not awaitable unless a future adapter or
awaitable rule explicitly adds that support.

---

## Basic syntax

```sec
let value := await task
```

If `task` has type:

```sec
Task[T]
```

then a successful completion produces a value of type:

```sec
TaskOutcome[T]
```

Example:

```sec
fn Calculate() int {
    return 42
}

let calculation := try spawn Calculate()
let outcome := await calculation
```

After `await`, `outcome` has type `TaskOutcome[int]`. Normal completion is
represented by `Completed(42)`; the same expression retains this static type
even when analysis proves normal completion.

The binding `calculation` has been consumed and may not be used again.

---

## Task[void]

Awaiting a `Task[void]` waits for completion and produces
`TaskOutcome[void]`.

```sec
let worker := try spawn Work()
let outcome := await worker
```

This is valid when `Work()` returns `void`; there is no special direct-`void`
await rule.

---

## Result-returning tasks

`await` preserves the complete task result type.

```sec
fn Load() Result[Image, IOError] {
}

let loading := try spawn Load()
let outcome := await loading
```

`outcome` has type:

```sec
TaskOutcome[Result[Image, IOError]]
```

`Completed(Err(IOError.InvalidValue))` is normal task completion. It is neither
`Failed(TaskError)` nor cancellation. `await` does not automatically unwrap
either `TaskOutcome` or the nested `Result`.

Typed program errors remain ordinary values.

---

## Consumption

`await` consumes the owning task handle.

```sec
let task := try spawn Calculate()
let outcome := await task
```

Afterward:

```sec
task.running // invalid
```

The task execution state and task-owned scheduling resources may be destroyed
after the result has been transferred.

The returned value continues with the normal ownership rules of its type.

---

## Result transfer

The value produced by `await` follows the copy and move rules of `T`.

For a move-only result:

```sec
Task[File]
```

ownership of the `File` moves from the completed task to the awaiting code.

For a trivially copyable result:

```sec
Task[int32]
```

the implementation may copy the representation.

The source-level task handle is still consumed.

Invalid:

```sec
let worker := try spawn thread Work()
await worker
```

Expected diagnostic:

```text
await requires Task[T]; got Thread[T]
```

Invalid:

```sec
let process := try spawn process Program()
await process
```

Expected diagnostic:

```text
await requires Task[T]; got Process
```

Thread and process completion use `join` unless a future rule defines an
explicit awaitable adapter.

---

## Direct await

A newly spawned task may be awaited directly.

```sec
let outcome := await (try spawn Calculate())
```

This is equivalent in semantics to:

```sec
let task := try spawn Calculate()
let outcome := await task
```

The compiler may optimize the physical scheduling when observable task semantics
remain unchanged.

---

## Suspension and blocking

`await` waits without prescribing one physical implementation.

The selected task backend may:

- suspend the current lightweight task
- block an operating-system thread
- yield to a cooperative scheduler
- wait through an RTOS primitive
- resume through an event loop

A backend must preserve the same language semantics.

`await` must not busy-wait unless the selected target profile explicitly
requires and documents that behavior.

---

## Join

`join` waits for completion while preserving the task handle.

```sec
join task
```

After a successful join, the task remains available for status inspection and
value extraction.

Example:

```sec
let worker := try spawn Calculate()

join worker

if worker.done {
    let value := worker.value
}
```

`join` and `await` are distinct:

```text
join task
    waits for completion
    preserves Task[T]

await task
    waits for completion
    consumes Task[T]
    resolves the task outcome
```

---

## Task value

`task.value` is unavailable before completion has been established.

Valid:

```sec
join task
let value := task.value
```

Invalid:

```sec
let task := try spawn Calculate()
let value := task.value
```

Expected diagnostic:

```text
task value is unavailable before join or completion
```

Accessing `task.value` transfers or copies `T` according to the normal rules of
`T`.

For move-only `T`, taking `task.value` consumes the stored result.

The task handle may remain available for final status inspection but no longer
contains an available result.

Repeated extraction of a move-only result is invalid.

---

## Completed status

A task returning normally becomes `Completed`.

This includes functions returning:

```sec
Err(error)
```

when their declared return type is:

```sec
Result[T, E]
```

A returned `Err(E)` is not a task execution failure.

---

## Cancellation

Cancellation is distinct from normal completion.

A task may become cancelled because:

- another owner requests cancellation
- its parent propagates cancellation
- the program task manager requests shutdown cancellation
- the task executes the `cancel` statement

A cancelled `Task[T]` has no normal value of type `T`.

Therefore awaiting a task always produces the compiler-known outcome type:

```sec
TaskOutcome[T]
```

Conceptually:

```sec
type TaskOutcome[T] union {
    Completed(T)
    Cancelled
    Panicked(PanicInfo)
    Failed(TaskError)
}
```

For `T == void`:

```sec
TaskOutcome[void]
```

uses `Completed` without a payload.

---

## Await outcome

The canonical semantic model treats:

```sec
await task
```

as resolving the complete task outcome with the static type `TaskOutcome[T]`.
Proof of normal completion may optimize representation or unreachable branches,
but it does not change that type to `T`. The result preserves:

- completed value
- cancellation
- panic with `PanicInfo`
- task execution failure

Example form:

```sec
match await task {
    Completed(value) => Use(value)
    Cancelled => HandleCancellation()
    Panicked(info) => HandlePanic(info)
    Failed(error) => HandleTaskFailure(error)
}
```

`await` must never invent a default `T` for cancelled or failed tasks.

---

## Task[void] outcome

For a void task:

```sec
match await task {
    Completed => Continue()
    Cancelled => HandleCancellation()
    Panicked(info) => HandlePanic(info)
    Failed(error) => HandleTaskFailure(error)
}
```

Discarding or otherwise handling this outcome follows the ordinary union,
must-use, and discard rules; await does not silently erase abnormal outcomes.

---

## Cancellation requests

The task owner may request cancellation before awaiting:

```sec
task.cancel()
let outcome := await task
```

`cancel()` requests cancellation.

It does not guarantee that the task has already stopped when the call returns.

`await` remains the synchronization point that observes the final outcome.

---

## Cancellation points

`await` is a cancellation point for the currently executing task.

If cancellation has been requested for the current task while it is waiting, the
scheduler may resume it through cancellation control flow.

The language must preserve deterministic cleanup:

- defer blocks execute
- owned values are destroyed
- active borrows end correctly
- mutex guards must already have been released

---

## Mutex guards

A live `MutexGuard[T]` may not cross `await`.

Invalid:

```sec
let mut state := State.lock()
state.running = true

let value := await worker
```

Expected diagnostic:

```text
mutex guard state remains active across await
```

The diagnostic should identify:

- the lock acquisition
- the live guard
- the await expression

The programmer must end the guard scope first:

```sec
{
    let mut state := State.lock()
    state.running = true
}

let value := await worker
```

---

## Borrows across await

Ordinary borrows may cross `await` only when the compiler proves that:

- the owner remains alive
- no conflicting access occurs
- the referenced location remains stable
- the task may safely resume with the reference
- the borrow does not cross an invalid task boundary

A borrow retained by another running task remains active until that task
completes.

`await` may establish the point where such a borrow ends.

---

## Multiple observers

Multiple observers may wait for task completion.

They do not receive ownership of the task result.

Only the owning task handle may:

- consume the task through `await`
- take `task.value`
- detach the task
- transfer ownership

Observers may inspect:

- running
- done
- cancelled
- failed
- cancelRequested

Observer completion waits must not transfer `T`.

---

## Repeated await

An owning task may be awaited only once.

Invalid:

```sec
let task := try spawn Calculate()

let first := await task
let second := await task
```

Expected diagnostic:

```text
use of consumed task task
```

A preserved task after `join` is not considered awaited until the owner consumes
or resolves it.

---

## Awaiting returned tasks

A function may return a task to its caller.

```sec
fn StartWorker() Result[Task[int], TaskSpawnError] {
    return spawn Calculate()
}

let worker := try StartWorker()
let outcome := await worker
```

Ownership moves through the return value.

The original function is not responsible for awaiting the returned task after
the move.

---

## Await in expressions

`await` is an expression whose task operand produces `TaskOutcome[T]`.

Examples:

```sec
let outcome := await task
return await task
Use(await task)
```

Evaluation order must remain explicit and deterministic.

The compiler must not reorder other side effects across `await` when doing so
changes observable behavior.

---

## Await in defer

`await` inside `defer` is invalid in version 0.1.

A deferred block executes during cleanup.

Suspending cleanup while function destruction is in progress complicates:

- ownership finalization
- cancellation
- nested cleanup
- mutex release
- task shutdown

Expected diagnostic:

```text
await is not allowed inside defer
```

---

## Await in static initialization

`await` is not permitted in compile-time or hidden static initialization.

Sec does not introduce implicit asynchronous initialization.

Runtime task startup and waiting must be visible in ordinary executable code.

---

## Main function

The main program may await tasks normally.

A task left unresolved when `main` completes is invalid unless it has been
explicitly detached.

Detached tasks are handled by the program task manager according to the shutdown
rules.

---

## Memory synchronization

Successful completion synchronization establishes the memory visibility required
for the awaiting task to observe the completed task's published result.

The precise happens-before and ordering rules are defined in:

```text
concurrency_memory_model.txt
```

Reading `task.done` alone does not replace join or await synchronization.

---

## Semantic validation

The compiler must validate at least:

- the operand is `Task[T]`
- the task handle is owned and available
- the task has not already been consumed
- the task result is used in a compatible context
- cancellation and task failure remain represented
- live mutex guards do not cross await
- retained borrows remain valid
- await does not occur inside defer
- target profile supports task waiting
- task ownership is resolved after await

---

## Semantic IR

Semantic IR must represent await and join explicitly.

At minimum:

```text
TaskJoin
TaskAwait
TaskOutcome
TaskTakeValue
TaskConsume
TaskCancelResume
```

The IR must record:

- concrete `Task[T]` type
- owner being consumed
- result ownership transfer
- cancellation path
- panic path carrying `PanicInfo`
- task failure path
- cleanup before suspension
- borrows live across suspension
- continuation after completion
- source location

The backend must not infer task completion semantics from ordinary function
calls or low-level synchronization instructions.

---

## Diagnostics

Examples:

```text
await requires Task value
```

```text
use of consumed task worker
```

```text
task value is unavailable before join or completion
```

```text
task result has already been taken
```

```text
mutex guard state remains active across await
```

```text
await is not allowed inside defer
```

```text
cancelled task cannot produce int
```

```text
task failure must be handled before using the completed value
```

---

## Restrictions

`await` must not:

- create a task
- detach a task
- silently ignore cancellation
- silently ignore task execution failure
- invent a default result
- copy a move-only task handle
- allow repeated result extraction
- hold a mutex guard across suspension
- bypass ownership or borrowing rules
- be lowered as a simple function call without task semantics

---

## Related rules

Detailed behavior is defined in:

```text
tasks.md
spawn.txt
concurrency.txt
mutex.txt
concurrency_memory_model.txt
```
