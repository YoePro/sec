# MD-017 — `sec analyse` CLI, reporting and compiler help

**Status:** Applied 2026-10-17 — new owning rulebook `rules/tooling/cli.md`; `rules/compiler/compiler_analysis.md` §§ 61(4), 62(5); CLI spellings synchronized in `pitfall_analysis.md`, `parameter_usage_analysis.md`, `isr_analysis.md`, `data_races.md`, `deadlock_analysis.md`, `stack_analysis.md` and `closure_analysis.md`. Implemented in `cmd/compiler` (command registry and help, analysis-name registry, `-f/--function`, `-d/--depth`, `--info`, `--verbose`, `--json` with `sec.analysis-report` v1); tests `cmd/compiler/analyse_cli_test.go`. Governance `tooling.cli`. MD-017 removed from `missing-decisions.yaml`. Known holes (verified 2026-10-17): isr, deadlocks and data-races cannot run individually yet (selecting them fails with an explanation); § 11(4)–(5) recursion/specialization/open-target and § 11(11) cold/incremental/parallel tests remain.  
**Created:** 2026-10-17  
**Updated:** 2026-10-17  
**Revision:** 1.0  
**Sec version:** 0.1  
**Canonical path:** `rules/corrections/applied/md017-sec-analyse-cli-and-help-correction-20261017.md`  
**Replaces:** None. Resolves the normative decisions recorded by MD-017; does not imply that MD-017 may be removed before rulebook/governance synchronization.  
**Decision owner:** Sec language design  
**Affected normative rulebooks:** `rules/compiler/compiler_analysis.md`, `rules/analysis/pitfall_analysis.md`, `rules/analysis/parameter_usage_analysis.md`, `rules/analysis/isr_analysis.md`, `rules/analysis/data_races.md`, `rules/analysis/deadlock_analysis.md`, and the canonical CLI/tooling rulebook to be established for global command syntax and help.  
**Affected surfaces:** `sec analyse`, `sec help`, CLI argument parsing, human and JSON reporting, analysis registry, diagnostics, tests, and governance.

---

## § 1. Scope and normative separation

**§ 1(1)** This correction defines the user-facing selection, analysis focus, reporting, analysis budget, machine-readable reporting, and help behavior of the Sec command-line interface.

**§ 1(2)** It does not redefine any analysis's safety proof, semantic fact ownership, supported target/profile contract, or diagnostic identity. `compiler_analysis.md` remains authoritative for those semantics.

**§ 1(3)** Reporting verbosity and analysis depth are independent. Less output does not mean less analysis. A smaller advisory analysis budget does not establish safety that an omitted proof would otherwise have established.

**§ 1(4)** CLI spelling is specified here as the canonical contract. An implementation may retain migration aliases only if the CLI owner records them explicitly; an undocumented synonym must not silently become normative.

## § 2. Command forms and analysis names

**§ 2(1)** The following command forms are canonical:

```text
sec analyse
sec analyse --all
sec analyse <analysis-name>
sec analyse [<analysis-name>] --function <callable-selector>
sec analyse [<analysis-name>] -f <callable-selector>
sec analyse [<analysis-name>] --depth interactive|standard|deep
sec analyse [<analysis-name>] -d interactive|standard|deep
sec analyse [<analysis-name>] --info
sec analyse [<analysis-name>] --verbose
sec analyse [<analysis-name>] --json

sec help
sec help <command>
sec --help
sec <command> --help
```

**§ 2(2)** `sec analyse` and `sec analyse --all` request every available registered analysis for the selected compilation context. `--all` is an explicit spelling of the default, not a more expensive mode.

**§ 2(3)** A positional `<analysis-name>` selects a single analysis family. The canonical names agreed for existing individually described analysis families are:

| CLI name | Owning analysis family |
| --- | --- |
| `isr` | ISR analysis |
| `deadlocks` | Deadlock analysis |
| `data-races` | Data race analysis |
| `pitfalls` | Pitfall analysis |
| `parameter-usage` | Parameter usage analysis |
| `stack` | Stack analysis |

**§ 2(4)** These names are lowercase ASCII; multiword names use hyphens. No alternate pluralizations, shortened aliases or flag-based individual-analysis selector are introduced by this correction.

**§ 2(5)** Other analysis families that already exist or are introduced later require one uniquely registered canonical name before individual CLI selection is exposed. Their names must be drawn from the single analysis registry and reflected consistently in the parser, help, JSON and tests. The table in § 2(3) must not be interpreted as a complete inventory of every compiler analysis fact producer.

**§ 2(6)** Unknown names and malformed selectors produce a concise explanatory diagnostic, show available names or nearest unambiguous choices when possible, and point to `sec help analyse`. They must never silently trigger `--all`.

## § 3. Function-focused analysis

**§ 3(1)** `--function` and `-f` are exact synonyms. They select a callable *analysis root*, not a textual output filter.

**§ 3(2)** The requested analysis includes the selected callable and its transitively reachable call targets, direct and indirect as required by the canonical call graph and callable contracts. Recursive cycles use the existing interprocedural fixed-point/convergence rules.

**§ 3(3)** An analysis may also require facts about callers, globally reachable resources, shared state, concurrency peers, task/interrupt contexts, target profiles or FFI contracts to decide an issue arising from the chosen root. Function focus must not omit those dependencies or misrepresent a partial result as complete. It must, however, avoid unrelated work when safe dependency pruning is provable.

**§ 3(4)** Findings originating in a callee may be reported if they affect the selected root. The report must preserve the relevant call path or dependency chain so the programmer can see why the finding belongs to the focused analysis.

**§ 3(5)** A callable selector resolves to exactly one canonical callable identity, including applicable module, overload, specialization and receiver context. A unique short source name such as `myFunc` is accepted. An ambiguous short name must be rejected with concrete disambiguation guidance. No overload or specialization is chosen by guesswork.

**§ 3(6)** Closed indirect target sets are traversed precisely. Open call targets must use their declared contracts with conservative unknown handling; neither foreign callbacks nor dynamic calls may be assumed absent because a target body is unavailable.

**§ 3(7)** Selection does not weaken required compilation and profile proofs. User-visible reports explain when a dependency remains unresolved, unavailable or only contractually trusted.

## § 4. Depth and proof completeness

**§ 4(1)** `--depth` and `-d` are synonymous. Their permitted values are `interactive`, `standard` and `deep`.

**§ 4(2)** The default for `sec analyse` is `deep`, retaining the existing command behavior. An explicit `--depth deep` is equivalent to the default.

**§ 4(3)** Reduced depths may limit expensive advisory refinement and cross-procedure exploration as permitted by `compiler_analysis.md` § 62. They must not suppress mandatory errors, mislabel `Unproven` as `Valid`, or claim full proof when relevant analyses were omitted.

**§ 4(4)** The report, including machine-readable output, identifies any relevant incomplete, not-run, unknown or unproven state. The textual summary should translate this into clear user-facing language instead of relying on solver terminology.

**§ 4(5)** The selected target, `CompilationPlan` and applicable profile remain authoritative. `--depth` does not relax a profile's mandatory requirements.

## § 5. Human-readable output

**§ 5(1)** Normal human output reports actionable diagnostics and findings at the user's configured effective severity. If no reportable findings remain, successful `sec analyse` is silent and exits successfully.

**§ 5(2)** Ordinary output does not print the internal analysis graph, every examined function, proof fact or progress status. A successful command must not generate a wall of explanatory data merely because Deep analysis ran.

**§ 5(3)** `--info` produces the normal findings plus a concise human-readable summary: requested analyses, selected root/scope (when applicable), depth, counts of reportable findings and any important limitations or unproven dependencies. The summary must remain short and relevant; it is not a dump of fact records.

**§ 5(4)** `--verbose` includes the `--info` summary and more detailed provenance, call paths, analysis-specific explanations and completeness information. It does not implicitly enable unrelated compiler-debug or backend-IR dumps.

**§ 5(5)** `--info` and `--verbose` are *presentation options*. They are not the diagnostic severity named `INFO`. Diagnostics retain their configured severity independently of whether a summary is requested. Existing user controls may promote eligible `INFO` diagnostics to `WARNING` or `ERROR`; this command does not redefine mandatory diagnostic severities.

**§ 5(6)** `--verbose` subsumes `--info`. Supplying both must not duplicate output.

**§ 5(7)** Unknown facts, real errors and skipped advisory work must not be presented as equivalent to “no problems found.” The silence rule in § 5(1) applies only to an actually successful invocation without reportable findings or required limitations.

## § 6. Machine-readable reporting

**§ 6(1)** `--json` requests one machine-readable analysis report rather than the normal human diagnostic layout. Its purpose includes CI, IDE integrations, automated testing and external tools.

**§ 6(2)** The report is explicitly versioned. The CLI/tooling owner must register a stable report schema and compatibility policy rather than relying on human-readable text parsing.

**§ 6(3)** At minimum, the schema must preserve: schema identity/version; selected target or compilation context where relevant; requested analysis family/families; focus root and resolved canonical callable identity if provided; depth; executed and incomplete analysis status; deterministic diagnostic records with stable identities and locations; proof completeness/unknown status; and a short summary of findings.

**§ 6(4)** An invocation with no findings still returns a valid JSON report. Empty diagnostics do not imply complete proof when the completeness field reports otherwise.

**§ 6(5)** JSON must preserve current diagnostic promotion configuration, stable identifiers, deterministic ordering, and the distinction between `Invalid`, `Unproven` and valid findings. It should reuse the canonical diagnostic-occurrence representation where compatible rather than defining a second competing diagnostic type.

**§ 6(6)** A future JSON schema revision must not silently reinterpret previously published fields. Producers and consumers need an explicit schema version or compatibility signal.

**§ 6(7)** `--json` is an output-format selection, not an analysis selection or a means of increasing analysis depth. The implementation should keep human presentation options separate rather than attempting to serialize verbose prose as JSON.

## § 7. Exit status and failures

**§ 7(1)** A completed analysis with no reportable failures returns exit status zero.

**§ 7(2)** Analysis rejection, invalid CLI use, inability to resolve a requested function or required unresolved safety/profile proof must be observable as failure under the canonical diagnostic and command exit policy. The correction does not invent a new numeric exit-code taxonomy.

**§ 7(3)** Advisory warnings do not automatically become language errors unless configuration or the owning analysis/profile contract requires it.

**§ 7(4)** An unavailable analysis or an incomplete result must not be silently turned into success when the requested contract requires completion. The command should explain the missing dependency or implementation capability.

## § 8. `sec help`: the compiler as a mentor

**§ 8(1)** `sec help` lists public CLI commands, each with a short purpose statement. It is a general Sec command, not a mode of `sec analyse`.

**§ 8(2)** `sec help analyse`, `sec analyse --help`, and equivalent command-specific forms must explain the same canonical arguments, names, options, defaults, common examples and expected outputs.

**§ 8(3)** `sec --help` provides the same top-level command discovery as `sec help`. `sec help <command>` is the canonical form for explanatory help; `sec <command> --help` is its contextual equivalent.

**§ 8(4)** Help must explain why and when a command is useful. For `analyse`, it must distinguish `--depth` (work and proof budget), `--info` (short report), `--verbose` (details), `--json` (machine-readable output), and `-f` (rooted call-graph analysis).

**§ 8(5)** Help shall present practical commands in ordinary English, meaningful descriptions of analysis names, and actionable guidance for selecting overloads and interpreting `Unproven` or incomplete results. It must not require the programmer to know internal Semantic IR nomenclature.

**§ 8(6)** The CLI parser, help display, analysis name registry and machine-readable output use one canonical command/option registry (or generated equivalent from an authoritative definition). Divergence between parser behavior and displayed help is a defect.

**§ 8(7)** Unknown commands/options produce constructive guidance, e.g. nearest valid names and a pointer to `sec help`, without pretending that a rejected command was run.

**§ 8(8)** `sec diagnostics <ID>` retains responsibility for in-depth diagnostic explanations, including its existing JSON form. General `sec help` may link to it but need not duplicate full diagnostic catalogues.

## § 9. Canonical usage examples

**§ 9(1)** The following examples must appear in help or CLI tests, with usage and result semantics matching this document:

```sh
# Analyze all registered analysis families. Remain silent on clean success.
sec analyse

# Explicitly request all analyses and the deepest refinement.
sec analyse --all --depth deep

# Analyze one family.
sec analyse isr
sec analyse data-races
sec analyse parameter-usage
sec analyse pitfalls
sec analyse deadlocks
sec analyse stack

# Analyze one root and its reachable call graph.
sec analyse -f myFunc
sec analyse deadlocks --function myFunc

# Reduce the analysis budget without manufacturing positive proof.
sec analyse -d standard

# Request a short human summary, detailed explanation, or JSON.
sec analyse --info
sec analyse --verbose
sec analyse --json

# Discover and understand commands.
sec help
sec help analyse
sec analyse --help
sec --help
```

**§ 9(2)** In user-facing examples, `myFunc` is a placeholder for a uniquely resolved callable, not a new reserved word or API.

## § 10. Implementation and migration obligations

**§ 10(1)** The implementation must provide one authoritative registry of analysis identifiers, command names, option spellings, help text and relevant metadata. Each executable individual analysis must map to exactly one registered CLI name.

**§ 10(2)** The CLI parser must support positional analysis selection, root selection, depth selection and presentation formats without changing semantic analysis results at equal depth and `CompilationPlan`.

**§ 10(3)** `-f` must use canonical call graph identities and validated interprocedural analysis dependencies; it must not be a substring filter on printed diagnostics.

**§ 10(4)** Reporting implementations must separate core diagnostic facts from human rendering and machine serialization. They must not duplicate diagnostic ownership or create a second inconsistent severity system.

**§ 10(5)** Add an authoritative CLI/help owning rulebook if none exists. Synchronize `compiler_analysis.md` § 61–62 and each individually selectable analysis rulebook to the canonical spellings without renumbering existing normative paragraphs.

**§ 10(6)** Preserve historical implementation progress accurately: existing `sec analyse`/`--all` behavior may be implemented; newly specified flags, report schemas or help must not be marked implemented until checked against compiler code and tests.

**§ 10(7)** Update the appropriate per-book governance fragments and `language-rulebook-status.md`. Do not overwrite the legacy read-only implementation-status ledger or introduce duplicate implementation entries. Remove MD-017 from `missing-decisions.yaml` only after all owning rulebooks and relevant governance have been synchronized.

## § 11. Required validation and regression tests

**§ 11(1)** Verify normal clean output is empty and exit status is zero; actual findings remain visible by default and are ordered deterministically.

**§ 11(2)** Verify `--all` and the no-argument form request the same analyses with Deep depth by default.

**§ 11(3)** Verify each canonical individual analysis name, unknown-name diagnostic, and consistency of registry/parser/help/JSON.

**§ 11(4)** Verify `-f` and `--function` have equal behavior for direct, transitive, recursive and mutually recursive calls, generic specialization, overloaded names and ambiguous selectors.

**§ 11(5)** Verify indirect closed/open callable target handling and that unknown FFI/system-call contracts, shared-state and relevant cross-thread/ISR dependencies do not silently disappear under focused analysis.

**§ 11(6)** Verify `--depth interactive|standard|deep` and `-d` selection. A reduced depth may yield incomplete/unknown analysis but must never manufacture a positive safety proof.

**§ 11(7)** Verify that `--info` adds a concise summary, `--verbose` adds detailed evidence, their combination does not repeat text, and severity promotions remain independent of presentation mode.

**§ 11(8)** Verify valid JSON with zero findings, with findings, with incomplete proof, with target changes and with focused callable identity; ensure stable ordering, declared schema version and existing diagnostic identity semantics.

**§ 11(9)** Verify `sec help`, `sec help analyse`, `sec --help` and `sec analyse --help` describe current options and do not drift from parser behavior. Unknown commands provide mentor-style suggestions.

**§ 11(10)** Verify analysis failures, unproven mandatory requirements and unavailable required analyses cannot produce misleading clean-success output.

**§ 11(11)** Validate that cold, incremental, parallel and focused analysis agree on applicable canonical findings under equivalent completed analysis depth and CompilationPlan.

## § 12. Decision record and remaining work

**§ 12(1)** MD-017 is **design-decided** by this correction. Source integration and test validation remain separate work.

**§ 12(2)** The CLI implementation's exact internal framework is not specified. User-facing semantics and authoritative identity/compatibility guarantees are normative.

**§ 12(3)** The complete field-by-field JSON schema needs a canonical machine-readable definition as part of implementation, consistent with the minimum requirements in § 6. This is a schema-delivery task, not permission to revisit the accepted CLI command model.

**§ 12(4)** This correction does not decide unrelated CLI features (`sec fmt` source selection, new analyses, or new diagnostic codes).
