# MD-027, MD-033, MD-036 and MD-054 — Contextual Literals, Import Sorting and Formatter Configuration

- **Status:** Applied 2026-10-12 to `rules/types/types.md` ("Contextual None and empty-literal arguments"), `rules/declarations/functions.md` (§ 19), `rules/errors/errorhandling.md` (§ 10.2), `rules/tooling/formatter.md` (§ 4(18)–(23), § 5(7), § 13(5), § 13(7)–(10), § 29(11)), `rules/projects/projects.md` (§ 6(1), § 6(8)–(10)) and `rules/tooling/lsp.md` ("Shared formatter"); `language_corrections = false` was already stated in formatter.md § 4(5) and § 4(17); layout use of the width settings remains tracked in governance
- **Created:** 2026-10-12
- **Last updated:** 2026-10-12
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Proposed canonical path:** `rules/corrections/pending/md027-md033-md036-md054-contextual-literals-formatter-configuration-correction-20261012.md`
- **Decision IDs:** `MD-027`, `MD-033`, `MD-036`, `MD-054`
- **Amends:** `rules/types/types.md`, `rules/errors/errorhandling.md`, `rules/foundations/grammar.md` (only where a grammar clarification is needed), `rules/declarations/functions.md` or the canonical overload rulebook, `rules/tooling/formatter.md`, `rules/projects/projects.md`, `rules/projects/modules.md`, `rules/tooling/lsp.md` (integration only)
- **Governance:** Synchronize the existing relevant fragments, including `governance/formatting.yaml`, rather than inventing parallel entries.
- **Related correction:** MD-055 (import comment attachment and group-heading spacing), approved 2026-10-12.
- **Reference:** Latest user-supplied `missing-decisions.yaml` (updated 2026-10-09).
- **Replaces:** No published numbered paragraphs. Preserve existing paragraph identifiers when applying changes.

## 1. Purpose and authority

**1.1.** This document records decisions made by the language owner. It is a correction for integration into existing canonical rulebooks, **not** a new competing formatter rulebook or an implementation-status audit.

**1.2.** The explicit default **`language_corrections = false` is a previously established decision**. It is reaffirmed here because the user requires it to be visible in the normative formatter rulebook. If the current rulebook already states it precisely, retain it and do not introduce a duplicate paragraph. If absent or contradicted, repair the owning paragraph.

**1.3.** Keep the formatter's supported option names, value ranges and other existing defaults as specified by the current canonical formatter rulebook. Examples in this correction must not be mistaken for an exhaustive option schema.

## 2. MD-027 — Contextual typing of `None` and `[]`

**2.1.** A bare `None` expression has no independently inferable `Option[T]` type. An empty collection literal `[]` likewise requires sufficient expected collection-type context. Existing contextual positions — typed declarations, assignments and returns — remain valid. **Call arguments and constructor/initializer arguments are contextual positions as well.**

**2.2.** When an argument position has one unambiguous expected parameter type, `None` may be typed as that `Option[T]` and `[]` may be typed as the applicable empty collection type. This does not create a universal implicit conversion between distinct `Option` or collection types.

```sec
fn AcceptOption(value: Option[int]) void {
}

fn AcceptItems(values: int[]) void {
}

AcceptOption(None)
AcceptItems([])
```

**2.3.** Function overloads are distinguished by their **complete input-parameter signature** (including types and other existing overload-signature components). A difference in return type alone never makes two otherwise identical signatures distinct. Return type and the type expected for a call result must **not** select an overload.

```sec
fn Fun(a: int, b: string) void {
}

fn Fun(a: string, b: int) void {
}

fn Fun(a: int, b: int) void {
}
```

All three overloads are distinct. The following two declarations, by contrast, **must not coexist** solely because they return different types:

```sec
fn Invalid(a: int, b: string) int {
    return 1
}

fn Invalid(a: int, b: string) string {
    return "one"
}
```

**2.4.** Overload resolution considers **all supplied input arguments together**. Arguments with independently known types can eliminate inapplicable overloads before the compiler determines the contextual types of `None` or `[]`. Candidate resolution must be performed consistently across arguments, rather than requiring an arbitrary overload decision before inspecting later arguments.

```sec
fn Set(key: string, value: Option[int]) void {
}

fn Set(key: int, value: Option[string]) void {
}

Set("count", None)  // Valid: the first overload is selected.
Set(42, None)       // Valid: the second overload is selected.
```

**2.5.** If the remaining viable overloads offer incompatible contextual types, the call is ambiguous. The compiler must not choose by declaration order, arbitrary defaulting, expected return type or an unannounced conversion.

```sec
fn Store(value: Option[int]) void {
}

fn Store(value: Option[string]) void {
}

let number: Option[int] := None
Store(number)  // Valid: Option[int] determines the overload.
Store(None)    // Error: ambiguous overload.
```

The same ambiguity rule applies to `[]` when overloads require distinct possible collection types.

**2.6.** The same contextual-argument resolution applies to ordinary function calls, methods, `init(...)` and `new` construction under their respective existing call/constructor rules. No new source syntax or dynamic overload dispatch is introduced.

**2.7.** Diagnostics must identify the conflicting candidate parameter types when overload ambiguity prevents context inference, and explain that an explicitly typed argument/binding resolves the ambiguity.

## 3. MD-033 — Deterministic, syntactic import sorting

**3.1.** Unless the source separates imports into explicit groups, the formatter sorts imports **alphabetically by their logical import path**. It does not classify imports automatically as standard-library, project, external or platform imports, and does not need the `CompilationPlan` or module-resolution results merely to sort them.

**3.2.** A group boundary is an intentional empty line in a grouped import region. The formatter sorts imports **within each group**; it does not merge, move or reorder imports **across** explicit group boundaries. Attached comments and detached-comment safeguards remain governed by MD-055 and the canonical formatter comment-attachment rules.

**3.3.** Optional comments may describe groups. A group-heading comment goes immediately before that group's first import, **without an empty line**. Separate groups with **one blank line**. The comment is documentation for readers, not a parser directive or a special import-class identifier.

```sec
import (
    // Standard library
    "fmt"
    "io"
    "net/http"
    "str"

    // Project modules
    "app/config"
    "app/server"

    // External dependencies
    "github.com/example/library"
)
```

**3.4.** Without manually separated groups or attached comment constraints, import paths are sorted in one alphabetic sequence. The formatter must not invent categories, insert group headings or insert blank lines based on a guessed package origin.

**3.5.** The module/import rules continue to determine *valid* paths: `core` does not require and must not receive an explicit import; a standard-library module is imported by its logical path (for example, `"net/http"`, **not** `"stdlib/net/http"`). These facts do not give the formatter a separate import sorting category. Special-purpose import roots receive no automatic grouping behavior from this correction.

**3.6.** If a detached or ambiguous comment makes safe sorting uncertain, preserve the affected import region's relative order rather than move comments or imports speculatively, as already decided in MD-055. Sorting is deterministic and idempotent. Do not change any comment text.

## 4. MD-036 and MD-054 — The `[format]` project table

**4.1.** The canonical project formatter configuration is the **`[format]` table** in the project's **`.sec/sec.toml`** manifest. It is a recognized project-manifest table, not an unknown-table error. The formatter rulebook owns the formatter option meanings; the project rulebook owns the TOML placement, loading and validation integration.

```toml
# .sec/sec.toml

[format]
format_version = 1
indentation_width = 4
vertical_style = "compact"
line_width = 120
language_corrections = false
```

**4.2.** The example shows accepted field spellings and representative values discussed and decided for Sec 0.1. The owning formatter rulebook remains authoritative for the complete set of supported configuration keys, including width modes, adaptive widths and their validated domains; those are not redefined or guessed here.

**4.3.** If the `[format]` table is absent, formatter settings use their normative defaults. If the table exists but omits a setting, **that setting** uses its normative default; other explicit settings remain in effect. A missing project manifest is not a reason to invent values from the host operating system or another unrelated project.

**4.4.** The previously decided default for **`language_corrections` is `false`**. This must appear explicitly in the formatter rulebook's canonical configuration/defaults section. Enabling it requires an explicit configuration or invocation choice. `language_corrections` is distinct from mandatory canonical whitespace/layout formatting; the existing separation between accepted-syntax normalization, optional language correction and safe fixes remains in force. Do not silently turn on repairs because `sec fmt` was invoked.

**4.5.** The order of precedence, highest to lowest, is:

1. **Explicitly supplied formatter CLI options** for the invocation.
2. **Settings in the selected project's `.sec/sec.toml` `[format]` table.**
3. **The canonical formatter defaults.**

A CLI option that is **not specified** must not shadow a project setting merely because the command-line parser has its own built-in default. The effective resolved options must be deterministic.

```toml
# .sec/sec.toml

[format]
indentation_width = 4
language_corrections = false
```

```sh
sec fmt --indentation-width 2 source.sec
```

This invocation uses indentation width **2** without changing the manifest. The normal default indentation width is **4**, but four is not a mandatory language-wide indentation width. The same explicit-over-project precedence applies to supported formatter options generally, including an explicitly controllable `language_corrections` setting. The exact CLI switch for that setting, if not already specified by the CLI rulebook, is **not invented here**.

**4.6.** `format_version` is a versioned configuration-format contract. When the **effective** format version is unsupported, the formatter must issue a useful diagnostic and **must not modify any source file** as a fallback. Invalid key names, types and option values likewise result in a diagnostic rather than silently ignored or guessed settings. Fail validation **before** changing any files in a requested operation.

**4.7.** The CLI formatter and the LSP must resolve and interpret the same project manifest and options through shared configuration code. When formatting a file with an established Sec project context, both use that project's `.sec/sec.toml`. For a stand-alone file with no selected/resolved Sec project, apply formatter defaults plus any explicit invocation options. Do not silently borrow a neighboring or host project's settings. Existing project-root discovery and LSP workspace rules determine whether a file belongs to a project; this correction does **not** introduce a separate, competing project-discovery algorithm.

**4.8.** The formatter remains deterministic for the tuple (source, effective options, formatter version). Explicit CLI overrides can intentionally produce different formatting for one run and do not rewrite the project manifest. Both CLI and LSP must validate input configuration consistently; interactive editor-provided formatting preferences must not silently override the project's decided configuration model.

**4.9.** MD-054 and MD-036 are duplicate trackers for substantially the **same missing source-level contract**. Their requirements must be integrated together into `formatter.md`, `projects.md`, and applicable CLI/LSP integration. Mark each tracker satisfied only after the rulebooks, relevant tests and governance agree; do not remove requirements merely by deleting a duplicate tracker.

## 5. Required tests

**5.1. MD-027 — Contextual literals and overloads:**

1. `None` used as an argument when the uniquely selected parameter is `Option[T]`.
2. `[]` used as an argument when a compatible empty collection parameter type is uniquely known.
3. Both cases through initializer and `new` argument resolution.
4. Overload selection by all input arguments, including an independently typed argument after a contextual literal.
5. `Store(None)` ambiguity with `Option[int]` and `Option[string]` overloads; verify actionable candidate diagnostics.
6. Equivalent `[]` ambiguity with distinct collection parameter types.
7. An explicit typed binding disambiguates an otherwise ambiguous call.
8. Equal parameter signatures with different return types are rejected as duplicate overloads.
9. Expected result/return context does not select among overload candidates.

**5.2. MD-033 — Imports:**

1. A single ungrouped import region sorts alphabetically by logical path.
2. Explicit blank-line-separated groups sort internally and do not exchange entries across groups.
3. Optional group headings appear immediately before first imports, with no empty line between heading and first import.
4. Detached, inline or ambiguously attached comments do not move to another group; preserve the affected region when necessary.
5. `Format(Format(source)) == Format(source)` for imports with and without groups.
6. Sorting does not require module lookup, an installed stdlib inventory or `CompilationPlan`.

**5.3. MD-036 / MD-054 — Manifest and CLI:**

1. Absent `[format]` applies all normative defaults, particularly `language_corrections = false`.
2. Partial `[format]` applies explicit values plus defaults for omitted keys.
3. Supported `[format]` keys parse with correct types and validation.
4. `sec fmt --indentation-width 2` overrides project `indentation_width = 4` without changing the manifest.
5. An omitted CLI option does not overwrite an explicit project setting.
6. A project may explicitly enable `language_corrections`; default runs do **not** enable it.
7. Unsupported effective `format_version`, invalid configuration or unknown fields fail before any source files are modified.
8. LSP and CLI share effective project settings; stand-alone files use defaults in the absence of a project.
9. Multiple supported indentation widths produce deterministic, idempotent formatting.
10. Existing mandatory canonical formatting still runs while optional language corrections are disabled.

## 6. Integration and closure

**6.1.** Integrate MD-027 into the existing expected-type, overload-resolution, `None`/`[]`, and constructor-argument sections of the owning rulebooks. Preserve existing public paragraph identifiers. Where the current compiler already does this, only its documentation and tests may need changes.

**6.2.** Integrate MD-033 by replacing the previous automatic import-class sorting requirement with a single alphabetic sort **within user-authored groups**. Preserve MD-055's independent comment-attachment rules. Remove obsolete requirements to detect standard-library membership for formatting.

**6.3.** Integrate MD-036 and MD-054 jointly. Add `[format]` to the project manifest schema and document the CLI > manifest > defaults precedence in the formatter and project rulebooks. **Explicitly verify or add `language_corrections = false` in the normative formatter defaults**; this is the user's pre-existing decision, not a new proposal.

**6.4.** Check and synchronize the relevant existing governance fragments and regression tests. Only then close or remove the entries from the authoritative most recently supplied `missing-decisions.yaml`, consistent with its removal contract. Do **not** replace that YAML or create a separate audit file.

**6.5.** The accessible public copy of `rules/tooling/formatter.md` checked when authoring this correction was older than the formatter rules described by the latest supplied `missing-decisions.yaml`. Consequently, this file **does not claim to have verified the current repository text**; apply changes against the actual latest canonical rulebook and avoid duplicate provisions.
