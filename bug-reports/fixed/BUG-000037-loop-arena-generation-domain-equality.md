# BUG-000037 — Loop Arena generation equality ignores zero-valued domain identity

## Status

Fixed 2026-11-21.

## Reproduction

`TestLoopArenaGenerationEdges` compares generation maps `{first: 0}` and
`{second: 0}`. Before the fix, `arenaGenerationStatesEqual` returned true:
map lengths matched and a missing Go map key read as zero. The new regression
failed with "different ArenaDomains compared equal" before the correction.

## Expected and actual behavior

Loop-header change detection compares both canonical domain keys and their
generations. Equal epoch numbers on different domains are not the same state.
The old helper omitted key-presence checking. This is a directly reproduced
helper defect; no source program accepting an invalid reference as a result
of this particular key collision has been established.

## Scope

- Phase: Sema loop-header change detection.
- Governance: `frontend.loop-arena-generation-fixed-point`.
- Rules: `rules/memory/arena.md` §§4.5,38;
  `rules/control-flow/flowcontrol_while.md` §§28–29;
  `rules/control-flow/flowcontrol_for.md` §§30–31,42.

## Resolution and verification

Require key presence before comparing generation values. Focused race tests
cover domain identity, detached fallthrough/continue joins, break exclusion,
and source-backed stale reads on later while/for body and condition evaluation.
Valid break-only and freshly allocated per-iteration controls pass. A temporary
Go overlay that restores entry epochs instead of merged header epochs removes
all five stale-use diagnostics, confirming sensitivity of the source tests to
replay. Runtime Arena epoch representation and lowering are outside this fix.
