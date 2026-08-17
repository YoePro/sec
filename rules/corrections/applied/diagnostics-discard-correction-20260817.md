# Diagnostics correction — discard diagnostics

- **Status:** Applied normative correction
- **Applied:** 2026-08-17
- **Created:** 2026-08-16
- **Last updated:** 2026-08-16
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `56be75d`
- **Target rulebook:** `rules/tooling/diagnostics.txt`

---

## Correction

The central diagnostic registry must allocate stable IDs and symbolic names for discard-specific primary diagnostics.

The numeric ID is registry-owned and must be allocated from the existing appropriate family without reusing a published ID.

Required symbolic rules include at least:

### `ownership.implicit-discarded-result`

Classification:

```text
configurable advisory
recommended family: A3xxx
default severity: info
allowed: off, info, warning, error
```

The ID must remain unchanged when severity changes.

### `ownership.unhandled-must-use`

Classification:

```text
mandatory semantic error
recommended family: S4xxx
```

Used when a must-use value would otherwise disappear implicitly.

### `ownership.non-discardable-value`

Classification:

```text
mandatory semantic error
recommended family: S4xxx
```

The diagnostic must identify the obligation that prevents explicit discard.

### `ownership.use-after-discard`

Classification:

```text
mandatory semantic error
recommended family: S4xxx
```

The diagnostic must include a related source location for the discard that made the Place unavailable.

### `ownership.discard-while-borrowed`

Classification:

```text
mandatory semantic error
recommended family: S5xxx
```

The diagnostic must identify the conflicting borrow when known.

### `ownership.useless-explicit-discard`

Classification:

```text
configurable advisory
recommended family: A3xxx
default severity: info
```

Used for cases such as discarding a literal or constant with no observable ownership effect.

### `ownership.redundant-void-discard`

Classification:

```text
configurable advisory
recommended family: A3xxx
default severity: info
```

Used when explicit `discard` wraps an already valid `void` statement.

## Mandatory versus advisory

Must-use, ownership, borrow, and lifecycle-safety diagnostics are mandatory and cannot be disabled, demoted, or source-suppressed.

Only advisory discard diagnostics participate in `off`/`info`/`warning`/`error` project policy.

## Cross-reference

The semantic conditions for these diagnostics are defined by:

```text
rules/control-flow/discard.md
```
