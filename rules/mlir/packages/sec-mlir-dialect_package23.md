# Sec MLIR Program — Package 23: String Values, UTF-8 and Core Materialization

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec-mlir-dialect_package23.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P23`

---

**Target dialect schema:** v19 (planned; v18 predecessor required)  
**Target lowering:** v19 (planned)  
**GitHub review date:** 2026-10-16  
**Exact GitHub SHA:** Not established by public web inspection; implementer must run `git rev-parse HEAD`.  
**Decisions:** MD-050 (existing), MD-071 (P23-D1), MD-072 (P23-D2), MD-073 (P23-D3) (recorded in `missing-decisions.yaml`)  
**Decision status (2026-10-17):** MD-050, MD-071 (P23-D1) and MD-072 (P23-D2) are decided by `rules/corrections/applied/md050-md071-md072-string-utf8-positions-materialization-correction-20261017.md`: every `string` is valid UTF-8 (`types.md`, "Valid UTF-8 invariant"); `IndexOf`/`LastIndexOf`/`Slice`/`GetRune`/`RuneAt` use rune positions and `GetByte`/`ByteAt`/`CharAt` byte indexes (`core-library.md` § 5.7); `FromByteArray`/`FromRuneArray` borrow and return `Result[string, StringError]`, `ToByteArray`/`ToRuneArray` return `Result[byte[]/rune[], CollectionError]` (§§ 5.2–5.3). Where this package says a behavior is gated by those three decisions, the decided rule now applies; `StringValidateUTF8Op` validates at decoding boundaries only. MD-073 (P23-D3) remains open; MD-074 was decided 2026-10-17 (`len(...)` is `uint`, `rules/corrections/applied/md051-md074-md088-length-numeric-string-correction-20261017.md`).  

---

## § 1. Goal and completeness boundary

Implement high-level semantic string values, valid-UTF-8 scalar/byte length primitives, bytewise comparison and search, core-private checked substring substrate, and MD-004 **single-materialization** concatenation/interpolation. Extend the P18/P19 array/allocation infrastructure for materializing byte/rune/char results **only after** the corresponding public signature and UTF-8 policy are approved. P23 is a specification, not evidence that P15–P22 have been implemented.

## § 2. Actual predecessor status

User-confirmed: P1–P14 implemented; P15 implementation in progress; P16–P17 corrected plans, not confirmed implemented. P18–P22 packages exist as specifications, not a claim of executable support. P23 target is **Sec MLIR schema v19** after proposed v18 and **lowering spec v19**; local planning of a version does not bump repository's currently implemented dialect. Record actual HEAD and actual implemented schema before modifications. Never overwrite user's corrected P18 or updated P16/P17 plans.

## § 3. Canonical authority order

Read, without stale August path guesses: `rules/types/types.md`, `rules/library/core-library.md`, `rules/compiler/compiler_known_members.md`, `rules/types/contracts.md`, relevant applied corrections MD-004 and MD-058, `rules/memory/copy_move.md`, `rules/memory/layout.md`, `rules/memory/allocation.md`, `rules/compiler/semantic_ir.md`, `rules/mlir/sec_mlir_dialect.md`, `rules/mlir/sec_mlir_lowering.md`; then the loader-proven `sec/core/string.sec`, the actual frontend/compiler sources and current governance. This order is normative > current implementation > P23-specific additive detail where no conflict exists.

## § 4. Repository audit evidence

Current public `main` was read on 2026-10-16; exact HEAD SHA not independently verified. `sec/core/string.sec` visibly has mixed old/new names or placeholder bodies depending on snapshot: inspect `RuneLen`, `ByteLen`, `FromByteArray`, `FromRuneArray`, `IndexOf`, `Slice`, `Split`, `ToRuneArray`. The legacy generator `internal/codegen/mlir/string_length.go` already contains a rune-count helper; `internal/codegen/readiness/string_contracts.go` guards unsupported string-contract paths. Codex must inspect and reuse these, report line-accurate SHA-specific evidence and fix only confirmed mismatches. No backend support claim follows from a source declaration.

## § 5. Mandatory MD-058 length units

`string.Len: uint` counts **Unicode scalar values**, not bytes/graphemes/display columns; `string.SizeOf: uint` on a **value** counts UTF-8 payload bytes, excluding terminator, descriptor and capacity. `string.SizeOf` on a **type** is physical descriptor layout size, a separate query. Compatibility lowercase `.len` also counts scalars. Compiler-known `len(text): int` counts scalars and evaluates receiver once. `"éΩ"` => Len 2, SizeOf 4; `"Hej π"` => Len 5, SizeOf 6. Remove public `ByteLen`, `RuneLen`, `CharLen` synthetic aliases and prevent privileged core shadowing of compiler-known properties.

## § 6. String type identity and encoding

`string` is an immutable compiler-known scalar-like semantic value **distinct** from `byte[]`, `char[]`, `rune[]`, C strings and safe slices. Its actual bytes are represented as UTF-8 when valid; **MD-050** still governs whether arbitrary invalid UTF-8 can exist as a `string`. Do not turn the valid-UTF-8 path into an unsupported feature: implement it and block only branches requiring the undecided invalid-UTF-8 rule. No implicit normalization, locale collation, case folding or zero-terminator scanning.

## § 7. String copy/ownership

`rules/memory/copy_move.md` §4.5 explicitly makes `string` **implicitly copyable**, immutable, allocation-free, infallible and free of mutable aliasing. Copy must preserve lifetime. Acceptable physical mechanisms are representation-selected; do **not** invent mandatory reference counting, hidden allocation or universally owned heap buffers. A substring/string view still has valid backing/lifetime provenance; a descriptor copied to longer-lived storage cannot dangle. Use P17 `CopyValueOp`/ownership facts as soon as implemented.

## § 8. String storage semantic data

Define `StringValueFacts` keyed by Semantic IR ValueID: `StringValueID`, encoding status (KnownValid / UnknownPendingMD050), backing kind (StaticLiteral / ImmutableMaterialized / ImmutableView / OtherResolved), data storage identity, offset/byte-extent, lifetime dependency, allocation context, epoch/invalidation dependency, source range/provenance, known or runtime scalar count. These are **semantic facts**, not a physical `(ptr,len)` descriptor definition. Keep numeric byte offset distinct from scalar index types in analysis even when both are `uint` in source.

## § 9. Static literals and empty string

Static source literals may use immutable static backing and are compile-time materialized: no runtime allocation. Empty literal has zero bytes and zero scalars; its safe string value is valid independent of any physical pointer/sentinel. Embedded NUL is ordinary payload and counts as one scalar/byte. Preserve literal escapes and source-to-byte decoding established by lexer; Semantic IR sees resolved exact UTF-8 bytes, not a reinterpreted lexer token.

## § 10. String length operations

Add `StringRuneLengthOp -> uint`, `StringByteLengthOp -> uint`, and `StringSignedRuneLengthOp -> int`. Reuse compiler-known member IDs for `.Len`, `.SizeOf`, `.len`, and `len(text)` rather than dispatching by spelling in backend. For static valid text, both counts may fold. For runtime valid text, rune counting scans UTF-8 scalar boundaries without allocation, reads in-bounds only, and does not produce a new string. The cross-family `uint`→`int` representability issue from P16/P18 remains unresolved; never silently truncate; emit signed-length operation only with proven representability or report explicit implementation gap.

## § 11. UTF-8 scan contract

For known-valid UTF-8: every scalar starts with one non-continuation byte; a robust general validator decodes 1–4 bytes and rejects overlong encodings, surrogates and >U+10FFFF. A rune-counter optimized via non-continuation count is valid only if the input carries proof of valid encoding, and must respect exact byte bounds. Use target-size independent integer/byte arithmetic; avoid C NUL scans, overreads, Unicode normalization and grapheme segmentation. Invalid-input behavior stays gated by MD-050.

## § 12. String equality and compare

String equality compares exact logical content (on the known-valid path equivalent to encoded UTF-8 bytes), not descriptor addresses. `Compare(other)` is ordinal lexicographic Unicode scalar order, consistent with lexicographic UTF-8 bytes for valid UTF-8. Return negative/zero/positive `int` without promising absolute difference. Both operations are read-only, allocate nothing and evaluate operands once. `==`/`!=` must not be pointer equality, `memcmp` beyond explicit bounds, locale collation or normalization.

## § 13. StartsWith/EndsWith/Contains

Keep source core behavior where it is actually canonical: exact case-sensitive byte-content tests without allocation. The current source comments specify that **empty prefix/suffix do not match**; do not silently substitute another language's convention. `Contains` dispatches via `IndexOf` with the core-defined search behavior. Ensure empty needle and offset semantics are tested and do not infer public index units from `.Len`. Any conflicting rule belongs to MD-071 (P23-D1).

## § 14. Search and substring offset decision gate

`IndexOf`, `LastIndexOf`, `IndexOf(value,start)`, checked `Slice(start,end)`, `GetRune`, `ByteAt`, `CharAt`, `RuneAt` currently mix byte and scalar terminology. The private `__StringSliceUnchecked` is explicitly byte-offset based and requires `start <= end <= value.SizeOf`, while public `.Len` is scalar-based. **MD-071 (P23-D1)** requests the missing *public* offset-unit and UTF-8-boundary contract. Until accepted, implement private byte-range IR and proven-safe byte operations, but DO NOT silently change public index semantics to match the easiest implementation.

## § 15. Private slicing primitive

Preserve the already specified **source-file-private** `__StringSliceUnchecked(value,start,end)` helper. It is not a public compiler-known member, not an importer-visible function, and must not bypass checked public API validation. High-level `StringSubrangeUncheckedOp` has an explicit verified proof precondition: 0≤start≤end≤value.SizeOf, caller-established content-validity boundary as applicable, and valid backing lifetime. No allocation is implied, no C NUL scan, and no safe slice type is fabricated.

## § 16. Public checked substring boundary

`string` does **not** use array slice syntax. Checked public `Slice` or equivalent core member returns the exact `Result[string,RangeError]` resolved by the real core declaration. Preserve evaluation order; invalid bound ordering maps `RangeError.StartAfterEnd`; extent overflow maps `RangeError.OutOfBounds`. Additional UTF-8 boundary behavior must not be invented: blocked behind MD-071 (P23-D1)/MD-050 if required. Returning a string borrowing existing backing requires that the backing remains alive; otherwise Sema/proven plan must select a permissible materialization with allocation/error or reject.

## § 17. Split and traversal

Preserve `StringSplitIterator` as an ordinary core `Iterator[string]` with resolved static `Next`/`IsFinished` semantics. Split is lazy and must not eagerly materialize an array. `SplitToArray` is separate and potentially allocating. Existing `SplitOnce`/`SplitLines` implementations in `sec/core/string.sec` may contain placeholders or additional APIs not yet normative; do not silently canonize them. Runtime iterator CFG lowering depends on the corresponding for-loop package; P23 can provide string operators/iterator helper identities now, and gate full `for` lowering until implemented.

## § 18. Trim family and whitespace

`Trim`, `TrimStart`, `TrimEnd` are resolved core operations; trim boundaries must be Unicode scalar-aligned, and byte offsets may be used internally after validation. Do not invent new whitespace property tables or normalization; reuse canonical rune classification where implemented. Core-private view/subrange reuse must preserve backing lifetime. No implicit allocation merely because a new string descriptor is returned.

## § 19. MD-004: one string materialization

Runtime concatenation/interpolation is **one** fallible materialization (MD-004), not left-associated allocating intermediate strings. Evaluate operands/interpolated expressions exactly once, left-to-right according to source semantics, and retain ordered pieces with their source values and conversion effects. Measure total bytes with checked arithmetic; select active P19 allocation context at Sema, ensure result storage outlives uses, allocate final storage once, fill each piece, publish exactly one immutable string on success. Failed allocation is `AllocationError`, propagated through ordinary P10/P11 `try`/Result; no null, panic or hidden provider selection.

## § 20. Static concat and interpolation

Fully compile-time-known concatenation/interpolation folds to immutable static bytes with no runtime allocation or error; preserve evaluation/side effects if source plan says some operand effects remain. Do not claim constant folding if any runtime operand or side effect exists. Maintain a separate resolved static plan from the dynamic allocation plan.

## § 21. StringConcatPlan consumption

Read and reuse frontend `StringConcatPlan` (or current renamed equivalent); the builder queries it read-only and may not re-evaluate string coercion, interpolation parsing or allocation context. Required pieces: source-order segment IDs, segment kind StaticBytes/StringValue/ResolvedStringConversion, one resulting semantic `string`, exact failure type `AllocationError`, selected AllocationContext, ownership/capture/lifetime facts, constant-fold status. Missing Sema fact is a frontend gap, not license for lowering to scan AST for `+`.

## § 22. Checked materialization failure CFG

The high-level operation returns `(string candidate, i1 failed, AllocationError error)` or equivalent typed checked result with success-only candidate consumption. On failure, no result string is published, no partial storage escapes, and cleanup releases temporary resources in correct P17 order. On success, publish immutable backing and return the candidate with its lifetime/allocation domain. `try` handles only this allocated materialization failure; earlier operand effects retain their own error behavior.

## § 23. FromByteArray/FromRuneArray source conflict

`rules/library/core-library.md` lists owning-array/infallible illustrative signatures but explicitly leaves final signatures pending; the current `sec/core/string.sec` instead declares ref parameters and Result errors, and one rune body is a stub. **MD-072 (P23-D2)** owns this independent source signature/ownership/allocation-error question; MD-050 owns the validity rule. Until approval do not invent `string.FromByteArray` source overloads or ABI; implement a **lower-level, non-public** byte/rune materializer with explicit bounds/encoding assumption and allocation context, so selected approved signature can reuse it.

## § 24. ToByteArray/ToRuneArray/ToCharArray

These return **independent owning dynamic arrays** on the required semantic surface, not `ref byte[]`/`ref rune[]` views. `ToByteArray` yields exact UTF-8 payload bytes, `ToRuneArray` one rune per scalar, `ToCharArray` follows char domain. No terminator. The source does not mutate/consume; returned arrays can be mutated independently. Allocation/error signatures remain to be synchronized with MD-072 (P23-D2); invalid-UTF8 branch tied to MD-050; char >255 branch tied to MD-073 (P23-D3). Reuse P18 owner lifecycle and P19 allocation facts where implemented.

## § 25. Text-sequence ToString

For exactly `byte`, `char`, `rune` elements in fixed arrays, dynamic owning arrays and their slice views, a resolved compiler-known `ToString()` materializes **one string** using the represented elements only. Bytes are interpreted as encoded bytes (MD-050 boundary), runes encoded as Unicode scalars, chars follow their canonical 0..255 semantics (P23-D3). No zero terminator scan; neither element sequence nor slice is consumed by string conversion. Do not add a generic array formatting special case.

## § 26. Identity string.ToString

`string.ToString()` returns the already canonical `Result[string,StringError]` with `Ok(self)`, infallible/no allocation/no byte copy, source remains reusable. Lower to existing P11/P17 Result+string-copy semantics. No implicit deep copy. Preserve Result source type even if known always successful.

## § 27. Char and rune scope

`char` represents numeric values 0..255; `rune` is Unicode scalar U+0000..U+10FFFF excluding surrogate range. `rune` to UTF-8 encodes 1–4 bytes, including U+0000 and non-BMP. `char` cannot directly represent every possible rune; byte reinterpretation is NOT equivalent to char-to-string conversion. `MD-073 (P23-D3)` asks for lossless/failure/selection behavior for `ToCharArray` on Unicode scalars >255 and char sequence encoding. Do not silently replace out-of-range values or assume Latin-1 bytes are UTF-8 without a rule.

## § 28. P22 contract interoperability

String named-type contracts use **rune count** for `minLen/maxLen/exactLen/notEmpty` and **byte count** for `minByteLen/maxByteLen/exactByteLen` (MD-058 + 2026-10-09 correction). Reuse P23 RuneLength/ByteLength operations; no second backend string-count helper or second contract error type. Conversion validation still creates public `ConversionError.Contract(ContractError...)` with source-declaration ordinal. Regex remains MD-010 gated.

## § 29. Allocation/lifetime and noalloc profiles

Any materializing op must carry P19 context/provider selected before Semantic IR; read/copy/compare/count/known-view operations may not allocate. In no-allocation profiles, reject runtime materialization absent proof of complete static/allocation elimination, never choose silent heap fallback. Materializing strings stored in Arena cannot escape beyond Arena lifetime or be silently promoted. Conditional cleanup must preserve original allocation error and not destroy an incompletely published string.

## § 30. Effect analysis and ownership

Classify StringConst/Len/SizeOf/Compare/Prefix/Search/Identity as no allocation; string copy as `CopyTrivial`/infallible allocation-free per canonical rule. Concat/interpolation as `MayAllocate` + `RequiresAllocationContext` only when the selected plan actually needs ambient allocation; materializing conversions may allocate. `StringSubrangeUnchecked` is read-only and lifetime-dependent; borrow/lifetime metadata follows P15/P17. String concat pieces cannot be consumed early or evaluated twice.

## § 31. FFI and ptr boundaries

`string.ptr`/`Ptr` is an **unsafe** byte-storage pointer when valid/addressable. It does not pin, own, extend lifetime, guarantee NUL termination, or supply foreign ABI shape. No `strlen`, no implicit C CString. FFI explicit encoding/retention/terminator contract belongs to later package. No physical string descriptor ABI selected in P23.

## § 32. Semantic IR type/facts

Retain canonical `StringType`/existing type ID and source `string` identity. Add immutable `StringValueFacts`, `StringLiteralPlan`, `ResolvedStringOperationPlan`, `ResolvedStringMaterializationPlan`, and when needed `ResolvedStringRangePlan` keyed to resolved AST IDs; implement read-only plan queries. String storage/access is separate from P16 `ref byte[]` and P18 owning `byte[]`. Value type no hardcoded pointer count. Explicit static backing vs borrowed/view vs materialized origin.

## § 33. Semantic IR operations

P23 adds high-level ops or exact equivalent stable intrinsic IDs: `StringLiteralOp`, `StringRuneLengthOp`, `StringByteLengthOp`, `StringSignedRuneLengthOp`, `StringCompareOp`, `StringEqualsOp`, `StringStartsWithOp`, `StringEndsWithOp`, `StringIndexOfOp`, `StringSubrangeUncheckedOp`, `StringValidateUTF8Op` (MD-050 rule-dependent), `StringConcatMaterializeOp`, `StringToArrayOp(kind=byte|char|rune)`, `SequenceToStringOp(kind=byte|char|rune)`, `StringIdentityOp`, and `StringRuneIteratorStepOp` as appropriate. Existing `GetLength`/`SizeOfValue` intrinsic identities may be used instead of duplicating ops: the *semantic distinction* is required, not exact Go names.

## § 34. Sec MLIR schema v19

Target schema v19 adds `!sec.string` **only if not already represented by the predecessor dialect type**; never duplicate an existing string type under a second spelling. Ops recommended: `sec.string.literal`, `.rune_len`, `.byte_len`, `.signed_rune_len`, `.compare`, `.equals`, `.starts_with`, `.ends_with`, `.find`, `.subrange_unchecked`, `.concat_checked`, `.to_array_checked`, `.from_sequence_checked`, `.identity`, `.rune_next` and `sec.string.utf8_validate` only when its semantics are resolved. String materialization ops include allocation context, domain, lifetime/ownership metadata and checked results. Schema v18 remains regression input.

## § 35. High-level lowering boundaries

Lower pure rune count over **known-valid** UTF-8 using byte loads, non-continuation scan or full decode; lower compare/search to bounded byte loops or trusted compiler/core helpers with explicit lengths; any helper remains non-semantic and must be recorded in call graph/effect analysis. Lower checked runtime concat to one allocation/copy protocol, but keep high-level allocation/string descriptors while P18/P19 physical representation is unavailable. Do not select `llvm.ptr`, `malloc`, descriptor `(ptr,len)` or target libc as the canonical meaning.

## § 36. Verification and versioning

Register `--sec-verify-strings` and `--sec-verify-string-materialization`. Verify explicit byte/rune unit tags, valid backing/provenance, no use of invalid UTF-8 without proof, public decision gates, static literal bytes, failure branch polarity, candidate dominance, single final allocation per MD-004, no unapproved source API, exact result error type, no hidden allocator and string copy lifetime. Keep target dialect version 19 **planned** until predecessor implemented. Use ordinary MLIR verifier before and after new passes.

## § 37. Passes and diagnostics

Register proposed `--sec-lower-string-core` (pure comparisons/count) and `--sec-lower-string-materialization` (checked one-materialization, only when P18/P19 prerequisites available). Sema/frontend diagnostics should explain obsolete `ByteLen`/`RuneLen`, incorrect `.Len` vs `.SizeOf`, invalid byte-range boundary once decided, invalid UTF-8 once MD-050 resolves, missing allocation context, escaping backing, and forbidden array-to-string cast. Unsupported features are typed `UnsupportedFeatureError` in new pipeline with exact source location, not backend wrong-code.

## § 38. Implementation phases

**Phase A (available before P15–P19 completion):** baseline audit + synthetic registry normalization; current core declarations/comments and existing Sema plan/read-only queries; pure static UTF-8 literal/length/compare tests; v19 type/op definitions and local verifiers on artificial valid IR. **Phase B (P15/P17 present):** backing origin/lifetime/safe private subrange and string copies with cleanup. **Phase C (P18/P19 present):** owning conversion arrays, runtime MD-004 concat, allocation/check/cleanup. **Phase D (decisions MD-050, MD-071 (P23-D1)–D3):** public disputed APIs and invalid-text handling. No phase may be marked complete merely because docs exist.

## § 39. Required source positive tests

UTF-8 text: empty, ASCII, `"éΩ"`, `"Hej π"`, emoji, combining mark sequence, embedded NUL. Assert `.Len`, `.len`, value `.SizeOf`, `len(text)` in representable cases; associated `string.SizeOf` remains descriptor size. Static literal fold, copy preserves source, no hidden allocation; equality and Compare lexical scalar ordering; prefix/suffix nonempty/empty policy from actual source; Contains/read-only search, long and embedded-NUL; `string.ToString()` yields `Ok` identity with no allocation.

## § 40. Required failure/negative tests

Reject fabricated UTF-8 proof; no post-bound read; no string-array index syntax; no `string(value)` stand-in for FromByteArray; no public ByteLen/RuneLen/CharLen; illegal byte slice index units until MD-071 (P23-D1); impossible `len` narrowing; wrong borrowed lifetime and Arena escape; mutable alias to immutable string; string concat without `try` or valid ambient context; attempted implicit heap/allocator; concat candidate used on failure; wrong `AllocationError` type; incorrect core member ID; stale/raw ptr lifetime misuse.

## § 41. Unicode and conversion test matrix

UTF-8: 1-,2-,3-,4-byte scalars, invalid continuation, truncated sequence, overlong, surrogate, U+10FFFF, > max, NUL, arbitrary byte 0xFF (gated MD-050). Check `ToByteArray` ownership independence, to/from rune array parity for known-valid strings, `char` 0..255 and >255 cases gated MD-073 (P23-D3), zero-element arrays, static/dynamic owner and slices, slice bounded views, no terminator search, allocation failure preserving original source, no hidden provider. Tests for unresolved public signature must be marked blocked, not green.

## § 42. MD-004 tests

Fold compile-time-known concat and interpolation to static bytes, no runtime fail; runtime 3+ pieces evaluate once in order, including function calls; no observable intermediate strings; one successful final backing allocation; total-byte count overflow; allocation failure with `AllocationError`; result lifetime sufficient/escape check; cleanup on all failure edges; ordinary `try` local/naked propagation to exact `Result[string,AllocationError]`; noalloc target rejects runtime materialization unless eliminated; 32/64 target plan differences.

## § 43. P22 and legacy differential tests

Compare runtime string lengths and byte contracts with P22's Sema/CTE evaluator and existing legacy paths for supported valid UTF-8. Detect the existing helper's `llvm`-specific assumptions and do not copy them into canonical semantics. Verify `MD-058` source units independent of LLVM pointer length. Compare new path / legacy for results and diagnostics, distinguish explicitly unsupported from semantic mismatch. Record `go test ./...` / `check-sec-mlir` results only when executed.

## § 44. Governance and documentation

Reuse existing implementation/governance IDs; query current `governance/` before updating. Do not introduce duplicate `string` API declaration inventories. Every compiler-known public member must have its **exact** Sec declaration and structured LSP-visible documentation; no placeholders or undocumented overloads. Avoid renumbering existing rulebook clauses: append additive numbered sections with provenance. Decisions are recorded in `missing-decisions.yaml` (P23-D1–D3 as MD-071–MD-073, 2026-10-16) rather than as separate decision files. Do not close existing MD-050 or MD-010.

## § 45. Acceptance gates

Gate A: repository SHA/predecessor matrix, exact current core declarations, read-only Sema plans, immutable static string facts, correct UTF-8 `.Len`/`.SizeOf`, no public aliases, string allocation-free copy, string literal/compare equality, v19 ops parse/print/verify, pure tests. Gate B: P15/P17 backing/subrange lifetime and ownership verifier; MD-004 one-materialization, P18/P19 alloc context + `AllocationError`, array independence and cleanup, noalloc/32/64 tests. Gate C: MD-071/MD-072/MD-073/MD-050 approved and synchronized before dependent source overloads/Unicode error cases. Gate D: fresh full build and end-to-end IR -> MLIR tests; governance synchronized. No gate is auto-passed.

## § 46. Required implementation report

Codex report SHA/date, implemented schema vs planned target, P15–P22 actual status, MD-004/MD-058 and MD-050/MD-071–MD-073 (P23-D1–D3) state, source paths read, existing core API inconsistencies and corrections, resolved Sema plans and intrinsic IDs, complete operation/tablegen list, length-UTF8/byte algorithm, string lifetime/ownership, static fold, search/substring gates, concat one materialization and allocation-context flow, returned byte/rune array independence, error mapping, verifiers/passes, 32/64/noalloc/Unicode tests, legacy differential, exact commands/results, blockers, deviations, governance diff, P24 physical layout prerequisites.

## § 47. Non-goals and P24 handoff

Do not implement stdlib/str text algorithms, Unicode normalization or grapheme segmentation, locale collation, full formatting, general `ToUpper/ToLower`, SplitLines placeholder, stable C string ABI, implicit copy allocation, universal GC/refcount, raw pointer exposure, generic container iteration, or physical descriptor/LLVM ABI. P24 **Physical Value Representation and Layout Lowering** can consume completed semantic strings, aggregates, places, slices, ownership and Arena dependencies; verify which actually implemented before starting.

## Source references reviewed

- `rules/types/types.md`
- `rules/types/contracts.md`
- `rules/library/core-library.md`
- `rules/compiler/compiler_known_members.md`
- `rules/compiler/semantic_ir.md`
- `rules/compiler/compiler_pipeline.md`
- `rules/memory/layout.md`
- `rules/memory/copy_move.md`
- `rules/memory/allocation.md`
- `rules/memory/reference_model.md`
- `rules/errors/errorhandling.md`
- `rules/mlir/sec_mlir_dialect.md`
- `rules/mlir/sec_mlir_lowering.md`
- `sec/core/string.sec`
- `sec/core/error.sec`
- `rules/corrections/applied/string-length-contracts-correction-20261009.md`
- `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md`
- `rules/corrections/applied/md058-collection-capacity-reservation-correction-20261012.md`
- `governance/types.yaml`
- `language-rulebook-status.md`
