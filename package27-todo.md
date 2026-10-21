# SEC-MLIR Package 27 — implementation TODO

Package: `SEC-MLIR-P27` — LLVM Dialect Handoff and Executable Vertical Slices  
Primary rulebook: `rules/mlir/packages/sec-mlir-dialect_package27.md`  
Package contract: `rules/mlir/packages/sec-mlir-dialect_package27.yaml`

Normative inputs:

- `rules/mlir/normative-sync/sec_llvm_track_sync_package27.md`
- `rules/mlir/semantic-ir/sec_semantic_ir_llvm_handoff_package27.md`
- `rules/mlir/dialect-versions/sec_mlir_dialect_package27.md` (no schema bump)
- `rules/mlir/lowering-versions/sec_mlir_lowering_v23.md` (planned lowering v23)
- `rules/mlir/packages/sec-mlir-dialect_package27_decision-index.md`
- `rules/compiler/compiler_pipeline.md` § 45(6); `missing-decisions.yaml` MD-101

This is an implementation plan, not a completion report. It was installed
2026-10-20 from `rules/package27-todo-proposal.md` and validated against the
live checkout: item IDs P27-01..P27-31 are the proposal's and stay stable;
validation notes credit what the single track already does (with evidence)
and name gaps the proposal did not see. Execute one item at a time with its
focused positive and negative test, and record the evidence row below.

## Inventory snapshot — 2026-10-20

| Area | Current implementation | P27 consequence |
|---|---|---|
| Track | One track since 2026-10-17 (MD-101): Semantic IR → Sec/standard MLIR → `sec-mlir-opt --sec-lower-to-llvm` → LLVM dialect → `mlir-translate` → same-build `llc` object → system C driver link only (`internal/mlir/toolchain.go` `CompileObject`, `cmd/compiler/build_command.go`, since 2026-10-18). `internal/codegen` contains only `targetplan`; no `--pipeline` switch exists. | The S0 slice and the `llc`/link contract are already executable; P27 hardens and verifies them instead of building them. |
| Executable coverage | `TestNativeTrackBuildsAndRuns` builds and runs 19 fixtures (`testdata/build/native/`): integer arithmetic and comparison, loops, `for` with constant steps, switch (integer, bool, enum, subjectless), exact integer conversions, char/rune comparison, ordinary integer enums, discard, statics as LLVM globals, explicit return, panic/assert/overflow/division traps and the P20 binary-float subset. | S0 (§ 49) is covered and exceeded; P27 must not re-gate these families as "not ready". |
| Capability gates | `sec-check-lowering-support` admits a fixed operation-name set; `sec-finalize-for-llvm` rejects leftover Sec ops/types and then **strips every `sec.*` attribute**. | Gate reports are operation-name based (no stage/target specificity); finalize stripping conflicts with package book § 15 (P27-06). |
| Schema | Specification v12; default emission schema 11; explicit schema 12 accepted (P16 partial). | P27 does not bump the schema (dialect amendment § 1). |
| Toolchain identity | Tools resolve from `--mlir-bin` / `SEC_MLIR_BIN` / default build / PATH; `llc` comes from the same lookup as `mlir-translate`, but no version identity is checked. | P27-15/P27-26 add the identity check and mixed-toolchain rejection. |
| Decisions | MD-101 is open: trap endpoint, i8 reason codes, hosted `main` adapter, entry identity, internal calling convention, Semantic IR level 14 await confirmation. | Keep them interim; P27-31's report lists them for an owner decision. |

## Authority and dependency constraints

1. One track only; no fallback backend (`compiler_pipeline.md` § 45(6)).
2. MD-101's interim choices stay interim; P27 does not turn trap+unreachable,
   the C `main` adapter or the internal convention into language rules.
3. Aggregates/references/descriptors/foreign calls stay gated on P17, P24,
   P25 and P26 at their own admission rows (book §§ 27–29).
4. New decisions follow the project practice: a simple choice is asked of the
   owner; a substantial fork becomes an `MD-###` entry (replacing book § 53's
   standalone `P27-D#.md` files).
5. Governance: progress goes to the existing compiler/lowering entries
   (`compiler.native-*`/track entries in `governance/compiler.yaml`); a
   `lowering.sec-mlir-package27` entry is created when P27 implementation
   starts, as for P17.

## Completion boundary

As defined by the package book § 56 for the S0 scope. P27 does not implement
physical aggregates, references, strings, Arena, FFI, a panic runtime, an
unwinding model or schema 23.

## A. Synchronization and baseline

- [ ] P27-01 — Capture `git rev-parse HEAD`, `git status --short`, toolchain
  paths/versions and currently emitted schema; preserve uncommitted work.
- [x] P27-02 — Verify schema v12 is accepted explicitly, schema v11 remains
  default and full P16 slice emission is not implicitly claimed.

  Verified 2026-10-20 from code: `emissionSchema` (`internal/lowering/secmlir/emitter_options.go`)
  defaults to schema 11 and admits 11/12 only; `SecDialect::verifyOperationAttribute` accepts
  versions 1–12 and rejects slice types under schema < 12. A dedicated regression test that
  asserts both facts is still part of P27-04's baseline run.

- [ ] P27-03 — Read MD-101 and the current pipeline, plus `package25-todo.md`,
  `package26-todo.md` and live governance fragments; classify confirmed versus
  planned.
- [ ] P27-04 — Run baseline `go test ./...`, focused CMake/lit and existing
  `sec build`/emit commands; record pre-existing failures (known: the two
  stdlib/io tests blocked by MD-109).

## B. Capability gates and stage boundaries

- [ ] P27-05 — Reuse the existing `sec-check-lowering-support` capability
  table; add stage/target/profile specificity, positive and negative tests.

  Today the table is `isLoweredSecOperation` in `SecToLLVM.cpp`, keyed by
  operation name only; its rejection text already maps to `[unsupported-lowering]`
  with exit 4 through `internal/mlir`.

- [ ] P27-06 — Harden `sec-finalize-for-llvm` to reject every leftover Sec
  op/type/undischarged semantic attr; test malformed inputs.

  Validation finding: the pass already rejects leftover Sec ops and types but
  then strips **all** `sec.*` discardable attributes and argument/result
  attributes, which book § 15 forbids ("do not strip them optimistically").
  Introduce an explicit whitelist of fully discharged metadata (for example
  `sec.scalar_kind`, `sec.function_id`, provenance) and reject the rest
  (`sec.conversion_proof`, reference facts, try/match metadata) unless the
  owning pass consumed them.

- [ ] P27-07 — Add or extend the LLVM-handoff verification boundary with
  target/layout/ABI/obligation checks; test wrong proof and target triple.
- [ ] P27-08 — Distinguish invalid-source, unsupported-target,
  unsupported-implementation and internal defect diagnostics and
  machine-readable IDs.
- [x] P27-09 — Verify no retired direct-LLVM or AST-to-LLVM fallback exists in
  build/emit paths.

  Verified 2026-10-20: `internal/codegen` holds only `targetplan`; no `--pipeline` option in
  `cmd/compiler`; `emit-mlir`/`emit-llvm`/`build` all call `lowerToLLVMDialect`
  (`cmd/compiler/native_pipeline.go`). Removal happened 2026-10-17 (MD-101).

## C. S0 executable scalar/CFG track

The S0 slice already exists: `testdata/build/native/add.sec` is the S0-A
shape (internal scalar call, arithmetic, comparison, branch) and the native
test builds, links and runs it with 18 further fixtures. Items below are
credited where that evidence covers them and keep only the missing checks.

- [x] P27-10 — Build minimal already-supported scalar+CFG SemIR fixture with
  deterministic expected result; verify SemIR.

  Evidence: `TestNativeTrackBuildsAndRuns/add` (exit status 5) and `/loop`, `/switch`.

- [x] P27-11 — Emit mixed Sec/standard MLIR for fixture; verify scalar types
  and target width.

  Evidence: `emit-sec-mlir` verifies through `sec-mlir-opt`; target width comes from the
  module `dlti.dl_spec` (lit `scalar-layout-32.mlir`, `target_scalar_parity_test.go`).

- [x] P27-12 — Lower legal S0 operations to standard MLIR without new Sec
  wrappers; test ordinary plus failing checked arithmetic.

  Evidence: `sec-lower-checked-integers` → `arith`; native `overflow_trap`, `division_trap`.

- [x] P27-13 — Lower func/cf/arith through registered standard LLVM
  conversions; assert fully legal LLVM dialect.

  Evidence: upstream arith/cf/memref/func-to-LLVM after `sec-finalize-for-llvm`; lit
  `Conversion/SecToLLVM/*.mlir`.

- [ ] P27-14 — Translate matching LLVM dialect to LLVM IR and run LLVM
  verifier; check target triple/data layout.

  Partial: `mlir-translate --mlir-to-llvmir` runs and the triple is set from
  `sec.target_triple`; no explicit LLVM IR verifier invocation or data-layout
  string check exists.

- [ ] P27-15 — Use matching-build `llc` to generate object and verify correct
  object format; reject wrong-build `llc`.

  Partial: `CompileObject` runs `llc -filetype=obj -relocation-model=pic` from the same
  lookup as `mlir-translate` (user decision 2026-10-18). Missing: version identity check,
  object-format verification and the wrong-build negative test.

- [x] P27-16 — Link object via system C driver in link-only mode and execute
  host fixture where available.

  Evidence: `build_command.go` passes only the object to clang; native fixtures execute.

- [x] P27-17 — Test checked-failure path without LLVM poison/UB and verify
  selected panic/trap policy is currently supported.

  Evidence: `overflow_trap`, `division_trap`, `panic_trap`, `assert_trap` trap through
  `sec-lower-failure-endpoints` (`llvm.intr.trap` + unreachable, MD-101 interim).

## D. Predecessor-conditional feature admissions

- [ ] P27-18 — Add conditionally supported enum/union/Result/struct/fixed-array
  capability entries, without claiming readiness until P17/P24 evidence.

  Note: ordinary integer enums (constants, eq/ne, switch, conversions with a transported
  exactness proof) are already lowered (`sec-lower-integer-enums`); the gate applies to
  bit-backed enums, unions, Result, structs and fixed arrays.

- [ ] P27-19 — Add conditionally supported reference/slice/string/array/Arena
  entries gated on P15/P16/P18/P19/P23/P24/P26.
- [ ] P27-20 — Add P25-gated native-aggregate/foreign ABI/callback/vararg
  entries and reject unsupported FFI currently.
- [ ] P27-21 — Add P20/P21 numeric capability gates without assuming
  std-LLVM float/decimal operations imply Sec semantics.

  Note: the P20 binary-float subset (canonical-bit constants, neg, add/sub/mul/div/rem,
  six comparisons, plain float by plan width) is already lowered and executed
  (`lowering.sec-mlir-package20`, native `floats`); core float methods, checked
  conversions and all decimal operations stay gated.

- [ ] P27-22 — Add stage-specific negative test for every deferred family and
  ensure no object is reported as successful.

## E. Proof safety, targets and caching

- [ ] P27-23 — Test unproved LLVM flags (`nsw`, `nuw`, `noalias`, `nonnull`,
  `inbounds`) rejected or never emitted.
- [ ] P27-24 — Test branch/phi type mismatch, wrong alignment and missing
  safety guard negative inputs.
- [ ] P27-25 — Test 32-bit vs 64-bit `int` on available profiles; generate an
  alternate target object without executing it.
- [ ] P27-26 — Test failure-atomic artifact publication and wrong
  LLVM-toolchain selection.

  Note: `build` writes temporary `.ll`/`.o` files, but clang writes the final program path
  directly; atomic publication is not implemented.

- [ ] P27-27 — Test O0/O2 behavior and deterministic stage dumps on supported
  host fixtures.
- [ ] P27-28 — Test stale/incompatible CompilationPlan/toolchain/ABI cache
  input against independently verified cold compile.

## F. Documentation and handoff

- [ ] P27-29 — Update existing per-book governance fragments using canonical
  IDs; do not replace pre-existing status documents.
- [ ] P27-30 — Update P27 supported/blocked capability table and run
  targeted/full tests; record commands, exit status and not-run items.
- [ ] P27-31 — Publish implementation report with exact HEAD, dirty state,
  LLVM versions, S0 artifacts, deviations, pending MD-101 details and
  suggested P28.

## Evidence format for each item

`ID | commit/dirty baseline | exact command(s) | expected | observed | test
result | blockers | changed files | next gate`. Mark `not run` and `blocked`
distinctly from `passed`.

## Recommended execution order

1. A: baseline (P27-01, P27-03, P27-04).
2. B: P27-06 (finalize whitelist) first — it is a correctness gap — then
   P27-05/07/08.
3. C: close P27-14/15 (IR verifier, `llc` identity) on the existing slice.
4. E: P27-23..28 on the existing fixtures; D only as predecessors land.
5. F: governance, capability table and report; put MD-101's interim choices
   to the owner for a decision.
