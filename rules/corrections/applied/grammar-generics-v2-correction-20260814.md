# Correction: generics grammar v2

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** foundation grammar

## Generic parameters

```text
GenericParameterList
    := "[" GenericParameter ("," GenericParameter)* ","? "]"

GenericParameter
    := Identifier GenericConstraintClause?

GenericConstraintClause
    := ":" InterfaceType ("&" InterfaceType)*
```

`&` combines multiple interface constraints.

## Generic functions and methods

Eligible function/method declarations may have a generic parameter list immediately after the function name.

```sec
fn Identity[T](value: T) T {
    return value
}
```

```sec
fn Map[U](value: T) U {
    ...
}
```

## Generic call arguments

```text
GenericArgumentList
    := "[" Type ("," Type)* ","? "]"
```

For function/method calls, an explicit argument list may be a positional prefix of the declaration's generic parameter list. Remaining parameters are inferred.

Generic argument holes are invalid.

## Generic named types and interfaces

Generic parameter lists are valid on eligible named type and interface declarations.

```sec
type ID[T] int

interface Repository[T] {
    ...
}
```

## Enums

Generic parameter lists are invalid on enum declarations.

## Interface method requirements

An interface declaration may be generic, but one of its method requirements may not introduce additional method-level generic parameters.
