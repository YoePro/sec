# Runtime checks correction — try failure sets and revision-2 propagation

- **Status:** Applied normative correction
- **Applied:** 2026-08-24
- **Created:** 2026-08-24
- **Last updated:** 2026-08-24
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/errors/runtime_checks.md`

---

## Correction

Retain the current full-expression `try`, left-to-right evaluation,
first-failure behavior, and error assignability rules.
Synchronize them with errorhandling revision 2.

### Failure sets

Each individual fallible operation has a defined failure type, but one protected
expression may contain multiple fallible sources with different concrete error
types.

The compiler may track an internal failure set for such an expression.
This set is analysis metadata only and must not become an inferred source type or
public anonymous union.

### Propagation

For naked Result/error propagation, every unhandled failure source must be
assignable to the enclosing declared Result error type.

Concrete errors are assignable to compiler-known `error`.
The compiler must not choose a user union wrapper or variant.

### Local handlers are partial

A local `try` handler list no longer has to be exhaustive.
Unmatched failures continue through ordinary `try` propagation.

`Err(_)` may handle a heterogeneous failure set without introducing a payload
binding.

A named catch-all binding requires a single compiler-resolved binding type and
must not cause Sema to invent a hidden union or common type solely for the local
binding.

### Handler-generated failures

Failures originating in a handler body or guard are outside the protected set of
the same `try`. They require their own ordinary handling/propagation.

### Option

`Option[T]` `try` behavior belongs to `errorhandling.md`; `None` remains absence
and does not become a runtime-check error.

## Cross-reference

```text
rules/errors/errorhandling.md
```
