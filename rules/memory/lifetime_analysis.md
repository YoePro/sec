# Lifetime Analysis

- **Status:** Normative
- **Created:** 2026-08-29
- **Last updated:** 2026-08-29
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/memory/lifetime_analysis.md`
- **Replaces:** `rules/memory/lifetime_analysis.txt`
- **Repository baseline reviewed:** `fafd8cb`

---

## § 1 Purpose and authority

**§ 1(1)** Lifetime analysis proves that every non-owning access, borrowed dependency, returned reference, captured reference, cleanup dependency, and storage-backed view remains valid for its complete semantic use.

**§ 1(2)** Lifetime analysis is mandatory compile-time semantic analysis. It is not an optional lint, optimization, deep-analysis mode, or backend convenience.

**§ 1(3)** Ordinary Sec code must not require programmer-written lifetime names, lifetime parameters, lifetime arithmetic, or explicit region annotations.

**§ 1(4)** The compiler carries lifetime complexity by deriving constraints from ownership, borrowing, storage, control flow, calls, captures, destruction, allocation, platform contracts, and unsafe boundaries.

**§ 1(5)** `rules/memory/ownership.md` owns Place availability, ownership transfer, partial and conditional availability, and ownership-state refinement.

**§ 1(6)** `rules/memory/borrowing.md` owns shared and mutable borrow authority, reborrowing, Place overlap, and borrow live ranges.

**§ 1(7)** `rules/memory/destruction.md` owns cleanup, deterministic destruction, partial and conditional destruction, custom `free`, and cleanup ordering.

**§ 1(8)** `rules/memory/copy_move.md` owns copy/move classification and transfer syntax. Lifetime analysis consumes those resolved actions and proves their lifetime consequences.

**§ 1(9)** `rules/memory/reference_model.md` owns safe-reference provenance, generation/epoch validity, stable/weak handle semantics, address-space validity, and representation choices.

**§ 1(10)** `rules/memory/storage.md` owns storage-origin and storage-domain classification. Lifetime analysis consumes that classification and must not replace it with ad-hoc stack/heap guesses.

**§ 1(11)** `rules/platform/fixed-address-bindings.md` and `rules/platform/hardware-register-access.md` own physical-address, mapping, endpoint, volatility, and hardware-authority semantics. This rulebook owns lifetime relationships that depend on those contracts.

**§ 1(12)** Interrupt registration, interrupt routing, handler execution rules, vector binding, masking, nesting, priority, ISR stack rules, and interrupt-specific capture contracts are owned by the interrupt rulebooks. This rulebook defines only the general lifetime constraints that those rulebooks must satisfy.

---

## § 2 Core invariant

**§ 2(1)** A safe reference, view, slice, borrowed closure capture, or other non-owning access must never be usable beyond the complete validity of the value and storage on which it depends.

**§ 2(2)** Conceptually:

```text
use lifetime <= reference lifetime <= referent value lifetime <= required storage validity
```

**§ 2(3)** Additional bounds may apply, including ownership availability, borrow authority, allocation lifetime, generation/epoch validity, mapping lifetime, address stability, platform validity, execution-context authority, and foreign retention contracts.

**§ 2(4)** Satisfying one bound does not imply that another bound is satisfied.

**§ 2(5)** Physical bytes may remain in memory after a semantic lifetime ends. Safe Sec code must not treat residual representation as a live value or valid reference.

**§ 2(6)** Lifetime correctness is semantic. Optimization, register allocation, stack-slot reuse, tail calls, inlining, or backend storage choices must not weaken the invariant.

---

## § 3 Distinct lifetime concepts

### § 3.1 Required distinctions

**§ 3.1(1)** The compiler must distinguish at least:

```text
value lifetime
storage lifetime
ownership duration
availability state
reference lifetime
borrow live range
temporary lifetime
resource lifetime
allocation lifetime
mapping lifetime
closure-environment lifetime
callback retention lifetime
platform validity
address stability
generation or epoch validity
lexical scope
physical stack-frame lifetime
backend lifetime metadata
```

**§ 3.1(2)** These terms are not interchangeable.

### § 3.2 Lexical scope is not lifetime

**§ 3.2(1)** Lexical scope is a source-code region and commonly provides an upper bound for local bindings.

**§ 3.2(2)** A value may end before lexical scope exit because it is moved, discarded, destroyed, replaced, deallocated, invalidated, or proven dead subject to the destruction rules.

**§ 3.2(3)** A borrow may end before lexical scope exit after its final proven use and all derived dependencies end.

### § 3.3 Storage lifetime is not value lifetime

**§ 3.3(1)** Storage may survive multiple sequential values.

```sec
let mut value := CreateFirst()
value = CreateSecond()
```

**§ 3.3(2)** The first value lifetime ends before the second begins even when both occupy the same storage.

### § 3.4 Resource lifetime is not wrapper lifetime

**§ 3.4(1)** An external resource may end while wrapper storage remains.

**§ 3.4(2)** Storage validity must never be used as proof that an external handle, device, mapping, registration, or other semantic resource is still live.

---

## § 4 Lifetime sources and bounds

**§ 4(1)** Lifetime bounds may arise from:

```text
local declaration
parameter validity
caller-owned storage
return-value ownership
reference provenance
aggregate ownership
variant activation
temporary expression
loop iteration
static or global storage
thread-local storage
allocation owner
arena generation
closure environment
callback registration
foreign retention contract
fixed-address storage contract
runtime hardware mapping
platform availability contract
```

**§ 4(2)** The compiler must preserve the source of each bound far enough to explain diagnostics and to export safe interprocedural summaries.

**§ 4(3)** When several bounds apply, the usable lifetime is limited by the earliest invalidating bound on each control-flow path.

**§ 4(4)** An unknown or opaque bound must be treated conservatively unless a canonical contract supplies stronger facts.

---

## § 5 Value lifetime

### § 5.1 Beginning

**§ 5.1(1)** A value lifetime begins when that value becomes validly initialized according to its type and construction rules.

**§ 5.1(2)** Aggregate fields may begin lifetimes independently during partial construction when the construction/destruction model permits it.

**§ 5.1(3)** Reinitialization begins a new value lifetime in the reinitialized Place.

### § 5.2 End

**§ 5.2(1)** A value lifetime ends when the current value is destroyed, moved out without remaining ownership at that Place, discarded, replaced, transferred out, deallocated with its storage, invalidated by a canonical operation, or ceases to be the active semantic variant.

**§ 5.2(2)** A move does not mutate the source into `None`, `null`, zero, default, or another ordinary value; the former value lifetime at that source Place ends and the Place becomes unavailable according to ownership rules.

**§ 5.2(3)** Replacement ends the old value lifetime before the new value lifetime begins.

**§ 5.2(4)** A compiler optimization may reuse storage immediately after a semantic value lifetime ends only when no language rule requires the old value, reference, borrow, cleanup dependency, address identity, or observable storage effect to remain live.

### § 5.3 Partial values

**§ 5.3(1)** A partially available aggregate contains sub-Places whose value lifetimes may differ.

**§ 5.3(2)** Moving one independently movable field ends that field's current value lifetime without ending still-owned disjoint sibling value lifetimes.

**§ 5.3(3)** Whole-value operations remain subject to ownership-v2 availability rules and may not infer whole-value lifetime merely from one live field.

---

## § 6 Storage lifetime

### § 6.1 Definition

**§ 6.1(1)** Storage lifetime is the period during which a storage location is valid for the uses permitted by its storage contract.

**§ 6.1(2)** Storage lifetime may be controlled by the current invocation, caller, allocation owner, arena, static program image, runtime mapping, foreign system, platform, device contract, or another canonical owner.

### § 6.2 Common storage classes

**§ 6.2(1)** Lifetime analysis must support at least the storage classes recognized by the canonical storage model, including local automatic storage, caller-owned parameter storage, owned allocation storage, static/global storage, thread-local storage, aggregate substorage, arena-backed storage, foreign storage, fixed-address storage, and runtime-mapped hardware storage.

**§ 6.2(2)** Storage class is not inferred from syntax alone when the canonical storage model carries stronger metadata.

### § 6.3 No hidden extension

**§ 6.3(1)** The compiler must not extend source-level storage lifetime by silently promoting a local value to heap storage merely to make an escaping reference compile.

**§ 6.3(2)** Stack-to-heap promotion may occur only when a separate language/library construct already gives the value owned storage with the required lifetime; it must not be invented as a hidden lifetime repair.

---

## § 7 Ownership availability and lifetime

### § 7.1 Separate dimensions

**§ 7.1(1)** Ownership availability and lifetime are related but separate facts.

**§ 7.1(2)** `Available` means the current owner still owns a value in the Place on the current path.

**§ 7.1(3)** `Unavailable` means that owner no longer owns a live value there, with a separate reason such as moved, discarded, detached, or destroyed.

**§ 7.1(4)** `ConditionallyAvailable` means ownership differs among reachable runtime paths.

### § 7.2 Lifetime consequence

**§ 7.2(1)** A reference or borrow cannot be created from an ownership-unavailable Place.

**§ 7.2(2)** A non-owning access cannot remain valid across an ownership action that ends or replaces the referred value lifetime.

**§ 7.2(3)** `is available` / `is not available` refines ownership availability only. It does not prove borrow authority, generation validity, mapping validity, device liveness, or platform access permission.

### § 7.3 Conditional availability

**§ 7.3(1)** If a Place is conditionally available, lifetime analysis must preserve path-specific value-lifetime facts until control flow proves the Place available or unavailable on the current path.

**§ 7.3(2)** A target policy may forbid dynamic ownership bookkeeping. In such builds, lifetime analysis must reject code whose later safe use or cleanup requires unresolved runtime ownership state that cannot be represented within the selected policy.

**§ 7.3(3)** `discard` may be used as an ownership-convergence operation according to the ownership/destruction rulebooks; lifetime analysis must update the outgoing state accordingly.

---

## § 8 Reference lifetime

### § 8.1 Core bounds

**§ 8.1(1)** A safe reference cannot begin before its referent is initialized and available.

**§ 8.1(2)** A safe reference cannot remain usable after referent destruction, move, discard, incompatible replacement, deallocation, invalidating remap, generation/epoch invalidation, or loss of any required platform validity.

**§ 8.1(3)** A safe reference does not own or automatically extend the referent lifetime.

### § 8.2 Reference values may outlive semantic validity physically

**§ 8.2(1)** The representation bits of a reference may remain stored after semantic validity ends.

**§ 8.2(2)** Safe Sec must reject use after semantic invalidation even when an address still numerically points into allocated or mapped memory.

### § 8.3 Mutability does not extend lifetime

**§ 8.3(1)** `ref mut T` does not keep `T` alive merely because it carries exclusive access authority.

**§ 8.3(2)** A shared or mutable reference remains bounded by all lifetime and validity requirements of its referent.

---

## § 9 Borrow lifetime

### § 9.1 Non-lexical analysis

**§ 9.1(1)** Borrow live ranges are determined by actual uses and dependencies, not lexical scope alone.

**§ 9.1(2)** The compiler must use the narrowest provably correct borrow live range consistent with all surviving holders, reborrows, returned references, captures, deferred uses, views, and aggregate-contained references.

```sec
let value := CreateValue()
let view := ref value
Inspect(view)
Consume(<-value)
```

**§ 9.1(3)** The shared borrow may end after the final proven use of `view` when no derived dependency survives.

### § 9.2 Derived borrows

**§ 9.2(1)** A reborrow cannot outlive its source reference or the source borrow authority from which it derives.

**§ 9.2(2)** Moving a `ref mut` holder transfers that holder's borrow obligation rather than ending the underlying borrow.

**§ 9.2(3)** Copying a shared reference may keep the shared borrow live through the copied holder.

### § 9.3 Destruction and replacement

**§ 9.3(1)** Destruction, move, discard, replacement, deallocation, or invalidating remap is illegal while a conflicting live borrow still depends on the affected Place.

**§ 9.3(2)** Lifetime analysis must consume the canonical Place-overlap result from `borrowing.md`; it must not replace proven disjointness with root-only conservatism when the borrow analyzer already proved narrower Places.

---

## § 10 Temporaries and expression lifetimes

### § 10.1 Temporary lifetime

**§ 10.1(1)** A temporary value exists for the semantic region required to complete its containing expression and any ownership transfer, borrow, cleanup, or result-position use required by that expression.

**§ 10.1(2)** A temporary must not be silently extended beyond a safe bound solely to satisfy an escaping reference.

### § 10.2 Borrowing temporaries

**§ 10.2(1)** A reference to a temporary is invalid when the reference would outlive the temporary expression lifetime.

**§ 10.2(2)** Specialized rulebooks may reject explicit borrowing of non-addressable temporaries entirely even when a narrow physical lifetime could exist.

### § 10.3 Fresh returned values

**§ 10.3(1)** A fresh owned value returned by a function begins its receiving lifetime under the receiving owner without requiring a source move marker.

**§ 10.3(2)** This rule does not create a returned reference to the callee's local storage; owned return and borrowed return are distinct lifetime models.

---

## § 11 Control-flow and path-sensitive lifetime

### § 11.1 Branches

**§ 11.1(1)** Lifetime analysis is flow-sensitive and path-sensitive.

**§ 11.1(2)** `if`, `switch`, `select`, `match`, `try`, short-circuit control flow, and other branching constructs must preserve the lifetime facts of every continuing path.

**§ 11.1(3)** A path that terminates through `return`, propagated failure, panic/termination semantics, or another non-continuing edge does not constrain later code that cannot be reached from that path.

### § 11.2 Joins

**§ 11.2(1)** At a control-flow join, a reference/use is accepted only when its required validity is satisfied for every incoming path that can reach that use.

**§ 11.2(2)** Path-disjunctive provenance may be preserved as a finite compiler-owned set when that increases safe precision.

**§ 11.2(3)** When provenance or lifetime alternatives exceed a deterministic implementation bound, the compiler may conservatively widen to unknown and reject uses that require stronger proof.

### § 11.3 Loops

**§ 11.3(1)** Loop lifetime analysis must compute a fixed point over entry, normal fallthrough, `continue`, backedge, and relevant `break` states.

**§ 11.3(2)** Zero-iteration behavior must be represented unless the loop form proves at least one iteration.

**§ 11.3(3)** Iteration-local bindings begin fresh lifetimes for each iteration and must not be accidentally retained as one cross-iteration Place.

**§ 11.3(4)** References or borrows that would become invalid on a later iteration must be diagnosed at the operation that becomes invalid, with the causal earlier edge reported when practical.

---

## § 12 Aggregates, projections, and partial lifetimes

### § 12.1 Field-sensitive lifetime

**§ 12.1(1)** Aggregate lifetime analysis must be Place-sensitive where ownership and borrowing permit independent sub-Place reasoning.

**§ 12.1(2)** A field reference is bounded by the field value lifetime, its containing storage validity, and every owner/mapping/allocation bound required to reach that field.

### § 12.2 Partial moves

**§ 12.2(1)** A legal partial move ends the moved sub-Place's current value lifetime while leaving disjoint owned sub-Places live.

**§ 12.2(2)** Reinitializing the moved sub-Place starts a new value lifetime and may restore whole-aggregate availability when all required parts are live again.

**§ 12.2(3)** A type with custom `free` that forbids partial moves must be treated as one lifetime/destruction unit for ownership transfer according to the ownership/destruction rulebooks.

### § 12.3 Arrays, slices, and views

**§ 12.3(1)** Array-element and slice/view lifetimes derive from the underlying storage and element lifetimes, not from the descriptor's storage alone.

**§ 12.3(2)** A slice or view descriptor may be copied or moved without extending the lifetime of the referenced elements.

**§ 12.3(3)** Proven disjoint ranges may carry independent borrow constraints while sharing one allocation/storage lifetime.

### § 12.4 Unions and active variants

**§ 12.4(1)** A reference into a union payload is valid only while that payload remains the active semantic state and the owning union storage remains valid.

**§ 12.4(2)** Replacing the active variant ends references into the prior active payload even when the union storage address remains unchanged.

**§ 12.4(3)** `match` payload borrows are bounded by the resolved arm/guard lifetime and may not escape unless the general reference-return/capture rules prove a longer valid dependency.

---

## § 13 Function-call lifetime semantics

### § 13.1 Call preparation

**§ 13.1(1)** Before a call commits, the compiler must resolve relevant argument value lifetimes, borrow regions, ownership transfers, alias relationships, invalidation effects, escape/retention behavior, and returned-reference relationships.

**§ 13.1(2)** Argument evaluation order and failure during argument preparation must not prematurely commit an ownership or borrow state that the call never receives.

### § 13.2 By-value parameters

**§ 13.2(1)** A by-value parameter receives its own value according to copy/move rules.

**§ 13.2(2)** Passing a copyable value by value does not create a borrow relationship to the source merely because the backend later elides the copy.

**§ 13.2(3)** A normal by-value parameter must not silently become a consuming parameter because the argument type is move-only; consuming transfer is controlled by the explicit parameter/call-site ownership contract.

### § 13.3 Borrowed parameters

**§ 13.3(1)** A `ref` or `ref mut` parameter receives call-bounded borrowed authority unless the function's canonical contract explicitly retains or returns a derived reference.

**§ 13.3(2)** An opaque or imported function must not be assumed non-retaining merely because no body is available locally.

### § 13.4 Consuming parameters

**§ 13.4(1)** A `->` parameter transfers ownership according to ownership/copy-move rules and therefore ends the caller's source value lifetime when the consuming call commits.

**§ 13.4(2)** The mandatory caller-side `<-` marker is an ownership rule; lifetime analysis consumes the resulting transfer fact.

### § 13.5 Post-call update

**§ 13.5(1)** After a call, lifetime state must be updated from the resolved contract rather than reconstructed from return type alone.

---

## § 14 Returned references and exported lifetime relations

### § 14.1 General rule

**§ 14.1(1)** A returned safe reference must derive from storage whose validity extends beyond function return for the complete caller-visible reference lifetime.

**§ 14.1(2)** Returning a reference to ordinary local automatic storage is invalid.

```sec
fn Invalid() ref Value {
    let value := CreateValue()
    return ref value
}
```

**§ 14.1(3)** The compiler must reject this even if optimization could physically preserve the bytes.

### § 14.2 Parameter-derived result

**§ 14.2(1)** A returned reference derived from a borrowed parameter may be valid.

```sec
fn First(value: ref Pair) ref Item {
    return ref value.First
}
```

**§ 14.2(2)** The exported function metadata must record the dependency needed by the caller.

### § 14.3 Receiver-derived result

**§ 14.3(1)** A returned reference derived from `self` is bounded by the receiver's referent lifetime and any narrower field/storage validity.

### § 14.4 Multiple possible sources

**§ 14.4(1)** When a returned reference may derive from several input references, the exported relation must be representable conservatively for every path.

**§ 14.4(2)** Sec 0.1 should prefer a safe lifetime intersection or rejection when a more precise stable caller-visible relation cannot be represented.

### § 14.5 Owned parameter local lifetime

**§ 14.5(1)** Returning a reference into an owned by-value parameter is invalid unless ownership of backing storage also escapes in a representation that keeps the referent alive and the reference relation representable.

### § 14.6 Mutable returned reference

**§ 14.6(1)** Returning `ref mut` derived from a `ref mut` input may continue the exclusive borrow beyond call return.

**§ 14.6(2)** The caller's granting authority remains restricted for the returned mutable reference's live range.

### § 14.7 No invented lifetime syntax

**§ 14.7(1)** Unambiguous returned-reference relations must be inferred and exported in compiler/module metadata without source lifetime names.

**§ 14.7(2)** Bodyless declarations such as interfaces, extern declarations, and other opaque contracts may eventually require dedicated source contract syntax; this rulebook does not invent that syntax.

---

## § 15 Methods and receiver lifetime

### § 15.1 Borrowing receiver

**§ 15.1(1)** A method that borrows `self` may return a reference into `self` when the relation is representable and borrow authority remains valid.

### § 15.2 Mutable receiver

**§ 15.2(1)** A mutable method may invalidate references into fields it replaces, moves, destroys, or structurally invalidates.

**§ 15.2(2)** The compiler may use canonical method effect summaries to retain references to proven-disjoint stable sub-Places when safe.

### § 15.3 Whole-self consumption

**§ 15.3(1)** Ordinary Sec methods do not consume the whole `self`; whole-object terminal destruction is owned by lifecycle `free` and other explicit ownership boundaries defined by specialized rulebooks.

**§ 15.3(2)** A method may move or destroy an owned member when its receiver authority and ownership rules permit it; lifetime analysis must end references into that member while preserving proven-disjoint members.

---

## § 16 Closures, lambdas, and callable lifetimes

### § 16.1 Capture modes

**§ 16.1(1)** Lifetime analysis consumes the explicit capture mode resolved by the lambda/ownership rulebooks.

```sec
capture(value)
capture(<-value)
capture(ref value)
capture(ref mut value)
```

**§ 16.1(2)** An owned copied or moved capture lives according to the closure environment's owned lifetime.

**§ 16.1(3)** A borrowed capture never extends the captured referent lifetime.

### § 16.2 Non-escaping callable

**§ 16.2(1)** A callable proven by canonical contract to execute only during the current call may borrow local data whose lifetime covers that call.

**§ 16.2(2)** Public or opaque APIs must encode non-retention in compiler-visible contract metadata; the compiler must not infer it solely from a currently available implementation body.

### § 16.3 Escaping callable

**§ 16.3(1)** A callable that may be stored, returned, scheduled, retained, or invoked after the creation scope ends cannot borrow shorter-lived local data.

**§ 16.3(2)** Owned captures may permit such escape when closure-environment storage and destruction cover the required lifetime.

### § 16.4 Callable summaries

**§ 16.4(1)** Function/callable types and summaries must preserve lifetime-relevant retention and returned-reference relationships required for safe indirect calls.

**§ 16.4(2)** Erasing lifetime-relevant callable metadata at an indirect call boundary is unsound and forbidden for safe calls.

---

## § 17 Defer and delayed lifetime dependencies

### § 17.1 Deferred use extends required lifetime

**§ 17.1(1)** A value or reference used by a registered `defer` has a delayed dependency at least until that deferred action executes.

**§ 17.1(2)** The compiler must reject move, discard, destruction, replacement, deallocation, or other invalidation that would make the later deferred use invalid.

### § 17.2 Place sensitivity

**§ 17.2(1)** Lifetime analysis should preserve canonical Place precision for deferred dependencies rather than extending unrelated sibling fields or unrelated locals.

### § 17.3 Unified cleanup order

**§ 17.3(1)** Lifetime analysis must respect the unified cleanup ordering defined by `destruction.md` and `defer.md`; it must not end storage or references before a later registered cleanup that still needs them.

---

## § 18 Allocation and arena lifetimes

### § 18.1 Owned allocation

**§ 18.1(1)** An allocation lifetime begins when allocation succeeds and ends when the owning allocator/owner deallocates or invalidates the allocation according to its contract.

**§ 18.1(2)** References into an allocation are bounded by allocation validity and by any narrower object/value lifetime within that allocation.

**§ 18.1(3)** A `RawPtr[T]` does not extend allocation lifetime.

### § 18.2 Arena lifetime

**§ 18.2(1)** Arena-backed references and views cannot outlive the arena or the relevant arena generation/epoch.

**§ 18.2(2)** `Arena.Reset()` or equivalent canonical invalidation ends the validity of references tied to the prior generation even when the physical backing storage remains allocated.

**§ 18.2(3)** Branch and loop joins must conservatively merge generation state.

### § 18.3 Generation is not ownership availability

**§ 18.3(1)** Generation/epoch validity answers whether a reference still denotes the correct live storage incarnation.

**§ 18.3(2)** Ownership `is available` answers whether the current owner still owns a value in a Place.

**§ 18.3(3)** Neither concept substitutes for the other.

---

## § 19 Static, global, and thread-local storage

### § 19.1 Static lifetime

**§ 19.1(1)** Static or global storage may have program- or target-defined long lifetime, but long storage lifetime does not imply unrestricted safe access.

### § 19.2 Mutable global access

**§ 19.2(1)** References to mutable global/static storage remain subject to initialization, borrow authority, synchronization, target access, and shutdown lifetime rules.

**§ 19.2(2)** Lifetime analysis must not treat synchronization correctness as implied by long lifetime.

### § 19.3 Thread-local storage

**§ 19.3(1)** A reference into thread-local storage is additionally bounded by the execution/thread contract of that storage.

**§ 19.3(2)** Such a reference must not cross to another execution context unless a specialized rulebook proves that the storage identity and access remain valid there.

---

## § 20 Fixed-address external storage

### § 20.1 Fixed address is not unlimited lifetime

**§ 20.1(1)** A fixed physical address does not by itself prove permanent, initialized, owned, available, or safe storage.

**§ 20.1(2)** Lifetime analysis must consume the canonical storage-origin and lifetime/availability contract associated with the fixed-address binding.

### § 20.2 Address stability

**§ 20.2(1)** `AddressStability.Fixed` constrains relocation while the contract is valid; it does not create ownership or extend the lifetime contract.

### § 20.3 External ownership

**§ 20.3(1)** A fixed-address binding that refers to platform-owned storage does not become the owner merely because it has a source-level name.

**§ 20.3(2)** Safe references/views into such storage remain bounded by the platform contract and by any mapping/authority object that establishes validity.

---

## § 21 Runtime hardware mappings and register views

### § 21.1 Mapping ownership

**§ 21.1(1)** An owning runtime mapping resource owns the mapping lifetime when the platform API contract says it does.

**§ 21.1(2)** Destroying or moving that owner follows ownership/destruction rules and may end the mapping lifetime.

### § 21.2 Register views

**§ 21.2(1)** A typed register view derived from a mapping is non-owning unless a specialized platform contract says otherwise.

**§ 21.2(2)** Its lifetime is bounded by the mapping or canonical owner that establishes endpoint validity.

**§ 21.2(3)** A register view must not escape beyond that owner/mapping lifetime.

### § 21.3 Remapping

**§ 21.3(1)** A remap that changes physical realization must end or invalidate old views unless the platform contract proves that those views preserve identity and address stability.

**§ 21.3(2)** The compiler must not silently retarget a live safe view to different physical storage merely because the new mapping has compatible type/layout.

### § 21.4 Mapping lifetime versus device liveness

**§ 21.4(1)** Mapping lifetime and external device liveness are separate facts.

**§ 21.4(2)** A mapping may remain owned and structurally valid while the external device is unavailable.

**§ 21.4(3)** Device liveness must not be modeled as ownership `is available` or as ordinary safe-reference lifetime unless a specific platform contract explicitly couples them.

### § 21.5 No hidden mapping manager

**§ 21.5(1)** Lifetime support for hardware mappings must not require a global Sec mapping registry, garbage collector, runtime ownership table, or background lifetime service.

---

## § 22 Volatile and physical access effects

### § 22.1 Volatility is not lifetime

**§ 22.1(1)** Volatile semantics govern observable physical storage access and do not by themselves extend or shorten the lifetime of a Sec value/reference.

**§ 22.1(2)** A value read from volatile storage becomes an ordinary Sec value snapshot whose later copy/move lifetime is governed by ordinary memory rules.

### § 22.2 External mutation

**§ 22.2(1)** External hardware may mutate volatile storage while Sec owns or borrows the software-side mapping capability.

**§ 22.2(2)** Such hardware mutation is not itself a Sec ownership transfer or borrow conflict, though hardware/platform rulebooks may impose separate access ordering or coherence constraints.

### § 22.3 Observable access cannot be optimized from lifetime alone

**§ 22.3(1)** Lifetime facts must not be used to remove, merge, speculate, duplicate, or reorder volatile/hardware accesses in ways forbidden by the physical-access contract.

---

## § 23 Raw pointers, unsafe, and FFI

### § 23.1 RawPtr

**§ 23.1(1)** `RawPtr[T]` carries no automatic safe lifetime, ownership, non-null, bounds, borrow, or retention guarantee beyond what its creating unsafe/foreign contract establishes.

**§ 23.1(2)** Copying a RawPtr does not extend the lifetime of pointed-to storage.

**§ 23.1(3)** Converting RawPtr to a safe reference requires proof of the complete safe-reference lifetime and authority obligations defined by raw-pointer/reference rules.

### § 23.2 FFI contracts

**§ 23.2(1)** Foreign APIs require explicit compiler-visible lifetime/retention contracts for safe wrappers.

**§ 23.2(2)** Relevant distinctions include call-bounded borrow, retained pointer, returned owned pointer, returned borrow from input, static foreign storage, next-call validity, explicit-release validity, callback retention, and thread-local foreign storage.

**§ 23.2(3)** C/C++ pointer types alone are insufficient proof of these relationships.

### § 23.3 Safe wrapper obligation

**§ 23.3(1)** A safe wrapper must translate foreign lifetime behavior into Sec ownership/reference semantics and keep unresolved foreign uncertainty inside explicit unsafe code.

**§ 23.3(2)** A foreign-retained pointer must not derive from ordinary local stack storage unless the foreign use is proven to end before that storage ends.

---

## § 24 Escaping execution contexts

### § 24.1 General escape rule

**§ 24.1(1)** Any callable, reference, view, pointer contract, or captured dependency that may be used after the current call returns is an escaping lifetime use.

**§ 24.1(2)** Escaping use requires backing storage and ownership/capture semantics that cover the complete possible execution/retention contract.

### § 24.2 Tasks, threads, callbacks, event handlers

**§ 24.2(1)** Task queues, worker threads, retained callbacks, event registrations, foreign callbacks, and similar deferred execution contexts must not borrow ordinary call-local storage beyond its valid lifetime.

**§ 24.2(2)** Owned capture/transfer may be used when the destination execution object's ownership and destruction contract safely outlives the use.

### § 24.3 Concurrency is separate

**§ 24.3(1)** Sufficient lifetime does not imply synchronization, race freedom, thread transferability, ISR safety, or access-context legality.

**§ 24.3(2)** Those properties are checked by their specialized rulebooks/analyses in addition to lifetime analysis.

---

## § 25 Interrupt boundary

**§ 25(1)** Interrupt-specific source syntax and execution semantics are intentionally outside this rulebook.

**§ 25(2)** An interrupt registration or handler capture that may execute after its registration call is an escaping execution context under § 24.

**§ 25(3)** Ordinary local automatic references must not escape into such a handler unless the future interrupt specification defines a concrete execution/lifetime arrangement that proves them valid.

**§ 25(4)** Static/global/fixed-address storage may still require synchronization, volatile/hardware semantics, execution-context authority, and interrupt-safety rules; long lifetime alone is insufficient.

**§ 25(5)** The interrupt rulebook may impose stricter constraints but must not weaken the general lifetime invariants in this rulebook.

---

## § 26 Error handling and lifetime

### § 26.1 `try` paths

**§ 26.1(1)** `try` and fallible operations create ordinary control-flow edges for lifetime analysis.

**§ 26.1(2)** An unmatched failure that propagates leaves the current continuation and therefore does not constrain later code on the success continuation except through cleanup that executes during propagation.

**§ 26.1(3)** A handler that continues must establish a lifetime-valid recovery state for every value/reference it returns or leaves live.

### § 26.2 Handler boundary

**§ 26.2(1)** Failures or lifetime effects originating inside a handler body belong to that handler's ordinary control flow and are not retroactively part of the protected operation's lifetime region.

### § 26.3 Result and Option payloads

**§ 26.3(1)** References stored in `Result`, `Option`, or other payload-bearing unions remain bounded by their referent/storage lifetimes and by the active payload lifetime.

**§ 26.3(2)** Consuming `.Ok()` / `.Err()` ends the consumed Result value lifetime at its source according to ownership rules; any borrowed projection/accessor must remain bounded by the Result and underlying payload validity.

---

## § 27 Generics and monomorphization

### § 27.1 Generic constraints

**§ 27.1(1)** Generic code must be lifetime-safe for every concrete instantiation admitted by its type/ownership/borrow contracts.

**§ 27.1(2)** Monomorphization may improve precision using concrete field layout, copyability, destruction, storage, and reference provenance facts.

### § 27.2 No hidden source lifetime parameters

**§ 27.2(1)** Compiler-internal lifetime relations may be parameterized or symbolic without exposing programmer-written lifetime parameters in Sec 0.1.

### § 27.3 Separate compilation

**§ 27.3(1)** Exported generic/function/interface metadata must preserve lifetime-relevant relationships needed by callers after the original body is unavailable.

---

## § 28 Separate compilation and summaries

### § 28.1 Required summaries

**§ 28.1(1)** Public/importable callable metadata must preserve every caller-relevant lifetime relationship that cannot be reconstructed safely from the signature's ordinary types alone.

**§ 28.1(2)** Such metadata may include:

```text
returned-reference origin relation
receiver-derived relation
parameter-derived relation
finite multi-source relation
call retention / non-retention
captured dependency escape
invalidation effects
mapping/view dependency
storage-domain dependency
generation/epoch dependency
```

### § 28.2 Determinism

**§ 28.2(1)** Lifetime summaries must be deterministic and independent of source traversal order, map iteration order, worker scheduling, or declaration ordering.

### § 28.3 Recursive calls

**§ 28.3(1)** Recursive and mutually recursive summary inference must use a terminating fixed-point strategy with conservative widening where required.

---

## § 29 Lifetime inference model

### § 29.1 Constraint generation

**§ 29.1(1)** Lifetime analysis may internally generate region/validity constraints such as:

```text
reference R must not outlive target Place P
reborrow B must not outlive source reference S
borrow B must cover every use of holder H
returned reference R derives from parameter P
closure C must not outlive captured reference R
aggregate A must not outlive reference field F
mapping view V must not outlive mapping M
value V must remain live through deferred use D
```

### § 29.2 Constraint direction

**§ 29.2(1)** The compiler must preserve correct constraint direction; reversing outlives relations is a correctness bug.

### § 29.3 Conservative rejection

**§ 29.3(1)** If no sound conservative lifetime assignment or representable caller contract exists, the program must be rejected rather than repaired through hidden allocation, hidden retention, or unsound metadata.

### § 29.4 Internal regions are not source features

**§ 29.4(1)** Compiler-internal regions, lifetime variables, graph nodes, SCCs, and dataflow lattices are implementation techniques and do not create source-visible lifetime syntax.

---

## § 30 Semantic IR requirements

### § 30.1 Facts before lowering

**§ 30.1(1)** Before lifetime-sensitive source semantics can be erased, Semantic IR must preserve enough resolved facts to verify and lower:

```text
storage origin and domain
Place/provenance relation
reference/reborrow relation
borrow begin/end or equivalent live-range fact
ownership transfer and invalidation
replacement/reinitialization
allocation/mapping owner dependencies
generation/epoch constraints
returned-reference summaries
closure capture lifetime/escape
FFI retention facts
defer-delayed dependencies
partial/conditional availability interaction
```

### § 30.2 Verifier

**§ 30.2(1)** Semantic IR verification must reject impossible or contradictory lifetime states rather than relying on later backends to infer source semantics.

### § 30.3 High-level erasure

**§ 30.3(1)** Lifetime facts may be erased progressively only after all later stages that require them receive equivalent constraints or resolved plans.

---

## § 31 MLIR and backend lowering

### § 31.1 Preservation

**§ 31.1(1)** Lowering must preserve storage validity, destruction order, borrow-dependent alias constraints, deallocation ordering, returned-reference validity, volatile/hardware access semantics, address stability, and mapping/view invalidation.

### § 31.2 LLVM lifetime metadata

**§ 31.2(1)** LLVM `lifetime.start` / `lifetime.end` or equivalent backend metadata are optimization hints derived from proven Sec semantics; they do not define Sec lifetimes.

**§ 31.2(2)** The compiler must not emit an early backend lifetime-end marker while any Sec reference, borrow, defer, cleanup, mapping/view dependency, or observable requirement still needs the storage.

### § 31.3 No over-strong alias assumptions

**§ 31.3(1)** Lifetime/borrow facts may inform `noalias` or related optimization only to the degree proven by canonical Sec borrow/provenance analysis.

**§ 31.3(2)** Unknown foreign aliases, volatile storage, hardware aliases, RawPtr, and opaque calls must not receive stronger assumptions than their contracts justify.

---

## § 32 Diagnostics

### § 32.1 Mentor principle

**§ 32.1(1)** Lifetime diagnostics must explain the programmer-visible cause rather than only compiler-theory terminology.

**§ 32.1(2)** A useful diagnostic should identify, when available:

```text
the reference/view/value that is invalid
where its dependency originated
which operation ended or may end validity
which later use is rejected
the relevant path or branch
one practical safe restructuring
```

### § 32.2 Examples of mentor explanations

**§ 32.2(1)** For a returned local reference, diagnostics should say that the referenced local stops existing when the function returns and suggest returning an owned value or borrowing caller-owned storage.

**§ 32.2(2)** For use after arena reset, diagnostics should name the reset/generation change and suggest reacquiring a reference after reset.

**§ 32.2(3)** For register-view escape, diagnostics should identify the mapping whose destruction/unmap bounds the view.

**§ 32.2(4)** For a deferred use, diagnostics should identify the `defer` registration/use that requires the value to remain live.

### § 32.3 Required safety diagnostics are errors

**§ 32.3(1)** A proven lifetime-safety violation is a compile error and is not downgraded by lint/advisory severity configuration.

---

## § 33 LSP and tooling

### § 33.1 Shared semantic facts

**§ 33.1(1)** LSP lifetime information must consume compiler/Sema facts rather than reimplement lifetime inference independently.

### § 33.2 Hover and navigation

**§ 33.2(1)** Tooling may expose origin, owner, borrow dependency, returned-reference source, generation/epoch, mapping owner, and invalidation cause where useful.

**§ 33.2(2)** Tooling should avoid exposing internal region IDs as the primary user model.

### § 33.3 Code actions

**§ 33.3(1)** A lifetime code action must be offered only when the compiler can prove the transformation preserves semantics and safety.

**§ 33.3(2)** Tooling must not automatically add hidden allocation, convert arbitrary borrowed data to owned copies, or weaken hardware/volatile semantics merely to silence a lifetime error.

---

## § 34 Required implementation phases

**§ 34(1)** A conforming implementation requires a coherent vertical slice across frontend/Sema, interprocedural summaries, Semantic IR, lowering, diagnostics, and tooling.

**§ 34(2)** Parser work is minimal because lifetime analysis introduces no general new source lifetime syntax.

**§ 34(3)** Frontend implementation must use canonical Place, ownership, borrowing, destruction, storage, and escape facts rather than parallel incompatible lifetime bookkeeping.

**§ 34(4)** Interprocedural implementation must persist/version exported lifetime summaries for separate compilation.

**§ 34(5)** Semantic IR must preserve lifetime relations long enough to support verified lowering.

**§ 34(6)** Backend lowering must not infer ownership/lifetime source semantics from unused SSA values, machine addresses, or ABI coincidence.

---

## § 35 Required tests

### § 35.1 Local value and borrow tests

**§ 35.1(1)** Tests must cover:

```text
borrow ends after final proven use
borrow survives through copied shared-reference holder
ref mut move transfers holder obligation
reborrow cannot outlive source reference
move while overlapping borrow is live is rejected
replacement while overlapping borrow is live is rejected
destruction while overlapping borrow is live is rejected
disjoint field borrow permits unrelated sibling lifetime end
```

### § 35.2 Return/reference tests

**§ 35.2(1)** Tests must cover:

```text
returned reference to local rejected
returned reference to temporary rejected
returned reference derived from one borrowed parameter accepted
returned reference derived from receiver accepted
mutable returned reference preserves exclusive borrow
shared input cannot produce ref mut result
multi-source returned reference uses conservative representable relation
returned reference metadata survives separate compilation
```

### § 35.3 Aggregate tests

**§ 35.3(1)** Tests must cover:

```text
reference-containing struct cannot outlive referent
partial move ends only moved field lifetime
reinitialization starts a new field lifetime
union payload reference ends on variant replacement
slice/view lifetime follows underlying storage
aggregate whole-value transfer preserves contained reference dependencies
```

### § 35.4 Control-flow tests

**§ 35.4(1)** Tests must cover:

```text
branch-specific lifetime and provenance merge
terminated branch does not poison later continuation
loop fixed point catches later-iteration invalidation
break and continue lifetime states
zero-iteration preservation
finite path-disjunctive provenance and deterministic conservative widening
```

### § 35.5 Defer/error tests

**§ 35.5(1)** Tests must cover:

```text
defer extends required value/reference lifetime
move before deferred use rejected
cleanup ordering does not end storage before defer use
try success and failure paths maintain valid cleanup/lifetime state
handler recovery value cannot contain escaping local reference
return try does not bypass returned-reference validation
```

### § 35.6 Allocation/generation tests

**§ 35.6(1)** Tests must cover:

```text
arena-backed reference valid before reset
arena-backed reference invalid after reset
generation state merged across branches and loops
RawPtr does not extend allocation lifetime
deallocation rejected while safe reference remains live
```

### § 35.7 Hardware/fixed-address tests

**§ 35.7(1)** Tests must cover:

```text
fixed address alone does not imply unlimited lifetime
runtime register view cannot outlive mapping
mapping move transfers owner without retargeting existing view identity
mapping destruction invalidates dependent views
remap invalidates old views unless contract proves stability
device liveness remains separate from mapping lifetime and is available
volatile snapshot becomes ordinary Sec value
volatile access is not removed/reordered from lifetime optimization
```

### § 35.8 Closure/callback/FFI tests

**§ 35.8(1)** Tests must cover:

```text
non-escaping closure may borrow call-local data
escaping closure cannot borrow shorter-lived local
owned capture may outlive creator when closure owns storage
foreign call-bounded borrow accepted by explicit contract
foreign-retained local reference rejected
foreign returned borrow respects input/static contract
opaque callback retention remains conservative
```

### § 35.9 IR/lowering tests

**§ 35.9(1)** Tests must cover:

```text
Semantic IR preserves reference origin and returned relation
Semantic IR preserves borrow/reborrow dependency
Semantic IR preserves mapping/view dependency
no early backend lifetime.end
no hidden heap promotion
no over-strong noalias from uncertain provenance
cleanup/deallocation follows lifetime proof
```

---

## § 36 Final invariants

**§ 36(1)** Before safe Semantic IR/lowering completes, the compiler must be able to justify all of the following for every relevant path:

```text
every safe reference has a known or conservatively bounded origin
every safe reference use occurs while the referent value is live
every safe reference use occurs while required storage is valid
every borrow is live for all dependent uses and no longer than required
every move, discard, replacement, destruction, reset, deallocation, or remap respects live references and borrows
every returned reference has a caller-visible safe dependency relation
every escaped closure/callback dependency has sufficient backing lifetime
every reference-containing aggregate is lifetime-bounded by its contents
every arena/generation dependency is valid
every hardware register view is bounded by its mapping/owner contract
every fixed-address reference consumes a real platform lifetime contract rather than assuming permanence
every FFI lifetime obligation is represented or remains inside unsafe code
no hidden heap promotion or runtime borrow tracking is required to make safe code appear valid
```

**§ 36(2)** When any invariant cannot be proven conservatively, safe Sec compilation must reject the affected program rather than guess.
