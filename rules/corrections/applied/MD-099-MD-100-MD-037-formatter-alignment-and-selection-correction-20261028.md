# MD-099, MD-100 and MD-037 — Formatter alignment and deterministic file selection

- **Decisions:** MD-099 — `formatter-trailing-comment-column-padding-limit`; MD-100 — `formatter-enum-values-always-aligned`; MD-037 — `sec-fmt-directory-and-project-selection`
- **Status:** **Applied — MD-099/MD-100 verified 2026-10-27; MD-037 implementation and governance closure verified 2026-10-28**
- **Created:** 2026-10-27
- **Last updated:** 2026-10-28
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Reference repository:** `YoePro/sec`, `main` at `5426af6cf8cce6fe85ff897b97050dfd330ef4fe` (2026-10-26)
- **Applied correction location:** `rules/corrections/applied/MD-099-MD-100-MD-037-formatter-alignment-and-selection-correction-20261028.md`
- **Archive after normative integration:** `rules/corrections/applied/MD-099-MD-100-MD-037-formatter-alignment-and-selection-correction-YYYYMMDD.md`
- **Primary normative owner:** `rules/tooling/formatter.md`, in particular §§ 2–4, 9, 12, 15, 25, 28 and 30
- **Related owners:** `rules/projects/projects.md` (single manifest, root and recognized directories); `rules/projects/modules.md` (project source versus selected compilation graph); `rules/tooling/lsp.md` (shared formatter)
- **Implementation:** `internal/formatter/trailing_comment_alignment.go`, `internal/formatter/binding_alignment.go`, `internal/formatter/formatter.go`, `cmd/compiler/main.go`, associated tests
- **Governance:** existing `governance/formatting.yaml` / `tooling.formatter-v2`, existing LSP/CLI project-tooling owners; **no duplicate entries**
- **Decision tracking:** `missing-decisions.yaml` MD-099, MD-100 and MD-037
- **Decision authority:** Approved by the Sec language owner 2026-10-27. MD-100's enum exception itself was already approved 2026-10-17; MD-099's cap is already normative in formatter.md § 12(6).

> **Integration contract:** This is a correction and precise command-selection amendment **to existing normative rulebooks**, not a parallel formatter specification. MD-099 fixes an implementation discrepancy with existing rules; MD-100 closes the exception-scope question without changing the already-approved enum policy; MD-037 defines directory/project discovery for the existing `sec fmt` CLI. Preserve all existing normative paragraph identifiers. Do not claim any implementation or test suite has passed before executing it.

---

## § 1. Scope, baseline, and preserved decisions

**§ 1(1).** MD-099 applies the established maximum-alignment rule of `formatter.md` §§ 9(9)–(10), 12(6) to trailing comments in every eligible nonexempt declaration group. It introduces **no new layout preference**.

**§ 1(2).** MD-100 confirms that the mandatory alignment exception of `formatter.md` § 15(4) belongs specifically to enum **explicit-value** groups. Its existing alignment and trailing-comment requirements remain unchanged. Other declaration families do not inherit that exception.

**§ 1(3).** MD-037 defines how `sec fmt` and `sec fmt --check` expand explicit directory paths into project/standalone Sec source files. The **per-file formatter is unchanged** and consumes the existing shared `[format]` configuration rules.

**§ 1(4).** None of these decisions requires a new source-language construct, CompilationPlan-based formatting, a separate project manifest, arbitrary host discovery or compiler target selection.

**§ 1(5).** Because MD-099 repairs a deviation from already-published formatting rules, MD-100 clarifies an existing exception, and MD-037 changes file **selection** rather than per-file canonical bytes, these amendments do **not** require changing `format_version` from its existing value `1`. Any later independent intentional style change still follows `formatter.md` § 4(4).

## § 2. MD-099 — trailing comments honor `max_alignment_padding`

**§ 2(1).** For all ordinary (nonexempt) contiguous homogeneous declaration groups, a candidate trailing-comment column is subject to the **same effective `max_alignment_padding`** limit as a candidate primary type/initializer column.

**§ 2(2).** Calculate required padding from the *formatted preceding code cell*, including previously accepted alignment columns. Include every row in the group when finding the widest code cell, including rows without trailing comments. The candidate is rejected for the **whole group** if any commented row would require padding greater than the configured cap. Exactly equal to the cap is permitted.

**§ 2(3).** When the candidate comment column is rejected, every affected trailing comment uses only the ordinary single structural separating space immediately after its code, with its comment text preserved. The formatter must not leave per-row ad hoc large gaps or align a subset of rows that happens to fit.

**§ 2(4).** A trailing-comment pass must not **restore visual columns** previously discarded by the primary-alignment pass due to the maximum-padding limit. Apply alignment decisions consistently to the final layout; a late comment pass may not bypass an earlier rejected primary column.

**§ 2(5).** Primary and trailing-comment columns are still distinct candidate columns: rejection of only the trailing-comment column does **not** force an otherwise legal type or initializer column to be dropped. If the primary candidate was already rejected, the trailing comments cannot silently reconstruct its spacing. This is the precise meaning of applying the limit to a structural group rather than blindly retaining far-right comment alignment.

**§ 2(6).** The rule applies equally to trailing `//` comments and qualifying trailing single-line `/* ... */` comments under `formatter.md` § 12(9a). Blank lines, standalone comments, multiline items and incompatible declaration boundaries continue to end groups as already specified.

**§ 2(7).** The mandatory enum explicit-value exception is defined in § 3 and is **not capped** by `max_alignment_padding`.

## § 3. MD-100 — enum-only mandatory alignment exception

**§ 3(1).** In a compatible contiguous group of enum members having explicit `=` values, the `=` anchors **must** align to one column even if the required padding exceeds `max_alignment_padding`, as already required by `formatter.md` § 15(4).

**§ 3(2).** The trailing `//` and qualifying single-line `/* ... */` comments of that group also align to their distinct structural comment column, regardless of the ordinary padding cap. The enum exception applies only where the existing § 15(4) group conditions are met; it does not merge across blank lines, standalone comments, implicit members or declaration boundaries.

**§ 3(3).** No corresponding unconditional exception is granted to struct fields, named/typed variable declarations, function or method parameters, register fields, named-type declarations, or other non-enum declaration groups. They retain the ordinary candidate-column rules and § 2 padding cap.

**§ 3(4).** The existing `[format].max_alignment_padding` setting is retained with its established validated domain `1..1024`, default `4 * indentation_width`, and existing manifest/CLI ownership. Enum's exception is not a reason to remove, disable or reinterpret this setting.

**§ 3(5).** No new `enum_alignment` flag, separate comment padding option, or source formatter directive is introduced.

## § 4. MD-099/100 examples

**§ 4(1).** With `max_alignment_padding = 4`, a non-enum group whose candidate comment column would need more than four spaces uses one space instead:

```sec
// Ordinary group: keep comments attached, do not force a distant column.
type Sample struct {
    x: int, // First
    veryLongFieldName: int, // Second
}
```

The example illustrates **comment-column drop**. Exact primary type-column layout remains governed by the existing syntax-anchor formatter and the same cap.

**§ 4(2).** An enum explicit-value group remains aligned regardless of the cap:

```sec
type State enum {
    Ok             = 0,   // Ready
    VeryLongStatus = 1,   // Busy
}
```

The conceptual example illustrates the mandatory `=` column and distinct trailing-comment column. Real fixtures must use the canonical enum declaration syntax and formatting expected by the currently selected formatter version; do not assume a schematic punctuation detail overrides the grammar.

## § 5. MD-037 — accepted command inputs and scope

**§ 5(1).** `sec fmt` accepts one or more explicit paths, each denoting a Sec source file or a directory to traverse recursively. Both the normal formatting command and `sec fmt --check` use the **same** path expansion rules. Existing `--stdin`, `--fix`, explicit formatter options and `--` literal-path handling remain unchanged except for the expanded path category.

```sh
sec fmt source.sec
sec fmt src/
sec fmt .
sec fmt --check .
sec fmt src/ tests/example_test.sec
```

**§ 5(2).** There is no implicit unrequested project-wide scan: supplying `.` is the explicit request to scan the current directory. The existing error for no source paths may remain. Standalone files not belonging to a Sec project continue to use formatter defaults plus explicit overrides.

**§ 5(3).** Recursively discovered source files have the exact `.sec` suffix. Explicit **file** arguments retain the current file-input contract and are not filtered by implicit directory exclusions merely because they happen to be test fixtures or live in a normally excluded directory. The command must never silently format arbitrary non-Sec files found during traversal.

**§ 5(4).** Directory-mode selection is based on the **explicit filesystem subtree** and not on the compilation source graph, current Target, imports, `@when`, active Variant, SourceSelection or an executable CompilationPlan. Consequently it includes valid project `.sec` files that are not currently imported or selected for building.

**§ 5(5).** Source files matching `*_test.sec`, ordinary source tests and ordinary modules under `internal/` are included in normal recursive selection. Test source participation in a build is a separate project/compiler concern and does not control formatting.

## § 6. Directory exclusion and filesystem boundaries

**§ 6(1).** Recursive scanning excludes project-reserved `.sec/` and `bin/` subtrees, VCS metadata (`.git/`, `.hg/`, `.svn/`), recognized dependency/vendor copies, recognized generated-output locations, hidden directories, and `testdata/` fixture trees by default. The exclusion is about **automatic discovery**, not a statement that `.sec` fixtures cannot ever be formatted explicitly.

**§ 6(2).** Do not infer that a `.sec` file is generated or vendored merely from the spelling of its filename or source content. The exclusions for generated/dependency files apply to canonically recognized output/dependency locations (or explicit recognized project metadata), not to guesswork or filesystem enumeration order. A normal project source directory called `internal` is **never** excluded merely because of its name.

**§ 6(3).** Recursive traversal does **not** follow symbolic links to directories or files. This prevents accidental source rewriting outside the explicitly selected tree, duplicate traversal and cycles. An explicit symlink file must be rejected with an actionable diagnostic rather than silently rewriting its target or replacing the link.

**§ 6(4).** Where a directory scan begins inside a valid project, its known project root supplies the configuration boundary. It must not silently continue into a distinct nested project manifest; nested projects under an existing project are already invalid per `projects.md` § 4. If such a boundary is encountered, report the project-boundary conflict rather than adopting an unrelated manifest. A standalone directory scan outside any project must not silently borrow a nearby project configuration.

**§ 6(5).** Paths resolving to non-files/non-directories, missing paths, inaccessible directories, unreadable files or unresolved boundaries are diagnostics. An explicit directory containing zero eligible `.sec` files may complete without edits, but must not be interpreted as a request to scan another directory.

## § 7. Deterministic, unique source selection

**§ 7(1).** Expand all file and directory arguments **before writing**. Normalize their lexical absolute paths for deduplication and sort the resulting unique files deterministically by normalized path (no locale-dependent collation or filesystem enumeration order). Preserve a stable user-facing display path in diagnostics.

**§ 7(2).** Overlapping arguments such as `sec fmt . src/file.sec` do not format `src/file.sec` twice. Multiple spellings of the same normal file must not produce multiple write attempts; canonical path normalization/deduplication is independent of target compilation identity.

**§ 7(3).** File selection is frozen for a single invocation. If files or directories change during execution, report a focused filesystem error when the operation can no longer safely process the selected snapshot; do not silently expand newly appearing paths mid-run.

**§ 7(4).** No implicit formatting of Sec's installed standard library, platform libraries, cached dependencies or compiler sources occurs unless those paths are explicitly within the supplied selection scope and are not excluded as managed external data.

## § 8. Configuration preflight and mutation policy

**§ 8(1).** Determine the effective formatter options **for every selected file**, according to `formatter.md` § 4(18)–(23): explicit invocation overrides, project `[format]` table, then defaults. A project config is not replaced by a target-dependent or editor-dependent formatter configuration.

**§ 8(2).** Finish **all** input path selection, readable-file checks, manifest/configuration resolution and configuration validation before modifying **any** selected source. Unknown/invalid formatter keys, invalid values, incompatible format versions and project-boundary errors fail the invocation without partial writes. This preserves `formatter.md` § 4(21)'s prewrite guarantee.

**§ 8(3).** Once the preflight succeeds, ordinary formatting modifies only the files selected as eligible and successfully formatted, in deterministic order. Preserve each file's applicable permissions and use the existing safe replacement/write mechanism. Do not rewrite `sec.toml` as a side effect of formatting.

**§ 8(4).** A malformed or incomplete Sec source file must preserve every CST-uncertain region byte-for-byte under `formatter.md` § 25. For bulk file-formatting, the safe initial CLI policy is to leave the entire malformed file unchanged, report it as malformed and continue with other fully valid files. Return nonzero if any selected file could not be safely fully formatted. This CLI policy does not prohibit an LSP document formatter from processing surrounding provably safe regions under its own existing contract.

**§ 8(5).** In `--check` mode, never write files. Aggregate a deterministic report of each valid file that would change and each malformed file, using the existing `format.noncanonical-source` and `format.malformed-source` diagnostic meanings. Return nonzero if any such item exists or if preflight fails.

**§ 8(6).** Existing `--fix` opt-in language-correction policy is unchanged. Directory traversal does **not** implicitly enable language correction or invent missing syntax to make malformed input format successfully.

## § 9. Core implementation requirements

**§ 9(1).** MD-099: pass the resolved `MaxAlignmentPadding` value to the trailing-comment alignment pass in `internal/formatter/trailing_comment_alignment.go` and check group-wide comment-column padding before emitting it. Both invocations from `internal/formatter/formatter.go` must use the same resolved cap. Ensure the CST-based `alignBindingGroup` comment-column pass uses the same cap; no late pass may reintroduce a rejected column.

**§ 9(2).** MD-100: preserve the existing enum-specific mandatory alignment path and explicit non-exempt alignment defaults for structs, params, named types, registers and bindings. Do not implement an unapproved new setting or a universal always-align policy.

**§ 9(3).** MD-037: extend `cmd/compiler/main.go` `runFmtCommand` with a well-delimited path-expansion/preflight phase. Use the existing `formatconfig.ForSource`, `internal/formatter` formatter and safe write utility; do not fork formatter behavior for directory inputs.

**§ 9(4).** Use the existing project root/manifest discovery utilities, the project rulebook's root boundaries and documented source directories. No independent hidden project resolver, target selection, recursive compiler import scan or new manifest is permitted.

**§ 9(5).** Where the implementation presently accepts only ordinary explicit files, report MD-037 as **decided, implementation pending** until recursive selection and end-to-end tests work. Do not claim functionality merely from a rulebook amendment.

## § 10. Required conformance/regression tests

**§ 10(1).** MD-099: cover at-cap, one-over-cap, mixed commented/uncommented rows, a previously dropped primary column, type/initializer columns still valid while comment column drops, long trailing comment text (which does not itself set the comment *starting* column), standalone comment boundaries, `//` and trailing `/* ... */`.

**§ 10(2).** MD-100: cover enums with very unequal name lengths, explicit `=` and trailing comments, both comment spellings, implicit member separators and blank lines. Verify enum alignment is unaffected by low caps while struct, register, parameter, and variable groups still honor the cap.

**§ 10(3).** MD-037: test one file, nested directories, project root `sec fmt .`, explicit multiple paths, overlapping paths, test files, ordinary `internal/`, unused modules, empty directory, standalone no-manifest mode, and project `[format]` precedence.

**§ 10(4).** Test exclusions for reserved `.sec/`, `bin/`, recognized generated/dependency paths, hidden directories and `testdata/`; confirm an explicitly named fixture file remains formatable. Test symlink cycles/non-following, missing paths, path alias deduplication and stable sorting.

**§ 10(5).** Test full preflight before writes with two files where the later file has an invalid formatter configuration. No file may be modified in that scenario. Test malformed source preservation and nonzero status with valid siblings still processed; test `--check` aggregates all noncanonical and malformed results without writing.

**§ 10(6).** Confirm CLI/LSP formatting parity on the same file and configuration, formatter idempotence, and `format_version = 1` compatibility. Test the previous bootstrap-lexer trailing-comment regression. Execute actual tests and record commands and result provenance before implementation acceptance.

## § 11. Stable rulebook integration

**§ 11(1).** `rules/tooling/formatter.md` § 4(14): remove the obsolete claim that it is undecided whether the trailing-comment column is capped. Retain the option's exact range, default, provenance and MD-098 references.

**§ 11(2).** Update/clarify existing §§ 9(9)–(10) and 12(4)–(6) to make group-wide comment-column rejection and enum exemption unambiguous. **Do not** renumber existing paragraph IDs; preserve § 12(6) as the already normative MD-099 authority.

**§ 11(3).** Preserve § 15(4)'s mandatory enum policy; attach the explicit enum-only scope clarification under that established authority or a new uniquely numbered subordinate paragraph without shifting existing IDs.

**§ 11(4).** Extend § 28 (CLI, LSP, diagnostics) **append-only** with MD-037 selection, exclusion, preflight and malformed-bulk behavior. Preserve the existing § 28(1)–(8). If an implementing branch has already appended later § 28 paragraphs, allocate unused IDs instead of overwriting or renumbering them.

**§ 11(5).** Reconcile `rules/tooling/lsp.md` only for shared formatter/configuration behavior; avoid defining a second formatter implementation. Reconcile `rules/projects/projects.md` and `modules.md` only as needed for root/discovery and reserved directory references, without altering what constitutes a compiled module or selected Target.

**§ 11(6).** Do not create a new standalone formatter rulebook, change existing project or formatter option schemas, silently enlarge the `format_version` namespace, or introduce a competing source-selection algorithm.

## § 12. Governance, acceptance and closure

**§ 12(1).** **MD-099:** normative rule exists already; mark its implementation nonconformance as identified work and close the *decision* tracker once rulebook clarification/governance is synchronized. Keep the regression/test work independently visible until complete.

**§ 12(2).** **MD-100:** decision resolved by confirmation of the enum-only exception; close the decision tracker after integrating the explicit exclusion scope and synchronizing applicable governance. It is not contingent on inventing unconditional alignment for other constructs.

**§ 12(3).** **MD-037:** decision resolved by the § 5–8 file-selection contract. Integrate before removing the normative `missing-decisions.yaml` entry; leave formatter CLI implementation/acceptance pending until the directory-selection suite actually passes.

**§ 12(4).** Update only **existing** `governance/formatting.yaml` / applicable CLI-LSP status entries by stable IDs and actual evidence. Do not duplicate functionality under invented owner names or mark a group fully implemented when it is not.

**§ 12(5).** After applying the correction to canonical rulebooks, archive the original file under `rules/corrections/applied/` with the actual sync date, preserving section/paragraph identities and the source HEAD reference. No files in the live repository have been modified by the creation of this correction.
