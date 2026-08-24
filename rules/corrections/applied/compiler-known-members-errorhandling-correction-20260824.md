# Compiler-known members correction — Result projections

- **Status:** Applied normative correction
- **Applied:** 2026-08-24
- **Created:** 2026-08-24
- **Last updated:** 2026-08-24
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/compiler/compiler_known_members.md`

---

## Correction

Add the Sec 0.1 compiler-known Result projection surface.

For `Result[T, E]`:

```text
Ok()   -> Option[T]    consuming receiver
Err()  -> Option[E]    consuming receiver
OkRef  -> Option[ref T] shared/non-consuming property
ErrRef -> Option[ref E] shared/non-consuming property
```

### Consuming methods

Canonical source:

```sec
result.Ok()
result.Err()
```

The registry must mark the receiver as consumed.
The retained active payload is moved or copied into the returned Option according
to ordinary value semantics. No hidden clone is permitted.

The non-retained active payload is destroyed/discarded according to its ordinary
obligations. Sema must reject a projection that could silently abandon a
non-discardable active payload.

### Borrowed properties

Canonical source:

```sec
result.OkRef
result.ErrRef
```

These are read-only property-like queries because they inspect state without
mutating or consuming the receiver.

They create a shared borrow only when the selected payload is active.
The borrow cannot outlive the Result.

### Tooling registry

Expose stable compiler-known IDs, exact receiver/result types, consuming versus
shared receiver mode, effect summary, and the normative rulebook source.

LSP completion and hover must come from the same registry facts.

## Cross-reference

```text
rules/errors/errorhandling.md
rules/memory/ownership.md
rules/control-flow/discard.md
```
