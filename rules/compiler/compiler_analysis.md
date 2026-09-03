# Compiler Analysis

- Status: Normative
- Created: 2026-09-03
- Last updated: 2026-09-03
- Document revision: 2.0
- Sec language version: 0.1
- Canonical path: `rules/compiler/compiler_analysis.md`
- Replaces: `rules/compiler/compiler_analysis.txt`
- Repository baseline reviewed: `998d8d1`

---

## § 1 Purpose and authority

§ 1(1) This rulebook defines the common architecture, coordination rules, proof model, dependency model, execution scope, and tooling contract for Sec compiler analyses.

§ 1(2) It does not redefine the detailed semantics owned by specialized analysis or language rulebooks.

§ 1(3) Specialized analysis rulebooks own their respective domains, including:

```text
analysis/call_graph.md
analysis/effect_analysis.md
analysis/escape_analysis.md
analysis/closure_analysis.md
analysis/parameter_usage_analysis.md
analysis/pitfall_analysis.md
analysis/stack_analysis.md
analysis/data_races.md
analysis/deadlock_analysis.md
analysis/isr_analysis.md
```

§ 1(4) Memory rulebooks own ownership, borrowing, lifetime, references, storage, allocation, Arena, destruction, transferability, and unsafe semantics.

§ 1(5) Control-flow rulebooks own source control-flow semantics.

§ 1(6) Interface/generic/iterator rulebooks own conformance and iteration semantics.

§ 1(7) Platform/ABI/FFI/register/interrupt rulebooks own target-specific semantic requirements.

§ 1(8) `compiler/semantic_ir.md` owns the canonical post-analysis semantic representation consumed by lowering.

§ 1(9) This rulebook defines how all of those analyses compose into one compiler-owned analysis system.

---

## § 2 Core principles

§ 2(1) Compiler analysis exists to establish facts required for:

```text
language validity;
memory safety;
resource safety;
target validity;
effect guarantees;
execution safety;
diagnostics;
tooling;
safe optimization;
lowering.
```

§ 2(2) Analysis must not change Sec language semantics.

§ 2(3) Each semantic fact has one canonical owner.

§ 2(4) Other analyses consume that fact rather than independently recomputing a competing interpretation.

§ 2(5) Analyses may mutually depend through explicit iterative/fixed-point coordination.

§ 2(6) A valid compiler architecture does not require one rigid globally linear pass sequence.

§ 2(7) Analysis may be implemented as passes, demand-driven queries, worklists, data-flow solvers, graph algorithms, fixed points, cached summaries, or equivalent mechanisms.

§ 2(8) Implementation strategy may vary provided the same canonical semantic result is produced.

---

## § 3 Analysis input

§ 3(1) Analysis operates on a canonical compilation snapshot.

§ 3(2) The snapshot includes where relevant:

```text
source files/modules;
resolved project configuration;
selected logical build target;
selected target variant;
selected profile;
CompilationPlan;
imports/dependencies;
compiler-known declarations;
target/platform knowledge;
validated AST/Sema state;
canonical type/symbol identities;
cached validated summaries.
```

§ 3(3) Analysis facts are valid only for the snapshot/plan under which they were derived.

§ 3(4) Compiler-host architecture, operating system, pointer width, locale, environment ordering, or filesystem accidents must not silently replace selected target facts.

---

## § 4 Project and build context

§ 4(1) Project-aware analysis consumes the canonical project/manifest/target model from the project rulebooks.

§ 4(2) Single-file inspection commands may use explicitly defined host/default context where the compiler command contract permits it.

§ 4(3) Build-producing commands require a complete target/CompilationPlan before target-dependent lowering.

§ 4(4) An analysis must never silently switch to another target because the source is invalid for the selected one.

§ 4(5) Target-specific invalidity is reported against the selected target.

---

## § 5 Canonical semantic identities

§ 5(1) Analysis facts must use compiler-owned semantic identities.

§ 5(2) Relevant identities include:

```text
module
symbol
type
callable
generic specialization
interface conformance
Place/root/sub-Place
storage domain
invalidation domain
allocation
ArenaDomain
reference origin
execution root
task/thread/process/ISR context
hardware resource
foreign symbol/contract
target/profile/CompilationPlan.
```

§ 5(3) Source names are diagnostic labels, not sufficient analysis identities.

§ 5(4) Numeric addresses are not sufficient storage/reference identities.

§ 5(5) AST node pointers are not stable cross-analysis identities.

---

## § 6 Analysis fact ownership

§ 6(1) Each fact family has one canonical analysis owner.

§ 6(2) Examples:

```text
type/conformance resolution:
    semantic/type analysis

ownership availability and transfer:
    ownership analysis

borrow compatibility/live ranges:
    borrowing analysis

reference/lifetime dependency:
    lifetime/reference analysis

escape destination/retention:
    escape analysis

callable reachability:
    call graph

effects and guarantees:
    effect analysis

closure capture/target-set summaries:
    closure analysis

parameter demand:
    parameter-usage analysis

stack resource demand:
    stack analysis

data-race proof:
    data-race analysis

deadlock proof:
    deadlock analysis

ISR cross-analysis validity:
    ISR analysis

pitfall/advisory evidence:
    pitfall analysis
```

§ 6(3) A consumer may cache a canonical fact but must retain its owner/provenance.

§ 6(4) A consumer must not reinterpret an owner's `Unknown`/`Unproven` fact as positive proof.

---

## § 7 Proof states

§ 7(1) Analysis facts that represent safety or validity should use an explicit proof state where uncertainty is possible.

§ 7(2) Canonical conceptual states are:

```text
Valid
Invalid
Unproven
```

§ 7(3) An analysis may use richer internal lattices if they map unambiguously to the canonical meaning.

§ 7(4) `Invalid` means the compiler has proof of violation.

§ 7(5) `Valid` means sufficient proof exists for the requested property in the current snapshot/plan.

§ 7(6) `Unproven` means the compiler lacks sufficient proof either way.

§ 7(7) `Unproven` must not be displayed as `Invalid` unless a language/profile rule explicitly requires positive proof and therefore rejects uncertainty.

§ 7(8) In such positive-proof contexts the diagnostic should explain that required proof is missing rather than pretending a concrete violation was proven.

---

## § 8 Fact provenance

§ 8(1) Every nontrivial analysis fact should preserve enough provenance for diagnostics, invalidation, and audit.

§ 8(2) Provenance may include:

```text
source location;
originating declaration;
call edge;
capture edge;
transfer edge;
storage/reference origin;
target/profile fact;
FFI contract;
hardware resource;
previous state transition;
dependent analysis fact;
trusted assertion;
compiler-known rule;
summary/cache origin.
```

§ 8(3) Positive trusted facts from FFI/platform/unsafe contracts retain trust provenance.

§ 8(4) Provenance may be compacted after validation only if diagnostics/invalidation remain correct.

---

## § 9 Analysis dependencies

§ 9(1) Analysis dependencies form a directed graph, not necessarily one fixed list.

§ 9(2) A dependency means one analysis consumes facts produced by another.

§ 9(3) Example relationships include:

```text
ownership -> borrowing;
ownership + borrowing + storage -> lifetime/reference;
call graph -> effects;
call graph + closure targets -> effects;
call graph + effects -> ISR;
call graph + stack summaries -> stack roots;
escape + closure + transferability -> task/thread validation;
Place/storage identity -> races;
effects + call graph + synchronization -> deadlocks/ISR;
all canonical semantic facts -> pitfall analysis.
```

§ 9(4) Cyclic dependencies are permitted only through a deterministic fixed-point or equivalent convergent mechanism.

---

## § 10 Analysis scheduling

§ 10(1) The compiler may schedule analyses in phases.

§ 10(2) A typical conceptual progression is:

```text
foundational semantic resolution;
local control/data-flow facts;
ownership/borrow/lifetime/reference facts;
call/capture/execution graph facts;
interprocedural effects/escape/transfer summaries;
resource/concurrency/platform analyses;
cross-analysis validators such as ISR;
advisory analyses such as parameter usage/pitfalls;
Semantic IR finalization/verification.
```

§ 10(3) This ordering is descriptive rather than a required implementation pass list.

§ 10(4) Demand-driven analysis may compute a later fact early if all dependencies are available.

§ 10(5) No analysis may consume a positive fact whose prerequisites are stale or incomplete.

---

## § 11 Local versus interprocedural analysis

§ 11(1) Analyses declare whether their facts are:

```text
expression-local;
statement-local;
basic-block-local;
function-local;
module-local;
interprocedural;
execution-root/global;
CompilationPlan-wide.
```

§ 11(2) A local analysis must not make an interprocedural positive claim without a validated callable summary or reachable-body proof.

§ 11(3) Interprocedural analyses use canonical call/callback/closure/interface target facts.

§ 11(4) Unknown/open dynamic target sets are conservative.

---

## § 12 Control-flow graph

§ 12(1) Semantic control-flow analysis establishes canonical function CFG facts before analyses that require path sensitivity.

§ 12(2) CFG facts include where relevant:

```text
basic blocks;
successors/predecessors;
branch conditions;
loop edges;
break/continue targets;
return/termination edges;
error propagation edges;
panic/termination edges;
cleanup/defer edges;
match/try arm edges;
task/thread execution-root creation edges where represented separately.
```

§ 12(3) The CFG must reflect Sec semantics rather than backend exception/control-flow conventions.

---

## § 13 Data-flow framework

§ 13(1) Path-sensitive analyses may use canonical data-flow lattices.

§ 13(2) A data-flow analysis defines:

```text
domain/lattice;
entry state;
transfer function;
join/merge;
edge refinements;
termination/convergence criterion;
diagnostic/proof extraction.
```

§ 13(3) Join must be conservative.

§ 13(4) A join must not create a stronger positive fact than every incoming path justifies.

§ 13(5) Path predicates may refine facts where the language exposes canonical tests, including ownership availability, Option/Result/member/variant tests, bounds/range facts, and target/build conditions.

---

## § 14 Fixed-point analyses

§ 14(1) Recursive call graphs, loops, recursive generic instantiations, closure target propagation, effect propagation, and other cyclic analyses may require fixed points.

§ 14(2) Fixed-point computation must:

```text
terminate;
be deterministic;
use a monotonic/convergent domain or equivalent strategy;
not infer positive proof from lack of further progress;
retain conservative unknown/unproven state where necessary.
```

§ 14(3) Analysis budgets may stop refinement but must not silently convert incomplete analysis to positive proof.

---

## § 15 Analysis budgets

§ 15(1) Expensive analyses may use compiler/project-configurable budgets.

§ 15(2) Budgets may bound:

```text
call-depth expansion;
target-set expansion;
path sensitivity;
loop widening iterations;
recursive specialization analysis;
closure-flow propagation;
stack path exploration;
deadlock graph exploration;
pitfall advisory search.
```

§ 15(3) Budget exhaustion produces conservative results.

§ 15(4) If a language/profile guarantee requires proof, budget exhaustion may cause rejection as `Unproven`.

§ 15(5) If the analysis is advisory, budget exhaustion may reduce confidence/coverage without making the program invalid.

---

## § 16 Type analysis

§ 16(1) Type analysis establishes canonical expression/declaration types.

§ 16(2) It resolves:

```text
operators;
calls/methods/properties;
interface conformance;
generic substitution;
conversion legality;
Result/Option typing;
named/distinct carriers;
units;
registers;
collections/shaped types;
compiler-known members/interfaces.
```

§ 16(3) Every runtime expression reaching ownership/control-flow analysis has one resolved semantic type, except where a canonical staged inference rule explicitly permits deferred resolution.

§ 16(4) Type analysis must not erase ownership, effect, unsafe, ABI, unit, reference, or target-sensitive callable/type facts.

---

## § 17 Interface and conformance analysis

§ 17(1) Interface analysis resolves explicit/implicit compiler-known conformance according to `interfaces.md`.

§ 17(2) Conformance is represented by a stable semantic identity.

§ 17(3) Interface/member lookup is completed before Semantic IR generation.

§ 17(4) Static conformance does not require runtime dynamic dispatch.

§ 17(5) Open dynamic interface values retain conservative target-set/contract facts for interprocedural analyses.

---

## § 18 `Iterator[T]` analysis

§ 18(1) `Iterator[T]` is resolved through ordinary compiler-known generic interface conformance.

§ 18(2) The compiler establishes:

```text
iterator type/value;
concrete Iterator[T] conformance;
yielded T;
resolved Next() target;
Next() ownership/mutability/effects;
loop binding semantics;
iterator lifetime/destruction;
structural mutation dependencies.
```

§ 18(3) Iterator analysis must not be a naming-convention probe for a method called `Next`.

§ 18(4) Iterator analysis must not be a closed whitelist of nominal iterable types.

§ 18(5) Compiler-known collection/range/string iteration may provide specialized analysis facts while remaining semantically equivalent to the canonical iteration contract.

§ 18(6) Compiler and LSP consume the same conformance/Next resolution.

---

## § 19 Ownership analysis

§ 19(1) Ownership analysis owns the canonical availability lattice and ownership transitions from `ownership.md`.

§ 19(2) It determines:

```text
owner creation;
copy/move;
consuming boundaries;
Place availability;
partial ownership;
conditional ownership;
discard convergence;
replacement/reinitialization;
destruction responsibility;
ownership transfer commit.
```

§ 19(3) Availability and unavailability reason are separate facts.

§ 19(4) Ownership analysis consumes canonical Place identity.

§ 19(5) Other analyses must not infer ownership transfer merely from low-level stores/loads or move-only type classification.

---

## § 20 Borrow analysis

§ 20(1) Borrow analysis owns shared/mutable borrow compatibility and live ranges.

§ 20(2) It uses canonical Place relationships and NLL/final-use reasoning.

§ 20(3) It determines where:

```text
shared borrow may begin/end;
mutable borrow may begin/end;
reborrow is valid;
Places are disjoint/overlapping;
move/destruction/replacement conflicts with active borrow.
```

§ 20(4) Runtime generation/reference checks never replace borrow exclusivity proof.

---

## § 21 Lifetime/reference analysis

§ 21(1) Lifetime/reference analysis proves reference/dependency validity using canonical origin, storage, epoch, borrow, escape, and execution facts.

§ 21(2) It covers:

```text
returned references;
captured references;
defer;
closures;
Arena/storage epochs;
mapping lifetime;
task/thread retention;
FFI retention;
thread-local affinity;
reference derivation;
relocation/pinning dependencies.
```

§ 21(3) Multiple finite returned-reference origins are valid when all possible origins satisfy the required lifetime relation.

§ 21(4) No hidden heap/Arena promotion is introduced to repair an invalid lifetime.

---

## § 22 Escape analysis

§ 22(1) Escape analysis follows `analysis/escape_analysis.md`.

§ 22(2) Escape is a semantic relationship between a subject and a destination/retention domain.

§ 22(3) Escape analysis must not assume that escape implies allocation.

§ 22(4) Invalid escapes are rejected.

§ 22(5) Valid escaping storage/capture must already have a lifetime/storage mechanism sufficient for the destination.

§ 22(6) Escape facts feed closure, allocation, lifetime, transferability, FFI, task/thread, and optimization analysis.

---

## § 23 Closure analysis

§ 23(1) Closure analysis follows `analysis/closure_analysis.md`.

§ 23(2) It owns canonical facts concerning:

```text
capture identity/mode;
environment ownership;
callable-flow;
escape class;
possible invocation targets;
capture lifetime;
allocation/materialization requirement;
task/thread transfer implications.
```

§ 23(3) Closure allocation is not inferred solely from “closure escapes”; the canonical closure/allocation rules decide required storage.

---

## § 24 Call graph

§ 24(1) Call graph analysis follows `analysis/call_graph.md`.

§ 24(2) The canonical graph includes semantic callable relationships such as:

```text
direct calls;
method/property calls;
interface/indirect target sets;
closure calls;
callbacks;
generated cleanup/destruction helpers;
task spawn roots;
thread entry roots;
interrupt roots;
foreign callback relationships;
compiler-generated helpers.
```

§ 24(3) Call graph edges retain provenance and execution-context semantics.

§ 24(4) A task/thread/ISR execution edge is not necessarily an ordinary synchronous call edge.

---

## § 25 Effect analysis

§ 25(1) Effect analysis follows `analysis/effect_analysis.md`.

§ 25(2) It derives compiler-owned effects such as:

```text
MayPanic;
MayAllocate;
MayBlock;
MaySuspend;
MaySpawn;
MayIO;
MayAccessVolatile;
MayMutateExternalState;
MayUseNondeterministicInput;
```

and other canonical effects.

§ 25(3) It separately verifies guarantees such as `noPanic`, `noAlloc`, and `noBlock`.

§ 25(4) Effects propagate through reachable direct/indirect/generated operations according to canonical call/execution graphs.

§ 25(5) `MayAllocate` and `RequiresAllocationContext` remain separate facts.

§ 25(6) Logical shaped operations and storage-producing shaped operations may have different allocation effects.

---

## § 26 Parameter-usage analysis

§ 26(1) Parameter-usage analysis follows `analysis/parameter_usage_analysis.md`.

§ 26(2) It is primarily advisory unless another rule consumes one of its canonical facts as a safety requirement.

§ 26(3) It may derive demand dimensions such as:

```text
read;
write;
consume;
retain;
escape;
call;
address;
thread/task transfer;
effect contribution.
```

§ 26(4) Advisory suggestions must not silently rewrite callable signatures.

---

## § 27 Stack analysis

§ 27(1) Stack analysis follows `analysis/stack_analysis.md`.

§ 27(2) It distinguishes semantic execution-stack/resource demand from target machine frame implementation.

§ 27(3) It consumes:

```text
call graph;
recursion;
target plan;
function frame estimates;
task/thread/ISR roots;
interrupt nesting;
dynamic allocation/stack constructs where permitted;
generated cleanup/wrappers.
```

§ 27(4) Strict targets/profiles may require positive bounded proof.

---

## § 28 Data-race analysis

§ 28(1) Data-race analysis follows `analysis/data_races.md`.

§ 28(2) It consumes canonical memory-location identity, execution contexts, ownership/borrowing, transferability, synchronization, atomics, FFI/external mutation, volatile/MMIO, and ISR relationships.

§ 28(3) Volatile is not synchronization.

§ 28(4) Generation/reference validity is not synchronization.

§ 28(5) Unsafe does not legalize a data race.

---

## § 29 Deadlock analysis

§ 29(1) Deadlock analysis follows `analysis/deadlock_analysis.md`.

§ 29(2) It consumes canonical synchronization/resource acquisition facts, execution contexts, blocking effects, lock ordering, calls, tasks/threads, callbacks, and FFI behavior.

§ 29(3) Unknown external lock/block behavior remains conservative.

§ 29(4) Deadlock analysis must distinguish proven cycle from possible/unproven risk according to its canonical proof model.

---

## § 30 ISR analysis

§ 30(1) ISR analysis follows `analysis/isr_analysis.md`.

§ 30(2) ISR analysis is a cross-analysis constraint verifier.

§ 30(3) It consumes:

```text
call graph;
effects;
stack;
data-race;
deadlock;
FFI;
transferability;
hardware access;
allocation context;
panic;
blocking;
target interrupt profile.
```

§ 30(4) It verifies canonical ISR/interrupt-safe requirements including `noPanic`, `noAlloc`, `noBlock`, bounded work, and target-specific constraints.

§ 30(5) Unsafe does not waive ISR constraints.

---

## § 31 Pitfall analysis

§ 31(1) Pitfall analysis follows `analysis/pitfall_analysis.md`.

§ 31(2) It consumes canonical facts from analyses that own Sec semantics.

§ 31(3) It must not independently recompute type, bounds, ownership, lifetime, escape, FFI, or concurrency truth.

§ 31(4) Pitfall findings are advisory unless another canonical rule explicitly upgrades a represented pattern to a language error.

§ 31(5) Confidence/evidence/suppression/budget/tooling behavior is owned by the pitfall rulebook.

---

## § 32 Definite initialization

§ 32(1) Definite-initialization analysis proves readable values are initialized on every reachable path.

§ 32(2) It consumes canonical default/construction/availability facts.

§ 32(3) It supports:

```text
locals;
parameters where relevant;
partial struct/aggregate construction;
union active state;
branch/loop joins;
replacement/reinitialization;
construction failure.
```

§ 32(4) Reading known-uninitialized storage is a compile-time error.

§ 32(5) Uninitialized raw storage is governed by raw/unsafe/storage rules and is not ordinary readable `T`.

---

## § 33 Reachability

§ 33(1) Reachability analysis identifies semantically executable/unreachable paths.

§ 33(2) It consumes:

```text
CFG;
constant conditions;
exhaustiveness;
panic/termination;
compile-time conditions;
target/build conditions;
proven impossible conversions/checks where applicable.
```

§ 33(3) Statically unreachable source diagnostics follow warning/error policy.

§ 33(4) Backend `unreachable` is emitted only when Semantic IR/lowering has a Sec proof or a prior non-returning operation.

---

## § 34 Compile-time evaluation interface

§ 34(1) Compile-time evaluation provides resolved constants/facts required by analyses.

§ 34(2) Its detailed semantics belong to the canonical compile-time evaluation rulebook when finalized.

§ 34(3) Analysis consumes compile-time results for:

```text
array lengths;
enum values;
attributes;
register widths;
range bounds;
target/build conditions;
generic values;
layout requirements;
capacity proofs;
contracts.
```

§ 34(4) Target-dependent compile-time evaluation uses selected target facts rather than compiler-host behavior.

---

## § 35 Contract analysis

§ 35(1) Contract analysis validates canonical type/operation contracts.

§ 35(2) Contracts may include:

```text
named-type ranges/finite sets;
units;
register widths/fields;
regex/format constraints where canonical;
generic/interface constraints;
layout/address constraints;
target/profile requirements;
compiler-known API preconditions.
```

§ 35(3) Compile-time proof is preferred where sufficient.

§ 35(4) Runtime checks remain explicit when language semantics require dynamic validation.

---

## § 36 Generic analysis

§ 36(1) Generic analysis resolves canonical generic arguments/constraints/conformances.

§ 36(2) It establishes deterministic specialization requests/identities where the current compiler pipeline requires specialization.

§ 36(3) It must preserve:

```text
ownership;
effects;
interfaces/conformances;
layout requirements;
target/profile dependence;
copy/destruction classification;
reference/storage semantics.
```

§ 36(4) Recursive/cyclic generic analysis uses deterministic cycle handling/fixed points.

§ 36(5) Detailed monomorphization/lowering behavior belongs to their planned canonical rulebooks.

---

## § 37 Allocation and Arena analysis

§ 37(1) Allocation analysis consumes `allocation.md`, `arena.md`, effects, storage, lifetime, escape, and target/provider facts.

§ 37(2) It determines where relevant:

```text
allocation-capable operation;
allocation domain;
RequiresAllocationContext;
provider availability;
failure channel;
ArenaDomain;
Arena state version;
Arena epoch;
capacity demand;
reset/release blockers;
growth policy;
no-hidden-allocation guarantee.
```

§ 37(3) Arena capacity analysis may classify demand as:

```text
Exact
Bounded
Unknown
Unbounded
```

§ 37(4) Strict profiles may require positive bounded proof.

---

## § 38 Storage/reference analysis

§ 38(1) Storage/reference analysis consumes canonical `storage.md` and `reference_model.md` facts.

§ 38(2) It establishes:

```text
storage origin/domain;
backing relation;
address stability;
memory space;
invalidation domain;
validity epoch;
reference origin/provenance;
bounds;
relocation/pinning dependency;
mapping/foreign dependency.
```

§ 38(3) Storage/ref facts are shared across ownership, lifetime, escape, transferability, races, FFI, hardware, and lowering.

---

## § 39 Transferability analysis

§ 39(1) Transferability analysis follows `transferability.md`.

§ 39(2) It proves whether a specific value/capability may cross a specific execution boundary under the active policy.

§ 39(3) Boundaries include:

```text
task;
physical thread;
process adapter;
ISR;
foreign callback context.
```

§ 39(4) Exclusive transfer and concurrent sharing remain separate.

§ 39(5) A nominal type-wide summary is used only when valid for all represented values/conditions.

---

## § 40 Platform and target analysis

§ 40(1) Target/platform analysis validates selected-target semantic availability.

§ 40(2) It consumes the canonical `CompilationPlan`.

§ 40(3) It validates:

```text
target architecture/profile;
pointer/int widths;
address spaces;
ABI availability;
platform modules;
syscalls;
register/MMIO resources;
fixed addresses;
interrupt identities;
atomic support;
inline assembly availability/constraints;
target intrinsics;
memory spaces;
backing/allocation providers;
runtime requirements.
```

§ 40(4) Target analysis does not redefine general language semantics.

---

## § 41 ABI analysis

§ 41(1) ABI analysis validates calls/data crossing ABI boundaries.

§ 41(2) It consumes canonical layout, target, FFI, callable, ownership, and representation facts.

§ 41(3) It validates:

```text
calling convention;
parameter classification;
return classification;
aggregate representation;
enum/union representation;
alignment/padding;
varargs;
foreign callbacks;
symbol/linkage rules.
```

§ 41(4) ABI compatibility is not inferred solely from equal size/alignment.

---

## § 42 Hardware/register analysis

§ 42(1) Hardware/register analysis validates register declarations and concrete hardware access plans.

§ 42(2) It consumes:

```text
register semantic layout;
target resources;
fixed-address/mapping facts;
volatile rules;
access width/order;
permissions;
shadow/specialized field semantics;
interrupt/access context.
```

§ 42(3) Signal active-high/active-low interpretation is outside compiler semantics.

---

## § 43 FFI analysis

§ 43(1) FFI analysis validates trusted foreign contracts and call adaptation.

§ 43(2) It consumes:

```text
ABI;
layout;
ownership/retention;
nullability;
RawPtr/reference rules;
effects;
callbacks;
thread affinity;
panic/unwind/abort;
allocation/deallocation;
target platform.
```

§ 43(3) Unknown foreign behavior is conservative where positive proof is required.

§ 43(4) FFI contract facts retain trust provenance.

---

## § 44 Unsafe analysis

§ 44(1) Unsafe analysis validates lexical unsafe context and unsafe-callable/operation contracts.

§ 44(2) Unsafe accepts only explicitly designated proof obligations.

§ 44(3) Every unrelated analysis remains active.

§ 44(4) Compiler-proven false obligations remain errors inside unsafe.

§ 44(5) Trust provenance remains available for downstream effect, ISR, FFI, tooling, and audit analysis.

---

## § 45 Execution contexts

§ 45(1) Analyses must distinguish execution contexts where semantics differ.

§ 45(2) Relevant contexts include:

```text
ordinary synchronous function;
task;
physical thread;
process entry;
ISR;
foreign callback;
compiler-generated cleanup/helper.
```

§ 45(3) Context identity affects effects, stack, transferability, thread-local validity, synchronization, allocation context, panic containment, and platform restrictions.

§ 45(4) Analyses must not flatten every execution edge into one ordinary call graph.

---

## § 46 Execution roots

§ 46(1) Whole-program/interprocedural analyses operate from canonical execution roots.

§ 46(2) Roots may include:

```text
program entry;
exported/externally callable functions;
task entry;
thread entry;
interrupt entry;
foreign callback entry;
test entry;
compiler/runtime-generated entrypoints.
```

§ 46(3) An analysis declares which root classes it consumes.

§ 46(4) Root-specific profiles/requirements remain attached.

---

## § 47 Per-CompilationPlan analysis

§ 47(1) Facts that depend on target/profile/layout/platform/resources are per-`CompilationPlan`.

§ 47(2) Examples include:

```text
layout;
ABI;
stack size;
atomic support;
interrupt profile;
allocation provider;
memory space;
reference representation;
epoch width;
inline assembly;
hardware resources.
```

§ 47(3) Target-independent facts may be reused only when their dependencies exclude target-sensitive state.

§ 47(4) A cached fact from one plan must not be reused as proof in an incompatible plan.

---

## § 48 Cross-analysis validators

§ 48(1) Some properties are valid only after composing multiple canonical analyses.

§ 48(2) A cross-analysis validator consumes existing facts rather than redefining each component.

§ 48(3) ISR analysis is a canonical example.

§ 48(4) Additional examples may include:

```text
safe FFI callback retention;
Arena-backed task transfer;
strict bounded-resource profile validation;
hardware access in concurrent contexts.
```

§ 48(5) Cross-analysis validation retains provenance to the underlying cause.

---

## § 49 Runtime checks versus static proofs

§ 49(1) Analysis classifies required safety conditions as:

```text
statically proven;
statically disproven;
requires defined runtime validation;
requires trusted/unsafe contract;
unsupported/unproven under active profile.
```

§ 49(2) Runtime validation is introduced only where the language/profile defines such behavior.

§ 49(3) A runtime check is not an acceptable replacement for a rule that requires compile-time proof.

§ 49(4) Proven checks may be removed during Semantic IR/lowering.

---

## § 50 Optimization analysis

§ 50(1) Optimization analysis may consume canonical analysis facts.

§ 50(2) It must never change Sec observable behavior.

§ 50(3) Valid proof-driven opportunities include:

```text
constant folding;
dead unreachable block removal;
copy elimination;
move representation elimination;
bounds/check elimination;
reference epoch-check elimination;
Arena capacity-check elimination;
iterator specialization/inlining;
devirtualization from proven target set;
allocation elimination where semantically permitted;
layout/address optimization;
effect-driven code removal.
```

§ 50(4) Optimization must not introduce hidden allocation, ownership transfer, cleanup reorder, panic/error change, race, lifetime violation, or stronger backend UB assumption.

---

## § 51 Semantic IR finalization

§ 51(1) Semantic IR generation/finalization consumes completed required analysis facts.

§ 51(2) A function/module must not be considered lowering-ready while required safety/semantic facts remain unresolved.

§ 51(3) Semantic IR may retain `Unproven` facts only when:

```text
the language/profile permits runtime validation;
or the fact is advisory/tooling-only;
or a later target-plan analysis is explicitly responsible before lowering.
```

§ 51(4) Semantic IR verification ensures represented facts/operations are mutually consistent.

---

## § 52 Analysis summaries

§ 52(1) Interprocedural/separate-compilation analysis uses validated summaries.

§ 52(2) A summary declares:

```text
fact owner/domain;
callable/type identity;
preconditions;
postconditions;
effects;
ownership/borrow/escape behavior;
transfer/retention;
resource demand;
target/profile dependency;
trust provenance;
version/dependency fingerprint.
```

§ 52(3) Summaries must be conservative.

§ 52(4) A stale/incompatible summary must be invalidated rather than silently trusted.

---

## § 53 Open and closed target sets

§ 53(1) Indirect calls/callbacks/interface values may have a closed finite target set or an open contract.

§ 53(2) Closed target-set analyses may combine concrete target facts.

§ 53(3) Open targets use the declared callable/interface contract.

§ 53(4) Unknown behavior outside the contract is conservative.

§ 53(5) Analyses must not assume one observed implementation is the only possible target when the semantic set is open.

---

## § 54 Incremental invalidation

§ 54(1) Analysis facts are invalidated by semantic dependencies, not merely by changed source text.

§ 54(2) Dependencies may include:

```text
symbol/type declarations;
generic specialization/conformance;
call target sets;
function bodies;
ownership/borrow/lifetime facts;
effects;
target/profile;
layout;
FFI/platform contracts;
hardware resources;
allocation/provider policy;
iterator conformance/Next target;
task/thread/ISR relationships.
```

§ 54(3) Incremental invalidation should be as narrow as correctness permits.

§ 54(4) Reused facts retain proof that all dependencies remain compatible.

---

## § 55 Analysis snapshot consistency

§ 55(1) All facts used to validate one compilation result must belong to one logically consistent snapshot.

§ 55(2) LSP may compute progressively refined results but must not combine stale facts from incompatible snapshots into positive proof.

§ 55(3) A later analysis result supersedes an older result only when its dependency snapshot is compatible/newer according to the tooling model.

---

## § 56 Parallel analysis

§ 56(1) Independent analyses may execute in parallel.

§ 56(2) Parallelism must not affect semantic results, diagnostics ordering requirements, stable IDs, summary serialization, or cache keys.

§ 56(3) Shared analysis services must be deterministic and thread-safe.

§ 56(4) Racy internal caches must not leak nondeterminism into compiler semantics.

---

## § 57 Determinism

§ 57(1) Equivalent source and `CompilationPlan` must produce equivalent analysis facts.

§ 57(2) Determinism requirements include:

```text
target sets;
fixed-point results;
effect summaries;
diagnostic primary/cause selection;
specialization identities;
analysis summaries;
serialized cache records;
iterator resolution;
resource bounds.
```

§ 57(3) Host map iteration, pointer addresses, goroutine/thread timing, or filesystem enumeration order must not change results.

---

## § 58 Diagnostics

§ 58(1) Every rejecting analysis must produce mentor-style diagnostics.

§ 58(2) A diagnostic should identify:

```text
what is invalid or unproven;
where it occurs;
the canonical reason/rule;
the causal source operation/declaration;
related locations;
why the compiler cannot establish safety;
a practical safe correction where known.
```

§ 58(3) Diagnostics distinguish `Invalid` from `Unproven`.

§ 58(4) Diagnostics should prefer the earliest meaningful semantic cause over cascaded symptoms.

§ 58(5) Cross-analysis diagnostics trace through canonical fact provenance.

---

## § 59 Diagnostic ownership

§ 59(1) The analysis owning the violated fact normally owns the primary diagnostic category.

§ 59(2) A cross-analysis validator may issue a composite diagnostic while linking the underlying owned facts.

§ 59(3) Advisory analyses must not duplicate a primary compiler error as a competing warning.

§ 59(4) Pitfall analysis may suppress findings already covered by stronger canonical diagnostics.

---

## § 60 LSP

§ 60(1) LSP consumes compiler analysis results rather than reimplementing Sec semantics.

§ 60(2) LSP may request reduced-budget/progressive analysis.

§ 60(3) Progressive analysis may report states such as:

```text
pending/refining;
Valid;
Invalid;
Unproven.
```

§ 60(4) User-facing UI should avoid exposing internal solver terminology where a clearer explanation exists.

§ 60(5) LSP and compiler builds must agree when run with equivalent complete analysis depth and CompilationPlan.

---

## § 61 `sec analyse`

§ 61(1) `sec analyse` or equivalent analysis command may expose deeper canonical analysis reports.

§ 61(2) Reports may include:

```text
call graph;
effects;
escape;
closure target sets;
parameter usage;
stack demand;
Arena demand;
race/deadlock findings;
ISR proof;
pitfalls;
ownership/lifetime provenance;
iterator resolution;
target/profile applicability.
```

§ 61(3) Reports distinguish errors, unproven safety requirements, warnings, and informational/advisory findings.

---

## § 62 Analysis modes

§ 62(1) Compiler tooling may offer different analysis-depth modes.

§ 62(2) A reduced mode may skip expensive advisory/interprocedural refinement.

§ 62(3) A reduced mode must not falsely report positive proof for a skipped required analysis.

§ 62(4) Build-producing modes must run every analysis required by the selected language/profile guarantees.

---

## § 63 Strict profiles

§ 63(1) Profiles may require stronger positive proof.

Examples:

```text
bounded stack;
bounded Arena/storage demand;
no panic;
no allocation;
no blocking;
ISR safety;
no unknown FFI effects;
no dynamic availability bookkeeping;
no runtime reference metadata/checks.
```

§ 63(2) Under such a profile `Unproven` may be a compile-time rejection.

§ 63(3) The diagnostic must identify the profile requirement and missing proof.

---

## § 64 Hosted permissive profiles

§ 64(1) Hosted profiles may permit runtime checks or conservative runtime mechanisms where canonical rules define them.

§ 64(2) They may accept `Unknown` advisory resource estimates where no safety contract requires a static bound.

§ 64(3) Hosted permissiveness must not weaken ownership, borrow, type, data-race, reference, or other compile-time guarantees that remain mandatory.

---

## § 65 Trusted facts

§ 65(1) Some analysis facts originate from trusted declarations outside ordinary Sec verification.

§ 65(2) Examples:

```text
FFI contracts;
target knowledge packs;
ABI descriptions;
inline assembly contracts;
platform resources;
compiler intrinsics.
```

§ 65(3) Trusted facts retain provenance.

§ 65(4) A trusted claim is not the same as a compiler-proven source fact.

§ 65(5) Changes to trusted dependencies invalidate dependent analyses.

---

## § 66 Error recovery

§ 66(1) Interactive/frontend recovery may continue analysis after earlier errors to improve diagnostics.

§ 66(2) Facts derived through invalid/recovered AST nodes are marked non-authoritative.

§ 66(3) Such facts must never be used as positive proof for code generation.

§ 66(4) Semantic IR/lowering must not consume unresolved recovered invalid constructs.

---

## § 67 Analysis and package implementation milestones

§ 67(1) Implementation packages such as P1–P14 describe implementation progress, not alternative language semantics.

§ 67(2) Newer canonical analysis rulebooks may describe semantics not yet implemented by an older package.

§ 67(3) Such later rulebook synchronization does not retroactively rewrite the historical implementation scope of an already completed package.

§ 67(4) Governance records whether current compiler implementation has caught up with the canonical rule.

§ 67(5) A package-specific amendment should be integrated into the canonical owning rulebook once the semantic model is stabilized, while package documents may remain as implementation history.

---

## § 68 Required test families: architecture

§ 68(1) Tests verify:

```text
fact owner uniqueness;
dependency graph;
fixed-point convergence;
determinism;
snapshot consistency;
per-CompilationPlan separation;
cache invalidation;
open/closed target behavior;
budget exhaustion conservatism.
```

---

## § 69 Required test families: type/interface/Iterator

§ 69(1) Tests include:

```text
type resolution before ownership;
generic conformance;
compiler-known Iterator[T] conformance;
static Next() target;
user-defined iterator implementation;
no naming-convention lookup;
no closed iterable whitelist;
LSP/compiler resolution parity;
specialization preserves conformance/effects.
```

---

## § 70 Required test families: ownership/memory

§ 70(1) Tests include:

```text
availability lattice;
partial/conditional ownership;
borrow NLL;
lifetime/escape;
storage invalidation;
Arena reset/release;
reference epochs;
raw-pointer boundary;
destruction/cleanup;
transferability.
```

---

## § 71 Required test families: interprocedural/effects

§ 71(1) Tests include:

```text
direct/indirect calls;
closure targets;
callbacks;
recursive fixed point;
effects;
guarantees;
allocation context;
parameter usage;
stack demand;
task/thread/ISR roots;
unknown open target conservatism.
```

---

## § 72 Required test families: concurrency/platform

§ 72(1) Tests include:

```text
data race;
deadlock;
ISR cross-analysis proof;
thread-local migration;
atomics;
volatile-not-sync;
FFI unknown effects;
hardware register access;
target/profile mismatch;
ABI/layout facts;
inline assembly trusted facts.
```

---

## § 73 Required test families: diagnostics/tooling

§ 73(1) Tests include:

```text
Invalid versus Unproven diagnostics;
cause-chain provenance;
earliest meaningful error;
advisory suppression;
progressive LSP refinement;
sec analyse reports;
incremental invalidation;
deterministic output.
```

---

## § 74 Completion criteria

§ 74(1) Compiler-analysis architecture is complete when every canonical Sec analysis fact has one owner and all consumers use shared semantic identities.

§ 74(2) Dependency coordination is complete when cyclic/interprocedural analyses converge deterministically without unsafe positive assumptions.

§ 74(3) Snapshot/plan support is complete when cached/incremental/parallel facts cannot cross incompatible `CompilationPlan` or dependency states.

§ 74(4) Required-proof handling is complete when `Valid`, `Invalid`, and `Unproven` are correctly distinguished across normal and strict profiles.

§ 74(5) Tooling support is complete when compiler, LSP, and `sec analyse` consume the same facts at differing allowed budgets.

§ 74(6) Iterator support is complete when `Iterator[T]` is resolved through the canonical interface system and consumed consistently by loop/Semantic IR analysis.

§ 74(7) Analysis architecture must not be marked complete merely because individual local passes exist.

---

## § 75 Core summary

§ 75(1) Sec analysis is a coordinated graph of canonical fact producers/consumers, not one rigid linear list of passes.

§ 75(2) Each semantic fact has one canonical owner.

§ 75(3) Canonical identities are shared across analyses.

§ 75(4) `Valid`, `Invalid`, and `Unproven` are distinct proof outcomes.

§ 75(5) Cyclic/interprocedural analysis uses deterministic fixed points or equivalent convergent mechanisms.

§ 75(6) Analysis budgets reduce refinement, never manufacture positive proof.

§ 75(7) Required target/profile facts are per-`CompilationPlan`.

§ 75(8) Compiler/LSP/`sec analyse` consume the same semantic facts.

§ 75(9) `Iterator[T]` is ordinary compiler-known generic interface conformance resolved by the canonical interface/type system; no separate naming-convention iterator model exists.

§ 75(10) P1–P14 remain implementation-history milestones; canonical analysis rulebook updates record the current language/compiler model without retroactively changing their historical scope.
