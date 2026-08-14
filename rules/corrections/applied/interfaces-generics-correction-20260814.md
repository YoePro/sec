# Correction: generic constraints in interfaces documentation

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Targets:** `rules/declarations/interfaces.md` and related interface examples

## Generic constraints use `:`

Generic constraints do not use `implements`.

Invalid:

```sec
fn Print[T implements Printable](value: ref T) void {
    ...
}
```

Canonical:

```sec
fn Print[T: Printable](value: ref T) void {
    ...
}
```

`T: Printable` means that `T` is constrained to concrete types that satisfy `Printable`.

It does not declare implementation.

Explicit implementation remains:

```sec
impl Document implements Printable {
    ...
}
```

## Generic interfaces

Interfaces may themselves have generic parameters.

```sec
interface Sink[T] {
    mut fn Write(value: T) void
}
```

A concretely instantiated interface such as `Sink[byte]` is a concrete interface contract.

## Method-level generic parameters

Interface method requirements may use the generic parameters of the interface but may not introduce additional method-level generic parameters.

Invalid:

```sec
interface Mapper[T] {
    fn Map[U](value: T) U
}
```

This keeps runtime interface dispatch finite and does not introduce runtime generic dispatch.
