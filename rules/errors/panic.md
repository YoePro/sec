# Panic

## Status

This document is the canonical panic, assertion, checked-unreachable, and panic
containment rulebook for Sec.

It defines:

- what panic means;
- what panic is not;
- panic domains;
- containment;
- task and thread outcomes;
- root process behavior;
- defer and destructor behavior;
- assertions;
- checked unreachable;
- panic information;
- allocation-free panic reporting;
- no-panic verification;
- detached-task requirements;
- absence of a mandatory runtime.

This document must be read with:

```text
runtime_checks.md
defer.md
destruction.txt
spawn.md
await.md
select.md
cancellation.md
ownership.md
```

---

# No mandatory runtime

Panic support must not introduce a required general Sec runtime.

A panic endpoint may be:

```text
a compiler-emitted non-returning function
a user-provided function
a target support symbol
a scheduler hook when a scheduler is explicitly used
a direct target trap
a test harness boundary
```

A freestanding Sec program may define panic behavior without linking a Sec
runtime library.

A panic-free program may contain no panic endpoint at all after proof and dead
code elimination.

---

# Purpose

Panic represents a broken internal invariant or another explicitly
panic-selected failure path.

Panic is not the ordinary mechanism for expected errors.

Expected errors use:

```text
Result
Option where semantically appropriate
named unions
explicit status types
try
match
ordinary control flow
```

---

# Error versus panic

## Expected business error

Examples:

```text
customer not found
invoice already posted
credit limit exceeded
inventory unavailable
concurrent update
invalid user input
```

These return explicit values.

They are not panic.

## Expected technical error

Examples:

```text
file not found
disk full
allocation failure
timeout
connection failure
unsupported target feature
```

These require a panic-free result path.

They are not necessarily panic.

## Runtime safety failure

Examples:

```text
overflow
division by zero
bounds failure
contract failure
invalid shift
```

These have both:

```text
fallible path
panic-capable ordinary path
```

as defined by `runtime_checks.md`.

## Broken invariant

Examples:

```text
assertion failure
checked unreachable reached
corrupt internal state
impossible compiler-generated state reached
```

These are natural panic reasons.

## External catastrophe

Examples:

```text
power loss
hardware failure
operating-system termination
foreign memory corruption
external process kill
```

Sec cannot guarantee containment or reporting for all such events.

---

# Core panic rule

> Panic terminates the current panic domain and never resumes the failed stack.

Panic is not catch-and-resume exception handling.

No code continues at the panic site.

---

# No general panic catching

Sec does not provide a general user-level construct equivalent to:

```text
try/catch panic
recover and resume
catch all exceptions
```

A panic may be observed only at an explicit containment boundary.

Examples:

```text
awaiting a managed task
joining a managed thread
a declared supervisor
a test harness
the root panic endpoint
```

Containment observes termination.

It does not resume the failed execution.

---

# Panic domains

A panic domain is the execution unit terminated by panic.

Canonical domains:

```text
managed task
managed thread
root process domain
freestanding or bare-metal root domain
```

A function is not a panic domain.

Returning from a panic as an implicit hidden error would create exception-like
control flow and is not permitted.

---

# Managed task panic

A panic in a managed task:

1. stops the task;
2. runs permitted panic cleanup;
3. marks the task outcome as panicked;
4. records panic information;
5. notifies awaiter or supervisor;
6. does not automatically terminate the process;
7. never resumes the task stack.

Conceptual outcome:

```sec
type TaskOutcome[T] union {
    Completed(T)
    Cancelled
    Panicked(PanicInfo)
}
```

Exact standard type naming belongs to concurrency rulebooks.

---

# Managed thread panic

A panic in a managed Sec thread:

1. stops the thread;
2. runs permitted panic cleanup;
3. marks the join outcome as panicked;
4. notifies joiner or supervisor;
5. does not automatically terminate the process;
6. never resumes the failed thread.

A thread hosting indispensable scheduler or process infrastructure may be
declared noncontainable by its profile.

That is a target or execution-policy property.

It is not the default for ordinary managed threads.

---

# Root process panic

A panic in the root process domain terminates the process.

Before termination, the selected panic policy may:

```text
run permitted cleanup
write structured panic information
flush a preallocated panic sink
invoke a user hook
trigger a debugger trap
```

No normal continuation occurs.

---

# Bare-metal and freestanding panic

A root panic on a freestanding target transfers control to the configured
non-returning panic endpoint.

Possible policies:

```text
disable interrupts and halt
reset
enter safe state
write diagnostic register
signal watchdog
blink or signal hardware
trap to monitor
```

Sec does not prescribe one mandatory runtime behavior.

The handler is part of the target or application contract.

---

# Panic containment

Containment means:

```text
the failed domain terminates
another domain observes the failure
the process may continue
```

Containment does not mean:

```text
the failed operation returned an ordinary Result
the stack resumed
the invariant became valid
shared mutable state is automatically trustworthy
```

---

# No automatic escalation

A contained task or thread panic does not automatically escalate to the process.

A supervisor may explicitly choose:

```text
record
restart
disable work item
cancel siblings
escalate to parent
terminate process
```

Escalation is policy.

It is not the default language action.

---

# Mandatory observation

A contained panic may never disappear silently.

Every managed panic must reach one of:

```text
awaiter
joiner
supervisor
panic sink
test harness
root handler
```

---

# Detached tasks

A detached task has no ordinary awaiter.

A detached task must therefore have:

```text
a supervisor
or a declared panic sink
```

Detaching a task without any panic observation path is invalid.

The exact supervisor syntax belongs to concurrency rulebooks.

---

# Shared mutable state

Panic containment does not automatically preserve shared-state consistency.

Example:

```text
state mutation begins
some fields are updated
panic occurs
cleanup releases a lock
state remains partially updated
```

Programs requiring continued service should use:

```text
transactional updates
copy-then-commit
message passing
task-owned state
version validation
rollback
poisoning
```

---

# Poisoning

A synchronization or shared-state guard may be marked poisoned when panic occurs
during protected mutation.

A later acquisition may return:

```sec
type LockError union {
    Poisoned(PanicInfo)
}
```

Exact synchronization semantics belong to future lock and concurrency rules.

Poisoning is not mandatory for every lock.

The type or profile declares the policy.

---

# Panic cleanup

Panic cleanup may include:

```text
defer bodies
destructors
task-local cleanup
thread-local cleanup
containment bookkeeping
panic-sink notification
```

All panic cleanup must be noPanic.

---

# Defer is noPanic

Every defer body is implicitly transitively noPanic.

This applies during:

```text
normal return
early return
panic cleanup
cancellation cleanup where defer applies
```

A defer body that may panic is a compile-time error.

---

# Destructors are noPanic

Every destructor is transitively noPanic.

A destructor may not:

```text
assert dynamically
perform unhandled checked arithmetic
perform unhandled bounds access
call a panic-capable function
invoke unknown foreign code
use a panic-capable allocation shortcut
```

---

# Fallible cleanup

A cleanup operation may return an error.

The defer or destructor must handle it locally without panic.

```sec
defer {
    match resource.TryClose() {
        Ok() => {
        }

        Err(error) => {
            cleanupLog.Record(error)
        }
    }
}
```

The error-recording path must also be noPanic.

---

# Double panic

A panic during panic cleanup violates the noPanic cleanup contract.

If it nevertheless occurs because of:

```text
unsafe code
foreign code
compiler defect
hardware corruption
incorrect trusted annotation
```

the current panic domain terminates immediately through the minimal panic
endpoint.

No second unwinding sequence is started.

---

# Panic cleanup guarantee

For managed tasks and managed threads, the canonical containment policy performs
panic cleanup before reporting `Panicked`.

For a root process or bare-metal domain, the selected profile declares whether
cleanup is:

```text
full domain cleanup
limited cleanup
none
```

Portable code must not rely on root-domain cleanup after panic.

Critical application paths should use `@noPanic`.

---

# No-panic code

Conceptual:

```sec
@noPanic
fn ProcessInvoice(...) Result[void, InvoiceError] {
}
```

`@noPanic` is transitively verified by the compiler.

It guarantees:

```text
no language-defined panic
no panic-capable reachable call
no unproven assertion
no reachable checked unreachable
no unknown foreign abort path
```

It does not guarantee protection from external catastrophe.

---

# No-panic entrypoints

Build configuration may require:

```text
selected entrypoints are noPanic
all reachable program code is noPanic
all interrupt handlers are noPanic
all transaction handlers are noPanic
```

Exact manifest syntax belongs to build rules.

---

# Assertions

`assert` states an internal invariant.

Canonical conceptual form:

```sec
assert condition
```

An optional message or structured reason may be added by later grammar
synchronization.

---

# Assertion meaning

If the condition is true:

```text
execution continues
analysis may refine facts
the check may be optimized away
```

If the condition is false:

```text
panic current domain with AssertionFailed
```

---

# Assertions are always active

`assert` is not removed merely because the build is optimized.

Debug and release builds have the same assertion semantics.

The compiler may remove an assertion only when it proves the condition true.

---

# Assertions are not business validation

Invalid:

```sec
assert customer.Exists
```

when customer absence is expected input.

Invalid:

```sec
assert account.Balance >= amount
```

when insufficient funds is a normal business result.

Use explicit error handling.

---

# Assertions in no-panic code

An assertion is allowed in `@noPanic` code only when the compiler proves it
always true.

```sec
type NonNegative int range 0..int.Max

@noPanic
fn Use(value: NonNegative) int {
    assert value >= 0
    return value
}
```

The assertion may be eliminated.

An unproven assertion makes the function panic-capable.

---

# Assertion refinement

After a successful assertion, the compiler may use the condition for later
analysis.

```sec
assert index >= 0 && index < values.Length
let value := values[index]
```

The later bounds check may be removed.

---

# Assertion messages

If message syntax is provided:

```sec
assert condition, "message"
```

the message expression is evaluated only on failure.

A panic-critical or no-allocation profile may restrict messages to:

```text
static strings
stable panic codes
constant metadata
allocation-free formatting
```

Dynamic formatting must not be required for panic reporting.

---

# Debug assertions

A future separate form may exist:

```sec
debug assert condition
```

It is not required for Sec 0.1.

If introduced:

```text
it may be omitted by profile
its condition must be pure
it must have no observable side effects
it must not establish facts required for correctness when omitted
```

Ordinary `assert` remains always active.

---

# Assert versus assume

`assert` performs a checked validation.

It is not an unchecked optimizer promise.

Sec 0.1 does not expose a normal safe-language `assume`.

A future unchecked assumption would require:

```text
unsafe syntax
explicit proof responsibility
no panic semantics
clear invalid-behavior contract
```

It must remain distinct from `assert`.

---

# Checked unreachable

Canonical:

```sec
unreachable
```

It means:

> Control flow must not reach this point. If it does, panic the current domain.

It is a checked control-flow assertion.

---

# Unreachable behavior

If the compiler proves the statement unreachable:

```text
no code is emitted
no panic effect remains
```

If reachability remains possible:

```text
the statement is panic-capable
reaching it panics with UnreachableReached
```

---

# Unreachable in no-panic code

`unreachable` is valid in `@noPanic` code only when the compiler proves the
statement cannot execute.

Otherwise the function is not noPanic.

---

# Unreachable is not optimizer undefined behavior

Sec 0.1 does not expose an unchecked user-level unreachable promise.

The compiler may lower checked unreachable conceptually as:

```text
call selected non-returning panic endpoint
backend unreachable
```

The backend `unreachable` appears only after the defined non-returning Sec panic
path.

---

# Compiler-proven dead code

Code proven unreachable after:

```text
return
break
continue
panic
unreachable
exhaustive control flow
```

is a compile-time error according to Sec diagnostics policy.

The existence of an `unreachable` statement does not permit arbitrary dead code
after it.

---

# Explicit panic

The lexer reserves `panic`.

Canonical explicit panic syntax and payload shape remain to be finalized.

The semantic requirements are fixed:

```text
panic terminates the current domain
panic never returns
panic is visible in effect analysis
panic is forbidden in noPanic code
panic information has an allocation-free minimum representation
```

Illustrative only:

```sec
panic InvariantError.InvalidLedgerState
```

Do not lock exact source syntax from this example alone.

---

# Panic reason IDs

Every implicit language panic has a stable reason ID.

Examples:

```text
panic.arithmetic-overflow
panic.division-by-zero
panic.invalid-shift
panic.bounds
panic.contract
panic.assertion-failed
panic.unreachable-reached
panic.explicit
panic.invalid-reference-generation
panic.foreign-abort
```

Final IDs belong to the diagnostics and panic registry.

---

# PanicInfo

Conceptual form:

```sec
type PanicInfo struct {
    ID: PanicID
    File: string
    Line: uint
    Column: uint
    Function: string
}
```

Optional profile data may include:

```text
message
operation
type
task ID
thread ID
source expression
call stack
related panic
```

The minimum representation must not require dynamic allocation.

---

# Allocation-free panic path

Panic may occur during allocation failure or memory pressure.

The minimum panic path may not require:

```text
heap allocation
dynamic string concatenation
growing collection
symbolization
filesystem access
network access
locking a potentially poisoned allocator
```

A profile may add richer reporting only when resources are available.

---

# Panic sink

A panic sink receives structured panic information.

It must be:

```text
noPanic
non-returning for root panic
or bounded and noPanic for contained reporting
allocation-free in the minimum path
```

A sink may:

```text
write to a preopened descriptor
write to a fixed ring buffer
emit a target debug instruction
store in static memory
notify a supervisor
```

---

# Root panic endpoint

The root endpoint does not return.

Conceptual signature:

```sec
unsafe extern "Sec" fn PanicRoot(info: ref PanicInfo) never
```

This signature is illustrative.

The final `never` surface syntax is not required by this rulebook.

The endpoint may instead be compiler-known.

---

# Contained panic reporting

A task or thread panic endpoint may return control to supervisor machinery, not
to the failed stack.

Conceptual sequence:

```text
failed-domain cleanup
construct bounded PanicInfo
mark outcome Panicked
wake awaiter or joiner
destroy domain control block when safe
scheduler continues
```

This requires scheduler support only when managed tasks or threads are used.

It does not impose scheduler support on programs that do not use them.

---

# No mandatory scheduler

A program using no managed tasks or threads requires no scheduler.

Task and thread panic containment belongs to those features' support code.

It is not a global Sec runtime requirement.

---

# Foreign panic and unwind

Foreign exceptions or unwinding may not cross Sec frames by default.

FFI declarations must classify foreign behavior.

Possible effects:

```text
no unwind
may abort
may return error
may unwind to foreign boundary
```

Unknown foreign behavior is not noPanic.

---

# Unsafe panic violations

Unsafe code may break panic guarantees only through an explicitly trusted
boundary.

A false trusted annotation is a program defect.

The compiler may report:

```text
panic guarantee depends on trusted foreign or unsafe promise
```

---

# Task outcome

A task outcome distinguishes panic from ordinary errors returned by the task.

Given:

```sec
fn Worker() Result[Value, WorkerError]
```

completion is conceptually:

```sec
TaskOutcome[Result[Value, WorkerError]]
```

Possible outcomes:

```text
Completed(Ok(value))
Completed(Err(workerError))
Cancelled
Panicked(panicInfo)
```

A returned `Err` is not a panic.

---

# Thread outcome

Joining a thread follows the same distinction:

```text
normal returned value
normal returned error value
cancelled where supported
panicked
```

Exact naming belongs to thread rules.

---

# Supervisor policy

A supervisor may configure reactions.

Conceptual policies:

```text
Report
Restart
RestartWithLimit
Disable
CancelGroup
Escalate
TerminateProcess
```

No policy is implied by panic beyond reporting and containment.

---

# Panic and cancellation

Cancellation is not panic.

Cancellation uses separate explicit control flow and cleanup rules.

A supervisor may cancel sibling tasks after a panic.

That is policy, not equivalence.

---

# Panic and Result

Panic is not automatically converted to `Err`.

A containment boundary reports `Panicked(PanicInfo)` as execution outcome.

It does not pretend that the function returned its declared error type.

---

# Panic and transactions

Critical transaction paths should be noPanic.

```sec
@noPanic
fn PostInvoice(...) Result[PostedInvoice, PostInvoiceError] {
}
```

Rollback should be explicit or performed through verified noPanic cleanup.

Panic containment is an isolation boundary, not ordinary transaction control
flow.

---

# Panic and logging

Panic reporting should not depend on ordinary application logging being healthy.

The minimum panic sink should be independent where possible.

Application logging may receive a copy after containment.

---

# Panic and testing

A test harness may be a declared containment boundary.

Tests may assert that an operation panics with a stable panic ID.

The harness observes termination of the test domain.

It does not create general production catch-and-resume behavior.

---

# Panic strategy and portability

A profile may choose root behavior such as:

```text
cleanup then terminate
immediate terminate
trap
reset
```

Portable application logic must not depend on root panic cleanup.

Managed task and thread containment remains explicit when those features are
used.

---

# Panic strategy configuration

Illustrative build choices:

```text
panic = "forbid"
panic = "process-terminate"
panic = "trap"
panic = "custom"
```

Task and thread containment are feature policies at their own domain levels.

Exact manifest syntax is not locked here.

---

# Runtime-free implementation examples

## Hosted process without tasks

Checks branch directly to a user or compiler-provided non-returning endpoint.

No scheduler or Sec runtime is required.

## Bare metal

Checks branch to a target trap or application panic handler.

No allocator, unwinder, or runtime library is required.

## Managed tasks

The explicitly used task implementation supplies domain bookkeeping and
containment.

Programs not using tasks do not link it.

## Panic-free program

All reachable panic paths are proven absent.

No panic endpoint need remain in the binary.

---

# Diagnostics

Suggested diagnostic families:

```text
panic.assert-used-for-expected-error
panic.assert-not-proven-in-no-panic
panic.unreachable-not-proven
panic.explicit-in-no-panic
panic.defer-may-panic
panic.destructor-may-panic
panic.detached-without-supervisor
panic.unknown-foreign-effect
panic.root-handler-may-return
panic.sink-may-panic
panic.cleanup-may-panic
panic.unobserved-contained-panic
panic.shared-state-may-be-poisoned
```

---

# Tests

Required files:

```text
panic_valid.sec
panic_invalid.sec
panic_assert_valid.sec
panic_assert_invalid.sec
panic_unreachable_valid.sec
panic_unreachable_invalid.sec
panic_no_panic_valid.sec
panic_no_panic_invalid.sec
panic_task_containment_valid.sec
panic_task_containment_invalid.sec
panic_thread_containment_valid.sec
panic_thread_containment_invalid.sec
panic_defer_valid.sec
panic_defer_invalid.sec
panic_destructor_valid.sec
panic_destructor_invalid.sec
```

---

# Behavior tests

Verify:

```text
assert true continues
assert false terminates current domain
proven assert removed
unproven assert rejected in noPanic
proven unreachable removed
reachable unreachable panics
task panic reported as Panicked
thread panic reported as Panicked
contained panic does not kill process
root panic terminates process
detached panic reaches sink
defer runs in managed-domain cleanup
defer cannot panic
destructor cannot panic
double panic uses immediate minimal endpoint
```

---

# Binary dependency tests

Verify:

```text
panic-free binary links no panic support
bare-metal panic uses target handler or trap
hosted root panic needs no general runtime
task containment support links only when tasks are used
thread containment support links only when managed threads are used
panic minimum path allocates nothing
```

---

# Required synchronization

This document must remain synchronized with:

```text
runtime_checks.md
attributes.md
grammar.md
operators.md
defer.md
destruction.txt
ownership.md
copy_move.md
spawn.md
await.md
select.md
cancellation.md
threads.md
unsafe.md
platform/ffi.md
allocation.txt
diagnostics.txt
lsp.md
formatter.md
compiler_pipeline.txt
semantic_ir.txt
build rules
language-rulebook-status.md
rules_implementations.txt
```

---

# Appendix A — Codex implementation plan

## A.1 Add rulebook

Add:

```text
rules/errors/panic.md
```

Update status and implementation trackers.

## A.2 Panic effect

Add compiler-visible effects:

```text
NoPanic
MayPanic
```

Retain detailed internal causes.

## A.3 Panic domains

Represent:

```text
TaskDomain
ThreadDomain
RootProcessDomain
FreestandingRootDomain
```

## A.4 Assertions

Add parser and AST support once grammar is locked.

Sema:

```text
type-check bool condition
attempt proof
add refinement on success path
mark panic effect if unproven
reject in noPanic if unproven
```

## A.5 Unreachable

Add checked unreachable statement.

Prove where possible.

Otherwise lower through a defined panic endpoint before backend unreachable.

## A.6 Cleanup verification

Mark defer bodies and destructors implicit noPanic.

Reject complete call-chain causes.

## A.7 PanicInfo

Define an allocation-free minimum representation.

Use static source metadata where possible.

## A.8 Root endpoint

Support:

```text
target trap
user handler
compiler helper
```

Verify non-returning behavior.

## A.9 Task and thread containment

Extend control blocks with panic outcomes.

Do not convert panic into the function's declared Result.

Wake awaiters and joiners.

## A.10 Detached observation

Reject detached managed work without supervisor or panic sink.

## A.11 Double panic

Install an immediate minimal path for trusted-boundary violations during cleanup.

## A.12 No-runtime verification

Add link and object-file tests proving:

```text
no general runtime dependency
support linked only when feature used
panic-free binary removes endpoint
```

## A.13 Tooling

LSP hover should show:

```text
noPanic
may panic
panic causes
containment domain
```

Call hierarchy should reveal panic paths.

---

# Design summary

Panic is not ordinary error handling.

Expected failures use explicit values and `try`.

Panic terminates the current panic domain and never resumes the failed stack.

Managed task and thread panic is contained and reported without automatic
process escalation.

Root process panic terminates the process.

Bare-metal panic uses a configured non-returning target or application handler.

Every panic must be observed.

Detached panic requires a supervisor or sink.

Defer bodies and destructors are always noPanic.

Assertions are always active internal-invariant checks.

Unproven assertions are forbidden in noPanic code.

`unreachable` is always checked and never an unchecked optimizer promise in Sec
0.1.

Panic information has stable IDs and an allocation-free minimum path.

Panic support does not require a general Sec runtime.

Programs using no managed concurrency do not link scheduler support.

Panic-free programs may link no panic support at all.
