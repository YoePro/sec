# Sec <VERSION> — Version Definition, Specification, Implementation and Acceptance

**Document ID:** `SEC-VERSION-<VERSION>`  
**Document kind:** Version-specific planning and delivery contract; **not itself a normative language rulebook**  
**Sec language version:** `<VERSION>`  
**Created:** `<YYYY-MM-DD>`  
**Updated:** `<YYYY-MM-DD>`  
**Document revision:** `0.1`  
**Status:** `Draft — critical review pending`  
**Owner/decision authority:** Sec language owner  
**Repository baseline:** `YoePro/sec`, `main` HEAD `<FULL_SHA>` (verified `<DATE>`)  
**Roadmap:** `sec-release-staging-0.1x-to-0.2-detailed-rev04-2026-11-02.md` §§17–19  
**Normative rulebooks:** `<exact paths and stable paragraph IDs>`  
**Existing MD/correction identifiers:** `<IDs or none>`  
**Governance integrations:** `<existing IDs; never create duplicates>`  
**Release-channel target:** `develop → test → rc → final` (subject to tool support)  
**Revision discipline:** Preserve published paragraph and requirement IDs. Add new numbered subsections/corrections; never silently reuse retired IDs.

> **Template policy:** Every `<placeholder>` is **intentionally unresolved**, not permission for a compiler or AI to guess its value. Remove placeholders before approval. All code examples, APIs, identifiers, error variants, ABIs and target claims must be drawn from accepted rulebooks or approved decisions. New compiler code and Sec source code must be written in English.
>
> **Completion rule:** A specification-only deliverable may close its own documented design task without claiming executable functionality. Every executable feature promised by this release must have compiler/native and conformance evidence on its declared support matrix before the release itself closes.

---

## § 1. Release purpose and boundary

**§ 1(1) Problem solved.** `<Define the actual programmer-facing problems, not a list of attractive syntax.>`

**§ 1(2) Promised release behavior.** `<Exactly what works when the version is marked complete.>`

**§ 1(3) Explicit exclusions.** `<Future work, unimplemented APIs, unsupported targets and specification-only work clearly separated.>`

**§ 1(4) Relationship to existing normative Sec version.** `<Distinguish a new source rule from implementation/verification of an already approved 0.1 rule.>`

**§ 1(5) Dependencies.** `<Earlier version guarantees, compiler packages, ABI/layout/CTE, core/stdlib interfaces.>`

## § 2. Delivery inventory with stable requirement IDs

| Requirement ID | Workstream | Source idea or MD | Nature of scope | Normative status | Implementation status | Existing governance owner | Release criterion |
|---|---|---|---|---|---|---|---|
| `SEC-<VERSION>-01` | `<name>` | `<reference>` | `<new language / 0.1 implementation / book only / tooling / library>` | `<actual>` | `<actual>` | `<ID>` | `<observable outcome>` |

**§ 2(1) Inventory rules.** Reconcile each requirement with current `rules/`, corrections, `missing-decisions.yaml`, `governance/` and implementation tests before altering its status. Stable requirement IDs are document pointers, not parallel governance truth.

**§ 2(2) Missing-decisions intake.** `<Justification and owner approval for every added substantial MD; record alternatives and dependent rulebooks.>`

## § 3. Critical review before new design approval

Repeat this analysis **for each genuinely new feature**, not just once for the release.

### § 3.1 Current source language and real problems

**§ 3.1(1)** `<Current valid Sec solution, source reference, and what goes wrong or becomes awkward.>`

**§ 3.1(2)** `<Real example from core/stdlib/application or well-evidenced representative use; expected behavior and compile/execution outcome.>`

### § 3.2 Alternatives and strongest objections

**§ 3.2(1)** `<Best argument against adding this feature.>`

**§ 3.2(2)** `<Do nothing / use existing language / stdlib / compiler optimization / tooling / smaller syntax change: compare.>`

**§ 3.2(3)** `<Why the added implementation and teaching complexity is warranted, if accepted.>`

### § 3.3 Ergonomics, intuition and complexity

**§ 3.3(1)** `<Before/after idiomatic code examples; prefer intent without semantic bureaucracy.>`

**§ 3.3(2)** `<Predictability of ownership, effects, failure, evaluation, allocation, performance and error messages.>`

**§ 3.3(3)** `<Consistency with similar Sec forms; formatter, IDE, generic and exception cases.>`

### § 3.4 Owner decision

**§ 3.4(1)** `<Accepted / implementation only / tooling-library / needs design / deferred / rejected / already covered; rationale; decision identifier.>`

## § 4. Normative rulebook work

| Book | New/revision | Exact topic/paragraphs | Source API ownership | ABI ownership | Approval/implementation dependencies |
|---|---|---|---|---|---|
| `<path>` | `<NEW / REV>` | `<stable IDs>` | `<exact public declarations>` | `<ABI or no-new-ABI rationale>` | `<IDs>` |

**§ 4(1)** Each new language area with independent semantics normally has its own canonical book. Small changes revise existing owners. A version document references APIs from authoritative books; it never independently invents incompatible duplicate declarations.

**§ 4(2)** Books must have full metadata, stable numbered paragraphs, exact public declarations and member-level documentation directly before declarations, constructors using accepted `init(...)` syntax, complete error/type enums and no unresolved pseudo APIs. Open choices remain named and block acceptance.

**§ 4(3)** Document grammar, type identity, generic constraints, attributes, methods/properties, ownership/borrowing, failure, panic, destruction, effects, scheduling/cancellation, CTE, runtime boundaries and observable evaluation order.

## § 5. ABI, representation and target requirements

**§ 5(1) ABI classification.** `<Native Sec ABI layout and calling/return semantics, or precise no-new-ABI reference with rationale.>`

**§ 5(2) FFI and system ABI.** `<Allowed/forbidden foreign use, adapters, C ABI compatibility or explicitly unsupported.>`

**§ 5(3) Lifetime/identity.** `<Ownership, provenance, storage producer, destination construction, cleanup and invalidation.>`

**§ 5(4) Platform matrix.** `<Target and architecture list, capabilities, supported and unsupported combinations; explicit diagnostics.>`

**§ 5(5) Reproducible artifacts.** `<Schema, CompilationPlan, ABI fingerprints, symbols, debug information, incremental compatibility.>`

## § 6. Compiler implementation handoff

| Layer | Exact work | Existing owner/API/plan | Verification requirement |
|---|---|---|---|
| Lexer/parser/CST | `<...>` | `<...>` | `<...>` |
| Type checking/Sema | `<...>` | `<...>` | `<...>` |
| CTE/defaults/contracts | `<...>` | `<...>` | `<...>` |
| Ownership, effects and provenance | `<...>` | `<...>` | `<...>` |
| Semantic IR | `<...>` | `<...>` | `<...>` |
| Sec MLIR dialect and verifier | `<...>` | `<...>` | `<...>` |
| Lowering/LLVM/native target | `<...>` | `<...>` | `<...>` |
| Core/stdlib/runtime | `<...>` | `<...>` | `<...>` |
| ABI/layout/FFI/debug | `<...>` | `<...>` | `<...>` |
| Formatter/LSP/CLI/manual | `<...>` | `<...>` | `<...>` |

**§ 6(1)** Implementation is divided into small dependency-ordered tasks. Work one task at a time and test that task before moving on. The compiler does not re-decide source semantics in IR/lowering. Missing downstream infrastructure is a named blocker, not a claim of partial executable support.

## § 7. Conformance and invalid-case matrix

| Test ID | Domain | Source or fixture | Expected result/diagnostic | Target/phase | Evidence |
|---|---|---|---|---|---|
| `SEC-<VERSION>-T01` | Valid source | `<...>` | `<exact>` | `<...>` | `<...>` |
| `SEC-<VERSION>-T02` | Invalid source | `<...>` | `<exact diagnostic>` | `<...>` | `<...>` |

Cover real source and full integration: parser recovery, Sema, CTE, ownership, borrowing, evaluation order, errors, generics, ABI/FFI, MLIR verifier, native runtime, cross-target negative tests, LSP/formatter, incremental recompilation, regression, fuzz/property where applicable, usability/dogfooding. Include cross-feature tests, not only isolated happy paths.

## § 8. Granular work and dependency ledger

| Work ID | Prerequisite | One scoped task | Files/owning component | Test immediately after | Owner/MD | Status/evidence |
|---|---|---|---|---|---|---|
| `SEC-<VERSION>-W01` | `<...>` | `<...>` | `<...>` | `<...>` | `<...>` | `<...>` |

**§ 8(1)** An implementation AI must not implement several untested tasks as one opaque change. When public API/ABI semantics are missing, record the owner decision point first.

**§ 8(2)** Release readiness is not dependent on coding work occurring in numeric version order. It is dependent on resolved contracts and complete tests.

## § 9. Version readiness gates and known exclusions

| Gate | Completion requirement | Status | Evidence/reference |
|---|---|---|---|
| `V0` Inventory | All accepted and candidate items identified | `<status>` | `<source>` |
| `V1` Critical review | Alternatives/objections/UX and interactions recorded | `<status>` | `<source>` |
| `V2` Normative complete | Owner-approved exact rulebooks and API/ABI | `<status>` | `<source>` |
| `V3` Implementation plan | Granular tasks and conformance matrix | `<status>` | `<source>` |
| `V4` Implementation | Promised public compiler/native paths complete | `<status>` | `<source>` |
| `V5` Verified | Test, target, regression and ergonomic evidence | `<status>` | `<source>` |
| `V6` Release ready | Docs, governance, MD and owner sign-off | `<status>` | `<source>` |

**§ 9(1)** `<Known unsupported features and targets, explicitly excluded by version agreement; no hidden gaps.>`

**§ 9(2)** `<Spec-only deliverables with later execution gate; do not mislabel them as implemented.>`

**§ 9(3)** `<Latest test results with commands, exact compiler commit and release candidate identity.>`

## § 10. Final owner acceptance and sync record

**§ 10(1) Source semantics approved:** `<date, exact decisions and stable IDs>`

**§ 10(2) Rulebooks synchronized:** `<files/revisions>`

**§ 10(3) Implementation and tests verified:** `<commands, targets and evidence>`

**§ 10(4) Existing governance and MD ledger updated:** `<IDs and status changes, no duplicates>`

**§ 10(5) Ergonomic/intuitive acceptance:** `<dogfooding examples, changes made, open exclusions>`

**§ 10(6) Channel/tag and release metadata:** `<dev/test/rc/final evidence>`

**§ 10(7) Owner final approval:** `<pending/approved with date>`

---

## Appendix A. Template usage guardrails

- This template is **not** an accepted feature, a normative API, an ABI promise, or evidence of compiler support.
- Preserve previous document revisions and section identity; amendments are reviewed and linked.
- Reuse the exact current names and syntax in authoritative Sec rules; do not copy from proposals as though already accepted.
- A single major feature can have its own detailed normative book and test specification; the version document must still enumerate and gate it.
- Serious issues discovered in an existing feature can be assigned to this release only after owner approval and reconciliation with the existing MD/governance owner. Trivial bug fixes continue in their normal audit/implementation process.
- The acceptance objective is usable, coherent, complete language behavior—not minimum feature count or maximum feature count.
