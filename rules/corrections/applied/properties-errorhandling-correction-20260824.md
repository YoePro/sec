# Properties correction — explicit fallible setter error type

- **Status:** Applied normative correction
- **Applied:** 2026-08-24
- **Created:** 2026-08-24
- **Last updated:** 2026-08-24
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/declarations/properties.md`

---

## Correction

Replace inferred fallible-setter error contracts with an explicit declared
error type.

Canonical:

```sec
property TopSpeed: Speed {
    try set value SpeedError {
        if value < MinimumSpeed {
            return Err(SpeedError.TooLow)
        }

        self._speed = value
    }
}
```

The error type after the setter parameter is required and must resolve to
`error` or a concrete Sec error type.

### Success and failure

For `try set`:

```text
normal completion        -> success
return                   -> early success
return Err(errorValue)   -> failure
```

`return Ok()` is invalid inside a `try set` body.
The setter is not a source-level `Result[void, E]` function.

### Interface requirements

Canonical interface requirement:

```sec
property Position: Point {
    try set value PositionError
}
```

The implementation must satisfy the declared error contract.

### Assignment

Fallible setter assignment requires `try`.
Both forms are valid:

```sec
try object.Property = value
```

```sec
try object.Property = value {
    Err(errorValue) => Handle(errorValue)
}
```

### Getters

Property getters are infallible in Sec 0.1.
Do not add `try get` syntax in this revision.

## Cross-reference

```text
rules/errors/errorhandling.md
```
