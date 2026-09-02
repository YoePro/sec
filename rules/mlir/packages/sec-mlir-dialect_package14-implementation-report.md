# SEC MLIR Package 14 implementation report

This report closes the mandatory reporting contract in
`sec-mlir-dialect_package14.md` section 108. It describes the implementation
audited on 2026-09-02. The section-107 evidence index is maintained separately
in `sec-mlir-dialect_package14-acceptance-matrix.md`.

1. **Repository HEAD.** Package 14 was audited against repository HEAD
   `475add863959208ea2af90dfe0c8755352617ec8` plus the pending P14-48 and
   P14-70 through P14-77 completion diff. The implementation started from the
   recorded P14 baseline `069c111b96714ee4c9423f5f7a64d1e7a6045f31`, which
   is newer than the rulebook's minimum `152c772` baseline.
2. **Package 13 source.** The baseline contains the merged Package 13 schema-9
   implementation. Package 14 did not depend on a private or uncommitted P13
   patch; schema-9 structs and high-level aggregate storage were already the
   predecessor contract in the audited repository.
3. **Previous packages.** Packages 1 through 13 remain operational. The full
   Go suite passes, the full Sec MLIR suite passes 91/91, and the explicit
   adjacent schema-9/10/11 group passes 17/17. Schema 9 remains accepted while
   compiler-generated Package 14 modules use schema 10.
4. **Array/spread rules.** `rules/collections/collections.md` and
   `rules/declarations/spread.md` permit one or more fixed-array literal spreads
   with compile-time lengths. The implementation retains one compact Sema and
   IR segment per source spread in left-to-right order.
5. **IndexError rules.** `rules/errors/runtime_checks.md` and
   `rules/library/core-library.md` name `core::IndexError.OutOfBounds` as the
   single typed fixed-index error. Ordinary access remains panic-capable;
   fallible access constructs that exact enum case rather than a parallel
   `BoundsError`.
6. **Files added.** P14-specific additions are `package14-todo.md`, this report,
   the section-107 acceptance matrix, `internal/sema/array_shape_test.go`,
   `internal/ir/semantic/package14_test.go`, the fixed-array Sema/source
   fixtures under `testdata/semantic_ir/`, the eleven unsupported fixtures
   under `testdata/semantic_ir/package14_unsupported/`, the schema-10 dialect
   and metadata tests, the two array-index guard tests, the two P6 scalar-layout
   tests, and the P8 array-boundary test. The exact maintained paths are listed
   by `lowering.sec-mlir-package14.tests` in `implementation-status.yaml`.
7. **Files modified.** The implementation modifies array-related AST/parser
   handling, Sema types/analyzer/defaults/semantic facts/effects/Places and
   compiler-known members, Semantic IR model/builder/printer/verifier, the Go
   Sec MLIR emitter, the checked legacy generator boundary, Sec ODS/C++ type and
   operation registration/verifiers, the analysis pass registry and guard
   verifier, P6 scalar conversion, LSP diagnostics/tests, focused Go tests,
   `package14-todo.md`, and `implementation-status.yaml`. The exact code list is
   recorded by `lowering.sec-mlir-package14.code`; unrelated changes since the
   baseline are not attributed to Package 14.
8. **Sema array shape.** `ArrayShapeKind` explicitly distinguishes `fixed` and
   `dynamic`. A fixed array owns a canonical exact decimal length; a dynamic
   owning array owns no fixed length. Non-array types use neither fact, and new
   correctness decisions do not infer dynamic shape from a negative sentinel.
9. **Arbitrary-precision length.** `NewFixedArrayType` canonicalizes a
   non-negative `big.Int` into immutable decimal `ArrayLengthDecimal`.
   `FixedArrayLength` returns a defensive `big.Int` copy. Identity, display,
   generics, defaults, plans, diagnostics, Semantic IR, and MLIR preserve the
   exact value without host-width narrowing.
10. **Target-`uint` validation.** `ValidateArrayTypeForScalarPlan` recursively
    validates each exact length against the selected compilation plan's 32- or
    64-bit unsigned maximum. One semantic module can therefore be accepted for
    a 64-bit output and rejected for a 32-bit output deterministically without
    mutating shared facts.
11. **Legacy `int64` compatibility.** Parsed length syntax and the exact Sema
    value are authoritative. The old `int64` cache is populated only after an
    explicit representability check, and dynamic-to-`-1` translation is
    isolated behind the legacy compatibility helper. The legacy physical MLIR
    generator reports an unsupported-layout error above `int64` instead of
    changing source-language validity.
12. **`ResolvedArrayLiteralPlan`.** Sema publishes immutable element type,
    exact total length, and source-ordered entries carrying source index, entry
    kind, exact contribution length, resolved type, and transfer action.
    `ResolvedArrayLiteralPlanOf` returns defensive copies, performs no
    inference, and does not mutate the analyzer.
13. **Compact spread.** Each fixed-array spread is evaluated once and remains
    one plan/IR operand regardless of conceptual length. Multiple spreads retain
    source order; exact `big.Int` addition checks the target length. Runtime
    length, element mismatch, and deferred ownership actions reject explicitly.
14. **`ArrayConstructOp`.** Semantic IR `array.construct` consumes one ordered
    element/spread operand per compact plan entry and records kind, exact length,
    and action arrays. Verification checks operand/action/type agreement and an
    exact arbitrary-precision sum equal to the result length, guaranteeing full
    initialization.
15. **`ArrayDefaultOp`.** `array.default` represents one compact infallible
    fixed-array default. It accepts the supported recursively trivial subset and
    the zero-length exception, rejects unrepresented cleanup obligations, and
    never substitutes `undef`, poison, or a partially readable aggregate.
16. **`ArrayLengthOp`.** `array.len` consumes one fixed-array value, returns the
    compiler-known target-sized `uint`, and retains its exact decimal foldable
    value. Emission independently checks target-`uint` representability for the
    selected plan.
17. **`ResolvedArrayIndexPlan`.** The immutable read-only Sema fact preserves
    exact array length, array/element/index types, signedness, read/write/borrow
    use, transfer action, check/proof kind, optional exact constant, ordinary or
    fallible failure mode, and the resolved `IndexError` type.
18. **Bounds classification.** Exact constants use arbitrary-precision
    `0 <= I < N` checking. Valid constants, wholly contained named ranges, and
    dominating branch/assertion/contract/analysis refinements become
    `proven-safe` with closed provenance; all other integer indexes remain
    `runtime-check` in their original signed or unsigned source type.
19. **`ArrayIndexInBoundsOp`.** The total semantic predicate consumes the exact
    array/index SSA pair and records source signedness. Runtime control flow uses
    its one result for the success/failure branch; no hard-coded signless `i64`
    normalization is introduced.
20. **`ArrayExtractOp`.** Trivial element reads carry `copy-trivial`, exact
    result type, check kind, and either proof provenance or the matching runtime
    guard result. Verification rejects missing/wrong guards, mismatched array or
    index SSA, false-edge access, and unsupported ownership actions.
21. **`ArrayReplaceOp`.** Trivial indexed mutation is functional: it accepts an
    array, index, and exact element value and returns the same array type.
    Simple and nested assignments guard before the RHS, evaluate the RHS once,
    rebuild leaf-to-root, and commit exactly one root storage store.
22. **Ordinary bounds CFG.** Unproven ordinary access evaluates array/place
    before index exactly once, computes one bounds predicate, extracts or
    replaces only on the true path, and sends the false path to terminating
    `fail.bounds operation="fixed-array-index"`. Negative signed, `I == N`,
    `I > N`, and zero-length runtime cases share this model.
23. **Fallible `IndexError` CFG.** `try array[index]` shares the same predicate
    and guarded success extraction. Its failure block constructs precisely
    `core::IndexError.OutOfBounds`; it either produces the enclosing typed
    `Result.err` or enters resolved local handlers. It contains no
    `BoundsFailureOp`/`sec.fail.bounds` endpoint.
24. **Local and naked `try`.** Sema records distinct `bounds-propagation` and
    `handled-bounds` try kinds. Naked propagation works for
    `Result[int32, IndexError]` and `Result[int128, IndexError]`; local handling
    supports the exact `Err(IndexError.OutOfBounds)` arm and exhaustive
    `Err(_)` payload discard without reconstructing intent from syntax later.
25. **Effects.** Unproven ordinary indexing contributes direct and transitive
    `MayBoundsPanic`. Proven-safe access contributes none. Valid fallible access
    moves only bounds failure into typed error flow while retaining panic effects
    from array/index operands and handler bodies. `@noPanic` accepts proven and
    otherwise panic-free fallible cases, rejects dynamic ordinary access, and
    is not bypassed by `unsafe`.
26. **Schema 10.** ODS/C++ registers high-level
    `!sec.array<T, "N">`, `sec.array.construct`, `sec.array.default`,
    `sec.array.len`, `sec.array.index_in_bounds`, `sec.array.extract`,
    `sec.array.replace`, and terminating `sec.fail.bounds`. Type and operation
    verifiers enforce canonical decimal lengths, types, actions, sums,
    signedness, proof vocabulary, and result identity.
27. **Index guard verifier.** Registered pass
    `--sec-verify-array-index-guards` requires a matching single-use predicate,
    immediate conditional branch, same array/index SSA values, dedicated true
    successor dominance, and no false/rejoined reachability. Proven operations
    instead require one of the closed proof-provenance values.
28. **P5/P6/P8 compatibility.** P5 retains trivial fixed-array storage as
    high-level `!sec.storage<!sec.array<...>>`. P6 recursively resolves target
    `int`/`uint` through arrays, nesting, operations, and P13 structs for 32/64
    plans while preserving wrappers. P8 normalizes checked scalar arithmetic
    without recursing into or lowering an array wrapper or operation.
29. **P13 nesting.** Tests cover struct-in-array and array-in-struct values,
    nominal identities, `int128`/`uint256` fields, compact recursive defaults,
    guarded nested extraction/replacement, leaf-to-root rebuild, and one root
    commit. P11/P12 additionally retain guarded union payload and match-value
    behavior for trivial fixed arrays.
30. **Wide arrays.** Matrices cover `int128`, `uint128`, `uint256`, wide enum
    metadata, wide struct fields, parameters/results, literal segments,
    defaults, extraction/replacement, signed and unsigned indexes, and target
    `int`/`uint` resolution on both compilation plans.
31. **Large-length compactness.** Lengths above host `int64` and above
    `uint64` remain exact in semantic identity, Sema plans, Semantic IR, and
    schema text. Huge spreads remain O(source entries), huge defaults remain one
    operation, exact sums use arbitrary precision, and no test allocates the
    conceptual array.
32. **Zero length.** `T[0]` has distinct exact identity, accepts empty
    construction, and is defaultable even when `T` is not. Default resolution
    does not inspect or construct an element. No constant index is valid, and a
    dynamic index remains guarded with a terminating ordinary failure path.
33. **Bounds tests.** Sections 95–98 cover zero/last/wide constants, exact
    invalid constants without partial IR, named-range/branch/assertion proofs,
    signed/unsigned runtime access, negative-capable/at-length/above-length and
    zero-length paths, exactly-once evaluation, SSA guard reuse, ordinary
    failure, typed fallibility, local/catch-all handlers, effects, and
    `@noPanic`.
34. **Unsupported ownership.** Maintained source fixtures reject move-only and
    semantic-copy spread boundaries, move-only reads, indexed move-out,
    shared/mutable element borrow, non-trivial replacement/destruction, dynamic
    owning arrays, slices, and array-to-slice creation without partial Semantic
    IR. Equality/membership lowering, foreign array ABI, and physical layout
    requests also reject explicitly at their Package 14 boundary.
35. **CMake and lit commands.** The full command is
    `cmake --build build/sec-mlir --target check-sec-mlir -j2`. Adjacent schemas
    were rerun with
    `/home/jonas/mlir/llvm-project/build/bin/llvm-lit -sv
    --filter='schema(9|10|11)'
    /home/jonas/small-projects/sec/build/sec-mlir/test`. P14-77 repeated the full
    CMake target after the complete report/governance diff and passed 91/91.
36. **LLVM/MLIR version.** `build/sec-mlir/bin/sec-mlir-opt --version` reports
    LLVM `24.0.0git`, optimized build with assertions. The associated runner is
    lit `24.0.0dev` from `/home/jonas/mlir/llvm-project/build/bin/llvm-lit`.
37. **MLIR result.** P14-73 passed all 91 checked-in Sec MLIR tests and all 17
    explicitly selected adjacent schema-9/10/11 tests. After this report was
    installed, the P14-77 final full run again passed 91/91; the exact command
    and result are recorded in `implementation-status.yaml`.
38. **Go result.** A clean isolated snapshot passed `go test ./...` and
    `go vet ./...` for P14-72. The complete suite also passed after the final
    P14-48 test matrix. P14-77 then passed the complete Go suite with absolute
    `SEC_MLIR_BIN=/home/jonas/small-projects/sec/build/sec-mlir/bin`; the final
    `go vet ./...` run also passed.
39. **End-to-end result.** With absolute
    `SEC_MLIR_BIN=/home/jonas/small-projects/sec/build/sec-mlir/bin`, one
    unedited source fixture passes parsing, target-aware Sema, verified Semantic
    IR, deterministic schema-10 emission, array-index guard verification, and
    scalar-core lowering on both 32- and 64-bit plans. Outputs retain high-level
    arrays and contain no unresolved cast, MemRef/LLVM layout, allocation,
    runtime helper, or LLVM dialect.
40. **Deviations.** No current source-reachable type resolves to
    `CopySemantic`, so that spread rejection is tested at the immutable
    action-to-IR adapter. Assertion provenance is executable in Sema; the P14
    Semantic IR consumes the resulting proof but does not add a general assert
    operation. Runtime negative behavior is represented by the preserved signed
    predicate and guarded CFG because P14 deliberately selects no executable
    physical array layout. Bounded legacy AST/default and physical-generator
    compatibility remains isolated and is not semantic authority.
41. **Package 15 recommendations.** Build one canonical `PlaceID`/place-path
    layer for locals, struct fields, fixed-array elements, mutability,
    generations, origins, shared/mutable references, and reborrows. Reuse the
    P14 array/index facts and guard dominance rather than parsing member/index
    syntax again. Keep slices and array-to-slice borrowing in Package 16,
    move/copy/destruction actions in Package 17, and physical pointer/layout or
    LLVM choices outside the Package 15 semantic reference contract.
