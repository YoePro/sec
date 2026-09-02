# Reference Model

- Status: Normative
- Created: 2026-09-02
- Last updated: 2026-09-02
- Document revision: 2.0
- Sec language version: 0.1
- Canonical path: `rules/memory/reference_model.md`
- Replaces: previous revision of `rules/memory/reference_model.md`
- Repository baseline reviewed: `814a584` (latest publicly verifiable `main`; current `main` contents reviewed 2026-09-02)

---

## § 1 Purpose and authority

§ 1(1) This rulebook defines the underlying validity and representation model for Sec safe references and handle-like reference identities.

§ 1(2) It defines:

```text
safe-reference guarantees;
reference categories;
storage identity;
provenance;
spatial validity;
temporal validity;
type validity;
access authority;
nullability;
address-space compatibility;
relocation correctness;
validity epochs;
generation representation;
generation exhaustion;
stable handles;
weak handles;
profile-selected runtime representations;
reference equality;
stale-reference failure;
stale-handle resolution;
FFI/raw-pointer boundaries;
Semantic IR requirements;
lowering constraints;
diagnostics and tooling.
```

§ 1(3) `rules/memory/references.md` owns the source-facing semantics of `ref T`, `ref mut T`, reference creation, source forms, call behavior, returned references, and ordinary reference use.

§ 1(4) This rulebook owns the lower-level semantic guarantees and representation choices that make those source-facing rules safe.

§ 1(5) `borrowing.md` owns borrow authority and borrow live ranges.

§ 1(6) `lifetime_analysis.md` owns lifetime and escape proof.

§ 1(7) `storage.md` owns storage identity, storage domains, invalidation domains, epochs, relocation, pinning, backing storage, and memory spaces.

§ 1(8) `memory_model.md` owns the common Place/object/storage/value model.

§ 1(9) `raw_pointers.md` owns `RawPtr[T]` semantics.

§ 1(10) `unsafe.md` owns unsafe contexts and caller proof obligations for raw-to-safe conversion.

§ 1(11) `allocation.md`, `arena.md`, collections rulebooks, FFI rulebooks, concurrency rulebooks, and platform rulebooks own their respective invalidation/storage/retention/access contracts.

§ 1(12) This book incorporates the generational-reference model; no separate `generational_references.md` rulebook is required.

---

## § 2 Core principle

§ 2(1) Sec defines reference semantics independently of physical runtime representation.

§ 2(2) A compiler may represent a safe reference using:

```text
an address;
an address and length;
an address and expected epoch;
a slot identity and generation;
a capability pointer;
a target-specific tagged pointer;
a side-table key;
or another representation preserving every required semantic guarantee.
```

§ 2(3) A target/profile may change runtime cost, metadata shape, check placement, check elimination, address width, epoch width, hardware assistance, side-table use, or slot-table use.

§ 2(4) A profile must not silently weaken the source-level meaning of `ref`.

§ 2(5) In particular, a profile must not silently reinterpret `ref T` as a raw pointer.

§ 2(6) Where a safe-reference guarantee cannot be proven or dynamically preserved, the safe operation is rejected or an explicit raw/unsafe boundary is required.

---

## § 3 Non-goals

§ 3(1) The reference model does not introduce:

```text
visible lifetime parameters;
visible region parameters;
general linear types;
mandatory garbage collection;
mandatory reference counting;
mandatory handle tables;
mandatory generation metadata;
mandatory pin syntax;
nullable safe references;
implicit RawPtr conversion;
general pointer arithmetic on safe references.
```

§ 3(2) The compiler may use internal lifetime, region, epoch, domain, provenance, and handle identities without exposing them in Sec source.

§ 3(3) Stable handles and weak handles are semantic mechanisms whose final public source APIs are not fixed by Sec 0.1 through this book.

---

## § 4 Reference guarantee decomposition

§ 4(1) Safe-reference validity is composed from separate guarantees.

§ 4(2) At minimum the compiler must distinguish:

```text
temporal validity;
spatial validity;
storage identity;
provenance;
type validity;
initialization;
access authority;
nullability;
borrow compatibility;
relocation correctness;
address-space correctness;
concurrency correctness;
trust provenance where applicable.
```

§ 4(3) A single `IsSafePointer` boolean is insufficient as the canonical model.

§ 4(4) Satisfying one guarantee does not imply the others.

§ 4(5) A generation match alone does not establish a safe reference.

---

## § 5 Storage identity

§ 5(1) A storage identity identifies the semantic storage domain/incarnation referenced by a safe reference.

§ 5(2) Storage identity is not merely a numeric address.

§ 5(3) Reuse of the same numeric address for a later storage incarnation does not preserve the old identity.

§ 5(4) Safe references tied to an ended identity must not become valid for later storage at the same address.

§ 5(5) Storage identity is shared with the canonical storage/memory model and must not be reimplemented independently by reference analysis.

---

## § 6 Invalidation domain

§ 6(1) An invalidation domain is the smallest domain whose previous dependencies become invalid after one semantic invalidating event.

§ 6(2) Examples include:

```text
one allocation;
one Arena;
one collection backing store;
one reusable slot;
one registry domain;
one owner control block;
one runtime mapping.
```

§ 6(3) An invalidation domain may contain many objects or references.

§ 6(4) Invalidation may advance an epoch or end the domain.

---

## § 7 Validity epoch

§ 7(1) A validity epoch identifies one live incarnation of an invalidation domain.

§ 7(2) A generation is a common numeric representation of an epoch.

§ 7(3) An epoch belongs to an invalidation domain, not merely to an address.

§ 7(4) Physical epoch representation may be:

```text
counter;
compound identity;
randomized token;
domain identity plus slot generation;
hardware tag;
retired identity plus replacement.
```

§ 7(5) Source-level semantics depend on identity/incarnation validity, not on one required counter representation.

---

## § 8 Provenance

§ 8(1) Provenance is compiler-tracked evidence describing where a reference came from and which storage identity/location it is authorized to access.

§ 8(2) Provenance may be static, dynamic, target-assisted, trusted, or a combination.

§ 8(3) Safe derivation preserves or narrows provenance.

§ 8(4) Arbitrary `RawPtr` manipulation may lose compiler-proven provenance.

§ 8(5) Raw-to-safe conversion must re-establish sufficient provenance through a trusted owner/platform/unsafe contract.

§ 8(6) Numeric address equality is not provenance.

---

## § 9 Direct references

§ 9(1) A direct reference is a safe reference whose access resolves directly to the current storage address.

§ 9(2) A direct reference may additionally depend on:

```text
bounds;
expected epoch;
provenance;
address space;
borrow authority;
pin dependency;
mapping/domain identity.
```

§ 9(3) Direct references are appropriate for short-lived borrows, stable allocations, Arena allocations before invalidation, pinned values, addressed storage, fields, and elements when all guarantees are preserved.

§ 9(4) While a direct reference is live, physical relocation is forbidden unless the compiler proves every affected use remains correct through the transformation.

§ 9(5) A direct reference is not required to carry runtime metadata when static proof already establishes validity.

---

## § 10 Stable handles

§ 10(1) A stable handle is a long-lived identity that resolves through a stable slot or equivalent lookup mechanism.

§ 10(2) A stable handle may survive physical relocation of its target.

§ 10(3) Conceptually it contains or identifies:

```text
domain identity;
slot identity;
expected generation.
```

§ 10(4) Resolution obtains the current target address from the slot/table/target mechanism.

§ 10(5) Stable-handle identity is independent of current physical target address.

§ 10(6) A stable handle does not imply ownership unless its handle kind explicitly defines ownership/retention.

§ 10(7) Stable handles must not replace ordinary short-lived direct `ref` values by default.

§ 10(8) Stable handles may require metadata, indirection, synchronization, or slot tables only when the selected profile/API uses them.

§ 10(9) The exact source-level stable-handle API is not fixed by Sec 0.1 in this book.

---

## § 11 Weak handles

§ 11(1) A weak handle does not keep its target alive.

§ 11(2) Resolution of a weak handle is fallible ordinary program behavior.

§ 11(3) Conceptual resolution may return:

```text
Option[ref T]
Result[ref T, StaleReferenceError]
```

§ 11(4) The exact public API and error type are not fixed by this book.

§ 11(5) A missing/removed target must not cause panic merely because weak resolution failed.

§ 11(6) A separate asserting resolution API may panic if its explicit contract says so.

---

## § 12 `ref T`

§ 12(1) `ref T` is a safe, non-null, typed shared reference.

§ 12(2) A valid `ref T` guarantees:

```text
non-nullness;
correct alignment for T;
valid initialized T representation;
read authority;
live storage for the borrow live range;
correct storage identity/provenance;
spatial validity for T/subobjects;
address-space compatibility;
borrow compatibility;
relocation correctness.
```

§ 12(3) `ref T` is non-owning.

§ 12(4) `ref T` does not mean globally immutable storage.

§ 12(5) Mutation through another authority must still satisfy aliasing/concurrency rules.

§ 12(6) Physical representation is profile-selected.

---

## § 13 `ref mut T`

§ 13(1) `ref mut T` is a safe, non-null, typed mutable reference with exclusive mutable authority for its borrow live range.

§ 13(2) In addition to shared-reference guarantees it requires:

```text
write authority;
writable storage;
exclusive mutable access;
no conflicting shared or mutable borrow;
compatible synchronization;
compatible address-space rules.
```

§ 13(3) A runtime generation check does not replace exclusive-borrow analysis.

§ 13(4) Safe derivation cannot upgrade `ref T` into `ref mut T`.

---

## § 14 Slice references

§ 14(1) `ref T[]` is a safe bounded shared view over zero or more elements.

§ 14(2) A mutable slice uses the corresponding mutable-reference form defined by array/slice rules.

§ 14(3) A slice reference guarantees:

```text
one compatible storage identity/domain;
valid element type;
defined length;
valid bounded extent;
live storage for the borrow live range;
correct element alignment;
read or mutable authority as applicable;
borrow compatibility;
address-space compatibility.
```

§ 14(4) Permitted indices are `0..<length`.

§ 14(5) An empty slice is semantically valid.

§ 14(6) An empty slice may use a hidden null/sentinel physical base only when length is zero, the base is never dereferenced, no scalar safe reference is fabricated from it, and all operations respect zero length.

§ 14(7) This representation detail does not make safe references nullable.

---

## § 15 `RawPtr[T]`

§ 15(1) `RawPtr[T]` is not a safe-reference category.

§ 15(2) It may be null, dangling, misaligned, out of bounds, uninitialized, unowned, unbounded, provenance-unknown, or generation-untracked.

§ 15(3) Merely possessing/storing/copying/moving/comparing/passing a raw pointer does not by itself require unsafe where `raw_pointers.md` says otherwise.

§ 15(4) Interpreting raw storage as live typed safe storage requires the unsafe/raw-pointer proof boundary.

§ 15(5) Raw-pointer equality remains raw address equality and must not be reused as safe-reference equality.

---

## § 16 Addressed storage

§ 16(1) Platform-addressed storage may be exposed through target-aware declarations such as:

```sec
@address(Peripheral.GPIOA)
let mut GPIOA: GPIORegisters
```

§ 16(2) Addressed storage may be typed, target-bound, fixed/stable by contract, volatile by hardware rules, and outside allocator generation domains.

§ 16(3) Addressed storage normally does not require allocation generations merely because it has a fixed address.

§ 16(4) Reference validity still depends on platform/mapping/device/storage contracts.

§ 16(5) Fixed numeric address does not imply compiler ownership or permanent device liveness.

---

## § 17 Temporal validity

§ 17(1) Referenced storage/object state must remain valid for the complete reference use.

§ 17(2) Temporal validity protects against at least:

```text
use after object lifetime end;
use after destruction;
use after free;
use after Arena reset;
use after Arena release;
use after collection backing replacement;
use after slot removal/reuse;
use after mapping invalidation;
use after owner-domain invalidation.
```

§ 17(3) Temporal validity may be statically proven or dynamically checked according to the selected profile.

§ 17(4) Epoch/generation validation is one temporal-validity mechanism.

§ 17(5) A live numeric address is insufficient when the referenced storage incarnation has ended.

---

## § 18 Spatial validity

§ 18(1) Every safe-reference access must remain inside its authorized spatial extent.

§ 18(2) For `ref T`, the extent is the referenced `T` and valid subobjects derived from it.

§ 18(3) For slice references, the extent is the represented bounded element range.

§ 18(4) Temporal validity does not imply spatial validity.

§ 18(5) A live allocation can still be accessed out of bounds.

§ 18(6) Spatial narrowing through field/index/range derivation must be preserved.

---

## § 19 Type validity

§ 19(1) A safe reference guarantees a valid typed object/view.

§ 19(2) Type validity includes:

```text
required alignment;
initialized storage;
valid T representation;
valid discriminants/union state;
compatible access width;
address-space compatibility;
representation invariants.
```

§ 19(3) `RawPtr[T]` does not automatically establish type validity.

§ 19(4) Unsafe proof cannot make a compiler-proven invalid representation valid.

---

## § 20 Access authority

§ 20(1) Reference category determines ordinary access authority.

```text
ref T:
    read authority

ref mut T:
    exclusive read/write authority
```

§ 20(2) Additional operation-specific authority may exist for:

```text
volatile access;
atomic access;
execute access;
DMA ownership;
foreign retention;
MMIO privilege;
mapping access.
```

§ 20(3) These authorities are separate from numeric addressability.

§ 20(4) Possessing a safe reference does not grant unrelated device/platform authority.

---

## § 21 Nullability

§ 21(1) Safe scalar references are non-null.

§ 21(2) Optionality is explicit:

```sec
Option[ref T]
```

§ 21(3) `RawPtr[T]` may be null.

§ 21(4) Slice internal representation may use a hidden null/sentinel only under § 14(6).

§ 21(5) A raw-to-safe conversion must prove non-nullness for scalar references.

---

## § 22 Borrow compatibility

§ 22(1) Generation validity does not grant alias authority.

§ 22(2) Shared/mutable borrow compatibility is governed by `borrowing.md`.

§ 22(3) Reference validation must consume canonical borrow live-range facts.

§ 22(4) Field/range/index disjointness proven by the Place model may permit independent references.

§ 22(5) No dynamic generation mechanism may be used as a substitute for compile-time borrow exclusivity.

---

## § 23 Subreferences

§ 23(1) A safe reference derived from another safe reference may retain or narrow authority.

§ 23(2) A derived subreference has:

```text
the same compatible storage identity;
the same or shorter lifetime;
equal or narrower spatial bounds;
the same or weaker access authority;
compatible address space;
compatible epoch dependency.
```

§ 23(3) A shared reference cannot become mutable through safe derivation.

§ 23(4) A field/subobject reference cannot reconstruct unrestricted access to its container without an explicit valid boundary.

§ 23(5) Slice/range derivation narrows bounds and preserves compatible storage identity.

---

## § 24 Reference derivation provenance

§ 24(1) Reference derivation must retain a chain or equivalent proof sufficient to relate the child reference to its source storage/object.

§ 24(2) The compiler need not retain source-level syntax after canonical provenance facts are established.

§ 24(3) Derivation through field/index/range/deref/view mappings must preserve bounds and authority.

§ 24(4) A derived reference must not outlive any parent/source dependency required for validity.

---

## § 25 Relocation correctness

§ 25(1) A live direct reference must remain correct if storage relocates, or relocation must be forbidden.

§ 25(2) Generation matching alone does not solve relocation.

§ 25(3) If a direct reference stores/resolves to an old address and storage moves, that direct reference becomes invalid unless the compiler updates every use through a semantics-preserving transformation.

§ 25(4) Stable handles may survive relocation because they resolve current address indirectly.

§ 25(5) Relocation legality consumes canonical address-stability and pinning/storage facts.

---

## § 26 Pinning

§ 26(1) Pinning prevents physical relocation for a defined interval.

§ 26(2) Pinning may be required by:

```text
foreign retention;
DMA;
OS registration;
intrusive structures;
hardware descriptors;
self-referential layouts;
target-specific address contracts.
```

§ 26(3) Pinning does not by itself imply ownership, permanent lifetime, thread safety, copyability, allocation, or reference counting.

§ 26(4) An active pin dependency forbids incompatible physical relocation.

§ 26(5) The exact general source-level pin syntax is not fixed by this book.

§ 26(6) Specialized APIs may provide pin guards/contracts.

---

## § 27 Address spaces

§ 27(1) Every reference belongs to a compiler/target-recognized address space.

§ 27(2) Examples include:

```text
ordinary RAM;
flash;
MMIO;
DMA-visible storage;
secure memory;
non-secure memory;
device-local memory;
foreign address spaces.
```

§ 27(3) Numerically equal addresses in different address spaces are not automatically interchangeable.

§ 27(4) Safe conversion across spaces requires compiler-known semantic compatibility.

§ 27(5) Raw conversion requires an explicit unsafe boundary where the source rules permit it.

§ 27(6) A target may reject a safe-reference form in an address space whose guarantees cannot be preserved.

---

## § 28 Concurrency correctness

§ 28(1) Generation checking does not prevent data races.

§ 28(2) A reference used concurrently must satisfy ownership, sharing, mutability, synchronization, atomic, memory-ordering, reclamation, and ISR rules.

§ 28(3) A generation check followed by a load is insufficient when another execution context can invalidate/reclaim the object between check and use.

§ 28(4) Concurrent invalidation may require:

```text
ownership excluding invalidation;
lock;
atomic slot protocol;
critical section;
hazard reference/protection;
epoch-based reclamation;
reference counting;
pinning during access;
target-specific synchronization.
```

§ 28(5) Sec 0.1 does not require one universal concurrent reclamation mechanism.

§ 28(6) Generation width does not imply atomicity.

---

## § 29 Allocation generations

§ 29(1) An allocation generation identifies one live incarnation of one allocation identity where the selected mechanism uses generations.

§ 29(2) The generation changes or the allocation identity is retired when storage is reclaimed/reused/replaced in a way that could otherwise revive stale references.

§ 29(3) Generation metadata may reside in allocation metadata, side tables, owner control blocks, hardware tags, encoded pointers/handles, or elsewhere.

§ 29(4) No particular representation is mandatory.

§ 29(5) A provider may omit runtime generation metadata when static proof already prevents stale safe-reference use.

---

## § 30 Arena epochs

§ 30(1) An Arena may use one shared owner-wide validity epoch.

§ 30(2) References requiring runtime validation retain the expected Arena epoch.

§ 30(3) Ordinary allocation does not advance the Arena epoch.

§ 30(4) Arena reset advances/replaces the current epoch while preserving the Arena domain as specified by `arena.md`.

§ 30(5) Reset invalidates prior Arena allocations and dependent references.

§ 30(6) Arena release ends the Arena domain and invalidates all dependent references.

§ 30(7) One Arena epoch may cover many allocations; one generation field per allocation is not required.

---

## § 31 Collection storage epochs

§ 31(1) A collection may use one epoch for its backing-storage invalidation domain.

§ 31(2) The epoch may change when backing storage is:

```text
reallocated;
replaced;
structurally rebuilt;
compacted with direct-reference invalidation.
```

§ 31(3) Element mutation preserving storage identity/reference validity does not necessarily advance the epoch.

§ 31(4) Collection mutation rules remain coordinated with borrowing, iterator/view, freeze, and transfer semantics.

---

## § 32 Slot generations

§ 32(1) A reusable slot may use a slot generation to distinguish successive occupants.

§ 32(2) A stable handle retains domain identity, slot identity, and expected generation.

§ 32(3) Slot reuse must advance/replace the generation or otherwise create a distinguishable new identity.

§ 32(4) An old handle must never resolve to the new occupant solely because the slot number was reused.

---

## § 33 Default logical epoch width

§ 33(1) The default logical validity-epoch width is 64 bits.

§ 33(2) This default also applies to 32-bit general-purpose targets.

§ 33(3) Pointer width and epoch width are independent.

§ 33(4) A 64-bit logical epoch may use multiple machine words on a narrower target.

§ 33(5) Runtime representation may be optimized or omitted when proof permits.

---

## § 34 Compile-time-selected shorter widths

§ 34(1) A target/build profile may select a shorter epoch/generation width only at compile time.

§ 34(2) Epoch width is never dynamically selected as ordinary runtime program state.

§ 34(3) A shorter width is valid only when stale-reference safety is preserved through one or more of:

```text
statically bounded invalidation count;
slot retirement;
domain retirement;
explicit exhaustion handling;
proof stale references cannot survive reuse;
equivalent target-specific guarantee.
```

§ 34(4) The profile must reject unsafe reuse patterns it cannot prove/check.

§ 34(5) Shorter width is a representation policy, not weaker reference semantics.

---

## § 35 Wider widths

§ 35(1) Future hardened/target profiles may use wider logical epochs such as 128 bits.

§ 35(2) Wider width does not alter source-level reference semantics.

§ 35(3) Epoch width is not exposed as ordinary mutable program state through this model.

---

## § 36 Constrained bare-metal profiles

§ 36(1) Constrained profiles should omit runtime reference metadata whenever all required validity is statically proven.

§ 36(2) Common examples include:

```text
bounded lexical borrows;
fixed-address storage;
fixed arrays;
non-relocating statically allocated values;
Arena references proven not to cross reset/release.
```

§ 36(3) A constrained profile may use compact compile-time-selected generations where required safety/exhaustion behavior exists.

§ 36(4) When validity cannot be proven or checked, the compiler rejects the safe operation or requires a raw/unsafe form.

§ 36(5) A constrained profile must not silently weaken safe-reference guarantees.

---

## § 37 Generation increment

§ 37(1) Numeric generation/epoch increments use checked arithmetic.

§ 37(2) Generations must never silently wrap.

§ 37(3) A generation or epoch must never wrap into a value that can make an older stale reference valid again.

§ 37(4) If the physical mechanism is non-numeric, it must provide the equivalent non-revival guarantee.

---

## § 38 Generation exhaustion

§ 38(1) Finite generation spaces may exhaust.

§ 38(2) Exhaustion may exhaust/retire an identity domain.

§ 38(3) Exhaustion must never revive stale references.

§ 38(4) Valid exhaustion strategies include:

```text
retire slot permanently;
retire allocation identity;
replace Arena/domain with fresh independent identity;
rekey domain preserving old-domain distinction;
return explicit exhaustion error when API is fallible;
deterministic panic or target trap when recovery is unavailable.
```

§ 38(5) Simple wraparound to zero is forbidden when any old matching identity may still exist.

§ 38(6) The owning API/rulebook determines whether exhaustion is fallible, panic-producing, or prevented by proof/retirement.

---

## § 39 Slot retirement

§ 39(1) A reusable slot whose generation space exhausts may be permanently retired.

§ 39(2) A retired slot must not receive a new live occupant under the exhausted identity.

§ 39(3) New allocations may use another slot/identity.

§ 39(4) Slot retirement permits compact generations where the resulting capacity/resource policy is acceptable.

---

## § 40 Arena epoch exhaustion

§ 40(1) Arena epoch exhaustion may be handled by:

```text
replace complete Arena identity domain;
recreate Arena with fresh independent identity;
return reset failure where API is fallible;
deterministic panic or target trap otherwise.
```

§ 40(2) A replacement domain must remain distinguishable from every prior live/stale represented domain identity.

§ 40(3) The Arena API/rulebook owns the programmer-visible exhaustion behavior.

---

## § 41 Atomicity and generation representation

§ 41(1) Generation width does not imply atomicity.

§ 41(2) A 64-bit epoch may not be atomically readable/writable on a 32-bit target.

§ 41(3) Concurrent invalidation/resolution requires a safe synchronized protocol.

§ 41(4) Valid mechanisms may include target atomics, locks, critical sections, versioned-slot protocols, compact atomic generation plus retirement, or ownership excluding concurrent invalidation.

§ 41(5) A torn unsynchronized epoch protocol is invalid.

---

## § 42 Initialization requirements

§ 42(1) A safe typed reference may only observe initialized valid typed storage.

§ 42(2) Safe typed allocation returns a fully initialized valid value according to the allocation/default/constructor/type rules.

§ 42(3) This does not require redundant physical zeroing.

§ 42(4) The compiler may directly initialize semantic fields and omit dead/redundant writes.

§ 42(5) Uninitialized typed storage is not readable through a safe reference.

§ 42(6) Internal/raw uninitialized storage remains outside safe reference access until initialization validity is established.

§ 42(7) Sec 0.1 does not require a public `Uninit[T]` type through this rulebook.

---

## § 43 Check placement

§ 43(1) Dynamic validity checks are inserted only when required by profile and proof state.

§ 43(2) Checks may include:

```text
epoch comparison;
slot live-state validation;
bounds check;
address-space check;
type/tag check;
hardware-capability validation;
mapping/domain validation.
```

§ 43(3) Statically proven guarantees should remove corresponding runtime checks.

§ 43(4) Eliminating one check must not remove unrelated guarantees.

§ 43(5) Check placement must preserve concurrency/reclamation correctness; a check must not be moved outside the interval where its proof remains valid.

---

## § 44 Ordinary safe references should not become stale

§ 44(1) Safe Sec code should normally prevent creation/use of stale ordinary `ref` values.

§ 44(2) Static prevention uses ownership, borrowing, lifetime/escape analysis, Arena analysis, relocation restrictions, collection mutation rules, call/effect analysis, storage facts, and target knowledge.

§ 44(3) Runtime validation of ordinary `ref` values is primarily a hardening/dynamic-model/foreign-trust/target-safety mechanism rather than normal business control flow.

§ 44(4) The programmer is not required to `try` every ordinary safe-reference access.

---

## § 45 Stale ordinary-reference failure

§ 45(1) A stale ordinary safe reference represents violation of a safe-reference guarantee.

§ 45(2) Use/dereference of such a stale ordinary safe reference results in deterministic panic or target trap according to the active profile.

§ 45(3) This is not normal recoverable business failure.

§ 45(4) `try` does not catch this panic under Sec 0.1 panic semantics.

§ 45(5) A profile may statically prove the stale path impossible and remove the check.

---

## § 46 Stale stable/weak-handle resolution

§ 46(1) Resolution failure of a stable/weak handle whose target was removed/replaced is normal fallible behavior.

§ 46(2) Canonical handle APIs return explicit optional/error results for fallible resolution.

§ 46(3) They do not panic merely because the target is no longer live.

§ 46(4) An explicitly asserting handle-resolution operation may panic by contract.

§ 46(5) This distinction between ordinary `ref` failure and handle resolution failure is normative.

---

## § 47 Safe-reference equality

§ 47(1) Safe-reference equality means:

```text
same live storage identity;
and same referenced semantic location within that storage.
```

§ 47(2) Numeric address equality alone is insufficient.

§ 47(3) References to the same live field/element/location may compare equal.

§ 47(4) References to different live incarnations at the same reused address are not equal.

§ 47(5) Equality must respect address-space identity where relevant.

---

## § 48 Stable-handle equality

§ 48(1) Stable-handle equality means:

```text
same domain identity;
same slot identity;
same generation/epoch identity.
```

§ 48(2) A stale handle and a later handle to a new occupant are not equal even if the slot number and physical address are reused.

§ 48(3) Equality does not imply the target is currently resolvable unless the handle API explicitly guarantees liveness.

---

## § 49 Raw-pointer equality

§ 49(1) `RawPtr[T]` equality uses target raw-address equality according to raw-pointer/address-space rules.

§ 49(2) It does not imply:

```text
same live allocation;
same storage identity;
same provenance;
same generation;
safe dereference;
same owner;
same address space unless established by the pointer types/target.
```

§ 49(3) Raw equality must not be substituted for safe-reference equality.

---

## § 50 Safe reference to RawPtr

§ 50(1) Conversion from a safe reference to `RawPtr[T]` may be permitted through an explicit unsafe/FFI/platform operation.

§ 50(2) The conversion/use must preserve or verify where applicable:

```text
source borrow remains live;
foreign use is call-bounded or declared retained;
mutability compatibility;
address-space/ABI compatibility;
no invalid relocation during use;
pinning requirements;
mapping/storage lifetime.
```

§ 50(3) The resulting raw pointer is raw and must not be assumed to preserve safe-reference guarantees after arbitrary manipulation.

§ 50(4) Exact source spelling belongs to the canonical conversion/raw-pointer rules.

---

## § 51 RawPtr to safe reference

§ 51(1) Converting raw storage to `ref T`, `ref mut T`, or a safe slice/view is unsafe unless a specialized trusted adapter proves the full contract.

§ 51(2) Required proof includes:

```text
non-nullness where required;
alignment;
valid initialized representation;
bounds;
lifetime;
ownership compatibility;
borrow/alias compatibility;
address-space compatibility;
read/write authority;
foreign retention compatibility;
relocation safety;
storage identity/provenance;
mapping/domain validity.
```

§ 51(3) Generation/provenance may be established through a trusted storage owner/platform mechanism.

§ 51(4) Known contradictory facts cause compile-time rejection even inside unsafe.

---

## § 52 Foreign retention

§ 52(1) FFI declarations/contracts must distinguish call-bounded from retained pointer use.

§ 52(2) Non-retained pointer use may be bounded by the foreign call.

§ 52(3) Retained or unknown retention requires sufficient lifetime, ownership, pinning/relocation, synchronization, and foreign effect proof.

§ 52(4) Unknown retention is conservative.

§ 52(5) Exact FFI source syntax belongs to the FFI rulebook.

---

## § 53 Serialization and persistence

§ 53(1) Safe references and stable/weak handles are not automatically serializable or persistent.

§ 53(2) Their identities are normally scoped to domains such as:

```text
one process;
one allocator/provider instance;
one registry;
one Arena;
one mapping/device lifetime;
one runtime domain.
```

§ 53(3) Persistent identity is a domain/application-specific ID rather than an ordinary memory reference.

§ 53(4) Serialization must use explicit persistent identity/representation semantics.

§ 53(5) Process transfer follows transferability/IPC rules rather than serializing in-process reference bits.

---

## § 54 Language move versus relocation

§ 54(1) Ownership move and physical storage relocation are distinct.

§ 54(2) Reference analysis must distinguish:

```text
ownership transfer;
logical move;
physical address change;
slot relocation;
copy;
pinning.
```

§ 54(3) A move preserving effective storage address may preserve direct references when ownership/borrow/lifetime rules permit it.

§ 54(4) A move causing physical relocation invalidates direct references unless the compiler proves/upgrades all uses correctly or the reference kind is relocation-stable.

§ 54(5) `copy_move.md` remains authoritative for value ownership semantics.

---

## § 55 Collection relocation

§ 55(1) Collection operations must declare/derive whether they preserve element addresses and backing identity.

§ 55(2) Operations that may relocate backing storage must:

```text
be forbidden while conflicting direct references are live;
or invalidate the relevant storage epoch;
or use stable indirection;
or be proven not to affect the referenced element.
```

§ 55(3) Element mutation preserving storage identity remains subject to ordinary borrowing/concurrency rules.

§ 55(4) Collection-specific rules determine which structural operations invalidate which references/views/iterators.

---

## § 56 Arena integration

§ 56(1) Arena-backed reference validity depends on:

```text
Arena domain identity;
current Arena epoch where needed;
allocation/object extent;
borrow live range;
escape restrictions;
mapping/address-space facts where applicable.
```

§ 56(2) Arena reset and release are invalidating events.

§ 56(3) NLL/final-use analysis may permit reset after the final use of all dependent borrows even when source variables remain lexically in scope.

§ 56(4) `arena.md` owns source API; this book owns reference-validity consequences.

---

## § 57 Allocation integration

§ 57(1) Dynamic allocation does not require generations when static reference safety already prevents stale use.

§ 57(2) Allocator-backed dynamic lifetime models may use allocation generations/control blocks/side tables as profile-selected mechanisms.

§ 57(3) Allocation identity and reference epoch are distinct facts even when represented together.

§ 57(4) Hidden allocation must not be introduced solely to obtain a convenient reference representation.

---

## § 58 Storage integration

§ 58(1) The reference model consumes canonical `StorageOrigin`, `AddressStability`, `MemorySpaceKind`, storage-domain identity, invalidation-domain identity, validity epoch, mapping lifetime, and backing relations from `storage.md`.

§ 58(2) Reference analysis must not infer these from lexical variable names or raw addresses.

§ 58(3) Storage-domain end or epoch advance invalidates dependent references according to their dependency.

§ 58(4) Physical storage reuse after end does not preserve old reference identity.

---

## § 59 Borrowing integration

§ 59(1) Reference validity consumes canonical borrow live ranges and Place-overlap relationships.

§ 59(2) NLL/final-use shortening may remove relocation/reset/transfer conflicts after the final use.

§ 59(3) Runtime reference metadata does not replace borrow checking.

§ 59(4) Reborrow relationships must preserve origin/provenance and authority.

---

## § 60 Lifetime-analysis integration

§ 60(1) The reference model consumes canonical origin/lifetime dependency sets from `lifetime_analysis.md`.

§ 60(2) Returned references may have multiple possible origins when every origin satisfies the required return lifetime.

§ 60(3) Multiple finite origins do not by themselves make a returned reference invalid.

§ 60(4) Dynamic representation may encode whichever origin/domain facts are required by the selected profile.

§ 60(5) Lifetime analysis must not be duplicated by the reference model.

---

## § 61 Transferability integration

§ 61(1) Reference transfer across tasks/threads/processes/ISR/foreign callbacks is governed by `transferability.md`.

§ 61(2) A reference may be lifetime-valid yet non-transferable due to thread affinity, process locality, address space, mapping, synchronization, destruction, or execution policy.

§ 61(3) Ordinary safe references do not become valid process references merely by copying their representation.

§ 61(4) Stable-handle transferability depends on handle-domain/thread/process contracts.

---

## § 62 Effect analysis

§ 62(1) Reference validity and effects are separate semantic dimensions.

§ 62(2) Examples:

```text
generation comparison alone does not imply allocation/blocking;
slot-table growth may allocate;
concurrent handle resolution may block if implemented with a lock;
foreign conversion may cross unsafe/trust boundaries;
addressed storage access may be volatile;
handle resolution may read shared metadata.
```

§ 62(3) Reference operations publish their actual effects.

§ 62(4) Profile-selected implementation must satisfy effect contracts such as `noAlloc`, `noBlock`, and ISR restrictions.

---

## § 63 Unsafe integration

§ 63(1) Unsafe does not disable the reference model.

§ 63(2) Unsafe may accept specific obligations concerning raw pointer validity, lifetime, alignment, provenance, foreign retention, ownership, and address-space assumptions.

§ 63(3) Only explicitly accepted obligations become trusted rather than compiler-proven.

§ 63(4) Unrelated rules remain active.

§ 63(5) Compiler-proven contradictions remain errors.

---

## § 64 No mandatory runtime

§ 64(1) The reference model introduces no mandatory:

```text
garbage collector;
reference counting;
global allocation table;
global handle table;
scheduler;
generation manager;
tracing collector;
runtime exception system.
```

§ 64(2) Profiles may use metadata/helpers when required.

§ 64(3) Proven direct references may lower to plain machine addresses.

§ 64(4) Programs not using dynamic handle/generation mechanisms need not link them.

---

## § 65 Hosted optimized profile

§ 65(1) A hosted optimized profile may typically lower:

```text
short-lived ref T:
    address only

slice:
    address + length

stable handle:
    slot + generation

Arena reference needing dynamic hardening:
    address + expected Arena epoch
```

§ 65(2) This is illustrative profile policy, not mandatory layout.

§ 65(3) Static proof may remove metadata/checks.

---

## § 66 Hosted hardened profile

§ 66(1) A hardened profile may use:

```text
address + epoch;
side-table validation;
hardware memory tags;
capability bounds;
additional provenance checks.
```

§ 66(2) Additional hardening must preserve ordinary source semantics and effects.

§ 66(3) Hardened metadata must not become visible as ordinary mutable program state unless another explicit API exposes it.

---

## § 67 32-bit general-purpose profile

§ 67(1) A typical 32-bit general-purpose policy may use:

```text
32-bit address;
64-bit logical owner/domain epoch by default;
compile-time-selected compact slot generation where retirement is supported;
no runtime generation on fully proven references.
```

§ 67(2) A 32-bit target must not truncate the logical epoch merely for representation convenience.

§ 67(3) Concurrent use still requires an atomic/synchronized protocol appropriate to the target.

---

## § 68 Constrained bare-metal profile

§ 68(1) A constrained bare-metal policy may use:

```text
address-only proven references;
no generation for fixed MMIO bindings;
no generation for bounded lexical borrows;
compact generations only where proof/exhaustion policy permits;
rejection of dynamic safe-reference patterns that cannot be proven or checked.
```

§ 68(2) Such a profile may remain runtime-free.

§ 68(3) Target constraints do not weaken safe-reference semantics.

---

## § 69 Reference-analysis domains

§ 69(1) The compiler should keep separate analysis facts for:

```text
lifetime validity;
spatial bounds;
provenance;
initialization;
type validity;
borrow compatibility;
relocation;
pinning;
address space;
validity epoch;
concurrency/reclamation protocol;
trust provenance;
handle domain/slot identity;
mapping/platform validity.
```

§ 69(2) These facts may share underlying canonical storage/Place identities.

§ 69(3) A compact summary may exist only if it can be expanded/traced to the required underlying guarantees for diagnostics and verification.

---

## § 70 Semantic IR requirements

§ 70(1) Semantic IR must preserve enough information to implement all required reference semantics.

§ 70(2) It must be able to represent:

```text
reference category;
source storage identity;
invalidation-domain identity;
address space;
spatial extent;
borrow kind/live range;
access authority;
validity-epoch dependency;
slot identity;
handle resolution;
reference derivation;
bounds narrowing;
physical relocation;
pinning dependency;
unsafe raw conversion;
foreign retention;
mapping/platform dependency;
invalidation event;
stale failure behavior.
```

§ 70(3) These may be nodes, attributes, references to canonical side tables, or equivalent facts.

§ 70(4) Semantic IR must distinguish direct safe references from stable/weak handles and from `RawPtr`.

§ 70(5) Semantic IR verification must reject contradictory reference guarantees.

---

## § 71 Canonical semantic operations

§ 71(1) Semantic IR must support distinctions equivalent to:

```text
CreateDirectReference
CreateSliceReference
DeriveSubreference
NarrowReferenceBounds
ValidateReference
ValidateEpoch
ResolveStableHandle
ResolveWeakHandle
AcquirePinDependency
ReleasePinDependency
RecordInvalidation
RecordRelocation
ConvertSafeReferenceToRaw
ConvertRawToSafeReference
EndReferenceDependency
```

§ 71(2) Exact internal operation names are implementation details.

§ 71(3) A backend load/store is not an adequate substitute for the semantic operation before its proof obligations have been resolved.

---

## § 72 Semantic IR invalidation facts

§ 72(1) Invalidating events must be explicit or canonically represented where they affect reference validity.

§ 72(2) Events include allocation end, Arena reset/release, collection backing replacement, slot reuse/removal, mapping remap/unmap, and domain retirement.

§ 72(3) The verifier must be able to relate a reference dependency to an invalidating event.

§ 72(4) Static proof may eliminate runtime invalidation metadata after the semantic relationship is verified.

---

## § 73 Lowering

§ 73(1) Lowering consumes Semantic-IR/Sema reference facts.

§ 73(2) It may choose address-only representation only when all omitted guarantees are proven elsewhere.

§ 73(3) Runtime epoch/slot/provenance metadata must be preserved where required.

§ 73(4) Backend `nonnull`, `dereferenceable`, `noalias`, alignment, lifetime, or address-space metadata must not exceed proven Sec guarantees.

§ 73(5) A stale-reference panic/trap must remain defined Sec behavior where the check remains dynamic.

§ 73(6) A stale-handle resolution must remain fallible rather than be lowered to UB or unconditional trap.

§ 73(7) Address-space/capability semantics must be preserved.

§ 73(8) Lowering must not introduce a global handle table/generation manager unless the selected representation actually requires one.

---

## § 74 Check elimination

§ 74(1) The compiler should eliminate runtime reference checks when equivalent safety is statically proven.

§ 74(2) Check elimination must be guarantee-specific.

§ 74(3) Proving lifetime does not automatically prove bounds.

§ 74(4) Proving generation does not automatically prove borrow exclusivity.

§ 74(5) Proving address-space compatibility does not automatically prove type validity.

§ 74(6) Profile hardening checks may remain even where language-level proof would otherwise suffice if the profile contract intentionally retains them.

---

## § 75 Optimization and relocation

§ 75(1) Optimization may promote values to SSA/registers, relocate storage, elide checks, merge identities, or remove metadata only when every observable reference guarantee is preserved.

§ 75(2) Optimizations must not cause a direct reference to silently follow relocated storage unless an equivalent reference transformation is proven.

§ 75(3) Stable-handle indirection may enable relocation where direct references would forbid it.

§ 75(4) Address-observable unsafe/FFI/hardware operations constrain optimization according to their contracts.

---

## § 76 Panic and stale references

§ 76(1) Dynamic stale ordinary-reference detection uses the panic semantics defined by `panic.md`.

§ 76(2) The stable panic reason should distinguish invalid/stale reference generation or equivalent canonical cause.

§ 76(3) The minimum stale-reference failure path must respect panic profile/runtime-free requirements.

§ 76(4) Handle-resolution failure is not automatically a panic source.

---

## § 77 Diagnostics

§ 77(1) Reference diagnostics must follow the mentor-compiler principle.

§ 77(2) Diagnostics should show:

```text
reference creation/origin;
reference category;
storage/domain identity where meaningful;
borrow/lifetime dependency;
invalidating event;
invalid use;
relocation/pin issue;
selected profile where relevant;
selected epoch/generation policy where relevant;
trusted boundary;
safe alternative.
```

§ 77(3) Diagnostics should distinguish:

```text
lifetime ended;
storage/domain invalidated;
out of bounds;
wrong provenance;
invalid representation;
borrow conflict;
relocation conflict;
address-space mismatch;
stale ordinary ref;
stale handle resolution;
unknown proof.
```

§ 77(4) Diagnostics must not tell users to add `unsafe` when the compiler already proves the operation invalid.

---

## § 78 Informational diagnostics

§ 78(1) Compiler/LSP may report informational facts such as:

```text
reference validity fully proven; runtime epoch check removed;
stable handle uses compact slot generation with retirement;
Arena uses one shared epoch for all Arena allocations;
reference lowered as address-only under this profile.
```

§ 78(2) Such diagnostics are optional/configurable.

§ 78(3) Informational output must not be required for semantic correctness.

---

## § 79 LSP and tooling

§ 79(1) LSP and `sec analyse` consume the same canonical reference facts as compilation.

§ 79(2) Tooling may expose:

```text
reference category;
bounds;
origin/provenance;
borrow status/live range;
storage identity/domain;
relocation stability;
pin dependency;
epoch dependency;
handle domain/slot identity;
invalidation source;
profile representation;
trust provenance;
address space.
```

§ 79(3) Tooling must not reconstruct reference semantics independently from syntax.

§ 79(4) Hover should distinguish source-level guarantee from selected runtime representation.

§ 79(5) Incremental invalidation must account for ownership/borrow/lifetime/storage/layout/profile/FFI/platform/concurrency changes.

---

## § 80 Required basic tests

§ 80(1) Required tests include:

```text
ref T is non-null;
Option[ref T] optionality;
ref mut exclusivity;
shared borrow compatibility;
field-split borrows;
subreference narrowing;
slice bounds;
empty slice representation does not make ref nullable;
safe-reference equality;
RawPtr equality distinction.
```

---

## § 81 Temporal/provenance tests

§ 81(1) Required tests include:

```text
use after owner destruction rejected;
use after free rejected;
use after Arena reset rejected;
use after Arena release rejected;
use after collection backing replacement rejected;
use after slot reuse rejected;
mapping remap invalidates dependent reference;
same numeric address/new identity does not revive stale ref;
last-use borrow ending before reset accepted;
static proof removes dynamic validity check.
```

---

## § 82 Generation tests

§ 82(1) Required tests include:

```text
64-bit logical default epoch on 32-bit general-purpose profile;
no epoch metadata for fully proven ref;
shorter compile-time-selected generation with bounded proof;
runtime-selected generation width rejected;
checked increment;
no wrap;
slot retirement at exhaustion;
Arena domain replacement;
explicit exhaustion error;
deterministic panic/trap where recovery unavailable;
concurrent generation protocol cannot tear.
```

---

## § 83 Stable/weak handle tests

§ 83(1) Required tests include:

```text
relocation preserves stable handle;
slot reuse invalidates old handle;
weak/stable resolution returns no target fallibly;
handle equality includes domain/slot/generation;
handle does not imply ownership;
owning handle retains target only when explicit;
concurrent resolution requires valid reclamation/synchronization protocol.
```

---

## § 84 Initialization tests

§ 84(1) Required tests include:

```text
safe typed allocation yields initialized valid T;
semantic default initialization need not zero all bytes;
compiler may omit redundant whole-buffer zeroing;
safe ref cannot read uninitialized storage;
raw uninitialized storage requires unsafe initialization workflow.
```

---

## § 85 FFI/raw tests

§ 85(1) Required tests include:

```text
safe reference passed for call-bounded foreign use;
foreign retention requires declared lifetime/pinning support;
mutable foreign use requires compatible borrow;
RawPtr to ref requires unsafe/trusted proof;
foreign raw return does not automatically gain provenance;
pinning required for retained direct address when relocation possible;
address-space mismatch rejected;
known-invalid raw-to-safe conversion rejected inside unsafe.
```

---

## § 86 Profile/binary tests

§ 86(1) Required tests include:

```text
proven ref lowers to address only;
proven slice lowers to address plus length where selected;
unused epoch support omitted;
no global generation manager linked;
32-bit target may retain 64-bit logical epoch;
constrained profile omits metadata when proven;
compact generation width is compile-time constant;
hardware-capability lowering preserves guarantees.
```

---

## § 87 Concurrency tests

§ 87(1) Required tests include:

```text
generation check alone does not make racing invalidation safe;
lock/atomic/critical-section protocol preserves handle resolution;
torn multiword epoch protocol rejected;
reference transfer respects thread/task affinity;
ISR references obey interrupt synchronization/lifetime rules;
volatile access is not synchronization.
```

---

## § 88 Semantic IR/lowering tests

§ 88(1) Required tests include:

```text
direct ref distinct from stable handle, weak handle and RawPtr;
reference origin/storage identity preserved;
subreference narrowing preserved;
epoch dependency preserved where needed;
invalidating event visible to verifier;
stale ordinary ref lowers to defined panic/trap;
stale handle remains fallible;
backend metadata no stronger than proven Sec facts;
static proof removes runtime reference metadata/checks.
```

---

## § 89 Completion criteria

§ 89(1) Source/reference integration is complete when `references.md` source forms map to one canonical reference-validity model.

§ 89(2) Borrow/lifetime integration is complete when reference validity consumes canonical Place, borrow-live-range, origin, and escape facts without duplicating those analyses.

§ 89(3) Storage integration is complete when storage identity, invalidation domain, epoch, relocation, pinning, and mapping facts use the canonical storage model.

§ 89(4) Generation support is complete when default/compact widths, checked increment, exhaustion, retirement, and no-stale-revival semantics are implemented across relevant domains.

§ 89(5) Stable/weak handles are complete when identity, relocation, fallible resolution, equality, ownership distinction, concurrency, and profile representation are all implemented.

§ 89(6) FFI/raw support is complete when retention, pinning, address spaces, raw provenance loss, and trusted reconstruction participate in canonical analysis.

§ 89(7) Semantic IR/lowering is complete when every surviving reference guarantee and invalidation/handle operation is explicit or equivalently proven.

§ 89(8) Tooling is complete when compiler, LSP, diagnostics, and `sec analyse` consume the same reference facts.

§ 89(9) This rulebook must not be marked implemented merely because generation checks exist.

---

## § 90 Core summary

§ 90(1) Sec safe-reference guarantees are defined independently of physical representation.

§ 90(2) Safe references are non-null typed non-owning access values whose validity combines lifetime, bounds, provenance, initialization/type validity, authority, borrow compatibility, relocation, address space, and concurrency correctness.

§ 90(3) A validity epoch identifies one live incarnation of an invalidation domain; generation is one possible representation.

§ 90(4) The default logical epoch width is 64 bits, including on 32-bit general-purpose targets.

§ 90(5) Shorter widths are compile-time profile decisions permitted only with proof/retirement/exhaustion semantics preserving stale-reference safety.

§ 90(6) Generations never silently wrap into values that can revive stale references.

§ 90(7) Ordinary stale `ref` use is a safety violation producing deterministic panic/trap when dynamically detected.

§ 90(8) Stable/weak-handle target disappearance is normal fallible resolution.

§ 90(9) Stable handles may survive physical relocation through indirection and do not imply ownership unless explicitly defined.

§ 90(10) Safe-reference equality uses live storage identity plus referenced location; RawPtr equality uses raw address semantics.

§ 90(11) Generation checks never replace borrowing, bounds, initialization, relocation, address-space, or concurrency proof.

§ 90(12) The compiler may eliminate runtime metadata/checks when it proves equivalent guarantees.

§ 90(13) Profiles that cannot prove or check a safe-reference guarantee reject the safe operation or require an explicit RawPtr/unsafe boundary.

§ 90(14) The reference model requires no mandatory garbage collector, reference counter, handle table, allocator, generation manager, or general runtime.
