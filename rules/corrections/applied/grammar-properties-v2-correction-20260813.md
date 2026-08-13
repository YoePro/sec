# Correction: property grammar v2

- **Status:** Applied 2026-08-13
- **Created:** 2026-08-13
- **Last updated:** 2026-08-13
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** foundation grammar

## Implementation property

```text
PropertyDecl
    := "property" Identifier ":" Type "{" PropertyAccessor+ "}"

PropertyAccessor
    := "get" Block
     | "set" Identifier Block
     | "try" "set" Identifier Block
```

A property may contain at most one getter and at most one setter form.

`set` and `try set` are mutually exclusive.

## Static property

```text
StaticPropertyDecl
    := "static" PropertyDecl
```

The explicit setter identifier remains mandatory.

## Interface property requirement

```text
InterfacePropertyDecl
    := "property" Identifier ":" Type "{"
           InterfacePropertyAccessor+
       "}"

InterfacePropertyAccessor
    := "get"
     | "set" Identifier
     | "try" "set" Identifier
```

There is no accessor body in an interface declaration.

There is no implicit setter parameter in any property form.
