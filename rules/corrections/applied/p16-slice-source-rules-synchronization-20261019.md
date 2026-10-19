# Correction: P16 canonical slice source-rule synchronization

- **Status:** Applied 2026-10-19
- **Created:** 2026-10-19
- **Document revision:** 1
- **Sec version:** 0.1
- **Work item:** P16-04
- **Source authority:** `rules/mlir/normative-sync/sec_slice_sync_v1.md` §§ 1–19;
  the applied MD-074/MD-088 length correction (2026-10-17) §§ 1–3;
  `rules/collections/collections.md` § 7.5 (MD-094, 2026-10-18)
- **Targets:** `rules/collections/collections.md`,
  `rules/errors/runtime_checks.md`, `rules/library/core-library.md`,
  `rules/memory/reference_model.md`, `rules/memory/copy_move.md`

## Audit and application

The table covers every source synchronization section. Existing correct text
is retained; the owning sections below now contain the missing details.

| Sync section | Canonical owner and result |
|---|---|
| 1 explicit construction | collections §§ 7.3, 7.5; no implicit owner conversion or bare-range slice creation |
| 2 explicit fallible borrow | collections § 7.5 and runtime_checks "Slice checks"; `try ref` / `try ref mut`, grouped equivalent; obsolete bare-range example replaced |
| 3 half-open normalization | collections § 7.4; omitted start 0, omitted end source length, inclusive increment after validation; exact arithmetic, one source/start/end evaluation in order |
| 4 empty slices | collections § 7.4 and reference_model § 14(9); zero-length source and equal exclusive bounds valid, provenance/lifetime retained |
| 5 core RangeError mapping | collections § 7.5 and runtime_checks "Slice checks"; InvalidRange / StartAfterEnd / OutOfBounds; existing core-library § 20.1 declares this same enum |
| 6 deterministic precedence | collections § 7.5; the existing MD-094 single-primary constant diagnostic is retained, runtime selection and optimization preservation now explicit |
| 7 ordinary range panic | collections § 7.5 and runtime_checks "Slice checks"; panic.bounds with slice-range provenance |
| 8 fallible spatial failure | collections § 7.5; try converts RangeError, never stale-reference failure |
| 9 index errors | collections § 7.5 and runtime_checks "Slice checks"; IndexError.OutOfBounds or panic.bounds with slice-index provenance, no BoundsError |
| 10 temporal before spatial | reference_model § 14(11), collections § 7.5, runtime_checks "Slice checks"; operand evaluation, dynamic validity, spatial check; stale panic remains uncatchable by spatial try |
| 11 exact static precision | collections § 7.4 and reference_model § 14(9); arbitrary-precision endpoints and nested composition, no host-int64 correctness limit |
| 12 conservative dynamic ranges | collections § 7.4 and reference_model § 14(9); overlap unless proof, no runtime borrow checker |
| 13 slice classifications | collections § 7.1, copy_move § 4.4(4), reference_model § 14(8), core-library § 13.1; shared CopyTrivial, mutable MoveOnly, both trivially destructible and non-owning |
| 14 equality invalid | collections § 7.1 and reference_model § 14(12); no scalar-reference equality extension to slices |
| 15 unsigned property | collections § 3.1 and core-library § 13.1 already declare Len: uint; sync heading/example and stale core summary spelling corrected to .Len |
| 16 unsigned global len | core-library § 1.4 and collections § 3.1 already agree; obsolete core implementation-summary int claim corrected to uint |
| 17 unsigned representability | core-library § 1.4 retains exact uint length and separate checked signed conversion; no new signed-length policy |
| 18 owning array boundary | collections §§ 6.1, 7.1 and reference_model § 14(8); T[] owner remains separate from borrowed slices |
| 19 synchronization sweep | canonical source books updated here; existing Sema exact ranges/plans/effects and regression fixtures remain the implementation anchors; Semantic IR and dialect/lowering installation is independently completed by P16-05 |

`runtime_checks.md` previously included "valid storage origin" in the spatial
slice-check list and showed `try values[start..<end]`. The replacement makes
explicit that temporal failure is a different panic path and slice production
requires `ref`. The source semantics are already required by the supplied sync;
this correction introduces no new syntax or error type.

No manual files or standard-library sources are changed. P16's supplied sync
§ 15 spelling is updated in place to `.Len` to agree with collections § 3.1;
its resolved §§ 16–17 already reflect the accepted unsigned-length correction.

## Implementation boundary

This completes source-rule synchronization, not full source-to-binary slice
lowering. P16-11 source evaluation/normalization verification, P16-17 origin
propagation, P16-19 comprehensive Sema tests and remaining IR/MLIR production
have their own implementation items. Existing P16 range, overlap, provenance,
plan, effect and unsigned-length tests remain relevant; their presence does
not complete those broader items. MD-084 metadata serialization remains open
and is not decided by these source rules.
