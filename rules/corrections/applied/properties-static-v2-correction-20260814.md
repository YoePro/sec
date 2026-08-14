# Correction: static property setter syntax

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-13
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** `rules/declarations/properties.md`

Static properties obey the same explicit setter-parameter rule as instance properties.

Canonical:

```sec
static property Mode: Mode {
    set mode {
        Application._mode = mode
    }
}
```

Fallible:

```sec
static property Mode: Mode {
    try set mode {
        ...
    }
}
```

Invalid:

```sec
static property Mode: Mode {
    set {
        Application._mode = value
    }
}
```

There is no implicit setter variable in static properties.
