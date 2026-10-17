# Sec Command-Line Interface

## Status

Normative tooling rulebook, revision 1.0 (2026-10-17).

Created by the MD-017 correction
(`rules/corrections/applied/md017-sec-analyse-cli-and-help-correction-20261017.md`),
which made this book the owner of global command syntax, `sec help` and the
`sec analyse` command surface. Analysis semantics, proof obligations and
diagnostic identities stay owned by `rules/compiler/compiler_analysis.md`, the
individual analysis rulebooks and `rules/tooling/diagnostics.md`.

---

# § 1. Scope and separation

§ 1(1) This book defines the user-facing command forms, option spellings, help
behavior, analysis selection, analysis focus, analysis budget, human output,
machine-readable analysis report and exit behavior of the `sec` command.

§ 1(2) It never redefines what an analysis proves. Reporting verbosity and
analysis depth are independent: less output does not mean less analysis, and a
smaller advisory budget never establishes a proof that an omitted analysis
would have established.

§ 1(3) The spellings in this book are canonical. A migration alias exists only
when this book records it; an undocumented synonym is never normative.

# § 2. Canonical spelling

§ 2(1) Command names, analysis names and long options are lowercase ASCII;
multiword names use hyphens. Short options are one letter after `-`.

§ 2(2) A long option that takes a value accepts both `--name value` and
`--name=value`. A short option takes its value as the next argument.

§ 2(3) `--diagnostic-format <human|json>` remains accepted by every
diagnostic-producing command (`rules/tooling/diagnostics.md` § 14).

# § 3. Command registry

§ 3(1) The compiler owns one authoritative command registry: command name,
recorded aliases, synopsis lines, a one-line purpose and command-specific
details. The dispatcher, the usage synopsis, `sec help` and every
command-specific help render from that registry, so parser behavior and
displayed help cannot diverge; a divergence is a defect.

§ 3(2) The analysis-name registry of § 5.2 is part of the same authority: the
`sec analyse` parser, its help, the human report and the JSON report use it.

§ 3(3) The public commands of Sec 0.1 are `check`, `analyse`, `build`,
`test`, `fmt`, `init`, `diagnostics`, `emit-ir`, `emit-sec-mlir`,
`emit-mlir`, `emit-llvm`, the debug views `parse`/`ast`/`sema` and
`lex`/`token`, and `help`.

# § 4. Help

§ 4(1) `sec help` and `sec --help` list every public command with its
purpose. `sec help` is a general command, not a mode of another command.

§ 4(2) `sec help <command>` is the canonical explanatory form;
`sec <command> --help` (and `-h`) is its contextual equivalent and prints the
same text.

§ 4(3) Command help explains when and why the command is useful, its
synopsis, options and defaults, common examples and expected output, in
ordinary English and without Semantic IR terminology.

§ 4(4) An unknown command, help topic, option or analysis name is reported
with the nearest valid name when one is unambiguously close and a pointer to
`sec help`; a rejected command is never run.

§ 4(5) `sec diagnostics <ID>` keeps the in-depth explanation of diagnostics;
`sec help` may refer to it and does not duplicate the catalogue.

# § 5. `sec analyse`

## § 5.1 Forms

```text
sec analyse [--all | <analysis>] [-f|--function <callable>]
            [-d|--depth interactive|standard|deep]
            [--info | --verbose | --json] [--target <os-arch>]
            [<file.sec|dir|glob>...]
```

§ 5.1(1) Without source arguments the current directory is analysed.

§ 5.1(2) `sec analyse` and `sec analyse --all` request every available
registered analysis for the selected compilation context. `--all` is the
explicit spelling of the default, not a more expensive mode, and cannot be
combined with an analysis name.

§ 5.1(3) A first positional word that is a lowercase name without path
separator, extension or glob, and is not an existing path, is an analysis
name. A directory whose name equals an analysis name is written as a path
(`./stack`).

## § 5.2 Analysis names

§ 5.2(1) The canonical names are:

| Name | Analysis family | Owning rulebook |
|---|---|---|
| `isr` | ISR analysis | `rules/analysis/isr_analysis.md` |
| `deadlocks` | Deadlock analysis | `rules/analysis/deadlock_analysis.md` |
| `data-races` | Data race analysis | `rules/analysis/data_races.md` |
| `pitfalls` | Pitfall analysis | `rules/analysis/pitfall_analysis.md` |
| `parameter-usage` | Parameter usage analysis | `rules/analysis/parameter_usage_analysis.md` |
| `stack` | Stack analysis | `rules/analysis/stack_analysis.md` |

§ 5.2(2) No alternate pluralization, shortened alias or flag-based selector
exists. Another analysis family (for example closure analysis) gets exactly
one registered name before it becomes individually selectable; the table is
not an inventory of every fact producer.

§ 5.2(3) An unknown name is rejected with the available names, the nearest
name when unambiguous and a pointer to `sec help analyse`; it never falls
back to `--all`.

§ 5.2(4) A registered family that cannot run on its own in the current
implementation (because it exists only as mandatory compilation diagnostics or
is not implemented) is rejected with an explanation when selected alone; it is
never reported as a clean result (§ 5.6(4)).

## § 5.3 Function-focused analysis

§ 5.3(1) `--function` and `-f` are exact synonyms. They select a callable
analysis root, not a text filter.

§ 5.3(2) The focused result covers the root and every callable transitively
reachable from it through the canonical call graph (direct and indirect
targets; cycles converge under the interprocedural rules). Findings in a
reachable callee are reported with the call path from the root.

§ 5.3(3) Facts the focused analysis depends on — callers, shared state,
concurrency peers, interrupt contexts, profiles, foreign contracts — are not
omitted; an implementation may prune unrelated work only when the pruning is
provably safe. Unresolved, unavailable or only contractually trusted
dependencies are explained, and open call targets keep their declared
contracts with conservative unknown handling.

§ 5.3(4) The selector resolves to exactly one canonical callable: a canonical
callable ID, a `Type.Method` name or a unique short name. An ambiguous name
(overloads, specializations) is rejected with the candidates; nothing is
chosen by guesswork.

## § 5.4 Human-readable output

§ 5.4(1) The default output shows the actionable diagnostics and findings at
their effective severity, in deterministic order. A successful run without
reportable findings or required limitations prints nothing and exits 0.

§ 5.4(2) The default output never prints the call graph, every examined
function, proof facts or progress.

§ 5.4(3) `--info` adds a short summary: analyses run (and families that
cannot run alone), scope, depth, target, result counts and limitations.

§ 5.4(4) `--verbose` adds the detailed facts, provenance, call paths and
evidence and includes the `--info` summary once; supplying both never repeats
output. It does not enable compiler-debug or IR dumps.

§ 5.4(5) `--info` and `--verbose` are presentation options, not the `INFO`
diagnostic severity; diagnostic severities and project promotions are
unchanged by them.

§ 5.4(6) `--json` (§ 6) replaces the human layout and cannot be combined with
`--info` or `--verbose`.

§ 5.4(7) Unknown facts, errors and incomplete analysis are never presented as
"no problems found": an incomplete result prints its limitation even in the
default output.

## § 5.5 Depth

§ 5.5(1) `--depth` and `-d` are synonyms with the values `interactive`,
`standard` and `deep`. The default is `deep`.

§ 5.5(2) A reduced depth may limit advisory refinement and cross-procedure
exploration as `compiler_analysis.md` § 62 permits. It never suppresses a
mandatory error, never reports `Unproven` as valid and never relaxes a
profile or `CompilationPlan` requirement; the report names the reduced depth
as a limitation.

## § 5.6 Exit status

§ 5.6(1) A completed analysis without reportable failures exits 0.

§ 5.6(2) Invalid command use, an unresolvable or ambiguous focus, an unknown
analysis name and a selected analysis that cannot run exit with the usage
failure status (1); rejected source and proven errors exit with the
diagnostic failure status of the compiler (3). No new numeric exit-code
taxonomy is introduced.

§ 5.6(3) Advisory findings do not fail the command unless configuration or
the owning analysis/profile contract requires it.

§ 5.6(4) An unavailable or incomplete analysis is never turned into success
when the request requires it to complete.

# § 6. Machine-readable analysis report

§ 6(1) `sec analyse --json` writes exactly one JSON document to standard
output: format `sec.analysis-report`, version `1`.

§ 6(2) Version 1 has these fields, none of which is reinterpreted within the
version:

```text
format        "sec.analysis-report"
version       1
status        "complete" | "incomplete" | "rejected"
target        canonical target name
depth         interactive | standard | deep
analyses      [{ name, status: "executed" | "not-selectable", complete, note? }]
focus         null | { selector, callable (canonical ID), name, reachable [IDs] }
diagnostics   [canonical emitted occurrences, rules/tooling/diagnostics.md § 14]
findings      [{ analysis, class, rule, location { file, start, end },
                 message, callable?, path?, actions [] }]
limitations   [text]
summary       { errors, unproven, warnings, advisory, information }
```

§ 6(3) A run without findings still writes a valid report; an empty findings
list never implies complete proof when `status` or `limitations` say
otherwise. A rejected program writes a report with `status: "rejected"` and
its diagnostics.

§ 6(4) Diagnostics reuse the canonical emitted-occurrence representation, so
stable IDs, severities after project promotion and proof states are
preserved. Findings and diagnostics are ordered deterministically.

§ 6(5) A later version must change `version`; producers never silently
reinterpret published fields.

# § 7. Required tests

§ 7(1) Tests cover: silent clean output and exit 0; default versus `--all`;
every canonical analysis name and the unknown-name diagnostic; `-f` versus
`--function` including callee paths and ambiguous selectors; `-d` versus
`--depth` and invalid values; `--info`, `--verbose` and their combination;
JSON with no findings, with findings, with a focus and for a rejected
program; `sec help`, `sec --help`, `sec help analyse` and `sec analyse --help`
agreeing; and mentor suggestions for unknown commands and topics.

§ 7(2) The remaining conformance obligations of the MD-017 correction § 11 —
recursion and generic specialization under focus, open indirect targets,
cold/incremental/parallel agreement and optimized builds — are tracked by the
owning governance entry until implemented.
