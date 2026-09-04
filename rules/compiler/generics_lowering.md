# Generic Lowering

- **Status:** Canonical normative rulebook
- **Created:** 2026-09-04
- **Last updated:** 2026-09-04
- **Document revision:** 1.0
- **Language version:** Sec 0.1
- **Canonical path:** `rules/compiler/generics_lowering.md`

## 1. Purpose

This rulebook defines the compiler boundary between generic templates and
concrete representation-dependent lowering. It complements
`rules/declarations/generics.md`; source declaration, constraint, inference,
and substitution semantics remain owned there.

## 2. Compile-time specialization

Sec generics are compile-time templates. Every reachable complete generic
argument list produces or reuses a canonical concrete specialization.

Lowering must not introduce:

- runtime type descriptors;
- generic dictionaries;
- type erasure;
- runtime generic dispatch;
- implicit boxing;
- runtime values for generic type parameters.

## 3. Concrete identity

A concrete generic nominal identity includes at least:

```text
GenericDeclarationIdentity
OrderedConcreteGenericArguments
CanonicalConcreteTypeIdentity
```

The same declaration and argument list canonicalize to the same Sec type.
Different argument lists identify different Sec types even when their physical
representations are identical.

Concrete callable and static-storage symbols must include sufficient canonical
generic identity to avoid collisions between specializations.

## 4. Substitution boundary

Before representation-dependent Semantic IR, Sec MLIR, ABI, FFI, or backend
lowering, the compiler must:

1. resolve the source generic declaration;
2. validate arity and constraints;
3. form the ordered substitution;
4. validate the concrete substituted declaration;
5. canonicalize or reuse its concrete identity;
6. reject unresolved or non-converging specialization;
7. lower the resulting concrete type or callable through its ordinary rules.

No unresolved generic parameter may reach executable representation-dependent
IR.

## 5. Generic nominal types

Generic structs, unions, enums, named types, and interfaces preserve their
source declaration identity together with their ordered concrete arguments.
Their concrete layout, copy/move behavior, destruction, defaults, and ABI facts
are recomputed after substitution where those facts depend on an argument.

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

Specialization must fail before backend lowering for:

- wrong generic arity;
- unknown or unsatisfied constraints;
- unresolved generic parameters;
- non-converging recursive instantiation;
- invalid concrete layout or representation;
- symbol identity collisions;
- concrete enum representations rejected by the ordinary enum rules.

Concrete-dependent diagnostics should identify both the generic declaration and
the concrete substitution that caused the failure.

## 9. Determinism

Canonical identities and emitted symbols must not depend on discovery order,
map iteration, filesystem enumeration, or worker scheduling. Repeated requests
for the same concrete specialization must reuse the same semantic identity.

## 10. Cross-rulebook ownership

- source generics, constraints, inference, and substitution:
  `rules/declarations/generics.md`;
- enum domains, members, and representation: `rules/declarations/enums.md`;
- Semantic IR ownership and identity: `rules/compiler/semantic_ir.md`;
- compilation phases and plan boundaries:
  `rules/compiler/compiler_pipeline.md`;
- concrete ABI and foreign boundaries: platform ABI and FFI rulebooks;
- dialect-specific operations and conversions: `rules/mlir/`.

Implementation progress belongs in `implementation-status.yaml`.
