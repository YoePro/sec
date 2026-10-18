# SEC-MLIR Package 22 — implementation TODO

Package: `SEC-MLIR-P22` — Named-Type Contracts and Runtime Validation Lowering  
Primary rulebook: `rules/mlir/packages/sec-mlir-dialect_package22.md`  
Package contract: `rules/mlir/packages/sec-mlir-dialect_package22.yaml`

Normative inputs:

- `rules/mlir/normative-sync/sec_named_contracts_sync_package22.md`
- `rules/mlir/normative-sync/sec_p20_p21_conversion_compatibility_package22.md`
- `rules/mlir/semantic-ir/sec_semantic_ir_named_contracts_package22.md`
- `rules/mlir/dialect-versions/sec_mlir_dialect_v18.md`
- `rules/mlir/lowering-versions/sec_mlir_lowering_v18.md`
- `rules/mlir/packages/sec-mlir-dialect_package22_decision-index.md`

This is an implementation plan, not a completion report or a new contract
language. It reuses the existing `frontend.type-contracts-v2` governance entry
and continues P15–P21 without claiming their unfinished work is complete.

## Inventory snapshot — 2026-10-18

Audited HEAD:

```text
831f6e1
```

The working tree has 137 changed entries. Its intended predecessor changes and
untracked files must be captured before P22 acceptance can be reproduced.

| Area | Current implementation | P22 consequence |
|---|---|---|
| Source contracts | Sema has named-type contracts, canonical CTE, inherited/effective contract handling, defaults, compile-time diagnostics and contract provenance. | Reuse these facts; P22 must expose immutable plans rather than a second evaluator. |
| Runtime source flow | Constrained conversions and `try` assignments are recognized in Sema, with existing failure-set and diagnostic behavior. | Preserve intrinsic-first and transactional semantics; this is not yet represented end-to-end in Semantic IR. |
| Contract families | Scalar, membership, length, string and unique-contract analyses already have focused Sema packages. | Implement runtime lowering only for families whose resolved predicates and underlying IR operations are genuinely available. |
| Semantic IR | Named scalar construction explicitly rejects named contracts/units; no P22 contract-check sequence or validated nominal construction exists. | P22 Semantic IR work remains real and must fail closed until implemented. |
| Core errors | `ContractKind`, `ContractError` and `ConversionError` are canonical core identities. | Use `ConversionError.Contract(ContractError.Violation { Kind, DeclarationIndex })`; do not create another error family. |
| String/collection dependencies | String contract plans and fixed-array facts exist; MD-050 is decided (every `string` is valid UTF-8), while several collection representations/lowerings remain gated. | Keep MD-010 and the P15–P18/P23 boundaries explicit per predicate family. |
| Schema/track | P22 targets schema 18 after P21 schema 17; Sec has one native lowering track. | Do not claim v18 or restore a legacy backend before executable predecessors exist. |

Primary implementation anchors:

```text
internal/sema/contracts.go
internal/sema/contract_cte.go
internal/sema/contract_values.go
internal/sema/length_contracts.go
internal/sema/stringcontract/plan.go
internal/sema/unique_contracts.go
internal/sema/analyzer_try_assignment.go
internal/ir/semantic/named_scalar.go
internal/ir/semantic/try_assignment.go
internal/lowering/secmlir/emitter.go
sec/core/error.sec
governance/types.yaml
```

## Authority and dependency constraints

1. **MD-012 is settled:** public checked conversion failure is core
   `ConversionError`; a declared-contract failure wraps
   `ContractError.Violation { Kind, DeclarationIndex }`. P22 must not create
   NumericConversionError, ContractCheckError, DecimalConversionError or a new
   Result wrapper.
2. **Contract scope is fixed:** contracts attach to named types, not variables
   or ad hoc fields. No `require`, implicit setter or new source syntax belongs
   to P22.
3. **Existing blockers remain external:** regex execution is MD-010; P21
   D1–D3 (MD-066–MD-068) remain P21-owned. MD-050 is settled (2026-10-17):
   every `string` is valid UTF-8, so string predicates never handle invalid
   UTF-8; their lowering waits on P23 string lowering, not on a decision.
4. **Predecessors are feature-specific:** P15/P17 gate place-sensitive
   transactional assignment; P16/P18 gate slice/dynamic-array predicates; P21
   gates decimal work; schema 17 must exist before v18 activation.
5. **One track only:** accepted work extends Sec MLIR → standard MLIR → LLVM.
   Historical references to legacy lowering do not authorize another backend.

## Completion boundary

P22 is complete only when a runtime conversion to a constrained named type
uses immutable Sema plans to perform exactly one underlying conversion,
intrinsic failure first, ordered inherited/local contract predicates, canonical
first-failure payload construction, and final nominal construction only on the
success path. The same representation must support guarded `try` flow and
transactional checked assignment where the required P15/P17 machinery exists.

P22 does not decide regex semantics, invalid UTF-8 behavior, decimal arithmetic
policy, broad collection iteration, physical named-type ABI/layout, a mandatory
runtime, or any new public error/source syntax.

## A. Establish baseline and synchronize authority

- [ ] P22-01 — Record the exact execution snapshot, untracked inventory,
  predecessor matrix, toolchain versions and `SEC_MLIR_BIN` setting.
- [ ] P22-02 — Run and record relevant predecessor Go, vet, dialect/lowering
  and native-track controls; separate pre-existing failures.
- [ ] P22-03 — Reconcile the package inputs with current contracts, types,
  defaults, CTE, errors, runtime checks, collections, Semantic IR, dialect and
  lowering rulebooks without renumbering canonical paragraphs.
- [ ] P22-04 — Confirm MD-010 and P21-D1–D3 (MD-066–MD-068) state in
  `missing-decisions.yaml`; record new genuine ambiguity there rather than in
  P22 code or plans.
- [ ] P22-05 — Update the existing `frontend.type-contracts-v2` governance
  entry in `governance/types.yaml` with P22's accurate partial/dependency
  status. Do not create a duplicate integration ID; validate via `yamlstatus`.

## B. Credit the existing frontend contract foundation

- [x] P22-06 — Resolve named-type contracts through canonical semantic CTE,
  preserve named identity, validate known defaults/values at compile time and
  produce existing source diagnostics for statically impossible values.

  Current anchors: `internal/sema/contracts.go`, `contract_cte.go`,
  `contract_values.go` and their focused tests.

- [x] P22-07 — Preserve effective inherited/local contract order and source
  provenance in Sema, including declaration-local ordinals and deterministic
  contract-conjunction diagnostics.

  Current anchors: `internal/sema/contract_conjunction_order_test.go` and
  `internal/sema/contractdiagnostics/locations_test.go`.

- [x] P22-08 — Resolve source-level scalar, membership, parity/divisibility,
  length, string and unique contract facts using established Sema ownership
  packages and semantic equality plans.

- [x] P22-09 — Recognize constrained conversion and checked `try` assignment
  in frontend analysis, including the requirement that a constrained
  destination is handled through the existing fallible-assignment path.

  Current anchors: `internal/sema/analyzer_try_assignment.go` and
  `internal/sema/try_failure_set.go`. This does not prove P22 IR/lowering
  transactionality.

- [x] P22-10 — Reuse the canonical core `ContractKind`, `ContractError` and
  `ConversionError` identities for source semantics and diagnostics; retain
  intrinsic conversion failure separately from declared-contract failure.

## C. Publish immutable resolved contract plans

- [ ] P22-11 — Add a detached read-only
  `ResolvedNamedContractConversionPlan` query keyed by conversion identity:
  nominal/underlying/source type IDs, intrinsic conversion plan, effective
  ordered contracts, origin TypeID, declaration-local index, CTE operands,
  equality/length plan, proof class, source location and ConversionError ID
  (§§9–13).
- [ ] P22-12 — Record an immutable `ContractPredicatePlan` per runtime check;
  reuse current CTE/equality/length analysis and never re-evaluate source AST,
  overload resolution or borrowing in builder/lowering (§§10, 14–17).
- [ ] P22-13 — Classify each predicate as proven true, proven false, runtime
  check or unavailable dependency. Permit proof elimination only where it
  cannot alter observable first-failure kind/index (§§12, 17).
- [ ] P22-14 — Establish a per-family readiness map: scalar range/parity/
  divisibility/finite/membership first; strings and fixed arrays only with their
  canonical plans; slices, dynamic arrays and unsupported equality/iteration
  families remain explicit capability boundaries (§§14–16).
- [ ] P22-15 — Preserve intrinsic target-domain conversion failure before every
  declared contract and construct the exact nested core payload only for the
  first failed declared check (§§11–13, 19).

## D. Semantic IR and control-flow representation

- [ ] P22-16 — Add the semantic records for contract predicate plans, ordered
  sequences, validated named construction and checked assignment commit without
  a generic opaque validator (§§23).
- [ ] P22-17 — Add `ContractCheckOp` from a resolved typed predicate and
  `ConstructNamedValidatedOp` only on an all-success-dominated edge; reject
  unchecked nominal construction for P22 paths (§§18, 23).
- [ ] P22-18 — Build intrinsic conversion CFG first, then ordered contract
  checks, canonical `ContractError.Violation`/`ConversionError.Contract`
  payloads and existing Result/try handler flow. Candidate nominal values must
  never be consumed on failure (§§18–19).
- [ ] P22-19 — Add `CheckedNamedAssignmentCommitPlan` that delegates actual
  copy/move/replacement/destruction to P15/P17 facts after total RHS success;
  reject unsupported paths rather than store-first lowering (§20).
- [ ] P22-20 — Materialize proven valid defaults as prevalidated nominal values
  and reject failed explicit defaults without fallback; gate aggregate work on
  implemented ownership/value packages (§21).

## E. Schema 18, emission and verification

- [ ] P22-21 — Install schema-18 only after executable schema-17 predecessor
  reconciliation. Reuse `!sec.named`, ordinary enum/union errors and Result;
  add only necessary contract operations/attributes (§24).
- [ ] P22-22 — Emit P22 operations from verified semantic plans only. Preserve
  predicate kind, origin TypeID, declaration index, plan ID, proof and source
  location as compiler metadata—not user-visible error fields (§§23–24).
- [ ] P22-23 — Add `--sec-verify-named-contracts` for typed inputs, origin/
  ordinal uniqueness, intrinsic-success dominance, branch polarity, exact core
  error identities, candidate dominance, nominal identity and early-commit
  rejection (§§26).
- [ ] P22-24 — Add deterministic parse/print and negative fixtures for unknown
  predicate kinds, missing plans, reordered checks, wrong payloads, regex
  success, unsupported collection acceptance and invalid validated construction
  (§§24, 26).

## F. Lower through the single native track

- [ ] P22-25 — Add `--sec-lower-named-contract-predicates` to lower scalar
  range, parity, multipleOf, finite and resolved membership through exact
  Arith/CF after all Sema proofs are present (§25).
- [ ] P22-26 — Lower string rune-count and byte-count predicates from the
  shared string-contract plan once P23 string lowering exists (MD-050 is
  settled: strings are always valid UTF-8). Preserve ordered
  independent byte/scalar checks and avoid descriptor-size/grapheme semantics
  (§§14–15, 25).
- [ ] P22-27 — Lower fixed-array/collection length and `unique` only when
  canonical iteration, equality, ownership and representation support is
  available; otherwise return structured unsupported-lowering (§16, 25).
- [ ] P22-28 — Preserve first-failure CFG and nominal validation through
  lowering, with no hidden allocation, panic, contract runtime table, ABI
  choice or physical error-layout inference (§§18–19, 22, 25).
- [ ] P22-29 — Check target/noalloc capability for each required predicate and
  maintain explicit unsupported boundaries for P15–P18/P21 dependencies.

## G. Required test matrix

- [ ] P22-30 — Test CTE operands, inherited/local order, origin/local ordinal,
  intrinsic-before-contract failure, proven checks and nominal identity from
  source through Semantic IR (§§9–14, 28).
- [ ] P22-31 — Test scalar range, membership, parity, multipleOf and finite
  predicates across target-sized and 8–256-bit integers plus selected floats/
  decimals, without host-width truncation (§14, 29).
- [ ] P22-32 — Test string logical scalar and UTF-8 byte lengths, embedded NUL,
  non-BMP, ordered byte/scalar failures and the P23 lowering boundary
  (§§14–15, 30).
- [ ] P22-33 — Test supported fixed-array length/unique checks, equality-plan
  behavior and explicit rejection for unsupported slice/dynamic families
  (§16, 30).
- [ ] P22-34 — Test Result/naked try/local handlers, exact nested error
  payloads, candidate failure isolation and transactional assignment once P15/
  P17 prerequisites are present (§§18–20, 31).
- [ ] P22-35 — Add Semantic IR/MLIR negative tests, deterministic print/parse,
  target 32/64/noalloc checks and differential native-track controls (§§32–33).
- [ ] P22-36 — Run focused Go/vet/dialect/lowering/lit suites and full
  repository checks; record exact commands/results in governance.

## H. Acceptance, governance and handoff

- [ ] P22-37 — Reconcile acceptance gate A with current source facts and do
  not count parsing/frontend diagnostics as runtime-lowering completion (§36).
- [ ] P22-38 — Enable dependent acceptance gate B only after actual v17,
  P15/P17 and family-specific predicate prerequisites are installed (§36).
- [ ] P22-39 — Synchronize governance, rulebook status, decision references,
  capability boundaries and implementation evidence without duplicate IDs.
- [ ] P22-40 — Produce the required implementation report with snapshot,
  prerequisites, plan/IR/schema/lowering behavior, tests, deviations and the
  next-package boundary (§37–40).

## Recommended execution order

1. A: capture the snapshot and reconcile current rulebooks/MD dependencies.
2. C: extract immutable Sema plans and complete scalar readiness first.
3. D: build intrinsic-first contract CFG and validated nominal construction.
4. E/F: after schema-17 and required P15/P17 work exists, install schema 18
   and widen the single native track one predicate family at a time.
5. G/H: keep every unsupported family explicit and promote status only with
   focused plus end-to-end evidence.
