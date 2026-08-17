# Ownership

## Status

This document is the canonical ownership rulebook for Sec. The former legacy
text rulebook has been replaced and is no longer canonical.

This rulebook defines the source-language ownership model.

Detailed copy classification, borrowing, lifetimes, destruction, discard,
transferability, and lowering remain synchronized with their specialized
rulebooks.

`copy_move.md` is the canonical rulebook for copy classification, derived
copyability, named duplication, and explicit move syntax.

Default initialization establishes an available owned value when the type is
owning. Defaultability is independent of copyability and move-only status.
Default construction creates the type's ordinary destruction responsibility,
but it must not silently allocate or acquire a unique external resource. See
`default_values.md`.

---

# Current implementation status

## Implemented

The current compiler already implements substantial ownership foundations:

- local symbol state for assigned and moved values;
- source locations for moves;
- move reasons including ordinary move, discard, and detach;
- hard diagnostics for use after move;
- hard diagnostics for use after discard;
- related source locations identifying where a value became unavailable;
- basic move-only type classification;
- ownership transfer from move-only values in existing consuming contexts;
- transfer of move-only by-value function arguments;
- transfer of returned move-only locals;
- transfer of move-only payloads used in aggregate and result construction;
- borrow checks before moving a tracked local;
- branch-sensitive moved-state propagation through several control-flow forms;
- unresolved task and thread checks at scope exit;
- tracked resource checks for open file-like resources;
- reinitialization of a moved mutable binding through ordinary assignment;
- clearing of moved state after successful mutable reinitialization;
- explicit `discard` state tracking;
- rejection of indexed extraction of move-only elements from arrays and slices;
- AST assignment nodes that retain the source assignment operator as text;
- lexer and parser support for `:<-` and `<-`;
- AST ownership mode for declarations and assignments;
- rejection of ordinary copy syntax for named move-only sources;
- explicit moves from named sources, including copyable values;
- `@noCopy` parsing and AST storage on nominal type declarations;
- propagation of explicit nominal non-copyability through generic instances
  and containing aggregates;
- semantic-token and TextMate grammar classification for move operators and
  `@noCopy`;
- a user-invoked LSP quick fix for the proven `S1007` implicit-move diagnostic,
  which changes `:=` to `:<-` or `=` to `<-` and explains that the source
  becomes unavailable.
- a structured compile-time `Place` representation with root identity, field,
  property, constant/dynamic index, slice, and reference-dereference
  projections;
- place mutability and addressability classification without runtime metadata;
- conservative place-overlap analysis with disjoint struct fields, distinct
  constant array indices and normalized static slice intervals;
- place-sensitive borrow creation, read, assignment, and self-move overlap
  checks on the integrated expression forms;
- canonical referent Places for directly created local reference and slice
  aliases, including field/index reborrows and provenance preservation across
  shared-reference copies and mutable-reference moves, with the granting borrow
  transferred to the new mutable-reference binding;
- normalized static slice Place intervals with disjoint-range, slice/index,
  empty-range and composed local-alias overlap analysis;
- rejection of borrows from temporary and non-addressable expressions;
- explicit moves from nested struct-field places;
- field-sensitive unavailable state for moved struct fields, preserving access
  to disjoint sibling fields;
- rejection of whole-aggregate use while a field remains moved;
- reinitialization of a moved field through successful field assignment;
- conservative rejection of partial moves through references, properties,
  registers, indexes, slices, fixed-address storage, and other nonlocal roots;
- projection-sensitive move-state merging across continuing `if`, `switch`,
  `select`, and statement-`match` paths, including all-branch field
  reinitialization;
- a Place-availability loop fixed point that joins entry, normal back-edge and
  `continue` states, then rechecks the next condition and iteration;
- separation of `break` exits from back-edges and per-iteration restoration of
  `for` bindings;
- loop-header may-state joins for active borrows and local reference origins,
  including normal fallthrough and `continue`, with next-iteration rechecking;
- post-loop joins including explicit `break` borrow/reference states and
  bounded path-disjunctive provenance for known origins and conservative
  rejection only after provenance becomes missing or over-limit;
- nested, field-sensitive reference provenance for locally constructed struct
  and fixed-array aggregates through transfer, replacement assignment,
  constant/dynamic indexing and control-flow joins;
- compile-time function summaries that instantiate returned parameter/receiver
  Place sets, projections and aggregate paths at direct calls and transfer the
  returned reference's granting borrow to its caller-side holder;
- compile-time must-closed state for local `io.File` and `io.Directory`, joined
  by intersection across branches and across loop entry, fallthrough,
  `continue`, and `break` cleanup edges;
- union variant payload Place projections, including payloads nested below
  ordinary struct fields;
- inferred move of move-only payload bindings from reusable local union
  subjects, ordinary copy of copyable payloads, and forwarding from temporary
  subjects;
- union-payload availability merging for statement and expression `match`,
  whole-subject invalidation, reinitialization, and overlapping-borrow checks.
- explicit branch-scoped `ref` and `ref mut` bindings for named-union and Result
  payload Places, including mutability, conflict and escape validation;

These implementation facts do not override the normative rules below.

---

## Partially implemented

The current ownership implementation is incomplete or follows older semantics in
these areas:

- move availability uses places for roots and nested struct-field projections;
  indexes, slices and aliased move state remain incomplete even though local
  reference-alias borrow provenance is canonicalized;
- aliased field availability remains incomplete;
- copy classification is incomplete for several core and collection types;
- compiler-known and source-declared `@noCopy` types are classified and
  enforced for direct named-source copy;
- `string` is not yet fully enforced as the source-level copyable value defined
  by this rulebook;
- branch merging distinguishes available and unavailable nested-field paths,
  but does not yet expose the complete initialization and destruction lattice;
- destruction responsibility is tracked only indirectly;
- move diagnostics do not yet consistently use stable registered IDs beyond
  ordinary-copy rejection;
- Semantic IR does not yet record every copy, move, initialization, and
  reinitialization operation explicitly;
- MLIR lowering still relies on frontend-specific behavior for some values.

---

## Not implemented

The following are not yet implemented:

- aliased aggregate partial-move analysis;
- multi-field active-variant borrowed destructuring;
- recursive aggregate availability analysis;
- dynamic-list indexed consuming extraction;
- full collection-specific consuming extraction;
- complete type-level copy classification;
- the complete general attribute parser beyond the implemented `@noCopy`
  nominal-type path;
- user-defined must-use and ownership attributes;
- complete FFI ownership contracts;
- explicit Semantic IR operations for construction, copy, move, replacement,
  and reinitialization;
- `sec fmt --fix` ownership corrections;
- LSP ownership visualization and additional code actions;
- complete ownership integration tests.

---

# Purpose

Ownership determines which value or storage place is responsible for a value's
lifetime and destruction.

Sec ownership must guarantee:

- every owned value has one current owner;
- every owned value is destroyed at most once;
- ownership transfer is statically known;
- use after move is rejected;
- use after discard is rejected;
- reinitialization is distinguished from replacement;
- references do not silently become owners;
- raw pointers do not imply ownership;
- no runtime ownership table is required;
- backend representation does not redefine source semantics.

Ownership complexity belongs primarily in the compiler.

Source syntax must still make destructive transfer between reusable places
visible.

---

# Core terminology

## Value

A value is a typed semantic entity.

Examples include:

```text
integer value
string value
struct value
array value
list value
file handle wrapper
thread handle
reference value
```

A value may or may not own storage or an external resource.

---

## Place

A place is a source location that can contain a value.

Examples:

```sec
value
person.name
array[index]
object.property
```

Not every expression is a reusable place.

A function call result is normally a temporary value rather than a reusable
source place.

---

## Binding

A binding associates a source name with a place or value.

Example:

```sec
let value := CreateValue()
```

`value` is the binding.

The created result is the value owned by that binding.

A binding may continue to exist lexically after its value has been moved or
discarded.

The binding then has no available value.

---

## Owner

The owner is the binding, aggregate field, collection slot, return result,
runtime handle, or other semantic location responsible for a value's lifetime.

Ownership is not necessarily equal to physical memory location.

Moving a value may transfer ownership without moving bytes.

---

## Temporary

A temporary is a value produced during expression evaluation without a reusable
user-visible source binding.

Examples:

```sec
CreateBuffer()
left + right
Person {
    name: "Ada",
}
```

A temporary may directly initialize or replace a destination.

It does not require explicit move syntax because there is no reusable named
source that could be accidentally consumed.

---

## Storage

Storage is the physical location or abstract backend value holding a value.

Examples include:

```text
stack slot
register
static storage
arena allocation
heap allocation
MLIR tensor value
memref-backed buffer
device handle field
```

Ownership semantics are defined before backend storage selection.

---

# Ownership guarantees

For every owned value, the compiler must determine:

- where ownership begins;
- who currently owns the value;
- whether the value is available;
- whether an operation copies or moves it;
- which borrows exist;
- who is responsible for destruction;
- where ownership ends or transfers;
- whether a lifecycle obligation remains unresolved.

No valid program may:

- read a moved value;
- read a discarded value;
- move a value twice;
- destroy a value twice;
- silently copy a move-only value;
- move through an incompatible active borrow;
- abandon a non-discardable lifecycle obligation;
- infer ownership from `RawPtr[T]`;
- silently create shared ownership.

---

# Ownership and target hardware

Ownership is source-language semantics and does not depend on target
architecture.

The physical implementation may differ by target.

Examples:

```text
hosted target
    an owned value may use allocator-backed storage

bare-metal target
    an owned value may use static, stack, arena, or device-specific storage

GPU target
    an owned shaped value may lower through device memory or SSA tensor values

MMIO target
    a source declaration may represent fixed-address volatile storage
```

A semantic move may lower to:

- no machine instruction;
- register assignment;
- descriptor copy;
- pointer transfer;
- resource-handle transfer;
- destination-style bufferization;
- target-specific operation.

Ownership transfer must never be inferred from the emitted machine instructions.

Moving responsibility for a hardware handle does not necessarily read, write, or
relocate the underlying hardware resource.

Addressed registers, MMIO storage, and volatile hardware state are not ordinary
movable owned values unless their dedicated rules explicitly define such an
operation.

---

# Ownership states

A binding or tracked place may have one of these semantic states:

```text
Uninitialized
Available
PartiallyAvailable
Moved
Discarded
Detached
ConditionallyAvailable
```

The implementation may use a more detailed internal lattice.

---

## Uninitialized

The place exists but no value has been constructed in it.

Example:

```sec
let mut resource: Resource
```

An uninitialized value cannot be read, copied, moved, borrowed, or destroyed.

---

## Available

The place contains a valid initialized value.

Normal operations are permitted according to mutability, copy classification,
borrows, and type rules.

---

## Partially available

An aggregate has one or more unavailable owned fields while other fields remain
available.

The complete aggregate cannot be used as a whole.

Supported direct fields may still be used independently.

Partial availability is initially limited to specifically permitted struct
fields.

---

## Moved

Ownership has transferred elsewhere.

The source no longer contains an available source-level value.

The source must not be:

- read;
- copied;
- borrowed;
- moved again;
- destroyed.

A mutable binding may later be reinitialized.

---

## Discarded

The previous value was explicitly consumed and destroyed.

The binding remains unavailable.

A mutable binding may later be reinitialized.

---

## Detached

A lifecycle handle was consumed by an explicit detach operation.

The binding remains unavailable.

Detach-specific rules determine what happens to the execution result and
lifecycle obligation.

---

## Conditionally available

Control flow does not prove that the value is available on every continuing
path.

The value cannot be used until analysis proves one valid state on all relevant
paths.

---

# Mutability and availability

Mutability and availability are separate.

```text
immutable binding
    cannot be assigned a new value after initialization

mutable binding
    may be replaced or reinitialized

available binding
    currently contains a valid value

unavailable binding
    currently contains no source-level value
```

A mutable binding may be unavailable.

An immutable binding may also be unavailable after move.

---

# Reinitialization after move or discard

A mutable binding may be reused after its previous value has been moved or
discarded.

Example after move:

```sec
let mut source := CreateBuffer()
let destination :<- source

source = CreateBuffer()
Use(source)
```

Example after discard:

```sec
let mut resource := OpenResource()
discard resource

resource = OpenResource()
Use(resource)
```

The assignment is reinitialization.

It does not destroy an old value because no old value remains available.

The compiler must update the binding state to `Available`.

An immutable binding cannot be reinitialized:

```sec
let source := CreateBuffer()
let destination :<- source

source = CreateBuffer()
```

Expected error:

```text
cannot reinitialize immutable binding source after move
```

The source spelling remains in scope even when its value is unavailable.

---

# Move does not create a default value

Moving from a binding does not assign a source-level default value.

Example:

```sec
let mut text := "hello"
let moved :<- text
```

Afterward, `text` is unavailable.

It is not automatically:

```sec
""
```

The compiler may physically clear storage for safety, optimization, debugging,
or target requirements.

That physical state is not observable as a valid Sec value.

The programmer must explicitly reinitialize a mutable binding:

```sec
text = ""
```

This rule applies equally to:

- mutable sources;
- immutable sources;
- strings;
- scalars;
- structs;
- handles;
- collections.

---

# Copy and move classification

Every resolved type has a copy classification.

Initial categories are:

```text
TriviallyCopyable
SemanticallyCopyable
ConditionallyCopyable
MoveOnly
ExplicitlyNonCopyable
```

The exact internal names may differ.

---

## Trivially copyable

A trivially copyable value can be duplicated without type-defined behavior.

Typical examples:

```text
bool
byte
char
rune
integers
unsigned integers
ordinary floats
simple decimals
fieldless enums
RawPtr[T]
shared references
arrays of trivially copyable elements
structs whose fields and type semantics permit trivial copy
```

Representation equality alone does not make a type semantically copyable.

---

## Semantically copyable

A semantically copyable value may use compiler-known copy semantics beyond a
raw bitwise copy.

Implicit semantic copy must be:

- infallible;
- bounded;
- free from hidden heap allocation;
- free from hidden external resource acquisition;
- free from externally observable side effects;
- unsurprising for the type.

A copy that may allocate, fail, block, perform I/O, or duplicate an external
resource must use an explicit named operation.

---

## Conditionally copyable

A generic or aggregate type is copyable only when its relevant contained types
are copyable.

Examples:

```text
Option[T]
Result[T, E]
array T[N]
struct Box[T]
closure with captures
```

The compiler resolves the classification for each concrete instantiation.

---

## Move-only

A move-only type cannot be duplicated through ordinary copy syntax.

Typical examples include:

```text
file ownership
socket ownership
device handle ownership
memory mapping ownership
unique allocation ownership
MutexGuard[T]
ref mut T
unresolved Task[T]
unresolved Thread[T]
types with move-only fields
types with a custom free operation unless explicit copy semantics exist
```

Move-only classification belongs to the type, not to an individual variable.

---

## Explicitly non-copyable nominal types

A nominal type may need to forbid copy even when its representation would
otherwise be copyable.

The canonical source declaration is `@noCopy` on a nominal type as defined by
`attributes.md`. The frontend parses and enforces this path. Compiler-known
core types may carry the same internal classification directly.

Explicit non-copyability is a nominal policy. It forbids implicit copy but does
not by itself forbid source-level ownership transfer.

---

## Separate ownership and storage properties

Copyability, movability, relocatability, address stability, and pinning are
separate compiler properties.

- non-copyable does not imply non-movable;
- movable does not imply freely relocatable;
- source-level move does not require physical relocation;
- explicitly non-copyable may arise from nominal policy;
- move-only may instead be derived from resource ownership or lifecycle;
- pinning and address stability constrain storage even when ownership can be
  transferred.

A valid explicit move may reuse storage, transfer ownership metadata, be
elided, or construct directly in destination storage. It does not establish a
general permission to relocate bytes.

---

# No move-only variable modifier

Sec does not make an individual variable move-only independently of its type.

Invalid conceptual syntax:

```sec
let moveOnly value: string := "hello"
```

A binding is:

```text
mutable or immutable
available or unavailable
```

The type determines whether ordinary copy is legal.

The programmer may still explicitly move a copyable value.

---

# Copy and move syntax

Sec distinguishes ordinary initialization/assignment from explicit ownership
transfer.

The canonical forms are:

```sec
let destination := source
let destination :<- source

let destination: Type := source
let destination: Type <- source

destination = source
destination <- source
```

---

## Tokens

The ownership tokens are:

```text
:<-     move initialization
<-      move assignment or move replacement
```

They are dedicated ownership-transfer tokens.

They do not imply that `->` exists as an operator.

`->` has no meaning in Sec 0.1 unless another rulebook explicitly assigns one.

---

## Ordinary inferred declaration

```sec
let destination := expression
```

performs ordinary initialization.

Its exact behavior depends on the right-hand expression category.

From an existing reusable place:

```sec
let destination := source
```

the operation requires `source` to be implicitly copyable.

It copies and preserves `source`.

From a fresh temporary:

```sec
let destination := CreateBuffer()
```

the temporary initializes `destination` directly.

No explicit move token is required.

There is no reusable source binding to preserve.

---

## Explicit move declaration

```sec
let destination :<- source
```

moves from the existing source place.

Afterward:

```text
destination
    Available

source
    Moved
```

This syntax is valid even when the source type is copyable.

It explicitly requests ownership transfer instead of copy.

Example with `string`:

```sec
let original := "hello"
let moved :<- original
```

`original` becomes unavailable.

---

## Typed ordinary initialization

```sec
let destination: Type := expression
```

uses the same ordinary initialization rules.

A named reusable source must be copyable.

A fresh temporary may directly initialize the destination.

---

## Typed move initialization

```sec
let destination: Type <- source
```

moves from the existing source place into a typed declaration.

The canonical syntax does not use:

```sec
let destination: Type :<- source
```

because the colon already introduces the explicit type.

Canonical forms:

```sec
let inferred :<- source
let explicit: Type <- source
```

---

## Ordinary assignment

```sec
destination = expression
```

performs replacement when `destination` is available.

Conceptually:

1. evaluate the complete right-hand expression;
2. validate copy or direct temporary initialization;
3. validate aliases and borrows;
4. destroy the old destination value;
5. install the new value;
6. transfer destruction responsibility to the destination.

From an existing reusable source place, `=` requires copyability.

From a fresh temporary, direct replacement is allowed:

```sec
current = CreateBuffer()
```

No explicit move token is required.

---

## Move assignment

```sec
destination <- source
```

moves from the source place and replaces or initializes the destination.

When the destination is available:

1. evaluate and validate the source;
2. validate that source and destination do not conflict;
3. validate borrows;
4. destroy the previous destination value;
5. move the source value into the destination;
6. mark the source `Moved`;
7. mark the destination `Available`.

When the mutable destination is unavailable:

1. validate the source;
2. move the source value into the destination;
3. mark the source `Moved`;
4. mark the destination `Available`.

No old destination destruction occurs in the unavailable case.

---

# Copy syntax never silently moves a named source

When the right-hand side is an existing reusable source place:

```sec
let destination := source
```

and the source type is move-only, this is a hard error.

Likewise:

```sec
destination = source
```

is a hard error when copying the named source is impossible.

The compiler must not silently reinterpret these forms as move.

Suggested diagnostic:

```text
Buffer cannot be copied from source
```

Suggested help:

```text
use `let destination :<- source` to transfer ownership
```

or:

```text
use `destination <- source` to transfer ownership
```

This rule keeps source ownership visible.

---

# Automatic correction of an obvious move error

When all of the following are proven:

- the ordinary source form is invalid only because the named source is
  move-only;
- the destination accepts the source type;
- moving is legal;
- no borrow conflict exists;
- no overload or conversion meaning changes;
- the source and destination do not alias illegally;

the compiler may offer an automatic fix:

```text
:=  ->  :<-
=   ->  <-
```

The correction is available through:

```text
LSP code action
sec fmt --fix
another explicit compiler fix command
```

Ordinary `sec fmt` must not apply this semantic correction.

The fix must never be applied when the type is copyable.

If copy is legal, `:=` or `=` means copy and must remain unchanged.

---

# Fresh temporaries

A fresh temporary may initialize an owned destination without explicit move
syntax.

Examples:

```sec
let buffer := CreateBuffer()
let file: File := OpenFile()
current = CreateBuffer()
```

This is direct construction or temporary transfer.

It is not an implicit move from an existing user-visible place.

The compiler may use:

- copy elision;
- return-value optimization;
- destination passing;
- MLIR destination-style operations;
- register forwarding;
- bufferization.

These optimizations do not change ownership semantics.

---

# Chained temporary expressions

An expression derived entirely from a temporary may transfer into its final
destination without explicit move syntax.

Conceptual example:

```sec
let value := CreateContainer().TakeValue()
```

The compiler must still validate that no intermediate borrowed view escapes and
that each intermediate value is destroyed exactly once.

If an intermediate expression names reusable source storage, ordinary copy and
explicit move rules apply to that source.

---

# Function parameters

## By-value parameters

A by-value parameter owns its received value for the duration defined by the
function.

Its local binding is mutable by default. Reassignment replaces the callee-owned
working value under the ordinary destruction and definite-assignment rules.
For a copyable argument this cannot mutate the caller's retained copy.

For a copyable argument:

```sec
Process(value)
```

the caller's value is copied.

The caller retains its source value.

For a move-only argument:

```sec
Consume(resource)
```

ownership transfers to the parameter.

The caller's `resource` becomes unavailable after successful argument
evaluation.

No call-site `<-` expression syntax is required in Sec 0.1.

The function signature and resolved type determine whether the by-value
parameter consumes a move-only value.

An explicit `-> name: T` parameter forces consumption even when `T` would
otherwise be copied. It consumes the value itself, not a pointee merely
represented by that value, and cannot be combined with `ref` or `ref mut`.
Overloads may not differ only by ordinary versus forced-consuming mode.

The LSP must make this ownership effect visible.

---

## Borrowed parameters

A reference parameter does not take ownership of the referent.

```sec
Inspect(ref value)
Modify(ref mut value)
```

The caller remains owner.

Borrowing restrictions apply for the duration of the borrow.

The reference parameter binding itself is not implicitly rebindable. `ref mut`
grants exclusive mutable access to the referent, not mutable binding syntax.

---

## Argument evaluation

Arguments and spread sources are evaluated strictly left-to-right. Ownership
transfer into the outer callee is committed only after every argument has
evaluated successfully and the call is ready to enter the callee.

If a later argument fails, earlier caller bindings have not been consumed by
that outer call and caller-owned temporaries are cleaned normally. Effects and
ownership transfers performed inside an earlier argument expression are not
rolled back.

If one argument moves a value needed by another argument, the compiler must
reject the call or enforce the defined evaluation order without use after move.

---

## Function-call diagnostics

A call consuming a move-only argument is valid.

The LSP should display:

```text
resource is moved into parameter resource
```

A later source use is a hard error.

A function wanting a non-consuming view must declare a reference parameter.

---

# Return values

## Return is an ownership-transfer context

```sec
return value
```

transfers the function result to the caller.

An owned local returned as the result is not copied merely to preserve a local
whose scope is ending.

Example:

```sec
fn CreateBuffer() Buffer {
    let buffer := Buffer.Create()
    return buffer
}
```

`buffer` transfers into the return result.

The callee must not destroy it afterward.

The caller becomes the owner.

---

## No return move arrow

Sec 0.1 does not use:

```sec
return <- value
```

The syntax is unnecessary.

`return` already identifies a consuming result context.

The lexer and parser should not introduce `return <-` as a separate ownership
form.

---

## Returning copyable locals

Returning a copyable local still establishes ownership of the returned result.

The compiler should transfer or directly materialize the local into the return
result rather than preserve an unnecessary local copy.

For trivial machine values, copy and move may lower identically.

Semantic IR should use one consistent return-transfer operation.

---

## Returning from non-owned storage

Return does not permit moving from storage the function does not own.

Example:

```sec
fn Name(person: ref Person) string {
    return person.name
}
```

If `string` is copyable, the field value is copied into the returned result.

If the field type is move-only, returning it by value from a shared reference is
invalid.

A mutable reference also does not automatically grant ownership transfer from
the referent.

A dedicated consuming operation is required.

---

## Returning references

Returning a reference does not transfer ownership of the referent.

Lifetime analysis must prove that the referenced storage survives the return.

Returning a reference to local storage is invalid.

---

# Result, Option, unions, and aggregate construction

Constructing an owning payload is a consuming context for move-only input.

Examples:

```sec
return Ok(resource)

let option := Some(resource)

let response := Response {
    Data: resource,
}
```

A copyable payload is copied.

A move-only payload transfers ownership.

The source becomes unavailable.

The compiler must track ownership separately for each constructed payload.

Fallible or branching construction must ensure partially constructed payloads
are destroyed exactly once.

---

# Struct construction

A struct literal constructs a new aggregate.

Example:

```sec
let session := Session {
    name: name,
    file: file,
}
```

For each field:

- copyable source values are copied;
- move-only source values are transferred;
- borrowed fields follow reference rules;
- fresh temporaries construct directly.

The compiler must track partial construction when later field evaluation fails
or exits.

The struct becomes owner of all successfully installed owning fields.

---

# Field reads

Reading a copyable field copies it:

```sec
let name := person.name
```

Both remain available.

Reading a move-only field through ordinary initialization is invalid:

```sec
let file := session.file
```

when `File` is move-only.

Suggested help:

```text
use `let file :<- session.file` to transfer ownership
```

---

# Partial move from struct fields

Sec may move an eligible directly named struct field:

```sec
let file :<- session.file
```

Afterward:

```text
file
    Available

session.file
    Moved

other session fields
    individually available

session as a complete value
    PartiallyAvailable
```

Methods requiring complete `self` cannot be called while a required field is
unavailable.

Whole-value copy, move, borrow, return, and destruction must account for the
partial state.

---

## Initial partial-move requirements

A direct struct-field move is permitted only when all applicable conditions
hold:

- the source aggregate is owned;
- the field is directly named;
- the field type is movable;
- no active borrow conflicts;
- no alias can observe an invalid complete aggregate;
- the containing type has no opaque ownership invariant;
- the containing type has no custom `free` requiring complete state;
- the field is not volatile;
- the field is not a register field;
- the field is not accessed through `RawPtr`;
- the layout is not foreign-opaque;
- Sema can track the field independently.

The initial implementation may support only local struct bindings.

---

## Field reinitialization

A moved field in a mutable struct may be reinitialized:

```sec
session.file = OpenFile()
```

The fresh temporary initializes the unavailable field.

No old field value is destroyed.

After every missing required field is reinitialized, the complete struct becomes
`Available` again.

From an existing move-only source:

```sec
session.file <- replacement
```

explicit move syntax is required.

An immutable struct cannot have a moved field reinitialized.

---

# Custom destruction and partial move

A type with custom destruction is move-only by default.

Partial move from such a type is initially forbidden unless the type's ownership
contract explicitly supports field-wise destruction.

Reason:

> A custom destructor may require invariants spanning multiple fields.

The compiler must not call custom destruction on a semantically incomplete value
unless the type rule explicitly defines that behavior.

---

# Arrays and fixed-size collections

## Ordinary element read

Reading an element copies it when the element type is copyable:

```sec
let item := values[23]
```

The array remains complete.

---

## Move-only element read

Ordinary indexing must not silently move a move-only element.

Invalid:

```sec
let item := resources[23]
```

Suggested help should recommend:

- borrow;
- replacement;
- a collection-specific consuming operation;
- another supported API.

---

## Move-out from fixed arrays

Sec 0.1 does not permit:

```sec
let item :<- resources[23]
```

through ordinary fixed-array indexing.

A fixed array has fixed length, but moving an element out would also create a
partially initialized array.

Compile-time and runtime indexes require different tracking strategies.

This is deferred until the language defines:

- empty fixed slots;
- partial fixed-array initialization;
- compile-time index tracking;
- runtime initialization bitmap policy;
- destruction of partially initialized fixed arrays;
- complete-value restrictions.

---

## Replacing a fixed-array element

A mutable fixed array may replace an element:

```sec
resources[23] = CreateResource()
```

when the right-hand side is a temporary.

The old element is destroyed after the replacement source has been safely
evaluated.

A named move-only source requires a dedicated move replacement form once indexed
move targets are supported:

```sec
resources[23] <- replacement
```

Support for indexed move targets must be specified by `collections.md`.

An API such as:

```sec
values.Replace(index, replacement)
```

may return the previous element without leaving the array incomplete.

Its exact API belongs to core.

---

# Dynamic collections

Dynamic collections may support consuming extraction because their logical
membership and length may change.

The collection rule defines:

- whether indexed extraction exists;
- whether an item is removed;
- whether order is preserved;
- whether the operation is fallible;
- which bounds or missing-key result is returned;
- how iteration freeze applies.

---

## list

For `list[T]`, consuming indexed extraction is a natural operation.

Canonical ownership syntax may be:

```sec
let item :<- values[index]
```

Semantically it:

- validates the index;
- moves the selected element into `item`;
- removes the element from the list;
- decreases list length;
- preserves or updates order according to list semantics;
- performs structural mutation;
- invalidates relevant iterators and views;
- preserves ownership of remaining elements.

This syntax is valid only for mutable list storage.

It must not be treated as a copy followed by removal.

Core may implement it directly, through Sec code, Semantic IR, MLIR, or
target-specific lowering.

The collection rulebook must be synchronized before implementation.

---

## map

Map lookup does not yet use `<-` syntax for consuming extraction.

Use an explicit core operation such as:

```sec
let removed := values.Remove(key)
```

or:

```sec
let removed := values.Take(key)
```

The final API must define missing-key behavior and return type.

Possible results include `Option[V]` or another explicit type.

The ownership rule does not choose the final map API.

---

## set

Set removal or consuming extraction uses an explicit operation until its exact
API is locked.

Conceptual form:

```sec
let stored := values.Take(probe)
```

This may return the actual stored value while removing it.

`set` does not gain arbitrary integer indexing merely to support move.

---

## Other dynamic collections

Queue, deque, stack, ring buffer, heap, and other stdlib types expose
type-specific consuming operations.

Examples may include:

```text
Pop
Take
Remove
Dequeue
Extract
```

Their return types and failure behavior are defined by stdlib.

---

# Slices and views

A slice or view is normally non-owning.

Copying a shared slice copies the descriptor and borrow relationship.

It does not copy elements.

A mutable slice or mutable view is move-only or reborrowed according to
borrowing rules.

Moving a view transfers the view's borrow obligation.

It does not transfer ownership of the underlying elements.

Discarding a view ends the view binding, not the referent lifetime.

---

# string

`string` is an immutable, implicitly copyable source-language value.

Example:

```sec
let first := "hello"
let second := first
```

Both remain available.

The copy must be:

- infallible;
- free from hidden allocation;
- free from mutable aliasing;
- free from observable side effects.

The compiler and core may implement string copy as:

- descriptor copy;
- reference to immutable static storage;
- shared immutable backing with compiler-proven lifetime;
- arena-backed immutable storage;
- another representation preserving source semantics.

No garbage collector or hidden reference-counting requirement follows from this
rule.

A string may be explicitly moved:

```sec
let moved :<- first
```

Afterward `first` is unavailable.

Move and copy may use identical machine instructions while retaining different
source semantics.

---

# Resource-owning types

A type owning an external or unique resource is move-only unless it defines a
safe independent copy operation.

Examples:

```text
File
Socket
DeviceHandle
MemoryMap
UniqueAllocation[T]
MutexGuard[T]
```

A raw handle field being integer-sized does not make the wrapper copyable.

A custom `free` operation makes the type move-only by default.

Duplicating an operating-system or hardware resource requires an explicit named
operation.

If duplication can fail, it must return `Result`.

It must not be hidden inside `:=` or `=`.

---

# Hardware handles and MMIO

Ownership is especially important for hardware-facing values.

Moving a hardware handle transfers responsibility for:

- finalization;
- release;
- disable;
- interrupt unregistration;
- DMA completion;
- buffer ownership;
- exclusive device access.

It does not imply copying or relocating the device itself.

An addressed register declaration such as an `@address(...)` binding represents
volatile external storage.

It is not an ordinary owned value that may be moved away from its hardware
address.

Reading or writing such storage follows register and volatile rules.

A wrapper owning permission to use a device may be move-only even when the
underlying register block remains fixed.

---

# References

References do not own their referents.

```text
ref T
    shared non-owning access

ref mut T
    exclusive non-owning access

RawPtr[T]
    raw non-owning address with no ownership inference
```

A shared reference is normally copyable.

A mutable reference is move-only unless safely reborrowed.

Moving a reference transfers the reference binding and its borrow obligation.

It does not transfer the referent's ownership.

---

# Raw pointers

`RawPtr[T]` never implies ownership.

Copying a raw pointer copies only the address value.

Moving a raw pointer may make the source binding unavailable when explicit move
syntax is used, but it still does not transfer ownership of the pointed-to
storage.

Ownership may cross FFI through a raw pointer only when an explicit FFI contract
defines:

- allocation origin;
- owner before the call;
- owner after success;
- owner after failure;
- required release operation;
- nullability;
- lifetime;
- thread restrictions.

---

# Borrow conflicts

A value cannot be moved or discarded while an incompatible borrow remains live.

Invalid:

```sec
let view := ref value
let moved :<- value
```

The diagnostic must point to:

- the attempted move;
- the active borrow;
- the borrow holder when known.

A mutable borrow is itself exclusive and normally move-only.

Reinitialization must not invalidate active references to the old value.

Borrow analysis determines when last use ends a borrow.

---

# Destruction

Every available owned value is destroyed exactly once unless ownership transfers
elsewhere.

Move transfers destruction responsibility.

The source is not destroyed after move.

Replacement assignment destroys the old destination value after safe evaluation
of the new value.

Reinitialization of an unavailable destination destroys no old value.

A partially available aggregate destroys only its still-initialized fields,
unless a type-specific rule forbids partial state.

Destruction order is defined by `destruction.txt`.

---

# Discard

```sec
discard value
```

consumes the current value, performs deterministic destruction, and marks the
source unavailable.

A mutable discarded binding may later be reinitialized.

This rule supersedes the current implementation behavior that rejects assignment
to a discarded binding.

Implicit discard of an ordinary call result follows `discard.md`.

Must-use and non-discardable obligations remain enforced.

---

# Must-use and lifecycle ownership

Ownership and must-use are related but distinct.

```text
ownership
    identifies responsibility for a value

must-use
    forbids implicit loss of a semantic obligation

discardability
    determines whether explicit discard is permitted
```

Unresolved lifecycle handles such as:

```text
Task[T]
Thread[T]
Process
```

must be:

- awaited;
- joined;
- detached;
- returned;
- transferred;
- otherwise resolved by their type rule.

They cannot be silently destroyed.

A successful spawn result containing such a handle is not discardable as a
whole.

---

# Channels and transfer

Sending a move-only value through an owning channel transfers ownership to the
channel operation or receiver according to channel semantics.

After successful transfer, the sender's source becomes unavailable.

A revocable or fallible send must define ownership on every outcome:

```text
success
    receiver or channel owns the value

failure
    sender retains or regains ownership

cancellation
    rule must define who owns the value

revocation
    rule must define whether ownership returns
```

The compiler must not lose ownership on an error path.

Copyable values may be copied when channel semantics request copy.

---

# Tasks and threads

Values captured or passed into task and thread entry contexts follow
transferability and ownership rules.

A move-only capture transfers ownership into the execution context.

A copyable capture may copy.

Detached execution requires every captured owned value to remain valid for the
execution lifetime.

The owner of a task or thread handle remains responsible for its lifecycle
obligation.

---

# Closures

Capture modes are explicit:

```text
value          ordinary owned copy/move capture
-> value       forced-consuming owned capture
ref value      shared borrowed capture
ref mut value  exclusive mutable borrowed capture
```

Owned capture bindings are mutable inside the closure. `->` forces transfer
even for a copyable value. No `capture(mut value)` form exists.

A closure is copyable only when:

- every owned capture is copyable;
- no mutable reference would be duplicated;
- the closure representation permits copy;
- capture semantics permit independent use.

A closure capturing a move-only value becomes move-only.

A closure capturing `ref mut` becomes move-only.

A consuming `-> fn` closure is move-only. Invocation consumes it when moving
environment state out leaves the callable unusable.

Moving the closure transfers its captures.

The source closure becomes unavailable.

---

# Generics

Generic code must preserve copy and move distinctions for every instantiation.

A generic operation using ordinary source copy:

```sec
let second := first
```

requires the resolved `T` to be copyable.

If `T` may be move-only, the generic must:

- express a copy constraint;
- use explicit move syntax where applicable;
- restructure into a consuming context;
- use a named duplication operation;
- borrow instead.

The compiler must not generate one instantiation that copies and another that
silently moves for the same ordinary source syntax.

That would violate source-level predictability.

---

# Named types

A named type does not automatically inherit implicit copy merely because its
base representation is copyable.

Its copy classification depends on:

- declared semantics;
- contracts;
- units;
- custom destruction;
- owned fields;
- external resource ownership;
- explicit non-copyability;
- core-defined behavior.

A simple named numeric type is normally copyable.

A named resource wrapper is normally move-only.

---

# Interfaces

An interface value must distinguish:

```text
borrowed interface reference
owned interface value
descriptor referring to externally owned storage
move-only erased owner
```

Copying an owning interface must not duplicate ownership accidentally.

The initial rule should make an owning erased interface value move-only unless
independent copy semantics are known.

Borrowed interface references follow normal reference rules.

Interface receiver capability is part of the interface contract:

```sec
fn Inspect() Data
mut fn Update() void
-> fn Consume() ResultData
```

A shared borrow may call only shared methods. A mutable borrow may call shared
and mutable methods but does not transfer ownership and therefore cannot call a
consuming method. A consuming method requires an owned interface value and
consumes that receiver on successful invocation.

---

# Static values

A static binding owns its value for the program or explicitly defined static
lifetime.

A static owned value is not moved into a shorter-lived local by ordinary copy
syntax when it is move-only.

Access may instead require:

- borrow;
- explicit static handoff;
- a synchronization guard;
- a type-specific operation.

Static initialization and destruction order belong to `initialization.md` and
`static.md`.

---

# Arena ownership

An arena owns its allocated storage.

A value placed in an arena may own logical subresources while the arena owns the
storage.

References into the arena cannot outlive its generation.

Moving an arena-backed value transfers the value's semantic ownership only when
the arena and lifetime rules permit it.

It does not transfer the arena itself unless the arena handle is moved.

Reset invalidates values and references according to arena rules.

---

# Shared ownership

Sec 0.1 has no implicit shared ownership.

The language does not automatically introduce:

- reference counting;
- garbage collection;
- shared owner tables;
- weak references;
- ownership cycles.

Multiple shared references may point to one owner.

Those references remain non-owning.

A future explicit nominal type such as:

```sec
Shared[T]
Weak[T]
```

would require a separate rulebook and explicit allocation, destruction, cycle,
thread-safety, and failure semantics.

---

# Self-move and aliasing

Explicit move must reject invalid self-transfer:

```sec
value <- value
```

For a copyable type:

```sec
value = value
```

may be accepted and optimized to no operation.

For a move-only type, ordinary `=` is already invalid because copy is not
available.

The compiler must also detect overlapping move source and destination places
when they would cause destruction before a required read.

Example conceptual hazard:

```sec
object.field <- object
```

Alias and place analysis determine legality.

---

# Control flow

Ownership state is path-sensitive.

Example:

```sec
if condition {
    let moved :<- value
}
```

After the `if`, `value` is not definitely available because one continuing path
moved it.

Use is invalid unless all continuing paths establish an available value.

---

## Branch merge

At control-flow merge, the compiler must combine states conservatively.

Conceptual examples:

```text
Available + Available
    Available

Moved + Moved
    Moved

Discarded + Discarded
    Discarded

Available + Moved
    ConditionallyAvailable

Available + Discarded
    ConditionallyAvailable

PartiallyAvailable + Available
    conditionally complete state requiring field analysis
```

A branch that cannot continue does not affect availability after the merge.

---

## Loops

Loop ownership analysis requires a fixed point.

A value moved during one iteration cannot be assumed available in the next
iteration unless every continuing path reinitializes it.

Example:

```sec
while condition {
    Consume(resource)
}
```

is invalid for move-only `resource` unless `resource` is reinitialized before
the loop continues.

---

## switch, match, and select

Every continuing arm contributes ownership state.

Values moved in multiple mutually exclusive arms may be valid when each arm owns
the transfer independently.

A value moved by one arm and retained by another becomes conditionally
available after the construct.

For a whole-payload plain match binding, an implicitly copyable payload is
copied, a move-only payload is moved only from an owned reusable Place that may
legally transfer it, and a fresh temporary payload may be forwarded directly.
The compiler must not silently clone or turn a by-value binding into a borrow.

A subject available only through `ref` or `ref mut` cannot transfer ownership
of a move-only payload. Such access requires copyability or an explicit pattern
`ref` / `ref mut` borrow as permitted by the subject authority.

For a guarded move-only by-value binding, pattern success creates only a
prospective binding. The move commits after guard success when the arm is
selected. Consuming the prospective binding inside the guard is invalid. A
guard-false edge retains the subject's ownership state except for actual guard
side effects, and candidate arm borrows end before the next arm is tested.

Plain shallow named-field match bindings are copy-only. A move-only field must
be borrowed or ownership must be taken through a legal whole-payload binding;
shallow destructuring never creates a hidden partial union move.

Every continuing match arm contributes its availability state to the
post-match merge. Available plus moved becomes conditionally available, and a
later whole-value use is rejected unless every continuing path establishes
availability. Terminating arms do not contribute a post-match state. Match
ownership facts retain the subject Place, affected payload, arm and binding,
guard-success commit point, and merge provenance for diagnostics and tooling.

Select operations must also account for ownership of messages on:

- selected arm;
- unselected arms;
- cancellation;
- default;
- failed send or receive.

---

# Evaluation order and failure

The compiler must preserve ownership across failures and early exits.

For replacement:

```sec
destination <- CreateOrGetSource()
```

the old destination remains valid until the right-hand evaluation has completed
successfully enough to commit replacement.

For fallible operations:

```text
before success
    old destination remains owned

after success
    old destination is destroyed
    new destination owns the value

on failure
    ownership follows the operation's Result contract
```

The compiler must not destroy a destination before a source expression that
depends on it finishes evaluation.

---

# Exceptions and panic

Sec does not use hidden exception ownership semantics.

Future panic rules must define cleanup on unwind or abort profiles.

Ownership analysis must still determine:

- initialized values at the panic point;
- which destructors run;
- which values were already moved;
- which partial aggregates require cleanup;
- which lifecycle obligations cannot be abandoned.

No value may be destroyed twice during panic cleanup.

---

# FFI

Ownership crossing FFI must be explicit.

An FFI declaration must eventually express enough information to determine:

- whether an argument is borrowed;
- whether ownership transfers on call;
- whether ownership transfers only on success;
- whether a return value is owned;
- who releases returned storage;
- whether the foreign function retains a pointer;
- thread and lifetime restrictions.

In the absence of an explicit ownership contract:

- references may not escape;
- `RawPtr[T]` remains non-owning;
- move-only ownership must not be assumed transferred;
- returned foreign pointers do not automatically become owned values.

Unsafe does not disable ownership rules.

---

# Semantic analysis

Sema must determine for every relevant value use:

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

Sema must also determine:

- source place;
- destination place;
- copy classification;
- source availability;
- destination availability;
- mutability;
- active borrows;
- partial aggregate state;
- lifecycle obligations;
- destruction responsibility;
- control-flow successor state.

---

# Semantic IR

Semantic IR must make ownership explicit.

At minimum, the IR should support operations equivalent to:

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

Each operation records, where relevant:

```text
source
destination
type
ownership classification
source state before
source state after
destination state before
destination state after
destruction action
borrow relation
source location
explicit or inferred origin
```

Move origin should distinguish:

```text
ExplicitMoveInitialization
ExplicitMoveAssignment
InferredReturnTransfer
InferredArgumentTransfer
InferredAggregateTransfer
InferredTemporaryTransfer
LifecycleTransfer
```

The backend must not infer ownership from unused SSA values, loads, stores,
pointers, or MLIR types.

---

# MLIR and lowering

MLIR lowering receives validated ownership operations.

It may optimize:

- copy elision;
- return-value optimization;
- destination passing;
- buffer reuse;
- in-place update;
- tensor bufferization;
- register forwarding;
- dead temporary destruction;
- trivial move elimination.

It must preserve:

- source unavailability;
- destruction count;
- borrow validity;
- external resource responsibility;
- failure-path ownership;
- volatile and hardware semantics.

A move may lower to no operation.

It remains semantically real.

---

# Diagnostics

Ownership safety diagnostics are hard errors.

They cannot be disabled or demoted.

Stable IDs must identify the rule rather than severity.

---

## Copy of move-only source

Symbolic rule:

```text
ownership.copy-of-move-only
```

Example:

```sec
let destination := source
```

Suggested diagnostic:

```text
error[S....]: Buffer cannot be copied from source
```

Suggested help:

```text
use `let destination :<- source` to transfer ownership
```

The diagnostic should provide a machine-applicable fix when safe.

---

## Move assignment required

Example:

```sec
destination = source
```

Suggested diagnostic:

```text
error[S....]: File cannot be copied into destination
```

Suggested help:

```text
use `destination <- source` to transfer ownership
```

---

## Use after move

Symbolic rule:

```text
ownership.use-after-move
```

Example:

```text
error[S....]: value source is unavailable because it was moved
```

The diagnostic must include the source location where the move occurred.

---

## Use after discard

Canonical rule remains defined by `discard.md`.

The diagnostic includes the discard location.

---

## Reinitializing immutable binding

Symbolic rule:

```text
ownership.reinitialize-immutable
```

Example:

```text
error[S....]: cannot reinitialize immutable binding source after move
```

---

## Move while borrowed

Symbolic rule:

```text
ownership.move-while-borrowed
```

The diagnostic identifies:

- move location;
- borrow origin;
- holder;
- expected borrow end where available.

---

## Partial move unsupported

Symbolic rule:

```text
ownership.partial-move-unsupported
```

The diagnostic explains which condition prevents the field move.

---

## Fixed-array move-out

Symbolic rule:

```text
ownership.fixed-index-move-out
```

Suggested message:

```text
cannot move Resource out through fixed-array indexing in Sec 0.1
```

Suggested help:

```text
borrow the element or use a replacement operation that keeps the array initialized
```

---

## Non-discardable lifecycle

Defined jointly with `discard.md`, task, thread, and process rules.

---

# Formatter requirements

The canonical formatter rulebook remains separate.

Ownership requires these canonical spellings:

```sec
let destination :<- source
let destination: Type <- source
destination <- source
```

There is one space before and after `:<-` and `<-`.

The formatter must preserve:

```text
:= versus :<-
= versus <-
```

Ordinary formatting must not change copy semantics into move semantics.

In particular, ordinary formatting must never change:

```sec
let value := Create()
```

into:

```sec
let value :<- Create()
```

The first form is already canonical direct initialization from a temporary.

The formatter must also not change:

```sec
let destination := source
```

into move syntax merely because Sema reports that `source` is move-only.

That change belongs to `--fix` or an LSP code action.

---

# `sec fmt --fix`

`sec fmt --fix` may apply obvious, local, machine-proven corrections.

For ownership, it may change:

```text
:= to :<-
= to <-
```

only when ordinary copy is impossible and move is the one unambiguous valid
correction.

The fix must not run when:

- the source is copyable;
- multiple overloads would change;
- a borrow conflict exists;
- conversion behavior changes;
- source and destination may alias illegally;
- moving would create another error;
- the source is not a reusable place;
- the result is already a temporary direct initialization.

---

# Non-ownership formatter normalization

The formatter rulebook must separately define these accepted-input
normalizations:

```text
func  -> fn
x++   -> x += 1
x--   -> x -= 1
```

These do not require `--fix` when parsing is unambiguous.

They are canonical syntax normalization rather than ownership inference.

The resulting compound assignment remains subject to normal type, mutability,
contract, and `try` rules.

Example:

```sec
count++
```

formats as:

```sec
count += 1
```

For a constrained type requiring fallible assignment, noncanonical increment
input does not bypass the required `try` semantics.

---

# LSP requirements

A dedicated:

```text
lsp.md
```

rulebook is required.

It will be written after this ownership rule is locked.

The ownership model reserves at least these LSP responsibilities:

- semantic highlighting of moved and unavailable bindings;
- hover display of copy classification;
- inlay hints for consuming by-value parameters;
- inlay hints for inferred return transfer;
- code action `:=` to `:<-`;
- code action `=` to `<-`;
- navigation to move origin;
- navigation to active borrow origin;
- explanation of partial aggregate state;
- suggestion to borrow instead of transfer;
- warning before a rename or edit creates use after move;
- generic-instantiation copyability display;
- collection extraction guidance;
- fixed-array move-out explanation;
- ownership data for debugger and code lens features.

The LSP must not silently edit source without an explicit user action.

---

# Required tests

## Declaration tests

Valid:

```sec
let first := "hello"
let second := first

let moved :<- first
```

Invalid:

```sec
let first := OpenFile()
let second := first
```

Expected fix:

```sec
let second :<- first
```

---

## Typed declaration tests

Valid:

```sec
let first := OpenFile()
let second: File <- first
```

Invalid copy:

```sec
let second: File := first
```

---

## Assignment tests

Valid:

```sec
let mut destination := CreateBuffer()
let source := CreateBuffer()

destination <- source
```

Valid temporary replacement:

```sec
destination = CreateBuffer()
```

Invalid named copy:

```sec
destination = source
```

when `Buffer` is move-only.

---

## Reinitialization tests

Valid:

```sec
let mut source := CreateBuffer()
let destination :<- source

source = CreateBuffer()
```

Valid after discard:

```sec
let mut source := CreateBuffer()
discard source

source = CreateBuffer()
```

Invalid immutable reinitialization:

```sec
let source := CreateBuffer()
let destination :<- source

source = CreateBuffer()
```

---

## Use-after-move tests

Invalid:

```sec
let source := CreateBuffer()
let destination :<- source

Use(source)
```

The diagnostic must point to both move and use.

---

## Return tests

Valid:

```sec
fn Create() Buffer {
    let value := CreateBuffer()
    return value
}
```

No `return <-` is required or accepted.

---

## Parameter tests

Valid move-only consumption:

```sec
fn Consume(value: Buffer) void {
}

let value := CreateBuffer()
Consume(value)
```

Later use is invalid.

Valid copyable argument:

```sec
let text := "hello"
Print(text)
Print(text)
```

---

## Struct tests

Valid copyable field read:

```sec
let name := person.name
```

Valid field move when supported:

```sec
let file :<- session.file
```

Invalid whole-value use while partial:

```sec
Use(session)
```

Valid mutable field reinitialization:

```sec
session.file = OpenFile()
Use(session)
```

---

## Fixed-array tests

Valid copyable read:

```sec
let item := values[2]
```

Invalid move-only ordinary read:

```sec
let item := resources[2]
```

Invalid explicit move-out in Sec 0.1:

```sec
let item :<- resources[2]
```

---

## list tests

When list extraction is implemented:

```sec
let item :<- values[index]
```

must:

- return the selected owned value;
- reduce length by one;
- mutate structurally;
- reject immutable list storage;
- reject conflicting iteration or borrows;
- report bounds failure through the collection rule.

---

## Formatter tests

Input:

```sec
let destination:<-source
```

Output:

```sec
let destination :<- source
```

Input:

```sec
destination<-source
```

Output:

```sec
destination <- source
```

Ordinary formatter must not change:

```sec
let destination := source
```

to move syntax.

`--fix` may change it only after a machine-proven move-only copy error.

Input:

```sec
value++
value--
```

Output:

```sec
value += 1
value -= 1
```

Input:

```sec
func Run() void {
}
```

Output:

```sec
fn Run() void {
}
```

---

# Required synchronization

This rulebook must remain synchronized with:

```text
copy_move.md
contracts.md
types.md
functions.md
struct.md
collections.md
shaped-types.md
borrowing.txt
references.txt
lifetime_analysis.txt
destruction.txt
discard.md
transferability.md
channels.md
tasks.txt
threads.md
processes.txt
platform/ffi.md
declarations/registers.md
platform/fixed-address-bindings.md
static.md
allocation.txt
memory_model.md
semantic_ir.txt
compiler_pipeline.txt
diagnostics.txt
lexical_structure.md
operators.md
grammar.md
formatter.md
lsp.md
rules_implementations.txt
language-rulebook-status.md
```

The canonical ownership, copy/move and formatter documents use their `.md`
filenames; legacy text filenames are not canonical.

---

# Appendix A — Codex synchronization plan

## Purpose

This appendix is an implementation and documentation checklist intended for
Codex.

It is not a separate language proposal.

The normative decisions are in the main rulebook.

Codex must update dependent files without redesigning the decisions below.

---

## A.1 Canonical decisions to preserve

Codex must preserve these decisions exactly:

```text
1. `:=` and `=` never silently move from an existing reusable named place.

2. `:<-` is move initialization for an inferred declaration.

3. `<-` is move initialization after an explicit type and move assignment for
   existing destinations.

4. Fresh temporaries may initialize or replace without explicit move syntax.

5. `return expression` is already a consuming return context.

6. `return <- expression` is not introduced.

7. By-value parameters copy copyable values and consume move-only values.

8. Struct, Result, Option, union, and owning collection construction may infer
   transfer for move-only inputs.

9. Explicit move is allowed for copyable values and makes the source
   unavailable.

10. A moved mutable binding may be reinitialized.

11. A moved immutable binding may not be reinitialized.

12. Move never assigns a source-level default value.

13. Use after move is a mandatory error.

14. `string` is immutable and implicitly copyable.

15. Sec 0.1 has no implicit shared ownership.

16. Fixed-array indexed move-out is not supported in Sec 0.1.

17. Dynamic `list` may support indexed consuming extraction.

18. Ordinary formatting does not change copy syntax into move syntax.

19. `sec fmt --fix` may apply a proven `:=` to `:<-` or `=` to `<-` fix.

20. `func` to `fn` and `x++`/`x--` canonicalization do not require `--fix`.
```

---

## A.2 File migration

The ownership, copy/move and formatter filename migrations are complete.
Repository references use the canonical `.md` names and no duplicate canonical
files remain.

---

## A.3 Lexer

Add tokens for:

```text
<-
:<-
++
--
```

Requirements:

- longest match chooses `:<-` before `:` and `<-`;
- longest match chooses `<-` before `<` and `-`;
- `++` and `--` are accepted as noncanonical source input;
- `->` remains unassigned;
- source locations cover the complete token;
- lexer tests cover adjacency, whitespace, malformed forms, and range/operator
  interactions.

Update:

```text
lexical_structure.md
lexer implementation
lexer token definitions
lexer tests
VS Code grammar
future LSP lexical classification
```

---

## A.4 Parser

Add declaration forms:

```sec
let value :<- source
let value: Type <- source
```

Add assignment form:

```sec
target <- source
```

Parser must distinguish:

```text
:
:=
:<-
=
<-
```

Add noncanonical postfix forms:

```sec
value++
value--
```

Normalize them in AST or a dedicated syntax-normalization phase to:

```sec
value += 1
value -= 1
```

Do not implement them as expressions returning a value.

They are statement-only aliases.

Parser recovery may recognize:

```sec
func Name() void {
}
```

as an unambiguous noncanonical function declaration so the formatter can emit
`fn`.

Do not add `return <-`.

---

## A.5 AST

Extend `LetStatement` with an ownership initialization mode.

Conceptual field:

```text
InitializationMode
    Ordinary
    Move
```

Do not infer move merely from the token string in later phases.

Extend or reuse `AssignmentStatement.Operator` for:

```text
<-
```

Ensure normalized increment/decrement becomes a compound assignment AST or
equivalent canonical node.

Preserve original source ranges for diagnostics and formatter fixes.

---

## A.6 Sema copy and move selection

Replace current blanket `markMoveSource` behavior for declarations and
assignments.

Required logic for ordinary declaration/assignment:

```text
if RHS is a reusable source place:
    if source type is implicitly copyable:
        Copy
    else:
        emit ownership.copy-of-move-only
        attach safe move fix when possible

if RHS is a fresh temporary:
    direct construction or temporary transfer
```

Required logic for explicit move:

```text
require movable source place
require available source
require no conflicting borrow
require legal destination
mark source Moved
mark destination Available
```

Do not mark a source moved merely because its type is move-only when the source
syntax requested ordinary copy.

---

## A.7 Availability state

Replace or extend identifier-only maps with a structured ownership-state model.

It must support:

```text
Uninitialized
Available
PartiallyAvailable
Moved
Discarded
Detached
ConditionallyAvailable
```

Record:

```text
reason
source token
source range
field path where applicable
control-flow origin
```

Use stable queries such as:

```text
RequireAvailable
CanCopy
CanMove
CanReinitialize
MergeOwnershipState
```

Do not rely on one boolean moved map for final field-sensitive analysis.

---

## A.8 Reinitialization

Update mutable assignment handling:

```text
Moved mutable binding + valid assignment
    reinitialization

Discarded mutable binding + valid assignment
    reinitialization

Uninitialized mutable binding + valid assignment
    initialization

Available mutable binding + valid assignment
    replacement
```

Remove the current rule that rejects all assignment to a discarded mutable
binding.

Do not destroy an old value during initialization or reinitialization.

Reject reinitialization of immutable bindings.

---

## A.9 Function calls

Preserve current by-value move-only argument transfer.

Ensure:

- copyable argument remains available;
- move-only by-value argument becomes unavailable;
- ref parameter does not transfer referent ownership;
- `ref mut` follows exclusive-borrow rules;
- argument evaluation order is respected;
- ownership on fallible call paths is defined;
- LSP metadata records consuming parameters.

Do not introduce call-site `<-` syntax in this task.

---

## A.10 Returns

Preserve and formalize:

```text
return local owned value
    transfer into return result

return fresh temporary
    direct return construction

return copyable field through non-owning reference
    copy

return move-only field through non-owning reference
    error
```

Do not add `return <-`.

Semantic IR must record return ownership transfer explicitly.

---

## A.11 Structs and partial moves

Implement in stages.

Stage 1:

- ordinary copyable field reads;
- hard error for ordinary read of move-only field;
- diagnostic suggesting `:<-`;
- no partial move yet if place tracking is not ready.

Stage 2:

- direct local struct field move;
- field-specific unavailable state;
- complete aggregate becomes partial;
- unaffected fields remain available;
- complete-self method calls rejected;
- mutable field reinitialization;
- complete state restoration;
- destruction of only initialized fields.

Do not permit partial move for:

```text
custom free
register
volatile
RawPtr access
foreign-opaque layout
borrowed aggregate
unresolved union variant
```

---

## A.12 Arrays and slices

Keep current Sec 0.1 rule:

```text
copyable indexed read
    copy

move-only indexed ordinary read
    error

explicit indexed move-out
    error
```

Do not add hidden initialization bitmaps.

Document future fixed-slot empty-state work separately.

Update diagnostics to mention borrowing or replacement.

---

## A.13 Collections

Update `collections.md` and `shaped-types.md`:

- ordinary collection reads copy when the element is copyable;
- `list` may support `let value :<- list[index]` as structural consuming
  extraction;
- fixed arrays do not use this behavior;
- map and set retain explicit `Remove` or `Take` APIs until their return
  semantics are locked;
- collection extraction must preserve iteration-freeze and borrow rules.

Implement list extraction only after parser, Sema place analysis, and core API
semantics agree.

Owning dynamic arrays `T[]` are `MoveOnly` for every element type. The owner is
the sole authority that destroys initialized elements and reclaims its backing
allocation. No descriptor copy may duplicate that authority. `RemoveAt`
performs explicit consuming extraction; ordinary indexing never silently moves
an element. Slices carry no reclamation authority.

---

## A.14 string

Update type classification:

```text
string
    immutable
    implicitly copyable
    explicitly movable
```

Ensure implicit string copy:

- cannot allocate;
- cannot fail;
- cannot create mutable aliasing;
- preserves backing lifetime.

Update:

```text
types.md
core-library.md
sec/core/string.sec
copy_move.md
memory_model.md
Semantic IR
MLIR lowering
tests
```

---

## A.15 Discard and destruction

Keep `discard.md` semantics.

Change current discarded-binding assignment behavior:

```text
mutable discarded binding
    may be reinitialized

immutable discarded binding
    may not be reinitialized
```

Ensure explicit move transfers destruction responsibility.

Ensure source destruction is suppressed after move.

Ensure replacement and reinitialization are distinct in Semantic IR.

---

## A.16 Diagnostics

Register stable diagnostic IDs for:

```text
ownership.copy-of-move-only
ownership.use-after-move
ownership.reinitialize-immutable
ownership.move-while-borrowed
ownership.partial-move-unsupported
ownership.fixed-index-move-out
ownership.invalid-self-move
```

Diagnostics must include:

- concrete type;
- source name or place;
- move origin;
- related source location;
- machine-applicable fix when safe;
- explanatory note when hardware or lifecycle ownership is involved.

Safety errors are mandatory.

---

## A.17 Semantic IR

Add or finalize:

```text
ConstructValue
CopyValue
MoveValue
ReplaceValue
ReinitializeValue
DiscardValue
DestroyValue
ReturnValue
TransferArgument
TransferField
TransferCollectionElement
```

Record explicit versus inferred transfer origin.

Semantic IR creation must fail internally if ownership selection remains
unresolved.

MLIR must not choose copy versus move.

---

## A.18 Formatter migration and update

Rename:

```text
formatter.md
    -> formatter.md
```

Add canonical ownership formatting:

```sec
let value :<- source
let value: Type <- source
target <- source
```

Ordinary formatter:

- preserves ordinary versus move syntax;
- does not require Sema;
- never rewrites `let x := Function()` to `let x :<- Function()`;
- normalizes unambiguous `func` to `fn`;
- normalizes `x++` to `x += 1`;
- normalizes `x--` to `x -= 1`.

`sec fmt --fix`:

- may use Sema;
- may apply ownership fixes only when unambiguous;
- reports which fixes were applied;
- preserves diagnostics when a fix is unsafe;
- supports dry-run/check output if formatter CLI rules provide it.

Add formatter golden tests.

---

## A.19 LSP reservation

Move `lsp.md` in `language-rulebook-status.md` from:

```text
Candidate
```

to:

```text
Planned
```

Do not write the complete LSP rulebook until ownership implementation contracts
are stable.

Reserve LSP data structures and protocol-independent semantic facts for:

```text
copy classification
move origin
availability state
consuming parameter
safe code action
borrow conflict
partial move
reinitialization
```

---

## A.20 Rulebook synchronization

Update all relevant references and contradictions.

Highest-priority files:

```text
copy_move.md
contracts.md
types.md
functions.md
struct.md
collections.md
shaped-types.md
borrowing.txt
references.txt
lifetime_analysis.txt
destruction.txt
discard.md
memory_model.md
transferability.md
semantic_ir.txt
compiler_pipeline.txt
diagnostics.txt
lexical_structure.md
formatter.md
rules_implementations.txt
language-rulebook-status.md
```

`copy_move.md` requires a substantial rewrite because its current ordinary
initialization and assignment rules allow implicit move from named move-only
sources.

---

## A.21 Status document

The synchronized status is:

```text
ownership.md
    Living

copy_move.md
    Written

formatter.md
    Written — implementation partial

lsp.md
    Living
```

All four documents belong to the canonical written set.

---

## A.22 Tests

Create or update:

```text
ownership_valid.sec
ownership_invalid.sec
ownership_control_flow_valid.sec
ownership_control_flow_invalid.sec
ownership_struct_valid.sec
ownership_struct_invalid.sec
ownership_collections_valid.sec
ownership_collections_invalid.sec
```

Add Go unit tests for:

```text
copy classification
move selection
place classification
state merge
reinitialization
safe fix generation
field state
list extraction state
```

Every invalid Sec case must contain:

```sec
/* Expected error: ...
 * Reason: ...
 */
```

Verify:

- no use after move reaches Semantic IR;
- no moved value is destroyed;
- no unavailable destination old value is destroyed during reinitialization;
- every available owned value is destroyed exactly once;
- diagnostic source and related locations are stable;
- formatter output is idempotent;
- `--fix` does not alter already valid copy semantics.

---

## A.23 Implementation order

Recommended implementation order:

```text
1. Rename and synchronize rulebook references.
2. Add lexer tokens.
3. Add parser and AST ownership mode.
4. Implement ordinary-copy versus explicit-move selection.
5. Implement mutable reinitialization after move/discard.
6. Add hard diagnostics and safe fixes.
7. Add Semantic IR ownership operations.
8. Update formatter.
9. Add LSP semantic metadata hooks.
10. Implement struct field partial moves.
11. Implement list consuming extraction.
12. Complete MLIR and backend verification.
```

Do not start list extraction before explicit move and place-state analysis are
stable.

---

# Design summary

Sec has one-owner semantics without runtime ownership tracking.

Ordinary initialization and assignment copy from an existing reusable source
place.

They never silently move that source.

Explicit move uses:

```sec
let destination :<- source
let destination: Type <- source
destination <- source
```

Fresh temporaries initialize directly without move syntax.

Return is already a consuming ownership-transfer context and does not use
`return <-`.

By-value parameters and aggregate construction infer transfer for move-only
values.

Moving any value makes the source unavailable.

A mutable source may later be explicitly reinitialized.

An immutable source cannot.

Move does not assign an empty or default value.

Use after move is always a hard compiler error.

`string` is immutable and copyable, but may be explicitly moved.

Fixed-array indexed move-out is deferred.

Dynamic `list` may support indexed consuming extraction.

Sec 0.1 introduces no implicit shared ownership.

The formatter preserves ownership semantics, while `sec fmt --fix` and the
future LSP may apply explicit, machine-proven ownership corrections.
