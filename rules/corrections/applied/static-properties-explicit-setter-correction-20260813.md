# Correction: explicit setter parameter for static properties

- **Status:** Applied 2026-08-13
- **Created:** 2026-08-13
- **Last updated:** 2026-08-13
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** `rules/declarations/static.md`

## Required correction

Any static-property example or grammar that uses an implicit setter value must be replaced.

Invalid:

```sec
static property Mode: ApplicationMode {
    set {
        Application._mode = value
    }
}
```

Canonical:

```sec
static property Mode: ApplicationMode {
    set mode {
        Application._mode = mode
    }
}
```

Fallible static setters use the same explicit parameter rule:

```sec
static property Mode: ApplicationMode {
    try set mode {
        ...
    }
}
```

The setter parameter name is programmer-chosen and may be any valid identifier.

There is no implicit C#-style `value` binding in Sec.

This correction applies equally to prose, grammar, examples, formatter expectations, and tests in `static.md`.
