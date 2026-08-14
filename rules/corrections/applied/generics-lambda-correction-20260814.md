# Correction: generic functions versus lambdas

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** `rules/declarations/generics.md`

Generic named functions and generic methods are canonical Sec 0.1.

Generic lambda templates are not.

Invalid:

```sec
fn[T](value: T) T {
    return value
}
```

Reason: a generic lambda would be an anonymous compile-time generic template, not one concrete runtime callable value.

Use a named generic function and concretely specialize it before using it as a callable value.

```sec
fn Identity[T](value: T) T {
    return value
}

let identity: fn(int) int := Identity[int]
```
