# Destruction correction — discard terminal ownership

- **Status:** Applied normative correction
- **Applied:** 2026-08-17
- **Created:** 2026-08-16
- **Last updated:** 2026-08-16
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `56be75d`
- **Target rulebook:** `rules/memory/destruction.txt`

---

## Correction

Explicit and implicit discard use the same resolved deterministic destruction machinery as normal lifetime end.

For an owned non-trivial value, successful discard must:

1. execute the resolved destruction plan;
2. consume the current value;
3. cancel any pending automatic cleanup responsibility for that consumed value;
4. mark the consumed place unavailable when applicable;
5. prevent later destruction of the already consumed value.

A later legal reinitialization of a mutable binding creates a new destruction responsibility at the reinitialization point.

Discard must never cause double destruction.

### Aggregate destruction under discard

Discard of a complete aggregate destroys only still-initialized, still-owned components according to ordinary destruction order.

```text
struct
    destroy still-initialized owned fields

fixed array
    destroy still-initialized elements

union
    destroy only the active initialized unmoved payload

Result[T,E]
    destroy only the active Ok or Err payload

Option[T]
    destroy Some payload when active; None has no payload
```

Moved or previously discarded components are skipped.

### References and borrowed views

Discarding a reference, mutable reference, or borrowed slice/view does not destroy the referent or backing elements.

Ending the reference holder may end its borrow lifetime according to borrowing analysis.

### Implicit call-result discard

A discardable non-`void` temporary returned from an ordinary standalone call is destroyed before the next statement just as though its terminal ownership action had been made explicit.

This is semantic destruction, not merely omission of an unused backend value.

## Cross-reference

Detailed legality and must-use rules are defined by:

```text
rules/control-flow/discard.md
```
