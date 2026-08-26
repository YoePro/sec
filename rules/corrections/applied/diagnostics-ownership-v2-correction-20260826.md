# Correction: mentor diagnostics for ownership and moves

- **Status:** Normative correction
- **Created:** 2026-08-26
- **Last updated:** 2026-08-26
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/tooling/diagnostics.txt`

---

## Correction

Ownership diagnostics must follow the compiler's mentor principle and the
canonical semantics in `rules/memory/ownership.md` revision 2.0.

### Required explanation shape

For use-after-consume and related ownership errors, diagnostics should identify:

1. the place/value that cannot be used;
2. the earlier source operation that changed ownership;
3. the practical consequence in ordinary language;
4. a concrete correction when one is safe to suggest.

Example style:

```text
error: `resource` cannot be used here because ownership was transferred earlier

  Consume(<-resource)
          ^^^^^^^^^^ this call takes ownership of `resource`

  Use(resource)
      ^^^^^^^^ `resource` no longer contains an owned value

help: use `resource` before the consuming call, or restructure the code so it is
      not needed afterwards
```

Compiler-theory terms such as lattice joins or affine-state transitions may be
secondary details, not the primary explanation.

### Missing explicit move

When a reusable source would be silently consumed, reject the code and show the
required marker, for example:

```text
help: write `Consume(<-resource)` to transfer ownership explicitly
```

### Conditional availability

When a place may have been consumed on only some continuing paths, explain that
fact and point to the relevant source path. When appropriate, suggest
`is available` refinement or `discard` convergence.

### Runtime ownership bookkeeping policy

If a selected target/project policy rejects dynamic ownership state, report that
source semantics would require runtime ownership bookkeeping and explain how to
restructure or converge the ownership state. Do not describe this as a random
backend limitation.

### Redundant discard

Discarding a statically known already-unavailable place is legal. Tooling may
report it as redundant according to the existing diagnostic severity/policy
framework, but it is not an ownership error.

## Cross-reference

Existing diagnostic severity configuration remains unchanged by this correction.
