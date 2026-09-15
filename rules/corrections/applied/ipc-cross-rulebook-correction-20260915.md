# IPC Cross-Rulebook Correction

- **Status:** Applied
- **Created:** 2026-09-15
- **Last updated:** 2026-09-15
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical applied path:** `rules/corrections/applied/ipc-cross-rulebook-correction-20260915.md`
- **Primary normative owner introduced:** `rules/concurrency/ipc.md`
- **Implementation governance:** `governance/concurrency_ipc.yaml`
- **Governance tags:** `concurrency.ipc-v1`, `governance.rulebook-sync`, `analysis.ipc-v1`, `platform.ipc-v1`

---

## 1. Purpose

This correction synchronizes existing canonical Sec rulebooks and governance with the normative IPC model in `rules/concurrency/ipc.md`.

It applies the already locked Sec 0.1 decisions for:

```text
pipes
typed IPCSender[T]/IPCReceiver[T]
shared memory
process-shared synchronization
SharedValue[T]
IPCAtomic[T]
capability/resource transfer
Command direct standard-I/O resource binding
IPC select/blocking/cancellation integration
process-shared memory ordering and analysis
target capability planning
```

This correction does not make `rules/concurrency/ipc.md` a duplicate owner of existing process, channel, atomic, memory-order, or File semantics.

Instead, each existing owner is synchronized at the exact integration boundary described below.

The applied correction is archived at:

```text
rules/corrections/applied/ipc-cross-rulebook-correction-20260915.md
```

or the actual application date if applied later.

---

## 2. Correction authority

After application, canonical ownership is:

```text
rules/concurrency/ipc.md
    IPC-owned types and semantics

rules/concurrency/processes.md
    Process[T], Command, process lifecycle, process standard-I/O source surface

rules/memory/transferability.md
    ProcessTransferable semantic eligibility

rules/concurrency/channels.md
    ordinary in-process Channel[T]

rules/concurrency/select.md
    generic select syntax/readiness/commit semantics

rules/concurrency/blocking.md
    generic waiting/blocking/suspension rules

rules/concurrency/atomics.md
    Atomic[T], MemoryOrder, CompareExchangeResult[T], Swap and atomic order matrix

rules/concurrency/concurrency_memory_model.md
    acquire/release/SeqCst/happens-before semantics

rules/library/stdlib.md and canonical io.File declarations
    File public API and implementation contract
```

---

# Part I — `rules/concurrency/processes.md`

## 3. Revision requirement

Increment the process rulebook revision from `2.0` to the next revision and update its date.

The process rulebook must reference `rules/concurrency/ipc.md` as the canonical owner of:

```text
Pipe
PipeReader
PipeWriter
capability/resource transfer
existing-resource process standard-I/O binding
```

The process rulebook remains the owner of `Command` itself.

---

## 4. Replace `CommandInputMode`

Replace:

```sec
enum CommandInputMode {
    Inherit
    Closed
    Pipe
}
```

with:

```sec
enum CommandInputMode {
    Inherit
    Closed
    Pipe
    Resource
}
```

Add the normative meaning:

```text
Resource
    The caller supplied an existing owned standard-input resource through one of
    the canonical Command resource-binding overloads. Command owns that resource
    after successful configuration commit until Start() resolves its child-side
    binding or later deterministic cleanup releases the retained configuration.
```

`Resource` is ordinary Sec resource ownership. It is not a raw native handle mode.

---

## 5. Replace `CommandOutputMode`

Replace:

```sec
enum CommandOutputMode {
    Inherit
    Discard
    Pipe
}
```

with:

```sec
enum CommandOutputMode {
    Inherit
    Discard
    Pipe
    Resource
}
```

Add the corresponding normative meaning for existing caller-supplied stdout/stderr resources.

---

## 6. Extend `CommandConfigurationError`

Replace the canonical declaration with:

```sec
enum CommandConfigurationError error {
    InvalidState
    InvalidWorkingDirectory
    NotFound
    NotDirectory
    PermissionDenied
    InvalidEnvironmentName
    InvalidEnvironmentValue
    InvalidIOResource
    OutOfMemory
}
```

Add:

### `InvalidIOResource`

The supplied `File`, `PipeReader`, or `PipeWriter` cannot satisfy the requested standard-stream direction or canonical portable process-I/O contract.

Examples include a `File` without the required read/write authority.

On `InvalidIOResource`:

```text
Command configuration remains unchanged
supplied consuming source ownership does not commit
```

Pipe direction mismatches that cannot overload-resolve are compile-time errors rather than runtime `InvalidIOResource`.

---

## 7. Extend the exact `Command` public surface

Keep the existing mode setters:

```sec
fn SetStdin(mode: CommandInputMode) Result[void, CommandConfigurationError]
fn SetStdout(mode: CommandOutputMode) Result[void, CommandConfigurationError]
fn SetStderr(mode: CommandOutputMode) Result[void, CommandConfigurationError]
```

Add these exact overloads:

```sec
fn SetStdin(file: File) Result[void, CommandConfigurationError]
fn SetStdin(pipe: PipeReader) Result[void, CommandConfigurationError]

fn SetStdout(file: File) Result[void, CommandConfigurationError]
fn SetStdout(pipe: PipeWriter) Result[void, CommandConfigurationError]

fn SetStderr(file: File) Result[void, CommandConfigurationError]
fn SetStderr(pipe: PipeWriter) Result[void, CommandConfigurationError]
```

These overloads are valid only in `ProcessStatus.Created`, like the existing configuration setters.

---

## 8. Resource setter ownership

A resource setter consumes its resource argument when configuration commit succeeds.

Existing named move-only values therefore use ordinary explicit move syntax:

```sec
try command.SetStdout(<-logFile)
```

For:

```sec
let result := command.SetStdout(<-logFile)
```

the canonical ownership rule is:

```text
Ok()
    Command owns logFile's resource capability
    source binding is unavailable
    StdoutMode == CommandOutputMode.Resource

Err(...)
    previous Command configuration is unchanged
    source binding remains available
```

This is a conditional ownership commit, not a call-entry unconditional move.

The same rule applies to all six resource overloads.

---

## 9. Retaining a parent resource

Where the parent also needs access, the caller explicitly duplicates the resource before configuring the child:

```sec
let childLog := try logFile.Duplicate()
try command.SetStdout(<-childLog)
```

`Duplicate()` is an explicit resource operation and is never implicit copy.

---

## 10. Resource reconfiguration

Replacing a previously configured Resource stream is transactional.

If the new configuration commits successfully, the previously Command-owned resource is destroyed/released according to its canonical resource contract.

If the new configuration fails, the previous Resource configuration remains unchanged and ownership of the new source remains with the caller.

Changing Resource mode to `Inherit`, `Closed`, `Discard`, or `Pipe` follows the same transactionality rule.

---

## 11. Resource-mode `Start()`

For a Resource-configured stream, `Start()` must:

1. prepare a child-side capability binding using the IPC route-specific target adapter;
2. keep the Command-owned source configuration intact until process-start commit;
3. commit the child standard-stream capability together with successful process bootstrap;
4. after successful commit, release any parent-side configuration capability that is no longer semantically required;
5. avoid retaining a hidden extra capability that changes pipe EOF, File lifetime, or resource-count behavior.

If `Start()` fails before commit:

```text
Command returns/remains Created
configuration remains intact
Command retains the configured Resource capability
retry remains possible
temporary child/native capability state is rolled back
```

A semantically valid Resource configuration that fails during actual child-native setup returns the existing:

```sec
CommandStartError.IOSetupFailed
```

when no more specific start error applies.

---

## 12. `Take*Pipe()` with Resource mode

Preserve the exact existing extraction methods:

```sec
fn TakeStdinPipe() Option[PipeWriter]
fn TakeStdoutPipe() Option[PipeReader]
fn TakeStderrPipe() Option[PipeReader]
```

Add the normative rule:

```text
When the corresponding stream mode is Resource, Take*Pipe() returns None because
Command did not create a new parent-side anonymous pipe endpoint.
```

---

## 13. Portable resource scope

Document that Sec 0.1 portable direct resource bindings are exactly:

```text
stdin:
    File
    PipeReader

stdout:
    File
    PipeWriter

stderr:
    File
    PipeWriter
```

Do not add generic raw handles or socket overloads to the portable `Command` API in this correction.

---

## 14. Resolve the previous existing-resource IPC dependency

Replace the current unresolved process text that says existing File/socket/native binding waits on future `ipc.md` with the resolved rule:

```text
Direct existing-resource standard-I/O binding is defined by the Command Resource
mode and the canonical type-preserving capability-transfer rules in rules/concurrency/ipc.md.
Portable Sec 0.1 supports File and the direction-correct Pipe endpoint types.
Sockets and arbitrary raw native handles remain outside the portable 0.1 surface.
```

This closes the previous process D-I dependency.

---

## 15. Replace process standard-I/O capability terminology

Replace canonical CompilationPlan facts:

```text
ProcessStandardIOInheritance
ProcessPipes
```

with semantic facts:

```text
ProcessStandardIOPipes
ProcessStandardIOResourceBinding(T, role)
```

where the internal `role` distinguishes:

```text
stdin
stdout
stderr
```

Do not expose native inheritance as a source-semantic target capability.

Backend inheritance, descriptor duplication, `DuplicateHandle`, handle lists, or equivalent native setup are realization strategies only.

---

## 16. Process pipe capability requirements

Document that `CommandInputMode.Pipe` / `CommandOutputMode.Pipe` require the selected CompilationPlan to provide:

```text
ExternalProcessExecution
IPCPipes
ProcessStandardIOPipes
```

plus ordinary target/runtime requirements for the process start.

---

## 17. Process Resource capability requirements

Each direct binding requires:

```text
ExternalProcessExecution
ProcessStandardIOResourceBinding(T, role)
```

for the concrete resource type and stream role.

Semantic `ProcessTransferable(T)` alone does not prove that this particular route is supported by the selected target.

---

## 18. Process analysis update

Extend process analysis text so that direct standard-I/O resource configuration participates in:

```text
conditional ownership commit
rollback
capability identity tracking
process-start bootstrap cleanup
route-specific target validation
```

Process join remains forbidden from secretly draining pipe output.

The new Resource mode does not change that rule.

---

# Part II — `rules/memory/transferability.md`

## 19. Preserve `ProcessTransferable` ownership

`transferability.md` remains the sole canonical owner of compiler-derived `ProcessTransferable` eligibility.

Do not create a second IPC-local `ProcessTransferable` definition.

---

## 20. Clarify semantic eligibility versus route support

Extend the process-boundary section to state explicitly:

```text
ProcessTransferable proves that a value/capability has a semantically valid
process-transfer contract.

A selected target must additionally provide a route-specific adapter for the
actual boundary operation.
```

At minimum the target model may distinguish:

```text
CapabilityDuplicate(T)
ProcessStartupTransfer(T)
IPCTypedTransfer(T)
ProcessStandardIOResourceBinding(T, role)
```

A type can therefore be `ProcessTransferable` while one particular route is unsupported on the selected target.

---

## 21. Clarify process capability transfer

Extend the process-boundary rules to state that a resource-owning capability may preserve logical resource identity while receiving a different native representation in the destination process.

Copying a numeric native descriptor/handle representation is never by itself a valid process transfer adapter.

---

## 22. Clarify transactional aggregate transfer

Add an explicit cross-reference to `ipc.md`:

```text
A typed IPC transfer of a composite value containing multiple owned capabilities
commits ownership as one source-semantic operation. Pre-commit failure preserves
the complete source value and requires rollback of prepared destination state.
```

This is an application of the existing transfer-validation-before-commit rule, not a new ownership model.

---

## 23. Channel section correction

Where `transferability.md` refers to future IPC channels/adapters, replace that vague wording with:

```text
Ordinary Channel[T] remains in-process. Typed process messaging is owned by
rules/concurrency/ipc.md through IPCSender[T]/IPCReceiver[T] and is a separate process
boundary operation.
```

---

# Part III — `rules/concurrency/channels.md`

## 24. Remove future automatic process-channel ambiguity

Replace language equivalent to:

```text
Separate processes do not communicate through ordinary Channel[T] unless a
future IPC adapter explicitly defines that mapping.
```

with the canonical Sec 0.1 rule:

```text
Ordinary Channel[T], Sender[T], and Receiver[T] are strictly in-process
concurrency capabilities. They never cross a process isolation boundary and are
never automatically lowered to IPC.

Typed process messaging uses IPCSender[T] and IPCReceiver[T] as defined by
rules/concurrency/ipc.md.
```

A future explicit bridge, if ever standardized, must remain a distinct adapter abstraction and must not retroactively make ordinary `Channel[T]` process-transferable.

---

## 25. Preserve ordinary channel semantics

Do not copy IPC clean-vs-abnormal process termination, schema identity, materialization, or capability-transfer semantics into ordinary channels.

Their existing in-process send/receive/capacity/ticket/lifetime model remains owned by `channels.md`.

---

# Part IV — `rules/concurrency/select.md`

## 26. Replace planned IPC wording with canonical selectable IPC operations

The current section that describes IPC receive/process readiness as possible future work must be synchronized.

The following IPC operations are primitive selectable operations in Sec 0.1:

```text
PipeReader.Read
PipeWriter.Write
IPCSender[T].Send
IPCReceiver[T].Receive
IPCMutex.Lock
IPCSemaphore.Acquire
```

Also retain the already canonical process join/observer integration from `processes.md`.

---

## 27. IPC select readiness/commit

Add the generic rule that IPC operation readiness is non-destructive.

A non-selected branch must not:

```text
consume pipe bytes
write pipe bytes
commit a typed message
transfer message ownership
remove/materialize a typed message
acquire an IPCMutex
produce IPCMutexGuard
consume an IPCSemaphore permit
```

Only the selected branch may commit its operation.

---

## 28. Composed pipe operations

Explicitly state that:

```text
PipeReader.ReadExact
PipeWriter.WriteAll
```

are composed operations and are not primitive selectable operations in Sec 0.1.

They may partially progress before a later wait when called ordinarily, which makes them unsuitable as primitive single-commit select candidates.

---

## 29. Moved IPC send in select

Add the canonical ownership example:

```sec
select {
    sender.Send(<-message) => {
        // ownership commits only if this selected Send returns Ok()
    }

    after 1s => {
        // message remains available
    }
}
```

If the Send branch is selected but returns `Err`, ownership remains available because semantic send commit did not occur.

---

## 30. IPC synchronization select

For:

```sec
select {
    guard := mutex.Lock() => {
        UseProtectedState()
    }

    semaphore.Acquire() => {
        ConsumePermit()
    }
}
```

only the selected branch may acquire its resource.

A non-selected mutex branch creates no guard.

A non-selected semaphore branch consumes no permit.

---

# Part V — `rules/concurrency/blocking.md`

## 31. Extend the potentially waiting operation inventory

Add:

```text
PipeReader.Read
PipeReader.ReadExact
PipeWriter.Write
PipeWriter.WriteAll
IPCSender[T].Send
IPCReceiver[T].Receive
IPCMutex.Lock
IPCSemaphore.Acquire
SharedValue[T].Lock
```

These operations follow the existing Task-suspend versus physical-Thread block/park policy.

---

## 32. Extend guard rules

Apply the existing no-live-guard-across-wait rule to:

```text
IPCMutexGuard
SharedValueGuard[T]
```

in addition to ordinary `MutexGuard[T]`.

The rule is based on actual liveness.

Destroying the guard before the wait makes later waiting legal.

---

## 33. IPC cancellation commit

Add:

```text
Cancellation may stop an IPC wait only before the operation's semantic commit.
Before commit, queue/ownership/lock/permit state is unchanged.
After commit, the operation completes to its canonical result and cancellation
cannot revoke the committed effect.
```

Do not add `Cancelled` variants to IPC error enums.

---

## 34. Extend deadlock dependencies

The generic blocking/deadlock interaction must include:

```text
IPCMutex identities
IPCSemaphore waits
typed IPC Send/Receive dependencies
pipe backpressure
process join/wait
```

A zero-capacity typed IPC transport makes `Send()` a possible rendezvous wait.

The existing parent-join/child-pipe-output deadlock pattern remains canonical and must not be repaired by hidden pipe draining.

---

# Part VI — `rules/concurrency/concurrency_memory_model.md`

## 35. Add process-shared synchronization identities

Clarify that canonical synchronization identities may be shared across process isolation domains only through an explicitly process-shared abstraction defined by `ipc.md`.

Ordinary process-local storage is not shared merely because two processes originated from one spawn relationship.

---

## 36. IPC mutex synchronization

Add the normative edge:

```text
normal release of IPCMutex identity M
    synchronizes with
later successful acquisition of the same logical M
```

using the same release/acquire publication model as the canonical mutex memory-order rules, but across valid process-shared storage.

---

## 37. IPC semaphore synchronization

Add:

```text
IPCSemaphore.Release()
    publishes with release semantics to the permit relation

successful Acquire() consuming that permit
    acquires the corresponding publication
```

Do not infer synchronization merely because the same semaphore type exists; the relation is through the same logical semaphore/permit flow.

---

## 38. IPC atomic synchronization

State that `IPCAtomic[T]` reuses the exact canonical atomic semantics of:

```text
MemoryOrder
per-atomic modification order
Load
Store
Swap
CompareExchange
release sequences
SeqCst ordering
```

but its valid synchronization scope includes all processes/executions holding capabilities to the same logical IPC atomic identity.

The backend must not use a scope narrower than required by that logical identity.

---

## 39. Process join is not generic shared-memory publication

Preserve and make explicit:

```text
Process join proves process completion/quiescence and reaping according to
processes.md, but it does not by itself publish arbitrary SharedMemory contents.
```

Shared-memory visibility is established through the synchronization mechanisms defined by IPC/memory-model rules.

---

## 40. Raw shared memory and races

Add an explicit note:

```text
unsafe SharedMemory.View does not disable the Sec data-race rule. Concurrent
conflicting raw shared-memory access remains invalid unless the program's
explicit synchronization protocol establishes the required ordering/exclusion.
```

---

# Part VII — `rules/concurrency/atomics.md`

## 41. No duplicate atomic vocabulary

Do not introduce a new ordinary atomic result type or replacement-operation name for IPC.

`ipc.md` must consume the current canonical ordinary declarations:

```sec
enum MemoryOrder {
    Relaxed
    Acquire
    Release
    AcqRel
    SeqCst
}

enum CompareExchangeResult[T] {
    Exchanged
    NotExchanged(T)
}
```

and the canonical operation name:

```text
Swap
```

rather than a second `Exchange` operation.

This correction therefore requires no replacement of the ordinary atomic public surface.

---

## 42. Add IPC cross-reference

Add a short authority note to `atomics.md`:

```text
rules/concurrency/ipc.md defines IPCAtomic[T], the lock-free process-shared capability form.
IPCAtomic[T] reuses MemoryOrder, CompareExchangeResult[T], Swap naming, and the
canonical operation/order validity model from this rulebook. IPC-specific type
eligibility, process-shared target capability, duplication, and lifecycle remain
owned by ipc.md.
```

---

## 43. Compare-exchange ordering authority

Where IPC material or implementation notes describe compare-exchange success/failure order validity, the final canonical validity is the matrix in `atomics.md` and `concurrency_memory_model.md`.

Do not add a conflicting second IPC-only weaker-than-success ordering rule.

This is a harmonization requirement: ordinary atomic ownership of the common order model takes precedence over provisional design examples that used a stricter table during IPC discussion.

---

# Part VIII — `rules/library/stdlib.md` and `io.File`

## 44. Add exact `File.Duplicate()` requirement

The canonical `io.File` surface must include:

```sec
fn Duplicate() Result[File, IOError]
```

`File` remains `@noCopy`; ordinary copy must never duplicate a file resource.

---

## 45. `File.Duplicate()` semantics

`Duplicate()` creates a second independently owned `File` capability referring to the same underlying open resource.

It is not:

```text
reopen by path
copy file contents
clone independent file-position state
```

Any underlying open-resource state that the native duplication contract shares, including file-position state where applicable, remains shared accordingly.

The duplicate preserves or reduces authority and must never silently increase access rights.

---

## 46. `File.Duplicate()` failure

Failure returns canonical `IOError` and leaves the original `File` unchanged and available.

Temporary native resources produced before failure must be released.

Static target incapability to support the required File capability operation for a particular route is diagnosed through CompilationPlan rather than disguised as a runtime generic IPC handle conversion.

---

## 47. Close-on-exec remains default

The existing high-level File rule that ordinary opened files are close-on-exec remains correct.

Direct child standard-I/O resource binding must explicitly establish only the selected child capability during `Command.Start()`.

Do not weaken File's general close-on-exec policy or bulk-inherit unrelated files merely to implement Resource mode.

---

# Part IX — `rules/concurrency/concurrency.md`

## 48. IPC overview

Update the umbrella concurrency rulebook to recognize the canonical distinction:

```text
Channel[T]
    in-process task/thread communication

IPCSender[T]/IPCReceiver[T]
    typed process-isolation message transport

Pipe
    byte-stream IPC

SharedMemory / SharedValue / IPCAtomic
    explicit process-shared storage forms
```

Do not describe IPC as unfinished/planned after `ipc.md` is committed.

---

# Part X — `language-rulebook-status.md`

## 49. IPC row

Replace:

```text
| `ipc.md` | Planned | ... |
```

with a Written entry equivalent to:

```text
| `ipc.md` | **Written** | Revision 1.0 defines canonical pipes, typed IPC,
shared memory/mappings, IPCMutex/IPCSemaphore, SharedValue[T], IPCAtomic[T],
capability/resource transfer, Command resource binding, target capabilities,
analysis, lowering, security, and conformance obligations. Implementation is
tracked by `concurrency.ipc-v1` in `governance/concurrency_ipc.yaml`. |
```

---

## 50. Remove IPC from deferred areas

Remove `IPC` from the deferred-area list.

IPC is no longer an unwritten or deferred design area after `rules/concurrency/ipc.md` is committed.

This documentation-status change does not claim compiler/runtime implementation.

---

## 51. Process row

Update the process row to note that direct existing-resource standard-I/O binding is now normatively resolved through IPC Resource mode/capability transfer.

The process implementation may remain `planned` in governance.

---

## 52. Channel row

If `channels.md` remains `Written — sync required` for unrelated reasons, do not incorrectly mark all channel synchronization complete solely because this IPC correction was applied.

The IPC-specific change is only the strict in-process/process-boundary clarification.

---

# Part XI — Governance

## 53. Add the IPC governance fragment

Create:

```text
governance/concurrency_ipc.yaml
```

using the separately delivered canonical fragment.

Its primary integration ID is:

```text
concurrency.ipc-v1
```

Do not place this new integration in root `implementation-status.yaml`.

---

## 54. Register the fragment

Add to `governance/index.yaml`:

```yaml
  - path: concurrency_ipc.yaml
    owns: inter-process communication, process-shared storage/synchronization, and capability-transfer semantics
```

Place it with the other `concurrency_*` fragments.

Keep `index.yaml` as registry/ownership metadata only; do not copy integration status text into it.

---

## 55. Update `governance/concurrency_process.yaml`

Keep `concurrency.processes-v2` owned by the process fragment.

Within that existing integration:

1. update the summary to include Resource-mode direct standard-I/O binding through the canonical IPC capability model;
2. replace the remaining item for `CommandInputMode Inherit/Closed/Pipe` and `CommandOutputMode Inherit/Discard/Pipe` with the four-variant Resource forms;
3. add the six direct resource-binding overload implementation requirements;
4. add `InvalidIOResource`;
5. replace `ProcessStandardIOInheritance` / `ProcessPipes` with `ProcessStandardIOPipes` and `ProcessStandardIOResourceBinding(T, role)`;
6. remove the remaining item that says File/socket/native standard-I/O binding must be resolved before completion, because the portable File/Pipe scope is now resolved;
7. retain sockets/raw handles outside the portable 0.1 scope rather than recording them as an unresolved IPC design dependency;
8. update the governance reconciliation line that currently says root `implementation-status.yaml` remains mutable implementation state so it instead references the split governance ledger as canonical;
9. add this correction to `corrections_applied` after application.

Do not duplicate the `concurrency.ipc-v1` integration entry into the process fragment.

---

## 56. Adjacent governance fragments

Do not copy the entire IPC integration into:

```text
concurrency_atomics.yaml
concurrency_blocking.yaml
concurrency_channels.yaml
concurrency_memory_model.yaml
concurrency_select.yaml
analysis.yaml
lowering.yaml
memory.yaml
platform.yaml
stdlib.yaml
```

Those semantic owners may receive changes to their pre-existing integrations when implementation work occurs, but `concurrency.ipc-v1` has exactly one canonical governance owner.

---

# Part XII — Analysis and Semantic IR synchronization

## 57. `semantic_ir.md`

Add/extend the canonical semantic distinctions needed by IPC:

```text
logical IPC resource identity
capability owner identity
shared backing identity
synchronization identity
route-specific transfer adapter
prepared transfer/materialization state
semantic commit
rollback cleanup
readiness without commit
process-shared atomic order/scope
```

Exact internal opcode names remain implementation-defined.

The semantic distinctions are normative.

---

## 58. Conditional ownership commit

Ensure Semantic IR can represent a consuming value that remains available on operation failure.

This applies to at least:

```text
IPCSender.Send(<-value)
Command.SetStdin/Stdout/Stderr(<-resource)
process startup transfer
composite capability transfer
```

`try` must preserve cleanup of the still-owned failure-path source.

---

## 59. Deadlock analysis

Ensure the canonical deadlock analysis can consume IPC facts for:

```text
IPCMutex ownership/wait
IPCSemaphore waits
IPCSender.Send wait
IPCReceiver.Receive wait
pipe backpressure
process join/wait
```

Cross-process cycles are within scope.

---

## 60. Data-race analysis

Ensure the canonical data-race analysis can consume:

```text
shared backing identity
SharedValue synchronization identity
IPCMutex/IPCSemaphore synchronization
IPCAtomic MemoryOrder communication
unsafe shared mapping access
```

Process-local values materialized by ordinary IPC transfer are not automatically shared aliases.

---

# Part XIII — Diagnostics synchronization

## 61. Diagnostic categories

The compiler/LSP must distinguish at least:

```text
T is not ProcessTransferable
T is ProcessTransferable but this target route adapter is missing
T is not SharedStorable
T is not IPCAtomicValue
T is IPCAtomicValue but target width/profile is unsupported
existing named consuming IPC value requires <-
IPCMutexGuard/SharedValueGuard crosses wait
recursive IPCMutex acquisition
invalid IPC atomic MemoryOrder
malformed runtime typed transfer
```

Stated examples in `ipc.md` are guidance for mentor-style wording; stable diagnostic IDs remain owned by diagnostics governance/rulebooks.

---

# Part XIV — Test synchronization

## 62. Required test suites

Applying this correction requires the conformance groups from `rules/concurrency/ipc.md` to be represented in compiler/runtime/platform tests:

```text
pipes
typed IPC
shared memory
IPCMutex/IPCSemaphore
SharedValue
IPCAtomic
capability transfer
Command Resource mode
select/cancellation
process death
target capability resolution
malformed transport/resource limits
```

A target must not claim a canonical IPC family merely because the native OS exposes a similarly named primitive.

The complete Sec semantics must be verified.

---

# Part XV — Application checklist

## 63. Files to add

```text
rules/concurrency/ipc.md
governance/concurrency_ipc.yaml
rules/corrections/applied/ipc-cross-rulebook-correction-20260915.md
```

After correction application, move the correction to the canonical `applied/` path.

---

## 64. Files requiring normative synchronization

At minimum:

```text
rules/concurrency/processes.md
rules/memory/transferability.md
rules/concurrency/channels.md
rules/concurrency/select.md
rules/concurrency/blocking.md
rules/concurrency/concurrency_memory_model.md
rules/concurrency/atomics.md
rules/library/stdlib.md
rules/concurrency/concurrency.md
rules/compiler/semantic_ir.md
language-rulebook-status.md
```

Additional implementation files may change without becoming new normative owners.

---

## 65. Governance files requiring synchronization

```text
governance/index.yaml
governance/concurrency_process.yaml
```

Add `governance/concurrency_ipc.yaml` as the one canonical IPC integration owner.

Do not add IPC work to root `implementation-status.yaml`.

---

## 66. Atomics harmonization gate

Before marking this correction applied, verify that `rules/concurrency/ipc.md` and implementation code use exactly:

```text
Swap
CompareExchangeResult[T]
MemoryOrder
```

from the current canonical atomics rulebook.

Do not introduce:

```text
Exchange
AtomicCompareExchangeResult[T]
IPCMemoryOrder
```

as parallel public APIs.

---

## 67. Process D-I closure gate

After application, no process/governance document may continue to describe existing File/Pipe standard-I/O binding as an unresolved IPC design question.

The portable Sec 0.1 answer is complete:

```text
Command Resource mode
File / direction-correct Pipe endpoint overloads
type-preserving IPC capability transfer
route-specific target validation
transactional configuration/start ownership
```

Sockets and arbitrary raw handles remain intentionally outside the portable 0.1 direct-binding scope.

---

## 68. Status gate

`rules/concurrency/ipc.md` may be documented as **Written** while `concurrency.ipc-v1` remains `planned`.

Documentation completion and implementation completion are separate governance dimensions.

No file should imply that a written IPC rulebook means compiler/runtime/platform support is already implemented.
