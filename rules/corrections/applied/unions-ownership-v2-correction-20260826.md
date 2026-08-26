# Correction: union move syntax and payload ownership transfer

- **Status:** Normative correction
- **Created:** 2026-08-26
- **Last updated:** 2026-08-26
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/declarations/unions.md`

---

## Correction

Synchronize union move examples and construction ownership with
`rules/memory/ownership.md` revision 2.0.

### Canonical move syntax

Replace legacy examples using word-form move syntax such as:

```sec
let target := move source
```

with canonical explicit move syntax:

```sec
let target :<- source
```

or another context-appropriate `<-` source move.

A moved-from union follows the ordinary ownership availability rules. It does
not become the union-specific `empty` initialization state.

### Variant payload construction

Union constructors do not silently consume reusable source places. If a
reusable payload source must transfer ownership, the move is explicit:

```sec
let choice := Choice.Some(<-resource)
```

For a struct-like variant:

```sec
let message := Message.Data {
    Payload: <-resource,
}
```

Plain construction may copy a copyable reusable source. It must reject a
non-copyable reusable source rather than infer a hidden destructive move.
Fresh temporaries may be forwarded without a redundant move marker.

### Matching remains separate

This correction does not alter the union rulebook's restrictions against hidden
partial moves during reusable-subject destructuring. Match binding ownership is
owned by the match and ownership rulebooks.

## Cross-reference

`rules/memory/ownership.md` revision 2.0 owns explicit reusable-source transfer;
`rules/memory/copy_move.md` owns copyability classification.
