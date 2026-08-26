# Correction: explicit destructive transfer from reusable sources

- **Status:** Normative correction
- **Created:** 2026-08-26
- **Last updated:** 2026-08-26
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/memory/copy_move.md`

---

## Correction

Synchronize copy/move classification examples and consuming contexts with
`rules/memory/ownership.md` revision 2.0.

### Ordinary syntax never hides a move from a reusable source

When a reusable source can be copied, ordinary value syntax may copy it.
When a reusable source cannot be copied, ordinary value syntax must not silently
change into a destructive move.

Therefore examples such as these are invalid for a non-copyable reusable
source:

```sec
Consume(resource)
let option := Some(resource)
let result := Ok(resource)
```

Canonical consuming forms are explicit:

```sec
Consume(<-resource)
let option := Some(<-resource)
let result := Ok(<-resource)
```

The same rule applies to union variants, struct/aggregate fields, collection or
wrapper payloads, and equivalent owning construction contexts.

### Consuming parameter contract

A `->` parameter requires `<-` at the call site for a reusable source even when
the source type is copyable. This preserves the API's explicit consumption
contract.

### Fresh temporary forwarding

Fresh temporaries may be forwarded directly into their first owner without
synthetic move boilerplate:

```sec
Consume(CreateResource())
let option := Some(CreateResource())
```

### Return boundary

Returning an owned local transfers ownership. The following are equivalent with
respect to ownership transfer:

```sec
return resource
```

```sec
return <-resource
```

The marker is optional at return boundaries. Remove statements that
`return <-value` is nonexistent or inherently invalid.

### Copyability and destruction remain separate

Do not use move-only/copy classification as a proxy for whether a value requires
non-trivial destruction. The destruction classification is an independent
compiler property.

## Cross-reference

`rules/memory/ownership.md` revision 2.0 owns explicit source-transfer and
availability semantics.
