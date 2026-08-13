# Correction: generic impls and implementation fragments

**Target:** `rules/declarations/generics.txt`  
**Source rule:** `rules/declarations/impl.md` revision 2.0  
**Date:** 2026-08-13

## Replace the stale one-block wording

Replace statements equivalent to:

```text
A generic type follows the normal rule of at most one impl block.
```

with the revision-2.0 implementation model:

> A generic named type has exactly one primary ordinary `impl Type[T...]` in its
> defining module. Additional same-module behavior fragments may use
> `impl extends Type[T...]`. The primary and all valid fragments form one merged
> implementation/member surface.

Example:

```sec
type Stack[T] struct {
    items: list[T],
}

impl Stack[T] {
    fn Push(value: T) void {
        // ...
    }
}
```

A separate file in the same module may contain:

```sec
impl extends Stack[T] {
    fn Pop() Option[T] {
        // ...
    }
}
```

## Generic parameter rules

The generic target parameters/arguments in every extension fragment must match
the target generic declaration under the same generic-impl rules as the primary
implementation.

`extends` does not introduce unrelated generic parameters.

Duplicate methods, properties, nested declarations, `init` signatures, or other
members remain duplicates even when they occur in different generic impl
fragments/files.

## Lifecycle members

Generic impls may contain `init`/`free` when permitted by `impl.md` and the
instantiated target type's semantics.

Lifecycle overload selection/monomorphization follows the same concrete generic
specialization model as other impl members.

## Related rule

Update references from `impl.txt` to `impl.md` revision 2.0.
