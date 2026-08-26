# Correction: preserve semantic ownership markers

- **Status:** Normative correction
- **Created:** 2026-08-26
- **Last updated:** 2026-08-26
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/tooling/formatter.md`

---

## Correction

The formatter must preserve all source-level ownership markers defined by
`rules/memory/ownership.md` revision 2.0.

Canonical examples include:

```sec
let destination :<- source
```

```sec
destination <- source
```

```sec
Consume(<-source)
```

```sec
let option := Some(<-source)
```

```sec
let packet := Packet {
    Payload: <-source,
}
```

```sec
capture(<-source)
```

```sec
return <-source
```

The formatter may normalize whitespace according to ordinary formatting rules,
but it must never add, remove, or relocate a semantic move marker as a
style-only rewrite.

The legacy capture spelling:

```sec
capture(-> source)
```

is not canonical revision-2 syntax and must not be produced by the formatter.

Formatter idempotence tests must include ownership markers in call, capture,
payload, declaration, assignment, and optional return positions.

## Cross-reference

Parser/Sema decide whether a marker is legal or required. The formatter only
preserves and canonically lays out the accepted syntax.
