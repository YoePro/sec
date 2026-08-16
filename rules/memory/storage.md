# Storage

## Status

This document is the canonical storage rulebook for Sec 0.1.

It defines:

- storage classification;
- storage identity;
- backing-storage relations;
- reclamation authority;
- address stability;
- storage lifetime versus object lifetime;
- storage placement guarantees;
- relocation and replacement;
- invalidation domains and validity epochs;
- runtime generations for dynamically reclaimable storage;
- explicit backing storage;
- typed uninitialized storage;
- capacity and growth semantics;
- pinning constraints;
- memory spaces;
- mapped, foreign, and fixed-address storage contracts;
- concurrent reclamation requirements;
- Semantic IR requirements;
- lowering constraints;
- diagnostics;
- required tests.

This rulebook does not introduce new source syntax.

In particular, it does not introduce:

- a `storage` keyword;
- a `stack` keyword;
- a general `pin` keyword;
- source-level region declarations;
- source-level lifetime parameters;
- general memory-space annotations;
- general placement-construction syntax.

The internal names used by this rulebook describe required semantics.
Compiler data structures may use different implementation names only when the
same information and guarantees are preserved.

Adjacent rulebooks retain responsibility for their specialized areas:

```text
memory_model.md
    the common abstract memory machine

allocation.txt
    allocation operations, allocation contexts, providers, and failure

arena.md
    programmer-visible Arena behavior

ownership.md
    ownership of values

copy_move.md
    copy, move, and ownership transfer

borrowing.txt
    borrow compatibility and borrow liveness

reference_model.md
    references, handles, provenance, bounds, and generation-bearing access

lifetime_analysis.txt
    lifetime and escape proofs

destruction.txt
    deterministic destruction and cleanup

collections.md
    collection families and collection APIs

shaped-types.md
    shaped values, views, layouts, and memory spaces

platform/ffi.md
    foreign ABI and ownership contracts

declarations/registers.md
    register type and bit-layout semantics

platform/fixed-address-bindings.md
    @address, MMIO, and volatile fixed-address binding semantics

concurrency_memory_model.md
    synchronization, visibility, ordering, and data races

effect_analysis.md
    inferred and ordered effects

semantic_ir.txt
    the complete Semantic IR contract
```

This rulebook owns the canonical definitions of storage origin, backing
relation, reclamation authority, address stability, region dependencies,
invalidation domains, validity epochs, and storage-domain transitions.

---

# Current implementation status

The current compiler contains several foundations required by this rulebook.

## Implemented

The compiler currently implements:

- semantic type objects;
- symbol-level storage-origin metadata;
- reference storage-origin metadata;
- addressed-symbol metadata;
- volatile-symbol metadata;
- local-symbol metadata;
- an older storage-origin classification containing:
  - `Inline`;
  - `Static`;
  - `Arena`;
  - `External`;
  - `Foreign`;
  - `FixedAddress`;
  - `Unknown`;
- recognition of local symbols as inline storage under the older model;
- recognition of addressed register bindings as fixed-address storage under the
  older model;
- semantic recognition of `Arena`;
- Arena allocation-domain identity;
- Arena validity-generation metadata;
- `Arena.Alloc[T](count)` recognition;
- `Arena.Reset()` recognition;
- Arena generation advancement;
- rejection of selected uses of stale Arena references and slices;
- conservative generation merging through selected control-flow forms;
- initial move-state tracking;
- initial reference-origin and escape analysis;
- rejection of returned references into local function storage;
- propagation of selected reference origins through fields, indexes, slices,
  and aggregate values;
- prohibition of hidden escape promotion;
- prohibition of hidden allocation during ordinary copy, move, borrow,
  parameter passing, return, and escape analysis;
- initial concurrency rules for address stability, atomics, volatile access,
  publication, and synchronization.

## Partially implemented

The current implementation is partial in these areas:

- storage classification is not propagated consistently through every value,
  place, reference, aggregate, and Semantic IR operation;
- reference provenance is root-symbol based in several analyses;
- Arena generation analysis is conservative and incomplete;
- lifetime analysis does not yet use one complete region and dependency graph;
- place identity is not represented uniformly;
- field-sensitive and element-sensitive overlap analysis is incomplete;
- partial initialization is not comprehensively tracked;
- partial move state is incomplete;
- replacement and reinitialization are not represented consistently as distinct
  operations;
- value destruction is not lowered completely;
- allocation origin is selected semantically only in limited cases;
- address stability is not a first-class storage property throughout the
  compiler;
- memory-space classification is incomplete;
- foreign storage contracts are incomplete;
- concurrent reclamation protocols are type-specific rather than represented by
  one common storage model.

## Not implemented yet

The compiler has not yet completed:

- migration from the older storage-origin list to the canonical origin list in
  this rulebook;
- first-class `BackingRelation`;
- first-class `ReclamationAuthority`;
- first-class `AddressStability`;
- the complete compiler-internal region model;
- general invalidation-domain identities;
- the complete `EstablishDomain`, `AdvanceEpoch`, and `EndDomain` state model;
- general runtime generations for allocator-backed allocations;
- reusable-slot generations;
- general collection backing-storage epochs;
- general typed uninitialized storage;
- complete initialization masks for typed slots;
- explicit backing-storage binding and rebinding operations;
- general pin guards;
- general reclamation-protection guards;
- first-class memory-space contracts;
- mapped-storage contracts;
- complete foreign-storage contracts;
- complete fixed-address alias validation;
- all Semantic IR operations required by this rulebook;
- complete MLIR and LLVM lowering for these storage semantics;
- complete storage diagnostics and test suites.

The older compiler categories remain implementation facts only until migrated.
They do not override the canonical model defined below.

---

# Purpose

Storage is the capacity in which a materialized representation may exist.

Storage is not the same as:

- a value;
- an object;
- a binding;
- a place;
- an allocation;
- a resource;
- ownership;
- a reference;
- an address.

This rulebook ensures that Sec can describe storage consistently across:

- hosted applications;
- long-running servers;
- embedded systems;
- bare metal;
- operating-system code;
- arenas;
- explicit allocators;
- fixed-capacity collections;
- growable collections;
- reusable slot tables;
- FFI;
- memory mappings;
- MMIO;
- multiple memory spaces;
- multithreaded and asynchronous programs.

The same source-level storage semantics apply across targets.

A target profile may restrict available storage mechanisms.

It must not silently change:

- ownership;
- object lifetime;
- storage lifetime;
- initialization;
- invalidation;
- address stability;
- generation behavior;
- destruction;
- reclamation;
- observable access behavior.

---

# Core principles

The canonical storage principles are:

```text
value ownership != backing-storage relation

value ownership != reclamation authority

object lifetime != storage lifetime

destruction != reclamation

move != physical relocation

scope lifetime != storage lifetime

address != storage identity

address stability != lifetime

generation validation != concurrent reclamation safety
```

A value may own initialized objects without owning the bytes that contain those
objects.

A value may control backing storage without having authority to reclaim the
physical bytes individually.

Destroying an object ends its object lifetime.

It does not necessarily reclaim the storage in which the object existed.

Moving a value transfers semantic ownership.

It does not necessarily copy bytes, move bytes, change the storage address, or
change the storage domain.

Safe Sec code must never infer storage safety from a numeric address alone.

---

# Non-goals

This rulebook does not define:

- a mandatory heap;
- garbage collection;
- reference counting;
- one universal allocator;
- one universal concurrent reclamation protocol;
- implicit escape promotion;
- hidden dynamic allocation;
- a universal object header;
- a universal generation representation;
- a universal mapping API;
- a universal foreign-storage wrapper;
- a universal pinning API;
- a public region calculus;
- programmer-written lifetime annotations;
- automatic conversion of raw bytes into objects;
- implicit transfer between memory spaces.

---

# Terminology

## Value

A value is a typed semantic entity.

A value may exist without materialized storage, for example as an SSA value.

## Object

An object is a materialized value of type `T` whose object lifetime has begun.

An object exists only while its representation is valid for `T` and its object
lifetime remains active.

## Storage

Storage is capacity in which representations may exist.

Storage may exist without containing a live object.

## Binding

A binding associates a source name with a value, place, reference, or compiler
entity.

A binding is not the value or storage itself.

## Place

A place is a source-level location expression that may identify a value-bearing
location or operation target.

Examples:

```sec
value
person.Name
array[index]
```

## Backing storage

Backing storage is storage used by a value to contain elements, payloads,
entries, nodes, or another internal representation separate from the value's
ordinary descriptor state.

## Storage domain

A storage domain is a compiler-visible identity describing one storage lifetime
and its governing storage properties.

A storage domain is independent of its current numeric address.

## Storage identity

Storage identity distinguishes one storage domain or storage subdomain from
another.

Reusing the same address does not reuse the previous storage identity unless the
semantics explicitly preserve that identity through an epoch transition.

## Allocation identity

Allocation identity identifies one logical allocation.

It is distinct from allocator identity, storage-domain identity, and address.

## Region

A region is a compiler-internal identity used for lifetime and
storage-dependency analysis.

A region is not:

- source syntax;
- a source type;
- a lexical scope;
- a storage origin;
- necessarily an invalidation domain.

A place, value, view, reference, or handle may depend on more than one region.

## Invalidation domain

An invalidation domain is the smallest storage domain whose previous
dependencies become invalid through one semantic event.

Examples may include:

- one allocation;
- one Arena allocation epoch;
- one collection backing store;
- one reusable slot;
- one mapping;
- one owner control block.

## Validity epoch

A validity epoch identifies one live incarnation of an invalidation domain.

A generation is a common runtime representation of an epoch.

An epoch belongs to an invalidation domain, not merely to a numeric address.

## Reclamation

Reclamation makes storage unavailable for its previous purpose and may release,
reset, unmap, return, or reuse it.

Reclamation is distinct from destruction.

## Relocation

Relocation changes the physical location used for a storage domain or backing
store.

Relocation is distinct from moving a value.

## Pinning

Pinning is a temporary constraint that prevents relocation and, where required
by the pin contract, reclamation of a storage domain.

---

# Canonical storage classification

Storage classification consists of independent properties.

At minimum, the compiler must distinguish:

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

These properties must not be collapsed into one storage-class enum.

---

# Storage origin

`StorageOrigin` identifies the primary storage domain from which the storage
comes and the primary rule governing its lifetime.

The canonical storage origins are:

```text
Automatic
Static
ThreadLocal
Arena
AllocatorBacked
Unknown
```

The origins are mutually exclusive for one storage root.

Embedded substorage inherits the origin of its containing storage root.

The following are not storage origins:

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

Those concepts are represented by other properties or contracts.

---

# Automatic storage

`Automatic` storage belongs to an activation or executing storage scope.

It may be used for:

- materialized local values;
- materialized parameters;
- temporaries;
- compiler-generated local storage;
- caller-provided result storage associated with an active call.

`Automatic` does not promise physical stack allocation.

The compiler may represent automatic values through:

- SSA values;
- machine registers;
- stack slots;
- caller-provided return storage;
- eliminated storage.

A value that requires a longer storage lifetime than can be proven for its
automatic domain is invalid.

The compiler must not solve the problem through hidden dynamic allocation.

The automatic storage domain ends when its governing activation or storage
scope ends.

Moving an object out of an automatic place ends that object's lifetime in the
place but does not end the complete automatic storage domain.

---

# Static storage

`Static` storage belongs to a static storage domain.

It may represent:

- module-level static storage;
- function-local static storage;
- type-associated static storage;
- target-provided static storage described by an explicit static contract.

Static storage duration does not imply:

- immutability;
- thread safety;
- atomicity;
- synchronization;
- shared ownership;
- permanent object lifetime.

Replacing an object in static storage changes the object lifetime.

It does not normally change the static storage lifetime or storage epoch.

Sec 0.1 must not require hidden global constructors, hidden process-exit
destructor registries, or implicit global resource ownership.

The complete declaration and initialization rules remain defined by
`static.md`.

---

# Thread-local storage

`ThreadLocal` storage has one separate storage instance per physical thread.

Each physical thread receives a distinct storage-domain identity for the same
thread-local declaration.

Thread-local storage ends when the corresponding physical thread ends.

Task migration does not transfer the identity of physical thread-local storage.

A reference to thread-local storage must not cross suspension or migration when
the compiler cannot prove that it resumes on the same physical thread under a
valid thread-local contract.

Thread-local storage is not ordinary static storage because its identity and
termination are bound to a physical thread.

The complete source and concurrency rules remain defined by `thread_local.md`.

---

# Arena storage

`Arena` storage belongs to a specific Arena allocation domain.

Arena storage carries or depends on:

- Arena domain identity;
- current Arena validity epoch;
- backing-storage dependencies;
- capacity policy;
- memory-space identity;
- Arena reclamation authority.

Ordinary Arena allocation does not advance the Arena epoch.

Arena reset advances the Arena epoch while keeping the Arena domain live.

Arena release ends the Arena domain.

Destroying an individual Arena-backed object does not normally reclaim its
individual bytes.

An Arena may own or borrow physical backing storage without changing the
`Arena` origin of allocations produced from it.

The complete Arena API, allocation, capacity, reset, release, and nesting rules
remain defined by `arena.md`.

---

# Allocator-backed storage

`AllocatorBacked` storage is established through an individual allocation whose
lifetime is governed by an allocator or compatible provider.

An allocator-backed storage domain must preserve enough semantic information to
identify:

- allocation identity;
- allocator or provider identity;
- matching reclamation operation;
- size or extent;
- alignment;
- memory space;
- address stability;
- validity epoch or generation when required;
- ownership and reclamation responsibility.

`AllocatorBacked` does not imply one universal heap.

The provider may be:

- a platform allocator;
- an operating-system service;
- a fixed-block pool;
- a target allocator;
- a verified runtime helper;
- another explicit allocation provider.

Successful allocation establishes a storage domain.

Individual deallocation ends that storage domain.

A later allocation at the same numeric address must have a distinguishable
identity or epoch so that old safe handles cannot become valid again.

---

# Unknown storage origin

`Unknown` means the compiler lacks sufficient verified storage information.

`Unknown` never provides a positive safety guarantee.

It must not be interpreted as:

- automatic;
- static;
- allocator-backed;
- owned;
- valid forever;
- safe to return;
- safe to retain;
- safe to share;
- safe to reclaim;
- safe to access concurrently.

When required safety cannot be established through static proof or a verified
runtime protocol, safe code is invalid.

---

# Backing relation

`BackingRelation` describes how a value relates to backing storage.

The canonical values are:

```text
None
Embedded
Owned
Borrowed
```

## No separate backing storage

`None` means that the value has no separate backing storage.

A scalar value or a descriptor that owns no current allocation may have this
relation.

## Embedded

`Embedded` means that the backing storage is part of the representation of an
enclosing object.

Examples include:

- a field inside a struct;
- a fixed array `T[N]` inside an object;
- inline capacity inside a bounded collection representation.

Embedded storage:

- inherits the storage origin of the enclosing object;
- cannot be rebound independently;
- follows the enclosing storage lifetime;
- may have separate object lifetimes for individual contained objects;
- may relocate only when relocation of the enclosing object is permitted.

## Owned

`Owned` means that the value has exclusive semantic control over separate
backing storage.

`Owned` does not necessarily mean that the value may reclaim the physical bytes
individually.

For example, an Arena-backed collection may own its element objects and control
its allocation while `ReclamationAuthority` remains `Arena`.

## Borrowed

`Borrowed` means that backing storage is supplied and controlled by another
owner or domain.

The borrower must not:

- deallocate the backing storage;
- reset it;
- replace it;
- relocate it;
- rebind it implicitly;
- extend its lifetime;
- infer ownership from exclusive access alone.

The backing owner must not reclaim, replace, relocate, or reuse the relevant
storage while a valid borrow depends on it.

Borrowing backing storage does not by itself determine ownership of the objects
stored there.

---

# Reclamation authority

`ReclamationAuthority` identifies which object or domain may release, reset, or
reuse storage.

The canonical values are:

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

## EnclosingObject

The storage is embedded and disappears or becomes reusable with the enclosing
object's storage.

## StaticDomain

The storage is governed by a static domain.

## ThreadDomain

The storage is governed by one physical thread's thread-local domain.

## Arena

The Arena controls reset or release of the storage.

## OwningValue

The owning value or its deterministic destruction is responsible for
individual reclamation.

## ExternalOwner

Another Sec value, foreign owner, caller, runtime object, or explicit provider
controls reclamation.

## Platform

The loader, operating system, hardware, linker, device, or target platform
controls storage lifetime or reclamation.

## No reclamation

No reclamation occurs during the relevant program lifetime.

`None` does not imply that the storage may be accessed without other lifetime
or synchronization rules.

## Unknown reclamation authority

The compiler cannot identify a verified reclamation authority.

Safe reclamation or retention cannot be inferred.

---

# Address stability

`AddressStability` describes whether the physical address of storage may change.

The canonical values are:

```text
Movable
Stable
Fixed
Unknown
```

## Movable

The storage may relocate during its live storage lifetime when the governing
type and operation permit relocation.

Relocation must preserve or invalidate dependencies according to the relevant
invalidation-domain rules.

## Stable

The address does not change during the current storage lifetime or guaranteed
stability interval.

The address itself is not part of the storage's semantic identity.

## Fixed

The storage is bound to a target address or address domain whose location is
part of its semantics.

Examples include:

- MMIO;
- linker-defined regions;
- interrupt vectors;
- platform control blocks;
- fixed device windows.

## Unknown address stability

The compiler cannot prove address stability.

Safe code must not rely on a stable address.

---

# Orthogonal properties and contracts

The following are not storage origins:

```text
mutability
volatility
atomicity
sharing
thread safety
pinning
mapping
foreign provenance
fixed-address placement
DMA accessibility
```

They are independent properties or contracts.

The same storage may, for example, be:

```text
origin: AllocatorBacked
backing relation: Owned
address stability: Stable
mapped: true
foreign: true
memory space: TargetDefined(ForeignSharedMemory)
```

---

# Memory spaces

A memory space identifies storage with distinct access or transfer rules.

The common memory-space kinds are:

```text
Ordinary
MMIO
TargetDefined
```

## Ordinary

`Ordinary` storage supports ordinary Sec loads, stores, references, copy, move,
and borrowing subject to the normal type, ownership, and concurrency rules.

Automatic, static, thread-local, Arena, and allocator-backed storage may all
occupy ordinary memory.

## MMIO

`MMIO` storage requires target-recognized hardware access semantics.

Its contract may require:

- exact access widths;
- volatile operations;
- ordering constraints;
- barriers;
- read or write side effects;
- target-specific instructions;
- restrictions on ordinary references.

MMIO is distinct from `AddressStability.Fixed` and from volatility, although an
addressed register binding normally combines them through
`rules/platform/fixed-address-bindings.md`.

## TargetDefined

A target may define additional concrete memory spaces, for example:

- accelerator-local memory;
- secure memory;
- non-secure memory;
- non-volatile memory;
- DSP-local memory;
- GPU global memory;
- GPU shared memory;
- foreign address spaces.

Every target-defined space must provide a compiler-known contract defining:

```text
identity
supported access operations
addressability
allowed reference forms
required access widths
alignment rules
atomic support
volatile requirements
coherence rules
accessible execution entities
supported storage origins
explicit transfer operations
target availability
```

Storage origin, foreign provenance, DMA accessibility, sharing, volatility,
atomicity, and address stability remain separate properties.

Transfers between different memory spaces must be explicit and may be fallible.

Sec 0.1 does not introduce general memory-space annotations. It does provide
compiler-known structured destination requests for operations that explicitly
produce shaped storage.

## Source-level storage requests

`MemorySpace` is a compiler-known nominal storage descriptor for an access and
transfer domain. It remains orthogonal to `StorageOrigin`, `BackingRelation`,
`ReclamationAuthority`, and `AddressStability`. Automatic, static, Arena, and
allocator-backed storage are not memory-space values.

The common destination request is:

```sec
struct StorageRequest {
    MemorySpace: Option[MemorySpace]
    MinAlignment: Option[uint]
}
```

An explicitly supplied field is a hard requirement. `None` means no additional
requirement for that dimension; it does not explicitly request `Ordinary`.
`MinAlignment: Some(n)` requires actual destination alignment of at least `n`.
Allocation authority such as `ref mut Arena` is supplied separately and is not
stored in `StorageRequest`.

Allocation-context resolution for an explicit memory-space request must select
a provider capable of satisfying it and must not silently substitute another
space. Public `.MemorySpace` observation is read-only; changing memory space
requires an explicit storage-producing operation.

A successful synchronous `TransferTo` returns a fully initialized destination
and leaves no unrepresented asynchronous dependency on the source. DMA or other
asynchronous transfer requires a distinct explicit API or handle that represents
completion, cancellation, publication, and source/destination lifetime.

---

# Regions and storage dependencies

Regions are compiler-internal lifetime and storage-dependency identities.

A region may represent or depend on:

- an activation;
- an owner;
- backing storage;
- an Arena;
- an allocation;
- a mapping;
- a thread-local instance;
- another compiler-known storage domain.

A place, value, view, reference, or handle may depend on multiple regions.

Conceptually:

```text
StorageDependency {
    RegionIdentity
    InvalidationDomainIdentity?
    ExpectedEpoch?
}
```

This structure is illustrative rather than mandatory implementation syntax.

Region metadata is primarily compiler information.

It need not exist at runtime when all relevant lifetime and invalidation facts
are proved statically.

A region may:

- contain multiple invalidation domains;
- represent one invalidation domain;
- depend on multiple invalidation domains;
- remain live while one contained invalidation domain advances its epoch.

Example:

```text
Arena.Reset()
    Arena owner region remains live
    Arena backing-storage region remains live
    Arena allocation epoch advances
```

---

# Invalidation domains

An invalidation domain is the smallest domain whose previous dependencies become
invalid through one semantic event.

The compiler must not invalidate a larger domain when a smaller precise domain
is sufficient.

Examples:

```text
allocation domain
Arena allocation epoch
collection backing-storage domain
collection element-layout domain
reusable slot domain
mapping domain
object-incarnation domain
```

Object lifetime and storage invalidation must remain distinct.

Replacing one field object does not automatically invalidate the complete
containing storage domain.

---

# Invalidation-domain states

An invalidation domain is in one of these semantic states:

```text
Absent
Live(epoch)
Ended
```

The primitive transitions are:

```text
EstablishDomain
AdvanceEpoch
EndDomain
```

Preserving an epoch is the absence of an invalidating transition.

It is not a lifetime event.

---

# EstablishDomain

`EstablishDomain` performs:

```text
Absent -> Live(initial epoch)
```

It creates or establishes:

- a new domain identity;
- an initial live epoch;
- region dependencies;
- storage classification;
- reclamation authority;
- any required runtime identity or generation state.

Examples include:

- successful allocator-backed allocation;
- Arena creation;
- initial collection backing storage;
- mapping establishment;
- reusable-slot establishment;
- thread-local instance establishment.

An ended domain identity must not ordinarily become live again.

Later storage at the same address must use a new distinguishable domain identity
or a formally preserved logical identity with a new epoch established before
stale access is possible.

---

# AdvanceEpoch

`AdvanceEpoch` performs:

```text
Live(old epoch) -> Live(new epoch)
```

The domain remains live.

Its previous incarnation becomes invalid.

Rules:

- domain identity is preserved;
- the new epoch must be distinguishable from stale epochs;
- every dependency on the old epoch becomes stale;
- new dependencies use the new epoch;
- epoch representation must not silently wrap and make stale dependencies valid;
- generation exhaustion must use widening, domain retirement, slot retirement,
  replacement identity, or another proven strategy.

Examples include:

- Arena reset;
- collection backing-storage reallocation under preserved logical identity;
- collection backing replacement under preserved logical identity;
- reusable-slot reuse;
- invalidating compaction;
- remapping under preserved logical mapping identity;
- explicit backing-storage rebinding under preserved owner identity.

---

# EndDomain

`EndDomain` performs:

```text
Live(epoch) -> Ended
```

Rules:

- every dependency on the domain becomes invalid;
- no new ordinary dependency may be created;
- ordinary access through the domain is forbidden;
- the domain identity is not revived;
- later storage at the same address must remain distinguishable.

Examples include:

- individual deallocation;
- Arena release;
- thread exit for that thread's thread-local domain;
- unmap without continued logical mapping;
- permanent reclamation by an external owner;
- destruction of a controlling storage owner;
- target-defined memory-domain shutdown.

---

# Replacement

Replacing one logical storage domain with another is represented as:

```text
EndDomain(old)
EstablishDomain(new)
```

Replacement must be used when the logical identity is not preserved.

Reset or reuse should use `AdvanceEpoch` only when the same logical domain
continues.

---

# Object lifetime and storage lifetime

Storage may exist without containing a live object.

An object's lifetime begins only after initialization has successfully produced
a valid value of the object's type.

An object's lifetime ends through:

- destruction;
- move from the place;
- replacement;
- explicit object-lifetime termination by a compiler-known operation;
- storage-domain termination.

Ending an object lifetime does not necessarily reclaim the storage.

Reclaiming or resetting storage ends the validity of every dependent object and
reference that has not already ended safely.

Storage may be reused only after the previous object's lifetime in the reused
slots has ended.

Moving an object out of a place:

- ends the source object's lifetime;
- transfers semantic ownership;
- does not necessarily move bytes;
- does not necessarily reclaim the source storage;
- does not necessarily end the containing storage domain.

Embedded storage follows the enclosing storage lifetime.

Individual embedded object lifetimes may begin and end separately when the type
and initialization model permit it.

References must remain valid against:

- the relevant object lifetime;
- every required storage lifetime;
- every required region dependency;
- every required invalidation domain;
- the current expected epoch;
- borrowing and access-authority rules.

Destruction and storage reclamation must be distinct Semantic IR operations.

---

# Typed object storage

Typed object storage contains initialized objects with active object lifetimes.

A fixed array:

```sec
T[N]
```

contains `N` `T` objects according to the normal array initialization rules.

It is not empty construction capacity.

A slice or view over `T[N]` refers to existing initialized objects.

---

# Typed uninitialized storage

Typed uninitialized storage reserves correctly sized and aligned slots for `T`
without claiming that live `T` objects already exist in those slots.

A dedicated compiler-known representation is required.

The exact source type or core API is not defined by this rulebook.

Rules:

- an uninitialized slot must not be read as `T`;
- an uninitialized slot must not be borrowed as `ref T` or `ref mut T`;
- an uninitialized slot must not be destroyed as `T`;
- successful construction begins the object lifetime for that slot;
- destruction ends the object lifetime and returns the slot to an uninitialized
  state;
- initialization state must be tracked for every relevant slot or range;
- partial initialization must be cleaned up correctly on every exit path;
- moving an initialized object out of a slot ends that slot's object lifetime;
- raw bytes do not become typed objects merely because size and alignment are
  sufficient.

A collection using typed uninitialized storage distinguishes:

```text
extent
    number of typed slots physically available

capacity
    number of slots currently available to the value

length
    number of slots currently containing live managed objects
```

The invariant is:

```text
0 <= length <= capacity <= extent
```

---

# Safe typed backing storage

Safe Sec code may use explicitly provided backing storage only through a typed
storage representation.

The representation must establish enough information to validate:

- element type;
- extent;
- capacity where applicable;
- mutability;
- alignment;
- layout compatibility;
- storage lifetime;
- region dependencies;
- initialization state;
- backing relation;
- memory space.

Raw byte storage does not contain typed objects merely because it is large
enough.

Creating typed access from raw storage requires either:

- a compiler-known checked storage operation; or
- an explicit `unsafe` boundary.

A checked safe operation must validate all required size, alignment, layout,
lifetime, memory-space, and initialization facts before exposing typed access.

A `RawPtr[byte]`, byte array, or byte slice must not be freely cast into typed
backing storage in safe code.

---

# Object ownership in backing storage

Backing-storage relation and object ownership are independent.

A collection that receives empty typed backing storage and constructs elements
inside it may own the constructed element objects without owning the backing
bytes.

A view over already initialized objects normally borrows the objects and must
not destroy them.

The type or constructor contract must state whether it:

- owns object lifetimes in the storage;
- borrows existing objects;
- constructs new objects;
- consumes transferred objects;
- returns objects to another owner.

This behavior must never be inferred from `BackingRelation.Borrowed` alone.

A value that owns objects in borrowed backing storage must end or transfer all
owned object lifetimes before releasing its storage borrow.

It must not reclaim the backing storage.

Destroying a borrowed view does not affect the backing storage domain.

Destroying a borrowed-storage container may destroy objects that the container
itself constructed and still owns, according to its type contract.

---

# Embedded backing storage

Embedded backing storage:

- is part of the enclosing representation;
- inherits the enclosing storage origin;
- cannot be rebound independently;
- cannot be individually reclaimed;
- relocates only with the enclosing object;
- retains a fixed physical extent defined by the enclosing type;
- may contain separately tracked object lifetimes.

A fixed array `T[N]` always has a fixed embedded extent.

---

# Borrowed backing storage

A borrower must not relocate, reallocate, reset, replace, or reclaim borrowed
backing storage.

The borrower may change its binding to another backing store only through an
explicit rebinding operation permitted by the type.

Before rebinding:

- all objects owned by the borrower in the old storage must be destroyed or
  transferred;
- dependent references and views must have ended or become safely invalidated;
- active borrows of the old binding must end;
- pins and reclamation protections must be resolved;
- the old backing-storage borrow must be released.

The new binding establishes new region and invalidation dependencies.

A borrower must never silently convert borrowed fixed storage into owned
dynamically allocated storage when capacity is exhausted.

---

# Owned backing storage

Owned backing storage may be replaced or grown only through an explicit
operation permitted by the type contract.

A replacement operation must:

1. establish or acquire valid new storage;
2. construct, move, or copy live objects according to their type semantics;
3. handle partial failure without leaks, double destruction, or stale ownership;
4. preserve the required evaluation and destruction order;
5. end old object lifetimes that are not transferred;
6. reclaim old storage through the correct authority;
7. invalidate direct references and views that depended on the old storage;
8. preserve stable handles only when their resolution contract remains valid;
9. perform the required epoch transition.

If logical backing identity is preserved, replacement uses `AdvanceEpoch`.

If logical identity is replaced, it uses:

```text
EndDomain(old)
EstablishDomain(new)
```

Backing storage must never be rebound, replaced, or relocated implicitly.

---

# Capacity and growth

A type with backing storage must classify its capacity policy as conceptually:

```text
FixedCapacity
Growable
```

The exact compiler or library representation may differ.

## Fixed-capacity storage

Embedded and borrowed backing storage normally have a fixed maximum extent.

Capacity exhaustion must not cause:

- hidden allocation;
- hidden relocation;
- hidden rebinding;
- hidden conversion to owned storage;
- silent truncation;
- overwrite beyond capacity.

A fallible operation must return an explicit error when exhaustion is possible.

A compile-time-provable overflow must be rejected statically when the operation
cannot succeed.

Logical capacity may be reduced only when every live element remains within the
valid range.

## Growable storage

Growable owned storage may increase capacity only through an operation whose:

- allocation behavior is explicit;
- failure behavior is explicit;
- invalidation behavior is explicit;
- memory-space behavior is explicit;
- ownership transfer is defined.

Growth that can allocate or relocate must be fallible unless the active provider
proves an infallible bounded operation.

Growth without relocation or element invalidation does not normally advance the
backing epoch.

Growth with invalidating relocation follows the backing-replacement rules.

The same value must not silently switch from borrowed fixed storage to owned
allocated storage.

---

# Storage selection and placement

The compiler may optimize physical placement.

It must not change storage semantics.

Automatic values may lower to:

- SSA;
- registers;
- stack;
- caller-provided return storage;
- eliminated storage.

Taking an address, returning a reference, capturing a value, retaining a value,
or detecting escape must not cause hidden dynamic allocation.

Dynamic storage may be established only through:

- an explicit allocating operation;
- an operation whose documented semantics allocate;
- an explicit or compiler-propagated allocation context defined by
  `allocation.txt`;
- a target contract that explicitly provides storage.

If the required storage lifetime cannot be proved or explicitly supplied, the
program is invalid.

The compiler may change physical placement only when it preserves:

- storage identity;
- object identity where observable;
- address stability;
- ownership;
- object lifetime;
- storage lifetime;
- destruction;
- reclamation;
- invalidation;
- generation semantics;
- memory-space semantics;
- volatile and atomic behavior;
- observable ordering.

Moving a descriptor value does not move its backing storage.

Backing storage may relocate only when its type and active constraints permit
it.

---

# Runtime validity generations

Every invalidation domain has a semantic validity epoch.

Dynamically reclaimable or reusable storage normally requires a
runtime-represented domain identity and generation when safe references or
handles may:

- escape their immediate lexical use;
- be stored;
- be returned;
- cross calls that may invalidate the domain;
- cross task or thread boundaries;
- survive reset, replacement, removal, or reuse;
- be resolved after the storage may have been recycled.

This normally applies to:

- allocator-backed allocations;
- resettable Arenas;
- reallocating collection storage;
- reusable slots;
- replaceable mappings;
- externally invalidatable storage wrappers.

Runtime generation state belongs to the invalidation domain.

An individual reference or handle may carry:

- domain identity;
- slot identity where applicable;
- expected generation;
- bounds;
- access authority;
- protection dependency.

The reference model decides the exact reference or handle category and
representation.

A short-lived direct reference does not need to carry an expected generation or
perform a runtime generation check when the compiler proves that:

- the domain remains live;
- no invalidating operation occurs before the final use;
- no concurrent invalidation is possible;
- the reference does not escape;
- all region dependencies remain valid.

This optimization does not remove the semantic epoch from the domain.

It only removes redundant per-reference metadata or checks.

Generation values must not silently wrap so that stale references or handles
become valid again.

A profile must use one or more of:

- a sufficiently wide checked generation;
- domain retirement;
- slot retirement;
- replacement domain identity;
- a compound identity;
- another proven non-reuse strategy.

---

# Pinning

Pinning is a temporary region-bounded constraint on storage.

Pinning is not:

- a storage origin;
- a backing relation;
- a reclamation authority;
- a new address-stability category;
- ownership;
- mutability;
- thread safety;
- automatic lifetime extension.

While a pin is active, the relevant storage must not be:

- relocated;
- rebound;
- replaced;
- deallocated;
- reset;
- unmapped;
- reclaimed;
- ended in another way prohibited by the pin contract.

An operation requiring forbidden relocation or reclamation while storage is
pinned must:

- be rejected statically;
- be unavailable through the type API;
- block under an explicitly defined synchronization protocol; or
- fail explicitly.

Pinning must not silently extend the owner or storage lifetime.

The pin itself depends on every required owner and storage region.

References requiring the pinned address must not outlive the pin unless another
independent address-stability and reclamation guarantee is established.

Ending a pin does not itself advance the epoch or invalidate references.

A move-only compiler-known pin guard is the preferred model.

The exact source API is defined by core, FFI, DMA, device, or other specialized
rules.

Sec 0.1 does not require a general `pin` keyword.

---

# Concurrent invalidation and reclamation

Generation validation detects stale identity.

It does not prevent concurrent invalidation.

This sequence is unsafe without a reclamation protocol:

```text
1. Validate generation.
2. Another execution entity reclaims or replaces storage.
3. Access the previous storage.
```

Safe access to concurrently reclaimable storage requires a verified protocol
that protects the domain from successful validation through the final dependent
access.

Conceptually:

```text
1. Acquire reclamation protection.
2. Validate identity and expected generation.
3. Access the object while protection remains active.
4. Release protection.
```

Possible protocol classes include:

- exclusive ownership;
- shared ownership with defined reclamation;
- lock guards;
- pin guards;
- atomic slot protocols;
- hazard protection;
- epoch-based reclamation;
- compiler-known target protocols.

This list does not require source-level types with these exact names.

Reclamation, reset, replacement, or reuse must wait until all relevant
protections for the old incarnation have ended.

A reference acquired under a guard must not outlive the guard unless another
independent guarantee is established.

Pinning may prevent relocation or reclamation but does not by itself prevent
data races or grant mutation authority.

Storage that may be asynchronously invalidated by another thread, task, foreign
library, operating system, device, callback, interrupt, or runtime must not be
exposed as an ordinary long-lived safe reference without a verified protection
contract.

When neither static exclusion nor a verified runtime protocol exists, safe
access is invalid.

An explicit `unsafe` boundary may acknowledge responsibility but does not make
the operation memory-safe.

---

# Canonical invalidation behavior

The following table defines the default classification of common events.

| Storage or operation | Event | Domain result |
|---|---|---|
| Automatic | activation storage begins | `EstablishDomain` |
| Automatic | ordinary mutation | no invalidating transition |
| Automatic | move from one place | source object lifetime ends; storage domain remains |
| Automatic | replacement in existing place | old object lifetime ends; new object lifetime begins |
| Automatic | governing activation ends | `EndDomain` |
| Static | static storage established | `EstablishDomain` |
| Static | ordinary mutation or replacement | no storage-domain transition |
| ThreadLocal | thread instance established | `EstablishDomain` |
| ThreadLocal | physical thread exits | `EndDomain` |
| Arena | Arena established | `EstablishDomain` |
| Arena | ordinary allocation | no Arena epoch transition |
| Arena | reset | `AdvanceEpoch` |
| Arena | release | `EndDomain` |
| AllocatorBacked | successful allocation | `EstablishDomain` |
| AllocatorBacked | ordinary mutation | no invalidating transition |
| AllocatorBacked | in-place growth without invalidation | no invalidating transition |
| AllocatorBacked | reallocation preserving logical identity | `AdvanceEpoch` |
| AllocatorBacked | logical replacement | `EndDomain(old)` then `EstablishDomain(new)` |
| AllocatorBacked | deallocation | `EndDomain` |
| Collection | element mutation in place | no backing transition |
| Collection | append without relocation or element invalidation | no backing transition |
| Collection | invalidating backing reallocation | `AdvanceEpoch` or logical replacement |
| Collection | backing replacement | `AdvanceEpoch` or logical replacement |
| Collection | remove element | removed object lifetime ends |
| Collection | index-changing movement | relevant layout or slot domain advances when handles may survive |
| Collection | clear | element lifetimes end; backing domain may remain |
| Owned collection | destruction | owned backing domain ends when reclaimed |
| Borrowed view | destruction | underlying backing domain remains |
| Borrowed storage | owner reuses preserved logical storage | `AdvanceEpoch` |
| Borrowed storage | owner replaces logical storage | epoch advance or logical replacement |
| Borrowed storage | owner permanently reclaims storage | `EndDomain` |
| Reusable slot | slot established | `EstablishDomain` |
| Reusable slot | occupant removed | occupant object lifetime ends |
| Reusable slot | slot reused | `AdvanceEpoch` before new occupant becomes accessible |
| Reusable slot | slot storage retired | `EndDomain` |
| Mapping | mapping established | `EstablishDomain` |
| Mapping | content mutation | no storage-domain transition |
| Mapping | remap preserving mapping identity | `AdvanceEpoch` |
| Mapping | replacement by new mapping identity | `EndDomain(old)` then `EstablishDomain(new)` |
| Mapping | unmap | `EndDomain` |
| Fixed address | external value changes | no storage epoch transition by itself |
| Fixed address | binding created | creates a binding or view, not necessarily the platform storage |

A specialized type or target contract may refine this table only by preserving
or strengthening safety.

---

# Collection storage domains

A collection may contain several distinct domains:

```text
collection object domain
backing-storage domain
element-layout domain
reusable slot domains
stable-handle control domain
```

The compiler must not treat them as one identity when their invalidation rules
differ.

Direct element references normally depend on the current backing-storage and
layout incarnation.

Stable slot handles normally depend on:

- collection control-domain identity;
- slot identity;
- expected slot generation;
- a reclamation-protection protocol during resolution.

Removing one element must not necessarily advance the complete collection
backing epoch when other stable slot identities remain unaffected.

Structural mutation while direct element references are live is governed by
borrowing and collection rules.

Runtime epochs are required when a type deliberately permits safe handles to
survive possible structural invalidation.

---

# Mapped storage contract

Mapped storage is an orthogonal contract.

It is not a storage origin.

Its storage origin remains independently classified. An individually established
and reclaimed mapping may be `AllocatorBacked`; a platform-owned mapping may be
`Static` or `Unknown` according to the verified contract.

A mapping contract must define:

```text
mapping identity
mapping owner
mapped extent
alignment
layout or element interpretation
memory space
access permissions
address stability
invalidation operations
reclamation operation
thread and process visibility
synchronization requirements
```

Moving a mapping wrapper does not move or remap the underlying storage.

An owning wrapper may have reclamation authority for `unmap`.

A borrowed mapping view does not.

`unmap` ends the mapping domain.

Remapping under preserved logical mapping identity advances its epoch when old
references become invalid.

Replacing the logical mapping ends the old domain and establishes a new one.

References and views must not outlive an invalidating remap or unmap.

---

# Foreign storage contract

Foreign storage is an orthogonal provenance and lifetime contract.

It is not a storage origin.

Its storage origin remains independently classified when the contract proves
one. Otherwise the origin is `Unknown`.

A foreign storage contract must define at least:

```text
foreign owner
allocator or provider
matching release operation
lifetime
extent
alignment
layout
mutability
retention behavior
thread access
invalidation behavior
success ownership
failure ownership
memory space
synchronization requirements
```

`RawPtr[T]` does not establish:

- ownership;
- lifetime;
- bounds;
- valid representation;
- reclamation authority;
- generation safety;
- thread safety.

Sec may reclaim foreign storage only through the matching operation specified by
the contract.

Retained pointers, callbacks, and asynchronous foreign access require explicit
lifetime, invalidation, and thread contracts.

Asynchronous foreign invalidation must not be exposed through an ordinary
unprotected long-lived `ref`.

Unknown foreign lifetime is conservative and must not be treated as static or
unlimited.

The complete ABI and wrapper rules remain defined by `rules/platform/ffi.md`.

---

# Fixed-address storage contract

Fixed-address storage is an orthogonal placement contract using
`AddressStability.Fixed`.

It is not a storage origin.

Its storage origin remains independently classified. A linker-defined static
region may be `Static`; a platform-owned hardware region may be `Unknown` unless
a stronger target contract exists.

A fixed-address contract must define:

```text
address identity
target address domain
extent
alignment
layout
memory space
access permissions
aliasing policy
reclamation authority
lifetime or availability contract
```

The address is part of the storage semantics.

The storage must not relocate.

A wrapper or handle referring to the storage may remain movable.

Creating a binding to fixed-address storage:

- does not allocate storage;
- does not initialize the external storage;
- does not transfer platform ownership;
- does not by itself prove unlimited lifetime.

`Fixed` alone does not imply:

- volatility;
- mutability;
- atomicity;
- thread safety;
- permanent lifetime.

An `@address` register binding is volatile because
`rules/platform/fixed-address-bindings.md` defines that additional rule.

Statically known overlapping fixed-address bindings must be validated against:

- layout;
- extent;
- mutability;
- aliasing;
- access width;
- memory-space contract.

Dynamic or unverifiable fixed addresses require an explicit `unsafe` boundary
or a compiler-known checked API.

---

# Storage effects

Effect analysis must preserve storage-relevant effects whose ordering may change
validity or reclamation.

At minimum, summaries must be able to represent operations that may or must:

```text
EstablishDomain
AdvanceEpoch
EndDomain
AllocateStorage
ReclaimStorage
RelocateStorage
RebindStorage
PinStorage
UnpinStorage
AcquireReclamationProtection
ReleaseReclamationProtection
```

A useful call-summary distinction is:

```text
NoInvalidation
MayAdvanceEpoch(domain)
MustAdvanceEpoch(domain)
MayEndDomain(domain)
MustEndDomain(domain)
```

These are effect-summary concepts.

They are not additional invalidation-domain state transitions.

Calls whose storage effects are unknown must be treated conservatively.

The compiler must not move a storage-invalidating operation across:

- dependent accesses;
- destruction;
- pin acquisition or release;
- reclamation-protection acquisition or release;
- volatile accesses;
- synchronization;
- foreign calls with relevant effects;
- cleanup boundaries;
- defer behavior.

---

# Semantic analysis requirements

Semantic analysis must determine or conservatively classify:

- storage origin;
- backing relation;
- reclamation authority;
- address stability;
- memory space;
- storage identity;
- allocation identity where applicable;
- region dependencies;
- invalidation-domain dependencies;
- expected epochs;
- runtime-generation requirements;
- object initialization state;
- storage initialization state;
- move state;
- active borrows;
- active pins;
- active reclamation protections;
- permitted relocation;
- permitted replacement;
- permitted reclamation;
- possible concurrent invalidation.

Safe code must be rejected when required information remains `Unknown` and no
verified runtime protocol establishes safety.

The compiler must not infer ownership, lifetime, or reclamation authority from:

- a numeric address;
- `RawPtr[T]`;
- exclusive mutability alone;
- physical adjacency;
- equal storage size;
- a matching layout without a valid object-lifetime operation.

---

# Semantic IR requirements

Semantic IR must distinguish stable storage facts, path-dependent state, and
explicit semantic operations.

## Stable storage facts

A materialized storage domain must expose or reference facts equivalent to:

```text
StorageDomain {
    Identity
    Origin
    BackingRelation
    ReclamationAuthority
    AddressStability
    MemorySpace
    RegionDependencies
    InvalidationDomain
    RuntimeGenerationRequired
}
```

This structure is illustrative rather than mandatory syntax.

An invalidation domain may be absent when the storage cannot be invalidated or
reused independently in any relevant safe access pattern.

## Path-dependent state

The following are control-flow facts and must not be treated as immutable
storage-domain properties:

```text
InitializationState
CurrentEpochKnowledge
ActiveBorrows
ActivePins
ActiveReclamationProtections
MovedState
ObjectLifetimeState
PartialInitializationMask
PartialMoveMask
```

A place-state model must distinguish at least:

```text
Uninitialized
PartiallyInitialized
Initialized
Moved
LifetimeEnded
```

Aggregates and typed slot storage require field- or slot-sensitive state where
semantics depend on it.

## Explicit operations

Semantic IR must be able to represent operations equivalent to:

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

The exact node names may differ.

The semantic distinctions must remain explicit.

In particular:

- `ConstructObject` does not imply storage allocation;
- `EndObjectLifetime` does not imply storage reclamation;
- value move does not imply physical relocation;
- physical relocation does not necessarily imply logical replacement;
- epoch advancement is distinct from relocation;
- generation validation is distinct from reclamation protection;
- destruction is distinct from deallocation or reset.

## Runtime metadata

Semantic IR must specify required semantics without requiring one physical
layout.

Runtime information may include:

```text
domain identity
current generation
expected generation
slot identity
pin state
protection state
bounds
control-block address
```

The backend may implement this through:

- object metadata;
- side tables;
- slot metadata;
- owner control blocks;
- atomics;
- tagged or capability addresses;
- target hardware support;
- eliminated metadata after proof.

---

# Lowering and optimization

Lowering may select target representations and remove redundant operations only
when the canonical semantics remain equivalent.

The compiler may eliminate runtime identity, generation metadata, checks, pins,
or protection operations only when it proves all corresponding safety facts.

It must not remove or reorder behavior in a way that changes:

- storage lifetime;
- object lifetime;
- destruction order;
- reclamation order;
- invalidation timing;
- generation visibility;
- concurrent protection;
- volatile behavior;
- atomic behavior;
- memory-space transfer;
- foreign retention;
- fixed-address access.

LLVM pointer values must not be used as the sole representation of storage
identity or validity.

Backend address equality must not be treated as proof of one logical storage
identity.

A target profile may reject a storage operation that the target cannot support.

It must not silently weaken the safety model.

---

# Diagnostics

Storage diagnostics must identify the relevant semantic facts when available.

A diagnostic should identify:

- the storage or invalidation domain;
- the original storage dependency;
- the invalidating operation;
- the place or reference being used;
- expected and current generation when relevant;
- active borrow, pin, or reclamation protection;
- forbidden relocation, replacement, reset, or reclamation;
- the source location that established the dependency;
- the source location that caused invalidation;
- the reason static proof failed.

Required diagnostic classes include:

- hidden dynamic allocation would be required;
- automatic storage does not live long enough;
- borrowed backing storage cannot be reclaimed;
- borrowed backing storage cannot be replaced;
- embedded storage cannot be rebound independently;
- fixed-capacity storage exhausted;
- capacity change would exclude live objects;
- raw storage cannot be used as typed storage safely;
- typed slot is uninitialized;
- typed slot is already initialized;
- object lifetime has ended;
- storage domain has ended;
- stale generation;
- relocation invalidates a live direct reference;
- storage is pinned;
- reclamation protection is active;
- concurrent generation check is unprotected;
- unknown storage origin or authority is insufficient for safe use;
- foreign contract lacks required lifetime or reclamation information;
- fixed-address bindings overlap incompatibly;
- memory-space access or transfer is unsupported.

Example diagnostic shape:

```text
error: reference depends on an earlier Arena epoch

  reference created here: value.sec:12:17
  Arena reset here:       value.sec:18:5
  reference used here:    value.sec:21:9

  expected epoch: 4
  current epoch:  5

help: end the reference before Reset or acquire new storage after Reset
```

Example concurrent-reclamation diagnostic shape:

```text
error: generation validation does not protect this access from concurrent reuse

  handle validated here: connection.sec:44:18
  access occurs here:     connection.sec:46:9
  slot may be reused by:  CloseConnection

help: resolve the handle through a protected slot guard
```

Diagnostic IDs and configurable severity follow `diagnostics.txt`.

---

# Required tests

The implementation must provide parser-independent semantic and lowering tests
for this rulebook.

At minimum, tests must cover the following groups.

## Storage origins

- automatic local storage;
- automatic parameter storage;
- static storage;
- separate thread-local instances;
- Arena storage;
- allocator-backed storage;
- conservative unknown origin.

## Backing relations

- no backing storage;
- embedded fixed array `T[N]`;
- owned backing storage;
- borrowed backing storage;
- Arena-backed owned object content with Arena reclamation authority;
- borrowed storage with separately owned object lifetimes.

## Object and storage lifetimes

- storage existing before construction;
- construction beginning object lifetime;
- destruction ending object lifetime without reclaiming storage;
- move ending the source object lifetime without ending storage;
- replacement in existing storage;
- storage reuse after object-lifetime termination;
- invalid access after storage-domain termination.

## Typed uninitialized storage

- construction into an empty typed slot;
- read before initialization;
- destruction of uninitialized slot;
- duplicate construction;
- partial initialization cleanup;
- move from initialized slot;
- reuse after destruction;
- `length`, `capacity`, and `extent` invariants.

## Placement

- automatic value kept in SSA;
- automatic value materialized on stack;
- no hidden dynamic allocation after address taking;
- no hidden dynamic allocation after escape;
- descriptor move without backing relocation;
- rejected lifetime that would require hidden promotion.

## Arena invalidation

- ordinary allocation preserving epoch;
- reset advancing epoch;
- stale reference after reset;
- release ending domain;
- use after release;
- borrowed Arena backing remaining externally owned.

## Allocator-backed storage

- allocation establishing identity;
- deallocation ending identity;
- same-address reuse with distinguishable generation or identity;
- reallocation preserving logical identity and advancing epoch;
- logical replacement establishing a new domain;
- generation exhaustion or retirement behavior.

## Collections

- append without relocation;
- invalidating reallocation;
- element removal;
- clear preserving backing storage;
- reusable slot generation;
- stale slot handle;
- direct reference invalidated by relocation;
- stable handle resolving after permitted relocation;
- fixed-capacity exhaustion without hidden growth.

## Borrowed backing storage

- owner outliving borrower;
- conflicting owner reuse while borrowed;
- explicit rebinding;
- destruction of borrower-owned objects before borrow release;
- borrowed view destruction not reclaiming backing;
- rejection of implicit conversion to owned storage.

## Pinning tests

- relocation rejected while pinned;
- reset rejected while pinned;
- reclamation rejected while pinned;
- pin guard bounded by owner lifetime;
- reference not escaping pin guard;
- unpin preserving epoch;
- pinned access still requiring synchronization for mutation.

## Mapped, foreign, and fixed-address storage

- map and unmap;
- invalidating remap;
- borrowed mapping view;
- foreign storage with matching release operation;
- missing foreign lifetime contract;
- fixed address without implied mutability;
- addressed register volatility through
  `rules/platform/fixed-address-bindings.md`;
- overlapping fixed-address validation;
- target-defined memory-space restrictions.

## Concurrent reclamation

- stale generation detected under a guard;
- check-then-access rejected without protection;
- slot protection spanning validation and access;
- reclamation waiting for guards;
- reference not escaping a lock or protection guard;
- asynchronous foreign invalidation requiring a protocol.

## Semantic IR and lowering

- distinct construction and allocation operations;
- distinct destruction and reclamation operations;
- explicit epoch advance;
- explicit domain end;
- explicit backing rebinding;
- explicit physical relocation;
- explicit pin acquisition and release;
- explicit reclamation protection;
- safe elimination of redundant generation checks;
- preservation of required checks across calls and control flow.

---

# Required synchronization

Adopting this rulebook requires synchronization with nearby canonical documents.

At minimum:

## `memory_model.md`

The older origin list:

```text
Inline
Static
Arena
External
Foreign
FixedAddress
Unknown
```

must be replaced or mapped to the canonical model:

```text
Automatic
Static
ThreadLocal
Arena
AllocatorBacked
Unknown
```

The following older concepts must move to independent properties:

```text
Inline
    Embedded backing relation or automatic placement detail

External
    Borrowed relation or external-owner reclamation authority

Foreign
    foreign storage contract

FixedAddress
    AddressStability.Fixed plus fixed-address contract
```

## `reference_model.md`

Reference and handle categories must consume the storage-domain, region,
invalidation, and runtime-generation model from this document.

Reference-specific bounds, provenance, handle resolution, equality, and stale
failure behavior remain owned by `reference_model.md`.

## `allocation.txt`

Allocation selection and failure remain owned by `allocation.txt`.

It must use `AllocatorBacked` and `Arena` origins consistently and preserve the
no-hidden-promotion rule.

## `arena.md`

Arena ownership kinds and backing kinds must map to `BackingRelation` and
`ReclamationAuthority` without creating a competing global storage model.

Arena reset and release must use `AdvanceEpoch` and `EndDomain` respectively.

## `collections.md` and `shaped-types.md`

Collection storage models must distinguish:

- embedded, owned, and borrowed backing;
- typed object storage and typed uninitialized storage;
- extent, capacity, and length;
- direct-reference epochs and reusable-slot generations;
- fixed-capacity and growable behavior;
- memory-space contracts.

## `effect_analysis.md`

Ordered effects must expose storage invalidation, reclamation, rebinding,
relocation, pinning, and protection behavior when relevant to legality or
optimization.

## `semantic_ir.txt`

Semantic IR must add or map all stable facts, path-dependent states, and
explicit operations required by this rulebook.

## `platform/ffi.md`, `declarations/registers.md`, `platform/fixed-address-bindings.md`, and concurrency rulebooks

These documents must consume the foreign, fixed-address, MMIO, generation, and
concurrent-reclamation principles without redefining storage origin or epoch
ownership.

No canonical rulebook may retain a competing definition of storage origin,
region, invalidation domain, or validity epoch.

---

# Canonical summary

The following rules are canonical:

```text
Storage origin identifies the primary storage domain.

Backing relation identifies how a value relates to separate backing storage.

Reclamation authority identifies who may release, reset, or reuse storage.

Address stability identifies whether storage may relocate.

Memory space identifies distinct access and transfer rules.

Regions are compiler-internal lifetime and storage-dependency identities.

Invalidation domains are the smallest domains invalidated by one semantic event.

Validity epochs identify live incarnations of invalidation domains.

The primitive domain transitions are EstablishDomain, AdvanceEpoch, and
EndDomain.

Object lifetime and storage lifetime are separate.

Destruction and reclamation are separate.

Move and physical relocation are separate.

Safe explicit backing storage is typed.

Raw storage requires a checked compiler-known operation or unsafe code before it
may contain typed objects.

Capacity exhaustion never causes hidden allocation or relocation.

Escape never causes hidden dynamic promotion.

Dynamically reclaimable or reusable domains require runtime generations when
safe references or handles may survive invalidation.

The compiler may remove per-reference metadata or checks only after proving the
same validity statically.

Pinning prevents relocation and required reclamation for a bounded lifetime but
does not grant ownership, mutability, thread safety, or unlimited lifetime.

Generation validation alone does not make concurrent reclamation safe.

Unknown storage information provides no positive safety guarantee.
```
