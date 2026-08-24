# Formatter correction — errorhandling revision 2 syntax

- **Status:** Applied normative correction
- **Applied:** 2026-08-24
- **Created:** 2026-08-24
- **Last updated:** 2026-08-24
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/tooling/formatter.md`

---

## Correction

The formatter must preserve and canonicalize revision-2 error-handling syntax
without changing semantics.

Canonical forms include:

```sec
enum IOError error {
    ReadError
}
```

```sec
type DetailedError union error {
    Failed {
        Code: int
    }
}
```

```sec
try set value IOError {
    ...
}
```

```sec
try Read() {
    Err(errorValue) where IsTemporary(errorValue) => Retry()
}
```

```sec
if option is Some(value) {
    ...
}
```

```sec
return try Load()
```

The formatter must not emit obsolete:

```text
explicit Ok/Some handlers inside try
try { match { ... } }
try set without its declared error type when formatting canonical valid source
```

Formatting must not rewrite consuming `.Ok()`/`.Err()` into borrowed `OkRef`/
`ErrRef`, or the reverse; those operations have different ownership semantics.

## Cross-reference

```text
rules/errors/errorhandling.md
```
