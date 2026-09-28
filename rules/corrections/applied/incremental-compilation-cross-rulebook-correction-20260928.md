# Incremental Compilation Cross-Rulebook Correction

- **Status:** Applied correction
- **Created:** 2026-09-27
- **Last updated:** 2026-09-28
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/corrections/applied/incremental-compilation-cross-rulebook-correction-20260928.md`
- **Applied:** 2026-09-28
- **Triggered by:** `rules/compiler/incremental_compilation.md` revision 1.0
- **Repository baseline reviewed:** `main-reviewed-2026-09-27`

---
## § 1. Purpose

§ 1(1) This correction harmonizes existing rulebooks, status metadata, and governance handoffs with the canonical incremental-compilation rulebook at:

```text
rules/compiler/incremental_compilation.md
```

§ 1(2) This correction changed path, ownership, and future-placeholder wording only except where an existing handoff now identifies the newly defined incremental owner.

§ 1(3) It does not renumber unrelated normative paragraphs and does not transfer semantic ownership from modules, compiler analysis, monomorphization, Semantic IR, ABI, debug information, linking, LSP, or compiler testing.

---
## § 2. `rules/compiler/compiler_testing.md`

§ 2(1) In § 87(2), replace the obsolete path:

```text
rules/incremental_compilation.md
```

with:

```text
rules/compiler/incremental_compilation.md
```

§ 2(2) § 87 continues to mean:

- `compiler_testing.md` owns generic cold-versus-incremental differential and test infrastructure;
- `incremental_compilation.md` owns dependency tracking, invalidation, cache identity/validity, legal reuse, stale-state prevention, plan isolation, and feature-specific incremental evidence obligations.

§ 2(3) In § 87(4), wording that treats the incremental contract as not yet existing should be removed or rephrased so the current canonical rulebook is consumed.

---
## § 3. `rules/compiler/debug_information.md`

§ 3(1) In § 87(4), replace:

```text
rules/incremental_compilation.md
```

with:

```text
rules/compiler/incremental_compilation.md
```

§ 3(2) The ownership boundary remains unchanged: debug information owns debug semantics and artifact/source correlation; incremental compilation owns cache invalidation and reuse of semantic, debug, backend, and link artifact state.

---
## § 4. `rules/compiler/linking.md`

§ 4(1) In the link-cache section, replace the unqualified future owner reference:

```text
incremental_compilation.md
```

with:

```text
rules/compiler/incremental_compilation.md
```

§ 4(2) `rules/compiler/linking.md` remains authoritative for `LinkPlan`, `LinkEnvironment`, link roots, link cache input identity, LTO legality, binary surfaces, toolchain interaction, and final-artifact verification.

§ 4(3) `rules/compiler/incremental_compilation.md` owns whether a previously derived link-related result remains reusable under the current link contract.

---
## § 5. `rules/compiler/compiler_pipeline.md`

§ 5(1) § 90(3) currently refers generically to the future/current incremental-compilation rulebook.

§ 5(2) Replace that placeholder wording with an explicit reference to:

```text
rules/compiler/incremental_compilation.md
```

§ 5(3) `compiler_pipeline.md` continues to own the canonical pipeline and stage boundaries. The incremental book owns detailed dependency, validity, invalidation, publication, and reuse rules at those boundaries.

---
## § 6. `rules/projects/modules.md`

§ 6(1) References near the related-rulebook section that describe incremental-compilation rules as future consumers should be updated to identify the current canonical consumer:

```text
rules/compiler/incremental_compilation.md
```

§ 6(2) No `ModuleIdentity`, `ModuleInstance`, `ModuleSurface`, visibility, import, or module-surface semantics move to the incremental rulebook.

§ 6(3) Incremental compilation consumes those identities and surfaces for dependency-driven reuse and invalidation.

---
## § 7. `language-rulebook-status.md`

§ 7(1) Replace the planned entry:

```text
incremental_compilation.md | Planned
```

with the canonical entry:

```text
compiler/incremental_compilation.md | Written
```

§ 7(2) The description should identify the written book as the canonical model for logical snapshots, dependency/fingerprint validity, invalidation, persistent/shared cache correctness, module/source/provenance reuse, plan-specific artifact reuse, history independence, and incremental conformance.

§ 7(3) Remove `incremental_compilation.md` from the canonical planned-rulebook set and add `compiler/incremental_compilation.md` to the canonical written-rulebook set.

§ 7(4) Documentation status remains distinct from implementation status. The new compiler governance integration initially remains `planned` until an incremental vertical slice is verified.

---
## § 8. `governance/compiler.yaml`

§ 8(1) Add one integration:

```text
compiler.incremental-compilation-v1
```

to the existing `governance/compiler.yaml` fragment.

§ 8(2) Do not create a new governance fragment solely for incremental compilation.

§ 8(3) In `compiler.debug-information-v1` governance reconciliation, replace the future wording that says `incremental_compilation.md` will own cache invalidation/reuse when written with a current reference to:

```text
rules/compiler/incremental_compilation.md
compiler.incremental-compilation-v1
```

§ 8(4) The debug integration retains its feature-specific debug conformance obligations.

---
## § 9. `governance/testing.yaml`

§ 9(1) Update `testing.compiler-verification-v1.incremental-differential-hook` so that it no longer describes incremental semantics as owned by a future rulebook.

§ 9(2) Its resulting ownership statement should be equivalent to:

```text
Generic cold-versus-incremental differential, matrix, fuzzing,
mutation, fault-injection, race, and regression infrastructure is
owned by testing.compiler-verification-v1.

Concrete incremental dependency, invalidation, cache-validity,
publication, history-independence, and feature-specific evidence
obligations are owned by rules/compiler/incremental_compilation.md
and compiler.incremental-compilation-v1.
```

§ 9(3) Do not duplicate the incremental implementation backlog in `governance/testing.yaml`.

---
## § 10. `governance/index.yaml`

§ 10(1) No new fragment entry is required.

§ 10(2) The existing `compiler.yaml` ownership of compiler pipeline, linking, initialization, and artifacts is sufficient to host `compiler.incremental-compilation-v1`.

§ 10(3) An ownership-description wording expansion may be made later for clarity, but it is not required for the semantic integration and should not create a duplicate owner.

---
## § 11. Existing generic fingerprint rules

§ 11(1) `rules/compiler/monomorphization.md` already distinguishes canonical generic identity from cache validity and already defines generic-specific cache/fingerprint rules.

§ 11(2) Those rules remain normative and are consumed by the new incremental rulebook.

§ 11(3) Do not rewrite an `InstantiationKey` as an incremental cache identity or make a cache fingerprint part of language-level generic identity.

---
## § 12. Application checklist

Apply this correction by verifying all of the following:

- [x] `rules/compiler/incremental_compilation.md` exists at the canonical path.
- [x] `compiler_testing.md` points to the canonical path and no longer treats the contract as absent.
- [x] `debug_information.md` points to the canonical path.
- [x] `linking.md` identifies the canonical incremental owner for detailed cache architecture/reuse.
- [x] `compiler_pipeline.md` points to the canonical path.
- [x] `projects/modules.md` identifies the canonical incremental consumer without transferring module semantics.
- [x] `language-rulebook-status.md` marks `compiler/incremental_compilation.md` Written and removes the planned root-level item.
- [x] `governance/compiler.yaml` contains `compiler.incremental-compilation-v1` exactly once.
- [x] `compiler.debug-information-v1` handoff refers to the current incremental owner.
- [x] `governance/testing.yaml` incremental differential hook refers to the current incremental owner.
- [x] no duplicate incremental governance fragment is added to `governance/index.yaml`.
- [x] no unrelated stable normative paragraph numbers are renumbered merely to apply these cross-references.

---
## § 13. Non-goals

This correction does not:

- choose a query-engine implementation;
- define a cache serialization format;
- declare persistent/shared caching implemented;
- change `ModuleSurface` semantics;
- change generic specialization identity;
- change Semantic IR meaning;
- change ABI/layout rules;
- change debug-information semantics;
- change `LinkPlan` or linking semantics;
- transfer generic compiler-test infrastructure out of `testing.compiler-verification-v1`.
