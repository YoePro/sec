# Correction: ownership move markers at calls, captures, payloads, and returns

- **Status:** Normative correction
- **Created:** 2026-08-26
- **Last updated:** 2026-08-26
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/foundations/grammar.md`

---

## Correction

Synchronize the grammar with `rules/memory/ownership.md` revision 2.0.

### Capture syntax

Replace the legacy consuming capture entry using `->` with an explicit move
expression using `<-`.

Conceptually:

```text
CaptureEntry
    ::= Identifier
      | "<-" Identifier
      | "ref" Identifier
      | "ref" "mut" Identifier
```

Canonical:

```sec
capture(<-resource)
```

Legacy:

```sec
capture(-> resource) // invalid
```

### Move expressions in argument and payload positions

The parser must allow an explicit move expression wherever an owning expression
position accepts it, including function arguments and constructor/aggregate
payloads:

```sec
Consume(<-resource)
```

```sec
let option := Some(<-resource)
```

```sec
let packet := Packet {
    Payload: <-resource,
}
```

Grammar acceptance does not itself decide whether the source is movable or
whether the target is consuming; those are Sema/ownership checks.

### Return move marker

Remove any grammar statement that `return <-value` does not exist. Both forms
are grammatical:

```sec
return value
return <-value
```

The ownership rulebook makes the move marker optional at a return boundary.

### Assignment remains a statement

This correction does not make assignment a general value-producing expression.
Existing assignment-statement rules remain unchanged.

## Cross-reference

`rules/memory/ownership.md` revision 2.0 owns when a move marker is required,
optional, or invalid.
