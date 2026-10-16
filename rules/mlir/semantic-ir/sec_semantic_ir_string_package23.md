# Semantic IR Amendment — Strings and UTF-8

- **Status:** Normative additive amendment (subject to existing MD decisions)
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec_semantic_ir_string_package23.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P23`

---

## § 1. String category

`StringType` denotes immutable UTF-8-oriented string semantic values distinct from byte/rune arrays/slices and external C strings. No fixed ABI layout in Semantic IR.

## § 2. Value facts

`StringValueFacts` stores backing identity, encoding validity fact, origin (static/materialized/view), byte-range, lifetime, arena/epoch dependency, optional scalar count and ownership/copy classification. Source `string` identity remains the same for all origins.

## § 3. String constants

Static `StringLiteralOp` stores exact decoded immutable UTF-8 payload and source location, including NUL. No dynamic allocation for compile-time string literals.

## § 4. Copy

Use P17 copy semantics; string copy is infallible allocation-free. Preserve backing lifetime and cannot expose mutable alias to backing.

## § 5. Length units

Distinct `StringRuneLength` (uint), `StringByteLength` (uint), `StringSignedRuneLength` (int) operations, or exact existing `GetLength`/`SizeOfValue` intrinsic IDs. No implicit unit substitution.

## § 6. Encoding scan

UTF-8 rune count requires proven-valid input or final MD-050-decided error route. No invalid-byte count as ordinary scalar under undecided semantics.

## § 7. Comparison

Equality and `Compare` are ordinal content operations, not descriptor/pointer comparison. Search/prefix operations stay read-only; offset units for public location results gated MD-071 (P23-D1).

## § 8. Private substring

`StringSubrangeUnchecked` is core-only private byte-range projection. Proof includes extent and lifetime. Does not fabricate array slice type or expose safe ref mut.

## § 9. Resolved member plan

Builder uses stable `CompilerKnownMemberID` and immutable resolved type/member/source data, never string name ad hoc and never re-runs Sema.

## § 10. MD-004 dynamic concatenation

`StringConcatMaterialize` is one semantic operation with ordered pieces, Sema-selected allocation context, checked output byte count, single final publication, `AllocationError` and ownership/cleanup dependencies.

## § 11. Static concatenation

If front end proves fully static content, collapse into static literal bytes without allocating or observing intermediate strings. Preserve effects of evaluated operands.

## § 12. Text arrays

`StringToArray(kind)` yields independent P18 owner, and `SequenceToString(kind)` yields a string; preserve exact element data/bounds and materialization facts. Public error/parameter form gated MD-072 (P23-D2) and MD-050/MD-073 (P23-D3) as relevant.

## § 13. String identity ToString

`StringIdentity` yields already-typed core `Result[string,StringError]` success; no allocation/content copy.

## § 14. Error/try CFG

Fallible materialization uses checked `(candidate, failed, error)` or typed `Result`, success-only candidate, original error propagated via P10/P11. No private string-specific error algebra invented.

## § 15. Effects

Separate no-allocate count/compare/identity from materializing allocate/context effects and potentially failing conversions. Connect P19 context and P17 cleanup only when predecessors are implemented.

## § 16. Verifier

Verify type/provenance, literal payload, length units, lifetime/epoch, single-allocation concat, typed errors, success-only use and private-helper authorization.

## § 17. Printer

Deterministic stable printed semantic fields and source location. Never dump raw physical pointer or choose descriptor layout.
