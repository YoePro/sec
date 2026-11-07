# Concurrency Runtime Model

- **Status:** Normative
- **Created:** 2026-07-29
- **Last updated:** 2026-11-07 (stable chapter and paragraph numbering)
- **Document revision:** 1
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/concurrency_runtime_model.md`
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`
- **Related governance:** `governance/concurrency_model.yaml` (`concurrency.model-v2` umbrella), `governance/concurrency_task.yaml`, `governance/concurrency_thread.yaml`
- **Related rulebooks:** `rules/concurrency/concurrency.md`, `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/concurrency/await.md`, `rules/concurrency/blocking.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/scheduling.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/platform/interrupts.md`, `rules/platform/ffi.md`, `rules/compiler/semantic_ir.md`

---

## § 1 Current implementation status

**§ 1(1)** Implemented:

- compiler-known task handle type exists from earlier work;
- compiler-known thread handle and thread observer types exist;
- compiler-known thread runtime metadata/error types exist:
  `ThreadConfig`, `ThreadContext`, `ThreadID`, `ThreadPriority`,
  `ThreadStatus`, `ThreadSpawnError`, `ThreadStartError`,
  plus legacy `ThreadSchedulingError` and `ThreadTerminationError` identities
  pending removal from the portable thread surface;
- a legacy compiler-known `ThreadContextError` identity remains implementation
  debt, but is not part of the portable API; `thread_local.md` instead owns
  exact `ThreadAttachment` and `ThreadAttachError` identities;
- compiler-known `ThreadLocal[T]` key type exists;
- unresolved task and thread handles are tracked conservatively at local scope
  exit;
- spawned lambda bodies are analyzed with a current task cancellation context.

**§ 1(2)** Not implemented yet:

- runtime capability profile validation;
- general current task/thread context propagation through ordinary call graphs;
- fallible spawn result model;
- thread creation/start/join/detach runtime operations;
- wait-set/runtime registration operations;
- task suspension and physical thread parking lowering;
- TLS attach/detach and per-thread initialization;
- runtime control-block lowering;
- Semantic IR runtime operations.

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

**§ 4(2)** At minimum:

```text
task execution
physical threads
process creation
task suspension
thread parking
wait-set registration
timers
channels
mutexes
atomics
thread-local storage
interrupt integration
dynamic allocation
static task/thread storage
detached lifecycle management
panic reporting
unsafe native termination
```

**§ 4(3)** Supporting tasks does not imply physical threads.

**§ 4(4)** Supporting physical threads does not imply a task executor.

**§ 4(5)** Supporting interrupts does not imply either.

---

## § 5 Execution creation is fallible

**§ 5(1)** Creation expressions return `Result`:

```text
spawn task     Result[Task[T], TaskSpawnError]
spawn thread   Result[Thread[T], ThreadSpawnError]
spawn process  Result[Process, ProcessSpawnError]
```

**§ 5(2)** The promoted default form:

```sec
spawn Work()
```

**§ 5(3)** is task creation and therefore also fallible.

**§ 5(4)** Typical source uses `try`:

```sec
let worker := try spawn Work()
```

**§ 5(5)** or explicit matching.

**§ 5(6)** A compile-time unsupported capability is a compiler diagnostic, not a runtime
spawn error.

---

## § 6 Task terminal outcome

**§ 6(1)** An already-created task publishes exactly one canonical terminal outcome:

```sec
type TaskOutcome[T] union {
    Completed(T)
    Cancelled
    Panicked(PanicInfo)
    Failed(TaskError)
}
```

**§ 6(2)** Runtime task state preserves the complete declared return type `T` separately
from cancellation, panic, and execution failure. In particular,
`Completed(Err(E))` is normal completion data and must not be rewritten as
`Failed(TaskError)`. `TaskSpawnError` remains exclusive to failure before a task
exists; `TaskError` names execution failure after successful creation. The
complete `TaskError` variant inventory is intentionally not fixed here.

---

## § 7 Core execution control block

**§ 7(1)** The compiler may model task, thread and process owners through a shared internal
execution-control concept.

**§ 7(2)** This is not required to be one runtime struct.

**§ 7(3)** The backend may specialize or erase fields by execution kind and profile.

**§ 7(4)** A control block may conceptually contain:

- lifecycle state;
- result storage;
- panic or termination information;
- cancellation state;
- observer state;
- wait registrations;
- cleanup responsibility;
- native handle;
- generation identity.

**§ 7(5)** Unused capabilities must not impose runtime storage cost.

---

## § 8 Result storage

**§ 8(1)** `Task[T]` and `Thread[T]` own terminal result storage.

**§ 8(2)** The runtime must preserve:

- complete declared return type;
- move-only result ownership;
- repeated copyable reads after join;
- one move of a move-only result;
- separate cancellation, panic and execution failure;
- deterministic destruction of an unconsumed result.

**§ 8(3)** Result storage may be:

- inline in a control block;
- separately allocated when profile permits;
- statically reserved;
- optimized into joiner storage when ownership proof permits.

**§ 8(4)** Optimization must preserve observable lifecycle semantics.

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

**§ 9(2)** A task may migrate after suspension unless affinity rules say otherwise.

**§ 9(3)** Task identity is independent of worker thread identity.

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

**§ 10(3)** The backend must provide:

- creation;
- start;
- join;
- detach cleanup;
- cancellation request state;
- completion publication;
- native identity;
- stack policy.

**§ 10(4)** If it cannot, `spawn thread` is unsupported.

---

## § 11 Static embedded storage

**§ 11(1)** A profile may provide explicit static storage types such as:

```sec
ThreadStorage(capacity)
TaskStorage[StateSize]
```

**§ 11(2)** `ThreadStorage` is non-generic and `@noCopy`; its constructor capacity is
compile-time-required. Explicit backing storage must not be replaced by hidden
allocation.

**§ 11(3)** The compiler should determine:

- storage size;
- alignment;
- lifetime;
- exclusive use;
- detached ownership;
- re-use point;
- target compatibility.

**§ 11(4)** Static storage enables runtime-free or minimal-runtime concurrency.

---

## § 12 Deferred activation

**§ 12(1)** Deferred thread creation requires:

- control block initialization;
- argument ownership transfer;
- publication of initial state;
- user callable suppression until `Start()`;
- fallible start reporting;
- correct cleanup if never started.

**§ 12(2)** A profile may implement this through:

- native suspended thread creation;
- RTOS suspended task;
- static inactive thread slot;
- internal start gate.

**§ 12(3)** No user code may execute early.

---

## § 13 Task suspension

**§ 13(1)** A task-aware runtime should suspend logical tasks at waits.

**§ 13(2)** It may register readiness with:

- channel state;
- thread completion;
- process completion;
- timer;
- mutex;
- I/O;
- cancellation;
- `select`.

**§ 13(3)** The runtime must unregister losing operations without committing them.

**§ 13(4)** `Task.Yield()` is a logical task scheduling and current-task cancellation point;
it keeps the task runnable after normal commit and must not be reduced to a
physical-thread yield when that would lose logical semantics. Runtime machinery
used internally for ordinary `main` does not create a source-visible root
`TaskContext`.

---

## § 14 Physical thread blocking

**§ 14(1)** Physical thread waits use native park, wait or block operations.

**§ 14(2)** `Thread.Yield()` is only a physical scheduler hint. It is not a task
suspension, cancellation point, or memory-synchronization operation.

**§ 14(3)** A task runtime may use a native blocking adapter only when:

- the profile permits worker blocking;
- executor deadlock analysis is satisfied;
- cancellation behavior is defined;
- source commit semantics are preserved.

---

## § 15 Select runtime

**§ 15(1)** `select` requires a wait-set or equivalent registration protocol.

**§ 15(2)** Each candidate must support conceptual operations:

```text
prepare
check-ready
register
commit
cancel-registration
```

**§ 15(3)** Exactly one branch commits.

**§ 15(4)** Source-order priority determines which ready branch wins.

**§ 15(5)** The runtime must not consume values from losing branches.

**§ 15(6)** A no-runtime target may lower a fixed select to:

- static polling with bounded semantics;
- interrupt-driven flags;
- compiler-generated state machine;
- direct RTOS wait group.

---

## § 16 Channels

**§ 16(1)** Channels may use specialized backends:

```text
task-local
executor-shared
thread-shared
hybrid task/thread
ISR-safe fixed queue
```

**§ 16(2)** The compiler selects the least expensive backend that satisfies all observed
capabilities.

**§ 16(3)** Cross-thread use requires thread-safe synchronization.

**§ 16(4)** Unused revocation, expiration, priority or statistics capabilities should have
zero metadata cost where possible.

---

## § 17 Mutexes and atomics

**§ 17(1)** `Mutex[T]` is compiler-known but may map to:

- task-aware mutex;
- native mutex;
- RTOS mutex;
- spin-based primitive when profile permits;
- interrupt-masking critical section only when explicitly defined as equivalent
  for that target and usage.

**§ 17(2)** Atomics map to target-supported widths and memory orders.

**§ 17(3)** Unsupported widths must be rejected or lowered through an explicitly approved
lock backend.

---

## § 18 Thread-local storage

**§ 18(1)** Thread-local storage may map to:

- static compiler-assigned TLS offsets;
- ELF or platform TLS;
- Windows TLS/FLS;
- RTOS thread slots;
- fields in a Sec thread control block.

**§ 18(2)** Access must preserve one value per physical thread.

**§ 18(3)** A task migration must not move thread-local values with the task.

**§ 18(4)** Foreign threads must have an attached Sec `ThreadContext` before using
compiler-managed TLS.

---

## § 19 Detached lifecycle manager

**§ 19(1)** Detaching transfers lifecycle cleanup to a program or target manager.

**§ 19(2)** The manager must handle:

- cancellation request at normal shutdown;
- wake-up of cancellation-aware waits;
- result discard;
- terminal cleanup;
- native handle release;
- thread-local destruction where guaranteed.

**§ 19(3)** A no-runtime profile may reject detach or require static detached slots.

**§ 19(4)** Detach must not silently leak lifecycle resources.

---

## § 20 Panic and abnormal termination

**§ 20(1)** The runtime records panic separately from normal result and cancellation.

**§ 20(2)** Unsafe native termination records `Terminated`.

**§ 20(3)** The runtime must not fabricate a normal `T` after either state.

**§ 20(4)** A minimal target may store compact status information but must preserve the
semantic distinction.

---

## § 21 Observers

**§ 21(1)** Observers require retained completion/status state.

**§ 21(2)** The backend may use:

- generation-indexed static tables;
- reference-counted control blocks;
- program-lifetime observer tables;
- compile-time bounded observer slots.

**§ 21(3)** Observer support must not retain join-only native resources.

**§ 21(4)** A profile may restrict observer count when storage is bounded.

---

## § 22 Allocation policy

**§ 22(1)** No concurrency operation may hide dynamic allocation when the selected profile
forbids it.

**§ 22(2)** Allocation-capable profiles must report allocation failure through the owning
operation's `Result`.

**§ 22(3)** Explicit arena or static storage follows allocation and lifetime rules.

**§ 22(4)** The compiler should report the allocation chain leading from source operation to
allocator.

---

## § 23 Foreign thread attachment

**§ 23(1)** A foreign callback may enter Sec on a thread not created by Sec.

**§ 23(2)** The platform adapter must establish a `ThreadContext` before code uses:

- `Thread.Current()`;
- thread-local storage;
- cancellation observation;
- Sec panic tracking;
- runtime-managed blocking integration.

**§ 23(3)** Attachment may be automatic in generated FFI wrappers when safe. Such a wrapper
destroys only an attachment it created and reuses an already-active attachment
for nested callbacks.

**§ 23(4)** Manual attachment uses:

```sec
Thread.AttachCurrent() Result[ThreadAttachment, ThreadAttachError]
```

**§ 23(5)** Main and Sec-created threads are already attached. User TLS factories remain
lazy and do not run merely because an attachment is established.

---

## § 24 ISR integration

**§ 24(1)** The runtime model must identify ISR-safe operations separately.

**§ 24(2)** ISR code must not depend on:

- task suspension;
- ordinary thread parking;
- heap allocation;
- unbounded observer lists;
- ordinary mutexes;
- thread-local initialization.

**§ 24(3)** ISR-to-runtime wake-up may use:

- atomics;
- fixed queues;
- event flags;
- RTOS ISR APIs;
- deferred interrupt work.

---

## § 25 Target profile validation

**§ 25(1)** For every concurrency operation the compiler must determine:

- required capability;
- preferred capabilities;
- storage requirements;
- allocation behavior;
- blocking behavior;
- cancellation behavior;
- ISR legality;
- native lowering;
- cleanup path.

**§ 25(2)** Preferred unsupported options produce stable warnings.

**§ 25(3)** Required unsupported options produce compile-time errors.

---

## § 26 Core runtime error types

**§ 26(1)** All language-level concurrency runtime errors must be declared in:

```text
core/errors.sec
```

**§ 26(2)** At minimum:

```sec
enum TaskSpawnError error {
    OutOfMemory
    ResourceLimit
    ExecutorUnavailable
    InvalidConfiguration
    NativeFailure
}

enum ThreadSpawnError error {
    OutOfMemory
    ResourceLimit
    StackAllocationFailed
    InvalidConfiguration
    PermissionDenied
    // Required TLS runtime/target infrastructure failed before callable
    // execution commit; this is not lazy user-factory failure.
    ThreadLocalInitializationFailed
    NativeFailure
}

enum ProcessSpawnError error {
    OutOfMemory
    ResourceLimit
    ExecutableNotFound
    PermissionDenied
    InvalidConfiguration
    NativeFailure
}

enum ThreadStartError error {
    InvalidState
    ResourceUnavailable
    PermissionDenied
    NativeFailure
}

enum ThreadAttachError error {
    AlreadyAttached
    ResourceUnavailable
    NativeFailure
}
```

**§ 26(3)** The core surface must additionally provide the generic `TaskOutcome[T]` shape
above and the exact closed Sec error enum `TaskError` with variants
`OutOfMemory`, `ResourceLimit`, `ExecutorUnavailable`, and `NativeFailure`.
`InvalidConfiguration` remains a task-creation error under `TaskSpawnError`.

**§ 26(4)** If task or process rulebooks define additional execution failure types, they
must also be placed in `core/errors.sec` using canonical Sec error-type syntax.

**§ 26(5)** Compiler diagnostics are not runtime error values.

---

## § 27 Semantic IR and lowering

**§ 27(1)** Semantic IR records source semantics before runtime selection.

**§ 27(2)** Runtime lowering may then choose:

- static;
- native;
- executor;
- RTOS;
- state-machine;
- direct syscall;
- library-call implementation.

**§ 27(3)** The lowering must not infer ownership, cancellation or result semantics from
low-level representation.

**§ 27(4)** Runtime capability decisions must remain inspectable in compiler diagnostics and
analysis output.

---

## § 28 Diagnostics

**§ 28(1)** Examples:

```text
target profile does not provide task execution
```

```text
target profile does not support physical threads
```

```text
explicit ThreadStorage requires 65536 bytes but target provides 32768
```

```text
operation requires hidden allocation but target profile forbids allocation
```

```text
foreign thread must be attached before accessing ThreadLocal[State]
```

```text
detached execution is not supported by runtime-free target profile
```

**§ 28(2)** Diagnostics must use stable IDs.

---

## § 29 Required synchronization

**§ 29(1)** Cross-check and update:

```text
spawn.md
tasks.md
threads.md
processes.md
await.md
scheduling.md
blocking.md
transferability.md
cancellation.md
channels.md
select.md
mutex.md
atomics.md
thread_local.md
concurrency.md
concurrency_memory_model.md
allocation.md
static.md
platform/ffi.md
compiler.md
compiler_analysis.md
compiler_pipeline.md
semantic_ir.md
mlir.txt
core-library.md
core/errors.sec
```
