# Registers correction — open bit-backed enums

**Status:** Applied
**Applied:** 2026-08-13
**Language version:** Sec 0.1
**Created:** 2026-08-13
**Last updated:** 2026-08-13
**Target:** `rules/platform/registers.txt`
**Source of truth:** `rules/declarations/enums.md`

A register field whose declared type is `enum Name bit[N]` preserves all N raw bits read from
the register.

The field value remains a valid value of the open bit-backed enum even when the bit pattern
has no declared member name.

A register read must not:

```text
trap
invent a declared member
coerce an unknown pattern to the enum default
silently discard or rewrite bits
```

Hardware-facing code should normally handle undeclared enum encodings explicitly, commonly
through a final `_` arm in `match`.
