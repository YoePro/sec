# Reference Model

## Status

This document is the canonical reference-model rulebook for Sec 0.1.

It defines:

- the semantic guarantees of safe references;
- the distinction between safe references, slices, stable handles, weak handles,
  addressed storage, and `RawPtr[T]`;
- temporal validity, spatial validity, storage identity, provenance, type
  validity, access authority, nullability, relocation correctness, address-space
  correctness, and concurrency correctness;
- validity epochs and generational reference checks;
- allocation generations, arena epochs, collection storage epochs, and slot
  generations;
- direct references, stable handles, and weak handles;
- compile-time-selected epoch widths;
- generation exhaustion and stale-reference prevention;
- profile-dependent runtime representations;
- initialization requirements for safe typed allocation;
- failure semantics for stale references and stale handles;
- reference equality;
- FFI and `RawPtr[T]` boundaries;
- Semantic IR, diagnostics, LSP behavior, tests, and implementation planning;
- the rule that no mandatory runtime, handle table, garbage collector, or
  generation manager is introduced.

This document incorporates the previously planned generational-reference
rulebook. No separate `generational_references.md` rulebook is required.

---

# Purpose

Sec references must provide strong memory-safety guarantees without forcing one
physical pointer representation on every target.

The language therefore defines reference semantics independently of runtime
representation.

A compiler may represent a reference as:

```text
an address;
an address and length;
an address and expected epoch;
a slot identity and generation;
a hardware capability;
a target-specific tagged pointer;
a side-table key;
or another representation that preserves the required semantics.
```

The selected representation must preserve every semantic guarantee of the
reference kind.

---

# Core principle

```text
Sec defines the semantic guarantees of a reference independently of its
physical runtime representation.

A target or build profile may select another representation only when the same
source-level guarantees remain valid.
```

A profile may change:

```text
runtime cost;
metadata layout;
check placement;
check elimination;
address width;
epoch width;
hardware assistance;
side-table use;
slot-table use.
```

A profile may not silently change `ref T` from a safe reference into a raw
pointer.

---

# Non-goals

The reference model does not introduce:

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

The compiler may use internal lifetime, region, epoch, and provenance identities
without exposing them in Sec source.

---

# Terminology

## Storage identity

A compiler-recognized identity for one storage domain or one live incarnation
of storage.

A storage identity is not merely a numeric address.

## Validity epoch

A logical identity value describing one live incarnation of an invalidation
domain.

A generation is a common representation of a validity epoch.

## Invalidation domain

A storage owner or storage group whose existing references become invalid after
one semantic event.

Examples:

```text
one allocation;
one arena;
one collection storage buffer;
one reusable slot;
one registry domain.
```

## Direct reference

A safe reference whose access ultimately resolves directly to the current
storage address.

## Stable handle

A long-lived identity that resolves through a stable slot or equivalent lookup
mechanism and may therefore survive physical relocation.

## Weak handle

A handle that does not itself keep the target alive and whose resolution is
normally fallible.

## Stale reference

A reference whose expected storage identity or validity epoch no longer matches
the live storage.

## Stale handle

A handle whose domain, slot, or generation no longer identifies a live target.

## Provenance

Compiler-tracked evidence describing where a reference came from and which
storage identity it is authorized to access.

---

# Reference categories

Sec distinguishes several pointer-like categories.

They must not be collapsed into one universal pointer type.

---

# `ref T`

`ref T` is a safe, non-null, typed shared reference to one valid `T`.

Conceptual example:

```sec
fn Read(ref value: Item) int {
    return value.count
}
```

A valid `ref T` guarantees:

```text
non-nullness;
correct alignment for T;
valid initialized T representation;
read authority;
valid storage for the complete borrow live range;
correct storage provenance;
no access outside the referenced T or valid subobjects;
address-space compatibility;
borrow compatibility.
```

A `ref T` does not imply ownership.

A `ref T` does not imply that the storage is globally immutable.

It means that mutation through this reference is not permitted and that all
other mutation must obey Sec's aliasing and concurrency rules.

---

# `ref mut T`

`ref mut T` is a safe, non-null, typed mutable reference with exclusive mutable
authority for its borrow live range.

Conceptual example:

```sec
fn Increment(ref mut value: int) void {
    value += 1
}
```

In addition to the guarantees of `ref T`, `ref mut T` guarantees:

```text
write authority;
exclusive mutable access for the borrow live range;
no conflicting shared or mutable borrow;
writable storage;
compatible synchronization and address-space rules.
```

The type system and borrow checker enforce these guarantees.

A runtime generation check does not replace exclusive-borrow analysis.

---

# `ref T[]`

`ref T[]` is a safe bounded view over zero or more `T` elements.

A valid slice reference guarantees:

```text
one storage identity;
a valid element type;
a defined length;
a valid bounded range;
valid storage for the borrow live range;
correct element alignment;
read authority;
borrow compatibility.
```

A mutable slice uses the corresponding mutable-reference form according to the
canonical array and slice rulebook.

A slice may be empty.

An empty slice remains semantically valid even when its physical base field uses
a target-specific null or sentinel representation, provided that:

```text
length is zero;
the base is never dereferenced;
no safe ref T is constructed from the base alone;
all slice operations respect the zero length.
```

This representation detail does not make safe references nullable.

---

# Stable handles

A stable handle is a compiler- or library-supported identity for a target that
may:

```text
relocate;
be removed;
reuse storage slots;
outlive a short lexical borrow;
be resolved later.
```

The canonical conceptual representation is:

```text
domain identity + slot identity + expected generation
```

A stable handle may survive physical relocation because resolution obtains the
current address from a slot table or equivalent mechanism.

A stable handle does not keep the target alive unless its handle kind explicitly
defines ownership.

The source-level spelling and standard APIs for stable handles are not fixed by
Sec 0.1.

The semantic model is fixed by this rulebook.

---

# Weak handles

A weak handle does not guarantee that the target remains alive.

Resolution is normal fallible program behavior.

Conceptual forms:

```sec
let value := handle.Resolve()
```

Possible result types include:

```text
Option[ref T]
Result[ref T, StaleReferenceError]
```

The exact source API is deferred.

A stale weak handle must not require panic merely because the target has been
removed.

---

# `RawPtr[T]`

`RawPtr[T]` is the raw unsafe and FFI-oriented pointer representation.

It may:

```text
be null;
be invalid;
be misaligned;
refer to uninitialized storage;
have no known bounds;
have no known lifetime;
have no known ownership;
have no runtime generation;
have no compiler-valid provenance after foreign manipulation.
```

Possessing, storing, moving, comparing, or passing a `RawPtr[T]` is not by
itself an unsafe operation.

Interpreting it as live typed storage is unsafe.

The complete unsafe rules belong to `unsafe.md`.

---

# Addressed storage

Addressed storage is bound through target-aware declarations such as:

```sec
@address(Peripheral.GPIOA)
let mut GPIOA: GPIORegisters
```

It is:

```text
target-bound;
typed;
volatile according to addressed-storage rules;
stable according to the target binding;
not an ordinary allocator-owned generation domain.
```

Addressed storage normally does not use allocation generations.

Raw numeric addresses remain trusted target assertions according to
`attributes.md` and `unsafe.md`.

---

# Safe-reference guarantees

Safe reference validity is composed from several separate guarantees.

A generation match alone is not enough.

---

# Temporal validity

The referenced storage must remain live for the complete reference use.

Temporal validity protects against:

```text
use after destruction;
use after free;
use after arena reset;
use after arena release;
use after collection storage replacement;
use after slot removal and reuse;
use after owner invalidation.
```

Temporal validity may be proven statically or checked dynamically.

Generation or epoch checks are one implementation mechanism.

---

# Spatial validity

Every access must remain within the permitted storage extent.

For `ref T`, the permitted extent is:

```text
the referenced T and valid subobjects derived from it.
```

For `ref T[]`, the permitted indices are:

```text
0..<length
```

Temporal validity does not imply spatial validity.

A live allocation can still be accessed out of bounds.

---

# Storage provenance

A safe reference must originate from the correct live storage identity.

Example:

```text
allocation A uses address X;
A is destroyed;
allocation B later reuses address X.
```

A stale reference to A must not become valid for B merely because the numeric
address is reused.

Reference validity therefore depends on storage identity, not address alone.

---

# Type validity

A safe `ref T` guarantees:

```text
correct alignment for T;
initialized storage;
a valid T representation;
compatible access width;
valid representation invariants;
valid target address space for T.
```

A `RawPtr[T]` does not automatically establish these guarantees.

Converting `RawPtr[T]` into `ref T` or `ref mut T` is unsafe.

---

# Access authority

The reference kind defines its access authority.

```text
ref T
    read authority

ref mut T
    exclusive read and write authority
```

Additional operation-specific authorities may exist for:

```text
volatile access;
atomic access;
execute access;
DMA ownership;
foreign retention.
```

These authorities are not inferred merely from numeric addressability.

---

# Nullability

Safe references are never null.

```text
ref T
ref mut T
ref T[]
```

represent valid reference values according to their category.

Optional references use an explicit optional type:

```sec
Option[ref T]
```

`RawPtr[T]` may be null.

An empty slice may use a null-like internal base representation only as a hidden
implementation detail with length zero.

---

# Borrow compatibility

A valid generation does not grant alias authority.

Sec continues to enforce:

```text
compatible shared borrows;
one exclusive mutable borrow;
field-split borrows when non-overlap is proven;
no conflicting ref and ref mut;
no use after the borrow ends.
```

Borrow live ranges may be non-lexical and end at last use.

---

# Subreferences and narrowing

A safe reference derived from another safe reference may only retain or narrow
its authority.

Example:

```sec
let header := ref packet.header
```

The derived reference has:

```text
the same storage identity;
the same or shorter lifetime;
narrower spatial bounds;
the same or weaker access authority;
compatible address space;
compatible epoch dependency.
```

A shared reference cannot become mutable through safe derivation.

A field reference cannot reconstruct unrestricted access to the containing
allocation without an explicit unsafe boundary.

For slices:

```sec
let part := values[10..<20]
```

The new slice obtains narrower bounds and the same storage identity.

---

# Relocation correctness

A live reference must remain correct when storage moves, or physical relocation
must be forbidden while that reference is live.

Generation matching alone does not solve relocation.

A direct reference contains or resolves to the current address.

If the object moves and the reference is not updated or indirected, the direct
reference becomes invalid.

---

# Direct references

A direct reference conceptually resolves to:

```text
current storage address
```

It may additionally carry or depend on:

```text
bounds;
expected epoch;
provenance metadata;
address-space metadata.
```

A direct reference is suitable for:

```text
stack values;
short-lived borrows;
stable heap allocations;
arena allocations before invalidation;
pinned values;
addressed storage;
fields and slice elements under active borrows.
```

While a direct reference is live, the target may not physically relocate unless
the compiler proves every use remains correct through transformation.

---

# Stable handle resolution

A stable handle conceptually resolves through:

```text
domain -> slot -> current address
```

The handle checks:

```text
domain identity;
slot identity;
expected generation;
slot live state;
type and access compatibility.
```

Because the slot contains the current address, the target may relocate without
changing the handle identity.

The cost may include:

```text
one indirection;
slot metadata;
generation comparison;
synchronization.
```

Stable handles must not replace ordinary short-lived `ref` values by default.

---

# Pinning

Pinning prevents physical relocation for a defined lifetime.

Pinning may be required for:

```text
self-referential structures;
foreign APIs retaining addresses;
DMA;
OS registration;
intrusive data structures;
hardware descriptors;
target-specific address contracts.
```

Pinning does not by itself imply:

```text
ownership;
thread safety;
copyability;
permanent lifetime;
allocation;
reference counting.
```

The exact source syntax for pinning is deferred.

The semantic relationship is fixed:

```text
active pin dependency prevents physical relocation.
```

---

# Address spaces

A reference belongs to a compiler- and target-recognized address space.

Examples include:

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

Numerically equal addresses in different address spaces are not automatically
interchangeable.

Safe conversion requires compiler knowledge and semantic compatibility.

Raw conversion requires an unsafe boundary.

---

# Concurrency correctness

Generation checking does not prevent data races.

Concurrency safety continues to depend on:

```text
ownership;
sharing rules;
mutability;
synchronization;
atomic operations;
memory ordering;
reclamation protocol;
ISR rules.
```

A generation check followed by a load is not sufficient when another execution
context can invalidate the object between the check and the load.

Concurrent invalidation may require:

```text
ownership preventing invalidation;
a lock;
an atomic slot protocol;
a critical section;
hazard references;
epoch-based reclamation;
reference counting;
pinning during access;
target-specific synchronization.
```

Sec 0.1 does not require one universal concurrent reclamation mechanism.

---

# Validity epochs

A validity epoch identifies one live incarnation of an invalidation domain.

When an invalidating event occurs, the epoch changes or the domain ends.

A generation is the usual numeric representation of an epoch.

The language semantics use the broader term validity epoch because the physical
representation may be:

```text
a counter;
a compound identity;
a randomized token;
a domain identity plus slot generation;
a hardware tag;
a retired identity with replacement.
```

---

# Epoch ownership

An epoch belongs to an invalidation domain, not merely to an address.

This rule is canonical:

```text
A generation belongs to an invalidation domain, not merely to a numeric
address.
```

Possible invalidation domains include:

```text
allocation;
arena;
collection storage;
slot;
registry;
owner control block.
```

---

# Allocation generations

An allocation generation identifies one live incarnation of one dynamically
owned allocation identity.

The generation changes or the identity is retired when the allocation is:

```text
destroyed;
freed;
reused;
replaced by another logical allocation.
```

The allocator may store the generation in:

```text
allocation metadata;
a side table;
an owner control block;
a hardware tag;
an encoded pointer or handle.
```

No particular representation is mandatory.

---

# Arena epochs

An arena may use one owner-wide epoch.

Conceptual state:

```text
Arena {
    identity
    current epoch
}
```

Arena-backed references that require runtime validation retain the expected
arena epoch.

On reset:

```text
the arena remains live;
the epoch changes;
all previous arena allocations are invalidated;
all references depending on previous allocations become stale;
capacity is restored according to the arena rules.
```

On release:

```text
the domain ends;
all arena-backed storage is invalidated;
all dependent references become stale;
future ordinary arena use is forbidden.
```

One arena epoch may invalidate many allocations at once.

The implementation does not need one generation field per arena allocation.

---

# Collection storage epochs

A collection may use one storage epoch for its backing storage.

The epoch may change when storage is:

```text
reallocated;
replaced;
structurally rebuilt;
compacted in a way that invalidates direct references.
```

Element mutation that preserves storage identity and reference validity does not
necessarily change the storage epoch.

Structural mutation rules remain coordinated with iteration freeze, borrowing,
and collection semantics.

---

# Slot generations

A reusable slot uses a slot generation to distinguish successive live occupants.

Conceptual slot:

```text
Slot {
    current address
    current generation
    live state
}
```

A stable handle contains:

```text
domain identity
slot identity
expected generation
```

When an element is removed and the slot is reused, the slot generation changes.

An old handle must not resolve to the new occupant.

---

# Runtime generation width

Validity epochs use a 64-bit logical width by default.

This default also applies to 32-bit general-purpose targets.

Pointer width and epoch width are separate concerns.

Example:

```text
32-bit target address:
    32 bits

logical epoch:
    64 bits
```

A 64-bit epoch may be stored as two machine words on a 32-bit target.

The physical representation may be optimized or omitted where proof permits.

---

# Compile-time-selected shorter widths

A target or build profile may select a shorter epoch width only at compile time.

Epoch width is never selected dynamically at runtime.

A shorter width is permitted only when the compiler can preserve the
stale-reference guarantee through one or more of:

```text
statically bounded invalidation count;
slot retirement;
domain retirement;
explicit exhaustion handling;
proof that stale references cannot survive reuse;
an equivalent target-specific guarantee.
```

The selected width is part of the concrete compilation plan.

---

# Future wider widths

Future targets or hardened profiles may use wider epochs such as 128 bits.

This does not change source-level reference semantics.

The language does not expose epoch width as ordinary runtime program state.

---

# Constrained bare-metal profiles

A constrained profile should omit runtime generation metadata when validity is
statically proven.

Common cases include:

```text
stack borrows;
fixed addressed storage;
short-lived arena borrows proven not to cross reset;
fixed arrays;
non-relocating statically allocated values.
```

A constrained target may use a shorter compile-time-selected generation when the
required proof or exhaustion behavior exists.

When validity cannot be proven or checked, the compiler must:

```text
reject the safe operation;
or require an explicit RawPtr/unsafe boundary.
```

The profile may not silently weaken safe-reference semantics.

---

# Generation increments

Generation and epoch increments use checked arithmetic.

They never silently wrap.

Canonical rule:

```text
A generation or validity epoch must never wrap into a value that can make an
older stale reference valid again.
```

---

# Generation exhaustion

Finite generations may eventually reach their maximum representable value.

Exhaustion is allowed to exhaust an identity domain.

Exhaustion must never revive a stale reference.

At exhaustion, an implementation may:

```text
retire the slot permanently;
retire the allocation identity;
replace the arena control domain with a fresh independent identity;
rekey the domain while preserving old-domain distinction;
return an explicit exhaustion error when the API is fallible;
produce a deterministic panic or target trap when recovery is unavailable.
```

The implementation must not continue with:

```text
MAX_GENERATION + 1 = 0
```

when an older matching identity may still exist.

---

# Slot retirement

When a reusable slot exhausts its generation range, the implementation may
retire the slot permanently and allocate another slot.

The retired slot must not receive a new live occupant under the exhausted
identity.

This permits compact slot generations when slot retirement is acceptable.

---

# Arena exhaustion

When an arena epoch is exhausted, safe choices include:

```text
replace the complete identity domain;
recreate the arena with a fresh independent identity;
make reset fail when the API is fallible;
produce deterministic panic or target trap otherwise.
```

A fresh domain identity must remain distinguishable from every previous live or
stale domain identity that may still be represented.

---

# Atomicity and generation width

Generation width does not imply atomicity.

A 64-bit epoch may not be atomically readable or writable on a 32-bit target.

Concurrent invalidation and resolution require an atomic or otherwise
synchronized protocol.

Possible mechanisms include:

```text
target-supported atomic operations;
locks;
critical sections;
versioned slot protocols;
32-bit atomic generations with retirement;
ownership preventing concurrent invalidation.
```

The compiler must not generate an unsynchronized torn epoch protocol and call it
safe.

---

# Allocation initialization

Safe typed allocation always produces a fully initialized valid value.

Example:

```sec
let value := allocator.New[Value]()
```

The resulting storage must contain a valid `Value` according to:

```text
type defaults;
field defaults;
constructors;
contracts;
representation validity.
```

This does not require every physical byte to be zeroed before initialization.

The compiler may initialize fields directly and omit redundant writes.

---

# Zeroing and allocation identity

Memory is not universally zeroed merely to establish allocation identity.

Zeroing does not prevent stale references from matching reused addresses.

Storage identity and epoch validity solve that problem.

Zeroing may still be required by:

```text
type defaults;
security policy;
secret erasure;
allocator contract;
explicit zeroed allocation API;
target requirements.
```

These concerns remain separate from reference identity.

---

# Uninitialized storage

Uninitialized typed storage is not readable through a safe reference.

Raw uninitialized allocation remains unsafe until a dedicated initialized-state
model is defined.

Sec 0.1 does not require a public `Uninit[T]` type.

Compiler and core implementation may use internal uninitialized storage only
while preserving all initialization rules before safe access.

---

# Check placement

Dynamic validity checks are inserted only when required by the selected profile
and proof state.

Possible checks include:

```text
epoch comparison;
slot live-state check;
bounds check;
address-space check;
type-tag check;
hardware capability validation.
```

The compiler should remove checks when validity is statically proven.

---

# Safe ordinary references

Safe Sec code should not normally create stale ordinary `ref` values.

The compiler prevents stale references through:

```text
ownership analysis;
borrow checking;
escape analysis;
arena lifetime analysis;
relocation restrictions;
collection mutation rules;
call and effect analysis;
target knowledge.
```

Runtime reference checks are therefore primarily:

```text
hardening;
defense against unsafe violations;
defense against foreign contract violations;
protection in dynamic lifetime models;
target-specific safety support.
```

---

# Stale ordinary reference failure

A stale ordinary safe reference indicates a violated safety guarantee.

Dereferencing or otherwise using it results in:

```text
deterministic panic;
or target trap where the profile uses trap semantics.
```

It is not normal fallible business logic.

The programmer does not write `try` for every ordinary reference access.

---

# Stale handle resolution

A stale weak or stable handle is normal fallible resolution.

The resolution API returns an explicit optional or error result.

It does not panic merely because the referenced object was removed.

A handle API may separately provide an asserting resolution operation whose
failure panics according to its explicit contract.

---

# Reference equality

Safe-reference equality means:

```text
the same live storage identity;
and the same referenced location within that storage.
```

Numeric address equality alone is insufficient when storage identity differs.

Two references to the same live field or element may compare equal.

Two references to different live incarnations at the same reused address are
not equal.

---

# Stable-handle equality

Stable-handle equality means:

```text
the same domain identity;
the same slot identity;
the same generation.
```

A stale handle and a later handle to a new slot occupant are not equal even when
the slot number is reused.

---

# `RawPtr` equality

`RawPtr[T]` equality is raw address equality according to the target and FFI
rules.

It does not imply:

```text
same live allocation;
same provenance;
same generation;
safe dereference;
same address space unless the pointer types establish it.
```

---

# Safe reference to `RawPtr`

Conversion from a safe reference to `RawPtr[T]` may be permitted for an explicit
foreign or unsafe operation.

The compiler must preserve or verify:

```text
the safe borrow remains live;
the foreign call does not retain the pointer unless declared;
mutability is compatible;
the address space is ABI-compatible;
the object does not relocate during the foreign use;
all pinning requirements are satisfied.
```

The resulting `RawPtr[T]` is raw.

It must not be assumed to retain safe-reference guarantees after arbitrary
foreign manipulation.

---

# `RawPtr` to safe reference

Converting `RawPtr[T]` to `ref T`, `ref mut T`, or `ref T[]` is unsafe.

The programmer or trusted wrapper must establish:

```text
non-nullness;
correct alignment;
initialized valid T representation;
valid bounds;
valid lifetime;
ownership compatibility;
alias compatibility;
address-space compatibility;
read or write authority;
foreign retention compatibility;
relocation safety.
```

Generation or provenance may need to be established through a trusted storage
owner or target mechanism.

---

# Foreign retention

An FFI declaration must distinguish whether foreign code may retain a pointer
beyond the call.

When retention is not declared, the compiler may treat the foreign use as
bounded by the call.

When retention is declared or unknown, the compiler must require sufficient
lifetime, pinning, ownership, and effect guarantees.

Unknown retention is conservative.

Exact FFI syntax belongs to the FFI rulebook.

---

# Serialization and persistence

Safe references and stable handles are not automatically serializable or
persistent.

Their identities are normally valid only within one domain such as:

```text
one process;
one allocator instance;
one registry;
one arena instance;
one device lifetime;
one runtime domain.
```

A persistent identifier is a domain-specific ID, not a memory reference.

Serialization must use explicit persistent identity semantics.

---

# Moves and physical relocation

A language-level move does not always imply physical relocation.

Reference analysis must distinguish:

```text
ownership transfer;
logical move;
physical address change;
slot relocation;
copying;
pinning.
```

A move that preserves the effective storage address may preserve direct
references when all ownership and borrow rules permit it.

A move that changes the physical address invalidates direct references unless
they are updated through a compiler-proven transformation or represented as
stable handles.

The copy and move rulebook remains authoritative for value semantics.

---

# Collection relocation

Collections must declare or infer whether structural operations preserve element
addresses.

Operations that may relocate storage must:

```text
be forbidden while conflicting direct references are live;
or invalidate the relevant storage epoch;
or use stable indirection;
or be proven not to affect the referenced element.
```

Element mutation that does not relocate storage remains subject to ordinary
borrow rules.

---

# Arena integration

Arena-backed references depend on:

```text
arena identity;
current arena epoch;
allocation extent;
borrow live range;
escape restrictions.
```

Arena reset and release are invalidating events.

Non-lexical borrow liveness may permit reset after the last use even when the
reference variable remains lexically in scope.

The arena rulebook defines programmer-visible arena APIs.

This document defines the reference validity consequences.

---

# Effect analysis integration

Reference validity operations may introduce effects according to their actual
semantics.

Examples:

```text
handle resolution may read shared metadata;
slot-table growth may allocate;
foreign pointer conversion may cross unsafe provenance;
addressed storage access may be volatile;
concurrent resolution may block if its API uses a lock.
```

Generation checking itself does not imply allocation or blocking.

Effect summaries remain separate from reference validity.

---

# Unsafe integration

`unsafe` does not disable the reference model.

It permits specific operations whose proof obligations are accepted by the
programmer or trusted implementation.

Unsafe operations may establish:

```text
raw pointer validity;
manual lifetime assumptions;
manual alignment assumptions;
manual provenance assumptions;
manual foreign retention assumptions.
```

Only the specific assumed obligation loses compiler proof.

All unrelated Sec rules remain active.

---

# No mandatory runtime

The reference model introduces no mandatory runtime.

It does not require:

```text
garbage collection;
reference counting;
a global allocation table;
a global handle table;
a scheduler;
a generation manager;
a tracing collector;
a runtime exception system.
```

A profile may use metadata or helpers when needed.

The compiler may lower proven references to plain machine addresses.

---

# Profile examples

## Hosted optimized profile

Typical lowering:

```text
short-lived ref T:
    address only

slice:
    address + length

stable handle:
    slot + generation

arena dynamic hardening:
    address + expected arena epoch where required
```

## Hosted hardened profile

Possible lowering:

```text
address + epoch;
side-table validation;
hardware memory tags;
capability bounds;
additional provenance checks.
```

## 32-bit general-purpose profile

Typical policy:

```text
32-bit address;
64-bit logical owner epoch by default;
compile-time-selected compact slot generation where retirement is supported;
no runtime generation on statically proven references.
```

## Constrained bare-metal profile

Typical policy:

```text
address-only proven references;
no generation for fixed MMIO;
no generation for bounded lexical borrows;
compile-time-selected compact generations only where proof permits;
reject dynamic safe-reference patterns that cannot be proven or checked.
```

---

# Semantic IR requirements

Semantic IR must represent enough information to preserve reference semantics.

At minimum, it must be able to represent:

```text
reference category;
source storage identity;
address space;
spatial extent;
borrow kind;
borrow live range;
mutability authority;
validity epoch dependency;
slot identity;
handle resolution;
reference derivation;
bounds narrowing;
physical relocation;
pinning dependency;
unsafe raw conversion;
foreign retention;
invalidation event;
stale failure behavior.
```

These facts may be explicit nodes, attributes, or analysis side tables.

---

# Reference-analysis domains

The compiler should keep separate analysis facts for:

```text
lifetime validity;
bounds;
provenance;
initialization;
type validity;
borrow compatibility;
relocation;
pinning;
address space;
generation or epoch;
concurrency protocol;
trust provenance.
```

A single `IsSafePointer` boolean is insufficient.

---

# Invalidation events

The compiler must model invalidation events explicitly.

Initial events include:

```text
owner destruction;
allocation free;
arena reset;
arena release;
collection storage replacement;
slot removal;
slot reuse;
physical relocation of direct-reference target;
foreign invalidation declared by contract;
target-specific memory-domain reset.
```

Every invalidation event identifies which references and handles are affected.

---

# Static check elimination

A runtime validity check may be removed when the compiler proves:

```text
the storage identity remains live;
no invalidating event reaches the use;
the epoch cannot change;
the borrow remains valid;
the reference remains within bounds;
no physical relocation occurs;
concurrent invalidation is impossible;
address-space compatibility is fixed.
```

Check elimination must be semantic proof, not an optional optimization required
for correctness.

---

# Diagnostics

Reference diagnostics must explain:

```text
which guarantee failed;
where the reference or handle was created;
which storage identity it depends on;
which invalidating event occurred;
where the invalid use or resolution occurred;
whether the failure is static, dynamic, trusted, or unknown;
the active target and profile;
which safe alternative exists where defined.
```

---

# Use-after-reset diagnostic

Example:

```text
error: reference `view` depends on arena storage invalidated by reset

reference created:
    parser.sec:40

arena reset:
    parser.sec:46

invalid use:
    parser.sec:49
```

---

# Relocation diagnostic

Example:

```text
error: direct reference `item` may be invalidated by collection relocation

reference created:
    registry.sec:71

relocating operation:
    registry.sec:78

help:
    end the borrow before the structural mutation,
    use a stable handle,
    or use storage with stable addresses
```

---

# Epoch exhaustion diagnostic

Example:

```text
error: arena epoch domain is exhausted

arena:
    scratch

epoch width selected by profile:
    32 bits

help:
    use the default 64-bit epoch profile,
    recreate the arena with a fresh domain identity,
    or use an API that reports reset exhaustion
```

---

# Unsafe conversion diagnostic

Example:

```text
error: RawPtr[Value] cannot be converted to ref Value in safe code

missing proof obligations:
    lifetime
    alignment
    initialization
    alias compatibility
    provenance

help:
    validate the pointer inside a narrow unsafe context or use a safe wrapper
```

---

# LSP behavior

The LSP should display:

```text
reference category;
borrow kind;
storage identity where useful;
known bounds;
address space;
relocation stability;
pinning status;
epoch or generation dependency;
static versus runtime validation;
unsafe trust provenance;
invalidating operations;
handle fallibility;
selected profile representation when available.
```

Hover information should remain readable and avoid exposing unnecessary
compiler-internal identifiers by default.

---

# Information diagnostics

The compiler or LSP may report information such as:

```text
reference validity is fully proven; runtime epoch check removed

stable handle uses compile-time-selected 32-bit slot generation with slot
retirement

arena uses one shared 64-bit epoch for all arena-backed allocations
```

These diagnostics may be configurable.

---

# Implementation status

The rulebook is normative even when compiler support is incomplete.

Implementation tracking must distinguish:

```text
reference syntax;
borrow checking;
escape analysis;
bounds analysis;
provenance analysis;
relocation analysis;
pinning;
epoch analysis;
stable handles;
weak handles;
profile representation;
FFI retention;
concurrent invalidation;
diagnostics;
LSP support.
```

Do not mark the complete rulebook implemented when only generation checks exist.

---

# Required tests

Create or update:

```text
references_valid.sec
references_invalid.sec
reference_nullability_valid.sec
reference_nullability_invalid.sec
reference_bounds_valid.sec
reference_bounds_invalid.sec
reference_provenance_valid.sec
reference_provenance_invalid.sec
reference_relocation_valid.sec
reference_relocation_invalid.sec
reference_epochs_valid.sec
reference_epochs_invalid.sec
stable_handles_valid.sec
stable_handles_invalid.sec
weak_handles_valid.sec
weak_handles_invalid.sec
reference_ffi_valid.sec
reference_ffi_invalid.sec
reference_concurrency_valid.sec
reference_concurrency_invalid.sec
reference_profiles_valid.sec
reference_profiles_invalid.sec
```

---

# Basic reference tests

Test:

```text
ref T is non-null;
Option[ref T] is valid optionality;
ref mut exclusivity;
shared borrow compatibility;
field-split borrows;
subreference narrowing;
slice bounds;
empty slice;
reference equality;
RawPtr equality distinction.
```

---

# Temporal-validity tests

Test:

```text
use after owner destruction rejected;
use after free rejected;
use after arena reset rejected;
use after arena release rejected;
use after collection relocation rejected;
use after slot reuse rejected;
last-use borrow ending before reset accepted;
static proof removes dynamic check.
```

---

# Generation tests

Test:

```text
64-bit default epoch on 32-bit general-purpose profile;
no epoch for statically proven reference;
shorter compile-time-selected epoch with bounded proof;
runtime width selection rejected;
checked increment;
no wrap;
slot retirement at exhaustion;
arena domain replacement;
explicit exhaustion error;
deterministic trap where recovery is unavailable.
```

---

# Stable-handle tests

Test:

```text
relocation preserves stable handle;
slot reuse invalidates old handle;
fallible resolution returns no target;
handle equality includes domain, slot, and generation;
handle does not imply ownership;
owning handle kind retains target only when explicitly defined;
concurrent resolution requires valid protocol.
```

---

# Allocation initialization tests

Test:

```text
safe typed allocation returns initialized valid T;
zero-default arrays are semantically zero-initialized;
compiler may omit redundant whole-buffer zeroing;
uninitialized storage cannot be read through safe ref;
raw uninitialized allocation requires unsafe handling.
```

---

# FFI tests

Test:

```text
safe reference passed for call-bounded foreign use;
foreign retention requires declared lifetime support;
mutable foreign use requires compatible borrow;
RawPtr to ref requires unsafe;
foreign pointer return does not automatically gain provenance;
pinning required for retained direct address;
address-space incompatibility rejected.
```

---

# Binary and representation tests

Verify:

```text
proven ref lowers to address only;
proven slice lowers to address and length;
unused epoch support is omitted;
no global generation manager is linked;
32-bit target may use 64-bit epoch control metadata;
constrained profile omits epoch metadata when proven;
shorter profile width is compile-time constant;
hardware capability lowering preserves semantics.
```

---

# Required synchronization

This rulebook must remain synchronized with:

```text
memory_model.md
ownership.md
copy_move.md
unsafe.md
effect_analysis.md
allocation rulebook
arena rulebook
arrays and slices rulebook
collections rulebook
FFI rulebook
attributes.md
addressed-storage rules
concurrency rulebook
thread and task rulebooks
interrupt and ISR rulebooks
stack analysis rulebook
call_graph.md
Semantic IR rulebook
compiler pipeline rulebook
formatter.md
lsp.md
diagnostics rulebook
language-rulebook-status.md
rules_implementations.txt
```

The separate planned entry for `generational_references.md` must be removed or
marked merged into `reference_model.md`.

---

# Appendix A — Canonical guarantee table

| Guarantee | Meaning | Primary mechanisms |
|---|---|---|
| Temporal validity | Storage remains live | ownership, borrow checking, epochs |
| Spatial validity | Access remains within extent | type bounds, slice length, bounds checks |
| Provenance | Reference belongs to correct storage identity | compiler provenance, domain identity |
| Type validity | Storage is valid initialized `T` | initialization analysis, type rules |
| Access authority | Read/write rights are valid | `ref`, `ref mut`, borrow checker |
| Nullability | Safe references are never null | type system, `Option[ref T]` |
| Relocation correctness | Address remains correct or indirected | pinning, stable handles, relocation proof |
| Address-space correctness | Reference uses compatible memory domain | target knowledge, address-space typing |
| Concurrency correctness | Access and invalidation are synchronized | ownership, atomics, locks, reclamation |

---

# Appendix B — Canonical representation examples

| Reference kind | Possible representation |
|---|---|
| Proven `ref T` | address |
| Proven `ref T[]` | address + length |
| Hardened direct reference | address + expected epoch |
| Arena-backed hardened reference | address + arena epoch |
| Stable handle | domain + slot + generation |
| Weak handle | domain + slot + generation with fallible resolution |
| Addressed storage | target-known fixed address |
| `RawPtr[T]` | raw ABI-compatible address |
| Capability target reference | hardware capability |

---

# Appendix C — Canonical invalidation table

| Event | Invalidates |
|---|---|
| Owner destruction | references depending on owner storage |
| Allocation free | references to that allocation incarnation |
| Arena reset | prior arena-backed allocations and references |
| Arena release | all arena-backed storage and arena use |
| Collection reallocation | direct references to old backing storage |
| Slot removal | handles and references to removed occupant |
| Slot reuse | old generation handles |
| Physical relocation | direct references unless preserved by proof or indirection |
| Foreign invalidation | references covered by the foreign contract |

---

# Appendix D — Codex implementation plan

## D.1 Add the rulebook

Add:

```text
rules/reference_model.md
```

Remove or merge the planned entry:

```text
generational_references.md
```

Update:

```text
language-rulebook-status.md
rules/rules_implementations.txt
```

Mark `reference_model.md` Written.

Do not mark the complete reference model Implemented.

---

## D.2 Inventory existing compiler support

Locate current logic for:

```text
ref and ref mut syntax;
slice references;
borrow checking;
escape analysis;
array and slice bounds;
move invalidation;
arena invalidation;
collection relocation;
RawPtr;
FFI pointer conversion;
addressed storage;
target address spaces;
pinning or stable-address assumptions;
existing generation metadata;
LSP reference diagnostics.
```

Reuse existing facts instead of introducing parallel models.

---

## D.3 Add storage identities

Introduce stable compiler identities for:

```text
allocation domains;
arena domains;
collection storage domains;
slot domains;
addressed target storage;
foreign storage where declared.
```

Do not use numeric address as the sole identity.

---

## D.4 Add reference fact structure

A conceptual structure may include:

```go
type ReferenceFacts struct {
    Kind             ReferenceKind
    Storage          StorageIdentity
    AddressSpace     AddressSpaceID
    Bounds           BoundsValue
    BorrowKind       BorrowKind
    Lifetime         LifetimeID
    EpochDependency  *EpochIdentity
    Relocation       RelocationClass
    PinDependency    *PinIdentity
    Provenance       ProvenanceKind
}
```

This is illustrative, not mandatory source architecture.

---

## D.5 Add invalidation events

Represent:

```text
free;
destroy;
arena reset;
arena release;
collection storage replacement;
slot removal;
slot reuse;
physical relocation;
foreign invalidation.
```

Connect each event to affected storage identities.

---

## D.6 Integrate non-lexical borrow liveness

Use control-flow last-use analysis.

Permit borrow end before lexical scope exit.

Reject every use after invalidation.

---

## D.7 Add epoch domains

Implement epoch metadata as an analysis abstraction first.

Support:

```text
allocation generation;
arena epoch;
collection storage epoch;
slot generation.
```

Do not force runtime metadata where proof eliminates it.

---

## D.8 Add profile selection

The concrete CompilationPlan selects:

```text
default epoch width;
shorter proven widths;
metadata placement;
check policy;
hardware assistance;
concurrent protocol support.
```

Width selection is compile-time only.

Default logical epoch width is 64 bits.

---

## D.9 Implement checked generation changes

Generation increments must use checked arithmetic.

Implement:

```text
no wrap;
slot retirement;
domain retirement;
explicit exhaustion;
deterministic trap fallback.
```

Add tests near maximum values without requiring billions of operations.

---

## D.10 Add stable-handle analysis

Prepare compiler representation for:

```text
domain identity;
slot identity;
expected generation;
fallible resolution;
relocation survival;
ownership-independent handle semantics.
```

Do not invent source syntax beyond accepted language rules.

---

## D.11 Add relocation analysis

Classify storage as:

```text
stable address;
may relocate;
pinned;
indirect stable slot;
unknown.
```

Reject direct references that cross possible relocation.

---

## D.12 Add initialization integration

Ensure safe typed allocation produces initialized valid `T`.

Do not mandate redundant zeroing.

Reject safe reads of uninitialized storage.

---

## D.13 Integrate FFI

Track:

```text
call-bounded pointer use;
foreign retention;
foreign mutation;
pinning requirement;
address-space compatibility;
RawPtr provenance loss;
trusted pointer-to-reference conversion.
```

Unknown foreign behavior is conservative.

---

## D.14 Integrate concurrency

Do not treat generation checks as sufficient synchronization.

Require:

```text
ownership exclusion;
atomic protocol;
lock;
critical section;
or another verified reclamation mechanism.
```

Reject unsupported concurrent handle models on constrained profiles.

---

## D.15 Semantic IR

Expose reference derivation, narrowing, invalidation, relocation, pinning, handle
resolution, and unsafe conversion explicitly enough for analysis and lowering.

---

## D.16 Diagnostics

Add cause chains showing:

```text
reference creation;
storage identity;
invalidating event;
invalid use;
selected profile;
selected epoch width;
trusted boundary;
safe alternative.
```

Use stable diagnostic IDs.

---

## D.17 LSP

Add hover and navigation for:

```text
reference kind;
bounds;
borrow status;
relocation stability;
epoch dependency;
handle identity;
invalidation source;
profile representation;
trust provenance.
```

---

## D.18 Tests

Run:

```text
go test ./...
compiler build
LSP build
formatter tests
reference fixtures
arena fixtures
collection relocation fixtures
FFI fixtures
target profile matrix
binary dependency tests
```

Do not claim implementation complete until all reference-safety domains are
integrated.

---

# Final canonical summary

Sec safe-reference semantics are independent of physical runtime
representation.

`ref T`, `ref mut T`, and `ref T[]` are safe, non-null, typed, bounded, lifetime-
valid references.

`RawPtr[T]` remains a separate raw unsafe and FFI-oriented representation.

Reference safety consists of distinct guarantees:

```text
temporal validity;
spatial validity;
storage provenance;
type and initialization validity;
access authority;
nullability;
borrow compatibility;
relocation correctness;
address-space correctness;
concurrency correctness.
```

Generation matching is one temporal-validity mechanism, not the complete memory-
safety model.

A validity epoch belongs to an invalidation domain, not merely to an address.

Invalidation domains include allocations, arenas, collection storage, and
reusable slots.

Validity epochs use a 64-bit logical width by default, including on 32-bit
general-purpose targets.

Shorter widths are permitted only when selected at compile time and when the
compiler preserves stale-reference safety through bounded reuse, retirement,
explicit exhaustion handling, or equivalent proof.

Future profiles may use wider epochs without changing source semantics.

Generation increments are checked and never wrap into a value that can revive a
stale reference.

At exhaustion, the domain is retired, safely replaced, or reports deterministic
failure.

A safe typed allocation produces a fully initialized valid value.

Memory is not universally zeroed merely to establish allocation identity.

Safe ordinary references are expected to remain valid.

A stale ordinary safe reference produces deterministic panic or target trap.

A stale weak or stable handle resolves fallibly.

A stable handle identifies a domain, slot, and generation and may survive
physical relocation.

A stable handle does not imply ownership unless its handle kind explicitly says
so.

Safe-reference equality compares live storage identity and referenced location.

Stable-handle equality compares domain, slot, and generation.

`RawPtr` equality is raw address equality.

Generation checks do not replace borrow checking, bounds checking,
initialization analysis, relocation analysis, address-space validation, or
concurrency synchronization.

Profiles may omit runtime metadata and checks when the compiler proves the same
guarantees.

A profile that cannot prove or check a safe-reference guarantee must reject the
safe operation or require an explicit `RawPtr` and unsafe boundary.

The reference model introduces no mandatory runtime, garbage collector,
reference counter, handle table, allocator, or generation manager.
