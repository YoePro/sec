# Structured Concurrency

## Purpose

Structured concurrency defines optional higher-level ownership of child
execution entities.

This rulebook is planned and does not define final syntax.

It must account for:

- child task ownership
- child thread ownership
- cancellation propagation
- scope exit diagnostics
- detached exceptions
- result collection

It must not replace the move-only lifecycle rules for `Task[T]` and `Thread[T]`.

## Consuming await during caller cancellation

Structured scopes preserve the lifecycle obligation when caller cancellation
wins before a consuming await commits. The await operation keeps internal
ownership of the consumed child handle, invokes `RequestCancel()`, and waits
through terminal child cleanup; it must not detach the child or restore the
source binding.

Any child-held borrow into waiter-owned state remains live through that cleanup.
The owning state and its destruction must therefore remain valid until the
child reaches a terminal outcome and releases the borrow.
