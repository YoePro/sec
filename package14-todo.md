# SEC-MLIR Package 14 — implementation TODO

Package: `SEC-MLIR-P14` — Fixed Array Semantic Value Representation  
Primary rulebook: `rules/mlir/packages/sec-mlir-dialect_package14.md`  
Package governance: `rules/mlir/packages/sec-mlir-dialect_package14.yaml`  
Canonical amendments: `rules/mlir/semantic-ir/sec_semantic_ir_fixed_array_v1.md`,
`rules/mlir/dialect-versions/sec_mlir_dialect_v10.md`, and
`rules/mlir/lowering-versions/sec_mlir_lowering_v10.md`

## Inventory snapshot — 2026-08-28

Audited repository HEAD:

```text
069c111b96714ee4c9423f5f7a64d1e7a6045f31
```

Package 13 is implemented at this HEAD. The current frontend already has useful
fixed-array support:

- postfix `T[N]` parsing and array literals;
- compile-time length expressions and constant index diagnostics;
- fixed-array literal spread with compact checked length accounting;
- defaultability, copy/destruction classification, equality/membership typing,
  Place-backed indexing, iteration, call spread, and compiler-known collection
  facts;
- a legacy physical MLIR/LLVM path for selected fixed-array operations and
  iteration.

That support is not the canonical Package 14 representation. Current Sema still
uses `int64 ArrayLength` and `-1` for dynamic owning arrays. Default resolution
may expand one semantic entry per element. The maintained Semantic IR and Sec
MLIR emitter have no fixed-array type or operations, and the implemented dialect
remains schema 9.

Verified in the audited working tree:

- `go test ./...` — passed;
- `go vet ./...` — passed;
- `cmake --build build/sec-mlir --target check-sec-mlir -j2` — passed, 78/78;
- schema-v10 and lowering-v10 rulebooks are installed;
- `rules/collections/collections.md` already recognizes fixed-array literal
  spread and no longer contains the stale no-spread rule.

The physical legacy array lowering is evidence of frontend maturity, not the
P14 implementation model. P14 must remain high-level and runtime-free: no
`!llvm.array`, `undef`, GEP, MemRef layout, hard-coded `i64` indexing, or runtime
array service may become the canonical Semantic IR/Sec MLIR path.

## Completion boundary

Package 14 owns:

- explicit fixed/dynamic array shape and arbitrary-precision fixed length;
- plan-specific target-`uint` validation;
- compact literal, spread, and default plans;
- fixed-array values and operations in Semantic IR;
- safe value indexing, bounds proof/check plans, ordinary bounds failure, and
  fallible `IndexError.OutOfBounds` flow;
- trivial extraction, replacement, high-level storage, effects, schema 10, and
  the array-index guard verifier;
- compatibility with P5/P6/P8/P11/P12/P13.

Do **not** pull these into P14:

- dynamic owning-array or slice Semantic IR;
- element `ref`/`ref mut`, array-to-slice borrowing, or physical references
  (Package 15/16);
- move-out, semantic-copy spread, or non-trivial destruction/replacement
  (Package 17);
- equality or membership lowering;
- physical layout, aggregate ABI, foreign ABI, MemRef, LLVM arrays, GEP, or a
  mandatory runtime.

Every new semantic function should cite the governing Package 14 or canonical
rulebook section where that improves traceability. Generic helpers only need a
short purpose comment.

## A. Lock the baseline, rule synchronization, and governance

- [x] P14-01 — Record the implementation baseline as repository HEAD
  `069c111b96714ee4c9423f5f7a64d1e7a6045f31`, which contains completed P13.
- [x] P14-02 — Verify the P13 struct value, storage, schema-9, P6, and P8 tests
  remain green before introducing array wrappers.
- [x] P14-03 — Confirm the current collections and spread rulebooks already
  permit one or multiple fixed-array literal spreads with compile-time length.
- [x] P14-04 — Synchronize `rules/errors/runtime_checks.md` and
  `rules/library/core-library.md` with errorhandling-v2: fixed-index failure is
  the concrete error `IndexError.OutOfBounds`, not a parallel `BoundsError`.
- [x] P14-05 — Confirm the Semantic IR fixed-array amendment, schema-v10 dialect
  rulebook, lowering-v10 rulebook, and Package 14 YAML are installed.
- [x] P14-06 — Record the green starting gates: `go test ./...`, `go vet ./...`,
  and `check-sec-mlir` 78/78.
- [x] P14-07 — Add `lowering.sec-mlir-package14` to
  `implementation-status.yaml` as `partial`, with exact rules, code, tests,
  deferred boundaries, baseline HEAD, toolchain version, and commands.
- [x] P14-08 — Add one absolute-path `SEC_MLIR_BIN` helper/regression for P14
  source-to-schema-10 tests; never depend on a Go package working directory.

Completed 2026-08-28: runtime-check and core-library wording now names the
errorhandling-v2 concrete `IndexError.OutOfBounds` channel. Governance records
the exact P14 baseline, current frontend/legacy foundations, deferred package
boundaries, toolchain, and green commands without claiming schema-10 code is
implemented. The existing absolute-path verifier helper is now explicitly
shared by P13/P14 and has a package-named regression ready for later schema-10
source tests.

## B. Replace the canonical `int64/-1` array-shape model

- [x] P14-09 — Introduce an explicit canonical array-shape kind (`fixed` versus
  `dynamic`) and an immutable arbitrary-precision fixed length. Dynamic shape
  has no length and non-array types have neither field set.
- [x] P14-10 — Keep the parsed length expression as source syntax while removing
  the AST/Sema `int64` cache as semantic authority. Any legacy cache must be
  clearly marked and populated only after an explicit representability check.
- [x] P14-11 — Rewrite fixed-length resolution to evaluate a non-negative exact
  `big.Int`; reject runtime-dependent, non-integer, and negative expressions
  without rejecting a valid value merely because it exceeds host `int64`.
- [x] P14-12 — Remove `ArrayLength == -1` from canonical correctness decisions.
  Isolate dynamic-to-legacy-sentinel translation behind one checked
  compatibility helper used only by old backends.
- [x] P14-13 — Migrate fixed-array type identity, display, equality,
  `sameConcreteType`, generic substitution, function signature identity,
  recursive-storage checks, and deterministic hashing/printing to exact decimal
  length.
- [x] P14-14 — Migrate defaults, copy/destruction classification, membership,
  Place projections, slicing prechecks, call spread, diagnostics, and LSP type
  facts to explicit shape without conflating `T[N]`, owning `T[]`, or slices.
- [x] P14-15 — Add CompilationPlan validation that independently checks exact N
  against target `uint` on 32- and 64-bit outputs. One multi-output build must be
  allowed to accept one plan and reject another deterministically.
- [x] P14-16 — Require legacy physical-layout/codegen consumers to check
  `IsInt64`/target layout limits and return an explicit unsupported error rather
  than changing source-language validity.
- [x] P14-17 — Add the section-90 length matrix: zero, ordinary expressions,
  negative/runtime/non-integer rejection, uint32/uint64 boundaries, one-above
  rejection, `>int64 && <=uint64`, exact identity, and absence of a dynamic
  sentinel. Tests must not allocate huge arrays.

Completed 2026-08-28: Sema now records explicit fixed/dynamic shape and an
immutable canonical decimal length, with defensive `big.Int` access. Parser
length expressions remain authoritative while the old `int64` field is only a
checked compatibility cache. Type identity, display, generics, equality,
copy/destruction, membership, bounds/slicing, Place, call spread, compiler-known
members, LSP presentation, literal spread, and defaults now consume explicit
shape/exact length; large literal/default facts remain compact. The same fact
can be validated independently against 32- and 64-bit scalar plans. The legacy
LLVM-array generator reports an explicit unsupported-layout error for lengths
outside `int64`. The section-90 matrix covers exact expressions, zero,
negative/runtime/non-integer input, both target boundaries, above-boundary
failures, above-`int64` identity, defensive copies, no sentinel conflation, and
no huge allocation.

## C. Add compact compiler-owned literal and default plans

- [ ] P14-18 — Define immutable `ResolvedArrayLiteralEntryKind`, transfer-action,
  entry, and plan facts. Each source element/spread is one entry with exact
  length; the plan owns no duplicate AST nodes.
- [ ] P14-19 — Add read-only
  `ResolvedArrayLiteralPlanOf(*ast.ArrayLiteral)` returning defensive copies and
  never re-inferring, mutating Sema, or allocating O(expanded length) records.
- [ ] P14-20 — Refactor current `arrayLiteralSegments` analysis into the
  authoritative plan while preserving left-to-right source order, target-shaped
  literals, exact inference, and the target-required empty literal rule.
- [ ] P14-21 — Preserve every fixed-array spread as one source entry; evaluate
  its expression once, allow multiple spreads, add exact lengths with `big.Int`,
  and reject runtime-length sources or mismatched element types.
- [ ] P14-22 — Record `construct-direct`, `copy-trivial`, and the later action
  vocabulary from compiler-owned copy facts. Accept only the P14 trivial subset
  in the new IR and reject semantic-copy/move/borrow actions explicitly.
- [ ] P14-23 — Prove a literal containing a spread of conceptual length above
  `int64` remains O(number of source entries) in Sema and diagnostics.
- [x] P14-24 — Replace expanded fixed-array `DefaultResolution.Elements` as new
  IR authority with one compact exact-length default fact. Preserve old expanded
  data only behind a bounded legacy compatibility path.
- [x] P14-25 — Implement the zero-length exception: `T[0]` is defaultable even
  when T is non-defaultable, and querying/building the default must not construct
  or inspect a T value.
- [ ] P14-26 — Add the complete section-91/93 plan and default tests, including
  ordinary/target/inferred/empty literals, multiple spreads, read-only query,
  compact large lengths, nested defaults, non-defaultable elements, and no
  `undef`/poison substitute.

## D. Implement fixed arrays in Semantic IR

- [ ] P14-27 — Add canonical fixed-array type data keyed by element `TypeID` and
  exact immutable length; intern, compare, clone, print, and verify it without
  host-width conversion.
- [ ] P14-28 — Extend the module/type verifier with fixed/dynamic separation,
  canonical non-negative decimal length, supported element type, exact nominal
  nesting, and deterministic diagnostics.
- [ ] P14-29 — Add compact array construction segments (`element`/`spread`) and
  `ArrayConstructOp`. Verify operand type, action, segment length, exact total,
  result type, and full initialization.
- [ ] P14-30 — Add `ArrayDefaultOp` with one element type and exact length.
  Permit zero length or the supported infallible trivial default subset and
  reject unrepresented cleanup obligations.
- [ ] P14-31 — Add `ArrayLengthOp` returning compiler-known target-sized `uint`;
  retain its exact compile-time value as a foldable semantic fact.
- [ ] P14-32 — Add `ArrayIndexInBoundsOp` producing the canonical bounds
  predicate from one array value and one source-typed index value.
- [ ] P14-33 — Add guarded `ArrayExtractOp` for copy-trivial reads and
  `ArrayReplaceOp` for trivial replacement, carrying exact index-plan proof or
  guard provenance rather than guessing safety from CFG shape.
- [ ] P14-34 — Add terminating `BoundsFailureOp` for ordinary failed bounds
  checks and the explicit `IndexError.OutOfBounds` construction/branch for the
  fallible path.
- [ ] P14-35 — Extend Semantic IR printer/verifier tests for zero/large lengths,
  nested arrays, wide/struct/enum elements, compact segments/defaults, exact
  result types, bad sums, guard mismatches, and every P14 operation.
- [ ] P14-36 — Ensure every P14 `UnsupportedFeatureError` retains package 14,
  returns no partial module, and emits no placeholder operation.

## E. Resolve indexing, bounds control flow, `try`, and effects

- [ ] P14-37 — Define immutable `ResolvedArrayIndexPlan` facts with exact array
  type/length, element type, original index type, use kind, check kind,
  proof kind/provenance, and ordinary versus fallible failure mode.
- [ ] P14-38 — Add read-only
  `ResolvedArrayIndexPlanOf(*ast.IndexExpression)` and make all new IR consumers
  use it instead of recomputing constant bounds or member syntax.
- [ ] P14-39 — Perform constant bounds evaluation with arbitrary precision for
  signed/unsigned and wide constants. Reject negative, N, and greater-than-N at
  compile time; valid constants are proven-safe and emit no failure operation.
- [ ] P14-40 — Integrate existing range, branch, assertion/contract, and analysis
  refinements as explicit proof kinds. Zero-length arrays never have a
  proven-valid element index.
- [ ] P14-41 — Classify every remaining integer index as runtime-checked while
  preserving its source type through comparison and downstream operations; do
  not normalize it to a hard-coded `i64`.
- [ ] P14-42 — Preserve evaluation order and exactly-once behavior: evaluate the
  array/place first and index second; for assignment evaluate target/index,
  then RHS completely, then commit replacement.
- [ ] P14-43 — Build the ordinary runtime CFG from one predicate: success
  dominates extraction/replacement and failure terminates in
  `BoundsFailureOp`. Negative signed and `>= N` cases must fail.
- [ ] P14-44 — Implement `try array[index]` as a resolved fallible operation:
  success produces T and failure produces precisely `IndexError.OutOfBounds`,
  with compatible naked propagation and direct local handlers.
- [ ] P14-45 — Extend compiler-owned try facts with bounds-propagation/handled
  bounds kinds. The backend must not reconstruct fallibility from unused Result
  values or textual `try`.
- [ ] P14-46 — Integrate effects: unproven ordinary access adds
  `MayBoundsPanic`; proven-safe access removes that effect; fallible access moves
  bounds failure into the typed error flow while retaining operand effects.
- [ ] P14-47 — Enforce `@noPanic` for ordinary dynamic access and accept proven
  or fallible indexing. `unsafe` must never bypass fixed-array bounds checks.
- [ ] P14-48 — Add sections 95–98 tests for constant/proven/runtime/fallible
  indexing, wide indexes, zero length, negative signed runtime values,
  exactly-once evaluation, local/catch-all handlers, `@noPanic`, and absence of
  `sec.fail.bounds` on proven/fallible success models.

## F. Add trivial mutable storage and nested replacement

- [ ] P14-49 — Extend P5 high-level storage eligibility to copy-trivial,
  trivially destructible fixed arrays whose element representation is already
  supported. Never memref-lower the wrapper in P14.
- [ ] P14-50 — Lower defaulted and explicit mutable local fixed arrays through
  semantic storage declare/init/load/store operations with no physical layout.
- [ ] P14-51 — Implement indexed assignment as RHS-first whole-array load,
  guarded/proven `ArrayReplaceOp`, and exactly one root store.
- [ ] P14-52 — Rebuild nested array/struct paths leaf-to-root using P13 struct
  replacement and P14 array replacement, preserving the exact root type and one
  commit.
- [ ] P14-53 — Add section-99 tests for scalar/wide replacement, runtime guard,
  nested arrays/structs, RHS exactly once/before commit, one root store,
  high-level storage, and explicit non-trivial rejection.

## G. Implement Sec MLIR schema 10 and verification

- [ ] P14-54 — Bump generated modules to schema 10 only after schema 9 remains a
  checked compatibility input; update ODS/C++ registration and version gates.
- [ ] P14-55 — Implement `!sec.array<T, "N">` with canonical arbitrary-precision
  decimal `StringAttr`; reject empty, signed, leading-zero, negative, or otherwise
  non-canonical length spelling except canonical `"0"`.
- [ ] P14-56 — Implement `sec.array.construct` with compact ordered segment
  metadata/actions and exact C++ verification of operands, spread types, and sum.
- [ ] P14-57 — Implement `sec.array.default`, `sec.array.len`,
  `sec.array.index_in_bounds`, `sec.array.extract`, `sec.array.replace`, and
  terminating `sec.fail.bounds` with exact schema-v10 verifiers.
- [ ] P14-58 — Register `--sec-verify-array-index-guards`. Verify dominance,
  true-path use, same array/index SSA, runtime versus proven-safe modes, and
  mandatory proof provenance for unchecked proven-safe operations.
- [ ] P14-59 — Extend the Go schema-10 emitter from verified Semantic IR;
  preserve exact lengths, source segment order, actions, proof kinds, locations,
  target index types, and high-level storage.
- [ ] P14-60 — Add all section-100/101 MLIR round-trip and invalid tests,
  including large lengths, multiple segments, bad sums/types, all operations,
  guard failures, proven-safe provenance, and explicit schema-9 regression.

## H. Prove package compatibility and source-to-schema-10 integration

- [ ] P14-61 — P6: recursively resolve target-sized `int`/`uint` in array
  elements for 32- and 64-bit plans while preserving array length, nesting,
  operations, tags, and surrounding P13 struct wrappers.
- [ ] P14-62 — P8: prove checked-integer signless normalization neither recurses
  through `!sec.array` nor lowers array operations.
- [ ] P14-63 — P13: cover array-in-struct, struct-in-array, nested replacement,
  defaults, wide fields/elements, and exact nominal identities in both directions.
- [ ] P14-64 — P11/P12: allow supported trivial fixed arrays in union payloads
  and match values without introducing array patterns or weakening union guards.
- [ ] P14-65 — Add source-to-verified-Semantic-IR-to-schema-10 tests for every
  section-102 case: literals/spreads/defaults/zero/nesting/functions/Len,
  constant and dynamic ordinary/fallible indexing, handlers, and replacement.
- [ ] P14-66 — Run the same source-built schema-10 module through an absolute
  `sec-mlir-opt` path on 32- and 64-bit CompilationPlans and assert no unresolved
  casts, physical array layout, runtime helper, or LLVM dialect.

## I. Prove unsupported boundaries are explicit and safe

- [ ] P14-67 — Add source-to-Semantic-IR rejection tests for move-only and
  semantic-copy spread, move-only element reads, element borrows, move-out,
  non-trivial replacement/destruction, dynamic owning arrays, slices, and
  array-to-slice conversion.
- [ ] P14-68 — Reject equality/membership lowering, foreign fixed-array ABI, and
  any physical layout request at the P14 package boundary while leaving their
  already-valid frontend typing or legacy path intact where applicable.
- [ ] P14-69 — Assert successful P14 modules contain no `undef`, poison, partial
  readable array, hidden allocation, MemRef array layout, `!llvm.array`, GEP,
  hard-coded semantic `i64` index, LLVM dialect, or mandatory runtime call.
- [ ] P14-70 — Prove determinism: canonical decimal lengths, type/segment/source
  order, nested printing, diagnostics, and 32/64 multi-output validation must be
  independent of host word size and map traversal.

## J. Final acceptance, governance, and report

- [ ] P14-71 — Map every section-107 acceptance item to a focused test or exact
  implementation location. Do not mark the package implemented from aggregate
  test success alone.
- [ ] P14-72 — From a fresh isolated worktree containing the exact P14 diff, run
  `go test ./...` and `go vet ./...`; record commands, HEAD, and results.
- [ ] P14-73 — Run the full `check-sec-mlir` target and every adjacent schema
  regression with the exact LLVM/MLIR version recorded.
- [ ] P14-74 — Run the package-specific 32/64 source → Sema → verified Semantic
  IR → schema-10 → verifier pipeline with an absolute tool path.
- [ ] P14-75 — Update `lowering.sec-mlir-package14` with the exact integrated
  surface, deferred P15/P16/P17 boundaries, commands, results, audited HEAD, and
  `status: implemented` only when every section-107 criterion is satisfied.
- [ ] P14-76 — Write the mandatory section-108 implementation report covering
  all 41 requested topics, deviations, and Package 15 recommendations.
- [ ] P14-77 — Re-run `git diff --check`, unique-key YAML validation, complete Go
  tests with absolute `SEC_MLIR_BIN`, and the full MLIR suite before handoff.

## Recommended execution order

1. P14-01–08 — lock rules, baseline, governance, and reproducible tools.
2. P14-09–17 — make fixed-array shape and length canonical.
3. P14-18–26 — establish compact Sema literal/default authority.
4. P14-27–36 — add the complete fixed-array Semantic IR value layer.
5. P14-37–48 — implement safe indexing, `try`, and effect semantics.
6. P14-49–53 — add trivial mutable storage and transactional replacement.
7. P14-54–60 — implement schema 10 and the guard verifier in C++.
8. P14-61–66 — prove predecessor compatibility and source integration.
9. P14-67–70 — lock unsupported and architecture boundaries.
10. P14-71–77 — final acceptance, governance, and implementation report.
