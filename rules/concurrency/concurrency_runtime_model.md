# Concurrency Runtime Model

- **Status:** Normative
- **Created:** 2026-07-29
- **Last updated:** 2026-11-13 (revision 2.0; normative runtime-model synchronization)
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/concurrency_runtime_model.md`
- **Replaces:** Revision 1 at the same canonical path; original chapter/paragraph identifiers retained
- **Repository baseline reviewed:** `ee75e5d036d6611519a1b34d570bb6778002acfd`
- **Implementation governance:** `governance/concurrency_model.yaml` (`concurrency.runtime-model-v1`), with specialist entries in task/thread/process/scheduling/cancellation/select/thread-local/atomic/platform governance
- **Related rulebooks:** `rules/concurrency/concurrency.md`, `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/concurrency/processes.md`, `rules/concurrency/spawn.md`, `rules/concurrency/await.md`, `rules/concurrency/blocking.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/scheduling.md`, `rules/concurrency/select.md`, `rules/concurrency/channels.md`, `rules/concurrency/mutex.md`, `rules/concurrency/atomics.md`, `rules/concurrency/thread_local.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/transferability.md`, `rules/platform/target_profiles.md`, `rules/platform/platform_model.md`, `rules/platform/interrupts.md`, `rules/platform/ffi.md`, `rules/compiler/semantic_ir.md`
- **Applied correction:** `rules/corrections/applied/concurrency-runtime-model-normative-fix-20261113-20261113.md`

---

## § 1 Authority and implementation boundary

**§ 1(1)** This rulebook specifies the portable concurrency runtime obligations required to realize Sec's already-defined tasks, physical threads, Sec-callable processes, waiting, synchronization, observers, and deterministic cleanup. It does not own their public type declarations, accepted source syntax, or separate lifecycle semantics; those remain with the specialized rulebooks.

**§ 1(2)** Implementation progress, existing code and tests, open integration points, verification evidence, and target-specific rollout belong exclusively in `governance/concurrency_model.yaml` (`concurrency.runtime-model-v1`) and the specialist governance fragments. This normative text must not state which features are currently implemented or pending.

## § 2 Purpose

**§ 2(1)** The concurrency runtime model defines which runtime services a target profile may
provide and how compiler-generated code uses them.

**§ 2(2)** Sec does not require one universal runtime.

**§ 2(3)** A program may target:

- hosted operating systems;
- RTOS environments;
- bare metal;
- statically scheduled embedded systems;
- environments with tasks but no physical threads;
- environments with physical threads but no task executor;
- single-threaded systems with interrupts.

**§ 2(4)** Source-level semantics must remain stable across runtime models.

---

## § 3 No-required-runtime principle

**§ 3(1)** A language feature must not automatically imply a heavyweight managed runtime.

**§ 3(2)** The compiler may use:

- direct native system calls;
- native OS threads;
- RTOS primitives;
- compiler-generated state machines;
- static control blocks;
- fixed-capacity queues;
- direct atomics;
- target-specific wait sets;
- optional hosted scheduler support.

**§ 3(3)** A target profile must declare the services it supplies.

**§ 3(4)** Unsupported semantics must be rejected, not silently changed.

---

## § 4 Independent capabilities
**§ 4(1)** A profile declares capabilities independently.

**§ 4(2)** The selected immutable `CompilationPlan` resolves capability facts independently where they affect the program. Relevant domains include:

```text
task execution and executor scheduling
physical/native thread execution
Sec-callable process creation and process isolation
task suspension and physical-thread waiting
wait/readiness registration and cancellation wakeups
channels and their storage/communication modes
mutex and atomic operations, widths and memory orders
timers and Context-aware waits
per-physical-thread local storage and attachment
ISR execution and cross-context handoff
allocation domains, dynamic-allocation permission and static storage
thread configuration and deferred start
detached execution, completion cleanup and shutdown policy
panic containment/reporting and target-specific termination
```

This inventory describes **independent validation domains**, not a mandatory universal runtime, fixed runtime-interface shape, automatically available API set, or blanket claim that every target supplies each capability.

**§ 4(3)** Supporting tasks does not imply physical threads.

**§ 4(4)** Supporting physical threads does not imply a task executor.

**§ 4(5)** Supporting interrupts does not imply either.

**§ 4(6)** Capability resolution and provenance are governed by `rules/platform/target_profiles.md` and `rules/platform/platform_model.md`. A compiler must not infer target support from compiler-host facilities or substitute another execution kind to bypass a missing required capability.

---

## § 5 Execution creation is fallible

**§ 5(1)** The raw source-level creation result depends on the requested execution kind and preserves the complete declared normal return type `T` of the selected callable:

```sec
spawn Work()          // Result[Task[T], TaskSpawnError]
spawn task Work()     // Result[Task[T], TaskSpawnError]
spawn thread Work()   // Result[Thread[T], ThreadSpawnError]
spawn process Work()  // Result[Process[T], ProcessSpawnError]
```

**§ 5(2)** An unqualified `spawn` is exactly task creation. It does not select physical-thread creation because a target happens to implement tasks with native threads.

**§ 5(3)** Spawn failure occurs **before** a successfully created owning execution handle exists. After successful creation, terminal task failure, thread termination, and process completion failure are classified by their corresponding specialized contracts. A callable returning `Result[V,E]` retains that complete return type inside its execution handle; `Err(E)` returned by the callable is not a spawn failure.

**§ 5(4)** Ordinary `Result`, `try`, and `match` rules apply to the raw creation result. For example:

```sec
let worker := try spawn Work()
```

**§ 5(5)** Runtime creation failures use the exact closed error type owned by `tasks.md`, `threads.md`, or `processes.md`; this rulebook neither invents additional error variants nor flattens one error domain into another. Ownership commit and rollback must follow the owning spawn/ownership contract. No implementation may assume a universal task/thread failure-commit rule while `MD-128` remains unresolved; `spawn process` follows its already-fixed transactional startup rule.

**§ 5(6)** A target/profile that cannot supply a required execution kind is rejected during compile-time target validation. Unsupported task, physical-thread, or Sec-process capability must never be disguised as a runtime `TaskSpawnError`, `ThreadSpawnError`, or `ProcessSpawnError`, nor silently replaced by synchronous execution or another kind of concurrency.

---

## § 6 Task terminal outcome

**§ 6(1)** A successfully created `Task[T]` has exactly the four terminal outcomes defined by `rules/concurrency/tasks.md` § 11: `Completed(T)`, `Cancelled`, `Panicked(PanicInfo)`, and `Failed(TaskError)`. These are alternatives of `TaskOutcome[T]`, not runtime scheduling states. The complete source-visible declaration belongs to `tasks.md` and must not be independently redefined here.

**§ 6(2)** Runtime result publication preserves the callable's complete declared `T`, including `Completed(Err(E))` for a normally returned `Result[V,E]`. A pre-creation `TaskSpawnError` is never a task terminal outcome. `Failed(TaskError)` is an error of an **already-created task execution mechanism**, with the exact closed `TaskError` inventory owned by `tasks.md` § 15. `Panicked` is distinct from `Failed` and is observable only where the selected panic policy permits containment.

---

## § 7 Core execution control block

**§ 7(1)** The compiler may use a common **internal conceptual model** of lifecycle ownership, completion, and runtime control for tasks, physical threads, and processes. It does not establish a user-visible `ExecutionControlBlock` type, require identical storage layout, or erase execution-kind identity.

**§ 7(2)** No common native runtime object, allocator, OS thread, or process memory region is required. In particular, a `Process[T]` owner and its parent-side completion/IPC representation cannot be treated as ordinary memory shared with the isolated child process.

**§ 7(3)** A target realization may specialize or omit storage for capabilities that the selected execution kind and profile do not need, while preserving source-level owner identity, transferability, terminal outcome, synchronization, and cleanup facts.

**§ 7(4)** Conceptually applicable runtime facts may include lifecycle state, terminal payload ownership, cancellation request state, observer identity, readiness registrations, cleanup authority, native identity, source provenance, and generation/lifetime identity. Actual fields and layout are target/runtime-private. They do not expand a public `Task[T]`, `Thread[T]`, or `Process[T]` API.

**§ 7(5)** Unused source-unobservable capabilities need not incur runtime storage or dispatch cost. Elision or specialization is permitted only when it preserves all validated observable semantics, including storage lifetime and completion synchronization.

---

## § 8 Result storage

**§ 8(1)** An execution owner is responsible for whatever terminal state and payload representation its specialized contract requires. A task publishes `TaskOutcome[T]`; a physical thread has thread-status and post-join value/panic/termination availability; a Sec process transports its typed completion across the isolation boundary. These are **not** one interchangeable result API.

**§ 8(2)** The realization must preserve, where applicable:

- the complete declared callable result type and nested `Result` structure;
- move-only payload ownership and exactly-once consumption/destruction;
- task await's consuming `TaskOutcome[T]` transfer;
- thread join and terminal property availability as defined in `threads.md`;
- process completion transport and native reaping as defined in `processes.md`;
- cancellation, panic, execution failure, and hard/abnormal termination as separate **kind-specific** states;
- completion publication and memory-visibility guarantees owned by `concurrency_memory_model.md`.

**§ 8(3)** Terminal payload storage may be inline, statically reserved, separately allocated when allowed, or optimized into the receiving representation. A process child value must be materialized through its typed process-transfer/IPC contract rather than assumed to be directly addressable from the parent.

**§ 8(4)** An optimization may not duplicate move-only payload extraction, fabricate values after abnormal completion, retain a source-level owner after detach, or let a caller observe committed terminal completion before the completion/payload visibility obligations are satisfied.

---

## § 9 Task runtime models
**§ 9(1)** Valid task implementations include:

```text
compiler state machines
cooperative executor
worker pool
stackful fibers
RTOS tasks
native thread per task
event loop
static task slots
```

**§ 9(2)** A logical task may migrate between physical workers only at permitted scheduling boundaries and only while preserving `TaskContext`, task ownership, cancellation identity, borrow/provenance constraints, TLS attachment rules, and any verified execution affinity. Native preemption is not automatically a source-level migration point (`scheduling.md` §§ 16–19).

**§ 9(3)** Task identity is independent of worker thread identity.

**§ 9(4)** A task implemented by a native thread remains semantically `Task[T]`, not `Thread[T]`; physical thread identity, affinity, thread-local values, and `Thread.Current()` must not be replaced with task identity. Ordinary program entry is not an implicit root task merely because runtime machinery is present.

---

## § 10 Thread runtime models
**§ 10(1)** A physical thread maps to a target-native physical execution abstraction.

**§ 10(2)** Valid implementations include:

```text
POSIX thread
Windows thread
RTOS native task/thread
declared hardware-thread abstraction
```

**§ 10(3)** A thread-capable target must realize the exact creation, native identity, eager/deferred start where configured, join, detach cleanup, cancellation requests, terminal publication, explicit `ThreadStorage`, and memory-ordering semantics required by `threads.md`, `thread_local.md`, and the selected `CompilationPlan`. `Start()` is a fallible operation for an already-created deferred thread; it is not a universal second creation step.

**§ 10(4)** If the selected target lacks the required physical-thread capability or cannot satisfy a `Required(...)` configuration constraint, the compiler rejects the operation. It must not lower `spawn thread` to ordinary `spawn task`, ignore deferred start, silently substitute heap storage for explicit thread backing storage, or invent an unsafe portable thread-termination API.

**§ 10(5)** A successful deferred thread creation yields an owning `Thread[T]` in `ThreadStatus.Created`; no user callable code runs before successful `Start()` commit. Thread-specific state and errors remain exactly as specified in `threads.md` §§ 15–21.

---

## § 11 Static embedded storage

**§ 11(1)** The canonical explicitly supplied physical-thread backing-storage API is `ThreadStorage(capacity)`, whose non-generic move-only declaration and constructor are owned by `threads.md` § 8. It is not `ThreadStorage[N]`. This runtime book defines **no** additional public `TaskStorage[StateSize]` type; a target may use compiler-private static task slots or control storage without exposing that spelling as a Sec source API.

**§ 11(2)** `ThreadStorage` is `@noCopy`; its constructor capacity is a compile-time `uint` fact, and construction itself performs no dynamic allocation. An explicit `ThreadConfig.Storage` borrow must retain the exact exclusive backing-storage lifetime until thread/native cleanup releases it. Hidden heap or alternate backing storage may not replace a required explicit storage contract.

**§ 11(3)** The compiler/runtime validates target-correct size, alignment, address/lifetime stability, exclusive access, startup failure cleanup, join/detach release point, and reusability of the chosen storage. Physical backing must consume target/plan facts, not compiler-host assumptions.

**§ 11(4)** Static execution storage is a permitted realization strategy, not a promise that all static programs can run tasks or threads. Lack of a required scheduler, storage, or lifecycle facility remains a target-capability error.

---

## § 12 Deferred activation

**§ 12(1)** Only the explicit deferred physical-thread mode in `threads.md` is standardized by this section. Successful creation establishes `Thread[T]` and `ThreadStatus.Created` with a valid lifecycle owner, thread configuration, and any committed backing-storage obligations; the user callable must not execute until `Start()` succeeds. Ordinary tasks and `spawn process` do not gain a deferred-start source mode by analogy.

**§ 12(2)** A target may realize deferred start using native suspended creation, an RTOS mechanism, a start gate, or an equivalent target-private facility, provided the exact source-visible `Start() Result[void, ThreadStartError]` contract and error/cleanup behavior are preserved. An unsupported required deferred-start capability is rejected rather than silently run eagerly.

**§ 12(3)** `Start()` failure must not be reported as a fresh `ThreadSpawnError` or fabricate a successful running thread. An owner of a never-started thread retains the ordinary unresolved lifecycle obligation until it is resolved by a legal thread operation; cleanup and backing-storage release follow `threads.md` and ownership/destruction rules.

---

## § 13 Task suspension

**§ 13(1)** Waiting in a logical task may be realized through suspension, continuation, event-loop readiness, or another target-supported mechanism. A backend may physically block an executor worker only where the selected profile and `blocking.md` permit that implementation without changing cancellation, progress, ownership, or completion semantics.

**§ 13(2)** A runtime may use readiness registrations for channel operations, selectable completion, mutex waits, timers, I/O, context cancellation, and other explicitly supported waitable operations. This is a conceptual implementation inventory, not an assertion that every listed event is a legal `select` branch or shares one universal wait API.

**§ 13(3)** Before one selected wait commits, losing or cancelled registrations must be released without consuming their source-level operands, transferring values, or creating completion synchronization that did not occur. The branch-readiness and first-commit rules of `select.md`, `await.md`, and `cancellation.md` govern the observable result.

**§ 13(4)** `Task.Yield()` is a current-logical-task scheduling **and cancellation** point, returns `void` after normal commit, and establishes no memory-synchronization edge by itself. `Thread.Yield()` does not substitute for it. Normal `main` has no implicit `TaskContext`; a runtime-private main scheduling mechanism cannot fabricate one (`scheduling.md`).

---

## § 14 Physical thread blocking

**§ 14(1)** A physical-thread-context wait may use a native park, wait, or block operation when the operation's source contract and selected target/profile permit it. Physical blocking and logical task suspension are distinct effects even if one native facility happens to implement both.

**§ 14(2)** `Thread.Yield()` is a physical scheduler hint, not a logical task scheduling point, task/thread cancellation point, or memory-synchronization edge. It does not grant `TaskContext` to the current thread.

**§ 14(3)** A task wait may physically block a worker only when the selected plan permits worker blocking, any proven executor-deadlock/starvation constraints are satisfied, cancellation wakeup and first-commit behavior are preserved, and no forbidden interrupt/no-block context is crossed. No backend may silently convert a source-level nonblocking or cancellation-aware operation into an incompatible physical wait.

---

## § 15 Select runtime

**§ 15(1)** The runtime must realize the `select` contract from `select.md`; it need not expose a public wait-set type or a single shared native wait implementation.

**§ 15(2)** Each legal branch kind must provide enough readiness, reservation, commit, and cancellation semantics to support atomic branch selection. A possible internal protocol is `prepare`, `check-ready`, `register`, `commit`, `cancel-registration`, but these words are not Sec API declarations or mandated low-level function names.

**§ 15(3)** At most one branch commits per execution of `select`. When a ready branch is selected, its source-level operation commits exactly once. A cancellation or termination that wins before branch commit follows its owning rule's first-commit semantics.

**§ 15(4)** When multiple branches are ready at the same semantic selection point, the earliest **source-order** ready branch wins. Target notification order, scheduler policy, random selection, or hidden round-robin fairness must not change this language-visible rule.

**§ 15(5)** Losing branches must not consume message values, join or await capabilities, owned handles, reservations, or outcome payloads; all remaining wait registrations must be cleaned up as prescribed by the selected branch and cancellation contract.

**§ 15(6)** A runtime-free or statically bounded target may use a verified finite state machine, fixed polling, interrupts, or target-provided wait groups only when the implementation provides the exact readiness, progress, source-order priority, non-selected cleanup, and context-safety behavior required by the selected operations.

---

## § 16 Channels

**§ 16(1)** The channel runtime may choose target-specific representations and scheduling adapters for in-process task/thread communication, including statically bounded storage or specialized task-aware and physical-thread-aware wakes. An ordinary `Channel` is not an `IPCSender`/`IPCReceiver` across process isolation; process messaging is owned by `ipc.md`.

**§ 16(2)** Backend specialization is allowed only when it preserves the full public channel source semantics: capacity/rendezvous behavior, close results, send/receive ownership, cancellation and selected-wait commit, message destruction, and required memory publication.

**§ 16(3)** Shared-channel access across physical threads requires valid synchronization and the complete transferability/lifetime proof. Task-only implementation assumptions cannot be applied when an otherwise legal channel may be used across threads.

**§ 16(4)** Any unused runtime-private metadata can be optimized away only if it does not change source-visible channel state, selection priority, completion results, error distinctions, or diagnostics. Optional features are not implicitly present merely because a backend could implement them.

---

## § 17 Mutexes and atomics

**§ 17(1)** `Mutex[T]` may be realized through native or task-aware synchronization only when the exact public lock/guard, cancellation/context, blocking, ownership, release, and memory-ordering contracts of `mutex.md` and the selected `CompilationPlan` are met. A platform spinlock or local interrupt masking is not automatically a semantics-preserving mutex implementation, particularly across processors, tasks, ISRs, or migration boundaries.

**§ 17(2)** `Atomic[T]` operation eligibility, supported widths, alignment, memory order, context safety, and synchronization semantics are defined by `atomics.md` and `concurrency_memory_model.md`. The selected target chooses a verified realization for each concrete atomic operation.

**§ 17(3)** If a target cannot realize the required atomic contract, compilation fails unless the target explicitly supplies a verified **semantically equivalent**, context-legal emulation. A hidden blocking lock fallback is **not** automatically permitted; blocking, ISR, progress, and memory-order requirements remain binding (`atomics.md` § 38).

---

## § 18 Thread-local storage

**§ 18(1)** Thread-local storage may use native TLS, RTOS thread slots, or compiler/runtime-owned per-physical-thread storage, provided the exact `ThreadLocal[T]`, initialization, attachment, borrow, and destruction contracts of `thread_local.md` are met.

**§ 18(2)** One key identifies separate per-physical-thread instances. The initialization policy, including lazy first-access materialization and user-factory failure boundaries, is controlled by `thread_local.md`, not by an eager runtime-wide initialization assumption.

**§ 18(3)** Logical task migration does not carry a physical thread's TLS values to another worker. A thread-affine borrow may not remain live over a scheduling point that could migrate the task unless the relevant pinning/provenance proof is established.

**§ 18(4)** Foreign physical threads need an established Sec attachment before accessing Sec-managed TLS. `Thread.AttachCurrent()` and `ThreadAttachment` are governed by `thread_local.md`; no logical `TaskContext` is fabricated by attaching a foreign thread.

---

## § 19 Detached lifecycle manager

**§ 19(1)** `detach` explicitly transfers the source-level lifecycle responsibility according to the owning task, thread, or process rule. The runtime/target retains only the internal capabilities necessary to finish, publish/discard permitted results, and perform eventual cleanup; it must not create a second visible owner.

**§ 19(2)** Detached cleanup obligations are **kind-specific**. They may include eventual payload disposal, native handle release, TLS destruction at physical-thread exit, process reaping, and removal of completion/observer registrations. Detach does **not** universally request cancellation at normal program shutdown and does not itself establish a completion synchronization edge. The selected target's shutdown/cancellation contract governs any shutdown action.

**§ 19(3)** A profile may reject detachment if it cannot prove the required lifetime, shutdown, cleanup/reaping, allocation, or storage capability. Fixed-capacity detached slots are one possible internal realization; they are not a general public source type.

**§ 19(4)** Neither detach nor program shutdown may silently abandon cleanup obligations, retain invalid scope borrows, fabricate a normal completion result, or implicitly hard-kill an execution entity. Safe continuation of a detached process is further qualified by `processes.md` and the target/platform contract.

---

## § 20 Panic and abnormal termination

**§ 20(1)** The runtime preserves distinct completion domains for each execution kind. Task termination uses `TaskOutcome[T]` (`Completed`, `Cancelled`, `Panicked`, `Failed`); physical threads use their exact terminal `ThreadStatus` and corresponding post-join metadata; processes use their own completion, panic, and termination contracts.

**§ 20(2)** `Terminated` is a source-visible thread/process abnormal-termination category **where their owning rules define it**; it is not a new universal `TaskOutcome[T]` variant and must not be injected into task results or treated as ordinary cooperative cancellation. Selected target panic-containment policy controls whether a panic can be reported rather than terminating the whole program.

**§ 20(3)** No backend may synthesize normal `T` or a successful `Result` when execution ended by panic, cancellation, runtime execution failure, or abnormal hard termination. Failure after successful creation is not retroactively a spawn-creation error.

**§ 20(4)** An implementation may store compact physical status or transport metadata, but it must preserve exact source-visible classification, required diagnostic/observer facts, owned payload destruction, and memory-publication guarantees for its kind and profile.

---

## § 21 Observers

**§ 21(1)** Observer handles are non-owning capabilities with the exact public surface prescribed by `tasks.md`, `threads.md`, or `processes.md`. Observation does not duplicate await/join/detach authority, create an owning lifecycle handle, or grant move-only terminal payload extraction.

**§ 21(2)** The runtime may use generation-indexed static tables, reference-counted internal state, or bounded observer slots provided a successful observation cannot refer to a recycled execution identity as a different live entity.

**§ 21(3)** Observer state must not keep join-only native resources or old source-level owners alive without a rulebook-authorized lifetime. Thread observers are metadata-only and do not acquire a portable `Wait()`/select API merely because process observers may have one.

**§ 21(4)** A profile may bound observer storage/count, but a compiler/runtime must enforce the chosen bounds without silently replacing the public observer behavior or introducing hidden allocation on an allocation-forbidden target.

---

## § 22 Allocation policy

**§ 22(1)** A selected no-dynamic-allocation profile forbids hidden dynamic allocation along the complete executable concurrency operation and its reachable required helpers. Static, stack, caller-provided, and pre-provisioned storage remain permitted when they satisfy their own size, lifetime, and ownership contracts.

**§ 22(2)** Allocation failure follows the owning operation's **actual** result/error model. Pre-creation task, thread, or process allocation failure may be represented by its canonical spawn-error variant; a post-creation execution failure must not be reclassified as spawn failure. Not every concurrent operation returns `Result`, so the runtime must not invent one universally or change public signatures merely to report an allocation failure.

**§ 22(3)** Arena, static, and `ThreadStorage` backing follow their explicit lifetime, alignment, exclusivity, and target-profile constraints. The runtime must not silently substitute an unrelated heap allocator or extend an invalid borrow to make concurrency possible.

**§ 22(4)** Compiler diagnostics and semantic facts must preserve sufficient source/plan provenance to identify a forbidden allocation and its relevant call/effect path under the program's configured diagnostic-analysis level; this rulebook does not mandate one specific diagnostic algorithm or internal allocator representation.

---

## § 23 Foreign thread attachment

**§ 23(1)** A foreign callback can enter Sec on a physical thread not created by Sec. That fact does not create a logical Sec task or make `Task.Current()` valid.

**§ 23(2)** Before using Sec operations requiring an attached physical-thread identity—such as `Thread.Current()`, managed `ThreadLocal[T]`, physical-thread cancellation observation, or thread-specific cleanup—the platform/FFI boundary must establish a valid `ThreadContext` attachment with the correct lifetime and capabilities.

**§ 23(3)** Generated FFI adapters may attach foreign physical threads when their canonical FFI/platform contract permits it. Nested entry must not duplicate an existing attachment or destroy an attachment that the wrapper does not own; cleanup follows `thread_local.md` and foreign-callback unwind/return rules.

**§ 23(4)** Explicit attachment uses the canonical API:

```sec
Thread.AttachCurrent()  // Result[ThreadAttachment, ThreadAttachError]
```

The exact error variants, attachment ownership, and the already-attached failure rule are owned by `thread_local.md`.

**§ 23(5)** Program `main` and Sec-created physical threads begin attached under the canonical physical-thread model. User `ThreadLocal[T]` initializers/factories remain lazy under their own initialization rules; attachment does not eagerly materialize every TLS key.

---

## § 24 ISR integration

**§ 24(1)** ISR legality is a **separate target/context proof**. An operation may be used in an ISR only when its full effects—including generated helpers, allocation, blocking, progress, panic, synchronization, and required runtime state—satisfy `rules/platform/interrupts.md`. A language-level type or function name alone is not proof of ISR safety.

**§ 24(2)** Sec 0.1 ISR execution cannot rely on blocking or scheduler parking, incompatible task suspension, dynamic allocation, invalid TLS initialization/attachment, ordinary mutex acquisition lacking an ISR-safe realization, or unbounded waiting. Fixed-capacity/stack/static storage and explicit `Result` handling remain legal where the full operation and cleanup are proven ISR-safe.

**§ 24(3)** An ISR may communicate with deferred work through verified atomics, target-declared ISR APIs, fixed-capacity queues, or equivalent safe handoff mechanisms. Local interrupt masking is not automatically cross-core, DMA, or device synchronization. Any source `spawn` occurring in ISR context must itself meet the `noAlloc`/`noBlock`/runtime/termination requirements; a blanket `spawn` permission or prohibition is not introduced here.

---

## § 25 Target profile validation

**§ 25(1)** The compiler uses the selected immutable `CompilationPlan` to validate each required concurrency operation's execution kind, target capability, physical or logical context, storage/ownership requirements, allocation domain, blocking/suspension effect, cancellation behavior, ISR safety, resource bounds, synchronization, and supported lowering/cleanup path. Compiler-host capabilities are not substitutes for target facts.

**§ 25(2)** Where a specialized API defines `Preferred(value)` (notably `ThreadSetting[T]`), an unsupported preference follows that API's stable diagnostic and permitted target-default/approximation policy; it may be promoted to an error by target/profile rules. This does not define a universal preference model for all concurrency operations.

**§ 25(3)** A statically unsupported `Required(value)` or unavailable execution capability produces a compile-time target/configuration error, not a runtime spawn error. Unknown or insufficiently proven mandatory target facts must not be silently assumed true or repaired by changing source semantics.

---

## § 26 Core runtime error types

**§ 26(1)** Core-visible execution and concurrency error types use their **exact canonical type identities** declared by the owning feature rules and validated through trusted core/compiler-known declarations. This runtime-model book does not require a universal `core/errors.sec` file: source-file placement and low-level runtime mapping are not public type semantics. The current trusted source layout includes `sec/core/error.sec`, but a normative type is not defined by that filename.

**§ 26(2)** The complete error inventories are owned separately:

- `tasks.md` §§ 5 and 15: `TaskSpawnError` and `TaskError`;
- `threads.md` §§ 18–19: `ThreadStartError` and `ThreadSpawnError`;
- `processes.md` § 5: `ProcessSpawnError`, process completion/termination errors, and distinct `Command` errors;
- `thread_local.md` § 29: `ThreadAttachError`.

A duplicated legacy error declaration in this runtime book is **not** authoritative. In particular, `ProcessSpawnError` has the exact variants `OutOfMemory`, `ResourceLimit`, `PermissionDenied`, `TransferFailed`, `BootstrapFailed`, `NativeFailure` as defined by `processes.md`. `ExecutableNotFound` and `InvalidConfiguration` do not belong to that Sec-callable spawn error. External executable creation has separate `Command` APIs and errors.

**§ 26(3)** Task terminal values use the exact `TaskOutcome[T]` declaration from `tasks.md`; `TaskError` contains exactly `OutOfMemory`, `ResourceLimit`, `ExecutorUnavailable`, and `NativeFailure`. `InvalidConfiguration` belongs to `TaskSpawnError`, not post-creation `TaskError`. Thread termination metadata, thread initialization failures, and process completion/transport failures remain in their own closed error/status categories.

**§ 26(4)** All represented error types and status categories must preserve the selected specialized rulebook's exact variants, parameter types, payload meanings, and error boundary. No generic runtime abstraction, backend alias, or centralized convenience enum may silently add variants, merge creation/operation errors, or reinterpret normal `Completed(Err(E))` as task execution failure.

**§ 26(5)** Source diagnostics for unavailable target capabilities or malformed source contracts are compile-time diagnostics, not source-created runtime error values. Diagnostics use the canonical compiler diagnostic registry; their identity is independent of public `Result` error variants.

---

## § 27 Semantic IR and lowering

**§ 27(1)** Semantic IR must represent resolved source-level execution kind, fallible creation, owning handle identity, input ownership/borrow/transfer facts, runtime capability and selected-plan requirements, cancellation and waiting commit points, completion/terminal classification, observer/lifecycle effects, and relevant provenance before target lowering.

**§ 27(2)** Target lowering may use verified static storage, native threads, an executor, RTOS services, state-machine transformations, direct system calls, or trusted runtime/library adapters when that realization satisfies the exact kind-specific source contract. Process isolation and typed transfer/IPC semantics must remain distinct from in-process task/thread execution.

**§ 27(3)** Lowering must not reconstruct omitted source ownership, cancellation, readiness priority, physical/task identity, target capability, result type, panic category, or completion-publication semantics from raw loads/stores or host-specific assumptions. Compiler-known IR operation identities and concrete Sec MLIR encodings remain owned by `semantic_ir.md` and the dialect books.

**§ 27(4)** Validation and diagnostic/LSP consumers must be able to trace unsupported capabilities and violated runtime requirements to the canonical selected `CompilationPlan` and semantic source location. Internal representation strategy, pass names, and exact operation-lowering order are not additional source-language rules.

---

## § 28 Diagnostics

**§ 28(1)** Compiler diagnostics distinguish at least these failure causes without replacing them with one generic runtime error:

```text
requested execution kind is unavailable on the selected target
required thread configuration or backing storage cannot be realized
thread deferred-start mode is not supported by the selected profile
source operation would require forbidden dynamic allocation
logical task context is required but no current task exists
foreign physical thread has no valid required Sec attachment
ISR operation needs blocking/allocation/runtime facilities forbidden in ISR context
atomic operation has no valid width/order/context-preserving realization
detached execution lacks a valid target cleanup or reaping contract
```

These are **diagnostic categories**, not a new registry of IDs or fixed strings. Exact registered IDs, severities, notes, and help follow `rules/tooling/diagnostics.md` and the owning feature rulebooks.

**§ 28(2)** A diagnostic must explain the violated contract, the resolved execution kind and selected target/context where relevant, and a valid correction path when known. Unknown target proof cannot masquerade as a valid capability. Distinguish source errors, compiler capability failures, runtime `Result` errors, and post-creation terminal outcomes.

---

## § 29 Required synchronization

**§ 29(1)** The normative runtime-boundary requirements in this rulebook are interpreted together with their active canonical owners:

```text
rules/concurrency/spawn.md
rules/concurrency/tasks.md
rules/concurrency/threads.md
rules/concurrency/processes.md
rules/concurrency/concurrency.md
rules/concurrency/await.md
rules/concurrency/scheduling.md
rules/concurrency/blocking.md
rules/concurrency/cancellation.md
rules/concurrency/select.md
rules/concurrency/channels.md
rules/concurrency/mutex.md
rules/concurrency/atomics.md
rules/concurrency/thread_local.md
rules/concurrency/concurrency_memory_model.md
rules/memory/ownership.md
rules/memory/borrowing.md
rules/memory/transferability.md
rules/platform/target_profiles.md
rules/platform/platform_model.md
rules/platform/interrupts.md
rules/platform/ffi.md
rules/compiler/semantic_ir.md
rules/library/core-library.md
```

This dependency list is **not** an implementation checklist. Cross-book implementation work, tests, compatibility cleanup, and verification status are tracked by the corresponding registered governance fragments, principally `governance/concurrency_model.yaml` (`concurrency.runtime-model-v1`). No `.txt` rulebook reference, guessed public runtime storage type, or obsolete `core/errors.sec` location remains normative merely because an older version of this book named it.
