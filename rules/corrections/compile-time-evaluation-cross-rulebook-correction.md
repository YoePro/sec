# Compile-Time Evaluation Cross-Rulebook Correction

- Status: Normative correction and synchronization package
- Created: 2026-09-06
- Last updated: 2026-09-06
- Sec language version: 0.1
- Repository baseline reviewed: `0f5027d`
- Primary rulebook: `rules/compiler/compile_time_evaluation.md`
- Intended correction area: `rules/corrections/`
- Companion correction: `rules/corrections/compiler-known-fundamentals-cross-rulebook-correction.md`
- Classification: Required synchronization after the canonical CTE rulebook.

---

## § 1 Purpose

§ 1(1) This correction synchronizes rulebooks that currently use broader or older notions such as:

```text
compile-time constant;
constant expression;
known at compile time;
constant evaluator.
```

§ 1(2) It does not require every statically provable fact to become semantic CTE.

§ 1(3) The durable authority after synchronization remains with each owning rulebook plus `rules/compiler/compile_time_evaluation.md`.

---

## § 2 Required context terminology

§ 2(1) Introduce the compiler-semantic terms:

```text
PlanTimeRequiredContext
SemanticCompileTimeRequiredContext
```

as defined by `compile_time_evaluation.md`.

§ 2(2) These are not Sec declarations.

§ 2(3) A rulebook that requires expression evaluation before runtime lowering must identify the correct context class rather than relying on an ambiguous phrase such as `must be constant`.

§ 2(4) Static proof, range analysis, constant propagation, or optimizer knowledge does not automatically create a required context.

---

## § 3 `rules/foundations/attributes.md`

§ 3(1) Classify initial attribute expressions that must be resolved before the active semantic graph is established as `PlanTimeRequiredContext`.

§ 3(2) Preserve the existing restricted expression contract for plan-time attributes.

§ 3(3) Do not permit ordinary user-function semantic CTE in `@when` merely because the same function could execute later during semantic CTE.

§ 3(4) Preserve the current phase ordering:

```text
CompilationRequest
    -> plan-time attribute/selection evaluation
    -> active source graph
    -> ordinary typed Sema
```

§ 3(5) Attribute-specific validation that occurs later may consume the already-established plan-time value without changing the initial context class.

---

## § 4 `rules/declarations/static.md`

§ 4(1) Where the static rule requires an initializer to be established during compilation, classify the initializer as `SemanticCompileTimeRequiredContext`.

§ 4(2) Permit ordinary user-defined CTE according to `compile_time_evaluation.md`.

§ 4(3) Preserve the distinction:

```text
compile-time initializer value
!=
mutable runtime static storage
```

§ 4(4) A failed required initializer must not be rewritten as hidden runtime initialization.

§ 4(5) A successfully materialized static value receives ordinary program storage identity rather than evaluator-local identity.

---

## § 5 `rules/compiler/initialization.md`

§ 5(1) Synchronize initialization terminology with semantic CTE.

§ 5(2) Static values whose owning declaration requires compile-time establishment are resolved before startup/runtime initialization.

§ 5(3) The artifact encodes the materialized result according to ordinary storage/layout lowering.

§ 5(4) Required CTE failure cannot be repaired by inserting an undeclared runtime startup action.

---

## § 6 Arrays and collection extents

§ 6(1) In the owning array/collection rule, every fixed extent that accepts an expression and is required before type formation must be classified as `SemanticCompileTimeRequiredContext`.

§ 6(2) The owning rule remains authoritative for:

```text
which source expression forms are allowed;
required integer type/category;
valid extent domain;
zero/negative/overflow behavior.
```

§ 6(3) CTE success with a value outside the legal extent domain is a collection/type validation error, not evaluator failure.

§ 6(4) Dynamic collection values used transiently during CTE do not thereby gain static materialization.

---

## § 7 `rules/collections/shaped-types.md`

§ 7(1) Replace ambiguous uses of `compile-time-known` with the correct category where evaluation is actually required.

§ 7(2) Static shaped extents/rank values that are source expressions required to form a shaped type are `SemanticCompileTimeRequiredContext`.

§ 7(3) Compiler-known shaped properties whose values are statically known do not automatically make every source read of that property a required CTE context.

§ 7(4) Preserve the owning shaped semantics for:

```text
.Rank
.Shape
.Len
.Strides
.Layout
.MemorySpace
.IsContiguous
.Ptr
.SizeOf
```

without treating those names as a generic reflection API.

§ 7(5) The companion compiler-known-fundamentals correction governs declaration completeness for compiler-known source members/types referenced by shaped rules.

---

## § 8 `rules/declarations/registers.md`

§ 8(1) Register width and bit-field values that must be established before register type formation are `SemanticCompileTimeRequiredContext` where the grammar permits expressions.

§ 8(2) The register rule owns legal width/range/domain restrictions.

§ 8(3) A successful CTE value outside the legal register domain is a register semantic error, not CTE evaluator failure.

§ 8(4) CTE must not read target register/MMIO state.

---

## § 9 `rules/platform/fixed-address-bindings.md`

§ 9(1) Distinguish:

```text
compile-time target address metadata;
CTE-local evaluator address identity;
actual target memory access.
```

§ 9(2) Target/platform address metadata may be a compile-time input where the owning address/attribute rule permits it.

§ 9(3) CTE-local object addresses must not escape into fixed-address runtime bindings.

§ 9(4) Semantic CTE must not dereference target hardware memory.

---

## § 10 Types and contracts

§ 10(1) Do not change ordinary local contract checks into required CTE merely because Sema can prove a value statically.

§ 10(2) Where a conversion/value is statically established, the contract rule may diagnose a violation at compile time using canonical Sema/static-analysis facts.

§ 10(3) CTE is one possible source of a statically established typed value, not the only source.

§ 10(4) Preserve runtime contract semantics where the value is not required or proven at compile time.

---

## § 11 Units

§ 11(1) Unit conversion or unit-expression facts that Sema proves statically do not automatically become required CTE contexts.

§ 11(2) A unit-related source position that explicitly requires a compile-time scalar must identify that requirement in the owning unit/type rule.

§ 11(3) CTE must preserve exact unit and nominal type semantics; it must not evaluate only the underlying carrier representation.

---

## § 12 Enums

§ 12(1) Enum explicit values required by the enum declaration semantics are semantic compile-time values.

§ 12(2) The enum rule owns:

```text
whether arbitrary semantic CTE expressions are legal there;
the backing-compatible result requirement;
duplicate-value/alias rules;
range restrictions.
```

§ 12(3) Generic enum concrete identity remains owned by generics/generic-lowering/monomorphization rules.

§ 12(4) CTE does not create generic enum identity.

---

## § 13 Generic constraints

§ 13(1) Generic constraint/conformance proof is compile-time Sema but is not a `SemanticCompileTimeRequiredContext` merely because it occurs during compilation.

§ 13(2) Remove or correct wording that treats ordinary user-defined compile-time execution as excluded from Sec generics generally.

§ 13(3) Preserve the actual exclusion:

```text
generic declaration generation or rewriting through CTE;
general structural compiler reflection through CTE;
runtime generic type machinery.
```

§ 13(4) Ordinary concrete generic functions may execute during semantic CTE after the concrete specialization contract is satisfied.

---

## § 14 `rules/compiler/generics_lowering.md`

§ 14(1) Add `compile_time_evaluation.md` as authority for execution of concrete generic callables during CTE.

§ 14(2) Preserve generic lowering's distinction between semantic concretization and optimization.

§ 14(3) CTE may consume concrete generic specialization semantics but does not own specialization discovery, identity allocation, worklists, deduplication, or physical emission.

---

## § 15 `rules/compiler/monomorphization.md`

§ 15(1) CTE requests for a concrete generic callable participate in the canonical specialization-demand model rather than creating a separate CTE specialization identity.

§ 15(2) The same canonical concrete specialization identity is used for runtime and CTE semantic uses.

§ 15(3) Monomorphization remains authoritative for specialization demand, recursion/resource policy specific to specialization, caching, ownership, and physical realization.

§ 15(4) CTE remains authoritative for execution resource budgets and evaluation failure.

---

## § 16 Copy/move

§ 16(1) Remove or correct any rule implying that a compile-time value is automatically copyable.

§ 16(2) Replace it with:

> A compile-time value follows the ordinary copy/move semantics of its concrete Sec type.

§ 16(3) CTE may hold move-only transient values.

§ 16(4) Materialization eligibility is separate from copyability.

---

## § 17 Destruction

§ 17(1) Refine statements such as `compile-time constants require no runtime destruction`.

§ 17(2) Distinguish:

```text
CTE-local transient value:
    ordinary destruction occurs during CTE where semantically reached;
    evaluator storage reclamation is compiler-internal.

materialized static/runtime value:
    ordinary storage/type destruction rules apply to the materialized program object.
```

§ 17(3) The fact that a semantic value was computed at compile time does not itself redefine the runtime destruction category of a separately materialized program object.

---

## § 18 Borrowing, references, and raw pointers

§ 18(1) Cross-reference CTE-local abstract reference identity.

§ 18(2) Preserve ordinary Sec borrowing/lifetime legality during CTE.

§ 18(3) Explicitly forbid transient evaluator address/generation identity from escaping materialization.

§ 18(4) Distinguish target address metadata from evaluator-local addresses.

§ 18(5) Do not infer runtime generational-reference state from evaluator-internal liveness IDs.

---

## § 19 Effect analysis

§ 19(1) `effect_analysis.md` remains authoritative for canonical effect facts.

§ 19(2) `compile_time_evaluation.md` owns whether an actually executed effect is allowed in semantic CTE.

§ 19(3) Do not create a separate CTE effect taxonomy that competes with ordinary Sec effect analysis.

§ 19(4) Preserve path-sensitive CTE execution: a forbidden effect on an unexecuted branch does not by itself invalidate the concrete evaluation.

---

## § 20 Panic and runtime checks

§ 20(1) A panic path reached during required CTE becomes compile-time evaluation failure with source provenance.

§ 20(2) `assert` failure reached during required CTE likewise becomes evaluation failure.

§ 20(3) A normal `Result.Err(...)` remains a successfully evaluated program value.

§ 20(4) Language-defined operation failures reached during CTE are classified under `SemanticOperationFailed`, while the owning runtime-check/numeric/indexing/contract rule owns the exact semantic reason.

---

## § 21 Modules

§ 21(1) Public source-accessible functions may be CTE-executed cross-module.

§ 21(2) ModuleSurface/separate-compilation metadata may carry semantic bodies/dependencies sufficient for CTE without exposing private source names.

§ 21(3) Compiler semantic availability does not bypass source visibility.

§ 21(4) `_name` and `__name` retain their ordinary source visibility semantics.

§ 21(5) An origin body may execute its already-resolved private/sourcefile-only helper calls through compiler semantic metadata.

§ 21(6) Imported CTE execution must use origin declaration identities and compatible semantic artifact fingerprints.

---

## § 22 `rules/compiler/semantic_ir.md`

§ 22(1) Add semantic CTE as a consumer of canonical typed Semantic IR.

§ 22(2) Semantic IR consumed by CTE must preserve:

```text
typed operations;
resolved callable/member identities;
control flow;
ownership/borrow legality;
effects;
concrete generic identity where required;
source provenance.
```

§ 22(3) CTE must not redo source Sema from raw AST.

§ 22(4) Imported semantic bodies used as CTE proof require compatibility validation.

---

## § 23 `rules/compiler/compiler_pipeline.md`

§ 23(1) Represent CTE as a semantic service around the semantic-to-lowering pipeline rather than as one monolithic phase after all Sema.

§ 23(2) Conceptually:

```text
CompilationPlan
    -> plan-time evaluation
    -> active source graph
    -> typed semantic analysis
        <-> semantic CTE as required
        <-> generic/type/layout/target semantic services
    -> verified Semantic IR
    -> Sec-MLIR / MLIR
    -> LLVM
    -> artifact
```

§ 23(3) A semantic construct may require CTE before the rest of the program's Semantic IR is complete.

§ 23(4) CTE may consume a later-owned semantic fact only when that fact is available without creating a phase cycle.

§ 23(5) Required CTE cannot fall back to MLIR/LLVM optional folding.

---

## § 24 Sec-MLIR and MLIR lowering

§ 24(1) Required semantic CTE values needed by executable lowering must be established before the dependent operation is emitted in unresolved form.

§ 24(2) Sec-MLIR/MLIR constant folding is optional optimization.

§ 24(3) MLIR infrastructure may be reused internally for CTE if it preserves canonical typed Sec semantics and target-plan correctness.

§ 24(4) CTE must not execute compiler-host ABI semantics as a substitute for cross-target Sec semantics.

---

## § 25 Diagnostics and LSP

§ 25(1) Diagnostics must distinguish:

```text
NotEvaluable;
EvaluationFailed;
ResourceLimit;
DependencyCycle;
ResultNotMaterializable;
ImplementationGap;
CompilerInvariantFailure;
Cancelled.
```

§ 25(2) These are compiler-semantic categories/labels, not Sec error declarations.

§ 25(3) User-facing diagnostics should explain the required context and root cause instead of merely saying `not a constant expression`.

§ 25(4) LSP consumes compiler CTE results/provenance and must not implement a separate evaluator.

§ 25(5) Cancellation of obsolete LSP evaluation is not a source diagnostic.

---

## § 26 `language-rulebook-status.md`

§ 26(1) Change:

```text
compile_time_evaluation.md | Planned
```

to `Written`.

§ 26(2) Remove `compile_time_evaluation.md` from the planned canonical-rulebook set.

§ 26(3) Remove from `Still to decide`:

```text
user-defined compile-time execution;
allocation and I/O restrictions;
compile-time panic;
recursion and loop limits.
```

§ 26(4) Replace the blanket exclusion:

```text
compile-time reflection
```

with a narrower exclusion equivalent to:

```text
general-purpose structural compile-time reflection and declaration metaprogramming
```

§ 26(5) The narrower wording must not prohibit individually designed compiler-known properties such as layout or shaped facts.

§ 26(6) `TypeOf` is not introduced by this correction.

---

## § 27 Legacy constant-evaluator planning

§ 27(1) Any legacy implementation plan describing one shared literal/operator `constant evaluator` remains directionally useful but is superseded as the complete semantic design.

§ 27(2) Simple constant-expression evaluation is a subset of the canonical semantic CTE service.

§ 27(3) The complete implementation must additionally cover according to the status ledger:

```text
typed user functions;
control flow;
loops;
recursion;
transient allocation;
ownership/borrowing/destruction;
cross-module execution;
generic concrete calls;
dependency tracking;
resource limits;
materialization;
diagnostics;
incremental caching/tooling.
```

---

## § 28 Implementation status

§ 28(1) Merge the entry from `implementation-status-compile-time-evaluation.yaml`.

§ 28(2) Do not mark CTE globally complete merely because literals, enum constants, array extents, or local constant folding already exist.

§ 28(3) Distinguish language validity from current evaluator coverage.

---

## § 29 Superseding invariant

§ 29(1) The canonical synchronization rule is:

> A language construct that requires expression evaluation before runtime lowering must state whether it is plan-time-required or semantic-CTE-required. Static proof and optional optimizer folding remain separate compiler activities.

§ 29(2) Required semantic CTE follows the canonical typed Sec semantics defined by `rules/compiler/compile_time_evaluation.md`.
