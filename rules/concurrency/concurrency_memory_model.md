# Concurrency Memory Model

- **Status:** Normative
- **Created:** 2026-09-06
- **Last updated:** 2026-09-06
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/concurrency_memory_model.md`
- **Replaces:** Earlier unversioned revision at the same canonical path
- **Repository baseline reviewed:** `0f5027d`
- **Related rulebooks:** `rules/concurrency/concurrency.md`, `rules/concurrency/atomics.md`, `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/concurrency/mutex.md`, `rules/concurrency/channels.md`, `rules/concurrency/cancellation.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/transferability.md`, `rules/analysis/data_races.md`, `rules/compiler/semantic_ir.md`, `rules/platform/target_profiles.md`, `rules/platform/platform_model.md`, `rules/platform/ffi.md`, `rules/platform/volatile.md`

---

## § 1. Purpose

**Governance tags:** `concurrency.memory-model-v2`

§ 1(1) This rulebook defines when memory effects performed by one Sec execution context become visible to another.

§ 1(2) It defines program order, synchronization, publication, happens-before, atomic ordering, per-atomic modification order, release sequences, fences, mutex ordering, execution completion ordering, and compiler/backend reordering constraints.

§ 1(3) It applies to tasks, threads, mixed task/thread execution, interrupt interaction where the platform rules permit it, foreign callbacks, and other shared-memory execution contexts recognized by Sec.

§ 1(4) Process and IPC memory visibility is governed by the explicit transport/shared-memory contract and does not inherit ordinary in-process shared-memory semantics automatically.

§ 1(5) Data-race analysis consumes this memory model but is owned by `data_races.md`.

---

## § 2. Core principle

**Governance tags:** `concurrency.memory-model-v2`, `sema.data-race-analysis`

§ 2(1) Ownership and borrowing determine who may access a value.

§ 2(2) The concurrency memory model determines when memory effects may be observed safely across concurrent execution contexts.

§ 2(3) A program must satisfy both models.

§ 2(4) Valid lifetime does not imply valid concurrent access.

§ 2(5) Valid synchronization does not make an invalid reference valid.

§ 2(6) Memory ordering does not create ownership.

§ 2(7) Ownership transfer does not permit the compiler to omit the publication ordering required by an execution boundary.

---

## § 3. Terminology

**Governance tags:** `concurrency.memory-model-v2`

§ 3(1) An **execution context** is a task, physical thread, interrupt context, foreign callback context, or another canonical context that may overlap another context.

§ 3(2) A **memory effect** is a read, write, atomic operation, synchronization operation, publication event, fence, or other canonical operation whose ordering can affect shared-memory observation.

§ 3(3) **Program order** is the order required by Sec evaluation/control-flow semantics within one execution context.

§ 3(4) **Synchronizes-with** is a cross-context relation created by a synchronization rule.

§ 3(5) **Happens-before** is the transitive ordering relation formed from program-order and synchronizes-with edges.

§ 3(6) **Modification order** is the total order of modifications for one atomic object.

§ 3(7) A **release sequence** is the release-headed contiguous RMW modification chain defined by this rulebook.

§ 3(8) **Publication** is the act of making data reachable/observable by another execution context through a defined boundary or synchronization mechanism.

---

## § 4. Exact memory-order type

**Governance tags:** `concurrency.memory-model-v2`, `concurrency.atomics-v2`, `tooling.atomics-v2`

§ 4(1) The canonical source-visible memory-order type is:

```sec
enum MemoryOrder {
    Relaxed
    // Atomicity only for the atomic operation itself.

    Acquire
    // Acquire ordering for an operation that reads atomic state.

    Release
    // Release ordering for an operation that writes/publishes atomic state.

    AcqRel
    // Combined acquire and release ordering for read-modify-write operations.

    SeqCst
    // Sequentially consistent ordering and participation in the global SeqCst order.
}
```

§ 4(2) `MemoryOrder` is compiler-known and also a real source-visible core enum.

§ 4(3) The compiler and LSP must not implement only the name while omitting its variants or documentation.

§ 4(4) The exact ownership, core placement, and API requirements are cross-defined with `atomics.md`.

---

## § 5. Exact compare-exchange result type

**Governance tags:** `concurrency.memory-model-v2`, `concurrency.atomics-v2`

§ 5(1) Compare-exchange uses the canonical result type:

```sec
enum CompareExchangeResult[T] {
    Exchanged
    // Compare matched and the desired value was stored.

    NotExchanged(T)
    // Compare did not exchange.
    // The payload is the value observed by that atomic operation.
}
```

§ 5(2) A successful `CompareExchange` is an atomic read-modify-write.

§ 5(3) A failed `CompareExchange` is an atomic read only.

§ 5(4) The success/failure distinction therefore affects modification order and release-sequence participation.

---

## § 6. Exact fence surface

**Governance tags:** `concurrency.memory-model-v2`, `concurrency.atomics-v2`

§ 6(1) Sec 0.1 provides:

```sec
atomic.Fence(order: MemoryOrder) void
// Establishes an explicit ordering fence.
// The function itself does not access an Atomic[T].
```

§ 6(2) The argument is mandatory.

§ 6(3) Valid orders are:

```text
Acquire
Release
AcqRel
SeqCst
```

§ 6(4) `Relaxed` is invalid for `atomic.Fence`.

§ 6(5) `atomic.Fence` has no implicit/default `SeqCst` overload.

---

## § 7. Referenced compiler-known concurrency types

**Governance tags:** `concurrency.memory-model-v2`, `concurrency.tasks-v2`, `tooling.atomics-v2`

§ 7(1) This memory model materially depends on task and mutex types defined by their owning concurrency rulebooks. Their relevant source surface is repeated here so an implementation cannot satisfy this rulebook by recognizing only their names.

§ 7(2) The canonical task-handle type-use syntax is:

```sec
Task[T]
// `Task` is the compiler-known nominal generic task-handle type.
// `[T]` supplies exactly one type argument.
// `T` is the complete declared return type of the spawned task body.
// The physical handle representation is compiler/runtime-defined and is not
// exposed as ordinary source fields.
```

§ 7(3) `Task[T]` is move-only and carries lifecycle responsibility according to `tasks.md`.

§ 7(4) Awaiting `Task[T]` has the exact static result type `TaskOutcome[T]`.

§ 7(5) The canonical source-visible task outcome declaration is:

```sec
type TaskOutcome[T] union {
    Completed(T)
    // The task function completed normally.
    // The payload is exactly the task function's declared return type T.

    Cancelled
    // The task terminated through cooperative task cancellation.
    // No T payload is produced.

    Panicked(PanicInfo)
    // A panic escaped the task boundary under a runtime/profile that can
    // represent task-local panic termination.
    // PanicInfo is the canonical panic-information type.

    Failed(TaskError)
    // An already-created task failed at the task-execution/runtime layer.
    // TaskError is distinct from an application Result error returned as T.
}
```

§ 7(6) `type ... union` declares a Sec union type; `[T]` declares one generic type parameter; `Completed(T)`, `Panicked(PanicInfo)`, and `Failed(TaskError)` are payload-bearing variants; `Cancelled` is payload-less.

§ 7(7) `TaskOutcome[T]` is a real source-visible core declaration and must be exposed by compiler Sema and LSP tooling according to `tasks.md`.

§ 7(8) The memory model materially depends on the compiler-known generic mutex types:

```sec
Mutex[T]
// Owns one protected value of T together with one synchronization identity.

MutexGuard[T]
// Represents one successful exclusive acquisition of Mutex[T].
// It is move-only and provides controlled access to the protected T.
```

§ 7(9) `Mutex[T]` and `MutexGuard[T]` are nominal compiler-known generic types. Their physical lock/runtime representation is not an ordinary source-visible struct layout and must not be invented by this rulebook.

§ 7(10) The mutex operations relevant to this memory model have the canonical public surface:

```sec
impl Mutex[T] {
    fn Lock() MutexGuard[T]
    // Waits until exclusive ownership of the mutex acquisition is obtained.
    // Successful acquisition provides acquire synchronization.

    fn TryLock() Option[MutexGuard[T]]
    // Attempts immediate acquisition.
    // Some(MutexGuard[T]) means acquisition succeeded and provides acquire
    // synchronization; None means no acquisition occurred.
}
```

§ 7(11) `impl Mutex[T]` associates the public methods with the generic mutex type. Instance-method `self` is implicit according to `impl.md` and is therefore not written in these signatures.

§ 7(12) `Lock` and `TryLock` use the canonical public CamelCase naming rule. Older lowercase spellings in legacy mutex material are non-canonical and must be synchronized by the corrections workflow.

§ 7(13) Destruction of the owning `MutexGuard[T]` releases the acquisition; that release provides the mutex release synchronization defined by this memory model and `mutex.md`.

§ 7(14) The exact complete task and mutex APIs remain owned by `tasks.md` and `mutex.md`. Repeating the relevant surface here does not authorize a competing declaration or alternate spelling.

---

## § 8. Sequential execution and program order

**Governance tags:** `concurrency.memory-model-v2`

§ 8(1) Within one execution context, Sec evaluation and control-flow semantics establish program order.

§ 8(2) If operation A is sequenced before operation B by Sec semantics, the compiler may transform them only when all observable single-context and concurrent semantics remain valid.

§ 8(3) Program order contributes edges to happens-before.

§ 8(4) A release operation constrains preceding operations from being observed as if they occurred after the release when that would violate the release contract.

§ 8(5) An acquire operation constrains following operations from being observed as if they occurred before the acquire when that would violate the acquire contract.

---

## § 9. Happens-before

**Governance tags:** `concurrency.memory-model-v2`, `sema.data-race-analysis`

§ 9(1) If A happens-before B, the effects of A must be visible to B to the extent required by the relevant storage and synchronization semantics.

§ 9(2) Happens-before includes program-order edges.

§ 9(3) Happens-before includes synchronizes-with edges.

§ 9(4) Happens-before is transitive.

If:

```text
A happens-before B
B happens-before C
```

then:

```text
A happens-before C
```

§ 9(5) The compiler and backend must preserve the observable consequences of happens-before.

---

## § 10. Synchronizes-with

**Governance tags:** `concurrency.memory-model-v2`

§ 10(1) Synchronizes-with is created only by a canonical synchronization rule.

§ 10(2) Examples include:

- release/acquire atomic communication;
- release-sequence communication;
- valid fence/atomic communication;
- mutex release followed by a successful later acquisition of the same mutex;
- successful execution completion followed by the owning join/await synchronization defined by the relevant execution-kind rulebook;
- channel send/receive commit where the channel rule defines publication;
- spawn publication;
- explicit FFI/platform synchronization contracts.

§ 10(3) Ordinary function calls do not create cross-context synchronizes-with edges merely by being function calls.

§ 10(4) Volatile access does not create a synchronizes-with edge.

§ 10(5) Status polling does not create completion synchronization unless the owning status API explicitly defines such semantics.

---

## § 11. Spawn publication

**Governance tags:** `concurrency.memory-model-v2`, `frontend.transferability`

§ 11(1) Values moved or copied into successfully created task/thread execution are published before the child first accesses those received values.

Example:

```sec
let data := BuildData()
let worker := try spawn Process(<-data)
```

§ 11(2) Initialization writes sequenced before the successful creation publication happen-before the child execution's first access to the transferred value.

§ 11(3) The same principle applies to valid captured owned values.

§ 11(4) A borrow crossing an execution boundary remains subject to lifetime, aliasing, address-stability, and transferability rules.

§ 11(5) Publication does not make later unsynchronized shared mutation valid.

§ 11(6) Process publication is defined only through explicit process/IPC semantics.

---

## § 12. Task completion

**Governance tags:** `concurrency.memory-model-v2`, `concurrency.tasks-v2`

§ 12(1) All ordinary writes validly performed by a task before terminal publication happen-before a successful task join or await that observes that terminal completion.

§ 12(2) Awaiting `Task[T]` yields the canonical `TaskOutcome[T]` defined by `tasks.md`.

§ 12(3) The awaiting context must observe the fully initialized terminal outcome and all effects that the task validly publishes through completion.

§ 12(4) Completion synchronization does not retroactively make prior invalid data races valid.

§ 12(5) Reading a task status property without the synchronization promised by join/await is not a substitute for completion synchronization.

---

## § 13. Thread completion

**Governance tags:** `concurrency.memory-model-v2`

§ 13(1) Writes validly performed by a thread before terminal publication happen-before a successful join that observes that termination.

§ 13(2) Thread detach does not itself establish a completion synchronization edge to the detaching context.

§ 13(3) Thread completion memory semantics remain distinct from task scheduling implementation.

---

## § 14. Mutex ordering

**Governance tags:** `concurrency.memory-model-v2`, `sema.data-race-analysis`

§ 14(1) Successful unlock/release of a mutex provides release synchronization.

§ 14(2) A later successful lock/acquisition of the same mutex identity provides acquire synchronization according to `mutex.md`.

§ 14(3) The release synchronizes with the matching later successful acquisition according to mutex semantics.

§ 14(4) Protected writes before release happen-before accesses after the synchronizing acquisition.

§ 14(5) Two different mutex identities do not synchronize merely because they protect values of the same type.

§ 14(6) A live `MutexGuard[T]` remains subject to the execution-boundary and suspension rules in `mutex.md`.

---

## § 15. Atomic indivisibility

**Governance tags:** `concurrency.memory-model-v2`, `concurrency.atomics-v2`

§ 15(1) A Sec atomic operation is indivisible with respect to other atomic operations on the same atomic storage identity.

§ 15(2) Atomic loads observe one complete atomic value permitted by the memory model.

§ 15(3) Atomic stores and RMW operations publish one complete modification.

§ 15(4) Atomic operations must not tear.

§ 15(5) A target implementation must preserve indivisibility for the complete concrete atomic value.

---

## § 16. Per-atomic modification order

**Governance tags:** `concurrency.memory-model-v2`, `concurrency.atomics-v2`

§ 16(1) Every atomic storage identity has its own total modification order.

§ 16(2) Modification order is per atomic object; Sec does not define one global modification order for all atomic objects.

§ 16(3) Every modification of one atomic object appears exactly once in that object's modification order.

§ 16(4) Modifications include:

```text
Store
Swap
FetchAdd
FetchSub
FetchAnd
FetchOr
FetchXor
successful CompareExchange
```

§ 16(5) Pure reads do not appear in modification order.

§ 16(6) `Load` is a pure atomic read.

§ 16(7) failed `CompareExchange` is a pure atomic read.

§ 16(8) `atomic.Fence` is not a modification of an atomic object.

§ 16(9) Modification order must remain coherent with happens-before constraints on modifications to the same atomic object.

---

## § 17. RMW coherence

**Governance tags:** `concurrency.memory-model-v2`, `concurrency.atomics-v2`

§ 17(1) A read-modify-write operation reads and modifies one coherent atomic state as one indivisible operation.

§ 17(2) The value read by an RMW is the value immediately preceding that RMW modification in the atomic object's modification order, subject to the operation's success condition for compare-exchange.

§ 17(3) A successful `CompareExchange` therefore reads the state it replaces.

§ 17(4) A failed `CompareExchange` does not enter modification order and returns the value it observed through `NotExchanged(T)`.

---

## § 18. Relaxed ordering

**Governance tags:** `concurrency.memory-model-v2`

§ 18(1) `MemoryOrder.Relaxed` guarantees atomicity and modification-order coherence for the atomic object.

§ 18(2) It does not create acquire synchronization for later ordinary memory.

§ 18(3) It does not create release synchronization for earlier ordinary memory.

§ 18(4) Relaxed operations may still participate in a release sequence when they are RMW modifications following a release head.

Example:

```sec
Requests.FetchAdd(1, MemoryOrder.Relaxed)
```

does not by itself publish unrelated ordinary writes.

---

## § 19. Acquire ordering

**Governance tags:** `concurrency.memory-model-v2`

§ 19(1) `MemoryOrder.Acquire` applies to an atomic operation that reads atomic state.

§ 19(2) An acquire operation can synchronize with a qualifying release operation or release sequence when it observes a value carried by that synchronization relationship.

§ 19(3) Ordinary operations sequenced after the acquire must observe memory consistently with the established happens-before edge.

Example:

```sec
if Ready.Load(MemoryOrder.Acquire) {
    Use(Data)
}
```

§ 19(4) The acquire does not create publication merely because it executes; it must participate in a valid synchronization relation.

---

## § 20. Release ordering

**Governance tags:** `concurrency.memory-model-v2`

§ 20(1) `MemoryOrder.Release` applies to an atomic operation that performs a modification.

§ 20(2) Ordinary operations sequenced before the release are published through a matching acquire relation when the synchronization conditions are met.

Example:

```sec
Data.Value = 10
Ready.Store(true, MemoryOrder.Release)
```

§ 20(3) A matching acquire that observes the released value or a value carried through its release sequence can establish visibility of the prior write.

§ 20(4) Release does not itself perform acquire ordering.

---

## § 21. Acquire-release ordering

**Governance tags:** `concurrency.memory-model-v2`

§ 21(1) `MemoryOrder.AcqRel` combines acquire and release semantics.

§ 21(2) It is valid for read-modify-write operations.

§ 21(3) It is invalid for pure `Load`.

§ 21(4) It is invalid for pure `Store`.

§ 21(5) It is invalid as a failed compare-exchange order because that path performs no modification.

---

## § 22. Sequential consistency

**Governance tags:** `concurrency.memory-model-v2`

§ 22(1) `MemoryOrder.SeqCst` provides the acquire/release behavior appropriate to the operation and participates in one global order of sequentially consistent atomic operations and sequentially consistent fences.

§ 22(2) That global order must be consistent with the ordering requirements of this memory model.

§ 22(3) All execution contexts must observe the SeqCst operations consistently with one permitted total SeqCst order.

§ 22(4) `SeqCst` is the default for ordinary atomic object operations when no explicit order is supplied.

§ 22(5) The compiler may lower an operation using a stronger target primitive when semantics remain equivalent, but must not weaken the requested ordering.

---

## § 23. Operation/order matrix

**Governance tags:** `concurrency.memory-model-v2`, `concurrency.atomics-v2`

§ 23(1) Valid orders are:

| Operation category | Relaxed | Acquire | Release | AcqRel | SeqCst |
|---|---:|---:|---:|---:|---:|
| atomic `Load` | yes | yes | no | no | yes |
| atomic `Store` | yes | no | yes | no | yes |
| atomic RMW success | yes | yes | yes | yes | yes |
| `CompareExchange` failure | yes | yes | no | no | yes |
| `atomic.Fence` | no | yes | yes | yes | yes |

§ 23(2) Invalid combinations are compile-time semantic errors.

---

## § 24. Compare-exchange ordering

**Governance tags:** `concurrency.memory-model-v2`, `concurrency.atomics-v2`

§ 24(1) `CompareExchange` has a success order and a failure order.

§ 24(2) The success order is applied only when the exchange occurs.

§ 24(3) The failure order is applied only when the exchange does not occur.

§ 24(4) Failure orders are limited to:

```text
Relaxed
Acquire
SeqCst
```

§ 24(5) `Release` and `AcqRel` are invalid failure orders because no modification occurs on that path.

§ 24(6) Sec imposes no additional language rule requiring the failure order to be weaker than the success order.

§ 24(7) A success path and failure path may therefore request different ordering strengths for different semantic reasons.

---

## § 25. Release-sequence definition

**Governance tags:** `concurrency.memory-model-v2`, `concurrency.atomics-v2`

§ 25(1) A release sequence belongs to one atomic storage identity.

§ 25(2) It begins with a release modification on that atomic object.

§ 25(3) The head may be a `Release`, `AcqRel`, or `SeqCst` modification with release semantics.

§ 25(4) The release sequence continues only through a contiguous sequence of atomic read-modify-write modifications on the same atomic object.

§ 25(5) The RMW modifications extending the sequence may use `MemoryOrder.Relaxed`.

§ 25(6) The RMW operations may be performed by execution contexts different from the context that performed the release head.

§ 25(7) A plain later `Store` is not an RMW and therefore terminates the preceding release sequence.

§ 25(8) A failed `CompareExchange` is not a modification and does not extend the release sequence.

§ 25(9) A successful `CompareExchange` is RMW and may extend the release sequence.

---

## § 26. Release-sequence example

**Governance tags:** `concurrency.memory-model-v2`

Given:

```sec
// Producer
Data.Value = 42
Counter.Store(1, MemoryOrder.Release)

// Another execution context
Counter.FetchAdd(1, MemoryOrder.Relaxed)

// Consumer
let observed := Counter.Load(MemoryOrder.Acquire)
```

§ 26(1) The release store creates the head modification.

§ 26(2) The relaxed `FetchAdd` is an RMW modification immediately following the head in the relevant release sequence when no intervening non-RMW modification breaks it.

§ 26(3) If the acquire load observes the value produced by that RMW, it can synchronize with the release head through the release sequence.

§ 26(4) The producer's prior write to `Data.Value` then happens-before the consumer operations sequenced after the acquire.

---

## § 27. Release-sequence break

**Governance tags:** `concurrency.memory-model-v2`

Given:

```sec
Counter.Store(1, MemoryOrder.Release)
Counter.FetchAdd(1, MemoryOrder.Relaxed)
Counter.Store(100, MemoryOrder.Relaxed)
Counter.FetchAdd(1, MemoryOrder.Relaxed)
```

§ 27(1) The first `FetchAdd` can extend the release sequence headed by the release store.

§ 27(2) The plain relaxed `Store(100, ...)` is a new non-RMW modification and breaks that earlier release sequence.

§ 27(3) The later `FetchAdd` is not part of the original release sequence headed by the first release store.

§ 27(4) An acquire that observes the later value does not automatically receive the publication associated with the old release head merely because an RMW occurred later.

---

## § 28. Release/acquire publication

**Governance tags:** `concurrency.memory-model-v2`

§ 28(1) A release modification can publish ordinary prior writes.

§ 28(2) An acquire operation that reads the release modification or a value carried through its valid release sequence can synchronize with that release.

§ 28(3) The synchronizes-with edge plus program-order edges establishes happens-before.

§ 28(4) The relation is based on actual atomic communication, not merely on using `Release` and `Acquire` somewhere in the program.

---

## § 29. Release fence to acquire atomic

**Governance tags:** `concurrency.memory-model-v2`

§ 29(1) A release fence can participate in synchronization when it is sequenced before an atomic modification that communicates with an acquire operation.

Conceptual pattern:

```sec
Data.Value = 42

atomic.Fence(MemoryOrder.Release)
Ready.Store(true, MemoryOrder.Relaxed)

// Another execution context:
if Ready.Load(MemoryOrder.Acquire) {
    Use(Data)
}
```

§ 29(2) The release fence does not synchronize directly with the acquire operation merely because both exist.

§ 29(3) The relaxed atomic modification after the release fence provides the atomic communication carrier.

§ 29(4) When the acquire operation observes the qualifying value, the release-fence publication participates in the happens-before relation.

---

## § 30. Release atomic to acquire fence

**Governance tags:** `concurrency.memory-model-v2`

§ 30(1) An acquire fence can participate in synchronization when an atomic read sequenced before the fence observes a qualifying release modification or release sequence.

Conceptual pattern:

```sec
// Producer
Data.Value = 42
Ready.Store(true, MemoryOrder.Release)

// Consumer
let ready := Ready.Load(MemoryOrder.Relaxed)

if ready {
    atomic.Fence(MemoryOrder.Acquire)
    Use(Data)
}
```

§ 30(2) The acquire fence does not turn an unrelated prior load into a synchronization carrier.

§ 30(3) The prior atomic load must observe the qualifying release-carried value.

---

## § 31. Fence-to-fence synchronization

**Governance tags:** `concurrency.memory-model-v2`

§ 31(1) A release fence and acquire fence can synchronize through atomic communication.

Conceptual pattern:

```sec
// Producer
Data.Value = 42
atomic.Fence(MemoryOrder.Release)
Ready.Store(true, MemoryOrder.Relaxed)

// Consumer
let ready := Ready.Load(MemoryOrder.Relaxed)

if ready {
    atomic.Fence(MemoryOrder.Acquire)
    Use(Data)
}
```

§ 31(2) The producer's relaxed atomic modification and consumer's relaxed atomic read are the communication carrier between the fences.

§ 31(3) Two fences without the required atomic communication do not synchronize.

§ 31(4) A fence must not be documented as a universal global "flush memory" operation.

---

## § 32. `AcqRel` fence

**Governance tags:** `concurrency.memory-model-v2`

§ 32(1) `atomic.Fence(MemoryOrder.AcqRel)` combines the applicable release-fence and acquire-fence constraints at one program point.

§ 32(2) It does not itself read from or modify an atomic object.

§ 32(3) Synchronization still requires the appropriate atomic communication relation.

---

## § 33. `SeqCst` fence

**Governance tags:** `concurrency.memory-model-v2`

§ 33(1) `atomic.Fence(MemoryOrder.SeqCst)` provides acquire/release fence semantics and participates in the global SeqCst order.

§ 33(2) It does not itself create an atomic value modification.

§ 33(3) SeqCst fence ordering must remain consistent with the other sequentially consistent operations required by the program.

---

## § 34. Atomic and ordinary memory

**Governance tags:** `concurrency.memory-model-v2`, `sema.data-race-analysis`

§ 34(1) Atomic access to one atomic object does not automatically synchronize unrelated ordinary storage.

§ 34(2) A relaxed counter update does not publish unrelated ordinary writes.

§ 34(3) Publication of ordinary writes requires a defined synchronization mechanism such as release/acquire communication, mutex synchronization, channel publication, completion synchronization, spawn publication, or another explicit contract.

§ 34(4) The presence of an atomic object in a struct does not make all fields of the struct concurrently safe.

---

## § 35. Data races

**Governance tags:** `concurrency.memory-model-v2`, `sema.data-race-analysis`

§ 35(1) Ordinary unsynchronized data races are invalid Sec behavior.

§ 35(2) This rulebook defines the synchronization/happens-before facts consumed by canonical race analysis.

§ 35(3) `data_races.md` owns access pairing, Place overlap, proof classifications, diagnostics, and incremental analysis.

§ 35(4) No atomic memory order can make overlapping ordinary `ref mut` borrows valid if the borrow rules already prohibit them.

§ 35(5) `unsafe` does not make a data race semantically valid.

---

## § 36. Atomic interior mutation

**Governance tags:** `concurrency.memory-model-v2`, `concurrency.atomics-v2`

§ 36(1) Atomic methods may mutate contained atomic state through a shared reference because `Atomic[T]` defines compiler-known synchronized interior mutation.

Example:

```sec
fn Increment(counter: ref Atomic[uint64]) void {
    counter.FetchAdd(1)
}
```

§ 36(2) This rule applies only to the atomic object's contained state.

§ 36(3) It does not authorize unsynchronized mutation of surrounding ordinary memory.

---

## § 37. Publication and address stability

**Governance tags:** `concurrency.memory-model-v2`, `analysis.transferability`

§ 37(1) Publication can make storage reachable by additional execution contexts.

§ 37(2) After publication, source-level movement must preserve all live address and synchronization identities.

§ 37(3) Atomics, mutexes, shared references, task captures, wait queues, interrupt registrations, and foreign registrations may therefore impose address-stability requirements.

§ 37(4) The backend may use indirection where allowed, but source-level identity must remain stable.

---

## § 38. Static storage

**Governance tags:** `concurrency.memory-model-v2`, `sema.data-race-analysis`

§ 38(1) Static lifetime is not synchronization.

§ 38(2) Immutable static storage may be shared after valid initialization.

§ 38(3) Mutable static storage requires synchronization or proof of non-overlapping exclusive access.

§ 38(4) Static `Atomic[T]` and `Mutex[T]` values provide their specialized synchronization semantics; ordinary mutable static values do not gain such semantics automatically.

---

## § 39. Initialization publication

**Governance tags:** `concurrency.memory-model-v2`

§ 39(1) Compile-time static initialization is visible before concurrent execution that begins after program initialization.

§ 39(2) Runtime initialization must be ordered before publication to another execution context.

§ 39(3) A task or thread must not observe partially initialized storage through an invalid publication path.

§ 39(4) The compiler must reject statically provable initialization/publication races.

---

## § 40. Cancellation visibility

**Governance tags:** `concurrency.memory-model-v2`

§ 40(1) Cancellation state is communicated through the canonical task cancellation mechanism.

§ 40(2) Cancellation visibility does not imply publication of unrelated application data.

§ 40(3) Cleanup performed during cooperative cancellation retains the ordinary synchronization semantics of any mutex release, atomic operation, channel operation, or other primitive used during cleanup.

§ 40(4) Forced external termination is not equivalent to cooperative cancellation and may not provide cleanup publication guarantees.

---

## § 41. Blocking and yielding

**Governance tags:** `concurrency.memory-model-v2`

§ 41(1) Blocking or suspension does not by itself create memory synchronization.

§ 41(2) A mutex acquisition can create synchronization because the mutex contract defines it.

§ 41(3) Await/join can create completion synchronization because the execution-kind contract defines it.

§ 41(4) Plain sleep does not publish ordinary memory by itself.

§ 41(5) Plain scheduler yield does not publish ordinary memory by itself.

§ 41(6) A context switch is not a source-level memory barrier unless a canonical primitive gives it such semantics.

---

## § 42. Volatile

**Governance tags:** `concurrency.memory-model-v2`, `compiler.platform-model`

§ 42(1) Volatile access is not synchronization.

§ 42(2) Volatile does not provide atomicity.

§ 42(3) Volatile does not provide acquire semantics.

§ 42(4) Volatile does not provide release semantics.

§ 42(5) Volatile does not create race freedom.

§ 42(6) Memory-mapped I/O may require volatile access and separate hardware ordering/completion mechanisms.

§ 42(7) The compiler must keep volatile and atomic semantics distinct in Semantic IR.

---

## § 43. FFI

**Governance tags:** `concurrency.memory-model-v2`, `compiler.platform-model`

§ 43(1) A foreign function call does not automatically publish memory to other execution contexts.

§ 43(2) Foreign synchronization must be described by a verified FFI/platform contract before it can satisfy Sec synchronization requirements.

§ 43(3) FFI contracts may need to define atomic representation, memory ordering, callback concurrency, retention, thread affinity, shared-memory ownership, and volatile effects.

§ 43(4) The compiler must not infer a synchronization contract from a native function name.

---

## § 44. Interrupts

**Governance tags:** `concurrency.memory-model-v2`, `platform.atomics-v2`, `sema.data-race-analysis`

§ 44(1) Interrupt handlers form concurrent/preemptive execution contexts when platform semantics permit overlap with ordinary code.

§ 44(2) Shared storage between ISR and ordinary execution must use primitives valid for the selected `CompilationPlan`.

§ 44(3) Ordinary mutexes are not assumed interrupt-safe.

§ 44(4) Atomic operations are ISR-usable only when the concrete target lowering is valid in that context.

§ 44(5) Device-specific barriers are separate from generic Sec memory fences unless the platform rule explicitly maps them.

---

## § 45. Lock-free algorithms and reclamation

**Governance tags:** `concurrency.memory-model-v2`, `concurrency.atomics-v2`

§ 45(1) Correct atomic ordering does not by itself establish safe memory reclamation.

§ 45(2) Lock-free pointer algorithms remain subject to ownership, lifetime, ABA, reclamation, and transferability rules.

§ 45(3) Compare-exchange success does not prove that a pointer identity remained continuously live between observations.

§ 45(4) The compiler must not infer reclamation safety from the presence of atomic operations.

---

## § 46. No out-of-thin-air values

**Governance tags:** `concurrency.memory-model-v2`

§ 46(1) Atomic and ordinary operations must not produce a value without a valid source in the program execution permitted by Sec semantics.

§ 46(2) Compiler speculation and backend transformations must not introduce out-of-thin-air values.

§ 46(3) This restriction applies even when a weaker hardware model would otherwise permit aggressive speculation.

---

## § 47. Compiler reordering

**Governance tags:** `concurrency.memory-model-v2`, `analysis.semantic-ir-v2`

§ 47(1) The compiler may reorder operations only when all observable Sec semantics remain valid.

§ 47(2) It must preserve acquire boundaries.

§ 47(3) It must preserve release boundaries.

§ 47(4) It must preserve SeqCst ordering.

§ 47(5) It must preserve modification-order and RMW semantics.

§ 47(6) It must preserve release-sequence semantics.

§ 47(7) It must preserve fence relationships.

§ 47(8) It must preserve mutex synchronization.

§ 47(9) It must preserve spawn publication.

§ 47(10) It must preserve completion synchronization.

§ 47(11) It must preserve FFI/platform synchronization contracts.

§ 47(12) It must preserve volatile observable-order rules without upgrading volatile into general synchronization.

---

## § 48. Hardware reordering

**Governance tags:** `concurrency.memory-model-v2`, `compiler.platform-model`

§ 48(1) The backend must emit target operations sufficient to enforce the source-level memory order.

§ 48(2) The same Sec source semantics apply independently of whether the target is strongly or weakly ordered.

§ 48(3) Stronger target hardware does not weaken source requirements.

§ 48(4) Weaker target hardware requires appropriate target barriers/instructions/runtime mechanisms.

§ 48(5) The backend must not use compiler-host memory ordering as a substitute for target lowering.

---

## § 49. Sequential consistency does not cover ordinary racy code

**Governance tags:** `concurrency.memory-model-v2`

§ 49(1) Sec does not guarantee one global sequential order for all ordinary concurrent accesses.

§ 49(2) Sequential consistency in this rulebook concerns SeqCst atomics/fences and their defined relationships.

§ 49(3) Ordinary shared-memory accesses require valid ownership and synchronization.

§ 49(4) A program containing an invalid data race does not become valid because another atomic object uses `SeqCst`.

---

## § 50. Message passing

**Governance tags:** `concurrency.memory-model-v2`, `frontend.transferability`

§ 50(1) A successful message-send/receive pair establishes ownership and visibility according to the owning channel/message primitive.

§ 50(2) A sent owned value must be fully initialized at the transfer commit point.

§ 50(3) A successful receive obtains the ownership/publication guarantee defined by the primitive.

§ 50(4) Channel memory semantics are not inferred from implementation queues; they are source-level contracts.

---

## § 51. Process boundaries

**Governance tags:** `concurrency.memory-model-v2`

§ 51(1) Separate processes do not share ordinary Sec memory by default.

§ 51(2) IPC establishes visibility according to the transport contract.

§ 51(3) Explicit shared-memory IPC must separately define mapping lifetime, ownership, synchronization, atomic compatibility, process lifetime, and failure behavior.

§ 51(4) This book does not invent process-specific lifecycle or termination semantics.

---

## § 52. Semantic analysis

**Governance tags:** `concurrency.memory-model-v2`, `sema.data-race-analysis`

§ 52(1) Semantic analysis must preserve enough facts to validate atomic orderings before lowering.

§ 52(2) It must distinguish atomic object identity.

§ 52(3) It must distinguish read, write, and RMW operations.

§ 52(4) It must distinguish successful and failed compare-exchange paths.

§ 52(5) It must validate the exact order matrix.

§ 52(6) It must preserve publication, execution-context, and synchronization facts consumed by race/deadlock analysis.

§ 52(7) It must not defer invalid memory-order combinations to LLVM or another backend verifier.

---

## § 53. Semantic IR

**Governance tags:** `concurrency.memory-model-v2`, `analysis.semantic-ir-v2`, `semantic-ir.atomics-v2`

§ 53(1) Semantic IR must preserve synchronization explicitly enough for verification and lowering.

§ 53(2) It must preserve:

- program-order relevant effects;
- spawn publication;
- completion synchronization;
- mutex acquire/release;
- atomic loads/stores/RMW;
- compare-exchange success/failure;
- explicit fences;
- memory orders;
- atomic storage identity;
- synchronization source/target relationships where resolved;
- source provenance;
- platform/FFI ordering contracts.

§ 53(3) Concrete Semantic IR opcode names are owned by `semantic_ir.md`.

§ 53(4) Low-level lowering must not infer atomic semantics from ordinary load/store instructions or naming conventions.

---

## § 54. LSP and diagnostics

**Governance tags:** `tooling.atomics-v2`, `concurrency.memory-model-v2`

§ 54(1) LSP hover must expose the exact `MemoryOrder` declaration and documentation.

§ 54(2) LSP diagnostics must use the same order-validation facts as compiler Sema.

§ 54(3) An invalid order should produce a focused diagnostic, for example:

```text
MemoryOrder.Release is not valid for Atomic.Load
```

§ 54(4) Compare-exchange diagnostics should distinguish the success order from the failure order.

§ 54(5) A target capability failure should remain distinguishable from a language-semantic order error.

§ 54(6) Tooling must not suggest status polling, volatile access, or sleep/yield as a substitute for required synchronization.

---

## § 55. Restrictions

**Governance tags:** `concurrency.memory-model-v2`

§ 55(1) Static lifetime must not be treated as synchronization.

§ 55(2) Volatile access must not be treated as atomic or synchronized access.

§ 55(3) Task status polling must not be treated as task completion synchronization.

§ 55(4) Ordinary data races must not be permitted.

§ 55(5) Requested atomic ordering must not be weakened.

§ 55(6) Atomic values must not tear.

§ 55(7) Speculation must not create out-of-thin-air values.

§ 55(8) Memory ordering must not bypass ownership or borrowing.

§ 55(9) The compiler must not assume one universal hardware memory model.

§ 55(10) Process IPC must not be assumed to share ordinary in-process task memory semantics.

§ 55(11) A plain `Store` must not be treated as an RMW merely to extend a release sequence.

§ 55(12) A failed `CompareExchange` must not be treated as a modification.

§ 55(13) Two fences must not be treated as synchronizing without the required atomic communication relation.

---

## § 56. Governance

**Governance tags:** `concurrency.memory-model-v2`, `concurrency.atomics-v2`, `sema.data-race-analysis`, `analysis.semantic-ir-v2`, `semantic-ir.atomics-v2`, `tooling.atomics-v2`, `compiler.platform-model`, `frontend.transferability`

§ 56(1) Mutable implementation information for this rulebook must be maintained in `implementation-status.yaml`.

§ 56(2) The primary governance integration is `concurrency.memory-model-v2`.

§ 56(3) Atomic API implementation is tracked through `concurrency.atomics-v2` and its frontend/tooling/IR/lowering/platform integrations.

§ 56(4) Data-race analysis consumes this rulebook without redefining its memory-order semantics.

§ 56(5) Semantic IR and lowering must preserve the normative modification-order, release-sequence, fence, and compare-exchange semantics even when current implementation coverage is partial.

§ 56(6) Cross-rulebook synchronization required by this revision is tracked in the accompanying corrections document.
