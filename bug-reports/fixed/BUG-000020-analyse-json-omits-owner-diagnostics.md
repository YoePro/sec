# BUG-000020 — sec analyse JSON omits selected owner diagnostics

## Status

Fixed, 2026-11-03.

## Report

Successful `sec analyse --json` output counted registered Sema advisories in its
summary but omitted their emitted diagnostic occurrences. In particular,
`sec analyse parameter-usage --json` could report two advisories with empty
`diagnostics` and `findings` arrays. The text presentation did print both
parameter-usage recommendations.

## Reproduction

Use `testdata/tooling/analyse_cli/mixed_families.sec`:

```sh
go run ./cmd/compiler analyse parameter-usage --json testdata/tooling/analyse_cli/mixed_families.sec
```

Before the fix the regression's all-analysis report had `summary.advisory = 3`,
one pitfall finding, and zero diagnostics despite two A2001 recommendations.
The new process regression reproduced this before the production change.

## Expected and actual behavior

JSON must retain selected canonical diagnostics with their stable IDs,
severities, source locations and repair information. The parameter-usage-only
report now contains the two A2001 occurrences; pitfalls-only output contains
only its optional finding. A focused recommendation contains one occurrence,
and mandatory errors still reject either selector.

## Scope and fix

`runAnalyseCommand` deferred successful Sema warning output to `writeHuman`.
The JSON branch skipped that method, so the canonical reporter never received
those warnings. The shared `emitSelectedDiagnostics` helper now feeds the
already family/focus-filtered warnings to the reporter in both modes, once per
run. JSON still uses the existing occurrence format and version; no parallel
warning serialization or semantic analysis was introduced.

## Rulebooks and ownership

- `rules/tooling/cli.md` §§5.2, 5.4, 6(2–4).
- `rules/tooling/diagnostics.md` §14.
- `rules/corrections/applied/md017-sec-analyse-cli-and-help-correction-20261017.md` §§6, 10(4).
- Governance owner: `tooling.cli`; individual selector acceptance also closes
  the existing `sema.pitfall-analysis` and `sema.parameter-usage-analysis` items.

## Validation

`TestAnalyseIndividualFamilies` exercises the actual CLI process with a mixed
source: human/verbose/JSON family isolation, positive A2001 diagnostics, exact
locations, canonical severities, transitive focus, silent clean input and
mandatory rejected input. `TestAnalyseIndividualFamilyNames` checks canonical
selection and rejection of aliases/conflicting `--all` selection.

Verification passed on 2026-11-03: normal new-selector regressions (3.391s),
targeted selector/family/JSON/presentation tests with the race detector
(56.837s), and `go vet ./cmd/compiler`.
