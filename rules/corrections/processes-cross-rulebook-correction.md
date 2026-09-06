# Process v2 cross-rulebook synchronization correction

- **Status:** Pending normative correction
- **Created:** 2026-09-06
- **Last updated:** 2026-09-06
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `0f5027d`
- **Intended path:** `rules/corrections/processes-cross-rulebook-correction.md`
- **Primary authority:** `rules/concurrency/processes.md`
- **Affected canonical books:** `rules/concurrency/spawn.md`, `rules/concurrency/select.md`, `rules/concurrency/blocking.md`, `rules/concurrency/concurrency.md`, `language-rulebook-status.md`, `implementation-status.yaml`

---

## 1. Purpose and authority

This correction synchronizes rulebooks that predate the normative Sec 0.1 process model in `rules/concurrency/processes.md` revision 2.0.

It does **not** redefine process semantics. Where this correction discusses process creation, process transferability, process lifecycle, process completion, process observation, process target support, or process-specific synchronization, `rules/concurrency/processes.md` remains the canonical owner.

The affected books must stop treating process spawning and process completion as postponed, unnamed, or placeholder behavior.

This correction remains in `rules/corrections/` until all changes below have been merged into their owning canonical books. It may then be moved to `rules/corrections/applied/` according to the correction governance rules.

---

## 2. Correction to `rules/concurrency/spawn.md`

### 2.1 Canonical result type

All text that leaves the result of `spawn process` undefined, delegated to `processes.txt`, or described only as a future process handle must be replaced by the canonical type rule:

```sec
spawn process Work()  // Result[Process[T], ProcessSpawnError]
```

where `T` is the complete declared normal return type of `Work`.

Examples:

```sec
fn Calculate() int {
    return 42
}

let worker := try spawn process Calculate()
// worker: Process[int]
```

and:

```sec
fn Load() Result[Document, IOError] {
    // ...
}

let worker := try spawn process Load()
// worker: Process[Result[Document, IOError]]
```

`spawn.md` must not collapse a callable-returned `Result[T, E]` into process failure.

### 2.2 Execution-kind table

The source-facing spawn result table must use the concrete process type:

```sec
spawn Work()          // task form defined by the task rulebooks
spawn task Work()     // task form defined by the task rulebooks
spawn thread Work()   // Result[Thread[T], ThreadSpawnError]
spawn process Work()  // Result[Process[T], ProcessSpawnError]
```

Any stale wording that lists `Task[T]`, `Thread[T]`, and an unnamed "process handle" must be removed.

The exact task result form must remain owned by the current task/spawn rulebooks and must not be changed by this correction beyond any already-canonical task synchronization work.

### 2.3 Eager process creation

Generic wording that says spawned execution is eager must be scoped so that it remains correct for each execution kind.

For the process form, the canonical rule is:

```text
spawn process Callable(...)
```

is eager and returns no user-visible unstarted `Process[T]` state. A successful result means that process identity, child bootstrap, startup transfer commit, callable entry, completion transport, and lifecycle ownership have been established according to `processes.md`.

`spawn.md` must not introduce `ProcessConfig`, deferred process creation, or `Process.Start()` semantics.

### 2.4 Process arguments, captures, and receivers

Task/thread borrowing rules must not be applied mechanically to `spawn process`.

For process spawn:

- ordinary `ref T` and `ref mut T` do not cross the process boundary;
- a numeric `RawPtr[T]` does not become a valid child pointer merely by transfer;
- explicit arguments, captures, and instance-method receiver state must satisfy the canonical `ProcessTransferable` proof/adapter rules owned by `rules/memory/transferability.md`;
- process transfer creates or establishes child-valid state rather than a hidden shared ordinary reference to parent storage;
- an instance-method receiver is a process startup input; the method implementation is compiler/linker materialization and `self` in the child refers to the transferred child-local receiver state.

### 2.5 Explicit move marker at process boundaries

When an existing named source binding is consumed by process startup, the call site must contain the ordinary Sec move marker `<-`.

Valid consuming form:

```sec
let payload := BuildPayload()
let worker := try spawn process Run(<-payload)
```

Invalid when `payload` must be consumed:

```sec
let payload := BuildPayload()
let worker := try spawn process Run(payload)
```

The compiler must diagnose the missing move marker rather than silently consuming the binding.

The same rule applies to a consumed named method receiver or captured value.

Fresh temporaries do not require a synthetic move marker merely to indicate ownership that has never been bound to a reusable identifier.

### 2.6 Transactional startup ownership

Process startup is a fallible ownership boundary.

For an existing named value explicitly transferred with `<-`:

```text
Ok(Process[T])
    ownership commits and the source binding becomes unavailable

Err(ProcessSpawnError)
    startup ownership does not commit and the source value remains owned on
    the failure path for ordinary cleanup/recovery
```

`spawn.md` must not describe process move semantics in a way that loses the source value merely because evaluation reached the `<-` expression.

### 2.7 Semantic IR requirements

The spawn Semantic IR must preserve at least:

```text
execution kind: Process
callable / concrete process entry
result type: Process[T]
creation error type: ProcessSpawnError
process-transfer adapters for arguments, captures, and receiver
startup transfer prepare/commit/rollback facts
source location
```

The process form must not retain ordinary cross-process borrow facts and must not lower to task/thread execution.

### 2.8 Diagnostics

`spawn.md` must reference the process-specific diagnostics owned by `processes.md`, including at least:

```text
target profile does not support process execution
process spawn consumes '<name>'; write <-<name> to transfer ownership
value of type '<type>' cannot cross the process boundary
```

Runtime creation failures after semantic validity are represented by `ProcessSpawnError`, not by compile-time diagnostics.

### 2.9 References

Every canonical reference to:

```text
rules/concurrency/processes.txt
```

must become:

```text
rules/concurrency/processes.md
```

---

## 3. Correction to `rules/concurrency/select.md`

### 3.1 Process completion is no longer provisional

Any statement that process completion "may later" become selectable or is awaiting the process rulebook must be removed.

Sec 0.1 has two canonical process-completion select forms.

### 3.2 Owning process join branch

An owning process may participate through `join`:

```sec
select {
    join worker => {
        HandleCompletion(worker.Status)
    }

    message := receiver.Receive() => {
        HandleMessage(message)
    }
}
```

The process join branch is ready when the process has reached a terminal state and the join operation can commit without additional waiting.

If selected, the branch performs the same one-shot lifecycle operation as ordinary:

```sec
join worker
```

Selection therefore:

- consumes the one join capability;
- establishes process lifecycle/completion synchronization;
- collects/reaps native terminal state where required;
- releases join-only native resources;
- unlocks the terminal payload allowed by the resulting `ProcessStatus`;
- preserves the resolved process owner for terminal inspection and unconsumed normal result ownership.

If the branch is not selected, it must not consume or mutate the process join capability, reap the child, detach it, or take terminal payload ownership.

### 3.3 Non-owning observer wait branch

A `ProcessObserver` may participate through its canonical wait operation:

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

The branch is ready when the observed process has reached a terminal `ProcessStatus`.

Selecting this branch performs only non-owning terminal observation. It does not join, reap owner lifecycle state, detach, terminate, or unlock the owner's `Value`, `CompletionError`, `Panic`, or `Termination` payload.

`ProcessObserver.Wait()` is repeatable. Once terminal status is retained, later waits are immediately ready with the same terminal status.

### 3.4 Generic select ownership remains unchanged

`select.md` remains the canonical owner of:

- readiness probing;
- source-order and fairness rules;
- commit-after-selection semantics;
- non-selected branch preservation;
- binding behavior inside selected branches.

`processes.md` owns only the process-specific readiness and commit meaning of process join and process observer wait.

---

## 4. Correction to `rules/concurrency/blocking.md`

### 4.1 Exact process wait names

Generic references such as:

```text
join Process
```

must be made type-correct:

```text
join Process[T]
join Command
ProcessObserver.Wait()
```

where the operation is applicable.

### 4.2 Process join synchronization

Successful process join establishes:

- terminal process lifecycle synchronization;
- proof that the joined child can perform no further execution;
- stable owner-side process terminal metadata;
- canonical completion transfer/result availability according to status;
- native completion collection/reaping where required.

It does **not** establish a generic shared-memory publication rule for arbitrary child addresses or ordinary child storage.

Explicit shared-memory IPC retains its own visibility/order/synchronization contract.

### 4.3 Task versus physical-thread waiting

For both owning process join and `ProcessObserver.Wait()`:

- from a Task, the backend should suspend the Task when the selected process backend can register completion without blocking the executor worker;
- from a physical Thread, the implementation may park/block that Thread;
- physical executor-worker blocking is permitted only where the selected runtime/profile explicitly allows it.

### 4.4 Process operations in effect/blocking analysis

The existing blocking/effect analysis must classify the process operations whose selected implementation may wait, perform filesystem/native I/O, allocate, or invoke process control.

At minimum this includes where applicable:

```text
spawn process
Command.Start()
Command.Validate()
Command.SetWorkingDirectory()
Process[T].Terminate()
Command.Terminate()
ProcessObserver.Wait()
join Process[T]
join Command
```

`Command.Validate()` is not a pure in-memory validation because it may perform executable/filesystem/permission preflight.

`Command.SetWorkingDirectory()` is not a pure in-memory setter because the process rules require eager existence, directory-kind, and permission validation at setter time.

### 4.5 Guards and ISR restrictions

Existing rules that prohibit a live synchronization guard across potentially waiting operations apply to process joins and observer waits without a process-specific exception.

The following process operations are not ISR-safe in Sec 0.1 unless a more specific canonical platform rule proves a strictly narrower read-only operation safe:

```text
spawn process
Command.Start()
Command.Validate()
Command.SetWorkingDirectory()
Process[T].Terminate()
Command.Terminate()
ProcessObserver.Wait()
join Process[T]
join Command
```

Immutable bounded metadata observation such as reading `ProcessObserver.Status` remains subject to the interrupt rulebook's ordinary proof requirements.

---

## 5. Correction to `rules/concurrency/concurrency.md`

### 5.1 Canonical process rulebook

Every reference to the process rulebook must use:

```text
rules/concurrency/processes.md
```

and must stop describing that rulebook as an unfinished placeholder.

### 5.2 Process overview

The concurrency overview must summarize, without duplicating the full process rulebook, that Sec 0.1 defines:

```sec
spawn process Work(...)  // Result[Process[T], ProcessSpawnError]
```

for Sec-callable isolated subprocesses, and `Command` for external executable launch.

It must state that:

- Task, Thread, Process, and Command lifecycle objects are distinct abstractions;
- process execution has a distinct process-isolation domain and explicit process-transfer boundary;
- ordinary `ref`/`ref mut` relationships do not cross that boundary;
- general process communication belongs to `ipc.md` rather than ordinary in-process `Channel[T]`;
- process lifecycle, completion, join, detach, termination, observer, reaping, and process-specific synchronization belong to `processes.md`.

### 5.3 Cancellation independence

The overview must not imply that Task or Thread cancellation implicitly terminates or detaches an owned process.

Process lifecycle responsibility must be resolved explicitly according to `processes.md`.

### 5.4 Detached process lifetime

The overview may state that detach removes the Sec owner-child lifetime dependency on targets that provide canonical process detach support. Normal termination of the former Sec owner does not itself terminate that detached child. External platform, container, service-manager, session, or system policy remains outside this guarantee.

---

## 6. Correction to `language-rulebook-status.md`

### 6.1 Canonical process row

Replace the placeholder row for:

```text
concurrency/processes.txt
```

with a row for:

```text
concurrency/processes.md
```

whose status is `Written` and whose note identifies revision 2.0 as the normative Sec 0.1 process model covering `Process[T]`, `Command`, process lifecycle/completion, process standard I/O, target capabilities, analysis, and lowering obligations.

The note must not claim compiler/runtime implementation merely because the rulebook is written.

### 6.2 IPC row

`ipc.md` remains `Planned`, but its note must no longer say process communication is postponed with process spawning.

It should instead identify `ipc.md` as the planned canonical owner of:

```text
PipeReader / PipeWriter
general inter-process messaging
shared memory
process-aware synchronization
process capability/handle transfer
existing-resource Command standard-I/O binding
```

### 6.3 Sync-required books

Where the status format supports it, mark at least these books as requiring synchronization with process v2 until this correction is applied:

```text
concurrency/spawn.md
concurrency/select.md
concurrency/blocking.md
concurrency/concurrency.md
errors/panic.md
```

The panic synchronization is separately specified by `panic-info-compiler-known-correction.md`.

---

## 7. Correction to `implementation-status.yaml`

### 7.1 Mutable implementation state

Add the process integration described by:

```text
implementation-status-processes.yaml
```

without marking any compiler/runtime work implemented solely because `processes.md` has been written.

The integration must keep its concrete parser/Sema/core/runtime/IR/lowering/target/test work in `remaining` until corresponding implementation evidence exists.

### 7.2 Pending corrections

The process integration must record both pending correction paths:

```text
rules/corrections/processes-cross-rulebook-correction.md
rules/corrections/panic-info-compiler-known-correction.md
```

### 7.3 No duplicate IPC ownership

The implementation ledger must not assign ownership of generic pipes, shared memory, or general process capability transfer to the process integration. Those remain owned by the future IPC integration.

---

## 8. Revision metadata changes

When this correction is applied:

- `rules/concurrency/concurrency.md` must increment from revision 2.0 to revision 2.1 and set `Last updated` to `2026-09-06` or the actual later application date;
- `spawn.md`, `select.md`, and `blocking.md` must use the repository's current normative metadata format when next synchronized; if they do not currently carry revision metadata, do not fabricate historical revisions solely to apply this correction;
- `language-rulebook-status.md` and `implementation-status.yaml` must update their mutable update metadata according to their owning governance rules.

---

## 9. Required consistency tests

The synchronized repository must verify at least:

```text
spawn process Work() has raw type Result[Process[T], ProcessSpawnError]
process spawn does not inherit ordinary task borrow rules
missing <- on a consumed named process startup value is diagnosed
failed process startup preserves pre-commit source ownership
select join on Process[T] commits a real one-shot join only when selected
ProcessObserver.Wait() is selectable and non-owning
process join in blocking.md does not claim arbitrary child-memory publication
concurrency.md no longer describes processes.md as unfinished
no canonical rulebook references rules/concurrency/processes.txt
language-rulebook-status.md lists concurrency/processes.md as Written
implementation-status.yaml keeps process implementation work planned until evidence exists
```

---

## 10. Completion condition

This correction is complete only when every affected canonical book and mutable status ledger has been synchronized and no stale normative text remains that:

- leaves `spawn process` result unnamed;
- allows task-style ordinary borrows across the process boundary;
- treats process completion in `select` as future work;
- gives process join a thread-like arbitrary shared-memory publication meaning;
- describes `processes.txt` as the canonical process book;
- claims process/IPC implementation merely from rulebook completion.
