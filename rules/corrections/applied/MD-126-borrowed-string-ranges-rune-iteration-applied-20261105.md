# MD-126 — Borrowed UTF-8 String Ranges and Rune Iteration

| Field | Value |
| --- | --- |
| Document | MD-126 |
| Title | Borrowed UTF-8 String Ranges and Rune Iteration |
| Created | 2026-11-05 |
| Revision | 1.0 |
| Last updated | 2026-11-05 |
| Decision owner | Sec language owner |
| Status | **Owner decision applied to the canonical rulebooks 2026-11-05; compiler, core and stdlib implementation not done** (tracked as remaining items in `frontend.types-core`, `frontend.collections`, `frontend.for-loops-v2`, `frontend.borrowing`, `frontend.errorhandling-v2`, `compiler.compiler-known-surfaces`, `core.string-slice-helper`, `frontend.semantic-ir-v2`) |
| Applies to | Sec 0.1.x; planned implementation gate 0.1.2 (unless owner changes release target) |
| Repository | `YoePro/sec` |
| Audited branch | `main` |
| Audited commit | `0cdacb04fb3757a26569d51c21a5ae445f0facaf` |
| Replaces proposal | `rules/corrections/applied/MD-126-string-byte-range-iteration-proposal-20261104-20261107.md` (proposal only; do not rewrite an applied historical correction) |
| Primary rulebooks | `rules/types/types.md`, `rules/collections/collections.md`, `rules/control-flow/flowcontrol_for.md`, `rules/library/core-library.md`, `rules/compiler/compiler_known_members.md`, `rules/memory/{references,reference_model,borrowing}.md`, `rules/errors/runtime_checks.md` |
| Related decisions | MD-050, MD-058, MD-063, MD-071, MD-072, MD-094, MD-115/116, MD-117/119, MD-126; P16 and P23 |

> **Authority and integration note.** This document records the owner-selected semantic direction from 2026-11-05 and prescribes a coordinated integration. It **does not** claim that `StringView`, string bracket slicing, or its lowering already exist. Preserve existing canonical paragraph numbers: amend named paragraphs or append stable new subparagraphs; never renumber existing paragraphs or reclassify already implemented work as completed without tests. Detailed API additions below are normative integration requirements for implementing the approved behavior, but new externally exposed helper names beyond `StringView`/the existing syntax must be reconciled against the core registry before publication.

---

## § 1. Scope and rationale

**§ 1(1).** `string` remains a built-in, valid, canonical **UTF-8 text** type. It is not `byte[]`, `char[]`, `rune[]`, and no source-visible language rule may reinterpret its storage as an array of `rune` objects.

**§ 1(2).** String range selection must be a **borrowed** view with no character-data copying, allocation, ownership transfer, or independent source-content materialization. Iterating the view decodes one Unicode scalar (`rune`) at a time, as ordinary string iteration already does.

**§ 1(3).** Prefer the existing bracket range notation to a string-specific `Slice`/`RuneSlice` method. An ordinary bracket range has the language's existing panic-capable bounds behavior; `try` selects its recoverable bounds-error form.

**§ 1(4).** This decision **supersedes the proposed public byte-range `SliceBytes`** from the earlier MD-126 proposal. Public string ranges use rune coordinates. The original byte-span use case must be migrated explicitly, not silently converted into rune-index syntax. A separate byte-oriented view API is not approved by MD-126.

**§ 1(5).** It does **not** introduce a logical `ref rune`, a new reference-to-decoded-value category, a heap-allocated rune object, `ref ref rune`, or a new general iterator protocol. Ordinary `for ch in text` continues to give `ch: rune` by value. A `rune` is a scalar (max four UTF-8 encoded bytes; a 32-bit scalar value); the decoder can produce it directly into a register even on a 32-bit CPU.

---

## § 2. Exact source syntax and indexed units

**§ 2(1).** The following source expressions are valid, subject to the source and lifetime requirements below:

```sec
let text := "Aπ😀b"

let first := text[1..2]       // Inclusive rune range: "π😀".
let second := text[1..<3]    // Exclusive rune range: "π😀".
let empty := text[2..<2]     // Valid empty borrowed view.
let tail := text[2..]        // "😀b".
let all := text[..]          // Borrowed view of the entire text.

for ch in text[1..<3] {
    Process(ch)              // ch is rune, not ref rune.
}
```

**§ 2(2).** Endpoints are **zero-based Unicode-scalar (rune) indices** relative to the indexed string/view. `text.Len` counts runes; `text.SizeOf` counts encoded UTF-8 bytes. Never use `SizeOf` as the bound of a rune range.

**§ 2(3).** Retain the *existing* generic range notation from `collections.md` § 7.4, including open endpoints:

- `text[start..end]`: upper endpoint inclusive;
- `text[start..<end]`: upper endpoint exclusive;
- `text[..]`, `text[start..]`, `text[..end]`, `text[..<end]`.

The normalized semantic range is always `[start, endExclusive)`. In particular, `text[text.Len..<text.Len]` is an empty view; `text[text.Len..text.Len]` is out of bounds when the source is not allowed to contain a rune at `Len`.

**§ 2(4).** The source operand is evaluated **once**, then explicit start and end **once each, in source order**, using the canonical range-normalization rules. Inclusive-end normalization must not overflow; constant-invalid ranges receive compile-time diagnostics according to MD-094; dynamically invalid bounds follow § 3.

**§ 2(5).** These rules add **one deliberate string-specific exception** to ordinary array-slice formation: `text[A..<B]` creates a borrowed `StringView` **without a preceding `ref`**. Array/sequence slicing remains `ref values[A..<B]` or `ref mut values[A..<B]`. Do not silently add implicit slice creation to arbitrary collection types.

**§ 2(6).** Range slicing does not change the meaning of a standalone `text[index]`. Single-rune indexing is **outside the scope of this correction** unless separately specified; do not invent its semantics or an assignable `rune` Place while implementing ranges.

---

## § 3. Failure policy: panic versus `try`

**§ 3(1).** An ordinary string range such as `text[A..<B]` checks the interval and follows the established **`panic.bounds` / `slice-range` provenance** when a dynamic bounds failure occurs. It does not manufacture a `Result`, return an empty string, clamp endpoints, or silently discard invalid values.

**§ 3(2).** `try text[A..<B]` is the fallible counterpart. The exact failure type is **`RangeError`**, *not* `StringError`. The success type is `StringView`; `try` participates in the normal propagation/local-handler rules and does not itself leave a `Result[...]` wrapper in the successful binding.

```sec
fn Select(text: ref string, start: uint, end: uint) Result[StringView, RangeError] {
    let selection := try text[start..<end]
    return Ok(selection)
}

fn SelectOrSkip(text: ref string, start: uint, end: uint) void {
    let selection := try text[start..<end] {
        Err(_) => return
    }
    for ch in selection {
        Inspect(ch)
    }
}
```

**§ 3(3).** Preserve existing error identities and precedence (`collections.md` § 7.5 and `runtime_checks.md`): invalid/negative or unrepresentable endpoint -> `RangeError.InvalidRange`; reversed range -> `RangeError.StartAfterEnd`; out of bounds -> `RangeError.OutOfBounds`. Rune ranges always lie on scalar boundaries, so **no new mid-UTF-8-code-point error variant** is needed.

**§ 3(4).** When a statically invalid range is provable, the current compile-time-invalid-range rule takes priority; do not silently convert every provable invalid range into a generated panic. A `try` does not catch unrelated stale-reference or memory-validity failures.

**§ 3(5).** Remove public `string.Slice(start,end)` instead of retaining a duplicated API. Do **not** change `Slice()` on unrelated user types or add a general `.Slice` method to arrays. `try text[A..<B]` is the approved non-panicking replacement.

---

## § 4. `StringView`: type, ownership, lifetime and representation

**§ 4(1).** The result of a string range has the compiler-known, source-visible type **`StringView`** (canonical spelling proposed for integration; the final type spelling must be consistently registered in core, Sema, LSP and Semantic IR before release). It is a **read-only, non-owning bounded view over UTF-8 text**, distinct from an owned/copy-trivial `string` value and from `ref byte[]`/`ref rune[]`.

**§ 4(2).** Every view carries the provenance and lifetime obligation of its originating string's backing storage. A view or aggregate containing a view may be copied *only as a borrow holder*; copying it never copies text and never extends backing lifetime. It is trivially destructible; destroying it does not destroy the source. A view may not escape the proven lifetime of its origin. Compiler-known null/sentinel physical representation for empty views is permitted only under the existing safe bounded-view rules.

**§ 4(3).** Its representation is implementation-defined (for example backing identity + byte start/end, with a cached rune count if useful). **No FFI/ABI-visible descriptor layout is selected.** Logical bounds remain rune coordinates; hidden byte coordinates may be used to traverse valid UTF-8 efficiently.

**§ 4(4).** `StringView` is **not** a true `ref string` to a complete `string` Place, and is **not** `ref rune[]`: neither type could represent the requested substring without changing its settled meaning. No implicit conversion from `StringView` to `string` is introduced; such a conversion would allow a borrow to escape or silently materialize text.

**§ 4(5).** A view of a view uses relative rune indexes and keeps the ultimate source origin. It may narrow, but cannot widen, the source range or lifetime.

```sec
let first := text[1..<4]
let nested := first[0..<2]  // Same source backing; first two runes of first.
```

**§ 4(6).** Allowed sources are a live `string` Place, a valid shared reference to string, or another live `StringView`. The source must satisfy normal borrow creation and lifetime rules. Do not grant general explicit-borrow lifetime extension to arbitrary non-addressable temporaries. The enclosing `for` has its existing one-time iterable-expression evaluation and temporary-retention rules, but these must not be used to fabricate a longer-lived returned view.

**§ 4(7).** String immutability and existing borrow overlap/concurrency rules apply. A view preserves UTF-8 validity because its endpoints are scalar boundaries. UTF-8 is neither normalized nor transcoded. `StringView` has no `ref mut` equivalent in this correction, and no writable string byte/rune element is exposed.

**§ 4(8).** `StringView` has the fully specified public minimum in § 4(8a), as well as bracket ranges and compiler-known rune iteration. No write methods, implicit byte-array methods, or automatically inherited unrelated string members arise.

**§ 4(8a).** The **minimum public `StringView` surface for MD-126** is exactly the following read-only contract; the type itself is compiler-known and cannot be re-declared as an ordinary user `struct`. All operations are non-consuming, and except for `ToString()` none allocates or materializes independent text. A view can also use bracket ranges without a `.Slice()` method. These signatures must be inserted into the canonical core/member registry before public integration:

```sec
impl StringView {
    property Len: uint { get }       // Unicode scalar count within this view.
    property SizeOf: uint { get }    // UTF-8 byte count within this view.

    fn IsEmpty() bool

    fn GetRune(index: uint) Result[rune, IndexError]
    fn RuneAt(index: uint) rune

    fn GetByte(index: uint) Result[byte, IndexError]
    fn ByteAt(index: uint) byte
    fn CharAt(index: uint) char

    fn StartsWith(prefix: StringView) bool

    fn ToString() Result[string, StringError]
}
```

The declarations express required **public contract shapes**, not a claim that the underlying compiler-known type can already be written in source. `Len` and `SizeOf` and all indices are **view-relative**. `IsEmpty` is true exactly when `Len == 0`; `StartsWith` performs ordinal Unicode-scalar prefix comparison without normalization, case folding, or allocation; `GetRune`/`GetByte` report `IndexError.OutOfBounds` outside their respective unit domains; `RuneAt`/`ByteAt`/`CharAt` use ordinary bounds panic. `ByteAt` exposes encoded bytes read-only and does not turn the view into `byte[]`. This is the complete required public minimum for MD-126; additional string-like search/formatting members are deliberately **not** implicitly inherited by name. Explicit materialization follows § 4(9).

**§ 4(9).** Where a **truly independent** `string` is explicitly needed, the conversion must be an explicit text materialization with appropriate allocation and error semantics, not an implicit view-to-string cast. The existing universal `ToString() -> Result[string, StringError]` rule is the preferred explicit surface: `StringView.ToString()` must return its **text content**, not a type-summary fallback. Its implementation may share storage *only if* it preserves independent value lifetime/ownership after the source dies; otherwise it allocates and copies. This operation is separate from cheap range creation.

**§ 4(10).** The normative API minimum and any LSP/CKM registry entries for `StringView` must be written **in full** when applied; do not add placeholders such as “usual string methods”. In particular, `StringView` arguments to an existing `string` parameter are not automatically accepted without a declared overload or materialization.

---

## § 5. Iteration: reuse the compiler's string traversal

**§ 5(1).** `StringView` is a compiler-known **string-family iterable**. Its traversal uses the same Unicode-scalar decoder, order, bounds and lazy state progression as ordinary `string` iteration. This is the current built-in `for ch in string` mechanism, **not** the separate `StringSplitIterator` used by `string.Split(separator)` to iterate substrings.

```sec
fn ContainsRune(input: ref string, wanted: rune) bool {
    for ch in input {
        if ch == wanted {
            return true
        }
    }
    return false
}

fn ContainsRuneInRange(input: ref string, start: uint, end: uint, wanted: rune) bool {
    for ch in input[start..<end] {
        if ch == wanted {
            return true
        }
    }
    return false
}
```

**§ 5(2).** In **both** loops the binding `ch` has type `rune` (a small scalar value), not `ref rune`. Decoding a 1–4-byte UTF-8 sequence to one scalar is not a copy of the string backing. Do not add a logical `ref rune` or a `RawPtr[rune]` escape path.

**§ 5(3).** The `for` source/view is borrowed. Each yielded rune is read and decoded to a scalar; no per-rune heap object is required. String data is not copied. No new `Iterator[ref rune]` or general `Iterable` factory is required. Existing `for` evaluation, `break`, `continue`, cleanup, and one-/two-binding index rules apply; a two-binding index, where supported, is view-relative rune position.

**§ 5(4).** Preserve the generic pack decisions **MD-117/MD-119**. `for value in` a pack of `ref T` elements copies the **reference holder**, and `for ref value` reborrows its referent, never creating `ref ref T`. This does **not** imply that string runes are stored `ref T` elements. A plain `for ch` over a string view decodes a value; a `for ref ch` over string/view remains invalid unless separately decided.

**§ 5(5).** The optimized lowering may hoist byte-boundary translation, decode directly into registers, and retain only a descriptor/cursor. No runtime allocation is necessary for view creation or traversal on 32-bit or 64-bit targets. The creation cost may be O(prefix rune count), since locating rune positions in variable-length UTF-8 need not be O(1). Per-rune traversal is proportional to represented UTF-8 bytes.

---

## § 6. Rulebook amendments — exact ownership

The table below is **integration work**, not a claim that these files already implement MD-126. Amend existing sections using their stable paragraph numbering.

| File | Exact existing anchor | Required normative edit |
| --- | --- | --- |
| `rules/types/types.md` | `# string`, “Valid UTF-8 invariant” | Replace “Array slice syntax does not apply directly to string” with explicit string-range view contract; retain UTF-8 invariant, `Len` rune/`SizeOf` byte distinction, and no array interpretation. Define predeclared `StringView` as separate non-owning UTF-8 type. |
| `rules/foundations/grammar.md` | “Slice expression”, “Index expression”, “Reference expression”; “Iterable for loop” | Preserve grammar, add examples for `text[a..<b]` and `try text[a..<b]`; specify type-directed semantics, with no general implicit array borrowing. Do not add a `RuneSlice` token/keyword. |
| `rules/collections/collections.md` | §§ 7.1–7.7, especially 7.2–7.5 | Distinguish existing `ref T[]` array slices from `StringView`; add string-specific source/implicit borrow and identical normalized ranges/error precedence. Explain nested views and scalar boundaries. Keep array slice rule `ref values[...]`. |
| `rules/control-flow/flowcontrol_for.md` | §§ 4–7, 12–13, 13A, 19, 37, 40 | Add `StringView` to compiler-known iterables, share string decoder, bind rune **by value**, forbid `for ref` for decoded text. Preserve MD-117/119 pack semantics and `StringSplitIterator` distinctness. |
| `rules/library/core-library.md` | §§ 1.5, 5.1–5.7, 13, public string table | Remove public `string.Slice(start,end)` signature and use bracket ranges. Define the minimal `StringView` property/method signatures and their exact units/effects, including explicit content materialization; do not confuse core-privileged byte helper with public range semantics. |
| `rules/compiler/compiler_known_members.md` | “Internal core string-slice helper”, string member registry, effects, borrowed/ownership integration | Update public lookup: remove `Slice` on string, introduce view and range operation identities with correct types/borrow facts. Retain private `__StringSliceUnchecked` only as a core-internal **owned `string` descriptor operation**, not the public borrow constructor, until its own governance migration. |
| `rules/memory/references.md` | §§ 3.3, 5–7, 9, 12–13 | Recognize non-owning bounded text views and their lifetimes/provenance, without claiming a physical `ref rune`. Validate returned view relations against source storage. |
| `rules/memory/reference_model.md` | §§ 2, 12, 14, 19, 23 | Add the string-bounded-view semantic category; do not relax ordinary `ref T` initialized-`T` requirements or reinterpret `ref string` as a view. No raw rune pointer. |
| `rules/memory/borrowing.md` | §§ 5, 14, 23 | Add the explicit *source-kind-specific* bracket-view borrow exception, nesting, escape restrictions, and reborrow/origin facts. |
| `rules/errors/runtime_checks.md` | “Slice checks” | Extend existing `panic.bounds` / `try RangeError` contract and precedence to string-backed ranges; no new error type. |
| `rules/errors/errorhandling.md` | `try` expression boundaries and propagation | Confirm bodyless/local `try text[a..<b]` typechecks against `RangeError`, including use in `for` source; do not implement try as a blanket panic catcher. |
| `rules/compiler/semantic_ir.md` | Range, Place/reference, String operation, return-origin and loop sections | Record a resolved **StringView** origin+range+validity plan, UTF-8 scalar boundaries, normalized rune units, byte cursor where needed, and no-allocate/non-owning effects. Reuse returned-slice origin metadata where valid, without pretending a view is `ref rune[]`. |
| `rules/mlir/semantic-ir/sec_semantic_ir_string_package23.md` | String operations / substring | Add a **borrowed string-range** semantic operation distinct from `StringSubrangeUnchecked` (owned `string` result). |
| `rules/mlir/packages/sec-mlir-dialect_package23.md` | §§ 14–18, 29–36 | Replace outdated “string does not use array slice syntax” and `Slice -> Result[string,RangeError]`; specify zero-copy view, owned materialization only when requested, reused rune iterator, verifier/typed failure edges; keep P23 unfinished as appropriate. |
| `rules/mlir/normative-sync/sec_string_core_sync_package23.md`; package decision index / schema-19 / lowering-19 docs | Public slicing/byte translation and P23 source contracts | Reconcile with the new MD-126 view, preserve `MD-050/071/072` authority. No invented schema-19 implementation claim: repository's currently accepted schema-12 path remains distinct from future P23 plans. |
| `rules/corrections/applied/md050-md071-md072-string-utf8-positions-materialization-correction-20261017.md` | §§ 3–4, 7–10 | **Historical decision record: DO NOT rewrite.** Add a *new* MD-126 supersession reference in canonical owning books, respecting old MD-071 rune-unit authority. |
| `rules/corrections/applied/MD-126-string-byte-range-iteration-proposal-20261104-20261107.md`; `missing-decisions.yaml` MD-126 | Entire old proposal/status | Mark the byte-range proposal superseded by this correction; update the MD-126 decision and integration status. No premature `implemented` claim. |
| `rules/analysis/escape_analysis.md` | `Parameter(0).Slice(2..8)` origin notation | **Do not replace** this abstract provenance-path notation: it is not a source-level call to `string.Slice()`. Add `StringView` provenance handling in analysis where necessary. |

---

## § 7. Audited `.Slice()` source inventory at commit `0cdacb0`

**§ 7(1).** Scope of this inventory: GitHub-indexed direct **`.Slice(` occurrences** in Sec source and the relevant semantic fixture on audited `main`. Exactly **32 source-language member-call sites** were found (30 in five `stdlib/str` files, 1 in `sec/core/number_parse.sec`, 1 in `sec/stdlib/net/http/structured_fields.sec`), plus **1 semantic fixture**. These are source locations at the stated commit and will shift after changes. Other compiler/go-code `.Slice`/`Slice(...)` symbols, abstract origin paths, and the private `__StringSliceUnchecked` are not the same API and must not be globally renamed.

**§ 7(2).** **Migration rules for every call**: (a) preserve original start/end **rune index** units, (b) translate old `Slice(start,end)` to `text[start..<end]` (the old method's upper bound was exclusive), (c) select ordinary panic form only where appropriate or `try text[start..<end]` for recoverable failure, (d) delete obsolete `Err` handlers only after verifying the chosen new failure policy, (e) handle the `StringView` vs owned `string` type boundary at each consumer. No blind regex-only rewrite is acceptable.

### § 7A. `sec/stdlib/str/edit.sec` — 8 calls

| Old line | Current call/result | Required migration |
| ---: | --- | --- |
| 90 | `prefix := try value.Slice(0, _RuneIndexOfBoundary(value, byteIndex))` | `value[0..<_RuneIndexOfBoundary(value, byteIndex)]` -> `StringView`; use view-aware `Builder.Append`. |
| 96 | `suffix := try value.Slice(_RuneIndexOfBoundary(value, byteIndex), value.Len)` | `value[_RuneIndexOfBoundary(value, byteIndex)..<value.Len]` -> view. |
| 166 | Same prefix in `Insert(..., insertion: rune)` | Same change; don't change rune insertion encoding. |
| 172 | Same suffix in rune `Insert` | Same change. |
| 254 | `prefix` for `RemoveRange` (byte start translated to rune) | Keep the explicit `_RuneIndexOfBoundary` conversion; return view. |
| 260 | `suffix` for `RemoveRange` (byte end translated to rune) | Keep explicit conversion; return view. |
| 339 | `prefix` for `ReplaceRange` | View; preserve replacement and edit error semantics. |
| 345 | `suffix` for `ReplaceRange` | View; preserve replacement and edit error semantics. |

The public edit positions are **UTF-8 byte offsets** under open MD-090, while string-range indices are **rune offsets**. Do not remove `_RuneIndexOfBoundary` or silently change public edit units in MD-126. The resulting `prefix/suffix` feed `Builder.Append`, which currently only has `Append(string)` and `Append(rune)`; add an explicit view-capable overload/internal forwarding path before converting these call sites. The output of edit operations remains `Result[string, EditError]` and must not return a borrowed view.

### § 7B. `sec/stdlib/str/replace.sec` — 4 calls

| Old line | Current call/result | Required migration |
| ---: | --- | --- |
| 157 | `before := try value.Slice(position, found)` in `ReplaceN` | `value[position..<found]`, borrowed prefix segment; append through view-capable builder. |
| 176 | `remainder := try value.Slice(position, value.Len)` | `value[position..<value.Len]`, borrowed remainder; append into owned result. |
| 255 | `before := try value.Slice(0, position)` in `ReplaceLast` | `value[0..<position]`, view. |
| 262 | `suffix := try value.Slice(afterStart, value.Len)` | `value[afterStart..<value.Len]`, view; `afterStart` uses `search.Len` in runes. |

Keep `position`, `found`, and `search.Len` in rune units; the builder owns its result. Preserve existing error translation.

### § 7C. `sec/stdlib/str/escape.sec` — 3 calls

| Old line | Current call/result | Required migration |
| ---: | --- | --- |
| 212 | `segment := try value.Slice(_RuneIndexOfBoundary(value, segmentStart), _RuneIndexOfBoundary(value, index))` | `value[_RuneIndexOfBoundary(value, segmentStart)..<_RuneIndexOfBoundary(value, index)]`; do not mistake byte scanner offsets for rune indices. Builder must accept a view. |
| 341 | `suffix := try value.Slice(_RuneIndexOfBoundary(value, segmentStart), value.Len)` | `value[_RuneIndexOfBoundary(value, segmentStart)..<value.Len]`; builder view append. |
| 441 | `contents := try value.Slice(1, value.Len - 1)` in `Unquote` | `value[1..<value.Len - 1]` -> view; **the next call `UnescapeLiteral(contents)` currently expects `string`**. Introduce an internal view-consuming unescape implementation (preferred, zero intermediate copy), or explicitly materialize a string with correctly handled allocation errors. Never implicitly cast the view to string. |

Existing `UnescapeLiteral` public API and error position semantics must be preserved. Its parser uses byte offsets internally; ensure the view's relative byte/rune observables are correct. The explicit `Unquote` bounds depend on the two ASCII quote characters; keep prevalidation and avoid `Len - 1` unsigned underflow.

### § 7D. `sec/stdlib/str/split.sec` — 8 calls

| Old line | Current call/result | Required migration |
| ---: | --- | --- |
| 110 | `remainder := try value.Slice(position, value.Len)` in `SplitN` | `value[position..<value.Len]` produces a view; **cannot directly append it to `string[]`**. Convert explicitly to independently valid `string` or adjust the *declared public API* only by separate decision. |
| 166 | `retained := try value.Slice(position, nextPosition)` in `SplitAfter` | View; must materialize/retain an independently valid `string` for `result: string[]`. |
| 228 | `remainder := try value.Slice(position, value.Len)` in `SplitAfterN` | Same. |
| 243 | `retained := try value.Slice(position, nextPosition)` in `SplitAfterN` | Same. |
| 322 | `field := try value.Slice(start, position)` in `Fields` | View; explicit string materialization before `result.Append(field)`. |
| 344 | `field := try value.Slice(start, position)` in `Fields` final field | Same. |
| 388 | `field := try value.Slice(start, position)` in `FieldsBy` | Same. |
| 408 | `field := try value.Slice(start, position)` in `FieldsBy` final field | Same. |

`Split*`/`Fields*` return `Result[string[], CollectionError]`. They are **materializing APIs** even if constructing the interim view is free. The inserted view-to-string conversion may require allocation or a proven independent lifetime; map `StringError.Allocation` to the existing public collection error explicitly. Do not allow a view borrowed from a parameter to escape inside an owning `string[]` without a valid independent owner. `string.Split(separator)` / `StringSplitIterator.Next(): Option[string]` is **a different API** and is not deleted by this correction.

### § 7E. `sec/stdlib/str/whitespace.sec` — 7 calls

| Old line | Current call/result | Required migration |
| ---: | --- | --- |
| 218 | `line := try value.Slice(lineStart, position)` in `Dedent` pass 1 | `value[lineStart..<position]` -> view; `_UpdateDedentPrefix` needs a view contract. |
| 235 | `line := try value.Slice(lineStart, value.Len)` in `Dedent` pass 1 | `value[lineStart..<value.Len]`; view-aware prefix helper. |
| 263 | `line := try value.Slice(lineStart, position)` in `Dedent` pass 2 | View-aware `_AppendDedentedLine`. |
| 282 | `line := try value.Slice(lineStart, value.Len)` in `Dedent` pass 2 | Same. |
| 635 | `suffix := try line.Slice(indentation.Len, line.Len)` | Use `line[indentation.Len..<line.Len]` where `line` is `StringView`; `Builder.Append` must handle view. |
| 688 | `indentation := try line.Slice(0, length)` in `_UpdateDedentPrefix` | Use `line[0..<length]`, preserving view lifetime and rune units. |
| 711 | `shared := try previous.Slice(0, matched)` in `_UpdateDedentPrefix` | Use `previous[0..<matched]`, relative to the previous view. |

**Critical transitive changes:** `_AppendDedentedLine(builder: ref mut Builder, line: string, indentation: string)` at **line 620** currently requires owning `string` parameters; change to view-accepting internal parameters/overloads and ensure `StringView.StartsWith(StringView)` or equivalent exact prefix matching exists. `_UpdateDedentPrefix(current: Option[string], line: string) Option[string]` at **line 670** must become a lifetime-checked view-based helper, preferably `Option[StringView]` for the current indentation state and `StringView` for the line. Its return must preserve the backing origin of the source, not a local-parameter storage address. `Builder.Append` must accept views. Both helpers must avoid hidden conversion or hidden allocation. The public `Dedent` remains `Result[string, CollectionError]`.

**Separate correctness review identified while auditing:** `_UpdateDedentPrefix` increments `length`/`matched` against `.Len` (runes) but reads `.ByteAt(length)` and `.ByteAt(matched)` (bytes). For its strictly ASCII leading indentation this may coincide until the first non-ASCII scalar; it still requires a dedicated Unicode/index-unit test and should not be silently justified by this migration. The helper's character classification and prefix comparison must use consistent byte/rune coordinates.

### § 7F. Other production source — 2 calls

| File / old line | Current code | Required migration |
| --- | --- | --- |
| `sec/core/number_parse.sec:33` | `part := try source.Slice(startByte, endByte)` | **Existing unit mismatch:** `startByte`/`endByte` are declared UTF-8 **byte** offsets (`_NumberSpan`). Do not replace with `source[startByte..<endByte]`. Either carry rune positions alongside the scanner's byte positions, perform an explicit byte-boundary-to-rune-index conversion before `source[runeStart..<runeEnd]`, or use a separately justified private core byte-range traversal. Preserve `NumberParseError.InvalidNumber` mapping. Then iterate `for character in part`, with `character: rune`. |
| `sec/stdlib/net/http/structured_fields.sec:283` | `key := try self._input.Slice(self._RuneIndexAt(start), self._RuneIndexAt(self._position))` | Conversion to rune offsets is already present: `self._input[self._RuneIndexAt(start)..<self._RuneIndexAt(self._position)]` gives a **view**, but `_ParseKey() -> Result[string, StructuredFieldError]` at **line 262** returns an **independently valid string**. Explicitly materialize before `return Ok(key)`, or redesign the internal parser-key ownership/retention with validated lifetime metadata. Keep invalid syntax and allocation failures distinct (`StructuredFieldError.InvalidSyntax` versus its existing `AllocationFailed` path). Do not return a borrowed view as `string`. |

### § 7G. Source tests and non-call references

| File / line | Required action |
| --- | --- |
| `testdata/sema/string_utf8_positions_valid.sec:7` | Replace `let middle: Result[string, RangeError] := text.Slice(1, 3)` with a valid `StringView` bracket-range fixture. Add separate `try` failure/propagation cases and UTF-8 multibyte tests. |
| `sec/core/string.sec:756` | **Remove** public `fn Slice(start: uint, end: uint) Result[string, RangeError]`, including old documentation and direct helper invocation, after all references are migrated. The private `_ByteOffsetOfRune` logic (around line 651) is reusable for bracket range creation. |
| `sec/core/string.sec` (multiple `__StringSliceUnchecked(...)` sites) | **Not `.Slice()` calls.** They are explicit low-level descriptor creation in `StringSplitIterator`, trimming and internal substring helpers. Leave working while reconciling representation/lifetime/owned result with the new `StringView` contract; audit but do not blindly rewrite. |
| `sec/stdlib/str/builder.sec:172` | Add declared, documented `Append(value: StringView) Result[void, CollectionError]` (or a separately named equivalent fixed by the API spec) that copies **into builder output storage only**, not into an intermediate string. Retain existing `Append(string)` and `Append(rune)` overloads. Preserve transactional rollback and allocation errors. |
| `sec/stdlib/str/stdlib_str.md` (around line 1508) | Update the explanatory reference to `string.Slice` and document the view-aware builder and internal helper contracts. Do not let the open byte-offset edit-unit question MD-090 be silently decided here. |
| `rules/analysis/escape_analysis.md:1121` | `Parameter(0).Slice(2..8)` is a **symbolic provenance path**, not a string `.Slice()` method call. Do not alter by search/replace. |
| `rules/collections/collections.md:2494` and other references to `Slice()` on collections | These concern deprecated array methods or other slice concepts; not part of public `string.Slice` removal. |

**§ 7(3).** A future source audit after implementation must repeat both semantic call lookup and a repository-wide literal search for `.Slice(`, `Slice(start`, `string.Slice`, and `Result[string, RangeError]` to catch added code, documentation, embedded Go fixtures, LSP hovers and any consumers not indexed at the audited commit.

---

## § 8. Exact compiler and integration work

**§ 8(1). Parser/AST/formatter.** Reuse `ast.SliceExpression`, `parseSliceExpression`, canonical range tokenization and `try` expression parsing. Add valid string/view source examples and fixed-point formatter tests. There is no source grammar for `RuneSlice` or `.Slice` compatibility. Target files include `internal/parser/expression.go`, `internal/ast/ast.go`, `internal/formatter/*`, `rules/foundations/grammar.md`.

**§ 8(2). Sema range plans.** `internal/sema/slice_creation_plans.go` presently rejects types it cannot slice (including string); extend *only* string/view range sources with a distinct resolved source kind and result type. Reuse exact endpoint normalization (`internal/sema/slice_normalization.go`, `slice_range_classification.go`, source-origin plans) while marking **rune** units, no `ref T[]` element array, no mutation, and source-backed borrow lifetime. Ensure `try` recognizes the `RangeError` failure edge without the array-only `try ref` spelling.

**§ 8(3). Sema iteration.** `internal/sema/analyzer_for.go` presently treats `string` as `forCategoryString`, binds a decoded `rune` by value, and rejects `for ref` on strings. Add `StringView` as the same string-family category; preserve value-yield semantics and two-binding index-unit rules. Do not import the heterogeneous-pack semantics as a synthetic `ref rune` source. Update `internal/sema/analyzer_for_categories.go`, `iterator_lowering_readiness.go` only if the current split requires it.

**§ 8(4). Borrow/return analysis.** The view has a complete source storage-origin, nested range projection, and liveness dependencies. Update the existing Sema origin and reference analysis (`slice_places.go`, `slice_origin_dependencies.go`, `slice_return_origins.go`, `reference_inference.go`, `reference_place_origins.go` as applicable). Return a view from a `ref string` parameter only with valid returned-origin facts; reject returning a view of dead local string/temporary. Do not make `StringView` an ownership-bearing `string` or an ordinary `ref rune[]`.

**§ 8(5). Semantic IR.** Reuse existing range guard/`RangeError` flow and immutable plans only after Sema has supplied valid `StringView` range/provenance facts. Existing `internal/ir/semantic/slice_*` operations assume contiguous `T` elements; adapt through a distinct string-view plan (or a validated generic bounded-view abstraction), not by claiming UTF-8 has contiguous stored `rune` objects. String iteration decodes scalars through P23 string-family operations and uses the same loop lowering path.

**§ 8(6). MLIR/backend.** Update P23 string/view operations and their verifier to keep distinct owned string, borrowed string view, and `rune` yield value. String-range conversion from rune bounds to UTF-8 byte boundaries is checked and non-allocating; range guard must dominate view publication; ordinary failure emits `panic.bounds` with `slice-range`, checked failure returns `RangeError`. Honor existing schema/profile compatibility and the **one** canonical Semantic IR → Sec MLIR → LLVM track; do not claim P23 completion or invent a second backend. Relevant anchors include `internal/lowering/secmlir/*`, `mlir/include/sec/Dialect/Sec/*` and `mlir/lib/{Dialect,Analysis,Conversion}/*` after the P23 contracts are updated.

**§ 8(7). Core/stdlib.** Implement the source-visible `StringView` operations and borrowed bracket range without calling the old public method. Keep the private `__StringSliceUnchecked` *separate* as an owned-string descriptor helper until its independent `core.string-slice-helper` and MD-052 migration is resolved. Migrate the § 7 calls in dependency order: view type + builder-view append -> `edit`/`replace`/`escape`/`whitespace` -> owning `split`/HTTP -> byte-span `number_parse` -> public core method removal.

**§ 8(8). Diagnostics/LSP.** Hover must say “borrowed UTF-8 string range; indexes count runes; no allocation; ordinary bounds panic; `try` returns `RangeError`”. Reject `ref mut` on view, view-to-string implicit conversions, stale/escaping views and `for ref ch` over decoded UTF-8. For former `text.Slice(a,b)` calls, the compiler-as-mentor diagnostic should suggest `text[a..<b]` or `try text[a..<b]` and explain that the result is borrowed; do not silently rewrite or automatically materialize.

---

## § 9. Required conformance tests

**§ 9(1). Parse/format:** `text[1..2]`, `text[1..<3]`, all open forms, `try text[1..<3]`, `for ch in text[1..<3]`, nested view slicing, source/endpoint evaluation once, formatter idempotence, invalid range grammar recovery. Array `ref values[...]` remains unchanged.

**§ 9(2). UTF-8 units:** use `"Aπ😀b"` (`Len == 4`, `SizeOf == 8`) and verify rune ranges select `"π😀"` for `[1..<3]` and `[1..2]`, `[2..<2]` is empty; a multi-byte scalar must never be split. Test NUL, combining characters, non-normalized sequences, non-BMP, and string views with empty range at `Len`.

**§ 9(3). Failure classification:** start>end, negative/signed-invalid endpoints, exclusive end>Len, inclusive end==Len, static errors vs dynamic panic, `try` propagation and locally handled `RangeError` cases, exact precedence, no integer-overflow during inclusive normalization. `try` must not catch unrelated panic/invalid storage generation.

**§ 9(4). Ownership:** no materialized string from bracket slicing; escape-from-local fails; caller-sourced view return succeeds when lifetime proves valid; nested views preserve origin/range; no unsafe conversion to `ref rune[]`, `RawPtr[rune]` or `ref mut` text; user code cannot turn `StringView` into owned `string` implicitly.

**§ 9(5). Performance/effects:** prove view creation and iteration have **no allocation effect**, no `string` content copy, no per-rune allocation, and decode into `rune` scalar values. Verify on 32-bit and 64-bit target profiles where supported. Include a large UTF-8 text and midrange iteration; do **not** require O(1) rune-index-to-byte-offset translation.

**§ 9(6). Integration:** existing plain `for ch in string` unchanged, `for ch in StringView` same rune values, `for ref ch` over strings/views rejected, ordinary `for ref x` over actual `rune[]` permitted, MD-117/119 pack-loop reference-holder behavior unchanged. Confirm `StringSplitIterator` still returns its `Option[string]` elements independently.

**§ 9(7). Migrations:** compile and test every § 7 source site with *declared* view consumers. Include `Builder.Append(StringView)` transactional rollback; `StringView.ToString()` materialization and failure mapping; string-array split lifetime independence; Unicode-aware dedent/escape; HTTP returned key ownership; `number_parse` byte-to-rune conversion. Confirm no stale public `.Slice()` calls or old LSP API suggestions remain.

**§ 9(8). Verification gates:** parser, Sema, borrow/lifetime, Semantic IR, MLIR verifier/lowering, native execution when available, stdlib integrations, LSP hover/completion, formatter, diagnostics, regression corpus, and `go test ./...` / `go vet ./...` according to existing project testing policy. Distinguish missing P23 runtime prerequisites from a broken MD-126 source contract; do not mark implementation complete merely because syntax type-checks.

---

## § 10. Governance synchronization and staged implementation

**§ 10(1).** Reuse **existing** governance integration IDs; do not create duplicates for matching responsibilities. At minimum, reconcile:

| Governance file | Existing ID / owner | MD-126 work |
| --- | --- | --- |
| `governance/core.yaml` | `core.string-slice-helper`, `compiler.compiler-known-surfaces` | Separate private owned-string helper from public borrowed range, core registry and LSP identities. |
| `governance/collections.yaml` | `frontend.collections` | Type-directed string range and borrowed view creation. |
| `governance/control_flow.yaml` | `frontend.for-loops-v2` | StringView iteration using rune-by-value yields. |
| `governance/lifetime.yaml` | `frontend.borrowing`, `frontend.lifetime-analysis` | Borrowed view origin, nested projections and escape rejection. |
| `governance/types.yaml` | `frontend.types-core` | Built-in StringView identity, UTF-8/rune/byte units and no implicit cast. |
| `governance/parser.yaml` | `frontend.grammar-conformance` | Preserve bracket syntax; add strings as new semantic category, not grammar fork. |
| `governance/errors.yaml` | `frontend.errorhandling-v2` | `try` string range -> `RangeError`, panic policy. |
| `governance/analysis.yaml` | `analysis.reference-model`, `interprocedural.references`, `analysis.iterator` | View origin summaries and compiler-known iteration facts. |
| `governance/lowering.yaml` | `lowering.sec-mlir-package16`, `frontend.semantic-ir-v2`, relevant P23 owner when registered | Reuse range guards and integrate P23 view/string semantics without false completion. |
| `governance/stdlib.yaml` | Appropriate existing str/io/net-http owner after lookup | Record source migration and builder-view dependent work. Do not invent an unregistered `stdlib.str` ID. |

**§ 10(2).** `governance/index.yaml` must remain the single fragment index. Update only the existing owner's integration item (including remaining/implemented/test evidence), not an invented parallel MD-126 implementation fragment unless the governance schema explicitly requires a new, uniquely owned capability.

**§ 10(3).** Recommended rollout in individually verifiable steps:

1. Apply this MD-126 correction to canonical books without renumbering, replacing the open byte-oriented MD-126 question; retain missing implementation status.
2. Introduce the exact `StringView` semantic type and borrowed range plans, including failed-bounds typing and origin metadata; test frontend grammar/Sema and invalid cases.
3. Extend existing string-family `for` resolution to views, preserving rune-value iteration; test against 32-bit target capability where possible.
4. Define and test explicit `StringView` text readers and materialization (`ToString`) as well as builder view append; do not assume hidden implicit conversions.
5. Migrate the § 7 production sites **one file/function at a time**, with tests before continuing. Resolve the `number_parse` coordinate mismatch rather than masking it.
6. Update Semantic IR, P23, MLIR verification/lowering, LSP, formatter and governance to reflect the actual achieved scope.
7. Remove the public `.Slice()` API only when its production and test users have been migrated; retain internal `__StringSliceUnchecked` under its distinct migration status.
8. Run cross-repository migration search and test conformance. Close MD-126 design status separately from compiler/lowering/runtime implementation status.

---

## § 11. Explicit exclusions / no new decisions implied

**§ 11(1).** This correction does not settle MD-090's `stdlib/str` edit position units, create a public byte-based range, change UTF-8 decoding validity, add grapheme clusters, add normalization, add mutable string element borrows, or introduce a user-visible `RuneIterator` factory.

**§ 11(2).** This correction does not implicitly alter `Split(separator)` or `StringSplitIterator.Next() -> Option[string]`, which are *substring splitting* APIs and not the compiler-known Unicode scalar iterator used in `for ch in text`.

**§ 11(3).** Do not convert every older `string`-producing helper into `StringView` or every array result to views. APIs that own independently valid results keep those contracts unless separately changed. Explicit materialization is allowed and potentially allocating; **creation and traversal of the borrowed view are not**.

**§ 11(4).** The canonical `StringView` source spelling and the exact § 4(8a) public member list must be fully entered into the core/CKM registry before declaring this correction integrated. They must not be silently fabricated by Sema or LSP. If a technical inconsistency is found, isolate it as a new genuine design conflict for owner approval; do not silently relax ordinary `ref T` or add hidden allocation to make tests pass.

---

## § 12. Repository audit reproducibility

**§ 12(1).** Audit input: GitHub `main` HEAD `0cdacb04fb3757a26569d51c21a5ae445f0facaf` verified 2026-11-05. Inspected the string implementation, `str` consumers, HTTP parser, core number parser, semantic fixture, canonical rulebooks, P16/P23 plans, and existing governance owners.

**§ 12(2).** The 32 production direct `.Slice(` calls at this snapshot are exactly: `edit.sec` (8), `replace.sec` (4), `escape.sec` (3), `split.sec` (8), `whitespace.sec` (7), `number_parse.sec` (1), `structured_fields.sec` (1). One fixture in `testdata/sema/string_utf8_positions_valid.sec` adds one additional `.Slice(` call. Additions after the audited commit must be located anew before merge.

**§ 12(3).** Useful repository audit commands from project root (run by implementer on the current checkout):

```sh
# Literal member-call inventory, excluding Git metadata.
rg -n --hidden --glob '!.git/**' '\.Slice\s*\(' sec testdata rules governance

# Public method declarations, contracts and documentation references.
rg -n --hidden --glob '!.git/**' 'string\.Slice|fn Slice\(|Slice\(start|Result\[string, RangeError\]' sec testdata rules governance

# Private byte descriptor helper is a separate compatibility migration.
rg -n '__StringSliceUnchecked|_SliceUtf8Bytes' sec internal cmd rules governance

# Update all dependent code before deleting the old core member.
rg -n 'StringSplitIterator|_UpdateDedentPrefix|_AppendDedentedLine|Append\(value: string\)' sec rules governance
```

**§ 12(4).** The inventory intentionally distinguishes **direct string `.Slice()` calls** from Go standard-library `slices`, generic slice types, array reference syntax, and abstract analysis notation. Do not infer an identical change from a matching spelling alone.

