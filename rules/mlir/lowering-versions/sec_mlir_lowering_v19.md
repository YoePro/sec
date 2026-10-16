# Sec MLIR Lowering — Planned v19

- **Status:** Normative additive amendment (subject to existing MD decisions)
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/sec_mlir_lowering.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P23`

---

## § 1. Prerequisites

Check actual current implementation of schemas v15–v18, P15–P19 and full Go/MLIR toolchain. Lower only operations whose prerequisites are present; report explicit UnsupportedFeatureError for missing pipeline nodes.

## § 2. Literal lowering

Keep static string literal symbol + exact UTF-8 bytes and symbol lifetime. No intermediate `byte[]` source value and no runtime allocation. Static backing representation remains high-level until layout package.

## § 3. ByteLen

Can use proven stored UTF-8 byte extent or bounded runtime descriptor fact. Never compute `strlen` and never use scalar `.Len`.

## § 4. RuneLen

For proven-valid UTF-8 use bounded scalar count; reuse or replace legacy generated helper after parity tests. Invalid-UTF8 behavior remains MD-050 gated.

## § 5. Signed `len`

Emit target `int` width only after a proven representable scalar count, else diagnostic/explicit unsupported gap matching P16/P18; no truncation.

## § 6. Compare/equality

Lower to bounded lexicographic byte operations for proven-valid UTF-8; do not compare addresses or signed host bytes. Preserve exact source operand evaluation.

## § 7. Private byte substring

Translate only after checked public caller or proven core origin; use byte interval and lifetime facts, do not materialize new storage or widen lifetime. Do not make helper generally compiler-known.

## § 8. Public offset-sensitive methods

Before MD-071 (P23-D1) approval implement only private byte range and pure comparison; no guessed public byte/rune interpretation.

## § 9. Static concat

Fold to static immutable bytes, keep source effects. Do not create Result failure edge after full static proof.

## § 10. Runtime MD-004 concat

In source order evaluate segments, then checked total UTF-8 byte count; consume P19 context, acquire final backing once, write piece content, publish immutable string and cleanup candidates on failure. A source `try` uses `AllocationError` exact P10/P11 flow.

## § 11. Conversion to owning arrays

Use P18 backing/owner, P19 allocation context, P17 destructor when implemented. Byte result count equals source `.SizeOf`; rune count equals `.Len`. Result arrays independent and mutable without changing source. Public signature choice gated MD-072 (P23-D2).

## § 12. Byte/rune to string

Validate/encode without implicit C encoding or terminator scans. Do not treat array view as string without lifetime proof or allocation contract. UTF-8 policy gated MD-050, signature MD-072 (P23-D2).

## § 13. Char sequence

Preserve numeric char domain 0..255 and explicit unicode conversion, with MD-073 (P23-D3) gate for nonrepresentable scalars.

## § 14. String identity

Return `Ok(self)` without allocation or byte copy, preserving source type identity and owner/lifetime. No ABI assumptions.

## § 15. P22 integration

Reuse exactly one rune count and byte count implementation for P22 string length contracts. Preserve contract violation order, error class and source ordinal.

## § 16. Tests and target

Require valid-UTF8 byte/rune differential tests, 32/64 CompilationPlans, noalloc static/dynamic cases, malformed MLIR guard tests; no silent fallback to LLVM-specific generated helper. Run and report `go test ./...` / `check-sec-mlir` only when actually executed.

## § 17. Passes

`--sec-lower-string-core`, `--sec-lower-string-materialization`; ordinary verifiers plus new P23 verifiers run pre/post each pass. Maintain effects and source locations.

## § 18. Physical boundary

Do not select string pointer/length descriptor, malloc or C ABI in this lowering spec; prepare the verified high-level representation for proposed P24 layout package.
