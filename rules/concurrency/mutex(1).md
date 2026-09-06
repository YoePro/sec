# Mutex

- **Status:** Normative
- **Created:** 2026-09-06
- **Last updated:** 2026-09-06
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/mutex.md`
- **Replaces:** Earlier unversioned revision at the same canonical path
- **Repository baseline reviewed:** `0f5027d`
- **Related rulebooks:** `rules/concurrency/concurrency.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/concurrency/atomics.md`, `rules/analysis/deadlock_analysis.md`, `rules/analysis/data_races.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/transferability.md`, `rules/types/types.md`, `rules/types/units.md`, `rules/declarations/impl.md`, `rules/compiler/semantic_ir.md`, `rules/platform/target_profiles.md`, `rules/platform/platform_model.md`, `rules/tooling/lsp.md`

---

## § 1. Purpose and authority

**Governance tags:** `concurrency.mutex-v2`

§ 1(1) `Mutex[T]` provides exclusive synchronized access to exactly one owned value of type `T`.

§ 1(2) A mutex combines one protected value, one synchronization identity, one exclusive-acquisition discipline, deterministic guard release, and compiler-known concurrency semantics.

§ 1(3) This rulebook owns the Sec 0.1 source-visible mutex and mutex-guard surface.

§ 1(4) This rulebook also specifies the exact `Context`, `ContextSource`, `ContextError`, `duration`, and `Instant` surface materially required by context-aware mutex acquisition.

§ 1(5) The general cancellation model remains owned by `cancellation.md`.

§ 1(6) General temporal and unit semantics remain owned by their respective rulebooks.

§ 1(7) Mutable implementation status does not belong in this normative rulebook.

---

## § 2. Core model

**Governance tags:** `concurrency.mutex-v2`, `analysis.transferability`

§ 2(1) `Mutex[T]` is a compiler-known nominal generic type.

§ 2(2) `MutexGuard[T]` is a compiler-known nominal generic type representing exactly one successful exclusive acquisition of one `Mutex[T]`.

§ 2(3) `Mutex[T]` owns the protected `T`; the protected value is not independently accessible.

§ 2(4) `MutexGuard[T]` is not a copy of `T`.

§ 2(5) `MutexGuard[T]` is an access capability tied to one live acquisition.

§ 2(6) Ordinary contention is normal control flow and is not a Sec error.

§ 2(7) Mutex synchronization is source-language semantics and must not be inferred only from backend lock calls.

---

## § 3. Public naming

**Governance tags:** `concurrency.mutex-v2`, `tooling.mutex-v2`

§ 3(1) Public mutex methods use the canonical Sec public-member CamelCase convention.

§ 3(2) The canonical acquisition names are:

```text
Lock
TryLock
```

§ 3(3) Legacy lowercase spellings such as `lock` and `tryLock` are not canonical public API spellings.

§ 3(4) Compiler diagnostics, LSP completion, hover, formatter fixtures, documentation, and tests must use the canonical spellings.

---

## § 4. Compiler-known and source-visible declarations

**Governance tags:** `concurrency.mutex-v2`, `frontend.mutex-v2`, `tooling.mutex-v2`

§ 4(1) `Mutex[T]`, `MutexGuard[T]`, `Context`, `ContextSource`, `ContextError`, and `Instant` are compiler-known/core concepts used materially by this rulebook.

§ 4(2) Compiler-known status means that the compiler recognizes canonical identity and special semantics.

§ 4(3) Compiler-known status does not permit an implementation to expose only an internal name while omitting the source-visible declaration surface.

§ 4(4) Physical runtime fields for mutexes, guards, cancellation registrations, timers, and scheduler state are implementation-private and must not become ordinary public source fields.

§ 4(5) The compiler-known identity and the source-visible core declaration represent one semantic symbol.

---

## § 5. Exact `Mutex[T]` declaration

**Governance tags:** `concurrency.mutex-v2`, `frontend.mutex-v2`, `tooling.mutex-v2`

§ 5(1) The exact source-visible declaration shape is:

```sec
@noCopy
type Mutex[T]
// `@noCopy` means values of this type cannot be copied.
// Existing owned values may be moved when ownership/address-stability rules permit.
//
// `[T]` declares exactly one generic type parameter.
// `T` is the complete protected value type.
//
// Runtime synchronization storage is opaque/compiler-known and is not
// exposed as ordinary Sec fields.
```

§ 5(2) `Mutex[T]` has exactly one generic type parameter.

§ 5(3) `Mutex[T]` is non-copyable for every `T`, independently of whether `T` is copyable.

§ 5(4) Copying a mutex would duplicate synchronization identity and is always invalid.

§ 5(5) The compiler must not expose a public field containing the protected `T`.

---

## § 6. Exact `MutexGuard[T]` declaration

**Governance tags:** `concurrency.mutex-v2`, `analysis.transferability`, `tooling.mutex-v2`

§ 6(1) The exact source-visible declaration shape is:

```sec
@noCopy
type MutexGuard[T]
// `@noCopy` means a guard cannot be copied.
//
// `[T]` is the protected value type of the acquired Mutex[T].
//
// One live MutexGuard[T] owns exactly one successful exclusive acquisition.
// Destroying the final owner of the guard releases that acquisition.
// Runtime lock token/owner state is compiler/runtime-private.
```

§ 6(2) `MutexGuard[T]` is move-only for every `T`.

§ 6(3) A guard may be moved only while remaining within the same execution entity and while ordinary lifetime rules remain valid.

§ 6(4) A guard must never be copied.

§ 6(5) A guard is destroyed exactly once by its final owner.

§ 6(6) Guard destruction performs the canonical mutex release operation.

---

## § 7. Exact `ContextError`

**Governance tags:** `concurrency.context-v1`, `tooling.context-v1`

§ 7(1) The exact declaration is:

```sec
enum ContextError error {
    Cancelled
    // The supplied Context was explicitly cancelled before
    // the waiting operation committed.

    DeadlineExceeded
    // The effective Context deadline expired before
    // the waiting operation committed.
}
```

§ 7(2) `ContextError` is a nominal Sec error enum.

§ 7(3) `Cancelled` and `DeadlineExceeded` are the complete variants required by this rulebook.

§ 7(4) Neither variant has a payload.

§ 7(5) `ContextError.Cancelled` is distinct from terminal cancellation of the current execution entity.

---

## § 8. Exact `Context` declaration

**Governance tags:** `concurrency.context-v1`, `analysis.transferability`, `tooling.context-v1`

§ 8(1) The exact source-visible declaration shape is:

```sec
@noCopy
type Context
// `@noCopy` makes an owned Context move-only.
//
// A standalone owned Context owns one context identity.
// It may be moved to a new owner or borrowed as `ref Context`.
//
// Context does not expose cancellation authority.
// In particular, Context has no `Cancel()` method.
//
// Parent linkage, cancellation state, deadlines, and runtime registrations
// are compiler/runtime-private.
```

§ 8(2) A standalone owned `Context` is destroyed exactly once by its final owner.

§ 8(3) An owned standalone `Context` may be moved using ordinary Sec ownership semantics.

§ 8(4) A `ref Context` is a non-owning borrow and does not transfer or duplicate ownership.

§ 8(5) A `Context` owned by `ContextSource` cannot be moved out of that source.

---

## § 9. Exact `ContextSource` declaration

**Governance tags:** `concurrency.context-v1`, `analysis.transferability`, `tooling.context-v1`

§ 9(1) The exact source-visible declaration shape is:

```sec
@noCopy
type ContextSource
// `@noCopy` makes the source move-only.
//
// ContextSource owns:
// - exactly one derived child Context,
// - cancellation authority for that Context branch,
// - associated deadline/timer/cancellation registrations.
//
// The owned child Context cannot be moved out of the source.
```

§ 9(2) `ContextSource` represents unique ownership of cancellation authority for its branch.

§ 9(3) Copying a `ContextSource` is invalid.

§ 9(4) Moving a `ContextSource` transfers the source, its child Context, and cancellation authority.

§ 9(5) A source derived from a parent Context has a lifetime dependency on that parent Context.

§ 9(6) A derived source cannot outlive the parent Context from which it was derived.

---

## § 10. Exact `Context` public surface

**Governance tags:** `concurrency.context-v1`, `frontend.context-v1`, `tooling.context-v1`

§ 10(1) The canonical public surface materially required by this rulebook is:

```sec
impl Context {
    static fn Background() Context
    // Creates a standalone root Context with no parent cancellation
    // and no effective deadline.

    fn WithCancel() ContextSource
    // Creates a cancellable child Context owned by the returned ContextSource.

    fn WithTimeout(timeout: duration) ContextSource
    // Creates a child whose requested deadline is derived from the current
    // monotonic time plus `timeout`, bounded by any earlier parent deadline.

    fn WithDeadline(deadline: Instant) ContextSource
    // Creates a child with an absolute monotonic deadline,
    // bounded by any earlier parent deadline.

    IsCancelled bool
    // True after explicit cancellation or effective deadline expiry commits.

    Deadline Option[Instant]
    // The effective absolute monotonic deadline.
    // None means there is no effective deadline.
}
```

§ 10(2) `static fn Background()` is callable through the type and creates an owned standalone `Context`.

§ 10(3) `WithCancel`, `WithTimeout`, and `WithDeadline` return exactly one value: `ContextSource`.

§ 10(4) Sec 0.1 does not use Go-style multiple return values for context creation.

§ 10(5) `Context` intentionally has no public `Cancel()` method.

---

## § 11. Existing `duration` temporal type

**Governance tags:** `concurrency.context-v1`, `frontend.temporal-duration`

§ 11(1) `duration` is the canonical temporal parameter type for relative timeouts in this rulebook.

§ 11(2) The exact type-use syntax is:

```sec
duration
```

§ 11(3) `duration` is an existing compiler-known/core temporal type.

§ 11(4) The existing core extension surface is:

```sec
module core

impl duration {
}
```

§ 11(5) This rulebook does not redefine the physical representation of `duration`.

§ 11(6) Mutex and Context APIs must not introduce a competing uppercase `Duration` type merely for waiting.

---

## § 12. Time-unit timeout sugar

**Governance tags:** `concurrency.context-v1`, `frontend.temporal-duration`, `frontend.units-v2`

§ 12(1) A time-dimension unit expression may be accepted at a `duration` parameter when the canonical temporal/unit conversion rules prove a valid conversion.

Examples:

```sec
context.WithTimeout(5<s>)
mutex.Lock(250<ms>)
```

§ 12(2) The declared parameter types remain `duration`.

§ 12(3) Unit expressions are call-boundary semantic sugar and do not create additional mutex/context overloads.

§ 12(4) The compiler resolves time dimension and scale through canonical unit metadata before materializing the `duration`.

§ 12(5) Mutex code must not contain an independent hard-coded list of time-unit spellings.

§ 12(6) Unit availability/import rules remain owned by the units rulebook.

---

## § 13. Exact `Instant` declaration

**Governance tags:** `concurrency.context-v1`, `frontend.temporal-instant`, `tooling.context-v1`

§ 13(1) The exact declaration shape required by Context deadlines is:

```sec
type Instant
// A copyable point on Sec's monotonic runtime clock.
//
// Instant is not wall-clock/calendar time.
// It does not represent UTC, local time, a date, or a timezone.
// Its physical representation is compiler/runtime-defined.
```

§ 13(2) `Instant` is a source-visible compiler-known/core temporal type.

§ 13(3) `Instant` is copyable and has no ownership/destruction responsibility.

§ 13(4) `WithDeadline` uses `Instant` because a deadline is an absolute monotonic point, not a relative duration.

§ 13(5) General APIs for obtaining/comparing/arithmetic on `Instant` belong to the temporal rulebook.

---

## § 14. Exact `ContextSource` public surface

**Governance tags:** `concurrency.context-v1`, `frontend.context-v1`, `tooling.context-v1`

§ 14(1) The canonical public surface is:

```sec
impl ContextSource {
    Context ref Context
    // Borrowed access to the child Context owned by this ContextSource.
    // The returned reference cannot outlive the source.

    fn Cancel() void
    // Requests cancellation of the still-existing owned child Context.
    // The operation is idempotent.
    // Cancel does not destroy the Context or ContextSource.
}
```

§ 14(2) `Context` is a public borrowed property of type `ref Context`.

§ 14(3) Reading the property does not transfer ownership.

§ 14(4) The property must never materialize an owned copy of the child Context.

§ 14(5) `Cancel()` owns the cancellation-authority role.

§ 14(6) `Cancel()` does not require a mutable source binding merely to request cancellation; it is a compiler-known lifecycle/state operation.

---

## § 15. Cancellation versus destruction

**Governance tags:** `concurrency.context-v1`, `concurrency.memory-model-v2`

§ 15(1) `ContextSource.Cancel()` and destruction are distinct semantic events.

§ 15(2) `Cancel()` changes the state of a still-existing Context to cancelled.

§ 15(3) After `Cancel()`, the source and child Context remain alive until normal destruction/move rules end their lifetimes.

§ 15(4) Repeated `Cancel()` calls are idempotent.

§ 15(5) Destroying a `ContextSource` destroys its owned child Context and associated runtime registrations/resources.

§ 15(6) Destruction is not itself a cancellation event.

§ 15(7) No valid borrow or derived child may outlive the source/context whose destruction would invalidate it.

---

## § 16. Context hierarchy

**Governance tags:** `concurrency.context-v1`, `analysis.transferability`

§ 16(1) A Context may derive child contexts through `WithCancel`, `WithTimeout`, or `WithDeadline`.

§ 16(2) Parent cancellation propagates downward to all still-existing descendants.

§ 16(3) Cancelling a child source does not cancel its parent.

§ 16(4) Cancelling one child branch does not cancel sibling branches merely because they share a parent.

§ 16(5) A child Context cannot outlive the parent Context from which it was derived.

---

## § 17. Context deadline inheritance

**Governance tags:** `concurrency.context-v1`, `frontend.temporal-instant`

§ 17(1) A child Context cannot have an effective deadline later than an earlier effective parent deadline.

§ 17(2) `WithDeadline(requested)` therefore uses the earlier of the parent effective deadline, if present, and the requested deadline.

§ 17(3) `WithTimeout(timeout)` derives a requested deadline from monotonic current time plus `timeout`, then applies the same parent bound.

§ 17(4) `Context.Deadline` reports the effective deadline after parent bounding.

§ 17(5) Parent cancellation may cancel the child before its deadline.

---

## § 18. `Mutex[T]` construction

**Governance tags:** `concurrency.mutex-v2`, `frontend.mutex-v2`

§ 18(1) A mutex is constructed from exactly one initial value of `T`.

§ 18(2) Canonical construction syntax is:

```sec
let state := Mutex(
    ApplicationState {
        Running: false
        Connections: 0
    }
)
```

§ 18(3) Type inference may infer `Mutex[ApplicationState]`.

§ 18(4) Explicit typing is also valid.

§ 18(5) `Mutex(value)` consumes/initializes the protected value according to ordinary constructor/move rules.

§ 18(6) The compiler must not permit an uninitialized mutex whose protected storage can later be accessed.

---

## § 19. Static mutexes

**Governance tags:** `concurrency.mutex-v2`, `concurrency.memory-model-v2`

§ 19(1) Static mutexes are a canonical mechanism for synchronized mutable static state.

```sec
static let State: Mutex[ApplicationState] := Mutex(
    ApplicationState {
        Running: false
        Connections: 0
    }
)
```

§ 19(2) The `Mutex[T]` binding normally remains immutable because protected mutation occurs through a guard.

§ 19(3) `static let mut` is not required merely to mutate protected `T`.

§ 19(4) Static lifetime does not itself replace synchronization.

---

## § 20. Exact mutex acquisition surface

**Governance tags:** `concurrency.mutex-v2`, `frontend.mutex-v2`, `tooling.mutex-v2`

§ 20(1) The exact Sec 0.1 public surface is:

```sec
impl Mutex[T] {
    fn Lock() MutexGuard[T]
    // Waits until acquisition commits.
    // Current-execution cancellation remains active while waiting.

    fn TryLock() Option[MutexGuard[T]]
    // Never waits.
    // Some(guard): acquisition committed immediately.
    // None: the mutex was not immediately available.

    fn Lock(timeout: duration) Option[MutexGuard[T]]
    // Some(guard): acquisition committed before timeout.
    // None: timeout committed before acquisition.
    // Current-execution cancellation remains terminal cancellation.

    fn Lock(context: ref Context) Result[MutexGuard[T], ContextError]
    // Ok(guard): acquisition committed first.
    // Err(ContextError.Cancelled): Context cancellation committed first.
    // Err(ContextError.DeadlineExceeded): Context deadline committed first.
    // Current-execution cancellation remains terminal cancellation.
}
```

§ 20(2) This signature surface is normative.

§ 20(3) Core/compiler implementation may use privileged intrinsic bodies, but the public signatures must match exactly.

§ 20(4) Ordinary instance methods have implicit `self` according to `impl.md`.

§ 20(5) There is no `Lock(timeout, context)` overload in Sec 0.1.

§ 20(6) There is no portable `LockError` in Sec 0.1.

---

## § 21. Basic `Lock()`

**Governance tags:** `concurrency.mutex-v2`, `concurrency.cancellation-v1`

§ 21(1) `Lock()` waits until exclusive acquisition commits unless current-execution cancellation wins first.

§ 21(2) Ordinary contention is not an error.

§ 21(3) Successful acquisition returns exactly one `MutexGuard[T]`.

§ 21(4) `Lock()` does not return `Result` or `Option`.

§ 21(5) When the current execution entity supports cooperative cancellation, waiting in `Lock()` is a cancellation point.

§ 21(6) If cancellation commits before lock acquisition, the operation does not return and terminal cancellation cleanup begins.

§ 21(7) If lock acquisition commits first, the execution owns the guard and ordinary cleanup must release it before terminal cancellation completes.

---

## § 22. `TryLock()`

**Governance tags:** `concurrency.mutex-v2`

§ 22(1) `TryLock()` performs one non-blocking acquisition attempt.

§ 22(2) Its exact return type is `Option[MutexGuard[T]]`.

§ 22(3) `Some(guard)` means acquisition committed immediately.

§ 22(4) `None` means the mutex was not immediately available.

§ 22(5) `None` is neither an error nor cancellation.

§ 22(6) `TryLock()` is not a cancellation point merely by being a mutex operation because it performs no wait.

---

## § 23. Timeout `Lock(duration)`

**Governance tags:** `concurrency.mutex-v2`, `concurrency.cancellation-v1`, `frontend.temporal-duration`

§ 23(1) The timeout overload is:

```sec
fn Lock(timeout: duration) Option[MutexGuard[T]]
```

§ 23(2) `Some(guard)` means acquisition committed before timeout.

§ 23(3) `None` means timeout committed before acquisition.

§ 23(4) Timeout is normal alternate control flow and is not `ContextError`.

§ 23(5) Current-execution cancellation remains independently active while waiting.

§ 23(6) If current-execution cancellation wins first, the operation does not return `None`; terminal cancellation occurs.

§ 23(7) The implementation must preserve one winning commit among acquisition, timeout, and current-execution cancellation.

```sec
match State.Lock(5<s>) {
    Some(mut state) => {
        state.Running = true
    }

    None => {
        HandleTimeout()
    }
}
```

§ 23(8) `5<s>` is converted to the canonical `duration` parameter by temporal/unit rules.

---

## § 24. Context-aware `Lock(ref Context)`

**Governance tags:** `concurrency.mutex-v2`, `concurrency.context-v1`, `concurrency.cancellation-v1`

§ 24(1) The context-aware overload is:

```sec
fn Lock(context: ref Context) Result[MutexGuard[T], ContextError]
```

§ 24(2) `Ok(guard)` means lock acquisition committed before Context cancellation/deadline.

§ 24(3) `Err(ContextError.Cancelled)` means explicit cancellation of the supplied Context branch committed first.

§ 24(4) `Err(ContextError.DeadlineExceeded)` means the Context's effective deadline expired first.

§ 24(5) Context cancellation is an operation-local typed result and does not by itself terminally cancel the current execution entity.

§ 24(6) Current-execution cancellation remains separate terminal control flow.

§ 24(7) If current-execution cancellation commits first, `Lock(context)` does not return `ContextError`.

§ 24(8) The implementation must preserve exactly one winning commit among acquisition, Context cancellation/deadline, and current-execution cancellation.

§ 24(9) The returned guard does not retain the supplied Context.

---

## § 25. Already-cancelled/expired Context

**Governance tags:** `concurrency.mutex-v2`, `concurrency.context-v1`

§ 25(1) A context-aware lock must observe Context state before committing a wait outcome.

§ 25(2) An already explicitly-cancelled Context produces `Err(ContextError.Cancelled)` unless acquisition had already committed.

§ 25(3) An already-expired effective deadline produces `Err(ContextError.DeadlineExceeded)` unless another earlier committed cause controls the Context.

§ 25(4) The runtime must not acquire and leak a guard after returning a Context error.

---

## § 26. Acquisition commit semantics

**Governance tags:** `concurrency.mutex-v2`, `concurrency.cancellation-v1`, `concurrency.memory-model-v2`

§ 26(1) Waiting mutex operations have an atomic semantic commit boundary.

§ 26(2) Before acquisition commit, no guard exists and the caller owns no acquisition.

§ 26(3) After acquisition commit, exactly one guard exists and the caller owns the acquisition.

§ 26(4) Cancellation or timeout must not partially commit acquisition.

§ 26(5) Wait registrations must be removed consistently with the winning outcome.

§ 26(6) Backend races between wakeup, timeout, and cancellation must resolve to exactly one source-level outcome.

---

## § 27. No portable lock runtime error

**Governance tags:** `concurrency.mutex-v2`, `compiler.platform-model`

§ 27(1) Sec 0.1 does not expose `Result[MutexGuard[T], LockError]` for ordinary mutex acquisition.

§ 27(2) Ordinary contention is not an error.

§ 27(3) Timeout is represented by `Option` on the timeout overload.

§ 27(4) explicit Context cancellation/deadline is represented by `ContextError`.

§ 27(5) Current-execution cancellation is terminal control flow.

§ 27(6) A target/profile that cannot provide a conforming mutex implementation must reject the program/configuration where possible.

§ 27(7) Native platform error codes must not automatically leak into the portable API.

---

## § 28. Guard ownership and movement

**Governance tags:** `concurrency.mutex-v2`, `analysis.transferability`

§ 28(1) One successful acquisition produces exactly one owning guard.

§ 28(2) The guard owns release responsibility.

§ 28(3) The guard may move between bindings/functions within the same execution entity.

§ 28(4) A consuming call uses the canonical call-site move marker.

```sec
fn ConsumeGuard(guard: MutexGuard[ApplicationState]) void {
    UseGuard(guard)
}

let guard := State.Lock()
ConsumeGuard(<-guard)
```

§ 28(5) After the move, the previous binding is unavailable.

---

## § 29. Guard borrowing

**Governance tags:** `concurrency.mutex-v2`, `analysis.transferability`

§ 29(1) Read-only helper code may borrow a guard:

```sec
fn InspectState(state: ref MutexGuard[ApplicationState]) void {
    Log(state.Connections)
}
```

§ 29(2) Mutable helper code may borrow a mutable guard:

```sec
fn UpdateState(state: ref mut MutexGuard[ApplicationState]) void {
    state.Connections += 1
}
```

§ 29(3) Borrowing does not create a second acquisition or second release owner.

§ 29(4) References derived through guard forwarding are bounded by the guard and relevant guard borrow.

---

## § 30. Direct protected-member forwarding

**Governance tags:** `concurrency.mutex-v2`, `frontend.mutex-guard-forwarding`, `tooling.mutex-v2`

§ 30(1) Member access through `MutexGuard[T]` is forwarded to the protected `T`.

```sec
let mut state := State.Lock()

state.Running = true
state.Connections += 1
```

§ 30(2) The programmer does not need `.Value`, `.Get()`, or equivalent wrapper access.

§ 30(3) The static type of `state` remains `MutexGuard[ApplicationState]`.

§ 30(4) Forwarding does not make `MutexGuard[T]` structurally identical to `T`.

§ 30(5) Forwarding is compiler-known member lookup semantics, not an implicit conversion.

---

## § 31. Forwarding lookup order

**Governance tags:** `frontend.mutex-guard-forwarding`, `tooling.mutex-v2`

§ 31(1) Member lookup on `MutexGuard[T]` proceeds in this order:

1. canonical guard-specific members, if any;
2. ordinary member lookup on protected `T`.

§ 31(2) Sec 0.1 defines no public `Unlock`, `Value`, or `Get` guard member.

§ 31(3) Deterministic destruction owns ordinary unlock semantics.

§ 31(4) Tooling must preserve the declaring type of a forwarded member.

---

## § 32. Forwarded fields and properties

**Governance tags:** `frontend.mutex-guard-forwarding`, `tooling.mutex-v2`

§ 32(1) Accessible fields of `T` may be reached through a live guard.

§ 32(2) Accessible properties of `T` may be reached through a live guard.

§ 32(3) Original visibility rules still apply.

§ 32(4) A guard does not make a private protected member public.

§ 32(5) Getter/setter typing and any `try` requirement remain those of `T`.

---

## § 33. Forwarded methods

**Governance tags:** `frontend.mutex-guard-forwarding`

§ 33(1) Accessible methods of `T` may be resolved through a live guard when receiver requirements can be satisfied without moving the complete protected `T` out of the mutex.

§ 33(2) A read-only receiver is satisfied through read access to protected `T`.

§ 33(3) A mutable receiver requires mutable guard access.

§ 33(4) A method that consumes the complete protected `T` cannot be invoked through forwarding in Sec 0.1.

§ 33(5) Generic inference, overload resolution, visibility, error typing, and ownership semantics remain those of the original method.

---

## § 34. Guard binding mutability

**Governance tags:** `concurrency.mutex-v2`, `frontend.mutex-guard-forwarding`

§ 34(1) A non-mutable guard binding permits read-only protected access.

```sec
let state := State.Lock()
Log(state.Connections)
```

§ 34(2) Mutation requires a mutable guard binding.

```sec
let mut state := State.Lock()
state.Connections += 1
```

§ 34(3) The guard represents exclusive lock ownership even when its local binding is immutable.

§ 34(4) Binding mutability does not make the mutex reentrant.

---

## § 35. Protected references

**Governance tags:** `concurrency.mutex-v2`, `analysis.transferability`

§ 35(1) A reference obtained through forwarded access cannot outlive the guard.

§ 35(2) A reference obtained from a borrowed guard cannot outlive the relevant guard borrow.

§ 35(3) The compiler must reject a protected reference escaping beyond the guard lifetime.

```sec
fn GetName() ref string {
    let state := State.Lock()
    return ref state.Name
}
```

§ 35(4) A diagnostic should identify both the guard acquisition and escaping reference.

---

## § 36. Moving protected values

**Governance tags:** `concurrency.mutex-v2`, `analysis.transferability`

§ 36(1) The complete protected `T` cannot be moved out through `MutexGuard[T]`.

§ 36(2) A consuming protected method that would move complete `T` is invalid through forwarding.

§ 36(3) Copying a copyable protected subvalue out is permitted under ordinary Sec rules.

§ 36(4) Moving an owned subvalue is permitted only when ordinary partial-move rules allow it and protected `T` is restored to a valid releasable state before guard destruction.

§ 36(5) Mutex protection does not weaken ordinary ownership invariants.

---

## § 37. Deterministic unlock

**Governance tags:** `concurrency.mutex-v2`, `concurrency.memory-model-v2`

§ 37(1) The mutex is released when the owning guard is destroyed.

§ 37(2) Guard destruction must release the mutex on ordinary deterministic cleanup paths including normal return, early return, `break`, `continue`, error propagation, and supported unwind/cancellation cleanup.

§ 37(3) `defer` is not required for ordinary mutex release.

§ 37(4) There is no public `guard.Unlock()` in Sec 0.1.

§ 37(5) Mutex release provides release synchronization defined by `concurrency_memory_model.md`.

---

## § 38. Cancellation while holding a guard

**Governance tags:** `concurrency.mutex-v2`, `concurrency.cancellation-v1`

§ 38(1) A cancellation request does not asynchronously destroy a live guard.

§ 38(2) Safe Sec does not tear down protected code at an arbitrary instruction.

§ 38(3) If cancellation becomes terminal through an allowed path, deterministic cleanup destroys the guard before terminal cancellation completes.

§ 38(4) Cancellation while holding a guard does not automatically poison the mutex.

---

## § 39. Execution-entity binding

**Governance tags:** `concurrency.mutex-v2`, `analysis.transferability`

§ 39(1) A `MutexGuard[T]` is bound to the execution entity that acquired it.

§ 39(2) This identity must not be represented solely as an operating-system thread ID because tasks may migrate.

§ 39(3) A guard may move between functions within the same execution entity.

§ 39(4) A guard may not be moved or borrowed into another task/thread.

§ 39(5) A guard may not be captured by a spawned closure, detached, or stored in shared concurrent storage.

---

## § 40. Spawn restriction

**Governance tags:** `concurrency.mutex-v2`, `analysis.transferability`

§ 40(1) A live guard cannot cross a spawn boundary by move, borrow, or capture.

```sec
let guard := State.Lock()
let worker := try spawn UseState(<-guard)
```

§ 40(2) The compiler must diagnose the acquisition and transfer boundary.

Suggested diagnostic:

```text
MutexGuard[ApplicationState] is bound to the current execution entity and cannot cross spawn
```

---

## § 41. Suspension restriction

**Governance tags:** `concurrency.mutex-v2`, `sema.deadlock-analysis`

§ 41(1) A live `MutexGuard[T]` may not cross `await`.

§ 41(2) A live guard may not cross a suspending/blocking `join`.

§ 41(3) A live guard may not cross `select`.

§ 41(4) A live guard may not cross another compiler-classified suspension boundary unless a future explicit rule defines a safe pinned-guard mechanism.

§ 41(5) Sec 0.1 defines no pinned-guard mechanism and no separate `AsyncMutex` merely to bypass this rule.

---

## § 42. Non-reentrant mutex

**Governance tags:** `concurrency.mutex-v2`, `sema.deadlock-analysis`

§ 42(1) `Mutex[T]` is non-reentrant.

§ 42(2) The same execution entity may not acquire the same mutex while it owns a live guard for that mutex.

§ 42(3) Reentrancy would create two exclusive guard capabilities for the same protected `T`.

§ 42(4) Sec 0.1 provides no reentrant mutex type.

---

## § 43. Repeated-lock detection

**Governance tags:** `concurrency.mutex-v2`, `sema.deadlock-analysis`, `tooling.mutex-v2`

§ 43(1) When analysis can prove same-execution reacquisition while a guard is live, compilation must fail.

Suggested diagnostic:

```text
mutex State is already locked by the current execution entity
```

§ 43(2) The diagnostic should identify both acquisitions.

§ 43(3) Checked profiles may provide runtime self-lock detection when identity is not statically provable.

§ 43(4) Full dynamic deadlock detection is not required by the mutex type itself.

---

## § 44. Memory synchronization

**Governance tags:** `concurrency.mutex-v2`, `concurrency.memory-model-v2`

§ 44(1) Successful mutex acquisition provides acquire synchronization.

§ 44(2) Guard destruction/release provides release synchronization.

§ 44(3) Release of a mutex synchronizes with the matching later successful acquisition of the same mutex identity according to the concurrency memory model.

§ 44(4) Writes to protected state before release become visible after the synchronizing acquisition.

§ 44(5) Different mutex identities do not synchronize merely because their protected types are equal.

---

## § 45. Protected access path

**Governance tags:** `concurrency.mutex-v2`, `sema.data-race-analysis`

§ 45(1) Protected `T` may only be accessed through a live guard or another explicitly defined unsafe/platform mechanism with an equivalent proven contract.

§ 45(2) `Mutex[T]` must not expose ordinary `Get`, `Value`, `RefValue`, or `RawValue` members that bypass acquisition.

§ 45(3) The compiler must not synthesize an ordinary `ref T` directly from `Mutex[T]`.

§ 45(4) `unsafe` does not make a data race semantically valid.

---

## § 46. Mutex movement and address stability

**Governance tags:** `concurrency.mutex-v2`, `analysis.transferability`, `concurrency.memory-model-v2`

§ 46(1) `Mutex[T]` is `@noCopy` but may be moved while ordinary ownership and synchronization identity rules permit it.

§ 46(2) Before publication or locking, moving an owned mutex may be valid.

§ 46(3) Once another execution context depends on mutex identity, movement must preserve that identity.

§ 46(4) A live guard, waiter, or identity-dependent reference forbids a source-level move that would invalidate it.

§ 46(5) A backend may use indirection to preserve identity while source ownership moves.

---

## § 47. Mutexes inside structs

**Governance tags:** `concurrency.mutex-v2`, `analysis.transferability`

§ 47(1) A struct may own a `Mutex[T]`.

```sec
type Server struct {
    State: Mutex[ServerState]
}
```

§ 47(2) Each struct instance owns its own mutex identity.

§ 47(3) Moving the enclosing struct moves the mutex only when mutex movement rules permit it.

§ 47(4) Copying the enclosing struct is invalid when it would copy contained `@noCopy Mutex[T]`.

---

## § 48. Mutex destruction

**Governance tags:** `concurrency.mutex-v2`, `analysis.transferability`

§ 48(1) Destroying `Mutex[T]` destroys its protected `T` exactly once.

§ 48(2) A mutex must not be destroyed while a live guard exists.

§ 48(3) A mutex must not be destroyed while a valid waiter, reference, or published execution context can still use it.

§ 48(4) The compiler must prove destruction safety where static ownership/lifetime information is sufficient.

§ 48(5) Forced unsafe termination may violate cleanup and is outside ordinary safe destruction guarantees.

---

## § 49. Poisoning

**Governance tags:** `concurrency.mutex-v2`

§ 49(1) Sec 0.1 does not use implicit mutex poisoning.

§ 49(2) Panic, task failure, or cancellation does not automatically mark the mutex permanently unusable.

§ 49(3) Deterministic guard cleanup releases the mutex when the applicable cleanup model runs.

§ 49(4) Application invariant recovery remains the responsibility of protected code and typed error/state handling.

---

## § 50. Fairness and priority

**Governance tags:** `concurrency.mutex-v2`, `compiler.platform-model`

§ 50(1) Sec 0.1 does not guarantee strict FIFO mutex fairness.

§ 50(2) A target/runtime may use FIFO, priority-aware, scheduler-defined, or platform-native waiting.

§ 50(3) Priority inheritance, priority ceiling, and scheduler-specific priority behavior are target/profile properties.

§ 50(4) Target priority/fairness policy must not change source ownership or reentrancy semantics.

---

## § 51. Interrupt and non-blocking contexts

**Governance tags:** `concurrency.mutex-v2`, `compiler.platform-model`

§ 51(1) Ordinary `Mutex[T]` is not assumed ISR-safe.

§ 51(2) Blocking `Lock` overloads are invalid in contexts whose contract forbids blocking unless the selected profile proves a compatible implementation with identical source semantics.

§ 51(3) `TryLock()` may be usable in additional contexts only when the selected `CompilationPlan` explicitly permits it.

§ 51(4) Local interrupt masking is not automatically equivalent to a portable mutex across cores/execution domains.

---

## § 52. FFI

**Governance tags:** `concurrency.mutex-v2`, `compiler.platform-model`

§ 52(1) A foreign mutex is not automatically equivalent to Sec `Mutex[T]`.

§ 52(2) A foreign adapter must define ownership, acquisition, release, execution binding, cancellation behavior, memory ordering, destruction, and context validity.

§ 52(3) Native error codes do not automatically become a portable `LockError`.

---

## § 53. Context use in service/cloud code

**Governance tags:** `concurrency.context-v1`

§ 53(1) `Context` is designed to propagate through operation boundaries such as HTTP requests, service calls, database operations, and I/O waits.

§ 53(2) A function that only observes/propagates Context should normally accept:

```sec
ctx: ref Context
```

§ 53(3) Accepting `ref Context` does not grant cancellation authority.

§ 53(4) A callee may derive its own child branch and may cancel that branch through its own `ContextSource`.

§ 53(5) Borrowing the parent Context never grants authority to cancel the parent.

---

## § 54. Owning standalone Context

**Governance tags:** `concurrency.context-v1`, `analysis.transferability`

§ 54(1) `Context.Background()` returns an owned standalone `Context`.

§ 54(2) An owned standalone Context may be moved into another owning object.

```sec
type HttpConnection struct {
    Context: Context
}

let root := Context.Background()

let connection := HttpConnection {
    Context: <-root
}
```

§ 54(3) After the move, the previous binding is unavailable.

§ 54(4) The final owner destroys the standalone Context exactly once.

§ 54(5) Borrowing it for operations does not change its owner.

---

## § 55. Owning derived Context branches

**Governance tags:** `concurrency.context-v1`, `analysis.transferability`

§ 55(1) `WithCancel`, `WithTimeout`, and `WithDeadline` return an owning `ContextSource`.

§ 55(2) The source owns the derived child Context.

§ 55(3) `source.Context` has static type `ref Context`.

§ 55(4) The child cannot be moved out of the source.

§ 55(5) An object that must own a cancellable derived branch should own/move the whole `ContextSource`.

§ 55(6) Destroying the source ends the branch lifetime rather than emitting an implicit cancellation event.

---

## § 56. Context memory ordering

**Governance tags:** `concurrency.context-v1`, `concurrency.memory-model-v2`

§ 56(1) Cancellation requests and observations must be safely synchronized by compiler/runtime implementation.

§ 56(2) Context cancellation state must be observable by registered wait operations without a data race.

§ 56(3) Deadline expiry participates in the same commit discipline as explicit cancellation.

§ 56(4) Context cancellation publication does not automatically publish arbitrary application data.

---

## § 57. Sema requirements

**Governance tags:** `frontend.mutex-v2`, `frontend.context-v1`, `frontend.mutex-guard-forwarding`, `sema.deadlock-analysis`, `sema.data-race-analysis`

§ 57(1) Sema must track mutex identity, protected `T`, guard ownership, guard moves/borrows, guard lifetime, and acquiring execution entity.

§ 57(2) Sema must enforce `@noCopy` for Mutex, MutexGuard, Context, and ContextSource.

§ 57(3) Sema must enforce guard execution-boundary and suspension restrictions.

§ 57(4) Sema must enforce statically provable non-reentrancy.

§ 57(5) Sema must track protected references derived through forwarding.

§ 57(6) Sema must reject movement of complete protected `T`.

§ 57(7) Sema must resolve exact `Lock` overloads from argument type.

§ 57(8) Sema must distinguish canonical `duration` timeout from `ref Context`.

§ 57(9) Sema must reject moving `ContextSource.Context` out and enforce its borrow lifetime.

§ 57(10) Sema must enforce derived-source lifetime dependency on parent Context.

§ 57(11) Time-unit expressions must convert through canonical temporal/unit rules, not a mutex-specific table.

---

## § 58. Direct-forwarding Sema

**Governance tags:** `frontend.mutex-guard-forwarding`, `tooling.mutex-v2`

§ 58(1) When direct guard-member lookup does not resolve a canonical guard-specific member, Sema performs member lookup on protected `T`.

§ 58(2) The resulting semantic member retains `T` as its declaring type.

§ 58(3) The access path retains guard/mutex identity.

§ 58(4) Mutable protected access requires mutable guard access.

§ 58(5) Reference results are lifetime-bounded by the guard.

§ 58(6) Consuming complete-`T` operations are rejected.

---

## § 59. Semantic IR

**Governance tags:** `analysis.semantic-ir-v2`, `semantic-ir.mutex-v2`

§ 59(1) Semantic IR must preserve mutex/context semantics explicitly enough that lowering does not rediscover them from ordinary calls or names.

§ 59(2) It must preserve:

- mutex identity;
- concrete `Mutex[T]` and protected `T`;
- acquisition kind: blocking, try, timeout, context-aware;
- timeout `duration` where applicable;
- Context identity/borrow where applicable;
- acquisition commit outcome;
- guard identity and execution owner;
- guard moves/borrows;
- forwarded protected access;
- release/destruction;
- cancellation/deadline registration;
- synchronization edges;
- source provenance;
- target capability requirements.

§ 59(3) Concrete Semantic IR opcode names are owned by `semantic_ir.md`.

§ 59(4) This rulebook does not require the legacy hard-coded `MutexCreate`/`MutexContextLock` vocabulary.

§ 59(5) Context-aware lock must preserve the distinction between `ContextError` and terminal current-execution cancellation.

---

## § 60. Lowering

**Governance tags:** `lowering.mutex-v2`, `compiler.platform-model`

§ 60(1) Lowering consumes validated mutex/context Semantic IR plus the selected `CompilationPlan`.

§ 60(2) Lowering must preserve exclusive acquisition, exactly-once release, and acquire/release synchronization.

§ 60(3) Lowering must preserve acquisition/timeout/context/current-cancellation commit races.

§ 60(4) Lowering must preserve ContextSource registration cleanup and lifetime semantics.

§ 60(5) Lowering must not implement a Context error by terminally cancelling the current task.

§ 60(6) Lowering must not implement current-task cancellation as `Err(ContextError.Cancelled)`.

§ 60(7) Backend lock/runtime names are not the source-language definition.

---

## § 61. LSP hover

**Governance tags:** `tooling.mutex-v2`, `tooling.context-v1`

§ 61(1) Hover for `Mutex[T]` must expose `@noCopy`, generic arity 1, protected `T`, exact acquisition overloads, and non-reentrant semantics.

§ 61(2) Hover for `MutexGuard[T]` must expose `@noCopy`, generic arity 1, execution-entity binding, deterministic release, and direct forwarding.

§ 61(3) Hover for `Context` must expose `@noCopy`, exact methods/properties, and absence of `Cancel()`.

§ 61(4) Hover for `ContextSource` must expose `@noCopy`, borrowed `Context ref Context`, `Cancel()`, and destruction semantics.

§ 61(5) Hover for `ContextError` must expose both variants and meanings.

§ 61(6) Hover on a forwarded member must identify the original protected declaring type while hover on the guard expression remains `MutexGuard[T]`.

---

## § 62. Completion and navigation

**Governance tags:** `tooling.mutex-v2`, `tooling.context-v1`

§ 62(1) Completion on `Mutex[T]` must use `Lock` and `TryLock`.

§ 62(2) Completion on `MutexGuard[T]` must include accessible forwarded members of `T`.

§ 62(3) Completion must not invent `Unlock`, `Value`, or `Get`.

§ 62(4) Completion on `ContextSource` must expose `Context` and `Cancel`.

§ 62(5) Completion on `Context` must not expose `Cancel`.

§ 62(6) Navigation must treat compiler-known identity and source-visible declaration as one symbol.

---

## § 63. Diagnostics

**Governance tags:** `tooling.mutex-v2`, `frontend.mutex-v2`, `frontend.context-v1`

§ 63(1) Diagnostics should distinguish language invalidity, ownership invalidity, cancellation/control-flow behavior, and target capability failure.

Examples:

```text
Mutex requires exactly one type argument
```

```text
mutex State is already locked by the current execution entity
```

```text
MutexGuard[ApplicationState] is bound to the current execution entity and cannot cross spawn
```

```text
mutex guard state remains active across await
```

```text
cannot return reference to mutex-protected value beyond guard lifetime
```

```text
cannot move the complete protected ApplicationState out of MutexGuard[ApplicationState]
```

```text
Context is @noCopy; borrow it as ref Context or move ownership explicitly
```

```text
ContextSource.Context is borrowed from its ContextSource and cannot be moved out
```

§ 63(2) Target capability diagnostics should name the relevant selected `CompilationPlan` fact when practical.

---

## § 64. Restrictions

**Governance tags:** `concurrency.mutex-v2`, `concurrency.context-v1`

§ 64(1) `Mutex[T]`, `MutexGuard[T]`, `Context`, and `ContextSource` must not be copied.

§ 64(2) Complete protected `T` must not be moved out through a guard.

§ 64(3) A live guard must not cross spawn, `await`, suspending `join`, or `select`.

§ 64(4) `Mutex[T]` is not reentrant.

§ 64(5) Sec 0.1 has no public guard `Unlock()`.

§ 64(6) Sec 0.1 has no mutex `LockError`.

§ 64(7) Sec 0.1 has no `Lock(timeout, context)` overload.

§ 64(8) `Context` has no `Cancel()`.

§ 64(9) Destroying `ContextSource` must not be treated as an implicit cancellation event.

§ 64(10) Backend failures must not silently alter portable result types.

---

## § 65. Implementation freedom

**Governance tags:** `concurrency.mutex-v2`, `compiler.platform-model`

§ 65(1) A target/runtime may implement `Mutex[T]` using an OS mutex, futex-like primitive, scheduler mutex, RTOS primitive, runtime lock, or another mechanism preserving the source contract.

§ 65(2) Context/ContextSource may use inline state, runtime registrations, intrusive links, indirection, or another representation preserving ownership/lifetime semantics.

§ 65(3) No managed runtime or garbage collector is required by this rulebook.

§ 65(4) Runtime representation must not weaken `@noCopy` source ownership or child/parent lifetime rules.

§ 65(5) Runtime implementation-private fields must not become public source members.

---

## § 66. Governance

**Governance tags:** `concurrency.mutex-v2`, `concurrency.context-v1`, `frontend.mutex-v2`, `frontend.mutex-guard-forwarding`, `frontend.context-v1`, `frontend.temporal-duration`, `frontend.temporal-instant`, `frontend.units-v2`, `tooling.mutex-v2`, `tooling.context-v1`, `semantic-ir.mutex-v2`, `lowering.mutex-v2`, `compiler.platform-model`, `analysis.transferability`, `analysis.semantic-ir-v2`, `sema.deadlock-analysis`, `sema.data-race-analysis`, `concurrency.memory-model-v2`

§ 66(1) Mutable implementation information for this rulebook must be maintained in `implementation-status.yaml`.

§ 66(2) Primary mutex governance is `concurrency.mutex-v2`.

§ 66(3) The Context surface materially required by this book is governed by `concurrency.context-v1` until a dedicated Context rulebook becomes canonical.

§ 66(4) A future dedicated Context rulebook may take ownership of these exact declarations but must not silently change them.

§ 66(5) Semantic IR, lowering, LSP, parser/Sema, temporal conversion, and target capability must preserve the exact surface and semantics defined here.

§ 66(6) Cross-rulebook synchronization required by this revision is tracked in the accompanying correction document.
