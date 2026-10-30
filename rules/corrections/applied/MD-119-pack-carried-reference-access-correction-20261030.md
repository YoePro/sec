# MD-119 — Access to References Carried by Native Variadic Packs

- **Status:** **APPLIED 2026-10-30** — normatively integrated (references.md § 3.4(10) and § 9(11); functions.md § 38A(8)(6); flowcontrol_for.md § 13A(4); borrowing.md § 11.4 and copy_move.md already govern without change). MD-119 removed from missing-decisions.yaml. The § 8 conformance cases are carried by the existing MD-117 owners (governance/generics.yaml regression matrix, control_flow.yaml heterogeneous iteration, lifetime.yaml pack borrowing); Sema still rejects concrete pack expansion (S1177), so implementation stays pending there.
- **Decision ID:** MD-119
- **Created:** 2026-10-30
- **Last updated:** 2026-10-30
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline:** `YoePro/sec`, `main` at `aa59b4c18c91ed04eef69c7dd0028e41630067f5` (verified 2026-10-30)
- **Destination before integration:** `rules/corrections/md119-pack-carried-reference-access-correction.md`
- **Primary normative owners:** `rules/memory/references.md` § 3.4; `rules/declarations/functions.md` § 38A
- **Related normative owners:** `rules/memory/borrowing.md` § 11.4; `rules/control-flow/flowcontrol_for.md` § 13A; `rules/memory/copy_move.md`; `rules/compiler/semantic_ir.md`
- **Related decisions:** MD-061 (explicit reborrows); MD-063 (explicit whole-value dereference); MD-080 (explicit reference creation); MD-117 (heterogeneous native variadic type packs, including reference origins)
- **Implementation tracking:** Reuse existing feature owners in `governance/declarations.yaml`, `governance/generics.yaml`, `governance/control_flow.yaml` and `governance/lifetime.yaml`. Do not create a duplicate MD-119 implementation owner.

---

## § 1. Scope and authority

**§ 1(1).** MD-119 asks whether reference values **carried by** a native variadic pack have different access or dereference syntax from ordinary references. They do not. This correction makes the consequence of the already accepted reference model explicit; it introduces no new Sec operator, implicit dereference, reference category, pack-value category or lifetime rule.

**§ 1(2).** MD-117 § 38A(8) already governs which references carried by a pack may be returned or forwarded. MD-119 does not weaken or replace those caller-origin, exclusivity or no-pack-escape rules.

**§ 1(3).** This document describes required source semantics, **not** verified implementation completeness. Current missing-decision and governance files may lag accepted normative decisions. Integration, implementation and conformance are tracked independently.

## § 2. Reference-holder expressions retain their type

**§ 2(1).** If a selected pack element has concrete type `ref T`, an expression denoting that carried reference has type `ref T`. It is **not** implicitly converted to `T` by virtue of being in a pack.

**§ 2(2).** If a selected pack element has type `ref mut T`, an expression denoting that carried reference has type `ref mut T`. The holder is move-only, and the pack's no-element-move-out rule still applies.

**§ 2(3).** The ordinary rule in `references.md` § 3.4 applies:

| Expression | Semantic result | Conditions |
| --- | --- | --- |
| `value` | Reference holder, `ref T` or `ref mut T` | Ordinary copy/move/borrow limits apply |
| `*value` | Referent Place, `T` | By-value read requires copyable `T`; writes require mutable authority |
| `value.Field` | Referent's field projection | Same as ordinary reference projection; no whole-value read |
| `value[index]` | Referent's element projection, when type permits | Same as ordinary reference projection; no whole-value read |
| `ref value` | Shared reborrow of the referent, `ref T` | Valid `ref` or `ref mut` source; origin retained |
| `ref mut value` | Exclusive reborrow of the referent, `ref mut T` | Requires a valid `ref mut` source and exclusivity |

**§ 2(4).** `ref value` does **not** produce `ref ref T`. A borrowed reference is not a second-level reference merely because a pack element carries it (`borrowing.md` § 11.4).

## § 3. Pack indexing and member access

**§ 3(1).** A heterogeneous pack's selected position must be statically known to yield a concrete element type (`functions.md` § 38A(5)). A selected position carrying a reference retains that concrete reference type under § 2.

**§ 3(2).** `values[index]` is not an implicit load of its referent. For a position containing `ref T`, `*values[index]` denotes the referenced Place when the index is a valid statically resolved pack position. Indexing a pack does not grant move-out permission for a move-only `ref mut` holder.

**§ 3(3).** No runtime-indexed heterogeneous pack access may invent an unspecified common `T`, `any`, boxing or type erasure.

## § 4. `for` iteration over packs

**§ 4(1).** `functions.md` § 38A(5) and `flowcontrol_for.md` § 13A already assign the ordinary binding meanings to `for value`, `for ref value` and `for ref mut value`. A binding formed from a pack-carried reference follows the same referent/reborrow semantics as ordinary reference bindings.

**§ 4(2).** `for value` over shared-reference elements is permitted when copying the `ref T` holder is legal; inside that iteration `value` still has type `ref T`. `for value` cannot extract a move-only `ref mut T` element from a pack.

**§ 4(3).** `for ref value` over a pack-carried `ref T` or `ref mut T` produces an allowed **shared reborrow of the referent** with type `ref T`, not `ref ref T`.

**§ 4(4).** `for ref mut value` over a pack-carried `ref mut T` produces a **mutable reborrow of the referent** with type `ref mut T`, never a copy or move of the holder. The source holder's conflicting authority is suspended for the child borrow's required live range. It is invalid over a `ref T` source.

**§ 4(5).** The exact effects of an operation on `*value` depend on the concrete `T`; generic specialization must type-check them as normal. Examples (schematic signatures using MD-117 syntax):

```sec
fn Observe[...Types](values: ...ref Types) void {
    for ref value in values {
        // value has type ref T for this concrete pack position.
        // *value is the referent Place, not a pack element move.
    }
}

fn Mutate[...Types](values: ...ref mut Types) void {
    for ref mut value in values {
        // value has type ref mut T for this concrete pack position.
        // A write through *value requires the ordinary rules for T.
    }
}
```

## § 5. Returning and forwarding references

**§ 5(1).** A carried `ref T` may be copied or reborrowed and returned when the verified origin derives from caller-side storage and passes `functions.md` § 38A(8), `references.md` § 9 and lifetime analysis. The pack holder itself never escapes by address; the resulting reference refers to the established caller-side object.

**§ 5(2).** A carried `ref mut T` **cannot be moved out of the pack as one element**. Returning a newly formed `ref mut` reborrow of its referent may be valid **only** when the ordinary exclusive-borrow, holder-liveness, caller-origin and returned-reference proofs permit that return. `ref mut value` does not manufacture an independent second mutable right.

**§ 5(3).** A returned reference retains its exact caller origin (including a finite origin set and statically selected position where applicable). Neither pack iteration nor static expansion resets origin information or extends lifetime.

**§ 5(4).** Attempting to return a reference to the ephemeral pack's own parameter storage remains invalid. Attempting to return `*value` by value is governed by the referent `T` copy rules; it is not a way to move a move-only referent through a borrow.

## § 6. Prohibited reinterpretations

**§ 6(1).** The compiler MUST NOT treat `value` as an implicit `*value` when `value` carries a reference in a pack.

**§ 6(2).** The compiler MUST NOT synthesize `ref ref T` or `ref mut ref T` from pack borrowing. Explicit reborrows of reference holders retain the original referent type; MD-119 creates no nested-reference mechanism.

**§ 6(3).** The compiler MUST NOT copy `ref mut`, move one pack element out, conflate unrelated caller origins, permit competing active mutable access, or let an unverified returned reference outlive its referent.

**§ 6(4).** Calling, spreading and returning keep the normal ownership and call-transfer rules. No `fmt` or other stdlib exception is introduced.

## § 7. Required normative synchronization

**§ 7(1).** In `rules/declarations/functions.md` § 38A(8)(6), replace the undecided MD-119 sentence with an explicit cross-reference to `references.md` § 3.4 and the applicable pack/iteration rules. Keep § 38A's paragraph numbering stable and retain the already established provenance rules.

**§ 7(2).** In `rules/memory/references.md` § 3.4, append a narrow clarification that references carried by native variadic packs retain ordinary holder/referent syntax. Do not renumber existing paragraphs or rewrite the general reference model.

**§ 7(3).** In `rules/control-flow/flowcontrol_for.md` § 13A, clarify that `for ref`/`for ref mut` reborrow the referent of a reference-carrying element; there is no nested reference or moved-out mutable holder. Preserve existing § 13A numbering.

**§ 7(4).** Confirm that `rules/memory/borrowing.md` § 11.4 and `rules/memory/copy_move.md` already govern the behavior. Add a cross-reference only if necessary; do not duplicate their normative substance.

**§ 7(5).** Update the existing stable-ID feature owners in `governance/declarations.yaml`, `governance/generics.yaml`, `governance/control_flow.yaml` and `governance/lifetime.yaml` only where the source change or test evidence actually affects them. No newly invented `md119.*` implementation ID and no unrelated implementation claims.

## § 8. Conformance tests

**§ 8(1).** Positive: a shared-reference pack element retains `ref T`; explicit `*` produces referent access; `.Field`/`[index]` referent projection does not require a whole-value read; a shared reborrow preserves origin; `for value` over copyable `ref T` and `for ref value` preserve the correct reference meaning.

**§ 8(2).** Positive: a mutable-reference pack element can be reborrowed exclusively through `ref mut value` and `for ref mut value`; the source's conflicting access is unavailable until the child borrow ends; proven disjoint mutable referents remain permitted.

**§ 8(3).** Positive: an eligible returned shared or mutable **reborrow** retains caller-side origin and can be verified in a concrete specialization, with a statically selected pack position and a valid result type. Verify finite-origin and projection propagation where supported.

**§ 8(4).** Negative: implicit whole-value dereference; `ref mut` from a shared reference; copied or individually moved-out mutable pack reference; overlapping mutable references; escaping a reference to pack-owned storage; returning an invalid or unknown-origin reference; runtime-indexed heterogeneous pack access with no concrete element type.

**§ 8(5).** Negative: reading a move-only referent through `*value` by value; claiming borrowed referent ownership, double destruction, or loss of reference validity after return.

**§ 8(6).** Each test must distinguish Sema acceptance from Semantic IR, Sec MLIR and executable support. An unsupported lowerer does not invalidate correct source semantics and must fail closed, not fabricate support.

## § 9. Integration and closure

**§ 9(1).** Integrate one feature slice at a time: edit owning rulebook provisions; add focused positive and negative tests; implement only missing semantics; run focused tests; update existing governance IDs. Do not silently alter accepted MD-117 provenance semantics or the ordinary reference model.

**§ 9(2).** Once the affected rulebooks and governance are synchronized, remove or transition MD-119 in `missing-decisions.yaml` according to its documented policy. Implementation may remain pending if not yet proven; do not falsely mark it complete.

**§ 9(3).** Archive the correction in `rules/corrections/applied/` with its **actual application date** only after normative integration, following `rules/corrections/README.md`. Keep all referenced existing rulebook paragraph numbers stable.

---

## § 10. Finding

**§ 10(1).** The answer to MD-119 is **ordinary reference syntax without a special case**: holder `value`, referent Place `*value`, field/element projections `value.Field`/`value[index]`, and explicit shared/mutable reborrow `ref value`/`ref mut value`, subject to ordinary borrow and pack rules.

**§ 10(2).** This is a consistency clarification deriving from MD-061, MD-063 and MD-117, not a new owner-level language choice. It does not by itself establish that the complete heterogeneous-pack implementation has passed its conformance gate.
