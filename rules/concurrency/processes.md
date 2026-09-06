# Sec Processes

- **Status:** Normative
- **Created:** 2026-09-06
- **Last updated:** 2026-09-06
- **Document revision:** 2.0
- **Language version:** Sec 0.1
- **Canonical path:** `rules/concurrency/processes.md`
- **Replaces:** `rules/concurrency/processes.txt`

## 1. Purpose

This rulebook defines process execution in Sec.

A Sec process is an independently scheduled execution entity in a distinct process
isolation domain. On hosted targets this is normally an operating-system process.
A target-equivalent execution entity may implement Sec process semantics only when
it provides the isolation, transfer, lifecycle, completion, and cleanup guarantees
defined by this rulebook.

Processes are distinct from:

- logical `Task[T]` execution;
- physical `Thread[T]` execution in a shared address space;
- interrupt execution;
- scheduler entities that do not provide a process isolation boundary.

Sec defines two process-creation families:

```sec
let worker := try spawn process Work(message)
```

for a Sec callable executed in a new process, and:

```sec
let command := new Command("git", ["status"])
try command.Start()
```

for an external executable.

The two creation families have different creation APIs and different normal result
types, but share the same process identity, lifecycle, observation, join, detach,
termination, reaping, and platform model where their meanings are the same.

This rulebook also defines:

- `Process[T]`;
- `Command`;
- process identity and status;
- normal process completion;
- process panic and abnormal termination;
- fallible process creation;
- process-callable materialization;
- process startup transfer and completion transfer;
- process observers;
- process standard-I/O configuration;
- join, detach, hard termination, and native cleanup;
- process memory-order and synchronization boundaries;
- target capability requirements;
- semantic-analysis and Semantic IR obligations.

This rulebook does not redefine the generic process-transferability rules. Those
are owned by `rules/memory/transferability.md`.

This rulebook does not define general inter-process messaging, shared-memory APIs,
or general native-handle transfer. Those are owned by `rules/ipc.md`.

Ordinary in-process `Channel[T]` is not process IPC.

---

## 2. Core distinction: task, thread, and process

The canonical distinction is:

```text
Task
    logical concurrent execution managed by a Sec runtime or scheduler

Thread
    physical execution sharing the owning process address space

Process
    independently scheduled execution in a distinct process isolation domain
```

`spawn process` is a semantic execution-kind request.

A backend must never silently lower:

```sec
spawn process Work()
```

to:

```text
a Task
a Thread
a coroutine
a worker job
an RTOS task without process isolation
```

A target that cannot implement the process semantics required by this rulebook
must reject the relevant operation at compile time.

A scheduler-capable target does not thereby support Sec processes.

---

## 3. Process isolation

### 3.1 Ordinary Sec storage is process-local

Parent and child processes do not share ordinary Sec storage.

Conceptually:

```text
parent process
    ordinary Sec object A

child process
    ordinary Sec object B
```

A value crossing the process boundary is transferred through a canonical
process-transfer representation.

It is not an ordinary alias to parent storage.

Even when a backend uses copy-on-write or a `fork`-like native mechanism, source
semantics remain the process-transfer semantics defined here.

The backend must not expose native address-space-copy implementation details as
ordinary shared Sec object identity.

### 3.2 Ordinary references do not cross

An ordinary safe reference does not become valid in another process.

Invalid:

```sec
fn Inspect(data: ref Data) void {
    // ...
}

let worker := try spawn process Inspect(ref data)
```

The same rule applies to:

```text
ref T
ref mut T
```

The fact that a parent keeps the referenced storage alive until `join` does not
make the reference process-valid.

This is an address-space validity rule, not merely a lifetime rule.

### 3.3 Raw pointers do not become cross-process pointers

Copying the numeric representation of `RawPtr[T]` into another process does not
make it a pointer to corresponding storage in that process.

`unsafe` does not waive this rule.

A process-aware shared-memory or native capability adapter must define any
cross-process pointer-like semantics explicitly.

### 3.4 Explicit shared memory is a separate model

Shared memory is an explicit process-transfer/IPC facility.

Any shared-memory type used by Sec processes must define:

- mapping ownership;
- process-aware addressing;
- mapping lifetime;
- process-compatible synchronization;
- transfer rules;
- destruction and unmapping rules.

Ordinary `ref T` is not such a facility.

---

## 4. Process creation families

### 4.1 Sec-callable process creation

Canonical form:

```sec
let worker := try spawn process Work(message)
```

If `Work(...)` returns `T`, the raw expression has type:

```sec
Result[Process[T], ProcessSpawnError]
```

and the expression after `try` has type:

```sec
Process[T]
```

`spawn process` is eager in Sec 0.1.

There is no deferred `Process[T]` start state and no `ProcessConfig` in Sec 0.1.

### 4.2 External executable creation

External executable invocation uses `Command`.

Canonical form:

```sec
let command := new Command("git", ["status", "--short"])
try command.Start()
```

`new Command(...)` creates a process launch description.

It does not create the external process.

`Command.Start()` is the process-creation boundary.

### 4.3 `Command` is not `Process[T]`

`Command` is the external-executable type.

`Process[T]` is the typed owner returned by `spawn process` for a Sec callable.

They must not be aliases for one another.

The type family is intentionally:

```text
Task[T]
Thread[T]
Process[T]
Command
```

Sec does not add a `Handle` suffix to these owning resource types.

---

## 5. Required compiler-known and core types

All source-visible types in this section are normative Sec 0.1 types.

An implementation may place these declarations in `core` during Sec 0.1.

A compiler must not implement only the names while leaving the members, variants,
or signatures unspecified.

Every enum variant, member, property, method, availability rule, modifier, and
error meaning defined here is part of the language contract.

### 5.1 `ProcessID`

Canonical declaration:

```sec
type ProcessID uint
```

For this compiler-known type, the underlying `uint` width follows the selected
target's canonical unsigned machine width.

Therefore, for example:

```text
64-bit target -> 64-bit ProcessID representation
32-bit target -> 32-bit ProcessID representation
```

`ProcessID` is:

- immutable;
- copyable;
- equality-comparable;
- a Sec process-execution identity;
- not an operating-system PID;
- not an owning process capability.

A `ProcessID` identifies one Sec process execution within the current Sec program
execution.

A runtime must not reuse a `ProcessID` during the same Sec program execution.

If the target-sized identity space is exhausted, further process creation fails
as a resource-exhaustion condition rather than wrapping and reusing an earlier
identity.

Reading a `ProcessID` never grants the right to join, terminate, detach, reap, or
otherwise control the process.

### 5.2 `ProcessStatus`

Canonical declaration:

```sec
enum ProcessStatus {
    Created
    Running
    Completed
    CompletionFailed
    Panicked
    Terminated
}
```

The variants are exhaustive for the portable Sec 0.1 process lifecycle surface.

A backend may have additional internal states, but they must normalize to the
portable states above.

#### `Created`

A process-owning launch object exists, but no child process execution has yet been
established.

This state is used by `Command` before successful `Start()`.

A successfully returned `Process[T]` from `spawn process` is never source-visible
in `Created` state.

#### `Running`

A process execution has been established and no terminal outcome has yet been
observed by the Sec runtime.

`Running` does not mean that the process is executing instructions on a CPU at the
exact instant the property is read.

A process may be scheduled, sleeping, waiting, or blocked and still be `Running`.

#### `Completed`

The execution reached normal completion and its declared normal completion value
has been successfully established in owner-side Sec state.

For `Process[T]`, the normal value is `T`.

For `Command`, the normal value is `ExitStatus`.

A callable whose declared type is `Result[T, E]` and which returns `Err(error)`
still completes normally. The normal value is the returned `Result[T, E]`.

A non-zero external process exit code also remains normal `Completed` execution.

#### `CompletionFailed`

The child reached normal execution completion, but Sec could not establish the
declared normal completion value in owner-side Sec state.

This state is distinct from:

- process creation failure;
- Sec panic;
- abnormal native termination.

The terminal payload is `ProcessCompletionError`.

#### `Panicked`

A Sec callable used as a process entry terminated through Sec panic rather than
normal return, and canonical panic information was successfully established for
the owner.

`Command` does not produce `Panicked` through the portable external-executable
model.

An external executable is not automatically treated as a Sec-callable process
even when the executable itself was compiled from Sec source.

#### `Terminated`

The process ended without normal Sec completion through hard termination,
platform termination, crash/fault, or another abnormal process-level event.

The terminal payload is `ProcessTermination`.

The terminal states are exactly:

```text
Completed
CompletionFailed
Panicked
Terminated
```

The nonterminal states are exactly:

```text
Created
Running
```

### 5.3 `ExitStatus`

Canonical declaration:

```sec
type ExitStatus struct {
    Code: uint32,
}

impl ExitStatus {
    property Success: bool {
        get {
            return Code == 0
        }
    }
}
```

`ExitStatus` is the normal completion value for `Command`.

`Code` is the portable normalized normal exit code.

`Success` is `true` exactly when:

```sec
Code == 0
```

A non-zero `Code` is still a normal `ExitStatus` and therefore still belongs to
`ProcessStatus.Completed`.

`ExitStatus` never represents:

- a Sec panic;
- a hard termination request;
- a signal/fault termination;
- a native crash.

Those conditions use other terminal states.

### 5.4 `ProcessTerminationKind`

Canonical declaration:

```sec
enum ProcessTerminationKind {
    Requested
    External
    Fault
    Unknown
}
```

#### `Requested`

The Sec process owner successfully requested hard termination and the observed
terminal outcome can be attributed to that request.

#### `External`

A process-level actor or platform mechanism outside the owning Sec process
terminated the child without normal completion.

Examples may include:

- another process or user;
- a service manager;
- a container/runtime policy;
- a watchdog;
- an external termination event.

#### `Fault`

Execution ended because of a crash/fault-equivalent event.

Examples may include:

- access violation;
- illegal instruction;
- fatal machine exception;
- fatal process fault signal.

#### `Unknown`

The runtime can prove abnormal termination but cannot classify the termination
more precisely without inventing information.

A backend must use `Unknown` rather than misclassifying an event merely to avoid a
fallback state.

### 5.5 `ProcessTermination`

Canonical declaration:

```sec
type ProcessTermination struct {
    Kind: ProcessTerminationKind,
}
```

Portable process code uses `Kind`.

The portable type does not contain a universal signal number or universal native
termination code.

POSIX signal identities, Windows process status values, and target-specific
termination metadata are not one semantic type and must not be forced into a
portable integer field.

Target-specific immutable termination metadata may be exposed by the selected
platform implementation where its owning platform rulebook defines that metadata
fully.

### 5.6 `ProcessCompletionError`

Canonical declaration:

```sec
enum ProcessCompletionError error {
    TransferFailed
    MaterializationFailed
    OutOfMemory
    ResourceLimit
    ProtocolFailure
    NativeFailure
}
```

#### `TransferFailed`

The child produced a normal value, but the canonical completion representation
could not be transported to the owner.

This may include a runtime failure in serialization, transport, capability
transfer, or another selected process-transfer adapter.

#### `MaterializationFailed`

The completion representation reached owner-side completion machinery, but the
normal Sec value could not be established as a valid owner-side value.

#### `OutOfMemory`

Completion could not be retained or materialized because required memory could
not be obtained.

#### `ResourceLimit`

A runtime/native resource limit prevented completion delivery or materialization.

#### `ProtocolFailure`

The established Sec process completion protocol could not verify or complete a
valid terminal completion record.

This includes malformed or truncated runtime completion state and protocol-state
mismatch. It is not a user-level returned error from the child callable.

#### `NativeFailure`

The target runtime could not establish completion information and no more
specific portable variant describes the failure.

`NativeFailure` is a fallback and must not replace a more precise variant.

### 5.7 `ProcessSpawnError`

Canonical declaration:

```sec
enum ProcessSpawnError error {
    OutOfMemory
    ResourceLimit
    PermissionDenied
    TransferFailed
    BootstrapFailed
    NativeFailure
}
```

#### `OutOfMemory`

Required process-creation, bootstrap, or startup-transfer memory could not be
established.

#### `ResourceLimit`

The runtime or target refused creation because a process, handle, object, IPC,
or equivalent creation resource limit was reached.

#### `PermissionDenied`

The selected runtime/security context denied the requested Sec process creation.

#### `TransferFailed`

A statically valid startup argument, receiver, or capture transfer could not
complete at runtime before startup commit.

This variant applies only to startup transfer.

Normal child return-transfer failures use `ProcessCompletionError`.

#### `BootstrapFailed`

A native/target child could be created or creation could begin, but the canonical
Sec child bootstrap could not establish a runnable callable process before
startup commit.

This includes failures to establish:

- the child entry identity;
- startup argument materialization;
- required completion transport;
- the bootstrap handshake;
- required runtime child initialization before user callable entry.

Any partially created child must be stopped and cleaned up before this error is
returned.

#### `NativeFailure`

A target-native creation failure occurred and no more specific portable variant
applies.

There is no `Unsupported` variant.

A statically unsupported process target is a compile-time error.

### 5.8 `ProcessTerminationError`

Canonical declaration:

```sec
enum ProcessTerminationError error {
    InvalidState
    PermissionDenied
    NotSupported
    ResourceUnavailable
    NativeFailure
}
```

#### `InvalidState`

Termination was requested for a process that is not in a state where a hard
termination request can be issued.

Examples include:

- a `Command` still in `Created`;
- an already `Completed` process;
- an already `CompletionFailed` process;
- an already `Panicked` process;
- an already `Terminated` process.

When the invalid state is statically provable, the compiler should diagnose it
rather than intentionally generating a predictable runtime failure.

#### `PermissionDenied`

The selected target/runtime denied the termination operation.

#### `NotSupported`

The selected target supports the general process-termination capability, but the
particular established process capability cannot be terminated by the portable
operation for a reason that can only be discovered at runtime.

If the selected target statically lacks process termination altogether, the call
is a compile-time error instead.

#### `ResourceUnavailable`

The runtime lifecycle resource required to request termination is no longer
available even though terminal lifecycle state has not yet been established.

#### `NativeFailure`

The native termination operation failed and no more specific portable variant
applies.

### 5.9 `CommandConfigurationError`

Canonical declaration:

```sec
enum CommandConfigurationError error {
    InvalidState
    InvalidWorkingDirectory
    NotFound
    NotDirectory
    PermissionDenied
    InvalidEnvironmentName
    InvalidEnvironmentValue
    OutOfMemory
}
```

#### `InvalidState`

A configuration operation was requested after the `Command` left
`ProcessStatus.Created`.

#### `InvalidWorkingDirectory`

The supplied working-directory representation is structurally invalid or cannot
be represented for the selected target.

#### `NotFound`

`SetWorkingDirectory()` could not find the requested directory at the time the
setter performed its eager check.

#### `NotDirectory`

The requested working-directory path exists but does not identify a directory.

#### `PermissionDenied`

The current execution context cannot use/enter the requested working directory
with the permissions required for process startup at the time of the setter
check.

#### `InvalidEnvironmentName`

An environment variable name violates the portable/target environment-name
rules.

#### `InvalidEnvironmentValue`

An environment variable value cannot be represented for the selected target.

#### `OutOfMemory`

The new configuration could not be retained without violating transactional
configuration semantics.

On every `CommandConfigurationError`, the previously committed command
configuration remains unchanged.

### 5.10 `CommandValidationError`

Canonical declaration:

```sec
enum CommandValidationError error {
    InvalidState
    InvalidExecutable
    NotFound
    PermissionDenied
    InvalidWorkingDirectory
    NotDirectory
    InvalidEnvironmentName
    InvalidEnvironmentValue
    InvalidArgument
    NativeFailure
}
```

#### `InvalidState`

`Validate()` was requested when the command was no longer in `Created`.

#### `InvalidExecutable`

The executable request is structurally invalid or cannot be represented for the
selected target.

An empty executable request is invalid.

#### `NotFound`

The executable or another required filesystem object could not be resolved at
validation time.

#### `PermissionDenied`

The current runtime context does not permit a required executable or directory
access at validation time.

#### `InvalidWorkingDirectory`

The configured effective working-directory representation is invalid for the
selected target.

#### `NotDirectory`

The effective working-directory path exists but is not a directory.

#### `InvalidEnvironmentName`

The effective child environment contains a name that cannot be materialized for
the target.

#### `InvalidEnvironmentValue`

The effective child environment contains a value that cannot be materialized for
the target.

#### `InvalidArgument`

An executable argument cannot be represented by the selected target's process
invocation ABI.

#### `NativeFailure`

The non-starting preflight could not complete because of a target/native failure
that no more specific portable variant describes.

`Validate()` does not reserve process slots, memory, pipe objects, handles, or
other process-creation resources. Therefore creation-resource exhaustion is not a
validation guarantee.

### 5.11 `CommandStartError`

Canonical declaration:

```sec
enum CommandStartError error {
    InvalidState
    InvalidExecutable
    NotFound
    PermissionDenied
    InvalidWorkingDirectory
    NotDirectory
    InvalidEnvironmentName
    InvalidEnvironmentValue
    InvalidArgument
    ResourceLimit
    OutOfMemory
    IOSetupFailed
    NativeFailure
}
```

#### `InvalidState`

`Start()` was requested when the command was no longer in `Created`.

#### `InvalidExecutable`

The executable request could not be used as a valid target executable request at
the authoritative start boundary.

#### `NotFound`

The executable or another required filesystem object could not be resolved when
actual creation was attempted.

This may occur even after a successful earlier `Validate()`.

#### `PermissionDenied`

Actual execution or required launch access was denied.

#### `InvalidWorkingDirectory`

The configured effective working-directory representation is invalid for the
selected target at start time.

#### `NotDirectory`

The effective working-directory path exists but is not a directory at start
time.

#### `InvalidEnvironmentName`

The effective environment contains a target-invalid environment variable name.

#### `InvalidEnvironmentValue`

The effective environment contains a target-invalid value.

#### `InvalidArgument`

An executable argument cannot be materialized in the selected target process
invocation.

#### `ResourceLimit`

Actual process creation or required bootstrap/standard-I/O setup was prevented by
an applicable target/runtime resource limit.

#### `OutOfMemory`

Required process-creation or bootstrap state could not be established because of
memory exhaustion.

#### `IOSetupFailed`

The configured standard-I/O mode is semantically valid, but actual native pipe,
handle-duplication, descriptor-binding, or equivalent standard-I/O setup failed
and no more specific portable variant applies.

#### `NativeFailure`

Actual external process creation failed for a target-native reason that no more
specific portable variant describes.

### 5.12 `CommandInputMode`

Canonical declaration:

```sec
enum CommandInputMode {
    Inherit
    Closed
    Pipe
}
```

#### `Inherit`

The child starts with standard input established from the effective parent
standard input at `Start()` commit.

#### `Closed`

The child starts with a valid standard-input endpoint that contains no user input
and reaches end-of-input.

This variant is not Sec `null` or `nil` semantics.

Sec does not introduce ordinary `null`/`nil` through this API.

A backend may use a target-native empty-input facility to realize the semantics.

#### `Pipe`

`Start()` establishes an anonymous one-way pipe from a parent-side `PipeWriter`
to child standard input.

### 5.13 `CommandOutputMode`

Canonical declaration:

```sec
enum CommandOutputMode {
    Inherit
    Discard
    Pipe
}
```

#### `Inherit`

The corresponding child output stream is established from the matching effective
parent output stream at `Start()` commit.

#### `Discard`

Bytes written by the child to the configured output stream are intentionally
discarded.

This is not Sec `null`/`nil` semantics.

#### `Pipe`

`Start()` establishes an anonymous one-way pipe from the child output stream to a
parent-side `PipeReader`.

### 5.14 `ProcessObserver`

`ProcessObserver` is a compiler-known, non-owning process observation value.

It is copyable.

Its required public surface is exactly:

```text
property ID: ProcessID
property Name: string
property Status: ProcessStatus
property Platform: ProcessPlatform
fn Wait() ProcessStatus
```

All four properties are read-only.

`ProcessObserver` does not expose:

```text
Value
CompletionError
Panic
Termination
Start
Terminate
join capability
detach capability
```

`Wait()` is repeatable.

If the observed process is already terminal, `Wait()` returns immediately with
that terminal `ProcessStatus`.

If it is not terminal, `Wait()` waits until one of:

```text
Completed
CompletionFailed
Panicked
Terminated
```

is observed.

`Wait()` does not join or reap through the owning source lifecycle capability.

It does not make an owning process result available.

The implementation may retain minimal observation state after the owning handle
is joined or detached, but an observer must not keep join-only native resources,
owner-only completion payload storage, or unreaped native process state alive.

### 5.15 `ProcessPlatform`

`ProcessPlatform` is a compiler-known target-resolved immutable process platform
view.

The exact concrete declaration is owned by the selected target/platform.

This is intentionally not one fixed portable struct layout.

Every process-capable target must provide a complete concrete declaration for its
resolved `ProcessPlatform` type.

Every resolved declaration must expose at least the read-only member:

```text
property ID: <target-native process identity type>
```

The placeholder above is not a source type. The selected platform must replace it
with an actual fully declared Sec type appropriate to that target's native process
identity ABI.

The platform rulebook must document that concrete type completely.

`ProcessPlatform.ID` is distinct from `ProcessID`.

It is native identity metadata and is not an owning capability.

Native process identity may be retained as metadata after process termination and
join. A target may reuse the native process identifier for a different future
native process; therefore it must not be used as Sec's stable process identity.

The portable common `ProcessPlatform` surface does not expose a raw owning or
mutating native process handle.

A target-specific API that exposes a raw lifecycle/mutating native handle requires
`unsafe` and must define how the operation interacts with Sec lifecycle state.

Using an unsafe raw handle does not automatically mark a `Process[T]` or `Command`
as joined, detached, or otherwise lifecycle-resolved.

### 5.16 Canonical panic dependency

`Process[T].Panic` uses the canonical `PanicInfo` type owned by the panic
rulebook.

`PanicInfo` and every compiler-known type it requires, including `PanicID`, must
be fully declared by that canonical owner.

This rulebook does not define a second process-specific panic-info type.

---

## 6. `Process[T]`

### 6.1 Declaration and ownership

The canonical compiler-known type is:

```sec
@noCopy
type Process[T]
```

`Process[T]` is generic in the normal callable return type `T`.

It is not user-constructible.

Ordinary source cannot write:

```sec
new Process[int]()
```

A `Process[T]` is created by a successful canonical Sec process-creation
operation, initially:

```sec
spawn process Callable(...)
```

`Process[T]` owns:

- one process lifecycle responsibility;
- one join capability until resolved;
- hard-termination authority where supported;
- native lifecycle/reaping obligations not yet transferred or released;
- canonical terminal process state;
- the normal completion value when one exists;
- canonical process completion error when one exists;
- canonical process panic information when one exists;
- canonical abnormal termination information when one exists;
- runtime bookkeeping required to preserve these semantics.

`Process[T]` is move-only even when `T` is copyable.

Moving `Process[T]` transfers all unresolved lifecycle and result ownership.

Copying it is invalid.

### 6.2 Required public surface

The required public surface is exactly:

```text
property ID: ProcessID
property Name: string
property Status: ProcessStatus
property Value: T
property CompletionError: ProcessCompletionError
property Panic: PanicInfo
property Termination: ProcessTermination
property Platform: ProcessPlatform
fn Observe() ProcessObserver
fn Terminate() Result[void, ProcessTerminationError]
```

All properties are read-only from ordinary source.

The property names and method names use Sec's public API CamelCase convention.

### 6.3 `ID`

`ID` is always available on a successfully created `Process[T]`.

It is Sec identity, not native identity.

### 6.4 `Name`

`Name` is immutable diagnostic/observation metadata.

For a named function or method, the compiler derives `Name` from the canonical
qualified callable identity.

For an anonymous callable, the compiler creates a stable diagnostic name within
the current build.

The generated anonymous name need not be ABI-stable across different builds.

`Name` is not a runtime callable lookup key and is not required to be the native
OS process title.

### 6.5 `Status`

`Status` is a nonblocking snapshot observation.

A successful `spawn process` may return after the child has already completed, so
the first source observation is not guaranteed to be `Running`.

A `Process[T]` is never returned in `Created` state.

### 6.6 `Value`

`Value` is available only after:

```text
successful join
+
Status == ProcessStatus.Completed
```

For copyable `T`, ordinary reads follow normal Sec copy semantics.

For move-only `T`, extracting `Value` transfers ownership of the stored normal
result and makes the stored result unavailable for a second extraction.

This result extraction is a compiler-known process-result-slot operation. It does
not put the outer `Process[T]` into ordinary aggregate partial-move state.

The outer process value remains valid for terminal metadata inspection after a
move-only result is extracted.

The result slot itself becomes unavailable.

If the joined `Process[T]` is destroyed while still owning an unconsumed normal
result, that result is destroyed according to ordinary deterministic destruction.

### 6.7 `Process[void]`

A callable returning `void` creates:

```sec
Process[void]
```

After successful join and normal completion:

```sec
worker.Value
```

exists and has type `void`.

It is not `Option[void]`.

### 6.8 `CompletionError`

`CompletionError` is available only after:

```text
successful join
+
Status == ProcessStatus.CompletionFailed
```

It is unavailable for all other process statuses.

### 6.9 `Panic`

`Panic` is available only after:

```text
successful join
+
Status == ProcessStatus.Panicked
```

`Panicked` may be reported only when the Sec runtime has established canonical
`PanicInfo` for the owner.

If a child disappears during panic handling and canonical panic information
cannot be established, the owner must not receive fabricated `PanicInfo`.

The terminal state is then `Terminated` with the most accurate available
`ProcessTermination` classification.

### 6.10 `Termination`

`Termination` is available only after:

```text
successful join
+
Status == ProcessStatus.Terminated
```

### 6.11 `Platform`

`Platform` is always available for a successfully created `Process[T]`.

It is immutable observation/platform metadata.

### 6.12 `Observe()`

```sec
let observer := worker.Observe()
```

returns:

```sec
ProcessObserver
```

and never transfers lifecycle ownership.

### 6.13 `Terminate()`

```sec
try worker.Terminate()
```

requests hard process termination.

It is defined in detail in the termination section below.

---

## 7. `spawn process`

### 7.1 Result type

If the callable has declared return type `T`:

```sec
fn Work() T {
    // ...
}
```

then:

```sec
spawn process Work()
```

has type:

```sec
Result[Process[T], ProcessSpawnError]
```

Normal style is:

```sec
let worker := try spawn process Work()
```

### 7.2 Nested `Result` is preserved

If:

```sec
fn Load() Result[Document, IOError] {
    // ...
}
```

then:

```sec
let worker := try spawn process Load()
```

has type:

```sec
Process[Result[Document, IOError]]
```

A user-level `Err(IOError...)` returned by the callable is normal callable
completion.

It does not become `ProcessSpawnError`, `ProcessCompletionError`, panic, or
termination.

### 7.3 Eager semantics

`spawn process` is eager in Sec 0.1.

There is no source operation:

```text
Process.Start()
```

and no deferred callable-process configuration.

When the raw spawn returns `Ok(Process[T])`, the process execution is already
established.

The child callable may have begun executing or may already have reached terminal
state before the parent next observes the handle.

### 7.4 Process callable requirement

The selected callable must have a compiler-known process-materialization strategy.

Conceptually, the compiler must know how to establish:

- the concrete callable entry;
- the concrete generic specialization when generic;
- the method receiver representation when applicable;
- closure/capture representation when applicable;
- startup argument schema;
- completion schema;
- child bootstrap identity.

This semantic capability may be represented internally as a compiler fact.

It is not a user-written interface that every callable manually implements.

### 7.5 Named functions

A named function is valid when its callable entry, startup inputs, and return type
satisfy all process rules.

```sec
fn Worker(message: Message) Report {
    // ...
}

let worker := try spawn process Worker(message)
```

### 7.6 Generic functions

A concrete generic specialization may be a process entry:

```sec
let worker := try spawn process Work[int](value)
```

The compiler materializes the already-resolved concrete specialization.

No runtime generic process model is introduced.

### 7.7 Instance methods and `self`

An instance method may be a process entry.

Example:

```sec
type Worker struct {
    Config: Config,
    Cache: Cache,
}

impl Worker {
    fn Run(job: Job) Report {
        return ProcessJob(self.Config, self.Cache, job)
    }
}
```

For:

```sec
let process := try spawn process worker.Run(job)
```

the method implementation code is not serialized as runtime object state.

The compiler/linker materializes the concrete method implementation into the
child-capable program image or equivalent process entry realization.

The receiver value is a process startup input.

Semantically, the receiver value is transferred as a whole receiver value into
child-local state.

Inside the child method:

```sec
self.Config
self.Cache
```

refer to the child-local materialized receiver.

They do not refer to the parent object's memory.

The compiler may optimize physical transfer only when it proves the optimization
preserves the semantics of transferring the complete receiver value.

A receiver type that contains process-invalid ordinary references or other
non-transferable state is not process-transferable merely because the method does
not appear to read those fields in one particular optimization configuration.

### 7.8 Receiver copying and moving

If the receiver is copyable and process-transferable, process startup may transfer
a copy and the parent binding remains available.

If an existing named receiver must be consumed, the call site must make the move
explicit:

```sec
let process := try spawn process (<-worker).Run(job)
```

Omitting `<-` when the receiver must be consumed is a compile-time error.

The method itself does not become a consuming method merely because process
startup moved the receiver into the child. Process startup owns the boundary
transfer; ordinary method semantics apply inside the child-local receiver.

### 7.9 Callable values and lambdas

A callable value or lambda may be process-spawned only when the compiler can
materialize its complete legal process entry and environment.

A capture-less lambda may be valid.

An owned captured environment may be valid when every captured value has a
canonical process-transfer representation.

An ordinary borrowed capture into parent storage is invalid.

When inline capture of an existing named move-only value consumes the binding,
the capture must use the ordinary explicit move marker in the capture expression.

Conceptually:

```sec
capture(<-resource) fn() void {
    Use(resource)
}
```

When an existing callable value itself owns move-only captured state and is
consumed as the process callable, the callable value must likewise be explicitly
moved at the consuming source boundary.

The compiler must never silently clone a resource-owning closure environment.

### 7.10 Runtime-selected callable values

The source callable need not syntactically be a literal function identifier.

A runtime-selected callable is valid only when the compiler/linker has a
process-materialization strategy for the complete legal target set.

An opaque foreign function pointer, arbitrary executable address, or unmodelled
runtime closure is not automatically a valid Sec process callable.

---

## 8. Startup argument, receiver, and capture transfer

### 8.1 Evaluation occurs in the parent

All explicit arguments, receivers, and capture expressions are evaluated in the
parent process using ordinary Sec evaluation order before startup commit.

The backend must not defer ordinary source argument evaluation into the child.

### 8.2 Process transferability is boundary-specific

A startup value must satisfy the canonical process-transferability rules.

`ProcessTransferable` is a compiler semantic fact, not necessarily a user-facing
interface.

A value may be thread-transferable but not process-transferable.

A value may be process-transferable through:

- serialization;
- an explicit process transfer representation;
- a shared-memory capability;
- native handle duplication/inheritance;
- another canonical adapter defined by the owning transfer/IPC rulebook.

Process transferability does not imply bitwise copying.

### 8.3 Copyable named values

A copyable, process-transferable value may cross without consuming the source
binding.

```sec
let count: int := 10
let worker := try spawn process Work(count)
```

The child receives its own transferred value semantics.

### 8.4 Consumed named values require `<-`

When an existing named binding is consumed as process startup payload, the source
must explicitly use `<-`.

This applies to:

- explicit arguments;
- instance-method receivers;
- captured values whose ownership is consumed;
- an existing callable value whose owned environment is consumed.

Example:

```sec
let data := LoadData()
let worker := try spawn process Work(<-data)
```

If `data` must be consumed, this is invalid:

```sec
let worker := try spawn process Work(data)
```

Expected diagnostic shape:

```text
process spawn consumes "data"; write <-data to transfer ownership
```

The explicit marker documents that the existing identifier may become unavailable
when process creation commits.

### 8.5 Fresh temporaries do not require a synthetic move marker

A fresh temporary with no reusable source binding does not require `<-` merely to
prove that an identifier cannot be used afterward.

Example:

```sec
let worker := try spawn process Work(CreateData())
```

remains valid when `CreateData()` produces a consuming startup value.

### 8.6 Transactional ownership commit

Process startup is fallible.

Therefore a source move does not irreversibly commit merely because `<-` was
parsed.

The process-transfer operation must distinguish preparation, commit, and failure
rollback.

For:

```sec
match spawn process Work(<-data) {
    Ok(worker) => {
        // data is unavailable here.
    }

    Err(error) => {
        // data remains available here.
    }
}
```

ownership is committed only on successful process-start commit.

Every pre-commit `ProcessSpawnError` preserves source ownership on the failure
path.

The same rule applies when `try` propagates the failure:

```sec
let worker := try spawn process Work(<-data)
```

On success, `data` becomes unavailable.

On the propagated failure path, the still-owned `data` participates in ordinary
failure-path cleanup/destruction.

A process-transfer adapter that irreversibly destroys source ownership before it
can know whether startup commits does not satisfy the `spawn process` contract.

### 8.7 Startup publication

Child user code must not access startup values until all required startup
arguments, receiver state, and captures have been fully materialized in child-side
Sec state.

Conceptually:

```text
parent evaluation
    ↓
process-transfer preparation
    ↓
child creation/bootstrap
    ↓
child-side materialization
    ↓
startup commit
    ↓
child user callable may access inputs
```

Copyable transferred values are child-local snapshots unless their explicit
transfer adapter defines shared state.

Moved values become child-owned only at successful commit.

### 8.8 Failure after native child creation but before commit

If a native child exists but canonical Sec bootstrap cannot commit, the runtime
must:

- prevent user callable execution from beginning with partial inputs;
- stop the bootstrap child where required;
- reap/clean the partially created child;
- close temporary transport and completion resources;
- roll back transfer preparation;
- preserve source ownership;
- return `Err(ProcessSpawnError...)`.

A failed `spawn process` must never leave a hidden running/orphaned process behind.

### 8.9 Successful commit meaning

`Ok(Process[T])` means all of the following are established:

- a real process execution exists;
- `ProcessID` exists;
- startup inputs are committed;
- moved source ownership has transferred;
- a concrete child process entry is established;
- the completion protocol exists;
- lifecycle responsibility belongs to the returned `Process[T]`.

Failures after this point belong to process lifecycle/completion, not process
creation.

---

## 9. Normal completion and completion transfer

### 9.1 `T` is the normal callable return type

If:

```sec
fn Calculate() int {
    return 42
}
```

then:

```sec
let worker := try spawn process Calculate()
```

has type:

```sec
Process[int]
```

### 9.2 Child return and owner-side value are separate events

A child `return value` first completes the callable locally.

The value must then cross the process boundary using the canonical completion
transfer.

Conceptually:

```text
child callable returns T
    ↓
completion representation/transfer
    ↓
owner-side materialization of T
```

Only after owner-side materialization succeeds is the process `Completed`.

### 9.3 Completion transfer may fail after normal callable return

A callable may return normally while the completion transfer fails because of:

- transport failure;
- capability duplication failure;
- materialization failure;
- memory exhaustion;
- resource exhaustion;
- protocol failure;
- another selected adapter/runtime failure.

In that case:

```text
Status == ProcessStatus.CompletionFailed
```

and there is no normal `Value`.

### 9.4 Return type must be process-transferable

The compiler must reject a statically non-transferable return type at the process
spawn site.

For example, a function returning an ordinary child-local reference cannot be a
normal `Process[T]` completion:

```sec
fn Invalid() ref Data {
    // ...
}
```

unless the returned type itself is a canonical process-aware transfer capability
rather than an ordinary reference.

### 9.5 One terminal typed completion payload is part of process lifecycle

`Process[T]` includes one terminal child-to-owner typed completion payload.

This does not turn `Process[T]` into a general IPC channel.

General ongoing communication, streaming, multi-message transfer, shared memory,
and handle passing belong to `ipc.md`.

---

## 10. External executable `Command`

### 10.1 Declaration and ownership

The canonical compiler-known type is:

```sec
@noCopy
type Command
```

`Command` is move-only for its entire lifetime.

It remains move-only even in `Created`, where no native child exists yet.

This avoids state-dependent copyability.

Moving a `Command` transfers its complete launch configuration and any established
process lifecycle ownership.

### 10.2 Construction

The canonical constructor surface is:

```sec
impl Command {
    init(executable: string, arguments: string[]) {
        // Compiler/core implementation.
    }
}
```

Source usage is:

```sec
let command := new Command("git", ["status"])
```

Construction is infallible at the process API level.

It creates a launch description and sets:

```text
Status == ProcessStatus.Created
```

It does not:

- prove that the executable exists;
- prove that it is executable;
- perform executable search;
- open the executable;
- create native process handles;
- create pipes;
- establish process bootstrap;
- create a child process.

### 10.3 Required public surface

The required `Command` public surface is exactly:

```text
property Executable: ref string
property Arguments: ref string[]
property WorkingDirectory: Option[ref string]
property InheritsEnvironment: bool
property StdinMode: CommandInputMode
property StdoutMode: CommandOutputMode
property StderrMode: CommandOutputMode
property Status: ProcessStatus
property ID: Option[ProcessID]
property Name: string
property Value: ExitStatus
property CompletionError: ProcessCompletionError
property Termination: ProcessTermination
property Platform: Option[ProcessPlatform]

fn SetWorkingDirectory(path: string) Result[void, CommandConfigurationError]
fn ClearWorkingDirectory() Result[void, CommandConfigurationError]
fn SetEnvironment(name: string, value: string) Result[void, CommandConfigurationError]
fn RemoveEnvironment(name: string) Result[void, CommandConfigurationError]
fn ClearEnvironment() Result[void, CommandConfigurationError]
fn InheritEnvironment() Result[void, CommandConfigurationError]
fn SetStdin(mode: CommandInputMode) Result[void, CommandConfigurationError]
fn SetStdout(mode: CommandOutputMode) Result[void, CommandConfigurationError]
fn SetStderr(mode: CommandOutputMode) Result[void, CommandConfigurationError]
fn TakeStdinPipe() Option[PipeWriter]
fn TakeStdoutPipe() Option[PipeReader]
fn TakeStderrPipe() Option[PipeReader]
fn Validate() Result[void, CommandValidationError]
fn Start() Result[void, CommandStartError]
fn Observe() Option[ProcessObserver]
fn Terminate() Result[void, ProcessTerminationError]
```

All properties are read-only from ordinary source.

The `PipeReader` and `PipeWriter` types are owned and fully defined by `ipc.md`.

This rulebook defines exactly how `Command` obtains and transfers ownership of
those endpoints when pipe modes are selected.

### 10.4 `Executable`

`Executable` is the configured executable request.

It is immutable after construction through the portable `Command` API.

Creating a new `Command` is the canonical way to select a different executable.

### 10.5 `Arguments`

`Arguments` is the configured argument array.

It does not include a portable requirement for the user to repeat the executable
as an `argv[0]` element.

The target backend constructs the native invocation representation required by the
selected platform.

Arguments are immutable after construction through the portable `Command` API.

### 10.6 No implicit shell

`Command` never implicitly evaluates a command string through a shell.

This:

```sec
let command := new Command("git status && echo done", [])
```

is an executable request with that literal executable identity. It is not a shell
program.

If a shell is desired, the shell executable and its arguments must be selected
explicitly.

The process API does not implicitly provide:

- shell quoting;
- variable expansion;
- globbing;
- pipeline parsing;
- shell redirection;
- command substitution.

### 10.7 Executable resolution

An executable request containing an explicit target path component is resolved as
that path.

An explicit absolute path uses that path directly.

An explicit relative executable path is resolved relative to the effective child
working directory at `Validate()` or `Start()` respectively.

A bare executable name uses the selected target's canonical executable-search
mechanism.

Where the target uses a PATH-like environment search, the effective child
environment is used.

Executable lookup is not shell execution.

### 10.8 `Name`

`Name` is immutable Sec diagnostic/display metadata derived at construction.

The default is the final executable name component.

Examples conceptually include:

```text
"git" -> "git"
"/usr/bin/git" -> "git"
"tools/compiler" -> "compiler"
```

An invalid/empty executable request may derive an empty display name until
validation rejects the request.

`Name` does not control native process title, native executable lookup, or
arguments.

---

## 11. `Command` working directory

### 11.1 Default

Without explicit configuration, the child inherits the parent process's current
working directory as observed at `Start()`.

Construction does not snapshot the working directory.

### 11.2 `WorkingDirectory`

`WorkingDirectory` has type:

```sec
Option[ref string]
```

Meaning:

```text
None
    use the parent's current working directory at Start()

Some(path)
    use the explicitly configured child working directory
```

### 11.3 `SetWorkingDirectory()`

```sec
try command.SetWorkingDirectory(path)
```

performs an eager current-environment check.

At the time of the call it must verify, as far as the selected target can
portably determine, that the requested path:

- is structurally representable;
- currently exists;
- currently identifies a directory;
- is currently usable/enterable with sufficient permissions for the intended
  process working-directory operation.

A successful setter does not reserve the directory or guarantee that the same
conditions remain true later.

`Validate()` and `Start()` must check the relevant conditions again.

An empty explicit path is structurally invalid.

### 11.4 Relative working directories

A configured relative working directory is interpreted relative to the parent
process's current working directory at the time of the operation that resolves
it.

Therefore a later `Validate()` or `Start()` may observe different external
filesystem state.

### 11.5 `ClearWorkingDirectory()`

```sec
try command.ClearWorkingDirectory()
```

returns the command to inherited-working-directory mode:

```text
WorkingDirectory == None
```

### 11.6 Configuration state

Working-directory configuration methods are valid only in `Created`.

A statically provable post-start configuration attempt is a compiler diagnostic.

When state cannot be proven statically, the method returns:

```sec
Err(CommandConfigurationError.InvalidState)
```

---

## 12. `Command` environment

### 12.1 Default environment

By default:

```text
InheritsEnvironment == true
```

and the child receives the effective parent process environment as observed at
`Start()`.

The environment is a startup snapshot.

The child receives its own process environment state after creation.

### 12.2 Environment configuration model

The observable configuration model consists of:

- inherited or empty base environment;
- explicit overrides;
- explicit removals.

The internal collection representation is not source semantics.

### 12.3 `SetEnvironment()`

```sec
try command.SetEnvironment("LANG", "C")
```

creates or replaces the effective child variable named `LANG`.

An explicit override wins over an inherited value.

Setting a previously removed variable removes its removal marker.

### 12.4 `RemoveEnvironment()`

```sec
try command.RemoveEnvironment("SECRET")
```

ensures that the named variable is absent from the child environment, including
when the same variable exists in the inherited parent environment.

Removing a name also removes an explicit override for that name.

### 12.5 `ClearEnvironment()`

```sec
try command.ClearEnvironment()
```

sets:

```text
InheritsEnvironment == false
```

and resets environment configuration to an empty environment by:

- removing the inherited base;
- clearing explicit overrides;
- clearing explicit removals.

Subsequent `SetEnvironment()` calls build from that empty environment.

### 12.6 `InheritEnvironment()`

```sec
try command.InheritEnvironment()
```

resets environment configuration to the default inherited mode:

```text
InheritsEnvironment == true
```

and clears explicit overrides and removals.

It is a full reset, not an additive merge with previous overrides.

### 12.7 Environment-name validity

A portable environment variable name must:

- be non-empty;
- contain no NUL code unit required to terminate the selected native process API;
- contain no `=` separator.

A selected target may impose additional representability restrictions.

Target environment-name comparison follows the target environment model.

Portable source must not rely on case-sensitive distinctions that the selected
target treats as the same environment name.

### 12.8 Environment-value validity

An environment value may be empty.

It must be representable for the selected target process ABI and may not contain a
native string terminator that the target API cannot represent as data.

### 12.9 Snapshot timing

Inherited environment state is observed independently at `Validate()` and
`Start()`.

A successful `Validate()` does not freeze the parent environment.

---

## 13. `Command.Validate()`

### 13.1 Signature

```sec
fn Validate() Result[void, CommandValidationError]
```

### 13.2 Meaning

`Validate()` performs a non-starting preflight validation of the `Command` as it
is configured at the time of the call.

It may check:

- executable resolution;
- executable existence;
- current execution permission;
- current working-directory validity;
- environment representability;
- argument representability;
- selected standard-I/O capability support.

It does not create the child process.

### 13.3 No reservation guarantee

A successful `Validate()` does not guarantee that a later `Start()` succeeds.

Validation does not reserve:

- executable filesystem objects;
- permissions;
- process slots;
- memory;
- pipe objects;
- native process handles;
- environment state;
- other external dependencies.

External state may change between validation and start.

### 13.4 Lifecycle

`Validate()` is valid only in `Created`.

It never changes `Status`.

Both `Ok()` and `Err(...)` leave:

```text
Status == ProcessStatus.Created
```

and preserve the committed command configuration.

### 13.5 Effects

`Validate()` is not guaranteed nonblocking.

Executable search, filesystem metadata, permission checks, and related target
operations may require blocking/native I/O.

It participates in ordinary blocking/effect analysis.

---

## 14. `Command.Start()`

### 14.1 Signature

```sec
fn Start() Result[void, CommandStartError]
```

### 14.2 Start is the external creation commit boundary

Before successful `Start()`:

```text
no child process lifecycle belongs to Command
ID == None
Platform == None
Observe() == None
```

After successful creation commit:

```text
a real child process exists
ID == Some(ProcessID)
Platform == Some(ProcessPlatform)
Command owns unresolved process lifecycle responsibility
```

An immediate subsequent `Status` read may observe `Running` or a terminal state if
the external executable finished quickly.

### 14.3 Start checks again

`Start()` must perform the authoritative executable, working-directory,
environment, argument, standard-I/O, permission, resource, and native creation
checks required by the selected target.

It must not rely on an earlier `Validate()` as a guarantee.

### 14.4 Failed start rollback

A failed `Start()` performs complete source-visible rollback.

After every `Err(CommandStartError...)`:

```text
Status == ProcessStatus.Created
ID == None
Platform == None
no child lifecycle is owned
configuration remains intact
```

The caller may modify the configuration and retry.

Temporary native process, pipe, handle, bootstrap, and other creation resources
must be released before the error is returned.

A failed `Start()` must not leave a hidden running/orphaned child behind.

### 14.5 Retry

Retry is explicitly valid while the command remains `Created`:

```sec
let command := new Command("worker", args)

match command.Start() {
    Err(CommandStartError.NotFound) => {
        try command.SetEnvironment("PATH", "/opt/workers/bin")
        try command.Start()
    }

    _ => {}
}
```

### 14.6 No chained return requirement

`Start()` returns `Result[void, CommandStartError]`.

The canonical form is:

```sec
let command := new Command("git", args)
try command.Start()
```

`Start()` does not return the `Command` again solely to enable method chaining.

---

## 15. External process normal completion

### 15.1 `Command.Value`

`Command.Value` has type:

```sec
ExitStatus
```

It is available only after:

```text
successful join
+
Status == ProcessStatus.Completed
```

### 15.2 Non-zero exit remains normal completion

For an external program that exits normally with code `1`:

```text
Status == ProcessStatus.Completed
Value.Code == 1
Value.Success == false
```

A non-zero normal exit code is not:

- `CommandStartError`;
- `ProcessCompletionError`;
- `ProcessTermination`.

### 15.3 Abnormal native termination

A signal, access violation, hard kill, crash, or equivalent abnormal termination
uses:

```text
Status == ProcessStatus.Terminated
```

and exposes `Command.Termination` after join.

### 15.4 Completion failure

If the external program reaches normal native completion but Sec cannot establish
the required portable `ExitStatus`, the state is:

```text
ProcessStatus.CompletionFailed
```

and `CompletionError` is available after join.

---

## 16. Standard input, output, and error

### 16.1 Three independent standard streams

`Command` configures independently:

```text
Stdin
Stdout
Stderr
```

Defaults are:

```text
StdinMode  == CommandInputMode.Inherit
StdoutMode == CommandOutputMode.Inherit
StderrMode == CommandOutputMode.Inherit
```

### 16.2 Configuration methods

```sec
try command.SetStdin(CommandInputMode.Pipe)
try command.SetStdout(CommandOutputMode.Pipe)
try command.SetStderr(CommandOutputMode.Inherit)
```

These methods modify launch configuration only.

They do not allocate native pipes.

They are valid only in `Created`.

### 16.3 Actual I/O creation occurs during `Start()`

When one or more modes are `Pipe`, actual anonymous pipe creation and child
binding occur during `Start()`.

The setup participates in the same creation commit as the external process.

If only part of the requested standard-I/O setup succeeds, the operation must
rollback all temporary I/O resources before returning an error.

### 16.4 Pipe direction

For `StdinMode == Pipe`:

```text
parent PipeWriter
    ↓
child stdin
```

For `StdoutMode == Pipe`:

```text
child stdout
    ↓
parent PipeReader
```

For `StderrMode == Pipe`:

```text
child stderr
    ↓
parent PipeReader
```

### 16.5 Pipe ownership is extracted with `Take*Pipe()`

Direct partial move from `Command` is not used for pipe extraction because it
would collide with the ordinary aggregate partial-move rules and could make the
whole `Command` unusable for lifecycle methods such as `join`, `Terminate()`, or
`Observe()`.

The canonical extraction API is:

```sec
fn TakeStdinPipe() Option[PipeWriter]
fn TakeStdoutPipe() Option[PipeReader]
fn TakeStderrPipe() Option[PipeReader]
```

Conceptually, each method consumes an internally retained optional endpoint:

```text
Some(endpoint)
    ↓ Take...
None + returned endpoint
```

The outer `Command` remains a complete usable value.

### 16.6 `TakeStdinPipe()`

Returns `Some(PipeWriter)` only when:

- `Start()` succeeded;
- `StdinMode == CommandInputMode.Pipe`;
- `Command` still owns the parent-side stdin writer.

Otherwise it returns `None`.

### 16.7 `TakeStdoutPipe()`

Returns `Some(PipeReader)` only when:

- `Start()` succeeded;
- `StdoutMode == CommandOutputMode.Pipe`;
- `Command` still owns the parent-side stdout reader.

Otherwise it returns `None`.

### 16.8 `TakeStderrPipe()`

Returns `Some(PipeReader)` only when:

- `Start()` succeeded;
- `StderrMode == CommandOutputMode.Pipe`;
- `Command` still owns the parent-side stderr reader.

Otherwise it returns `None`.

### 16.9 Endpoint lifetime is independent after extraction

Once an endpoint has been returned from a `Take*Pipe()` method, it is an
independently owned IPC resource.

Moving or destroying the `Command` does not destroy an endpoint whose ownership
has already been transferred to another binding.

### 16.10 Remaining endpoints and detach/free

Pipe endpoints still retained by `Command` are ordinary owned resources of the
command/runtime state.

When a resolved command is destroyed, remaining endpoints are closed/destroyed
according to the canonical pipe destruction rules.

When a running command is detached, any endpoint still owned by the consumed
`Command` is released as part of transferred/destroyed owner state.

A caller that needs an endpoint after detach must take ownership before detach.

### 16.11 Closing stdin writer

Closing or destroying the parent `PipeWriter` connected to child stdin causes the
child to observe end-of-input after already buffered bytes have been consumed,
according to canonical pipe semantics.

### 16.12 Closing stdout/stderr reader

Closing a parent read endpoint while the child still writes may cause later child
writes to fail according to canonical pipe/target rules.

The process API does not promise that output writes continue to succeed after all
readers disappear.

### 16.13 Join never drains standard-I/O pipes

`join` does not implicitly read, drain, consume, or buffer piped stdout or stderr.

The runtime must not secretly allocate an unbounded output buffer merely to make
`join` complete.

Therefore this may deadlock:

```sec
try command.SetStdout(CommandOutputMode.Pipe)
try command.Start()

join command
```

if the child fills the stdout pipe and blocks while the parent waits without a
reader.

The caller must arrange an explicit consumer where required.

### 16.14 Existing resource binding

The base `Command` surface in this rulebook does not invent a process-specific
`File`, socket, or raw-native-handle binding exception.

Binding an already existing I/O capability directly into child standard I/O
requires the canonical cross-process capability/handle-transfer semantics owned
by `ipc.md`.

If `ipc.md` defines such a transferable endpoint/capability, `Command` integration
must use that canonical model rather than a second process-specific ownership
model.

An implementation must not silently add ad-hoc file-descriptor or native-handle
redirection semantics that bypass the IPC/transferability rules.

---

## 17. Child-owned process state after creation

### 17.1 Command configuration defines initial state only

`Command` standard-I/O, environment, and working-directory configuration define
the child's initial process state at successful creation.

They are not live references to mutable child process state.

### 17.2 Child may change its own state

After successful creation, the child owns its process-local facilities and may,
when its platform/I/O APIs permit:

- change its working directory;
- change its environment;
- close stdin/stdout/stderr;
- replace standard streams;
- duplicate streams;
- redirect output to files, sockets, or other capabilities;
- perform other process-local changes.

These changes do not mutate the parent-side `Command` configuration.

### 17.3 Pipe endpoints are not retargeted

If the child starts with stdout connected to a parent pipe and later replaces its
stdout, the parent `PipeReader` continues to represent the original pipe.

It does not follow the child's later redirection.

When the child closes the original write side and buffered data is consumed, the
reader eventually observes end-of-input according to pipe semantics.

### 17.4 Sec-callable subprocesses

A `spawn process` child also receives its initial working directory, environment,
and standard streams through the platform's canonical child-process inheritance
semantics at process creation.

After startup, the child may independently change its own process-local state.

`Process[T]` does not acquire the `Command` launch-configuration methods merely
for symmetry.

---

## 18. Process observers

### 18.1 Creating an observer from `Process[T]`

```sec
let observer := worker.Observe()
```

is infallible because a successful `Process[T]` already represents an established
process execution.

### 18.2 Creating an observer from `Command`

`Command` may exist before any process exists.

Therefore:

```sec
command.Observe()
```

has type:

```sec
Option[ProcessObserver]
```

Meaning:

```text
Created -> None
process established -> Some(ProcessObserver)
```

### 18.3 Observer lifetime

An observer may outlive movement, join, or detach of the owning process value.

It does not keep join-only native resources or owner-only terminal payloads alive.

The runtime may preserve minimal immutable observation state such as:

- `ProcessID`;
- name;
- terminal status;
- target-resolved immutable platform metadata required by the observer.

### 18.4 Observer cannot resolve lifecycle ownership

`ProcessObserver.Wait()` does not satisfy the owning handle's join/detach
obligation.

An owner must still explicitly resolve lifecycle ownership.

---

## 19. Join

### 19.1 Raw process handles are not awaitable

A raw `Process[T]` or `Command` is not awaitable in Sec 0.1.

Invalid:

```sec
await worker
```

Process lifecycle completion uses `join`.

### 19.2 Syntax

```sec
join worker
```

or:

```sec
join command
```

### 19.3 What join waits for

A successful join waits until the complete Sec terminal process outcome is known.

This includes, where applicable:

- native process termination/completion observation;
- normal completion transfer;
- owner-side completion materialization;
- panic reporting;
- abnormal termination classification;
- required native reaping/collection.

### 19.4 Join consumes join capability, not the whole value

A successful join:

- consumes the one join capability;
- establishes process completion synchronization;
- resolves join-owned native lifecycle/reaping resources;
- preserves process identity;
- preserves immutable terminal status;
- preserves an unconsumed normal result;
- preserves terminal metadata payloads;
- marks the owning source value as joined.

The same binding remains usable after join:

```sec
join worker

let id := worker.ID
let status := worker.Status
```

A second join is invalid.

### 19.5 Status polling does not replace join

This does not make `Value` available:

```sec
if worker.Status == ProcessStatus.Completed {
    // worker.Value is still unavailable before join.
}
```

Status is observational.

Join establishes owner-side terminal synchronization and lifecycle resolution.

### 19.6 Terminal payload availability

After successful join:

```text
Completed
    -> Value available

CompletionFailed
    -> CompletionError available

Panicked
    -> Panic available on Process[T]

Terminated
    -> Termination available
```

`Command` has no portable `Panic` payload because external command execution is
not the typed Sec-callable panic protocol.

### 19.7 Join from a task

When the selected backend can register process completion without blocking the
physical executor worker, joining from task context suspends the current task.

The backend may use target-appropriate mechanisms such as a process completion
event, native process handle wait registration, process notification source, or
runtime watcher.

### 19.8 Join from a physical thread

Joining from a physical thread may park or block the physical thread.

### 19.9 Backend fallback

A backend may physically block an executor worker for process completion only when
the selected profile explicitly permits that realization.

It must not silently introduce forbidden executor starvation.

---

## 20. Process synchronization and memory model

### 20.1 Startup publication

Successful process-start commit establishes ordering sufficient for the child to
observe fully materialized startup arguments, receiver state, and captures.

No child user code may observe a half-published startup value.

### 20.2 Join establishes process lifecycle synchronization

Successful `join` establishes ordering for:

- process lifecycle completion;
- canonical process completion transfer;
- canonical terminal metadata;
- native lifecycle/reaping resolution.

It proves that the joined child process can perform no further execution.

### 20.3 Join does not publish arbitrary child memory

Process join does not mean:

```text
all arbitrary child addresses become visible in the parent
```

or:

```text
all child ordinary writes become parent ordinary-memory writes
```

Ordinary child storage remains process-local.

### 20.4 Shared-memory quiescence

After successful join, the owner knows that the joined child can perform no
future accesses to an explicit shared-memory capability.

This is a useful lifecycle/quiescence fact.

It does not by itself replace the synchronization/publication rules defined by the
shared-memory type.

### 20.5 IPC owns IPC synchronization

Pipe send/read, message send/receive, shared-memory locks, process-aware atomics,
events, and other IPC operations own their own ordering guarantees.

Process join does not duplicate or replace those contracts.

### 20.6 Detach establishes no completion synchronization

Detach does not prove that the child is done and does not publish a completion
value.

### 20.7 Termination request establishes no completion synchronization

A successful hard termination request is not proof that the child has already
stopped or been reaped.

`join` remains the terminal synchronization operation.

---

## 21. `ProcessObserver.Wait()` and `select`

### 21.1 `Wait()` semantics

```sec
let status := observer.Wait()
```

waits until terminal process observation and returns exactly one of:

```text
ProcessStatus.Completed
ProcessStatus.CompletionFailed
ProcessStatus.Panicked
ProcessStatus.Terminated
```

It is repeatable and non-owning.

### 21.2 Wait from task/thread contexts

From a task, `Wait()` suspends where the backend can register process completion
without blocking the executor worker.

From a physical thread, it may park/block.

The same selected-profile fallback restrictions as process join apply.

### 21.3 Selectable process join

Owning process join is selectable:

```sec
select {
    join worker => {
        HandleProcess(worker.Status)
    }

    message := rx.Receive() => {
        HandleMessage(message)
    }
}
```

The join branch is ready when process join can commit without further waiting.

If selected, it performs a real join and consumes the join capability.

If not selected, it must not:

- consume join capability;
- reap through the owner capability;
- take terminal payloads;
- detach;
- change process ownership.

### 21.4 Selectable observer wait

Non-owning completion observation is selectable:

```sec
select {
    status := observer.Wait() => {
        Report(status)
    }

    after 1s => {
        ReportTimeout()
    }
}
```

Selecting this branch only commits the observer wait.

It does not join the owner or unlock owner payloads.

---

## 22. Hard termination

### 22.1 Portable hard termination

Both `Process[T]` and a running `Command` expose:

```sec
fn Terminate() Result[void, ProcessTerminationError]
```

The operation requests the selected platform to forcibly end process execution
without requiring cooperation from child user code.

### 22.2 Hard process termination is not `unsafe`

Portable process termination is not `unsafe` merely because it is destructive.

The process isolation boundary prevents ordinary child memory from being shared
as the parent's ordinary Sec object graph.

This differs from hard termination of a shared-address-space thread.

### 22.3 No child cleanup guarantee

Hard termination makes no guarantee that the child executes:

- `defer`;
- deterministic destruction;
- buffer flushes;
- user shutdown handlers;
- transaction rollback;
- normal return;
- normal completion transfer.

Process isolation protects parent Sec memory safety. It does not make child
external side effects transactional.

### 22.4 Successful request is not completed termination

```sec
try worker.Terminate()
```

means that the selected platform accepted the hard-termination request.

It does not necessarily mean that the process is already terminal and reaped at
the instant the method returns.

Canonical destructive shutdown is:

```sec
try worker.Terminate()
join worker
```

After join, the terminal state may be inspected.

### 22.5 Termination race with normal completion

Normal completion and termination may race.

Exactly one terminal outcome wins.

If normal completion commits first, the process is `Completed` or
`CompletionFailed` as appropriate.

If hard termination prevents normal completion first, the process is
`Terminated`.

The process must never expose two terminal outcomes for one execution.

### 22.6 `Requested` classification

When the terminal abnormal outcome can be attributed to a successful owner
`Terminate()` request:

```text
Termination.Kind == ProcessTerminationKind.Requested
```

---

## 23. Detach

### 23.1 Syntax for `Process[void]`

```sec
detach worker
```

is valid for `Process[void]`.

### 23.2 Non-void result requires explicit discard

For `Process[T]` where `T` is not `void`:

```sec
detach worker discard
```

is required.

The programmer must explicitly acknowledge that the normal result will not be
owned by the former process handle.

### 23.3 `Command` detach

`Command` has normal result type `ExitStatus`.

Therefore a started unresolved command uses explicit result discard:

```sec
detach command discard
```

### 23.4 Detach semantics

Detach:

- consumes the owning process value;
- relinquishes the join capability;
- relinquishes normal result ownership;
- relinquishes owner terminal payload ownership;
- transfers required reaping/native cleanup responsibility to the runtime or
  target lifecycle mechanism;
- allows the child to continue running;
- establishes no completion synchronization edge.

Detach is not termination.

### 23.5 Platform-qualified independent execution

Detach removes the Sec owner-child lifetime dependency.

On a selected platform that provides process execution and the required detached
lifecycle/reaping facilities, normal termination of the former Sec owner does not
itself terminate the detached child.

A detached child may continue independently.

This guarantee is platform-capability qualified.

Bare-metal, schedulerless, or other targets that do not provide canonical process
execution cannot pretend to implement detach; relevant process operations are
compile-time errors.

A process-capable target that cannot provide canonical detach/reaping semantics
must reject `detach` at compile time even when other process operations are
supported.

External platform policies outside Sec semantics may still terminate a detached
child independently, including:

- container shutdown;
- service-manager policy;
- process-group policy;
- job/session policy;
- machine shutdown;
- external administrative termination.

### 23.6 Detached reaping

Detach must not create unreaped native process leakage.

On Unix-like targets, the runtime/platform realization must prevent detached
children from becoming permanently leaked zombies.

On targets with retained native process handles, those handles must be released
according to the target lifecycle contract.

Reaping/cleanup must not implicitly kill a child merely because the child was
detached.

### 23.7 Observer after detach

An observer may remain usable after detach.

The runtime may reap native process state and retain only minimal observation
metadata required to report eventual terminal status.

---

## 24. Owning lifecycle and destruction

### 24.1 Unresolved process owners may not silently leave scope

A successfully started `Command` or a successful `Process[T]` owns unresolved
process lifecycle responsibility until that responsibility is:

- joined;
- detached;
- moved to another owner;
- otherwise explicitly resolved by a canonical operation.

An unresolved owner leaving scope is a compile-time lifecycle error when the
compiler can prove it.

### 24.2 No implicit join in destruction

Destruction must not silently wait for a forgotten process.

Scope exit must not turn into an arbitrary hidden blocking join.

### 24.3 No implicit termination in destruction

Destruction must not silently kill a child process.

### 24.4 No implicit detach in destruction

Forgetting to resolve a process owner must not silently become fire-and-forget
execution.

### 24.5 Created `Command` destruction

A never-started `Command` owns no child lifecycle and may be destroyed normally.

### 24.6 Resolved owner destruction

A joined process owner may still retain:

- normal result storage;
- terminal metadata;
- platform metadata;
- remaining parent-side pipe endpoints for `Command`;
- runtime observation/bookkeeping state.

These resources participate in deterministic destruction.

### 24.7 Lifecycle drain step

The compiler/runtime implementation of `free` for both `Command` and `Process[T]`
must perform an internal infallible lifecycle drain step after process lifecycle
responsibility has already been explicitly resolved.

The canonical destruction shape is:

```sec
impl Command {
    free {
        Drain()
    }
}

impl Process[T] {
    free {
        Drain()
    }
}
```

In this declaration shape, `Drain()` names the compiler/runtime destruction-only
operation defined by this section. It is not an ordinary public method and is not
callable from user source.

This rulebook refers to that internal destruction step as `Drain`.

`Drain` is not a public source method and cannot be called by ordinary user code.

Its semantic signature is:

```text
Drain(resolved process owner) -> void
```

The step may release:

- no-longer-needed native process handles;
- completed process bookkeeping;
- runtime registrations;
- retained platform lifecycle resources;
- other owner-side lifecycle resources safe to release after resolution.

It must not:

- join an unresolved process;
- terminate a process;
- detach a process;
- read stdout;
- read stderr;
- drain pipe data;
- consume user-visible process output;
- return `Result`.

`Drain` in this section is lifecycle cleanup. It is explicitly not standard-I/O
`drain` semantics.

After custom lifecycle cleanup, remaining initialized owned Sec fields/resources
are destroyed according to the ordinary destruction rules.

The lifecycle drain step must not manually double-destroy resources already
represented as ordinary owned fields.

### 24.8 Runtime invariant

If an unresolved process owner reaches the resolved-owner lifecycle drain path
after successful semantic validation, this is a compiler/runtime invariant
failure, not ordinary source behavior that the destructor should silently repair.

---

## 25. Process identity and native identity

### 25.1 Stable Sec identity

`ProcessID` remains stable for the Sec process execution even after terminal
completion and join.

### 25.2 Native identity may be reused

`Platform.ID` is target-native identity metadata.

A target may reuse a native process identifier after a native process has ended.

Therefore portable code that requires stable Sec execution identity uses:

```sec
process.ID
```

not:

```sec
process.Platform.ID
```

### 25.3 Moving ownership does not change process identity

```sec
let id := worker.ID
let other := <-worker
```

preserves the same execution identity:

```text
other.ID == id
```

Moving a Sec owner does not change operating-system parentage or native process
identity.

### 25.4 Owning process values are not automatically cross-process transferable

A `Process[T]`, `Command`, or `ProcessObserver` is not automatically a valid
cross-process process-control payload merely because its metadata can be copied.

Transferring a process-control capability into another process requires an
explicit canonical process-transfer/IPC mechanism.

---

## 26. Target process capabilities

The selected target/CompilationPlan must carry process capabilities as compiler
facts.

These are not runtime bool properties and are not user-written enums.

The capability model must distinguish at least:

```text
ProcessExecution
ExternalProcessExecution
ProcessDetach
ProcessTermination
ProcessStandardIOInheritance
ProcessPipes
```

### 26.1 `ProcessExecution`

The target can implement canonical:

```sec
spawn process Callable(...)
```

including:

- distinct process isolation;
- startup transfer;
- child bootstrap;
- process identity;
- normal completion transfer;
- join/completion observation;
- native lifecycle cleanup/reaping.

A target lacking this capability rejects `spawn process` at compile time.

### 26.2 `ExternalProcessExecution`

The target can launch an external executable through `Command.Start()`.

This capability is distinct from `ProcessExecution`.

A target may theoretically support one without the other.

Constructing a `Command` value does not itself require external process execution,
but `Validate()` and `Start()` require the capability for the selected target.

### 26.3 `ProcessDetach`

The target/runtime can relinquish Sec owner-child lifecycle responsibility while
still satisfying required detached cleanup/reaping semantics.

If absent, `detach` is a compile-time error.

### 26.4 `ProcessTermination`

The target provides the portable hard process-termination operation defined by
this rulebook.

If absent statically, `Terminate()` is a compile-time error.

`ProcessTerminationError.NotSupported` remains for dynamic per-process limitations
on a target that otherwise supports the feature.

### 26.5 `ProcessStandardIOInheritance`

The target can establish the canonical initial parent-to-child standard stream
inheritance required by the selected creation form.

`spawn process` requires this capability for its default Sec 0.1 standard-stream
inheritance semantics.

`Command.Start()` requires it for every standard stream configured as `Inherit`.

### 26.6 `ProcessPipes`

The target can establish the anonymous one-way process pipes defined by this
rulebook and `ipc.md`.

Selecting a `Pipe` mode on a target that statically lacks this capability is a
compile-time error.

Runtime pipe setup failure on a target that supports the capability uses the
appropriate runtime start error, including `IOSetupFailed`, `ResourceLimit`, or
`OutOfMemory` as applicable.

### 26.7 Reaping is not an optional add-on to owning process support

A target cannot claim canonical owning process support if it cannot resolve the
native terminal lifecycle required by `join`.

There is no valid process implementation that creates children but permanently
leaks their terminal native lifecycle state.

### 26.8 Static unsupported versus runtime failure

Canonical rule:

```text
CompilationPlan proves the capability impossible
    -> compile-time diagnostic

CompilationPlan provides the capability but the actual operation fails
    -> Result error
```

A compiler must not hide a statically known unsupported process operation inside a
runtime `NativeFailure`.

---

## 27. Bare-metal, RTOS, and target-equivalent processes

### 27.1 Bare-metal without process isolation

A bare-metal target without an isolated process execution model does not support
Sec process execution merely because it can execute code concurrently.

Such targets reject:

```sec
spawn process Work()
```

and external command start operations that require unavailable process support.

### 27.2 Schedulerless targets

A schedulerless target without process facilities has no canonical process
creation, join, detach, process reaping, or process hard-termination semantics.

The compiler rejects the corresponding source operations.

### 27.3 RTOS tasks are not automatically processes

An RTOS task or scheduler entity is not a Sec process merely because it has a
separate stack or scheduling identity.

If it shares an address space and lacks the required process isolation/transfer
boundary, it must use the task/thread model appropriate to the target rather than
`process`.

### 27.4 Target-equivalent process implementation

A nontraditional platform may implement Sec processes if it genuinely provides:

- a distinct process isolation domain;
- explicit startup transfer;
- independent process identity;
- typed completion transport;
- join/completion observation;
- lifecycle cleanup;
- the target capabilities required by the source operations used.

The underlying platform does not need to use POSIX terminology.

The semantics, not the native API name, determine whether the target supports Sec
processes.

---

## 28. Parent exit and detached children

### 28.1 Owned children before normal parent completion

Normal parent process completion must not silently abandon unresolved owned child
processes.

All owned `Process[T]`/started `Command` values reachable through normal parent
completion paths must have their lifecycle responsibility joined, detached,
transferred, or otherwise canonically resolved.

### 28.2 Detached children

A detached process is no longer lifetime-bound to the former Sec owner.

On a platform that provides the required independent process/detach facilities,
normal completion of the former owner does not itself terminate the detached
child.

### 28.3 Abnormal owner death

Sec cannot guarantee ordinary deterministic cleanup when the parent itself dies
abnormally through external kill, machine failure, container shutdown, fatal
fault, or equivalent platform event.

Target/external policy may terminate or preserve the child independently.

The source-level detach guarantee does not override external system policy.

---

## 29. Cancellation independence

### 29.1 Task cancellation does not imply process cancellation

Cancellation of a task that owns or waits on a process does not implicitly:

- terminate the process;
- detach the process;
- discard the process result;
- transfer lifecycle responsibility.

Process control is explicit.

### 29.2 Cancellation paths must resolve process ownership

If cancellation control flow can leave a scope that owns an unresolved process,
ordinary ownership/lifecycle analysis must require an explicit valid resolution
path.

Application policy may choose, for example:

```text
Terminate + join
detach
transfer ownership
```

but the compiler/runtime must not choose an implicit process policy merely because
the surrounding task was cancelled.

### 29.3 Observer wait cancellation

A cancellable task may stop waiting on a `ProcessObserver` according to the
ordinary task cancellation/wait rules without thereby changing child process
lifecycle ownership.

---

## 30. Blocking, ISR, and effects

### 30.1 Potentially waiting operations

The following process operations may wait or perform native/blocking work:

```text
join Process[T]
join Command
ProcessObserver.Wait()
spawn process ...
Command.Validate()
Command.Start()
Command.SetWorkingDirectory(...)
Process[T].Terminate()
Command.Terminate()
```

Their actual effect classification follows the selected lowering and the
canonical blocking/effect rulebooks.

### 30.2 `SetWorkingDirectory()` effect

Because `SetWorkingDirectory()` performs real current filesystem/directory and
permission checks, it is not merely an in-memory builder mutation and is not
guaranteed `@noBlock`.

### 30.3 `Validate()` effect

`Validate()` may perform executable search, filesystem access, and permission
checks and is not guaranteed `@noBlock`.

### 30.4 ISR restrictions

The following are not ISR-safe in Sec 0.1:

```text
spawn process
Command.Validate()
Command.Start()
Command.SetWorkingDirectory()
join Process[T]
join Command
ProcessObserver.Wait()
Process[T].Terminate()
Command.Terminate()
```

A bounded immutable metadata read such as `observer.Status` may be allowed only
when the selected target/core implementation satisfies the canonical ISR access
rules.

The interrupt rulebook owns final ISR call-path validation.

### 30.5 Live guards across waits

Existing blocking rules that prohibit holding incompatible live synchronization
guards across waiting operations apply equally to process join and observer wait.

This rulebook does not create a process-specific exception.

---

## 31. Data-race and deadlock analysis

### 31.1 Ordinary parent/child storage is not shared

Ordinary parent and child Sec objects do not form one shared-memory race domain.

A data-race analysis must not invent a race merely because a value was copied or
moved through process startup transfer.

### 31.2 Explicit shared memory reintroduces shared concurrency

A process-aware shared-memory abstraction introduces its own shared concurrency
domain.

The data-race analysis for that abstraction must understand:

- shared region identity;
- process participants;
- reads/writes;
- process-compatible synchronization;
- mapping lifetime;
- mapping/value ownership.

Those details are owned by the shared-memory/IPC/data-race rulebooks.

### 31.3 Process wait edges

Deadlock analysis must be able to represent process wait dependencies such as:

```text
parent waits to join child
child waits for IPC from parent
child blocks writing a full stdout pipe
child waits on a shared process synchronization primitive
process A waits for process B
```

### 31.4 Pipe backpressure

A provable cycle involving process join and undrained pipe backpressure may be a
compile-time deadlock diagnostic under the canonical deadlock rules.

When only a plausible risk is known, the analysis may emit the appropriate
warning/configurable diagnostic rather than changing runtime semantics.

### 31.5 Detached execution remains live

After detach, analysis must not treat the child as completed.

A detached process may continue:

- performing I/O;
- using explicit IPC/shared-memory capabilities;
- producing external side effects.

Runtime ownership of reaping responsibility is not proof of process completion.

---

## 32. Process call-site move diagnostics

The compiler must track consuming process startup precisely.

If a named binding must be consumed but `<-` is omitted, compilation fails.

Examples of expected diagnostic shapes:

```text
process spawn consumes "data"; write <-data to transfer ownership
```

```text
process spawn consumes receiver "worker"; write <-worker at the process boundary
```

The diagnostic should explain:

- which source binding would be consumed;
- why the process boundary requires ownership transfer;
- where `<-` must be written;
- that the binding becomes unavailable only after successful startup commit;
- that failed startup preserves ownership on the failure path.

Compiler diagnostics must not describe a process-transfer move as an ordinary
borrow merely because an instance method normally receives implicit `self`.

---

## 33. Process status and payload availability

### 33.1 `Process[T]`

The normative availability matrix is:

| Member | Running | Terminal before join | Completed after join | CompletionFailed after join | Panicked after join | Terminated after join |
|---|---:|---:|---:|---:|---:|---:|
| `ID` | yes | yes | yes | yes | yes | yes |
| `Name` | yes | yes | yes | yes | yes | yes |
| `Status` | yes | yes | yes | yes | yes | yes |
| `Platform` | yes | yes | yes | yes | yes | yes |
| `Observe()` | yes | yes | yes | yes | yes | yes |
| `Value` | no | no | yes | no | no | no |
| `CompletionError` | no | no | no | yes | no | no |
| `Panic` | no | no | no | no | yes | no |
| `Termination` | no | no | no | no | no | yes |

`Terminal before join` means status polling has observed a terminal state but the
owner has not joined.

Payloads remain unavailable until join.

### 33.2 `Command`

The normative availability model is:

| Member | Created | Running | Terminal before join | Completed after join | CompletionFailed after join | Terminated after join |
|---|---:|---:|---:|---:|---:|---:|
| `Executable` | yes | yes | yes | yes | yes | yes |
| `Arguments` | yes | yes | yes | yes | yes | yes |
| `WorkingDirectory` | yes | yes | yes | yes | yes | yes |
| `InheritsEnvironment` | yes | yes | yes | yes | yes | yes |
| `StdinMode` | yes | yes | yes | yes | yes | yes |
| `StdoutMode` | yes | yes | yes | yes | yes | yes |
| `StderrMode` | yes | yes | yes | yes | yes | yes |
| `Status` | yes | yes | yes | yes | yes | yes |
| `ID` | `None` | `Some` | `Some` | `Some` | `Some` | `Some` |
| `Platform` | `None` | `Some` | `Some` | `Some` | `Some` | `Some` |
| `Observe()` | `None` | `Some` | `Some` | `Some` | `Some` | `Some` |
| `Value` | no | no | no | yes | no | no |
| `CompletionError` | no | no | no | no | yes | no |
| `Termination` | no | no | no | no | no | yes |

`Command` does not expose a portable `Panic` member.

---

## 34. `Command` configuration state rules

The following methods are configuration operations and are valid only in
`Created`:

```text
SetWorkingDirectory
ClearWorkingDirectory
SetEnvironment
RemoveEnvironment
ClearEnvironment
InheritEnvironment
SetStdin
SetStdout
SetStderr
```

When invalid state is statically known, the compiler should diagnose the call.

When state is runtime-dependent, they return:

```sec
Err(CommandConfigurationError.InvalidState)
```

Every configuration mutation is transactional from source semantics.

On error, the previously committed configuration remains unchanged.

`Start()` success freezes launch configuration for that process execution.

The child may later change its own process-local state, but those changes do not
make the parent-side launch configuration mutable again.

---

## 35. Process creation diagnostics

A selected target without callable process execution must reject:

```sec
spawn process Work()
```

with a process-specific diagnostic such as:

```text
target profile does not support process execution
```

A selected target without external executable execution must reject relevant
`Command.Validate()`/`Command.Start()` use with a diagnostic such as:

```text
target profile does not support external process execution
```

A selected target without detach support must reject:

```text
target profile does not support detached process execution
```

A selected target without process pipe support must reject a requested pipe mode
with a process-specific diagnostic such as:

```text
target profile does not support process pipe creation
```

Other required diagnostic families include:

```text
process has already been joined
```

```text
process value is unavailable before successful join
```

```text
process completed without a normal value because completion transfer failed
```

```text
process completed without a normal value because it panicked
```

```text
process completed without a normal value because it was terminated
```

```text
detaching Process[T] with non-void result requires explicit discard
```

```text
external Command detach requires explicit ExitStatus discard
```

```text
process spawn cannot transfer ordinary reference across process boundary
```

```text
process spawn consumes "value"; write <-value to transfer ownership
```

```text
process owner leaves scope with unresolved lifecycle responsibility
```

Process diagnostics must use stable process-specific diagnostic IDs.

Task, thread, and process diagnostics must not share one ambiguous diagnostic ID
merely because their wording is similar.

---

## 36. Semantic analysis requirements

The compiler must validate at least:

- selected target process capabilities;
- callable process materializability;
- concrete generic callable specialization;
- receiver transferability;
- explicit argument transferability;
- closure/capture transferability;
- explicit `<-` on every consumed existing named process-start value;
- no ordinary `ref`/`ref mut` crossing the process boundary;
- startup transfer adapters;
- transactional startup ownership commit;
- process return transferability;
- `Process[T]` result type;
- `ProcessSpawnError` result type;
- one join capability;
- terminal payload availability;
- move-only normal result extraction;
- process lifecycle scope resolution;
- detach result discard;
- observer restrictions;
- target-specific platform access;
- hard-termination availability;
- command lifecycle state;
- command configuration mutation state;
- command working-directory validity/effects;
- command environment structural validity;
- command argument representability where statically knowable;
- command standard-I/O mode capabilities;
- pipe ownership extraction state;
- blocking/effect constraints;
- ISR restrictions;
- `select` readiness/commit correctness;
- cancellation-reachable process ownership paths.

Semantic analysis must not infer process support merely from scheduler, thread, or
RTOS-task support.

---

## 37. Semantic IR requirements

Semantic IR must preserve process semantics explicitly.

It must not lower source process operations directly to opaque native calls before
process ownership, transfer, lifecycle, and completion facts are represented.

At minimum the semantic representation must be able to distinguish operations
conceptually equivalent to:

```text
ProcessSpawn
CommandCreate
CommandConfigure
CommandValidate
CommandStart
ProcessMoveOwner
ProcessObserve
ProcessObserverWait
ProcessJoin
ProcessTerminateRequest
ProcessDetach
ProcessDetachDiscard
ProcessTakeStdinPipe
ProcessTakeStdoutPipe
ProcessTakeStderrPipe
ProcessComplete
ProcessCompletionFailed
ProcessPanic
ProcessTerminated
ProcessLifecycleDrain
ProcessTransferPrepare
ProcessTransferCommit
ProcessTransferRollback
```

The exact internal operation names are implementation-defined.

The semantic distinctions are not.

### 37.1 `ProcessSpawn` facts

A process spawn representation must preserve at least:

- execution kind `Process`;
- callable identity;
- concrete specialization;
- receiver when present;
- arguments;
- captures/environment when present;
- copied versus consumed source values;
- required explicit move markers/provenance;
- selected process transfer adapters;
- return type `T`;
- `Process[T]` result type;
- `ProcessSpawnError` failure type;
- bootstrap strategy;
- completion transport;
- target/CompilationPlan;
- source location.

### 37.2 Command facts

A command representation must preserve at least:

- executable request;
- argument array;
- effective working-directory configuration;
- environment inheritance/override/removal configuration;
- standard-I/O modes;
- command lifecycle state;
- start commit boundary;
- pipe ownership slots;
- `ExitStatus` completion type;
- target/CompilationPlan;
- source location.

### 37.3 Transactional transfer

Semantic IR must represent startup transfer preparation separately from successful
ownership commit where a consumed source value can be restored on creation
failure.

A backend must not erase this distinction by lowering an explicit source move to
an irreversible native transfer before the process start commit is known.

### 37.4 Process kind cannot be inferred later

The backend must receive explicit process execution kind from semantic IR.

It must not decide from callable shape or target convenience whether a source
spawn was intended to be a task, thread, or process.

---

## 38. Lowering and runtime obligations

A conforming process runtime/backend must provide the mechanisms necessary for the
selected process capabilities, including as required:

- process identity allocation;
- process creation;
- child bootstrap;
- startup transfer preparation/commit/rollback;
- completion transfer;
- terminal status observation;
- panic reporting for Sec-callable subprocesses;
- native abnormal termination normalization;
- join wait registration/blocking;
- native terminal collection/reaping;
- detach reaping responsibility;
- hard termination;
- standard-I/O inheritance;
- process pipe creation;
- owner/observer retained state separation;
- resolved lifecycle drain.

Runtime implementation details may differ between targets.

Portable source semantics may not.

---

## 39. Cross-rulebook ownership and required synchronization

### 39.1 `spawn.md`

`rules/concurrency/spawn.md` must treat the process form as:

```sec
spawn process Work()  // Result[Process[T], ProcessSpawnError]
```

and must not inherit task/thread borrow rules into the process boundary.

The process-specific rules in this book own process callable materialization,
startup transfer, process handle type, and process creation failure.

### 39.2 `transferability.md`

`rules/memory/transferability.md` owns the canonical process-transferability proof
and adapter contract.

This rulebook consumes that proof for:

- startup arguments;
- receivers;
- captures;
- normal completion values;
- process-aware capabilities.

### 39.3 `blocking.md`

`rules/concurrency/blocking.md` owns generic blocking/effect rules.

This rulebook classifies process join, process observer wait, process creation,
validation, working-directory validation, and termination as operations that must
participate in those analyses.

### 39.4 `select.md`

`rules/concurrency/select.md` must include canonical process completion readiness
for:

```sec
join process
```

and:

```sec
status := observer.Wait()
```

with the readiness/commit semantics defined here.

### 39.5 `panic.md`

The panic rulebook owns `PanicInfo` and `PanicID`.

Those types must be fully declared by that book and made available to the process
implementation.

This rulebook owns only the process containment/transport use of canonical panic
information.

### 39.6 `ipc.md`

`rules/ipc.md` owns:

- `PipeReader`;
- `PipeWriter`;
- general process messaging;
- process shared memory;
- general handle/capability transfer;
- direct binding of existing transferable I/O capabilities where defined;
- IPC-specific synchronization and lifetime semantics.

This book owns how `Command` requests anonymous standard-I/O pipes and how those
pipe endpoints are extracted from the command owner.

### 39.7 `threads.md`

Threads and processes may share lifecycle vocabulary such as `join`, `detach`,
`ID`, `Name`, `Status`, `Value`, `Observe()`, and `Platform` where meanings align.

They remain distinct execution and memory models.

Thread shared-memory happens-before rules must not be copied into process join.

### 39.8 `concurrency.md`

The concurrency overview must identify processes as isolated execution entities
and must not describe process communication as ordinary `Channel[T]` sharing.

### 39.9 Destruction and ownership books

The canonical ownership, copy/move, partial-move, borrowing, and destruction books
own the general rules that this process model relies on.

In particular:

- consumed named process-start values use explicit `<-`;
- failed process startup retains ownership on the failure path;
- `Take*Pipe()` avoids leaving `Command` in ordinary partial-move state;
- unresolved lifecycle owners may not silently be destroyed;
- `free` cannot propagate ordinary `Result` errors;
- remaining owned result/pipe resources participate in deterministic destruction.

---

## 40. Non-goals and forbidden shortcuts

A conforming implementation must not:

- lower `spawn process` to a task/thread because process support is difficult;
- treat ordinary `ref` as cross-process shared memory;
- treat numeric `RawPtr[T]` copying as cross-process pointer validity;
- silently consume a named move-only startup value without `<-`;
- discard a moved startup value when spawn fails before commit;
- expose a `Process[T]` before startup/bootstrap commit;
- leave a failed-spawn child running or unreaped;
- report `Completed` without an owner-side normal completion value;
- fabricate `PanicInfo` when panic information was not established;
- classify a normal non-zero command exit as abnormal termination;
- treat `Terminate()` success as equivalent to `join`;
- implicitly terminate a child on owner destruction;
- implicitly join a child on owner destruction;
- implicitly detach a forgotten child on owner destruction;
- drain stdout/stderr during `join` or lifecycle `Drain`;
- use direct aggregate partial move of command pipe endpoints in a way that makes
  lifecycle methods invalid;
- implement detach by dropping native resources and leaking zombie/native state;
- expose raw native lifecycle capability as portable ordinary process identity;
- turn RTOS tasks into Sec processes without the required isolation model;
- hide statically known unsupported target capabilities inside runtime errors;
- invent process-specific existing-file/native-handle transfer rules that bypass
  the canonical IPC/transferability model.

---

## 41. Summary of normative process model

The canonical Sec 0.1 process model is:

```text
Sec callable process
    spawn process Callable(...)
        -> Result[Process[T], ProcessSpawnError]

External executable process
    new Command(executable, arguments)
    Command configuration
    Command.Validate() optional preflight
    Command.Start()
        -> Result[void, CommandStartError]
```

A `Process[T]` is an eager, move-only, typed process lifecycle/result owner.

A `Command` is a move-only external-executable launch description that becomes a
process lifecycle/result owner after successful `Start()`.

Process startup is an explicit process-transfer boundary.

Existing named values consumed at that boundary require `<-`.

Ownership commits only on successful startup.

Ordinary parent and child Sec memory is isolated.

Normal process completion carries exactly one typed terminal completion value.

Ongoing inter-process communication belongs to IPC.

Process lifecycle is resolved explicitly with `join`, detach, ownership transfer,
or other canonical lifecycle operations.

`join` establishes terminal process synchronization and native reaping but does
not publish arbitrary child memory or drain standard-I/O pipes.

Detach relinquishes Sec owner-child lifecycle responsibility and allows independent
continued execution only on targets that provide the required process/detach
facilities.

Hard termination is an explicit safe process-control operation in the Sec
memory-safety sense, but it gives no child cleanup guarantee and does not replace
join.

Targets that cannot implement these semantics reject the corresponding source
operations at compile time.
