# References

- Status: Normative
- Created: 2026-08-29
- Last updated: 2026-08-29
- Document revision: 2.0
- Sec language version: 0.1
- Canonical path: `rules/memory/references.md`
- Replaces: `rules/memory/references.txt`
- Repository baseline reviewed: `fafd8cb`

---

## § 1 Purpose and authority

§ 1(1) This rulebook defines the source-level semantics of safe Sec references.

§ 1(2) A safe reference provides temporary, non-owning access to an existing live value or storage location.

§ 1(3) A safe reference never transfers ownership merely because it is created, copied where permitted, passed, returned, stored, or compared.

§ 1(4) `rules/memory/reference_model.md` owns the complete semantic reference model, including storage identity, validity epochs, generation checks, relocation correctness, stable handles, weak handles, profile-selected representations, and stale-reference failure behavior.

§ 1(5) `rules/memory/borrowing.md` owns shared and mutable borrow authority, overlap, reborrowing, borrow live ranges, and conflicts.

§ 1(6) `rules/memory/lifetime_analysis.md` owns lifetime proof, escaping references, returned-reference relationships, storage-lifetime bounds, and interprocedural lifetime summaries.

§ 1(7) `rules/memory/ownership.md` owns ownership and Place availability.

§ 1(8) `rules/memory/copy_move.md` owns copy/move classification and explicit transfer syntax.

§ 1(9) `rules/memory/raw_pointers.md` owns raw-pointer operations and the unsafe boundary to safe references.

§ 1(10) `rules/platform/fixed-address-bindings.md`, `rules/platform/hardware-register-access.md`, and `rules/platform/volatile.md` own physical-address, hardware-storage, mapping, access-context, and volatile semantics.

§ 1(11) `rules/platform/interrupts.md` owns interrupt execution contexts. It does not define a separate reference type or interrupt-specific memory model.

§ 1(12) When this rulebook conflicts with an older reference rule, the newer v2 memory, platform, and interrupt rulebooks named above take precedence.

---

## § 2 Core invariants

§ 2(1) A safe reference must never be usable beyond the complete validity of the value and storage it references.

§ 2(2) A safe reference must never grant more access authority than the source Place or source reference permits.

§ 2(3) A safe reference must preserve valid provenance to one or more compiler-known live storage origins.

§ 2(4) A safe reference must remain spatially valid for every access performed through it.

§ 2(5) Safe-reference validity is not established by numeric address equality alone.

§ 2(6) Generation or epoch validity does not replace borrowing, bounds, initialization, ownership, address-space, access-context, or concurrency proof.

§ 2(7) Reference creation must not silently allocate memory.

§ 2(8) Reference creation must not extend the lifetime of the referenced object.

§ 2(9) A safe reference must not be used as an implicit ownership handle.

§ 2(10) If the compiler cannot prove or preserve the guarantees required for a safe reference under the selected target/profile, the safe operation must be rejected or require an explicit unsafe/raw boundary.

---

## § 3 Reference kinds

### § 3.1 Shared reference

§ 3.1(1) `ref T` is a safe, non-null, typed shared reference to one valid `T`.

§ 3.1(2) `ref T` grants read authority only.

§ 3.1(3) Multiple compatible shared references may coexist according to `borrowing.md`.

§ 3.1(4) `ref T` does not imply that the underlying storage is globally immutable.

§ 3.1(5) Mutation through another access path remains subject to ordinary borrow, ownership, concurrency, hardware-access, and synchronization rules.

Example:

```sec
fn Inspect(value: ref Packet) uint32 {
    return value.Id
}
```

### § 3.2 Mutable reference

§ 3.2(1) `ref mut T` is a safe, non-null, typed reference with exclusive mutable authority for its borrow live range.

§ 3.2(2) `ref mut T` grants read and write authority where the referenced storage itself is writable.

§ 3.2(3) `ref mut T` requires exclusivity according to `borrowing.md`.

§ 3.2(4) A mutable reference is not ownership.

§ 3.2(5) Moving a mutable-reference binding transfers the reference holder and its active granting-borrow obligation; it does not transfer ownership of the referenced value.

Example:

```sec
fn Increment(value: ref mut int) void {
    value += 1
}
```

### § 3.3 Slice and bounded views

§ 3.3(1) A safe slice or bounded reference view is a non-owning reference category whose complete valid range is part of its semantic contract.

§ 3.3(2) A safe bounded view must not access outside its valid range.

§ 3.3(3) An empty slice may use a target-specific null or sentinel physical base representation only as a hidden implementation detail when its length is zero and no safe scalar reference is fabricated from that base.

§ 3.3(4) Hidden empty-slice representation does not make ordinary safe references nullable.

---

## § 4 Nullability and optional references

§ 4(1) `ref T` and `ref mut T` are never null as source-level values.

§ 4(2) Optionality is expressed explicitly.

Example:

```sec
let current: Option[ref Device]
```

§ 4(3) `None` means that an `Option` contains no reference value.

§ 4(4) `None` must not be used to represent ownership availability, stale generations, hardware-device liveness, or invalid raw addresses.

§ 4(5) A stale ordinary safe reference is a violated safety guarantee, not an ordinary `None` state.

---

## § 5 Creating references

§ 5(1) A safe reference may be created only when the source Place or source reference is valid for the requested access.

§ 5(2) Creating `ref` from an owned Place does not consume that Place.

§ 5(3) Creating `ref mut` does not consume ownership but establishes exclusive mutable borrow authority.

§ 5(4) A Place that is `Unavailable` cannot be referenced.

§ 5(5) A `ConditionallyAvailable` Place may be referenced only on a path where ownership analysis has refined that Place to `Available`.

§ 5(6) `is available` is an ownership test. It does not itself establish borrow compatibility, reference generation validity, hardware access permission, or synchronization.

Example:

```sec
if package.Payload is available {
    let payload := ref package.Payload
    Use(payload)
}
```

§ 5(7) Reference creation from a shared reference may retain or narrow shared authority.

§ 5(8) Reference creation from a mutable reference may create a compatible reborrow according to `borrowing.md`.

§ 5(9) Safe derivation must never upgrade shared authority to mutable authority.

---

## § 6 Provenance and storage origin

§ 6(1) Every safe reference has semantic provenance identifying the storage origin or finite set of possible origins that can back the reference.

§ 6(2) Provenance is a compiler semantic fact and need not be stored in the runtime representation when statically proven.

§ 6(3) Provenance may include:

- canonical Place roots and projections;
- caller-owned parameters;
- static/global storage;
- arena or allocation identities;
- fixed-address storage identities;
- runtime mapping identities;
- returned-reference source relations;
- active union/Result variant payload Places;
- other canonical storage identities defined by the reference model.

§ 6(4) Field, index, slice, and other safe projections preserve or narrow provenance.

§ 6(5) A reference must not acquire provenance merely because a raw numeric address happens to equal a live address.

§ 6(6) When control flow produces multiple possible safe origins, the compiler may retain a finite path-disjunctive origin set.

§ 6(7) Every possible origin in such a set must satisfy the lifetime, borrow, type, spatial, address-space, and execution-context requirements of the use.

§ 6(8) If a required safe proof degrades to unknown provenance, the compiler must reject operations that require that missing proof rather than inventing runtime provenance.

§ 6(9) The source language does not require programmer-written provenance annotations for ordinary safe code.

---

## § 7 Lifetime relationship

§ 7(1) A reference may not outlive the storage and live value on which it depends.

§ 7(2) A reference may have a shorter lifetime than its referenced owner.

§ 7(3) Reference lifetime is derived by `lifetime_analysis.md`; this rulebook does not introduce source lifetime parameters.

§ 7(4) Reference lifetime may end at final proven use rather than at the lexical end of the surrounding scope.

§ 7(5) Storing a reference in another value does not extend the referent lifetime.

§ 7(6) Copying a shared-reference holder does not extend the underlying storage lifetime; every surviving holder remains bounded by the same valid origin.

§ 7(7) Moving a mutable-reference holder transfers the holder's lifetime and borrow obligations to the destination.

§ 7(8) Destruction, deallocation, arena reset, remap, relocation, replacement, or another invalidating operation must not occur while a live safe reference still requires the invalidated storage, unless the canonical reference model provides an equivalent validity-preserving mechanism.

---

## § 8 Reference parameters

§ 8(1) Reference mutability is part of the function signature.

§ 8(2) A `ref T` parameter borrows from the caller for the call-bounded relationship or for any longer relationship explicitly proven by the callable contract.

§ 8(3) A `ref mut T` parameter receives exclusive mutable borrow authority for the required live range.

§ 8(4) Passing a reference parameter does not transfer ownership of the referenced value.

§ 8(5) Passing a `ref mut` holder to a borrowed parameter reborrows it when required by `borrowing.md`; it does not implicitly consume the holder.

§ 8(6) A callable must not retain a passed reference beyond the proven contract.

§ 8(7) A bodyless, foreign, indirect, or separately compiled callable whose retention behavior is not sufficiently known must be treated conservatively.

---

## § 9 Returning references

§ 9(1) Returning a reference is valid only when every possible returned origin remains valid after the callee returns.

§ 9(2) Returning a reference to ordinary callee-local function storage is invalid.

Example:

```sec
fn Invalid() ref int {
    let value := 42
    return ref value
}
```

§ 9(3) Returning a reference derived from caller-owned input is permitted when the compiler can prove the returned relationship.

Example:

```sec
fn Identity(value: ref Item) ref Item {
    return value
}
```

§ 9(4) Returning a reference derived from a receiver is permitted when the receiver-backed lifetime outlives the returned reference.

§ 9(5) Returning one of several possible caller-owned origins is permitted when the compiler can represent and prove the complete finite returned-origin relationship.

§ 9(6) Multiple possible origins are not invalid merely because they are distinct; every candidate must individually satisfy the required lifetime and borrow constraints.

§ 9(7) If a returned-reference relationship cannot be proven for every candidate origin, the return is invalid.

§ 9(8) Returned-reference summaries may describe parameters, receiver origins, projections, finite multi-source sets, and aggregate result paths.

§ 9(9) Separate compilation must preserve sufficient validated returned-reference metadata before imported summaries may be used as positive proof.

§ 9(10) Unknown or stale summary data must never be treated as proof of a safe returned reference.

---

## § 10 References inside aggregates and variants

§ 10(1) A struct, fixed array, union payload, `Option`, `Result`, tuple-like internal value, or other aggregate may contain safe references only while their referenced origins remain valid.

§ 10(2) Moving or copying a reference-containing aggregate preserves the contained reference provenance according to the copy/move rules of each contained reference kind.

§ 10(3) Returning an aggregate that contains a reference is subject to the same escape and lifetime rules as returning that reference directly.

§ 10(4) Storing a reference to callee-local storage into caller-owned aggregate storage is invalid when the reference could escape the local lifetime.

§ 10(5) A branch-scoped `ref` or `ref mut` payload binding from `match` must not escape the branch unless the compiler proves a valid independent origin relationship.

§ 10(6) An aggregate does not become an owner of referenced storage merely because it stores references.

---

## § 11 Copying and moving reference values

§ 11(1) A shared `ref T` value may be copied when its reference kind is copyable under `copy_move.md`.

§ 11(2) Copying a shared reference creates another reference value with equivalent provenance and validity dependencies; it does not copy the referent.

§ 11(3) A mutable `ref mut T` holder must not be implicitly copied.

§ 11(4) A named reusable mutable-reference holder that is moved must use the explicit move syntax required by `copy_move.md`.

Example:

```sec
let next :<- current
```

§ 11(5) Moving a reference never moves the referenced object merely because the reference holder moved.

§ 11(6) Destruction of a non-owning reference holder never destroys the referent.

---

## § 12 Subreferences and narrowing

§ 12(1) A reference safely derived from another safe reference may retain or narrow authority, bounds, lifetime, and provenance.

§ 12(2) A field subreference preserves the source storage identity and narrows the referenced Place.

§ 12(3) A constant-index subreference preserves the source storage identity and narrows the referenced element Place.

§ 12(4) A dynamic-index or range-derived reference must preserve safe bounds and conservative overlap semantics.

§ 12(5) A subreference must never reconstruct unrestricted access to a wider containing allocation through safe language semantics.

§ 12(6) A derived shared reference cannot become mutable through safe derivation.

§ 12(7) A derived reference must not outlive its source reference when the source reference is the lifetime-limiting authority.

---

## § 13 Bounds and spatial validity

§ 13(1) Every safe reference access must remain within the spatial extent authorized by the reference.

§ 13(2) For `ref T`, valid access is limited to the referenced `T` and valid subobjects derived from it.

§ 13(3) For a slice, valid element indices are the slice's defined bounds.

§ 13(4) Spatial validity and temporal validity are independent requirements.

§ 13(5) A live allocation does not make an out-of-bounds reference valid.

§ 13(6) A valid index does not make a stale reference valid.

§ 13(7) Bounds proof may be static or may use language-defined checked access according to the relevant collection/shaped-type rules.

---

## § 14 Storage identity and generations

§ 14(1) Safe-reference validity depends on live storage identity, not only numeric address.

§ 14(2) A validity epoch or generation may participate in the reference contract for storage domains that can be invalidated and reused.

§ 14(3) A generation belongs to an invalidation domain, not merely to an address.

§ 14(4) Arena reset, slot reuse, collection storage replacement, remapping, owner replacement, and similar operations may create new storage incarnations.

§ 14(5) A stale reference must not become valid merely because a new object later occupies the same numeric address.

§ 14(6) Runtime generation metadata is not mandatory when the compiler can prove validity statically.

§ 14(7) Where the selected profile requires a runtime generation or epoch check, that check preserves reference-model semantics and does not turn the reference into an optional value.

§ 14(8) Ordinary safe code is expected not to intentionally operate on stale ordinary references.

§ 14(9) A detected stale ordinary safe reference follows the deterministic panic/trap semantics defined by `reference_model.md`; ordinary dereference does not become `try`-based business logic.

---

## § 15 Relocation and pinning

§ 15(1) A direct safe reference must continue to identify the correct storage if the referenced object relocates.

§ 15(2) If the implementation cannot preserve a live direct reference through relocation, relocation is forbidden while that dependency remains live.

§ 15(3) A stable-handle mechanism may preserve identity through relocation according to `reference_model.md`.

§ 15(4) Pinning is a storage/address-stability property, not ownership.

§ 15(5) Pinning does not imply copyability, thread safety, permanent lifetime, allocation, or reference counting.

§ 15(6) The source syntax for general pinning remains outside this rulebook until canonically defined elsewhere.

---

## § 16 Fixed-address and hardware-backed references

§ 16(1) A fixed numeric or symbolic physical address does not by itself prove unlimited lifetime, validity, ownership, or safe reference construction.

§ 16(2) Safe references to fixed-address storage require the canonical storage/lifetime/availability contract defined by platform rules.

§ 16(3) Hardware-register access permission is not inferred merely from having a `ref` or `ref mut`.

§ 16(4) Volatile access semantics are separate from reference ownership and lifetime semantics.

§ 16(5) A runtime mapping may own mapping lifetime while typed register views borrow from that mapping.

§ 16(6) A typed register view must not outlive the mapping or platform contract that validates it.

§ 16(7) Remapping may invalidate prior views when semantic identity or address stability is not canonically preserved.

§ 16(8) External device liveness is distinct from reference lifetime and Place ownership availability.

§ 16(9) `is available` must not be used as a substitute for device-presence, hardware-ready, mapped, powered, or bus-valid state.

§ 16(10) Physical hardware storage does not become an ordinary Sec-owned movable object merely because a typed reference or view can access it.

---

## § 17 Interrupt execution contexts

§ 17(1) Interrupt execution uses the ordinary Sec safe-reference model.

§ 17(2) `@isr` and `@interrupt` do not introduce a special pointer/reference type.

§ 17(3) Every reference used in ISR execution must remain valid for the complete actual ISR use and for any synchronous call reached from that ISR.

§ 17(4) A reference to handler-local stack storage must not escape the handler execution lifetime.

§ 17(5) A reference captured or retained for deferred work after interrupt return must be backed by storage whose lifetime and ownership contract covers that later execution.

§ 17(6) A deferred-work handoff must not smuggle a borrow to ISR-local storage into another execution context.

§ 17(7) References to static, fixed-address, caller-provided, preallocated, or already-owned storage are permitted in ISR execution only when their ordinary lifetime, borrowing, concurrency, access-context, and platform contracts are valid.

§ 17(8) Interrupt preemption, nesting, and reentrancy do not themselves invalidate a reference, but they may create concurrency or alias conflicts that make an access invalid.

§ 17(9) Volatile access is not synchronization between ISR and non-ISR contexts.

§ 17(10) A reference that is temporally valid may still be illegal in an ISR because the reachable operation violates ISR access-context, runtime, synchronization, or hardware-access rules.

§ 17(11) ISR stack-domain rules are owned by `interrupts.md` and stack analysis; this rulebook only requires that references into stack storage never outlive the relevant stack frame and execution lifetime.

---

## § 18 Closures, callbacks, and escaping execution

§ 18(1) A closure that captures a reference retains the captured reference's lifetime dependency.

§ 18(2) A closure may escape its creator only when every captured reference remains valid for the complete closure use.

§ 18(3) Returning a closure that captures a reference to ordinary creator-local storage is invalid.

§ 18(4) Passing a reference through a callback is call-bounded only when the callback/foreign contract proves that it is not retained.

§ 18(5) Unknown retention is conservative.

§ 18(6) Task, thread, event, deferred-work, callback, interrupt-registration, and other later-execution paths are escaping uses when they may run after the creating call returns.

§ 18(7) The compiler must not repair an illegal reference escape by silently promoting local storage to the heap.

---

## § 19 Concurrency and shared state

§ 19(1) Reference validity does not imply race freedom.

§ 19(2) Generation checks do not substitute for synchronization.

§ 19(3) `ref mut` exclusivity remains a borrow-authority requirement even when the target storage uses generation metadata.

§ 19(4) A shared `ref` does not authorize unsynchronized concurrent mutation through another execution context.

§ 19(5) Concurrent invalidation requires a valid canonical reclamation/synchronization protocol when invalidation can race with access.

§ 19(6) ISR masking is synchronization only where the concurrency/platform analysis proves it excludes every conflicting execution context relevant to the access.

§ 19(7) Local interrupt masking is not universal synchronization for other cores or DMA.

---

## § 20 Reference equality

§ 20(1) Safe-reference equality compares semantic referenced identity, not ownership.

§ 20(2) Two safe references compare equal when they identify the same live storage identity and the same referenced location within that storage.

§ 20(3) Numeric address equality alone is insufficient when storage identities differ.

§ 20(4) Two references to different live incarnations that reuse one numeric address do not compare equal.

§ 20(5) Raw-pointer equality remains raw-address equality according to raw-pointer and target rules and does not imply safe-reference equality.

---

## § 21 Raw-pointer boundary

§ 21(1) Implicit conversion between safe references and `RawPtr[T]` is forbidden.

§ 21(2) Converting a safe reference to a raw pointer requires the explicit unsafe/FFI mechanism defined by the raw-pointer and FFI rulebooks.

§ 21(3) A raw pointer does not inherit safe-reference guarantees merely because it originated from a safe reference.

§ 21(4) Converting `RawPtr[T]` to `ref T`, `ref mut T`, or a safe bounded view requires an unsafe boundary or a compiler-trusted wrapper that establishes every required guarantee.

§ 21(5) Required guarantees include, as applicable:

- non-nullness;
- alignment;
- initialized valid representation;
- bounds;
- live storage;
- provenance;
- alias authority;
- ownership compatibility;
- address-space compatibility;
- foreign retention compatibility;
- relocation safety.

§ 21(6) Raw-pointer storage, copy, move, comparison, or passing is not itself proof that dereference is valid.

---

## § 22 FFI retention

§ 22(1) Foreign declarations must distinguish call-bounded pointer/reference use from retained use where the ABI/contract permits retention.

§ 22(2) A non-retaining foreign call may borrow a safe reference for the proven call-bounded use when FFI conversion rules allow it.

§ 22(3) Retained foreign use requires a lifetime long enough for the complete retention period.

§ 22(4) Retained direct addresses may require pinning or another address-stability mechanism.

§ 22(5) Mutable foreign access requires compatible exclusive authority.

§ 22(6) Foreign code returning a raw pointer does not automatically establish safe provenance.

§ 22(7) Unknown foreign retention must not be treated as non-retaining.

---

## § 23 Reference-containing values and destruction

§ 23(1) A safe reference holder is non-owning and its destruction does not destroy the referent.

§ 23(2) Destruction of the owner must not occur while a live reference still requires that owner, except where a canonical validity-preserving mechanism makes the reference independent of that owner.

§ 23(3) Partial move/destruction of an aggregate invalidates only reference relationships that depend on the moved/destroyed Place or an overlapping Place.

§ 23(4) Reinitialization begins a new value lifetime for the reinitialized Place and must not make references to the previous value valid.

§ 23(5) Conditional availability does not itself create a conditional reference; safe reference creation still requires path-specific proof of an available live Place.

---

## § 24 Diagnostics

§ 24(1) Reference diagnostics must follow the mentor-compiler principles defined by diagnostics rules.

§ 24(2) A reference diagnostic should identify:

- the rejected reference use or escape;
- the source origin or candidate origins;
- the operation or boundary that ends validity;
- the relevant borrow/lifetime/provenance reason;
- a practical safe restructuring when one is known.

§ 24(3) Diagnostics should lead with programmer concepts such as "this reference points to a local value that ends when the function returns" rather than compiler-theory jargon.

§ 24(4) When a failure is caused by ISR execution context, diagnostics should identify both the reference lifetime/storage cause and the interrupt root/context when relevant.

§ 24(5) Unknown provenance and proven-invalid provenance must be distinguished when that difference changes the programmer's remedy.

---

## § 25 LSP and analysis tooling

§ 25(1) LSP must consume the same canonical reference, provenance, borrowing, lifetime, ownership, hardware, and ISR facts as compilation.

§ 25(2) LSP must not implement a second weaker reference-validity engine.

§ 25(3) Useful hover/navigation may expose, where appropriate:

- reference kind;
- mutability;
- canonical origin;
- possible origin set;
- borrow scope/live range;
- storage domain;
- generation/epoch dependency;
- mapping/view dependency;
- returned-reference relation;
- escape/retention status.

§ 25(4) Tooling may omit expensive nice-to-have visualization in Interactive mode, but mandatory invalid safe-reference uses must not be silently treated as valid.

§ 25(5) Separate-compilation reference summaries must be versioned, validated, deterministic, and invalidated when their semantic dependencies change.

---

## § 26 Semantic IR

§ 26(1) Semantic IR must preserve every reference fact still required to prove or lower safe semantics after Sema.

§ 26(2) Such facts may include:

- reference kind and mutability;
- canonical provenance;
- storage origin/domain;
- returned-reference relationship;
- generation/epoch dependency;
- bounds/view metadata;
- mapping/view dependency;
- relevant escape/retention facts;
- borrow/reborrow relation where needed by later verification/lowering.

§ 26(3) High-level reference facts may be erased only after equivalent verified lower-level constraints or representation decisions exist.

§ 26(4) Semantic IR verification must reject contradictory reference facts rather than letting lowerings reinterpret them.

---

## § 27 Lowering and representation

§ 27(1) Source-level safe-reference semantics are independent of one mandatory runtime representation.

§ 27(2) A proven ordinary `ref T` may lower to a plain machine address when all required guarantees are statically established.

§ 27(3) A reference may require additional bounds, epoch, capability, address-space, or indirection metadata when the selected profile and proof state require it.

§ 27(4) Hosted and embedded targets need not use identical physical reference representations.

§ 27(5) They must preserve identical source-level safety guarantees unless a different explicit source type or unsafe boundary is used.

§ 27(6) The compiler must not introduce a mandatory garbage collector, reference counter, handle table, generation manager, or hidden heap allocation merely to implement ordinary safe references.

§ 27(7) Backend lifetime/noalias metadata is an optimization result of proven Sec semantics, not the source of those semantics.

§ 27(8) Optimizations must not strengthen alias/lifetime assumptions beyond canonical provenance and borrowing proof.

§ 27(9) Raw pointers, FFI, volatile access, hardware aliases, interrupts, and opaque calls must remain conservative where stronger proof is absent.

---

## § 28 Required test families

§ 28(1) A conforming implementation maintains regression coverage for basic reference creation and validity.

§ 28(2) Required basic tests include:

- shared reference creation;
- mutable reference creation;
- safe reference non-nullness;
- explicit `Option[ref T]`;
- shared-reference copy;
- mutable-reference move;
- no ownership transfer through reference creation.

§ 28(3) Required lifetime/escape tests include:

- returned local rejected;
- returned caller-owned parameter accepted;
- returned receiver projection accepted when valid;
- finite multi-origin returned reference accepted only when every origin is valid;
- aggregate-contained local reference escape rejected;
- closure-captured local reference escape rejected;
- no hidden heap promotion repair.

§ 28(4) Required provenance/projection tests include:

- field projection;
- constant index;
- dynamic index conservative join;
- static slice range composition;
- path-disjunctive origins;
- unknown-origin safe reborrow rejection where proof is required.

§ 28(5) Required generation/storage tests include:

- arena reset invalidation;
- collection storage replacement invalidation;
- storage-address reuse does not revive stale reference;
- static proof removes unnecessary runtime epoch check;
- stale ordinary safe reference follows canonical panic/trap semantics.

§ 28(6) Required hardware tests include:

- fixed-address binding does not imply unlimited lifetime;
- register view cannot outlive runtime mapping;
- mapping destruction invalidates dependent view;
- remap invalidates old view unless stability is proven;
- device liveness is not `is available`;
- volatile access does not create reference ownership.

§ 28(7) Required interrupt tests include:

- ISR uses valid static/fixed/preallocated reference;
- ISR-local reference cannot escape handler lifetime;
- deferred-work handoff cannot retain ISR-local reference;
- nested/preempting execution still requires ordinary borrow/concurrency proof;
- volatile is not ISR synchronization;
- reference-valid operation may still fail ISR execution-context rules.

§ 28(8) Required FFI/raw tests include:

- safe reference to call-bounded foreign use;
- retained foreign use requires sufficient lifetime;
- mutable foreign use requires exclusive authority;
- raw-to-safe conversion requires unsafe/trusted proof;
- foreign-returned raw pointer has no automatic safe provenance.

§ 28(9) Required tooling tests include compiler/LSP parity and mentor diagnostics that identify origin, invalidation/escape cause, and remedy.

---

## § 29 Completion criteria

§ 29(1) References v2 frontend support is complete when the compiler can create, propagate, join, return, store, project, borrow, reborrow, compare, and diagnose safe references with canonical provenance and lifetime relationships for all Sec 0.1 source forms.

§ 29(2) Interprocedural support is complete when direct, indirect, generic, interface, recursive, imported, and separately compiled call relationships preserve validated returned-reference and retention summaries.

§ 29(3) Platform support is complete when fixed-address storage, runtime mappings, register views, remapping, address spaces, volatile access, and ISR execution consume canonical reference/lifetime facts without redefining them.

§ 29(4) Lowering support is complete when every runtime representation preserves source-level safe-reference semantics and all unnecessary metadata/checks can be eliminated only through valid proof.

§ 29(5) Tooling support is complete when compiler, LSP, `sec analyse`, summaries, incremental recomputation, and diagnostics use the same canonical reference facts.

§ 29(6) A reference implementation must not be marked complete merely because `ref` syntax parses or because one generation-check mechanism exists.

---

## § 30 Core summary

§ 30(1) `ref T` and `ref mut T` are safe, typed, non-null, non-owning reference values.

§ 30(2) References borrow authority; they do not own the referent.

§ 30(3) Reference safety requires lifetime, provenance, bounds, initialization, type, address-space, borrow, ownership, relocation, execution-context, and concurrency validity where applicable.

§ 30(4) Safe references do not require one universal physical representation.

§ 30(5) Generation checks are one validity mechanism, not a replacement for the rest of Sec's memory model.

§ 30(6) Returning and escaping references are valid only when every possible origin remains valid for the complete later use.

§ 30(7) Fixed address, MMIO, volatile access, ISR execution, and raw pointers do not bypass ordinary reference safety.

§ 30(8) Ordinary programmers should not need explicit lifetime or provenance syntax; the compiler carries that proof burden and explains failures in mentor-oriented language.
