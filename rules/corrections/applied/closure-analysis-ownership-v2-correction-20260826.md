# Correction: closure capture transfer classification

- **Status:** Normative correction
- **Created:** 2026-08-26
- **Last updated:** 2026-08-26
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/analysis/closure_analysis.md`

---

## Correction

Synchronize closure-analysis facts with the explicit capture model in
`rules/memory/ownership.md` revision 2.0.

### Canonical capture classification

The analysis must distinguish at least these source-level capture intentions:

```text
CopyCapture       capture(value)
MoveCapture       capture(<-value)
SharedBorrow      capture(ref value)
MutableBorrow     capture(ref mut value)
```

Any old `ForcedConsume` representation derived from `capture(-> value)` must be
migrated to the canonical `capture(<-value)` syntax/fact.

### Plain capture never silently becomes move capture

A plain owned capture is a copy request. Closure analysis must not reclassify
`capture(value)` as a destructive move merely because the captured type is
non-copyable. That case is a semantic error with a suggested repair such as:

```sec
capture(<-value)
```

or an appropriate borrow capture.

### Transfer point

For an explicit move capture, the outer source becomes unavailable only when
closure-environment construction successfully commits the ownership transfer.
The resulting environment owns the captured value and is responsible for its
later destruction or transfer.

### Diagnostics and LSP

Expose the explicit capture kind and the source availability consequence as
canonical compiler facts. Do not infer user-visible ownership intent from
backend capture layout.

## Cross-reference

`rules/declarations/lambda-functions.md` owns capture surface syntax;
`rules/memory/ownership.md` revision 2.0 owns availability and transfer rules.
