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
