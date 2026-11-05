# MD-126: Iterating a byte range of a string — proposal

- **Status:** Superseded 2026-11-05 by the owner decision `rules/corrections/applied/MD-126-borrowed-string-ranges-rune-iteration-applied-20261105.md` (rune-index string ranges yielding a borrowed `StringView`; the `SliceBytes` option A was not chosen). Kept as a historical proposal.
- **Created:** 2026-11-04
- **Sec version:** current baseline 0.1.1; the proposed form targets 0.1.2
- **Related:** MD-071 (rune positions; applied in
  `md050-md071-md072-string-utf8-positions-materialization-correction-20261017.md`),
  MD-090 (str edit position unit, open), MD-092 (`CharAt`), MD-052 (private
  low-level bodies), flowcontrol_for.md § 19, collections.md § 7

## § 1 Question

`sec/core/number_parse.sec` validates a suffix given as byte offsets
`startByte..endByte`. The intended code, written in the style of another
language, is:

```sec
// startByte/endByte lie on scalar boundaries, as the caller normally guarantees.
// The slice is O(1) and only the interesting range is iterated.
for character in source[startByte..endByte] {
    if character.IsUnicodeDigit || !character.IsLetter {
        return Err(NumberParseError.InvalidNumber)
    }
}
```

The question is how to write the equivalent in Sec 0.1.1, and whether 0.1.2
should add a form for it.

## § 2 Current rules (0.1.1)

**§ 2(1).** `string` stores canonical UTF-8. `Len` counts runes and `SizeOf`
counts bytes (MD-058, MD-071 § 2).

**§ 2(2).** Public string positions are rune indexes:

- `Slice(start, end) Result[string, RangeError]` takes a half-open rune range
  (core-library § 5.7; MD-071 § 3).
- Only the separately named byte members `GetByte`, `ByteAt` and `CharAt` take
  encoded byte indexes (MD-071 § 3(5)).
- Externally visible positions must be rune coordinates (§ 3(6)).
- No member may be duplicated solely to distinguish units (§ 4(2)).

**§ 2(3).** `source[a..b]` is not valid on a string:

- A slice borrows a contiguous range of stored elements of one type
  (collections.md § 7.2).
- A string is not a source of slices, and its runes are not stored elements.
- Slice creation is always explicit: `ref values[a..<b]`, or
  `try ref values[a..<b]` in its fallible form (§§ 7.3, 7.5).

**§ 2(4).** String iteration yields decoded `rune` values. `ref` and `ref mut`
rune bindings do not exist (flowcontrol_for.md §§ 19, 40), and S1199 reports
them.

**§ 2(5).** The private `__StringSliceUnchecked` takes verified byte boundaries
and builds a string descriptor with no check and no allocation. It is core-only
(core-library § 1, correction-string-slice-helper).

**§ 2(6).** `rune` has no public member giving its UTF-8 length.

## § 3 What can be written in 0.1.1 without a language change

**§ 3(1). The bounds are rune indexes.** This is the canonical form, and it is
verified to type-check:

```sec
let suffix := try source.Slice(start, end)
for character in suffix {
    if character.IsUnicodeDigit || !character.IsLetter {
        return Err(NumberParseError.InvalidNumber)
    }
}
```

`Slice` maps `RangeError` to the caller's error through `try` handlers.
Finding the byte position of a rune index is O(n) in the prefix. The loop
then visits only the range.

**§ 3(2). The bounds must stay byte offsets.** Iterate the whole string and
keep a byte cursor:

```sec
let mut offset: uint := 0
for character in source {
    if offset >= endByte {
        break
    }
    if offset >= startByte && (character.IsUnicodeDigit || !character.IsLetter) {
        return Err(NumberParseError.InvalidNumber)
    }
    offset += _Utf8Width(character)
}
```

`_Utf8Width` is a small local helper that returns 1, 2, 3 or 4 from the
scalar value, because § 2(6) gives no public member for it. The loop scans the
prefix up to `startByte`. That costs O(startByte) instead of O(1), but the
semantics are correct and allocation-free.

**§ 3(3). Inside core.** Core may use `__StringSliceUnchecked(startByte,
endByte)` when the boundaries are proven. That is the private, unchecked form
of exactly this operation, and it is not available to user code.

## § 4 Options for 0.1.2

**Option A — a checked byte-range slice member (recommended).** Add one
public member next to the existing byte members:

```sec
fn SliceBytes(startByte: uint, endByte: uint) Result[string, RangeError]
```

The member works as follows:

- The two arguments are encoded UTF-8 byte offsets, documented as such
  (MD-071 § 3(5)).
- It checks `startByte <= endByte <= SizeOf` and that both offsets are scalar
  boundaries. A boundary check only inspects the byte at the offset, so the
  whole check is O(1).
- On success it returns a string over the same storage, with no allocation.
  This is the public, checked counterpart of `__StringSliceUnchecked`.
- The example becomes:

  ```sec
  for character in try source.SliceBytes(startByte, endByte) {
      ...
  }
  ```

Advantages:

- It gives O(1) slicing as in the example.
- No new syntax, iteration rule or borrow form is needed.
- The unit is visible in the name.

Cost: it is a deliberate exception to MD-071 § 3(6), because public positions
would again include byte coordinates. That exception is limited to a member
whose name states the unit, the same way `GetByte`/`ByteAt` already are.

**Option B — index-range syntax on string (`source[a..<b]`).** Not
recommended:

- The unit (bytes or runes) is invisible at the call site.
- It contradicts collections.md § 7, where `[a..<b]` borrows stored elements
  and requires an explicit `ref`.
- It would reopen MD-071 for the most common spelling.

**Option C — an opaque string position type (in the style of Swift's
`String.Index`).**

- Searches and iteration produce validated positions.
- Slicing between two positions is O(1) without any public byte or rune
  arithmetic.

This is the most unit-safe design, but it changes search, slicing, `str` (MD-090)
and LSP signatures widely. It fits 0.2 better than 0.1.2.

**Option D — a range-limited iteration form.** One example is
`for character in source.RunesFrom(startByte, endByte)` returning a borrowed
lazy iterator. MD-071 § 3(6) already allows byte and rune counters internally.
In practice it gives the same capability as Option A but adds an iterator
surface. Option A covers it with an ordinary string value.

## § 5 Recommendation

**§ 5(1).** Use § 3(1) now in 0.1.1 wherever the caller can work in rune
indexes, and § 3(2) where the byte offsets are given.

**§ 5(2).** Adopt Option A for 0.1.2 as an additive core member gated to
0.1.2, as for `Err(.Variant...)`.

**§ 5(3).** Leave Option C to the 0.2 design work.

## § 6 Decisions needed from the owner

**§ 6(1). D1 — form.** Option A, B, C or D for 0.1.2 (recommended: A).

**§ 6(2). D2 — boundary error.** A non-scalar byte boundary needs an error.
The choices are:

- reuse `RangeError.InvalidRange` (recommended: no new variant, and the range
  as given is invalid), or
- add a new variant such as `RangeError.NotScalarBoundary`.

**§ 6(3). D3 — MD-071 exception.** Accept that one public member takes byte
offsets, with the unit in its name. This needs an explicit addition to MD-071
§ 3(5) and a statement that § 4(2) is not violated, because `SliceBytes` adds
a new capability rather than duplicating `Slice`.

**§ 6(4). D4 — rune width.** Optionally add `rune.SizeOf` (the canonical
UTF-8 length 1–4) so that § 3(2)-style byte cursors need no local helper. The
recommendation is yes, as a separate small member under the same 0.1.2 gate.

**§ 6(5). D5 — relation to MD-090.** If D1 is A, `str` edit functions can
take byte offsets obtained from `SliceBytes`-compatible boundaries. MD-090
stays a separate decision.

## § 7 Implementation sketch after approval (not started)

- core-library § 5.7: add the member, its contract and the version gate.
- `sec/core/string.sec`: implement it over `__StringSliceUnchecked` after the
  boundary checks.
- Sema: register the compiler-known member, with hover documentation stating
  byte units.
- Semantic IR: P23 string values. Native string materialization is still
  pending, so execution waits on P23 as for every string operation.
- Tests: valid boundaries, a mid-scalar offset, reversed and out-of-range
  offsets, an empty range, and iteration over the result.
