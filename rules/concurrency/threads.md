# Threads

- **Status:** Normative
- **Created:** 2026-09-24
- **Last updated:** 2026-09-24
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/threads.md`
- **Replaces:** Earlier legacy revision at the same canonical path
- **Repository baseline reviewed:** `main-reviewed-2026-09-24`
- **Implementation governance:** `governance/concurrency_thread.yaml`
- **Related rulebooks:** `rules/concurrency/concurrency.md`, `rules/concurrency/spawn.md`, `rules/concurrency/tasks.md`, `rules/concurrency/await.md`, `rules/concurrency/select.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/blocking.md`, `rules/concurrency/scheduling.md`, `rules/concurrency/structured_concurrency.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/concurrency/thread_local.md`, `rules/concurrency/mutex.md`, `rules/errors/panic.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/transferability.md`, `rules/memory/destruction.md`, `rules/compiler/semantic_ir.md`, `rules/platform/platform_model.md`, `rules/platform/target_profiles.md`, `rules/tooling/lsp.md`

---

## § 1. Purpose and authority

**Governance tags:** `concurrency.thread-v2`

§ 1(1) A Sec thread is an explicit physical/native target execution identity.

§ 1(2) A `Thread[T]` is distinct from:

- a logical `Task[T]`;
- a process;
- the physical executor thread currently running a migratable task;
- an interrupt service routine.

§ 1(3) `spawn thread` requests a physical thread or the selected target profile's canonical native-thread equivalent.

§ 1(4) A backend must never silently lower `spawn thread` to an ordinary task.

§ 1(5) This rulebook owns the physical thread source model, thread configuration, explicit thread storage, lifecycle, join, detach, terminal status/result metadata, observation, current-thread context, cancellation-facing thread members, and thread-specific compiler/runtime obligations.

§ 1(6) Mutable implementation status belongs only in `governance/concurrency_thread.yaml`.

---

## § 2. Canonical creation syntax

**Governance tags:** `concurrency.thread-v2`, `frontend.thread-v2`

§ 2(1) The basic form is:

```sec
let worker := try spawn thread Work()
```

§ 2(2) If `Work()` returns `T`, the raw expression before `try` has type:

```sec
Result[Thread[T], ThreadSpawnError]
```

§ 2(3) The successful payload is:

```sec
Thread[T]
```

§ 2(4) Thread creation is eager by default.

§ 2(5) A target/profile without physical-thread support rejects `spawn thread` at compile time.

§ 2(6) Unsupported physical threads are not represented by `ThreadSpawnError`.

---

## § 3. Complete compiler-known/source-visible type set

**Governance tags:** `concurrency.thread-v2`, `tooling.thread-v2`

§ 3(1) The canonical Sec 0.1 thread surface materially owned by this rulebook includes:

```text
Thread[T]
ThreadObserver[T]
ThreadConfig
ThreadSetting[T]
ThreadStartMode
ThreadPriority
ThreadStatus
ThreadID
ThreadContext
ThreadPlatform
ThreadStorage
CpuSet
ThreadTerminationKind
ThreadTermination
ThreadSpawnError
ThreadStartError
```

§ 3(2) `PanicInfo` is owned by `panic.md` and is reused rather than redefined.

§ 3(3) Legacy compiler-known names that have no canonical public Sec 0.1 operation are not retained merely because an earlier implementation introduced an internal identity.

§ 3(4) In particular, this revision does not define portable public `ThreadSchedulingError` or `ThreadContextError` APIs.

§ 3(5) A target-specific platform extension may define additional exact types under its own rulebook without enlarging the portable thread surface implicitly.

---

## § 4. Exact `ThreadSetting[T]`

**Governance tags:** `concurrency.thread-v2`, `frontend.thread-v2`

§ 4(1) The exact declaration is:

```sec
type ThreadSetting[T] union {
    // Use the selected target/profile default.
    Default

    // Request this value as a preference.
    // The target may ignore or approximate it only under the diagnostic
    // rules defined by this thread configuration model.
    Preferred(T)

    // Require this value as part of the program's thread configuration.
    // A target/configuration that cannot guarantee it must be rejected.
    Required(T)
}
```

§ 4(2) There is no implicit conversion from plain `T` to `Preferred(T)`.

§ 4(3) Source code must write `Preferred(value)` or `Required(value)` explicitly when it is not using `Default`.

§ 4(4) This avoids configuration-specific coercion magic.

---

## § 5. Exact `ThreadStartMode`

**Governance tags:** `concurrency.thread-v2`

§ 5(1) The exact declaration is:

```sec
enum ThreadStartMode {
    // Creation may begin executing the callable as soon as creation commits.
    Eager

    // Creation establishes the physical thread identity but the user callable
    // must not execute until Start() commits successfully.
    Deferred
}
```

§ 5(2) `Eager` is the default start mode.

§ 5(3) `Deferred` is a semantic requirement, not a preference.

---

## § 6. Exact `ThreadPriority`

**Governance tags:** `concurrency.thread-v2`

§ 6(1) The exact portable declaration is:

```sec
enum ThreadPriority {
    // Lowest portable relative scheduler priority.
    Lowest

    // Lower than the target's normal/default thread priority.
    Low

    // Portable normal/default relative priority.
    Normal

    // Higher than normal.
    High

    // Highest portable relative scheduler priority.
    Highest
}
```

§ 6(2) These values are relative portable intents, not native numeric priorities.

§ 6(3) `Highest` does not imply real-time scheduling.

§ 6(4) Real-time scheduling policy, priority bands, deadlines, or native scheduler classes require separately specified target/platform APIs.

---

## § 7. Exact `CpuSet`

**Governance tags:** `concurrency.thread-v2`, `compiler.platform-model`

§ 7(1) `CpuSet` is a compiler-known immutable set of canonical logical CPU indices:

```sec
type CpuSet
```

§ 7(2) Its canonical literal form is:

```sec
CpuSet { 2, 3 }
```

§ 7(3) `CpuSet` is not an alias of `set[uint]`.

§ 7(4) Duplicate indices in one literal are rejected or canonicalized as the same set member according to ordinary set-literal diagnostics; they do not name multiple CPU instances.

§ 7(5) Every index must be valid for the selected target/variant when statically knowable.

§ 7(6) A target with no CPU-affinity concept may still parse/type `CpuSet`, but applying it as `Required(...)` must fail target validation.

---

## § 8. Exact `ThreadStorage`

**Governance tags:** `concurrency.thread-v2`, `analysis.borrowing`

§ 8(1) `ThreadStorage` is not a value-generic type.

§ 8(2) Its canonical declaration shape is:

```sec
@noCopy
type ThreadStorage struct {
    _capacity: uint

    init(capacity: uint) {
        _capacity = capacity
    }
}

impl ThreadStorage {
    property Capacity: uint {
        get {
            return _capacity
        }
    }
}
```

§ 8(3) The constructor argument `capacity` is compile-time-required.

§ 8(4) Construction reserves compiler/target-managed backing storage with the selected target's required alignment.

§ 8(5) `ThreadStorage(capacity)` performs no dynamic allocation.

§ 8(6) `_capacity` is source-private; the actual reserved stack/control backing storage is compiler/platform-managed hidden state.

§ 8(7) `Capacity` is immutable after construction.

---

## § 9. Thread storage example

**Governance tags:** `concurrency.thread-v2`

§ 9(1) Canonical explicit-storage use is:

```sec
static let mut workerStorage := ThreadStorage(65536)

let config := ThreadConfig {
    Name: Default
    Stack: Required(65536)
    Affinity: Default
    Priority: Default
    Start: ThreadStartMode.Deferred
    Storage: Some(ref mut workerStorage)
}

let worker := try spawn thread <-config Work()
```

§ 9(2) Explicit backing storage is a semantic requirement.

§ 9(3) It must not be silently replaced by heap/runtime stack allocation.

---

## § 10. Exact `ThreadConfig`

**Governance tags:** `concurrency.thread-v2`, `frontend.thread-v2`

§ 10(1) The exact declaration is:

```sec
@noCopy
type ThreadConfig struct {
    Name: ThreadSetting[string]
    Stack: ThreadSetting[uint]
    Affinity: ThreadSetting[CpuSet]
    Priority: ThreadSetting[ThreadPriority]
    Start: ThreadStartMode
    Storage: Option[ref mut ThreadStorage]

    init() {
        Name = Default
        Stack = Default
        Affinity = Default
        Priority = Default
        Start = ThreadStartMode.Eager
        Storage = None
    }
}
```

§ 10(2) `ThreadConfig` is `@noCopy` because it may hold an exclusive mutable storage borrow.

§ 10(3) A named `ThreadConfig` consumed by `spawn thread` uses the ordinary call-site move marker:

```sec
let worker := try spawn thread <-config Work()
```

§ 10(4) A fresh inline configuration value requires no synthetic move marker.

---

## § 11. Inline thread configuration

**Governance tags:** `concurrency.thread-v2`, `frontend.thread-v2`

§ 11(1) The thread grammar may provide contextual inline configuration sugar:

```sec
let worker := try spawn thread {
    Name: Preferred("worker")
    Stack: Required(65536)
    Affinity: Preferred(CpuSet { 2, 3 })
    Priority: Preferred(ThreadPriority.High)
    Start: ThreadStartMode.Eager
} Work()
```

§ 11(2) The block materializes one fresh `ThreadConfig`.

§ 11(3) Omitted fields use the exact `ThreadConfig.init()` defaults.

§ 11(4) A plain value such as:

```sec
Priority: ThreadPriority.High
```

does not implicitly mean `Preferred(ThreadPriority.High)`.

§ 11(5) Such a type mismatch is diagnosed normally.

---

## § 12. Preference semantics

**Governance tags:** `concurrency.thread-v2`, `compiler.platform-model`

§ 12(1) `Preferred(value)` requests target realization without making exact realization part of portable program correctness.

§ 12(2) If a preference cannot be implemented:

- the compiler emits a stable diagnostic;
- target/profile policy may promote the diagnostic to an error;
- the thread remains semantically valid using the target default or documented approximation.

§ 12(3) The compiler/runtime must not claim the preference was honored when it was not.

§ 12(4) A preference that would alter language-level correctness must instead be expressed as `Required`.

---

## § 13. Required semantics

**Governance tags:** `concurrency.thread-v2`, `compiler.platform-model`

§ 13(1) `Required(value)` makes realization of that value a semantic requirement.

§ 13(2) If the selected CompilationPlan can prove the requirement unsupported, compilation fails.

§ 13(3) If realization depends on runtime/native resource creation and can still fail after static validation, the applicable `ThreadSpawnError` or `ThreadStartError` represents that runtime failure.

§ 13(4) The compiler/runtime must never downgrade `Required(value)` to a silent preference.

---

## § 14. Configuration target validation

**Governance tags:** `concurrency.thread-v2`, `compiler.platform-model`

§ 14(1) Stack, affinity, priority, deferred start, and explicit storage are validated against the selected immutable `CompilationPlan`.

§ 14(2) Host-machine support is irrelevant unless the host is also the selected target.

§ 14(3) Target/profile limits may reject configurations such as:

- impossible CPU sets;
- stack below a required minimum;
- stack above a target maximum;
- unsupported deferred start;
- unsupported explicit storage;
- unsupported required affinity;
- unsupported required priority.

§ 14(4) Statically unsupported requirements are compile-time diagnostics, not runtime error variants.

---

## § 15. Eager start

**Governance tags:** `concurrency.thread-v2`

§ 15(1) With `ThreadStartMode.Eager`, the user callable may begin immediately after creation commits.

§ 15(2) A successful eager `spawn thread` may return when the thread is:

- Running;
- Completed;
- Cancelled;
- Panicked;
- Terminated.

§ 15(3) Source code must not assume its first observed `Status` is `Running`.

§ 15(4) `Created` is reserved for a successfully created deferred thread whose callable has not started.

---

## § 16. Deferred start

**Governance tags:** `concurrency.thread-v2`

§ 16(1) With `ThreadStartMode.Deferred`, successful creation yields:

```text
Status == ThreadStatus.Created
```

until start commits.

§ 16(2) The user callable must not execute before successful `Start()`.

§ 16(3) A target may realize deferred creation through native suspended creation, an RTOS primitive, or an internal start gate.

§ 16(4) The physical mechanism is irrelevant if no user callable code executes before start commit.

---

## § 17. `Start()`

**Governance tags:** `concurrency.thread-v2`

§ 17(1) The owning thread handle exposes:

```sec
fn Start() Result[void, ThreadStartError]
```

§ 17(2) `Start()` does not consume `Thread[T]`.

§ 17(3) Successful start changes a valid deferred `Created` thread into live execution eligibility.

§ 17(4) The thread may already be terminal before source code next reads `Status`.

§ 17(5) Calling `Start()` on an eager thread or already-started/terminal thread is invalid.

§ 17(6) When statically provable, invalid start state is a compile-time diagnostic.

§ 17(7) Otherwise it returns `Err(ThreadStartError.InvalidState)`.

---

## § 18. Exact `ThreadStartError`

**Governance tags:** `concurrency.thread-v2`, `tooling.thread-v2`

§ 18(1) The exact declaration is:

```sec
enum ThreadStartError error {
    // The thread is not in a state where deferred start can commit.
    InvalidState

    // A runtime/native resource needed to start the already-created
    // deferred thread is temporarily or permanently unavailable.
    ResourceUnavailable

    // The runtime/native environment denied the requested start operation.
    PermissionDenied

    // A target/runtime start failure occurred and is not represented
    // by another ThreadStartError variant.
    NativeFailure
}
```

§ 18(2) Comments documenting variants appear before the variants.

---

## § 19. Exact `ThreadSpawnError`

**Governance tags:** `concurrency.thread-v2`, `tooling.thread-v2`

§ 19(1) The exact declaration is:

```sec
enum ThreadSpawnError error {
    // Memory required for the thread/control representation could not
    // be obtained.
    OutOfMemory

    // The target/runtime thread resource limit was reached.
    ResourceLimit

    // Required runtime-provided stack/backing storage could not be
    // established.
    StackAllocationFailed

    // A runtime-derived configuration valid at source/target validation
    // could not be instantiated.
    InvalidConfiguration

    // The native environment denied creation under the requested settings.
    PermissionDenied

    // Required thread-local initialization failed before callable execution
    // was committed.
    ThreadLocalInitializationFailed

    // A target/runtime creation failure occurred and is not represented
    // by another ThreadSpawnError variant.
    NativeFailure
}
```

§ 19(2) Unsupported physical-thread capability is not a `ThreadSpawnError`.

§ 19(3) A statically invalid required configuration is not a `ThreadSpawnError`.

---

## § 20. Spawn ownership commit

**Governance tags:** `concurrency.thread-v2`, `frontend.transferability`

§ 20(1) Thread spawn evaluates callable/arguments/configuration in ordinary Sec evaluation order.

§ 20(2) Values crossing the thread boundary follow ordinary move/copy/borrow semantics plus thread transferability/lifetime rules.

§ 20(3) A named move-only source value passed by value uses `<-`.

§ 20(4) Successful thread creation commits transferred ownership to the new thread.

§ 20(5) Failure before creation commit must preserve or clean up source ownership according to the canonical spawn/ownership transaction rules; a backend must not invent a thread-specific leak or double-destruction path.

§ 20(6) A consumed `ThreadConfig` that fails before successful thread creation releases its configuration-held borrows during ordinary failure cleanup.

---

## § 21. Explicit storage borrow lifetime

**Governance tags:** `concurrency.thread-v2`, `analysis.borrowing`

§ 21(1) `Storage: Some(ref mut storage)` grants exclusive use of that `ThreadStorage` to the created thread lifecycle.

§ 21(2) On successful creation, that mutable borrow remains active until the thread/runtime no longer uses the backing storage.

§ 21(3) For a joined thread, the borrow is released only after join has completed all join-owned native cleanup.

§ 21(4) For a detached thread, the runtime may retain the borrow until detached terminal cleanup is complete.

§ 21(5) Source code must not reuse or mutate the storage while the exclusive borrow remains active.

§ 21(6) A detached thread using local/scope storage is invalid if the storage may die before detached cleanup.

---

## § 22. Exact `ThreadStatus`

**Governance tags:** `concurrency.thread-v2`, `concurrency.cancellation-v2`

§ 22(1) The exact declaration is:

```sec
enum ThreadStatus {
    // A deferred physical thread exists but its callable has not started.
    Created

    // The thread callable is live: running, runnable, or waiting.
    Running

    // The callable returned normally.
    Completed

    // Cooperative thread cancellation committed terminally.
    Cancelled

    // The callable terminated through a contained Sec panic.
    Panicked

    // Unsafe/platform-level abnormal termination prevented normal
    // Sec completion.
    Terminated
}
```

§ 22(2) Backend-private states may exist but must map to this exact public state model.

§ 22(3) `Cancelled`, `Panicked`, and `Terminated` are distinct terminal categories.

---

## § 23. Exact termination metadata

**Governance tags:** `concurrency.thread-v2`

§ 23(1) The exact portable termination-kind declaration is:

```sec
enum ThreadTerminationKind {
    // A Sec owner/platform control path requested hard termination and the
    // terminal state can be attributed to that request.
    Requested

    // A platform actor outside the owning Sec thread capability terminated
    // the thread.
    External

    // Execution ended because of a native fault/crash-equivalent event.
    Fault

    // Abnormal termination is known but cannot be classified more precisely.
    Unknown
}
```

§ 23(2) The exact portable payload is:

```sec
type ThreadTermination struct {
    // Portable high-level termination classification.
    Kind: ThreadTerminationKind
}
```

§ 23(3) The portable type contains no universal signal number, exception code, or native status integer.

§ 23(4) Target-specific immutable termination metadata belongs to the selected platform declaration.

---

## § 24. Exact `ThreadID`

**Governance tags:** `concurrency.thread-v2`

§ 24(1) The canonical declaration is opaque:

```sec
type ThreadID
```

§ 24(2) `ThreadID` is copyable and immutable.

§ 24(3) It is Sec's stable logical identity for one physical-thread execution identity.

§ 24(4) It is distinct from:

- `TaskID`;
- `ProcessID`;
- a native OS/RTOS thread identifier;
- a raw owning native handle.

§ 24(5) Sec 0.1 defines no arithmetic over `ThreadID`.

---

## § 25. Exact `Thread[T]`

**Governance tags:** `concurrency.thread-v2`, `analysis.transferability`

§ 25(1) The canonical compiler-known/source-visible type identity is:

```sec
@noCopy
type Thread[T]
```

§ 25(2) `T` is the callable's complete declared normal return type.

§ 25(3) `Thread[T]` owns:

- one unresolved thread lifecycle obligation;
- one join capability until consumed;
- deferred-start authority while applicable;
- cooperative cancellation-request authority;
- terminal status;
- normal result storage when applicable;
- panic metadata when applicable;
- abnormal termination metadata when applicable;
- join-owned native resources until join/detach resolution.

§ 25(4) Moving `Thread[T]` transfers all unresolved ownership.

§ 25(5) Copying it is invalid even when `T` is copyable.

---

## § 26. Exact public `Thread[T]` surface

**Governance tags:** `concurrency.thread-v2`, `tooling.thread-v2`

§ 26(1) The required portable public surface is exactly:

```text
property ID: ThreadID
property Name: string
property Status: ThreadStatus
property Value: T
property Panic: PanicInfo
property Termination: ThreadTermination
property Platform: ThreadPlatform
fn Observe() ThreadObserver[T]
fn Start() Result[void, ThreadStartError]
fn RequestCancel() void
```

§ 26(2) All properties are read-only from ordinary source.

§ 26(3) Join and detach remain language operations, not methods.

§ 26(4) Public names use Sec CamelCase.

§ 26(5) Legacy lowercase spellings such as `worker.id`, `worker.status`, `worker.value`, and `worker.platform` are non-canonical.

---

## § 27. `ID`

**Governance tags:** `concurrency.thread-v2`

§ 27(1) `ID` is available for every successfully created `Thread[T]`.

§ 27(2) Moving the handle preserves the same `ThreadID`.

§ 27(3) Joining preserves the same `ThreadID`.

§ 27(4) Native identity reuse does not reuse Sec `ThreadID` identity.

---

## § 28. `Name`

**Governance tags:** `concurrency.thread-v2`

§ 28(1) `Name` is immutable Sec observation/diagnostic metadata.

§ 28(2) A configured preferred/required name becomes the Sec logical name when creation succeeds.

§ 28(3) When `Name` is `Default`, the compiler/runtime derives a stable diagnostic name from the callable/current creation context.

§ 28(4) The logical Sec `Name` is not required to equal a truncated/native OS thread title.

§ 28(5) Native naming metadata, when exposed, belongs to `ThreadPlatform`.

---

## § 29. `Status`

**Governance tags:** `concurrency.thread-v2`

§ 29(1) `Status` is a nonblocking immutable snapshot.

§ 29(2) Reading `Status` does not join the thread.

§ 29(3) Reading `Status` does not establish full completion synchronization.

§ 29(4) Reading a terminal status does not by itself make terminal payload properties available where successful join is also required.

---

## § 30. `Value`

**Governance tags:** `concurrency.thread-v2`, `analysis.transferability`

§ 30(1) `Value` has exact type `T`.

§ 30(2) `Value` is available only after:

```text
successful join
+
Status == ThreadStatus.Completed
```

§ 30(3) For copyable `T`, ordinary repeated reads follow normal copy semantics.

§ 30(4) For move-only `T`, extracting `Value` transfers ownership from the hidden result slot and makes that result unavailable for a second extraction.

§ 30(5) Result extraction does not make the outer `Thread[T]` an ordinary partially moved aggregate.

§ 30(6) `Thread[void].Value` has type `void` after normal joined completion.

---

## § 31. `Panic`

**Governance tags:** `concurrency.thread-v2`, `errors.panic-v2`

§ 31(1) `Panic` has exact type:

```sec
PanicInfo
```

§ 31(2) It is available only after:

```text
successful join
+
Status == ThreadStatus.Panicked
```

§ 31(3) `PanicInfo` is the canonical panic-owned type; this rulebook does not define a thread-specific copy.

§ 31(4) If the selected panic policy cannot contain a panic at the thread boundary, the program/runtime may terminate before source-level `Panicked` observation is possible.

---

## § 32. `Termination`

**Governance tags:** `concurrency.thread-v2`

§ 32(1) `Termination` has exact type:

```sec
ThreadTermination
```

§ 32(2) It is available only after:

```text
successful join
+
Status == ThreadStatus.Terminated
```

§ 32(3) The portable payload reports only the portable classification.

§ 32(4) Native signal/fault/status metadata must not be fabricated into a portable integer.

---

## § 33. Terminal availability table

**Governance tags:** `concurrency.thread-v2`

§ 33(1) After successful join:

```text
Completed
    Value available
    Panic unavailable
    Termination unavailable

Cancelled
    Value unavailable
    Panic unavailable
    Termination unavailable

Panicked
    Value unavailable
    Panic available
    Termination unavailable

Terminated
    Value unavailable
    Panic unavailable
    Termination available
```

§ 33(2) `Created` and `Running` cannot remain the observed status after a successfully committed join.

---

## § 34. Join semantics

**Governance tags:** `concurrency.thread-v2`, `concurrency.select-v2`

§ 34(1) Canonical syntax is:

```sec
join worker
```

§ 34(2) Join waits for terminal physical-thread completion.

§ 34(3) A successful join:

- establishes thread-completion synchronization;
- consumes the one-shot join capability;
- releases join-owned native resources;
- releases explicit storage borrow when native cleanup no longer needs it;
- preserves the `Thread[T]` handle;
- preserves `ID`, `Name`, terminal `Status`, `Platform`;
- preserves unconsumed terminal payloads.

§ 34(4) Join does not itself return `T`.

§ 34(5) Join does not itself consume a copyable or move-only normal result.

§ 34(6) A second join is invalid.

---

## § 35. Selectable join

**Governance tags:** `concurrency.thread-v2`, `concurrency.select-v2`

§ 35(1) `join Thread[T]` is selectable in Sec 0.1.

§ 35(2) Readiness means:

```text
the physical thread is terminal
and
the outstanding join capability can commit without further waiting
```

§ 35(3) If selected, join performs exactly the ordinary commit in § 34.

§ 35(4) If not selected:

- join capability remains available;
- no native resource is released by join;
- no result availability is unlocked by join;
- no completion synchronization is established by that branch.

§ 35(5) Native event-notification order does not override select source-order priority.

---

## § 36. Join from task context

**Governance tags:** `concurrency.thread-v2`, `concurrency.blocking-v1`

§ 36(1) When executed by a logical task, waiting join should suspend the task when the selected backend can register thread completion without blocking the executor worker.

§ 36(2) A backend may physically block only where the selected profile permits that fallback.

§ 36(3) The source lifecycle/result semantics are identical regardless of the physical waiting implementation.

---

## § 37. Join from physical-thread context

**Governance tags:** `concurrency.thread-v2`, `concurrency.blocking-v1`

§ 37(1) When executed directly by a physical thread, waiting join may park/block that physical thread.

§ 37(2) Join must not busy-wait unless the selected target/profile explicitly defines an allowed bounded busy-wait strategy.

§ 37(3) ISR context may not perform thread join in Sec 0.1.

---

## § 38. Join cancellation

**Governance tags:** `concurrency.thread-v2`, `concurrency.cancellation-v2`

§ 38(1) A cancellation-aware waiting join is a current-execution cancellation point.

§ 38(2) Join commit and current-execution cancellation compete under the universal cancellation-point commit rule.

§ 38(3) If join commits first, the join capability is consumed and terminal handle state/payload availability are real.

§ 38(4) If caller cancellation commits first:

- join does not commit;
- the join capability remains owned;
- no terminal payload is newly made available by join;
- no join-owned resource is released by that failed wait.

§ 38(5) The surrounding cancellation cleanup must still resolve the owning `Thread[T]` lifecycle explicitly according to structured lifecycle rules.

§ 38(6) This rule does not invent implicit detach, implicit hard termination, or implicit join-after-cancellation.

---

## § 39. Unresolved lifecycle obligation

**Governance tags:** `concurrency.thread-v2`, `analysis.transferability`

§ 39(1) An unresolved `Thread[T]` must not be silently destroyed at ordinary scope exit.

§ 39(2) Before its owning path ends, it must be:

- joined;
- detached where valid;
- moved to another valid owner;
- otherwise consumed by an explicitly standardized lifecycle operation.

§ 39(3) Cancellation and error-propagation edges participate in this lifecycle analysis.

§ 39(4) A function whose cancellation/error path can abandon an unresolved thread handle must be rejected unless cleanup resolves it.

---

## § 40. Observer declaration

**Governance tags:** `concurrency.thread-v2`, `tooling.thread-v2`

§ 40(1) The canonical observer type is:

```sec
type ThreadObserver[T]
```

§ 40(2) `ThreadObserver[T]` is copyable.

§ 40(3) It is non-owning and never duplicates the `Thread[T]` lifecycle owner.

§ 40(4) Its exact public surface is:

```text
property ID: ThreadID
property Name: string
property Status: ThreadStatus
property Platform: ThreadPlatform
```

§ 40(5) All observer properties are read-only.

§ 40(6) `ThreadObserver[T]` exposes no `Value`, `Panic`, `Termination`, `Start`, `RequestCancel`, join capability, detach capability, or hard-termination authority.

---

## § 41. Creating an observer

**Governance tags:** `concurrency.thread-v2`

§ 41(1) An owning thread creates an observer through:

```sec
fn Observe() ThreadObserver[T]
```

§ 41(2) Observer creation is infallible.

§ 41(3) It does not consume or borrow away lifecycle ownership.

§ 41(4) Destroying an observer does not affect the physical thread.

---

## § 42. Observer lifetime

**Governance tags:** `concurrency.thread-v2`

§ 42(1) An observer may outlive movement, join, or detach of the owning `Thread[T]`.

§ 42(2) Minimal retained observation state may include:

- `ThreadID`;
- logical name;
- current/terminal `ThreadStatus`;
- immutable target-resolved platform metadata.

§ 42(3) An observer must not keep join-only native resources, result payload storage, panic payload storage, termination payload storage, or unresolved explicit-storage ownership alive solely for observation.

§ 42(4) Observer retention must not require garbage collection or an owning reference cycle.

---

## § 43. Observer and select

**Governance tags:** `concurrency.thread-v2`, `concurrency.select-v2`

§ 43(1) Sec 0.1 does not define an exact public blocking `Wait()` operation on `ThreadObserver[T]`.

§ 43(2) Therefore `ThreadObserver[T]` is not itself a primitive selectable completion wait in Sec 0.1.

§ 43(3) The compiler/runtime must not synthesize a hidden observer-select operation.

§ 43(4) A future exact observer wait may be added only through a synchronized rulebook/API revision.

---

## § 44. Detach

**Governance tags:** `concurrency.thread-v2`, `frontend.discard-v2`

§ 44(1) A `Thread[void]` may be detached:

```sec
detach worker
```

§ 44(2) A non-void thread requires explicit result discard:

```sec
detach worker discard
```

§ 44(3) Detach consumes the owning source handle.

§ 44(4) Detach transfers lifecycle/native cleanup responsibility to the runtime/program lifecycle manager.

§ 44(5) Detach does not establish completion synchronization.

§ 44(6) Detached execution continues independently of the consumed source handle.

---

## § 45. Detach and borrowed state

**Governance tags:** `concurrency.thread-v2`, `analysis.borrowing`

§ 45(1) A detached thread must not retain a reference to scope-owned state that may die before the thread finishes using it.

§ 45(2) Static/program-lifetime storage may still require race/synchronization proof.

§ 45(3) Explicit `ThreadStorage` supplied to a detached thread must outlive runtime cleanup.

§ 45(4) If the compiler cannot prove the required lifetime, detach is rejected.

---

## § 46. Cooperative cancellation

**Governance tags:** `concurrency.thread-v2`, `concurrency.cancellation-v2`

§ 46(1) The owning handle exposes:

```sec
fn RequestCancel() void
```

§ 46(2) `RequestCancel()`:

- is idempotent;
- does not consume the handle;
- requests cancellation of that physical thread identity;
- has no effect after terminal completion;
- does not force immediate termination.

§ 46(3) It does not resolve the lifecycle obligation.

§ 46(4) The owner must still join, detach, or transfer the handle.

---

## § 47. Current physical thread

**Governance tags:** `concurrency.thread-v2`, `concurrency.cancellation-v2`

§ 47(1) The canonical current-thread lookup is:

```sec
Thread.Current()
```

§ 47(2) It returns:

```sec
ThreadContext
```

§ 47(3) `ThreadContext` is an immutable non-owning view of the current physical/native thread.

§ 47(4) It is not `Thread[T]`.

§ 47(5) It may represent:

- the program main thread;
- a Sec-created physical thread;
- an attached foreign FFI thread;
- an executor worker currently running a logical task.

---

## § 48. Exact `ThreadContext` surface

**Governance tags:** `concurrency.thread-v2`, `tooling.thread-v2`

§ 48(1) The canonical compiler-known/source-visible identity is:

```sec
type ThreadContext
```

§ 48(2) The exact portable public surface is:

```text
property ID: ThreadID
property Name: string
property CancelRequested: bool
property Platform: ThreadPlatform
```

§ 48(3) `ThreadContext` has no join, detach, result, start, or owner cancellation-request authority.

§ 48(4) `CancelRequested` is a live immutable observation of cancellation request state for the current physical thread identity.

§ 48(5) `CancelRequested` is not a selectable operation.

---

## § 49. `Thread.Current()` declaration

**Governance tags:** `concurrency.thread-v2`

§ 49(1) The canonical static surface is:

```sec
impl Thread {
    static fn Current() ThreadContext
    static fn Yield() void
}
```

§ 49(2) `Thread.Current()` requires a target/profile with a physical-thread context.

§ 49(3) It continues to refer to the physical worker even when that worker currently runs a logical task.

§ 49(4) A backend must not fabricate a `ThreadContext` from `Task.Current()` identity.

---

## § 50. `Thread.Yield()`

**Governance tags:** `concurrency.thread-v2`, `concurrency.scheduling-v1`

§ 50(1) `Thread.Yield()` requests that the native scheduler give another runnable physical thread an opportunity to execute.

§ 50(2) It is a scheduling hint.

§ 50(3) It guarantees none of:

- fairness;
- that another thread runs;
- a physical context switch;
- memory synchronization.

§ 50(4) `Thread.Yield()` is invalid in ISR context.

§ 50(5) `Task.Yield()` remains a separate logical-task operation.

---

## § 51. Thread-local state

**Governance tags:** `concurrency.thread-v2`

§ 51(1) Thread-local storage belongs to physical `Thread` identity, not logical task identity.

§ 51(2) A migratable task therefore must not assume physical thread-local identity remains stable across task suspension/resumption.

§ 51(3) Thread-local initialization failure during native creation uses:

```sec
ThreadSpawnError.ThreadLocalInitializationFailed
```

where that initialization is required before creation commit.

§ 51(4) Detailed thread-local declaration semantics remain owned by `thread_local.md`.

---

## § 52. Exact `ThreadPlatform` model

**Governance tags:** `concurrency.thread-v2`, `compiler.platform-model`

§ 52(1) `ThreadPlatform` is a compiler-known target-resolved immutable thread platform view:

```sec
type ThreadPlatform
```

§ 52(2) It intentionally has no single fixed portable struct layout.

§ 52(3) Every thread-capable selected platform must provide a complete concrete Sec declaration for its resolved `ThreadPlatform`.

§ 52(4) Every concrete declaration must expose at least a read-only native identity member:

```text
property ID: <target-native thread identity type>
```

§ 52(5) The placeholder above is specification notation, not a Sec source type; the selected platform must replace it with one exact declared Sec type.

§ 52(6) `ThreadPlatform.ID` is not `ThreadID`.

§ 52(7) Raw owning or mutating native-thread handles are not part of the common portable platform view.

---

## § 53. Unsafe/platform hard termination boundary

**Governance tags:** `concurrency.thread-v2`, `compiler.platform-model`

§ 53(1) Safe portable Sec 0.1 defines no general hard-kill method on `Thread[T]`.

§ 53(2) A selected platform may define an unsafe target-specific hard-termination operation only in its fully specified platform API.

§ 53(3) Such an operation must define:

- its exact capability/receiver;
- exact error type;
- interaction with Sec lifecycle ownership;
- resulting `ThreadStatus`;
- whether join remains required;
- native resource cleanup;
- effects on `ThreadStorage`;
- panic/destructor/defer guarantees.

§ 53(4) This portable rulebook does not expose the legacy underspecified `worker.platform.Terminate()` as a universal method.

§ 53(5) A successfully observed abnormal hard/platform termination is represented portably by `ThreadStatus.Terminated` plus `ThreadTermination`.

---

## § 54. Normal completion

**Governance tags:** `concurrency.thread-v2`

§ 54(1) `Completed` means the callable returned its declared `T` normally.

§ 54(2) If `T` is:

```sec
Result[V, E]
```

then returning `Err(E)` is still:

```text
ThreadStatus.Completed
```

§ 54(3) The returned `Result[V, E]` remains inside `Value`.

§ 54(4) Thread runtime logic must not inspect generic `T` to reclassify application-level errors.

---

## § 55. Cancelled completion

**Governance tags:** `concurrency.thread-v2`, `concurrency.cancellation-v2`

§ 55(1) `Cancelled` means cooperative cancellation committed terminally for the thread.

§ 55(2) A cancellation request alone does not force `Cancelled`.

§ 55(3) If the callable returns normally before cancellation terminally commits, `Completed` wins.

§ 55(4) `Cancelled` has no normal `Value`, `Panic`, or `Termination` payload.

---

## § 56. Panicked completion

**Governance tags:** `concurrency.thread-v2`, `errors.panic-v2`

§ 56(1) `Panicked` means a Sec panic escaped the callable and was contained at the physical thread boundary by the selected panic policy.

§ 56(2) The canonical payload is `PanicInfo`.

§ 56(3) `Panicked` is not `Terminated`.

§ 56(4) A target policy that terminates the whole program/process instead of containing thread panic may prevent source-level terminal observation.

---

## § 57. Terminated completion

**Governance tags:** `concurrency.thread-v2`

§ 57(1) `Terminated` means the physical thread ended abnormally outside normal return/cooperative cancellation/contained panic semantics.

§ 57(2) Examples may include unsafe termination, external native termination, or a fault that the selected runtime can classify without terminating the whole program first.

§ 57(3) `ThreadTerminationKind.Unknown` must be used rather than inventing a false classification.

§ 57(4) Hard termination provides no portable guarantee that user `defer`, destructors, lock release, or thread-local cleanup ran.

---

## § 58. Memory publication on spawn

**Governance tags:** `concurrency.thread-v2`, `concurrency.memory-model-v2`

§ 58(1) Successful thread creation publishes all moved, copied, and valid borrowed arguments/captures to the new thread.

§ 58(2) Writes sequenced before committed creation happen-before the new thread's first access to those published values as required by the concurrency memory model.

§ 58(3) Failed creation establishes no child-execution publication edge.

§ 58(4) Ordinary shared mutation after creation still requires valid synchronization.

---

## § 59. Completion synchronization

**Governance tags:** `concurrency.thread-v2`, `concurrency.memory-model-v2`

§ 59(1) Successful join establishes thread-completion synchronization.

§ 59(2) Ordinary writes in the thread sequenced before terminal completion become visible to the successful joining continuation according to the canonical happens-before rule.

§ 59(3) Merely polling `Status` does not replace join synchronization.

§ 59(4) Detach creates no completion synchronization edge for the former owner.

§ 59(5) `RequestCancel()` is not a completion synchronization operation.

---

## § 60. Thread arguments and borrows

**Governance tags:** `concurrency.thread-v2`, `analysis.transferability`, `analysis.borrowing`

§ 60(1) Owned values may cross the thread boundary by move when parameter/capture semantics consume them.

§ 60(2) Copyable values may cross by copy.

§ 60(3) Shared/mutable references may cross only when analysis proves:

- owner lifetime;
- address stability;
- alias legality;
- concurrent access legality;
- thread escape/detach lifetime;
- target/runtime representation validity.

§ 60(4) A mutable borrow passed to a thread remains exclusive until thread completion/lifecycle resolution releases it.

§ 60(5) Thread creation does not extend an otherwise invalid borrow.

---

## § 61. Move-only thread results

**Governance tags:** `concurrency.thread-v2`, `analysis.transferability`

§ 61(1) A move-only `T` is stored exactly once on normal completion.

§ 61(2) Successful joined extraction through `Value` transfers ownership exactly once.

§ 61(3) Detaching a non-void thread with `discard` commits explicit eventual result destruction.

§ 61(4) A runtime must destroy an unconsumed terminal move-only result exactly once when the joined owner is later destroyed.

---

## § 62. Destruction of joined handles

**Governance tags:** `concurrency.thread-v2`, `memory.destruction`

§ 62(1) After successful join, destruction of `Thread[T]` may reclaim remaining metadata/runtime bookkeeping.

§ 62(2) Any still-owned normal result/panic/termination payload is destroyed according to ordinary payload rules.

§ 62(3) Join-owned native resources must not be released twice.

§ 62(4) A moved-out `Value` must not also be destroyed by the thread handle.

---

## § 63. Destruction of unresolved handles

**Governance tags:** `concurrency.thread-v2`, `memory.destruction`

§ 63(1) Source-level destruction of an unresolved owning `Thread[T]` is not an implicit detach.

§ 63(2) It is not an implicit join.

§ 63(3) It is not an implicit cancellation request.

§ 63(4) Static lifecycle analysis should reject paths that reach such destruction.

§ 63(5) Defensive runtime/compiler checks may diagnose an invariant violation but must not silently choose lifecycle policy.

---

## § 64. Data-race analysis

**Governance tags:** `concurrency.thread-v2`, `analysis.data-races`

§ 64(1) Physical threads may execute simultaneously and share address-space storage.

§ 64(2) Shared mutable data therefore participates in ordinary thread/task data-race analysis.

§ 64(3) Ownership transfer that leaves one exclusive owner does not itself create a race.

§ 64(4) Borrowed/shared aliases require synchronization compatible with their accesses.

§ 64(5) Thread-local storage is not shared merely because the owning `ThreadContext` can be observed elsewhere.

---

## § 65. Deadlock/starvation analysis

**Governance tags:** `concurrency.thread-v2`, `sema.deadlock-analysis`

§ 65(1) Thread join contributes a wait edge from the waiting execution to the joined physical thread.

§ 65(2) A join executed from a task may additionally consume/park executor capacity depending on the selected lowering.

§ 65(3) Cancellation of the join wait does not automatically resolve the underlying owned thread lifecycle.

§ 65(4) Deadlock/starvation analysis must therefore distinguish:

- wait cancellation;
- physical thread terminal completion;
- lifecycle ownership resolution;
- executor-worker blocking.

§ 65(5) Holding incompatible live guards across join remains governed by blocking/mutex rules.

---

## § 66. Target/CompilationPlan requirements

**Governance tags:** `concurrency.thread-v2`, `compiler.platform-model`

§ 66(1) The selected immutable `CompilationPlan` is target truth for physical-thread behavior.

§ 66(2) Relevant facts may include:

- physical thread availability;
- native thread creation;
- stack/storage alignment and limits;
- maximum thread count;
- affinity model;
- priority model;
- deferred-start capability;
- thread-local initialization;
- join/wait primitives;
- task-to-thread wait integration;
- cancellation wakeup support;
- target-resolved `ThreadPlatform`;
- detached-thread runtime management.

§ 66(3) Compiler-host behavior must not substitute for selected target facts.

§ 66(4) Platform support and compiler implementation support are separate diagnostics.

---

## § 67. ISR restrictions

**Governance tags:** `concurrency.thread-v2`, `compiler.platform-model`

§ 67(1) Sec 0.1 ISR code must not perform:

```text
spawn thread
Thread[T].Start()
join Thread[T]
detach Thread[T]
Thread.Yield()
```

§ 67(2) `RequestCancel()` from ISR is permitted only if the selected target/platform explicitly marks that exact path bounded and interrupt-safe.

§ 67(3) Ordinary portable source must not assume ISR-safe cancellation request support.

§ 67(4) Immutable metadata reads may be permitted only where the target/core implementation satisfies canonical ISR access rules.

---

## § 68. Semantic analysis

**Governance tags:** `frontend.thread-v2`, `analysis.transferability`

§ 68(1) Sema must validate at least:

- selected target thread support;
- callable and complete return type `T`;
- raw spawn result type;
- `ThreadConfig`;
- explicit `ThreadSetting[T]` wrapping;
- `CpuSet`;
- stack/affinity/priority/start/storage requirements;
- `ThreadStorage` compile-time capacity;
- config ownership and mutable storage borrow;
- thread argument/capture transfer;
- deferred-start state;
- one join capability;
- selectable join;
- terminal-property availability;
- move-only `Value` extraction;
- observer restrictions;
- detach/discard rules;
- cancellation authority/context;
- platform-view type;
- panic/termination payload identity;
- thread-local requirements;
- unresolved lifecycle on every control-flow edge.

§ 68(2) Sema must reject legacy implicit plain-value-to-Preferred conversion.

§ 68(3) Sema must reject legacy lowercase public member names as canonical API.

§ 68(4) Sema must not synthesize observer `Wait()` or observer select.

---

## § 69. Semantic IR requirements

**Governance tags:** `semantic-ir.thread-v2`

§ 69(1) Semantic IR must preserve enough explicit thread facts that lowering never reconstructs semantics from arbitrary calls/names.

§ 69(2) Required facts include:

- physical-thread execution kind;
- concrete `T`;
- callable/arguments/captures;
- `ThreadConfig` and each `ThreadSetting` classification;
- target-resolved configuration decisions;
- explicit storage identity/capacity/borrow;
- creation prepare/commit/failure;
- `ThreadID`;
- deferred/eager start state;
- one-shot start state;
- one-shot join capability;
- selectable-join readiness/commit;
- current-execution cancellation race for join;
- result-slot ownership/availability;
- panic/termination payload;
- observer creation/retained metadata;
- detach/discard policy;
- `RequestCancel`;
- current thread context;
- memory synchronization edges;
- source provenance;
- target capability requirements.

§ 69(3) Concrete opcode/instruction names remain owned by `semantic_ir.md`.

§ 69(4) The literal opcode list from the legacy thread book is not normative.

§ 69(5) IR verification must reject duplicated join capability, duplicated result ownership, use of released explicit storage, and contradictory terminal state/payload combinations.

---

## § 70. Lowering

**Governance tags:** `lowering.thread-v2`, `compiler.platform-model`

§ 70(1) Lowering consumes validated thread Semantic IR plus the selected `CompilationPlan`.

§ 70(2) Lowering must preserve:

- physical-thread identity rather than task substitution;
- exact `ThreadSpawnError`/`ThreadStartError` boundaries;
- explicit Preferred/Required semantics;
- no hidden replacement of explicit storage;
- deferred-start no-user-code-before-start rule;
- exactly-one join capability;
- selectable join semantics;
- completion synchronization;
- exact terminal payload availability;
- move-only result ownership;
- detach cleanup transfer;
- cooperative cancellation identity;
- target-resolved platform metadata.

§ 70(3) Backend convenience must not add implicit lifecycle behavior.

---

## § 71. LSP hover

**Governance tags:** `tooling.thread-v2`

§ 71(1) Hover/navigation must expose the exact declarations/variants/payloads defined by this book.

§ 71(2) Hover for `ThreadConfig` must show:

```text
Name
Stack
Affinity
Priority
Start
Storage
```

with their exact types.

§ 71(3) Hover for `ThreadSetting[T]` must show `Default`, `Preferred(T)`, and `Required(T)`.

§ 71(4) Hover for `ThreadStatus` must show exactly six statuses.

§ 71(5) Hover for `Thread[T]` must show canonical CamelCase properties/methods and availability rules.

§ 71(6) Hover for `ThreadPlatform` must resolve to the selected target's exact concrete declaration.

§ 71(7) LSP must not present stale `ThreadSchedulingError` or `ThreadContextError` as portable APIs without a real canonical operation.

---

## § 72. Completion and navigation

**Governance tags:** `tooling.thread-v2`

§ 72(1) Completion on `Thread[T]` uses canonical names:

```text
ID
Name
Status
Value
Panic
Termination
Platform
Observe
Start
RequestCancel
```

§ 72(2) Completion should suppress/mark conditionally unavailable terminal payload properties when status/join facts prove they cannot be accessed.

§ 72(3) Completion in `ThreadConfig` should offer explicit `Default`, `Preferred(...)`, and `Required(...)` values rather than inserting a plain value with implicit preference semantics.

§ 72(4) Navigation for compiler-known types resolves to their canonical source/core declaration or selected platform declaration.

---

## § 73. Diagnostics

**Governance tags:** `tooling.thread-v2`, `frontend.thread-v2`

§ 73(1) Suggested diagnostics include:

```text
target profile does not support physical threads
```

```text
ThreadConfig.Priority requires ThreadSetting[ThreadPriority]; use Preferred(...), Required(...), or Default
```

```text
required thread affinity is not supported by selected target
```

```text
thread affinity preference is not supported; selected target default will be used
```

```text
ThreadStorage capacity must be compile-time-known
```

```text
thread storage workerStorage is still exclusively borrowed by worker
```

```text
cannot start thread worker because it is not in deferred Created state
```

```text
thread worker has already been joined
```

```text
thread Value is available only after successful join with Status == Completed
```

```text
thread Panic is available only after successful join with Status == Panicked
```

```text
thread Termination is available only after successful join with Status == Terminated
```

```text
detaching Thread[T] with non-void result requires explicit discard
```

```text
detached thread worker cannot retain reference to local value data
```

§ 73(2) Diagnostics should distinguish target unsupported, compiler unsupported, ownership/lifetime, configuration, and runtime typed failure.

---

## § 74. Examples

**Governance tags:** `concurrency.thread-v2`

§ 74(1) Ordinary eager thread:

```sec
let worker := try spawn thread Work()

join worker

match worker.Status {
    Completed => {
        Use(<-worker.Value)
    }

    Cancelled => {
        HandleCancellation()
    }

    Panicked => {
        HandlePanic(worker.Panic)
    }

    Terminated => {
        HandleTermination(worker.Termination)
    }

    Created | Running => {
        unreachable
    }
}
```

§ 74(2) Deferred configured thread:

```sec
let config := ThreadConfig {
    Name: Preferred("worker")
    Stack: Required(65536)
    Affinity: Preferred(CpuSet { 2, 3 })
    Priority: Preferred(ThreadPriority.High)
    Start: ThreadStartMode.Deferred
    Storage: None
}

let worker := try spawn thread <-config Work()

try worker.Start()
join worker
```

§ 74(3) Cooperative cancellation:

```sec
let worker := try spawn thread Work()

worker.RequestCancel()
join worker
```

§ 74(4) Requesting cancellation does not predetermine the final status.

---

## § 75. Restrictions

**Governance tags:** `concurrency.thread-v2`

§ 75(1) Portable Sec 0.1 threads must not:

- silently become logical tasks;
- copy `Thread[T]`, `ThreadConfig`, or `ThreadStorage`;
- implicitly wrap plain configuration values as `Preferred`;
- silently ignore `Required`;
- silently replace explicit thread storage;
- execute deferred callable code before successful `Start()`;
- expose a second join capability;
- make result payloads available before valid join/status conditions;
- implicitly detach unresolved handles;
- implicitly hard-terminate unresolved handles;
- treat status polling as completion synchronization;
- synthesize hidden observer `Wait()`/select;
- expose native thread ID as `ThreadID`;
- force target-specific native termination codes into portable integers;
- expose a universal safe hard-kill operation.

---

## § 76. Explicitly absent Sec 0.1 APIs

**Governance tags:** `concurrency.thread-v2`

§ 76(1) This rulebook does not define:

```text
ThreadOutcome[T]
await Thread[T]
ThreadObserver[T].Wait()
select ThreadObserver[T]
portable hard Thread[T].Terminate()
portable ThreadSchedulingError API
portable ThreadContextError API
real-time scheduling policy API
dynamic generic ThreadStorage[N]
implicit Preferred conversion
automatic lifecycle resolution at destruction
```

§ 76(2) Future additions require explicit normative design.

---

## § 77. Conformance scenarios

**Governance tags:** `concurrency.thread-v2`

§ 77(1) Conformance tests must include at least:

- `spawn thread Work()` raw type;
- unsupported target compile-time rejection;
- exact `ThreadSetting[T]`;
- no plain-value-to-Preferred conversion;
- exact ThreadPriority;
- exact CpuSet literal;
- non-generic ThreadStorage;
- compile-time-required storage capacity;
- no dynamic allocation for ThreadStorage construction;
- exact ThreadConfig/defaults;
- named config consumed with `<-`;
- eager versus deferred start;
- Start invalid-state handling;
- explicit-storage borrow lifetime through join;
- explicit-storage detached lifetime rejection where invalid;
- exact ThreadSpawnError variants;
- exact ThreadStartError variants;
- exact six-state ThreadStatus;
- exact ThreadTerminationKind/ThreadTermination;
- canonical CamelCase Thread surface;
- move-only Thread[T];
- Value availability/copy/move behavior;
- Panic availability;
- Termination availability;
- selectable join;
- non-selected join no-effect;
- caller cancellation before join commit preserves join capability;
- unresolved cancellation path requires lifecycle cleanup;
- observer copyability/non-ownership;
- observer no Wait/select;
- detach/discard requirements;
- RequestCancel symmetry;
- Thread.Current physical identity;
- CancelRequested is bool observation;
- Thread.Yield no synchronization;
- target-resolved ThreadPlatform;
- spawn publication and join synchronization;
- LSP exact hover/completion/navigation;
- Semantic IR one join capability/result ownership verification.

---

## § 78. Cross-rulebook synchronization

**Governance tags:** `concurrency.thread-v2`

§ 78(1) `select.md` treats `join Thread[T]` as normative selectable Sec 0.1 behavior.

§ 78(2) `cancellation.md` uses the exact ThreadStatus declaration with comments placed before variants.

§ 78(3) `cancellation.md` remains owner of general `RequestCancel`, `CancelRequested`, `cancel`, and cancellation-point commit semantics.

§ 78(4) `spawn.md` uses the new `ThreadConfig` ownership syntax and exact thread configuration surface.

§ 78(5) `thread_local.md` remains owner of thread-local declaration semantics.

§ 78(6) `panic.md` remains owner of `PanicInfo`.

§ 78(7) `platform_model.md` remains owner of how the selected platform provides the exact concrete `ThreadPlatform`.

§ 78(8) `semantic_ir.md` remains owner of concrete IR vocabulary.

§ 78(9) `language-rulebook-status.md` should record revision 2.0 synchronization after correction application.

---

## § 79. Governance

**Governance tags:** `concurrency.thread-v2`, `frontend.thread-v2`, `semantic-ir.thread-v2`, `lowering.thread-v2`, `tooling.thread-v2`, `analysis.transferability`, `analysis.borrowing`, `sema.deadlock-analysis`, `compiler.platform-model`

§ 79(1) `governance/concurrency_thread.yaml` is the sole canonical implementation-status owner for the portable physical-thread surface defined by this revision.

§ 79(2) Related select/cancellation/spawn/panic/platform work is cross-linked to its owning fragment rather than duplicated as competing integration entries.

§ 79(3) Root `implementation-status.yaml` is not a second canonical ledger.

§ 79(4) Cross-rulebook synchronization required by this revision was tracked by the accompanying threads-v2 correction.

§ 79(5) After synchronization is applied, the correction belongs under `rules/corrections/applied/`.
