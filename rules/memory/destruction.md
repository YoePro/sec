# Destruction

- **Status:** Normative
- **Created:** 2026-08-28
- **Last updated:** 2026-08-28
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/memory/destruction.md`
- **Replaces:** `rules/memory/destruction.txt`
- **Repository baseline reviewed:** `069c111`

---

## § 1 Purpose and authority

**§ 1(1)** Destruction is the deterministic semantic end of an owned Sec value's lifetime.

**§ 1(2)** This rulebook defines when destruction responsibility exists, when it ends, how cleanup is ordered, how partial and conditional ownership affect destruction, how custom `free` participates, and what later compiler stages must preserve.

**§ 1(3)** Destruction is ownership-aware and path-sensitive. The compiler must never destroy a value that the current owner no longer owns.

**§ 1(4)** Ordinary destruction must not require garbage collection, reference counting, runtime borrow tracking, a global destruction registry, or programmer-written lifetime annotations.

**§ 1(5)** `rules/memory/ownership.md` owns Place identity, availability, partial and conditional availability, ownership transfer, reinitialization authority, and `is available` semantics.

**§ 1(6)** `rules/memory/copy_move.md` owns copy/move classification and explicit move syntax. This rulebook owns the cleanup consequences of those actions.

**§ 1(7)** `rules/control-flow/discard.md` owns discard syntax and discardability. This rulebook owns deterministic destruction performed by a legal discard.

**§ 1(8)** `rules/control-flow/defer.md` owns defer registration and invocation-scoped behavior. This rulebook owns the automatic-destruction side of the unified cleanup order.

**§ 1(9)** `rules/declarations/impl.md` owns lifecycle declaration syntax for `init` and `free`. This rulebook owns destruction semantics of fully and partially constructed values.

**§ 1(10)** Physical hardware storage, volatile accesses, register semantics, ISR restrictions, and target execution constraints are governed by their specialized platform rulebooks. Destruction must respect those rules and must not invent physical hardware side effects.

---

## § 2 Core terminology

### § 2.1 Destruction

**§ 2.1(1)** Destruction ends the current owner's responsibility for a value and performs every cleanup action required by the resolved type and current ownership state.

**§ 2.1(2)** Destruction may lower to no machine operation for a trivially destructible value.

### § 2.2 Cleanup

**§ 2.2(1)** Cleanup is the compiler-generated sequence of destruction and registered deferred work that executes when a lifetime, scope, control-flow region, or invocation exits according to Sec semantics.

**§ 2.2(2)** Cleanup is not synonymous with deallocation, `free`, or `defer`.

### § 2.3 Resource release

**§ 2.3(1)** Resource release is a type- or API-defined operation that releases an external or logical resource such as a handle, mapping, lock state, foreign object, device registration, or allocator-owned block.

### § 2.4 Deallocation

**§ 2.4(1)** Deallocation releases owned allocation storage through the allocator or authority that owns that storage.

**§ 2.4(2)** A `RawPtr[T]` does not imply deallocation responsibility.

### § 2.5 `free`

**§ 2.5(1)** `free` is the custom lifecycle cleanup member used only when compiler-derived destruction of owned fields cannot fully express the type's cleanup obligation.

**§ 2.5(2)** `free` is not an ordinary method and is not called as `value.free()`.

### § 2.6 Terminal ownership action

**§ 2.6(1)** Every non-trivially destructible owned value must eventually reach exactly one semantically valid terminal ownership action on every reachable path where the value exists.

**§ 2.6(2)** Terminal ownership actions include destruction, legal discard, return transfer, consuming transfer, ownership transfer into another owner, permitted FFI transfer, or a termination operation explicitly defined to bypass cleanup.

---

## § 3 Destruction classification

### § 3.1 Required classification

**§ 3.1(1)** Every resolved Sec type must have a compiler-derived destruction classification before destruction-sensitive semantic analysis completes.

**§ 3.1(2)** The canonical conceptual classifications are:

```text
TriviallyDestructible
NonTriviallyDestructible
```

**§ 3.1(3)** An implementation may use richer internal classifications if observable semantics remain equivalent.

### § 3.2 Trivially destructible

**§ 3.2(1)** A type is trivially destructible when ending an owned value's lifetime requires no observable cleanup action.

**§ 3.2(2)** Typical examples include ordinary scalar values, references, mutable references, `RawPtr[T]`, fieldless enums, and aggregates containing only trivially destructible owned fields.

**§ 3.2(3)** Trivial destruction emits no mandatory machine operation, though the compiler may still update compile-time ownership state.

### § 3.3 Non-trivially destructible

**§ 3.3(1)** A type is non-trivially destructible when ending an owned value's lifetime may require observable cleanup directly or recursively through owned sub-values.

**§ 3.3(2)** Typical examples include owning collections, owned allocations, resource wrappers, file/socket/device handles, mappings, foreign owned resources, types with custom `free`, and aggregates containing non-trivially destructible owned fields.

**§ 3.3(3)** Classification follows the resolved representation and ownership contract, not the source type name alone.

**§ 3.3(4)** A `string` or another standard type may be trivial or non-trivial according to its canonical ownership representation. The compiler must not infer cleanup cost solely from the spelling of the type.

### § 3.4 Independent from copy classification

**§ 3.4(1)** Destruction classification and copy classification are separate semantic properties.

**§ 3.4(2)** `@noCopy` does not by itself imply non-trivial destruction.

**§ 3.4(3)** A copyable value may still require destruction if its canonical copy semantics produce independently owned cleanup responsibility.

---

## § 4 General exact-once rule

**§ 4(1)** An owned value that remains owned when its lifetime ends must be destroyed exactly once if its type requires destruction.

**§ 4(2)** A value whose ownership has transferred elsewhere must not be destroyed by its former owner.

**§ 4(3)** An unavailable Place is never destroyed again by its former owner.

**§ 4(4)** A Place that was never successfully initialized does not own a value and must not be destroyed.

**§ 4(5)** A compiler optimization may remove physical cleanup only when it proves that the observable destruction semantics remain unchanged.

**§ 4(6)** Double destruction is a compiler correctness defect. It must never be treated as an acceptable backend artifact.

---

## § 5 Automatic destruction

**§ 5(1)** Ordinary owned values are destroyed automatically when their lifetime ends unless ownership has already transferred or another canonical terminal ownership action has ended the responsibility.

**§ 5(2)** Ordinary source code must not require explicit user-written cleanup solely to end normal local value lifetimes.

```sec
fn Process() void {
    let buffer := CreateBuffer()
    Use(buffer)
}
```

**§ 5(3)** If `buffer` still owns a non-trivial value when its lifetime ends, the compiler inserts the required destruction.

**§ 5(4)** Automatic destruction is source-language semantics. It must not be inferred late from unused backend values.

---

## § 6 Cleanup registration and unified LIFO order

### § 6.1 Automatic destruction registration

**§ 6.1(1)** Successful initialization of an owned value establishes a destruction responsibility at that initialization point.

**§ 6.1(2)** Legal reinitialization establishes a new destruction responsibility for the new value.

**§ 6.1(3)** Move, discard, detach, return transfer, and other terminal ownership actions cancel the former owner's destruction responsibility for the transferred or ended value.

### § 6.2 Unified cleanup stack

**§ 6.2(1)** Deferred cleanup and ordinary automatic destruction participate in one common LIFO cleanup order according to source-semantic registration time.

```sec
fn Example() void {
    let first := OpenFirst()

    defer {
        Log("leaving")
    }

    let second := OpenSecond()
}
```

**§ 6.2(2)** The conceptual registration order is:

```text
destroy first
defer Log
destroy second
```

**§ 6.2(3)** The corresponding exit order is:

```text
destroy second
run defer Log
destroy first
```

**§ 6.2(4)** This ordering is normative.

**§ 6.2(5)** `defer` is not a separate phase that runs all deferred blocks before or after all automatic destruction.

### § 6.3 Earliest legal destruction

**§ 6.3(1)** Unified cleanup ordering does not force every value to remain live until invocation exit.

**§ 6.3(2)** A value may be destroyed at its earliest legal lifetime end when ownership, borrowing, defer dependencies, control flow, and observable ordering permit it.

**§ 6.3(3)** A deferred use may extend the lifetime of the value it reads until the deferred block executes.

---

## § 7 Deterministic destruction order

### § 7.1 Locals

**§ 7.1(1)** Owned local values in the same active cleanup region are destroyed in reverse successful initialization/cleanup-registration order, subject to earlier legal lifetime end.

**§ 7.1(2)** A declaration whose initialization never completed is not destroyed as a completed value.

### § 7.2 Struct fields

**§ 7.2(1)** Compiler-derived struct field destruction occurs in reverse declaration order unless a more specialized type rule defines another deterministic order.

```sec
type Example struct {
    First: First
    Second: Second
    Third: Third
}
```

**§ 7.2(2)** Default field destruction order is:

```text
Third
Second
First
```

**§ 7.2(3)** Optimization must not reorder observable field destruction.

### § 7.3 Fixed arrays

**§ 7.3(1)** A fixed array destroys initialized still-owned elements in reverse index order.

**§ 7.3(2)** Elements already moved or discarded are skipped.

### § 7.4 Dynamic owning collections

**§ 7.4(1)** An owning dynamic collection destroys its initialized still-owned elements according to the collection's canonical destruction order and then releases its backing storage through the recorded allocation authority.

**§ 7.4(2)** Capacity beyond the initialized logical element range is not destroyed as if it contained live values.

---

## § 8 Temporaries and full expressions

**§ 8(1)** Owned temporary values must be destroyed deterministically if ownership has not transferred.

**§ 8(2)** A temporary normally remains valid through the enclosing full expression unless the language rule for that construct or an earlier ownership transfer ends it sooner.

**§ 8(3)** Successfully created remaining temporaries in one full expression are destroyed in reverse successful creation order after the expression completes.

```sec
Use(CreateFirst(), CreateSecond())
```

**§ 8(4)** With left-to-right argument evaluation, the conceptual sequence is:

```text
CreateFirst
CreateSecond
Use
destroy remaining second temporary
destroy remaining first temporary
```

**§ 8(5)** A temporary forwarded into its first owner is not also destroyed by the forwarding context.

**§ 8(6)** Lifetime shortening of temporaries must not change observable destruction order, reference validity, defer behavior, volatile effects, or expression evaluation order.

---

## § 9 Move, return, and ownership transfer

### § 9.1 Move cancellation

**§ 9.1(1)** When ownership moves from a Place, the former owner's destruction responsibility for that moved value ends at successful transfer commitment.

**§ 9.1(2)** Moved-from storage is not destroyed by the former owner.

### § 9.2 Consuming calls

**§ 9.2(1)** A reusable source passed to a consuming `->` parameter must use the explicit call-site move marker required by `copy_move.md`.

```sec
Consume(<-resource)
```

**§ 9.2(2)** After successful transfer commitment, caller-side destruction responsibility for `resource` ends.

**§ 9.2(3)** Fallible argument preparation must not cancel source cleanup before the consuming transfer actually commits.

### § 9.3 Return transfer

**§ 9.3(1)** Returning an owned value transfers destruction responsibility to the caller/result owner.

```sec
return resource
```

**§ 9.3(2)** The optional return move marker does not change destruction semantics:

```sec
return <-resource
```

**§ 9.3(3)** The callee must not destroy a value after it has been transferred into the return result.

### § 9.4 Variant and aggregate transfer

**§ 9.4(1)** When an owned payload is explicitly moved into an owning aggregate or variant, the new container becomes responsible for that payload's eventual destruction.

**§ 9.4(2)** The source container or Place must not later destroy the transferred payload.

---

## § 10 Partial availability and partial destruction

### § 10.1 Partially available aggregates

**§ 10.1(1)** A `PartiallyAvailable` aggregate destroys exactly its still-owned `Available` sub-Places when its lifetime ends.

```sec
let package := LoadPackage()
Consume(<-package.Payload)
```

**§ 10.1(2)** At scope exit, `package.Payload` is not destroyed through `package` because ownership moved earlier.

**§ 10.1(3)** Other still-owned fields, such as `package.Header`, are destroyed normally if required.

**§ 10.1(4)** Partial destruction is recursive for nested Places where partial moves are permitted.

### § 10.2 Whole-value operations

**§ 10.2(1)** Whole-value destruction must respect the current per-sub-Place ownership state.

**§ 10.2(2)** A compiler must not invoke a whole-value destruction path that assumes complete ownership when the type's canonical rules require field-sensitive partial destruction instead.

### § 10.3 Custom `free`

**§ 10.3(1)** A type defining custom `free` does not permit partial moves from its owned fields in Sec 0.1.

**§ 10.3(2)** This restriction guarantees that a completed-value `free` body may rely on the complete valid ownership state promised by the type's lifecycle contract.

---

## § 11 Conditional availability and conditional destruction

### § 11.1 Conditional state

**§ 11.1(1)** A `ConditionallyAvailable` Place is available on some continuing control-flow paths and unavailable on others.

**§ 11.1(2)** Destruction must occur only on runtime paths where the current owner still owns the value.

**§ 11.1(3)** The compiler must resolve this statically whenever control-flow facts are sufficient.

### § 11.2 Runtime bookkeeping

**§ 11.2(1)** When static control-flow facts cannot fully resolve required conditional destruction, a hosted implementation may use hidden SSA state, a drop/availability flag, split cleanup blocks, or an equivalent representation.

**§ 11.2(2)** Such bookkeeping is compiler state. It does not automatically change the ABI layout of the source type.

**§ 11.2(3)** A target/project policy may forbid dynamic ownership bookkeeping.

**§ 11.2(4)** Under such a policy, the compiler must reject only code paths whose correct destruction semantics genuinely require forbidden dynamic state.

### § 11.3 `is available`

**§ 11.3(1)** `is available` and `is not available` refine ownership availability according to `ownership.md`.

**§ 11.3(2)** These tests do not test `null`, `None`, payload value, borrow validity, or generational-reference validity.

**§ 11.3(3)** Destruction lowering may reuse the same resolved availability facts needed by legal `is available` checks, but the language does not require a particular runtime representation.

---

## § 12 `discard` and destruction convergence

### § 12.1 Available Place

**§ 12.1(1)** Legal `discard place` of an `Available` owned value ends the value, runs required deterministic destruction, and leaves the Place `Unavailable` with discard provenance.

### § 12.2 Already unavailable Place

**§ 12.2(1)** `discard place` on an already unavailable Place is a legal no-op with respect to destruction.

**§ 12.2(2)** No second destruction occurs.

**§ 12.2(3)** Tooling may issue the redundancy advisory permitted by `discard.md`, but semantic validity is unchanged.

### § 12.3 Conditionally available Place

**§ 12.3(1)** `discard` of a `ConditionallyAvailable` Place destroys the value only on paths where the current owner still owns it and performs no destruction on paths where it is already unavailable.

**§ 12.3(2)** Every outgoing path is `Unavailable` after successful discard convergence.

```sec
let mut package := LoadPackage()

if condition {
    Consume(<-package.Payload)
}

discard package.Payload
package.Payload = CreateBuffer()
```

**§ 12.3(3)** After the discard, the assignment is ordinary reinitialization and requires no old-value destruction.

### § 12.4 Discardability

**§ 12.4(1)** Destruction semantics do not make a non-discardable value discardable.

**§ 12.4(2)** Discard legality is resolved before destruction convergence commits.

---

## § 13 Reinitialization and replacement

### § 13.1 Reinitialization

**§ 13.1(1)** Assignment to an unavailable mutable Place is reinitialization.

**§ 13.1(2)** Reinitialization performs no old-value destruction because no old owned value exists in that Place.

**§ 13.1(3)** Successful reinitialization establishes a new destruction responsibility at the reinitialization point.

### § 13.2 Replacement

**§ 13.2(1)** Assignment to an available mutable Place is replacement.

**§ 13.2(2)** Replacement ends/destroys the old destination value according to its canonical destruction plan before the new value becomes the destination's owned value.

**§ 13.2(3)** The compiler must validate the complete source expression, type/contract conversions, borrow constraints, overlap rules, and fallible preparation before destructive commitment to the destination.

**§ 13.2(4)** The compiler must not destroy the old destination and only afterward discover that the replacement operation was semantically invalid.

### § 13.3 Conditionally available replacement

**§ 13.3(1)** A mutable `ConditionallyAvailable` Place may be repaired by assignment.

**§ 13.3(2)** On a path where the old value is still owned, replacement destroys it. On a path where the Place is already unavailable, the operation is reinitialization.

**§ 13.3(3)** Hosted implementations may perform this conditional cleanup automatically.

**§ 13.3(4)** A target/project policy that forbids dynamic ownership bookkeeping may require explicit convergence first:

```sec
discard package.Payload
package.Payload = CreateBuffer()
```

### § 13.4 Immutable Places

**§ 13.4(1)** Destruction does not grant reinitialization authority. An immutable Place cannot be reinitialized or replaced by ordinary assignment.

---

## § 14 Construction and partial initialization

### § 14.1 Successful construction

**§ 14.1(1)** A successfully completed `init` establishes one fully valid instance and transfers all remaining construction cleanup responsibilities into the completed value.

### § 14.2 Fallible construction

**§ 14.2(1)** If construction fails, only successfully initialized owned fields/resources and construction temporaries are cleaned up.

**§ 14.2(2)** Cleanup follows reverse successful initialization/registration order.

**§ 14.2(3)** The completed-value custom `free` body does not run for an outer value that never completed construction.

**§ 14.2(4)** Every resource acquired during `init` must have a statically defined cleanup path on every failure path after acquisition and on successful lifetime completion.

```sec
type Resources struct {
    First: First
    Second: Second
    Third: Third
}
```

**§ 14.2(5)** If construction of `Second` fails after `First` succeeded, `First` is cleaned up, `Second` is not treated as initialized, and later fields are not evaluated.

### § 14.3 Partial field initialization

**§ 14.3(1)** When the language permits staged field initialization, the compiler must track destruction responsibility at sufficient field granularity to clean only initialized owned sub-values.

---

## § 15 Custom `free`

### § 15.1 When `free` is needed

**§ 15.1(1)** Most types should rely on compiler-derived destruction and define no custom `free`.

**§ 15.1(2)** A custom `free` is appropriate when the type owns a resource that cannot be represented and released correctly through ordinary owned field destruction.

**§ 15.1(3)** Typical examples include foreign resources held through non-owning raw representation, platform handles whose release is not represented by the field type, mappings, device registrations, or similar opaque ownership.

### § 15.2 Lifecycle status

**§ 15.2(1)** `free` is a lifecycle member, not an ordinary function/method.

**§ 15.2(2)** Ordinary user code must not call `value.free()` directly.

**§ 15.2(3)** The compiler invokes `free` as part of complete-value destruction when the type declares it and the completed value remains owned.

### § 15.3 `self` during `free`

**§ 15.3(1)** At `free` entry, the completed value remains initialized and is under exclusive compiler-controlled destruction authority.

**§ 15.3(2)** `self` must not escape, be returned, be resurrected, or be transferred as a complete value.

**§ 15.3(3)** Whole-`self` lifetime termination is the purpose of `free`; ordinary methods do not consume whole `self`.

**§ 15.3(4)** A type with custom `free` is not partially movable in Sec 0.1, so the `free` body may rely on the complete ownership invariant required by the type.

**§ 15.3(5)** The prohibition on ordinary whole-`self`-consuming methods does not prohibit a compiler-known terminal lifecycle operation on a semantic builtin when a canonical rulebook defines the operation and the compiler owns its complete ownership transition.

**§ 15.3(6)** After such an operation consumes the builtin owner, the source Place is unavailable and automatic destruction must recognize the consumed state so it does not repeat the terminal release.

**§ 15.3(7)** `Arena.Release()` is the Sec 0.1 instance of this rule. It terminates the ArenaDomain and suppresses a second automatic Release; its exact lifecycle semantics are defined by `rules/memory/arena.md`. This exception creates no general user-defined whole-`self`-consuming method facility.

### § 15.4 Owned fields during `free`

**§ 15.4(1)** In Sec 0.1, custom `free` should release resources not already represented by ordinary owned fields.

**§ 15.4(2)** Owned fields remain compiler-managed and are automatically destroyed after the `free` body unless another specialized rule explicitly says otherwise.

**§ 15.4(3)** Sec 0.1 does not permit moving owned fields out of `self` during `free`.

**§ 15.4(4)** The compiler must reject a statically provable manual release that would cause the same owned resource to be released again by automatic field destruction.

### § 15.5 Order

**§ 15.5(1)** Destruction of a completed custom-`free` type occurs conceptually as:

```text
run custom free body
destroy remaining initialized owned fields in canonical field order
release/deallocate representation-owned storage where required
```

**§ 15.5(2)** A custom `free` body does not suppress automatic destruction of ordinary owned fields.

### § 15.6 `defer` inside `free`

**§ 15.6(1)** `defer` is forbidden inside `free` in Sec 0.1.

**§ 15.6(2)** Locals and temporaries created inside `free` still follow ordinary automatic destruction rules.

### § 15.7 Fallibility

**§ 15.7(1)** `free` has no ordinary return value and must not return `Result`.

**§ 15.7(2)** Destruction cannot require a caller to recover from a cleanup failure after the value's ownership lifetime has already ended.

**§ 15.7(3)** A resource whose meaningful close operation can fail should expose an explicit fallible lifecycle method before automatic destruction when appropriate.

```sec
impl File {
    fn Close() Result[void, IOError] {
        return CloseHandleChecked(ref mut self.Handle)
    }

    free {
        CloseHandleBestEffort(self.Handle)
    }
}
```

**§ 15.7(4)** A successful explicit close must update the value's own lifecycle state so later automatic destruction does not release the same underlying resource twice.

**§ 15.7(5)** Ordinary methods may mutate lifecycle state or consume eligible members but may not consume whole `self`.

### § 15.8 Panic and unsafe

**§ 15.8(1)** `free` must not silently translate a recoverable cleanup failure into panic.

**§ 15.8(2)** `free` is not implicitly `unsafe`. Unsafe operations require the ordinary explicit unsafe context.

---

## § 16 Result, Option, unions, and consuming projections

### § 16.1 Result

**§ 16.1(1)** Destroying `Result[T, E]` destroys only the active retained payload.

**§ 16.1(2)** If the active payload has moved out, the Result owner must not destroy that payload again.

### § 16.2 Option

**§ 16.2(1)** Destroying `Option[T]` destroys the payload only when the active state is `Some` and the payload remains owned.

**§ 16.2(2)** `None` has no payload destruction.

### § 16.3 Tagged unions

**§ 16.3(1)** A tagged union destroys only its active initialized still-owned payload.

**§ 16.3(2)** Representation optimization must preserve active-variant destruction semantics.

### § 16.4 Consuming `.Ok()` and `.Err()`

**§ 16.4(1)** Consuming `Result.Ok()` retains/transfers the active `Ok` payload into the resulting `Option[T]` and ends the inactive `Err` payload according to ordinary destruction/discard rules.

**§ 16.4(2)** Consuming `Result.Err()` retains/transfers the active `Err` payload into the resulting `Option[E]` and ends the inactive `Ok` payload according to ordinary destruction/discard rules.

**§ 16.4(3)** The consumed Result owner must not later destroy a payload whose ownership has transferred through the projection.

### § 16.5 Untagged foreign unions

**§ 16.5(1)** The compiler must not guess which field of an untagged foreign union owns a resource.

**§ 16.5(2)** Safe ownership of non-trivial foreign union payloads requires an explicit wrapper/contract that identifies active ownership.

---

## § 17 Structs, arrays, collections, tensors, and strings

### § 17.1 Structs

**§ 17.1(1)** A struct without custom `free` derives destruction recursively from its still-owned initialized fields.

### § 17.2 Fixed arrays

**§ 17.2(1)** Fixed arrays destroy initialized still-owned elements in reverse index order.

### § 17.3 Owning dynamic arrays and lists

**§ 17.3(1)** An owning dynamic array/list destroys initialized elements according to its canonical order and then releases its backing allocation.

**§ 17.3(2)** A removed element whose ownership has transferred to a result is not destroyed by the collection.

### § 17.4 Shaped/tensor values

**§ 17.4(1)** An owning tensor/matrix/vector representation destroys still-owned initialized element/resources as required and releases owned backing storage through its canonical allocation authority.

**§ 17.4(2)** A borrowed tensor view or slice does not destroy backing elements or backing storage merely because the view's lifetime ends.

**§ 17.4(3)** Destruction cost may be significant for large owning shaped values, but performance significance is a tooling/analysis concern and does not change correctness semantics.

### § 17.5 Strings

**§ 17.5(1)** String destruction follows the canonical string representation.

**§ 17.5(2)** A static literal or borrowed view may be trivially destructible.

**§ 17.5(3)** An owned string representation releases its owned storage when required.

**§ 17.5(4)** Ordinary replacement such as assigning a new string value may automatically end the previous owned representation; the programmer is not required to write `discard` merely because cleanup exists.

---

## § 18 References, borrows, and non-owning views

**§ 18(1)** Destroying or discarding a reference value ends the holder value; it does not destroy the referent.

**§ 18(2)** Destroying or discarding a borrowed slice/view does not destroy the backing storage or backing elements.

**§ 18(3)** Destruction of an owned value must be rejected while an incompatible live borrow requires that value or overlapping Place to remain valid.

**§ 18(4)** A deferred read is a delayed use and may extend the owned value's lifetime until the defer executes.

**§ 18(5)** Generational-reference validity and ownership availability remain separate facts. Destruction must update any required invalidation domain according to the reference model, but this rulebook does not redefine generation semantics.

---

## § 19 Control-flow exits

### § 19.1 Return

**§ 19.1(1)** Before an ordinary function/method return completes, required cleanup for values not transferred into the return result executes according to the unified cleanup order.

**§ 19.1(2)** The return expression/result value is established before cleanup destroys its former local owners where required.

### § 19.2 `return try`

**§ 19.2(1)** `return try expression` forwards the successful result value or propagates failure according to error handling semantics.

**§ 19.2(2)** In either case, the transferred/propagated payload is excluded from destruction by the ownership context that transferred it.

**§ 19.2(3)** Other initialized still-owned locals are cleaned before control exits.

### § 19.3 Error propagation

**§ 19.3(1)** Unmatched `try` failure propagation executes required cleanup on the propagating path.

**§ 19.3(2)** A handler-local recovery value that continues execution establishes ordinary ownership/destruction responsibility for that recovery result.

**§ 19.3(3)** Failure originating inside a handler/guard is outside the protected outer `try` and follows its own cleanup path.

### § 19.4 Break and continue

**§ 19.4(1)** `break` executes cleanup for every scope/region actually exited by the break edge.

**§ 19.4(2)** `continue` executes cleanup for per-iteration values/regions exited before entering the next iteration.

**§ 19.4(3)** Loop-carried owned values that remain live must not be destroyed by iteration cleanup.

### § 19.5 Match and switch

**§ 19.5(1)** Each `match`/`switch` branch carries its own destruction state.

**§ 19.5(2)** Branch-local owned values are cleaned when leaving that branch unless ownership transfers.

**§ 19.5(3)** Join state follows the canonical ownership availability merge rules.

---

## § 20 Loops

**§ 20(1)** Loop destruction analysis must distinguish pre-loop owned values, per-iteration values, loop-carried values, conditionally initialized values, values moved during an iteration, and values retained after loop exit.

**§ 20(2)** A value local to one iteration is destroyed at the end of that iteration unless ownership transfers elsewhere.

**§ 20(3)** A Place moved in one iteration is unavailable in later iterations unless valid control flow reinitializes it before later use.

**§ 20(4)** Loop fixed-point analysis must preserve exact-once destruction across entry, backedge, continue, break, and zero-iteration paths.

**§ 20(5)** A compiler must reject a loop ownership state it cannot model safely rather than emit speculative cleanup.

---

## § 21 Allocation and deallocation

### § 21.1 Separation

**§ 21.1(1)** Destruction of a value and deallocation of its storage are distinct semantic actions.

**§ 21.1(2)** Stack storage disappears with its storage scope, but any non-trivial value in that storage must be semantically destroyed first.

### § 21.2 Allocator pairing

**§ 21.2(1)** Every explicit owning allocation must preserve enough semantic information to select the matching deallocation authority.

**§ 21.2(2)** Memory allocated by one allocator must not be deallocated through an incompatible allocator.

**§ 21.2(3)** Where allocator identity is required for correctness, the owning representation must preserve it until deallocation.

### § 21.3 Raw pointers

**§ 21.3(1)** A raw pointer alone never creates ownership or deallocation responsibility.

**§ 21.3(2)** The compiler must never infer that `RawPtr[T]` should be freed solely because its lifetime ends.

---

## § 22 FFI ownership and destruction

**§ 22(1)** Foreign ownership contracts must distinguish borrowed foreign pointers, caller-owned foreign resources, Sec-owned foreign resources, transfer to foreign code, transfer from foreign code, and resources requiring foreign release operations.

**§ 22(2)** When ownership legally transfers to foreign code, Sec's former owner becomes unavailable and must not destroy the transferred resource.

**§ 22(3)** Failure behavior at an ownership-transferring FFI call must define whether foreign code accepted ownership.

**§ 22(4)** Ambiguous foreign ownership transfer must be rejected or isolated behind an explicit unsafe wrapper with a defined ownership contract.

**§ 22(5)** An owning Sec wrapper around a foreign resource may define custom `free` to call the matching foreign release operation.

---

## § 23 Static and global lifetime destruction

**§ 23(1)** Static/global destruction is permitted only when the canonical program initialization/shutdown plan and target profile can provide deterministic cleanup semantics for that storage lifetime.

**§ 23(2)** A compiler must not assume process-exit cleanup merely because the target is hosted.

**§ 23(3)** A target that lacks supported deterministic static teardown must reject static/global owned values whose required destruction cannot otherwise be satisfied.

**§ 23(4)** Runtime-free/freestanding targets must not acquire an implicit global destruction registry solely to support static teardown.

**§ 23(5)** When deterministic static destruction is supported, order is derived from the canonical dependency/use/shutdown plan rather than arbitrary source, import, linker, map, filesystem, or worker order.

---

## § 24 Program termination and panic

### § 24.1 Normal termination

**§ 24.1(1)** Returning normally from `main` executes ordinary cleanup required by the canonical program shutdown plan.

### § 24.2 Forced termination

**§ 24.2(1)** A raw platform termination operation that is specified not to unwind/clean callers does not promise destruction after the termination point.

**§ 24.2(2)** Abnormal hardware or operating-system termination cannot guarantee pending cleanup.

### § 24.3 Panic

**§ 24.3(1)** Destruction semantics on panic paths are governed by the canonical panic/unwind rulebook.

**§ 24.3(2)** Until unwinding is explicitly supported for a path, the compiler must not assume that panic executes every pending cleanup action.

**§ 24.3(3)** Destruction must not silently introduce an exception-unwinding runtime.

---

## § 25 Concurrency, transfer, and destruction

**§ 25(1)** Destruction does not itself imply synchronization.

**§ 25(2)** Ownership transferred to another thread/task/execution owner must not be destroyed by the former owner.

**§ 25(3)** Thread/task transfer must be represented as ownership transfer before cleanup placement is finalized.

**§ 25(4)** Shared/ref-counted destruction, if introduced through a specialized explicit type, is not the default Sec ownership model.

**§ 25(5)** ISR-specific destruction restrictions, blocking rules, allocator availability, and device-lifetime rules are defined by ISR/platform rulebooks; this rulebook requires destruction plans to respect them.

---

## § 26 Volatile, registers, and physical hardware storage

**§ 26(1)** Ending the lifetime of a local register value or ordinary snapshot does not itself perform a volatile read or write.

**§ 26(2)** Destroying a volatile/MMIO accessor, typed register view, or shadow-bearing accessor performs no implicit hardware read/write, reset, clear, acknowledgement, commit, or shadow flush.

**§ 26(3)** A volatile read produces an ordinary Sec value snapshot according to `volatile.md`; later destruction of that local snapshot follows ordinary value semantics and does not repeat the hardware read.

**§ 26(4)** Physical hardware storage is not treated as an ordinary owned movable Sec value merely because its bits are addressable.

**§ 26(5)** `rules/platform/hardware-register-access.md` defines compiler-known runtime hardware mappings as move-only owned resources. Destroying the mapping owner may perform only the mapping contract's explicit platform release/unmap cleanup; destroying a borrowed register view does not release the mapping or perform a hardware transaction.

**§ 26(6)** Other target-side register operations, including explicit shadow commit, remain explicit and must not be inferred from ordinary scope exit.

---

## § 27 Interfaces and dynamic concrete values

**§ 27(1)** An owning interface value must destroy its owned concrete value through resolved type metadata, generated dispatch, or an equivalent statically validated mechanism.

**§ 27(2)** A borrowed interface view does not destroy the concrete value merely because the view ends.

**§ 27(3)** Interface destruction must not require a global garbage-collected object model.

**§ 27(4)** Lowering must preserve whether an interface representation owns or borrows its concrete payload.

---

## § 28 Generics

**§ 28(1)** Destruction classification of a generic instantiation is resolved from the concrete instantiated type arguments and representation.

```sec
type Box[T] struct {
    Value: T
}
```

**§ 28(2)** `Box[int]` and `Box[File]` may have different destruction plans.

**§ 28(3)** Monomorphization may generate specialized destruction functions or inline destruction plans.

**§ 28(4)** Generic lowering must not assume trivial destruction before instantiation has proven it.

---

## § 29 Recursive ownership and destruction

**§ 29(1)** Recursive owning types require a finite runtime ownership graph.

**§ 29(2)** Direct infinitely sized recursive values are rejected by layout/type rules.

**§ 29(3)** Indirect recursive owning structures may require recursive destruction.

**§ 29(4)** A target/profile may restrict destruction recursion when stack or realtime constraints require it.

**§ 29(5)** The compiler may transform recursive destruction into iteration when observable semantics are preserved.

---

## § 30 Semantic IR requirements

**§ 30(1)** Ownership-sensitive destruction decisions must be explicit in maintained Semantic IR before backend translation may erase source ownership structure.

**§ 30(2)** Semantic IR must be able to represent or encode, when required:

```text
DestroyValue
DestroyPlace
DestroySubPlace
DestroyActiveVariant
ConditionalDestroy
InvokeFree
DeallocateOwnedStorage
RegisterCleanup
ExecuteCleanup
CleanupRegion
CancelCleanupAfterMove
CancelCleanupAfterDiscard
Replace
Reinitialize
TransferToReturn
TransferToArgument
```

**§ 30(3)** Equivalent operation names are permitted if the same facts and verifier guarantees are preserved.

**§ 30(4)** Semantic IR must preserve source provenance sufficient to diagnose destruction origin, ownership transfer, partial state, custom `free`, and allocator authority.

**§ 30(5)** Every non-trivial owned value must have one valid terminal ownership action on every reachable path where it exists.

**§ 30(6)** Unsupported destruction-sensitive semantics must fail explicitly rather than be approximated by unused SSA or ordinary copy lowering.

---

## § 31 Sec MLIR and lowering requirements

**§ 31(1)** Sec MLIR/lowering must preserve exact-once destruction, cleanup order, partial availability, conditional destruction, construction failure cleanup, defer ordering, return/error transfer, allocator pairing, and custom `free` semantics.

**§ 31(2)** Ownership/destruction metadata must not be erased before cleanup control flow and terminal actions are fixed.

**§ 31(3)** Lowering may use direct calls, inline cleanup, cleanup blocks, loops, generated helpers, conditional flags, SSA control state, or target-specific release calls when source semantics permit them.

**§ 31(4)** A target policy forbidding dynamic ownership bookkeeping must be checked before lowering introduces hidden runtime ownership state.

**§ 31(5)** By LLVM translation, destruction placement must already be resolved into ordinary explicit control flow/calls/operations.

**§ 31(6)** LLVM IR must not decide whether a Sec value should be destroyed based on liveness or unused SSA alone.

---

## § 32 Optimization

**§ 32(1)** Permitted optimizations include removing trivial destruction, inlining cleanup, merging equivalent cleanup blocks, eliminating cleanup for never-initialized/moved values, scalarizing aggregate cleanup, eliminating allocation/deallocation pairs, and devirtualizing interface destruction when proven safe.

**§ 32(2)** Optimization must not change observable destruction order.

**§ 32(3)** Optimization must not skip required resource release, double destroy, destroy moved/discarded values, access unavailable sub-Places, reorder volatile accesses, suppress required foreign release, change explicit fallible close behavior, or introduce forbidden hidden allocation/runtime state.

**§ 32(4)** Copy/move elision must preserve the same terminal destruction owner as the source semantics.

---

## § 33 Diagnostics and mentor behavior

### § 33.1 Required explanation quality

**§ 33.1(1)** Destruction diagnostics must explain the programmer's ownership/lifecycle problem in ordinary language before relying on compiler-theory terminology.

**§ 33.1(2)** When applicable, a diagnostic should identify:

```text
which value or Place is affected
where ownership was established
where it moved, was discarded, or was replaced
where destruction would occur or be duplicated
which custom free or field is involved
which borrow/defer keeps the value alive
which allocator/release authority is required
what safe source-level change is available
```

### § 33.2 Representative diagnostics

**§ 33.2(1)** Use-after-destruction/move diagnostics must point to the earlier terminal action.

**§ 33.2(2)** Partial move from a custom-`free` type must explain that the type's cleanup requires the complete value and identify the `free` declaration.

**§ 33.2(3)** Conditional destruction that requires forbidden runtime ownership bookkeeping must explain why ownership differs across paths and suggest restructuring or explicit `discard` convergence.

**§ 33.2(4)** A statically redundant second `discard` is not a semantic error; tooling may issue only the configured redundancy advisory.

**§ 33.2(5)** Missing/incompatible allocator release diagnostics must identify the allocation origin and required deallocation authority when known.

### § 33.3 LSP

**§ 33.3(1)** LSP should surface destruction classification, cleanup owner, custom `free`, partial/conditional cleanup state, ownership-transfer origin, and pending deferred uses when available from Sema.

**§ 33.3(2)** Automatic code actions that insert `discard`, move cleanup, or restructure ownership must be offered only when semantic preservation is proven.

---

## § 34 Verification requirements

**§ 34(1)** A destruction verifier must prove, for all represented non-trivial owned values and relevant sub-Places:

```text
no value is destroyed more than once
no moved/discarded value is destroyed by its former owner
no uninitialized value/sub-Place is destroyed
all still-owned non-trivial values have a terminal action
partial aggregates destroy only still-owned sub-Places
conditional cleanup matches runtime ownership paths
return/argument/aggregate transfers cancel former-owner cleanup
construction failure cleans only successful initialization
custom free executes only for completed values
custom free runs at most once per completed value
remaining owned fields are destroyed after custom free
defer/automatic cleanup order matches registration order
allocator/deallocator pairing is valid
no unresolved destruction semantics reach LLVM lowering
```

**§ 34(2)** A verifier failure after valid source has passed Sema is an internal compiler error, not a source diagnostic.

---

## § 35 Required language/compiler tests

**§ 35(1)** The maintained test suite must cover at least the following semantics.

### § 35.1 Basic destruction

**§ 35.1(1)** Trivially destructible scalar produces no required cleanup operation.

**§ 35.1(2)** Non-trivial local is destroyed exactly once at its legal lifetime end.

**§ 35.1(3)** Locals destroy in reverse successful registration/initialization order when lifetimes otherwise overlap.

**§ 35.1(4)** Struct fields destroy in reverse declaration order.

**§ 35.1(5)** Fixed array elements destroy in reverse index order.

### § 35.2 Move and transfer

**§ 35.2(1)** Explicit move cancels former-owner destruction.

**§ 35.2(2)** Consuming `->` call transfer cancels caller destruction only after successful transfer commitment.

**§ 35.2(3)** `return value` and `return <-value` both transfer destruction responsibility to the result owner.

**§ 35.2(4)** Explicit owning aggregate/variant payload move transfers destruction responsibility to the container.

### § 35.3 Partial state

**§ 35.3(1)** Partially moved aggregate destroys only still-owned fields.

**§ 35.3(2)** Moved/discarded field is never destroyed twice.

**§ 35.3(3)** Custom-`free` type rejects partial field move.

### § 35.4 Conditional state

**§ 35.4(1)** Conditional move produces path-sensitive conditional destruction when paths rejoin.

**§ 35.4(2)** Static control-flow proof eliminates unnecessary runtime availability/drop state.

**§ 35.4(3)** Target policy forbidding dynamic ownership bookkeeping rejects only cases that genuinely require it.

**§ 35.4(4)** `is available` ownership refinement and conditional destruction share compatible state without becoming `None`/null semantics.

### § 35.5 Discard and replacement

**§ 35.5(1)** Discard of available non-trivial value destroys once and converges to unavailable.

**§ 35.5(2)** Second discard of already unavailable Place is legal no-op and performs no destruction.

**§ 35.5(3)** Discard of conditionally available Place destroys only still-owned runtime paths and converges all outgoing paths to unavailable.

**§ 35.5(4)** Reinitialization of unavailable mutable Place performs no old destruction and registers new cleanup.

**§ 35.5(5)** Replacement of available mutable Place destroys old value exactly once before new destination ownership commits.

**§ 35.5(6)** Conditional replacement destroys old value only on paths where it is still owned.

### § 35.6 Defer

**§ 35.6(1)** Create first, register defer, create second cleans in order: second, defer, first.

**§ 35.6(2)** Defer-referenced value remains alive until deferred use.

**§ 35.6(3)** Unrelated value may still destroy at earliest legal lifetime end.

**§ 35.6(4)** `defer` inside `free` is rejected.

### § 35.7 Construction

**§ 35.7(1)** Fallible construction cleans only already initialized fields/resources.

**§ 35.7(2)** Completed-value `free` does not run for failed partial construction.

**§ 35.7(3)** Successful construction transfers field cleanup into completed-value destruction responsibility.

### § 35.8 `free`

**§ 35.8(1)** Direct `value.free()` call is rejected.

**§ 35.8(2)** `free` cannot return a value or `Result`.

**§ 35.8(3)** Whole `self` cannot escape or be moved from `free`.

**§ 35.8(4)** Owned field move out of `free` is rejected in Sec 0.1.

**§ 35.8(5)** After custom `free`, remaining owned fields destroy exactly once in canonical order.

**§ 35.8(6)** Unsafe operation inside `free` requires ordinary unsafe authorization.

### § 35.9 Result/Option/error handling

**§ 35.9(1)** Result destroys only active retained payload.

**§ 35.9(2)** Option destroys `Some` payload only.

**§ 35.9(3)** Consuming `.Ok()` / `.Err()` transfer retained payload and destroy/end inactive side correctly.

**§ 35.9(4)** Bodyless `try` propagation cleans other still-owned locals.

**§ 35.9(5)** Partial `try` handler recovery and unmatched propagation each retain correct destruction responsibility.

**§ 35.9(6)** `return try` transfers success/failure payload before local cleanup.

### § 35.10 Loops/control flow

**§ 35.10(1)** `break` and `continue` execute only cleanup for regions they exit.

**§ 35.10(2)** Per-iteration values destroy once per reached iteration.

**§ 35.10(3)** Loop-carried value is not destroyed on continue if still required.

**§ 35.10(4)** Zero-iteration path does not destroy never-created iteration locals.

### § 35.11 FFI/allocation

**§ 35.11(1)** Raw pointer lifetime end does not infer foreign free.

**§ 35.11(2)** Owning foreign wrapper calls matching foreign release exactly once.

**§ 35.11(3)** Transfer to foreign code cancels Sec destruction only when ownership acceptance semantics say transfer committed.

**§ 35.11(4)** Mismatched allocator/deallocator is rejected when provable.

### § 35.12 Volatile/register

**§ 35.12(1)** Destruction of local volatile snapshot performs no extra volatile read/write.

**§ 35.12(2)** Scope exit never infers register clear/device reset from physical storage lifetime.

### § 35.13 IR/lowering

**§ 35.13(1)** Semantic IR records explicit terminal ownership/destruction actions where needed.

**§ 35.13(2)** Sec MLIR preserves partial and conditional cleanup.

**§ 35.13(3)** LLVM receives already-resolved cleanup control flow and never infers Sec destruction from unused SSA.

**§ 35.13(4)** Verifier catches deliberately mutated double-destroy and missing-destroy IR tests.

---

## § 36 Source/tooling conventions

**§ 36(1)** Source code uses `free` only as the lifecycle declaration keyword inside `impl`; internal compiler operations may use names such as `destroy` or `drop` without creating additional source keywords.

**§ 36(2)** Formatter output must preserve ownership-significant `discard`, move, defer, and lifecycle syntax without adding/removing semantic operations merely for style.

**§ 36(3)** Code comments in compiler implementation should reference this rulebook and stable clauses where practical, for example:

```text
rules/memory/destruction.md § 12.3(1)
rules/memory/destruction.md § 15.5(1)
```

---

## § 37 Cross-rulebook boundaries

**§ 37(1)** This rulebook owns:

```text
destruction classification
automatic destruction responsibility
exact-once destruction
field/local/element destruction order
partial and conditional destruction consequences
replacement/reinitialization cleanup consequences
custom free destruction semantics
construction-failure cleanup
deallocation as part of owning destruction
cleanup consequences of ownership transfer
Semantic IR/lowering destruction requirements
```

**§ 37(2)** Adjacent rulebooks own:

```text
ownership.md           Place ownership, Availability, reasons, is available
copy_move.md           Copy/move classification and explicit move syntax
borrowing.md           Borrow validity and overlap
references.md          Reference provenance/generation rules
discard.md             Discard syntax, discardability, must-use policy
defer.md               Defer syntax/registration/invocation behavior
impl.md                init/free declaration syntax and lifecycle member identity
errorhandling.md       Result/error/try semantics
volatile.md            Volatile storage/access semantics
allocation rules       Allocation selection and placement
initialization rules   Program/static initialization and shutdown planning
platform/ISR rules     Execution-context and hardware-specific restrictions
```

**§ 37(3)** If an older rulebook conflicts with a later locked ownership/destruction decision reflected here, this revision-2 rulebook and the newer specialized revision-2 rulebooks are authoritative for Sec 0.1.

---

## § 38 Revision 2.0 semantic delta

**§ 38(1)** Revision 2.0 consolidates and updates the pre-v2 destruction rulebook without reintroducing implementation-status material into the normative document.

**§ 38(2)** Major revision-2 deltas include:

```text
Availability and UnavailableReason are separate ownership dimensions.
PartiallyAvailable aggregates destroy only still-owned Available sub-Places.
ConditionallyAvailable Places support path-sensitive conditional destruction.
is available is an ownership-state test, not null/Option semantics.
discard converges to Unavailable and is a legal no-op when already unavailable.
discard of ConditionallyAvailable destroys only still-owned paths.
Hosted replacement of conditionally available values may perform automatic conditional cleanup.
Strict target/project policy may require explicit discard convergence rather than hidden runtime ownership state.
A type with custom free forbids partial moves in Sec 0.1.
Ordinary methods may not consume whole self; complete lifetime termination belongs to free.
defer and automatic destruction share one source-ordered LIFO cleanup model.
return and return try transfer payload ownership before local cleanup.
Result.Ok()/Err() consuming projections participate in exact-once destruction.
Static/global destruction follows the canonical target shutdown plan rather than a blanket hosted/freestanding prohibition.
Volatile/MMIO storage lifetime does not imply hardware cleanup side effects.
Semantic IR and Sec MLIR must carry resolved destruction semantics rather than letting LLVM infer them.
```

**§ 38(3)** Implementation status, known gaps, migration actions, and required verification commands belong in `implementation-status-destruction.yaml`, not in this normative rulebook.

## § 39 Test-invocation termination

**§ 39(1)** Controlled test termination through `testing.Pass`, `testing.Fail`, `testing.Skip`, failed `testing.Require`/`RequireEqual`, or unexpected `Err` propagation must destroy every still-owned value in the current invocation according to the ordinary cleanup plan.

**§ 39(2)** A test outcome transition does not waive exact-once destruction, skip registered `defer`, or convert owned values into leaked test-runner state.

**§ 39(3)** Each subtest is a separate invocation cleanup boundary. The parent resumes only after child cleanup completes. Source-level testing semantics are owned by `rules/tooling/testing.md`.
