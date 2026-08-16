# Correction: `RawPtr[T]` null value domain versus null source syntax

- **Status:** Applied 2026-08-16
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** `rules/memory/raw_pointers.txt`

## Required correction

Replace the statement that a `RawPtr[T]` may represent null only at an FFI boundary or inside an extern wrapper.

Canonical rule:

> Null is part of the possible value domain of `RawPtr[T]`.

A `RawPtr[T]` may therefore contain a null address after it has been produced by a foreign operation, unsafe conversion, platform operation, or another valid raw-pointer-producing operation.

Possessing, storing, moving, returning, passing, or testing such a `RawPtr[T]` does not by itself create a safe reference and does not by itself assert pointee validity.

Safe references remain non-null:

```text
ref T
ref mut T
ref T[]
```

A null raw pointer may never be converted to a safe reference without first establishing the non-nullness and all other reference proof obligations.

## Source-level null construction is separate

The fact that `RawPtr[T]` can contain null does not imply that ordinary Sec code may freely construct null pointers through a `null` literal.

The exact source contexts and syntax for producing or writing a foreign null pointer are owned by the FFI/raw-pointer construction rules.

This distinction is required:

```text
RawPtr value domain
    may include null

safe reference value domain
    never includes null

source-level null literal/construction
    separately restricted
```

FFI wrappers should normally convert nullable foreign results into explicit safe-domain representations such as:

```sec
Option[RawPtr[T]]
```

or a validated domain-specific wrapper when raw nullability should not escape the foreign boundary.
