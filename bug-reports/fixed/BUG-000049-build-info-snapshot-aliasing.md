# BUG-000049 — BuildInfo snapshot aliases optional compiler inputs

## Status

Fixed, 2026-11-23.

## Reproduction and rule

Configure an analyzer with optional version/source facts, then mutate the
producer's strings or boolean. Previously BuildInfo and fresh CTE analyzers
observed the mutations. The new TestBuildInfoSnapshotOwnership failed on the
original implementation before the fix. Public snapshot getters also exposed
the same pointers.

`rules/compiler/compiler_known_members.md` §113B(1–4) requires a single,
internally consistent snapshot of explicit plan inputs. The optional fields
are defined by `rules/library/core-library.md` §28(3).
Owner: `tooling.project-version-model`.

## Resolution and verification

Detach all three optional values when configuring Sema, capture the detached
snapshot for configuration replay, and detach public BuildInfo and
BuildInfoCurrentOf results. Producer inputs and read-only consumers can no
longer change configured semantic CTE inputs through pointer aliases.

The platform build-metadata resolver separately accepts requested, already
resolved source facts and copies only requested-and-available values.
Tests cover the request/availability matrix, false and empty present values,
source-producer mutation, ordinary Current access, semantic CTE and replay.
No Git collection, source inspection, new configuration syntax or cache policy
is introduced by this fix.
