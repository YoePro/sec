# BUG-000029 — A raw spawn Result can hide an unresolved child at scope exit

## Status

Fixed and verified on 2026-11-18; reproduced during thread creation Result integration the same day.

## Report

A named `Result[Task[T], TaskSpawnError]` or
`Result[Thread[T], ThreadSpawnError]` can leave scope unused even though a
successful creation would leave an unresolved owned child. Direct Task/Thread
bindings are checked; the creation Result wrapper is not traversed by
`checkUnresolvedTaskAtScopeExit`.

## Reproduction

```sh
sec check testdata/sema/raw_spawn_result_scope_bug.sec
```

The fixture binds raw task and thread creation results and lets both leave
scope. Original behavior: exit 0, no diagnostics. Fixed behavior: exit 3, two
S1247/ownership.unresolved-creation-result errors with source ranges and mentor
help, one for each potentially unresolved child ownership obligation.

Explicit discard of either creation Result is already rejected. Successful
`try` extraction followed by join/await/detach, forwarding a creation Result,
and exhaustive `match` handling are separate valid controls.

## Scope

- Area: ownership/error-handling lifecycle integration.
- Rulebooks: `rules/concurrency/tasks.md` §§4–5,30;
  `rules/concurrency/threads.md` §§2,34;
  `rules/control-flow/discard.md` §9;
  `rules/memory/ownership.md`.
- Governance: `frontend.ownership-v2`, existing
  `ownership-errorhandling-sync` remaining work in `governance/lifetime.yaml`.
- Affected phase: Sema. Native spawn lowering is still unavailable; this
  finding does not claim an executed child leak.

## Resolution

Fixed in Sema and transported through compiler human/JSON output and LSP.
Result match payload extraction now commits the same canonical Place move as
union payload extraction. Arm-local variant facts and definite payload moves
resolve the carrier's child obligation through the existing lifecycle flow
snapshots. Continuing paths intersect those facts; a conditional move alone
cannot hide a live child. Mutation authority and reinitialization invalidate
the discharged carrier fact. No synthetic whole-carrier move is introduced.

Checks cover lexical scope exit, owning parameter fallthrough, explicit early
return and implicit try propagation. Match payload owners are checked at arm
exit as well. Failed creation owns no child, including mutable Err-only
storage; forwarding, try extraction, exhaustive match and loop failure breaks
remain valid. The implementation reuses canonical variant, availability and
lifecycle facts rather than recognizing source spellings of spawn.

Validation:

- Complete internal/sema, internal/ir/semantic and internal/diagnostics suites.
- Focused Sema race tests for creation ownership, projections, matches and
  existing state-correlation regressions.
- Compiler/LSP race tests for creation ownership, raw task/thread Result
  boundaries and failure-domain diagnostics; invalid emission creates no IR.
- Original reproduction now fails with two registered S1247 diagnostics.
- Positive/negative controls: testdata/sema/creation_lifecycle_{valid,invalid}.sec.

Native spawn lowering/runtime, general recursive aggregate availability and
replacement policy remain separate governance work. No runtime leak claim or
new language policy is inferred from this validation fix.
