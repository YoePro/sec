# Owner[T] (Working Name) — Initial Design Baseline for Sec 0.1.1

- **Status:** Accepted design direction — partial normative baseline; public construction/API and some safety rules remain open
- **Created:** 2026-10-14
- **Last updated:** 2026-10-15
- **Document revision:** 0.2
- **Sec language version:** 0.1.1 (target for the ownership abstraction provisionally named `Owner[T]`); Sec 0.2 is relevant only to future linear-type integration
- **Provisional path:** `rules/memory/owner.md` (working title/path, not an established canonical rulebook; final naming remains open)
- **Related missing decision:** `MD-018` (recursive union indirection); additional missing decisions will be opened as implementation reveals undecided cases
- **Owning and related rulebooks:** `rules/memory/ownership.md`, `rules/memory/reference_model.md`, `rules/memory/borrowing.md`, `rules/memory/copy_move.md`, `rules/memory/destruction.md`, `rules/memory/allocation.md`, `rules/memory/arena.md`, `rules/memory/layout.md`, `rules/declarations/unions.md`, and applicable compiler/generics/diagnostics rules
- **Replaces:** Revision 0.1 of this design baseline (2026-10-14); no existing canonical rulebook paragraph is replaced.
- **Revision policy:** Preserve all paragraph identifiers once published. Extend by inserting new numbered paragraphs, appendices, or separately tracked corrections; do not renumber existing references.
- **Governance:** This document records a design baseline, not implementation completion. Keep `MD-018` open until all relevant owning rules, implementation status, and conformance tests are integrated. Record additional gaps as new MD entries; do not misclassify proposals as completed decisions.

---

## § 1. Purpose and decision status

**§ 1(1) [LOCKED SEMANTICS; WORKING NAME].** Sec 0.1.1 targets an explicit, unique **value-ownership capability** for an initialized `T` in separately provided storage. `Owner[T]` is the **current working name only**; the final public name is not decided and may change in later revisions. Earlier discussion also used `Owned[T]`. References to `Owner[T]` in this document indicate the proposed concept, not a locked language spelling.

**§ 1(2) [LOCKED].** The primary use case is safe, finite-sized owning indirection in recursive data structures **without automatic allocation** merely because a field has type `Owner[T]`.

**§ 1(3) [LOCKED].** Value ownership, storage ownership/reclamation, access/borrowing, lifetime, availability, and generation/epoch validity are distinct semantic responsibilities. This type must integrate with the existing Sec analyses rather than invent a parallel ownership or pointer-validity system.

**§ 1(4) [STATUS].** `[LOCKED]` means agreed semantic direction. `[WORKING]` means a favored architecture to be proved and specified during implementation. `[OPEN]` is a remaining normative choice. `[WORKING NAME]` explicitly means the displayed identifier is illustrative and may be renamed; it does not establish canonical public syntax. No `[WORKING]` or `[OPEN]` paragraph authorizes a compiler to invent source syntax or public APIs.

## § 2. What `Owner[T]` means

**§ 2(1) [LOCKED].** An *available* `Owner[T]` identifies one valid, initialized, live `T`, and uniquely bears responsibility for that object's value lifetime. `Owner[T]` is not an owning copy of a borrowed `ref T`.

**§ 2(2) [LOCKED].** `Owner[T]` provides **indirection with a finite-sized representation independent of the inline size of `T`**, so that recursive type layout can terminate. The representation is selected by the compiler/target contract; it is not necessarily one machine pointer or one fixed set of runtime fields.

**§ 2(3) [LOCKED].** A value of type `Owner[T]` does **not itself perform implicit allocation** when declared, stored, moved, borrowed, or embedded in a type. No hidden heap/arena promotion may be introduced to repair recursive layout or a failing lifetime check.

**§ 2(4) [LOCKED].** A storage producer must provide the actual backing storage. Such a producer **may itself allocate** according to its explicit API and allocation-effect contract; `Owner[T]` does not prohibit allocation in the program. The distinction is between explicit storage acquisition and allocation hidden in the semantics of an owning field.

**§ 2(5) [LOCKED].** `Owner[T]` is an exclusive, non-copyable ownership carrier. Ownership changes through Sec's existing consuming/move rules, including required `<-` notation for moves from named reusable places. A moved-from place becomes unavailable; it does not automatically become `None`.

**§ 2(6) [LOCKED].** `Owner[T]` itself is not nullable or optional. Absence is modeled with `Option[Owner[T]]`, and ordinary Sec still has no general `null`. Representation optimization for `Option[Owner[T]]` is allowed only when justified by canonical layout/validity facts; equal-size layout is **not** promised by this baseline.

## § 3. Relationship to other ownership forms

**§ 3(1) [LOCKED].** `ref T` and `ref mut T` remain non-owning safe references. Neither grants destruction authority, storage-reclamation authority, or the ability to mint an `Owner[T]` through a cast.

**§ 3(2) [LOCKED].** `Owner[T]` bears **value ownership**, but does not, by that fact alone, bear authority to reclaim its backing bytes. Allocation-domain or slot-reclamation authority stays with the storage provider, subject to a future precise release protocol.

**§ 3(3) [WORKING].** A separate `Box[T]` may eventually be defined to combine value ownership with storage/allocation responsibility. It is **not** defined by this document, and an occurrence of `Box[T]` in older recursion examples must not be treated as a defined built-in owning indirection.

**§ 3(4) [OPEN].** The exact generic admissibility rule for `Owner[ref T]` and `Owner[ref mut T]` remains to be ratified. The **recommended restriction** is to reject these immediate reference types, because owning a reference value is not owning its referent and must never upgrade borrowed authority. The same restriction does not automatically prohibit owning a struct that *contains* valid, lifetime-checked borrowed fields.

## § 4. Object identity, generation and storage dependency

**§ 4(1) [LOCKED].** An `Owner[T]` must be associated with an actual object identity, the object's live generation/epoch where applicable, and an appropriate storage-domain/slot dependency. None of these facts alone constitutes the ownership right.

**§ 4(2) [LOCKED].** The object's storage must remain valid for the entire lifetime of the live owned value and for any compatible outstanding borrows. A storage reset, release, slot reuse or foreign invalidation must not invalidate a still-live `Owner[T]` or a safe reference derived from it.

**§ 4(3) [LOCKED].** Moves of the ownership capability do not inherently relocate the underlying object. If construction or transfer **does** involve physical relocation, existing borrowing, pinning, address-stability, provenance, and reference-validity rules determine legality.

**§ 4(4) [LOCKED].** Generation/epoch validation and exclusive ownership are **not equivalent**. A generation-identifying handle does not become an owner merely because it correctly resolves a live slot. No new `GenPtr[T]`, `OwnerGen[T]`, or equivalent public type is introduced by this baseline.

**§ 4(5) [WORKING].** A reusable pool slot should not be returned to the pool while it holds a live owned value. After valid destruction, slot reuse must follow the existing non-revival generation requirements. Whether slot release is immediate or deferred belongs to the storage producer's contract.

## § 5. Birth and transfer of an owner

**§ 5(1) [LOCKED].** Ownership may originate **only with unique ownership authority**. A plain `ref T`, `ref mut T`, or `RawPtr[T]` must not be silently upgraded to `Owner[T]`.

**§ 5(2) [WORKING].** The primary producer path directly initializes `T` in valid, prepared backing storage and issues exactly one `Owner[T]` as part of that operation. The storage producer retains any storage-reclamation authority that its domain contract assigns to it.

**§ 5(3) [WORKING].** A second path may relocate an **already owned** `T` into new prepared storage and issue `Owner[T]`. Relocation must obey all existing move, borrow, pinning, address-stability and identity rules. Values which cannot be physically relocated may still be eligible for direct construction in their final storage.

**§ 5(4) [WORKING].** For relocation construction, use a **two-phase transaction**: (1) prepare and validate sufficient storage without consuming the old owner; (2) perform the proven-safe, infallible ownership transfer/placement. A storage-preparation failure must not silently consume or destroy the original value.

**§ 5(5) [OPEN].** The exact Sec declarations for storage reservation, uninitialized slot tokens, placement initialization, allocation failure, and issuing or transferring `Owner[T]` are **not chosen**. In particular, no `new Owner[T](...)`, `Owner[T](ref value)`, `pool.CreateOwner[...]`, or `Arena.NewOwner[...]` signature is established here. An implementation must not invent one merely because the type constructor has been named.

**§ 5(6) [OPEN].** The source and compiler contracts for explicitly selecting a storage producer/allocator, including whether one may pass an allocator to a `Box[T]` constructor, need their own decision. A future `Box[T]` does not imply a hidden allocation by `Owner[T]`.

## § 6. Borrowing, moves and destruction

**§ 6(1) [LOCKED].** `Owner[T]` supports the existing Sec ownership semantics: a consuming move transfers one unique lifetime responsibility and invalidates the source place; borrowing does not transfer ownership.

**§ 6(2) [WORKING].** A live owner should be able to expose safe shared and exclusive access to its `T` according to normal `ref` / `ref mut` rules. The **exact method/property declarations and receiver capabilities are not yet selected**. This paragraph is not an authorization to fabricate `.Value`, `.Borrow()` or `.BorrowMut()` APIs.

**§ 6(3) [LOCKED].** Destruction of the owned `T` and reclamation of its backing storage are separate. Only one destruction of `T` may occur; the storage producer must not duplicate it. No observable `T` may remain in a slot after its lifetime is ended and before lawful reinitialization.

**§ 6(4) [OPEN].** The precise relationship between `Owner[T]` termination, user-defined `free()`, pool slot return, arena reset/release, failures during cleanup, and recursively nested owners must be specified. In particular, no arbitrary automatic destruction of potentially unbounded recursive chains on the call stack may be assumed safe without a defined stack/cleanup policy.

**§ 6(5) [OPEN].** `Owner[T]`'s handling of partial moves, direct field projection, generics, trait/interface constraints, and concurrency-safe handoff requires integration tests and any needed further decisions. Existing ownership checks must not be weakened.

## § 7. Recursive data structures and `Option`

**§ 7(1) [LOCKED].** The design goal is that `Owner[T]` forms a **valid finite-sized indirection boundary** in recursive type declarations. Plain by-value recursion remains invalid.

**§ 7(2) [TARGET SHAPE — not yet compilable].** This illustrates the intended recursive type syntax, **not** a construction API or claim of compiler support:

```sec
// Target Sec 0.1.1 source shape: requires Owner[T] implementation.
type List[T] union {
    Node {
        value: T
        next: Owner[List[T]]
    }

    End
}
```

**§ 7(3) [LOCKED].** A `Node` whose `next` is `Owner[List[T]]` must own an actual next `List[T]` value, including if that value is the `End` variant. The presence of an `End` variant does not magically provide storage for such a value.

**§ 7(4) [TARGET SHAPE — not yet compilable].** Where a next node may be absent without an allocated `End` value, use ordinary optional ownership:

```sec
// Target Sec 0.1.1 source shape: requires Owner[T] implementation.
type Node[T] struct {
    value: T
    next: Option[Owner[Node[T]]]
}
```

**§ 7(5) [LOCKED].** `None` describes an available, empty `Option`. It is never the state of a moved-from or destroyed `Owner[T]`. Availability, optional absence, and generation validity remain separate concepts.

## § 8. Linear types and version boundaries

**§ 8(1) [LOCKED].** `Owner[T]` is targeted for **Sec 0.1.1**. It must not depend on an already implemented Sec 0.2 linear-type system.

**§ 8(2) [DESIGN INTENT].** Sec 0.2 is expected to introduce linear types. The intent is to extend the **same** `Owner[T]` abstraction to types carrying linear obligations, rather than create a competing `LinearOwner[T]` family.

**§ 8(3) [OPEN FOR 0.2].** Which terminal operation discharges a linear obligation, including whether and how `free()` counts as consumption, is **not decided** by Sec 0.1.1 `Owner[T]` semantics. The 0.1.1 implementation must not promise accidental weakening of linear guarantees.

## § 9. Public API and syntax boundary

**§ 9(1) [LOCKED SEMANTICS; WORKING NAME].** The proposed ownership abstraction requires compiler/core recognition as a generic ownership form. The provisional spelling `Owner[T]` is **not yet canonical**. Recognizing the eventual spelling does **not** define public methods, constructors, properties, allocation interfaces, or low-level representation.

**§ 9(2) [OPEN].** The owner rulebook must eventually reproduce **complete exact Sec declarations and structured documentation** for every public compiler-known or core-exposed member. No placeholder pseudo-constructors, `static New()`, or partial API descriptions may be treated as an implementation specification. Constructors, where adopted, follow Sec `init(...)` rules.

**§ 9(3) [LOCKED].** No generic implicit cast from `T`, `ref T`, `ref mut T`, or `RawPtr[T]` to `Owner[T]` is introduced. Storage creation, initialization, ownership issuance, and release must be explicit in semantics, with no hidden allocation.

## § 10. Implementation and conformance gates

**§ 10(1) [REQUIRED].** A future implementation must test, at minimum:

1. Direct recursive-by-value layout is rejected; recursion through `Owner[T]` has finite target-resolved layout.
2. Owner creation is unique; borrowed reference / raw pointer upgrade is rejected.
3. Owner move obeys named-place `<-` rules; source becomes unavailable; no copying, use-after-move, or double destruction.
4. `Option[Owner[T]]` distinguishes `None`, available owner, and moved-from/unavailable source.
5. Storage lifetime prevents early Arena reset/release and pool slot reuse; stale generations cannot revive.
6. Direct-in-place construction can obey pinned/address-stable contracts; illegal relocation is rejected.
7. Allocation preparation failure preserves the original owned value in the two-phase path.
8. Value destruction and storage reclamation occur exactly under their respective authorities.
9. Affine ownership works under 0.1.1; 0.2 linear obligations are not silently assumed discharged.
10. Layout, effects, generics, diagnostics and cold/incremental conformance agree on owner identity and validity facts.

**§ 10(2) [REQUIRED].** Tests must cover hosted, Arena-backed, preallocated pool, and no-allocation profiles when the corresponding producer has been specified. Unsupported producers must be rejected with mentor-style diagnostics, not guessed or silently repaired.

**§ 10(3) [REQUIRED].** Invalid constructions must be rejected before backend lowering. A compiler must not accept the target-shape examples of § 7 as fully supported until the ownership producer, destruction and lifetime rules required to execute them are implemented.

## § 11. Open decision register — expect additional MDs

**§ 11(1).** This is an intentionally **incomplete initial revision**. The list below identifies known outstanding design work; it is **not** a claim that these questions are answered. Create or maintain separately identified MD entries as needed. Preserve stable rulebook paragraph references in later revisions.

| Topic | Open normative question | Likely owner |
|---|---|---|
| Producer API | Exact source syntax for direct construction and ownership issuance | owner / allocation / arena |
| Prepared storage | Typed token for reserved, not-yet-initialized storage; lifetime and cancellation | allocation / arena / memory model |
| Relocation | Preconditions and transaction semantics for moving existing `T` into prepared storage | ownership / reference model |
| Storage contract | Who and when releases a slot; how the source represents storage-domain dependencies | owner / arena / layout |
| Generic arguments | Explicit restriction for `Owner[ref T]` and `Owner[ref mut T]` | owner / types / borrowing |
| Access API | Exact canonical borrow/access declarations and receiver constraints | owner / borrowing / core |
| Destruction | `free()` interaction, recursive cleanup and stack safety, cleanup failure | owner / destruction |
| Allocation effects | Whether a particular producer is allocating or merely consuming preallocated slots | owner / allocation / effects |
| Box[T] | Independent owned-storage abstraction, allocator parameter, fallibility, destruction | separate future design |
| Linear 0.2 | Obligations, terminal consumption, and generics interaction | future linear types |

**§ 11(2).** `MD-018` should not be removed solely because the ownership abstraction has a working name and a proposed recursive layout. Its actual finite representation, legal use, and construction/destruction contracts must be integrated into canonical rulebooks, implemented and verified. Do not prematurely declare `Box[T]` implemented or defined.

**§ 11(3).** After each implementation discovery, record newly identified undecided forks in the latest authoritative `missing-decisions.yaml` and link corrections or follow-on rulebooks. If a rule is already locked, implement it rather than reopening it. The working baseline is expected to receive revisions.

## § 12. Integration notes

**§ 12(1).** Reconcile `rules/declarations/unions.md` §18's illustrative `Box[List[T]]` with the future `Owner[T]` specification. Do **not** silently rename all uses of `Box[T]`; that is a separate abstraction and decision.

**§ 12(2).** Update ownership, reference-model, copy/move, borrowing, destruction, layout, allocation, arena, compiler-known declaration, diagnostics, LSP and governance contracts only as required by finalized decisions. Preserve source- and documentation-level paragraph identifiers.

**§ 12(3).** This document records **no complete public API** and is **not an implementation task list that authorizes guessed APIs**. Start implementation only on fully specified semantic slices and surface real specification gaps as additional MD items.

---

*Design baseline first recorded 2026-10-14; revision 0.2 updated 2026-10-15 to mark `Owner[T]` explicitly as a working name. The next revision should resolve the first source/API contract for storage producers while preserving the existing Sec ownership, availability and validity model.*
