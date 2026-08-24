# Match correction — open `error` narrowing

- **Status:** Applied normative correction
- **Applied:** 2026-08-24
- **Created:** 2026-08-24
- **Last updated:** 2026-08-24
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/control-flow/flowcontrol_match.md`

---

## Correction

Keep the Sec 0.1 prohibition on general runtime type patterns, but add the
error-specific narrowing required by compiler-known `error`.

For:

```sec
let result: Result[Data, error] := ...
```

this is valid:

```sec
match result {
    Ok(value) => Use(value)
    Err(IOError.NotFound) => UseDefault()
    Err(errorValue) => Handle(errorValue)
}
```

A concrete error widened to `error` retains its concrete type, variant, payload,
ownership, and destruction identity.

### Open-domain exhaustiveness

`error` is open over all concrete Sec error types.
Therefore concrete error arms alone do not exhaust `Err(error)`.

An exhaustive `Result[T, error]` match requires an error fallback such as:

```sec
Err(errorValue) => ...
```

or:

```sec
Err(_) => ...
```

unless Sema has a proven narrower closed error state.

### Generic catch-all rule remains

The existing rule remains: generic `_` must not silently hide an otherwise
unhandled Result `Err` state.

`Err(_)` is the explicit error acknowledgement form.

### No general runtime type patterns

This correction does not permit forms such as arbitrary `SomeType(value)` type
patterns over `any`, interfaces, structs, or unrelated values.
The special narrowing exists only for the compiler-known error root relation.

## Cross-reference

```text
rules/errors/errorhandling.md
rules/types/types.md
```
