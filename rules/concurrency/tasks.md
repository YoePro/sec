# Tasks

- **Status:** Normative
- **Created:** 2026-09-04
- **Last updated:** 2026-09-04
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/tasks.md`
- **Replaces:** `rules/concurrency/tasks.txt`
- **Repository baseline reviewed:** `777beb8`
- **Related rulebooks:** `rules/concurrency/spawn.md`, `rules/concurrency/await.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/concurrency_runtime_model.md`, `rules/concurrency/scheduling.md`, `rules/concurrency/structured_concurrency.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/concurrency/threads.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/transferability.md`, `rules/memory/destruction.md`, `rules/errors/panic.md`, `rules/errors/errorhandling.md`, `rules/compiler/semantic_ir.md`, `rules/platform/target_profiles.md`, `rules/control-flow/discard.md`

---

## § 1. Purpose and authority

**Governance tags:** `concurrency.tasks-v2`

§ 1(1) This rulebook defines the language semantics of tasks in Sec.

§ 1(2) A task is a language-level concurrent execution unit represented by the compiler-known generic type:

```sec
Task[T]
```

§ 1(3) `T` is the complete declared return type of the task body.

§ 1(4) A task is not defined as an operating-system thread, process, fiber, coroutine frame, worker-pool job, or event-loop callback.

§ 1(5) A target or runtime may use any implementation strategy that preserves the semantics defined by this rulebook and the related concurrency rulebooks.

§ 1(6) Task syntax, task ownership, task completion, task observation, and task result transfer are language semantics and must not vary according to backend implementation.

§ 1(7) Mutable implementation coverage, frontend status, lowering status, target coverage, and test coverage do not belong in this normative rulebook. They are tracked through governance.

---

## § 2. Core terminology

**Governance tags:** `concurrency.tasks-v2`

§ 2(1) The term **task function** means the callable whose execution forms the body of a task.

§ 2(2) The term **task handle** means the owning `Task[T]` value through which the task lifecycle is controlled.

§ 2(3) The term **task execution outcome** means the terminal result of the task runtime itself, represented by `TaskOutcome[T]`.

§ 2(4) The term **task function result** means the value of type `T` returned normally by the task function.

§ 2(5) A task function result and a task execution outcome are distinct concepts.

§ 2(6) A task may complete normally even when its returned value represents an application-level error.

§ 2(7) A task execution failure is not the same as a returned Sec `Err(...)`.

§ 2(8) A cancellation is not a normal return value.

§ 2(9) A panic is not a normal return value.

§ 2(10) Task creation failure occurs before a `Task[T]` exists and is therefore not a `TaskOutcome[T]`.

---

## § 3. `Task[T]`

**Governance tags:** `concurrency.tasks-v2`, `frontend.transferability`

§ 3(1) `Task[T]` is a compiler-known generic task-handle type.

§ 3(2) The type argument `T` is the complete declared return type of the spawned callable.

Example:

```sec
fn Calculate() int {
    return 42
}

let worker := try spawn Calculate()
```

The type of `worker` is:

```sec
Task[int]
```

§ 3(3) If the task function returns `Result[V, E]`, the task type preserves that complete return type.

Example:

```sec
fn LoadImage() Result[Image, IOError] {
    // ...
}

let worker := try spawn LoadImage()
```

The type of `worker` is:

```sec
Task[Result[Image, IOError]]
```

§ 3(4) `Task[T]` must not erase, flatten, reinterpret, or otherwise transform `T`.

§ 3(5) `Task[Result[V, E]]` is not equivalent to `Result[Task[V], E]`.

§ 3(6) The task handle represents lifecycle responsibility in addition to identifying the concurrent execution.

§ 3(7) The physical runtime representation of `Task[T]` is implementation-defined.

§ 3(8) The physical representation must not change the ownership, result, cancellation, panic, or failure semantics of `Task[T]`.

---

## § 4. Task creation

**Governance tags:** `concurrency.tasks-v2`, `frontend.transferability`

§ 4(1) Task creation is fallible.

§ 4(2) The explicit task creation form:

```sec
spawn task Work()
```

has the raw result type:

```sec
Result[Task[T], TaskSpawnError]
```

when `Work()` returns `T`.

§ 4(3) The promoted default task creation form:

```sec
spawn Work()
```

has the same task-creation semantics and the same raw result type:

```sec
Result[Task[T], TaskSpawnError]
```

§ 4(4) Successful creation produces `Ok(Task[T])`.

§ 4(5) Failed creation produces `Err(TaskSpawnError)` and no `Task[T]` comes into existence.

§ 4(6) Ordinary Sec error handling applies to the creation result.

Typical propagation:

```sec
let worker := try spawn Work()
```

§ 4(7) A program may instead handle `TaskSpawnError` locally using ordinary `Result`, `try`, or `match` rules.

§ 4(8) Task creation must not silently convert a runtime creation failure into a panic.

§ 4(9) A target that does not provide task execution capability at all must be rejected during compilation or target validation rather than represented as a normal runtime `TaskSpawnError`.

§ 4(10) The exact scheduling point after successful creation is governed by `scheduling.md`, but successful task creation establishes a valid owning task handle before the caller can use that handle.

---

## § 5. `TaskSpawnError`

**Governance tags:** `concurrency.tasks-v2`

§ 5(1) `TaskSpawnError` is the error type for failures that prevent a task from being created.

§ 5(2) The canonical variants are:

```sec
enum TaskSpawnError error {
    OutOfMemory
    ResourceLimit
    ExecutorUnavailable
    InvalidConfiguration
    NativeFailure
}
```

§ 5(3) `TaskSpawnError.OutOfMemory` represents failure to obtain memory required to create the task under the selected runtime or target model.

§ 5(4) `TaskSpawnError.ResourceLimit` represents exhaustion of a task, worker, thread, slot, or comparable runtime resource limit.

§ 5(5) `TaskSpawnError.ExecutorUnavailable` represents a configured task executor that cannot accept or create the requested task.

§ 5(6) `TaskSpawnError.InvalidConfiguration` represents a runtime task configuration that is valid as language input but cannot be instantiated under the selected execution configuration.

§ 5(7) `TaskSpawnError.NativeFailure` represents a lower-level platform or runtime creation failure that is not represented more specifically by another variant.

§ 5(8) `TaskSpawnError` must not be used for failures occurring after successful creation of the `Task[T]`.

§ 5(9) `TaskSpawnError` must not be used for a normal `Err(E)` returned by a task function.

§ 5(10) The owning declaration of the core error type and its runtime mapping are governed by the core-library and runtime rulebooks.

---

## § 6. Task-handle ownership

**Governance tags:** `concurrency.tasks-v2`, `frontend.transferability`

§ 6(1) `Task[T]` is move-only for every `T`.

§ 6(2) Copyability of `T` does not make `Task[T]` copyable.

§ 6(3) Copying a task handle would duplicate lifecycle responsibility and is therefore invalid.

§ 6(4) Moving a task handle transfers lifecycle responsibility.

Example:

```sec
let first := try spawn Calculate()
let second :<- first
```

§ 6(5) After the move in § 6(4), `first` is unavailable according to the ordinary ownership rules.

§ 6(6) A reusable move-only source must use the canonical explicit move syntax required by `ownership.md`.

§ 6(7) A newly produced temporary may transfer directly where the ordinary ownership rules permit fresh-value transfer without an explicit source marker.

§ 6(8) Moving a task handle does not create, restart, duplicate, or otherwise alter the underlying task execution.

§ 6(9) The task identity remains the same across ownership transfer of its handle.

§ 6(10) Task-handle ownership is independent of the physical worker on which the task executes.

---

## § 7. Lifecycle responsibility

**Governance tags:** `concurrency.tasks-v2`, `frontend.discard-v2`

§ 7(1) An owning `Task[T]` handle carries a lifecycle obligation.

§ 7(2) A local owning task handle must not leave scope unresolved.

§ 7(3) The lifecycle obligation is resolved by one of the language-defined terminal handle actions, including:

- `await`,
- `join`,
- `detach`,
- explicit transfer of the owning task handle to another owner.

§ 7(4) Moving the handle transfers the lifecycle obligation rather than resolving it.

§ 7(5) Observing a task does not resolve the owning handle's lifecycle obligation.

§ 7(6) Requesting cancellation does not by itself resolve the owning handle's lifecycle obligation.

§ 7(7) The compiler must reject a control-flow path on which an owning local task handle reaches scope exit while still unresolved.

§ 7(8) Lifecycle checking is path-sensitive.

§ 7(9) A handle resolved on one branch but unresolved on another does not satisfy the lifecycle requirement for the merge unless ordinary control-flow analysis proves the unresolved path unreachable.

§ 7(10) Lifecycle responsibility is a compile-time property and must not require runtime ownership tracking.

---

## § 8. Parent and child tasks

**Governance tags:** `concurrency.tasks-v2`

§ 8(1) A task created by another task may have a logical parent-child relationship according to the structured-concurrency rules.

§ 8(2) Parent-child relationship is not defined by operating-system thread identity.

§ 8(3) A task may execute on a different physical worker from its parent.

§ 8(4) A task may migrate between physical workers when the selected scheduler permits migration.

§ 8(5) Structured lifetime requirements, child completion requirements, and propagation policies are governed by `structured_concurrency.md`.

§ 8(6) This rulebook does not imply that detaching a task preserves structured parent-child ownership.

---

## § 9. Current task context

**Governance tags:** `concurrency.tasks-v2`

§ 9(1) Code executing as a task has a current logical task context.

§ 9(2) The current task context is associated with the logical task, not with whichever physical thread or worker is currently executing it.

§ 9(3) Task-local cancellation state, identity, and scheduler-visible task state must follow the logical task across permitted migration.

§ 9(4) Code executing outside a spawned task does not acquire a fictitious task context merely because it runs on a thread that also executes tasks.

§ 9(5) APIs that depend on current-task context must define their behavior when no current task exists.

---

## § 10. Task lifecycle states

**Governance tags:** `concurrency.tasks-v2`

§ 10(1) A task has non-terminal execution states and exactly one terminal execution state.

§ 10(2) The implementation may expose task status through a task-specific status type.

§ 10(3) The exact set of scheduler-internal non-terminal states is not fixed by this rulebook.

§ 10(4) Scheduler-internal distinctions must not change the language-visible terminal outcome model.

§ 10(5) The language-visible terminal categories are:

- completed normally,
- cancelled,
- panicked,
- failed at the task-execution level.

§ 10(6) A task enters at most one terminal category.

§ 10(7) After a task becomes terminal, it must not resume execution.

§ 10(8) Normal return from the task function produces normal completion even when the returned `T` is itself an error-bearing value such as `Result[V, E]`.

§ 10(9) Cancellation, panic, and execution failure must remain distinguishable from normal completion.

---

## § 11. `TaskOutcome[T]`

**Governance tags:** `concurrency.tasks-v2`

§ 11(1) `TaskOutcome[T]` is the canonical value describing how an already-created task terminated.

§ 11(2) Its normative shape is:

```sec
type TaskOutcome[T] union {
    Completed(T)
    Cancelled
    Panicked(PanicInfo)
    Failed(TaskError)
}
```

§ 11(3) Exactly one variant of `TaskOutcome[T]` is active.

§ 11(4) `TaskOutcome[T]` is not a scheduler-state enum.

§ 11(5) Non-terminal states such as ready, queued, scheduled, suspended, or running are not `TaskOutcome[T]` variants.

§ 11(6) `TaskOutcome[T]` is not `Result[T, E]`.

§ 11(7) Ordinary `try` propagation for `Result` must not automatically unwrap `TaskOutcome[T]`.

§ 11(8) The four variants distinguish normal task-function completion from three distinct kinds of abnormal task execution termination.

§ 11(9) The complete task-function return type remains nested inside `Completed`.

§ 11(10) The physical runtime representation of the union is implementation-defined, but the four-way semantic distinction is normative.

---

## § 12. `Completed(T)`

**Governance tags:** `concurrency.tasks-v2`

§ 12(1) `Completed(value)` means that the task function reached normal language-level completion and produced its declared return value.

§ 12(2) `value` has exactly the task type parameter `T`.

§ 12(3) The task runtime must not reinterpret `value`.

§ 12(4) If `T` is `Result[V, E]`, both `Ok(V)` and `Err(E)` are normal task-function return values.

Example:

```sec
fn LoadImage() Result[Image, IOError] {
    // ...
}

let worker := try spawn LoadImage()
let outcome := await worker
```

The type of `outcome` is:

```sec
TaskOutcome[Result[Image, IOError]]
```

A normal application-level error appears as:

```sec
Completed(Err(IOError.InvalidValue))
```

and not as:

```sec
Failed(...)
```

§ 12(5) `Completed(Err(error))` means the task execution mechanism succeeded and the task function normally returned its own error value.

§ 12(6) A task runtime must not promote the inner `Err(E)` into `TaskError`.

§ 12(7) When `T` is move-only, ownership of the completed value is owned by the active `Completed` payload until that payload is transferred according to ordinary union and ownership rules.

§ 12(8) When `T` is copyable, ordinary copy rules govern copying the payload after completion.

§ 12(9) `TaskOutcome[void]` is valid. Its `Completed` case carries the language's `void` success payload semantics rather than changing the outcome type.

---

## § 13. `Cancelled`

**Governance tags:** `concurrency.tasks-v2`

§ 13(1) `Cancelled` means that the task terminated through the language-defined cooperative cancellation mechanism.

§ 13(2) `Cancelled` does not contain a `T` payload.

§ 13(3) Cancellation must not be represented as a fabricated default value of `T`.

§ 13(4) Cancellation must not be represented as `Completed(T)`.

§ 13(5) Cancellation must not be represented as `Failed(TaskError)` merely because the task did not produce `T`.

§ 13(6) Requesting cancellation and observing `Cancelled` are different events.

§ 13(7) A cancellation request does not guarantee immediate task termination.

§ 13(8) Detailed cancellation checkpoints, acknowledgement, shielding, and structured cancellation behavior are governed by `cancellation.md`.

---

## § 14. `Panicked(PanicInfo)`

**Governance tags:** `concurrency.tasks-v2`

§ 14(1) `Panicked(info)` means that the task terminated because a panic escaped the task execution boundary.

§ 14(2) `PanicInfo` is the panic information type governed by `panic.md`.

§ 14(3) Panic information must obey the bounded and target-compatible panic-reporting requirements of `panic.md`.

§ 14(4) A panic is distinct from a returned `Err(E)`.

§ 14(5) A panic is distinct from cooperative cancellation.

§ 14(6) A panic is distinct from `Failed(TaskError)`.

§ 14(7) A task runtime must not silently convert a panic into a normal `Completed` value.

§ 14(8) A target that supports task isolation must preserve enough information to produce the `Panicked(PanicInfo)` outcome required by the selected panic/runtime policy.

§ 14(9) Target-specific hard-abort policies that cannot produce a recoverable task outcome remain governed by `panic.md` and the target profile; this rulebook does not weaken those policies.

---

## § 15. `Failed(TaskError)`

**Governance tags:** `concurrency.tasks-v2`

§ 15(1) `Failed(error)` means that a task which was successfully created later failed at the task-execution or task-runtime level.

§ 15(2) The payload type is `TaskError`.

§ 15(3) `TaskError` is a named Sec error type reserved for failures of an already-created task execution mechanism.

§ 15(4) `TaskError` is deliberately distinct from `TaskSpawnError`.

§ 15(5) `TaskSpawnError` means that no task was created.

§ 15(6) `TaskError` means that a task existed and subsequently reached the execution-failure terminal category.

§ 15(7) `TaskError` is deliberately distinct from the task function's own error type `E` when `T` is `Result[V, E]`.

§ 15(8) This rulebook does not freeze the complete variant inventory of `TaskError`.

§ 15(9) Future task-runtime failure categories may be added under `TaskError` without replacing `TaskOutcome[T]` or creating a separate top-level error type for every execution failure category.

§ 15(10) Names such as `TaskError.ExecutionError` may be used by later core/runtime design when a concrete category is specified, but no such example name in explanatory material constitutes a frozen variant unless added normatively to the owning error definition.

§ 15(11) Codex, compiler implementation work, and governance synchronization must not invent `TaskError` variants merely to complete an implementation.

---

## § 16. Await

**Governance tags:** `concurrency.tasks-v2`

§ 16(1) Awaiting a task uses:

```sec
await worker
```

where `worker` owns `Task[T]`.

§ 16(2) The static type of the await expression is always:

```sec
TaskOutcome[T]
```

§ 16(3) The compiler must not change the static result type of `await Task[T]` to `T` merely because analysis can prove that a particular path completes normally.

§ 16(4) The compiler must not collapse `TaskOutcome[T]` to `Result[T, TaskError]`.

§ 16(5) Await waits or suspends as required until the task reaches a terminal state.

§ 16(6) The physical mechanism may block a native thread, park a worker, suspend a logical task, resume a continuation, or use another target-specific mechanism.

§ 16(7) Those implementation choices must preserve the same language-visible `TaskOutcome[T]`.

§ 16(8) Await consumes the owning task handle and resolves its lifecycle obligation.

§ 16(9) Ownership of any terminal payload transfers into the produced `TaskOutcome[T]`.

§ 16(10) Await does not silently propagate an inner `Result` returned by the task function.

Example:

```sec
let outcome := await worker

match outcome {
    Completed(Ok(image)) => Use(image)
    Completed(Err(error)) => HandleIO(error)
    Cancelled => HandleCancellation()
    Panicked(info) => HandlePanic(info)
    Failed(error) => HandleTaskFailure(error)
}
```

§ 16(11) `await Task[void]` still produces `TaskOutcome[void]`.

§ 16(12) This rulebook does not authorize implicit discarding of an await outcome. Ordinary discard and must-use rules determine whether an unused `TaskOutcome[T]` is legal in a particular context.

---

## § 17. Join

**Governance tags:** `concurrency.tasks-v2`

§ 17(1) `join` synchronizes with task completion while preserving the task handle for terminal inspection.

Conceptual use:

```sec
join worker
```

§ 17(2) A successful join resolves the handle's outstanding join obligation without consuming the handle value itself.

§ 17(3) After join, the task is terminal.

§ 17(4) Joining an already joined task through the same owning lifecycle capability is invalid.

§ 17(5) Joining does not convert abnormal task termination into normal completion.

§ 17(6) Terminal status remains observable after join.

§ 17(7) A completed task value is available only when the terminal state is normal completion.

§ 17(8) Access to a move-only completed value may transfer that stored value at most once.

§ 17(9) Copyable completed values follow their ordinary copy semantics.

§ 17(10) A cancelled, panicked, or failed task has no `T` completion value.

§ 17(11) Join must establish the same completion synchronization edge required for safe visibility of task-produced memory effects.

§ 17(12) Await and join are not interchangeable: await transfers a `TaskOutcome[T]` and consumes the owning handle; join preserves a terminal handle for inspection.

---

## § 18. Identity, name, status, and common handle surface

**Governance tags:** `concurrency.tasks-v2`

§ 18(1) Task identity is represented by a task-specific identity type such as `TaskID`.

§ 18(2) `TaskID` is distinct from `ThreadID` and process identity types.

§ 18(3) The common concurrency-handle surface may expose member names harmonized across tasks, threads, and processes, including:

```text
id
name
status
value
Observe()
platform
```

§ 18(4) Harmonized member names do not imply identical member types or identical execution semantics across execution kinds.

§ 18(5) A task's `status` is a snapshot of execution state, not ownership of the terminal result.

§ 18(6) Reading status must not consume the task handle.

§ 18(7) Reading status must not by itself synchronize and transfer a terminal `TaskOutcome[T]`.

§ 18(8) `value` is meaningful only for normal completion.

§ 18(9) The exact task status type and its non-terminal scheduler states are owned by the task/runtime status rules and must not be inferred from a backend's private scheduler-state machine.

§ 18(10) A task may have a diagnostic or programmer-visible name without making that name its identity.

---

## § 19. Cancellation

**Governance tags:** `concurrency.tasks-v2`

§ 19(1) An owning task handle may request cooperative cancellation according to `cancellation.md`.

Conceptual form:

```sec
worker.RequestCancel()
```

§ 19(2) Requesting cancellation does not consume the handle.

§ 19(3) Repeated cancellation requests are idempotent unless a more specific cancellation API explicitly defines additional behavior.

§ 19(4) A cancellation request does not imply that the task has already terminated.

§ 19(5) A task may continue until it reaches a cancellation observation point.

§ 19(6) If the task terminates through cancellation, its terminal outcome is `Cancelled`.

§ 19(7) If the task ignores or never observes a cancellation request and instead returns normally, its terminal outcome is `Completed(T)`.

§ 19(8) If the task panics after a cancellation request but before cancellation termination, the actual terminal category is determined by the panic/cancellation ordering rules, not by the mere existence of the request.

§ 19(9) Cancellation must not fabricate a `TaskError`.

---

## § 20. Panic and task failure boundaries

**Governance tags:** `concurrency.tasks-v2`

§ 20(1) A task execution boundary must preserve the distinction among:

- normal return,
- cooperative cancellation,
- escaping panic,
- task-runtime execution failure.

§ 20(2) A normal function-level `Err(E)` is part of normal return.

§ 20(3) A panic is represented by `Panicked(PanicInfo)` when the selected target/runtime panic policy permits task-boundary recovery.

§ 20(4) A task-runtime execution failure is represented by `Failed(TaskError)`.

§ 20(5) A runtime must not classify an application-level error by inspecting the contents of `T`.

§ 20(6) A runtime must not inspect a generic `Result` payload to decide whether the task "failed".

§ 20(7) The execution boundary therefore remains valid for any `T`, including nested unions, enums, `Result`, `Option`, and user-defined error-bearing types.

---

## § 21. Task observers

**Governance tags:** `concurrency.tasks-v2`

§ 21(1) A task may provide a non-owning observation capability through `Observe()`.

§ 21(2) The task observer is conceptually a `TaskObserver[T]` or compiler-equivalent non-owning task observation type.

§ 21(3) Creating an observer does not duplicate the owning `Task[T]` handle.

§ 21(4) An observer must not acquire lifecycle responsibility.

§ 21(5) An observer may inspect permitted metadata such as identity, name, platform information, and status.

§ 21(6) An observer may participate in observation-oriented synchronization primitives such as `select` when those rules permit it.

§ 21(7) An observer must not:

- await and consume the owning task result,
- join as the lifecycle owner,
- detach the task,
- transfer or take a move-only completion value,
- request operations reserved to the owning lifecycle capability unless a separate rule explicitly grants them.

§ 21(8) Destroying an observer does not affect task execution.

§ 21(9) Observer lifetime must not create an owning reference cycle requiring garbage collection.

§ 21(10) Runtime retention required solely for observation must be bounded or explicitly owned according to the concurrency runtime model.

---

## § 22. Detach

**Governance tags:** `concurrency.tasks-v2`, `frontend.discard-v2`

§ 22(1) Detaching transfers the task out of the current structured lifecycle obligation.

§ 22(2) Plain detach is valid only when the task's normal completion value does not require explicit result disposal.

Canonical form for a void task:

```sec
detach worker
```

§ 22(3) A task whose `T` carries a value requires explicit discard intent when detaching without retaining its completion value.

Canonical form:

```sec
detach worker discard
```

§ 22(4) Explicit discard applies to the detached task's eventual result/outcome observation obligation; it does not retroactively discard a `TaskSpawnError`.

§ 22(5) Spawn failure must already have been handled before a valid `Task[T]` exists to detach.

§ 22(6) Detaching `Task[Result[V, E]]` with explicit discard means that the caller intentionally relinquishes observation of both the normal returned `Result[V, E]` and the task's terminal execution outcome.

§ 22(7) Detach consumes the owning task handle.

§ 22(8) After detach, the previous owner must not use the handle.

§ 22(9) Detach must not silently preserve borrowed state whose lifetime cannot outlive the detaching scope.

§ 22(10) The runtime must retain whatever internal state is required for the detached task to finish safely without retaining the consumed source-level handle as an owner.

---

## § 23. Task arguments and ownership transfer

**Governance tags:** `concurrency.tasks-v2`, `frontend.transferability`, `analysis.transferability`

§ 23(1) Values passed across a task boundary obey ordinary ownership plus the additional transferability rules.

§ 23(2) Passing an owned move-only value into a spawned task transfers exclusive ownership when the task parameter consumes that value.

§ 23(3) Reusable move-only source bindings use the canonical explicit move marker required by `ownership.md`.

Conceptual example:

```sec
let resource := OpenResource()
let worker := try spawn Use(<-resource)
```

§ 23(4) After committed ownership transfer, the original binding is unavailable.

§ 23(5) Copyable values may cross the boundary by copy when ordinary parameter and transferability rules permit it.

§ 23(6) The compiler must not silently clone a move-only resource for task creation.

§ 23(7) Moving exclusive ownership into another task does not require the source value to satisfy simultaneous shared-access safety merely because execution becomes concurrent.

§ 23(8) Borrowed references crossing a task boundary require proof of lifetime validity, alias compatibility, address stability, and any migration restrictions required by `transferability.md`.

§ 23(9) A detached task must not retain a reference whose validity depends on a scope that may end before the task.

§ 23(10) This rulebook does not redefine the ordinary call-boundary transfer commit point.

§ 23(11) Ownership behavior when evaluation of a spawn expression fails before task creation is governed jointly by `spawn.md`, `ownership.md`, and `transferability.md`.

§ 23(12) No implementation may invent a different source-level ownership rule for failed spawn merely to simplify a runtime API.

---

## § 24. Task captures and closures

**Governance tags:** `concurrency.tasks-v2`, `frontend.transferability`, `analysis.transferability`

§ 24(1) A spawned lambda or closure crosses a task execution boundary.

§ 24(2) Captured values therefore obey both closure-capture rules and task transferability rules.

§ 24(3) Capturing a move-only value by ownership makes that value unavailable to the source context once transfer commits.

§ 24(4) Capturing by shared or mutable reference requires the same proof obligations as any other borrow crossing a task boundary.

§ 24(5) A task runtime must not extend an invalid source borrow by copying a reference into a longer-lived task context.

§ 24(6) Detached closures require capture lifetimes that remain valid independently of the detaching source scope.

---

## § 25. Scheduling and migration

**Governance tags:** `concurrency.tasks-v2`, `platform.transferability`

§ 25(1) A successfully created task becomes eligible for execution according to `scheduling.md`.

§ 25(2) Task scheduling is a property of the logical task runtime and selected target profile.

§ 25(3) A task is not permanently bound to the physical worker that first executes it unless an explicit target/runtime rule establishes such a restriction.

§ 25(4) A migratable task may resume on another physical worker.

§ 25(5) Task identity, cancellation context, owned values, and language-visible task state must survive permitted migration.

§ 25(6) A backend may implement tasks using:

- a cooperative executor,
- a worker pool,
- fibers,
- an RTOS task facility,
- native threads,
- an event loop,
- statically allocated task slots,
- another target-defined mechanism.

§ 25(7) Backend selection must not alter the type of `spawn`, `Task[T]`, or `await Task[T]`.

§ 25(8) A backend must not expose private scheduler states as portable language semantics unless those states are separately standardized.

---

## § 26. Memory visibility and synchronization

**Governance tags:** `concurrency.tasks-v2`, `analysis.transferability`, `lowering.transferability`

§ 26(1) Task creation, execution, join, await, cancellation, and terminal publication must obey the concurrency memory model.

§ 26(2) Ownership transfer does not by itself authorize a data race.

§ 26(3) Borrowed or shared state remains subject to the concurrency memory model and transferability analysis.

§ 26(4) Completion synchronization through join or await must make task-produced memory effects visible according to `concurrency_memory_model.md`.

§ 26(5) A runtime implementation must not reorder terminal outcome publication in a way that allows the caller to observe terminal completion without the memory effects guaranteed by the task completion edge.

§ 26(6) Task migration must preserve these synchronization guarantees.

---

## § 27. Recursive spawning and resource analysis

**Governance tags:** `concurrency.tasks-v2`, `analysis.transferability`

§ 27(1) Recursive or dynamically repeated task creation is permitted only where the selected target/profile permits the resulting resource behavior.

§ 27(2) A compiler analysis may prove a static upper bound for task creation.

§ 27(3) A target profile may require such a bound.

§ 27(4) A target profile may reject task creation patterns that cannot satisfy its static resource policy.

§ 27(5) A hosted profile may permit dynamically bounded task populations and report runtime resource exhaustion through `TaskSpawnError`.

§ 27(6) Compile-time rejection of an unsupported resource model and runtime `TaskSpawnError.ResourceLimit` are distinct mechanisms.

§ 27(7) The compiler must not assume unbounded task resources merely because the host used to build the compiler provides them.

---

## § 28. Semantic IR requirements

**Governance tags:** `concurrency.tasks-v2`, `semantic-ir.transferability`, `analysis.semantic-ir-v2`

§ 28(1) Semantic IR must preserve task semantics explicitly enough that lowering does not need to reconstruct them from backend instructions.

§ 28(2) Semantic IR must distinguish at least:

- fallible task creation,
- the owning `Task[T]` handle,
- ownership transfer of task arguments and captures,
- task observation,
- cancellation request,
- join,
- await,
- detach,
- terminal normal completion,
- terminal cancellation,
- terminal panic,
- terminal execution failure,
- transfer of terminal payload ownership.

§ 28(3) Semantic IR must preserve the complete type parameter `T`.

§ 28(4) Semantic IR for await must produce or transfer `TaskOutcome[T]`, not a backend-dependent direct `T`.

§ 28(5) Semantic IR must preserve the distinction between:

```sec
Completed(Err(error))
```

and:

```sec
Failed(taskError)
```

§ 28(6) Semantic IR must preserve the distinction between `TaskSpawnError` and `TaskError`.

§ 28(7) Semantic IR must carry sufficient ownership facts to prevent duplicate destruction of moved task arguments, task handles, and terminal payloads.

§ 28(8) Semantic IR must carry sufficient transferability facts for target validation and lowering.

§ 28(9) Concrete Semantic IR operation names are owned by `semantic_ir.md`; this rulebook defines required semantics rather than inventing a competing operation vocabulary.

---

## § 29. Lowering and runtime requirements

**Governance tags:** `concurrency.tasks-v2`, `lowering.transferability`, `platform.transferability`

§ 29(1) Lowering may choose target-specific runtime mechanisms only after semantic validation has established a valid task operation.

§ 29(2) Lowering must not convert fallible spawn into an infallible primitive at the language boundary.

§ 29(3) Lowering must preserve `TaskOutcome[T]` as the four-way terminal semantic outcome even when the runtime internally uses status codes, tagged records, callbacks, futures, or platform handles.

§ 29(4) Lowering may optimize away representation overhead when it proves the observable semantics unchanged.

§ 29(5) Such optimization must not change the static Sec type of an await expression.

§ 29(6) A proof that a task cannot cancel, panic, or fail may optimize branches or representation, but does not change:

```sec
await Task[T] -> TaskOutcome[T]
```

§ 29(7) A runtime must not require garbage collection solely to implement task lifecycle ownership.

§ 29(8) A target that forbids dynamic allocation must use a compatible task-runtime strategy or reject configurations that cannot satisfy the task semantics.

§ 29(9) Hidden host facts must not substitute for selected target/profile facts during task lowering.

---

## § 30. Diagnostics

**Governance tags:** `concurrency.tasks-v2`, `tooling.transferability`, `frontend.discard-v2`

§ 30(1) Task diagnostics should state which task rule was violated rather than reporting only a low-level type or runtime implementation failure.

§ 30(2) An unresolved owning task handle at scope exit should identify the handle and suggest a valid lifecycle action when one is applicable.

§ 30(3) Attempted copying of `Task[T]` should state that the task handle is move-only and owns lifecycle responsibility.

§ 30(4) Use after moving a task handle should identify the ownership transfer.

§ 30(5) Invalid detach of a value-returning task should explain the explicit discard form when discard is permitted.

§ 30(6) Invalid cross-task references should identify the lifetime, mutability, transferability, migration, or address-stability reason.

§ 30(7) A target without task capability should produce a target/capability diagnostic rather than fabricating a runtime `TaskSpawnError`.

§ 30(8) Diagnostics involving nested task results should preserve both layers.

For example, tooling should distinguish:

```text
task completed normally with Result[Image, IOError]
```

from:

```text
task execution failed with TaskError
```

§ 30(9) Tooling must not describe a normal `Completed(Err(E))` as a task runtime failure.

---

## § 31. Restrictions

**Governance tags:** `concurrency.tasks-v2`

§ 31(1) `Task[T]` must not be implicitly copied.

§ 31(2) `await Task[T]` must not have flow-dependent static result type.

§ 31(3) `TaskOutcome[T]` must not be silently flattened into `T`.

§ 31(4) `TaskOutcome[Result[V, E]]` must not be silently flattened into `Result[V, E]`.

§ 31(5) A returned `Err(E)` must not be reclassified as `Failed(TaskError)`.

§ 31(6) A failed task creation must not produce a fake `Task[T]`.

§ 31(7) Cancellation must not fabricate a completion value.

§ 31(8) Panic must not be silently reclassified as cancellation.

§ 31(9) Task execution failure must not be silently reclassified as application-level `Err(E)`.

§ 31(10) An observer must not become a second lifecycle owner.

§ 31(11) A detached task must not retain invalid borrowed state.

§ 31(12) Backend scheduler implementation details must not redefine portable task semantics.

---

## § 32. Ownership of adjacent rules

**Governance tags:** `concurrency.tasks-v2`

§ 32(1) `spawn.md` owns detailed spawn grammar, callable forms, spawn-argument evaluation, and task/thread/process spawn selection.

§ 32(2) `await.md` owns general await syntax and awaitable-expression rules while this book owns the task-specific result type of awaiting `Task[T]`.

§ 32(3) `cancellation.md` owns cancellation request, observation, checkpoints, shielding, and propagation.

§ 32(4) `panic.md` owns panic semantics and `PanicInfo`.

§ 32(5) `scheduling.md` owns scheduling policy and scheduler-visible execution behavior.

§ 32(6) `structured_concurrency.md` owns structured parent-child lifecycle rules.

§ 32(7) `concurrency_memory_model.md` owns memory-order and synchronization semantics.

§ 32(8) `ownership.md` owns move syntax, availability, and ownership transfer.

§ 32(9) `borrowing.md` owns borrow compatibility and borrow lifetime.

§ 32(10) `transferability.md` owns cross-execution transfer proofs.

§ 32(11) `discard.md` owns general discardability and must-use semantics.

§ 32(12) `semantic_ir.md` owns concrete Semantic IR representation and operation naming.

§ 32(13) `target_profiles.md` owns selected target capabilities and restrictions.

---

## § 33. Governance

**Governance tags:** `concurrency.tasks-v2`, `frontend.discard-v2`, `frontend.transferability`, `analysis.transferability`, `semantic-ir.transferability`, `lowering.transferability`, `platform.transferability`, `tooling.transferability`, `analysis.semantic-ir-v2`

§ 33(1) Mutable implementation information for this rulebook must be maintained in `implementation-status.yaml`.

§ 33(2) The governance integration for this revision is `concurrency.tasks-v2`.

§ 33(3) Existing transferability and discard integrations must reference `rules/concurrency/tasks.md` rather than the replaced `rules/concurrency/tasks.txt`.

§ 33(4) Governance may record parser, semantic-analysis, Semantic IR, lowering, runtime, platform, tooling, and test coverage independently.

§ 33(5) Governance status must not weaken or redefine a normative clause in this rulebook.

§ 33(6) A feature recorded as unimplemented or partial remains normative unless the language rulebook explicitly marks the language feature itself as deferred.

§ 33(7) Implementation milestones must not substitute an older task model for the semantics in this revision.

§ 33(8) Corrections required in adjacent rulebooks by this revision are tracked separately through the corrections workflow.
