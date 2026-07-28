# Concurrency Runtime Model

## Purpose

The concurrency runtime model describes which runtime services a target profile
may provide.

This rulebook is planned and does not define final syntax.

It must preserve Sec's no-required-runtime principle.

Targets may use:

- bare-metal executors
- RTOS primitives
- operating-system threads
- compiler state machines
- hosted schedulers
- direct native synchronization primitives

The selected runtime model must not change source-level ownership, lifecycle,
result or synchronization semantics.
