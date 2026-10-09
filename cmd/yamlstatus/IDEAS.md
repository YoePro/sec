# yamlstatus — ideas for further subcommands

`yamlstatus` currently reports statistics and selects remaining points. The
ideas below come from work that was repeatedly done by hand with throwaway
scripts while synchronizing governance during implementation, refactoring and
correction integration. Each one keeps the governance files authoritative: the
read-only checks only report, and the few writing helpers make exact,
reviewable edits.

Ordered roughly by how much time or risk they would have saved.

## 1. `-check`: validate the ledger strictly

**Implemented (2026-10-09)** as the `check` subcommand; see `codex.md`. Mixed
indentation itself is not detectable after parsing, so `check` reports its
observable trace instead: an item whose text swallowed the next `- ` item, or
an item that turned into a mapping. Edit commands now refuse changes that add
a problem. The first run found 24 problems in the ledger (16 items parsed as
one-pair mappings, one merged lexer item, verification results filed under
`implemented` of the wrong integration, an implemented point filed under
`verification`, and an empty `remaining:` decoding as null); all were fixed.

Run the same structural checks after every governance edit and fail with a
nonzero status.

- **Duplicate mapping keys.** A second `governance_reconciliation:` or
  `verification:` key in one integration is silently dropped by ordinary YAML
  loading; the first one disappears without an error. This happened in
  practice and was only caught by a custom duplicate-key loader.
- **Unique integration IDs** across all fragments, and each ID owned by the
  fragment `index.yaml` assigns it to.
- **Shape of known fields**: `verification` items have `command` and `result`;
  `implemented`/`remaining` are lists of strings or of `{id, description}`
  where the fragment already uses that form; `status` is one of the values in
  `index.yaml`.
- **Mixed list indentation inside one entry**, which breaks parsing when an
  item is inserted at the wrong depth (four versus six spaces).

## 2. `-paths`: check referenced files

List every `code:`/`tests:` path (and `rules:` path) that does not exist on
disk, with file, line and integration ID. Today this needs a grep pipeline; it
found one stale reference (`internal/sema/call_graph_test.go`) that had never
existed.

Optional `-paths -go` restricts the check to `.go` files under `internal/` and
`cmd/`.

## 3. `-uses PATH`: who references a file?

Print every integration that lists a given path, for example
`-uses internal/sema/stack_budget.go`. Before moving, merging or deleting a
source file this answers "which governance entries must change?" in one call.
The writing counterpart is `repath` in section 4.3.

## 4. Harmonize, add and change data

**Implemented (2026-10-09):** `fmt` (with `-check`, `-diff`, `-fix-types`,
`-write`), type-hazard reporting, and `set`, `add`, `remove`, `move`,
`verify`, `repath` and `new`; see `codex.md`, "Editing governance with
yamlstatus". Canonical key order is not implemented: reordering keys would
rewrite almost every integration and needs an agreed order first.

A single editing layer that all writing commands share, built on the
`gopkg.in/yaml.v3` node API that the module already depends on. Working on
`yaml.Node` rather than decoding into structs keeps comments, key order and
untouched values intact, so an edit changes only what it names.

Every writing command shows a unified diff and changes nothing until `-write`
is given. After writing it reruns the format and `-check` validation, so a
command can never leave a fragment unparsable or with duplicate keys.

### 4.1 `fmt`: harmonize syntax

Rewrite fragments into one canonical layout, and with `fmt -check` exit nonzero
when a file is not canonical (useful in tests or CI). The output must be
idempotent and must never change a parsed value.

Observed variation that this would remove:

- list items under a four-space key are indented both four and six spaces,
  often within the same file (`analysis.yaml` has 278 and 1295 lines of each);
- date fields are written double-quoted, single-quoted and unquoted;
- `document_revision` appears as `"2.0"`, `'2.0'`, `2.0`, `2`, and free text;
- long strings switch between plain, quoted and folded `>-` scalars.

A canonical style could be: one fixed indentation for nested lists, double
quotes only where YAML requires quoting, folded `>-` for prose longer than the
line width, and a canonical key order per integration (`id`, `area`, `status`,
dates, `document_revision`, `summary`, `rules`, `code`, `tests`, `implemented`,
`partial`, `remaining`, `required_tests`, `verification`, ...).

### 4.2 Type hazards are reported, not silently fixed

Some differences are not cosmetic, because YAML assigns them other types:

- an unquoted `1.0` or `2.0` decodes as a float (`1.0` becomes `1`), and an
  unquoted `2026-10-07` decodes as a timestamp;
- date fields contain `audited: true`, `integrated: true` or
  `integrated: null` instead of a date.

`fmt` should list these with file, line and integration ID. Converting them
(for example quoting a version so it stays the string `"1.0"`) belongs behind an
explicit flag such as `fmt -fix-types`, and a value that is not a date where a
date is expected needs a human decision rather than an automatic rewrite.

### 4.3 Data commands

All commands address an integration by exact ID and a list or field by key,
and they format the inserted text with the same rules as `fmt`:

- `set ID KEY VALUE`: set a scalar such as `status`, `revised` or
  `document_revision`;
- `add ID KEY TEXT [-at N]`: append (or insert) an item into `implemented`,
  `remaining`, `tests`, `code`, `rules`, ...; refuse an exact duplicate in the
  same list;
- `remove ID KEY N|TEXT`: remove an item by 1-based position or exact text;
- `move ID FROM N TO`: move a finished point, for example from `remaining` to
  `implemented`, which is the most common edit after implementing something;
- `verify ID COMMAND RESULT`: add a well-formed `command`/`result` item at the
  top of `verification`;
- `repath OLD=NEW ...`: rewrite moved file paths in every fragment, dropping the
  old line when `NEW` is already listed, and print the integrations touched so
  each can get a verification item;
- `new FILE ID`: create an integration with the required fields from
  `index.yaml`, after checking that the ID is unused.

A command that matches nothing, a list or ID that does not exist, or an
ambiguous ID fails with a nonzero status instead of guessing.

## 5. `-md ID`: missing-decision cross-references

For a missing-decision ID such as `MD-012`, list every mention in
`governance/`, `rules/`, `language-rulebook-status.md`, Go sources and Sec
sources, grouped by file. With `-md closed`, report references to IDs that are
no longer in `missing-decisions.yaml` but still read like open blockers
("blocked on MD-012", "await MD-012", "remain undecided"). Closing a decision
always requires finding and rewording these.

## 6. `-corrections`: applied-correction bookkeeping

- Every `corrections_applied:` path exists under `rules/corrections/applied/`.
- Every file in `rules/corrections/applied/` is referenced by at least one
  integration or rulebook (otherwise the archive is disconnected from the
  ledger).
- Correction files left in the repository root or in
  `rules/corrections/` with a non-applied status are listed as pending.

## 7. `-diff BASELINE`: remaining-count changes

`codex.md` asks for the remaining count before and after an implementation.
Store a snapshot (`-snapshot file.json`) and later print per-fragment and
per-integration changes in remaining/implemented counts against it, or against
`git show HEAD:governance/...`. That turns the manual "remaining 7 to 6"
bookkeeping in verification results into one command.

## 8. `-diagnostics`: registry versus governance

Compare the diagnostic IDs registered in `internal/diagnostics` with the IDs
mentioned in `governance/errors_diagnostics.yaml` and the rulebooks: IDs used
in prose but not registered, registered IDs never mentioned in governance, and
the next free ID per family (for example `S1139`). Picking a new ID currently
means grepping the whole repository to be sure a number is unused.

## 9. `-grep TEXT` limited to remaining work

A plain text search restricted to `remaining` items (with integration ID,
file and line), for example `-grep "C::"` or `-grep MD-021`. This complements
`-exclude`, which only filters selection.

## Small conveniences

- `-list FILE` printing integration IDs with their remaining counts, to pick an
  `-id` without opening the file.
- `-status partial|planned|implemented` as a filter for statistics and
  selection.
- Machine-readable output (`-json`) for the statistics view as well, not only
  for a selected point.
- A short summary line at the end of every write command: files changed,
  integrations touched, and whether `-check` still passes.
