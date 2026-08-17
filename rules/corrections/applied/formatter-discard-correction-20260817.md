# Formatter correction — discard formatting

- **Status:** Applied normative correction
- **Applied:** 2026-08-17
- **Created:** 2026-08-16
- **Last updated:** 2026-08-16
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `56be75d`
- **Target rulebook:** `rules/tooling/formatter.md`

---

## Correction

Canonical explicit discard formatting is:

```sec
discard expression
```

The formatter must:

- preserve the `discard` keyword when it is present in source;
- format the operand using ordinary expression formatting;
- preserve evaluation order and ownership semantics;
- keep `discard` as a statement rather than formatting it as a function call;
- reject no otherwise valid file merely because the implicit-discard advisory is configured differently.

Ordinary formatting must not automatically transform:

```sec
Calculate()
```

into:

```sec
discard Calculate()
```

or remove explicit `discard` from the reverse form.

Reason: explicit discard documents programmer acknowledgement and may satisfy a project diagnostic policy or must-use rule. Insertion/removal therefore changes semantic/documentation intent even when runtime machine behavior would otherwise be equivalent.

A separately requested semantic code action may offer explicit discard when analysis proves the transformation valid.

## Cross-reference

Source semantics are defined by:

```text
rules/control-flow/discard.md
```
