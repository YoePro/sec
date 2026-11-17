# MD-090 — Rune-indexed positions for `stdlib/str` immutable editing

- **Date:** 2026-11-16
- **Revision:** 1.0
- **Status:** Applied 2026-11-17 (stdlib_str.md §§ 11.1–11.9, edit.sec, edit_test.sec, view-capable Builder.Append; MD-090 decided-implementation-pending until the tests can execute)
- **Sec versions:** 0.1 and 0.1.1
- **Repository:** `YoePro/sec`, `main` at `7520210ab62ff36024a796f3076c1df4250460be`
- **Decision record:** `missing-decisions.yaml` MD-090
- **Canonical targets:** `sec/stdlib/str/stdlib_str.md` §§ 11.2–11.9, `sec/stdlib/str/edit.sec`, `sec/stdlib/str/edit_test.sec`
- **Related authorities:** MD-071 (core rune positions); applied MD-126 (rune-indexed borrowed string ranges and `StringView`); `rules/library/core-library.md`

## § 1. Decision

**§ 1(1).** `str.Insert`, `str.RemoveRange` and `str.ReplaceRange` accept **zero-based Unicode scalar (`rune`) indexes**, not UTF-8 byte offsets. A range remains half-open: `[startRune, endRune)`. The end position `value.Len` is legal. These operations edit immutable `string` values and produce owned `string` results; the original value remains unchanged.

**§ 1(2).** Both `Insert` overloads use rune indexes. The character argument of the `rune` overload changes the inserted value's type, not the positional unit.

**§ 1(3).** This decision aligns with core `string.IndexOf` and the applied MD-126 string-range indexing contract. It does not change byte-level APIs, UTF-8 representation, Unicode normalization, grapheme-cluster semantics or escaped-input error positions elsewhere in `str`.

## § 2. Canonical public API

**§ 2(1).** Replace the § 11.4 edit signatures with:

```sec
fn Insert(value: string, runeIndex: uint, insertion: string) Result[string, EditError]
fn Insert(value: string, runeIndex: uint, insertion: rune) Result[string, EditError]
fn RemoveRange(value: string, startRune: uint, endRune: uint) Result[string, EditError]
fn ReplaceRange(value: string, startRune: uint, endRune: uint, replacement: string) Result[string, EditError]
```

**§ 2(2).** The public edit error union retains range-order, rune-position bounds and allocation failures. Remove the byte-boundary-specific variant:

```sec
type EditError union error {
    OutOfBounds(uint),
    StartAfterEnd,
    Allocation(CollectionError),
}
```

**§ 2(3).** `EditError.OutOfBounds(position)` now reports an invalid **rune index**. `StartAfterEnd` is tested before individual bounds validation on the range operations, preserving existing error precedence. An empty edit still validates all supplied indexes before returning the original string.

## § 3. Normative amendments without renumbering

**§ 3(1).** In `stdlib_str.md` § 11.2, replace the byte-position rule and its conversion rationale with rune-index position semantics. Use parameter names `runeIndex`, `startRune` and `endRune`. Explicitly keep the range half-open.

**§ 3(2).** In § 11.3, remove `NotRuneBoundary` and document `OutOfBounds(uint)` in runes. In § 11.4, use the § 2(1) signatures above. In §§ 11.5–11.7, validate against `value.Len`, not `value.SizeOf`, and remove byte-boundary checks. The complete range `[0, value.Len)` removes the entire string.

**§ 3(3).** In § 11.8, preserve immutable editing, allocation semantics and the recommendation to use `Builder` for repeated edits. In § 11.9, replace byte-split rejection tests with rune-index bounds and multibyte text correctness tests. Keep all existing section and paragraph identifiers stable for code references.

**§ 3(4).** Update § 11.1 source documentation and related public hover comments to state that edit positions are rune indexes. Do not change unrelated byte-offset APIs.

## § 4. Implementation boundaries

**§ 4(1).** In `edit.sec`, replace the public byte-position parameters and byte-range validation with rune-index bounds validation. Remove private `_RuneIndexOfBoundary` and `_ValidateEditBoundary` if no remaining callers need them. Never reinterpret a rune index as a byte offset.

**§ 4(2).** Follow the **applied MD-126** range contract: `value[0..<runeIndex]` and `value[runeIndex..<value.Len]` produce borrowed `StringView` values. This replaces historical `string.Slice` calls. An implementation must not reintroduce the old `Slice` API or silently pretend `StringView` is an owned `string`.

**§ 4(3).** Feed those borrowed views into result materialization through the approved view-capable `Builder.Append` path (or its explicitly justified internal equivalent). Preserve original source lifetime while views are used. Public edit results remain `Result[string, EditError]`; no borrowed view escapes.

**§ 4(4).** Preserve existing allocation failure mapping to `EditError.Allocation`. A no-op edit returns the original immutable value without requiring materialization. The `rune` overload still inserts the rune's UTF-8 encoding, not its numeric bytes.

## § 5. Required regression cases

**§ 5(1).** Update `edit_test.sec` and relevant documentation fixtures to use rune positions, in particular:

```sec
test "Insert after multibyte rune using rune index" {
    let result := try Insert("AåB", 2, "!")
    testing.ExpectEqual("Aå!B", result)
}

test "Remove full multibyte rune using rune indexes" {
    let result := try RemoveRange("A🦊B", 1, 2)
    testing.ExpectEqual("AB", result)
}
```

**§ 5(2).** Cover the beginning and end (`0`, `value.Len`), empty strings, no-op edits, replacement of 2/3/4-byte runes, combining runes (without normalization), insertion using an `IndexOf` result, reversed ranges, and invalid rune indexes. Remove expectations for `NotRuneBoundary` and do not conflate byte `SizeOf` with rune `Len`.

**§ 5(3).** Confirm the imported `str` package/API, parser/Sema, and tests use the new names and result variants. Do not claim code is implemented or test gates have passed until executed in a matching checkout.

## § 6. Tracking and handoff

**§ 6(1).** After owner approval, record MD-090 as **`decided-implementation-pending`** in `missing-decisions.yaml`, rather than deleting it prematurely. Once canonical rulebook §§ 11.2–11.9 and associated tests are synchronized and accepted, close/remove the entry according to the registry's ordinary process.

**§ 6(2).** Only update relevant library implementation governance if the actual ownership fragment includes this work; do not invent a new ledger owner. Preserve the separate compiler and library implementation distinction in `governance/index.yaml`.

**§ 6(3).** This file records an approved change to the public API, not a claim of a code change, successful build or completed migration. MD-126's historical correction remains unchanged; this decision resolves only its explicitly deferred MD-090 position-unit question.
