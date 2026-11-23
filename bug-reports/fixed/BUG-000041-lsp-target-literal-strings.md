# BUG-000041 — LSP rejects TOML literal strings in target facts

## Status

Fixed, 2026-11-23.

## Report

The LSP's target reader used Go string unquoting for TOML values. Valid literal
strings such as `source='cmd/worker'`, `variants=['linux']`, `os='linux'` and
`arch='amd64'` prevented resolution of the project's scalar plan. Sema then
reported S1258 on `BuildInfo.Current()` and could not infer fields of its result
for hover and completion. Explicitly typed BuildInfo parameters still worked.

## Reproduction

`TestLSPBuildInfoMembers` supplies a temporary project with those literal-string
Target/Variant values and opens `testdata/tooling/build_info/members.sec` in it.
Before the fix, the fixture's resolved-snapshot assertion failed with S1258;
hover over the inferred `info.VersionBuild` was also absent. Changing just the
manifest quotes to basic strings made the test pass.

## Expected and actual behavior

Both TOML basic and literal strings must provide the same declared target
facts. Editor features must consume that selected target's metadata, rather
than losing the plan because the manifest uses legal TOML spelling.

## Scope

- Area: LSP target plan resolution and BuildInfo presentation.
- Rulebooks: `rules/projects/projects.md` §5(2), §§17,19;
  `rules/compiler/compiler_known_members.md` §113B;
  `rules/tooling/lsp.md` Target-aware analysis and §4.1(5–7).
- Governance: `tooling.project-version-model`.
- Phase: LSP target selection before Sema.

## Resolution

The shared target-string reader uses the existing TOML decoder with a typed
string destination, including each string in the variant-name array. It no
longer applies Go literal rules. The source-backed regression verifies all 16
field types, exact hover ranges, static/instance completion separation,
Current's signature and availability, and an inferred Current result. Field
completion documentation reuses the existing hover renderer.
