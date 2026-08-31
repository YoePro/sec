# Borrowing

- **Status:** Normative
- **Created:** 2026-08-28
- **Last updated:** 2026-08-28
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/memory/borrowing.md`
- **Replaces:** `rules/memory/borrowing.txt`
- **Repository baseline reviewed:** `069c111`

---

## § 1 Purpose and authority

**§ 1(1)** Borrowing provides temporary access authority to an existing Sec Place without transferring ownership of the borrowed value.

**§ 1(2)** Borrowing is a compile-time aliasing and authority discipline. Sec borrowing must not require runtime borrow counters, runtime borrow locks, garbage collection, or programmer-written lifetime annotations.

**§ 1(3)** This rulebook defines shared and mutable borrow authority, borrow creation, borrow live ranges, reborrowing, Place overlap, control-flow merging, interaction with ownership operations, and compiler/tooling obligations.

**§ 1(4)** `rules/memory/ownership.md` owns Place availability, ownership transfer, partial and conditional availability, and `is available` / `is not available`.

**§ 1(5)** `rules/memory/copy_move.md` owns copy and move classification of reference values and all ownership-transfer syntax.

**§ 1(6)** `rules/memory/destruction.md` owns cleanup and destruction. This rulebook defines when an active borrow prevents destruction, replacement, discard, or another ownership-invalidating action.

**§ 1(7)** `rules/memory/reference_model.md` owns safe-reference validity, provenance, generation/epoch semantics, address-space validity, stable/weak handle semantics, and runtime representation choices.

**§ 1(8)** `rules/memory/references.md` owns source-level safe-reference semantics. Canonical lifetime-analysis responsibilities are defined by `rules/memory/lifetime_analysis.md` revision 2. This rulebook must not invent visible lifetime parameters or pre-empt future physical-addressing rules.

**§ 1(9)** `rules/declarations/functions.md`, `rules/declarations/lambda-functions.md`, `rules/control-flow/flowcontrol_match.md`, `rules/control-flow/defer.md`, collection rulebooks, FFI rulebooks, and concurrency rulebooks may impose context-specific restrictions in addition to the core borrow rules here.

---

## § 2 Core terminology

### § 2.1 Borrow

**§ 2.1(1)** A borrow is temporary authority to access a referent through a safe reference or equivalent compiler-known view without becoming the owner of the referent.

**§ 2.1(2)** A borrow has at least:

```text
borrow kind
referent Place or finite set of possible Places
holder identity where applicable
creation point
live range
mutability authority
source provenance
```

### § 2.2 Referent

**§ 2.2(1)** The referent is the Place whose storage is accessed under borrowed authority.

### § 2.3 Borrow holder

**§ 2.3(1)** A borrow holder is a reference value, slice/view value, closure capture, returned reference dependency, defer dependency, parameter binding, or compiler-known temporary relation that keeps borrowed authority live.

**§ 2.3(2)** A holder is not the owner of the referent merely because the holder itself is copied, moved, returned, stored, or destroyed.

### § 2.4 Borrow live range

**§ 2.4(1)** The borrow live range is the set of control-flow points where borrowed authority must remain valid because a holder or derived borrow may still be used.

### § 2.5 Reborrow

**§ 2.5(1)** A reborrow creates narrower or equal borrowed authority from an existing safe reference without transferring ownership of the referent.

### § 2.6 Overlap

**§ 2.6(1)** Two borrowed Places overlap when they may designate any common storage according to canonical Place/provenance analysis.

**§ 2.6(2)** Overlap is semantic, not merely textual or address-equality based.

---

## § 3 Borrow kinds

### § 3.1 Shared borrow

**§ 3.1(1)** `ref T` denotes shared borrowed authority to a valid `T`.

**§ 3.1(2)** Shared borrowed authority permits reading the referent.

**§ 3.1(3)** Shared borrowed authority does not permit mutation through that shared reference.

**§ 3.1(4)** Any number of compatible shared borrows may coexist over overlapping storage.

### § 3.2 Mutable borrow

**§ 3.2(1)** `ref mut T` denotes exclusive mutable borrowed authority to a valid mutable `T`.

**§ 3.2(2)** Mutable borrowed authority permits reading and writing the referent through that authority.

**§ 3.2(3)** A mutable borrow requires exclusive authority over its overlapping Place for the live range of that borrow.

**§ 3.2(4)** A mutable borrow may not coexist with another overlapping mutable borrow or an overlapping shared borrow except through a legal reborrow relationship whose authority is nested and constrained by this rulebook.

### § 3.3 Borrow kind is not ownership

**§ 3.3(1)** Neither `ref T` nor `ref mut T` transfers ownership of `T`.

**§ 3.3(2)** Mutable authority is not ownership authority. A `ref mut T` may mutate `T` but may not move an owned move-only subvalue out of `T` merely because mutation is permitted.

---

## § 4 Core borrow invariants

**§ 4(1)** A shared borrow prevents overlapping mutation, replacement, reinitialization, move, discard, destruction, and invalidating structural operations for the duration of the shared borrow.

**§ 4(2)** While an overlapping mutable borrow is active, the owner and unrelated aliases may not directly read, mutate, replace, reinitialize, move, discard, or destroy the referent through competing authority.

**§ 4(3)** Access through the granting mutable reference or a legal reborrow derived from it is not treated as conflicting owner access.

**§ 4(4)** A borrow must never outlive storage validity, provenance validity, address-space validity, or any generation/epoch dependency required by the canonical reference model.

**§ 4(5)** Passing a reference value, copying a shared reference, moving a mutable reference value, or storing a reference in an aggregate does not transfer ownership of the referent.

**§ 4(6)** Borrow correctness is mandatory language safety analysis and cannot be disabled by diagnostic severity configuration.

---

## § 5 Borrow creation syntax and addressability

### § 5.1 Explicit local borrow

**§ 5.1(1)** An explicit shared borrow is written:

```sec
let view := ref value
```

**§ 5.1(2)** An explicit mutable borrow is written:

```sec
let view := ref mut value
```

### § 5.2 Addressable Place requirement

**§ 5.2(1)** Explicit borrow creation requires a reusable addressable Place with sufficient authority.

**§ 5.2(2)** A non-addressable computed expression cannot be borrowed merely because its result type is otherwise borrowable.

**§ 5.2(3)** Sec 0.1 does not make arbitrary temporary-expression lifetime extension part of general explicit borrow syntax.

**§ 5.2(4)** A specialized rulebook may define a compiler-materialized call-lifetime or operation-lifetime Place only when that behavior is explicit in that rulebook and preserves all borrowing guarantees.

### § 5.3 Source validity

**§ 5.3(1)** Borrow creation requires the target Place to contain a valid initialized value on the current control-flow path.

**§ 5.3(2)** Borrow creation from an `Unavailable` Place is invalid.

**§ 5.3(3)** Borrow creation from a `ConditionallyAvailable` Place is invalid unless current-path control-flow refinement proves the Place `Available`.

---

## § 6 Availability and borrowing are separate

**§ 6(1)** Availability answers whether the current owner still owns a value in a Place.

**§ 6(2)** Borrow compatibility answers whether that Place may currently be accessed with requested shared or mutable authority.

**§ 6(3)** Therefore:

```sec
if package.Payload is available {
    Use(package.Payload)
}
```

proves ownership availability only. It does not prove that no conflicting borrow is active.

**§ 6(4)** `is available` and `is not available` must never be interpreted as borrow-status tests, lock-status tests, generation-validity tests, `Option` tests, or null checks.

**§ 6(5)** A Place may be `Available` yet temporarily inaccessible through the owner because an exclusive mutable borrow is active.

**§ 6(6)** A `PartiallyAvailable` aggregate may borrow an `Available` disjoint sub-Place even when borrowing the whole aggregate is invalid.

**§ 6(7)** Whole-value borrowing requires the complete whole-value Place to be `Available` under the ownership rules.

---

## § 7 Mutability authority

**§ 7(1)** A shared borrow may be created from an immutable or mutable Place when ordinary access rules permit reading that Place.

**§ 7(2)** A mutable borrow requires the addressed Place to provide mutable authority.

**§ 7(3)** `ref mut` may not be created from an immutable Place merely because its type is mutable in another context.

**§ 7(4)** Mutability of a root binding propagates to ordinary owned sub-Places according to declaration/member rules; borrowing does not silently upgrade immutable storage to mutable storage.

**§ 7(5)** A method's inferred receiver authority is governed by the function/impl rules. Borrowing may supply shared or mutable receiver authority, but an ordinary method may not use borrowing as a mechanism to consume whole `self`.

---

## § 8 Borrow live ranges

### § 8.1 Compiler-derived liveness

**§ 8.1(1)** Sec source code does not contain explicit lifetime parameters or lifetime annotations.

**§ 8.1(2)** Borrow live ranges are derived by control-flow and use analysis.

**§ 8.1(3)** A borrow begins when borrowed authority is established.

**§ 8.1(4)** On a control-flow path, a borrow may end after its final reachable use when no surviving holder, returned dependency, capture, defer dependency, aggregate, or derived borrow still requires that authority.

**§ 8.1(5)** A lexical scope may outlive the semantic borrow live range.

**§ 8.1(6)** The compiler may retain internal bookkeeping longer than the semantic live range only when doing so does not reject otherwise valid Sec source or alter observable semantics.

### § 8.2 Holder termination

**§ 8.2(1)** Reassigning or destroying a reference holder ends the authority held solely by that holder after all derived dependencies have ended.

**§ 8.2(2)** Moving a move-only reference holder transfers the holder's borrow obligation to the destination rather than ending the referent borrow.

**§ 8.2(3)** Copying a shared reference creates another holder. The shared borrow remains live while any surviving holder or derived borrow still requires it.

### § 8.3 Path sensitivity

**§ 8.3(1)** Borrow liveness is path-sensitive.

**§ 8.3(2)** A borrow that ends on one path but remains live on another must be conservatively considered live at a merge point reachable from the latter path.

**§ 8.3(3)** Terminating paths do not contribute borrow state to later unreachable merge points.

---

## § 9 Shared-borrow semantics

**§ 9(1)** Direct owner reads of an overlapping Place remain legal while only compatible shared borrows are active.

**§ 9(2)** Direct owner mutation of an overlapping Place is invalid while any shared borrow is live.

**§ 9(3)** Replacement, reinitialization, discard, destruction, ownership move, and structural invalidation of overlapping storage are mutations for shared-borrow conflict purposes.

**§ 9(4)** A shared borrow may be copied according to the copy/move classification of `ref T` without changing ownership of the referent.

**§ 9(5)** A shared reborrow from `ref T` or `ref mut T` creates shared authority no stronger than the source authority.

---

## § 10 Mutable-borrow semantics

**§ 10(1)** A mutable borrow reserves exclusive mutable authority over the overlapping Place for its live range.

**§ 10(2)** While that mutable borrow is live, direct overlapping owner reads are invalid unless performed through the granting reference/reborrow relationship.

**§ 10(3)** While that mutable borrow is live, direct overlapping owner mutation is invalid unless performed through the granting reference/reborrow relationship.

**§ 10(4)** Disjoint owner Places remain independently usable when the compiler proves they do not overlap the mutable borrow.

**§ 10(5)** `ref mut T` is move-only as a reference value according to `copy_move.md`; moving that reference transfers borrowed authority but does not move `T`.

**§ 10(6)** A mutable reference may be reborrowed without consuming the parent reference value when the borrowing context is explicitly a borrow context.

**§ 10(7)** During an overlapping child reborrow, the parent mutable reference may not exercise authority that conflicts with the child borrow.

**§ 10(8)** When the child reborrow ends, the parent reference may again exercise its remaining authority if all other validity requirements still hold.

---

## § 11 Reference values and reborrowing

### § 11.1 Shared reference values

**§ 11.1(1)** Copying `ref T` copies the non-owning reference value and its provenance/borrow dependency.

**§ 11.1(2)** Copying `ref T` does not copy or duplicate the referent.

### § 11.2 Mutable reference values

**§ 11.2(1)** `ref mut T` must not be copied.

**§ 11.2(2)** Explicitly moving a `ref mut T` value transfers the holder and makes the moved-from reference Place unavailable according to ownership/copy-move rules.

### § 11.3 Reborrow is not move

**§ 11.3(1)** Passing an existing `ref mut T` to a parameter of type `ref mut T` is a reborrow operation, not an ownership move of the caller's reference holder.

**§ 11.3(2)** Therefore a caller's mutable reference may remain usable after the call when the call-bounded reborrow has ended and no other rule invalidated it.

**§ 11.3(3)** Passing an existing safe reference to an ordinary by-value parameter follows reference-value copy/move classification instead and is not automatically a reborrow unless the parameter contract is borrowed.

---

## § 12 Place identity and overlap

**§ 12(1)** Borrow analysis must operate on canonical semantic Places rather than only root variable names.

**§ 12(2)** A whole Place overlaps every descendant projection.

**§ 12(3)** Equal Places overlap.

**§ 12(4)** Distinct struct fields may be treated as disjoint when the language representation and Place model prove they do not overlap.

**§ 12(5)** Different active union variant payload projections are structurally disjoint, but borrowing the whole union overlaps the active payload.

**§ 12(6)** Reference and slice aliases must carry canonical origin information sufficient to project later accesses back to their possible referent Places.

**§ 12(7)** When control flow yields a finite set of possible referent Places, the compiler may retain that finite set and check every alternative without introducing a runtime provenance tag.

**§ 12(8)** Unknown or over-limit provenance must be conservative. The compiler must not pretend that one origin is proven when several or unknown origins remain possible.

---

## § 13 Fields, properties, and aggregate Places

### § 13.1 Disjoint fields

**§ 13.1(1)** Separate fields of a struct may be borrowed independently when disjointness is proven.

```sec
type Pair struct {
    Left: int
    Right: int
}

let left := ref mut pair.Left
let right := ref mut pair.Right
```

**§ 13.1(2)** The example is valid only when `Left` and `Right` are proven disjoint Places and `pair` provides mutable authority.

### § 13.2 Whole aggregate

**§ 13.2(1)** Borrowing an entire aggregate conflicts with borrowing any overlapping sub-Place.

**§ 13.2(2)** An active whole-value mutable borrow blocks direct use of every overlapping field through the owner.

### § 13.3 Properties

**§ 13.3(1)** A property must not be assumed to denote independently addressable stored state merely because it is selected with member syntax.

**§ 13.3(2)** Until compiler-known property effect/addressability metadata proves narrower behavior, a borrowed/mutated property projection conservatively overlaps the receiver storage that the property may access.

**§ 13.3(3)** A property returning a safe reference carries the returned reference's provenance/lifetime contract; it does not make the property itself an owned storage field.

---

## § 14 Arrays, indices, slices, and views

### § 14.1 Constant indices

**§ 14.1(1)** Distinct compile-time constant indices into the same fixed storage may be treated as disjoint when element storage does not overlap.

### § 14.2 Runtime indices

**§ 14.2(1)** Two runtime/dynamic indices into the same storage are conservatively overlapping unless an analysis proves otherwise.

### § 14.3 Slice ranges

**§ 14.3(1)** Statically known slice ranges may be normalized to half-open intervals for overlap analysis.

**§ 14.3(2)** Proven disjoint static intervals may be borrowed independently.

**§ 14.3(3)** A compile-time known empty interval borrows no elements.

**§ 14.3(4)** Symbolic or runtime ranges overlap conservatively unless disjointness is proven.

### § 14.4 Slice and view values

**§ 14.4(1)** A shared slice/view is a non-owning value whose descriptor may be copied only while preserving its backing-storage borrow dependency.

**§ 14.4(2)** A mutable slice/view carries exclusive borrowed authority and follows the move-only/reborrow rules defined for its canonical type.

**§ 14.4(3)** Projecting an index or subrange through a local slice alias must preserve and narrow canonical referent provenance where statically possible.

### § 14.5 Structural mutation

**§ 14.5(1)** While an element/slice borrow is live, an operation that may relocate storage, change element addresses, reorder borrowed elements, or invalidate the referenced range is a borrow conflict.

**§ 14.5(2)** Mutating an element through an iterator- or slice-provided `ref mut` is permitted when the mutable reference grants authority to that element and no other conflict exists.

---

## § 15 Function parameters and calls

### § 15.1 Borrowed parameters

**§ 15.1(1)** A parameter declared `value: ref T` receives shared borrowed authority.

**§ 15.1(2)** A parameter declared `value: ref mut T` receives exclusive mutable borrowed authority.

**§ 15.1(3)** The caller remains owner of the referent for both forms.

### § 15.2 Call-site borrow creation

**§ 15.2(1)** Passing an owned compatible Place to a `ref T` parameter may create a call-bounded shared borrow without a separate source-level `ref` marker at the call site.

**§ 15.2(2)** Passing a mutable owned compatible Place to a `ref mut T` parameter may create a call-bounded mutable borrow without a separate source-level `ref mut` marker at the call site.

**§ 15.2(3)** This implicit call-site borrowing is permitted because it does not consume caller ownership. It must never be confused with the mandatory `<-` marker required by consuming ownership transfer.

**§ 15.2(4)** Overload/call resolution must know the selected parameter borrow mode before borrow legality is committed.

### § 15.3 Reborrowed arguments

**§ 15.3(1)** Passing `ref T` to `ref T` may reborrow shared authority.

**§ 15.3(2)** Passing `ref mut T` to `ref T` may create a shared reborrow whose live range prevents conflicting mutable use through the parent reference.

**§ 15.3(3)** Passing `ref mut T` to `ref mut T` may create a mutable reborrow whose live range temporarily reserves the overlapping authority.

### § 15.4 Argument order and borrow commit

**§ 15.4(1)** Function arguments are evaluated left-to-right according to the function rulebook.

**§ 15.4(2)** Borrow preparation for one argument must not violate the validity of later argument evaluation.

**§ 15.4(3)** A call with overlapping mutable-reference arguments is invalid.

**§ 15.4(4)** A call with one mutable-reference argument and another overlapping shared/reference/direct-access argument is invalid when their live call ranges overlap.

**§ 15.4(5)** Failure while evaluating a later argument must not manufacture a borrow state inconsistent with the actual argument expressions already evaluated. Cleanup and ownership-transfer commit remain governed by the function/error/destruction rulebooks.

---

## § 16 Method receivers

**§ 16(1)** Borrow rules apply to method receivers exactly as to equivalent Places and parameter authorities.

**§ 16(2)** A method body that only needs shared receiver access may be invoked through compatible shared authority.

**§ 16(3)** A method body that requires mutation must obtain compatible mutable/exclusive receiver authority according to the canonical method inference rules.

**§ 16(4)** A method call must not overlap a receiver mutable borrow with conflicting argument borrows.

**§ 16(5)** Ordinary methods may move eligible owned members only when ownership and borrow rules permit the member move. Borrowing does not permit whole-`self` consumption by an ordinary method.

---

## § 17 Branches and control-flow joins

**§ 17(1)** Borrow state is analyzed separately along control-flow paths.

**§ 17(2)** At a merge, a borrow that remains live on any continuing incoming path remains potentially live after the merge.

**§ 17(3)** Branch-local holders that cannot escape their branch end before the merge.

**§ 17(4)** A holder declared outside the branch may keep a borrow live after the branch if any continuing path stores or retains that borrow in the holder.

**§ 17(5)** A control-flow join may retain a finite set of possible reference origins. Later reborrows/accesses must be checked against every possible origin.

**§ 17(6)** An unreachable or terminating branch does not force its borrow state into a later merge.

**§ 17(7)** Borrow-state merging must never require runtime borrow tags merely to distinguish static control-flow paths.

---

## § 18 Loops

**§ 18(1)** Borrow validity must hold across every possible loop iteration and backedge.

**§ 18(2)** A borrow that may survive a backedge is considered live when validating the next condition and next iteration.

**§ 18(3)** A borrow created in one iteration must not conflict with an access in a later iteration.

**§ 18(4)** Fresh loop-body local holders end at their iteration boundary when they do not escape that iteration.

**§ 18(5)** `continue` edges contribute to the next-iteration borrow state.

**§ 18(6)** `break` edges contribute to post-loop borrow state only on the exits they reach.

**§ 18(7)** Zero-iteration paths must be included for loop forms whose semantics permit zero iterations.

**§ 18(8)** A reference must not escape an iteration if the referenced storage may be invalidated before the next possible use.

**§ 18(9)** Collection iteration must preserve the structural-stability restrictions of § 14.5 and the canonical collection/for-loop rulebooks.

---

## § 19 Match and pattern borrowing

### § 19.1 Whole-payload borrow

**§ 19.1(1)** A pattern such as:

```sec
Some(ref value)
```

creates shared borrowed authority to the active payload.

**§ 19.1(2)** A pattern such as:

```sec
Some(ref mut value)
```

creates mutable borrowed authority to the active payload and requires mutable authority over the reusable subject.

### § 19.2 Scope

**§ 19.2(1)** Match payload borrows are arm-scoped unless another canonical rule explicitly permits a returned/derived reference that satisfies full reference-origin rules.

**§ 19.2(2)** A pattern borrow is visible in that arm's `where` guard and body.

**§ 19.2(3)** If a guarded arm is not selected, candidate borrow authority established solely for that arm ends before matching continues to the next arm.

### § 19.3 Borrowed subjects

**§ 19.3(1)** A subject accessed through `ref UnionType` may provide compatible shared payload borrows.

**§ 19.3(2)** A subject accessed through `ref mut UnionType` may provide compatible shared or mutable payload reborrows.

**§ 19.3(3)** Borrowed subject authority never permits ownership move-out of a move-only payload.

### § 19.4 Shallow destructuring

**§ 19.4(1)** `ref` and `ref mut` field bindings in shallow match destructuring borrow the corresponding proven payload sub-Places.

**§ 19.4(2)** Borrowed payload destructuring must not invent independence between fields that may overlap.

**§ 19.4(3)** Recursive/nested borrowed destructuring beyond the forms accepted by the canonical match rulebook is not introduced by this rulebook.

---

## § 20 Closure and lambda captures

### § 20.1 Shared capture

**§ 20.1(1)** `capture(ref value)` creates a shared borrow retained by the closure environment.

**§ 20.1(2)** The outer owner must remain valid and non-conflicting for as long as the closure may use that capture.

### § 20.2 Mutable capture

**§ 20.2(1)** `capture(ref mut value)` creates exclusive mutable borrowed authority retained by the closure environment.

**§ 20.2(2)** While the closure retains that capture, conflicting outer access is invalid.

**§ 20.2(3)** A closure containing a mutable-reference capture is move-only and requires at least the callable capability specified by the lambda rulebook.

### § 20.3 Escape

**§ 20.3(1)** A closure with borrowed captures may escape only when every captured borrow remains valid for the escaped closure's complete possible use range.

**§ 20.3(2)** Returning a closure that borrows a function-local owned value is invalid when the local storage cannot outlive the returned closure.

**§ 20.3(3)** No explicit lifetime syntax is introduced for closure captures.

### § 20.4 Owned captures are separate

**§ 20.4(1)** `capture(value)` and `capture(<-value)` are ownership/copy-move operations, not borrow forms.

**§ 20.4(2)** Borrow analysis must nevertheless preserve reference/storage dependencies carried inside an owned captured value such as a slice, view, reference, or nested closure.

---

## § 21 Defer and delayed use

**§ 21(1)** A registered `defer` may extend the effective future use of a value or reference until the defer executes.

**§ 21(2)** If a deferred operation will read or borrow a Place later, an earlier move, discard, destruction, or invalidation that would make that future use invalid must be rejected.

**§ 21(3)** A defer dependency is not automatically an escaping arbitrary callback; its invocation boundary is defined by the defer rulebook.

**§ 21(4)** A reference holder used by defer cannot be moved or destroyed before the deferred use when that move/destruction would end the required borrow authority.

**§ 21(5)** Place-sensitive defer effects may narrow conflicts to proven sub-Places; until such precision is proven, conservative overlap is required.

---

## § 22 Moves, discard, replacement, and destruction

### § 22.1 Move while borrowed

**§ 22.1(1)** Moving an owned Place is invalid while any incompatible overlapping borrow is live.

**§ 22.1(2)** Moving a disjoint sub-Place remains legal when the active borrow does not overlap that sub-Place and all ownership rules permit the move.

### § 22.2 Discard while borrowed

**§ 22.2(1)** `discard` may not destroy or invalidate a value while an incompatible overlapping borrow still requires that value.

**§ 22.2(2)** Discard of a reference holder ends that holder's authority according to reference-value semantics; it does not discard/destroy the referent.

### § 22.3 Replacement and reinitialization

**§ 22.3(1)** Replacing an `Available` Place is invalid while an incompatible borrow overlaps the old value that would be destroyed or invalidated.

**§ 22.3(2)** Reinitializing an `Unavailable` Place must not invalidate a borrow that could still legally designate the previous storage incarnation under the reference model.

**§ 22.3(3)** Collection/storage replacement that changes backing storage must invalidate or reject incompatible outstanding references according to storage/reference rules before replacement commits.

### § 22.4 Destruction

**§ 22.4(1)** Automatic destruction must not occur while a live borrow still requires the value.

**§ 22.4(2)** Borrow liveness may therefore delay the earliest legal destruction point but does not transfer destruction responsibility from the owner to the reference holder.

---

## § 23 Returned references and origin propagation

**§ 23(1)** A returned safe reference must satisfy canonical reference-origin and lifetime rules.

**§ 23(2)** A reference to ordinary function-local storage may not be returned when that storage ends with the function invocation.

**§ 23(3)** A returned reference may propagate allowed authority from a parameter, receiver, arena/static domain, stable storage, or another origin recognized by the reference/lifetime rulebooks.

**§ 23(4)** Direct and transitive calls must preserve sufficient origin/provenance information to validate the caller-side borrow.

**§ 23(5)** This rulebook does not freeze the full syntax or summary representation for returned-reference origin analysis; `reference_model.md`, `references`, and future lifetime-v2 material own those details.

---

## § 24 FFI boundary

**§ 24(1)** An extern parameter of type `ref T` or `ref mut T` represents a non-null safe Sec borrow specialized to the foreign call boundary when the canonical FFI rulebook permits that parameter form.

**§ 24(2)** Such a foreign borrow is call-bounded unless an explicit FFI retention contract says otherwise.

**§ 24(3)** Foreign code must not retain a call-bounded Sec safe-reference address after the call.

**§ 24(4)** Raw foreign pointer returns, retained pointer fields, nullable foreign pointers, and unproven foreign aliases use `RawPtr[T]` or another canonical FFI abstraction rather than pretending to be ordinary safe borrows.

**§ 24(5)** Borrow legality must be established before ABI lowering. The backend must not infer safe borrow authority from a raw address representation.

---

## § 25 Volatile, addressed, and physical storage boundaries

**§ 25(1)** Borrowing a Sec value and observing volatile/physical storage are distinct semantics.

**§ 25(2)** `volatile` is a storage-access property. A volatile read may produce an ordinary Sec snapshot value whose later borrows follow ordinary Sec borrowing rules.

**§ 25(3)** The existence of an address, register width, MMIO mapping, or volatile access contract does not by itself grant safe `ref` or `ref mut` authority.

**§ 25(4)** Safe borrowing from physical/addressed storage must satisfy the specialized platform/addressing/register rulebooks in addition to the core reference/borrow guarantees.

**§ 25(5)** `rules/platform/hardware-register-access.md` owns the specialized physical-addressing/register-to-hardware contract. A typed register view derived from a compiler-known runtime mapping borrows the mapping/resource lifetime and must not outlive its mapping owner.

**§ 25(6)** A volatile access must not be duplicated, removed, reordered, or converted into ordinary cached access merely because a later Sec borrow of a snapshot is optimized.

**§ 25(7)** Borrowing a register view does not grant hardware privilege, mapping authority, security-domain authority, or resource ownership beyond the authority carried by the live mapping/resource contract.

---

## § 26 Generation and epoch validity

**§ 26(1)** Borrow compatibility and generation/epoch validity are separate safety dimensions.

**§ 26(2)** A runtime generation check does not grant shared or mutable borrow authority.

**§ 26(3)** A compile-time valid borrow may still require a runtime generation/epoch check when the canonical reference model says the referent's temporal validity cannot be proven statically.

**§ 26(4)** Conversely, a matching generation/epoch does not make two conflicting mutable/shared borrows legal.

**§ 26(5)** Constrained targets may use address-only references where lifetime and invalidation are statically proven; borrowing semantics remain unchanged.

---

## § 27 Concurrency

**§ 27(1)** Borrowing does not make shared state thread-safe.

**§ 27(2)** A borrow crossing a task/thread/concurrency boundary must satisfy both this rulebook and the canonical send/share/data-race/concurrency-memory rules.

**§ 27(3)** `ref mut T` exclusivity is necessary but not sufficient to prove safe cross-thread transfer or synchronization.

**§ 27(4)** Shared borrows do not imply immutable storage globally; concurrent mutation requires the explicit synchronization model defined by concurrency rulebooks.

**§ 27(5)** Borrow analysis and data-race analysis may share Place/provenance facts, but neither may silently weaken the other's requirements.

---

## § 28 Generics, interfaces, and opaque calls

**§ 28(1)** Generic instantiation must preserve `ref` and `ref mut` parameter modes exactly.

**§ 28(2)** Copy/move traits of `T` do not change a parameter declared `ref T` into by-value ownership or a parameter declared by value into a borrow.

**§ 28(3)** Interface/callable contracts must preserve borrow mode in signature identity/compatibility as defined by their canonical rulebooks.

**§ 28(4)** For an opaque/imported call with insufficient effect/retention information, borrow analysis must use the conservative contract defined by the function/FFI/module rules rather than assuming a shorter or non-retained borrow.

**§ 28(5)** Borrow-related function summaries may be inferred/serialized for separate compilation, but they must not change the source-level contract silently.

---

## § 29 Static analysis requirements

**§ 29(1)** Before ownership-sensitive lowering, Sema/analysis must resolve enough information to validate:

```text
borrow kind
referent Place or conservative provenance set
holder
creation point
mutability authority
addressability
Place overlap
borrow live range
branch and loop liveness
reborrow parent/child relation
reference-origin dependencies
storage invalidation dependencies
move/discard/destruction conflicts
escape/return dependencies
```

**§ 29(2)** Place overlap should be as precise as proven facts permit and conservative otherwise.

**§ 29(3)** The compiler must use finite path-disjunctive provenance when practical rather than collapsing immediately to one root if several known alternatives can be represented statically.

**§ 29(4)** Bounded widening is permitted for compiler scalability, but widening must only make analysis more conservative and must never permit an unsafe alias pattern.

**§ 29(5)** Borrow analysis must not introduce runtime borrow state solely because compile-time provenance is difficult.

---

## § 30 Semantic IR requirements

**§ 30(1)** Semantic IR must preserve enough resolved information that later lowering does not need to rediscover source-level borrow legality.

**§ 30(2)** Semantic IR must be able to represent or attach facts for at least:

```text
BorrowShared
BorrowMutable
ReborrowShared
ReborrowMutable
borrow source Place/provenance
holder identity
borrow begin
borrow end/live-range boundary
mutable authority
range/extent when relevant
returned-reference origin
closure/defer retention dependency
storage invalidation dependency
```

**§ 30(3)** Borrowed function arguments and method receiver borrows must remain distinguishable from by-value copy/move transfers.

**§ 30(4)** A `ref mut` call reborrow must not be lowered as consumption of the caller's mutable-reference holder.

**§ 30(5)** Match-arm candidate borrows and guard-false borrow ends must be represented accurately enough to avoid keeping rejected-arm authority live.

**§ 30(6)** Semantic IR may encode some borrow facts in side tables/analysis metadata rather than executable operations when that representation preserves all required semantics and diagnostics.

---

## § 31 Lowering and backend requirements

**§ 31(1)** Borrow legality is resolved before low-level backend code generation.

**§ 31(2)** Backend lowering may erase borrow bookkeeping completely when safety has been proven statically and no runtime reference-validity check is required.

**§ 31(3)** Backend lowering must preserve reference provenance, bounds, address-space, generation/epoch checks, and invalidation behavior required by `reference_model.md`.

**§ 31(4)** Backend optimization may not extend a mutation across a source-semantic borrow conflict or reorder an invalidating operation before the final required borrow use.

**§ 31(5)** No backend may infer permission to mutate or move a referent merely from pointer/address uniqueness in low-level IR when source borrow authority did not permit it.

**§ 31(6)** Volatile/MMIO access requirements remain source-observable and must survive borrow/reference optimization.

---

## § 32 Diagnostics and mentor behavior

**§ 32(1)** Borrow diagnostics must explain the programmer's conflicting actions in ordinary language rather than only reporting internal alias-analysis terms.

**§ 32(2)** A conflict diagnostic should identify, where applicable:

```text
the Place the programmer tried to use
the requested action
the existing borrow kind
the borrow creation/retention location
why the Places overlap
when the borrow remains live
what safe restructuring is available
```

**§ 32(3)** A mutable-borrow conflict should distinguish "already mutably borrowed" from "shared borrow still active".

**§ 32(4)** A loop-carried conflict should say that a borrow may remain active from a previous iteration and point to its origin.

**§ 32(5)** A move/discard/destruction conflict should say that the value cannot leave/end ownership while a reference still needs it.

**§ 32(6)** When a borrow could end earlier after its last use, diagnostics/help may recommend reordering or narrowing holder use rather than telling the programmer to add lifetime syntax.

**§ 32(7)** When `is available` is true but a borrow still blocks access, diagnostics must explain that availability and access authority are different concepts.

**§ 32(8)** Diagnostics should point to both the original borrow and conflicting access whenever source locations are available.

---

## § 33 LSP requirements

**§ 33(1)** The LSP must consume compiler-produced borrow facts rather than implementing a parallel borrow checker.

**§ 33(2)** Hover/analysis should be able to expose, where useful:

```text
reference type
shared or mutable borrow
referent Place/provenance
possible origin set
borrow live range/end reason
reborrow relation
mutable authority
storage/generation dependency
conflicting outstanding borrow
```

**§ 33(3)** Navigation from a borrow conflict should be able to jump to the borrow origin/holder that keeps authority live.

**§ 33(4)** Inlay hints may show non-obvious shared/mutable borrow effects, but disabling hints does not alter mandatory safety checks.

**§ 33(5)** A safe quick fix must not replace a required ownership transfer with a borrow or a borrow with a copy unless the transformed program is proven semantically equivalent under the owning rulebooks.

---

## § 34 Formatter requirements

**§ 34(1)** The formatter must preserve semantic borrow syntax including `ref`, `ref mut`, `capture(ref ...)`, `capture(ref mut ...)`, and match pattern borrow markers.

**§ 34(2)** Formatting must not insert or remove borrow markers in a way that changes ownership/borrow behavior.

**§ 34(3)** Formatting may normalize whitespace around borrow syntax according to canonical formatter policy.

---

## § 35 Required conformance tests

**§ 35(1)** A conforming Sec frontend must include tests covering at least the following cases.

### § 35.1 Basic shared/mutable borrows

**§ 35.1(1)** Multiple overlapping shared borrows are accepted.

**§ 35.1(2)** Shared plus overlapping mutable borrow is rejected.

**§ 35.1(3)** Two overlapping mutable borrows are rejected.

**§ 35.1(4)** Mutable borrow from immutable Place is rejected.

**§ 35.1(5)** Direct owner read while overlapping mutable borrow is live is rejected.

**§ 35.1(6)** Direct owner mutation while overlapping shared or mutable borrow is live is rejected.

### § 35.2 Liveness

**§ 35.2(1)** A borrow may end before lexical scope exit after its final proven use.

**§ 35.2(2)** A surviving copied shared-reference holder keeps the borrow live.

**§ 35.2(3)** Reassigning the sole reference holder ends its old borrow when no derived dependency survives.

### § 35.3 Place precision

**§ 35.3(1)** Mutable borrows of disjoint struct fields coexist.

**§ 35.3(2)** Whole-object borrow conflicts with any descendant field borrow.

**§ 35.3(3)** Distinct compile-time constant array indices may coexist.

**§ 35.3(4)** Dynamic indices conservatively conflict.

**§ 35.3(5)** Disjoint static slice intervals coexist.

**§ 35.3(6)** Symbolic overlapping/unknown slice ranges are conservative.

**§ 35.3(7)** Empty static slice interval borrows no elements.

### § 35.4 Calls and reborrows

**§ 35.4(1)** Owned Place passed to `ref T` creates shared call borrow and preserves ownership.

**§ 35.4(2)** Mutable owned Place passed to `ref mut T` creates mutable call borrow and preserves ownership.

**§ 35.4(3)** Existing `ref mut T` passed to `ref mut T` parameter reborrows and remains usable after call when reborrow ends.

**§ 35.4(4)** Existing `ref mut T` passed to an ordinary by-value move-only reference parameter follows copy/move rules rather than silent reborrow.

**§ 35.4(5)** Overlapping mutable call arguments are rejected.

### § 35.5 Ownership interaction

**§ 35.5(1)** Move of overlapping borrowed Place is rejected.

**§ 35.5(2)** Move of proven disjoint sibling Place remains valid.

**§ 35.5(3)** Discard/destruction/replacement of overlapping borrowed Place is rejected.

**§ 35.5(4)** `is available` does not bypass an active borrow conflict.

### § 35.6 Control flow

**§ 35.6(1)** Branch-local holder ends before merge when it cannot escape.

**§ 35.6(2)** Outer holder retaining borrow on one branch keeps borrow active after merge.

**§ 35.6(3)** Terminating branch borrow does not pollute later continuing state.

**§ 35.6(4)** Loop-carried borrow conflicts with next-iteration incompatible access.

**§ 35.6(5)** `break`/`continue` edges preserve correct borrow liveness.

### § 35.7 Match/lambda/defer

**§ 35.7(1)** `Some(ref value)` borrows payload for arm/guard scope.

**§ 35.7(2)** `Some(ref mut value)` requires mutable reusable subject authority.

**§ 35.7(3)** Guard-false candidate borrow ends before next arm.

**§ 35.7(4)** Borrowed subject cannot move move-only payload.

**§ 35.7(5)** `capture(ref value)` prevents closure from outliving referent lifetime.

**§ 35.7(6)** `capture(ref mut value)` blocks conflicting outer access and makes closure move-only.

**§ 35.7(7)** Deferred future use blocks earlier invalidating move/discard/destruction.

### § 35.8 FFI/reference model

**§ 35.8(1)** Extern `ref`/`ref mut` parameter is call-bounded/non-retained absent explicit retention contract.

**§ 35.8(2)** Runtime generation validity does not make an aliasing-invalid borrow legal.

**§ 35.8(3)** Borrow-safe code requiring no runtime temporal check lowers without runtime borrow metadata.

---

## § 36 Implementation boundaries and exclusions

**§ 36(1)** Sec 0.1 does not expose programmer-written lifetime parameters.

**§ 36(2)** Sec 0.1 does not require runtime borrow counters or runtime borrow locks.

**§ 36(3)** Sec 0.1 does not assume arbitrary runtime index/range disjointness without proof.

**§ 36(4)** Sec 0.1 does not make `ref mut` ownership of the referent.

**§ 36(5)** Sec 0.1 does not treat `is available` as permission to ignore borrow conflicts.

**§ 36(6)** Sec 0.1 does not infer safe references from raw/foreign/physical addresses merely because an address is non-zero or aligned.

**§ 36(7)** Sec 0.1 does not define the forthcoming physical-addressing/register hardware model in this rulebook.

**§ 36(8)** Symbolic/runtime range disjointness, imported/opaque alias summaries, function-value reference summaries, recursive summary fixed points, and broader borrowed destructuring may be implemented incrementally, but lack of precision must cause conservative rejection rather than unsafe acceptance.

---

## § 37 Summary

**§ 37(1)** Borrowing grants temporary authority without transferring ownership.

**§ 37(2)** `ref` is shared readable authority; `ref mut` is exclusive mutable authority.

**§ 37(3)** Borrow legality is compile-time, Place-sensitive, path-sensitive, and may use non-lexical final-use liveness.

**§ 37(4)** Ownership availability, borrow access authority, and generation/epoch validity are separate dimensions.

**§ 37(5)** Shared references may have multiple holders; mutable references are move-only but may be reborrowed.

**§ 37(6)** Moves, discard, destruction, replacement, and storage invalidation are rejected while incompatible overlapping borrows are live.

**§ 37(7)** Disjoint fields, constant indices, and statically disjoint ranges may be borrowed independently when proven.

**§ 37(8)** Functions, methods, match patterns, closures, defer, FFI, collections, and concurrency reuse this one borrow model rather than inventing parallel alias semantics.

**§ 37(9)** The compiler must explain borrow conflicts as a mentor: what is borrowed, where, why the later operation conflicts, and how the programmer can restructure the code safely.
