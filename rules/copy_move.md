# Copy and Move

## Status

This document is the canonical copy-and-move rulebook for Sec. The former
legacy text rulebook has been replaced and is no longer canonical.

This document is synchronized with:

```text
ownership.md
discard.md
formatter.md
lsp.md
```

The normative decisions in this file supersede older rules that allowed
ordinary `:=` and `=` syntax to silently move a move-only named source.

Defaultability and copyability are independent. Reinitialization may construct a
fresh semantic default where the syntax and type permit it. Omitted struct
fields are constructed directly, not copied from a hidden global value; an
empty list default is likewise construction, not copy. These rules do not alter
the explicit move operators.

---

# Current implementation status

The compiler already contains a substantial first implementation of copy
classification and move tracking.

The direct named-source frontend path follows the canonical explicit-move
rules. Place-sensitive state, explicit ownership IR, and backend verification
remain to be migrated.

## Implemented

The current compiler implements:

- a `CopyClassification` model with:
  - trivial;
  - semantic;
  - move-only;
  - conditional;
  - non-copyable;
- `CopyClassificationOf`;
- `TriviallyCopyable`;
- `MoveOnly`;
- primitive type copy classification;
- `string` currently classified as trivially copyable;
- shared references currently classified as copyable;
- `ref mut` currently classified as move-only;
- `RawPtr[T]` currently classified as trivially copyable;
- array and struct classification derived recursively;
- custom-destruction influence on type classification;
- move tracking for local identifiers;
- move reasons including:
  - moved;
  - returned;
  - discarded;
  - detached;
- source token tracking for moves;
- hard diagnostics for later use of moved values;
- hard diagnostics for later use of discarded values;
- related source locations for previous move or discard;
- rejection of move while the root value is borrowed;
- rejection of move or discard while a registered defer block retains a
  delayed root-symbol use, including conservative branch merging and a related
  defer source location;
- move propagation through:
  - struct literals;
  - array literals;
  - `Ok(...)`;
  - `Err(...)`;
  - spread expressions;
  - conversion expressions;
- transfer of move-only values into selected by-value parameters;
- preservation of values passed through `ref` and `ref mut` parameters;
- transfer of returned move-only locals;
- transfer of tracked resources through return;
- branch merging of moved state in:
  - `if`;
  - `switch`;
  - `select`;
  - `match`;
- assignment that clears moved state after successful reinitialization;
- assignment that clears close-tracked resource state;
- indexed assignment validation;
- rejection of move-only extraction through ordinary array and slice indexing;
- explicit AST assignment operator text;
- dedicated lexer tokens for `:<-` and `<-`, with longest-match handling before
  `:` and `<`;
- parser support for inferred move initialization, typed move initialization,
  and move assignment;
- parser rejection when inferred declarations use `<-` or typed declarations
  use `:<-`;
- explicit AST ownership modes on declarations and assignments;
- Sema rejection of ordinary copy syntax for named move-only sources through
  diagnostic `S1007`;
- explicit move of named copyable and move-only values;
- enforcement of compiler-known `CopyNonCopyable` types in ordinary named
  copies and consuming transfer contexts;
- internal type metadata for explicit nominal non-copyability and distinct
  nominal-policy, compiler-known, and aggregate-field diagnostic causes;
- conservative rejection of ordinary named-source copies whose generic
  copyability has not been proven;
- self-move rejection for direct and overlapping root destinations;
- mutable reinitialization after move or explicit discard;
- formatter preservation and LSP semantic-token classification of both move
  operators;
- LSP quick fixes from `:=` to `:<-` and from `=` to `<-` for `S1007`;
- tests for derived aggregate copy, compiler-known and aggregate non-copyable
  rejection, generic no-proof copy and explicit move, method-name neutrality,
  and several move, borrow, return, resource, branch, and lifecycle cases.

## Partially implemented

The implementation is partial in these areas:

- move state is primarily root-identifier based;
- member and field move state is not tracked independently;
- branch state merges moved identifiers conservatively but do not yet implement
  the complete ownership-state lattice;
- type classification exists, but implicit copy implementation is incomplete
  for semantic-copy types;
- destruction responsibility is inferred indirectly rather than represented as
  explicit Semantic IR;
- aggregate construction moves move-only identifiers recursively, but does not
  yet record complete destination ownership;
- call argument transfer uses parameter `ref` information and type
  classification, but does not yet expose structured consumption metadata;
- current strings are treated as views or static literals at lowering level;
- loops do not yet implement the complete fixed-point ownership model;
- current diagnostics are not all registered with stable ownership IDs;
- LSP move quick fixes are operator-aware, but do not yet prove every borrow,
  alias, and place-overlap condition required for universally safe automatic
  application.

## Not implemented

The following are not yet implemented:

- place-based move tracking;
- field-sensitive partial moves;
- field reinitialization after partial move;
- complete aggregate availability masks;
- fixed-array partial move;
- dynamic-list consuming extraction;
- a general relocation classification;
- pinned or stable-address values;
- explicit copy/move operations in Semantic IR;
- MLIR ownership lowering;
- backend verification of destruction responsibility;
- complete ABI ownership verification;
- LSP ownership hints, move-site navigation, and partial-state display;
- complete copy/move diagnostic registry;
- complete copy/move test files.

## Awaiting canonical declaration syntax

Three source declaration mechanisms cannot be implemented without language
decisions that do not yet exist:

- source syntax requiring that a nominal type remain compiler-derivably
  copyable;
- source syntax explicitly forbidding derived implicit copy on a nominal type;
- generic constraints requiring proven copyability.

Named duplication methods such as `Copy`, `Clone`, or fallible `Duplicate` are
already ordinary methods. They do not change the type's implicit-copy
classification.

Compiler-known types may already carry `CopyNonCopyable`, and Sema enforces that
classification. General source syntax remains delegated to the planned
`attributes.md` rulebook and to `types.txt`. No annotation spelling is
canonical yet.

## Intentionally unsupported in Sec 0.1

Sec 0.1 does not support arbitrary user-defined function bodies as hidden
implementations of implicit copy. Method-name-based recognition of `Copy`,
`Clone`, `Duplicate`, `Snapshot`, or `ToOwned` is likewise unsupported, as is
implicit duplication through a fallible method. These are language guarantees,
not pending implementation work.

---

# Purpose

Copy and move are distinct source-language operations.

Copy duplicates a value while preserving the source.

Move transfers the current value or ownership responsibility and makes the
source unavailable.

The distinction must be:

- deterministic;
- statically validated;
- visible where an existing reusable source is consumed;
- explicit in Semantic IR;
- compatible with deterministic destruction;
- compatible with borrowing;
- compatible with generics;
- independent of garbage collection;
- independent of runtime ownership tables;
- independent of backend representation;
- optimizable without semantic change.

---

# Relationship to ownership

`ownership.md` defines:

```text
owner
place
binding
temporary
availability
reinitialization
destruction responsibility
partial availability
```

This rulebook defines:

```text
copy classification
copy operation
move operation
source syntax
context-sensitive transfer
copy and move validation
copy and move lowering
```

Where the two overlap, they must remain synchronized.

---

# Core semantic operations

The compiler distinguishes at least:

```text
Construct
Copy
Move
BorrowShared
BorrowMutable
Read
Mutate
Replace
Reinitialize
Discard
Destroy
TransferToReturn
TransferToArgument
TransferToAggregate
TransferToCollection
TransferAcrossChannel
TransferAcrossFFI
```

Two operations may lower to identical machine instructions while remaining
semantically different.

Example:

```text
copy of a string descriptor
move of a string descriptor
```

may both lower to a few register moves.

The source remains available after copy and unavailable after move.

A backend load or store must not be used to infer which semantic operation
occurred.

---

# Construction

Construction creates a new initialized value.

Examples:

```text
literal construction
struct literal
enum construction
union construction
Result construction
Option construction
function result
type conversion
allocation
compiler-generated aggregate construction
```

Construction establishes ownership when the constructed value is owning.

Construction is not automatically copy or move.

Input values used by construction may themselves be:

```text
copied
moved
borrowed
converted
```

according to their source category and construction context.

---

# Copy

Copy creates another value of the same semantic type while preserving the
source.

After a valid copy:

```text
source
    Available

destination
    Available
```

Both may be used independently according to type semantics.

For an owning copied result, each copy has its own valid destruction
responsibility.

Copy must never create two owners of one unique resource.

---

# Move

Move transfers the current value or ownership responsibility to a destination.

After move:

```text
source
    Moved

destination
    Available
```

The source must not be:

```text
read
copied
borrowed
moved again
discarded
destroyed
used as a complete aggregate
```

until a mutable source place is validly reinitialized.

Destruction responsibility transfers to the destination.

A move may require no physical instruction.

It remains semantically significant.

---

# Move does not mean reset

Move does not assign a default value to the source.

Example:

```sec
let mut original := "hello"
let moved :<- original
```

Afterward, `original` is unavailable.

It is not automatically:

```sec
""
```

The compiler may clear physical storage for target or security reasons.

That physical state is not a valid source-level value.

A mutable source may be explicitly reinitialized:

```sec
original = ""
```

An immutable source cannot be reinitialized.

---

# Borrow

Borrowing does not copy or transfer ownership.

Shared borrow:

```sec
let view := ref value
```

Mutable borrow:

```sec
let view := ref mut value
```

The owner remains responsible for the referent.

Copy and move operations must respect active borrow rules.

Borrow semantics are canonical in:

```text
borrowing.txt
references.txt
lifetime_analysis.txt
```

---

# Copy classification

Every resolved type has one copy classification.

Canonical categories:

```text
TriviallyCopyable
SemanticallyCopyable
ConditionallyCopyable
MoveOnly
ExplicitlyNonCopyable
```

The implementation may use current internal names:

```text
CopyTrivial
CopySemantic
CopyConditional
CopyMoveOnly
CopyNonCopyable
```

Classification is resolved before validating ordinary copy syntax.

---

# Trivially copyable types

A trivially copyable value can be duplicated without type-defined behavior.

Typical examples:

```text
bool
byte
char
rune
signed integers
unsigned integers
ordinary floats
simple decimals
fieldless enums
function values without owned captures
RawPtr[T]
shared references
copyable register snapshots
arrays of trivially copyable elements
structs composed only of trivially copyable fields when semantics permit
```

Trivial copy classification is semantic.

Equal size or bitwise representation alone does not prove copyability.

---

# Semantically copyable types

A semantically copyable value may require compiler-known behavior beyond raw
bit copying.

Implicit semantic copy must be:

- infallible;
- bounded;
- free from hidden heap allocation;
- free from hidden external resource acquisition;
- free from blocking;
- free from I/O;
- free from mutable aliasing;
- free from externally observable side effects;
- unsurprising for the type.

A semantic copy may use compiler or core implementation.

It must not hide an operation that can fail.

---

# Conditionally copyable types

A generic or aggregate type may be copyable only when relevant contained types
are copyable.

Examples:

```text
Option[T]
Result[T, E]
array T[N]
tuple-like explicit structures
generic struct
closure
vector
matrix
tensor descriptor
```

The compiler resolves the concrete classification for each instantiation.

Generic source syntax must not silently change from copy to move between
instantiations.

---

# Move-only types

A move-only type cannot be duplicated by ordinary copy syntax.

Typical examples:

```text
File
Socket
DeviceHandle
MemoryMap
UniqueAllocation[T]
MutexGuard[T]
ref mut T
unresolved Task[T]
unresolved Thread[T]
unresolved Process
types containing move-only fields
types with custom destruction unless safe copy is explicitly defined
```

Move-only classification belongs to the type, not to a particular variable.

---

# Explicitly non-copyable types

A type may forbid copy even if its representation would otherwise be copyable.

Reasons include:

```text
logical identity
external uniqueness
address registration
hardware ownership
single-use protocol state
security policy
custom invariants
```

The exact user syntax for explicit non-copyability belongs to the type and
attribute rulebooks.

Compiler-known types may use this classification before general source syntax
exists.

---

# No move-only variable modifier

Sec does not declare an individual variable move-only independently of its type.

A binding is:

```text
mutable or immutable
available or unavailable
```

The type determines whether ordinary copy is legal.

A copyable value may still be explicitly moved.

---

# Default copy derivation

The compiler derives classification where possible.

A struct is trivially copyable when:

- every owned field is trivially copyable;
- every reference field permits copying;
- the type has no custom destruction;
- the type has no unique ownership invariant;
- the type is not explicitly non-copyable;
- no field requires address stability that copy would violate.

A struct is move-only when:

- any required field is move-only;
- the type has custom destruction without a compiler-proven semantic-copy
  classification;
- the type owns an external resource;
- independent duplication cannot be derived safely.

A struct is explicitly non-copyable when nominal policy forbids derived copy.
It is also non-copyable when any required field is non-copyable. This
classification does not by itself determine whether ownership may move.

A generic type is conditionally copyable when its classification depends on
generic arguments.

Representation alone does not establish copyability. Field ownership,
destruction behavior, mutable-reference rules, compiler-known restrictions,
and nominal policy all participate in the compiler's proof.

---

# Implicit copy policy

Ordinary implicit copy is permitted only when the resolved type classification
allows it.

Implicit copy must not:

```text
allocate
fail
block
perform I/O
duplicate a unique external handle
introduce reference counting
introduce garbage collection
create mutable shared state
```

An operation requiring any of those behaviors must be an explicit named
operation.

Examples:

```sec
let duplicate := source.Copy()
let duplicate := try source.Duplicate()
```

Exact API names belong to the type.

---

# User-defined copy semantics

Copyability is compiler-determined from the type category, stored fields,
ownership and reference behavior, destruction rules, resource ownership,
compiler-known semantic properties, nominal restrictions, and proven generic
requirements.

A user-defined aggregate receives derived copyability when the compiler proves
that all relevant parts permit copy. No method declaration is required.

Sec 0.1 does not allow an arbitrary user-defined body to implement ordinary
implicit copy. Consequently, `let second := first` cannot hide allocation,
failure, blocking, I/O, resource acquisition, observable side effects,
unbounded work, or mutable alias creation.

Core and compiler-known types may have privileged semantic-copy behavior only
when it is bounded, infallible, non-blocking, allocation-free, free from I/O
and hidden resource acquisition, and safe under the ownership and aliasing
rules. This privilege does not extend to arbitrary user code.

A nominal type may explicitly forbid otherwise derivable implicit copy. A
future positive declaration may require the compiler to prove that a nominal
type remains derivably copyable; such a declaration is a verified requirement,
not a custom copy implementation. The source syntax for both policies remains
owned by the planned `attributes.md` rulebook.

# Explicit named duplication

A type may expose a named duplication operation.

Examples:

```text
Copy
Clone
Duplicate
Snapshot
ToOwned
```

These names are ordinary method names. They have no compiler-recognized effect
on assignment or initialization semantics, even when the method is infallible
and returns the receiver type. A permanent core API rule may define a
particular API, but the spelling alone never changes copy classification.

A fallible duplication returns `Result`.

Example:

```sec
let duplicate := try file.Duplicate()
```

This is an ordinary function call.

It is not implicit assignment copy.

Named duplication may allocate, fail, perform domain-specific work, duplicate
an external resource, or return another type. Those visible operations remain
available to move-only and non-copyable types without making them implicitly
copyable.

---

# Source syntax overview

Canonical source forms:

```sec
let destination := source
let destination :<- source

let destination: Type := source
let destination: Type <- source

destination = source
destination <- source
```

Meaning:

| Form | Meaning |
|---|---|
| `:=` | ordinary initialization |
| `:<-` | move initialization with inferred type |
| `=` | ordinary assignment or replacement |
| `<-` | move initialization or move replacement |
| `return expression` | return-result transfer context |
| by-value argument | copy copyable value or transfer move-only value |
| aggregate payload | copy copyable value or transfer move-only value |

---

# Ordinary initialization from an existing place

Example:

```sec
let destination := source
```

When `source` is an existing reusable place:

- copy is required;
- the source type must be implicitly copyable;
- source remains available;
- destination becomes available.

If the type is move-only, this is a hard error.

The compiler must not reinterpret `:=` as move.

Suggested diagnostic:

```text
Buffer cannot be copied from source
```

Suggested help:

```text
use `let destination :<- source` to transfer ownership
```

---

# Move initialization with inferred type

Example:

```sec
let destination :<- source
```

This explicitly moves from `source`.

The destination type is inferred.

Afterward:

```text
destination
    Available

source
    Moved
```

The form is valid for:

```text
move-only values
copyable values
```

Moving a copyable value is an explicit programmer choice.

---

# Typed ordinary initialization

Example:

```sec
let destination: Buffer := source
```

When `source` is an existing reusable place:

- source must be copyable;
- copy initializes destination;
- source remains available.

When the right side is a fresh temporary, direct construction is allowed.

---

# Typed move initialization

Canonical:

```sec
let destination: Buffer <- source
```

The colon introduces the explicit type.

The move token is therefore `<-`, not `:<-`.

Afterward `source` is unavailable.

---

# Ordinary assignment

Example:

```sec
destination = source
```

When `source` is an existing reusable place:

- source must be copyable;
- source remains available;
- destination receives a copy.

When destination was already available, this is replacement.

When destination was unavailable and mutable, this is reinitialization.

---

# Move assignment

Example:

```sec
destination <- source
```

This explicitly moves `source` into `destination`.

When destination is available, this is move replacement.

When destination is unavailable and mutable, this is move reinitialization.

The source becomes unavailable.

---

# Fresh temporaries

A fresh temporary may directly initialize or replace a destination without
explicit move syntax.

Examples:

```sec
let buffer := CreateBuffer()
let file: File := OpenFile()
current = CreateBuffer()
```

The temporary has no reusable user-visible source place.

The operation is:

```text
direct construction
temporary forwarding
temporary transfer
```

depending on lowering.

The formatter must never change these forms to `:<-`.

---

# Temporary versus place

Sema must classify the right-hand expression.

## Reusable place

Examples:

```sec
source
object.field
array[index]
```

A reusable place can remain available or become unavailable.

Copy and explicit move rules apply.

## Temporary

Examples:

```sec
CreateBuffer()
left + right
User {
    Name: "Ada",
}
```

The temporary is consumed by its destination.

No explicit move token is needed.

## Borrowed place

Examples:

```sec
ref value
person.name through ref Person
```

Ownership cannot transfer merely because the value is readable.

Copy may be possible when the value type is copyable.

---

# Automatic fix from copy to move

When ordinary copy syntax is invalid only because the source is move-only, the
compiler may provide a machine-applicable fix.

Examples:

```text
:= -> :<-
=  -> <-
```

The fix is automatically safe only when:

- source is an existing reusable place;
- move is legal;
- destination type is valid;
- no active borrow conflicts;
- source and destination do not overlap illegally;
- no overload or conversion changes;
- no later source use becomes invalid;
- no different error would remain.

If later source use exists, the LSP must show consequences and classify the
action as a refactoring or multi-edit fix.

Ordinary formatter never performs this inference.

---

# Reinitialization

A mutable binding may be reinitialized after move or discard.

Example after move:

```sec
let mut source := CreateBuffer()
let destination :<- source

source = CreateBuffer()
```

Example after discard:

```sec
let mut source := CreateBuffer()
discard source

source = CreateBuffer()
```

No previous value is destroyed during reinitialization.

An immutable binding cannot be reinitialized.

---

# Replacement ordering

For available destination:

```sec
destination = expression
```

or:

```sec
destination <- source
```

the compiler must preserve safe ordering.

Conceptual sequence:

```text
1. evaluate source expression;
2. validate conversions, contracts, borrows, and ownership;
3. ensure fallible operations have succeeded;
4. destroy old destination value;
5. install new value;
6. establish destination ownership;
7. mark moved source unavailable when applicable.
```

The old destination remains valid until commit.

---

# Self-assignment

Copy self-assignment:

```sec
value = value
```

may be accepted for copyable values and optimized to no operation.

The formatter and compiler may emit an advisory diagnostic.

Move self-assignment:

```sec
value <- value
```

is invalid.

Suggested diagnostic:

```text
cannot move value into itself
```

---

# Overlapping source and destination

The compiler must reject move operations where source and destination overlap in
a way that would invalidate evaluation.

Conceptual invalid example:

```sec
object.field <- object
```

Exact legality depends on place analysis.

Copy may use temporary storage when overlap is possible.

Backend `memcpy` versus `memmove` choice does not determine source semantics.

---

# Function arguments

A by-value parameter receives an owned parameter value.

For a copyable argument:

```sec
Process(value)
```

the caller value is copied.

The caller retains `value`.

For a move-only argument:

```sec
Consume(resource)
```

ownership transfers to the parameter.

The caller's `resource` becomes unavailable after successful argument transfer.

Sec 0.1 does not require call-site syntax:

```sec
Consume(<- resource)
```

The function signature and resolved type determine the transfer.

The LSP must expose consuming parameters.

---

# Reference parameters

Shared parameter:

```sec
fn Inspect(value: ref Buffer) void {
}
```

Mutable parameter:

```sec
fn Modify(value: ref mut Buffer) void {
}
```

Passing to these parameters does not transfer referent ownership.

The reference value itself follows reference copy/move classification.

---

# Argument evaluation order

Ownership transfer must follow the defined argument evaluation order.

Invalid patterns include one argument moving a value needed by a later
argument.

Example:

```sec
Use(Consume(resource), Inspect(resource))
```

must be rejected when the first argument consumes `resource`.

The diagnostic should explain the evaluation order and move origin.

---

# Fallible calls and argument ownership

For a call that may fail before accepting ownership, the function or ABI
contract must define ownership on every outcome.

Possible rules:

```text
ownership transfers before call
ownership transfers only on success
ownership returns to caller on failure
callee always consumes
```

Ordinary Sec by-value calls should use one deterministic language rule.

Special FFI or channel operations may define result-sensitive ownership
explicitly.

---

# Return values

## Return result context

Every non-reference return creates or transfers a function result owned by the
caller.

Canonical syntax:

```sec
return value
```

Sec does not use:

```sec
return <- value
```

`return` already identifies the result-transfer context.

---

# Returning an owned local

Example:

```sec
fn CreateBuffer() Buffer {
    let buffer := Buffer.Create()
    return buffer
}
```

The local `buffer` transfers into the return result.

The callee must not destroy it after return.

The caller becomes owner.

This applies even when the type is copyable.

The compiler should not preserve an unnecessary local copy whose scope is
ending.

---

# Returning a temporary

Example:

```sec
return Buffer.Create()
```

The temporary directly constructs or forwards into the return result.

No move token is required.

---

# Returning from borrowed storage

Example:

```sec
fn Name(person: ref Person) string {
    return person.Name
}
```

The function does not own `person.Name`.

If `string` is copyable, the field is copied into the return result.

If the field type is move-only, returning it by value is invalid.

A reference parameter does not grant ownership transfer.

---

# Returning static or shared storage

A copyable value may be copied from static or shared storage into the result.

A move-only value cannot be removed from static or shared storage without a
dedicated ownership handoff operation.

---

# Return-value optimization

The compiler may use:

```text
RVO
NRVO
caller-provided return storage
destination passing
SSA forwarding
register return
```

These are lowering strategies.

The semantic result remains one owned return value.

---

# Aggregate construction

An owning aggregate field is a consuming context for move-only input.

Example:

```sec
let session := Session {
    Name: name,
    File: file,
}
```

For each field:

- copyable named source is copied;
- move-only named source transfers;
- temporary constructs directly;
- reference field borrows or copies the reference according to type.

The aggregate becomes owner of moved payloads.

This is intentionally different from ordinary `:=` between two reusable
places.

---

# Result construction

Example:

```sec
return Ok(resource)
```

If `resource` is move-only, it transfers into the `Ok` payload.

If the payload is copyable, it may be copied.

The wrapper classification depends on both payload types.

Example:

```text
Result[int, Error]
    conditionally copyable if Error is copyable

Result[File, Error]
    move-only
```

---

# Option construction

Example:

```sec
let value := Some(resource)
```

A move-only `resource` transfers into `Some`.

`Option[T]` is copyable only when `T` is copyable.

---

# Union construction

A union variant owns its active payload according to the union rule.

Constructing a move-only payload transfers ownership.

Copying a union requires safe copy behavior for every active possibility or
proof of the active copyable variant.

Moving a union transfers active payload ownership.

---

# Conversion expressions

A conversion may:

```text
reuse representation
construct a new value
copy
move
borrow
validate
fail
```

The conversion rule must define ownership.

Current recursive move marking through conversions must be replaced by explicit
conversion ownership classification.

A conversion must not silently consume a named copyable source unless syntax or
conversion semantics explicitly require it.

---

# Spread expressions

Spread may copy or consume elements according to source and destination rules.

Ordinary spread of a copyable source may copy.

Spread of a move-only owning source must require an explicitly consuming
context.

The final spread rulebook defines syntax.

Semantic IR must record element-level or bulk ownership transfer.

---

# Fields

## Copyable field read

Example:

```sec
let name := person.Name
```

When `Name` is copyable, the field is copied.

Both remain available.

## Move-only field read with ordinary syntax

Invalid:

```sec
let file := session.File
```

when `File` is move-only.

Suggested help:

```text
use `let file :<- session.File` to transfer ownership
```

## Explicit field move

Example:

```sec
let file :<- session.File
```

When partial move is supported, the field becomes unavailable and the aggregate
becomes partially available.

---

# Partial moves

A partial move transfers selected fields while preserving others.

After:

```sec
let file :<- session.File
```

conceptual state:

```text
file
    Available

session.File
    Moved

session.Name
    Available

session
    PartiallyAvailable
```

Whole-value operations requiring complete `session` are invalid until
reinitialization.

---

# Initial partial-move policy

Partial field move may be supported only when:

- source aggregate is owned;
- field is directly named;
- no conflicting borrow exists;
- field state can be tracked independently;
- containing type has no opaque ownership invariant;
- containing type has no custom destruction requiring complete state;
- source is not volatile;
- source is not a register;
- source is not behind `RawPtr`;
- source is not foreign-opaque;
- union active-state rules are satisfied.

The initial implementation may limit this to local mutable struct bindings.

---

# Field reinitialization

A moved field in a mutable aggregate may be reinitialized.

Temporary:

```sec
session.File = OpenFile()
```

Existing move-only source:

```sec
session.File <- replacement
```

After every required field is available, the aggregate becomes complete.

No old moved field value is destroyed.

---

# Custom destruction and partial move

A type with custom destruction is move-only by default.

Partial move is initially forbidden unless its destruction contract explicitly
supports partial state.

Reason:

```text
custom destruction may depend on invariants spanning multiple fields
```

The compiler must not call complete-value destruction on an incomplete value.

---

# Arrays

## Array copy

An array is copyable only when:

- element type is copyable;
- no hidden allocation occurs;
- copy is semantically valid.

Copy duplicates every element.

Large copies remain legal when type rules permit them, but may receive
performance diagnostics.

## Whole-array move

A whole array may be moved explicitly:

```sec
let moved :<- values
```

Every element ownership transfers.

The source array becomes unavailable.

## Element read

```sec
let item := values[23]
```

copies the element when the element type is copyable.

## Move-only element

Ordinary read is invalid when the element is move-only.

Sec 0.1 also rejects:

```sec
let item :<- values[23]
```

for fixed arrays.

Moving one element would create a partially initialized fixed array.

This remains deferred until exact initialization tracking is defined.

---

# Fixed-array replacement

A mutable fixed array may replace an element with a temporary:

```sec
values[23] = CreateResource()
```

The previous element is destroyed after safe evaluation.

A method such as:

```sec
let previous := values.Replace(23, replacement)
```

may return the old value while keeping the array complete.

The core API must define exact behavior.

Indexed `<-` destination support requires a separate array-rule update.

---

# Slices

A slice is a non-owning view.

Copying a shared slice copies the descriptor and borrow relation.

It does not copy elements.

A mutable slice or mutable view follows exclusive-reference semantics and is
move-only unless reborrowed.

Moving a slice transfers the view and borrow obligation, not element ownership.

Ordinary indexing copies a copyable element.

Move-out through slice indexing is not supported in Sec 0.1.

---

# `string`

`string` is:

```text
immutable
implicitly copyable
explicitly movable
```

Example copy:

```sec
let first := "hello"
let second := first
```

Both remain available.

Example move:

```sec
let moved :<- first
```

`first` becomes unavailable.

Implicit string copy must be:

- infallible;
- free from hidden allocation;
- free from mutable aliasing;
- free from side effects.

The implementation may use:

```text
descriptor copy
static immutable storage
compiler-proven backing lifetime
arena-backed immutable storage
another equivalent representation
```

No hidden reference counting requirement follows.

---

# Dynamic collections

Owning dynamic collections are normally move-only unless the core type defines
an infallible, allocation-free copy representation.

Because Sec 0.1 has no implicit shared ownership, ordinary ownership of dynamic
elements should not be duplicated silently.

Explicit whole-collection duplication uses a named operation.

Example conceptual API:

```sec
let duplicate := try values.Copy()
```

Exact API belongs to core.

---

# Dynamic collection element reads

Ordinary element access copies the element when the element type is copyable.

Example:

```sec
let value := values[index]
```

The collection retains its element.

For move-only elements, ordinary copy access is invalid.

---

# Consuming `list` extraction

A mutable `list[T]` may support:

```sec
let item :<- values[index]
```

as consuming extraction.

Semantics:

- validate index;
- move selected element into `item`;
- remove the element;
- decrease list length;
- structurally mutate the list;
- preserve remaining element ownership;
- invalidate relevant views and iterators;
- obey iteration-freeze rules.

This feature is planned and not yet implemented.

It must not be enabled before parser, place analysis, core API, and collection
rules agree.

---

# Map extraction

Map lookup remains ordinary non-consuming lookup.

Consuming extraction uses an explicit operation until missing-key semantics are
locked.

Conceptual forms:

```sec
let removed := values.Remove(key)
let removed := values.Take(key)
```

The return type may be:

```text
Option[V]
Result[V, E]
another explicit type
```

The map rulebook decides.

---

# Set extraction

A set may expose:

```sec
let stored := values.Take(probe)
```

to remove and return the actual stored value.

`set` does not gain arbitrary index syntax solely for move.

---

# Other collections

Stack, queue, deque, ring buffer, heap, and similar types expose named consuming
operations such as:

```text
Pop
Take
Remove
Dequeue
Extract
```

The operations define ownership on success and failure.

---

# Iteration

Ordinary iteration does not silently consume the collection.

Loop variables may receive:

```text
copy
shared borrow
mutable borrow
explicit consuming item
```

according to iterator semantics.

A consuming iterator must be explicit in type or operation.

Structural mutation during ordinary iteration follows iteration-freeze rules.

---

# Structs

A struct's copy classification derives from fields and type semantics.

A struct with all copyable fields is not automatically copyable when it owns a
unique external identity.

A struct with one move-only field is move-only.

A custom destruction operation makes a struct move-only by default.

Explicit copy support must produce an independently valid struct.

---

# Enums

Fieldless enums are trivially copyable.

Enums with owned payloads, if supported, derive classification from payloads.

Aliases and integer representation do not alter semantic copy rules.

---

# Registers

A register value used as a local snapshot may be copyable according to its
register type semantics.

An addressed register:

```sec
@address(...)
```

represents fixed volatile hardware storage.

It is not an ordinary movable owned value.

Reading produces a snapshot or performs a volatile access according to register
rules.

A wrapper owning exclusive permission to a device may be move-only even when
the register block remains fixed.

---

# Hardware ownership

Move semantics matter for:

```text
device handles
DMA buffer ownership
interrupt registration tokens
peripheral lease objects
exclusive bus access
memory-mapped resource wrappers
```

Moving such a wrapper transfers responsibility.

It does not physically move the hardware.

Stable-address resources may require:

```text
move-only handle
non-movable storage
relocatable descriptor
```

These concepts must be represented separately.

---

# References

## Shared reference

```text
ref T
```

is copyable.

Copying creates another non-owning shared reference.

The referent ownership does not change.

## Mutable reference

```text
ref mut T
```

is move-only.

Copying it would violate exclusivity.

A reborrow may create another constrained mutable reference according to borrow
rules.

## Reference move

Moving a reference transfers the reference binding and borrow obligation.

It does not transfer the referent.

---

# Raw pointers

`RawPtr[T]` is trivially copyable.

Copy duplicates the address value only.

No ownership is implied.

Explicitly moving a raw pointer may make the source binding unavailable, but it
still does not transfer ownership of pointed storage.

FFI contracts define real ownership separately.

---

# Function values and closures

A plain function value is copyable.

A closure classification derives from captures.

A closure is copyable only when:

- every owned capture is copyable;
- no mutable reference would be duplicated;
- representation permits independent use;
- capture semantics allow copy.

A closure capturing a move-only value is move-only.

A closure capturing `ref mut` is move-only.

Moving the closure transfers captures.

---

# Interfaces

An interface representation must distinguish:

```text
borrowed interface reference
owned erased value
descriptor referring to external storage
move-only erased owner
```

An owned erased interface value is move-only unless independent copy semantics
are known.

A borrowed interface reference follows reference rules.

Current trivial interface classification is provisional and must be reviewed
before owned interface values are implemented fully.

---

# Named types

A named type does not inherit copyability solely from representation.

Classification depends on:

```text
base type
fields
contracts
units
custom destruction
resource ownership
explicit non-copyability
core-defined semantics
```

Simple named numeric types are normally copyable.

Resource wrappers are normally move-only.

---

# Contracts

Contracts do not themselves imply move-only behavior.

Copying a valid constrained value preserves validity when representation and type
semantics permit copy.

Assignment or reinitialization may require `try` because the destination type
contract is checked.

Move does not bypass destination contract validation when conversion or
reconstruction is required.

Pure same-type move should preserve already-proven validity.

---

# Units

Unit-bearing numeric values are copyable when their numeric representation is
copyable.

Exact unit conversion may construct another value.

Conversion ownership follows numeric value semantics and does not consume the
source unless explicitly requested.

---

# Generics

Generic code must express copy requirements.

Example:

```sec
let second := first
```

requires generic `T` to be copyable.

The compiler must not instantiate this as copy for one `T` and silent move for
another `T`.

If the generic intends transfer, it uses:

```sec
let second :<- first
```

or a consuming parameter/result context.

The type system needs constraints or inferred requirements representing:

```text
Copyable
Movable
ExplicitlyNonCopyable
```

Exact syntax belongs to generics and interfaces rules.

---

# Overload resolution

Copy versus move must not be selected as a hidden overload discriminator for
ordinary source syntax.

For:

```sec
let destination := source
```

the compiler first requires copyability.

It does not choose a consuming overload because copy failed.

For function calls, resolved parameter modes and types determine argument
transfer.

LSP signature help must show consumption.

---

# Methods and receivers

A method receiver may be:

```text
shared borrowed
mutable borrowed
owned consuming
ordinary compiler-known instance receiver
```

A consuming receiver transfers ownership of the receiver value.

The source becomes unavailable.

The exact method declaration syntax for consuming receivers requires its own
rule if not already represented by value receiver semantics.

Implicit `self` does not remove ownership analysis.

---

# Properties

Property get may:

```text
copy a value
return a reference
construct a value
transfer from owned storage only through explicit consuming semantics
```

A normal getter must not silently move a move-only field out of a reusable
object.

Property set performs replacement or reinitialization according to target state.

Fallible setters require `try`.

---

# Static values

Static owned values have static lifetime.

A move-only static value cannot be silently moved into a shorter-lived local.

Access requires:

```text
borrow
guard
explicit static handoff
type-specific operation
```

Copyable static values may be copied.

---

# Arenas

An arena owns storage.

Moving an arena-backed logical value transfers its logical ownership only when
arena lifetime rules permit.

It does not transfer arena ownership unless the arena handle itself is moved.

Copying an arena-backed descriptor must not outlive the arena.

Reset invalidates values and references according to generation rules.

---

# Allocation

Allocation constructs owned storage.

The returned value may be move-only.

Moving allocation ownership transfers responsibility for release.

Copying an owning allocation is invalid unless an explicit deep-copy operation
exists.

No silent heap allocation may occur during ordinary copy.

---

# Destruction

Copy creates a new destruction responsibility only when the copied result owns
independent resources.

Move transfers destruction responsibility.

The source must not be destroyed after move.

Replacement destroys the old destination after safe source evaluation.

Reinitialization destroys no old destination value.

Partial aggregates destroy only still-initialized fields when partial state is
supported.

---

# Discard

`discard` consumes and destroys a value.

It is not copy.

It is not move to another owner.

After:

```sec
discard value
```

the source becomes unavailable.

A mutable binding may be reinitialized.

Implicit discarded call results follow `discard.md`.

---

# Must-use values

Must-use is distinct from move-only.

Examples:

```text
Result[int, Error]
    may be copyable
    always must-use

Thread[int]
    move-only
    must-use
    non-discardable while unresolved
```

Copy/move classification does not remove must-use handling requirements.

---

# Tasks and threads

Task and thread handles are move-only while they represent unique lifecycle
obligations.

Passing, returning, storing, or sending them transfers ownership.

They cannot be copied.

Move source becomes unavailable.

Join, await, detach, and cancel semantics determine lifecycle resolution.

---

# Channels

Sending a move-only value transfers ownership according to channel operation
semantics.

On success:

```text
channel or receiver owns the value
```

On failure:

```text
ownership must remain with or return to sender
```

Revocable and cancellable sends must define every outcome.

Copyable values may be copied if the channel API specifies copy.

The compiler must not lose ownership on a failed send.

---

# Select

Select requires branch-sensitive ownership.

A value offered to multiple alternative send branches cannot be transferred
into all branches simultaneously.

The selected branch receives ownership according to operation semantics.

Unselected branches retain no transfer.

Current special-case tracking should be replaced by general select ownership
analysis.

---

# Transferability

A value may be movable within one thread but not transferable across threads.

Copy classification and transferability are separate.

Examples:

```text
ref T
    copyable reference value
    may not be thread-transferable

File
    move-only
    may be transferable

MutexGuard[T]
    move-only
    may be non-transferable
```

Concurrency transfer follows `transferability.md`.

---

# FFI

FFI ownership must be explicit.

A foreign declaration eventually needs metadata for:

```text
borrowed argument
consumed argument
ownership transfer on success
ownership transfer on failure
owned return
borrowed return
retained pointer
release function
allocator identity
thread restrictions
```

In the absence of explicit metadata:

- `RawPtr[T]` is non-owning;
- references may not escape;
- ownership is not assumed transferred;
- returned pointers are not automatically owned.

Unsafe does not disable ownership rules.

---

# Copy size policy

Copy legality does not depend solely on size.

Large copyable values may include:

```text
large fixed arrays
large structs
wide vectors
matrices
register snapshots
```

The compiler may:

- emit an advisory performance diagnostic;
- optimize the copy away;
- pass indirectly according to ABI;
- use destination storage;
- suggest `ref`;
- suggest explicit move when the source is no longer needed.

It must preserve copy semantics.

Suggested advisory:

```text
performance.large-value-copy
```

---

# Address stability and relocation

Copyability, movability, relocatability, address stability, and pinning are
separate properties. None may be inferred solely from another.

A type may be:

```text
relocatable
address-stable
pinned
fixed-address
```

A valid source-level move transfers ownership and makes the source unavailable.
It does not prove that physical bytes may be moved to a new address. The
compiler may reuse storage, transfer metadata, elide the move, construct into
destination storage, or preserve a stable address.

A semantic move of a relocatable value may physically move bytes.

A semantic move of a stable-address resource may transfer only a handle while
storage remains fixed.

A fixed-address register cannot be relocated.

The exact relocation classification belongs to memory and storage rules.

---

# Atomic values

Atomic values may be copyable as values only through defined atomic load
semantics.

Copying an atomic storage object is not necessarily ordinary representation
copy.

Moving an atomic storage location may be forbidden because synchronization
identity is tied to address.

The atomics rulebook defines exact behavior.

---

# Mutexes and locks

Mutex is non-copyable and supports explicit ownership transfer under the
currently implemented move rules.

This statement does not imply that every initialized mutex may always be
physically relocated. Address stability, pinning, waiter state, foreign
integration, and target implementation may impose independent restrictions.

A `MutexGuard[T]` is move-only.

Moving the guard transfers unlock responsibility.

Copying the guard is invalid.

Discarding the guard releases according to destruction rules.

---

# Events and subscriptions

An event descriptor may be copyable if non-owning.

A subscription token owning registration lifetime is move-only unless explicit
shared subscription semantics exist.

Moving transfers unregister responsibility.

---

# Error paths

Every fallible operation must preserve ownership on all outcomes.

Example conceptual move replacement:

```sec
try destination <- source {
    Err(error) => {
        // Ownership rule must define destination and source states here.
    }
}
```

If the move itself cannot fail but destination contract conversion can fail,
ownership must not transfer before validation succeeds.

Source remains available on failure unless the operation explicitly returns it
through another result.

---

# `try` and constrained destinations

Assignment to a constrained named type may be fallible.

Example:

```sec
try percent += Percent(i) {
    Err(error) => {
        discard error
    }
}
```

For move assignment into a constrained destination:

- validate before committing move where possible;
- preserve source on failed validation;
- destroy old destination only after success;
- mark source moved only after commit.

Semantic IR must model commit ordering.

---

# Control flow

Ownership state is path-sensitive.

After:

```sec
if condition {
    let moved :<- value
}
```

`value` is not definitely available on all paths.

Use after the `if` is invalid unless every continuing path establishes an
available value.

---

# Branch merge

Conceptual state merge:

```text
Available + Available
    Available

Moved + Moved
    Moved

Available + Moved
    ConditionallyAvailable

Discarded + Available
    ConditionallyAvailable

PartiallyAvailable + Available
    field-sensitive merged state
```

A non-continuing branch does not constrain later state.

The current implementation's union of moved maps is a conservative first step.

---

# Loops

Loop ownership requires fixed-point analysis.

A value moved in one iteration must be reinitialized on every path reaching the
next iteration.

Invalid:

```sec
while running {
    Consume(resource)
}
```

unless `resource` is recreated before continuation.

Break and continue states must be merged separately.

---

# Match

Match patterns may:

```text
borrow payload
copy payload
move payload
bind complete variant
ignore payload
```

The pattern form must determine behavior.

Moving a payload makes the matched source partially or completely unavailable
according to type rules.

Exhaustive arms merge ownership state.

---

# Switch

Switch expressions are normally read or copied according to type.

A switch must not silently consume its subject unless switch syntax explicitly
defines consumption.

Case control flow merges ownership state.

---

# Defer

A deferred operation may retain a borrow or future use.

Moving or discarding the value earlier is invalid when the deferred operation
still requires it.

Example:

```sec
defer Close(resource)
let moved :<- resource
```

must be rejected.

The diagnostic points to both defer capture and move.

---

# Panic and cleanup

Panic behavior must destroy only values still owned at the panic point.

Moved sources are not cleaned up.

Partially constructed destinations clean up installed fields.

The panic model may abort or unwind by profile, but must not duplicate
destruction.

---

# ABI parameters

Physical parameter passing may use:

```text
registers
stack slots
hidden pointers
caller-provided storage
split values
```

Semantic copy/move remains fixed before ABI lowering.

A by-value move-only parameter transfers ownership regardless of physical
strategy.

---

# ABI return values

An owned return transfers result ownership to the caller.

Physical strategy may include:

```text
return registers
split registers
hidden return pointer
caller-allocated result
target-specific aggregate return
```

The callee must not destroy the returned result.

The caller establishes cleanup responsibility after successful return.

---

# SSA

SSA values do not define ownership.

A move may reuse the same SSA value.

A copy may be optimized away.

Still:

```text
SSA reuse is not semantic reuse
SSA renaming is not copy
SSA forwarding is not ownership transfer
```

Semantic IR records ownership explicitly.

---

# Semantic IR

Required operations include:

```text
ConstructValue
CopyValue
MoveValue
BorrowShared
BorrowMutable
ReplaceValue
ReinitializeValue
DiscardValue
DestroyValue
ReturnValue
TransferArgument
TransferField
TransferCollectionElement
```

Each operation records where relevant:

```text
source
destination
type
source place
destination place
copy classification
source state before
source state after
destination state before
destination state after
destruction responsibility
borrow relationship
source location
explicit or inferred origin
```

---

# Move origins

Semantic IR must distinguish:

```text
ExplicitMoveInitialization
ExplicitMoveAssignment
InferredReturnTransfer
InferredArgumentTransfer
InferredAggregateTransfer
InferredResultPayloadTransfer
InferredOptionPayloadTransfer
InferredTemporaryTransfer
ChannelTransfer
LifecycleTransfer
FFITransfer
```

This supports diagnostics, LSP hints, and backend verification.

---

# Copy origins

Semantic IR should distinguish:

```text
OrdinaryInitializationCopy
OrdinaryAssignmentCopy
ArgumentCopy
BorrowedStorageReturnCopy
FieldReadCopy
CollectionElementCopy
ExplicitNamedDuplication
CompilerGeneratedCopy
```

---

# MLIR lowering

MLIR receives resolved semantic operations.

It may implement:

```text
copy elision
move elision
destination passing
buffer reuse
in-place update
tensor bufferization
register forwarding
stack-slot reuse
memcpy
memmove
target-specific handle transfer
```

It must preserve:

```text
source availability
destruction count
external resource ownership
borrow validity
failure-path ownership
address stability
volatile behavior
```

MLIR must not choose whether source syntax means copy or move.

---

# LLVM lowering

LLVM lowering may optimize physical data movement.

It must not:

```text
duplicate unique ownership
destroy moved sources
drop destination destruction
end lifetime too early
relocate fixed-address storage
```

LLVM lifetime intrinsics are optimization metadata, not source lifetime truth.

---

# Internal compiler invariants

Before Semantic IR completes:

- every value use has a semantic operation;
- every type has resolved copy classification;
- every ordinary named-source copy uses a copyable type;
- every explicit move has an available source;
- every transfer has one destination;
- every source-after state is known;
- every branch merge is valid;
- every partial move has valid field state;
- every generic requirement is satisfied.

Before MLIR lowering completes:

- destruction responsibility is fixed;
- cleanup paths are explicit;
- every semantic copy has a valid implementation;
- every semantic move has a valid continuation;
- address stability is respected;
- ABI transfer preserves ownership.

Any violation after frontend verification is an internal compiler error.

---

# Diagnostics

Copy/move safety diagnostics are hard errors.

Suggested stable rules:

```text
ownership.copy-of-move-only
ownership.use-after-move
ownership.move-while-borrowed
ownership.invalid-self-move
ownership.overlapping-move
ownership.reinitialize-immutable
ownership.partial-move-unsupported
ownership.fixed-index-move-out
ownership.copy-requires-explicit-operation
ownership.move-from-nonowned-storage
ownership.move-from-fixed-address
```

---

# Copy of move-only

Example:

```sec
let destination := source
```

Diagnostic:

```text
error[S....]: Buffer cannot be copied from source
```

Help:

```text
use `let destination :<- source` to transfer ownership
```

The diagnostic includes a safe fix when proven.

---

# Use after move

Example:

```sec
let moved :<- source
Use(source)
```

Diagnostic:

```text
error[S....]: value source is unavailable because it was moved
```

Related location:

```text
value moved here
```

This error cannot be configured away.

---

# Move while borrowed

Diagnostic:

```text
error[S....]: cannot move source while it is borrowed
```

Related location identifies borrow origin and holder.

---

# Move from borrowed storage

Diagnostic explains that readable access does not imply ownership.

Suggested help:

```text
copy the value if it is copyable, return a reference, or add an explicit ownership-taking operation
```

---

# Large copy advisory

Example:

```text
info[A....]: copying Frame copies 8192 bytes
help: pass by `ref` or explicitly move the value when the source is no longer needed
```

Severity is configurable.

The copy remains valid.

---

# LSP integration

The LSP must expose:

```text
copy classification
operation at cursor
move origin
destination
availability state
consuming parameter
copy size
partial aggregate state
safe move fix
borrow alternative
destruction point
```

Inlay examples:

```text
value /* copied */
resource /* moved */
```

A move fix preview shows later source uses.

---

# Formatter integration

Canonical formatting:

```sec
let destination :<- source
let destination: Buffer <- source
destination <- source
```

Ordinary formatter preserves:

```text
:= versus :<-
= versus <-
```

`sec fmt --fix` may apply a proven move correction.

It must never change direct temporary initialization:

```sec
let value := CreateBuffer()
```

---

# Required tests

Create or update:

```text
copy_move_valid.sec
copy_move_invalid.sec
copy_move_control_flow_valid.sec
copy_move_control_flow_invalid.sec
copy_move_struct_valid.sec
copy_move_struct_invalid.sec
copy_move_collections_valid.sec
copy_move_collections_invalid.sec
copy_move_ffi_valid.sec
copy_move_ffi_invalid.sec
copy_move_hardware_valid.sec
copy_move_hardware_invalid.sec
```

Every invalid test contains:

```sec
/* Expected error: ...
 * Reason: ...
 */
```

---

# Core valid cases

## Copy

```sec
let first := "hello"
let second := first

Use(first)
Use(second)
```

## Explicit move of copyable value

```sec
let first := "hello"
let second :<- first

Use(second)
```

Later use of `first` is invalid.

## Move-only declaration

```sec
let first := CreateBuffer()
let second :<- first
```

## Direct temporary

```sec
let value := CreateBuffer()
```

## Reinitialization

```sec
let mut first := CreateBuffer()
let second :<- first

first = CreateBuffer()
```

## Return

```sec
fn Create() Buffer {
    let value := CreateBuffer()
    return value
}
```

## By-value parameter

```sec
fn Consume(value: Buffer) void {
}

let value := CreateBuffer()
Consume(value)
```

Later use is invalid.

---

# Core invalid cases

## Implicit named move

```sec
let first := CreateBuffer()
let second := first
```

## Assignment implicit move

```sec
let mut destination := CreateBuffer()
let source := CreateBuffer()

destination = source
```

## Use after move

```sec
let source := CreateBuffer()
let destination :<- source

Use(source)
```

## Immutable reinitialization

```sec
let source := CreateBuffer()
let destination :<- source

source = CreateBuffer()
```

## Move while borrowed

```sec
let source := CreateBuffer()
let view := ref source
let destination :<- source
```

## Fixed array element move

```sec
let item :<- values[2]
```

---

# Compiler unit tests

Required Go tests include:

```text
copy classification
classification recursion
custom destruction classification
ordinary source copy
explicit move
temporary classification
place classification
reinitialization
replacement ordering
borrowed move rejection
branch merge
loop fixed point
return transfer
argument transfer
aggregate transfer
partial field state
array rejection
list extraction
diagnostic fixes
Semantic IR origins
MLIR verification
```

---

# Required synchronization

This rulebook must remain synchronized with:

```text
ownership.md
discard.md
formatter.md
lsp.md
types.txt
contracts.md
functions.txt
struct.txt
arrays-slices.txt
collections-shaped-types.md
borrowing.txt
references.txt
lifetime_analysis.txt
destruction.txt
allocation.txt
memory_model.md
transferability.md
channels.md
tasks.txt
threads.md
processes.txt
ffi.txt
registers.txt
static.md
semantic_ir.txt
compiler_pipeline.txt
diagnostics.txt
grammar.md
operators.md
lexical_structure.md
rules_implementations.txt
language-rulebook-status.md
```

---

# Appendix A — Codex replacement plan

## A.1 Rename rulebook

The filename migration is complete. `rules/copy_move.md` is canonical,
repository references are updated, and no duplicate canonical file remains.

## A.2 Preserve existing classification

Retain and test the existing internal classification API:

```text
CopyClassificationOf
TriviallyCopyable
MoveOnly
```

Adjust classifications only where this rulebook requires.

## A.3 Add lexer tokens

Add:

```text
MOVE_INIT  for :<-
MOVE_ASSIGN for <-
```

Exact internal names may differ.

Longest-match rules:

```text
:<- before :
<- before <
```

Do not assign meaning to `->`.

## A.4 Extend AST declarations

Add initialization mode to `LetStatement`.

Conceptual:

```go
type InitializationMode int

const (
    InitializationOrdinary InitializationMode = iota
    InitializationMove
)
```

Preserve source token and range.

Typed form:

```sec
let value: Type <- source
```

uses move mode.

## A.5 Extend assignment AST

Preserve `<-` as a distinct assignment operator.

Sema converts it into a move operation.

Do not treat it as compound arithmetic assignment.

## A.6 Classify expression source

Implement a reusable place classifier.

Conceptual categories:

```text
Temporary
LocalPlace
FieldPlace
IndexPlace
StaticPlace
BorrowedPlace
FixedAddressPlace
RawPointerPlace
UnknownPlace
```

Every copy/move validation uses this classification.

## A.7 Replace implicit named move

Remove current behavior where `markMoveSource` is called unconditionally after
ordinary declaration or assignment.

Required ordinary behavior:

```text
named reusable source + copyable type
    copy

named reusable source + move-only type
    error with move fix

temporary
    direct construction or forwarding
```

## A.8 Explicit move handling

For `:<-` and `<-`:

- require movable source place;
- require source availability;
- reject borrow conflict;
- reject illegal overlap;
- mark source moved;
- establish destination;
- transfer destruction responsibility.

Allow explicit move of copyable types.

## A.9 Retain inferred consuming contexts

Continue to infer transfer for move-only values in:

```text
return
by-value parameter
struct field construction
Result payload
Option payload
union payload
channel transfer
lifecycle transfer
```

Do not require call-site or return arrow syntax.

## A.10 Reinitialization

Change current state handling:

```text
Moved mutable + assignment
    reinitialize

Discarded mutable + assignment
    reinitialize

Uninitialized mutable + assignment
    initialize

Available mutable + assignment
    replace
```

Remove the current hard error rejecting assignment to a discarded mutable
binding.

Reject immutable reinitialization.

## A.11 Move state model

Replace separate root maps over time with structured state.

Conceptual:

```go
type AvailabilityState int

const (
    Uninitialized AvailabilityState = iota
    Available
    PartiallyAvailable
    Moved
    Discarded
    Detached
    ConditionallyAvailable
)
```

Record reason, token, field path, and branch origin.

## A.12 Field-sensitive places

Implement field paths:

```text
root
root.field
root.field.subfield
```

Stage implementation:

1. diagnostics for ordinary copy of move-only field;
2. explicit direct field move;
3. partial aggregate state;
4. field reinitialization;
5. partial destruction;
6. branch merging.

## A.13 Arrays and slices

Preserve current rejection of move-only indexed extraction.

Add specific diagnostic ID and help.

Do not add runtime initialization bitmap in Sec 0.1.

## A.14 Lists

Do not implement indexed consuming extraction until:

```text
list core API
parser
place classification
borrow invalidation
iteration freeze
bounds result
```

are synchronized.

When implemented, lower:

```sec
let value :<- list[index]
```

as one structural extraction operation.

## A.15 String

Preserve source classification:

```text
string -> copyable
```

Verify no hidden allocation or mutable aliasing occurs.

Add explicit move support.

## A.16 Interface review

Current interfaces are classified trivially copyable.

Review owned interface representation before relying on that classification.

Borrowed interface descriptors may be copyable.

Owned erased values should derive or carry move-only status.

## A.17 Calls

Preserve current `markMovedCallArguments` behavior conceptually, but replace it
with explicit transfer operations.

Expose parameter consumption metadata to LSP.

Respect argument evaluation order.

## A.18 Returns

Preserve current move/resource transfer for owned locals.

Represent one `ReturnValue` transfer in Semantic IR.

Do not add `return <-`.

## A.19 Aggregate construction

Replace recursive `markMoveSource` with explicit field/payload transfer
classification.

Record destination ownership.

Handle failure cleanup for partially constructed aggregates.

## A.20 Diagnostics

Register stable IDs.

Attach machine-applicable edits for:

```text
:= -> :<-
=  -> <-
```

only when safe.

Use related locations for use after move and borrow conflicts.

## A.21 Semantic IR

Implement explicit operations and origins defined above.

No verified source program may reach MLIR with unresolved copy/move intent.

## A.22 MLIR

Add ownership-aware lowering.

Verify:

```text
no copied unique resource
no moved source destruction
no lost destination cleanup
no fixed-address relocation
```

## A.23 Formatter

Use the shared formatter.

Add token spacing and golden tests.

Ordinary formatting preserves operation.

`--fix` consumes compiler-provided edits.

## A.24 LSP

Add:

```text
hover classification
move inlay hints
consuming parameter hints
go to move
safe move quick fix
partial state display
consequence preview
```

## A.25 Status documents

Update:

```text
language-rulebook-status.md
rules_implementations.txt
```

Current status:

```text
copy_move.md
    Living

explicit move syntax
    Implemented through lexer, parser, AST, Sema, formatter preservation,
    semantic-token classification, and initial LSP quick fixes

current implicit named move behavior
    replaced for direct named declaration and assignment sources;
    general place-sensitive analysis remains incomplete
```

## A.26 Implementation order

Recommended order:

```text
1. Add lexer tokens.
2. Add parser and AST modes.
3. Add source-place classification.
4. Reject ordinary copy of move-only named sources.
5. Implement explicit move.
6. Add safe diagnostics and fixes.
7. Allow mutable reinitialization after discard.
8. Add Semantic IR operations.
9. Update formatter and LSP.
10. Add field-sensitive moves.
11. Add list consuming extraction.
12. Complete MLIR and ABI verification.
```

---

# Design summary

Sec distinguishes copy from move.

Ordinary `:=` and `=` copy from existing reusable places.

They never silently move a named move-only source.

Explicit move uses:

```sec
let destination :<- source
let destination: Type <- source
destination <- source
```

Fresh temporaries require no move token.

`return expression` is already a result-transfer context.

By-value parameters and aggregate payload construction infer transfer for
move-only values.

A copyable value, including `string`, may be explicitly moved.

Every moved source becomes unavailable.

Mutable bindings may be reinitialized.

Immutable bindings may not.

Fixed-array indexed move-out is deferred.

Dynamic `list` consuming extraction is planned.

Semantic IR records every copy and move explicitly.

No backend phase may revise source-level copy or move meaning.
