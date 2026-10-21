# MD-063 — Explicit whole-value access through safe references

- **Status:** Applied 2026-10-21 — the owner approved introduction after a merit analysis ("Isåfall - inför"); integrated into the rulebooks listed below and implemented in parser, Sema and Semantic IR. Known holes are listed in the application note.
- **Created:** 2026-10-21
- **Last updated:** 2026-10-21
- **Document revision:** 0.1 (review draft)
- **Sec language version:** 0.1
- **Proposed canonical path:** `rules/corrections/md063-explicit-whole-value-dereference.md`
- **Replaces:** Nothing; supplements the MD-063 entry in `missing-decisions.yaml`
- **Affected normative rulebooks:** `rules/memory/references.md`, `rules/memory/reference_model.md`, `rules/memory/borrowing.md`, `rules/memory/copy_move.md`, `rules/foundations/operators.md`, `rules/compiler/semantic_ir.md`
- **Related compiler package:** P15 (and downstream P16/P17 behavior)
- **Repository review:** Public `main` rulebooks and `missing-decisions.yaml` inspected 2026-10-21. Exact `main` HEAD commit SHA could not be verified; recheck these files against the actual integration baseline before application.

> **Application note (2026-10-21).** Merit analysis: before this correction the compiler could neither read nor replace a whole referent (`r + 1`, `r = 5`, `r += 1` and `return r` as a value were rejected, and `*r` was a parse error), while `references.md` § 3.2 showed `value += 1` on a `ref mut int` and `operators.md` already listed a "mutable dereferenced reference" as an assignment target. The proposal closes that gap without implicit conversions and reuses the existing `ref.deref-place` / `place.read` / `place.write` Semantic IR, so it was introduced.
>
> **Integrated into:** `rules/memory/references.md` rev 2.2 (§§ 3.2, 3.4 new, 8(8), 20(6), 28(2), 30(9)); `rules/foundations/operators.md` ("Dereference", "Line-start dereference statements", precedence table, compound-assignment targets); `rules/foundations/grammar.md` ("Prefix expressions", "Assignment statements"); `rules/compiler/semantic_ir.md` § 45(9a). `reference_model.md`, `borrowing.md` and `copy_move.md` were audited and need no change: their rules apply to `*r` as to any Place.
>
> **Decisions added during application:** (1) Line-start ambiguity, not covered by this review: the formatter writes multiline products with a leading `*` (formatter § 9(13)), so the owner decided (2026-10-21) that a `*` beginning a later line is a dereference that starts a new statement only when it introduces a syntactically valid assignment target directly followed by an assignment operator (or `++` / `--`). The search may cross lines inside the target's brackets but never passes into another standalone statement. Multiplication on a continuation line otherwise keeps its meaning, and the decision is syntactic, never type-driven. This only reinterprets source that would otherwise be an invalid chained assignment. (2) `*` on a slice view (`ref T[]`) is rejected explicitly, because a view has no whole referent value. (3) As the base of a projection, `(*r).Field` performs no whole-value read, so it is valid for a move-only `T`.
>
> **Implementation:** `internal/parser/expression.go`, `internal/parser/parser.go`, `internal/parser/grammar_metadata.go`; `internal/sema/reference_dereference.go` (new) with hooks in `analyzer_inference.go`, `analyzer.go`, `analyzer_lifetime.go`, `synchronization_resources.go`, `reference_plans.go` and `mentor_try.go`; `internal/ir/semantic/reference_use.go` (whole-referent Place and compound write through a reference). Tests: `internal/parser/dereference_test.go`, `internal/sema/reference_dereference_test.go`, `internal/ir/semantic/reference_dereference_test.go`.
>
> **Known hole (verified 2026-10-21):** the native track (`emit-sec-mlir`, `build`) stops at package 14, so `*r` reaches Semantic IR only through package-15 builds. Non-trivial replacement or non-copy-trivial reads through `*r` stay an explicit P15 lowering boundary. No diagnostic IDs were added (§ 15(3)): the authority, copy and move rejections reuse S1165 and S1007; the holder and operand rejections have no ID yet.
>
> The original review text follows unchanged.

---

## § 1. Scope and question

**§ 1(1)** MD-063 concerns how Sec source code reads or replaces the **whole referent** through a safe reference (`ref T` or `ref mut T`) and how this is distinguished from operations on the reference holder itself.

**§ 1(2)** The underlying memory model is already defined: a reference is a non-owning value; its referent is a canonical `Place`; borrowing constrains access; Sema resolves the operation before Semantic IR generation. MD-063 shall not redesign these facts.

**§ 1(3)** The design goals are explicit programmer intent, type consistency, equal semantics for parameters and local reference holders, and no context-sensitive implicit conversion of `ref T` into `T`.

**§ 1(4)** The following clauses are **proposed normative language** awaiting review. Existing MD-061 reborrow, copy/move, and call-site borrowing rules remain in force independently of the approval of this proposal.

## § 2. Proposed syntax and classification

**§ 2(1)** Prefix `*` applied to a safe reference expression shall denote **dereference to its referenced `Place[T]`**. Prefix `*` is a source-level unary operator; it does not create an owner, a copy, or a separate borrow merely by naming a Place.

```sec
let mut number := 10
let r := ref mut number

let snapshot := *r
*r = 25
*r += 1
```

**§ 2(2)** In a value context, a dereferenced Place is read in accordance with the copy/read rules for `T`. In an assignment target context, it denotes the referent destination Place. The expression's context determines the permitted operation, not a silent conversion of the reference itself into `T`.

**§ 2(3)** `*` on `ref T` yields a **read-only** referent Place; `*` on `ref mut T` yields a **read/write** referent Place, subject to current borrow, Place availability, lifetime, storage, and mutability restrictions.

**§ 2(4)** Dereferencing a valid **safe** reference does not itself require `unsafe`. The validity, generation and provenance checks required by the reference model remain applicable.

**§ 2(5)** No `&` address-of operator, `.Value` synthetic member, `.value` fallback, special `ref r = ...` assignment syntax, or implicit whole-value dereference is introduced by MD-063.

## § 3. Distinct forms of access

**§ 3(1)** These distinct operations retain these meanings:

| Source form | Semantic meaning |
|---|---|
| `ref owner` | Create a shared borrow of an eligible Place |
| `ref mut owner` | Create an exclusive mutable borrow of an eligible Place |
| `ref r` where `r` holds a reference | Shared **reborrow of the referent**, as fixed by MD-061 |
| `ref mut r` where `r` holds a mutable reference | Exclusive **reborrow of the referent**, as fixed by MD-061 |
| `r` | Reference holder value, of type `ref T` or `ref mut T` |
| `*r` | Referent `Place[T]`, subject to its access mode |
| `r.Field` | Existing projection of a member on the referenced Place |
| `r = otherReference` | Assignment/rebinding of mutable holder, if legal |
| `*r = newValue` | Replacement of the whole referent, if legal |

**§ 3(2)** `ref r` and `*r` are **not synonyms**. The first produces a reference value (reborrow), the second denotes the existing referent Place for subsequent use. This distinction holds regardless of expected type.

**§ 3(3)** The spelling and semantics of the above operations are **identical** whether `r` is a local binding, a parameter, a field, or a valid expression returning a safe reference. No parameter-only implicit whole-value dereference is permitted.

## § 4. Whole-referent reads

**§ 4(1)** A value read through a reference shall be explicitly spelled `*r`, or the corresponding parenthesized dereference of a more complex reference expression. The bare reference value `r` shall not implicitly become a `T` in arithmetic, boolean operators, comparisons, assignment, returns, or ordinary by-value call arguments.

```sec
fn ReadNumber(r: ref int) int {
    return *r
}

fn AddOne(r: ref int) int {
    return *r + 1
}
```

**§ 4(2)** A whole-value read is valid only if `T` supports the required by-value reading/copy operation. For a move-only referent, `let x := *r` is invalid when it would require a forbidden copy. `let x :<- *r` is also invalid: a borrow does not grant ownership that may be moved out.

**§ 4(3)** Existing non-consuming member/element projections through safe references are preserved. `r.Field` does **not** imply permission for a bare `r` to participate in scalar arithmetic or by-value calls. It is Place projection, not a general implicit whole-value conversion.

## § 5. Whole-referent writes

**§ 5(1)** Replacing the entire referent is spelled `*r = expression`. The reference must grant mutable authority and the target Place must be writable and legally replaceable.

```sec
fn SetNumber(r: ref mut int) void {
    *r = 25
}
```

**§ 5(2)** `r = expression` targets the **holder binding**; it never means replacement of the referent. An immutable holder cannot be reassigned. A mutable holder can be rebound only to a compatible reference value under normal borrow/lifetime/assignment rules.

```sec
let mut first := 10
let mut second := 20
let mut r := ref mut first

*r = 15                 // Writes to first.
r = ref mut second      // Rebinds the holder if borrow analysis permits it.
*r = 25                 // Writes to second.
```

**§ 5(3)** The mutable authority of `ref mut T` does not require that the **holder binding** be mutable. The following is valid when the referent is writable:

```sec
let mut number := 10
let r := ref mut number
*r = 25
```

**§ 5(4)** Replacement through `*r` shall follow the canonical assignment, contract, fallibility, ownership, and destruction ordering rules. Source evaluation and target evaluation occur only as many times as those rules permit; neither a hidden copy nor a hidden move out of the referent is allowed.

**§ 5(5)** Initial P15 support may be limited to trivially replaceable targets. Replacing resource-owning or otherwise non-trivial values is governed by the later applicable package/rulebook. Such implementation limits shall not be silently mistaken for different source-language syntax.

## § 6. Compound assignments and mutating operators

**§ 6(1)** For a whole referent, compound assignment uses explicit dereference:

```sec
fn Increment(value: ref mut int) void {
    *value += 1
}
```

**§ 6(2)** `value += 1` when `value: ref mut int` is **invalid**: it targets a reference value, not an integer Place. The same rejection shall apply to locals and parameters.

**§ 6(3)** `*r += expression` shall use the same evaluation-once, arithmetic-check, fallibility, old-value-preservation, and commit rules as compound assignment to any other writable Place. The target expression shall be evaluated exactly once.

**§ 6(4)** Existing `r.Field += expression` and `r[index] += expression`, where the latter is valid for the collection/reference category, remain legal Place-projection forms and do not require inserting `*` for the entire enclosing value.

## § 7. Reference equality versus referent equality

**§ 7(1)** `r == s` for compatible safe references compares **referenced live storage identity and semantic location**, according to `reference_model.md` § 47. Numeric address equality alone is insufficient.

**§ 7(2)** `*r == *s` compares the **referent values** according to the equality rules for `T`, provided those values are comparable and the required reads/accesses are legal.

```sec
let a := 10
let b := 10
let ra := ref a
let rb := ref b

let samePlace := ra == rb
let sameValue := *ra == *rb
```

**§ 7(3)** No implicit elementwise or referent-value equality is introduced for references. The comparison of a reference with a plain `T` shall not silently dereference either operand.

## § 8. Calls, parameters, methods, and returns

**§ 8(1)** A parameter declaration `value: ref T` or `value: ref mut T` continues to denote borrow authority; `*value` within its body denotes its referent Place. There is no special dereference exemption for function parameters.

**§ 8(2)** The pre-existing *call-site* rule remains: an owned compatible Place may be implicitly borrowed when the **selected parameter signature** requires `ref T` or `ref mut T` (`borrowing.md` § 15.2). That rule creates a borrow during argument preparation; it does not read `T` through a reference or silently move ownership.

```sec
fn Increment(value: ref mut int) void {
    *value += 1
}

fn Example() void {
    let mut number := 10
    Increment(number)
}
```

**§ 8(3)** Conversely, a function parameter requiring **owned/value `T`** is not satisfied by a bare `ref T`. The caller shall spell `*r` for an allowed whole-value read:

```sec
fn ConsumeNumber(value: int) void {
    // No reference retained.
}

fn ForwardNumber(r: ref int) void {
    ConsumeNumber(*r)
}
```

**§ 8(4)** Calling a method through a reference continues to follow the selected method's receiver contract and established auto-projection/receiver rules. MD-063 does not introduce a new method lookup rule or make consuming receivers possible through a borrow.

**§ 8(5)** `return *r` returns a **value** and requires its valid by-value read. `return ref r` would instead be a **reborrow**, whose escape/returned-reference legality must be proven independently. The two are never interchangeable.

**§ 8(6)** Overload selection must not change the meaning of `*r`, `ref r`, or a bare `r`. Existing call-site borrowing remains a documented, signature-directed argument-construction mechanism; it shall not be generalized into other expressions.

## § 9. Syntax, precedence, and formatter contract

**§ 9(1)** The parser shall recognize prefix `*` as a unary dereference operator only in prefix-expression position. In infix position `*` retains multiplication semantics.

**§ 9(2)** Normal postfix member, indexing, and call operators bind more tightly than prefix unary operators. Thus `*r.Field` parses as `*(r.Field)`, and `*r * 2` as `(*r) * 2`. To project from the dereferenced whole value explicitly, `(*r).Field` is syntactically unambiguous; ordinary reference member access should be written canonically as `r.Field`.

**§ 9(3)** The formatter shall preserve meaning and operator precedence. It shall not silently insert/remove whole-value dereferences or reinterpret a holder assignment as a referent assignment. Parentheses may be added/removed only with semantics preserved.

**§ 9(4)** No new compiler-known user member `Value`, no `&` token meaning, and no C-style raw pointer type syntax is implied by this prefix operator.

## § 10. Member access, indexing, and symmetry

**§ 10(1)** `r.Field` remains the canonical member-projection form through a safe reference; `r[index]` remains legal where the referenced type exposes indexable Places and the relevant collection contracts allow it.

**§ 10(2)** These are explicitly **Place-projection rules**, not implicit reading/replacing of a whole referenced `T`. They apply uniformly to local holders, parameters, fields, and returned references.

**§ 10(3)** `(*r).Field`, if syntactically used, is the composition of explicit whole-referent dereference and field projection. It shall not confer additional authority or different runtime effects. The formatter/documentation should prefer `r.Field` when equivalent. This is ordinary expression composition, not a second special-case syntax for a separate kind of reference.

**§ 10(4)** No source context may give a parameter holder an implicit whole-value operation that the same locally declared reference holder would be denied.

## § 11. Borrowing, availability, lifetimes, and reborrows

**§ 11(1)** `*r` may be evaluated only while `r` and its referent remain valid and authorized. Existing provenance, generation/epoch, spatial validity, access-context, lifetime, and availability rules are unchanged.

**§ 11(2)** Creating `*r` does not change the owner of the referent. Forbidding moves out of borrowed Places includes moves requested using explicit `<-`.

**§ 11(3)** Reborrowing `ref r` / `ref mut r` retains MD-061's semantics: it borrows the underlying referent, not the storage of the holder, and Sema creates the resolved child borrow with compatible authority and lifetime.

**§ 11(4)** A `ref mut` holder is move-only, but may be explicitly moved as a holder when permitted by copy/move and borrowing rules. Such a move transfers holder responsibility, never the ownership of the referent.

**§ 11(5)** Borrow live ranges end at their last required use on each path according to the separately resolved MD-065 correction. Whole-referent access is a use of the corresponding authority dependency; it must not artificially extend the borrow to scope exit.

## § 12. Safe references, optional references, and raw pointers

**§ 12(1)** The new `*` operation is defined for **safe** `ref T` and `ref mut T` only.

**§ 12(2)** `Option[ref T]` is not itself a reference; an `Option` value cannot be prefixed by `*` until a valid reference has been obtained through the ordinary Option handling path.

**§ 12(3)** `RawPtr[T]` is not a safe reference. MD-063 does not introduce `*rawPointer` as a safe dereference or decide raw-pointer reading, conversions, or unsafe access syntax (see MD-070 and `raw_pointers.md`).

**§ 12(4)** Volatile, addressed, hardware, atomic, synchronization, and unsafe contexts retain their specialized access and authority rules. Source `*` does not bypass them.

## § 13. Sema and canonical Place model

**§ 13(1)** Sema shall distinguish at least three categories in the resolved expression: **holder value**, **referent Place**, and **read value**. These shall not be conflated merely because a target machine may represent them using the same address or register.

**§ 13(2)** Sema shall type the prefix dereference of `ref T` / `ref mut T` as the referenced Place with appropriate access rights. A value-use of that Place shall invoke the ordinary copy/read classification of `T`; an assignment-use shall invoke ordinary target/replace classification.

**§ 13(3)** A bare reference holder has reference type; numeric operators, whole-value target assignment, and by-value `T` arguments shall not silently change its type. Rebinding the holder is a distinct typed assignment operation.

**§ 13(4)** Diagnostics for an invalid whole-value access shall point to the reference expression, its source reference type, the denied operation, and a safe correction where one exists. No speculative automatic fix may insert `*` where doing so would require an invalid copy or a forbidden move.

## § 14. Semantic IR / MLIR representation

**§ 14(1)** Semantic IR shall retain the established reference facts: origin/provenance, canonical storage and Place identity, bounds, mutability, borrow/lifetime dependency, and validity policy.

**§ 14(2)** The source prefix `*r` shall resolve before Semantic IR generation to the **referent Place** operation already required by `semantic_ir.md` § 45(8). A value read and a whole-value replacement are separate subsequent operations.

**§ 14(3)** The canonical lowering distinction is conceptual:

```text
r              -> reference holder value
*r             -> reference-to-Place resolution
read(*r)       -> validated whole-value read/copy
*r = expression -> validated whole-Place replacement
r = otherRef   -> typed holder rebinding
```

These lines describe resolved meaning, not new Sec syntax or an exact list of IR opcode spellings.

**§ 14(4)** The IR verifier checks structural consistency and resolved facts without re-running an independent source-level borrow/type solver. Backends must not re-infer dereference semantics from raw addresses or AST spelling.

**§ 14(5)** For P15, the first executable subset may support only trivial reads/replacements, with non-trivial copy/replacement lowering deferred to the applicable later package. Tests must distinguish unimplemented lowering from language-invalid operations.

## § 15. Diagnostics and negative constraints

**§ 15(1)** Negative examples and their required diagnostic intent:

```sec
fn InvalidArithmetic(r: ref int) void {
    let x := r + 1               // Invalid: ref int is not int.
}

fn InvalidHolderWrite(r: ref mut int) void {
    r = 25                       // Invalid: holder is a ref mut int, not int.
}

fn InvalidSharedWrite(r: ref int) void {
    *r = 25                      // Invalid: shared reference grants no write authority.
}

fn InvalidRefCompound(r: ref mut int) void {
    r += 1                       // Invalid: compound operator targets holder.
}
```

**§ 15(2)** Diagnostics shall explain *why* the operation is invalid and distinguish “add `*` to access the referenced value” from “cannot copy/move this move-only referent,” “holder is immutable,” or “borrow authority is insufficient.”

**§ 15(3)** This draft does not assign diagnostic IDs. Existing diagnostics and naming conventions shall be audited before adding new IDs; tests shall not hardcode guessed identifiers.

## § 16. Required regression and conformance matrix

**§ 16(1)** The integration test suite shall cover the following cases at parser, Sema, Semantic IR, and available backend stages:

| Case | Expected result |
|---|---|
| Prefix `*r` where `r: ref int` | Valid read of int |
| Prefix `*r` where `r: ref mut int` | Valid read of int |
| `*r = 25` for writable `ref mut int` | Valid whole-referent write |
| `*r += 1` for writable `ref mut int` | Valid compound write; target once |
| `r += 1` for `ref mut int` | Compile-time rejection |
| `r = 25` where `r` is reference-typed | Compile-time type/binding rejection |
| `r = ref mut other` on mutable holder | Rebind only if borrow/lifetime proof allows |
| `*r = 25` for `ref int` | Compile-time authority rejection |
| `r.Field` and `r.Field = value` | Existing member projection behavior preserved |
| `r[index]` | Existing index projection rules preserved |
| `r == s` | Reference identity comparison |
| `*r == *s` | Referent value comparison when legal |
| `ref r` / `ref mut r` | MD-061 reborrow behavior unchanged |
| `*r` as by-value function argument | Legal only when value can be read/copied |
| `r` as argument to borrowed parameter | Pre-existing call-site/reborrow rule applies |
| `r` as by-value `T` argument | No silent referent read |
| `return *r` | Legal only under return type/read restrictions |
| `return ref r` | Existing reference-origin/escape rules apply |
| `let x := *r` where `T` is move-only | Reject forbidden copy |
| `let x :<- *r` | Reject move from borrowed referent |
| `*optionRef` where expression is Option[ref T] | Reject non-reference operand |
| `*rawPointer` | No new safe raw dereference |
| `*r.Field`, `(*r).Field`, `*r * 2` | Correct parse/precedence; no surprise writes |
| Reference parameter vs local reference | Identical whole-value access semantics |
| Shared/mutable reference lifetime after last use | MD-065 semantics preserved |
| Invalid generation / stale reference access | Preserve canonical reference-model failure behavior |

**§ 16(2)** Compound assignment tests shall exercise evaluation order, failure/rollback, alias conflicts, and any applicable contract/volatile behavior without requiring P15 to implement later-package semantics prematurely.

**§ 16(3)** Conformance tests shall not equate the lack of a currently available lowering route with a source-language rejection.

## § 17. Required rulebook and tool integrations after approval

**§ 17(1)** Amend `rules/memory/references.md` to specify `*` source syntax and fix § 3.2's implicit whole-value example. Preserve reborrow source forms and safe-reference invariants.

**§ 17(2)** Amend `rules/foundations/operators.md` to declare prefix dereference, precedence, assignment/compound-assignment use, and reference-versus-value comparisons. Preserve existing numeric multiplication and member projection semantics.

**§ 17(3)** Amend `rules/compiler/semantic_ir.md` only to tie its existing explicit whole-value operations to the resolved source spelling and appropriate provenance/source locations. Do not insert syntax parsing into Semantic IR.

**§ 17(4)** Audit `rules/memory/reference_model.md`, `borrowing.md`, `copy_move.md`, function/method rules, the formatter, LSP, and compiler tests for examples or code that silently assumes implicit whole-value dereference. Modify only actual conflicts; do not rewrite unrelated normative decisions.

**§ 17(5)** P15 shall receive parser, Sema and IR tests for trivial whole-value operations; unsupported later-package lowering shall remain explicit. A downstream package can extend implementation support without redefining the source syntax.

**§ 17(6)** After the owner accepts the correction and canonical rulebooks are updated, synchronize the applicable governance fragments and remove or close MD-063 in `missing-decisions.yaml` only when the registry's stated completion conditions are met.

## § 18. Review status and decision rationale

**§ 18(1)** The owner expressed qualified agreement that prefix `*` likely communicates useful programmer intent, but explicitly requested a second review **before** the change is introduced. Therefore this file stays **Proposed / Not applied**.

**§ 18(2)** Rejected direction for this proposal: `.Value` or a compiler-known `.value` member. It collides conceptually with ordinary member namespaces and does not distinguish referent access as clearly as an operator.

**§ 18(3)** Rejected direction for this proposal: special `ref r = value` target semantics. `ref r` already means reborrow under MD-061 and should not switch meaning by expression context.

**§ 18(4)** Rejected direction for this proposal: a bare holder such as `r` implicitly behaving as `T` in arithmetic, comparison, and by-value calls. Such behavior obscures intent and risks context-dependent interpretation.

**§ 18(5)** Chosen proposal: reserve `*r` for explicit **whole** Place access, while keeping `r.Field` / eligible `r[index]` as well-defined Place projections. This distinction is source-level consistent for all reference holders, irrespective of where they are declared.

**§ 18(6)** Items for owner's final reading: clarity of operator precedence (§ 9), reference holder rebinding versus referent replacement (§ 5), canonical member projection (§ 10), and the precise no-move-from-borrow boundary (§ 11). These are review prompts, not a request to reopen settled independent memory rules.

---

## Sources reviewed

- [MD-063 in missing-decisions.yaml](https://github.com/YoePro/sec/blob/main/missing-decisions.yaml), area `whole-value-access-through-references`.
- [references.md](https://github.com/YoePro/sec/blob/main/rules/memory/references.md), especially §§ 3, 5, 7–8.
- [borrowing.md](https://github.com/YoePro/sec/blob/main/rules/memory/borrowing.md), especially §§ 8, 11.4, 15.2.
- [reference_model.md](https://github.com/YoePro/sec/blob/main/rules/memory/reference_model.md), especially § 47.
- [copy_move.md](https://github.com/YoePro/sec/blob/main/rules/memory/copy_move.md), especially §§ 4.4–6.
- [operators.md](https://github.com/YoePro/sec/blob/main/rules/foundations/operators.md), especially assignment, compound assignment and reference equality rules.
- [semantic_ir.md](https://github.com/YoePro/sec/blob/main/rules/compiler/semantic_ir.md), especially §§ 45–46.
- [functions.md](https://github.com/YoePro/sec/blob/main/rules/declarations/functions.md), function/parameter rules.

**Integration instruction:** Check actual `main` HEAD before using the document. If any authority or already-implemented behavior has changed, reconcile that change explicitly and preserve this document's paragraph numbers across revisions. Do not silently promote draft wording into normative authority.
