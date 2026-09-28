# Compiler Testing and Verification

- **Status:** Normative
- **Created:** 2026-09-26
- **Last updated:** 2026-09-26
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/compiler/compiler_testing.md`
- **Replaces:** Planned compiler-testing entry; no earlier canonical rulebook
- **Repository baseline reviewed:** `main-reviewed-2026-09-26`
- **Implementation governance:** `governance/testing.yaml` (`testing.compiler-verification-v1`)
- **Related rulebooks:** `rules/tooling/testing.md`, `rules/tooling/diagnostics.txt`, `rules/compiler/compiler_pipeline.md`, `rules/compiler/semantic_ir.md`, `rules/compiler/debug_information.md`, `rules/compiler/monomorphization.md`, `rules/compiler/linking.md`, `rules/mlir/sec_mlir.md`, `rules/platform/target_profiles.md`, `rules/platform/abi.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/destruction.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/concurrency/tasks.md`, `rules/concurrency/threads.md`, `rules/concurrency/processes.md`, `rules/concurrency/ipc.md`, `rules/tooling/lsp.md`, `rules/compiler/incremental_compilation.md`

---

## § 1. Purpose and authority

**Governance tags:** `testing.compiler-verification-v1`

§ 1(1) This rulebook defines the canonical Sec 0.1 model for testing and verifying the compiler implementation.

§ 1(2) It owns the shared verification methodology and compiler-test infrastructure contracts for:
- compiler unit tests;
- frontend and semantic conformance tests;
- invalid-source and diagnostic tests;
- Semantic IR tests;
- lowering and representation tests;
- backend and object-artifact tests;
- executable conformance tests;
- regression tests;
- fuzzing and generated testing;
- mutation testing;
- compiler-test matrices;
- test isolation, reproducibility, sharding, and flake handling;
- compiler-test corpus and provenance;
- debugger-driving and debug-information conformance infrastructure;
- traceability between compiler obligations and sufficient test evidence.

§ 1(3) This rulebook defines how compiler correctness is evidenced. It does not redefine the language, compiler, target, ABI, runtime, library, or tooling semantics that the tests verify.

§ 1(4) The canonical behavior under test remains owned by the rulebook that defines that behavior.

§ 1(5) Mutable implementation status belongs in `governance/testing.yaml`, not in this normative rulebook.

§ 1(6) This rulebook introduces no source-visible Sec testing declaration, compiler-test attribute, compiler-test type, fuzzing declaration, mutation API, or reflection API.

---

## § 2. Separation from source-language testing

**Governance tags:** `testing.compiler-verification-v1`, `tooling.language-testing`

§ 2(1) `rules/tooling/testing.md` remains the canonical owner of source-language testing, including:
- `test "name" { ... }`;
- `*_test.sec`;
- the compiler-known `testing.*` surface;
- `TestCompilationPlan`;
- `sec test`;
- source-test discovery, selection, execution, outcomes, subtests, and editor integration.

§ 2(2) Compiler verification is a separate concern.

§ 2(3) Compiler tests may be implemented using host-language tests, Sec fixtures, scripts, object inspection, external tools, generated programs, emulators, debuggers, or other suitable infrastructure.

§ 2(4) The normative compiler-testing contract does not require the compiler or its test harness to be implemented in any particular programming language.

§ 2(5) An ordinary Sec program or source-level Sec test may be used as compiler-test input without transferring ownership of source-test semantics to this rulebook.

---

## § 3. Canonical compiler-test classes

**Governance tags:** `testing.compiler-verification-v1`

§ 3(1) Sec compiler verification recognizes at least the following semantic test classes:

```text
Unit
Frontend/Semantic
Invalid/Diagnostic
Semantic IR
Lowering/Representation
Backend/Codegen
Execution/Conformance
Regression
```

§ 3(2) These are compiler-test classifications, not Sec source-language declarations or compiler-known source types.

§ 3(3) A compiler test may belong to more than one class.

§ 3(4) Physical repository location does not determine the complete semantic classification of a test.

§ 3(5) A regression that verifies a Semantic IR ownership failure may therefore simultaneously be classified as `Semantic IR`, `Regression`, and the relevant feature area.

---

## § 4. Complementary correctness evidence

**Governance tags:** `testing.compiler-verification-v1`

§ 4(1) Compiler correctness requires complementary evidence at more than one verification level.

§ 4(2) The required levels include, where applicable:
- local or unit verification;
- stage-boundary verification;
- end-to-end semantic verification.

§ 4(3) No unit-test suite, structural verifier, lowering test, backend verifier, linker success, or executable smoke test substitutes for all other applicable levels.

§ 4(4) Structural validity and Sec semantic correctness are distinct obligations.

§ 4(5) A representation may be valid for its host IR or object format while still representing the wrong Sec program.

---

## § 5. Normative test oracle

**Governance tags:** `testing.compiler-verification-v1`

§ 5(1) Canonical Sec rulebooks, compiler contracts, target contracts, and ABI contracts define expected behavior.

§ 5(2) Existing compiler output is not normative merely because a previous compiler version produced it.

§ 5(3) A checked-in golden artifact is valid evidence only when its expected meaning is independently derived or reviewed against the owning canonical contract.

§ 5(4) A compiler change must not be justified solely by regenerating expected output from the changed compiler.

§ 5(5) Tests may record current implementation structure when that structure is itself the tested contract, but incidental implementation details must not become language semantics through test inertia.

---

## § 6. Owning contract boundary

**Governance tags:** `testing.compiler-verification-v1`

§ 6(1) A compiler invariant or defect should be tested at the earliest compiler boundary that canonically owns the behavior.

§ 6(2) Later-stage or execution evidence is added when the contract also has downstream or runtime consequences.

§ 6(3) A frontend ownership error should therefore have a frontend or semantic test even if the same defect could later crash generated code.

§ 6(4) A Semantic IR invariant should have a Semantic IR test even when later lowering also exposes the failure.

§ 6(5) A target ABI realization should have a target/backend test even when a high-level source test happens to execute successfully.

§ 6(6) Test source should normally minimize unrelated behavior while preserving the condition being verified.

---

## § 7. Positive, negative, and boundary coverage

**Governance tags:** `testing.compiler-verification-v1`

§ 7(1) Compiler testing must cover both acceptance and rejection boundaries where the underlying rule has both.

§ 7(2) A rule is not adequately evidenced merely because one valid example compiles or one invalid example is rejected.

§ 7(3) Boundary-sensitive features require representative edge coverage, including relevant empty, singleton, limit, last-valid, first-invalid, nesting, width, alignment, or state-transition boundaries.

§ 7(4) Exact relevant boundaries are determined by the owning contract.

§ 7(5) Expected source rejection counts as test success only when the required diagnostic contract is satisfied.

---

## § 8. Invalid input and compiler failure

**Governance tags:** `testing.compiler-verification-v1`

§ 8(1) Compiler-test infrastructure must support intentionally malformed, syntactically invalid, semantically invalid, target-invalid, or otherwise rejected Sec input.

§ 8(2) Invalid fixtures are first-class compiler inputs and need not form an ordinary buildable Sec project as a corpus.

§ 8(3) A compiler panic, assertion failure, crash, uncontrolled hang, or internal corruption is never a successful expected result for an ordinary invalid-source test.

§ 8(4) The harness must distinguish at least conceptually between:
- expected source compilation failure;
- compiler/internal failure;
- artifact-generation failure;
- program execution outcome;
- harness/infrastructure failure.

§ 8(5) An expected program panic or target termination may be a successful execution oracle when required by the owning rulebook, but it must not mask compiler or harness failure.

---

## § 9. Traceability to sufficient evidence

**Governance tags:** `testing.compiler-verification-v1`

§ 9(1) Compiler-test metadata and governance must permit tests to be associated with canonical feature or integration obligations and, where relevant, regression identities.

§ 9(2) There is no required one-to-one relationship between a compiler obligation and a test.

§ 9(3) One test may verify several related obligations.

§ 9(4) One obligation may require several tests.

§ 9(5) One test is sufficient when it demonstrably covers the complete relevant contract.

§ 9(6) Additional tests are required where distinct success, rejection, boundary, target, optimization, debug, failure, or runtime behaviors form separate parts of the obligation.

§ 9(7) Governance implementation state and individual test-run history remain separate.

---

## § 10. Canonical diagnostic ownership

**Governance tags:** `testing.compiler-verification-v1`, `errors.diagnostics`

§ 10(1) `rules/tooling/diagnostics.txt` remains the sole canonical owner of:
- diagnostic IDs;
- symbolic names;
- diagnostic families;
- severity policy;
- mandatory versus configurable classification;
- occurrence structure;
- localization;
- extended explanations;
- notes and help;
- fix semantics;
- machine-readable diagnostic identity;
- LSP diagnostic identity.

§ 10(2) This rulebook introduces no parallel diagnostic identity or occurrence model.

§ 10(3) Compiler diagnostic tests use the registered stable diagnostic ID as the primary semantic identity.

§ 10(4) Tests may additionally validate the registered symbolic name and other definition metadata when those facts are relevant.

---

## § 11. Diagnostic registry and catalog conformance

**Governance tags:** `testing.compiler-verification-v1`, `errors.diagnostics`

§ 11(1) The central diagnostic registry and its exported catalog are mandatory compiler-conformance surfaces.

§ 11(2) Compiler tests must verify, as applicable:
- uniqueness of active and retired IDs;
- uniqueness and stability of symbolic names;
- valid definition metadata;
- preservation of retired IDs;
- catalog representation of every registered definition;
- preservation of canonical definition fields in machine-readable output;
- absence of unregistered emitted primary diagnostics.

§ 11(3) The canonical diagnostic inspection commands are:

```text
sec diagnostics
sec diagnostics --json
sec diagnostics <ID>
```

§ 11(4) `sec diagnostics` lists registered definitions in canonical stable ordering.

§ 11(5) `sec diagnostics --json` provides the corresponding machine-readable catalog.

§ 11(6) `sec diagnostics <ID>` provides detailed information for one diagnostic identity sufficient to help a human or machine consumer understand the rule and continue investigation.

§ 11(7) The per-ID detail must remain consistent with the registry and catalog and may include the complete rule description, common causes, valid alternatives, mandatory/configurable status, related IDs, examples, and relevant version notes.

§ 11(8) Registry/catalog tests verify definitions. Source fixtures verify concrete diagnostic occurrences.

---

## § 12. Diagnostic occurrence assertions

**Governance tags:** `testing.compiler-verification-v1`, `errors.diagnostics`

§ 12(1) Diagnostic conformance tests primarily verify the structured facts owned by the diagnostics rulebook.

§ 12(2) Depending on the diagnostic contract, those facts include:
- stable ID;
- effective severity;
- primary source span;
- related source locations;
- structured message arguments or semantic context;
- notes;
- help;
- fix presence and classification.

§ 12(3) Exact localized prose is not the semantic oracle unless the test specifically verifies rendering, message catalogs, or localization.

§ 12(4) During implementation migration, legacy tests may temporarily inspect rendered message text, but such comparison is migration debt rather than the final canonical diagnostic oracle when structured data exists.

---

## § 13. Fixture source and expectation separation

**Governance tags:** `testing.compiler-verification-v1`

§ 13(1) Compiler source input and the compiler-test expectation are separate concepts.

§ 13(2) Expectations may be stored in harness code, sidecar data, or safely interpreted fixture metadata.

§ 13(3) Compiler-testing-only Sec source syntax is not introduced for expectations.

§ 13(4) The test harness may support named source markers or equivalent metadata that resolve to canonical source spans.

§ 13(5) Such markers are test-infrastructure references and must not alter the Sec semantics being tested.

§ 13(6) The concrete source submitted to the compiler must be defined unambiguously.

---

## § 14. Diagnostic-set assertion modes

**Governance tags:** `testing.compiler-verification-v1`, `errors.diagnostics`

§ 14(1) The compiler-test harness must support at least the semantic capabilities represented by:

```text
Exact
Require
Forbid
```

§ 14(2) The exact names and storage syntax are implementation-defined.

§ 14(3) `Exact` requires the expected primary diagnostic set and rejects unexpected additional primary diagnostics.

§ 14(4) `Require` requires the listed diagnostics while intentionally permitting others.

§ 14(5) `Forbid` requires that specified diagnostics do not occur.

§ 14(6) Small focused invalid fixtures should normally use exact-set assertions so newly introduced cascades fail the test.

§ 14(7) Recovery and integration fixtures may use selective assertions when complete-set equality is intentionally not their contract.

---

## § 15. Parser and semantic recovery testing

**Governance tags:** `testing.compiler-verification-v1`, `errors.diagnostics`, `parser`

§ 15(1) Parser recovery is tested as structured compiler behavior rather than only as emitted prose.

§ 15(2) Relevant recovery tests verify:
- the expected primary diagnostic;
- the expected recovery action or recovery class;
- the relevant recovery context;
- continued parsing of independent later source;
- suppression or deduplication of same-cause cascades;
- restoration of recovery state after speculative rollback;
- bounded diagnostic and recovery behavior.

§ 15(3) Recovery-created unknown or poisoned state must not be reported as if it were a proven Sec semantic fact.

§ 15(4) When the diagnostics/parser contract defines a diagnostic cap or terminal recovery condition, boundary tests must cover behavior below, at, and beyond the relevant limit.

§ 15(5) Current host-language implementation structs are not frozen by this rulebook; their canonical observable recovery semantics are what tests preserve.

---

## § 16. Diagnostic identity across interfaces

**Governance tags:** `testing.compiler-verification-v1`, `errors.diagnostics`, `tooling.lsp`

§ 16(1) The same diagnostic occurrence must preserve the same canonical diagnostic identity across compiler, CLI, LSP, and machine-readable interfaces that expose that occurrence.

§ 16(2) Presentation and localization may differ by interface.

§ 16(3) Cross-interface tests must detect drift in diagnostic identity and, where promised by the interface, effective severity and source locations.

§ 16(4) A renderer-specific representation is not permitted to invent a different semantic diagnostic rule.

---

## § 17. Diagnostic severity and localization

**Governance tags:** `testing.compiler-verification-v1`, `errors.diagnostics`

§ 17(1) Registry default severity and effective occurrence severity are separate test concerns.

§ 17(2) Mandatory diagnostics must remain mandatory under all permitted configuration.

§ 17(3) Advisory diagnostics may change effective severity only according to the canonical diagnostic configuration rules and without changing diagnostic identity.

§ 17(4) Localization and rendering tests verify message catalogs separately from semantic diagnostic tests.

§ 17(5) Message-catalog tests must cover canonical fallback availability, required named arguments, rejection of unknown or missing arguments, and template validity where those facilities are implemented.

---

## § 18. Related diagnostic information and fixes

**Governance tags:** `testing.compiler-verification-v1`, `errors.diagnostics`

§ 18(1) When the owning diagnostic or feature rulebook requires causal source locations, concrete substitutions, target facts, notes, help, or other structured explanatory information, compiler tests must verify those facts.

§ 18(2) Emitting the correct diagnostic ID at an unrelated or materially incorrect source location is not sufficient.

§ 18(3) A machine-applicable fix is an executable contract.

§ 18(4) Tests for a machine-applicable fix must be able to:
1. compile the original fixture;
2. observe the expected diagnostic;
3. apply the proposed edit;
4. compile or otherwise validate the resulting source;
5. verify that the targeted condition is removed.

§ 18(5) Where the fix classification claims semantic preservation, the relevant semantic behavior must also be compared.

---

## § 19. Diagnostic determinism and CompilationPlan dependence

**Governance tags:** `testing.compiler-verification-v1`, `errors.diagnostics`

§ 19(1) For identical source, compiler version, compiler configuration, and concrete CompilationPlan, canonical diagnostic results must be deterministic.

§ 19(2) Parallel analysis, map iteration, worker completion order, randomized hashing, or filesystem enumeration must not leak nondeterminism into canonical diagnostic identity, structured facts, or ordering.

§ 19(3) A diagnostic fixture whose expected behavior depends on target, profile, ABI, variant, capability, or compile-time program facts must identify the relevant CompilationPlan inputs.

§ 19(4) Different explicit plans may legitimately produce different diagnostics.

§ 19(5) Repeated execution of the same declared plan must not vary without an explicitly modeled nondeterministic test dimension.

§ 19(6) Complete diagnostic-registry conformance requires every primary diagnostic the compiler can emit to participate in the canonical registry/catalog model.

---

## § 20. Compiler-stage correctness oracles

**Governance tags:** `testing.compiler-verification-v1`, `compiler.backend-architecture`

§ 20(1) Compiler testing verifies relevant compiler stages and stage boundaries separately.

§ 20(2) A successful later stage does not prove an earlier representation correct.

§ 20(3) Depending on the selected pipeline, relevant evidence may include:
- frontend semantic facts;
- Semantic IR;
- Sec MLIR or equivalent lowering representations;
- backend IR;
- object or image artifacts;
- execution.

§ 20(4) Each stage uses the kind of oracle appropriate to its contract.

§ 20(5) A stage test must distinguish structural validity from preservation of Sec semantics.

---

## § 21. Semantic IR as semantic checkpoint

**Governance tags:** `testing.compiler-verification-v1`, `lowering.semantic-ir`

§ 21(1) Semantic IR is the primary compiler-semantic checkpoint before representation-dependent lowering obscures high-level Sec facts.

§ 21(2) Semantic IR tests must be able to verify canonical facts such as, where applicable:
- binding and declaration identity;
- Place identity and projections;
- availability;
- ownership transfer and commit;
- partial availability;
- borrow relations;
- cleanup obligations;
- effect facts;
- generic specialization identity;
- source/debug provenance;
- logical task/process identity;
- transactional compiler-known facts.

§ 21(3) Structured semantic assertions are required where appropriate.

§ 21(4) Textual Semantic IR dumps are useful reviewed regression evidence but must not be the only semantic oracle when incidental ordering or formatting can change independently of the tested fact.

---

## § 22. Negative verifier testing

**Governance tags:** `testing.compiler-verification-v1`, `lowering.semantic-ir`

§ 22(1) Compiler verifier infrastructure must be tested with deliberately invalid representations.

§ 22(2) Relevant negative verifier cases include malformed types, invalid control-flow relations, illegal availability transitions, missing cleanup responsibility, incompatible operands, invalid specialization identity, stale provenance, and other verifier-owned invariants.

§ 22(3) A verifier that has only been exercised on compiler-produced valid input is insufficiently tested.

§ 22(4) Semantic IR, Sec MLIR, and later verifier layers each require negative evidence for invariants they claim to enforce.

§ 22(5) Structural verifier success never substitutes for independent Sec semantic correctness.

---

## § 23. Lowering invariants

**Governance tags:** `testing.compiler-verification-v1`, `compiler.backend-architecture`, `lowering`

§ 23(1) Lowering tests primarily assert the semantic or representation properties that must survive transformation.

§ 23(2) Tests should not freeze an exact lower-level operation sequence when the contract permits several correct realizations.

§ 23(3) Exact IR or assembly goldens are appropriate when exact representation is itself contract-relevant.

§ 23(4) Examples of invariant-oriented lowering checks include:
- exactly-once cleanup on required committed paths;
- preserved memory order;
- preserved source/debug provenance;
- correct ownership state transition;
- correct discriminant and payload relation;
- correct ABI width and calling classification.

§ 23(5) Semantically valid Sec that cannot be realized on the selected target must produce the canonical target/compiler-plan diagnostic rather than malformed lower-level IR.

---

## § 24. Backend and artifact verification

**Governance tags:** `testing.compiler-verification-v1`, `compiler.backend-architecture`, `compiler.linking`

§ 24(1) Backend and artifact tests verify physical properties when Sec, ABI, target, or artifact contracts require them.

§ 24(2) Such properties include, where applicable:
- ABI parameter and return placement;
- type layout and alignment;
- memory width;
- atomic operation and ordering;
- volatile and MMIO realization;
- interrupt entry and return conventions;
- symbol identity, binding, and visibility;
- sections;
- relocations;
- object/image metadata;
- debug encoding and correlation.

§ 24(3) Unconstrained instruction selection must not become normative through overly specific assembly goldens.

§ 24(4) Object-format structural validity and Sec-required object semantics are separate assertions.

---

## § 25. Executable conformance

**Governance tags:** `testing.compiler-verification-v1`

§ 25(1) Representative language and runtime obligations must be verified by compiling and executing programs where execution is part of the contract.

§ 25(2) Execution tests check explicit Sec-observable outcomes such as, where relevant:
- returned values;
- emitted bytes or messages;
- destruction count and order;
- panic or termination outcome;
- synchronization behavior;
- process or IPC transfer behavior;
- FFI behavior;
- artifact/runtime interaction.

§ 25(3) Successful compilation, successful linking, or absence of a crash is not sufficient when a stronger observable semantic oracle is available.

§ 25(4) Execution conformance complements rather than replaces stage-local tests.

---

## § 26. Instrumentation and deterministic fault injection

**Governance tags:** `testing.compiler-verification-v1`

§ 26(1) Compiler/runtime test environments may provide non-source-language instrumentation and deterministic fault injection.

§ 26(2) Such infrastructure may observe or force behavior including allocation failure, process creation failure, IPC transfer failure, mapping failure, I/O failure, resource-limit failure, peer death, cleanup events, or scheduler events.

§ 26(3) Test hooks are compiler/runtime test infrastructure and do not automatically become Sec source APIs or program semantics.

§ 26(4) Harness components used as correctness oracles must have their own verification.

§ 26(5) A faulty observer or injector must not be able to silently certify a faulty compiler.

---

## § 27. Differential execution

**Governance tags:** `testing.compiler-verification-v1`

§ 27(1) Differential execution is required where independently varied compiler modes are required to preserve Sec-observable semantics.

§ 27(2) Relevant dimensions include, as applicable:
- optimization classes;
- debug-information levels;
- cold versus incremental compilation under the canonical incremental contract;
- compatible target realizations.

§ 27(3) Differential testing compares canonical observable behavior, not incidental binary identity unless reproducibility or artifact identity is itself the contract.

§ 27(4) Differential testing is supplementary evidence and does not replace specification-derived oracles.

§ 27(5) Two paths that share the same faulty transformation do not become correct merely because they agree.

---

## § 28. Concurrent and stress testing

**Governance tags:** `testing.compiler-verification-v1`

§ 28(1) Thread, task, process, IPC, scheduler, and similar execution tests must use bounded harness completion.

§ 28(2) A deadlock, livelock, or non-termination that exceeds the declared test bound is a failure rather than an indefinitely stalled test run.

§ 28(3) Repeated or stress execution provides useful additional evidence for concurrency-sensitive bugs.

§ 28(4) Repetition does not prove absence of races, deadlocks, or semantic defects and does not replace static compiler invariants or deterministic conformance tests.

§ 28(5) The harness must classify compiler failure, artifact-generation failure, program execution outcome, and harness failure separately.

§ 28(6) Runtime sanitizers, race detectors, and external dynamic-analysis tools may provide additional evidence but do not define Sec correctness.

---

## § 29. Target capability evidence

**Governance tags:** `testing.compiler-verification-v1`, `platform`

§ 29(1) A target, runtime, compiler, or CompilationPlan capability claim requires sufficient conformance evidence.

§ 29(2) Positive tests verify supported behavior.

§ 29(3) Negative tests verify that explicitly unsupported behavior is rejected through the canonical diagnostic or planning path.

§ 29(4) Portable conformance fixtures should be reused across targets that declare the prerequisites required by those fixtures.

§ 29(5) Target-specific suites supplement portable suites for target-only contracts.

§ 29(6) Reproducible bugs discovered through stress, fuzzing, external tools, or target-specific execution should receive stable regression fixtures.

---

## § 30. Fuzzing as required compiler verification

**Governance tags:** `testing.compiler-verification-v1`

§ 30(1) Fuzzing is a required category of Sec compiler verification.

§ 30(2) This requirement does not mandate a particular fuzzing engine, host language, or vendor tool.

§ 30(3) Required fuzzing coverage includes, as applicable:

```text
arbitrary-source robustness
structure-aware source/compiler paths
compiler-internal representation/verifier robustness
```

§ 30(4) Compiler fuzzing defined here is distinct from future source-language fuzz declarations or a user-facing `sec fuzz` command.

---

## § 31. Fuzz oracles by input class

**Governance tags:** `testing.compiler-verification-v1`

§ 31(1) The required fuzz oracle depends on the class of generated input.

§ 31(2) Arbitrary malformed source primarily tests robustness:
- no compiler crash;
- no uncontrolled panic or assertion;
- bounded termination;
- bounded resource behavior according to the fuzz target;
- controlled diagnostic or compiler result.

§ 31(3) Arbitrary malformed input does not require a uniquely predetermined diagnostic ID for every byte sequence.

§ 31(4) Structure-aware or semantics-aware generation may use stronger semantic, differential, stage, or execution oracles when the generator establishes the necessary preconditions.

§ 31(5) Fuzzing never replaces curated diagnostic or conformance fixtures.

---

## § 32. Generated and metamorphic testing

**Governance tags:** `testing.compiler-verification-v1`

§ 32(1) Compiler testing may generate valid programs, constrained invalid programs, or semantics-preserving program variants.

§ 32(2) A generator that claims validity must define the assumptions and semantic invariants under which its output is considered valid.

§ 32(3) A metamorphic transformation must define the preconditions under which it preserves the semantic properties being compared.

§ 32(4) Binding renaming, trivia changes, redundant parentheses, or other transformations are usable only where their equivalence follows from the owning Sec rules.

§ 32(5) Generated-test infrastructure is itself test infrastructure and must be verified.

§ 32(6) Compiler correctness must not be inferred from an unverified generator merely assuming that generated programs are valid or equivalent.

---

## § 33. Replayable fuzz failures and regression promotion

**Governance tags:** `testing.compiler-verification-v1`

§ 33(1) A fuzz-discovered failure must preserve the concrete triggering input where practical.

§ 33(2) Failure provenance includes enough information to reproduce the relevant compiler configuration, CompilationPlan, fuzz target, and failure classification.

§ 33(3) A reproducible compiler defect should be reduced where practical without changing its failure predicate.

§ 33(4) Reproducible defects become stable ordinary regression evidence.

§ 33(5) Perfect minimality is not required.

§ 33(6) A larger faithful reproduction is preferable to a smaller fixture that demonstrates a different defect.

§ 33(7) The original fuzz artifact may be retained in addition to the reduced regression.

---

## § 34. Resource and termination fuzzing

**Governance tags:** `testing.compiler-verification-v1`

§ 34(1) Compiler fuzzing covers resource and termination robustness in addition to ordinary crashes.

§ 34(2) Fuzz targets use explicit time, memory, input-size, recursion/depth, or equivalent resource controls appropriate to their purpose.

§ 34(3) Pathological complexity, unbounded allocation, non-termination, and recovery loops are valid compiler-robustness findings.

§ 34(4) Purely performance- or timing-based findings must be reproduced under controlled conditions before being treated as stable conformance failures when environmental load could explain the observation.

§ 34(5) Coverage metrics may guide fuzz exploration.

§ 34(6) No coverage percentage is a correctness oracle or normative compiler-completion threshold.

---

## § 35. Corpus replay versus active fuzz exploration

**Governance tags:** `testing.compiler-verification-v1`

§ 35(1) Saved seeds and preserved fuzz regressions are replayed deterministically in appropriate regular compiler-test suites.

§ 35(2) Active coverage-guided, random, or generated exploration is a separate activity executed under an explicit time/resource budget.

§ 35(3) Active exploration may run during developer workflows, scheduled CI, continuous fuzzing, or release validation.

§ 35(4) Release or compiler correctness is never defined as “fuzzing has finished”.

§ 35(5) A release gate may require healthy fuzz targets, successful replay of saved failures, completion of configured campaigns, and absence of unresolved release-blocking findings.

---

## § 36. Verifier fuzzing and invariant mutation

**Governance tags:** `testing.compiler-verification-v1`, `lowering.semantic-ir`

§ 36(1) Semantic IR and later verifier fuzzing should include structure-aware mutation of otherwise valid representations.

§ 36(2) Such mutation deliberately violates individual invariants so the relevant verifier is expected to reject the representation cleanly.

§ 36(3) Examples include removing cleanup responsibility, changing operand type, using an unavailable Place, breaking a CFG edge, or corrupting specialization identity.

§ 36(4) Random structurally meaningless representations may supplement this strategy but do not substitute for targeted invariant mutation.

---

## § 37. Isolation and declared inputs

**Governance tags:** `testing.compiler-verification-v1`

§ 37(1) A normal compiler test must be reproducible from its declared inputs and prerequisites.

§ 37(2) A test must not depend on:
- test execution order;
- prior mutable state from another test;
- worker identity;
- undeclared environment variables;
- undeclared current-directory state;
- nondeterministic filesystem order;
- unrelated machine state.

§ 37(3) Mutable temporary and output state is isolated per test or explicitly shared through declared harness resources.

§ 37(4) Relevant environment differences are normalized, declared, or represented as test prerequisites or matrix dimensions.

§ 37(5) Locale-independent semantic diagnostic tests must not accidentally depend on process locale.

---

## § 38. Parallel safety and explicit resource serialization

**Governance tags:** `testing.compiler-verification-v1`

§ 38(1) Compiler tests should be independently runnable and parallel-safe by default.

§ 38(2) A test that requires exclusive hardware, debugger, emulator, global compiler state, port, runtime, or another mutable shared facility must declare that requirement.

§ 38(3) Harnesses should serialize on the actual shared resource where practical rather than globally serializing unrelated tests.

§ 38(4) Explicit serialization is test metadata, not an accidental property of source-file or directory order.

---

## § 39. Sharding, scheduling, and stable test identity

**Governance tags:** `testing.compiler-verification-v1`

§ 39(1) A compiler suite may be split across workers, processes, machines, or CI shards without changing test meaning.

§ 39(2) A stable compiler-test identity is independent of worker or shard assignment.

§ 39(3) Stable test identity may be derived from canonical suite, fixture, and case identity or another deterministic harness scheme.

§ 39(4) Test identity is repository/harness identity and is not a public language-level numbering scheme.

§ 39(5) Shard and worker information may be recorded as failure provenance.

§ 39(6) Randomized-order and repeat modes may be used to expose hidden shared state or nondeterminism.

---

## § 40. Reproducible randomness

**Governance tags:** `testing.compiler-verification-v1`

§ 40(1) Any test mode using random generation, randomized order, scheduler perturbation, or similar controlled nondeterminism records the effective seed and relevant configuration.

§ 40(2) For generated-source failures, the concrete triggering input should be preserved where practical.

§ 40(3) A seed alone is not a permanent regression artifact when generator evolution could change the generated program.

§ 40(4) Generator version and configuration are useful additional provenance.

---

## § 41. Flaky failures

**Governance tags:** `testing.compiler-verification-v1`

§ 41(1) If identical declared inputs produce inconsistent results, the compiler, runtime, test, or infrastructure has an unresolved defect.

§ 41(2) A flaky test is not an ordinary pass.

§ 41(3) Automatic retry may be used to classify a failure as deterministic or intermittent.

§ 41(4) Retry must not erase the original observed failure or convert it into an ordinary pass.

§ 41(5) Repeated later success does not by itself prove that the unexplained earlier failure was irrelevant.

---

## § 42. Skip and quarantine

**Governance tags:** `testing.compiler-verification-v1`

§ 42(1) `Skip`-like and `Quarantine`-like states are semantically distinct even if the harness uses different names.

§ 42(2) A skipped test is intentionally not applicable because declared prerequisites are absent.

§ 42(3) A quarantined test should be applicable but has a known unresolved reliability or correctness problem.

§ 42(4) Quarantined tests do not count as passing conformance evidence.

§ 42(5) Quarantine metadata must retain an explicit reason and sufficient provenance to investigate the problem.

§ 42(6) A test whose purpose is to verify correct rejection of an unsupported target feature is executed and is not skipped merely because the feature is unsupported.

---

## § 43. Reproduction and bounded test execution

**Governance tags:** `testing.compiler-verification-v1`

§ 43(1) A compiler-test failure must expose enough context to identify and rerun the failing case.

§ 43(2) Relevant failure context includes, as applicable:
- stable test identity;
- fixture identity or concrete generated input;
- compiler version/revision;
- target and CompilationPlan;
- optimization and debug configuration;
- seed;
- expected versus actual result;
- captured output;
- relevant generated artifacts.

§ 43(3) Every ordinary case must be independently selectable and runnable.

§ 43(4) A deliberately multi-step scenario counts as one independently runnable case.

§ 43(5) Timeouts bound execution and classify hangs or deadlocks as failures.

§ 43(6) Wall-clock sleeps should not substitute for explicit synchronization when deterministic coordination is available.

§ 43(7) Exact test-runner CLI syntax is implementation-defined.

---

## § 44. Contract-driven conformance matrices

**Governance tags:** `testing.compiler-verification-v1`

§ 44(1) Required compiler-test matrix dimensions are derived from the normative obligation being verified.

§ 44(2) Sec does not require the full Cartesian product of every target, variant, ABI, optimization, debug, backend, runtime, and feature dimension.

§ 44(3) Sec requires sufficient coverage of the distinct classes and boundaries that can materially affect the tested contract.

§ 44(4) Target-independent frontend tests need not be mechanically duplicated across all targets.

§ 44(5) ABI-, layout-, atomic-, debug-, backend-, or capability-sensitive tests include the dimensions that materially affect those contracts.

---

## § 45. Semantic cases and matrix executions

**Governance tags:** `testing.compiler-verification-v1`

§ 45(1) A canonical semantic test case and a concrete matrix execution are distinct concepts.

§ 45(2) One semantic case may instantiate into several concrete executions under different CompilationPlans or test configurations.

§ 45(3) Coverage associates the semantic case with each required matrix cell.

§ 45(4) Concrete execution records preserve the exact relevant plan and matrix dimensions.

§ 45(5) Multiple configurations of one semantic case do not become unrelated test definitions merely because they execute separately.

---

## § 46. Capability-driven positive and negative coverage

**Governance tags:** `testing.compiler-verification-v1`, `platform`

§ 46(1) Advertised target/profile facilities receive applicable positive conformance tests.

§ 46(2) Explicitly unsupported facilities receive negative rejection or planning tests where compiler-visible use exists.

§ 46(3) Runtime tests that require an unavailable facility are not applicable to that target, but non-applicability does not count as positive evidence.

§ 46(4) Portable test definitions are reused across targets that claim the required prerequisites.

§ 46(5) Each target must earn its own passing evidence; results from another target are not inherited.

§ 46(6) Adding a new target capability claim may therefore create new required matrix executions automatically.

---

## § 47. Implementation path as matrix dimension

**Governance tags:** `testing.compiler-verification-v1`, `compiler.backend-architecture`

§ 47(1) During compiler migrations, the implementation path that produced the result is a test dimension.

§ 47(2) A passing legacy lowering/backend path is not evidence that its canonical replacement path works.

§ 47(3) Tests may require a specific canonical path and explicitly forbid fallback.

§ 47(4) The harness must be able to determine which implementation path actually produced a result when multiple paths remain active.

§ 47(5) A migration is not verified merely because all tests pass through an older path.

---

## § 48. Representative mode classes

**Governance tags:** `testing.compiler-verification-v1`

§ 48(1) Optimization, debug, ABI/layout, pointer width, endianness, atomic width, and similar dimensions are included when they can materially affect the tested contract.

§ 48(2) Test matrices use representative semantically distinct classes and boundaries rather than mechanically enumerating irrelevant combinations.

§ 48(3) Canonical debug-information levels are:

```text
None
LineTables
Full
```

and are instantiated where required by the tested contract.

§ 48(4) Unoptimized and optimized classes must both be represented where optimization can change physical realization while preserving semantics.

§ 48(5) Pointer width, endianness, ABI family, and alignment classes are separate matrix dimensions only where the contract under test depends on them.

---

## § 49. Multi-Variant and cross-compilation isolation

**Governance tags:** `testing.compiler-verification-v1`, `compiler.platform-resolution`

§ 49(1) Compiler tests must include cases proving isolation between concrete CompilationPlans.

§ 49(2) Relevant isolation includes:
- Target × Variant facts;
- ABI and target-profile facts;
- artifacts;
- diagnostics;
- implementation path;
- caches when applicable.

§ 49(3) Multi-Variant tests must be capable of exercising more than one concrete plan in an interacting build context so plan leakage can be detected.

§ 49(4) Compiler-host state must not become target semantics.

§ 49(5) Host variation may be used as a conformance dimension to detect leakage of host libraries, startup files, tool defaults, filesystem paths, or other host state into a target plan.

---

## § 50. Required evidence versus CI scheduling

**Governance tags:** `testing.compiler-verification-v1`

§ 50(1) Required conformance matrix cells are defined independently of when or where project CI executes them.

§ 50(2) Expensive hardware, emulator, debugger, or platform tests may run on different schedules.

§ 50(3) A required cell that has not run does not become passing evidence.

§ 50(4) The harness must distinguish states equivalent in meaning to:

```text
Passed
Failed
NotRun
NotApplicable
Blocked
Quarantined
```

§ 50(5) Exact internal names are implementation-defined.

§ 50(6) `NotApplicable` requires an actual capability or prerequisite reason.

---

## § 51. Feature and capability closure

**Governance tags:** `testing.compiler-verification-v1`

§ 51(1) A compiler feature, target capability, or implementation area is test-conformant only when its relevant normative obligations have sufficient passing evidence across the distinct dimensions that can materially affect them.

§ 51(2) No test-count threshold, line-coverage percentage, mutation-score percentage, or raw number of matrix cells substitutes for this evidence.

§ 51(3) Known untested required cells remain visible gaps.

§ 51(4) Quarantined evidence is not passing evidence.

§ 51(5) Missing canonical-path coverage during migration remains a visible gap even when a legacy path passes.

---

## § 52. Deterministic and inspectable matrix selection

**Governance tags:** `testing.compiler-verification-v1`

§ 52(1) Given the same compiler/test corpus, toolchain definitions, target information, and test configuration, required matrix expansion and selection must be deterministic.

§ 52(2) Filesystem ordering, map iteration, worker ordering, and unrelated host state must not change which required cases are selected.

§ 52(3) The harness must be able to expose:
- why a case was selected;
- why a case was excluded;
- the concrete matrix dimensions;
- relevant capability prerequisites;
- the required implementation path.

§ 52(4) This selection/execution plan must be consumable by humans and machine tooling.

§ 52(5) The selection plan is compiler-test metadata, not Sec source semantics.

---

## § 53. Harness self-testing and fail-closed behavior

**Governance tags:** `testing.compiler-verification-v1`

§ 53(1) Compiler-test infrastructure that determines pass or fail must itself have direct tests.

§ 53(2) This includes, where implemented:
- fixture discovery;
- expectation parsing;
- `Exact`, `Require`, and `Forbid` behavior;
- source-marker resolution;
- matrix expansion and selection;
- outcome classification;
- timeout behavior;
- golden comparison;
- artifact collection;
- skip/quarantine handling;
- fault-injection infrastructure;
- test observers.

§ 53(3) Malformed or incomplete test metadata fails closed.

§ 53(4) Unknown canonical identifiers, unsupported schema fields, invalid source markers, missing required artifacts, and harness-internal errors produce explicit test or infrastructure failure rather than silent pass or accidental skip.

§ 53(5) Harness self-tests should include deliberately wrong expected/actual pairs to prove that failures are detected.

---

## § 54. Avoiding circular verification

**Governance tags:** `testing.compiler-verification-v1`

§ 54(1) A test oracle must not establish correctness merely by recomputing the expected result through the same implementation path being tested.

§ 54(2) Reuse of compiler infrastructure is permitted when it does not create a circular oracle capable of reproducing the same defect on both expected and actual sides.

§ 54(3) Rulebook-derived fixtures, independently reviewed expectations, independent calculations, and appropriate external verifiers are preferred when they increase oracle independence.

§ 54(4) A test of a layout algorithm, for example, must not treat the same layout algorithm called twice as independent evidence.

---

## § 55. Coverage as test-suite evidence

**Governance tags:** `testing.compiler-verification-v1`

§ 55(1) Code, branch, condition, feature, or obligation coverage may be used to evaluate test-suite reach.

§ 55(2) Coverage can identify important compiler paths for which no test currently provides execution evidence.

§ 55(3) Coverage does not establish semantic correctness merely because a line or branch was executed.

§ 55(4) No global coverage percentage is a normative Sec compiler correctness or completion criterion.

§ 55(5) Coverage data may inform the sufficient-evidence review required by § 9 and § 51.

---

## § 56. Mutation testing for soundness-critical logic

**Governance tags:** `testing.compiler-verification-v1`

§ 56(1) Sec compiler verification requires mutation testing or equivalent deliberate implementation-fault testing for soundness-critical compiler decisions.

§ 56(2) The requirement does not apply indiscriminately to every formatting, logging, CLI, or incidental helper.

§ 56(3) Mutation campaigns may run on schedules different from ordinary tests.

§ 56(4) Soundness-critical areas must nevertheless have evidence that plausible semantic implementation faults are detected by the relevant test suite.

§ 56(5) Important candidate areas include ownership, borrowing, type compatibility, generics, effects, cleanup, IR verification, atomics, ABI/layout, target capability checks, and process/IPC transactional state.

---

## § 57. Semantic mutation operators

**Governance tags:** `testing.compiler-verification-v1`

§ 57(1) Generic mutation operators may be used.

§ 57(2) Compiler-specific semantic mutation operators should be used where they model plausible high-risk compiler defects more directly.

§ 57(3) Examples include:
- accepting `Unavailable` as `Available`;
- omitting an explicit-move check;
- dropping a cleanup edge;
- weakening an atomic order;
- skipping a generic constraint;
- treating a capability predicate as always true;
- accepting an inactive variant payload;
- committing ownership before a fallible transfer commits.

§ 57(4) Mutation testing asks whether the test suite depends on the semantic decision, not merely whether it executes the surrounding code.

---

## § 58. Surviving mutant review

**Governance tags:** `testing.compiler-verification-v1`

§ 58(1) Mutation outcomes must distinguish at least semantically between killed, survived, equivalent, unbuildable, timed-out, and infrastructure-failure cases where those outcomes arise.

§ 58(2) Exact internal names are implementation-defined.

§ 58(3) A mutant that executes successfully without relevant test failure remains a survivor until reviewed evidence establishes that it is equivalent or otherwise irrelevant to the selected contract.

§ 58(4) A green test suite must not automatically classify a surviving mutant as equivalent.

§ 58(5) A non-equivalent survivor represents a test or specification evidence gap and must be resolved or explicitly tracked.

§ 58(6) No mutation-score percentage substitutes for survivor analysis.

---

## § 59. Golden regeneration

**Governance tags:** `testing.compiler-verification-v1`

§ 59(1) Golden artifacts may be regenerated as a maintenance operation.

§ 59(2) Regenerated expected output is a proposed oracle update, not proof that new compiler output is correct.

§ 59(3) Meaningful golden changes must remain reviewable against the canonical owning contract.

§ 59(4) Bulk golden regeneration must not silently redefine the specification as current compiler behavior.

§ 59(5) Golden output should be normalized to suppress irrelevant implementation noise while preserving semantically meaningful differences.

---

## § 60. Evidence traceability when tests change

**Governance tags:** `testing.compiler-verification-v1`

§ 60(1) Removing, replacing, merging, or materially weakening a test must not silently erase the only evidence for a normative obligation or target capability.

§ 60(2) Obligation-to-test traceability must make resulting evidence gaps detectable.

§ 60(3) A test may be retired when its evidence is redundant, obsolete, or replaced by stronger evidence.

§ 60(4) Where a harness oracle and compiler implementation are changed together, review must distinguish the compiler-contract change from the test-infrastructure change so the oracle does not become circular.

---

## § 61. External test infrastructure

**Governance tags:** `testing.compiler-verification-v1`

§ 61(1) External tools that materially affect compiler-test results must be constrained or recorded sufficiently for reproduction.

§ 61(2) Relevant external tools include, as applicable:
- LLVM tools;
- linkers;
- debuggers;
- emulators;
- simulators;
- hardware probes;
- execution providers;
- sanitizer or analysis tools.

§ 61(3) Intentional testing across several external versions is represented as an explicit matrix dimension.

§ 61(4) Uncontrolled external-tool version drift must not masquerade as compiler nondeterminism.

---

## § 62. Regression as preserved demonstrated failure

**Governance tags:** `testing.compiler-verification-v1`

§ 62(1) A regression test represents a concrete previously observed compiler defect or failure predicate.

§ 62(2) Where practical, a regression introduced with a fix must be demonstrated to fail against the defective implementation and pass with the correction.

§ 62(3) A regression must verify the violated compiler or language contract rather than merely an incidental symptom.

§ 62(4) When a runtime crash was caused by an earlier semantic defect, the permanent regression should include the earlier owning-contract assertion where practical, in addition to downstream evidence when useful.

---

## § 63. Regression reduction

**Governance tags:** `testing.compiler-verification-v1`

§ 63(1) Original user, fuzz, crash, or generated reproducers may be preserved separately from curated permanent regression fixtures.

§ 63(2) Permanent regression fixtures should remove irrelevant complexity where practical.

§ 63(3) Reduction must preserve the failure predicate.

§ 63(4) Reduction must not transform an optimizer, ownership, type, or lowering defect into an unrelated failure that merely has a similar external symptom.

§ 63(5) A larger understandable and faithful fixture is preferable to a smaller misleading fixture.

---

## § 64. Conformance versus regression provenance

**Governance tags:** `testing.compiler-verification-v1`

§ 64(1) Curated conformance tests systematically verify normative obligations.

§ 64(2) Regression tests additionally record provenance from an observed defect.

§ 64(3) A regression may also provide ordinary feature, stage, or target conformance evidence.

§ 64(4) `Regression` is therefore a classification and selection property rather than a requirement for an isolated duplicate corpus.

§ 64(5) The harness should be able to select historical regressions independently while allowing those same cases to run in their ordinary feature/stage suites.

---

## § 65. Current canonical semantics govern regressions

**Governance tags:** `testing.compiler-verification-v1`

§ 65(1) Historical regression tests do not freeze superseded language or compiler semantics.

§ 65(2) If the canonical owning rule changes legitimately, affected regression expectations must be updated, replaced, or retired with sufficient traceability.

§ 65(3) Test history must never become a shadow language specification.

§ 65(4) The expected behavior of a permanent test must remain derivable from current canonical contracts.

---

## § 66. Permanent fixture identity and provenance

**Governance tags:** `testing.compiler-verification-v1`

§ 66(1) Curated permanent compiler-test cases have stable harness identity.

§ 66(2) Permanent regression provenance retains enough information to explain why the fixture exists.

§ 66(3) Relevant provenance may include:
- affected obligation;
- bug, issue, or governance identity;
- failure classification;
- relevant matrix dimensions;
- original reproducer;
- fix revision.

§ 66(4) No new public Sec language-level numbering scheme is required for compiler tests.

§ 66(5) Human-readable fixture names may describe the semantic condition rather than encode the complete bug history.

---

## § 67. Persistent structured test metadata

**Governance tags:** `testing.compiler-verification-v1`

§ 67(1) Persistent structured compiler-test metadata must have explicit semantics.

§ 67(2) When compatibility across harness revisions requires it, the metadata format must carry an explicit schema or version identity.

§ 67(3) Unknown or unsupported fields and schema versions fail closed rather than being silently ignored.

§ 67(4) Format evolution uses controlled migration so an old strong oracle cannot silently become a weaker new oracle.

§ 67(5) Exact physical metadata format is implementation-defined.

---

## § 68. Distinct corpus roles

**Governance tags:** `testing.compiler-verification-v1`

§ 68(1) Compiler testing distinguishes at least conceptually between:
- curated conformance fixtures;
- curated regression fixtures;
- generated/property inputs;
- fuzz exploration corpus;
- external corpus.

§ 68(2) Generated transient cases need not be stored individually.

§ 68(3) Reproducible defects discovered by generation or fuzzing are promoted to preserved failure artifacts and stable regression evidence.

§ 68(4) Membership in a fuzz or exploration corpus does not make an input a normative Sec language example.

§ 68(5) Fuzz engines may prune exploration corpus data without deleting canonical regression evidence.

---

## § 69. External corpora

**Governance tags:** `testing.compiler-verification-v1`

§ 69(1) External corpora may supplement Sec compiler verification.

§ 69(2) Persistent external conformance evidence must retain sufficient provenance and a pinned version, revision, digest, or equivalent stable identity for reproduction.

§ 69(3) Applicable license and storage constraints must be respected.

§ 69(4) External expected behavior does not override canonical Sec rulebooks.

§ 69(5) Intentionally testing a moving external version is a separate compatibility probe rather than the same reproducible pinned conformance case.

---

## § 70. Active regression evidence and retirement

**Governance tags:** `testing.compiler-verification-v1`

§ 70(1) A permanent regression participates in the ordinary relevant feature, stage, and matrix suites where applicable.

§ 70(2) Historical provenance is not a reason to run a regression only as an archival suite.

§ 70(3) Tests may be removed or replaced when obsolete, redundant, or superseded.

§ 70(4) Evidence traceability must expose any newly uncovered obligation caused by retirement.

§ 70(5) Execution artifacts, crash dumps, CI logs, and temporary reduction candidates are distinct from the permanent regression corpus and need not be retained indefinitely by this rulebook.

---

## § 71. Debug-information conformance layers

**Governance tags:** `testing.compiler-verification-v1`, `compiler.debug-information-v1`

§ 71(1) Debug-information conformance is a mandatory compiler-testing category.

§ 71(2) The required layers are:
- compiler structural debug-model tests;
- physical encoding/backend tests;
- supported debugger/integration tests.

§ 71(3) Success in one layer does not substitute for the others when the selected debug contract requires all three.

§ 71(4) Presence or structural validity of debug sections alone is not Sec debug conformance.

---

## § 72. Debugger-observed contracts

**Governance tags:** `testing.compiler-verification-v1`, `compiler.debug-information-v1`

§ 72(1) Requirements involving stepping, breakpoints, source location, frame identity, variable visibility, or other debugger-observed behavior require integration tests using a supported debugger path.

§ 72(2) Harness assertions must normalize incidental debugger presentation rather than treating a complete human-readable transcript as a stable semantic golden.

§ 72(3) Canonical assertions target Sec observations such as source range, logical callable/frame identity, value availability, and concrete specialization identity.

§ 72(4) Normalization must not remove differences that violate the Sec debug contract.

---

## § 73. Independent `LineTables` and `Full` suites

**Governance tags:** `testing.compiler-verification-v1`, `compiler.debug-information-v1`

§ 73(1) `LineTables` and `Full` are tested independently.

§ 73(2) `Full` includes additional variable, type, availability, and value-location obligations but does not replace `LineTables` tests.

§ 73(3) Debug conformance matrices include representative optimization levels and target classes.

§ 73(4) Optimized debug testing is mandatory where the target claims the corresponding debug contract.

§ 73(5) Debug correctness must not rely only on unoptimized fixed stack locations.

---

## § 74. Debugger values and semantic availability

**Governance tags:** `testing.compiler-verification-v1`, `compiler.debug-information-v1`

§ 74(1) `Full` debug tests verify canonical source binding, type, value identity, and availability.

§ 74(2) Required cases include, as applicable:
- initialization;
- move;
- partial move;
- reinitialization;
- destruction;
- inactive variants;
- optimized-out values;
- constants;
- register and memory locations;
- piecewise reconstruction;
- generic substitutions;
- per-instantiation state.

§ 74(3) Stale physical bits never satisfy a requirement for a live Sec value.

§ 74(4) An unavailable result is correct when exact semantic reconstruction is impossible.

§ 74(5) A debugger must not fabricate a value merely to avoid reporting unavailability.

---

## § 75. Logical versus physical debug identity

**Governance tags:** `testing.compiler-verification-v1`, `compiler.debug-information-v1`

§ 75(1) Debug tests verify concrete generic identities, inline call chains, and logical task/thread/process relationships independently of machine address or physical stack shape.

§ 75(2) Tests must not assume one Sec logical frame equals one physical machine frame.

§ 75(3) Suspension, resumption, worker migration, inlining, outlining, code sharing, tail calls, and frame elimination must preserve the truthful logical identity required by the debug contract.

§ 75(4) Compiler-generated machinery remains synthetic for ordinary source stepping.

§ 75(5) User-authored code reached through compiler-generated machinery remains debuggable according to its own source identity.

---

## § 76. Observational safety of debugger inspection

**Governance tags:** `testing.compiler-verification-v1`, `compiler.debug-information-v1`

§ 76(1) Where the debug-information contract forbids effectful reconstruction, tests must verify that ordinary inspection does not:
- execute user code;
- invoke property accessors;
- acquire synchronization;
- perform IPC;
- advance program iterators;
- perform unsafe side-effecting device or MMIO reads.

§ 76(2) When a value cannot be observed safely or exactly, the debugger reports unavailability or the corresponding canonical unavailable state.

§ 76(3) Debugger observation does not create a Sec borrow or extend a Sec lifetime.

---

## § 77. Native debugger compatibility versus Sec-aware conformance

**Governance tags:** `testing.compiler-verification-v1`, `compiler.debug-information-v1`

§ 77(1) Standard/native debugger compatibility and canonical Sec-aware debug conformance are distinct test goals.

§ 77(2) A native debugger may expose a truthful reduced view without satisfying the complete Sec `Full` contract.

§ 77(3) A target claiming canonical `Full` support requires a supported Sec-aware path satisfying the active-feature contract.

§ 77(4) Test matrices must preserve the distinction between reduced compatibility evidence and complete Sec-aware conformance evidence.

---

## § 78. Debug artifact/source correlation and negative capability tests

**Governance tags:** `testing.compiler-verification-v1`, `compiler.debug-information-v1`

§ 78(1) Debug conformance includes, where applicable:
- embedded and separate debug artifact correlation;
- rejection of incompatible or stale companion artifacts;
- source-content mismatch detection;
- source path remapping;
- reproducibility across workspace roots;
- relocation and ASLR;
- post-link and LTO/coalescing mappings;
- unsupported level, placement, or active-feature combinations.

§ 78(2) Unsupported debug combinations must produce the canonical compile/build diagnostic.

§ 78(3) A requested debug contract must not silently downgrade to a weaker level.

§ 78(4) A stale or mismatched debug companion must not be accepted merely because it appears structurally plausible.

---

## § 79. Debug-level semantic equivalence

**Governance tags:** `testing.compiler-verification-v1`, `compiler.debug-information-v1`

§ 79(1) Representative semantic programs must be executed under:

```text
None
LineTables
Full
```

where those levels are supported for the selected target.

§ 79(2) Relevant optimization classes are combined with debug levels according to the contract-driven matrix rules.

§ 79(3) Debug configuration may change metadata and debug-only support machinery.

§ 79(4) Debug configuration must not change Sec-observable program semantics.

---

## § 80. Normative ownership of compiler testing

**Governance tags:** `testing.compiler-verification-v1`

§ 80(1) This rulebook is the canonical owner of compiler-verification methodology and shared compiler-test infrastructure semantics.

§ 80(2) It owns compiler fixtures and oracles, matrices, fuzzing, mutation testing, regression/corpus handling, harness behavior, test evidence workflows, and debugger-driving infrastructure.

§ 80(3) It does not take semantic ownership from the rulebooks defining the behavior being verified.

§ 80(4) Compiler tests are evidence for canonical contracts and do not become an independent semantic specification.

---

## § 81. Evidence obligations from semantic owners

**Governance tags:** `testing.compiler-verification-v1`

§ 81(1) A rulebook introducing or materially changing a compiler obligation should define the relevant verification obligations where required for implementation closure.

§ 81(2) The owning rulebook may require, for example, a Semantic IR invariant, a failure-injection case, a target capability test, a differential execution test, or debugger conformance.

§ 81(3) This rulebook supplies the common infrastructure and evidence semantics used to satisfy those requirements.

§ 81(4) Sufficient test evidence is part of claiming an implementation obligation verified.

§ 81(5) The semantic owner does not need to redefine compiler-test fixture syntax, matrix mechanics, fuzzing, regression, or harness semantics.

---

## § 82. Implementation governance owner

**Governance tags:** `testing.compiler-verification-v1`

§ 82(1) Compiler-testing implementation status belongs in the existing governance fragment:

```text
governance/testing.yaml
```

§ 82(2) The compiler-verification integration is:

```text
testing.compiler-verification-v1
```

§ 82(3) It is separate from:

```text
tooling.language-testing
```

which tracks source-language test semantics and `sec test`.

§ 82(4) No additional governance fragment is required for compiler testing.

§ 82(5) `governance/index.yaml` may broaden the ownership description of `testing.yaml` to include compiler-testing infrastructure without creating another semantic owner.

---

## § 83. Governance separation from feature backlogs

**Governance tags:** `testing.compiler-verification-v1`

§ 83(1) `testing.compiler-verification-v1` tracks shared compiler-test infrastructure and conformance machinery.

§ 83(2) Feature-specific governance remains responsible for its own semantic implementation and feature-specific missing evidence.

§ 83(3) Cross-links may record dependencies or gaps.

§ 83(4) The same feature backlog must not be duplicated in both the feature ledger and compiler-testing ledger.

§ 83(5) Test execution history is not implementation governance.

§ 83(6) Governance may record verification commands or representative evidence, but ordinary CI run history belongs to test or CI infrastructure.

---

## § 84. Compiler-testing integration completion

**Governance tags:** `testing.compiler-verification-v1`

§ 84(1) The compiler-testing integration is not complete merely because host unit tests exist or because the repository contains many test files.

§ 84(2) Completion requires implemented and verified infrastructure for all applicable mandatory categories defined by this rulebook, including:
- stage-local and end-to-end verification;
- structured invalid diagnostics and recovery testing;
- Semantic IR and lowering tests;
- negative verifier testing;
- backend/artifact inspection;
- executable conformance;
- matrices;
- fuzzing and corpus replay;
- isolation and reproducibility;
- harness self-tests;
- mutation testing;
- regression provenance and corpus lifecycle;
- debug structural, physical-format, and debugger integration testing.

§ 84(3) No raw test-count, line-coverage percentage, or mutation-score percentage substitutes for these contracts.

---

## § 85. Infrastructure completion versus feature conformance

**Governance tags:** `testing.compiler-verification-v1`

§ 85(1) Complete compiler-testing infrastructure means Sec has the mechanisms required to produce, execute, classify, and audit canonical compiler evidence.

§ 85(2) It does not mean every compiler feature or target is implemented or conformant.

§ 85(3) A feature may remain partial while the shared compiler-testing infrastructure is complete.

§ 85(4) Conversely, many feature-specific ad-hoc tests do not make compiler-testing infrastructure complete when mandatory shared capabilities remain missing.

---

## § 86. Implementation-language neutrality and source exclusions

**Governance tags:** `testing.compiler-verification-v1`, `tooling.language-testing`

§ 86(1) This rulebook does not require the compiler or compiler-test harness to use Go, Sec, C++, or another specific implementation language.

§ 86(2) It introduces no source-visible:
- `CompilerTest` type;
- `FuzzCase` type;
- `Mutation` type;
- compiler-test attribute;
- compiler-test source directive;
- test reflection API.

§ 86(3) Ordinary Sec source and source-level tests may be used as compiler fixtures.

§ 86(4) `rules/tooling/testing.md` remains canonical for `test`, `testing.*`, `TestCompilationPlan`, and `sec test`.

§ 86(5) Compiler fuzzing required by this rulebook does not define source-language fuzz declarations or a `sec fuzz` command.

---

## § 87. Relationship to incremental compilation

**Governance tags:** `testing.compiler-verification-v1`

§ 87(1) This rulebook owns general test machinery capable of comparing cold and incremental compilation.

§ 87(2) `rules/compiler/incremental_compilation.md` owns incremental compiler semantics, including dependency tracking, invalidation, cache identity, legal reuse, stale-state prevention, and target/generic specialization separation.

§ 87(3) Incremental compiler-testing obligations are derived from that owning contract rather than guessed or duplicated here.

§ 87(4) This rulebook consumes the feature-specific obligations defined by the canonical incremental rulebook and does not invent or duplicate cache or invalidation semantics merely to create tests.

---

## § 88. Integration and completion audit

**Governance tags:** `testing.compiler-verification-v1`

§ 88(1) Integrating this rulebook requires cross-rulebook harmonization without transferring semantic ownership.

§ 88(2) The integration must include:
- canonical documentation and tests for `sec diagnostics <ID>`;
- clarification in `rules/tooling/testing.md` that deferred source-level fuzz syntax does not defer mandatory compiler fuzzing;
- completion of the debug-information handoff to compiler-owned debugger-driving test infrastructure;
- addition of `testing.compiler-verification-v1` to `governance/testing.yaml` without duplicating compiler or feature governance;
- an appropriate ownership-description update in `governance/index.yaml`;
- update of `language-rulebook-status.md` from `Planned` to `Written`.

§ 88(3) Before this rulebook is considered design-complete, every locked compiler-testing decision represented by §§ 1–88 must be auditable in the final text and governance handoff.

§ 88(4) Any semantic conflict discovered by compiler testing is resolved in the owning canonical rulebook or correction mechanism rather than silently redefining behavior in a test.

§ 88(5) Implementation findings may reveal genuine specification defects, but a failing test does not by itself authorize an implementation-local semantic exception.

---

# Appendix A. Canonical evidence model

This appendix summarizes the normative structure above without introducing additional rules.

```text
Canonical rulebook / target / ABI contract
                    |
                    v
          compiler obligation
                    |
                    v
       sufficient test evidence
                    |
       +------------+------------+
       |            |            |
       v            v            v
 stage-local    matrix cells   end-to-end
 evidence       / targets      evidence
       |            |            |
       +------------+------------+
                    |
                    v
           conformance status
```

A green test is evidence only for the contract and matrix conditions it actually verifies.

A large test count is not a substitute for traceability.

A successful backend is not a substitute for frontend or Semantic IR correctness.

A successful executable is not a substitute for verifier and lowering evidence.

A valid lower-level representation may still implement the wrong Sec program.

---

# Appendix B. Canonical compiler-test class summary

```text
Unit
    Isolated implementation behavior.

Frontend/Semantic
    Source acceptance and canonical semantic analysis.

Invalid/Diagnostic
    Required rejection, structured diagnostics, and recovery.

Semantic IR
    Canonical compiler-semantic facts before representation-dependent lowering.

Lowering/Representation
    Preservation of required invariants into lower representations.

Backend/Codegen
    Target, ABI, object, instruction-property, and artifact realization.

Execution/Conformance
    Observable Sec behavior of generated programs.

Regression
    Preserved evidence for a demonstrated prior compiler defect.
```

A test may belong to several classes.

---

# Appendix C. Diagnostic compiler-test summary

The diagnostics rulebook remains authoritative.

Compiler-testing consumes at least:

```text
sec diagnostics
sec diagnostics --json
sec diagnostics <ID>
```

Focused invalid fixtures normally verify:
- registered diagnostic ID;
- effective severity;
- exact primary diagnostic set;
- canonical source span;
- required related source information;
- required structured context;
- required fix classification.

Rendering and localization tests verify prose separately.

---

# Appendix D. Matrix summary

A semantic test case may expand into several concrete executions.

Typical dimensions include only those relevant to the tested contract:

```text
Target
Variant
TargetProfile
ABI
Optimization
DebugInformationLevel
debug placement
implementation/lowering path
runtime/execution provider
feature/capability set
host, only where host leakage or host compatibility is being tested
```

The complete Cartesian product is not required.

Every required cell must remain distinguishable from `NotRun`, `NotApplicable`, `Blocked`, or `Quarantined` evidence.

---

# Appendix E. Decision audit D1-D88

This appendix is a review aid. The normative requirements are the numbered sections above.

| Decision | Normative home |
| --- | --- |
| D1 | §§ 1-3 |
| D2 | § 3 |
| D3 | § 4 |
| D4 | § 5 |
| D5 | § 6 |
| D6 | §§ 7-8 |
| D7 | § 8 |
| D8 | §§ 5, 23, 59 |
| D9 | § 9 |
| D10 | § 10 |
| D11 | § 11 |
| D12 | § 12 |
| D13 | § 13 |
| D14 | § 14 |
| D15 | § 15 |
| D16 | § 16 |
| D17 | § 17 |
| D18 | § 18 |
| D19 | § 19 |
| D20 | § 20 |
| D21 | § 21 |
| D22 | § 22 |
| D23 | § 23 |
| D24 | § 24 |
| D25 | § 25 |
| D26 | § 26 |
| D27 | § 27 |
| D28 | § 28 |
| D29 | § 29 |
| D30 | § 30 |
| D31 | § 31 |
| D32 | § 32 |
| D33 | § 33 |
| D34 | § 34 |
| D35 | § 35 |
| D36 | § 36 |
| D37 | § 37 |
| D38 | § 38 |
| D39 | § 39 |
| D40 | § 40 |
| D41 | § 41 |
| D42 | § 42 |
| D43 | § 43 |
| D44 | § 44 |
| D45 | § 45 |
| D46 | § 46 |
| D47 | § 47 |
| D48 | § 48 |
| D49 | § 49 |
| D50 | § 50 |
| D51 | § 51 |
| D52 | § 52 |
| D53 | § 53 |
| D54 | § 54 |
| D55 | § 55 |
| D56 | § 56 |
| D57 | § 57 |
| D58 | § 58 |
| D59 | § 59 |
| D60 | § 60 |
| D61 | § 61 |
| D62 | § 62 |
| D63 | § 63 |
| D64 | § 64 |
| D65 | § 65 |
| D66 | § 66 |
| D67 | § 67 |
| D68 | § 68 |
| D69 | § 69 |
| D70 | § 70 |
| D71 | § 71 |
| D72 | § 72 |
| D73 | § 73 |
| D74 | § 74 |
| D75 | § 75 |
| D76 | § 76 |
| D77 | § 77 |
| D78 | § 78 |
| D79 | § 79 |
| D80 | § 80 |
| D81 | § 81 |
| D82 | § 82 |
| D83 | § 83 |
| D84 | § 84 |
| D85 | § 85 |
| D86 | § 86 |
| D87 | § 87 |
| D88 | § 88 |

---

# Appendix F. Sec 0.1 exclusions

This rulebook does not define:
- source-language fuzz declarations;
- a `sec fuzz` source-testing command;
- source-level mutation testing;
- a public compiler-test source DSL;
- one mandatory host test framework;
- one mandatory fuzzing engine;
- one mandatory debugger;
- one universal hardware test protocol;
- one universal CI scheduler;
- one universal test-result byte protocol;
- one universal persistent test-metadata format;
- a fixed global line-coverage percentage;
- a fixed mutation-score percentage;
- a requirement to execute the full Cartesian product of all test dimensions.
