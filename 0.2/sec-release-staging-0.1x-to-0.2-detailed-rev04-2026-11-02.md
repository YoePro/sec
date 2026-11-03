# Sec 0.1.x → 0.2.0 — Detailed Language, Rulebook, API/ABI and Delivery Roadmap

**Created:** 2026-11-02  
**Updated:** 2026-11-02  
**Document revision:** 0.4 (supersedes revision 0.3; preserves all existing section identifiers; adds §§17–19)  
**Status:** Owner-directed release model; detailed **candidate** allocation, not a normative language rulebook  
**Language milestone:** all accepted Sec 0.2.0 features must first ship in a numbered Sec 0.1.x release (`1 ≤ x ≤ 100`)  
**Source idea snapshot:** `Sec-0.2-Ideas-Final-2026-10-29.md`, §§1–34  
**Prior release plan:** `sec-release-staging-0.1x-to-0.2-revised-2026-11-02.md`, rev. 0.2  
**Repository baseline reviewed:** `YoePro/sec`, `main` = `cbd88bdd32b9d4b4ff58d3a27927ced4e2822600` (2026-11-01; verified unchanged during 2026-11-02 review)  
**Decision authority:** Sec owner. Proposals, preferred directions, settled design decisions and verified implementation are distinct.  
**Procedure amendment:** 2026-11-02 — mandatory per-version document, critical design review, selective MD intake and complete per-version handoff.  
**Version-document template:** `sec-release-version-document-template-2026-11-02.md` (supporting non-normative template).  
**Revision discipline:** Preserve section identifiers for existing references; append new numbered paragraphs or use corrections rather than renumbering normative rulebooks. No files in `rules/` or `governance/` are changed by this roadmap.  

> **Owner direction, 2026-11-02:** `rational` and `complex` are intended to be **first-class, lowercase, compiler-known language types**, not ordinary `Rational[T]` or `Complex[T]` stdlib nominals. Detailed representation, arithmetic, parameterization, conversion and ABI contracts remain undecided until specified.
>
> **Owner's linearity suggestion:** investigate type-level `@mustUse` as the declaration mechanism for linear obligations. This is a preferred *candidate*, not proof that the existing must-use-result behavior implements exact-once resource consumption.
>
> **Owner procedure, 2026-11-02:** Every `0.1.x` release has at least one version document, written as a complete specification and implementation checklist **before** its release. A later release may be designed or implemented before an earlier one is published, but its version gate cannot be marked complete while unmet dependencies remain. Every accepted feature must survive a rigorous challenge to its necessity, ergonomics, intuition and interaction with the existing language.  
>
> **0.2.0 release promise:** integrate, verify and document the previously delivered accepted feature set; introduce **no new source-language feature** in `0.2.0` itself. `stable` at 0.2 means a stable **feature-complete baseline within its declared support matrix**, not mature production stability. Full production-grade maturity may begin around 0.6.0, provisionally.

---

## § 1. Versioning invariants

**§ 1(1) Feature delivery before milestone.** Every accepted source-language feature promised by 0.2.0 has an exact introduction release in `0.1.1`–`0.1.100`, with normative specification, implementation and verification completed by that release. An idea not accepted may remain a research item or be deferred; do not silently count it as part of the required 0.2 inventory.

**§ 1(2) The ceiling is not a schedule.** One hundred intermediate slots are available, but empty versions are unnecessary. Sections below propose approximately fifty meaningful delivery boundaries to reduce large atomic implementation packages. Release numbering remains adjustable until owner-approved and published.

**§ 1(3) Existing reservations are preserved.** `Owner[T]` (working spelling) is already targeted at `0.1.1`; contextual `Err(.Variant...)` patterns at `0.1.2`; the regex rulebook under MD-010 is added to `0.1.1`. Existing references that call standalone `struct Name` or linear types "Sec 0.2" must be synchronized **after approval** with their actual first `0.1.x` delivery version.

**§ 1(4) First-class numeric owner direction.** `rational` and `complex` are compiler-known lowercase types. They require native type identities and source/CTE/Sema/Semantic IR/MLIR/ABI/runtime contracts; do not satisfy their release with a `stdlib/math` nominal alias. A generic representation such as `complex[T]`, a particular width or arbitrary-precision storage is *not* approved merely by this classification.

**§ 1(5) Linear candidate.** Type-level `@mustUse` is the preferred spelling to investigate for linear obligations. The exact operator/lifetime meaning and interaction with existing Result-use obligations must be defined explicitly; no `@linear` keyword is automatically added.

**§ 1(6) Already implemented ≠ feature backlog.** Numeric digit separators, ordinary multi-argument overload resolution, selected contextual `.Member` forms, `list[T, Capacity]`, `vector`/`matrix`/`tensor`, built-in `never` semantics in the IR and heterogeneous type packs already have Sec 0.1 coverage. Check exact scope and implementation before making new tickets.

**§ 1(7) Independent pipelines.** Work that improves MLIR optimization, Sema, LSP, diagnostics, formatting, backend portability, core/stdlib, platform rules, or ABI verification without changing source-language contracts may ship continuously. A language version is required for new source-visible guarantees/behavior, not for every implementation package.

**§ 1(8) One authoritative source.** `rules/` determines normative semantics, `governance/` tracks implemented/partial/planned truth, correction/MD registers carry unresolved decisions, and this roadmap tracks release intent. Never use the roadmap to overwrite or duplicate governance status.

**§ 1(9) Ergonomics and intuition.** Every language feature requires realistic examples, common-path convenience, predictable behavior, evidence that annotations communicate genuine intent, diagnostics and IDE expectations. Do not make a syntax improvement by hiding copying, moving, allocation, blocking, errors or effects.

**§ 1(10) A release version is a verification gate.** No "0.1.x complete" merely because a book is written or a parser recognizes syntax. Canonical semantics, public API, ABI or a documented no-ABI-change proof, compiler/native implementation, regression tests, and documentation must be demonstrably coherent.

**§ 1(11) Version documents are compulsory.** For **every** accepted/published `0.1.x` version, create and maintain at least one version-specific document under an owner-approved path, using §18. Its purpose is to make the entire release executable as a work plan: what must be specified, implemented, tested, synchronized and explicitly left out. Additional feature packets and rulebooks may accompany it, but cannot replace the version document.

**§ 1(12) Definition precedes completion.** A version's complete design/implementation plan must pass the §17 critical review before the version can be released. Work on a later version may proceed in parallel or out of numeric order when dependencies permit. Starting or even finishing such work does not backdate the version's normative status or release readiness.

**§ 1(13) Existing 0.1 decisions may be 0.1.1 release obligations.** MD-117 heterogeneous native variadics and MD-120 generic error-category parameters/declarations are **Sec 0.1 normative scope**. They are added to the `0.1.1` *integration and conformance* checklist by owner direction, without retroactively changing their normative language-version declarations. `0.1.1` must track what is already implemented versus still missing; a parser acceptance test is not sufficient to claim native availability.

**§ 1(14) Selective missing-decision intake.** A substantial unresolved MD may be assigned to a `0.1.x` version if it closes a real language gap, unblocks a required capability or produces a significant ergonomic/safety benefit. The owner approves inclusion. MD IDs and existing governance entries remain authoritative; the roadmap and version document only reference and assign them, never duplicate the same state.

**§ 1(15) Language design is not feature accumulation.** Every item must justify its complexity against a real programming problem and show why existing Sec mechanisms, core/stdlib APIs, compiler analysis or tooling cannot adequately solve it. Avoid both gratuitous minimalism and speculative language growth. **Ergonomics and intuition are release acceptance criteria**, not optional polish.

---

## § 2. Detailed pre-0.2 release allocation (provisional)

Each entry is a *candidate release target*. Numbers are neither promises nor approval of unresolved features. The rightmost column cites idea-book sections and the most important ship criterion. Full per-release checklists appear in §8; source-to-release coverage appears in §12.

| Gate | Theme | Direction / acceptance status | Idea source | Ship condition |
|---|---|---|---|---|
| **0.1.1** | Owning indirection, regex rulebook, heterogeneous packs and generic errors | owner-assigned release scope; MD-117/MD-120 already normative for 0.1 | §18, §10; MD-010, MD-117, MD-120 | Finish agreed `Owner[T]` scope and the accepted regex rulebook; complete/verify existing 0.1 MD-117 type-pack and MD-120 error-generic semantics end-to-end, with exact remaining implementation work tracked, no duplicate decisions or false implementation claims. |
| **0.1.2** | Contextual Result error patterns | preassigned | §7 | `Err(.Variant...)` matches unambiguous closed error types, without widening expression syntax. |
| **0.1.3** | Declaration ceremony reduction I | candidate | §2.1, §31 | Omit `let` only in syntactically unambiguous binding contexts; keep distinction between new binding and assignment. |
| **0.1.4** | Contextual enum and union names | candidate | §4.1, §31 | Extend only genuinely missing contextual `.Member` forms; preserve explicit typing on ambiguity. |
| **0.1.5** | Context-directed inference consistency | candidate | §4.3, §20 | Bidirectional inference where one expected type exists, without return-type-driven overload selection. |
| **0.1.6** | Conditional value expression | candidate | §5.1 | Add conditional value selection `?:` with precise grouping, branch typing and evaluation. |
| **0.1.7** | Short context-typed lambdas | candidate | §8.1 | Allow concise lambdas when context supplies complete parameter types without guessing. |
| **0.1.8** | Regex execution and named contracts | candidate implementation gate | MD-010; §24 | Deliver regex engine and public runtime API from 0.1.1 specification, including named-string contract validation. |
| **0.1.9** | Ranges in collection literals | candidate | §3.1 | Literal range expansion with finite extent and familiar evaluation order; preserve collection storage contracts. |
| **0.1.10** | Membership and value-set expressions | candidate | §6 | Extend `in`/`not in` only where semantics of a set/pattern are statically defined. |
| **0.1.11** | General nested patterns | candidate | §7.1 | Permit direct nested destructuring in `match`, beyond narrow 0.1 Result exceptions. |
| **0.1.12** | Ownership-aware pattern ergonomics | candidate | §7.2, §11 | Use minimal authority needed by binding uses, with explicit exceptions when inference is unsafe. |
| **0.1.13** | Value blocks and multi-condition value selection | candidate | §8.2, §5.2 | Local value-producing blocks and a coherent multi-condition value-selection form with defined scopes and early exits. |
| **0.1.14** | Direct declaration forms | candidate migration | §2.2, §2.3 | Introduce direct `struct Name`, `union Name` etc only after consistent grammar and migration plan. |
| **0.1.15** | Flow-sensitive facts exposed coherently | candidate source semantics | §4.2, §21 | Make proven range, availability, variant and capability facts consistently usable in source. |
| **0.1.16** | Reference shorthand for reference holders | candidate | §10.1, §14 | Avoid redundant `ref` only where a value already carries correct borrowed authority. |
| **0.1.17** | Multiple-origin provenance | candidate | §10.2, §11 | Retain reference origins across branches without obligating programmers to name lifetimes. |
| **0.1.18** | Nonescapable values and views | candidate | §10.3, §9.3 | Define nonescaping capabilities as a concrete constraint when compiler inference cannot suffice. |
| **0.1.19** | Place and projection ergonomics | candidate | §11, §12 | Unify expression Place identity for field/index/property/method call and assignment. |
| **0.1.20** | Construction: required fields | candidate | §15.1 | Require explicit initialization of fields without a defined default, avoiding surprising zero/default insertion. |
| **0.1.21** | Construction: visibility and factory boundaries | candidate | §15.2 | Permit control over direct construction without forcing boilerplate on ordinary types. |
| **0.1.22** | Function argument labels | candidate | §16 | Express informative parameter labels consistently, without inventing implicit dependency injection. |
| **0.1.23** | Public `never` type semantics | candidate (IR already known) | §28.3 | Expose the already-known uninhabited type consistently in valid source type positions. |
| **0.1.24** | Float16 numeric family | candidate | §28.1 | Explore `float16` as first-class numeric scalar only after storage/arithmetic and target profile are defined. |
| **0.1.25** | Bfloat16 numeric family | candidate | §28.1 | Explore `bfloat16` independently of `float16`; do not silently equate storage and compute formats. |
| **0.1.26** | First-class `rational`: type and arithmetic | OWNER-DIRECTED first-class; semantics pending | §28.1, §28.5 | Specify and implement lowercase compiler-known `rational`; no `Rational[T]` stdlib substitute. |
| **0.1.27** | `rational` integration, exact unit scales | OWNER-DIRECTED first-class; integration decision pending | §28.1, §28.5 | Connect rational to existing units only where a *separate* unit metadata decision permits it. |
| **0.1.28** | First-class `complex`: core type and operations | OWNER-DIRECTED first-class; semantics pending | §28.1, §29.2 | Specify and implement lowercase compiler-known `complex`; avoid duplicate `Complex[T]` nominal API. |
| **0.1.29** | `complex` integration and numeric algorithms | OWNER-DIRECTED first-class; integration decision pending | §29.2, §29.5 | Connect builtin `complex` to stable public math and generic algorithm contracts. |
| **0.1.30** | Operator annotations: declaration and lookup | candidate | §30.1–30.4 | Use explicit annotation of ordinary static overloads, proposal spelling `@operator("+")`. |
| **0.1.31** | Operator annotations: advanced interactions | candidate | §30.2–30.6 | Resolve mixed user types, generic operator requirements and compound assignment without violating Place evaluation. |
| **0.1.32** | Generic numerical capability contracts | candidate | §20, §29, §30 | Make generic numerical algorithms usable for builtin and user mathematical carriers without a second dispatch model. |
| **0.1.33** | Mathematical library breadth | library delivery; language change only if needed | §29.3–29.5 | Define `Quaternion[T]`, `Dual[T]`, `Polynomial[T]` as nominal library candidates; separate optional `UnitQuaternion[T]` and `Rotation3[T]`. |
| **0.1.34** | Generic capability checking | candidate | §20, §21 | Derive actionable constraints from generic body usage without requiring redundant signatures. |
| **0.1.35** | Equivalent ownership implementations | candidate | §13, §25 | Allow semantic-equivalent storage/call implementations where optimizer proves identical observable behavior. |
| **0.1.36** | Type-level `@mustUse` linear declarations | OWNER SUGGESTION; semantics pending | §18.1–18.2, §14 | Investigate `@mustUse` before type definitions to designate a mandatory exact-once lifetime obligation. |
| **0.1.37** | Linear terminal protocols and ownership | candidate | §18, §23.2, §23.3 | Apply @mustUse-like obligations to reservations, transactions, resource acknowledgements and terminal states where accepted. |
| **0.1.38** | Linear generics and aggregates | candidate | §9.4, §18, §20 | Make exact-once obligations compose through Option, Result, arrays, containers, generics and recursion. |
| **0.1.39** | Copy/move ergonomic simplification | candidate | §14, §11 | Remove only redundant transfer notation where ownership transfer is statically unavoidable; protect explicit ownership intent. |
| **0.1.40** | Purity and verified effect promises | candidate | §19 | Add user-facing purity only if all observable effects are consistently tracked and proven. |
| **0.1.41** | Higher-order effect composition | research to candidate | §32, §9.3, §19 | Infer callback and generic effects across composition without a full effect-handler language. |
| **0.1.42** | Function preconditions | candidate | §24, §32 | Define verified input contracts with compile-time proof or unavoidable runtime enforcement. |
| **0.1.43** | Function postconditions and result facts | candidate | §24, §32 | Define result-dependent guarantees with safe proof and error/return/never interactions. |
| **0.1.44** | Proof obligations as tooling contracts | tooling; source guarantee only if approved | §21, §32, §33.2 | Explore Goose-style reject-if-residual-bound-check tooling without disabling runtime checks. |
| **0.1.45** | Callable captures and invocation cardinality | candidate | §9.1–9.4 | Express distinction between moveable, shareable, concurrently-callable and once-callable closures only when inference needs source contracts. |
| **0.1.46** | Isolation regions for concurrency | candidate analysis+source | §22 | Prove legal transfer/isolation using region reachability, liveness and external aliases. |
| **0.1.47** | Cancellation safety contracts | candidate | §23.1, §23.3 | Define cancel-safe operation boundaries without changing the existing TaskOutcome model. |
| **0.1.48** | Reservations and permit capabilities | candidate API/library | §23.2, §23.6 | Explore explicit capacity reservations on channels and similar bounded resources. |
| **0.1.49** | Task lifecycle and structured ownership | candidate | §23.3, §23.5 | Review whether explicit task-lifetime ownership contracts close gaps without spawning a new concurrency model. |
| **0.1.50** | Execution-domain and blocking guarantees | candidate | §23.4, §23.7 | Distinguish transferable from execution-domain-constrained and may-block callables. |
| **0.1.51** | Backpressure and channel protocol composition | candidate | §23.5–23.6 | Complete protocol semantics for WouldBlock, closed, send reservations and cancellation where truly missing. |
| **0.1.52** | Cooperative scheduling and select fairness | candidate | §23.8–23.9 | Define observable fairness guarantees, rather than promising scheduler implementation details. |
| **0.1.53** | Semantic compiler inspection tools | tooling milestone | §21, §31 | Build stable inspection of call graph, effects, ownership, type decisions and proof reasons for programmers. |
| **0.1.54** | Destination-oriented lowering conformance | compiler milestone | §13, §25, §33 | Verify result-location construction and numeric fusion without introducing extra mandatory source syntax. |
| **0.1.55** | Pre-0.2 integration and compatibility freeze | integration gate; no new language semantics | §1–34 | Close only **accepted** 0.2 feature scope, inventory unchanged feature effects and compatibility. |

> **Important:** Gates labelled *candidate* are not accepted language changes. `0.1.53`–`0.1.55` illustrate quality/tooling milestones rather than forced new-language releases; they can merge with earlier releases. `0.1.56`–`0.1.100` remain free for real dependencies, remediations and additional accepted changes. The 0.2.0 scope must not depend on a feature introduced for the first time in `0.2.0`.

---

## § 3. Regex specification and execution (MD-010)

**§ 3(1) Existing state.** The `regex` contract is reserved in `rules/types/contracts.md`; lexer/parser/AST/Sema/LSP hold its string pattern and applicability. It is currently unavailable (mandatory S1058) because the regex dialect and executable validator are not normatively defined. `missing-decisions.yaml` MD-010 remains open. Specification and implementation are different deliverables.

**§ 3(2) `0.1.1` specification deliverable.** Write the dedicated, language-wide regex rulebook; do **not** define a contract-only dialect. Its normative paragraphs must cover: source pattern escapes versus regex escapes; decoded UTF-8 input; Unicode scalar/case/class/property matching; anchors; whole-value type contract versus search semantics; grouping, named/numbered captures, greediness, unsupported features; engine complexity and worst-case resource policy; flags; pattern compilation and caching; invalid-pattern diagnostics; exact runtime API types, constructors, errors, ownership, allocation, lifetime and thread safety; contract validation on literals/defaults and runtime conversions; platform implementation and ABI boundaries.

**§ 3(3) `0.1.8` execution gate.** Implement the accepted design across compile-time validation, constant evaluation, Sema plans, Semantic IR, Sec MLIR, runtime engine, public API, LSP, negative diagnostics and conformance. Replace S1058 only when complete. If P22/P23 or storage prerequisites block the full feature, **move the release target**, not the standard of completion.

**§ 3(4) Integration.** MD-010 closes only after regex specification, engine, contracts and tests are synchronized. Reuse the existing error/governance IDs; do not create a second regex-feature ledger.

---

## § 4. Feature status and compiler/library/tooling boundaries

**§ 4(1) Status labels.** `research`, `owner_direction`, `design_pending`, `accepted`, `normative_complete`, `implementation_partial`, `implementation_complete`, `verified`, `deferred`, `rejected`, `already_covered` are **planning annotations**. Do not substitute them for governance's existing `planned`/`partial`/`implemented` status values.

**§ 4(2) Release rule.** Only `accepted → normative_complete → implementation_complete → verified` can be marked delivered. Targeting a release does not imply acceptance. A specification-only task (regex in 0.1.1) can be complete as a *book deliverable* while the independently scheduled runtime feature remains unavailable.

**§ 4(3) No manufactured features.** Native SIMD optimization, shaped numeric fusion, compiler query UX, bug fixes, stable source spans, cache behavior, and default-copy optimizations belong primarily to compiler/tooling. An improvement there does not automatically become a new language feature or cause a new source-language version.

**§ 4(4) Core and stdlib.** A first-class compiler type such as `rational` or `complex` has source identity owned by the language; `sec/core` can supply the trusted public operations. Nominal math-library types such as `Quaternion[T]` and `Polynomial[T]` remain library scope unless separate evidence supports promotion. An accepting release may ship library APIs without introducing new primitive syntax.

**§ 4(5) User experience.** At each gate, evaluate a beginner's likely expectation, an experienced programmer's predicted behavior, real source length, IDE discoverability, and diagnostic usefulness. The compiler must explain a correction without hiding important semantics.

**§ 4(6) Existing normative versus new delivered scope.** Version documents must label every item as (a) an already accepted language rule being finished/verified in that release, (b) a genuinely new candidate requiring owner approval, (c) a specification-only dependency deliberately inactive until a later gate, or (d) an independent compiler/library/tooling improvement. This prevents the inclusion of MD-117/MD-120 in 0.1.1 from rewriting 0.1 history.

**§ 4(7) Decision challenge.** No proposal becomes accepted solely because a compiler could implement it, another language has it, or its author favors it. Record the best case **against** each proposal, the simplest competing solution, multi-feature interactions, and evidence for an ergonomic benefit. Use §17.

---

## § 5. Channel naming and release stability

| Example | Channel | Requirement |
|---|---|---|
| `0.1.26-dev.1` | **develop** | Experiment or partial implementation; incomplete API/ABI allowed only if explicitly identified. |
| `0.1.26-test.1` | **test** | Reproducible integrated candidate for CI and dogfooding, with explicit missing scope. |
| `0.1.26-rc.1` | **release candidate** | Scope frozen; full proposed release criteria must pass before final tag. |
| `0.1.26` | **milestone release** | That version's accepted feature set is actually implemented and verified, despite language remaining experimental. |
| `0.2.0-rc.1` | **integration candidate** | No new features; whole accepted 0.2 inventory verified together. |
| `0.2.0` | **stable baseline (relative)** | The approved 0.2 language set is integrated and tested; **not** a production-readiness or cross-platform permanent ABI promise. |
| `0.6.0` | **potential production stability** | Merely a current aspiration; separate criteria and evidence required. |

**§ 5(1) SemVer-like suffixes are proposals.** Validate compatibility with current Sec manifest version parsing, project tooling and release automation before publishing. Version strings, schema versions (MLIR), compiler binary versions and language versions are different things.

**§ 5(2) Tag/source trace.** Every artifact needs source commit, normative-rulebook snapshot, governance fingerprint, compiler/MLIR schema, supported targets, language version, feature flags, known unsupported cases and conformance evidence.

**§ 5(3) Release branches.** `main` is integrated current work, not automatically stable; development/testing/release channels describe immutable builds and tags. Breaking changes require explicit migration plans while pre-1.0, not silent semantic drift.

---

## § 6. 0.2.0 acceptance gate

1. No required feature is new in 0.2.0; each accepted promise has a verified `0.1.x` introduction with `x ≤ 100`.
2. Every accepted language/API/ABI item has a canonical normative owner and all affected rulebooks synchronized; pending items are explicitly deferred from the 0.2 promise.
3. Source-valid examples compile and run across declared targets, and invalid examples fail with the designated diagnostics; emitted MLIR and native paths exist wherever claimed.
4. Ownership/lifetime, error, effects, unit, constraints, generic specialization, CTE, ABI/FFI, deterministic cleanup and concurrency interactions have conformance evidence.
5. LSP, formatter, compiler CLI, version metadata, manual and source API documentation agree with the canonical rulebooks.
6. Cold and incremental compilation behave equivalently; lowering/schema and target regressions pass; no unproven optimization is relied upon for safety.
7. Real application dogfooding, ergonomics and intuition review, and external/bootstrapping feedback have identified and resolved release blockers or formally scoped exclusions.
8. `0.2.0-rc.*` is a feature freeze and quality gate, not an opportunity to smuggle in unresolved features.

---

## § 7. Immediate documentation work order

1. Create the **`0.1.1` version document first** (§18; proposed path `planning/versions/sec-0.1.1.md`). Its scope is `Owner[T]`, the MD-010 regex specification, completion of MD-117 heterogeneous variadics, and completion of MD-120 generic errors. Record separate specification versus runtime/implementation gates.
2. For each scoped item, conduct the §17 critical review: real use cases, strongest counterargument, ergonomic/intuitive behavior, necessary API/ABI, costs, existing alternatives, and interactions. Identify only genuinely new owner decisions.
3. Reconcile existing MD-117 and MD-120 normative decisions, exact pending implementation work, and governance owners; do **not** reopen their accepted 0.1 semantics or mistakenly classify partial support as complete.
4. Write `rules/regex/regular_expressions.md` (proposed path) to establish one accepted regex language for both ordinary use and named-type contracts (MD-010); keep runtime activation separately tracked for 0.1.8 until verified.
5. Complete the exact `Owner[T]` public API, storage authority, layout, recursive destruction and ABI decisions using `Sec-Owner-0.1.1.md`; transfer accepted semantics into the canonical owning rulebooks without renumbering paragraphs.
6. Draft **independent new normative books** for lowercase `rational`, lowercase `complex`, `@operator`, `@mustUse` linear obligations and function contracts in dependency order, only after version-document review identifies the real language contract and owner approval.
7. Reuse existing canonical books and governance IDs wherever appropriate. Apply necessary existing-book revisions with stable paragraph identities; source examples, tests and implementation packages are part of the handoff.
8. Work in small independently testable implementation steps and validate the entire release scope before a final tag. A later version's design/implementation can begin before an earlier version's release, subject to genuine dependencies. Do not require alphabetical/numeric work order as a substitute for a dependency graph.

---

## § 8. Release-by-release specification and implementation checklists

**Every entry below names a proposed delivery gate.** The `Normative owner` line lists canonical existing books and clearly marked **proposed new books**. The API/ABI line describes *what must be decided and specified*, not an existing public declaration. The test line is an acceptance criterion; it does not claim those tests already exist or pass.

### § 8.1 — 0.1.1: Owning indirection, regex rulebook, heterogeneous type packs and generic errors

**Status:** owner-assigned release scope; implementation completeness not claimed. **Ideas:** §18, §10; MD-010, MD-117 and MD-120. **Depends on:** Sec 0.1 ownership/generics/Result/ABI, P15–P25 where relevant, current regex parser/CTE.

**Four separately tracked deliverables:**

1. **`Owner[T]` (0.1.1 target):** complete the owning-indirection source/API, storage producer, recursive layout, borrowing, destruction and ABI contract; implement and verify only the fully accepted public scope. The working type name is not yet a final declaration.
2. **Regex (0.1.1 specification gate):** write and approve a complete dedicated regex rulebook (MD-010), with API/ABI/runtime requirements and executable conformance expectations. Regex contract execution is separately targeted to 0.1.8; do not claim 0.1.1 regex runtime support or remove S1058 prematurely.
3. **Heterogeneous variadic type packs (MD-117; already Sec 0.1 normative):** complete compiler support for `[...Types]` and typed `...Types`, `...ref Types`, `...ref mut Types` across permitted calls, specialization, element iteration, spreading, ownership, cleanup, Semantic IR, MLIR and native execution. Review actual remaining governance gaps rather than implementing the already covered subset again.
4. **Generic error parameters/declarations (MD-120; already Sec 0.1 normative):** complete `[E: error]` constraints, `Result[T, E]` propagation, generic error enums/unions, generic interface specializations such as `interface FormatSink[E: error]`, exact borrowed interface dispatch, generic inference, and required core/stdlib integrations. Keep `error` distinct from a user interface or an erased type.

**Normative owners / revisions:** `rules/memory/owner.md` [NEW, proposed]; `rules/regex/regular_expressions.md` [NEW, proposed]; `rules/types/contracts.md` [REV]; `rules/declarations/functions.md` §38A, `generics.md`, `interfaces.md`, `spread.md`, `rules/errors/errorhandling.md`, `rules/declarations/enums.md` and `unions.md` [RECONCILE already accepted rules]; plus affected compiler/ABI/tooling rulebooks.

**Required API/ABI/compiler handoff:** four independent implementation matrices; existing MD-117/MD-120 identities and exact callable/error ABI preserved; no implicit boxing/heap allocation, type erasure, ownership repair or unverified target fallback. `Owner[T]` is not considered delivered with undecided constructor/producer/destructor behavior. Regex's normative book and runtime activation remain separate acceptance items.

**Acceptance tests:** complete static positive/negative packs and generic errors, polymorphic/generic interface conformance, copied/moved/borrowed pack positions, empty and mixed packs, exact error-channel identities and forbidden widening, specialization and lowering verification, native runs on the claimed target matrix, cleanup and ABI/FFI negatives where supported; `Owner[T]` recursive layout and resource-lifetime tests; regex spec's valid/invalid example catalogue and future runtime test obligations. Unsupported boundaries prevent completion of the affected release feature rather than being silently counted as tested.

**Mandatory version document:** see §19 for the `0.1.1` scoping and critical-review checklist. For MD-117 and MD-120, 0.1.1 is an **integration/delivery commitment**, not a retroactive move of their Sec 0.1 normative activation.

### § 8.2 — 0.1.2: Contextual Result error patterns

**Status:** preassigned. **Ideas:** §7. **Depends on:** 0.1 Result and match.

**Source-visible delivery:** `Err(.Variant...)` matches unambiguous closed error types, without widening expression syntax.

**Normative owner / revisions:** rules/errors/errorhandling.md [REV]; rules/control-flow/flowcontrol_match.md [REV].

**Required API, ABI and compiler specification:** Unambiguous pattern binder/error identity; match ownership and lowering; no new ABI unless pattern changes data shape (not expected).

**Acceptance tests:** Pattern conformance, ambiguous/error-negative cases, exhaustiveness, LSP.

### § 8.3 — 0.1.3: Declaration ceremony reduction I

**Status:** candidate. **Ideas:** §2.1, §31. **Depends on:** lexer and existing typed/inferred declarations.

**Source-visible delivery:** Omit `let` only in syntactically unambiguous binding contexts; keep distinction between new binding and assignment.

**Normative owner / revisions:** rules/types/types.md [REV]; rules/foundations/grammar.md [REV]; rules/foundations/names_scopes_visibility.md [REV].

**Required API, ABI and compiler specification:** Binding grammar, mutability, scopes, shadowing, equality with previous form; no ABI change.

**Acceptance tests:** Parser roundtrip, formatter, shadowing, redefinition, LSP refactor, representative programs.

### § 8.4 — 0.1.4: Contextual enum and union names

**Status:** candidate. **Ideas:** §4.1, §31. **Depends on:** existing contextual `.Member` rules.

**Source-visible delivery:** Extend only genuinely missing contextual `.Member` forms; preserve explicit typing on ambiguity.

**Normative owner / revisions:** rules/declarations/enums.md [REV]; rules/declarations/unions.md [REV]; rules/types/types.md [REV].

**Required API, ABI and compiler specification:** Expected-type propagation and static candidate selection; no ABI change.

**Acceptance tests:** Ambiguous scope, generic contexts, literals, diagnostics, completion.

### § 8.5 — 0.1.5: Context-directed inference consistency

**Status:** candidate. **Ideas:** §4.3, §20. **Depends on:** 0.1.4 and ordinary overload selection.

**Source-visible delivery:** Bidirectional inference where one expected type exists, without return-type-driven overload selection.

**Normative owner / revisions:** rules/types/types.md [REV]; rules/declarations/generics.md [REV]; rules/declarations/functions.md [REV].

**Required API, ABI and compiler specification:** Specify inference direction, precedence and ambiguity; internal Sema facts, unchanged call ABI.

**Acceptance tests:** Nested generic literals, overload sets, wrong expected types, no hidden conversions.

### § 8.6 — 0.1.6: Conditional value expression

**Status:** candidate. **Ideas:** §5.1. **Depends on:** expression and type inference.

**Source-visible delivery:** Add conditional value selection `?:` with precise grouping, branch typing and evaluation.

**Normative owner / revisions:** rules/foundations/operators.md [REV]; rules/foundations/grammar.md [REV]; rules/control-flow/flowcontrol_if.md [REV].

**Required API, ABI and compiler specification:** Associativity, precedence, branch-specific borrow/move/try/panic; SSA join semantics; no intrinsic ABI.

**Acceptance tests:** Exhaustive precedence, control-flow, ownership, effects, dead-branch, formatter tests.

### § 8.7 — 0.1.7: Short context-typed lambdas

**Status:** candidate. **Ideas:** §8.1. **Depends on:** 0.1.5.

**Source-visible delivery:** Allow concise lambdas when context supplies complete parameter types without guessing.

**Normative owner / revisions:** rules/declarations/lambda-functions.md [REV]; rules/declarations/functions.md [REV].

**Required API, ABI and compiler specification:** Capture and callable identity, inferred parameter types, escaping/borrowing; existing closure ABI.

**Acceptance tests:** Ambiguous callbacks, generic specialization, captures, nonescape, code completion.

### § 8.8 — 0.1.8: Regex execution and named contracts

**Status:** candidate implementation gate. **Ideas:** MD-010; §24. **Depends on:** 0.1.1 regex spec + string materialization + P22/P23 lowerings.

**Source-visible delivery:** Deliver regex engine and public runtime API from 0.1.1 specification, including named-string contract validation.

**Normative owner / revisions:** rules/regex/regular_expressions.md [REV/complete]; rules/types/contracts.md [REV]; rules/errors/runtime_checks.md [REV].

**Required API, ABI and compiler specification:** Compiled pattern ownership, limits, streaming/search, captures, errors, cancellation/resource policy; check/CTE/lowering and ABI; replace S1058 only once proven.

**Acceptance tests:** Unicode corpus, invalid input/patterns, complexity/resource bounds, deterministic tests, enforced runtime contracts, FFI/target tests.

### § 8.9 — 0.1.9: Ranges in collection literals

**Status:** candidate. **Ideas:** §3.1. **Depends on:** existing range and list/array lowerings.

**Source-visible delivery:** Literal range expansion with finite extent and familiar evaluation order; preserve collection storage contracts.

**Normative owner / revisions:** rules/collections/collections.md [REV]; rules/foundations/grammar.md [REV].

**Required API, ABI and compiler specification:** Constant/runtime expansion, overflow, capacity, allocation, spread interaction; no new ABI.

**Acceptance tests:** Capacity failures, duplicate evaluation, zero length, primitive/rune ranges, formatter.

### § 8.10 — 0.1.10: Membership and value-set expressions

**Status:** candidate. **Ideas:** §6. **Depends on:** 0.1.9 and existing membership.

**Source-visible delivery:** Extend `in`/`not in` only where semantics of a set/pattern are statically defined.

**Normative owner / revisions:** rules/foundations/operators.md [REV]; rules/control-flow/flowcontrol_match.md [REV].

**Required API, ABI and compiler specification:** Static membership dispatch, unions/enum handling, potential fallibility; no auto-generated dynamic set.

**Acceptance tests:** Operator precedence, evaluation count, pattern/equality, no implicit allocation.

### § 8.11 — 0.1.11: General nested patterns

**Status:** candidate. **Ideas:** §7.1. **Depends on:** 0.1.2, 0.1.10.

**Source-visible delivery:** Permit direct nested destructuring in `match`, beyond narrow 0.1 Result exceptions.

**Normative owner / revisions:** rules/control-flow/flowcontrol_match.md [REV]; rules/declarations/unions.md [REV].

**Required API, ABI and compiler specification:** Binding kinds, irrefutability, exhaustiveness, partial access, alias/liveness; no ABI change.

**Acceptance tests:** Every nested position: valid/invalid, hidden fields, variant shadowing, effects.

### § 8.12 — 0.1.12: Ownership-aware pattern ergonomics

**Status:** candidate. **Ideas:** §7.2, §11. **Depends on:** 0.1.11.

**Source-visible delivery:** Use minimal authority needed by binding uses, with explicit exceptions when inference is unsafe.

**Normative owner / revisions:** rules/memory/borrowing.md [REV]; rules/control-flow/flowcontrol_match.md [REV].

**Required API, ABI and compiler specification:** Resolve read/borrow/move semantics before lowering, keep unique owner and exact once; no hidden `ref mut` rights.

**Acceptance tests:** Borrow conflicts, partial moves, invalidation, destruction, match expression result.

### § 8.13 — 0.1.13: Value blocks and multi-condition value selection

**Status:** candidate. **Ideas:** §8.2, §5.2. **Depends on:** 0.1.6, 0.1.12.

**Source-visible delivery:** Local value-producing blocks and a coherent multi-condition value-selection form with defined scopes and early exits.

**Normative owner / revisions:** rules/control-flow/flowcontrol_if.md [REV]; rules/foundations/grammar.md [REV].

**Required API, ABI and compiler specification:** Return/exit/try/defer, expression result type and storage lifetime; no new ABI.

**Acceptance tests:** Branch joins, temporary lifetimes, nested blocks and formatting.

### § 8.14 — 0.1.14: Direct declaration forms

**Status:** candidate migration. **Ideas:** §2.2, §2.3. **Depends on:** 0.1.3; existing grammar currently labels this Sec 0.2.

**Source-visible delivery:** Introduce direct `struct Name`, `union Name` etc only after consistent grammar and migration plan.

**Normative owner / revisions:** rules/foundations/grammar.md [REV]; rules/declarations/struct.md [REV]; rules/declarations/unions.md [REV].

**Required API, ABI and compiler specification:** Old/new syntax equivalence, source compatibility policy, formatter, error recovery; same nominal type/ABI.

**Acceptance tests:** Migration across generic, nested, public, FFI and empty declarations.

### § 8.15 — 0.1.15: Flow-sensitive facts exposed coherently

**Status:** candidate source semantics. **Ideas:** §4.2, §21. **Depends on:** Sema proof and provenance infrastructure.

**Source-visible delivery:** Make proven range, availability, variant and capability facts consistently usable in source.

**Normative owner / revisions:** rules/analysis/flow_analysis.md [NEW if needed]; rules/types/types.md [REV].

**Required API, ABI and compiler specification:** Proof lifetime, invalidation across calls, backend-proof metadata; no source runtime witness.

**Acceptance tests:** Stale facts, mutations/threads, diagnostics, no unsafe omitted checks.

### § 8.16 — 0.1.16: Reference shorthand for reference holders

**Status:** candidate. **Ideas:** §10.1, §14. **Depends on:** 0.1.15.

**Source-visible delivery:** Avoid redundant `ref` only where a value already carries correct borrowed authority.

**Normative owner / revisions:** rules/memory/borrowing.md [REV]; rules/memory/references.md [REV].

**Required API, ABI and compiler specification:** Reborrow rules, no authority elevation, no reference-to-reference type; ABI unchanged.

**Acceptance tests:** Explicit/missing ref contrasts, overload sensitivity, mutable references.

### § 8.17 — 0.1.17: Multiple-origin provenance

**Status:** candidate. **Ideas:** §10.2, §11. **Depends on:** 0.1.15.

**Source-visible delivery:** Retain reference origins across branches without obligating programmers to name lifetimes.

**Normative owner / revisions:** rules/memory/reference_model.md [REV]; rules/analysis/escape_analysis.md [REV].

**Required API, ABI and compiler specification:** Finite alternative origins, join and escape proofs; ABI references unchanged.

**Acceptance tests:** Divergent branch roots, reference lifetime, compile-time invalidation.

### § 8.18 — 0.1.18: Nonescapable values and views

**Status:** candidate. **Ideas:** §10.3, §9.3. **Depends on:** 0.1.16–0.1.17.

**Source-visible delivery:** Define nonescaping capabilities as a concrete constraint when compiler inference cannot suffice.

**Normative owner / revisions:** rules/memory/references.md [REV]; rules/analysis/escape_analysis.md [REV].

**Required API, ABI and compiler specification:** Allowed fields, returns, captures, callbacks, generics, ABI/FFI escape prohibition.

**Acceptance tests:** Borrowed iterators, temporary views, rejected return/closure/storage escapes.

### § 8.19 — 0.1.19: Place and projection ergonomics

**Status:** candidate. **Ideas:** §11, §12. **Depends on:** 0.1.12–0.1.18.

**Source-visible delivery:** Unify expression Place identity for field/index/property/method call and assignment.

**Normative owner / revisions:** rules/memory/storage.md [REV]; rules/declarations/properties.md [REV]; rules/compiler/semantic_ir.md [REV].

**Required API, ABI and compiler specification:** Evaluation once; projected references, writeback, accessible Place and access modes; no ABI changes unless layout plan.

**Acceptance tests:** Chained projections, mutation, getters, compound assignment, alias conflicts.

### § 8.20 — 0.1.20: Construction: required fields

**Status:** candidate. **Ideas:** §15.1. **Depends on:** 0.1.14, existing constructors.

**Source-visible delivery:** Require explicit initialization of fields without a defined default, avoiding surprising zero/default insertion.

**Normative owner / revisions:** rules/declarations/struct.md [REV]; rules/types/default_values.md [REV].

**Required API, ABI and compiler specification:** Field presence, generic substitution, defaults, escape/partial-construction cleanup; ABI field layout unchanged.

**Acceptance tests:** Invalid missing fields, splats/spread, generic, initialized fields, old syntax migration.

### § 8.21 — 0.1.21: Construction: visibility and factory boundaries

**Status:** candidate. **Ideas:** §15.2. **Depends on:** 0.1.20.

**Source-visible delivery:** Permit control over direct construction without forcing boilerplate on ordinary types.

**Normative owner / revisions:** rules/declarations/struct.md [REV]; rules/declarations/impl.md [REV].

**Required API, ABI and compiler specification:** Public constructors, field visibility, initializer/factory fallibility; ABI consistent.

**Acceptance tests:** Positive/negative accessibility, user-owned types, FFI construction.

### § 8.22 — 0.1.22: Function argument labels

**Status:** candidate. **Ideas:** §16. **Depends on:** 0.1.5, API conventions.

**Source-visible delivery:** Express informative parameter labels consistently, without inventing implicit dependency injection.

**Normative owner / revisions:** rules/declarations/functions.md [REV]; rules/foundations/grammar.md [REV].

**Required API, ABI and compiler specification:** Overload identity, labels versus names, call syntax, defaults/spreads, ABI unchanged.

**Acceptance tests:** Overload ambiguity, named/positional mixing, formatter, LSP rename.

### § 8.23 — 0.1.23: Public `never` type semantics

**Status:** candidate (IR already known). **Ideas:** §28.3. **Depends on:** existing NeverType/TypeNever/!sec.never.

**Source-visible delivery:** Expose the already-known uninhabited type consistently in valid source type positions.

**Normative owner / revisions:** rules/types/types.md [REV]; rules/control-flow/flowcontrol_match.md [REV].

**Required API, ABI and compiler specification:** Unreachable result, subtyping/join, ABI no normal return, pointer/aggregate restrictions.

**Acceptance tests:** Panic, recursive functions, generic return, dead branches, invalid values.

### § 8.24 — 0.1.24: Float16 numeric family

**Status:** candidate. **Ideas:** §28.1. **Depends on:** numeric plan and float lowering.

**Source-visible delivery:** Explore `float16` as first-class numeric scalar only after storage/arithmetic and target profile are defined.

**Normative owner / revisions:** rules/types/types.md [REV]; rules/library/core-library.md [REV].

**Required API, ABI and compiler specification:** Representation, rounding, promotion, literal classification, ABI/FFI and fallback.

**Acceptance tests:** CTE/native cross-check, NaN/Inf/subnormal, conversion, target-matrix.

### § 8.25 — 0.1.25: Bfloat16 numeric family

**Status:** candidate. **Ideas:** §28.1. **Depends on:** 0.1.24, numeric conversion.

**Source-visible delivery:** Explore `bfloat16` independently of `float16`; do not silently equate storage and compute formats.

**Normative owner / revisions:** rules/types/types.md [REV]; rules/library/core-library.md [REV].

**Required API, ABI and compiler specification:** Rounding, width, hardware vs software, source/core APIs and ABI/FFI.

**Acceptance tests:** Round-trips, range/special values, cross-target conformance.

### § 8.26 — 0.1.26: First-class `rational`: type and arithmetic

**Status:** OWNER-DIRECTED first-class; semantics pending. **Ideas:** §28.1, §28.5. **Depends on:** numeric/CTE/conversion base; must settle bounded vs unbounded representation.

**Source-visible delivery:** Specify and implement lowercase compiler-known `rational`; no `Rational[T]` stdlib substitute.

**Normative owner / revisions:** rules/types/rational.md [NEW]; rules/types/types.md [REV]; rules/foundations/operators.md [REV].

**Required API, ABI and compiler specification:** Exact quotient, numerator/denominator domain, canonicalization, invalid denominator, overflow or allocation model, literals, comparison, formatting, no hidden precision loss, ABI/FFI constraints.

**Acceptance tests:** Reference oracle and algebraic properties, corner cases, CTE vs runtime, failure & memory effects, every target.

### § 8.27 — 0.1.27: `rational` integration, exact unit scales

**Status:** OWNER-DIRECTED first-class; integration decision pending. **Ideas:** §28.1, §28.5. **Depends on:** 0.1.26; units MD decisions.

**Source-visible delivery:** Connect rational to existing units only where a *separate* unit metadata decision permits it.

**Normative owner / revisions:** rules/types/rational.md [REV]; rules/types/units.md [REV]; rules/library/core-library.md [REV].

**Required API, ABI and compiler specification:** Keep decimal semantics; exact scale metadata vs user-runtime rational are distinct; conversion/rounding rules and ABI call plans.

**Acceptance tests:** Scale 1/60, negative scales, dimension conversions, precision loss, float/decimal interop.

### § 8.28 — 0.1.28: First-class `complex`: core type and operations

**Status:** OWNER-DIRECTED first-class; semantics pending. **Ideas:** §28.1, §29.2. **Depends on:** 0.1.24–0.1.27 as appropriate; requires owner choice of precision model.

**Source-visible delivery:** Specify and implement lowercase compiler-known `complex`; avoid duplicate `Complex[T]` nominal API.

**Normative owner / revisions:** rules/types/complex.md [NEW]; rules/types/types.md [REV]; rules/foundations/operators.md [REV].

**Required API, ABI and compiler specification:** Component type/default precision or parameterization, layout/alignment, construction, multiplication/division, equality, literals, effects, ABI/FFI, units and formatting.

**Acceptance tests:** Identities, signed zero/NaN/Inf, precision, type/CTE/MLIR/ABI matrix, cross-target.

### § 8.29 — 0.1.29: `complex` integration and numeric algorithms

**Status:** OWNER-DIRECTED first-class; integration decision pending. **Ideas:** §29.2, §29.5. **Depends on:** 0.1.28.

**Source-visible delivery:** Connect builtin `complex` to stable public math and generic algorithm contracts.

**Normative owner / revisions:** rules/types/complex.md [REV]; rules/library/stdlib.md [REV]; sec/core/complex.sec [PROPOSED].

**Required API, ABI and compiler specification:** Math functions, mixed scalar/complex conversion, conjugate/magnitude/phase, error model and native ABI stable within profile.

**Acceptance tests:** FFT and signal examples, CTE/runtime equivalence, non-allocation claims when made.

### § 8.30 — 0.1.30: Operator annotations: declaration and lookup

**Status:** candidate. **Ideas:** §30.1–30.4. **Depends on:** 0.1.22 and generic/typed-call infra.

**Source-visible delivery:** Use explicit annotation of ordinary static overloads, proposal spelling `@operator("+")`.

**Normative owner / revisions:** rules/foundations/operator_overloading.md [NEW]; rules/foundations/attributes.md [REV]; rules/declarations/impl.md [REV].

**Required API, ABI and compiler specification:** Allowed operators/arity, module coherence, static all-argument overload match, ownership/effects; built-in operator extension prohibited.

**Acceptance tests:** Multiple candidates, imported collisions, precedence, static-only, no return-type dispatch, no implicit move.

### § 8.31 — 0.1.31: Operator annotations: advanced interactions

**Status:** candidate. **Ideas:** §30.2–30.6. **Depends on:** 0.1.30.

**Source-visible delivery:** Resolve mixed user types, generic operator requirements and compound assignment without violating Place evaluation.

**Normative owner / revisions:** rules/foundations/operator_overloading.md [REV]; rules/declarations/generics.md [REV]; rules/memory/ownership.md [REV].

**Required API, ABI and compiler specification:** Mixed operand pairs, unary/binary, `x` decision, inplace hooks vs `+=` rewrite, ABI reuse of ordinary calls.

**Acceptance tests:** No hidden allocation/conversion; borrowing, errors, single evaluation and algebraic-law nonassumption.

### § 8.32 — 0.1.32: Generic numerical capability contracts

**Status:** candidate. **Ideas:** §20, §29, §30. **Depends on:** 0.1.26–0.1.31.

**Source-visible delivery:** Make generic numerical algorithms usable for builtin and user mathematical carriers without a second dispatch model.

**Normative owner / revisions:** rules/declarations/generics.md [REV]; rules/declarations/interfaces.md [REV]; rules/library/stdlib.md [REV].

**Required API, ABI and compiler specification:** Static requirements for arithmetic, type/unit preservation and format; return type not overload selector; no implicit interface boxing.

**Acceptance tests:** Generic algorithms with rational/complex and nominal carriers; missing-operation diagnostics.

### § 8.33 — 0.1.33: Mathematical library breadth

**Status:** library delivery; language change only if needed. **Ideas:** §29.3–29.5. **Depends on:** 0.1.29–0.1.32; no automatic new primitive.

**Source-visible delivery:** Define `Quaternion[T]`, `Dual[T]`, `Polynomial[T]` as nominal library candidates; separate optional `UnitQuaternion[T]` and `Rotation3[T]`.

**Normative owner / revisions:** sec/stdlib/math/ [API SPEC NEW]; rules/library/stdlib.md [REV].

**Required API, ABI and compiler specification:** Exact public declarations, invariants, algebra (quaternion noncommutativity), autodiff, polynomial storage, errors, precision, operator use; C ABI only via explicit bridge.

**Acceptance tests:** Numerical reference cases, generic substitution, alias/lifetime, fused execution correctness.

### § 8.34 — 0.1.34: Generic capability checking

**Status:** candidate. **Ideas:** §20, §21. **Depends on:** 0.1.32.

**Source-visible delivery:** Derive actionable constraints from generic body usage without requiring redundant signatures.

**Normative owner / revisions:** rules/declarations/generics.md [REV]; rules/compiler/compiler_analysis.md [REV].

**Required API, ABI and compiler specification:** Conformance and substitution, separate compilation, stable diagnostic fact IDs; ABI per specialization.

**Acceptance tests:** Generic invalid source diagnostics, specialization cache identity, LSP query.

### § 8.35 — 0.1.35: Equivalent ownership implementations

**Status:** candidate. **Ideas:** §13, §25. **Depends on:** 0.1.19 and compiler destination plans.

**Source-visible delivery:** Allow semantic-equivalent storage/call implementations where optimizer proves identical observable behavior.

**Normative owner / revisions:** rules/memory/ownership.md [REV]; rules/compiler/semantic_ir.md [REV].

**Required API, ABI and compiler specification:** No differing ownership, errors, allocation observable, synchronization; ABI plan unchanged unless explicit versioned adapter.

**Acceptance tests:** Source-level equivalence and differential cold/native tests, failure paths.

### § 8.36 — 0.1.36: Type-level `@mustUse` linear declarations

**Status:** OWNER SUGGESTION; semantics pending. **Ideas:** §18.1–18.2, §14. **Depends on:** Owner[T] base, destruction, result-policy separation.

**Source-visible delivery:** Investigate `@mustUse` before type definitions to designate a mandatory exact-once lifetime obligation.

**Normative owner / revisions:** rules/memory/linear_obligations.md [NEW]; rules/foundations/attributes.md [REV]; rules/memory/copy_move.md [REV].

**Required API, ABI and compiler specification:** Distinguish ignored expression Result vs linear object; terminal operation, destructor/free, move, panic, error, return, generic/enum wrappers; ABI no hidden runtime counters.

**Acceptance tests:** One discharge/zero/two discharges, divergent paths, cancellation, generic type use, clear diagnostics.

### § 8.37 — 0.1.37: Linear terminal protocols and ownership

**Status:** candidate. **Ideas:** §18, §23.2, §23.3. **Depends on:** 0.1.36.

**Source-visible delivery:** Apply @mustUse-like obligations to reservations, transactions, resource acknowledgements and terminal states where accepted.

**Normative owner / revisions:** rules/memory/linear_obligations.md [REV]; rules/concurrency/channels.md [REV].

**Required API, ABI and compiler specification:** Consumed-by-commit/rollback/ack/discard? exact once; fallible terminals; spawned and selected paths; private state/ABI.

**Acceptance tests:** Transaction/state-machine negative cases, cleanup and cancellation safety.

### § 8.38 — 0.1.38: Linear generics and aggregates

**Status:** candidate. **Ideas:** §9.4, §18, §20. **Depends on:** 0.1.37.

**Source-visible delivery:** Make exact-once obligations compose through Option, Result, arrays, containers, generics and recursion.

**Normative owner / revisions:** rules/memory/linear_obligations.md [REV]; rules/declarations/generics.md [REV]; rules/collections/collections.md [REV].

**Required API, ABI and compiler specification:** Deep vs shallow obligations, partial moves, optional absence, wrapper destruction, sum variants, ABI/layout unchanged unless required.

**Acceptance tests:** Nested containers, recursion, conditional returns, generic specialization; no leaked obligation.

### § 8.39 — 0.1.39: Copy/move ergonomic simplification

**Status:** candidate. **Ideas:** §14, §11. **Depends on:** 0.1.35–0.1.38.

**Source-visible delivery:** Remove only redundant transfer notation where ownership transfer is statically unavoidable; protect explicit ownership intent.

**Normative owner / revisions:** rules/memory/copy_move.md [REV]; rules/memory/ownership.md [REV].

**Required API, ABI and compiler specification:** Call-site `<-` retained or deliberately revised only after approval, borrow vs move and destruction; ABI preserved.

**Acceptance tests:** Source transformations, no hidden copy, moved-from diagnostics, generic calls.

### § 8.40 — 0.1.40: Purity and verified effect promises

**Status:** candidate. **Ideas:** §19. **Depends on:** existing effect summaries.

**Source-visible delivery:** Add user-facing purity only if all observable effects are consistently tracked and proven.

**Normative owner / revisions:** rules/analysis/effect_analysis.md [REV]; rules/foundations/attributes.md [REV].

**Required API, ABI and compiler specification:** Purity boundaries, IO/global mutation/FFI/volatile/alloc/sync, error and panic effects; ABI/cache plan sensitivity.

**Acceptance tests:** Indirect calls, getters, recursive SCC, hidden allocations, target-specific effects.

### § 8.41 — 0.1.41: Higher-order effect composition

**Status:** research to candidate. **Ideas:** §32, §9.3, §19. **Depends on:** 0.1.40.

**Source-visible delivery:** Infer callback and generic effects across composition without a full effect-handler language.

**Normative owner / revisions:** rules/analysis/effect_analysis.md [REV]; rules/declarations/lambda-functions.md [REV].

**Required API, ABI and compiler specification:** Effect polymorphism, specialization, separate compilation, rule for error/cancellation propagation; transport remains callable ABI.

**Acceptance tests:** Higher-order @noAlloc/@noBlock, escape, library stream/map callbacks.

### § 8.42 — 0.1.42: Function preconditions

**Status:** candidate. **Ideas:** §24, §32. **Depends on:** 0.1.40–0.1.41 and contracts.

**Source-visible delivery:** Define verified input contracts with compile-time proof or unavoidable runtime enforcement.

**Normative owner / revisions:** rules/declarations/function_contracts.md [NEW]; rules/errors/runtime_checks.md [REV].

**Required API, ABI and compiler specification:** Input/ref mutation, named types, error/panic policy, side-effect restrictions, public source API and FFI contract boundary.

**Acceptance tests:** Static invalid, runtime valid/invalid, no build-flag bypass, emitted checks.

### § 8.43 — 0.1.43: Function postconditions and result facts

**Status:** candidate. **Ideas:** §24, §32. **Depends on:** 0.1.42.

**Source-visible delivery:** Define result-dependent guarantees with safe proof and error/return/never interactions.

**Normative owner / revisions:** rules/declarations/function_contracts.md [REV]; rules/compiler/semantic_ir.md [REV].

**Required API, ABI and compiler specification:** Old state and named result access, ownership escape, early return, partial initialized values, ABI return planning.

**Acceptance tests:** All normal exits, fallible results, panic paths, provenance, generic postconditions.

### § 8.44 — 0.1.44: Proof obligations as tooling contracts

**Status:** tooling; source guarantee only if approved. **Ideas:** §21, §32, §33.2. **Depends on:** 0.1.42–0.1.43, MLIR.

**Source-visible delivery:** Explore Goose-style reject-if-residual-bound-check tooling without disabling runtime checks.

**Normative owner / revisions:** rules/tooling/compiler_queries.md [NEW if needed]; rules/analysis/pitfall_analysis.md [REV].

**Required API, ABI and compiler specification:** Canonical semantic query + proof evidence; internal optimizer output not ABI; target/optimization plan recorded.

**Acceptance tests:** Proof/not-provable differences, target-dependent results, no unsound runtime-check removal.

### § 8.45 — 0.1.45: Callable captures and invocation cardinality

**Status:** candidate. **Ideas:** §9.1–9.4. **Depends on:** 0.1.18, 0.1.38.

**Source-visible delivery:** Express distinction between moveable, shareable, concurrently-callable and once-callable closures only when inference needs source contracts.

**Normative owner / revisions:** rules/declarations/lambda-functions.md [REV]; rules/analysis/closure_analysis.md [REV].

**Required API, ABI and compiler specification:** Capture state, noescape, transfer, invocation count, closure representation/call ABI.

**Acceptance tests:** One-shot callbacks, reentrancy, mutable captures, borrowing, spawn.

### § 8.46 — 0.1.46: Isolation regions for concurrency

**Status:** candidate analysis+source. **Ideas:** §22. **Depends on:** 0.1.17, 0.1.45.

**Source-visible delivery:** Prove legal transfer/isolation using region reachability, liveness and external aliases.

**Normative owner / revisions:** rules/analysis/region_analysis.md [NEW if needed]; rules/memory/transferability.md [REV].

**Required API, ABI and compiler specification:** Region identity, ownership crossing task/thread/IPC, ABI transfer of capability vs raw data.

**Acceptance tests:** No false data-race approval, false-negative tradeoffs, parallel execution matrix.

### § 8.47 — 0.1.47: Cancellation safety contracts

**Status:** candidate. **Ideas:** §23.1, §23.3. **Depends on:** 0.1.37–0.1.46.

**Source-visible delivery:** Define cancel-safe operation boundaries without changing the existing TaskOutcome model.

**Normative owner / revisions:** rules/concurrency/cancellation.md [REV]; rules/concurrency/tasks.md [REV].

**Required API, ABI and compiler specification:** Cancelled partial progress, recoverable owned values, resource reclamation, state and effects; ABI task handles stable.

**Acceptance tests:** Cancel in-flight send/receive/IO/locks, no leaked `@mustUse` obligations.

### § 8.48 — 0.1.48: Reservations and permit capabilities

**Status:** candidate API/library. **Ideas:** §23.2, §23.6. **Depends on:** 0.1.47.

**Source-visible delivery:** Explore explicit capacity reservations on channels and similar bounded resources.

**Normative owner / revisions:** rules/concurrency/channels.md [REV]; sec/core/channel.sec [API REV if approved].

**Required API, ABI and compiler specification:** Unique permits and send-result ownership, closed/cancelled outcomes, no hidden allocation; ABI/endpoint lifetime and layout.

**Acceptance tests:** Permit transfer, cancellation, closed channels, capacity exhaustion, linear discharge if adopted.

### § 8.49 — 0.1.49: Task lifecycle and structured ownership

**Status:** candidate. **Ideas:** §23.3, §23.5. **Depends on:** 0.1.47–0.1.48.

**Source-visible delivery:** Review whether explicit task-lifetime ownership contracts close gaps without spawning a new concurrency model.

**Normative owner / revisions:** rules/concurrency/tasks.md [REV]; rules/concurrency/spawn.md [REV].

**Required API, ABI and compiler specification:** Detach/join/cancel/destruct, error and TaskOutcome, handle ABI and scheduler interaction.

**Acceptance tests:** Nested task exit, cancel/error races, resource return and timing.

### § 8.50 — 0.1.50: Execution-domain and blocking guarantees

**Status:** candidate. **Ideas:** §23.4, §23.7. **Depends on:** 0.1.40–0.1.49.

**Source-visible delivery:** Distinguish transferable from execution-domain-constrained and may-block callables.

**Normative owner / revisions:** rules/concurrency/scheduling.md [REV]; rules/analysis/effect_analysis.md [REV].

**Required API, ABI and compiler specification:** @noBlock verification, scheduling constraints, ABI callback transport and target capabilities.

**Acceptance tests:** Wrong-thread, target unsupported, blocking inside task, generic callbacks.

### § 8.51 — 0.1.51: Backpressure and channel protocol composition

**Status:** candidate. **Ideas:** §23.5–23.6. **Depends on:** 0.1.48–0.1.50.

**Source-visible delivery:** Complete protocol semantics for WouldBlock, closed, send reservations and cancellation where truly missing.

**Normative owner / revisions:** rules/concurrency/channels.md [REV]; rules/concurrency/select.md [REV].

**Required API, ABI and compiler specification:** Exact result variants, ownership on failure, enqueue ordering, sender/receiver ABI.

**Acceptance tests:** Race/saturation, no lost moved value, cancelled producer/consumer.

### § 8.52 — 0.1.52: Cooperative scheduling and select fairness

**Status:** candidate. **Ideas:** §23.8–23.9. **Depends on:** 0.1.49–0.1.51.

**Source-visible delivery:** Define observable fairness guarantees, rather than promising scheduler implementation details.

**Normative owner / revisions:** rules/concurrency/scheduling.md [REV]; rules/concurrency/select.md [REV].

**Required API, ABI and compiler specification:** Progress/starvation policy, cancellation tie-breaks, no hidden fairness semantics; ABI unchanged.

**Acceptance tests:** Stress/deterministic interleavings, cross-target consistency and documented limitations.

### § 8.53 — 0.1.53: Semantic compiler inspection tools

**Status:** tooling milestone. **Ideas:** §21, §31. **Depends on:** 0.1.40–0.1.52.

**Source-visible delivery:** Build stable inspection of call graph, effects, ownership, type decisions and proof reasons for programmers.

**Normative owner / revisions:** rules/tooling/cli.md [REV]; rules/tooling/lsp.md [REV].

**Required API, ABI and compiler specification:** Read-only query IDs and JSON/LSP response versioning, no language ABI impact.

**Acceptance tests:** CLI/LSP parity, deterministic IDs, separate-compilation mismatch.

### § 8.54 — 0.1.54: Destination-oriented lowering conformance

**Status:** compiler milestone. **Ideas:** §13, §25, §33. **Depends on:** 0.1.35 and numeric/shaped support.

**Source-visible delivery:** Verify result-location construction and numeric fusion without introducing extra mandatory source syntax.

**Normative owner / revisions:** rules/compiler/semantic_ir.md [REV]; rules/mlir/sec_mlir.md [REV]; rules/platform/abi.md [REV if needed].

**Required API, ABI and compiler specification:** Observable copy/move/destruct, alias, allocation and ABI hidden-result consistency; no guaranteed global zero-copy claim.

**Acceptance tests:** Execution + differential IR/backend + target-matrix; no optimization may change semantics.

### § 8.55 — 0.1.55: Pre-0.2 integration and compatibility freeze

**Status:** integration gate; no new language semantics. **Ideas:** §1–34. **Depends on:** all accepted gates 0.1.1–0.1.54.

**Source-visible delivery:** Close only **accepted** 0.2 feature scope, inventory unchanged feature effects and compatibility.

**Normative owner / revisions:** All accepted canonical rulebooks [REV where required]; rules/compiler/compiler_testing.md [REV].

**Required API, ABI and compiler specification:** Complete public API/ABI matrices and native/foreign boundaries, MLIR schema compatibility, errors/diagnostics, migration notes.

**Acceptance tests:** Cold/incremental conformance, target matrix, realistic apps, full stdlib interaction, no Sema-only false claims.

---

## § 9. First-class numeric type specification packets — `rational` and `complex`

### § 9.1 Status and public identity

**Decided direction from owner:** both are first-class Sec language types whose public names start with lowercase letters: **`rational`** and **`complex`**. This replaces the old idea-book uncertainty about whether they should instead be the ordinary stdlib nominal `Complex[T]` or another uppercase library type. `Complex[T]` remains a *historical proposal* and must not become a duplicate parallel public primitive by accident.

**Undecided details:** concrete runtime representation, whether `complex` is one fixed-component scalar or a parameterized compiler-known family, whether `rational` uses bounded/exact storage or arbitrary precision, allocation behavior, numerical literals, mathematical promotions, FFI compatibility and conversion forms. First-class identity does not answer these questions. Preserve them as real decision points.

### § 9.2 New `rules/types/rational.md` — required normative sections

1. Full file header (status, revision, creation/update dates, Sec version, canonical path, owner, replaces, cross-references), stable § identifiers and no placeholders in an *accepted* book.
2. Canonical type identity, admissible contexts (bindings, fields, generics, arrays, union variants, constants, literals if adopted), layout-dependent restrictions and initialization/default rules.
3. Mathematical domain: finite exact fractions; zero-denominator policy; sign handling, gcd/canonical normalization, equality/hash/order laws and negative zero policy.
4. Precision/resource model decision: bounded numerator/denominator with explicit overflow; arbitrary precision with visible allocation/failure; or a separately approved hybrid. **Do not imply arbitrary precision and fixed-size/no-allocation arithmetic simultaneously.**
5. Exact grammar and contextual typing/literal rules, including the distinction between ordinary integer division and rational construction; specify whether a rational literal exists before using examples that assume one.
6. Exact public core declarations: constructors (`init` or actual approved fallible constructor form), all arithmetic/compare operations, conversion and parsing APIs, `ToString`/format API, errors, field/property visibility, parameter borrow/consume contracts, allocation and panic effects. Do not write `static fn New()` as a pseudo-constructor.
7. Algebraic/overflow semantics for add/sub/mul/div/remainder if present, comparisons, normalization and internal temporary precision. Division by zero is an explicit contract, not inferred from mathematical prose.
8. Conversion table: signed/unsigned integer, exact `decimal`/`decimal128`, binary floats, `float16`/`bfloat16` if available, unit-bearing values, named types and string; state exact/rounded/fallible/lossy paths and their error channels.
9. Compile-time evaluation: supported operations, exactness bounds, constant folding, diagnostic behavior, limits/halting and resource budget.
10. Units: keep existing `decimal` semantics; separately decide whether unit `Scale` metadata has a rational internal carrier. Unit metadata's static fraction representation is not automatically a runtime `rational` value.
11. **ABI:** target-dependent storage descriptor or fixed scalar layout, alignment/size, pass/return classification, hidden result location, lifetimes, memory ownership, calling convention identity, C ABI/FFI legality, symbol/type fingerprints, separate compilation and schema migration. Where representation is not C-compatible, explicitly require a typed conversion/bridge rather than a cast.
12. Semantic IR/MLIR: canonical rational operation and failure plan identities, exactness/proof facts, target-specific lowering, overflow/allocation effects, debug info, optimizations that preserve visible arithmetic.
13. API documentation and LSP requirements: method/property hover, unit/precision information, text materialization and diagnostic recommendations.
14. Test matrix: algebraic properties with high-precision oracle; normalization; all failure paths; literal/CTE/runtime equivalence; target/ABI/FFI; generics and units; allocation and destructor evidence. No sign-off based only on CTE.

### § 9.3 New `rules/types/complex.md` — required normative sections

1. Same complete header, source-status discipline and stable paragraph numbers as `rational`.
2. Canonical lowercase `complex` identity and exact decision about component model: fixed representation, width-specific companion types or compiler-known parameterization. Do not presuppose source spelling like `complex[float32]` until approved.
3. Construction and exact public core declarations: `init`, real/imaginary access, conjugate/absolute magnitude/argument, arithmetic operations, comparison domain, formatting and parsing, checked/fallible cases, ownership and effects. Every public signature and member must be listed once canonical.
4. Numeric semantics: complex division stability; NaN, signed zero and infinities; rounding and ordering restrictions; equality/hash; algebraic assumptions prohibited for IEEE-like floating arithmetic.
5. Promotions and conversions: component types, scalar+complex rules, `rational`/`decimal` compatibility, explicit/implicit and fallible/lossy cases, literal typing and generic specialization.
6. Units/dimensions: whether components and operations can carry compatible quantities, what the mathematical dimension of the result is, and what combinations are disallowed.
7. **ABI:** whether representation is two consecutive components or a target-dependent native complex class; alignment, padding, register passing, C99/C++ complex compatibility **only where explicitly validated**, varargs/FFI exclusion by default, return storage, debug representation and ABI fingerprints.
8. Semantic IR/MLIR: intrinsic arithmetic, component extraction, calling conventions, constant evaluation, vectorized lowering if legal; separate semantics from the user-defined `@operator` proposal.
9. Core/library boundary: compiler-known identity and privileged operations belong to core; FFT, transforms and broader numerical algorithms are stdlib. Avoid reintroducing an unrelated `Complex[T]` nominal API.
10. Test matrix: cross-target math reference results, signed zero/NaN/Inf, division and branches, mixed types, units, formatting and interop, zero-allocation claims only when verified.

### § 9.4 Shared numerical design decisions that must precede implementation

| Question | `rational` | `complex` | Gate |
|---|---|---|---|
| Primitive identity | **Owner direction: yes; lowercase** | **Owner direction: yes; lowercase** | Version/type registry |
| Width / parameterization | Pending | Pending | Before signatures and layout |
| Default value and literal rules | Pending | Pending | Before parser/CTE |
| Error and allocation model | Pending | Pending | Before API review |
| Core methods/constructors | Pending exact surface | Pending exact surface | Before implementation |
| Native ABI + FFI legality | Pending explicit target plan | Pending explicit target plan | Before native calling support |
| Operator semantics | Core intrinsic plus potential user-defined integration | Core intrinsic plus potential user-defined integration | Before operator annotation tests |
| Units interaction | Exact scale metadata vs runtime type | Component/result dimensions | Before unit-enabled algorithms |
| Test and documentation completeness | Required | Required | Before 0.1.26/0.1.28 declaration of completion |

### § 9.5 Additional numeric candidates are not automatic blockers

`float16` and `bfloat16` have candidate gates 0.1.24/0.1.25 but are not owner-approved here as primitives. `BigInt`, `BigUint`, `BigFloat` and `float128` require a separate acceptance decision and, if approved for 0.2, new `0.1.x ≤ 100` slots. `Quaternion[T]`, `Dual[T]`, `Polynomial[T]` are *library candidates*, not new lowercase primitive types. Public `never` is already an IR-known special type; its 0.1.23 work is about exposing source semantics rather than inventing a new IR type.

---

## § 10. Type-level `@mustUse` as a linear obligation — specification packet

### § 10.1 Proposed meaning and the naming hazard

**Owner suggestion:** use `@mustUse` on a type declaration to mark *linear* behavior. This is a source-level proposal, not yet part of Sec 0.1's closed accepted attribute inventory. If approved, update `rules/foundations/attributes.md` and its registry before enabling it in the parser.

**Important distinction:** the compiler already treats certain results (notably `Result`) as must-use at expression-statement boundaries. "Do not ignore this returned value" is not the same guarantee as "an owned live object must reach exactly one valid terminal discharge". Treating those policies as the same just because they share the word *use* would be an error.

A useful candidate interpretation is: `@mustUse` on an owning nominal type declares a **mandatory discharge obligation**, enforced across all paths. What exactly counts as a discharge remains undecided; ordinary reading, copying, borrowing or automatic destruction **must not silently satisfy it** unless a precise adopted contract explicitly says so.

### § 10.2 New `rules/memory/linear_obligations.md` — mandatory topics

1. Exact legal annotation placement (type declarations, generic/derived types, return types?), applicability, inheritance and invalid targets; whether the rule belongs to a type versus a specific capability/token.
2. Formal operational states: available-undischarged; transferred-to-new-owner; discharged; invalid/moved-from; errors on double discharge or falling out of scope. Distinguish obligation state from optional `None` and storage generation validity.
3. Allowed terminal operations and proof of completion: consuming call, commit, rollback, release, ack, cancel? Decide case by case; do **not** assume generic `free()` automatically discharges transaction correctness.
4. `discard`, `defer`, custom `free`, implicit cleanup, panic, early return, `try`, cancellation and process termination. Define whether exceptional termination is considered discharge, a forced terminal path, or program-level abort; the compiler cannot promise impossible guarantees.
5. Move/borrow semantics: the existing `<-` ownership transfer, reborrowing and unique owner rules remain authoritative. Does the obligation move with the value? How does scope-based cleanup differ from semantic completion?
6. Wrapper/container propagation: `Option[T]`, `Result[T,E]`, unions, structs, arrays/list, generics, closure captures, tasks, channels and IPC. Specify each **public** type/API invariant exactly if amended.
7. Interface/generic constraints and declaration: whether `@mustUse` affects generic bound inference, method sets, conformance and overload selection; whether a library can declare a linear protocol without creating a new type.
8. Call/return ABI: transport of an obligated value without implicit duplication; compile-time obligation state is not by itself a runtime counter. Decide any actual runtime tag if truly required, and document native/C ABI legality and diagnostic boundaries.
9. Semantic IR/MLIR: stable obligation identity, transfer/discharge plans, invalid edges, destruction and branch joins, drop obligation detection, incremental summary invalidation; verifier must reject missing discharge, not silently insert it.
10. Diagnostics and examples: include one accepted and one rejected example for each terminal path and nested/generic case; diagnostic names must say **which obligation** remains and offer a legal discharge without inventing ownership authority.
11. Conformance: unit, semantic, invalid, IR, lowering, backend, execution, regression and fuzz/property testing of control-flow histories; retain all evidence in governance under the owning canonical IDs.

### § 10.3 Relationship to `Owner[T]`

`Owner[T]` already targets 0.1.1 and is move-only, but **not automatically linear**: a unique owned object may ordinarily be destroyed at scope exit. The `@mustUse` design must distinguish **ownership of a value**, **its storage reclamation**, and **a protocol obligation to complete something**. `Owner[T]` can later carry a value of a `@mustUse` type if the general rules allow it; do not create `LinearOwner[T]` merely to work around missing type-level obligation semantics.

### § 10.4 Intuition test for the spelling

The name `@mustUse` naturally suggests "the value may not be ignored"; some programmers will not immediately infer "exactly one terminal operation is required". **Do not reject the owner's proposed spelling**, but require concrete code examples and error diagnostics to verify that this distinction is learnable and predictable. No separate `@linear` spelling is chosen here.

---

## § 11. Mandatory rulebook production contract for all new books and revisions

### § 11.1 Header, ownership and paragraph identity

Every accepted new book or revision must declare **Title, canonical path, Created, Last updated, Document revision, Sec language version introduced, Status, Normative owner, Replaces/supersedes, Related books and MDs, Repository baseline reviewed**, and exact source of authorized decisions. All paragraphs receive stable identifiers (`§ N(M)` or established existing conventions); corrections append or amend without renumbering existing citations. Drafts distinguish `[LOCKED]`, `[PROPOSED]`, `[OPEN]`, `[IMPLEMENTED]` and `[TESTED]` where meaningful and never equate them.

### § 11.2 Detailed normative contents — no API placeholders

A book is implementation-ready only when it provides **every public type/constructor/method/property/function/enum/union variant/interface member** in exact current Sec syntax; comments describing variants/members are placed immediately **before** the declaration. For every function include:

- Purpose, signature, visibility, static/instance/receiver capabilities, parameter types and ownership (`ref`, `ref mut`, consuming), return type and fallible error type.
- Success, error and panic behavior, validation order, data/evaluation effects, allocation/synchronization/blocking/suspension, reentrancy, deterministic cleanup and cancellation.
- Type/generalization/literal inference, contracts, range/unit bounds, generic constraints, exact overload/collision rules and diagnostics.
- Edge conditions, invalid forms, source examples, interaction with existing language operations and compatibility/migration.

No `...`, "known values may include", `static fn New()`, guessed constructors, unspecified enums, or undefined public aliases in accepted APIs. If a decision is open, explicitly keep the book **draft**, with a named decision ID, rather than guessing a source API.

### § 11.3 ABI and physical representation matrix

Every book must either define the relevant public **ABI obligations** or explicitly state **"No new ABI: existing rules/platform/abi.md and layout facts apply"**, with rationale. For an ABI-changing type, include:

| Area | Required declaration |
|---|---|
| Source identity vs physical identity | Compiler-known/nominal ID, specialized generic identity, ABI fingerprint ownership |
| Layout | Size, alignment, padding, address space, storage descriptor, target-conditional variants, validity/niches |
| Calls | Native Sec ABI arg and return classification; receiver, hidden result place, by-ref/by-value rules, cleanup on failure |
| Foreign boundaries | C/system ABI admissible types and explicit adapter rules; no automatic reinterpretation |
| Lifetime | Ownership of returned/borrowed storage, pin/relocation, transfer, destruction and error cleanup |
| Atomicity/concurrency | Memory model, shareability, ordering, synchronization if applicable |
| Compilation artifacts | Symbol/type fingerprints, separate compilation, incremental invalidation, cross-target selection |
| IR & lowering | Required Sema plans, Semantic IR facts, Sec MLIR operations, schema version and verifier contracts |
| Debug | Source values mapped to physical locations, target-dependent debug restrictions |
| Compatibility | Release/version gate, old ABI detection, migration and forbidden silent fallback |

**The language should not freeze a single target-independent struct layout simply to simplify implementation.** Normative source semantics and target-specific ABI classifications must be separate. A compiler builder must receive *enough exact information* to implement each target path deterministically.

### § 11.4 Mandatory compiler handoff

List the exact parser/CST/formatter forms; Sema acceptance/rejection and immutable resolved plan; Semantic IR op(s) and invariants; Sec MLIR dialect and lowering (including schema negotiation); native/foreign ABI plan; runtime/core/stdlib owner; diagnostics and LSP effects. Do not create a second resolver in IR/lowering, silently substitute a different error family or claim an unsupported backend feature implemented.

### § 11.5 Mandatory conformance test catalogue

For each public operation, build a matrix with **valid source**, **invalid source**, **CTE**, **semantic analysis**, **Semantic IR**, **MLIR verification**, **lowering**, **native execution**, **ABI/FFI**, **target capability negative**, **ownership/borrowing/destruction**, **error/panic/effects**, **LSP/formatter**, **regression**, **fuzzing/property checks** where applicable. Define expected diagnostics/error variants and observable results, not only test filenames. Verify old behavior and representative multi-feature programs; document genuinely unsupported targets instead of silently skipping.

### § 11.6 Sync order and implementation authority

1. **Begin with the per-version document (§18)** and the §17 adversarial design review. List already normative, accepted-new, proposed, specification-only, and already implemented work separately.
2. For genuinely open semantics only, obtain the Sec owner's decision and record it under an existing MD ID if one owns the question; create a new MD only if needed. Never reopen a locked 0.1 decision solely because it is placed in a 0.1.1 delivery package.
3. Update the canonical owning rulebook/new book and all cross-rulebooks; preserve existing paragraph identities. Define every public declaration, errors, ownership/effects, ABI, IR and target behavior exactly.
4. Attach the canonical conformance matrix and evidence requirements to the version document. Turn incomplete items into small, independently testable implementation tasks. Compiler AIs work **one point at a time with tests**, never resolve an undecided API by guessing.
5. Implement parser/Sema/IR/MLIR/backend/runtime and public core/stdlib work per dependency, including regression, ergonomics, source and target tests; unsupported prerequisites remain visible.
6. Update formatter, LSP, manual, source documentation and sample programs **before** release tagging; verify that source code examples compile or are explicitly marked future syntax.
7. Reconcile existing `governance/` IDs, statuses and evidence, plus relevant MD/correction records. Create new status IDs only for genuinely uncovered work, never duplicate an existing entry. Preserve `identified_bugs` in the user's separate audit process.
8. Run a final critical review and whole-release conformance check (§§17–19). Publish immutable `-rc` artifacts and final tag only after all accepted criteria pass. Version documents, normative rulebooks, governance and toolchain capabilities must agree. Work on subsequent releases may overlap but does not waive this gate.

---

## § 12. Complete map of the 34-division ideas snapshot

The table maps **all 34 numbered ideas chapters** to staged work, continuous existing work, or explicit exclusion. Subsections are covered by their chapter's detailed release checklists; none is silently treated as a brand-new feature merely because it appears in the idea snapshot.

| Ideas | Topic | Delivery mapping | Boundary |
|---|---|---|---|
| §1 | Less bureaucracy, meaning of annotations, intuition | Cross-cutting, all `0.1.x` | Acceptance criterion, not parser feature |
| §2 | Declarations and keyword consistency | 0.1.3, 0.1.14 | Existing declaration semantics retained unless versioned |
| §3 | Ranges inside literals | 0.1.9 | Capacity/evaluation guarantees |
| §4 | Enum context and type inference | 0.1.4, 0.1.5, 0.1.15 | Do not duplicate supported `.Member` |
| §5 | Conditional and multi-condition expressions | 0.1.6, 0.1.13 (multichoice considered) | `?:` previously reserved, not approved |
| §6 | `in` and `not in` | 0.1.10 | Preserve same conceptual membership model |
| §7 | Nested and ownership-aware match | 0.1.11–0.1.12 | Result-specific 0.1.2 exception distinct |
| §8 | Context lambdas and value blocks | 0.1.7, 0.1.13 | Capture/escape exactness |
| §9 | Callable environments, transferability, cardinality | 0.1.45 | Existing callable analysis remains base |
| §10 | References, origins, nonescape | 0.1.16–0.1.18 | No source lifetime annotations just for inference |
| §11 | Places/projections | 0.1.19; compiler continuous | Same evaluation and alias model |
| §12 | Internal access mode | 0.1.19; compiler continuous | Internal mechanism not source ceremony |
| §13 | Equivalent ownership implementations | 0.1.35, 0.1.54 | Proof of equivalence; not implicit new effects |
| §14 | Copy/move bureaucracy | 0.1.39 | Preserve intentional consuming syntax unless decided |
| §15 | Required fields/constructors | 0.1.20–0.1.21 | Partial construction cleanup |
| §16 | API argument labels | 0.1.22 | No implicit dependency injection |
| §17 | Numeric formatting | **Already partially covered**; core/stdlib audit | `_` separators already implemented 0.1; only real gaps become tasks |
| §18 | Linear/affine types and capability decomposition | 0.1.36–0.1.38 | Owner-preferred `@mustUse` under review |
| §19 | Purity/effects | 0.1.40–0.1.41 | Existing effects base, source additions only if needed |
| §20 | Generic capability checks | 0.1.32, 0.1.34 | Sema/inference, avoid wrappers |
| §21 | Semantic compiler queries/call graph | 0.1.15, 0.1.53; tooling ongoing | CLI/LSP, not automatically a new type |
| §22 | Region concurrency reasoning | 0.1.46 | Analysis can mature independently |
| §23 | Tokio-derived cancellation, permits, scheduling | 0.1.37, 0.1.47–0.1.52 | Existing Task/Channel/Context models stay canonical |
| §24 | Function contracts | 0.1.42–0.1.43 | Runtime safety checks never optionally bypassed |
| §25 | Result-location semantics | 0.1.35, 0.1.54; compiler ongoing | Optimization not source-ABI guarantee by default |
| §26 | Explicitly rejected directions | **Excluded** | No throw/implicit DI/general reflection/hidden repair |
| §27 | Summary of candidate ideas | **Index only** | No duplicate backlog |
| §28 | Numeric candidates, `never`, intervals | 0.1.23–0.1.29; optional other slots | Owner direction: `rational` and `complex` primitive; intervals separate |
| §29 | Math library, quaternions, duals, polynomials | 0.1.29, 0.1.32–0.1.33 | Nominal library math distinct from primitive complex |
| §30 | User-defined operators | 0.1.30–0.1.31 | Ordinary static overloads, built-in types protected |
| §31 | Ergonomics and intuition | **All gates** | Review user programs, not just syntax economy |
| §32 | Effects and verifiable contracts | 0.1.40–0.1.44 | Koka/SPARK ideas are investigation only |
| §33 | MLIR/SIMD, Goose comparisons | 0.1.44, 0.1.54; R&D | Relative refs/packed enums are **not** accepted primitive deliverables |
| §34 | Snapshot boundary | All gates | Idea ≠ decision ≠ implemented ≠ verified |

---

## § 13. Critical dependencies and change-impact register

| Dependency chain | Why it matters | Release implication |
|---|---|---|
| Owner[T] → generic owned references → @mustUse | Linear disposal cannot be specified by assuming ownership is exactly-once protocol completion | Owner[T] 0.1.1 baseline; linear 0.1.36+ |
| Regex spec → engine/contract lowering | S1058 currently correctly rejects unvalidated regex contracts | Book at 0.1.1, executable gate 0.1.8 subject to P22/P23 |
| Core numeric models → rational/complex ABI → operators | Literal/promotions/representation must be fixed before tests can promise native arithmetic | 0.1.26–0.1.32 |
| Generic callable rules → @operator generics | Operator overloads are statically resolved ordinary functions, not new runtime method tables | 0.1.30–0.1.32 |
| Match projection/provenance → linear wrapper handling | Hidden moves through nested patterns could invalidate ownership or discharge proof | 0.1.11–0.1.19 before 0.1.36–0.1.38 |
| Effects → verified contracts → optimization proofs | A compiler may omit a runtime check only after proving safety, never based on an unchecked build mode | 0.1.40–0.1.44 |
| Isolation/callable effects → cancellation/permits | A linear permit with cancellation must have an unambiguous disposition | 0.1.36–0.1.52 |
| Native Sec ABI → foreign ABI adapter → new types | First-class type does not guarantee FFI compatibility or frozen representation | Numeric/Owner/linear books must specify target matrix |
| IR schema ≠ language version | Introducing `!sec.rational` does not by itself make a source operation executable | Each release must have end-to-end backend evidence |
| Actual UX trials → release acceptance | Syntax and semantics that are correct but unintuitive deserve revision before 0.2 freeze | Every gate and 0.1.55/0.2.0 |

**Reserved slots:** `0.1.56`–`0.1.100` are available for split risks, accepted future ideas and implementation dependencies. If a planned release becomes impracticable, change its target **before** publishing a release, without claiming incomplete functionality as stable.

---

## § 14. Open design questions, risks and non-decisions

1. **`rational` representation:** finite width vs arbitrary precision, canonicalization cost, allocation/failure, literal and operator semantics, target and C ABI; distinguish compile-time exact fractions in unit metadata from user runtime values.
2. **`complex` representation:** component width/default, optional parameterization, native layout/ABI/FFI and conversions; whether compiler-intrinsic operators are all needed and how they interact with user `@operator` functions.
3. **`@mustUse`:** scope and meaning of the annotation; exact-once terminal obligation vs simply forbidden ignored results; whether `free`/automatic destruction count, and behavior on panic/cancellation or nested generics.
4. **`@operator`:** ownership/coherence across nominal types and modules, consuming operands without hidden moves, precedence/associativity, comparisons, `x`, and compound assignment.
5. **Regex:** normative language scope, engine resource guarantees and complete API, integrated as one dialect used both by contracts and ordinary programs.
6. **Source changes:** optional `let`, direct declarations, keyword consistency, conditional operators and label syntax need migration and formatter/LSP rules that respect existing valid Sec 0.1.
7. **Proof contracts:** a failed proof is not the same as a failed runtime check; decide when compile rejection is intended and when mandatory runtime enforcement remains.
8. **Concurrency:** prevent duplicate abstractions and new hidden effects; preserve exact ownership of in-flight values and cancellation outcomes.
9. **API/ABI:** do not confuse native ABI with C ABI or public semantic type with physical layout. Unsupported targets require explicit compile failures, not silent fallback.
10. **Library scope:** library mathematical types and numerics are not proof of primitive support; only `rational` and `complex` have the new owner-directed first-class classification here.
11. **Detailed implementation order:** no MLIR schema number or actual compiler package is promised by this planning document. When a book is accepted, add its exact immutable Semantic IR/MLIR and target dependencies in its own implementation package.
12. **Versioning:** test whether existing Sec manifest parser accepts prerelease version syntax; no assumption that a suffix is already supported.

### § 14.1 No premature normative decisions

This revision **records the owner's choice of primitive/type category** for `rational` and `complex` and the **suggestion** of `@mustUse` for linear declarations. It does **not** settle their complete semantics, approve operator annotations, approve all other idea-book proposals, or mark a release implemented. A later accepted rulebook must supply the exact public API/ABI contract and diagnostics before implementation begins.

### § 14.2 Done criteria for this roadmap revision

- Every numbered idea chapter has an explicit mapping (§12).
- All major accepted/potential source changes have candidate `0.1.x ≤ 100` gates (§2/§8), with true acceptance still to be decided.
- First-class `rational`/`complex`, `@mustUse`, regex, operators and contracts each have a dedicated rulebook/documentation path (§3, §§9–11).
- API, ABI, CTE, ownership, IR/MLIR, target, tests and tooling are specified as mandatory *contents* of each book before build tasks (§11).
- Release channel terminology and the meaning of `0.2.0` remain precise and separate from production stability (§5–§6).

**Next working method:** review one version and its rulebook contracts at a time; decide genuinely open semantics, then produce the complete normative book or revision with stable paragraph identifiers and a conformance/implementation handoff. Preserve cross-rulebook ownership and governance IDs.


---

## § 15. Consolidated new-rulebook inventory and ownership

The following paths are **proposals** unless an existing path is named. Creating a file does not approve its source syntax or complete its implementation. Canonical placement must be reconciled with `rules/README.md` before adoption.

| New book / planned authority | Earliest candidate | Scope that must be complete in the book | Existing owners not to duplicate |
|---|---|---|---|
| `rules/memory/owner.md` (working name) | 0.1.1 | Unique value ownership through indirection, storage producer, `Owner[T]` authority, recursion, transfer, borrows, cleanup, exact public declarations and ABI | `ownership.md`, `reference_model.md`, `allocation.md`, `layout.md`, `destruction.md` |
| `rules/regex/regular_expressions.md` | 0.1.1 (normative), 0.1.8 (runtime) | One regex dialect for library use and named string contracts; exact public API, complexity/resource policy, Unicode, errors, target runtime | `types/contracts.md`, `types/types.md`, `errors/runtime_checks.md` |
| `rules/types/rational.md` | 0.1.26 | First-class lowercase primitive domain, representation, math, numeric conversions, core source API, FFI/ABI, CTE and lowering | `types/types.md`, `types/units.md`, `core-library.md`, `platform/abi.md` |
| `rules/types/complex.md` | 0.1.28 | First-class lowercase primitive domain, component precision, arithmetic, APIs, layout/ABI and typed interop | Same numeric/ABI/units owners, `shaped-types.md` where involved |
| `rules/foundations/operator_overloading.md` | 0.1.30 | Static annotated operator declarations, module coherence, overload ranking, ownership/effects and compound operators | `operators.md`, `declarations/functions.md`, `declarations/impl.md`, `foundations/attributes.md` |
| `rules/memory/linear_obligations.md` | 0.1.36 | Proposed type-level `@mustUse`, discharge semantics, transfer, aggregate/generic propagation, error/panic/cancel, API and ABI | `copy_move.md`, `destruction.md`, `attributes.md`, `owner.md` |
| `rules/declarations/function_contracts.md` | 0.1.42 | Source-level pre/postconditions, compile-time proof, mandatory runtime checks, source API, purity/effect boundary and ABI | `types/contracts.md`, `analysis/effect_analysis.md`, `errors/runtime_checks.md` |
| `rules/analysis/flow_analysis.md` (only if needed) | 0.1.15 | Authoritative flow-fact/provenance rules if no existing book is a suitable owner | Existing `types.md`, `semantic_ir.md`, reference and analysis books |
| `rules/analysis/region_analysis.md` (only if needed) | 0.1.46 | Isolation-region proof model and transfer to concurrency boundaries | `transferability.md`, `data_races.md`, `escape_analysis.md` |
| `rules/tooling/compiler_queries.md` (only if needed) | 0.1.44/0.1.53 | Stable compiler proof/query API for CLI and LSP, versioned machine-readable responses | `tooling/cli.md`, `tooling/lsp.md`, `analysis/pitfall_analysis.md` |

**Conditional books:** an unexpectedly broad feature may earn a dedicated new book only when it has a distinct canonical semantic owner. Otherwise revise the existing book. Do not create one book for each syntax sugar item simply because it has a separate release gate.

---

## § 16. Existing-rulebook revision register

This register lists likely revisions; it is **not** a list of already needed edits to make immediately. Each row must be narrowed to exact stable paragraphs and tested API/ABI consequences after the corresponding feature is accepted.

| Existing book / area | Staged triggers | Revision work and ABI boundary |
|---|---|---|
| `rules/foundations/grammar.md`, `operators.md`, `lexical_structure.md`, `attributes.md` | 0.1.3–0.1.14, 0.1.30, 0.1.36 | Exact grammar, precedence, syntax versions, closed attribute registry; normally no machine ABI, but complete AST/formatter/diagnostic shape |
| `rules/types/types.md`, `default_values.md`, `units.md`, `contracts.md` | 0.1.4–0.1.5, 0.1.20, 0.1.23–0.1.29 | Type identities, literal/default/units, conversions, byte/scalar differences, CTE, first-class numeric types, contract runtime effects; ABI annex for changed scalars |
| `rules/declarations/functions.md`, `impl.md`, `generics.md`, `interfaces.md`, `spread.md` | **0.1.1 (MD-117 and MD-120 integration)**, 0.1.5, 0.1.22, 0.1.30–0.1.34, 0.1.42–0.1.45 | Finish already accepted heterogeneous type-pack calling/forwarding and generic error category/interface specialization; then later overload/labels, capability requirements and interface obligations; same resolved-call ABI unless explicitly amended |
| `rules/declarations/struct.md`, `unions.md`, `enums.md`, `properties.md`, `lambda-functions.md` | 0.1.4, 0.1.7, 0.1.11–0.1.14, 0.1.19–0.1.21, 0.1.45 | Required fields, partial construction, pattern/projection/capture, chosen layout and ABI obligations; no unapproved implicit temporaries |
| `rules/memory/ownership.md`, `borrowing.md`, `copy_move.md`, `references.md`, `reference_model.md`, `destruction.md`, `storage.md`, `layout.md` | 0.1.1, 0.1.12, 0.1.16–0.1.19, 0.1.35–0.1.39 | Owner indirection, lifetime/provenance, linear obligations, move ergonomics, physical storage and ABI; stable Place/object identity |
| `rules/collections/collections.md`, `shaped-types.md` | 0.1.9, 0.1.32–0.1.33, 0.1.38 | Literal expansions, generic algorithms and linear elements; respect known bounded-capacity/shape representation and layout |
| `rules/errors/errorhandling.md`, `runtime_checks.md`, `panic.md` | **0.1.1 (MD-120 implementation integration)**, 0.1.2, 0.1.8, 0.1.26–0.1.29, 0.1.42–0.1.44 | Exact generic error-family identity, Result propagation and generic error unions/enums first; subsequent error patterns, numeric/regex errors and validity checks; result ABI and cleanup on failure |
| `rules/analysis/effect_analysis.md`, `escape_analysis.md`, `closure_analysis.md`, `data_races.md`, `call_graph.md` | 0.1.15–0.1.18, 0.1.34, 0.1.40–0.1.46 | Proof joins, effects across higher-order calls, nonescape and isolation, stable summary/caching metadata; no new ABI unless callable plan changes |
| `rules/concurrency/cancellation.md`, `channels.md`, `select.md`, `tasks.md`, `spawn.md`, `scheduling.md`, `blocking.md` | 0.1.37, 0.1.47–0.1.52 | Exact async result variants, cancellation state, permits, domain and fairness; public APIs and native handle transport explicit |
| `rules/library/core-library.md`, `stdlib.md`; `sec/core/`; `sec/stdlib/math/` | 0.1.26–0.1.33, 0.1.48 | Full source declarations, formatting/conversions, maths and optional permit methods, allocation/error contracts; first-class primitive identity stays with compiler |
| `rules/compiler/semantic_ir.md`, `compiler_testing.md`, `incremental_compilation.md`, `compile_time_evaluation.md` | All semantics-affecting stages; especially 0.1.26–0.1.55 | Immutable plans and verifier contracts, CTE/native equivalence, incremental compatibility, no resolver duplication; explicit schema/ABI interfaces |
| `rules/mlir/` packages/dialect/lowering/versioned amendments | Whenever native realization changes | Precise schema and lowering ops, supported target matrix, semantic fallback errors, backend negative tests; MLIR schema version is not Sec release version |
| `rules/platform/abi.md`, `ffi.md`, `platform_model.md` | 0.1.1, 0.1.26–0.1.29, 0.1.36–0.1.38, 0.1.45–0.1.50 | Native and C ABI classifications, target capabilities, layout, names/fingerprints, foreign legality, call boundary cleanup |
| `rules/tooling/cli.md`, `lsp.md`, `formatter.md`, `testing.md`; manuals | All new source syntax/APIs | Diagnostics, hover, completion, source examples, migration, user intent and conformance; release-info output reproducibility |

### § 16.1 Book handoff as a bounded implementation package

A book is **ready for compiler implementation** only if an engineer can obtain from it the exact grammar, type identity, every public declaration and error, the full observable semantics, compile-time/static proof obligations, source-to-IR lowering contract, ABI classification or explicit inherited ABI rule, target limitations and a reproducible conformance matrix. If any of those affects correctness and is unresolved, record it as an owner decision instead of leaving implementation authors to choose on their own.

**Do not treat a numerical test or successful build as semantic approval.** A release remains candidate until canonical books and governance agree on the exact same delivered facts.

---

## § 17. Mandatory feature-intake and critical design procedure

This procedure applies to **every** candidate feature or significant revision, including work imported from an idea document, another programming language, compiler implementation, real application experience, or `missing-decisions.yaml`. It applies equally to proposals that seem small or obvious. A feature does not earn approval merely because it is possible or attractive.

### § 17.1 Start with the programmer's problem

**§ 17.1(1) Problem statement.** Every candidate must state a concrete, recurring programmer problem, including examples of the best valid **current Sec 0.1** solution and what is awkward, unsafe, misleading, costly, unexpressible or unnecessarily repetitive about it. Do not use fabricated unsupported syntax as evidence of a current deficiency.

**§ 17.1(2) Realistic evidence.** Prefer small genuine core/stdlib/application examples and actual diagnostics. Compare simple usage, nontrivial composition, invalid usage and maintenance over time. If an idea is motivated only by a hypothetical capability, keep it `research`.

**§ 17.1(3) Scope classification.** Decide whether the solution actually belongs to (a) source language semantics/syntax, (b) core or stdlib public API, (c) compiler optimization/MLIR, (d) compiler analysis, (e) tooling/editor/formatter, (f) platform/ABI, or (g) documentation/diagnostics. Do not spend a language feature to solve a problem the compiler or library can already solve safely.

### § 17.2 Challenge the proposal rather than defend it

**§ 17.2(1) Strongest counterargument.** For every new feature, write at least one substantial argument **against** it. Compare the simplest reasonable alternative, including doing nothing. Examine whether the new feature merely hides a bad library API or an implementation gap.

**§ 17.2(2) Avoid both extremes.** Sec is neither a deliberately minimal Go clone nor a maximalist Ada clone. Reject artificial restrictions that make ordinary code cumbersome; also reject syntax, annotations and generic mechanisms whose benefits do not justify additional concepts or exceptional rules.

**§ 17.2(3) Ergonomics.** Show how frequently encountered tasks read before/after the change, how many distinct concepts/annotations are needed, and whether the programmer repeats information the compiler already proves. Extra syntax is justified when it communicates meaningful intent, ownership, an effect, an allocation or an operation that cannot safely be inferred.

**§ 17.2(4) Intuition.** Ask what a competent programmer would predict about evaluation order, result type, move/copy/borrow, lifetime, failure, allocation, synchronization, blocking, cancellation, performance cost and method/property behavior. A correct rule may still be a design defect if normal usage is surprising. Test against other existing Sec facilities, not only the local feature.

**§ 17.2(5) Complexity budget.** Record new grammar forms, name-resolution rules, special cases, implicit behavior, API surface, compiler analyses, ABI/target burdens, maintenance/teaching costs, migration and failure modes. State why this cost is justified. Reusing a unified existing mechanism is preferred to adding a parallel one.

**§ 17.2(6) Comparative evidence.** Study relevant other-language solutions where helpful, but distinguish their semantics and constraints from Sec's. A borrowed idea is an input to review, never its own acceptance argument.

### § 17.3 Required interaction and regression challenge

Every candidate must check interactions with the **actual current Sec model**:

1. Scope, overloads, nominal identity, generics, constraints, interface conformance and existing compiler-known member lookup.
2. Evaluation order and once-only evaluation; copy/move, `ref`/`ref mut`, availability, partial moves, destruction, error/panic/`try`/`defer` and cancellation.
3. Place identity, storage stability, lifetime/provenance, allocations and effects, CTE, defaults, named-type contracts and units.
4. Collections, shaped types, pattern matching, lambdas/captures, callable values, task/thread/channel effects and target-specific behavior where applicable.
5. Parser and CST, formatter round-trip, LSP/CLI diagnostics, Semantic IR plans, MLIR verifier/lowering, native and C/system ABI, debug information and incremental compilation.
6. Old valid programs must retain semantics or have an explicit, justified **versioned migration**. Unsupported target/compiler slices fail clearly, not with silent unsafe fallback.

Do **not** require irrelevant implementation detail in user-facing syntax. Do require the owning rulebooks to define every semantic consequence. Use realistic *valid*, *invalid*, *edge* and *composed* source examples before approving a new rule.

### § 17.4 Missing-decisions intake

**§ 17.4(1) Reconcile, do not duplicate.** Search `missing-decisions.yaml`, applied/pending corrections, the canonical owning rulebooks and existing governance before opening a new MD or scope item. Retain exact IDs and prior decisions. A new version is not permission to restate an already approved rule as undecided.

**§ 17.4(2) Worth staging.** Consider an existing major open MD when it blocks a promised feature, causes a real semantic hole, is unsafe/contradictory, affects multiple rulebooks, or carries substantial ergonomic value. Do not bulk-copy every MD into `0.1.x`. Small issues remain with their owning implementation/audit work unless they materially change the release contract.

**§ 17.4(3) Explicit owner decision.** Record the problem, candidate destination, actual normative status, implementation status and upstream/downstream dependencies. The Sec owner alone approves whether the MD is in scope and its version; review does not silently change the MD's meaning or status.

**§ 17.4(4) Closure.** A decision can be approved before implementation. Mark MD resolution, normative synchronization and implementation completion separately, using the repository's **existing** closure/status rules and governance IDs.

### § 17.5 Review outcomes

For every proposed feature use exactly one **planning disposition** until it is reconsidered:

| Disposition | Meaning |
|---|---|
| `already_covered` | Existing Sec rule is sufficient; improve tests/docs/tooling as needed. |
| `implementation_only` | Correct semantics exist; finish compiler/core/stdlib realization. |
| `tooling_or_library` | Language addition is unnecessary; solve with the appropriate existing layer. |
| `accepted_for_version` | Owner approves the source-visible or normative change and its version after critique; exact book contract required. |
| `needs_design` | Real need exists but exact semantics/API/ABI/interaction remains undecided. |
| `deferred` | Valuable but not required for this release sequence. |
| `rejected` | Cost, conflict or risk outweighs benefit; record rationale. |

The dispositions above are **not** new `governance/` status values. A scope decision never becomes an `implemented` claim without executable evidence.

---

## § 18. Mandatory per-version document — content, control and lifecycle

### § 18.1 One owner-readable delivery contract per release

**§ 18.1(1) Required document.** Create at least one dedicated Markdown version document **for each planned or published `0.1.x` version** before the version's implementation is treated as a release workstream. Suggested paths, subject to repository approval, are `planning/versions/sec-0.1.1.md`, `planning/versions/sec-0.1.2.md`, etc. The document is a **planning/acceptance authority**; it does not replace canonical `rules/` semantics or `governance/` implementation truth.

**§ 18.1(2) Multiple books.** A release may own several distinct feature packets and new/revised normative rulebooks. A single version document must enumerate, depend on, link to and accept/reject **every** such deliverable. One major area with independent language semantics should normally receive its own rulebook (regex is the model); small refinements should normally revise existing canonical books. Never duplicate the same semantic owner in two books.

**§ 18.1(3) Early work.** A release's specification, implementation and conformance tests may be undertaken before the preceding numerical version is published, provided dependencies are tracked explicitly. Completion of an implementation task is not publication of the language release. A published release must have met all its own gates and any prerequisite release/compatibility contracts.

**§ 18.1(4) No guessing.** Implementers and coding AIs are required to stop at a genuinely unresolved source/API/ABI choice, record its exact question and alternatives in the version document/MD register, and obtain owner approval. They may implement independently resolved tasks, one at a time, with tests. They must not infer public APIs from partial examples or invent fallback semantics to make a test pass.

### § 18.2 Required version-document sections

Each version document must have stable numbered sections and a substantial header. The **minimum content** is:

1. **Identity and provenance:** version, title, document ID/path, created/updated, revision/status, baseline commit/branch, owner and revision policy; previous/next dependency context, affected `0.2.0` promise.
2. **Objective and exclusions:** programmer problems, intended observable behavior, explicit out-of-scope items, proposed versus approved decisions, and any specification-only milestone.
3. **Itemized delivery inventory:** stable item IDs (e.g. `SEC-0.1.1-01`), source idea/MD, scope category from §17, acceptance status, normative owner, governance ID, dependencies, implementation status and acceptance evidence. Never invent `implemented` status.
4. **Adversarial review:** current Sec baseline, at least one realistic use case, best alternative and strongest objection, ergonomic/intuitive consequences, complexity budget and decision outcome for **each** genuinely new capability.
5. **Normative books and revisions:** exact new/revised book paths, stable paragraph references, public API completeness, source syntax/grammar, error/effect and ownership model. Every new public operation has a full contract; include reference to §11 and exact version compatibility.
6. **API/ABI and platform matrix:** exact public types and signatures **in their owning books** (reference them, do not invent/duplicate competing definitions); native Sec calling/return ABI, source-vs-physical identity, layout and FFI legality, target caveats, allocation, destruction, concurrency and debug. Explicitly state and justify `No new ABI` where appropriate.
7. **Compiler handoff:** parser/CST/formatter, type checker/Sema and CTE, resolved semantic plans, Semantic IR, Sec MLIR and schema/lowering/LLVM, core/stdlib/runtime, incremental fingerprints and build/release metadata.
8. **Conformance tests:** positive, negative, edge, interaction, regression, property/fuzz where relevant, target/layout/ABI, native execution, diagnostics/LSP/manual. Specify **expected behavior and errors**, not merely test filenames; label tests blocked by absent compiler infrastructure.
9. **Task sequence:** granular dependency-based implementation tasks with stable IDs, owning repository area, one-step test, blocking decisions, integration point and evidence location. Avoid an undifferentiated “implement feature” task.
10. **Readiness and closure:** outstanding blockers, explicit accepted exclusions, user-experience/dogfooding result, owner decision record, final governance/MD synchronization, release-candidate evidence and signed-off definition of done.

The separate `sec-release-version-document-template-2026-11-02.md` contains a reusable skeleton. Its prose and IDs are illustrative only, never normative by themselves.

### § 18.3 Release readiness gates

| Gate | State | Evidence required |
|---|---|---|
| `V0` | Inventory started | Scope candidates and previous normative decisions identified; no implementation claim. |
| `V1` | Design challenged | §17 problem/alternatives/UX/interaction review recorded; no unresolved design requirement silently delegated. |
| `V2` | Specification approved | Owner has approved needed choices; canonical new books and revisions carry exact API/ABI/semantics with stable section references. |
| `V3` | Implementation planned | Dependency graph, small implementation tasks and conformance matrix are complete; owners and accepted exclusions explicit. |
| `V4` | Implemented | All promised source/compiler/core/runtime/lowering paths implemented on claimed targets; no missing emitter masquerades as coverage. |
| `V5` | Verified | Automated positive/negative/ABI/target/regression and practical usability checks pass, with reproducible evidence. |
| `V6` | Release ready | Manual/LSP/formatter/docs synchronized; MD/governance updated; owner approves RC/final artifact and known exclusions. |

A **specification-only** accepted deliverable (e.g. regex rulebook in `0.1.1`) can pass its *own* `V2/V3` book criteria and be marked `spec_complete`, while explicitly **not** passing the future engine's `V4/V5`. Such a book deliverable does not imply that regex execution is offered in `0.1.1`. Version `0.1.1` is not fully ready until every **promised-for-0.1.1** execution deliverable passes all relevant gates.

### § 18.4 Change control and authoritative status

1. Number version-document requirements as stable item IDs; do not renumber published normative paragraphs. Retire or supersede an ID explicitly rather than silently reusing it.
2. Maintain an explicit separation of **Normative status**, **Implementation status** and **Release gate**. A source feature that was specified in 0.1 but finished in 0.1.1 stays **specified for 0.1**.
3. Sync the existing `governance/` owner and MD record after each independently verified implementation step; never create parallel truth in roadmap tables.
4. All material mid-release scope changes require owner approval, impact assessment and a revised version document. When a feature cannot be completed, move or remove its release assignment **before** claiming that version complete; preserve any previously locked normative activation.
5. Run the §17 review again when real implementation/dogfooding reveals a semantic or ergonomic surprise. A working implementation is not a veto against improving a poor design, and a beautiful spec is not evidence that the code works.

---

## § 19. `0.1.1` initial version-document scope and critical completion questions

This section is the **input** for the separate `0.1.1` version document, not a substitute for it. Status descriptions refer to the checked repository baseline, not to hypothetical future work. Its four streams must each have distinct stable requirements and evidence.

### § 19.1 `SEC-0.1.1-01` — `Owner[T]` owning indirection

- **Normative state:** accepted design direction, working public name and incomplete construction/lifetime API (`Sec-Owner-0.1.1.md`). The required new/revised canonical books must not assume an `Owner[T]` constructor or release method that has not been decided.
- **Critical questions:** Is an extra value-ownership carrier really essential for recursion and stable storage, compared to existing owned values, arenas and references? Can an owner be constructed/destructed without implicit allocation or arbitrary recursive stack growth? What is the smallest learnable API that makes storage authority and lifetime obligations obvious?
- **Release evidence:** approved exact API/ABI and storage-producer protocol; owning recursive layout, borrow/move/cleanup and invalid-source tests; native execution where promised; known target and allocation failures handled explicitly.

### § 19.2 `SEC-0.1.1-02` — Dedicated regular-expression rulebook (MD-010)

- **Normative state:** MD-010 remains open pending a dedicated regex language. Lexer/parser/Sema currently retain the regex contract but correctly reject unvalidated contracts with `S1058`.
- **Critical questions:** What is the minimum genuinely useful **coherent** regex dialect? Which patterns are accepted/rejected? What Unicode semantics, worst-case algorithmic costs and limits prevent denial-of-service-like pathological matching? Can the programmer predict escaping, anchoring and capture indexing? Is the same language used for both API calls and named-type contracts?
- **0.1.1 delivered scope:** **accepted complete rulebook and full API/ABI/engine implementation specification**, example corpus, error taxonomy, runtime integration plan and deferred test matrix. **No** claim that regex runtime or contract matching is enabled; that separate implementation gate is provisionally `0.1.8` (§3).
- **Release evidence:** normative owner sign-off on complete rulebook and downstream ABI/engine contract; static/negative documentation examples and explicit delayed runtime coverage; retain existing MD-010 closure rules until complete integration.

### § 19.3 `SEC-0.1.1-03` — Heterogeneous variadic type packs (MD-117)

- **Normative state:** `rules/declarations/functions.md` §38A defines the feature for **Sec 0.1**. Several parser/Sema/specialization/forwarding slices already exist. Current governance records remaining lowering, element-loop, reference-holder, method and interface-related boundaries; inspect their exact state before implementation.
- **Critical questions:** Is each remaining restriction a real semantic necessity or an implementation limitation? Are `[...Types]`, positional type identity, owned/borrowed packs, `Len`, compile-time indexing, `for` and forwarding consistent with ordinary generics? Can every position be compiled without `any`, implicit boxing, invisible copies, nested `ref ref` or leaking a pack? Are diagnostics intuitive for empty/mixed packs and rejected runtime-length spreads?
- **Release evidence:** all accepted source forms from §38A work across parser, Sema, specialization, Semantic IR, MLIR/backend and native/declared target matrix; ownership/transfer/cleanup, overload and generics, LSP/formatter, negative cases and mixed-feature regression pass. Any genuinely unsupported accepted form remains a release blocker or is explicitly revised by the owner, never silently downgraded.

### § 19.4 `SEC-0.1.1-04` — Generic error channels, enums/unions and interfaces (MD-120)

- **Normative state:** approved 2026-10-31, normatively synchronized 2026-11-01, **Sec 0.1** semantics; implementation verification was still pending in the approved MD-120 record. Some Sema/governance slices are implemented. Verify actual completion by stage, not by the correction's `APPLIED` label.
- **Representative valid syntax:**

```sec
interface FormatSink[E: error] {
    mut fn WriteText(text: string) Result[void, E]
}
```

- **Critical questions:** Does generic `E` preserve its exact concrete error identity through interface specialization, `try`, wrappers and borrowed views? Are `FormatSink[IOError]` and `FormatSink[error]` correctly distinct? Do `Err(_)` discardability, unknown `E` variants, mixed error channels, generic enum/union payload ownership and inference behave without implicit variance/erasure? Are core `format` and stdlib `fmt` integrated **without** losing independent format errors (MD-118)?
- **Release evidence:** complete normative-positive and negative MD-120 conformance matrix in frontend and backend; distinct specialized type and ABI identities; resolved interface dispatch, try/widening and generic wrapper cleanup; actual core/stdlib formatting integration as specified or an explicitly owner-approved release split that does not falsify generic error support.

### § 19.5 Cross-stream closure

1. Produce the **separate** version document containing four requirement IDs, full subitem dependency/task lists, exact governance IDs, supported target matrix and release proof links.
2. Confirm MD-117 and MD-120 remain *language 0.1 normative decisions*, even though their **complete implementation and verification** are release obligations for `0.1.1`.
3. `Owner[T]` is an executable deliverable only once its exact public semantics and backend/storage implementation are approved and verified; partial ownership without a defined construction/discharge protocol is not releasable.
4. Regex is intentionally **specification-only** in `0.1.1`; the runtime/contract feature stays unavailable and retains S1058 until its separately assigned implementation version.
5. Challenge each stream for usability and accidental new complexity. Compare before/after valid Sec code, negative diagnostics, expected effects and interactions; avoid unnecessary source annotations and APIs.
6. `0.1.1` must satisfy §§17–18 and its own version document's owner-approved gate. No requirement is marked complete merely because a rulebook exists, a test passes in isolation or a future implementation task has been described.

