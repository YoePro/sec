# Sec Language Rulebook Status

## Purpose

This document is the canonical inventory of the rulebooks expected for the Sec
language, compiler, target model, core library, and standard library.

It replaces the former temporary `temp2.txt` checklist.

This document tracks documentation status only.

The physical rulebook layout and directory responsibilities are indexed by
`rules/README.md`. Paths in this inventory are relative to `rules/` unless a
repository-root path is written explicitly.

It does not claim that a written rulebook has been implemented by the compiler.

Implementation progress is governed by the split ledger under:

```text
governance/
```

`implementation-status.yaml` is retained as a migration source for entries that
have not yet moved into a governance fragment.

Rulebooks contain normative requirements and must not duplicate the current
repository implementation state. Existing status sections are migrated one
rulebook at a time into the implementation ledger.

---

# Status definitions

| Status | Meaning |
|---|---|
| **Written** | A rulebook exists and is part of the current rule set. |
| **Written — sync required** | A rulebook exists, but newer decisions must be merged into it or into related rulebooks. |
| **Written — repository sync pending** | The rulebook has been written after the latest repository version reviewed by this document and must be added to `rules/`. |
| **Living** | The document exists and is intentionally updated as implementation progresses. |
| **Planned** | The rulebook is expected but has not yet been written. |
| **Deferred** | The topic is intentionally outside the immediate Sec 0.1 closure work. |
| **Covered** | The topic is covered by another canonical rulebook and should not have a duplicate document. |
| **Candidate** | The need or final filename has not yet been locked. |

A rulebook may be written while the corresponding feature remains entirely
unimplemented.

---

# Repository baseline

The current repository already contains the broad language, ownership,
concurrency, compiler, MLIR, diagnostics, and project rule sets.

The following newer rulebooks have been added to the canonical inventory:

```text
collections/collections.md
collections/shaped-types.md
concurrency/ipc.md
concurrency/thread_local.md
control-flow/discard.md
declarations/generics.md
compiler/generics_lowering.md
compiler/monomorphization.md
compiler/incremental_compilation.md
declarations/static.md
memory/ownership.md
memory/borrowing.md
tooling/lsp.md
tooling/formatter.md
memory/copy_move.md
memory/destruction.md
memory/memory_model.md
foundations/operators.md
types/default_values.md
types/contracts.md
foundations/grammar.md
foundations/attributes.md
errors/panic.md
errors/runtime_checks.md
analysis/effect_analysis.md
memory/unsafe.md
memory/reference_model.md
analysis/call_graph.md
memory/arena.md
memory/storage.md
memory/layout.md
compiler/compiler_known_members.md
analysis/escape_analysis.md
analysis/closure_analysis.md
analysis/parameter_usage_analysis.md
analysis/pitfall_analysis.md
analysis/stack_analysis.md
analysis/data_races.md
analysis/deadlock_analysis.md
analysis/isr_analysis.md
platform/volatile.md
platform/hardware-register-access.md
platform/interrupts.md
platform/inline_assembly.md
compiler/compiler.md
compiler/compiler_analysis.md
compiler/compiler_pipeline.md
compiler/semantic_ir.md
```

These were written after the older temporary checklist was last synchronized.

---

# 1. Language foundations and lexical rules

| Rulebook | Status | Notes |
|---|---|---|
| `foundations/language_philosophy.md` | **Written — revision 2.0** | Normative design principles in numbered paragraphs (§§ 1–23), including compiler-as-mentor diagnostics, prove/check/reject, unsafe as a transferred proof obligation, backend independence, and decision discipline. It guides new decisions and the interpretation of genuinely unspecified areas but never overrides or invents semantics owned by a more specific rulebook (§ 1(7)–(8)); it has no implementation-governance work items. |
| `foundations/lexical_structure.md` | **Written** | Canonical lexical rules; Go frontend NFC identifier validation and balanced interpolation-expression lexing are implemented. Interpolation AST parts, expression parsing, source ranges, text escape materialization, ordinary semantic traversal, formatting-contract selection, maximal frontend concat planning, and the allocation/failure policy (try with StringError, S1097/S1098) are implemented; bootstrap parsing remains pending, and Semantic IR/backend lowering is tracked by MD-004 lowering in governance/sema.yaml. Implementation status is tracked by `frontend.lexical-structure` in `governance/lexer.yaml`. Lexer escape validation is implemented with L1006-L1008 diagnostics and preserved source spans; character scalar-count validation uses L1009, malformed base/separator diagnostics use L1010-L1012, invalid numeric suffixes use L1013, missing exponent digits use L1014, focused unterminated-token diagnostics use L1015-L1019, and otherwise invalid source characters including embedded U+0000 token starts use L1020 with one-scalar recovery rather than EOF truncation. All nineteen required lexical error categories and their twenty stable diagnostic distinctions now have exhaustive coverage, including maximal non-base decimal recovery across integer, fractional, leading-period, and exponent forms. Documentation block comments now attach to source-ordered declaration nodes in the parser AST, including nested member and local-declaration contexts; ordinary formatter comment attachment remains partial. The Go-frontend-supported contextual contract spellings and seven-name lowercase collection/shaped type inventory are centralized across the lexer and their parser/tooling consumers while retaining IDENT tokenization. Collection/shaped names receive parser-positioned LSP type classes and generic-lookahead TextMate highlighting, preserving ordinary contextual `set` identifiers. Reserved declaration names are centralized across the Go lexer, Sema, diagnostics, and LSP with mandatory S1028 coverage across declaration namespaces and explicit contextual exceptions; the three new length words remain to be mirrored into the separate bootstrap project. LSP, formatter, and TextMate coverage now distinguishes the full lexical tooling inventory, including parser-owned contextual operators/modifiers, documentation comments, attributes, bare underscore, raw declaration metadata, and visibility-prefixed identifiers. Confusable-identifier detection (UTS #39, non-suppressible error within a conflicting namespace and scope, unit symbols excluded) and fallible runtime interpolation are now normative via `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md`; their implementation is pending. MD-001 (2026-10-03): UTS #39 confusable detection is implemented with Unicode 15.0.0 data and mandatory S1099, covering every conflicting declaration domain (module, local, parameter, generic parameter, struct field, enum/union variant, type member, and interface requirement); MD-005: L1021 reports an unmatched `}` in interpolated text with in-string recovery; MD-009: `regex` joins the reserved contract-word inventory. MD-043 (2026-10-08, `rules/corrections/applied/md043-char-rune-literal-correction-20261008.md`): § 12.7 validates `t` literals against `0..255` separately from `r` Unicode-scalar validation, and § 13 makes a character literal `rune` by default; the frontend implements both, rejecting out-of-range values with S1138. MD-020/MD-024 (2026-10-09, `rules/corrections/applied/md020-md021-md023-md024-ffi-c-bindings-null-correction-20261008.md`): `::` is one longest-match token without surrounding whitespace and `null` is reserved; the lexer still produces two colon tokens and treats `null` contextually. |
| `types/types.md` | **Written** | Canonical replacement for the retired `types.txt`; implementation is tracked by `frontend.types-core`, `frontend.literal-family-suffix-v2`, `frontend.temporal-builtin-types`, `frontend.temporal-duration`, `frontend.temporal-instant`, `frontend.wide-numeric-language-types`, and `frontend.target-sized-integer-semantics` in `governance/types.yaml`. MD-014 (2026-10-03): plain `float` is platform-sized like `int`/`uint` (float32 on 32-bit, float64 on 64-bit) and implemented in Sema and Semantic IR. Compiler and LSP now share the canonical target registry, and LSP analysis uses a selected project variant's scalar plan. Active target switching and distinct-plan output-variant diagnostics are governed by `frontend.target-sized-integer-semantics` and `tooling.lsp-multi-target-diagnostics`; the implementation snapshot under lsp.md Multi-target diagnostics was removed on 2026-10-08 so that progress is owned by governance. MD-012 conversion failure layers are documented. On 2026-10-08, legacy direct LLVM explicitly rejects platform-sized float before output when selected scalar facts cannot be consumed; fixed-width float signatures remain distinct, and compiler JSON includes the lowering limitation for emit and build. On 2026-10-08, selected legacy wide numeric paths retain exact supported carriers and reject unavailable checked arithmetic and decimal128 capabilities with source-backed unsupported-lowering; canonical Semantic IR/Sec MLIR support remains tracked separately. MD-043 (2026-10-08, `rules/corrections/applied/md043-char-rune-literal-correction-20261008.md`): `byte` and `char` are distinct 8-bit types over `0..255`, single-quoted literals default to `rune`, and unsuffixed integers never shape to `char` (implemented in the frontend with S1138; Semantic IR still lacks a char/rune comparison operation); MD-012 (`rules/corrections/applied/md012-md022-conversion-errors-ffi-literals-correction-20261008.md`): runtime checked conversions report the core `ConversionError`. Frontend typing of `ConversionError` is pending. |
| `types/temporal.md` | **Written — revision 1.1** | Canonical temporal representation, trusted-core boundary, and UTC wall-clock API for `datetime.Now`, `date.Today`, and `time.Now` through private core-only `_now: datetime`; backing structs, core properties, core-only Sema resolution, stable intrinsic/effect facts, static-initializer rejection, and visibility-aware LSP support are implemented, while `UTCWallClock` plan validation, broader compile-time-context integration, IR, and lowering remain. Implementation status is tracked by `frontend.temporal-builtin-types` in `governance/types.yaml`. |
| `types/contracts.md` | **Written — revision 2.1** | Canonical named-type contracts; replaces the obsolete variable-contract model. The frontend now parses and represents `minLen`, `maxLen`, and `exactLen`, validates their nonnegative compile-time values and applicability, proves inherited/local consistency with `notEmpty` and fixed-array extents, and checks explicit string defaults in the canonical length units. Compile-time string literals are also proved against inherited membership, `notEmpty`, and length contracts at storage, aggregate, conversion, call, and return boundaries; selected call arguments and returns share the existing integer-contract proof. Runtime string-length validation on valid UTF-8 is supported by native LLVM for bodyless checked conversions; MD-050 must settle invalid UTF-8 and byte-slice boundaries before the string runtime requirement closes; collection validation, handled checks, maintained Semantic IR and Sec MLIR lowering remain. Implementation progress is tracked by `frontend.type-contracts-v2` in `governance/types.yaml`. Inline field contracts are recognized legacy syntax with a focused migration diagnostic and no automatic rewrite. MD-009 (2026-10-03): `regex` is reserved, has the RegexContract production, and has no same-line requirement; regex validation semantics remain MD-010. MD-010–MD-014 correction (2026-10-03): contract arguments and defaults are ordinary expressions in SemanticCompileTimeRequiredContexts (MD-011; immutable module bindings accepted, calls/getters await the semantic CTE executor with S1101); regex language moves to a dedicated future rulebook (MD-010 open); conversion failure layers are explicit and constant conversions check intrinsic representability before contracts (MD-012 open for the public error type). The 2026-09-25 governance-sync fragment was merged on 2026-10-07; its reported inline-contract and repeated range/multipleOf bugs were not reproducible against current code. On 2026-10-08, primitive operators over immutable non-integer bindings share width-aware/exact CTE values, and canonical contract/member provenance supplies related source locations through CLI and LSP. The 2026-10-09 string-length correction makes Len equal RuneLen and introduces reserved minByteLen/maxByteLen/exactByteLen string contracts and matching ContractKind variants; types.md, core-library.md, compiler_known_members.md, grammar.md and lexical_structure.md are synchronized. On 2026-10-08, all embedded implementation-progress prose was migrated to the canonical governance integration; the rulebook retains normative requirements and links directly to its owner. On 2026-10-08, the legacy direct LLVM audit closes silent validation loss: raw APIs reject contract ASTs, compiler emit/build paths check used canonical inherited/imported and nested contract facts, and fallible assignments and unresolved representations are rejected explicitly. MD-012 (2026-10-08, `rules/corrections/applied/md012-md022-conversion-errors-ffi-literals-correction-20261008.md`): `ContractKind`, `ContractError` and `ConversionError` are declared in `sec/core/error.sec`, with first-violated-contract `DeclarationIndex` semantics; native bodyless string checks use `ConversionError.Contract`; runtime typing for other contract families and maintained IR/MLIR lowering remains pending. |
| `types/default_values.md` | **Written — revision 1.2** | Canonical primitive, constrained, aggregate, list and explicit-default semantics, including declared-member defaults for integer-, string-, and bit-backed enums; all four temporal types are explicitly non-defaultable and default initialization never reads a clock. Implementation progress is tracked by `frontend.default-values` and `frontend.default-values-inherited-membership` in `governance/types.yaml`. MD-011/MD-014 (2026-10-03): explicit defaults are semantic-CTE expressions; range-constrained plain `float` derives its default from the platform width. The 2026-09-25 governance-sync fragment was merged on 2026-10-07 (its revision-2.0 label conflicts with this rulebook's declared revision 1.2 and was not adopted); the re-audit fixed defaults of generic struct and union-variant instances. On 2026-10-08, explicit, inherited membership, ambiguous/missing default, omitted-field and missing-initializer diagnostics retain related type/contract/default/field locations, verified across files and LSP UTF-16 overlays. On 2026-10-08, a shared default matrix verifies formatter preservation and LSP hover/completion across all resolver default kinds and non-defaultable forms; completion uses bounded array previews, and named list derivations retain their empty defaults. On 2026-10-08, the required diagnostic-name inventory is complete with mandatory S1136 `types.default-cycle` and S1137 `backend.default-left-undefined`; CLI/LSP transport is verified separately from future failure producers. |
| `types/units.md` | **Written** | Canonical revision 2.0 carrier-independent unit model; implementation progress is tracked by `frontend.units-v2` and `frontend.units-declaration-authority` in `governance/types.yaml`, while the standard unit catalog is tracked by `stdlib.units-catalog` in `governance/stdlib.yaml`. Unit symbols are documented as a separate namespace that never conflicts with ordinary identifiers.  Unit and ordinary type of different modules may share a spelling (core `A`/`K` beside a project `type A`), with the unit's impl kept in its own module (2026-10-04); the same-module case and bare-name conversion calls await MD-040. Implicit fixed conversions now record a compiler-resolved conversion plan and are proven exact by decimal representation or, for bounded integer carriers, by compiler-proven value ranges; a unit annotation no longer makes a numeric carrier change valid (2026-10-06). Value-aware binary-floating fixed conversion planning now proves finite source intervals exact at the selected width, and all output paths explicitly reject these plans until unit conversion lowering exists (2026-10-08). 33 standard-library units now declare safety-relevant Kind metadata; Sec 0.1 unit-polymorphic type parameters are explicitly rejected with S1135 (2026-10-08). Catalog helper bodies still require migration from legacy bracket-dimension syntax. Explicit rounding policies await MD-047 and unit-annotated register field values await MD-046. |
| `foundations/grammar.md` | **Written** | Canonical grammar distinguishes instance let from explicitly static impl bindings and includes the integer-valued `minLen`, `maxLen`, `exactLen`, `minByteLen`, `maxByteLen`, and `exactByteLen` contract productions. Interpolation holes have frontend formatting-contract selection and mixed concat/interpolation chains have maximal frontend plans; allocation/IR/backend completion remains pending. Sec 0.1 accepts only canonical syntax: `type Name TypeReference` has one underlying type, legacy `type Name = ...` forms and inline contracts are invalid with focused migration diagnostics, standalone `struct` is Sec 0.2 syntax, and prefix `[]T` was never normative (`rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md`); the parser rejects them with focused migration diagnostics P2023–P2029 while retaining the AST for recovery (`governance/parser.yaml`). MD-009 (2026-10-03): RegexContract is part of TypeContract. MD-011 (2026-10-03): contract arguments and `DefaultClause` use ordinary `Expression`; range bounds are no longer `SignedNumericConstant`. Status migration (2026-10-05): the implementation-status sections, matrix, and Appendix A were removed from the rulebook and their normative content kept; frontend.grammar-conformance and frontend.parser-recovery-result are implemented, and long-term parser work is tracked by frontend.parser-long-term in governance/parser.yaml. MD-020/MD-024 (2026-10-09, `rules/corrections/applied/md020-md021-md023-md024-ffi-c-bindings-null-correction-20261008.md`): `ForeignTypeReference`/`ForeignCallbackExpression` use only `c::`, and `ForeignNullSentinel`/`ForeignNullTest` define `is null` and `is not null`. |
| `foundations/operators.md` | **Written** | Canonical operator semantics; `in` and contextual `not in` membership are accepted by the frontend with nominal element compatibility retained. Statement-only `++` and `--` are implemented as parser-normalized compound assignments with formatter and LSP support in `frontend.increment-decrement-aliases`. Reserved `?` now produces the canonical focused P2012 parser diagnostic without acquiring Sec 0.1 expression semantics. Interpolation formatting contracts resolve to immutable Sema facts and mixed finite chains resolve to one maximal `StringConcatPlan`; allocation policy, Semantic IR consumption, and lowering remain pending. Dynamic-array backend support and the remaining cross-phase operator work are tracked by `frontend.dynamic-array-membership`, `frontend.negated-membership`, and the migrated `frontend.operators-v2` in `governance/sema.yaml`. `S1023` is a non-blocking potential-overflow warning with checked runtime semantics retained; `frontend.logical-short-circuit-flow` in `governance/control_flow.yaml` records selected `&&`/`||` RHS condition facts to suppress it when local proof is available. Runtime concatenation and interpolation now require `try` (no implicit allocation panic) per `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 5; the allocator/allocation-context choice remains MD-004. MD-006 (2026-10-03): `is`/`is not` state tests are ordinary bool expressions at the non-chainable equality level, with A2002/A2003 bad-practice diagnostics and explicit corrections; MD-004: runtime concatenation/interpolation requires try (S1097), records the resolved active allocation context and fails with StringError (allocation failure is StringError.Allocation, 2026-10-03), and is rejected without a context (S1098); Semantic IR lowering of the plan remains. Compound assignment (2026-10-05): integer `op=` and `++`/`--` publish their underlying checked operator and panic effect (fixing an unsound @noPanic acceptance of `i += 1`) and lower to Semantic IR for mutable locals. `&&`/`||` short-circuit flow, `!`, and integer range membership (`in`/`not in` with `..`/`..<`) also reach verified Semantic IR (2026-10-05). MD-043 (2026-10-08, `rules/corrections/applied/md043-char-rune-literal-correction-20261008.md`): `char` orders by unsigned 8-bit value. |
| `foundations/names_scopes_visibility.md` | **Written** | Static and instance bindings require different receivers and share the combined member-name namespace. Sema enforces uppercase initials on the declared component of user-defined nominal types after visibility prefixes (S1038), including qualified internal names. Generic parameters (S1039) and function/method parameters (S1040) cannot shadow same-module source types; broader namespace and visibility work remains. Unit symbols occupy a separate unit-symbol namespace (`rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 8); parameters, generic parameters, locals, module functions, and module variables no longer conflict with unit symbols; a unit and a nominal type of the same spelling still conflict through the shared type table.  Shadowing (2026-10-04): parameters, setter value parameters, generic parameters, and locals are checked against generic parameters, the current module's declarations, core declarations, and uppercase compiler-known types (S1104/S1105 added); imports/aliases, nested declarations, and per-module scoping of module variables remain. |
| `foundations/attributes.md` | **Written — revision 1.1** | Canonical closed Sec 0.1 attribute set, syntax, attachment, selection, target binding, `@noCopy`, verified guarantees, conflicts, and formatter/LSP behavior. Implementation status is tracked exclusively by `frontend.attributes-v2` in `governance/attributes.yaml`. General attribute parser (2026-10-05): one registry-driven parser for attachment sets with uniform S1112–S1115 validation; unimplemented attributes are rejected by Sema. @noBlock (S1116), the implication graph, conflicts (S1117), and plan-time arguments (S1118) are implemented (2026-10-05). Attribute LSP completion, hover with effective guarantees and cause paths, and quick fixes are implemented (2026-10-05); excluded-source display awaits selection. |
| `memory/unsafe.md` | **Written** | Canonical revision 2.0 unsafe contexts, operation-level obligations, unsafe functions/extern calls, trusted declarations, safe wrappers, trust provenance, target/FFI/assembly boundaries, Semantic IR, lowering, diagnostics, and tooling. Implementation progress is distributed across `analysis.unsafe` in `governance/analysis.yaml`, and `frontend.unsafe` plus `semantic-ir.unsafe` in `governance/memory.yaml`; the lowering, platform, and tooling entries remain in `implementation-status.yaml`. |

## Contextual words and operators

Sec may use a spelling contextually without making it a globally reserved word.

The following must be handled contextually:

```sec
let x := 10
let product := left x right
```

Here `x` is:

- an ordinary identifier in declaration and expression-name position;
- a matrix-multiplication operator between compatible shaped expressions.

The built-in collection type `set` and the property accessor spelling `set`
must follow the same principle.

```sec
let values: set[int]
```

In a type position followed by generic arguments, `set` is the built-in type
constructor.

Inside the property-accessor grammar, `set` introduces or identifies the setter.

Outside those contexts, `set` may remain available as an ordinary identifier if
the grammar can resolve it unambiguously.

`set` must therefore not become a globally reserved keyword solely because of
the collection type.

The parser must resolve it from:

- the current grammar context;
- token lookahead;
- type position versus property accessor position.

This decision requires synchronization of:

```text
foundations/lexical_structure.md
foundations/grammar.md
types/types.md
declarations/properties.md
collections/collections.md
collections/shaped-types.md
tooling/formatter.md
VS Code grammar
LSP token classification
```

---

# 2. Declarations, functions, and control flow

| Rulebook | Status | Notes |
|---|---|---|
| `declarations/struct.md` | **Written** | Canonical named aggregate declarations, field tags, complete/partial/spread construction, recursive defaults, field Places, copy/move, equality, destruction, and layout boundaries. Implementation is tracked by `frontend.structs`. |
| `declarations/enums.md` | **Written** | Revision 2.1 adds compile-time generic nominal enum templates and concrete owner identity while retaining the closed integer/string and open bit-backed enum model. Implementation is tracked by `frontend.enums` and `frontend.generics-v2`. |
| `declarations/unions.md` | **Written** | Canonical closed nominal unions, payload construction, explicit defaults, empty initialization state, matching, ownership, equality, generics, recursion, and representation requirements. Implementation is tracked by `frontend.unions`; recursive union storage is rejected through the shared layout graph (S1071), and the §18 `Box` indirection awaits MD-018. MD-006 (2026-10-03): `is Variant`/`is empty` and their `is not` forms are ordinary bool expressions with refinement for the negated form. |
| `declarations/registers.md` | **Written** | Canonical nominal `register[N]` types, logical bit layout, nested registers, field access semantics, conversions, and impl eligibility. Implementation is tracked by `frontend.registers`. |
| `declarations/functions.md` | **Written** | Canonical revision 2.0 functions, owned/borrowed/consuming parameters, call-transfer commit, overloads, and native typed variadics. Frontend and lowering progress is tracked by `frontend.functions-v2` in `governance/declarations.yaml`. Ordinary bodyless signatures receive P2019 with return-sensitive help; legal extern/interface signatures are exempt. |
| `declarations/lambda-functions.md` | **Written** | Canonical revision 2.0 lambda, capture, callable-capability, and closure rulebook. Frontend and lowering progress is tracked by `frontend.lambda-functions-v2` in `governance/declarations.yaml`. |
| `lambdas.md` | **Covered** | Covered by `declarations/lambda-functions.md`; no duplicate rulebook expected. |
| `closures.md` | **Covered** | Covered by `declarations/lambda-functions.md`; no duplicate rulebook expected. |
| `declarations/generics.md` | **Written** | Canonical revision 2.1 compile-time type generics, now including generic enums with phantom nominal identity parameters, concrete member owners, ordinary generic impl scope, and no runtime generic machinery. Implementation progress is tracked by `frontend.generics-v2`; generic impl scopes keep the target's declared constraints, and LSP hover presents ordered multiple-constraint sets and guaranteed methods from Sema facts, including impl target parameters. |
| `declarations/interfaces.md` | **Written** | Revision 2.1 includes compiler-known generic interfaces such as statically dispatched `Iterator[T]`; ordinary receiver capabilities, inheritance, and primary-impl conformance remain canonical. Frontend progress is tracked by `frontend.interfaces` in `governance/declarations.yaml` and `frontend.for-loops-v2` in `governance/control_flow.yaml`.  `Self` in requirements and located, explained conformance diagnostics (S1106/S1107 at the implementation, requirement as related location) are implemented (2026-10-04). Exact selected overload facts for direct, inherited, generic, and static interface requirements now share the call-graph invocation contract (2026-10-08; `frontend.interface-method-overloads`). Runtime interface dispatch remains unsupported. |
| `declarations/impl.md` | **Written** | Corrected static/instance distinction: bare let is instance-owned; explicit static let is type-owned. Per-instance storage implementation remains pending. |
| `declarations/properties.md` | **Written** | Canonical property declarations, explicit setter parameters, fallible setters, impl fragments, static properties, and interface requirements. Frontend progress is tracked by `frontend.properties` in `governance/declarations.yaml`. |
| `control-flow/defer.md` | **Written** | Canonical revision 2.0 invocation-scoped deferred cleanup, unified LIFO ordering with automatic destruction, lifetime extension, forbidden control transfer, and callable-context boundaries. Implementation progress is tracked by `frontend.defer-v2` and `frontend.defer-cleanup-state-isolation` in `governance/control_flow.yaml`. |
| `declarations/spread.md` | **Written** | Canonical postfix spread for fixed-array calls/literals and same-type struct construction. Frontend progress is tracked by `frontend.spread`. |
| `control-flow/flowcontrol_if.md` | **Written** | Canonical revision 2.0 `if`/`else if`/`else` semantics, boolean-only conditions, short-circuiting, state-test boundaries, branch flow, and diagnostics. Implementation progress is tracked by `frontend.if-statements-v2` and `frontend.logical-short-circuit-flow` in `governance/control_flow.yaml`; immutable `ResolvedIfFlow` facts now expose true/false path execution, the implicit no-branch path, and continuation for return checking and future analysis consumers. MD-006 (2026-10-03): state tests compose with &&/|| and negate with `is not`; `is Some(binding)` binds only as the complete condition; negated binding forms have focused diagnostics. |
| `control-flow/flowcontrol_for.md` | **Written** | Revision 2.1 adds explicit compiler-known `Iterator[T]` conformance and static `Next() Option[T]` iteration without runtime dispatch; infinite, range, collection, ownership, flow, and cleanup rules remain canonical. Implementation progress is tracked by `frontend.for-loops-v2` and `frontend.loop-arena-generation-fixed-point` in `governance/control_flow.yaml`. Detached iteration dependency facts now record source/backing structural and live-storage requirements, reusable versus temporary lifetime, entry/body/backedge/exit borrows, nested source relationships and explicit unknown backing (2026-10-07). The resolved Iterator[T] plan carries conformance, Next identity, source/binding kinds and temporary cleanup, and each loop records the static Next call so effects and call hierarchy see it (`analysis.iterator`). Structural mutation or replacement of an actively iterated collection is rejected with S1075 (`frontend.for-loops-v2`). Iteration requires canonical compiler-known categories or explicit intrinsic Iterator[T] conformance; legacy Vec/Set/Map iteration and method-name discovery are rejected (`analysis.iterator`). |
| `control-flow/flowcontrol_for.md` | **Written** | Canonical compiler-known collection, shaped-value, and explicit `Iterator[T]` iteration; implementation progress is tracked by `frontend.for-loops-v2` and `frontend.loop-arena-generation-fixed-point` in `governance/control_flow.yaml`. |
| `control-flow/flowcontrol_while.md` | **Written** | Canonical revision 2.0 condition-controlled loops, boolean conditions, loop-control targets, non-continuing loops, flow merging, and explicit Sec 0.1 exclusions. Implementation progress is tracked by `frontend.while-statements-v2`, `frontend.logical-short-circuit-flow`, and `frontend.loop-arena-generation-fixed-point` in `governance/control_flow.yaml`. MD-006 (2026-10-03): composed and negated state tests are accepted; a while condition never binds. Semantic IR (2026-10-05): while/break/continue lower to verified explicit loop CFG records (semantic_ir.md § 66, § 67(4)); cleanup edges and Sec MLIR remain. |
| `control-flow/flowcontrol_switch.md` | **Written** | Canonical revision 2.0 subject and subjectless switches, ordered value/range/relational cases, explicit fallthrough, case flow, and statement-only boundaries. Implementation progress is tracked by `frontend.switch-statements-v2` and the migrated `frontend.switch-cfg-and-enum-coverage` in `governance/control_flow.yaml`; enum alias duplicate cases carry stable S1015 identity and earlier equivalent-case provenance, while immutable `ResolvedSwitchFlow` facts now expose frontend exhaustiveness, unmatched continuation, clause order, and fallthrough for return analysis and future analysis consumers. |
| `control-flow/flowcontrol_match.md` | **Written** | Canonical revision 2.0 structural and variant matching, exhaustiveness, guarded ownership commit, contextual arm-block values, union empty state, and match/LSP facts. Implementation progress is tracked by `frontend.match-v2` and `frontend.match-termination-consistency` in `governance/control_flow.yaml`. MD-008 (2026-10-03): `Err(ConcreteError.Variant)` arms on a concrete Result take part in closed exhaustiveness in the frontend; Semantic IR rejects them explicitly; payload-carrying union variants await MD-029. |

---

# 3. Collections, arrays, and shaped values

| Rulebook | Status | Notes |
|---|---|---|
| `collections/collections.md` | **Written** | Canonical fixed-array, owning dynamic-array, slice, list, map, and set semantics. Frontend progress is tracked by `frontend.collections` and `frontend.dynamic-array-semantic-identity` in `governance/collections.yaml`. § 5.6a range segments in array literals and § 6.7 Append(range) added and implemented in the frontend and Semantic IR (2026-10-05). |
| `collections/shaped-types.md` | **Written** | Canonical runtime/static shaped values, affine views, layout, storage requests and transfer, broadcasting, vector/matrix algebra, and contraction semantics. Implementation progress is tracked by `frontend.shaped-types`, `frontend.shaped-static-extent-product`, and `frontend.shaped-contextual-x` in `governance/collections.yaml`. |
| `declarations/spread.md` | **Written** | Fixed-array expansion and struct construction integration. Frontend progress is tracked by `frontend.spread`. |
| `control-flow/flowcontrol_for.md` | **Written** | Canonical compiler-known collection, shaped-value, and explicit `Iterator[T]` iteration; implementation progress is tracked by `frontend.for-loops-v2` and `frontend.loop-arena-generation-fixed-point` in `governance/control_flow.yaml`. |

The first-class language types are expected to include:

```sec
list[T]
list[T, Capacity]

map[K, V]
map[K, V, Capacity]

set[T]
set[T, Capacity]

vector[T, N]
matrix[T, Rows, Columns]
tensor[T, Dimensions...]
tensor_view[T, Rank]
```

The related nominal types include:

```sec
Shape[Rank]
Strides[Rank]
TensorLayout[Rank]
MemorySpace
```

The collection rulebook is not fully implemented until the required public APIs
and algorithms also exist in stdlib.

Expected stdlib data structures include at least:

```sec
Stack[T]
Queue[T]
Deque[T]
LinkedList[T]
RingBuffer[T, Capacity]

BinaryHeap[T]
PriorityQueue[T, Priority]

OrderedMap[K, V]
OrderedSet[T]
MultiMap[K, V]
MultiSet[T]
FlatMap[K, V]
FlatSet[T]

BitSet[N]
BloomFilter[T, Bits]
Trie[K, V]
RadixTree[K, V]

Tree[T]
BinaryTree[T]
Graph[Node, Edge]
DirectedAcyclicGraph[Node, Edge]

Complex[T]
Quaternion[T]
Polynomial[T]

Grid[T, Rows, Columns]
Image[T, Width, Height]
Volume[T, X, Y, Z]
```

## Remaining collection closure questions

The main type families are decided.

The remaining details to close are:

- map and set literal syntax;
- final equality and hashing interface names;
- exact capacity and allocation error taxonomy;
- dynamic owned tensor extents;
- source syntax for layout and memory-space policies;
- precise public stdlib API names;
- sparse layout policy API.

---

# 4. Ownership, borrowing, lifetime, and storage

| Rulebook | Status | Notes |
|---|---|---|
| `memory/allocation.md` | **Written** | Canonical revision 2.0 allocation contexts, effects, Arena integration, failure, capacity, target policy, Semantic IR, lowering, diagnostics, and LSP. Implementation progress is tracked by the six `*.allocation` entries in `governance/allocation.yaml` (synchronized 2026-10-07: `@noAlloc` is verified transitively with S1108; graph-wide allocation/unknown fixed points converge through recursive components without analysis-depth cutoffs; canonical allocation capability facts distinguish availability from activation and preserve unresolved profiles; MMIO/fixed-address views remain separate from Arena ownership and canonical volatile access is allocation-free while retaining volatile effects; canonical allocation-free/may-allocate/unknown callable facts and represented implicit/explicit allocation context/domain evidence are shared by `sec analyse` and LSP, with navigable canonical S1108 allocation cause paths, dependency-aware invalidation on body/import/contract/profile changes, and verified compiler/LSP/sec analyse parity for the represented model under source-selected hosted/freestanding targets at all analysis depths, retaining unresolved selection and operation/cleanup coverage conservatively; § 4 allocation-free operations are covered). MD-004 (2026-10-03): § 17(6)–(9) define runtime string materialization through the canonical active context; the frontend resolves hosted/embedded-arena/noalloc contexts, freestanding awaits MD-034. |
| `memory/arena.md` | **Written** | Canonical revision 2.0 Arena ownership, ArenaDomain identity, backing/providers, allocation, Reset/Release lifecycle, validity epochs, execution dependencies, effects, capacity analysis, Semantic IR, lowering, target profiles, diagnostics, and tooling. The restored §1.1 cross-reference index names all adjacent authorities. Implementation progress is distributed across `analysis.arena` in `governance/analysis.yaml`, `frontend.arena-value-traits-and-domain-identity`, `frontend.arena-borrowed-backing-lifetime`, `frontend.arena`, and `semantic-ir.arena` in `governance/memory.yaml`; the lowering, platform, and tooling entries remain in `implementation-status.yaml`. Arena move-only/destruction traits propagate through aggregates, and New/Alloc type requirements use the distinct § 120 families S1072–S1074. The represented Arena frontend now has file/function rule traceability under correction25 Part II, with unchanged operation and epoch-flow bodies extracted into focused files; represented operation-point failures now use distinct S1126/S1127 IDs with dependency provenance and operation-specific help through CLI/JSON and LSP. Full Reset/Release dependency legality remains partial (2026-10-07). |
| `memory/ownership.md` | **Written** | Canonical revision 2.0 ownership, including Correction 30 exact-Place availability rules and the 2026-09-03 recursive terminal-return forwarding correction for aggregate, union, Option, Result, and value-producing control-flow construction. Implementation progress is tracked by `frontend.ownership-v2` in `implementation-status.yaml`; the § 19 custom-free partial-move ban is implemented (S1085), and unavailable-use diagnostics separate conditional, definite, and partial availability from the retained reasons (S1088–S1090). |
| `memory/borrowing.md` | **Written** | Canonical revision 2.0 borrowing, including shared/mutable borrow authority, reborrowing, Place overlap, control-flow merging, match/defer interactions, and reference-origin obligations. Implementation progress is tracked by `frontend.borrowing` in `governance/lifetime.yaml`, `semantic-ir.borrowing` in `governance/memory.yaml`, `tooling.borrowing` in `governance/tooling.yaml`, and `lowering.borrowing` in `implementation-status.yaml`. |
| `memory/references.md` | **Written** | Canonical revision 2.0 safe-reference semantics, including provenance, views, returned references, generations, relocation, hardware/ISR boundaries, Semantic IR, lowering, diagnostics, and LSP. Implementation progress is tracked by `frontend.references` and `semantic-ir.references` in `governance/memory.yaml`; the interprocedural, lowering, platform, and tooling entries remain in `implementation-status.yaml`. |
| `memory/reference_model.md` | **Written** | Canonical revision 2.0 validity and representation model for safe references, stable and weak handles, storage identity, provenance, spatial/temporal/type validity, authority, relocation, address spaces, epochs, target profiles, FFI/ISR boundaries, Semantic IR, lowering, and diagnostics. Implementation progress is distributed across `analysis.reference-model` in `governance/analysis.yaml`, and `frontend.reference-model` plus `semantic-ir.reference-model` in `governance/memory.yaml`; the lowering, platform, and tooling entries remain in `implementation-status.yaml`. |
| `memory/reference_model.md` | **Written** | Canonical revision 2.0 validity and representation model for safe references, stable and weak handles, storage identity, provenance, spatial/temporal/type validity, authority, relocation, address spaces, epochs, target profiles, FFI/ISR boundaries, Semantic IR, lowering, and diagnostics. Implementation progress is distributed across `analysis.reference-model` in `governance/analysis.yaml`, and `frontend.reference-model` plus `semantic-ir.reference-model` in `governance/memory.yaml`; the lowering, platform, and tooling entries remain in `implementation-status.yaml`. |
| `memory/raw_pointers.md` | **Written** | Canonical revision 2.0 unchecked raw-address semantics, operations, unsafe obligations, target/address-space rules, ownership/lifetime boundaries, Semantic IR, lowering, diagnostics, and tooling. Implementation progress is tracked by the six `*.raw-pointers` entries in `governance/memory.yaml`. |
| `memory/copy_move.md` | **Living** | Canonical copy/move semantics, including recursive terminal-return ownership forwarding without redundant inner `<-` markers while non-terminal construction retains explicit-move requirements. Implementation progress is tracked by `frontend.copy-move` in `governance/lifetime.yaml`, `semantic-ir.copy-move` in `governance/memory.yaml`, and the lowering/tooling entries still in `implementation-status.yaml`. |
| `memory/lifetime_analysis.md` | **Written** | Canonical revision 2.0 lifetime analysis, including value/storage/reference lifetimes, Place-sensitive invalidation, non-lexical borrows, control-flow and loop joins, returned-reference summaries, defer/capture dependencies, arena epochs, fixed-address and runtime-mapping boundaries, Semantic IR obligations, and diagnostics. Implementation progress is tracked by `frontend.lifetime-analysis` in `governance/lifetime.yaml`, `semantic-ir.lifetime-analysis` in `governance/memory.yaml`, and the interprocedural, lowering, platform, and tooling entries still in `implementation-status.yaml`. |
| `memory/destruction.md` | **Written** | Canonical revision 2.0 deterministic destruction, exact-once cleanup responsibility, partial and conditional aggregate cleanup, custom `free`, construction-failure cleanup, unified defer/destruction ordering, and target-policy boundaries. Implementation progress is tracked by `frontend.destruction` in `governance/lifetime.yaml`, `semantic-ir.destruction` in `governance/memory.yaml`, and the lowering, target-policy, and tooling entries still in `implementation-status.yaml`. Custom `free` is parsed and analyzed in the frontend (one per type, non-trivial destruction, no defer, no whole-self consumption, no partial moves; S1084–S1087), Semantic IR rejects it explicitly, and its open points await MD-025. |
| `memory/memory_model.md` | **Written** | Canonical revision 2.0 abstract memory machine separating values, objects, bindings, Places, storage, representations, ownership, borrows, provenance, validity, concurrency, hardware effects, and lowering obligations. Implementation progress is distributed across `analysis.memory-model` in `governance/analysis.yaml`, and `frontend.memory-model` plus `semantic-ir.memory-model` in `governance/memory.yaml`; the lowering, platform, and tooling entries remain in `implementation-status.yaml`. |
| `declarations/static.md` | **Written** | Explicit static is required for type-owned impl bindings, including immutable bindings. Implementation is tracked by frontend.static-declarations-members. |
| `control-flow/discard.md` | **Written** | Canonical revision 2.0 explicit and implicit discard, must-use/discardability, reinitialization, lifecycle-handle, and deterministic destruction semantics. The implemented frontend slice and remaining lowering, Place, diagnostics, path-sensitive, and aggregate-temporary work are tracked by `frontend.discard-v2` and `frontend.discard-aggregate-temporary-ownership` in `governance/control_flow.yaml`. |
| `memory/storage.md` | **Written** | Canonical revision 2.0 storage origin, backing relation, reclamation authority, address stability, regions, invalidation domains, epochs, pin/protection state, memory spaces, placement, concurrency, Semantic IR, lowering, and diagnostics. Implementation progress is distributed across `analysis.storage` in `governance/analysis.yaml`, and `frontend.storage` plus `semantic-ir.storage` in `governance/memory.yaml`; the lowering, platform, and tooling entries remain in `implementation-status.yaml`. |
| `memory/layout.md` | **Written** | Canonical revision 2.0 semantic/native/explicit layout, size, alignment, stride, padding, aggregate and union representation, completeness, validity, target plans, ABI/register integration, Semantic IR, lowering, and tooling. The by-value layout dependency graph (structs, unions, Option/Result payloads, generic instantiation; S1055/S1071 with full paths) is implemented. Implementation progress is tracked by `analysis.layout` in `governance/analysis.yaml` and the other five `*.layout` entries in `governance/memory.yaml`. § 18(4a) (MD-014, 2026-10-03): plain `float` shares the platform width fact with `int`/`uint`. |

`storage_allocation.txt` from the old checklist is replaced by the clearer
canonical rulebook:

```text
memory/storage.md
```

Allocation policy remains in:

```text
memory/allocation.md
```

Physical storage categories and placement belong in:

```text
memory/storage.md
```

---

# 5. Errors, panic, and runtime checks

| Rulebook | Status | Notes |
|---|---|---|
| `errors/errorhandling.md` | **Written** | Canonical revision 2.1 compiler-known `error`, typed Result channels, direct compatible Result/Option carrier returns, Result projections, general Result/Option/fallible-operation `try`, partial guarded handlers, explicit fallible-setter contracts, ownership, Semantic IR, diagnostics, and LSP requirements. Parser rejects obsolete nested `match` try wrappers with P2012; Sema reports shadowed Err handlers with S1045, rejects non-discardable concrete Err(_) payloads in try and match with S1006, and resolves naked Option try as Some-unwrapping plus compatible None propagation without Option/Result conversion. Local Option handlers, open-error discardability, and broader handler semantics remain partial. Implementation progress is tracked by `frontend.errorhandling-v2` in `governance/errors.yaml`. MD-008 (2026-10-03): § 27.2a concrete error-variant match arms are implemented in the frontend. § 23 (MD-012, 2026-10-03) records the intrinsic-versus-contract conversion failure layers; the public error type remains open. MD-012 (2026-10-08, `rules/corrections/applied/md012-md022-conversion-errors-ffi-literals-correction-20261008.md`): both runtime conversion-failure layers use `ConversionError`, without aliasing the existing narrower core errors. |
| `errors/panic.md` | **Written — revision 2.3** | Canonical panic domains, containment, cleanup, checked unreachable, task/thread/process outcomes, exact `PanicID uint32` and five-field `PanicInfo`, no-panic verification, and runtime-free support model. Sec 0.1 assertion syntax is locked to `assert condition` or `assert condition, "message"`; frontend effect analysis records unproven assertions as `MayPanic`, proves literal `true` panic-free, and treats literal `false` as non-returning, while broader proof and lowering remain. Explicit panic is locked to the statements `panic` and `panic "message"`; function-like and dynamic-message forms remain invalid, and valid statements now publish immutable reason, static-message, and source/function provenance facts for later stages. Exact build-manifest syntax remains open. |
| `errors/runtime_checks.md` | **Written** | Canonical checked-operation model, proof elimination, fallible `try` paths, panic-capable ordinary paths, typed propagation, no-panic integration, and runtime-free lowering requirements. MD-012 (2026-10-08, `rules/corrections/applied/md012-md022-conversion-errors-ffi-literals-correction-20261008.md`): checked conversion, including contract construction, fails with `ConversionError`. |
| `library/core-library.md` | **Written** | Canonical core/compiler boundary, privileged compiler-known declaration policy, source-visible core declarations, and language-level core errors including panic, concurrency, context, atomic, process, and thread families. |
| `core/errors.sec` | **Implementation artifact** | Every language-level runtime error type must be declared here. |

A runtime check does not imply a general managed runtime.

A check may lower to:

- inline comparisons and branches;
- a direct panic path;
- a target-specific trap;
- a profile-selected handler;
- a statically eliminated operation.

The canonical behavior is defined by `errors/panic.md` and `errors/runtime_checks.md`; their
implementation remains tracked separately.

---

# 6. Concurrency and asynchronous execution

| Rulebook | Status | Notes |
|---|---|---|
| `concurrency/concurrency.md` | **Written** | Canonical revision 2.2 umbrella model synchronized with task v2, process v2, IPC v1, execution-kind boundaries, transferability, race/deadlock ownership, Semantic IR, and immutable `CompilationPlan`-driven lowering. Implementation is tracked by `concurrency.model-v2` in `governance/concurrency_model.yaml` and the specialist governance entries. |
| `concurrency/concurrency_memory_model.md` | **Written** | Revision 2.0 defines exact MemoryOrder, per-atomic modification order, release sequences, compare-exchange paths, fences, completion publication, and analysis/lowering obligations; IPC visibility follows its explicit transport or shared-memory contract rather than inheriting ordinary in-process semantics. |
| `concurrency/concurrency_runtime_model.md` | **Written** | Canonical no-required-runtime profile model, runtime capability surface, task/thread/process/context errors, and target-selected lowering boundaries. |
| `concurrency/tasks.md` | **Written** | Canonical revision 2.0 task semantics: fallible task spawn, move-only lifecycle ownership, `TaskOutcome[T]`, cancellation, panic/execution-failure separation, observers, transferability, Semantic IR, and runtime-independent lowering. Implementation is tracked by `concurrency.tasks-v2` in `governance/concurrency_task.yaml`. |
| `concurrency/spawn.md` | **Written** | All spawn forms are fallible; `spawn process` yields `Result[Process[T], ProcessSpawnError]` with process-specific transactional transfer. |
| `concurrency/await.md` | **Written** | Synchronized revision 2.0 defines invariant `await Task[T] -> TaskOutcome[T]` typing, exact `TaskError`, consuming lifecycle ownership, await commit semantics, and structured pre-commit caller-cancellation cleanup without implicit detach. Implementation progress is tracked by `concurrency.await-v2` in `governance/concurrency_await.yaml`. |
| `concurrency/threads.md` | **Written — synchronized revision 2.0** | Revision 2.0 defines exact thread configuration/storage, physical lifecycle, terminal properties, selectable join, observer/current-thread boundaries, cooperative cancellation, and target-resolved platform metadata. Implementation remains partial and is tracked by `concurrency.thread-v2` in `governance/concurrency_thread.yaml`. |
| `concurrency/thread_local.md` | **Written — revision 2.0 synchronized** | Explicit `Borrow`/`BorrowMut`/`Replace` TLS access, deterministic lazy per-thread initialization, thread-bound borrows, reverse successful-initialization destruction, and owned foreign-thread attachment through `ThreadAttachment`/`ThreadAttachError`. The frontend now resolves the exact three TLS access methods, their concrete result types, and `Replace` argument ownership; provenance, initialization/runtime, constructors, attachment, and lowering remain partial under `concurrency.thread-local-v2` in `governance/concurrency_thread_local.yaml`. |
| `concurrency/scheduling.md` | **Written — synchronized revision 2.0** | Revision 2.0 defines `Task.Yield()` as a cancellation-aware logical scheduling point, `Thread.Yield()` as a non-cancelling physical scheduler hint, explicit task-migration and execution-affinity constraints, and target-independent ordinary-main `TaskContext` semantics. Implementation remains partial under `concurrency.scheduling-v2` in `governance/concurrency_scheduling.yaml`. |
| `concurrency/blocking.md` | **Written** | Includes owning process/Command join, non-owning ProcessObserver waits, IPC waiting operations and guard liveness, process effects, cancellation commit, deadlock edges, and ISR restrictions. Implementation status is tracked by `frontend.blocking-effects` in `governance/concurrency_blocking.yaml` (blocking-effect summary and @noBlock verification, 2026-10-05). |
| `concurrency/cancellation.md` | **Written** | Revision 2.0 defines distinct task/thread cancellation, inferred cancellable-execution effects, exactly-one commit, and Context/ContextSource. |
| `concurrency/structured_concurrency.md` | **Covered** | No separate rulebook is required. Parent-child lifecycle, explicit detach, consuming-await cancellation cleanup, cancellation propagation, and deterministic child cleanup are covered by `concurrency/concurrency.md`, `concurrency/tasks.md`, `concurrency/await.md`, `concurrency/cancellation.md`, and the ownership/destruction rulebooks. |
| `memory/transferability.md` | **Written** | Canonical revision 2.0 boundary-specific transferability and shareability across tasks, physical threads, processes, interrupts, and foreign callbacks, including IPC route adapters, transactional capability transfer, closure/reference/capability dependencies, and platform constraints. Implementation progress is distributed across `analysis.transferability` in `governance/analysis.yaml`, `frontend.transferability` and `semantic-ir.transferability` in `governance/memory.yaml`, and the lowering, platform, and tooling entries still in `implementation-status.yaml`. |
| `analysis/data_races.md` | **Written** | Canonical data-race analysis rules; implementation status is tracked by `sema.data-race-analysis` in `implementation-status.yaml`. |
| `analysis/deadlock_analysis.md` | **Written** | Canonical deadlock-analysis rules; implementation status is tracked by `sema.deadlock-analysis` in `implementation-status.yaml`. |
| `concurrency/channels.md` | **Written** | Revision 2.0 defines exact in-process Channel/endpoint/result APIs, allocation-only fallible construction, sender sharing limits, revocable-ticket ownership, expiration/statistics, select/cancellation commit semantics, and strict IPC separation. Implementation remains partial and is tracked by `concurrency.channels-v2` in `governance/concurrency_channels.yaml`. |
| `concurrency/events.md` | **Written — sync required** | C#-style publish/subscribe event model; distinct from readiness/completion. |
| `concurrency/select.md` | **Written — synchronized revision 2.0** | Revision 2.0 defines source-order prepare/readiness/commit semantics, exact selectable operation categories, unified Task/Thread/Process/Command join selection, and excludes nonblocking `Try*` and `CancelRequested` branches. Implementation remains partial and is tracked by `concurrency.select-v2` in `governance/concurrency_select.yaml`. |
| `concurrency/mutex.md` | **Written** | Revision 2.0 defines canonical Lock overloads, @noCopy guards, forwarding, Context ownership, duration/Instant, and cancellation commit semantics. |
| `concurrency/atomics.md` | **Written** | Revision 2.0 defines canonical Atomic[T], MemoryOrder, CompareExchangeResult[T], CamelCase operations, fences, and target-capability separation; IPCAtomic[T] reuses its order and compare-exchange contracts while IPC owns process-shared eligibility and lifecycle. |
| `concurrency/processes.md` | **Written** | Revision 2.1 is the normative Sec 0.1 model for Process[T], Command, lifecycle/completion, standard I/O, Resource-mode direct File/Pipe binding, target capabilities, analysis, and lowering. Frontend implementation has begun with target-sized nominal `ProcessID` and the exact `ProcessStatus` enum; process creation, lifecycle, runtime, and lowering remain partial and are tracked by `concurrency.processes-v2` in `governance/concurrency_process.yaml`. |
| `concurrency/ipc.md` | **Written** | Revision 1.0 defines canonical pipes, typed IPC, shared memory/mappings, IPCMutex/IPCSemaphore, SharedValue[T], IPCAtomic[T], capability/resource transfer, Command Resource binding, target capabilities, analysis, lowering, security, and conformance obligations. Implementation remains planned and is tracked by `concurrency.ipc-v1` in `governance/concurrency_ipc.yaml`. |

Threads are considered design-complete for Sec 0.1.

The expected implementation order is:

```text
parser
AST
Sema
analysis
Semantic IR
MLIR and target lowering
```

IPC is design-complete for Sec 0.1 but its compiler/runtime/platform implementation remains planned.

---

# 7. Type system, core, and standard library integration

| Rulebook | Status | Notes |
|---|---|---|
| `types/types.md` | **Written** | Canonical fundamental, scalar, temporal, named, collection-shaped, and declaration type contract; implementation status is maintained in `governance/types.yaml`. The 2026-10-08 temporal-duration unit-spelling audit is complete; time-quantity conversion remains blocked by MD-015, including its syntax, rounding and failure policy. |
| `types/temporal.md` | **Written — revision 1.1** | Canonical temporal backing representation, core-authority boundary, and UTC wall-clock properties through private `_now`; the frontend intrinsic, direct effect facts, static-initializer rejection, core property projections, and visibility-aware LSP completion are implemented, while capability validation, broader compile-time-context integration, IR, and lowering remain. |
| `declarations/generics.md` | **Written** | Type-generic declarations, parameters, constraints, inference, generic enums/interfaces/named types/methods, and template-level validity. Canonical concrete specialization is owned by `compiler/monomorphization.md`; compile-time value parameters still require separate normative semantics. |
| `declarations/interfaces.md` | **Written** | Interface declarations, generic constraints, and compiler-known `Iterator[T]`; implementation progress is tracked by `frontend.interfaces` in `governance/declarations.yaml` and `frontend.for-loops-v2` in `governance/control_flow.yaml`. |
| `declarations/impl.md` | **Written** | Corrected static/instance distinction: bare let is instance-owned; explicit static let is type-owned. Per-instance storage implementation remains pending. |
| `declarations/properties.md` | **Written** | Implementation progress is tracked by `frontend.properties` in `governance/declarations.yaml`. |
| `library/core-library.md` | **Written** | Compiler-known core declarations, privileged impl access, source-visible compiler/core identity, and required language-level core errors. |
| `compiler/compiler_known_members.md` | **Written** | Canonical typed registry, stable member identities, fallback-versus-authoritative policy, universal `ToString() Result[string, StringError]` (every ToString declaration and overload, enforced with S1100; StringError is the shared text failure family of ToString, concatenation, and interpolation; decisions 2026-10-03, MD-035/MD-038), property-only `SizeOf`, and private core-only `_now: datetime` with `MayUseNondeterministicInput` and `UTCWallClock`; `_now` is registered and provenance-gated in Sema with stable direct effect facts, while plan validation, Semantic IR, and lowering remain. Existing Sema/LSP registry integration remains partial.  Every registry entry carries signature, documentation, rule section, receiver pattern, and target restriction, and LSP definition navigates to a read-only synthetic definition (`sec-compiler-known:` scheme, VS Code content provider) when no core declaration exists (2026-10-04). |
| `library/stdlib.md` | **Written — partially implemented** | Standard-library boundaries and target contracts, including the canonical `stdlib/hw` area and reserved `hw/spi`, `hw/i2c`, `hw/i2s`, and `hw/uart` infrastructure, plus Linux/amd64 streaming file IO, exact and complete caller-buffer reads, writes/copy, seek/flush/truncate/close, directory iteration, non-recursive path operations, path bridging, explicit resource lifecycle diagnostics, and the `File.Duplicate()` contract required for direct IPC/process resource binding. Hardware-bus governance is tracked by `stdlib.hardware-bus-infrastructure` in `governance/stdlib.yaml`; allocating complete-file APIs and directory-list APIs remain pending. |

Built-in lowercase types may receive privileged implementations in core.

The standard library may use those types and provide higher-level nominal
types and algorithms, but it may not redefine or globally extend their
first-class member surface.

Examples:

```sec
impl string {
}

impl list[T] {
}

impl matrix[T, Rows, Columns] {
}
```

Ordinary user code may define implementations for its own nominal types, but may
not globally extend built-in lowercase types.

Nominal core, stdlib, and user types begin with uppercase letters:

```sec
Result[T, E]
Option[T]
Thread[T]
Shape[Rank]
Stack[T]
OrderedMap[K, V]
```

---

# 8. Compiler architecture, analysis, and IR

| Rulebook | Status | Notes |
|---|---|---|
| `compiler/compiler.md` | **Written** | Canonical revision 2.0 compiler responsibilities, Target/Variant and CompilationPlan authority, source selection, compiler-known surfaces, frontend orchestration, backend boundaries, diagnostics, and tooling contracts. Implementation progress is tracked by `frontend.compiler-source-selection` and `frontend.compiler-pipeline` in `governance/compiler.yaml`, alongside the remaining compiler-core family in `implementation-status.yaml`. |
| `compiler/compiler_analysis.md` | **Written** | Canonical revision 2.0 analysis-coordination and fact-ownership model. Implementation progress is tracked by `analysis.compiler-analysis` in `governance/analysis.yaml`. Deterministic dependency scheduling runs existing semantic producers and validators; shared bounded reference/demand fixed points publish conservative widening before consumers, with detached dependency/convergence records and implementation documentation (2026-10-06). |
| `compiler/compiler_pipeline.md` | **Written** | Canonical revision 2.0 phase-boundary pipeline from CompilationRequest and CompilationPlan through frontend analysis, Semantic IR, Sec MLIR, target artifacts, linking, testing, and publication. Implementation progress is tracked by `frontend.compiler-pipeline` in `governance/compiler.yaml`, `semantic-ir.compiler-pipeline` in `governance/lowering.yaml`, and `analysis.compiler-pipeline` in `governance/analysis.yaml`. Reachable protocol iteration now requires canonical Sema resolution, Next invocation and storage/lifetime plans before Semantic IR or legacy LLVM/MLIR backend generation; Invalid and Unproven prerequisite issues now retain registered S1124/S1125 identities, per-issue spans and help through CLI/JSON pipeline transport, including wrapped and joined failures. Owner-supplied stack-budget proof states also survive CLI/LSP transport; typed unsupported lowering retains its separate capability category (2026-10-07). |
| `compiler/incremental_compilation.md` | **Written** | Canonical Sec 0.1 correctness model for logical snapshots, dependency and fingerprint validity, fail-closed reuse, set-valued and negative invalidation, transactional publication, persistent/shared cache integrity, ModuleSurface-aware invalidation, source/provenance separation, plan-specific artifact reuse, history independence, and cold-versus-incremental conformance. Implementation is planned under `compiler.incremental-compilation-v1` in `governance/compiler.yaml`. |
| `compiler/semantic_ir.md` | **Written** | Canonical revision 2.0 typed Semantic IR model, including ownership, allocation, effects, concurrency, IPC resource/transfer/commit facts, collections, shaped values, panic, checks, and resolved iterator plans. Implementation progress is tracked by `frontend.semantic-ir-v2`, `semantic-ir.core-v2`, and `semantic-ir.compiler-pipeline` in `governance/lowering.yaml`, together with the owning specialist governance entries. MD-004 (2026-10-03): § 17(8) requires the resolved allocation context and failure channel; runtime string materialization is still an explicit unsupported-lowering case. |
| `compiler/rules_implementations.txt` | **Living** | Legacy implementation notes being migrated into `implementation-status.yaml`. |
| `analysis/call_graph.md` | **Written** | Canonical callable reachability and execution relationships. Implementation status is tracked by `analysis.call-graph` in `governance/analysis.yaml`. Closed joined and open function-value/interface target sets now retain validated public invocation contracts, known explicit implementations and conservative absent guarantees; stack evidence uses one canonical boundary per opaque invocation. Declaration and lexical syntax identities now survive unrelated snapshot edits with refreshed source navigation; explicit CompilationPlan/model binding qualifies every node, closure, site, root and reference, and scoped workspace publication and schema-3 checkpoints preserve these identities. A shared domain-summary consumer now unions all nine canonical may-effects over synchronous dependencies, retains explicit unknown domains and detached call-site cause witnesses, terminates over recursion and separates spawn creation from spawned-body effects. A workspace graph-index service now validates exact scope/dependency fingerprints, retains committed checkpoints, invalidates dependent graph units and rejects obsolete publication; AnalyzeIndexed publishes only successful Sema, with cache errors kept separate. Validated defer bodies now have distinct nodes and deferred same-stack edges for reachability, effects, recursion, stack composition and checkpoints, including empty bodies and lambda-owned cleanup. Validated test bodies now retain internal callable nodes; explicit test-plan selection produces scoped test roots with helper/cleanup/worker reachability, production-root isolation and checkpoint preservation. Default test harness/driver integration, driver/LSP cache wiring, remaining domain producers, imported guarantees, full graph coverage and ordered cleanup/lifetime/root/plan paths remain pending (2026-10-07). |
| `analysis/stack_analysis.md` | **Written** | Canonical semantic and machine stack-resource analysis; implementation status is tracked by `sema.stack-analysis` in `governance/analysis.yaml`. The immutable StackBound representation now distinguishes Exact, UpperBound, Unknown and Unbounded, validates arbitrary-precision finite byte counts, and keeps unknown separate from zero; separate semantic and machine summary models now preserve own-frame, transitive maximum and supplied cause evidence in independent callable/CompilationPlan namespaces with detached deterministic snapshots. Direct acyclic call paths now compose supplied frame facts through the canonical call graph: nested chains sum, sequential/alternative calls take their maximum, and missing, recursive, foreign or open-callable evidence remains Unknown. Non-leaf finite results retain conservative UpperBound precision. Closed callable target sets now contribute their maximum, including joined named values, lambdas and capturing closures; missing targets or open/inconsistent coverage cannot yield finite proofs, and cause selection is independent of target order. Partial evidence now retains known frame prefixes, independent contributors and deterministic representative paths to every distinct unknown boundary at both measurement levels, including missing-frame gaps and closed-target alternatives; these facts never replace an unknown overall budget proof. A separate immutable RecursionDepthBound model now retains ExactDepth, UpperBoundDepth, UnknownDepth and UnboundedDepth with validated arbitrary-precision counts; depth inference and recursive composition remain pending. Immutable physical-domain and measurement-level stack budgets now support arbitrary-precision availability and explicit comparisons that separate satisfaction, exact excess, unproven upper bounds, Unknown and Unbounded. Explicit active-budget diagnostics now distinguish exact excess (S1121), unavailable sufficiency proofs (S1122) and proven unbounded demand (S1123), with shared CLI/LSP identity and help. Project budget syntax is tracked by MD-044 and build policy remains pending. Open callable consumers now compose verified finite contract bounds with canonical known targets at independent levels/plans, require verified no-reentry facts and retain conservative uncertainty for missing or incompatible guarantees. External consumers now compose explicit verified foreign/runtime/platform whole-call and runtime-effect bounds at exact canonical sites and independent level/plan identities, including matching panic paths; missing contracts or reentry proof remain Unknown. Contract producer syntax/import and trust provenance are tracked by MD-045; frame producers and recursive stack-proof computation remain unimplemented (2026-10-06). |
| `analysis/escape_analysis.md` | **Written — revision 2.0** | Canonical escape subjects, destinations, retention, summaries, diagnostics, and no-silent-promotion contract. Implementation status is tracked by `sema.escape-analysis`. Represented escape failures now use mandatory central S1128–S1133 identities with origin/sink locations and remedies through CLI/JSON and LSP; unknown control-flow provenance has a distinct identity and does not assert proven local-storage escape. Full retention/transfer analysis remains partial (2026-10-07). Revision 2.0 (2026-10-07, `rules/corrections/applied/escape-analysis-rev2-sync-20261007.md`) synchronizes call-bounded implicit borrowing, explicit reusable-source transfer, intrinsic return transfer, and the diagnostics boundary; `return <-value` and `return value` now record equal facts and a value copied out through a borrow no longer records a false returned dependency, while call-boundary OwnershipTransferred and retention facts remain unimplemented. |
| `analysis/closure_analysis.md` | **Written** | Canonical capture, callable-flow, target-set, closure-summary, and analysis-budget model. Implementation status is tracked by `sema.closure-analysis`. Capture and creation snapshots defensively detach structural type data on source/referent Places and alternative origins. |
| `analysis/parameter_usage_analysis.md` | **Written — revision 2.0** | Canonical multidimensional parameter-demand and advisory-narrowing model. Implementation status is tracked by `sema.parameter-usage-analysis`; parameter demand consumes Sema-owned `if` reachability, excludes proven never-executed branches, stops after statements or resolved branch flow that cannot continue the current block, and distinguishes direct by-value WholeValue demand from projected and reference-only access. Reachable fixed-array return materialization contributes ExactExtent; resolved sequence Ptr access contributes AddressRequired and contiguous shape/storage demand. `sec analyse` (default and `--all`) reports parameter demand and structured shared-reference candidate confidence, reasons, and blockers at Deep depth, with cost evidence kept separate from semantic demand; individual selection awaits MD-017. Deterministic node/nesting budgets at all three depths admit complete bodies, expose skipped coverage and per-rule incompleteness, retain supported findings, and preserve normative Sema errors (2026-10-05). Endpoint reachability now follows nested guards, boolean and short-circuit paths, assertions and scoped exits; uncertain saved/live length relations after mutation remain explicitly incomplete rather than becoming a bounds proof (2026-10-06). Optional LSP hover and A2001 severity are configurable through sec.analysis.parameters, default off, with resolved declaration identity, compiler-owned candidate evidence and live configuration refresh (2026-10-06). A six-category regression matrix verifies the integrated narrowing/FFI fallback/policy slice and recursive projection widening; binding identity isolates nested parameters, function-value invocation records callee use, and capture creation records partial caller demand without folding lambda bodies into it (2026-10-06). Shared-reference policy now preserves explicit free lifecycle ownership through owned aggregates and collections; semantic snapshots retain CustomFree, and A2001 array messages offer only verified same-type references. Explicit layout contract syntax remains blocked by MD-019 (2026-10-06). Function-value and closure demand now consumes canonical known/open target sets and public contracts, with separate lambda parameters, preserved projection evidence and dimension-specific escape uncertainty (2026-10-07). Revision 2.0 (2026-10-07, `rules/corrections/applied/parameter-usage-analysis-rev2-sync-20261007.md`) synchronizes ownership transfer with consuming `->` parameters, `<-` for reusable Places, marker-free fresh temporaries, intrinsic return transfer, and the diagnostics boundary; `return <-value` now yields the same demand as `return value`, and ownership-changing recommendation code actions remain pending. |
| `analysis/pitfall_analysis.md` | **Written — revision 2.0** | Revision 2.0 (2026-10-04, `rules/corrections/applied/pitfall-analysis-rev2-sync-20261004.md`) is a synchronization revision: versioned header, governance ownership moved to `sema.pitfall-analysis` in `governance/analysis.yaml`, `&&`/`||` examples, pitfall rule identities kept distinct from registered diagnostic IDs, and project-configuration syntax owned by `projects/projects.md`; implementation notes were moved out of the rulebook into governance. Canonical semantic pitfall-finding, evidence, suppression, confidence, corrective-action, budget, incremental, LSP, and FFI-contract model. `sema.pitfall-analysis` is tracked in `governance/analysis.yaml`; resolved bindings, compiler-known `Len`, flow and constant facts support endpoint suppression, ineffective `<= Len`/`> Len` guard findings, and conservative upper/lower neighbor checks for canonical `0..<Len` traversal; proven tautological and impossible integer interval conditions are reported with a canonical `in` suggestion, and Deep analysis offers a proven range-membership fix with side-effect suppression (2026-10-03). Control-flow guard correlation reports strict guards that protect nothing while another access is unguarded on every path (wrong guard subject) and out-of-range checks that neither leave the path nor re-establish the index before the access (2026-10-03). Structural mutation of the traversed collection inside a `..<Len` loop is classified as proven invalid (Clear then index), likely mistake (RemoveAt/Remove/Clear/Insert with index use), or a suppressed safe pattern (break/return after the mutation, Append, draining loop) (2026-10-03). A loop to `Capacity` that indexes the collection is a likely mistake unless `Len == Capacity` is proven on the path (2026-10-04). `0..<X.Len - 1` traversals that skip the final element without pairwise or separate-final-element evidence, and fragile inclusive `0..X.Len - 1` loops (proven fix with a non-empty proof, likely mistake without) are reported (2026-10-04). `sec analyse` (default and `--all`) presents findings, suppressions, actions and not-evaluated rules at Deep depth, keeping ProvenInvalid as an error; individual selection awaits MD-017. All current automatic fix families require complete observable-equivalence evidence at publication; getter/volatile reads and proof-invalidating loop starts cannot acquire ProvenFix status (2026-10-06). A source false-positive corpus now covers sixteen intentional patterns and eleven positive controls at every analysis depth, including neighbor/sentinel/endpoint intent, parallel collections, ring buffers, capacity work, protocol ranges, fixed extents and byte-buffer FFI calls without inferred contracts. Nonempty proof now survives one verified read-only scalar traversal of the same collection; broader mutation-aware dominance remains pending (2026-10-06). Canonical idiom suggestions now have shared in/..< action metadata and priority, complete omitted-last-element replacements, and CLI replacement text, with unchanged validity/depth/fix-safety boundaries (2026-10-06). Certified constant-result fixes now distinguish pure interval/type-range simplifications from intent edits and evaluation-removing suggestions, with source-context and CLI regressions (2026-10-07). Canonical foreign pointer/extent relationships now correlate disjoint known Ptr and Len/SizeOf storage origins with advisory evidence; missing contracts and uncertain reference provenance remain silent (2026-10-07). Foreign extent-unit correlation now distinguishes canonical element and byte quantities with exact scalar/native layout facts; byte/zero equivalence and unknown layouts remain silent, and size corrections stay advisory (2026-10-07). Optional pitfall LSP diagnostics now use registered A2004 with project/client diagnostic policy, current-depth configuration refresh and freshly resolved code actions; exact certified edits share a compiler fix materializer, intent suggestions remain manual, and mandatory owners survive advisory suppression (2026-10-07). Explicit AnalyzeWithPitfallCache integration now supports dependency-qualified in-memory reuse of complete optional body searches, exact scope/configuration compatibility and generation-checked invalidation/publication; incomplete facts, recovery and normative owners remain cold, and hosts own prerequisite revisions and cache lifetime rather than enabling a default CLI/LSP cache policy (2026-10-07). |
| `analysis/effect_analysis.md` | **Written** | Canonical compile-time effect domains and propagation model, including the distinction between logical shaped operations and storage-producing shaped operations. Initial Arena event sites, synchronous `MayAllocate` propagation, cause paths, and LSP hover are implemented; the complete effect set, guarantees, contexts, indirect targets, and per-plan analysis remain. |
| `analysis/isr_analysis.md` | **Written** | Canonical cross-analysis ISR constraint verifier: resolved profiles, reusable requirement summaries, execution-context propagation, stack/race/deadlock/FFI composition, Valid/Invalid/Unproven proof states, incremental dependencies, and progressive LSP refinement. Implementation status is tracked by `sema.isr-analysis`. |
| `compiler/parser_recovery.md` | **Written** | Canonical deterministic recovery model; ordinary unterminated statement blocks, switch/select bodies, match arms, try handlers, typed struct/array literals, completed grouped expressions, call argument lists, and index/slice postfix expressions at EOF retain partial AST contents for tooling while errors still block code generation. Literal/group/call/index/slice recovery preserves completed children or bounds and records the appropriate virtual closing delimiter; an empty EOF index position is retained as `InvalidExpression`. Empty comma-delimited call arguments retain their position as `InvalidExpression` without losing later arguments or declarations. Invalid type metadata covers base and postfix families and propagates through diagnosed composite children. Select header recovery guarantees progress; other specialized recovery remains partial. Recovery events reach the concrete syntax model as exact `internal/cst` recovery nodes, with missing-token anchors marked exact only when the parser proved the insertion point (2026-10-09). |
| `compiler/generics_lowering.md` | **Written** | Verified concrete-generic closure and representation-dependent lowering boundary. Canonical specialization identity and demand are owned by `compiler/monomorphization.md`; implementation remains partial under `compiler.generics-lowering`. |
| `compiler/monomorphization.md` | **Written** | Canonical demand-driven specialization model: `InstantiationIdentity`, dependency graph, simultaneous substitution, semantic-versus-physical realization, implementation sharing, cross-module artifacts, ABI/FFI, incremental fingerprints, termination/resource limits, diagnostics, cost policy, and determinism. Implementation is tracked by `compiler.monomorphization`. |
| `compiler/linking.md` | **Written** | Canonical CompilationPlan-specific LinkPlan, binary symbol identity, native/foreign resolution, archives, reachability, dead stripping/LTO, deterministic toolchain materialization, and artifact verification. The existing direct clang-driver build path is a legacy partial slice; canonical work is tracked by `compiler.linking`. |
| `compiler/compile_time_evaluation.md` | **Written** | Canonical Sec 0.1 plan-time and typed semantic CTE model, including user functions, effects, ownership, transient allocation, generics, materialization, diagnostics, caching, and lowering boundaries. Implementation is tracked by `compiler.compile-time-evaluation`. § 8(5)–(7) (MD-011, 2026-10-03) classify contract arguments and explicit defaults as SemanticCompileTimeRequiredContexts; Sema evaluates them through shared compile-time evaluation, and calls/getters await the executor (S1101). |

File extensions for new rulebooks should preferably converge on:

```text
.md
```

Existing `.txt` rulebooks do not need to be renamed immediately unless a
repository-wide migration is chosen.

---

# 9. Sec MLIR dialect and lowering

The Sec MLIR material has its own category because governance, current
specifications, historical schema snapshots, Semantic IR amendments, normative
synchronization notes, and bounded implementation packages have different
lifecycles.

| Rulebook group | Status | Notes |
|---|---|---|
| `mlir/sec_mlir.md` | **Written** | Governance and canonical high-level Sec MLIR boundary. |
| `mlir/sec_mlir_dialect.md` | **Written** | Current canonical dialect specification, synchronized through schema 9 / SEC-MLIR-P13. |
| `mlir/sec_mlir_lowering.md` | **Written** | Current canonical lowering specification, synchronized through lowering version 9 / SEC-MLIR-P13. |
| `mlir/mlir.txt` | **Written — sync required** | General MLIR architecture notes. |
| `mlir/mlir-optimize.txt` | **Living** | Updated as optimization support grows. |
| `mlir/packages/` | **Living** | Numbered implementation packages 1–19 with separate package YAML files. |
| `mlir/dialect-versions/` | **Written** | Historical dialect snapshots retained for schema history and regression work; not the current canonical specification. |
| `mlir/lowering-versions/` | **Written** | Historical lowering snapshots retained for compatibility and regression work. |
| `mlir/semantic-ir/` | **Written** | Package-scoped amendments to the canonical Semantic IR rulebook. |
| `mlir/normative-sync/` | **Written** | Package-scoped synchronization amendments for non-MLIR rulebooks. |

Implementation state remains in `implementation-status.yaml` and in the
package-local YAML files under `mlir/packages/`; it must not be inferred merely
from the presence of a versioned document.

---

# 10. Hardware, platform, interrupts, and ABI

| Rulebook | Status | Notes |
|---|---|---|
| `platform/ffi.md` | **Written** | Canonical revision 2.0 foreign declarations, C ABI type families, data representations, callbacks, varargs, strings, ownership, effects, symbols, and legality. `governance/ffi.yaml` owns `frontend.ffi-v2`; Sema rejects unresolved generic extern declarations with S1041, while broader ABI support remains partial. `C::` fundamental scalars parse and resolve through each target's C ABI model as distinct types (S1076–S1078, P2021–P2022); `c::` bindings await MD-021. C/system extern signatures are checked per position for FFI legality (S1079); int/uint and Sec-enum legality await MD-023. The unsafe-only `null` sentinel, contextual RawPtr typing, `is null`, and equality rejection are implemented (S1080–S1083); its lexical status awaits MD-024. MD-022 (2026-10-08, `rules/corrections/applied/md012-md022-conversion-errors-ffi-literals-correction-20261008.md`): § 8 defines `C::bool` and C character-literal shaping by code-point value under the active ABI model, with no implicit typed C/Sec scalar conversion; C character-literal shaping is not yet implemented. MD-020/021/023/024 (2026-10-09, `rules/corrections/applied/md020-md021-md023-md024-ffi-c-bindings-null-correction-20261008.md`): lowercase `c::` is the only C qualification, the platform C binding environment is selected by the CompilationPlan without imports and fails closed, target-sized and 128/256-bit integers and closed enums need ABI proof, and `null`/`is null`/`is not null` are unsafe-only; the compiler still uses `C::`, rejects `is not null`, and accepts `uint`, `int128`, and closed enums in foreign signatures, and the binding-environment format remains open (MD-021). |
| `platform/fixed-address-bindings.md` | **Written** | Canonical `@address`, MMIO volatility, binding mutability, validation, overlap, and addressed-access semantics. Implementation is tracked by `frontend.fixed-address-bindings` in `governance/platform.yaml`. |
| `platform/abi.md` | **Written** | Canonical Sec, C, and system ABI families; plan-selected classification, call plans, signatures, fingerprints, MLIR staging, and separate-compilation compatibility. Implementation is tracked by `lowering.abi-model`. MD-023 (2026-10-09, `rules/corrections/applied/md020-md021-md023-md024-ffi-c-bindings-null-correction-20261008.md`): foreign `int`/`uint` and 128/256-bit integers require ABI proof per position. |
| `platform/target_profiles.md` | **Written** | Canonical Hosted, RTOS, and BareMetal profile families; capability activation, execution and safety policy, typed resource limits, derived profiles, immutable resolved identity, provenance, fingerprints, and compiler-consumer queries. Implementation is tracked by `platform.target-profiles` in `governance/platform.yaml`. |
| `platform/platform_model.md` | **Written — revision 1.1** | Canonical Target/Variant terminology, immutable CompilationPlan resolution, typed platform submodels, capabilities including `UTCWallClock`, source selection, fingerprints, diagnostics, and LSP invalidation. Implementation is tracked by `compiler.platform-model` in `governance/platform.yaml`. § 26 (MD-014, 2026-10-03): the target pointer width resolves `int`, `uint`, and plain `float` together.  Registered hosted targets now include linux-riscv64 and freebsd-armv7 (planned; decision 2026-10-04). |
| `platform/volatile.md` | **Written** | Canonical volatile physical-access semantics, mandatory `@address` region validation, explicit raw volatile operations, physical access contracts, optimizer invariants, representation eligibility, lowering, diagnostics, and tooling. Compiler-known RawPtr volatile methods, unsafe/non-void frontend validation, effect facts, and shared LSP exposure are implemented; target validation and lowering remain under `platform.volatile` in `governance/platform.yaml`. |
| `platform/hardware-register-access.md` | **Written** | Canonical logical hardware-register access, safe implicit observation, explicit `Read()`/`Write()`, shadow state, resource/endpoint identity, transaction planning, width/alignment/footprints, ordering/completion, access context, runtime mappings, faults, and verified IR/lowering. Implementation is tracked by `platform.hardware-register-access` in `governance/platform.yaml`. |
| `platform/inline_assembly.md` | **Written** | Canonical revision 1.0 inline-assembly operands, constraints, clobbers, effects, control-flow/stack boundaries, symbol dependencies, target restrictions, and lowering contract. Implementation progress is tracked by `platform.inline-assembly-v1` in `governance/platform.yaml`. |
| `platform/interrupts.md` | **Written** | Canonical interrupt identities/binding, ISR roots, priority/preemption/nesting/masking, classes and lifecycle, configuration capabilities, ISR-safe execution, concurrency/stack integration, startup/linking, diagnostics, tooling, and completion. Implementation is tracked by `platform.interrupts-v1` in `governance/platform.yaml`. |
| `analysis/isr_analysis.md` | **Written** | Compiler verification for profile-scoped interrupt safety using canonical analysis results; implementation status is tracked by `sema.isr-analysis`. |

This group is a central remaining language-closure block.

Storage, layout, ABI, FFI, volatile, registers, and interrupts must be designed
as one compatible model.

---

# 11. Modules, projects, initialization, and build

| Rulebook | Status | Notes |
|---|---|---|
| `projects/projects.md` | **Written — revision 2.0** | Canonical project model replacing `projects.txt` (2026-10-04): single manifest, human ProjectName and RFC 9562 ProjectUUID, structured `[version]` with format placeholders and Target overrides, visible manifest build numbers with transactional automatic increment (superseding the hidden `.sec/state/build-counters.toml`), Targets/Variants/profiles, compile-time parameters, and additive transactional `sec init` including `sec init module`; module semantics are delegated to `projects/modules.md`. Implementation status is tracked in `governance/tooling_projects.yaml`: discovery, UUID generation, the init scaffold with its `[version]` table, and the structured version model (parsing, validation, rendering, and Target overrides in `internal/project`) exist; validating discovered manifests, build numbering, `sec init module`/Target creation, dry run, and transactional init are pending. |
| `projects/modules.md` | **Written** | Canonical module identity, membership, imports, cycles, visibility, resolution, surfaces, separate compilation, and incremental-tooling model. Implementation progress is tracked by `frontend.modules`. CLI assembles canonical directory-module siblings before Sema; LSP retains declarations from recoverable sibling files. |
| `compiler/initialization.md` | **Written** | Canonical executable entry, runtime-free startup, initialization/shutdown plans, dependency ordering, startup rollback, static-destruction integration, target termination, and lowering/linking boundaries. Implementation is tracked by `compiler.program-initialization`. |
| `compiler/linking.md` | **Written** | Canonical link planning and final artifact semantics; implementation is tracked by `compiler.linking`. |

The following must eventually be defined coherently:

```text
module identity
import resolution
internal visibility
cross-module initialization coordination
deinitialization order
entry points
generic symbol identity
FFI linkage
dead stripping
multiple target outputs
```

---

# 12. Diagnostics, formatter, and compiler tooling

| Rulebook | Status | Notes |
|---|---|---|
| `tooling/diagnostics.md` | **Written** | Revision 2.0 separates permanent ID namespaces from topic-family metadata, preserves published and retired identities, adopts the implemented `L` lexer namespace, defines canonical definition/occurrence/fix schemas, makes `sec diagnostics <ID>` the detailed lookup surface, and defines no source-level suppression syntax for Sec 0.1. Registry listing, JSON catalog/schema export, detailed known-ID lookup, structured parser transport, growing semantic-ID migration, LSP code transport, and mandatory S3001 are implemented; occurrence unification, full ID migration, localization, policy resolution, emitted-occurrence JSON, complete dead/unused analysis, and end-to-end fixes remain partial under `errors.diagnostics` in `governance/errors_diagnostics.yaml`. § 31(8) publishes L1021 (MD-005). |
| `tooling/formatter.md` | **Written** | Revision 2.0 defines canonical, project-configured formatting with stable paragraph IDs, `format_version = 1`, lossless error-tolerant syntax requirements, complete Sec surface layout, malformed-region preservation, and an opt-in grammar-aware Language Corrections layer. The shared CLI/LSP formatter, explicit-file check/stdin modes, lexer-backed CST foundation, delimiter handling, selected spacing/alignment behavior, CST-owned line attachment for parser-verified `@noCopy`/`@noPanic` attributes, required trailing commas for parser-valid multiline callable parameter lists, parser-owned compact range/slice plus contextual `step` spacing, and CST-anchored struct-field type/tag alignment exist; the full layout/configuration/correction/conformance contract remains partial under `tooling.formatter-v2` in `governance/formatting.yaml`. § 27(36) state-test corrections (MD-006) are implemented in the explicit fix layer. MD-013 (2026-10-03): trailing comments no longer end alignment groups, occupy the structural column one standard space after the widest cell, and register fields align through the shared field engine.  Multiline parameter lists are normalized: deliberate multiline lists get one parameter per line, a trailing comma, and `)` on its own line; mixed lists collapse to one line when they fit 120 columns (§ 16(2a), 2026-10-04). Parser missing-token and skipped-token recovery is attached to the CST as synthetic zero-width and real-element-range nodes, and the missing-separator correction consumes them (2026-10-09, `tooling.cst-lexical-foundation`). Project-owned manifest configuration (§ 4) awaits MD-054, and detached comments in import regions (§ 13) await MD-055. |
| `tooling/testing.md` | **Written** | Canonical source-level `test`, `*_test.sec`, `sec test`, compiler-known `testing.*`, subtests, integration tests, `TestCompilationPlan`, execution-provider, structured-result, LSP and editor-integration semantics. Sema validates `ExpectEqual`/`RequireEqual` with canonical equality rules (S1042-S1043) and `Pass`/`Fail`/`Skip` signatures (S1044); terminal flow, execution, and lowering remain open. Implementation progress is tracked by `tooling.language-testing`. |
| `compiler_diagnostics.md` | **Covered** | Compiler diagnostic policy remains canonical in `tooling/diagnostics.md`; avoid duplication. |
| `compiler/debug_information.md` | **Written** | Canonical Sec 0.1 debug-information model: None/LineTables/Full, source step points, truthful optimized values, ownership/lifetime availability, concrete generic identities, logical task/thread/process debug views, split artifacts/source correlation, target-neutral lowering, and compiler conformance. Implementation is tracked by `compiler.debug-information-v1` in `governance/compiler.yaml`. |
| `compiler/compiler_testing.md` | **Written** | Canonical compiler-verification model covering compiler test classes, structured diagnostic and recovery verification, Semantic IR and lowering checks, backend/artifact and executable conformance, fuzzing and mutation testing, deterministic isolation and contract-driven matrices, regression/corpus provenance, and debug-information conformance infrastructure. Implementation progress is tracked by `testing.compiler-verification-v1` in `governance/testing.yaml`. |
| `compiler/incremental_compilation.md` | **Written** | Canonical incremental-compilation contract for snapshot consistency, dependency-driven invalidation, cache validity and publication, module/source/provenance reuse, plan isolation, artifact reuse, history independence, and stateful conformance. Implementation remains planned under `compiler.incremental-compilation-v1` in `governance/compiler.yaml`. |
| `tooling/lsp.md` | **Living** | Canonical language-server architecture and feature rulebook; static and instance bindings are distinct, and impl static-removal advice is withdrawn. Nested member completion uses the active cursor and recoverable same-module declarations. Canonical nested standard-library imports resolve from `sec/stdlib` before project fallback instead of relying on a small hardcoded module list, while unresolved canonical imports produce mandatory `S1051` at the exact path in individual and batched diagnostics. Shared bidirectional source-position mapping now covers diagnostics, hover, navigation, references, highlights, call hierarchy, document symbols, edits, and semantic tokens using protocol UTF-16 coordinates, compiler scalar columns, multiline tokens, and LF/CRLF/CR normalization. Initial `textDocument/typeDefinition` navigation consumes Sema-resolved binding/expression types and declaration provenance, including same-module cross-file targets; compound-type component selection and the shared workspace index remain. Project-backed analysis now passes an unambiguous target variant's shared `ResolvedScalarPlan` to Sema; active target switching and distinct-plan multi-target diagnostics remain pending. Try hover now presents compiler-resolved local Option None recovery, including exhaustive or guarded-partial handler coverage and residual propagation. Property hover and completion preserve resolved accessor contracts, including a fallible setter's exact error type and explicit `try` requirement. Diagnostic refreshes are coalesced per module with obsolete-result rejection; in-flight analysis cancellation and cross-feature caching remain pending. Workspace symbols (`workspace/symbol`, Ctrl+T) project the document-symbol declarations of every workspace and open source with case-insensitive subsequence ranking, editor-context `_`/`__` visibility, read-only core marking, and cached per-file summaries (2026-10-03). Member completion in a direct return value, including property getters, offers only members of the expected type and omits the getter's own property, and `==`/`!=` operands, subject-switch `case` items, and match-arm patterns offer only (uncovered) values of the subject or left-operand type (decision 2026-10-03); proven missing commas get an "Insert missing ','" quick fix (2026-10-03). Inlay hints show inferred `let` types, parameter names of resolved multi-argument calls, and ownership effects the source does not spell (implicit `ref`/`ref mut` call-site borrows, `moves if selected` match bindings, semantic copies), individually configurable (2026-10-04). Implementation remains partial as detailed in governance.  Module assembly and imports select only files whose `#target` matches the active target (the document's own, the project's, or the host), so platform-specific files of one module may declare the same names (2026-10-04).  An interface conformance code lens summarizes each interface's implementations across every target of the module's platform files (2026-10-04).  Target-independent documents in modules with platform files get multi-target diagnostics with applicability on open and save (2026-10-04). |

---

# 13. Canonical rulebook set: written

The following rulebooks are currently considered written.

```text
memory/allocation.md
collections/collections.md
memory/arena.md
foundations/attributes.md
concurrency/atomics.md
concurrency/await.md
concurrency/blocking.md
memory/borrowing.md
concurrency/cancellation.md
analysis/call_graph.md
concurrency/channels.md
compiler/compiler.md
compiler/compiler_analysis.md
compiler/compiler_pipeline.md
compiler/compiler_testing.md
compiler/compiler_known_members.md
compiler/debug_information.md
compiler/incremental_compilation.md
compiler/linking.md
concurrency/concurrency.md
concurrency/concurrency_memory_model.md
concurrency/concurrency_runtime_model.md
memory/copy_move.md
library/core-library.md
control-flow/defer.md
types/default_values.md
memory/destruction.md
tooling/diagnostics.md
declarations/enums.md
errors/errorhandling.md
analysis/effect_analysis.md
concurrency/events.md
platform/ffi.md
platform/abi.md
control-flow/flowcontrol_for.md
control-flow/flowcontrol_if.md
control-flow/flowcontrol_match.md
control-flow/flowcontrol_switch.md
control-flow/flowcontrol_while.md
tooling/formatter.md
tooling/testing.md
declarations/functions.md
declarations/lambda-functions.md
declarations/generics.md
compiler/generics_lowering.md
compiler/monomorphization.md
declarations/impl.md
declarations/interfaces.md
analysis/isr_analysis.md
foundations/language_philosophy.md
foundations/lexical_structure.md
memory/layout.md
memory/lifetime_analysis.md
memory/memory_model.md
mlir/mlir-optimize.txt
mlir/mlir.txt
mlir/sec_mlir.md
mlir/sec_mlir_dialect.md
mlir/sec_mlir_lowering.md
concurrency/mutex.md
foundations/operators.md
memory/ownership.md
errors/panic.md
concurrency/ipc.md
concurrency/processes.md
projects/projects.md
projects/modules.md
declarations/properties.md
memory/raw_pointers.md
memory/reference_model.md
memory/references.md
declarations/registers.md
platform/fixed-address-bindings.md
platform/volatile.md
platform/hardware-register-access.md
platform/interrupts.md
platform/inline_assembly.md
compiler/rules_implementations.txt
errors/runtime_checks.md
concurrency/scheduling.md
concurrency/select.md
compiler/semantic_ir.md
concurrency/spawn.md
declarations/spread.md
declarations/static.md
memory/storage.md
declarations/struct.md
concurrency/tasks.md
concurrency/threads.md
memory/transferability.md
types/types.md
memory/unsafe.md
declarations/unions.md
types/units.md
types/contracts.md
tooling/lsp.md
foundations/grammar.md
compiler/parser_recovery.md
compiler/compile_time_evaluation.md
```

The following newer rulebooks are also written and present in the canonical
repository state:

```text
collections/collections.md
collections/shaped-types.md
concurrency/thread_local.md
control-flow/discard.md
analysis/escape_analysis.md
analysis/closure_analysis.md
analysis/parameter_usage_analysis.md
analysis/pitfall_analysis.md
analysis/stack_analysis.md
analysis/data_races.md
analysis/deadlock_analysis.md
analysis/isr_analysis.md
platform/interrupts.md
platform/inline_assembly.md
```

The package-local MLIR documents under `mlir/packages/`, historical dialect and
lowering snapshots, Semantic IR amendments, and normative synchronization notes
are indexed as document groups in section 9 rather than duplicated file by file
in this canonical-rulebook list.

---

# 14. Canonical rulebook set: planned

No canonical Sec 0.1 rulebook remains planned. The incremental-compilation
rulebook completed the previously open design-closure item; implementation
status remains independently tracked in governance.

The following remains a candidate until its value as a separate rulebook is
confirmed:

```text
library/stdlib.md
```

---

# 15. Deferred areas

The following areas are intentionally deferred and do not block immediate Sec
0.1 closure work:

```text
spawn process implementation
general process supervision
dynamic-rank tensors
arbitrary user-defined operator overloading
```

A deferred area must still be listed so it is not mistaken for an accidental
omission.

---

# 16. Open design-space register

This section tracks unresolved language surface rather than missing documents.

## Panic and runtime failure

`errors/panic.md` and `errors/runtime_checks.md` now define panic domains, containment,
cleanup, no-panic defer and destruction, foreign-boundary restrictions,
task/thread outcomes, assertions, checked unreachable, checked-operation
outcomes, typed fallible paths, and the no-mandatory-runtime model.

Still to decide or lock in their owning rulebooks:

- exact build-manifest syntax for root panic strategy and required no-panic
  entrypoints;
- exact supervisor source syntax in the concurrency rulebooks.

## Storage, layout, and ABI

| `memory/storage.md` | **Written** | Canonical revision 2.0 storage origin, backing relation, reclamation authority, address stability, regions, invalidation domains, epochs, pin/protection state, memory spaces, placement, concurrency, Semantic IR, lowering, and diagnostics. Implementation progress is distributed across `analysis.storage` in `governance/analysis.yaml`, and `frontend.storage` plus `semantic-ir.storage` in `governance/memory.yaml`; the lowering, platform, and tooling entries remain in `implementation-status.yaml`. |
authority, address stability, memory spaces, invalidation domains, validity
epochs, and storage-domain transitions. `memory/layout.md` now defines semantic and
native layout, alignment, padding, stride, field order, packing and endianness
semantics, aggregate representations, layout compatibility, and plan-specific
layout queries.

Still to decide or lock in the owning syntax, ABI, FFI, and target rulebooks:

- final source syntax for explicit layout, packing, alignment, field offsets,
  endianness, and memory-space contracts;
- exact FFI-stable representation contracts and their source attachment;
- concrete `ABIModel` definitions and classification algorithms for each supported target ABI.

## Attributes and effects

`foundations/attributes.md` now defines the closed Sec 0.1 attribute inventory and excludes
custom user annotations. `analysis/effect_analysis.md` defines inferred effects,
verified guarantees, interface compatibility, conservative indirect effects,
and per-compilation-plan propagation.

Still deliberately deferred:

- effect-constrained function-type syntax;
- generic effect-constraint syntax;
- unsafe foreign effect-declaration syntax;
- inline-assembly effect-declaration syntax;
- future `noSuspend`, purity, determinism, arena-split, and visible-region
  source forms.

## Platform and hardware

`foundations/attributes.md` now defines target-selection and interrupt-binding
attribute syntax. `platform/target_profiles.md` defines canonical profile
families, activation, policy, resources, resolution, provenance, and typed
compiler queries. `platform/volatile.md`, `platform/fixed-address-bindings.md`,
and `platform/hardware-register-access.md` now define the volatile, MMIO-binding,
and hardware-register transaction model. `platform/interrupts.md` defines the
canonical interrupt model, while `platform/inline_assembly.md` defines the
target-specific machine-operation boundary. These books consume shared platform
facts without redefining hardware access. Still to decide in the remaining
platform rulebooks:

- native platform views;

## Closures

Still to decide for Sec 0.1:

- immutable value capture only versus fuller capture support;
- mutable captures;
- reference captures;
- escaping environment storage;
- callable mutability;
- task/thread transfer.

The canonical rule is `declarations/lambda-functions.md`.

## Compile-time evaluation and generics

`compiler/compile_time_evaluation.md` now locks user-defined semantic CTE,
executed-effect restrictions, transient allocation, panic/failure handling,
loops/recursion and resource budgets. Const/value generics remain separately
excluded unless an owning rulebook introduces them.

## Collections and shaped types

Still to decide:

- literal syntax;
- final equality/hash contract names;
- dynamic owned tensor extents;
- layout policy syntax;
- memory-space syntax;
- final collection error types;
- sparse policy API.

## Modules and initialization

Still to decide:

- module identity and duplicate resolution;
- import cycles;
- explicit module/program initialization order beyond compile-time static dependencies;
- deinitialization;
- initialization failure;
- symbol identity and mangling.

## Explicit Sec 0.1 exclusions

The following must be either included or explicitly marked as excluded:

```text
macros
general-purpose structural compile-time reflection and declaration metaprogramming
runtime reflection
tuples
multiple return values
variadic functions
default parameters
named arguments
implicit conversions
inheritance
exceptions
garbage collection
dynamic-rank tensors
general user-defined operator overloading
```

---

# 17. Language-design closure criterion

Sec 0.1 may be declared design-complete when:

1. every expected rulebook is written, covered, merged, or explicitly deferred;
2. no canonical rulebook leaves programmer-visible syntax or behavior as an
   unresolved placeholder;
3. no two canonical rulebooks contradict each other;
4. every runtime failure has a typed error, panic path, trap, or compile-time
   diagnostic;
5. every target-dependent feature has explicit capability and fallback
   semantics;
6. core and stdlib responsibilities are defined for every compiler-known type;
7. a canonical grammar and operator precedence table exist;
8. the explicit exclusions for Sec 0.1 are recorded;
9. documentation status remains separate from compiler implementation status.

After design closure, implementation may continue phase by phase without
reopening language semantics unless an implementation finding proves a genuine
design defect.
