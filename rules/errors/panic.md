# Panic

- Status: Normative
- Created: 2026-09-01
- Last updated: 2026-09-01
- Document revision: 2.0
- Sec language version: 0.1
- Canonical path: `rules/errors/panic.md`
- Replaces: previous revision of `rules/errors/panic.md`
- Repository baseline reviewed: `814a584`

---

## § 1 Purpose and authority

**§ 1(1)** This rulebook defines panic, assertions, checked `unreachable`, panic effects, containment, panic reporting, and no-panic verification in Sec.

**§ 1(2)** Panic represents a broken internal invariant or another failure path whose canonical rule explicitly selects panic semantics.

**§ 1(3)** Panic is not the ordinary mechanism for expected business or technical errors.

**§ 1(4)** Expected failures use explicit values such as `Result`, `Option` where semantically appropriate, named unions, status types, `try`, `match`, and ordinary control flow.

**§ 1(5)** `rules/errors/runtime_checks.md` owns the panic-versus-fallible behavior of checked arithmetic, bounds, contracts, shifts, and other runtime safety checks.

**§ 1(6)** `rules/errors/errorhandling.md` owns `Result`, `Option`, `try`, error propagation, and typed recoverable failures.

**§ 1(7)** `rules/control-flow/defer.md` owns defer syntax, registration, and ordinary cleanup ordering.

**§ 1(8)** `rules/memory/destruction.md` owns automatic destruction and lifecycle cleanup.

**§ 1(9)** `rules/platform/interrupts.md` owns ISR execution-context restrictions, including Sec 0.1 `noPanic` requirements.

**§ 1(10)** Concurrency rulebooks own task/thread construction, awaiting/joining, cancellation, supervision, and execution-domain mechanics; this rulebook owns the meaning of panic when such domains exist.

**§ 1(11)** Target/build rulebooks own exact panic endpoint configuration and platform integration.

**§ 1(12)** This revision locks the Sec 0.1 assertion syntax defined in § 15.

---

## § 2 No mandatory runtime

**§ 2(1)** Panic support must not introduce a mandatory general Sec runtime.

**§ 2(2)** A panic endpoint may be implemented as:

```text
compiler-emitted non-returning function
user-provided function
target support symbol
scheduler hook when a scheduler is explicitly used
direct target trap
test-harness boundary
```

**§ 2(3)** A freestanding Sec program may define panic behavior without linking a general Sec runtime library.

**§ 2(4)** A program proven panic-free may contain no panic endpoint after semantic proof, reachability analysis, lowering, dead-code elimination, and linking.

**§ 2(5)** Panic reporting must have an allocation-free minimum path.

**§ 2(6)** Managed task/thread panic containment may require feature-specific support only when those features are used.

---

## § 3 Error versus panic

### § 3.1 Expected failures

**§ 3.1(1)** Expected business outcomes are ordinary values and are not panic.

Examples include customer absence, insufficient credit, unavailable inventory, concurrent update, and invalid user input.

**§ 3.1(2)** Expected technical failures such as file-not-found, disk-full, allocation failure, timeout, and connection failure are not automatically panic.

**§ 3.1(3)** Where a failure is expected to be handled, the canonical API must provide a panic-free recoverable path.

### § 3.2 Runtime safety failure

**§ 3.2(1)** Runtime safety failures include checked arithmetic overflow, division by zero, invalid shift, bounds failure, and contract failure.

**§ 3.2(2)** Their ordinary and fallible forms are defined by `runtime_checks.md`.

### § 3.3 Broken invariant

**§ 3.3(1)** Assertion failure and reached checked `unreachable` are canonical panic reasons.

**§ 3.3(2)** Corrupt internal state and impossible compiler-generated states may also select panic when their owning rule defines that behavior.

### § 3.4 External catastrophe

**§ 3.4(1)** Power loss, hardware failure, operating-system termination, foreign memory corruption, and external process kill are not guaranteed to obey Sec containment or panic-reporting semantics.

---

## § 4 Core panic rule

**§ 4(1)** Panic terminates the current panic domain and never resumes the failed stack.

**§ 4(2)** Code after the panic point does not continue in that execution domain.

**§ 4(3)** Panic is not catch-and-resume exception handling.

**§ 4(4)** Panic is compiler-visible as an execution effect.

**§ 4(5)** Panic does not implicitly change a function's declared return type.

**§ 4(6)** Panic does not implicitly become `Err`.

**§ 4(7)** A panic path is non-returning with respect to the failed stack even when another execution domain later observes the panic outcome.

---

## § 5 No general panic catching

**§ 5(1)** Sec 0.1 provides no general user-level construct equivalent to `try/catch panic`, `recover and resume`, or catch-all exceptions.

**§ 5(2)** Panic may be observed only at a canonical containment boundary, such as awaiting a managed task, joining a managed thread, a declared supervisor, a test harness, or the root panic endpoint.

**§ 5(3)** Containment observes termination; it does not resume the failed operation.

**§ 5(4)** A containment boundary must not pretend that the failed function returned normally.

---

## § 6 Panic domains

**§ 6(1)** A panic domain is the execution unit terminated by panic.

**§ 6(2)** Canonical categories include managed task, managed thread, root process domain, and freestanding/bare-metal root domain.

**§ 6(3)** An ordinary function invocation is not by itself a panic domain.

**§ 6(4)** Returning panic as a hidden error from every function is forbidden.

**§ 6(5)** Panic-domain identity is an execution-context fact available to analysis/lowering where containment behavior depends on it.

---

## § 7 Managed task and thread panic

**§ 7(1)** A panic in a managed task or managed thread terminates that execution domain and never resumes its failed stack.

**§ 7(2)** The domain outcome records that execution panicked and carries bounded panic information.

**§ 7(3)** The awaiter, joiner, or supervisor is notified according to the concurrency contract.

**§ 7(4)** An ordinary contained task/thread panic does not automatically terminate the process.

**§ 7(5)** Where the selected containment policy guarantees panic cleanup, cleanup runs according to the canonical defer/destruction order before the panicked outcome is published.

**§ 7(6)** No task/thread implementation may assume cleanup on a path whose selected panic policy does not provide it.

Conceptually:

```sec
type TaskOutcome[T] union {
    Completed(T)
    Cancelled
    Panicked(PanicInfo)
    Failed(TaskError)
}
```

**§ 7(7)** Exact task outcome naming and await semantics belong to
`rules/concurrency/tasks.md`. `Panicked(PanicInfo)` remains distinct from
`Failed(TaskError)` and from a normally returned Sec `Err(E)`. A selected
hard-termination panic policy need not recover a task-local panic into an
outcome when the panic rulebook says the execution domain cannot contain it.

---

## § 8 Root panic

### § 8.1 Hosted root

**§ 8.1(1)** A panic in the root process domain terminates the process.

**§ 8.1(2)** No normal continuation occurs after root panic.

**§ 8.1(3)** Before termination, the selected policy may perform only panic-safe bounded reporting, permitted cleanup, debugger trap, or user hook.

### § 8.2 Freestanding and bare metal

**§ 8.2(1)** A root panic on a freestanding target transfers control to the configured non-returning panic endpoint.

Possible policies include:

```text
halt
reset
enter safe state
write diagnostic register
signal watchdog
signal hardware
trap to monitor/debugger
```

**§ 8.2(2)** Sec does not prescribe one mandatory bare-metal root behavior.

**§ 8.2(3)** Portable code must not depend on root-domain cleanup after panic unless the resolved profile explicitly guarantees it.

---

## § 9 Containment, escalation, and observation

**§ 9(1)** Containment means the failed domain terminates while another domain may observe that failure and continue.

**§ 9(2)** Containment does not mean the failed operation returned `Result`, the failed stack resumed, the invariant became valid, or shared mutable state became trustworthy.

**§ 9(3)** A contained panic does not automatically escalate to process panic.

**§ 9(4)** Supervisor policy may explicitly record, restart, disable, cancel siblings, escalate, or terminate the process.

**§ 9(5)** A contained managed panic must not disappear silently.

**§ 9(6)** Every contained managed panic must reach a canonical observer such as an awaiter, joiner, supervisor, panic sink, test harness, or root handler.

**§ 9(7)** Detached managed work without an ordinary awaiter must have a supervisor or declared panic sink.

---

## § 10 Shared mutable state and poisoning

**§ 10(1)** Panic containment does not automatically preserve consistency of shared mutable state.

**§ 10(2)** Code that mutates shared state and can panic may leave partially committed logical state even when locks are correctly released.

**§ 10(3)** Programs requiring continued service should use explicit consistency strategies such as transactional update, copy-then-commit, message passing, task-owned state, version validation, rollback, or poisoning.

**§ 10(4)** A synchronization abstraction may define poisoning when panic occurs during protected mutation.

**§ 10(5)** Poisoning is not mandatory for every synchronization primitive.

---

## § 11 Panic cleanup

**§ 11(1)** Panic cleanup is governed jointly by this rulebook, `defer.md`, and `destruction.md`.

**§ 11(2)** Cleanup may include reached defer entries, automatic destruction, task/thread-local cleanup, containment bookkeeping, and panic-sink notification where the selected panic policy guarantees those actions.

**§ 11(3)** Cleanup that is required to run on panic must itself be transitively `noPanic`.

**§ 11(4)** Panic cleanup must preserve the canonical unified LIFO cleanup order where defer/destruction rules require cleanup to execute.

**§ 11(5)** Panic cleanup must not silently introduce a general exception-unwinding runtime.

**§ 11(6)** If a target/profile selects immediate trap/reset/no-cleanup root behavior, pending ordinary cleanup is not guaranteed.

### § 11.1 Defer

**§ 11.1(1)** A defer body that may run during panic cleanup must be transitively `noPanic`.

**§ 11.1(2)** Fallible cleanup inside defer must handle its recoverable error locally without converting it to panic.

### § 11.2 Destruction

**§ 11.2(1)** Destruction required on a panic path must not panic.

**§ 11.2(2)** Custom `free` and automatic destruction must not silently translate recoverable cleanup failure into panic.

**§ 11.2(3)** The compiler must not assume that every panic path performs destruction when the selected panic policy does not guarantee cleanup.

---

## § 12 Double panic

**§ 12(1)** A panic during panic cleanup violates the required no-panic cleanup contract.

**§ 12(2)** If a second panic nevertheless occurs because of unsafe code, foreign code, compiler defect, hardware corruption, or incorrect trusted metadata, the current panic domain terminates immediately through the minimal panic endpoint.

**§ 12(3)** A second cleanup/unwind sequence must not be started.

---

## § 13 Panic information and reason IDs

**§ 13(1)** Every implicit language panic has a stable canonical reason identifier.

Canonical reason categories include at least:

```text
arithmetic overflow
division by zero
invalid shift
bounds failure
contract failure
assertion failure
checked unreachable reached
invalid reference generation
explicit panic
foreign abort/trusted-boundary failure
```

**§ 13(2)** Exact registry spelling and numeric representation belong to the diagnostics/panic registry.

**§ 13(3)** Panic observation uses a bounded structured panic-information value.

Conceptually:

```sec
type PanicInfo struct {
    ID: PanicID
    File: string
    Line: uint
    Column: uint
    Function: string
}
```

**§ 13(4)** The exact public ABI/layout/name of `PanicInfo` is not fixed by the conceptual example.

**§ 13(5)** Optional profile information may include message, operation, type, task/thread ID, source expression, call stack, and related panic.

**§ 13(6)** The minimum representation must not require dynamic allocation.

---

## § 14 Allocation-free panic path

**§ 14(1)** The minimum panic path must not require heap allocation, dynamic string concatenation, growing collections, symbolization, filesystem/network access, or blocking on a potentially poisoned allocator.

**§ 14(2)** A profile may provide richer reporting only when doing so does not violate active panic-context guarantees.

**§ 14(3)** Static source/message metadata may be represented through stable IDs or constant tables.

---

## § 15 Assertions

### § 15.1 Canonical syntax

**§ 15.1(1)** Sec 0.1 defines exactly these ordinary assertion forms:

```sec
assert condition
assert condition, "message"
```

**§ 15.1(2)** Conceptual grammar:

```text
assert_statement :=
    "assert" boolean_expression
    [ "," string_literal ]
```

**§ 15.1(3)** The comma is the canonical separator before the optional assertion message.

**§ 15.1(4)** Function-like `assert(...)` is not the canonical Sec 0.1 form.

**§ 15.1(5)** The optional message is a string literal in Sec 0.1.

**§ 15.1(6)** Sec 0.1 does not define dynamically constructed assertion messages.

### § 15.2 Condition typing

**§ 15.2(1)** The assertion condition must have type `bool`.

**§ 15.2(2)** Sec applies no truthiness conversion.

Valid:

```sec
assert count > 0
assert state == State.Ready, "state must be ready"
```

Invalid when `count` is not `bool`:

```sec
assert count
```

### § 15.3 Meaning

**§ 15.3(1)** If the condition evaluates true, execution continues.

**§ 15.3(2)** If the condition evaluates false, the current panic domain panics with the stable assertion-failure reason.

**§ 15.3(3)** The optional message adds diagnostic information and does not create a separate recoverable error value.

**§ 15.3(4)** Assertion condition evaluation follows ordinary Sec expression evaluation order and side-effect rules.

### § 15.4 Messages

**§ 15.4(1)** The message literal is compile-time/static diagnostic metadata.

**§ 15.4(2)** The minimum panic path must be able to identify assertion failure without dynamic string construction or allocation.

**§ 15.4(3)** A target/profile may omit full message text only when a stable source/message identifier preserves its declared diagnostic contract.

**§ 15.4(4)** Stripping message text must not change program control flow.

### § 15.5 Assertions are always active

**§ 15.5(1)** Ordinary `assert` has identical semantic meaning in debug and optimized builds.

**§ 15.5(2)** The compiler may eliminate an assertion only when it proves the condition true on every path reaching it.

**§ 15.5(3)** Optimization level must not by itself disable ordinary assertions.

### § 15.6 Assertion refinement

**§ 15.6(1)** After a successful assertion, the compiler may use the proven condition as a fact for subsequent semantic analysis.

```sec
assert index >= 0 && index < values.Length
let value := values[index]
```

**§ 15.6(2)** Later redundant checks may be removed when proof is complete.

**§ 15.6(3)** Assertion-based refinement must use the same canonical fact system as ordinary control-flow refinement.

### § 15.7 Not business validation

**§ 15.7(1)** `assert` is for programmer/internal invariants, not expected input or business outcomes.

**§ 15.7(2)** Expected validation failure must use explicit recoverable error/control-flow mechanisms.

### § 15.8 Assertions in `@noPanic`

**§ 15.8(1)** An assertion is valid in `@noPanic` code only when the compiler proves its condition true for every path reaching it.

```sec
type NonNegative int range 0..int.Max

@noPanic
fn Use(value: NonNegative) int {
    assert value >= 0
    return value
}
```

**§ 15.8(2)** An unproven assertion contributes `MayPanic`.

**§ 15.8(3)** A proven assertion may be eliminated and contributes no remaining panic effect.

### § 15.9 Debug assertions and assume

**§ 15.9(1)** Sec 0.1 defines no separate `debug assert` form.

**§ 15.9(2)** `assert` is a checked validation, not an unchecked optimizer promise.

**§ 15.9(3)** Sec 0.1 exposes no ordinary safe-language `assume`.

---

## § 16 Checked `unreachable`

**§ 16(1)** Canonical syntax is:

```sec
unreachable
```

**§ 16(2)** Reaching `unreachable` panics the current domain with the stable unreachable reason.

**§ 16(3)** If the compiler proves the statement unreachable, no panic path is emitted for it.

**§ 16(4)** A proven-unreachable statement contributes no remaining panic effect.

**§ 16(5)** If reachability remains possible, `unreachable` is panic-capable.

**§ 16(6)** `unreachable` is valid in `@noPanic` code only when the compiler proves execution cannot reach it.

**§ 16(7)** Checked `unreachable` is not optimizer undefined behavior.

**§ 16(8)** Backend `unreachable` may appear only after a defined non-returning Sec panic/trap path or after the source path is proven impossible.

---

## § 17 Explicit panic

**§ 17(1)** The exact Sec 0.1 explicit-panic source syntax and payload shape are not locked by this revision.

**§ 17(2)** Regardless of future syntax, explicit panic must terminate the current panic domain, never return to the failed stack, be visible in effect analysis, violate unresolved `@noPanic`, and support an allocation-free minimum representation.

**§ 17(3)** Illustrative syntax from older material is non-normative until a separate grammar decision locks it.

---

## § 18 Panic sinks and root endpoints

**§ 18(1)** A minimum-path panic sink must be `noPanic` and allocation-free.

**§ 18(2)** Root panic reporting ultimately transfers to a non-returning root endpoint.

**§ 18(3)** A contained-domain sink may return to supervisor/scheduler machinery but never to the failed stack.

**§ 18(4)** The endpoint may be compiler-known, user-provided, target-provided, or a direct target trap.

**§ 18(5)** This rulebook does not require a public `never` source type merely to express the endpoint.

---

## § 19 Foreign code and unsafe boundaries

**§ 19(1)** Foreign exceptions/unwinding must not cross Sec frames unless a dedicated FFI rule explicitly defines and proves such behavior.

**§ 19(2)** FFI contracts must classify relevant panic/abort/unwind behavior.

**§ 19(3)** Unknown foreign behavior is not positive proof of `noPanic`.

**§ 19(4)** `unsafe` does not automatically waive `noPanic` or panic-domain requirements.

**§ 19(5)** A trusted foreign/unsafe annotation may provide an explicit proof boundary only where canonical FFI/unsafe rules permit it.

---

## § 20 Panic, `Result`, and cancellation

**§ 20(1)** Panic is not automatically converted into `Err`.

**§ 20(2)** Returned `Err(error)` is normal completion of the function's recoverable result channel.

**§ 20(3)** A contained panic is abnormal termination of the execution domain.

**§ 20(4)** A containment API must preserve the distinction between returned recoverable error and panicked execution.

**§ 20(5)** Cancellation is not panic.

**§ 20(6)** A supervisor may cancel sibling work after observing panic; that policy does not make cancellation and panic equivalent.

---

## § 21 `@noPanic`

**§ 21(1)** `@noPanic` is a compiler-verified transitive guarantee.

**§ 21(2)** A callable satisfying `@noPanic` has no reachable language-defined panic on any valid execution path covered by the contract.

**§ 21(3)** Proof includes direct operations, direct/indirect calls, relevant generic specializations, callbacks, FFI effects, compiler helpers, cleanup, defer, destruction, and runtime checks.

**§ 21(4)** Unknown mandatory effect information is not positive proof of `noPanic`.

**§ 21(5)** Panic capability may arise from unhandled checked runtime operations, unproven assertions, reachable checked `unreachable`, future explicit panic, panic-capable calls, unknown/foreign abort paths, compiler/runtime helpers, and required panic-capable cleanup.

**§ 21(6)** A source proven unreachable contributes no panic effect.

**§ 21(7)** A checked operation whose panic path is eliminated by proof or replaced by a canonical handled fallible path contributes no panic effect for that resolved operation.

---

## § 22 ISR interaction

**§ 22(1)** Sec 0.1 `@isr` and `@interruptSafe` imply `noPanic`, `noAlloc`, and `noBlock` according to `interrupts.md`.

**§ 22(2)** An unproven assertion is invalid in ISR code.

**§ 22(3)** A proven assertion may remain as a source-level invariant and be eliminated without violating `noPanic`.

**§ 22(4)** Reachable checked `unreachable`, future explicit panic, panic-capable helper calls, panic-capable runtime checks, unsafe cleanup, or unknown foreign abort behavior make an ISR invalid.

**§ 22(5)** `unsafe` does not waive ISR `noPanic`.

**§ 22(6)** ISR panic analysis includes generated wrappers/helpers and reachable cleanup paths according to `interrupts.md`.

---

## § 23 Panic strategy and portability

**§ 23(1)** A resolved target/profile may select root behavior such as cleanup-then-terminate, limited cleanup, immediate termination, trap, reset, or custom endpoint.

**§ 23(2)** Exact configuration syntax is owned by build/profile/platform rulebooks.

**§ 23(3)** Portable application logic must not depend on one root panic cleanup strategy unless it targets a profile guaranteeing it.

**§ 23(4)** Managed task/thread containment remains a feature-domain policy separate from root panic strategy.

**§ 23(5)** No target/profile may redefine panic as recoverable hidden exception flow.

---

## § 24 Runtime-free implementation

**§ 24(1)** A hosted process with no managed tasks may branch directly from a panic site to a user/compiler/target-provided non-returning endpoint.

**§ 24(2)** A bare-metal program may branch directly to trap/reset/halt/custom panic code without allocator, scheduler, unwinder, or general runtime library.

**§ 24(3)** Programs using managed tasks/threads may link only the containment machinery required by those used features.

**§ 24(4)** A program proven entirely panic-free may remove unreachable panic support.

---

## § 25 Effect analysis

**§ 25(1)** Panic is represented by canonical compiler effect facts.

**§ 25(2)** Analysis must distinguish at least proven no-panic behavior from behavior that may panic.

**§ 25(3)** Internal facts should retain detailed panic causes and source provenance.

**§ 25(4)** Panic effects propagate synchronously through reachable calls.

**§ 25(5)** Contained asynchronous child panic is not automatically a synchronous panic effect of the parent; concurrency rules define observation/escalation behavior.

**§ 25(6)** Separate-compilation effect summaries must be versioned/validated before serving as positive `noPanic` proof.

**§ 25(7)** Panic effects remain distinct from allocation, blocking, I/O, volatile, FFI, synchronization, and mutation effects.

---

## § 26 Semantic IR

**§ 26(1)** Semantic IR must preserve explicit panic-producing operations that remain after semantic proof.

**§ 26(2)** Semantic IR must preserve enough information to recover panic reason/category, source provenance, panic effect, relevant execution-domain facts, assertion/unreachable origin, static diagnostic message/ID where present, and required cleanup relationships.

**§ 26(3)** A proven assertion may be absent from Semantic IR after its refinement facts are incorporated canonically.

**§ 26(4)** An unproven assertion must lower as an explicit checked branch to the assertion panic path.

**§ 26(5)** Checked `unreachable` retains defined semantics until proof removes it or lowering materializes its non-returning panic endpoint.

**§ 26(6)** Semantic IR verification must reject contradictory panic/no-panic facts.

---

## § 27 Lowering

**§ 27(1)** Lowering must preserve panic reason, non-returning behavior, and selected containment/root endpoint semantics.

**§ 27(2)** Backend `unreachable` must not turn a defined Sec panic into undefined behavior.

**§ 27(3)** Assertion failure lowering must not allocate merely to report a string-literal message.

**§ 27(4)** Static source/message metadata may be encoded as constant data or stable IDs.

**§ 27(5)** Root panic lowering must respect target/profile cleanup policy.

**§ 27(6)** Managed containment lowering must never resume the failed stack.

**§ 27(7)** Optimizations may remove panic checks only when canonical analysis proves the panic condition impossible.

**§ 27(8)** Lowering must not introduce a general unwinding runtime merely for convenience.

---

## § 28 Diagnostics and tooling

**§ 28(1)** Panic diagnostics follow the mentor-compiler principle.

**§ 28(2)** Diagnostics should explain the panic-capable operation, concrete cause, source/call path, violated guarantee, missing proof, and a practical alternative when known.

**§ 28(3)** An `@noPanic` diagnostic should identify the first relevant panic-capable operation and transitive path from the verified root.

**§ 28(4)** Assertion diagnostics distinguish syntax/type errors, unproven assertion under `@noPanic`, and inappropriate use where an owning rule requires recoverable validation.

**§ 28(5)** ISR panic diagnostics identify the interrupt root/context and reachable panic cause.

**§ 28(6)** LSP and `sec analyse` must consume the same canonical panic-effect and proof facts as compilation.

**§ 28(7)** Tooling may expose `noPanic`, `may panic`, panic reasons, source causes, transitive cause paths, containment domains, and assertion refinement.

**§ 28(8)** Incremental analysis invalidates panic summaries when relevant bodies, targets, generic specializations, FFI contracts, runtime-check rules, cleanup plans, target profiles, or imports change.

---

## § 29 Required tests

**§ 29(1)** Assertion syntax/semantics tests include:

```text
assert true
assert false
assert boolean_expression
assert boolean_expression, "message"
formatter preserves comma separator
missing comma before message rejected
non-bool condition rejected
non-literal Sec 0.1 message form rejected
proven assertion refines analysis
proven assertion removable
unproven assertion contributes MayPanic
unproven assertion rejected under @noPanic
assertion message path allocates nothing
```

**§ 29(2)** Checked-unreachable tests include proven elimination, reachable panic, `MayPanic`, rejection under `@noPanic` when unproven, and defined panic before backend `unreachable`.

**§ 29(3)** Error/panic separation tests include returned `Err`, contained `Panicked`, no implicit panic-to-Err conversion, and `try` not catching panic.

**§ 29(4)** Containment tests include managed task/thread reporting, no automatic process escalation, root termination, detached observation, and no failed-stack resumption.

**§ 29(5)** Cleanup tests include policy-guaranteed cleanup ordering, no assumed cleanup under no-cleanup profile, no-panic cleanup, local handling of fallible cleanup, and double-panic minimal termination.

**§ 29(6)** ISR tests include direct/transitive panic rejection, unproven assert rejection, proven assert acceptance/elimination, unknown foreign behavior, and unsafe not waiving `noPanic`.

**§ 29(7)** Binary tests include allocation-free minimum panic path, bare-metal target endpoint, hosted root without general runtime, feature-only containment linkage, and removal of panic support from proven panic-free binary.

**§ 29(8)** Compiler, LSP, `sec analyse`, formatter, Semantic IR, and maintained backend tests must agree on assertion syntax, panic effects, no-panic proof, and panic reason provenance.

---

## § 30 Completion criteria

**§ 30(1)** Frontend support is complete when canonical `assert` and checked `unreachable` parse/format/type-check, panic-producing operations are classified, assertion refinement is represented, and direct diagnostics are complete.

**§ 30(2)** Effect analysis is complete when every panic source and callable form participates in deterministic transitive `MayPanic`/`noPanic` proof, including indirect calls, generics, callbacks, FFI, cleanup, compiler helpers, recursion, and separate compilation.

**§ 30(3)** Cleanup integration is complete when every panic policy preserves exactly the cleanup guarantees defined jointly by panic/defer/destruction without assuming nonexistent unwinding.

**§ 30(4)** Concurrency containment is complete when managed task/thread panic is recorded and observed without resuming failed execution or collapsing panic into returned `Result`.

**§ 30(5)** ISR integration is complete when interrupt roots consume the same canonical panic effects and enforce Sec 0.1 `noPanic` transitively.

**§ 30(6)** Semantic IR/lowering is complete when reasons, checks, endpoints, source provenance, assertion messages/IDs, and containment/root policy are represented and lowered without undefined-behavior substitution or hidden runtime allocation.

**§ 30(7)** Tooling is complete when compiler, LSP, `sec analyse`, diagnostics, formatter, and call hierarchy use the same canonical panic facts.

**§ 30(8)** Panic must not be marked fully implemented merely because checked arithmetic already records `MayPanic`.

---

## § 31 Core summary

**§ 31(1)** Panic terminates the current panic domain and never resumes the failed stack.

**§ 31(2)** Expected failures use explicit recoverable values and control flow.

**§ 31(3)** Sec 0.1 provides no general catch-and-resume panic mechanism.

**§ 31(4)** Ordinary assertions are written:

```sec
assert condition
assert condition, "message"
```

**§ 31(5)** Assertions are always semantically active and may be removed only after proof.

**§ 31(6)** Assertion messages are string literals/static diagnostic metadata in Sec 0.1.

**§ 31(7)** Checked `unreachable` has defined panic behavior and is never an unchecked optimizer promise.

**§ 31(8)** `@noPanic` is a transitive compiler-verified guarantee.

**§ 31(9)** Sec 0.1 ISR execution is `noPanic`.

**§ 31(10)** The minimum panic path is allocation-free and does not require a general Sec runtime.

**§ 31(11)** Exact explicit `panic` source syntax remains intentionally unresolved by this revision.

**§ 31(12)** Panic cleanup occurs only where the selected canonical panic policy guarantees it; no rule may silently invent exception unwinding.
