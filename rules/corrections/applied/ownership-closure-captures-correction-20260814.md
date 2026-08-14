# Correction: closure capture ownership and callable consumption

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Targets:** ownership and copy/move rulebooks

## Capture modes

```sec
capture(value)
capture(-> value)
capture(ref value)
capture(ref mut value)
```

Meanings:

- plain capture: ordinary by-value copy/move by concrete type;
- `->`: forced ownership transfer, even for copyable values;
- `ref`: shared borrow;
- `ref mut`: exclusive mutable borrow.

Owned capture bindings are mutable inside the closure.

No `capture(mut value)` syntax exists.

## Closure copy/move

Closure copy/move classification composes from the complete environment and callable capability.

At minimum:

- move-only capture -> move-only closure;
- `ref mut` capture -> move-only closure;
- `-> fn` callable -> move-only closure;
- copyable owned captures may permit a copyable non-consuming closure;
- copying a mutable closure with copyable owned state duplicates that state independently.

The compiler must not silently turn a copied mutable closure into aliases of one mutable environment.

## Consuming callable

If invocation moves environment state out such that the closure cannot remain valid, the callable type is `-> fn`.

A successful `-> fn` invocation consumes the callable value.

Reuse is invalid.
