# Correction 8 — recursive generic unions bypass finite-representation checks

## Audit context

- Repository: `github.com/YoePro/sec`
- Repository baseline: `c515862`
- Audited: `2026-08-19`
- Primary rulebook: `rules/declarations/generics.md`
- Document revision: **2.0**
- Created: `2026-08-14`
- Last updated: `2026-08-14`

## Classification

Implementation bug.

This is not a language-rule change.

Revision 2.0 requires every recursive generic type to terminate in a finite
concrete representation. The rule is not limited to structs. Generic unions are
explicitly supported, and their concrete variant payloads must be substituted
before layout.

Current Sema has dedicated recursive-generic storage checks for generic struct
fields, but not for generic union payloads or generic union payload fields.

As a result, direct, array-mediated, and changing-argument recursive generic
unions can bypass the required finite-representation diagnostics.

---

## Normative requirements

`rules/declarations/generics.md` revision 2.0 states that:

- generic unions are supported;
- generic union variants are substituted with concrete types before layout,
  construction, matching, and destruction;
- generic recursion must terminate in a finite concrete representation;
- direct infinite storage is invalid;
- changing generic arguments must not create non-converging instantiation;
- the compiler must detect recursive-instantiation cycles;
- every reachable concrete type instance must have known concrete
  field/variant representation, size, alignment, ownership properties, and
  destruction behavior.

The Sema requirements explicitly include:

- detect infinite recursive instantiation;
- substitute concrete types recursively;
- re-evaluate concrete layout/default/copy/move/destruction properties.

---

# Bug 1 — template-level generic recursion detection is struct-only

## Affected code

`internal/sema/analyzer.go`

Generic struct fields receive this check before type resolution:

```go
if len(stmt.GenericParameters) > 0 {
    switch genericRecursiveStorageKind(
        name,
        genericParameterNameValues(stmt.GenericParameters),
        field.Type,
    ) {
    case "direct":
        ...
    case "nonconverging":
        ...
    }
}
```

`genericRecursiveStorageKind()` also follows fixed-array element types, so the
implemented struct path detects cases such as:

```sec
type Node[T] struct {
    next: Node[T],
}
```

and:

```sec
type Infinite[T] struct {
    next: Infinite[[]T],
}
```

where applicable.

`typeFromUnionDeclaration()` does not invoke this generic recursion analysis for:

- `variant.Payload`;
- entries in `variant.PayloadFields`.

Instead it resolves the payload and only tests:

```go
if sameConcreteType(typ, payload) {
    // recursive union
}
```

That ordinary concrete-type equality test is not sufficient while analyzing a
generic template.

---

# Bug 2 — a self-reference such as `Loop[T]` resolves through an `InvalidType`
# predeclaration skeleton

Before declaration bodies are analyzed, `registerTypeDeclarations()` installs:

```go
a.types[name] = Type{
    Name:              name,
    Module:            a.currentModule,
    Kind:              InvalidType,
    GenericParameters: params,
    ...
}
```

This correctly reserves the generic name and arity, but it is not yet a
completed struct/union semantic template.

While the body of a generic union is being analyzed, a payload such as:

```sec
Loop[T]
```

finds that predeclaration entry.

`resolveType()`:

1. validates the generic arity;
2. resolves `T`;
3. assigns `TypeArgs = [T]`;
4. instantiates only if the template kind is `StructType` or `UnionType`.

At this point the predeclaration kind is still `InvalidType`, so
`instantiateGenericType()` is not called.

The payload is returned as a type with approximately:

```text
Name = Loop
Kind = InvalidType
GenericParameters = [T]
TypeArgs = [T]
```

The union currently being built has approximately:

```text
Name = Loop
Kind = UnionType
GenericParameters = [T]
TypeArgs = []
```

`sameConcreteType()` compares named types using their name and type arguments.
The names match, but the type-argument lists do not:

```text
[] != [T]
```

so the direct recursive union is not recognized.

---

# Example 1 — direct recursive generic union

```sec
type Loop[T] union {
    End
    Next(Loop[T])
}
```

`Loop[int]` requires storage for a `Loop[int]` inside a `Loop[int]` without
indirection.

This has infinite size and must be rejected.

The generic struct equivalent is already diagnosed, but the generic union form
can bypass the corresponding check.

---

# Example 2 — named payload field

The same rule applies to structured union payloads:

```sec
type Loop[T] union {
    End
    Next {
        value: Loop[T],
    }
}
```

The payload field is by-value recursive storage and must be rejected.

Current generic union payload-field analysis only uses the ordinary
`sameConcreteType()` check after resolution.

---

# Example 3 — fixed-array-mediated recursion

```sec
type Loop[T] union {
    End
    Next(Loop[T][2])
}
```

or the canonical equivalent fixed-array spelling accepted by the current
grammar.

A fixed array stores its elements inline, so this remains infinite storage.

The existing generic struct checker recursively follows fixed-array storage.
Generic union payloads do not receive the corresponding generic check.

---

# Example 4 — changing-argument non-convergence

```sec
type Infinite[T] union {
    End
    Next(Infinite[T[]])
}
```

The exact source spelling of the type argument should follow the canonical
collection grammar.

The semantic problem is that instantiation changes:

```text
Infinite[T]
-> Infinite[T[]]
-> Infinite[T[][]]
-> ...
```

and never reaches a finite repeated concrete instance.

Revision 2.0 requires this to be diagnosed.

The existing struct path distinguishes this as non-converging generic
instantiation. The union path has no equivalent check.

---

# Bug 3 — concrete generic-union instantiation does not perform a recursive
# storage check after substitution

`instantiateGenericType()` performs an additional concrete recursive-storage
check for struct fields:

```go
field.Type = substituteGenericType(field.Type, substitution)

if genericStructFieldHasDirectRecursiveStorage(out, field.Type) {
    ...
}
```

The function then substitutes union variants:

```go
for _, variant := range typ.UnionVariants {
    if variant.Payload != nil {
        payload := substituteGenericType(*variant.Payload, substitution)
        variant.Payload = &payload
    }

    for _, field := range variant.PayloadFields {
        field.Type = substituteGenericType(field.Type, substitution)
        ...
    }

    out.UnionVariants = append(out.UnionVariants, variant)
}
```

There is no corresponding finite-storage check on either substituted union
payload form.

Thus the template-level omission is not repaired during concrete
monomorphization.

---

# Related structural hazard — unresolved generic skeletons can be embedded in
# template metadata

The same predeclaration mechanism means a generic type reference encountered
before the referenced generic declaration has completed body analysis can carry
`Kind=InvalidType` into another template's semantic metadata.

This correction requires auditing that broader case as part of the fix.

Do not treat `InvalidType` with a known generic name and arity as a successfully
resolved concrete storage type.

If forward generic references are intended to work — as template
pre-registration implies — they must be completed/fixed up before
representation-dependent analysis or concrete instantiation.

This broader forward-reference issue should either be fixed with the same
template-finalization architecture or recorded separately in governance if it
cannot be completed in the same change.

---

# Required correction

## 1. Apply recursive-generic storage analysis to every inline storage edge

The finite-representation analysis must cover at least:

- generic struct fields;
- generic union direct payloads;
- generic union named payload fields;
- fixed arrays containing any of the above.

The test must be based on storage semantics, not declaration syntax alone.

References, slices, and other representations whose owning rulebook provides a
finite indirection boundary must remain valid.

## 2. Detect direct same-instantiation recursion for generic unions

Reject:

```text
U[T] -> U[T]
```

when the edge is inline storage.

The diagnostic should identify the recursive generic declaration and eventually
the cycle path.

## 3. Detect changing-argument non-convergence for union payloads

Reject paths such as:

```text
U[T] -> U[F(T)] -> U[F(F(T))] -> ...
```

when no finite representation boundary breaks the storage recursion.

Do not special-case only one syntactic transformation such as a slice argument;
the implementation must operate on canonical generic instance identities.

## 4. Perform the proof on concrete specialization as well

Concrete monomorphization must not assume that template-level syntax checking
proved every case.

After substitution, verify that the concrete storage graph is finite before the
instance is eligible for:

- layout;
- default analysis;
- copy/move classification;
- destruction analysis;
- construction;
- match payload semantics;
- ABI/Semantic IR/backend lowering.

## 5. Do not use unresolved `InvalidType` skeletons as resolved payload types

Generic predeclarations may reserve names and arity, but a reference to such a
template must not be mistaken for a completed semantic type.

Choose a coherent architecture, for example:

- two-phase generic template construction followed by reference fixup; or
- explicit template-reference semantic nodes that are resolved when all
  declaration headers/bodies needed for representation are registered.

Do not silently embed `InvalidType` into otherwise accepted union/struct
metadata.

## 6. Preserve canonical instance caching

Cycle detection must distinguish:

```text
same canonical instance revisited through inline storage
```

from:

```text
same canonical instance reached through a finite indirection
```

and from:

```text
an unbounded sequence of changing canonical generic arguments
```

The existing canonical generic-instance identity/cache can be reused, but the
cycle detector must not depend on already materializing an infinite sequence of
instances.

---

# Governance reconciliation

The root governance entry `frontend.generics-v2` currently lists as implemented:

```text
direct, array-mediated, and changing-argument recursive generic storage
diagnostics with indirection cases retained
```

That statement is too broad.

At the audited baseline, those diagnostics are implemented for the tested
generic-struct storage path, but not for generic-union payload storage.

The status should therefore be narrowed to:

```text
direct, array-mediated, and changing-argument recursive generic struct storage
diagnostics, with tested indirection cases retained
```

and generic-union recursion should be listed as remaining until corrected.

This does **not** mean generic unions are wholly unimplemented. Generic union
template registration, ordinary payload substitution, construction, and several
frontend tests already exist.

---

# Required regression tests

Add tests covering at least:

## Generic union direct recursion

1. `U[T]` direct payload of `U[T]` is rejected;
2. named payload field of `U[T]` is rejected;
3. fixed-array payload containing `U[T]` is rejected;
4. fixed-array named payload field containing `U[T]` is rejected.

## Non-converging recursion

5. direct payload changing `U[T] -> U[F(T)]` is rejected;
6. named payload field changing arguments is rejected;
7. array-mediated changing-argument recursion is rejected;
8. diagnostic eventually reports the complete instantiation path required by
   revision 2.0.

## Valid finite recursion

9. `ref U[T]` payload is accepted where ordinary union/reference rules permit it;
10. a slice/reference-mediated recursive payload remains accepted where its
    owning representation is finite;
11. a recursive path broken by a valid owning container is classified according
    to that container's representation rules.

## Generic metadata integrity

12. no accepted generic union concrete instance contains `InvalidType` payload
    metadata;
13. no accepted generic struct concrete instance contains `InvalidType` field
    metadata merely because of declaration order;
14. forward generic references are either fully resolved before
    representation-dependent use or rejected with a focused diagnostic;
15. repeated requests for a valid concrete union instance still reuse the
    canonical cached instance.

## Existing behavior

16. existing generic struct direct-recursion tests continue to pass;
17. existing generic struct array-recursion tests continue to pass;
18. existing generic struct changing-argument tests continue to pass;
19. existing generic `Maybe[T]` union substitution/construction tests continue
    to pass.

## Applied 2026-08-23

Generic union payloads now receive the same pre-resolution direct, fixed-array,
and changing-argument recursion checks as generic struct fields. Concrete union
payloads are checked again after substitution. The broader forward-reference
and complete instantiation-path work remains tracked in `implementation-status.yaml`.
