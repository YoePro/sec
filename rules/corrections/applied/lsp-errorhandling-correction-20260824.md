# LSP correction — errorhandling revision 2

- **Status:** Applied normative correction
- **Applied:** 2026-08-24
- **Created:** 2026-08-24
- **Last updated:** 2026-08-24
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/tooling/lsp.md`

---

## Correction

The LSP must consume Sema's revision-2 error-handling facts and must not maintain
a parallel Result/Option/ownership model.

### Try hover

Hover at `try` and its protected operand should show:

```text
resolved protected type/carrier
success type produced by try
locally handled alternate/failure states
unhandled states that propagate
propagation target type
concrete-to-error widening when present
remaining panic effect when relevant
```

Remove any tooling claim that `Option[T]` cannot be used with `try`.
For Option, explain `Some(T)` success and `None` propagation/recovery.

### Pattern guidance

When the user writes a carrier-incompatible pattern, explain the actual resolved
carrier and offer the appropriate pattern family.

Example intent:

```text
`Find()` returns `Option[Device]`.
`Ok` belongs to `Result`, so it cannot match this value.
Use `Some(device)` or `None`.
```

### Result projections

Completion/hover must expose:

```text
.Ok()   consuming, result Option[T]
.Err()  consuming, result Option[E]
.OkRef  shared borrow, result Option[ref T]
.ErrRef shared borrow, result Option[ref E]
```

After a consuming projection, later-use diagnostics must point back to the
consume location.

### Fallible setters

Signature help and hover must show the explicitly declared setter error type from:

```sec
try set value ErrorType
```

### Open error root

Hover should distinguish:

```text
static type: error
concrete error type: IOError
```

when Sema has that control-flow fact.

## Cross-reference

```text
rules/errors/errorhandling.md
rules/tooling/diagnostics.txt
```
