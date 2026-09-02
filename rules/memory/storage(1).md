# Storage

- Status: Normative
- Created: 2026-09-02
- Last updated: 2026-09-02
- Document revision: 2.0
- Sec language version: 0.1
- Canonical path: `rules/memory/storage.md`
- Replaces: previous revision of `rules/memory/storage.md`
- Repository baseline reviewed: `814a584` (latest publicly verifiable `main`; current `main` contents reviewed 2026-09-02)

---

## § 1 Purpose and authority

§ 1(1) This rulebook defines the canonical storage model of Sec 0.1.

§ 1(2) Storage is the capacity in which a materialized representation may exist.

§ 1(3) Storage is distinct from a value, object, binding, Place, allocation, resource, ownership, reference, and numeric address.

§ 1(4) This rulebook owns the canonical meanings of:

```text
StorageOrigin
BackingRelation
ReclamationAuthority
AddressStability
MemorySpaceKind
RegionDependencies
InvalidationDomain
ValidityEpoch
```

§ 1(5) These properties are orthogonal and must not be collapsed into one storage-class enum.

§ 1(6) `memory_model.md` owns the common abstract memory machine and Place/state model.

§ 1(7) `allocation.md` owns allocation-capable operations, allocation contexts, failure, and no-hidden-allocation rules.

§ 1(8) `arena.md` owns the programmer-visible Arena API and Arena-specific reset/release/capacity semantics.

§ 1(9) `ownership.md`, `copy_move.md`, `borrowing.md`, `lifetime_analysis.md`, `references.md`, `raw_pointers.md`, `destruction.md`, and `transferability.md` own their specialized memory semantics.

§ 1(10) `layout.md` owns materialized representation size, alignment, field placement, stride, padding, and representation validity.

§ 1(11) Platform rulebooks own fixed-address bindings, hardware mappings, MMIO, volatile access, FFI storage contracts, target memory spaces, and target capabilities.

§ 1(12) Concurrency rulebooks own visibility, ordering, synchronization, data races, and concurrent execution semantics.

§ 1(13) This rulebook introduces no new source keyword, source region syntax, lifetime parameter syntax, generic pin keyword, or general memory-space annotation.

---

## § 2 Core principles

§ 2(1) Value ownership is not backing-storage ownership.

§ 2(2) Value ownership is not reclamation authority.

§ 2(3) Object lifetime is not storage lifetime.

§ 2(4) Destruction is not reclamation.

§ 2(5) Move is not physical relocation.

§ 2(6) Scope lifetime is not storage lifetime.

§ 2(7) Numeric address is not storage identity.

§ 2(8) Address stability is not lifetime.

§ 2(9) Generation validation is not concurrent reclamation safety.

§ 2(10) A value may own initialized objects without owning the physical bytes that contain them.

§ 2(11) Destroying an object ends its object lifetime but need not reclaim its storage.

§ 2(12) Moving a value transfers semantic ownership and need not move bytes or change address.

§ 2(13) Safe Sec code must never infer storage validity or identity from numeric address alone.

§ 2(14) A target profile may restrict available storage mechanisms but must not silently change ownership, object lifetime, storage lifetime, initialization, invalidation, address stability, generation behavior, destruction, reclamation, or observable access behavior.

---

## § 3 Terminology

### § 3.1 Value

§ 3.1(1) A value is a typed semantic entity.

§ 3.1(2) A value may exist without materialized storage, for example as an SSA value.

### § 3.2 Object

§ 3.2(1) An object is a materialized value of type `T` whose object lifetime has begun.

§ 3.2(2) An object exists only while its representation is valid for `T` and its object lifetime remains active.

### § 3.3 Storage

§ 3.3(1) Storage is capacity in which representations may exist.

§ 3.3(2) Storage may exist without containing a live object.

### § 3.4 Binding and Place

§ 3.4(1) A binding associates a source name with a value, Place, reference, or compiler entity.

§ 3.4(2) A Place is a source-level location expression identifying a value-bearing location or operation target.

§ 3.4(3) Binding, Place, object, and storage identity must not be conflated.

### § 3.5 Backing storage

§ 3.5(1) Backing storage is storage used by a value for elements, payloads, entries, nodes, or another internal representation separate from its ordinary descriptor state.

### § 3.6 Storage domain

§ 3.6(1) A storage domain is a compiler-visible identity describing one storage lifetime and its governing storage properties.

§ 3.6(2) A storage-domain identity is independent of current numeric address.

### § 3.7 Allocation identity

§ 3.7(1) Allocation identity identifies one logical allocation.

§ 3.7(2) Allocation identity is distinct from allocator/provider identity, storage-domain identity, and address.

### § 3.8 Region

§ 3.8(1) A region is a compiler-internal identity used for lifetime and storage-dependency analysis.

§ 3.8(2) A region is not source syntax, source type, lexical scope, storage origin, or necessarily an invalidation domain.

### § 3.9 Invalidation domain

§ 3.9(1) An invalidation domain is the smallest storage domain whose prior dependencies become invalid through one semantic event.

§ 3.9(2) Examples include an allocation, Arena epoch, collection backing store, reusable slot, mapping, or owner control block.

### § 3.10 Validity epoch

§ 3.10(1) A validity epoch identifies one live incarnation of an invalidation domain.

§ 3.10(2) A generation is a common runtime representation of an epoch.

§ 3.10(3) An epoch belongs to an invalidation domain, not merely to an address.

### § 3.11 Reclamation

§ 3.11(1) Reclamation makes storage unavailable for its previous purpose and may release, reset, unmap, return, or reuse it.

§ 3.11(2) Reclamation is distinct from destruction.

### § 3.12 Relocation

§ 3.12(1) Relocation changes the physical location used for a storage domain or backing store.

§ 3.12(2) Relocation is distinct from moving a value.

### § 3.13 Pinning

§ 3.13(1) Pinning is a temporary constraint preventing relocation and, where the contract requires it, reclamation.

§ 3.13(2) Pinning does not transfer ownership or imply permanent lifetime.

---

## § 4 Canonical storage classification

§ 4(1) Storage classification consists of independent compiler facts.

§ 4(2) At minimum the compiler must model:

```text
StorageOrigin
BackingRelation
ReclamationAuthority
AddressStability
MemorySpaceKind
RegionDependencies
InvalidationDomain
ValidityEpoch
```

§ 4(3) An implementation may use different internal type names only when the same semantic distinctions are preserved.

§ 4(4) A single `StorageClass` enum that conflates these dimensions is not a conforming canonical model.

---

## § 5 StorageOrigin

§ 5(1) `StorageOrigin` identifies the primary storage domain from which storage comes and the principal rule governing its lifetime.

§ 5(2) Canonical origins are:

```text
Automatic
Static
ThreadLocal
Arena
AllocatorBacked
Unknown
```

§ 5(3) Origins are mutually exclusive for one storage root.

§ 5(4) Embedded substorage inherits the origin of its containing storage root.

§ 5(5) The following are not origins:

```text
Inline
Stack
External
CallerProvided
CompilerTemporary
Mapped
Foreign
FixedAddress
Device
SharedMemory
Volatile
Atomic
Pinned
```

§ 5(6) Those concepts are represented by other properties or contracts.

---

## § 6 Automatic storage

§ 6(1) `Automatic` storage belongs to an activation or executing storage scope.

§ 6(2) It may represent materialized locals, parameters, temporaries, compiler-generated local storage, or caller-provided result storage associated with an active call.

§ 6(3) `Automatic` does not promise physical stack allocation.

§ 6(4) Automatic values may lower to SSA, registers, stack slots, caller-provided return storage, or eliminated storage.

§ 6(5) A value requiring a longer lifetime than its automatic domain permits is invalid.

§ 6(6) The compiler must not repair such an invalid lifetime through hidden dynamic allocation.

§ 6(7) Moving an object out of an automatic Place ends that object lifetime in the Place but does not end the complete automatic storage domain.

---

## § 7 Static storage

§ 7(1) `Static` storage belongs to a static storage domain.

§ 7(2) It may represent module-level static storage, function-local static storage, type-associated static storage, and target-provided static storage with an explicit static contract.

§ 7(3) Static duration does not imply immutability, thread safety, atomicity, synchronization, shared ownership, or permanent object lifetime.

§ 7(4) Replacing an object in static storage changes the object lifetime without normally changing the static storage lifetime or epoch.

§ 7(5) Sec 0.1 must not require hidden global constructors, hidden process-exit destructor registries, or implicit global resource ownership.

---

## § 8 ThreadLocal storage

§ 8(1) `ThreadLocal` storage has one separate storage-domain identity per physical thread.

§ 8(2) Thread-local storage ends when the corresponding physical thread ends.

§ 8(3) Task migration does not transfer physical thread-local storage identity.

§ 8(4) A reference to thread-local storage must not cross suspension/migration when same-thread resumption cannot be proven.

§ 8(5) Thread-local storage is not ordinary static storage because identity and termination are bound to a physical thread.

---

## § 9 Arena storage

§ 9(1) `Arena` storage belongs to a specific Arena allocation domain.

§ 9(2) Arena storage depends on Arena domain identity, validity epoch, backing storage, capacity policy, memory space, and Arena reclamation authority.

§ 9(3) Ordinary Arena allocation does not advance the Arena epoch.

§ 9(4) Arena reset advances the epoch while keeping the Arena domain live.

§ 9(5) Arena release ends the Arena domain.

§ 9(6) Destroying an individual Arena-backed object does not normally reclaim its individual bytes.

§ 9(7) An Arena may own or borrow physical backing storage without changing the `Arena` origin of allocations produced from it.

---

## § 10 AllocatorBacked storage

§ 10(1) `AllocatorBacked` storage is established through an individual allocation governed by an allocator or compatible provider.

§ 10(2) The storage domain must identify enough information to preserve allocation identity, provider identity, matching reclamation operation, extent, alignment, memory space, address stability, validity epoch where required, and reclamation responsibility.

§ 10(3) `AllocatorBacked` does not imply one universal heap.

§ 10(4) Providers may include platform allocators, operating-system services, fixed-block pools, target allocators, verified runtime helpers, or another explicit provider.

§ 10(5) Successful allocation establishes a storage domain.

§ 10(6) Individual deallocation ends that storage domain.

§ 10(7) Reuse of the same numeric address must not revive safe dependencies to the previous allocation identity.

---

## § 11 Unknown storage origin

§ 11(1) `Unknown` means verified storage information is insufficient.

§ 11(2) `Unknown` never provides a positive guarantee.

§ 11(3) It must not be interpreted as automatic, static, allocator-backed, owned, valid forever, safe to return, safe to retain, safe to share, safe to reclaim, or safe to access concurrently.

§ 11(4) When required safety cannot be established through proof or a verified runtime protocol, safe code is invalid.

---

## § 12 BackingRelation

§ 12(1) `BackingRelation` describes how a value relates to separate backing storage.

§ 12(2) Canonical values are:

```text
None
Embedded
Owned
Borrowed
```

### § 12.1 None

§ 12.1(1) `None` means the value has no separate backing storage.

### § 12.2 Embedded

§ 12.2(1) `Embedded` means backing storage is part of an enclosing representation.

§ 12.2(2) Embedded storage inherits origin and storage lifetime from the enclosing storage root.

§ 12.2(3) Embedded storage cannot be rebound independently.

§ 12.2(4) Individual contained objects may have distinct object lifetimes.

### § 12.3 Owned

§ 12.3(1) `Owned` means the value has exclusive semantic control over separate backing storage.

§ 12.3(2) `Owned` does not necessarily mean individual byte-level reclamation authority.

### § 12.4 Borrowed

§ 12.4(1) `Borrowed` means backing storage is supplied and controlled by another owner/domain.

§ 12.4(2) The borrower must not deallocate, reset, replace, relocate, rebind, or extend the backing lifetime unless the canonical contract explicitly grants such authority.

§ 12.4(3) Exclusive access does not by itself imply backing-storage ownership.

§ 12.4(4) The backing owner must not invalidate storage while a valid dependency requires it.

---

## § 13 ReclamationAuthority

§ 13(1) `ReclamationAuthority` identifies which object or domain may release, reset, or reuse storage.

§ 13(2) Canonical values are:

```text
EnclosingObject
StaticDomain
ThreadDomain
Arena
OwningValue
ExternalOwner
Platform
None
Unknown
```

§ 13(3) `EnclosingObject` means embedded storage follows the enclosing storage.

§ 13(4) `StaticDomain` means a static domain governs storage.

§ 13(5) `ThreadDomain` means one physical thread's thread-local domain governs storage.

§ 13(6) `Arena` means the Arena controls reset/release.

§ 13(7) `OwningValue` means the owning value or deterministic destruction governs individual reclamation.

§ 13(8) `ExternalOwner` means another Sec value, caller, foreign owner, runtime object, or explicit provider governs reclamation.

§ 13(9) `Platform` means loader, OS, hardware, linker, device, or target governs storage.

§ 13(10) `None` means no reclamation occurs during the relevant program lifetime.

§ 13(11) `Unknown` grants no safe reclamation or retention assumption.

---

## § 14 AddressStability

§ 14(1) `AddressStability` describes whether the physical address may change.

§ 14(2) Canonical values are:

```text
Movable
Stable
Fixed
Unknown
```

§ 14(3) `Movable` permits relocation when governing type/operation rules permit it.

§ 14(4) `Stable` means the address remains unchanged during the current guaranteed stability interval.

§ 14(5) `Fixed` means address/location is part of the storage contract, as for MMIO, linker regions, vectors, control blocks, or device windows.

§ 14(6) `Unknown` provides no stable-address guarantee.

§ 14(7) `Stable` does not mean permanent lifetime.

§ 14(8) `Fixed` does not mean the compiler owns or may reclaim storage.

---

## § 15 Orthogonal properties

§ 15(1) Mutability, volatility, atomicity, sharing, thread safety, pinning, mapping, foreign provenance, fixed-address placement, and DMA accessibility are orthogonal properties/contracts.

§ 15(2) The compiler must not infer any of these merely from `StorageOrigin`.

§ 15(3) One storage root may combine, for example:

```text
origin: AllocatorBacked
backing relation: Owned
address stability: Stable
mapped: true
foreign: true
memory space: TargetDefined(ForeignSharedMemory)
```

---

## § 16 MemorySpaceKind

§ 16(1) A memory space identifies storage with distinct access/transfer rules.

§ 16(2) Common kinds are:

```text
Ordinary
MMIO
TargetDefined
```

### § 16.1 Ordinary

§ 16.1(1) `Ordinary` supports ordinary Sec loads, stores, references, copy, move, and borrowing subject to normal rules.

### § 16.2 MMIO

§ 16.2(1) `MMIO` requires target-recognized hardware access semantics.

§ 16.2(2) Its contract may require exact widths, volatile operations, ordering constraints, barriers, read/write side effects, target instructions, and restrictions on ordinary references.

§ 16.2(3) MMIO is distinct from `AddressStability.Fixed` and from volatility.

### § 16.3 TargetDefined

§ 16.3(1) Targets may define accelerator-local, secure, non-secure, non-volatile, DSP-local, GPU, foreign, or other memory spaces.

§ 16.3(2) Each target-defined space must provide compiler-known identity, allowed access operations, addressability, reference forms, widths, alignment, atomics, volatility, coherence, accessible execution entities, supported origins, transfer operations, and target availability.

§ 16.3(3) Cross-space transfers must be explicit and may be fallible.

---

## § 17 Object lifetime and storage lifetime

§ 17(1) Object lifetime begins when a valid object is constructed or initialized in storage.

§ 17(2) Storage may exist before object lifetime begins.

§ 17(3) Object lifetime may end while storage remains live.

§ 17(4) A later object may be constructed in the same storage under ordinary initialization/replacement rules.

§ 17(5) Safe references to the old object do not become references to the new object merely because the address is reused.

§ 17(6) Storage lifetime bounds every object lifetime materialized within that storage.

---

## § 18 Initialization and typed uninitialized storage

§ 18(1) Storage is not automatically initialized merely because it has a type-compatible size/alignment.

§ 18(2) Safe readable `T` requires a live valid initialized `T`.

§ 18(3) General typed uninitialized storage must be represented distinctly from ordinary readable `T`.

§ 18(4) Partial initialization must be tracked at the granularity required by the owning type and construction semantics.

§ 18(5) Destruction may run only for subobjects whose object lifetime actually began.

§ 18(6) Reclamation of raw storage does not replace required destruction of initialized objects.

---

## § 19 Replacement and reinitialization

§ 19(1) Replacement of an available mutable Place ends the old object lifetime after required destruction and begins a new object lifetime.

§ 19(2) Reinitialization of an unavailable mutable Place begins a new object lifetime without destroying an absent old object.

§ 19(3) Replacement and reinitialization are distinct semantic operations even when they lower similarly.

§ 19(4) Conditionally available storage follows ownership-v2 path-sensitive replacement rules.

§ 19(5) Storage identity need not change merely because object lifetime changes.

---

## § 20 Invalidation domains and epochs

§ 20(1) A semantic invalidation event acts on a canonical invalidation domain.

§ 20(2) Events may include Arena reset, allocation deallocation, reusable-slot reuse, collection backing replacement, mapping remap/unmap, or owner-control-block invalidation.

§ 20(3) An invalidation event may advance an epoch or end the domain.

§ 20(4) A later live incarnation must be distinguishable from the invalidated incarnation.

§ 20(5) Runtime generations are one implementation mechanism, not mandatory syntax or representation.

§ 20(6) Static proof may eliminate runtime epoch/generation metadata and checks.

---

## § 21 Establish, advance, and end domain

§ 21(1) The canonical storage state model includes operations equivalent to:

```text
EstablishStorageDomain
AdvanceStorageEpoch
EndStorageDomain
```

§ 21(2) Establishing a domain creates a new storage identity and initial live epoch.

§ 21(3) Advancing an epoch invalidates dependencies tied to the previous epoch while preserving the broader domain where the owning contract says so.

§ 21(4) Ending a domain invalidates all dependencies requiring that domain.

§ 21(5) Backends may lower these concepts without runtime operations when static proof suffices.

---

## § 22 Backing storage binding

§ 22(1) Values with separate backing storage must preserve an explicit semantic relation to that backing domain.

§ 22(2) Binding backing storage must establish extent, lifetime, memory space, address-stability, ownership/reclamation relation, and relevant invalidation dependency.

§ 22(3) Rebinding backing storage is a semantic operation and must not silently occur during ordinary copy/move.

§ 22(4) Rebinding must invalidate or update dependent views/references according to canonical reference rules.

§ 22(5) Borrowed backing cannot be rebound by the borrower unless the backing contract explicitly permits it.

---

## § 23 Relocation

§ 23(1) Physical relocation is a storage operation, not an ownership move.

§ 23(2) Relocation is valid only when every live dependency remains valid or is canonically invalidated/updated.

§ 23(3) Direct references requiring stable addresses can forbid relocation for their live ranges.

§ 23(4) Stable-handle/reference-model mechanisms may preserve identity across relocation when explicitly supported.

§ 23(5) Backend optimizations may relocate storage only within the semantic address-stability and dependency contract.

---

## § 24 Pinning

§ 24(1) Pinning prevents relocation for a defined interval.

§ 24(2) A pin may additionally prevent reclamation only when its contract says so.

§ 24(3) Pinning does not imply ownership, permanent lifetime, thread safety, or synchronization.

§ 24(4) Sec 0.1 defines no general source-level `pin` keyword through this rulebook.

§ 24(5) Specialized APIs may expose pin guards or equivalent compiler-known contracts.

§ 24(6) Pin acquisition/release must participate in borrow/lifetime/effect analysis where required.

---

## § 25 Reclamation protection

§ 25(1) Concurrently reclaimable storage may require an explicit reclamation-protection protocol.

§ 25(2) Generation checks alone do not make racing reclamation safe.

§ 25(3) Protection may use locks, epochs, hazards, reference counts, ownership transfer, platform facilities, or another canonical type-specific mechanism.

§ 25(4) The compiler need not impose one universal concurrent reclamation protocol.

§ 25(5) The protection mechanism must cover the entire access interval that depends on the storage remaining unreclaimed.

---

## § 26 Capacity and growth

§ 26(1) Capacity is a storage property of backing domains used by collections, arenas, buffers, and similar abstractions.

§ 26(2) Growth may preserve or replace backing storage depending on the canonical type operation.

§ 26(3) Growth that replaces backing storage may invalidate references/views/iterators tied to the previous invalidation domain.

§ 26(4) Growth must not be silently introduced by operations defined as allocation-free.

§ 26(5) Fixed-capacity storage must reject or return a defined failure when capacity cannot satisfy an operation.

---

## § 27 Mapped storage

§ 27(1) Runtime mapping is a storage/resource contract, not a `StorageOrigin`.

§ 27(2) A mapping owner controls mapping lifetime according to the platform mapping rulebook.

§ 27(3) Typed views into a mapping are non-owning dependencies bounded by mapping lifetime/epoch.

§ 27(4) Remapping may preserve the owner while invalidating prior views.

§ 27(5) Device liveness is separate from mapping lifetime, storage identity, and Place availability.

§ 27(6) A raw pointer into a mapping does not keep the mapping alive.

---

## § 28 Foreign storage

§ 28(1) Foreign provenance is a contract orthogonal to `StorageOrigin`.

§ 28(2) Foreign storage contracts must specify lifetime, ownership/reclamation responsibility, retention, mutability, extent, alignment, address space, and concurrency where relevant.

§ 28(3) Foreign storage may be Automatic, Static, AllocatorBacked, or otherwise represented depending on the verified contract.

§ 28(4) Unknown foreign lifetime/ownership grants no positive safe guarantee.

§ 28(5) A foreign pointer alone does not establish safe Sec storage provenance.

---

## § 29 Fixed-address and hardware storage

§ 29(1) Fixed-address placement is represented through `AddressStability.Fixed` plus platform contracts, not as a `StorageOrigin`.

§ 29(2) MMIO storage uses `MemorySpaceKind.MMIO` or a canonical target-defined equivalent.

§ 29(3) `@address`/platform bindings must consume target knowledge and storage lifetime/availability contracts.

§ 29(4) Fixed numeric address does not imply permanent object lifetime or compiler ownership.

§ 29(5) Active-high/active-low signal meaning remains outside compiler storage semantics.

§ 29(6) Scope exit/destruction of a binding must not invent hardware reset, register clearing, or physical reclamation.

---

## § 30 Volatile and atomic storage

§ 30(1) Volatility and atomicity are independent of storage origin and address stability.

§ 30(2) Volatile access preserves required observable storage accesses.

§ 30(3) Volatile is not synchronization.

§ 30(4) Atomicity is defined by concurrency/atomic rulebooks and target support.

§ 30(5) Storage classification must not encode `volatile` or `atomic` as origins.

---

## § 31 DMA and external agents

§ 31(1) DMA accessibility is a platform/storage contract, not a storage origin.

§ 31(2) External agents may mutate storage without ordinary Sec execution.

§ 31(3) Safe access requires the relevant ownership, synchronization, cache/coherence, mapping, and device contracts.

§ 31(4) A register write that starts DMA does not automatically transfer Sec ownership unless the DMA contract explicitly models ownership handoff.

---

## § 32 Transferability and storage

§ 32(1) Storage origin and memory space may constrain task/thread/process/ISR transferability.

§ 32(2) Moving a value does not extend its backing storage lifetime.

§ 32(3) Thread-local or task-local storage may make otherwise transferable values non-transferable.

§ 32(4) Cross-process transfer requires explicit adapter/shared-memory semantics; ordinary storage identity does not cross a process boundary.

§ 32(5) ISR transfer requires storage accessible and valid in interrupt execution plus ordinary interrupt restrictions.

---

## § 33 Semantic IR: stable facts

§ 33(1) Semantic IR must preserve stable storage facts required after Sema.

§ 33(2) Relevant facts may include:

```text
StorageOrigin
BackingRelation
ReclamationAuthority
AddressStability
MemorySpaceKind
storage-domain identity
allocation identity
invalidation-domain identity
region dependencies
target/platform storage contract
foreign/mapping contract
```

§ 33(3) Stable facts must not be recomputed independently by lowerings from lexical names or backend pointer types.

---

## § 34 Semantic IR: path-dependent state

§ 34(1) Path-dependent storage state may include:

```text
current validity epoch/generation
object lifetime active/inactive
initialization state
pin state
reclamation-protection state
mapping live/remapped/ended
backing relation after rebind
```

§ 34(2) Path-dependent facts must join conservatively across control flow.

§ 34(3) Static proof may eliminate state that no longer affects observable or safety semantics.

---

## § 35 Semantic IR: explicit operations

§ 35(1) Semantic IR must be able to represent operations equivalent to:

```text
EstablishStorageDomain
AdvanceStorageEpoch
EndStorageDomain
ConstructObject
EndObjectLifetime
BindBackingStorage
RebindBackingStorage
RelocateBackingStorage
AcquirePin
ReleasePin
AcquireReclamationProtection
ReleaseReclamationProtection
ValidateGeneration
```

§ 35(2) Exact internal node names are implementation details.

§ 35(3) Allocation, initialization, destruction, reclamation, relocation, and epoch invalidation must remain distinguishable until equivalent lower-level semantics exist.

---

## § 36 Lowering

§ 36(1) Lowering consumes Sema/Semantic-IR storage semantics and must not invent new source-level storage meaning.

§ 36(2) Runtime metadata may be object metadata, side tables, slot metadata, owner control blocks, atomics, tagged/capability addresses, target hardware support, or eliminated after proof.

§ 36(3) The model does not require one universal runtime storage header.

§ 36(4) Lowering must preserve required generation checks, pin/protection semantics, address spaces, volatility, mapping lifetime, and reclamation ordering.

§ 36(5) Backend pointer equality or address reuse must not substitute for canonical storage identity.

§ 36(6) Optimizations may remove storage operations/checks only after equivalent proof.

---

## § 37 Diagnostics

§ 37(1) Storage diagnostics must follow the mentor-compiler principle.

§ 37(2) Diagnostics should identify the affected value/Place, storage origin/domain, invalidating/reclamation operation, relevant dependency, and a practical remedy.

§ 37(3) Diagnostics should distinguish object lifetime end from storage reclamation.

§ 37(4) Diagnostics should distinguish address instability from lifetime failure.

§ 37(5) Unknown storage facts must be described as insufficient proof, not as guaranteed invalidity unless invalidity is proven.

Example intent:

```text
error: this view may refer to the previous backing storage

`values.Grow()` can replace the collection backing store, so the earlier view
cannot be used after the growth operation.

help: create the view after the collection has finished growing
```

---

## § 38 LSP and tooling

§ 38(1) LSP and `sec analyse` must consume the same canonical storage facts as compilation.

§ 38(2) Tooling may expose origin, backing relation, reclamation authority, address stability, memory space, epoch dependency, mapping/foreign contract, and invalidation cause.

§ 38(3) Tooling must not expose legacy storage-origin categories as normative when the canonical v2 model has replaced them.

§ 38(4) Incremental analysis must invalidate storage facts when declaration, allocation, target, mapping, FFI, layout, or control-flow dependencies change.

---

## § 39 Required test families

§ 39(1) Required classification tests include every canonical `StorageOrigin`, `BackingRelation`, `ReclamationAuthority`, `AddressStability`, and `MemorySpaceKind`.

§ 39(2) Required lifetime tests include object lifetime shorter than storage lifetime, replacement in stable storage, address reuse with new identity, Arena reset epoch advance, and domain end.

§ 39(3) Required backing tests include embedded, owned, borrowed, rebind, relocation, and invalidation of dependent references/views.

§ 39(4) Required initialization tests include typed uninitialized storage, partial initialization, destruction only of initialized subobjects, and reinitialization.

§ 39(5) Required platform tests include fixed-address storage, MMIO, mapping/remap, foreign storage, volatile access, and device liveness separation.

§ 39(6) Required concurrency tests include generation not replacing reclamation protection and storage identity shared with race/synchronization analysis.

§ 39(7) Required IR/lowering tests include explicit domain/epoch/object/backing/pin/protection operations and proof-based check elimination.

§ 39(8) Required tooling tests include compiler/LSP parity and mentor diagnostics.

---

## § 40 Completion criteria

§ 40(1) Frontend storage support is complete when canonical storage properties propagate through every relevant value, Place, reference, aggregate, allocation, mapping, and operation.

§ 40(2) Lifetime integration is complete when region dependencies and invalidation domains use one canonical identity model rather than root-name approximations.

§ 40(3) Reference integration is complete when epochs/generations, mapping dependencies, address stability, and storage identity feed safe-reference validity.

§ 40(4) Destruction/reclamation integration is complete when object cleanup and raw-storage reclamation remain distinct in analysis and lowering.

§ 40(5) Platform integration is complete when memory spaces, fixed-address, mapping, foreign, volatile, and hardware contracts consume the same storage model.

§ 40(6) Semantic IR/lowering support is complete when every required stable fact, path state, and semantic operation is explicit or equivalently proven.

§ 40(7) Tooling support is complete when compiler, LSP, diagnostics, and analyses use canonical storage facts.

---

## § 41 Core summary

§ 41(1) Storage is capacity for materialized representation; it is not the value, object, owner, reference, or address.

§ 41(2) Storage classification is multi-dimensional and must not be collapsed into one enum.

§ 41(3) Canonical origins are `Automatic`, `Static`, `ThreadLocal`, `Arena`, `AllocatorBacked`, and `Unknown`.

§ 41(4) Foreign, mapped, fixed-address, volatile, atomic, pinned, and shared-memory properties are orthogonal contracts, not storage origins.

§ 41(5) Object lifetime may begin/end many times within one live storage domain.

§ 41(6) Numeric address reuse never revives an old storage identity or safe reference.

§ 41(7) Generation/epoch is an invalidation mechanism, not a complete concurrent reclamation protocol.

§ 41(8) Move is ownership transfer, not physical relocation.

§ 41(9) Destruction ends object lifetime; reclamation ends/reuses storage.

§ 41(10) Sema establishes storage semantics, Semantic IR preserves/verifies them, and lowerings implement them without inventing new memory meaning.
