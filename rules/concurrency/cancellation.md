# Cancellation

- **Status:** Normative
- **Created:** 2026-09-06
- **Last updated:** 2026-09-24
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/cancellation.md`
- **Implementation governance:** `governance/concurrency_cancellation.yaml`
- **Replaces:** Earlier unversioned revision at the same canonical path
- **Repository baseline reviewed:** `0f5027d`
- **Related rulebooks:** `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/concurrency/mutex.md`, `rules/concurrency/channels.md`, `rules/concurrency/select.md`, `rules/concurrency/await.md`, `rules/concurrency/concurrency.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/concurrency/concurrency_runtime_model.md`, `rules/concurrency/blocking.md`, `rules/concurrency/scheduling.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/transferability.md`, `rules/memory/destruction.md`, `rules/control-flow/defer.md`, `rules/compiler/semantic_ir.md`, `rules/platform/ffi.md`, `rules/platform/interrupts.md`, `rules/tooling/lsp.md`

---

## § 1. Purpose and authority

**Governance tags:** `concurrency.cancellation-v2`

§ 1(1) Cancellation is Sec's cooperative mechanism for requesting that a cancellable execution entity stop before normal completion.

§ 1(2) This rulebook owns the language semantics of:

- cancellation requests for `Task[T]` and `Thread[T]`;
- current task/thread cancellation observation;
- the `cancel` statement;
- compiler-inferred cancellable-execution effects;
- cancellation-point commit rules;
- the canonical `Context`, `ContextSource`, and `ContextError` cancellation model;
- the distinction between execution cancellation and operation-local Context cancellation/deadline;
- cancellation cleanup and memory-order requirements.

§ 1(3) Task result/outcome ownership remains owned by `tasks.md`.

§ 1(4) Physical thread lifecycle and native termination remain owned by `threads.md` and platform rules.

§ 1(5) Exact commit semantics of a channel, select branch, mutex acquisition, await, join, I/O wait, or other blocking operation remain jointly governed by this rulebook and the owning operation rulebook.

§ 1(6) Mutable implementation status does not belong in this normative rulebook.

§ 1(7) Mutable implementation status for cancellation is maintained through
`governance/concurrency_cancellation.yaml`.

---

## § 2. Cancellation is cooperative

**Governance tags:** `concurrency.cancellation-v2`

§ 2(1) Requesting cancellation does not asynchronously destroy ordinary safe Sec execution.

§ 2(2) A cancellation request becomes terminal cancellation only when the target execution observes and commits cancellation according to this rulebook.

§ 2(3) Observation may occur through:

- an explicit `CancelRequested` check followed by `cancel`;
- a cancellation-aware wait or blocking operation;
- another compiler/runtime-defined cancellation point whose source semantics are specified by its owning rulebook.

§ 2(4) Safe Sec does not tear down an ordinary task or thread at an arbitrary instruction merely because cancellation was requested.

§ 2(5) Cooperative cancellation preserves ordinary cleanup guarantees unless another explicit unsafe/platform rule states otherwise.

---

## § 3. Distinct concepts

**Governance tags:** `concurrency.cancellation-v2`

§ 3(1) Cancellation is distinct from a normal function return.

§ 3(2) Cancellation is distinct from returning `Err(E)`.

§ 3(3) Cancellation is distinct from panic.

§ 3(4) Cancellation is distinct from unsafe hard thread termination.

§ 3(5) Cancellation is distinct from process kill/termination.

§ 3(6) Cancellation is distinct from hardware reset.

§ 3(7) Direct timeout of an operation is distinct from terminal task/thread cancellation.

§ 3(8) Expiration of a supplied `Context` deadline is represented as `ContextError.DeadlineExceeded` for Context-aware operations and does not by itself terminally cancel the current task/thread.

---

## § 4. Supported execution identities

**Governance tags:** `concurrency.cancellation-v2`, `concurrency.tasks-v2`

§ 4(1) Ordinary cooperative execution cancellation applies to:

```text
Task[T]
Thread[T]
```

§ 4(2) A `Task[T]` is a logical/green Sec execution identity.

§ 4(3) A `Thread[T]` is a physical/native target execution identity.

§ 4(4) Task cancellation identity and physical thread cancellation identity are always distinct.

§ 4(5) A task may migrate between physical executor threads without changing its task cancellation identity.

§ 4(6) A physical worker executing a task does not become the task's cancellation identity.

§ 4(7) Process cancellation/termination is outside this task/thread cancellation model and remains owned by the process rulebooks.

---

## § 5. Symmetric task/thread cancellation surface

**Governance tags:** `concurrency.cancellation-v2`, `tooling.cancellation-v2`

§ 5(1) Task and thread cancellation APIs deliberately use symmetric names where the semantic role is shared.

§ 5(2) The owning handles use:

```sec
worker.RequestCancel()
```

§ 5(3) Current-execution observation uses:

```sec
Task.Current().CancelRequested
Thread.Current().CancelRequested
```

§ 5(4) Symmetric naming does not make task and thread identities interchangeable.

§ 5(5) Tooling should preserve the symmetry while still displaying task-specific versus thread-specific types.

---

## § 6. Relevant `Task[T]` cancellation surface

**Governance tags:** `concurrency.cancellation-v2`, `concurrency.tasks-v2`, `tooling.cancellation-v2`

§ 6(1) The task-handle type materially used by this rulebook is:

```sec
@noCopy
type Task[T]
// `@noCopy` means the owning task handle cannot be copied.
// `[T]` is the complete declared return type of the task function.
// The handle owns unresolved task lifecycle responsibility.
// Runtime representation is compiler/runtime-private.
```

§ 6(2) The exact cancellation-related public extension is:

```sec
impl Task[T] {
    fn RequestCancel() void
    // Requests cooperative cancellation of this task identity.
    // Does not consume the owning Task[T] handle.
    // Is idempotent.
    // Has no effect after terminal completion.
}
```

§ 6(3) `RequestCancel()` is a lifecycle/state operation and does not require the local task-handle binding itself to be `mut` merely to request cancellation.

§ 6(4) `RequestCancel()` does not resolve the owning handle's lifecycle obligation.

§ 6(5) A caller must still `await`, `join`, `detach`, or transfer the owning handle according to `tasks.md`.

---

## § 7. Relevant `Thread[T]` cancellation surface

**Governance tags:** `concurrency.cancellation-v2`, `tooling.cancellation-v2`

§ 7(1) The physical thread-handle type materially used by this rulebook is:

```sec
@noCopy
type Thread[T]
// `@noCopy` means the owning thread handle cannot be copied.
// `[T]` is the complete declared return type of the thread callable.
// The handle owns unresolved physical-thread lifecycle responsibility.
// Runtime/native representation is compiler/platform-private.
```

§ 7(2) The exact cancellation-related public extension is:

```sec
impl Thread[T] {
    fn RequestCancel() void
    // Requests cooperative cancellation of this physical thread identity.
    // Does not consume the owning Thread[T] handle.
    // Is idempotent.
    // Has no effect after terminal completion.
}
```

§ 7(3) `RequestCancel()` is not unsafe hard termination.

§ 7(4) Requesting cancellation does not imply that the physical thread has already stopped.

---

## § 8. Exact `TaskContext` cancellation view

**Governance tags:** `concurrency.cancellation-v2`, `tooling.cancellation-v2`

§ 8(1) The compiler-known/source-visible current-task view is:

```sec
type TaskContext
// Immutable, non-owning view of the currently executing logical Task.
// Does not own Task[T] lifecycle or result ownership.
// Does not grant RequestCancel() authority.
// Physical representation is compiler/runtime-private.
```

§ 8(2) The exact cancellation-related public extension is:

```sec
impl TaskContext {
    CancelRequested bool
    // Live immutable observation of whether cancellation has been requested
    // for this logical task identity.
}
```

§ 8(3) `TaskContext` is not `Task[T]`.

§ 8(4) Reading `CancelRequested` does not acquire task lifecycle ownership.

§ 8(5) Reading `CancelRequested` does not consume a task outcome.

§ 8(6) Any additional task-context metadata is owned by the task/runtime rulebooks and must not change the cancellation semantics defined here.

---

## § 9. Exact `ThreadContext` cancellation view

**Governance tags:** `concurrency.cancellation-v2`, `tooling.cancellation-v2`

§ 9(1) The compiler-known/source-visible current-thread view is:

```sec
type ThreadContext
// Immutable, non-owning view of the current physical/native thread.
// Does not own Thread[T] lifecycle, join capability, or result ownership.
// Does not grant RequestCancel() authority.
// Physical representation is compiler/platform-private.
```

§ 9(2) The exact cancellation-related public extension is:

```sec
impl ThreadContext {
    CancelRequested bool
    // Live immutable observation of whether cancellation has been requested
    // for this physical thread identity.
}
```

§ 9(3) `ThreadContext` is not `Thread[T]`.

§ 9(4) Existing thread identity/name/platform metadata remains owned by `threads.md` and may be declared in additional canonical `impl ThreadContext` surface there.

§ 9(5) Such additional members must not alter or duplicate the `CancelRequested` semantic identity defined here.

---

## § 10. `Task.Current()`

**Governance tags:** `concurrency.cancellation-v2`, `concurrency.tasks-v2`, `tooling.cancellation-v2`

§ 10(1) The canonical current-task lookup surface is:

```sec
impl Task {
    static fn Current() TaskContext
    // Returns the immutable non-owning context of the currently executing
    // logical task.
}
```

§ 10(2) `Task.Current()` is valid only when a current logical task context exists.

§ 10(3) Calling `Task.Current()` where no task context exists is a compile-time error when statically known.

§ 10(4) A backend must not fabricate a task context merely because code executes on a worker thread that is capable of running tasks.

§ 10(5) The returned `TaskContext` follows the logical task across permitted worker migration.

### `Task.Yield()` cancellation point

§ 10(6) `Task.Yield()` requires the same real logical task context as `Task.Current()`.
It is a current-task cancellation point participating in the canonical
first-commit race: cancellation winning prevents return to the following source
statement, while normal yield winning keeps the task runnable and later returns
`void`. `Thread.Yield()` is a physical scheduler hint and is not a cancellation
point. Neither operation creates a memory-synchronization edge. Exact scheduling
and migration semantics are owned by `scheduling.md`.

---

## § 11. `Thread.Current()`

**Governance tags:** `concurrency.cancellation-v2`, `tooling.cancellation-v2`, `compiler.platform-model`

§ 11(1) The canonical current-thread lookup surface is:

```sec
impl Thread {
    static fn Current() ThreadContext
    // Returns the immutable non-owning context of the current physical/native
    // thread where the selected target provides a physical thread context.
}
```

§ 11(2) `Thread.Current()` refers to the physical execution thread even when that thread currently executes a logical task.

§ 11(3) On a target/profile without the relevant physical-thread capability, use of `Thread.Current()` must be rejected according to target capability rules rather than fabricated as a task identity.

---

## § 12. Task and thread observation remain distinct

**Governance tags:** `concurrency.cancellation-v2`

§ 12(1) In task code:

```sec
Task.Current().CancelRequested
```

observes the logical task cancellation request.

§ 12(2) In the same task code:

```sec
Thread.Current().CancelRequested
```

observes the physical executor thread cancellation request, if the physical thread context is available.

§ 12(3) The two properties may therefore have different values at the same instant.

§ 12(4) Requesting task cancellation must not implicitly request cancellation of the physical worker thread.

§ 12(5) Requesting physical thread cancellation must not silently rewrite task cancellation state unless a separate explicit structured/runtime policy defines propagation.

---

## § 13. Exact `cancel` syntax

**Governance tags:** `concurrency.cancellation-v2`, `frontend.cancellation-v2`

§ 13(1) The complete Sec 0.1 source syntax is:

```sec
cancel
```

§ 13(2) `cancel` is a statement, not an expression.

§ 13(3) `cancel` has no value and cannot appear where an expression is required.

§ 13(4) `cancel` is terminating for control-flow and reachability analysis.

§ 13(5) Source after an unconditional `cancel` is unreachable under the ordinary reachability rules.

---

## § 14. Meaning of `cancel`

**Governance tags:** `concurrency.cancellation-v2`, `frontend.cancellation-v2`

§ 14(1) `cancel` commits cooperative cancellation of the current cancellable execution entity.

§ 14(2) If a current logical task exists, `cancel` terminates that task as cancelled.

§ 14(3) If no current logical task exists but execution is inside an explicit cancellable physical thread callable, `cancel` terminates that physical thread as cancelled.

§ 14(4) `cancel` outside a valid cancellable task/thread execution context is a compile-time error.

§ 14(5) A logical task therefore takes precedence over its physical executor thread for the meaning of bare `cancel`.

§ 14(6) Code that intends to respond to physical thread cancellation while running inside a task must not assume that bare `cancel` terminates the worker thread; bare `cancel` terminates the current logical task.

---

## § 15. Cleanup performed by `cancel`

**Governance tags:** `concurrency.cancellation-v2`, `frontend.destruction-v2`

§ 15(1) Terminal cooperative cancellation executes active ordinary `defer` cleanup according to `defer.md`.

§ 15(2) Terminal cooperative cancellation performs deterministic destruction of owned values according to destruction rules.

§ 15(3) Terminal cooperative cancellation releases ordinary resources whose owners are destroyed by cleanup.

§ 15(4) A live `MutexGuard[T]` owned by the cancelling execution must be destroyed/released before terminal cancellation completes.

§ 15(5) Cancellation does not synthesize a normal return value of the callable's declared type.

§ 15(6) Cancellation does not become a normal `Err(E)` merely because the callable's declared type is `Result[T, E]`.

---

## § 16. `cancel` inside `defer`

**Governance tags:** `concurrency.cancellation-v2`, `frontend.cancellation-v2`

§ 16(1) `cancel` is invalid inside a `defer` body.

§ 16(2) Cleanup code must not recursively replace the terminal control flow already performing deterministic cleanup.

§ 16(3) The compiler must diagnose the `cancel` statement specifically rather than reporting only generic unreachable/return-type fallout.

Suggested diagnostic:

```text
cancel is not valid inside defer cleanup
```

---

## § 17. Explicit observation pattern

**Governance tags:** `concurrency.cancellation-v2`

§ 17(1) A logical task may explicitly observe its own cancellation state:

```sec
if Task.Current().CancelRequested {
    cancel
}
```

§ 17(2) An explicit physical thread callable may use the symmetric form:

```sec
if Thread.Current().CancelRequested {
    cancel
}
```

§ 17(3) Reading `CancelRequested` does not itself terminate execution.

§ 17(4) `cancel` performs the terminal transition.

§ 17(5) Code may intentionally observe a request and perform bounded cleanup/work before executing `cancel`, provided it does not violate other lifecycle or shutdown contracts.

---

## § 18. Cancellation-request memory semantics

**Governance tags:** `concurrency.cancellation-v2`, `concurrency.memory-model-v2`

§ 18(1) `RequestCancel()` must be safely observable by the target execution identity.

§ 18(2) `CancelRequested` observation must be data-race-free.

§ 18(3) The compiler/runtime may implement cancellation state through atomics or another target/runtime primitive that preserves the same source semantics.

§ 18(4) Cancellation-request publication does not automatically publish arbitrary unrelated mutable program data.

§ 18(5) Application data still requires mutex, atomic, channel, completion, or another canonical synchronization relation.

---

## § 19. Compiler-inferred cancellable-execution effect

**Governance tags:** `frontend.cancellation-effect-v1`, `concurrency.cancellation-v2`

§ 19(1) Sec 0.1 defines a compiler semantic effect meaning:

```text
requires current cancellable execution
```

§ 19(2) This effect is compiler-inferred and has no required source annotation.

§ 19(3) Sec 0.1 does not require syntax such as:

```sec
@cancellable
fn Work() void
```

§ 19(4) A function directly containing `cancel` requires a current cancellable execution context.

§ 19(5) A function calling another callable that requires current cancellable execution inherits the requirement unless the call is proven unreachable.

§ 19(6) The effect is part of semantic callable analysis even though it is not written in ordinary source syntax.

§ 19(7) **Amendment MD-138 (2026-11-24, Sec 0.1.1, `rules/corrections/applied/MD-138-callable-execution-requirements-v0.1.1-correction-2026-11-24-20261124.md` § 1(2)).** Inference stays the default for body-bearing native functions, methods and lambdas. From Sec 0.1.1 a callable may also state its contract with the `require(...)` clause (§ 24A); `@cancellable` and other annotation spellings remain absent. No program is required to repeat an inferred requirement at any call level.

---

## § 20. Transitive effect propagation

**Governance tags:** `frontend.cancellation-effect-v1`

§ 20(1) Given:

```sec
fn Inner() void {
    cancel
}

fn Outer() void {
    Inner()
}
```

both `Inner` and `Outer` require current cancellable execution.

§ 20(2) Propagation is transitive through the semantic call graph.

§ 20(3) Recursive strongly connected components containing a cancellation-dependent path carry the effect as required by their reachable calls.

§ 20(4) Generic instantiations preserve the effect of the instantiated callable body and callees.

§ 20(5) The compiler must not require a programmer to duplicate an inferred cancellation annotation at every call level.

---

## § 21. Callable values cannot erase the effect

**Governance tags:** `frontend.cancellation-effect-v1`, `analysis.semantic-ir-v2`

§ 21(1) Storing a cancellation-dependent function in a callable value must preserve the effect.

Example:

```sec
let callback := Inner
```

§ 21(2) Indirect invocation of `callback` still requires a current cancellable execution context.

§ 21(3) Assigning or converting a cancellation-dependent callable must not erase the effect merely because the ordinary parameter/return signature matches a context-free callable.

§ 21(4) Callable compatibility analysis must therefore include the hidden/inferred cancellation requirement.

§ 21(5) Semantic IR must preserve the effect at indirect call boundaries.

§ 21(6) **Amendment MD-138 (2026-11-24, Sec 0.1.1, `rules/corrections/applied/MD-138-callable-execution-requirements-v0.1.1-correction-2026-11-24-20261124.md` § 6).** The hidden requirement of § 21(4) is, from Sec 0.1.1, the normalized execution-requirement component of the canonical function type, and function types can declare it (§ 24A). Converting a requirement-bearing callable to a plain `fn(...) T` is rejected, also for a known closed target and inside a task context; this closes the open-target erasure of MD-138.

---

## § 22. Invalid context-free invocation

**Governance tags:** `frontend.cancellation-effect-v1`, `tooling.cancellation-v2`

§ 22(1) Calling a cancellation-dependent callable where no cancellable `Task` or eligible explicit `Thread` execution context exists is invalid.

Suggested diagnostic:

```text
call to ProcessBatch requires a current cancellable task or thread execution context
```

§ 22(2) The diagnostic should identify the first relevant cancellation-dependent operation/callee where practical.

§ 22(3) A compiler must not silently reinterpret `cancel` as `return` merely to make a context-free call valid.

---

## § 23. `Task.Current()` effect

**Governance tags:** `frontend.cancellation-effect-v1`, `concurrency.tasks-v2`

§ 23(1) `Task.Current()` requires a current logical task context.

§ 23(2) This is a more specific semantic requirement than generic current-cancellable-execution because a physical thread without a logical task does not satisfy it.

§ 23(3) A function calling `Task.Current()` therefore carries a compiler-known current-task requirement through ordinary call/effect analysis.

§ 23(4) The compiler must not satisfy that requirement with `Thread.Current()`.

---

## § 24. `Thread.Current()` capability requirement

**Governance tags:** `frontend.cancellation-effect-v1`, `compiler.platform-model`

§ 24(1) `Thread.Current()` requires a target/profile physical-thread context.

§ 24(2) This requirement is separate from the logical-task requirement.

§ 24(3) A task running on a target with physical worker threads may observe both task and thread contexts while retaining distinct identities.

---

## § 24A. Execution requirement contracts (Sec 0.1.1)

**Governance tags:** `frontend.cancellation-effect-v1`

§ 24A(1) **Amendment MD-138 (2026-11-24, Sec 0.1.1, `rules/corrections/applied/MD-138-callable-execution-requirements-v0.1.1-correction-2026-11-24-20261124.md` §§ 1, 4).** A native callable has an execution-requirement contract: conditions the caller's execution environment must satisfy before invocation. It is not a parameter, value, runtime assertion, cancellation request, scheduler selection or result. Its source spelling is the `require(...)` clause (`rules/foundations/grammar.md` § 101A). The Sec 0.1.1 vocabulary is exactly:

| Entry | Meaning | Satisfied when |
|---|---|---|
| `task` | current logical task | a real current `TaskContext` exists under the task model |
| `thread` | current physical thread context | the active target/profile and execution environment establish a valid physical/native thread context as required by `Thread.Current()` |
| `cancellable` | current cancellable execution | bare `cancel` has an eligible current logical task or, without one, an eligible explicit cancellable physical thread (§ 14) |

§ 24A(2) `task` implies `cancellable`. Nothing else implies anything: `thread` does not imply `cancellable` (ordinary main or an attached foreign thread may have a physical context without an eligible cancellable identity), `cancellable` implies neither `task` nor `thread`, and `task` and `thread` are independent.

§ 24A(3) A clause creates no task, thread, process or implicit `Context`, requests no cancellation, selects no scheduler and grants no handle authority. `require(cancellable)` states that an eligible identity is available, not that a request is present or that the callable reaches a cancellation point. Satisfaction is proved statically from canonical execution-context and `CompilationPlan` facts; missing proof is not permission, and a failed proof is a compile-time diagnostic, never a runtime `Err`, panic, implicit spawn or disabled operation. Restrictions for ISR, `defer`, shielding, FFI and unsafe code stay independently enforced; the clause cannot waive them or create a cancellation point.

§ 24A(4) **Inference (§ 5 of the correction).** Every concrete native callable has a requirement summary, separate from its ordinary signature and its may-effect and guarantee summaries, and part of its resolved semantic type. `Task.Current()` and `Task.Yield()` contribute `task`, `Thread.Current()` contributes `thread`, bare `cancel` contributes `cancellable`, an ordinary synchronous call contributes the callee's effective requirements and an indirect synchronous invocation those of the resolved function type. The summary is the conjunction over all semantically reachable operations and calls, with a stable fixed point for mutual recursion and concrete types and callees for generic specializations; it never depends on optional optimization or a runtime target. Other context-aware operations do not become context-dependent through this table.

§ 24A(5) Without a clause, a body-bearing native callable's contract is its inferred summary. With a clause, the declaration is its contract: every environment satisfying the declaration must satisfy the inferred requirements, otherwise the declaration is diagnosed with the first relevant originating operation or callee and the missing requirement. A stronger declaration than the body needs is legal and stays part of the public contract. The compiler neither strengthens an insufficient declaration nor weakens a stronger one, and never accepts a body because its current callers happen to satisfy more.

```sec
fn Inspect() void require(task) {          // valid
    let current := Task.Current()
    discard current.CancelRequested
}

fn ReservedForTasks() void require(task) {  // valid: stronger than needed
}

fn Invalid() void require(cancellable) {   // invalid: cancellable does not establish a task
    let current := Task.Current()
    discard current.CancelRequested
}
```

§ 24A(6) Inference does not cross a callable-definition boundary because a closure is constructed, captured, stored, returned or moved; the contract belongs to the callable constructed. Capture expressions, arguments and other operations evaluated in the current context keep their own requirements, and invocation, including indirect invocation, is checked separately. An independently prohibited operation stays prohibited. A concrete root or any boundary with a fixed execution contract must satisfy the requirements; a context-free program entry is never accepted by inventing a root task.

§ 24A(7) **Identity and compatibility (§ 6).** The canonical function type includes the normalized requirement set; it does not depend on parameter names, captures, entry order or whether the contract was written or inferred. With `task` present, `cancellable` is redundant in the identity; the source clause is kept for diagnostics and formatting. For an assignment or conversion of a callable with effective requirements `S` to a destination type declaring `D`, the requirement component is compatible exactly when `D` implies `S`. This is checked from the portable contract, not from the assignment site, in addition to every capability and signature check.

| `S` (source) | `D` (destination) | Result |
|---|---|---|
| none | none | valid |
| none | task | valid |
| task | none | invalid |
| cancellable | task | valid |
| task | cancellable | invalid |
| thread | task | invalid |
| thread | task, thread | valid |
| task, thread | task | invalid |
| task, cancellable | task | valid; equivalent |

```sec
let inferred := NeedsTask                         // valid: type fn() void require(task)
let typed: fn() void require(task) := NeedsTask   // valid
let invalid: fn() void := NeedsTask               // invalid, also inside a task
```

§ 24A(8) After conversion to a stronger declared contract, invocation uses the destination contract; known implementation facts may support optimization but never weaken it, and no requirement-stripping conversion exists. Requirements survive legal storage in structs, unions, collections, generic arguments and return values; a facility that cannot preserve them rejects the operation instead of treating the value as context-free. The requirement dimension composes with, and is checked independently of, `mut fn` and `-> fn` capabilities, ownership and capture modes. Overload selection uses requirement identity only where a parameter is itself a callable type; declarations are not overloaded by their own clause and there is no dispatch or ranking by the current context.

§ 24A(9) **Indirect calls, interfaces and generics (§ 7).** A call through an explicitly typed callable parameter uses its declared contract, whatever the target set; no whole-program target discovery is needed. A function that only transports a requirement-bearing callable without invoking it does not acquire its requirement. Statically resolved generic instantiations keep the requirement components of their substituted types; there are no effect variables, effect-polymorphic `fn` types or implicit existential requirements, and a plain `fn(...) T` in a generic declaration stays context-free. A native interface method may declare a clause (omitted means none); a conforming method's effective contract must be compatible under § 24A(7), and interface dispatch and constraint calls use the declared member contract. Inheritance and conflicting-member rules are unchanged; normalized requirement identity takes part in signature comparison and conflicting contracts are not merged. MD-053 still owns effect guarantees on interface requirements.

```sec
fn Execute(callback: fn() void require(task)) void {   // infers require(task)
    callback()
}

fn Forward(callback: fn() void require(task)) fn() void require(task) {  // no requirement of its own
    return callback
}

interface TaskWorker {
    fn Work() void require(task)
}
```

§ 24A(10) **Spawn boundaries (§ 8).** Ordinary synchronous calls inherit the current context. A spawned callable runs in a new execution identity, and its requirements are checked against that identity and the active target/profile, not propagated to the creator. `spawn` and `spawn task` establish a logical task and satisfy `task` and `cancellable`; `thread` additionally needs proof of a valid physical thread context for the child. `spawn thread` satisfies `thread` and `cancellable` but not `task`. `spawn process` provides no logical task or cancellable thread by itself; a `thread` requirement must be proved from the child's bootstrap and target contracts. Spawn arguments and callable/capture preparation evaluated in the creator keep the creator's requirements; operations inside a spawned block body belong to the child. Eager and deferred thread creation are checked alike, and no creation-failure error variant is added for missing static context proof. MD-128, MD-129 and MD-133 are unaffected.

§ 24A(11) **Contract categories (§ 9).** Invocation requirements (`require(...)`), verified guarantees (`@noAlloc`, `@noPanic`, `@noBlock` and other verified attributes) and the body form are distinct; attributes are not requirements and `require(cancellable)` is not `@noPanic` or `@noBlock`. `MayAllocate` and `RequiresAllocationContext` stay separate facts; no `require(allocator)` or other entry is approved. Future entries need their own satisfaction, inference, normalization, compatibility and diagnostic rules; `require(y != 0)` and `require(feature(...))` are not legal and argument preconditions or Design by Contract are outside this decision.

---

## § 25. Cancellation points

**Governance tags:** `concurrency.cancellation-v2`

§ 25(1) A cancellation point is an operation boundary at which a pending current-execution cancellation request may compete with the operation's normal commit.

§ 25(2) Canonical examples may include:

- `await`;
- cancellation-aware `join`;
- blocking channel send/receive;
- waiting `select`;
- mutex acquisition;
- timer waits;
- I/O waits;
- explicit runtime/task yield where the owning rule defines cancellation awareness;
- target-declared interruptible waits.

§ 25(3) Inclusion in this list does not invent an API surface for the owning operation.

§ 25(4) The owning operation rulebook defines what state constitutes successful operation commit.

---

## § 26. Universal cancellation-point commit rule

**Governance tags:** `concurrency.cancellation-v2`, `analysis.semantic-ir-v2`

§ 26(1) A cancellation point must resolve to exactly one committed source-level outcome.

§ 26(2) Before operation commit, current-execution cancellation may win and no partial operation result/ownership transfer may remain.

§ 26(3) After operation commit, the committed operation outcome and its ownership consequences exist and must be cleaned up normally before terminal cancellation can complete.

§ 26(4) Runtime wakeup races must not produce both outcomes.

§ 26(5) Runtime wakeup races must not produce neither outcome while losing source-visible ownership/state.

---

## § 27. Mutex acquisition cancellation

**Governance tags:** `concurrency.cancellation-v2`, `concurrency.mutex-v2`

§ 27(1) For ordinary `Mutex[T].Lock()`, lock acquisition and current-execution cancellation compete.

§ 27(2) If cancellation commits before acquisition, no `MutexGuard[T]` exists.

§ 27(3) If acquisition commits first, the caller owns the guard.

§ 27(4) If terminal cancellation follows successful acquisition, ordinary deterministic cleanup must destroy/release the guard.

§ 27(5) `Mutex[T].Lock(context: ref Context)` additionally has a separate operation-local Context cancellation/deadline outcome according to `mutex.md`.

---

## § 28. Channel and select commit discipline

**Governance tags:** `concurrency.cancellation-v2`

§ 28(1) Cancellation must not partially transfer a channel message.

§ 28(2) If cancellation wins before a channel send/receive commit, message ownership remains according to the pre-commit channel state defined by `channels.md`.

§ 28(3) If current-execution cancellation wins before a `select` branch commits:

- no branch may consume ownership;
- no message may be transferred;
- no task/thread result may be consumed;
- losing registrations must be removed;
- current-execution cancellation cleanup proceeds.

§ 28(4) If a select branch commits first, that branch's ordinary ownership/result semantics apply before later cancellation cleanup.

§ 28(5) The potentially waiting ordinary-channel operations are `Send`,
`Send(message, lifetime)`, `SendRevocable`, and `Receive`.

§ 28(6) `TrySend`, `TryReceive`, `Share`, `MessageTicket.Revoke`, `Discard`,
`Statistics`, and endpoint `Close` are non-waiting operations.

§ 28(7) Sec 0.1 channel APIs do not accept `ref Context`. A `Context` in
lexical scope does not implicitly cancel a channel wait; operation wait
timeouts are composed explicitly through `select` and `after`.

§ 28(8) If current-execution cancellation wins before channel commit, no
message transfer, ticket creation, receive removal, statistics increment, or
terminal-state change occurs. If channel commit wins first, its exact result
and ownership effects remain committed and later cancellation cannot roll them
back.

§ 28(9) `Task.Current().CancelRequested` and
`Thread.Current().CancelRequested` are ordinary `bool` observations, not
selectable operations. A waiting select already participates in
current-execution cancellation without exposing cancellation as a branch
result.

---

## § 29. Await and join cancellation discipline

**Governance tags:** `concurrency.cancellation-v2`, `concurrency.tasks-v2`

§ 29(1) Cancellation of the waiting execution is distinct from cancellation of the execution being awaited/joined.

§ 29(2) If caller cancellation wins before wait/join commit, the caller's terminal cancellation path proceeds and the owning handle/result state must remain valid according to the owning await/join rule.

§ 29(3) If completion/wait commit wins first, the completion outcome and ownership transfer are real and must not be rolled back by a later cancellation observation.

§ 29(4) A backend must not lose an owning task/thread handle in the race between completion and caller cancellation.

§ 29(5) For `await`, `await.md` owns the pre-commit caller-cancellation
cleanup: retain lifecycle ownership, invoke `RequestCancel()` on the awaited
task, wait for terminal state, destroy the terminal outcome exactly once, and
then complete caller cancellation. This consuming-await rule does not change
the distinct owning-handle semantics of `join`.

---

## § 30. Exact `TaskOutcome[T]` cancellation category

**Governance tags:** `concurrency.cancellation-v2`, `concurrency.tasks-v2`

§ 30(1) The task terminal outcome type used by this rulebook is:

```sec
type TaskOutcome[T] union {
    // The task function returned normally.
    Completed(T)

    // Cooperative task cancellation committed terminally.
    Cancelled

    // Panic escaped the task boundary under the selected panic policy.
    Panicked(PanicInfo)

    // The already-created task failed at the execution/runtime layer.
    Failed(TaskError)
}
```

§ 30(2) A cancelled task has no `T` payload.

§ 30(3) A task returning `Err(E)` as its declared `T` still completes through `Completed(T)` rather than `Cancelled`.

§ 30(4) A cancellation request that was never terminally committed does not force the final outcome to `Cancelled`.

---

## § 31. Relevant `ThreadStatus` terminal category

**Governance tags:** `concurrency.cancellation-v2`

§ 31(1) The current thread status model materially referenced by cancellation is:

```sec
enum ThreadStatus {
    // Deferred thread exists but has not begun its callable.
    Created

    // The callable is executing or runnable/waiting as a live thread.
    Running

    // The callable returned normally.
    Completed

    // Cooperative thread cancellation committed terminally.
    Cancelled

    // The thread terminated through Sec panic under the applicable policy.
    Panicked

    // Unsafe/platform-level abnormal termination prevented normal Sec completion.
    Terminated
}
```

§ 31(2) Backend-private states may exist but must map to the canonical source-visible states where the thread API exposes status.

§ 31(3) `Cancelled` is distinct from `Terminated`.

---

## § 32. Cancellation request does not determine final outcome

**Governance tags:** `concurrency.cancellation-v2`

§ 32(1) `RequestCancel()` records a cooperative request; it does not immediately assign the terminal outcome.

§ 32(2) If an execution returns normally before terminal cancellation commits, its normal completion category wins.

§ 32(3) If panic commits before cancellation termination, the panic outcome is not rewritten merely because cancellation had been requested.

§ 32(4) Once one terminal execution outcome commits, later cancellation requests have no effect on that terminal category.

---

## § 33. Context capability model

**Governance tags:** `concurrency.context-v1`, `concurrency.cancellation-v2`

§ 33(1) Sec separates Context observation/propagation from cancellation authority.

§ 33(2) `Context` is the observation/propagation capability.

§ 33(3) `ContextSource` owns cancellation authority for one derived Context branch.

§ 33(4) `Context` itself does not expose `Cancel()`.

§ 33(5) This separation prevents a callee that only receives `ref Context` from cancelling its caller's Context branch.

---

## § 34. Exact `ContextError`

**Governance tags:** `concurrency.context-v1`, `tooling.cancellation-v2`

§ 34(1) The exact source-visible error type is:

```sec
enum ContextError error {
    Cancelled
    // Explicit or inherited Context cancellation committed first.

    DeadlineExceeded
    // The effective Context deadline expired first.
}
```

§ 34(2) The variants have no payload.

§ 34(3) `ContextError` is an operation-level typed error for Context-aware APIs.

§ 34(4) `ContextError.Cancelled` is not the same event as terminal cancellation of the current task/thread.

---

## § 35. Exact `Context` declaration

**Governance tags:** `concurrency.context-v1`, `analysis.transferability`

§ 35(1) The canonical Context declaration shape is:

```sec
@noCopy
type Context
// Owns one Context identity when held as a standalone value.
// May be moved to another owner.
// May be borrowed as `ref Context`.
// Cannot be copied.
// Does not grant cancellation authority.
// Parent/cancellation/deadline runtime state is representation-private.
```

§ 35(2) A standalone owned Context is destroyed exactly once by its final owner.

§ 35(3) Borrowing a Context does not duplicate its identity.

§ 35(4) A Context owned inside `ContextSource` cannot be moved out.

---

## § 36. Exact `ContextSource` declaration

**Governance tags:** `concurrency.context-v1`, `analysis.transferability`

§ 36(1) The canonical ContextSource declaration shape is:

```sec
@noCopy
type ContextSource
// Owns exactly one derived child Context.
// Owns cancellation authority for that child branch.
// Owns associated deadline/timer/cancellation registrations.
// Cannot be copied.
```

§ 36(2) Moving a ContextSource transfers all of those ownership responsibilities.

§ 36(3) The source-owned Context cannot be extracted as an owned value.

§ 36(4) A derived ContextSource has a lifetime dependency on its parent Context and cannot outlive it.

---

## § 37. Exact Context public surface

**Governance tags:** `concurrency.context-v1`, `tooling.cancellation-v2`

§ 37(1) The canonical source-visible Context surface is:

```sec
impl Context {
    static fn Background() Context
    // Creates a standalone root Context with no parent cancellation
    // and no effective deadline.

    fn WithCancel() ContextSource
    // Creates a cancellable child Context owned by the returned source.

    fn WithTimeout(timeout: duration) ContextSource
    // Creates a child with an effective monotonic deadline derived from
    // timeout and bounded by an earlier parent deadline.

    fn WithDeadline(deadline: Instant) ContextSource
    // Creates a child with the requested absolute monotonic deadline,
    // bounded by an earlier parent deadline.

    IsCancelled bool
    // True after this Context reaches a terminal Context state through
    // explicit/inherited cancellation or deadline expiration.

    Error Option[ContextError]
    // None while active.
    // Some(Cancelled) when cancellation committed first.
    // Some(DeadlineExceeded) when effective deadline expiration committed first.

    Deadline Option[Instant]
    // Effective absolute monotonic deadline, or None when unbounded.
}
```

§ 37(2) `Context` intentionally has no `Cancel()` method.

§ 37(3) Context derivation returns one `ContextSource`; Sec 0.1 does not use Go-style multiple return values.

---

## § 38. Exact ContextSource public surface

**Governance tags:** `concurrency.context-v1`, `tooling.cancellation-v2`

§ 38(1) The canonical source-visible ContextSource surface is:

```sec
impl ContextSource {
    Context ref Context
    // Borrowed access to the child Context owned by this source.
    // The borrow cannot outlive ContextSource.

    fn Cancel() void
    // Requests cancellation of the still-existing child Context.
    // Idempotent.
    // Does not destroy Context or ContextSource.
}
```

§ 38(2) The `Context` property does not copy or transfer the child Context.

§ 38(3) `Cancel()` does not require `ContextSource` to be held in a mutable binding merely to request cancellation.

---

## § 39. Context terminal states and first-wins cause

**Governance tags:** `concurrency.context-v1`, `concurrency.cancellation-v2`

§ 39(1) A Context begins active with:

```text
IsCancelled == false
Error == None
```

§ 39(2) Exactly one terminal Context cause may commit:

```text
Cancelled
DeadlineExceeded
```

§ 39(3) The first terminal cause to commit wins.

§ 39(4) Once terminal, `Context.Error` is immutable.

§ 39(5) A later `ContextSource.Cancel()` does not rewrite `DeadlineExceeded` to `Cancelled`.

§ 39(6) A later deadline expiry does not rewrite `Cancelled` to `DeadlineExceeded`.

§ 39(7) Concurrent parent cancellation, child cancellation, and deadline expiry must resolve to one committed terminal cause.

---

## § 40. ContextSource cancellation versus destruction

**Governance tags:** `concurrency.context-v1`

§ 40(1) `ContextSource.Cancel()` changes state of a still-existing Context.

§ 40(2) After `Cancel()`, the ContextSource remains alive until normally moved/destroyed.

§ 40(3) After `Cancel()`, the child Context remains alive and observable as terminal.

§ 40(4) Destroying ContextSource destroys its owned child Context and associated registrations/resources.

§ 40(5) Destruction is not itself a cancellation event.

§ 40(6) No valid borrow or derived child may outlive the lifetime whose destruction would invalidate it.

---

## § 41. Context hierarchy propagation

**Governance tags:** `concurrency.context-v1`

§ 41(1) Parent Context cancellation propagates to still-existing descendants.

§ 41(2) Cancelling a child source does not cancel its parent.

§ 41(3) Cancelling one child branch does not cancel sibling branches merely because they share a parent.

§ 41(4) Parent propagation participates in the same first-terminal-cause commit rule as direct child cancellation and deadline expiry.

§ 41(5) A child source cannot outlive its parent Context.

---

## § 42. Context deadline inheritance

**Governance tags:** `concurrency.context-v1`

§ 42(1) A child Context cannot have an effective deadline later than an earlier effective parent deadline.

§ 42(2) `WithDeadline(requested)` therefore uses the earlier of the requested deadline and the parent's effective deadline, if any.

§ 42(3) `WithTimeout(timeout)` derives a requested monotonic deadline from current monotonic time plus `timeout`, then applies the same parent bound.

§ 42(4) `Context.Deadline` reports the effective deadline after this inheritance rule.

---

## § 43. `duration` and `Instant`

**Governance tags:** `concurrency.context-v1`, `frontend.temporal-duration`, `frontend.temporal-instant`

§ 43(1) Relative Context timeouts use the existing compiler-known/core temporal type:

```sec
duration
```

§ 43(2) Sec must not introduce a competing uppercase `Duration` solely for cancellation/context APIs.

§ 43(3) A compatible time-unit expression such as:

```sec
5<s>
250<ms>
```

may materialize as `duration` through the canonical conversion of `rules/types/temporal.md` § 5 (MD-015).

§ 43(4) Absolute Context deadlines use:

```sec
type Instant
// Copyable point on Sec's monotonic runtime clock.
// Not wall-clock/calendar/UTC/local time.
// Representation is compiler/runtime-private.
```

§ 43(5) General Instant acquisition/arithmetic belongs to the temporal rulebook.

---

## § 44. Context cancellation is operation-local

**Governance tags:** `concurrency.context-v1`, `concurrency.cancellation-v2`

§ 44(1) Passing `ref Context` to an operation does not make that Context the current task/thread cancellation identity.

§ 44(2) If the supplied Context reaches `Cancelled` first, a Context-aware operation returns/propagates `ContextError.Cancelled` according to its declared result type.

§ 44(3) If its deadline reaches `DeadlineExceeded` first, the operation returns/propagates `ContextError.DeadlineExceeded`.

§ 44(4) Neither outcome terminally cancels the current task/thread merely because the Context ended.

§ 44(5) The current execution's own cancellation remains independently active unless the owning operation rule explicitly states otherwise.

---

## § 45. Current-execution cancellation versus Context cancellation

**Governance tags:** `concurrency.cancellation-v2`, `concurrency.context-v1`

§ 45(1) Consider a Context-aware wait inside a cancellable task.

§ 45(2) At least these semantic outcomes may compete:

```text
operation commit
supplied Context cancellation/deadline
current task cancellation
```

§ 45(3) Operation commit produces the operation's normal committed result.

§ 45(4) Context cancellation/deadline produces the operation's declared Context error/alternate result.

§ 45(5) Current task cancellation does not return a `ContextError`; it proceeds through terminal task cancellation cleanup.

§ 45(6) The runtime must preserve exactly one committed winner when those events race.

---

## § 46. Context observation

**Governance tags:** `concurrency.context-v1`, `tooling.cancellation-v2`

§ 46(1) User code may observe:

```sec
ctx.IsCancelled
ctx.Error
ctx.Deadline
```

§ 46(2) `IsCancelled == false` requires `Error == None`.

§ 46(3) `Error != None` requires `IsCancelled == true`.

§ 46(4) `Deadline != None` does not imply that the Context has expired.

§ 46(5) A Context can be explicitly cancelled before its deadline.

---

## § 47. Context memory ordering

**Governance tags:** `concurrency.context-v1`, `concurrency.memory-model-v2`

§ 47(1) Context terminal-state publication must be safely observable by wait registrations and Context observers.

§ 47(2) `IsCancelled` and `Error` must represent one coherent terminal-state commit.

§ 47(3) The runtime must not expose `IsCancelled == true` while `Error == None` after the terminal commit becomes observable.

§ 47(4) Context cancellation/deadline publication does not automatically publish unrelated mutable application data.

§ 47(5) Application data still requires a canonical synchronization mechanism.

---

## § 48. Context usage pattern

**Governance tags:** `concurrency.context-v1`

§ 48(1) A function that only observes/propagates Context should normally accept:

```sec
ctx: ref Context
```

§ 48(2) The borrow grants no cancellation authority.

§ 48(3) A callee may derive its own branch:

```sec
fn QueryDatabase(ctx: ref Context) Result[Data, DatabaseError] {
    let query := ctx.WithTimeout(2<s>)
    return ExecuteQuery(query.Context)
}
```

§ 48(4) The callee may cancel its own derived branch through `query.Cancel()`.

§ 48(5) It cannot cancel the borrowed parent Context because `Context` has no `Cancel()`.

---

## § 49. Standalone and source-owned Context lifetimes

**Governance tags:** `concurrency.context-v1`, `analysis.transferability`

§ 49(1) `Context.Background()` returns a standalone owned Context.

§ 49(2) A standalone Context may be moved to another owner using ordinary Sec move rules.

§ 49(3) A Context created as the child owned by ContextSource is externally borrow-only.

§ 49(4) `source.Context` has static type:

```sec
ref Context
```

§ 49(5) The child cannot be moved out of its source.

§ 49(6) An object that must own a cancellable derived branch should own/move the complete ContextSource.

---

## § 50. Panic and cancellation

**Governance tags:** `concurrency.cancellation-v2`

§ 50(1) Panic is not cancellation.

§ 50(2) A cancellation request does not rewrite an already committed panic outcome.

§ 50(3) A panic that commits before terminal cancellation remains a panic outcome according to the owning task/thread panic boundary.

§ 50(4) Cleanup behavior follows the panic/destruction rulebooks for panic and this rulebook for cancellation; neither concept may be silently collapsed into the other.

---

## § 51. Unsafe hard termination

**Governance tags:** `concurrency.cancellation-v2`, `compiler.platform-model`

§ 51(1) Safe Sec has no general forced task/thread kill operation as part of cooperative cancellation.

§ 51(2) Platform-specific unsafe thread termination, where provided, is not `RequestCancel()` and does not produce cooperative `Cancelled` semantics.

§ 51(3) Unsafe termination may skip ordinary `defer`, deterministic destruction, mutex release, thread-local cleanup, and normal result construction according to the owning platform/thread contract.

§ 51(4) This rulebook does not define a process-kill API.

---

## § 52. Detached execution

**Governance tags:** `concurrency.cancellation-v2`, `compiler.platform-model`

§ 52(1) Detaching an execution handle does not make cooperative cancellation meaningless.

§ 52(2) Runtime/program-lifecycle policy may request cancellation of detached work during shutdown according to the owning runtime model.

§ 52(3) Safe Sec must not silently convert an ignored cancellation request into unsafe hard termination.

§ 52(4) A profile may diagnose or define shutdown policy for non-cooperative detached work.

---

## § 53. Blocking FFI

**Governance tags:** `concurrency.cancellation-v2`, `compiler.platform-model`

§ 53(1) A foreign blocking call may be non-cancellable.

§ 53(2) Its FFI/platform contract must describe whether and how the wait can be interrupted safely.

§ 53(3) The compiler/runtime must not assume that an arbitrary foreign call is a cancellation point.

§ 53(4) A cancellation-dependent execution path entering a known non-cancellable indefinite foreign wait may produce a diagnostic according to analysis/profile policy.

---

## § 54. Interrupt context

**Governance tags:** `concurrency.cancellation-v2`, `compiler.platform-model`

§ 54(1) An ISR is not cancelled through the ordinary task/thread cancellation model.

§ 54(2) An ISR may request cancellation of another task/thread only when the selected target declares the concrete request operation ISR-safe.

§ 54(3) Portable `RequestCancel()` must not be assumed ISR-safe on every target.

§ 54(4) Bare `cancel` inside ISR context is invalid because ISR completion follows interrupt rules rather than task/thread cancellation semantics.

---

## § 55. Cancellation and structured task relationships

**Governance tags:** `concurrency.cancellation-v2`, `concurrency.tasks-v2`

§ 55(1) Parent-child task cancellation propagation is governed by the structured-concurrency/task relationship rules.

§ 55(2) This rulebook requires any such propagation to preserve distinct task identities and cooperative cancellation semantics.

§ 55(3) Physical threads do not automatically inherit task cancellation merely because a task created/spawned them.

§ 55(4) Explicit shared `Context` propagation is a separate mechanism from task-identity cancellation propagation.

§ 55(5) **Amendment MD-129 (2026-11-24, Sec 0.1 and later compatible versions, `rules/corrections/applied/MD-129-spawn-parent-cancellation-correction-2026-11-24-20261124.md` §§ 1–2).** Cancellation inheritance applies only to a real structured parent-child cancellation link (`spawn.md` § 9, `tasks.md` § 8, `concurrency.md` § 19), never to lexical nesting, handle ownership alone, a physical worker identity or the use of task operations. A task spawned from ordinary program entry or from a native thread without a current logical task has no task parent, and no root `TaskContext` is invented. Parent and child keep separate identities and request states: an inherited request sets the child's own request state and never aliases the parent's flag. There is no detached-at-creation syntax; a later detach is not evidence that the child was parentless at creation, and moving or returning a handle transfers the lifecycle obligation without reparenting.

§ 55(6) **Amendment MD-129 (2026-11-24, Sec 0.1 and later compatible versions, `rules/corrections/applied/MD-129-spawn-parent-cancellation-correction-2026-11-24-20261124.md` § 3).** An existing cooperative request on the parent is not by itself a reason for `spawn` or `spawn task` to fail. If the parent has a committed request when a linked child is successfully established, including one made before the spawn began, the child begins with its own request pending. The request is established before the child can observe its cancellation state or execute a cancellation point, so it never observes `Task.Current().CancelRequested == false` because publication was delayed. The raw result stays `Result[Task[T], TaskSpawnError]` with no `Cancelled` creation error, and creation never fabricates `TaskOutcome.Cancelled`: the child may cancel cooperatively, return normally before terminal cancellation commits, panic or fail under the ordinary outcome model. A pending request on the parent is not a terminated parent and permits no source execution after terminal cancellation. Without a parent-child link, a request on the creator does not set the child's state; independently delivered requests and shared `Context` state keep their own rules.

§ 55(7) **Amendment MD-129 (2026-11-24, Sec 0.1 and later compatible versions, `rules/corrections/applied/MD-129-spawn-parent-cancellation-correction-2026-11-24-20261124.md` § 4).** A successful `detach` (`tasks.md` § 22) also breaks the structured cancellation propagation link of that task. Requests committed to the child before the detach boundary stay pending while the child is nonterminal; detach never clears, undoes, shields or replaces them. A request committed by the former parent after detach does not reach the child through the removed link; independently valid mechanisms such as a shared `Context` are unaffected. Detach gives no completion synchronization or immediate termination, and `detach worker discard` stays required where the result needs disposal.

```sec
let worker := try spawn Work()   // parent already requested: the child starts requested
detach worker                    // breaks future propagation, keeps the received request
```

§ 55(8) **Amendment MD-129 (2026-11-24, Sec 0.1 and later compatible versions, `rules/corrections/applied/MD-129-spawn-parent-cancellation-correction-2026-11-24-20261124.md` § 5).** Link establishment and initial request publication are coordinated with concurrent parent cancellation so no request is lost between checking the parent and publishing the link. A parent request committed before successful link establishment is in the child's initial state; one committed afterwards while the link is active follows ordinary propagation; failed creation publishes no lasting relationship. Detach and propagation have a well-defined logical order: a request received while the link was active is never revoked because detach won a later race, a request owed by the still-active link is not lost because propagation work was queued when detach occurred, and a request committed after link removal does not travel through it. The mechanism (synchronized publication, a recorded owed request or an equivalent) is not prescribed. Terminal outcomes stay immutable, and request and relationship transitions are data-race-free under § 18 and `concurrency_memory_model.md` without becoming a general publication mechanism.

§ 55(9) **Amendment MD-129 (2026-11-24, Sec 0.1 and later compatible versions, `rules/corrections/applied/MD-129-spawn-parent-cancellation-correction-2026-11-24-20261124.md` §§ 6–7).** No cross-kind inheritance exists: `spawn thread` from a requested task gives the thread no task request (`Thread.Current().CancelRequested` observes the thread identity, § 55(3)), `spawn process` from a requested task terminates or resolves nothing (`processes.md` § 29), and an already cancelled or expired `Context` is handled through its own contract without manufacturing execution cancellation. Ownership follows MD-128 (`spawn.md` § 10(8)): a genuine creation failure rolls back pending transfers, while success commits exactly once even for a requested child, and the child's later cooperative cancellation rolls nothing back. A successful creation always has its ordinary handle lifecycle obligation, which a pending request never replaces. Await caller-cancellation cleanup (`await.md` §§ 17–24, 52) is a separate explicit request. `spawn` is not a cancellation point.

---

## § 56. Semantic analysis

**Governance tags:** `frontend.cancellation-v2`, `frontend.cancellation-effect-v1`

§ 56(1) Sema must validate the context in which bare `cancel` appears.

§ 56(2) Sema must infer and propagate the current-cancellable-execution effect.

§ 56(3) Sema must preserve the effect through direct calls, generic instantiation, recursion, callable values, and indirect calls.

§ 56(4) Sema must distinguish current-task requirements from physical-thread requirements.

§ 56(5) Sema must distinguish task cancellation identity from thread cancellation identity.

§ 56(6) Sema must validate `Context`/`ContextSource` `@noCopy` ownership and borrow lifetimes.

§ 56(7) Sema must reject moving `ContextSource.Context` out of its source.

§ 56(8) Sema must validate Context child lifetime dependency on parent Context.

§ 56(9) Sema must preserve cancellation-point commit ownership facts required by the owning operation.

§ 56(10) Sema must reject `cancel` inside `defer`.

§ 56(11) **Amendment MD-138 (2026-11-24, Sec 0.1.1, `rules/corrections/applied/MD-138-callable-execution-requirements-v0.1.1-correction-2026-11-24-20261124.md` § 10(2)–(3)).** Sema stores normalized requirements in callable type identity, validates explicit contracts, infers requirements over the relevant call edges, verifies conversions and checks invocation boundaries (§ 24A). Unresolved facts are never a proven empty set. Function and closure type construction, overload resolution, interface conformance, generic substitution, monomorphization, exported signatures, callable storage and indirect dispatch consume the same facts; a changed requirement set is a semantic dependency change for incremental analysis and caches, and no optimization or analysis-depth setting disables the mandatory checks.

---

## § 57. Semantic IR

**Governance tags:** `analysis.semantic-ir-v2`, `semantic-ir.cancellation-v2`

§ 57(1) Semantic IR must preserve cancellation semantics explicitly enough that lowering does not rediscover them from names or ordinary returns.

§ 57(2) It must preserve at least:

- task versus thread cancellation identity;
- cancellation request operation;
- current task/thread observation;
- bare `cancel` terminal operation;
- inferred callable cancellation-context requirement;
- cancellation-point registration and commit outcome;
- current-execution cancellation versus Context outcome;
- Context identity;
- ContextSource identity/ownership;
- Context terminal cause;
- Context parent/child relation;
- deadline registration;
- source provenance;
- selected target/profile capability constraints.

§ 57(3) Concrete Semantic IR opcode names are owned by `semantic_ir.md`.

§ 57(4) This rulebook does not require the legacy hard-coded opcode list as the canonical vocabulary.

§ 57(5) Cancellation must remain distinct from return, panic, error propagation, timeout, and unsafe termination.

§ 57(6) **Amendment MD-138 (2026-11-24, Sec 0.1.1, `rules/corrections/applied/MD-138-callable-execution-requirements-v0.1.1-correction-2026-11-24-20261124.md` § 10(4)).** Semantic IR preserves the requirements of callable values, their invocation contracts, call-site context proofs and spawn-boundary validation; the verifier rejects a call or assignment that would erase or fail to satisfy a contract. No runtime requirement mask, extra source parameter, hidden ABI parameter or dynamic assertion is mandated.

---

## § 58. Lowering and runtime

**Governance tags:** `lowering.cancellation-v2`, `compiler.platform-model`

§ 58(1) Lowering consumes validated cancellation/context Semantic IR plus the selected immutable `CompilationPlan`.

§ 58(2) Lowering must preserve idempotent cancellation requests.

§ 58(3) Lowering must preserve exactly-one terminal execution outcome.

§ 58(4) Lowering must preserve exactly-one Context terminal cause.

§ 58(5) Lowering must preserve cancellation-point commit atomicity.

§ 58(6) Lowering must preserve deterministic cleanup before cooperative terminal cancellation completes.

§ 58(7) Lowering must preserve task/thread identity distinction even when both map to the same operating-system primitive on a specific target.

§ 58(8) Runtime representation must not require a garbage collector merely to satisfy Context/cancellation semantics.

---

## § 59. LSP hover and completion

**Governance tags:** `tooling.cancellation-v2`

§ 59(1) Hover for `TaskContext` and `ThreadContext` must expose their non-owning role and exact `CancelRequested bool` property.

§ 59(2) Hover for `Task[T].RequestCancel` and `Thread[T].RequestCancel` must expose `fn RequestCancel() void` and idempotent cooperative semantics.

§ 59(3) Hover for `Context`, `ContextSource`, and `ContextError` must expose the exact declarations and ownership rules defined by this book.

§ 59(4) Completion on `Context` must not offer `Cancel`.

§ 59(5) Completion on `ContextSource` must offer `Context` and `Cancel`.

§ 59(6) Completion/hover must use `CancelRequested`, not legacy `cancelRequested`.

§ 59(7) Tooling may display an inferred callable effect such as "requires cancellable execution context" without requiring source annotation.

§ 59(8) **Amendment MD-138 (2026-11-24, Sec 0.1.1, `rules/corrections/applied/MD-138-callable-execution-requirements-v0.1.1-correction-2026-11-24-20261124.md` § 10(5)–(6)).** Hover and signature help show declared or inferred requirements in clause spelling, marking inferred ones as not source text and keeping them apart from other effects. Completion inside a clause offers exactly `task`, `thread` and `cancellable`. The formatter keeps clause attachment, presence, entry order and meaning, prints `require(task, thread)` with ordinary comma spacing and list conventions, and never adds an inferred clause, removes a stronger or redundant entry or drops a parenthesis needed for attachment.

---

## § 60. Diagnostics

**Governance tags:** `tooling.cancellation-v2`, `frontend.cancellation-effect-v1`

§ 60(1) Suggested diagnostics include:

```text
cancel is not valid outside a cancellable task or explicit thread context
```

```text
cancel is not valid inside defer cleanup
```

```text
call to ProcessBatch requires a current cancellable task or thread execution context
```

```text
Task.Current() requires a current logical task context
```

```text
ContextSource.Context is borrowed from its ContextSource and cannot be moved out
```

```text
Context does not grant cancellation authority; cancel the owning ContextSource instead
```

§ 60(2) Diagnostics must distinguish logical task cancellation from physical thread cancellation.

§ 60(3) Diagnostics must distinguish Context cancellation/deadline from terminal current-execution cancellation.

§ 60(4) Diagnostics must not suggest volatile access as a cancellation-state synchronization mechanism.

§ 60(5) **Amendment MD-138 (2026-11-24, Sec 0.1.1, `rules/corrections/applied/MD-138-callable-execution-requirements-v0.1.1-correction-2026-11-24-20261124.md` § 10(7)).** Diagnostics name the missing or incompatible requirement, give the originating operation or callee chain for inferred requirements when available, and suggest a correction, for example:

```text
callable requires a current logical task; destination fn() void does not preserve that requirement
help: use fn() void require(task) as the callable type

function Invalid declares require(cancellable), but Task.Current() requires a current logical task
help: declare require(task) or remove the task-dependent operation

unknown execution requirement 'allocator' in Sec 0.1.1
help: supported execution requirements are task, thread, and cancellable
```

These texts are illustrative; identifiers follow the diagnostics registry.

---

## § 61. Restrictions

**Governance tags:** `concurrency.cancellation-v2`, `concurrency.context-v1`

§ 61(1) Cancellation must not asynchronously tear down ordinary safe execution at an arbitrary instruction.

§ 61(2) `cancel` must not be used outside a valid cancellable execution context.

§ 61(3) `cancel` must not be used inside `defer`.

§ 61(4) A cancellation request must not consume the owning task/thread handle.

§ 61(5) Cancellation request state must not be treated as synchronization for unrelated application data.

§ 61(6) Task cancellation must not be conflated with physical thread cancellation.

§ 61(7) Context cancellation must not be conflated with terminal task/thread cancellation.

§ 61(8) `Context` must not expose `Cancel()`.

§ 61(9) `Context` and `ContextSource` must not be copied.

§ 61(10) Context terminal cause must not change after first terminal commit.

§ 61(11) ContextSource destruction must not be treated as an implicit cancellation event.

§ 61(12) The inferred cancellation effect must not be erased through callable indirection.

§ 61(13) Backend/native cancellation primitives must not silently change the portable result/outcome model.

---

## § 62. Governance

**Governance tags:** `concurrency.cancellation-v2`, `concurrency.context-v1`, `frontend.cancellation-v2`, `frontend.cancellation-effect-v1`, `tooling.cancellation-v2`, `semantic-ir.cancellation-v2`, `lowering.cancellation-v2`, `concurrency.tasks-v2`, `concurrency.mutex-v2`, `concurrency.memory-model-v2`, `analysis.transferability`, `analysis.semantic-ir-v2`, `compiler.platform-model`

§ 62(1) Mutable implementation information for cancellation must be maintained
in `governance/concurrency_cancellation.yaml`. Related tagged areas remain
owned by their respective governance fragments.

§ 62(2) The primary cancellation integration is `concurrency.cancellation-v2`.

§ 62(3) The canonical Context ownership/cancellation model is governed here through `concurrency.context-v1` until a dedicated Context rulebook supersedes this ownership explicitly.

§ 62(4) The compiler-inferred callable requirement is governed through `frontend.cancellation-effect-v1`.

§ 62(5) Tasks and threads retain ownership of their broader lifecycle/status APIs while synchronizing the exact cancellation-related extensions defined here.

§ 62(6) Cross-rulebook synchronization required by this revision is tracked in the accompanying corrections document.
