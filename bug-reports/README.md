# Bug reports

This directory records reproducible implementation defects separately from
`missing-decisions.yaml`. A missing decision is a gap in the normative design;
a bug report is a violation of an already-established rule, contract, or
implementation expectation.

`index.yaml` is the canonical ledger. It tracks every report from intake to
fix and names the report file, affected area, rulebooks, governance owner,
reproduction evidence, and fix evidence.

## Lifecycle

1. Create `open/BUG-000001-short-title.md` for an incoming report.
2. Add one matching entry to `index.yaml` with status `open`.
3. Reproduce and triage the fault against the applicable rulebooks and
   corrections. Update the ledger entry with the owning area, rulebooks, and
   governance integration.
4. When the fix has appropriate validation evidence, set the ledger entry to
   `fixed` and move the report to `fixed/` in the same change.

An imported audit may contain several independently actionable findings. Keep
the audit file intact under `open/` and create one ledger entry per finding.
Ledger IDs are the fixed-width, monotonically assigned `BUG-000001` form;
preserve the audit's own stable finding ID in `source_id`. Imported filenames
do not need to be renamed to the `BUG-000001` convention.

Do not close a report merely because parsing succeeds: record the actual
affected phase, such as semantic analysis, Semantic IR, lowering, runtime, or
tooling. A bug that exposes an unresolved normative choice must also reference
the applicable `MD-NNN` entry; the bug report itself does not decide language
semantics.

## Report file template

```md
# BUG-000001 — Short title

## Status

Open

## Report

Describe the observed behavior and why it violates the cited contract.

## Reproduction

Minimal source, command, fixture, or deterministic steps.

## Expected and actual behavior

State both outcomes precisely, including diagnostics or artifact behavior.

## Scope

- Area:
- Rulebooks and corrections:
- Governance integration:
- Affected phase:

## Resolution

Fill in only when fixed: implementation change, validation evidence, and a
commit reference when one is available. A documentation-only correction needs
cross-rulebook consistency evidence; an executable regression is required when
compiler behavior changes.
```
