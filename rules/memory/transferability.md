# Transferability

## Purpose

Transferability defines which values, references and capabilities may cross
execution boundaries.

The relevant boundaries are:

```text
task boundary
physical thread boundary
process boundary
interrupt boundary
foreign callback boundary
```

A value does not become transferable merely because it appears in `spawn`,
a channel or unsafe code.

---

## General principle

Transferability is derived from the complete value graph.

A composite value is transferable only when every relevant contained value and
capability is transferable under the selected boundary.

The compiler must not reduce transferability to one shallow marker.

---

## Compiler-known capabilities

The compiler may internally derive capabilities equivalent to:

```text
TaskTransferable
ThreadTransferable
ProcessTransferable
ISRTransferable
ThreadSafeShared
```

These are semantic capabilities, not necessarily user-written interfaces.

A type may satisfy one without satisfying another.

Example:

- an owned file wrapper may move to another thread;
- the same wrapper may not cross a process boundary;
- a shared immutable string may be thread-safe;
- a thread-local reference is not task-migration-safe.

---

## Owned values into tasks

An owned argument passed into a spawned task follows ordinary copy or move rules.

Move-only values move.

Copyable values may copy.

The task owns transferred values until it moves, returns or destroys them.

Task transfer does not require the value to be safe for simultaneous access when
ownership becomes exclusive to the task.

---

## Owned values into threads

An owned argument passed into a spawned physical thread must satisfy thread
transfer requirements.

At minimum:

- its representation is valid in another physical thread;
- destruction is valid in that thread or is explicitly constrained;
- it contains no thread-bound references;
- it contains no live mutex guard;
- it does not depend on stack storage that ends before the thread;
- target ABI and runtime requirements are satisfied.

Exclusive ownership transfer may make a non-shareable value transferable.

Thread-safe sharing is a separate question.

---

## Borrowed values

A borrowed argument may cross a task or thread boundary only when the compiler
proves:

- the referent outlives all possible use;
- conflicting access is impossible;
- the reference remains valid after migration where applicable;
- detached execution cannot outlive the referent;
- storage remains address-stable where required.

A `ref mut` transfer grants exclusive access for the entire proven borrow
interval.

The original owner cannot access the value during that interval.

---

## Detached execution

A detached task or thread must not retain ordinary references to scope-owned
values.

Valid detached state normally consists of:

- owned moved values;
- static immutable references;
- synchronized static mutable state;
- allocator- or arena-owned values with sufficient lifetime;
- explicit runtime-owned resources.

Static lifetime does not make concurrent mutation safe.

---

## Shared immutable values

A shared immutable value may be used by several tasks or threads when:

- the type is deeply immutable for the shared duration;
- no hidden mutable state is reachable;
- lazy initialization is synchronized;
- reference counts or metadata are thread-safe where present.

A type with logical immutability but unsynchronized internal mutation is not
automatically thread-safe.

---

## Shared mutable values

Shared mutable state requires a valid synchronization mechanism such as:

- `Mutex[T]`;
- valid atomic storage;
- channel ownership transfer;
- another compiler-known synchronized type.

A raw shared `ref mut` must not be duplicated across concurrent execution
entities.

---

## Move-only lifecycle handles

The following are move-only owners:

```text
Task[T]
Thread[T]
Process
Subscription
Sender[T]
Receiver[T]
MessageTicket[T]
```

They may move to another valid owner when their boundary-specific rules permit.

Moving a handle transfers responsibility.

It does not duplicate the underlying execution entity or resource.

---

## Observers

Observer handles may be copyable when their representation supports safe
retained observation.

Examples:

```text
TaskObserver
ThreadObserver
ProcessObserver
```

An observer does not gain lifecycle or result ownership through transfer.

Copying an observer must not preserve join-only native resources.

---

## Mutex guards

`MutexGuard[T]` is not transferable.

It must not be:

- moved to another task;
- moved to another physical thread;
- captured by spawned work;
- sent through a channel;
- stored for detached execution;
- kept across suspension or join.

This applies even if the protected `T` would otherwise be transferable.

---

## Thread-local values

A `ThreadLocal[T]` declaration identifies one value per physical thread.

The static key may be referenced globally.

A reference obtained from:

```sec
ThreadLocalValue.value
```

is thread-bound.

It must not:

- move to another physical thread;
- cross `await`;
- cross a task scheduling point where migration is possible;
- be returned with a wider lifetime;
- be captured by work running elsewhere.

Owned copies extracted from thread-local storage follow the ordinary rules of
their type.

---

## Closures

A closure crossing an execution boundary is transferable only when all captures
are valid.

The compiler must classify each capture as:

- copied;
- moved;
- shared borrow;
- mutable borrow;
- thread-bound;
- static;
- raw/unsafe.

A closure containing a non-transferable capture is non-transferable.

A closure must not hide transfer through an opaque callable representation.

---

## Method receivers

Spawning a method transfers or borrows its receiver according to the method
signature.

The compiler must treat implicit `self` exactly like an explicit argument for:

- ownership;
- lifetime;
- thread transfer;
- escape;
- detach;
- data-race analysis.

Implicit receiver syntax must not weaken checks.

---

## Collections and aggregates

Arrays, structs, unions, tuples and collection-like values derive
transferability recursively.

A collection is thread-transferable only when:

- its elements are transferable;
- its allocator or storage backend permits cross-thread ownership;
- its internal metadata may be destroyed in the destination context;
- no active iterator or borrow invalidates transfer.

A collection with synchronized internal storage may additionally be shareable.

---

## Named and distinct types

A named or distinct type does not automatically change transferability.

It inherits representation-based restrictions unless its implementation adds:

- thread-bound state;
- synchronized state;
- custom destruction;
- FFI affinity;
- target restrictions.

The compiler must evaluate the complete semantic type, not only the base
representation.

---

## Raw pointers

`RawPtr[T]` is not automatically transferable or thread-safe.

Unsafe code may move or copy a raw pointer, but the programmer remains
responsible for:

- lifetime;
- ownership;
- aliasing;
- target address-space validity;
- synchronization;
- thread affinity;
- foreign API contracts.

`unsafe` does not make a data race valid.

---

## FFI handles

A foreign handle may be:

- thread-affine;
- process-local;
- freely transferable;
- copyable but not concurrently usable;
- callback-thread-bound.

Its wrapper or extern contract must declare the correct capability.

Unknown foreign handles are conservatively non-transferable in safe code.

---

## Process boundaries

Ordinary Sec references and pointers do not cross a process boundary.

Process transfer requires an explicit IPC adapter or shared-memory contract.

A process-transferable value must define:

- representation or serialization;
- ownership after send;
- failure behavior;
- versioning where relevant;
- handle duplication or transfer;
- target support.

An in-process `Channel[T]` is not IPC.

---

## Interrupt boundaries

An ISR may access only values valid under interrupt rules.

Transfer from ISR to ordinary code must use:

- ISR-safe atomics;
- fixed ISR-safe queues/channels;
- bounded notifications;
- volatile/MMIO capture followed by deferred processing.

An ordinary closure, mutex guard or heap-backed collection is not ISR
transferable by default.

---

## Static variables

Immutable static values may be shared when fully initialized and deeply
immutable.

Mutable static values require synchronization.

A mutable static does not become transferable merely because all access occurs
inside `unsafe`.

---

## Result and Option

`Result[T, E]` is transferable only when both `T` and `E` satisfy the boundary.

`Option[T]` is transferable only when `T` satisfies the boundary.

The same recursive rule applies to task and thread outcomes.

---

## Compiler analysis

The compiler must determine:

- transfer boundary;
- owned and borrowed arguments;
- capture modes;
- recursive field capabilities;
- allocator/storage constraints;
- thread affinity;
- active borrows;
- address stability;
- destruction context;
- detach escape;
- process representation;
- ISR restrictions;
- foreign contracts.

Whole-program analysis should derive transferability automatically where
possible.

The language should not require users to annotate every ordinary type.

---

## Runtime errors

Transferability violations are compile-time semantic failures whenever
statically known.

This rule introduces no generic runtime `TransferError`.

IPC and platform transfer failures belong to their specific APIs, and any
language-level runtime error types must be declared in:

```text
core/errors.sec
```

---

## Semantic IR

Semantic IR must record:

```text
TransferOwnedToTask
TransferOwnedToThread
TransferBorrowToTask
TransferBorrowToThread
TransferObserver
TransferToProcessAdapter
TransferFromISR
```

Each transfer operation must retain:

- boundary kind;
- source owner;
- destination owner;
- copied or moved state;
- retained borrow;
- transfer capability proof;
- source location.

---

## Diagnostics

Examples:

```text
value socket is thread-affine and cannot move to thread worker
```

```text
detached thread worker cannot retain reference to local value data
```

```text
MutexGuard[State] cannot cross a task boundary
```

```text
reference to thread-local LastError cannot remain live across await
```

```text
RawPtr[Packet] is not proven safe for cross-thread transfer
```

```text
ordinary Channel[Message] cannot cross a process boundary
```

Diagnostics must have stable IDs and identify the concrete boundary.

---

## Required synchronization

Cross-check and update:

```text
spawn.md
tasks.txt
threads.md
processes.txt
channels.md
mutex.md
thread_local.md
static.md
atomics.md
events.md
ownership.md
borrowing.txt
references.txt
raw_pointers.txt
copy_move.md
lifetime_analysis.txt
destruction.txt
platform/ffi.md
concurrency.md
concurrency_memory_model.txt
data_races.md
structured_concurrency.md
semantic_ir.txt
diagnostics.txt
core/errors.sec
```
