# Escape Analysis

- **Status:** Normative
- **Created:** 2026-08-08
- **Last updated:** 2026-11-07 (stable chapter, subsection, and paragraph numbering)
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/analysis/escape_analysis.md`
- **Replaces:** Earlier unversioned revision at the same canonical path
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`
- **Implementation governance:** `governance/analysis.yaml` (`sema.escape-analysis`)
- **Related rulebooks:** `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/copy_move.md`, `rules/memory/reference_model.md`, `rules/memory/storage.md`, `rules/memory/layout.md`, `rules/memory/destruction.md`, `rules/declarations/functions.md`, `rules/declarations/lambda-functions.md`, `rules/analysis/closure_analysis.md`, `rules/analysis/parameter_usage_analysis.md`, `rules/analysis/call_graph.md`, `rules/analysis/effect_analysis.md`, `rules/analysis/stack_analysis.md`, `rules/platform/ffi.md`, `rules/tooling/diagnostics.md`, `rules/tooling/lsp.md`, `rules/compiler/semantic_ir.md`, `rules/compiler/incremental_compilation.md`

---

## § 1 Status and authority

**§ 1(1)** This is the normative compiler-analysis rulebook for Sec 0.1 escape analysis.

**§ 1(2)** This rulebook defines the semantic purpose, analysis domain, dataflow model,
interprocedural summaries, conservative behavior, consumers, diagnostic
requirements, and completion criteria of Sec escape analysis.

**§ 1(3)** Mutable implementation status belongs to `sema.escape-analysis` in
`governance/analysis.yaml`.

**§ 1(4)** Escape analysis consumes canonical ownership, borrowing, storage, reference,
callable, and FFI facts. It does not redefine their source syntax or legality.

---

## § 2 Purpose

**§ 2(1)** Escape analysis determines whether and how a value, reference, view, address,
callable environment, handle, or storage dependency becomes usable outside the
context for which its current semantic dependencies were established.

**§ 2(2)** Escape analysis is a semantic dependency-flow analysis.

**§ 2(3)** It is not a storage-placement algorithm.

**§ 2(4)** It must not silently choose heap allocation, arena allocation, copying,
retention, or another ownership/storage strategy in order to make an otherwise
invalid program compile.

**§ 2(5)** Escape analysis exists to provide reusable facts for:

- lifetime validation;
- reference validation;
- ownership validation;
- storage validation;
- closure analysis;
- parameter-usage analysis;
- stack analysis;
- call-graph refinement;
- effect analysis;
- concurrency and ISR analysis;
- diagnostics and LSP tooling;
- optimization;
- later compiler lowering where an escape proof is useful.

---

## § 3 Normative role

**§ 3(1)** This rulebook is normative for:

- the definition of semantic escape;
- the distinction between value escape, borrow escape, address escape,
  retention, capture, concurrency transfer, and foreign retention;
- the canonical classes of escape subjects and destinations;
- the facts produced by escape analysis;
- the required interaction with canonical Place provenance;
- control-flow joins and bounded path-disjunctive provenance;
- loop and recursion fixed points;
- interprocedural escape summaries;
- conservative treatment of unresolved calls and opaque retention;
- aggregate, container, view, closure, handle, and backing-storage dependency
  propagation;
- separate-compilation summary requirements;
- diagnostic information required from escape facts;
- analysis consumers;
- required test classes;
- completion criteria for Sec 0.1 escape analysis.

**§ 3(2)** This rulebook does not redefine:

- ownership;
- copy or move legality;
- object lifetime;
- storage lifetime;
- storage origin;
- backing relation;
- reclamation authority;
- reference validity;
- borrow legality;
- closure capture syntax;
- FFI retention syntax;
- task/thread ownership rules;
- physical calling convention;
- physical stack placement.

**§ 3(3)** Those semantics remain owned by their respective normative rulebooks.

---

## § 4 Relationship to other analyses

**§ 4(1)** Escape analysis is intentionally separate from lifetime analysis.

**§ 4(2)** Conceptually:

```text
Escape analysis:
    Where and how does a value, capability, or dependency travel?

Lifetime analysis:
    Does the source remain valid for the destination's required lifetime?
```

**§ 4(3)** Escape analysis is also separate from ownership analysis:

```text
Ownership analysis:
    Who owns the value, and is copy/move/consume legal?

Escape analysis:
    Does that value or one of its dependencies become usable elsewhere?
```

**§ 4(4)** Escape analysis is separate from storage analysis:

```text
Storage facts:
    Where does storage come from, how is it invalidated, and who may reclaim it?

Escape facts:
    Where does a dependency on that storage travel?
```

**§ 4(5)** These analyses cooperate through explicit facts and shared canonical Place
provenance rather than duplicating one another's semantic models.

---

## § 5 Core rule

**§ 5(1)** Escape is a semantic dependency-flow property, not a physical storage-placement
decision.

**§ 5(2)** The following are distinct:

```text
value escape
reference escape
view escape
address escape
callable-environment escape
handle escape
backing-storage dependency
physical storage escape
```

**§ 5(3)** Returning or transferring an owned value does not imply that the source
`Automatic` storage itself escapes.

**§ 5(4)** For example:

```sec
fn MakeValue() LargeStruct {
    let value := LargeStruct { }
    return value
}
```

**§ 5(5)** The value may be transferred to the caller while the callee's `Automatic`
storage does not survive the function invocation.

**§ 5(6)** The compiler may implement this through result storage, semantic move, SSA
elision, or another semantics-preserving lowering.

**§ 5(7)** Escape analysis must not reinterpret such a transfer as implicit storage
promotion.

**§ 5(8)** A non-reference return is already an ownership/result boundary under the
canonical copy/move rules. Therefore `return value` may transfer a move-only
owned local to the caller. The equivalent explicit documentation form
`return <-value` is permitted but not required.

**§ 5(9)** Escape analysis must classify the resolved return as `ValueTransfer` regardless
of whether the optional return marker was written.

**§ 5(10)** The source local's `Automatic` storage does not thereby become caller storage.
The returned value may be lowered through caller result storage, move elision,
SSA forwarding, or another semantics-preserving mechanism. This preserves the
distinction between owned-value escape and physical source-storage escape.

---

## § 6 No implicit escape promotion

**§ 6(1)** Sec does not repair escape by changing storage behind the programmer's back.

**§ 6(2)** An analysis result such as:

```text
reference to Automatic local
    +
return to caller
```

**§ 6(3)** must never trigger an implicit transformation such as:

```text
promote local to heap
```

**§ 6(4)** or:

```text
deep-copy backing storage
```

**§ 6(5)** or another source-invisible ownership/storage change.

**§ 6(6)** When the required lifetime or retention cannot be provided by the program's
explicit semantics, safe code is invalid.

**§ 6(7)** This rule applies even when the active target provides a heap allocator.

---

## § 7 Escape is not a boolean

**§ 7(1)** The primary result of escape analysis must not be only:

```text
Escapes = true
```

**§ 7(2)** The analysis must retain enough information to determine:

- what escapes;
- how it escapes;
- where it escapes;
- what semantic origins/dependencies are carried;
- whether the behavior is known or conservative unknown.

**§ 7(3)** The canonical conceptual dimensions are:

```text
EscapeSubject
EscapeMode
EscapeDestination
```

**§ 7(4)** The exact implementation representation is not prescribed.

---

## § 8 Escape subjects

**§ 8(1)** Canonical escape subjects include:

```text
OwnedValue
SafeReference
View
RawPointer
CallableEnvironment
Handle
StorageDependency
Unknown
```

### § 8.1 OwnedValue

**§ 8.1(1)** Ownership of the value itself is transferred or made available in another
context.

**§ 8.1(2)** Examples include:

```sec
return value
```

**§ 8.1(3)** and ownership transfer into another owner.

**§ 8.1(4)** Owned-value escape may be ordinary and valid.

**§ 8.1(5)** It does not imply that source storage must escape.

**§ 8.1(6)** A non-reference return is already an ownership/result boundary, so
`return value` and the optional `return <-value` are the same `ValueTransfer`.

### § 8.2 SafeReference

**§ 8.2(1)** A safe reference becomes usable outside the context in which its current
validity dependencies were established.

**§ 8.2(2)** This is commonly the subject relevant to returned-borrow and retained-borrow
validation.

### § 8.3 View

**§ 8.3(1)** A non-owning or view-like descriptor becomes usable elsewhere while continuing
to depend on backing storage.

**§ 8.3(2)** Examples include:

- slices;
- borrowed collection views;
- subranges;
- other descriptor values that do not own the storage they expose.

### § 8.4 RawPointer

**§ 8.4(1)** A raw pointer/address capability may escape even when safe-reference guarantees
no longer apply.

**§ 8.4(2)** Unsafe code does not make pointer flow invisible to escape analysis.

**§ 8.4(3)** Raw-pointer escape facts remain relevant to:

- FFI analysis;
- closure analysis;
- task/thread transfer analysis;
- ISR analysis;
- target/hardware analysis;
- diagnostics and compiler auditing.

### § 8.5 CallableEnvironment

**§ 8.5(1)** A capturing callable may carry an environment whose dependencies escape with the
callable value.

**§ 8.5(2)** The exact capture model is refined by `rules/analysis/closure_analysis.md`.

**§ 8.5(3)** Escape analysis must nevertheless be able to represent environment dependencies
and their movement.

### § 8.6 Handle

**§ 8.6(1)** A handle may intentionally be designed to survive address relocation or storage
reuse through a generation/domain protocol.

**§ 8.6(2)** Handles must not be silently treated as ordinary safe references.

### § 8.7 StorageDependency

**§ 8.7(1)** Some values expose no direct borrowed reference while still requiring backing
storage to remain valid.

**§ 8.7(2)** Examples include view descriptors and wrappers whose validity depends on a
storage domain.

**§ 8.7(3)** The dependency must follow the value when the value escapes.

### § 8.8 Unknown

**§ 8.8(1)** When the subject or dependency behavior cannot be resolved soundly, the analysis
must represent conservative unknown rather than assume non-escape.

---

## § 9 Escape modes

**§ 9(1)** Canonical escape modes include:

```text
ValueTransfer
BorrowEscape
AddressEscape
Retention
Capture
ConcurrencyTransfer
ForeignRetention
Unknown
```

**§ 9(2)** Multiple modes may apply to one source across different flows.

### § 9.1 ValueTransfer

**§ 9.1(1)** Ownership of a value transfers to a different context.

**§ 9.1(2)** This is distinct from retaining a borrow.

### § 9.2 BorrowEscape

**§ 9.2(1)** A safe reference or view becomes usable beyond its current borrow/lifetime
context.

**§ 9.2(2)** Example:

```sec
fn Broken() ref int {
    int value := 10
    return ref value
}
```

**§ 9.2(3)** Escape analysis determines that the returned reference originates from the
local value.

**§ 9.2(4)** Lifetime/storage validation determines that the origin cannot satisfy caller
use.

**§ 9.2(5)** `int value := 10` is valid Sec 0.1 declaration syntax and is preserved here.
Any Sec 0.2 declaration-syntax migration belongs to the rulebooks that define
Sec 0.2.

### § 9.3 AddressEscape

**§ 9.3(1)** An address capability such as a raw pointer leaves its original context.

**§ 9.3(2)** Address escape remains visible even in unsafe code.

### § 9.4 Retention

**§ 9.4(1)** A callee, aggregate, registry, callback mechanism, or other destination may keep
a value/dependency after the immediate operation returns.

**§ 9.4(2)** Retention is not implied merely by crossing a function-call boundary.

### § 9.5 Capture

**§ 9.5(1)** A dependency becomes part of a callable environment.

**§ 9.5(2)** Capture does not itself imply that the environment escapes the function.

**§ 9.5(3)** The later flow of the callable determines the destination/lifetime demand.

### § 9.6 ConcurrencyTransfer

**§ 9.6(1)** A value, capability, callable, handle, or dependency becomes available in
another execution context such as a task or thread.

**§ 9.6(2)** Ownership, lifetime, Send-like restrictions, and synchronization remain separate
questions.

### § 9.7 ForeignRetention

**§ 9.7(1)** Foreign code may retain a value/address/callback beyond the immediate call.

**§ 9.7(2)** The allowed retention is determined by the FFI contract.

### § 9.8 Unknown

**§ 9.8(1)** When retention or transfer behavior cannot be resolved, the result must remain
conservative.

---

## § 10 Escape destinations

**§ 10(1)** Canonical destination classes include:

```text
Caller
OuterPlace
Aggregate
RetainingCall
ClosureEnvironment
Task
Thread
StaticStorage
ThreadLocalStorage
ForeignCode
ReturnedValue
Unknown
```

**§ 10(2)** The implementation may attach additional structured information.

### § 10.1 Caller

**§ 10.1(1)** A returned reference/view/handle remains available to the caller.

**§ 10.1(2)** This can be valid when the returned dependency originates from caller-provided
storage.

### § 10.2 OuterPlace

**§ 10.2(1)** A value or dependency is stored in a place whose required validity extends
beyond the current inner context.

### § 10.3 Aggregate

**§ 10.3(1)** A dependency is placed into an aggregate/container that may later escape.

**§ 10.3(2)** Storing into an aggregate does not by itself prove long-lived escape; the
carrier's later flow determines that.

### § 10.4 RetainingCall

**§ 10.4(1)** A callee contract permits retention after call return.

### § 10.5 ClosureEnvironment

**§ 10.5(1)** The dependency becomes part of a callable environment.

### § 10.6 Task / Thread

**§ 10.6(1)** The dependency crosses execution-context boundaries.

### § 10.7 StaticStorage / ThreadLocalStorage

**§ 10.7(1)** The dependency is stored in storage whose required lifetime is governed by the
static or thread-local domain.

### § 10.8 ForeignCode

**§ 10.8(1)** The dependency enters foreign code under a defined or conservative retention
contract.

### § 10.9 ReturnedValue

**§ 10.9(1)** A carrier aggregate, callable, view, handle, or other value is returned.

### § 10.10 Unknown

**§ 10.10(1)** The destination/lifetime demand cannot be resolved soundly.

---

## § 11 Function and lexical boundaries

**§ 11(1)** A function boundary does not by itself imply long-lived escape.

**§ 11(2)** A lexical scope boundary does not by itself define all escape behavior.

**§ 11(3)** For example:

```sec
fn Inspect(value: ref Value) void {
    ...
}

Inspect(value)
```

**§ 11(4)** with a proven non-retaining call contract represents call-duration use, not
retention beyond the call: a call-bounded shared borrow with no escape beyond
the call.

**§ 11(5)** Conversely:

```sec
fn Register(value: ref Value) void {
    ...
}

Register(value)
```

**§ 11(6)** with a retaining contract may create an escape even though the source and call
occur in the same lexical scope: the borrowed dependency may escape beyond call
return.

**§ 11(7)** Sec 0.1 borrowed parameters create the required call-bounded borrow from a
compatible caller Place without a separate `ref` or `ref mut` marker at the
call site.

**§ 11(8)** Escape analysis therefore consumes the resolved parameter borrow mode and
retention contract. It must not infer borrow mode from obsolete call-site
syntax.

**§ 11(9)** Escape is determined by future usability and retention requirements, not merely
by syntax nesting.

---

## § 12 Call retention contracts

**§ 12(1)** Call behavior must distinguish at least:

```text
NoRetention
MayRetain
TransfersOwnership
UnknownRetention
```

**§ 12(2)** These are conceptual categories. The exact function/FFI metadata representation
belongs to the corresponding function/FFI/compiler metadata rules.

### § 12.1 NoRetention

**§ 12.1(1)** The callee may use the argument only for the call's permitted dynamic extent and
may not retain the dependency after return.

### § 12.2 MayRetain

**§ 12.2(1)** The callee may keep the argument or a dependency derived from it after return.

### § 12.3 TransfersOwnership

**§ 12.3(1)** The callee consumes/transfers ownership according to the function's ownership
contract.

**§ 12.3(2)** This is not equivalent to retaining a borrow.

### § 12.4 UnknownRetention

**§ 12.4(1)** Retention behavior is unresolved or unavailable.

**§ 12.4(2)** Unknown retention is conservative for reference-like/address-like/callable
arguments unless another explicit contract supplies stronger guarantees.

---

## § 13 Call-boundary semantic classification

**§ 13(1)** Escape analysis consumes the resolved call contract.

**§ 13(2)** For a borrowed parameter:

```sec
fn Inspect(value: ref Buffer) void {
    ...
}

Inspect(buffer)
```

**§ 13(3)** the call establishes borrowed authority without transferring ownership.

**§ 13(4)** For a mutable borrowed parameter:

```sec
fn Modify(value: ref mut Buffer) void {
    ...
}

Modify(buffer)
```

**§ 13(5)** the call establishes temporary exclusive borrowed authority without transferring
ownership.

**§ 13(6)** For an ordinary by-value parameter, copy or ownership transfer follows the
concrete type's canonical copy/move classification.

**§ 13(7)** For a reusable move-only caller Place, transfer is explicit:

```sec
Process(<-resource)
```

**§ 13(8)** For an explicit consuming parameter:

```sec
fn Transform(-> data: BigArray) BigArray {
    ...
}

Transform(<-data)
```

**§ 13(9)** the resolved call contract is ownership transfer even when the concrete type
would otherwise be copyable.

**§ 13(10)** Fresh temporaries may satisfy consuming transfer without a redundant `<-`
marker.

**§ 13(11)** Escape analysis must record the resolved semantic relation:

```text
borrow
mutable borrow
copy
ownership transfer
retention
```

**§ 13(12)** rather than using source-token presence as the escape model.

**§ 13(13)** The escape summary must remain valid after source-level syntax has been lowered
to Semantic IR.

---

## § 14 Canonical sources

**§ 14(1)** Escape summaries and diagnostics need symbolic origin categories.

**§ 14(2)** Canonical conceptual sources include:

```text
Parameter
Receiver
Local
Temporary
Static
ThreadLocal
ArenaStorage
AllocatorBackedStorage
ForeignStorage
CapturedValue
ReturnedCallResult
Unknown
```

**§ 14(3)** Source classification describes origin.

**§ 14(4)** It does not itself decide validity, lifetime, ownership, or physical placement.

---

## § 15 Canonical sinks

**§ 15(1)** Canonical conceptual escape sinks include:

```text
Return
OuterAssignment
AggregateStore
StaticStore
ThreadLocalStore
RetainingCall
ClosureCapture
TaskTransfer
ThreadTransfer
ForeignCall
UnknownCall
HandleConstruction
```

**§ 15(2)** These sinks represent operations or destinations that may extend dependency
usability beyond the current context.

---

## § 16 Place provenance

**§ 16(1)** Escape analysis must consume canonical Place provenance.

**§ 16(2)** It must not replace the compiler's Place model with a separate symbol-only
origin system.

**§ 16(3)** Relevant projections include, where supported by canonical Place analysis:

```text
field
property where semantically addressable
constant index
dynamic index
slice/range
reference dereference
union variant payload
nested aggregate path
```

**§ 16(4)** Escape precision should preserve these projections whenever doing so is sound
and within the analysis precision budget.

**§ 16(5)** For example:

```sec
return ref pair.left
```

**§ 16(6)** should preserve:

```text
Parameter(0).Field(left)
```

**§ 16(7)** when `pair` is a parameter reference and the Place model can represent that
projection.

**§ 16(8)** It must not unnecessarily collapse the origin to the whole parameter.

---

## § 17 Disjoint places

**§ 17(1)** When canonical Place analysis proves places disjoint, escape analysis should
preserve that precision.

**§ 17(2)** Examples include, where proven:

```text
obj.left    vs obj.right
array[3]    vs array[7]
slice[0..4] vs slice[8..12]
```

**§ 17(3)** Escape of one disjoint projection must not automatically classify unrelated
projections as escaping.

---

## § 18 Dynamic indices and projections

**§ 18(1)** When an exact index or projection cannot be known, escape analysis may widen
locally.

**§ 18(2)** For example:

```text
Parameter(0).Index(3)
```

**§ 18(3)** may widen to:

```text
Parameter(0).AnyIndex
```

**§ 18(4)** without discarding the known root `Parameter(0)`.

**§ 18(5)** Conservative uncertainty should remain as local and structured as practical.

---

## § 19 Views and slices

**§ 19(1)** A view carries a dependency on its backing storage.

**§ 19(2)** Example:

```sec
fn Tail(values: ref int[]) ref int[] {
    return values[1..]
}
```

**§ 19(3)** The returned view must preserve that its backing dependency derives from
`values`.

**§ 19(4)** Static range information should be retained where known.

**§ 19(5)** Dynamic ranges may use an unknown/dynamic range projection while preserving the
known backing root.

**§ 19(6)** Escape of a slice/view does not imply ownership escape of the complete backing
aggregate.

**§ 19(7)** It does imply that the required backing-storage dependency travels with the
view.

---

## § 20 Aggregates as dependency carriers

**§ 20(1)** Aggregates may contain:

- owned values;
- safe references;
- views;
- handles;
- callable environments;
- other dependency-carrying values.

**§ 20(2)** Escape analysis must track relevant contained dependencies structurally.

**§ 20(3)** Example:

```sec
type Pair struct {
    item: ref Item
    count: int
}
```

**§ 20(4)** Returning the complete `Pair` carries the dependency in `item`.

**§ 20(5)** Reading or copying only `count` does not cause the `item` dependency to escape.

---

## § 21 Containers

**§ 21(1)** Placing a dependency in a local container does not automatically imply escape
outside the function.

**§ 21(2)** Example:

```sec
localCollection.Add(ref local)
```

**§ 21(3)** establishes a contained dependency.

**§ 21(4)** If the collection remains local and is destroyed before the dependency becomes
invalid, the operation may be valid.

**§ 21(5)** If the collection later escapes, the contained dependency escapes transitively.

**§ 21(6)** For dynamic collections where exact element identity cannot be represented
cheaply, the analysis may use conservative summarized element provenance.

**§ 21(7)** Sec 0.1 does not require a fully precise shape analysis for every collection
implementation.

**§ 21(8)** Compiler-known collection contracts may supply summary behavior.

---

## § 22 Fixed arrays

**§ 22(1)** Fixed arrays may be tracked per element when static index precision is practical.

**§ 22(2)** For large or dynamically indexed arrays, analysis may widen to summarized
projections such as:

```text
AnyIndex
```

**§ 22(3)** The compiler must not require one analysis node per physical element of a large
array solely to remain sound.

---

## § 23 Tagged unions and Result/Option payloads

**§ 23(1)** When the active variant is known, escape dependencies should be associated with
the active payload projection.

**§ 23(2)** Inactive payload storage must not be treated as a live dependency.

**§ 23(3)** When control flow merges several possible active variants, dependencies from all
possible live variants are joined conservatively.

**§ 23(4)** The same principle applies to `Result` and `Option` payloads where represented
as tagged semantic alternatives.

---

## § 24 Current provenance versus historical escape

**§ 24(1)** Escape analysis must distinguish:

```text
current provenance / current carrier dependencies
```

**§ 24(2)** from:

```text
historical escape facts
```

**§ 24(3)** Current provenance may change through mutation, rebinding, move, destruction,
or replacement.

**§ 24(4)** Historical escape facts are monotone.

**§ 24(5)** For example:

```sec
holder.ref = ref a
holder.ref = ref b
```

**§ 24(6)** after the second assignment, `holder.ref` no longer currently depends on `a`
when no alias retains the old value.

**§ 24(7)** However, if the earlier value was already passed to a retaining call, that
historical escape remains true.

---

## § 25 Assignment

**§ 25(1)** Dependency propagation through assignment must follow Sec's semantic
classification of the operation.

**§ 25(2)** Escape analysis must not infer copy versus move merely from assignment syntax.

**§ 25(3)** It consumes ownership/copy-move classification.

**§ 25(4)** Escape analysis must never decide that a value is movable merely because a flow
would be easier to represent as `ValueTransfer`.

**§ 25(5)** Likewise, it must not reinterpret an ordinary by-value call as a borrow or
reinterpret a borrow as ownership transfer to avoid an escape/lifetime failure.

**§ 25(6)** When ownership or borrow legality rejects the operation, escape facts may still
be retained for diagnostics, but escape analysis must not repair the operation
through hidden copy, heap promotion, storage substitution, or contract
weakening.

### § 25.1 Copy

**§ 25.1(1)** Copy creates a new value according to the type's copy semantics.

**§ 25.1(2)** Contained reference/view/handle dependencies are copied according to their own
semantic rules.

**§ 25.1(3)** Escape analysis does not decide whether a type is copyable.

### § 25.2 Move

**§ 25.2(1)** Move transfers the relevant dependencies to the destination and updates source
availability/ownership according to ownership analysis.

**§ 25.2(2)** The analysis must not model semantic move as if source and destination both
remain independent owners.

### § 25.3 Borrow/view rebinding

**§ 25.3(1)** Rebinding replaces the current provenance of the destination reference/view.

**§ 25.3(2)** It does not erase historical escape facts produced by the previous binding.

---

## § 26 Partial moves and structural updates

**§ 26(1)** Partial move, replacement, and reinitialization must operate at canonical Place
projection granularity.

**§ 26(2)** A move from:

```text
object.left
```

**§ 26(3)** must not automatically invalidate unrelated `object.right` escape provenance
when ownership/Place semantics keep the sibling valid.

**§ 26(4)** Escape analysis must consume the compiler's authoritative partial-availability
facts rather than implement its own competing partial-move legality model.

---

## § 27 Destruction and object lifetime

**§ 27(1)** The following events remain distinct:

```text
destruction
object lifetime end
storage reclamation
storage epoch transition
```

**§ 27(2)** Destroying a carrier ends its future ability to carry dependencies after its
object lifetime ends.

**§ 27(3)** Destruction does not retroactively erase escape that already occurred through
another value/capability.

**§ 27(4)** Escape analysis must not redefine lifetime validity.

**§ 27(5)** Lifetime analysis owns whether later use is legal.

---

## § 28 Storage domains, epochs, and generations

**§ 28(1)** Escape analysis preserves dependencies needed by the storage/reference model.

**§ 28(2)** A dependency may include relation to:

```text
storage domain
epoch/generation requirement
backing storage
reclamation/protection contract
```

**§ 28(3)** An epoch transition may make a previous dependency stale.

**§ 28(4)** It does not erase the historical fact that the dependency escaped.

**§ 28(5)** Handles that validate generation at runtime may remain valid semantic values
across address reuse according to their own contracts.

**§ 28(6)** Escape analysis must not collapse handles into ordinary direct references.

---

## § 29 Returned owned values

**§ 29(1)** A returned owned value must be distinguished from a returned borrow into local
storage.

**§ 29(2)** Example:

```sec
fn Make() Buffer {
    let buffer := Buffer(...)
    return buffer
}
```

**§ 29(3)** The return summary should represent an owned result such as:

```text
FreshOwnedValue
```

**§ 29(4)** or an equivalent symbolic result.

**§ 29(5)** It must not claim that the caller receives a borrow into the callee's local
storage merely because the source variable was local.

**§ 29(6)** A non-reference return is already an ownership/result boundary under the
canonical copy/move rules. `return buffer` may transfer a move-only owned local;
the explicit `return <-buffer` is permitted but not required, and both are
classified as `ValueTransfer`.

---

## § 30 Returned references and views

**§ 30(1)** Returned borrowed values must preserve symbolic origin paths.

**§ 30(2)** Examples include:

```text
Parameter(0)
Receiver
Parameter(1).Field(data)
Parameter(0).Index(3)
Parameter(0).Slice(2..8)
Captured(0)
Static(GlobalX)
Unknown
```

**§ 30(3)** Multiple possible origins must remain representable:

```text
Parameter(0) OR Parameter(1)
```

**§ 30(4)** Returned views must additionally preserve relevant backing dependencies.

---

## § 31 Calls

**§ 31(1)** Call transfer uses a callee escape summary or an explicit contract.

**§ 31(2)** Conceptually, for:

```sec
let result := F(arg0, arg1)
```

**§ 31(3)** analysis performs:

1. obtain the applicable summary/contract for `F`;
2. substitute symbolic `Parameter(0)` origins with the provenance of `arg0`;
3. substitute symbolic `Parameter(1)` origins with the provenance of `arg1`;
4. substitute the receiver where applicable;
5. apply symbolic projections;
6. propagate returned origins/dependencies to `result`;
7. apply callee retention/transfer/capture/concurrency/foreign effects to the
   arguments;
8. preserve source evaluation order for operand-side effects.

**§ 31(4)** The implementation may optimize this process but must preserve the same facts.

---

## § 32 Function summaries

**§ 32(1)** Every analyzable function must be able to produce a composable symbolic escape
summary.

**§ 32(2)** A conceptual summary includes:

```text
EscapeSummary {
    Parameters
    Receiver
    Return
    Retention
    Captures
    ConcurrencyTransfers
    ForeignTransfers
    UnknownEffects
}
```

**§ 32(3)** The exact in-memory and persisted representation is implementation-defined.

**§ 32(4)** The semantic information is normative.

---

## § 33 Parameter summary facts

**§ 33(1)** For each parameter, the summary must be able to represent combinations of facts
equivalent to:

```text
NoEscape
Returned
Retained
StoredInEscapingCarrier
OwnershipTransferred
Captured
TransferredToTask
TransferredToThread
PassedToForeign
UnknownRetention
```

**§ 33(2)** These are not necessarily mutually exclusive.

**§ 33(3)** A parameter may, for example, be both `Returned` and `Retained`.

---

## § 34 Receiver summary facts

**§ 34(1)** Method receivers must be represented explicitly in summaries.

**§ 34(2)** Example:

```sec
fn GetData() ref Buffer {
    return ref self.data
}
```

**§ 34(3)** must be able to produce a symbolic result equivalent to:

```text
Receiver.Field(data)
```

**§ 34(4)** Receiver-origin flows must not be collapsed to `Unknown` solely because the
receiver is implicit in source syntax.

---

## § 35 Return summaries

**§ 35(1)** A return summary must be able to represent:

```text
FreshOwnedValue
Parameter(index)
Receiver
Static(symbol)
ThreadLocal(symbol)
Captured(index)
Unknown
```

**§ 35(2)** and applicable projections such as:

```text
Field(name)
Index(constant)
AnyIndex
Slice(range)
UnknownProjection
```

**§ 35(3)** and disjunctions of possible sources.

**§ 35(4)** The summary must also carry relevant storage/backing dependencies for view- and
handle-like returned values.

---

## § 36 NoEscape

**§ 36(1)** `NoEscape` is a positive proof.

**§ 36(2)** It may be produced only when analysis proves that the relevant dependency:

- is not returned;
- is not retained;
- is not stored into an escaping carrier;
- is not captured by an escaping callable;
- is not transferred to a task/thread;
- is not exposed to foreign retention;
- does not pass through unresolved behavior that may retain it.

**§ 36(3)** Absence of a discovered escape is not automatically `NoEscape`.

---

## § 37 Unknown escape

**§ 37(1)** The analysis must distinguish:

```text
ProvenNoEscape
KnownEscape
UnknownEscape
```

**§ 37(2)** `UnknownEscape` is a first-class conservative result.

**§ 37(3)** Typical causes include:

- unresolved function-value target;
- missing persisted summary;
- opaque imported implementation;
- unknown foreign retention;
- precision exhaustion;
- unsupported call form;
- unknown compiler-known contract.

**§ 37(4)** Unknown must never be used as proof of non-escape.

---

## § 38 Function-value calls

**§ 38(1)** Function-value calls must consume callable-target information when available.

**§ 38(2)** For example, if a separate callable-flow analysis proves:

```text
Targets(f) = { A, B }
```

**§ 38(3)** then the escape result of calling `f` must conservatively combine the applicable
summaries of `A` and `B`.

**§ 38(4)** When the target set is unknown, the compiler uses the function/call contract if
one exists and otherwise falls back conservatively.

**§ 38(5)** The full callable-target/capture model is refined by `rules/analysis/closure_analysis.md`.

**§ 38(6)** Escape analysis must remain sound before that additional precision exists.

---

## § 39 Closures

**§ 39(1)** A capturing callable establishes explicit dependencies from its environment to
captured entities.

**§ 39(2)** Escape analysis distinguishes at least non-capturing callables, non-escaping
owned closures, escaping owned closures, shared-borrow closures,
mutable-borrow closures, and consuming closures.

**§ 39(3)** For example, conceptually:

```text
ClosureEnvironment
    depends on Local(state)
```

**§ 39(4)** If the closure remains local, the dependency may remain local.

**§ 39(5)** If the closure is returned, retained, stored into an escaping carrier, or
transferred to another execution context, its contained dependencies escape
transitively.

**§ 39(6)** `rules/analysis/closure_analysis.md` owns detailed capture mode and callable-flow precision.

**§ 39(7)** Escape analysis owns the movement/retention facts once those dependencies are
known.

**§ 39(8)** Owned closures may escape when their environment receives sufficient lifetime.
Borrowed closures may escape only while every captured borrow remains valid.
Environment storage is compiler-selected and may be eliminated, lexical,
regional, static, or dynamically owned; escaping does not itself promise heap
allocation. Any selected allocation or resource effect remains visible to the
relevant analyses.

---

## § 40 Task and thread transfer

**§ 40(1)** Task/thread transfer must remain explicit escape information.

**§ 40(2)** The analysis must distinguish semantic ownership transfer from borrowed
cross-context access.

**§ 40(3)** Conceptually useful distinctions include:

```text
MoveToTask
BorrowToTask
MoveToThread
BorrowToThread
```

**§ 40(4)** or equivalent structured facts.

**§ 40(5)** Escape analysis classifies the transfer.

**§ 40(6)** Other rules/analyses decide:

- whether ownership transfer is legal;
- whether the dependency lifetime is sufficient;
- whether thread/task sharing is permitted;
- whether synchronization is required.

---

## § 41 FFI and foreign retention

**§ 41(1)** Foreign calls must use explicit retention contracts defined by the FFI model.

**§ 41(2)** Escape analysis must be able to consume retention classes equivalent to:

```text
CallOnly
MayRetainUntilReturnOfHandle
MayRetainUntilExplicitRelease
MayRetainIndefinitely
TransfersOwnership
Unknown
```

**§ 41(3)** The exact source syntax and FFI declaration format are defined by `rules/platform/ffi.md`.

**§ 41(4)** Unknown foreign retention is conservative.

**§ 41(5)** Escape analysis consumes the canonical resolved FFI retention/ownership
contract.

**§ 41(6)** It must not infer foreign retention from parameter names, pointer type alone,
foreign symbol name, library name, or calling convention alone.

**§ 41(7)** Safe borrowed values must not rely on an unknown foreign lifetime.

**§ 41(8)** Raw-pointer escape in unsafe code may remain permitted while still being
recorded as escape.

---

## § 42 Static and thread-local destinations

**§ 42(1)** Storing a borrowed dependency into `Static` or `ThreadLocal` storage must be
represented as an escape to the corresponding storage domain.

**§ 42(2)** Escape analysis reports the destination and origin.

**§ 42(3)** Lifetime/storage validation determines whether the dependency can satisfy the
required lifetime.

**§ 42(4)** Owned value transfer remains separate from borrowed dependency retention.

**§ 42(5)** A borrow produced by `ThreadLocal[T].Borrow()` or `BorrowMut()` additionally
depends on the TLS key and current physical-thread attachment generation. It
must not escape to another thread, a migratable task, wider static/heap
storage, a returned context without the same-thread proof, or detached
execution. An independently owned value derived from TLS follows its ordinary
escape rules and does not retain TLS provenance merely because of its origin.

---

## § 43 Interprocedural composition

**§ 43(1)** Escape summaries must be composable.

**§ 43(2)** For a call chain:

```text
F -> G -> H
```

**§ 43(3)** `F` should be able to obtain a correct symbolic summary without every caller
re-analyzing all of `H` from source each time.

**§ 43(4)** Summaries therefore use symbolic parameter/receiver/capture/static paths rather
than caller-local Places.

---

## § 44 Recursion and SCCs

**§ 44(1)** Direct recursion, mutual recursion, recursive method cycles, and recursive
callable-summary cycles require fixed-point analysis.

**§ 44(2)** Call graph SCC information should be reused where applicable.

**§ 44(3)** The result must be independent of source declaration order.

**§ 44(4)** The implementation may use bottom-up monotone discovery, conservative initial
states, or another sound fixed-point algorithm.

**§ 44(5)** The normative requirements are:

- soundness;
- monotonicity;
- termination;
- declaration-order independence;
- conservative handling of unresolved behavior.

---

## § 45 Forward dataflow model

**§ 45(1)** Escape analysis is a monotone forward dataflow analysis over canonical Semantic
IR control flow and Place provenance.

**§ 45(2)** Conceptually, a relevant analysis state includes:

```text
Origins
Dependencies
EscapeFacts
CarrierFacts
Precision
```

**§ 45(3)** The exact representation is implementation-defined.

---

## § 46 Current state and historical state

**§ 46(1)** The analysis must model at least two kinds of information:

```text
current value/carrier provenance
historical escape facts
```

**§ 46(2)** Current provenance may be replaced or invalidated.

**§ 46(3)** Historical escape facts only accumulate.

**§ 46(4)** This distinction is required for sound mutation/rebinding analysis.

---

## § 47 Control-flow join

**§ 47(1)** At a join point, all possible continuing-path origins and escape facts must be
preserved.

**§ 47(2)** Conceptually:

```text
Origins      = union of possible origins
Dependencies = conservative union/structural join
EscapeFacts  = union
CarrierFacts = structural join
```

**§ 47(3)** The analysis must not arbitrarily choose one predecessor origin.

---

## § 48 Path-disjunctive provenance

**§ 48(1)** The analysis should preserve multiple alternative origins when practical.

**§ 48(2)** For example:

```sec
ref Item r

if condition {
    r = ref a
} else {
    r = ref b
}

return r
```

**§ 48(3)** must preserve the possibility:

```text
Origin(r) = a OR b
```

**§ 48(4)** until a valid conservative widening is required.

---

## § 49 Bounded precision

**§ 49(1)** Path-disjunctive precision may be bounded.

**§ 49(2)** The bound is an implementation tuning parameter and may change between compiler
versions.

**§ 49(3)** When the precision budget is exhausted, analysis must widen conservatively.

**§ 49(4)** It must never:

- discard a possible escape;
- choose one arbitrary source;
- convert uncertainty into `NoEscape`.

---

## § 50 Structured widening

**§ 50(1)** Widening should preserve as much independently known structure as practical.

**§ 50(2)** A preferred conceptual progression is:

```text
Parameter(0).Field(a).Index(3)
        ↓
Parameter(0).Field(a).AnyIndex
        ↓
Parameter(0).AnyProjection
        ↓
Parameter(0)
        ↓
Unknown
```

**§ 50(3)** The implementation need not use these exact classes.

**§ 50(4)** The normative rule is that loss of one dimension of precision should not force
unnecessary loss of unrelated known facts.

---

## § 51 Loops

**§ 51(1)** Loops require fixed-point dataflow.

**§ 51(2)** The loop state must conservatively combine:

```text
entry state
normal body fallthrough
continue edges
other continuing back-edges
break exit states for post-loop analysis
```

**§ 51(3)** Zero-iteration behavior must be preserved where the language permits it.

**§ 51(4)** Iteration-local bindings must not be incorrectly retained across iterations.

---

## § 52 Loop widening

**§ 52(1)** Loop-carried symbolic paths may otherwise grow without bound.

**§ 52(2)** The analysis must widen projections/origin sets where necessary to guarantee
termination.

**§ 52(3)** Widening should preserve known roots and known storage/dependency classes where
possible.

---

## § 53 Summary termination

**§ 53(1)** Interprocedural summary construction must also guarantee termination.

**§ 53(2)** Recursive symbolic path growth such as conceptually:

```text
x.field.field.field...
```

**§ 53(3)** must be bounded through projection widening or equivalent finite abstraction.

---

## § 54 Error, panic, and cleanup paths

**§ 54(1)** Escape analysis must operate over the canonical control-flow representation.

**§ 54(2)** Success, explicit error, panic, and cleanup paths participate according to their
actual semantics.

**§ 54(3)** Dependencies that exist only on one path must not be invented on unrelated
paths.

**§ 54(4)** When path precision is merged, the result must remain conservative.

---

## § 55 Defer and delayed uses

**§ 55(1)** A deferred operation creates a future use at its defined execution point.

**§ 55(2)** A deferred use may extend the required internal validity of a dependency.

**§ 55(3)** It does not automatically imply escape outside the function.

**§ 55(4)** Escape analysis must consume canonical defer/cleanup control flow or equivalent
delayed-use facts.

**§ 55(5)** It must not assume that a panic means no deferred cleanup/use occurs when the
language requires cleanup to run.

---

## § 56 Unreachable paths

**§ 56(1)** Proven unreachable runtime paths do not contribute escape facts.

**§ 56(2)** Compile-time-eliminated paths do not contribute runtime escape facts once the
language/compiler semantics have definitively removed them.

**§ 56(3)** Optimizer speculation must not be used to weaken source-level safety.

---

## § 57 Separate compilation

**§ 57(1)** Escape analysis must remain sound when a callee body is unavailable.

**§ 57(2)** The compiler must be able to consume a persisted escape summary or explicit
function/FFI contract.

**§ 57(3)** The minimum persisted semantic information includes, where applicable:

```text
parameter escape/retention facts
receiver escape/retention facts
return symbolic origins
ownership transfer relevant to escape
capture/retention behavior
concurrency transfer
foreign retention
unknown/conservative marker
summary schema/version
```

**§ 57(4)** Persisted summaries are compiler metadata.

**§ 57(5)** They are not user-written lifetime annotations.

---

## § 58 Persisted summary validation

**§ 58(1)** An imported summary must be validated against the relevant identity information,
including as applicable:

```text
function identity
function type/signature
generic specialization identity
relevant semantic/compiler version
escape-summary schema version
CompilationPlan-dependent identity
```

**§ 58(2)** A stale or incompatible summary must never be used for a safety proof.

**§ 58(3)** If the function body is available, the summary may be recomputed.

**§ 58(4)** If the body is unavailable and no compatible contract exists, analysis must fall
back conservatively.

---

## § 59 Summary versioning

**§ 59(1)** Persisted escape summaries require an internal schema/semantic version.

**§ 59(2)** The escape-summary version is independent of the Sec language version.

**§ 59(3)** When the meaning or schema changes incompatibly, old summaries must be rejected,
invalidated, or conservatively ignored.

**§ 59(4)** The compiler must never silently reinterpret old summary data with new meaning.

---

## § 60 Generic functions

**§ 60(1)** A generic function may share a generic escape summary when escape behavior is
independent of concrete specialization.

**§ 60(2)** A specialization-specific summary is required when escape behavior depends on
facts such as:

- ownership/copy traits;
- concrete field structure;
- callable behavior;
- target-selected implementation;
- specialization-selected code;
- other type-dependent semantics.

**§ 60(3)** The implementation may reuse equivalent summaries when equivalence is proven.

---

## § 61 CompilationPlan dependence

**§ 61(1)** Escape semantics should remain target-independent when the analyzed semantic
body is target-independent.

**§ 61(2)** When target/profile/configuration selection changes the semantic body or
contracts, the applicable `CompilationPlan` identity must participate in cache
selection/invalidation.

**§ 61(3)** A compiler must not reuse a summary across plans when the relevant escape
behavior differs.

**§ 61(4)** Escape analysis runs for the resolved `CompilationPlan`.

**§ 61(5)** Project, Target, Variant, and project-configuration precedence are owned by
`rules/projects/projects.md`. Escape analysis must not invent a second
project-configuration syntax.

**§ 61(6)** A persisted escape summary may be reused only when its schema, semantic
dependencies, and CompilationPlan-sensitive compatibility requirements remain
valid under the incremental-compilation rules.

**§ 61(7)** Missing or incompatible metadata falls back conservatively rather than being
accepted as proof of non-escape.

---

## § 62 Summary invalidation

**§ 62(1)** Cached escape summaries must be invalidated when relevant dependencies change.

**§ 62(2)** These include, as applicable:

```text
function body
callee summary
resolved call target
function contract
foreign retention contract
callable target set
generic specialization
Place/provenance semantics
relevant type semantics
CompilationPlan-selected path
analysis algorithm/schema version
```

**§ 62(3)** Incremental compilation may use dependency hashes or equivalent mechanisms.

**§ 62(4)** The normative requirement is correctness, not one cache implementation.

---

## § 63 Public compiler-facing result

**§ 63(1)** Escape analysis must expose reusable compiler-facing facts.

**§ 63(2)** A conceptual result is:

```text
EscapeAnalysisResult {
    FunctionSummaries
    PlaceFacts
    ValueFacts
    UnknownFacts
}
```

**§ 63(3)** This is not a required Go type.

**§ 63(4)** The normative requirement is that consumers can obtain the needed facts without
re-running an incompatible private escape interpretation.

---

## § 64 Cause paths

**§ 64(1)** Escape facts that lead to diagnostics must be able to produce a meaningful cause
path.

**§ 64(2)** Example:

```text
local `buffer`
    ↓ borrowed here
view `slice`
    ↓ stored in `request.body`
`request`
    ↓ passed to `Queue`
parameter may be retained
    ↓
dependency escapes local storage lifetime
```

**§ 64(3)** Cause paths may be reconstructed lazily.

**§ 64(4)** They need not be stored permanently for every successful analysis.

---

## § 65 Diagnostic architecture

**§ 65(1)** Escape analysis primarily produces facts.

**§ 65(2)** User diagnostics are derived by combining escape facts with the normative facts
from lifetime, ownership, storage, reference, FFI, concurrency, and other
relevant analyses/rules.

**§ 65(3)** Conceptually:

```text
EscapeResult
    +
Lifetime facts
    +
Ownership facts
    +
Storage/reference contracts
        ↓
Validation
        ↓
Diagnostic
```

**§ 65(4)** This separation keeps the escape result reusable for non-diagnostic consumers.

---

## § 66 Diagnostic categories

**§ 66(1)** The compiler must provide stable diagnostics for invalid escape classes.

**§ 66(2)** Categories should cover at least behavior equivalent to:

```text
escape.reference-outlives-origin
escape.view-outlives-backing
escape.retained-borrow
escape.static-store-of-short-lived-reference
escape.thread-local-store-of-short-lived-reference
escape.foreign-retention
escape.task-transfer
escape.thread-transfer
escape.closure-capture
escape.unknown-retention
escape.analysis-precision-exhausted
```

**§ 66(3)** These strings are stable escape-analysis categories unless and until a
registered diagnostic definition explicitly uses one as its symbolic diagnostic
name.

**§ 66(4)** Every emitted primary compiler diagnostic must use a stable registered
diagnostic ID governed by `rules/tooling/diagnostics.md`.

**§ 66(5)** Escape-analysis categories are not automatically diagnostic IDs.

**§ 66(6)** Diagnostic severity, configurability, notes, help, related locations, structured
fixes, CLI transport, JSON transport, and LSP diagnostic identity are owned by
`rules/tooling/diagnostics.md`.

**§ 66(7)** An existing published diagnostic ID must never be repurposed merely because an
escape category appears similar.

**§ 66(8)** Precision-exhaustion reporting is normally informational/debug-oriented rather
than a default user warning.

---

## § 67 Diagnostic quality

**§ 67(1)** An escape diagnostic should identify, where known:

- the escaping value/capability;
- the semantic origin;
- the relevant Place projection;
- the escape sink;
- the retention/transfer mode;
- the source storage/lifetime constraint;
- the destination requirement;
- the cause path;
- a source-level remedy when one can be recommended without changing semantics
  implicitly.

**§ 67(2)** A diagnostic should not merely say:

```text
reference escapes
```

**§ 67(3)** when the compiler knows the dependency chain.

**§ 67(4)** When the escape depends on an earlier borrow, transfer, capture, storage
placement, or retaining call, that source location should be transported as a
related location when available.

**§ 67(5)** Escape diagnostics should expose semantic facts such as:

```text
origin Place
origin storage domain
escape sink
escape mode
retention contract
backing-storage dependency
cause path
```

**§ 67(6)** rather than forcing the renderer or LSP to reconstruct them from English text.

**§ 67(7)** The analysis must not emit a misleading concrete lifetime claim when the actual
reason for rejection is conservative UnknownEscape or UnknownRetention.

---

## § 68 Unknown-retention diagnostics

**§ 68(1)** When safety cannot be established because retention behavior is unknown, the
compiler should explain the missing contract.

**§ 68(2)** For example, conceptually:

```text
error: retention behavior of function value is unknown

this argument contains a reference to local `buffer`
the callee may retain the argument after this call
```

**§ 68(3)** or:

```text
error: foreign retention contract is required

the foreign function receives a pointer derived from `data`
```

**§ 68(4)** Unknown behavior should not be reported as a misleading concrete lifetime fact.

---

## § 69 Precision exhaustion

**§ 69(1)** Reaching an internal precision cap is not normally a source warning by itself.

**§ 69(2)** The analysis widens conservatively.

**§ 69(3)** Compiler analysis dumps, mentor mode, or development diagnostics may expose that
precision was widened.

**§ 69(4)** If widening causes safe compilation to fail because the compiler cannot prove a
required guarantee, the final diagnostic should explain that the required
safety relation could not be proven and should include the most specific known
cause.

---

## § 70 LSP consumers

**§ 70(1)** Escape results should be suitable for LSP/IDE features such as:

```text
Returned borrow from parameter `buffer`
This parameter may be retained by the callee
This closure captures `state` and escapes through the return value
```

**§ 70(2)** These user-facing features are not required to be fully implemented with the
first escape-analysis implementation.

**§ 70(3)** The analysis contract must not prevent them.

---

## § 71 Parameter-usage analysis consumer

**§ 71(1)** `rules/analysis/parameter_usage_analysis.md` consumes escape facts.

**§ 71(2)** For example, facts such as:

```text
ReadOnly
NoEscape
NoRetention
NoIdentityRequirement
```

**§ 71(3)** may allow parameter-usage analysis to recommend a borrowed reference or view in
place of an unnecessarily large by-value parameter.

**§ 71(4)** Escape analysis must not itself rewrite function signatures or issue those
higher-level ergonomic recommendations.

**§ 71(5)** Revision 2.0 parameter-usage analysis consumes semantic ownership-transfer facts
rather than source move-token presence.

**§ 71(6)** Escape summaries therefore expose semantic facts such as:

```text
NoEscape
NoRetention
Returned
Retained
OwnershipTransferred
```

**§ 71(7)** and must not encode `caller wrote <-` as the inter-analysis contract.

**§ 71(8)** Source syntax remains available separately for diagnostics and code actions when
needed.

---

## § 72 Closure-analysis consumer

**§ 72(1)** `rules/analysis/closure_analysis.md` consumes:

- callable-environment dependencies;
- whether the closure escapes;
- where it escapes;
- whether it crosses task/thread/foreign boundaries;
- whether retention outlives capture dependencies.

**§ 72(2)** Closure analysis may refine callable-target and capture precision.

**§ 72(3)** Escape analysis remains the authority for dependency movement/retention once
those dependencies are identified.

---

## § 73 Stack-analysis consumer

**§ 73(1)** Stack analysis may consume proven non-escape and known escape facts when
reasoning about valid storage strategies and frame lifetime.

**§ 73(2)** Escape does not command heap allocation.

**§ 73(3)** A value that cannot remain in `Automatic` storage must either already have an
explicitly valid storage strategy or the program is invalid.

---

## § 74 Call-graph consumer and dependency

**§ 74(1)** Escape analysis may consume call-graph SCCs and resolved direct call edges.

**§ 74(2)** Call graph may in turn consume escape/callable facts to refine:

- retained callbacks;
- closure reachability;
- task/thread reachability;
- indirect-call targets where known.

**§ 74(3)** The analyses may cooperate through fixed points or staged refinement.

**§ 74(4)** Neither may assume unsound facts from the other.

---

## § 75 Effect-analysis relationship

**§ 75(1)** Escape facts and effects are related but not identical.

**§ 75(2)** Escape analysis owns dependency movement and retention.

**§ 75(3)** Effect analysis owns the canonical effect model.

**§ 75(4)** The compiler must not duplicate the complete effect summary inside escape
summaries merely because certain operations such as spawn, foreign interaction,
or reclamation affect both analyses.

---

## § 76 ISR and concurrency consumers

**§ 76(1)** Later analyses must be able to ask questions such as:

```text
Does this dependency leave the current execution context?
Does this callable environment depend on thread-local state?
Can this argument be retained asynchronously?
Does this handle cross task/thread boundaries?
```

**§ 76(2)** Task/thread/foreign escape classes are therefore mandatory parts of the escape
analysis contract.

---

## § 77 Semantic IR boundary

**§ 77(1)** Normative Sec escape analysis runs on Semantic IR and canonical Place provenance
before Sec MLIR lowering.

**§ 77(2)** Later MLIR/LLVM alias, escape, liveness, or optimization analyses may exist.

**§ 77(3)** They do not replace the normative Sec escape analysis used to enforce language
safety.

**§ 77(4)** The compiler must not lower to machine pointers and then attempt to reconstruct
Sec escape semantics from low-level IR.

**§ 77(5)** Escape proofs required by later lowering may be transferred as explicit facts,
analysis results, or discharged obligations according to the Sec MLIR
architecture.

---

## § 78 Governance

**§ 78(1)** Normative escape-analysis behavior remains in this rulebook.

**§ 78(2)** Mutable implementation status belongs to:

```text
governance/analysis.yaml
```

**§ 78(3)** under:

```text
sema.escape-analysis
```

**§ 78(4)** That integration should track granular implementation capabilities including at
least:

```text
escape subject classification
escape destination classification
escape mode classification
canonical Place provenance
disjoint projection precision
view/backing-storage dependencies
aggregate/container propagation
current versus historical provenance
direct-call summaries
recursive summary fixed points
callable-target integration
closure-environment integration
borrow retention
ownership-transfer escape
return transfer
task/thread transfer
FFI retention
static/TLS storage escape
separate-compilation summaries
incremental invalidation
diagnostic evidence
LSP integration
```

**§ 78(5)** Implementation state owned by another subsystem remains in that subsystem's
governance entry.

**§ 78(6)** In particular:

```text
ownership and copy/move legality
    owning memory/declaration governance integration

borrowing legality
    owning memory governance integration

diagnostic registry and transport
    governance/errors_diagnostics.yaml

closure capture/callable-flow implementation
    sema.closure-analysis

parameter-demand implementation
    sema.parameter-usage-analysis

call graph implementation
    analysis.call-graph or its current canonical integration

FFI contract implementation
    owning FFI governance integration

LSP presentation
    governance/tooling_lsp.yaml

incremental summary compatibility/invalidation
    owning incremental-compilation governance integration
```

**§ 78(7)** `sema.escape-analysis` may depend on those integrations but must not duplicate
their mutable implementation state.

---

## § 79 Required implementation capabilities

**§ 79(1)** Sec 0.1 escape analysis is required to support, soundly and according to the
available language features:

```text
direct functions
methods and receivers
parameters
returns
locals
temporaries
canonical Place projections
struct fields
fixed-array elements
slices/views
union payloads
Result/Option payloads
aggregate-contained dependencies
moves
copies
rebinding
replacement
branches
loops
defer/cleanup control flow
recursive functions
mutual recursion
direct-call summaries
separate-compilation summaries
static storage
thread-local storage
retaining calls
foreign retention
task/thread transfers
closure environments
function-value calls conservatively
```

**§ 79(2)** The detailed callable-flow precision of function values may be provided later by
`rules/analysis/closure_analysis.md`, but escape behavior must remain conservative and sound
without it.

---

## § 80 Positive test requirements

**§ 80(1)** The test suite must include valid examples equivalent to:

```text
return owned local value
return borrow from parameter
return borrow from receiver field
non-retaining call with local borrow
local aggregate containing local borrow that does not escape
disjoint field references
disjoint constant array elements
disjoint static slice ranges
returned slice backed by caller input
move owned value into returned aggregate
safe static reference
valid TLS-contained dependency where contracts permit
valid task ownership transfer
valid thread ownership transfer
valid foreign CallOnly pointer use
recursive summary convergence
mutually recursive summary convergence
```

---

## § 81 Negative test requirements

**§ 81(1)** The test suite must include invalid examples equivalent to:

```text
return reference to Automatic local
return view backed by Automatic local
store local borrow into Static storage
store local borrow into longer-lived ThreadLocal storage
pass local borrow to retaining call
foreign code may retain pointer/reference into local storage
return aggregate containing local reference
return callable containing invalid local capture
task retains reference to ending local
thread transfer of invalid borrowed dependency
transitive escape through multiple calls
transitive escape through aggregate field
escape through union payload
escape present on only one branch
escape discovered only after loop fixed point
```

---

## § 82 Precision test requirements

**§ 82(1)** The test suite must protect analysis precision as well as soundness.

**§ 82(2)** Required classes include:

```text
field A escape does not mark disjoint field B
constant index 1 does not imply escape of constant index 2
disjoint static slice ranges remain distinct
dynamic index widens only index precision when root remains known
unknown projection preserves known root when possible
branch merge retains all possible origins
projection widening terminates
origin-count cap widens conservatively
aggregate-contained origin precision survives whole-value transfer
```

---

## § 83 Interprocedural test requirements

**§ 83(1)** Required classes include:

```text
direct returned parameter
transitive returned parameter
receiver-derived return
field/index/slice-projection-derived return
callee retention propagates to caller
callee ownership transfer remains distinct from borrow retention
recursive SCC
mutually recursive SCC
declaration-order independence
generic specialization behavior
missing summary conservative fallback
stale/incompatible summary rejection
```

---

## § 84 Callable and closure test requirements

**§ 84(1)** Even before full closure-analysis precision, the escape suite must include:

```text
non-capturing named function value
known function-value targets
unknown function-value target
capturing closure remains local
capturing closure returned
closure passed to non-retaining call
closure passed to retaining call
closure transferred to task
closure transferred to thread
```

**§ 84(2)** Unknown callable-target cases may use conservative behavior.

---

## § 85 FFI test requirements

**§ 85(1)** Required FFI classes include:

```text
foreign CallOnly
foreign explicit retention
foreign ownership transfer
foreign unknown retention
RawPtr escape in unsafe
safe reference/view rejected when foreign lifetime is unknown
callback retention contract
```

---

## § 86 Storage and generation test requirements

**§ 86(1)** Escape integration tests must include cases equivalent to:

```text
arena-backed owned result where the language/storage contract permits transfer
borrow escaping across arena reset
handle escaping across generation change
view backed by borrowed storage
allocator-backed owned transfer
storage-domain reuse at the same numeric address
```

**§ 86(2)** Escape analysis may not be the sole analysis responsible for the final
diagnostic in these tests.

**§ 86(3)** The tests verify correct cooperation with storage, lifetime, and reference
analysis.

---

## § 87 Concurrency test requirements

**§ 87(1)** Required classes include:

```text
move-owned value to task
invalid borrow to longer-lived task
move-owned value to thread
invalid borrowed dependency across thread boundary
invalid TLS dependency across thread boundary
protected/generational handle transfer where contract permits
foreign asynchronous retention
```

---

## § 88 Differential Place-provenance tests

**§ 88(1)** Because escape analysis depends on canonical Place provenance, tests must ensure
that known Place precision is not unnecessarily discarded.

**§ 88(2)** For example:

```text
Place:  Parameter(0).Field(left)
Escape: Parameter(0).Field(left)
```

**§ 88(3)** should remain precise when no widening requirement exists.

**§ 88(4)** New Place/provenance capabilities should normally receive corresponding escape
integration tests.

---

## § 89 Revision 2.0 ownership and borrow-call test requirements

**§ 89(1)** The test suite must include at least:

```text
shared borrowed parameter called without explicit call-site ref
mutable borrowed parameter called without explicit call-site ref mut
borrowed non-retaining call remains call-local
borrowed retaining call produces retention escape
reusable move-only caller Place transfer uses canonical <-
fresh temporary consumption remains marker-free
explicit -> consuming parameter yields ownership-transfer escape
return value yields ValueTransfer without requiring <-
return <-value yields equivalent escape facts when legal
owned return does not imply physical source-storage escape
escape summary stores semantic ownership transfer rather than source-token presence
escape category identity remains distinct from registered diagnostic ID
unknown retention diagnostic reports uncertainty rather than invented concrete lifetime
```

---

## § 90 Completion criteria

**§ 90(1)** Escape analysis is complete for Sec 0.1 when all applicable conditions below are
met:

1. All required escape subjects can be represented.
2. All required escape sinks can be classified.
3. Escape modes and destinations remain distinct.
4. Canonical Place provenance is consumed rather than replaced.
5. Relevant field/index/slice/union/aggregate precision is preserved within a
   bounded model.
6. Precision exhaustion widens conservatively.
7. Branches and loops reach sound fixed points.
8. Direct interprocedural summaries are composable.
9. Recursive and mutually recursive SCC summaries converge soundly.
10. Summary results are independent of declaration order.
11. Separate compilation can consume validated/versioned summaries or explicit
    conservative contracts.
12. Unknown behavior is explicit and conservative.
13. Ownership transfer remains distinct from borrow retention.
14. Aggregates, containers, views, handles, and callable environments carry
    dependencies transitively.
15. Task/thread transfer is represented explicitly.
16. Foreign retention is represented explicitly and unknown foreign behavior is
    conservative.
17. Lifetime/storage/reference validation can consume the produced facts.
18. Parameter-usage, closure, stack, call-graph, effect, concurrency/ISR, LSP,
    and optimization consumers can consume the required facts without inventing
    a competing escape model.
19. Diagnostics can identify origin, sink, mode, and meaningful cause path.
20. Positive, negative, precision, interprocedural, callable, FFI,
    storage/generation, and concurrency test classes pass.
21. No invalid escape is repaired through implicit heap promotion, hidden copy,
    hidden backing replacement, or another source-invisible semantic change.

---

## § 91 Summary of mandatory rules

```text
Escape analysis determines whether and how a value, reference, view, address,
callable environment, handle, or storage dependency becomes usable outside the
context for which its current semantic dependencies were established.

Escape is a semantic dependency-flow property, not a physical storage-placement
decision.

Value escape, reference escape, address escape, backing-storage dependency, and
physical storage escape are distinct concepts.

Returning or transferring an owned value does not imply that source Automatic
storage itself escapes.

Escape analysis classifies escape form and destination rather than reducing the
result to a boolean.

A function boundary does not by itself imply retention or long-lived escape.

Owned value escape may be valid and ordinary.

Escape analysis identifies dependency movement. Lifetime analysis determines
whether source dependencies remain valid for the destination's required
lifetime.

Escape analysis must not repair invalid escape through hidden allocation,
implicit heap promotion, hidden deep copy, or another source-invisible ownership
or storage change.

Escape analysis consumes canonical Place provenance and preserves relevant
field, index, slice, union-payload, and aggregate-contained origin precision.

Control-flow joins may produce multiple possible origins.

Bounded precision is permitted, but exhausted precision must widen
conservatively and must never produce unsound non-escape.

Aggregates, arrays, containers, views, handles, and callable environments carry
dependencies structurally.

Current carrier provenance may change through mutation, move, rebinding,
replacement, destruction, or lifetime end. Historical escape facts remain
monotone.

Move follows ownership transfer. Copy follows Sec copy semantics. Escape
analysis does not independently decide move/copy legality.

Object destruction, object lifetime end, storage reclamation, and storage epoch
transition remain distinct events.

Call transfer instantiates symbolic callee summaries with caller provenance.

Function summaries describe parameter, receiver, return, retention, capture,
concurrency, foreign, and unknown escape behavior.

NoEscape is a positive proof and must not be inferred from absence of a known
escape.

Unknown escape and retention are explicit first-class results.

Recursive interprocedural analysis is solved to a sound conservative fixed point
and must be independent of declaration order.

Unknown call targets use explicit contracts where available and otherwise fall
back conservatively.

Function-value calls combine known target summaries and remain conservative when
target sets are unknown.

Closure capture creates dependencies from callable environments to captured
entities; closure analysis may refine capture and target precision.

Task/thread transfers remain explicit escape classes and do not collapse
ownership, lifetime, and synchronization into one property.

FFI retention is governed by explicit foreign contracts and unknown foreign
retention is conservative.

Separate compilation uses versioned, validated summaries or explicit contracts.
Missing or incompatible metadata falls back conservatively.

Lifetime, ownership, storage, reference, and escape analyses cooperate through
shared facts and canonical Place provenance rather than duplicating one
another's semantic models.

Normative Sec escape analysis runs on Semantic IR before Sec MLIR lowering.
Later backend escape/alias analyses do not replace it.

Mutable implementation status is governed by `sema.escape-analysis` in
`governance/analysis.yaml` rather than by this normative rulebook.
```
