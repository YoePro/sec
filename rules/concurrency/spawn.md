# Spawn

- **Status:** Normative
- **Created:** 2026-07-26
- **Last updated:** 2026-11-07 (stable chapter and paragraph numbering)
- **Document revision:** 1
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/spawn.md`
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`
- **Related governance:** `governance/concurrency_task.yaml`, `governance/concurrency_thread.yaml`, `governance/concurrency_model.yaml` (`concurrency.model-v2` umbrella)
- **Related rulebooks:** `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/concurrency/await.md`, `rules/concurrency/blocking.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/scheduling.md`, `rules/concurrency/concurrency_runtime_model.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/analysis/escape_analysis.md`, `rules/compiler/semantic_ir.md`, `rules/platform/ffi.md`

---

## § 1 Purpose

**§ 1(1)** `spawn` starts an operation as a new execution entity.

**§ 1(2)** It is the explicit boundary between synchronous execution and concurrent
execution.

**§ 1(3)** A normal function call remains synchronous.

```sec
let value := Calculate()
```

**§ 1(4)** A spawned function call creates a task by default.

```sec
let task := try spawn Calculate()
```

**§ 1(5)** This is shorthand for:

```sec
let task := try spawn task Calculate()
```

## § 2 Syntax

```text
spawn-expression:
    spawn expression
    spawn task expression
    spawn thread expression
    spawn process expression
```

**§ 2(1)** Typical use:

```sec
let worker := try spawn Work()
let taskWorker := try spawn task Work()
let threadWorker := try spawn thread Work()
let childProcess := try spawn process Program()
```

**§ 2(2)** The initial implementation requires the operand of `spawn` to be a callable expression.

**§ 2(3)** Examples:

```sec
spawn Work()
spawn Process(value)
spawn service.Run()
spawn callback(value)
```

**§ 2(4)** `task`, `thread` and `process` are contextual spawn modifiers only when they
occur directly after `spawn`.

**§ 2(5)** They do not need to be general-purpose reserved keywords.

**§ 2(6)** Example:

```sec
let thread := "worker"
```

**§ 2(7)** may remain valid when `thread` is not reserved elsewhere.

**§ 2(8)** `spawn` is an expression and may be used wherever its resulting execution handle
is valid.

## § 3 Result type

**§ 3(1)** If the spawned operation returns `T`, these forms have the following conceptual
result types:

```sec
spawn Work()          // Result[Task[T], TaskSpawnError]
spawn task Work()     // Result[Task[T], TaskSpawnError]
spawn thread Work()   // Result[Thread[T], ThreadSpawnError]
spawn process Work()  // Result[Process[T], ProcessSpawnError]
```

**§ 3(2)** `spawn expression` is exactly equivalent to `spawn task expression`.

**§ 3(3)** For every execution kind, `T` is the callable's complete declared normal return
type. A callable returning `Result[U, E]` therefore produces
`Process[Result[U, E]]`; process-creation failure is the outer
`ProcessSpawnError` and is never flattened with the callable result.

**§ 3(4)** Example:

```sec
fn Calculate() int {
    return 42
}

let calculation := try spawn Calculate()
```

**§ 3(5)** `calculation` has type `Task[int]`; the raw spawn expression has type
`Result[Task[int], TaskSpawnError]`.

**§ 3(6)** A function returning `void` through `spawn` or `spawn task` produces
`Result[Task[void], TaskSpawnError]`.

**§ 3(7)** A fallible function preserves its complete return type:

```sec
fn Load() Result[Image, IOError] {
}

let loading := try spawn Load()
```

**§ 3(8)** The successfully created task type is:

```sec
Task[Result[Image, IOError]]
```

**§ 3(9)** `spawn` does not unwrap or alter the function return type.

**§ 3(10)** Task creation failure occurs before a `Task[T]` exists and remains distinct
from every outcome of the successfully created task.

**§ 3(11)** `Task[T]`, `Thread[T]`, and `Process[T]` are distinct handle types.

**§ 3(12)** They are not interchangeable and must not be silently lowered across kinds.

**§ 3(13)** Invalid lowering:

```sec
spawn thread Work()
```

**§ 3(14)** must not become an ordinary task on a target without physical threads.

**§ 3(15)** Expected diagnostic:

```text
target profile does not support physical threads
```

## § 4 Thread configuration

**§ 4(1)** Thread configuration is owned by `threads.md`. A named `ThreadConfig` is
move-only and is consumed with the ordinary move marker:

```sec
let worker := try spawn thread <-config Work()
```

**§ 4(2)** A fresh inline configuration needs no synthetic move marker and uses the exact
CamelCase fields and explicit `ThreadSetting[T]` variants:

```sec
let worker := try spawn thread {
    Name: Preferred("worker")
    Stack: Required(65536)
    Affinity: Preferred(CpuSet { 2, 3 })
    Priority: Preferred(ThreadPriority.High)
    Start: ThreadStartMode.Deferred
} Work()
```

**§ 4(3)** Omitted fields use `ThreadConfig.init()` defaults. Plain values do not
implicitly become `Preferred(...)`.

**§ 4(4)** Invalid lowering:

```sec
spawn process Program()
```

**§ 4(5)** must not become a task or thread.

**§ 4(6)** Expected diagnostic:

```text
target profile does not support process execution
```

**§ 4(7)** The spawn IR must record the requested execution kind:

```text
Task
Thread
Process
```

**§ 4(8)** The backend must not infer execution kind from the called function.

**§ 4(9)** For `Process`, Semantic IR additionally preserves the concrete callable entry,
`Process[T]` and `ProcessSpawnError` result identities, transfer adapters for
arguments/captures/receiver, startup prepare/commit/rollback facts, and source
provenance. It must not retain ordinary cross-process borrow facts or lower the
operation as task/thread execution.

## § 5 Common ownership

**§ 5(1)** All returned execution handles are move-only lifecycle owners unless another
rule explicitly defines a weaker observer type.

**§ 5(2)** A spawned execution entity must not be silently forgotten.

**§ 5(3)** Before the owning scope exits, the handle must be:

- awaited where supported
- joined
- detached
- moved to another valid owner
- otherwise consumed by an explicitly valid lifecycle operation

**§ 5(4)** The operand type determines the concrete semantics of:

```sec
join handle
detach handle
```

## § 6 Eager scheduling

**§ 6(1)** `spawn` creates the selected execution kind eagerly.

**§ 6(2)** After the `spawn` expression completes, the new task is scheduled, running or already completed.

**§ 6(3)** The implementation may begin execution immediately.

**§ 6(4)** The language does not guarantee that the spawned operation executes before the next source statement.

**§ 6(5)** The language does guarantee that the operation has been submitted to the selected
execution mechanism. In particular, successful `spawn process` establishes the
process identity, child bootstrap, startup-transfer commit, callable entry,
completion transport, and lifecycle owner defined by `processes.md`. It never
exposes an unstarted `Process[T]`; there is no `ProcessConfig` or
`Process.Start()` surface.

## § 7 Synchronous calls

**§ 7(1)** Function declarations do not become asynchronous by name or return type.

```sec
let value := CalculateAsync()
```

**§ 7(2)** The suffix `Async` has no language meaning.

**§ 7(3)** Only `spawn` creates a new task:

```sec
let task := try spawn CalculateAsync()
```

**§ 7(4)** Sec does not require `async fn` for functions that may be spawned.

**§ 7(5)** The same function may be called synchronously or spawned.

## § 8 Current task context

**§ 8(1)** The spawned operation executes with a new current task context.

**§ 8(2)** Inside the spawned operation:

```sec
Task.Current().CancelRequested
```

**§ 8(3)** refers to the new task.

**§ 8(4)** Ordinary function calls made by the spawned operation continue inside the same task context unless they use `spawn` again.

**§ 8(5)** A nested `spawn` creates a new child task context.

## § 9 Parent and child tasks

**§ 9(1)** The currently executing task becomes the parent of a newly spawned task.

```sec
fn Parent() void {
    let child := try spawn Child()
    let outcome := await child
    Handle(outcome)
}
```

**§ 9(2)** The returned `Task[T]` handle is owned by the receiving expression or binding.

**§ 9(3)** Parenthood does not replace ownership.

**§ 9(4)** The owner of the task handle remains responsible for resolving the task.

## § 10 Argument evaluation

**§ 10(1)** Spawn arguments are evaluated in normal Sec evaluation order before ownership is transferred to the new task.

**§ 10(2)** The compiler must preserve normal argument evaluation order.

**§ 10(3)** Successfully evaluated argument values are copied, moved or borrowed according to their types and parameter modes.

**§ 10(4)** The backend must not reorder argument evaluation across the task creation boundary when that changes observable behavior.

**§ 10(5)** For `spawn process`, arguments, captures, and instance-method receiver state
must satisfy the canonical `ProcessTransferable` proof or adapter from
`rules/memory/transferability.md`. Ordinary `ref T` and `ref mut T` never
cross the process boundary, and a numeric `RawPtr[T]` does not become a
child-valid pointer merely by transfer. Transferred receiver state becomes
child-local; method code is compiler/linker materialization rather than runtime
object payload.

**§ 10(6)** An existing named binding consumed by process startup requires the ordinary
`<-` marker; fresh temporaries require no synthetic marker. Startup is
transactional: on `Ok(Process[T])` the transfer commits and the source becomes
unavailable, while on `Err(ProcessSpawnError)` it does not commit and the
source remains owned for normal recovery or cleanup.

## § 11 Ownership transfer

**§ 11(1)** Arguments passed to owned parameters follow normal copy and move rules.

```sec
fn Process(data: Data) void {
}

let data := Data.Create()
let worker := try spawn Process(<-data)
```

**§ 11(2)** If `Data` is move-only:

- ownership of `data` moves into the task
- the caller may not use `data` afterward
- the task owns the value until it transfers or destroys it

**§ 11(3)** If `Data` is copyable, the argument may be copied according to normal Sec rules.

**§ 11(4)** `spawn` must not silently clone move-only values.

## § 12 Borrowed arguments

**§ 12(1)** A spawned task may receive borrowed arguments when the compiler proves that the borrowed value remains valid until the task completes.

```sec
fn Inspect(data: ref Data) void {
}

fn Run() void {
    let data := Data.Create()
    let worker := try spawn Inspect(ref data)
    let outcome := await worker
    Handle(outcome)
}
```

**§ 12(2)** The borrow remains active while the task may use it.

**§ 12(3)** The owner may not be destroyed, moved, mutably borrowed in conflict or mutated in conflict until the task has completed.

## § 13 Mutable borrows

**§ 13(1)** A `ref mut` argument gives the spawned task exclusive access for the duration of the borrow.

```sec
fn Update(data: ref mut Data) void {
}

let mut data := Data.Create()
let worker := try spawn Update(ref mut data)
```

**§ 13(2)** While the task may use the mutable borrow, the parent task must not access `data` directly or create another overlapping borrow.

**§ 13(3)** The compiler must track this borrow until task completion is established.

## § 14 Escaping tasks

**§ 14(1)** A task that borrows scope-owned data may not outlive that data.

**§ 14(2)** Invalid:

```sec
fn Invalid() Task[void] {
    let data := Data.Create()
    let worker := try spawn Inspect(ref data)
    return worker
}
```

**§ 14(3)** Expected diagnostic:

```text
task worker cannot escape while borrowing local value data
```

**§ 14(4)** The same rule applies to returning, storing, transferring or detaching the task.

## § 15 Detached tasks

**§ 15(1)** A detached task may not retain references to scope-owned data.

**§ 15(2)** Invalid:

```sec
fn Invalid() void {
    let data := Data.Create()
    let worker := try spawn Inspect(ref data)
    detach worker
}
```

**§ 15(3)** Expected diagnostic:

```text
detached task worker cannot retain reference to local value data
```

**§ 15(4)** A detached task should normally own its required data:

```sec
fn Valid() void {
    let data := Data.Create()
    let worker := try spawn Process(<-data)
    detach worker
}
```

**§ 15(5)** Static or program-owned references still require valid concurrency-safe access.

**§ 15(6)** Static lifetime alone does not make shared mutation safe.

## § 16 Captures and callable values

**§ 16(1)** A spawned lambda or callable value follows the normal capture rules.

**§ 16(2)** Captured values are copied or moved into the task according to their types.

**§ 16(3)** A spawned closure may not retain a reference whose lifetime is shorter than the task.

**§ 16(4)** A `MutexGuard[T]` must not be captured, moved or borrowed into another task.

## § 17 Spawned methods

**§ 17(1)** Methods may be spawned.

```sec
let worker := try spawn service.Run()
```

**§ 17(2)** Receiver handling follows the method signature.

**§ 17(3)** A receiver taken by value is copied or moved.

**§ 17(4)** A read-only `self` receiver creates a shared borrow.

**§ 17(5)** A mutating `self` receiver creates an exclusive mutable borrow.

**§ 17(6)** The receiver must remain valid for the complete task use.

## § 18 Task handle ownership

**§ 18(1)** The raw result of task `spawn` is
`Result[Task[T], TaskSpawnError]`. Its successful payload is a move-only
`Task[T]` lifecycle owner.

**§ 18(2)** The task handle must be stored, returned, passed to another owner, awaited, joined or detached explicitly.

**§ 18(3)** Ignoring a spawned task is invalid.

```sec
spawn Work()
```

**§ 18(4)** Expected diagnostic:

```text
spawned task result must be owned, awaited, joined or detached
```

**§ 18(5)** This prevents implicit fire-and-forget execution.

## § 19 Spawn in expressions

**§ 19(1)** `spawn` is valid in typed expression contexts when its fallible creation result
is handled by the surrounding expression or return contract.

```sec
let first := try spawn First()
let second: Task[int] := try spawn Second()
return spawn Worker() // enclosing return type includes TaskSpawnError
```

**§ 19(2)** Directly awaiting a newly spawned task is valid:

```sec
let task := try spawn Calculate()
let outcome := await task
```

**§ 19(3)** The compiler may optimize the implementation but must preserve task semantics.

## § 20 Spawn in loops

**§ 20(1)** Spawning inside loops is allowed.

**§ 20(2)** Every created task must still have its ownership resolved.

**§ 20(3)** The compiler should diagnose statically evident unbounded task creation, including:

- spawned tasks whose handles are lost
- detached spawn in unbounded loops
- recursive spawn cycles
- exponential child-task creation

**§ 20(4)** Target profiles may impose task-count limits.

## § 21 Recursive spawn

**§ 21(1)** A function may spawn itself or participate in an indirect spawn cycle only when the selected profile permits it and task creation is bounded.

**§ 21(2)** Example of statically unbounded creation:

```sec
fn Bomb() void {
    let first := try spawn Bomb()
    let second := try spawn Bomb()

    await first
    await second
}
```

**§ 21(3)** The compiler should report the spawn cycle and explain why creation is unbounded.

## § 22 Scheduling

**§ 22(1)** `spawn` does not select a physical thread.

**§ 22(2)** The target profile may implement spawned tasks using native threads, lightweight threads, worker pools, cooperative scheduling, event loops, RTOS tasks or statically allocated task state machines.

**§ 22(3)** Source-level semantics must remain unchanged.

**§ 22(4)** A target without a configured task execution mechanism must reject `spawn`.

## § 23 Blocking behavior

**§ 23(1)** The `spawn` expression may perform the work required to create and schedule a task.

**§ 23(2)** It must not wait for the spawned operation to complete.

**§ 23(3)** Resource exhaustion during task creation must follow the task-creation failure model defined by the concurrency profile.

**§ 23(4)** The compiler must not silently execute the entire operation synchronously merely because task creation is unavailable.

## § 24 Cancellation inheritance

**§ 24(1)** A child task may inherit cancellation state or cancellation linkage from its parent according to the concurrency rules.

**§ 24(2)** Creating a child after cancellation has already been requested must have well-defined behavior.

**§ 24(3)** The initial model should allow the child to be created with cancellation already requested.

**§ 24(4)** `spawn` must not silently clear cancellation state.

**§ 24(5)** Detailed cancellation propagation is defined in `concurrency.txt`.

## § 25 Static analysis

**§ 25(1)** The compiler must determine for every `spawn`:

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

**§ 25(2)** Borrow and ownership effects remain active until completion, transfer or another proven task-lifetime boundary.

## § 26 Semantic IR

**§ 26(1)** Semantic IR must represent `spawn` explicitly.

**§ 26(2)** A spawn operation must record at least:

```text
callable
arguments
receiver
result type
fallible task-creation result
TaskSpawnError before handle existence
Task[T] type
copied values
moved values
retained borrows
captures
parent task
target task profile
source location
```

**§ 26(3)** The backend must not infer ownership or borrowing from low-level loads and stores.

## § 27 Diagnostics

**§ 27(1)** Examples:

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
target profile does not support process execution
```

```text
process spawn consumes 'payload'; write <-payload to transfer ownership
```

```text
value of type 'T' cannot cross the process boundary
```

```text
unbounded recursive task creation detected: Bomb -> spawn Bomb
```

## § 28 Restrictions

**§ 28(1)** `spawn` must not:

- infer asynchronous behavior from a function name
- change the function's declared return type
- silently detach the execution entity
- silently discard the execution handle
- silently clone move-only arguments
- permit invalid references to escape
- transfer a mutex guard across execution-entity boundaries
- bypass ownership or borrowing rules
- silently fall back to synchronous execution

## § 29 Related rules

**§ 29(1)** Detailed behavior is defined in:

```text
tasks.md
await.txt
concurrency.txt
threads.md
processes.md
mutex.md
static.txt
concurrency_memory_model.md
```
