# MD-092 — Preserve `string.CharAt` as a byte-indexed convenience alias

- **Decision:** MD-092 — `string-charat-byte-access-name`
- **Status:** **Owner-approved — normative correction pending repository integration; existing implementation to be simplified where appropriate**
- **Created:** 2026-10-27
- **Last updated:** 2026-10-27
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Reference repository:** `YoePro/sec`, `main` at `5426af6cf8cce6fe85ff897b97050dfd330ef4fe` (2026-10-26)
- **Pending correction location:** `rules/corrections/MD-092-string-CharAt-byte-alias-correction.md`
- **Archive after normative integration:** `rules/corrections/applied/MD-092-string-CharAt-byte-alias-correction-YYYYMMDD.md`
- **Primary normative owner:** `rules/library/core-library.md`, especially § 5.3 and § 5.7
- **Existing implementation:** `sec/core/string.sec`, `impl string`, `CharAt(index: uint) char`
- **Related contracts:** `rules/types/types.md` (`char`, `rune`, `byte`); MD-050/MD-071/MD-072 string-position correction; UTF-8 string invariants
- **Implementation/governance:** update the existing core-string owners, tests, and documentation; do not create duplicate governance ownership
- **Tracking:** `missing-decisions.yaml` MD-092
- **Decision authority:** **Explicitly approved and reiterated by the Sec language owner on 2026-10-27.** Retention of `CharAt()` is intentional. This correction supersedes suggestions to rename, remove, or deprecate it.

> **Integration requirement:** Preserve `CharAt()` as part of the public `string` API. This is a convenience alias for conversion of one UTF-8 byte to `char`, **not** a Unicode scalar reader. Do not reopen its removal as a design question. Integrate the rulebook and tests; simplify the implementation to delegate to `ByteAt()` when valid. Preserve stable existing paragraph identifiers.

---

## § 1. Purpose and disposition

**§ 1(1).** MD-092 is **decided**: `string.CharAt(index: uint) char` remains a public convenience method in Sec 0.1. The decision is a **retention and semantic clarification**, not a migration or deprecation proposal.

**§ 1(2).** The method exists specifically so callers need not repeatedly write `char(text.ByteAt(index))`. The compiler and library must not remove it because its name could be interpreted as a Unicode-character operation. That interpretation is excluded by its documented contract.

**§ 1(3).** The separate `ByteAt` and `RuneAt` APIs also remain. `CharAt` is an additional useful spelling; it does not replace or weaken either one.

## § 2. Exact API and definition

**§ 2(1).** The canonical public signature is:

```sec
impl string {
    fn CharAt(index: uint) char
}
```

**§ 2(2).** For every valid index `index`, its value is exactly equivalent to:

```sec
char(text.ByteAt(index))
```

The variable `text` denotes the receiving string value. `CharAt` does **not** decode a UTF-8 scalar, inspect adjacent bytes, normalize text, reinterpret locale or grapheme boundaries, or turn a multi-byte scalar into one rune.

**§ 2(3).** A valid index satisfies `index < text.SizeOf`. Indices are **zero-based UTF-8 byte positions**, not rune positions and not character/grapheme positions. Every encoded byte is independently addressable by this API, including continuation bytes.

**§ 2(4).** The result is a Sec `char` whose numeric value is that encoded byte's value in the range `0..255`, following the existing explicit conversion `char(byteValue)`. It is **not** an assertion that the byte is a complete Unicode scalar.

**§ 2(5).** For `index >= text.SizeOf`, `CharAt` exhibits exactly the same bounds/panic contract as `ByteAt(index)`. It must not independently change the exception/error taxonomy, return `Result`, or introduce a second bounds-checking policy.

**§ 2(6).** `CharAt` is read-only, does not allocate, does not mutate the receiver and is a pure convenience method except for the established out-of-bounds panic. It has no special ownership or lifetime behavior.

## § 3. Relationship to `ByteAt` and `RuneAt`

**§ 3(1).** The canonical distinctions are:

| Member | Coordinate system | Result | Purpose |
| --- | --- | --- | --- |
| `ByteAt(index)` | UTF-8 byte index | `byte` | Direct encoded-byte access |
| `CharAt(index)` | UTF-8 byte index | `char` | Convenience alias for `char(ByteAt(index))` |
| `RuneAt(index)` | Unicode scalar/rune index | `rune` | Decoded scalar access |
| `GetByte(index)` | UTF-8 byte index | `Result[byte, IndexError]` | Fallible encoded-byte access |
| `GetRune(index)` | Rune index | `Result[rune, IndexError]` | Fallible decoded-scalar access |

**§ 3(2).** `string.Len` counts runes; `string.SizeOf` counts encoded UTF-8 bytes. The correct bound for `CharAt` is **`SizeOf`** even when `Len` differs. Existing callers that mix these coordinate systems must be audited on their own terms, but that is **not** justification for deleting or changing `CharAt`.

**§ 3(3).** No implicit conversion of `CharAt` results to `rune` is introduced. The method preserves `char` as the existing `0..255` type and the existing byte-to-char conversion contract.

## § 4. Canonical implementation

**§ 4(1).** The preferred Sec implementation is the short delegation:

```sec
/**
 * Returns the encoded UTF-8 byte at the given byte index as a char.
 *
 * Description:
 *   Equivalent to char(self.ByteAt(index)). This is byte access,
 *   not Unicode-scalar decoding.
 * Parameters:
 *   - index: Zero-based UTF-8 byte index below self.SizeOf.
 * Panics:
 *   - When index is not below self.SizeOf, as for ByteAt.
 * Allocation:
 *   - Does not allocate.
 */
fn CharAt(index: uint) char {
    return char(self.ByteAt(index))
}
```

**§ 4(2).** The example is intended to replace duplicate direct `Ptr.Offset(...).Read()` and bounds-check logic in the current `sec/core/string.sec`. If the compiler currently blocks this exact spelling for an unrelated incomplete lowering feature, preserve the equivalent existing implementation and track the implementation limitation separately; **do not reinterpret that as a language-design reopening**.

**§ 4(3).** `ByteAt` remains the single semantic authority for retrieving a byte and handling invalid positions. A compiler optimization may inline either member provided it preserves the same observable behavior.

## § 5. Existing callers and compatibility

**§ 5(1).** Existing `CharAt` calls in core, `sec/stdlib/net/url`, `sec/stdlib/net/http`, `sec/stdlib/net/ip`, and other modules must remain valid after this correction. **No automatic rename or bulk migration is required.**

**§ 5(2).** Do not introduce deprecation warnings, removal schedules, a replacement-only path, or diagnostics urging callers to use `RuneAt` or `ByteAt` merely because they called `CharAt`.

**§ 5(3).** Independently identified index-unit bugs in callers may be corrected using their actual intended coordinate system. A caller scanning encoded ASCII bytes may legitimately use `CharAt`; a caller scanning Unicode scalars must use rune-based APIs. These are ordinary call-site correctness questions, not part of MD-092's API decision.

## § 6. Required normative integration

**§ 6(1).** In `rules/library/core-library.md` § 5.3, explicitly include `fn CharAt(index: uint) char` among the public `impl string` members. The listed minimum surface currently names `ByteAt` and `RuneAt`, while `CharAt` appears only in the prose of § 5.7; keep the public inventory and its actual implementation consistent.

**§ 6(2).** In § 5.7, replace the sentence claiming that `CharAt`'s name or future is undecided with the **settled** contract in §§ 1–3 of this correction. Preserve § 5.7's existing rune-index rules and the separate byte-index rules for `GetByte` and `ByteAt`.

**§ 6(3).** In `sec/core/string.sec`, remove the documentation caveat that renaming or redesigning `CharAt` remains undecided; add the explicit delegation/equivalence and valid index domain. Prefer § 4's implementation.

**§ 6(4).** If core-member registries, hover/LSP descriptions, API reference tables, or compiler-known inventories mention `CharAt`, update them consistently. Do not invent a new compiler intrinsic for this wrapper.

**§ 6(5).** Do not renumber existing rulebook sections or normative paragraphs. Integrate by updating the paragraphs that already own this contract and append distinct new identifiers only if necessary.

## § 7. Verification and regression tests

**§ 7(1).** Required positive cases include ASCII bytes, NUL (`0`), bytes with values above `127`, and each byte of a multi-byte UTF-8 scalar. For each valid byte index, assert `text.CharAt(i) == char(text.ByteAt(i))`.

**§ 7(2).** Required negative case: `i == text.SizeOf` and `i > text.SizeOf` must produce the same out-of-bounds panic behavior as `ByteAt`. A byte index in the middle of a scalar is **valid for this byte-oriented API** and must not be rejected as a non-rune boundary.

**§ 7(3).** Check that `CharAt` returns `char`, `ByteAt` returns `byte`, `RuneAt` returns `rune`, and no signature or indexing-unit drift occurs.

**§ 7(4).** Existing `CharAt` call sites must continue to type-check. A focused test may assert that the implementation delegates through `ByteAt` or otherwise has equivalent behavior, but tests must not force an implementation detail over the normative alias contract.

**§ 7(5).** Where supported by the compiler, run core/string tests, relevant stdlib/net regressions, source/type checks, and compiler-known/API/LSP consistency checks. Report unsupported-lowering limitations separately rather than fabricating success.

## § 8. Governance and closure

**§ 8(1).** This correction decides MD-092 normatively. Its tracking entry may be removed from `missing-decisions.yaml` only after the primary rulebook reflects the decision and applicable governance is synchronized. Implementation status and tests remain separately tracked where incomplete.

**§ 8(2).** Use the existing core/string and compiler-known governance owners; no new duplicate governance IDs are created by this correction.

**§ 8(3).** When integration is complete, archive this file under `rules/corrections/applied/` with an actual integration date and maintain its paragraph numbers and decision provenance.

**§ 8(4).** **Owner decision is final for Sec 0.1:** `CharAt()` remains as the convenience alias `char(self.ByteAt(index))`. No subsequent MD-092 work should propose removing, renaming, or deprecating it absent an explicit *new* owner-initiated reconsideration.
