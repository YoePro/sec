# Correction: interface and conformance grammar

- **Status:** Applied 2026-08-13
- **Created:** 2026-08-13
- **Last updated:** 2026-08-13
- **Document revision:** 1
- **Sec language version:** 0.1

Apply these grammar changes to synchronize with `rules/declarations/interfaces.md`.

## Required forms

```text
InterfaceDecl
    := "interface" Identifier InterfaceImplementsClause? InterfaceBody

InterfaceImplementsClause
    := "implements" TypeName ("," TypeName)*

PrimaryImplDecl
    := "impl" TypeName ImplImplementsClause? ImplBody

ImplImplementsClause
    := "implements" TypeName ("," TypeName)*

ExtendedImplDecl
    := "impl" "extends" TypeName ImplBody
```

`ImplImplementsClause` is valid only on the primary implementation.

## Interface method modifiers

```text
InterfaceMethod
    := "fn" MethodSignature
     | "mut" "fn" MethodSignature
     | "->" "fn" MethodSignature
     | "static" "fn" MethodSignature
```

Meaning:

- `fn`: shared/non-mutating receiver contract;
- `mut fn`: mutable/exclusive receiver contract;
- `-> fn`: consuming receiver contract;
- `static fn`: type-level member without receiver.

Concrete implementation methods continue to use ordinary `fn` syntax with implicit `self`.

## Remove stale forms

Remove or deprecate canonical use of:

```text
type T struct implements Interface { ... }
ref self
ref mut self
impl Interface for Type
```

The final canonical conformance form is:

```sec
impl Type implements Interface {
    ...
}
```
