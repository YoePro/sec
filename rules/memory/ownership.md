# Ownership

- **Status:** Normative
- **Created:** 2026-08-26
- **Last updated:** 2026-08-28
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/memory/ownership.md`
- **Replaces:** pre-v2 content at `rules/memory/ownership.md` and the legacy ownership rulebook
- **Repository baseline reviewed:** `b3315f6` (latest semantic source/rulebook parent: `45e5cd4`)

---

## 1. Purpose

Ownership defines which Sec place is responsible for an owned value, when that
responsibility transfers, when a source becomes unavailable, and which value must
be destroyed when ownership ends.

The ownership model has these primary goals:

- every owned value has one current owner unless another rulebook explicitly
  defines shared ownership;
- ownership transfer is deterministic;
- destructive transfer from a reusable source is visible in source code;
- use after move, discard, detach, or destruction is rejected;
- a value is destroyed at most once;
- partial ownership of aggregates is tracked by place;
- mutability and availability remain separate concepts;
- borrowing does not silently become ownership;
- raw pointers do not imply ownership;
- source semantics do not depend on backend representation;
- the compiler explains ownership errors in ordinary programmer language.

Ownership analysis is mandatory semantic analysis. It is not an optional lint,
optimization, or deep-analysis feature.

Detailed copy classification belongs to `rules/memory/copy_move.md`.
Detailed borrowing belongs to `rules/memory/borrowing.md`.
Detailed destruction belongs to `rules/memory/destruction.md`.

For generic instances, `rules/compiler/generics_lowering.md` requires concrete
ownership, copy/move, partial-move, and destruction facts after substitution
and before ownership-sensitive executable lowering. A backend must not infer a
weaker ownership contract from physical representation or incomplete lowering
coverage.
The abstract value/place/storage model is anchored by
`rules/memory/memory_model.md`.

---

## 2. Core terminology

### 2.1 Value

A value is a typed semantic entity.

Examples include:

```text
integer value
string value
struct value
array value
list value
tensor value
file handle wrapper
thread handle
reference value
```

A value may or may not own storage or an external resource.

### 2.2 Place

A place is a source-level location that may contain a value.

Examples:

```sec
value
packet.Header
packet.Payload
array[index]
object.Property
```

Places may be projected from other places. A projected place may refer to a
field, a statically proven element, a union payload, or another place form whose
owning rulebook permits independent ownership tracking.

Not every expression is a reusable place.

### 2.3 Reusable source place

A reusable source place is a place whose source spelling could otherwise be used
again after the current expression or statement.

Examples:

```sec
resource
packet.Payload
items[0]
```

when the relevant place rules establish stable reusable ownership identity.

The distinction matters because Sec requires destructive transfer from a
reusable source place to be visible.

### 2.4 Binding

A binding associates a name with a place or value.

```sec
let value := CreateValue()
```

The binding name may remain lexically in scope after its current value has
become unavailable.

### 2.5 Owner

The owner is the semantic location responsible for a value's lifetime.

An owner may be:

```text
local binding
parameter binding
struct field
aggregate element where independent ownership is permitted
union payload
closure environment field
return result
task or thread handle holder
another rulebook-defined owned location
```

Ownership is not the same thing as physical address.

A semantic move may transfer ownership without moving bytes.

### 2.6 Temporary

A temporary is a value produced during expression evaluation without a reusable
user-visible source place.

Examples:

```sec
CreateBuffer()
left + right
Packet {
    Header: header,
    Payload: CreateBuffer(),
}
```

A fresh temporary does not require an explicit move marker merely because it is
forwarded into its first owner.

### 2.7 Storage

Storage is the physical or abstract backend location used to represent a value.

Examples include:

```text
stack slot
register
static storage
arena allocation
heap allocation
SSA value
MLIR tensor value
memref-backed buffer
device handle field
```

Ownership is defined before backend storage selection.

---

## 3. Fundamental ownership guarantees

For every tracked owned value, the compiler must determine at every relevant
program point:

- where ownership began;
- the current owner;
- whether the relevant place is available;
- whether an operation copies or consumes;
- which active borrows restrict an operation;
- which owner is responsible for destruction;
- whether the value is partially available;
- whether availability differs across continuing control-flow paths;
- where ownership ends or transfers;
- which source operation caused unavailability.

No valid Sec program may:

- read an unavailable value;
- copy an unavailable value;
- borrow an unavailable value;
- move an unavailable value again;
- destroy a value twice;
- silently copy a non-copyable value;
- silently consume a reusable named source through ordinary value syntax;
- move through an incompatible active borrow;
- infer ownership from `RawPtr[T]`;
- silently create shared ownership;
- use a whole aggregate while a required owned sub-place is unavailable.

---

## 4. Independent semantic dimensions

Ownership availability is not the same concept as mutability, borrow access, or
reference-generation validity.

The compiler must keep these dimensions separate.

```text
mutability
    may this place be written or reinitialized?

availability
    does this owner still own a value in this place on this path?

borrow/access authority
    may this place be accessed while current borrows are active?

generation validity
    does this reference still designate the correct live invalidation domain?
```

A place may be immutable and available.
A place may be immutable and unavailable after a move.
A mutable place may be unavailable and later reinitialized.
An available place may still be temporarily inaccessible because of an active
incompatible borrow.
A valid generation does not prove ownership availability of every sub-place.

---

## 5. Availability model

Revision 2 separates availability from the reason that a place became
unavailable.

### 5.1 Availability states

A tracked place has one of these source-semantic availability states:

```text
Uninitialized
Available
PartiallyAvailable
Unavailable
ConditionallyAvailable
```

The compiler may use a richer internal lattice, but diagnostics and rulebook
reasoning must preserve these meanings.

### 5.2 Uninitialized

The place exists but no value has been constructed in it.

```sec
let mut resource: Resource
```

An uninitialized place cannot be read, copied, moved, borrowed, or destroyed.

### 5.3 Available

The place contains a valid owned value.

Normal operations are permitted subject to:

- mutability;
- copy/move classification;
- borrowing;
- contracts;
- type-specific rules.

### 5.4 PartiallyAvailable

An aggregate is `PartiallyAvailable` when at least one independently tracked
owned sub-place is unavailable while another remains available.

Example:

```sec
let package := LoadPackage()
let payload :<- package.Payload
```

After the move:

```text
package.Payload    Unavailable
package.Header     Available
package            PartiallyAvailable
```

### 5.5 Unavailable

A place is `Unavailable` when the current owner no longer owns a source-level
value there.

An unavailable place cannot be:

- read;
- copied;
- borrowed;
- moved again;
- used as an available sub-place;
- destroyed as if a value were still present.

A mutable unavailable place may later be reinitialized.

### 5.6 ConditionallyAvailable

A place is `ConditionallyAvailable` when continuing control-flow paths disagree
about whether the current owner still owns a value there.

Example:

```sec
let package := LoadPackage()

if condition {
    Consume(<-package.Payload)
}
```

After the `if`, when both paths continue:

```text
package.Payload    ConditionallyAvailable
package            conditionally partial
```

A conditionally available place cannot be used as an ordinary available place
unless control-flow refinement proves it available on the current path.

---

## 6. Unavailability reason

`Unavailable` has a separate reason used for semantic provenance and mentor
diagnostics.

Initial reasons include:

```text
Moved
Discarded
Detached
```

The implementation may retain multiple possible reasons after a control-flow
join.

Example:

```text
Availability: Unavailable
Reason: Moved
Source: Consume(<-buffer)
```

or:

```text
Availability: Unavailable
Reason: Discarded
Source: discard buffer
```

The reason does not create a distinct top-level availability state.

For continued use, both are unavailable.

This separation avoids multiplying states such as
`PartiallyMovedAndPartiallyDiscarded`.

---

## 7. Mutability and availability

Mutability controls writing and reinitialization.
Availability controls whether a currently owned value exists.

They are orthogonal.

```text
immutable binding
    ordinary assignment/reinitialization is forbidden

mutable binding
    ordinary replacement/reinitialization may be permitted

available place
    currently owns a value

unavailable place
    currently owns no value
```

A move is not ordinary assignment.

Therefore an immutable binding may be consumed:

```sec
let resource := OpenResource()
Consume(<-resource)
```

and may be partially consumed when partial moves are otherwise legal:

```sec
let package := LoadPackage()
let payload :<- package.Payload
Use(package.Header)
```

The immutable `package` cannot later repair the moved field by assignment.

```sec
package.Payload = CreateBuffer()
```

is invalid because the root binding is immutable.

---

## 8. Field mutability and receiver authority

Stored data members are not independently frozen merely because the language
does not put `mut` on each field declaration.

The authority to mutate a member is derived from the access path and receiver
context.

For ordinary external access through an immutable root:

```sec
let package := LoadPackage()
package.Header.Version = 2
```

is invalid.

For a method executing with mutable/exclusive receiver authority, owned data
members may be mutated, replaced, reinitialized, discarded, or moved when all
other ownership and borrow rules permit the operation.

A method may therefore consume a member:

```sec
impl Package {
    fn ReleasePayload() void {
        Destroy(<-self.Payload)
    }
}
```

The compiler derives the required mutable/exclusive receiver authority from the method body and validates the call accordingly.

A method does not need per-field `mut` declarations to modify its owned state.

---

## 9. Whole-self consumption is forbidden for ordinary methods

An ordinary instance method must not consume the complete `self` value.

The end of the entire instance lifetime is owned by lifecycle destruction,
including `free` where a custom destructor exists.

Therefore ordinary methods do not have a source-language whole-self consuming
receiver mode.

A method may consume or discard owned members of `self`, producing partial
availability when permitted.

This rule does not remove consuming callable semantics for closure/function
values. It applies to ordinary instance-method receivers.

A semantic builtin may define a compiler-known terminal lifecycle operation
when a canonical rulebook defines the operation, the compiler owns its
ownership transition, the operation consumes the source owner without
producing a continuing whole-`self` value, and automatic destruction can
recognize that the owner has already been consumed. This is not a general
source-language facility for user-defined consuming methods.

For Sec 0.1, `Arena.Release()` is such an operation. After successful Release,
the Arena owner Place is unavailable and later automatic destruction must not
release the same ArenaDomain again. Exact Arena lifecycle semantics remain
owned by `rules/memory/arena.md`.

`free` is the lifecycle operation whose purpose is to finalize the complete
instance.

---

## 10. Explicit move syntax

### 10.1 Principle

A reusable source place is never silently consumed by ordinary value syntax.

If ownership leaves a reusable source place, the consuming source must be
explicitly marked with `<-`, except at the function return boundary described in
Section 17.

This is a readability rule as well as an ownership rule.

The caller should be able to see where a reusable value disappears.

### 10.2 Move initialization

```sec
let destination :<- source
```

consumes `source` and initializes `destination` with the transferred value.

### 10.3 Move assignment

```sec
destination <- source
```

moves the source value into an existing mutable destination according to the
replacement/reinitialization rules.

### 10.4 Move expression in a consuming context

When a consuming context accepts an expression argument or payload, `<-` marks
the reusable source being consumed:

```sec
Consume(<-resource)
Some(<-resource)
```

```sec
let packet := Packet {
    Header: header,
    Payload: <-resource,
}
```

### 10.5 Explicit move of a copyable value

`<-` may intentionally consume a value whose type would otherwise be copyable.

```sec
let value := 42
ConsumeNumber(<-value)
```

After a successful committed move, the source is unavailable even though its
type is copyable.

---

## 11. Ordinary copy syntax must not become an implicit move

Ordinary copy-like source syntax means copy when copying is legal.

For a non-copyable reusable source, it must be rejected rather than silently
changed into a move.

Invalid for `@noCopy` or otherwise move-only `resource`:

```sec
let option := Some(resource)
```

Valid:

```sec
let option := Some(<-resource)
```

Likewise invalid:

```sec
let second := first
```

when `first` is a reusable move-only source.

Use:

```sec
let second :<- first
```

The same principle applies across function arguments, aggregate fields, union
payloads, `Option`, `Result`, closure captures, and other ownership-taking
construction forms.

---

## 12. Fresh temporaries do not require move markers

A fresh temporary has no reusable source place that can later surprise the
programmer by becoming unavailable.

Therefore these forms do not require `<-`:

```sec
let buffer := CreateBuffer()
Consume(CreateBuffer())
let option := Some(CreateBuffer())
```

```sec
let packet := Packet {
    Payload: CreateBuffer(),
}
```

The temporary is forwarded into its destination according to ordinary
construction and call semantics.

The compiler may lower forwarding without physically materializing an
intermediate value.

---

## 13. Function parameters and call-site consumption

### 13.1 Ordinary by-value parameter

An ordinary by-value parameter is an owned callee-local value.

```sec
fn Process(value: T) void {
    ...
}
```

If the argument type is copyable, a plain reusable source may be copied:

```sec
Process(value)
```

If the argument type is non-copyable and a reusable named/place source must
transfer ownership, the call site must show the move:

```sec
Process(<-resource)
```

This replaces older rules that allowed a named move-only argument to disappear
through an ordinary call spelling.

### 13.2 Explicit consuming parameter

A parameter may force ownership consumption even for a copyable type:

```sec
fn Consume(-> value: T) void {
    ...
}
```

A reusable source passed to that parameter must always carry an explicit move
marker:

```sec
Consume(<-resource)
```

Invalid:

```sec
Consume(resource)
```

when `resource` is a reusable source place.

The compiler must diagnose the missing explicit ownership transfer and suggest
the `<-` form.

### 13.3 Temporary argument to a consuming parameter

A fresh temporary may be passed without redundant move syntax:

```sec
Consume(CreateResource())
```

No reusable caller-owned source is hidden by this transfer.

### 13.4 Parameter contract and type copyability are independent

`->` is an explicit API contract.

A copyable type passed to `->` is consumed.
A move-only type passed by ordinary by-value parameter still requires `<-` when
a reusable named source is transferred.

The compiler must not use overload resolution to decide whether the caller
loses ownership.

---

## 14. Call transfer commit

Arguments are evaluated left-to-right.

Ownership transfer from prepared reusable sources into the callee is committed
only when all arguments for that outer call have evaluated successfully and the
call is ready to enter the callee.

Conceptually:

```text
1. resolve callable
2. evaluate arguments left-to-right
3. validate borrows, conversions, and explicit move requirements
4. if argument evaluation fails, do not commit the outer call transfer
5. clean caller-owned temporaries created for the failed call
6. when all arguments are ready, commit the ownership transfers
7. enter the callee
```

Example:

```sec
let resource := OpenResource()

try Use(
    <-resource,
    LoadConfiguration(),
) {
    Err(error) => Handle(error)
}
```

If evaluating `LoadConfiguration()` fails before `Use` is entered, the outer
call has not committed ownership of `resource` to `Use`.

Effects and ownership transfers completed inside evaluation of an earlier
argument expression are not rolled back.

### 14.1 Pending call-transfer reservation

Preparing an explicit destructive transfer from a reusable Place in an outer
call argument creates a compiler-owned pending reservation for that exact
source Place. The caller still owns the value until call entry, but evaluation
of later sibling arguments may not read, copy, borrow, move, discard, replace,
reinitialize, consume, or mutate an overlapping Place.

The reservation follows the canonical overlap relation from Section 18. A
proven-disjoint sibling may remain usable:

```sec
Use(
    <-package.Payload,
    Inspect(package.Header),
)
```

Using the whole overlapping aggregate is invalid:

```sec
Use(
    <-package.Payload,
    Inspect(package),
)
```

Unsupported or ambiguous aliasing is conservative.

### 14.2 Commit and cancellation

If a later outer argument fails before call entry, only reservations belonging
to that outer call are canceled. The caller retains ownership of those reserved
sources, subject to effects that independently completed while evaluating
earlier argument expressions.

When every argument and call constraint succeeds, every prepared transfer is
committed exactly once: the caller Place becomes `Unavailable` and the callee
receives ownership.

### 14.3 Nested call transactions

Each call owns its own reservation transaction. A nested call that successfully
commits a transfer has completed an argument-evaluation effect; an outer-call
failure must not roll that nested transfer back. Compiler-known calls and
constructors with equivalent multi-argument ownership semantics follow the same
transaction rule.

---

## 15. Aggregate, union, Option, and Result construction

Construction owns its stored payloads according to the containing type.

A reusable source that must be consumed into a payload requires `<-`.

```sec
let optional := Some(<-resource)
```

```sec
let response := Ok(<-resource)
```

```sec
let message := Message {
    Payload: <-resource,
}
```

The same rule applies to a tagged-union payload constructor.

A copyable reusable source may be used without `<-` when ordinary construction
copies it.

A fresh temporary may be forwarded without `<-`.

Construction must never infer a destructive move solely because the payload type
is move-only.

---

## 16. Closure captures

Closure captures are explicit.

Revision 2 uses these ownership forms:

```sec
capture(value)
capture(<-value)
capture(ref value)
capture(ref mut value)
```

Their meanings are:

```text
capture(value)
    ordinary owned copy capture; the source must be copyable

capture(<-value)
    consuming owned capture; ownership leaves the outer source

capture(ref value)
    shared borrowed capture

capture(ref mut value)
    exclusive mutable borrowed capture
```

Plain capture does not silently move a move-only outer source.

Invalid for a non-copyable reusable source:

```sec
capture(resource) fn() void {
    Use(resource)
}
```

Valid:

```sec
capture(<-resource) fn() void {
    Use(resource)
}
```

`capture(<-value)` may also force consuming capture for a copyable value.

The old `capture(-> value)` spelling is not canonical revision-2 syntax.

---

## 17. Function return boundary

Returning an owned local transfers or forwards the returned value to the
caller's result ownership.

The return boundary already makes this transfer explicit enough that a move
marker is not required.

Canonical concise form:

```sec
return resource
```

An explicit marker is also permitted when the programmer wants to emphasize the
move:

```sec
return <-resource
```

Both forms have the same ownership result when returning the owned value.

The terminal return context propagates recursively through structural owning
construction that directly forms the returned value. Consequently, reusable
move-only sources need no repeated inner marker in forms such as:

```sec
return Some(resource)
return Packet.Data(resource)
return Response { Body: resource }
return Ok(Response { Body: resource })
```

This propagation also reaches every continuing value-producing arm of a
`match` or equivalent control-flow expression that directly supplies the
return. It does not rewrite ordinary call parameter modes: `return
Wrap(resource)` validates `resource` according to `Wrap`'s declared parameter.

Terminal forwarding remains subject to Place availability, borrow, lifetime,
storage-origin, partial-move, indexed-extraction, volatile/MMIO, and related
ownership restrictions. An explicit `<-` remains legal at any equivalent
terminal structural edge and changes no ownership result.

The marker is optional at the return boundary only; this does not weaken the
requirement for reusable source moves inside continuing caller control flow.

Receiving a fresh function result does not require a move marker:

```sec
let resource := CreateResource()
```

The call result is a fresh value owned by the receiving context.

---

## 18. Partial moves

### 18.1 Basic rule

A partial move transfers ownership from an independently tracked sub-place while
leaving disjoint owned sub-places with the original aggregate owner.

Example:

```sec
type Package struct {
    Header: Header
    Payload: Buffer
}

let package := LoadPackage()
let payload :<- package.Payload
```

After the move:

```text
package.Payload    Unavailable / Moved
package.Header     Available
package            PartiallyAvailable
```

### 18.2 Whole-value and sub-place operations

The normative rule is:

> Whole-value operations require the whole value to be `Available`.
> Sub-place operations require only the addressed sub-place to be `Available`.

Therefore:

```sec
Use(package.Header)
```

may remain valid, while:

```sec
Use(package)
```

is invalid until the aggregate has returned to complete availability.

### 18.3 Immutable aggregates

Partial moves may occur from an immutable binding when otherwise legal:

```sec
let package := LoadPackage()
let payload :<- package.Payload
```

Immutability prevents later assignment/reinitialization; it does not prevent an
explicit ownership transfer.

### 18.4 Mutable aggregates

A mutable aggregate may repair a moved field:

```sec
let mut package := LoadPackage()
let payload :<- package.Payload

package.Payload = CreateBuffer()
```

After successful field reinitialization, the field is `Available` again.
If every required field is available, the aggregate returns to `Available`.

### 18.5 Supported partial-move places

A partial move is legal only when Sema can establish a stable independently
owned sub-place and the relevant type/storage rule permits independent transfer.

The compiler must conservatively reject partial moves through unsupported or
ambiguous aliases, properties, volatile/fixed-address hardware state, or other
places whose ownership cannot be separated safely.

---

## 19. Custom `free` forbids partial moves in Sec 0.1

A type that defines custom `free` does not permit partial moves from its owned
fields in Sec 0.1.

Example:

```sec
type ResourcePair struct {
    First: Resource
    Second: Resource
}

impl ResourcePair {
    free {
        ...
    }
}
```

This is invalid:

```sec
let first :<- pair.First
```

The reason is semantic, not merely implementation convenience.
A custom `free` may rely on invariants over the complete valid `self` state.
The compiler must not silently replace that destructor with ad-hoc field-wise
cleanup after dismantling the value.

A future rulebook may define an explicit opt-in contract for partial destruction
of custom-destructor types. Sec 0.1 does not.

---

## 20. Control-flow availability merge

Ownership availability is path-sensitive.

At a control-flow join, the compiler merges only continuing paths.

Terminating paths such as `return`, propagation, panic/termination, or another
non-continuing edge do not poison availability after the join.

### 20.1 Same state on all paths

If all continuing paths prove the place `Available`, it remains `Available`.

If all continuing paths prove it `Unavailable`, it becomes `Unavailable`.
The compiler may retain a set of unavailability reasons for diagnostics.

### 20.2 Different availability on continuing paths

If some continuing paths prove `Available` and others prove `Unavailable`, the
place becomes `ConditionallyAvailable`.

Example:

```sec
let package := LoadPackage()

if condition {
    Consume(<-package.Payload)
}
```

### 20.3 Terminating moved path

```sec
let package := LoadPackage()

if condition {
    Consume(<-package.Payload)
    return
}

Use(package)
```

`Use(package)` may be valid because the path that consumed `Payload` does not
reach the continuation.

---

## 21. `is available` and `is not available`

Sec defines compiler-known ownership-state tests:

```sec
place is available
place is not available
```

These tests answer:

> Does the exact tested Place contain a complete currently owned value available
> for an ordinary operation on that Place on this runtime path?

For an aggregate Place, `is available` requires the complete recursive
availability mask to be `Available`. A partially available aggregate therefore
tests false as a whole even when one or more disjoint sub-places remain
available. Testing a projected sub-place observes only that exact sub-place.

They do not answer:

- whether a value is `None`;
- whether a raw pointer is `null`;
- whether a reference generation is valid;
- whether an active borrow currently permits access;
- whether the value itself encodes an empty state.

### 21.1 Example

```sec
if package.Payload is available {
    Use(package.Payload)
}
```

Within the true branch, ownership analysis refines the tested place to
`Available` unless another operation changes it.

```sec
if package.Payload is not available {
    RecoverWithoutPayload()
}
```

The negative test is the logical negation of whole availability. Its true branch
therefore means that the exact tested Place is not wholly `Available`; it does
not generally mean that every sub-place is `Unavailable`.

Canonical refinement is:

```text
true branch of `place is available`
    -> tested exact Place is Available

false branch of `place is available`
    -> tested exact Place is not wholly Available;
       preserve every still-possible sub-place/state alternative

true branch of `place is not available`
    -> same negative refinement

false branch of `place is not available`
    -> tested exact Place is Available
```

Only when prior facts restrict a leaf to exactly `Available|Unavailable` may a
true `is not available` branch refine directly to `Unavailable`. A partial or
conditional aggregate retains its surviving recursive mask, and an
`Uninitialized` Place remains distinguishable from an unavailable formerly
owned value. An availability test observes state and must not invent a moved,
discarded, detached, or other `UnavailableReason`.

### 21.2 Static resolution is mandatory

The compiler must resolve availability statically whenever control-flow facts are
sufficient.

A runtime ownership-state check must not be generated when Sema can already
prove the answer.

If a test is statically known and makes contained statements unreachable, the
normal Sec unreachable-code policy applies.

### 21.3 Dynamic ownership state

A genuinely conditional ownership fact may require runtime state whenever a
later ownership-dependent operation cannot be implemented correctly from
static control-flow facts alone. This includes availability tests, discard,
replacement/reinitialization, destruction/cleanup, ownership-sensitive
transfer/return, and other rulebook-defined ownership decisions. Source code
need not spell `is available` for runtime ownership state to be necessary.

The language does not mandate a fat pointer or a specific hidden flag layout.
Possible lowering includes:

```text
existing control-flow condition
SSA state
hidden local/drop flag
another equivalent backend representation
```

The representation is implementation-defined and must not change public type or
ABI layout merely because local ownership analysis needs state.

---

## 22. Static-ownership target policy

Sec must support a compilation policy that rejects code requiring dynamic
ownership bookkeeping.

This is important for bare-metal, realtime, and other targets where hidden state
or conditional cleanup cost must be avoided.

The source syntax for that project/target policy is defined elsewhere and is not
locked by this rulebook.

Under such a policy, code that leaves a place `ConditionallyAvailable` may still
be legal when all later ownership decisions are statically resolvable.

The compiler must reject only the point where correct semantics would require
forbidden dynamic ownership state.

Diagnostics must explain how to restructure the code, commonly by converging
ownership explicitly with `discard`.

---

## 23. `discard` as ownership convergence

`discard place` means:

> After this statement, the programmer intentionally owns no value in this
> place.

It therefore converges availability to `Unavailable`.

### 23.1 Available input

For an available discardable place:

```text
Available
    -> destroy current value when required
    -> Unavailable / Discarded
```

### 23.2 Already unavailable input

For an already unavailable place:

```text
Unavailable
    -> no destruction
    -> remains Unavailable
```

This is valid.

A statically redundant discard may receive an advisory under the existing
project diagnostic policy, but it is not a semantic error.

### 23.3 Conditionally available input

For a conditionally available place:

```text
ConditionallyAvailable
    -> destroy only on paths where the value is still owned
    -> do nothing on paths where it is already unavailable
    -> Unavailable on every outgoing path
```

Example:

```sec
let mut package := LoadPackage()

if condition {
    Consume(<-package.Payload)
}

discard package.Payload
package.Payload = CreateBuffer()
```

After the `discard`, the field is definitely unavailable.
The subsequent assignment is ordinary reinitialization.

### 23.4 Discardability still applies

Convergence does not bypass non-discardable lifecycle obligations.

`discard` remains legal only when the relevant current/possible owned value is
discardable under the canonical discard rules.

---

## 24. Reinitialization and replacement

### 24.1 Unavailable destination

Assignment to an unavailable mutable place is reinitialization.

```sec
let mut resource := OpenResource()
Consume(<-resource)

resource = OpenResource()
```

No old value remains to destroy.

After successful initialization, the place becomes `Available` and receives a
new destruction responsibility.

### 24.2 Available destination

Assignment to an available mutable place is replacement.

```sec
let mut destination := CreateBuffer()
destination = CreateBuffer()
```

The old destination value is ended/destructed according to ordinary destruction
rules before the new value becomes the destination's owned value.

Hosted Sec code does not require an explicit `discard` before ordinary
replacement.

### 24.3 Move assignment into an available destination

```sec
destination <- source
```

replaces the old destination value and transfers ownership from `source`.

The complete operation must be semantically validated before ownership transfer
is committed.

The compiler must not destroy the destination and only afterward discover that
the source move was illegal.

### 24.4 Conditionally available destination

A mutable `ConditionallyAvailable` place may be repaired by assignment.

```sec
package.Payload = CreateBuffer()
```

Semantically:

```text
if an old value is still owned on this path:
    end/destroy the old value
otherwise:
    no old destruction
initialize the new value
mark the place Available
```

Hosted implementations may perform the necessary conditional cleanup
automatically.

A target/project policy that forbids dynamic ownership bookkeeping may require
explicit convergence first:

```sec
discard package.Payload
package.Payload = CreateBuffer()
```

### 24.5 Immutable place

An immutable place cannot be reinitialized or replaced through ordinary
assignment.

Explicit ownership transfer may still make it unavailable.

---

## 25. Destruction follows current availability

Destruction responsibility follows the value still owned by each place.

An unavailable place is never destroyed again by its former owner.

A partially available aggregate destroys exactly its still-owned available
sub-places unless a valid whole-value custom destruction rule owns the complete
state.

Example:

```sec
let package := LoadPackage()
Consume(<-package.Payload)
```

At scope exit:

```text
package.Payload    not destroyed by package; ownership left earlier
package.Header     destroyed if its type requires destruction
```

Nested partial availability follows the same rule recursively where partial
moves are permitted.

This is a mandatory safeguard against double destruction.

---

## 26. Destruction classification

Every resolved Sec type has a compiler-derived destruction classification.

At minimum:

```text
TriviallyDestructible
NonTriviallyDestructible
```

A trivially destructible value requires no observable cleanup when its owned
lifetime ends.

A non-trivially destructible value may require cleanup directly or recursively
through owned sub-values.

This classification is independent of copy/move classification.

Examples that are typically trivially destructible include ordinary scalar
values and aggregates containing only trivially destructible fields.

Examples that may be non-trivially destructible include:

```text
owning collections
owned allocations
large owned shaped buffers when their representation owns storage
file/socket/device handles
memory mappings
foreign owned resources
types with custom free
aggregates containing non-trivially destructible owned fields
```

A language type such as `string` must not be classified solely from its source
name. Classification follows the actual canonical ownership representation
specified for that type.

The correctness classification determines cleanup requirements.
Separate performance analyses may decide whether a particular implicit cleanup
is noteworthy; they do not change ownership semantics.

---

## 27. Borrow interaction

Ownership transfer requires authority to remove the value from the source place.

A move or discard must be rejected when an incompatible live borrow overlaps the
source place.

Shared and mutable references do not become owners of their referents merely
because the reference value itself is copied, moved, returned, or discarded.

`ref mut T` may itself have move-only value semantics while still representing
borrowed authority rather than ownership of `T`.

Place overlap and borrow lifetime are governed by the borrowing rulebook.

---

## 28. `try`, `match`, `Result`, and ownership

Error handling and matching reuse the normal ownership model rather than
inventing parallel move semantics.

### 28.1 Match bindings

By-value match payload bindings copy copyable payloads and move move-only
payloads only when the canonical match rules permit a visible consuming binding
context.

`ref` and `ref mut` bindings borrow.

Path-sensitive ownership merges apply after continuing arms.

### 28.2 Guarded moves

A prospective move-only binding guarded by `where` must not commit ownership
merely because its pattern structurally matched.

Ownership commit occurs only after the guard succeeds and the arm is selected.

### 28.3 Try handlers

`try` is specialized control-flow sugar and uses the same payload ownership facts
as the corresponding match semantics.

Partial handlers add implicit propagation exits.
Only continuing success/recovery paths participate in post-try ownership merge.

### 28.4 Consuming Result projections

The error-handling rulebook defines:

```sec
result.Ok()
result.Err()
```

as consuming transformations of an owned `Result`.

After such a call the original owned `Result` is unavailable.

Borrowed projections such as `OkRef` and `ErrRef` remain non-consuming and obey
ordinary borrow lifetimes.

### 28.5 `return try`

`return try expression` forwards ownership through the return boundary according
to the error-handling rulebook.

The return boundary retains the optional-move-marker rule from Section 17.

---

## 29. Generational references are not availability flags

Generational references and place availability solve different problems.

Generation validity answers whether a reference still designates the correct
live storage/invalidation generation.

Ownership availability answers whether the current owner still owns a value in a
specific place.

A partial move may make `package.Payload` unavailable while `package` storage and
its generation remain live and valid.

Therefore generation checking must not replace ownership availability analysis.

---

## 30. Hardware, fixed-address storage, and FFI

Ownership is source-language semantics and does not depend on target
architecture.

However, not every storage-backed value is an ordinary movable owner.

Addressed registers, MMIO, volatile fixed-address storage, and foreign storage
must not be moved merely because they are representable as places.

Their dedicated rulebooks define whether ownership transfer is meaningful.

A compiler-known runtime hardware mapping defined by
`rules/platform/hardware-register-access.md` is an ordinary move-only resource
value. The mapping owner controls its mapping lifetime; typed register views
borrow that owner and must not outlive it. Moving the mapping transfers the
resource responsibility rather than copying hardware authority.

`RawPtr[T]` is a plain address value and does not imply ownership of pointed-to
storage, ownership of a hardware mapping, canonical endpoint identity, mapping
authority, privilege, or security-domain authority.

FFI ownership may cross the language boundary only through explicit foreign
contracts or wrappers that state retention and ownership behavior.

---

## 31. Runtime bookkeeping and ABI

Ownership semantics do not mandate a runtime ownership table.

When `ConditionallyAvailable` source semantics genuinely require runtime state,
the compiler may use a local implementation detail such as SSA state or a drop
flag.

Such bookkeeping:

- must not silently alter the public source type;
- must not be interpreted as `Option[T]`;
- must not be interpreted as nullable storage;
- must not make a reference generational merely for availability;
- must be statically eliminated when the answer is provable;
- may be forbidden by a target/project static-ownership policy.

A public ABI must not change merely because one local function uses an
availability test, unless the relevant ABI rulebook explicitly defines such a
representation.

---

## 32. Compiler implementation requirements

The compiler must maintain canonical ownership facts rather than reconstructing
ownership independently in each subsystem.

At minimum, semantic analysis must represent enough information to determine:

```text
Place identity
Availability
UnavailableReason/provenance
partial aggregate availability
conditional availability
copy versus consuming operation
explicit move source
active borrow overlap
reinitialization versus replacement
destruction responsibility
call-transfer commit
return forwarding
```

The frontend must not rely on backend instruction shape to infer a move.

Semantic IR must eventually preserve explicit ownership operations and cleanup
responsibility so lowerings do not guess from unused SSA values.

LSP and formatter behavior must consume compiler-owned semantic facts rather than
reimplementing the ownership state machine.

---

## 33. Mentor diagnostics

Ownership diagnostics are part of the language experience.

The compiler must describe the programmer's situation rather than requiring
knowledge of compiler theory.

A diagnostic should identify:

1. which value/place cannot be used;
2. what earlier operation changed its ownership state;
3. why the requested operation is unsafe or invalid;
4. what source-level correction is likely appropriate.

### 33.1 Missing move marker at a call

For:

```sec
Consume(resource)
```

when `Consume` takes ownership of the reusable source, prefer a message such as:

```text
`Consume` takes ownership of `resource`.

Passing this value will make `resource` unavailable, so the ownership transfer
must be written explicitly.

help: write `Consume(<-resource)`
```

### 33.2 Use after move

```text
`result` cannot be used here because ownership was transferred earlier.

The consuming operation was here:
    ...

help: use `result` before that operation, borrow it instead when appropriate,
      or reinitialize a mutable binding before using it again.
```

### 33.3 Conditional availability

```text
`package.Payload` may no longer be available here.

It was consumed on one possible execution path.

help: test `package.Payload is available` before using it, or restructure the
      control flow so availability is known statically.
```

### 33.4 Static-ownership target

```text
this ownership decision would require runtime state

`package.Payload` is present on some paths and already consumed on others.
This build forbids dynamic ownership bookkeeping.

help: converge ownership explicitly, for example with `discard package.Payload`,
      or restructure the control flow so availability is statically known.
```

Technical terms such as affine state, lattice join, or path-sensitive merge may
appear as secondary detail, but must not be required to understand the primary
diagnostic.

---

## 34. Formatter and LSP requirements

The formatter must preserve semantic ownership syntax, including:

```sec
:<-
<-
Consume(<-value)
capture(<-value)
return <-value
```

It must not insert or remove a semantic move marker merely as a style rewrite.

LSP semantic tokens, hover, diagnostics, and code actions should expose, where
useful:

- whether a place is available, unavailable, partial, or conditional;
- the reason and source location for unavailability;
- whether an argument is copied or consumed;
- when a call requires explicit `<-`;
- when a capture copies, borrows, or consumes;
- whether `is available` is statically known or runtime-dependent;
- whether a target policy forbids required runtime ownership state.

Automatic fixes must only be offered when compiler facts prove the ownership
change intended by the fix.

---

## 35. Required semantic tests

At minimum, revision-2 ownership conformance must cover:

1. ordinary copy of a copyable reusable source preserves the source;
2. ordinary copy syntax for a non-copyable reusable source is rejected;
3. `let destination :<- source` consumes the source;
4. `destination <- source` performs move assignment;
5. explicit `<-` may consume a copyable source;
6. consuming function parameter requires `Consume(<-resource)` for a reusable source;
7. plain `Consume(resource)` is rejected when the reusable source would be consumed;
8. ordinary by-value move-only parameter also requires `<-` for a reusable source;
9. fresh temporary argument to an ownership-taking call requires no move marker;
10. call transfer commits only after all outer-call arguments evaluate successfully;
11. `Some(resource)` is rejected when reusable move-only `resource` would be consumed;
12. `Some(<-resource)` transfers ownership;
13. aggregate and union payload construction require explicit `<-` for reusable consuming sources;
14. `capture(value)` copies only and rejects a non-copyable reusable source;
15. `capture(<-value)` consumes the capture source;
16. `capture(ref value)` and `capture(ref mut value)` remain borrow captures;
17. `capture(-> value)` is rejected as stale syntax;
18. `return resource` transfers ownership without requiring `<-`;
19. `return <-resource` is also accepted;
20. receiving a fresh move-only return value with ordinary initialization is accepted;
21. terminal return context forwards reusable move-only values recursively through aggregate, Option, Result, union, and value-producing match construction;
22. explicit `<-` remains accepted and ownership-equivalent on those terminal construction paths;
23. equivalent non-terminal construction still requires explicit `<-` and never hides destructive consumption;
24. nested terminal construction still rejects illegal borrowed, indexed, volatile, MMIO, or unavailable extraction;
25. ordinary methods cannot consume whole `self`;
26. methods may consume an owned member when receiver authority and partial-move rules permit it;
27. immutable root field assignment is rejected;
28. mutable receiver authority permits member mutation without per-field `mut` declarations;
29. partial move leaves sibling fields usable and rejects whole-value use;
30. partial move from immutable aggregate is accepted but reinitialization is rejected;
31. moved field of mutable aggregate may be reinitialized and restore whole availability;
32. partial move from a type with custom `free` is rejected;
33. branch with Available/Unavailable continuing paths produces `ConditionallyAvailable`;
34. terminating moved path does not poison post-branch availability;
35. `is available` refines true path to available;
36. binary `Available|Unavailable` leaf refines the true `is not available` path to unavailable;
37. statically known availability tests are folded without runtime ownership state;
38. `is available` does not behave as `None`, `null`, or a generation check;
39. strict static-ownership target rejects an ownership-dependent operation that genuinely requires forbidden runtime state;
40. discard of available place destroys when required and converges to unavailable;
41. discard of already unavailable place is legal no-op;
42. discard of conditionally available place converges all continuing paths to unavailable;
43. discard does not bypass non-discardable lifecycle obligations;
44. assignment to unavailable mutable place is reinitialization;
45. assignment to available mutable place automatically ends the old value before replacement;
46. hosted assignment to conditionally available mutable place repairs it to available;
47. static-ownership target may require explicit discard convergence before such replacement;
48. partial aggregate destruction destroys only still-owned available sub-places;
49. moved/discarded sub-place is never destroyed twice;
50. incompatible active borrow blocks move/discard;
51. consuming `Result.Ok()` / `Result.Err()` makes the original Result unavailable;
52. borrowed Result projections remain non-consuming;
53. try/match guarded move commits only after selected guard success;
54. mentor diagnostics identify source operation and practical correction;
55. formatter preserves every ownership marker;
56. LSP uses the same canonical ownership facts as Sema.
57. whole `is available` is false for a partially available aggregate;
58. whole `is not available` preserves still-owned sibling fields and its recursive mask;
59. uninitialized `is not available` preserves uninitialized provenance;
60. availability tests never invent moved, discarded, or detached reasons;
61. an earlier `<-source` call argument reserves against later overlapping read and borrow;
62. a reservation blocks a later overlapping second move;
63. a proven-disjoint sibling Place remains usable during reservation;
64. later outer-argument failure cancels only the outer reservation;
65. a successful outer call commits every prepared transfer exactly once;
66. a committed nested-call transfer is not rolled back by outer-call failure;
67. conditional discard can require runtime ownership state without an availability query;
68. conditional replacement can require runtime ownership state without an availability query;
69. conditional scope-exit cleanup can require runtime ownership state without an availability query;
70. strict static-ownership policy rejects the actual operation requiring forbidden runtime state.

---

## 36. Cross-rulebook ownership

This rulebook owns:

- source-level ownership responsibility;
- availability states and unavailability reasons;
- the separation of mutability, availability, borrowing, and generation validity;
- the general rule requiring visible consumption of reusable sources;
- pending call-transfer reservations and atomic commit/cancellation;
- whole-value versus sub-place availability requirements;
- partial availability and conditional availability;
- `is available` / `is not available` ownership-state semantics;
- discard as ownership-state convergence, in coordination with `discard.md`;
- ownership consequences of reinitialization and replacement;
- the prohibition on ordinary whole-self-consuming methods;
- the static-first/runtime-when-needed availability principle;
- runtime ownership-state necessity for every ownership-dependent operation;
- mentor-level ownership diagnostics requirements.

Other rulebooks own:

- exact copy/move classification and copy operators: `copy_move.md`;
- borrow lifetime and overlap legality: borrowing rulebook;
- detailed cleanup ordering and custom destruction: destruction rulebook;
- `discard` syntax and discardability: `rules/control-flow/discard.md`;
- function signature and overload syntax: `rules/declarations/functions.md`;
- lambda syntax and callable types: `rules/declarations/lambda-functions.md`;
- match patterns and guards: `rules/control-flow/flowcontrol_match.md`;
- error and try semantics: `rules/errors/errorhandling.md`;
- generation and reference validity: reference-model rulebooks;
- fixed-address/register/FFI ownership boundaries: platform and FFI rulebooks;
- diagnostic severity policy: canonical diagnostics/project policy rules;
- target/project spelling for static-ownership policy: project/target rulebooks;
- Semantic IR and backend representation: compiler/MLIR rulebooks.

---

## 37. Summary

The revision-2 ownership model can be summarized as:

```text
Own values explicitly.
Copy when copying is legal.
Show destructive transfer from reusable sources with <-.
Do not require move boilerplate for fresh temporaries or function-return boundaries.
Track availability per place.
Keep availability separate from mutability, borrowing, and generation validity.
Allow partial ownership only where the compiler can prove safe independent places.
Treat custom free as requiring complete self in Sec 0.1.
Use is available only for ownership-state refinement, never as null/Option semantics.
Use discard to converge ownership to Unavailable.
Destroy exactly what is still owned, exactly once.
Prefer static proof; permit checked runtime ownership state only when policy allows it.
Explain mistakes like a mentor, not like a compiler textbook.
```
