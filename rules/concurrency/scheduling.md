# Scheduling

## Current implementation status

Implemented:

- compiler-known `ThreadPriority` type;
- compiler-known `ThreadSchedulingError` type.

Not implemented yet:

- scheduling policy validation;
- `Task.Yield()` and `Thread.Yield()`;
- deferred activation scheduling;
- priority and affinity configuration checks;
- blocking/suspension classification;
- starvation/fairness diagnostics;
- scheduling Semantic IR operations.

## Purpose

Scheduling defines how Sec tasks and physical threads are made runnable and how
they may make progress.

Scheduling policy is target-dependent.

Source-level ownership, lifecycle, result, cancellation and memory-order
semantics are target-independent.

A task is not semantically a thread even when a backend uses one native thread
per task.

---

## Execution kinds

Sec distinguishes:

```text
Task
Thread
Process
Interrupt service routine
```

Tasks are logical scheduled operations.

Threads are physical native execution contexts.

Processes have separate process lifecycle and normally a separate address space.

Interrupt service routines are hardware-triggered restricted execution contexts.

A scheduler must not silently change one requested execution kind into another.

---

## Default task scheduling

```sec
let worker := try spawn Work()
```

is equivalent to:

```sec
let worker := try spawn task Work()
```

The selected task profile may use:

- compiler-generated state machines;
- a cooperative executor;
- a worker pool;
- lightweight stackful tasks;
- RTOS tasks;
- native threads;
- an event loop;
- statically scheduled task slots.

The implementation choice must not change task semantics.

A task may resume on a different physical thread after suspension.

---

## Native thread scheduling

```sec
let worker := try spawn thread Work()
```

requests a physical native thread.

The target may map this to:

- a POSIX thread;
- a Windows thread;
- an RTOS native task/thread;
- another target-declared physical execution abstraction.

It must not become an ordinary task.

---

## Eager scheduling

Task and thread spawn are eager by default.

After successful eager creation, the execution entity is:

- submitted;
- runnable;
- running;
- or already completed.

The language does not guarantee that it runs before the next source statement.

---

## Deferred activation

A thread may be created with:

```sec
start: Deferred
```

The callable must not execute before:

```sec
try worker.Start()
```

Deferred activation is a semantic requirement.

A backend may implement it through native suspended creation or an internal
start gate.

It must not ignore the option.

Task and process configuration may adopt the same concept where their rulebooks
explicitly permit it.

---

## Yield

Task yield:

```sec
Task.Yield()
```

means:

> Keep the current task runnable and give the selected task scheduler an
> opportunity to run another ready task.

Thread yield:

```sec
Thread.Yield()
```

means:

> Send a scheduling hint for the current physical thread to the native
> scheduler.

Neither operation guarantees:

- fairness;
- another entity running;
- a context switch;
- progress by a specific waiter;
- synchronization or memory publication.

Yield is not a substitute for:

- blocking waits;
- channels;
- mutexes;
- atomics;
- `select`;
- cancellation checks.

---

## Scheduling points

A task scheduling point may occur at:

- `await`;
- task-context `join`;
- blocking channel operations;
- a waiting `select`;
- mutex acquisition;
- cancellation-aware waits;
- `Task.Yield()`;
- a target-declared suspension operation.

A backend may preempt tasks at additional points when source semantics remain
unchanged.

A physical thread may be preempted by the native scheduler at any permitted
machine instruction boundary.

The language does not promise cooperative-only physical threads.

---

## Fairness

Sec v0.1 provides no universal fairness guarantee.

A target profile must document any stronger guarantee.

The implementation must not claim fairness solely because it uses:

- round-robin scheduling;
- a worker pool;
- OS priorities;
- cooperative yielding.

Source-order `select` is intentionally priority ordered and may starve later
branches.

Programmers requiring independent progress may use another task or thread.

---

## Starvation

Starvation is possible through:

- source-order `select`;
- thread priority differences;
- CPU affinity;
- cooperative tasks that never reach scheduling points;
- lock contention;
- resource monopolization;
- native scheduler behavior.

The compiler should diagnose statically evident starvation hazards when useful,
but starvation is not generally decidable.

---

## Thread priority

Portable thread priority is represented by:

```sec
ThreadPriority
```

The exact portable levels are defined in core and must be few enough to map
across supported targets.

A creation-time request is written in `ThreadConfig`.

A plain value is preferred:

```sec
priority: ThreadPriority.High
```

A required value is explicit:

```sec
priority: Required(ThreadPriority.High)
```

A preferred unsupported priority is ignored with a compile-time warning.

A required unsupported priority is a compile-time error.

Portable levels do not expose arbitrary native numeric priority values.

Target-specific priorities belong behind the platform view.

---

## Runtime priority changes

A target may support:

```sec
try worker.SetPriority(ThreadPriority.High)
```

The operation returns:

```sec
Result[void, ThreadSchedulingError]
```

Runtime priority changes are requests, not compile-time guarantees.

A target without runtime priority changes returns
`ThreadSchedulingError.Unsupported` unless compilation can reject the call for a
fixed target.

---

## CPU affinity

Portable affinity is represented by:

```sec
CpuSet
```

Creation-time affinity belongs in `ThreadConfig`.

A plain affinity value is preferred.

`Required(...)` makes it mandatory.

The compiler must validate compile-time-known CPU indexes against a fixed target
when possible.

A multi-target build may defer machine-count validation to target-specific
lowering or runtime startup where topology is not compile-time fixed.

---

## Runtime affinity changes

A target may support:

```sec
try worker.SetAffinity(CpuSet { 2, 3 })
```

The operation returns:

```sec
Result[void, ThreadSchedulingError]
```

Changing affinity may fail because of:

- unsupported target;
- invalid CPU set;
- permission;
- process restrictions;
- native scheduler failure.

---

## Task affinity

Task affinity is not implied by thread affinity.

A task may migrate between physical threads.

Sec v0.1 does not define portable task affinity.

A future task scheduler profile may provide task affinity, but it must state
whether affinity is:

- executor affinity;
- worker-set affinity;
- physical CPU affinity;
- non-migration;
- best-effort preference.

No thread rule may accidentally give tasks stable TLS or native-thread identity.

---

## Priority inversion

Thread priority does not change mutex ownership semantics.

Targets supporting priority inheritance or priority ceiling may expose those as
mutex or scheduling policies.

The compiler should report known high-risk combinations when profile information
is available.

Sec does not silently insert priority inheritance into every mutex.

---

## Blocking and scheduler integration

A blocking operation in a task context should suspend the task rather than block
an executor worker when the backend can register readiness.

A physical thread wait parks or blocks that physical thread.

A profile may permit task-worker blocking when no nonblocking adapter exists.

Such behavior must be visible in the profile and blocking analysis.

---

## Cancellation and scheduling

Cancellation is cooperative.

A requested cancellation may wake a task or thread waiting in a
cancellation-aware operation.

Waking due to cancellation does not mean the execution entity has already
terminated.

It resumes at a cancellation boundary and performs required cleanup.

Unsafe hard termination is not scheduling.

---

## Main thread

Program entry executes in a physical thread context.

It may or may not also have a task context depending on the target profile.

`Thread.Current()` must be valid when a Sec thread context is attached.

`task` retains its default empty state outside a spawned task unless another rule
creates an explicit root task.

---

## ISR restrictions

ISR execution is not scheduled like a task or ordinary thread.

An ISR must not call:

```sec
Task.Yield()
Thread.Yield()
```

An ISR must not block, await, join or acquire ordinary blocking mutexes.

Deferred work must be transferred through an ISR-safe primitive.

---

## No-required-runtime targets

A bare-metal profile may have:

- one main execution context;
- interrupt handlers;
- a statically generated cooperative executor;
- no native physical thread support.

Such a target may support tasks but reject `spawn thread`.

Another bare-metal target may provide multicore native threads without a general
task runtime.

Capabilities are independent and explicitly declared by the profile.

---

## Compiler analysis

The compiler should record:

- execution kind;
- scheduler profile;
- potential scheduling points;
- possible task migration;
- physical blocking;
- requested priority;
- requested affinity;
- preferred and required options;
- deferred activation;
- ISR context;
- no-block and no-suspend effects.

Whole-program analysis should identify statically provable:

- non-yielding cooperative loops;
- impossible required affinity;
- priority configuration conflicts;
- task suspension while holding forbidden guards;
- recursive unbounded spawn;
- unsupported scheduler operations.

---

## Core error types

The following runtime error type must be declared in:

```text
core/errors.sec
```

```sec
enum ThreadSchedulingError {
    Unsupported
    InvalidValue
    PermissionDenied
    NativeFailure
}
```

`ThreadStartError` and spawn errors are defined by the thread and spawn rules and
must also be declared in `core/errors.sec`.

Compile-time scheduling diagnostics are not runtime errors.

---

## Semantic IR

Semantic IR must distinguish:

```text
TaskYieldCurrent
ThreadYieldCurrent
ThreadPriorityRequest
ThreadAffinityRequest
ThreadStartDeferred
ThreadStart
SchedulerSuspendTask
SchedulerResumeTask
ThreadPark
ThreadWake
```

Yield operations must not be lowered to memory fences.

Scheduling operations must preserve cancellation and ownership effects.

---

## Diagnostics

Examples:

```text
thread affinity is not supported by target; preference will be ignored
```

```text
required thread priority is not supported by target
```

```text
Task.Yield() is not valid outside a task context
```

```text
Thread.Yield() is not valid in an interrupt routine
```

```text
task may resume on another physical thread after this suspension
```

```text
cooperative task loop has no scheduling or cancellation point
```

Diagnostics must have stable IDs.

---

## Required synchronization

Cross-check and update:

```text
spawn.md
tasks.txt
threads.md
await.md
blocking.md
cancellation.md
concurrency.md
concurrency_runtime_model.md
concurrency_memory_model.txt
mutex.md
select.md
thread_local.md
data_races.md
deadlock_analysis.md
compiler_analysis.md
semantic_ir.md
core-library.md
core/errors.sec
```
