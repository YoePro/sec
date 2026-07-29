# Concurrency Runtime Model

## Current implementation status

Implemented:

- compiler-known task handle type exists from earlier work;
- compiler-known thread handle and thread observer types exist;
- compiler-known thread runtime metadata/error types exist:
  `ThreadConfig`, `ThreadContext`, `ThreadID`, `ThreadPriority`,
  `ThreadStatus`, `ThreadSpawnError`, `ThreadStartError`,
  `ThreadSchedulingError`, `ThreadTerminationError` and
  `ThreadContextError`;
- compiler-known `ThreadLocal[T]` key type exists;
- unresolved task and thread handles are tracked conservatively at local scope
  exit;
- spawned lambda bodies are analyzed with a current task cancellation context.

Not implemented yet:

- runtime capability profile validation;
- general current task/thread context propagation through ordinary call graphs;
- fallible spawn result model;
- thread creation/start/join/detach runtime operations;
- wait-set/runtime registration operations;
- task suspension and physical thread parking lowering;
- TLS attach/detach and per-thread initialization;
- runtime control-block lowering;
- Semantic IR runtime operations.

## Purpose

The concurrency runtime model defines which runtime services a target profile may
provide and how compiler-generated code uses them.

Sec does not require one universal runtime.

A program may target:

- hosted operating systems;
- RTOS environments;
- bare metal;
- statically scheduled embedded systems;
- environments with tasks but no physical threads;
- environments with physical threads but no task executor;
- single-threaded systems with interrupts.

Source-level semantics must remain stable across runtime models.

---

## No-required-runtime principle

A language feature must not automatically imply a heavyweight managed runtime.

The compiler may use:

- direct native system calls;
- native OS threads;
- RTOS primitives;
- compiler-generated state machines;
- static control blocks;
- fixed-capacity queues;
- direct atomics;
- target-specific wait sets;
- optional hosted scheduler support.

A target profile must declare the services it supplies.

Unsupported semantics must be rejected, not silently changed.

---

## Independent capabilities

A profile declares capabilities independently.

At minimum:

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

Supporting tasks does not imply physical threads.

Supporting physical threads does not imply a task executor.

Supporting interrupts does not imply either.

---

## Execution creation is fallible

Creation expressions return `Result`:

```text
spawn task     Result[Task[T], TaskSpawnError]
spawn thread   Result[Thread[T], ThreadSpawnError]
spawn process  Result[Process, ProcessSpawnError]
```

The promoted default form:

```sec
spawn Work()
```

is task creation and therefore also fallible.

Typical source uses `try`:

```sec
let worker := try spawn Work()
```

or explicit matching.

A compile-time unsupported capability is a compiler diagnostic, not a runtime
spawn error.

---

## Core execution control block

The compiler may model task, thread and process owners through a shared internal
execution-control concept.

This is not required to be one runtime struct.

The backend may specialize or erase fields by execution kind and profile.

A control block may conceptually contain:

- lifecycle state;
- result storage;
- panic or termination information;
- cancellation state;
- observer state;
- wait registrations;
- cleanup responsibility;
- native handle;
- generation identity.

Unused capabilities must not impose runtime storage cost.

---

## Result storage

`Task[T]` and `Thread[T]` own terminal result storage.

The runtime must preserve:

- complete declared return type;
- move-only result ownership;
- repeated copyable reads after join;
- one move of a move-only result;
- separate cancellation, panic and execution failure;
- deterministic destruction of an unconsumed result.

Result storage may be:

- inline in a control block;
- separately allocated when profile permits;
- statically reserved;
- optimized into joiner storage when ownership proof permits.

Optimization must preserve observable lifecycle semantics.

---

## Task runtime models

Valid task implementations include:

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

A task may migrate after suspension unless affinity rules say otherwise.

Task identity is independent of worker thread identity.

---

## Thread runtime models

A physical thread maps to a target-native physical execution abstraction.

Valid implementations include:

```text
POSIX thread
Windows thread
RTOS native task/thread
declared hardware-thread abstraction
```

The backend must provide:

- creation;
- start;
- join;
- detach cleanup;
- cancellation request state;
- completion publication;
- native identity;
- stack policy.

If it cannot, `spawn thread` is unsupported.

---

## Static embedded storage

A profile may provide static storage types such as:

```sec
ThreadStorage[StackSize]
TaskStorage[StateSize]
```

Explicit backing storage must not be replaced by hidden allocation.

The compiler should determine:

- storage size;
- alignment;
- lifetime;
- exclusive use;
- detached ownership;
- re-use point;
- target compatibility.

Static storage enables runtime-free or minimal-runtime concurrency.

---

## Deferred activation

Deferred thread creation requires:

- control block initialization;
- argument ownership transfer;
- publication of initial state;
- user callable suppression until `Start()`;
- fallible start reporting;
- correct cleanup if never started.

A profile may implement this through:

- native suspended thread creation;
- RTOS suspended task;
- static inactive thread slot;
- internal start gate.

No user code may execute early.

---

## Task suspension

A task-aware runtime should suspend logical tasks at waits.

It may register readiness with:

- channel state;
- thread completion;
- process completion;
- timer;
- mutex;
- I/O;
- cancellation;
- `select`.

The runtime must unregister losing operations without committing them.

---

## Physical thread blocking

Physical thread waits use native park, wait or block operations.

A task runtime may use a native blocking adapter only when:

- the profile permits worker blocking;
- executor deadlock analysis is satisfied;
- cancellation behavior is defined;
- source commit semantics are preserved.

---

## Select runtime

`select` requires a wait-set or equivalent registration protocol.

Each candidate must support conceptual operations:

```text
prepare
check-ready
register
commit
cancel-registration
```

Exactly one branch commits.

Source-order priority determines which ready branch wins.

The runtime must not consume values from losing branches.

A no-runtime target may lower a fixed select to:

- static polling with bounded semantics;
- interrupt-driven flags;
- compiler-generated state machine;
- direct RTOS wait group.

---

## Channels

Channels may use specialized backends:

```text
task-local
executor-shared
thread-shared
hybrid task/thread
ISR-safe fixed queue
```

The compiler selects the least expensive backend that satisfies all observed
capabilities.

Cross-thread use requires thread-safe synchronization.

Unused revocation, expiration, priority or statistics capabilities should have
zero metadata cost where possible.

---

## Mutexes and atomics

`Mutex[T]` is compiler-known but may map to:

- task-aware mutex;
- native mutex;
- RTOS mutex;
- spin-based primitive when profile permits;
- interrupt-masking critical section only when explicitly defined as equivalent
  for that target and usage.

Atomics map to target-supported widths and memory orders.

Unsupported widths must be rejected or lowered through an explicitly approved
lock backend.

---

## Thread-local storage

Thread-local storage may map to:

- static compiler-assigned TLS offsets;
- ELF or platform TLS;
- Windows TLS/FLS;
- RTOS thread slots;
- fields in a Sec thread control block.

Access must preserve one value per physical thread.

A task migration must not move thread-local values with the task.

Foreign threads must have an attached Sec `ThreadContext` before using
compiler-managed TLS.

---

## Detached lifecycle manager

Detaching transfers lifecycle cleanup to a program or target manager.

The manager must handle:

- cancellation request at normal shutdown;
- wake-up of cancellation-aware waits;
- result discard;
- terminal cleanup;
- native handle release;
- thread-local destruction where guaranteed.

A no-runtime profile may reject detach or require static detached slots.

Detach must not silently leak lifecycle resources.

---

## Panic and abnormal termination

The runtime records panic separately from normal result and cancellation.

Unsafe native termination records `Terminated`.

The runtime must not fabricate a normal `T` after either state.

A minimal target may store compact status information but must preserve the
semantic distinction.

---

## Observers

Observers require retained completion/status state.

The backend may use:

- generation-indexed static tables;
- reference-counted control blocks;
- program-lifetime observer tables;
- compile-time bounded observer slots.

Observer support must not retain join-only native resources.

A profile may restrict observer count when storage is bounded.

---

## Allocation policy

No concurrency operation may hide dynamic allocation when the selected profile
forbids it.

Allocation-capable profiles must report allocation failure through the owning
operation's `Result`.

Explicit arena or static storage follows allocation and lifetime rules.

The compiler should report the allocation chain leading from source operation to
allocator.

---

## Foreign thread attachment

A foreign callback may enter Sec on a thread not created by Sec.

The platform adapter must establish a `ThreadContext` before code uses:

- `Thread.Current()`;
- thread-local storage;
- cancellation observation;
- Sec panic tracking;
- runtime-managed blocking integration.

Attachment may be automatic in generated FFI wrappers when safe.

Manual attachment, if exposed, is fallible and uses `ThreadContextError`.

---

## ISR integration

The runtime model must identify ISR-safe operations separately.

ISR code must not depend on:

- task suspension;
- ordinary thread parking;
- heap allocation;
- unbounded observer lists;
- ordinary mutexes;
- thread-local initialization.

ISR-to-runtime wake-up may use:

- atomics;
- fixed queues;
- event flags;
- RTOS ISR APIs;
- deferred interrupt work.

---

## Target profile validation

For every concurrency operation the compiler must determine:

- required capability;
- preferred capabilities;
- storage requirements;
- allocation behavior;
- blocking behavior;
- cancellation behavior;
- ISR legality;
- native lowering;
- cleanup path.

Preferred unsupported options produce stable warnings.

Required unsupported options produce compile-time errors.

---

## Core runtime error types

All language-level concurrency runtime errors must be declared in:

```text
core/errors.sec
```

At minimum:

```sec
enum TaskSpawnError {
    OutOfMemory
    ResourceLimit
    ExecutorUnavailable
    InvalidConfiguration
    NativeFailure
}

enum ThreadSpawnError {
    OutOfMemory
    ResourceLimit
    StackAllocationFailed
    InvalidConfiguration
    PermissionDenied
    ThreadLocalInitializationFailed
    NativeFailure
}

enum ProcessSpawnError {
    OutOfMemory
    ResourceLimit
    ExecutableNotFound
    PermissionDenied
    InvalidConfiguration
    NativeFailure
}

enum ThreadStartError {
    InvalidState
    ResourceUnavailable
    PermissionDenied
    NativeFailure
}

enum ThreadSchedulingError {
    Unsupported
    InvalidValue
    PermissionDenied
    NativeFailure
}

enum ThreadTerminationError {
    Unsupported
    PermissionDenied
    InvalidState
    NativeFailure
}

enum ThreadContextError {
    NotAttached
    AlreadyAttached
    ResourceUnavailable
    NativeFailure
}
```

If task or process rulebooks define additional execution failure types, they must
also be placed in `core/errors.sec`.

Compiler diagnostics are not runtime error values.

---

## Semantic IR and lowering

Semantic IR records source semantics before runtime selection.

Runtime lowering may then choose:

- static;
- native;
- executor;
- RTOS;
- state-machine;
- direct syscall;
- library-call implementation.

The lowering must not infer ownership, cancellation or result semantics from
low-level representation.

Runtime capability decisions must remain inspectable in compiler diagnostics and
analysis output.

---

## Diagnostics

Examples:

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

Diagnostics must use stable IDs.

---

## Required synchronization

Cross-check and update:

```text
spawn.md
tasks.txt
threads.md
processes.txt
await.md
scheduling.md
blocking.md
transferability.md
cancellation.md
structured_concurrency.md
channels.md
select.md
mutex.md
atomics.md
thread_local.md
concurrency.md
concurrency_memory_model.md
allocation.txt
static.md
ffi.txt
compiler.txt
compiler_analysis.txt
compiler_pipeline.txt
semantic_ir.txt
mlir.txt
rules_implementations.txt
core-library.md
core/errors.sec
```
