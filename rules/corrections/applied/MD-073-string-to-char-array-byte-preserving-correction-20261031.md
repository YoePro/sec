# MD-073 — Byte-preserving `string.ToCharArray()`

- **Decision ID:** MD-073 (`char-array-conversion-outside-char-domain`, P23-D3)
- **Document kind:** Normative correction for canonical Sec rulebooks
- **Status:** APPROVED by language owner, 2026-10-31; applied 2026-10-31 to `compiler_known_members.md`, `core-library.md` § 5.3, `types.md`, P23 package/todo and the Semantic IR P23 note; Sema signature `Result[char[], CollectionError]`; lowering pending with P18/P19/P23
- **Created:** 2026-10-31
- **Last updated:** 2026-10-31
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository reviewed:** `YoePro/sec`, `main` at `1eba0fd61642f2273a2ff9918ab453006c72bc80` (checked 2026-10-31)
- **Correction staging path:** `rules/corrections/md073-string-to-char-array-byte-preserving-correction.md`
- **Primary normative owners:** `rules/types/types.md`, `rules/library/core-library.md`, `rules/compiler/compiler_known_members.md`
- **Related implementation owner:** `rules/mlir/packages/sec-mlir-dialect_package23.md`, `package23-todo.md`, and their existing governance entries
- **Related settled decisions:** MD-050 (valid UTF-8 strings), MD-072 (materializing owning array and `CollectionError`), MD-091 (`byte[]` `ToString()` is not a decoder)
- **Supersedes:** The unresolved MD-073 alternatives in `missing-decisions.yaml` and provisional char conversion descriptions that imply rune-to-char representability checking

## § 1. Decision and scope

**§ 1(1).** A Sec `char` is a distinct 8-bit type with numeric domain `0..255` and its own character-oriented members. It is **not** a restricted `rune`. Equal 8-bit representations for `byte` and `char` do not create an implicit conversion between the distinct source-language types (`types.md`, `byte`/`char`).

**§ 1(2).** For Sec 0.1, `string.ToCharArray()` is **byte-preserving**. It produces one `char` for **each byte of the source string's UTF-8 encoding**, in byte order, with an identical numeric value. It is conceptually equivalent to `ToByteArray()` with an exact per-element `char(byte)` conversion. It **does not** iterate Unicode scalars/runes to produce the destination elements and **does not** attempt a checked `rune`-to-`char` conversion.

**§ 1(3).** The earlier MD-073 dilemma about Unicode scalar values above 255 is thereby resolved: a scalar encoded in multiple UTF-8 bytes simply contributes multiple `char` elements. No truncation, replacement or Unicode scalar-range failure occurs. This is an intentional binary-oriented facility, not a claim that the output contains one element per human-readable character.

## § 2. Exact source contract

**§ 2(1).** The canonical public method is:

```sec
impl string {
    fn ToCharArray() Result[char[], CollectionError]
}
```

It is available on valid Sec `string` values and requires no arguments. It returns an independently owned dynamic `char[]` on success, without consuming or mutating the source.

**§ 2(2).** If the source's encoded byte length is `self.SizeOf`, the success result has **exactly** `self.SizeOf` elements. For each zero-based byte index `i` in this range, the resulting element's numeric value equals `self.ByteAt(i)`. This uses **byte count**, never `self.Len` (rune count). Empty input yields an empty array. All bytes, including zero and UTF-8 continuation bytes, are preserved. No terminator is appended.

**§ 2(3).** The method uses the existing `CollectionError` family for materialization failures (allocation, capacity, or size limitations according to the canonical collection/allocation rules). It introduces **no** `ConversionError`, `StringError`, `InvalidUtf8`, `InvalidCodePoint`, or new MD-073-specific failure variant. Where the selected target lacks a valid allocation context, the existing allocation-profile legality rules apply; do not introduce a hidden allocator.

**§ 2(4).** Result ownership, independence, and failure atomicity are identical to the approved `ToByteArray` materialization contract: no alias of mutable `string` backing is exposed, no partly formed successful array is published on failure, and the source remains intact. Internal compiler/core reuse of a byte-array materialization path is encouraged. Do not require two separately allocated arrays merely to preserve the conceptual `char(byte)` conversion, but do not introduce a source-visible implicit `byte[]` to `char[]` cast or unsound storage retyping.

## § 3. Examples and observable behavior

**§ 3(1).** Examples below show numeric `char` literal values. They are illustrative Sec expressions; full materialization may await P18/P19/P23 lowering.

```sec
let ascii := try "ABC".ToCharArray()  // [0x41t, 0x42t, 0x43t]
let accent := try "é".ToCharArray()    // [0xC3t, 0xA9t]
let omega := try "Ω".ToCharArray()     // [0xCEt, 0xA9t]
```

**§ 3(2).** The UTF-8 sequence of `"é"` is `C3 A9`, **not** the single byte `E9` of ISO-8859-1/Windows-1252. `ToCharArray()` does not encode into an ambient ANSI/locale/codepage encoding. An explicit codepage codec is required for that purpose. ASCII (`U+0000..U+007F`) preserves one element per scalar because ASCII and UTF-8 agree over that range.

**§ 3(3).** No round-trip invariant `text.ToCharArray().ToString() == text` is imposed for non-ASCII text. The already established `char` and char-sequence `ToString()` operations retain **their own** interpretation of the `char` elements, not a special implicit UTF-8 byte decoder. In particular, do not change `char[].ToString()` to decode the original UTF-8 bytes just to make this conversion round-trip. Explicit decoding of a byte sequence uses the designated byte-to-string decoding API, not the character-sequence rendering operation.

**§ 3(4).** `char.ToString()` and char-sequence `ToString()` remain character-to-text materialization, not a raw-byte reinterpretation or a promise of a chosen legacy codepage. The existing independent representation must be retained and clarified separately where needed; MD-073 does not invent a locale-dependent default.

## § 4. Relationship to existing APIs and language types

**§ 4(1).** `ToByteArray()` returns `Result[byte[], CollectionError]`, containing precisely the original UTF-8 bytes. `ToCharArray()` returns `Result[char[], CollectionError]`, with the **same numeric bytes** and the distinct element type `char`. `ToRuneArray()` returns `Result[rune[], CollectionError]`, containing one Unicode scalar per element. Neither `char` nor `byte` becomes equivalent to `rune`.

**§ 4(2).** Ordinary explicit scalar conversions (`char(byteValue)`, `byte(charValue)`) remain total and value-preserving for `0..255`. The byte-preserving collection operation does not authorize implicit array conversions or weaken type identity or overload resolution.

**§ 4(3).** `byte[]` / byte-sequence `ToString()` remains a type/collection summary as specified by MD-091; it does not decode text. `char[]` `ToString()` retains the separate character-sequence contract. `string` retains its valid-UTF-8 invariant under MD-050.

## § 5. Required rulebook synchronization

**§ 5(1).** In `rules/compiler/compiler_known_members.md`, replace the undecided `ToCharArray` char-interpretation/length language with the exact `Result[char[], CollectionError]` and byte-preserving behavior of §§ 1–4. Repair the examples that omit `try`. Fix the earlier round-trip expectation so it does not require non-ASCII text to survive `ToCharArray().ToString()` unchanged; do **not** change `char[].ToString()` to be a byte decoder.

**§ 5(2).** In `rules/library/core-library.md` § 5.3, include the complete public `string.ToCharArray()` signature alongside `ToByteArray()` and `ToRuneArray()`, retaining the shared collection-allocation contract and explicitly distinguishing `self.SizeOf` from `self.Len`. Clarify `char`-sequence conversion in the relevant textual API discussion as necessary.

**§ 5(3).** In `rules/types/types.md`, confirm the byte-preserving `string`→`char[]` operation without changing the established `char` domain, literal rules, `rune` identity or UTF-8 invariant. This method does not reclassify `char` as Unicode text, Latin-1 or an implicit `byte` alias.

**§ 5(4).** Synchronize P23 (`sec-mlir-dialect_package23.md` § 24, § 27 and its `package23-todo.md`) by removing MD-073 as a *design* gate. Keep real P18/P19 allocation/ownership, P23 semantic/physical lowering and negative conformance work independently pending until implemented. Preserve the existing MD-050/072/091 contracts.

**§ 5(5).** Keep existing paragraph identifiers stable. Append new numbered provisions or amend text in place without renumbering previously cited sections. Update `language-rulebook-status.md` only if its tracked documentation state actually changes.

## § 6. Required tests

**§ 6(1).** Positive cases: empty, ASCII, embedded NUL, UTF-8 two-byte `é`, three-byte `€`, four-byte `😀`, mixed text, and arbitrary `0x80..0xFF` byte values that occur as parts of valid UTF-8 sequences. Require result type `char[]`, count `SizeOf`, element-by-element equality with the UTF-8 bytes, no terminator and independent ownership/mutation.

**§ 6(2).** Negative cases: unavailable allocator/context; real allocation/capacity/size failure; attempted implicit `byte[]` ↔ `char[]` conversion; accidental rune-count allocation; invalid expectation of `ConversionError.OutOfRange` for `Ω`; any attempt to treat continuation bytes as independently decoded runes. On failure publish no successful result or partial output.

**§ 6(3).** Cross-API tests compare `ToByteArray()` and `ToCharArray()` numeric bytes and distinguish `ToRuneArray()`. Ensure char-sequence `ToString()` retains its own existing semantics and does not opportunistically reinterpret the sequence as UTF-8 based on its origin.

**§ 6(4).** Compiler tests should cover source type checking and diagnostics, then Semantic IR, MLIR, actual execution, and 32/64-bit target plans when the predecessor storage/allocation pipeline supports the path. Mark unsupported backend work as pending, not passed.

## § 7. Implementation and governance handoff

**§ 7(1).** Integrate one discrete part at a time: canonical type/API text, source declarations and compiler registry, Sema validation and resolved semantics, Semantic IR/materialization, lowering, focused positive/negative tests, then governance status. Test each part before beginning the next; do not produce dummy implementations or claim execution support where the allocation pipeline still rejects the operation.

**§ 7(2).** Reuse existing P23, core/collection, frontend, and governance owners. Search the current `governance/*.yaml` stable IDs before editing; update implemented/remaining/identified-bugs by their existing machine-readable IDs rather than inventing duplicate `md073.*` integration owners. MD-073's normative decision and its implementation acceptance remain separate statuses.

**§ 7(3).** After canonical rulebooks **and** governance are synchronized, resolve/remove MD-073 from `missing-decisions.yaml` according to the register's policy. Keep implementation gaps in the existing governance/P23 tracking. Archive this correction under `rules/corrections/applied/` with the *actual application date* only when integration is complete, as required by `rules/corrections/README.md`.

## § 8. Final decision

**§ 8(1).** APPROVED: `string.ToCharArray()` is the byte-preserving, owning-`char[]` counterpart of `string.ToByteArray()`, with one `char` per UTF-8 byte, the same numeric contents, a `Result[char[], CollectionError]` public signature and no rune-range rejection or implicit legacy-codepage conversion.

**§ 8(2).** Not claimed: P18/P19/P23 executable materialization, automatic non-ASCII round trips through `char[].ToString()`, or implementation completion in the current repository.
