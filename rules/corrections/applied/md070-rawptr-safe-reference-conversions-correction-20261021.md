# MD-070 — RawPtr and safe-reference conversion spelling

- **Status:** Applied 2026-10-21 — integrated into the owning rulebooks and implemented in the compiler-known registry, Sema and LSP; see the application note.
- **Created:** 2026-10-21
- **Last updated:** 2026-10-21
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical correction destination:** `rules/corrections/applied/` after integration
- **Primary normative owners:** `rules/memory/raw_pointers.md`; `rules/compiler/compiler_known_members.md`
- **Related normative owners:** `rules/memory/reference_model.md`, `rules/memory/references.md`, `rules/memory/borrowing.md`, `rules/memory/lifetime_analysis.md`, `rules/memory/unsafe.md`
- **Implementation governance:** affected memory/compiler governance fragments, according to the current `governance/` schema
- **Decision tracking:** `missing-decisions.yaml`, MD-070
- **Replaces:** the unresolved source-spelling and resulting-reference contract in MD-070; does not replace existing safety rules
- **Repository basis:** Public `main` documents reviewed 2026-10-21 (raw_pointers.md §§ 5, 8, 17–18, 23, 32; compiler_known_members.md `Ptr` contract; MD-070 entry)

---

## § 1. Purpose and scope

§ 1(1) This correction assigns the source spelling for conversions between `RawPtr[T]` and safe references. It does not redesign borrowing, ownership, lifetimes, reference identity, `unsafe`, pointer representation, or target-specific validity.

§ 1(2) The decision adds **no new explicit or implicit source-level lifetime parameters**, lifetime annotations, generic lifetime arguments, or lifetime-introduction syntax.

§ 1(3) The two unsafe conversion methods specified below are restricted to the compiler-known `RawPtr[T]` type. No other type receives these methods by universal fallback, implicit receiver adaptation, or automatic extension.

§ 1(4) The two conversion directions are different operations. Producing a raw address *removes* safe-reference guarantees from the result; producing a safe reference *asserts and must respect* all required reference guarantees.

## § 2. RawPtr-to-reference API

§ 2(1) The following are the complete additional public member **signatures** introduced for `RawPtr[T]`; the owning compiler-known registry supplies them as compiler-provided operations. These are interface contracts, not user-implementable methods and not method bodies:

```sec
impl RawPtr[T] {
    /**
     * Constructs a shared safe reference from this raw pointer.
     *
     * Description:
     *   Requires the complete raw-to-shared-reference safety contract.
     *   The returned reference does not own or extend the pointed-to storage.
     *
     * Safety:
     *   The address must be non-null, correctly aligned, point to a valid
     *   initialized T in live readable storage, and permit shared borrowing.
     *   Provenance, aliasing, epoch, and address-space constraints apply.
     */
    unsafe fn AsRef() ref T

    /**
     * Constructs an exclusive mutable safe reference from this raw pointer.
     *
     * Description:
     *   Requires the shared-reference contract plus exclusive mutable
     *   authority and all valid mutation/replacement obligations.
     *   The returned reference does not own or extend the storage.
     *
     * Safety:
     *   The address must be writable, safely represent T, and permit an
     *   exclusive borrow for the reference's entire semantic live range.
     */
    unsafe fn AsRefMut() ref mut T
}
```

§ 2(2) The signatures above are specification declarations of **compiler-known** methods. A Sec source implementation may not shadow or redefine their authority. They must be exposed in compiler-known member lookup, LSP metadata, and generated API documentation with the same return types and unsafe status.

§ 2(3) Both methods are unsafe. Example:

```sec
let observer := unsafe pointer.AsRef()
let editor := unsafe otherPointer.AsRefMut()
```

§ 2(4) The `AsRef` and `AsRefMut` names do not create general conversion traits, general-purpose reference methods, or methods on `ref T`, `ref mut T`, or `T`.

§ 2(5) `RawPtr[void]` cannot directly form a safe `ref void` or `ref mut void` through these methods. A concrete compatible typed pointer interpretation is required first under existing raw-pointer rules.

§ 2(6) There is no implicit raw-to-safe conversion, automatic nullable `Option` conversion, or implicit failure fallback. When the operation's proof obligation is false, `unsafe` does not make the reference valid.

## § 3. Safe reference or addressable Place to RawPtr

§ 3(1) The compiler-known unsafe property `.Ptr` is the existing canonical source spelling for obtaining a `RawPtr` from an addressable referent/Place; no second general reference-to-raw conversion operator or method is introduced by MD-070.

```sec
let mut number := 42
let pointer := unsafe number.Ptr
```

§ 3(2) Where `.Ptr` is applied through an existing safe reference, the address sought is the **referent's storage**, not storage used internally to represent the reference binding. A different address for a reference object's own representation must not be silently substituted.

§ 3(3) This correction does not introduce an additional `.Ptr` overload with separate semantic identity, nor assume that a proposed `*` source form from still-open MD-063 has already been adopted. Registry and parser changes must synchronize the exact supported reference-receiver spelling with the compiler-known `Ptr` contract and the eventual MD-063 outcome.

§ 3(4) `.Ptr` requires the existing unsafe context and addressability conditions, and never extends owner lifetime, preserves borrow rights in the pointer value, or pins/keeps storage alive.

§ 3(5) A raw pointer obtained from an existing safe reference must not be used to bypass an ongoing exclusive or shared borrow conflict and recreate incompatible safe access.

## § 4. Existing lifetime model; no new parameters

§ 4(1) `AsRef()` and `AsRefMut()` establish normal Sec reference values and use **the existing Sema provenance, `BorrowID`, `LifetimeID`, validity policy, invalidation epoch/domain, and storage-live-range model**; no new lifetime syntax or parameter participates in the call.

§ 4(2) When the compiler knows the allocation/storage origin, it must preserve that origin and constrain the new reference to its actual lifetime and valid epoch. When the origin or duration is known only under a trusted/unsafe contract, the caller assumes exactly those unproven obligations under `unsafe.md` and `raw_pointers.md` § 18.

§ 4(3) A bare numeric address is not, by itself, evidence of a safe lifetime or borrowing authority. The compiler must not fabricate a static, immortal, or otherwise unconstrained lifetime merely because `AsRef()` is invoked.

§ 4(4) `unsafe` transfers **unproven** validity obligations to the programmer/trusted wrapper. It does **not** disable Sema checks or override any conflict, dangling reference, null address, invalid epoch, address-space mismatch, or lifetime failure that the compiler can prove.

§ 4(5) References produced from raw pointers do not grant ownership, pin the underlying storage, prevent an Arena reset, authorize destruction, or turn untracked foreign mutation into ordinary safe mutation.

§ 4(6) References derived by these methods follow the normal Sec rules for copying shared references, noncopyable mutable references, reborrowing, return/escape analysis, and borrow-end placement (including MD-065).

## § 5. Required proof obligations

§ 5(1) `AsRef()` requires non-nullness; proper alignment; initialized, valid `T` representation; readable, live storage; correct provenance; compatible target address space; and no violation of ordinary shared-reference alias and foreign-mutation guarantees.

§ 5(2) `AsRefMut()` additionally requires writable storage, exclusive mutable authority for the complete live range, absence of conflicting live safe references or unmodeled foreign mutation, and valid replacement/destruction semantics.

§ 5(3) Static contradictions are compilation errors even within `unsafe`. Facts unknown to Sema remain explicit unsafe caller obligations, not compiler-guaranteed properties.

§ 5(4) `RawPtr` arithmetic, conversions, and address copying never themselves produce or prolong these safe-reference guarantees.

## § 6. Semantic IR and MLIR contract

§ 6(1) Semantic IR must represent the raw-to-safe conversion as a distinct unsafe semantic operation or equivalently identified compiler-known operation and must preserve its original raw-pointer operand and target reference kind.

§ 6(2) The operation must establish canonical reference facts, including reference kind, `BorrowID`, `LifetimeID`, provenance, validity policy/proof, invalidation domain/epoch dependency, address stability and target address space where applicable. The exact serialized MLIR attribute spelling belongs to the appropriate dialect schema and MD-084; MD-070 does not invent those spellings.

§ 6(3) Verification must reject structurally missing, invalid or contradictory facts. It must not silently synthesize safe facts from a raw address alone.

§ 6(4) Lowering must preserve source target/address-space semantics and must not treat a raw-to-reference conversion as an ownership transfer or allocation.

## § 7. Diagnostics

§ 7(1) Diagnostics should identify the particular operation, violated/unknown contract and suggested repair. Representative conditions include: missing `unsafe`; known null, dangling or unaligned pointer; conflicting mutable/shared authority; unsupported `RawPtr[void]`; incompatible address space; and invalid reference escape.

§ 7(2) For unprovable but not statically contradicted obligations, the diagnostic must explain the unsafe responsibility rather than falsely asserting that the compiler proved safety.

## § 8. Required regression tests

§ 8(1) Positive tests: `AsRef()` from live non-null `RawPtr[T]` in `unsafe`; `AsRefMut()` with demonstrably exclusive authority; borrowed reference use within the proven lifetime; raw pointer copied without ownership effects; compiler-known member and LSP resolution.

§ 8(2) Negative tests: calls outside `unsafe`; calls on `T`, `ref T` or `ref mut T`; provable null/misalignment/dead epoch; conflicting reference authority; invalid reference escape; unsupported target address space; direct `RawPtr[void].AsRef()`; implicit raw-to-safe conversion; and raw-pointer use to circumvent known borrowing violations.

§ 8(3) Ensure `value.Ptr` continues to refer to source value storage, yields the documented `RawPtr` element type, and does not allocate, extend lifetime, or grant safe authority. Audit use of `.Ptr` on reference receivers against the established compiler-known member-resolution rules; keep the MD-063 interaction explicit.

§ 8(4) Test compiler/formatter/LSP rendering of `unsafe pointer.AsRef()` and `unsafe pointer.AsRefMut()` without introducing lifetime arguments.

## § 9. Integration checklist and decision status

§ 9(1) Integrate into `raw_pointers.md` §§ 8, 17, 18 and 32 **without renumbering existing normative clauses**. Add paragraph IDs or a stable subsection as needed rather than repurposing old citations.

§ 9(2) Register both methods and the reference-receiver `.Ptr` contract in `compiler_known_members.md`, with exact owner and diagnostics. Synchronize `references.md`, `reference_model.md`, `lifetime_analysis.md`, `unsafe.md`, P15 tests and affected governance fragments.

§ 9(3) Update `missing-decisions.yaml` only after the normative owner and governance records are synchronized; until then MD-070 may remain open for integration tracking even though the **owner-level design choice is decided**.

§ 9(4) MD-063 remains separate and under review. This correction shall not silently approve its proposed explicit `*` syntax.

§ 9(5) User-approved design choices recorded on 2026-10-21: `AsRef()` and `AsRefMut()` are specific to `RawPtr[T]`; safe reference semantics remain those of Sec; **no new lifetime parameters**.

---

**Reference URLs (public main as reviewed):**

- https://github.com/YoePro/sec/blob/main/missing-decisions.yaml
- https://github.com/YoePro/sec/blob/main/rules/memory/raw_pointers.md
- https://github.com/YoePro/sec/blob/main/rules/compiler/compiler_known_members.md
- https://github.com/YoePro/sec/blob/main/rules/memory/unsafe.md
- https://github.com/YoePro/sec/blob/main/rules/declarations/impl.md

---

## Application note (2026-10-21)

**Integrated into:**
- `rules/memory/raw_pointers.md` rev 2.1: § 8(3)–(4) and § 17(6) now state the locked spellings; adds § 17(7)–(9), the new § 18A(1)–(6) and § 32(7). No existing clause is renumbered.
- `rules/compiler/compiler_known_members.md`: "RawPtr[T] compiler-known methods".
- `rules/memory/references.md` § 21(7).
- `rules/memory/reference_model.md` §§ 50(4), 51(5).
- `rules/memory/unsafe.md` § 22(5).
- `rules/memory/lifetime_analysis.md` § 23.1(4).

**Reconciled with decisions made after this correction was written:**
1. MD-063 was applied the same day, so § 3(3) and § 9(4) are superseded. `(*r).Ptr` is accepted and means the same as `r.Ptr`: the referent's storage.
2. MD-079 limits call-form `unsafe` to one call. The examples `unsafe pointer.AsRef()` stand, but `.Ptr` is a property, so the § 3(1) example `let pointer := unsafe number.Ptr` must be written inside an `unsafe { }` block. That is the form used in `raw_pointers.md` § 17(7).

**Implementation:**
- `internal/sema/raw_pointer_references.go` (new): typing and checks.
- Registry entries `CKM-RAWPTR-AS-REF` and `CKM-RAWPTR-AS-REF-MUT`, with caller obligations, in `compiler_known_physical_storage.go`.
- Dispatch in `analyzer_inference.go`.
- `.Ptr` through `(*r)` in `analyzer.go`.
- The implicit-conversion hint in `reference_dereference.go`.
- An explicit Semantic IR capability boundary in `internal/ir/semantic/reference_lowering.go`.

**Tests:**
- `internal/sema/raw_pointer_references_test.go`
- `internal/sema/raw_pointer_registry_test.go`, extended to cover the 8/9-operation surface.

**§ 8 coverage:**
- **Covered:**
  - Positive: `AsRef`, `AsRefMut`, use of the result, `.Ptr` on a value, through `r` and through `(*r)`, and registry/LSP metadata.
  - Negative: outside `unsafe`, `RawPtr[void]`, a non-RawPtr receiver, proven null and implicit conversion.
  - Formatting of `unsafe pointer.AsRef()`.
- **Open:** dead epoch, misalignment, conflicting authority, unsupported address space, and use of a raw pointer to bypass a known borrow conflict. These are known holes below.

**Known holes (verified 2026-10-21):**
- The Semantic IR raw-to-safe operation (§ 6) is not implemented. Lowering stops at an explicit capability boundary until MD-084 spells it.
- Sema proves a null pointer false, and since 2026-10-22 also a known origin. For an immutable binding initialized from `place.Ptr`, `r.Ptr` or `(*r).Ptr`, it rejects `AsRefMut` over any live borrow, `AsRef` over a live mutable borrow, and either over an origin that no longer holds a value (`TestRawPointerReferenceConversionsCheckKnownOrigins`). Misalignment and dead Arena epochs are not yet diagnosed, and neither are origins that pass through arithmetic, mutable pointer bindings or calls.
- A reference formed from an arbitrary raw pointer has no Sema origin, so its escape is bounded only by the caller's unsafe contract.
