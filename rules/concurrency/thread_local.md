# Thread-Local Storage

## Current implementation status

Implemented:

- compiler-known `ThreadLocal[T]` type;
- compiler-known `ThreadContext` and `ThreadContextError` types;
- generic arity validation for `ThreadLocal[T]`;
- `ThreadLocal[T]` is non-copyable.

Not implemented yet:

- static `ThreadLocal[T]` initializer semantics;
- `.value` property access;
- per-thread slot initialization and destruction state;
- thread-bound reference provenance;
- diagnostics for TLS references across task suspension;
- foreign thread attach/detach semantics;
- target TLS capacity validation;
- Semantic IR thread-local operations.

## Purpose

Thread-local storage provides one logically separate value for each physical
thread.

Thread-local state is distinct from:

- task-local state;
- global static state;
- process-local state;
- CPU-core-local state;
- interrupt-local state.

A task may migrate between physical threads.

Therefore thread-local state must never be treated as task-local state.

---

## Declaration

Thread-local storage uses the compiler-known generic type:

```sec
ThreadLocal[T]
```

A static declaration may use:

```sec
static let LastError := ThreadLocal[int](0)
```

The binding is immutable.

The per-thread value may be mutable through the compiler-known `.value`
property.

```sec
LastError.value = 5
let error := LastError.value
```

The declaration identifies one TLS key and initializer for the program.

---

## Copyable initializer

When the initializer value is copyable:

```sec
static let Counter := ThreadLocal[int](0)
```

each attached physical thread receives its own initialized copy.

The original initializer is not shared mutable state.

---

## Factory initializer

A move-only or nontrivial value requires an initializer callable:

```sec
static let Context := ThreadLocal[WorkerContext](fn() WorkerContext {
    return WorkerContext.Create()
})
```

The factory executes once for each physical thread on first required
initialization or thread attachment according to the target profile.

It must return an owned `T`.

The factory must satisfy the profile's allocation and blocking restrictions.

A no-allocation profile may require compile-time or static initialization.

---

## Per-thread identity

For each attached physical thread:

```sec
ThreadLocalKey.value
```

refers to that thread's own value.

Two physical threads access different storage even when running the same
function.

Two tasks running sequentially on the same physical thread observe the same
thread-local value.

One task that migrates may observe different values before and after suspension.

---

## Access type

The `.value` property provides access to the current physical thread's storage.

Conceptually:

```text
get -> ref T or copied T according to use
set -> replaces the current thread's T
```

The compiler must apply ordinary property, ownership and destruction rules.

Replacing a nontrivial value destroys the previous per-thread value exactly
once.

Moving out of `.value` leaves the current thread's slot uninitialized until it is
explicitly assigned again or a rule-defined reinitializer runs.

The compiler must track this state.

---

## References to thread-local values

A reference to thread-local storage is thread-bound.

Example:

```sec
let current := ref Context.value
```

The reference must not remain live across:

- `await`;
- task-context `join`;
- `Task.Yield()`;
- a waiting `select`;
- another task suspension point;
- transfer to another task;
- transfer to another physical thread;
- return to a scope with wider lifetime.

Reason:

> The task may resume on another physical thread where the reference would name
> different or invalid storage.

Owned copies or moves extracted from TLS follow the ordinary rules of `T`.

---

## ThreadLocal key transfer

The static `ThreadLocal[T]` key may be referenced from any valid physical thread.

The key does not itself contain one shared `T`.

It identifies how to locate the current thread's instance.

Passing the key does not pass another thread's value.

---

## Current thread requirement

Thread-local access requires an attached Sec `ThreadContext`.

Sec-created physical threads and the main thread are attached by the generated
runtime or target startup code.

Foreign threads entering through FFI must be attached before TLS access.

Generated callback wrappers may attach automatically when the target profile
permits it.

Manual attachment, if required, is fallible:

```sec
let context := try Thread.AttachCurrent()
```

The result before `try` is:

```sec
Result[ThreadContext, ThreadContextError]
```

Detaching a foreign thread context performs normal TLS destruction.

---

## Main thread

The main physical thread has its own TLS instance.

Whether initialization is eager at program startup or lazy on first access is a
target implementation detail when observable behavior is unchanged.

Initialization order must follow static initialization rules.

---

## Sec-created thread initialization

Thread-local initialization required by a new thread is part of successful
thread creation or deferred start.

Failure produces:

```sec
ThreadSpawnError.ThreadLocalInitializationFailed
```

or the corresponding start error when initialization is intentionally deferred
until `Start()`.

The user callable must not begin before required TLS initialization succeeds.

---

## Destruction

On normal physical thread exit, initialized thread-local values are destroyed in
reverse successful initialization order for that thread.

Destruction must run for:

- normal return;
- cooperative cancellation;
- panic when the panic rules preserve thread cleanup;
- normal foreign thread detachment.

Destruction is not guaranteed after:

- unsafe hard thread termination;
- process kill;
- power loss;
- hardware reset;
- kernel failure.

The implementation must not destroy another physical thread's TLS values.

---

## Detached threads

A detached thread still owns and destroys its thread-local values at normal
terminal exit.

The detached lifecycle manager must keep required TLS metadata alive until that
cleanup completes.

---

## Task interaction

Task-local and thread-local values are not interchangeable.

A task should use future `TaskLocal[T]` support when state must follow the task
across migration.

Using `ThreadLocal[T]` from task code is valid only when the programmer wants
physical-thread-local behavior.

The compiler should warn when a migratable task reads and writes TLS across
suspension in a pattern likely to assume task-local persistence.

---

## Thread affinity

A task or thread with a proven non-migration guarantee may keep a TLS reference
across task scheduling points only if the relevant scheduling rule explicitly
establishes stable physical-thread affinity.

Sec v0.1 provides no general task affinity guarantee.

Therefore the default analysis rejects TLS references across task suspension.

---

## FFI

Native TLS and Sec `ThreadLocal[T]` may be mapped together by a platform backend.

An extern function's native TLS is not automatically represented by Sec
`ThreadLocal[T]`.

Wrappers must preserve:

- calling thread;
- attachment;
- destruction;
- callback reentrancy;
- panic boundaries.

A raw native TLS key belongs behind the platform view and `unsafe`.

---

## ISR restrictions

Ordinary `ThreadLocal[T]` access is invalid in an ISR.

An ISR may interrupt a thread, but its execution context is not ordinary thread
code and may be nested.

Per-core or interrupt-local storage requires a separate rule and type.

The compiler must not lower ISR TLS access to ordinary thread TLS.

---

## Static initialization

A `ThreadLocal[T]` key is static data.

Its declaration follows `static.md`.

The key may be initialized during program startup without constructing every
thread's `T`.

The initializer or factory must remain available for future thread attachment.

No hidden heap allocation is permitted when the selected profile forbids it.

---

## Storage backends

Valid backends include:

```text
compiler-assigned static TLS offset
ELF or platform TLS
Windows TLS or FLS
RTOS thread-local slot
field in Sec thread control block
fixed table indexed by ThreadID generation
```

The backend must preserve:

- one value per physical thread;
- initialization exactly once per live instance;
- destruction exactly once when guaranteed;
- thread-bound references;
- foreign attachment rules.

Unused thread-local declarations may be removed when unobservable.

---

## Capacity

A target may have a fixed TLS slot or byte limit.

The compiler should calculate statically required TLS where possible.

Exceeding a fixed target limit is a compile-time error.

Dynamically attached foreign threads may fail with `ThreadContextError` when
runtime resources are exhausted.

---

## Core error types

The following runtime error type must be declared in:

```text
core/errors.sec
```

```sec
enum ThreadContextError {
    NotAttached
    AlreadyAttached
    ResourceUnavailable
    NativeFailure
}
```

Thread-local initialization failure during thread creation is represented by:

```sec
ThreadSpawnError.ThreadLocalInitializationFailed
```

and must also be declared in `core/errors.sec`.

Compiler lifetime and suspension diagnostics are not runtime errors.

---

## Semantic analysis

The compiler must track:

- TLS key identity;
- initializer kind;
- initialized/uninitialized slot state;
- current physical thread context;
- task migration possibility;
- references into TLS;
- suspension points;
- move-out and replacement;
- destruction requirements;
- foreign attachment;
- ISR context;
- target TLS capacity;
- allocation and blocking effects of factories.

---

## Semantic IR

Semantic IR must represent:

```text
ThreadLocalDeclare
ThreadLocalInitializeCurrent
ThreadLocalGetCurrent
ThreadLocalBorrowCurrent
ThreadLocalSetCurrent
ThreadLocalMoveOutCurrent
ThreadLocalDestroyCurrent
ThreadAttachCurrent
ThreadDetachCurrent
```

TLS references must retain a thread-bound provenance marker until all migration
and lifetime checks are complete.

---

## Diagnostics

Examples:

```text
reference to thread-local Context cannot remain live across await
```

```text
ThreadLocal[State] cannot be accessed before the current foreign thread is attached
```

```text
thread-local initializer may allocate but target profile forbids allocation
```

```text
thread-local storage requires 2048 bytes but target limit is 1024
```

```text
ordinary ThreadLocal[T] access is not permitted in an interrupt routine
```

Diagnostics must have stable IDs.

---

## Required synchronization

Cross-check and update:

```text
threads.md
tasks.txt
scheduling.md
blocking.md
transferability.md
cancellation.md
concurrency_runtime_model.md
concurrency.md
concurrency_memory_model.txt
static.md
properties.md
types.md
functions.md
ownership.md
borrowing.md
references.md
lifetime_analysis.md
destruction.txt
platform/ffi.md
compiler_analysis.md
semantic_ir.md
core-library.md
core/errors.sec
```
