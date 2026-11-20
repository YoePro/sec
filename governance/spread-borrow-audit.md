# Call-spread borrow and provenance audit

Owner: `frontend.spread-validity-and-ownership` in `declarations.yaml`.
Audited: 2026-11-20. Defect: `BUG-000031`.

The audit covers prepared call arguments, not struct/array literal spread.
The contracts are `rules/declarations/spread.md` §§3–4,
`rules/memory/borrowing.md` §§15.2–15.5, and
`rules/memory/lifetime_analysis.md` §28. An implementation limit is reported
under `rules/compiler/compiler_pipeline.md` §32 rather than classified as a
source-language violation.

## Findings and resolution

1. `validateCallArgumentBorrows` skipped every prepared slot. Thus
   `Exclusive(values, values...)` could borrow `values` exclusively for the
   first parameter and still read it to prepare later scalar copies. Sema now
   consumes the source/slot plan before borrow commit and checks each spread
   source read against earlier exclusive reservations. Referenced sources use
   their canonical referent Place, including field projections, rather than a
   reference holder's storage. One source occurrence produces at most one read
   conflict, regardless of its expansion count. The diagnostic is the existing
   mandatory `S1161`, with the earlier argument as its related source.

2. Prepared slots were also skipped by reference-plan and escape recording,
   while returned-origin instantiation received the repeated whole array or
   pack expressions. A source table and element index do not themselves prove
   the lifetime or retention of a prepared reference. Until per-slot borrow,
   referent and capture-environment plans exist, prepared arguments requiring
   borrowed destinations or carrying stored references/callable environments
   stop at `S1239`. No resolved call or returned-origin fact is published for
   them. This is a conservative implementation boundary, not a rule banning
   reference-bearing spread in Sec. Passing explicit indexed arguments uses
   the existing ordinary reference checks and is the suggested alternative.

3. Expected-type generic overload selection passed runtime-segment flags to
   ownership/reference helpers where prepared-value flags were needed. That
   path now uses the same prepared flags as ordinary overload selection, so a
   consuming generic destination receives copies and cannot consume the
   original fixed array.

4. Declared heterogeneous-pack forwarding uses its existing separate
   specialization, loan and returned-origin machinery. It does not enter the
   ordinary prepared-slot bypass. Its whole-pack source-origin sets remain
   conservative, and its owned/shared/exclusive forwarding regressions remain
   part of verification. No independent lifetime engine was added.

5. A computed spread source such as `Snapshot(values)...` has no reusable
   Place describing accesses during its evaluation. If earlier arguments
   have already prepared borrows, Sema reports `S1239` for missing access-effect
   provenance. Materializing the source before the call avoids that boundary
   and lets ordinary borrow checks validate its evaluation separately.
   Empty spreads retain their source evaluation despite supplying no slots;
   the same check runs at their original position, including trailing empty
   sources. An empty computed source before the borrow remains accepted.

## Coverage and remaining capabilities

| Case | Result |
| --- | --- |
| Exclusive earlier argument, overlapping fixed-array source | `S1161` |
| Exclusive earlier argument, overlapping struct field source | `S1161` |
| Exclusive earlier argument, overlapping runtime slice source | `S1161` |
| Shared earlier borrow and scalar copies | Accepted |
| Scalar copies prepared before a later exclusive argument | Accepted |
| Distinct arrays or disjoint struct fields | Accepted |
| Duplicate scalar spreads, homogeneous forwarding and whole-pack transfer | Accepted |
| Expected-type consuming generic call followed by original-array use | Accepted |
| Prepared borrowed destination, stored reference or callable environment | `S1239` pending per-slot provenance |
| Computed source evaluated after a borrowed argument | `S1239` pending access-effect provenance |
| Computed source materialized before preparing borrowed arguments | Accepted |
| Empty computed source after/before preparing a borrowed argument | `S1239` / accepted |

The new source fixtures run at Interactive, Standard and Deep depth on both
32-bit and 64-bit plans. CLI check/IR and unsaved LSP tests retain the same
diagnostic identities and source links. Existing pack/reference regressions
cover the separate forwarding path.

Per-slot reference support remains partial. This audit does not complete
runtime homogeneous-pack representation, borrowed-array/semantic-copy
normalization, native array lowering, opaque foreign retention contracts, or
generic lifetime infrastructure owned elsewhere. Scalar copy facts are not
borrow/retention facts and must not be reused as such.
