# Sec Documentation and Implementation TODO

## Status

This file is a non-normative project roadmap and work list.

All normative Sec language and compiler semantics belong under:

```text
rules/
```

This file must not introduce or override language semantics.

Manuals, examples, AI knowledge files, implementation plans, and books are
derived from the normative rulebooks.

---

# Immediate decisions already established

- `reference_model.md` is the canonical rulebook for safe references,
  generational validity, validity epochs, stable handles, and `RawPtr`
  boundaries.
- `call_graph.md` is the canonical rulebook for callable reachability,
  execution relationships, roots, indirect targets, and call-graph analysis.
- A separate `generational_references.md` must not be created.
- The permanent `lsp.md` rulebook must consume compiler call-graph facts and
  must not define a separate LSP-owned call graph.
- `arena.md` is the next large normative rulebook.
- `compiler_known_members.md` is required before Sec 0.1 documentation closure.
- All normative documents live under `rules/`.
- Derived documents are never normative sources.

---

# Existing string representation members

The current Linux amd64 standard-library write implementation uses:

```sec
value.ptr
value.len
```

These names already exist in active Sec source and must be treated as existing
language/compiler/core integration, not as completely new features.

The compiler-known member work must determine whether lowercase:

```text
.ptr
.len
```

are:

- privileged internal representation members;
- temporary bootstrap spellings;
- permanent language-visible members;
- or implementation details hidden behind public canonical members.

No rename may be performed before all existing source and rules have been
inventoried.

---

# Recommended public naming direction

The recommended canonical public pair is:

```text
Ptr
Len
```

Rationale:

- both are compact low-level systems-programming names;
- `Ptr` is already an abbreviation;
- `Len` is the matching abbreviation;
- exposing both `Len` and `Length` would duplicate one semantic member;
- using `Length` while retaining `Ptr` creates an inconsistent naming pair.

The final decision belongs in:

```text
rules/compiler_known_members.md
```

Possible compatibility handling for existing lowercase `.ptr` and `.len` must
be decided there or in a synchronized core/stdlib rulebook.

---

# Required compiler-known member inventory

The first rulebook pass must cover at least:

```text
Ptr
Len
SizeOf()
ToString()
ToByteArray()
ToCharArray()
ToRuneArray()
```

Required examples include:

```sec
let text := myCharArray[start..<end].ToString()
let size := myInt.SizeOf
let chars := "test string".ToCharArray()
let runes := "test string".ToRuneArray()
```

The inventory must also decide whether each member is:

```text
compiler intrinsic;
compiler-known semantic member lowered to a core helper;
ordinary privileged core implementation;
target-specific helper;
or a combination depending on type and CompilationPlan.
```

The compiler-known/core boundary must be explicit.

---

# Immediate work order

## 1. Synchronize call-graph semantics into `rules/lsp.md`

Status:

```text
Complete
```

Required work:

- add direct callers and callees;
- add possible indirect callers and targets;
- add closed target sets and open callable contracts;
- distinguish direct, interface, function-value, closure, foreign, generated,
  deferred, destruction, task, thread, process, interrupt, and callback
  relationships;
- expose `CompilationPlan` and active target variant;
- distinguish same-stack recursion from task-, thread-, and process-creation
  cycles;
- expose roots reaching a callable;
- expose effect, panic, allocation, stack, and unsafe-provenance paths;
- expose task spawn origin;
- expose `await` and `join` synchronization origin where known;
- expose continuation information;
- define hover, call hierarchy, code lens, navigation, cancellation, partial
  results, and snapshot requirements;
- state that LSP consumes the compiler-owned canonical graph;
- do not create a second LSP-specific graph.

Output:

```text
updated rules/lsp.md
```

---

## 2. Write `rules/compiler_known_members.md`

Status:

```text
Required before arena closure audit
```

The rulebook must define:

- what “compiler-known” means;
- distinction between semantic existence and physical implementation;
- lookup and member resolution;
- interaction with privileged core implementations;
- interaction with user-defined nominal types;
- whether overriding is allowed;
- effects and allocation behavior;
- compile-time evaluation;
- Semantic IR representation;
- target-specific lowering;
- diagnostics;
- formatter behavior;
- LSP behavior;
- tests;
- implementation requirements.

### Initial member set

#### `Ptr`

Define:

- supported types;
- contiguity requirements;
- addressability requirements;
- result type;
- lifetime and borrow relationship;
- empty-view behavior;
- relocation invalidation;
- arena reset/release invalidation;
- FFI retention;
- safe acquisition versus unsafe dereference;
- whether lowercase `.ptr` remains internal or public.

#### `Len`

Define:

- arrays;
- slices;
- strings;
- collections;
- shaped values where applicable;
- compile-time versus runtime value;
- whether lowercase `.len` remains internal or public;
- why `Length` is not a duplicate alias unless an explicit compatibility reason
  is approved.

String length must not remain ambiguous.

Decide whether string `Len` means:

```text
bytes;
runes;
chars;
or another explicitly defined unit.
```

If one generic `Len` cannot be unambiguous, define explicit members such as:

```text
ByteLen
RuneLen
CharLen
```

without inventing them until the string model is audited.

#### `SizeOf()`

Define:

- value-form and possible type-form syntax;
- active target layout;
- compile-time constant behavior;
- incomplete or dynamically sized types;
- arrays, slices, strings, references, handles, and shaped values;
- relation to `layout.md`;
- no value evaluation or memory read.

#### `ToString()`

Define at least:

- primitive types;
- numeric formatting defaults;
- `char`;
- `rune`;
- `char[]` and `ref char[]`;
- `rune[]` and `ref rune[]`;
- byte storage where supported;
- named types;
- user implementations;
- allocation and effects;
- selected-view semantics;
- no implicit terminator search for bounded views.

#### `ToByteArray()`

Define:

- string encoding;
- ownership of the returned array;
- allocation behavior;
- terminator behavior;
- invalid data behavior where applicable;
- relation to `FromByteArray`.

#### `ToCharArray()`

Define:

- exact meaning of `char` for strings;
- ownership;
- allocation;
- element count;
- malformed input behavior if conversion can fail;
- relationship to indexing and slicing;
- exact round-trip expectations.

#### `ToRuneArray()`

Define:

- Unicode scalar-value conversion;
- ownership;
- allocation;
- malformed input behavior;
- relationship to `string.FromRuneArray`;
- exact round-trip expectations.

### Synchronization targets

Synchronize with:

```text
rules/core-library.md
rules/stdlib.md
rules/impl.txt
rules/properties.txt
rules/types.md
rules/collections.md
rules/reference_model.md
rules/raw_pointers.txt
rules/unsafe.md
rules/ffi.txt
rules/layout.md
rules/semantic_ir.txt
rules/effect_analysis.md
rules/lsp.md
```

---

## 3. Design and write `rules/arena.md`

Status:

```text
Next major rulebook
```

The design discussion must resolve:

- arena creation;
- arena ownership;
- arena value semantics;
- capacity;
- backing storage;
- fixed-capacity arenas;
- dynamically backed arenas;
- allocation syntax and API;
- typed allocation;
- array allocation;
- alignment;
- allocation failure;
- initialization;
- destructors;
- reset;
- release;
- validity epoch updates;
- reference invalidation;
- stable handles;
- nested arenas;
- arena transfer;
- task and thread use;
- concurrent allocation;
- concurrent reset/release;
- escape analysis;
- return values;
- closure captures;
- collection backing storage;
- loop use;
- deterministic cleanup;
- panic and cancellation;
- effects:
  - `ArenaCreate`
  - `ArenaAllocate`
  - `ArenaReset`
  - `ArenaRelease`
- compile-time bounded capacity;
- target profiles;
- bare-metal lowering;
- no mandatory runtime;
- diagnostics;
- Semantic IR;
- tests;
- Codex implementation requirements.

Synchronize with:

```text
rules/reference_model.md
rules/allocation.txt
rules/storage.md
rules/layout.md
rules/ownership.md
rules/borrowing.txt
rules/lifetime_analysis.txt
rules/escape_analysis.md
rules/destruction.txt
rules/effect_analysis.md
rules/call_graph.md
rules/tasks.txt
rules/threads.md
rules/cancellation.md
rules/panic.md
rules/runtime_checks.md
rules/target_profiles.md
```

---

# Repository synchronization immediately after the three tasks

Update:

```text
language-rulebook-status.md
rules/rules_implementations.txt
```

Required status changes include:

```text
reference_model.md           Written
call_graph.md                Written
compiler_known_members.md    Written
arena.md                     Written
lsp.md                       Living, synchronized
generational_references.md   Covered by reference_model.md
```

Verify repository existence before marking a file Written.

Do not rely on previously generated download artifacts as proof that a rulebook
has been committed under `rules/`.

---

# Documentation closure inventory

After `arena.md`, create a non-normative inventory:

```text
documentation-closure-inventory.md
```

Classify every planned or candidate rulebook as:

```text
Required standalone rulebook for 0.1
Merge into an existing canonical rulebook
Written but repository synchronization is pending
Written but semantic synchronization is required
Implementation-only document
Derived documentation
Deferred to 0.2 or later
Obsolete
```

For each item record:

```text
canonical owner;
overlapping documents;
dependencies;
blocking questions;
recommended action;
reason;
0.1 release impact.
```

Do not automatically write every historically planned document.

---

# Recommended remaining rulebook order

The following order is dependency-oriented.

The closure inventory may merge or defer items before writing begins.

## Group A — Verify recent rulebooks and foundations

1. Verify and synchronize `attributes.md`.
2. Verify and synchronize `unsafe.md`.
3. Verify and synchronize `effect_analysis.md`.
4. Verify and synchronize `reference_model.md`.
5. Verify and synchronize `call_graph.md`.
6. Synchronize `lsp.md`.
7. Write `compiler_known_members.md`.
8. Write `arena.md`.

Reason:

```text
later analysis, target, failure, storage, and tooling documents depend on these
semantic foundations.
```

---

## Group B — Storage, layout, and runtime failure

Recommended order:

1. `storage.md`
2. `layout.md`
3. `panic.md`
4. `runtime_checks.md`

### `storage.md`

Owns:

```text
stack storage;
static storage;
inline storage;
arena storage;
allocator-backed storage;
caller-provided storage;
collection backing storage;
addressed storage;
storage placement terminology.
```

It must consume `arena.md` and `reference_model.md`.

### `layout.md`

Owns:

```text
size;
alignment;
padding;
field order;
packing;
observable representation;
shaped layout;
memory spaces;
layout guarantees.
```

It must support `SizeOf()` and ABI work.

### `panic.md`

Owns:

```text
panic payload;
propagation;
abort/unwind policy;
cleanup;
destructor panic;
tasks and threads;
FFI boundary;
configured handler or trap.
```

### `runtime_checks.md`

Owns:

```text
bounds;
overflow;
division by zero;
contracts;
shape;
layout;
stale-reference hardening;
profile-selected check behavior.
```

It depends on the panic model.

---

## Group C — Whole-program analyses

Recommended order:

1. `escape_analysis.md`
2. `stack_analysis.md`
3. `isr_analysis.md`

### `escape_analysis.md`

Consumes:

```text
ownership;
borrowing;
reference model;
arena;
closures;
spawn;
callbacks;
FFI retention;
call graph.
```

### `stack_analysis.md`

Consumes:

```text
call graph;
same-stack SCCs;
target ABI;
defer;
destruction;
indirect callable contracts.
```

### `isr_analysis.md`

Consumes:

```text
call graph;
effects;
stack;
allocation;
blocking;
volatile;
atomics;
shared-state analysis;
unsafe provenance.
```

`isr_analysis.md` may require final synchronization after the hardware group.

---

## Group D — Target, hardware, and ABI

Recommended order:

1. `target_profiles.md`
2. `volatile.md`
3. `interrupts.md`
4. `abi.md`
5. `platform_model.md`
6. `inline_assembly.md`
7. final synchronization of `isr_analysis.md`
8. synchronize `ffi.txt`
9. synchronize `registers.txt`

### Reasoning

`target_profiles.md` defines which guarantees and services exist.

`volatile.md` must be stable before interrupt and MMIO rules.

`interrupts.md` defines source and execution semantics.

`abi.md` depends on layout and callable representation.

`platform_model.md` connects target services and source selection.

`inline_assembly.md` consumes ABI, effects, volatile, unsafe, and target rules.

---

## Group E — Modules, initialization, and linking

Recommended order:

1. `modules.md`
2. `initialization.md`
3. `linking.md`

This group must resolve:

```text
module identity;
imports;
cycles;
internal visibility;
static initialization;
deinitialization;
entrypoints;
generic symbol identity;
foreign symbols;
dead stripping;
multiple target outputs.
```

---

## Group F — Compile-time and generic lowering

Recommended order:

1. `compile_time_evaluation.md`
2. `generics_lowering.md`
3. `monomorphization.md`

Compile-time values and evaluation must be stable before final generic lowering
and specialization identity are frozen.

This group must synchronize with:

```text
generics.txt;
call_graph.md;
layout.md;
linking.md;
incremental_compilation.md.
```

---

## Group G — Compiler tooling and verification

Recommended order:

1. `incremental_compilation.md`
2. `compiler_testing.md`
3. `debug_information.md`

### `incremental_compilation.md`

Depends on stable:

```text
semantic identities;
call graph;
generic specializations;
CompilationPlan;
module dependencies;
diagnostic causes.
```

### `compiler_testing.md`

Defines:

```text
unit tests;
valid fixtures;
invalid fixtures;
diagnostic tests;
Semantic IR tests;
MLIR tests;
backend tests;
target matrices;
regression tests;
bootstrap tests.
```

### `debug_information.md`

Depends on:

```text
ABI;
layout;
generics;
tasks;
threads;
continuations;
optimized code;
source mapping.
```

---

# Cross-rulebook semantic synchronization

After all required 0.1 normative documents exist:

1. Create `documentation_graph.yaml`.
2. Assign one canonical owner to every semantic concept.
3. Extract normative statements.
4. Build a conflict matrix.
5. Synchronize terminology.
6. Synchronize examples.
7. Replace duplicated rules with canonical cross-references.
8. Verify every rulebook path under `rules/`.
9. Verify all open 0.1 questions are resolved or explicitly deferred.
10. Produce a semantic synchronization report.

Required non-normative outputs:

```text
documentation_graph.yaml
documentation_graph.md
normative-ownership.md
terminology.md
semantic-sync-report.md
documentation-conflicts.md
```

Release gate:

```text
documentation-conflicts.md contains no unresolved Sec 0.1 semantic conflict.
```

---

# Implementation governance

Local implementation sections remain inside rulebooks.

They own:

```text
required compiler behavior;
required diagnostics;
required tests;
local implementation requirements.
```

A cross-rulebook implementation graph owns:

```text
dependency order;
status;
blockers;
Codex-ready work;
acceptance gates;
critical path;
cross-feature synchronization.
```

Create:

```text
implementation_graph.yaml
implementation_status.md
```

`implementation_graph.yaml` is the non-normative status source.

`implementation_status.md` is the human-readable view.

---

# Implementation graph node requirements

Each implementation node must include:

```text
stable feature ID;
title;
canonical rulebook sections;
hard dependencies;
soft dependencies;
compiler areas;
target/profile scope;
current status by compiler layer;
blockers;
acceptance tests;
diagnostic requirements;
LSP requirements;
unlocked nodes.
```

Allowed status values:

```text
not_applicable
not_started
partial
implemented
verified
blocked
deferred
```

`implemented` means code exists.

`verified` means:

```text
the implementation matches the frozen rulebook;
all acceptance tests pass;
required diagnostics are correct;
required target/profile behavior is verified.
```

---

# Codex governance

Codex may implement a node only when:

```text
the normative rule is sufficiently frozen;
all hard semantic questions are resolved;
all required dependencies are available;
acceptance tests are defined;
the node is marked ready.
```

Codex must not:

```text
invent missing language semantics;
silently choose between conflicting rulebooks;
change a normative rule to simplify implementation;
duplicate an existing analysis structure;
mark a feature verified because it merely compiles.
```

Codex findings must be classified as:

```text
implementation bug;
missing dependency;
specification ambiguity;
specification conflict;
missing test;
target limitation;
design question.
```

Specification ambiguity or conflict returns to design governance.

---

# External input inventories

Before the final Sec 0.1 documentation freeze, perform at least:

## Language-design inventory

Review external ideas concerning:

```text
error handling;
contracts;
ownership;
borrowing;
safe references;
generational handles;
concurrency;
structured concurrency;
pattern matching;
generics;
units;
FFI;
embedded programming;
diagnostics;
tooling ergonomics.
```

## Compiler and safety inventory

Review external ideas concerning:

```text
call graphs;
effects;
escape analysis;
stack analysis;
whole-program analysis;
separate compilation;
incremental compilation;
capability pointers;
memory tagging;
arena invalidation;
ISR verification;
FFI trust;
target profiles;
diagnostic cause chains.
```

Classify every finding:

```text
Adopt for 0.1
Clarify existing 0.1 design
Implementation technique only
Record for 0.2
Reject
Requires experiment
```

---

# Sec 0.1 documentation release gate

Sec 0.1 is documentation-complete when:

```text
all required normative rulebooks exist under rules/;
every normative concept has one owner;
no unresolved semantic conflicts remain;
terminology is synchronized;
examples use current syntax;
planned rulebooks are classified;
acceptance tests exist for major features;
implementation_graph.yaml covers every normative feature;
external pre-freeze inventories are dispositioned;
the normative set has a versioned snapshot.
```

Release artifacts:

```text
SEC-DOCUMENTATION-VERSION
documentation-changelog.md
known-limitations-0.1.md
deferred-to-0.2.md
normative-document-index.md
implementation_graph.yaml
implementation_status.md
```

Recommended tag:

```text
docs-v0.1.0
```

---

# Sec 0.1 implementation tracks

## Go compiler track

Implement the frozen 0.1 specification using:

```text
normative rulebooks;
implementation_graph.yaml;
acceptance tests;
target profiles.
```

## Sec bootstrap track

Continue implementing the compiler in Sec to discover:

```text
syntax friction;
boilerplate;
type ergonomics;
core gaps;
stdlib gaps;
diagnostic weaknesses;
ownership usability;
generic usability;
compiler-workload issues.
```

Bootstrap findings must be classified before changing the specification.

---

# Sec 0.2 milestone

Sec 0.2 means:

```text
external expert feedback has been reviewed;
accepted feedback has been incorporated;
externally discovered contradictions or unsound assumptions are corrected;
central capabilities required for complete bootstrapping are specified;
bootstrap experience has been incorporated where justified;
major remaining bootstrap blockers are resolved or explicitly scoped.
```

External proposals are classified as:

```text
accepted;
accepted with modification;
already covered;
implementation technique;
deferred;
rejected with rationale.
```

---

# Sec 0.3 milestone

Sec 0.3 means:

```text
the complete core-library surface is defined;
the compiler-known/core boundary is stable;
most of the standard-library surface is defined;
core is substantially implemented;
most of stdlib is at least partially implemented;
the bootstrap compiler can rely on a stable language and library foundation.
```

The 0.3 work must distinguish:

```text
compiler-known semantic member;
compiler intrinsic;
privileged core declaration;
core implementation helper;
stdlib API;
platform API.
```

---

# Derived documentation after the 0.1 freeze

Create:

```text
sec-ai-knowledge.md
sec-ai-knowledge-compact.md
manuals/
examples/
book-plan.md
reviewer-pack/
```

Every derived document must state:

```text
Derived from Sec normative rules version: 0.1.x
Not a normative source.
```

---

# Current next actions

```text
[x] Synchronize call_graph.md into rules/lsp.md
[ ] Write rules/compiler_known_members.md
[ ] Design rules/arena.md
[ ] Write rules/arena.md
[x] Synchronize language-rulebook-status.md
[x] Synchronize rules/rules_implementations.txt
[ ] Create documentation-closure-inventory.md
[ ] Classify every remaining planned rulebook
[ ] Complete required Sec 0.1 normative rulebooks in dependency order
[ ] Build documentation_graph.yaml
[ ] Run full semantic synchronization
[ ] Build implementation_graph.yaml
[ ] Generate implementation_status.md
[ ] Perform external input inventories
[ ] Apply accepted pre-freeze findings
[ ] Freeze and tag docs-v0.1.0
[ ] Derive AI knowledge documents
[ ] Derive manuals and examples
[ ] Create the book plan
[ ] Publish the external reviewer pack
[ ] Incorporate accepted external feedback into Sec 0.2
[ ] Stabilize core and most of stdlib for Sec 0.3
```
