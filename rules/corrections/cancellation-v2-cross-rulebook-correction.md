# Correction — Cancellation v2 cross-rulebook synchronization

- **Status:** Pending synchronization
- **Created:** 2026-09-06
- **Last updated:** 2026-09-06
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `0f5027d`
- **Primary owning rulebook:** `rules/concurrency/cancellation.md`
- **Governance fragment:** `implementation-status-cancellation.yaml`
- **Classification:** Normative synchronization of decided cancellation/context semantics

---

## 1. Canonical task/thread cancellation surface

Synchronize compiler/core, LSP, tests and related rulebooks to:

```sec
impl Task[T] {
    fn RequestCancel() void
}

impl Thread[T] {
    fn RequestCancel() void
}

impl Task {
    static fn Current() TaskContext
}

impl TaskContext {
    CancelRequested bool
}

impl Thread {
    static fn Current() ThreadContext
}

impl ThreadContext {
    CancelRequested bool
}
```

Use canonical public CamelCase `CancelRequested`.

Replace legacy lowercase examples such as:

```sec
task.cancelRequested
Thread.Current().cancelRequested
```

Do not make Task and Thread cancellation identities interchangeable.

---

## 2. `rules/concurrency/tasks.md`

Add the exact compiler-known/source-visible current-task surface:

```sec
type TaskContext

impl Task {
    static fn Current() TaskContext
}

impl TaskContext {
    CancelRequested bool
}
```

Document that:

- TaskContext is immutable/non-owning;
- it follows the logical task across worker migration;
- it owns no Task[T] lifecycle/result capability;
- `Task.Current()` requires a current logical task;
- `Task.Current()` is not synthesized from a physical worker thread;
- `Task[T].RequestCancel()` is idempotent, non-consuming and does not resolve lifecycle obligation.

Keep `TaskOutcome[T].Cancelled` as the terminal cancellation category.

---

## 3. `rules/concurrency/threads.md`

Synchronize legacy lowercase observation:

```sec
Thread.Current().CancelRequested
```

and the ThreadContext extension:

```sec
impl ThreadContext {
    CancelRequested bool
}
```

Keep existing ThreadContext identity/name/platform metadata owned by `threads.md`.

Do not remove that metadata merely because cancellation.md specifies only the cancellation extension.

Ensure `Thread[T].RequestCancel()` is cooperative, idempotent, non-consuming and distinct from unsafe `Terminate()`.

Keep `ThreadStatus.Cancelled` distinct from `ThreadStatus.Terminated`.

---

## 4. `rules/concurrency/mutex.md`

Add the newly locked Context observation member:

```sec
impl Context {
    Error Option[ContextError]
}
```

Document first-terminal-cause-wins:

```text
active
  -> Cancelled
  -> DeadlineExceeded
```

Once one terminal cause commits, later cancellation/deadline events must not replace it.

Keep the already-decided mutex distinction:

```text
Lock(ref Context) ContextError
    operation-local outcome

current task/thread cancellation
    terminal execution control flow
```

Do not turn current-execution cancellation into `Err(ContextError.Cancelled)`.

---

## 5. Core/source-visible declarations

Ensure real source-visible compiler-known declarations/surfaces exist for:

```sec
@noCopy
type Task[T]

@noCopy
type Thread[T]

type TaskContext

type ThreadContext

@noCopy
type Context

@noCopy
type ContextSource

enum ContextError error {
    Cancelled
    DeadlineExceeded
}
```

Do not implement these only as compiler string/name tables.

Compiler-known identity and source-visible declaration must resolve as one semantic symbol in Sema/LSP.

---

## 6. Inferred cancellable-execution effect

Add a semantic callable effect equivalent to:

```text
requires current cancellable execution
```

Rules:

- direct `cancel` implies the effect;
- calling a callable with the effect propagates it transitively;
- recursion/SCC analysis preserves it;
- generic instantiations preserve it;
- function/callable values preserve it;
- indirect calls cannot erase it;
- calling such a callable where no suitable Task/Thread execution exists is a compile-time error.

Do not require source syntax such as:

```sec
@cancellable
```

in Sec 0.1.

Add the more specific effect/fact for `Task.Current()` requiring a current logical task.

Treat `Thread.Current()` as requiring selected-target physical thread capability.

---

## 7. Parser and control flow

Keep `cancel` as the exact statement syntax:

```sec
cancel
```

It is terminating for reachability/control-flow analysis.

Reject `cancel` inside `defer` with a focused diagnostic.

Do not lower `cancel` as `return`.

Do not synthesize a normal T/Err(E) result on cancellation.

---

## 8. Cancellation-point commit integration

Synchronize blocking operation rulebooks to the universal invariant:

```text
exactly one outcome commits
```

For a current-execution cancellation race:

- before operation commit: no partial ownership/result transfer remains;
- after operation commit: committed ownership/result is real and ordinary cleanup applies before terminal cancellation completes.

Apply this to mutex, channel, select, await/join, timer/I/O waits and other declared cancellation points.

The owning operation rulebook retains the definition of its own commit point.

---

## 9. Context canonical ownership

Synchronize Context surface to:

```sec
impl Context {
    static fn Background() Context
    fn WithCancel() ContextSource
    fn WithTimeout(timeout: duration) ContextSource
    fn WithDeadline(deadline: Instant) ContextSource

    IsCancelled bool
    Error Option[ContextError]
    Deadline Option[Instant]
}

impl ContextSource {
    Context ref Context
    fn Cancel() void
}
```

Keep:

- Context/ContextSource `@noCopy`;
- no Context.Cancel();
- source-owned child Context cannot be moved out;
- ContextSource.Context borrow cannot outlive source;
- Cancel() idempotent;
- source destruction is not cancellation;
- parent cancellation propagates downward;
- child cancellation does not propagate upward;
- derived source cannot outlive parent Context;
- first Context terminal cause is immutable.

---

## 10. Memory model

Update `concurrency_memory_model.md` where needed to preserve:

- RequestCancel -> CancelRequested safe visibility;
- cancellation state is not synchronization for unrelated application data;
- Context IsCancelled/Error terminal state is one coherent publication;
- cancelled task/thread completion followed by successful await/join establishes the ordinary completion synchronization edge;
- Context cancellation does not itself publish unrelated application data.

---

## 11. Semantic IR

Update `semantic_ir.md` so it can preserve:

- Task versus Thread cancellation identity;
- cancellation request;
- current cancellation observation;
- terminal `cancel`;
- inferred callable cancellation effect;
- cancellation-point registration/commit winner;
- Context identity/source identity;
- Context parent relation;
- deadline registration;
- first-wins terminal Context cause;
- current-execution cancellation versus Context error;
- source provenance and CompilationPlan requirements.

Do not require the legacy literal opcode list unless those names independently remain canonical in semantic_ir.md.

---

## 12. LSP

Hover/completion must expose:

```text
RequestCancel
CancelRequested
Task.Current
Thread.Current
Context.Error
ContextSource.Cancel
```

Use CamelCase exactly.

Hover for TaskContext/ThreadContext must make logical task versus physical thread identity explicit.

Tooling may surface the inferred function requirement as explanatory metadata such as:

```text
requires cancellable execution context
```

without writing a source annotation.

Completion on Context must not offer Cancel.

---

## 13. Tests

Add/synchronize tests for:

- Task RequestCancel idempotence/non-consuming semantics;
- Thread RequestCancel symmetry;
- Task.Current().CancelRequested;
- Thread.Current().CancelRequested;
- task/thread identity distinction on executor threads;
- bare cancel task-first targeting;
- cancel outside cancellable execution;
- cancel inside defer rejection;
- deterministic cleanup on cancel;
- inferred direct cancellation effect;
- transitive effect propagation;
- effect preservation through callable values/indirect calls;
- Task.Current specific-context requirement;
- cancellation-point exactly-one commit;
- TaskOutcome.Cancelled distinction from Completed(Err(...));
- ThreadStatus.Cancelled distinction from Terminated;
- Context Error Option[ContextError];
- first-terminal-cause-wins;
- source destruction not cancellation;
- parent-to-child Context cancellation propagation;
- current-execution cancellation distinct from ContextError;
- cancellation request/observation memory visibility without unrelated-data publication.

---

## 14. Non-decisions

Do not invent while applying this correction:

- process cancellation/termination APIs;
- unsafe task kill;
- a generic CancellationError for terminal execution cancellation;
- an @cancellable source annotation;
- a shared Task/Thread cancellation identity;
- Context.Cancel();
- copyable Context or ContextSource;
- Go-style multiple Context returns;
- a second Duration type;
- replacement of ThreadContext's existing non-cancellation metadata.

Any such change requires a separate explicit language-design decision.
