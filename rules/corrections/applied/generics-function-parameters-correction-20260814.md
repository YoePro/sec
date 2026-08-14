# Correction: consuming and variadic generic functions

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** `rules/declarations/generics.md`

Generic functions may use ordinary, borrowed, consuming, and typed variadic parameters.

```sec
fn Store[T](-> value: T) void {
    ...
}
```

The `->` contract makes consumption unconditional across concrete instantiations.

For copyable `T`, the caller value is still consumed.

For move-only `T`, the behavior agrees with ordinary move-only by-value transfer.

Typed generic variadics are valid:

```sec
fn Count[T](values: ...T) int {
    return values.Len
}
```

Each concrete instantiation has one concrete variadic element type.

`-> values: ...T` is invalid.

Generic monomorphization must preserve parameter ownership mode and variadic shape in the concrete callable instance.
