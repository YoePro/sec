# Concurrency Memory Model

## Purpose

The concurrency memory model defines when writes performed by one execution
entity become visible to another execution entity.

It also defines:

- synchronization
- publication
- happens-before relationships
- atomic ordering
- mutex ordering
- task and thread completion ordering
- data-race validity
- compiler reordering limits
- backend lowering requirements

The memory model applies to all concurrent Sec execution.

---

## Core principle

Ordinary ownership and borrowing determine who may access a value.

The concurrency memory model determines when concurrent accesses may observe
writes safely.

A program must satisfy both models.

Valid lifetime does not imply valid concurrent access.

Valid synchronization does not make an invalid reference valid.

---

## Sequential execution

Within one execution entity, operations follow normal Sec evaluation and
control-flow rules.

The compiler may optimize and reorder operations only when observable behavior is
preserved.

Observable behavior includes:

- ordinary single-task semantics
- atomic operations
- mutex synchronization
- task or thread creation
- task or thread completion
- cancellation
- FFI effects
- volatile effects
- process and IPC effects

---

## Concurrency visibility

A write by one task or thread is guaranteed visible to another task or thread
only when a defined synchronization relationship exists.

Examples include:

- ownership transfer into a spawned task
- successful mutex unlock followed by lock
- release atomic followed by matching acquire atomic
- successful task completion followed by join or await
- successful thread completion followed by join
- process or IPC synchronization defined by its transport

Without synchronization, concurrent visibility is not guaranteed.

---

## Happens-before

The memory model uses a happens-before relation.

If operation A happens-before operation B, effects of A must be visible to B as
defined by the relevant synchronization primitive.

Happens-before is:

- ordered within one execution entity
- created by synchronization operations
- transitive

If:

```text
A happens-before B
B happens-before C
```

then:

```text
A happens-before C
```

---

## Program order

Within one execution entity, earlier operations happen-before later operations
when ordinary control flow requires that order.

Example:

```sec
state.value = 10
Ready.store(true, MemoryOrder.Release)
```

The write to `state.value` happens-before the release store in program order.

The compiler must not reorder the ordinary write after the release store.

---

## Spawn publication

Values moved or copied into a newly spawned task are published to that task.

Example:

```sec
let data := BuildData()
let worker := spawn Process(data)
```

All writes that initialize `data` before `spawn` happen-before the child task's
first access to its received value.

This applies to:

- moved arguments
- copied arguments
- captured owned values
- receiver values
- task metadata required for startup

The backend must not begin child access before argument publication is complete.

The same publication rule applies to `spawn thread`.

All writes sequenced before successful thread creation happen-before the new
thread's first access to its received values.

The same rule applies to process creation only through explicitly defined
process and IPC publication rules.

---

## Spawned borrows

A reference passed to a spawned task does not create ownership.

The referenced storage must remain alive and must not be accessed in conflict.

Publication guarantees that writes completed before spawn are visible to the
child task.

Example:

```sec
let data := BuildData()
let worker := spawn Inspect(ref data)
```

The child may observe the initialized `data`.

Further concurrent mutation requires valid synchronization.

The same lifetime and synchronization requirements apply to references passed to
a spawned thread.

---

## Task completion

All ordinary writes performed by a task before successful completion happen-before
a successful join or await that observes that completion.

Example:

```sec
let worker := spawn BuildResult()
let result := await worker
```

The awaiting task must observe the fully initialized result and all memory effects
that the task validly published through owned result transfer.

---

## Join

A successful `join task` establishes a synchronization edge from task completion
to the joining task.

After join:

- completion status is visible
- task failure status is visible
- cancellation status is visible
- stored result state is visible
- writes validly published by the task are visible

Reading `task.done` without join does not provide the same synchronization
guarantee.

---

## Await

A successful `await task` establishes the same completion synchronization as
join and additionally consumes the task handle.

All writes that happen-before task completion become visible to the awaiting task.

The result value is transferred according to ordinary ownership rules.

## Thread completion

All ordinary writes performed by a thread before successful termination
happen-before a successful `join threadHandle` that observes that termination.

After a successful thread join, writes validly published by that thread are
visible to the joining execution entity.

`detach threadHandle` does not establish a synchronization edge.

Detaching relinquishes join ownership; it does not publish ordinary writes to
another execution entity.

## Mixed synchronization

The same synchronization edges apply across:

- task to task
- task to thread
- thread to task
- thread to thread

Examples include:

- channel send happens-before the matching receive commit
- mutex unlock happens-before a later successful lock on the same mutex
- task completion happens-before successful task await or join
- thread completion happens-before successful thread join

Using a physical thread does not weaken ownership, borrowing or synchronization
requirements.

Data races are invalid Sec programs.

`unsafe` may permit operations the safe language cannot express, but it does not
make a data race valid.

---

## Status polling

Reading task status properties such as:

```sec
task.done
task.running
task.cancelled
task.failed
```

is an observation.

Status polling alone must not be used as a replacement for join or await when
memory synchronization is required.

The implementation may expose current state safely, but ordinary non-atomic data
written by the task is not thereby guaranteed visible.

---

## Mutex synchronization

Unlocking a mutex performs release synchronization.

A later successful lock of the same mutex performs acquire synchronization.

All writes performed while holding the guard happen-before accesses performed
after a later successful lock.

Example:

```sec
{
    let mut state := State.lock()
    state.value = 10
}
```

Later:

```sec
{
    let state := State.lock()
    Use(state.value)
}
```

The second guard must observe the synchronized protected state.

---

## Mutex guard scope

A `MutexGuard[T]` represents exclusive access to the protected value.

All ordinary reads and writes through the guard are synchronized by the lock.

The compiler may optimize within the guard scope but must not move protected
access:

- before successful lock acquisition
- after unlock
- across await
- across join
- into another task

---

## Non-reentrant identity

A mutex synchronization edge applies to the same mutex identity.

Two distinct `Mutex[T]` values do not synchronize merely because they protect
values of the same type.

Mutex identity must remain stable after publication.

---

## Atomic operations

Atomic operations are indivisible with respect to other atomic operations on the
same storage location.

They also provide ordering according to `MemoryOrder`.

The default order is:

```sec
MemoryOrder.SeqCst
```

---

## Relaxed ordering

```sec
MemoryOrder.Relaxed
```

guarantees atomicity for the operation itself.

It does not create acquire or release synchronization for surrounding ordinary
memory.

Relaxed atomics are suitable for independent counters and statistics.

Example:

```sec
Requests.fetchAdd(1, MemoryOrder.Relaxed)
```

This does not publish unrelated non-atomic writes.

---

## Acquire ordering

```sec
MemoryOrder.Acquire
```

prevents later memory operations in the same task from being reordered before
the acquire operation.

A successful acquire may synchronize with a release operation on the same atomic
or through a valid release sequence.

Example:

```sec
if Ready.load(MemoryOrder.Acquire) {
    Use(Data)
}
```

When the load observes a matching released value, prior published writes become
visible.

---

## Release ordering

```sec
MemoryOrder.Release
```

prevents earlier memory operations in the same task from being reordered after
the release operation.

Example:

```sec
Data.value = 10
Ready.store(true, MemoryOrder.Release)
```

A matching acquire load that observes `true` may then observe `Data.value == 10`.

---

## Acquire-release ordering

```sec
MemoryOrder.AcqRel
```

combines acquire and release semantics for read-modify-write operations.

It is not valid for a pure load or pure store when the operation does not support
both directions.

---

## Sequential consistency

```sec
MemoryOrder.SeqCst
```

provides acquire and release semantics and participates in one global
sequentially consistent order for sequentially consistent atomic operations.

This is the default because it is the easiest ordering to reason about.

The implementation may use stronger ordering than requested.

It must not use weaker ordering.

---

## Compare-exchange ordering

Compare-and-exchange has:

- success ordering
- failure ordering

The success order applies when the exchange occurs.

The failure order applies when the current value differs from `expected`.

Failure ordering may use:

```sec
MemoryOrder.Relaxed
MemoryOrder.Acquire
MemoryOrder.SeqCst
```

It must not use release-only semantics because no write occurs on failure.

---

## Release sequences

A release sequence may extend synchronization through compatible atomic
read-modify-write operations on the same atomic location.

Version 0.1 may expose this through the standard acquire/release behavior without
requiring source-level terminology.

Backend lowering must preserve the target's valid release-sequence semantics.

---

## Atomic and ordinary memory

Atomic access to one storage location does not automatically synchronize unrelated
ordinary storage.

Example:

```sec
Counter.fetchAdd(1, MemoryOrder.Relaxed)
```

does not publish another ordinary variable.

Publication requires:

- release/acquire ordering
- mutex synchronization
- task completion synchronization
- ownership transfer
- another defined synchronization mechanism

---

## Data races

A data race occurs when:

- two tasks access the same memory location concurrently
- at least one access writes
- the accesses are not ordered by happens-before
- the accesses are not valid atomic operations on the same atomic storage
- the accesses are not protected by the same synchronization primitive

Data races are invalid Sec programs.

The compiler must reject statically provable data races.

Sec does not define ordinary data races as acceptable behavior.

---

## Data-race consequences

The language must not rely on C-style undefined behavior as the primary user
model for ordinary data races.

A data race should result in:

- compile-time error when statically provable
- checked runtime diagnostic when supported and not statically provable
- explicit unsafe responsibility only when raw memory access prevents proof

Unsafe code does not make a data race valid.

---

## Conflicting borrows

Concurrent shared references are valid only for read-only access.

Concurrent mutable access requires exclusive ownership or synchronization.

The borrow checker and memory model work together.

Example:

```sec
let mut state := State.Create()

let first := spawn Update(ref mut state)
let second := spawn Update(ref mut state)
```

This is invalid before backend lowering.

No memory ordering can make two overlapping `ref mut` borrows valid.

---

## Atomics and aliasing

Atomic mutation through shared references is valid because `Atomic[T]` defines
its own synchronized interior mutation.

Example:

```sec
fn Increment(counter: ref Atomic[uint64]) void {
    counter.fetchAdd(1)
}
```

This does not permit ordinary unsynchronized mutation of surrounding fields.

---

## Publication

A value becomes published when another task may access it.

Publication may occur through:

- spawn argument transfer
- task capture
- shared static storage
- storing a reference in shared synchronized storage
- atomic pointer publication
- IPC or process transfer
- explicit runtime registration

Before publication, a uniquely owned value may be moved freely.

After publication, address stability and synchronization rules may restrict
movement.

---

## Address stability

Mutexes, atomics and references shared with another task may require stable
storage identity.

The compiler must reject moves that may invalidate:

- mutex identity
- atomic identity
- shared references
- task captures
- wait queues
- backend synchronization state

A backend may use movable indirection internally, but source-level identity must
remain stable.

---

## Static storage

Static storage is published whenever multiple tasks may access it.

Immutable static storage may be shared when validly initialized.

Mutable static storage requires synchronization.

Example:

```sec
static let State: Mutex[ApplicationState]
```

A plain:

```sec
static let mut State: ApplicationState
```

must not be accessed concurrently without proof of exclusive access.

---

## Initialization publication

Compile-time static initialization is visible before concurrent task execution
begins.

Runtime initialization must be explicit.

A task must not observe partially initialized static storage.

The compiler must reject or guard execution paths where task creation may occur
before required runtime initialization completes.

---

## Cancellation visibility

A cancellation request must become visible to the target task through the task
cancellation mechanism.

Reading:

```sec
task.cancelRequested
```

must safely observe the cancellation state.

The implementation may use atomic or scheduler-managed storage.

Cancellation visibility does not imply synchronization of unrelated application
data.

---

## Cancellation and cleanup

A task that observes cancellation and exits through `cancel` performs normal
cleanup.

Cleanup operations remain ordered within the task.

Mutex unlocks, atomic releases and destruction performed during cancellation
retain their normal synchronization semantics.

Forced process termination provides no such guarantee.

---

## Blocking operations

A blocking or suspending operation must define whether it creates synchronization.

Examples:

```text
Mutex.lock()
    acquire synchronization

await
    task completion synchronization

join
    task completion synchronization

plain sleep
    no publication by itself

plain scheduler yield
    no publication by itself
```

Yielding execution is not a memory synchronization primitive.

---

## Fences

Version 0.1 may expose explicit memory fences only for low-level code.

Conceptual form:

```sec
atomic.fence(MemoryOrder.SeqCst)
```

A fence orders memory but does not access an atomic value itself.

Fences should be restricted to valid memory orders.

Most application code should use mutexes, task synchronization or ordinary
atomic operations instead.

---

## Compiler reordering

The compiler may reorder ordinary operations only when all observable semantics
remain valid.

It must preserve:

- acquire boundaries
- release boundaries
- sequentially consistent order
- mutex lock and unlock boundaries
- spawn publication
- task completion publication
- FFI barriers
- volatile access order
- explicit fences

It must not move an ordinary access across a synchronization edge when doing so
changes visibility.

---

## Hardware reordering

The backend must emit instructions or runtime calls sufficient to enforce the
requested memory order on the target architecture.

The same Sec source semantics apply on:

- x86-64
- ARM64
- ARM32
- RISC-V
- RTOS targets
- bare-metal targets

A stronger hardware model must not weaken source-level guarantees.

A weaker hardware model requires appropriate barriers.

---

## Sequential consistency of ordinary code

Sec does not guarantee that all ordinary concurrent accesses appear in one global
sequential order.

Only properly synchronized programs receive defined cross-task visibility.

Within one task, ordinary semantics remain deterministic subject to defined
external effects.

---

## Message passing

Message passing establishes ownership and visibility according to the message
primitive.

A sent owned value must be fully initialized before transfer.

A successful receive obtains the value and required visibility.

Detailed channel or IPC semantics are defined separately.

Message passing must not expose partially initialized values.

---

## Process boundaries

Processes do not share ordinary Sec memory by default.

IPC creates visibility according to the transport.

Shared-memory IPC must define:

- ownership
- synchronization
- atomic compatibility
- process lifetime
- mapping lifetime
- failure behavior

Ordinary task memory-order guarantees do not automatically apply across process
boundaries unless the primitive explicitly defines them.

---

## FFI

Foreign code may use a different memory model.

FFI boundaries must specify or wrap:

- atomic representation
- memory ordering
- thread safety
- callback concurrency
- shared-memory ownership
- volatile behavior

The compiler must not assume that an ordinary foreign function call publishes
memory safely.

A foreign synchronization primitive requires an explicit adapter.

---

## Volatile

Volatile access is not synchronization.

It may prevent removal or merging of specific accesses but does not provide:

- atomicity
- ownership
- acquire semantics
- release semantics
- data-race safety

Memory-mapped I/O may require volatile rules in addition to synchronization.

---

## Interrupts

ISR communication requires primitives valid for the selected target profile.

Ordinary mutexes are not interrupt-safe by default.

Atomics may be used only when the operation is supported and interrupt-safe.

The memory model must account for execution between ordinary code and interrupt
handlers.

Target profiles may define additional device barriers.

---

## Lock-free algorithms

Lock-free algorithms must still satisfy:

- ownership
- lifetime
- atomic ordering
- memory reclamation
- ABA prevention where required

Atomic pointers do not by themselves make an algorithm memory-safe.

The compiler must not infer reclamation safety from compare-exchange usage.

---

## Out-of-thin-air values

Atomic and ordinary operations must not produce values without a valid source in
the program execution.

The compiler and backend must not introduce out-of-thin-air values through
speculative transformations.

---

## Tearing

Atomic operations must not tear.

A successful atomic load observes one complete value permitted by the atomic
ordering.

Ordinary concurrent non-atomic access that may tear is invalid when it forms a
data race.

Target support must be verified for the complete atomic width.

---

## False sharing

False sharing is a performance concern, not a semantic error.

A future attribute may permit cache-line alignment or padding.

The language memory model does not guarantee cache placement.

---

## Semantic analysis

The compiler should determine:

- publication points
- task ownership transfer
- active shared references
- conflicting accesses
- mutex synchronization
- atomic ordering validity
- task completion synchronization
- static initialization visibility
- address-stability requirements
- FFI synchronization effects
- interrupt-safe atomic support
- statically provable happens-before relations

Whole-program analysis should be used when available.

---

## Semantic IR

Semantic IR must preserve synchronization explicitly.

At minimum:

```text
TaskPublish
TaskComplete
TaskJoin
TaskAwait
MutexAcquire
MutexRelease
AtomicLoad
AtomicStore
AtomicReadModifyWrite
AtomicCompareExchange
AtomicFence
StaticPublish
IPCTransfer
```

IR must record:

- storage identity
- operation kind
- memory order
- task identity
- synchronization source
- synchronization target
- publication state
- source location

Low-level lowering must not infer memory ordering from naming conventions.

---

## Diagnostics

Examples:

```text
shared mutable access to State is not ordered by synchronization
```

```text
atomic load cannot use MemoryOrder.Release
```

```text
compare-exchange failure order cannot use release semantics
```

```text
task status polling does not synchronize access to result data; use join or await
```

```text
cannot move mutex State after publication
```

```text
concurrent ordinary and atomic access to the same storage is invalid
```

```text
runtime initialization of State may race with task creation
```

```text
target arm32 cannot provide required atomic uint64 operation
```

---

## Restrictions

The memory model must not:

- make static lifetime equivalent to synchronization
- make volatile equivalent to atomic
- make task status polling equivalent to join
- permit ordinary data races
- weaken requested atomic ordering
- invent values through speculation
- allow tearing of atomic values
- bypass ownership or borrowing
- assume one universal hardware memory model
- assume process IPC shares ordinary task memory semantics

---

## Related rules

Detailed behavior is defined in:

```text
tasks.txt
spawn.txt
await.txt
concurrency.txt
mutex.txt
atomics.txt
static.txt
processes.txt
spawn_process.txt
ipc.txt
ffi.txt
```
