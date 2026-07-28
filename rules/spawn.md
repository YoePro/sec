# Spawn

## Purpose

`spawn` starts an operation as a new execution entity.

It is the explicit boundary between synchronous execution and concurrent
execution.

A normal function call remains synchronous.

```sec
let value := Calculate()
```

A spawned function call creates a task by default.

```sec
let task := spawn Calculate()
```

This is shorthand for:

```sec
let task := spawn task Calculate()
```

## Syntax

```text
spawn-expression:
    spawn expression
    spawn task expression
    spawn thread expression
    spawn process expression
```

Typical use:

```sec
let worker := spawn Work()
let taskWorker := spawn task Work()
let threadWorker := spawn thread Work()
let childProcess := spawn process Program()
```

The initial implementation requires the operand of `spawn` to be a callable expression.

Examples:

```sec
spawn Work()
spawn Process(value)
spawn service.Run()
spawn callback(value)
```

`task`, `thread` and `process` are contextual spawn modifiers only when they
occur directly after `spawn`.

They do not need to be general-purpose reserved keywords.

Example:

```sec
let thread := "worker"
```

may remain valid when `thread` is not reserved elsewhere.

`spawn` is an expression and may be used wherever its resulting execution handle
is valid.

## Result type

If the spawned operation returns `T`, these forms have the following conceptual
result types:

```sec
spawn Work()          // Task[T]
spawn task Work()     // Task[T]
spawn thread Work()   // Thread[T]
spawn process Work()  // process handle type defined by processes.txt
```

`spawn expression` is exactly equivalent to `spawn task expression`.

`spawn.md` does not finalize the process result model.

Example:

```sec
fn Calculate() int {
    return 42
}

let calculation := spawn Calculate()
```

`calculation` has type `Task[int]`.

A function returning `void` through `spawn` or `spawn task` produces
`Task[void]`.

A fallible function preserves its complete return type:

```sec
fn Load() Result[Image, IOError] {
}

let loading := spawn Load()
```

The task type is:

```sec
Task[Result[Image, IOError]]
```

`spawn` does not unwrap or alter the function return type.

`Task[T]`, `Thread[T]` and the process handle type are distinct handle types.

They are not interchangeable and must not be silently lowered across kinds.

Invalid lowering:

```sec
spawn thread Work()
```

must not become an ordinary task on a target without physical threads.

Expected diagnostic:

```text
target profile does not support physical threads
```

Invalid lowering:

```sec
spawn process Program()
```

must not become a task or thread.

Expected diagnostic:

```text
target profile does not support process creation
```

The spawn IR must record the requested execution kind:

```text
Task
Thread
Process
```

The backend must not infer execution kind from the called function.

## Common ownership

All returned execution handles are move-only lifecycle owners unless another
rule explicitly defines a weaker observer type.

A spawned execution entity must not be silently forgotten.

Before the owning scope exits, the handle must be:

- awaited where supported
- joined
- detached
- moved to another valid owner
- otherwise consumed by an explicitly valid lifecycle operation

The operand type determines the concrete semantics of:

```sec
join handle
detach handle
```

## Eager scheduling

`spawn` creates an eager task.

After the `spawn` expression completes, the new task is scheduled, running or already completed.

The implementation may begin execution immediately.

The language does not guarantee that the spawned operation executes before the next source statement.

The language does guarantee that the operation has been submitted to the selected task execution mechanism.

## Synchronous calls

Function declarations do not become asynchronous by name or return type.

```sec
let value := CalculateAsync()
```

The suffix `Async` has no language meaning.

Only `spawn` creates a new task:

```sec
let task := spawn CalculateAsync()
```

Sec does not require `async fn` for functions that may be spawned.

The same function may be called synchronously or spawned.

## Current task context

The spawned operation executes with a new current task context.

Inside the spawned operation:

```sec
task.cancelRequested
```

refers to the new task.

Ordinary function calls made by the spawned operation continue inside the same task context unless they use `spawn` again.

A nested `spawn` creates a new child task context.

## Parent and child tasks

The currently executing task becomes the parent of a newly spawned task.

```sec
fn Parent() void {
    let child := spawn Child()
    await child
}
```

The returned `Task[T]` handle is owned by the receiving expression or binding.

Parenthood does not replace ownership.

The owner of the task handle remains responsible for resolving the task.

## Argument evaluation

Spawn arguments are evaluated in normal Sec evaluation order before ownership is transferred to the new task.

The compiler must preserve normal argument evaluation order.

Successfully evaluated argument values are copied, moved or borrowed according to their types and parameter modes.

The backend must not reorder argument evaluation across the task creation boundary when that changes observable behavior.

## Ownership transfer

Arguments passed to owned parameters follow normal copy and move rules.

```sec
fn Process(data: Data) void {
}

let data := Data.Create()
let worker := spawn Process(data)
```

If `Data` is move-only:

- ownership of `data` moves into the task
- the caller may not use `data` afterward
- the task owns the value until it transfers or destroys it

If `Data` is copyable, the argument may be copied according to normal Sec rules.

`spawn` must not silently clone move-only values.

## Borrowed arguments

A spawned task may receive borrowed arguments when the compiler proves that the borrowed value remains valid until the task completes.

```sec
fn Inspect(data: ref Data) void {
}

fn Run() void {
    let data := Data.Create()
    let worker := spawn Inspect(ref data)
    await worker
}
```

The borrow remains active while the task may use it.

The owner may not be destroyed, moved, mutably borrowed in conflict or mutated in conflict until the task has completed.

## Mutable borrows

A `ref mut` argument gives the spawned task exclusive access for the duration of the borrow.

```sec
fn Update(data: ref mut Data) void {
}

let mut data := Data.Create()
let worker := spawn Update(ref mut data)
```

While the task may use the mutable borrow, the parent task must not access `data` directly or create another overlapping borrow.

The compiler must track this borrow until task completion is established.

## Escaping tasks

A task that borrows scope-owned data may not outlive that data.

Invalid:

```sec
fn Invalid() Task[void] {
    let data := Data.Create()
    let worker := spawn Inspect(ref data)
    return worker
}
```

Expected diagnostic:

```text
task worker cannot escape while borrowing local value data
```

The same rule applies to returning, storing, transferring or detaching the task.

## Detached tasks

A detached task may not retain references to scope-owned data.

Invalid:

```sec
fn Invalid() void {
    let data := Data.Create()
    let worker := spawn Inspect(ref data)
    detach worker
}
```

Expected diagnostic:

```text
detached task worker cannot retain reference to local value data
```

A detached task should normally own its required data:

```sec
fn Valid() void {
    let data := Data.Create()
    let worker := spawn Process(data)
    detach worker
}
```

Static or program-owned references still require valid concurrency-safe access.

Static lifetime alone does not make shared mutation safe.

## Captures and callable values

A spawned lambda or callable value follows the normal capture rules.

Captured values are copied or moved into the task according to their types.

A spawned closure may not retain a reference whose lifetime is shorter than the task.

A `MutexGuard[T]` must not be captured, moved or borrowed into another task.

## Spawned methods

Methods may be spawned.

```sec
let worker := spawn service.Run()
```

Receiver handling follows the method signature.

A receiver taken by value is copied or moved.

A read-only `self` receiver creates a shared borrow.

A mutating `self` receiver creates an exclusive mutable borrow.

The receiver must remain valid for the complete task use.

## Task handle ownership

The result of `spawn` is a move-only `Task[T]`.

The task handle must be stored, returned, passed to another owner, awaited, joined or detached explicitly.

Ignoring a spawned task is invalid.

```sec
spawn Work()
```

Expected diagnostic:

```text
spawned task result must be owned, awaited, joined or detached
```

This prevents implicit fire-and-forget execution.

## Spawn in expressions

The initial implementation should permit `spawn` in typed expression contexts.

```sec
let first := spawn First()
let second: Task[int] := spawn Second()
return spawn Worker()
```

Directly awaiting a newly spawned task is valid:

```sec
let value := await spawn Calculate()
```

The compiler may optimize the implementation but must preserve task semantics.

## Spawn in loops

Spawning inside loops is allowed.

Every created task must still have its ownership resolved.

The compiler should diagnose statically evident unbounded task creation, including:

- spawned tasks whose handles are lost
- detached spawn in unbounded loops
- recursive spawn cycles
- exponential child-task creation

Target profiles may impose task-count limits.

## Recursive spawn

A function may spawn itself or participate in an indirect spawn cycle only when the selected profile permits it and task creation is bounded.

Example of statically unbounded creation:

```sec
fn Bomb() void {
    let first := spawn Bomb()
    let second := spawn Bomb()

    await first
    await second
}
```

The compiler should report the spawn cycle and explain why creation is unbounded.

## Scheduling

`spawn` does not select a physical thread.

The target profile may implement spawned tasks using native threads, lightweight threads, worker pools, cooperative scheduling, event loops, RTOS tasks or statically allocated task state machines.

Source-level semantics must remain unchanged.

A target without a configured task execution mechanism must reject `spawn`.

## Blocking behavior

The `spawn` expression may perform the work required to create and schedule a task.

It must not wait for the spawned operation to complete.

Resource exhaustion during task creation must follow the task-creation failure model defined by the concurrency profile.

The compiler must not silently execute the entire operation synchronously merely because task creation is unavailable.

## Cancellation inheritance

A child task may inherit cancellation state or cancellation linkage from its parent according to the concurrency rules.

Creating a child after cancellation has already been requested must have well-defined behavior.

The initial model should allow the child to be created with cancellation already requested.

`spawn` must not silently clear cancellation state.

Detailed cancellation propagation is defined in `concurrency.txt`.

## Static analysis

The compiler must determine for every `spawn`:

- callable target
- concrete return type
- resulting `Task[T]` type
- argument evaluation order
- copied values
- moved values
- retained shared borrows
- retained mutable borrows
- captured values
- parent task
- whether the task may escape
- whether the selected target profile supports task creation

Borrow and ownership effects remain active until completion, transfer or another proven task-lifetime boundary.

## Semantic IR

Semantic IR must represent `spawn` explicitly.

A spawn operation must record at least:

```text
callable
arguments
receiver
result type
Task[T] type
copied values
moved values
retained borrows
captures
parent task
target task profile
source location
```

The backend must not infer ownership or borrowing from low-level loads and stores.

## Diagnostics

Examples:

```text
spawned task result must be owned, awaited, joined or detached
```

```text
task worker cannot escape while borrowing local value data
```

```text
detached task worker cannot retain reference to local value data
```

```text
cannot move value data into task because it is currently borrowed
```

```text
cannot spawn task with MutexGuard[State]; mutex guards are task-bound
```

```text
target profile does not provide task execution
```

```text
unbounded recursive task creation detected: Bomb -> spawn Bomb
```

## Restrictions

`spawn` must not:

- infer asynchronous behavior from a function name
- change the function's declared return type
- silently detach the execution entity
- silently discard the execution handle
- silently clone move-only arguments
- permit invalid references to escape
- transfer a mutex guard across execution-entity boundaries
- bypass ownership or borrowing rules
- silently fall back to synchronous execution

## Related rules

Detailed behavior is defined in:

```text
tasks.txt
await.txt
concurrency.txt
threads.md
processes.txt
mutex.txt
static.txt
concurrency_memory_model.txt
```

## Current implementation status

Implemented:

- parser accepts `spawn CallExpression`
- sema requires the operand to be a call expression
- sema returns `Task[T]` where `T` is the spawned call return type
- function argument copy/move checks reuse ordinary call analysis
- standalone `spawn` expression statements are rejected
- legacy `spawn { ... }` is now a semantic error

Not implemented yet:

- parser and sema support for `spawn task`, `spawn thread` and `spawn process`
- `detach`
- spawn backend/profile validation
- task borrow extension until completion
- escaping-task borrow checks
- recursive spawn-cycle diagnostics
- Semantic IR/MLIR lowering for recorded spawn execution kind
