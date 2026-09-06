# Correction — Mutex v2 cross-rulebook synchronization

- **Status:** Pending synchronization
- **Created:** 2026-09-06
- **Last updated:** 2026-09-06
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `0f5027d`
- **Primary owning rulebook:** `rules/concurrency/mutex.md`
- **Governance fragment:** `implementation-status-mutex.yaml`
- **Classification:** Normative synchronization of decided mutex/context semantics

---

## 1. Canonical mutex surface

Synchronize compiler, LSP, core, tests, examples, and related rulebooks to:

```sec
@noCopy
type Mutex[T]

@noCopy
type MutexGuard[T]

impl Mutex[T] {
    fn Lock() MutexGuard[T]
    fn TryLock() Option[MutexGuard[T]]
    fn Lock(timeout: duration) Option[MutexGuard[T]]
    fn Lock(context: ref Context) Result[MutexGuard[T], ContextError]
}
```

Remove legacy public lowercase spellings `lock` and `tryLock`.

Do not add in Sec 0.1:

```text
Lock(timeout, context)
LockError
MutexGuard.Unlock()
MutexGuard.Value
MutexGuard.Get()
```

---

## 2. Core source-visible declarations

Create or update an appropriate source-visible core declaration surface for:

```sec
@noCopy
type Mutex[T]

@noCopy
type MutexGuard[T]

@noCopy
type Context

@noCopy
type ContextSource

enum ContextError error {
    Cancelled
    DeadlineExceeded
}

type Instant
```

These declarations are opaque/compiler-known where runtime representation is implementation-private.

If the current grammar cannot yet represent declaration-only compiler-known opaque types exactly, extend the compiler/core declaration mechanism rather than replacing these types with undocumented compiler-private names.

The LSP and compiler must resolve each core declaration and compiler-known identity as one semantic symbol.

Do not expose runtime mutex handles, scheduler wait nodes, cancellation links, timer registrations, or pointer fields as ordinary public source members.

---

## 3. Context surface

Synchronize to:

```sec
impl Context {
    static fn Background() Context
    fn WithCancel() ContextSource
    fn WithTimeout(timeout: duration) ContextSource
    fn WithDeadline(deadline: Instant) ContextSource

    IsCancelled bool
    Deadline Option[Instant]
}

impl ContextSource {
    Context ref Context
    fn Cancel() void
}
```

Normative ownership:

- `Context` is `@noCopy`.
- `ContextSource` is `@noCopy`.
- `Context` itself does not expose `Cancel()`.
- `ContextSource` owns the derived child Context.
- `ContextSource` owns cancellation authority.
- `ContextSource.Context` is a borrow and cannot outlive the source.
- the source-owned child Context cannot be moved out.
- `Cancel()` is idempotent.
- `Cancel()` changes state of a still-existing Context.
- destroying ContextSource destroys child Context and registrations/resources.
- destruction is not itself a cancellation event.
- parent cancellation propagates to descendants.
- child cancellation does not propagate to the parent.
- a derived ContextSource cannot outlive the parent Context.

---

## 4. `rules/concurrency/cancellation.md`

Replace the old implementation-possibility wording around a merely internal source/token split with the now-decided public Context capability model where relevant.

Do not change terminal task/thread cancellation semantics.

Keep distinct:

```text
current execution cancellation
    terminal execution control flow

supplied Context cancellation/deadline
    operation-local ContextError for Context-aware APIs
```

For mutex waits:

- acquisition commit before current-execution cancellation -> caller owns guard;
- current-execution cancellation before acquisition commit -> terminal cancellation, no returned guard;
- Context cancellation/deadline in `Lock(ref Context)` -> typed `ContextError`, not task cancellation.

Update cancellation-point examples to canonical `Lock` spelling.

---

## 5. `rules/concurrency/concurrency_memory_model.md`

Ensure mutex synchronization uses canonical `Lock`/`TryLock` names.

Preserve:

```text
guard destruction/release
    release synchronization

later successful acquisition of same mutex identity
    acquire synchronization
```

Add/retain Context cancellation-state publication as compiler/runtime synchronization that does not publish arbitrary unrelated application memory.

Do not treat ContextSource destruction as a cancellation event.

---

## 6. `sec/core/duration.sec` and temporal rules

The repository already contains:

```sec
module core

impl duration {
}
```

Keep `duration` as the canonical timeout type.

Do not introduce a competing uppercase `Duration` solely for mutex/context APIs.

Define/complete the temporal conversion rule so a compatible time-dimension unit expression can materialize as `duration` at a call boundary.

Examples:

```sec
mutex.Lock(5<s>)
context.WithTimeout(250<ms>)
```

These resolve to the single declarations:

```sec
fn Lock(timeout: duration) Option[MutexGuard[T]]
fn WithTimeout(timeout: duration) ContextSource
```

Do not create one overload per time unit.

---

## 7. Units integration

Use canonical unit metadata to identify time-dimension quantities.

Do not hard-code `s`, `ms`, `us`, `ns`, etc. inside mutex overload resolution.

The current units library already defines `s` with dimension `[time^1]` and scaled units such as `ms`, `us`, and `ns` with the same dimension.

Whether a particular unit symbol is in scope remains governed by units/module rules.

The compiler consumes unit identity/dimension/scale from the canonical units model.

---

## 8. Monotonic `Instant`

Add a canonical source-visible compiler-known `Instant` identity required by:

```sec
fn Context.WithDeadline(deadline: Instant) ContextSource
Deadline Option[Instant]
```

`Instant` is:

- copyable;
- a point on a monotonic runtime clock;
- not UTC;
- not local time;
- not a calendar date/time;
- representation-private.

The owning temporal rulebook must define the complete general API for obtaining/comparing/arithmetic on `Instant`.

Do not invent those unrelated APIs while applying this correction.

---

## 9. Direct `MutexGuard[T]` member forwarding

Synchronize Sema and LSP to:

```text
MutexGuard[T] member lookup
    1. canonical guard-specific member, if one exists
    2. accessible member lookup on protected T
```

Sec 0.1 defines no public guard-specific `Unlock`, `Value`, or `Get`.

Forwarding supports accessible fields, properties, and non-consuming methods of `T`.

Mutating protected access requires mutable guard access.

References obtained through forwarding cannot outlive the guard/guard borrow.

The complete protected `T` cannot be moved/consumed through the guard.

Any partial move must obey ordinary Sec partial-move rules and protected `T` must be valid before guard destruction/release.

LSP hover must distinguish:

```text
guard expression:
    MutexGuard[ApplicationState]

forwarded member:
    ApplicationState.Connections
```

---

## 10. Ownership and transferability rulebooks

Synchronize `ownership.md`, `borrowing.md`, and `transferability.md` where needed:

- `Mutex[T]` is `@noCopy`.
- `MutexGuard[T]` is `@noCopy`.
- `Context` is `@noCopy`.
- `ContextSource` is `@noCopy`.
- `@noCopy` does not prohibit borrowing.
- standalone owned Context may be moved.
- `ref Context` is the normal non-owning propagation form.
- source-owned Context is borrow-only from outside ContextSource.
- guard moves remain within the acquiring execution entity.
- live guard cannot cross spawn/task/thread boundaries.
- live guard cannot cross await/suspending join/select.

Use the canonical call-site move marker for consuming ownership transfers.

---

## 11. Deadlock and race analysis

`deadlock_analysis.md` should consume:

- mutex identity;
- acquisition sites;
- guard lifetime;
- same-execution reacquisition;
- suspension boundaries;
- call-graph/lock-order facts.

`data_races.md` should consume:

- mutex acquire/release happens-before;
- protected Place access through guards;
- guard lifetime;
- illegal unsynchronized aliases.

Do not move ownership of those analyses back into `mutex.md`.

---

## 12. Semantic IR

Update `semantic_ir.md` so the semantic model can preserve:

- mutex identity;
- concrete `Mutex[T]`;
- protected `T`;
- acquisition kind;
- timeout `duration`;
- supplied Context identity/borrow;
- acquisition/timeout/context/current-cancellation commit outcome;
- guard identity and execution owner;
- guard moves/borrows;
- forwarded protected access;
- guard destruction/release;
- Context cancellation/deadline registration;
- synchronization edges;
- source provenance;
- CompilationPlan capability requirements.

Do not require the legacy mutex book's literal opcode list unless those names independently remain canonical in `semantic_ir.md`.

The rulebook owns semantics; `semantic_ir.md` owns concrete IR vocabulary.

---

## 13. LSP

Hover must show exact declarations, generic arity, `@noCopy`, members, result types, and comments.

Completion must use `Lock` and `TryLock`, and must not offer legacy lowercase public spellings as canonical API.

Completion on `MutexGuard[T]` must include accessible forwarded members from `T`.

Completion on `Context` must not show `Cancel()`.

Completion on `ContextSource` must show `Context` and `Cancel`.

Hover/navigation for `ContextSource.Context` must show `ref Context` and its source-bounded lifetime.

Hover for `ContextError` must show exactly `Cancelled` and `DeadlineExceeded`.

---

## 14. Formatter and documentation

Update formatter fixtures and generated documentation to canonical CamelCase mutex names.

Generated documentation must expose:

- generic arity;
- `@noCopy`;
- exact overload signatures;
- exact Context/ContextSource ownership;
- ContextError variants;
- direct member forwarding;
- absence of public Unlock/Value/Get;
- `duration` as canonical timeout type.

---

## 15. Tests

Add or synchronize tests for:

- `Mutex[T]` and `MutexGuard[T]` `@noCopy`;
- canonical `Lock`/`TryLock`;
- legacy lowercase spelling diagnostics;
- `Lock()` result type;
- `TryLock()` `Option` result;
- `Lock(duration)` `Option` result;
- time-unit-expression to `duration` conversion;
- `Lock(ref Context)` typed `Result`;
- exact `ContextError`;
- no portable `LockError`;
- no `Lock(timeout, context)`;
- direct guard field/property/method forwarding;
- forwarded-member LSP declaring type;
- guard mutation requiring mutable binding;
- protected references not outliving guard;
- complete protected `T` not consumable;
- no public guard `Unlock/Value/Get`;
- deterministic release on scope/return/error/cancellation cleanup;
- same-execution reentrant acquisition rejection;
- guard spawn/await/join/select rejection;
- Context and ContextSource `@noCopy`;
- standalone Context move and borrow;
- source-owned Context not movable out;
- `ContextSource.Context` lifetime;
- `ContextSource.Cancel()` idempotence;
- Context has no `Cancel()`;
- source destruction is not cancellation;
- parent-to-child cancellation propagation;
- child cancellation not propagating upward;
- effective parent deadline bounding;
- current-execution cancellation distinct from ContextError;
- acquire/release memory synchronization.

---

## 16. Non-decisions

Do not invent while applying this correction:

- reentrant mutexes;
- recursive lock counts;
- public manual guard unlocking;
- mutex poisoning;
- strict FIFO fairness;
- `AsyncMutex`;
- `LockError`;
- `Lock(timeout, context)`;
- hidden reference-counted Context values;
- copyable Context;
- copyable ContextSource;
- `Context.Cancel()`;
- multiple return values for context creation;
- a competing uppercase `Duration` type;
- wall-clock semantics for `Instant`;
- unrelated complete temporal APIs.

Any such change requires a separate explicit language-design decision.
