# Blocking

## Purpose

Blocking defines operations that wait for progress by another execution entity,
device, timer or platform facility.

Sec distinguishes:

```text
task suspension
physical thread blocking or parking
busy waiting
interrupt context
```

The same source operation may suspend a task or block a physical thread depending
on the caller context and selected backend.

Ownership, commit and memory-order semantics must not change.

---

## Core distinction

In task context, a wait should suspend the logical task when the backend can
register readiness.

In physical thread context, a wait parks or blocks the current physical thread.

Busy waiting repeatedly polls without parking or suspension.

Busy waiting is not the default lowering for a Sec blocking operation.

---

## Potentially blocking operations

The following may wait:

```text
await Task[T]
join Task[T]
join Thread[T]
join Process
Channel.Send
Receiver.Receive
select without ready branch
Mutex.lock
timer wait
I/O readiness or completion
platform wait operations
```

`Try...` operations are nonblocking when their rule explicitly guarantees it.

A `default` branch makes `select` nonblocking.

---

## Await

`await task` is task-specific.

It consumes the owning `Task[T]`.

In a task backend it normally suspends the current task.

A profile may block a physical worker only when:

- the profile explicitly permits it;
- no suspension backend exists;
- deadlock and executor-starvation rules are still enforced.

---

## Join

`join handle` waits for completion while preserving terminal handle information.

Caller context determines the physical wait mechanism.

Task joining a thread:

```text
suspend current task where supported
```

Physical thread joining a thread:

```text
park or block current physical thread
```

A successful join establishes the completion synchronization edge defined by the
concurrency memory model.

---

## Channel operations

Blocking channel send and receive preserve the same commit semantics in task and
thread contexts.

Task context may suspend.

Thread context may block.

Cancellation or timeout must not partially commit the message.

Only the selected successful operation transfers ownership.

---

## Select

A waiting `select` registers all candidate operations and commits exactly one
ready branch.

In task context, it suspends the task.

In thread context, it parks or blocks the thread.

A non-selected branch must not:

- consume a value;
- move a message;
- take a task result;
- change channel state;
- retain ownership after registration is cancelled.

---

## Mutex acquisition

`Mutex[T].lock()` may wait.

In task context the backend should use task-aware suspension where available.

In thread context it blocks or parks.

The mutex's ownership and memory-order semantics are identical.

A runtime must not substitute a task-local lock when the mutex may cross a thread
boundary.

---

## Mutex guards across waits

`MutexGuard[T]` is execution-bound and may not remain live across:

- `await`;
- `join`;
- blocking channel operations;
- waiting `select`;
- task suspension;
- `Task.Yield()`;
- transfer to another task or thread.

The compiler should apply the same conservative rule to physical thread waits
unless a later lock policy explicitly permits a safe blocking wait while holding
that guard.

This prevents:

- executor deadlock;
- lock-order inversion;
- task migration with thread-owned lock state;
- hidden long critical sections.

---

## Cancellation-aware waits

A cancellation-aware wait may resume because cancellation was requested.

The operation must then follow its own commit rule.

For example, a channel send that has not committed retains ownership of the
message.

Cancellation is not permitted to create a partial operation.

The resumed execution entity performs ordinary cleanup before becoming
`Cancelled`.

---

## Timeouts

Timeouts are expressed through `select` and `after` unless a specific API rule
defines another timeout form.

Example:

```sec
select {
    message := rx.Receive() => {
        Use(message)
    }

    after 500ms => {
        HandleTimeout()
    }
}
```

A timeout branch must not commit another branch.

A timeout is not cancellation unless user code explicitly requests or performs
cancellation.

---

## Nonblocking operations

An operation documented as nonblocking must complete immediately with an
explicit state.

Examples include conceptual:

```text
TryLock
TrySend
TryReceive
status observation
```

Nonblocking does not mean successful.

It means the operation returns without waiting.

---

## Busy waiting

The compiler and backend must not lower ordinary waits to unbounded busy loops
unless the target profile explicitly selects a busy-wait implementation.

A busy-wait profile must document:

- CPU consumption;
- memory ordering;
- interrupt interaction;
- cancellation checks;
- power implications;
- maximum intended duration.

Compiler-generated busy loops must contain the required atomic or volatile
semantics.

---

## Blocking effects

Functions may be classified by effects such as:

```text
mayBlock
noBlock
maySuspend
noSuspend
```

The existing attributes include:

```sec
@noBlock
```

A function marked `@noBlock` must not directly or transitively call an operation
that may physically block on the selected target.

Task suspension and physical blocking are separate effects.

A function may be nonblocking for a physical thread while still permitting task
suspension only when its contract says so explicitly.

---

## Call graph analysis

The compiler should use the complete call graph to determine:

- whether a function may block;
- whether a function may suspend;
- which target profiles change lowering;
- whether an ISR reaches blocking code;
- whether a detached execution entity has a finite shutdown path;
- whether a mutex guard crosses a wait;
- whether a cooperative executor can lose all workers to blocking calls.

Indirect calls must carry conservative effect information.

---

## FFI

An extern function is potentially blocking unless its declaration or imported
contract states otherwise.

Example conceptual metadata:

```sec
@noBlock
unsafe extern "C" fn NativeTryRead(...) int
```

Incorrect external effect declarations are unsafe contract violations.

The compiler may trust them for optimization and diagnostics but cannot verify
foreign implementation behavior.

---

## ISR context

An interrupt routine must not:

- await;
- join;
- block on channels;
- wait in `select`;
- acquire an ordinary blocking mutex;
- call an unannotated potentially blocking extern function;
- sleep;
- yield.

ISR-safe operations must be bounded and explicitly declared.

---

## Main and initialization

Blocking during initialization is profile-dependent.

An embedded profile may forbid indefinite waits during `@init`.

A hosted profile may allow them.

Whole-program analysis should report initialization cycles where one initializer
waits for work that cannot start until initialization finishes.

---

## Deadlock interaction

Blocking operations contribute edges to deadlock analysis.

The compiler should track:

- held mutex identities;
- joined execution entities;
- awaited tasks;
- channel dependencies;
- deferred thread start;
- executor worker consumption;
- cancellation dependencies.

Statically proven deadlocks are compiler errors.

Potential cycles may be warnings or configurable diagnostics.

---

## Memory ordering

A wait operation establishes memory ordering only when its own rule defines a
synchronization edge.

Examples:

- successful join;
- successful await;
- mutex lock after unlock;
- channel receive after matching send;
- committed atomic wait/wake where specified.

Sleeping, yielding or timing out does not by itself publish ordinary memory.

---

## Runtime errors

This rulebook introduces no independent runtime `BlockingError`.

Operation-specific failures belong to their owning APIs and all language-level
runtime error types must be declared in:

```text
core/errors.sec
```

Compiler errors such as blocking in ISR or violating `@noBlock` remain
diagnostics, not core runtime errors.

---

## Semantic IR

Semantic IR must distinguish:

```text
TaskSuspend
TaskResume
ThreadPark
ThreadWake
BlockingJoin
SuspendingJoin
BlockingChannelWait
SuspendingChannelWait
BlockingMutexWait
SuspendingMutexWait
SelectRegister
SelectSuspend
SelectPark
SelectCommit
TimeoutRegister
CancellationWake
```

The IR must record whether a wait may physically block under the selected
profile.

---

## Diagnostics

Examples:

```text
blocking operation is not permitted in @noBlock function Poll
```

```text
interrupt routine UartIRQ may call blocking function Read
```

```text
MutexGuard[State] cannot remain live across join
```

```text
all executor workers may block waiting for tasks scheduled on the same executor
```

```text
busy-wait lowering is not permitted by target profile
```

Diagnostics must have stable IDs.

---

## Required synchronization

Cross-check and update:

```text
await.md
threads.md
tasks.txt
channels.md
select.md
mutex.md
scheduling.md
cancellation.md
deadlock_analysis.md
data_races.md
concurrency.md
concurrency_runtime_model.md
concurrency_memory_model.md
ffi.txt
compiler_analysis.txt
semantic_ir.txt
diagnostics.txt
core/errors.sec
```
