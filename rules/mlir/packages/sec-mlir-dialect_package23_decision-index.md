# P23 Decision Index

- **Status:** Decision index; proposed questions are OPEN
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec-mlir-dialect_package23_decision-index.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P23`

---

## § 1. Newly identified independent decisions

| ID | Question | Status | Ownership |
|---|---|---|---|
| MD-071 (P23-D1) | Public string indices/offset units and valid UTF-8 byte boundaries | Decided 2026-10-17: rune positions for IndexOf/LastIndexOf/Slice/GetRune/RuneAt, byte indexes for GetByte/ByteAt/CharAt, existing RangeError members | `rules/corrections/applied/md050-md071-md072-string-utf8-positions-materialization-correction-20261017.md`; `core-library.md` § 5.7 |
| MD-072 (P23-D2) | Byte/rune array-string materialization source signatures, ownership and allocation/error surface | Decided 2026-10-17: borrowed `ref` input with `Result[string, StringError]`; owning `Result[byte[]/rune[], CollectionError]` output; always independent storage | Same correction; `core-library.md` §§ 5.2–5.3 |
| MD-073 (P23-D3) | `ToCharArray` and char-to-string conversion outside 0..255 | Open | Recorded in `missing-decisions.yaml` (2026-10-16) |

## § 2. Existing decisions and blockers — do not duplicate

| ID | State | P23 handling |
|---|---|---|
| MD-004 | Applied | One runtime concatenation/interpolation materialization; AllocationError. |
| MD-050 | Applied 2026-10-17 | Every string is valid UTF-8; malformed bytes stay binary and strict decoding returns `StringError.InvalidUtf8` (`types.md`, "Valid UTF-8 invariant"). |
| MD-058 | Applied | `.Len` scalar count; value `.SizeOf` byte count; no public ByteLen/RuneLen. |
| MD-010 | Existing | Regex contract does not become P23 string algorithm. |
| P21-D1–D3 | P21-specific | No interference. |

## § 3. Approval protocol

The Sec owner maps each P23-D# to an MD-###. Mapped 2026-10-16: P23-D1 → MD-071, P23-D2 → MD-072, P23-D3 → MD-073; the decision texts (facts, options, non-accepted recommendation, gates) now live in those `missing-decisions.yaml` entries instead of separate files. Each is a **proposal**, not an authorized semantic change. Codex may implement only independent work while any decision remains open. Record the approved MD identifier and synchronization commit/revision before implementing gated source APIs.

## § 4. No invented decision for length arithmetic

`len(string)` `int` representability on extreme runtime lengths is already flagged across P16/P18; keep that cross-package gap rather than create a synonymous P23-D4. Recorded 2026-10-16 as MD-074 and decided 2026-10-17: `len(...)` returns `uint` like `.Len`, so no representability gap remains.
