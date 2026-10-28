# Formatter

- **Status:** Normative
- **Created:** 2026-07-31
- **Last updated:** 2026-10-28 (MD-029/MD-042 error-pattern and inactive-arm preservation §§ 20(11)–20(12); MD-062 metadata spacing and preservation policy §§ 22(11)–22(13)); 2026-10-27 (MD-099 comment-column cap §§ 9(10a), 12(6a); MD-100 enum-only exemption § 15(4a); MD-037 `sec fmt` path selection §§ 28(9)–28(14)); 2026-10-18 (trailing single-line block comments, MD-097; direct unsafe call, MD-079)
- **Document revision:** 2.3
- **Sec language version:** 0.1
- **Canonical path:** `rules/tooling/formatter.md`
- **Replaces:** Earlier legacy revision at the same canonical path
- **Repository baseline reviewed:** `main-reviewed-2026-09-23`
- **Implementation governance:** `governance/formatting.yaml`
- **Related rulebooks:** `rules/tooling/testing.md`, `rules/tooling/lsp.md`, `rules/concurrency/await.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`

---

## § 1. Purpose and authority

**Governance tags:** `tooling.formatter-v2`

§ 1(1) This rulebook defines the canonical Sec 0.1 source formatting model.

§ 1(2) The formatter is part of the official Sec toolchain and defines the canonical presentation of source text for a selected formatting configuration.

§ 1(3) This rulebook owns:

- canonical whitespace and indentation;
- brace and block layout;
- vertical spacing;
- structural alignment;
- line-width and continuation behavior;
- comment formatting;
- import layout and ordering;
- declaration and expression layout;
- control-flow layout;
- formatting of ownership, unit, register, test, and other Sec-specific syntax;
- behavior on malformed and incomplete source;
- formatter configuration and format-version semantics;
- formatter invariants and conformance tests;
- the optional Language Corrections model.

§ 1(4) This rulebook does not redefine language semantics. When another normative rulebook owns the syntax or semantics of a construct, the formatter preserves that construct and applies only the layout rules defined here.

§ 1(5) Mutable implementation status belongs in the implementation-governance fragment, not in this normative rulebook.

§ 1(6) Examples in this revision use current Sec syntax only. Historical Sec syntax may appear only when it is explicitly shown as input to a Language Correction.

---

## § 2. Stable normative paragraph identifiers

**Governance tags:** `tooling.formatter-v2`

§ 2(1) Every normative paragraph identifier in this rulebook is a stable external reference.

§ 2(2) An identifier such as `§ 10(7)` may be referenced by compiler source, tests, diagnostics, governance files, documentation, or other rulebooks.

§ 2(3) Once a published revision assigns a normative identifier, later revisions must not renumber that identifier merely because surrounding text is inserted, removed, reorganized, or expanded.

§ 2(4) A removed normative paragraph leaves its identifier reserved. The identifier must not later be reused for an unrelated rule.

§ 2(5) New normative paragraphs added to an existing section use new paragraph identifiers and do not shift existing paragraph identifiers.

§ 2(6) Section identifiers are likewise stable after publication. New subject matter that cannot be added without renumbering existing sections must be appended in a way that preserves existing section identities.

§ 2(7) Editorial wording may change across document revisions, but references to an existing paragraph identifier must continue to identify the same normative subject.

§ 2(8) Document revision, Sec language version, formatting style version, and formatter implementation version are separate concepts.

---

## § 3. Canonical formatting model

**Governance tags:** `tooling.formatter-v2`

§ 3(1) Formatting is canonical for a selected project formatting configuration.

§ 3(2) Canonical formatting is idempotent:

```text
Format(Format(source)) == Format(source)
```

§ 3(3) Ordinary formatting must preserve program semantics.

§ 3(4) Whitespace is generally formatter-owned except where source text itself is data, where comments contain preserved preformatted material, where malformed source is protected by § 25, or where this rulebook explicitly preserves a clean layout choice.

§ 3(5) A clean permitted single-line form may remain single-line.

§ 3(6) A clean permitted multiline form may remain multiline even when it would fit on one line.

§ 3(7) A mixed or partially formatted layout is normalized to the canonical layout.

§ 3(8) Width or structural rules may force a single-line construct to become multiline.

§ 3(9) A wider viewport or larger effective width must not by itself collapse a deliberately clean multiline construct that this rulebook permits to remain multiline.

§ 3(10) The formatter must not rewrite literal data merely to satisfy line width.

§ 3(11) The general principle is:

> Break syntax, not data.

§ 3(12) Ordinary formatting does not perform semantic cleanup such as removing redundant parentheses, translating foreign-language syntax, changing literal spelling, or inferring programmer intent. Such transformations belong only to Language Corrections under § 26 and § 27.

---

## § 4. Formatter configuration and format version

**Governance tags:** `tooling.formatter-v2`

§ 4(1) The initial formatting style version is:

```text
format_version = 1
```

§ 4(2) `format_version` identifies the canonical formatting contract, not the formatter executable version and not this document revision.

§ 4(3) A formatter implementation update must not deliberately change canonical output for an existing `format_version`.

§ 4(4) A normative style change that intentionally changes canonical output requires a new `format_version` unless the change corrects an implementation that did not conform to the already-published format contract.

§ 4(5) The default project formatting configuration is:

```text
format_version = 1

vertical_style = "structured"

indentation_width = 4

width_mode = "fixed"
line_width = 120

adaptive_min_width = 80
adaptive_max_width = 200
adaptive_fallback_width = 120

max_alignment_padding = 4 × indentation_width

language_corrections = false
```

§ 4(6) `vertical_style` accepts:

```text
structured
compact
```

§ 4(7) `structured` is the default.

§ 4(8) `indentation_width` accepts `2`, `4`, or `6` spaces. Tabs are not canonical indentation.

§ 4(9) `width_mode = "fixed"` uses `line_width` as the preferred line width.

§ 4(10) `width_mode = "adaptive"` derives an effective width from the editor or presentation environment and clamps it to the inclusive range `adaptive_min_width..adaptive_max_width`.

§ 4(11) The default adaptive range is `80..200`.

§ 4(12) A formatter invocation with no usable adaptive viewport uses `adaptive_fallback_width`, whose default is `120`.

§ 4(13) Preferred line width is a target, not a source-language limit. An unbreakable or deliberately preserved construct may exceed it.

§ 4(14) The maximum alignment padding is the project setting `max_alignment_padding`, a number of spaces in `1..1024`. When it is omitted the default is four indentation widths; with the default indentation width this is sixteen spaces. A project whose ordinary, non-exempt declaration groups — for example eligible struct fields or `let` declarations whose names differ greatly in length — need a wider column may raise it so the group keeps one aligned column instead of § 9(10) dropping the alignment. Wide enum member names are not a reason to raise it: enum explicit-value groups are always aligned regardless of the limit (§ 15(4), MD-100). Zero, negative, out-of-range and non-integer values are rejected by manifest validation, never read as disabling alignment; there is no `sec fmt` command-line override, and `sec init` writes the key only as the commented example `# max_alignment_padding = 16`. The same cap applies to the trailing-comment column of every non-exempt group (§ 12(6), MD-099, decided 2026-10-27) (MD-098, decided 2026-10-18). Adding the key does not change the canonical output of a project that omits it, so it requires no new `format_version`.

§ 4(15) All configuration that changes canonical source bytes is project-owned. Personal editor preferences must not silently produce a different canonical project source form.

§ 4(16) Brace placement, colon attachment, operator placement, trailing-comma policy, and other rules explicitly fixed by this rulebook are normative and are not formatter preference knobs.

§ 4(17) `language_corrections` controls the optional correction layer. It is disabled by default and is separate from ordinary canonical formatting.

§ 4(18) The project formatting configuration is the `[format]` table of the project's `.sec/sec.toml` manifest (`projects.md` § 6(8)). This rulebook owns the meaning and domains of its keys, which are exactly the § 4(5) settings; the project rulebook owns its placement, loading and manifest validation (MD-036/MD-054, 2026-10-12):

```toml
[format]
format_version = 1
indentation_width = 4
vertical_style = "compact"
line_width = 120
language_corrections = false
```

§ 4(19) An absent `[format]` table applies every § 4(5) default. A table that omits a setting applies that setting's default while its explicit settings remain in effect. A missing project manifest never makes the formatter take values from the host or an unrelated project.

§ 4(20) The effective options are resolved with this precedence, highest first: explicitly supplied formatter invocation options; the selected project's `[format]` settings; the § 4(5) defaults. An invocation option that was not supplied never shadows a project setting, and an explicit option never rewrites the manifest. For example, `sec fmt --indentation-width 2 source.sec` formats with width 2 although the project sets `indentation_width = 4`; four is the default, not a language-wide requirement. Enabling `language_corrections` requires an explicit project setting or invocation choice (`sec fmt --fix`); invoking `sec fmt` alone never enables it.

§ 4(21) When the effective `format_version` is unsupported, the formatter reports a diagnostic and modifies no source file. Unknown keys, wrongly typed values and values outside their domains are diagnostics as well, never ignored or guessed. Validation completes before any file of the requested operation is changed.

§ 4(22) The CLI formatter and the LSP resolve and validate the same project manifest through shared configuration code. A file that belongs to a Sec project uses that project's `.sec/sec.toml`; a stand-alone file uses the defaults plus explicit invocation options. Project membership follows the existing project-root and workspace rules; editor-provided formatting preferences never override the project configuration (§ 4(15)).

§ 4(23) Formatting is deterministic for the tuple (source, effective options, formatter version).

---

## § 5. Required syntax representation

**Governance tags:** `tooling.formatter-v2`

§ 5(1) A conforming formatter must operate from an error-tolerant, lossless syntax representation sufficient to reconstruct all source bytes that matter to formatting and preservation.

§ 5(2) The representation must preserve at least:

- source tokens and their exact lexemes;
- whitespace trivia;
- line comments;
- block comments;
- raw or otherwise source-sensitive string text;
- exact source ranges;
- malformed regions;
- parser recovery structure sufficient to distinguish proven syntax from uncertain syntax.

§ 5(3) A semantic AST alone is insufficient as the formatter source of truth because ordinary ASTs discard layout and may discard source text that must be preserved.

§ 5(4) The compiler may produce CST and AST as parallel outputs from shared lexer/parser logic.

§ 5(5) Sec must not maintain separate, diverging grammars merely to support formatting.

§ 5(6) Ordinary formatting must not require semantic analysis.

§ 5(7) Import sorting is purely syntactic: it consults no module classification metadata, module lookup, installed standard-library inventory or `CompilationPlan` (§ 13(7); MD-033, 2026-10-12).

§ 5(8) The concrete CST storage implementation is not defined by this rulebook. The behavioral requirements in this section and § 25 are normative.

---

## § 6. Indentation and basic whitespace

**Governance tags:** `tooling.formatter-v2`

§ 6(1) Canonical indentation uses spaces only.

§ 6(2) Each nested structural block increases indentation by one configured indentation width.

§ 6(3) Continuation indentation is structural and must remain visually subordinate to the construct it continues.

When a returned expression starts on the same line as `return`, its subsequent
continuation lines use one additional configured `indentation_width` relative
to the `return` statement. Four spaces is the default, not a fixed width.
The first operand remains beside `return`; existing operator placement and
line breaks are preserved. For example, with the default configuration:

```sec
    return character in 9r..13r ||
        character == 32r ||
        character == 133r
```

Nested delimited expressions retain their relative structural indentation.
A delimiter opened on the `return` line retains its ordinary interior and
closing-delimiter layout; its existing indentation is not increased again.
Comments follow the expression they continue, while multiline literal contents
remain byte-for-byte unchanged. A following statement is not a continuation.

The same configured continuation step applies to multiline `if` and `else if`
conditions that start beside their keyword. Continuations are relative to the
header line's structural indentation, including a header beginning `} else if`.
The condition's continuation indentation does not increase the body or a
subsequent statement's indentation. For example:

```sec
    } else if character == 10r
        || character == 133r
        || character == 8232r
        || character == 8233r {
        HandleWhitespace()
    }
```

§ 6(4) A colon in a declaration binds to the identifier on its left:

```sec
let host: string := "localhost"
```

not:

```sec
let host : string := "localhost"
```

§ 6(5) Binary operators normally have one space on both sides in single-line expressions:

```sec
let total := base + tax
```

§ 6(6) Unary operators attach to their operand according to the language grammar:

```sec
!ready
-value
```

§ 6(7) Assignment and initialization operators use surrounding spaces:

```sec
value = next
let count := 3
```

§ 6(8) Trailing whitespace is not canonical except where it is literal source data inside a preserved source-sensitive construct.

---

## § 7. Vertical spacing

**Governance tags:** `tooling.formatter-v2`

§ 7(1) The formatter never emits more than one consecutive empty line in ordinary formatted source.

§ 7(2) The formatter does not place an empty line immediately inside an opening or closing brace merely for decoration.

§ 7(3) In `structured` style, major declaration groups receive structural separation where defined by this rulebook.

§ 7(4) In `compact` style, optional structural blank lines are removed while required separation and comment attachment remain intact.

§ 7(5) Aggregate or group type declarations are separated from surrounding unrelated declarations in `structured` style.

§ 7(6) Adjacent simple named-type declarations may remain a compact declaration block when they form one homogeneous group.

§ 7(7) A doc-commented function, method, property, test declaration, or similar major declaration receives structural separation in `structured` style unless it is the first declaration in the enclosing region or the documentation is file-level documentation.

§ 7(8) Documentation comments remain attached to the declaration they document.

§ 7(9) A deliberate standalone `//` comment may divide two declaration groups and therefore ends alignment across that boundary.

§ 7(10) In `structured` style, a multiline control-flow block may be separated from surrounding ordinary statements when the separation improves structural reading.

§ 7(11) Consecutive simple declarations may remain together.

§ 7(12) Consecutive `defer` blocks remain together unless an intervening comment or syntax requires separation.

§ 7(13) In `structured` style, a very long outlier may start a new alignment group when aligning it with the previous group would create excessive whitespace. `compact` style removes that optional blank line.

---

## § 8. Braces and executable blocks

**Governance tags:** `tooling.formatter-v2`

§ 8(1) Opening braces for declarations and control-flow constructs are placed on the same line as the construct header whenever the header itself is not structurally multiline.

§ 8(2) Canonical `if`/`else` layout is:

```sec
if condition {
    First()
} else {
    Second()
}
```

§ 8(3) `} else {` is canonical and must not be split into three independent lines.

§ 8(4) Executable brace blocks are multiline even when they contain only one statement.

§ 8(5) The formatter rewrites a one-line executable block such as:

```sec
defer { Close() }
```

into:

```sec
defer {
    Close()
}
```

§ 8(6) Aggregate value literals are not executable blocks and may use a clean permitted single-line representation where the grammar and width rules allow it.

§ 8(7) Empty structural declaration blocks may remain compact:

```sec
type Marker struct {}
interface Empty {}
```

§ 8(8) An empty executable block follows the ordinary executable-block rules of its construct and must not be reformatted as an aggregate value literal.

---

## § 9. Structural horizontal alignment

**Governance tags:** `tooling.formatter-v2`

§ 9(1) Alignment is based on syntactic anchors, not arbitrary character columns.

§ 9(2) Compatible declarations in one contiguous homogeneous group may align identifier, type, initializer, tag, or trailing-comment anchors.

§ 9(3) Example:

```sec
let host:    string   := "localhost"
let port:    uint16   := 8080
let timeout: Duration := 30<s>
```

§ 9(4) `mut` may occupy a structural modifier column when a mixed declaration group benefits from it:

```sec
let     host:    string   := "localhost"
let mut port:    uint16   := 8080
let     timeout: Duration := 30<s>
```

§ 9(5) Struct fields follow the same structural alignment principle:

```sec
type Endpoint struct {
    Host:    string,
    Port:    uint16,
    Timeout: Duration,
}
```

§ 9(6) Struct field tags are secondary alignment anchors and remain attached to the field before the field delimiter required by the grammar.

§ 9(7) Named-value groups align `:=` where useful:

```sec
Distance (
    TellusLuna := 1<unit>
    TellusSol  := 10<unit>
)
```

§ 9(8) Blank lines and standalone comments terminate an alignment group. A trailing comment belongs to its source item and does not terminate the group. Documentation comments and other structural group boundaries already defined by this rulebook retain their semantics (MD-013; `rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` §§ 5.7–5.11). A one-line `/* ... */` that follows code and ends its source line is a trailing comment exactly like `// ...` (§ 12(9a); MD-097).

§ 9(8a) In declaration alignment the colon remains attached to the identifier; alignment never introduces whitespace between an identifier and its colon, and padding follows the complete `identifier:` cell.

§ 9(8b) Trailing comments occupy the next structural column of the local declaration-table layout. Within the compatible alignment group, each following column begins after the standard spacing in effect from the widest preceding cell, the same single space that separates `identifier:` from its type in the widest row; shorter cells receive the padding needed to reach that column:

```sec
type Point struct {
    x:        int, // horizontal
    longName: int,
    y:        int, // vertical
}
```

The spacing before a trailing comment is never hard-coded independently of that standard spacing (MD-013; `rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` §§ 5.12–5.19).

§ 9(9) Primary syntax anchors take precedence over distant secondary columns. Alignment must not create large empty gaps merely to align a far-right tag or comment.

§ 9(10) If required alignment padding exceeds the configured maximum alignment padding, the formatter drops candidate alignment columns for the complete contiguous group rather than producing extreme spacing. Enum value groups are exempt (§ 15(4)).

§ 9(10a) *Per-column decisions (MD-099, 2026-10-27).* The limit is decided separately for each candidate column of a group, and the trailing-comment column is one of those columns. Dropping only the comment column keeps an otherwise legal type or initializer column. A column that was already dropped is never recreated by a later pass, including the trailing-comment pass. Struct fields, bindings, parameters, register fields, named types and other non-enum groups have no exemption from the limit.

§ 9(11) Properties with bodies are behavioral declarations and are not horizontally aligned across separate property declarations.

§ 9(12) Functions and methods are not horizontally aligned against neighboring functions or methods.

§ 9(13) Multiline infix expressions use a stable operand column with leading operators:

```sec
let total :=
      basePrice
    + shipping
    - discount
```

§ 9(14) Logical expressions use the same principle:

```sec
let ready :=
      hasConfiguration
   && hasConnection
   && !isStopping
```

§ 9(15) The leading-operator layout applies to ordinary arithmetic, logical, comparison, bitwise, and string-concatenation expressions when those expressions are broken across lines. Inline-start returned expressions and `if`/`else if` conditions follow the continuation layout in § 6(3) instead, preserving operator placement.

---

## § 10. Preferred width and wrapping

**Governance tags:** `tooling.formatter-v2`

§ 10(1) Preferred width is a formatting target, not a hard source limit.

§ 10(2) The formatter first breaks at syntactic boundaries.

§ 10(3) String literal contents are never split or rewritten merely to satisfy preferred width.

§ 10(4) Once an ordinary comma-separated syntactic list becomes multiline, it uses its complete multiline layout rather than a half-broken mixture, except where this rulebook explicitly permits packed simple scalar collection elements.

§ 10(5) Simple scalar collection values may pack multiple elements on a line when multiline layout remains readable and width permits it.

§ 10(6) Complex collection elements use one element per line once the collection is multiline.

§ 10(7) Alignment yields to structural wrapping when alignment would force unreasonable width or padding.

§ 10(8) Existing line breaks are not generally authoritative, except that a clean permitted multiline construct may remain multiline under § 3(6).

§ 10(9) Long type expressions may use a hanging continuation after `:=` or another enclosing syntactic anchor rather than forcing a very wide declaration line.

§ 10(10) A construct that cannot be broken without reducing clarity may exceed preferred width.

---

## § 11. Delimiters, comma-separated lists, and trailing commas

**Governance tags:** `tooling.formatter-v2`

§ 11(1) A single-line comma-separated list has no trailing comma.

§ 11(2) A multiline comma-separated list has a trailing comma when the grammar permits a trailing comma for that list.

§ 11(3) A multiline closing delimiter appears on its own aligned line.

§ 11(4) The opening delimiter remains attached to its construct unless the construct-specific grammar requires another form.

§ 11(5) A trailing comma is not an independent permanent multiline marker. If a construct canonically becomes single-line, the trailing comma is removed.

§ 11(6) A clean permitted multiline layout may remain multiline even if it would fit on one line; in that case its multiline trailing-comma rule remains in force.

§ 11(7) The formatter never invents comma syntax for a construct whose grammar is comma-free.

§ 11(8) In particular, named-value groups and register fields follow their own grammar and do not inherit comma policy from calls, arrays, structs, or parameter lists.

---

## § 12. Comments

**Governance tags:** `tooling.formatter-v2`

§ 12(1) A normal line comment begins with `// ` when it contains text.

§ 12(2) An empty line-comment paragraph is written:

```sec
//
```

§ 12(3) Consecutive line comments form one comment block unless syntax or an empty line separates them.

§ 12(4) Trailing line comments, and trailing single-line block comments (§ 12(9a)), align in the structural column after the code columns of one contiguous homogeneous declaration group (§ 9(8b)).

§ 12(5) A blank line, standalone comment, or incompatible declaration terminates trailing-comment alignment. A trailing comment itself does not terminate it.

§ 12(6) Trailing-comment alignment obeys the maximum alignment padding rule.

§ 12(6a) *Group-wide comment-column cap (MD-099, 2026-10-27; `rules/corrections/applied/MD-099-MD-100-MD-037-formatter-alignment-and-selection-correction-20261028.md` § 2).* The candidate comment column is decided for each non-exempt group as follows:
- **Candidate column.** It lies one space after the widest formatted code cell of the group. Every row counts, including rows without a comment and any columns already accepted. The text of a comment never moves the column.
- **Rejection.** If any commented row would need more padding than the effective `max_alignment_padding`, the column is rejected for the whole group. Padding exactly equal to the cap is allowed.
- **After rejection.** Every trailing comment then follows its code after one space. No subset of rows is aligned, and no ad hoc gaps remain.
- **Scope.** This applies to `//` comments and to trailing single-line `/* ... */` comments (§ 12(9a)) alike. Enum explicit-value groups are exempt (§ 15(4a)).

§ 12(7) The formatter does not aggressively reflow ordinary `//` prose.

§ 12(8) A multiline block comment uses aligned star form:

```sec
/*
 * Text
 */
```

§ 12(9) A single-line block comment may remain:

```sec
/* text */
```

§ 12(9a) A one-line `/* ... */` that appears after code on the same source line and ends that line is a **trailing comment**, treated exactly like a trailing `// ...` comment: both spellings share the same structural trailing-comment column and alignment-group behavior, and a trailing block comment does not terminate a compatible declaration or enum explicit-value group (§§ 9(8), 9(8b), 12(4)–12(6), 15(4)). A block comment followed by more code on its line is an **inline comment** and one beginning after a line break is a **standalone comment**; their handling is unchanged. Width limits, padding policy, group boundaries and the enum exemption continue to apply. This clarifies the single structural alignment rule; it is not a second comment-alignment model (MD-097, 2026-10-18):

```sec
enum Example {
    Short     = 1  /* first */
    LongerKey = 20 // second
    End       = 3  /* third */
}
```

§ 12(10) Documentation block comments use the same visual multiline alignment while retaining their semantic documentation form:

```sec
/**
 * Summary.
 */
```

§ 12(11) In a multiline block comment, paragraph separation uses one empty comment line:

```sec
/*
 * First paragraph.
 *
 * Second paragraph.
 */
```

§ 12(12) Block-comment indentation follows surrounding syntax.

§ 12(13) Prose in a block comment may be reflowed according to preferred width when doing so does not alter preformatted material.

§ 12(14) Preformatted comment content is preserved rather than reflowed.

§ 12(15) Documentation comments remain immediately attached to the declaration or member they document.

§ 12(16) Comments documenting enum variants, union variants, public members, or similar declarations appear before the declaration they describe and are never moved after it by the formatter.

---

## § 13. Imports

**Governance tags:** `tooling.formatter-v2`

§ 13(1) Exactly one import may remain in single-import form:

```sec
import "fmt"
```

§ 13(2) Two or more imports are formatted as one import region:

```sec
import (
    "fmt"
    "net/http"
    sys "platform/linux"
)
```

§ 13(3) The formatter may gather imports that are scattered among top-level declarations into the canonical import region.

§ 13(4) The formatter must never remove an import merely because it appears unused. Unused-import removal is not formatting.

§ 13(5) Imports are sorted alphabetically by their logical import path, not by alias spelling.

§ 13(6) Comments attached to an import move with that import when imports are reordered.

§ 13(7) The formatter does not classify imports as standard-library, project, external or platform modules, and does not need module resolution or a `CompilationPlan` to sort them (MD-033, 2026-10-12). Import-path validity, such as `core` needing no import and standard-library modules being imported by logical path (`"net/http"`, not `"stdlib/net/http"`), remains owned by `modules.md` and gives the formatter no sorting category.

§ 13(8) An intentional empty line in a grouped import region is an explicit group boundary. The formatter sorts imports within each group and never merges, moves or reorders imports across group boundaries.

§ 13(9) Without explicit groups, the imports form one alphabetic sequence in both vertical styles. The formatter never invents groups, headings or blank lines from a guessed import origin.

§ 13(10) Optional `//` comments may describe groups (§ 13(11)–(14)); they document groups for readers and are not parser directives or import-class identifiers:

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

§ 13(11) A standalone group-heading comment that starts a blank-line-separated run of a grouped import declaration, and is unambiguously attached to the imports that immediately follow it, is placed directly above the first import of its group, with no empty line between the heading and that import. Distinct commented groups are separated by one empty line (MD-055, 2026-10-12):

```sec
import (
    // Network libraries
    "net/http"
    "net/url"

    // File libraries
    "fs"
    "io"
)
```

§ 13(12) A group heading belongs to its group, not to an individual import. Sorting and regrouping never move an import across a group boundary or attach a heading to another import; ordinary comments attached to one import travel with it under § 13(6).

§ 13(13) When comment attachment or group boundaries are ambiguous, the formatter preserves the order of the affected import region and the comments' relative positions. It may apply unambiguous local whitespace and indentation changes but does not guess a comment's owner or invent a grouping: a genuinely detached comment, a trailing group comment, or an inline block comment is not reclassified as a heading.

§ 13(14) When a heading relationship is unambiguous, an unnecessary empty line between the heading and its first import is removed. Indentation inside `import (...)` uses the configured `indentation_width`; the example's four spaces are the default. Formatting of headed groups is deterministic and idempotent. This does not authorize reordering across comments or rewriting comment text.

---

## § 14. Top-level file layout

**Governance tags:** `tooling.formatter-v2`

§ 14(1) The canonical major top-level order is:

```text
module
import region
declarations in source order
```

§ 14(2) A module declaration occupies its own line.

§ 14(3) The import region follows the module declaration and precedes ordinary declarations.

§ 14(4) Ordinary top-level declarations remain in source order.

§ 14(5) The formatter must not sort functions, types, tests, properties, implementations, or other ordinary top-level declarations.

§ 14(6) Existing vertical-spacing rules apply between top-level declarations.

§ 14(7) This rulebook does not invent formatting behavior for compiler-directive syntax that is not yet defined by the canonical Sec grammar.

---

## § 15. Type declarations and aggregate syntax

**Governance tags:** `tooling.formatter-v2`

§ 15(1) A non-empty structural declaration block such as a struct, enum, union, register, interface, or implementation block is multiline.

§ 15(2) An aggregate value literal is distinct from a structural declaration block and may use a clean permitted single-line or multiline representation.

§ 15(3) Struct fields use the ordinary declaration alignment rules of § 9.

§ 15(4) Enum explicit-value assignments align their `=` anchors when they form a homogeneous contiguous group and the grammar uses `=` for that declaration form. An enum value group is always aligned: the § 9(10) maximum alignment padding does not apply to it, so members of very different name lengths still share one value column, and their trailing comments — `//` and trailing single-line `/* ... */` alike (§ 12(9a)) — share one comment column after the widest value cell (§ 9(8b)). Only a blank line, a standalone comment, an implicit member or a declaration boundary ends the group (MD-100, user decision 2026-10-17).

§ 15(4a) *Scope of the enum exemption (MD-100, decided 2026-10-27).* The mandatory alignment of § 15(4) and its uncapped comment column apply only to enum explicit-value groups. Struct fields, named and typed bindings, function and method parameters, register fields, named-type declarations and every other group keep the ordinary candidate-column rules and the cap of §§ 9(10), 12(6a). `max_alignment_padding` keeps its domain, default and ownership (§ 4(14)). There is no `enum_alignment` setting, separate comment-padding option or source directive.

§ 15(5) Documentation comments remain before the member or variant they document and break alignment groups when appropriate.

§ 15(6) A generic parameter or generic argument list remains single-line if it cleanly fits and has not been deliberately written in a permitted multiline form.

§ 15(7) A multiline generic list uses one element per line and a trailing comma when the grammar defines the list as comma-separated:

```sec
Result[
    ValueType,
    ErrorType,
]
```

§ 15(8) A clean deliberately multiline generic list may remain multiline even if later width conditions would allow it to collapse.

§ 15(9) A short `implements` list may remain on the declaration header:

```sec
type Car struct implements Vehicle, Serializable, Inspectable {
    ...
}
```

§ 15(10) When width requires continuation, an `implements` list may use a compact continuation:

```sec
type Car struct implements Vehicle,
    Serializable, Inspectable {
    ...
}
```

§ 15(11) A deliberately vertical `implements` list is also permitted:

```sec
type Car struct implements
    Vehicle,
    Serializable,
    Inspectable {
    ...
}
```

§ 15(12) The formatter must not create an extreme hanging indent merely to align every continued interface name under the first interface token.

---

## § 16. Functions, methods, properties, lambdas, and attributes

**Governance tags:** `tooling.formatter-v2`

§ 16(1) A function or method signature remains single-line when it cleanly fits and has not been deliberately written in a permitted multiline form.

§ 16(2) A multiline parameter list uses one parameter per line with a trailing comma:

```sec
fn Connect(
    host: string,
    port: uint16,
    timeout: Duration,
) Result[Connection, ConnectError] {
    ...
}
```

§ 16(2a) Under § 3(7) a parameter list whose first parameter follows a line break after `(` is a deliberate multiline form and is completed to the § 16(2) layout, while a mixed list whose first parameter shares the line of `(` is normalized under § 16(1): single-line when it fits the preferred width, otherwise the § 16(2) layout. A blank line between parameters is an alignment-group boundary (§ 9(8)) and keeps the list multiline.

§ 16(3) Compatible parameter declarations may align their `Name: Type` structure when the alignment remains within the configured padding limit.

§ 16(4) The return type follows the closing parameter delimiter. If the complete signature becomes structurally multiline, the return type follows the ordinary long-type and width rules rather than introducing a separate arrow syntax.

§ 16(5) Sec methods use the language's implicit receiver model. The formatter must not invent an explicit receiver parameter.

§ 16(6) A function or method body is an executable block and therefore follows § 8.

§ 16(7) Properties use ordinary declaration spacing:

```sec
property Name: string {
    get {
        return _name
    }

    set value {
        _name = value
    }
}
```

§ 16(8) In `structured` style, accessors in one property may be separated by one structural blank line. `compact` removes that optional blank line.

§ 16(9) A fallible setter keeps `try set` together:

```sec
try set value {
    ...
}
```

§ 16(10) A lambda uses ordinary `fn` syntax and ordinary function formatting:

```sec
let double := fn(value: int) int {
    return value * 2
}
```

§ 16(11) Explicit capture syntax remains attached to the lambda:

```sec
let multiply := capture(factor) fn(value: int) int {
    return value * factor
}
```

§ 16(12) A multiline capture list uses one capture per line with a trailing comma. Captures are not packed across multiple entries once the capture list is multiline.

§ 16(13) Function types use the normal function-type syntax and the same parameter-list breaking rules:

```sec
fn(int, string) bool
```

§ 16(14) Attributes are written one per line and immediately before the declaration they modify. Multiple attributes, when present, each occupy their own line. Example:

```sec
@build(target: "linux")
fn Example() void {
    ...
}
```

§ 16(15) Attributes remain in source order. The formatter never sorts attributes.

§ 16(16) A long attribute argument list follows ordinary call formatting.

§ 16(17) Documentation, attributes, and the declaration appear in this order with no blank line between them:

```sec
/**
 * Summary.
 */
@build(target: "linux")
fn Example() void {
    ...
}
```

§ 16(18) Function modifiers remain compact on the function declaration line where the grammar permits them:

```sec
unsafe fn ReadRaw() void {
    ...
}
```

```sec
extern "C" fn write(
    fd: int32,
    buffer: RawPtr[byte],
    length: uint,
) int64
```

§ 16(19) The formatter does not reorder modifiers independently of grammar.

§ 16(20) Valid C foreign qualification is always emitted as `c::name` or `c::namespace::name`, including `c::fn(...)`, `c::flex[T]`, and `c::callback(...)`, with no space inside or around `::`. Formatting is idempotent. The formatter does not rewrite separated `:` tokens into `::`, does not change the meaning of invalid source, and does not rewrite the removed `C::` spelling into `c::` unless an explicitly enabled Language Correction defines that migration (MD-020, `rules/corrections/applied/md020-md021-md023-md024-ffi-c-bindings-null-correction-20261008.md` §§ 2.5, 7.2):

```sec
extern "C" fn GetVersion(length: c::stddef::size_t) c::int
```

---

## § 17. Calls, expressions, chains, indexing, and slicing

**Governance tags:** `tooling.formatter-v2`

§ 17(1) A call remains single-line when it cleanly fits and has not been deliberately written in a permitted multiline form.

§ 17(2) A multiline call uses one argument per line with a trailing comma:

```sec
Connect(
    host,
    port,
    timeout,
)
```

§ 17(3) Calls do not use a half-packed multiline layout.

§ 17(4) Named call arguments may align their structural anchors when they form one homogeneous group and the padding limit permits it.

§ 17(5) A nested inner call may remain single-line when it independently fits even when its surrounding call is multiline.

§ 17(6) A multiline member or property chain uses leading dots:

```sec
let result :=
    client
        .Request()
        .WithHeader(name, value)
        .WithTimeout(timeout)
        .Send()
```

§ 17(7) Each continued chain segment occupies its own structural continuation line unless a segment's own syntax requires a multiline call.

§ 17(8) A multiline call inside a chain follows the ordinary multiline call rules.

§ 17(9) Ordinary indexing remains compact:

```sec
values[index]
matrix[row][column]
```

§ 17(10) If an index expression itself must break, the index delimiters become structural:

```sec
values[
    CalculateIndex(
        first,
        second,
    )
]
```

§ 17(11) The indexing delimiter level does not acquire a trailing comma merely because the inner expression contains comma-separated syntax.

§ 17(12) Slicing remains compact around the range expression:

```sec
values[start..end]
```

§ 17(13) Ordinary formatting never removes parentheses merely because they appear redundant. Parenthesis removal is a Language Correction and only applies when enabled and unambiguous.

---

## § 18. Control flow

**Governance tags:** `tooling.formatter-v2`

§ 18(1) `if`, `while`, `for`, `match`, and `switch` use same-line opening braces.

§ 18(2) Parentheses are not introduced around ordinary Sec conditions by the formatter.

§ 18(3) A `match` with short homogeneous arms may align `=>` when the padding limit permits:

```sec
match value {
    Some(item) => Use(item)
    None       => HandleMissing()
}
```

§ 18(4) A block match arm uses an executable block:

```sec
match value {
    Some(item) => {
        Prepare(item)
        Use(item)
    }

    None => {
        HandleMissing()
    }
}
```

§ 18(5) In `structured` style, block match arms may be separated by one blank line. `compact` removes that optional separation.

§ 18(6) Short match arms remain grouped tightly.

§ 18(7) `switch` case alternatives remain comma-separated according to the switch grammar:

```sec
switch value {
    case 1, 3, 5, 10..<20:
        Selected()

    case >= 100:
        Large()

    default:
        Other()
}
```

§ 18(8) When a switch case alternative list becomes too long, continuation lines hang structurally under the case header rather than creating a large alignment column.

§ 18(9) The case `:` follows the final alternative. The formatter does not insert a trailing comma immediately before the case colon.

§ 18(10) A switch case body is indented one structural level from its case label.

§ 18(10a) Inside `switch`, each `case` or `default` label is indented one configured indentation level relative to the `switch` statement, its body one further configured level, and the closing `}` aligns with `switch`. The number of spaces per level is the configured `indentation_width` (§ 4(5)); the § 18(7) example shows the default four spaces, not a fixed width (MD-032, 2026-10-12).

§ 18(11) In `structured` style, switch cases may be separated by one blank line. `compact` removes that optional separation.

§ 18(12) Canonical loop forms include:

```sec
for item in items {
    Use(item)
}
```

```sec
for {
    Poll()
}
```

```sec
while running {
    Poll()
}
```

§ 18(13) Sec formatting does not invent a C-style `for` clause.

§ 18(14) Long loop or condition expressions follow ordinary expression continuation rules.

§ 18(15) `break` and `continue` require no formatter-specific layout beyond ordinary statement indentation.

---

## § 19. `try`, `defer`, and `unsafe`

**Governance tags:** `tooling.formatter-v2`

§ 19(1) `try` propagation remains an ordinary prefix expression:

```sec
let data := try Read(path)
```

§ 19(2) A `try` handler block uses ordinary block and match-arm formatting:

```sec
let data := try Read(path) {
    Err(error) => return Err(error)
}
```

§ 19(3) Short homogeneous handler arms may align `=>` under the same padding limits as `match` arms.

§ 19(4) Handler arms with blocks follow the ordinary match-arm block rules.

§ 19(5) The formatter preserves an explicit surrounding `match` when the source contains one and never adds or removes a semantic `match` merely to restyle `try`.

§ 19(6) A long `try` operand call uses ordinary multiline call formatting. The handler opening brace follows the completed operand expression.

§ 19(7) Sec 0.1 `defer` formatting is block-based:

```sec
defer {
    Close()
}
```

§ 19(8) A one-line `defer` executable block is expanded under § 8.

§ 19(9) Consecutive defers remain together.

§ 19(10) In `structured` style, a defer that closes a setup sequence may remain attached to that setup, while a following unrelated work sequence may receive one structural blank line.

§ 19(11) An `unsafe` executable block follows ordinary executable-block formatting:

```sec
unsafe {
    RawOperation()
}
```

§ 19(12) An `unsafe fn` declaration follows ordinary modifier and function formatting.

§ 19(13) The formatter must not widen or narrow an unsafe boundary by moving operations into or out of an `unsafe` block.

§ 19(14) The direct single-call authorization `unsafe Call()` (`rules/memory/unsafe.md` § 4(9)–(13)) has exactly one space between `unsafe` and the call; the call keeps its ordinary member, generic-argument and argument formatting. The formatter never rewrites a single-operation form into a block or a one-call block into the direct form as a stylistic preference (MD-079, 2026-10-18):

```sec
unsafe pointer.Write(value)
let value := unsafe pointer.Read() + SafeAdjustment()
```

---

## § 20. Patterns and destructuring

**Governance tags:** `tooling.formatter-v2`

§ 20(1) Simple nested patterns remain compact when they fit:

```sec
Some(value)
Some(SameSite.Strict)
Ok(Some(value))
```

§ 20(2) Pattern delimiters do not acquire interior padding:

```sec
Some(value)
```

not:

```sec
Some( value )
```

§ 20(3) A comma-separated pattern payload that becomes multiline follows the ordinary complete multiline-list rule:

```sec
Variant(
    first,
    second,
)
```

§ 20(4) A clean struct pattern may remain single-line when permitted by width and grammar:

```sec
Point { X: x, Y: y }
```

§ 20(5) A multiline struct pattern follows aggregate structural alignment:

```sec
Point {
    X: x,
    Y: y,
}
```

§ 20(6) A multiline pattern used as a match arm completes before the arm separator:

```sec
Some(
    VeryLongPattern(
        first,
        second,
    ),
) => {
    Process()
}
```

§ 20(7) A destructuring declaration may keep its initializer after the closing pattern delimiter when that line remains readable:

```sec
let Point {
    X: x,
    Y: y,
} := point
```

§ 20(8) If the initializer must also break, it follows ordinary expression continuation:

```sec
let Point {
    X: x,
    Y: y,
} :=
    CalculatePoint(
        first,
        second,
    )
```

§ 20(9) `_` is formatted as an atomic wildcard pattern.

§ 20(10) Ownership-bearing pattern syntax is preserved exactly according to the canonical pattern grammar. The formatter must never add or remove ownership or borrow markers merely for style.

§ 20(11) MD-029 error patterns are preserved as written: `Err(.Variant)`, `Err(.Variant(info))`, `Err(.Variant { Field: name })` and the explicit non-binding forms `Variant {}` and `Variant(_)`. The formatter never expands a contextual `.Variant` to its qualified form or the reverse, never inserts or removes the empty `{}` or `(_)`, and never rewrites a variant because of reachability or payload analysis. The braces of a field pattern inside `Err(...)` are kept distinct from the braces of a `try` handler list, `match` body or arm body, and formatting is idempotent.

§ 20(12) A `match` arm or `try` handler reported as proven inactive (MD-042) is never removed, reordered or commented out by the formatter (`rules/control-flow/flowcontrol_match.md` § 25A).

---

## § 21. Ownership, borrow, and consuming markers

**Governance tags:** `tooling.formatter-v2`

§ 21(1) A consuming parameter uses the canonical parameter marker before the parameter name:

```sec
fn ConsumingFire(->buffer: Buffer) void {
    ...
}
```

§ 21(2) The `->` consuming-parameter marker attaches directly to the parameter name:

```sec
->buffer
```

§ 21(3) The formatter must not produce whitespace between `->` and the parameter name.

§ 21(4) A consuming call-site marker attaches directly to the consumed named value:

```sec
ConsumingFire(<-buffer)
```

§ 21(5) The formatter must not produce whitespace between `<-` and its operand binding.

§ 21(6) Multiline calls preserve the same ownership spelling:

```sec
Send(
    context,
    <-buffer,
    timeout,
)
```

§ 21(7) The formatter never inserts a consuming marker merely because a type is move-only or a callee parameter is consuming.

§ 21(8) Fresh temporaries that do not require a source move marker remain unmarked:

```sec
Send(CreateBuffer())
```

§ 21(9) Return sites do not acquire `<-` from formatting.

§ 21(10) Borrow type syntax follows ordinary type formatting:

```sec
source: ref Buffer
target: ref mut Buffer
```

§ 21(11) Alignment may treat ownership or mutability markers as structural prefix information, but must never separate a marker from the identifier or type component to which the grammar attaches it.

---

## § 22. Registers and units

**Governance tags:** `tooling.formatter-v2`

§ 22(1) A register declaration is a structural declaration block and follows the ordinary indentation and alignment rules.

§ 22(2) Register fields do not inherit struct comma policy. The formatter must not invent commas between register fields.

§ 22(3) Example:

```sec
type MotorProtocol register[8] {
    Speed:   bit[4]<rpm>
    Enabled: bit
    _:       bit[3]
}
```

§ 22(4) Reserved `_` register fields participate in structural alignment like other register fields.

§ 22(4a) Register fields are clients of the same local structural alignment engine as struct and declaration fields, not of a register-specific policy: `:` stays attached to the field identifier, the type or bit-field column aligns locally, trailing comments may occupy the aligned trailing-comment column, blank lines and standalone comments terminate the group, and trailing comments do not:

```sec
type Status register[16] {
    Ready:     bit
    ErrorCode: bit[4]  // error code
    _:         bit[11]
}
```

(MD-013; `rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` §§ 5.20–5.25, 9.4.)

§ 22(5) Address attributes follow ordinary attribute formatting. Canonical platform-aware examples use symbolic platform addresses where available:

```sec
@address(ports.USB3)
let mut usbControl: USBControl
```

§ 22(6) The formatter does not replace a symbolic address with a numeric address and does not replace a numeric address with a symbolic address. Address selection is not formatting.

§ 22(7) Unit declarations follow ordinary declaration spacing:

```sec
unit rpm uint physical
unit SEK decimal other
```

§ 22(8) Operators inside a unit expression enclosed by the unit annotation syntax are compact:

```sec
decimal<m/s>
bit[4]<rpm>
```

§ 22(9) The formatter must not produce ordinary runtime-expression spacing inside a unit annotation:

```sec
decimal<m / s>
```

is not canonical.

§ 22(10) Runtime arithmetic remains ordinary expression syntax and therefore retains ordinary operator spacing:

```sec
let speed := distance / time
```

§ 22(11) Unit metadata declarations use `Key: value` with one space after the colon. Arithmetic in exact numeric metadata uses ordinary binary-operator spacing and attached unary signs, for example `Scale: -(1 / 60)` and `Offset: 273.15`. Numeric metadata powers use binary-operator spacing (`2 ^ -3`); dimension axis powers stay compact (`time^-2`). Qualified enum members stay compact (`Kind: UnitKind.Energy`), as does a named-unit suffix in `Reference: 0.001<W>`. Whitespace normalization never merges distinct lexical tokens, including two unary signs into `++` or `--`.

§ 22(12) Metadata formatting preserves the written expression: literal digits, signs, grouping, operators, dimension/factor order, enum qualification and unit identity are retained. It does not evaluate a fraction, replace it with a decimal approximation, normalize dimensions or reinterpret custom enum members. Comments and literal contents retain their meaning under § 12 and § 29.

§ 22(13) The Sec 0.1 metadata migration policy preserves accepted bare enum values in ordinary formatting and Language Corrections mode: `Kind: temperature`, `System: SI`, and `Status: active` remain spelled that way. Typed values also retain their concrete qualification. Case/underscore normalization of accepted metadata keys still uses their canonical PascalCase spelling, including `BaseUnit`; `IsBaseunit` is not an alias. Value migration to qualified enum names belongs to compiler-authoritative semantic handling; the formatter does not select or rewrite an enum identity from a bare spelling.

---

## § 23. `assert`, ranges, and `step`

**Governance tags:** `tooling.formatter-v2`

§ 23(1) Canonical assertion syntax is statement syntax, not function-call syntax:

```sec
assert condition
```

§ 23(2) An assertion message follows a comma:

```sec
assert value > 0, "value must be positive"
```

§ 23(3) A long assertion condition keeps the first operand with `assert` and continues with ordinary leading-operator layout:

```sec
assert configuration.IsValid
    && connection.IsReady
    && !context.CancelRequested
```

§ 23(4) A long assertion message may follow on a structural continuation line after the condition delimiter when needed:

```sec
assert configuration.IsValid
    && connection.IsReady
    && !context.CancelRequested,
    "configuration must be ready"
```

§ 23(5) Range operators remain compact:

```sec
0..10
0..<10
start..end
```

§ 23(6) A range with `step` remains compact as one conceptual expression where possible:

```sec
0..100 step 5
```

§ 23(7) Range expressions should remain on one line whenever syntactically possible, even when they exceed the preferred width slightly.

§ 23(8) The formatter prefers a moderate width overrun to a visually fragmented range expression.

---

## § 24. Test declarations

**Governance tags:** `tooling.formatter-v2`, `tooling.testing-v1`

§ 24(1) A Sec test declaration uses the canonical top-level test form:

```sec
test "Request parses GET" {
    ...
}
```

§ 24(2) A test declaration is formatted as an ordinary top-level declaration with a named executable block.

§ 24(3) There is exactly one space between `test` and the test-name string and one space between the test-name string and the opening brace.

§ 24(4) The test body follows ordinary Sec executable-block formatting.

§ 24(5) The formatter must not rewrite, split, concatenate, normalize, or otherwise modify the test-name string merely to satisfy preferred width.

§ 24(6) A long test name may exceed preferred width:

```sec
test "The parser correctly preserves all comments surrounding a malformed generic declaration" {
    ...
}
```

§ 24(7) In `structured` style, adjacent top-level tests receive the same structural separation as adjacent functions or other major executable declarations.

§ 24(8) In `compact` style, optional blank separation between adjacent tests is removed.

§ 24(9) Nested `test` declarations are not created by formatting. Subtests expressed through `testing.Run(...)` follow ordinary call and lambda formatting.

§ 24(10) Legacy attribute-based test declarations are not canonical Sec syntax in this revision and must not appear in ordinary formatter examples.

§ 24(11) Converting a historical test function into a named `test "..." {}` declaration is not an automatic Language Correction because deriving a human test name from a function identifier is not uniquely determined.

---

## § 25. Malformed and incomplete source

**Governance tags:** `tooling.formatter-v2`

§ 25(1) The formatter formats syntax that the lossless error-tolerant representation proves to be valid.

§ 25(2) A parser recovery node or other uncertain syntax region is preserved byte-for-byte.

§ 25(3) The formatter must not reindent, re-space, wrap, reorder comments, repair tokens, or insert syntax inside an uncertain region.

§ 25(4) The formatter must not synthesize a missing source token merely to make malformed source look valid.

§ 25(5) Valid syntax surrounding an uncertain region may still be formatted normally when doing so does not alter bytes owned by the uncertain region.

§ 25(6) Format-on-save may run while a user is in the middle of typing a token, string, declaration, or expression. The formatter must not damage that incomplete source.

§ 25(7) The governing principle is:

> Format what the CST proves; preserve what it cannot prove.

§ 25(8) The CLI and LSP may report parser or formatter diagnostics for malformed source.

§ 25(9) `sec fmt --check` must not report a malformed file as fully conforming merely because the format-safe regions are already canonical.

---

## § 26. Language Corrections model

**Governance tags:** `tooling.formatter-v2`

§ 26(1) Language Corrections are an optional source-fix layer distinct from ordinary formatting.

§ 26(2) Language Corrections are disabled by default:

```text
language_corrections = false
```

§ 26(3) When disabled, ordinary formatting must not perform a transformation merely because the formatter recognizes foreign-language muscle memory, obsolete Sec syntax, or redundant syntax.

§ 26(4) When enabled, a Language Correction may perform a local syntactic rewrite only when the intended Sec construct and resulting Sec syntax are uniquely determined from the local syntactic structure.

§ 26(5) A correction must not guess programmer intent.

§ 26(6) A correction must not infer mutability, ownership intent, a missing type, an overload choice, control-flow meaning, error-handling policy, allocation policy, or another semantic decision that is not uniquely established by the source.

§ 26(7) Corrections are grammar-context-sensitive. They are not blind textual or regular-expression substitutions.

§ 26(8) The same token sequence may be correctable in a known declaration or type position and uncorrectable elsewhere.

§ 26(9) Multiple individually safe corrections may be composed when each intermediate or final transformation remains syntactically unambiguous.

§ 26(10) When a correction is not safe, the source remains unchanged. A compiler or LSP diagnostic may still explain the expected Sec syntax.

§ 26(11) Known foreign symbols may be mapped through an explicit whitelist only when the canonical Sec symbol and semantic correspondence are defined.

§ 26(12) An unknown namespace, library type, ownership wrapper, error construct, or language-specific semantic feature must not be translated merely because its spelling resembles a Sec construct.

§ 26(13) The correction catalogue is extensible through dogfooding.

§ 26(14) A new correction may be added without changing the correction model when the transformation is locally unambiguous, preserves the uniquely determined intended Sec construct, and is covered by positive and negative conformance tests.

§ 26(15) Expanding the correction catalogue does not change ordinary canonical formatter output while `language_corrections = false`.

---

## § 27. Initial Language Corrections catalogue

**Governance tags:** `tooling.formatter-v2`

§ 27(1) The initial catalogue includes common foreign function keywords where the construct is unambiguously a Sec function declaration:

```sec
func Parse() void
```

becomes:

```sec
fn Parse() void
```

and equivalent unambiguous `function` or `proc` declaration keywords become `fn`.

§ 27(2) A foreign return arrow in a function return-type position may be removed:

```sec
fn Parse() -> Result[string, Error]
```

becomes:

```sec
fn Parse() Result[string, Error]
```

§ 27(3) A declaration with a missing Sec colon and foreign-style type placement may be corrected when declaration structure is unambiguous:

```sec
let value int = 3
```

becomes:

```sec
let value: int := 3
```

§ 27(4) The same declaration rule applies to unambiguous property syntax:

```sec
property Name string {
    ...
}
```

becomes:

```sec
property Name: string {
    ...
}
```

§ 27(5) The same declaration rule may apply to function parameters and stored fields when the parser can prove their declaration context.

§ 27(6) Foreign mutable `var` local declarations may map to Sec mutable bindings when the declaration is unambiguous:

```sec
var value = 3
```

becomes:

```sec
let mut value := 3
```

§ 27(7) Foreign immutable `const` local declarations may map to Sec immutable bindings only when the construct does not carry additional foreign compile-time semantics that would make the mapping ambiguous:

```sec
const value = 3
```

becomes:

```sec
let value := 3
```

§ 27(8) A plain foreign `let` must not be made mutable merely because another language gives `let` different usage conventions. The formatter does not guess mutability.

§ 27(9) Foreign Go-style array or slice type placement may be corrected in a proven type position:

```sec
[]byte
```

becomes:

```sec
byte[]
```

and:

```sec
[][]byte
```

becomes:

```sec
byte[][]
```

§ 27(10) Foreign generic delimiters may be corrected in a proven generic type or generic call position:

```sec
Result<string, Error>
```

becomes:

```sec
Result[string, Error]
```

§ 27(11) Rust-style turbofish may be corrected when the target and generic argument structure are unambiguous:

```sec
Parser::Parse::<Token>(input)
```

becomes:

```sec
Parser.Parse[Token](input)
```

§ 27(12) `::` between components of an otherwise valid Sec qualified name may be corrected to `.`:

```sec
Status::Ready
```

becomes:

```sec
Status.Ready
```

§ 27(13) `::` is not blindly replaced everywhere. Special foreign roots, namespaces, or library names require either an explicit known-symbol mapping or no correction.

§ 27(13a) The C foreign qualification `c::` is canonical Sec syntax, not foreign-language muscle memory. § 27(12) never rewrites `c::name`, `c::namespace::name`, `c::fn`, `c::flex`, or `c::callback` into member access.

§ 27(14) A known foreign symbol mapping must name a real canonical Sec symbol. The correction engine must not implement a general rule such as removing every `std::` prefix.

§ 27(15) Statement-form increment and decrement are corrected when the operation is an independent statement:

```sec
i++
i--
```

becomes:

```sec
i += 1
i -= 1
```

§ 27(16) Increment or decrement embedded in a larger expression is not corrected unless a future rule can prove a unique Sec semantic equivalent.

§ 27(17) Redundant control-condition parentheses may be removed when doing so is syntactically and semantically unambiguous:

```sec
if (ready) {
    Start()
}
```

becomes:

```sec
if ready {
    Start()
}
```

§ 27(18) The same rule applies to unambiguous `while`, `switch`, and supported `for` condition forms.

§ 27(19) Redundant expression parentheses may be removed only when precedence and grouping are provably unchanged:

```sec
return (value)
```

may become:

```sec
return value
```

while:

```sec
(a + b) * c
```

must remain grouped.

§ 27(20) A trailing foreign statement semicolon may be removed when it is unambiguously only a statement terminator.

§ 27(21) A correction must not treat a semicolon-separated C-style multi-statement line as a trivial whitespace problem unless the complete transformation is explicitly defined and unambiguous.

§ 27(22) A C-style infinite loop may be corrected:

```sec
for (;;) {
    Poll()
}
```

becomes:

```sec
for {
    Poll()
}
```

§ 27(23) A Rust-style infinite `loop` may be corrected to the Sec infinite-loop form when the construct is otherwise unambiguous.

§ 27(24) A Go-style condition loop may be corrected when the header contains only one condition expression:

```sec
for running {
    Poll()
}
```

becomes:

```sec
while running {
    Poll()
}
```

§ 27(25) A C-style loop containing initialization, condition, and increment clauses is not automatically translated because doing so requires control-flow and declaration decisions beyond local syntactic correction.

§ 27(26) Foreign `defer` expression syntax may be corrected to Sec block-only `defer` when the deferred operation is unambiguous:

```sec
defer Close()
```

becomes:

```sec
defer {
    Close()
}
```

§ 27(27) Historical Sec consuming-parameter syntax is corrected:

```sec
fn ConsumingFire(buffer: <- Buffer) void {
    ...
}
```

becomes:

```sec
fn ConsumingFire(->buffer: Buffer) void {
    ...
}
```

§ 27(28) The correction engine must not add `->` merely because a parameter type is move-only.

§ 27(29) Foreign inequality spelling may be corrected only where the foreign token sequence has one unambiguous Sec comparison meaning. An example is F#-style `<>` in a proven comparison expression becoming `!=`.

§ 27(30) Foreign aggregate declaration keywords may be corrected when the resulting Sec declaration is uniquely determined, for example:

```sec
struct Point {
    ...
}
```

becoming:

```sec
type Point struct {
    ...
}
```

§ 27(31) Equivalent unambiguous enum or union declaration forms may use the same correction principle.

§ 27(32) The correction engine does not automatically translate `null`, `nil`, `nullptr`, foreign `new`, foreign ownership wrappers, Rust `?`, foreign exception handling, C-style pointer/borrow syntax, or another construct whose Sec meaning is not uniquely determined.

§ 27(33) The legacy Sec assigned named-type form is corrected because it has the same nominal meaning as the canonical form:

```sec
type UserID = uint64
```

becomes:

```sec
type UserID uint64
```

§ 27(34) The legacy compact variant form `type Name = First Second ...` is not corrected, because its Sec 0.1 replacement (`enum` or `union`, with or without the `error` marker) is not uniquely determined. It receives only the focused migration diagnostic required by `rules/foundations/grammar.md`.

§ 27(35) Invalid legacy Sec syntax is never converted by ordinary formatting while `language_corrections = false`; §§ 27(33)–27(34) apply only when Language Corrections are enabled (MD-002 and MD-003; `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` §§ 3–4).

§ 27(37) A comma forgotten between two items of a comma-separated list written on separate lines — struct declaration fields, parameters, call arguments, and array literal elements — is inserted directly after the earlier item and before any trailing comment, but only where the parser proved the missing-comma repair (`rules/compiler/parser_recovery.md` "Missing comma"). Items on the same line are not repaired, and lists whose grammar accepts a line break as separator (enum values, union variants, register fields, struct literal items) are never given commas by this correction:

```sec
type Point struct {
    x: int // horizontal
    y: int
}
```

becomes:

```sec
type Point struct {
    x: int, // horizontal
    y: int,
}
```

§ 27(36) The redundant state-test comparison `(state is Idle) == true` is corrected to `state is Idle`, and the negated state test `!(state is Idle)` is corrected to `state is not Idle`, only where the parent expression binds more loosely than the equality/state-test level so the rewrite preserves grouping. `option is not Some(value)` and `!(option is Some(value))` are never corrected by deleting the binding, and the null test has no negated correction (MD-006; `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` §§ 7.9, 7.15, 7.25).

---

## § 28. CLI, LSP, and diagnostics

**Governance tags:** `tooling.formatter-v2`, `tooling.lsp-v2`

§ 28(1) CLI formatting and LSP formatting use the same canonical formatter rules.

§ 28(2) Given identical source, project configuration, formatter style version, and effective width, CLI and LSP formatting must produce equivalent source bytes.

§ 28(3) Adaptive width may legitimately produce different wrapping when the effective width differs, but all other canonical rules remain identical.

§ 28(4) Format-on-save must respect the clean multiline preservation rules of § 3.

§ 28(5) Ordinary formatter diagnostics may report malformed syntax, unavailable formatting context, unsupported configuration, or internal formatter failure.

§ 28(6) When Language Corrections are disabled, compiler or LSP diagnostics may still provide a correction suggestion without changing source.

§ 28(7) Diagnostics for a foreign or historical syntax form should show canonical Sec syntax when the intended correction is unambiguous.

§ 28(8) The formatter must not claim semantic validity merely because formatting succeeded.

§ 28(9) *Path selection (MD-037, decided 2026-10-27; `rules/corrections/applied/MD-099-MD-100-MD-037-formatter-alignment-and-selection-correction-20261028.md` §§ 5–8).* `sec fmt` and `sec fmt --check` expand their paths in the same way. Each path is a Sec source file or a directory that is traversed recursively:

```sh
sec fmt source.sec
sec fmt src/
sec fmt .
sec fmt --check .
sec fmt src/ tests/example_test.sec
```

`.` is the explicit request to scan the current directory. Nothing is scanned without being requested, and an empty path list stays an error. The per-file formatter, `--stdin`, `--fix` and `--` are unchanged.

§ 28(10) *Directory selection.*
- **What a scan selects.** It selects files whose names end exactly in `.sec`, based only on the explicit filesystem subtree. Imports, Target, `@when`, Variant, SourceSelection and the CompilationPlan play no part, so unused modules are included. `*_test.sec` files and ordinary `internal/` modules are included as well.
- **Excluded subtrees.** A scan skips the project-reserved `.sec/` and `bin/` directories, `.git/`, `.hg/`, `.svn/`, recognized dependency or vendor and generated-output locations, hidden directories and `testdata/`.
- **No guessing.** A file is never treated as generated or vendored because of its name or content, and a normal directory called `internal` is never excluded.
- **Explicit files.** A file named explicitly is formatted even when it lies in an excluded directory or is a fixture.
- **Never traversed.** Non-Sec files are never touched.
- **Symbolic links.** Traversal never follows links. An explicitly named link is rejected with a diagnostic.
- **Installed sources.** The installed standard library, platform libraries, caches and compiler sources are formatted only when they lie inside an explicitly selected subtree and are not excluded as managed external data.

§ 28(11) *Project boundaries.*
- **Configuration.** A scan that starts inside a project takes its configuration from that project's root (`projects.md`).
- **Nested projects.** A nested project manifest is reported as a boundary conflict, never adopted.
- **Standalone scans.** A scan outside any project uses formatter defaults plus explicit overrides, never a nearby project's configuration.
- **Path errors.** Missing paths, paths that are neither files nor directories, and unreadable paths are diagnostics.
- **Empty directories.** A directory without eligible files completes without edits.

§ 28(12) *Deterministic selection.* All arguments are expanded before any write. Paths are normalized to lexical absolute paths, deduplicated, and sorted by normalized path without locale collation or dependence on filesystem order. Diagnostics keep a stable display path. Overlapping arguments, such as `sec fmt . src/file.sec`, format each file once. The selection is frozen for the invocation, and paths that appear during the run are not added.

§ 28(13) *Preflight and writes.*
- **Configuration.** Effective options are resolved per file under §§ 4(18)–4(23).
- **Preflight.** Selection, readability, manifest and configuration validation all finish before any file is modified. An invalid key or value, an incompatible format version or a boundary conflict fails the invocation with no writes.
- **Writing.** Only selected files that formatted successfully are written, in deterministic order, with the existing safe replacement and their permissions preserved. `sec.toml` is never rewritten.
- **Malformed files in bulk mode.** A malformed file is left completely unchanged and reported (§ 25), and the remaining valid files are still processed. The exit status is nonzero if any selected file could not be fully formatted. The LSP keeps its own § 25 behavior.
- **`--check`.** This mode never writes. It reports, deterministically, every file that would change (`format.noncanonical-source`) and every malformed file (`format.malformed-source`), and returns nonzero if any exist or preflight fails.
- **Language correction.** Directory mode never enables `--fix` implicitly.

§ 28(14) MD-099, MD-100 and MD-037 repair, clarify or select files without changing a valid file's canonical bytes under its published rules, so `format_version` stays `1` (§ 4(4)).

---

## § 29. Formatter invariants

**Governance tags:** `tooling.formatter-v2`

§ 29(1) Idempotence is mandatory.

§ 29(2) Ordinary formatting is semantics-preserving.

§ 29(3) Ordinary formatting does not add, remove, or substitute language syntax except canonical punctuation and grouping syntax explicitly owned by this formatting contract, such as multiline trailing commas and canonical import-region structure.

§ 29(4) Literal token spelling is preserved unless an enabled Language Correction explicitly owns that transformation.

§ 29(5) Numeric literal spelling is not normalized by ordinary formatting.

§ 29(6) String literal contents are not normalized by ordinary formatting.

§ 29(7) Comments are preserved. Comment movement is limited to syntax-aware movement of an attached comment with a construct that the formatter is explicitly permitted to reorder, such as an import.

§ 29(8) Comment text changes only under the comment-formatting rules of § 12 and must preserve preformatted material.

§ 29(9) Uncertain malformed regions remain byte-for-byte unchanged under § 25.

§ 29(10) Ordinary formatting does not require semantic analysis.

§ 29(11) Import sorting has no semantic exception (§ 5(7)); no formatting pass authorizes general semantic rewriting.

§ 29(12) `format_version = 1` is a conformance-test contract.

---

## § 30. Conformance testing

**Governance tags:** `tooling.formatter-v2`

§ 30(1) Every normative formatter behavior must have a golden formatting test where practical.

§ 30(2) The minimum golden structure is:

```text
messy input -> canonical output
canonical output -> unchanged
boundary case
```

§ 30(3) Every golden canonical output is formatted a second time automatically to verify idempotence.

§ 30(4) Valid-source round-trip testing must verify:

```text
parse source
format source
parse formatted source
compare equivalent syntax/AST meaning modulo trivia and source positions
```

§ 30(5) Round-trip testing must verify that meaningful token structure is preserved by ordinary formatting.

§ 30(6) Regression suites must cover at least:

- line comments;
- block comments;
- documentation comments;
- malformed and incomplete source;
- very long lines;
- deeply nested constructs;
- alignment boundaries;
- imports with aliases and attached comments;
- ownership markers;
- registers and units;
- tests;
- patterns and destructuring;
- Language Corrections positive cases;
- Language Corrections negative cases.

§ 30(7) Width tests must include effective widths `80`, `120`, and `200`.

§ 30(8) Fixed-width boundary tests around the default must include at least `119`, `120`, and `121` columns.

§ 30(9) Vertical-style tests must cover both `structured` and `compact`.

§ 30(10) Indentation tests must cover widths `2`, `4`, and `6`.

§ 30(11) Malformed-source tests must verify exact byte preservation for every protected uncertain region.

§ 30(12) Every Language Correction requires both:

- positive tests proving the intended correction occurs;
- negative tests proving nearby ambiguous or semantically different syntax is not rewritten.

§ 30(13) Test-declaration conformance must include formatter round-trip coverage for `test "name" { ... }` and must verify exact preservation of the test-name string.

§ 30(14) Import tests must verify that sorting and grouping do not remove imports and that attached comments move with their imports.

§ 30(15) CLI/LSP equivalence tests must compare output for the same effective width and project configuration.

---

## § 31. Non-goals and forbidden shortcuts

**Governance tags:** `tooling.formatter-v2`

§ 31(1) A conforming formatter must not use semantic guesses to make invalid source compile.

§ 31(2) A conforming formatter must not maintain a second independent Sec grammar solely for formatting.

§ 31(3) A conforming formatter must not discard comments or source data that are not formatter-owned.

§ 31(4) A conforming formatter must not rewrite arbitrary malformed source in the hope of repairing it.

§ 31(5) A conforming formatter must not use blind regular-expression replacement as the implementation model for grammar-sensitive Language Corrections.

§ 31(6) A conforming formatter must not reorder ordinary top-level declarations.

§ 31(7) A conforming formatter must not remove imports as an ordinary formatting action.

§ 31(8) A conforming formatter must not change ownership, borrowing, mutability, unsafe boundaries, control flow, or error behavior merely for style.

§ 31(9) A conforming formatter must not use obsolete Sec syntax as canonical output.

---

## § 32. Summary of the Sec 0.1 formatting contract

**Governance tags:** `tooling.formatter-v2`

§ 32(1) Sec formatting is canonical, deterministic, and idempotent for a selected project formatting configuration.

§ 32(2) The default style is four-space indentation, structured vertical spacing, fixed preferred width `120`, and `format_version = 1`.

§ 32(3) Clean permitted multiline intent may be preserved, while mixed layouts are normalized.

§ 32(4) Structural syntax takes precedence over arbitrary horizontal alignment.

§ 32(5) Executable blocks are multiline; aggregate values may retain clean single-line or multiline layout where permitted.

§ 32(6) Multiline comma-separated lists use complete multiline layout with canonical trailing commas where the grammar permits them.

§ 32(7) Comments, malformed regions, literal data, ownership markers, and test-name strings are preserved according to their dedicated rules.

§ 32(8) Imports form one canonical region and may be sorted and grouped, while ordinary declarations remain in source order.

§ 32(9) Ordinary formatting is distinct from optional Language Corrections.

§ 32(10) Language Corrections fix only locally unambiguous syntax and never guess programmer intent.

§ 32(11) The correction catalogue may expand through dogfooding without weakening the unambiguity requirement.

§ 32(12) Published normative paragraph identifiers are stable references and are never casually renumbered in later revisions.
