# Correction — Threads v2 cross-rulebook synchronization

- **Status:** Applied
- **Created:** 2026-09-24
- **Last updated:** 2026-09-24
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `main-reviewed-2026-09-24`
- **Primary owning rulebook:** `rules/concurrency/threads.md`
- **Implementation governance:** `governance/concurrency_thread.yaml`
- **Classification:** Normative synchronization of physical-thread API, configuration, lifecycle, and select/cancellation semantics

---

## 1. Replace legacy thread surface

Synchronize all compiler-known/core declarations, frontend member lookup, LSP, tests, docs, Semantic IR, lowering, and runtime code to revision 2.0 of `rules/concurrency/threads.md`.

Portable `Thread[T]` remains:

```sec
@noCopy
type Thread[T]
```

The public portable surface is exactly:

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

Use CamelCase.

Remove legacy lowercase source-facing spellings:

```text
id
name
status
value
panic
termination
platform
```

as canonical API.

---

## 2. Freeze `ThreadSetting[T]`

Add the exact source-visible/compiler-known declaration:

```sec
type ThreadSetting[T] union {
    // Use the selected target/profile default.
    Default

    // Request this value as a preference.
    Preferred(T)

    // Require this value as part of program semantics.
    Required(T)
}
```

Do not provide a thread-config-only implicit conversion:

```text
T -> Preferred(T)
```

Invalid legacy shorthand:

```sec
Priority: ThreadPriority.High
```

Canonical:

```sec
Priority: Preferred(ThreadPriority.High)
```

or:

```sec
Priority: Required(ThreadPriority.High)
```

or:

```sec
Priority: Default
```

---

## 3. Freeze `ThreadStartMode`

Use exactly:

```sec
enum ThreadStartMode {
    // Callable may begin once creation commits.
    Eager

    // Callable must not execute before Start() commits.
    Deferred
}
```

`Eager` is the default.

`Deferred` is a semantic requirement and may not be ignored.

---

## 4. Freeze `ThreadPriority`

Use exactly:

```sec
enum ThreadPriority {
    Lowest
    Low
    Normal
    High
    Highest
}
```

Treat these as portable relative intents.

Do not define them as native scheduler numeric values.

Do not claim `Highest` is a real-time scheduling guarantee.

---

## 5. Add canonical `CpuSet`

Add compiler-known:

```sec
type CpuSet
```

with canonical literal form:

```sec
CpuSet { 2, 3 }
```

`CpuSet` is immutable and is not an alias of:

```sec
set[uint]
```

Validate CPU indices against the selected CompilationPlan where statically possible.

---

## 6. Replace `ThreadStorage[N]`

Remove the legacy conceptual/value-generic form:

```sec
ThreadStorage[64KiB]()
```

Use non-generic:

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

The constructor capacity is compile-time-required.

Construction reserves target-aligned compiler/platform-managed backing storage.

It does not dynamically allocate.

---

## 7. Freeze `ThreadConfig`

Use exactly:

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

A named config is consumed by thread spawn:

```sec
let worker := try spawn thread <-config Work()
```

Fresh inline config requires no synthetic move marker.

---

## 8. Inline configuration

Update contextual thread configuration examples to canonical field names and explicit `ThreadSetting` wrappers.

Example:

```sec
let worker := try spawn thread {
    Name: Preferred("worker")
    Stack: Required(65536)
    Affinity: Preferred(CpuSet { 2, 3 })
    Priority: Preferred(ThreadPriority.High)
    Start: ThreadStartMode.Deferred
} Work()
```

Omitted inline fields use the exact `ThreadConfig.init()` defaults.

Do not preserve the old plain-value-is-preferred magic.

---

## 9. Explicit storage ownership

On successful spawn with:

```sec
Storage: Some(ref mut storage)
```

the exclusive mutable borrow of `storage` remains active while the thread/runtime may use the backing storage.

Joined thread:
- release the storage borrow only after join-owned native cleanup is complete.

Detached thread:
- runtime may retain the storage borrow through detached terminal cleanup;
- storage lifetime must outlive that cleanup;
- invalid local/scope storage detach is rejected.

Spawn failure before creation commit releases config-held storage borrow during normal failure cleanup.

Never replace explicit storage with hidden heap allocation.

---

## 10. Freeze `ThreadSpawnError`

Use exactly:

```sec
enum ThreadSpawnError error {
    // Memory required for thread/control representation could not be obtained.
    OutOfMemory

    // Native/runtime thread resource limit was reached.
    ResourceLimit

    // Runtime-provided stack/backing storage could not be established.
    StackAllocationFailed

    // Runtime-derived configuration could not be instantiated.
    InvalidConfiguration

    // Native environment denied creation.
    PermissionDenied

    // Required thread-local initialization failed before callable execution.
    ThreadLocalInitializationFailed

    // Unclassified target/runtime creation failure.
    NativeFailure
}
```

Unsupported physical-thread target remains a compile-time diagnostic.

Statically invalid required configuration remains a compile-time diagnostic.

---

## 11. Freeze `ThreadStartError`

Use exactly:

```sec
enum ThreadStartError error {
    // Thread cannot be started from its current state.
    InvalidState

    // Native/runtime resource needed for start is unavailable.
    ResourceUnavailable

    // Native environment denied start.
    PermissionDenied

    // Unclassified target/runtime start failure.
    NativeFailure
}
```

Comments remain before the variants they document.

---

## 12. Remove stale unused portable error identities

The legacy implementation/status text lists:

```text
ThreadSchedulingError
ThreadContextError
```

as compiler-known types, but no exact portable Sec 0.1 public operation owns them.

Do not retain them as public/core API merely because the internal identities already exist.

Remove or internalize them unless another canonical rulebook introduces an exact public operation and takes ownership.

This correction does not forbid target-specific scheduling/context errors in a fully specified platform API.

---

## 13. Freeze `ThreadStatus`

Use exactly:

```sec
enum ThreadStatus {
    // Deferred thread exists but callable has not started.
    Created

    // Callable is live.
    Running

    // Callable returned normally.
    Completed

    // Cooperative cancellation committed terminally.
    Cancelled

    // Contained Sec panic ended the callable.
    Panicked

    // Unsafe/platform-level abnormal termination prevented normal completion.
    Terminated
}
```

Update `cancellation.md` examples so documentation comments appear before these variants.

---

## 14. Add exact termination metadata

Add:

```sec
enum ThreadTerminationKind {
    Requested
    External
    Fault
    Unknown
}

type ThreadTermination struct {
    Kind: ThreadTerminationKind
}
```

In the actual documented declaration, place comments before every enum variant/member.

Do not add a universal signal number or native exception/status integer.

Target-native metadata remains platform-specific.

---

## 15. Terminal properties

After successful join:

```text
Status == Completed
    Value available

Status == Cancelled
    no Value/Panic/Termination

Status == Panicked
    Panic available

Status == Terminated
    Termination available
```

`Value` has exact `T`.

`Panic` has exact canonical `PanicInfo`.

`Termination` has exact `ThreadTermination`.

For move-only `T`, extracting Value moves once from the result slot.

---

## 16. Join and select

Synchronize `threads.md`, `select.md`, Sema, IR, runtime and tests so:

```sec
join Thread[T]
```

is selectable in Sec 0.1.

Selected join:
- waits/commits only when terminal;
- consumes one-shot join capability;
- establishes thread completion synchronization;
- releases join-owned native resources;
- releases explicit-storage borrow when cleanup permits;
- preserves the Thread[T] handle and terminal payloads.

Non-selected join:
- consumes no join capability;
- releases no native resource;
- unlocks no terminal payload;
- creates no completion synchronization edge.

---

## 17. Join cancellation

A waiting thread join may be a current-execution cancellation point.

If join commit wins:
- join effects are committed.

If caller cancellation wins first:
- join does not commit;
- join capability remains owned;
- no lifecycle/resource cleanup is implied by join;
- surrounding cancellation cleanup still owns the unresolved Thread[T].

Do not choose automatically between:
- detach;
- hard terminate;
- join-to-completion.

Lifecycle analysis must require a valid cleanup path.

---

## 18. Thread observer

Freeze:

```sec
type ThreadObserver[T]
```

as copyable, non-owning metadata observation.

Exact public surface:

```text
property ID: ThreadID
property Name: string
property Status: ThreadStatus
property Platform: ThreadPlatform
```

Creating it:

```sec
fn Observe() ThreadObserver[T]
```

is infallible and non-consuming.

Observer has no:
- Value;
- Panic;
- Termination;
- Start;
- RequestCancel;
- join/detach capability;
- hard termination authority.

---

## 19. No hidden observer Wait/select

Remove legacy wording that says a `ThreadObserver` participates in completion select without an exact operation.

Sec 0.1 defines no:

```sec
ThreadObserver[T].Wait()
```

and no primitive select on `ThreadObserver[T]`.

Do not synthesize a hidden compiler operation.

This aligns with select-v2's exact-operation requirement.

---

## 20. Cooperative cancellation

Keep:

```sec
impl Thread[T] {
    fn RequestCancel() void
}
```

Semantics:
- cooperative;
- idempotent;
- non-consuming;
- no-op after terminal state;
- does not resolve lifecycle ownership;
- distinct from hard termination.

Current thread observation remains:

```sec
Thread.Current().CancelRequested
```

and is an ordinary `bool` property, not selectable.

---

## 21. ThreadContext

Keep compiler-known:

```sec
type ThreadContext
```

with exact public portable surface:

```text
property ID: ThreadID
property Name: string
property CancelRequested: bool
property Platform: ThreadPlatform
```

Keep:

```sec
impl Thread {
    static fn Current() ThreadContext
    static fn Yield() void
}
```

`Thread.Current()` always means the physical/native execution identity, including when a logical task is running on that physical thread.

---

## 22. ThreadPlatform

Use the same target-resolved pattern already used for process platform views.

Portable identity:

```sec
type ThreadPlatform
```

Every thread-capable target must provide a complete concrete declaration.

It must expose at least native identity metadata:

```text
property ID: <target-native thread identity type>
```

The selected platform replaces that specification placeholder with one real Sec type.

Do not coerce native identity to `uint64`.

Do not expose raw owning/mutating native handles through the portable common view.

---

## 23. Remove universal platform Terminate assumption

Remove the underspecified portable example:

```sec
worker.platform.Terminate()
```

as a universal thread API.

Safe portable Sec 0.1 has no general hard-kill method.

A target may define an unsafe target-specific termination API only if it fully defines:
- receiver/capability;
- exact errors;
- lifecycle effect;
- resulting status;
- cleanup/join requirements;
- storage interaction.

Portable observation of abnormal termination remains:

```text
ThreadStatus.Terminated
ThreadTermination
```

---

## 24. Thread.Yield

Keep:

```sec
Thread.Yield()
```

as a physical native scheduling hint.

It creates no memory synchronization edge.

It guarantees no fairness/context switch/other-thread execution.

It is invalid in ISR context.

---

## 25. Memory model

Preserve:

```text
successful thread creation
    publishes transferred/valid borrowed inputs to the new physical thread

successful join
    establishes completion synchronization
```

Status polling alone is not completion synchronization.

Detach does not establish a completion synchronization edge.

RequestCancel does not establish completion synchronization for unrelated application memory.

---

## 26. Semantic IR

Replace reliance on the legacy literal opcode list with semantic facts.

Preserve at least:
- physical thread execution kind;
- T/callable/arguments/captures;
- exact ThreadConfig/ThreadSetting classification;
- target-resolved config decisions;
- explicit storage identity/capacity/borrow;
- creation transaction;
- ThreadID;
- start state;
- join capability;
- selectable join readiness/commit;
- join cancellation race;
- result/panic/termination availability/ownership;
- observer metadata retention;
- detach/discard;
- RequestCancel;
- ThreadContext;
- memory edges;
- target requirements/source provenance.

Concrete IR vocabulary remains owned by `semantic_ir.md`.

---

## 27. LSP/tooling

Hover/completion/navigation must use exact CamelCase portable thread surface.

Show exact:
- ThreadSetting[T];
- ThreadPriority;
- ThreadStartMode;
- ThreadStatus;
- ThreadTerminationKind;
- ThreadTermination;
- ThreadSpawnError;
- ThreadStartError;
- ThreadConfig;
- ThreadStorage;
- CpuSet.

For `ThreadPlatform`, navigate to the selected target's concrete declaration.

Do not show stale ThreadSchedulingError/ThreadContextError as portable APIs without an owner.

Do not offer ThreadObserver.Wait/select.

---

## 28. Governance

Populate:

```text
governance/concurrency_thread.yaml
```

with:

```text
concurrency.thread-v2
```

Do not create `implementation-status-threads.yaml`.

Do not append duplicate status to root `implementation-status.yaml`.

Cross-link:
- select governance for generic selection mechanics;
- cancellation governance for cooperative cancellation;
- panic governance for PanicInfo;
- platform governance for concrete ThreadPlatform declarations.

---

## 29. language-rulebook-status.md

After applying the correction, mark `concurrency/threads.md` synchronized at revision 2.0.

Suggested summary:

```text
Revision 2.0 defines exact thread configuration/storage, physical lifecycle,
terminal properties, selectable join, observer/current-thread boundaries,
cooperative cancellation, and target-resolved platform metadata.
```

---

## 30. Required tests

Add/update tests for:

- exact raw spawn result;
- target unsupported compile-time failure;
- explicit ThreadSetting wrappers;
- no plain-value preference coercion;
- exact ThreadPriority/StartMode;
- CpuSet identity/literal;
- non-generic allocation-free ThreadStorage;
- compile-time capacity;
- ThreadConfig noCopy/defaults;
- config move consumption;
- explicit storage borrow lifetime;
- eager/deferred start;
- Start errors;
- exact ThreadSpawnError/ThreadStartError;
- exact ThreadStatus;
- exact ThreadTermination;
- CamelCase public surface;
- Value/Panic/Termination availability;
- move-only result once;
- join one-shot/preserved handle;
- selectable join;
- join cancellation preserves lifecycle owner;
- observer copy/non-owner/no Wait;
- detach borrow/result-discard;
- RequestCancel;
- Thread.Current/CancelRequested;
- Thread.Yield;
- target ThreadPlatform;
- spawn/join memory synchronization;
- IR ownership/lifecycle verification;
- LSP exact declarations.
