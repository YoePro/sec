# Types correction — compiler-known `error` and Result error channels

- **Status:** Applied normative correction
- **Applied:** 2026-08-24
- **Created:** 2026-08-24
- **Last updated:** 2026-08-24
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/types/types.md`

---

## Correction

Add `error` to the compiler-known lowercase fundamental type set.

### Type category

`error` is a language-level special error root type.
It requires no import and is independent of backend representation.

The type overview must include it among compiler-known fundamental/special types.

### Error-specific assignability

For a concrete type declared as an error type:

```text
ConcreteError -> error
```

is a valid implicit widening.

The following remain invalid implicit conversions:

```text
error -> ConcreteError
ErrorA -> ErrorB
```

unless another explicit operation is defined.

This is not general inheritance and must not make unrelated named types
assignable.

### Identity preservation

Widening to `error` preserves concrete error identity, active variant, payload,
ownership state, and destruction requirements.

Type erasure at a backend boundary must not erase source-semantic identity needed
for matching, destruction, or diagnostics.

### Result constraint

Update the Result overview so:

```text
Result[T, E]
```

requires `E` to be `error` or a concrete Sec error type.

`Result[T, string]` is not a valid error channel.

### Open versus precise channel

`Result[T, ConcreteError]` is a precise concrete error channel.
`Result[T, error]` is intentionally open but still statically typed.

## Cross-reference

Detailed error semantics are owned by:

```text
rules/errors/errorhandling.md
```
