# Memory Model

## Status

This document is the canonical abstract memory-model rulebook for Sec. The
former legacy text rulebook has been replaced and is no longer canonical.

This rulebook defines the common memory concepts used by:

```text
ownership.md
copy_move.md
borrowing.txt
references.txt
raw_pointers.txt
lifetime_analysis.txt
destruction.txt
allocation.txt
concurrency_memory_model.txt
ffi.txt
declarations/registers.md
platform/fixed-address-bindings.md
```

It does not duplicate the complete analysis algorithms or source syntax from
those specialized rulebooks.

Default initialization, defined by `default_values.md`, begins the lifetime of
a valid initialized value. A semantic default is neither uninitialized storage
nor necessarily all-bits-zero. Omitted struct fields and mutable declarations
without source initializers must be initialized before becoming readable.

An empty `list[T]` default owns no allocated element storage and constructs no
elements. A safe slice remains non-defaultable because every slice requires a
valid storage origin and lifetime.

---

# Current implementation status

The current compiler already contains several foundations required by this
memory model.

## Implemented

The current compiler implements:

- semantic type objects;
- reference mutability;
- reference-origin names and source tokens;
- local versus external reference-origin classification;
- reference storage-origin metadata;
- arena generation metadata on reference types;
- symbol storage-origin metadata;
- addressed-symbol metadata;
- volatile-symbol metadata;
- local-symbol metadata;
- the storage origins:
  - `Inline`;
  - `Static`;
  - `Arena`;
  - `External`;
  - `Foreign`;
  - `FixedAddress`;
  - `Unknown`;
- allocation-effect metadata:
  - `none`;
  - `arena`;
  - `foreign`;
  - `unknown`;
- active allocation-context representation;
- copy classification:
  - trivial;
  - semantic;
  - conditional;
  - move-only;
  - non-copyable;
- recursive trivial-destruction classification;
- recursive copy classification for several aggregate types;
- non-owning slice semantics in the current type model;
- initial move-state tracking for local symbols;
- hard use-after-move and use-after-discard diagnostics;
- delayed root-symbol uses retained by registered defer blocks, preventing an
  earlier move or discard without imposing runtime tracking;
- initial reference-escape analysis;
- rejection of returned references into local function storage;
- propagation of reference origins through selected fields, indexes, slices, and
  aggregate values;
- recognition of caller-owned parameter storage;
- recognition of ordinary locals as inline storage;
- recognition of addressed register bindings as fixed-address storage;
- semantic recognition of `Arena`;
- semantic recognition of `Arena.Alloc[T](count)`;
- semantic recognition of `Arena.Reset()`;
- arena-generation advancement;
- rejection of selected uses of stale-generation references and slices;
- conservative generation merging through several control-flow forms;
- prohibition of hidden escape promotion in the allocation rules;
- prohibition of hidden allocation during ordinary copy, move, borrow,
  parameter passing, return, and escape analysis;
- initial concurrency-memory-model rules for atomics, synchronization,
  publication, volatile access, and address stability.

## Partially implemented

The current implementation is partial in these areas:

- storage origin is represented in Sema but not propagated consistently through
  every expression, place, reference, aggregate, and Semantic IR operation;
- reference provenance is root-symbol based in several analyses;
- arena generation analysis is conservative and incomplete;
- lifetime analysis is flow-sensitive in selected cases but does not yet use one
  complete lifetime graph;
- place identity is not represented uniformly;
- field-sensitive and element-sensitive overlap analysis is incomplete;
- partial initialization is described by rules but not comprehensively tracked;
- partial move state is not implemented fully;
- replacement and reinitialization are not represented as distinct Semantic IR
  operations;
- value destruction is specified but not lowered completely;
- allocation origin is selected semantically in limited cases but no complete
  allocation propagation exists;
- custom resource ownership and resource-state transitions remain
  type-specific;
- current string lowering uses string views or static literals and does not yet
  represent every possible string storage model;
- interfaces do not yet distinguish every owned and borrowed erased
  representation;
- target layout, alignment, padding, and ABI facts are not represented through
  one canonical layout model;
- raw-pointer analysis does not yet preserve a complete internal provenance
  model;
- address stability is described by concurrency rules but is not a general
  first-class storage property throughout the compiler;
- volatile semantics exist for addressed symbols but complete MMIO lowering and
  device ordering remain target work.

## Not implemented

The following are not yet implemented completely:

- a unified abstract-memory representation;
- a canonical `Place` model;
- place paths;
- memory-location identity;
- disjointness and overlap proofs;
- separate value-state and storage-state lattices;
- complete partial-initialization masks;
- complete aggregate lifetime masks;
- valid-representation analysis;
- general uninitialized-storage types;
- first-class addressability classification;
- first-class relocation classification;
- first-class address-stability classification;
- first-class memory-space classification across all types and operations;
- complete reference provenance;
- complete raw-pointer provenance metadata;
- alignment verification for every unsafe conversion;
- complete size and layout queries before target lowering;
- padding-aware unsafe access rules;
- target-endianness propagation;
- complete temporary-lifetime lowering;
- complete full-expression cleanup;
- explicit Semantic IR operations for:
  - storage creation;
  - initialization;
  - replacement;
  - reinitialization;
  - copy;
  - move;
  - destruction;
  - deallocation;
  - volatile access;
  - fixed-address access;
  - provenance conversion;
- complete MLIR memory-space lowering;
- complete backend verification;
- complete FFI memory contracts;
- complete diagnostics for memory-model violations;
- complete memory-model test suites.

---

# Purpose

The Sec memory model defines:

- what a value is;
- what storage is;
- when storage contains a valid value;
- how source places refer to storage;
- when a value may be read or written;
- how ownership relates to storage;
- how references remain valid;
- how addressability and relocation differ;
- how allocation, initialization, destruction, and deallocation differ;
- how ordinary memory differs from volatile, atomic, foreign, and fixed-address
  memory;
- which facts are target-independent;
- which facts are target-dependent;
- which compiler stage establishes or preserves each fact.

The memory model is designed for:

```text
hosted applications
embedded systems
bare metal
operating-system code
FFI
hardware access
multithreading
task concurrency
shaped data
accelerators
multiple memory spaces
```

The same source-language memory semantics apply across targets.

A target profile may restrict available storage mechanisms.

It must not silently change source ownership, initialization, reference, or
destruction semantics.

---

# Scope

This document defines the abstract memory machine.

Specialized rulebooks define:

```text
ownership.md
    who owns an owned value

copy_move.md
    duplication and ownership transfer

borrowing.txt
    simultaneous non-owning access

references.txt
    safe reference syntax and validity

raw_pointers.txt
    unsafe address values and operations

lifetime_analysis.txt
    how the compiler proves lifetime relations

destruction.txt
    deterministic destruction and cleanup order

allocation.txt
    allocation contexts, arenas, and allocation failure

concurrency_memory_model.txt
    happens-before, atomic ordering, visibility, and data races

ffi.txt
    foreign contracts and ABI boundaries

declarations/registers.md
    register layouts

platform/fixed-address-bindings.md
    addressed hardware access
```

---

# Core principle

Safe Sec code may read a value of type `T` only when:

- a valid value of `T` has been initialized;
- its value lifetime is active;
- its storage remains valid when storage is required;
- the source place is available;
- ownership and borrowing permit the access;
- the reference or view remains valid;
- the storage generation remains current;
- the representation is valid for `T`;
- the alignment is valid when an address is used;
- the operation is valid for the target memory space;
- concurrency rules permit the access.

Safe Sec code may write a value only when the corresponding mutation,
replacement, or reinitialization rules are satisfied.

---

# Abstract machine

The Sec abstract machine reasons about:

```text
values
objects
bindings
places
subplaces
storage
memory locations
allocations
resources
references
raw pointers
lifetimes
program points
execution entities
memory spaces
```

Source semantics are defined at this level.

MLIR, LLVM IR, registers, stack frames, physical RAM, caches, and machine
instructions are implementations of the abstract machine.

They do not define source semantics.

---

# Terminology

## Value

A value is a typed semantic entity.

Examples:

```text
the integer 42
a string value
a Person value
a File value
a reference value
an enum value
a matrix value
```

A value may exist entirely as an abstract or SSA value.

A value does not necessarily have:

```text
a source binding
an address
an allocation
observable identity
owned storage
```

---

## Object

An object is a materialized typed value whose value lifetime has begun.

`object` is a memory-model term.

It does not imply:

```text
class
inheritance
reference semantics
heap allocation
OOP
```

A local scalar materialized in storage is an object.

A struct in an arena is an object.

A fixed-address register block may be modeled as an external object.

An optimized value that remains entirely in SSA may not require a materialized
object until an address or target operation requires one.

---

## Binding

A binding associates a source name with a value, place, reference, or compiler
entity.

Example:

```sec
let value := CreateValue()
```

`value` is the binding.

The binding has:

```text
name
scope
mutability
type
availability state
source location
```

A binding is not the same as:

```text
the value
the storage
the allocation
the resource
```

A binding may remain in lexical scope after its current value has moved or been
discarded.

---

## Place

A place is a source-level location expression that can identify a value-bearing
location or an addressable operation target.

Examples:

```sec
value
person.Name
array[index]
object.property
```

A place may be:

```text
readable
writable
addressable
movable
borrowable
volatile
atomic
fixed-address
conditionally available
```

Not every expression is a place.

Examples of non-place expressions:

```sec
CreateValue()
left + right
match result {
    Ok(value) => value
    Err(error) => fallback
}
```

---

## Root place

A root place is a place not derived from another source place.

Examples:

```text
local binding
parameter storage
static binding
foreign object
fixed-address binding
allocation root
```

---

## Subplace

A subplace is a place within another place.

Examples:

```sec
person.Name
pair.Left
array[3]
matrix[row, column]
```

A subplace path identifies the relation to its root.

Conceptually:

```text
session
session.File
session.File.Handle
```

---

## Storage

Storage is abstract or physical capacity capable of containing values.

Examples:

```text
stack slot
static data
arena allocation
caller-provided result storage
aggregate field storage
array element storage
foreign allocation
fixed hardware address
device memory
compiler temporary storage
register-backed storage
```

Storage may exist without containing a valid value.

Storage may outlive several successive values.

A value may exist without separately observable storage.

---

## Memory location

A memory location is a distinguishable storage region relevant to reads,
writes, aliasing, atomicity, borrowing, or concurrency.

A memory location may represent:

```text
one scalar object
one struct field
one array element
one register
one atomic object
one external region
```

Whether two source places refer to the same memory location is a semantic and
layout question.

---

## Allocation

An allocation is a storage region created by an allocation operation or
provided as an externally managed region.

An allocation has:

```text
origin
size
alignment
memory space
allocation lifetime
deallocation responsibility
```

An allocation may contain multiple objects over time.

Allocation is not synonymous with heap allocation.

Examples:

```text
arena block
foreign allocation
static region
device allocation
caller-provided output buffer
```

---

## Resource

A resource is a semantic entity whose lifetime or release operation may differ
from the storage containing its wrapper.

Examples:

```text
file descriptor
socket
device lease
DMA channel
interrupt registration
mutex ownership state
foreign handle
memory mapping
subscription
task or thread lifecycle
```

A resource may have no meaningful memory address.

Moving a wrapper may transfer resource responsibility without moving the
external resource.

Destroying wrapper storage is not automatically the same as releasing the
resource.

---

## Reference

A reference is a compiler-validated non-owning access capability.

Canonical categories:

```text
ref T
ref mut T
```

A reference carries semantic facts including:

```text
referent type
mutability
origin
storage origin
lifetime boundary
borrow relation
generation or epoch where applicable
```

References do not own their referents.

---

## Raw pointer

`RawPtr[T]` is an address-like value for unsafe, foreign, and low-level
operations.

A raw pointer does not imply:

```text
ownership
validity
initialization
alignment
lifetime extension
borrow permission
thread safety
```

The compiler may retain known origin and provenance facts internally.

The source-level raw-pointer type does not promise safe dereference.

---

## Memory space

A memory space identifies a storage domain with distinct access or transfer
rules.

Examples may include:

```text
ordinary host memory
static memory
arena memory
device memory
shared accelerator memory
MMIO
foreign memory
target-specific address spaces
```

`MemorySpace` may also appear in shaped-type metadata.

Memory-space identity is distinct from nominal type identity unless a type rule
makes it part of the type.

---

# Distinct lifetimes

The following lifetimes are distinct:

```text
lexical scope
binding lifetime
value lifetime
object lifetime
storage lifetime
allocation lifetime
reference lifetime
borrow lifetime
resource lifetime
temporary lifetime
physical stack-frame lifetime
backend lifetime metadata
```

They must not be used as synonyms.

---

# Lexical scope

Lexical scope is a source-code region controlling name visibility and providing
an upper bound for many local bindings.

Lexical scope does not fully determine exact lifetime.

A value may end before scope exit because it is:

```text
moved
discarded
destroyed
replaced
invalidated
proven dead
no longer the active variant
```

A borrow may end after its final relevant use.

---

# Binding lifetime

A binding lifetime begins when the declaration becomes active in its scope.

It ends when the binding leaves scope.

The binding may be:

```text
uninitialized
available
moved
discarded
detached
partially available
conditionally available
```

during that lifetime.

Binding lifetime does not prove that a readable value exists.

---

# Value lifetime

A value lifetime begins when a valid value becomes initialized.

It ends when the value is:

```text
destroyed
moved from its source place
discarded
replaced
transferred out
invalidated
no longer the active union variant
no longer initialized on the current path
```

A moved value continues at its destination as the same semantic ownership
responsibility or as a destination value according to the copy/move rules.

The source value lifetime ends.

---

# Object lifetime

An object lifetime is the interval during which a materialized value exists in
storage as an object of its type.

Object lifetime begins after valid initialization commits.

Object lifetime ends before or at:

```text
destruction
replacement
move-out when the source no longer contains the object
storage invalidation
deallocation
active-variant change
```

Storage may remain after object lifetime ends.

---

# Storage lifetime

Storage lifetime is the interval during which the storage region may physically
or abstractly hold values.

Storage may outlive a particular object.

Example:

```sec
let mut value := CreateFirst()
value = CreateSecond()
```

The binding and storage may remain.

The first object lifetime ends.

The second object lifetime begins in the same storage.

---

# Allocation lifetime

Allocation lifetime begins after successful allocation.

It ends when the allocation is deallocated, reset, unmapped, released, or
otherwise invalidated by its storage-origin rules.

Objects within an allocation may have narrower lifetimes.

A reference cannot remain valid beyond the allocation lifetime.

A raw pointer does not extend allocation lifetime.

---

# Reference lifetime

A reference lifetime is the interval in which that reference may be used.

It cannot extend beyond:

```text
referent validity
storage validity
allocation validity
arena generation
borrow permission
resource-state requirements
target memory-space validity
```

The physical address bits may remain after semantic reference lifetime ends.

Safe Sec code cannot use the reference after that point.

---

# Borrow lifetime

Borrow lifetime is the interval in which borrow restrictions apply to the
referent.

The compiler should infer the narrowest provably correct borrow lifetime.

Borrow lifetime is usually non-lexical.

It may end after the reference's final relevant use.

---

# Resource lifetime

Resource lifetime is determined by the resource contract.

It may begin or end independently from wrapper storage.

Example:

```sec
let mut file := OpenFile()
file.Close()
```

Possible result:

```text
file binding
    still in scope

wrapper storage
    still exists

external handle
    released

resource state
    closed or unavailable

later destruction
    must not close twice
```

---

# Temporary lifetime

A temporary lifetime begins when the temporary is successfully created.

A temporary remains valid as required by expression evaluation and references.

The canonical initial rule is:

- operands evaluate in the Sec-defined evaluation order;
- successfully created expression temporaries remain valid until the enclosing
  full expression completes unless ownership moves earlier;
- remaining owned temporaries are destroyed in reverse successful creation
  order;
- early control-flow exit performs required temporary cleanup;
- a moved temporary is not destroyed by its previous owner.

The compiler may shorten a temporary lifetime only when this does not change:

```text
reference validity
destruction order
observable side effects
defer behavior
evaluation order
volatile effects
atomic effects
FFI effects
```

---

# Full expression

A full expression is the expression boundary after which temporary cleanup may
run.

Typical full-expression boundaries include:

```text
expression statement
initializer
assignment right-hand side
return expression
condition evaluation
individual control-flow header expression
```

The exact grammar mapping must remain synchronized with expression and
destruction rules.

---

# Initialization model

Storage and values have separate state.

Storage may exist before a value lifetime begins.

Safe source code must never treat uninitialized storage as a value of `T`.

---

# Storage states

Conceptual storage states include:

```text
Absent
Reserved
Allocated
Uninitialized
PartiallyInitialized
Initialized
Invalidated
Deallocated
```

The implementation may use a more precise state system.

---

# Value availability states

Canonical source availability states are defined by ownership rules:

```text
Uninitialized
Available
PartiallyAvailable
Moved
Discarded
Detached
ConditionallyAvailable
```

Storage state and value availability are related but distinct.

Example:

```text
storage
    remains allocated

value
    moved and unavailable
```

---

# Uninitialized storage

Uninitialized storage contains no valid value of the intended type.

It must not be:

```text
read as T
copied as T
moved as T
borrowed as initialized T
destroyed as T
exposed through safe ref T
```

Safe allocation APIs must initialize storage before exposing readable `T`.

A future general uninitialized-memory API must use:

```text
a distinct uninitialized type
an explicit initialization protocol
or unsafe operations
```

It must not expose arbitrary uninitialized bytes as ordinary `ref mut T[]`.

---

# Initialization

Initialization establishes a valid value in storage.

Initialization commits only after:

```text
source evaluation succeeds
required conversions succeed
contracts succeed
required allocation succeeds
required field construction succeeds
representation is valid
```

A failed initialization does not create a complete value to destroy.

Successfully initialized subobjects and temporaries still require cleanup.

---

# Partial initialization

An aggregate under construction may be partially initialized.

The compiler must know which fields or elements were initialized successfully.

On failure:

- only successfully initialized owned subobjects are destroyed;
- cleanup follows the canonical reverse successful initialization order;
- the complete aggregate's custom `free` operation does not run unless partial
  construction is explicitly supported;
- uninitialized fields are not read or destroyed.

Partial initialization state must not escape as an ordinary complete `T`.

---

# Reinitialization

Reinitialization creates a new value lifetime in storage that currently contains
no available value.

Examples include assignment after:

```text
move
discard
explicit uninitialized declaration
partial field move followed by field restoration
```

Reinitialization destroys no previous value because no previous available value
remains.

The destination must be mutable and otherwise valid.

---

# Replacement

Replacement ends one value lifetime and starts another in the same destination
place.

Conceptual order:

1. evaluate and validate the replacement source;
2. preserve the old destination until commit is safe;
3. destroy the old destination value;
4. initialize the new value;
5. transfer destruction responsibility.

Fallible validation must occur before destructive commit whenever required by
the operation.

---

# Destruction

Destruction ends a value or object lifetime and performs type-defined cleanup.

Destruction may:

```text
destroy owned fields
release external resources
run custom free behavior
invalidate dependent references
```

Destruction does not necessarily reclaim storage.

---

# Deallocation

Deallocation ends an allocation lifetime and makes its storage unavailable.

Deallocation is distinct from destruction.

Before deallocation:

- every live owned object requiring destruction must have been handled;
- no valid safe reference may depend on the allocation;
- target and allocator contracts must permit deallocation.

---

# Fundamental distinctions

The compiler must preserve these distinctions:

```text
destruction != deallocation
resource release != storage reclamation
move != physical relocation
scope exit != exact value lifetime
binding lifetime != value lifetime
value lifetime != allocation lifetime
RawPtr loss != deallocation
arena object destruction != arena-byte reclamation
copy != alias creation
reference copy != referent copy
```

---

# Ownership and storage

Every owned value has exactly one current owner.

Not every value is owning.

Examples of non-owning values include:

```text
references
raw pointers
views
some descriptors
function references
```

Storage ownership and value ownership may differ.

Example:

```text
arena
    owns raw storage

value in arena
    owns its fields and external resources

reference
    borrows the initialized value
```

Moving an arena-backed value may transfer value ownership without transferring
arena ownership.

---

# Storage origins

Semantic analysis records storage origin independently from nominal type.

Minimum origins:

```text
Inline
Static
Arena
External
Foreign
FixedAddress
Unknown
```

A future implementation may internally distinguish additional origins such as:

```text
CompilerTemporary
CallerProvidedReturn
Device
SharedMemory
MappedMemory
```

These refinements must map back to canonical semantics.

---

# Inline storage

Inline storage is storage associated directly with:

```text
local values
parameters
aggregate fields
fixed arrays
compiler materialization
caller-provided value storage
```

`Inline` does not promise physical stack allocation.

The compiler may keep a value in registers or SSA.

It may materialize storage only when required.

---

# Static storage

Static storage has a program-wide or explicitly defined static duration.

Static lifetime does not imply:

```text
immutability
thread safety
atomicity
synchronization
ownership sharing
```

Sec 0.1 should avoid requiring hidden global constructors, destructor
registries, or process-exit cleanup.

Non-trivial owned global values require an explicit future design.

---

# Arena storage

Arena storage is reclaimed by arena reset or arena destruction.

An arena allocation has:

```text
arena identity
generation or epoch
size
alignment
memory space
```

Values stored in arena memory follow normal value ownership and destruction.

Destroying a value does not necessarily reclaim its arena bytes.

Reset may proceed only when no valid dependent owned value, reference, slice,
capture, deferred operation, or returned value remains.

---

# External storage

External storage is provided and owned outside the current value.

Examples:

```text
caller-owned parameter storage
buffer owned by another Sec object
platform-owned region
borrowed output storage
```

An external-storage reference does not extend the external owner's lifetime.

---

# Foreign storage

Foreign storage follows an explicit FFI contract.

The contract must eventually define:

```text
allocator
owner
lifetime
alignment
layout
mutability
retention
thread access
release operation
success ownership
failure ownership
```

A foreign pointer is not automatically an owned allocation.

---

# Fixed-address storage

Fixed-address storage has an address that is part of its semantics.

Examples:

```text
MMIO registers
interrupt vector memory
platform control blocks
linker-defined regions
device windows
```

Fixed-address storage cannot be relocated as ordinary value storage.

A wrapper or handle referring to it may be movable.

The storage itself remains fixed.

---

# Unknown storage origin

Unknown origin means the compiler lacks enough verified information.

Safe operations must remain conservative.

Unknown does not mean:

```text
heap
owned
static
valid forever
safe to escape
safe to share
```

Unsafe or FFI code must provide stronger proof or wrapper semantics before safe
use.

---

# Identity

Sec distinguishes:

```text
value equality
object identity
storage identity
allocation identity
resource identity
address
```

These are not interchangeable.

---

# Value equality

Value equality compares semantic values according to type rules.

It must not depend on:

```text
padding bytes
uninitialized bytes
storage address
allocation identity
irrelevant descriptor bits
```

unless a type explicitly defines identity-based equality.

---

# Object identity

Not every object has observable identity.

Identity may matter for:

```text
mutexes
atomics
published shared state
foreign registrations
device ownership
self-referential structures
address-keyed systems
```

A type rule must define when identity is observable.

---

# Storage identity

Storage identity distinguishes the underlying location.

Two places may contain equal values while referring to different storage.

Two references may refer to the same storage location.

Storage identity is relevant to:

```text
aliasing
borrowing
atomics
data races
address stability
FFI
```

---

# Allocation identity

Allocation identity distinguishes storage regions created or provided as
allocations.

Two subplaces may share one allocation identity while referring to disjoint
locations.

Allocation identity is used for:

```text
deallocation
bounds
arena generation
provenance
overlap analysis
```

---

# Resource identity

Resource identity belongs to the external or logical resource.

A resource may retain the same identity when its owning wrapper moves.

Copying the wrapper must not duplicate unique resource identity.

---

# Address

An address is a target representation of a location.

Address equality does not by itself prove:

```text
same active object
same provenance
same lifetime
same owner
safe aliasing
valid dereference
```

Targets may reuse addresses after lifetimes end.

---

# Addressability

A place is addressable when the language permits creation of a reference or raw
pointer to its storage.

A value need not have a physical address until an operation requires one.

The compiler may materialize addressable storage from SSA when semantics permit.

Materialization must not:

```text
introduce hidden dynamic allocation
extend lifetime incorrectly
change identity
change destruction order
change volatile behavior
```

---

# Non-addressable values

Examples may include:

```text
pure compile-time values
temporary arithmetic values
some register-only compiler values
abstract shaped values before bufferization
```

The compiler may materialize a non-addressable abstract value when an explicit
operation requires addressability and the result remains semantically valid.

---

# Relocation

Relocation is physical movement of representation while preserving semantic
value or object validity.

Relocation is not the same as source move.

A source move may require no physical relocation.

A physical relocation may occur during optimization without a source move when
identity and references are preserved.

---

# Relocatable storage

A value is relocatable during a period when changing its physical storage
location cannot be observed through valid Sec operations.

The compiler may relocate such values.

---

# Address-stable storage

Address-stable storage must retain its address for a defined validity period.

Address stability may be required by:

```text
live references
published pointers
mutex identity
atomic identity
foreign retention
self-references
DMA registration
device contracts
```

Address-stable does not necessarily mean permanently fixed-address.

Stability may begin and end at defined program points.

---

# Fixed-address versus address-stable

```text
fixed-address
    address is permanently part of storage semantics

address-stable
    address must remain unchanged for a required interval

relocatable
    physical storage may change without observable effect
```

These classifications are distinct.

---

# Place relationships

The compiler must classify relationships between places.

Canonical relationships:

```text
Same
Disjoint
Contains
ContainedBy
PotentiallyOverlapping
Unknown
```

---

# Same place

Two place expressions identify the same semantic location.

Example:

```sec
value
value
```

after resolution to the same binding and path.

---

# Disjoint places

Two places are provably non-overlapping.

Examples may include separate direct struct fields:

```sec
pair.Left
pair.Right
```

when the type and representation permit independent access.

Disjointness permits selected simultaneous borrows and independent state
tracking.

---

# Containing place

An aggregate place contains its subplaces.

Example:

```text
session
    contains session.File
```

Moving or destroying a containing place affects every owned available subplace.

A partially unavailable subplace makes complete-place operations invalid where
required.

---

# Potentially overlapping places

The compiler cannot prove disjointness.

Examples:

```sec
values[a]
values[b]
```

when `a` and `b` are runtime values.

Potential overlap must be treated conservatively for mutable borrowing, move,
replacement, atomics, and data-race analysis.

---

# Place paths

A place path contains:

```text
root identity
field projections
constant indexes
dynamic indexes
slice projections
variant projections
dereference projections
```

The compiler may use symbolic indexes and constraints to prove disjointness.

Source semantics do not depend on one internal path representation.

---

# Reads

A read observes a value.

A valid safe read requires:

- initialized value;
- active value lifetime;
- available place;
- valid representation;
- valid reference or direct access;
- permitted borrow state;
- valid storage generation;
- target-supported access;
- concurrency permission;
- correct volatile or atomic operation when applicable.

A read of a copyable value may produce a copy.

A read of a move-only value does not silently move it.

---

# Writes

A write modifies storage or an object.

A write may represent:

```text
mutation
replacement
reinitialization
atomic store
volatile store
foreign write
hardware register write
```

A valid safe write requires:

- writable place;
- mutable permission;
- compatible borrow state;
- valid target storage;
- valid type and representation;
- satisfied contract;
- correct concurrency primitive;
- correct volatile or atomic semantics when required.

---

# Mutation

Mutation changes part of an existing value while preserving the containing
value lifetime where type rules permit it.

Examples:

```sec
counter += 1
person.Name = "Ada"
```

Mutation may create narrower field-value lifetimes for replaced fields.

Mutation does not automatically create a new complete object lifetime.

---

# Interior mutation

Interior mutation permits controlled mutation through a shared reference.

It is available only through compiler-known or explicitly declared safe types
such as:

```text
Atomic[T]
Mutex[T]
other synchronization or cell types
```

Interior mutation is not ordinary mutable aliasing.

The type must define:

```text
synchronization
ownership
thread safety
address stability
destruction
failure
```

---

# Aliasing

Aliasing exists when multiple access paths may refer to overlapping storage.

Aliasing is not automatically invalid.

Validity depends on:

```text
shared versus mutable access
lifetime
ownership
atomicity
interior mutation
volatile access
FFI contracts
concurrency
```

Safe shared references may alias for read access.

A mutable reference requires exclusivity according to borrowing rules.

Raw pointers may alias, but unsafe code bears the proof burden.

---

# Effective access width

A memory access has an effective width and alignment determined by:

```text
type
operation
register rule
atomic rule
FFI ABI
target lowering
```

MMIO and atomic accesses may require exact widths.

The compiler must not split, merge, narrow, or widen observable accesses when
that changes semantics.

---

# References and provenance

A safe reference carries compiler-validated provenance.

Provenance includes enough semantic origin information to prove:

```text
what storage the reference may access
which object or region it derives from
which generation it depends on
which bounds apply
which mutability permission exists
how long access remains valid
```

Provenance is a compiler concept.

It does not require a runtime pointer header.

---

# Reference derivation

A derived reference must remain within the valid storage, object, bounds, and
lifetime permitted by its origin.

Examples:

```text
field reference
array-element reference
slice reference
reborrow
reference returned from a declared source relation
```

Derived references propagate storage origin and generation dependency.

---

# Reference conversion

Safe conversion between reference forms requires proof of:

```text
non-null validity where applicable
alignment
active value
valid representation
sufficient lifetime
bounds
mutability and exclusivity
storage origin
generation
```

No safe reference conversion may fabricate a new longer lifetime.

---

# Raw-pointer provenance

A raw pointer may retain internal compiler metadata when derived from a known
reference or allocation.

This can improve:

```text
diagnostics
bounds checks
optimization
FFI verification
address-space checking
```

However, source-level `RawPtr[T]` does not promise safe provenance.

A raw pointer created from:

```text
integer address
unknown FFI result
opaque foreign cast
```

may have unknown provenance.

---

# Raw-pointer dereference

Unsafe dereference requires the programmer or wrapper to establish:

- address identifies accessible storage;
- storage is alive;
- required alignment is satisfied;
- sufficient bytes exist;
- a valid active value of `T` exists for read;
- writing `T` is valid for write;
- aliasing rules are satisfied;
- concurrent access is valid;
- target memory space supports the operation;
- volatile or atomic semantics are used when required.

Unsafe does not make an invalid dereference defined.

---

# Bounds

Safe references, slices, arrays, and views carry or derive bounds.

An access must remain within:

```text
object bounds
allocation bounds
slice bounds
subobject bounds
memory-space constraints
```

One-past or sentinel pointer rules, if any, belong to raw-pointer and FFI
rulebooks.

They are not implied for safe references.

---

# Valid representation

A value of type `T` may be read only when storage contains a valid active
representation of `T`.

Validity may require:

```text
allowed enum discriminant
active union tag and payload
valid bool representation
valid reference
valid character or rune constraints
valid contract state
valid atomic representation
valid register snapshot
valid nominal invariants
```

Not every possible bit pattern is necessarily a valid `T`.

---

# Representation creation

A valid representation is created by:

```text
language initialization
validated conversion
compiler-generated construction
safe deserialization API
validated FFI wrapper
explicit unsafe proof
```

Bytewise copying arbitrary storage does not automatically construct a valid
value.

---

# Invalid representation

Safe code cannot create or observe an invalid representation.

Unsafe code that creates an invalid active `T` causes the program to violate
memory-model preconditions.

The compiler may assume safe values have valid representations.

It must not assume unvalidated foreign or raw bytes are valid `T`.

---

# Size

A sized type has a target-known finite storage size at the stage where storage
is allocated or ABI lowering occurs.

Source-level size may depend on:

```text
target architecture
layout
generic arguments
constant dimensions
ABI
memory space
```

Unsized or dynamically sized abstractions must be behind:

```text
reference
slice
descriptor
owning container
explicit dynamic object representation
```

Exact sizedness rules belong to type and layout rulebooks.

---

# Alignment

Every addressable type has a required alignment for a selected target and
memory space.

Safe language operations satisfy alignment automatically.

Unsafe conversion or dereference must prove alignment.

Misaligned access may be:

```text
invalid
lowered through explicit byte operations
supported by a target-specific operation
handled by a dedicated unaligned API
```

The compiler must not silently issue target-invalid misaligned accesses.

---

# Padding

Padding is representation space inserted for layout or alignment.

Padding is not a source-level field or initialized value.

Rules:

- safe code does not read padding as semantic data;
- value equality does not compare padding;
- hashing does not depend on uninitialized padding;
- copying a value must not expose padding contents;
- serialization must use field semantics, not raw padding;
- FFI may expose padding only through explicit layout and unsafe byte access;
- the compiler may leave padding unspecified;
- security-sensitive lowering may clear padding when required by policy.

---

# Endianness

Endianness affects byte representation.

It does not change the abstract arithmetic value.

Rules:

- ordinary numeric semantics are endian-independent;
- byte views are target-endian unless an API specifies another order;
- serialization formats must define endian explicitly;
- registers and protocols may define endian explicitly;
- FFI follows ABI endian;
- the compiler must not assume little-endian source semantics;
- target lowering uses target endian where representation is not explicitly
  fixed.

---

# Layout

Layout includes:

```text
size
alignment
field offsets
padding
discriminant representation
calling-convention representation
memory-space representation
```

Layout is target-dependent unless a rule explicitly fixes it.

Source type semantics remain target-independent.

Explicit fixed layouts may be required for:

```text
FFI
registers
wire protocols
storage formats
shared memory
hardware descriptors
```

A separate `layout.md` rulebook should define complete layout syntax and
guarantees.

Until it exists, this file defines the boundary only.

---

# Representation versus serialization

In-memory representation is not a serialization format.

The compiler may change ordinary internal layout across:

```text
targets
compiler versions
optimization modes
ABIs
memory spaces
```

unless an explicit stable layout is requested.

Persistent and network formats require explicit encoding rules.

---

# Allocation

Dynamic allocation is explicit through the operation being performed.

The compiler may select or propagate an allocation context for an operation
defined as allocation-capable.

It must not introduce allocation for an operation that is not defined as
allocating.

The following do not allocate merely because they occur:

```sec
let second := first
target = source
Inspect(value)
return value
ref value
```

Escape does not justify hidden allocation.

---

# Allocation success

A fallible allocation creates storage only on success.

The resulting allocation records:

```text
origin
identity
size
alignment
memory space
generation where applicable
deallocation responsibility
```

Safe readable values are exposed only after initialization.

---

# Allocation failure

Allocation failure uses `Result`.

It must not silently:

```text
return null
return invalid storage
panic
terminate
shorten the allocation
continue with partial capacity
```

unless an API and target profile explicitly define an infallible allocation.

---

# No hidden escape promotion

The compiler must not repair an invalid escaping reference by moving local
storage into an arena or heap.

Example:

```sec
fn Invalid() ref byte[] {
    let data: byte[16] := [...]
    return ref data[..]
}
```

This is invalid.

An owning operation that intentionally creates escaping storage must allocate
directly in a suitable context.

---

# No mandatory heap

Sec does not require a traditional global heap.

Possible allocation contexts include:

```text
compiler-managed arena
caller-propagated arena
explicit arena
target-provided arena
foreign allocator
device allocator
no dynamic allocation
```

Profiles may restrict allocation.

Ownership and reference rules remain the same.

---

# Arena generation

Arena-backed values and references may depend on an arena generation.

Conceptually:

```text
Arena
    identity
    current generation

dependent value or reference
    arena identity
    generation at creation
```

Reset advances generation and invalidates older dependencies.

Generation metadata is semantic.

It does not require:

```text
garbage collection
reference counting
runtime borrow table
fat pointer
runtime generation check
```

A target or debug mode may optionally add checks.

---

# Static storage and initialization

Compile-time constants require no runtime object initialization.

Immutable static data with trivial destruction may be supported directly.

Sec 0.1 should not require:

```text
hidden global constructors
hidden process-exit finalizers
global destructor registry
```

Owned global values requiring non-trivial initialization or destruction require
an explicit future rule.

---

# External and foreign storage

External and foreign storage must not be assumed to outlive a reference merely
because an address exists.

The owner contract determines lifetime.

A callback, retained pointer, or asynchronous foreign use requires an explicit
escape and thread-access contract.

---

# Memory spaces

The compiler must track memory space where access or transfer rules differ.

A transfer between memory spaces may require:

```text
copy
move of a descriptor
DMA
mapping
synchronization
cache maintenance
conversion
allocation
```

The operation must be explicit in API or language semantics.

Ordinary assignment must not silently perform expensive or fallible cross-space
transfer.

---

# Host and device values

A shaped value may be abstract before lowering.

Its logical value semantics remain independent of whether lowering chooses:

```text
SSA tensor
host buffer
device buffer
shared buffer
view descriptor
```

The compiler must preserve ownership, lifetime, and synchronization during
bufferization and memory-space transfer.

---

# Volatile memory

Volatile access is observable.

The compiler must preserve the required access occurrence, width, and ordering
relative to other volatile and defined external effects.

Volatile does not provide:

```text
atomicity
ownership
acquire semantics
release semantics
data-race safety
thread synchronization
```

Volatile and atomic are separate.

---

# MMIO

Memory-mapped I/O uses fixed-address and usually volatile semantics.

MMIO rules may additionally require:

```text
exact access width
access ordering
read side effects
write side effects
reserved-bit preservation
device barriers
target-specific instructions
```

The compiler must not treat MMIO as ordinary RAM.

Moving a handle to MMIO does not move MMIO storage.

---

# Registers

A source-level register declaration may describe:

```text
bit layout
access width
reserved bits
units
fixed address
volatile behavior
```

A local register snapshot is a value.

The hardware register location is fixed-address storage.

Snapshot copy rules and hardware access rules are distinct.

---

# Atomics

Atomic storage has synchronized interior-mutation semantics.

Atomicity is tied to:

```text
storage identity
access width
alignment
target support
memory order
```

Copying an atomic value snapshot is different from copying the atomic storage
object.

An initialized atomic storage object may be non-copyable and non-relocatable
after publication.

Exact rules belong to `concurrency_memory_model.txt` and atomics rulebooks.

---

# Mutexes

A mutex may require stable storage identity after initialization or publication.

The compiler must not relocate it during a period requiring stability.

A mutex guard is a separate move-only ownership value.

Moving the guard transfers unlock responsibility.

---

# Concurrency boundary

This memory model defines:

```text
values
objects
storage
locations
identity
initialization
ordinary access
references
provenance
```

`concurrency_memory_model.txt` defines:

```text
execution entities
visibility
happens-before
publication
atomic ordering
mutex synchronization
data races
compiler and hardware reordering
```

Ownership and borrowing determine who may access memory.

Concurrency ordering determines when writes become visible across execution
entities.

Both models must be satisfied.

---

# Sequential execution

Within one execution entity, operations follow Sec's evaluation and
control-flow rules.

The compiler may optimize only when observable behavior is preserved.

Observable behavior includes:

```text
ordinary value semantics
destruction
allocation
FFI
volatile access
atomic operations
synchronization
resource effects
panic behavior
```

---

# Data races

A data race is not defined merely by aliasing.

It requires conflicting concurrent access without required ordering.

The full definition belongs to concurrency rules.

Safe Sec must not permit ordinary data races.

Raw pointers and unsafe code do not make a race defined.

---

# FFI boundary

A foreign ABI may use another memory model.

Sec wrappers must define or verify:

```text
layout
size
alignment
endianness
ownership
allocation origin
reference lifetime
pointer retention
mutability
thread safety
atomic representation
volatile behavior
callback concurrency
release operation
success and failure ownership
```

The compiler must not infer safe publication or lifetime extension from an
ordinary foreign call.

---

# Unsafe

`unsafe` permits operations whose proof cannot be completed by the compiler.

It transfers the proof obligation to the programmer or wrapper.

Unsafe obligations may include:

```text
valid address
live storage
alignment
bounds
valid representation
initialization
provenance
aliasing
lifetime
ownership
thread safety
atomic support
volatile behavior
FFI contract
```

Unsafe does not disable:

```text
syntax
types
scope
visibility
control flow
ordinary ownership
must-use
Result handling
target representability
deterministic destruction
```

The compiler may continue every analysis possible inside unsafe code.

---

# Memory safety guarantees

Safe Sec aims to prevent:

```text
use after value lifetime
use after deallocation
use after arena reset
double destruction
double resource release
dangling references
invalid safe dereference
read of uninitialized storage
read of moved or discarded value
invalid mutable aliasing
implicit unique-resource duplication
hidden escape allocation
invalid union payload access
invalid enum representation
out-of-bounds safe access
misaligned safe access
ordinary data races
relocation of fixed-address storage
```

Some guarantees depend on complete implementation of the corresponding
analysis.

---

# Deterministic destruction

Every available owned value is destroyed exactly once unless ownership
transfers.

Destruction order is defined by `destruction.txt`.

The backend must not infer cleanup from lexical scope alone.

It receives explicit cleanup responsibility from Semantic IR.

---

# Early destruction

The compiler may end a value lifetime before lexical scope exit only when:

- no later valid use exists;
- no reference depends on the value;
- destruction order remains observationally equivalent;
- defer behavior remains valid;
- resource effects remain ordered;
- concurrency and volatile behavior remain valid.

Early physical storage reuse must preserve semantic lifetime rules.

---

# Storage reuse

The compiler may reuse storage for a later value when previous object lifetime
has ended and all relevant constraints are satisfied.

Storage reuse must preserve:

```text
alignment
address stability
reference validity
destruction
debug semantics where required
volatile behavior
FFI retention
```

A reused address does not make the new object identical to the previous object.

---

# Copy and storage

Copy creates a new semantic value while preserving the source.

Physical strategies may include:

```text
register copy
descriptor copy
field copy
bulk memory copy
copy elision
recomputation
```

A bulk byte copy is valid only when type semantics permit it.

Padding and invalid representations must not become observable.

---

# Move and storage

Move transfers value or ownership responsibility.

Physical strategies may include:

```text
no instruction
descriptor transfer
register forwarding
storage reuse
pointer transfer
field transfer
resource-handle transfer
```

Move does not require physical byte relocation.

The moved source is unavailable even if old bits remain.

---

# Reference and storage

Creating a reference may require materializing stable storage.

Such materialization must not silently allocate dynamically.

The compiler may use existing inline, caller, static, arena, or target storage
when semantics allow.

A reference prevents incompatible relocation or replacement during its valid
lifetime.

---

# Slices and views

A slice or view is non-owning unless a specific type explicitly says otherwise.

It carries or derives:

```text
storage origin
bounds
element type
mutability
lifetime
generation
memory space
```

Copying a shared slice copies the descriptor and borrow relation.

It does not copy elements or storage.

Moving a mutable view transfers the view and borrow obligation.

---

# Strings

`string` is an immutable, source-level copyable value.

Its storage representation may include:

```text
static immutable data
borrowed immutable view
compiler-proven immutable backing
arena-backed immutable data
inline representation
```

Implicit copy must not allocate or create mutable aliasing.

String representation and storage lifetime must preserve ordinary copy
semantics.

A future separate owning string type may have non-trivial destruction.

---

# Arrays

A fixed array owns or contains fixed element storage according to its containing
object.

Every initialized element has a value lifetime.

Array storage remains fixed in length.

Move-out from one fixed element is not part of Sec 0.1 ordinary indexing.

Partial fixed-array initialization requires explicit future state rules.

---

# Dynamic collections

A dynamic collection may own:

```text
descriptor state
allocation
elements
capacity
external resources
```

Collection storage lifetime and individual element value lifetimes are
distinct.

Structural mutation may:

```text
relocate elements
invalidate references
change allocation
change storage identity
change bounds
```

Collection APIs must declare these effects.

Ordinary indexing must not silently consume an element.

---

# Unions and active variants

A union value has one active variant according to its tag or proven state.

Only the active payload has an active value lifetime.

Changing variants:

- destroys the old active payload when required;
- initializes the new payload;
- updates valid representation;
- invalidates references to the old payload.

Reading an inactive payload is invalid.

---

# Enums

An enum object must contain a valid declared value or alias according to enum
rules.

Foreign and raw integer values require validated conversion.

Invalid discriminants are not safe enum values.

---

# Contracts and memory

A contract constrains valid semantic values.

Storage containing a representation outside the contract is not a valid value
of the constrained nominal type.

Validated construction begins the value lifetime only after contract success.

Replacement and reinitialization commit only after required checks.

Move of a same-type valid value preserves the contract proof.

---

# Closures

Closure storage contains capture state.

Capture fields have ownership and lifetime.

A closure may be:

```text
non-capturing value
inline capturing object
arena-backed object
foreign callback wrapper
```

Returning or retaining a closure requires every captured reference and owned
value to remain valid.

Physical closure representation does not weaken capture semantics.

---

# Interfaces

Interface representations may include:

```text
type descriptor
method table
borrowed data reference
owned erased value
inline payload
indirect payload
```

The compiler must distinguish borrowed and owned interface values.

A descriptor alone does not establish ownership.

Layout and ABI representation may differ by target.

---

# Zero-sized and empty values

A type may require no runtime data bytes.

A zero-sized value still has:

```text
type
value lifetime
ownership state where relevant
destruction semantics where relevant
source identity rules
```

The compiler may coalesce physical storage only when identity and addressability
are not observable.

Exact zero-sized-type support belongs to type and layout rules.

---

# Function parameters

Parameter passing may use:

```text
register values
stack values
hidden pointers
caller-provided storage
split ABI values
```

These are ABI choices.

Source semantics determine:

```text
copy
move
borrow
mutable borrow
foreign contract
```

before ABI lowering.

---

# Return storage

A return result may use:

```text
register return
split registers
caller-provided storage
hidden return pointer
destination passing
SSA forwarding
```

The caller owns an owned returned value.

The callee must not destroy transferred result ownership.

Return storage strategy does not change source copy/move semantics.

---

# Stack is not a source guarantee

An ordinary local does not promise physical stack storage.

The compiler may use:

```text
registers
SSA
stack
caller storage
inlining
elision
```

It must not silently choose dynamic allocation merely because the value escapes
or requires an address.

A stack analysis reports target lowering, not source-level storage identity.

---

# Heap is not a source primitive

Sec does not require a universal heap concept.

Dynamic storage comes from explicit allocation-capable operations and selected
allocation contexts.

A target may provide:

```text
arena
foreign allocator
device allocator
fixed pool
no dynamic allocation
```

---

# Cache and coherence

Ordinary source semantics do not expose cache lines.

Concurrent and device-visible memory may require:

```text
atomic synchronization
cache maintenance
device barriers
DMA ownership transitions
platform APIs
```

A target-specific API or future Knowledge Pack may enrich diagnostics.

The abstract memory model does not assume universal cache coherence.

---

# DMA

DMA may access storage concurrently with the CPU or device.

A safe DMA API must define:

```text
buffer ownership
memory space
alignment
size
cache maintenance
publication
completion
cancellation
device lifetime
CPU access restrictions
```

DMA does not become safe merely because a pointer is available.

Detailed DMA rules belong to platform and device APIs.

---

# Semantic analysis

Sema must determine, where relevant:

```text
value type
place category
place path
storage origin
memory space
initialization state
availability state
ownership
copy/move operation
reference origin
borrow state
lifetime relation
generation dependency
bounds
addressability
relocation constraints
address stability
volatile status
atomic status
fixed-address status
resource state
destruction responsibility
allocation effect
```

Sema establishes source semantics.

No lower stage may invent a different memory meaning.

---

# Program points

The compiler should reason at defined program points.

Useful points include:

```text
before expression
after expression
after successful initialization
before and after copy
before and after move
before and after replacement
before and after reinitialization
before and after destruction
before and after deallocation
before and after borrow creation
before and after final borrow use
control-flow edges
block entry and exit
call entry and return
cleanup edges
panic edges
```

---

# Semantic IR

Semantic IR must make memory semantics explicit.

Required conceptual operations include:

```text
ReserveStorage
AllocateStorage
DeallocateStorage
BeginObjectLifetime
EndObjectLifetime
ConstructValue
InitializeValue
InitializeField
CopyValue
MoveValue
BorrowShared
BorrowMutable
ReadValue
WriteValue
MutateValue
ReplaceValue
ReinitializeValue
DiscardValue
DestroyValue
ReturnValue
TransferArgument
TransferField
TransferCollectionElement
CreateReference
Reborrow
CreateRawPointer
RawLoad
RawStore
VolatileLoad
VolatileStore
AtomicLoad
AtomicStore
MemorySpaceTransfer
ArenaReset
InvalidateGeneration
```

The exact operation set may be normalized.

All required semantic distinctions must remain representable.

---

# Semantic IR metadata

Operations and values record where applicable:

```text
type
source place
destination place
place relationship
storage origin
allocation identity
memory space
size
alignment
bounds
initialization state
availability state
ownership state
copy classification
reference provenance
borrow relation
generation
relocation class
address-stability requirement
volatile status
atomic status
resource state
destruction responsibility
source location
target applicability
```

---

# Semantic IR verification

Before lowering, verification must ensure:

- every read targets an initialized available value;
- every write has permission;
- every reference has a valid origin and lifetime relation;
- every move has one valid source and destination;
- moved sources are unavailable;
- every owned value has one terminal responsibility;
- partial initialization cleanup is complete;
- replacement and reinitialization are distinguished;
- deallocation does not precede required destruction;
- no safe reference outlives storage;
- no stale arena generation is used;
- fixed-address storage is not relocated;
- volatile and atomic operations remain explicit;
- memory-space transfers are explicit;
- FFI assumptions are represented;
- backend-required layout facts are available.

A failure is an internal compiler error after successful Sema.

---

# MLIR lowering

MLIR receives resolved memory semantics.

It may choose suitable dialects and representations such as:

```text
SSA values
memref
tensor
LLVM pointers
target memory spaces
bufferization
explicit allocation
explicit deallocation
```

MLIR may optimize:

```text
copy elision
move elision
buffer reuse
destination passing
stack-slot reuse
scalar replacement
register forwarding
in-place update
dead storage elimination
```

It must preserve:

```text
value validity
reference validity
ownership
destruction
allocation origin
address stability
volatile access
atomic ordering
memory spaces
fixed-address identity
FFI layout
```

---

# LLVM lowering

LLVM IR is not the Sec memory model.

LLVM operations must be generated only after Sec semantics are resolved.

LLVM lifetime intrinsics are optimization metadata.

They do not define Sec value lifetime.

LLVM poison, undef, alias metadata, and pointer provenance must be used only in
ways consistent with Sec's safe-value and unsafe-operation rules.

The backend must not introduce observable reads of uninitialized or padding
data.

---

# Target lowering

Target lowering determines:

```text
size
alignment
field offsets
ABI
endianness
atomic support
address spaces
volatile instruction selection
MMIO barriers
calling convention
register allocation
stack frame
```

Target lowering may reject a program when the target cannot represent a required
operation safely.

It must not reinterpret source semantics.

---

# Diagnostics

Memory-model diagnostics require stable IDs.

Suggested rules:

```text
memory.read-uninitialized
memory.use-after-value-lifetime
memory.use-after-deallocation
memory.invalid-representation
memory.misaligned-access
memory.out-of-bounds
memory.invalid-reference-origin
memory.reference-outlives-storage
memory.stale-arena-generation
memory.invalid-place-overlap
memory.fixed-address-relocation
memory.address-stability-violation
memory.invalid-memory-space-access
memory.implicit-allocation
memory.deallocate-while-borrowed
memory.deallocate-live-object
memory.invalid-volatile-access
memory.invalid-atomic-width
memory.invalid-ffi-layout
memory.unknown-storage-origin
```

Safety diagnostics are mandatory errors.

Performance and portability findings may be advisory.

---

# Diagnostic quality

A diagnostic should identify:

```text
operation
type
place
storage origin
value state
required alignment or bounds
origin location
invalidating operation
related references
target
suggested safe correction
```

Example:

```text
error[S....]: reference view is no longer valid because arena scratch was reset
```

Related locations:

```text
reference created here
arena generation advanced here
invalid use here
```

---

# Testing

Required test categories:

```text
initialization
partial initialization
reinitialization
replacement
destruction
deallocation
storage origins
references
raw pointers
arena generations
place overlap
addressability
relocation
address stability
fixed-address storage
alignment
padding
endianness
valid representations
volatile
atomics
FFI
memory spaces
target lowering
```

---

# Valid source tests

Examples should include:

```sec
let mut value := CreateFirst()
value = CreateSecond()
```

```sec
let storage := try arena.Alloc[byte](4096)
Use(storage)
```

```sec
let first := "hello"
let second := first
```

```sec
let moved :<- first
```

```sec
let view := ref value
Inspect(view)
Mutate(value)
```

where non-lexical lifetime analysis proves the borrow ended.

---

# Invalid source tests

Required invalid cases include:

```text
read before initialization
use after move
use after discard
use after destruction
use after arena reset
return reference to local storage
move while borrowed
deallocate while referenced
read inactive union payload
raw-to-reference conversion without proof
misaligned raw dereference
fixed-address relocation
ordinary access to required volatile storage
ordinary concurrent data race
```

Every invalid Sec test contains:

```sec
/* Expected error: ...
 * Reason: ...
 */
```

---

# Compiler unit tests

Required Go tests include:

```text
storage-origin propagation
place-path construction
place overlap
state transitions
lifetime begin and end
partial initialization cleanup
reference provenance
arena generations
alignment queries
layout queries
valid representation
Semantic IR verification
MLIR memory-space lowering
fixed-address verification
volatile lowering
target endian behavior
```

---

# Fuzzing and model checking

Fuzz:

```text
control-flow state merges
nested aggregates
partial construction
partial move
references
arena reset
unsafe casts
raw pointer arithmetic
layout combinations
target alignments
```

Small-state model tests should verify:

```text
no read before initialization
no double destruction
no destruction of moved source
no deallocation with live safe reference
no stale generation use
one terminal responsibility per owned value
```

---

# Required synchronization

This rulebook must remain synchronized with:

```text
ownership.md
copy_move.md
discard.md
borrowing.txt
references.txt
raw_pointers.txt
lifetime_analysis.txt
destruction.txt
allocation.txt
concurrency_memory_model.txt
data_races.md
deadlock_analysis.md
ffi.txt
declarations/registers.md
platform/fixed-address-bindings.md
collections.md
shaped-types.md
types.md
contracts.md
functions.md
struct.md
unions.md
atomics rulebook
mutex rulebook
target_profiles.md
platform_model.md
semantic_ir.txt
compiler_pipeline.txt
diagnostics.txt
language-rulebook-status.md
rules_implementations.txt
```

A complete future layout rulebook should be added as:

```text
layout.md
```

and marked Planned in the status document.

---

# Appendix A — Codex replacement and implementation plan

## A.1 Rename the rulebook

The filename migration is complete. `rules/memory/memory_model.md` is canonical,
repository references are updated, and no duplicate canonical file remains.

---

## A.2 Preserve existing semantic metadata

Preserve and test the existing fields and types representing:

```text
StorageOrigin
AllocationEffect
AllocationContext
ReferenceOriginName
ReferenceOriginToken
ReferenceOriginLocal
ReferenceOriginStorage
ReferenceOriginGeneration
Symbol.Storage
Symbol.Addressed
Symbol.Volatile
Symbol.Local
CopyClassification
```

Do not remove working metadata before the replacement model is available.

---

## A.3 Introduce a canonical place model

Create a reusable place representation shared by:

```text
ownership
copy/move
borrowing
lifetime
destruction
data-race analysis
LSP
```

Conceptual structure:

```go
type Place struct {
    Root        PlaceRoot
    Projections []PlaceProjection
    Type        Type
    Storage     StorageOrigin
}
```

Possible projections:

```text
Field
ConstantIndex
DynamicIndex
Slice
Variant
Dereference
PropertyStorage
```

Exact Go structure is an implementation choice.

---

## A.4 Implement place relationships

Add one canonical relationship query:

```text
Same
Disjoint
Contains
ContainedBy
PotentiallyOverlapping
Unknown
```

Use it for:

```text
borrow conflicts
move conflicts
partial moves
replacement
destruction
atomics
data races
```

Do not maintain separate incompatible overlap algorithms.

---

## A.5 Separate storage state and value state

Introduce distinct internal models.

Storage state:

```text
Absent
Reserved
Allocated
Uninitialized
PartiallyInitialized
Initialized
Invalidated
Deallocated
```

Value availability:

```text
Uninitialized
Available
PartiallyAvailable
Moved
Discarded
Detached
ConditionallyAvailable
```

Do not overload one moved-map or assigned-map with both concepts.

---

## A.6 Create lifetime events

Represent explicit events:

```text
StorageReserved
StorageAllocated
ObjectInitialized
FieldInitialized
ObjectMoved
ObjectDestroyed
StorageInvalidated
StorageDeallocated
ResourceReleased
GenerationAdvanced
```

These may be data-flow events or Semantic IR operations.

---

## A.7 Propagate storage origin

Every place, reference, slice, view, allocation result, field, aggregate, return,
and captured value must carry or derive storage origin.

Unknown origin remains conservative.

Do not infer `Inline` merely from type.

---

## A.8 Extend reference provenance

Reference metadata should include:

```text
root place
storage origin
allocation or region identity
generation
bounds
mutability
validity region
memory space
```

Preserve current origin name and token for diagnostics.

---

## A.9 Retain raw-pointer origin facts

When creating `RawPtr[T]` from a known source, retain optional internal
provenance facts.

Do not treat those facts as a safe source-level guarantee after unsupported
arithmetic or foreign escape.

Add provenance-loss events where required.

---

## A.10 Initialization analysis

Implement:

```text
definite initialization
field initialization masks
partial-construction cleanup
variant active-state tracking
array initialization state where supported
```

Reject reads of uninitialized state.

Do not expose uninitialized `T` through safe references.

---

## A.11 Replacement and reinitialization

Represent these separately.

Replacement:

```text
old value exists and must be destroyed
```

Reinitialization:

```text
no old available value exists
```

Update assignment after move and discard accordingly.

---

## A.12 Addressability

Add queries:

```text
IsAddressable
CanMaterializeAddress
RequiresAddressStability
IsFixedAddress
IsRelocatable
```

Materialization must not create dynamic allocation implicitly.

---

## A.13 Address stability

Track stability intervals caused by:

```text
safe references
foreign-retained pointers
published mutexes
published atomics
DMA registration
self-reference
target contracts
```

Reject relocation or storage reuse during the interval.

---

## A.14 Layout service

Create a target-aware layout service providing:

```text
size
alignment
field offsets
padding
discriminant representation
ABI representation
memory space
```

Do not put layout calculations separately in unrelated backends.

Add `layout.md` later as the canonical source rule.

---

## A.15 Valid representation

Add validation metadata for:

```text
bool
enum
union
reference
char
rune
contracts
atomics
register snapshots
```

Safe construction and conversion produce valid representations.

Raw and FFI operations require explicit validation or unsafe proof.

---

## A.16 Alignment and bounds

Centralize:

```text
required alignment
known alignment
allocation bounds
object bounds
slice bounds
subobject bounds
```

Use common diagnostics across raw pointers, references, arrays, slices, FFI, and
atomics.

---

## A.17 Padding policy

Ensure compiler-generated:

```text
equality
hashing
serialization
copy
debug output
```

does not depend on unspecified padding bytes.

Raw byte access remains explicit and unsafe where applicable.

---

## A.18 Endianness

Add target endian to target metadata.

Use explicit endian in:

```text
serialization
registers
protocol layouts
foreign layout
```

Ordinary arithmetic remains endian-independent.

---

## A.19 Temporary cleanup

Lower full-expression temporaries through explicit cleanup.

Preserve:

```text
Sec evaluation order
reverse successful temporary destruction
move suppression
early-exit cleanup
```

---

## A.20 Allocation and deallocation

Keep allocation explicit.

Add Semantic IR operations for allocation and deallocation.

Verify:

```text
no hidden escape promotion
no deallocation with live safe references
no deallocation before object cleanup
no allocator mismatch
```

---

## A.21 Arena generations

Preserve current generation analysis.

Migrate it from root-name special cases into region and storage-origin
dependencies.

Add field, slice, capture, return, and aggregate propagation.

---

## A.22 Volatile and fixed address

Represent volatile and fixed-address operations explicitly in Semantic IR.

Do not lower them as ordinary loads and stores followed by optional flags.

Verify exact access width and target support.

---

## A.23 Memory spaces

Propagate memory-space metadata through:

```text
allocations
shaped values
views
references
copies
moves
calls
FFI
device operations
```

Cross-space transfers must be explicit operations.

---

## A.24 Concurrency integration

Share storage identity and place relationships with
`concurrency_memory_model.txt`.

Atomics, mutexes, publication, and race analysis must use the same location
model.

Do not create a second concurrency-only storage identity.

---

## A.25 Semantic IR

Implement or normalize the operations listed in this rulebook.

Add verification before MLIR lowering.

No raw backend operation should be emitted without resolved memory semantics.

---

## A.26 MLIR

Map:

```text
ordinary storage
memory spaces
fixed-address storage
volatile operations
atomics
allocation
deallocation
views
shaped values
```

to appropriate MLIR dialects and target lowering.

Do not infer ownership or lifetime from MLIR alone.

---

## A.27 Diagnostics

Register stable memory diagnostics.

Every diagnostic should include related source locations when available.

Expose structured data to LSP:

```text
place
origin
state
generation
bounds
alignment
target
fix
```

---

## A.28 LSP integration

Expose:

```text
storage origin
value state
addressability
reference origin
generation dependency
memory space
alignment
size
layout
fixed-address status
volatile status
resource lifetime
```

Only expose facts actually known by the compiler.

---

## A.29 Tests

Create:

```text
memory_model_valid.sec
memory_model_invalid.sec
memory_initialization_valid.sec
memory_initialization_invalid.sec
memory_references_valid.sec
memory_references_invalid.sec
memory_arena_valid.sec
memory_arena_invalid.sec
memory_raw_valid.sec
memory_raw_invalid.sec
memory_hardware_valid.sec
memory_hardware_invalid.sec
memory_ffi_valid.sec
memory_ffi_invalid.sec
```

Retain specialized tests in their own rule areas.

---

## A.30 Status documents

Update:

```text
language-rulebook-status.md
rules_implementations.txt
```

Mark:

```text
memory_model.md
    Written

memory_model.md
    replaced

layout.md
    Planned
```

Record existing implemented metadata separately from the incomplete unified
model.

---

## A.31 Recommended implementation order

```text
1. Rename and synchronize rulebook references.
2. Introduce canonical place and place-path types.
3. Implement place relationship queries.
4. Separate storage and value states.
5. Propagate storage origins consistently.
6. Extend reference provenance.
7. Implement replacement versus reinitialization.
8. Implement partial initialization state.
9. Add explicit lifetime and cleanup events.
10. Add addressability and stability classification.
11. Add target layout service.
12. Add valid-representation, bounds, and alignment verification.
13. Add explicit Semantic IR memory operations.
14. Integrate volatile, fixed-address, and memory-space lowering.
15. Integrate concurrency storage identity.
16. Complete MLIR and backend verification.
17. Expose memory facts through LSP.
```

---

# Design summary

The Sec memory model separates:

```text
value
object
binding
place
storage
memory location
allocation
resource
reference
raw pointer
```

Every owned value has one current owner.

Not every value is owning or addressable.

Storage may exist without a valid value.

Uninitialized storage is not a value of `T`.

Value, storage, allocation, reference, borrow, resource, and temporary lifetimes
are distinct.

Initialization begins a value lifetime.

Destruction ends a value lifetime.

Deallocation ends an allocation lifetime.

Move does not imply physical relocation.

Physical relocation does not necessarily imply source move.

Address-stable and fixed-address storage are distinct.

References have compiler-validated provenance.

Raw pointers do not provide safe validity guarantees.

Layout, size, alignment, padding, and ABI are target-dependent unless explicitly
fixed.

Padding is not source-level initialized data.

Ordinary numeric semantics are endian-independent.

Volatile is not atomic or synchronization.

Concurrency visibility remains defined by the separate concurrency memory model.

Unsafe transfers proof obligations but does not disable Sec semantics.

Sema establishes memory semantics.

Semantic IR preserves and verifies them.

MLIR and target lowering implement them without inventing new ownership,
allocation, lifetime, or storage meaning.
