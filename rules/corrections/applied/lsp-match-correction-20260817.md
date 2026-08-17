# LSP correction — interactive `match` ownership visibility

- **Status:** Applied normative correction
- **Applied:** 2026-08-17
- **Created:** 2026-08-17
- **Last updated:** 2026-08-17
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `56be75d`
- **Target rulebook:** `rules/tooling/lsp.md`

---

## Correction

The language server must expose compiler-resolved `match` ownership, borrowing, availability, and exhaustiveness facts during normal interactive editing.

The LSP must not create a parallel match or ownership analyzer.

It consumes the same Sema/ownership facts used by batch compilation.

### Pattern-binding hover

Hover on a payload binding must be able to show its resolved action.

Examples:

```text
Type: Resource
Binding mode: move
Source: option.Some payload
Commit: when this arm is selected
```

```text
Type: Header
Binding mode: copy
Source: packet.Header payload
```

```text
Type: ref Resource
Binding mode: shared borrow
```

```text
Type: ref mut Resource
Binding mode: mutable borrow
```

### Guarded move visibility

For a move-only by-value binding in a guarded arm, the LSP must distinguish:

```text
prospective move
```

from:

```text
committed move
```

Pattern success alone must not make the subject appear moved.

Before guard success, tooling may describe the effect as:

```text
Ownership effect: moves payload if this arm is selected
```

After Sema resolves a selected-path effect for diagnostics/dataflow, the move location is the guard-success arm-selection commit point or the associated binding source location retained by compiler provenance.

### Post-match availability

Hover and diagnostics after a match must expose the compiler's merged availability state.

Examples include:

```text
State: Moved
```

```text
State: Partially available
```

```text
State: Conditionally available
Reason: moved in Some arm, retained in None arm
```

A later invalid use is a mandatory language-safety diagnostic.

It is not configurable advisory feedback.

### Related locations

Where available, ownership diagnostics should link:

- the later invalid use;
- the arm that moved the value;
- the binding that received ownership;
- the affected payload or subject Place;
- the post-match merge reason.

### Inlay hints

Configurable ownership hints may show:

```text
copy
move
ref
ref mut
moves if selected
```

for match bindings when the effect is not obvious.

Disabling an inlay hint never disables the underlying ownership analysis or diagnostics.

### Exhaustiveness and `empty`

Match diagnostics and generated-match actions must use the compiler's resolved coverage domain, including:

- unique ordinary enum numeric value classes;
- complete open bit-enum domains;
- reachable union `empty` state;
- guarded residual coverage.

The LSP may suggest a final `_` for uncovered open hardware encodings.

When `empty` is reachable, tooling should distinguish it from a declared union variant.

### Completion and refactoring

Completion and code actions must not recommend a payload use that is invalid under the resolved ownership state.

Generate-exhaustive-match actions must use canonical Sec 0.1 patterns and must not generate direct bool, general literal, range, nested recursive, or ordinary struct-subject patterns.

## Cross-reference

Canonical semantics are defined by:

```text
rules/control-flow/flowcontrol_match.md
rules/memory/ownership.md
```
