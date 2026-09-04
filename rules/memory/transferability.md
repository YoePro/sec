# Transferability

- Status: Normative
- Created: 2026-09-02
- Last updated: 2026-09-02
- Document revision: 2.0
- Sec language version: 0.1
- Canonical path: `rules/memory/transferability.md`
- Replaces: previous revision of `rules/memory/transferability.md`
- Repository baseline reviewed: `814a584` (latest publicly verifiable `main` while this revision was prepared)

---

## § 1 Purpose and authority

§ 1(1) This rulebook defines when Sec values, references, capabilities, and closures may cross execution or isolation boundaries.

§ 1(2) Relevant boundary categories include:

```text
task boundary
physical thread boundary
process boundary
interrupt boundary
foreign callback boundary
```

§ 1(3) Transferability is boundary-specific.

§ 1(4) A value may be transferable across one boundary and non-transferable across another.

§ 1(5) Transferability is evaluated for the actual value and transfer operation, not merely for the nominal type name.

§ 1(6) Transferability is derived from the complete reachable semantic value graph and the active execution policy.

§ 1(7) `rules/memory/ownership.md` owns ownership and Place availability.

§ 1(8) `rules/memory/copy_move.md` owns copy/move classification and explicit ownership-transfer syntax.

§ 1(9) `rules/memory/borrowing.md` owns shared/mutable borrow authority and overlap.

§ 1(10) `rules/memory/lifetime_analysis.md` owns lifetime proof and escape.

§ 1(11) `rules/memory/references.md` and `reference_model.md` own safe-reference validity and provenance.

§ 1(12) `rules/memory/raw_pointers.md` owns raw-address semantics.

§ 1(13) `rules/memory/destruction.md` owns destruction and cleanup.

§ 1(14) Concurrency rulebooks own task/thread/process/channel/synchronization APIs and execution mechanics.

§ 1(15) `rules/platform/interrupts.md` owns ISR execution-context restrictions.

§ 1(16) FFI rulebooks own foreign ownership, retention, callback-thread, and ABI contracts.

§ 1(17) This rulebook does not introduce a new ownership, lifetime, borrow, or concurrency model.

---

## § 2 Core principle

§ 2(1) Crossing a boundary is valid only when the complete transfer contract is valid.

§ 2(2) The transfer contract includes, where relevant:

```text
ownership mode
copy/move legality
reference lifetime
borrow compatibility
thread or task affinity
destruction context
storage origin
address stability
allocator/storage-domain restrictions
synchronization
foreign contracts
process representation
interrupt restrictions
target/runtime policy
```

§ 2(3) A value does not become transferable merely because it appears in `spawn`, a channel operation, unsafe code, an FFI call, or an interrupt-related API.

§ 2(4) Task-boundary transfer follows `rules/concurrency/tasks.md`. Reusable
move-only sources use the ownership-v2 explicit move syntax, while borrows,
migration, address stability, detach lifetime, and simultaneous shared access
retain their ordinary transferability obligations. Wrapping `T` in
`TaskOutcome[T]` does not independently change the transferability of `T`;
ordinary union/payload ownership rules apply.

§ 2(4) `unsafe` does not make a data race, dangling reference, thread-affinity violation, or invalid process transfer valid.

§ 2(5) The compiler must reject statically known invalid transfers.

§ 2(6) This rulebook introduces no generic runtime `TransferError`.

§ 2(7) Runtime failure caused by IPC, platform, queue capacity, process startup, or a foreign API belongs to the specific API that performs the operation.

---

## § 3 Transfer facts and internal capabilities

§ 3(1) The compiler may internally derive semantic capabilities equivalent to:

```text
TaskTransferable
ThreadTransferable
ProcessTransferable
ISRTransferable
ThreadSafeShared
```

§ 3(2) These names are compiler semantic facts, not required user-written interfaces.

§ 3(3) Sec 0.1 does not require programmers to annotate every ordinary type with transferability traits.

§ 3(4) The compiler must derive transferability automatically when the required facts are available.

§ 3(5) A boundary capability must not erase more specific restrictions such as:

```text
thread affinity
active borrow
local lifetime
current owner
destination destruction restriction
runtime mapping lifetime
foreign callback-thread restriction
ISR access-context restriction
```

§ 3(6) A type-level capability may be used as a reusable summary only when it is valid for every value of that type under the summarized conditions.

§ 3(7) Value-specific facts may make one instance transferable while another instance of the same nominal type is not.

§ 3(8) Example reasons include a reference to local storage, a thread-affine foreign handle, a currently active borrow, or a runtime-selected storage backend.

---

## § 4 Transferability is not shareability

§ 4(1) Exclusive ownership transfer and concurrent sharing are separate concepts.

§ 4(2) A value may be safe to move to another task or thread without being safe for simultaneous access from both.

§ 4(3) `ThreadTransferable` does not imply `ThreadSafeShared`.

§ 4(4) `ThreadSafeShared` does not imply that the value may be destroyed in any thread.

§ 4(5) A move-only owner may be transferable because ownership becomes exclusive to the destination.

§ 4(6) A copyable value may still be non-transferable because a copied value contains thread-bound or process-local capability.

§ 4(7) Shareability analysis must include all reachable mutable state and synchronization behavior.

---

## § 5 Value-graph derivation

§ 5(1) Composite transferability is recursive.

§ 5(2) For a struct, every semantically contained field or capability relevant to the selected transfer must satisfy that boundary.

§ 5(3) For an array or fixed aggregate, every reachable element category must satisfy the boundary.

§ 5(4) For `Option[T]`, transferability requires the active payload `T` to satisfy the boundary when present.

§ 5(5) For `Result[T, E]`, both possible payload families `T` and `E` must satisfy the boundary.

§ 5(6) For a union, every variant that can reach the transfer site must satisfy the boundary.

§ 5(7) For a collection, transferability includes:

```text
element transferability
storage/backend transferability
allocator/allocation-domain restrictions
internal metadata restrictions
destruction-context restrictions
active iterator/borrow restrictions
```

§ 5(8) A compiler may use active-variant/path facts to prove a specific value transferable without requiring unreachable inactive payloads to satisfy the transfer.

§ 5(9) A shallow marker on the outer type is insufficient when reachable contained capabilities violate the boundary.

---

## § 6 Ownership transfer

§ 6(1) Owned values cross a boundary using ordinary Sec copy/move rules.

§ 6(2) A move-only value must be explicitly moved at a boundary that consumes ownership where `copy_move.md` requires the `<-` marker.

§ 6(3) A copyable value may be copied only if the copied value is itself valid in the destination execution context.

§ 6(4) Ownership transfer gives the destination responsibility for the transferred owned value.

§ 6(5) Ownership transfer does not duplicate the underlying execution entity, resource, file descriptor, hardware mapping, foreign object, or other unique resource unless its type explicitly defines duplication semantics.

§ 6(6) After a consuming transfer, the source Place becomes unavailable according to `ownership.md`.

§ 6(7) A transfer operation must be fully validated before destructively committing ownership state.

---

## § 7 Borrowed transfers

§ 7(1) A borrowed value may cross an execution boundary only when the compiler proves the borrow remains valid for every possible use in the destination.

§ 7(2) Required proof includes:

```text
referent outlives all destination use
no conflicting access occurs
reference remains valid after execution migration where applicable
detached execution cannot outlive the referent
required storage remains address-stable
required generation/mapping remains valid
```

§ 7(3) A shared `ref T` does not make concurrent mutation safe.

§ 7(4) A `ref mut T` transferred to another execution entity grants exclusive mutable access for the complete proven borrow live range.

§ 7(5) The original owner and other aliases must not perform conflicting access during that live range.

§ 7(6) A borrow may end at final proven use rather than lexical scope end according to `borrowing.md`.

§ 7(7) Transferability analysis must use the same canonical borrow live-range facts as ordinary borrow checking.

§ 7(8) A borrow that is valid across a task boundary may still be invalid across a physical-thread boundary if the task may migrate to another thread and the referenced storage/capability is thread-bound.

---

## § 8 Task boundary

§ 8(1) A task boundary transfers execution ownership or borrowing into a separately scheduled task.

§ 8(2) Owned arguments passed to a task follow ordinary copy/move rules.

§ 8(3) A transferred owned value remains owned by the task until it is moved, returned, transferred, or destroyed.

§ 8(4) A task-local exclusive owner does not need to be safe for simultaneous access merely because it belongs to a task.

§ 8(5) Task transferability must include scheduler migration policy.

§ 8(6) When a task may migrate between physical threads, every live thread-affine dependency crossing a suspension/migration point must satisfy the physical-thread restrictions required by the scheduler.

§ 8(7) A task pinned to one physical thread may accept some thread-bound values that a migratable task cannot.

§ 8(8) Exact task pinning/scheduling APIs are owned by concurrency rulebooks.

§ 8(9) Transferability must not assume a task is pinned unless the canonical scheduling contract proves it.

---

## § 9 Physical thread boundary

§ 9(1) An owned value transferred to another physical thread must satisfy thread-transfer requirements.

§ 9(2) At minimum, the compiler must prove or consume a valid contract that:

```text
the representation is valid in the destination thread
destruction is permitted in the destination thread or explicitly constrained
no contained reference is bound to the source thread
no live guard/capability forbids transfer
no borrowed stack storage ends too early
target ABI/runtime restrictions are satisfied
```

§ 9(3) Exclusive ownership transfer may make a non-shareable value thread-transferable.

§ 9(4) Thread-safe sharing remains a separate proof.

§ 9(5) A thread-affine value is not transferable merely because its representation is trivially copyable.

§ 9(6) Destination-thread destruction restrictions are part of transferability.

---

## § 10 Detached execution

§ 10(1) Detached work may outlive the scope that created it.

§ 10(2) Detached execution must not retain ordinary references to scope-owned values whose lifetime may end first.

§ 10(3) Valid detached state normally consists of one or more of:

```text
owned moved values
static immutable references
synchronized static mutable state
allocation-domain-owned values with sufficient lifetime
explicit runtime-owned resources
validated foreign-owned resources
```

§ 10(4) Static lifetime does not make unsynchronized mutation safe.

§ 10(5) Detached execution must also satisfy panic-observation and lifecycle requirements defined by concurrency and panic rulebooks.

---

## § 11 Shared immutable values

§ 11(1) A value may be shared concurrently as immutable only when the complete reachable state is safe for that sharing.

§ 11(2) Required properties include, where relevant:

```text
deep immutability for the shared duration
no unsynchronized hidden mutable state
synchronized lazy initialization
thread-safe metadata/reference counting if present
valid storage lifetime
valid destruction/reclamation behavior
```

§ 11(3) Logical immutability at the API surface is insufficient if the implementation performs unsynchronized interior mutation.

§ 11(4) A shared immutable view into mutable storage is valid only while ordinary borrow/concurrency rules prevent invalid mutation.

---

## § 12 Shared mutable values

§ 12(1) Shared mutable state requires a canonical synchronization mechanism.

§ 12(2) Examples include:

```text
Mutex[T]
valid atomic storage
channel ownership transfer
compiler-known synchronized container
another synchronization primitive defined by concurrency rules
```

§ 12(3) A raw shared `ref mut` must not be duplicated across concurrent execution entities.

§ 12(4) `unsafe` does not legalize an unsynchronized data race.

§ 12(5) Volatile access is not synchronization.

§ 12(6) Interrupt masking is synchronization only where concurrency/interrupt analysis proves that it excludes every relevant conflicting execution context.

---

## § 13 Move-only lifecycle handles

§ 13(1) Lifecycle handles may be move-only owners.

Examples include:

```text
Task[T]
Thread[T]
Process
Subscription
Sender[T]
Receiver[T]
MessageTicket[T]
```

§ 13(2) Such a handle may move to another valid owner when its boundary-specific contract permits.

§ 13(3) Moving the handle transfers responsibility represented by that handle.

§ 13(4) Moving the handle does not duplicate the underlying execution entity or resource.

§ 13(5) A lifecycle handle may be non-transferable even if its fields are trivially movable when the represented runtime/native resource has affinity restrictions.

---

## § 14 Observer handles

§ 14(1) Observer handles may be copyable where their representation supports safe retained observation.

Examples include:

```text
TaskObserver
ThreadObserver
ProcessObserver
```

§ 14(2) Copying or transferring an observer does not grant lifecycle ownership.

§ 14(3) Copying an observer must not duplicate or retain native resources whose semantics are join-only or owner-only unless the observer abstraction explicitly provides a valid retained representation.

§ 14(4) Observer transferability is derived from the observer contract, not inferred from the observed object's owner type.

---

## § 15 Mutex guards and equivalent scoped capabilities

§ 15(1) `MutexGuard[T]` is non-transferable unless a future synchronization rule explicitly defines a transferable guard form.

§ 15(2) An ordinary `MutexGuard[T]` must not be:

```text
moved to another task
moved to another physical thread
captured by spawned work
sent through a channel
stored for detached execution
kept across suspension
kept across join/wait where the mutex rule forbids it
```

§ 15(3) This restriction applies even when protected `T` would otherwise be transferable.

§ 15(4) The same principle applies to other scoped capabilities whose validity depends on one execution scope, thread, lock state, interrupt mask, transaction, or foreign callback frame.

---

## § 16 Thread-local values

§ 16(1) A `ThreadLocal[T]` declaration identifies one logical value per physical thread according to the thread-local rulebook.

§ 16(2) The static thread-local key may be globally nameable without making any thread-specific value transferable.

§ 16(3) A reference obtained from a thread-local value is thread-bound.

§ 16(4) A thread-bound reference must not:

```text
move to another physical thread
cross await when task migration is possible
cross a task migration point
be returned with a wider lifetime/affinity
be captured by work running elsewhere
```

§ 16(5) Owned copies extracted from thread-local storage follow the ordinary transferability rules of the copied value.

§ 16(6) Copying a thread-local value does not carry the thread-local identity unless the copied type itself embeds thread affinity.

---

## § 17 Closures

§ 17(1) A closure crossing an execution boundary is transferable only when every capture is valid for that boundary.

§ 17(2) The compiler must retain each capture mode, including:

```text
owned copy
owned move
shared borrow
mutable borrow
thread-bound capture
static capture
raw/unsafe capture
foreign capability
```

§ 17(3) A closure with one non-transferable live capture is non-transferable across that boundary.

§ 17(4) Opaque callable representation must not hide non-transferable captures.

§ 17(5) Closure transferability is value-specific because different closure instances of the same callable shape may capture different origins or capabilities.

§ 17(6) Escaping closure lifetime remains governed by `lifetime_analysis.md`.

§ 17(7) A closure intended for ISR execution must additionally satisfy interrupt callable restrictions and ISR transferability.

---

## § 18 Method receivers

§ 18(1) A receiver participating in spawned/transferred execution is checked exactly like an explicit argument.

§ 18(2) The compiler must include receiver:

```text
ownership
copy/move mode
lifetime
borrow state
transfer boundary
thread affinity
escape
detach behavior
data-race implications
destruction context
```

§ 18(3) Implicit `self` syntax must not weaken transfer checks.

§ 18(4) An ordinary method does not gain permission to consume whole `self` merely because the method is used as a transfer entry point.

---

## § 19 Collections and aggregates

§ 19(1) Arrays, structs, unions, Results, Options, tuples/internal aggregates, collections, and shaped values derive transferability recursively.

§ 19(2) A collection transferred exclusively between threads requires at least:

```text
transferable elements
compatible storage backend
compatible allocation domain
destination-valid metadata
destination-valid destruction
no invalid active borrow/iterator/view
```

§ 19(3) A collection with synchronized internal storage may additionally be `ThreadSafeShared`.

§ 19(4) A view is non-owning and derives transferability from its referenced storage, lifetime, borrow authority, and thread/process/ISR restrictions.

§ 19(5) Moving an owning collection does not automatically make previously issued references or iterators transferable.

---

## § 20 Named and distinct types

§ 20(1) A named or distinct type does not automatically change transferability solely because it has a distinct nominal name.

§ 20(2) Transferability must include semantic restrictions introduced by the named type or its implementation.

§ 20(3) Such restrictions may include:

```text
thread-bound state
synchronized interior state
custom destruction
foreign affinity
platform restrictions
runtime mapping
special storage origin
```

§ 20(4) The compiler must inspect the complete semantic type contract, not only the underlying representation.

---

## § 21 Raw pointers

§ 21(1) `RawPtr[T]` is not automatically safe to transfer or share merely because the raw-pointer value itself is copyable.

§ 21(2) Raw-pointer value transfer copies or moves only an address value.

§ 21(3) Safe transfer analysis must not infer pointee lifetime, ownership, aliasing, synchronization, thread affinity, or address-space validity from `RawPtr[T]` alone.

§ 21(4) Safe code must not transfer a raw pointer into an execution context that will dereference it unless a canonical trusted/unsafe/FFI/platform contract establishes the required validity.

§ 21(5) Unsafe code remains responsible for:

```text
pointee lifetime
ownership contract
aliasing
target address-space validity
synchronization
thread affinity
foreign API restrictions
mapping lifetime
```

§ 21(6) `unsafe` does not make a data race or dangling raw-pointer use valid.

§ 21(7) A raw pointer crossing a process boundary is only an address representation and does not become a valid pointer in the destination process.

---

## § 22 FFI handles

§ 22(1) A foreign handle may be:

```text
thread-affine
process-local
freely transferable
copyable but not concurrently usable
callback-thread-bound
destructible only in a specific context
```

§ 22(2) The Sec wrapper or extern contract must expose the relevant capability facts.

§ 22(3) Unknown foreign handles are conservatively non-transferable in safe code when boundary safety cannot be proven.

§ 22(4) FFI metadata claiming transferability is a trusted contract and must be included in semantic analysis.

§ 22(5) A foreign handle may be transferable by ownership while remaining non-shareable.

§ 22(6) A handle may be copyable as a machine value while semantically non-copyable or non-transferable as a resource.

---

## § 23 Foreign callback boundary

§ 23(1) A callback invoked by foreign code crosses a foreign execution boundary.

§ 23(2) The callback contract must identify where relevant:

```text
calling thread or allowed thread set
whether callbacks may be concurrent
whether callback may occur after registration returns
whether callback may occur after owner destruction
reentrancy
panic/abort restrictions
blocking restrictions
allocation restrictions
```

§ 23(3) Captured/bound callback state must remain valid for the complete foreign callback lifetime.

§ 23(4) A callback-thread-bound foreign API may reject otherwise thread-transferable Sec captures.

§ 23(5) Unknown callback-thread behavior is conservative.

§ 23(6) Foreign callbacks do not bypass ordinary ownership, lifetime, borrowing, panic, or concurrency rules.

---

## § 24 Process boundary

§ 24(1) Ordinary safe references do not cross a process boundary.

§ 24(2) Ordinary `RawPtr[T]` values do not become valid destination-process pointers merely by copying their numeric representation.

§ 24(3) Process transfer requires an explicit IPC, serialization, shared-memory, duplicated-handle, inherited-handle, or platform-specific adapter contract.

§ 24(4) `ProcessTransferable` therefore includes the selected process-transfer representation/adapter.

§ 24(5) A process-transferable value must define or inherit sufficient rules for:

```text
representation or serialization
ownership after send
failure behavior
versioning where required
native handle duplication/transfer
shared-memory lifetime where used
target/platform support
```

§ 24(6) An in-process `Channel[T]` is not IPC merely because it transfers ownership between tasks or threads.

§ 24(7) A value can be thread-transferable while not being process-transferable.

§ 24(8) A pure serializable value can be process-transferable through an adapter even when its in-process representation contains implementation details that are not copied verbatim.

---

## § 25 Shared memory across processes

§ 25(1) Shared-memory process transfer is distinct from serialization.

§ 25(2) References into shared memory require a process-aware representation whose validity is defined by the IPC/shared-memory rulebook.

§ 25(3) Ordinary in-process `ref T` and `RawPtr[T]` must not be assumed meaningful in another process.

§ 25(4) Shared-memory synchronization must use process-compatible synchronization primitives.

§ 25(5) Shared-memory lifetime must cover every process use.

§ 25(6) Mapping ownership and remapping remain separate from logical value ownership.

---

## § 26 Interrupt boundary

§ 26(1) ISR execution uses ordinary Sec memory semantics plus interrupt-specific execution restrictions.

§ 26(2) `ISRTransferable` means that a value/capability may validly participate in the specified interrupt-boundary operation.

§ 26(3) ISR transferability is direction- and operation-sensitive.

§ 26(4) Transfer from ordinary code into state later observed by an ISR must use storage and synchronization permitted by `interrupts.md`.

§ 26(5) Transfer from ISR execution to deferred/ordinary code must use an ISR-safe handoff mechanism.

Examples may include:

```text
ISR-safe atomics
fixed-capacity ISR-safe queues/channels
bounded notifications
preallocated ownership handoff
volatile/MMIO capture followed by deferred processing
```

§ 26(6) An ordinary closure, mutex guard, dynamically growing collection, task-local borrow, or arbitrary allocator-backed object is not ISR-transferable by default.

§ 26(7) A value that is lifetime-safe may still be non-transferable into ISR execution because it violates `noPanic`, `noAlloc`, `noBlock`, bounded-work, synchronization, or hardware-access rules.

§ 26(8) `unsafe` does not waive interrupt transfer restrictions.

---

## § 27 ISR ownership handoff

§ 27(1) Ownership may cross an interrupt boundary only through an operation whose interrupt/concurrency contract explicitly permits that ownership transfer.

§ 27(2) The handoff must not allocate when ISR policy forbids allocation.

§ 27(3) The handoff must be bounded where ISR policy requires bounded work.

§ 27(4) The source must lose ownership when a consuming ISR handoff commits.

§ 27(5) The destination must acquire exactly one valid owner.

§ 27(6) Failed/non-committed handoff must preserve the source ownership state according to the handoff API contract.

§ 27(7) Hardware/DMA ownership transitions require the specific platform/DMA contract and are not inferred from raw register writes.

---

## § 28 Static values

§ 28(1) Immutable static values may be shared when fully initialized and deeply safe for the requested concurrent access.

§ 28(2) Mutable static values require appropriate synchronization for concurrent access.

§ 28(3) Static lifetime does not imply transferability, synchronization, interrupt safety, or foreign callback safety.

§ 28(4) A mutable static does not become safe merely because all access occurs inside `unsafe`.

§ 28(5) A static foreign/platform resource may still be thread-affine or execution-context-restricted.

---

## § 29 Allocation domains and transferability

§ 29(1) Allocation origin can affect transferability.

§ 29(2) A value allocated in one Arena may transfer ownership to another task/thread only when the Arena/storage lifetime covers the destination use and the allocation domain permits that execution context.

§ 29(3) Moving the value does not move or extend the Arena.

§ 29(4) A reference into an Arena does not keep the Arena alive.

§ 29(5) A thread-local or task-local allocator may make its allocated owners non-transferable even when the payload type is otherwise transferable.

§ 29(6) Transferability analysis must use canonical allocation/storage-domain facts rather than allocator variable names.

---

## § 30 Destruction context

§ 30(1) Transferability includes whether the value may be destroyed in the destination context.

§ 30(2) A type whose cleanup must execute on one specific thread is not freely thread-transferable unless transfer preserves or delegates that cleanup restriction.

§ 30(3) A type whose cleanup is forbidden in ISR execution is not transferable as an owned value whose destruction may occur in the ISR.

§ 30(4) Custom `free` restrictions are part of the transfer contract.

§ 30(5) Moving responsibility to another execution entity must not silently make previously invalid cleanup legal.

§ 30(6) A wrapper may implement destination-safe destruction by scheduling cleanup back to an allowed owner/context only when that behavior is explicitly part of its canonical contract.

---

## § 31 Active borrows and iterators

§ 31(1) An otherwise transferable owner may become temporarily non-transferable while incompatible borrows, guards, iterators, or views are live.

§ 31(2) The compiler may permit transfer after those live ranges end.

§ 31(3) Transfer must not invalidate a live safe reference or iterator in the source execution context.

§ 31(4) Moving an aggregate while a disjoint sub-Place is borrowed is permitted only when ordinary ownership/borrow rules permit the specific move and the transfer boundary can preserve every remaining dependency.

§ 31(5) Whole-value transfer requires the whole value to satisfy ordinary availability and borrow requirements.

---

## § 32 Partial and conditional ownership

§ 32(1) A partially available aggregate may transfer only the owned Place/subvalue that is itself valid to transfer.

§ 32(2) Whole-value transfer of a `PartiallyAvailable` aggregate is invalid.

§ 32(3) A `ConditionallyAvailable` Place may be transferred only on paths where ownership analysis proves the Place available or after a canonical convergence operation.

§ 32(4) Transferability analysis must not conflate `is available` with thread safety, lifetime validity, or transfer capability.

§ 32(5) A transferred sub-Place becomes unavailable at the source according to ownership semantics.

---

## § 33 Result and Option

§ 33(1) `Option[T]` derives transferability recursively from the possible active `T` payload.

§ 33(2) `Result[T, E]` derives transferability recursively from possible `T` and `E` payloads.

§ 33(3) A consuming `.Ok()` or `.Err()` transformation may produce a new value whose transferability is evaluated from the retained active payload.

§ 33(4) Task/thread/process outcome wrappers derive transferability from their payloads and observer/lifecycle semantics.

§ 33(5) Panic outcome metadata may have separate transfer constraints defined by panic/concurrency rulebooks.

---

## § 34 Channels

§ 34(1) Sending through an in-process channel is an execution-boundary transfer operation.

§ 34(2) Channel send must validate the boundary appropriate to the channel's execution domain.

§ 34(3) A task-only channel and a cross-thread channel may impose different transferability requirements.

§ 34(4) A channel does not make a non-transferable payload transferable.

§ 34(5) A consuming send transfers ownership only after the send operation commits according to the channel contract.

§ 34(6) Failed/non-committed send must preserve ownership according to the channel API's failure contract.

§ 34(7) Channel synchronization does not automatically make arbitrary references in a payload valid beyond their lifetime.

§ 34(8) IPC channels/process adapters are separate from ordinary in-process channels.

---

## § 35 Atomics and synchronized wrappers

§ 35(1) Atomic storage may permit shared cross-thread or ISR access only for types/operations supported by the atomic rulebook and target.

§ 35(2) Atomicity does not automatically make a surrounding aggregate thread-safe.

§ 35(3) Synchronized wrappers may provide `ThreadSafeShared` capability when their complete implementation preserves the synchronization contract.

§ 35(4) A synchronized wrapper may still be non-transferable because of allocator, destruction, foreign, or platform affinity.

§ 35(5) Volatile storage alone does not provide synchronized transferability.

---

## § 36 Task migration

§ 36(1) Task migration is a physical-thread boundary for thread-bound dependencies.

§ 36(2) A migratable task must not retain thread-bound references/capabilities across a scheduling point that may move it to another thread.

§ 36(3) Owned task state that is thread-transferable may migrate with the task.

§ 36(4) A pinned task may retain permitted thread-bound state while the pinning guarantee remains active.

§ 36(5) The compiler must consume scheduler/pinning facts from the canonical concurrency model rather than assume one policy globally.

§ 36(6) `await` is a migration-sensitive boundary when the scheduling model permits migration across suspension.

---

## § 37 Recursive and cyclic value graphs

§ 37(1) Transferability derivation may encounter recursive/cyclic type or ownership graphs.

§ 37(2) Compiler analysis must use deterministic fixed-point or equivalent canonical analysis.

§ 37(3) Recursive analysis must terminate.

§ 37(4) Unknown cycles must not be treated as positive proof of transferability.

§ 37(5) Runtime pointer cycles do not automatically imply non-transferability when ownership/synchronization/storage contracts provide a valid transferable abstraction.

---

## § 38 Generic code

§ 38(1) Generic callable/type transferability depends on instantiated type/value/capability facts.

§ 38(2) A generic API requiring transferability must express that requirement through compiler-known semantic constraints or a canonical future constraint mechanism.

§ 38(3) Sec 0.1 does not require introducing user-visible `TaskTransferable` or `ThreadTransferable` interfaces merely to implement generic transfer checks.

§ 38(4) Generic specialization must not erase thread affinity, active borrow, storage-origin, or foreign-contract facts.

§ 38(5) Separate-compilation generic summaries must be validated before serving as positive proof.

---

## § 39 Interfaces and dynamic dispatch

§ 39(1) An interface value or dynamically dispatched callable does not become transferable merely because the interface declaration is transferable in the abstract.

§ 39(2) The concrete value and implementation contract must satisfy the requested boundary.

§ 39(3) An interface may define a boundary capability requirement only when the interface rulebook provides a canonical way to express that requirement.

§ 39(4) Unknown dynamic target/value capability is conservative.

§ 39(5) Existential/boxed representation must not hide non-transferable concrete state.

---

## § 40 Compiler analysis

§ 40(1) Transferability analysis must identify the exact boundary operation.

§ 40(2) Analysis must consume canonical facts for:

```text
copy/move mode
source and destination owner
borrow live ranges
lifetime/escape
capture mode
recursive value graph
allocator/storage domain
thread/task affinity
active guards/iterators/views
address stability
destruction context
detached execution
process adapter/representation
ISR restrictions
foreign contracts
target/runtime policy
```

§ 40(3) Whole-program analysis should derive capabilities automatically where possible.

§ 40(4) Separate compilation may use validated summaries.

§ 40(5) Missing positive evidence must be conservative when the boundary requires proof.

§ 40(6) Transferability analysis must not duplicate ownership, borrow, lifetime, data-race, or ISR engines; it consumes their canonical facts.

---

## § 41 Boundary-specific summaries

§ 41(1) Compiler summaries may encode transferability as reusable boundary predicates.

§ 41(2) Summaries must identify the boundary kind and any conditions under which the proof holds.

§ 41(3) Conditions may include:

```text
destination thread class
scheduler migration policy
required synchronized wrapper
allocator/storage domain
destruction affinity
foreign runtime contract
process adapter
ISR context
```

§ 41(4) A summary valid for one target/profile must not be reused blindly for an incompatible target/profile.

§ 41(5) Imported summaries must be versioned and dependency-validated.

---

## § 42 Semantic IR

§ 42(1) Semantic IR must represent execution-boundary transfers explicitly where the transfer remains semantically relevant after Sema.

§ 42(2) Canonical operation categories may include:

```text
TransferOwnedToTask
TransferOwnedToThread
TransferBorrowToTask
TransferBorrowToThread
TransferObserver
TransferToProcessAdapter
TransferFromISR
TransferToISR
TransferToForeignCallbackContext
```

§ 42(3) Exact internal operation names are implementation details, but the represented semantic distinctions are normative.

§ 42(4) Each transfer operation must preserve, where relevant:

```text
boundary kind
source owner
destination owner
copied or moved state
retained borrow relationship
transfer capability proof
storage/affinity restrictions
destruction-context restriction
process/ISR/foreign adapter facts
source location
```

§ 42(5) Semantic IR must not collapse ownership transfer into concurrent sharing.

§ 42(6) Semantic IR verification must reject contradictory transfer facts.

§ 42(7) Transfer facts may be erased only after an equivalent lower-level ownership/lifetime/synchronization/execution contract has been established.

---

## § 43 Lowering

§ 43(1) Lowering must preserve transfer ownership mode and execution-boundary semantics.

§ 43(2) Backend/runtime helpers must not silently copy a move-only resource to implement a transfer.

§ 43(3) Backend/runtime helpers must not heap-promote borrowed/local state merely to repair an illegal transfer.

§ 43(4) Task/thread/channel lowering must preserve commit/failure ownership behavior.

§ 43(5) Process transfer lowering must use the selected serialization/IPC/shared-memory/handle-transfer contract.

§ 43(6) ISR handoff lowering must preserve bounded/noalloc/noblock/nopanic and synchronization requirements.

§ 43(7) Optimization must not introduce cross-thread sharing for a value proven safe only for exclusive transfer.

§ 43(8) Optimization must not remove synchronization required by `ThreadSafeShared` proof.

---

## § 44 Diagnostics

§ 44(1) Transferability diagnostics must identify the concrete boundary.

§ 44(2) Diagnostics should explain:

```text
which value/capability cannot cross
which boundary is involved
which contained field/capture/reference causes the failure
which affinity/lifetime/borrow/destruction/foreign/ISR rule is violated
where the incompatible dependency originated
a practical alternative when one is known
```

§ 44(3) Diagnostics should avoid reducing all failures to a vague "type is not transferable".

Example:

```text
error: `socket` cannot move to thread `worker`

`socket` contains a foreign handle that must be used and destroyed on the
thread that created it.

help: keep the socket on its owning thread and send data/messages to that thread
```

Example:

```text
error: detached task cannot retain reference to local `data`

the task may continue after `data` is destroyed at the end of this scope.

help: move an owned value into the task or use storage whose lifetime covers the task
```

Example:

```text
error: `MutexGuard[State]` cannot cross this task boundary

the guard is scoped to the current execution context and must be released before spawning
```

§ 44(4) Diagnostics must use stable IDs where the diagnostics registry defines them.

---

## § 45 LSP and tooling

§ 45(1) LSP and `sec analyse` must consume the same transfer facts as compilation.

§ 45(2) Tooling may expose:

```text
boundary-specific transferability
shareability
thread/task affinity
destruction affinity
blocking active borrow/guard
process adapter requirement
ISR restriction
foreign transfer contract
```

§ 45(3) Hover should not display an unconditional `ThreadTransferable` label when the actual proof is conditional on value-specific facts.

§ 45(4) Call hierarchy/spawn/channel views may identify transfer boundaries.

§ 45(5) Tooling should navigate to the contained field/capture/foreign contract causing a transfer failure.

§ 45(6) Incremental analysis must invalidate transfer summaries when relevant type, ownership, lifetime, concurrency, target, FFI, or platform dependencies change.

---

## § 46 No runtime transfer checker

§ 46(1) Sec does not require a generic runtime transferability checker.

§ 46(2) Safe transferability violations known at compile time are compile-time errors.

§ 46(3) The compiler must not attach hidden runtime "transferable" flags to ordinary values merely to implement this rulebook.

§ 46(4) API-specific runtime failure remains permitted for operations whose external execution may fail, such as IPC send, process startup, queue capacity, or platform handle duplication.

§ 46(5) Such API failure is not a runtime correction of a statically invalid Sec transfer.

---

## § 47 Required test families

### § 47.1 Owned transfer

§ 47.1(1) Required tests include:

```text
copyable owned value copied into valid task
move-only owned value explicitly moved into valid task
source unavailable after consuming transfer
thread-transferable owner moved to thread
non-thread-transferable owner rejected
transfer validation occurs before ownership commit
```

### § 47.2 Borrowed transfer

§ 47.2(1) Required tests include:

```text
shared borrow crosses task only while referent lives
mutable borrow grants exclusive destination access
conflicting source access rejected during transferred borrow
detached borrow to local rejected
thread-bound reference rejected on migratable task
borrow accepted after incompatible earlier borrow ends
```

### § 47.3 Thread/task affinity

§ 47.3(1) Required tests include:

```text
thread-local reference cannot move threads
thread-local reference cannot cross migration-capable await
pinned task may retain permitted thread-local dependency
migratable task cannot assume pinning
destination-thread destruction restriction enforced
```

### § 47.4 Composite values

§ 47.4(1) Required tests include:

```text
struct transfer derived recursively
Option transfer derived from payload
Result transfer derived from both payload families
active union variant path refinement
collection storage backend can block transfer
active iterator/guard can temporarily block owner transfer
```

### § 47.5 Closures

§ 47.5(1) Required tests include:

```text
owned-copy capture transfer
owned-move capture transfer
shared-borrow capture transfer
mutable-borrow capture transfer
thread-bound capture rejects thread transfer
opaque callable cannot hide non-transferable capture
```

### § 47.6 Raw/FFI

§ 47.6(1) Required tests include:

```text
RawPtr value copy does not prove pointee transferability
safe raw-pointer cross-thread dereference requires trusted contract
foreign thread-affine handle rejected
foreign transferable-by-owner but non-shareable handle accepted for move only
callback-thread-bound capture enforced
unknown foreign handle conservative
```

### § 47.7 Process transfer

§ 47.7(1) Required tests include:

```text
ordinary ref cannot cross process
ordinary RawPtr not valid destination-process pointer
serializable value crosses through explicit process adapter
native handle follows duplication/transfer contract
ordinary in-process Channel is not IPC
shared-memory representation requires process-aware reference/synchronization
```

### § 47.8 ISR transfer

§ 47.8(1) Required tests include:

```text
ISR-safe atomic handoff accepted
fixed-capacity ISR-safe queue handoff accepted
ordinary mutex guard rejected
unbounded growing collection rejected
ISR handoff obeys noAlloc/noBlock/noPanic
volatile access alone does not establish synchronization
preallocated ownership handoff commits exactly one owner
```

### § 47.9 IR/lowering/tooling

§ 47.9(1) Required tests include:

```text
Semantic IR preserves boundary kind
owned transfer distinct from borrowed transfer
transfer distinct from concurrent sharing
lowering does not copy move-only resource
lowering does not hidden-allocate to repair illegal transfer
compiler/LSP agree on transfer failure
diagnostic identifies nested blocking field/capture
```

---

## § 48 Completion criteria

§ 48(1) Frontend transferability is complete when every Sec 0.1 execution-boundary operation validates ownership, borrow, lifetime, recursive value graph, affinity, destruction, FFI, process, ISR, and target constraints through canonical facts.

§ 48(2) Task/thread support is complete when task migration/pinning, detached execution, lifecycle handles, observers, channels, thread-local values, guards, and destination destruction are modeled without conflating transfer with sharing.

§ 48(3) Process support is complete when explicit IPC/serialization/shared-memory/handle adapters define process-transfer contracts and ordinary in-process references/pointers are rejected across process boundaries.

§ 48(4) ISR support is complete when transfer into/from interrupt execution consumes canonical interrupt, ownership, lifetime, synchronization, noalloc/noblock/nopanic, and bounded-work facts.

§ 48(5) FFI support is complete when foreign handle/callback thread affinity, retention, ownership, concurrent-use, and destruction contracts participate in transfer proof.

§ 48(6) Semantic IR support is complete when boundary transfer operations preserve all facts needed by lowering and verification without hiding ownership/borrow semantics.

§ 48(7) Lowering support is complete when maintained backends/runtime helpers preserve transfer commit, ownership, synchronization, affinity, process, and ISR semantics without hidden copy/allocation.

§ 48(8) Tooling support is complete when compiler, LSP, `sec analyse`, diagnostics, call hierarchy, and incremental summaries use the same canonical transfer facts.

§ 48(9) Transferability must not be marked complete merely because one spawn/channel path recursively checks field types.

---

## § 49 Core summary

§ 49(1) Transferability answers: may this specific value/capability cross this specific execution boundary under the active execution policy?

§ 49(2) Transferability is not one universal type trait.

§ 49(3) Internal compiler capabilities such as `TaskTransferable`, `ThreadTransferable`, `ProcessTransferable`, `ISRTransferable`, and `ThreadSafeShared` are derived semantic facts, not required user-written interfaces.

§ 49(4) Exclusive transfer and concurrent sharing are separate.

§ 49(5) Composite transferability is derived recursively over the reachable semantic value graph.

§ 49(6) Borrowed transfer additionally requires lifetime, alias, migration, and storage-validity proof.

§ 49(7) Task migration introduces physical-thread restrictions for thread-bound state.

§ 49(8) Ordinary references and raw pointers do not directly cross process boundaries as valid destination-process references.

§ 49(9) ISR transfer is operation- and direction-sensitive and must satisfy interrupt execution restrictions.

§ 49(10) `unsafe` does not waive transferability, lifetime, synchronization, affinity, or data-race requirements.

§ 49(11) The compiler derives transferability from canonical ownership, borrow, lifetime, concurrency, FFI, platform, and target facts rather than duplicating those analyses.

§ 49(12) Safe transferability failures are compile-time errors; Sec requires no generic runtime transfer checker.
