# Applied Correction — Monomorphization Cross-Rulebook Harmonization

- Status: Applied normative correction package
- Created: 2026-09-04
- Last updated: 2026-09-04
- Document revision: 1.0
- Language version: Sec 0.1
- Applies with: `rules/compiler/monomorphization.md` revision 1.0

Application note: the delivered rulebook's own canonical-path metadata places
it at `rules/compiler/monomorphization.md`. The synchronization therefore uses
that canonical path; unqualified `rules/monomorphization.md` paths retained
below record the wording of the received correction package.

## 1. Purpose

This correction package removes overlapping ownership and conflicting wording from existing generic rulebooks when `rules/monomorphization.md` becomes canonical.

After the edits are merged, this correction should be moved to `rules/corrections/applied/` according to the repository correction workflow.

## 2. `rules/declarations/generics.md`

### 2.1 Purpose and monomorphization references

Keep the statement that Sec generics are compile-time and monomorphized, but make `rules/monomorphization.md` the canonical owner for concrete specialization identity, demand, specialization graph, physical implementation sharing, termination, and cross-module specialization behavior.

The introductory description must not imply that every semantic specialization necessarily produces distinct backend code.

### 2.2 Replace section 24 with the following normative distinction

```markdown
## 24. Recursive generic structures

Generic declarations may participate in recursive type and callable structures.

Recursive by-value representation is governed by `rules/memory/layout.md` and the owning declaration rulebooks. A finite generic instantiation set may still have invalid recursive by-value layout.

Concrete specialization demand is governed by `rules/monomorphization.md` and must converge to a finite set of canonical instantiations.

Revisiting an existing canonical instantiation is not by itself an error. Ordinary runtime recursion and finite mutually recursive specialization remain governed by the call graph and owning semantic rules.

A generic dependency that is proven to generate an unbounded sequence of distinct canonical instantiations is rejected according to `rules/monomorphization.md`.
```

### 2.3 Replace section 25 with the following

```markdown
## 25. Concrete specialization

Generics are specialized according to `rules/monomorphization.md` before runtime-relevant generic semantics cross into representation-dependent lowering.

A concrete instantiation includes the complete canonical substitution of all generic parameters required by that specialization.

For every reachable concrete generic type or callable, the compiler must resolve every generic-dependent semantic fact required by the owning language rules, including the complete concrete type surface, applicable ownership/copy/move/destruction semantics, concrete callable signatures, concrete member and associated identities, and per-specialization static-storage identity.

Physical size, alignment, field offsets, ABI classification, target-sized representation, and other `CompilationPlan`-dependent representation facts are resolved by their owning layout, ABI, and lowering stages when required. They are not prerequisites for target-independent generic identity merely because the generic specialization is concrete.

Runtime-relevant concrete IR must not depend on unresolved generic parameters. The verified lowering boundary is defined by `rules/compiler/generics_lowering.md`.
```

### 2.4 Section 26 demand wording

Keep demand-driven specialization as the canonical rule.

Clarify that eager compiler precomputation or caching may not create additional active semantic instantiations, LinkRoots, or mandatory emitted backend code merely for implementation convenience.

### 2.5 Section 33 Sema requirements

Retain parser/Sema requirements for generic declaration registration, constraints, inference, scopes, and template validity.

Replace the monomorphization-owned requirements with references to `rules/monomorphization.md`.

In particular:

- replace `canonicalize identical concrete instantiations` with a requirement to use canonical `InstantiationIdentity` and demand rules from `rules/monomorphization.md`;
- replace `detect infinite recursive instantiation` with a requirement to apply the termination/non-convergence model from `rules/monomorphization.md`;
- replace `re-evaluate concrete layout/default/copy/move/destruction properties` with language that distinguishes semantic type properties from plan-specific physical layout;
- retain the requirement that generic bodies use only capabilities guaranteed by their declared constraints;
- retain rejection of unresolved runtime-relevant generic semantics before concrete lowering.

### 2.6 Section 35 diagnostics

Remove:

```text
recursive generic instantiation does not produce a finite type
```

as one combined diagnostic category.

Use distinct owning diagnostics for:

```text
generic instantiation does not converge
recursive by-value layout has no finite size
generic instantiation resource limit exceeded
```

The first and third are governed by `rules/monomorphization.md`; recursive layout is governed by layout and owning declaration rules.

### 2.7 Replace section 37 ownership opening

Use:

```markdown
## 37. Cross-rulebook ownership

This rulebook owns Sec type-generic declarations, generic parameters and arguments, constraints, constraint satisfaction, inference, generic parameter scope, generic declaration families, and template-level semantic validity.

Canonical concrete specialization, InstantiationIdentity, demand-driven monomorphization, the specialization dependency graph, semantic-versus-physical generic realization, implementation-sharing policy, monomorphization termination, and generic-specific cross-module specialization behavior are owned by `rules/monomorphization.md`.
```

Keep the existing related-rule ownership list, with `rules/compiler/generics_lowering.md` identified as the concrete generic closure/lowering boundary rather than an owner of generic specialization semantics.

## 3. `rules/compiler/generics_lowering.md`

### 3.1 Replace section 1 purpose

```markdown
## 1. Purpose

This rulebook defines the verified compiler boundary from canonical concrete generic specializations into representation-dependent lowering.

Source generic declaration, constraint, and inference semantics are owned by `rules/declarations/generics.md`.

Canonical specialization identity, demand, the instantiation dependency graph, application of canonical substitutions, specialization termination, and semantic-versus-physical realization are owned by `rules/monomorphization.md`.

This rulebook consumes those canonical specialization facts and defines the requirements that must hold before and during concrete representation-dependent lowering.
```

### 3.2 Section 2

Keep the prohibition on runtime type descriptors, generic dictionaries, type erasure, runtime generic dispatch, implicit boxing, and runtime values for generic type parameters.

Change wording that implies this rulebook itself creates or owns every specialization. It consumes canonical demanded specializations from `rules/monomorphization.md`.

### 3.3 Replace section 3 identity ownership

The section should state that concrete generic nominal identity is defined by `rules/monomorphization.md` and the owning type rulebooks.

Generic lowering must preserve:

```text
InstantiationIdentity
OrderedConcreteGenericArguments
CanonicalConcreteTypeIdentity
```

where applicable, but must not define a second competing identity formula.

Concrete callable and storage binary identities remain governed by linking, ABI, static, and monomorphization rules as applicable.

### 3.4 Rename section 4 to `Concrete closure boundary`

Replace its phase list with the following conceptual contract:

```markdown
## 4. Concrete closure boundary

Before representation-dependent Semantic IR, Sec MLIR, ABI, FFI, or backend lowering consumes a runtime-relevant concrete specialization, the compiler must verify that the specialization is canonical and that all generic-dependent semantic choices required at that boundary are resolved.

The canonical specialization itself, including its substitution and InstantiationIdentity, is supplied by `rules/monomorphization.md`.

The closure verifier rejects unresolved generic parameters or unresolved generic-dependent semantic choices in runtime-relevant concrete IR.

The closure verifier does not require physical size, alignment, field offsets, ABI classification, target-sized representation, or machine symbols before the owning plan-realization stage requires them.
```

### 3.5 Section 5 generic nominal types

Replace wording that says layout, copy/move, destruction, defaults, and ABI facts are all simply "recomputed after substitution" with a layered statement:

- semantic generic-dependent type properties are resolved from their owning rulebooks after substitution;
- plan-specific physical layout and ABI facts may remain unresolved until their owning lowering stages;
- nominal identity and ordered concrete generic arguments must be preserved throughout lowering.

### 3.6 Section 8 failure classification

Do not classify all `invalid concrete layout or representation` as one generic specialization failure.

Distinguish:

- invalid substitution or unresolved generic semantics: generic/Sema failure;
- proven non-converging specialization: monomorphization failure;
- recursive by-value semantic layout: layout failure;
- unsupported or invalid plan-specific representation/ABI: lowering/ABI/plan failure;
- symbol collision: linking/binary-identity failure;
- concrete enum representation invalidity: enum/layout failure.

All failures must retain concrete specialization provenance where the specialization caused or exposed the failing condition.

### 3.7 Replace section 10 ownership list

Add:

- canonical specialization identity, demand, substitution application, specialization termination, implementation sharing, and generic cross-module behavior: `rules/monomorphization.md`;

Change the existing `source generics, constraints, inference, and substitution` item so that `rules/declarations/generics.md` owns source generics, constraints, and inference, while canonical substitution application belongs to `rules/monomorphization.md`.

## 4. `language-rulebook-status.md`

Apply all of the following:

1. Change `monomorphization.md` from `Planned` to `Written`.
2. Remove `monomorphization.md` from the planned canonical rulebook set and add it to the written set.
3. Change the `declarations/generics.md` note so it no longer says that the book owns concrete monomorphization.
4. Change the `compiler/generics_lowering.md` note so it describes the verified concrete closure/lowering boundary rather than canonical specialization ownership.
5. Remove the following items from `Compile-time evaluation and generics / Still to decide`:
   - generic specialization;
   - monomorphization guarantees;
   - cross-module generic ABI.

Keep the genuinely unresolved compile-time-evaluation items in that section.

## 5. `implementation-status.yaml`

1. Add `rules/monomorphization.md` to the `frontend.generics-v2` rule list because that existing integration remains the canonical frontend generics implementation slice.
2. Do not duplicate the parser, inference, generic enum, generic interface, and existing substitution implementation claims in a second frontend entry.
3. Add the `compiler.monomorphization` integration described by the companion governance YAML for the cross-phase specialization/realization work not adequately represented by `frontend.generics-v2`.
4. Reconcile any implementation-status wording that treats current recursive storage diagnostics as a complete implementation of semantic monomorphization termination.
5. Keep `frontend.generic-union-recursive-storage` as the implementation ledger for its concrete represented storage subset; do not use it as the canonical owner of the general InstantiationGraph termination model.

## 6. Revision metadata

After applying these changes:

- update `rules/declarations/generics.md` from revision 2.1 to revision 2.2 and set `Last updated` to 2026-09-04;
- update `rules/compiler/generics_lowering.md` from revision 1.0 to revision 1.1 and set `Last updated` to 2026-09-04;
- keep `rules/monomorphization.md` at revision 1.0;
- update the root implementation ledger date if the governance patch is merged.
