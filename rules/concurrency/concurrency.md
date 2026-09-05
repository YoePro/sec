# Concurrency

- **Status:** Normative
- **Created:** 2026-09-04
- **Last updated:** 2026-09-04
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/concurrency.md`
- **Replaces:** Earlier unversioned revision at the same canonical path
- **Repository baseline reviewed:** `777beb8`
- **Related rulebooks:** `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/concurrency/spawn.md`, `rules/concurrency/await.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/mutex.md`, `rules/concurrency/atomics.md`, `rules/concurrency/channels.md`, `rules/concurrency/select.md`, `rules/concurrency/scheduling.md`, `rules/concurrency/structured_concurrency.md`, `rules/concurrency/concurrency_runtime_model.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/concurrency/thread_local.md`, `rules/concurrency/processes.txt`, `rules/concurrency/ipc.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/transferability.md`, `rules/memory/destruction.md`, `rules/declarations/static.md`, `rules/analysis/data_races.md`, `rules/analysis/deadlock_analysis.md`, `rules/compiler/semantic_ir.md`, `rules/platform/target_profiles.md`, `rules/platform/platform_model.md`, `rules/platform/ffi.md`

---

## § 1. Purpose and authority

**Governance tags:** `concurrency.model-v2`

§ 1(1) This rulebook defines the common language-level concurrency model of Sec.

§ 1(2) It establishes invariants shared across tasks, threads, processes, synchronization primitives, shared state, message passing, cancellation, memory visibility, analysis, and lowering.

§ 1(3) Specialized concurrency rulebooks own the detailed syntax and semantics of their individual constructs.

§ 1(4) This rulebook must not duplicate specialized rules when a more specific canonical rulebook owns the detail.

§ 1(5) When a specialized concurrency rulebook and this overview address the same semantic detail, the specialized rulebook is authoritative for that detail unless an explicit correction states otherwise.

§ 1(6) Common invariants defined here remain binding on all specialized concurrency constructs.

§ 1(7) Mutable implementation status, current backend coverage, current test coverage, and temporary implementation limitations do not belong in this normative rulebook.

§ 1(8) Mutable implementation status is tracked through `implementation-status.yaml`.

---

## § 2. Core principles

**Governance tags:** `concurrency.model-v2`, `frontend.transferability`

§ 2(1) Sec concurrency is based on explicit execution creation, deterministic ownership, compile-time validation, explicit synchronization, typed failure, and target-independent source semantics.

§ 2(2) Concurrency must not weaken ordinary ownership rules.

§ 2(3) Concurrency must not weaken ordinary borrowing rules.

§ 2(4) Concurrency must not weaken deterministic destruction rules.

§ 2(5) Concurrency must not create hidden shared mutable state.

§ 2(6) Concurrency must not require garbage collection.

§ 2(7) Concurrency must not require runtime ownership tracking.

§ 2(8) Concurrency must not make data races valid.

§ 2(9) Concurrency must not silently detach work.

§ 2(10) Concurrency must not silently replace one execution kind with another execution kind when their semantics differ.

§ 2(11) Concurrency semantics must remain valid on hosted, RTOS, and bare-metal targets where the selected target profile provides the required capabilities.

§ 2(12) A target that cannot implement a required concurrency semantic must reject the program or configuration rather than silently change the program's meaning.

---

## § 3. Terminology

**Governance tags:** `concurrency.model-v2`

§ 3(1) An **execution entity** is a logical or physical execution context that may make progress independently of another execution context.

§ 3(2) A **task** is a logical Sec execution entity governed by `tasks.md`.

§ 3(3) A **thread** is an explicit physical target execution context governed by `threads.md`.

§ 3(4) A **process** is a distinct execution kind governed by the process and IPC rulebooks.

§ 3(5) A **boundary transfer** is movement, copying, borrowing, serialization, publication, or another defined transfer of a value or capability across an execution boundary.

§ 3(6) **Shared state** is storage that may be accessed by more than one potentially overlapping execution context.

§ 3(7) **Synchronization** is a language-recognized mechanism that establishes the access, ordering, or progress relationship required by the owning synchronization rule.

§ 3(8) **Publication** is the operation or synchronization event that makes a value or memory effect visible to another execution context according to the concurrency memory model.

§ 3(9) **Blocking** means an execution context cannot make further progress until some condition is satisfied.

§ 3(10) **Suspension** means a logical execution entity yields execution while retaining resumable state.

§ 3(11) Blocking and suspension are not synonymous.

§ 3(12) **Cancellation** is cooperative termination control as defined by `cancellation.md`; it is not arbitrary forced execution termination.

---

## § 4. Concurrency and parallelism

**Governance tags:** `concurrency.model-v2`, `compiler.platform-model`

§ 4(1) Concurrency means multiple execution entities may make progress independently.

§ 4(2) Parallelism means multiple execution entities execute physically at the same time.

§ 4(3) Sec does not equate concurrency with parallelism.

§ 4(4) A target may support concurrent tasks on one physical thread.

§ 4(5) A target may support parallel execution through multiple physical threads or execution units.

§ 4(6) A task does not imply one dedicated physical thread.

§ 4(7) A thread does imply the physical-thread execution semantics required by `threads.md`.

§ 4(8) A scheduler may migrate a task between physical workers when the task and target rules permit migration.

§ 4(9) Task migration must not change logical task identity, ownership, cancellation state, or language-visible task semantics.

§ 4(10) Physical execution strategy is a lowering/runtime decision constrained by the selected `CompilationPlan`.

---

## § 5. Execution kinds remain distinct

**Governance tags:** `concurrency.model-v2`, `semantic-ir.transferability`

§ 5(1) Tasks, threads, processes, interrupt contexts, and foreign callback contexts are distinct execution kinds.

§ 5(2) Their distinction must survive semantic analysis when it affects ownership, lifetime, scheduling, blocking, affinity, transferability, synchronization, or target requirements.

§ 5(3) A backend must not silently lower `spawn thread` to ordinary task creation.

§ 5(4) A backend must not silently model a process boundary as an in-process task or thread boundary.

§ 5(5) A compiler must not infer that a foreign callback executes in the caller's execution context unless the FFI contract proves that property.

§ 5(6) Interrupt execution must not be treated as an ordinary task or thread merely to reuse concurrency implementation machinery.

§ 5(7) Common APIs may use harmonized concepts such as identity, status, observation, waiting, or cancellation where the specialized rulebooks define them.

§ 5(8) Harmonized concepts do not erase execution-kind-specific semantics.

---

## § 6. Explicit execution creation

**Governance tags:** `concurrency.model-v2`, `frontend.discard-v2`

§ 6(1) Sec does not create asynchronous execution merely because a function is called.

§ 6(2) New task or thread execution requires explicit creation syntax defined by `spawn.md`.

§ 6(3) Process creation, where supported, must use the explicit process-creation semantics defined by its owning rulebook.

§ 6(4) The compiler must not silently convert an ordinary synchronous call into a spawned execution entity.

§ 6(5) The compiler must not silently detach a created execution entity.

§ 6(6) Spawn failure is a language-visible failure when the owning spawn rule defines creation as fallible.

§ 6(7) Lack of an execution capability in the selected target profile is a compile-time target/capability failure when the relevant rulebook defines that capability as unavailable rather than a runtime creation failure.

§ 6(8) Creation syntax must preserve ordinary argument evaluation, ownership, failure, and cleanup semantics.

---

## § 7. Task model summary

**Governance tags:** `concurrency.model-v2`, `concurrency.tasks-v2`, `frontend.discard-v2`

§ 7(1) `Task[T]` is the compiler-known owning handle for a logical task returning `T`.

§ 7(2) Task creation is fallible.

§ 7(3) The canonical raw type of ordinary task creation is:

```sec
Result[Task[T], TaskSpawnError]
```

§ 7(4) Typical propagation therefore uses:

```sec
let worker := try spawn Work()
```

§ 7(5) `Task[T]` is move-only regardless of the copy classification of `T`.

§ 7(6) An owning task handle carries a lifecycle obligation.

§ 7(7) Awaiting a task consumes the owning task handle according to `tasks.md`.

§ 7(8) The static type of awaiting a task is always:

```sec
TaskOutcome[T]
```

§ 7(9) The compiler must not change the static result type of `await Task[T]` to `T` because normal completion can be proven.

§ 7(10) The canonical task terminal outcome distinguishes:

```sec
type TaskOutcome[T] union {
    Completed(T)
    Cancelled
    Panicked(PanicInfo)
    Failed(TaskError)
}
```

§ 7(11) The task function's complete declared return type remains inside `Completed`.

§ 7(12) Therefore:

```sec
Task[Result[Image, IOError]]
```

awaits to:

```sec
TaskOutcome[Result[Image, IOError]]
```

§ 7(13) `Completed(Err(error))` is normal task execution completion and must not be reclassified as `Failed(TaskError)`.

§ 7(14) Detailed task lifecycle, observer, join, detach, task-error, and outcome semantics are owned by `tasks.md`.

---

## § 8. Thread model summary

**Governance tags:** `concurrency.model-v2`, `frontend.transferability`, `platform.transferability`

§ 8(1) `Thread[T]` represents an explicit physical target execution context returning `T`.

§ 8(2) Thread creation is distinct from task creation.

§ 8(3) Thread creation is fallible where `threads.md` defines native-thread creation failure.

§ 8(4) A target without physical-thread capability must reject explicit physical-thread creation according to the target/profile rules.

§ 8(5) Thread affinity, thread-local storage, physical stack behavior, and native scheduling properties must remain distinguishable from logical task semantics.

§ 8(6) A task that happens to execute on one physical thread does not thereby own that thread's thread-local identity.

§ 8(7) A task that may migrate must not retain thread-affine references or capabilities unless the scheduling and transferability rules prove the use valid.

§ 8(8) Detailed `Thread[T]`, `ThreadConfig`, thread status, thread observer, join, detach, cancellation, affinity, and physical-thread semantics are owned by `threads.md`.

---

## § 9. Process boundaries

**Governance tags:** `concurrency.model-v2`, `analysis.transferability`, `platform.transferability`

§ 9(1) Process execution is a distinct concurrency boundary.

§ 9(2) This overview does not define process creation syntax, process result shape, process lifecycle APIs, or process termination policy.

§ 9(3) Those semantics belong to the process and IPC rulebooks.

§ 9(4) Cross-process transfer must not be modeled as ordinary in-process reference or pointer transfer unless an explicit shared-memory or process adapter contract defines that representation.

§ 9(5) Process transfer may require serialization, IPC, shared memory, handle duplication, target-specific transfer, or another explicit adapter defined by the owning rules.

§ 9(6) `ref T`, `ref mut T`, `RawPtr[T]`, in-process addresses, and process-local handles must not be assumed transferable merely because their machine representation is copyable.

§ 9(7) Process transferability is a semantic property, not a bit-copy property.

§ 9(8) This section intentionally establishes only the common cross-boundary invariant and does not complete the currently unfinished process rulebook.

---

## § 10. Ownership confinement

**Governance tags:** `concurrency.model-v2`, `frontend.transferability`, `analysis.transferability`

§ 10(1) Exclusive ownership is the preferred default mechanism for mutable concurrent state.

§ 10(2) A value exclusively owned by one execution entity does not require synchronization merely because other execution entities exist.

§ 10(3) Moving exclusive ownership to another execution entity transfers responsibility for the owned value.

§ 10(4) Exclusive ownership transfer is not simultaneous sharing.

§ 10(5) A move-only source becomes unavailable when the canonical ownership transfer commits.

§ 10(6) Reusable move-only sources use the explicit move syntax required by `ownership.md`.

Conceptual example:

```sec
let data := Data.Create()
let worker := try spawn Consume(<-data)
```

§ 10(7) The compiler must not silently clone a move-only value to make concurrent code compile.

§ 10(8) The compiler must not silently heap-promote an invalid borrowed value to make it survive an execution boundary.

§ 10(9) Ownership state across execution boundaries must remain explicit in Semantic IR.

§ 10(10) Destruction responsibility must follow ownership transfer.

---

## § 11. Transferability

**Governance tags:** `frontend.transferability`, `analysis.transferability`, `semantic-ir.transferability`, `lowering.transferability`, `platform.transferability`

§ 11(1) Ordinary type assignability is not sufficient to prove that a value may cross every execution boundary.

§ 11(2) The compiler must validate boundary-specific transferability.

§ 11(3) Task transferability, thread transferability, process transferability, ISR transferability, and concurrent shareability may impose different requirements on the same type or value.

§ 11(4) Transferability analysis must account for ownership state.

§ 11(5) Transferability analysis must account for active borrows.

§ 11(6) Transferability analysis must account for lifetime and escape.

§ 11(7) Transferability analysis must account for thread affinity and task migration.

§ 11(8) Transferability analysis must account for allocation/storage domain restrictions.

§ 11(9) Transferability analysis must account for destruction-context restrictions.

§ 11(10) Transferability analysis must account for closure captures.

§ 11(11) Transferability analysis must account for FFI handle and callback contracts.

§ 11(12) Transferability analysis must account for process adapters where process boundaries are involved.

§ 11(13) Unsafe code does not waive transferability requirements that remain semantically necessary for a valid target boundary.

§ 11(14) Detailed proof rules are owned by `transferability.md`.

---

## § 12. Shared immutable access

**Governance tags:** `concurrency.model-v2`, `frontend.transferability`, `sema.data-race-analysis`

§ 12(1) Shared immutable access may be valid across concurrent execution contexts.

§ 12(2) Lifetime must remain valid for every concurrent use.

§ 12(3) The type and storage must permit the required form of concurrent sharing.

§ 12(4) Publication must make the value visible to the destination execution context according to the concurrency memory model.

§ 12(5) No conflicting mutation may occur without the synchronization required by the memory model.

§ 12(6) A shared reference is not automatically thread-safe merely because `ref T` is copyable as a reference value.

§ 12(7) A reference into thread-affine, task-local, temporary, movable, or otherwise constrained storage may be invalid across a concurrency boundary.

§ 12(8) The compiler must use canonical lifetime, Place/provenance, transferability, and execution-context facts when validating shared immutable access.

---

## § 13. Shared mutable access

**Governance tags:** `concurrency.model-v2`, `sema.data-race-analysis`, `frontend.transferability`

§ 13(1) Shared mutable state requires an explicit concurrency-safe synchronization or ownership mechanism.

§ 13(2) Two overlapping ordinary mutable borrows must not be created for concurrent use.

§ 13(3) An ordinary `ref mut T` does not become a concurrent synchronization primitive.

§ 13(4) A mutable reference may cross a boundary only when its exclusive access, lifetime, transferability, execution context, and storage validity are all proven.

§ 13(5) Concurrent mutation of the same location without the required synchronization is invalid.

§ 13(6) Valid shared mutation mechanisms may include:

- `Mutex[T]`,
- atomic types and operations,
- message-passing ownership transfer,
- a specialized synchronization abstraction defined by another rulebook,
- target/platform mechanisms exposed through a verified Sec abstraction.

§ 13(7) Unsafe raw-pointer access does not make an otherwise invalid concurrent race valid.

§ 13(8) Shared mutable state must not be introduced implicitly by closure capture, static storage, FFI, or backend lowering.

---

## § 14. Synchronization

**Governance tags:** `concurrency.model-v2`, `sema.data-race-analysis`, `sema.deadlock-analysis`

§ 14(1) Synchronization primitives provide semantic relationships, not merely library calls.

§ 14(2) A synchronization primitive must define its ownership/capability behavior.

§ 14(3) A synchronization primitive must define its blocking or nonblocking behavior.

§ 14(4) A synchronization primitive must define its memory-order or publication effect where applicable.

§ 14(5) A synchronization primitive must define its cancellation behavior where applicable.

§ 14(6) A synchronization primitive must define whether its capability may cross task, thread, process, interrupt, or foreign boundaries.

§ 14(7) The compiler and analyses must reason from canonical synchronization semantics rather than method names.

§ 14(8) A user function named `lock` is not automatically a mutex acquisition.

§ 14(9) A foreign operation is not automatically synchronization merely because its native implementation happens to contain a lock.

§ 14(10) Volatile access is not synchronization.

---

## § 15. Mutex overview

**Governance tags:** `concurrency.model-v2`, `sema.data-race-analysis`, `sema.deadlock-analysis`

§ 15(1) `Mutex[T]` is the primary exclusive shared-state primitive for ordinary Sec code where mutual exclusion is required.

§ 15(2) `Mutex[T]` owns its protected `T`.

§ 15(3) Protected access occurs through `MutexGuard[T]`.

§ 15(4) `MutexGuard[T]` is an exclusive access capability.

§ 15(5) `MutexGuard[T]` is move-only.

§ 15(6) Destruction of the guard releases the acquisition according to `mutex.md`.

§ 15(7) A live mutex guard is constrained by execution-entity and suspension rules.

§ 15(8) A live guard must not be treated as an ordinary transferable owned value.

§ 15(9) A live guard must not cross an execution boundary when `mutex.md` forbids that crossing.

§ 15(10) A live guard must not cross `await` where the canonical mutex rules forbid suspension with the guard active.

§ 15(11) `Mutex[T]` is non-reentrant in Sec 0.1.

§ 15(12) Mutex lock acquisition, `tryLock`, timeout/context forms, cancellation, fairness, priority inversion, destruction, and target integration are owned by `mutex.md`.

---

## § 16. Atomics overview

**Governance tags:** `concurrency.model-v2`, `sema.data-race-analysis`, `compiler.platform-model`

§ 16(1) Atomic operations are explicit synchronization operations for supported atomic types and operations.

§ 16(2) Atomics are appropriate for operations whose invariant can be expressed through the supported atomic object and ordering semantics.

§ 16(3) Atomics must not be treated as an automatic replacement for a mutex protecting a multi-location invariant.

§ 16(4) Atomicity is distinct from volatility.

§ 16(5) Atomicity is distinct from ordinary ownership exclusivity.

§ 16(6) Atomic operations must retain their selected memory-order semantics through Semantic IR and lowering.

§ 16(7) Target support for an atomic operation must be resolved through the selected `CompilationPlan`.

§ 16(8) Unsupported required atomic behavior must be rejected or lowered through an explicitly permitted runtime mechanism according to `atomics.md` and the target profile.

§ 16(9) Detailed atomic types, operations, memory-order values, compare/exchange semantics, fences, and target mapping are owned by `atomics.md`.

---

## § 17. Message passing and channels

**Governance tags:** `concurrency.model-v2`, `frontend.transferability`, `semantic-ir.transferability`

§ 17(1) Message passing is a first-class concurrency strategy.

§ 17(2) Channel operations must preserve ownership semantics of the transmitted value.

§ 17(3) A consuming send transfers ownership only at the commit point defined by the channel contract.

§ 17(4) A failed operation must preserve or consume the source exactly as defined by the owning channel rule.

§ 17(5) Channel buffering must not imply hidden semantic copying of move-only values.

§ 17(6) Channel receive must establish ownership of a received owned value according to the channel contract.

§ 17(7) Borrowed values transmitted through a channel require explicit semantics and lifetime proof; ordinary owned-message transfer must not be reinterpreted as retained borrowing.

§ 17(8) Channel blocking, cancellation, close behavior, select integration, and memory ordering are owned by the channel and select rulebooks.

§ 17(9) Cross-process IPC must not be inferred from ordinary in-process channel semantics.

---

## § 18. Static storage

**Governance tags:** `concurrency.model-v2`, `sema.data-race-analysis`

§ 18(1) Static lifetime does not imply concurrency safety.

§ 18(2) Immutable static data may be read concurrently when publication and type semantics permit it.

§ 18(3) Mutable static state that may be accessed concurrently requires synchronization or another valid concurrency-safe mechanism.

§ 18(4) The declaration:

```sec
static let mut State: ApplicationState
```

does not by itself make concurrent mutation safe.

§ 18(5) A protected form may use:

```sec
static let State: Mutex[ApplicationState]
```

when mutex semantics are appropriate.

§ 18(6) Static storage may satisfy a lifetime requirement for detached or long-lived execution without satisfying race freedom, transferability, synchronization, or shutdown requirements.

§ 18(7) Detailed static declaration and initialization semantics are owned by `static.md`.

---

## § 19. Structured concurrency

**Governance tags:** `concurrency.model-v2`, `concurrency.tasks-v2`, `analysis.transferability`

§ 19(1) Structured concurrency constrains the lifetime relationship between parent execution and child work.

§ 19(2) Owning child handles must be resolved, transferred, or explicitly detached according to their owning rulebooks.

§ 19(3) A lexical parent scope must not silently abandon unresolved owning child handles.

§ 19(4) Returning or moving a child handle transfers the lifecycle obligation.

§ 19(5) Awaiting or joining a child discharges the corresponding lifecycle responsibility as defined by the task or thread rulebook.

§ 19(6) Detach is an explicit escape from the ordinary structured lifecycle.

§ 19(7) Structured concurrency must preserve deterministic cleanup.

§ 19(8) Structured concurrency must preserve cancellation relationships defined by `cancellation.md`.

§ 19(9) Structured concurrency must not rely on garbage collection to eventually resolve abandoned execution handles.

§ 19(10) Detailed scope forms and child-propagation semantics are owned by `structured_concurrency.md`.

---

## § 20. Detachment

**Governance tags:** `concurrency.model-v2`, `frontend.discard-v2`, `analysis.transferability`

§ 20(1) Detachment must always be explicit.

§ 20(2) Detachment consumes or transfers the owning lifecycle capability according to the specialized execution-kind rulebook.

§ 20(3) Detachment must not leave the source binding usable as an owning handle.

§ 20(4) A detached execution entity must not retain a borrow whose source lifetime can end before the detached use.

§ 20(5) A detached execution entity must not retain thread-affine or storage-affine state unless the relevant transferability and target rules prove the use valid.

§ 20(6) Discard of a value-returning detached task or thread must follow the explicit discard rules.

§ 20(7) Detachment does not retroactively discard a spawn error; creation must first have succeeded.

§ 20(8) The runtime may retain internal lifecycle state after source-level detachment, but that retention must not create a second source-level owner.

§ 20(9) Program shutdown behavior for detached execution is owned by the runtime, cancellation, task/thread, and target rulebooks.

---

## § 21. Cancellation

**Governance tags:** `concurrency.model-v2`, `concurrency.tasks-v2`

§ 21(1) Task cancellation is cooperative.

§ 21(2) A cancellation request and cancellation completion are distinct events.

§ 21(3) Requesting cancellation does not by itself prove that the target task has terminated.

§ 21(4) A task may observe current cancellation state through the task-local cancellation facilities defined by `cancellation.md`.

§ 21(5) The `cancel` control operation terminates the current task through the canonical cancellation path where valid.

§ 21(6) Cancellation must preserve deterministic cleanup.

§ 21(7) Cancellation must preserve defer semantics.

§ 21(8) Cancellation must preserve destruction of still-owned values.

§ 21(9) Cancellation must preserve borrow validity and cleanup.

§ 21(10) Cancellation must not fabricate a normal return value.

§ 21(11) Cancellation must remain distinct from panic.

§ 21(12) Cancellation must remain distinct from task execution failure.

§ 21(13) Cancellation propagation, cancellation checkpoints, shielding, wait commit points, parent-child propagation, and shutdown behavior are owned by `cancellation.md`.

---

## § 22. Blocking and suspension

**Governance tags:** `concurrency.model-v2`, `sema.deadlock-analysis`, `compiler.platform-model`

§ 22(1) The compiler must preserve the distinction between operations that block a physical execution context and operations that suspend a logical task.

§ 22(2) An operation may be:

- nonblocking,
- task-suspending,
- physically blocking,
- indefinitely blocking,
- cancellation-aware,
- non-cancellable,
- conditionally blocking according to its resolved contract.

§ 22(3) These properties are semantic effects when they affect safety, scheduling, cancellation, deadlock, ISR legality, or target compatibility.

§ 22(4) A task-suspending operation need not block the physical worker.

§ 22(5) A native blocking system call may block the physical worker even when invoked from a task.

§ 22(6) A foreign call must not be assumed cancellation-aware or scheduler-aware without an explicit contract.

§ 22(7) A blocking or suspending operation must preserve active ownership and borrow invariants.

§ 22(8) Execution-entity-bound capabilities such as mutex guards may prohibit suspension.

§ 22(9) Target/profile rules may reject blocking operations in contexts such as interrupts or runtime-critical sections.

---

## § 23. Wait and completion synchronization

**Governance tags:** `concurrency.model-v2`, `sema.deadlock-analysis`, `sema.data-race-analysis`

§ 23(1) Await and join are semantic synchronization operations, not ordinary polling.

§ 23(2) A status read is not automatically equivalent to successful completion synchronization.

§ 23(3) A completion synchronization edge must provide the memory visibility required by the concurrency memory model.

§ 23(4) Await and join may have different ownership effects even when both wait for completion.

§ 23(5) Task await consumes the owning task handle and yields the task outcome according to `tasks.md`.

§ 23(6) Task join preserves the task handle according to `tasks.md`.

§ 23(7) Thread join follows `threads.md`.

§ 23(8) Completion waits participate in deadlock analysis because execution progress may depend on another execution entity completing.

§ 23(9) Waiting while holding a synchronization resource must be analyzed according to the resource and deadlock rules.

---

## § 24. Memory visibility and happens-before

**Governance tags:** `concurrency.model-v2`, `sema.data-race-analysis`

§ 24(1) The concurrency memory model defines the canonical ordering and visibility relationships between execution contexts.

§ 24(2) Ownership transfer and synchronization are related but distinct concepts.

§ 24(3) A legal ownership transfer across an execution boundary must be accompanied by the publication semantics required by the boundary.

§ 24(4) Successful task/thread completion synchronization publishes the completed execution's required visible effects to the waiting context.

§ 24(5) Mutex release/acquisition establishes the ordering defined by `mutex.md` and `concurrency_memory_model.md`.

§ 24(6) Atomic operations establish ordering according to their memory-order contract.

§ 24(7) Channel operations establish ordering according to their channel contract.

§ 24(8) Volatile operations do not become synchronization operations merely because they are observable.

§ 24(9) Compiler optimization and backend lowering must preserve the canonical happens-before relationships.

§ 24(10) The compiler must not strengthen a weak or absent synchronization relationship merely because the generated target instruction happens to provide stronger ordering on one platform.

§ 24(11) The compiler must not weaken a required synchronization relationship merely because one current target would appear to work without it.

§ 24(12) Precise memory-order semantics are owned by `concurrency_memory_model.md`.

---

## § 25. Data races

**Governance tags:** `concurrency.model-v2`, `sema.data-race-analysis`

§ 25(1) Ordinary unsynchronized data races are invalid Sec behavior.

§ 25(2) Data-race semantics apply across every execution context that may overlap, not only tasks.

§ 25(3) Relevant contexts may include tasks, threads, repeated spawn instances, foreign callbacks, interrupt execution, platform execution, and other canonical execution contexts.

§ 25(4) The precise definition of conflicting accesses, overlap, synchronization, happens-before, and race proof is owned by `data_races.md` and `concurrency_memory_model.md`.

§ 25(5) The compiler must consume canonical Place/provenance and storage-overlap facts rather than reasoning only from source variable names.

§ 25(6) Ownership exclusivity can eliminate sharing, but borrow exclusivity by itself must not be mistaken for cross-context synchronization.

§ 25(7) Atomic accesses must be classified according to the atomic rulebook rather than treated as ordinary unsynchronized accesses.

§ 25(8) Runtime instrumentation may assist checked profiles but does not replace required static semantic validation.

§ 25(9) This overview does not define a second race-analysis algorithm.

---

## § 26. Deadlocks

**Governance tags:** `concurrency.model-v2`, `sema.deadlock-analysis`

§ 26(1) Deadlock is a cyclic blocking or progress dependency that prevents required progress.

§ 26(2) Deadlock is distinct from a data race.

§ 26(3) Deadlock is distinct from starvation.

§ 26(4) Deadlock is distinct from livelock.

§ 26(5) Deadlock analysis is not limited to mutexes.

§ 26(6) Completion waits, resource acquisition, foreign waits, callbacks, condition/event semantics, platform synchronization, and interrupt interactions may participate where their canonical semantics create progress dependencies.

§ 26(7) The compiler must use the canonical deadlock analysis rather than duplicating ad hoc lock-order checks in unrelated features.

§ 26(8) `Mutex[T]` non-reentrancy provides direct semantic information for self-deadlock analysis.

§ 26(9) Waiting for another execution entity while holding a required resource may contribute to a deadlock cycle.

§ 26(10) Unknown dependency information must not be fabricated into a proven deadlock.

§ 26(11) Detailed `ProvenDeadlock`, `PotentialDeadlock`, `Unknown`, cycle realizability, witness, call-path, and incremental-analysis semantics are owned by `deadlock_analysis.md`.

---

## § 27. Resource amplification and task creation

**Governance tags:** `concurrency.model-v2`, `concurrency.tasks-v2`, `compiler.platform-model`

§ 27(1) Concurrent execution consumes target/runtime resources.

§ 27(2) Task creation patterns may require static or dynamic resource validation.

§ 27(3) Recursive spawn, spawn cycles, repeated detach, and unbounded creation loops may create resource amplification.

§ 27(4) A target profile may impose finite task, thread, stack, queue, or scheduler-resource constraints.

§ 27(5) A hosted runtime may report runtime creation exhaustion through the appropriate spawn-error type where the canonical spawn rule permits runtime failure.

§ 27(6) A static target may require a compile-time proof or bound before accepting a task-creation pattern.

§ 27(7) Compile-time rejection because the selected profile cannot satisfy a resource requirement is distinct from a runtime spawn failure.

§ 27(8) The compiler must not assume the resources of the compiler host are available on the selected target.

§ 27(9) Detailed task resource analysis belongs to task, scheduler, stack, call-graph, target-profile, and compiler-analysis rulebooks.

---

## § 28. Closures and captures

**Governance tags:** `concurrency.model-v2`, `frontend.transferability`, `analysis.transferability`

§ 28(1) A closure passed to newly created execution crosses an execution boundary.

§ 28(2) Its captures must therefore satisfy both closure semantics and boundary transferability.

§ 28(3) Capturing a copyable value does not automatically prove concurrent shareability of referenced storage inside that value.

§ 28(4) Capturing a move-only value by ownership transfers ownership when the canonical spawn transfer commits.

§ 28(5) Capturing a reference requires proof that the referent lifetime, aliasing, address stability, transferability, and execution context remain valid.

§ 28(6) Detached closures require capture lifetimes independent of a source scope that may end.

§ 28(7) Thread-affine captures must not enter migratable tasks unless the scheduler/transferability rules prove the task pinned or otherwise valid.

§ 28(8) The compiler must reason from resolved capture facts rather than closure representation.

---

## § 29. Thread-local and task-local state

**Governance tags:** `concurrency.model-v2`, `platform.transferability`

§ 29(1) Thread-local storage belongs to a physical thread domain.

§ 29(2) Task-local runtime state belongs to a logical task domain where task-local facilities exist.

§ 29(3) These domains are distinct.

§ 29(4) Task migration does not move the identity of thread-local storage.

§ 29(5) A reference into thread-local storage carries a physical-thread affinity dependency.

§ 29(6) A migratable task must not retain such a dependency across migration unless the scheduling and transferability rules prove the use valid.

§ 29(7) Task-local cancellation state follows the logical task rather than the current worker thread.

§ 29(8) Detailed thread-local semantics are owned by `thread_local.md`.

---

## § 30. FFI and foreign concurrency

**Governance tags:** `concurrency.model-v2`, `platform.transferability`, `sema.data-race-analysis`, `sema.deadlock-analysis`

§ 30(1) Foreign code may block, create foreign threads, call back into Sec, retain pointers, access shared memory, or use foreign synchronization.

§ 30(2) The compiler must not infer concurrency safety from a raw pointer, foreign handle, function name, or ABI alone.

§ 30(3) A safe wrapper must establish whatever ownership, retention, synchronization, thread-affinity, callback-context, and blocking contracts are necessary for safe Sec use.

§ 30(4) Foreign callbacks that may execute concurrently must enter the canonical execution-context model used by race, deadlock, lifetime, transferability, and ISR analyses.

§ 30(5) Unknown foreign retention must not be treated as proven non-retention.

§ 30(6) Unknown foreign blocking must not be treated as nonblocking.

§ 30(7) Unknown foreign callback context must not be treated as the current task or thread.

§ 30(8) Foreign synchronization must expose a verified contract before it may satisfy Sec synchronization requirements.

§ 30(9) Detailed FFI contracts are owned by `ffi.md`.

---

## § 31. Interrupt and concurrency interaction

**Governance tags:** `concurrency.model-v2`, `sema.data-race-analysis`, `sema.deadlock-analysis`, `compiler.platform-model`

§ 31(1) Interrupt execution is a concurrency context when it may overlap or preempt ordinary execution.

§ 31(2) ISR concurrency must participate in race analysis where memory locations may be shared.

§ 31(3) ISR waits or synchronization may participate in deadlock analysis where the interrupt rules permit such operations.

§ 31(4) Ordinary `Mutex[T]` must not be assumed interrupt-safe.

§ 31(5) Atomics or dedicated interrupt-safe primitives may be used only when the selected target/profile and owning rulebooks permit them.

§ 31(6) Interrupt masking, priority, nesting, preemption, and controller semantics are platform/interrupt facts rather than generic concurrency guesses.

§ 31(7) The concurrency model consumes those facts from the canonical `CompilationPlan`.

§ 31(8) Unsafe does not waive ISR execution-context restrictions.

---

## § 32. Unsafe concurrency

**Governance tags:** `concurrency.model-v2`, `platform.transferability`, `sema.data-race-analysis`

§ 32(1) `unsafe` does not disable concurrency semantics.

§ 32(2) Unsafe code remains subject to ownership rules for ordinary owned values.

§ 32(3) Unsafe code remains subject to deterministic destruction requirements.

§ 32(4) Unsafe code remains subject to explicit execution lifecycle rules.

§ 32(5) Unsafe code remains subject to target capability restrictions.

§ 32(6) Raw pointers may remove compiler proof of alias safety for a particular access, but they do not redefine the language to make data races valid.

§ 32(7) The programmer bears responsibility for safety properties that cannot be proven after entering raw-pointer concurrency.

§ 32(8) A compiler-proven invalid concurrent operation must not become valid merely because it appears inside `unsafe`.

§ 32(9) Unsafe FFI or platform synchronization must still satisfy the explicit contract required by the concurrency memory model.

---

## § 33. Target profiles and `CompilationPlan`

**Governance tags:** `concurrency.model-v2`, `compiler.platform-model`, `platform.transferability`

§ 33(1) Concurrency lowering must use the selected immutable `CompilationPlan`.

§ 33(2) Host-machine concurrency properties must not leak into target semantics.

§ 33(3) A `TargetProfile` may describe availability, activation, restrictions, and resource policies for tasks, threads, synchronization, atomics, detached execution, thread-local storage, task-local runtime storage, interrupts, and related facilities.

§ 33(4) `BuildProfile` must not silently change source-level concurrency semantics.

§ 33(5) Hosted, RTOS, and BareMetal are profile families rather than aliases for particular scheduler implementations.

§ 33(6) Hosted does not imply that every concurrency feature exists.

§ 33(7) RTOS does not imply native hosted threads.

§ 33(8) Bare metal does not imply that no concurrency runtime can exist.

§ 33(9) A profile may restrict execution kinds or synchronization mechanisms without changing the meaning of a construct that remains enabled.

§ 33(10) Unsupported required capabilities must be rejected before incompatible lowering.

§ 33(11) Scheduler choice, worker count, native thread mapping, task-slot strategy, atomic lowering, and mutex backend may vary by target while source semantics remain fixed.

---

## § 34. Runtime independence

**Governance tags:** `concurrency.model-v2`, `compiler.platform-model`, `lowering.transferability`

§ 34(1) Sec does not require one universal concurrency runtime.

§ 34(2) A target may implement tasks through an event loop, cooperative executor, worker pool, fibers, RTOS facilities, native threads, static state machines, or another conforming mechanism.

§ 34(3) A target may implement thread synchronization using native primitives or conforming runtime support.

§ 34(4) A bare-metal target may use statically allocated task and synchronization state where permitted.

§ 34(5) Runtime implementation strategy must not alter ownership.

§ 34(6) Runtime implementation strategy must not alter borrow validity.

§ 34(7) Runtime implementation strategy must not alter task outcome semantics.

§ 34(8) Runtime implementation strategy must not alter memory-order guarantees.

§ 34(9) Runtime implementation strategy must not silently add heap allocation where the language/target contract forbids it.

§ 34(10) Runtime implementation strategy must not silently add garbage collection or runtime ownership tracking.

---

## § 35. Semantic analysis

**Governance tags:** `concurrency.model-v2`, `frontend.transferability`, `analysis.transferability`, `sema.data-race-analysis`, `sema.deadlock-analysis`

§ 35(1) Semantic analysis must identify the execution kind of every concurrency operation.

§ 35(2) Semantic analysis must preserve the ownership state of concurrency handles and transferred values.

§ 35(3) Semantic analysis must preserve live borrow state relevant to execution boundaries.

§ 35(4) Semantic analysis must validate lifecycle obligations for move-only execution handles where required by the specialized rulebooks.

§ 35(5) Semantic analysis must validate transferability across task, thread, process, ISR, and foreign execution boundaries.

§ 35(6) Semantic analysis must identify synchronization capabilities and resource identities where required by downstream analyses.

§ 35(7) Semantic analysis must preserve blocking, suspension, cancellation, and completion effects required by analysis and lowering.

§ 35(8) Semantic analysis must preserve task/thread execution-context distinctions required by thread affinity and migration.

§ 35(9) Semantic analysis must preserve target/profile requirements.

§ 35(10) Semantic analysis must not implement a private ad hoc data-race algorithm when the canonical race analysis owns that reasoning.

§ 35(11) Semantic analysis must not implement a private ad hoc deadlock algorithm when the canonical deadlock analysis owns that reasoning.

§ 35(12) Specialized mandatory local checks may still reject directly invalid constructs when those checks are owned by the specialized rulebook, such as copying a move-only handle or crossing `await` with a forbidden live guard.

---

## § 36. Data-race analysis integration

**Governance tags:** `sema.data-race-analysis`, `concurrency.model-v2`

§ 36(1) The canonical data-race analysis consumes concurrency semantics rather than redefining them.

§ 36(2) The concurrency layer must provide or preserve facts sufficient to identify potentially overlapping execution contexts.

§ 36(3) It must provide or preserve synchronization/publication semantics.

§ 36(4) It must provide or preserve atomic access identity and ordering.

§ 36(5) It must provide or preserve ownership transfer and sharing state.

§ 36(6) It must provide or preserve canonical memory-location/Place identities used by analysis.

§ 36(7) Race analysis may classify proof strength according to its own rulebook.

§ 36(8) Loss of precision must not be converted into a fabricated proven race.

§ 36(9) The concurrency overview does not define diagnostic severity or the incremental-analysis algorithm for race findings.

---

## § 37. Deadlock analysis integration

**Governance tags:** `sema.deadlock-analysis`, `concurrency.model-v2`

§ 37(1) The canonical deadlock analysis consumes synchronization and execution-progress semantics.

§ 37(2) The concurrency layer must preserve resource acquisition and release semantics.

§ 37(3) It must preserve blocking and nonblocking wait semantics.

§ 37(4) It must preserve completion-wait dependencies.

§ 37(5) It must preserve cancellation or timeout guarantees only when those guarantees are canonical and sufficient to affect progress.

§ 37(6) It must preserve execution-context overlap, preemption, and reentry facts where relevant.

§ 37(7) Deadlock analysis may construct lock-order and generalized wait-for models according to its own rulebook.

§ 37(8) The concurrency overview must not maintain a second list of deadlock patterns as an independent source of truth.

---

## § 38. Semantic IR

**Governance tags:** `analysis.semantic-ir-v2`, `semantic-ir.transferability`, `concurrency.model-v2`

§ 38(1) Semantic IR must preserve concurrency operations explicitly until their semantic distinctions are safely lowered.

§ 38(2) Semantic IR must preserve task creation as fallible where required by canonical task rules.

§ 38(3) Semantic IR must preserve task await, join, cancellation request, terminal outcome, and detach as semantically distinct operations or equivalent resolved plans.

§ 38(4) Semantic IR must preserve thread creation and completion distinctly from task operations.

§ 38(5) Semantic IR must preserve owned and borrowed transfer distinctly.

§ 38(6) Semantic IR must preserve process-adapter transfer distinctly from ordinary in-process transfer.

§ 38(7) Semantic IR must preserve mutex/guard capability semantics.

§ 38(8) Semantic IR must preserve atomic operation identity and memory order.

§ 38(9) Semantic IR must preserve channel ownership commit/failure semantics.

§ 38(10) Semantic IR must preserve cancellation versus completion.

§ 38(11) Semantic IR must preserve task outcome semantics, including the distinction between `Completed(Err(E))` and `Failed(TaskError)`.

§ 38(12) Semantic IR must preserve execution-context and affinity facts where required.

§ 38(13) Semantic IR must preserve synchronization edges and source provenance required by verification and diagnostics.

§ 38(14) Concrete operation names and representation are owned by `semantic_ir.md`.

§ 38(15) This rulebook must not define a competing list of IR opcode names.

---

## § 39. Lowering

**Governance tags:** `lowering.transferability`, `analysis.semantic-ir-v2`, `compiler.platform-model`

§ 39(1) Concurrency lowering begins from validated Semantic IR and the immutable selected `CompilationPlan`.

§ 39(2) Lowering must not rediscover execution kind from source spelling.

§ 39(3) Lowering must not infer ownership transfer from low-level loads or stores.

§ 39(4) Lowering must not infer cancellation from generic control flow.

§ 39(5) Lowering must not infer synchronization from volatile access.

§ 39(6) Lowering must preserve source-level failure and commit semantics.

§ 39(7) Lowering must preserve task outcome semantics even when the runtime uses a different internal representation.

§ 39(8) Lowering must preserve required memory-order and publication edges.

§ 39(9) Lowering must preserve destruction and cleanup responsibility across suspension, cancellation, completion, and detach.

§ 39(10) Lowering must not repair an invalid borrowed transfer by hidden heap allocation.

§ 39(11) Lowering must not duplicate move-only values to satisfy a runtime ABI.

§ 39(12) Target-specific runtime calls remain implementation details once the required semantics have been preserved.

---

## § 40. Optimizations

**Governance tags:** `concurrency.model-v2`, `analysis.semantic-ir-v2`

§ 40(1) Optimization may remove, combine, inline, specialize, or lower concurrency machinery only when observable Sec semantics are unchanged.

§ 40(2) Optimization may prove that a task outcome has fewer reachable runtime variants, but it must not change the static source type of `await Task[T]`.

§ 40(3) Optimization may eliminate synchronization only when the canonical memory model and analysis prove it redundant.

§ 40(4) Optimization must not hoist or sink operations across synchronization boundaries in a way that violates happens-before semantics.

§ 40(5) Optimization must not extend a borrow across suspension when the original semantics ended the borrow earlier.

§ 40(6) Optimization must not shorten the lifetime of an owned resource across a point where another execution context may validly observe it.

§ 40(7) Optimization must not turn explicit concurrency into synchronous execution when doing so changes progress, cancellation, blocking, resource, or observation semantics.

§ 40(8) Optimization must not rely on host scheduling behavior.

---

## § 41. Diagnostics

**Governance tags:** `concurrency.model-v2`, `tooling.transferability`, `sema.data-race-analysis`, `sema.deadlock-analysis`

§ 41(1) Concurrency diagnostics should explain the semantic cause, not merely the backend symptom.

§ 41(2) Ownership-transfer diagnostics should identify the source value, boundary, and reason the original becomes unavailable.

§ 41(3) Transferability diagnostics should identify the concrete task, thread, process, ISR, or foreign boundary.

§ 41(4) Transferability diagnostics should identify the nested field, capture, reference, affinity, storage, or foreign contract that prevents transfer when known.

§ 41(5) Race diagnostics should identify the conflicting memory location/access path and representative conflicting execution contexts according to `data_races.md`.

§ 41(6) Deadlock diagnostics should identify representative acquisitions/waits and the relevant dependency cycle according to `deadlock_analysis.md`.

§ 41(7) Task diagnostics must distinguish task spawn failure, cancellation, panic, execution failure, and normal `Completed(Err(E))`.

§ 41(8) Mutex diagnostics should identify the acquisition and live guard when a suspension or boundary crossing is invalid.

§ 41(9) Target diagnostics should identify the selected target/profile capability that is missing or incompatible.

§ 41(10) FFI diagnostics should identify unknown or incompatible blocking, callback, retention, synchronization, or affinity contracts when they are the reason safe concurrency cannot be proven.

§ 41(11) Diagnostics should provide a corrective direction only when the suggested transformation is semantically valid.

---

## § 42. Tooling and LSP

**Governance tags:** `concurrency.model-v2`, `tooling.transferability`, `sema.data-race-analysis`, `sema.deadlock-analysis`

§ 42(1) Compiler, LSP, and `sec analyse` must consume the same canonical concurrency facts.

§ 42(2) The LSP must not present weaker concurrency semantics than command-line compilation.

§ 42(3) Mandatory safety checks must not disappear because interactive analysis uses a smaller advisory analysis budget.

§ 42(4) Progressive analysis may refine race, deadlock, transferability, and other findings as more analysis completes.

§ 42(5) Pending analysis must remain distinguishable from a proven safe result.

§ 42(6) Project analysis configuration must be watched and affected analyses recomputed or invalidated according to the compiler-analysis rules.

§ 42(7) Tooling may expose optional insights such as execution context, task ownership, transferability, lock order, wait dependencies, or synchronization edges.

§ 42(8) Optional insights must not redefine normative language behavior.

---

## § 43. Error handling and concurrency

**Governance tags:** `concurrency.model-v2`, `concurrency.tasks-v2`

§ 43(1) Sec typed application errors remain distinct from concurrency execution failures.

§ 43(2) A task function may return `Result[T, E]` as its ordinary return type.

§ 43(3) A returned `Err(E)` from that function is a normal returned value of the task function.

§ 43(4) Task spawn failure uses `TaskSpawnError`.

§ 43(5) Failure of an already-created task execution uses `TaskError` through `TaskOutcome[T].Failed`.

§ 43(6) Panic remains distinct from typed errors and task execution failure.

§ 43(7) Cancellation remains distinct from typed errors, panic, and task execution failure.

§ 43(8) Thread creation and execution errors use the thread-specific error model owned by `threads.md`.

§ 43(9) Process errors remain owned by the process rulebook and must not be inferred from the task error model.

§ 43(10) Generic `Result` handling must not automatically flatten or reinterpret task/thread outcome types.

---

## § 44. Cleanup and destruction

**Governance tags:** `concurrency.model-v2`, `frontend.transferability`

§ 44(1) Concurrency does not suspend deterministic destruction semantics.

§ 44(2) A value moved to another execution entity is destroyed by its new owner or subsequent owner according to ordinary ownership rules.

§ 44(3) A value retained by the source remains the source's destruction responsibility.

§ 44(4) A task or thread handle is destroyed only according to its lifecycle rules; ordinary scope exit must not silently abandon unresolved lifecycle responsibility.

§ 44(5) Cancellation must execute required cleanup.

§ 44(6) Normal task/thread completion must execute required cleanup before completion is published at the semantic completion point.

§ 44(7) Detachment transfers lifecycle responsibility without removing cleanup requirements that remain part of normal completion.

§ 44(8) Forced external termination may be unable to guarantee cleanup; the owning process/platform rule must state such behavior explicitly.

§ 44(9) Mutex guard destruction releases the lock according to `mutex.md`.

§ 44(10) Cleanup order remains governed by `destruction.md` and `defer.md`.

---

## § 45. No hidden sharing

**Governance tags:** `concurrency.model-v2`, `frontend.transferability`, `sema.data-race-analysis`

§ 45(1) Passing an owned value to spawned execution must not silently create a shared alias.

§ 45(2) Capturing an owned value must not silently convert ownership into reference sharing.

§ 45(3) Returning a task or thread handle must not create a second lifecycle owner.

§ 45(4) Observer creation must not create result ownership.

§ 45(5) Copying a reference value must not be described as copying the referent.

§ 45(6) Copying a `RawPtr[T]` must not create ownership, synchronization, or lifetime guarantees for the pointee.

§ 45(7) Moving a struct containing synchronization or affinity-sensitive members must respect the owning member rules.

§ 45(8) Backend closure environments, task records, or runtime descriptors must not create source-visible ownership that does not exist in Sec semantics.

---

## § 46. No hidden synchronization

**Governance tags:** `concurrency.model-v2`, `sema.data-race-analysis`

§ 46(1) The compiler must not assume that ordinary loads and stores are synchronized.

§ 46(2) The compiler must not assume that function-call boundaries synchronize memory.

§ 46(3) The compiler must not assume that task scheduling by itself synchronizes unrelated shared memory.

§ 46(4) The compiler must not assume that a status polling loop establishes completion visibility.

§ 46(5) The compiler must not assume that volatile operations provide atomicity or happens-before ordering.

§ 46(6) The compiler must not infer a mutex merely because one backend helper uses an internal lock.

§ 46(7) Synchronization exists only through a canonical synchronization, publication, ownership-transfer, completion, channel, atomic, platform, or FFI contract.

---

## § 47. Cross-rule semantic ownership

**Governance tags:** `concurrency.model-v2`

§ 47(1) `tasks.md` owns `Task[T]`, `TaskOutcome[T]`, task lifecycle, task join/await result semantics, task observers, and task detach behavior.

§ 47(2) `threads.md` owns `Thread[T]`, physical-thread semantics, thread configuration, affinity, thread lifecycle, and thread-specific error/status types.

§ 47(3) `spawn.md` owns spawn grammar, execution-kind selection, callable evaluation, and detailed spawn transfer/commit semantics.

§ 47(4) `await.md` owns general await syntax and awaitable-expression rules while task-specific outcome typing is owned by `tasks.md`.

§ 47(5) `cancellation.md` owns cancellation request, observation, checkpoints, propagation, shielding, and cancellation/operation commit semantics.

§ 47(6) `mutex.md` owns `Mutex[T]` and `MutexGuard[T]`.

§ 47(7) `atomics.md` owns atomic types, operations, orderings, and fences.

§ 47(8) `channels.md` and `select.md` own channel communication and select behavior.

§ 47(9) `scheduling.md` owns task scheduling semantics.

§ 47(10) `structured_concurrency.md` owns structured parent-child scope semantics.

§ 47(11) `concurrency_runtime_model.md` owns the common runtime contract and compiler-known runtime-facing concurrency types not otherwise owned by specialized rulebooks.

§ 47(12) `concurrency_memory_model.md` owns happens-before, visibility, atomic-order, and synchronization memory semantics.

§ 47(13) `thread_local.md` owns thread-local storage semantics.

§ 47(14) Process and IPC rulebooks own process-specific syntax, lifecycle, results, termination, and communication.

§ 47(15) `transferability.md` owns cross-boundary transfer proof.

§ 47(16) `data_races.md` owns canonical data-race analysis.

§ 47(17) `deadlock_analysis.md` owns canonical deadlock analysis.

§ 47(18) `semantic_ir.md` owns concrete Semantic IR representation and operation vocabulary.

§ 47(19) `target_profiles.md` and `platform_model.md` own target capability and immutable `CompilationPlan` resolution.

§ 47(20) `ffi.md` owns foreign concurrency contracts at the FFI boundary.

---

## § 48. Restrictions

**Governance tags:** `concurrency.model-v2`

§ 48(1) Sec concurrency must not introduce implicit tasks.

§ 48(2) Sec concurrency must not silently detach work.

§ 48(3) Sec concurrency must not silently share mutable state.

§ 48(4) Sec concurrency must not bypass ownership.

§ 48(5) Sec concurrency must not bypass borrowing.

§ 48(6) Sec concurrency must not copy move-only lifecycle handles.

§ 48(7) Sec concurrency must not copy move-only synchronization capabilities.

§ 48(8) Sec concurrency must not permit ordinary unsynchronized data races.

§ 48(9) Sec concurrency must not assume one task equals one thread.

§ 48(10) Sec concurrency must not require one universal runtime implementation.

§ 48(11) Sec concurrency must not silently downgrade explicit concurrent execution to synchronous execution when semantics differ.

§ 48(12) Sec concurrency must not infer process transfer from ordinary pointer/value bit copying.

§ 48(13) Sec concurrency must not treat volatile access as synchronization.

§ 48(14) Sec concurrency must not treat unsafe as an exemption from concurrency rules that remain semantically meaningful.

§ 48(15) Sec concurrency must not change source semantics according to the compiler host.

§ 48(16) Sec concurrency must not expose backend-private scheduler state as portable language semantics unless standardized by an owning rulebook.

---

## § 49. Governance

**Governance tags:** `concurrency.model-v2`, `frontend.discard-v2`, `frontend.transferability`, `analysis.transferability`, `semantic-ir.transferability`, `lowering.transferability`, `platform.transferability`, `tooling.transferability`, `sema.data-race-analysis`, `sema.deadlock-analysis`, `analysis.semantic-ir-v2`, `compiler.platform-model`, `concurrency.tasks-v2`

§ 49(1) Mutable implementation information for this rulebook must be maintained in `implementation-status.yaml`.

§ 49(2) The primary governance integration for this revision is `concurrency.model-v2`.

§ 49(3) Existing transferability governance applies to execution boundaries referenced by this rulebook.

§ 49(4) Existing data-race and deadlock governance applies to analyses referenced by this rulebook.

§ 49(5) Existing Semantic IR v2 governance applies to concurrency semantics that survive frontend validation.

§ 49(6) Existing platform-model governance applies to concurrency capability and execution-model resolution in `CompilationPlan`.

§ 49(7) Existing discard governance applies to task/thread lifecycle result discard.

§ 49(8) Existing task-v2 governance applies to the task semantics summarized here.

§ 49(9) Governance status must not weaken a normative clause in this rulebook.

§ 49(10) A normative concurrency feature may remain unimplemented or partial without ceasing to be normative unless the owning language rulebook explicitly marks the feature as deferred.

§ 49(11) Implementation milestones must not preserve superseded task, ownership, Semantic IR, target, race-analysis, or deadlock-analysis assumptions merely because older frontend or backend code still implements them.

§ 49(12) This revision requires no new cross-rulebook correction file beyond corrections already created for the task-v2 synchronization; it primarily replaces stale and duplicated material in the previous concurrency overview.
