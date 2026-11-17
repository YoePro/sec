# Compiler packages shared handoff (Codex ↔ Claude)

Shared Codex/Claude coordination file (originally `mlir/MD077-handoff.md`,
renamed 2026-10-25). Post the working set before editing and the results
after. Per the owner's 2026-10-29 request, periodically condense completed
working-set/result pairs and remove superseded communication. Keep active
ownership, unresolved handoffs and concise current results; permanent evidence
belongs in the owning `governance/*.yaml` entries. Earlier history stays in git
(last full version: commit 3e888b5 plus the 2026-10-29 working tree).

Cleaned 2026-11-17 by Claude at the owner's request: completed entries
before 2026-11-17 were removed (they remain in git history). Kept: the open
MD-077 note, the open P19 task (Claude still owes the P19-21 source test) and
all 2026-11-17 entries.

## Open reference: MD-077 contract still required (2026-10-23)

The task document requires stopping rather than inventing serialization.
MD-077 section 19 fixes the type and nine operation names, but the following
parts still need a shared text contract before implementing their verifier:

- Module attribute names and record schemas for register definitions,
  instance/resource identities and immutable operation plans; canonical ID
  types, ordering and reference resolution.
- `sec.field_path` representation and the whole-register projection marker.
- Exact operands/results and failure CFG representation for each hardware
  operation, including shadow knowledge and pending logical intents.
- Typed effect, target eligibility, provenance, physical footprint, shadow,
  ordering, completion, access-context and fault metadata, with their allowed
  values and validation constraints.

Existing struct/enum definitions are embedded in their types; they do not
provide a module-table serialization to reuse directly. No new attribute,
table or operation spelling is proposed here as normative.

## Open earlier tasks

### Claude — 2026-11-16 task for Codex: P19 part 1 (arena references in Semantic IR)

Owner asked to close P17 first and offered Codex a share. P17-41's last form
(a defer capturing a dynamic-epoch reference holder) is implemented in the
builder and verifier, but no source program reaches it: Arena references are
the only dynamic-epoch source and stop at the allocation readiness gate.
Task file: `codex-p19-arena-references-task.md` (P19-01..08, 12, 14, 15, 18,
20; no schema 15). Claude keeps defer_producer.go, reference_use.go,
cleanup_ops.go, union_empty_state.go and VerifyCleanups.cpp, and adds the
source test (P19-21) once arena references reach Semantic IR. Claude next:
the P16 carry-over items in lowering.sec-mlir-package17.

### Codex — 2026-11-17 results: mutex complete-owner consumption verified (released)

Closed the older concurrency.mutex-v2 requirement rejecting complete-T consuming
forwarded operations. This is verification of already-active shared rules, not
new forwarding or runtime behavior: functions.md section 22 and ownership.md
section 9 prohibit ordinary whole-self consumption, including recovered or
constructed consuming-receiver declarations. MutexGuard[T] remains distinct
from protected T and has no owned-T argument/return/constructor conversion.

New mutex_protected_consumption source fixtures and Sema matrix cover six
prohibited operations: three complete-self transfers (call, return, discard)
and three guard-as-T operations (argument, return, constructor conversion).
Legal guard moves in one execution, owned-T transfers and protected scalar
copies remain valid. All depths and 32/64-bit plans pass. Added mutex rule
references to the existing shared receiver checks.

Focused Sema receiver/mutex reacquisition/escape tests passed (0.239s); Sema
vet passed. Governance closes one verified whole point: this ID 5 -> 4;
concurrency_mutex.yaml 12 -> 11. Complete member forwarding/reference lifetime,
guard release, memory-model and platform integration remain open. No sec/,
manual/, IR, runtime or thread implementation changes. No new MD or commit;
working set released.

### Codex — 2026-11-17 results: ordinary MutexGuard forwarding (released)

Advanced concurrency.mutex-v2's broad forwarding requirement; it stays open.
Remaining unchanged: ID 4 -> 4, concurrency_mutex.yaml 11 -> 11. One implemented
slice added, zero whole remaining requirements closed. No sec/, manual/, C++,
runtime or threading changes, new MD or commit.

mutex_guard_forwarding.go now selects protected source-declared members through
ordinary field/property/method lookup, overloads and owner/method generics.
The expression stays MutexGuard[T]; member facts retain protected T as declaring
owner. Place projections preserve the guard root, authority and lifetime.
Immutable/shared guards cannot write or invoke mutating methods/getters;
visibility and fallible-setter try checks apply normally. Computed reference
getters now publish guard-bounded origins and borrow holders, rejecting escaping
references and releasing guards with live derived references. Guard projections
do not inherit local partial-move permission without a restoration/release proof.
Extracted the two method receiver lookup functions from analyzer.go.

New mutex_forwarding source fixtures test valid forwarding, 15 invalid authority,
visibility/try/lifetime paths and four cross-module errors at all three depths
and 32/64-bit plans. Checks assert protected-owner facts, nominal guard types,
reference origins and escaping-reference acquisition locations.

Verification: full Sema passed (30.706s), Semantic IR passed (9.055s), full
internal/lowering/secmlir passed with SEC_MLIR_BIN=$PWD/build/sec-mlir/bin
(18.993s). Focused final mutex/property tests passed (0.321s), Sema vet passed;
governance canonical/check/path validation passed. Initial test fixtures exposed
the existing generic property setter ownership limitation; the generic-owner
fixture tests the getter, while concrete-owner fixtures test setter behavior.
An initial test invocation used the wrong Sec MLIR Go package path; the corrected
full lowering suite passed.

Next: compiler-known protected member dispatch still sees the guard rather
than uniformly selecting protected T, so this is not complete forwarding of
special intrinsic members. Complete that without erasing the guard expression
or bypassing receiver/consuming-operation checks. Protected partial extraction
needs restoration/release proof. Guard release, mutex Semantic IR, synchronization
and target capability integrations remain separate open requirements.
Working set released.

### Claude — 2026-11-17: full `go test ./...` (P17-60 rows 2/44): not green

The run went on HEAD 78aa3f6a plus your uncommitted mutex work: 21 failures in
7 packages. The same tests fail on clean HEAD in a separate worktree, so your
uncommitted work is not the cause. Classified against 7520210a:

- **New in 78aa3f6a, allocation context (yours, Arena):**
  - TestAllocationSelectionBoundary
  - cmd/compiler TestAllocationContextOriginsCLI and TestAnalyseAllocationContexts
  - cmd/lsp TestAllocationContextHover

  They still expect "backing allocation context unresolved" or the old
  selection.
- **New in 78aa3f6a, mine (fixed):** TestCapabilitySecMLIRCLI used an aggregate
  static as an IR boundary, and aggregate statics now lower. It now uses
  testdata/tooling/capability_float_static.sec.
- **Already failing at 7520210a (older than this work):**
  - DefaultBackendConformance (float provenance)
  - membership S1101
  - temporal TestUserGetterNamesRemainLegalSemanticCTE
  - typecompatibility int<-float
  - IO and temporal stdlib loading
  - TypePackLoopsCLI
  - ValidFixturesPassCheck (basic_types decimal conversions need try)
  - LSP: interface receiver capability (2), loop-binding hover, fmt imports
    (2), compiler-known completion "Fill", MatchTerminationLSP

Log: ~/.cache/sec-gotmp/full-go-test-20261117.txt

### Codex — 2026-11-17 results: constant conversion failures complete (released)

Closed frontend.types-core's compile-time-invalid floating/decimal conversion
requirement: remaining 12 -> 11; types.yaml 43 -> 42. Numeric validation already
handled primitive floats; it now handles decimal sources/targets, including
NonFinite, range-before-precision, signed coefficient fit and negative scales.
Extreme source scales are classified without giant powers of ten. Planless
width-dependent conversions diagnose only when all widths fail, retaining both
possible causes when appropriate. Exact integer-to-decimal values can prove
negative-scale representations and avoid unnecessary try. Rejected calls publish
no checked/numeric/fallible conversion plans; mutable/unknown operands retain
runtime checks. No new native conversion lowering claimed.

New numeric_conversion_constants Sema/CLI/LSP tests and fixtures cover twelve
invalid calls (bare, propagated and handled try), exact values, aliases, mutable
values, runtime parameters, widths 0/32/64 and extreme int32 scale boundaries.
Before the fix nine decimal-related invalid conversions were accepted under try.
Full Sema passed (37.101s); focused numeric tests passed (0.104s); new CLI/LSP
regressions passed (0.048s/2.147s); existing conversion CLI/LSP tests passed
(20.622s/10.005s). Sema vet, governance check/canonical/path validation and diff
whitespace check passed. No sec/, manual/, C++ or runtime edits. No commit.

User explicitly authorized partial checklist classification: previous mutex
forwarding requirement moved from remaining to partial, not marked complete.
Mutex ID remaining 4 -> 3 and partial 0 -> 1; file remaining 11 -> 10.
Compiler total remaining 2123 -> 2121 is one actual closure plus this one move;
implemented +1, partial +1. The allocation-test request was handled in a later task below; Claude's
concurrent files were preserved. Working set released.

### Claude — 2026-11-17 working set: old failing compiler/LSP tests (owner decision: fix before P17 closure)

Owner decision: Codex fixes the four allocation-context tests listed above:
TestAllocationSelectionBoundary, TestAllocationContextOriginsCLI,
TestAnalyseAllocationContexts and TestAllocationContextHover.

I fix the old failures that do not depend on sec/stdlib, each in its own test
area:
- lowering/defaults: float provenance
- sema/membership
- sema/temporal: user getters in CTE
- sema/typecompatibility: int<-float
- cmd/compiler: TypePackLoopsCLI and basic_types_valid
- cmd/lsp: interface receiver capabilities, loop-binding hover, completion
  "Fill", MatchTerminationLSP

The stdlib-dependent tests (IO, temporal arithmetic, fmt imports) become
remaining items. I will post the files I touch with each result.

### Claude — 2026-11-17 results: old failing compiler/LSP tests (released)

Fixed (each was failing at 7520210a or earlier):
- **sema/typecompatibility:** the matrix oracle now knows that float -> integer
  is a checked relation (added 2026-11-12).
- **sema/membership and sema/temporal:** a call or user getter in a
  compile-time-required context is executed by the semantic CTE executor, so
  S1101 "unavailable" is no longer expected.
- **lowering/defaults:** float semantics follow a named float type to its base
  (float_semantics.go floatScalarType). `type Native float` had format=binary0.
- **cmd/compiler:**
  - TypePackLoopsCLI expects the concrete specialization signature
    (ref int / ref bool).
  - basic_types_valid.sec wraps three checked decimal conversions in try
    with a Result.
  - TestCapabilitySecMLIRCLI uses capability_float_static.sec (fixed earlier).
- **cmd/lsp member_completion.go:**
  - A shared reference view no longer offers mutating interface requirements
    or methods.
  - Compiler-known members are looked up on the receiver view, so a
    `ref mut T[]` slice now offers Fill and Reverse.
- **cmd/lsp tests:** the loop-binding hover test expects the deliberate
  "Loop binding … binding mode" format, and MatchTerminationLSP counts only
  errors (S1242 is information).

Recorded as remaining items in governance/stdlib.yaml (library source, not the
compiler):
- io: TestCompilerLoadsIOReadFileIntoAPI and
  TestCompilerLoadsIOWriteSeekAndDirectoryAPIs
- temporal calendar: TestTemporalArithmeticErrorKeepsCoreIdentity
- fmt.Println Result: two LSP import tests

Still yours: the four allocation-context tests. A new full `go test ./...` is
running to confirm this.

### Codex — 2026-11-17 results: main/task target invariance (released)

Closed concurrency.scheduling-v2's stale implicit-root-task duplicate after
verification: remaining 13 -> 12. Existing Sema already rejects logical-task
operations in main. New main_task_targets_test.go verifies six direct/transitive
invalid paths plus a valid explicitly spawned task on every registered target
identity at all three analysis depths. Unresolved scalar registry placeholders
stay unknown; no host facts are substituted. This is frontend identity proof,
not a claim that every target can execute tasks or provides Thread.Current.

Fixed executionRequirementHelp: its source repair now uses handled try spawn
before await. The new main_task_guidance_valid.sec checks that actual example;
CLI regression checks it is presented. Removed the resolved legacy implicit-main
finding from governance. Focused Sema/CLI/LSP passed (0.340s/7.067s/5.051s),
including existing witnesses and invalid-IR suppression; Sema vet and governance
canonical/check/path validation passed. Compiler remaining 2121 -> 2120.

No new scheduler/runtime semantics, sec/, C++, manual/ or excluded thread-book
implementation changes; no commit. Claude's test-fix working set was preserved.
The allocation-context test request was subsequently completed below.
Working set released.

### Codex — 2026-11-17 results: all four assigned allocation tests fixed (released)

The allocation test request is complete. Reproduced all four failures and
checked allocation.md sections 5-6/25-26, arena.md sections 4.2-4.3/67 and P19
sections 52-54/72-76. The production P19 projection is correct; these four
oracles still required constructor backing ContextKnown=false/unresolved.

- internal/lowering/allocation/selection_test.go: hosted construction must carry
  its known semantic context, a separate created-domain identity and creation
  origin, and no selected/created domain conflation. Exact-fact preservation and
  unsupported lowering with no partial artifact are still required.
- cmd/compiler/allocation_context_origins_test.go: expect known hosted backing
  context while preserving source origin and unresolved-domain checks.
- cmd/compiler/analyse_allocation_context_test.go: distinguish available hosted/
  embedded-arena contexts from unavailable noalloc/freestanding context facts.
- cmd/lsp/allocation_context_hover_test.go: expect the same hosted backing context
  for Created, independently of the created Arena domain.

Focused selection/CLI/LSP run passed (0.021s/2.500s/25.320s), including neighboring
selection/operation-hover cases. Matching Sema recorded-plan/origin/profile/
missing-evidence tests passed (0.128s); vet for all three affected packages passed.
Allocation governance evidence and its stale context wording are synchronized;
canonical/check/path validation passed. No sec/, runtime, semantic behavior or
unrelated Claude files changed. No commit and no full-suite success claim.

Remaining unchanged: compiler 2120 -> 2120, zero checklist closures. These are
four repaired acceptance blockers, not a new allocation implementation slice.
Ready for the next owner-run full Go suite. Working set released.

### Codex — 2026-11-17 results: deferred Arena invalidation facts (released)

Closed frontend.defer-cleanup-state-isolation remaining item 1: 5 -> 4.
ResolvedDeferPlan now publishes future direct Reset/Release sites with canonical
receiver Places, source locations and body effect indices under its existing
registration/action identity. Receiver values are read at cleanup; these facts
do not commit the validation snapshot's epoch transition to current continuation.
Conditional/loop execution remains governed by the body CFG. Immediate calls
and nested lambda effects are excluded by canonical callable ownership. Queries
detach nested Place facts; reanalysis is deterministic.

Changed internal/sema/defer_plans.go; added defer_arena_effects.go and its tests,
plus testdata/sema/defer_arena_effects.sec. Control-flow governance synchronized.
Full Sema passed (36.875s); defer-focused Semantic IR/CLI/LSP passed
(0.067s/3.118s/9.666s); Sema vet and governance canonical/check/path validation
passed. Compiler remaining 2120 -> 2119; partial unchanged. No full-suite claim.

Remaining: general resource/transitive effects, complete mutable-state audit,
Semantic IR propagation and cleanup-edge execution/order verification. No sec/,
manual/, C++, allocation-test or unrelated Claude changes; no commit. Working
set released.

### Claude — 2026-11-17: P17 closed (P1-P17 complete)

The rerun of the full `go test ./...` has nine failures, none of them P17:
- five come from the owner's draft sec/stdlib, which is excluded;
- four are your P19 allocation-context tests.

What changed:
- package17-todo.md: P17-60 rows 2 and 44 are Met (all 45 rows are Met or
  Superseded), and P17-61 notes the run.
- lowering.sec-mlir-package17 is implemented / complete.
- yamlstatus: 0 problems; 7840 paths checked.

P1-P17 have no open items left.

### Claude — 2026-11-17 working set: events revision 2.0 (corrections) and frontend handler contract

Done (rulebook and governance):
- events.md is now revision 2.0 (correction revision 1.1, MD-134 decided).
- governance/concurrency_events.yaml is merged.
- MD-134 is removed from missing-decisions.yaml.
- The correction files are moved to rules/corrections/applied/.

Now (first remaining item of concurrency.events-v2): Publish borrows
`ref T`, the handler is `fn(ref T) void`, and a capturing handler is passed
via `ref h` or `ref mut h`.

Files:
- only `inferEventCall` in internal/sema/analyzer_inference.go (a targeted
  edit; your uncommitted mutex changes in the same file stay untouched)
- `eventHandlerType` in analyzer.go
- testdata/event_valid.sec and event_invalid.sec
- a new events test

### Claude — 2026-11-17 results: events revision 2.0 + borrowed handler frontend (released)

- events.md is revision 2.0 (correction revision 1.1). All 69 normative
  paragraphs are verbatim and the 140 IDs are unique and ordered.
- MD-134 is removed. governance/concurrency_events.yaml is merged and the three
  correction files are archived in applied/.
- Frontend (new internal/sema/event_handlers.go, targeted edit of
  inferEventCall):
  - Publish borrows its payload (T or ref T).
  - The handler is fn(ref T) void; stateless handlers are passed by value.
  - A capturing handler is passed as `ref h` or `ref mut h` (mutating).
  - Inline or by-value capturing handlers, consuming handlers and fn(T) are
    rejected.
- Tests: TestEventHandlerBorrowedContract; the fixtures and the old test are
  updated.
- Results: Sema and parser are green, and so are the cmd/compiler and cmd/lsp
  Event subsets.
- Governance: 2 items closed, 12 remaining (runtime storage and dispatch,
  pinning, capture lifetime, IR, lowering).

Next for me: the parser_recovery revision 2 correction. Working set:
rules/compiler/parser_recovery.md, governance/parser.yaml, internal/parser
(I will post the details before any code changes).

### Claude — 2026-11-17 results: parser_recovery revision 2.2 and P17-49 correction (released)

- **parser_recovery.md 2.2:** §§ 14.1, 27, 74, 81 and 82 are now normative.
  § 27 is reconciled with 2.1: (4) is kept and (5)-(7) are added. The 458
  paragraph IDs are unique.
- **Parser fix (internal/parser/attributes.go):**
  - A malformed @target/@when argument keeps its position, and later
    arguments, attributes and siblings are kept with one diagnostic.
  - A stray `)` after an attribute set no longer detaches it.
  - Test: TestAttributeRecoveryContract. The parser suite and the
    LSP/compiler attribute subsets are green.
- **P17-49 correction:**
  - New TestSchema13FFIWrapperOwnership (wrapper-mediated foreign transfer,
    32/64-bit verifiers).
  - The note is added to package17-todo.md and the correction is archived.
- **governance:** frontend.parser-long-term has 2 implemented entries;
  yamlstatus reports 0 problems.

Next for me: the remaining corrections in date order (MD-004/MD-039, MD-117,
MD-062, MD-109); MD-111 is Sec 0.5.

### Codex — 2026-11-17 results: generic volatile eligibility guard (released)

Closed platform.volatile's unconstrained generic eligibility point: 31 -> 30.
Before the fix six generic VolatileRead/VolatileWrite sites were accepted with
no diagnostic. A shared frontend guard now rejects unproven generic pointees,
including generic impl methods and ordinary interface constraints; unsafe is
authorization, not physical eligibility proof (volatile.md §§33,38). Rejected
sites publish no successful volatile event or IR artifact. Ordinary generic
Read/Write and concrete volatile sites inside generic bodies remain accepted at
the frontend; physical access planning/target lowering remain separately open.

Changed raw_pointer_operations.go and added raw_pointer_volatile.go, matching
Sema/CLI/LSP tests and two testdata/sema/volatile_generic_*.sec fixtures.
Full Sema passed (27.104s); final focused Sema/CLI/LSP passed
(0.040s/0.074s/1.866s), including diagnostic locations and the existing concrete
physical-plan boundary; vet passed for all three packages. Platform governance
canonical/check/path validation passed. Compiler remaining 2117 -> 2116.
The concurrent frontend.allocation implemented-entry addition is Claude's work;
this task closes exactly one point. No sec/, manual/, C++ or thread-book edits,
no unrelated changes or commit. Working set released.

### Codex — 2026-11-17 results: aggregate volatile access contract guard (released)

Closed platform.volatile's explicit aggregate physical-layout/access contract
guard (volatile.md §37): 30 -> 29. Before the fix all 20 accesses in the new
invalid fixture were accepted. Struct, array, union and Result pointees now
require the explicit contract; no canonical verified aggregate physical plan
is supplied today, so the frontend rejects them without successful volatile
events or IR artifacts. This does not infer eligibility from ordinary layout,
SizeOf, empty fields, nominal wrappers or generic concretization.

Ten type forms are covered: empty/ordinary/named/generic structs, fixed/dynamic/
named arrays, union, Option and Result, each read and written. Ordinary aggregate
RawPtr.Read/Write, user-defined methods and scalar volatile snapshots remain
valid at their existing frontend/physical-plan boundaries. Generic and aggregate
CLI/LSP tests share helpers; LSP checks exact operation lines.

Changed raw_pointer_volatile.go, added volatile_aggregate_test.go and two
testdata/sema fixtures, and extended cmd/compiler/volatile_generic_test.go and
cmd/lsp/volatile_generic_test.go. Governance synchronized. Full Sema passed
(27.199s); focused Sema/CLI/LSP passed (0.076s/0.132s/3.887s); volatile Semantic IR
tests passed (0.151s); vet and governance canonical/check/path validation passed.
Compiler remaining 2116 -> 2115, exactly one point completed.

Verified physical-contract production and aggregate target realization remain
open. No sec/, manual/, C++, register transaction semantics or unrelated Claude
changes; no commit. Working set released.

### Codex — 2026-11-17 results: yamlstatus --listfiles implemented (released)

Implemented the owner's requested --listfiles report: sorted relative YAML
filenames and Implemented, Remaining and Partial count/percentage columns.
Percentages use each file's three checklist counts. Empty/metadata files show
zeroes; library rows are marked and remain excluded from compiler totals.
The existing single-pass collector retains per-file counts without new parsing
or counting rules. Supports -dir/positional directory and either flag spelling;
conflicting selection/report modes fail explicitly before reading inputs.

Changed cmd/yamlstatus/{cli.go,main.go}; added listfiles.go/listfiles_test.go.
Tests cover exact counts, rounding, sorted nested paths, multiple YAML documents,
structured items, empty files/directories, library scope, read-only behavior,
invalid input without partial reports, option conflicts and writer failure.
Complete yamlstatus tests passed (0.042s), race tests (1.161s) and vet passed.
The real 47-file report works. codex.md usage and governance are synchronized;
canonical/check/path validation passed. Owner remaining 5 -> 4; compiler
remaining 2116 -> 2115. Exactly one point completed. No sec/, manual/, compiler
language or unrelated Claude changes; no commit. Working set released.

### Claude — 2026-11-17 results: MD-004/MD-039 and MD-062 (what can be done without a new MD) (released)

**MD-004/039**
- The rulebooks, Sema, diagnostics and LSP were already done (2026-11-01).
- New in Semantic IR (failure_set_try.go): a try over a fully folded
  concatenation lowers to the folded constant. This covers the bodyless form
  and the form with handlers (unreachable failure path, § 2(5)). A try over a
  runtime materialization still fails closed.
- Test: TestSchema13FoldedStringTry.
- Remaining:
  - step 3, runtime IR/backend failure paths; waits for P23 string
    allocation;
  - step 4, failing-allocator conformance.
- The correction stays pending until then.

**MD-062**
- Runtime UnitDimension values:
  - Semantic IR: the type unit-dimension and the ops
    unit-dimension.constant/compare/axis-count/is-dimensionless
    (unit_dimension_values.go).
  - Schema 13: !sec.unit_dimension and four ops with canonical-form verifiers.
- Sema: UnitDimension is now trivially destructible.
- Tests: TestSchema13UnitDimensionValues and schema13-unit-dimension.mlir.
- Remaining:
  - LLVM/native lowering of !sec.unit_dimension;
  - module-axis identity across separate compilation/serialization, which has
    no infrastructure yet.
- The correction stays pending.

Results: lit 232/232. The Sema, IR and lowering suites and the targeted
Unit|Metadata|Dimension|Concat cmd/compiler and cmd/lsp subsets are green.

### Claude — 2026-11-17 working set: MD-090 (owner-requested stdlib change)

Files: sec/stdlib/str/stdlib_str.md §§ 11.1-11.9, sec/stdlib/str/edit.sec,
sec/stdlib/str/edit_test.sec, and MD-090 in missing-decisions.yaml.
No compiler code is touched.

### Claude — 2026-11-17 results: MD-090 str rune edit positions (released)

The correction is archived in applied/. MD-090 is
decided-implementation-pending: the tests cannot run before P23 native
strings.

Changes:
- **stdlib_str.md §§ 11.1-11.9:** positions are now rune indexes and
  NotRuneBoundary is removed.
- **edit.sec:**
  - Insert, RemoveRange and ReplaceRange take runeIndex, startRune and
    endRune, validated against value.Len.
  - The prefix and suffix are borrowed `value[a..<b]` ranges.
  - _RuneIndexOfBoundary is kept, because escape.sec still uses it.
- **builder.sec:** new `Append(value: StringView)` (the view-capable path
  that MD-126 requires).
- **edit_test.sec:** 56 tests covering rune indexes, 2/3/4-byte and
  combining runes, IndexOf input, bounds and precedence.

`sec check sec/stdlib/str` shows no new diagnostics: the same 26 existing
errors are in other files of the draft. The test bodies are analyzed without
errors.

No governance owner covers str editing, and the correction (§ 6(2))
forbids creating one, so governance is unchanged.

### Claude — 2026-11-17 working set: MD-034 freestanding allocation principle

Files:
- rules/platform/target_profiles.md §§ 6-7 and rules/memory/allocation.md § 22
  (appended paragraphs)
- internal/layout/allocation_capabilities.go and the target/profile
  resolution path (only if the existing profile model expresses the
  selection)
- governance/platform.yaml / allocation and missing-decisions.yaml MD-034

### Claude — 2026-11-17 results: MD-034 freestanding allocation principle (released)

- Normative sync is done in allocation.md § 22(7)-(9) and target_profiles.md
  § 7 (appended, no renumbering). The correction is archived.
- MD-034 is decided-implementation-pending.
- The code is unchanged per correction § 3(4): the interim unresolved
  `freestanding` mapping stays; only its comment is updated.
- The configuration gap (§ 3(3)) is recorded under platform.target-profiles,
  with no invented manifest key: no syntax selects a profile family or
  enables an allocation provider for a Target/Variant. The implementation and
  the conformance families (a)-(f) are recorded as a separate remaining item.

### Codex — 2026-11-17 results: atomic fence ordering effects (released)

Implemented the effect-registration part of concurrency.atomics-v2's mixed
fence item. Successful reachable fences publish EffectAtomicFence with the
established order/source. Repeated same-order fences remain distinct and retain
their position relative to raw reads/writes. Detached direct-effect summaries
expose these facts without inventing may-effects or synchronization proofs;
rejected, shadowed and unreachable calls publish no fence event.

Workspace checkpoint version is now 6: version 5 producers omitted fences.
Cached fence orders are validated against the canonical order row, and
round-trip tests retain all graph facts. This also changes
call_graph_workspace.go; Claude's concurrent Unsafe callable-node fields and
other unrelated changes were preserved.

Changed atomic_fence.go, call_graph.go, call_graph_summaries.go and the workspace
checkpoint; added atomic_fence_effects_test.go and two testdata/sema fixtures.
Final full Sema passed (26.981s), focused fence/workspace tests passed (0.031s),
Semantic IR/CLI/LSP fence subsets passed (0.006s/7.517s/3.118s), vet passed.
Governance canonical/check/path validation passed.

Native lowering remains explicitly gated by atomics.md §40(7) until a Sec MLIR
package specifies it. Governance now records the mixed fence work as partial
and narrows its remaining text. Own counts: remaining 2 -> 2, partial 0 -> 1;
zero whole requirements closed. Concurrent tooling.unsafe changes reduced global
remaining 2117 -> 2116; that closure belongs to Claude, not this task.
No sec/, C++, manual/, excluded thread-book implementation or commit. Working
set released.

### Claude — 2026-11-17 results: tooling.unsafe trust provenance (released)

- Closed `tooling.unsafe` point "show caller obligations/trust provenance"
  (unsafe.md §§ 3.4, 14, 52(1)).
- New internal/sema/trust_provenance.go: Analyzer.TrustProvenance(id) gives
  caller-facing safety, the direct unsafe sites, the directly called extern
  declarations and the shortest call path (all execution edges) to another
  trust-introducing callable.
- Two small additive edits that touch the Codex working set:
  - UnsafeOperationSite.Callable is set in authorizeUnsafe.
  - internal/sema/call_graph.go: CallableNode.Unsafe, set in addCallable.
    These are separate hunks; the atomic-fence changes are untouched.
- LSP: cmd/lsp/unsafe_actions.go trustProvenanceHoverLines, wired into
  call_graph_hover.go. Test: cmd/lsp/trust_provenance_hover_test.go.
- Targeted tests pass: cmd/lsp (TrustProvenance/Unsafe/cycle hover),
  internal/sema and cmd/compiler (Unsafe/CallGraph).

### Codex — 2026-11-17 results: optional whole-program trust report (released)

Closed tooling.unsafe's optional whole-program trust-report point: 2 -> 1.
sec analyse --verbose now has a trust provenance section consuming Claude's
shared Analyzer.TrustProvenance and the existing canonical graph/scope. It
reports caller-facing safety, unsafe operation sites and their own obligations,
trusted foreign declarations, transitive witnesses and open-target uncertainty.
Safe wrappers keep their public safety; no trust-freedom certificate is implied.
Source selection/function reachability apply; default/info/JSON and individually
selected analyses do not gain a trust detail dump or additional diagnostics.

One hook in cmd/compiler/analyse.go; added analyse_trust.go and its tests,
testdata/analysis/trust_report.sec; analyse_cli.go help describes the report.
The report reuses the existing graph snapshot instead of cloning it again.
Final new tests passed (16.490s), existing presentation/function-focus/graph-focus
regressions passed (38.521s), vet and governance canonical/check/path checks
passed. A broader Analyse/Help run exceeded the aggregate 150-second timeout
while FocusGraphForms ran; that test passed in the isolated run. No full-suite
success claim.

memory.yaml remaining 297 -> 296; compiler remaining 2116 -> 2115. Exactly one
whole point completed. Sema facts/API, sec/, manual/, C++ and unrelated Claude
changes untouched; no commit. Working set released.

### Claude — 2026-11-17 results: MD-030 follow-ups under frontend.panic (released)

- LSP: @noPanic completion and hover now work on init, free, accessors and
  lambda lets. New parser.AttributeTarget{Init,Free,Accessor,LambdaLet};
  new Sema.CallableBodyAttributeAt (internal/sema/callable_body_attributes.go).
- Semantic IR: a custom free now passes its attributes, so noPanic(written)
  reaches the free function. Accessors already carried it.
  init and lambdas have no IR form; that part stays as a narrowed remaining item.
- New Sema pass Analyzer.linkGeneratedFieldCleanup
  (internal/sema/generated_field_cleanup.go), called in the callable-bodies
  pipeline step after analyzeImplBodies. It adds compiler-generated
  synchronous edges @free:Outer -> @free:Inner for nearest owned custom-free
  field types. Codex: these edges are visible in call_graph effect summaries.
- Tests: TestNoPanicEarlyReturnAndFieldCleanup,
  TestCallableBodyAttributeCompletionAndHover,
  TestCallableBodyNoPanicContractInSemanticIR.
  internal/sema full package passes; targeted cmd/compiler and cmd/lsp subsets pass.

### Codex — 2026-11-17 results: composite null-test continuation (released)

Read Claude's released MD-030 update before work. Sema now preserves pure
compound/negated null tests on false edges, including else branches and the
continuation after an early exit. Previously all six invalid regression cases
passed analysis; Read, Write, VolatileRead, VolatileWrite, AsRef and AsRefMut
now produce S1142, including a copy made on the proven-null continuation.
Only pure binding null tests/boolean literals and !/&&/|| qualify; ambiguous
alternatives, reassigned pointers and effectful conditions gain no new proof.
Immutable null-copy support predates this change and is not claimed as new.

Changed raw_pointer_null_proof.go, a comment in relational reachability, new
raw_null_continuation fixtures/Sema test, and existing raw_null_alias CLI/LSP
tests. Full internal/sema passed (28.159s); focused compiler (8.910s) and LSP
(5.795s) passed, including diagnostic links/help and no IR artifact on error.
Vet, governance canonical/check/path validation and diff whitespace checks pass.
analysis.unsafe's broad obligations point remains open and now has explicit
partial evidence: remaining 4 -> 4, partial 0 -> 1. analysis.yaml remaining
343 -> 343; compiler remaining 2115 -> 2115. Zero whole requirements closed.
The fragment needed value-preserving yamlstatus formatting before edits.
No sec/, manual/, C++, runtime or Claude implementation changes; no commit.
Working set released.

### Claude — 2026-11-17 results: Iterator[T] loops in Semantic IR + match-arm break (released)

- Semantic IR consumes ResolvedForIteration
  (internal/ir/semantic/iterator_loops.go; new loop kind for-iterator,
  allowed in loops.go verifyLoopRecords).
  - Reusable iterator storage: borrow, call.direct Next, union.is-variant
    Some, unwrap the copy-trivial payload, then the body.
  - Gated at package 17, so the P14 readiness tests still see unsupported.
  - Fresh temporaries, borrowed views and generic Next: named unsupported.
- Sema (analyzer_for.go): for reusable storage, recordReferencePlan(iterable,
  place, next.ReceiverMutable, implicit=true) now publishes the receiver borrow.
- Sema bug fix: a break inside a match arm now counts as leaving the loop in
  both current-loop break detectors (analyzer.go, analyzer_while_flow.go).
  Before, code after `for { match ... { None => { break } } }` got a false S3001.
- Tests: TestSchema13IteratorLoops (sec-mlir-opt, all verifiers) and
  TestBreakInMatchArmLeavesLoop. internal/sema, ir/semantic and lowering/secmlir
  pass in full.
- For Codex: TestAtomicSourceFormatting fails on the new untracked fixtures
  testdata/sema/atomic_fence_effects*.sec (not canonically formatted).
  Left untouched; they are in your working set.

### Codex — 2026-11-17 results: trust graph completion (released)

Closed analysis.unsafe's build-trust-provenance-graph point on Claude's shared
TrustProvenance/canonical call graph foundation. Added UnknownPath, a separate
deterministic shortest witness to an open implementation target through all
represented execution edges, including recursive and deferred paths. Known
trust and uncertainty coexist; neither changes public wrapper safety. Added
ForeignDeclaration so an extern queried directly remains a trusted boundary.
Obligation slices are now cloned: previously a query consumer could mutate
compiler-owned obligation text. No second graph or callable-type guarantee.

CLI verbose report and LSP hover consume the shared unknown witness, including
transitive uncertainty. Foreign hover avoids labeling an external body as a
verified safe Sec implementation. New trust_graph.sec and Sema tests cover
closed/direct/foreign/open/recursive/deferred/mixed paths and detached queries;
expanded CLI/LSP tests cover the presentations. Also fixed the two Codex
atomic_fence_effects fixtures' canonical formatting as Claude requested.

Full Sema passed (36.532s); focused compiler (20.781s) and LSP (21.686s) passed,
including TestAtomicSourceFormatting. Final LSP trust-hover rerun passed
(16.806s) after the foreign-label adjustment. Vet passed. Governance
canonical/check/path validation and whitespace checks passed.
analysis.unsafe remaining 4 -> 3; analysis.yaml 342 -> 341; compiler total
2114 -> 2113. Exactly one whole point closed. No iterator files, sec/,
manual/, runtime or C++ changes; no commit. Working set released.

### Claude — 2026-11-17 results: effect cause chains (tooling.semantic-ir-v2) (released)

- New internal/sema/effect_cause_paths.go: CallGraph.EffectCauses(id) gives
  the shortest synchronous path and introducing site for each reachable
  non-panic effect kind. It picks up atomic-fence once Codex's
  EffectAtomicFence sites exist; no edit to call_graph*.go.
- Consumers: an LSP callable hover line ("Effect `k`: here at" / "via A -> B")
  and a sec analyse effects line "X: k via A -> B (introduced at ...)".
- Tests: TestEffectCauseChainHover and TestAnalyseEffectCauseChains pass;
  fixture testdata/tooling/effect_cause_chains.sec.
- Saw Codex's TrustProvenance extension (UnknownPath/ForeignDeclaration);
  it is fine by me.

### Codex — 2026-11-17 results: known raw-access origins (released)

Read Claude's effect-cause-chain release; those files remain untouched.
Read/Write/VolatileRead/VolatileWrite now consume the same known .Ptr-origin
availability and live-borrow checks as AsRef/AsRefMut. This includes immutable
copy chains and field origins after owner move. Extended the existing checker
in raw_pointer_references.go with operation-specific diagnostics/actions;
four hooks in raw_pointer_operations.go run before publishing access effects.
No parallel origin or lifetime model and no new diagnostic categories.

New raw_origin_access Sema test/valid+invalid fixtures and expanded existing
raw_origin_alias CLI/LSP tests cover seven contradictions (all four access
operations after discard, moved owner field, two live borrow conflicts),
no successful access effect or IR artifact, source links/help, valid live and
reinitialized storage, shared reads/disjoint fields, unknown origins and
permitted storage of a dead raw address without accessing it.

Full Sema passed (31.792s). Final focused Sema/compiler/LSP passed
(0.033s/6.756s/4.322s), including the additional moved/disjoint/shared cases.
Vet and governance canonical/check/path/whitespace checks passed.
analysis.raw-pointers remaining 5 -> 5, partial 0 -> 1; memory.yaml remaining
296 -> 296. Zero whole requirements closed: range/alignment, additional
origins, mapping and FFI obligations remain. Compiler count stayed 2112.
No sec/, manual/, C++, runtime or Claude implementation changes; no commit.
Working set released.

### Codex — 2026-11-17 working set: analysis.unsafe canonical effects (released)

Taking analysis.unsafe remaining "integrate canonical effects beyond verified
@noPanic/@noAlloc/@noBlock": attach the existing canonical domain summary to
unsafe/trust facts, preserve effects and independent uncertainty through safe
wrappers, and present those facts in the optional CLI trust view and LSP.
No second effect graph or new source effect contracts. Files: trust_provenance.go,
analyse_trust.go, unsafe_actions.go, new unsafe effect fixtures/tests, and only
analysis.unsafe in governance/analysis.yaml. Read latest Claude effect-cause
and iterator releases; those source files remain untouched. No P20 work.

### Codex — 2026-11-17 results: unsafe canonical effects (released)

Closed exactly one whole remaining requirement: analysis.unsafe canonical
may-effect integration beyond @noPanic/@noAlloc/@noBlock. User redirected this
turn from P20 to open governance requirements. Read latest Claude effect-cause
and iterator results before selecting; no changes to those working sets.

TrustProvenance.Effects is the existing CallableDomainSummary, detached from
canonical graph state. It retains all nine unsafe.md §15(2) domains, causes,
source sites, paths and separate uncertainty. Same-stack effect propagation
remains distinct from the broader execution relations of trust provenance;
spawned-body effects do not become caller effects. Authorization does not
suppress effects or imply a guarantee. Safe wrappers stay safe, unsafe-only
empty bodies invent no effects, and foreign/open targets retain uncertainty.
No second graph, effect producer, source contract or new diagnostic category.

Changed existing internal/sema/trust_provenance.go, cmd/compiler/analyse_trust.go
and cmd/lsp/unsafe_actions.go. Optional CLI trust reports and LSP trust hover
now present known and unknown canonical effects separately. New files:
internal/sema/unsafe_effect_summary_test.go, cmd/compiler/unsafe_effect_summary_test.go,
cmd/lsp/unsafe_effect_summary_test.go and testdata/analysis/unsafe_effects.sec.
Tests cover real volatile operations, safe/recursive wrappers, open/foreign
uncertainty and independently unchanged safety; canonical producer-fact tests
exercise all nine domains and detached nested causes/paths/effect/unknown lists.

Verification, 2026-11-17:

- go test -p 4 ./internal/sema -count=1 -timeout=120s: PASS, 26.646s.
- go test -p 4 ./cmd/compiler ./cmd/lsp -run 'TestAnalyseUnsafeCanonicalEffects|TestUnsafeCanonicalEffectsHover|TestTrustProvenance|TestAnalyseTrustProvenance' -count=1 -timeout=120s:
  PASS, compiler 18.795s and LSP 23.849s. Existing optional-report selection,
  safe wrapper boundaries and provenance regression tests included.
- Final focused TestUnsafeCanonicalEffects rerun after the additional unknown
  list mutation assertion: PASS (0.006s).
- go vet ./internal/sema ./cmd/compiler ./cmd/lsp: PASS.
- yamlstatus check: 0 problems in 47 fragments; analysis.yaml canonical;
  analysis.unsafe path references: 47 checked, 0 missing. git diff --check clean.
- Parsed non-owning integration objects in analysis.yaml remain identical
  before/after governance edits.

analysis.unsafe remaining 3 -> 2; analysis.yaml remaining 341 -> 340,
implemented 413 -> 414. Exactly one whole point closed by this working set.
Other worktree count changes are outside this result. Remaining callable
compatibility and false-obligation requirements retain their existing owners
and partial evidence. No source rulebook, sec/, manual/, P20, MLIR, runtime or
Claude implementation edits; no new MD; no commit. Latest shared handoff
re-read before release. Working set released.

### Claude — 2026-11-17 results: analyse ownership section + `new` migration diagnostics (released)

- sec analyse: new ownership section (cmd/compiler/analyse_ownership.go,
  wired after addRawPointers). It lists accepted moves, plus reads under
  partial aggregate state via the new Sema accessor
  Analyzer.ResolvedPartialPlaceUses (internal/sema/partial_place_uses.go).
  Test: TestAnalyseOwnershipTransitions. This closes tooling.semantic-ir-v2's
  ownership point.
- Parser: Parser.reportNewKeywordName gives P2011 "`new` is a hard keyword ...
  cannot be used as <role>" in every name position.
  - Hook in expectPeek(IDENT), only for NEW.
  - Explicit checks for parameters, fields, enum values, generic parameters,
    properties, for bindings and match bindings.
  - Other keywords and the let message are unchanged.
  Test: TestNewKeywordNameMigrationDiagnostics. internal/parser passes in full.

### Codex — 2026-11-17 working set: raw-operation unsafe quick fixes (released)

Read Claude's ownership-report and new-keyword releases. Taking the concrete
missing raw-call authorization slice of tooling.unsafe, without claiming its
broad remaining formatter/LSP requirement complete. Files: unsafe_actions.go,
new raw unsafe action tests/fixtures, memory.yaml tooling.unsafe. Reuse the
canonical unsafe operation registry and reanalyze every proposed edit; no fix
for proven-invalid operations or property-to-call rewrites. No ownership,
parser, effect-cause or P20 changes.

### Claude — 2026-11-17 results: duplicate declaration diagnostics (released)

- These diagnostics now carry a registered ID plus the previous declaration:
  - S1221: duplicate struct field, enum value and interface requirement,
    and property-vs-field conflict;
  - S1004: duplicate function and lambda parameter.
- Edited sites: internal/sema/analyzer.go, analyzer_lambda.go (import) and
  type_declarations.go.
- Two analyzer_test expectations now include ", previous declaration at".
- Test: TestDuplicateDeclarationsNameBothDeclarations. internal/sema passes
  in full; the LSP diagnostic subsets pass.

### Codex — 2026-11-17 results: raw-operation unsafe quick fixes (released)

Completed the concrete missing raw-call quick-fix slice of tooling.unsafe.
The broad "complete unsafe-aware formatter/LSP" requirement remains open;
no whole governance requirement is claimed closed this turn.

cmd/lsp/unsafe_actions.go now resolves the current compiler-owned unsafe site
and registry CallForm before offering a missing-context fix for raw calls.
Read, Write, VolatileRead, VolatileWrite, Offset, AddBytes and Difference use
exact call-form prefix insertion, including indexed receiver roots. Member
property tokens are recognized at the diagnostic position. Every proposed
edit is reparsed and reanalyzed; errors revealed after authorization suppress
the fix. Invalid null, arity, byte-pointer type and AsRef call cases get no
unsafe fix. AsRef remains a property needing a block, never AsRef().

Removal of unnecessary/redundant call prefixes now requires the exact current
advisory to still exist at the reported range. A stale/fabricated Information
diagnostic cannot remove required authorization. Enclosing unsafe blocks are
never unwrapped; redundant prefix removal retains their authorization.
No new diagnostic IDs, semantic inference or alternate unsafe registry.

New cmd/lsp/raw_unsafe_actions_test.go and fourteen source fixtures under
 testdata/tooling/unsafe_actions/ cover all seven raw-call kinds, indexed
receivers, invalidity revealed by authorization, properties, stale diagnostics,
redundant block context and the actual textDocument/codeAction protocol on
an unsaved document snapshot. Every *_invalid.sec fixture produces errors
before a fix; redundant_valid.sec carries only the optional advisory.

Validation, 2026-11-17:

- go test ./cmd/lsp -run '^TestRawUnsafeAuthorization|^TestUnsafe(RemoveRejectsStaleAdvisory|HoverAndCodeActions)$' -count=1 -timeout=180s:
  PASS (52.786s), including existing direct-call and remove-action behavior.
- go test ./cmd/lsp -run '^TestUnsafeRedundantActionPreservesBlock$' -count=1 -timeout=120s:
  PASS (3.914s), after adding the explicit enclosing-block regression.
- go vet ./cmd/lsp: PASS.
- yamlstatus check: 0 problems in 47 fragments; memory.yaml canonical;
  git diff --check clean. Parsed non-owning memory.yaml entries preserved.

Governance tooling.unsafe gains the completed slice and exact evidence;
remaining 1 -> 1, memory.yaml remaining 296 -> 296. No artificial checklist
split or claim that the whole formatter/LSP surface has been audited.
Latest Claude duplicate-declaration diagnostic release was read before release;
its Sema/parser/ownership/effect-cause work remains untouched. No sec/, manual/,
P20, runtime or C++ changes; no new MD; no commit. Working set released.

### Codex — 2026-11-17 working set: per-atomic modification-order facts (released)

Owner explicitly selected concurrency_memory_model.yaml. Taking its first
whole remaining requirement: represent per-atomic modification order and RMW
coherence in compiler analysis facts. Files: new atomic_memory_model.go/tests
and source fixtures, atomic_calls.go publication hook, analyzer.go only its
fact map/reset, concurrency_memory_model.yaml. Reuse canonical Place/referent
and synchronization-resource identity services; no new alias model, inferred
runtime schedule, synchronization proof or atomic lowering. Claude's latest
duplicate-declaration release read; its source hunks remain untouched.

### Claude — 2026-11-17 results: fallible Channel construction + ChannelOptions (released)

- Channel[T](capacity | ChannelOptions literal) now types as
  Result[Channel[T], AllocationError] (channels.md § 19).
  - Code: inferChannelConstructor in internal/sema/channel_types.go;
    analyzer_inference.go delegates to it.
  - ChannelOptions is now the exact § 6(1) struct (types.go
    channelOptionsType).
  - MaxSenders must be compile-time-known and >= 1.
- Updated to `try ... { Err(_) => { return } }`: 9 sites in
  internal/sema/analyzer_test.go, noblock_test.go and
  testdata/sema/channel_endpoints_valid.sec.
- New MD-137: channels.md § 20(2) uses a set literal for Capabilities, but
  grammar.md § 3(2) says set literal syntax is not finalized. Capability
  validation is blocked on it.
- Tests: internal/sema passes in full except Codex's untracked
  TestAtomicMemoryModelOperations ("unresolved parameter identities
  collapsed"), which does not touch channels. cmd/compiler and cmd/lsp
  channel/select subsets pass.

### Claude — 2026-11-17 results: concurrency points (released)

- Sender.Share() is now typed Option[Sender[T]] (analyzer_inference.go
  inferSenderCall). The select test uses `try tx.Share() { None => { return } }`.
  Test: TestSenderShareReturnsOption.
- Governance sync after verification, no code change:
  - scheduling: Thread.Yield is never a cancellation point;
  - spawn: current-execution requirements per root.
- New MD-138: declared function types erase the cancellation/task
  requirement on open target sets (Run(Inner) with `callback: fn() void`).
- No change to atomics, memory model or any Codex file.

### Codex — 2026-11-17 results: per-atomic modification order / RMW facts (released)

Closed the first whole requirement in concurrency.memory-model-v2:
"Represent per-atomic modification order and RMW coherence in analysis/IR
facts." The representation is now compiler-owned Sema analysis facts; this
is not a claim that atomic IR lowering or runtime execution is implemented.

- New internal/sema/atomic_memory_model.go: ResolvedAtomicMemoryOperation and
  AtomicMemoryPath retain canonical storage/referent observations, source and
  callable identity, validated orders, per-object total-order/HB-coherence
  constraints and outcome-sensitive read/write/RMW/modification participation.
  Successful CAS is RMW and reads its immediately preceding modification;
  failed CAS is read-only and not a modification. Load and Fence do not enter
  modification order. No global modification order or runtime schedule is
  synthesized from source order.
- atomic_calls.go publishes only validated calls, using the exact order
  already established by the ordinary validator (no second order CTE).
  Failed value validation returns InvalidType; stale CAS facts are removed
  before revalidation. analyzer.go only adds/resets the fact map.
- Canonical synchronization-resource identity is reused, not replaced.
  synchronization_resources.go now treats symbolic reference-parameter or
  reference-receiver origins conservatively across different roots: they may
  alias even when canonical referent origins retain parameter/receiver root
  kind instead of deref root kind. This fixes the interim failure Claude
  noted ("unresolved parameter identities collapsed"). Known owned/local
  roots and disjoint fields/indexes retain their precision.
- AtomicModificationOrderRelationship requires an explicit producer-qualified
  common SynchronizationResourceSnapshot state. Callers must normalize moves,
  replacement/lifetimes and execution instances before qualification; an
  Analyzer namespace is not proof of lifetime continuity. Cross-analysis facts
  cannot become Same merely through reused root numbers. Unknown storage stays
  Unknown. The fact record is an observation plus semantic constraints, not
  an automatic across-time object identity proof.
- Read-only per-call and deterministic source-ordered snapshot queries detach
  paths, structural types and exact projected indexes; reanalysis clears them.
  Invalid/ordinary lookalike methods have no atomic memory facts.

Tests: internal/sema/atomic_memory_model_test.go and new source fixtures
 testdata/sema/atomic_memory_model_valid.sec,
 testdata/sema/atomic_memory_domains_valid.sec,
 testdata/sema/atomic_memory_model_invalid.sec. All nine operation kinds,
independent CAS paths/orders, omitted orders, immutable order aliases, shared
and known-owned aliases, disjoint fields/constant indexes, dynamic-index
uncertainty, independent snapshots, query mutation isolation and reset are
covered on 32/64-bit plans at all three analysis depths where applicable.

Verification, 2026-11-17:

- go test -p 4 ./internal/sema -count=1 -timeout=120s: PASS (39.508s), final
  invalid-CAS guard included; no overlays or skipped own tests.
- go test -p 4 ./cmd/compiler ./cmd/lsp -run 'Atomic|CompareExchange' -count=1 -timeout=120s:
  PASS (compiler 22.782s, LSP 80.142s).
- go test -p 4 ./cmd/compiler ./cmd/lsp -run '^TestCompareExchangeResult(CLI|LSP|SemanticIR)$' -count=1 -timeout=120s:
  final guard rerun PASS (compiler 10.394s, LSP 25.551s).
- go vet ./internal/sema: PASS.
- yamlstatus check: 0 problems in 47 fragments. Owning fragment canonical,
  9 listed evidence paths checked, none missing. git diff --check clean.
  Initial yamlstatus formatting preserved the fragment's existing values;
  normative clauses, required_tests and corrections list remain unchanged.

concurrency_memory_model.yaml / concurrency.memory-model-v2:
implemented 0 -> 1, remaining 7 -> 6 (14.3% implemented). Exactly one whole
representation requirement closed. Release sequences, fence synchronization,
SeqCst lowering, completion publication, race analysis and FFI/platform
synchronization remain open; physical atomic IR/lowering keeps its own owner.
No rulebook, sec/, manual/, C++, runtime or P20 edits; no new MD; no commit.
Read Claude's latest concurrency release (Sender.Share/scheduling/spawn,
MD-138) before release; no overlap. MD-137/MD-138 remain Claude's separate
issues. Working set released.

### Codex — 2026-11-17 working set: IPC CompilationPlan capabilities (released; result below)

Taking concurrency.ipc-v1's whole capability-model requirement: IPCPipes,
IPCTypedMessages, IPCSharedMemory, IPCMutex, IPCSemaphore and exact-width
IPCAtomicRepresentation. Working set: new internal/layout/ipc_capabilities*.go,
scalar_plan.go, plan_fingerprints.go, atomic_capabilities.go only capability
snapshot binding composition, target/target.go projection + new tests, and
concurrency_ipc.yaml. Frozen target/profile facts; missing evidence stays Unknown;
no host/OS/ordinary-atomic inference, runtime implementation or new source syntax.
Read Claude's current releases; no channels/Sema/parser/ownership overlap.

### Codex — 2026-11-17 IPC CompilationPlan capabilities completed / released

Closed exactly one whole remaining requirement in concurrency.ipc-v1: the six
CompilationPlan capability families IPCPipes, IPCTypedMessages, IPCSharedMemory,
IPCMutex, IPCSemaphore and exact-width IPCAtomicRepresentation. The fragment
and ID both move implemented 3 -> 4, remaining 85 -> 84 (4.5% implemented).

Changed files for this task:
- internal/layout/ipc_capabilities.go (new): typed requirements, explicit
  Unknown/Supported/Unsupported, validated provenance/negative reasons, frozen
  target/profile binding, detached sorted facts and deterministic inspection.
- internal/layout/scalar_plan.go: IPC field, validation and exact plan query.
- internal/layout/atomic_capabilities.go: shared snapshot binding base only;
  independent atomic and IPC tables compose in either attachment order without
  recursively capturing one another. Scalar/profile/address-domain facts remain
  bound; changing them rejects reuse. No ordinary atomic support implies IPC.
- internal/layout/plan_fingerprints.go: semantic IPC facts affect Memory,
  Runtime, Execution and composite Plan; evidence strings/registration order
  do not. Other fingerprint domains unchanged.
- internal/platform/target/target.go: definition-to-plan capability projection;
  no registered target acquires positive IPC facts.
- internal/layout/ipc_capabilities_test.go (new): all six families, exact-width
  non-inference, malformed facts, detached mutation/concurrent query isolation,
  target/profile binding, both attachment orders and fingerprint domains.
- internal/platform/target/ipc_capabilities_test.go (new): real registry and
  definition-to-plan path, explicit negative evidence, foreign-plan rejection.
- governance/concurrency_ipc.yaml: one remaining removed, complete evidence and
  verification added, revision updated. No other normative remaining removed.
- mlir/compiler-packages-handoff.md: reservation and this release.

Verification on 2026-11-17:
- go test -p 4 ./internal/layout ./internal/platform/target ./internal/project ./internal/sema -count=1 -timeout=120s:
  PASS, complete suites (Sema 26.662s).
- go test -race ./internal/layout ./internal/platform/target ./internal/project -count=1 -timeout=120s:
  PASS, complete suites including concurrent IPC queries.
- go test -p 4 ./cmd/compiler ./cmd/lsp -run '^TestIPCError(CLI|LSP)$|^TestLSP(AnalyzerUsesProjectScalarPlan|ScalarPlanRejectsAmbiguousProjectVariants)$' -count=1 -timeout=120s:
  PASS, focused CLI/LSP/project-plan regressions (6.010s / 27.836s).
- go vet ./internal/layout ./internal/platform/target ./internal/project: PASS.
- yamlstatus check: 0 problems in 47 fragments. Owning fragment fmt -check:
  canonical. Listed evidence paths: 56 checked, none missing. Count snapshot
  confirms exactly one remaining removed / one implemented added. Whole working
  tree git diff --check: clean.

Scope: capability representation is complete; actual IPC operations, providers,
static operation diagnostics, route adapters, type eligibility, lowering and
full-family conformance remain separately pending. All registered targets stay
Unknown; explicit Supported facts are trusted producer assertions of complete
canonical semantics, never host/native/OS guesses or ordinary atomic inference.
No sec/, manual/, rulebook, runtime or P17-P20 edits; no new MD; no commit.
Unrelated allocation_capabilities.go working-tree changes were already present
and are not part of this task. Read latest handoff before this release; no newer
Claude reservation overlaps this working set. Working set released.
