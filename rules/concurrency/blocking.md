# Blocking

- **Status:** Normative
- **Created:** 2026-07-29
- **Last updated:** 2026-11-13 (revision 2.0; normative blocking/suspension and effect synchronization)
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/blocking.md`
- **Replaces:** Revision 1 at the same path; all existing chapter and paragraph IDs preserved
- **Repository baseline reviewed:** `ee75e5d036d6611519a1b34d570bb6778002acfd`
- **Implementation governance:** `governance/concurrency_blocking.yaml` (`concurrency.blocking-v2`, `frontend.blocking-effects`)
- **Related rulebooks:** `rules/analysis/effect_analysis.md`, `rules/analysis/deadlock_analysis.md`, `rules/foundations/attributes.md`, `rules/concurrency/await.md`, `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/concurrency/processes.md`, `rules/concurrency/ipc.md`, `rules/concurrency/channels.md`, `rules/concurrency/select.md`, `rules/concurrency/mutex.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/scheduling.md`, `rules/concurrency/concurrency_runtime_model.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/platform/target_profiles.md`, `rules/platform/interrupts.md`, `rules/platform/ffi.md`, `rules/compiler/initialization.md`, `rules/compiler/semantic_ir.md`, `rules/tooling/diagnostics.md`, `rules/tooling/lsp.md`
- **Applied correction:** `rules/corrections/applied/blocking-normative-fix-20261113-20261113.md`

---

## § 1 Purpose

**§ 1(1)** Blocking defines operations that wait for progress by another execution entity,
device, timer or platform facility.

**§ 1(2)** Sec distinguishes:

```text
task suspension
physical thread blocking or parking
busy waiting
interrupt context
```

**§ 1(3)** The same source operation may suspend a task or block a physical thread depending
on the caller context and selected backend.

**§ 1(4)** Ownership, commit and memory-order semantics must not change.

---

## § 2 Core distinction

**§ 2(1)** A source-level waiting operation has its own normative readiness, cancellation, commit, ownership, and memory-order contract. The target/runtime may realize an eligible wait by suspending the **logical task** when a real current task exists and the selected `CompilationPlan` provides a compatible suspension/readiness mechanism. It must preserve task identity across suspension and permitted migration.

**§ 2(2)** In a physical-thread execution context without a logical current task, a waiting operation may park or block the physical thread according to its owning API and target/profile policy. Ordinary `main` has a physical `ThreadContext` but does **not** acquire a fabricated root `TaskContext` merely because the backend implements waits using task-runtime machinery.

**§ 2(3)** Physical blocking or parking does not, by itself, invalidate a thread-local reference when the current physical-thread attachment generation remains the same. Logical task suspension may allow migration; such suspension may not carry live thread-affine or TLS borrows across a migration point without the proof required by `scheduling.md`, `thread_local.md`, and borrowing/transferability rules.

**§ 2(4)** Busy waiting polls for progress without completing a suspension or parking operation. It is distinct from a non-waiting `Try...` operation, from a scheduler yield, and from a synchronization edge.

**§ 2(5)** Source-language wait semantics do not require one physical realization. A busy-spin fallback, executor-worker block, scheduler park, native wait, or registration protocol is legal only if the selected profile and the operation's own contracts prove its progress, cancellation, latency, synchronization, and execution-context obligations. Such a choice must not silently change a task into a thread or vice versa.

---

## § 3 Potentially blocking operations

**§ 3(1)** The following **operation families** may wait according to their specialized rulebooks. This is a classification inventory, not a new declaration of methods on every type:

```text
Task[T] owning await, and task owning join
Thread[T] owning join
Process[T] and Command owning join
ProcessObserver.Wait()
ordinary Sender[T].Send / Sender[T].SendRevocable / Receiver[T].Receive
PipeReader.Read / ReadExact and PipeWriter.Write / WriteAll
IPCSender[T].Send / IPCReceiver[T].Receive
IPCMutex.Lock / IPCSemaphore.Acquire / SharedValue[T].Lock
select when no branch is immediately ready and no default branch applies
Mutex[T].Lock(), Lock(duration), Lock(ref Context)
timer/after waits, I/O readiness/completion, and target-declared wait facilities
```

Actual waiting depends on current readiness, chosen operation mode, and the relevant target/profile facts. A successfully completed, already-ready operation need not physically block or suspend merely because it belongs to a potentially waiting family.

**§ 3(2)** A `Try...` operation is non-waiting only when the owning rulebook provides that guarantee. Its result can still report contention, unavailable capacity, closure, or an ordinary API error. Neither a method name alone nor a generic implementation strategy establishes the guarantee.

**§ 3(3)** A valid `select` with a `default` branch is non-waiting at the source level: it selects a ready branch according to the source-order priority rule, or its default when none is ready. A `select` without default may wait. Preparation and losing-branch rollback remain subject to `select.md`; `default` does not create a memory barrier or imply that selected branch bodies are themselves nonblocking.

---

## § 4 Await

**§ 4(1)** `await task` accepts an owning `Task[T]` as specified by `await.md`. It is task-**operand**-specific; this does not imply that the **caller** must always have a current logical `TaskContext`. A normal `main` may await an owned task when the target supports the operation.

**§ 4(2)** Successful owning await commit consumes the `Task[T]` handle and transfers the exact `TaskOutcome[T]` result; it does not return bare `T`, flatten a nested `Result`, or silently change task failure/cancellation into ordinary callable return.

**§ 4(3)** If the child is not terminal, a real task caller normally suspends when the runtime supports suitable readiness registration. A completed task may be awaited without a suspension. The scheduling model must not fabricate task identity or erase logical cancellation and migration constraints.

**§ 4(4)** An await implementation may physically block an executor worker only when the selected target/profile permits that realization and executor starvation, deadlock, cancellation, cleanup, and completion guarantees remain satisfied. An ordinary physical-thread caller follows the applicable physical waiting path, not an invented task-suspension requirement.

**§ 4(5)** If **caller** cancellation wins before owning-await commit, the task lifecycle has already been consumed into the await operation. The canonical `await.md` cleanup retains that lifecycle internally, requests cooperative child cancellation, waits as necessary for terminal child completion, destroys the terminal outcome exactly once, and only then completes caller cancellation. It neither restores the source handle nor implicitly detaches the child. The target chooses a compatible physical wait mechanism without changing this source-level rule.

---

## § 5 Join

**§ 5(1)** `join handle` is a **kind-specific** owning completion operation for `Task[T]`, `Thread[T]`, `Process[T]`, and `Command` where their owning rulebooks define it. Successful join consumes the applicable **one-shot join capability**, not necessarily the entire source-visible handle; it does not itself mean task await, value extraction, detach, or cancellation.

**§ 5(2)** A real logical task may suspend while joining an incomplete operation, whereas a physical thread may park/block in accordance with its selected profile. Whether a particular join is immediately ready, selectable, or cancellation-aware is determined by its owning rulebook. A backend may not force caller cancellation to consume, lose, or silently detach a join owner.

**§ 5(3)** Joining a physical thread from task context uses task-aware completion registration/suspension when supported. Physical blocking of an executor worker is an explicit target/profile choice subject to deadlock and starvation proof, not a change to the physical thread's identity.

**§ 5(4)** Joining a physical thread from physical-thread-only context may block/park that physical thread. Deferred `ThreadStartMode.Deferred` remains a distinct source-visible state; a wait implementation may not covertly start the deferred callable to satisfy join.

**§ 5(5)** Successful task/thread join provides only the **completion synchronization** that `concurrency_memory_model.md` assigns to that kind of execution. It does not retroactively legalize earlier races. Other wait operations and observers establish a synchronization edge only where their own rules explicitly promise one.

**§ 5(6)** Successful `Process[T]` or `Command` join establishes the process/command completion and native cleanup/reaping guarantees described by `processes.md`. It proves required child quiescence and preserves the kind-specific allowed terminal metadata/payload access. It does **not** publish ordinary child-process addresses or process-private memory into the parent; explicit IPC/shared-memory facilities have separate ordering and visibility contracts.

**§ 5(7)** `ProcessObserver.Wait()` is non-owning, repeatable terminal observation. It performs no owner join, reaping, detach, termination, or terminal payload extraction. It may suspend in a task or park/block in a physical thread under the same selected-profile constraints; its existence does not make a `ProcessObserver` an owning `Process[T]`.

---

## § 6 Channel operations

**§ 6(1)** Ordinary `Sender[T]`/`Receiver[T]` channel send and receive remain **in-process** operations. A task-suspending backend and a physical-thread-blocking backend must implement the same channel capacity, closure, message ownership, result-variant, readiness, and commit contracts from `channels.md`. They do not become typed IPC merely because a target uses a native queue internally.

**§ 6(2)** When a channel operation must wait and a real task suspension facility is available, it may suspend that task. An already-ready operation may commit without suspension. Cancellation is governed by its owning channel rule, independently of whether a scheduler reschedules the current task.

**§ 6(3)** In a physical-thread execution context, a waiting channel operation may park/block according to the target/profile; it must not fabricate a current task merely to use a queue implementation.

**§ 6(4)** Current-execution cancellation or an explicit `select` timeout may win **before** channel operation commit, but neither may partly transfer a message. Ordinary channel APIs do not automatically adopt an unrelated `Context` in lexical scope, and they do not gain an invented `Cancelled` result variant. They retain the exact results declared by `channels.md`.

**§ 6(5)** A committed send or receive transfers ownership according to its exact channel result; a non-committed, cancelled, or losing branch does not transfer the message. A later cancellation request cannot roll back a committed send/receive or silently discard a move-only returned payload.

---

## § 7 Select

**§ 7(1)** `select` follows `select.md`: prepare eligible operands exactly once, determine semantic readiness, and commit **at most one** selected operation. If multiple operations are ready at the same semantic choice point, **source-order priority** selects the earliest ready branch. The runtime may register candidate readiness while waiting but cannot replace this policy with round-robin or random selection.

**§ 7(2)** In a real task context, a waiting select may suspend the logical task using a compatible readiness backend. A no-default select whose earlier selected operation is already ready need not suspend.

**§ 7(3)** In a physical-thread-only context, a waiting select may park/block as permitted by its profile. It does not thereby create a source-level `TaskContext` or make `select` a task-only construct.

**§ 7(4)** A branch that is not selected must not commit a message transfer, receive removal, mutex/permit acquisition, owning task/thread/process join, terminal payload transfer, or other source-visible operation effect. Prepared ownership remains conditional until selected commit. Losing registrations are removed, and cancellation cleanup of prepared values follows `select.md` and ordinary ownership/destruction rules.

---

## § 8 Mutex acquisition

**§ 8(1)** `Mutex[T].Lock()` is potentially waiting and returns an owning `MutexGuard[T]` only after acquisition commits. The distinct `TryLock()`, `Lock(duration)`, and `Lock(ref Context)` overload contracts are owned by `mutex.md`; this rulebook does not define a competing `LockError`, timeout result, or cancellation enum.

**§ 8(2)** In task context, an available task-aware mutex implementation should suspend the logical task while waiting rather than block the worker. It must preserve guard execution-identity binding, cancellation/acquisition commit rules, and mutex release/acquire synchronization.

**§ 8(3)** In physical-thread-only context, mutex acquisition may park/block the physical thread according to the selected target/profile. It does not silently change a mutex to a task-local lock merely because the current source is executing inside an executor.

**§ 8(4)** Mutex ownership, non-reentrancy, guard lifetime and release/acquire memory-order semantics remain identical across valid physical implementations. A cancelled/expired `Context` is an **operation-local** `ContextError` outcome, not an automatic terminal cancellation of the current execution entity.

**§ 8(5)** The runtime must not substitute a task-local lock for a mutex that may be shared across physical-thread boundaries. Any backend specialization must be supported by compiler-proven storage/transferability and selected-profile facts.

**§ 8(6)** Logical task suspension, executor-worker physical blocking, and direct physical-thread parking remain distinct effects. Ordinary `main` has no implicit root task, while `Task.Yield()` is a task scheduling/cancellation point and `Thread.Yield()` is a physical scheduling hint; neither creates a synchronization edge by itself.

---

## § 9 Mutex guards across waits

**§ 9(1)** A live `MutexGuard[T]` must obey `mutex.md` § 41: it cannot cross `await`, a potentially suspending/blocking `join`, `select`, or any other compiler-classified suspension boundary without an explicitly standardized exception; none exists for a pinned guard in Sec 0.1. `Task.Yield()` is a task scheduling and cancellation point that may enable migration, so a live execution-bound guard must not be carried across it contrary to these restrictions. For `IPCMutexGuard` and `SharedValueGuard[T]`, `ipc.md` additionally forbids crossing **any potentially waiting/suspending operation**, including relevant pipe, IPC send/receive, mutex, semaphore, and shared-value waits. The actual guard must be live across the boundary for a violation; a guard destroyed before the wait does not violate the rule.

**§ 9(2)** For physical-thread-only waits, apply the precise guard and execution-affinity restrictions from `mutex.md` and `ipc.md`. Do not invent a universal ban on every operation that happens to block a physical thread when the owning guard contract defines only suspension or a more specific boundary. Where an operation's blocking/suspension classification is unknown, mandatory guard-safety proof must fail closed rather than assume the guard can be safely retained.

**§ 9(3)** The restrictions preserve the actual safety obligations: execution-bound guard identity, lock-order and progress proof, task migration and thread-affine state, and bounded critical-section behavior. They do not imply that a successful wait automatically unlocks a guard, or that mutex release/acquire ordering is produced merely by parking a worker.

---

## § 10 Cancellation-aware waits

**§ 10(1)** Current-execution cancellation is a distinct semantic event from a supplied `Context` cancellation/deadline or a `select after` timeout. A cancellation-aware waiting operation participates in the canonical **first-commit** race defined by `cancellation.md` and the owning operation rulebook.

**§ 10(2)** For typed IPC, a cancellation or timeout win before operation commit leaves the queue/message/lock/permit state and caller ownership as specified by `ipc.md`. After successful commit, the operation has its ordinary committed effects. IPC error/result types do not acquire an unapproved `Cancelled` variant merely because the runtime can wake a waiter for cancellation.

**§ 10(3)** The exact owner of a wait defines its result and commit boundary. If current-execution cancellation wins first, no partially committed operation may remain; if the operation wins first, its committed result and cleanup obligations exist and cannot be revoked by a later cancellation observation. The two outcomes must not both commit or both disappear through a wakeup race.

**§ 10(4)** For ordinary channel send before commit, ownership of a message remains subject to the original channel/conditional-select preparation state; it is not irrevocably lost to an unselected, cancelled send. A committed send follows `ChannelSendResult[T]` or the exact other selected send-family result declared by `channels.md`.

**§ 10(5)** Cancellation does not fabricate an `Err(Cancelled)`, drop a move-only message, perform half a semaphore acquisition, or transfer a partially received result. Distinct `ContextError` and execution-cancellation paths must not be conflated.

**§ 10(6)** If current-execution cancellation commits, deterministic cleanup runs according to the owning task/thread, await/join, defer, destruction and guard rules. In particular, a caller-cancelled consuming `await` retains the child lifecycle and completes the exact child cleanup required by `await.md` before terminal caller cancellation.

---

## § 11 Timeouts

**§ 11(1)** Waiting-operation timeout may be composed through `select` and its canonical `after duration` branch. Other timeout forms exist only when defined by an owning API, such as `Mutex[T].Lock(duration)` or `Lock(ref Context)`. A `Context` supplied to one API does not implicitly attach to unrelated channel operations.

**§ 11(2)** Example of a canonical timeout competition:

```sec
select {
    message := rx.Receive() => {
        Use(message)
    }

    after 500<ms> => {
        HandleTimeout()
    }
}
```

The duration uses Sec's general units/temporal model, and timeout measurement follows the monotonic preparation/deadline rule in `select.md`.

**§ 11(3)** If the timeout branch wins, no other branch is committed and prepared losing-branch ownership is restored or cleaned according to `select.md`. If a ready operation commits first, a later timeout cannot undo its effects.

**§ 11(4)** Timeout is not terminal task/thread cancellation. A `ContextError.DeadlineExceeded`, an `after` branch, and explicit `cancel` have distinct owners, results, and cleanup consequences.

---

## § 12 Nonblocking operations

**§ 12(1)** A non-waiting operation returns without waiting for external progress or acquiring a deferred readiness registration. It may still perform finite computation, read current state, attempt immediate acquisition, return an error, or produce a result that reports unavailable capacity/contention.

**§ 12(2)** Canonical non-waiting operation examples include `Mutex[T].TryLock()`, channel `TrySend`/`TryReceive`, and non-waiting status observation where the owning type declares them. Their **exact** return types and observable side effects belong to the owning rulebooks.

**§ 12(3)** Non-waiting does not mean success, lock-free implementation, no dynamic allocation, no I/O, no panic, ISR-safety, or bounded worst-case execution time. Each is a separate effect/target/progress property to establish where required.

**§ 12(4)** The non-waiting guarantee is that the operation completes without semantic waiting; it does not permit `@noBlock` verification to ignore synchronous effects in helper calls, cleanup, foreign adapters, or target-specific lowerings.

---

## § 13 Busy waiting

**§ 13(1)** A backend must not silently replace an ordinary waiting operation with an unbounded polling loop. A polling realization is permitted only when the selected profile and the operation's normative progress/readiness semantics prove it valid. Busy waiting is not automatically safe because it avoids a sleeping syscall; it may starve an owner that must run to make progress.

**§ 13(2)** An explicitly permitted busy-wait implementation must respect target limits on CPU use, bounded progress where required, interrupt preemption and wakeup, cancellation observation/commit, power/latency, and any relevant allocation or no-block contract. Such implementation facts are profile/lowering concerns, not a new portable spin-wait API.

**§ 13(3)** Compiler-generated polling must preserve the operation's actual atomic, volatile, and memory-visibility requirements. Volatile is not atomic or synchronization; an atomic access uses the exact memory order and target-verified lowering from `atomics.md`. An unbounded polling loop may still violate ISR, no-block, progress, and deadline requirements even when each individual read is non-waiting.

---

## § 14 Blocking effects

**§ 14(1)** Effect analysis represents **`MayBlock`** (synchronous blocking/physical execution-domain occupation) and **`MaySuspend`** (logical asynchronous task suspension) as independent compiler facts owned by `rules/analysis/effect_analysis.md`. They are not a new source-visible four-keyword effect syntax. The compiler may also track exact wait causes, target requirements and execution contexts.

**§ 14(2)** Sec's existing portable source guarantee is `@noBlock`. This rulebook does not define public `@noSuspend`, `mayBlock`, `noBlock`, or `maySuspend` declaration modifiers independently of the canonical attribute rulebook.

**§ 14(3)** `@noBlock` requires proof that reachable execution does **not** synchronously block or park the relevant physical execution domain under the selected `CompilationPlan`. Proof includes ordinary calls, indirect/foreign calls, generics, generated helpers, cleanup, and target-specific physical waiting strategies. Unknown blocking behavior cannot satisfy a verified guarantee.

**§ 14(4)** `MaySuspend` does not imply `MayBlock`. A task-aware wait can be `MaySuspend` while not occupying the physical worker; `@noBlock` does **not** itself prohibit that suspension. Conversely, allowing task suspension does not authorize hidden physical blocking when the selected executor contract forbids it.

**§ 14(5)** This distinction must survive source analysis, callable summaries, Semantic IR, target validation, and LSP presentation. A function that is physically nonblocking on one selected profile may require a different target-specialized summary on another; source guarantees must never be validated by assuming a convenient lowering unavailable to the actual target.

**§ 14(6)** Process and command operations receive their effects from the owning `processes.md` declarations plus validated target/runtime behavior. This includes `spawn process`, `Command.Start()`, `Command.Validate()`, `Command.SetWorkingDirectory()`, termination requests, owner joins, observer waits, pipe operations, and process-shared synchronization. Name, syntax, or apparent immediacy alone does not establish an absence of filesystem/native blocking effects; a creation/configuration operation is not automatically pure or non-waiting.

---

## § 15 Call graph analysis

**§ 15(1)** The compiler consumes canonical resolved call/effect summaries to determine possible `MayBlock` and `MaySuspend` behavior, target-specific wait realization, reachable blocking from ISR or verified-`@noBlock` code, guard-live-across-wait hazards, executor starvation, and deadlock/progress dependencies. It must include concrete calls, reachable generic specializations, relevant indirect dispatch, foreign contracts, and deterministic cleanup/defer effects as required by `effect_analysis.md` and `deadlock_analysis.md`.

**§ 15(2)** Unknown or open-world call targets must retain conservative effect facts. A programmer-visible guarantee is not proven by absence of a known callee, missing imported summary, a profiler guess, or a runtime that happened not to block in one test. Spawned execution is a separate execution edge; a blocking call **inside the spawned callable** is not, by itself, synchronous blocking of the spawner, while the spawn operation may still have its own creation effects.

---

## § 16 FFI

**§ 16(1)** A foreign operation without a sufficiently trusted applicable no-block contract is conservatively classified as potentially blocking for a verified `@noBlock` guarantee, even if a particular test call completed immediately. Foreign callback execution, reentrancy, and thread attachment are independently governed by `ffi.md` and the execution-context rulebooks.

**§ 16(2)** An applicable `@noBlock` foreign declaration is a **trusted effect contract**: the declaration/provenance must satisfy `attributes.md`, `effect_analysis.md`, and FFI requirements. This paragraph does not standardize a special `unsafe extern` modifier order or create an ad-hoc foreign annotation syntax.

**§ 16(3)** A false trusted foreign declaration is an unsafe contract breach. A verified guarantee in Sec source cannot be treated as valid when a required foreign effect remains unknown or is contrary to the declared contract.

**§ 16(4)** The compiler cannot inspect arbitrary foreign machine behavior to establish its real blocking properties. The validated foreign summary and its trust origin must therefore be explicit semantic inputs to target/effect/ISR verification, not inferred from a function's name or native ABI.

---

## § 17 ISR context

**§ 17(1)** ISR execution must meet `rules/platform/interrupts.md` and `@interruptSafe` constraints. Ordinary task suspension, awaiting, owning join waits, blocking channel/IPC operations, waiting `select`, scheduler parking, sleeping, and a blocking mutex acquisition are invalid. `Task.Yield()` and `Thread.Yield()` are also invalid ISR operations. Even a non-sleeping spin wait may be invalid if bounded progress and the selected target's ISR contract cannot be proven.

**§ 17(2)** Process creation, command start/validation/working-directory operations, hard termination, process/command join, and observer waiting have the exact ISR legality determined by their specialized contracts; ordinary forms are not ISR-safe in Sec 0.1. A narrower immutable metadata read is not automatically safe without the applicable ISR proof. **Spawn is not categorically forbidden in an ISR**: `interrupts.md` permits it only where the complete creation/handoff operation is proven no-allocation, no-block, bounded, runtime-compatible and otherwise ISR-safe; a callable that will run later does not make its creation operation safe by itself.

**§ 17(3)** ISR safety is a composite proof, not the mere absence of `MayBlock`. It includes `MaySuspend`, panic/allocation constraints, interrupt reentrancy, duration/progress limits, target-specific atomic lowering, permitted cleanup/FFI effects, and the legality of compiler-generated helpers. When a required proof is unavailable, the compiler must not silently choose a blocking or allocation-based fallback.

---

## § 18 Main and initialization

**§ 18(1)** Program startup, static initialization, ordinary program entry, and shutdown are distinct stages governed by `rules/compiler/initialization.md` and the selected `CompilationPlan`. The existence of a runtime or task scheduler does not create implicit module initializers or a source-visible root task in ordinary `main`.

**§ 18(2)** Compile-time evaluation/static initialization cannot introduce hidden runtime waiting or task suspension. An embedded or other restricted target may disallow waits in particular startup phases according to its initialization/target policy. This paragraph does not standardize a new `@init` attribute.

**§ 18(3)** Hosted executable code in an ordinary legal execution context may perform waits only where the selected target and owning operation rules allow them. Permission to wait in `main` does not imply permission to call `Task.Current()` or `Task.Yield()` outside a real task, or to block a startup phase whose runtime dependencies are not yet available.

**§ 18(4)** Initialization/dependency analysis must diagnose proven waiting cycles, including waiting for execution or resources that cannot become available before the waiting startup dependency completes. Potential cycles follow the diagnostic classification of `deadlock_analysis.md`; the blocking rulebook must not introduce hidden startup threads or fabricated completion edges as a workaround.

---

## § 19 Deadlock interaction

**§ 19(1)** Waiting operations supply dependency/progress facts to the dedicated `rules/analysis/deadlock_analysis.md` owner. This rulebook does not define a separate deadlock algorithm or a parallel diagnostic registry.

**§ 19(2)** Relevant facts include held mutex/IPC guard identities, task/thread/process joins and await dependencies, channel and IPC backpressure, semaphore permits, pipe readiness, detached lifecycle and shutdown obligations, deferred thread start, executor worker occupancy, cancellation wakeups, and target-proven progress resources. The analysis must distinguish task suspension from physical worker occupation and must not treat a ready or non-selected operation as a committed wait.

**§ 19(3)** A statically **proven** deadlock or incompatible mandatory progress contract is a compiler error according to the owning deadlock rulebook and target profile. A warning or optional analysis level must not suppress a mandatory proven invalidity.

**§ 19(4)** A potential cycle lacking sufficient proof is not automatically a proven deadlock; its uncertainty, severity, and diagnostic detail follow `deadlock_analysis.md`, with source provenance and interprocedural facts preserved for tooling.

---

## § 20 Memory ordering

**§ 20(1)** Waiting, suspension, parking, cancellation wakeup, timer expiry, or scheduling does **not** create a memory synchronization edge merely because control flow pauses or resumes. The owning operation and `concurrency_memory_model.md` must explicitly define the edge and its commit conditions.

**§ 20(2)** Applicable synchronization examples include successful **task** join/await completion observation, successful **thread** join, mutex release-to-matching-acquire, committed channel message transfer, and the separately defined IPC/shared-memory/atomic synchronization contracts. Process join proves process completion and native cleanup but does **not** publish ordinary private child-process memory; process-shared memory and typed IPC require their own rules.

**§ 20(3)** Sleeping, `Task.Yield()`, `Thread.Yield()`, timeout, status polling, and mere registration for readiness do not by themselves publish ordinary memory. Losing `select` branches and uncommitted/cancelled waits cannot acquire a successful-operation synchronization edge by accident.

---

## § 21 Runtime errors

**§ 21(1)** `blocking.md` introduces no standalone runtime `BlockingError` type. It is not the owner of new wait-related result or error variants.

**§ 21(2)** Operation-specific runtime errors and non-error result variants belong to their exact declaring rulebooks: e.g. `ContextError` for supported Context-aware APIs, channel send/receive result types from `channels.md`, typed IPC and pipe error types from `ipc.md`, and creation/terminal errors from the task/thread/process books. This rulebook must not mandate the obsolete physical source path `core/errors.sec`; actual compiler-known/core declarations remain governed by their canonical source and core-library contracts.

**§ 21(3)** Violations such as blocking in a proven no-block or ISR context, invalid guard-across-wait, unavailable target capability, and proven deadlock are compiler diagnostics, **not** runtime error variants. Diagnostic identity, severity, source ranges and mentor explanations follow `rules/tooling/diagnostics.md` and the specialist analysis rulebooks.

---

## § 22 Semantic IR

**§ 22(1)** Semantic IR must preserve **semantic wait categories and commitments**, not require the historical pseudo-opcode-name list in revision 1 to exist literally. Facts include: the owning operation identity; requested and resolved execution kind/context; readiness and prepared ownership; potential `MayBlock` and `MaySuspend`; branch/operation commit versus cancellation; required wakeup and cleanup; guard lifetime and affinity; source-order `select` priority; target capability and provenance; and the exact synchronization edge where one exists. Concrete opcode names, verifier rules, and MLIR lowering are owned by `rules/compiler/semantic_ir.md` and its lowering/target contracts.

**§ 22(2)** Lowering must resolve target-specific physical suspension/blocking/parking/polling while preserving original operation, result, ownership, terminal-cancellation, synchronization and progress semantics. An unsupported required backend operation must fail closed; it may not silently block an executor worker, invent an allocation, fabricate task identity, or replace an ISR-invalid wait with an unproven spin loop. Effect summaries and selected-profile decisions remain inspectable for diagnostics/LSP.

---

## § 23 Diagnostics

**§ 23(1)** Illustrative mentor diagnostics (not newly allocated diagnostic IDs):

```text
blocking operation is not permitted in @noBlock function Poll
interrupt routine UartIRQ cannot perform a blocking or suspending wait
MutexGuard[State] remains live across a forbidden suspension boundary
all executor workers may block waiting for tasks scheduled on the same executor
busy-wait lowering cannot satisfy the selected target's progress and ISR contract
foreign blocking behavior is not proven safe for an @noBlock guarantee
```

The owning diagnostic registry and specialist rulebooks determine the actual stable IDs, severity, structured locations, help and whether a specific statement is a proven error or an uncertainty diagnostic.

**§ 23(2)** Diagnostics must use stable registered IDs and explain the invalid operation, relevant execution/target context, causal call path where applicable, and an actionable safe repair where one exists. Do not infer a code fix by changing `await` into `join`, inserting `Task.Yield()`, or silently changing execution kind or memory synchronization.

---

## § 24 Required synchronization

**§ 24(1)** This book's contracts are consumed together with the following active authorities; the current implementation and conformance status is tracked **only** in `governance/concurrency_blocking.yaml` and its specialist governance owners:

```text
rules/analysis/effect_analysis.md
rules/analysis/deadlock_analysis.md
rules/foundations/attributes.md
rules/concurrency/await.md
rules/concurrency/tasks.md
rules/concurrency/threads.md
rules/concurrency/processes.md
rules/concurrency/ipc.md
rules/concurrency/channels.md
rules/concurrency/select.md
rules/concurrency/mutex.md
rules/concurrency/scheduling.md
rules/concurrency/cancellation.md
rules/concurrency/concurrency_runtime_model.md
rules/concurrency/concurrency_memory_model.md
rules/platform/target_profiles.md
rules/platform/interrupts.md
rules/platform/ffi.md
rules/compiler/initialization.md
rules/compiler/semantic_ir.md
rules/tooling/diagnostics.md
rules/tooling/lsp.md
```

This is a **normative dependency map**, not an imperative list of implementation tasks. No incomplete task, coverage percentage or verification result belongs here.
