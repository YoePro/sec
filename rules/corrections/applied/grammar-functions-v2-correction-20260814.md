# Correction: function and parameter grammar v2

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** foundation grammar

## Ordinary function

```text
FunctionDecl
    := "fn" Identifier GenericParameterList? "(" ParameterList? ")" Type Block
```

The return `Type` and `Block` are mandatory for ordinary Sec functions.

Ordinary bodyless `fn` prototypes are not valid.

Interface method requirements and foreign `extern` declarations use their own grammar.

## Parameters

```text
ParameterList
    := Parameter ("," Parameter)* ","?

Parameter
    := Identifier ":" Type
     | "->" Identifier ":" Type
     | Identifier ":" "..." Type
```

Rules:

- `Identifier : Type` is ordinary by-value or a reference type according to `Type`;
- `-> Identifier : Type` is forced-consuming by-value;
- `Identifier : ... Type` is the final native variadic parameter;
- `->` cannot combine with `ref`, `ref mut`, or `...`;
- at most one variadic parameter is allowed;
- the variadic parameter must be last.

## Return shape

Ordinary functions declare exactly one return type.

`void` is the explicit no-result type.

Multiple return-type syntax is not part of Sec 0.1.
