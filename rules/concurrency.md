# Concurrency

## Purpose

Concurrency defines how multiple tasks execute, communicate and access shared
state safely.

This document defines the language-level concurrency model.

Detailed rules for task creation, waiting, mutexes, atomics, static storage and
memory ordering are defined in their respective documents.

---

## Core principles

Sec concurrency is based on:

- explicit task creation
- deterministic ownership
- compile-time borrow analysis
- explicit synchronization
- cooperative cancellation
- no implicit shared mutable state
- no hidden data races
- no mandatory garbage collector
- no runtime ownership tracking

Concurrency must not weaken the normal ownership, borrowing or destruction
rules.

---

## Tasks and concurrency

A task is one independently scheduled operation.

Tasks are created explicitly with:

```sec
spawn
```

Tasks may execute:

- concurrently
- in parallel
- cooperatively
- on the same physical thread
- on different physical threads

The selected backend decides the execution mechanism.

Source-level semantics must remain unchanged.

---

## Concurrency is not parallelism

Concurrency means multiple operations may make progress independently.

Parallelism means multiple operations execute at the same physical time.

A target may support concurrency without parallelism.

Examples include:

- a single-thread event loop
- a cooperative scheduler
- a bare-metal state-machine executor

The language must not assume that every task has its own thread.

---

## Shared state

Shared immutable state is permitted when its lifetime is valid.

Shared mutable state requires explicit synchronization.

Examples of valid synchronization mechanisms include:

- `Mutex[T]`
- atomics
- future `RwLock[T]`
- ownership transfer
- message passing
- task-confined state

The compiler must reject unsynchronized conflicting access when it can prove the
conflict.

The runtime or selected profile may provide additional checked diagnostics when
the conflict cannot be fully proven statically.

---

## Task-confined state

A value owned exclusively by one task requires no synchronization.

Example:

```sec
fn Worker(data: Data) void {
    Process(data)
}

let data := Data.Create()
let worker := spawn Worker(data)
```

If `Data` is move-only, ownership moves into the task.

No other task may access the value unless ownership is transferred again or a
valid synchronized shared abstraction is used.

Task confinement is the preferred model for mutable state.

---

## Shared immutable borrows

Multiple tasks may read the same value through shared references only when:

- the value outlives every task use
- the type permits concurrent shared access
- no task mutates the same location concurrently
- the memory model guarantees valid publication

Example:

```sec
let configuration := LoadConfiguration()

let first := spawn ReadConfiguration(ref configuration)
let second := spawn ReadConfiguration(ref configuration)

await first
await second
```

The compiler must track both borrows until the tasks complete.

---

## Shared mutable access

A plain mutable reference must not be shared concurrently between tasks.

Invalid:

```sec
let mut state := State.Create()

let first := spawn Update(ref mut state)
let second := spawn Update(ref mut state)
```

The two mutable borrows overlap.

Expected diagnostic:

```text
cannot create overlapping mutable task borrows of state
```

Shared mutation requires a concurrency-safe abstraction such as:

```sec
Mutex[State]
```

or an appropriate atomic type.

---

## Ownership transfer

Ownership may move between tasks.

Example:

```sec
fn Produce() Data {
    return Data.Create()
}

let producer := spawn Produce()
let data := await producer

let consumer := spawn Consume(data)
await consumer
```

Ownership transfer must remain explicit in Semantic IR.

The compiler must always know which task owns an owning value.

---

## Static storage

Static lifetime does not imply concurrency safety.

Example:

```sec
static let mut State: ApplicationState
```

This has sufficient lifetime for detached tasks but remains shared mutable
storage.

Concurrent unsynchronized access is invalid.

Preferred form:

```sec
static let State: Mutex[ApplicationState]
```

Detailed static rules are defined in `static.txt`.

---

## Task boundaries

The following operations create or cross task boundaries:

- `spawn`
- returning a `Task[T]`
- moving a task handle
- detaching a task
- message transfer between tasks
- process communication through IPC

A task boundary may change:

- ownership
- borrow duration
- cancellation relationships
- synchronization requirements
- memory visibility

The compiler must represent each boundary explicitly.

---

## Parent and child tasks

A task that creates another task becomes its parent.

Parent-child relationships may be used for:

- cancellation propagation
- diagnostics
- shutdown ordering
- tracing
- task limits
- structured concurrency

The child task handle still has one explicit owner.

A parent task must not exit while it still owns unresolved child tasks.

A child may be:

- awaited
- joined and resolved
- returned
- moved to another owner
- detached

---

## Cancellation

Cancellation is cooperative.

Requesting cancellation does not forcibly terminate a task.

A task may inspect:

```sec
task.cancelRequested
```

A task may terminate as cancelled with:

```sec
cancel
```

Normal `return` produces normal completion.

Cancellation must execute:

- defer cleanup
- deterministic destruction
- borrow cleanup
- task-local resource release

Forced process termination cannot guarantee cleanup.

---

## Cancellation propagation

Cancellation may propagate from parent to child tasks.

The initial rule should be:

- parent cancellation requests cancellation of owned child tasks
- detached tasks are not cancelled merely because their original creator exits
- program shutdown requests cancellation of detached tasks
- moved task ownership preserves the task's current cancellation state

A child created after cancellation has already been requested may begin with:

```sec
task.cancelRequested == true
```

The compiler and scheduler must not silently clear inherited cancellation.

---

## Cancellation awareness

A task is cancellation-aware when it has a reachable strategy for reacting to a
cancellation request.

Evidence may include:

- reading `task.cancelRequested`
- executing `cancel`
- calling a cancellation-aware operation
- waiting at a cancellation point
- having a statically finite execution path

Detached tasks should be cancellation-aware.

The compiler should diagnose statically evident detached tasks without a
cancellation or finite completion path.

Example:

```sec
fn InvalidWorker() void {
    while true {
        ProcessNext()
    }
}
```

When detached, this should produce a diagnostic unless the selected profile
explicitly permits non-cancellable background execution.

---

## Cancellation points

Operations that may suspend or block may be cancellation points.

Version 0.1 cancellation points include at least:

- `await`
- `join`
- `Mutex.lock()`
- task-aware waits
- future channel receive operations
- future timer waits

A cancellation point must define whether it:

- exits the current task as cancelled
- returns an explicit outcome
- ignores cancellation until completion

The default task-aware behavior should preserve cooperative cancellation.

---

## Blocking operations

An operation may be:

- non-blocking
- task-suspending
- thread-blocking
- indefinitely blocking
- cancellation-aware
- non-cancellable

These properties must be known to semantic analysis when relevant.

A blocking system or FFI call must not be assumed to be cancellation-aware.

Detached tasks using non-cancellable indefinite blocking should produce a
diagnostic when this can be determined.

---

## Mutexes

`Mutex[T]` is the primary exclusive shared-state primitive in version 0.1.

A mutex:

- owns exactly one `T`
- permits one exclusive guard at a time
- is non-reentrant
- returns `MutexGuard[T]` from `lock()`
- returns `Option[MutexGuard[T]]` from `tryLock()`
- may support overloaded `lock(...)` forms with timeout or context
- treats lock waiting as a cancellation point

A live mutex guard may not cross:

- `await`
- task transfer
- `spawn`
- `detach`

Detailed rules are defined in `mutex.txt`.

---

## Lock overloads

Sec supports function overloading.

Mutex waiting behavior should therefore prefer overloads over many unrelated
method names.

Examples:

```sec
let guard := State.lock()
let guard := State.lock(timeout)
let guard := State.lock(context)
let guard := State.lock(timeout, context)
```

The exact context type is defined separately.

Possible outcomes include:

```sec
MutexGuard[T]
Option[MutexGuard[T]]
Result[MutexGuard[T], LockError]
```

The chosen return type must match the meaning of the overload.

Ordinary blocking acquisition should not use `try` merely because it waits.

A non-blocking attempt remains:

```sec
State.tryLock()
```

---

## Atomics

Atomics provide synchronization for supported primitive operations.

They should be used for small independently updated values such as:

- counters
- flags
- state markers
- generation values

Atomics must not replace a mutex for multi-field invariants.

Detailed atomic operations and ordering rules are defined in `atomics.txt` and
`concurrency_memory_model.txt`.

---

## Data races

A data race occurs when:

- two tasks access the same memory location concurrently
- at least one access writes
- the accesses are not synchronized
- the accesses are not both valid atomic operations

Data races are invalid Sec behavior.

The compiler should reject statically provable data races.

Sec must not define ordinary data races as acceptable undefined behavior that
programmers are expected to debug manually.

When static proof is impossible, checked runtime instrumentation may be offered
by selected profiles.

---

## Race analysis

The compiler should analyze:

- static mutable storage
- shared references across tasks
- mutable references across tasks
- captured closure values
- task-returned references
- mutex-protected storage
- atomic storage
- task ownership transfer
- detached task lifetimes

Diagnostics should identify:

- the shared location
- the conflicting accesses
- the task creation sites
- the missing synchronization

---

## Task bombs

The compiler should detect statically evident unbounded task creation.

Examples include:

- direct recursive spawn
- indirect spawn cycles
- exponential child-task creation
- detached spawn in unbounded loops
- task creation without retained handles

Example:

```sec
fn Bomb() void {
    let first := spawn Bomb()
    let second := spawn Bomb()

    await first
    await second
}
```

Expected diagnostic shape:

```text
unbounded recursive task creation detected:
Bomb -> spawn Bomb
```

Target profiles may impose:

- maximum task count
- maximum detached task count
- static task-slot allocation
- per-parent limits
- scheduler queue limits

---

## Deadlocks

The compiler should detect statically provable deadlocks.

Version 0.1 should detect at least:

- same task locking the same non-reentrant mutex twice
- a mutex guard remaining active across `await`
- direct lock-order cycles when trivially provable
- waiting on a task that directly waits on the current task

Full dynamic deadlock detection is not required.

Complex deadlocks may remain runtime errors or profile diagnostics.

---

## Lock ordering

A future extension may support declared lock ordering.

Version 0.1 should not require explicit lock ranks.

The compiler may still detect simple inconsistent orderings from the call graph
when identities are statically known.

---

## Observers

Multiple task observers may inspect one task.

Observers do not own:

- the task
- the task result
- cancellation responsibility

Observers may wait for completion but may not independently consume `T`.

The task owner remains responsible for resolving the task lifecycle.

---

## Detached tasks

Detaching transfers task ownership to the program task manager.

Detached tasks:

- may continue after the originating scope exits
- may not retain scope-owned borrows
- should be cancellation-aware
- are cancelled during normal program shutdown
- must complete cleanup before normal termination

Ordinary detach accepts:

```sec
Task[void]
```

Non-void detach requires explicit discard:

```sec
detach task discard
```

---

## Synchronization and publication

Moving an owned value into a task publishes that value to the new task.

Successful join or await publishes the completed task result back to the waiting
task.

Mutex unlock publishes protected writes to a later successful lock.

Atomic operations publish according to their selected memory order.

The precise happens-before rules are defined in
`concurrency_memory_model.txt`.

---

## Unsafe code

Unsafe code does not disable concurrency validation.

Inside unsafe, the compiler must still enforce:

- task ownership
- mutex guard lifetime
- ordinary borrow rules
- static storage rules
- cancellation cleanup
- valid Result handling

Raw pointers may bypass some alias proof, but they do not make data races valid.

Concurrent raw-pointer access remains the programmer's explicit unsafe
responsibility.

---

## FFI interaction

Foreign calls may:

- block
- create foreign threads
- call back into Sec
- access shared memory
- ignore Sec cancellation

FFI declarations or wrappers should describe relevant concurrency effects.

A foreign callback executing concurrently with Sec tasks must obey the same
memory and synchronization rules.

The compiler must not infer thread safety from a raw pointer or foreign handle.

---

## Target profiles

A target concurrency profile defines implementation constraints such as:

- scheduler type
- native thread availability
- lightweight task support
- maximum tasks
- stack model
- supported blocking operations
- supported atomics
- mutex backend
- cancellation support

Profiles must not change core language semantics.

A target that cannot implement a required feature must reject the program.

---

## Semantic analysis

The compiler must track at least:

- task ownership
- task parentage
- task escape
- shared memory locations
- mutable access
- active borrows
- active mutex guards
- atomic access
- cancellation awareness
- blocking effects
- task creation cycles
- task limits
- task completion dependencies

Concurrency analysis should use the whole-program call graph when available.

---

## Semantic IR

Semantic IR must make concurrency operations explicit.

At minimum:

```text
TaskCreate
TaskMove
TaskDetach
TaskAwait
TaskJoin
TaskCancelRequest
TaskCancelCurrent
MutexLock
MutexTryLock
MutexUnlock
AtomicLoad
AtomicStore
AtomicReadModifyWrite
ConcurrencyFence
```

IR must record:

- ownership transfer
- borrow retention
- synchronization edges
- memory order
- cancellation behavior
- blocking behavior
- task identity
- source location

The backend must not infer concurrency semantics from ordinary calls or loads.

---

## Diagnostics

Examples:

```text
cannot create overlapping mutable task borrows of state
```

```text
shared mutable access to State requires synchronization
```

```text
detached task worker has no statically visible cancellation path
```

```text
mutex State is already locked by the current task
```

```text
mutex guard state remains active across await
```

```text
unbounded recursive task creation detected
```

```text
target profile supports at most 32 concurrent tasks
```

```text
foreign call may block indefinitely and is not cancellation-aware
```

---

## Restrictions

Concurrency must not:

- introduce implicit tasks
- silently detach work
- silently share mutable state
- bypass ownership
- bypass borrowing
- copy move-only task handles
- copy mutex guards
- permit unsynchronized data races
- assume one task equals one thread
- require one universal runtime implementation
- silently downgrade concurrent execution to synchronous execution

---

## Related rules

Detailed behavior is defined in:

```text
tasks.txt
spawn.txt
await.txt
mutex.txt
atomics.txt
static.txt
concurrency_memory_model.txt
processes.txt
spawn_process.txt
ipc.txt
```
