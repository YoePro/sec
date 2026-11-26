# BUG-000053 — Standalone analysis required a C-binding project manifest

## Status

Fixed, 2026-11-26.

## Report and reproduction

The retained-root refactoring of `cbindingplan.Resolve` routed ordinary source
analysis through the strict explicit-workspace entry point. Checking a valid
standalone source below a directory without `.sec/sec.toml` then failed before
Sema with `c-bindings error: cannot read project manifest`.

The broader compiler regression selection reproduced this in
`TestStringMaterializationAuthority`, `TestStringEnumConversionRepairsCLI`,
`TestStringRangesCompilerCLI` and `TestStringRangeTryCompilerCLI`.

## Expected behavior

Ordinary standalone analysis has no selected C-binding plan when no project
manifest exists. An explicitly selected workspace still requires its manifest
and must fail if that required file disappears.

## Scope

- Area/phase: project selection before Sema, shared CLI/LSP binding resolution.
- Rules: `rules/tooling/cli.md` §8(6), MD-021 correction §8.7(2)–(3).
- Governance: `tooling.workspace-harness`.

## Resolution

The ordinary adapter reads the optional manifest through the retained root and
returns no plan when unavailable, preserving the previous standalone contract.
The explicit workspace path keeps its strict precondition. Both paths use the
same snapshot-based resolver when a manifest exists; containment is unchanged.

`TestResolveStandaloneVersusExplicitWorkspace` verifies both contracts. The four
compiler reproductions and the UTF-8 position/materialization test pass after
the fix. No commit was created.
