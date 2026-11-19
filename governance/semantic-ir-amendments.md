# Semantic IR amendments and package maintenance

Owner: `governance.semantic-ir-v2` in [governance.yaml](governance.yaml).
Inventory reviewed: 2026-11-19.

[Semantic IR](../rules/compiler/semantic_ir.md) is the canonical semantic
rulebook. Its [authority](../rules/compiler/semantic_ir.md#1-purpose-and-authority),
[lowering boundary](../rules/compiler/semantic_ir.md#102-lowering-boundary) and
[Sec MLIR relationship](../rules/compiler/semantic_ir.md#103-sec-mlir-relationship)
separate validated meaning from a package's implementation inventory.

This is a responsibility crosswalk, not a new semantic specification or a claim
that any package passes acceptance. Retained amendments keep their original
text, section numbers and declared status. Proposed amendments remain proposed;
historical assumptions do not override later canonical rules or corrections.
An installed specification is not evidence of executable lowering. Consult the
owning fragment for current implementation, dependencies and open decisions.

## Complete amendment inventory

Every Markdown amendment in `rules/mlir/semantic-ir/` is listed once below.
Local section ranges refer to that retained amendment, not to the canonical
book's paragraph numbers.

For every row, verifier requirements map to canonical
[§§ 96–97](../rules/compiler/semantic_ir.md#96-semantic-verifier), deterministic
printing to [§§ 107–108](../rules/compiler/semantic_ir.md#107-determinism), and
test obligations to [§§ 112–119](../rules/compiler/semantic_ir.md#112-required-test-families-core).
These common mappings apply in addition to the topic-specific ones below.

| Package and retained amendment | Local sections mapped to canonical responsibilities | Live implementation owner |
| --- | --- | --- |
| P11 — [enum/union values](../rules/mlir/semantic-ir/sec_semantic_ir_enum_union_v1.md) | §§ 1–7 → [§§ 7, 22](../rules/compiler/semantic_ir.md#22-enums); §§ 8–14 → [§ 23](../rules/compiler/semantic_ir.md#23-tagged-unions); §§ 15–16 → §§ 32–42, 104; §§ 17–19 → [§§ 24–25](../rules/compiler/semantic_ir.md#24-result-and-option), 58; §§ 20–21 → common verifier/printer mappings. | `lowering.sec-mlir-enum-union-values` in [lowering.yaml](lowering.yaml) |
| P12 — [match CFG](../rules/mlir/semantic-ir/sec_semantic_ir_match_v1.md) | §§ 1–6, 11–15 → [§§ 69–71](../rules/compiler/semantic_ir.md#69-match), 93–95; §§ 7–10 → §§ 22–24; §§ 16–19 → §§ 32–43, 96–97; § 20 → § 108. | `lowering.sec-mlir-match-cfg` in [lowering.yaml](lowering.yaml) |
| P13 — [struct values](../rules/mlir/semantic-ir/sec_semantic_ir_struct_v1.md) | §§ 1–4 → [§§ 7, 13](../rules/compiler/semantic_ir.md#13-struct-definitions); §§ 5–10 → [§ 14](../rules/compiler/semantic_ir.md#14-struct-construction-plans), 27, 29; §§ 11–14 → [§ 15](../rules/compiler/semantic_ir.md#15-struct-operations), 10, 31, 40; §§ 15–19 → §§ 32–43, 69; §§ 20–22 → § 104 and common verifier/printer mappings. | `lowering.sec-mlir-package13` in [lowering.yaml](lowering.yaml) |
| P14 — [fixed arrays](../rules/mlir/semantic-ir/sec_semantic_ir_fixed_array_v1.md) | §§ 1–9, 20, 22–24 → [§ 16](../rules/compiler/semantic_ir.md#16-arrays-slices-and-ranges), 7, 29, 104; §§ 10–17 → §§ 16, 54–55, 71; §§ 18–19 → §§ 10, 30–31, 45; § 21 → §§ 32–43; §§ 25–26 → common verifier/printer mappings. | `lowering.sec-mlir-package14` in [lowering.yaml](lowering.yaml) |
| P15 — [Places and references](../rules/mlir/semantic-ir/sec_semantic_ir_place_reference_v1.md) | §§ 1–8 → [§ 10](../rules/compiler/semantic_ir.md#10-places-and-semantic-locations), 30, 45–48; §§ 9–24 → [§§ 45–48](../rules/compiler/semantic_ir.md#45-references), 54–55; §§ 25–28 → §§ 6, 45–48, 75, 105; §§ 29–30 → §§ 48, 104; §§ 31–32 → common verifier/printer mappings. | `lowering.sec-mlir-package15` in [lowering.yaml](lowering.yaml) |
| P16 — [slice values](../rules/mlir/semantic-ir/sec_semantic_ir_slice_v1.md) | §§ 1–26 → [§§ 126.1–126.26](../rules/compiler/semantic_ir.md#126-slice-semantic-values-sec-mlir-p16), respectively. The installation notice and its linked correction supersede the retained length/pin/version assumptions; also see §§ 16, 45–48. | `lowering.sec-mlir-package16` in [lowering.yaml](lowering.yaml) |
| P17 — [ownership and destruction](../rules/mlir/semantic-ir/sec_semantic_ir_ownership_destruction_v1.md) | §§ 1–12 → [§§ 32–41](../rules/compiler/semantic_ir.md#32-ownership-classification); §§ 13–19 → [§§ 39, 42](../rules/compiler/semantic_ir.md#42-destruction); §§ 20–27 → [§§ 43–44](../rules/compiler/semantic_ir.md#43-cleanup-plans), 55; §§ 28–30 → §§ 33, 38, 60–61, 73; §§ 31–33 → §§ 96–99, 108. | `lowering.sec-mlir-package17` in [lowering.yaml](lowering.yaml) |
| P18 — [owning dynamic arrays](../rules/mlir/semantic-ir/sec_semantic_ir_dynamic_array_v1.md) | §§ 1–22 → [§§ 127.1–127.22](../rules/compiler/semantic_ir.md#127-owning-dynamic-arrays-sec-mlir-p18), respectively; general storage, ownership, cleanup and allocation responsibilities remain in §§ 30–51. Installation is recorded in the P18 report, independently of remaining runtime acceptance. | `lowering.sec-mlir-package18` in [lowering.yaml](lowering.yaml) |
| P19 — [Arena and allocation context](../rules/mlir/semantic-ir/sec_semantic_ir_arena_v19.md) | §§ 1–7 → [§§ 49–51](../rules/compiler/semantic_ir.md#49-allocation-operations); §§ 8–9 → §§ 41, 49, 71; §§ 10–14 → §§ 42, 48, 50–51; §§ 15–22 → §§ 49, 52–53, 83, 86; §§ 23–25 → §§ 30–31, 45–48, 127; §§ 26–27 → common verifier/printer mappings. | `lowering.sec-mlir-package19` in [lowering.yaml](lowering.yaml), allocation owners in [allocation.yaml](allocation.yaml) |
| P20 — [binary floating point](../rules/mlir/semantic-ir/sec_semantic_ir_float_v1.md) | §§ 1–4, 10–11 → [§ 58](../rules/compiler/semantic_ir.md#58-arithmetic), 7–8, 11–12; §§ 5–9 → [§ 59](../rules/compiler/semantic_ir.md#59-conversions), 54, 71, 124; §§ 12–13 → §§ 52–53, 96. | `lowering.sec-mlir-package20` in [lowering.yaml](lowering.yaml) |
| P21 — [decimal operations](../rules/mlir/semantic-ir/sec_semantic_ir_decimal_package21.md) | §§ 1–5, 8 → [§ 58, including (6)–(7)](../rules/compiler/semantic_ir.md#58-arithmetic), 8, 11; §§ 6–7 → §§ 54, 58–59, 71–72; §§ 9–10 → common verifier/printer mappings. Preserve the amendment's proposed status and its explicit MD-066/067/068 synchronization. | `lowering.sec-mlir-package21` in [lowering.yaml](lowering.yaml) |
| P22 — [named contracts](../rules/mlir/semantic-ir/sec_semantic_ir_named_contracts_package22.md) | §§ 1–5, 7 → [§§ 54, 59](../rules/compiler/semantic_ir.md#54-runtime-checks), 7–8, 14, 29, 40–41; § 6 → §§ 25, 54, 71; § 8 → § 12; §§ 9–12 → §§ 96, 102–104, 108. Preserve proposed status; this crosswalk does not activate regex or other unresolved contracts. | `frontend.type-contracts-v2` in [types.yaml](types.yaml) |
| P23 — [strings and UTF-8](../rules/mlir/semantic-ir/sec_semantic_ir_string_package23.md) | §§ 1–9 → [§ 17](../rules/compiler/semantic_ir.md#17-strings), 29, 45; §§ 10–15 → §§ 17, 27, 49, 52, 71; §§ 16–17 → common verifier/printer mappings. Later string/range and materialization corrections still apply; an old package proposal is not a new source API decision. | `frontend.types-core` in [types.yaml](types.yaml), materialization owners in [core.yaml](core.yaml) and [analysis.yaml](analysis.yaml) |
| P24 — [resolved layout](../rules/mlir/semantic-ir/sec_semantic_ir_layout_package24.md) | §§ 1–6 → [§ 104](../rules/compiler/semantic_ir.md#104-layout-boundary), 7–8; §§ 7–8 → §§ 32, 54, 104; §§ 9–10 → §§ 87, 96, 108. Layout is distinct from callable ABI. | `semantic-ir.layout`, `lowering.layout` in [memory.yaml](memory.yaml) |
| P25 — [callable ABI intent](../rules/mlir/semantic-ir/sec_semantic_ir_callable_abi_package25.md) | §§ 1–4 → [§§ 60–63](../rules/compiler/semantic_ir.md#60-function-representation), 6–8, 87, 104; §§ 5–8 → §§ 36, 41–43, 49, 60–61, 73, 87; § 9 → § 96. Physical call classification does not redefine semantic parameter ownership. | `lowering.abi-model` in [lowering.yaml](lowering.yaml), `frontend.ffi-v2` in [ffi.yaml](ffi.yaml) |
| P26 — [physical references](../rules/mlir/semantic-ir/sec_semantic_ir_reference_physical_package26.md) | §§ 1–6 → [§§ 45–48](../rules/compiler/semantic_ir.md#45-references), 10, 30–31, 104; § 7 → §§ 60–61, 87; § 8 → common verifier/printer mappings. Physical descriptor plans must preserve the source reference facts. | `lowering.reference-model` in [memory.yaml](memory.yaml) |
| P27 — [LLVM handoff](../rules/mlir/semantic-ir/sec_semantic_ir_llvm_handoff_package27.md) | §§ 1–6 → [§§ 96–103](../rules/compiler/semantic_ir.md#96-semantic-verifier), 1–2, 119. Readiness and capability provenance are lowering boundaries, not additional source semantics or a second backend. | `lowering.compiler-pipeline` in [lowering.yaml](lowering.yaml) |

The P24/P25/P23 files retain proposed installation-path metadata that differs
from their current directory. Links above name the actual installed files;
this navigation does not rewrite their historical headers or decide open
representation/ABI/provider questions.

## Historical reports and acceptance evidence

The complete retained report/matrix set under `rules/mlir/packages/` is:

| Evidence | Canonical Semantic IR responsibilities |
| --- | --- |
| [P13 implementation report](../rules/mlir/packages/sec-mlir-dialect_package13-implementation-report.md) | §§ 13–15, 29, 104 |
| [P14 acceptance matrix](../rules/mlir/packages/sec-mlir-dialect_package14-acceptance-matrix.md) and [implementation report](../rules/mlir/packages/sec-mlir-dialect_package14-implementation-report.md) | §§ 16, 29, 54, 104 |
| [P15 acceptance matrix](../rules/mlir/packages/sec-mlir-dialect_package15-acceptance.md) and [implementation report](../rules/mlir/packages/sec-mlir-dialect_package15-implementation-report.md) | §§ 10, 30–31, 45–48 |
| [P16 implementation report](../rules/mlir/packages/sec-mlir-dialect_package16-implementation-report.md) | § 126 |
| [P17 implementation report](../rules/mlir/packages/sec-mlir-dialect_package17-implementation-report.md) | §§ 32–44 |
| [P18 implementation report](../rules/mlir/packages/sec-mlir-dialect_package18-implementation-report.md) | § 127 |

Each report has an additive navigation note linking current canonical sections
and the live ledger. Its dated results, scope, unresolved gates, old paths and
historical completion claims are preserved. A historical reference to the
retired root ledger is evidence history, not an instruction to recreate it.
P1–P14 architecture milestones are also indexed in
[analysis-ownership.md](analysis-ownership.md).

## Workflow for every new or updated package

1. **Locate the semantic owner.** Cite existing canonical Semantic IR sections
   and specialized source rulebooks/corrections. Add the amendment to the table
   above if there is one. A package's schema or milestone number is not a new
   semantic category.
2. **Separate rule changes from implementation work.** For a genuine semantic
   change, update the responsible canonical section through the correction
   workflow and synchronize the rulebook status index. For implementation-only
   progress, update the owning `governance/*.yaml` integration and package work
   list/report. Do not append opcode inventories, package gates, test counts or
   implementation-status fragments to the canonical semantic book.
3. **Retain one live status owner.** Reuse the domain integration named above;
   do not create duplicate checklists in this crosswalk. A new integration
   requires the usual primary-owner choice. Record unsupported and blocked
   stages explicitly instead of promoting a complete pipeline from frontend
   acceptance alone.
4. **Keep evidence historical.** Extend or add a dated report, retaining older
   results and acceptance boundaries. Give it direct links to the responsible
   canonical sections and this map. Do not rewrite an old completion claim to
   imply that later language forms or physical lowering were already covered.
5. **Review executable compatibility.** Follow
   [SCHEMA.md](../internal/ir/semantic/SCHEMA.md) for the independently owned
   Semantic IR version. Review dialect emission/version and predecessor gates
   separately. A package completion does not bump either version automatically.
6. **Validate before claiming completion.** Check the new/changed links and
   section references, amendment/report inventory coverage and unique live
   owners; preserve historical bodies when adding navigation. Run the affected
   implementation tests and ledger checks. Update governance with the actual
   results and remaining boundaries, not merely the existence of documents.

This workflow closes governance navigation and maintenance work. It does not
close open semantic decisions, implementation or runtime acceptance criteria.
