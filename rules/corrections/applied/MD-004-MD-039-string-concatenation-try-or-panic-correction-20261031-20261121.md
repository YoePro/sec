# MD-004 / MD-039 — String concatenation: explicit `try` or ordinary panic

- **Decision IDs:** MD-004 (narrow amendment); MD-039 (resolved)
- **Area:** `string-concatenation-failure-policy`, `compound-string-append-failure-policy`
- **Document kind:** Approved normative correction and implementation handoff
- **Status:** **APPROVED BY LANGUAGE OWNER — DESIGN**; canonical rulebook/governance synchronization, compiler implementation and acceptance tests pending
- **Created / approved:** 2026-10-31
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Reviewed repository:** `YoePro/sec`, `main` commit `40376a4ee6a63eff40d440964e00d62d27fc253d` (verified 2026-10-31)
- **Supersedes in scope:** MD-004's mandatory-`try` rule for ordinary string concatenation, and the undecided compound-string-append failure policy of MD-039. Other MD-004 guarantees remain intact.
- **Primary normative owners:** `rules/foundations/operators.md` (String concatenation, Compound concatenation assignment); `rules/memory/allocation.md` §§ 10, 17
- **Related normative owners:** `rules/errors/errorhandling.md` §§ 9, 11–14, 23, 31; `rules/errors/runtime_checks.md` (Allocation); `rules/errors/panic.md` §§ 3–4, 13–14, 25–28; `rules/foundations/attributes.md` (`@noPanic`, `@noAlloc`); `rules/foundations/grammar.md`; `rules/library/core-library.md` § 15.1; `rules/compiler/semantic_ir.md`
- **Previous correction to reconcile:** `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 5 (MD-004)
- **Implementation tracking:** existing `governance/allocation.yaml` (`frontend.allocation` and applicable analysis/lowering/target/tooling children); applicable `governance/errors.yaml`, `governance/errors_diagnostics.yaml`, and existing operator/compiler ownership. Reuse existing IDs and owners; do not create duplicates merely to record MD-039.

> **Integration instruction:** Treat this document as the newly approved language policy, **not** proof of implemented parser, Sema, Semantic IR, lowering, formatter, LSP or target behavior. Integrate **one existing normative section at a time**; test that change before the next step. Preserve stable rulebook § numbers, published diagnostic identities and governance IDs. Keep the original applied MD-004 correction as historical provenance; supersede only its contradictory clauses through the canonical owners and this dated correction. Do not mark implementation complete or remove MD-039 until rulebooks, governance, tests and relevant compiler phases agree.

---

## § 1. Approved scope

**§ 1(1).** In Sec 0.1, ordinary `string` concatenation using binary `+` and compound `string +=` has two source-selected failure modes:

1. **Explicit fallible mode:** `try` protects the operation and exposes its canonical `StringError` failure channel for handling or propagation.
2. **Ordinary mode:** without a protecting `try`, failure to materialize required string storage reaches Sec's non-returning **panic** mechanism instead of causing a missing-`try` compile-time error.

**§ 1(2).** The default is selected by source syntax, not by function return type, build mode, optimization, target, expected type, or compiler preference. A program may choose the two modes at different call sites.

**§ 1(3).** This decision does **not** make `ToString()` infallible, implicitly unwrap `Result`, permit arbitrary hidden type-to-string conversions, or change other allocating APIs. In particular, it does not repeal the general allocation error discipline of `allocation.md` § 10 for unrelated allocation calls.

**§ 1(4).** The decision explicitly covers binary `+` and compound `+=`. Runtime string **interpolation** also appears in the original MD-004 correction, but its implicit-panic policy was **not separately approved** in this MD-039 decision; do not silently extend the change to interpolation. When synchronizing MD-004, split combined descriptions of concatenation and interpolation rather than changing interpolation by implication. The precise interpolation alignment, if desired, requires its own explicit decision.

## § 2. Source forms and error mode

**§ 2(1).** For a runtime concatenation or append whose only failure is storage materialization:

```sec
let ordinary := "Hello " + name        // Ordinary: materialization failure panics.
let recoverable := try "Hello " + name // Fallible: materialization failure is StringError.

let mut text := "Hello"
text += suffix                         // Ordinary: materialization failure panics.
try text += suffix                     // Fallible: materialization failure is StringError.
```

These are illustrative statement fragments; any naked `try` must have a compatible enclosing error channel or a local handler.

**§ 2(2).** Under `try`, an allocation failure uses `StringError.Allocation(AllocationError)`, the existing canonical text error family (`core-library.md` § 15.1; `allocation.md` § 17(8)). Do not expose a bare `AllocationError` or introduce a second string-specific error type. Checked materialization-size failure follows the appropriate existing text/materialization classification; it must not silently wrap or truncate.

**§ 2(3).** Without `try`, failure of storage materialization invokes a compiler-visible, non-returning language panic. The panic does not return `Err`, does not implicitly propagate through the caller's `Result`, and is not caught by `try` elsewhere. The panic endpoint must be valid even after an allocation failure (the existing allocation-free minimum panic path).

**§ 2(4).** All other success semantics are identical: the resulting `string` value, exact-once/left-to-right evaluation, ownership, lifetime, active allocation context, and string encoding guarantees do not depend on the chosen failure mode.

**§ 2(5).** If the compiler proves the concatenation is entirely compile-time materialized, there is no runtime allocation, no allocation panic and no runtime `StringError`. Both source spellings may be written; `try` selecting the defined string-materialization operation remains legal even if constant folding proves its failure path unreachable. Ordinary infallible expressions unrelated to language-defined string materialization do not thereby become valid `try` operands.

## § 3. Result-producing text operands

**§ 3(1).** The previously approved direct text operands are `string`, `char`, `rune`, and **exactly** `Result[string, StringError]` where the string-concatenation rule admits fallible text segments. This acceptance now applies equally to `+` and `+=`.

**§ 3(2).** The outer `try` handles the text operand's `StringError` **and** the concatenation/append materialization's `StringError` in one protected operation:

```sec
let text := try "Value: " + value.ToString()
try text += value.ToString()
```

**§ 3(3).** A separately protected operand only handles that operand. The outer ordinary `+` or `+=` still uses the **panic** mode for its own allocation failure:

```sec
let text := "Value: " + (try value.ToString())
text += try value.ToString()
```

**§ 3(4).** The following forms remain invalid, because a naked `Result` must never be implicitly unwrapped or changed into a panic:

```sec
let text := "Value: " + value.ToString()   // Missing handling of Result operand.
text += value.ToString()                   // Missing handling of Result operand.
```

**§ 3(5).** No arbitrary `Result[Other, OtherError]`, `Option`, numerical value, or user-defined object becomes a text operand. Inapplicable operand types retain the existing `S1022` diagnostic or its stable canonical equivalent. The compiler must not automatically call `ToString()`.

**§ 3(6).** A `try` operation remains subject to normal `Result` propagation/handler compatibility, including `StringError` widening where explicitly supported. Any panic that a called `ToString()` may independently cause is **not** converted into a recoverable `StringError` by this decision.

## § 4. Compound assignment and transactional commit

**§ 4(1).** `string +=` continues to evaluate the target Place once, evaluate the appended operand once in source order, prepare the resulting string, and **commit only after successful preparation**. The assignment statement produces no value on success, as for other compound assignments.

**§ 4(2).** On failure during evaluation or materialization of the appended value, the append itself makes **no partial commit** to the destination. Under `try` the failure is recoverable; in ordinary mode a materialization failure panics before commit. Neither mode promises to undo arbitrary independent side effects the programmer explicitly caused while evaluating the operand.

**§ 4(3).** An in-place append optimization is permitted only when the compiler proves the existing requirements: unique ownership, adequate capacity, absence of conflicting aliasing, stable storage and compatible allocation context. An optimization must preserve **both** the selected failure mode and transactional commit behavior; it may not leave a partial string after failure.

**§ 4(4).** Generic fallible assignment and fallible property-setter rules (`errorhandling.md` § 23 and following) remain in force. A normal `string +=` without `try` is not an unhandled fallible setter; it is a distinct **ordinary panicking form** of a compiler-known string operation. No general non-`try` fallible assignment is introduced.

## § 5. Allocation contexts, guarantees and panic identity

**§ 5(1).** Both modes use the **same** resolved active allocation context. Missing or invalid context is still a **compile-time error**; the compiler may not replace it with a guessed allocator, an allocator in another lifetime domain, or a panic. Targets forbidding dynamic allocation require the existing proof that the materialization can be eliminated.

**§ 5(2).** `allocation.md` § 10(3) currently forbids an ordinary allocation failure from panicking. The implementation must explicitly amend this rule narrowly: **an operation that canonically declares both a recoverable `try` mode and an ordinary panic mode may panic in the latter**, without changing the default semantics of allocator APIs and other fallible allocations.

**§ 5(3).** Under `@noPanic` (including restrictions inherited from interrupt/ISR contexts), an unprotected `+` or `+=` that might need failing allocation violates the guarantee. A protecting `try` may satisfy the guarantee only when its `StringError` is legally handled or propagated and all other reachable panic effects are absent. A static proof that the operation needs no failing allocation may eliminate that panic effect.

**§ 5(4).** Under `@noAlloc`, either mode is rejected when it has a reachable allocation effect; `try` is not permission to allocate. Existing allocation and target restrictions remain authoritative.

**§ 5(5).** `panic.md` § 13 requires a **stable registered `PanicID` for every implicit language panic**. The ordinary string-materialization failure must use an approved, semantically suitable existing panic reason or obtain a new stable allocation/materialization-failure reason in the canonical diagnostics/panic registry. **Do not invent a numeric ID in a rulebook or silently substitute `ExplicitPanic`, `AssertionFailed`, or an unrelated reason.** Preserve the source site and minimum allocation-free panic path.

## § 6. Canonical rulebook and diagnostics synchronization

**§ 6(1).** In `rules/foundations/operators.md`, update **without renumbering**:

- **String concatenation / Fallible text operands:** retain the `string`/`char`/`rune` matrix, add the same admissible `Result[string, StringError]` operand for `+=`, and remove the blanket requirement that runtime `+` must have `try`.
- **Runtime allocation and failure policy:** document source-selected `try`/panic for `+`; replace the incorrect `AllocationError`-only `try` flow with `StringError.Allocation`.
- **`@noPanic` and `try` scope:** describe both source forms and the independent Result/panic effects of operands.
- **Compound concatenation assignment:** remove the unresolved choice and state § 3 and § 4 precisely.
- **Implementation status / test matrix:** correct old pending bullets that say every unprotected string concatenation must be rejected; do not claim lowering or transactional semantics implemented.

**§ 6(2).** In `rules/memory/allocation.md`, amend §§ 10 and 17 for the narrow exception to the normal allocation failure policy and preserve canonical allocator context/target/lifetime guarantees. Update the historical MD-004 cross-reference carefully: its rules for interpolation are not implicitly changed by this correction.

**§ 6(3).** In `rules/errors/errorhandling.md`, `rules/errors/runtime_checks.md`, `rules/errors/panic.md` and attributes, ensure that `try` selection, `Result` obligations, panic reasons, and no-panic/no-alloc proofs describe the same operation. Preserve all existing rules for unrelated fallible assignments and operations.

**§ 6(4).** `rules/foundations/grammar.md`, parser/AST, Sema, Semantic IR, and lowering must represent `try` on a compound string assignment, including the `Result[string, StringError]` operand and its whole-expression failure set, without reinterpreting `try` as a general panic catcher or changing its precedence globally.

**§ 6(5).** Existing `S1097` (`operators.string-materialization-requires-try`) must **not** be emitted merely because ordinary string `+` or `+=` lacks `try`. Keep its published identity reserved and use it only for an actually still-required `try` context if its meaning is not misleading; otherwise retire it without reuse. `S1096` must not reject `try string +=` as a non-fallible target, and `S1022` must not reject the specifically approved `Result[string, StringError]` operand when the whole expression is protected. Preserve `S1098` for invalid/missing allocation contexts. Diagnostics must explain the two modes and the correct `try` placement.

**§ 6(6).** LSP hover and quick fixes must distinguish (a) ordinary materialization may panic, (b) explicit `try` produces recoverable `StringError`, and (c) an unhandled `Result` operand is invalid. The formatter must preserve the programmer's source-selected `try` rather than adding or removing it.

## § 7. Focused conformance tests

**§ 7(1). Positive source / semantic tests:**

1. Ordinary runtime `+` and ordinary runtime `+=` compile, and their resolved plan records a possible allocation panic when not proven safe.
2. `try` runtime `+` and `try` runtime `+=` compile with recoverable `StringError.Allocation`; no materialization panic is introduced on the protected path.
3. `try "A" + value.ToString()` and `try s += value.ToString()` accept one protected, exact-once `Result[string, StringError]` operand plus materialization failure.
4. `"A" + (try value.ToString())` and `s += try value.ToString()` accept separately protected operands while retaining the ordinary outer panic mode.
5. Both ordinary and `try` source spellings on fully compile-time folded string literals produce the same constant result, with no runtime allocation or panic path.
6. Bare `char`, `rune` and `string` append operands retain their approved meaning; no hidden `ToString()` call.
7. On append failure, no partial destination commit; side effects external to the append itself remain governed by existing evaluation rules.

**§ 7(2). Negative source / diagnostics tests:**

1. Unprotected `"A" + value.ToString()` and `s += value.ToString()` remain errors for unhandled `Result`, **not** implicit panic unwrapping.
2. An unsupported `Result` element or non-text operand is rejected by the existing operand rules.
3. `try s += value.ToString()` is no longer rejected as non-fallible (`S1096`); an invalid `try` on an unrelated infallible assignment still is.
4. A missing/invalid allocation context is an error even in ordinary panic mode.
5. A possibly allocating ordinary form violates `@noPanic`; a possibly allocating form violates `@noAlloc` regardless of `try`.
6. A bare `try` with an incompatible enclosing error channel receives the normal propagation diagnostic.
7. An independent panic from a user operation inside the text plan is not converted to `StringError`.

**§ 7(3). Compiler, runtime and tooling gates:**

- A deliberately failing allocator/target hook distinguishes `Err(StringError.Allocation(...))` under `try` from a non-returning canonical panic without `try`.
- The ordinary panic exposes a registered stable reason, source location and no-allocation minimum endpoint.
- Semantic IR/MLIR/LLVM preserve source-selected failure paths, string-plan evaluation order, ownership and transactional commit; no backend silently changes mode.
- Tests verify that the compiler and LSP report the same effects, and formatter round-trips both syntaxes unchanged.
- A multi-file regression covers non-allocating folded strings, runtime text operands, direct fallible operands, `@noPanic`, `@noAlloc`, limited/freestanding profiles and missing allocation contexts.

## § 8. Governance and handoff

**§ 8(1).** Reconcile existing entries under `governance/allocation.yaml`, `governance/errors.yaml`, `governance/errors_diagnostics.yaml`, and the existing compiler/operator/semantic-IR/lowering owners. Add **stable machine-readable sub-IDs** for newly verified work if required, but do not invent duplicate top-level owners or overwrite existing implementation evidence.

**§ 8(2).** Update `missing-decisions.yaml` MD-039 from `open` to the repository's established **decided / implementation-pending** status after normative synchronization is verified. MD-004's earlier combined text policy is superseded only where this document explicitly says so. Avoid implying that interpolation or other allocations were also redesigned.

**§ 8(3).** Apply rulebook amendments and implementation steps in **small testable increments**: (1) normative source and allocation contract; (2) parser/Sema/operator plan; (3) diagnostics and LSP; (4) Semantic IR and backend failure/control flow; (5) focused conformance and regression acceptance. Test each increment before proceeding. Source signatures and user-facing semantics must drive IR/lowering, not vice versa.

**§ 8(4).** **Definition of done:** rulebooks and older correction references are reconciled, governance reflects actual evidence, the MD-039 ledger records the approved decision without prematurely claiming implementation, and source/IR/tooling/runtime tests distinguish every approved form.

---

## § 9. Final approved decision

**§ 9(1).** Sec 0.1 `string +` and `string +=` support **explicit `try` for recoverable `StringError`** and **ordinary omission of `try` for panic on storage-materialization failure**.

**§ 9(2).** `+=` supports `Result[string, StringError]` operands under an outer `try` exactly like `+`. Explicitly unwrapping the operand with an inner `try` leaves the outer concatenation/append in ordinary panic mode.

**§ 9(3).** The decision is approved at the language-design level; implementation and cross-rulebook synchronization remain separate work.
