# Cancellation

## Current implementation status

Implemented:

- `cancel` is a lexer keyword;
- parser support for `cancel` as a statement;
- AST representation as `CancelStatement`;
- a semantic current-task cancellation context for spawned lambda operands and
  legacy spawn blocks;
- semantic diagnostic for `cancel` outside a task or explicit thread context;
- `cancel` is treated as terminating for reachability analysis;
- `cancel` inside `defer` is rejected separately.

Not implemented yet:

- general modelling of valid current task or explicit thread cancellation
  context for named functions and ordinary calls;
- cancellation request methods on `Task[T]` and `Thread[T]`;
- cancellation observation through `task.cancelRequested` or
  `Thread.Current().cancelRequested`;
- cancellation points and commit semantics;
- cancellation outcome typing for await/join;
- Semantic IR cancellation operations;
- runtime wakeup and cleanup integration.

## Purpose

Cancellation is a cooperative request for an execution entity to stop before
normal completion.

Cancellation is distinct from:

- a normal function return;
- a returned `Err(E)`;
- panic;
- unsafe hard termination;
- process kill;
- hardware reset;
- timeout.

Portable cancellation preserves ordinary cleanup.

---

## Supported execution entities

The cancellation model applies to:

```text
Task[T]
Thread[T]
```

Processes use process-specific termination and control rules.

A process may later support cooperative cancellation through IPC, but it is not
the same as task or thread cancellation.

---

## Cooperative principle

Requesting cancellation does not immediately destroy execution.

The target execution entity must:

- observe the request;
- reach a cancellation point;
- or execute explicit cancellation-aware control flow.

Safe Sec does not asynchronously tear down an ordinary task or thread.

---

## Requesting cancellation

An owning task or thread handle may request cancellation:

```sec
worker.RequestCancel()
```

The operation is idempotent.

Multiple requests do not create multiple cancellations.

A request after terminal completion has no effect.

A request does not consume the handle.

---

## Current task observation

Inside a task:

```sec
if task.cancelRequested {
    cancel
}
```

`task.cancelRequested` is an immutable observation of the current task's
cancellation state.

It is not synchronization for unrelated data.

---

## Current thread observation

Inside an explicit physical thread:

```sec
if Thread.Current().cancelRequested {
    cancel
}
```

`Thread.Current()` returns a non-owning `ThreadContext`.

A task running on a physical executor thread observes its task cancellation
through `task`, not through the physical worker's thread cancellation state.

Task and physical thread cancellation identities remain distinct.

---

## cancel statement

```sec
cancel
```

terminates the current cancellable execution entity as `Cancelled`.

In a task context it cancels the current task.

In an explicit thread callable without a task context it cancels the current
thread.

Using `cancel` outside a cancellable task or thread context is invalid.

`cancel`:

- executes active `defer`;
- runs deterministic destruction;
- releases ordinary owned resources;
- does not create a normal result `T`;
- records terminal cancellation status;
- does not count as panic.

---

## Cancellation points

A cancellation point may include:

- `await`;
- task-context `join`;
- cancellation-aware thread join;
- blocking channel send or receive;
- a waiting `select`;
- mutex acquisition;
- timer or I/O wait;
- explicit cancellation check;
- `Task.Yield()` when the profile defines it as cancellation-aware;
- target-declared interruptible waits.

A cancellation point must preserve the operation's commit semantics.

Cancellation must not partially commit a channel message, lock acquisition or
selected branch.

---

## Cancellation during join

If the caller is a task and is cancelled while joining another execution entity,
the caller's join wait may be interrupted.

The joined handle remains owned and unresolved unless the join already committed.

The compiler and runtime must preserve exactly one of:

```text
join committed
caller cancellation won
```

No state may be lost between them.

A physical thread join may be cancellation-aware only when the target backend can
wake the thread safely.

---

## Cancellation during select

`select` is a cancellation point.

Cancellation competes with branch readiness.

Exactly one outcome commits.

If cancellation wins:

- no branch consumes ownership;
- no message is transferred;
- no task result is taken;
- all registrations are removed;
- the current execution entity performs cancellation cleanup.

---

## Cancellation during mutex wait

Cancellation before lock commit leaves the mutex unlocked by the caller.

Cancellation after lock commit means the caller owns the guard and must run
ordinary guard cleanup before becoming cancelled.

The backend must use an atomic commit boundary.

---

## Cancellation propagation

Task parent-child cancellation follows the task and structured concurrency
rules.

A child task created while parent cancellation is already requested must not
silently clear that state.

The initial task model may create the child with cancellation already requested.

Physical threads do not automatically inherit task cancellation merely because
they were spawned by a task.

Thread linkage must be explicit through:

- a future structured concurrency scope;
- a shared cancellation source;
- configuration defined by the thread rules.

---

## Cancellation source and observers

The compiler may implement cancellation with an internal source/token split.

The owning handle may request cancellation.

The executing context and non-owning observers may read cancellation state.

Reading state does not grant lifecycle ownership.

The source representation must not require a managed runtime.

---

## Terminal result

A cancelled `Task[T]` or `Thread[T]` has no normal value of type `T`.

Its terminal status is `Cancelled`.

A callable returning:

```sec
Err(error)
```

from `Result[T, E]` completes normally and is not cancelled.

Cancellation and business errors must remain distinguishable.

---

## Await and outcome

Awaiting a cancellable task must preserve cancellation in its outcome model.

Conceptually:

```sec
match await worker {
    Completed(value) => Use(value)
    Cancelled => HandleCancellation()
    Panicked(info) => HandlePanic(info)
    Failed(error) => HandleExecutionFailure(error)
}
```

The exact outcome type is task-specific.

Thread completion is inspected after `join` through `ThreadStatus` and terminal
payload members.

---

## Panic

Panic is not cancellation.

A panicked execution entity has terminal status `Panicked`.

Cancellation cleanup follows ordinary deterministic rules.

Panic cleanup follows the panic rulebook and may differ only where that rule
explicitly says so.

A cancellation request must not rewrite an already panicked outcome.

---

## Unsafe hard termination

Safe Sec has no general forced task or thread kill.

A platform-specific thread termination operation may exist behind `unsafe`:

```sec
unsafe {
    try worker.platform.Terminate()
}
```

Hard termination may skip:

- `defer`;
- destructors;
- mutex release;
- thread-local destruction;
- cancellation handlers;
- result construction.

The status becomes `Terminated`, not `Cancelled`.

A thread stuck because of hardware failure may be unrecoverable in-process.

Recommended isolation mechanisms include:

- process boundary;
- watchdog;
- subsystem reset;
- device reset;
- whole-system reset.

---

## Detached execution

Detached tasks and threads remain cancellation-managed by the selected program
runtime or lifecycle manager.

At normal program shutdown, the manager must:

- request cancellation;
- wake cancellation-aware waits;
- allow cleanup;
- wait for required termination according to profile policy.

A target profile must define what happens when detached work ignores
cancellation indefinitely.

The safe language must not silently force-kill it.

---

## Cancellation and blocking FFI

A foreign blocking call may be non-cancellable.

Its extern contract should declare this.

The compiler should diagnose detached or shutdown-critical paths that can enter
a non-cancellable foreign wait.

A platform adapter may provide an interruptible wrapper.

---

## ISR

An ISR cannot be cancelled through the ordinary task/thread model.

It may request cancellation of another task or thread only through an explicitly
ISR-safe operation.

Ordinary `RequestCancel()` is not ISR-safe unless the target contract says so.

An ISR must return according to interrupt rules.

---

## Memory ordering

A cancellation request must be safely observable by the target execution entity.

The request operation and observation use compiler-defined atomic
synchronization.

Cancellation state publication does not automatically publish arbitrary mutable
program data.

Program data still requires mutex, atomic, channel or other synchronization.

Terminal cancellation followed by successful join or await establishes the
completion synchronization edge.

---

## Core error types

Cancellation itself is a terminal outcome, not a `CancellationError`.

No generic cancellation runtime error is introduced.

The following related runtime error type must be declared in:

```text
core/errors.sec
```

```sec
enum ThreadTerminationError {
    Unsupported
    PermissionDenied
    InvalidState
    NativeFailure
}
```

Task, thread and process spawn errors and execution failures are declared by
their owning rulebooks in the same core file.

Compiler diagnostics are not runtime errors.

---

## Semantic analysis

The compiler must validate:

- valid current cancellation context;
- cancellation ownership;
- cancellation point commit rules;
- outcome availability;
- no normal value after cancellation;
- detached cleanup path;
- non-cancellable blocking calls;
- task/thread cancellation identity;
- ISR restrictions;
- unsafe termination context.

Whole-program analysis should detect statically evident loops that:

- ignore cancellation;
- contain no cancellation point;
- are detached;
- are required to terminate during shutdown.

---

## Semantic IR

Semantic IR must represent:

```text
CancelRequestTask
CancelRequestThread
CancelObserveCurrentTask
CancelObserveCurrentThread
CancelCurrentTask
CancelCurrentThread
CancellationPointEnter
CancellationPointCommitOperation
CancellationPointCommitCancel
CancellationWake
UnsafeThreadTerminate
```

Cancellation must remain distinct from return, panic and error propagation.

---

## Diagnostics

Examples:

```text
cancel is not valid outside a task or explicit thread context
```

```text
detached thread worker has no statically visible cancellation or completion path
```

```text
blocking extern call NativeWait is not cancellation-aware
```

```text
task cancellation does not cancel the physical worker thread
```

```text
ordinary cancellation request is not permitted in interrupt context
```

Diagnostics must use stable IDs distinct for task, thread and process rules.

---

## Required synchronization

Cross-check and update:

```text
tasks.txt
threads.md
spawn.md
await.md
structured_concurrency.md
blocking.md
scheduling.md
select.md
channels.md
mutex.md
processes.txt
concurrency.md
concurrency_runtime_model.md
concurrency_memory_model.txt
thread_local.md
platform/ffi.md
defer.md
destruction.txt
errorhandling.txt
semantic_ir.txt
diagnostics.txt
core-library.md
core/errors.sec
```
