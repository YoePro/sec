# Threads

## Current implementation status

Implemented:

- compiler-known `Thread[T]` type;
- compiler-known `ThreadObserver[T]`, `ThreadConfig`, `ThreadContext`,
  `ThreadID`, `ThreadPriority`, `ThreadStatus`, `ThreadSpawnError`,
  `ThreadStartError`, `ThreadSchedulingError`, `ThreadTerminationError` and
  `ThreadContextError` types;
- `Thread[T]` is classified as move-only;
- unresolved `Thread[T]` locals are diagnosed at scope exit;
- `discard thread` is rejected for unresolved `Thread[T]` handles;
- `detach thread` consumes a `Thread[void]` handle;
- `detach thread discard` consumes a non-void `Thread[T]` handle with explicit
  result discard.

Not implemented yet:

- `spawn thread` result model as `Result[Thread[T], ThreadSpawnError]`;
- thread configuration parsing and semantic validation;
- `join thread`;
- thread handle member access and methods;
- thread lifecycle status tracking;
- detached-thread reference escape analysis;
- detached-thread shutdown/runtime integration;
- cancellation request methods;
- thread lowering/runtime integration.

## Purpose

A thread is an explicit physical target execution context.

A Sec thread is distinct from:

- a logical `Task[T]`;
- a separate process;
- the physical worker thread currently executing a migratable task;
- an interrupt service routine.

`spawn thread` requests a native physical thread or the target profile's declared
native thread equivalent.

The backend must never silently lower `spawn thread` to an ordinary task.

---

## Basic syntax

```sec
let worker := try spawn thread Work()
```

If `Work()` returns `T`, the successful value has type:

```sec
Thread[T]
```

The complete expression before `try` has type:

```sec
Result[Thread[T], ThreadSpawnError]
```

A caller may handle creation failure explicitly:

```sec
match spawn thread Work() {
    Ok(worker) => {
        join worker
    }

    Err(error) => {
        HandleThreadSpawnError(error)
    }
}
```

`spawn thread` is eager by default.

---

## Thread creation failure

Native thread creation is fallible.

The source expression:

```sec
spawn thread Work()
```

returns:

```sec
Result[Thread[T], ThreadSpawnError]
```

`try` follows the ordinary Sec error-propagation rules:

```sec
let worker := try spawn thread Work()
```

A target that has no physical thread implementation must reject the program at
compile time.

This is not a runtime `ThreadSpawnError`.

Expected diagnostic:

```text
target profile does not support physical threads
```

---

## Thread configuration

Thread configuration may be written inline:

```sec
let worker := try spawn thread {
    name: "worker",
    stack: 64KiB,
    affinity: CpuSet { 2, 3 },
} Work()
```

The configuration block is contextually typed as `ThreadConfig`.

It is not an untyped map or anonymous struct.

A named configuration is also valid:

```sec
let myThreadConfig := ThreadConfig {
    name: "worker",
    stack: 64KiB,
    affinity: CpuSet { 2, 3 },
}

let worker := try spawn thread myThreadConfig Work()
```

The grammar is conceptually:

```text
spawn
execution-kind
optional-configuration
call-expression
```

The callable expression is always last.

---

## ThreadConfig

`ThreadConfig` is a compiler-known core type.

The initial model must support at least:

```text
name
stack
affinity
priority
start
storage
```

Additional target-specific information belongs behind target-resolved platform
configuration or a future extension mechanism.

A missing field uses the target profile's declared default.

---

## Preferred and required configuration

A configuration value may be either:

```text
Preferred
Required
```

A plain portable configuration value is preferred by default.

Example:

```sec
let worker := try spawn thread {
    affinity: CpuSet { 2, 3 },
} Work()
```

If the target does not support affinity:

- the compiler emits a warning;
- the affinity request is ignored;
- the remaining program is compiled;
- the diagnostic has a stable ID;
- the build profile may promote the warning to an error.

A semantically required option must be written explicitly:

```sec
let worker := try spawn thread {
    affinity: Required(CpuSet { 2, 3 }),
} Work()
```

If a required option cannot be guaranteed, compilation fails.

The compiler must never ignore a requirement whose absence changes source-level
program semantics.

The exact internal representation of `Preferred[T]` and `Required[T]` may be
compiler-known, but their meaning is shared by task, thread and process
configuration where applicable.

---

## Configuration classification

The following are normally preferences:

```text
name
affinity
priority
diagnostic metadata
```

The following are semantic requirements:

```text
deferred start
explicit backing storage
required affinity
required scheduling policy
minimum stack guarantee
```

A target profile may classify additional target-specific fields.

The classification must be known during semantic analysis or target validation.

---

## Eager and deferred start

Default:

```text
start: Eager
```

The user callable may begin as soon as creation succeeds.

Deferred creation is explicit:

```sec
let worker := try spawn thread {
    start: Deferred,
} Work()
```

After successful deferred creation:

```text
worker.status == ThreadStatus.Created
```

The callable must not execute before explicit start.

Start is fallible:

```sec
try worker.Start()
```

The expression before `try` returns:

```sec
Result[void, ThreadStartError]
```

After successful start:

```text
worker.status == ThreadStatus.Running
```

Calling `Start()` twice is a semantic error when statically provable and a
`ThreadStartError.InvalidState` otherwise.

Deferred start is not restricted to embedded targets.

A hosted backend may use native suspended creation or an internal start gate.
The implementation is valid only when no user callable code executes before
`Start()`.

If a target cannot preserve deferred semantics, it must reject that
configuration.

---

## Explicit thread storage

A target may support explicit thread backing storage:

```sec
static let workerStorage := ThreadStorage[64KiB]()

let worker := try spawn thread {
    start: Deferred,
    storage: ref workerStorage,
} Work()
```

Explicit storage is a requirement.

It must not be ignored or replaced by hidden heap allocation.

The compiler must validate:

- storage lifetime;
- exclusive ownership while the thread exists;
- stack size and alignment;
- target compatibility;
- detached lifetime;
- re-use only after complete cleanup.

Explicit storage is especially useful on MCU and RTOS targets but is not limited
to them.

---

## Thread type

`Thread[T]` is a compiler-known move-only lifecycle and result owner.

It owns:

- the join capability;
- lifecycle responsibility;
- the terminal outcome;
- the normal result when one exists;
- native cleanup obligations not yet released.

It is not copyable even when `T` is copyable.

Moving a `Thread[T]` transfers all unresolved lifecycle responsibility.

`Thread[T]`, `Task[T]` and process handles are distinct types.

---

## Common handle members

Thread handles harmonize member names with task and process handles where the
meaning is shared.

At minimum:

```sec
worker.id
worker.name
worker.status
worker.value
worker.Observe()
worker.platform
```

The member types remain thread-specific:

```text
worker.id      ThreadID
worker.status  ThreadStatus
```

The common member name does not require a common enum or machine
representation.

---

## Thread identity

`ThreadID` is the stable immutable Sec identity of a thread execution entity.

```sec
let id := worker.id
```

It is distinct from:

- `TaskID`;
- `ProcessID`;
- a platform thread ID;
- a native owning handle.

The target-resolved platform view may expose immutable native identity:

```sec
let nativeID := worker.platform.id
```

Native identity must use a target-appropriate type.

It must not be silently converted to `uint64`.

A raw owning or mutating platform handle requires `unsafe`.

---

## Name

A thread name is immutable through the portable thread handle:

```sec
let name := worker.name
```

The preferred name is supplied before execution through `ThreadConfig`.

A target may truncate, normalize or ignore a preferred name when it reports a
compile-time warning.

A required naming guarantee must fail if the target cannot satisfy it.

A platform-specific runtime rename operation, if exposed, belongs behind
`worker.platform` and may be fallible.

---

## Thread status

`ThreadStatus` is a compiler-known or core-defined thread-specific enum.

The initial semantic states are:

```text
Created
Running
Completed
Cancelled
Panicked
Terminated
```

`Completed` means the callable returned normally.

This includes a callable returning:

```sec
Err(error)
```

when its declared return type is `Result[T, E]`.

`Cancelled` means cooperative cancellation completed.

`Panicked` means the callable terminated through Sec panic.

`Terminated` means unsafe or platform-level abnormal termination prevented
normal Sec completion.

A backend may use additional internal states but must expose the same source
semantics.

---

## Normal result

A thread returning `T` stores a normal result only when its status is
`Completed`.

```sec
join worker
let result := worker.value
```

`worker.value` is unavailable before successful completion synchronization.

For copyable `T`, each ordinary read follows normal copy semantics:

```sec
join worker

let first := worker.value
let second := worker.value
```

The stored value remains until it is moved, discarded or the joined handle is
destroyed.

For move-only `T`, ordinary extraction moves the result:

```sec
join worker
let file := worker.value
```

No explicit `move(...)` syntax is required.

A second extraction of a moved result is invalid.

---

## Thread[void]

A callable returning `void` produces:

```sec
Thread[void]
```

After successful join:

```sec
worker.value
```

exists and has type `void`.

It represents normal completion without a payload.

It is not `Option[void]` and does not return `None`.

---

## Join

```sec
join worker
```

waits for terminal thread completion.

A successful join:

- establishes thread-completion synchronization;
- consumes the join capability;
- releases join-owned native resources;
- preserves immutable identity and terminal state;
- preserves an unconsumed normal result;
- marks the handle as joined.

The same source binding remains usable for terminal inspection:

```sec
join worker

let status := worker.status
let id := worker.id
let result := worker.value
```

A second join is invalid.

`join` does not itself consume a copyable result.

---

## Join from task and thread contexts

When called from a task context:

```sec
join worker
```

must suspend the current task when the selected backend can register thread
completion without blocking its physical executor worker.

When called from a physical thread context, it parks or blocks the current
physical thread.

The source-level lifecycle, ownership and memory-order semantics are identical.

A backend that cannot provide task suspension may block only when the selected
profile explicitly permits that behavior.

---

## Outcome after join

Normal value access is valid only for `Completed`.

The terminal state may be inspected:

```sec
join worker

match worker.status {
    Completed => Use(worker.value)
    Cancelled => HandleCancellation()
    Panicked => HandlePanic(worker.panic)
    Terminated => HandleTermination(worker.termination)
}
```

The exact payload types for panic and termination are defined by the panic and
platform rules.

A cancelled, panicked or terminated thread has no normal `value`.

---

## Observer

A thread may create a non-owning observer:

```sec
let observer := worker.Observe()
```

The observer type is:

```sec
ThreadObserver
```

or a compiler-equivalent thread-specific observer type.

An observer may:

- read immutable identity;
- read the name;
- read status;
- participate in completion observation and `select`;
- be copied when its retained-state representation permits it.

An observer may not:

- join;
- detach;
- start a deferred thread;
- take `value`;
- own native lifecycle cleanup;
- force termination.

The observer must not keep join-only native resources alive after the owning
handle resolves them.

---

## Detach

A `Thread[void]` may be detached:

```sec
detach worker
```

A non-void result requires explicit discard:

```sec
detach worker discard
```

Detach:

- consumes the local thread handle;
- relinquishes join and result ownership;
- allows execution to continue;
- transfers cleanup responsibility to the target runtime or program lifecycle
  manager;
- does not establish a completion synchronization edge.

Detached code must not retain references to scope-owned values that may die
before the thread.

---

## Cooperative cancellation

Portable thread cancellation is cooperative.

```sec
worker.RequestCancel()
```

requests cancellation but does not force immediate termination.

The running callable may observe:

```sec
Thread.Current().cancelRequested
```

or reach a cancellation-aware blocking operation.

The current thread may terminate as cancelled through the general `cancel`
statement.

Cancellation must run ordinary cleanup, `defer` and deterministic destruction.

---

## Unsafe hard termination

Safe Sec has no general force-kill operation.

A target may expose an unsafe platform operation:

```sec
unsafe {
    try worker.platform.Terminate()
}
```

This may produce:

```sec
Result[void, ThreadTerminationError]
```

After successful hard termination:

```text
worker.status == ThreadStatus.Terminated
```

No guarantee is made that:

- `defer` executed;
- destructors executed;
- mutexes were unlocked;
- invariants remain valid;
- thread-local values were destroyed;
- a normal result exists.

A process or watchdog is the preferred isolation boundary for code that must be
recoverable after non-cooperative failure.

---

## Current physical thread

```sec
let current := Thread.Current()
```

returns an immutable non-owning:

```sec
ThreadContext
```

It is not `Thread[T]`.

It may represent:

- the main thread;
- a Sec-created native thread;
- a foreign thread attached through FFI;
- the physical executor thread currently running a task.

At minimum it exposes:

```sec
current.id
current.name
current.cancelRequested
current.platform.id
```

It cannot be joined, detached or used to obtain a result.

---

## Yield

```sec
Thread.Yield()
```

requests that the native scheduler give another runnable physical thread an
opportunity to execute.

It is a scheduling hint.

It does not guarantee:

- that another thread runs;
- fairness;
- a context switch;
- any memory synchronization.

`Thread.Yield()` is invalid in ISR context.

Task yielding is separately written:

```sec
Task.Yield()
```

---

## Platform view

```sec
worker.platform
```

is a target-resolved immutable view type.

Examples may include:

```text
LinuxThreadPlatform
WindowsThreadPlatform
FreeRTOSThreadPlatform
```

Portable source may use common immutable information.

Target-specific source may use platform members under target build conditions.

A raw native handle requires `unsafe`.

Version 1.0 must provide compile-time target conditions so one source tree can
select target-specific code without duplicating entire implementations.

---

## Memory model

Successful creation publishes all moved, copied and valid borrowed arguments to
the new thread.

All writes sequenced before successful creation happen-before the new thread's
first access to those published values.

All ordinary writes before normal terminal completion happen-before a successful
join observing that completion.

Detach does not create a completion synchronization edge.

Status polling alone is not a replacement for join.

---

## Core error types

The following runtime error types must be declared in:

```text
core/errors.sec
```

```sec
enum ThreadSpawnError {
    OutOfMemory
    ResourceLimit
    StackAllocationFailed
    InvalidConfiguration
    PermissionDenied
    ThreadLocalInitializationFailed
    NativeFailure
}

enum ThreadStartError {
    InvalidState
    ResourceUnavailable
    PermissionDenied
    NativeFailure
}

enum ThreadSchedulingError {
    Unsupported
    InvalidValue
    PermissionDenied
    NativeFailure
}

enum ThreadTerminationError {
    Unsupported
    PermissionDenied
    InvalidState
    NativeFailure
}
```

Compiler diagnostics such as unsupported target, invalid ownership and
use-after-join are not runtime error values and must not be added to
`core/errors.sec`.

---

## Semantic analysis

The compiler must validate:

- target thread support;
- callable and return type;
- `ThreadConfig`;
- preferred and required options;
- spawn error type;
- moved, copied and borrowed arguments;
- backing storage lifetime;
- deferred start state;
- one join capability;
- value availability;
- move-only result extraction;
- detach result discard;
- observer restrictions;
- cancellation context;
- platform access;
- unsafe termination;
- thread-local initialization requirements.

---

## Semantic IR

Semantic IR must represent at least:

```text
ThreadCreate
ThreadCreateDeferred
ThreadStart
ThreadMove
ThreadObserve
ThreadCancelRequest
ThreadJoin
ThreadTakeValue
ThreadCopyValue
ThreadDiscardValue
ThreadDetach
ThreadDetachDiscard
ThreadComplete
ThreadCancel
ThreadPanic
ThreadTerminateUnsafe
ThreadYieldCurrent
ThreadCurrentContext
```

Creation IR must record:

- callable;
- arguments;
- result type;
- configuration;
- preferred and required options;
- copied and moved values;
- retained borrows;
- storage strategy;
- target profile;
- source location.

---

## Diagnostics

Examples:

```text
target profile does not support physical threads
```

```text
required thread affinity is not supported by target
```

```text
thread affinity is not supported by target; preference will be ignored
```

```text
cannot join deferred thread worker before it is started
```

```text
thread worker has already been joined
```

```text
thread value is unavailable before successful join
```

```text
thread worker completed without a normal value because it panicked
```

```text
detaching Thread[T] with non-void result requires explicit discard
```

```text
detached thread worker cannot retain reference to local value data
```

Diagnostics must use stable rule-specific IDs.

Task, thread and process diagnostics must have distinct IDs even when their text
is similar.

---

## Required synchronization

This rule must be merged with and cross-checked against:

```text
spawn.md
tasks.txt
await.md
processes.txt
concurrency.md
concurrency_memory_model.txt
scheduling.md
blocking.md
transferability.md
cancellation.md
concurrency_runtime_model.md
thread_local.md
structured_concurrency.md
data_races.md
deadlock_analysis.md
static.md
mutex.md
channels.md
select.md
copy_move.md
ownership.md
borrowing.txt
lifetime_analysis.txt
destruction.txt
errorhandling.md
semantic_ir.txt
core-library.md
rules_implementations.txt
core/errors.sec
```

In particular, older examples where `spawn thread` directly returns `Thread[T]`
must be changed to account for `Result[Thread[T], ThreadSpawnError]` or `try`.
