# Correction: static member grammar v2

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-13
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** foundation grammar

Canonical static forms include:

```text
StaticStorageDecl
    := "static" LetDecl

StaticMethodDecl
    := "static" FunctionDecl

StaticPropertyDecl
    := "static" PropertyDecl
```

Inside an impl:

- ordinary `fn` is instance-bound with implicit `self`;
- `static fn` is type-level and has no receiver.

Do not require an explicit `self` parameter to distinguish instance from static methods.

Static property setter grammar inherits the canonical property rule:

```text
"set" Identifier Block
"try" "set" Identifier Block
```

There is no implicit setter parameter.
