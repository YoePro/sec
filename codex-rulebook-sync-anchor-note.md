# Codex note — rulebook synchronization anchor

**Repository baseline reviewed:** `e0af215` (`feat(frontend): unify compiler-known collections and synchronize rulebooks`)
**Date:** 2026-08-11

Create and maintain a synchronization anchor for the separate rulebook-sync workstream.

The synchronization workstream owns updates to existing rulebooks marked `Written — sync required`, legacy `.txt` rulebooks, umbrella documents, cross-reference corrections, and migration of stale implementation notes into `implementation-status.yaml`.

The new-rulebook workstream is separate. Do not rewrite, merge, or reinterpret newly authored rulebooks merely to satisfy the sync workstream unless an actual contradiction is found.

When synchronizing against repository state:

- treat `rules/` as normative;
- treat `implementation-status.yaml` as the canonical mutable implementation ledger;
- never move implementation-progress claims back into normative rulebooks;
- preserve local/unsynced work and merge incrementally rather than replacing whole files;
- use `e0af215` as the current repository anchor until a newer `main` HEAD is explicitly reviewed;
- if `main` still equals the recorded anchor, do not re-read unchanged repository material;
- if `main` has advanced, inspect only the changed files relevant to the synchronization task before updating the anchor.

`main` still equals the reviewed `e0af215` anchor, so the repository-sync
workstream does not need to re-read unchanged baseline material.

The separate new-rulebook workstream has now processed the locally authored
`rules/isr_analysis.md` without advancing the Git anchor:

- `language-rulebook-status.md` lists it as **Written**;
- `implementation-status.yaml` contains the planned `sema.isr-analysis`
  integration and its complete implementation backlog;
- the one-use `implementation-status-isr-analysis-merge.yaml` fragment has
  been removed after merge.

These local updates remain outside the reviewed `main` baseline until they are
committed. A future rulebook-sync pass should therefore preserve them and only
inspect repository changes after `e0af215` that are relevant to its own scope.
