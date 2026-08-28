# SEC-MLIR Package 13 — completion TODO

Package: `SEC-MLIR-P13` — Struct Semantic Value Representation  
Governance: `lowering.sec-mlir-package13` in `implementation-status.yaml`  
Primary rulebook: `rules/mlir/packages/sec-mlir-dialect_package13.md`

## Inventory snapshot — 2026-08-27

The implemented vertical slice currently covers the Package 13 core:

- immutable Sema struct-literal and member-resolution facts;
- nominal Semantic IR struct definitions, tags, construction, spread, extract,
  replacement, high-level storage, and synthetic union payload structs;
- schema-9 `!sec.struct`, attributes, operations, C++ verification, and
  Sec-to-Core boundary handling;
- source-to-Semantic-IR and source-to-schema-9 regression tests.

Verified in this checkout:

- `go test ./internal/sema -run ResolvedStructPlans -count=1` — passed.
- `go test ./internal/ir/semantic -run Package13 -count=1` — passed.
- `SEC_MLIR_BIN=/home/jonas/small-projects/sec/build/sec-mlir/bin go test ./internal/lowering/secmlir -run Package13 -count=1` — passed.
- `cmake --build build/sec-mlir --target check-sec-mlir -j2` — passed, 78/78.

The command must use an **absolute** `SEC_MLIR_BIN`: Go runs package tests
from the package directory, so `SEC_MLIR_BIN=build/sec-mlir/bin` incorrectly
resolves below `internal/lowering/secmlir`.

The repository-wide acceptance blocker is closed: `go test ./...` and
`go vet ./...` pass with the absolute Package 13 MLIR tool path configured.
Package completion still requires the focused evidence and unsupported-boundary
tasks P13-28–53 below.

## Completion boundary

Do these items for P13 completion:

- prove every P13 section 74–83 requirement with focused coverage or an
  explicit mapping to an existing test;
- restore the repository-wide Go acceptance gate;
- run the complete P13 verification matrix with a portable tool invocation;
- update `lowering.sec-mlir-package13` from `partial` only after every
  acceptance command is green.

Do **not** pull these into P13:

- physical layout, offsets, ABI, MemRef/LLVM aggregate lowering;
- property operations, method dispatch, reflection, serialization, or FFI
  struct ABI;
- non-trivial move/borrow/destruction lowering. P17 owns the current
  ownership-aware action and replacement extension. P13 only needs a
  compatibility audit of its already emitted trivial subset.

## A. Make the P13 verification command reproducible

- [x] P13-01 — Change the documented/employed `SEC_MLIR_BIN` verification
  command to derive an absolute path from the repository root.
- [x] P13-02 — Add a small regression/helper test that demonstrates the
  Package 13 emitter verification uses the configured absolute tool path.
- [x] P13-03 — Re-run the four focused P13 commands and record their results
  in governance with the exact command spelling.

Completed 2026-08-27: `configuredSecMLIROptPath` rejects relative tool
directories, every real-tool emitter test uses the helper, and governance now
records `$PWD/build/sec-mlir/bin` for the package command.

## B. Close the repository-wide acceptance blockers

Each item below should begin with its named test alone, reduce to the smallest
Sec fixture, make the rule-backed fix, and then run the named test again.

### B1. Compiler/core-library blockers

- [x] P13-04 — Fix `TestCompilerAllowsCoreStringImpl`: allow the trusted core
  `impl string` path under the current ownership/type rules, without allowing
  arbitrary builtin impls.
- [x] P13-05 — Fix `TestCompilerLoadsCoreLibraryBeforeUserAnalysis`: restore
  resolved core instance member `value.IsEmpty`.
- [x] P13-06 — Fix `TestCompilerLoadsCoreRuneAndStringConversionAPI`: restore
  resolved core member `value.IsLetter`.
- [x] P13-07 — Reduce the `linux.Open` error-carrier mismatch to one function
  in `sec/platform/linux/file.sec` and correct `Err(LinuxError)` propagation.
- [x] P13-08 — Apply the same verified Linux error-carrier repair to `Read`,
  `Write`, `WriteStringFrom`, `Seek`, and `Flush`.
- [x] P13-09 — Apply it to directory/filesystem operations (`ReadDirectory`,
  `Access`, `CreateDirectory`, `RemoveFile`, `RemoveDirectory`, `Rename`,
  `Truncate`, and `Close`).
- [x] P13-10 — Fix the unreachable cleanup/owned-file diagnostic around
  `file.linux.amd64.sec:378–396` using the ownership rule, not a suppression.
- [x] P13-11 — Fix the corresponding directory cleanup/unreachable diagnostic
  around `file.linux.amd64.sec:797–815`.
- [x] P13-12 — Re-run the five failing `cmd/compiler` core/IO tests together.

Completed 2026-08-27: the builtin-impl acceptance test now uses the
compiler-selected canonical core root and a forged core-looking path has an
explicit negative regression. Core API tests use the current `IsEmpty` and
`IsLetter` property syntax. Linux syscall failures pass through one locally
handled checked enum conversion, mapping an undeclared errno conservatively to
`LinuxError.ioError`. File and directory helpers now capture the operation
result, close the owned resource exactly once on every path, and only then
select the returned result. All five named compiler tests and the complete
`cmd/compiler` package pass.

### B2. Sema baseline blockers

- [x] P13-13 — Reconcile `TestSymbolStorageOrigins` with the current rule that
  module bindings have static storage; update implementation or expectation
  only after checking `rules/declarations/static.md`.
- [x] P13-14 — Fix the first stale availability/discard diagnostic in
  `TestNamedIntegerRangeChecksConstantExpressions`.
- [x] P13-15 — Apply the same root-cause fix to
  `TestContractedVariableAssignmentRequiresTry` and
  `TestContractTypeRequiresExplicitConversionFromVariable`.
- [x] P13-16 — Restore named-unit resolution for the minimal `m` fixture in
  `TestStructTypeDeclarationRegistersNamedType`.
- [x] P13-17 — Restore named-unit resolution for the minimal `m/s` fixture;
  then re-run the property and Result tests that are currently masked by it.
- [x] P13-18 — Fix the mutable non-defaultable struct declaration diagnostic in
  `TestStructTypeDeclarationRegistersNamedType` after unit resolution is green.
- [x] P13-19 — Fix the bit-enum parser recovery failure in `TestBitEnumFixtures`.
- [x] P13-20 — Reconcile the move-only field-read diagnostic priority in
  `TestStructLiteralAndFieldAccess` and the property-access counterparts.
- [x] P13-21 — Restore the expected property getter/setter body diagnostics in
  `TestPropertyBodyChecks` after the unit blocker is removed.
- [x] P13-22 — Fix the borrow transition in
  `TestMovedMutableLocalCanBeReinitialized`.
- [x] P13-23 — Fix moved-value diagnostic precedence in
  `TestByValueFunctionArgumentMovesMoveOnlyLocal`.
- [x] P13-24 — Restore branch-scoped union-payload reference diagnostics in
  `TestAggregateContainingUnionPayloadReferenceCannotReturn`.
- [x] P13-25 — Restore caller-owned-field escape rejection in
  `TestStoreLocalReferenceIntoCallerOwnedFieldIsRejected`.
- [x] P13-26 — Restore interprocedural reference summaries in
  `TestInterproceduralProjectedReferenceSummary` and
  `TestParameterUsageDerivesSliceAndReferenceCallDemand`.
- [x] P13-27 — Re-run `go test ./internal/sema` and split any remaining
  failures into one new checklist item each; do not hide them by weakening
  assertions.

Completed 2026-08-27: module storage expectations follow static-v2; local try
handler bindings no longer leak discard/move state into later handlers; unit
fixtures declare their `m`/`s` identities without spelling-based implicit
units; Coordinate defaultability and the masked property/Result diagnostics are
restored; and the valid bit-enum fixture no longer contains invalid scratch
syntax. Reference-holder replacement now transitions borrows transactionally,
branch-scoped provenance has diagnostic priority, caller-owned projections
reject local-reference stores, and parameter-origin summaries survive field and
slice projection. The one additional P13-27 failure was the loop rebind test:
it now proves that legal holder replacement succeeds while the post-loop owner
still remains conservatively borrowed. The complete `internal/sema` package,
repository Go suite, Go vet suite, and 78-test MLIR suite pass.

## C. Complete the P13 Sema and Semantic-IR evidence matrix

These are test/audit tasks first. If a case is already covered elsewhere,
record the exact existing test and mark it complete rather than duplicating it.

- [x] P13-28 — Map section 74 type cases: empty, nested qualified identity,
  concrete generic identity, all active wide types, tags, optional `LayoutRef`,
  and property exclusion.
- [x] P13-29 — Add Semantic IR verifier tests for duplicate field ID, duplicate
  field name, and non-contiguous field ID rejection.
- [x] P13-30 — Prove `ResolvedStructLiteralPlan` is unchanged when legacy AST
  default materialization is enabled versus disabled.
- [x] P13-31 — Cover an empty/default-only literal, partial defaults, nested
  defaults, named constrained defaults, and a non-defaultable omitted field.
- [x] P13-32 — Cover multiple spreads and later-spread override, preserving
  source-entry order while final fields remain declaration ordered.
- [x] P13-33 — Cover explicit source evaluation order differing from field
  declaration order, including a spread source evaluated exactly once.
- [x] P13-34 — Cover all P13-supported construction actions
  (`construct-direct`, `copy-trivial`) and explicit rejection of move and
  semantic-copy in the P13 executable path.
- [x] P13-35 — Cover scalar, wide, nested, parameter, and return-value stored
  field reads, plus a property with identical syntax that must not become
  `StructExtractFieldOp`.
- [x] P13-36 — Cover defaulted and explicitly initialized mutable locals;
  assert RHS-before-load ordering, one root store for nested replacement, and
  only leaf-to-root `StructReplaceFieldOp` operations.
- [x] P13-37 — Cover synthetic payload identity stability by union TypeID plus
  variant index, wide fields, non-trivial payload rejection, and guard
  dominance for every unwrap.
- [x] P13-38 — Audit P17 compatibility: P13 records the current action
  vocabulary but emits only its verified trivial subset; add a regression that
  prevents accidental non-trivial P13 lowering.

Completed 2026-08-28: `TestPackage13BuildsEmptyAndConcreteGenericWideStructs`
maps section 74, including qualified nested-impl identities, all five active
wide scalars, tags, property exclusion, and optional `LayoutRef`.
`TestPackage13StructDefinitionVerifierRejectsFieldIdentityErrors` supplies the
three negative definition cases. The Sema fact tests now prove plan independence
from optional legacy AST default materialization, the complete default matrix,
non-defaultable omission, and multiple-spread precedence/order. Package 13
Semantic IR tests prove source evaluation order and single spread evaluation,
the trivial construction-action boundary against the current P17 vocabulary,
stored-field reads across scalar/wide/nested/parameter/return sources, property
separation, transactional mutable replacement, and stable guarded synthetic
union payloads including explicit rejection of a non-trivial payload type.

## D. Complete schema-9 MLIR evidence

- [x] P13-39 — Extend schema-9 round-trip coverage for empty, nested, generic,
  and wide `!sec.struct` values with ordered tags.
- [x] P13-40 — Add one invalid-MLIR case each for bad field ordinal, operand
  type, construct origin/action count, spread result count/type, extract
  action/type, and replace-field source/result/replacement mismatch.
- [x] P13-41 — Add a schema-8 regression assertion next to schema-9 tests so
  the compatibility promise is continuously explicit.
- [x] P13-42 — Cover P6 scalar resolution inside nested structs on 32-bit and
  64-bit targets while preserving identities, names, tags, and field ordinals.
- [x] P13-43 — Cover the P8 boundary: checked-integer signless normalization
  must not recurse into `!sec.struct` or lower struct operations.
- [x] P13-44 — Assert `--sec-lower-trivial-core` leaves
  `!sec.storage<!sec.struct<...>>` high-level.

Completed 2026-08-27: the schema-9 dialect tests now preserve two ordered
field tags, exercise every listed verifier mismatch, invoke the schema-8
round-trip beside schema 9, and cover nested target-sized fields on both
32-bit and 64-bit data layouts. The checked-integer and trivial-core checks
prove that struct wrappers and struct storage remain high-level.

Revalidated 2026-08-28: the round-trip now asserts the concrete generic
identity and wide type arguments directly. The 32-bit P6/P8 regression executes
real nested `sec.struct.extract` and `sec.struct.replace_field` operations and
proves that checked-integer lowering preserves both those operations and their
nested signed field type. Its trivial-core branch now asserts declare, init,
and load individually and rejects struct storage conversion to `memref`.

## E. End-to-end unsupported-boundary evidence

- [x] P13-45 — Add source-to-Semantic-IR rejection tests for move-only,
  semantic-copy, borrowed/reference, and resource-owning struct value paths.
- [x] P13-46 — Add explicit rejection tests for field borrow/ref/ref-mut,
  partial move, non-trivial replacement, property read/write, struct equality,
  method receiver lowering, and foreign struct ABI.
- [x] P13-47 — Assert rejected cases produce `UnsupportedFeatureError` (or the
  source-level diagnostic where required) and never placeholder/partial IR.
- [x] P13-48 — Assert every successful P13 construction has no `undef`, poison,
  hidden allocation, physical offset, or LLVM aggregate operation.

Completed 2026-08-28: `TestPackage13RejectsUnsupportedStructValuePathsWithoutPartialIR`
drives move-only reads and partial moves, shared and mutable references, owning
dynamic arrays, field borrows, non-trivial replacement, properties, equality,
method receivers, custom-free syntax, and foreign-struct syntax through their
current canonical parser, Sema, or Package 13 Semantic IR boundary. Every IR
rejection is a package-tagged `UnsupportedFeatureError` and returns no partial
module. Semantic-copy has no source-producible type classification yet; its
current P17-compatible fact is covered by
`TestPackage13AcceptsOnlyTrivialStructConstructionActions`, which forces the
compiler-owned action through the executable literal builder and proves P13
rejects it. `TestPackage13SuccessfulStructPathHasNoPlaceholderOrPhysicalLowering`
executes construction, spread, and field read while excluding placeholder,
allocation, physical-layout, and LLVM aggregate vocabulary.

## F. Final acceptance and governance

- [x] P13-49 — Run `go test ./...` from a clean checkout; it must pass.
- [x] P13-50 — Run `go vet ./...` and the full `check-sec-mlir` target.
- [x] P13-51 — Run the package-specific source → verified Semantic IR →
  schema-9 Sec MLIR test with an absolute tool path on both 32-bit and 64-bit
  scalar plans where applicable.
- [x] P13-52 — Update `implementation-status.yaml` with exact final commands,
  results, audited commit, and status `implemented` only when P13 section 90
  is fully satisfied.
- [x] P13-53 — Write the required P13 implementation report (rulebook section
  91), including explicit deviations and the boundary to P14/P15/P17.

Completed 2026-08-28 against repository HEAD
`e3d17698d756678cfb94bd9f8df1f11977635375` plus the exact pending Package 13
diff. A fresh detached worktree without inherited build artifacts passed
`go test ./...`; `go vet ./...`, the full absolute-path
`SEC_MLIR_BIN=$PWD/build/sec-mlir/bin go test ./...`, and
`cmake --build build/sec-mlir --target check-sec-mlir -j2` also passed. The
package-specific end-to-end test executes both 32- and 64-bit target plans and
verifies target-sized struct fields after scalar lowering. The required report
is `rules/mlir/packages/sec-mlir-dialect_package13-implementation-report.md`.

## Recommended execution order

1. P13-01 through P13-03 — make verification reproducible.
2. P13-04 through P13-27 — restore the mandatory whole-repository gate.
3. P13-28 through P13-38 — close or explicitly map the Sema/Semantic-IR matrix.
4. P13-39 through P13-44 — close schema-9/P5/P6/P8 evidence.
5. P13-45 through P13-48 — prove unsupported boundaries reject safely.
6. P13-49 through P13-53 — final acceptance, governance, and report.
