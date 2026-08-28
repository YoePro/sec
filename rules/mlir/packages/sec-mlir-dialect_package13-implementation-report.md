# SEC MLIR Package 13 implementation report

This report closes the mandatory reporting contract in
`sec-mlir-dialect_package13.md` section 91. It describes the implementation
audited on 2026-08-28.

1. **Repository HEAD.** The implementation was audited against
   `e3d17698d756678cfb94bd9f8df1f11977635375` plus the pending Package 13
   completion diff. This is newer than the required `152c772` sync baseline.
2. **Previous packages.** Packages 1 through 12 remain operational. The full Go
   suite and all 78 checked-in Sec MLIR regression tests pass.
3. **Files added.** This report is the only file added by the final acceptance
   block. Package 13's earlier implementation added its schema-9 dialect,
   lowering, and test files listed by `implementation-status.yaml`.
4. **Files modified.** The implementation surface comprises Sema semantic
   facts, Semantic IR model/builder/printer/verifier, Sec MLIR emission, the
   schema-9 ODS/C++ dialect, scalar lowering, Go tests, MLIR tests, the package
   checklist, and implementation governance. The final acceptance block changes
   `internal/lowering/secmlir/emitter_test.go`, `package13-todo.md`, and
   `implementation-status.yaml` in addition to this report.
5. **Struct definitions and IDs.** Semantic IR has nominal `StructDefinition`,
   `StructFieldDefinition`, and zero-based declaration-order `StructFieldID`.
   Verification rejects duplicate names, duplicate IDs, and non-contiguous IDs.
6. **Field tags.** Sema copies ordered key/value tags into compiler-owned facts;
   Semantic IR preserves them as `StructTag`, and emission serializes them as
   ordered `#sec.struct_tag` attributes with escaped values intact.
7. **Generic and nested identity.** Concrete type arguments are interned into
   nominal type identity. Qualified nested identities and synthetic payload
   identities remain stable; neither field layout nor spelling guesses identity.
8. **Literal-plan API.** `ResolvedStructLiteralPlanOf` returns defensive copies
   of source-ordered entries and declaration-ordered final-field decisions,
   including type, source, action, and canonical default resolution.
9. **AST compatibility.** Legacy AST default materialization may remain, but the
   new builder consumes only the immutable Sema plan. Tests remove or alter
   synthesized AST fields and obtain identical Semantic IR decisions.
10. **Defaults.** Omitted fields consume `DefaultResolution` through
    `buildResolvedDefault`; recursive values are built after explicit/spread
    source entries and in declaration order. No `undef` or poison is emitted.
11. **Origins.** Every constructed field records `explicit`, `spread`, or
    `default`, independently of its `construct-direct` or `copy-trivial` action.
12. **Evaluation order.** Entries are evaluated left-to-right exactly once into
    temporary values. Final operands are then selected from those values in
    declaration order; defaults are evaluated after source entries.
13. **Spread.** `StructSpreadFieldsOp` takes one exact-typed source and produces
    one declaration-ordered result per field. Multiple spreads and later
    overrides retain source order and do not reevaluate the source.
14. **Construction.** `StructConstructOp` is emitted only for a complete plan
    matching the definition. It receives every field, origin, and accepted
    action in declaration order.
15. **Member resolution.** `ResolvedStructMemberOf` is the semantic authority
    for stored-field versus property/other resolution. Lowering never guesses
    from a member name; metadata consumers may use `ResolvedStructFieldAt`.
16. **Extraction.** `StructExtractFieldOp` carries the nominal owner type,
    declaration field ID, exact result type, and trivial-copy action.
17. **Replacement.** `StructReplaceFieldOp` replaces one field of a trivial
    struct value and returns the same nominal struct type. Its verifier checks
    owner, field ordinal, replacement type, and result type.
18. **Mutable storage.** Defaulted and explicit mutable trivial structs use
    high-level `!sec.storage<!sec.struct<...>>` declare/init/load/store operations.
19. **Nested replacement.** The RHS is evaluated first, the root is loaded once,
    parent values are extracted top-down, replacements rebuild leaf-to-root, and
    the rebuilt root is stored once.
20. **Schema 9.** Schema 9 adds `StructTagAttr`, `StructFieldAttr`, nominal
    `!sec.struct`, and `sec.struct.construct`, `spread_fields`, `extract`, and
    `replace_field`, with ODS/C++ verification and schema-8 compatibility tests.
21. **Package 5.** The trivial-core pass deliberately leaves struct-typed
    storage declare/init/load/store high-level and never converts it to memref.
22. **Package 6.** Target scalar resolution recursively rewrites `!sec.int` and
    `!sec.uint` fields for 32- and 64-bit plans while preserving struct identity,
    ordinals, tags, nested wrappers, and operations.
23. **Package 8.** Checked-integer signless normalization does not recurse into
    struct wrappers and does not lower any schema-9 struct operation.
24. **Union payloads.** A struct-like union variant gets a non-source-nameable
    synthetic `StructDefinition`, keyed by union `TypeID` plus variant index and
    containing payload fields in declaration order.
25. **Package 12.** Guarded whole-payload binding projects trivial union fields
    and constructs the synthetic struct. Guard dominance is retained; non-trivial
    payload bindings reject explicitly.
26. **Wide tests.** Struct and union-payload matrices cover `int128`, `uint256`,
    generic wide arguments, nested wide fields, parameters, returns, extraction,
    replacement, tags, and round trips.
27. **Default tests.** Coverage includes empty, explicit-only, partial, nested,
    named constrained, explicit-type, all-default, and non-defaultable omission
    cases, plus independence from legacy AST mutation.
28. **Spread tests.** Coverage includes one and multiple spreads, exactly-once
    evaluation, declaration-order results, explicit and later-spread overrides,
    duplicate explicit fields, wrong types, and non-trivial rejection.
29. **Ownership boundary.** Move-only, semantic-copy, shared/mutable borrow,
    partial move, non-trivial replacement, resource ownership, custom free,
    equality, property, receiver, and foreign-ABI paths reject at their current
    parser, Sema, or package-tagged Semantic IR boundary without partial IR.
30. **CMake command.** `cmake --build build/sec-mlir --target check-sec-mlir -j2`.
31. **Toolchain.** `sec-mlir-opt --version` reports LLVM 24.0.0git, optimized
    build with assertions.
32. **MLIR result.** `check-sec-mlir` passed 78 of 78 tests on 2026-08-28.
33. **Go result.** `go test ./...` passed from a fresh detached worktree at the
    audited HEAD with the exact P13 diff applied and no existing build artifacts.
    `go vet ./...` and the full suite with `SEC_MLIR_BIN` also passed.
34. **End-to-end result.** With absolute
    `SEC_MLIR_BIN=$PWD/build/sec-mlir/bin`, the same source module passes parser,
    Sema, verified Semantic IR, schema-9 emission, guard/CFG verification, scalar
    lowering, and trivial lowering for both 32- and 64-bit plans. Target struct
    fields become `si32/ui32` and `si64/ui64` respectively, with no unresolved
    conversion cast or LLVM dialect.
35. **Deviations.** Semantic-copy has no source-producible classification yet,
    so its executable action-boundary test injects the compiler-owned fact.
    Field-reference, partial-move, custom-free, and foreign-struct paths remain
    deliberately rejected at the earliest currently representable boundary.
    Package 13 selects no physical layout and implements no non-trivial ownership.
36. **Package 14 recommendation.** Proceed with the documented fixed-array
    semantic value package: canonical type/length, construction/default/spread,
    checked indexing, trivial element operations, and high-level `!sec.array`.
    Keep field/reference Places in Package 15 and move/copy/destruction actions
    in Package 17; do not pull either ownership boundary into Package 14.
