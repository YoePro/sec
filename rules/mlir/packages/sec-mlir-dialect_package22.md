# Sec MLIR Program — Package 22: Named-Type Contracts and Runtime Validation Lowering

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec-mlir-dialect_package22.md`
- **Replaces:** None; additive package amendment only
- **Package:** `SEC-MLIR-P22`

---

## § 1. Mission

Implement the **already specified** Sec 0.1 named-type contract boundary from frontend Sema plans to canonical Semantic IR and high-level Sec MLIR, then to target-neutral checked predicates and Result control flow. Reuse current October language semantics, the existing `core::ConversionError` / `core::ContractError`, and the P10–P17 machinery when implemented. Do not introduce a second contract evaluator, a parallel error family, or a mandatory runtime.

This package targets **Sec MLIR schema v18 / lowering specification v18**, subject to the release of planned v17. It does not imply either version is currently implemented.

## § 2. Actual prerequisite status — October 16, 2026

| Package | User-confirmed status | P22 dependency rule |
|---|---|---|
| P1–P14 | Implemented | Reuse; regression required. |
| P15 | Implementation in progress | Place-sensitive assignment may depend on its completion. |
| P16–P17 | Corrected, current plans; implementation not confirmed | Slice origins, non-trivial assignment/cleanup stay gated until implemented. |
| P18–P21 | Specification/installation state is not verified as implemented | Do not claim their types, passes or schemas exist in executable code. |
| P21-D1–D3 | Awaiting user's MD-### synchronization/approval | No P22 inference or override. |

**Baseline:** Current public `main` was read on 2026-10-16. Exact HEAD SHA was not independently retrievable from the accessible GitHub page; Codex must record `git rev-parse HEAD` and fail preflight if source rules differ materially. Canonical files under `rules/types/`, `rules/errors/`, `rules/compiler/`, `rules/mlir/`, `sec/core/`, and `governance/` supersede August directory paths.

## § 3. Mandatory conflict correction — MD-012

MD-012 already resolved the conversion error system on 2026-10-08. The canonical **public** declarations are in `sec/core/error.sec`:

```text
core::ContractKind
core::ContractError
core::ConversionError
```

Runtime conversion into a named constrained type uses `ConversionError`; a failed declared contract is `ConversionError.Contract(ContractError.Violation { Kind, DeclarationIndex })`. For intrinsic destination-domain failures, use `ConversionError.OutOfRange`, `.InvalidScalar`, `.PrecisionLoss`, `.ScaleOverflow`, or `.NonFinite`, **as applicable**. Do not use obsolete/new `NumericConversionError`, `DecimalConversionError`, `ContractCheckError`, or a new `ConversionResult` family. Do not redeclare core members in compiler-owned metadata or other source files.

**Required cross-package compatibility:** P20's earlier proposed `NumericConversionError` is superseded by MD-012. P21's decimal conversion work must use `ConversionError` for conversion and its own *separately approved* arithmetic error policy for decimal arithmetic; do not confuse them. See `sec_p20_p21_conversion_compatibility_package22.md`. This is rulebook synchronization, **not** a new language decision.

## § 4. Normative authority and governance

Read in order:

```text
rules/types/contracts.md
rules/types/types.md
rules/types/default_values.md
rules/compiler/compile_time_evaluation.md
rules/foundations/operators.md
rules/collections/collections.md
rules/errors/runtime_checks.md
rules/errors/errorhandling.md
rules/corrections/applied/md012-md022-conversion-errors-ffi-literals-correction-20261008.md
rules/corrections/applied/string-length-contracts-correction-20261009.md
sec/core/error.sec
rules/compiler/semantic_ir.md
rules/mlir/sec_mlir.md
rules/mlir/sec_mlir_dialect.md
rules/mlir/sec_mlir_lowering.md
governance/types.yaml
```

Use `frontend.type-contracts-v2` and its existing stable governance IDs; this is a quality/completion package, not a second backlog index. Respect canonical headers and **do not renumber existing normative paragraphs** when synchronizing the implementation amendments.

## § 5. Reuse rather than replace existing code

Read and reuse at least:

```text
internal/sema/contracts.go
internal/sema/contract_values.go
internal/sema/contract_cte.go
internal/sema/nominal_membership.go
internal/sema/membership/value.go
internal/sema/length_contracts.go
internal/sema/stringcontract/plan.go
internal/sema/unique_contracts.go
internal/sema/collectionshape/unique.go
internal/sema/try_failure_set.go
internal/sema/type_references.go
internal/codegen/llvm/string_contracts.go
internal/codegen/mlir/string_length.go
internal/codegen/readiness/string_contracts.go
internal/ir/semantic/
internal/lowering/secmlir/
```

Existing Sema coverage includes range, integer divisibility, nominal membership, compile-time checks, length units and runtime valid-UTF-8 string length checking in legacy paths. **Do not rewrite established code just to match a P22 abstraction.** Expose immutable copies/IDs of resolved data.

## § 6. Contract inventory

Canonical supported contract spellings:

```text
range
in [...]
odd
even
multipleOf
finite
regex
minLen
maxLen
exactLen
notEmpty
minByteLen
maxByteLen
exactByteLen
unique
```

`regex` is syntax-accepted but runtime/CTE semantic validation remains **blocked by existing MD-010**, not P22-D1. The dedicated regex rulebook must precede matching lowering. No placeholder regex engine, POSIX/libc fallback, or acceptance without a check.

## § 7. Named-type identity

Contracts belong to named types, never variables or ad hoc struct fields. Preserve `TypeID`/nominal identity of the target separately from the underlying value representation. A checked construction yields a named value *only after* all intrinsic and declared checks pass. An existing proven-valid constrained value carries proof facts but does not become an alias for the primitive base type. Named ancestry, concrete generic substitutions, and units remain typed.

## § 8. No new source surface

Accepted sources use existing declarations/conversions:

```sec
type Percent int range 0..100
type NonEmptyName string notEmpty

fn Convert(raw: int) Result[Percent, ConversionError] {
    return Ok(try Percent(raw))
}
```

Runtime assignment to constrained named storage uses the existing checked `try` form. Do not introduce `require`, variable contracts, synthesized implicit setters, or user-defined operator declarations. Compile-time-known invalid values remain source errors, not guaranteed-failing generated branches.

## § 9. Resolve once: CTE vs runtime

Contract arguments and `default` expressions requiring compile-time values use **canonical semantic CTE**, not a local contract interpreter. Preserve exact arbitrary-precision integer, decimal, string and nominal constant values. If static CTE cannot establish a required bound/member/default, report the existing compile-time diagnostic. A *runtime source value* may still require runtime checking; unknown runtime values are not treated as proved.

## § 10. Read-only resolved plan

Add/adapt an immutable plan `ResolvedNamedContractConversionPlan` from Sema, queried by resolved conversion expression identity. Required fields:

```text
TargetNominalTypeID
UnderlyingTargetTypeID
SourceTypeID
ExplicitConversionRelationID
IntrinsicConversionPlanID
IntrinsicFailureMap
OrderedEffectiveContracts[]
    ContractKind
    DeclaringTypeID
    DeclarationIndex (zero-based within declaring type)
    InheritanceDepth
    CanonicalCTEOperand(s)
    ComparableEqualityPlanID, if required
    ProofClass (proven, runtime, impossible)
    SourceLocation
DefaultMaterializationPlanID, where relevant
ResultNominalTypeID
ErrorTypeID = core::ConversionError
CheckedAssignmentDestinationPlaceID, when applicable
```

`DeclaringTypeID` and source provenance are **compile-time metadata only**, not new fields in `ContractError.Violation`.

Queries must not mutate Analyzer state, re-evaluate CTE, or rerun overload/equality/borrowing resolution. Reuse existing analysis structures if they already carry facts; do not require duplicate case enums or IDs.

## § 11. Canonical conversion order

For a checked runtime conversion:

```text
1. Prove a declared conversion relation exists (compile time).
2. Evaluate source expression once.
3. Perform underlying target conversion; stop on intrinsic failure.
4. Evaluate inherited and local declared contracts in canonical effective order.
5. Stop on first violated contract, preserving its Kind and DeclarationIndex.
6. Construct nominal typed result only after all checks succeed.
```

All checked outputs are modeled with existing P9–P11 checked CFG/Result flow. No exception/panic is used for an ordinary checked-conversion failure.

## § 12. Declaration and inherited order

Preserve the canonical ancestor-to-derived **effective order** already resolved by Sema. Each effective contract retains its origin declaration and its source-order index *inside that declaration*. Do not globally renumber on inheritance flattening. The first actual failed check wins; proven redundant checks may be erased only when this cannot change observable first-failure identity. Treat conflicting/unsatisfiable contracts as declaration-time errors where canonically provable.

## § 13. Intrinsic failure precedence

An intrinsic destination-domain error (e.g. `int8` overflow or forbidden numeric precision loss) precedes all named contracts. A finite named-type contract failure uses `ContractKind.Finite` inside `ConversionError.Contract`, **not** `ConversionError.NonFinite`. The latter belongs only to intrinsic destination-domain restrictions. `ConversionError` is an ordinary distinct union error with Result/try behavior already defined elsewhere.

## § 14. Per-contract predicate semantics

- `range`: typed lower/upper bounds with exact inclusive/exclusive comparison; use resolved numeric/unit comparisons; never reinterpret source literals from machine representation.
- `in [...]`: compare each CTE member using canonical semantic equality; preserve membership source order, no hashing-based change of identity; no duplicate entries accepted by Sema.
- `odd/even`: test low bit of exact integer domain; support signed/unsigned 8–256 and target-sized `int`/`uint`; do not use host-width truncation.
- `multipleOf`: nonzero CTE divisor; exact signed mathematical divisibility with sign-independent validity; guard target signed-min/-1 arithmetic if lowering `%` would overflow.
- `finite`: correctly classify binary32/binary64/platform-width `float`; decimal without NaN/infinity is trivially finite if canonical representation guarantees it.
- `minLen/maxLen/exactLen/notEmpty`: use **logical elements** for collections and **Unicode scalar count** for strings; no normalization/grapheme counting.
- `minByteLen/maxByteLen/exactByteLen`: strings only; compare UTF-8 encoded payload bytes from value-form `SizeOf`, not descriptor size or `.Len`; no terminator.
- `unique`: compare direct collection elements with canonical resolved equality, not recursive collection uniqueness; preserve source element order and deterministic first-failure category.
- `regex`: explicit gate MD-010; no semantic success claim.

## § 15. Strings and UTF-8 blocker

Use the existing `stringcontract` shared immutable plan and legacy native/MLIR implementation facts for valid UTF-8. String `.Len` counts decoded Unicode scalar values; value-form `SizeOf` counts UTF-8 bytes; neither `.RuneLen` nor `.ByteLen` should be reintroduced as a public member. **MD-050** remains an existing unresolved invalid-UTF-8 policy according to governance; where semantics depend on it, keep an explicit capability block rather than inventing replacement policy. In a valid-string path, byte and scalar predicates are independent conjuncts in source order.

## § 16. Collections and prerequisites

For fixed arrays (P14), dynamic owning arrays (planned P18), slices (planned P16), strings and other supported collection shapes, use already-resolved element-count and equality plans. Implement only those paths whose Semantic IR representation and element-equality lowering exist. A source type being legal does **not** authorize an incomplete backend to emit a successful unchecked conversion. Keep a per-family readiness/capability map and precise `UnsupportedFeatureError`/diagnostic for deferred new-IR cases. Do not substitute list/set/map iteration by raw memory layout.

## § 17. Contract proof and elimination

Proof classes are:

```text
ProvenTrue
ProvenFalse
RuntimeCheck
UnavailableDependency
```

`ProvenFalse` on a required compile-time source is a declaration/conversion diagnostic. Dynamic conditions must be checked. `ProvenTrue` elimination retains Sema provenance and must preserve first-failure behavior. `UnavailableDependency` is a compile-time capability/implementation gap, not automatic pass/fail.

## § 18. Underlying + contract CFG

Represent the intrinsic conversion as its own checked operation/plan. Only on success evaluate contract predicates, exactly once for the value. Each predicate's failure branches to error construction; success continues to next predicate. Construct `!sec.named<...>` (reuse existing named type schema) at the final success edge. Failed edges cannot use a nominal candidate. The emitter must never silently re-run contract CTE or create a user-visible intermediate nominal value.

## § 19. Error construction

For contract `i`:

```text
ContractKind = resolved core enum variant
DeclarationIndex = resolved uint constant i
ContractError = ContractError.Violation { Kind, DeclarationIndex }
ConversionError = ConversionError.Contract(ContractError)
```

Use existing P11 enum/union/Result operations and nominal identities; **no parallel `core_error` encoding** when the newer normal enum/union representation is available. Preserve the complete payload through naked try and local handlers. Source diagnostics use location provenance, not a runtime contract-name pointer.

## § 20. Checked assignment commit

A destination constrained place remains unchanged until the entire RHS underlying conversion **and all contract checks** have succeeded. Only then run P17 replacement/init transfer as appropriate. On failure, no old destination destructor runs, no cleanup registration is altered, and no partial value is exposed. RHS evaluation and copy/move actions follow their existing Sema plans. For P15/P17-not-yet-implemented place/replacement paths, leave explicit gated work; no fake store-first lowering.

## § 21. Defaults

Canonical declaration default determination stays in `rules/types/default_values.md`. Explicit defaults and implicit selections require complete static proof and materializability; a valid default is a **prevalidated nominal value**, not an unchecked dynamic constructor. A failed explicit default never falls back silently to a synthesized default. Support aggregate-field defaults when P13–P14 and relevant ownership semantics are present.

## § 22. Runtime check effects

A fallible contract conversion yields typed Result control flow, not `panic.bounds` or `panic.contract`. `@noPanic` is compatible if all operand and predicate dependencies are no-panic. Do not introduce hidden allocation to check scalar contracts. Runtime `unique`/text equality or other type-specific equality may carry already declared comparison effects; do not suppress them or invent a pure comparison when canonical equality is unavailable. CompilationPlan can reject unsupported effects in restricted/noalloc profiles.

## § 23. Semantic IR v1 amendment

Keep Semantic IR version `1` unless its canonical schema governance explicitly requires a bump. Add/reuse:

```text
ResolvedNamedContractConversionPlan record
ContractPredicatePlan record
ContractCheckOp (result: bool; contract origin/ordinal)
ContractCheckSequence record (ordered runtime/proven segments)
ConstructNamedValidatedOp (only on all-success path)
ContractViolationPayloadOp (Kind, DeclarationIndex)
CheckedNamedAssignmentCommitPlan
```

Avoid a gigantic opaque `ValidateAnything` operation. Scalar `range`, parity, finite and equality check expansion can use existing canonical IR primitives after Sema has resolved the predicate; sequence metadata remains explicit.

## § 24. Sec MLIR schema v18

Re-use existing `!sec.named` and P11 ordinary enum/union types. Proposed **minimal additive** v18 operations (verify actual schema and add only missing):

```text
sec.contract.check              : value -> i1
sec.contract.violation          : ContractKind, uint -> ContractError
sec.contract.validated_named    : underlying T -> nominal T
```

Required attrs on `sec.contract.check`:

```text
sec.contract_kind
sec.contract_origin_type
sec.contract_declaration_index
sec.contract_predicate_plan_id
sec.contract_proof
sec.contract_source_location
```

The operation carries a typed *resolved* predicate plan (bounds, members, equality/sizing semantics) or references immutable plan metadata; it is not an interpreter of untyped source syntax.

`sec.contract.violation` may canonicalize directly to ordinary `sec.union.construct` and `sec.enum.constant` if that is the existing dialect pattern. Do not introduce a duplicate op merely for API symmetry; the authoritative goal is the invariant, not the spelling.

## § 25. Lowering v18

Add two separated passes, with names adjusted only to existing registration conventions:

```text
--sec-verify-named-contracts
--sec-lower-named-contract-predicates
```

Predicate lowering: exact Arith/CF comparisons (including checked wide integer/decimal prerequisites); finite float classification from P20 if ready; UTF-8 logical-length/byte-size plan from existing string-lowering; collection iteration/equality only when canonical semantic iterators are representable. Preserve contract origin and failure ordinal through lowering. Do not choose an ABI or final LLVM string descriptor here.

## § 26. Verification

Register or extend `--sec-verify-named-contracts` to reject:

```text
unchecked nominal creation
source contract on variable/field in new source form
missing or duplicate contract origin/ordinal
wrong core ContractKind/ConversionError identities
contract check before intrinsic success
reordered non-equivalent checks
failure edge entering nominal constructor
wrong value SSA fed to guard
invalid RangeError/ContractError substitution
out-of-range DeclarationIndex
unproven equality/length semantics
illegal regex success without MD-010 resolution
unknown/unsupported collection family silently accepted
checked assignment mutating old value on failure
```

## § 27. Independent delivery stages

Implement **one stage at a time**, test it, record evidence, then continue:

1. **Preflight:** exact SHA, governance registry, P1–P14 installed, P15–P21 actual status, `ConversionError` declaration check, no rewrites.
2. **Frontend plan extraction:** read-only resolved contract/CTE/order/origin/assignment facts; unit/generic inheritance checks.
3. **Scalar predicates:** range, integer parity/divisor, `finite`, scalar membership; source-to-Semantic-IR smoke tests.
4. **Failure channel:** intrinsic-first then contract, exact `ContractError.Violation`, Result/try handlers.
5. **String predicates:** runes/bytes/notEmpty and ordered errors against shared stringcontract package; note MD-050 gate.
6. **Collection predicates:** supported fixed arrays `len`/notEmpty and `unique` where equality/iteration is available; explicitly gate others.
7. **Assignment/default:** proven nominal defaults, transactional destination commit when P15/P17 primitives actually land.
8. **Dialect v18:** types/op verifier/print round-trips only after v17 baseline is installed.
9. **Lowering v18:** target-neutral checked CFG and capability diagnostics, no phantom backend.
10. **Governance/regression:** update existing stable IDs and collect exact tests/logs.

When a stage cannot execute due to a missing predecessor, mark it `blocked`, give exact missing ID, and continue only with genuinely independent stages. Do not mark package complete with blocked acceptance items.

## § 28. Source tests: types and CTE

Required positive: static bound expressions, immutable CTE data, static getter/ordinary function where CTE legal, exclusive/inclusive ranges, combined contracts, named ancestry, generic instantiation, explicit defaults, ordered finite sets and enum aliases, exact decimals and wide integers, units as typed. Required negative: illegal variable/field contract, runtime-bound argument, invalid default, unsatisfiable range + odd/even/LCM, duplicates, wrong membership family, missing required CTE, undeclared `require` contract.

## § 29. Runtime tests: scalar

For each of 32-bit and 64-bit CompilationPlan:

```text
int8 overflow before range failure
signed minimum multipleOf -1 without backend poison
range inclusive/exclusive boundaries
odd/even negative integers
uint256 contract range/multipleOf
inherited first-failure ordering
same declaration Kind/DeclarationIndex
finite float NaN and infinities
finite decimal and decimal128 only if P21 representation ready
runtime membership on enum aliases and scalar string values
no allocation/opaque runtime contract registry
```

## § 30. Runtime tests: string/collection

String positive/negative: ASCII, Unicode BMP, non-BMP, combining marks, embedded U+0000, empty, 2 scalars/4 UTF-8 bytes, inherited rune + byte constraints, source value evaluated once, exact first violated `Kind`/ordinal. Collection: fixed array length, zero-length array, nested aggregates for equality, `unique` on comparable direct elements, zero-width values, generic specialization, uncomparable element rejection, guarded slice and dynamic owner cases gated to actual P16/P18 implementation. Regex remains MD-010 blocked.

## § 31. Transaction and control-flow tests

```text
intrinsic error bypasses contract checks
contract failure prevents nominal materialization
checked assignment preserves old value on failure
checked assignment cleanup/destroy only after success
RHS evaluated exactly once
failure retains original nested contract payload
naked try to Result[T, ConversionError]
local specific/catch-all try handling
wrong enclosing error rejected
`@noPanic` proven/handled cases
P12 match on ConversionError.Contract and ContractKind
```

## § 32. MLIR negative and differential tests

Feed malformed schema-v18 MLIR intentionally: missing guard, wrong polarity, wrong ordinal, swapped Kind, mismatched input value, check applied before intrinsic conversion, fabricated source ID, `regex` falsely marked validated, `unique` without comparable element plan, use of candidate on failure, store before success, wrong core union/enum identity. Compare all permitted new-path outcomes to Sema/reference evaluator and **already supported legacy backend** on identical source. Distinguish expected unsupported from semantic disagreement.

## § 33. Target/regression/test commands

Run precise environment-appropriate variants of:

```text
go test ./internal/sema/... -count=1
go test ./internal/codegen/readiness/... -count=1
go test ./cmd/compiler/... -count=1
go test ./... -count=1
go vet ./...
cmake --build build/sec-mlir --target check-sec-mlir -j2
```

Run CLI and LSP diagnostics fixtures, 32/64 CompilationPlans, and at least one invalid-IR guard suite. Never report a test as passed unless executed. Preserve a build matrix listing unavailable MLIR dependencies explicitly.

## § 34. Governance discipline

Update **existing** `frontend.type-contracts-v2` in `governance/types.yaml` via the existing fragment sync mechanism. For every stable item record implemented/partial/remaining/identified_bugs, executable evidence, revision, code/test paths. Do not duplicate legacy fragments or invent unrelated IDs. Update `language-rulebook-status.md` only when authoritative backlog requires it; paused items remain visible. Existing MD-010 and MD-050 stay linked; do not copy them into new P22 decisions.

## § 35. Compatibility audit

Before code changes synchronize old P20/P21 proposed error language against MD-012. Record a **compatibility correction** for `NumericConversionError` in P20 and any P21 conversion plans referencing it; do not unilaterally change the still-unresolved P21 decimal arithmetic error decision. Preserve P20 float target-selected width (MD-014) and October length/SizeOf rules. This is a required audit, not authorization to overwrite previous package files wholesale.

## § 36. Acceptance gates

**Gate A — implementable without v17 runtime:**

```text
[ ] current repo SHA recorded
[ ] existing governance entries reused
[ ] read-only resolved named contract plan
[ ] effective inherited/local ordering and origin IDs
[ ] compiled CTE operands reused
[ ] public ConversionError / ContractError exact model
[ ] correct first-failure Kind and DeclarationIndex
[ ] intrinsic-first checked conversion boundary
[ ] scalar checker and compile-time provenance
[ ] no regex placeholder
[ ] no hidden allocation/side effects
[ ] focused Go tests run and recorded
```

**Gate B — after required local predecessors:**

```text
[ ] string predicates end-to-end and MD-050 boundary documented
[ ] permitted fixed-array/collection predicates complete
[ ] checked assignment transactional with P15/P17
[ ] schema v17 predecessor available
[ ] schema v18 ops round-trip/verify
[ ] v18 lowering preserving Result/try payloads
[ ] all applicable new-path family verifiers green
[ ] 32/64 target matrix and legacy differential tests green
[ ] check-sec-mlir and go test ./... actually pass
[ ] governance status synchronized
```

An item with missing predecessor remains **blocked**, never `done` or `partial-pass` unless its specified sub-scope genuinely passed.

## § 37. Required Codex implementation report

Report: SHA and date; predecessor matrix P1–P21; MD-012 compatibility check; current governance stable IDs; source files read and code added/modified; resolved plan API and read-only proof; conversion/error path; contract sequence and ordinal/origin semantics; scalar/string/collection supported matrix; regex/UTF8 blockers; exact checked assignment commit; Semantic IR ops; schema-v18 types/ops; lowering passes/verifiers; 32/64 tests, CLI/LSP and negative MLIR tests; test commands and actual outputs; failures and blockers; governance diff; follow-on recommendations. Do not claim P22 or the preceding planned packages implemented by delivery of these specifications.

## § 38. Explicit non-goals

No dedicated regex language/runtime implementation (MD-010). No invented `require` source contract. No unapproved user predicate contracts. No variable-level contracts. No new conversion error families. No parallel CTE engine. No mandatory GC/runtime/helper tables. No physical FFI ABI or LLVM contract format. No general iteration/owned-collection lowering missing from predecessors. No dynamic raw-pointer contract bypass.

## § 39. Existing and new decisions

**No new normative fork was necessary for this specification.** Existing MD-010 (regex) and MD-050 (invalid UTF-8) remain external dependencies; MD-012 (conversion errors) is already locked. P21-D1–D3 remain P21-specific decisions. Details and future-ID policy are in `sec-mlir-dialect_package22_decision-index.md`.

## § 40. Next-package boundary

P23 should be selected only after auditing then-current completed implementation and remaining compiler pipeline gaps. Reasonable candidates: compiler-known string/materialization or physical aggregate/reference/layout lowering; do not declare either preselected or implemented merely from P22.
