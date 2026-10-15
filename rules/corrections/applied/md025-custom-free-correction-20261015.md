# MD-025 — Custom `free`, Copyability, Partial Destruction, and Early Return

- **Status:** Applied 2026-10-15 to `rules/memory/copy_move.md` (§ 3.5(6)-(8)), `rules/memory/ownership.md` (§ 19), `rules/memory/destruction.md` (§§ 10.3(3), 15.4(5)-(6), 15.7(1a)), `rules/declarations/impl.md` (§ 19), `rules/declarations/interfaces.md` (§ 3.3), `rules/types/units.md` (qualified unit references) and `rules/foundations/grammar.md` (reserved `free`); MD-025.3 needed no grammar repair here because the stale status text was already gone. Sema implements and tests MD-025.1, .2, .4, .5 (provable case: discard of a self field inside free) and .6; Semantic IR still rejects custom free, and release-effect proofs beyond that case remain tracked in governance.
- **Created:** 2026-10-15
- **Last updated:** 2026-10-15
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/corrections/applied/md025-custom-free-correction-20261015.md`
- **Decision ID:** `MD-025` (subparts 1–6)
- **Primary rulebooks:** `rules/memory/destruction.md`, `rules/memory/ownership.md`, `rules/memory/copy_move.md`, `rules/declarations/impl.md`, `rules/foundations/grammar.md`
- **Related rulebooks:** `rules/declarations/interfaces.md`, `rules/types/units.md`, `rules/control-flow/discard.md`, compiler diagnostics/testing, Semantic IR, compiler-known lifecycle metadata
- **Replaces:** No existing numbered rulebook paragraphs. Amend the relevant paragraphs *in place*; retain all existing stable paragraph identifiers.
- **Governance:** Reconcile existing per-book implementation-status fragments and the authoritative missing-decisions index. Do not create duplicate implementation IDs or mark an untested implementation as validated.

---

## § 1. Purpose and decision status

**§ 1(1).** This correction records the six decisions or clarifications under `MD-025`. It supplements the current Sec 0.1 rules for `free`; it does not introduce a new ordinary method, manual destructor invocation, copy override, or a linear-type system.

**§ 1(2).** The language decisions in §§ 2–7 are locked for Sec 0.1. Their incorporation into the canonical rulebooks, compiler implementation, diagnostics, test suite, and governance is still required. A decision being locked does **not** establish that the compiler implements or validates it.

**§ 1(3).** Copyability, must-use classification, destruction obligations, resource release, and storage deallocation remain distinct semantic concepts. The presence of custom `free` affects copyability as specified below; it does not automatically invent another obligation.

**§ 1(4).** Sec 0.2 may add explicit linear lifetime obligations. No Sec 0.2 rule is made normative by this correction, and no final design for a linear `free` obligation is decided here.

## § 2. MD-025.1 — A type with custom `free` is `MoveOnly`

**§ 2(1).** Any nominal type whose complete, resolved implementation contains a user-defined `free` is **automatically `MoveOnly`** in Sec 0.1, regardless of whether all its physical fields would otherwise be trivially or semantically copyable.

**§ 2(2).** This has the same copy-restriction effect as implicit `@noCopy`. The programmer need not write `@noCopy` merely to protect the resource lifecycle; omission must not make the type copyable.

**§ 2(3).** `free` does **not** automatically imply `@mustUse`, a new source-level `@affine` annotation, or a Sec 0.2 linear obligation. An ordinary owned local that remains available may undergo compiler-controlled automatic destruction at its lifetime end.

**§ 2(4).** Copy classification propagates through owning aggregates, fixed arrays, owning variant payloads, and generic instantiations under the existing `copy_move.md` derivation rules. A borrowed `ref T` does not own the referent and follows the separate reference-copy rules. The mere presence of a `free` implementation does not make unrelated containing types copyable.

**§ 2(5).** Named reusable sources require explicit ownership-transfer syntax where a move is needed. The compiler must never reinterpret an ordinary copy-shaped initialization as an implicit move merely to make the program compile.

```sec
// Assume OpenHandle() returns Handle and Handle defines custom free.
let first := OpenHandle()
let second := first          // ERROR: Handle is MoveOnly.
```

```sec
// Independent valid example using an explicit ownership transfer.
let first := OpenHandle()
let second :<- first         // Valid move. 'first' becomes unavailable.
```

**§ 2(6).** A type may offer an **ordinary explicitly named** duplication operation if the type's domain permits it. Such a function does not grant implicit copyability. The proposed `@permitCopy` (or an equivalent escape hatch) is **rejected** for Sec 0.1; there is no copy exception for a `free` body that only logs.

**§ 2(7).** Copy classification must be computed after collection/merging of a type's valid `impl` and `impl extends` members and before every place where implicit copying is validated, including generic instantiations. Source-file order must not change the result.

## § 3. MD-025.2 — Field discard follows partial-move restrictions

**§ 3(1).** For a type with custom `free`, **discarding an owned field or owned sub-Place of an otherwise live complete `self`/aggregate is forbidden** in Sec 0.1 wherever a partial move of that sub-Place would be forbidden. This includes nested owned-field projections and does not depend on the field's trivial/nontrivial destruction classification.

**§ 3(2).** The reason is semantic symmetry: a completed-value custom `free` may rely on a complete ownership invariant. A field discard must not dismantle that invariant any more than a partial move may.

```sec
// Assume pair has a type with user-defined free and owns its fields.
discard pair.First       // ERROR: partial destruction is prohibited.
let first :<- pair.First // ERROR: partial move is prohibited.

discard pair             // Valid whole-value discard, if discardability permits.
```

**§ 3(3).** Whole-value destruction and legal whole-value `discard` remain allowed. They invoke the custom `free` for the completed owned value and then apply normal field cleanup in its required order.

**§ 3(4).** This restriction does **not** prohibit ordinary field reads, borrowing, or allowed in-place field mutation that preserves the complete-value contract. It does not change the existing rule that a legal `discard` on an already unavailable whole Place is a no-op rather than double destruction.

**§ 3(5).** `discard` syntax and general discardability remain owned by `rules/control-flow/discard.md`. This correction only constrains the Place category that may legally be discarded when its enclosing type has custom `free`.

## § 4. MD-025.3 — Grammar status correction; implementation is not validation

**§ 4(1).** The current `grammar.md` section **“`free` operation syntax”** incorrectly states that the parser creates an invalid statement and reports `free` as unimplemented. This description contradicts the current `FreeDeclaration ::= "free" Block` production and `impl.md` § 19. Replace the outdated status text **without changing the existing grammar production**.

**§ 4(2).** `free { ... }` is a **lifecycle declaration within a valid `impl`**, not a general expression, an ordinary `fn`, or an ordinary executable statement permitted at arbitrary source locations.

```sec
type Resource struct {
    Handle: int
}

impl Resource {
    free {
        ReleaseHandle(self.Handle)
    }
}
```

In this syntax illustration `ReleaseHandle` denotes a separately declared domain-specific function; the illustration does not add such a function to Sec's standard API.

**§ 4(3).** The implementation-status claim for this subpart is **Implemented / Unvalidated**: according to the recorded compiler audit, parser and semantic analysis recognize custom `free`, but comprehensive verification is not claimed. The stale grammar wording is a documentation defect that must still be repaired.

**§ 4(4).** Do not conflate three statuses: a normative language rule is *decided*, a compiler feature may be *implemented*, and that implementation may remain *unvalidated*. “Validated” can only mean that defined verification criteria were met, never that any code is proven 100% defect-free.

## § 5. MD-025.4 — Eligible implementation targets

**§ 5(1).** An **interface** must not declare a `free` lifecycle member, and an interface itself must not become the target of an ordinary `impl` that supplies `free`. An interface has no own completed instance with independently owned fields whose destruction can be implemented this way.

**§ 5(2).** The previous paragraph does **not** forbid a concrete type with custom `free` from implementing an interface through its ordinary valid `impl Concrete implements Interface` declaration. Interface conformance and custom destruction are separate mechanisms.

**§ 5(3).** A **unit** must not declare custom `free`, including through a unit implementation surface. Units express measurement semantics, not an independently destructible resource instance.

**§ 5(4).** A **register** type is **not forbidden** from declaring custom `free` by this correction. Register behavior and the representation of instance-bound data may evolve in Sec 0.2. Do not introduce a restriction now merely because today's layout/member model may evolve; applicable hardware/volatile/ISR rules continue to constrain actual operations.

**§ 5(5).** For other eligible nominal targets—structs, unions, enums, named scalars, and generic named types—the ordinary `impl` eligibility rules remain in force. A custom `free` on such a type triggers § 2 copy classification. This paragraph does not extend `impl` eligibility to arbitrary structural, interface, or foreign types.

**§ 5(6).** Eligibility is a semantic restriction, not merely a parser grammar shape. Where parsing accepts syntactically valid members on a semantically ineligible target, the compiler must diagnose the target/member incompatibility.

## § 6. MD-025.5 — Exact-once destruction and provable double release

**§ 6(1).** Every still-owned nontrivial value must undergo exactly one valid terminal destruction action on the paths where it exists. The compiler must neither generate nor silently permit a **second known release of that same owned resource** through its own cleanup machinery.

**§ 6(2).** When custom `free` completes, or exits early under § 7, the compiler **continues** destruction of the remaining owned fields. Writing `free` does not turn off field destruction, and manual release within `free` does not automatically mark an owned field unavailable.

**§ 6(3).** The precise outstanding concern in the original `MD-025.5` is `destruction.md` **§ 15.4(4)**: the compiler must reject a *statically provable* manual release of an owned field's resource that would subsequently be released again by compiler-managed field destruction. This is narrower than the general exact-once rule; the two requirements must not be confused.

**§ 6(4).** Static proof must be based on sound semantic facts: the identity/provenance of the resource and a known release/consume effect attributable to the same resource on a reachable path, together with an already scheduled automatic release. **A coincidental numeric handle value, function name such as `Close`, or an unverified guess about opaque FFI behavior is not proof.**

**§ 6(5).** A proven conflict is a **compile-time error** with the manually releasing operation and automatically owning field identified. If the compiler cannot prove what an opaque external operation does, the compiler must not incorrectly assert that it proved a double release. Existing FFI/unsafe/resource contracts remain applicable; this does not make unverified manual release safe.

**§ 6(6).** A legal `discard` of an already unavailable **whole** Place remains a no-op according to `destruction.md` § 12.2; this is distinct from a second *actual* destruction. The compiler may issue an INFO advisory, subject to the established diagnostic configuration policy.

**§ 6(7).** **Implementation/validation boundary:** this correction locks the distinction and minimum proof obligation but does not claim that the current compiler already knows the release effects of arbitrary external APIs. Exact compiler-recognized effects, summaries, alias/provenance tracking and supporting tests must be verified during implementation. If a new public annotation or effect contract is needed, record a further MD rather than inventing one in this correction.

## § 7. MD-025.6 — Valueless early `return` in `free`

**§ 7(1).** A custom `free` body may contain **bare `return`**. It means an ordinary early exit from the user-defined body and produces no value.

**§ 7(2).** `return expression` is **forbidden** in `free`, including `return true`, `return Err(...)`, `return Ok(...)`, and a returned `Result`. The custom lifecycle member has no result type.

**§ 7(3).** On any reachable bare-`return` path, the compiler performs required cleanup of completed locals/temporaries in the `free` body, then proceeds to the **same remaining automatically managed field destruction and storage-related cleanup** that follows ordinary completion of the body. An early return must never skip these phases.

**§ 7(4).** Ordinary non-fallible source control flow applies to early exit; the presence of custom `free` does not justify prohibiting an early exit without a correctness benefit. `defer` remains forbidden inside `free` in Sec 0.1 under the existing `destruction.md` § 15.6 rule; this correction does not change it.

```sec
type Connection struct {
    Handle: int
}

impl Connection {
    free {
        if self.Handle == 0 {
            return
        }

        ReleaseHandle(self.Handle)
    }
}
```

The `Handle == 0` sentinel and `ReleaseHandle` are illustrative *application* conventions, not compiler-known Sec handle semantics. Their correctness belongs to the application/API contract. The language guarantees the cleanup continuation after the early `return`.

## § 8. Diagnostics and usability

**§ 8(1).** Required errors must explain **what failed, why the ownership invariant matters, and how to repair the source**, using relevant source positions and types. They must not present a copy violation as a generic parser error or silently insert a move.

**§ 8(2).** Representative diagnostics (message text illustrative; stable IDs belong to the canonical diagnostics registry):

```text
ERROR: Cannot implicitly copy 'Resource': its implementation defines 'free',
so the type is MoveOnly. Use an explicit move if ownership should transfer.

ERROR: Cannot discard owned field 'First' of 'ResourcePair': the enclosing
type defines 'free' and requires a complete owned value during destruction.

ERROR: 'free' is not permitted on an interface or a unit.

ERROR: 'return' in 'free' cannot return a value. Use bare 'return'.

ERROR: This release targets an owned field that will be released again by
automatic field destruction. Update the lifecycle state or remove the
manual release, as permitted by the field's actual contract.
```

**§ 8(3).** Advisory messages about redundant whole-value discard or other harmless operations may default to **INFO**, not WARNING. User-configurable promotion of INFO to WARNING or ERROR does not weaken mandatory errors for unsafe ownership operations.

## § 9. Compiler integration and required tests

**§ 9(1).** Parser/AST: recognize `free` as a lifecycle member (`FreeDeclaration`); keep the existing production; permit bare `return`; reject return values; reject invalid member placement with semantic diagnostics when syntactically well-formed.

**§ 9(2).** Member collection/type classification: merge valid `impl` fragments before deriving copyability. A custom `free` forces `MoveOnly`, including when its fields are otherwise copyable and when `free` appears in a valid extension fragment or generic template.

**§ 9(3).** Place/ownership analysis: apply the same prohibition to `discard owner.Field` as to `<-owner.Field` when the enclosing type has custom `free`; include nested projections and branch-dependent availability. Whole-value destruction/discard remains governed by normal discardability.

**§ 9(4).** Destruction and Semantic IR: represent custom-`free` invocation, body exit, field cleanup, and storage cleanup as ordered ownership-sensitive operations. All ordinary returns from `free` must converge to the valid cleanup continuation; optimizations must not erase or duplicate observable release.

**§ 9(5).** Tests must include at least:

1. Custom `free` on a struct of plain integers makes it `MoveOnly`; implicit copy rejected, explicit `:<-` accepted.
2. `MoveOnly` propagates through owned struct/array/union/Option/Result/generic positions when their standard rules require it.
3. Copy classification independent of source order, including `impl extends`.
4. Custom `free` does not independently imply `@mustUse`.
5. `discard owner.Field`, nested owned projections, and partial move rejected for a custom-`free` owner.
6. Legal whole-value `discard` accepted; subsequent identical discard is an allowed no-op without duplicate `free`.
7. Interface custom `free` and ordinary `impl` targeting an interface rejected; concrete conformance unaffected.
8. Unit custom `free` rejected; register custom `free` not rejected merely due to the register target category (other target-specific rules still apply).
9. Bare `return` in `free` accepted from ordinary and nested conditional paths; return of a value rejected.
10. Early `return` still destroys eligible owned fields once, in canonical order, and cleans body locals.
11. A known double release of the same owned resource is rejected; distinct resource identities are not mistakenly conflated.
12. Opaque FFI calls are not labeled statically proven to release a field without actual trusted release-effect evidence.
13. No duplicate destruction on partial/conditional paths and no cleanup elision or illegal reordering in optimized IR/backends.
14. Existing restrictions on `defer` inside `free`, moving fields from `self`, and direct `value.free()` remain enforced.

**§ 9(6).** Tests must run in the repository's relevant parser, Sema, Semantic IR, optimization and runtime execution suites where supported. Compare cold and incremental behavior where these analyses interact with cached copy classification and destruction plans. Do not report validation without actual test evidence.

## § 10. Rulebook integration and governance checklist

**§ 10(1).** Update `rules/memory/copy_move.md` §§ 3–4 with the custom-`free` `MoveOnly` rule and propagation. Preserve the distinction between `@noCopy`, `@mustUse`, non-trivial destruction, and future linearity.

**§ 10(2).** Update `rules/memory/ownership.md` § 19 and `rules/memory/destruction.md` § 10.3 to make field `discard` as illegal as a partial move for completed owners declaring custom `free`. Reconcile `rules/control-flow/discard.md` accordingly without altering its general no-op rule for already unavailable Places.

**§ 10(3).** Correct the obsolete `rules/foundations/grammar.md` “`free` operation syntax” wording and keep `FreeDeclaration ::= "free" Block`. Align `rules/declarations/impl.md` § 19 with eligible targets, MoveOnly classification, and early-return semantics.

**§ 10(4).** Update `rules/memory/destruction.md` §§ 15.4–15.7 with the static-proof boundary and early-return cleanup continuation. Reconcile interface and unit rulebooks. Do not impose a blanket custom-`free` ban on registers.

**§ 10(5).** Update diagnostics/LSP, Semantic IR ownership/destruction verification and compiler testing rules only as necessary for these requirements. Keep existing diagnostic IDs and numbered canonical paragraphs stable, adding subordinate material rather than renumbering.

**§ 10(6).** Reconcile status per individual requirement: `MD-025.3` parser recognition and `MD-025.6` accepted bare return are **reported implemented but unvalidated**; other behavior, especially custom-`free` copy classification and owned-field discard rejection, requires implementation or verification. Document any discovered implementation bugs under existing governance `identified_bugs` entries or appropriately non-duplicating IDs.

**§ 10(7).** Do not remove `MD-025` from the authoritative missing-decisions backlog solely because design choices are locked. Close it only when canonical rules, implementation, tests, and governance are synchronized according to project policy; record any independently discovered implementation gaps as new MD items when they require a genuine design choice.

---

## § 11. Source baseline and audit note

**§ 11(1).** Based on the public Sec `main` rulebooks reviewed on 2026-10-15: `destruction.md` revision 2.0 §§ 10.3, 12.2, 15.4–15.7; `copy_move.md` §§ 3–6; `impl.md` §§ 2, 19 and 24; `grammar.md` `FreeDeclaration` and “`free` operation syntax”; and `ownership.md` § 19. Their current contents can change before this correction is merged.

**§ 11(2).** This file does not constitute a new compiler audit. Implementation statements in § 10(6) come from the recorded MD-025 audit description and have not been independently proven by running the compiler here.

**§ 11(3).** References used for integration:

- `https://github.com/YoePro/sec/blob/main/rules/memory/destruction.md`
- `https://github.com/YoePro/sec/blob/main/rules/memory/copy_move.md`
- `https://github.com/YoePro/sec/blob/main/rules/memory/ownership.md`
- `https://github.com/YoePro/sec/blob/main/rules/declarations/impl.md`
- `https://github.com/YoePro/sec/blob/main/rules/foundations/grammar.md`
- `https://github.com/YoePro/sec/blob/main/rules/declarations/interfaces.md`
- `https://github.com/YoePro/sec/blob/main/rules/control-flow/discard.md`
