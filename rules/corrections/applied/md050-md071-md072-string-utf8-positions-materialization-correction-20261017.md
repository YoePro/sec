# MD-050, MD-071 and MD-072 — Valid UTF-8 Strings, Rune Positions and Explicit Array Materialization

- **Status:** Applied 2026-10-17 — integrated into `rules/types/types.md` ("Valid UTF-8 invariant"), `rules/library/core-library.md` §§ 5, 5.1–5.3, 5.7 (new) and 15.1, `rules/compiler/compiler_known_members.md` (byte/char/rune sequence `ToString()`, `ToByteArray()`, `ToRuneArray()`, round trips, associated constructors, `FromByteArray`, `FromRuneArray`, materialization tests), `rules/memory/allocation.md` § 17(10), the P23 package and decision index, and `sec/stdlib/str/stdlib_str.md` §§ 11.2, 20.7. `sec/core/string.sec` corrected per § 7; compiler-known registry `ToByteArray`/`ToRuneArray` now return `Result[..., CollectionError]`; stdlib `str` (split, edit, replace, escape, whitespace) and `net/http` structured fields migrated to rune positions. Test `TestStringUTF8PositionsAndMaterializationSurface`. Governance `frontend.types-core` (`governance/types.yaml`). MD-050, MD-071 and MD-072 removed from `missing-decisions.yaml`; follow-ups MD-090 (str edit position unit), MD-091 (byte-sequence `ToString` validity, implementation choice) and MD-092 (`CharAt`). Known hole (verified 2026-10-17): the § 9 runtime conformance gates are open because `sec test` cannot execute test bodies yet. MD-073 and MD-074 remain open.
- **Created:** 2026-10-17
- **Last updated:** 2026-10-17
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/corrections/applied/md050-md071-md072-string-utf8-positions-materialization-correction-20261017.md`
- **Decision IDs:** MD-050, MD-071 (SEC-MLIR-P23/P23-D1), MD-072 (SEC-MLIR-P23/P23-D2)
- **Primary rulebooks:** `rules/types/types.md`, `rules/library/core-library.md`, `rules/compiler/compiler_known_members.md`, `rules/memory/allocation.md`
- **Related rulebooks:** `rules/collections/collections.md`, `rules/mlir/packages/sec-mlir-dialect_package23.md`, `rules/tooling/diagnostics.md`, `rules/tooling/lsp.md`, `sec/stdlib/str/stdlib_str.md` (when present), compiler testing and string storage/representation rules
- **Affected core source:** `sec/core/string.sec` (implementation corrections and source/LSP comments)
- **Replaces:** No stable numbered normative paragraphs. Amend contradictory statements in place; retain existing paragraph identifiers and references.
- **Supersedes earlier provisional forms:** The undecided constructor and array-return signatures in `core-library.md` §5.2–5.3 and implementation-status notes, and the unresolved encoding alternatives in `compiler_known_members.md` for the affected methods.
- **Other decisions preserved:** MD-058 (`string.Len` counts Unicode scalar values; `string.SizeOf` counts encoded UTF-8 bytes), MD-073 (`char` domain overflow, with exact API/error type to be synchronized separately), MD-074 (representability of `len(...) -> int`, unresolved), and the existing distinct allocation contexts.
- **Source baseline:** User-supplied `missing-decisions.yaml` dated 2026-10-16, MD-050/071/072; current public `main` file views of `sec/core/string.sec`, `rules/library/core-library.md` and `rules/compiler/compiler_known_members.md` retrieved 2026-10-17. Uncommitted local changes and the exact repository HEAD were not verified.

---

## § 1. Authority, scope and exclusions

**§ 1(1).** This correction establishes three interdependent string rules: `string` is **always valid UTF-8 text**, public string positions for the methods specified below are **Unicode-scalar (rune) indexes**, and array-to-string/string-to-array conversions have the exact safe borrowed/owning and fallible contracts specified below.

**§ 1(2).** `string` is **not a generic binary buffer**, even though its implementation stores UTF-8 bytes. A caller may store or transport arbitrary invalid bytes in binary storage without any UTF-8 check. No text type shall be fabricated before successful decoding or a separate explicitly specified transcoding operation.

**§ 1(3).** The rules do not introduce `Blob`, `Box[T]`, `Owner[T]`/`Owned[T]`, new `vector` semantics, a mutable string byte view, a new encoding enum, automatic character-set detection, automatic Unicode normalization, or an implicit conversion from arbitrary binary input to `string`. These belong to other design questions if needed.

**§ 1(4).** `char` remains a value in 0..255 and `rune` remains a Unicode scalar. `char[]` may preserve values outside ASCII, including `0xFF`; this does not imply that such a sequence is ASCII or valid UTF-8, nor does it select Latin-1 automatically. MD-073 records overflow for a scalar that cannot fit in `char`; that decision's exact public API and error-family synchronization remain separate.

## § 2. String representation and invariant (MD-050)

**§ 2(1).** Every **valid** `string` value in safe Sec code represents a sequence of Unicode scalar values encoded in *well-formed canonical UTF-8*. Invalid leading bytes, malformed continuation sequences, overlong encodings, encoded surrogates, and scalar values beyond U+10FFFF are not permitted in a `string`.

**§ 2(2).** This is a type invariant enforced at **all text-creation boundaries**, including string literals, binary decoding, string-returning runtime/FFI helpers and any compiler-known path that constructs or materializes `string`. Merely writing `unsafe` does not authorize ordinary code to present malformed bytes as a valid `string`. If privileged/FFI code bypasses validation, it incurs an explicit trusted safety obligation to establish the invariant before exposing the value; violations are not ordinary recoverable UTF-8 inputs.

**§ 2(3).** NUL (U+0000), control characters, combining marks and non-ASCII scalars are all valid `string` content. No zero-termination is implied. UTF-8 validity does not imply a Unicode normalization form, locale-dependent collation, case folding, grapheme segmentation, safety for a particular downstream protocol or textual policy.

**§ 2(4).** `string.Len` is the number of Unicode scalars. `string.SizeOf` is the number of encoded UTF-8 bytes. These facts are independent and must be carried through Sema, Semantic IR, Sec MLIR, LSP and runtime helpers. The prior MD-058 decision is unchanged.

**§ 2(5).** A malformed UTF-8 byte sequence received from a socket, HTTP request, file or another untrusted source is **valid binary input**. It must not cause a panic, memory violation, assertion failure or server-wide crash merely because it cannot be decoded as text. A requested *strict text conversion* can return a typed decoding error; the surrounding IO/protocol layer decides whether to reject a field, keep the body binary, log, or respond according to its own protocol specification.

**§ 2(6).** UTF-8 is the canonical encoding of `string`. ASCII, Latin-1, Windows code pages and UTF-16 do not become UTF-8 through reinterpretation or guessing. Non-UTF-8 text requires explicit decoding/transcoding facilities, with ownership, allocation and failure rules specified by their owning library. No `char*`-style or zero-terminated representation is introduced here.

## § 3. Rune positions at the public string API (MD-071)

**§ 3(1).** The following public members accept and/or return **zero-based Unicode-scalar indexes**, not UTF-8 byte offsets:

- `IndexOf(value: string) Option[uint]`
- `IndexOf(value: string, start: uint) Option[uint]`
- `LastIndexOf(value: string) Option[uint]`
- `Slice(start: uint, end: uint) Result[string, RangeError]`
- `GetRune(index: uint) Result[rune, IndexError]`
- `RuneAt(index: uint) rune`

**§ 3(2).** `Slice(start, end)` uses a half-open rune range `[start, end)`. Every index in `0..self.Len` denotes a boundary between complete UTF-8 scalars. `Slice` shall not cut a scalar in the middle. Existing `RangeError.StartAfterEnd` and `RangeError.OutOfBounds` are used for reversed and out-of-range rune ranges; no new scalar-boundary error is needed for this public API.

**§ 3(3).** `IndexOf(value, start)` searches from the specified rune index and returns the rune index of the first exact substring match. Out-of-range `start` returns `None`, not a newly introduced error. For an empty search value, return `Some(start)` when `start <= self.Len`, and `None` otherwise.

**§ 3(4).** `IndexOf(value)` is equivalent to `IndexOf(value, 0)`. `LastIndexOf(value)` returns the rune index of the last exact match; for an empty search value it returns `Some(self.Len)`. These are ordinal Unicode-scalar sequence searches. They do not normalize, fold case, or apply locale-sensitive rules.

**§ 3(5).** The *separately named byte-access members* `GetByte(index)`, `ByteAt(index)` and the currently existing byte-oriented `CharAt(index)` continue to use UTF-8 **byte indexes** while those members remain supported. They must be documented honestly as access to encoded bytes, not as rune indexes or Unicode scalar slicing. Whether `CharAt` should be renamed, deprecated or redesigned is not decided here. No writable string byte reference is granted.

**§ 3(6).** A compiler or implementation may search/scan UTF-8 bytes internally, including efficient byte-based substring matching. It must translate the externally visible positions to rune coordinates and may not expose internal byte offsets as the declared return values. Maintaining byte and rune counters in a lazy iterator is allowed, and need not allocate.

**§ 3(7).** This applies to **`string` only**. It does not redefine array, `vector`, list, binary view, or `char[]` positions as rune indexes. An unrelated binary `IndexOf` continues to use the index semantics of its own type.

## § 4. Public affected string declarations and LSP documentation

**§ 4(1).** The following are the exact affected member signatures. They are associated/instance declarations of built-in `string` provided through canonical trusted core and the compiler-known registry, **not constructors** or convenience aliases. Each public source declaration shall retain a structured source documentation comment immediately before the member; the examples below give the required semantics and member-local comment content. They are *declaration signatures* for normative API documentation, not partial implementation bodies.

```sec
impl string {
    /**
     * Finds the first exact occurrence of value.
     *
     * Parameters:
     *   - value: Text to find; an empty value matches rune position zero.
     * Returns:
     *   Some(zero-based rune index) or None if no match exists.
     * Allocation:
     *   Does not allocate.
     */
    fn IndexOf(value: string) Option[uint]

    /**
     * Finds the first exact occurrence of value at or after start.
     *
     * Parameters:
     *   - value: UTF-8 text to find, interpreted as Unicode scalars.
     *   - start: Zero-based rune position, inclusive.
     * Returns:
     *   Some(rune index) or None, including an out-of-range start.
     * Allocation:
     *   Does not allocate.
     */
    fn IndexOf(value: string, start: uint) Option[uint]

    /**
     * Finds the last exact occurrence of value.
     *
     * Parameters:
     *   - value: UTF-8 text to find; empty text matches at self.Len.
     * Returns:
     *   Some(zero-based rune index) or None if no match exists.
     * Allocation:
     *   Does not allocate.
     */
    fn LastIndexOf(value: string) Option[uint]

    /**
     * Produces text from the half-open rune interval [start, end).
     *
     * Parameters:
     *   - start: Inclusive Unicode-scalar index.
     *   - end: Exclusive Unicode-scalar index.
     * Returns:
     *   The exact selected text without normalization.
     * Errors:
     *   RangeError.StartAfterEnd or RangeError.OutOfBounds.
     */
    fn Slice(start: uint, end: uint) Result[string, RangeError]

    /**
     * Returns a Unicode scalar at the specified rune index.
     *
     * Parameters:
     *   - index: Zero-based rune index, not a byte offset.
     * Errors:
     *   IndexError.OutOfBounds when index is not below self.Len.
     */
    fn GetRune(index: uint) Result[rune, IndexError]

    /**
     * Returns a Unicode scalar at the specified rune index.
     *
     * Parameters:
     *   - index: Zero-based rune index, not a byte offset.
     * Panics:
     *   When index is not below self.Len.
     */
    fn RuneAt(index: uint) rune
}
```

**§ 4(2).** Do not replace or duplicate the existing members with new names solely to distinguish byte/rune units. Update their existing comments and actual lowering. Public byte-member contracts must explicitly say *encoded UTF-8 byte index*.

**§ 4(3).** Documentation and hover for any API returning a `uint` position must not silently imply byte offsets merely because its storage implementation uses byte offsets. `IndexOf`'s return `uint` measures runes; `GetByte`'s argument `uint` measures bytes. This unit distinction belongs in the LSP descriptions and diagnostics.

## § 5. Associated string materialization from borrowed sequences (MD-072)

**§ 5(1).** The two canonical associated functions have exactly these parameter and result types. They **borrow** the input array and do not consume or mutate it. Their resulting string must remain valid after the source array is moved, modified (when no borrow exists) or destroyed.

```sec
impl string {
    /**
     * Decodes exactly the represented bytes as strict UTF-8 text.
     *
     * Parameters:
     *   - value: Borrowed byte array; no terminator or capacity bytes are read.
     * Returns:
     *   An independent, valid UTF-8 string on success.
     * Errors:
     *   StringError.InvalidUtf8(byteOffset) for malformed UTF-8.
     *   StringError.Allocation(error) if text storage cannot be acquired.
     * Allocation:
     *   May allocate; uses the active permitted allocation context.
     */
    fn FromByteArray(value: ref byte[]) Result[string, StringError]

    /**
     * Encodes an array of Unicode scalars as canonical UTF-8 text.
     *
     * Parameters:
     *   - value: Borrowed rune array; every rune is already a valid scalar.
     * Returns:
     *   An independent UTF-8 string preserving scalar order.
     * Errors:
     *   StringError.Allocation(error) if text storage cannot be acquired.
     * Allocation:
     *   May allocate; uses the active permitted allocation context.
     */
    fn FromRuneArray(value: ref rune[]) Result[string, StringError]
}
```

**§ 5(2).** `FromByteArray` interprets its entire represented input as **UTF-8**, never as Latin-1, ASCII-with-extended-bytes, platform default encoding or an unspecified binary string. For invalid UTF-8 it returns `Err(StringError.InvalidUtf8(offset))`, where `offset` is the zero-based **byte offset of the leading byte of the first malformed/truncated UTF-8 sequence** (or the first immediately invalid byte); it does not replace, truncate, skip, panic or return a partially decoded `string`.

**§ 5(3).** `FromRuneArray` encodes scalar values to canonical UTF-8. In safe Sec every `rune` is already valid; ordinary input cannot trigger `InvalidCodePoint`. An invalid runtime `rune` fabricated by broken privileged code is an invariant violation, not a reason to weaken the safe API.

**§ 5(4).** In both conversions, normal fallible materialization may use the pre-existing `StringError.Allocation(AllocationError)` member. No new error family or invented combined error is required. Neither method adds a NUL terminator, normalization or case change.

**§ 5(5).** Borrowed-input creation **must not adopt or alias the source array's mutable backing**. If an implementation uses a sharing optimization, the observable result must still be independently owned and immutable, with full alias/ownership guarantees; no implicit reference-counted runtime is required or assumed. This revision does **not** add a consuming overload for zero-copy adoption.

**§ 5(6).** A failure during allocation or validation leaves the borrowed source unchanged and valid. Validation, allocation and materialization must not expose a partially formed `string` value or leak storage. With a binary network or HTTP input, malformed text is an ordinary recoverable conversion failure, not a fatal parser assertion.

## § 6. Materialization from `string` to owning arrays (MD-072)

**§ 6(1).** The two canonical instance methods have these exact signatures and document their allocation/failure contract separately for LSP:

```sec
impl string {
    /**
     * Materializes the UTF-8 encoding into a separate owning byte array.
     *
     * Returns:
     *   Exactly self.SizeOf bytes in encoded order; no terminator.
     * Errors:
     *   CollectionError for unavailable or insufficient result storage.
     * Allocation:
     *   May allocate; the returned mutable array never aliases string storage.
     */
    fn ToByteArray() Result[byte[], CollectionError]

    /**
     * Materializes the text into a separate owning rune array.
     *
     * Returns:
     *   Exactly self.Len Unicode scalar values in order.
     * Errors:
     *   CollectionError for unavailable or insufficient result storage.
     * Allocation:
     *   May allocate; the returned mutable array never aliases string storage.
     */
    fn ToRuneArray() Result[rune[], CollectionError]
}
```

**§ 6(2).** Both return owning arrays with storage independent of the source `string`, subject to the ordinary owned-array rules. Callers may mutate or move those arrays without altering the string. No hidden mutable alias to the string's UTF-8 storage is permitted.

**§ 6(3).** `ToByteArray` returns the **UTF-8 encoding already represented by the string** and does not transcode to another character set. Its element count is `self.SizeOf`. `ToRuneArray` decodes the guaranteed-valid UTF-8 into exactly `self.Len` scalar values. Neither method has an `InvalidUtf8` error path for a valid input `string`.

**§ 6(4).** Materialization failures use the existing `CollectionError` family, consistent with array allocation/growth. Do not silently discard failed storage operations, reinterpret them as successful empty arrays, or use an unrelated `StringError` solely because the source is a string.

**§ 6(5).** This correction does not implicitly change byte-, char- or rune-sequence `ToString()` signatures. Existing universal `ToString() -> Result[string, StringError]` must remain consistent with the UTF-8 invariant: raw byte-sequence `ToString()` cannot silently yield malformed strings. Synchronize sequence-specific documentation, registry, and tests accordingly; a distinct conversion error contract must not be invented by implementation. MD-073 separately owns the `char` representability error details.

## § 7. Implementation defects and required fixes in `sec/core/string.sec`

**§ 7(1).** **Confirmed existing defect: `StringSplitIterator.Next()` advances a byte position using a rune length.** Current source holds `self.position` as a byte offset, computes `nextPosition := index + self.separator.Len`, and feeds it to `__StringSliceUnchecked`, which takes byte offsets. For a multibyte separator, `.Len != .SizeOf`, so the result can be truncated or sliced at an invalid UTF-8 boundary. Replace this mixed-unit arithmetic with exact byte/rune coordinate handling and scalar-boundary-preserving slices. The defect exists even before `IndexOf` is migrated to rune results.

**§ 7(2).** **Additional migration defect:** `StringSplitIterator.Next()` currently passes `self.position` (a **byte** offset) as the `start` argument of `self.source.IndexOf(...)` and then treats the returned `index` as another byte offset. After §3, **both** directions become wrong: the supplied start must be a **rune** index, and the returned match position is a **rune** index. Use an internal UTF-8 search primitive that accepts/returns byte offsets while preserving scalar boundaries, or explicitly maintain and correctly convert both coordinate systems before byte slicing. Merely replacing `.Len` with `.SizeOf` does not repair this mixed-unit bug. Do not use `index + separator.SizeOf` unless `index` is itself a byte offset.

**§ 7(3).** **Additional migration defect:** `SplitOnce` currently uses the result of `IndexOf` directly as a byte slice endpoint and adds `separator.SizeOf`. Migrate it to an internal byte coordinate or convert from the returned rune coordinate. The returned pair must contain complete Unicode scalars on both sides, with no data loss or extra bytes.

**§ 7(4).** `IndexOf(value, start)` and `LastIndexOf(value)` currently use byte indexes/byte lengths and expose them as public positions. Migrate them to rune-index input/output while allowing internal byte scanning. Their empty-needle cases must use `self.Len`, not `self.SizeOf`.

**§ 7(5).** `Slice(start, end)` currently checks against `self.SizeOf` and directly forwards those values as byte offsets. Interpret inputs as rune positions, check against `self.Len`, resolve corresponding scalar-boundary byte offsets, and call the private byte helper only with the resolved valid byte bounds. No public byte-mid-scalar slicing is permitted.

**§ 7(6).** `GetRune(index)` currently **implements rune indexing**, but its preceding comment incorrectly says "byte index". Correct the LSP-visible source comment. Retain separately documented byte semantics for `GetByte` and `ByteAt`.

**§ 7(7).** `FromByteArray` already performs strict validation, but its comment says a string "should be able to hold binary data". Remove that outdated design direction and state the UTF-8 invariant explicitly. Audit its final `value.ToString()` materialization to guarantee that it preserves **validated bytes exactly**, exposes no mutable alias, and propagates allocation failure through the approved `StringError` variant.

**§ 7(8).** `FromRuneArray` currently contains a TODO and returns `Ok("")` rather than encoding its input. Implement scalar-to-UTF-8 materialization with the exact result/error signature in §5 and tests for empty, ASCII and multibyte runes.

**§ 7(9).** `ToRuneArray` currently has a fallible array-return implementation compatible in shape with §6. Verify that its append errors are handled and propagate `CollectionError`. Implement/verify `ToByteArray` with its exact declared signature; do not advertise an absent body as completed.

**§ 7(10).** These are distinct implementation and documentation defects, not proof that unrelated algorithms such as `Compare` are incorrect. `Compare`'s ordinal UTF-8 byte comparison may remain an optimized implementation of Unicode scalar lexicographic ordering for guaranteed-valid UTF-8 strings, subject to tests.

## § 8. Illustrative examples and observable expectations

**§ 8(1).** These are Sec examples and expected outcomes, not API additions:

```sec
let text := "Aπ😀b"

// Four Unicode scalars and eight UTF-8 bytes.
let runes := text.Len          // 4
let bytes := text.SizeOf       // 8

let position := text.IndexOf("😀")   // Some(2)
let last := text.LastIndexOf("b")   // Some(3)
let prefix := try text.Slice(0, 2) // "Aπ"
let middle := try text.Slice(1, 3) // "π😀"
let empty := try text.Slice(4, 4)  // ""

let encoded := try text.ToByteArray()
let reconstructed := try string.FromByteArray(ref encoded)
let scalars := try text.ToRuneArray()
let reconstructed2 := try string.FromRuneArray(ref scalars)
```

**§ 8(2).** For a byte array holding `[0xFF]`, `string.FromByteArray` returns `Err(StringError.InvalidUtf8(0))`. The byte array remains unchanged. The same byte may validly be stored, transported and compared in binary containers without any text conversion.

**§ 8(3).** For `"Aπ😀b"`, `GetByte(3)` refers to encoded byte `0xF0`, while `GetRune(2)` refers to rune U+1F600. These distinct units must be visible in documentation.

**§ 8(4).** An empty search matches at valid rune boundaries; `"Aπ😀b".LastIndexOf("")` returns `Some(4)`, not `Some(8)`.

## § 9. Testing and conformance gates

**§ 9(1).** Compiler/core tests shall cover valid UTF-8: ASCII, two-, three- and four-byte scalars, control characters, embedded NUL, combining sequences, adjacent multibyte scalars, and empty strings. Ensure `.Len` counts scalars and `.SizeOf` counts bytes consistently at compile time and runtime.

**§ 9(2).** Negative UTF-8 tests shall include standalone `0xFF`, continuation bytes at leading positions, overlong sequences, truncated 2/3/4-byte sequences, encoded surrogate ranges, values beyond U+10FFFF, and combinations near buffer ends. Each strict decoding failure shall report an in-bounds deterministic first-invalid-sequence byte offset and preserve the original buffer.

**§ 9(3).** Position tests shall include start/end of text, out-of-range search starts, empty needle, empty slice, reversed range, overflow/oversized `uint` arguments, rune indexes differing from byte offsets, and searching the same multibyte substring multiple times. Test `IndexOf`, `IndexOf(start)`, `LastIndexOf`, `Slice`, `GetRune` and `RuneAt` against one shared rune-index oracle.

**§ 9(4).** Split regression tests must use a **multibyte separator**, not merely multibyte text with an ASCII separator; test `Split`, `SplitToArray`, `SplitOnce`, retained-separator behavior where supported, consecutive separators, beginning/end separators and the empty-separator path. Assert every yielded substring remains valid UTF-8 and that reconstructing with the appropriate separator policy preserves the original scalar sequence.

**§ 9(5).** Materialization tests shall verify exact round trips, independent mutable-array storage, unchanged borrowed inputs, allocation-failure propagation using the exact error family, and that `ToByteArray` produces canonical UTF-8 bytes without extra NUL or unused capacity. Test conversions across different allocation profiles under their existing rules; do not invent a new fallback allocator.

**§ 9(6).** HTTP/IO regression tests shall demonstrate that malformed binary input can be received without constructing a string, that strict text decoding reports a typed ordinary error, and that the error does not panic/assert or take down unrelated server work. Protocol-specific response and recovery behavior remain owned by HTTP/IO libraries, not these string rules.

**§ 9(7).** Negative compile tests shall reject APIs that imply an invalid implicit binary-to-string cast, treat a rune index as an explicitly byte-typed offset without translation where units are tracked, or expose a mutable reference into `string`'s encoded bytes.

**§ 9(8).** Verify the exact member registry, Semantic IR / P23 lowering, cold/incremental equivalence, LSP hover and diagnostics, formatting of the declarations, and valid source under the compiler. No implementation should claim the MDs closed merely because these source rules are approved.

## § 10. Integration plan and governance

**§ 10(1).** Update `rules/types/types.md`: `string` valid UTF-8 invariant, byte/rune/char distinctions, no implicit encoding inference. Update `rules/library/core-library.md` §5.1–5.6 and §15.1: replace provisional from/to signatures; add explicit rune positions for public string search and slicing; preserve existing error variants, their meanings and documentation; update status notes. Extend its minimum member inventory for existing real overloads as necessary without inventing placeholders.

**§ 10(2).** Update `rules/compiler/compiler_known_members.md`: canonical borrowed and fallible array materialization signatures, exact byte-vs-rune coordinate contracts, valid UTF-8 round trips, proper association with the trusted core source declarations, and current implementation statuses. Reconcile byte/rune/char sequence `ToString()` rules with the universal `StringError` result and string validity invariant.

**§ 10(3).** Update `sec/core/string.sec` under §7, its per-member structured comments, semantic compilation and test fixtures. Synchronize `sec/stdlib/str/stdlib_str.md` and relevant IO/HTTP source APIs where their actual conventions depend on byte offsets. No new public binary container or encoding API is mandated by this correction.

**§ 10(4).** Update Sec MLIR package P23/public string offset and materialization contracts, Sema/Semantic IR facts, runtime helpers and compile-time evaluation. Byte-offset private intrinsics remain private and must receive verified byte coordinates. Incremental caches must invalidate incompatible summaries/semantic facts rather than reusing stale byte-index interpretations.

**§ 10(5).** Reconcile **existing** governance entries by their canonical IDs: record language-design decisions, partial core source support, confirmed defects, tests and validation distinctly. Do not duplicate the old missing-decision records in governance or mark unvalidated implementation as complete. Respect existing per-book fragment layout; do not modify legacy historical status YAML as current authority.

**§ 10(6).** Once normative rules, implementation, tests and governance are synchronized, MD-050, MD-071 and MD-072 can be removed from `missing-decisions.yaml` under its existing removal policy. MD-073 and MD-074 are not closed by this correction. The detailed generic binary storage ergonomics and any optional future consuming zero-copy conversions remain distinct possible future work.
