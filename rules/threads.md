# Threads

## Purpose

A thread is an explicit physical target execution context.

It is distinct from a Sec task.

```sec
let worker := spawn thread Work()
```

`spawn thread` requests a physical target thread or target-equivalent native
thread.

The backend must not silently lower it to an ordinary task.

## Target support

Valid thread lowerings may include:

- POSIX threads
- Windows threads
- RTOS threads
- another native physical thread abstraction declared by the target profile

A target without physical thread support must reject `spawn thread`.

Expected diagnostic:

```text
target profile does not support physical threads
```

## Thread type

`Thread[T]` is a compiler-known move-only lifecycle and result owner.

It is not copyable.

Moving a `Thread[T]` handle transfers responsibility for its lifecycle.

Thread identity is not task identity.

`Thread[T]` is not interchangeable with `Task[T]`.

## Join

```sec
join threadHandle
```

waits for thread termination.

A successful join:

- synchronizes memory with the completed thread
- collects the terminal outcome
- releases join-owned native resources
- marks the thread handle as joined

The final syntax for extracting a typed thread result is not decided here.

## Detach

```sec
detach threadHandle
```

relinquishes join ownership.

After detach, the handle cannot be joined.

Native resources are reclaimed when the detached thread terminates according to
the target profile.

A non-void result must be discarded explicitly.

Conceptually:

```sec
detach threadHandle discard
```

## Creation failure

Thread creation may fail.

The final API must expose failure explicitly.

This rulebook does not choose the final error type or result spelling.

## Stack

Thread stack storage is target-managed or explicitly configured by a future
rule.

The final design must account for:

- stack allocation
- stack size
- alignment
- minimum target stack size
- allocation failure
- destruction
- detached lifetime

This rulebook does not define final stack configuration syntax.

## Shutdown

Thread shutdown is cooperative by default.

Safe Sec code must not forcefully kill an ordinary thread.

Hard termination may be added later only as an explicit target-specific unsafe
operation, with no cleanup guarantee.

## Blocking

Thread waits block or park a physical thread.

This differs from task suspension, which may suspend a logical task without
blocking the physical worker.

## Result

`Thread[T]` may produce a typed result.

The result access syntax is intentionally left open.

This document does not finalize `.value`, `join` result values or any equivalent
API.

## Thread-local storage

Sec v0.1 does not define thread-local storage syntax.

Task-local state and thread-local state are not the same concept.

## Future rules

Affinity and priority are future rules.

They are not implied by `spawn thread`.

## Diagnostics

Examples:

```text
target profile does not support physical threads
cannot await Thread[T]; use join or an explicit future adapter
cannot join detached thread worker
thread worker is unresolved at scope exit
detaching Thread[T] with non-void result requires explicit discard
```
