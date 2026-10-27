# MD-115 — Canonical returned-slice range origin projections

- **Decision:** MD-115 — `returned-slice-origin-projection`
- **Status:** **Applied — normative integration and implementation verified 2026-10-27**
- **Created:** 2026-10-27
- **Last updated:** 2026-10-27
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Reference repository:** `YoePro/sec`, `main` at `5426af6cf8cce6fe85ff897b97050dfd330ef4fe` (2026-10-26)
- **Applied correction location:** `rules/corrections/applied/MD-115-returned-slice-origin-projections-correction-20261027.md`
- **Archive after full normative integration:** `rules/corrections/applied/MD-115-returned-slice-origin-projections-correction-YYYYMMDD.md`
- **Primary normative owners:** `rules/mlir/sec_mlir_dialect.md` (schema-12 amendment), `rules/compiler/semantic_ir.md`
- **Related normative owners:** `rules/mlir/packages/sec-mlir-dialect_package16.md` §§ 25–30, 66–67, 83–86; `rules/mlir/sec_mlir_lowering.md`; `rules/mlir/dialect-versions/sec_mlir_dialect_v12.md` (append-only amendment); `rules/mlir/lowering-versions/sec_mlir_lowering_v12.md` (append-only amendment); inherited `rules/mlir/dialect-versions/sec_mlir_dialect_v11.md` only by cross-reference; `rules/memory/references.md` § 9
- **Tracking:** `missing-decisions.yaml` MD-115; `package16-todo.md` P16-26, P16-27, P16-36, P16-55, P16-56, P16-58; existing `governance/lowering.yaml` integration `lowering.sec-mlir-package16`
- **Related decisions:** MD-084 (closed return-origin grammar and exact metadata), MD-085 (path/callee-aware verification), MD-065 (borrow-end boundaries), MD-116 (static-origin backing where applicable)
- **Decision authority:** Approved by the Sec language owner on 2026-10-27. In particular, the owner accepted **`[?]`** rather than the earlier MD-115 proposal **`[..]`** for an indeterminate interval.

> **Integration contract:** This file is the new normative *range-projection* amendment to the MD-084/MD-085 return-origin system. Do not edit historical paragraph numbers, quietly relax source borrow checks, or mark P16 accepted before verification and remaining implementation. The new `[?]` syntax is **compiler metadata only**, never source-level slicing syntax.

---

## § 1. Purpose, existing authority, and non-goals

**§ 1(1).** MD-115 adds an exact or conservative **slice-range projection** to the existing `sec.return_origins` path grammar. It fills the P16 §§ 66–67 gap for a reference/slice returned from a **sub-range** of a caller-supplied array or slice, while preserving the original source storage identity.

**§ 1(2).** Sema remains the sole authority for the legality of returning a slice, candidate origin set, lifetime/borrowing/exclusive authority, normalized range, and its known or dynamic provenance. Semantic IR retains these resolved facts. The Sec MLIR emitter serializes them; native verifiers check structural truth against represented operations, not a second source-language borrow checker.

**§ 1(3).** The decision does not add source syntax; a runtime tagged slice type; a physical slice descriptor, pointer-plus-length layout, or slice ABI; new error kinds; implicit `any`; a new lifetime annotation; or a new rule allowing an escaping reference to callee-local storage.

**§ 1(4).** Schema **12**, the P16 slice schema, owns this extension. The MD-084 schema-11 grammar remains the original grammar for its earlier contracts; schema-12 augments that inherited return-origin grammar explicitly. Schemas 1–11 must not silently accept new `[A..<B]` or `[?]` *return-origin* projections as if they were already part of the prior schema. Existing schema-11 root and element projections remain accepted in schema 12.

## § 2. Canonical schema-12 origin grammar

**§ 2(1).** Extend MD-084 § 4's grammar in schema 12 as follows. Existing root and projection forms are repeated below for clarity; this correction adds exactly the last two alternatives:

```ebnf
origin           := root projection*
root             := "param#" unsigned32 | "receiver" | "static@" unsigned32
projection       := ".field#" unsigned32
                  | "[" unsigned32 "]"
                  | "[*]"
                  | "[" natural "..<" natural "]"
                  | "[?]"
unsigned32       := "0" | nonzero_digit digit*  // existing MD-084 domain and checks
natural          := "0" | nonzero_digit digit*  // arbitrary precision
nonzero_digit    := "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9"
digit            := "0" | nonzero_digit
```

The comments in this specification grammar are explanatory, not part of serialized origin strings. Existing MD-084 constraints on `unsigned32`/existing projected index widths and on admissible fields are preserved; **range endpoints** use arbitrary-precision nonnegative integers. No source-level integer cast or host-width truncation is permitted while parsing these endpoints.

**§ 2(2).** Exact half-open interval projection spelling:

```text
param#0[1..<3]
```

It denotes the range **`[1, 3)` relative to the immediately preceding origin's current sequence/view**. The end is exclusive. The interval `A..<B` must satisfy `0 <= A <= B`. The canonical lexical form contains no whitespace, sign, redundant leading zeros, decimal separators, or alternative upper-bound spelling.

**§ 2(3).** Indeterminate range projection spelling:

```text
param#0[?]
```

`[?]` denotes **some valid sub-range of the immediately preceding sequence, with boundaries not statically expressible in the return summary**. It may be empty or span the entire input extent; the summary grants **no** disjointness or narrower-bound proof. It preserves the origin and containment relation, not a chosen runtime start/length pair.

**§ 2(4).** **Do not use `[..]` to mean an unknown range.** In Sec source, `[..]` means the whole current extent. In return-origin metadata, an unprojected root such as `param#0` denotes the unchanged, whole current source view when that is the established returned relationship. Unknown and unchanged are not synonyms. `[..]` is **not** a new canonical metadata projection in schema 12.

**§ 2(5).** Existing `[*]` means a **single element at an unknown index** and retains its established field/element projection semantics; it does not mean an unknown slice interval. A range projection preserves sequence/view identity, whereas an element projection changes from sequence to element. Neither form may be silently substituted for the other.

**§ 2(6).** The grammar is case-sensitive and whitespace-free; duplicate complete origin entries are forbidden; canonical arrays are emitted in the deterministic origin-summary order established by MD-084. A parser rejects malformed, ambiguous, overflowed *old fixed-width tokens*, or unsupported paths rather than repairing them.

## § 3. Type-aware meaning and valid source domains

**§ 3(1).** A range projection applies only to a sequence/view whose canonical type admits slice narrowing: a represented fixed-array origin or a shared/mutable slice origin, as established by P16. It produces a slice-range relationship of the same element type; it is not a field selection, integer-value extraction, or permitted implicit array-to-slice conversion.

**§ 3(2).** A range projection yields a type-compatible shared or mutable slice *view* of the same element type. When the range is the terminal projection, the returned result must be a compatible slice. A subsequent already permitted element projection may produce a direct element reference, but it must preserve the intervening range and its verified bounds; implementation may fail closed until that complete chain is supported. Authority may be retained or reduced when Sema permits it; it may not be increased. A mutable view/result cannot be justified by a shared origin or by the mere presence of a textual range suffix.

**§ 3(3).** `param#0[1..<3]` does **not** assert that every conceivable runtime parameter length is at least three. Actual creation must be valid on the executed path: any necessary dynamic range check/guard, source-reference validity guard and failure semantics are those already resolved by Sema and represented in the P16 IR/MLIR operations. A typed origin label cannot substitute for a dominating guard.

**§ 3(4).** An exact zero-length projection, such as `[2..<2]`, is valid when source-relative bounds are established. It borrows no elements, but still retains the source's storage, lifetime, epoch, and pin dependencies as required by P16 § 29. An empty range must not be treated as an originless reference.

**§ 3(5).** Projections may follow a canonical field path or eligible array/view projection. The verifier checks each path transition against the current typed referent and rejects an impossible composite path. Grammar acceptance alone never proves type or lifetime admissibility.

## § 4. Exactness, composition, and loss of precision

**§ 4(1).** P16 § 14's normalization to `[start, endExclusive)` and § 28's nested-range composition are authoritative. Exact ranges compose by **source-relative addition** when their complete bounds are statically established:

```text
[10..<20] followed by [2..<5]  ->  [12..<15]
```

The final origin summary represents the normalized range relative to the original eligible origin, not a sequence of redundant unnormalized subranges when exact composition is proven.

**§ 4(2).** Composition uses arbitrary-precision integer mathematics. It neither truncates at `int64`/`uint64` nor rounds, wraps, saturates, or fabricates a target `uint` proof. Runtime representability and guards are checked separately against the selected `CompilationPlan`.

**§ 4(3).** If a necessary bound is genuinely dynamic or the resolved semantic relationship is not statically expressible relative to the original eligible origin, the source origin is retained and range precision conservatively becomes `[?]`, unless an already represented and verified stronger relationship remains valid. **A compiler implementation gap or incomplete verifier is not permission to discard an exact range that Sema has established:** such a path remains unsupported until the exact contract can be transported and verified. A dynamic `SliceRangeID` may still exist in Sema/IR for local reasoning; this decision does not serialize that compiler-local ID as a runtime field or as a fabricated constant bound.

**§ 4(4).** A full-open reborrow of an **unchanged current view** preserves its existing origin summary. In particular, returning a parameter's complete view remains `param#0`. A full-open reborrow of a previously narrowed view preserves **that narrowed origin**; it does not widen to `param#0` or forget an existing `[A..<B]` / `[?]` projection.

**§ 4(5).** Unknown ranges conservatively overlap any portion of their source that they could occupy. A static exact sibling may be shown disjoint only by the already permitted P16 proof. `param#0[?]` is not evidence of a nonempty, proper, or independent slice. Unknown must never be optimized into a narrower exact range without a separate valid proof.

**§ 4(6).** A return can have a finite set of possible roots and ranges, including one exact origin and one unknown origin. Their distinct candidate paths remain distinct and checked independently. A union of alternatives is not license to drop one possibility or to choose the shortest-looking path.

## § 5. Canonical Semantic IR representation

**§ 5(1).** Extend the existing `ReturnOriginStep` algebra with a **slice-range** step, conceptually `ReturnOriginSlice` (final implementation identifier may follow established Go naming), in addition to existing `ReturnOriginField` and `ReturnOriginElement`. No second return-origin system, slice provenance owner, or new source expression node is introduced.

**§ 5(2).** A slice-range step has exactly one of two semantic cases:

```text
ExactRange(start: arbitrary-precision integer,
           endExclusive: arbitrary-precision integer)
UnknownRange
```

An exact case has both bounds, with `0 <= start <= endExclusive`. The unknown case carries no invented concrete bounds. The two cases cannot be simultaneously true or implicitly defaulted from missing fields.

**§ 5(3).** Return-origin steps are detached, immutable/deep-copied semantic facts. Arbitrary-precision integer instances are copied on publication/query/clone, not shared as mutable `big.Int` pointers. Equality, deterministic printing and caching compare canonical numeric values and path kinds, not mutable pointer identity.

**§ 5(4).** `ReturnOrigin` still carries the existing parameter/receiver/static root and its ordered projection path; source names are not identity. `static@N` still requires MD-116's verified declaration backing wherever a static slice-origin root is admitted. P16 adds only the new projected *range* information.

**§ 5(5).** Sema's `StaticSliceRange`, `SliceRangeID`, candidate origins, validity and borrowing plans remain authoritative. The Semantic IR builder uses those plans and existing source/IR guard decisions rather than re-evaluating a source slice expression to infer a more convenient origin.

## § 6. Schema encoding, proof transport, and versioning

**§ 6(1).** In schema 12, `sec.return_origins` remains the **same function-level array-of-strings attribute** defined by MD-084. The new `[A..<B]` / `[?]` suffixes are the only additions to the existing root/path grammar. There is no second `sec.slice_return_origins` attribute.

**§ 6(2).** The emitter serializes an exact range using the canonical decimal text of its arbitrary-precision endpoints. The native parser must retain the exact values (for example through arbitrary-precision decimal/APInt representations) and not narrow them to the `uint32_t` used for legacy root IDs or single-element projections. A safe parser may reject resource-exhausting input with an explicit resource diagnostic, but must never silently truncate or equate distinct valid bounds.

**§ 6(3).** The set-ordering, spelling, escaping, and duplicate rejection rules of MD-084 remain in force. Schema-12 parsing and printing round-trip the same exact or conservative meaning; no parser reinterprets `[?]` as `[*]` or silently normalizes it to `param#0`.

**§ 6(4).** The existing `sec.reference` dictionary and slice operation metadata continue to carry borrow/lifetime, validity, storage identity, source/provenance, epoch dependency, address stability, pinning, and address space as their respective owning contracts specify. **Do not duplicate these facts in the range suffix.** `[?]` is a statement about spatial precision only, not missing reference validity or authorization.

**§ 6(5).** Schemas 1–11 remain readable under their previously installed grammar. Emission requiring a returned sub-slice range either selects schema 12 through the existing explicit schema-selection mechanism or rejects emission as unsupported. Do not upgrade the declared schema invisibly and do not embed an unadmitted schema-12 suffix into schema 11.

## § 7. Native verifier and return-origin traversal

**§ 7(1).** Extend the existing MD-085 origin tracer, grammar/parser and type-aware admissibility check for the two range projections. The verifier must check that each *observed* returned slice origin is included in the declared finite `sec.return_origins` set under the established conservative-inclusion rules, while respecting the actual range operations represented in MLIR.

**§ 7(2).** For `sec.slice.borrow_shared`, `sec.slice.borrow_mut`, `sec.slice.reborrow_shared`, and `sec.slice.reborrow_mut`, the tracer examines the canonical normalized start/length operands, their origin, and the source slice/array. For an exact range claim, it proves **the selected half-open endpoints and source-relative composition** from the represented SSA/canonical range operations. It must not accept a declared exact range based solely on the attribute text.

**§ 7(3).** A range whose exact origin-relative bounds cannot be structurally established may be summarized as `[?]` **only when** its represented operations prove it stays within the eligible source and the already resolved permission to return it holds. Unknown is not an escape hatch for forged storage origins, missing spatial guards, unsupported source operations, or incorrect mutable authority.

**§ 7(4).** An explicit `[..]` full-open operation over a known current slice view preserves the current return-origin path. A full-open operation over a narrowed slice must retain the preceding narrow path. The verifier may not trace operand zero and erase narrowing merely because the *outermost* operation has `source-open-bound` proof kind.

**§ 7(5).** The verifier follows shared copies, mutable moves, reborrows, Place projections, block arguments, joins, and verified direct calls as required by MD-085. Direct-call composition substitutes the callee's parameter/receiver origin path against the actual caller input and composes slice ranges; it must not conflate a callee's static origin with an unrelated caller static or treat a callee's `param#0[?]` as an exact range.

**§ 7(6).** A returned local-array slice remains invalid even if annotated with `[?]` or a syntactically valid `[A..<B]`. The returned-reference proof must still establish every eligible storage root and any relevant lifetime, epoch, pin, and exclusivity obligations. The tracer reports failure rather than manufacturing an external root.

**§ 7(7).** Verification is deterministic and fail-closed on unsupported CFG relations, unknown unvalidated callee summaries, and unrepresentable projections. A source case whose exact bounds are *semantically unknown* but whose allowed source containment is proved may use `[?]`; a case whose bounds Sema established exactly cannot be weakened just to evade an unimplemented emitter or verifier.

## § 8. No new source syntax, errors, or runtime representation

**§ 8(1).** Ordinary Sec source expressions keep their existing grammar, including `values[1..<3]`, `values[..]`, and fallible forms with the required `try`. **`[?]` has no source-language spelling or parser role.**

**§ 8(2).** Existing range-check evaluation order, `RangeError`, guarded/naked propagation, source validity and failure precedence are unchanged. The returned-origin metadata does not perform a second range check, change `try` semantics, or introduce a hidden runtime bounds failure.

**§ 8(3).** No pointer layout, fat-slice representation, ABI, allocation, new hidden reference field, or automatic independent borrow checking is selected by this correction. P17/P18 and later representation packages retain their documented responsibilities.

## § 9. Canonical semantic examples

**§ 9(1).** Examples of the canonical **origin metadata** (not source expressions):

| Origin metadata | Semantic relation |
| --- | --- |
| `param#0` | The complete current input view, unchanged. |
| `param#0[1..<3]` | An exact normalized range `[1, 3)` within parameter 0's current view. |
| `param#0[?]` | An indeterminate range within parameter 0's current view; conservative overlap. |
| `param#0[*]` | A single element at an indeterminate index, not a slice range. |
| `param#0.field#2[4..<8]` | An exact range of the type-valid sequence in stored field 2. |
| `param#0[0..<0]` | An exact empty range that retains origin/lifetime facts. |

**§ 9(2).** Nested exact ranges normalize to one canonical relative range:

```text
parameter view [0..<100]
first narrowing   [10..<20]
second narrowing   [2..<5] (relative to first)
returned origin    param#0[12..<15]
```

**§ 9(3).** A dynamic source-relative sub-range whose precise endpoint cannot be published statically returns an origin summarized as `param#0[?]`, while dynamic bounds and guards still occur in the executable IR. It must not be serialized as `param#0`, `param#0[..]`, or `param#0[*]`.

**§ 9(4).** A full-open reborrow of an input whose current extent is the full parameter returns `param#0`. If the compiler first narrows to `[12..<15]` and then performs a full-open reborrow of that child, the returned origin remains `param#0[12..<15]`.

## § 10. Mandatory positive and negative tests

**§ 10(1).** **Parser/printer:** exact `[1..<3]`, `[0..<0]`, `[?]`, path-composed field/range, deterministic multi-origin summary; arbitrary-precision endpoints well beyond 32- and 64-bit host integers; repeated parse/print preserves the canonical bytes and semantics.

**§ 10(2).** **Grammar negatives:** `[3..<1]`, negative bounds, leading zeros, whitespace, omitted end, `[..]` as purported unknown, `[?` and `[*` malformed syntax, invalid non-sequence projection, illegal authority escalation, duplicate complete origins, and new projections in schema 11 or earlier.

**§ 10(3).** **Source/IR/schema positives:** fixed-array and slice-source subranges, shared/mutable permitted returns, empty range, nested exact composition, start-only or dynamic endpoints represented as `[?]`, an unchanged/full-open view retaining its current origin, and an already narrowed view retaining its narrowing through an outer `[..]`.

**§ 10(4).** **Verifier forgeries:** exact interval text contradicts actual range SSA operands; metadata asserts `[1..<3]` after a different range; guard missing/dominance wrong; `param#0` falsely hides a narrower range; `[?]` used to hide an illegal local origin; `[*]` used to mean a slice; loss of epoch/pin/address-stability facts; invalid typed field/range chain; forged disjointness; unsupported callee summary; wrong-origin merge.

**§ 10(5).** **Control-flow/interprocedural:** copy/move, full/partial reborrow, call-return origin composition, forwarded block arguments and joins, loops with a provable conservative fixed point, finite alternative origins, no accidental widening, and deterministic rejection where required proof is missing.

**§ 10(6).** **Target and predecessor matrix:** 32-bit and 64-bit CompilationPlans; schema-12 output through the existing `--sec-verify-places`, `--sec-verify-reference-guards`, `--sec-verify-borrow-semantics`, and `--sec-verify-slice-guards` passes, before/after the relevant P6 scalar-resolution stage; P15/MD-084/MD-085 regressions remain green. Failed emission/verification yields no partially published module artifact.

## § 11. Required synchronization and integration order

**§ 11(1).** Extend the canonical schema-12 portions of `rules/mlir/sec_mlir_dialect.md` with this additive MD-084 origin-grammar amendment, and add an **append-only** correction note to `rules/mlir/dialect-versions/sec_mlir_dialect_v12.md`. Do not rewrite the original schema-11 MD-084 grammar to claim schema-11 support for the new suffixes.

**§ 11(2).** Add the slice-range step and its exact/unknown contract to `rules/compiler/semantic_ir.md` (existing return-origin/reference and P16 slice sections), and add a precise return/lowering synchronization note to `rules/mlir/sec_mlir_lowering.md` and the version-12 lowering snapshot. Amend P16 §§ 66–67 / verifier requirements by stable-numbered additions rather than wholesale replacements.

**§ 11(3).** Update the existing `ReturnOriginStep` producer/cloning/printing/verification chain, the shared native return-origin parser and MD-085 tracer, and schema-12 emitter. The existing `ReturnOrigin` algebra is the only return-origin representation; source range truth continues to come from Sema. Do not introduce a separate return-summary path that can drift from MD-084/MD-085.

**§ 11(4).** Implement and validate the above in a sequence that preserves fail-closed gates: (a) canonical grammar and exact/unknown model; (b) parser/type verification; (c) SemIR range-origin producer/normalization; (d) emitter; (e) structural tracing/call composition; (f) source-to-schema-12 tests and regressions. Do not remove an existing rejection before the corresponding complete verifier is in place.

**§ 11(5).** Update `package16-todo.md` P16-26, P16-27, P16-36 with the actual resolved implementation boundaries. The existing `lowering.sec-mlir-package16` entry in `governance/lowering.yaml` remains the **single** package owner; amend its `implemented`, `remaining`, and tests with evidence as work advances. Do not create a duplicate ownership entry or promote P16 on the strength of a formatting/grammar change alone.

**§ 11(6).** When all affected normative rulebooks have been synchronized, remove or transition MD-115 in `missing-decisions.yaml` according to its documented closure policy. Record implementation work separately in governance and retain `partial` until the required acceptance gates pass. Only then move this pending correction to `rules/corrections/applied/` using the actual application date, and update rulebook-status tracking when appropriate.

## § 12. Final accepted decision and excluded claims

**§ 12(1).** **Accepted exact range syntax:** `[A..<B]` with canonical nonnegative arbitrary-precision half-open bounds; source-relative semantics; existing P16 checked normalization/composition.

**§ 12(2).** **Accepted unknown range syntax:** `[?]`; unknown range is not whole range `[..]` and is not an unknown element `[*]`. The exact source/storage origin and the conservative containment relation must remain intact.

**§ 12(3).** **Accepted metadata model:** extend existing `ReturnOriginStep`, `sec.return_origins`, and the MD-085 verifier; do not duplicate reference validity, lifetime, address-stability, epoch, or pin data in the range suffix.

**§ 12(4).** **Not claimed:** completion of P16-26, P16-27 or P16-36, resolution of broader P16 CFG/value-flow limitations, physical slice lowering, an additional source feature, or P16 final acceptance. The normative decision is complete upon integration; implementation status requires tests and governance evidence.
