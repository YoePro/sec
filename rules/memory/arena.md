# Arena

## Status

This document is the canonical arena rulebook for Sec 0.1.

It defines:

- the programmer-visible `Arena` type;
- arena allocation domains;
- owned, borrowed, static, and target-provided backing storage;
- fixed and growable arenas;
- implicit allocation-context propagation;
- manual typed arena allocation;
- allocation failure;
- initialization;
- ownership and destruction;
- reset and release;
- validity epochs;
- nested arenas;
- task and thread dependencies;
- cancellation and cleanup;
- static capacity analysis;
- Semantic IR requirements;
- Sec MLIR dialect requirements;
- lowering and optimization constraints;
- target-profile restrictions;
- diagnostics;
- LSP behavior;
- tests;
- staged compiler implementation.

All normative Arena semantics belong to this document.

General allocation semantics remain owned by `allocation.txt`.

Safe reference semantics, storage identity, validity epochs, stable handles, weak
handles, and `RawPtr[T]` remain owned by `reference_model.md`.

Effect inference and ordered arena effects remain owned by
`effect_analysis.md`.

Task, thread, cancellation, destruction, panic, call-graph, ownership, borrow,
and lifetime semantics remain owned by their corresponding canonical
rulebooks.

This rulebook consumes those semantics and defines their Arena-specific
application.

---

# Purpose

An Arena is a programmer-visible allocation domain for typed storage.

It provides:

```text
explicit dynamic-storage operations;
bulk reclamation;
predictable storage lifetime;
target-independent source semantics;
compiler-visible ownership and invalidation;
optional static capacity proof;
implementation without a mandatory runtime.
```

The normal programmer should not need to select or propagate an Arena through
ordinary application code.

The compiler may maintain and propagate an active allocation context for
operations whose semantics explicitly allocate.

Systems, embedded, FFI, allocator, bootstrap, and performance-sensitive code may
use an Arena explicitly.

---

# Core rule

```text
An Arena owns and controls one allocation domain.

The Arena may own, borrow, or otherwise control its physical backing storage.

Allocations produced from the Arena belong to the ArenaDomain and its current
validity epoch.

Reset reclaims allocations while keeping the Arena alive.

Release terminates the ArenaDomain.
```

---

# Non-goals

Sec 0.1 Arena does not define:

```text
garbage collection;
reference counting;
individual deallocation;
automatic heap promotion;
relocating compaction;
automatic trimming on Reset;
a destructor registry;
general placement construction;
safe uninitialized typed storage;
public raw access to the complete Arena backing store;
a concurrent Arena type;
a lock-free Arena type;
a self-referential Arena owner/result wrapper;
runtime reflection over Arena allocations;
a universal runtime allocator;
a universal Arena ABI.
```

These may be designed separately in a later language version.

---

# Terminology

## Arena

The programmer-visible move-only value that controls an allocation domain.

## ArenaDomain

The compiler-visible storage identity controlled by an Arena.

An ArenaDomain is not source-level region syntax.

It identifies one live allocation domain independently of the physical address
of its backing storage.

## Backing storage

The physical bytes from which Arena allocations are produced.

Backing storage may be:

```text
owned;
borrowed;
statically reserved;
target-provided;
segmented;
virtually reserved;
or otherwise supplied by the active CompilationPlan.
```

## Arena state version

The compiler SSA state produced after a mutating Arena operation.

Allocation, growth, Reset, and Release affect Arena state.

## Validity epoch

The logical incarnation of allocations within one ArenaDomain.

Ordinary allocation does not advance the epoch.

Reset advances the epoch.

Release ends the complete ArenaDomain.

## Allocation context

Compiler-visible semantic state through which an allocation-capable operation
obtains storage without a source-level Arena argument.

## Fixed Arena

An Arena whose available backing capacity cannot grow.

## Growable Arena

An Arena that may acquire additional stable backing without relocating any
existing live allocation.

## Arena dependency

A compiler-visible requirement that an ArenaDomain and, where relevant, one
validity epoch remain valid.

---

# Arena is a semantic builtin

`Arena` is a semantic builtin type.

Its existence does not require an ordinary source declaration in core.

Core or target code may provide helper implementations.

The compiler owns:

```text
member existence;
type checking;
ownership behavior;
allocation-domain identity;
effect classification;
Semantic IR meaning;
lowering requirements.
```

The exact helper implementation may vary by target and CompilationPlan.

---

# The lowercase `arena` identifier

Sec 0.1 does not require `arena` as a language keyword.

`Arena` is the builtin type.

A lowercase identifier such as:

```sec
let mut arena := try Arena.WithCapacity(4096)
```

is an ordinary identifier.

An implementation that currently reserves `arena` must remove that reservation
unless another canonical rulebook later assigns source syntax to the keyword.

No special scoped-arena syntax is introduced by this rulebook.

---

# Arena ownership

Arena is move-only and non-copyable.

This is required because an Arena contains or controls:

```text
allocation-domain identity;
backing-storage authority;
allocation cursor state;
capacity policy;
validity epoch;
provider state;
dependent storage.
```

Ordinary copying would create two apparent owners of one allocation domain.

This is invalid:

```sec
let second := first
```

when `first` has type `Arena`.

An ownership move is valid:

```sec
let second :<- first
```

After the move:

```text
second owns the same ArenaDomain;
second retains the same epoch;
second retains the same backing;
second retains the same allocation state;
first is moved-from and cannot be used.
```

Moving an Arena does not invalidate Arena-backed references.

Moving an Arena does not create a new ArenaDomain.

Moving an Arena normally requires no runtime operation.

---

# Arena composition

A type containing an Arena is move-only by composition.

Example:

```sec
type WorkerStorage struct {
    arena: Arena
}
```

Moving `WorkerStorage` moves the Arena owner.

Destroying `WorkerStorage` destroys the Arena field if the field is still owned.

The compiler must preserve normal field destruction order.

---

# Allocation-domain ownership versus backing ownership

An Arena always owns and controls its ArenaDomain.

The Arena does not necessarily own the physical backing bytes.

The backing ownership kind is one of:

```text
Owned
Borrowed
Static
TargetProvided
```

A growable Arena may contain multiple backing segments whose ownership kind is
defined by its provider.

The ownership kind must be known to Semantic IR before physical lowering.

---

# Borrowed fixed Arena

A borrowed fixed Arena is created from mutable contiguous backing storage.

Canonical source form:

```sec
let mut arena := Arena.FromBuffer(ref mut buffer)
```

Conceptual declaration:

```sec
fn FromBuffer(buffer: ref mut byte[]) Arena
```

`self` is implicit for instance methods in Sec and is not written as a parameter.

`FromBuffer`:

```text
creates a new ArenaDomain;
uses the supplied view as fixed backing;
takes an exclusive borrow of the supplied view;
does not allocate new backing storage;
does not grow;
is infallible;
keeps the backing owner outside the Arena;
does not deallocate the backing when released.
```

The supplied view must be:

```text
mutable;
contiguous;
addressable;
valid for the complete Arena lifetime;
large enough to represent its own bounds;
compatible with the target address space.
```

The Arena exclusively controls allocation from the borrowed view for the
Arena's live range.

Conflicting direct use of the backing view is invalid while the Arena owns the
borrow.

Example:

```sec
let mut arena := Arena.FromBuffer(ref mut buffer)

// Conflicting direct access to buffer is invalid here.

arena.Release()

// The borrow has ended.
// Direct use of buffer may be valid again.
```

---

# Empty borrowed Arena

A zero-length backing view may create a valid borrowed Arena.

```sec
let mut arena := Arena.FromBuffer(ref mut emptyBuffer)
```

The Arena:

```text
is live;
has zero capacity;
can perform zero-element allocations;
cannot satisfy an allocation requiring positive storage;
may be Reset;
may be Released.
```

A statically obvious zero-capacity Arena produces an informational diagnostic by
default.

The diagnostic is configurable and may be promoted to a warning.

It is not a semantic error.

---

# Owned fixed Arena

Canonical source form:

```sec
let mut arena := try Arena.WithCapacity(4096)
```

Conceptual declaration:

```sec
fn WithCapacity(capacity: uint) Result[Arena, AllocationError]
```

`WithCapacity`:

```text
creates a new ArenaDomain;
requests owned backing from the active target/profile provider;
creates fixed capacity;
does not grow;
may fail;
returns Result;
releases owned backing when the Arena is released or destroyed.
```

`WithCapacity` is available only when the active CompilationPlan supplies a
compatible backing provider.

The provider may be:

```text
a core helper;
a target intrinsic;
an operating-system service;
a platform allocator;
a statically selected memory pool;
another verified provider.
```

The provider contract must expose all effects and trust provenance required by
effect analysis and the call graph.

---

# Growable Arena

Sec 0.1 defines growable Arena semantics.

Canonical source form when the constructor is available:

```sec
let mut arena := try Arena.Growable(4096)
```

Conceptual declaration:

```sec
fn Growable(initialCapacity: uint) Result[Arena, AllocationError]
```

The initial compiler implementation may defer the public `Growable`
constructor.

The semantic model must still support growable compiler-managed or
target-provided allocation contexts.

A growable Arena may acquire additional backing only through a strategy that
does not relocate any existing live allocation.

Permitted strategies include:

```text
additional stable segments;
reserved virtual address space;
target-provided non-relocating extension;
another profile-defined stable strategy.
```

This strategy is forbidden:

```text
allocate a larger buffer;
copy prior Arena allocations;
change their addresses;
continue using prior references.
```

Growth must preserve:

```text
ArenaDomain identity;
current validity epoch;
all prior allocation addresses;
all prior allocation bounds;
all prior reference validity;
all prior ownership facts.
```

---

# Growth policy

The growth policy is selected by:

```text
constructor;
target profile;
CompilationPlan;
compiler-managed allocation-context policy.
```

It must be fixed for the live ArenaDomain.

The policy may include:

```text
initial capacity;
next-segment policy;
maximum capacity;
provider;
alignment constraints;
address-space constraints.
```

Exact public syntax for a maximum capacity is not required by Sec 0.1.

A bounded growable Arena becomes effectively fixed when its maximum capacity has
been acquired.

---

# Capacity

Arena capacity is measured in bytes.

Conceptual quantitative facts include:

```text
reserved capacity;
used capacity;
current-segment capacity;
total growable capacity;
maximum capacity;
alignment padding;
peak demand.
```

The rulebook does not require public properties for every quantitative fact.

Public `Capacity`, `Used`, or `Remaining` members are not introduced here.

Compiler-known member naming belongs to `compiler_known_members.md`.

---

# Typed count

`Arena.Alloc[T](count)` interprets `count` as the number of `T` elements.

It is not a byte count.

Example:

```sec
let values := try arena.Alloc[int32](100)
```

The requested payload is conceptually:

```text
CheckedMultiply(100, SizeOf(int32))
```

The required start offset is aligned according to `T`.

Alignment padding consumes Arena capacity.

The complete end offset is conceptually:

```text
alignedOffset = AlignUp(currentOffset, AlignOf(T))
payloadSize   = CheckedMultiply(count, SizeOf(T))
endOffset     = CheckedAdd(alignedOffset, payloadSize)
```

A fixed Arena can satisfy the allocation when:

```text
endOffset <= capacity
```

---

# Alignment

Every Arena allocation respects the layout and alignment of `T` for the active
CompilationPlan.

Alignment comes from the canonical layout model.

An Arena implementation must not assume that:

```text
SizeOf(T) == AlignOf(T);
all target alignments are powers of two without target proof;
the current cursor is already aligned;
all backing bases satisfy every possible alignment.
```

`Arena.FromBuffer` is valid only for allocations whose alignment can be
satisfied by the backing view and cursor arithmetic.

An allocation that cannot satisfy required alignment returns
`AllocationError`.

A statically impossible alignment produces a compile-time error.

---

# Safe typed allocation

The initial safe operations are:

```sec
let value := try arena.New[Value]()
let values := try arena.Alloc[Value](count)
```

Conceptual declarations:

```sec
fn New[T]() Result[ref mut T, AllocationError]

fn Alloc[T](
    count: uint,
) Result[ref mut T[], AllocationError]
```

These are instance methods.

The implicit Arena receiver is mutated.

The call requires mutable authority over the Arena owner for the duration of the
operation.

The temporary mutable borrow of Arena control state ends when the operation
returns.

The returned reference or slice retains a storage dependency on the
ArenaDomain.

It does not retain an exclusive borrow of the complete Arena control value.

Repeated allocation is therefore permitted when capacity remains available.

---

# Type requirements

Safe `New[T]` and `Alloc[T]` require:

```text
T has complete layout for the active CompilationPlan;
T is sized;
T has known alignment;
T has a valid compiler-defined default;
T is safely initializable;
T is trivially destructible.
```

Exact layout semantics belong to `layout.md`.

Exact default semantics belong to `default_values.md`.

Exact trivial-destruction classification belongs to the destruction and
copy/move rulebooks.

---

# `Arena.New[T]`

`New[T]` allocates storage for exactly one `T`.

It:

```text
checks layout;
checks alignment;
checks capacity or growth;
fully initializes one T;
returns ref mut T;
never exposes uninitialized T;
returns AllocationError on allocation failure.
```

The returned reference:

```text
is non-null;
is bounded to one T;
belongs to the current ArenaDomain;
belongs to the current validity epoch;
has mutable authority;
does not own the backing storage;
does not individually deallocate its storage.
```

---

# `Arena.Alloc[T]`

`Alloc[T](count)` allocates storage for exactly `count` `T` elements.

It:

```text
checks count arithmetic;
checks layout;
checks alignment;
checks fixed capacity or performs valid growth;
fully initializes every element;
returns ref mut T[];
never returns a shorter slice;
never returns a partially initialized slice;
returns AllocationError on allocation failure.
```

The returned slice:

```text
has element count equal to count;
has bounds equal to the complete allocation;
belongs to the current ArenaDomain;
belongs to the current validity epoch;
has mutable authority;
does not own the Arena backing;
does not individually deallocate its storage.
```

---

# Zero-element allocation

This is valid:

```sec
let values := try arena.Alloc[int](0)
```

It:

```text
succeeds;
returns a valid empty mutable slice;
consumes no capacity;
requires no growth;
does not create a dereferenceable element;
does not advance the epoch;
normally produces no diagnostic.
```

Zero-element allocation is common in generic code.

A literal zero count is not inherently suspicious.

A general useless-operation diagnostic may apply when the result and operation
are provably meaningless, but that is not an Arena-specific rule.

---

# Zero-sized types

The canonical layout rulebook owns whether user-visible zero-sized types exist.

When `SizeOf(T) == 0` is permitted:

```text
Alloc[T](count) may consume no payload bytes;
the result still has count elements semantically;
the implementation must preserve bounds and element identity semantics;
no dereferenceable byte address is implied merely by the count.
```

This rulebook does not require zero-sized user types in Sec 0.1.

---

# Full initialization

Safe Arena allocation always exposes fully initialized typed values.

Full initialization means:

```text
every value satisfies the semantic default rules for T;
every readable field or element has a valid value;
all type invariants required by the default are established.
```

It does not mean:

```text
every physical byte is zero;
padding bytes have a defined value;
all targets use bulk memset;
all representations have an all-zero default.
```

The compiler may initialize through:

```text
bulk zeroing where valid;
field stores;
element loops;
vectorized stores;
target intrinsics;
elided stores after proof.
```

Padding remains governed by layout, ABI, unsafe, and FFI rules.

---

# Default initialization failure

The compiler-defined default used by parameterless `New[T]` and `Alloc[T]`
must be infallible.

Allocation may fail.

Canonical default initialization for an accepted `T` may not introduce a second
independent fallible construction protocol.

A type without a valid canonical default is rejected by these allocation forms.

A later explicit initializer or placement-construction API requires a separate
rulebook.

---

# Trivial destruction restriction

Safe `New[T]` and `Alloc[T]` initially require `T` to be trivially destructible.

Reason:

```text
Reset reclaims storage in bulk;
Release terminates the ArenaDomain;
the Arena is not an implicit registry of arbitrary destructors.
```

Supporting arbitrary non-trivial `T` directly would require Arena metadata such
as:

```text
destructor callable;
element count;
element type;
initialization state;
destruction order;
partial-construction state.
```

Sec 0.1 does not require such a runtime registry.

Types owning external resources, custom destruction, locks, files, or owning
containers are not accepted directly by safe `New[T]` or `Alloc[T]` unless they
are classified as trivially destructible by the canonical rules.

---

# Owning containers and Arena backing

The trivial-destruction restriction does not prevent an owning container from
using Arena-backed raw or element storage.

The responsibilities remain distinct:

```text
Arena:
    owns or controls the storage domain

owning container:
    owns initialized elements
    tracks element count
    runs required destruction
    ends its storage dependency before Arena Reset or Release
```

An owning container may use Arena backing only when its own rulebook defines the
ownership and destruction protocol.

The Arena does not silently take over element ownership.

---

# Uninitialized storage

General safe uninitialized typed Arena allocation is not part of Sec 0.1.

An operation must not expose uninitialized storage as:

```text
ref mut T
ref mut T[]
```

A future API must use:

```text
an explicit uninitialized-memory type;
or
an explicit unsafe operation.
```

The API must prove initialization before producing an ordinary safe reference.

Raw byte storage used internally by core or compiler lowering is not ordinary
safe typed allocation.

---

# Allocation failure

Arena allocation failure uses:

```sec
AllocationError
```

The exact error variants are defined by the canonical core/error rules.

Arena creation or allocation must not silently:

```text
return null;
return an invalid reference;
return a shorter slice;
expose partial initialization;
fall back to an unrelated heap;
select another Arena;
panic merely because capacity is exhausted;
terminate the process.
```

The caller handles or propagates the failure through normal `Result` and `try`
rules.

---

# Stable source result type

`New[T]` and `Alloc[T]` return `Result` in their stable source contract.

A compiler proof that capacity is sufficient may eliminate the failure path in
Semantic IR or lowered code.

The proof does not change the source-level method type.

This keeps:

```text
generics;
interfaces;
function values;
module metadata;
callable contracts;
separate compilation
```

stable.

---

# Atomic allocation

An allocation is atomic from the Sec program's perspective.

Conceptual sequence:

```text
validate Arena state;
validate T;
compute checked payload size;
compute alignment padding;
check fixed capacity or acquire complete growth;
reserve the complete range;
fully initialize every value;
publish the reference or slice.
```

If the operation fails before publication:

```text
the Arena remains live;
the ArenaDomain is unchanged;
the validity epoch is unchanged;
the allocation cursor is unchanged;
all prior allocations remain valid;
no partial slice is returned;
no partially initialized value becomes observable.
```

---

# Growable allocation failure

When a growable Arena needs another segment and the provider fails:

```text
the allocation returns Err;
the prior Arena state remains physically unchanged;
existing segments remain linked and valid;
existing allocations remain valid;
no partial segment becomes observable;
the current epoch remains unchanged.
```

A newly acquired segment becomes part of the Arena only after the acquisition
has completed successfully.

---

# Repeated allocation

Allocating again from an Arena does not invalidate previous allocations.

Example:

```sec
let first := try arena.Alloc[byte](100)
let second := try arena.Alloc[byte](200)
```

When both succeed:

```text
first remains valid;
second remains valid;
their positive-sized storage ranges do not overlap;
both belong to the same ArenaDomain;
both normally belong to the same epoch.
```

The compiler may use non-overlap facts for alias analysis.

---

# No individual reclamation

The end of an individual reference's live range does not reclaim its Arena
bytes.

Example:

```sec
let first := try arena.Alloc[byte](100)
Use(first)

// The reference may be dead here.
// The first 100 bytes remain consumed in the current epoch.

let second := try arena.Alloc[byte](200)
```

Arena storage is reclaimed by:

```text
Reset;
Release;
another future explicitly defined reclamation operation.
```

Sec 0.1 has no individual `Free` operation for Arena allocations.

---

# Reset

Canonical source form:

```sec
arena.Reset()
```

Conceptual declaration:

```sec
fn Reset() void
```

`Reset`:

```text
keeps the Arena owner alive;
keeps the ArenaDomain identity;
ends the current validity epoch;
invalidates prior Arena allocations;
resets allocation cursors;
retains reusable backing;
starts a new validity epoch;
permits later allocation.
```

Reset is not Release.

---

# Reset requirements

Reset is valid only when no validity-preserving dependency crosses the reset
point.

This includes:

```text
owned values stored in the Arena;
ordinary shared references;
ordinary mutable references;
slices;
nested Arenas;
containers using Arena backing;
closure captures;
task captures;
thread arguments;
deferred operations;
foreign-retained dependencies;
strong handles;
returned values that remain live.
```

The compiler uses non-lexical lifetime analysis.

A binding may remain lexically in scope when its last use has passed.

Valid:

```sec
let values := try arena.Alloc[int](100)
Process(values)

// Last use has passed.
arena.Reset()
```

Invalid:

```sec
let values := try arena.Alloc[int](100)
arena.Reset()
Process(values)
```

---

# Reset and weak handles

Weak or explicitly stale-capable handles may survive Reset.

After Reset, resolution fails according to the handle contract.

Such a handle does not block Reset merely because the identity value remains
live.

A strong validity-preserving handle blocks Reset.

Canonical rule:

```text
A dependency blocks Reset when its contract requires storage validity to
continue.

A fallibly resolved stale-capable identity does not block Reset.
```

---

# Reset epoch

Reset advances the logical validity epoch.

The epoch belongs to the ArenaDomain.

Ordinary references from the old epoch must be dead.

Weak and stale-capable handles may retain the old expected epoch and fail
resolution.

Epoch increments are checked.

They must never wrap and revive stale references.

At epoch exhaustion the compiler/runtime strategy must:

```text
safely rekey the ArenaDomain;
or
produce deterministic panic/target trap.
```

A finite-width epoch may never silently wrap.

The default logical epoch width follows `reference_model.md`.

Runtime epoch metadata may be eliminated when proof makes it unnecessary.

---

# Reset capacity behavior

## Fixed Arena

Reset:

```text
sets used capacity to zero;
retains total capacity;
retains backing;
retains the ArenaDomain;
advances epoch.
```

## Borrowed Arena

Reset:

```text
sets the cursor to the beginning of the borrowed view;
retains the exclusive borrow;
does not return the view to its owner;
does not zero backing bytes;
advances epoch.
```

## Growable Arena

Reset:

```text
retains all currently acquired segments;
resets all segment cursors;
makes the reserved capacity reusable;
does not automatically trim segments;
does not return individual segments;
advances epoch.
```

A future `Trim` operation requires separate semantics.

---

# Reset does not zero storage

Reset does not require clearing every backing byte.

After Reset, prior typed values no longer exist.

A later safe typed allocation must initialize its new values before exposure.

The compiler may choose to zero storage when:

```text
the target benefits;
the type default permits it;
security policy requires it;
debug policy requests it.
```

Zeroing is an implementation strategy, not the base Reset semantics.

---

# Reset atomicity

Reset is atomic from the program's perspective.

Conceptual ordering:

```text
verify Reset is permitted;
prepare a distinguishable next epoch;
end the old epoch;
reset all allocation cursors;
publish the next epoch.
```

No code may observe:

```text
partially reset segments;
reused storage with the old epoch;
a mixture of old and new allocation state.
```

Concurrent observation is already forbidden for ordinary Arena.

---

# Release

Canonical source form:

```sec
arena.Release()
```

Conceptual declaration:

```sec
fn Release() void
```

`Release` is a consuming terminal operation.

It:

```text
consumes the Arena owner;
terminates the ArenaDomain;
invalidates validity-preserving dependencies;
returns owned backing to its provider;
ends borrowed backing access;
permits no later Arena use.
```

After Release, the source Arena value is consumed.

These are invalid:

```sec
arena.Reset()
discard try arena.New[int]()
discard try arena.Alloc[byte](100)
arena.Release()
```

when performed on the consumed value.

---

# Release requirements

Ordinary safe dependencies may not cross Release.

Invalid:

```sec
let values := try arena.Alloc[int](100)
arena.Release()
Process(values)
```

Weak stale-capable handles may remain as non-resolving identities.

A future Arena reusing the same physical address receives a new ArenaDomain.

Old handles must never resolve to the new domain merely because addresses match.

---

# Release by backing kind

## Owned backing

Release returns all owned backing segments to their provider.

## Borrowed backing

Release ends the exclusive borrow.

It does not deallocate or destroy the borrowed backing owner.

## Static backing

Release ends the ArenaDomain.

It does not deallocate static storage.

## Target-provided backing

Release follows the target provider contract.

It ends the ArenaDomain even when the target retains the physical bytes.

---

# Implicit Arena destruction

The programmer does not need to call `Release()` at every normal scope exit.

When a still-owned Arena reaches its normal destruction boundary, Arena
destruction performs terminal Release semantics.

This applies to:

```text
normal function return;
early return through Result propagation;
normal scope exit;
destruction of an owning field;
task or thread cleanup;
another canonical ownership boundary.
```

If explicit `Release()` already consumed the Arena, no second implicit Release
is generated.

Double Release is invalid.

---

# Destruction versus value destruction

Arena storage ownership and value ownership are distinct.

```text
Arena:
    controls raw storage

value:
    owns its fields, elements, and external resources

reference:
    borrows an initialized value
```

Destroying an ordinary value does not necessarily reclaim its Arena bytes.

Reset or Release does not replace ordinary value destruction.

The safe initial Arena allocation restriction avoids directly placing
non-trivially-destructible values under an implicit bulk-reclamation protocol.

---

# Early return

Return expressions are evaluated before local destruction.

A returned value must not depend on an Arena that is destroyed during function
exit.

Invalid:

```sec
fn Invalid() Result[ref int, AllocationError] {
    let mut arena := try Arena.WithCapacity(1024)
    let value := try arena.New[int]()
    return Ok(value)
}
```

The compiler must not repair this by silently promoting the Arena.

An Arena-backed reference may escape only when the Arena owner already exists
outside the callable and outlives the result.

---

# Returning an Arena

Returning the Arena owner itself is valid when no invalid internal reference is
returned with it.

Example:

```sec
fn CreateArena() Result[Arena, AllocationError] {
    let arena := try Arena.WithCapacity(4096)
    return Ok(<- arena)
}
```

Ownership moves to the result.

The same ArenaDomain continues.

---

# Self-referential Arena results

A local Arena and a reference into that Arena are not automatically bundled into
an ordinary self-referential result.

Example:

```sec
type ArenaResult struct {
    arena: Arena
    value: ref int
}
```

This shape requires explicit ownership and self-reference semantics not defined
by Sec 0.1 Arena.

The compiler must not infer such a relationship merely from field order.

---

# Nested Arenas

Nested Arenas require no special syntax.

Example:

```sec
let childBuffer := try parent.Alloc[byte](4096)
let mut child := Arena.FromBuffer(childBuffer)
```

The child:

```text
has its own ArenaDomain;
has its own epoch;
borrows backing from a parent allocation;
depends on the parent ArenaDomain and parent epoch.
```

The parent cannot Reset or Release while the child remains live.

Releasing the child:

```text
ends the child ArenaDomain;
ends the child borrow;
does not individually reclaim childBuffer from the parent.
```

A later parent Reset may reclaim the parent allocation.

---

# `defer`

Arena operations in `defer` follow canonical LIFO cleanup semantics.

A deferred use of Arena-backed storage must execute before deferred Release.

Example concept:

```sec
let mut arena := try Arena.WithCapacity(4096)

defer {
    arena.Release()
}

let values := try arena.Alloc[int](100)

defer {
    FinalUse(values)
}
```

LIFO order is:

```text
FinalUse(values)
Arena.Release()
```

A cleanup order that releases the Arena before a later dependent cleanup use is
invalid.

Ordered Arena effects and the call graph must preserve the cleanup order.

---

# Panic

Arena follows the canonical panic model.

## Cleanup-capable panic path

When the active CompilationPlan performs cleanup during panic:

```text
defer bodies execute according to canonical order;
owned values are destroyed;
owned Arenas are released unless ownership was transferred;
borrowed Arena access ends.
```

## Immediate trap or abort

When panic becomes an immediate target trap or abort:

```text
Arena destruction is not guaranteed;
owned backing may not be explicitly returned;
the current execution context does not continue.
```

Code requiring guaranteed cleanup must use a profile and guarantee compatible
with that requirement.

Arena does not introduce a universal unwinder.

---

# Cancellation

A cancellation request is not task completion.

A parent may not Reset or Release storage merely because cancellation has been
requested.

The parent must reach a semantic completion boundary proving that the child:

```text
has stopped executing;
has completed required cleanup;
can no longer access captured Arena storage.
```

When cancellation performs normal task cleanup:

```text
task-local Arenas are destroyed;
moved Arena ownership is released unless transferred to the result;
captured Arena dependencies end at task completion.
```

A hard-termination model that skips required cleanup is governed by the
cancellation and target-profile rules.

---

# Arena and tasks

Ordinary Arena is not implicitly concurrent.

A spawned task does not automatically inherit the parent's mutable allocation
context.

This would create hidden concurrent mutation of Arena state.

A task receives allocation capability through:

```text
a task-specific target/compiler context;
an Arena explicitly moved into the task;
another explicit task allocation contract;
or no allocation context.
```

---

# Borrowed Arena storage captured by a task

Example:

```sec
let values := try arena.Alloc[int](100)
let task := spawn Process(values)
```

The task retains a dependency on:

```text
the ArenaDomain;
the allocation epoch;
the allocation bounds;
the captured authority.
```

The parent cannot Reset or Release the Arena while task execution may access the
storage.

---

# Task completion proof

Physical completion timing does not determine static validity.

A parent needs a semantic completion boundary such as:

```text
await;
join;
structured-concurrency scope completion;
another statically proven completion event.
```

Runtime observation that a task probably finished is insufficient.

---

# `await`

`await`:

```text
proves task execution completion;
ends execution-local task captures;
establishes canonical memory visibility;
transfers result dependencies to the continuation;
consumes Task[T] according to await rules.
```

Example:

```sec
let task := spawn Process(values)
discard await task

arena.Reset()
```

Reset may be valid after `await` when no dependency was transferred through the
task result and no other dependency remains.

`await` does not call the task body again.

---

# `join`

`join` proves task completion while preserving the task handle.

Execution-local Arena dependencies may end at join.

The retained handle does not itself keep borrowed Arena storage live unless:

```text
the handle stores a result with such a dependency;
the handle contract explicitly preserves such a dependency.
```

---

# Task result dependencies

Completion ends execution-local dependencies.

Dependencies transferred through the task result remain live.

Example concept:

```sec
let task := spawn Find(values)
let found := await task

Use(found)

arena.Reset()
```

When `found` refers into the Arena, Reset is valid only after the last use of
`found`.

Task completion alone does not erase result dependencies.

---

# Moving an Arena into a task

Example:

```sec
let task := spawn Worker(<- arena)
```

After spawn:

```text
the parent no longer owns the Arena;
the parent cannot use the Arena;
the task owns the Arena.
```

At task completion:

```text
the Arena is destroyed in the task when still local;
or
ownership is moved into the task result.
```

If returned:

```sec
fn Worker(arena: Arena) Arena {
    return <- arena
}
```

and later awaited:

```sec
let arena := await task
```

the receiving owner controls the same ArenaDomain and current epoch.

---

# Task completion point

A task completion event occurs after:

```text
the task body has ended;
the result has been moved into outcome storage;
required defer bodies have completed;
required local destruction has completed;
task-owned Arenas have been released unless transferred;
execution can no longer access captured storage.
```

`await`, `join`, and structured concurrency observe this semantic point.

---

# Arena and threads

The same dependency rules apply to threads.

Additional thread requirements include:

```text
transferability;
thread-safety;
memory synchronization;
target thread support;
address-space compatibility.
```

A thread may borrow Arena-backed storage only when the compiler proves:

```text
storage outlives thread execution;
access authority is valid;
mutable access is exclusive;
the Arena is not concurrently allocated from;
the Arena is not Reset or Released;
the target memory model permits the transfer.
```

---

# Thread completion

The canonical thread join/completion operation:

```text
proves thread execution ended;
establishes required memory synchronization;
ends thread-local borrowed dependencies;
permits Reset or Release when no dependency was returned.
```

Physical completion without the semantic boundary is insufficient.

---

# Moving an Arena into a thread

An Arena may be moved into a thread when:

```text
the backing is transferable;
the provider permits transfer;
the target profile permits transfer;
no parent dependency conflicts;
the Arena does not depend on immovable thread-local state.
```

The thread owns and destroys the Arena unless ownership is transferred through
its result.

---

# Structured concurrency

A structured-concurrency scope may end child Arena dependencies when the
canonical scope semantics guarantee:

```text
all relevant children completed;
required cleanup completed;
no detached child retained the dependency;
no result transferred the dependency out of the scope.
```

A detached child prevents the scope from acting as a completion proof for its
captured Arena storage.

---

# Concurrent Arena access

Ordinary Arena does not support:

```text
concurrent New or Alloc;
parent allocation while child allocation occurs;
concurrent Reset;
concurrent Release;
Reset concurrent with storage access;
Release concurrent with storage access.
```

Ownership and borrow analysis reject these cases.

A future synchronized or concurrent Arena is a separate type or explicit
implementation.

It must not silently change ordinary Arena semantics.

---

# Allocation context

Every callable invocation has zero or one active allocation context.

The context is compiler-visible semantic state.

It is not:

```text
a source-level global;
an automatically visible Arena variable;
a universal thread-local allocator;
an implicit heap;
a mandatory runtime object.
```

---

# `MayAllocate` versus `RequiresAllocationContext`

These facts are distinct.

## `MayAllocate`

A summary effect indicating that reachable execution may perform an allocation
operation.

## `RequiresAllocationContext`

A callable requirement indicating that the callable contains or reaches an
implicit allocation operation that needs an active context.

Example:

```sec
fn Fill(arena: ref mut Arena) Result[ref mut byte[], AllocationError] {
    return arena.Alloc[byte](1024)
}
```

This function may allocate through an explicit Arena.

It need not require an ambient allocation context.

A function using an allocating string or collection operation without an
explicit Arena may require the ambient context.

---

# Synchronous propagation

A synchronous call propagates the active allocation context when the callee
requires it.

Conceptually:

```text
A(context X)
    -> B(context X)
        -> C(context X)
```

A function that does not require the context receives no mandatory runtime
parameter.

---

# Explicit Arenas are not ambient candidates

The compiler must not choose among ordinary Arena values merely because they are
in lexical scope.

Example:

```sec
let temporaryArena := ...
let outputArena := ...

let result := try Build()
```

The compiler must not guess which Arena should become ambient.

An explicit Arena argument is an ordinary value and does not automatically
rebind the ambient context.

---

# Allocation-context selection

For an allocating operation, selection follows this semantic order:

```text
explicit Arena selected by the operation;
propagated ambient allocation context;
compiler-managed local Arena with proven backing and non-escape;
target-provided context;
no context, producing a compile-time error.
```

The decision is made before physical lowering.

The backend must not change allocation origin or failure semantics.

---

# Compiler-managed local Arena

A compiler-managed local Arena is permitted only when:

```text
a concrete backing strategy exists;
the required lifetime is proven;
non-owning references do not escape;
failure semantics remain correct;
the selected profile permits it.
```

Possible backing includes:

```text
bounded stack storage;
static storage;
caller-context-backed stable storage;
target-provided local storage.
```

The compiler must not silently move escaping local storage into an Arena.

---

# Escaping results

An owning result may allocate through a caller-propagated context when its
owning type and storage model permit it.

A compiler-local Arena may not back an escaping non-owning reference.

Invalid conceptual behavior:

```text
create compiler-local Arena;
allocate temporary storage;
return ref into that storage;
destroy Arena.
```

The compiler rejects the escape.

---

# Spawned allocation contexts

A spawned task or thread is a new execution context.

It does not automatically receive the parent's mutable Arena context.

Its context comes from:

```text
task/thread profile;
target-provided context;
explicitly transferred Arena;
or absence of allocation capability.
```

A process receives a separate address-space allocation context.

An interrupt root has no allocation context unless the target/profile
explicitly supplies one and ISR rules permit its use.

---

# Foreign entrypoints

Foreign code does not automatically supply a Sec allocation context.

An exported Sec callable requiring one needs:

```text
a generated wrapper selecting a target-provided context;
an explicit Sec-aware ABI contract;
or rejection of the export.
```

A hidden Sec allocation-context parameter must not be added to an ordinary
foreign ABI without an explicit contract.

---

# Effect classification

Arena operations contribute ordered effects:

```text
ArenaCreate(A, capacity)
ArenaAllocate(A, size)
ArenaReset(A)
ArenaRelease(A)
```

Order matters.

These events are associated with one ArenaDomain.

They are not reduced to one unordered boolean.

---

# Summary effects

## `Arena.FromBuffer`

Contributes:

```text
ArenaCreate
```

It does not contribute `MayAllocate` merely for creating the view.

## `Arena.WithCapacity`

Contributes:

```text
ArenaCreate
MayAllocate
```

It also includes effects of the backing provider.

## `Arena.Growable`

Contributes:

```text
ArenaCreate
MayAllocate
```

Growth later contributes allocation/provider effects.

## `Arena.New` and `Arena.Alloc`

Contribute:

```text
ArenaAllocate
MayAllocate
```

This remains true when capacity already exists.

## `Arena.Reset`

Contributes:

```text
ArenaReset
```

## `Arena.Release`

Contributes:

```text
ArenaRelease
```

It may also include backing-provider effects.

---

# `@noAlloc`

Arena allocation is allocation for the canonical effect model.

Therefore `@noAlloc` forbids reachable `Arena.New` and `Arena.Alloc` unless the
effect rulebook later defines a narrower guarantee.

Creating a borrowed Arena view through `FromBuffer` does not itself imply
`MayAllocate`.

This rulebook does not introduce new effect attributes such as
`@noBackingAlloc`.

---

# Ordered-effect validity

The compiler must reject invalid sequences such as:

```text
ArenaRelease(A)
ArenaAllocate(A, size)
```

It must preserve valid ordering such as:

```text
ArenaCreate(A)
ArenaAllocate(A)
ArenaReset(A)
ArenaAllocate(A)
ArenaRelease(A)
```

Defer, destruction, branches, loops, tasks, threads, and implicit operations
participate in ordered-effect analysis.

---

# Call-graph integration

Call graph nodes and call sites retain:

```text
Arena effects;
allocation-context requirement;
Arena-demand summary;
explicit Arena argument identity where known;
task/thread dependency transfer;
destruction and defer operations;
provider calls;
open callable contracts.
```

Synchronous callees propagate Arena effects according to normal call-graph
rules.

Spawned body effects belong to the new execution context.

Task creation effects belong to the spawner.

---

# Arena-demand analysis

The compiler analyzes capacity per ArenaDomain and validity epoch.

Each result is classified as:

```text
Exact
Bounded
Unknown
Unbounded
```

The analysis records:

```text
known capacity;
minimum required capacity;
maximum proven use;
alignment overhead;
worst-case path;
contributing allocation sites;
unknown call or loop causes.
```

---

# Sequential demand

Sequential allocations in one epoch accumulate.

The end of a reference live range does not subtract its bytes.

Example:

```sec
discard try arena.Alloc[byte](100)
discard try arena.Alloc[byte](200)
```

The demand is at least:

```text
100 + 200 + alignment padding
```

---

# Branch demand

Mutually exclusive branches combine by maximum when they start from the same
Arena state.

Example:

```sec
if condition {
    discard try arena.Alloc[byte](100)
} else {
    discard try arena.Alloc[byte](500)
}
```

The branch contribution is:

```text
max(100, 500)
```

A later allocation is added to the maximum continuing state.

---

# Loop demand

A bounded loop without Reset multiplies per-iteration demand.

The compiler may use:

```text
constant bounds;
range contracts;
compile-time values;
proven upper bounds.
```

A valid Reset per iteration separates epochs.

The peak then becomes the maximum per-iteration demand rather than the sum
across all iterations.

An unknown loop with accumulating allocation produces unknown or unbounded
demand.

---

# Recursion demand

Recursive Arena demand uses the same-stack call graph.

A proven recursion bound may produce bounded demand.

Without a bound:

```text
allocation per recursive depth may become Unknown or Unbounded.
```

Spawn recursion is analyzed as concurrent resource creation, not same-stack
Arena accumulation.

---

# Function summaries

A callable may expose Arena-demand summaries for:

```text
each explicit Arena parameter;
the ambient allocation context.
```

Initial summary forms may support:

```text
constant;
sum;
maximum;
constant multiplication;
range upper bound;
unknown;
unbounded.
```

Sec 0.1 does not require a general theorem prover.

---

# Indirect calls

For a closed target set, Arena demand uses the maximum possible target demand.

For an open callable contract:

```text
use the declared Arena bound when present;
otherwise classify the contribution as Unknown.
```

A missing bound may be accepted in a permissive hosted profile.

It blocks proof in a strict bounded-memory profile.

---

# Growable demand

Growable Arenas still receive logical demand analysis.

The report may distinguish:

```text
initial capacity;
maximum proven logical demand;
minimum required growth;
maximum configured capacity;
provider failure possibility.
```

Growability does not make capacity analysis irrelevant.

---

# Statically impossible allocation

When an allocation can never succeed and source uses `try`, the success
continuation is unreachable.

Because proven dead code is an error in Sec, this is a compile-time error.

Example:

```sec
let mut arena := try Arena.WithCapacity(16)
let values := try arena.Alloc[int64](10)
```

A mentor diagnostic reports:

```text
required bytes;
available bytes;
element count;
element type;
alignment;
active CompilationPlan.
```

Explicit exhaustion testing remains valid when the caller handles the `Result`
directly.

Normal unreachable-branch rules still apply to a provably impossible `Ok`
branch.

---

# Proven sufficient capacity

When the compiler proves capacity and arithmetic safety, it may eliminate:

```text
runtime capacity branch;
overflow branch;
AllocationError construction;
dynamic offset computation.
```

The source method type remains `Result`.

The success-only proof is represented in Semantic IR and lowering.

---

# Zero-capacity diagnostic

A statically explicit zero-capacity Arena produces configurable information.

Examples include:

```sec
let arena := try Arena.WithCapacity(0)
```

and a statically known empty backing view.

Suggested message:

```text
information: Arena is created with zero capacity

the Arena cannot satisfy an allocation requiring storage

help:
    provide positive capacity
    or retain zero capacity when testing exhaustion behavior
```

No diagnostic is required when capacity may be zero only at runtime.

---

# Semantic IR requirements

Arena semantics remain explicit in Semantic IR until:

```text
ownership is verified;
borrows and dependencies are verified;
Reset and Release ordering is verified;
task/thread completion is verified;
effects are inferred;
capacity planning is complete;
target strategy is selected.
```

LLVM or MLIR lowering must not invent Arena semantics.

---

# Semantic IR concepts

Semantic IR distinguishes:

```text
Arena owner state;
ArenaDomain identity;
validity epoch;
allocation context;
backing kind;
growth policy;
typed allocation;
dependency;
ordered effect;
failure path.
```

Arena state version and validity epoch are different.

Allocation changes state version but not epoch.

Reset changes both state version and epoch.

Release consumes state and ends the ArenaDomain.

---

# Required Semantic IR operations

Semantic IR must represent at least:

```text
ArenaCreateBorrowed
ArenaCreateOwnedFixed
ArenaCreateGrowable
ArenaNew
ArenaAlloc
ArenaReset
ArenaRelease
ArenaDestroy
```

It must also represent:

```text
allocation-context propagation;
task/thread ownership transfer;
task/thread Arena dependency;
await/join completion;
result dependency transfer;
provider invocation;
try success and failure flow.
```

Exact implementation names may differ.

The semantic distinctions may not be erased prematurely.

---

# SSA Arena state

Mutating Arena operations use SSA state versions.

Conceptually:

```text
arena1 = create
arena2, result1 = allocate(arena1)
arena3, result2 = allocate(arena2)
arena4 = reset(arena3)
release(arena4)
```

One input owner is consumed.

Exactly one continuing owner state exists after each non-terminal operation.

Release produces no continuing Arena owner.

---

# Allocation failure in SSA

An allocation operation produces:

```text
one continuing Arena state;
one Result value.
```

On success, the state contains the advanced cursor or new segment.

On failure, the state is physically equivalent to the input state.

The input owner is still consumed so that two usable Arena control values do
not exist.

Error propagation cleanup uses the continuing Arena state.

---

# Reference provenance in Semantic IR

A successful Arena allocation records:

```text
ArenaDomain;
validity epoch;
element type;
bounds;
mutability authority;
storage origin;
allocation site.
```

Allocation does not change the epoch.

Reset creates a new epoch.

Release ends the domain.

This metadata may live in:

```text
SSA types;
operation attributes;
analysis side tables;
symbol metadata;
a combination.
```

It need not always exist in runtime representation.

---

# Sec MLIR path

The intended lowering path is:

```text
Sec source
    ↓
Sec Semantic IR
    ↓
high-level Sec MLIR dialect
    ↓
Arena planning and specialized Sec lowering
    ↓
standard MLIR dialects
    ↓
LLVM dialect and target code
```

Arena semantics must not be lowered directly to ordinary allocation operations
before Sec analyses have completed.

---

# Sec and MLIR region terminology

Sec compiler-internal storage identities should use names such as:

```text
ArenaDomain
StorageRegion
LifetimeRegion
```

MLIR `Region` refers to operation/block nesting.

Compiler code and documentation must not use the two meanings ambiguously.

---

# Recommended Sec MLIR types

Conceptual high-level types include:

```text
!sec.arena<backing, growth, profile>
!sec.arena_domain
!sec.alloc_context
!sec.ref<T>
!sec.ref_mut<T>
!sec.slice<T>
!sec.slice_mut<T>
!sec.allocation_error
```

Exact textual syntax is implementation-defined.

The semantic distinctions are required.

---

# Recommended Sec MLIR operations

Conceptual operations include:

```mlir
%arena = sec.arena.create_borrowed %buffer

%arena, %result =
    sec.arena.create_owned_fixed %capacity

%arena, %result =
    sec.arena.create_growable %initial_capacity

%next, %result =
    sec.arena.new %arena

%next, %result =
    sec.arena.alloc %arena, %count

%next =
    sec.arena.reset %arena

sec.arena.release %arena
```

The examples are implementation guidance.

They do not create source syntax.

---

# Multi-result IR

Sec 0.1 source functions return one value.

Semantic IR and MLIR may use multiple SSA results.

Arena allocation naturally produces:

```text
next Arena state;
Result of allocation.
```

This does not violate the source-language single-return rule.

---

# MLIR effects

Sec Arena operations should integrate with MLIR effect infrastructure.

The implementation should provide:

```text
MemoryEffectOpInterface where applicable;
a Sec-specific Arena effect interface;
ArenaDomain-aware resources;
ownership-consumption verification.
```

Standard `Allocate` or `Free` effects alone are insufficient.

`Reset` is not ordinary `Free` because backing remains live and reusable.

A conceptual custom resource may be:

```text
ArenaResource(ArenaDomain)
```

---

# Alias analysis

Two successful positive-sized allocations from one Arena epoch are
non-overlapping.

The compiler may expose this to MLIR alias analysis.

The compiler must not infer overlap merely because both references derive from
one backing base.

The compiler must preserve:

```text
separate bounds;
allocation order;
epoch;
domain identity.
```

Zero-element allocations do not imply a dereferenceable address.

---

# `memref`

MLIR `memref` may represent a typed bounded view after Arena semantics have been
analyzed.

`memref.alloc` is not the canonical high-level representation of
`Arena.Alloc`.

An Arena represents:

```text
one allocation domain;
many suballocations;
shared capacity;
bulk Reset;
bulk Release;
borrowed or segmented backing;
shared validity epoch;
move-only ownership;
task/thread dependencies.
```

Lowering each Arena allocation immediately to independent `memref.alloc` would
lose these semantics.

---

# Physical Arena planning

After verification, the compiler selects a physical strategy such as:

```text
stack-backed fixed Arena;
static-backed fixed Arena;
borrowed descriptor;
owned fixed descriptor;
segmented growable descriptor;
reserved-address-space Arena;
fully scalarized and eliminated Arena.
```

The strategy belongs to one concrete CompilationPlan.

---

# Example physical descriptor

A fixed Arena may lower conceptually to:

```text
base pointer;
capacity;
cursor;
optional epoch.
```

A growable Arena may lower to:

```text
current segment;
segment list or equivalent;
cursor;
capacity;
optional epoch;
provider state.
```

This is not a universal ABI.

Fields may be removed, combined, or represented differently after proof.

---

# Epoch lowering

Logical epoch semantics remain even when runtime epoch storage is absent.

Runtime epoch metadata may be:

```text
eliminated;
stored inline;
stored in a side table;
represented by a domain token;
represented by a target capability.
```

Elimination is valid when the compiler proves that no runtime stale-resolution
mechanism needs it.

---

# Fixed borrowed lowering

`Arena.FromBuffer` may lower conceptually to:

```text
base = buffer.Ptr
capacity = buffer.Len
cursor = 0
domain = fresh logical identity
epoch = initial logical epoch
```

`ArenaAlloc[T](count)` may lower to:

```text
payload = checked count * SizeOf(T)
aligned = checked AlignUp(cursor, AlignOf(T))
end = checked aligned + payload
if end > capacity:
    return AllocationError
cursor = end
initialize T elements
return typed bounded view
```

Proof may remove checks.

---

# Growable lowering

When the current segment cannot satisfy the request:

```text
compute complete request;
request a stable segment;
if provider fails:
    preserve prior state and return Err;
link the complete segment;
allocate from the segment.
```

Provider calls remain visible to call graph, effect analysis, stack analysis,
and trust provenance unless represented by a compiler intrinsic with a complete
summary.

---

# Optimization rules

## Permitted after proof

The compiler may:

```text
fold SizeOf and AlignOf;
precompute offsets;
combine capacity checks;
eliminate proven capacity checks;
eliminate proven overflow checks;
eliminate runtime epoch metadata;
scalar-replace Arena-backed values;
stack-lower a local Arena;
remove an unused zero-element allocation;
remove Reset immediately before Release;
remove the complete Arena descriptor;
inline Arena operations.
```

## Forbidden without proof

The compiler must not:

```text
move allocation across Reset;
move access across Reset or Release;
CSE distinct Arena allocations;
merge different epochs;
replace fixed exhaustion with hidden growth;
fall back to another allocator;
relocate live allocations during growth;
release before deferred dependent use;
share ordinary Arena mutation between tasks;
drop task/thread completion dependencies;
change allocation failure behavior;
change backing ownership.
```

---

# CSE, DCE, and LICM

Arena allocation is not pure.

Common-subexpression elimination must not merge two allocations.

Reset and Release have ownership and effect semantics even when they produce no
ordinary value.

Dead-code elimination may remove them only when the compiler proves that all
semantic effects are unobservable and ownership remains correct.

Loop-invariant code motion must not move allocation, Reset, or Release when the
transformation changes:

```text
allocation count;
failure timing;
capacity demand;
epoch boundary;
resource lifetime.
```

---

# MLIR verification

Local operation verification checks:

```text
operand and result types;
backing-policy compatibility;
count type;
complete T layout;
alignment;
trivial destruction;
result shape;
constructor policy.
```

Global analysis verifies:

```text
no live dependency crosses Reset;
no dependency crosses Release;
no use after move;
no double Release;
task/thread completion;
valid cleanup order;
valid allocation-context propagation;
capacity/profile requirements.
```

Local operation verifiers alone are insufficient.

---

# Compiler pipeline

A recommended implementation pipeline is:

```text
1. Parse ordinary member syntax.
2. Resolve Arena compiler-known members.
3. Validate type, layout, alignment, default, and destruction.
4. Build ownership and borrow facts.
5. Build explicit Semantic IR Arena operations.
6. Expand control flow, try, defer, and destruction.
7. Assign ArenaDomain identities and backing policies.
8. Verify lifetimes and dependencies.
9. Analyze task/thread transfers and completion.
10. Infer ordered and summary effects.
11. Compute call-graph Arena summaries.
12. Compute static capacity demand.
13. Verify target/profile requirements.
14. Emit mentor diagnostics.
15. Generate high-level Sec MLIR.
16. Select physical Arena strategy.
17. Apply proof-driven canonicalization.
18. Lower to standard MLIR dialects.
19. Lower to LLVM dialect and target code.
20. Perform final ownership, effect-order, ABI, and cleanup verification.
```

The exact pass names are implementation-defined.

The semantic order is required.

---

# Target profiles

Arena source semantics remain stable across profiles.

Profiles may differ in:

```text
available constructors;
backing providers;
growth;
capacity proof requirements;
runtime metadata;
failure guarantees;
thread transfer;
ISR use.
```

---

# Hosted profile

A hosted profile normally supports:

```text
Arena.FromBuffer;
Arena.WithCapacity;
growable Arena strategy;
compiler-managed ambient allocation context.
```

Unknown capacity demand may be accepted with information or warning.

Provider failure remains represented unless proven impossible.

---

# Embedded fixed profile

An embedded fixed profile normally uses:

```text
borrowed fixed Arenas;
static backing;
caller-provided backing;
target-provided fixed pools.
```

Owned or growable constructors are target-dependent.

Unknown or unbounded demand is normally an error when the profile requires a
complete capacity proof.

---

# Bare-metal bounded profile

A bare-metal bounded profile may require:

```text
all required capacity statically bounded;
no operating-system backing provider;
no hidden growth;
no mandatory runtime epoch metadata;
no allocation context without explicit backing.
```

The compiler may eliminate all Arena descriptor state when offsets and lifetime
are fully static.

---

# No-allocation profile

A profile may make dynamic allocation unavailable.

`Arena.FromBuffer` may remain available because it does not acquire backing.

`Arena.New` and `Arena.Alloc` still have `MayAllocate` and `ArenaAllocate`
according to the canonical effect model.

A profile or guarantee forbidding allocation may reject them even when backing
already exists.

This rulebook does not redefine `@noAlloc`.

---

# ISR use

Ordinary ISR code must not:

```text
create owned backing;
grow an Arena;
allocate from a shared Arena;
Reset storage visible to interrupted code;
Release storage visible to interrupted code.
```

A target may permit an ISR-exclusive preallocated Arena only when the canonical
ISR analysis proves:

```text
bounded demand;
no blocking;
no suspension;
no conflicting access;
safe Reset boundary;
bounded execution behavior;
target support.
```

---

# FFI

Arena-backed storage passed to foreign code follows FFI retention contracts.

Passing `slice.Ptr` and `slice.Len` for the duration of a synchronous call may
be valid when:

```text
the foreign call does not retain the pointer;
the Arena cannot Reset or Release during the call;
mutability and aliasing match the ABI contract;
address space and alignment are valid.
```

A foreign function that may retain the pointer creates a dependency extending
according to the declared retention contract.

Unknown retention must be treated conservatively.

`RawPtr[T]` does not keep the Arena alive.

---

# Diagnostics

Arena diagnostics are cause-aware mentor diagnostics.

They must retain:

```text
Arena declaration;
ArenaDomain identity where useful;
allocation site;
Reset or Release site;
task/thread capture;
await/join state;
defer/destruction path;
required and available bytes;
alignment;
active CompilationPlan;
call-graph path;
help.
```

---

# Required error categories

Errors include:

```text
Arena copy;
use after move;
use after Release;
double Release;
Reset with live dependency;
Release with live dependency;
allocation from consumed Arena;
returning reference into local Arena;
parent Reset while nested Arena lives;
task/thread dependency crossing Reset or Release;
missing allocation context;
unsupported backing provider;
invalid alignment;
incomplete or unsized T;
missing default;
non-trivially-destructible T;
statically impossible allocation through try;
unbounded demand in a strict profile;
epoch exhaustion without safe handling;
invalid foreign retention;
invalid ISR use.
```

---

# Warning categories

Warnings may include:

```text
unknown peak demand in a permissive profile;
open callable contract without Arena bound;
recursive accumulation with unknown bound;
growable demand above a profile recommendation;
foreign callback retention with conservative unknown lifetime.
```

Warning severity is configurable.

---

# Information categories

Information may include:

```text
statically explicit zero-capacity Arena;
runtime capacity check eliminated;
Arena fully stack-lowered;
runtime epoch metadata eliminated;
Reset immediately before Release;
capacity-utilization report;
Arena descriptor eliminated.
```

Analysis-only facts should normally appear in LSP or explicit compiler reports
rather than ordinary build output.

---

# Diagnostic examples

## Reset blocked by task

```text
error: Arena cannot be reset while a spawned task may access its storage

Arena:
    scratch

allocation:
    values

captured by:
    task created in StartWorkers

completion:
    not proven before Reset

help:
    await or join the task
    use a structured task scope
    or move the Arena into the task
```

## Impossible allocation

```text
error: Arena allocation can never succeed

required:
    80 bytes

available:
    16 bytes

allocation:
    10 elements of int64

help:
    increase capacity
    reduce the element count
    or use a growable Arena
```

## Deferred order

```text
error: deferred Arena release occurs before deferred use of Arena storage

release defer:
    line 14

dependent cleanup:
    line 18

help:
    register Release earlier so dependent cleanup executes first
```

---

# LSP behavior

The LSP consumes compiler-owned Arena analysis.

It must not build a separate Arena model.

Useful LSP features include:

```text
hover showing Arena backing, capacity, epoch, and profile;
go to Arena allocation origin;
find dependencies;
show Reset/Release blockers;
show task/thread captures;
show capacity summary;
show allocation-context origin;
show effect path;
show physical-lowering plan in analysis mode;
show diagnostic cause chain;
code lens for peak demand and utilization.
```

All results belong to one active CompilationPlan and snapshot.

---

# Static capacity reports

A capacity report may show:

```text
Arena:
    scratch

Backing:
    borrowed fixed

Capacity:
    4096 bytes

Maximum proven use:
    2720 bytes

Alignment padding:
    16 bytes

Remaining at peak:
    1376 bytes

Status:
    bounded
```

For unknown demand, the report shows the introducing loop, recursion component,
indirect call, or open callable contract.

---

# Separate compilation

Module metadata must preserve Arena-related callable facts required by callers:

```text
RequiresAllocationContext;
declared and inferred allocation effects;
Arena-demand summary;
explicit Arena parameter summaries;
provider requirements;
task/thread retention contracts;
open callable bounds;
trust provenance.
```

Public callers must not rely on undeclared stronger implementation facts.

Changing a public allocation-context requirement or Arena bound may require
dependent recompilation.

---

# Incremental compilation

Arena analysis invalidation includes changes to:

```text
capacity;
allocation count;
T layout;
T alignment;
T default;
T destruction classification;
loop bound;
control-flow reachability;
callee Arena demand;
call target set;
effect contract;
target profile;
task/thread capture;
Reset or Release location;
provider contract.
```

Stable ArenaDomain and call-site identities should be preserved across unrelated
edits where possible.

---

# Required source tests

Create or update:

```text
arena_from_buffer_valid.sec
arena_with_capacity_valid.sec
arena_growable_valid.sec
arena_new_valid.sec
arena_alloc_valid.sec
arena_zero_length_valid.sec
arena_zero_capacity_valid.sec
arena_reset_valid.sec
arena_release_valid.sec
arena_move_valid.sec
arena_nested_valid.sec
arena_task_borrow_valid.sec
arena_task_move_valid.sec
arena_thread_borrow_valid.sec
arena_thread_move_valid.sec
arena_await_dependency_valid.sec
arena_join_dependency_valid.sec
arena_result_dependency_valid.sec
arena_defer_order_valid.sec
arena_early_return_valid.sec
arena_failure_handling_valid.sec
arena_bounded_loop_valid.sec
arena_branch_capacity_valid.sec
arena_ambient_context_valid.sec
arena_explicit_context_valid.sec
```

---

# Required invalid tests

Create or update:

```text
arena_copy_invalid.sec
arena_use_after_move_invalid.sec
arena_use_after_release_invalid.sec
arena_double_release_invalid.sec
arena_reset_live_ref_invalid.sec
arena_release_live_ref_invalid.sec
arena_return_local_ref_invalid.sec
arena_nested_parent_reset_invalid.sec
arena_task_reset_invalid.sec
arena_thread_release_invalid.sec
arena_missing_context_invalid.sec
arena_nontrivial_type_invalid.sec
arena_incomplete_type_invalid.sec
arena_alignment_invalid.sec
arena_impossible_try_allocation_invalid.sec
arena_unbounded_embedded_invalid.sec
arena_defer_order_invalid.sec
arena_result_reset_invalid.sec
arena_foreign_retention_invalid.sec
arena_isr_invalid.sec
```

---

# Semantic IR tests

Golden tests must cover:

```text
borrowed creation;
owned fixed creation;
growable creation;
New;
Alloc;
zero-element allocation;
allocation failure state;
try propagation;
Reset epoch;
Release consumption;
implicit destruction;
move;
nested Arena;
task capture;
task ownership transfer;
await completion;
join completion;
thread completion;
result dependency transfer;
allocation-context propagation.
```

---

# Sec MLIR tests

Tests must cover:

```text
operation parsing and printing;
type verification;
effect interfaces;
ArenaResource identity;
ownership consumption;
invalid operation sequences;
source locations;
canonicalization;
CSE rejection;
DCE behavior;
LICM barriers;
Reset and Release ordering;
lowering to memref/LLVM views;
provider calls;
epoch elimination.
```

---

# Capacity-analysis tests

Tests must cover:

```text
exact sequential demand;
branch maximum;
branch plus continuation;
constant loop multiplication;
range-bounded loop;
Reset per iteration;
unknown loop;
unbounded recursion;
closed indirect target maximum;
open callable unknown bound;
alignment padding;
overflow;
zero-element allocation;
growable initial capacity;
maximum capacity;
statically impossible allocation;
proven sufficient capacity.
```

---

# Backend tests

At minimum test:

```text
Linux amd64;
Linux arm64;
Linux arm32;
one representative bare-metal target.
```

Verify:

```text
pointer width;
SizeOf;
alignment;
checked multiplication;
cursor arithmetic;
epoch representation;
borrowed backing;
provider calls;
segment growth;
Release;
absence of mandatory runtime dependencies.
```

---

# Optimization tests

Verify that the compiler may:

```text
eliminate a proven capacity check;
eliminate runtime epoch metadata;
stack-lower a local Arena;
fold fixed offsets;
remove redundant Reset before Release;
remove unused zero-element allocation.
```

Verify that it may not:

```text
CSE two allocations;
move allocation across Reset;
move use across Release;
hoist per-iteration allocation incorrectly;
remove required Release;
merge different epochs;
relocate a live allocation.
```

---

# Implementation graph nodes

The implementation graph should contain separate Arena work nodes such as:

```text
ARENA-TYPE
ARENA-COMPILER-KNOWN-MEMBERS
ARENA-CREATE-BORROWED
ARENA-CREATE-OWNED
ARENA-CREATE-GROWABLE
ARENA-TYPED-ALLOCATION
ARENA-DEFAULT-INITIALIZATION
ARENA-RESET
ARENA-RELEASE
ARENA-DESTRUCTION
ARENA-OWNERSHIP
ARENA-LIFETIME
ARENA-EPOCH
ARENA-TASK-DEPENDENCIES
ARENA-THREAD-DEPENDENCIES
ARENA-EFFECTS
ARENA-CALL-SUMMARIES
ARENA-CAPACITY-ANALYSIS
ARENA-SEMANTIC-IR
ARENA-MLIR-DIALECT
ARENA-MLIR-LOWERING
ARENA-DIAGNOSTICS
ARENA-LSP
ARENA-TARGET-PROFILES
ARENA-TESTS
```

Codex must not implement the complete Arena model as one undifferentiated task.

---

# Initial implementation order

Recommended order:

```text
1. Normalize the Arena builtin and remove the unused lowercase keyword.
2. Add stable ArenaDomain identity.
3. Add move-only Arena ownership.
4. Add borrowed fixed Arena construction.
5. Add typed New and Alloc validation.
6. Add allocation failure and atomic state flow.
7. Add Reset and Release ownership transitions.
8. Add implicit Arena destruction.
9. Add reference and epoch dependency verification.
10. Add allocation-context propagation.
11. Add task and thread dependency integration.
12. Add ordered Arena effects.
13. Add call-graph summaries.
14. Add static capacity analysis.
15. Add high-level Sec MLIR Arena operations.
16. Add fixed borrowed lowering.
17. Add owned fixed provider lowering.
18. Add growable segmented lowering.
19. Add optimization and metadata elimination.
20. Add LSP and complete diagnostics.
21. Verify the target matrix.
```

---

# Current implementation synchronization

The existing compiler already has partial Arena-related support through the
general allocation implementation.

The repository must be inventoried before implementation work begins.

Known partial areas include:

```text
Arena semantic builtin;
AllocationError semantic builtin;
Arena.Alloc recognition;
Arena.Reset recognition;
Arena.New recognition;
Arena.Release recognition and released-owner use rejection;
Arena.FromBuffer, Arena.WithCapacity, and Arena.Growable recognition;
typed compiler-known member registry entries with stable IDs;
LSP completion sourced from the compiler-known member registry;
lowercase arena lexed as an ordinary identifier rather than a keyword;
initial generation metadata;
initial stale-generation rejection;
initial branch and loop generation merging;
active allocation-context representation;
initial storage-origin metadata;
compiler-owned task-spawn and thread-start call-graph relationships;
derived task-entry and thread-entry roots for reachable spawn sites;
source-ordered direct call-graph effect sites for borrowed/owned/growable Arena
creation, typed allocation, Reset, and Release;
synchronous call-graph `MayAllocate` propagation with a concrete shortest
callable cause path;
LSP hover for direct Arena effects and propagated allocation cause paths;
```

The implementation is not complete merely because these partial pieces exist.

Known missing or incomplete areas include:

```text
allocation-context propagation;
path-aware ordered Arena effect-state merging across branches and loops;
complete effect inference beyond the current Arena `MayAllocate` summary;
complete ownership and move tracking;
complete default/layout/destruction validation;
Semantic IR Arena operations;
MLIR Arena dialect and lowering;
backing providers;
Arena dependency propagation and completion boundaries across the existing
task/thread call-graph relationships;
byte-accurate, alignment-aware capacity and demand analysis per
`CompilationPlan`;
complete diagnostics;
complete LSP definitions, capacity/effect details, and Arena-specific
diagnostics.
```

An implementation inventory must verify the repository rather than assuming
that this list remains current.

---

# Required synchronization

This rulebook must remain synchronized with:

```text
allocation.txt
attributes.md
borrowing.txt
call_graph.md
cancellation.md
compiler_analysis.txt
compiler_known_members.md
compiler_pipeline.txt
concurrency.md
concurrency_memory_model.txt
default_values.md
destruction.txt
diagnostics.txt
effect_analysis.md
escape_analysis.md
platform/ffi.md
declarations/interfaces.md
isr_analysis.md
layout.md
lifetime_analysis.md
lsp.md
ownership.md
panic.md
reference_model.md
runtime_checks.md
semantic_ir.txt
spawn.md
storage.md
structured_concurrency.md
rules/platform/target_profiles.md
tasks.txt
threads.md
unsafe.md
```

When one of these files is not yet written, its future rulebook must consume
Arena semantics from this document rather than redefine them.

---

# Rulebook ownership summary

```text
arena.md
    owns Arena source and compiler semantics

allocation.txt
    owns general allocation semantics and allocation-context policy

reference_model.md
    owns reference validity and physical representation freedom

effect_analysis.md
    owns effect inference and guarantee verification

call_graph.md
    owns callable reachability and execution relationships

tasks.txt / spawn.md / await.md
    own task lifecycle and synchronization semantics

threads.md
    owns thread lifecycle and synchronization semantics

destruction.txt
    owns ordinary destruction order

layout.md
    owns SizeOf, AlignOf, and physical layout

compiler_known_members.md
    owns compiler-known member inventory and naming

lsp.md
    owns tooling presentation, not Arena truth
```

---

# Final canonical summary

An Arena is a move-only programmer-visible allocation-domain owner.

The ArenaDomain identity is separate from physical backing addresses.

Backing may be owned, borrowed, static, or target-provided.

Independently, an Arena may be fixed or growable according to its capacity and
growth policy.

Borrowed Arenas take an exclusive borrow of mutable contiguous backing.

Owned Arenas acquire backing through a target/profile provider.

Growable Arenas may add stable segments but may never relocate a live
allocation.

Capacity is measured in bytes.

`Alloc[T](count)` interprets count as element count and includes checked size
arithmetic and alignment padding.

Safe `New[T]` and `Alloc[T]` fully initialize values and initially require sized,
defaultable, trivially destructible types.

Allocation returns `Result`, is atomic, never returns null or partial storage,
and never silently falls back to another allocation source.

Zero-element allocation is valid and normally silent.

A statically explicit zero-capacity Arena produces configurable information.

Repeated allocation does not invalidate prior allocations.

Reset retains the Arena and backing, reclaims the current epoch, advances the
validity epoch, and requires that no validity-preserving dependency crosses the
reset point.

Release consumes the Arena and terminates the ArenaDomain.

Normal Arena destruction performs terminal Release when the Arena remains
owned.

Arena storage dependencies extend through references, nested Arenas, closures,
tasks, threads, results, and foreign retention.

Task or thread completion ends execution-local dependencies only after a
semantic completion boundary.

Dependencies transferred through results remain live.

A task or thread does not automatically inherit the parent's mutable Arena
context.

Arena allocation context is compiler-visible and may be propagated through
synchronous calls without source-level boilerplate.

`MayAllocate` and `RequiresAllocationContext` are distinct facts.

Arena operations remain explicit in Semantic IR and high-level Sec MLIR until
ownership, lifetime, effects, task/thread dependencies, capacity, and target
planning are complete.

Arena state versions use SSA.

Validity epochs are separate from state versions.

MLIR `memref` may represent lowered typed views but does not replace high-level
Arena semantics.

Optimizers may remove checks and metadata only after proof.

The Arena model supports hosted, embedded, bare-metal, and no-runtime targets
without requiring a universal Sec runtime, garbage collector, reference
counter, or runtime Arena registry.
