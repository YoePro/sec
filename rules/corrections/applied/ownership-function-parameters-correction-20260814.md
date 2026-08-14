# Correction: function parameter ownership and call transfer

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Targets:** ownership, copy/move, and borrowing rulebooks

## Owned parameter binding

An ordinary by-value parameter is an owned mutable local binding.

```sec
fn Normalize(value: int) int {
    if value < 0 {
        value = 0
    }

    return value
}
```

For ordinary by-value parameters:

- copyable argument type: callee receives an owned copy;
- move-only argument type: ownership transfers to the callee.

The parameter binding may be reassigned under ordinary ownership/destruction rules.

This replaces stale wording that all parameters are immutable local symbols.

## Borrowed parameters

`ref T` remains shared borrowed access.

`ref mut T` remains exclusive mutable borrowed access.

The owned source remains with the caller.

Owned-parameter mutability does not change borrow authority.

## Forced-consuming parameter

```sec
fn Transform(-> value: T) T {
    ...
}
```

`->` forces ownership consumption even when `T` would otherwise be implicitly copied.

It does not grant ownership of a pointee represented by a non-owning pointer.

`->` cannot combine with `ref` or `ref mut`.

## Overloads

Two functions may not differ only by `T` versus `-> T`.

Ownership consumption must never be selected by overload resolution.

## Call transfer commit

Arguments evaluate left-to-right.

Caller-owned values and temporaries remain caller responsibility until all arguments have evaluated successfully and the call is ready to enter the callee.

At call entry, ownership transfer to consuming/move-only parameters is committed.

If a later argument evaluation fails before call entry:

- the callee is not entered;
- earlier caller-owned bindings are not consumed by that failed outer call;
- earlier temporaries are cleaned by the caller.

Effects and ownership changes performed by evaluating an earlier argument expression itself are not rolled back. Only the outer call's parameter-transfer commit is delayed.
