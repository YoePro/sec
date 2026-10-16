# P23 Normative Synchronization — String and UTF-8

- **Status:** Normative additive amendment (subject to existing MD decisions)
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec_string_core_sync_package23.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P23`

---

## § 1. Authority and scope

This is an **additive compatibility proposal**, not blanket replacement of existing public signatures. Synchronize MD-004 and MD-058 first. Read current source core declarations before editing.

## § 2. Correct string properties

Public `Len`/compatibility `.len` count Unicode scalars; instance `SizeOf` UTF-8 payload bytes; `len(string)` returns `int` scalar count; type-form `string.SizeOf` physical descriptor size. No public `ByteLen`, `RuneLen`, `CharLen`.

## § 3. Empty/NUL/Unicode

Count embedded U+0000 as data. No implicit normalization or grapheme counting. Byte text views require correct UTF-8 boundaries for known-valid string semantics; error behavior of invalid inputs remains MD-050.

## § 4. String copy

`string` remains implicitly copyable without allocation or mutable aliasing; copying preserves backing lifetime. No mandatory refcount.

## § 5. MD-004 materialization

Concatenation/interpolation runtime expression is one allocating/fallible operation, `AllocationError` via ordinary `try`, with Sema-selected allocation context. Static fully folded operation uses immutable static bytes. No hidden intermediate strings.

## § 6. Array conversions

`FromByteArray`/`FromRuneArray` and `ToByteArray`/`ToRuneArray` must use exact LSP source signatures once approved. Current core source diverges from rulebook; refer to MD-072 (P23-D2), not arbitrary signature invented here.

## § 7. MD-071 (P23-D1) offset units

Private `__StringSliceUnchecked` uses byte offsets and strict caller proof; this does not settle units/boundary policy for public `IndexOf`, `LastIndexOf`, `Slice`, `GetRune` and related overloads.

## § 8. MD-073 (P23-D3) char conversion

`char` is 0..255, `rune` is Unicode scalar; `ToCharArray` on scalars >255 needs explicit normative behavior, not silent lossy byte reinterpretation.

## § 9. MD-050 remains separate

MD-050 owns invalid UTF-8 string validity and decoding policy. Do not create a duplicate P23 decision for the same fork.

## § 10. No new API surface

Do not add generic `Bytes`, `Chars`, `Runes` aliases or `String` wrapper. Do not make the core-private unchecked helper importable. `SplitToArray`, `SplitOnce`, `SplitLines`, `ToUpper`, `ToLower` require their respective actual normative status checks.

## § 11. Core stubs

Replace only confirmed stubs/buggy bodies after reconciling with exact current rulebooks and decided public signatures. Examples to audit: rune length stub, rune array conversion stub, mixed byte/rune offset arithmetic, byte-range substring and iterator cursor.

## § 12. Preserve numbered paragraphs

Append amendments under source book ownership and do not renumber existing §§. Update existing governance fragments and implementation-status evidence; do not overwrite old MD records.
