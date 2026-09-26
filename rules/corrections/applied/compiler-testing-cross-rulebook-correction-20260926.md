# Compiler Testing Cross-Rulebook Correction

- **Status:** Applied correction
- **Created:** 2026-09-26
- **Last updated:** 2026-09-26
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/corrections/applied/compiler-testing-cross-rulebook-correction-20260926.md`
- **Applied:** 2026-09-26
- **Trigger:** Integration of `rules/compiler/compiler_testing.md` revision 1.0
- **Primary governance integration:** `testing.compiler-verification-v1`
- **Affected canonical owners:** diagnostics, source-level testing, debug information, rulebook status, testing governance

---

## § 1. Purpose

§ 1(1) This correction records the cross-rulebook synchronization required by `rules/compiler/compiler_testing.md`.

§ 1(2) It does not transfer semantic ownership from the affected rulebooks.

§ 1(3) The compiler-testing rulebook defines verification machinery. Each affected rulebook remains authoritative for the behavior being verified.

§ 1(4) The changes below were applied to their canonical owners on 2026-09-26, and this correction is archived in the repository's applied-corrections location.

---

## § 2. Canonical diagnostic detail command

**Target:** `rules/tooling/diagnostics.txt`

§ 2(1) The canonical diagnostic catalog and inspection command family is:

```text
sec diagnostics
sec diagnostics --json
sec diagnostics <ID>
```

§ 2(2) `sec diagnostics <ID>` is the canonical detailed lookup for one registered diagnostic ID.

§ 2(3) The detailed form is intended to help both human and machine-assisted investigation continue from a diagnostic ID.

§ 2(4) The detailed result may contain:
- the stable diagnostic ID;
- stable symbolic name;
- family;
- default severity;
- mandatory/configurable classification;
- current short explanation;
- extended rule explanation;
- common causes;
- valid alternatives or correction guidance;
- examples;
- related diagnostic IDs;
- relevant Sec/compiler version notes.

§ 2(5) Information returned by `sec diagnostics <ID>` must be derived from or checked against the same canonical registry/catalog ownership as `sec diagnostics` and `sec diagnostics --json`.

§ 2(6) The section currently titled **Diagnostic explanation command**, which presents:

```text
sec explain S3101
```

must be updated to use:

```text
sec diagnostics S3101
```

§ 2(7) No separate canonical `sec explain` interface is required by Sec 0.1 after this correction. An implementation may temporarily retain an alias during migration only if it does not create a second semantic source of diagnostic information.

§ 2(8) The current implementation-status section must add the detailed lookup as remaining work until `sec diagnostics <ID>` is implemented and covered by registry/catalog consistency tests.

---

## § 3. Diagnostic testing handoff

**Target:** `rules/tooling/diagnostics.txt`

§ 3(1) The diagnostics rulebook remains canonical for diagnostic identity, structure, localization, severity, help, fixes, and configuration.

§ 3(2) Its compiler-test guidance cross-references `rules/compiler/compiler_testing.md` for shared harness behavior, including:
- exact/required/forbidden diagnostic-set assertions;
- fixture/source-marker mechanics;
- recovery-suite execution;
- deterministic matrix execution;
- cross-interface compiler/CLI/LSP checks;
- machine-applicable-fix execution tests;
- evidence traceability.

§ 3(3) The diagnostics rulebook should not duplicate those harness rules.

---

## § 4. Diagnostic prefix allocation inconsistency is not resolved here

**Targets:** `rules/tooling/diagnostics.txt`, `internal/diagnostics/diagnostics.go`

§ 4(1) Current canonical text recommends lexical/tokenization allocation in `P1xxx` while the current registry contains published lexer IDs `L1001` through `L1020`.

§ 4(2) Published diagnostic IDs are intended to be stable and must not be casually renumbered.

§ 4(3) `rules/compiler/compiler_testing.md` does not choose between preserving the `L1xxx` family and changing the written allocation policy, nor does it authorize renumbering.

§ 4(4) The diagnostics owner must resolve this inconsistency explicitly before complete registry conformance is claimed.

§ 4(5) Governance should retain this as an identified bug until that owner-level decision is applied.

---

## § 5. Source-level testing and compiler fuzzing

**Target:** `rules/tooling/testing.md`

§ 5(1) Existing statements that fuzzing is a future design area refer to **source-language fuzzing semantics**.

§ 5(2) They must not be read as deferring compiler-implementation fuzzing.

§ 5(3) Update the non-goal/future-work wording to make the distinction explicit:

```text
Source-language fuzz declarations, input-generation semantics, shrinking semantics,
and a user-facing `sec fuzz` command remain future source-testing design areas.

Compiler implementation fuzzing is separate and is required by
`rules/compiler/compiler_testing.md`.
```

§ 5(4) `rules/tooling/testing.md` remains authoritative for source-visible testing and must not acquire the compiler fuzzing contracts from `rules/compiler/compiler_testing.md`.

---

## § 6. Debug-information handoff

**Targets:** `rules/compiler/debug_information.md`, `governance/compiler.yaml`

§ 6(1) The existing normative relationship in `debug_information.md` is already directionally correct: debug information defines the behavior and conformance categories, while `rules/compiler/compiler_testing.md` owns shared compiler-test harness and debugger-driving infrastructure.

§ 6(2) No debug semantic rule is moved.

§ 6(3) Update `compiler.debug-information-v1` governance reconciliation from a future handoff to an active dependency on `testing.compiler-verification-v1`.

§ 6(4) Feature-specific missing debug evidence remains in `compiler.debug-information-v1`; shared debugger adapter/harness implementation remains in `testing.compiler-verification-v1`.

§ 6(5) Do not duplicate the complete debug-information backlog into `governance/testing.yaml`.

---

## § 7. Testing governance integration

**Target:** `governance/testing.yaml`

§ 7(1) Add the integration:

```text
testing.compiler-verification-v1
```

beside, and distinct from:

```text
tooling.language-testing
```

§ 7(2) `tooling.language-testing` continues to track source-language test semantics and `sec test` implementation.

§ 7(3) `testing.compiler-verification-v1` tracks shared compiler-test infrastructure, evidence mechanics, fuzzing, mutation testing, compiler matrices, regression/corpus infrastructure, and debugger-driving test infrastructure.

§ 7(4) The companion `compiler-testing-governance-handoff.yaml` was the canonical merge payload for this integration and was removed after its contents were merged.

§ 7(5) Feature-specific semantic and target implementation backlogs remain with their current governance owners.

---

## § 8. Governance index ownership description

**Target:** `governance/index.yaml`

§ 8(1) The existing `testing.yaml` fragment remains the single testing-governance owner.

§ 8(2) Broaden its ownership description from language-only wording to wording equivalent to:

```text
testing.yaml
    owns: language and compiler testing infrastructure, conformance evidence, and test-tooling status
```

§ 8(3) Do not create a new `compiler_testing.yaml` governance fragment.

---

## § 9. Rulebook status

**Target:** `language-rulebook-status.md`

§ 9(1) Change the compiler-testing row from `Planned` to `Written`.

§ 9(2) Its description should identify at least:
- compiler test classes;
- structured diagnostic/recovery verification;
- Semantic IR and lowering verification;
- backend/artifact and executable conformance;
- fuzzing and mutation testing;
- deterministic isolation and matrix execution;
- regression/corpus provenance;
- debug-information conformance infrastructure;
- governance integration `testing.compiler-verification-v1`.

§ 9(3) Add `compiler/compiler_testing.md` to the canonical written-rulebook set.

§ 9(4) Remove `compiler_testing.md` from the planned-rulebook set.

§ 9(5) Keep `incremental_compilation.md` planned until that rulebook is written.

§ 9(6) This status update does not assert that the compiler-testing implementation is complete. Documentation status and implementation status remain separate.

---

## § 10. Incremental-compilation boundary

**Targets:** `rules/compiler/compiler_testing.md`, future `rules/incremental_compilation.md`

§ 10(1) `rules/compiler/compiler_testing.md` owns generic differential and matrix machinery capable of comparing cold and incremental compilation.

§ 10(2) The future incremental rulebook remains the semantic owner of dependency tracking, invalidation, cache identity, legal reuse, stale-state prevention, and target/generic-specialization cache separation.

§ 10(3) No cache semantic should be added to `rules/compiler/compiler_testing.md` merely to make an incremental test possible before that owner rulebook defines the behavior.

---

## § 11. Application checklist

§ 11(1) This correction is complete when all of the following are true:

```text
[x] rules/compiler/compiler_testing.md is installed at its canonical path
[x] sec diagnostics <ID> is canonical in rules/tooling/diagnostics.txt
[x] source-level fuzz deferral is distinguished from compiler fuzzing
[x] testing.compiler-verification-v1 is merged into governance/testing.yaml
[x] governance/index.yaml testing ownership wording is broadened
[x] compiler.debug-information-v1 points to the active compiler-testing integration
[x] language-rulebook-status.md marks compiler/compiler_testing.md Written
[x] compiler/compiler_testing.md is removed from the planned rulebook list
[x] diagnostic prefix allocation drift remains tracked until diagnostics ownership resolves it
```

§ 11(2) Applying this correction must not renumber unrelated stable rulebook paragraphs merely to insert cross-references.
