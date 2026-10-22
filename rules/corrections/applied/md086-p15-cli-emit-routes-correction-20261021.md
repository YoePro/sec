# MD-086 — P15 Semantic IR and Sec MLIR inspection routes

- **Status:** Applied 2026-10-22 — integrated and implemented; register entry removed.
- **Created:** 2026-10-21
- **Last updated:** 2026-10-21
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical correction destination:** `rules/corrections/applied/` after integration
- **Primary normative owners:** `rules/compiler/compiler_pipeline.md` (§§ 55, 58–61); `rules/mlir/packages/sec-mlir-dialect_package15.md`
- **Related owners:** `rules/mlir/sec_mlir_dialect.md`, `rules/mlir/sec_mlir_lowering.md` and command/CLI governance
- **Decision tracking:** `missing-decisions.yaml`, MD-086
- **Related decisions:** MD-084, MD-085, MD-087; P15-57, P15-65, P15-67, P15-72
- **Replaces:** the P14 package ceiling for the `emit-ir` and `emit-sec-mlir` routes when those routes can produce and verify the supported P15 subset
- **Reference baseline:** public `main` compiler pipeline, P15 TODO and dialect/lowering contracts reviewed 2026-10-21; default schema 11, explicit partial schema-12 support at baseline

---

## § 1. Purpose and separate compilation boundaries

§ 1(1) This decision exposes the **already implemented and independently verified** P15 high-level representation through inspection commands, without treating high-level emission as proof that physical reference lowering or native code generation is available.

§ 1(2) `emit-ir` stops after Sema/lowering-readiness, canonical Semantic IR construction and Semantic IR verification. `emit-sec-mlir` additionally emits and verifies the selected supported Sec MLIR schema.

§ 1(3) Neither inspection route requires target-code, LLVM reference ABI, or physical pointer/reference representation to be implemented merely to produce valid high-level output. Neither bypasses validity, type, ownership, lifetime or selected `CompilationPlan` checks required at its own boundary.

§ 1(4) The package ceiling of the inspection route is a **capability statement about its output boundary**, not an assertion that all higher or lower compilation stages are complete.

## § 2. CLI capability requirements

§ 2(1) `emit-ir` MUST allow every valid program in the implemented P15 Semantic IR subset after successful Sema and lowering-readiness validation and after its Semantic IR verifier succeeds. A P14-only gate must not reject such a program solely because it contains direct safe references or Place operations now supported by P15.

§ 2(2) `emit-sec-mlir` MUST allow every valid P15 subset program that its schema-11 emitter and required Sec MLIR verifiers can represent and verify. A P14-only route gate must not prevent it from reaching that verified boundary.

§ 2(3) `emit-mlir` is the separately documented generic MLIR boundary (compiler_pipeline.md § 60); it is **not** silently redefined to mean `emit-sec-mlir`. If the requested variant of `emit-mlir` entails lowering reference/Place constructs to a still-unavailable lower representation, it must reject that unsupported lowering with a capability diagnostic.

§ 2(4) `emit-llvm` requires that all relevant reference/Place semantics, layout, storage and ABI obligations be lowered. Until a correct physical reference representation is installed, a reference program requiring that representation must be rejected as unsupported lowering. No fake or silently erased reference metadata is allowed.

§ 2(5) Ordinary non-reference programs retain their established route behavior. Package-15 inspection enablement must not promote unrelated unfinished package features or change the physical-lowering contract of ordinary P14/P8 programs.

## § 3. Activation gate and end-to-end acceptance

§ 3(1) Enable the updated `emit-ir` route after tests demonstrate source -> Sema -> verified P15 Semantic IR for the exact subset the route advertises. The route need not wait for P15's full administrative completion report.

§ 3(2) Enable the updated `emit-sec-mlir` route after tests demonstrate source -> Sema -> verified P15 Semantic IR -> schema-11 emission -> schema-11 parsing/standard verification -> all applicable Sec MLIR verifiers, with the correct target plan and schema facts.

§ 3(3) The tests required by P15-65/P15-72 (and related test/governance tasks) are mandatory evidence for these routes. CLI wiring alone does not satisfy the gate, nor does successful schema parsing without corresponding Semantic IR verification.

§ 3(4) A route may be enabled for a proven supported subset while unrelated P15 completion or physical lowering remains unfinished. Any nonrepresented feature must yield a precise unsupported-capability diagnostic rather than a malformed intermediate form or a blanket, inaccurate statement that the program is invalid Sec.

## § 4. Selected schema and CompilationPlan

§ 4(1) Emit the actual selected supported schema and correct `CompilationPlan` data. Schema selection cannot silently depend on whether the source happened to include a reference, nor may metadata claim a different schema than the emitted operations use.

§ 4(2) At the 2026-10-21 baseline the default compiler emission schema is **11**. Explicit schema-12 selection and partial schema-12 parsing/verification exist but do not imply completed P16 slice emission. This correction does not change that default or grant automatic schema-12 promotion.

§ 4(3) For schema 12, advertise and accept only operations with implemented and verified emitter support at that schema level. A parser supporting schema 12 is not itself sufficient evidence to emit all schema-12 features.

§ 4(4) The MD-084 metadata spelling and MD-085 return-origin verification contracts apply to P15 direct references in both schemas where their operations are admitted. Emitters must not strip metadata to make the CLI route appear successful.

§ 4(5) When target-dependent widths, memory spaces or address-space facts are unsupported for the selected schema/plan, report a lowering capability failure; do not substitute host defaults or mislabel the output plan.

## § 5. Verification and atomic output

§ 5(1) Run the verification required by the exact boundary, including Semantic IR checks and, for `emit-sec-mlir`, schema/dialect and applicable Places/reference-guards/borrow-semantics verifiers. A missing required verifier is not equivalent to a successful check.

§ 5(2) Publish output only after its verification has succeeded. For file output, stage the bytes privately and atomically install the final artifact only on success; for stdout, avoid emitting an apparently complete artifact before validation has finished. Failure must not leave a partial output presented as valid.

§ 5(3) Failed emission and unsupported lowering must use diagnostics that distinguish invalid Sec source, target/profile unsupported features, current compiler implementation gaps, and internal compiler correctness failures as required by `compiler_pipeline.md` § 2.

§ 5(4) A program that fails MD-085 origin verification cannot be published by `emit-sec-mlir`; bypassing a verifier to obtain inspection output is forbidden.

## § 6. Required test updates

§ 6(1) Replace the P15-67 expectation represented by `TestReferenceProgramsAreRejectedByEveryEmitRoute`. That test no longer expresses the accepted contract.

§ 6(2) Add positive `emit-ir` tests with valid shared/mutable borrowing, reborrowing, field/element projections, lifetime ends and reference equality within P15 scope. Compare actual emitted operations/facts with verified Semantic IR, not merely process exit status.

§ 6(3) Add positive `emit-sec-mlir` tests for those cases on 32- and 64-bit plans; parse emitted output and run standard MLIR verification plus `--sec-verify-places`, `--sec-verify-reference-guards`, and `--sec-verify-borrow-semantics` where applicable.

§ 6(4) Add negative lower-route tests: `emit-llvm` and physical/lower `emit-mlir` requests fail when reference representation is required but not yet available; the same valid source succeeds at its earlier supported high-level inspection boundaries.

§ 6(5) Add negative upper-route tests: bad source, invalid borrowing, invalid return origins, unsupported P15 operation, unsupported target-defined address space or unimplemented schema selection must not produce published output.

§ 6(6) Test output-path atomicity, deterministic emission, correct schema/plan labels, compatibility of ordinary P14 programs and no accidental fallback to P14 when the P15 subset is supported.

§ 6(7) When explicit schema 12 is requested, test the supported subset and separately test unsupported slice forms. Verify that default schema remains 11 until a distinct explicit decision promotes it.

## § 7. Integration and governance instructions

§ 7(1) Amend the compiler pipeline inspection/emit boundary text and the P15 package specification without changing old numbered references. Keep the exact CLI spellings in the CLI/tooling contract; preserve the separation between pipeline boundary rules and user-facing command implementation details.

§ 7(2) Synchronize the P15 CLI route implementation and P15-65/P15-67/P15-72 acceptance suites. Update TODO checkboxes/status only after the corresponding tests run and evidence is recorded; document unsupported lower routes honestly.

§ 7(3) Synchronize the existing governance entry/fragment using its canonical stable ID and the current `governance/` schema. Do not create a duplicate `implementation-status.yaml` entry or assert complete P15 implementation based on just these two routes.

§ 7(4) Remove MD-086 from the outstanding-decision ledger only after normative integration and governance synchronization. The owner's accepted design decision is not itself proof that the CLI has been updated.

## § 8. Decision record

§ 8(1) **Owner decision accepted 2026-10-21:** `emit-ir` and `emit-sec-mlir` gain supported, verified P15 coverage at their own boundaries; their end-to-end tests are activation gates; `emit-llvm` and deeper `emit-mlir` may not fake missing physical reference lowering; schema/plan are accurate; unsuccessful emission never publishes partial output; tests/diagnostics must be updated.

§ 8(2) No new user-visible source syntax, reference ABI, default schema promotion, or automatic P16 feature enablement is authorized by this decision.

---

## Application note (2026-10-22)

**Integrated into:**
- `rules/compiler/compiler_pipeline.md` § 59(4)–(7) and § 61(4).
- The P15-67 note in `package15-todo.md`.

**Implementation:**
- `cmd/compiler/native_pipeline.go` adds `inspectionMaxPackage = 15` for `emit-ir` and `emit-sec-mlir`; `nativeTrackMaxPackage = 14` remains for `emit-mlir`, `emit-llvm` and `build`.
- The lower routes now name the missing physical representation of Places and references and point to the inspection routes.
- `emit-sec-mlir` additionally runs `--sec-verify-places`, `--sec-verify-reference-guards` and `--sec-verify-borrow-semantics` (`internal/mlir/toolchain.go` `VerifySecInspection`) before publishing.

**Tests:** `TestReferenceProgramsAreRejectedByEveryEmitRoute` is replaced by `TestReferenceProgramsReachOnlyTheInspectionBoundaries`:
- `emit-ir` content checks.
- `emit-sec-mlir` on 64-bit (linux-amd64) and 32-bit (linux-armv7) plans with all verifiers.
- `emit-llvm` / `emit-mlir` rejection that leaves existing output untouched.

**Known holes (verified 2026-10-22):**
- MD-085's path-aware verification was implemented on 2026-10-22, so `emit-sec-mlir` publishes only output that passes it.
- (Closed 2026-10-22) The § 6 matrix now has CLI-level tests in `TestReferenceInspectionRoutesCoverThePackage15Matrix`, using `testdata/semantic_ir/reference_emit_matrix.sec`. They cover borrows, reborrows, copies, equality, `*r`, temporaries, returned field origins and Sema-placed borrow ends, with deterministic schema-11 output on 32/64-bit plans. Invalid borrowing, an escaping return origin and a program beyond package 15 (slices) publish nothing and leave existing output untouched. The CLI has no schema-selection flag, so § 6(7) reduces to the default staying schema 11.
