# Scheduling

- **Status:** Normative
- **Created:** 2026-09-27
- **Last updated:** 2026-09-27
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/scheduling.md`
- **Replaces:** Earlier legacy revision at the same canonical path
- **Repository baseline reviewed:** `main-reviewed-2026-09-27`
- **Implementation governance:** `governance/concurrency_scheduling.yaml`
- **Related rulebooks:** `rules/concurrency/concurrency.md`, `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/concurrency/thread_local.md`, `rules/concurrency/await.md`, `rules/concurrency/select.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/blocking.md`, `rules/concurrency/mutex.md`, `rules/concurrency/channels.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/concurrency/concurrency_runtime_model.md`, `rules/analysis/deadlock_analysis.md`, `rules/compiler/semantic_ir.md`, `rules/platform/platform_model.md`, `rules/platform/target_profiles.md`, `rules/tooling/lsp.md`

---

## § 1. Purpose and authority

**Governance tags:** `concurrency.scheduling-v2`

§ 1(1) This rulebook defines the portable Sec 0.1 semantics of scheduling, scheduling points, yielding, task migration, physical-thread scheduling, scheduler-facing target requirements, and scheduling-specific analysis.

§ 1(2) Scheduling policy is target/runtime dependent.

§ 1(3) Source-visible execution kind, ownership, lifecycle, cancellation, result, borrow, memory-order, and task/thread identity semantics are target independent.

§ 1(4) This rulebook does not define one mandatory scheduler algorithm.

§ 1(5) Mutable implementation status belongs only in `governance/concurrency_scheduling.yaml`.

---

## § 2. Execution kinds remain distinct

**Governance tags:** `concurrency.scheduling-v2`

§ 2(1) Sec distinguishes:

```text
Task
Thread
Process
Interrupt service routine
```

§ 2(2) A task is a logical scheduled execution.

§ 2(3) A thread is a physical/native execution context.

§ 2(4) A process has a distinct lifecycle and normally a distinct address-space/process boundary.

§ 2(5) An ISR is a hardware-triggered restricted execution context.

§ 2(6) A scheduler/backend must not silently change one requested execution kind into another.

---

## § 3. Task implementation freedom

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.tasks-v2`

§ 3(1) A conforming task runtime may use:

- compiler-generated state machines;
- a cooperative executor;
- a worker pool;
- lightweight stackful tasks;
- RTOS task facilities;
- native threads;
- an event loop;
- statically allocated task slots;
- another target-declared mechanism.

§ 3(2) The implementation choice must not alter the source meaning of:

```sec
Task[T]
TaskContext
Task.Current()
Task.Yield()
await
join
detach
RequestCancel()
```

§ 3(3) In particular, using one native thread per task does not make `Task[T]` semantically a `Thread[T]`.

---

## § 4. Physical thread implementation

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.thread-v2`

§ 4(1) `spawn thread` requests a physical/native thread identity.

§ 4(2) A target may lower it to a POSIX thread, Windows native thread, RTOS physical execution identity, or another explicitly declared physical-thread facility.

§ 4(3) It must not be silently lowered to an ordinary logical `Task[T]`.

---

## § 5. Eager task scheduling

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.tasks-v2`

§ 5(1) Task spawn is eager by default.

§ 5(2) After successful task creation, the logical task may be submitted, runnable, running, waiting, or already terminal.

§ 5(3) Source code receives no guarantee that the new task executes before the next source statement.

§ 5(4) Source code receives no guarantee that it executes on another physical thread.

---

## § 6. Eager and deferred physical threads

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.thread-v2`

§ 6(1) Physical thread creation is eager by default.

§ 6(2) `ThreadStartMode.Deferred` creates a physical thread identity whose user callable must not execute before successful `Start()` commit.

§ 6(3) A backend may implement deferred start through native suspended creation or an internal start gate.

§ 6(4) Deferred start is a semantic requirement and must not be silently ignored.

§ 6(5) Sec 0.1 does not infer task deferred-start semantics from the thread feature.

---

## § 7. Exact task yield surface

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.tasks-v2`

§ 7(1) The exact portable task-yield surface is:

```sec
impl Task {
    static fn Yield() void
}
```

§ 7(2) `Task.Yield()` is valid only while executing in a real logical task context.

§ 7(3) It does not require or consume an owning `Task[T]` handle.

§ 7(4) It acts on the current logical task.

---

## § 8. Task yield scheduling semantics

**Governance tags:** `concurrency.scheduling-v2`

§ 8(1) A normal `Task.Yield()` commit keeps the current task runnable and gives the selected scheduler an opportunity to run another ready task.

§ 8(2) It does not guarantee another task runs, fairness, a physical context switch, worker migration, progress by a particular waiter, or memory synchronization.

§ 8(3) After a normal yield/resume, `Task.Yield()` returns `void`.

§ 8(4) `Task.Yield()` is not a replacement for a channel, mutex, atomic, blocking wait, `select`, join, or await.

---

## § 9. Task yield is a cancellation point

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.cancellation-v2`

§ 9(1) `Task.Yield()` is a current-task cancellation point.

§ 9(2) Yield commit and current-task cancellation commit participate in the canonical first-commit race.

§ 9(3) If cancellation commits first, the yield operation does not return to the following source statement and ordinary task cancellation cleanup begins.

§ 9(4) If normal yield commits first, the task remains runnable, the scheduler gets an opportunity to schedule another task, and execution later resumes with `Task.Yield()` returning `void`.

§ 9(5) A backend must not both return from the yield and execute the pre-commit cancellation path for the same yield operation.

---

## § 10. Task yield example

**Governance tags:** `concurrency.scheduling-v2`

§ 10(1) Cooperative bounded work may use:

```sec
while WorkRemains() {
    DoBoundedWork()
    Task.Yield()
}
```

§ 10(2) A pending task cancellation may commit at each `Task.Yield()`.

§ 10(3) This does not guarantee fairness among all runnable tasks.

---

## § 11. Exact physical-thread yield surface

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.thread-v2`

§ 11(1) The exact portable physical-thread yield surface is:

```sec
impl Thread {
    static fn Yield() void
}
```

§ 11(2) `Thread.Yield()` acts on the current physical/native thread.

§ 11(3) It is a scheduler hint.

§ 11(4) It does not require or consume an owning `Thread[T]` handle.

---

## § 12. Thread yield is not a cancellation point

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.cancellation-v2`

§ 12(1) `Thread.Yield()` is not a cancellation point in Sec 0.1.

§ 12(2) It does not implicitly inspect:

```sec
Thread.Current().CancelRequested
```

§ 12(3) It does not execute `cancel`.

§ 12(4) Physical-thread cooperative cancellation remains observable through explicit `CancelRequested` checks and canonical cancellation-aware waits.

§ 12(5) A target/native scheduler may still preempt a physical thread independently of Sec cancellation semantics.

---

## § 13. Thread yield guarantees

**Governance tags:** `concurrency.scheduling-v2`

§ 13(1) `Thread.Yield()` guarantees none of another thread running, fairness, a context switch, progress by a specific thread, cancellation observation, memory publication, or synchronization.

§ 13(2) It returns `void` after the target/native yield operation returns.

§ 13(3) It is invalid in ISR context.

---

## § 14. Task scheduling points

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.blocking-v1`

§ 14(1) A task scheduling point is a source/semantic operation at which the current logical task may suspend, yield, wait, or otherwise allow scheduling of another task.

§ 14(2) Sec 0.1 task scheduling points include at least:

```text
await
task-context waiting join
waiting channel operation
waiting select
waiting mutex acquisition
other cancellation-aware waits that suspend tasks
Task.Yield()
target-declared task suspension primitives
```

§ 14(3) A non-waiting operation is not a scheduling point merely because the backend performs internal bookkeeping.

---

## § 15. Scheduling point versus cancellation point

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.cancellation-v2`

§ 15(1) "Scheduling point" and "cancellation point" are different semantic properties.

§ 15(2) An operation may be both.

§ 15(3) `Task.Yield()` is both.

§ 15(4) `Thread.Yield()` is neither a task scheduling point nor a cancellation point; it is a physical scheduler hint.

§ 15(5) Each owning concurrency rulebook defines whether its wait operation is cancellation-aware.

---

## § 16. Task migration

**Governance tags:** `concurrency.scheduling-v2`, `analysis.scheduling-v2`

§ 16(1) A logical task may resume on a different physical thread after a scheduling point when no active semantic restriction prevents migration.

§ 16(2) Task identity remains unchanged across migration.

§ 16(3) `TaskContext` follows the logical task.

§ 16(4) `ThreadContext` follows the physical thread and may therefore differ before and after migration.

§ 16(5) Owned task-local values move with the logical task continuation/frame according to ordinary ownership semantics.

---

## § 17. No portable task affinity

**Governance tags:** `concurrency.scheduling-v2`

§ 17(1) Sec 0.1 defines no portable task-affinity API.

§ 17(2) Thread affinity does not imply task affinity.

§ 17(3) A task running on an affinity-constrained executor worker has no source-level stable physical-thread identity merely because of that runtime choice.

§ 17(4) A future task-affinity feature must define exactly whether it means non-migration, executor affinity, worker-set affinity, CPU affinity, or a preference.

---

## § 18. Native preemption is not automatically a migration point

**Governance tags:** `concurrency.scheduling-v2`, `analysis.borrowing`

§ 18(1) A backend/native scheduler may physically preempt execution at implementation-defined machine boundaries.

§ 18(2) Such preemption is not automatically a source-level task migration point.

§ 18(3) If compiler analysis has allowed a physical-thread-bound capability or reference to remain live, the backend must preserve the physical-thread identity required by that proof until the value's lifetime ends.

§ 18(4) A backend must not resume a task on another physical thread across an unmodeled point and thereby invalidate a Sec borrow/provenance guarantee.

§ 18(5) Backend scheduling freedom is constrained by validated semantic facts.

---

## § 19. Thread-local references and scheduling

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.thread-local-v2`

§ 19(1) A reference returned by `ThreadLocal[T].Borrow()` or `BorrowMut()` is bound to one physical-thread attachment generation.

§ 19(2) By default it must not remain live across a task scheduling point that may migrate the task.

§ 19(3) This includes `Task.Yield()`.

§ 19(4) Physical-thread blocking/parking alone does not invalidate such a reference when the same physical-thread identity is guaranteed on resume.

§ 19(5) Scheduler lowering must preserve the borrow-analysis result.

---

## § 20. Physical thread preemption

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.thread-v2`

§ 20(1) Sec does not promise cooperative-only physical threads.

§ 20(2) The selected native scheduler may preempt a physical thread at permitted machine-level points.

§ 20(3) Such preemption does not change its Sec `ThreadID`.

§ 20(4) Ordinary data-race and memory-model rules remain necessary because two physical threads may execute simultaneously.

---

## § 21. Main has physical thread context

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.thread-v2`

§ 21(1) Ordinary program entry executes with an attached physical Sec thread context.

§ 21(2) Therefore:

```sec
Thread.Current()
```

is valid in ordinary `main`.

§ 21(3) The physical main thread may be implemented by any target mechanism that preserves canonical `ThreadContext` semantics.

---

## § 22. No implicit root task in main

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.tasks-v2`

§ 22(1) Ordinary program entry does not implicitly create a logical root task.

§ 22(2) Therefore, unless source execution is inside an explicitly created/entered real task:

```sec
Task.Current()
```

is invalid.

§ 22(3) Likewise:

```sec
Task.Yield()
```

is invalid outside a real task context.

§ 22(4) A backend may internally execute `main` using machinery shared with its task runtime, but that implementation detail does not create source-visible `TaskContext`.

§ 22(5) Target choice must not make identical ordinary `main` source gain or lose implicit task identity.

---

## § 23. Main example

**Governance tags:** `concurrency.scheduling-v2`

§ 23(1) Canonical ordinary entry behavior is:

```sec
fn main() {
    let physical := Thread.Current()

    // Invalid: main is not implicitly a logical task.
    // let logical := Task.Current()

    // Invalid for the same reason.
    // Task.Yield()
}
```

§ 23(2) A real task context begins only through a canonical task-execution path.

---

## § 24. `Task.Current()`

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.tasks-v2`

§ 24(1) The exact task-current surface is:

```sec
impl Task {
    static fn Current() TaskContext
}
```

§ 24(2) `Task.Current()` refers to the current logical task.

§ 24(3) It must never be synthesized merely because the current physical thread is an executor worker capable of running tasks.

§ 24(4) It remains the same logical context across permitted task migration.

---

## § 25. `Thread.Current()`

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.thread-v2`

§ 25(1) The exact physical-current surface is:

```sec
impl Thread {
    static fn Current() ThreadContext
}
```

§ 25(2) `Thread.Current()` refers to the current physical/native Sec thread context.

§ 25(3) When a logical task is running on an executor worker, `Thread.Current()` identifies that physical worker, not the logical task.

§ 25(4) If the task migrates later, a subsequent `Thread.Current()` may return a different physical identity.

---

## § 26. Task scheduling and TLS identity

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.thread-local-v2`

§ 26(1) `ThreadLocal[T]` resolves against the current physical thread at each access.

§ 26(2) Two accesses by one logical task separated by a migratable scheduling point may resolve to different TLS values.

§ 26(3) Code requiring state stable across logical task migration must use task-owned state or another suitable mechanism rather than physical TLS.

---

## § 27. Blocking operation in task context

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.blocking-v1`

§ 27(1) A potentially blocking operation executed by a logical task should suspend the logical task rather than block its physical executor worker when the selected backend can register readiness.

§ 27(2) A profile may allow physical worker blocking where no suspension integration exists.

§ 27(3) Such blocking must be visible to blocking/starvation analysis and target profile declarations.

§ 27(4) The source-level operation semantics do not change merely because one backend suspends and another blocks.

---

## § 28. Waiting operation in physical thread context

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.blocking-v1`

§ 28(1) A waiting operation executed directly by a physical thread may park/block that physical thread.

§ 28(2) It does not create a logical task context.

§ 28(3) Resumption remains in the same physical thread identity unless the operation's execution-kind rule explicitly says otherwise.

---

## § 29. Scheduler fairness

**Governance tags:** `concurrency.scheduling-v2`

§ 29(1) Sec 0.1 provides no universal fairness guarantee for logical tasks or physical threads.

§ 29(2) A target/runtime profile may document a stronger guarantee.

§ 29(3) The implementation must not claim source-level fairness merely because it internally uses round-robin queues, OS scheduling, priorities, work stealing, `Task.Yield()`, or `Thread.Yield()`.

§ 29(4) Correct portable program semantics must not require an unstated fairness property.

---

## § 30. Starvation

**Governance tags:** `concurrency.scheduling-v2`, `sema.deadlock-analysis`

§ 30(1) Starvation may arise from source-order `select`, scheduler policy, thread priority, CPU affinity, lock contention, executor worker exhaustion, cooperative tasks that never reach scheduling points, resource monopolization, or native scheduler behavior.

§ 30(2) Starvation is not generally decidable.

§ 30(3) Analysis should diagnose statically evident high-risk patterns when supported.

---

## § 31. Source-order select remains authoritative

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.select-v2`

§ 31(1) Scheduler fairness must not override source-order `select` semantics.

§ 31(2) If several select branches are ready at one selection point, the first ready source branch wins according to `select.md`.

§ 31(3) Native notification order or worker scheduling order must not reorder that semantic priority.

---

## § 32. Portable thread priority

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.thread-v2`

§ 32(1) The exact portable priority type is owned by `threads.md`:

```sec
enum ThreadPriority {
    Lowest
    Low
    Normal
    High
    Highest
}
```

§ 32(2) Values are relative portable intents.

§ 32(3) They are not native numeric priorities.

§ 32(4) `Highest` does not promise a real-time scheduler class.

---

## § 33. Thread priority configuration

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.thread-v2`

§ 33(1) Creation-time priority is expressed through `ThreadConfig.Priority` with exact type:

```sec
ThreadSetting[ThreadPriority]
```

§ 33(2) Examples:

```sec
Priority: Preferred(ThreadPriority.High)
```

```sec
Priority: Required(ThreadPriority.High)
```

§ 33(3) A plain `ThreadPriority.High` does not implicitly mean `Preferred(...)`.

§ 33(4) Required unsupported priority is a target-validation error.

§ 33(5) Preferred unsupported priority follows the diagnostic/fallback policy from `threads.md`.

---

## § 34. No portable runtime priority mutation

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.thread-v2`

§ 34(1) Sec 0.1 defines no portable `Thread[T].SetPriority(...)`.

§ 34(2) Consequently scheduling does not own a portable `ThreadSchedulingError`.

§ 34(3) A target-specific runtime-priority operation requires a fully specified platform-owned API and error contract.

---

## § 35. Portable CPU affinity

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.thread-v2`

§ 35(1) Portable physical-thread affinity uses compiler-known:

```sec
type CpuSet
```

§ 35(2) Creation-time affinity is expressed through `ThreadConfig.Affinity` with type:

```sec
ThreadSetting[CpuSet]
```

§ 35(3) `CpuSet` is not `set[uint]`.

§ 35(4) Compile-time-known CPU indices are validated against the selected target/variant when that topology is fixed.

---

## § 36. No portable runtime affinity mutation

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.thread-v2`

§ 36(1) Sec 0.1 defines no portable `Thread[T].SetAffinity(...)`.

§ 36(2) Runtime affinity mutation, when supported, belongs to a complete target/platform API.

§ 36(3) Runtime affinity support does not imply portable task affinity.

---

## § 37. Priority inversion

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.mutex-v2`

§ 37(1) Thread priority does not alter mutex ownership semantics.

§ 37(2) A target/runtime may support priority inheritance, priority ceiling, or another policy only when declared by the applicable mutex/scheduling platform contract.

§ 37(3) Sec 0.1 does not silently insert priority inheritance into every mutex.

§ 37(4) Analysis may diagnose known high-risk priority inversion patterns where sufficient target information exists.

---

## § 38. Cancellation wakeups

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.cancellation-v2`

§ 38(1) A cancellation request may wake an execution waiting in a cancellation-aware operation.

§ 38(2) Such a wakeup does not itself mean that terminal cancellation has committed.

§ 38(3) The operation's cancellation-point commit rule determines whether cancellation wins.

§ 38(4) Required cleanup occurs before terminal cancellation completes.

§ 38(5) Unsafe hard termination is not scheduling and is not cooperative cancellation.

---

## § 39. Runnable does not mean running

**Governance tags:** `concurrency.scheduling-v2`

§ 39(1) A runnable task/thread is eligible to execute.

§ 39(2) The language does not guarantee immediate CPU service.

§ 39(3) Scheduler queues, target core count, priorities, affinity, and contention may delay execution.

§ 39(4) Portable code must not use "runnable" as a synchronization condition.

---

## § 40. Status versus scheduler-private state

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.tasks-v2`, `concurrency.thread-v2`

§ 40(1) Backends may use private states such as queued, parked, sleeping, waiting-IO, stealable, or running-on-worker-N.

§ 40(2) Such states are not automatically portable task/thread status values.

§ 40(3) Only states explicitly standardized by the owning rulebook are source-visible.

---

## § 41. No scheduling synchronization edge

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.memory-model-v2`

§ 41(1) Merely yielding, being preempted, becoming runnable, changing queue, or changing worker does not create a memory synchronization edge.

§ 41(2) `Task.Yield()` is not a memory fence.

§ 41(3) `Thread.Yield()` is not a memory fence.

§ 41(4) Synchronization comes from the selected canonical synchronization operation, atomic ordering, completion edge, channel transfer, lock, or other memory-model rule.

---

## § 42. Migration preserves logical semantics

**Governance tags:** `concurrency.scheduling-v2`

§ 42(1) Permitted task migration must preserve `TaskID`, `TaskContext`, cancellation state, ownership state, continuation state, language-visible task state, valid task-owned borrows, and task result/error/panic semantics.

§ 42(2) Migration may change current `ThreadID`, physical thread-local values, native worker metadata, and native CPU.

§ 42(3) A backend must not expose migration as an unexpected ownership copy/move event.

---

## § 43. Execution-affine capabilities

**Governance tags:** `concurrency.scheduling-v2`, `analysis.borrowing`

§ 43(1) Some values are valid only on one physical execution identity, including thread-bound TLS borrows and `ThreadAttachment`.

§ 43(2) While such a value is live, the compiler/runtime must either prove the logical task cannot migrate or reject a scheduling point that may migrate the task.

§ 43(3) The runtime must not "fix" an invalid migration by silently copying an execution-affine capability.

---

## § 44. ISR scheduling restrictions

**Governance tags:** `concurrency.scheduling-v2`, `analysis.isr-v1`

§ 44(1) ISR execution is not an ordinary schedulable task/thread context.

§ 44(2) ISR code must not call:

```sec
Task.Yield()
Thread.Yield()
```

§ 44(3) ISR code must not perform ordinary await, join, blocking select, blocking channel operations, or ordinary blocking mutex acquisition.

§ 44(4) Deferred work must cross into a schedulable context through an ISR-safe canonical primitive.

---

## § 45. Bare-metal and no-required-runtime targets

**Governance tags:** `concurrency.scheduling-v2`, `compiler.platform-model`

§ 45(1) A bare-metal target may provide one main physical context, interrupt handlers, a statically generated task executor, and no native thread creation.

§ 45(2) Such a target may support `Task[T]` while rejecting `spawn thread`.

§ 45(3) Another target may support physical threads while rejecting tasks.

§ 45(4) Task and thread capabilities are independent target facts.

§ 45(5) The compiler must not infer capability from the build host.

---

## § 46. Scheduler resource model

**Governance tags:** `concurrency.scheduling-v2`, `compiler.platform-model`

§ 46(1) A target/runtime profile may specify bounded scheduler resources such as maximum runnable tasks, executor worker count, static task slots, wait-registration capacity, timer registrations, and queue capacity.

§ 46(2) Statically impossible configurations are compile-time diagnostics.

§ 46(3) Runtime exhaustion already assigned an owning typed error remains owned by that operation's rulebook.

§ 46(4) Scheduling does not invent one universal `SchedulingError`.

---

## § 47. CompilationPlan

**Governance tags:** `concurrency.scheduling-v2`, `compiler.platform-model`

§ 47(1) The selected immutable `CompilationPlan` is the source of target scheduling truth.

§ 47(2) Relevant facts include task runtime availability, physical-thread availability, migration support, task suspension support, worker blocking policy, scheduler resource limits, timer/wait integration, priority support, affinity support, deferred physical-thread start, ISR restrictions, and no-allocation/no-blocking constraints.

§ 47(3) Host-machine scheduler behavior must not influence target semantics accidentally.

---

## § 48. Static scheduling analysis

**Governance tags:** `concurrency.scheduling-v2`, `analysis.scheduling-v2`

§ 48(1) Sema/analysis should record at least current execution kind, real task-context availability, scheduling points, cancellation points, potential task migration, physical-thread identity requirements, physical blocking, requested priority/affinity, Preferred/Required classification, deferred thread start, ISR context, target scheduler capabilities, and resource bounds where known.

§ 48(2) Analysis must distinguish task suspension from physical-thread preemption.

§ 48(3) Analysis must distinguish logical-task cancellation from physical-thread scheduler hints.

---

## § 49. Cooperative-progress diagnostics

**Governance tags:** `concurrency.scheduling-v2`, `analysis.scheduling-v2`

§ 49(1) On a cooperative task runtime, analysis may diagnose a statically evident unbounded task loop with no reachable scheduling/cancellation point.

§ 49(2) The diagnostic is target/profile sensitive.

§ 49(3) A preemptive task profile may not have the same progress hazard, but cancellation responsiveness may still be poor if no cancellation point is reached.

§ 49(4) Diagnostics must not claim general starvation freedom.

---

## § 50. Task yield and effect analysis

**Governance tags:** `concurrency.scheduling-v2`, `concurrency.cancellation-v2`

§ 50(1) Calling `Task.Yield()` requires a real logical task context.

§ 50(2) Because it is a cancellation point, analysis must include the current-task cancellation cleanup edge required by canonical cancellation-point rules.

§ 50(3) No user-written `@cancellable` annotation is introduced.

§ 50(4) The requirement/effect is compiler inferred according to `cancellation.md`.

---

## § 51. Thread yield and effect analysis

**Governance tags:** `concurrency.scheduling-v2`

§ 51(1) Calling `Thread.Yield()` requires a valid physical Sec thread context.

§ 51(2) It does not add a cancellation point merely because the current physical thread may have `CancelRequested == true`.

§ 51(3) It adds no synchronization effect.

§ 51(4) ISR analysis rejects it.

---

## § 52. Semantic IR requirements

**Governance tags:** `semantic-ir.scheduling-v2`

§ 52(1) Semantic IR must preserve scheduling semantics explicitly enough that lowering does not infer them from arbitrary function names.

§ 52(2) Required facts include at least:

- logical-task versus physical-thread execution kind;
- current task/thread context availability;
- task scheduling points;
- task migration permission/restriction;
- task-yield cancellation-point commit state;
- physical thread yield hint;
- execution-affine live values;
- target scheduler capability requirements;
- priority/affinity intent;
- deferred physical-thread start;
- physical blocking versus task suspension;
- source provenance.

§ 52(3) Concrete opcode/instruction names remain owned by `semantic_ir.md`.

§ 52(4) The legacy literal opcode list from the previous scheduling rulebook is not normative.

---

## § 53. IR verification

**Governance tags:** `semantic-ir.scheduling-v2`

§ 53(1) IR verification must reject a state where:

- `Task.Yield()` appears outside a task context;
- one task-yield operation both returns normally and commits pre-return cancellation;
- a task migrates while a live execution-affine value forbids migration;
- a backend task operation silently becomes a thread operation or vice versa;
- `Thread.Yield()` is modeled as a memory fence or cancellation point;
- ordinary main is given an implicit task context without an explicit language rule.

---

## § 54. Lowering

**Governance tags:** `lowering.scheduling-v2`, `compiler.platform-model`

§ 54(1) Lowering consumes validated scheduling Semantic IR plus the selected `CompilationPlan`.

§ 54(2) `Task.Yield()` may lower to executor requeue, continuation suspension/resume, cooperative scheduler yield, a target task-yield facility, or an equivalent operation preserving cancellation commit semantics.

§ 54(3) `Thread.Yield()` may lower to the target/native physical-thread yield primitive or an equivalent no-guarantee hint.

§ 54(4) Lowering must not implement `Task.Yield()` merely as `Thread.Yield()` when doing so loses logical task scheduling/cancellation semantics.

§ 54(5) Lowering must not fabricate a logical task context for `main`.

---

## § 55. LSP hover

**Governance tags:** `tooling.scheduling-v2`

§ 55(1) Hover for `Task.Yield()` must show:

```sec
static fn Yield() void
```

and document that it operates on the current logical task, is a scheduling point, is a cancellation point, and creates no synchronization edge.

§ 55(2) Hover for `Thread.Yield()` must show the same function type but document that it operates on the current physical thread, is a scheduler hint, is not a cancellation point, and creates no synchronization edge.

§ 55(3) Hover must not imply a task context exists in ordinary `main`.

---

## § 56. Completion and navigation

**Governance tags:** `tooling.scheduling-v2`

§ 56(1) Completion may offer `Task.Yield()` only where a task context is semantically available, or may offer it context-independently only if choosing it yields the precise context diagnostic.

§ 56(2) Completion/navigation for `ThreadPriority`, `CpuSet`, and thread configuration resolves to their canonical declarations owned by `threads.md`.

§ 56(3) Tooling must not expose stale portable `ThreadSchedulingError`.

---

## § 57. Diagnostics

**Governance tags:** `tooling.scheduling-v2`, `frontend.scheduling-v2`

§ 57(1) Suggested diagnostics include:

```text
Task.Yield() requires a logical task context
```

```text
Task.Current() requires a logical task context; main does not create one implicitly
```

```text
task may migrate to another physical thread after this scheduling point
```

```text
thread-local reference cannot remain live across Task.Yield()
```

```text
Thread.Yield() is not permitted in an interrupt routine
```

```text
required thread priority is not supported by the selected target
```

```text
required CPU affinity is not supported by the selected target
```

```text
cooperative task loop has no reachable scheduling or cancellation point
```

§ 57(2) Diagnostics must distinguish compiler support, target support, invalid execution context, migration/lifetime failure, and scheduler-policy warnings.

---

## § 58. Restrictions

**Governance tags:** `concurrency.scheduling-v2`

§ 58(1) Sec 0.1 scheduling must not:

- make tasks semantically identical to threads;
- make threads semantically identical to tasks;
- give ordinary `main` an implicit root task;
- permit `Task.Current()` outside a real task;
- permit `Task.Yield()` outside a real task;
- treat `Thread.Yield()` as a cancellation point;
- treat either yield as a memory fence;
- promise fairness without a profile guarantee;
- infer stable physical-thread identity for migratable tasks;
- migrate a task across an unmodeled point that invalidates execution-affine values;
- silently downgrade Required priority/affinity;
- invent a portable runtime `ThreadSchedulingError`;
- expose backend-private scheduler states as portable language semantics.

---

## § 59. Explicitly absent Sec 0.1 features

**Governance tags:** `concurrency.scheduling-v2`

§ 59(1) Sec 0.1 does not define:

```text
implicit root TaskContext for main
portable task affinity
portable task priority
portable runtime Thread.SetPriority
portable runtime Thread.SetAffinity
universal scheduler fairness
universal SchedulingError
Thread.Yield cancellation semantics
yield-as-memory-fence semantics
user-selectable scheduler algorithm API
```

---

## § 60. Conformance scenarios

**Governance tags:** `concurrency.scheduling-v2`

§ 60(1) Conformance tests must include at least:

- task and thread execution kinds remain distinct;
- `Task.Yield()` exact source surface;
- `Task.Yield()` valid in task context;
- `Task.Yield()` rejected outside task context;
- `Task.Yield()` is a cancellation point;
- cancellation winning `Task.Yield()` does not return to following statement;
- normal `Task.Yield()` returns `void`;
- `Task.Yield()` creates no synchronization edge;
- `Thread.Yield()` exact source surface;
- `Thread.Yield()` is not a cancellation point;
- `Thread.Yield()` creates no synchronization edge;
- `Thread.Yield()` rejected in ISR;
- main has `ThreadContext`;
- main has no implicit `TaskContext`;
- `Task.Current()` rejected in ordinary main;
- backend internal task machinery for main does not change source semantics;
- task migration preserves `TaskContext` and may change `ThreadContext`;
- TLS borrow rejected across migratable `Task.Yield()`;
- native preemption does not violate thread-bound borrow proof;
- physical-thread blocking may retain TLS borrow on same thread;
- no portable task affinity;
- exact `ThreadPriority` dependency;
- exact `CpuSet` dependency;
- no implicit priority preference conversion;
- Required priority/affinity target failure;
- no portable `ThreadSchedulingError`;
- source-order select priority not overridden by scheduler;
- cooperative no-yield loop diagnostic where profile requires it;
- Semantic IR distinguishes `Task.Yield()` and `Thread.Yield()`;
- LSP documents cancellation distinction.

---

## § 61. Cross-rulebook synchronization

**Governance tags:** `concurrency.scheduling-v2`

§ 61(1) `tasks.md` must add the exact `Task.Yield()` surface and state that it is valid only inside a real task context and is a cancellation point.

§ 61(2) `tasks.md` must preserve that `Task.Current()` is never synthesized merely from a physical worker thread.

§ 61(3) `threads.md` must preserve `Thread.Yield()` as a physical scheduler hint and explicitly state that it is not a Sec cancellation point.

§ 61(4) `thread_local.md` must list `Task.Yield()` among migratable task scheduling points across which TLS borrows are rejected by default.

§ 61(5) `cancellation.md` must include `Task.Yield()` among current-task cancellation points and must not include `Thread.Yield()` unless a future revision changes that decision.

§ 61(6) `blocking.md` must distinguish logical task suspension from physical-thread blocking/parking.

§ 61(7) `select.md` source-order priority remains authoritative regardless of scheduler/native notification order.

§ 61(8) Any legacy wording saying ordinary program entry may or may not have a task context depending on target must be removed.

§ 61(9) References to retired `structured_concurrency.md` should be removed where they merely duplicate lifecycle ownership rules; that book is Covered rather than an active scheduling owner.

§ 61(10) `semantic_ir.md` remains owner of concrete IR vocabulary.

---

## § 62. Governance

**Governance tags:** `concurrency.scheduling-v2`, `frontend.scheduling-v2`, `analysis.scheduling-v2`, `semantic-ir.scheduling-v2`, `lowering.scheduling-v2`, `tooling.scheduling-v2`, `compiler.platform-model`

§ 62(1) `governance/concurrency_scheduling.yaml` is the sole canonical implementation-status owner for the portable scheduling integration defined by this revision.

§ 62(2) Task/thread/TLS/select/cancellation-specific APIs and lifecycle rules remain owned by their canonical governance fragments and are cross-linked rather than duplicated as competing integrations.

§ 62(3) Root `implementation-status.yaml` is not a second canonical ledger.

§ 62(4) Cross-rulebook synchronization required by this revision is tracked by the accompanying scheduling-v2 correction.

§ 62(5) After synchronization is applied, the correction belongs under `rules/corrections/applied/`.
