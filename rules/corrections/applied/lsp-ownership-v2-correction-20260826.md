# Correction: ownership facts, availability refinement, and move assistance

- **Status:** Normative correction
- **Created:** 2026-08-26
- **Last updated:** 2026-08-26
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/tooling/lsp.md`

---

## Correction

Synchronize LSP ownership behavior with `rules/memory/ownership.md` revision
2.0.

The LSP must consume the same Sema ownership facts used by `sec build`; it must
not maintain a separate approximate ownership model.

### Surface canonical ownership facts

Where useful in hover, diagnostics, code actions, or related UI, expose:

- whether a place is currently available, partially available, unavailable, or
  conditionally available;
- the prior operation/source location that moved, discarded, or detached it;
- whether a call/payload/capture requires an explicit `<-` marker;
- whether a branch refines `place is available` or `place is not available`;
- whether a whole aggregate is unavailable because one of its sub-places moved;
- whether a build policy forbids required runtime ownership bookkeeping.

### Mentor-style actions

When a reusable source would be consumed without an explicit marker, offer the
canonical repair when unambiguous:

```sec
Consume(<-resource)
```

Likewise, for a non-copyable plain closure capture, suggest
`capture(<-resource)` or the appropriate borrow form rather than silently
changing capture mode.

For a conditionally available place, diagnostics/help may suggest an ownership
refinement:

```sec
if package.Payload is available {
    Use(package.Payload)
}
```

or explicit convergence when that matches the operation:

```sec
discard package.Payload
```

The LSP must explain these in programmer terms rather than requiring knowledge
of compiler-internal lattices or SSA.

## Cross-reference

`rules/tooling/diagnostics.txt` owns general diagnostic policy;
`rules/memory/ownership.md` revision 2.0 owns the semantic facts.
