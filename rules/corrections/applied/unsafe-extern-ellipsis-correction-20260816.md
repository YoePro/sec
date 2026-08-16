# Correction: bare ellipsis in `unsafe extern` example

- **Status:** Applied 2026-08-16
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** `rules/memory/unsafe.md`

## Problem

The unsafe rulebook currently contains a canonical-looking code example:

```sec
unsafe extern "system" fn rawSysCall(...) int64
```

After functions v2, native Sec typed variadics have explicit parameter syntax:

```sec
values: ...T
```

Foreign/C varargs are a separate FFI/ABI concern.

Bare `...` must therefore not appear in a normative code example as though it were already defined Sec parameter syntax.

## Correction

Replace the example with a concrete non-variadic signature, for example:

```sec
unsafe extern "system" fn rawSysCall(
    number: int,
    argument1: uint64,
) int64
```

The unsafe rulebook should state only that `unsafe extern` combines:

```text
unsafe
    caller has additional proof obligations

extern "..."
    declaration uses foreign linkage/calling contract

fn
    declaration is callable
```

The exact syntax and semantics of foreign variadic declarations are owned by the FFI and ABI rulebooks.

This correction does not define foreign varargs and does not reuse native Sec `...T` semantics for them.
