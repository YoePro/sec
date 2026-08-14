# Correction: static members and impl v2

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-13
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** `rules/declarations/impl.md`

## Static members

Both the primary implementation and same-module extension fragments may contain static behavioral/type-associated members.

```sec
impl Counter {
    static let Maximum: int := 100
}

impl extends Counter {
    static fn ResetTotal() void {
        Counter.Total = 0
    }
}
```

Primary and extension fragments form one combined member surface. Duplicate/conflicting static members are invalid across the complete implementation.

## Instance versus static methods

Ordinary `fn` inside an impl is instance-bound and has implicit `self`.

`static fn` is type-level and has no `self`.

Do not require `self`, `ref self`, or `ref mut self` in an ordinary method declaration.

## Lifecycle distinction

A `static fn` may be a named helper or factory but is not lifecycle `init`.

`new Type(...)` selects `init`; `static fn` is called explicitly by its declared name.
