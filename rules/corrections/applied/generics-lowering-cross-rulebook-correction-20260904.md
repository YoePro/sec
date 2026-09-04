# Applied Correction — Generic Lowering Cross-Rulebook Correction

- Status: Applied normative correction and synchronization package
- Created: 2026-09-04
- Last updated: 2026-09-04
- Sec language version: 0.1
- Repository baseline reviewed: `69d8b57`
- Primary new rulebook: `rules/compiler/generics_lowering.md`
- Intended correction area: `rules/corrections/`
- Related applied correction: `rules/corrections/applied/generic-enums-cross-rulebook-correction-20260904.md`
- Classification: Normative synchronization required by the canonical generic-lowering rulebook.

---

## § 1 Purpose

§ 1(1) This correction records cross-rulebook synchronization required when `rules/compiler/generics_lowering.md` becomes canonical.

§ 1(2) It does not replace the separate generic-enum correction; both corrections must be applied.

§ 1(3) After synchronization, the owning rulebooks become durable authority.

---

## § 2 `rules/declarations/generics.md`

§ 2(1) Keep `generics.md` authoritative for source syntax, declaration semantics, inference, constraints, and the fact that Sec generics are compile-time and monomorphized.

§ 2(2) Move or cross-reference detailed lowering requirements to `rules/compiler/generics_lowering.md`.

§ 2(3) Preserve the existing invariant that unresolved generic parameters must not reach representation-dependent MLIR, ABI, or LLVM lowering.

§ 2(4) Apply `generic-enums-cross-rulebook-correction.md`: remove the blanket generic-enum prohibition, remove `generic enums` from the unsupported list, remove the required rejection diagnostic, and add generic enum support as canonical Sec 0.1 semantics.

§ 2(5) Keep payload-bearing alternatives a union concern; this must not be phrased as a prohibition on parameterized enum identity or behavior.

---

## § 3 `rules/compiler/semantic_ir.md`

§ 3(1) Add `generics_lowering.md` as the canonical owner of generic template-to-concrete Semantic IR lowering.

§ 3(2) Clarify that Semantic IR may retain generic templates and concrete specializations, but any specialization passed to executable Sec-MLIR emission must satisfy the complete concrete substitution required by `generics_lowering.md`.

§ 3(3) Add an invariant equivalent to:

```text
every executable generic-dependent type is concrete before Sec-MLIR emission;
every constraint-dependent executable operation has a concrete semantic target;
generic template provenance may remain without retaining generic uncertainty.
```

§ 3(4) Imported lowering-ready generic templates are valid Semantic IR/separate-compilation information when compatibility has been established.

§ 3(5) Semantic IR may represent concrete specializations as fully materialized declarations or implementation-defined specialization views, provided all canonical consumers observe verified concrete semantics.

---

## § 4 `rules/compiler/compiler_pipeline.md`

§ 4(1) Add generic concretization to lowering-readiness validation.

§ 4(2) The semantic phase ordering must include the equivalent of:

```text
generic template validation
concrete substitution
concrete-dependent semantic revalidation
constraint discharge
ownership/destruction concretization
concrete Semantic IR verification
Sec-MLIR emission
generic-concreteness verification
representation-dependent lowering
```

§ 4(3) The physical compiler implementation may interleave these steps when dependencies are satisfied; the semantic boundary remains normative.

§ 4(4) Preserve the pipeline distinction between:

```text
invalid Sec program
valid Sec program unsupported by current compiler implementation
internal compiler error
```

and apply it explicitly to generic lowering.

§ 4(5) Do not allow backend success to repair unresolved generic semantics.

---

## § 5 `rules/projects/modules.md`

§ 5(1) Extend `ModuleSurface`/separate-compilation semantics to permit compiler-facing generic lowering summaries.

§ 5(2) A public generic declaration must expose sufficient canonical semantic template information to validate and lower permitted concrete instantiations without source-level reparsing or source-level name resolution of the origin module.

§ 5(3) Compiler-facing generic lowering metadata may include non-public transitive dependencies required by exported generic templates.

§ 5(4) Such dependencies do not become source-visible declarations, imports, or re-exports.

§ 5(5) Generic lowering surfaces require deterministic semantic fingerprints.

§ 5(6) A semantic change in an exported generic body or a transitively required private helper may invalidate dependent generic lowering summaries even when the public source signature is unchanged.

§ 5(7) Unrelated private implementation changes need not invalidate generic lowering summaries when dependency equivalence is proven.

§ 5(8) Stale or incompatible generic surfaces must fail closed or be rebuilt.

---

## § 6 `rules/mlir/sec_mlir_dialect.md`

§ 6(1) Generalize the existing concrete-generic struct/union identity model into a cross-dialect invariant: executable compiler-generated Sec-MLIR generic-dependent nominal types must have complete concrete generic identity.

§ 6(2) `!sec.struct` and `!sec.union` continue to preserve nominal declaration identity and concrete type arguments while remaining high-level and representation-independent.

§ 6(3) Generic enum instantiations use ordinary `!sec.enum` semantics with a canonical concrete enum type identity that distinguishes different generic argument lists.

§ 6(4) The dialect need not introduce executable generic parameter, generic instantiate, generic call, or generic constraint-call operations for Sec 0.1 ordinary generics.

§ 6(5) Generic provenance attributes may be introduced without making them runtime values.

§ 6(6) Dialect verification should support the generic-concreteness invariants defined by `generics_lowering.md`.

---

## § 7 `rules/mlir/sec_mlir_lowering.md`

§ 7(1) Generalize the current generic-union rule:

```text
Runtime union ops require concrete substituted payload types.
Do not lower unresolved generic union template values.
```

into a rule for all compiler-generated executable generic-dependent Sec-MLIR.

§ 7(2) Unresolved generic parameters, partially specialized nominal types, and unresolved constraint operations must not cross the canonical Sec-MLIR boundary.

§ 7(3) Concrete generic functions, methods, structs, named types, unions, enums, interfaces, impl members, and associated state use their ordinary concrete lowering paths after successful generic lowering.

§ 7(4) Layout lowering remains later than generic concretization.

§ 7(5) A missing ownership-aware Sec-MLIR operation for an otherwise valid concrete generic instance is an implementation gap, not a language-level generic error.

---

## § 8 `rules/platform/abi.md`

§ 8(1) Confirm that ABI classification consumes only concrete generic callable/data instances.

§ 8(2) Generic source declarations themselves do not define unresolved runtime ABIs.

§ 8(3) ABI lowering must not introduce runtime generic dictionaries or type descriptors as an implicit implementation of ordinary Sec generics.

§ 8(4) Any FFI/export restriction on a concrete generic instance remains owned by ABI/FFI rules.

---

## § 9 Ownership, copy/move, and destruction rulebooks

§ 9(1) Cross-reference `generics_lowering.md` for the rule that generic-dependent ownership and destruction facts are concretized after substitution and before ownership-sensitive executable lowering.

§ 9(2) Preserve the distinction between semantic copy/move/destruction and later physical optimization.

§ 9(3) A valid move-only specialization must never be lowered as `copy-trivial` merely because the current MLIR implementation lacks the required move-aware form.

§ 9(4) Missing lowering coverage must fail as an implementation gap.

---

## § 10 Effect analysis and volatile/hardware semantics

§ 10(1) Cross-reference the generic-lowering rule that concrete specialization may refine type-dependent effect facts but must not weaken, duplicate, eliminate, or reorder observable effects outside the owning effect/optimizer rules.

§ 10(2) Volatile, hardware, atomic, barrier, destruction, panic, and failure semantics remain ordinary concrete semantics after specialization.

---

## § 11 LSP/tooling

§ 11(1) The compiler workspace remains the semantic source of truth for generic declarations, concrete specialization identity, substitution context, and diagnostics.

§ 11(2) The LSP must not implement an independent generic-lowering engine.

§ 11(3) Tooling should preserve navigation from concrete generic uses to source generic declarations and may expose concrete specialization inspection separately.

§ 11(4) Imported generic diagnostics should preserve origin-module/source provenance where compatible artifacts provide it.

---

## § 12 `language-rulebook-status.md`

§ 12(1) Change:

```text
generics_lowering.md | Planned | Semantic and MLIR lowering of generic code.
```

to `Written`.

§ 12(2) The canonical path is:

```text
rules/compiler/generics_lowering.md
```

§ 12(3) Update the planned canonical rulebook set so `generics_lowering.md` is no longer listed as planned.

§ 12(4) Keep `monomorphization.md` and `compile_time_evaluation.md` separate planned rulebooks until their own design/writing work is complete.

§ 12(5) Remove stale "still to decide" wording that treats generic specialization/lowering as wholly undecided where the new rulebook closes the lowering boundary; retain questions specifically owned by `monomorphization.md` or `compile_time_evaluation.md`.

---

## § 13 `implementation-status.yaml`

§ 13(1) Add the per-rulebook integration item supplied by `implementation-status-generics-lowering.yaml`.

§ 13(2) Keep `frontend.generics-v2` for source/frontend generic implementation coverage.

§ 13(3) Remove `generic enums` from `frontend.generics-v2.excluded`.

§ 13(4) Add generic enums to required parser/Sema/lowering work according to the generic-enum correction.

§ 13(5) Cross-link `frontend.generics-v2` to `rules/compiler/generics_lowering.md` where its remaining work refers to Semantic IR, concrete-only MLIR, ownership/destruction concretization, or backend lowering.

§ 13(6) Do not mark generic lowering globally complete merely because some generic structs/unions already carry concrete arguments through Semantic IR or Sec-MLIR.

---

## § 14 Required test synchronization

§ 14(1) Move generic enum declarations that are invalid solely because they have generic parameters out of invalid testdata.

§ 14(2) Add valid generic enum identity and lowering tests.

§ 14(3) Add tests proving no unresolved generic parameter reaches canonical compiler-generated executable Sec-MLIR.

§ 14(4) Add cross-module tests where exported generic templates depend on private semantic helpers without exposing those helpers to source lookup.

§ 14(5) Add implementation-gap tests that distinguish valid-but-unimplemented generic lowering from invalid Sec.

---

## § 15 Superseding rule

§ 15(1) The canonical cross-rulebook invariant is:

> Generic source declarations may remain generic through frontend semantic template analysis, but a concrete specialization must be fully semantically concretized before representation-dependent executable lowering. Generic provenance may remain; unresolved generic semantics may not.

§ 15(2) Generic enums are included in that invariant as canonical Sec 0.1 generic nominal enum types.
