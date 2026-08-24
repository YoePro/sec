# Ownership correction — Result projections and try bindings

- **Status:** Applied normative correction
- **Applied:** 2026-08-24
- **Created:** 2026-08-24
- **Last updated:** 2026-08-24
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/memory/ownership.md`

---

## Correction

Synchronize ownership analysis with errorhandling revision 2.

### Consuming Result projections

Calling:

```sec
result.Ok()
result.Err()
```

consumes an owned `Result` receiver.
The receiver becomes unavailable exactly as after another consuming affine
operation.

A later read, borrow, move, projection, match, or discard of the same consumed
value is invalid until a separately legal reinitialization exists.

Sema must check this on every build and analysis path; it is not an optional
optimization or advisory.

### Borrowed Result projections

Reading:

```sec
result.OkRef
result.ErrRef
```

is non-consuming and creates an ordinary shared borrow of the active payload
when present.

The receiver must remain alive and sufficiently available for the borrow's
lifetime.

### Try/match payload bindings

Try handler payload bindings reuse the corresponding match ownership semantics.
For guarded move-only bindings, move commit occurs only after pattern match,
guard success, and handler selection.

### Path merge

Partial try handlers add implicit propagation paths to the control-flow graph.
Ownership availability after a try must merge every continuing success/recovery
path; propagating paths leave the scope through ordinary cleanup.

### Diagnostics

Use-after-consume diagnostics must point to the consuming projection or binding
and explain the source-level consequence in ordinary language.

## Cross-reference

```text
rules/errors/errorhandling.md
rules/memory/copy_move.md
rules/control-flow/flowcontrol_match.md
```
