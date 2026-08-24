# Sec Program Initialization and Shutdown

- **Status:** Normative
- **Created:** 2026-08-18
- **Last updated:** 2026-08-18
- **Document revision:** 1
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `20b3606`
- **Canonical path:** `rules/compiler/initialization.md`

## 1. Purpose

This rulebook defines the Sec 0.1 model for executable-program initialization,
entry, startup failure, normal shutdown, and target-specific termination.

It owns:

- the distinction between instance construction, compile-time static initialization, program startup, and program shutdown;
- canonical source-level executable entry contracts;
- the boundary between platform/runtime startup and ordinary Sec execution;
- `ProgramInitializationPlan`;
- `ProgramShutdownPlan`;
- startup dependency ordering;
- pre-entry startup failure and partial cleanup;
- runtime teardown ordering;
- integration of deterministic static destruction where supported;
- initialization/shutdown requirements exposed to lowering and linking.

It does not redefine lifecycle `init`, compile-time static evaluation, ordinary
scope cleanup, ownership/borrowing, module imports, ABI classification, linker
assembly, target-profile capabilities, or MLIR syntax. Those remain owned by
their canonical rulebooks.

## 2. Four distinct mechanisms

Sec distinguishes:

```text
Instance construction
Compile-time static initialization
Program startup
Program shutdown
```

A lifecycle `init(...)` member initializes an explicitly constructed instance.
A static initializer determines a static declaration's semantic initial value.
Program startup establishes the prerequisites required before executable entry.
Program shutdown performs the target's defined normal termination sequence.

None of these mechanisms implicitly substitutes for another.

## 3. Lifecycle `init` is not program startup

A lifecycle initializer:

```sec
type Widget struct {
    Value: int
}

impl Widget {
    init(value: int) {
        self.Value = value
    }
}
```

runs only as part of explicit construction:

```sec
let widget := new Widget(10)
```

The existence of `Widget`, its module, or an import of its module does not cause
the lifecycle initializer to execute.

Sec 0.1 has no rule that treats lifecycle `init` as a module or program startup
hook.

## 4. No implicit module initialization

Sec 0.1 has no implicit user-level module initializer.

For example:

```sec
import "storage/database"
```

creates a semantic module dependency. It does not imply:

```text
run database initialization before this module
```

The `ModuleGraph` is not a runtime module-initialization graph.

Runtime application setup is ordinary explicit Sec program flow.

## 5. Explicit runtime application setup

Application setup belongs in ordinary callable code:

```sec
fn main() int {
    let configuration := try LoadConfiguration() {
        Err(error) => {
            ReportStartupError(error)
            return 1
        }
    }

    let database := try OpenDatabase(configuration.Database) {
        Err(error) => {
            ReportStartupError(error)
            return 1
        }
    }

    return Run(database)
}
```

Failures after entry begins follow ordinary Sec `Result`, control-flow, `defer`,
destruction, ownership, and panic rules.

## 6. Static values are semantically initialized before entry

Static initialization is governed by the canonical static and compile-time
evaluation rules.

For example:

```sec
static let MaximumConnections: int := 100
```

has its semantic initial value resolved before runtime program execution.

Program startup does not execute hidden Sec code to evaluate such an initializer.

## 7. Physical static-storage establishment

A loader, runtime, firmware startup stub, or generated target startup sequence
may establish physical storage containing already-resolved static state.

Examples include:

```text
copy initialized data from immutable image storage to writable memory
zero uninitialized storage
establish thread-local backing
prepare platform-required memory regions
```

These are execution-environment operations, not semantic runtime static
initialization.

The distinction is:

```text
compile time:
    resolve static value

artifact construction:
    encode required initial state

program startup:
    establish physical storage containing that state
```

## 8. ProgramInitializationPlan

Every executable concrete `CompilationPlan` has one canonical
`ProgramInitializationPlan`.

Conceptually:

```text
ProgramInitializationPlan {
    Entry
    StaticStorageRequirements
    ExecutionEnvironmentRequirements
    RuntimeRequirements
    StartupActions
    TerminationPolicyRequirements
}
```

The exact in-memory representation is implementation-defined.

The plan contains compiler-, runtime-, platform-, and execution-environment
requirements. It does not introduce hidden user module initialization.

## 9. ProgramShutdownPlan

Every executable concrete `CompilationPlan` also has one canonical
`ProgramShutdownPlan`.

Conceptually:

```text
ProgramShutdownPlan {
    DetachedExecutionShutdown
    StaticDestructionRequirements
    RuntimeTeardownRequirements
    ExecutionEnvironmentTermination
}
```

`ProgramShutdownPlan` is not the mechanical reverse of
`ProgramInitializationPlan`.

Shutdown order is derived from actual lifetime and teardown dependencies.

## 10. Plan ownership and target dependence

Initialization and shutdown plans belong to a concrete `CompilationPlan`.

The same logical Target may therefore produce different plans for different
Variants:

```text
server / linux-amd64
server / windows-amd64
firmware / cortex-m4
```

Differences may include execution-environment startup, runtime facilities,
static-storage establishment, entry adapters, and termination policies.

These differences arise from resolved target/platform facts. The compiler host
is not an implicit initialization input.

## 11. No mandatory Sec runtime

Sec does not require a runtime when the active TargetProfile and reachable
program semantics require none.

A valid plan may have:

```text
RuntimeRequirements = none
```

The compiler must not synthesize an unconditional `sec_runtime_init()` contract
for all programs.

## 12. Runtime requirements are compiler-known

Features requiring runtime support contribute explicit compiler-known
requirements before target lowering:

```text
validated semantic program
        ↓
runtime requirement analysis
        ↓
ProgramInitializationPlan
        ↓
target/runtime lowering
```

Late lowering or the backend must not independently discover and silently add a
competing startup requirement.

## 13. Startup layers

Program startup is conceptually:

```text
Execution-environment startup
        ↓
Required Sec runtime startup
        ↓
Target entry
```

Execution-environment startup establishes platform prerequisites such as
physical static storage, stack/TLS infrastructure, and target runtime state.

Sec runtime startup establishes only facilities required by the resolved
program and TargetProfile.

Only after all mandatory prerequisites are established does source-level Sec
entry execution begin.

## 14. Startup actions are typed requirements

Startup actions are compiler-known typed operations rather than opaque backend
command strings.

An implementation may use categories equivalent to:

```text
EstablishStaticStorage
EstablishTLS
InitializeRuntimeFacility
InitializeScheduler
PlatformStartupAction
TransferToEntry
```

The exact categories are non-normative.

Each action must expose enough structured information for dependency validation,
failure handling, deterministic ordering, lowering, and diagnostics.

## 15. Startup dependency graph

Every mandatory startup action declares its prerequisites.

For example:

```text
InitializeScheduler
    requires AllocatorRuntime
    requires PlatformThreadSupport
```

These relations form a startup dependency graph distinct from `ModuleGraph`,
compile-time static dependencies, ordinary `CallGraph`, and shutdown
dependencies.

The startup graph must be acyclic.

A cycle among mandatory startup requirements makes the concrete
`CompilationPlan` invalid.

## 16. Deterministic startup ordering

The compiler topologically orders the startup dependency graph.

Startup order is not defined by source-file order, module import order,
declaration order, linker input order, registration order, map iteration order,
or compiler worker scheduling.

When independent actions need an observable ordering, the implementation uses a
canonical deterministic tie-breaker that is stable, plan-derived, and
host-independent.

## 17. Startup failure

A pre-entry failure that prevents a mandatory execution-environment or runtime
prerequisite from being established is a `ProgramStartupFailure`.

On `ProgramStartupFailure`:

- source entry has not begun;
- no entry-scope cleanup runs;
- only successfully established startup resources participate in applicable
  failure cleanup;
- cleanup respects declared lifetime/teardown dependencies.

Failures after entry begins are ordinary Sec execution failures.

## 18. Partial startup state

Fallible startup actions are tracked by establishment state.

Conceptually:

```text
established:
    StaticStorage
    TLS
    AllocatorRuntime

failed:
    SchedulerRuntime
```

Cleanup may act only on resources that reached their canonical established
state, unless an owning runtime/platform contract explicitly defines a
recoverable partial state.

## 19. Startup rollback dependencies

If:

```text
RuntimeB requires RuntimeA
```

and `RuntimeB` fails after `RuntimeA` was established, cleanup of `RuntimeA`
must wait until every established dependent resource has been removed.

Failure cleanup therefore follows dependency/lifetime requirements rather than
blindly reversing source or registration order.

## 20. Target entry contracts

Sec 0.1 defines:

```text
command  -> fn main() int
firmware -> fn main() void
library  -> no program entry
test     -> compiler-generated harness entry
```

Future Target kinds may define additional contracts in their owning rules.

## 21. Command entry

A command Target has exactly one concrete, non-generic, module-level source
entry:

```sec
fn main() int
```

It has no source-level parameters.

Command-line arguments, environment variables, process handles, and equivalent
execution-environment information are accessed through ordinary canonical
library/runtime facilities.

Alternate command entry forms such as these are not the Sec 0.1 entry contract:

```sec
fn main() void
fn main(args: string[]) int
fn main() Result[int, Error]
```

## 22. Entry name in the entry module

Within a Target's designated entry module, `main` denotes the compiler-known
entry identity and is not an overload family.

This is invalid:

```sec
fn main() int {
    return 0
}

fn main(value: int) int {
    return value
}
```

The entry declaration must also be non-generic.

## 23. Command termination status

The integer returned by command `main` is the logical termination status:

```text
0       -> successful command completion
nonzero -> unsuccessful command completion
```

The platform's physical encoding or mapping of that status belongs to the
resolved execution environment.

Sec does not define source `int` as physically identical to a kernel/process
exit-status representation.

## 24. No implicit Result-to-exit mapping

Sec 0.1 does not implicitly translate a `Result`-returning `main` into process
termination behavior.

It does not implicitly print errors, choose exit codes, panic, retry startup, or
inject command-line arguments.

User-level startup after entry begins remains explicit ordinary Sec flow.

## 25. Firmware entry

A firmware Target has exactly one concrete, non-generic, module-level source
entry:

```sec
fn main() void
```

For example:

```sec
fn main() void {
    InitializeHardware()

    while true {
        Process()
    }
}
```

A firmware `main` may semantically return.

Return does not implicitly re-enter `main`, reset the device, halt, or idle.
The active TargetProfile/ExecutionEnvironment defines normal firmware
termination behavior.

## 26. Library targets

A library Target has no program entry and does not acquire startup/shutdown
semantics merely by being compiled.

Explicit functions may perform setup when called, but compiling or importing the
library never creates hidden startup execution.

## 27. Test targets

A test Target may use a compiler-generated test harness as target entry.

The generated harness participates in the same initialization and normal
shutdown model as other executable targets.

Test discovery, ordering, result semantics, and reporting belong to compiler
testing rules.

## 28. Physical platform entry and Sec main

The physical operating-system, firmware, or runtime entry point is distinct from
source-level Sec `main`.

A target may use:

```text
OS loader / reset vector
        ↓
platform or generated startup adapter
        ↓
Sec main
```

Source-level `main` does not require `extern "C"` or `extern "system"` merely
because the physical platform entry uses such an ABI.

The startup adapter bridges the execution-environment entry contract to the
native Sec entry contract using the resolved ABI and target rules.

## 29. Entry cleanup before ProgramShutdown

Returning from source-level entry performs ordinary function cleanup first:

```text
main returns
        ↓
ordinary main-scope defer/destruction cleanup completes
        ↓
ProgramShutdown begins
```

Entry-scope cleanup is not global program shutdown.

## 30. Owned child execution at entry completion

Entry execution must not complete while it still owns unresolved structured
child execution entities.

Owned tasks/threads must be resolved according to concurrency rules, for example
by await/join, legal ownership transfer, or legal detach.

Legally detached execution has transferred ownership to the program-level
detached-execution manager/runtime and participates in global shutdown.

## 31. Normal shutdown phases

Normal shutdown follows dependency-respecting phases equivalent to:

```text
1. Entry invocation completes ordinary cleanup.
2. ProgramShutdown begins.
3. Detached Sec execution receives canonical shutdown/cancellation request.
4. Required detached execution cleanup completes.
5. Supported deterministic static destruction executes.
6. Remaining Sec runtime facilities are torn down.
7. Control or termination status is returned to the execution environment.
```

A target omits phases for facilities it does not have.

## 32. Shutdown lifetime dependencies

A facility required by another cleanup operation remains alive until the
dependent cleanup completes.

For example:

```text
StaticObjectA
    depends on AllocatorRuntime

DetachedTaskManager
    depends on SchedulerRuntime

SchedulerRuntime
    depends on PlatformThreadSupport
```

The named dependency must remain established while dependent cleanup executes.

## 33. Static destruction integration

When the active TargetProfile supports deterministic static destruction, the
compiler incorporates a canonical `StaticDestructionPlan` or equivalent
requirements into `ProgramShutdownPlan`.

Static destruction ordering follows canonical dependency/use relationships.

It is not defined by source-file order, declaration order across modules,
reverse import order, linker order, or arbitrary traversal order.

Independent destruction actions use a canonical deterministic tie-breaker when
an observable order must be chosen.

## 34. Shutdown dependency cycles

If teardown constraints require both:

```text
A must remain alive while destroying B
B must remain alive while destroying A
```

and no canonical ownership/destruction rule resolves the relation, the required
deterministic shutdown plan is invalid.

The compiler must not silently choose an arbitrary order.

Diagnostics identify the resources/facilities that form the cycle.

## 35. Normal and forced termination

Sec distinguishes:

```text
NormalTermination
ForcedTermination
```

Normal termination follows `ProgramShutdownPlan`.

Examples include command `main` returning, test harness completion, and firmware
`main` returning into its defined normal target policy.

Forced termination may bypass some or all cleanup where owning target, panic,
destruction, runtime, or execution-environment rules permit or require that
behavior.

This rulebook does not weaken or replace those owning guarantees.

## 36. Runtime teardown

Runtime facilities remain available until all cleanup actions depending on them
have completed.

For example, if static destruction requires allocator services, allocator
teardown occurs after that destruction.

Shutdown is therefore dependency-driven, not simply reverse-startup.

## 37. Semantic IR boundary

Semantic analysis records requirements including:

- target entry identity;
- reachable runtime requirements;
- static lifetime/destruction facts;
- detached-execution requirements;
- source-level termination requirements.

Semantic IR does not need to synthesize target-specific startup machine work
such as copying `.data`, zeroing `.bss`, or invoking a platform CRT entry stub.

Those details belong to plan resolution, platform lowering, and linking.

## 38. Plan construction

After semantic requirements and the concrete `CompilationPlan` are available,
the compiler performs staging equivalent to:

```text
validated semantic program
        +
CompilationPlan
        ↓
ResolveInitializationRequirements
        ↓
VerifyStartupDependencyGraph
        ↓
BuildProgramInitializationPlan
        ↓
ResolveShutdownRequirements
        ↓
VerifyShutdownDependencyGraph
        ↓
BuildProgramShutdownPlan
        ↓
VerifyPlanCompatibility
```

The exact implementation API is non-normative.

## 39. Plan compatibility verification

Before target lowering, the compiler verifies at least:

- source entry contract matches Target kind;
- mandatory runtime facilities are supported by TargetProfile;
- execution-environment prerequisites can be satisfied;
- startup dependencies are acyclic and satisfiable;
- shutdown dependencies are satisfiable;
- required deterministic static destruction is supported where demanded;
- the Target has a valid normal termination policy;
- required detached-execution shutdown support exists;
- startup failure cleanup can preserve declared lifetime requirements.

Failure rejects the concrete `CompilationPlan`.

## 40. MLIR/lowering boundary

Initialization and shutdown requirements are resolved before target-specific
lowering.

Conceptually:

```text
ProgramInitializationPlan
ProgramShutdownPlan
        ↓
platform/runtime lowering
        ↓
generated startup, entry-adapter, and shutdown representation
        ↓
lower MLIR
        ↓
LLVM/backend
```

MLIR materializes resolved plans. It must not independently rediscover and
silently add competing startup/runtime semantics.

Late discovery of a missing semantic/runtime requirement is a compiler invariant
failure or a reason to return to the owning earlier analysis.

## 41. Linking boundary

Initialization rules determine which startup/shutdown facilities are required.

They may expose requirements equivalent to:

```text
required startup object
required entry adapter
required runtime component
required termination adapter
required initialization section
```

`rules/compiler/linking.md` owns how object files, libraries, symbols, sections, entry symbols,
linker arguments, and final artifacts are assembled to satisfy those
requirements.

Initialization owns what must exist and the semantic dependencies.
Linking owns binary construction.

## 42. Diagnostics

Diagnostics use source/target terminology where possible.

Prefer:

```text
this target cannot provide the scheduler required by spawned tasks
```

over internal-only diagnostics such as:

```text
failed to resolve initialization node 17
```

Diagnostics should distinguish at least:

- missing entry;
- invalid entry signature;
- duplicate entry;
- unsupported runtime requirement;
- unsatisfied execution-environment prerequisite;
- startup dependency cycle;
- invalid startup rollback dependency;
- unsupported normal termination policy;
- unsupported deterministic static destruction requirement;
- shutdown dependency cycle;
- incompatible initialization/shutdown plan.

Cycle diagnostics show a deterministic representative dependency cycle.

## 43. Determinism

For the same validated source program, semantic requirements, `CompilationPlan`,
TargetProfile, runtime/environment model, and initialization schema, the compiler
produces the same:

- entry contract;
- startup requirements;
- startup dependency graph;
- `ProgramInitializationPlan`;
- shutdown dependency graph;
- `ProgramShutdownPlan`;
- deterministic tie-break ordering;
- initialization diagnostics.

The result does not depend on compiler host, filesystem enumeration, map
iteration, source traversal, or worker scheduling.

## 44. Incremental dependencies

Initialization planning depends on ABI- and target-relevant facts including, as
applicable:

- Target kind;
- Target entry module;
- entry signature;
- reachable runtime requirements;
- detached-execution usage;
- static lifetime/destruction requirements;
- TargetProfile capabilities;
- RuntimeEnvironment;
- ExecutionEnvironment;
- MemoryEnvironment;
- ABI requirements of generated adapters;
- termination policy;
- initialization-plan schema.

A change that provably preserves the complete initialization/shutdown contract
does not require downstream invalidation solely for initialization reasons.

A change that alters the contract invalidates dependent lowering and linking
artifacts.

Detailed cache architecture belongs to `incremental_compilation.md`.

## 45. Required test families

A conforming implementation includes regression coverage for:

### 45.1 Entry validation

- valid command `fn main() int`;
- missing command entry;
- wrong command return type;
- command entry with parameters;
- generic entry;
- duplicate/overloaded `main` in entry module;
- valid firmware `fn main() void`;
- invalid firmware entry;
- library target without entry;
- generated test harness entry.

### 45.2 No hidden module initialization

- importing a module does not emit or execute user module init;
- changing import order does not change startup execution;
- lifecycle `init` members execute only for explicit construction.

### 45.3 Static storage

- compile-time static values are available at entry;
- physical data/BSS/TLS establishment does not invoke hidden Sec static
  initializer code;
- target-specific storage establishment preserves semantic static state.

### 45.4 Runtime requirements

- runtime-free executable;
- allocator-requiring executable;
- scheduler-requiring executable;
- rejected runtime requirement on a profile that forbids it;
- transitive startup prerequisites.

### 45.5 Startup dependencies

- deterministic topological order;
- deterministic independent-action tie-break;
- startup cycle rejection;
- partial startup failure;
- rollback only for established facilities;
- dependency-correct rollback.

### 45.6 Entry completion and shutdown

- ordinary `main` locals clean before ProgramShutdown;
- command return-status propagation;
- user startup `Result` handling remains ordinary flow;
- unresolved owned child execution is rejected where concurrency requires
  resolution;
- detached execution transfers to global shutdown;
- detached cleanup precedes dependent runtime teardown;
- deterministic static destruction where supported;
- shutdown dependency cycle rejection;
- normal versus forced termination differences.

### 45.7 Multi-Variant and cross compilation

- one logical Target with Variant-specific plans;
- hosted versus bare-metal startup;
- requirements resolved from target facts rather than compiler host;
- deterministic plan representation/fingerprints where implemented.

### 45.8 Lowering/linking boundary

- initialization plan drives generated entry/startup lowering;
- MLIR does not independently add runtime initialization;
- linking receives startup/runtime/termination requirements from the plan;
- stale plan-dependent lowering is invalidated after relevant target/runtime
  changes.

## 46. Completion criteria

The Sec 0.1 initialization implementation is complete when every executable
concrete `CompilationPlan` can deterministically derive and verify:

1. canonical target entry contract;
2. execution-environment startup requirements;
3. reachable Sec runtime requirements;
4. startup dependency graph;
5. canonical `ProgramInitializationPlan`;
6. partial-state startup failure cleanup;
7. detached-execution shutdown requirements;
8. deterministic static-destruction requirements where supported;
9. runtime teardown dependencies;
10. canonical `ProgramShutdownPlan`;
11. valid target termination policy;
12. linking-facing startup/shutdown requirements;
13. lowering-facing plan data consumed rather than rediscovered;
14. deterministic diagnostics for invalid plans.

The implementation must do so without:

- implicit user-level module initialization;
- runtime evaluation of compile-time static initializers;
- source-order startup semantics;
- reverse-import-order destruction;
- backend-discovered hidden runtime requirements;
- unconditional mandatory Sec runtime startup;
- arbitrary shutdown ordering.

## 47. Non-goals for Sec 0.1

This rulebook does not require:

- C++-style global constructors;
- Go-style package initialization;
- user-defined module startup hooks;
- hidden lifecycle `init` execution;
- implicit `Result`-returning `main`;
- compiler-injected `main` argument arrays;
- mandatory runtime startup for runtime-free programs;
- implicit firmware reset/re-entry after `main` returns;
- one universal shutdown sequence across all TargetProfiles;
- linker-specific object/section naming;
- target-independent physical process-entry implementation.
