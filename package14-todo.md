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

- [x] P14-18 — Define immutable `ResolvedArrayLiteralEntryKind`, transfer-action,
  entry, and plan facts. Each source element/spread is one entry with exact
  length; the plan owns no duplicate AST nodes.
- [x] P14-19 — Add read-only
  `ResolvedArrayLiteralPlanOf(*ast.ArrayLiteral)` returning defensive copies and
  never re-inferring, mutating Sema, or allocating O(expanded length) records.
- [x] P14-20 — Refactor current `arrayLiteralSegments` analysis into the
  authoritative plan while preserving left-to-right source order, target-shaped
  literals, exact inference, and the target-required empty literal rule.
- [x] P14-21 — Preserve every fixed-array spread as one source entry; evaluate
  its expression once, allow multiple spreads, add exact lengths with `big.Int`,
  and reject runtime-length sources or mismatched element types.
- [x] P14-22 — Record `construct-direct`, `copy-trivial`, and the later action
  vocabulary from compiler-owned copy facts. Accept only the P14 trivial subset
  in the new IR and reject semantic-copy/move/borrow actions explicitly.
- [x] P14-23 — Prove a literal containing a spread of conceptual length above
  `int64` remains O(number of source entries) in Sema and diagnostics.
- [x] P14-24 — Replace expanded fixed-array `DefaultResolution.Elements` as new
  IR authority with one compact exact-length default fact. Preserve old expanded
  data only behind a bounded legacy compatibility path.
- [x] P14-25 — Implement the zero-length exception: `T[0]` is defaultable even
  when T is non-defaultable, and querying/building the default must not construct
  or inspect a T value.
- [x] P14-26 — Add the complete section-91/93 plan and default tests, including
  ordinary/target/inferred/empty literals, multiple spreads, read-only query,
  compact large lengths, nested defaults, non-defaultable elements, and no
  `undef`/poison substitute.

Completed 2026-08-28: P14-18 adds the compiler-owned compact literal-plan data
model, including the full transfer-action vocabulary and defensive copying of
every exact `big.Int` length and entry slice as the fact enters Sema ownership.
The facts are keyed by the original `*ast.ArrayLiteral`, but plan entries store
semantic decisions only, never duplicate AST nodes. P14-19 now exposes the
public read-only query with defensive slice and exact-length copies, nil/unknown
handling, no inference or analyzer mutation, and compact huge-length coverage.
P14-20 now makes the compact plan authoritative during ordinary literal
analysis. Inferred, target-shaped, spread-containing, and target-required empty
literals publish exact source-order plans; fixed-array result typing and current
diagnostics consume the same plan rather than a parallel segment model.
P14-21 and P14-23 preserve multiple spreads as distinct, source-indexed entries,
reject runtime-length and element-mismatched sources, and prove that a spread
longer than host `int64` still produces only one plan entry plus its neighboring
source elements. No test allocates or expands the conceptual array.

Completed 2026-08-29: P14-22 derives spread actions from compiler-owned copy
classification, preserves semantic-copy as a later action, and explicitly
rejects move-only/conditional/non-copyable sources. Semantic IR verifier tests
admit only construct-direct/copy-trivial and reject semantic-copy, move, and
shared/mutable borrow actions at the P14 boundary. P14-26 completes the compact
default matrix for scalar, wide, struct, nested, zero-length non-defaultable,
positive non-defaultable, and huge arrays. Source-to-IR tests prove one compact
`array.default`, no partial module for deferred ownership, and no `undef` or
poison substitute.

## D. Implement fixed arrays in Semantic IR

- [x] P14-27 — Add canonical fixed-array type data keyed by element `TypeID` and
  exact immutable length; intern, compare, clone, print, and verify it without
  host-width conversion.
- [x] P14-28 — Extend the module/type verifier with fixed/dynamic separation,
  canonical non-negative decimal length, supported element type, exact nominal
  nesting, and deterministic diagnostics.
- [x] P14-29 — Add compact array construction segments (`element`/`spread`) and
  `ArrayConstructOp`. Verify operand type, action, segment length, exact total,
  result type, and full initialization.
- [x] P14-30 — Add `ArrayDefaultOp` with one element type and exact length.
  Permit zero length or the supported infallible trivial default subset and
  reject unrepresented cleanup obligations.
- [x] P14-31 — Add `ArrayLengthOp` returning compiler-known target-sized `uint`;
  retain its exact compile-time value as a foldable semantic fact.
- [x] P14-32 — Add `ArrayIndexInBoundsOp` producing the canonical bounds
  predicate from one array value and one source-typed index value.
- [x] P14-33 — Add guarded `ArrayExtractOp` for copy-trivial reads and
  `ArrayReplaceOp` for trivial replacement, carrying exact index-plan proof or
  guard provenance rather than guessing safety from CFG shape.
- [x] P14-34 — Add terminating `BoundsFailureOp` for ordinary failed bounds
  checks and the explicit `IndexError.OutOfBounds` construction/branch for the
  fallible path.
- [x] P14-35 — Extend Semantic IR printer/verifier tests for zero/large lengths,
  nested arrays, wide/struct/enum elements, compact segments/defaults, exact
  result types, bad sums, guard mismatches, and every P14 operation.
- [x] P14-36 — Ensure every P14 `UnsupportedFeatureError` retains package 14,
  returns no partial module, and emits no placeholder operation.

Completed 2026-08-29: P14-27/P14-28 add the first Semantic IR fixed-array
type slice. `TypeArray` is keyed by element `TypeID` plus canonical exact
decimal length, participates in type interning, prints deterministically, and is
verified without host-width conversion. The builder now admits fixed-array
function parameter and return types only at `MaxPackage >= 14`, rejects dynamic
array types at the package boundary, and keeps array construction/index/storage
operations deferred to P14-29 and later.

Completed 2026-08-29: P14-29/P14-30/P14-31 add Semantic IR operation records
for `array.construct`, `array.default`, and `array.len`. The verifier checks
compact element/spread segments, exact segment sums, result type agreement,
construct-direct/copy-trivial P14 actions, zero-length construction, trivial
default eligibility, and target-sized `uint` length results. P14-36 now has a
source-builder boundary test proving non-trivial array literal lowering returns
a Package 14 `UnsupportedFeatureError`, no partial module, and no placeholder
operation; the trivial compact literal path is now connected by P14-50.

Completed 2026-08-29: P14-32/P14-33 add source-typed signed/unsigned bounds
predicates plus functional extract/replace records. Semantic IR verifies exact
array/index/result types, P14 trivial ownership gates, non-empty proven-safe
provenance, and runtime guard provenance. Runtime projections must name a
dominating predicate for the same array and index and remain unreachable from
that predicate's false edge; the verifier never reconstructs a Sema range
proof.

Completed 2026-08-29: P14-34 adds the terminating `fail.bounds` endpoint with
the required `fixed-array-index` operation identity. Guard failure edges now
must end either there or in an explicit `core::IndexError.OutOfBounds` enum
construction followed by the existing typed `Result.err` return path; no
special bounds-error type or runtime symbol is introduced.

Completed 2026-08-29: P14-35 completes the Semantic IR value-layer
printer/verifier matrix. Construction now covers empty, `int32`, `int128`,
`uint256`, nested-array, trivial-struct, enum, and compact above-`uint64`
spread cases with exact result types. Negative tests distinguish result-type
mismatches from exact segment-sum failures, while the combined operation tests
cover defaults, length, bounds predicates, extraction, replacement, guard
provenance, ordinary bounds failure, and fallible `IndexError.OutOfBounds`.

## E. Resolve indexing, bounds control flow, `try`, and effects

- [x] P14-37 — Define immutable `ResolvedArrayIndexPlan` facts with exact array
  type/length, element type, original index type, use kind, check kind,
  proof kind/provenance, and ordinary versus fallible failure mode.
- [x] P14-38 — Add read-only
  `ResolvedArrayIndexPlanOf(*ast.IndexExpression)` and make all new IR consumers
  use it instead of recomputing constant bounds or member syntax.
- [x] P14-39 — Perform constant bounds evaluation with arbitrary precision for
  signed/unsigned and wide constants. Reject negative, N, and greater-than-N at
  compile time; valid constants are proven-safe and emit no failure operation.
- [x] P14-40 — Integrate existing range, branch, assertion/contract, and analysis
  refinements as explicit proof kinds. Zero-length arrays never have a
  proven-valid element index.
- [x] P14-41 — Classify every remaining integer index as runtime-checked while
  preserving its source type through comparison and downstream operations; do
  not normalize it to a hard-coded `i64`.
- [x] P14-42 — Preserve evaluation order and exactly-once behavior: evaluate the
  array/place first and index second; for assignment evaluate target/index,
  then RHS completely, then commit replacement.
- [x] P14-43 — Build the ordinary runtime CFG from one predicate: success
  dominates extraction/replacement and failure terminates in
  `BoundsFailureOp`. Negative signed and `>= N` cases must fail.
- [x] P14-44 — Implement `try array[index]` as a resolved fallible operation:
  success produces T and failure produces precisely `IndexError.OutOfBounds`,
  with compatible naked propagation and direct local handlers.
- [x] P14-45 — Extend compiler-owned try facts with bounds-propagation/handled
  bounds kinds. The backend must not reconstruct fallibility from unused Result
  values or textual `try`.
- [x] P14-46 — Integrate effects: unproven ordinary access adds
  `MayBoundsPanic`; proven-safe access removes that effect; fallible access moves
  bounds failure into the typed error flow while retaining operand effects.
- [x] P14-47 — Enforce `@noPanic` for ordinary dynamic access and accept proven
  or fallible indexing. `unsafe` must never bypass fixed-array bounds checks.
- [x] P14-48 — Add sections 95–98 tests for constant/proven/runtime/fallible
  indexing, wide indexes, zero length, negative signed runtime values,
  exactly-once evaluation, local/catch-all handlers, `@noPanic`, and absence of
  `sec.fail.bounds` on proven/fallible success models.

Completed 2026-08-29: P14-37/P14-38 add immutable, read-only fixed-array
index-plan facts keyed by the original `*ast.IndexExpression`. Plans retain the
exact array length, array/element/index types, signedness, read/write/borrow use,
transfer action, check/proof kind, failure mode, and concrete `IndexError` only
when runtime failure remains possible. Exact mutable integers are defensively
copied and unknown queries perform no inference or analyzer mutation.

Completed 2026-08-29: P14-39/P14-41 make valid constants proven-safe using
arbitrary-precision arithmetic, including `uint256` values above host `int64`,
and reject negative, at-length, and above-length constants exactly. Remaining
signed and unsigned integer indexes are runtime-checked without changing their
source type. Named integer ranges wholly inside `0..<N` are also recorded as
range-proven; this was the initial range slice later completed by P14-40's
branch, assertion/contract, and analysis provenance.

Completed 2026-09-02: P14-40 adds dominating branch and assertion refinement
to the compiler-owned fixed-array index plan. Exact conjunctions/equalities,
reversed comparisons, inclusive/exclusive constant bounds, and the fixed-array
`Len` property can establish the required `0 <= index < N` predicate. Branch,
inline range-contract, and general assertion/analysis provenance remain
distinct through Sema and Semantic IR. Proof matching uses resolved binding
identity rather than identifier spelling; any intervening assignment
conservatively invalidates the active value proof. Tests retain runtime checks
for incomplete/disjunctive proofs, proof use outside its dominating branch,
post-mutation access, and every zero-length access. A source-built Semantic IR
regression proves that accepted branch provenance emits `array.extract`
without a redundant array bounds predicate or `fail.bounds` path.

Completed 2026-08-29: P14-43 connects ordinary fixed-array reads from their
compiler-owned index plans to verified Semantic IR. Proven-safe reads emit one
`array.extract` with the recorded proof and no runtime CFG. Runtime reads
evaluate array then index exactly once, emit one `array.index-in-bounds`, branch
to a guarded extraction, and terminate the false path in `fail.bounds` for
`fixed-array-index`; the existing verifier applies the same guard contract to
`array.replace`. Signed `int128`, unsigned `uint128`, target `uint`, and
zero-length runtime cases retain their source index semantics.

Progress 2026-08-29: the read half of P14-42 became source-to-IR tested with
effectful array/index calls in exact source order. P14-46 now records direct and
transitive `may-panic-bounds` effects for ordinary runtime indexes while
constant/range-proven indexes remain bounds-panic-free. P14-46/P14-48 were
still open at this checkpoint for fallible indexing, handlers, `@noPanic`, and
the remaining section-95–98 matrix.

Completed 2026-08-30: P14-44/P14-45 make runtime fixed-array indexing a
compiler-resolved fallible source under `try`. Sema records distinct
`bounds-propagation` and `handled-bounds` try kinds, changes the index plan from
ordinary panic failure to typed `IndexError`, and removes the bounds-panic
effect. Semantic IR evaluates array/index once, branches on the shared bounds
predicate, extracts only on success, and materializes exactly
`IndexError.OutOfBounds` for naked `Result` propagation or the existing local
handler engine. No `fail.bounds` is emitted on either fallible path.

Completed 2026-08-31: P14-46 now covers the full bounds-effect transition.
Ordinary unproven indexes contribute direct and transitive
`may-panic-bounds`; constant/range-proven and validated fallible indexes do
not. Fallible propagation and local handlers retain panic effects from array
and index operand calls and handler bodies. Invalid `try` constructs retain the
ordinary bounds effect and therefore cannot become false positive evidence for
panic freedom.

Completed 2026-08-31: P14-47 adds parsed, argument-free `@noPanic` attributes
for functions and methods and verifies them against the compiler-owned
transitive call-graph effect summary. Proven-safe and otherwise panic-free
fallible fixed-array access is accepted. Ordinary dynamic access, panic-capable
fallible operands, and the same access inside `unsafe` are rejected with the
introducing effect location and synchronous call chain.

Completed 2026-08-30: P14-42 now covers both reads and transactional writes.
Simple and nested mutable paths evaluate the root/place and each index once,
complete every required bounds guard before evaluating the RHS, evaluate the
RHS once only on the successful path, and commit no destination mutation until
the functional replacement value is complete.

Completed 2026-09-02: P14-48 closes the sections-95–98 matrix. New focused
tests cover zero and `N-1`, constant `int128`/`uint128`/`uint256`, a semantic
length above `int64`, and compile-time invalid rejection without a partial
runtime-failure module. Runtime tests cover a signed negative-capable index,
exact `N`, greater than `N`, unsigned flow, one guard/extraction/failure path,
and exact guard array/index SSA reuse; the existing zero-length and
array-before-index exactly-once cases remain explicit. Fallible source tests now
cover `Result[int32, IndexError]`, `Result[int128, IndexError]`, the exact
`OutOfBounds` fallback, `Err(_)`, and absence of `fail.bounds`. Existing focused
tests retain assertion/branch/range provenance, operand effects, and
`@noPanic`. The focused matrix, full `go test ./...`, and `go vet ./...` pass.

## F. Add trivial mutable storage and nested replacement

- [x] P14-49 — Extend P5 high-level storage eligibility to copy-trivial,
  trivially destructible fixed arrays whose element representation is already
  supported. Never memref-lower the wrapper in P14.
- [x] P14-50 — Lower defaulted and explicit mutable local fixed arrays through
  semantic storage declare/init/load/store operations with no physical layout.
- [x] P14-51 — Implement indexed assignment as RHS-first whole-array load,
  guarded/proven `ArrayReplaceOp`, and exactly one root store.
- [x] P14-52 — Rebuild nested array/struct paths leaf-to-root using P13 struct
  replacement and P14 array replacement, preserving the exact root type and one
  commit.
- [x] P14-53 — Add section-99 tests for scalar/wide replacement, runtime guard,
  nested arrays/structs, RHS exactly once/before commit, one root store,
  high-level storage, and explicit non-trivial rejection.

Completed 2026-08-30: P14-49/P14-50 restrict P5 high-level array storage to
recursively copy-trivial, trivially destructible fixed-array values and reject
even zero-length storage when the element ownership representation is deferred.
The Semantic IR builder now consumes compact literal plans directly, emits one
`array.construct` segment per source element/spread, and supports explicit,
defaulted, spread-containing, and nested mutable fixed-array locals through
`storage.declare/init/load`. Tests prove non-trivial rejection returns no
partial module and the successful representation contains no MemRef,
`llvm.array`, `undef`, or poison substitute.

Completed 2026-08-30: P14-51 lowers simple-root mutable local indexed
assignment transactionally. Proven-safe updates evaluate the index and RHS,
then load, replace, and commit once with explicit proof provenance. Runtime
updates evaluate the index once, validate through one array/index predicate,
evaluate the RHS only on the success path, reuse the exact guarded array/index
SSA pair in `array.replace`, and perform exactly one `storage.store`; failure
terminates in `fail.bounds`. Nested aggregate paths remain P14-52.

Completed 2026-08-30: P14-52/P14-53 add one compiler-owned nested aggregate
path from a mutable local root through stored struct fields and fixed-array
indexes. Bounds predicates and guarded extracts are emitted root-to-leaf;
`array.replace` and P13 `struct.replace-field` rebuild leaf-to-root before one
root store. The section-99 matrix covers `int32`, `int128`, two-dimensional
arrays, struct-in-array, array-in-struct, multiple runtime guards, RHS ordering,
one store, high-level storage, and the explicit non-trivial storage boundary.

## G. Implement Sec MLIR schema 10 and verification

- [x] P14-54 — Bump generated modules to schema 10 only after schema 9 remains a
  checked compatibility input; update ODS/C++ registration and version gates.
- [x] P14-55 — Implement `!sec.array<T, "N">` with canonical arbitrary-precision
  decimal `StringAttr`; reject empty, signed, leading-zero, negative, or otherwise
  non-canonical length spelling except canonical `"0"`.
- [x] P14-56 — Implement `sec.array.construct` with compact ordered segment
  metadata/actions and exact C++ verification of operands, spread types, and sum.
- [x] P14-57 — Implement `sec.array.default`, `sec.array.len`,
  `sec.array.index_in_bounds`, `sec.array.extract`, `sec.array.replace`, and
  terminating `sec.fail.bounds` with exact schema-v10 verifiers.
- [x] P14-58 — Register `--sec-verify-array-index-guards`. Verify dominance,
  true-path use, same array/index SSA, runtime versus proven-safe modes, and
  mandatory proof provenance for unchecked proven-safe operations.
- [x] P14-59 — Extend the Go schema-10 emitter from verified Semantic IR;
  preserve exact lengths, source segment order, actions, proof kinds, locations,
  target index types, and high-level storage.
- [x] P14-60 — Add all section-100/101 MLIR round-trip and invalid tests,
  including large lengths, multiple segments, bad sums/types, all operations,
  guard failures, proven-safe provenance, and explicit schema-9 regression.

Completed 2026-08-30: P14-55 registers the high-level `!sec.array<T, "N">`
type in ODS/C++, preserves the exact arbitrary-precision length as canonical
decimal `StringAttr`, admits zero, huge, nested-array, and struct-element forms,
and rejects empty, signed, whitespace-padded, leading-zero, storage, and
function-element forms. At that stage schema-9 emission remained unchanged;
P14-54 later opened schema-10 generation with the complete array operation set.

Completed 2026-08-30: P14-56 registers pure `sec.array.construct` with one
operand per source segment and exact `segment_kinds`, `segment_lengths`, and
`segment_actions` arrays. Its C++ verifier enforces element/spread types and
actions, canonical arbitrary-precision lengths, exact spread-length identity,
and an overflow-free exact segment sum equal to the result array length; empty
and above-`uint64` constructions remain compact.

Completed 2026-09-01: P14-57 registers all remaining schema-10 fixed-array
operations. Local C++ verifiers enforce array/element/result identity, resolved
integer index types and signedness, target-width unsigned length results,
`copy-trivial`, the exact proven/runtime bounds metadata vocabulary, functional
replacement identity, and terminating `fixed-array-index` bounds failure. The
focused round-trip and negative suite covers every operation; cross-block guard
dominance and true-edge validation remain exclusively P14-58.

Completed 2026-09-02: P14-58 registers the dedicated fixed-array index-guard
pass. Runtime extract/replace operations now require a single-use matching
`array.index_in_bounds` predicate on the same array and index SSA values, an
immediately following conditional branch, and a dedicated true successor that
dominates the operation. This rejects missing, mismatched, false-edge, and
post-merge guards. Proven-safe operations require one of the closed
compiler-owned proof kinds and do not require a runtime predicate.

Completed 2026-09-02: P14-54/P14-59 open compiler-generated schema 10 after the
complete verified array operation set and guard pass became available. The Go
emitter now maps exact nested array types, compact construction segments,
defaults, semantic length, signed/unsigned bounds predicates, proven and
runtime-guarded extraction/replacement, terminating bounds failure, locations,
and high-level array storage without choosing physical layout. It independently
checks every nested exact length against the selected target `uint`. Generated
modules verify through `sec-mlir-opt` on 32- and 64-bit plans; schema 9 remains
a passing compatibility input and schema 11 remains rejected.

Completed 2026-09-02: P14-60 audits the complete section-100/101 matrix across
the schema-10 array type, construction and operation suites plus the dedicated
guard-pass suites. Coverage includes zero and arbitrary-precision lengths,
canonical spelling failures, compact element/spread segments and bad sums or
types, every array operation, both missing extract/replace guards, false-edge
and non-dominating use, distinct array/index SSA values, all proven-safe proof
classes, missing proof provenance, and an accepted schema-9 compatibility
module.

## H. Prove package compatibility and source-to-schema-10 integration

- [x] P14-61 — P6: recursively resolve target-sized `int`/`uint` in array
  elements for 32- and 64-bit plans while preserving array length, nesting,
  operations, tags, and surrounding P13 struct wrappers.
- [x] P14-62 — P8: prove checked-integer signless normalization neither recurses
  through `!sec.array` nor lowers array operations.
- [x] P14-63 — P13: cover array-in-struct, struct-in-array, nested replacement,
  defaults, wide fields/elements, and exact nominal identities in both directions.
- [x] P14-64 — P11/P12: allow supported trivial fixed arrays in union payloads
  and match values without introducing array patterns or weakening union guards.
- [x] P14-65 — Add source-to-verified-Semantic-IR-to-schema-10 tests for every
  section-102 case: literals/spreads/defaults/zero/nesting/functions/Len,
  constant and dynamic ordinary/fallible indexing, handlers, and replacement.
- [x] P14-66 — Run the same source-built schema-10 module through an absolute
  `sec-mlir-opt` path on 32- and 64-bit CompilationPlans and assert no unresolved
  casts, physical array layout, runtime helper, or LLVM dialect.

Completed 2026-09-01: P14-61 extends the Package 6 scalar-layout type
conversion recursively through `!sec.array` and includes `sec.array.construct`
in operation type conversion. Focused 32- and 64-bit MLIR regressions preserve
canonical array lengths, nesting, construction attributes, struct identities,
type arguments, field tags, and surrounding P13 struct wrappers while resolving
only target-sized `!sec.int`/`!sec.uint` elements to signed/unsigned target-width
integers. The pass introduces neither physical array layout nor unrealized
conversion casts. These focused handwritten pass inputs deliberately omit the
generated-module schema attribute; P14-54 separately proves that generated
modules now carry schema 10.

Completed 2026-09-01: P14-62 adds a mixed Package 8 regression where an ordinary
checked signed addition is lowered to signless Arith while zero-length, spread,
and nested fixed arrays retain their signed/unsigned element types, exact
lengths, and high-level `sec.array.construct` operations. The result contains no
array-wrapper signless normalization, unrealized cast, or LLVM operation.

Completed 2026-09-01: P14-63 strengthens the source-built Semantic IR matrix in
both P13 nesting directions. `main::Pair[2]` retains the exact nominal Pair
element identity with an `int128` field, while `main::Holder` retains an
`int128[2]` field plus `uint256` metadata through extract, guarded replacement,
leaf-to-root rebuild, and one root commit. Recursive default coverage now proves
that a Holder default contains one compact `array.default` inside one
`struct.construct`; the existing struct-in-array, nested-array, and wide default
cases remain compact and verified.

Completed 2026-09-01: P14-64 proves the existing P11/P12 value path accepts a
copy-trivial `int128[2]` as a single union payload and as the result of ordinary
and guarded match expressions. Construction and projection retain
`copy-trivial`, every match arm remains an ordinary `union-variant` pattern,
array-valued fallback arms use compact `array.construct`, and a negative
mutation confirms the existing matching-variant guard still rejects an array
payload projection on the wrong path.

Completed 2026-09-02: P14-65 adds one unedited Sec source fixture that passes
through parsing, target-aware Sema, Package-14 Semantic IR construction and
verification, and schema-10 emission. It covers literals, compact spread,
zero/default arrays, both P13 nesting directions, nested arrays, array
parameters/results, `Len`, constant and dynamic ordinary indexing, fallible
propagation, a local `IndexError.OutOfBounds` handler, trivial replacement, and
nested replacement. The matrix exposed and closed the missing source-builder
bridge from the compiler-known fixed-array `Len` fact to `ArrayLengthOp`.

Completed 2026-09-02: P14-66 emits that same verified source-built module for
both 32-bit and 64-bit CompilationPlans, invokes `sec-mlir-opt` through the
required absolute `SEC_MLIR_BIN` path, verifies array-index guards, and runs
Package 6 scalar-layout resolution. The integration exposed and closed missing
type-conversion registration for `array.default`, `array.len`,
`array.index_in_bounds`, `array.extract`, and `array.replace`. Both outputs keep
schema 10, exact high-level array types and operations while rejecting unresolved
semantic scalars/casts, MemRef or LLVM array layout, GEPs, allocation/runtime
helpers, and the LLVM dialect.

## I. Prove unsupported boundaries are explicit and safe

- [x] P14-67 — Add source-to-Semantic-IR rejection tests for move-only and
  semantic-copy spread, move-only element reads, element borrows, move-out,
  non-trivial replacement/destruction, dynamic owning arrays, slices, and
  array-to-slice conversion.
- [x] P14-68 — Reject equality/membership lowering, foreign fixed-array ABI, and
  any physical layout request at the P14 package boundary while leaving their
  already-valid frontend typing or legacy path intact where applicable.
- [x] P14-69 — Assert successful P14 modules contain no `undef`, poison, partial
  readable array, hidden allocation, MemRef array layout, `!llvm.array`, GEP,
  hard-coded semantic `i64` index, LLVM dialect, or mandatory runtime call.
- [x] P14-70 — Prove determinism: canonical decimal lengths, type/segment/source
  order, nested printing, diagnostics, and 32/64 multi-output validation must be
  independent of host word size and map traversal.

Completed 2026-09-02: P14-67 adds maintained invalid source fixtures and one
pipeline matrix for move-only spread/read, shared and mutable element borrow,
indexed move-out, non-trivial replacement/destruction, owning dynamic arrays,
slices, and array-to-slice creation. Move-only spread/read and indexed move-out
are diagnosed by Sema and therefore by the LSP, including indexed move-only
reads wrapped by the dedicated terminal `Ok(...)` Result-return path;
valid-but-deferred forms return
Package-14 `UnsupportedFeatureError`, and no failed build exposes its partially
assembled module. Element borrow and move-out also have defensive builder gates,
and owned non-trivial fixed-array
parameters retain honest ownership until a specific operation or missing-cleanup
boundary rejects them. Current Sema has no source-reachable `CopySemantic` type,
so semantic-copy spread is locked directly at the immutable action-to-IR adapter
alongside move and borrow actions rather than being fabricated as source syntax.

Completed 2026-09-02: P14-68 preserves valid frontend typing for fixed-array
`==`, `!=`, and `in`, then rejects them by name at the Package-14 Semantic IR
operator boundary without exposing a partial module. Schema-10 emission also
rejects verified foreign functions with fixed arrays in either parameter or
result position, plus physical `LayoutRef` requests on structs or unions that
contain a fixed array. The boundary walks semantic type identity only; it does
not calculate or invent array size, alignment, stride, storage, or ABI shape.

Completed 2026-09-02: P14-69 runs the maintained section-102 source module
through verified Semantic IR and schema-10 emission on both 32-bit and 64-bit
plans. Every compact construction is independently checked for exact complete
segment coverage and P14-supported transfer actions, while defaults remain
compact and operand-free. The emitted modules reject unreadable placeholders,
hidden allocation, MemRef/LLVM array layout, insertvalue/GEP shortcuts,
signless hard-coded `i64` array indexes, traps, runtime symbols, and helper calls.

Completed 2026-09-02: P14-70 rebuilds the complete section-102 source fixture
twelve times with fresh parser, Sema, Semantic IR builder, type table and
internal maps. Permuted and duplicated source-file metadata canonicalizes to one
lexicographic order; function, type, nested-type and compact spread segment
order remains byte-identical. Alternating 32/64 emission order produces stable
per-plan schema-10 bytes without mutating the shared Semantic IR. A separate
target-width matrix rebuilds invalid array declarations and proves exact decimal
diagnostic text and source order remain stable while the expected uint32/uint64
differences stay independent.

## J. Final acceptance, governance, and report

- [x] P14-71 — Map every section-107 acceptance item to a focused test or exact
  implementation location. Do not mark the package implemented from aggregate
  test success alone.
- [x] P14-72 — From a fresh isolated worktree containing the exact P14 diff, run
  `go test ./...` and `go vet ./...`; record commands, HEAD, and results.
- [x] P14-73 — Run the full `check-sec-mlir` target and every adjacent schema
  regression with the exact LLVM/MLIR version recorded.
- [x] P14-74 — Run the package-specific 32/64 source → Sema → verified Semantic
  IR → schema-10 → verifier pipeline with an absolute tool path.
- [x] P14-75 — Update `lowering.sec-mlir-package14` with the exact integrated
  surface, deferred P15/P16/P17 boundaries, commands, results, audited HEAD, and
  `status: implemented` only when every section-107 criterion is satisfied.
- [x] P14-76 — Write the mandatory section-108 implementation report covering
  all 41 requested topics, deviations, and Package 15 recommendations.
- [x] P14-77 — Re-run `git diff --check`, unique-key YAML validation, complete Go
  tests with absolute `SEC_MLIR_BIN`, and the full MLIR suite before handoff.

Completed 2026-09-02: P14-71 adds
`rules/mlir/packages/sec-mlir-dialect_package14-acceptance-matrix.md`, mapping
all 56 section-107 criteria to named focused tests or exact implementation and
rule locations. Aggregate suite results are evidence only for the three
criteria that explicitly concern repository-wide regression gates. At that
checkpoint Package 14 stayed `partial`: P14-48 was still open, and P14-72–77
owned isolated validation, governance closure, the mandatory report, and final
handoff; those steps are now complete below.

Completed 2026-09-02: P14-72 built an isolated detached snapshot from audited
HEAD `475add863959208ea2af90dfe0c8755352617ec8` plus the exact pending
P14-70/P14-71 files. The resulting snapshot commit was
`8b2266b8da6d18c9daaf42eb20fe86bda8d743fd`, with tree
`42802a3733a0d5032390ea355e14c6d2ebed814f`. Its worktree was clean before and
after `go test ./...` and `go vet ./...`; both commands passed. An initial
invocation that accidentally remained in the primary worktree is deliberately
excluded from the recorded result.

Completed 2026-09-02: P14-73 records `sec-mlir-opt` as LLVM
`24.0.0git`, optimized with assertions, and `llvm-lit` as `24.0.0dev`.
`cmake --build build/sec-mlir --target check-sec-mlir -j2` passed all 91
tests. The adjacent schema-9/10/11 set was then selected explicitly with
`llvm-lit --filter='schema(9|10|11)'` and passed all 17 selected regressions,
including valid and invalid dialect metadata, array types/operations/guards,
P6 scalar-layout conversion, P8 normalization boundaries, and schema-9
compatibility.

Completed 2026-09-02: P14-74 ran
`TestEmitPackage14SourceModuleVerifiesOn32And64BitPlans` with the absolute
`SEC_MLIR_BIN=~/small-projects/sec/build/sec-mlir/bin`. Both the
32-bit and 64-bit subtests passed through source parsing, target-aware Sema,
verified Semantic IR, schema-10 emission, array-index guard verification, and
scalar-core lowering. The outputs retained high-level fixed arrays and every
required schema-10 array operation, resolved target `uint` independently, and
contained no unresolved semantic scalar/cast, MemRef/LLVM layout, allocation,
runtime symbol, or helper path.

Completed 2026-09-02: P14-75 promotes
`lowering.sec-mlir-package14` from `partial` to `implemented` only after P14-48
closed the final focused test gap and the acceptance matrix accounted for all
56 section-107 criteria. Governance now records audited repository HEAD
`475add863959208ea2af90dfe0c8755352617ec8` plus the pending P14-48 and
P14-70–75 diff,
the exact frontend/Sema, Semantic IR, schema-10, lowering, compatibility, and
safety-boundary surfaces, LLVM/lit versions, commands, and results. P15 owns
element Places/references, P16 owns slices and array-to-slice borrowing, and
P17 owns move-out, semantic-copy, and non-trivial array ownership/destruction.
P14-76 and P14-77 were report and final-validation work, not missing Package
14 implementation; both are now complete below.

Completed 2026-09-02: P14-76 adds
`rules/mlir/packages/sec-mlir-dialect_package14-implementation-report.md` with
all 41 section-108 topics in the required order. It records the audited
HEAD/P13 basis, exact P14 file and implementation surfaces, array semantics,
Sema and Semantic IR APIs, schema 10 and compatibility, tests and commands,
toolchain/results, explicit deviations, and the Package 15 recommendation.

Completed 2026-09-02: P14-77 validated the complete Package 14 handoff diff,
including the previously untracked acceptance matrix and implementation report,
with an isolated temporary Git index and `git diff --cached --check HEAD`.
`implementation-status.yaml` parses with 164 unique integration IDs and the
mandatory report has exactly 41 ordered items. With
`SEC_MLIR_BIN=~/small-projects/sec/build/sec-mlir/bin`, the complete
`go test ./...` suite passed; `go vet ./...` also passed. Finally,
`cmake --build build/sec-mlir --target check-sec-mlir -j2` passed all 91 Sec
MLIR regressions. Package 14 has no open TODO item.

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
