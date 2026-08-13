# Correction: Grammar sync for `impl` revision 2.0

**Target:** `rules/foundations/grammar.md`  
**Source rule:** `rules/declarations/impl.md` revision 2.0  
**Date:** 2026-08-13  
**Merge mode:** targeted correction; do not replace unrelated grammar content

## Required changes

Retain the implemented canonical implementation syntax:

```text
ImplDeclaration
    ::= "impl" [ "extends" ] TypeReference ImplBody
```

Retain:

```sec
impl Device {
}

impl extends Device {
}
```

as the canonical primary/fragment forms.

### Impl member grammar

Extend `ImplMember` to include lifecycle and nested implementation members:

```text
ImplMember
    ::= FunctionDeclaration
      | StaticFunctionDeclaration
      | StaticLetDeclaration
      | PropertyDeclaration
      | EventDeclaration
      | NestedTypeDeclaration
      | NestedUnitDeclaration
      | NestedEnumDeclaration
      | InitDeclaration
      | FreeDeclaration
      | NestedImplDeclaration
      | UnitMetadataDeclaration
```

Associated struct/union/register/named-type forms use their canonical nested
`type Name ...` syntax rather than standalone `struct Name ...` syntax.

### `init`

Add contextual impl-member grammar:

```text
InitDeclaration
    ::= "init" "(" [ ParameterList ] ")" [ TypeReference ] Block
```

The optional trailing `TypeReference` is the construction error type, not a
return type.

`init` is not introduced by `fn`.

### `free`

Add:

```text
FreeDeclaration
    ::= "free" Block
```

`free` is a special lifecycle member, not a function declaration.

### Nested impl

Add a nested implementation form that is valid only for a nested/associated type
owned by the enclosing type:

```text
NestedImplDeclaration
    ::= "impl" TypeReference ImplBody
```

Sema resolves an unqualified nested target against the enclosing owner.

### `new` expression

Add a construction expression:

```text
NewExpression
    ::= "new" TypeReference "(" [ ArgumentList ] ")"
```

`new Type(args...)` is an expression whose successful type is `Type`.

It selects lifecycle construction (`init` or a permitted implicit construction
path) and never means conversion/casting.

Keep existing conversion/call syntax separate:

```sec
Percent(50)          // conversion
new Buffer(4096)     // lifecycle construction
```

### Fallibility

A `new` expression whose selected `init` declares an error type is a fallible
construction operation and therefore participates in normal `try`/error
handling.

```sec
let value := try new Resource(args)
```

### Implicit self

Retain the canonical grammar statement that ordinary impl methods have implicit
`self` and do not write `self` in the parameter list.

Explicit legacy `ref self` / `ref mut self` forms must not be presented as
canonical source.

### Status correction

Mark these implementation deltas accurately:

- `impl Type` — implemented;
- `impl extends Type` — implemented;
- implicit self — implemented in Sema/canonical grammar;
- `init` — not implemented;
- nested `impl` — not implemented;
- `free` lifecycle member — reserved but not implemented;
- `new` keyword/expression — not implemented.
