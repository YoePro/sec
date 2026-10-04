# `analysis/pitfall_analysis.md` — Revision 2.0 synchronization

- **Status:** Normative synchronization patch
- **Created:** 2026-10-04
- **Target:** `rules/analysis/pitfall_analysis.md`
- **Target revision after application:** 2.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `main-reviewed-2026-10-04`
- **Implementation governance:** `governance/analysis.yaml` (`sema.pitfall-analysis`)
- **Purpose:** Synchronize the existing canonical pitfall-analysis rulebook without rewriting its already-current semantic model.

---

## 1. Replace the document header

Replace the current opening:

```md
# Pitfall Analysis
## Status

Normative compiler-analysis rulebook for Sec 0.1.

This rulebook defines the semantic purpose, finding model, confidence model,
evidence and suppression rules, initial pitfall catalog, diagnostic integration,
corrective-action requirements, analysis-budget behavior, tooling integration,
incremental behavior, tests, and completion criteria for Sec pitfall analysis.
Mutable implementation status does not belong in this rulebook. It is governed
by the repository-level `implementation-status.yaml` ledger.
```

with:

```md
# Pitfall Analysis

- **Status:** Normative
- **Created:** 2026-08-08
- **Last updated:** 2026-10-04
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/analysis/pitfall_analysis.md`
- **Replaces:** Earlier unversioned revision at the same canonical path
- **Repository baseline reviewed:** `main-reviewed-2026-10-04`
- **Implementation governance:** `governance/analysis.yaml` (`sema.pitfall-analysis`)
- **Related rulebooks:** `rules/analysis/effect_analysis.md`, `rules/analysis/escape_analysis.md`, `rules/analysis/parameter_usage_analysis.md`, `rules/analysis/call_graph.md`, `rules/analysis/data_races.md`, `rules/analysis/deadlock_analysis.md`, `rules/foundations/operators.md`, `rules/types/contracts.md`, `rules/collections/collections.md`, `rules/collections/shaped-types.md`, `rules/platform/ffi.md`, `rules/tooling/diagnostics.md`, `rules/tooling/lsp.md`, `rules/projects/projects.md`, `rules/compiler/compiler_analysis.md`, `rules/compiler/incremental_compilation.md`

---

## Status and authority

This is the normative compiler-analysis rulebook for Sec 0.1 pitfall analysis.

This rulebook defines the semantic purpose, finding model, confidence model,
evidence and suppression rules, initial pitfall catalog, diagnostic integration,
corrective-action requirements, analysis-budget behavior, tooling integration,
incremental behavior, tests, and completion criteria for Sec pitfall analysis.

Mutable implementation status does not belong in this rulebook. It is governed
by `sema.pitfall-analysis` in `governance/analysis.yaml`.
```

No existing semantic section after this point needs to be renumbered merely for
this synchronization revision.

---

## 2. Synchronize logical operator syntax

Sec's canonical logical operators are `&&` and `||`.

Replace this example:

```sec
if value >= 0 or value <= 10 {
    ...
}
```

with:

```sec
if value >= 0 || value <= 10 {
    ...
}
```

Replace the accompanying prose:

```text
rather than merely suggesting replacement of `or` with `and`.
```

with:

```text
rather than merely suggesting replacement of `||` with `&&`.
```

Replace this example:

```sec
if value >= 0 and value <= 10 {
    ...
}
```

with:

```sec
if value >= 0 && value <= 10 {
    ...
}
```

Replace this example:

```sec
GetValue() >= 0 and GetValue() <= 10
```

with:

```sec
GetValue() >= 0 && GetValue() <= 10
```

These are syntax synchronizations only. The pitfall-analysis semantics remain
unchanged.

---

## 3. Synchronize diagnostic governance

In the section currently headed:

```md
# Diagnostic ownership and coalescing
```

retain the semantic priority model, but replace:

```text
The repository-wide diagnostic governance determines concrete diagnostic IDs
and configurable severities.
```

with:

```text
`rules/tooling/diagnostics.md` defines diagnostic identity, severity,
configurability, structured occurrences, fixes, and CLI/LSP transport.

Pitfall rule identity is analysis identity. It must not be substituted for a
registered compiler diagnostic ID.
```

Keep the existing rule:

```text
Pitfall rule identities do not encode the current severity.
```

---

## 4. Keep internal pitfall rule identities distinct from diagnostic IDs

The existing section:

```md
# Stable pitfall rule identities
```

remains normative.

Clarify the paragraph following the conceptual examples to read:

```text
These are stable pitfall-analysis rule identities used for configuration,
testing, analysis dumps, profiling, dependency tracking, and tooling.

They are not compiler diagnostic IDs.

When a pitfall occurrence owns a user-facing advisory diagnostic, that
diagnostic must use a separately registered stable diagnostic ID from
`rules/tooling/diagnostics.md`.

When the underlying condition is a proven violation owned by another semantic
analysis, that owning analysis supplies the primary diagnostic identity.
```

This prevents Codex from registering strings such as
`pitfall.bounds.inclusive-length-index` directly as compiler diagnostic IDs.

---

## 5. Synchronize project diagnostic-policy ownership

Where the rulebook states that project diagnostic policy controls optional
pitfall findings, retain that behavior but add:

```text
Project-file syntax and configuration precedence are owned by
`rules/projects/projects.md`.

Pitfall analysis consumes the resolved diagnostic policy. It does not define a
second project-configuration syntax.
```

This does not change the Interactive, Standard, or Deep analysis model.

---

## 6. Replace the governance section

Replace the existing `# Governance` section with:

```md
# Governance

This rulebook contains normative pitfall-analysis behavior only.

Mutable implementation progress belongs to:

```text
governance/analysis.yaml
```

under the canonical integration ID:

```text
sema.pitfall-analysis
```

That governance integration owns implementation tracking for at least:

```text
finding model
range/bounds catalog
collection relationships
control-flow relationships
FFI relationships
evidence and suppression
corrective actions
incremental LSP integration
Deep analysis
```

Implementation claims, current bug lists, implementation percentages, and
temporary compiler limitations must not be copied into this rulebook.

Cross-area implementation work remains owned by its existing governance
integration. In particular:

```text
diagnostic transport
    governance/errors_diagnostics.yaml

LSP presentation
    governance/tooling_lsp.yaml

FFI semantic support
    governance/ffi.yaml

compiler analysis coordination
    owning compiler/analysis governance integration
```

The pitfall governance entry may depend on those integrations but must not
duplicate their implementation state.
```

---

## 7. Update the completion criterion

Replace completion criterion 19:

```text
19. mutable implementation status remains governed by
    `implementation-status.yaml`.
```

with:

```text
19. mutable implementation status remains governed by
    `sema.pitfall-analysis` in `governance/analysis.yaml`.
```

---

## 8. Update the final normative summary

Replace the final sentence:

```text
Mutable implementation progress is governed by `implementation-status.yaml`.
```

with:

```text
Mutable implementation progress is governed by `sema.pitfall-analysis` in
`governance/analysis.yaml`.
```

---

## 9. References to preserve

Do not change the following semantics during this revision:

- `PitfallFinding` remains the structured analysis result.
- `ProvenInvalid`, `LikelyMistake`, and `SuspiciousIntent` remain distinct.
- `Proven`, `High`, and `Medium` confidence remain distinct from classification.
- `Finding`, `NoFinding`, `Suppressed`, `NotEvaluated`, and `Pending` remain distinct.
- Proven normative invalidity remains owned by the underlying semantic rule.
- Pitfall analysis must not downgrade a normative error.
- Duplicate root-cause diagnostics remain coalesced.
- Suppressing evidence may suppress heuristic findings but never normative errors.
- FFI pointer/extent analysis requires canonical foreign-contract metadata and
  must not guess relationships from parameter names.
- `ProvenFix` remains distinct from heuristic `SuggestedEdit`.
- Fix safety must preserve evaluation count/order, effects, panic/error behavior,
  ownership, borrowing, and relevant control flow.
- Interactive, Standard, and Deep remain analysis-budget modes, not different
  language semantics.
- Missing prerequisite facts produce `NotEvaluated` or `Pending`, never an
  unsound `NoFinding`.
- Incremental invalidation remains dependency-driven.
- CLI analysis and LSP consume the same underlying structured pitfall model.

---

## 10. Governance synchronization

No new governance integration is created by this revision.

Update the existing `sema.pitfall-analysis` entry in
`governance/analysis.yaml` so that its canonical rulebook metadata points to:

```text
rules/analysis/pitfall_analysis.md
Document revision: 2.0
Repository baseline: main-reviewed-2026-10-04
```

Do not move implementation claims owned by diagnostics, FFI, LSP, compiler
analysis, or incremental compilation into the pitfall entry merely because
pitfall analysis consumes those facts.

---

## 11. Revision note

Revision 2.0 is a synchronization revision.

It does not redesign the pitfall-analysis model.

It:

1. moves implementation-status ownership from the legacy root ledger reference
   to `governance/analysis.yaml`;
2. synchronizes Sec logical operator examples from obsolete `and` / `or` spelling
   to canonical `&&` / `||`;
3. synchronizes user-facing diagnostic ownership with the revision-2 diagnostics
   model;
4. makes project-configuration ownership explicit;
5. preserves the existing finding, evidence, confidence, suppression, fix,
   budget, incremental, LSP, and FFI semantics.
