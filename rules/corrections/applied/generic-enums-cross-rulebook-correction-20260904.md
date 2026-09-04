# Applied Correction — Generic Enums Cross-Rulebook Correction

- **Status:** Applied normative correction and synchronization package
- **Created:** 2026-09-04
- **Last updated:** 2026-09-04
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `998d8d1`
- **Primary owning rulebooks:** `rules/declarations/generics.md`, enum declaration rulebook, planned `rules/compiler/generics_lowering.md`
- **Intended correction area:** `rules/corrections/`
- **Classification:** Normative correction of an over-restrictive Sec 0.1 generics rule

## 1. Purpose

### 1.1 Correction

Generic enums are part of the Sec 0.1 generic model.

The previous rule that enum declarations cannot have generic parameters is removed.

The previous exclusion was an unnecessary scope restriction. An enum does not need a generic payload in order for a generic parameter to be semantically meaningful. A generic argument may contribute to nominal type identity, constraints, associated behavior, or other compile-time generic semantics even when all enum variants remain payload-less.

### 1.2 Canonical principle

A generic enum is a compile-time generic nominal enum template.

Each concrete generic argument list produces or reuses a distinct concrete nominal enum instantiation according to the ordinary Sec generic identity rules.

Generic parameters do not exist as runtime values.

Generic enum support does not introduce runtime generic dispatch, type erasure, implicit boxing, reflection, or hidden runtime type descriptors.

### 1.3 Example

Valid:

```sec
enum SortOrder[T] {
    Ascending
    Descending
}
```

These are distinct concrete nominal enum types:

```sec
SortOrder[User]
SortOrder[Product]
```

They may have identical physical enum representation while remaining different Sec types.

---

## 2. `rules/declarations/generics.md`

### 2.1 Supported generic declaration families

Add enums to the Sec 0.1 supported generic declaration families.

The supported-family list shall include at least:

```text
structs
unions
enums
named types
interfaces
functions
methods
eligible implementation targets
eligible nested declarations
```

### 2.2 Replace the current generic-enum prohibition

Replace the current section that states:

```text
Enums do not accept generic parameters.
```

with the following rule.

A generic enum declaration may declare ordinary Sec type parameters.

Example:

```sec
enum State[T] {
    Ready
    Busy
}
```

`State[Connection]` and `State[Socket]` are different concrete nominal enum types.

The enum variant set remains governed by the ordinary enum rules.

A generic enum does not acquire union-style payloads merely because it is generic. Payload-bearing alternatives remain a union concept.

### 2.3 Generic parameter use

A generic enum parameter is semantically used when it contributes to the identity or behavior of the generic enum, even if the parameter does not occur in a variant value or in the physical representation.

The compiler must not reject a generic enum merely because the type parameter is phantom with respect to runtime storage.

Example:

```sec
enum Permission[T] {
    Read
    Write
}
```

The parameter `T` contributes to the concrete nominal identity.

### 2.4 Generic enum implementation behavior

An `impl` associated with a generic enum may use the enum's generic parameters according to the ordinary generic and impl rules.

Example:

```sec
enum Policy[T] {
    Default
    Strict
}

impl Policy[T] {
    fn Apply(value: T) void {
        ...
    }
}
```

All operations in the generic implementation remain subject to ordinary generic constraint and capability rules.

### 2.5 Underlying enum representation

Generic enums follow the ordinary enum representation rules after substitution.

Sec 0.1 does not gain a new implicit concept of "any enum backing type" merely through this correction.

If a generic parameter is used in a position that determines the enum's underlying representation, that use is valid only when the ordinary enum and generic rules can prove the concrete representation valid for every permitted instantiation.

A later dedicated rule may further constrain or extend generic parameterization of enum backing representation without revoking generic enum identity itself.

### 2.6 Unsupported-mechanism list

Remove:

```text
generic enums
```

from the list of mechanisms that are not part of the Sec 0.1 generic model.

### 2.7 Parser requirements

Remove the parser/Sema requirement to reject:

```text
generic parameters on enums
```

The parser must instead accept generic parameter lists on enum declarations using the same generic-parameter grammar used by other eligible generic declarations.

### 2.8 Sema requirements

Sema must:

- register a generic enum declaration as a generic nominal enum template;
- register its generic parameters as type symbols;
- enforce generic arity and constraint rules;
- preserve the enum declaration's nominal identity;
- create or resolve concrete enum identities from the complete generic argument list;
- validate ordinary enum rules after substitution;
- make the generic parameters available to matching generic impl scopes;
- reject only concrete instantiations that violate ordinary enum, generic, representation, or constraint rules.

### 2.9 Diagnostics

Remove the required diagnostic:

```text
enum declarations cannot have generic parameters
```

Generic enum diagnostics shall instead use ordinary generic diagnostics where applicable, including:

```text
wrong generic arity
unknown generic constraint
unsatisfied generic constraint
invalid concrete enum representation
```

with concrete instantiation context where the failure is concrete-dependent.

### 2.10 Best practice

Remove advice equivalent to:

```text
do not model payload variation with generic enums
```

when it is used to imply that generic enums themselves are invalid.

The documentation may still explain that payload-bearing alternatives belong in unions.

A preferable distinction is:

```text
Use unions when variants carry different payloads.
Use generic enums when parameterization of enum identity or behavior is useful.
```

---

## 3. Planned `rules/compiler/generics_lowering.md`

### 3.1 Generic enum lowering

Generic enum declarations participate in the same generic lowering pipeline as other generic nominal declarations.

Given:

```sec
enum State[T] {
    Ready
    Busy
}
```

and the substitution:

```text
T -> Connection
```

generic lowering produces a concrete nominal enum:

```text
State[Connection]
```

with the ordinary concrete enum variant set:

```text
Ready
Busy
```

### 3.2 Identity

Concrete generic enum identity includes at least:

```text
GenericEnumDeclarationIdentity
ConcreteGenericArguments
CanonicalConcreteEnumIdentity
```

Two different concrete argument lists produce different Sec types even when their runtime representation is identical.

### 3.3 Variant identity

Variants remain projections of the source enum declaration under the concrete owner identity.

Conceptually:

```text
VariantDeclarationIdentity
+
ConcreteEnumOwnerIdentity
```

must be sufficient to distinguish variant identity for concrete generic enum instantiations.

### 3.4 Representation lowering

The generic enum must be fully concretized before representation-dependent enum lowering.

After generic lowering, the concrete generic enum is handled by the ordinary enum lowering rules.

No unresolved generic parameter may reach executable representation-dependent MLIR or LLVM enum lowering.

### 3.5 No runtime generic machinery

Generic enum lowering must not introduce:

```text
runtime type descriptors
generic dictionaries
type erasure
runtime generic dispatch
implicit boxing
```

The generic parameter contributes only compile-time semantics unless another ordinary Sec feature independently requires runtime representation.

---

## 4. `rules/compiler/grammar.md`

### 4.1 Canonical grammar

Generic parameter syntax is valid on enum declarations.

The canonical declaration grammar must permit the equivalent of:

```sec
enum Name[T] {
    ...
}
```

including ordinary constraint syntax where generic constraints are otherwise valid.

### 4.2 Implementation-status wording

The current implementation-status statement that generic enums are incomplete may remain until parser/Sema/lowering support is actually implemented.

It must mean:

```text
canonical Sec 0.1 feature not yet fully implemented
```

and must not be interpreted as:

```text
feature forbidden by Sec 0.1
```

### 4.3 Parser compatibility

Any parser path that currently rejects an enum solely because a generic parameter list is present must be changed to parse the generic enum declaration and preserve its generic parameter information for Sema.

---

## 5. `implementation-status-generics.yaml`

### 5.1 Remove deliberate exclusion

Remove:

```yaml
- generic enums
```

from:

```yaml
deliberately_not_part_of_sec_0_1_generics:
```

### 5.2 Add required implementation work

Add a required change equivalent to:

```yaml
- id: generic-enums
  status: required
  areas:
    - parser
    - ast
    - sema
    - type_identity
    - enum_semantics
    - generic_impl
    - semantic_ir
    - lowering
    - diagnostics
    - tests
  note: >
    Generic enums are canonical Sec 0.1 generic nominal types.
    Concrete argument lists produce distinct nominal enum identities.
    Generic enums follow ordinary enum representation and behavior rules
    after substitution and must not introduce runtime generic machinery.
```

### 5.3 Test status

Replace:

```yaml
- generic enum rejected
```

with required valid coverage for generic enums.

At minimum include:

```text
generic enum with one type parameter
different concrete generic enum instantiations are distinct nominal types
same concrete generic enum instantiation canonicalizes to the same type
generic enum values use ordinary enum semantics after substitution
generic impl on generic enum can use the owner generic parameter
wrong generic enum arity is rejected
generic enum constraint failure is rejected where constraints are used
```

---

## 6. Generic parser/Sema testdata

### 6.1 `generics_invalid.sec`

Remove a generic enum declaration from the invalid suite when its only reason for invalidity is that it has generic parameters.

The existing test equivalent to:

```sec
enum GenericEnum[T] {
    First
    Second
}
```

must no longer be expected to fail.

### 6.2 Valid generic testdata

Add the generic enum declaration to valid generic test coverage.

Example:

```sec
enum SortOrder[T] {
    Ascending
    Descending
}

fn AcceptUserOrder(value: SortOrder[User]) void {
    discard value
}

fn UseGenericEnum() void {
    let userOrder: SortOrder[User] := SortOrder[User].Ascending
    AcceptUserOrder(userOrder)
}
```

Add a distinct-instantiation check demonstrating that:

```text
SortOrder[User]
```

and:

```text
SortOrder[Product]
```

are not assignment-compatible merely because the enum representations are identical.

### 6.3 Invalid generic enum tests that remain valid

Generic enum test failures remain appropriate when they exercise ordinary generic or enum errors rather than a blanket declaration prohibition.

Examples include:

```text
wrong generic arity
invalid generic argument
unsatisfied generic constraint
invalid underlying enum representation after substitution
ordinary duplicate/invalid enum member rules
```

---

## 7. Manual and language documentation

### 7.1 `14-generics.html`

Remove `generic enums` from any list titled or equivalent to:

```text
Current Limitations
Unsupported generic mechanisms
```

### 7.2 Add generic enum documentation

The generic documentation should explain that generic enums parameterize nominal enum identity and may also parameterize associated behavior.

Example:

```sec
enum SortOrder[T] {
    Ascending
    Descending
}
```

Explain that:

```sec
SortOrder[User]
SortOrder[Product]
```

are distinct types even if they use the same enum representation.

### 7.3 Union distinction

Documentation may continue to recommend unions for payload-bearing alternatives.

It must not use that recommendation as a reason to prohibit generic enums.

---

## 8. Replaced legacy generic rule material

### 8.1 Legacy rule

The replaced `rules/declarations/generics.txt` and any retained copies state that generic enums are unsupported in the initial implementation and place generic enum declarations in invalid-test lists.

Those statements are superseded by this correction.

### 8.2 Historical versus canonical status

If the legacy rule is retained for history, it need not be rewritten as normative source, but any tooling, migration note, implementation plan, or documentation that still consumes it must not treat its generic-enum prohibition as current Sec 0.1 semantics.

---

## 9. Enum rulebook synchronization

### 9.1 Enum declaration ownership

The canonical enum rulebook continues to own:

```text
variant declarations
underlying representation
explicit values
aliases
enum conversions
enum comparison
enum matching
enum representation
```

### 9.2 Generic ownership

The generic rulebook owns:

```text
generic parameter declaration
generic argument validation
generic constraints
concrete generic identity
generic substitution
```

### 9.3 Combined rule

A concrete generic enum is first resolved as a concrete generic nominal type and then follows ordinary enum semantics.

Neither rulebook should duplicate the other's full semantics.

---

## 10. Semantic invariants

### 10.1 Nominal identity

For a generic enum declaration `E[T]`:

```text
E[A] == E[A]
E[A] != E[B]
```

when `A` and `B` are different concrete Sec type identities.

This holds even when the physical enum representation is identical.

### 10.2 Compile-time generics

The generic argument does not exist as a runtime type parameter.

No runtime type test is required to distinguish `E[A]` from `E[B]`; their Sec type identities are resolved during compilation.

### 10.3 Ordinary enum behavior

Once concretized, the generic enum follows ordinary enum rules for:

```text
construction
assignment
conversion
comparison
switch
match
exhaustiveness
representation
```

### 10.4 Constraints

Operations in generic enum implementations may use only capabilities guaranteed by their declared generic constraints.

A concrete argument with additional accidental capabilities does not retroactively expand the generic enum template's contract.

---

## 11. Implementation guidance

### 11.1 Parser/AST

Do not special-case enum declarations as permanently non-generic.

Reuse the canonical generic parameter AST representation on enum declarations.

### 11.2 Sema

Generic enum template registration should follow the same declaration-identity and generic-scope principles used for other generic nominal declarations.

### 11.3 Concrete instance

A concrete enum instance should retain:

```text
source generic enum declaration
ordered concrete generic arguments
canonical concrete enum identity
ordinary concrete enum representation facts
```

### 11.4 Lowering

After successful generic lowering, downstream enum lowering should not need to know that the enum originated from an unresolved generic template except for preserved provenance/diagnostics metadata.

---

## 12. Required repository synchronization checklist

The following locations must be reviewed and synchronized where present:

```text
rules/declarations/generics.md
rules/compiler/grammar.md
implementation-status-generics.yaml
generic parser/Sema valid and invalid testdata
manual/docs generic limitations, including 14-generics.html
rules/declarations/generics.txt if retained as implementation input rather than pure archive
planned rules/compiler/generics_lowering.md
enum declaration rulebook if it contains an eligible-declaration list
language-rulebook/status or roadmap text if it classifies generic enums as deliberately excluded
```

Any other file that states or assumes:

```text
generic enums are forbidden
enum declarations cannot have generic parameters
generic enum declarations belong in invalid tests solely because they are generic
```

must be corrected to the canonical Sec 0.1 rule defined here.

---

## 13. Superseding rule

The canonical rule is:

> Sec 0.1 permits generic enum declarations. A generic enum is a compile-time generic nominal enum template. Each complete concrete generic argument list identifies a concrete nominal enum type, and ordinary enum semantics apply after substitution. Generic enums do not introduce runtime generic parameters or runtime generic dispatch.

This rule supersedes all earlier blanket prohibitions on generic enums.
