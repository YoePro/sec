# MD-084 — Canonical schema-11 reference metadata and schema-12 inheritance

- **Status:** Applied 2026-10-22 — integrated and implemented; register entry removed.
- **Created:** 2026-10-21
- **Last updated:** 2026-10-21
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical correction destination:** `rules/corrections/applied/` after integration
- **Primary normative owners:** `rules/mlir/sec_mlir_dialect.md`; `rules/mlir/sec_mlir_lowering.md`
- **Versioned owners to synchronize without renumbering historical paragraphs:** `rules/mlir/dialect-versions/sec_mlir_dialect_v11.md`; `rules/mlir/lowering-versions/sec_mlir_lowering_v11.md`
- **Implementation package:** `rules/mlir/packages/sec-mlir-dialect_package15.md` and its governance fragment
- **Decision tracking:** `missing-decisions.yaml`, MD-084
- **Related decisions:** MD-085 (verification depth), MD-087 (pass preservation), MD-086 (emission routes)
- **Replaces:** the unresolved schema-11 emitter metadata spelling choices recorded in MD-084; not the semantics of references, lifetime or address stability
- **Reference baseline:** public `main` rulebooks and MD-084 description reviewed 2026-10-21; schema 12 installed 2026-10-19, default emission schema 11 at this baseline

---

## § 1. Authority, applicability and non-goals

§ 1(1) This correction fixes the canonical schema-11 serialization of *already resolved* Semantic IR reference and Place facts. It introduces no source-language syntax, new borrow kind, ABI, physical pointer layout or independent semantic authority.

§ 1(2) The spelling contract is shared by schema 11 and schema 12. Schema 12 inherits it unchanged for shared direct-reference facts and for slice-related references as applicable; the existence of schema 12 does not automatically promote the default emitter or imply P16 completion.

§ 1(3) Sema remains authoritative for meaning. The emitter must preserve exact resolved facts and must fail closed where required facts have no representable form. MD-085 governs how deeply a verifier checks return-origin claims.

§ 1(4) The canonical consolidated rulebooks take precedence when historical version snapshots conflict. Keep existing paragraph identifiers stable; install amendments rather than silently renumbering older sections.

## § 2. Canonical metadata placement

§ 2(1) The schema-11 `sec.reference` *dictionary* is the canonical aggregate of reference facts on a `func.func` argument whose type is `!sec.ref<T>` or `!sec.ref_mut<T>`.

§ 2(2) The same `sec.reference` dictionary is attached to a direct call that produces a reference result, on the call according to the canonical emitter/verifier placement. The emitted facts describe the resulting reference, not the storage held by the callee. A placement that cannot identify which result is described is invalid rather than implicitly guessed; P15 currently supports the represented single-reference-result case.

§ 2(3) `sec.return_origins` is a `func.func` attribute on a function with a reference result. It is an array of canonical origin strings as defined in § 4. It is required and nonempty when the function promises a reference result, including permitted static/receiver forms. A function without a reference result must not emit an invented return-origin summary.

§ 2(4) `sec.address_taken = true` is the canonical marker on `sec.storage.declare` for storage that must remain reference-addressable. It is a boolean `true`, not a string, unit, or optional false marker. `--sec-verify-places` requires it when a `sec.place.storage` addresses the declaration. The producer must provide it before applicable P5/P6 lowering.

§ 2(5) Reference-producing operations may carry the existing operation properties for `borrow_id`, `lifetime_id`, validity, provenance, address stability, and relevant origin/epoch facts. This correction does **not** replace operation properties with `sec.reference` or require both forms redundantly. Dictionary and operation properties must agree when referring to the same fact.

§ 2(6) The supplemental **discardable** attributes `sec.origin_storages`, `sec.pinned`, and `sec.reference_address_space` are allowed only on operations defining exactly one reference as their result, according to their existing verifier-defined value representations. Do not attach them to unrelated operations or silently transfer them to a non-reference result.

## § 3. Exact `sec.reference` field set

§ 3(1) The schema-11 dictionary uses these exact keys, matching the P15-57 implementation choice:

| Key | Semantic content |
| --- | --- |
| `borrow_id` | Canonical borrow identity. |
| `lifetime_id` | Canonical semantic lifetime identity. |
| `validity_policy` | `proven` or `dynamic-epoch`. |
| `validity_proof` | Required proof data for proven validity as governed by the reference contract. |
| `invalidation_domain` | Applicable invalidation domain. |
| `epoch_dependency` | Applicable epoch dependency. |
| `address_stability` | Canonical `AddressStability` semantic value. |
| `provenance` | Resolved reference provenance. |
| `source_borrow` | Source borrow identity when semantically distinct and required. |
| `origin_storages` | Canonical set of origin storage identities. |
| `pinned` | Pin fact, separate from address stability. |
| `address_space` | Reference address space if representable. |

§ 3(2) Required keys and conditional omissions are governed by the underlying reference fact constraints: no placeholder IDs, fabricated proofs or made-up provenance. `dynamic-epoch` does not inherit `proven` proof rules; `proven` requires its permitted proof provenance. Values must satisfy the existing typed-verifier contracts.

§ 3(3) `source_borrow` may be omitted on a copy or move when Semantic IR defines it to equal `borrow_id`; a reborrow must retain a distinct source identity where required. This omission is a defined redundancy elimination, not a missing fact.

§ 3(4) On eligible reference operations, an empty `origin_storages`, `pinned = false`, and the ordinary/default address space may be omitted only where the receiving verifier reconstructs **exactly** the defined default. Omission never means unknown when a security-sensitive fact was required.

§ 3(5) `sec.relocation_class` is **not** a second authority for stability. Where historical schema-11 prose used that name, it is superseded for current emission by canonical `address_stability` with values `Movable`, `Stable`, `Fixed`, and `Unknown`. Pinning is recorded independently; no inference `pinned == Fixed` (or its converse) is permitted.

§ 3(6) If historical schema readers admit `sec.relocation_class` as legacy input, they must define a **separate, explicit** compatibility path. New schema-11/schema-12 emission must not use it or emit contradictory aliases. This correction does not promise automatic legacy rewriting.

## § 4. `sec.return_origins` grammar and canonicalization

§ 4(1) Every array element is a machine-parsable origin expression, **not** unrestricted human-readable text. The admitted surface grammar is:

```text
origin      := root projection*
root        := "param#" unsigned | "receiver" | "static@" unsigned
projection  := ".field#" unsigned | "[" unsigned "]" | "[*]"
unsigned    := "0" | nonzero-digit digit*
nonzero-digit := "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9"
digit       := "0" | nonzero-digit
```

§ 4(2) Decimal numbers are nonnegative, have no sign and no redundant leading zeroes. All tokens and punctuation are case-sensitive. No whitespace or undocumented escape sequences occur inside an origin. Malformed, overflowed-in-implementation or unsupported tokens are rejected, not normalized speculatively.

§ 4(3) `param#N` uses the canonical zero-based function parameter ordinal, not an SSA block-argument ordinal invented after lowering. `receiver` is permitted only if the callable's actual receiver contract supplies one; it is not a synonym for an arbitrary first argument. `static@N` identifies an eligible static origin under the existing resolved static-root scheme and does not by itself authorize returning a static reference.

§ 4(4) `.field#N` is a resolved field identity, `[K]` a constant element projection and `[*]` an abstract dynamic element projection. A parser and verifier must use type- and layout-aware admissibility rules; a lexically valid suffix alone never proves a valid projection.

§ 4(5) The origin array is a finite, canonical set: no duplicate exact entries; serialize deterministically in the ordering defined by the Semantic IR origin summary, with normalized tokens. If the emitter lacks a canonical origin-set ordering, it must adopt one deterministically and consistently across Go emitter, C++ printer and round-trip tests; never depend on hash-map iteration. This correction does not change the *meaning* of origin-set membership.

§ 4(6) The grammar above does not grant permission to invent origins. Existing Semantic IR origin summaries supply the roots and projections. When a future feature needs a new root/projection kind, add it through an explicit schema amendment, not a free-form fallback string.

## § 5. Calls, Place roots and target-dependent facts

§ 5(1) Direct calls use the exact argument-action values `shared-reference` and `mutable-reference` for operands of `!sec.ref<T>` and `!sec.ref_mut<T>` respectively. The schema-2 `copy-trivial`-only action rule does not substitute for these P15 operations. Normal non-reference arguments retain their appropriate existing actions.

§ 5(2) `sec.place.read` in P15's current copy-trivial subset carries `copy-trivial`. This correction does not prematurely allow nontrivial copies or transfers assigned to a later package.

§ 5(3) Dynamic Place-index identity is carried by the **existing SSA index operand**. Do not duplicate an invented dynamic identity attribute. Static index identities remain represented by their actual resolved projection facts.

§ 5(4) `sec.source_name` is provenance-only and must be emitted only if the canonical Semantic IR has an actual source-binding identity. An unnamed value-root does not justify manufacturing a source name. Absence is not a license to fabricate a binding.

§ 5(5) Target-defined address-space values of the form `target:...` lack a supported schema-11 representation at this baseline. Emission must reject them explicitly as unsupported lowering, preserving the resolved source facts and selected `CompilationPlan`. It must not silently map them to ordinary address space.

§ 5(6) Encoding reference metadata does not select a physical `RawPtr`, LLVM pointer, fat reference, side table, epoch field, or reference ABI.

## § 6. Verification, preservation and compatibility

§ 6(1) Dialect attribute verification checks legal placement, exact key spelling, value type, required-field combinations and consistency with result/argument types. A discardable attribute is *not* permission to discard facts still required downstream.

§ 6(2) Lowering or optimization may remove a fact only when its semantic obligation has been discharged and verified or when its elimination is explicitly permitted. Any pass retaining high-level references must preserve the metadata necessary for their later validity checks, lifetime checks and origin tracking.

§ 6(3) On every successful schema-11 or schema-12 round trip, reparsing and reprinting preserves the same semantic origin set, address-stability category, pinning fact, relevant reference IDs and operation argument actions.

§ 6(4) Schema-12 readers and emitters use the same metadata spelling for inherited reference concepts. Schema selection must be explicit according to installed capability: schema-12 parser support alone is not proof of full schema-12 emitter support.

§ 6(5) MD-085's structural verification of returned origins uses the canonical grammar from § 4; this correction does not authorize a new independent source-level borrow analysis.

## § 7. Required implementation synchronization

§ 7(1) Update the **consolidated** dialect and lowering contracts and install an amendment adjacent to existing v11.6/v11.7/v11.33–37 and v11 lowering reference-transport sections. Preserve historical section numbering and annotate superseded `sec.relocation_class` wording in place or by a clearly cross-referenced amendment.

§ 7(2) Synchronize the versioned v11 snapshots, P15 package contract and schema-12 inheritance with a clearly dated amendment; do not silently rewrite completed historical version descriptions without a change record.

§ 7(3) Synchronize Go emitter, C++ parsing/printing, `func.func` arg/result attribute verifiers, `--sec-verify-places`, `--sec-verify-borrow-semantics`, and P15/P16 tests. Do not introduce a second source of truth for these names.

§ 7(4) Update the **existing** implementation/governance entry by its stable ID and the relevant `governance/` fragment schema; do not duplicate entries in legacy `implementation-status.yaml`.

§ 7(5) Remove MD-084 from `missing-decisions.yaml` only after normative owners and governance agree. The owner's decision is already accepted; file creation alone is not integration.

## § 8. Acceptance and regression matrix

§ 8(1) Positive: parameter and direct-call-result `sec.reference` dictionaries parse, print, verify and round-trip; shared and mutable call actions match their reference operands; canonical origin forms `param#0`, `receiver`, `static@3`, `param#0.field#2`, `param#0[7]`, `param#0[*]` are accepted in semantically admissible contexts.

§ 8(2) Positive: `sec.address_taken = true` prevents premature P5 lowering; pinning and `AddressStability` independently survive eligible transformations; absent optional facts reconstruct exactly their documented defaults; omitted copy/move `source_borrow` follows the defined equality rule.

§ 8(3) Negative: typoed keys, unexpected result placement, wrong action, missing address-taken marker, malformed origin, duplicate origin, unsupported target address space, contradictory validity metadata, or conflicting legacy relocation metadata must be diagnosed.

§ 8(4) Determinism: identical Semantic IR and `CompilationPlan` produce identical metadata independent of map iteration and repeated runs, on 32- and 64-bit plans. Schema-11 regression inputs remain compatible with their established contracts.

§ 8(5) Schema 12: inherited metadata round-trips without new spellings; selecting schema 12 does not silently alter schema-11 default emission or fabricate not-yet-implemented slice operations.

## § 9. Final decision record

§ 9(1) **Owner decision accepted on 2026-10-21.** The seven discussed recommendations are approved: canonical names; strict origin strings; separate stability/pinning; fail-closed metadata preservation; `sec.address_taken = true`; SSA dynamic indices/no fabricated source names; unchanged schema-12 inheritance.

§ 9(2) Implementation status is **not** implied by this approval. This correction is ready for normative and governance integration, followed by targeted positive/negative verification.

---

## Application note (2026-10-22)

**Integrated into:**
- `rules/mlir/sec_mlir_dialect.md`: v11.7 amendment.
- `rules/mlir/sec_mlir_lowering.md`: v11.7 amendment.
- The dated amendment sections of `dialect-versions/sec_mlir_dialect_v11.md` and `lowering-versions/sec_mlir_lowering_v11.md`.
- `rules/mlir/packages/sec-mlir-dialect_package15.md`: §§ 62 and 67 amendments.
- `rules/compiler/compiler_pipeline.md` § 59(6), for schema selection.

**Implementation audit:** the emitter already used the canonical keys, `sec.address_taken = true`, the `shared-reference` / `mutable-reference` actions and SSA dynamic indices. It never emits `sec.relocation_class`, and it rejects target-defined address spaces.

**Changes made:**
1. `mlir/include/sec/Dialect/Sec/ReturnOrigins.h` (new) parses the § 4 grammar. The dialect attribute verifier (`SecDialect.cpp`) now rejects malformed and duplicate `sec.return_origins` entries; lit: `invalid-schema11-emitter-metadata.mlir`, with 7 new cases.
2. Compiler-materialized storage (MD-076 temporaries) no longer gets the fabricated `sec.source_name = "temporary"`. It now has an empty name plus `sec.synthesized = true` (§ 5(4); `temporary_borrows_emit_test.go`).

**Known hole (verified 2026-10-22):** type-aware admissibility of projections (§ 4(4)) is MD-085's verifier work.
