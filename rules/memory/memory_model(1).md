# Memory Model

- Status: Normative
- Created: 2026-09-02
- Last updated: 2026-09-02
- Document revision: 2.0
- Sec language version: 0.1
- Canonical path: `rules/memory/memory_model.md`
- Replaces: previous revision of `rules/memory/memory_model.md`
- Repository baseline reviewed: `814a584` (latest publicly verifiable `main`; current `main` contents reviewed 2026-09-02)

---

## § 1 Purpose and authority

§ 1(1) This rulebook defines the common abstract memory machine used by Sec 0.1.

§ 1(2) It defines shared concepts consumed by ownership, copy/move, borrowing, references, raw pointers, lifetime analysis, destruction, allocation, storage, layout, unsafe, transferability, concurrency, FFI, registers, hardware access, and lowering.

§ 1(3) Specialized rulebooks own their detailed source syntax and analysis algorithms.

§ 1(4) This book owns the common meanings of:

```text
value
object
binding
Place
root Place
sub-Place
storage location
storage domain
object lifetime
initialization
availability
unavailability reason
ownership state
memory location identity
Place relationship
addressability
relocation
reference dependency
resource state
memory-space relationship
semantic memory operation
```

§ 1(5) No specialized subsystem may invent a second incompatible Place, storage, location, ownership, lifetime, or reference-identity model.

§ 1(6) Sema establishes Sec memory semantics.

§ 1(7) Semantic IR preserves and verifies required memory facts and operations.

§ 1(8) MLIR/backend lowering implements those facts without inventing new ownership, allocation, lifetime, storage, or invalidation meaning.

---

## § 2 Core memory principles

§ 2(1) A value is not necessarily a materialized object.

§ 2(2) An object is a live materialized value in storage.

§ 2(3) Storage may exist without containing a live object.

§ 2(4) Binding identity, Place identity, object identity, storage identity, and numeric address are distinct concepts.

§ 2(5) Ownership is semantic responsibility for a value/resource, not mere address possession.

§ 2(6) Move transfers ownership; it does not inherently relocate bytes.

§ 2(7) Borrowing grants temporary authority; it does not transfer ownership.

§ 2(8) Reference validity requires lifetime/provenance/borrow/storage conditions; address equality alone is insufficient.

§ 2(9) Destruction ends value/object lifecycle responsibility; reclamation concerns storage.

§ 2(10) Allocation creates/provides storage; it does not automatically initialize a valid object.

§ 2(11) Unsafe transfers proof obligations but does not disable Sec semantics.

§ 2(12) Volatile access is not atomicity or synchronization.

§ 2(13) Memory-space and target constraints are explicit semantic facts, not backend guesses.

---

## § 3 Value

§ 3(1) A value is a typed semantic entity.

§ 3(2) Values may exist only in semantic/SSA form and need not occupy addressable storage.

§ 3(3) A value has semantic type identity independent of its physical layout.

§ 3(4) A value may be copyable, move-only, borrowed, owned, trivial, non-trivially destructible, or otherwise constrained according to specialized rules.

§ 3(5) A value may represent or control resources that are not ordinary memory.

---

## § 4 Object

§ 4(1) An object is a materialized value whose object lifetime has begun in storage.

§ 4(2) A storage location may contain no live object, one live object, or a set of live subobjects permitted by the type/construction model.

§ 4(3) A new object lifetime begins through valid initialization/construction/reinitialization.

§ 4(4) Object lifetime ends through move-out where applicable, replacement, explicit destruction, automatic destruction, variant change, owner termination, or another canonical lifecycle operation.

§ 4(5) End of object lifetime does not automatically end storage lifetime.

---

## § 5 Binding

§ 5(1) A binding associates a source name with a value, Place, reference, function/type entity, static member, or compiler-known entity.

§ 5(2) A binding is not itself the value or storage.

§ 5(3) Rebinding or shadowing names does not change semantic identity of already existing values/storage unless an operation explicitly does so.

§ 5(4) Compiler analyses must resolve source names to canonical semantic identities before memory reasoning.

---

## § 6 Place

§ 6(1) A Place is a source-level location expression that can identify a value-bearing location or addressable operation target.

Examples:

```sec
value
person.Name
array[index]
object.property
```

§ 6(2) Not every expression is a Place.

§ 6(3) Temporary computation values, arithmetic results, and arbitrary function results need not be Places unless materialized/addressable by a canonical rule.

§ 6(4) A Place may independently be readable, writable, addressable, movable, borrowable, volatile, atomic, fixed-address, partially available, or conditionally available.

§ 6(5) Those capabilities are not one combined flag.

---

## § 7 Root Place

§ 7(1) A root Place is a Place not derived from another source Place.

Examples include:

```text
local binding
parameter storage
static binding
thread-local binding
foreign object
fixed-address binding
allocation root
mapping root
```

§ 7(2) Root Place identity is compiler-owned and must not be approximated solely by source spelling.

§ 7(3) Distinct root Places remain distinct even when their numeric addresses temporarily coincide under unsafe/foreign behavior unless a canonical alias contract says otherwise.

---

## § 8 Sub-Places and Place paths

§ 8(1) A sub-Place is a Place within another Place.

Examples:

```sec
person.Name
pair.Left
array[3]
matrix[row, column]
```

§ 8(2) A Place path conceptually contains root identity and zero or more projections.

§ 8(3) Projections may include:

```text
field
constant index
dynamic index
range/slice
variant payload
dereference
view mapping
compiler-known projection
```

§ 8(4) The compiler may use symbolic indexes/ranges and constraints to prove identity, containment, or disjointness.

§ 8(5) Source semantics do not depend on one specific internal path encoding.

---

## § 9 Memory-location identity

§ 9(1) Memory-location identity describes the semantic location affected by reads, writes, borrows, moves, destruction, volatile access, and race analysis.

§ 9(2) Memory-location identity is derived from canonical Place/storage/provenance facts.

§ 9(3) Numeric address alone is insufficient where storage incarnation, variant, mapping, generation, or subobject identity matters.

§ 9(4) Concurrency analysis must consume the same location identity as ownership/borrowing/storage analysis.

§ 9(5) There must not be a separate concurrency-only location identity model.

---

## § 10 Place relationships

§ 10(1) For two Places, analysis may derive:

```text
Same
Disjoint
Contains
ContainedBy
MayOverlap
Unknown
```

§ 10(2) Exact internal names may differ.

§ 10(3) `Same` means both expressions identify the same semantic location.

§ 10(4) `Disjoint` means they are proven non-overlapping.

§ 10(5) `Contains`/`ContainedBy` capture aggregate/sub-Place relationships.

§ 10(6) `MayOverlap` means overlap is possible on represented paths.

§ 10(7) `Unknown` grants no positive disjointness guarantee.

§ 10(8) Borrow, move, destruction, mutation, race, and transfer analyses must consume these canonical relationships.

---

## § 11 Disjointness

§ 11(1) Separate direct struct fields are normally disjoint where type/layout semantics permit independent access.

§ 11(2) Distinct constant array indices are disjoint when both refer to distinct valid elements.

§ 11(3) Statically non-overlapping ranges/views may be disjoint.

§ 11(4) Dynamic indices/ranges require proof; lack of proof is not disjointness.

§ 11(5) Unions/overlays may cause semantic overlap according to active-variant/layout rules.

§ 11(6) MMIO/register aliases may require platform alias knowledge that overrides naive source-Place separation.

---

## § 12 Value state and storage state

§ 12(1) The compiler must distinguish value state from storage state.

§ 12(2) Value state concerns initialized object/value availability and ownership.

§ 12(3) Storage state concerns storage-domain lifetime, validity epoch, backing relation, mapping, reclamation, and address stability.

§ 12(4) A Place may have live storage while containing no currently available object.

§ 12(5) A Place may be unavailable because ownership moved while the storage slot remains valid for reinitialization.

§ 12(6) A stale storage epoch may make access invalid even when a lexical binding still exists.

---

## § 13 Availability model

§ 13(1) Canonical availability states are:

```text
Uninitialized
Available
PartiallyAvailable
Unavailable
ConditionallyAvailable
```

§ 13(2) Availability answers whether the current owner owns a value in the Place on the current path.

§ 13(3) `Unavailable` reasons are tracked separately.

§ 13(4) Example reasons include:

```text
Moved
Discarded
Detached
Destroyed
NeverInitialized
VariantInactive
```

§ 13(5) Specialized rulebooks may define additional provenance/reason facts without replacing the availability lattice.

§ 13(6) Mutability and availability are orthogonal.

---

## § 14 `is available`

§ 14(1) `place is available` queries ownership availability for the current owner/path.

§ 14(2) `place is not available` is its negation.

§ 14(3) Availability is not nullability.

§ 14(4) Availability is not `Option.Some/None`.

§ 14(5) Availability is not reference-generation validity.

§ 14(6) Availability is not borrow permission.

§ 14(7) Availability is not device liveness, mapping status, or hardware readiness.

§ 14(8) The compiler resolves availability statically where possible and uses runtime state only where the selected profile/semantics genuinely require it.

---

## § 15 Initialization

§ 15(1) Readable safe values must be initialized with a valid semantic representation.

§ 15(2) Default initialization is defined by `default_values.md`.

§ 15(3) A semantic default is not necessarily all-bits-zero.

§ 15(4) Mutable declarations/omitted fields requiring default initialization must obtain a valid initialized value before read.

§ 15(5) Uninitialized storage is not an ordinary readable value.

§ 15(6) Partial initialization requires field/element/payload-aware state.

§ 15(7) Destruction runs only for initialized subobjects whose lifetime began.

---

## § 16 Default values

§ 16(1) Defaultability is a semantic type property.

§ 16(2) An empty `list[T]` default owns no allocated element storage and constructs no elements.

§ 16(3) A safe slice is not defaultable merely by inventing a null reference because every slice requires a valid storage origin/lifetime contract.

§ 16(4) A type may have size/layout yet not have a valid zero/default representation.

---

## § 17 Ownership

§ 17(1) Ownership identifies the semantic entity responsible for the current value/resource lifecycle.

§ 17(2) Every owned move-only value has exactly one current owner on every valid execution path.

§ 17(3) Ownership transfer uses copy/move rules and explicit consuming syntax where required.

§ 17(4) Ownership does not automatically imply backing-storage reclamation authority.

§ 17(5) Ownership may be partial at sub-Place granularity.

§ 17(6) Whole-value operations require whole-value availability where specialized rules require it.

---

## § 18 Copy and move

§ 18(1) Copy creates a distinct value according to the type's copy semantics.

§ 18(2) Move transfers ownership without requiring physical byte relocation.

§ 18(3) Ordinary by-value parameters are copy semantics unless declared consuming.

§ 18(4) A type must not silently upgrade ordinary by-value syntax into destructive consumption because it is move-only.

§ 18(5) Named reusable move-only sources use explicit `<-` at consuming boundaries according to `copy_move.md`.

§ 18(6) Return is an ownership-transfer boundary whose marker rules are owned by `copy_move.md`.

---

## § 19 Partial move

§ 19(1) A sub-Place may be moved independently when ownership/layout/destruction rules permit it.

§ 19(2) The containing aggregate becomes `PartiallyAvailable`.

§ 19(3) Available disjoint sub-Places remain usable.

§ 19(4) Whole-value operations requiring all subobjects remain invalid.

§ 19(5) Destruction cleans up only still-owned subobjects.

§ 19(6) Types with custom `free` forbid partial moves in Sec 0.1.

---

## § 20 Conditional ownership

§ 20(1) Control flow may produce `ConditionallyAvailable`.

§ 20(2) A subsequent availability test may refine a Place to Available/Unavailable on branches.

§ 20(3) `discard` may normalize conditional ownership to a definitely unavailable state according to ownership/destruction rules.

§ 20(4) Replacement/reinitialization may normalize conditional ownership where target/profile rules permit required conditional cleanup.

---

## § 21 Replacement and reinitialization

§ 21(1) Assignment to an Available mutable Place is replacement.

§ 21(2) Replacement performs required old-value cleanup before establishing the new value.

§ 21(3) Assignment to an Unavailable mutable Place is reinitialization.

§ 21(4) Reinitialization does not destroy an absent old value.

§ 21(5) The compiler validates the complete operation before destructively committing destination cleanup/ownership changes.

§ 21(6) Replacement and reinitialization remain distinct Semantic IR concepts even where lowering is similar.

---

## § 22 Borrowing

§ 22(1) `ref T` grants shared read authority.

§ 22(2) `ref mut T` grants exclusive mutable authority.

§ 22(3) Borrowing is non-owning.

§ 22(4) Borrow live ranges may end at final proven use rather than lexical scope end.

§ 22(5) Disjoint Places may support independent borrows.

§ 22(6) Move/discard/replacement/destruction is blocked by incompatible live borrows.

§ 22(7) Reborrowing does not consume a reference holder unless the operation's ownership semantics separately do so.

---

## § 23 References

§ 23(1) Safe references are non-null typed non-owning access values.

§ 23(2) Reference validity depends on provenance, lifetime, storage identity/epoch, borrow authority, bounds, initialization, target/address-space, and applicable platform rules.

§ 23(3) Safe references do not extend referent lifetime.

§ 23(4) Reference physical representation may vary by target/profile.

§ 23(5) Stale references must not become valid merely because a later object uses the same address.

§ 23(6) Runtime generation checks are one validity mechanism, not the complete memory model.

---

## § 24 Raw pointers

§ 24(1) `RawPtr[T]` is an unchecked raw address value.

§ 24(2) Raw pointers are non-owning and may be null/dangling.

§ 24(3) Copying/moving a raw pointer does not affect pointee ownership/lifetime.

§ 24(4) Raw dereference/conversion operations require unsafe according to `raw_pointers.md`.

§ 24(5) Raw pointer presence does not create a safe Place/reference/provenance fact.

§ 24(6) Known-invalid raw operations remain invalid inside unsafe.

---

## § 25 Lifetime

§ 25(1) Value/object, storage, reference, borrow, allocation-domain, mapping, callback, task, and external resource lifetimes are distinct but related.

§ 25(2) Safe uses must fit within every lifetime dependency.

§ 25(3) Lifetime analysis is path-sensitive and interprocedural where required.

§ 25(4) The compiler must not repair an illegal escape through hidden heap/Arena promotion.

§ 25(5) Returned-reference relationships are valid only when all possible origins outlive the returned use.

---

## § 26 Storage

§ 26(1) Storage classification follows `storage.md`.

§ 26(2) Canonical storage properties are orthogonal and include origin, backing relation, reclamation authority, address stability, memory space, regions, invalidation domain, and epoch.

§ 26(3) Storage identity is not numeric address.

§ 26(4) Object lifetime may change without ending storage lifetime.

§ 26(5) Physical relocation is distinct from ownership move.

---

## § 27 Allocation

§ 27(1) Allocation creates/provides storage according to an allocation-capable operation.

§ 27(2) Arena is the default Sec 0.1 dynamic-allocation model where available.

§ 27(3) Copy, move, borrow, parameter passing, return, reference creation, and lifetime repair do not silently allocate.

§ 27(4) Allocation failure uses typed recoverable error handling.

§ 27(5) Allocation domain is compiler-visible and resolved before backend lowering.

---

## § 28 Destruction

§ 28(1) Destruction ends owned value/resource lifecycle responsibility according to `destruction.md`.

§ 28(2) Automatic destruction and `defer` use canonical cleanup ordering where cleanup executes.

§ 28(3) Moved/discarded subobjects are not destroyed again.

§ 28(4) Custom `free` is a lifecycle operation and does not become an ordinary consuming method.

§ 28(5) Destruction is distinct from storage reclamation.

---

## § 29 Resources

§ 29(1) A resource may represent memory or an external entity such as a file, socket, mapping, task, thread, process, device handle, subscription, or foreign object.

§ 29(2) Resource ownership/lifecycle is modeled through ordinary ownership/destruction plus type-specific resource state.

§ 29(3) Resource state must not be conflated with Place availability.

§ 29(4) Resource operations may be fallible independently of memory validity.

§ 29(5) Hardware device liveness is a resource/platform state, not ownership `is available`.

---

## § 30 Addressability

§ 30(1) Addressability means a canonical operation may obtain an address/reference to materialized storage.

§ 30(2) Not every value is addressable.

§ 30(3) Addressability may require materialization of an otherwise SSA value where semantics permit it.

§ 30(4) Materialization for addressability is not dynamic allocation unless the canonical operation is allocation-capable.

§ 30(5) Fixed-address and MMIO Places have platform-constrained addressability.

---

## § 31 Relocation and address stability

§ 31(1) Physical relocation may occur only when live dependencies remain valid or are canonically updated/invalidated.

§ 31(2) A live direct reference may constrain relocation.

§ 31(3) Pinning is a storage/address-stability constraint, not ownership.

§ 31(4) Raw numeric address identity is not sufficient proof of storage identity across relocation/reuse.

---

## § 32 Layout

§ 32(1) Materialized objects use canonical plan-resolved layout from `layout.md`.

§ 32(2) Layout is separate from ownership/lifetime/storage origin.

§ 32(3) By-value storage requires complete layout.

§ 32(4) Padding is not semantic data.

§ 32(5) Representation validity is required before bytes become readable typed values.

---

## § 33 Temporaries and full expressions

§ 33(1) Compiler-created temporaries have semantic lifetimes determined by the owning expression/control-flow rules.

§ 33(2) A temporary must remain alive through every required use, borrow, call, conversion, and deferred cleanup dependency.

§ 33(3) The compiler may eliminate physical temporary storage when semantic lifetime/effects remain preserved.

§ 33(4) Temporary materialization must not silently extend an illegal reference escape.

§ 33(5) Full-expression cleanup ordering must integrate with defer/destruction where applicable.

---

## § 34 Function calls

§ 34(1) Function calls preserve parameter copy/move/borrow contracts.

§ 34(2) Caller/callee storage relationships are compiler-visible for returned references, borrowed parameters, hidden result storage, FFI retention, and allocation context propagation.

§ 34(3) Caller-provided result storage is not automatically ownership of callee locals.

§ 34(4) Separate compilation must preserve validated memory summaries needed for safe calls.

§ 34(5) Unknown bodyless/foreign behavior is conservative where proof is required.

---

## § 35 Closures

§ 35(1) Closures preserve capture mode and origin.

§ 35(2) Owned copy, owned move, shared borrow, mutable borrow, static capture, raw/unsafe capture, and foreign capability remain distinct.

§ 35(3) Escaping closure lifetime must cover every captured borrow/reference.

§ 35(4) Closure creation must not hidden-allocate merely to repair an illegal capture escape.

§ 35(5) Transferability is derived from actual captures and destination boundary.

---

## § 36 Interfaces and erased values

§ 36(1) Type erasure must not erase ownership, borrowing, lifetime, destruction, storage, or transfer constraints required for safe semantics.

§ 36(2) Interface representation may be owned or borrowed according to canonical interface rules.

§ 36(3) Dynamic dispatch summaries must conservatively preserve memory effects/requirements.

§ 36(4) Physical erasure/boxing must not silently allocate unless the canonical interface operation is allocation-capable.

---

## § 37 Generics

§ 37(1) Generic code is checked against semantic constraints and concrete specializations as required.

§ 37(2) Copy/destruction/layout/transfer/lifetime behavior may depend on concrete type arguments.

§ 37(3) Generic compilation must not assume all type parameters are trivially copyable/addressable/layout-complete.

§ 37(4) Specialized memory summaries must be deterministic and validation-aware across separate compilation.

---

## § 38 Volatile

§ 38(1) Volatile access preserves observable storage access required by hardware/platform contracts.

§ 38(2) Volatile does not change value ownership.

§ 38(3) A volatile read produces an ordinary snapshot value after the physical access.

§ 38(4) Copy/move of the snapshot does not perform another volatile access.

§ 38(5) Volatile is not synchronization, atomicity, DMA ownership, or hardware privilege.

---

## § 39 Atomics and synchronization

§ 39(1) Atomic operations and synchronization are defined by concurrency rules and target support.

§ 39(2) Race analysis uses canonical memory-location identity and Place overlap.

§ 39(3) Atomicity does not make unrelated non-atomic fields safe.

§ 39(4) Mutex/guard/channel ownership operations must compose with ordinary ownership/borrow/lifetime semantics.

§ 39(5) Interrupt masking is synchronization only when it excludes every relevant conflicting execution context.

---

## § 40 Concurrency and races

§ 40(1) A data race is a concurrency violation independent of ownership availability.

§ 40(2) Exclusive transfer and concurrent sharing are distinct.

§ 40(3) Thread/task/process/ISR transfer uses `transferability.md`.

§ 40(4) Storage generation checks do not substitute for synchronization or reclamation protection.

§ 40(5) Concurrency analysis must not invent a separate storage identity model.

---

## § 41 Memory spaces

§ 41(1) Memory spaces are storage/platform facts from `storage.md` and target rules.

§ 41(2) Ordinary, MMIO, and target-defined spaces may have distinct access, atomic, volatility, coherence, and transfer rules.

§ 41(3) Cross-space transfer must be explicit where semantics require material movement or representation change.

§ 41(4) A backend address-space cast is not permission for an implicit source-level cross-space transfer.

---

## § 42 Fixed-address and hardware memory

§ 42(1) Fixed-address bindings preserve canonical type/layout/storage/platform contracts.

§ 42(2) A fixed address does not imply permanent object lifetime or ownership.

§ 42(3) Register/MMIO access may have exact width/order/side-effect requirements.

§ 42(4) Scope exit must not invent hardware reset/register clear/device deallocation.

§ 42(5) Signal polarity remains application/driver semantics, not compiler memory semantics.

---

## § 43 Interrupt execution

§ 43(1) ISR execution uses the same ownership, borrowing, lifetime, references, raw pointers, storage, and memory-location model as ordinary code.

§ 43(2) Interrupt-specific noPanic/noAlloc/noBlock/bounded-work and synchronization rules are additional execution constraints.

§ 43(3) ISR-local stack/reference state must not escape beyond valid handler lifetime.

§ 43(4) Deferred ISR work uses explicit safe handoff/transfer mechanisms.

§ 43(5) Volatile access alone does not make ISR/shared-state access race-free.

---

## § 44 FFI

§ 44(1) FFI contracts must expose ownership, borrowing, retention, nullability, lifetime, allocation/deallocation, layout/ABI, callback, threading, and panic/abort behavior where relevant.

§ 44(2) `RawPtr[T]` is the canonical raw-address boundary when safe references cannot express the foreign contract.

§ 44(3) Foreign memory does not automatically become Sec-owned.

§ 44(4) Raw foreign representations require explicit proof/wrapping before becoming safe typed values/references.

§ 44(5) Unknown foreign contracts are conservative.

---

## § 45 Unsafe

§ 45(1) Unsafe permits operations whose complete proof obligations cannot be established automatically.

§ 45(2) Unsafe does not disable type, ownership, borrow, lifetime, effect, target, cleanup, or representation checking.

§ 45(3) Compiler-proven invalid operations are rejected even inside unsafe.

§ 45(4) Safe wrappers may encapsulate unsafe implementation operations when their public preconditions/postconditions establish safe semantics.

§ 45(5) Unsafe is not a general backend-UB switch.

---

## § 46 Semantic operations

§ 46(1) Semantic IR must represent or preserve distinctions equivalent to:

```text
CreateStorage
EstablishStorageDomain
AdvanceStorageEpoch
EndStorageDomain
Initialize
ConstructObject
Copy
Move
BorrowShared
BorrowMutable
Reborrow
Replace
Reinitialize
Discard
Destroy
EndObjectLifetime
Allocate
Reclaim
BindBackingStorage
RebindBackingStorage
RelocateStorage
ValidateReference
VolatileRead
VolatileWrite
AtomicOperation
CrossMemorySpace
TransferOwnership
```

§ 46(2) Exact operation names are implementation details.

§ 46(3) Operations may be folded/eliminated only after proof preserves semantics.

---

## § 47 Semantic IR metadata

§ 47(1) Values/operations record where applicable:

```text
semantic type
source/destination Place
Place relationship
storage origin/domain
allocation identity
memory space
resolved layout identity
size/alignment/bounds
initialization state
availability state and reason
ownership state
copy classification
reference provenance
borrow relation/live range
generation/epoch
relocation/address-stability facts
volatile/atomic status
resource state
destruction responsibility
transfer boundary
source location
target applicability
```

§ 47(2) Metadata may be absent when proven irrelevant but must not be guessed later.

---

## § 48 Semantic IR verification

§ 48(1) Verification before lowering must reject contradictory or impossible memory states.

§ 48(2) Verification includes, where relevant:

```text
read only initialized available values
no use after move/discard/destruction
no illegal whole-value use of partial availability
no invalid copy of move-only value
no conflicting live borrows
no invalid returned/escaping reference
no stale generation use
no reclaim with live required dependency
no destruction of moved subobject
no allocator mismatch
no invalid layout/alignment/bounds
no illegal volatile/MMIO lowering
no invalid cross-space transfer
no transferability violation
```

§ 48(3) Backend success is not a substitute for Semantic IR verification.

---

## § 49 Lowering

§ 49(1) Lowering may choose efficient physical representation only within preserved Sec semantics.

§ 49(2) SSA/register promotion is allowed where addressability/observable storage behavior is preserved.

§ 49(3) Stack/static placement may replace dynamic allocation only through valid allocation-elimination proof, never hidden escape repair.

§ 49(4) Reference checks/metadata may be eliminated after proof.

§ 49(5) Backend `noalias`, lifetime, dereferenceable, nonnull, and alignment metadata must never exceed Sec proof.

§ 49(6) MLIR/LLVM undefined behavior must not be introduced where Sec defines a checked/panic/error behavior.

---

## § 50 Diagnostics

§ 50(1) Memory diagnostics must follow the mentor-compiler principle.

§ 50(2) Diagnostics should identify the Place/value, prior state-changing operation, relevant source location, violated memory rule, and practical next action.

§ 50(3) User-facing diagnostics should prefer terms such as moved, destroyed, borrowed, no longer available, local lifetime ended, mapping ended, or value not initialized over internal lattice jargon.

§ 50(4) Related locations should identify move/discard/borrow/reset/remap/destruction/transfer origins.

§ 50(5) A compiler should distinguish unknown proof from proven invalidity.

---

## § 51 LSP and tooling

§ 51(1) LSP and `sec analyse` consume the same canonical memory facts as compilation.

§ 51(2) Tooling may expose ownership/availability, borrow live ranges, storage origin/domain, reference origin, lifetime relation, layout, allocation effect, destruction responsibility, transferability, volatile/atomic status, and target constraints.

§ 51(3) LSP must not re-parse/reimplement Sec memory semantics independently.

§ 51(4) Incremental invalidation must follow semantic dependencies, not only textual scope.

---

## § 52 Required test families

§ 52(1) Place tests include roots, fields, constant/dynamic indices, ranges, variants, dereferences, same/disjoint/containment/may-overlap relationships.

§ 52(2) State tests include initialization, move, discard, partial/conditional availability, replacement, reinitialization, and destruction.

§ 52(3) Borrow/reference tests include reborrow, final-use lifetime, returned references, stale epochs, raw-to-safe boundaries, and mapping lifetime.

§ 52(4) Storage/allocation tests include automatic/static/thread-local/Arena/allocator-backed domains, no hidden promotion, reset/reclaim, relocation, and pin/protection.

§ 52(5) Layout tests include complete by-value layout, alignment, representation validity, and target dependence.

§ 52(6) Concurrency/transfer tests include races, atomics, mutex/channel transfer, task migration, process boundaries, ISR handoff, and volatile-not-synchronization.

§ 52(7) FFI/unsafe tests include caller obligations, foreign retention, raw pointers, inline machine boundaries, and proven-invalid unsafe rejection.

§ 52(8) IR/lowering/tooling tests verify one canonical memory model through all maintained stages.

---

## § 53 Completion criteria

§ 53(1) Frontend memory support is complete when canonical Place/location/state/storage/reference/ownership facts cover every Sec 0.1 source form.

§ 53(2) Analysis support is complete when ownership, borrow, lifetime, storage, allocation, destruction, transferability, concurrency, FFI, and target analyses consume shared identities/facts rather than parallel approximations.

§ 53(3) Semantic IR support is complete when all required memory operations/state are explicit or equivalently proven and verifier-enforced.

§ 53(4) Lowering support is complete when maintained MLIR/backends preserve Sec memory semantics without inventing hidden allocation, ownership, alias, lifetime, or UB assumptions.

§ 53(5) Tooling support is complete when compiler, LSP, diagnostics, and analyses use the same canonical memory facts.

---

## § 54 Core summary

§ 54(1) The memory model separates values, objects, Places, storage, ownership, references, resources, and addresses.

§ 54(2) Place identity/relationships are shared across ownership, borrowing, destruction, concurrency, and transfer analysis.

§ 54(3) Availability and unavailability reason are separate dimensions.

§ 54(4) Move transfers ownership; relocation moves storage.

§ 54(5) Object lifetime and storage lifetime are distinct.

§ 54(6) Safe references depend on provenance/lifetime/borrow/storage validity; raw pointers do not provide those guarantees.

§ 54(7) Allocation is explicit in allocation-capable operations and must not be hidden escape repair.

§ 54(8) Destruction is distinct from reclamation.

§ 54(9) Volatile is not atomicity/synchronization.

§ 54(10) Unsafe transfers proof obligations without disabling Sec analysis.

§ 54(11) Sema establishes memory semantics, Semantic IR preserves/verifies them, and MLIR/backends implement them without redefining the language.
