# Copy and Move

- **Status:** Normative
- **Created:** 2026-08-12
- **Last updated:** 2026-08-28
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/memory/copy_move.md`
- **Replaces:** pre-v2 `rules/memory/copy_move.md` and legacy `rules/memory/copy_move.txt`
- **Repository baseline reviewed:** `069c111`

---

## § 1 Purpose and authority

**§ 1(1)** This rulebook defines Sec source-language copy and move semantics.

**§ 1(2)** Copy duplicates a value while preserving the source. Move transfers a value or ownership responsibility and makes the consumed source Place unavailable.

**§ 1(3)** Copy and move semantics are determined before lowering. A backend optimization may remove physical copies or physical moves, but it must not change the source-language ownership result.

**§ 1(4)** `ownership.md` owns Place identity, availability, partial availability, ownership state, reinitialization authority, and ownership-state refinement. This rulebook owns copy classification, copy operations, move operations, explicit move syntax, transfer contexts, and copy/move conformance requirements.

**§ 1(5)** `borrowing.md` owns borrow validity. `destruction.md` owns destruction and cleanup. `volatile.md` owns volatile storage access. Where those books impose a stricter requirement, copy or move must respect that requirement.

---

## § 2 Core semantic operations

**§ 2(1)** The compiler must distinguish at least the following semantic actions when relevant:

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

**§ 2(2)** Two distinct semantic actions may lower to identical machine instructions. Representation equality never makes Copy and Move semantically interchangeable.

**§ 2(3)** A move does not assign a default value, zero value, `None`, `null`, or any other source-level value to the source Place. The source becomes unavailable.

**§ 2(4)** Physical clearing of moved-from storage for security or target reasons is not a source-level reinitialization.

---

## § 3 Copy classification

### § 3.1 Required classification

**§ 3.1(1)** Every resolved Sec type must have a copy classification before ordinary copy syntax is validated.

**§ 3.1(2)** The canonical conceptual classifications are:

```text
TriviallyCopyable
SemanticallyCopyable
ConditionallyCopyable
MoveOnly
ExplicitlyNonCopyable
```

**§ 3.1(3)** An implementation may use different internal names if the observable semantics are identical.

### § 3.2 Trivially copyable

**§ 3.2(1)** A trivially copyable value may be duplicated without invoking type-defined behavior, allocation, failure, ownership transfer, or observable side effects.

**§ 3.2(2)** Typical trivially copyable values include scalar primitives, fieldless enums, `RawPtr[T]`, shared references where permitted by reference rules, and aggregates whose complete resolved semantics permit trivial copying.

**§ 3.2(3)** Trivial copy classification is semantic. A backend may still lower a large trivial copy through `memcpy`, vector operations, registers, or target-specific instructions.

### § 3.3 Semantically copyable

**§ 3.3(1)** A semantically copyable value may be copied implicitly only when the language-defined copy is infallible, allocation-free, and free from observable side effects.

**§ 3.3(2)** Sec 0.1 does not recognize arbitrary user methods named `Copy`, `Clone`, `Duplicate`, `Snapshot`, `ToOwned`, or similar as implicit copy implementations.

**§ 3.3(3)** A named duplication method remains an ordinary explicit operation and may allocate, fail, duplicate an external resource, or return a different type without changing the source type's implicit-copy classification.

### § 3.4 Conditionally copyable

**§ 3.4(1)** A generic or aggregate type may be conditionally copyable when copyability depends on resolved type arguments, payloads, fields, captures, or other statically known constituents.

**§ 3.4(2)** Conditional copyability must be resolved at the concrete use site before an implicit copy is accepted.

### § 3.5 Move-only and explicitly non-copyable

**§ 3.5(1)** A move-only value may be transferred but not implicitly copied.

**§ 3.5(2)** `@noCopy` means exactly that implicit copying is forbidden for the annotated nominal type and every classification that must conservatively contain it.

**§ 3.5(3)** `@noCopy` does not forbid moving.

**§ 3.5(4)** Sec 0.1 defines no general `@noMove` attribute and no `@affine` source annotation.

**§ 3.5(5)** Address stability, pinning, fixed storage, non-relocation, thread transferability, and copyability are separate properties. A value may be movable between owners while its physical storage is required to remain stable.

---

## § 4 Derived copyability

### § 4.1 Arrays and fixed aggregates

**§ 4.1(1)** A fixed array is implicitly copyable only when its element type is implicitly copyable and the array type imposes no stronger ownership restriction.

**§ 4.1(2)** A struct containing any move-only or explicitly non-copyable owned field is not implicitly copyable.

**§ 4.1(3)** A nominal type may be non-copyable even when every physical field would otherwise be copyable, for example when the type owns a unique external identity.

### § 4.2 Enums and unions

**§ 4.2(1)** Fieldless enums are trivially copyable unless a separate rule gives the nominal type stronger semantics.

**§ 4.2(2)** A payload-bearing union derives copyability from every reachable active payload state required by the union contract.

**§ 4.2(3)** Copying a union copies only the active semantic value, but compile-time classification must guarantee that every permitted active state can be copied safely or that the active state is otherwise proven.

### § 4.3 Result and Option

**§ 4.3(1)** `Option[T]` is implicitly copyable only when its payload semantics permit copying `T`.

**§ 4.3(2)** `Result[T, E]` is implicitly copyable only when both possible active payload domains are implicitly copyable.

**§ 4.3(3)** Must-use classification is independent of copyability. A `Result[T, E]` may be copyable and still require explicit handling.

### § 4.4 References and raw pointers

**§ 4.4(1)** A shared `ref T` may be copyable according to borrowing and lifetime rules. Copying it copies the non-owning reference value, not the referent.

**§ 4.4(2)** `ref mut T` is move-only unless a separate reborrow operation is used. Copying a mutable reference would violate exclusivity.

**§ 4.4(3)** `RawPtr[T]` is trivially copyable as an address value. Copying or moving a raw pointer does not imply ownership of pointed storage.

### § 4.5 `string`

**§ 4.5(1)** In Sec 0.1, `string` is immutable and implicitly copyable under the canonical representation contract.

**§ 4.5(2)** Implicit string copy must remain infallible, allocation-free, side-effect-free, and free from mutable aliasing.

**§ 4.5(3)** The implementation may satisfy § 4.5(2) through descriptor copy, immutable backing storage, compiler-proven backing lifetime, arena-backed immutable storage, or another equivalent representation.

**§ 4.5(4)** The copyability of `string` does not require hidden reference counting.

### § 4.6 Dynamic owning collections and shaped values

**§ 4.6(1)** An owning dynamic collection is move-only unless its canonical type contract defines an infallible allocation-free independent copy representation.

**§ 4.6(2)** Explicit duplication of an owning collection is a named operation and does not make ordinary assignment an implicit copy.

**§ 4.6(3)** Non-owning slices and views follow reference/view rules rather than owning-container copy rules.

**§ 4.6(4)** A large copy may remain legal while also qualifying for analysis advice under § 22.

### § 4.7 Closures and function values

**§ 4.7(1)** A plain named function value is copyable.

**§ 4.7(2)** A closure derives copyability from its capture modes and callable capability.

**§ 4.7(3)** A closure owning a move-only capture is move-only.

**§ 4.7(4)** A closure capturing `ref mut` is move-only unless the borrowing rulebook defines a safe reborrow operation instead of copy.

**§ 4.7(5)** A consuming `-> fn` callable value is move-only even when its stored captures would otherwise be copyable.

---

## § 5 Ordinary copy syntax

### § 5.1 Initialization

**§ 5.1(1)** Ordinary initialization from an existing reusable Place uses copy semantics:

```sec
let destination := source
```

**§ 5.1(2)** The form in § 5.1(1) is valid only when `source` is implicitly copyable in that context.

**§ 5.1(3)** The compiler must never reinterpret ordinary `:=` as a destructive move because the source type happens to be move-only.

**§ 5.1(4)** If the copy is invalid, the diagnostic should explain that the source cannot be copied and show an explicit move form when that is a safe likely fix.

### § 5.2 Assignment

**§ 5.2(1)** Ordinary assignment from an existing reusable Place uses copy/replacement semantics:

```sec
destination = source
```

**§ 5.2(2)** The source remains available after a valid ordinary copy assignment.

**§ 5.2(3)** Assignment to an already available destination replaces the old destination value according to destruction rules.

**§ 5.2(4)** Assignment to an unavailable mutable destination reinitializes it.

### § 5.3 Copy of a copyable field or element

**§ 5.3(1)** Reading a copyable field by value copies it:

```sec
let name := person.Name
```

**§ 5.3(2)** Ordinary indexing of a copyable element copies that element unless the collection rulebook defines a reference/view result.

**§ 5.3(3)** Ordinary indexing must not silently move a move-only element out of a collection.

---

## § 6 Explicit move syntax

### § 6.1 Move initialization

**§ 6.1(1)** Move initialization with inferred type uses:

```sec
let destination :<- source
```

**§ 6.1(2)** Move initialization with an explicitly typed destination uses the grammar-defined typed move form:

```sec
let destination: Buffer <- source
```

**§ 6.1(3)** After a successful move from a reusable source Place, the source becomes unavailable according to `ownership.md`.

### § 6.2 Move assignment

**§ 6.2(1)** Move assignment or move replacement uses:

```sec
destination <- source
```

**§ 6.2(2)** An available destination ends its old value before the new value becomes owned by the destination, subject to commit ordering in § 12.

**§ 6.2(3)** An unavailable mutable destination is reinitialized by the move.

### § 6.3 Moving copyable values

**§ 6.3(1)** Explicit move syntax may be used with a copyable value.

**§ 6.3(2)** When explicit move syntax is used, the source becomes unavailable even if an ordinary copy would also have been legal.

**§ 6.3(3)** The compiler must not silently reinterpret an explicit move as a copy merely because the type is copyable.

### § 6.4 No implicit reset

**§ 6.4(1)** Moving from a source does not write a default value back to that source.

**§ 6.4(2)** A mutable unavailable source may later be explicitly reinitialized through ordinary assignment.

---

## § 7 The reusable-source visibility rule

**§ 7(1)** A reusable named or projected Place must never be silently consumed by ordinary value syntax.

**§ 7(2)** When ownership leaves such a Place and execution may continue in the source scope, the consuming source must be visibly marked with `<-`, unless another source-language construct defined by this rulebook is intrinsically terminal.

**§ 7(3)** The rule in § 7(2) applies regardless of whether the source type is copyable or move-only when the programmer explicitly chooses consumption.

**§ 7(4)** A fresh temporary has no reusable source Place to preserve and therefore does not require `<-` merely to forward it into its first owner.

**§ 7(5)** A returned fresh value received by the caller similarly requires no move marker at the receiving declaration.

---

## § 8 Function arguments

### § 8.1 Ordinary by-value parameters

**§ 8.1(1)** An ordinary by-value parameter does not become consuming merely because the argument type is move-only.

```sec
fn Inspect(value: Buffer) void
```

**§ 8.1(2)** Passing an existing reusable argument to an ordinary by-value parameter requires an implicit copy. If the argument cannot be copied, the call is invalid.

**§ 8.1(3)** The compiler must not upgrade § 8.1(1) into ownership transfer based solely on the type.

### § 8.2 Consuming parameters

**§ 8.2(1)** A parameter declared with `->` explicitly takes ownership:

```sec
fn Consume(-> value: Buffer) void
```

**§ 8.2(2)** A call that transfers ownership from an existing reusable source Place to a `->` parameter must use `<-` at the call site:

```sec
Consume(<-resource)
```

**§ 8.2(3)** This is invalid:

```sec
Consume(resource)
```

when `resource` is an existing reusable Place and the parameter is consuming.

**§ 8.2(4)** The call-site marker is required even when the source type is copyable. `->` is an API ownership contract, not a fallback for move-only values.

**§ 8.2(5)** A fresh temporary may be passed directly to a consuming parameter without synthetic move boilerplate:

```sec
Consume(CreateBuffer())
```

### § 8.3 Fallible argument evaluation and commit

**§ 8.3(1)** Arguments evaluate left-to-right.

**§ 8.3(2)** Transfer from prepared caller-owned reusable Places into the outer callee is committed only after every argument required for call entry has evaluated successfully and the call is ready to enter the callee.

**§ 8.3(3)** If a later argument fails before outer call entry, an earlier caller binding reserved for that outer call must remain owned by the caller.

**§ 8.3(4)** Ownership effects performed inside evaluation of an earlier argument expression are not rolled back.

---

## § 9 Return boundaries

### § 9.1 Return is intrinsically transferring

**§ 9.1(1)** Every non-reference function return creates or transfers one result value owned by the caller.

**§ 9.1(2)** The canonical concise form is:

```sec
return value
```

**§ 9.1(3)** `return value` transfers an owned local even when that local is move-only.

**§ 9.1(4)** The move marker is permitted but not required at the return boundary:

```sec
return <-value
```

**§ 9.1(5)** The optional marker in § 9.1(4) changes no ownership semantics. It is explicit documentation only.

### § 9.2 Terminal return context through construction

**§ 9.2(1)** The terminal return context propagates recursively through structural owning construction that directly forms the returned value.

This includes aggregate fields, union payloads, `Option.Some`, `Result.Ok`, `Result.Err`, and nested combinations of those forms:

```sec
return Some(resource)
return Packet.Data(resource)
return Response { Body: resource }
return Ok(Response { Body: resource })
```

**§ 9.2(2)** A reusable move-only source on such a direct return-construction path may transfer without an inner `<-`. An explicit `<-` remains permitted as documentation and has identical ownership semantics.

**§ 9.2(3)** A value-producing control-flow expression that directly supplies the return value propagates the terminal context into each continuing value-producing arm.

**§ 9.2(4)** An ordinary function call is not structural return forwarding. Its arguments retain their declared copy, borrow, and consuming parameter modes even when the call result is returned.

**§ 9.2(5)** Terminal forwarding does not permit move-out from borrowed, indexed, static, foreign, volatile, MMIO, unavailable, or otherwise restricted storage.

### § 9.3 Returned temporaries and caller reception

**§ 9.3(1)** A temporary may construct or forward directly into the return result:

```sec
return CreateBuffer()
```

**§ 9.3(2)** Receiving a fresh returned value requires no move marker:

```sec
let buffer := CreateBuffer()
```

**§ 9.3(3)** The language must not require move syntax at both ends of an already-obvious return transfer.

### § 9.4 Returning from borrowed storage

**§ 9.4(1)** A borrowed Place does not grant ownership transfer.

**§ 9.4(2)** Returning a field by value from borrowed storage is valid only if that field can be copied under ordinary copy rules.

**§ 9.4(3)** A move-only value cannot be removed from shared, borrowed, static, or foreign storage without a separate ownership-taking contract.

---

## § 10 Construction and payload transfer

### § 10.1 Aggregate construction

**§ 10.1(1)** Ordinary aggregate field syntax copies a reusable named source when the source is copyable.

**§ 10.1(2)** A reusable named or projected move-only source must use explicit `<-` when transferred into an owning aggregate field:

```sec
let session := Session {
    Name: name,
    File: <-file,
}
```

**§ 10.1(3)** The compiler must not infer a destructive move from plain field syntax merely because the source is move-only.

**§ 10.1(4)** A fresh temporary may construct an owning field directly without `<-`.

**§ 10.1(5)** When the aggregate directly forms a returned value, § 9.2 makes an inner move marker optional and propagates through nested aggregate construction.

### § 10.2 Option construction

**§ 10.2(1)** A reusable move-only source transferred into `Some` must be explicit:

```sec
let value := Some(<-resource)
```

**§ 10.2(2)** Plain `Some(resource)` copies when `resource` is copyable and is invalid when consuming ownership would be required.

**§ 10.2(3)** When `Some` directly forms a returned value, § 9.2 permits terminal forwarding without an inner move marker.

### § 10.3 Result construction

**§ 10.3(1)** Outside a terminal return boundary, a reusable move-only source transferred into `Ok` or `Err` must use explicit `<-`.

```sec
let result := Ok(<-resource)
```

**§ 10.3(2)** When `Ok` or `Err` directly forms a returned value, the general terminal-construction rule in § 9.2 makes the inner payload move marker optional:

```sec
return Ok(resource)
```

**§ 10.3(3)** The explicit form remains allowed:

```sec
return Ok(<-resource)
```

**§ 10.3(4)** § 9.2 is a return-boundary exception. It must not be generalized into implicit destructive payload transfer for non-terminal construction.

### § 10.4 Union construction

**§ 10.4(1)** A union variant owns its active owning payload according to the union rulebook.

**§ 10.4(2)** A reusable named move-only source transferred into an owning union payload must use `<-` at the source position.

**§ 10.4(3)** Moving a complete union transfers ownership of its active payload without exposing representation details.

**§ 10.4(4)** When a union variant directly forms a returned value, § 9.2 permits terminal payload forwarding without an inner move marker.

### § 10.5 Conversion expressions

**§ 10.5(1)** Every conversion that can affect ownership must have a defined ownership classification.

**§ 10.5(2)** A conversion must not silently consume a reusable named Place unless its source syntax visibly expresses consumption or the enclosing operation is an intrinsic terminal transfer under this rulebook.

---

## § 11 Closure capture

**§ 11(1)** Plain owned capture is copy capture:

```sec
capture(value)
```

**§ 11(2)** `capture(value)` requires the reusable source to be copyable. It must not silently move a move-only source.

**§ 11(3)** Consuming owned capture uses:

```sec
capture(<-value)
```

**§ 11(4)** `capture(<-value)` consumes the source even when the source type is otherwise copyable.

**§ 11(5)** Borrow captures remain:

```sec
capture(ref value)
capture(ref mut value)
```

and follow borrowing/lifetime rules.

---

## § 12 Replacement, reinitialization, and commit ordering

### § 12.1 Replacement of an available destination

**§ 12.1(1)** Assignment or move-assignment into an available mutable destination replaces its current value.

**§ 12.1(2)** The complete source expression, conversion, contract checks, borrow checks, and fallible operations must be validated before destructive commitment to the destination.

**§ 12.1(3)** Conceptually, replacement preserves this order:

```text
1. evaluate the source expression;
2. validate conversions, contracts, borrows, overlap, and ownership;
3. complete fallible preparation;
4. end or destroy the old destination value;
5. install the new value;
6. establish destination ownership;
7. commit source unavailability when a move occurred.
```

**§ 12.1(4)** The backend may fuse or reorder machine instructions only when the observable semantics of § 12.1(3) remain unchanged.

### § 12.2 Reinitialization of an unavailable destination

**§ 12.2(1)** Assignment to an unavailable mutable Place reinitializes it without destroying a nonexistent old value.

**§ 12.2(2)** Reinitialization restores availability according to `ownership.md`.

**§ 12.2(3)** An immutable unavailable Place cannot be reinitialized by assignment.

### § 12.3 Conditionally available destinations

**§ 12.3(1)** A mutable `ConditionallyAvailable` Place may be assigned a new value and becomes `Available` after successful assignment.

**§ 12.3(2)** On a runtime path where the old value is still owned, replacement must end that old value. On a path where it is already unavailable, the operation is reinitialization.

**§ 12.3(3)** Hosted implementations may perform the distinction in § 12.3(2) automatically when required.

**§ 12.3(4)** A target/project policy that forbids dynamic ownership bookkeeping may require ownership convergence, normally through `discard`, before such replacement.

---

## § 13 Self-move and overlap

**§ 13(1)** Copy self-assignment may be accepted for copyable types and optimized to a no-op.

**§ 13(2)** Move self-assignment is invalid:

```sec
value <- value
```

**§ 13(3)** A move must be rejected when source and destination overlap in a way that invalidates the transfer or destruction order.

**§ 13(4)** Whether machine lowering chooses `memcpy` or `memmove` never determines Sec copy/move semantics.

---

## § 14 Partial moves

**§ 14(1)** Partial move semantics are governed jointly by this rulebook and `ownership.md`.

**§ 14(2)** A partial move must be explicit at the moved sub-Place:

```sec
let payload :<- package.Payload
```

**§ 14(3)** A partial move may be permitted only when the source aggregate is owned, the projected Place can be tracked precisely enough, no conflicting borrow exists, and no stronger storage/destruction rule forbids it.

**§ 14(4)** A type with custom `free` does not permit partial moves from its fields in Sec 0.1.

**§ 14(5)** An immutable binding may undergo an explicit partial move when the addressed sub-Place is otherwise movable. Immutability prevents later assignment/reinitialization; it does not by itself forbid ownership transfer.

**§ 14(6)** Whole-value operations require the whole aggregate to be `Available`. Operations on an available sub-Place require only that addressed sub-Place to be available, subject to borrowing and access rules.

**§ 14(7)** Partial destruction must never destroy a sub-Place whose ownership has already moved elsewhere.

---

## § 15 Control flow and move state

### § 15.1 Branches

**§ 15.1(1)** Copy/move state is path-sensitive.

**§ 15.1(2)** A branch that does not continue to a join does not constrain availability after that join.

**§ 15.1(3)** When continuing paths disagree about whether a Place still owns a value, the ownership rulebook may classify the Place as `ConditionallyAvailable`.

### § 15.2 Loops

**§ 15.2(1)** Loop ownership requires fixed-point analysis.

**§ 15.2(2)** A Place moved on one iteration must be reinitialized on every path that attempts to use or move it on a later iteration.

**§ 15.2(3)** `break`, `continue`, and loop backedges must preserve separate ownership facts before merge.

### § 15.3 Match and switch

**§ 15.3(1)** `match` pattern syntax determines whether a payload is copied, moved, or borrowed according to the match rulebook.

**§ 15.3(2)** Whole-payload by-value match binding may move a move-only payload because the pattern construct itself defines that ownership mode. This is not ordinary implicit construction or call-site consumption.

**§ 15.3(3)** `switch` must not silently consume its subject unless switch syntax explicitly defines consumption.

---

## § 16 `discard` and copy/move

**§ 16(1)** `discard` is not Move. It is a terminal ownership-ending operation on the specified value or Place according to `discard.md` and `ownership.md`.

**§ 16(2)** `discard place` converges the Place to `Unavailable`.

**§ 16(3)** If the Place is already unavailable, `discard place` is a legal no-op with respect to destruction.

**§ 16(4)** If the Place is `ConditionallyAvailable`, `discard` destroys only on paths where the current owner still owns the value and leaves all outgoing paths unavailable.

**§ 16(5)** Discardability restrictions remain independent of copyability.

---

## § 17 Result projections and consuming transformations

**§ 17(1)** Compiler/core-defined `Result[T, E].Ok()` is a consuming transformation of an owned Result and produces `Option[T]`.

**§ 17(2)** Compiler/core-defined `Result[T, E].Err()` is a consuming transformation of an owned Result and produces `Option[E]`.

**§ 17(3)** The active retained payload transfers into the resulting `Option`; the inactive side is ended according to destruction/discard rules.

**§ 17(4)** No hidden clone is permitted for move-only payloads.

**§ 17(5)** After `.Ok()` or `.Err()` consumes an owned reusable Result Place, later use of that complete Result Place is invalid unless it is legally reinitialized.

---

## § 18 Methods and `self`

**§ 18(1)** An ordinary method must not consume the complete `self` value.

**§ 18(2)** Whole-instance lifetime termination belongs to `free` and destruction semantics.

**§ 18(3)** A method with sufficient mutable/exclusive receiver authority may move, discard, or replace an owned member of `self` when partial-move and borrow rules permit it.

```sec
impl Package {
    mut fn ReleasePayload() void {
        Destroy(<-self.Payload)
    }
}
```

**§ 18(4)** No syntax such as `(<-resource).Method()` is introduced for ordinary methods in Sec 0.1.

---

## § 19 Volatile and hardware storage

### § 19.1 Volatile is storage semantics

**§ 19.1(1)** `volatile` applies to access to storage. It is not a copy/move property of the resulting Sec value.

**§ 19.1(2)** Reading volatile storage performs the required volatile access and produces an ordinary Sec value snapshot.

**§ 19.1(3)** Copying or moving that local snapshot does not itself cause another volatile access.

**§ 19.1(4)** Two separate source-level reads from volatile storage are two separate observable accesses and must not be merged merely because the resulting values would compare equal.

### § 19.2 Volatile storage is not a movable owner

**§ 19.2(1)** The physical volatile/MMIO storage Place is not an ordinary Sec-owned reusable value from which ownership can be moved out with `:<-` or `<-`.

**§ 19.2(2)** A register value used as a local snapshot follows ordinary register-value copy/move semantics. The address-bound register storage remains fixed external storage.

**§ 19.2(3)** A volatile write is a physical storage effect. It is not automatically Move, ownership handoff, synchronization, DMA transfer, or lifecycle transfer.

**§ 19.2(4)** DMA, device, FFI, or peripheral contracts that transfer ownership must define that transfer separately and visibly.

### § 19.3 Managed values and volatile decoding

**§ 19.3(1)** A raw volatile read must not manufacture Sec ownership, borrowing, generation, or destruction state merely from physical bits.

**§ 19.3(2)** Types requiring managed ownership representation are not automatically valid volatile-decoding targets merely because their bit width is known.

---

## § 20 FFI, channels, and concurrent transfer

**§ 20(1)** Copyability and thread/process transferability are separate classifications.

**§ 20(2)** FFI ownership transfer is defined by the foreign contract and must not be inferred from ABI representation.

**§ 20(3)** Sending a move-only value through a channel transfers ownership only according to the channel operation's explicit contract.

**§ 20(4)** A fallible send must define ownership for both success and failure outcomes. The compiler must never silently lose the sender's ownership on a failed transfer.

**§ 20(5)** `select` and equivalent multi-path transfer constructs require branch-sensitive ownership so that ownership commits only to the selected transfer path.

---

## § 21 Generics and interfaces

**§ 21(1)** A generic operation may use implicit copy only when the concrete instantiation proves copyability under this rulebook.

**§ 21(2)** Lack of proof must not be treated as permission to copy or silently borrow.

**§ 21(3)** Monomorphization may specialize copy/move implementation, but it must preserve the source-level ownership contract.

**§ 21(4)** Owned erased interface values are move-only unless the interface representation and concrete contract prove an independent copy semantic.

**§ 21(5)** Borrowed interface references follow ordinary reference rules and do not transfer referent ownership.

**§ 21(6)** `rules/compiler/generics_lowering.md` requires concrete copy/move
classification before ownership-sensitive executable lowering. Missing
move-aware lowering must fail as an implementation gap and must never silently
lower a move-only specialization as copy-trivial.

---

## § 22 Performance analysis and reverse escape guidance

**§ 22(1)** A legal copy remains a copy even when it is expensive.

**§ 22(2)** Compiler analysis may advise that a large by-value parameter or local copy should instead use `ref`, a slice, a view, or another non-owning representation when the function does not need an independent value.

**§ 22(3)** The analysis in § 22(2) must not silently rewrite the source-language parameter contract from by-value to borrow.

**§ 22(4)** Example source syntax remains legal when copyability permits it:

```sec
fn Inspect(value: Buffer) void {
    Use(value)
}
```

**§ 22(5)** A mentor diagnostic may recommend:

```sec
fn Inspect(ref value: Buffer) void {
    Use(value)
}
```

or a slice/view form when analysis proves that copying the full value is unnecessary.

**§ 22(6)** Advice should be based on statically supported facts such as known size, known use pattern, escape behavior, or transfer requirements. The compiler must not invent runtime-size assumptions merely to warn.

---

## § 23 Destruction classification is separate

**§ 23(1)** Copy classification and destruction classification are separate semantic properties.

**§ 23(2)** `TriviallyDestructible` versus non-trivial destruction determines cleanup requirements, not whether ordinary copy syntax is legal.

**§ 23(3)** A type may be non-copyable but trivially destructible, or copyable while still requiring representation-aware cleanup, if its canonical type contract permits that combination.

**§ 23(4)** Replacement and move codegen must obey destruction rules independently of copy classification.

---

## § 24 Availability tests and copy/move

**§ 24(1)** `place is available` and `place is not available` are ownership-state tests defined by `ownership.md`.

**§ 24(2)** They are not copy operations, move operations, `Option` tests, null checks, generation-validity checks, or borrow-permission checks.

**§ 24(3)** A proven `Available` path may perform copy or explicit move if the remaining rules permit it.

**§ 24(4)** A proven `Unavailable` path may not read, copy, borrow, or move the old value.

**§ 24(5)** The compiler must resolve availability statically whenever possible and must not introduce runtime ownership bookkeeping when static control-flow facts are sufficient.

---

## § 25 Semantic IR requirements

**§ 25(1)** Semantic IR must preserve the distinction between Copy, Move, Replace, Reinitialize, Discard, Destroy, return transfer, argument transfer, aggregate transfer, and borrowing whenever the distinction is required for later correctness or verification.

**§ 25(2)** Semantic IR must not reconstruct ownership semantics from backend types, ABI categories, or textual syntax after Sema has already resolved the operation.

**§ 25(3)** At minimum, ownership-sensitive IR must preserve enough information to verify:

```text
source Place identity where relevant
source availability before and after transfer
destination ownership
copy classification
consuming parameter/argument action
replacement versus reinitialization
destruction responsibility
partial aggregate state
failure-path transfer commit
volatile access boundaries
address-stability constraints
```

**§ 25(4)** Unsupported ownership-sensitive lowering must fail explicitly rather than substitute an ordinary copy or lose destruction responsibility.

---

## § 26 Lowering and optimization

**§ 26(1)** Lowering may use copy elision, move elision, destination passing, buffer reuse, in-place update, tensor bufferization, register forwarding, stack-slot reuse, `memcpy`, `memmove`, or target-specific ownership transfer.

**§ 26(2)** Optimization may eliminate a physical move or physical copy only when source availability, destination value, destruction count, borrow validity, external ownership, volatile effects, and address stability remain semantically identical.

**§ 26(3)** Return-value optimization does not change the fact that one owned result transfers from callee to caller.

**§ 26(4)** Copy elision must not make a source unavailable when source semantics specify copy.

**§ 26(5)** Move elision must not leave a source available when source semantics specify move.

---

## § 27 ABI and representation independence

**§ 27(1)** ABI classification does not decide copy/move semantics.

**§ 27(2)** A value passed in registers may still be semantically moved. A large value passed through a hidden pointer may still be semantically copied.

**§ 27(3)** An owned return may use return registers, split registers, hidden return storage, caller-allocated result storage, or target-specific aggregate return without changing caller ownership.

**§ 27(4)** Foreign ABI lowering must preserve the source ownership contract or reject the boundary when it cannot do so safely.

---

## § 28 Diagnostics

### § 28.1 Mentor requirement

**§ 28.1(1)** Copy/move safety diagnostics must explain the programmer-visible ownership problem before compiler-theory terminology.

**§ 28.1(2)** A use-after-move diagnostic must identify the value, the operation that consumed it, and why the later use is invalid.

**§ 28.1(3)** When safe, the diagnostic should suggest the relevant explicit form, such as `:<-`, `<-`, `Consume(<-value)`, `Some(<-value)`, or a borrow alternative.

### § 28.2 Required error classes

**§ 28.2(1)** The compiler must diagnose at least:

```text
copy of non-copyable value
use after move
move while borrowed
invalid self-move
overlapping move
reinitialization of immutable Place
unsupported partial move
move from non-owned storage
move from fixed/volatile storage
missing consuming call-site marker
implicit consuming payload construction
invalid whole-self method consumption
```

**§ 28.2(2)** Error classification and configurable severity names are governed by the diagnostics rulebook.

### § 28.3 Performance advice

**§ 28.3(1)** Large-copy and reverse-escape guidance is advisory unless the project policy independently elevates it under the diagnostics governance model.

**§ 28.3(2)** A performance diagnostic must not claim that a legal copy is semantically invalid.

---

## § 29 LSP and tooling

**§ 29(1)** The LSP must consume the same resolved copy/move facts as compiler diagnostics.

**§ 29(2)** Tooling should be able to expose copy classification, operation at cursor, move origin, destination, consuming parameter, source availability, partial aggregate state, and a safe borrow/move alternative where provable.

**§ 29(3)** Formatter support must preserve all ownership-significant syntax including `:<-`, `<-`, consuming `->` parameter declarations, `capture(<-value)`, and optional `return <-value`.

**§ 29(4)** A code action must not insert `<-` unless Sema proves that the resulting consumption is valid and does not create an unreported later-use error.

---

## § 30 Conformance examples

### § 30.1 Copy versus move initialization

```sec
let first := source
let second :<- source
```

**§ 30.1(1)** `first := source` copies and requires copyability.

**§ 30.1(2)** `second :<- source` moves and consumes `source`.

### § 30.2 Consuming call

```sec
fn CloseHandle(-> handle: Handle) void {
    CloseNative(handle)
}

CloseHandle(<-currentHandle)
```

**§ 30.2(1)** Omitting `<-` at the call site is invalid for the reusable source `currentHandle`.

### § 30.3 Fresh temporary

```sec
CloseHandle(OpenHandle())
```

**§ 30.3(1)** The fresh temporary may forward directly to the consuming parameter.

### § 30.4 Owning payload

```sec
let option := Some(<-resource)
```

**§ 30.4(1)** A reusable move-only source requires explicit payload consumption.

### § 30.5 Return boundary

```sec
return resource
```

and:

```sec
return <-resource
```

**§ 30.5(1)** Both forms transfer the return value. The marker is optional.

### § 30.6 Volatile snapshot

```sec
let first := device.Status
let second := first
```

**§ 30.6(1)** If `device.Status` is volatile storage access, the first statement performs one volatile read. The second statement copies the ordinary local snapshot and performs no additional hardware read.

### § 30.7 No implicit move from ordinary by-value argument

```sec
fn Inspect(value: Buffer) void {
    Use(value)
}

Inspect(buffer)
```

**§ 30.7(1)** The call requires `buffer` to be copyable. The compiler must not consume `buffer` merely because `Buffer` is move-only.

---

## § 31 Explicit exclusions for Sec 0.1

**§ 31(1)** Sec 0.1 does not define arbitrary user-defined implicit copy bodies.

**§ 31(2)** Sec 0.1 does not define `@noMove` or `@affine` source annotations.

**§ 31(3)** Sec 0.1 does not infer destructive move from ordinary `:=`, `=`, ordinary by-value call syntax, ordinary aggregate payload syntax, or plain closure capture.

**§ 31(4)** Sec 0.1 does not move ownership out of volatile/MMIO storage merely because its representation can be read.

**§ 31(5)** Sec 0.1 does not permit ordinary methods to consume complete `self`.

**§ 31(6)** Sec 0.1 does not use backend representation, ABI lowering, or optimizer behavior to redefine copy/move source semantics.

---

## § 32 Cross-rulebook references

**§ 32(1)** The following rulebooks are normative companions to this document:

```text
rules/memory/ownership.md
rules/memory/borrowing.md
rules/memory/destruction.md
rules/control-flow/discard.md
rules/declarations/functions.md
rules/declarations/lambda-functions.md
rules/declarations/unions.md
rules/platform/volatile.md
rules/compiler/semantic_ir.md or its current canonical predecessor
rules/tooling/diagnostics.md or its current canonical predecessor
rules/tooling/lsp.md
rules/tooling/formatter.md
```

**§ 32(2)** If an older companion rulebook conflicts with this revision on explicit reusable-source consumption, the revision-2 ownership/copy-move rule takes precedence until the companion book is synchronized through governance/corrections.
