# Package 15 implementation report — 2026-10-25

P15-74 report in the exact section-156 topic order. **P15 remains partial.**
Completed mapping and execution tasks do not turn failed acceptance criteria
into passes. See [the 56-row acceptance matrix](sec-mlir-dialect_package15-acceptance.md).

## Producer increments — 2026-10-30

Two additional P15 producer slices are implemented and tested:

- Scalar static CTE materialization consumes frozen Sema initial-value facts
  through the existing scalar-global schema. It handles arithmetic/boolean
  expressions and resolved static dependencies, including exact wide integers,
  source-order-independent module dependencies, distinct function-local scopes,
  target-width floating operands in bool calculations and mutable initial values.
  No runtime startup computation is emitted. Forged conflicting/out-of-domain
  static initializer payloads fail Semantic IR verification and emit no MLIR.
- Ordinary copy-trivial whole-self value uses lower via one read from the existing
  borrowed receiver Place, preserving struct/named-scalar identities, reference
  capabilities and later receiver availability. Whole-self consumption remains
  forbidden (`functions.md` §22, `copy_move.md` §18); the old consuming-receiver
  completion requirement is obsolete, not an operation to invent.

Full Sema passes (16.560s); final full Semantic IR/nominal/emitter suites pass
(3.555s/0.119s/15.505s). New source cases pass schema 11 and 12 on 32/64 plans,
including all three P15 native verifiers and P5/P6/P8 preservation. Native
`static_cte` and `statics` cases pass through the single source-to-binary track
(2/2, 0.898s); vet passes and compiler/LSP packages compile. No new full-Go or
full native-lit snapshot is claimed for this delta.

P15 remains **partial**, with **3 → 3** remaining governance entries: storage/root
producers (static-held references, aggregate/executor-dependent initializers and
effective addressed/hardware facts/bindings/IR), generic receiver/root-dependent
return producers, and final package acceptance. MD-081 and MD-111 are decided;
their remaining producer work is implementation, not an open normative decision.
Physical reference representation/LLVM remains a later stage, not a P15 gate.

## Acceptance scope correction — 2026-10-30

The earlier statement that P15 must wait for the physical MD-061 optimization
matrix was too broad. P15 §§151–155 explicitly exclude physical representation
and LLVM generation. MD-061 §§6/7(5) require preservation/proof across available
transformations; each later lowering consumer must demonstrate the same safety
when it introduces physical operations. These guarantees do not make P15 depend
on completion of P24/P25/P26/P27. No safety requirement is waived.

New tests `TestPackage15HighLevelOptimizationBoundary` and
`TestPackage15OptimizationDependencyCFG` exercise the currently executable
boundary. Source reborrows, parent mutation after last use and returned origins
are verified on 32/64 plans in unlowered, scalar and integer stages, then after
canonicalize and CSE with all three P15 verifiers at each boundary. Unsupported
physical lowering remains rejected; no LLVM pointer/alias/lifetime promises are
introduced. Existing valid and malformed CFG cases cover epoch guards, lifetime
and transfer dependencies. Expected invalid chunks stop before an optimizer
could hide the error and emit no module; valid control chunks still emit.

The full physical/LLVM matrix remains explicitly owned by
`lowering.reference-model` in governance/memory.yaml. The erroneous P15 item is
removed by ownership reconciliation: P15 remaining **4 → 3** (storage/root
producers, receiver/return producers, final acceptance). P15 remains partial.
This audit changes implementation planning and tests, not language semantics or
the 56 normative criteria. Earlier full-Go execution below remains frozen
baseline evidence; the new test-only delta is verified by focused and complete
Semantic IR/emitter suites and vet, not labelled a new full-Go run.

## P15-75 revalidation — 2026-10-30

**The seven known Go test failures are repaired.** The full isolated Go run now
passes: **43 packages passed, 4 packages without tests, 0 failed**. There are
7952 passing test/subtest events (including nesting), with two tool-gated skips
whose execution was then explicitly verified below. The complete LSP package
passed in 539.859s. Full `go vet ./...` also passed.

The frozen source worktree is based on HEAD `aa59b4c18c91ed04eef69c7dd0028e41630067f5`
plus the current tracked/untracked source overlay, excluding distribution
binaries and the untracked root executable. Its 3989-file manifest is
`e9a65f8e08c1fd606ac7b9cc7efb3a6bb4791cea4261e8c2cb0adf180e5f036a`.
The copied `sec-mlir-opt` SHA-256 is
`039679b11d817bae2fb622084053a18d63486d6d18ef23697d2c10b4424f8e03`.
All implementation and test files in the primary tree were compared with that
manifest after execution; only acceptance/report/governance/status/handoff
text was changed afterward. No skipped or filtered failure was made green.

Repairs and their owners:

- `cmd/unicodegen/main.go` now emits the existing field alignment and ASCII
  range-membership spelling. The pinned Unicode 18.0.0 table/data is unchanged.
- `testdata/sema/collection_contracts/` uses fixed-array literals for the
  length/spread and uniqueness proofs. The three tests keep their exact
  diagnostic expectations and 32/64-bit, Interactive/Standard/Deep matrix;
  nonempty owning dynamic-array literals remain invalid.
- `TestNumericStringConstructorRelations` replaces the stale legacy oracle:
  primitive numeric-to-string relations remain valid, while direct nominal
  string constructors must produce located S1174 under MD-051/MD-093.
- Both IO tests now use checked-in fixtures under `testdata/compiler/io/` and
  the actual target-aware `check --target=linux-amd64` pipeline. This supplies
  the selected scalar plan instead of the old targetless analyzer.
- `inferRegisterOperationCall` no longer probes an already registered
  qualified free-function namespace as a runtime receiver. The dedicated
  `TestRegisterOperationLookupPreservesQualifiedFunctions` covers Read, Write
  and IsShadowKnown; ordinary register operation restrictions still pass.
- `sec/stdlib/io/transfer.sec` and legacy file Copy/Directory.Next form checked
  slice views before the fallible call. Ordinary slice checks retain their
  default panic semantics; IO/StringError propagation is kept separate, with
  no invented RangeError mapping. CopyN ends its mutable view before reading
  the chunk. This does not close MD-026 or implement MD-109/P23 runtime string
  materialization; no collect/Builder/URL migration is claimed.

Commands (`P15_SNAPSHOT` names the frozen worktree, `LLVM_LIT` and
`LLVM_TOOLS_BIN` name the configured LLVM build tools):

```sh
cd "${P15_SNAPSHOT}"
SEC_MLIR_BIN="${P15_SNAPSHOT}/_p15-tools" go test ./... -count=1 -json
SEC_MLIR_BIN="${P15_SNAPSHOT}/_p15-tools" go vet ./...
"${LLVM_LIT}" -sv "${P15_SNAPSHOT}/_p15-native/test"
"${LLVM_LIT}" -sv --filter='schema(9|10|11)' "${P15_SNAPSHOT}/_p15-native/test"
SEC_MLIR_BIN="${P15_SNAPSHOT}/_p15-tools" \
  SEC_LEGACY_MLIR_BIN="${LLVM_TOOLS_BIN}" \
  go test ./internal/lowering/secmlir -run '^TestExecuteBoolean(Equality|Not)ThroughSecMLIR$' -count=1 -v
go run ./cmd/yamlstatus check
git diff --check
```

The full native suite passed **191/191** in the frozen test tree; explicit
schema-9/10/11 selection passed **28/28**, with 163 excluded by that named
selection. The configured primary `check-sec-mlir -j2` target also passed
191/191 before freezing the source/tool. LLVM/MLIR is 24.0.0git, optimized with
assertions; lit is 24.0.0dev; Go is go1.26.8 linux/amd64.

`TestPackage15SourceToSchema11Pipeline` passed all **12/12** source/target
cases in the full run, with the three P15 verifiers and P6/P8 preservation.
The two Boolean execution tests that skipped in the full command because
`SEC_LEGACY_MLIR_BIN` was absent were explicitly rerun with both absolute tool
directories: **2/2 passed**, including native execution (0.442s). This is an
existing test-only tool variable, not a separate compiler backend.

Governance validation passed with **0 problems in 45 fragments**. The acceptance
inventory audit matched all **56 ordered criteria exactly** against §155,
all **46 report topics** against §156, and all eight required P15 rulebook
inventory entries to existing files/directories. Whitespace checks passed.

**P15-75 remains open for final package closure.** P15-42/42a still lack
addressed/hardware producers (MD-111), aggregate/non-literal static producers
and the separately gated static-reference initializer (MD-081); P15-47 still
has generic receivers and unrepresented root-dependent returns.
The full physical MD-061 matrix is retained by its later lowering consumers;
it does not gate P15 (see the scope correction above). P15 producer requirements
are neither waived nor arbitrarily moved. MD-116 scalar static
roots and MD-114 detached register transports are already implemented and no
longer counted as missing. The owning governance entry remains `partial`, with
**4 remaining items before and after** this prerequisite repair pass.

The numbered sections below preserve the historical 2026-10-25 report. Their
old failing-suite observations are superseded by this dated revalidation;
original evidence is retained rather than rewritten as a successful run.

## 1. repository HEAD implemented against

Implemented against HEAD `9eb7c77e4e3153b61d29610581e33e07e57e2fa0`, a verified descendant of
`152c772`. An isolated detached worktree overlaid the current tracked source diff
and untracked source/tests, including the concurrent P16 integration. Modified
`bin/` and `vscode/bin/` distribution binaries were excluded; native tools were
copied separately. The frozen source manifest covers **3478 files**:
`79b68edc50004b2061cd5d92e5182ace2cf36ba64008b4bc6f0272bd108e0e17`. Native tool SHA-256:
`d6133bfeba4a3ba7631fcb751f5328444b4c505807801309db50d978a83f5079`.

The full-suite snapshot predates the test-oracle correction described in section
43 and this report/governance update. The P15 production implementation is the
same. That later test-only delta was verified separately; final revalidation on
the latest tree remains P15-75. Source snapshot tools use `SEC_MLIR_BIN`; no
machine-specific workspace path is part of this report.

## 2. local/merged P13 and P14 status

P13/P14 are present in the merged baseline and retain their entries in
`governance/lowering.yaml`. The P15 source-to-schema-11 integration fixture
combines struct fields, fixed arrays and P12 match payload references. The
explicit schema-9/10/11 native selection passed 26/26 tests.

## 3. previous package status

Predecessor native and focused integration regressions pass. Repository-wide
Go acceptance does not pass: seven failing tests reproduce on clean HEAD; see
section 43. These are retained failures, not skipped or attributed to P15.

## 4. reference/runtime-check rule synchronization

The installed owners are `rules/memory/borrowing.md`,
`rules/memory/reference_model.md`, `rules/compiler/semantic_ir.md`, and the v11
consolidated dialect/lowering books. Applied MD-061/065/076/084/085 amendments
remain authoritative. This pass changes implementation/tests and evidence, not
normative language rules. MD-116 records the missing static Place text contract.

## 5. files added

This closure pass adds `internal/ir/semantic/reference_shared_call_test.go`,
`testdata/semantic_ir/reference_shared_call_reborrow.sec`, three
`testdata/sema/reference_shared_call_*_invalid.sec` fixtures, this report and
`sec-mlir-dialect_package15-acceptance.md`.

The shared candidate compatibility helper/tests are now focused as
`internal/sema/reference_call_arguments.go` and
`internal/sema/reference_call_arguments_test.go`, generalized from the previous
uncommitted slice-only files. Existing P16 overlay files are included in the
frozen combined-tree validation; they are not relabelled P15 work.

## 6. files modified

P15 code integration changes `internal/sema/analyzer_inference.go` (direct
and expected-type generic candidate checks) and
`internal/lowering/secmlir/package15_pipeline_test.go` (checked-in direct-reference
call fixture). The structured type-compatibility matrix now distinguishes
permitted call reborrowing from storage initialization/explicit conversion.
`missing-decisions.yaml`, `package15-todo.md`, the P15 governance entry and shared
handoff carry the new evidence and open boundaries. No distribution binary was
rebuilt or modified by this closure pass.

## 7. PlaceRootID migration

`internal/sema/place_identities.go` owns stable root identities from resolved
declarations. `place_root_identity_test.go::TestPlaceRootIdentityFollowsDeclarations`
and summary/generic identity cases provide focused evidence; source spelling is
not a correctness identity.

## 8. arbitrary-precision place-index migration

`PlaceProjection` retains arbitrary-precision constant indexes, and P15
producers consume resolved P14 index plans. `TestPackage15PlaceReferenceOperationsVerifyAndFormat`
includes index 18446744073709551619, above uint64. Dynamic indexes retain actual
SSA identity rather than inventing a text identifier.

## 9. PlaceRelationship implementation

`internal/sema/place.go` provides canonical relationship facts.
`place_relationship_test.go` covers fields, indexes, disjoint paths and joined
origins. P15 does not rerun alias inference in the emitter.

## 10. legacy PlacesOverlap compatibility

`TestLegacyPlacesOverlapDelegatesToRelationship` and
`TestLegacyPlacesOverlapHasNoCorrectnessCallers` preserve the compatibility
wrapper without retaining a second correctness model.

## 11. StorageIdentity/InvalidationDomain/EpochDependency representation

`internal/sema/place_identities.go` and `reference_plans.go` publish canonical
storage, invalidation and epoch identities. `TestRootPlacesCarryStorageIdentity`
and `TestReferencePlansPublishCompleteFacts` cover transport into plans; native
`ReferenceFacts.h` reads the serialized facts without source re-analysis.

## 12. ResolvedPlaceOf API

`internal/sema/reference_plans.go::ResolvedPlaceOf` returns a detached snapshot.
`TestReferencePlanQueriesAreReadOnly` verifies that querying and mutating a result
does not rerun analysis or modify the analyzer's facts.

## 13. ResolvedReferencePlanOf API

`ResolvedReferencePlanOf` is the analogous read-only creation/reborrow query.
The same read-only query regression covers it. Builders reject missing/uncheckable
plans instead of fabricating authority.

## 14. ReferenceFacts representation

`reference_plans.go` carries borrow/lifetime, validity/proof, storage/origins,
address space, stability, pinning and optional domain/epoch dependencies.
`TestReferencePlansPublishCompleteFacts` and schema metadata consistency tests
exercise these independent facts.

## 15. validity-policy implementation

Proven references retain their proof with no generation check. Dynamic epoch
uses validate their exact SSA and enter `fail.reference-generation` on false.
`TestPackage15LowersProvenReferenceUses`,
`TestPackage15DynamicReferenceUseValidatesOnce` and native CFG guard tests cover
the represented surface. Arena source allocation remains unsupported.

## 16. CompilationPlan epoch-width facts

`TestReferencePolicyEpochWidth` covers the 64-bit logical epoch default on
both pointer-width plans. Epoch width is a CompilationPlan semantic fact,
not a selected descriptor field or pointer width.

## 17. relocation/address-space facts

MD-084's canonical `address_stability` values and independent pin dependency
replace historical relocation-class wording. Address spaces are transported by
existing facts; target-specific spaces without representation fail closed.
`TestPackage15EmitRejectsUnrepresentedReferenceFacts` and
`reference-metadata-consistency.mlir` cover the boundary.

## 18. BorrowID/LifetimeID implementation

BorrowID and LifetimeID are nonzero typed metadata, not inferred durations.
Creation/reborrow gets resolved identities; copy/move preserves its contract.
`package15_test.go` and native metadata/lifetime CFG matrices reject inconsistent
or unrepresented identities.

## 19. Semantic IR place operations

Local/value/storage roots, field/array-element/variant payload projections,
dereferences and copy-trivial reads/writes are implemented by the focused
reference and payload producers. Static Place roots exist in Semantic IR but
cannot serialize (MD-116). Addressed roots require MD-111; they remain open.

## 20. Semantic IR reference operations

Borrow, shared-copy, move, shared/mutable reborrow, validity, comparison and
end-borrow operations are represented in Semantic IR and schema 11. Local and
function/receiver/parameter source routes use resolved Sema facts. The new shared
call fixture reaches existing reference operations without a new dialect op.

## 21. shared-copy/mutable-move implementation

Shared references are copyable; exclusive references are moved/reborrowed,
never duplicated by copy-only predecessor operations. The direct call fix permits
`ref mut T` to grant `ref T` while preserving the original type for overload
ranking. Negative source tests retain overlap, retained-result, nominal identity,
shared-to-mutable and storage-initialization restrictions.

## 22. reborrow implementation

`reference_use.go` lowers explicit, subplace, receiver and call reborrows.
`TestPackage15LowersExplicitReborrows` and
`TestPackage15LowersReferenceParametersAndPassing` remain focused regressions.
`TestPackage15SharedReferenceCallReborrow` verifies a distinct shared child and
retained mutable parent at both widths, including a returned shared dependency.

## 23. validity guard/failure CFG

The dynamic guard/failure endpoint is explicit and stays ahead of the
observing operation. `VerifyPlaceReferences.cpp`, `VerifyReferenceTransfers.cpp`
and CFG lit tests reject wrong SSA, missing/dominated guards, forged transfer
policy and invalid failure reasons; no Result fallback is introduced.

## 24. reference equality

Reference equality remains a P15 semantic operation with applicable dynamic
validation. `TestPackage15LowersProvenReferenceUses` and native comparison CFG
fixtures exercise it. P16 slice equality is not admitted by this feature.

## 25. end-borrow representation

Sema schedules path-specific required ends (MD-065); Semantic IR emits
represented `ref.end-borrow` markers and verifies later uses. Payload tests cover
arm exits and last use. General path-specific source scheduling limitations
remain explicit; scope-exit markers are not a substitute for unknown facts.

## 26. schema-v11 types/ops

`SecTypes.td`, `SecOps.td` and focused dialect implementations define
`!sec.place`, `!sec.ref`, `!sec.ref_mut` and the installed operation family.
Schema 12 inherits metadata, and pre-v11 inputs reject the newer types. MD-077
register types are installed, but its operations remain blocked on MD-114/111.

## 27. place verifier

`--sec-verify-places` is registered and is run for every 32/64 source pipeline
case. It checks represented roots, projections, authority, exact element guards
and variant membership. Static root serialization is not silently substituted
with `sec.place.storage`.

## 28. reference guard verifier

`--sec-verify-reference-guards` is registered and exercised by the source
pipeline, dynamic white-box tests and native CFG suites. Guard identity follows
unambiguous forwarding; ambiguous/rootless sources do not grant proof.

## 29. borrow verifier

`--sec-verify-borrow-semantics` is registered and checks copy/move/reborrow,
identity/metadata consistency, end/move ordering and structural returned origins.
It is not a second source-level borrow checker. All source pipeline cases run it.

## 30. P5 address-taken storage changes

Address-taken automatic storage is explicitly marked before applicable P5
lowering; `sec.place.storage` cannot address an unmarked declaration. Checked
integer/reference integration and schema-11 storage guard tests cover this;
reference-dependent storage is not erased by trivial-core lowering.

## 31. P6/P8 compatibility

`schema11-reference-scalar-layout.mlir` covers 32/64 P6 scalar resolution
and selective P8 integer lowering while retaining semantic wrappers and reference
metadata. The source pipeline also runs scalar/integer stages (MD-087).

## 32. P13 field-borrow integration

`TestPackage15LowersFieldBorrowsWithoutAggregateCopies` and
`TestPackage15FieldAndElementBorrowIntegration` cover P13 stored field identities
and nested sub-Places. Computed properties remain non-addressable.

## 33. P14 array-element-borrow integration

`TestPackage15LowersFixedArrayElementBorrows` preserves the evaluated source
and original index, exact bounds facts, guarded success edge and ordinary failure
path. Array borrowing does not copy the enclosing array.

## 34. P12 match-borrow integration

`internal/ir/semantic/payload_reference.go` and its tests cover variant-proven
P12 Option/Result/union payload borrows, receiver/field subjects and arm-end facts.
General aggregate-contained reference extraction still lacks per-path serialized
facts and remains a separate source capability boundary.

## 35. returned-reference integration

`return_origin.go` and `VerifyPlaceReferences.cpp` preserve input/subfield/
array-element/call summaries. MD-085 requires structural paths and only the
callee's declared origins. The new `ReturnShared` source function checks shared
return composition through a mutable-to-shared call. Static origins stay MD-116;
generic receivers remain P15-47 producer/dependency work; ordinary whole-self consumption is forbidden by the current rules.

## 36. proven-reference tests

Proven source and metadata tests run in the full root Sema/IR/emitter suites;
those packages passed in the frozen snapshot. The source pipeline has 12 passing
32/64 subtests. The acceptance matrix links individual focused tests rather than
using broad success as proof for unrelated semantic criteria.

## 37. dynamic-epoch tests

Dynamic validity is covered from resolved/verified Semantic IR onward by
`TestPackage15DynamicReferenceUseValidatesOnce` and
`TestPackage15EmitsDynamicReferenceGuards`, plus native CFG matrices. These are
white-box facts: a P15 source Arena allocator does not yet produce them.

## 38. 32/64-bit profile tests

`TestPackage15SourceToSchema11Pipeline` builds actual target-aware Sema/IR at
32-bit armv7 and 64-bit amd64, records schema 11 and the target, and runs all three
reference verifiers. `TestPackage15PlaceReferenceFactsAreDeterministic` covers
identity/metadata agreement between plans.

## 39. unsupported handle/slice/raw-conversion tests

`TestPackage15RejectsOutOfScopeReferencesWithoutPartialModule` retains P15's
handle/slice boundary and distinct raw-pointer semantics. Unrepresented addressed,
static-root and hardware constructs fail by name; no physical fallback route or
partial IR module is returned.

## 40. CMake commands

Commands run against the configured native build:

```sh
cmake --build build/sec-mlir --target check-sec-mlir -j4
llvm-lit -sv --filter='schema(9|10|11)' build/sec-mlir/test
```

`llvm-lit` here denotes the exact tool from the configured LLVM build. The Go
snapshot uses an absolute `SEC_MLIR_BIN`, containing separately copied native
tools; logs and binaries are not embedded in this report.

## 41. exact LLVM/MLIR version

Go 1.26.8 linux/amd64; LLVM/MLIR 24.0.0git, optimized build with assertions;
lit 24.0.0dev. `sec-mlir-opt --version`, `llvm-lit --version` and `go version`
were read during this execution.

## 42. check-sec-mlir result

Full `check-sec-mlir`: **165 discovered, 165 passed**. Adjacent filtered run:
**26 passed, 139 excluded**; no selected failures. The filter is recorded in
section 40 and was not broadened to hide any failing test.

## 43. go test ./... result

The isolated command completed (not interrupted):

```sh
SEC_MLIR_BIN="$P15_SNAPSHOT/_p15-tools" GOCACHE=/tmp/sec-go-cache \
  go test ./... -json -count=1
```

Exit 1 after **434.556s**. Package outcomes: **38 passed, 5 failed, 4 without
tests**. There were 6992 passing test/subtest events, eight failing test events
and two skips; these event counts include nesting and are not unique top-level
test totals. Root Sema, Semantic IR and Sec-MLIR emitter packages passed.

Seven failures reproduce in clean detached HEAD:

- `cmd/unicodegen`: `TestGeneratedUnicodeSourceIsCurrent`.
- `internal/sema/collectionshape`: `TestArrayLiteralLengthContractsUseExactCompactLength`,
  `TestArrayLiteralUniqueContractRejectsProvenDirectDuplicates`,
  `TestUniqueContractSemanticProofs`.
- `internal/sema/temporal`: `TestRepresentedMutexCallsDoNotInventUnitOverloads`.
- `cmd/compiler`: `TestCompilerLoadsIOReadFileIntoAPI`,
  `TestCompilerLoadsIOWriteSeekAndDirectoryAPIs` (stdlib IO/RangeError/removed
  string API; byte-to-string contract already has MD-109).

The eighth failure, `typecompatibility::TestParameterizedAndAggregateMatrix`,
expected a `ref mut int` argument to a `ref int` parameter to fail. Its template
is an invocation, not storage initialization, so that expectation contradicted
`borrowing.md` section 15.3(2). The oracle now accepts that call (and the analogous
slice call), while explicit conversion and authority upgrade remain rejected.
`go test ./internal/sema/typecompatibility -run TestParameterizedAndAggregateMatrix
-count=1` passed after that test-only correction. The newly added storage and
retention negative tests also pass; storage compatibility was not weakened.
The whole suite was not rerun after that oracle correction: P15-75 remains the
final revalidation task, and the repository-wide green criterion is not met.

`go vet ./...` in the same frozen snapshot passed in **13.088s**. Snapshot
`git diff --check` passed. Unique-key YAML parsing passed for 46 current ledger
files; full governance checking in that frozen snapshot reported the two missing
`concurrency_events.yaml` and `concurrency_spawn.yaml` registry fragments.

Subsequent registry repair (2026-10-25): both were empty fragments deleted in
commit `82de384`, with stale entries left in `governance/index.yaml`. Removing
those two entries loses no integration records. Current `yamlstatus check`
passes with **0 problems in 45 fragments**. The earlier snapshot failure above
is retained as historical evidence; governance is no longer a current blocker.

## 44. end-to-end source -> schema-v11 results

The isolated package-specific run completed with **12/12 subtests passed**:
six fixtures at each of 32/64 bits, with absolute configured native tooling.
The new fixture is checked in under `testdata/semantic_ir/` and covers mutable
parent reuse, shared returned dependencies and exact-overload preference.

```sh
SEC_MLIR_BIN="$P15_SNAPSHOT/_p15-tools" GOCACHE=/tmp/sec-go-cache \
  go test ./internal/lowering/secmlir \
  -run '^TestPackage15SourceToSchema11Pipeline$' -json -count=1
```

Every case runs `--sec-verify-places`, `--sec-verify-reference-guards` and
`--sec-verify-borrow-semantics`, plus the existing scalar/integer pipeline.
This is source-to-semantic-MLIR inspection, not executable reference ABI support.

## 45. deviations

Historical deviation notes below are superseded by the producer increments and
acceptance correction above. MD-114 detached-register transport, MD-116 static
Place roots and the MD-110 destination/bit-extraction spellings are implemented.
MD-081 is decided; static-held reference materialization still lacks its producer
and representation. MD-111 is decided and source schema/Sema facts exist;
effective plan assembly, named endpoints/bindings and hardware IR are still work.
No undecided language semantics is invented or waived to force P15 acceptance.

Remaining source work includes generic reference receivers and complete
root/return support (P15-42/42a/47). Nontrivial lifecycle/cleanup belongs to P17;
owning arrays to P18; physical reference representation to later lowering. The
MD-061 differential optimization matrix is not replaced by high-level tests.
Final P15-75 acceptance cannot be completed with these gaps and failing full-Go
or governance checks. MD-101 supersedes obsolete multiple-backend instructions.

## 46. recommendations for Package 16

P16 already inherits the installed P15 metadata/verifier family. Preserve
exact storage/origin/epoch, address stability/pin and borrow/lifetime facts through
slice parameters, calls and element Places. Share the call-compatibility rule
without converting storage initialization or weakening exact overload ranking.
Do not close P16 returned subranges until MD-115 is decided, or projected holders
until aggregate-contained reference facts have a concrete representation.
Keep physical descriptors/ABI and nontrivial ownership in their owning packages.


## MD-114 implementation amendment — 2026-10-28

This amendment records work after the frozen 2026-10-25 snapshot; it does not
rewrite that baseline evidence or promote P15 acceptance. The canonical detached
register table, typed field/write-proof attributes, pure extract/update path
verification and Go emission are now implemented. Constant/source-domain
register.from_bits and signed/unsigned register.to_bits use the same table.
Source/native inspection covers 32/64 plans and schemas 11/12 through P5/P6/P8;
field carriers through 256 bits, nominal enums/nested types, layout/reserved
bits, byte-order omission/retention and proof forgery rejection are covered.

MD-110 still needs the exact fits_bits destination-identity attribute and its
bit-extraction proof transport. Quantity representation and broader arithmetic/
CFG evidence remain named unsupported boundaries under MD-114 §§3(8),7(8).
Hardware endpoints/plans remain MD-111; no physical register representation or
binary support is selected. Current commands/results and the single ownership
record are in governance/lowering.yaml, lowering.sec-mlir-package15.
