# Generic Lowering

- **Status:** Canonical normative rulebook
- **Created:** 2026-09-04
- **Last updated:** 2026-09-04
- **Document revision:** 1.1
- **Language version:** Sec 0.1
- **Canonical path:** `rules/compiler/generics_lowering.md`

## 1. Purpose

This rulebook defines the verified compiler boundary from canonical concrete
generic specializations into representation-dependent lowering.

Source generic declaration, constraint, and inference semantics are owned by
`rules/declarations/generics.md`.

Canonical specialization identity, demand, the instantiation dependency graph,
application of canonical substitutions, specialization termination, and
semantic-versus-physical realization are owned by
`rules/compiler/monomorphization.md`.

This rulebook consumes those canonical specialization facts and defines the
requirements that must hold before and during concrete
representation-dependent lowering.

## 2. Compile-time specialization

Sec generics are compile-time templates. Lowering consumes the canonical
demanded concrete specializations supplied by
`rules/compiler/monomorphization.md`; it does not create a competing demand or
specialization identity model.

Lowering must not introduce:

- runtime type descriptors;
- generic dictionaries;
- type erasure;
- runtime generic dispatch;
- implicit boxing;
- runtime values for generic type parameters.

## 3. Preserved concrete identity

Concrete generic nominal identity is defined by
`rules/compiler/monomorphization.md` and the owning type rulebooks. Lowering
preserves, where applicable:

```text
InstantiationIdentity
OrderedConcreteGenericArguments
CanonicalConcreteTypeIdentity
```

Lowering must not define a second identity formula or merge semantically
distinct specializations merely because representations match. Concrete
callable and storage binary identities remain governed by linking, ABI, static,
and monomorphization rules as applicable.

## 4. Concrete closure boundary

Before representation-dependent Semantic IR, Sec MLIR, ABI, FFI, or backend
lowering consumes a runtime-relevant concrete specialization, the compiler must
verify that the specialization is canonical and that every generic-dependent
semantic choice required at that boundary is resolved.

The canonical specialization, including its substitution and
`InstantiationIdentity`, is supplied by
`rules/compiler/monomorphization.md`.

The closure verifier rejects unresolved generic parameters or unresolved
generic-dependent semantic choices in runtime-relevant concrete IR.

The closure verifier does not require physical size, alignment, field offsets,
ABI classification, target-sized representation, or machine symbols before the
owning plan-realization stage requires them.

## 5. Generic nominal types

Generic structs, unions, enums, named types, and interfaces preserve their
source declaration identity together with their ordered concrete arguments.
Semantic generic-dependent properties are resolved from their owning rulebooks
after substitution. Plan-specific physical layout and ABI facts may remain
unresolved until their owning lowering stages require them. Nominal identity
and ordered concrete arguments remain preserved throughout lowering.

## 6. Generic enums

Given:

```sec
enum State[T] {
    Ready
    Busy
}
```

and the substitution `T -> Connection`, generic lowering produces the concrete
nominal enum `State[Connection]` with the ordinary `Ready` and `Busy` member
set.

Concrete generic enum identity includes:

```text
GenericEnumDeclarationIdentity
OrderedConcreteGenericArguments
CanonicalConcreteEnumIdentity
```

A member identity is the source member declaration projected through its
concrete enum owner identity. Thus equally named members of `State[Connection]`
and `State[Socket]` remain members of different Sec types.

The generic enum must be fully concretized before representation-dependent enum
lowering. The resulting concrete type then follows
`rules/declarations/enums.md` for representation, constants, conversions,
comparison, `switch`, `match`, and exhaustiveness.

A generic parameter that is phantom with respect to runtime storage still
contributes to concrete nominal identity. It adds no runtime field or hidden
metadata.

## 7. Semantic IR requirements

Semantic IR must preserve enough information to identify:

- the source generic declaration;
- ordered generic parameters and constraints;
- ordered concrete generic arguments;
- the canonical concrete identity;
- substituted concrete member and payload types;
- source and instantiation provenance.

Representation-independent template IR, when present, must remain distinct from
executable concrete IR.

## 8. Failure and diagnostics

Failures retain concrete specialization provenance but remain classified by
their owning phase:

- invalid substitution or unresolved generic semantics: generic/Sema failure;
- proven non-converging specialization: monomorphization failure;
- recursive by-value semantic layout: layout failure;
- unsupported or invalid plan-specific representation or ABI: lowering/ABI/plan failure;
- symbol collision: linking/binary-identity failure;
- invalid concrete enum representation: enum/layout failure.

Lowering must not collapse these into one generic-specialization diagnostic.

## 9. Determinism

Canonical identities and emitted symbols must not depend on discovery order,
map iteration, filesystem enumeration, or worker scheduling. Repeated requests
for the same concrete specialization must reuse the same semantic identity.

## 10. Cross-rulebook ownership

- source generics, constraints, and inference: `rules/declarations/generics.md`;
- canonical specialization identity, demand, substitution application,
  specialization termination, implementation sharing, and generic cross-module
  behavior: `rules/compiler/monomorphization.md`;
- enum domains, members, and representation: `rules/declarations/enums.md`;
- Semantic IR ownership and identity: `rules/compiler/semantic_ir.md`;
- compilation phases and plan boundaries:
  `rules/compiler/compiler_pipeline.md`;
- concrete ABI and foreign boundaries: platform ABI and FFI rulebooks;
- dialect-specific operations and conversions: `rules/mlir/`.

Implementation progress belongs in `implementation-status.yaml`.
