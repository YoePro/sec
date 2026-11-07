# Blocking

- **Status:** Normative
- **Created:** 2026-07-29
- **Last updated:** 2026-11-07 (stable chapter and paragraph numbering)
- **Document revision:** 1
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/blocking.md`
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`
- **Implementation governance:** `governance/concurrency_blocking.yaml` (`frontend.blocking-effects`)
- **Related rulebooks:** `rules/concurrency/await.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/scheduling.md`, `rules/concurrency/mutex.md`, `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/platform/ffi.md`, `rules/platform/interrupts.md`, `rules/errors/runtime_checks.md`

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

**§ 2(1)** In task context, a wait should suspend the logical task when the backend can
register readiness.

**§ 2(2)** In physical thread context, a wait parks or blocks the current physical thread.

**§ 2(3)** Parking or blocking a physical thread does not by itself invalidate a
thread-local borrow when execution resumes on the same physical-thread
attachment generation. Logical task suspension remains different because the
task may resume on another physical thread.

**§ 2(4)** Busy waiting repeatedly polls without parking or suspension.

**§ 2(5)** Busy waiting is not the default lowering for a Sec blocking operation.

---

## § 3 Potentially blocking operations

**§ 3(1)** The following may wait:

```text
await Task[T]
join Task[T]
join Thread[T]
join Process[T]
join Command
ProcessObserver.Wait()
Channel.Send
Receiver.Receive
PipeReader.Read
PipeReader.ReadExact
PipeWriter.Write
PipeWriter.WriteAll
IPCSender[T].Send
IPCReceiver[T].Receive
IPCMutex.Lock
IPCSemaphore.Acquire
SharedValue[T].Lock
select without ready branch
Mutex.Lock
timer wait
I/O readiness or completion
platform wait operations
```

**§ 3(2)** `Try...` operations are nonblocking when their rule explicitly guarantees it.

**§ 3(3)** A `default` branch makes `select` nonblocking.

---

## § 4 Await

**§ 4(1)** `await task` is task-specific.

**§ 4(2)** It consumes the owning `Task[T]`.

**§ 4(3)** In a task backend it normally suspends the current task.

**§ 4(4)** A profile may block a physical worker only when:

- the profile explicitly permits it;
- no suspension backend exists;
- deadlock and executor-starvation rules are still enforced.

**§ 4(5)** If caller cancellation commits before await commit, the await dependency does
not instantly disappear. The operation retains the consumed task lifecycle,
requests cooperative cancellation, and may continue waiting for the child to
become terminal before caller cancellation completes. A backend must not model
this cleanup as implicit detach; physical blocking or suspension remains
profile-controlled.

---

## § 5 Join

**§ 5(1)** `join handle` waits for completion while preserving terminal handle information.

**§ 5(2)** Caller context determines the physical wait mechanism.

**§ 5(3)** Task joining a thread:

```text
suspend current task where supported
```

**§ 5(4)** Physical thread joining a thread:

```text
park or block current physical thread
```

**§ 5(5)** A successful join establishes the completion synchronization edge defined by the
concurrency memory model.

**§ 5(6)** For `Process[T]` and `Command`, successful join additionally proves child
quiescence, stabilizes terminal metadata and allowed completion payloads, and
collects/reaps native terminal state where required. It does not publish
arbitrary child addresses or ordinary child storage; explicit shared-memory IPC
retains its own visibility contract.

**§ 5(7)** `ProcessObserver.Wait()` is repeatable, non-owning terminal observation. It
does not perform owner join, reaping, detachment, termination, or terminal
payload transfer. From a Task these waits should suspend when the backend can
register completion without blocking the worker; from a physical Thread they
may park/block. Blocking an executor worker requires explicit profile support.

---

## § 6 Channel operations

**§ 6(1)** Blocking channel send and receive preserve the same commit semantics in task and
thread contexts.

**§ 6(2)** Task context may suspend.

**§ 6(3)** Thread context may block.

**§ 6(4)** Cancellation or timeout must not partially commit the message.

**§ 6(5)** Only the selected successful operation transfers ownership.

---

## § 7 Select

**§ 7(1)** A waiting `select` registers all candidate operations and commits exactly one
ready branch.

**§ 7(2)** In task context, it suspends the task.

**§ 7(3)** In thread context, it parks or blocks the thread.

**§ 7(4)** A non-selected branch must not:

- consume a value;
- move a message;
- take a task result;
- change channel state;
- retain ownership after registration is cancelled.

---

## § 8 Mutex acquisition

**§ 8(1)** `Mutex[T].Lock()` may wait.

**§ 8(2)** In task context the backend should use task-aware suspension where available.

**§ 8(3)** In thread context it blocks or parks.

**§ 8(4)** The mutex's ownership and memory-order semantics are identical.

**§ 8(5)** A runtime must not substitute a task-local lock when the mutex may cross a thread
boundary.

**§ 8(6)** Logical task suspension, physical executor-worker blocking, and direct
physical-thread parking are distinct scheduling facts. Task-context operations
use logical suspension where the selected target supports it; this must not
fabricate `TaskContext` for ordinary `main` or physical-thread-only code.

---

## § 9 Mutex guards across waits

**§ 9(1)** `MutexGuard[T]`, `IPCMutexGuard`, and `SharedValueGuard[T]` are execution-bound
and may not remain live across:

- `await`;
- `join`;
- blocking channel operations;
- waiting `select`;
- task suspension;
- `Task.Yield()`;
- transfer to another task or thread.

**§ 9(2)** The compiler should apply the same conservative rule to physical thread waits
unless a later lock policy explicitly permits a safe blocking wait while holding
that guard.

**§ 9(3)** This prevents:

- executor deadlock;
- lock-order inversion;
- task migration with thread-owned lock state;
- hidden long critical sections.

---

## § 10 Cancellation-aware waits

**§ 10(1)** A cancellation-aware wait may resume because cancellation was requested.

**§ 10(2)** For IPC, cancellation may stop a wait only before semantic commit. Before
commit, queue, ownership, lock, and permit state is unchanged. After commit, the
operation completes to its canonical result and cancellation cannot revoke the
effect. IPC error enums therefore do not gain `Cancelled` variants.

**§ 10(3)** The operation must then follow its own commit rule.

**§ 10(4)** For example, a channel send that has not committed retains ownership of the
message.

**§ 10(5)** Cancellation is not permitted to create a partial operation.

**§ 10(6)** The resumed execution entity performs ordinary cleanup before becoming
`Cancelled`.

---

## § 11 Timeouts

**§ 11(1)** Timeouts are expressed through `select` and `after` unless a specific API rule
defines another timeout form.

**§ 11(2)** Example:

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

**§ 11(3)** A timeout branch must not commit another branch.

**§ 11(4)** A timeout is not cancellation unless user code explicitly requests or performs
cancellation.

---

## § 12 Nonblocking operations

**§ 12(1)** An operation documented as nonblocking must complete immediately with an
explicit state.

**§ 12(2)** Examples include conceptual:

```text
TryLock
TrySend
TryReceive
status observation
```

**§ 12(3)** Nonblocking does not mean successful.

**§ 12(4)** It means the operation returns without waiting.

---

## § 13 Busy waiting

**§ 13(1)** The compiler and backend must not lower ordinary waits to unbounded busy loops
unless the target profile explicitly selects a busy-wait implementation.

**§ 13(2)** A busy-wait profile must document:

- CPU consumption;
- memory ordering;
- interrupt interaction;
- cancellation checks;
- power implications;
- maximum intended duration.

**§ 13(3)** Compiler-generated busy loops must contain the required atomic or volatile
semantics.

---

## § 14 Blocking effects

**§ 14(1)** Functions may be classified by effects such as:

```text
mayBlock
noBlock
maySuspend
noSuspend
```

**§ 14(2)** The existing attributes include:

```sec
@noBlock
```

**§ 14(3)** A function marked `@noBlock` must not directly or transitively call an operation
that may physically block on the selected target.

**§ 14(4)** Task suspension and physical blocking are separate effects.

**§ 14(5)** A function may be nonblocking for a physical thread while still permitting task
suspension only when its contract says so explicitly.

**§ 14(6)** Process operations are classified by their selected implementation effects.
This includes `spawn process`, `Command.Start()`, `Command.Validate()`,
`Command.SetWorkingDirectory()`, process/command `Terminate()`, owning
process or command join, and `ProcessObserver.Wait()`. Validation and
working-directory configuration may perform filesystem/native permission
checks and are not assumed to be pure in-memory operations.

---

## § 15 Call graph analysis

**§ 15(1)** The compiler should use the complete call graph to determine:

- whether a function may block;
- whether a function may suspend;
- which target profiles change lowering;
- whether an ISR reaches blocking code;
- whether a detached execution entity has a finite shutdown path;
- whether a mutex guard crosses a wait;
- whether a cooperative executor can lose all workers to blocking calls.

**§ 15(2)** Indirect calls must carry conservative effect information.

---

## § 16 FFI

**§ 16(1)** An extern function is potentially blocking unless its declaration or imported
contract states otherwise.

**§ 16(2)** Example conceptual metadata:

```sec
@noBlock
unsafe extern "C" fn NativeTryRead(...) int
```

**§ 16(3)** Incorrect external effect declarations are unsafe contract violations.

**§ 16(4)** The compiler may trust them for optimization and diagnostics but cannot verify
foreign implementation behavior.

---

## § 17 ISR context

**§ 17(1)** An interrupt routine must not:

- await;
- join;
- block on channels;
- wait in `select`;
- acquire an ordinary blocking mutex;
- call an unannotated potentially blocking extern function;
- sleep;
- yield.

**§ 17(2)** Process creation, command start/validation/working-directory checks, hard
termination, process/command join, and `ProcessObserver.Wait()` are likewise
not ISR-safe in Sec 0.1. A narrower immutable metadata read still requires the
ordinary interrupt-safety proof.

**§ 17(3)** ISR-safe operations must be bounded and explicitly declared.

---

## § 18 Main and initialization

**§ 18(1)** Blocking during initialization is profile-dependent.

**§ 18(2)** An embedded profile may forbid indefinite waits during `@init`.

**§ 18(3)** A hosted profile may allow them.

**§ 18(4)** Whole-program analysis should report initialization cycles where one initializer
waits for work that cannot start until initialization finishes.

---

## § 19 Deadlock interaction

**§ 19(1)** Blocking operations contribute edges to deadlock analysis.

**§ 19(2)** The compiler should track:

- held mutex identities;
- joined execution entities;
- awaited tasks;
- channel dependencies;
- IPC mutex identities;
- IPC semaphore waits;
- typed IPC send/receive dependencies;
- pipe backpressure;
- process join/wait dependencies;
- deferred thread start;
- executor worker consumption;
- cancellation dependencies.

**§ 19(3)** Statically proven deadlocks are compiler errors.

**§ 19(4)** Potential cycles may be warnings or configurable diagnostics.

---

## § 20 Memory ordering

**§ 20(1)** A wait operation establishes memory ordering only when its own rule defines a
synchronization edge.

**§ 20(2)** Examples:

- successful join;
- successful await;
- mutex lock after unlock;
- channel receive after matching send;
- committed atomic wait/wake where specified.

**§ 20(3)** Sleeping, yielding or timing out does not by itself publish ordinary memory.

---

## § 21 Runtime errors

**§ 21(1)** This rulebook introduces no independent runtime `BlockingError`.

**§ 21(2)** Operation-specific failures belong to their owning APIs and all language-level
runtime error types must be declared in:

```text
core/errors.sec
```

**§ 21(3)** Compiler errors such as blocking in ISR or violating `@noBlock` remain
diagnostics, not core runtime errors.

---

## § 22 Semantic IR

**§ 22(1)** Semantic IR must distinguish:

```text
TaskSuspend
TaskResume
ThreadPark
ThreadWake
BlockingJoin
SuspendingJoin
BlockingChannelWait
SuspendingChannelWait
BlockingMutexWait
SuspendingMutexWait
SelectRegister
SelectSuspend
SelectPark
SelectCommit
TimeoutRegister
CancellationWake
```

**§ 22(2)** The IR must record whether a wait may physically block under the selected
profile.

---

## § 23 Diagnostics

**§ 23(1)** Examples:

```text
blocking operation is not permitted in @noBlock function Poll
```

```text
interrupt routine UartIRQ may call blocking function Read
```

```text
MutexGuard[State] cannot remain live across join
```

```text
all executor workers may block waiting for tasks scheduled on the same executor
```

```text
busy-wait lowering is not permitted by target profile
```

**§ 23(2)** Diagnostics must have stable IDs.

---

## § 24 Required synchronization

**§ 24(1)** Cross-check and update:

```text
await.md
threads.md
tasks.md
channels.md
select.md
mutex.md
scheduling.md
cancellation.md
deadlock_analysis.md
data_races.md
concurrency.md
concurrency_runtime_model.md
concurrency_memory_model.md
platform/ffi.md
compiler_analysis.md
semantic_ir.md
diagnostics.md
core/errors.sec
```
