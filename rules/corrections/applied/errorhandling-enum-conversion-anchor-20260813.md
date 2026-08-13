# Error-handling anchor — checked enum conversion

**Status:** Applied
**Applied:** 2026-08-13
**Language version:** Sec 0.1
**Created:** 2026-08-13
**Last updated:** 2026-08-13
**Target:** `rules/errors/errorhandling.txt`
**Source of truth:** `rules/declarations/enums.md`

Checked enum conversions participate in the normal `try` model.

Canonical enum conversion error family:

```text
EnumValueError.UndeclaredValue
EnumValueError.OutOfRange
```

For a closed ordinary enum:

```sec
let value := try Color(raw)
```

may fail with:

```text
UndeclaredValue
    the integer is representable by the enum underlying type but is not a declared enum value

OutOfRange
    the integer cannot be represented by the enum underlying representation
```

For an open `bit[N]` enum, an undeclared but in-range bit pattern is valid and therefore never
produces `UndeclaredValue`. Only an out-of-width conversion can fail.

A conversion proven infallible by Sema requires no runtime validation.
A conversion that may fail requires the normal Sec `try` handling or propagation semantics.
