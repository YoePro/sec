# Sec MLIR — Package 27: LLVM Dialect Handoff and Executable Vertical Slices

- **Status:** Implementation specification; not implementation evidence
- **Created:** 2026-10-20
- **Last updated:** 2026-10-20
- **Revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/mlir/packages/sec-mlir-dialect_package27.md` (installed 2026-10-20)
- **Replaces:** None; additive amendment. Preserve all existing paragraph references.
- **Package:** `SEC-MLIR-P27`

---

**Scope:** one-track LLVM dialect convergence, capability closure, testable scalar/CFG slice.  **Status at authoring:** specification, not implementation.

## § 1. Goal and deliverable

P27 hardens and incrementally widens the **single existing** pipeline from verified Semantic IR through mixed Sec/standard MLIR to LLVM dialect, LLVM IR, a target object and a linkable artifact. This is *not* a new compiler backend. The deliverable is an executable, testable transition contract, explicit capability gates, and at least one narrowly scoped scalar/CFG end-to-end vertical slice that is independently reproducible. Earlier packages' source semantics and representation choices remain authoritative.

## § 2. October 20 synchronization: installed versus planned

The public `rules/mlir/sec_mlir_dialect.md` and `rules/mlir/sec_mlir_lowering.md` both specify **version 12**, installed on **2026-10-19**. The dialect **accepts explicit schema-12 selection** and has partial slice-type/local-operation verification; **default compiler emission remains schema 11**. P16's complete slice emission and acceptance remain unfinished. P17 through P26 specifications do not prove implementation. P27 must obtain `git rev-parse HEAD`, dirty state, exact generated schema support, test results and executable capabilities from the implementer's checkout. Do not repeat the obsolete claim that v11 is the latest *accepted* schema. Nor claim v12 is default or complete.

## § 3. October 17-18 single-track decisions

`rules/compiler/compiler_pipeline.md` §45(6) and applied MD-101 establish one track: Semantic IR → Sec/standard MLIR → LLVM dialect → LLVM IR → **matching-build `llc`** → target object → system C driver for **linking only**. Legacy direct LLVM and AST-to-LLVM-dialect paths are retired and must never be resurrected as fallbacks. Existing `sec-check-lowering-support` and `sec-finalize-for-llvm` are the primary capability gates; extend/reuse them. `emit-mlir` is the LLVM-dialect inspection boundary and is always verified; `emit-llvm` uses the existing `--mlir-bin` lookup contract. No `sec build --pipeline` switch is reinstated.

## § 4. Priorities, not package-number optimism

Implement a verified scalar/CFG vertical slice before attempting aggregates or descriptors. The source may be valid while a lowering feature is not ready: report `[unsupported-lowering]` with the specific missing capability, rather than inventing a result or silently choosing another backend. Do not globally block basic integer functions because P24–P26 remain planned. A feature-specific dependency must be proved at its actual admission boundary.

## § 5. Normative references

Read the *current* checkout versions of `rules/compiler/compiler_pipeline.md`, `rules/compiler/semantic_ir.md`, `rules/mlir/sec_mlir.md`, `rules/mlir/sec_mlir_dialect.md`, `rules/mlir/sec_mlir_lowering.md`, `rules/memory/layout.md`, `rules/memory/reference_model.md`, `rules/memory/storage.md`, `rules/platform/abi.md`, `rules/errors/panic.md`, `rules/compiler/initialization.md`, `missing-decisions.yaml`, and P15–P26 local plans, especially the corrected P18 and installed P16 schema-12 amendments. Follow the **normative precedence** in the compiler pipeline, not old package examples.

## § 6. Implementation inventory and proof of readiness

Before changes, record the current commit hash, uncommitted tracked/untracked files, LLVM/MLIR versions, executable paths, current emitted dialect versions, working test commands, and *per-feature* availability. Inspect `package25-todo.md`; its 2026-10-18 inventory at `831f6e1` is historical evidence, not the current HEAD. `package26-todo.md` may not be committed. Use live governance fragments; never infer implementation from schema-version text alone.

## § 7. Authority and no duplicate lowering engine

The new passes consume Sema/Semantic IR facts, existing P6/P8 scalar resolution, concrete CompilationPlan, selected layout/ABI when required, and discharged safety obligations. They do not recheck source type legality, redo borrow analysis, choose memory models, redo numeric checked semantics, or classify foreign ABI independently. Retain one MLIR conversion pipeline and one LLVM/target toolchain selection mechanism.

## § 8. Stage boundaries

The required inspected boundaries are `verified-semantic-ir`, `verified-sec-mlir`, `verified-standard-mlir`, `verified-llvm-dialect`, `verified-llvm-ir`, `target-object`, and `link`. Stage artifacts must clearly identify which boundary they contain; source/debug provenance and CompilationPlan identity survive as far as needed. An earlier stage may be requested without requiring readiness for all later stages.

## § 9. Capability-gated feature matrix

Introduce or extend a single canonical feature readiness matrix, keyed to **exact semantic operation family + target/profile + necessary layout/ABI/reference policy**. Example rows: `integer-constants`, `integer-checked-arith`, `integer-compare`, `bool-control-flow`, `direct-internal-call`, `trap-panic`, `enums-trivial`, `fixed-aggregates`, `borrowing`, `slices`, `strings`, `foreign-call`. A row is *ready* only when there is a full verified path to the requested boundary. It is not automatically ready because a parser/Sema visitor handles it. Never add a second competing readiness system.

## § 10. Capability decision order

Order: valid frontend/Semantic IR → target/profile legality → stage-specific implementation readiness → backend verification. Unsupported target ≠ unsupported compiler implementation ≠ invalid Sec. For a whole requested LLVM/object result, reject uncovered operations **before** emitting partial LLVM side effects; for a staged inspection, permit a high-level artifact when its own invariant boundary is complete.

## § 11. Independent predecessor gates

Scalar constants, signed/unsigned integer ops, ordinary boolean comparison, structured-to-CF and internal direct scalar calls may be implemented/tested with already installed P1–P14 infrastructure. A particular enum may be admitted only if its representation is completely discharged. Physical structs/fixed arrays/unions need P24 layout and P17 ownership where relevant. References/slices/descriptors need P15/P16/P18/P19/P24/P26 proofs. Foreign ABI and varargs need P25 `CallABIPlan`. No global "P26 completed" flag is the admission decision.

## § 12. No new Sec dialect schema by default

P27 primarily installs **lowering and bridge verification**, not a new Sec source type/operator family. Thus it **does not automatically bump the Sec dialect schema from planned v22 to v23**. The current installed dialect remains v12; planned P17–P26 schemas retain their own target versions. If an implementation genuinely needs a new serialized Sec operation/type/attribute, first document a *minimal* additive schema-v23 amendment and make it conditional on predecessor installation. Pure pipeline metadata/verification need not force a version bump. P27's lowering specification is **planned version 23** as a document revision, not an executable version claim.

## § 13. Pipeline algorithm and ordering

The pass driver selects a concrete `CompilationPlan`, verifies Semantic IR, constructs mixed Sec/standard MLIR, verifies installed dialects, runs completed semantics-specific lowering and checks, runs `sec-check-lowering-support` at the correct boundary, performs legal canonical conversions to standard MLIR, then executes `sec-finalize-for-llvm` to prove no undischarged Sec ops/types/attributes remain. Only then run registered standard-to-LLVM conversions, verify LLVM dialect, translate to LLVM IR, verify LLVM IR, use same-build `llc`, and link with the system C driver. Preserve exact pass names where already implemented.

## § 14. `sec-check-lowering-support` extension

The existing gate must report every unsupported operation family deterministically with the first actionable failure and source location (and optional bounded related diagnostics). The report includes requested output stage, target/profile, stage-specific missing prerequisite and a concise remedy. Maintain a machine-readable capability ID for governance and test assertions. It must never treat a missing implementation as a source semantic error.

## § 15. `sec-finalize-for-llvm` hard contract

Before LLVM conversion, prove **zero** remaining unlowered `sec.*` operations, Sec-only types, or undischarged Sec semantic attributes. Permit only an explicitly documented, fully lowered metadata whitelist (e.g., debug provenance) whose representation is legal in the target LLVM conversion. Reject unknown attributes, unresolved Result/error channels, abstract Places, reference proofs, cleanup tokens, allocation contexts, and future schema constructs; do not strip them optimistically. The gate fails closed.

## § 16. LLVM boundary verification

After conversion, the module must contain only operations/types supported by the selected MLIR→LLVM translation. Verify target triple, data layout, address-space constraints, calling-convention plan or permitted temporary internal scalar convention, symbol legality, branch/return type matching, and all dominance/side-effect obligations. Translation failures from otherwise valid verified input are compiler/toolchain defects (or diagnosed incompatible toolchain), not source errors.

## § 17. LLVM version binding and tool resolution

Select tools from one compatible MLIR/LLVM build. `sec-mlir-opt`, `mlir-translate`, `opt`/`llvm-as`/`llc` where used must match the selected build's LLVM IR dialect/serialization contract. Keep `--mlir-bin`/`SEC_MLIR_BIN`/known build directory/PATH search policy where current rulebook specifies it; reject a mixed-version pairing instead of attempting to compile version-specific `.ll` with arbitrary installed clang. Record the exact resolved paths and versions.

## § 18. Object and link steps

The canonical path is `mlir-translate --mlir-to-llvmir` (or equivalent matching API) → LLVM IR verification → matching-build `llc -filetype=obj` → object verification → system C driver **link** of object. Preserve selected target triple, CPU features, code model, relocation model and object format. Do not invoke the system C driver on textual LLVM IR. A bare-metal target may require a linker/driver profile different from hosted; follow CompilationPlan and fail clearly if missing.

## § 19. Executable vertical slice S0

Use one tiny Sec program in the already supported subset: a reachable internal scalar function returning target `int`, constant integer arithmetic (checked behavior preserved), comparison, and a conditional branch/merge. Track a single semantic result across Semantic IR, Sec/standard MLIR, LLVM dialect, LLVM IR and target object. Add **both successful and deterministic failing** tests. Avoid using strings, dynamic arrays, references, enums with unsupported physical form, exported C ABI or other unfinished features to prove base pipeline correctness.

## § 20. Integer representation and signedness

Reuse P6/P8 target integer resolution and signedness provenance; LLVM integer types are signless, but signed/unsigned arithmetic and conversion choice were settled before lowering. Keep active wide integer types up to 256 bits when the selected target pipeline supports their operations; do not silently narrow. Checked arithmetic lower via already verified predicates/guards and canonical failure endpoints rather than native LLVM overflow UB.

## § 21. Boolean and CFG lowering

Reuse `cf.br`, `cf.cond_br`, block arguments, `func.func` and existing conversion paths. At LLVM boundary all block argument types and phi-like values must agree; unreachable exhaustiveness residuals and error-flow traps are preserved. Reusing `arith`/`cf`/`func` legalizations is preferred over new Sec wrappers. Test loops, short-circuit and compound assignment only insofar as P14/current canonical track already supports them.

## § 22. Internal callable boundary

Until P25's exact native ABI plan is executable, **only the explicitly allowed existing internal scalar call convention** from MD-101 may be used for the narrow vertical slice. No "generic ABI" extrapolation to structs, descriptors, returned owners, or foreign calls. Functions/call sites must agree on the temporary internal physical convention; later P25 conversion reclassifies as specified by `CallABIPlan` without a second semantic calling system.

## § 23. Return and entry adapter

Do not invent a permanent C `main` export ABI. MD-101 contains interim hosted entry adapter details awaiting confirmation; preserve the current working adapter only behind its existing capability gate and do not elevate provisional integer status conversion or symbol spelling to a new normative language decision. If unsupported target entry policy is missing, report capability failure. Link the object through the chosen driver and support run tests only on runnable host targets.

## § 24. Deterministic failure endpoints

Keep distinct Sec failure causes (arithmetic overflow, division-by-zero, invalid shift, bounds, reference generation, synthesized unreachable). The MD-101 interim minimal hosted panic endpoint is trap + unreachable, **pending confirmation**. P27 must not create a universally mandated panic runtime or erase reasons before a later reporting policy can use them. Failure paths must not be translated into LLVM poison/UB; either materialize a correctly selected trap/panic endpoint or leave the feature unsupported.

## § 25. LLVM attributes must follow proof

Do not add stronger `nonnull`, `noalias`, `dereferenceable`, `align`, `noundef`, `readonly`, `nocapture`, `nsw`, `nuw`, inbounds GEP, `willreturn`, or `nounwind` than canonical analysis and target plan prove. A checked overflow path is not `nsw` until the relevant result is proved non-overflowing on that path. Unverified reference metadata must not turn into LLVM pointer attributes.

## § 26. Float and decimal gate

Float P20 and decimal P21 are specifications unless their concrete semantics and lowerings have been implemented and verified. Native LLVM f32/f64/arith support alone does not prove Sec floating literal, rounding, NaN or checked conversion guarantees. `decimal`/`decimal128` must never silently go through binary float. Add explicit capability rows and targeted late integration tests, not a broad assertion of readiness.

## § 27. Aggregate gate

High-level `!sec.struct`, fixed array, union/Result/Option, enum and named-contract values may cross the LLVM boundary only after canonical representation and valid-value/ownership obligations have been discharged. Consume P24 `ResolvedLayout`, P17 destruction and P22 contracts where applicable. Do not use ad hoc LLVM `{fields...}` layouts. Error paths and non-trivial destruction must survive.

## § 28. Reference and descriptor gate

A `ref`, `ref mut`, slice, string/array owner or Arena descriptor may cross only when P26 or existing equivalent has discharged every relevant guarantee and P24 selected physical layout. Preserve generation, spatial bounds, provenance, alignment, address-space, lifetime, relocation and mutation authority obligations. Do not infer that one machine pointer is enough, or silently drop generation. `RawPtr` adapters remain separately gated (MD-070 for public conversions).

## § 29. Foreign-call gate

Foreign and system calls require P25 target-specific `CallABIPlan`, P24 layout, FFI legality/provenance and any applicable P26 boundary conversion. `extern` safety follows October rules; do not reinsert unconditional unsafe requirement for ordinary `extern`. Never use the temporary native scalar convention for foreign calls, C varargs, callbacks, indirect foreign function pointers or ABI-dependent aggregates.

## § 30. Memory and effect ordering

Preserve allocation, release, volatile/MMIO, atomics, panic effects, cleanup/defer, reference invalidation and evaluation order. Do not hoist loads above required guards, delete failure side effects, reorder deallocation before destruction, or CSE distinct allocations. Conversion legality must carry these constraints until a lower dialect provides the equivalent ordering.

## § 31. Global/statics and linking boundary

No general statics, TLS, foreign symbol, initializer ordering or linker section support is implied by the S0 scalar track. Add them only with explicit feature readiness and tested target object/linking behavior. Do not choose a new global-init semantics in this package. Existing supported symbol naming stays stable for the same CompilationPlan; separate compilation must validate ABI signatures once P25 makes them available.

## § 32. Panic policy per target

Hosted, bare-metal, embedded and hardened profiles may differ in panic transport and runtime provision. Do not assume `libc`, libunwind, heap, unwinder, or a global panic hook. The canonical `CompilationPlan` determines available endpoint. If the existing minimal trap is insufficient to preserve an operation's semantics under the selected plan, reject the feature instead of manufacturing a fallback.

## § 33. Source and diagnostic provenance

Maintain canonical source location, expression/operation identity, semantic failure reason, selected CompilationPlan and pass stage in diagnostic payloads. A verifier/internal inconsistency produces an internal compiler diagnostic with offending op and pass name; implementation gap produces `[unsupported-lowering]`; invalid source remains frontend diagnostic. Do not emit generic "LLVM failed" for an already known missing capability.

## § 34. Pass failure atomicity

Failed conversion must not publish a partially translated LLVM module, object or linked artifact as successful output. Use temporary artifact paths with atomic success publication or equivalent pipeline guarantees. Stage-specific debug dumps may be retained only with explicit incomplete/failed labeling. Multi-target builds publish each target's result from a consistent plan snapshot.

## § 35. No premature LLVM optimization

Begin S0 with conservative/low optimization to make the SemIR→LLVM correspondence auditable; then add O1/O2 parity tests. Optimization never becomes a correctness precondition. Target LLVM passes must not assume absence of NaN, lack of traps, or nonaliasing unless facts are proven, and must not erase precise failure control flow.

## § 36. Data layout and target triple parity

The MLIR LLVM dialect module, translated LLVM IR, `llc` flags and link driver must agree with the selected CompilationPlan's triple and data layout. Host pointer size/endianness cannot fill missing target facts. Test 32-bit vs 64-bit target widths and cross-target object generation without executing target code. Missing backend support is a target/toolchain capability failure.

## § 37. Separate compilation and incremental caches

Artifact validity includes stage schema/version, CompilationPlan target/profile/layout/ABI, toolchain build identity, selected pipeline/capability set, relevant Semantic IR and dependent declarations. Recompute cold on incompatible cache state. Persist no cache as canonical truth. Incomplete or mixed snapshots cannot generate objects. Cold and incremental completed outputs must be semantically equivalent.

## § 38. Determinism and reproducibility

Repeat S0 compilation from the same source and CompilationPlan and compare stage-normalized textual MLIR/LLVM, symbol names and object metadata to the extent reproducibility contracts permit. Keep nondeterministic pointers/temp paths out of canonical semantic IDs. File/line metadata changes must not silently alter source behavior.

## § 39. LLVM handoff verifier

Register or extend a pass such as `--sec-verify-llvm-handoff` (reuse an existing equivalent if present). It checks complete Sec-op/type/attr elimination, target triple/layout agreement, legal types for the registered conversion patterns, absence of undischarged safety obligations, ABI plan requirements, source provenance mapping and panic endpoint availability. Must include malformed negative fixtures for unknown `sec.*`, unresolved Place, error Result, missing generation check, unknown layout and wrong target triple.

## § 40. LLVM artifact verifier

Register or extend `--sec-verify-llvm-artifact` (name may adapt to existing CLI) to check translated LLVM IR verification and exact toolchain identity, object format/triple and link outcome. Never assume a successful `mlir-translate` result proves executable correctness. Negative tests must catch wrong `llc`, corrupted IR, missing symbol, mismatched object format and falsely reported successful output.

## § 41. Runtime differential oracle

For S0 supported hosts, run the generated program and compare behavior against an independently checked Semantic IR evaluator/reference expectation or existing Sec conformance oracle—not the retired legacy backend. Cover success result and deterministic traps. Optimization O0/O2 and cold/incremental runs should agree on observable behavior. Keep feature subset precise; no whole-language differential correctness claim.

## § 42. One-item-at-a-time implementation discipline

Each TODO item is a separate change/test/report cycle: read the listed normative paragraphs; implement one item; run focused positive *and* negative tests; report command/status/output; update the item only after evidence; then proceed. Do not simultaneously implement later P24/25/26 work under P27 or silently tick blocked items. User's uncommitted files must be preserved.

## § 43. Phase A: baseline + runnable current pipeline

Capture HEAD/dirty state, versions, schema acceptance/default, actual `sec build`/`emit-mlir`/`emit-llvm` behavior, S0 fixture and current acceptance tests. Verify v12 parses with explicit selection and v11 remains default. Ensure no test silently treats v12 slice completeness as proven. Capture MD-101 interim behavior and any updated corrections.

## § 44. Phase B: capability and verified handoff

Extend existing `sec-check-lowering-support` and `sec-finalize-for-llvm`, add handoff verification/malformed IR tests, and freeze exact stage-specific capability diagnostics. Keep existing paths intact. Implement independent tests for unknown Sec ops, invalid types, wrong schema, missing metadata and target mismatch before adding LLVM coverage.

## § 45. Phase C: S0 executable vertical slice

Lower a minimal, already ready integer/boolean/CFG/internal-call subset through the existing matching MLIR/LLVM pipeline. For each fixture record emitted Sec/standard MLIR, LLVM dialect, LLVM IR, object and exit behavior. Assert `llc` is from the same LLVM build. Test checked arithmetic failure and one synthesized unreachable/bounds failure only if its existing high-level meaning and endpoint are fully supported.

## § 46. Phase D: predecessor-conditional expansion

Add a new feature *only when its P15–P26-dependent plans are executable*: enum conversion, fixed array/struct, Result, float, string, ref/slice/array, Arena, FFI ABI and so on. For each feature create a corresponding capability item, a no-regression negative fixture, and LLVM verifier tests. Unsupported features remain explicitly gated; do not fabricate a massive "all types" lowerer in P27.

## § 47. Phase E: objects, linking and target matrix

Verify hosted object+link success with system C driver linking only, correct imported symbol resolution and no-mixed-toolchain behavior. Cross-compile at least one second target where toolchain is available; never execute target code in cross tests. Preserve relocation/code model/CPU features and report unsupported object targets honestly.

## § 48. Phase F: integration, governance and handoff

Run targeted MLIR unit/lit, end-to-end source tests, CMake checks, `go test ./...`, `go vet ./...` where supported, plus cold/warm cache mutations. Update only existing per-book governance implementation-status fragments and `package27-todo.md`, recording status IDs and failure evidence. Deliver a report with exact HEAD, dirty files, toolchain, supported operation families, blocked gates, test outputs and next priorities.

## § 49. Baseline fixture S0-A

Conceptual Sec source (update parser details to currently valid syntax, never bypass Sema):
```sec
fn AddOne(value: int) int {
    return value + 1
}

fn main() int {
    let computed := AddOne(40)
    if computed == 41 {
        return 0
    }
    return 1
}
```
The exact hosted entry adapter is controlled by MD-101 and `initialization.md`; if this fixture's entry contract is incomplete, use a fully supported internal Sec function plus the existing checked harness instead of inventing ABI behavior.

## § 50. Negative S0 fixtures

At minimum: checked signed overflow, signed division by zero, invalid shift, unreachable branch proof mismatch, unsupported high-level struct lowering, unresolved safe ref, undeclared `sec.*`, schema-v12 slice unsupported for full LLVM where appropriate, foreign call without P25 plan, and mixed LLVM `llc` version. Assert diagnostics classify correctly, no object is published for failure, and no legacy fallback runs.

## § 51. Test compilation matrix

Test: default schema-v11 compiler emission; explicit schema-v12 dialect acceptance; P16 partial slice verify without claiming full compile; v1–v11 regression fixtures; planned v13–v22 fixtures only as optional/conditional when actually installed. For S0, run one host 64-bit and one alternate 32-bit/cross target if available; debug and release, O0/O2, cold/warm, and deterministic failing paths. Distinguish **not run**, **blocked**, **failed**, **passed**.

## § 52. Compatibility with predecessor amendments

P26's public file incorrectly says the installed dialect spec is v11. Apply a localized status correction in P27 sync notes: current **spec** v12, default **compiler emitted schema** v11, explicit v12 accepted, slice coverage partial. No semantic change is implied. P25 TODO and P24–P26 plans remain implementation evidence only where a real test confirms it.

## § 53. Decision policy

Check `missing-decisions.yaml` including local uncommitted updates; the public copy may lag. **MD-101 already tracks provisional LLVM-track details**, including failure trap, entry adapter and internal scalar ABI. **MD-070** covers public raw-pointer/reference conversions; MD-019/MD-021/MD-045 cover layout/foreign ABI prerequisites. Do not create duplicate P27-D# items merely to restate them. If a genuinely new normative choice arises, create standalone `P27-D1.md`, `P27-D2.md` etc. and add it to the decision index without silently settling it.

## § 54. Explicit exclusions

P27 does not implement the missing physical struct/union, reference, dynamic array, string, Arena or full FFI lowerings; it does not establish a universal panic ABI, general runtime, nontrivial destructor ABI, compiler-wide stable C export, automatic schema 23, LLVM exception/unwinding model, or second backend. These may be individually admitted only after owner packages' actual gates are satisfied.

## § 55. Architecture invariants

One verified Semantic IR→Sec/standard MLIR→LLVM dialect→LLVM IR→`llc`→link track. Source correctness is a Sema/semantic obligation, never decided by LLVM. `sec-check-lowering-support` and `sec-finalize-for-llvm` remain single authorities. Schema-v12 acceptance and schema-v11 default are distinct. No undischarged `sec.*` crosses the LLVM handoff. No language feature supported merely by LLVM availability. No backend-invented ABI/layout/reference/ownership semantics. No raw pointer safety fabrication. No stronger LLVM attributes than proven. No mixed LLVM/MLIR toolchain. No C-driver compilation of LLVM IR. No legacy fallback. No hidden panic semantic decision.

## § 56. Completion definition

P27 is **complete for its defined S0 scope**, not for all Sec lowering, only when the existing track's scalar+CFG executable slice passes stage verification, LLVM IR verifier, matching-build `llc`, target object/link and executable host tests; capability gates correctly fail on uncovered features; backward schema v11/v12 tests pass; negative/mutation/profile tests pass; current governance/TODO reflect actual evidence. Full P24–P26 lowering is explicitly separate.

## § 57. Required implementation report

Report exact HEAD and dirty tree; public/local rulebook versions; schema specification=12 and measured emission default/explicit acceptance; real LLVM/MLIR tool versions and paths; current capability matrix; completed P27 TODO IDs with per-item test commands and outputs; new/modified files; LLVM-dialect pass order; LLVM IR verification; `llc` same-build proof; link-driver invocation; native execution; alternate-target artifact check; negative gate/UB/attributes tests; unsupported features and predecessor blockers; cache/cold parity; decision-index comparison; governance updates; `go test ./...`/CMake/lit status; deviations and P28 handoff.

## § 58. Next package boundary

After P27 is validated, prioritize **P28: LLVM Convergence Expansion for Verified Aggregates and Error Paths**, or whichever highest-value implemented predecessor slice is ready. P28 must start from actual P17/P24/P25/P26 readiness rather than assume all P27 prerequisites have landed. If the LLVM path still lacks adequate verified feature coverage, extending and hardening this same path is preferable to starting concurrency/FFI/other broad lowerings prematurely.

---

## Synchronization note — 2026-10-20

Installed 2026-10-20 and validated against the live checkout
(`package27-todo.md`). Corrections: (1) the S0 slice of §§ 19, 45 and 49
already builds, links and runs, with 18 further native fixtures — including
statics as LLVM globals (contrary to § 31's "no statics implied"), ordinary
integer enums and the P20 binary-float subset (contrary to § 26's
"specifications unless implemented"); these stay admitted. (2) The same-build
`llc` object step of § 18 has existed since 2026-10-18 but without a version
identity check. (3) `sec-finalize-for-llvm` currently strips every `sec.*`
attribute, which § 15 forbids; P27-06 replaces this with a whitelist.
(4) § 53's standalone `P27-D#.md` files are replaced by the project practice:
simple choices are asked of the owner, substantial forks become `MD-###`
entries. (5) Files were installed at the established locations: sync note in
`normative-sync/`, Semantic IR amendment in `semantic-ir/`, lowering as
`sec_mlir_lowering_v23.md`.
