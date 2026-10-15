# MD-015 and MD-061 — Typed Duration Boundaries, Explicit Reborrowing and Optimization Safety

- **Status:** Applied 2026-10-15 to `rules/types/temporal.md` (§§ 5.5, 5.7, revision 1.2), `rules/types/units.md` ("Quantity literals"), `rules/concurrency/mutex.md` (§ 12(1)), `rules/memory/borrowing.md` (§§ 8.1(7)-(8), 11.4, revision 2.1), `rules/memory/reference_model.md` (§ 12(7)), `rules/memory/lifetime_analysis.md` (§ 9.3(3)), `rules/foundations/grammar.md` (RefExpression), `rules/compiler/semantic_ir.md` (§ 46(10)-(13)) and `rules/tooling/diagnostics.md` (§§ 18(4a), 24(4a)); Sema implements literal-only duration materialization, explicit reborrow typing, A2005 and last-required-use liveness for plain reference holders; Semantic IR lowers explicit reborrows; project-level `[diagnostics.rules]` policy is applied by CLI and LSP. Remaining (governance): non-lexical liveness for aggregate/view holders, the Sec MLIR/LLVM optimization matrix (schema 11 not yet emitted), and target/CLI/category/preset policy levels.
- **Created:** 2026-10-15
- **Last updated:** 2026-10-15
- **Document revision:** 1.0
- **Sec language version:** 0.1 (no dependency on planned 0.2 linear-type features)
- **Canonical path:** `rules/corrections/applied/md015-md061-duration-reborrow-correction-20261015.md`
- **Decision IDs:** `MD-015` (final typed-value clarification), `MD-061` (explicit reborrow and compiler pipeline)
- **Amends:** `rules/types/units.md`; the current canonical temporal/duration rulebook; `rules/foundations/grammar.md`; `rules/memory/borrowing.md`; `rules/memory/reference_model.md`; `rules/memory/lifetime_analysis.md`; `rules/compiler/semantic_ir.md`; the applicable Sec MLIR borrow/reference dialect package; `rules/tooling/diagnostics.md`; and relevant compiler testing and LSP rules.
- **Amends prior correction:** `rules/corrections/pending/md015-time-unit-duration-conversion-correction-20261013.md` (or its actual integrated location), **§§ 6.1–6.5 and 10.2**, especially § 6.3 and its dynamic-typed-value example.
- **Governance:** Update *existing* temporal conversion, borrow/reborrow, Semantic IR, MLIR, diagnostics and testing status entries; do not create duplicate governance identities.
- **Replaces:** No published numbered paragraphs. Amend contradictory text in place, preserve existing paragraph identifiers and introduce new subordinate paragraphs where necessary.

---

## § 1. Purpose, authority and relationship to existing decisions

**§ 1(1).** This correction makes the distinction between *contextually materialized unit-bearing literals* and *already typed Sec values* explicit at `duration` call boundaries. It also makes `ref` applied to a reference holder an explicit reborrow of the original referent, not a reference to the holder.

**§ 1(2).** Sec borrowing remains based on control-flow use, lifetime, provenance and availability analysis. The last required use of a borrow may precede the end of the lexical scope. This correction does not introduce visible lifetime parameters or defer non-lexical lifetimes to Sec 0.2.

**§ 1(3).** Normative source behavior is decided before lowering. Semantic IR, Sec MLIR, general MLIR and LLVM transformations must preserve it. Optimization safety is an existing Sec guarantee that is made operational and testable here, not a new borrowing regime.

**§ 1(4).** No new keyword, public reference type, public cast type, `duration` constructor, or runtime borrowing lock is introduced.

## § 2. MD-015 — Distinguish quantity literals from typed values

**§ 2(1).** When a quantity literal specifies a unit but no numeric carrier, the canonical default numeric carrier is **`decimal`**, according to the existing unit rules. A source expression such as `5<s>` is an ordinary unit-bearing numeric quantity **unless** a permitted expected-type position contextually materializes the literal as `duration`.

```sec
let elapsed := 5<s>                 // Unit-bearing decimal quantity, not duration.
let explicit := duration(5<s>)      // Explicit duration conversion.
let timeout: duration := 500<ms>    // Contextual literal materialization.

fn Wait(d: duration) void {
    // Implementation is owned by the time/scheduling API.
}

Wait(5<s>)                         // Valid contextual literal materialization.
Wait(500<ms>)                      // Valid contextual literal materialization.
Wait(explicit)                     // Valid: argument is already duration.
```

**§ 2(2).** An identifier or another expression whose result has **already been assigned a non-`duration` numeric unit type** does *not* acquire `duration` merely because an assignment, parameter or initializer expects `duration`. This is true even if constant propagation or range analysis happens to prove its value exactly representable. Optimization-dependent proofs must not silently change source-level conversion requirements.

```sec
let seconds: int<s> := 5
Wait(seconds)                       // ERROR: expected duration, found int<s>.

let inferred := 5<s>
Wait(inferred)                      // ERROR: already typed as a unit-bearing quantity.

let converted := duration(5<s>)
Wait(converted)                     // Valid.
```

**§ 2(3).** Already typed quantities require a **written** conversion when a `duration` is needed. Existing checked-conversion rules, including `ConversionError`, exactness, nanosecond representation, overflow and optional explicit rounding, remain in force.

```sec
let seconds: int<s> := 5
let timeout := try duration(seconds) // Explicit checked conversion when required.
Wait(timeout)
```

The example uses the checked conversion form; it does not require the compiler to invent a fallible conversion when the established rules prove the particular explicit conversion infallible.

**§ 2(4).** The same distinction applies in every expected-`duration` position, not only to `Wait`: assignments, typed initializers, ordinary arguments, initializer arguments, and whichever additional positions the canonical contextual-literal rules permit. An overload's return type never selects the overload.

**§ 2(5).** The compiler shall not reinterpret a typed `int<s>`, `decimal<ms>`, or any other unit-bearing binding as an untyped literal by tracing it to its initializer. A typed `duration` binding, by contrast, passes normally as `duration`.

**§ 2(6).** The preceding MD-015 correction's **§ 6.3** permitted contextual conversion of a statically range-proven *runtime typed* quantity; that permission is **superseded**. Its § 10.2 test asking that a dynamically typed, domain-proven value be accepted implicitly must be **removed or rewritten**. Explicit conversions may still use static proof to eliminate unnecessary runtime checks. All other established MD-015 rules continue unchanged, including `duration` as signed 64-bit nanoseconds, negative durations, exact conversion by default, explicit `RoundingMode`, core time units and the ban on silent overflow.

**§ 2(7).** Type-mismatch diagnostics must explain the distinction, not say that `int<s>` is a wrong physical dimension. A diagnostic may show a literal-based `duration` declaration and an explicit conversion of the typed value. It must not advise a hidden conversion or an invented overload.

## § 3. MD-061 — Explicit reborrow syntax and result type

**§ 3(1).** When `r` is a holder of a safe reference, `ref r` or `ref mut r` **refers to the holder's referent**, not to the storage occupied by the reference holder. It is an explicit *reborrow*, rather than construction of an additional safe-reference layer.

| Source holder | Expression | Result | Validity |
|---|---|---|---|
| `r: ref T` | `ref r` | `ref T` | Valid shared reborrow |
| `r: ref mut T` | `ref r` | `ref T` | Valid shared reborrow |
| `r: ref mut T` | `ref mut r` | `ref mut T` | Valid exclusive reborrow, subject to exclusivity |
| `r: ref T` | `ref mut r` | — | Compile-time error: cannot elevate authority |

**§ 3(2).** Explicit reborrows retain the referent's canonical Place/provenance, storage dependency and generation/epoch validity conditions. They do not move the owned `T`, create a new owner, extend its storage lifetime or mint mutation authority.

```sec
let mut value := 42
let r := ref mut value
let view := ref r                 // ref int of value, not ref (ref mut int).

Inspect(view)
Modify(r)                        // Permitted after view's final required use.
```

`Inspect` and `Modify` are illustrative callable names, not new core APIs. Their actual contracts must agree with the displayed borrow modes.

**§ 3(3).** A mutable child reborrow temporarily restricts conflicting use through its mutable parent; a shared child restricts conflicting mutation but can coexist with compatible shared access. The parent becomes usable again after the child's **last required use**, unless another live dependency prevents this. Overlapping mutable rights are never simultaneously active.

**§ 3(4).** Reborrowing from `ref T` as `ref mut T` is always invalid. Reborrow compatibility must use the existing canonical Place overlap, path-sensitive provenance and alias rules; disjoint proven fields must not be conservatively conflated without cause.

**§ 3(5).** No implicit nested `ref ref T` or `ref ref mut T` is synthesized from the `ref` operator on an already held reference. This paragraph does not introduce a general reference-to-reference type syntax or modify the established rules for aggregate types that happen to contain references.

**§ 3(6).** The reported inconsistency in which the expression type checker produces `ref ref T` while `ResolvedReferencePlanOf` produces a reborrow must be corrected at a **single canonical typed/semantic resolution point**. The Semantic IR builder must consume the resolved reborrow plan and may not independently reinterpret the expression.

## § 4. Non-lexical borrow liveness, errors and ergonomic behavior

**§ 4(1).** The semantic lifetime of a borrow is the set of program points needed to preserve every reachable future use and retention dependency, including reference holders, derived reborrows, aggregate storage, closures, `defer`, returned references and relevant opaque-call contracts. It is **not** mechanically the end of the surrounding source block.

**§ 4(2).** Analysis is path-sensitive and respects loop backedges. The borrow ends at the last *required* use on a path, not necessarily at the last textual occurrence of its variable. A deferred or escaping use may extend the borrow. An unknown foreign retention contract must not be treated as call-bounded without proof.

**§ 4(3).** Existing borrow conflicts remain mandatory compile-time errors. Sec does not weaken exclusivity to improve ergonomics; instead it should avoid unnecessary conflicts by using the established last-use, disjoint-Place, finite-origin and reborrow rules rather than imposing lexical restrictions when not required.

**§ 4(4).** Passing an existing `ref mut T` to a `ref mut T` parameter uses the existing call-bounded reborrow rules. After the call and any legitimately retained dependency end, the parent reference is usable again. A call that retains a reference must preserve that extended lifetime, even if the local argument is not read again.

## § 5. Information diagnostics and user-controlled severity

**§ 5(1).** The explanatory diagnostic for an otherwise legal explicit reborrow is **Information (`INFO`) by default**, not Warning and not Error. It explains that the referent is the underlying value/Place, rather than the reference-holder variable. Example message:

```text
INFO: Reborrow of 'value' through reference holder 'r'.
Result type: ref int.
The expression refers to 'value', not to the storage of 'r'.
```

**§ 5(2).** This diagnostic is educational. The compiler must **not** claim that replacing `ref r` with `ref value` is always appropriate: when the owner is temporarily inaccessible because `r` holds exclusive authority, reborrowing through `r` may be the correct form. An information diagnostic is not evidence of unsafe source code.

**§ 5(3).** The user/project may **promote any `INFO` diagnostic** to `WARNING` or `ERROR` through the canonical configurable diagnostic-severity mechanism. This is a general rule for INFO diagnostics, not a unique exception for reborrowing. An INFO promoted to ERROR participates in the normal selected diagnostic policy; the implementation must preserve the original diagnostic identity and explanation.

**§ 5(4).** Do **not** invent a configuration key or syntax here. Apply the existing diagnostics/CLI/LSP configuration contract, or record its unresolved exact spelling as a separate specification item if absent. User configurability does **not** permit downgrading mandatory language-safety errors into mere INFO messages or disabling borrow correctness.

**§ 5(5).** When reborrowing is **illegal**, emit the ordinary hard error with the failed authority rule, overlapping Place(s), borrow origin, and the use/dependency that keeps the conflicting borrow live. Include a useful source-level remedy when it is sound. Compiler CLI, `sec diagnostics <ID>`, JSON diagnostics and LSP should preserve consistent identities and distinctions between informational explanation and required safety enforcement.

## § 6. MLIR and LLVM: how optimized code remains borrowing-safe

**§ 6(1).** Source borrow legality and semantic last-use boundaries are established at Sema/Semantic IR level **before** machine-oriented optimization. An optimizer must not redefine what source-level `ref r`, `ref mut r` or the final required use means. Compile-time legal programs must not become invalid because a later pass extends an internal holder's lifetime; illegal source programs must not become legal because dead-code elimination hides a violation.

**§ 6(2).** Semantic IR must represent or retain equivalent canonical facts for borrow/reborrow kind, referent Place(s), provenance, holder identity, parent/child authority dependency, begin/end or last-required-use boundaries, generation/epoch dependence, storage invalidation and retained/escaping uses. Existing Semantic IR operations or metadata (including `ReborrowShared`, `ReborrowMutable`, and `EndBorrowDependency`) shall be used; this correction does not define a new dialect operation merely to duplicate them.

**§ 6(3).** Sec MLIR lowering must either:

1. preserve each safety-relevant dependency in operations, effects, ordered tokens, verifier-visible metadata or equivalent constraints understood by subsequent transformations; **or**
2. remove it only after proving that every relevant downstream access, alias/lifetime assumption, runtime validity check, invalidation and cleanup behavior remains correct without it.

A pass which cannot preserve or prove away a dependency must **fail closed** or decline that transformation. Merely attaching metadata that an unmodified downstream optimizer is free to ignore does not satisfy this contract.

**§ 6(4).** MLIR/LLVM optimization may reorder, merge or eliminate physical instructions when semantic equivalence is established. It must **not** move an actual referent access across a destructive or invalidating event without a sufficient proof, or move a generation/epoch validation away from a dependent access in a way that permits a stale access. No stronger `noalias`, dereferenceability, lifetime or non-null assumption may be emitted than the Sec proof supports.

**§ 6(5).** An `EndBorrowDependency`/last-use fact is not itself necessarily a runtime instruction. Removing compile-time borrow bookkeeping is legal when proof obligations are discharged. LLVM lifetime intrinsics, debugger locations and source lexical blocks are **not** substitutes for Sec borrow legality, and their presence or physical placement cannot be the sole basis of correctness.

**§ 6(6).** Address stability and pinning, moves versus physical relocation, `free()`/destructor/defer ordering, Arena reset/release, pool-slot reuse, escaped reference retention, volatile/MMIO accesses, FFI callbacks, atomics and synchronization preserve their own existing rules across all lowering boundaries. An optimization is illegal if it changes their observable behavior or relaxes a required safety guarantee.

**§ 6(7).** Semantic IR verification must reject contradictory borrow/reborrow plans and missing/invalid required dependencies. Verify at the already defined IR and lowering boundaries; do not require a full second source-level borrow checker in MLIR. For any lowering or optimization involving reference validity, establish the proof required by the target pipeline and rerun the applicable verifier after transformation. Where a proof cannot be expressed at the next level, the pass must preserve the necessary operation/check or refuse the lowering.

**§ 6(8).** This section is a strengthened cross-stage correctness obligation, not a demand that LLVM understand Sec source-level borrowing syntax. The compiler must *concretely* realize each retained dependency using the semantics available to its downstream optimization passes, and test that its chosen representation does not permit miscompilation.

## § 7. Required conformance and compiler regression tests

**§ 7(1). MD-015 contextual typing.** Verify all of the following:

```sec
fn Wait(d: duration) void {
}

Wait(5<s>)                          // Valid.
Wait(500<ms>)                       // Valid.

let typed: int<s> := 5
Wait(typed)                         // ERROR: typed quantity is not duration.

let inferred := 5<s>
Wait(inferred)                      // ERROR: inferred typed quantity is not duration.

let converted := duration(5<s>)
Wait(converted)                     // Valid.
```

Also cover assignments and initializers, signed/negative durations, decimal default carrier, explicit typed-value conversions, exactness/rounding/overflow, and rejection of implicit typed conversion **even when** optimization knows the stored numeric value.

**§ 7(2). MD-061 type and place identity.** Test the four reborrow cases in § 3(1), including that `ref r` never receives type `ref ref T`, and that the type-checker plan, Semantic IR and emitted Sec MLIR all agree on the same referent and provenance. Reject shared-to-mutable elevation.

**§ 7(3). Borrow live range.** Test last-use before lexical scope end, delayed last-use through `defer`, captures/returned references, calls with retained arguments, branches with different final uses, loop backedges, and static disjoint fields. Verify parent `ref mut` recovers usable authority only when all conflicting child dependencies have ended.

**§ 7(4). INFO and escalation.** Test default INFO for valid `ref r`, absence of an unjustified 'replace with `ref value`' suggestion, user promotion to WARNING and ERROR, retained diagnostic identity across CLI/JSON/LSP, and the inability to demote or bypass a real unsafe-borrow error. No arbitrary configuration syntax may be assumed by the tests until canonicalized.

**§ 7(5). Optimization matrix.** Run representative legal/illegal borrow cases before and after the available Sec Semantic IR/Sec MLIR/LLVM optimization levels (including unoptimized and optimized builds). Inspect transformed IR where a bug cannot be detected from runtime output alone. Compare behavior and enforce verifier invariants across:

1. Last-use followed by mutation through the parent reference.
2. Reborrow of shared and mutable holders, with overlapping and disjoint Places.
3. Return/capture/defer escape dependencies, including paths with no further lexical use.
4. Generation/epoch checks immediately protecting dependent dereferences.
5. Array/list relocation and structural invalidation.
6. Cleanup, `free()`, owner destruction, Arena reset/release and pool-slot reuse.
7. MMIO/volatile, atomics/synchronization and opaque FFI retention boundaries.
8. Adversarial or mutated Semantic IR/MLIR that omits a required borrow dependency or introduces a false `noalias`/lifetime assumption.

**§ 7(6).** Verify independently checked cold compilation and incremental compilation agree on accepted/rejected source, resolved reborrow provenance, emitted diagnostics and optimized runtime behavior. Cache reuse must not manufacture borrow proofs or lose required invalidation/generation facts.

## § 8. Integration and closure instructions

**§ 8(1).** Amend the earlier MD-015 correction (and any integrated temporal/unit text) so that its typed-domain-proven implicit-conversion permission in § 6.3 is removed. Keep the accepted exact **literal** conversion behavior, explicit checked conversions, and all other MD-015 guarantees. Update the conflicting test expectation in § 10.2 and any related governance claims.

**§ 8(2).** Update `borrowing.md` §§ 5, 8–11 and 30–32, the canonical reference/lifetime rules, the applicable grammar/typing rules and Semantic IR's reborrow plan to spell out `ref r` and `ref mut r` precisely. Preserve the original paragraphs and cross-references. Correct the known source type-checker versus Semantic IR plan inconsistency. Lower explicit reborrows through the established Sec MLIR representation when implemented; do not fabricate a new operator or backend capability.

**§ 8(3).** Document INFO by default for legal reborrows and the universal user-controlled `INFO → WARNING/ERROR` escalation policy in the diagnostic owner. Preserve mandatory safety errors. Use real diagnostic identity and configuration fields already sanctioned by the repository; if these are not yet specified, register that narrow implementation/specification follow-up rather than inventing them.

**§ 8(4).** Apply the optimizer safety obligations at the Semantic IR → Sec MLIR → MLIR/LLVM boundaries and keep verifiable evidence from regression, negative-IR and optimization-differential tests. No source-validity guarantee may depend on a particular optimizer level.

**§ 8(5).** Update **existing** governance fragments for the above behaviors and the latest `missing-decisions.yaml` only after the owning rulebooks and implementation evidence are synchronized. Until then, MD-015 and MD-061 are **decided**, not claimed integrated. Additional specific implementation gaps may be tracked separately; do not reopen the language decisions without an actual conflict.
