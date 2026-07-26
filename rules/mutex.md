# Mutex

## Purpose

`Mutex[T]` provides exclusive synchronized access to one owned value of type
`T`.

A mutex combines:

- one synchronization primitive
- one protected value
- one ownership boundary
- one compiler-known concurrency rule

`Mutex[T]` is a first-class compiler-known generic type.

It is not merely a standard-library wrapper.

---

## Type classification

The compiler recognizes:

```sec
Mutex[T]
MutexGuard[T]
```

alongside other compiler-known generic types such as:

```sec
Result[T, E]
Option[T]
Task[T]
```

The language server should provide:

- completion
- hover information
- canonical capitalization
- type-argument validation
- lock and guard diagnostics
- semantic highlighting
- quick fixes for invalid spelling

The canonical type name is:

```sec
Mutex
```

Incorrect capitalization may be normalized by the formatter.

Unknown spellings such as:

```sec
mutext[State]
```

are compiler errors.

Suggested diagnostic:

```text
unknown type "mutext"; did you mean "Mutex"?
```

The LSP should offer an explicit quick fix.

---

## Ownership

`Mutex[T]` owns exactly one value of type `T`.

Example:

```sec
static let State: Mutex[ApplicationState]
```

The protected `ApplicationState` is not separately accessible.

The only valid access path is through a live mutex guard.

The mutex and protected value share the same owner and lifetime.

Destroying the mutex destroys the protected value after synchronization
requirements are satisfied.

---

## Construction

A mutex is constructed with one initial value.

Example:

```sec
let state := Mutex(
    ApplicationState {
        running: false
        connections: 0
    }
)
```

Type inference may infer:

```sec
Mutex[ApplicationState]
```

An explicit type is also valid:

```sec
let state: Mutex[ApplicationState] := Mutex(
    ApplicationState {
        running: false
        connections: 0
    }
)
```

The exact constructor spelling may follow normal generic constructor rules.

The compiler must not permit an uninitialized mutex with accessible storage.

---

## Static mutexes

A static mutex is the preferred form for shared mutable static state.

```sec
static let State: Mutex[ApplicationState] := Mutex(
    ApplicationState {
        running: false
        connections: 0
    }
)
```

Inside an `impl`:

```sec
impl Application {
    static let State: Mutex[ApplicationState] := Mutex(
        ApplicationState {
            running: false
            connections: 0
        }
    )
}
```

A mutable `Mutex[T]` binding is usually unnecessary because mutation occurs
through the guard, not by replacing the mutex itself.

Preferred:

```sec
static let State: Mutex[ApplicationState]
```

Not normally required:

```sec
static let mut State: Mutex[ApplicationState]
```

Replacing or moving a globally shared mutex requires separate rules and should
not be supported implicitly.

---

## Lock acquisition

The basic blocking lock operation is:

```sec
let mut state := State.lock()
```

`lock()` waits until exclusive access can be acquired.

On success it returns:

```sec
MutexGuard[T]
```

Example:

```sec
let mut state := State.lock()

state.running = true
state.connections += 1
```

The lock remains held while the guard is alive.

---

## Guard type

`MutexGuard[T]` represents exactly one successful exclusive lock acquisition.

A guard:

- owns the active lock acquisition
- provides exclusive mutable access to `T`
- releases the lock when destroyed
- is move-only
- is bound to the task that acquired it
- may not cross task boundaries

The guard is not a copy of `T`.

It is a controlled access capability.

---

## Direct member access

For ergonomics, member access through a guard is forwarded to the protected
value.

```sec
let mut state := State.lock()

state.running = true
state.connections += 1
```

The programmer does not need to write:

```sec
state.value.running
```

The semantic type remains:

```sec
MutexGuard[ApplicationState]
```

The compiler must preserve the distinction in diagnostics, hover information and
Semantic IR.

---

## Guard mutability

The binding must be mutable when mutating protected data.

```sec
let mut state := State.lock()
state.running = true
```

A non-mutable guard binding permits read-only access:

```sec
let state := State.lock()
Log(state.connections)
```

The guard still represents exclusive lock ownership even when the local binding
is not mutable.

Binding mutability does not make the mutex shared or reentrant.

---

## Deterministic unlock

The lock is released when the owning guard is destroyed.

Example:

```sec
{
    let mut state := State.lock()
    state.running = true
}
```

The lock is released at the end of the block.

The guard must also release the lock during:

- normal return
- early return
- error propagation
- `break`
- `continue`
- task cancellation
- stack unwinding supported by the selected profile
- deterministic destruction

`defer` is not required for ordinary unlock.

---

## Move-only guard

`MutexGuard[T]` is move-only for every `T`.

Example:

```sec
let first := State.lock()
let second := first
```

This moves the guard to `second`.

After the move:

```sec
first.running // invalid
```

There remains exactly one owner of the active lock acquisition.

A guard must never be copied.

Copying would create:

- duplicate unlock responsibility
- multiple apparent exclusive accesses
- ambiguous destruction
- invalid mutable aliasing

---

## Guard parameters

A function may receive a guard by move:

```sec
fn ConsumeState(state: MutexGuard[ApplicationState]) void {
}
```

This transfers lock ownership into the function.

The caller may not use the previous binding afterward.

A helper function may borrow the guard:

```sec
fn InspectState(state: ref MutexGuard[ApplicationState]) void {
}
```

Mutable access requires:

```sec
fn UpdateState(state: ref mut MutexGuard[ApplicationState]) void {
}
```

Borrowing a guard does not create another lock acquisition.

---

## Task-bound guard

A guard is bound to the task that acquired it.

It may move between functions within the same task.

It may not be:

- moved into another task
- borrowed by another task
- captured by a spawned closure
- returned as part of an escaping task
- detached
- stored in task-shared storage

Invalid:

```sec
let state := State.lock()
let worker := spawn UseState(state)
```

Invalid:

```sec
let state := State.lock()
let worker := spawn UseState(ref state)
```

Expected diagnostic:

```text
MutexGuard[ApplicationState] is bound to the current task and cannot cross spawn
```

---

## Await restriction

A live mutex guard may not cross `await`.

Invalid:

```sec
let mut state := State.lock()
state.running = true

let result := await worker
```

Expected diagnostic:

```text
mutex guard state remains active across await
```

The diagnostic should identify:

- the lock acquisition
- the guard binding
- the await expression

Correct:

```sec
{
    let mut state := State.lock()
    state.running = true
}

let result := await worker
```

This rule applies even if the backend uses an async-aware mutex.

Sec version 0.1 does not introduce a separate `AsyncMutex`.

---

## Join restriction

A live mutex guard should not cross a blocking or suspending `join`.

Invalid:

```sec
let mut state := State.lock()
join worker
```

The same reasoning as `await` applies:

- another task may need the mutex
- the current task may suspend
- deadlock risk increases
- scheduling becomes unpredictable

The compiler should require the guard scope to end before `join`.

---

## Non-reentrant mutex

`Mutex[T]` is explicitly non-reentrant.

The same task may not acquire the same mutex twice while the first guard remains
alive.

Invalid:

```sec
let first := State.lock()
let second := State.lock()
```

Reentrancy is incompatible with the guard model because two active guards would
represent two exclusive mutable accesses to the same `T`.

Sec version 0.1 does not provide a reentrant mutex type.

---

## Repeated lock detection

When the compiler can prove that the same task locks the same mutex twice, it
must report a semantic error.

Suggested diagnostic:

```text
mutex State is already locked by the current task
```

The diagnostic should identify both acquisitions.

When mutex identity is not statically provable, checked profiles may use runtime
self-lock detection.

Runtime self-lock detection may produce:

- panic
- task failure
- a checked diagnostic trap

It must not silently deadlock in profiles that promise checked mutex behavior.

Full dynamic deadlock detection is not required.

---

## Basic lock result

The basic overload is:

```sec
fn lock(ref self) MutexGuard[T]
```

It waits until:

- the mutex is acquired
- the current task is cancelled
- the execution environment reports a lock failure

Ordinary contention is not an error.

The operation should not use `try` merely because it waits.

---

## Non-blocking acquisition

The non-blocking operation is:

```sec
fn tryLock(ref self) Option[MutexGuard[T]]
```

Example:

```sec
match State.tryLock() {
    Some(mut state) => {
        state.running = true
    }

    None => {
        HandleBusyState()
    }
}
```

`None` means the mutex was not immediately available.

It is not a task failure or application error.

---

## Lock overloads

Sec supports function overloading.

Additional waiting policies should therefore use `lock(...)` overloads rather
than many unrelated method names.

Planned forms may include:

```sec
State.lock()
State.lock(timeout)
State.lock(context)
State.lock(timeout, context)
```

The exact context type is defined by the concurrency rules.

A context may provide:

- cancellation
- deadline
- timeout
- task-local wait policy
- diagnostic metadata

Overloads must remain semantically distinct and statically typed.

---

## Timeout overload

A timeout-aware lock may use:

```sec
let outcome := State.lock(timeout)
```

A timeout is not equivalent to task cancellation.

A suitable return type may be:

```sec
Option[MutexGuard[T]]
```

when the only alternate result is timeout or unavailability.

A richer form may use:

## Current implementation status

Implemented:

- `Mutex[T]` and `MutexGuard[T]` are compiler-known generic types.
- `Mutex[T]` is non-copyable in copy classification.
- `MutexGuard[T]` is move-only.
- `Mutex(value)` constructs `Mutex[T]` from one initializer value.
- `mutex.lock()` returns `MutexGuard[T]`.
- `mutex.tryLock()` returns `Option[MutexGuard[T]]`.
- member access through `MutexGuard[T]` is forwarded to fields/properties on
  the protected `T`.

Not implemented yet:

- deterministic unlock/destruction lowering
- live guard crossing `await` or `join` diagnostics
- non-reentrant same-task lock detection
- guard task-bound escape checks for `spawn`
- timeout/context lock overloads
- static mutex initialization lowering
- Semantic IR/MLIR lowering for mutex operations

```sec
Result[MutexGuard[T], LockError]
```

when the operation distinguishes:

- timeout
- cancellation
- backend failure
- invalid context

The final return type must match the concrete lock context model.

Sec version 0.1 should avoid unnecessary lock error complexity.

---

## Context overload

A context-aware lock may use:

```sec
let outcome := State.lock(context)
```

The context may carry cancellation and deadline information.

The overload must not permit hidden borrowing of a short-lived context beyond
the lock call.

The mutex guard does not retain the context unless the type explicitly states
that behavior.

Context-aware waiting should integrate with the current task cancellation model.

---

## Cancellation

`lock()` is a cancellation point.

When cancellation is requested while waiting:

- the lock must not be acquired and leaked
- no guard may be produced unless acquisition completed
- task cleanup must remain deterministic
- the current task may exit through cancellation control flow

Cancellation after successful acquisition does not automatically destroy the
guard immediately.

The task must leave the guard scope through normal cleanup.

A task may observe cancellation while holding a guard, but it must release the
guard before executing control flow that suspends or crosses a task boundary.

---

## Cancellation before lock

If cancellation is already requested before `lock()` begins, the lock operation
should act as a cancellation point before waiting indefinitely.

The implementation may still need a minimal atomic check and scheduler handoff.

It must not silently clear the cancellation request.

---

## Protected value access

The protected `T` may only be accessed through a live guard.

Invalid conceptual access:

```sec
State.value.running
```

The mutex does not expose an unsynchronized raw reference to `T`.

APIs equivalent to the following are forbidden:

```sec
State.getUnsafe()
State.refValue()
State.rawValue()
```

Unsafe code does not automatically permit bypassing mutex ownership.

Any future raw escape must be explicitly unsafe and must not make data races
valid.

---

## Returning protected references

A reference into the protected value may not outlive the guard.

Invalid:

```sec
fn GetName() ref string {
    let state := State.lock()
    return ref state.name
}
```

Expected diagnostic:

```text
cannot return reference to mutex-protected value beyond guard lifetime
```

A copied or moved owned value may be returned when allowed by `T`.

Example:

```sec
fn GetName() string {
    let state := State.lock()
    return state.name
}
```

This is valid only if normal copy or move rules permit it without invalidating
the protected state.

Moving a field out of protected state requires the field and containing type to
remain valid according to ordinary move rules.

---

## Mutex inside structs

A struct may own a mutex.

```sec
type Server struct {
    state: Mutex[ServerState]
}
```

Each `Server` instance owns its own mutex and protected state.

Moving the `Server` moves the mutex only when the mutex type and target profile
permit movement before publication.

A mutex that has been shared, locked or published to another task may become
address-stable and non-movable.

The exact address-stability rule is defined by the concurrency memory model and
backend profile.

---

## Mutex movement

Before a mutex is published or locked, moving its owner may be permitted.

After concurrent access becomes possible, the physical mutex identity must remain
stable unless the backend provides a safe movable representation.

Semantic analysis should conservatively reject moves that may invalidate a live
or published mutex.

Examples include moving:

- a locked mutex
- a struct containing a locked mutex
- a static mutex
- a mutex referenced by another task

---

## Static lifetime

A static mutex has program or module lifetime.

This makes it suitable for detached tasks from a lifetime perspective.

Example:

```sec
static let State: Mutex[ApplicationState]
```

Detached tasks may access it through `lock()`.

This does not remove:

- cancellation requirements
- lock ordering requirements
- shutdown requirements
- deadlock analysis
- memory ordering rules

---

## Destruction

A mutex must not be destroyed while:

- a guard is alive
- another task may lock it
- another task is waiting on it
- the mutex has been published without a proven end of use

The compiler should prove valid destruction where possible.

Static mutexes are destroyed only during valid program shutdown if the target
profile supports such destruction.

Forced termination does not guarantee mutex or protected-value cleanup.

---

## Poisoning

Sec version 0.1 should not use implicit mutex poisoning.

A task failure or cancellation while holding a guard still runs deterministic
cleanup and releases the lock.

Application invariants remain the responsibility of the protected operation and
typed error handling.

A future checked mutex type may track invariant failure explicitly, but ordinary
`Mutex[T]` should not silently change behavior after unrelated task failure.

---

## Fairness

Version 0.1 does not guarantee strict lock fairness.

A backend may use:

- FIFO waiting
- priority-aware waiting
- scheduler-defined waiting
- platform mutex behavior

The selected profile should document fairness and priority-inversion behavior.

Source code must not rely on a specific acquisition order unless a future type
explicitly guarantees it.

---

## Priority inversion

RTOS and embedded profiles may require:

- priority inheritance
- priority ceiling
- scheduler-specific mutex configuration

These are target-profile properties.

They must not change the core `Mutex[T]` and `MutexGuard[T]` ownership model.

A profile that cannot safely implement required mutex behavior must reject the
program or configuration.

---

## Interrupt safety

Ordinary `Mutex[T]` is not interrupt-safe by default.

It must not be used in:

- ISR code
- `@interruptSafe` code
- contexts that forbid blocking

unless the selected profile explicitly defines a compatible implementation.

Atomics or dedicated interrupt-safe primitives should be used instead.

---

## FFI

A foreign mutex is not automatically equivalent to `Mutex[T]`.

Wrapping a foreign synchronization primitive requires an explicit adapter that
defines:

- ownership
- lock acquisition
- unlock behavior
- task binding
- cancellation behavior
- memory ordering
- destruction

The compiler must not infer safe protected access from an opaque foreign handle.

---

## Semantic analysis

The compiler must track:

- mutex identity
- protected type
- guard ownership
- guard moves
- guard borrows
- acquiring task
- guard lifetime
- task-bound restrictions
- await and join crossings
- repeated lock attempts
- protected references
- mutex publication
- mutex destruction
- cancellation points
- lock overload resolution

The analysis should integrate with:

- borrow checking
- move checking
- task analysis
- static lifetime analysis
- whole-program call graph analysis
- memory model validation

---

## Semantic IR

Semantic IR must represent mutex operations explicitly.

At minimum:

```text
MutexCreate
MutexMove
MutexPublish
MutexLock
MutexTryLock
MutexContextLock
MutexUnlock
MutexGuardMove
MutexGuardBorrow
MutexGuardAccess
MutexDestroy
```

IR must record:

- concrete `Mutex[T]` type
- mutex identity
- protected type
- acquiring task
- guard owner
- cancellation behavior
- wait policy
- timeout or context
- memory-order synchronization edges
- source location

The backend must not infer mutex semantics from ordinary function calls.

---

## Diagnostics

Examples:

```text
unknown type "mutext"; did you mean "Mutex"?
```

```text
Mutex requires exactly one type argument
```

```text
mutex State is already locked by the current task
```

```text
MutexGuard[ApplicationState] is bound to the current task and cannot cross spawn
```

```text
mutex guard state remains active across await
```

```text
mutex guard state remains active across join
```

```text
cannot return reference to mutex-protected value beyond guard lifetime
```

```text
cannot move mutex State while it is published to another task
```

```text
cannot destroy mutex State while guard state is active
```

```text
Mutex is not permitted in interrupt-safe context
```

---

## Restrictions

`Mutex[T]` must not:

- expose `T` without a guard
- permit copied guards
- permit reentrant locking
- permit guards across `await`
- permit guards across `join`
- permit guards across `spawn`
- permit guards across `detach`
- permit task-to-task guard transfer
- silently ignore cancellation
- silently poison itself
- imply strict fairness
- bypass ordinary ownership and borrowing
- be treated as a simple library struct by semantic analysis

---

## Future extensions

Possible future additions include:

```sec
RwLock[T]
```

Other possible developments include:

- lock contexts
- deadlines
- timeout-specific outcomes
- priority-aware mutexes
- interrupt-safe locks
- checked lock ordering
- explicit lock ranks
- condition variables
- semaphore types

These are not required for version 0.1.

`AsyncMutex` is not planned because `Mutex[T]` already integrates with task-aware
waiting while guards remain forbidden across suspension points.

---

## Related rules

Detailed behavior is defined in:

```text
tasks.txt
spawn.txt
await.txt
concurrency.txt
atomics.txt
static.txt
concurrency_memory_model.txt
```
