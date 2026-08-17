# Semantic IR correction — resolved `match` ownership and arm values

- **Status:** Applied normative correction
- **Applied:** 2026-08-17
- **Created:** 2026-08-17
- **Last updated:** 2026-08-17
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `56be75d`
- **Target rulebook:** `rules/compiler/semantic_ir.txt`

---

## Correction

The existing ResolvedMatchPlan / explicit match-CFG model must retain the complete Sema-resolved Sec 0.1 match semantics before lowering.

### Binding actions

Resolved payload and field bindings must distinguish at least:

```text
None
Discard
Copy
Move
SharedBorrow
MutableBorrow
TemporaryForward
```

A lower stage must not infer these actions from loads, stores, unused SSA values, or physical payload representation.

### Guarded move commit

For a guarded move-only by-value payload binding, the CFG must not place the committed move on the pattern-success edge before the guard.

Required ordering:

```text
subject evaluation
pattern test
candidate binding / guard access
optional guard evaluation
    false -> next arm without committed payload move
    true  -> commit selected arm binding action
arm body
```

The committed move belongs only to the guard-success selected-arm path.

The IR must preserve enough provenance to verify that no move-only value is consumed on both a rejected guarded path and a later arm.

### Borrowed bindings

Shared and mutable pattern borrows must be explicit semantic operations or equivalent verified facts.

Their lifetimes are branch-scoped.

A borrow created for a guard whose arm is rejected must not remain active on the next arm edge.

### Shallow field binding

Shallow struct-like union field bindings must preserve resolved field identity and binding mode.

Move-only shallow fields are rejected by Sema and therefore must not appear as move field actions in valid Sec 0.1 match IR.

### Expression-match arm block value

A block in expression-match result position may contextually produce its result from its final expression on each continuing path.

Semantic IR must represent that final expression as the arm's result value.

Terminating paths have no merge edge.

This contextual rule must not be generalized into arbitrary source block-expression semantics by the IR builder.

### Arm-result ownership and cleanup

When an arm produces a move-only result, result ownership must be established before arm-scope cleanup destroys values needed to form or transfer that result.

The merge block must receive the correctly owned arm result.

### Statement match and discard

A statement-match arm result must be resolved through the normal discard and must-use rules before Semantic IR.

Where implicit call-result discard is legal, IR must represent the terminal discard semantics rather than merely leave an unused SSA result.

Must-use violations do not lower as valid IR.

### Post-arm ownership merge

Match CFG metadata or ownership operations must preserve the Sema-resolved availability state of reusable Places across every continuing arm and at the merge.

Lowering must not reconstruct whether a subject is `Moved`, `PartiallyAvailable`, or `ConditionallyAvailable` from backend representation.

### Exhaustive residual

Compiler-proven exhaustive residual paths may continue to lower to synthesized unreachable control flow.

The proof must include reachable union `empty` state and open bit-enum domain semantics where applicable.

## Cross-reference

Canonical source semantics are defined by:

```text
rules/control-flow/flowcontrol_match.md
```
