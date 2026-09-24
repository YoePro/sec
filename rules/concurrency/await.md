# Await

- **Status:** Normative
- **Created:** 2026-09-16
- **Last updated:** 2026-09-24
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/await.md`
- **Replaces:** Earlier legacy revision at the same canonical path
- **Repository baseline reviewed:** `main-reviewed-2026-09-16`
- **Implementation governance:** `governance/concurrency_await.yaml`
- **Related rulebooks:** `rules/concurrency/tasks.md`, `rules/concurrency/spawn.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/blocking.md`, `rules/concurrency/structured_concurrency.md`, `rules/concurrency/scheduling.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/concurrency/mutex.md`, `rules/concurrency/select.md`, `rules/errors/panic.md`, `rules/errors/errorhandling.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/destruction.md`, `rules/compiler/semantic_ir.md`, `rules/platform/target_profiles.md`, `rules/tooling/lsp.md`

---

## § 1. Purpose and authority

**Governance tags:** `concurrency.await-v2`

§ 1(1) This rulebook defines the Sec 0.1 semantics of the `await` expression for owning `Task[T]` handles.

§ 1(2) `await` is the normal consuming synchronization operation that resolves one owned task lifecycle into one `TaskOutcome[T]`.

§ 1(3) This rulebook owns:

- `await` operand requirements;
- invariant result typing;
- task-handle consumption;
- await commit semantics;
- caller-cancellation versus task-completion races;
- lifecycle cleanup after caller cancellation;
- result ownership transfer;
- suspension/blocking behavior;
- await-specific borrow and guard restrictions;
- await-specific Semantic IR, lowering, diagnostics, and tooling obligations.

§ 1(4) `tasks.md` remains the canonical owner of `Task[T]`, `TaskOutcome[T]`, task lifecycle, `TaskError`, task creation, task status, detach, join, and observers.

§ 1(5) `cancellation.md` remains the canonical owner of cancellation requests, current-execution cancellation, and `Context`.

§ 1(6) `panic.md` remains the canonical owner of `PanicInfo` and panic containment policy.

§ 1(7) Mutable implementation status belongs in `governance/concurrency_await.yaml`, not in this normative rulebook.

---

## § 2. Core rule

**Governance tags:** `concurrency.await-v2`, `concurrency.tasks-v2`

§ 2(1) If an expression `task` has type:

```sec
Task[T]
```

then:

```sec
await task
```

has the invariant static type:

```sec
TaskOutcome[T]
```

§ 2(2) This type does not collapse to `T`, `Result[T, TaskError]`, or another narrower type merely because static analysis proves normal completion.

§ 2(3) `await` consumes the owning `Task[T]` handle.

§ 2(4) An owning `Task[T]` can be consumed by `await` at most once.

§ 2(5) `await` does not create a new task.

§ 2(6) `await` does not detach a task.

---

## § 3. Syntax

**Governance tags:** `concurrency.await-v2`, `frontend.await-v2`

§ 3(1) The canonical expression form is:

```sec
await expression
```

§ 3(2) The operand expression must produce one available owning `Task[T]`.

§ 3(3) Example:

```sec
let worker := try spawn Calculate()
let outcome := await worker
```

§ 3(4) Direct awaiting of a fresh task is valid:

```sec
let outcome := await (try spawn Calculate())
```

§ 3(5) Parentheses follow the ordinary precedence/grammar rules; this rulebook does not define special spawn-await parsing beyond the canonical expression grammar.

---

## § 4. Exact `TaskOutcome[T]`

**Governance tags:** `concurrency.await-v2`, `concurrency.tasks-v2`

§ 4(1) `await` resolves the canonical source-visible/compiler-known type owned by `tasks.md`:

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

§ 4(2) These four variants are the complete Sec 0.1 task-outcome surface.

§ 4(3) Exactly one variant is active.

§ 4(4) Scheduler states such as queued, runnable, running, blocked, or suspended are not `TaskOutcome[T]` variants.

§ 4(5) Comments documenting variants must appear before the variant declarations they describe.

---

## § 5. Exact `TaskError`

**Governance tags:** `concurrency.await-v2`, `concurrency.tasks-v2`

§ 5(1) The exact Sec 0.1 task-execution error type is:

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

§ 5(2) These four variants are the complete Sec 0.1 `TaskError` inventory.

§ 5(3) `TaskError` is distinct from `TaskSpawnError`.

§ 5(4) `TaskError` is distinct from an application-level error returned inside `T`.

§ 5(5) `InvalidConfiguration` is not a `TaskError` variant.

§ 5(6) Configuration failure that prevents task creation belongs to task creation and `TaskSpawnError`, not to an already-created task outcome.

§ 5(7) `TaskError` must not be fabricated merely because a task was cancelled or panicked.

---

## § 6. `TaskSpawnError` boundary

**Governance tags:** `concurrency.await-v2`, `concurrency.tasks-v2`

§ 6(1) `TaskSpawnError` describes failure before a `Task[T]` exists.

§ 6(2) Its canonical declaration remains:

```sec
enum TaskSpawnError error {
    // Memory required to create the task could not be obtained.
    OutOfMemory

    // A creation-time task/worker/thread/slot resource limit was exhausted.
    ResourceLimit

    // The configured executor could not accept/create the requested task.
    ExecutorUnavailable

    // A runtime task configuration valid as source input could not be
    // instantiated under the selected execution configuration.
    InvalidConfiguration

    // A lower-level creation failure was not represented more specifically.
    NativeFailure
}
```

§ 6(3) `await` never returns `TaskSpawnError` because its operand is an already-existing `Task[T]`.

---

## § 7. Exact `PanicInfo` dependency

**Governance tags:** `concurrency.await-v2`, `errors.panic-v2`

§ 7(1) The `Panicked` payload is the exact `PanicInfo` type owned by `panic.md`.

§ 7(2) Its portable Sec 0.1 source-visible shape is:

```sec
type PanicInfo struct {
    // Stable registered panic-reason identity.
    ID: PanicID,

    // Canonical source file metadata.
    File: string,

    // Canonical source line according to source-location rules.
    Line: uint,

    // Canonical source column according to source-location rules.
    Column: uint,

    // Canonical declaring/executing function metadata.
    Function: string,
}
```

§ 7(3) `PanicInfo` carries panic observation metadata, not task lifecycle ownership.

§ 7(4) Whether a task-local panic can become `Panicked(PanicInfo)` depends on the selected panic containment policy.

§ 7(5) A hard-termination policy that cannot contain the panic is not required to fabricate a recoverable `TaskOutcome[T]`.

---

## § 8. Normal completion

**Governance tags:** `concurrency.await-v2`, `concurrency.tasks-v2`

§ 8(1) `Completed(value)` means the task function returned normally.

§ 8(2) `value` has exactly type `T`.

§ 8(3) `await` does not reinterpret `T`.

§ 8(4) If `T` is `Result[V, E]`, both `Ok(V)` and `Err(E)` are normal completion values.

§ 8(5) Example:

```sec
fn LoadImage() Result[Image, IOError] {
    // ...
}

let worker := try spawn LoadImage()
let outcome := await worker
```

has type:

```sec
TaskOutcome[Result[Image, IOError]]
```

§ 8(6) A returned `Err(IOError.InvalidValue)` is represented as `Completed(Err(IOError.InvalidValue))`, not as `Failed(TaskError)`.

---

## § 9. Cancelled outcome

**Governance tags:** `concurrency.await-v2`, `concurrency.cancellation-v2`

§ 9(1) `Cancelled` means the awaited task itself reached terminal cooperative cancellation.

§ 9(2) `Cancelled` contains no `T`.

§ 9(3) The runtime must not invent a default `T`.

§ 9(4) A cancellation request alone does not force `Cancelled`; the awaited task must actually commit terminal cancellation.

§ 9(5) If the awaited task returns normally before terminal cancellation commits, its outcome is `Completed(T)`.

---

## § 10. Panicked outcome

**Governance tags:** `concurrency.await-v2`, `errors.panic-v2`

§ 10(1) `Panicked(info)` means the task terminated through a panic contained at the task boundary according to the selected policy.

§ 10(2) `info` has exact type `PanicInfo`.

§ 10(3) A panic is distinct from `Cancelled`, `Failed(TaskError)`, and a normally returned `Err(E)`.

---

## § 11. Failed outcome

**Governance tags:** `concurrency.await-v2`, `concurrency.tasks-v2`

§ 11(1) `Failed(error)` means an already-created task reached a terminal execution/runtime failure.

§ 11(2) `error` has exact type `TaskError`.

§ 11(3) The runtime must choose exactly one applicable `TaskError` variant.

§ 11(4) `Failed(TaskError)` is not a generic wrapper around application-level failures.

§ 11(5) `Failed(TaskError)` is not used for task-creation failure.

---

## § 12. `Task[void]`

**Governance tags:** `concurrency.await-v2`, `concurrency.tasks-v2`

§ 12(1) Awaiting `Task[void]` produces `TaskOutcome[void]`.

§ 12(2) The normal variant is matched as the payload-less void specialization:

```sec
match await worker {
    Completed => {
        Continue()
    }

    Cancelled => {
        HandleCancellation()
    }

    Panicked(info) => {
        HandlePanic(info)
    }

    Failed(error) => {
        HandleTaskFailure(error)
    }
}
```

§ 12(3) `await Task[void]` does not collapse to plain `void`.

---

## § 13. Exhaustive outcome handling

**Governance tags:** `concurrency.await-v2`, `control-flow.match`

§ 13(1) `TaskOutcome[T]` follows ordinary exhaustive union matching.

§ 13(2) No producer-refined exhaustiveness exception is introduced for `await`.

§ 13(3) A normal complete match is:

```sec
match await worker {
    Completed(value) => {
        Use(value)
    }

    Cancelled => {
        HandleCancellation()
    }

    Panicked(info) => {
        HandlePanic(info)
    }

    Failed(error) => {
        HandleTaskFailure(error)
    }
}
```

§ 13(4) Ordinary discard/must-use rules govern whether an outcome may be explicitly discarded.

§ 13(5) `await` does not silently erase abnormal task outcomes.

---

## § 14. Handle consumption

**Governance tags:** `concurrency.await-v2`, `frontend.transferability`

§ 14(1) The source `Task[T]` handle is consumed when control enters the await operation with an available owning operand.

§ 14(2) Ownership of the lifecycle obligation transfers from the source binding into the await operation.

§ 14(3) The source binding does not remain available while the await is suspended.

§ 14(4) The source binding does not become available again after successful await completion.

§ 14(5) The source binding does not become available again when caller cancellation wins.

§ 14(6) A compiler must reject use after await consumption.

Example:

```sec
let worker := try spawn Calculate()
let outcome := await worker

worker.RequestCancel() // invalid: worker was consumed by await
```

---

## § 15. Await operation ownership

**Governance tags:** `concurrency.await-v2`, `analysis.transferability`

§ 15(1) After source-handle consumption and before await finishes, the await operation internally owns the task lifecycle capability.

§ 15(2) Internal ownership is semantic ownership even if the backend represents it through a task control block, scheduler token, continuation record, stack slot, register, or another implementation mechanism.

§ 15(3) The await operation must not lose this ownership through suspension, migration, cancellation, panic cleanup, or backend wakeup races.

§ 15(4) Internal lifecycle ownership ends only by successful await commit and transfer of the terminal outcome, or by caller-cancellation cleanup that drives the awaited task to terminal state and destroys/discards the terminal outcome internally.

---

## § 16. Await commit point

**Governance tags:** `concurrency.await-v2`, `concurrency.cancellation-v2`

§ 16(1) The semantic **await commit point** is reached when the awaited task is terminal, the await operation has acquired the terminal `TaskOutcome[T]`, and ownership of that outcome is ready to transfer to the awaiting continuation.

§ 16(2) Merely observing that the task appears done is not await commit.

§ 16(3) Merely receiving a scheduler wakeup is not await commit.

§ 16(4) Merely receiving a cancellation request is not await commit.

§ 16(5) Await commit is an atomic semantic boundary with respect to caller cancellation.

---

## § 17. Completion wins before caller cancellation

**Governance tags:** `concurrency.await-v2`, `concurrency.cancellation-v2`

§ 17(1) If await commit wins before current-execution cancellation commits, the await expression produces the acquired `TaskOutcome[T]`.

§ 17(2) Ownership of any active outcome payload transfers to the caller.

§ 17(3) The consumed task handle remains consumed.

§ 17(4) A later caller-cancellation observation cannot roll back the completed await.

§ 17(5) If caller cancellation becomes terminal after await commit, the already-produced outcome is cleaned up through ordinary deterministic cleanup.

---

## § 18. Caller cancellation wins before await commit

**Governance tags:** `concurrency.await-v2`, `concurrency.cancellation-v2`

§ 18(1) `await` is a cancellation point for the current execution.

§ 18(2) If current-execution cancellation commits before await commit, the await expression produces no `TaskOutcome[T]` to source-level continuation code.

§ 18(3) The await operation retains internal ownership of the consumed awaited task handle.

§ 18(4) The await operation requests cooperative cancellation of the awaited task.

§ 18(5) The cancellation request is semantically equivalent to invoking the canonical task cancellation request on the internally owned handle.

§ 18(6) The awaited task is then driven/waited until it reaches one terminal `TaskOutcome[T]`.

§ 18(7) The resulting terminal outcome is destroyed/discarded internally according to ordinary destruction rules.

§ 18(8) Only after the awaited task lifecycle has been resolved may terminal cancellation of the waiting execution complete.

§ 18(9) This rule deliberately preserves structured lifecycle cleanup instead of detaching the awaited task implicitly.

---

## § 19. No implicit detach on caller cancellation

**Governance tags:** `concurrency.await-v2`, `concurrency.structured-v1`

§ 19(1) Caller cancellation during await must not implicitly detach the awaited task.

§ 19(2) The runtime must not transfer the awaited task to an unstructured background task manager merely to allow the caller to cancel quickly.

§ 19(3) Once await has consumed the handle, caller cancellation follows the cleanup rule in § 18.

---

## § 20. No handle restoration

**Governance tags:** `concurrency.await-v2`, `analysis.transferability`

§ 20(1) Caller cancellation must not restore the source `Task[T]` binding.

§ 20(2) A cancelled caller may itself be unwinding and cannot receive a resurrected lifecycle value.

§ 20(3) The runtime must not manufacture a second owning handle.

§ 20(4) The await operation remains the unique lifecycle owner until cleanup resolves the child task.

---

## § 21. Child may delay caller cancellation

**Governance tags:** `concurrency.await-v2`, `concurrency.cancellation-v2`

§ 21(1) Cancellation is cooperative.

§ 21(2) After caller cancellation wins before await commit, the await cleanup requests cancellation of the awaited task but cannot assume immediate termination.

§ 21(3) The waiting execution's terminal cancellation may therefore be delayed until the awaited task reaches a terminal outcome.

§ 21(4) A task that does not observe cooperative cancellation may continue running according to task/cancellation semantics.

§ 21(5) This delay is an intentional consequence of preserving lifecycle ownership and safe borrowed-state cleanup.

§ 21(6) The compiler/runtime must not replace this rule with implicit detach.

---

## § 22. Race atomicity

**Governance tags:** `concurrency.await-v2`, `concurrency.cancellation-v2`

§ 22(1) Await completion commit and caller-cancellation commit participate in a first-commit race.

§ 22(2) Exactly one side wins.

§ 22(3) The runtime must not produce both a source-visible `TaskOutcome[T]` and the pre-commit caller-cancellation cleanup path for the same await.

§ 22(4) The runtime must not produce neither while losing the owned task lifecycle.

§ 22(5) Wakeup ordering, OS scheduling, worker migration, and interrupt timing do not change the semantic result of the committed race.

---

## § 23. Cancellation of awaited task versus caller cancellation

**Governance tags:** `concurrency.await-v2`, `concurrency.cancellation-v2`

§ 23(1) Cancellation of the awaited task and cancellation of the waiting execution are distinct state machines.

§ 23(2) If the awaited task reaches `Cancelled` first and await commits first, source code receives `TaskOutcome.Cancelled`.

§ 23(3) If caller cancellation wins before await commit, source code does not receive the child's eventual `TaskOutcome`.

§ 23(4) The child's eventual `Cancelled`, `Completed`, `Panicked`, or `Failed` outcome is destroyed internally after caller-cancellation cleanup has taken ownership of lifecycle resolution.

---

## § 24. Cancellation request before await

**Governance tags:** `concurrency.await-v2`, `concurrency.cancellation-v2`

§ 24(1) The owner may request cancellation before awaiting:

```sec
worker.RequestCancel()
let outcome := await worker
```

§ 24(2) `RequestCancel()` does not consume the handle.

§ 24(3) `RequestCancel()` does not guarantee that the task is already terminal.

§ 24(4) Await still resolves the actual terminal outcome.

§ 24(5) A requested task may still return normally, panic, or fail before terminal cooperative cancellation wins.

---

## § 25. Suspension versus physical blocking

**Governance tags:** `concurrency.await-v2`, `concurrency.blocking-v1`

§ 25(1) `await` is a potentially waiting operation.

§ 25(2) The language does not require one universal physical implementation.

§ 25(3) A conforming backend may suspend a logical task, park/reschedule a worker, block a native thread when the selected profile permits it, wait through an RTOS primitive, or resume through an event loop or equivalent scheduler mechanism.

§ 25(4) In a task backend, suspension is preferred when available and required by the selected execution model.

§ 25(5) Physical worker blocking is permitted only according to `blocking.md` and target/profile policy.

§ 25(6) Busy-waiting is not permitted unless the selected target/profile explicitly requires and documents bounded busy-wait semantics.

---

## § 26. Resumption

**Governance tags:** `concurrency.await-v2`, `concurrency.scheduling-v1`

§ 26(1) Await resumption may occur on a different permitted physical worker/thread than the one that entered await.

§ 26(2) Logical current-task identity must remain unchanged across permitted migration.

§ 26(3) Task-local cancellation state follows the logical task.

§ 26(4) Thread-local state follows the physical thread rules and must not be silently treated as task-local state across migration.

§ 26(5) Backend migration must not change the `TaskOutcome[T]` result.

---

## § 27. Result ownership transfer

**Governance tags:** `concurrency.await-v2`, `analysis.transferability`

§ 27(1) On successful await commit, the active terminal outcome transfers into the source-level `TaskOutcome[T]`.

§ 27(2) For `Completed(T)`, ownership of move-only `T` transfers exactly once.

§ 27(3) Copyable `T` follows ordinary value semantics after the outcome has been materialized.

§ 27(4) `Panicked(PanicInfo)` transfers/copies `PanicInfo` according to its ordinary copyable value semantics.

§ 27(5) `Failed(TaskError)` carries the exact error value.

§ 27(6) `Cancelled` carries no payload.

§ 27(7) No outcome payload remains separately extractable from the consumed task handle after await.

---

## § 28. Nested `Result`

**Governance tags:** `concurrency.await-v2`, `errors.result`

§ 28(1) `await` does not automatically invoke `try` on a completed `Result`.

§ 28(2) If `worker` has type `Task[Result[Image, IOError]]`, then `await worker` has type `TaskOutcome[Result[Image, IOError]]`.

§ 28(3) `Completed(Err(error))` is not promoted to `Failed(TaskError)`.

§ 28(4) This rulebook does not define a special `try await` flattening construct.

§ 28(5) Any future composition between `try` and `await` must preserve the type layers unless explicitly standardized elsewhere.

---

## § 29. Direct await of returned task

**Governance tags:** `concurrency.await-v2`, `analysis.transferability`

§ 29(1) A function may return ownership of a task.

```sec
fn StartWorker() Result[Task[int], TaskSpawnError] {
    return spawn Calculate()
}

let worker := try StartWorker()
let outcome := await worker
```

§ 29(2) Once task ownership moves out of the returning function, that function no longer owns the lifecycle obligation.

§ 29(3) The receiver of the returned task becomes responsible for await/join/detach/transfer according to `tasks.md`.

---

## § 30. Await in expressions

**Governance tags:** `concurrency.await-v2`, `frontend.await-v2`

§ 30(1) `await` is an expression.

§ 30(2) Valid contexts include ordinary expression positions where `TaskOutcome[T]` is type-compatible.

```sec
let outcome := await worker
return await worker
Handle(await worker)
```

§ 30(3) Evaluation order remains the ordinary deterministic Sec evaluation order.

§ 30(4) Optimizations must not reorder observable side effects across an await boundary when that changes program behavior.

§ 30(5) Await commit is a semantic side-effect/synchronization boundary for optimization purposes.

---

## § 31. Await inside `defer`

**Governance tags:** `concurrency.await-v2`, `control-flow.defer`

§ 31(1) `await` inside `defer` is invalid in Sec 0.1.

§ 31(2) A deferred block executes during cleanup and must not introduce a new suspending lifecycle wait.

```sec
defer {
    let outcome := await worker
}
```

§ 31(3) Suggested diagnostic:

```text
await is not allowed inside defer
```

§ 31(4) The prohibition applies whether await would complete immediately or suspend at runtime.

---

## § 32. Await during destruction

**Governance tags:** `concurrency.await-v2`, `memory.destruction`

§ 32(1) Destruction/finalization code must not introduce an await suspension unless an owning rule explicitly standardizes asynchronous destruction.

§ 32(2) Sec 0.1 does not define asynchronous destruction.

§ 32(3) Cleanup triggered by caller cancellation of await may wait internally for the already-consumed child lifecycle as specified in § 18; this is runtime lifecycle cleanup, not user-authored `await` inside a destructor.

---

## § 33. Static and compile-time initialization

**Governance tags:** `concurrency.await-v2`, `compiler.initialization`

§ 33(1) `await` is not permitted during compile-time evaluation.

§ 33(2) `await` is not permitted in hidden static initialization that would introduce implicit asynchronous runtime execution.

§ 33(3) Runtime task startup and waiting must remain visible in ordinary executable control flow.

---

## § 34. Main function

**Governance tags:** `concurrency.await-v2`, `concurrency.tasks-v2`

§ 34(1) `main` may await owned tasks normally when the selected target/runtime supports tasks.

§ 34(2) Program-exit lifecycle rules remain owned by task/runtime/structured-concurrency rules.

§ 34(3) This rulebook does not authorize unresolved owned task handles to be silently abandoned when `main` returns.

---

## § 35. Await versus join

**Governance tags:** `concurrency.await-v2`, `concurrency.tasks-v2`

§ 35(1) `await` and `join` are distinct:

```text
await task
    waits for terminal task completion
    consumes Task[T]
    resolves lifecycle ownership
    returns TaskOutcome[T]

join task
    waits for terminal task completion
    preserves the owning task handle
    establishes terminal observation/synchronization
    does not itself transfer TaskOutcome[T]
```

§ 35(2) `await` does not become `join + value`.

§ 35(3) Join-specific repeatability/terminal-inspection rules remain owned by `tasks.md`.

§ 35(4) This rulebook does not invent task status/value members beyond those canonically defined by `tasks.md`.

---

## § 36. Await after join

**Governance tags:** `concurrency.await-v2`, `concurrency.tasks-v2`

§ 36(1) A task handle preserved by a valid join may subsequently be consumed by await if the owning task rules still consider its lifecycle/result capability available.

§ 36(2) Such an await may complete without suspension because the task is already terminal.

§ 36(3) It still consumes the handle and returns the full `TaskOutcome[T]`.

§ 36(4) If another operation has already consumed/taken a move-only completion payload in a way that makes complete outcome transfer impossible, the later await must be rejected according to task lifecycle/value-extraction rules.

---

## § 37. Repeated await

**Governance tags:** `concurrency.await-v2`, `analysis.transferability`

§ 37(1) Repeated await through the same owning handle is invalid because the first await consumes the handle.

```sec
let worker := try spawn Calculate()

let first := await worker
let second := await worker
```

§ 37(2) Suggested diagnostic:

```text
use of consumed task worker
```

§ 37(3) The compiler must diagnose this statically when ownership flow proves the second use invalid.

---

## § 38. Observer boundary

**Governance tags:** `concurrency.await-v2`, `concurrency.tasks-v2`

§ 38(1) A non-owning task observer cannot be used as the operand of owning `await`.

§ 38(2) Observer waits may synchronize with completion but do not transfer the owning `TaskOutcome[T]`.

§ 38(3) Observer APIs remain owned by `tasks.md`.

§ 38(4) The compiler must not implicitly upgrade an observer into an owning task handle.

---

## § 39. Threads are not awaitable

**Governance tags:** `concurrency.await-v2`, `concurrency.thread-v2`

§ 39(1) Sec 0.1 `await` is task-specific.

§ 39(2) `Thread[T]` is not accepted by `await`.

```sec
let worker := try spawn thread Work()
let outcome := await worker
```

§ 39(3) Suggested diagnostic:

```text
await requires Task[T]; got Thread[T]
```

§ 39(4) Thread completion uses its canonical join/lifecycle API unless a future explicit awaitable adapter is standardized.

---

## § 40. Processes are not awaitable

**Governance tags:** `concurrency.await-v2`, `concurrency.process-v1`

§ 40(1) Process handles are not accepted by Sec 0.1 `await`.

§ 40(2) Suggested diagnostic:

```text
await requires Task[T]; got Process
```

§ 40(3) Process completion remains governed by the process/join APIs.

---

## § 41. Other values are not awaitable

**Governance tags:** `concurrency.await-v2`, `frontend.await-v2`

§ 41(1) Sec 0.1 does not define a general structural `Awaitable` interface.

§ 41(2) Arbitrary user-defined values cannot become awaitable by naming a method `Await`, `Poll`, `Ready`, or similar.

§ 41(3) No implicit conversion to `Task[T]` occurs.

§ 41(4) Future generalized awaitability requires an explicit language/rulebook design.

---

## § 42. Mutex guards across await

**Governance tags:** `concurrency.await-v2`, `concurrency.mutex-v2`

§ 42(1) A live `MutexGuard[T]` may not cross an await suspension boundary.

§ 42(2) The compiler must conservatively reject an await while a guard remains live unless it proves the guard is ended/destroyed before suspension can occur.

§ 42(3) Suggested diagnostic:

```text
mutex guard state remains active across await
```

§ 42(4) The diagnostic should identify guard acquisition, live guard binding, await expression, and a scope/release action that can end the guard before await.

---

## § 43. Ordinary borrows across await

**Governance tags:** `concurrency.await-v2`, `analysis.borrowing`

§ 43(1) An ordinary borrow may remain live across await only when analysis proves all required lifetime/alias/address-stability/migration conditions.

§ 43(2) At minimum, analysis must prove the owner remains alive, referenced storage remains stable, no conflicting access occurs, suspension/resumption does not invalidate the reference, migration restrictions are satisfied, and cancellation cleanup cannot outlive the referenced owner.

§ 43(3) Await is not an automatic end of all borrows.

§ 43(4) Await may be the proven end point for a borrow retained by the awaited task only when the awaited lifecycle is fully resolved.

---

## § 44. Borrowed state used by awaited task

**Governance tags:** `concurrency.await-v2`, `analysis.borrowing`, `concurrency.structured-v1`

§ 44(1) If the awaited task validly holds a borrow into caller-owned state, that borrow remains live until the task reaches terminal state and releases the borrow according to task cleanup.

§ 44(2) Caller cancellation before await commit does not permit the caller to destroy that owner immediately.

§ 44(3) The cleanup rule in § 18 preserves this lifetime by requesting child cancellation and waiting for terminal task cleanup before caller cancellation completes.

§ 44(4) Implicit detach would violate this guarantee and is therefore forbidden.

---

## § 45. Move-only values live across await

**Governance tags:** `concurrency.await-v2`, `analysis.transferability`

§ 45(1) Ordinary owned move-only values may remain live across await if suspension storage can preserve their ownership and destruction requirements.

§ 45(2) The compiler must spill/preserve them as needed in the task continuation/frame representation.

§ 45(3) Caller cancellation must destroy still-owned values exactly once.

§ 45(4) A value transferred into the awaited task is not simultaneously owned by the waiter.

---

## § 46. `await` and cancellation effect inference

**Governance tags:** `concurrency.await-v2`, `concurrency.cancellation-v2`

§ 46(1) `await` is a cancellation point for current execution.

§ 46(2) Merely containing `await` does not imply that the function contains the `cancel` statement.

§ 46(3) The compiler-inferred cancellable-execution requirement remains governed by `cancellation.md`.

§ 46(4) This rulebook introduces no `@cancellable` source annotation.

---

## § 47. No `Context` operand

**Governance tags:** `concurrency.await-v2`, `concurrency.context-v1`

§ 47(1) Sec 0.1 `await` has no `Context` parameter.

§ 47(2) A lexical `Context` does not implicitly alter await.

§ 47(3) Context cancellation does not automatically cancel an await unless application/runtime logic converts that condition into current-execution/task cancellation through an explicitly defined API.

§ 47(4) No hidden ambient Context is read by `await`.

---

## § 48. Timeout

**Governance tags:** `concurrency.await-v2`, `concurrency.select-v2`

§ 48(1) Sec 0.1 defines no await timeout parameter or `Await(context)` method surface.

§ 48(2) Waiting timeout is composed through `select`/`after` when the select rules support the relevant task branch.

§ 48(3) A timeout branch that wins before task-result commit must preserve lifecycle ownership according to select/task rules; it must not silently abandon the task.

§ 48(4) This rulebook does not invent detached-timeout semantics.

---

## § 49. Select interaction

**Governance tags:** `concurrency.await-v2`, `concurrency.select-v2`

§ 49(1) Generic select syntax/readiness/commit remains owned by `select.md`.

§ 49(2) `await Task[T]` is selectable and a selected await branch has exact result type `TaskOutcome[T]`.

§ 49(3) Non-selected branches must not consume the owning task handle.

§ 49(4) A selected branch that commits task-outcome ownership must follow the same terminal-outcome transfer semantics as direct await.

§ 49(5) Mere branch preparation does not consume the task handle. If another branch or current-execution cancellation wins before await-branch commit, the handle remains source-owned and direct-await cancellation cleanup is not invoked.

§ 49(6) Once the await branch commits, it consumes the handle and the ordinary await lifecycle and cancellation rules apply from that commit point onward.

---

## § 50. Memory synchronization

**Governance tags:** `concurrency.await-v2`, `concurrency.memory-model-v2`

§ 50(1) Successful await commit establishes task-completion synchronization.

§ 50(2) Memory effects published by the completed task according to the concurrency memory model become visible to the awaiting continuation as required by the canonical completion happens-before edge.

§ 50(3) Merely observing a task status flag is not equivalent to await synchronization.

§ 50(4) A caller-cancellation cleanup path that internally waits for the child to become terminal must establish enough synchronization to safely destroy the terminal outcome and child-owned resources.

§ 50(5) Cancellation request state alone is not synchronization for unrelated application memory.

---

## § 51. Destruction after successful await

**Governance tags:** `concurrency.await-v2`, `memory.destruction`

§ 51(1) After await commit, task runtime/scheduling resources owned solely by the consumed handle may be reclaimed when no observer/internal dependency still requires them.

§ 51(2) The active `TaskOutcome[T]` payload is now owned by source-level code.

§ 51(3) Destruction of that outcome follows ordinary union/payload destruction rules.

§ 51(4) The runtime must not also destroy a transferred move-only `Completed(T)` payload.

---

## § 52. Destruction after caller-cancelled await

**Governance tags:** `concurrency.await-v2`, `memory.destruction`, `concurrency.cancellation-v2`

§ 52(1) If caller cancellation wins before await commit, no source-level `TaskOutcome[T]` is produced.

§ 52(2) After the awaited task reaches terminal state during cleanup, its terminal outcome is owned internally by the await cleanup path.

§ 52(3) That outcome and every active payload are destroyed exactly once.

§ 52(4) A `Completed(T)` produced despite the cancellation request is destroyed normally; it is not leaked.

§ 52(5) `Panicked(PanicInfo)` and `Failed(TaskError)` payloads are destroyed according to ordinary value rules.

---

## § 53. Panic in waiting execution

**Governance tags:** `concurrency.await-v2`, `errors.panic-v2`

§ 53(1) A panic of the waiting execution is distinct from caller cancellation.

§ 53(2) If the waiter panics while it internally owns a consumed task lifecycle, cleanup obligations follow structured task/destruction/panic containment rules.

§ 53(3) This rulebook does not authorize losing or silently detaching the awaited task merely because the waiter panics.

§ 53(4) Whether cleanup can fully run depends on the selected panic containment policy.

---

## § 54. Hidden await state

**Governance tags:** `concurrency.await-v2`

§ 54(1) A conforming implementation may internally maintain state equivalent to:

```text
awaited task identity/control block
consumed lifecycle ownership token
concrete T / TaskOutcome[T] type identity
wait registration
continuation/resumption identity
caller logical-task identity
caller cancellation registration/state
await commit state
terminal outcome staging storage
payload ownership state
borrow/live-across-suspension facts
source provenance
scheduler/backend wait state
cleanup-in-progress state
```

§ 54(2) These are implementation obligations/facts, not public source fields.

§ 54(3) The physical representation may vary by target/runtime.

§ 54(4) The representation must preserve exactly-once lifecycle ownership and outcome destruction/transfer.

---

## § 55. Target and profile requirements

**Governance tags:** `concurrency.await-v2`, `compiler.platform-model`

§ 55(1) The selected immutable `CompilationPlan` determines concrete task waiting support.

§ 55(2) Relevant capabilities may include task runtime availability, suspension/resumption support, worker parking/blocking policy, cancellation wakeup support, task-boundary panic containment, task runtime failure reporting, synchronization primitives, and bounded/no-allocation runtime constraints.

§ 55(3) A target that cannot support tasks at all must reject task creation/use through target validation rather than inventing an await runtime error.

§ 55(4) Await has no separate target-specific result type.

§ 55(5) Target implementation choices must preserve the exact `TaskOutcome[T]` semantics.

---

## § 56. Semantic analysis

**Governance tags:** `frontend.await-v2`, `analysis.transferability`

§ 56(1) Sema must validate at least:

- operand type is exactly an owning available `Task[T]`;
- handle ownership is available;
- handle has not already been consumed;
- result static type is exactly `TaskOutcome[T]`;
- all exact `TaskOutcome` payload types are resolved;
- move-only payload ownership is represented;
- caller-cancellation commit behavior is preserved;
- live mutex guards do not cross an invalid suspension;
- borrows remain valid across suspension and cancellation cleanup;
- await is not inside `defer`;
- await is not used in forbidden compile-time/static initialization;
- selected target/profile supports task waiting;
- lifecycle ownership is resolved on every control-flow edge.

§ 56(2) Sema must not infer `T` directly as await result.

§ 56(3) Sema must not rewrite `Task[Result[V, E]]` into a flattened result.

§ 56(4) Sema must distinguish a non-owning observer from owning `Task[T]`.

§ 56(5) Sema must reject `Thread[T]`, Process handles, and arbitrary user values as await operands.

---

## § 57. Flow-sensitive ownership analysis

**Governance tags:** `frontend.await-v2`, `analysis.transferability`

§ 57(1) Entering await consumes the source lifecycle handle on every runtime path that executes the await.

§ 57(2) The source binding is unavailable after the await expression in all continuing source paths.

§ 57(3) Caller cancellation before await commit transfers no handle back to source code.

§ 57(4) Successful outcome payload ownership must merge according to ordinary union branch analysis.

§ 57(5) A compiler must detect repeated await/use of the consumed handle where path information is sufficient.

---

## § 58. Borrow analysis

**Governance tags:** `frontend.await-v2`, `analysis.borrowing`

§ 58(1) Borrow analysis must model await as a potential suspension point.

§ 58(2) It must model caller-cancellation cleanup as potentially extending child/task-held borrows until child terminal cleanup completes.

§ 58(3) A borrow that would become invalid while the child remains running must cause a compile-time error.

§ 58(4) A raw backend assumption that caller cancellation ends all child access immediately is invalid.

---

## § 59. Blocking and deadlock analysis

**Governance tags:** `concurrency.await-v2`, `sema.deadlock-analysis`, `concurrency.blocking-v1`

§ 59(1) Await contributes a wait dependency from the waiting execution to the awaited task lifecycle.

§ 59(2) Caller-cancellation cleanup may preserve that wait dependency until the child terminates.

§ 59(3) Deadlock/starvation analysis must not assume caller cancellation necessarily breaks an await dependency immediately.

§ 59(4) Holding a mutex guard across await remains a diagnostic/prohibition according to mutex/blocking rules.

§ 59(5) Target profiles that physically block workers must participate in executor-starvation analysis.

---

## § 60. Semantic IR requirements

**Governance tags:** `semantic-ir.await-v2`, `concurrency.await-v2`

§ 60(1) Semantic IR must preserve await as an explicit semantic operation/fact rather than lowering it immediately to ordinary function calls or raw synchronization.

§ 60(2) Required facts include at least:

- concrete `Task[T]`;
- concrete `TaskOutcome[T]`;
- consumed lifecycle ownership;
- task identity/control identity;
- await commit boundary;
- terminal outcome category;
- active payload ownership;
- current-execution cancellation registration;
- caller-cancellation versus await-commit race;
- internal child cancellation request on caller-cancel cleanup;
- internal lifecycle retention until terminal child state;
- internal outcome destruction path;
- suspension/resumption continuation;
- live borrows across suspension;
- guard/suspension constraints;
- memory-synchronization edge;
- source provenance;
- selected target/profile requirements.

§ 60(3) Concrete opcode/instruction names remain owned by `semantic_ir.md`.

§ 60(4) The legacy literal list `TaskJoin`, `TaskAwait`, `TaskOutcome`, `TaskTakeValue`, `TaskConsume`, `TaskCancelResume` is not normative merely because it appeared in the previous await rulebook.

§ 60(5) IR verification must reject a state where lifecycle ownership is lost, duplicated, simultaneously transferred and internally retained, or destroyed twice.

---

## § 61. Lowering

**Governance tags:** `lowering.await-v2`, `compiler.platform-model`

§ 61(1) Lowering consumes validated await Semantic IR plus the selected `CompilationPlan`.

§ 61(2) Lowering may choose target/runtime-specific suspension or blocking mechanisms.

§ 61(3) Lowering must preserve invariant `TaskOutcome[T]`, exactly-once handle consumption, exactly-once terminal outcome transfer or internal destruction, await/caller-cancellation first-commit race, no implicit detach, child cancellation request on pre-commit caller cancellation, child lifecycle resolution before caller terminal cancellation completes, borrow/lifetime safety, task-completion memory synchronization, and panic/task-failure distinction.

§ 61(4) Lowering must not treat `await` as a normal call returning `T`.

§ 61(5) Lowering must not rely on host-language async semantics when the selected target differs.

---

## § 62. Runtime obligations

**Governance tags:** `runtime.await-v2`, `concurrency.await-v2`

§ 62(1) The runtime must provide an equivalent mechanism for registering completion wait and cancellation wakeup without losing lifecycle ownership.

§ 62(2) Completion and caller cancellation races must resolve atomically at the semantic level.

§ 62(3) A pre-commit caller-cancellation winner must cause cooperative cancellation to be requested for the awaited task.

§ 62(4) Runtime cleanup must continue to own/wait the child until terminal state.

§ 62(5) The terminal child outcome must then be destroyed internally.

§ 62(6) Only then may the waiting execution complete terminal cancellation.

§ 62(7) Runtime internals may optimize already-terminal tasks but must preserve the same ownership/commit semantics.

---

## § 63. LSP hover

**Governance tags:** `tooling.await-v2`

§ 63(1) Hover on an await expression must show `TaskOutcome[T]` with concrete substituted `T`.

§ 63(2) Hover must never show bare `T` merely because normal completion is statically proven.

§ 63(3) Hover/navigation for `TaskOutcome[T]` must expose all four exact variants.

§ 63(4) Hover/navigation for `TaskError` must expose exactly `OutOfMemory`, `ResourceLimit`, `ExecutorUnavailable`, and `NativeFailure`, with documentation before each variant in source documentation.

§ 63(5) Hover/navigation for `PanicInfo` must resolve to the canonical panic-owned declaration.

§ 63(6) LSP must consume compiler/Sema type facts rather than implement a parallel await type system.

---

## § 64. Completion and navigation

**Governance tags:** `tooling.await-v2`

§ 64(1) Completion after matching a `TaskOutcome[T]` should offer all four canonical variants.

§ 64(2) Completion must not invent `Success`, `Error`, `Timeout`, or other non-canonical outcome variants.

§ 64(3) Navigation from `TaskError` and its variants must resolve to the single canonical source-visible/core declaration.

§ 64(4) Navigation from `PanicInfo` must resolve to the panic/core declaration owned by `panic.md`.

§ 64(5) Diagnostics/fixes should use canonical `RequestCancel()` spelling rather than legacy `cancel()`.

---

## § 65. Diagnostics

**Governance tags:** `tooling.await-v2`, `frontend.await-v2`

§ 65(1) Suggested diagnostics include:

```text
await requires Task[T]; got Thread[T]
```

```text
await requires Task[T]; got Process
```

```text
await requires an owning Task[T]; got TaskObserver[T]
```

```text
use of consumed task worker
```

```text
await is not allowed inside defer
```

```text
mutex guard state remains active across await
```

```text
borrow may become invalid while awaited task is still running
```

```text
await is not available during compile-time evaluation
```

§ 65(2) Diagnostics should identify the consumed handle and prior consumption site.

§ 65(3) Borrow diagnostics should identify the owner, borrow creation, awaited task transfer/use, and await/cancellation cleanup lifetime.

§ 65(4) Diagnostics must not recommend implicit detach as an automatic fix for caller cancellation.

---

## § 66. Examples

**Governance tags:** `concurrency.await-v2`

§ 66(1) Ordinary completion:

```sec
fn Calculate() int {
    return 42
}

let worker := try spawn Calculate()

match await worker {
    Completed(value) => {
        Print(value)
    }

    Cancelled => {
        HandleCancellation()
    }

    Panicked(info) => {
        HandlePanic(info)
    }

    Failed(error) => {
        HandleTaskFailure(error)
    }
}
```

§ 66(2) Application-level error remains nested:

```sec
let worker := try spawn LoadImage()

match await worker {
    Completed(Ok(image)) => {
        Use(image)
    }

    Completed(Err(error)) => {
        HandleIO(error)
    }

    Cancelled => {
        HandleCancellation()
    }

    Panicked(info) => {
        HandlePanic(info)
    }

    Failed(error) => {
        HandleTaskFailure(error)
    }
}
```

§ 66(3) Cancellation request followed by lifecycle resolution:

```sec
let worker := try spawn Work()
worker.RequestCancel()
let outcome := await worker
```

§ 66(4) `RequestCancel()` does not predetermine which `TaskOutcome` variant is returned.

---

## § 67. Restrictions

**Governance tags:** `concurrency.await-v2`

§ 67(1) `await` must not:

- accept a non-owning observer as if it were `Task[T]`;
- accept `Thread[T]` or process handles;
- accept arbitrary user-defined awaitables in Sec 0.1;
- return bare `T`;
- flatten nested `Result`;
- invent a default `T` for abnormal outcomes;
- copy an owning task handle;
- restore a consumed task handle after caller cancellation;
- implicitly detach the awaited task on caller cancellation;
- lose the task lifecycle during a completion/cancellation race;
- produce both a source-visible outcome and a pre-commit caller-cancellation cleanup result;
- allow a live mutex guard to cross an invalid suspension;
- bypass ordinary borrow/lifetime rules;
- appear in `defer`;
- appear in compile-time evaluation;
- be lowered as an ordinary function call without task semantics.

---

## § 68. Explicitly absent Sec 0.1 forms

**Governance tags:** `concurrency.await-v2`

§ 68(1) This rulebook does not define:

```text
general Awaitable interface
await Thread[T]
await Process
await with Context
await timeout parameter
await returning T directly
implicit Result flattening
implicit detach on cancellation
implicit handle restoration
multi-await of one owning handle
asynchronous defer
asynchronous destruction
```

§ 68(2) Future additions require explicit normative design.

---

## § 69. Conformance scenarios

**Governance tags:** `concurrency.await-v2`

§ 69(1) Conformance tests must include at least:

- `await Task[int] -> TaskOutcome[int]`;
- `await Task[void] -> TaskOutcome[void]`;
- nested `Result` preservation;
- `Completed(Err(E))` as normal completion;
- exact four-way `TaskOutcome[T]`;
- exact four-way `TaskError`;
- exact `PanicInfo` payload;
- move-only completed payload transfer exactly once;
- task handle consumed by await;
- repeated await rejected;
- observer/thread/process await rejected;
- await inside defer rejected;
- compile-time await rejected;
- mutex guard across await rejected;
- valid borrow across await preserved;
- invalid borrow crossing await rejected;
- completion wins versus caller-cancellation race;
- caller cancellation wins before await commit;
- child cancellation requested on pre-commit caller cancellation;
- no implicit detach;
- no source-handle restoration;
- child terminal outcome destroyed internally on caller-cancel cleanup;
- child ignoring cancellation delays caller terminal cancellation;
- completion synchronization publishes task memory effects;
- direct await of a freshly spawned task;
- await after valid join where complete outcome remains available;
- LSP hover always shows `TaskOutcome[T]`;
- Semantic IR preserves lifecycle ownership and commit race.

---

## § 70. Cross-rulebook synchronization

**Governance tags:** `concurrency.await-v2`, `concurrency.tasks-v2`, `concurrency.cancellation-v2`

§ 70(1) This revision is synchronized with `tasks.md`, including exact `TaskError`.

§ 70(2) `tasks.md` and `cancellation.md` examples/declarations use documentation comments before the variants/members they document.

§ 70(3) Canonical examples and tooling use `RequestCancel()`; lowercase legacy cancellation spelling is not supported.

§ 70(4) `cancellation.md` § 29 cross-references the exact cleanup rule owned here when caller cancellation wins.

§ 70(5) `blocking.md` must continue to classify await as potentially waiting and must not imply caller cancellation necessarily removes the child lifecycle dependency immediately.

§ 70(6) `semantic_ir.md` owns concrete IR vocabulary; the old await opcode list is not canonical.

§ 70(7) `language-rulebook-status.md` records this rulebook as synchronized revision 2.0.

---

## § 71. Governance

**Governance tags:** `concurrency.await-v2`, `frontend.await-v2`, `semantic-ir.await-v2`, `lowering.await-v2`, `runtime.await-v2`, `tooling.await-v2`, `concurrency.tasks-v2`, `concurrency.cancellation-v2`, `concurrency.blocking-v1`, `concurrency.memory-model-v2`, `analysis.transferability`, `analysis.borrowing`, `sema.deadlock-analysis`, `compiler.platform-model`

§ 71(1) `governance/concurrency_await.yaml` is the canonical implementation-status owner for await expressions and task-outcome observation.

§ 71(2) `governance/concurrency_task.yaml` remains the canonical implementation-status owner for task type/outcome/error declarations themselves.

§ 71(3) This await governance fragment may reference required task synchronization but must not duplicate the task integration entry.

§ 71(4) Root `implementation-status.yaml` is not a second canonical ledger.

§ 71(5) Cross-rulebook synchronization required by this revision is tracked by the accompanying await-v2 correction.

§ 71(6) After all correction steps are applied, the correction should be archived under `rules/corrections/applied/`.

## Debug-information integration

`rules/compiler/debug_information.md` owns debugger-specific suspension and
stepping representation. An `await` source range and its live semantic state
remain available to debug lowering when `LineTables` or `Full` requires logical
suspended-task inspection. Compiler-generated resume and state-machine paths
remain implementation details and do not turn one `await` into a second source
call or source frame.
