# Sec Compiler Rulebooks — Modernization and Normative/Implementation Separation, Pass 1

| Field | Value |
|---|---|
| **Status** | Audited correction and integration contract; not yet applied to canonical rulebooks |
| **Created** | 2026-10-28 |
| **Last updated** | 2026-10-28 |
| **Document revision** | 1.0 |
| **Sec language version** | 0.1; incorporates owner-approved later 0.1.x direction where explicitly stated |
| **Repository** | `YoePro/sec`, `main` |
| **Verified HEAD** | `7be87775a851be60a692a50010d1ab368550b7d1` |
| **Primary targets** | `rules/compiler/rules_implementations.txt`; `rules/compiler/parser_recovery.md` |
| **Secondary review target** | `rules/compiler/compiler_known_members.md` |
| **Authoritative governance owners** | `governance/parser.yaml`; `governance/sema.yaml`; `governance/analysis.yaml`; `governance/errors_diagnostics.yaml`; feature-specific existing governance records |
| **Relevant decisions** | MD-003 (canonical syntax and migration), MD-029 (error patterns), MD-042 (reachability), MD-102 (contextual `Some`); MD-016, MD-052, MD-073 and MD-109 are related to later compiler-known-member review |
| **Disposition** | Preserve existing normative rules until reconciled; do not infer implementation completion from this document |

> **Execution rule:** Work on one canonical rulebook or migration target at a time. Before each edit, verify `main` HEAD and compare the existing sync identifier. For each target, inspect the current normative owner, reconcile existing governance IDs, make only the verified changes for that target, run focused tests and read the diff before continuing. Do not declare implementation complete merely because a specification has been updated.

---

## § 1. Purpose and document authority

**§ 1(1).** This document is an **integration/correction artifact**, not a replacement language rulebook and not a replacement governance file. It identifies the work required to modernize the `rules/compiler` directory without overwriting Sec's more recent normative choices or implementation history.

**§ 1(2).** A canonical rulebook defines stable language or compiler obligations. It must not contain an inventory of implemented, partially implemented, planned, pending, or failed implementation tasks. Those belong exclusively in the appropriate existing `governance/*.yaml` record and, where the repository uses them, package TODO files. A normative rule may specify a required compiler property or observable behavior; that is not the same as claiming it has been implemented.

**§ 1(3).** Preserve previously published numbered paragraph identities on later revisions. For an old **unnumbered** book, a first full numbered revision may assign new stable paragraph IDs; after publication, additions must not renumber earlier IDs. Do not use automatically shifting headings as persistent source-code references.

**§ 1(4).** Source authority, in descending order within each specialized subject, is the currently approved canonical rulebook and applied corrections, then implementation-facing package contracts consistent with those rules, then actual implementation as evidence of current support. An old design proposal or stale implementation-status statement never supersedes a later accepted rule.

**§ 1(5).** This work does not grant new authority to an old compiler-internal architecture, nor does it decide unrelated open MD questions opportunistically.

---

## § 2. Inventory and first-pass triage

**§ 2(1).** At the verified HEAD, `rules/compiler` contains the following files. The suggested action is a review priority, not a claim that all other files are current in substance.

| File | Observed form | First-pass action |
|---|---|---|
| `compiler.md` | Dated header, numbered `§` paragraphs; baseline September 2026 | Later semantic freshness review; keep stable IDs. |
| `compiler_analysis.md` | Dated header, numbered `§` paragraphs; baseline September 2026 | Later semantic freshness review; keep stable IDs. |
| `compiler_known_members.md` | Old `## Status` and unnumbered thematic headings; many October edits | **Priority 2**: separate normative registry from implementation notes, number paragraphs, reconcile MD-016/073/109 without deciding their open parts by fiat. |
| `compiler_pipeline.md` | Dated header, numbered `§` paragraphs; baseline September 2026 | Later semantic freshness review; keep stable IDs. |
| `generics_lowering.md` | Dated header and numbered sections; October update | Lower formatting priority; consider stable paragraph anchors. |
| `initialization.md` | Dated header and numbered sections; August baseline | Semantic freshness audit, not a mechanical rewrite. |
| `linking.md` | Dated header and numbered sections; August baseline | Semantic freshness audit, not a mechanical rewrite. |
| `monomorphization.md` | Dated header and numbered sections; October update | Lower formatting priority; keep its current specialization decisions. |
| `parser_recovery.md` | Old unnumbered headings; explicit historical status, roadmap, implementation appendix | **Priority 1**: first full normative revision and status/governance separation. |
| `rules_implementations.txt` | Mixed historical status ledger, former Go rules-package proposal, phases and illustrative code | **Priority 0**: classify, migrate any still-untracked status, retain only unique current norms in their actual owner books, then retire the `.txt`. |
| `semantic_ir.md` | Dated header and numbered `§` paragraphs; current October amendments | Preserve IDs; later focused freshness review only. |

**§ 2(2).** Merely counting the word `implementation` does not identify a forbidden task. Language text can legitimately refer to an implementation-defined representation or an implementation obligation; the prohibited content is **progress tracking, task lists, sprint phases, code-change instructions, or claims about current feature completeness** inside a canonical rulebook.

---

## § 3. `rules_implementations.txt`: correct classification and retirement

**§ 3(1).** This file is **not a homogeneous normative rulebook**. Its early sections are a dated status ledger for reference models, compiler-known members, LSP definition, interface cycles, unions, defaults, parser recovery, ownership/Place analysis, grammar, operators, and other areas. Later sections beginning with `Purpose` propose a distinct compiler-internal Go `rules` package, its conceptual `Context`, named helper functions, phase-by-phase extraction, a testing strategy, and a Codex implementation procedure.

**§ 3(2).** Do **not** rename this file to `rules_implementations.md` and declare it normative. Do **not** copy its suggested Go package layout, rule names, `Context` struct, diagnostic collector or staging phases into a normative Sec language rule merely because they were once written down.

**§ 3(3).** Modern compiler authority already resides in the canonical `compiler.md`, `compiler_analysis.md`, `compiler_pipeline.md`, `semantic_ir.md`, and `tooling/diagnostics.md`, with implementation recorded in feature-specific governance files. The old package proposal may be useful *historical design input* only where an actual missing normative invariant is identified. No separate `rules` package is required by this correction.

**§ 3(4).** Classify **every substantive clause** in the old `.txt` before retirement into one of these mutually exclusive destinations:

1. Existing normative requirement already covered elsewhere: retain its current owner, **do not duplicate**.
2. Valid normative requirement not covered elsewhere: propose one narrowly scoped amendment to the owning rulebook, preserving that book's IDs.
3. Status/remaining work/verification/callout: reconcile against an existing governance ID; add only previously untracked current work.
4. Historical or superseded design: remove from the active normative surface; retain historical provenance only in the repository's established archival convention, if applicable.

**§ 3(5).** A clause that says a low-level stage can only reject an unsupported source feature as a compiler defect needs correction. The current compiler distinguishes invalid Sec source, valid but **unimplemented lowering/capability**, and internal compiler invariant violations. Unsupported lowering must be surfaced by the established capability boundary with a truthful, focused diagnostic; it is not automatically an internal compiler bug. Preserve this distinction in `compiler_pipeline.md` and `semantic_ir.md` rather than importing the old text unchanged.

**§ 3(6).** Diagnostic wording and severity must follow the current `rules/tooling/diagnostics.md` stable-ID, severity, structured argument, and LSP/CLI consistency rules. The old proposal's `severity: error/warning` floor and optional diagnostic code are no longer a sufficient diagnostic contract; `Information` exists, and stable IDs are part of the public tooling contract.

**§ 3(7).** Verify governance counterparts before adding any entry. In particular, **`governance/parser.yaml` already contains** `frontend.parser-recovery-result`, `frontend.grammar-conformance`, and `frontend.parser-long-term`. It would be incorrect to create a second generic parser recovery tracker from old `.txt` status. Other families should likewise be routed to their **existing** owners rather than aggregated into one new rules-status tracker.

**§ 3(8).** Retirement is safe only after confirming references to `rules/compiler/rules_implementations.txt` are removed or redirected appropriately. Do not make it canonical again through a redirect from another normative rulebook.

---

## § 4. `parser_recovery.md`: material stale-content findings

**§ 4(1).** The existing `parser_recovery.md` is approximately 3,910 lines long. It has a large normative recovery catalog that **must be preserved where still valid**. Do **not** replace it with a short summary that silently deletes specialized recovery behavior, partial AST retention, delimiter/episode invariants, or tooling contracts.

**§ 4(2).** The following **progress-report/implementation-plan regions** are not normative material and must be extracted or retired, not reproduced in the refreshed rulebook:

| Old section (current unnumbered heading) | Disposition |
|---|---|
| `# Current implementation status` and its `Implemented` subsections | Reconcile with existing `governance/parser.yaml`; then remove from normative book. |
| `# Partly implemented` | Reconcile existing `frontend.parser-long-term`; remove. |
| `# Not implemented` | Reconcile existing remaining entries; remove. |
| `## Current implementation gap` under compilation-unit synchronization | Replace with timeless required behavior only; remaining state belongs to governance. |
| `# Current implementation corrections` | Move only still-valid outstanding actions to existing governance; remove. |
| `# Implementation status matrix` | Reconcile with governance; remove. |
| `# Tests`, `# Required recovery fixtures`, `# Parser bootstrap requirements` | Place actionable test/checklist/fixture work in governance or an implementation package; keep only normative observable invariants in the rulebook. |
| `# Required synchronization` | Extract to integration/governance instructions; cross-rulebook authority references may remain in the normative header. |
| `# Appendix A — Codex implementation plan` | Remove from normative book; this is explicitly a task plan. |
| Historical implementation claims in `# Design summary` | Replace with a timeless normative summary, not a status report. |

**§ 4(3).** The above is a **section classification**, not permission to discard a unique normative invariant accidentally embedded in a status section. Compare each region with current source owners first. Carry forward any still-current semantic requirement into a stable normative paragraph; governance owns the corresponding unimplemented work.

**§ 4(4).** Specific semantic freshness corrections required at the verified HEAD:

| Old claim/location | Current authority and required correction |
|---|---|
| `# Reserved syntax` lists `panic` and `assert` with reserved/nonimplemented tokens. | **Wrong.** Both have canonical Sec 0.1 semantics in `rules/errors/panic.md`; parser recovery must follow the *active* assertion/explicit-panic grammar. Do not classify valid occurrences as reserved. |
| `# Contextual x recovery` begins “After contextual matrix x is implemented”. | **Stale.** Contextual matrix `x` is parsed and semantically resolved today (`operators.md`, grammar and Sema). Recovery must specify how a missing operand is retained without implying the operator is future. |
| `# Compilation-unit synchronization` lists `#target` as an unqualified grammar start. | Reconcile file-level `#target` migration with canonical `@target` / `@when` rules. `#target` may be encountered as a documented compatibility form, but it must not be advertised as canonical new source syntax. Scope and validity come from attributes/compiler/grammar owners. |
| `# Compatibility syntax` suggests legacy constructs can simply be accepted. | Follow `grammar.md`'s MD-003 rule: noncanonical forms are invalid unless an active canonical owner **explicitly** defines a migration exception; default formatting must not silently normalize invalid code. Explicit Language Correction requires a proven semantics-preserving rewrite. |
| `# Enum recovery` states `Value: expression` “may be accepted as compatibility syntax”. | Do not turn this into normal accepted syntax or a soft warning. Follow canonical enum grammar and current migration diagnostics/fix contract. The parser recovery book preserves the erroneous source; it does not authorize obsolete enum syntax. |
| `# For recovery` calls out C-style and condition-only forms as unsupported. | This is compatible with current grammar, but **also preserve legal `for { ... }` infinite loops** and numeric/range iteration. Do not accidentally reject them because of “condition-only for” wording. |
| `# Match recovery` and `# Try-handler recovery` use pre-MD-029 pattern assumptions. | Synchronize accepted `Err(ErrorType.Variant(binding))`, shallow named-field destructuring, and contextual `Err(.Variant...)` only at the approved Sec 0.1.x compatibility boundary. The parser must not reject valid form as generally forbidden nested matching. |
| `# Parser diagnostic IDs` says “Recommended initial allocation”. | These are now stable registered diagnostic identities, not an optional initial proposal. Reference their canonical definitions in `rules/tooling/diagnostics.md` and the active diagnostic catalog; do not freeze obsolete or duplicated definitions in parser recovery. |
| Go pseudostructs describing `ParseResult`, `Diagnostic`, `RecoveryEvent`, etc. | Source-neutral *structural contracts* may remain, but replace speculative Go layouts with normative observable fields and invariant statements. The exact Go package, AST field names and implementations belong to compiler code/governance. |

**§ 4(5).** `parser_recovery.md` must not independently decide that `?` is permanently invalid. The current Sec 0.1 grammar reserves `?`; later conditional expression syntax remains governed by its own future rule. Recovery should diagnose under the active selected language version, without claiming future versions may never use the token.

**§ 4(6).** The newer MD-029/042/102 correction is an **approved design input**, not proof that the running frontend has already integrated the changed syntax or type inference. Do not mark any of those behaviors implemented in governance until source, tests, and tooling confirm it.

---

## § 5. Normative content the parser-recovery revision must retain

**§ 5(1).** Preserve these invariants as actual stable, numbered normative paragraphs:

1. Invalid source remains invalid: recovery must never silently reinterpret a malformed expression as a different valid program.
2. The parser makes forward progress, prevents unbounded error cascades, preserves enclosing syntax boundaries and avoids consuming legitimate siblings.
3. Partial parse results, invalid nodes, source ranges, skipped-token ranges, and uniquely justified virtual structural tokens are distinguishable from accepted source syntax.
4. Speculative parsing is transactional across all diagnostics, recovery events, virtual tokens, delimiter/context stacks, and mutated partial nodes.
5. Recovery is context-sensitive and depth-aware; same token spelling may have a different boundary role in a different grammar position.
6. A primary malformed construct normally produces one primary diagnostic with deterministic stable identity; secondary diagnostics must not obscure the original cause.
7. A recovered syntax error blocks successful batch compilation and artifact publication; LSP and formatting may retain valid neighboring structures without declaring the program valid.
8. The formatter never silently makes a semantics-changing repair, and LSP quick fixes are offered only where a concrete replacement is justified.
9. Internal invalid/error type facts suppress dependent semantic cascades without hiding independent errors in unaffected declarations.
10. Actual declaration and expression start sets come from canonical grammar/lexer metadata, not an ad hoc frozen list that becomes stale when new syntax is introduced.

**§ 5(2).** Retain the specialized family contracts for **declarations, generics, parameters, calls, arrays, type references, contracts, defaults, structs, struct literals, enums, unions, registers, interfaces, impl blocks, properties, events, functions, extern, bindings, assignments, `if`, `for`, `while`, `switch`, `match`, `try`, return, defer, discard, select, unsafe, inline assembly, expressions, members, indexing/slicing, ranges and contextual operators**, but synchronize every syntax claim against the owner of that construct.

**§ 5(3).** The primary source for accepted grammar is `rules/foundations/grammar.md` plus feature-specific rulebooks. Parser recovery only defines **how invalid/incomplete forms are represented and diagnosed**. Avoid copying entire valid grammars into recovery clauses, since that creates a second inconsistent syntax specification.

**§ 5(4).** The refreshed rulebook should be organized around the public/observable recovery contract, not the historical order in which a Go parser was developed. Suggested stable sections are: authority, parse outcome, parser diagnostic contract, recovery events, invalid-node contract, insertion/skipping, source ranges, deterministic episodes, boundary/progress, context-specific recovery, interaction with Sema/formatter/LSP, and the invariant summary. Specialized recovery clauses can form clearly numbered subsections under the context-specific section.

---

## § 6. Proposed normative additions for the revised parser book

The following clauses are **draft replacement norm text**. They are not implementation tasks. Integrate them at new stable paragraph IDs in the canonical `parser_recovery.md`; do not use these correction-local section numbers as canonical paragraph identifiers.

**§ 6(1) — Authority.** “Parser recovery describes the treatment of incomplete and malformed **source syntax under the selected Sec language version**. It must not broaden or narrow the accepted grammar independently of `rules/foundations/grammar.md` or a specialized active feature rulebook. On conflict, accepted grammar and semantic validity are determined by those owners; this book owns source preservation, diagnostic recovery and parser progress.”

**§ 6(2) — Canonical-syntax separation.** “A recognized legacy form is not made canonical by its recovery representation. An invalid legacy form remains a parse/validation error and blocks artifact publication. A language-correction action may rewrite it only where the canonical migration contract proves the correction unambiguous and meaning-preserving.”

**§ 6(3) — Pattern recovery.** “For Result/error patterns, recovery must distinguish an `Err` carrier pattern, a concretely qualified error variant, a contextually shortened `.Variant` when its enclosing error type is known, and an optional single shallow payload or field-pattern binding. Those source forms must not be rejected merely because the general nested-pattern grammar remains closed. Malformed payload patterns retain the outer `Err` handler/arm, source location and subsequent arms. A missing or ambiguous qualification is a semantic/type-resolution concern unless the syntax itself is malformed.”

**§ 6(4) — Variant coverage.** “Proof that a well-formed `match` arm cannot execute does not turn its syntax into an invalid pattern. Parser recovery does not emit an unreachable-arm error based on inferred active variants. The semantic diagnostic layer distinguishes source-order full coverage (invalid when the owning match rule says so) from a valid arm excluded only by proven subject state (informational under the approved MD-042 rule).”

**§ 6(5) — Contextual Option.** “A syntactically valid one-argument `Some(expression)` is not rejected as an unknown function at parse time merely because it has no immediately available Option target type. Semantic analysis determines whether context or the payload supplies a legal `Option[T]` instantiation, and separately validates ownership, borrowing and lifetime. `None()` remains a syntactically recognizable but invalid use of the absence value, with the canonical focused diagnostic.”

**§ 6(6) — Active control constructs.** “Parser synchronization recognizes active `panic`, `assert`, legal infinite `for`, selected source `@target`/`@when`, and contextual matrix `x` according to their owning rules. It must not treat a currently valid construct as reserved merely because an earlier parser-recovery revision predated that construct.”

**§ 6(7) — Diagnostic identity.** “Parser diagnostics have stable registered IDs, structured primary ranges, related locations, notes and help as defined by the canonical diagnostics rulebook. Severity and mandatory/configurable status are owned by that registry. A recovery event records the parser's treatment of invalid syntax; it does not silently insert a valid token into the actual source or change the program's semantic status.”

**§ 6(8) — Implementation independence.** “No implementation-language package name, Go struct layout, pass name, optimization choice, or current status is normative unless an owning compiler architecture rule explicitly makes that fact an externally required interface. The compiler may organize recovery internally in different ways provided the observable contract and proof/progress invariants are preserved.”

---

## § 7. Governance reconciliation, not governance duplication

**§ 7(1).** The canonical parser owners currently include:

| Existing governance ID | Preserve / reconcile |
|---|---|
| `frontend.parser-recovery-result` | Structured parse result, invalid nodes, recovery events, diagnostics and actual completed parser paths. Do not rewrite implemented entries as pending. |
| `frontend.grammar-conformance` | Current accepted grammar, legacy rejection and migrated parser/formatter syntax distinctions. |
| `frontend.parser-long-term` | Remaining special-case recovery defects, unretained invalid constructs, episode suppression, traversal policy, long-running fuzz work and parser coverage. |

**§ 7(2).** Reconcile the old `rules_implementations.txt` rows with the owners above and with specific records in `governance/sema.yaml`, `governance/errors_diagnostics.yaml`, `governance/analysis.yaml` and other existing feature owners. Use existing integration IDs and their machine-readable `implemented`, `remaining`, and `identified_bugs` organization where present. Never create a second global “rules implementation” truth source.

**§ 7(3).** Migrate any **newly established** implementation defect as an appropriately identified bug in its existing governance integration; use evidence, source locations, a reproducible check, severity and affected normative contract. Do not convert a speculative 2026-08-07 “not implemented” label into a new bug in October 2026 without checking current code and governance first.

**§ 7(4).** Tests, fixture creation, CI/fuzz campaigns, converter scripts, implementation-package phase plans, and specific Go refactors belong in governance and package plans. The rulebook may retain observable conformance requirements, not a list of tasks for Claude/Codex.

---

## § 8. Decision ledger and MD handling

**§ 8(1).** The parser recovery modernization does **not** itself expose a new unresolved design fork. Its task is consistency with already-accepted semantic owners. Therefore it should **not** create a new MD entry merely to modernize headings or remove implementation status.

**§ 8(2).** MD-029, MD-042, and MD-102 have owner-approved design decisions documented in their separate correction. When these changes land canonically, parser recovery and grammar must follow them; no new parser-only version of those decisions is appropriate.

**§ 8(3).** The later `compiler_known_members.md` review is where MD-016 (complete compiler-known declaration/structural contracts), MD-073 (char-array conversion), MD-109 (core string materialization) and related open items may become relevant. **Do not mark these solved** by merely updating a header or importing a proposal. MD-052's privileged core helper body syntax likewise remains a separate decision requiring its actual owner rules.

**§ 8(4).** When a newly found conflict is already settled by another approved rulebook, classify it as an outdated cross-reference or defect in the stale book, not as a new language-design decision.

---

## § 9. Application order and acceptance criteria

These are **integration instructions for this correction artifact**, not content to insert into the normative book.

**§ 9(1).** Inventory every active reference to `rules_implementations.txt` and identify its existing normative/governance owner. Classify each clause per § 3(4); preserve provenance and avoid duplicate governance IDs. Retire the old `.txt` only after this audit is complete.

**§ 9(2).** For `parser_recovery.md`, snapshot its original heading/section map and all links to it. Produce a first fully numbered normative revision with a complete metadata header and cross-rulebook authority. Keep every still-current specialized recovery rule and its observable contract; remove historical status and implementation phases under § 4(2). Introduce only the semantic corrections proven in § 4(4) and the approved paragraphs in § 6.

**§ 9(3).** Run focused source diagnostics, parser-recovery, malformed-input, deterministic-failure, formatter Language Correction, and LSP recovery tests. Compare diagnostics and partial-AST shape on existing golden malformed fixtures. A rulebook-only diff cannot count as proof these tests passed.

**§ 9(4).** Reconcile `governance/parser.yaml`, and only the **affected** records in other governance files. Note the new canonical revision and remaining implementation work without resetting actual implementation progress. Preserve machine-readable IDs and known bug provenance.

**§ 9(5).** Review the updated book for: stale language constructs, broken references, implementation-status paragraphs, task-oriented prose, unnumbered normative sections, inaccurate diagnostic IDs, and unintentional loss of established specialized recovery requirements. Repeat the focused diff/verification cycle before proceeding to the next book.

**§ 9(6).** The next standalone modernization target is `compiler_known_members.md`; it is materially larger than a formatting-only update because compiler-known APIs and MD-016 require exact authority distinctions and interface signatures. Treat it as a separate review, not an automated global renumbering.

---

## § 10. Non-claims

**§ 10(1).** This document does not claim a rewritten `rules/compiler/parser_recovery.md` has been merged, that `rules_implementations.txt` has been retired, or that any pending feature has been implemented. These changes must be applied and verified in the repository.

**§ 10(2).** It does not authorize replacement of established diagnostic IDs, changed public syntax, silent fallback to legacy compiler paths, or a new second semantic rule engine.

**§ 10(3).** The focus of this pass is to make Sec's compiler documentation **semantically current, normatively stable, and separate from engineering status**, with minimal disruption to established behavior and governance history.
