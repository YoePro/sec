# Sec Target Profiles

- **Status:** Normative
- **Created:** 2026-08-24
- **Last updated:** 2026-08-24
- **Document revision:** 1
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `45e5cd4`
- **Canonical path:** `rules/platform/target_profiles.md`

## 1. Purpose

This rulebook defines the Sec 0.1 target-profile model.

A `TargetProfile` is the canonical semantic capability, constraint, runtime-policy,
execution-policy, and resource-policy profile applied to one concrete
`CompilationPlan`.

This rulebook owns:

- canonical target-profile families;
- profile-family baseline dispositions;
- target-profile capability and policy resolution;
- context-sensitive execution restrictions;
- typed resource constraints and enforcement policies;
- derived/custom profile semantics;
- override precedence and legality;
- profile validation;
- immutable `ResolvedTargetProfile`;
- profile identity, provenance, and fingerprints;
- canonical profile queries consumed by compiler subsystems.

This rulebook does not define architecture/CPU identity, device memory maps,
ABI classification, runtime internals, scheduler algorithms, concurrency
semantics, interrupt semantics, MMIO/volatile semantics, stack/ISR analysis
algorithms, BuildProfile optimization semantics, linker mechanics, or exact
project configuration syntax.

## 2. Role and neighboring models

`TargetProfile` answers:

> Which semantic facilities, runtime capabilities, execution behaviors, safety
> policies, and resource constraints may this concrete Sec program rely on?

The following remain distinct:

```text
PlatformIdentity
ArchitectureModel
DeviceModel
RuntimeEnvironment
ExecutionEnvironment
TargetProfile
BuildProfile
```

Platform and device models describe what the target can provide.
`RuntimeEnvironment` describes available runtime services.
`ExecutionEnvironment` describes execution contexts and behavior.
`TargetProfile` selects and constrains which supported facilities are enabled,
disabled, required, or context-restricted.
`BuildProfile` controls optimization/debug/instrumentation choices and must not
silently change target-profile semantics.

## 3. Availability and activation

Capability availability and activation are separate:

```text
Availability:
    Supported
    Unsupported
    Unknown

Activation:
    Enabled
    Disabled
    NotApplicable
```

A profile may disable a supported capability. It may not enable or require a
capability the resolved platform cannot provide.

Program requirements never implicitly mutate or broaden the selected profile.

## 4. Canonical profile families

Sec 0.1 defines compiler-known families:

```text
Hosted
RTOS
BareMetal
```

They are capability/policy baselines, not OS, CPU, architecture, device, runtime,
or ABI identities.

Hosted does not imply all runtime facilities exist.
RTOS requires a task/scheduler execution facility but does not require native OS
threads.
BareMetal does not imply that no Sec runtime facilities may exist.

## 5. Family baseline dispositions

Profile-family baselines use dispositions equivalent to:

```text
Required
EnabledIfSupported
Disabled
TargetDefined
```

`Required` requires availability and enables the capability.
`EnabledIfSupported` enables it when supported and otherwise resolves disabled.
`Disabled` keeps it inactive even if available.
`TargetDefined` requires concrete target/platform resolution before
`CompilationPlan` freeze.

These are resolution inputs only. A frozen profile contains fully resolved facts.

## 6. Sec 0.1 baseline matrix

| Capability / policy | Hosted | RTOS | BareMetal |
|---|---|---|---|
| Ordinary hosted process lifecycle | Required | Disabled | Disabled |
| Heap allocation | EnabledIfSupported | Disabled | Disabled |
| Native threads | EnabledIfSupported | TargetDefined | Disabled |
| Sec tasks | EnabledIfSupported | Required | Disabled |
| Task scheduler | EnabledIfSupported | Required | Disabled |
| Detached execution | EnabledIfSupported | TargetDefined | Disabled |
| Thread-local storage | EnabledIfSupported | TargetDefined | Disabled |
| Task-local runtime storage | EnabledIfSupported | EnabledIfSupported | Disabled |
| Synchronization primitives | EnabledIfSupported | EnabledIfSupported | EnabledIfSupported |
| Atomics | EnabledIfSupported | EnabledIfSupported | EnabledIfSupported |
| Interrupt execution | Disabled | EnabledIfSupported | EnabledIfSupported |
| MMIO/device access | Disabled | EnabledIfSupported | EnabledIfSupported |
| Dynamic linking | EnabledIfSupported | Disabled | Disabled |
| Deterministic static destruction | EnabledIfSupported | TargetDefined | Disabled |
| Mandatory Sec safety semantics | Required | Required | Required |
| Defined panic-handling strategy | Required | Required | Required |

## 7. Allocation

Heap-allocation capability is distinct from ordinary automatic/static storage.
Disabling heap allocation forbids operations that require dynamic allocation but
does not prohibit values whose semantics can be implemented using automatic or
static storage.

Hosted enables supported heap allocation by default.
RTOS and BareMetal disable it by default, but a concrete profile may enable it
when a compatible allocator is provided.

## 8. Tasks, threads, scheduler, and detached execution

Tasks and threads remain distinct execution entities and have distinct capability
and resource facts.

RTOS requires Sec tasks and scheduler support but not native threads.
BareMetal disables tasks/scheduler by default but may enable supported scheduler
facilities.

Detached execution is separate from task creation and may be enabled only when
the runtime/termination model can satisfy ownership, cancellation/shutdown, and
cleanup requirements.

## 9. Blocking and interrupt-context policy

Blocking is a typed context-sensitive policy, not one global boolean.

The profile may constrain canonical concurrency effect classes such as:

```text
NonBlocking
TaskSuspending
ThreadBlocking
IndefinitelyBlocking
CancellationAware
NonCancellable
```

Where interrupt execution exists, the default interrupt-context baseline is:

```text
HeapAllocation = Forbidden
Blocking = Forbidden
```

unless an owning interrupt rule establishes a stronger safe contract.

Ordinary Hosted profiles disable direct interrupt execution and MMIO/device
access by default. RTOS and BareMetal enable them only when supported by
resolved platform/device/memory facts.

## 10. Synchronization and atomics

Synchronization and atomic facilities depend on canonical implementation
capability rather than one specific native instruction. A legal implementation
may use native instructions, runtime helpers, critical-section lowering, or
another strategy permitted by the owning rules.

## 11. Dynamic linking and deterministic static destruction

Hosted enables dynamic linking where supported by the `LinkEnvironment`.
RTOS and BareMetal disable it by default.

Hosted enables deterministic static destruction where supported.
RTOS leaves it target-defined.
BareMetal disables it by default, but may enable it when the target defines a
normal termination contract and required support.

## 12. SafetyCheckPolicy

All Sec 0.1 profile families preserve mandatory Sec safety semantics.

A required safety condition must either:

1. be proven statically; or
2. be checked dynamically when execution can otherwise violate it.

A dynamic safety check does not by itself require a Sec runtime; it may lower to
ordinary inline machine code.

The compiler may remove a dynamic check only when the owning rules prove the
condition cannot fail.

Neither `TargetProfile` nor `BuildProfile` may convert mandatory Sec safety into
undefined behavior for performance.

## 13. Panic policy

Every valid profile provides a defined panic-handling strategy for reachable
panic behavior. This does not require a heavy unwind runtime.

A context may forbid reachable panic. If an owning analysis proves a panic path
reachable in such a context, the program is invalid for that profile.

Lack of a large runtime never turns reachable panic into undefined behavior.

## 14. Typed resource constraints

Capability activation and quantitative resource limits are separate.

Examples:

```text
MaximumTasks: ResourceLimit[Count]
MaximumDetachedTasks: ResourceLimit[Count]
MaximumTasksPerParent: ResourceLimit[Count]
MaximumThreads: ResourceLimit[Count]
SchedulerQueueCapacity: ResourceLimit[Count]

MainStackBudget: ResourceLimit[Bytes]
ThreadStackBudget: ResourceLimit[Bytes]
TaskStackBudget: ResourceLimit[Bytes]
InterruptStackBudget: ResourceLimit[Bytes]

StaticStorageBudget: ResourceLimit[Bytes]
HeapBudget: ResourceLimit[Bytes]
```

Resource domains are typed so counts, bytes, and other units cannot be
interchanged accidentally.

## 15. Resource-limit states

Resolved resource limits use states equivalent to:

```text
KnownLimit(value)
NoDeclaredLimit
UnknownLimit
```

`KnownLimit` provides a canonical finite bound and may participate in static
proofs.

`NoDeclaredLimit` means the profile imposes no finite semantic limit. It does not
claim infinite physical resources.

`UnknownLimit` means no usable canonical bound is known. It is neither zero nor
unlimited and must never provide a positive static proof.

## 16. Stack and execution-resource policy

Stack policy distinguishes relevant execution kinds such as main, thread, task,
and interrupt stacks. Each may have a distinct stack model and budget.

The profile provides facts only; stack/ISR analyses determine program
requirements and compare them with those constraints.

Task constraints may independently cover total tasks, detached tasks, per-parent
tasks, and scheduler queue capacity. Thread constraints remain separate.
Sec does not assume one task equals one thread.

## 17. Heap budgets

Heap capability and heap budget are separate facts.

For example:

```text
HeapAllocation = Enabled
HeapBudget = KnownLimit(32768 bytes)
```

and:

```text
HeapAllocation = Enabled
HeapBudget = NoDeclaredLimit
```

have different resource semantics while both permit dynamic allocation.

## 18. Resource enforcement

A resource constraint may carry an enforcement policy equivalent to:

```text
StaticRequired
RuntimeEnforced
Advisory
```

`StaticRequired` requires the owning compiler analysis to prove the bound.

`RuntimeEnforced` permits dynamic use only when the selected runtime/platform
defines required exhaustion/failure behavior.

`Advisory` guides diagnostics or optimization without defining validity.

## 19. Compiler-known execution contexts

Context-sensitive policies use compiler-known contexts, including as applicable:

```text
Ordinary
Task
Thread
Interrupt
Startup
Shutdown
```

Projects do not create arbitrary string-named contexts as language semantics.

A context-specific policy may inherit or restrict a globally enabled capability.
It may not enable a capability that is globally disabled or unavailable.

## 20. Cancellation policy

Cancellation support is distinct from task availability. A profile may resolve
cancellation to states equivalent to:

```text
Supported
Restricted
Unavailable
```

with context-sensitive restrictions where required.

Detached execution may be enabled only when the owning shutdown/cancellation
contract is satisfiable.

## 21. Provenance

Resolved profile facts retain provenance from sources such as:

```text
profile family
target/platform registry
device definition
runtime definition
project Target configuration
project Variant configuration
explicit legal CLI override
compiler-derived target fact
```

Diagnostics use provenance to explain why a capability, policy, or resource
constraint has its value.

## 22. Explicit resolved resource state

A frozen profile need not provide a numeric bound for every resource, but every
semantically relevant resource fact must have an explicit state such as
`KnownLimit`, `NoDeclaredLimit`, or `UnknownLimit`.

Missing/null configuration is not implicit resource semantics.

## 23. Profile resolution phase

Profile resolution is explicit:

```text
Profile family baseline
        +
target/platform defaults
        +
device/runtime constraints
        +
project Target/Variant configuration
        +
legal explicit overrides
        ↓
ResolveTargetProfile
        ↓
ValidateTargetProfile
        ↓
ResolvedTargetProfile
        ↓
freeze CompilationPlan
```

No unresolved family disposition remains in a frozen plan.

## 24. Derived/custom profiles

Sec 0.1 permits target-, platform-, project-, or implementation-defined derived
profiles.

Conceptually:

```text
ProfileIdentity
BaseProfileLineage
ResolvedFacts
```

A derived profile has one canonical base profile lineage in Sec 0.1.
Multiple profile inheritance is not required.

A BareMetal-derived profile may enable allocator/tasks/interrupts while remaining
BareMetal if it still follows the BareMetal execution baseline.

## 25. Override precedence

Resolution applies this precedence from lower to higher authority:

```text
1. canonical profile-family baseline
2. target/platform profile defaults
3. device/runtime-required constraints
4. project Target/Variant profile configuration
5. explicit CLI override where legally overridable
```

Higher precedence does not override hard incompatibilities.

## 26. Constraints and preferences

Hard availability/physical constraints are distinct from policy preferences.

For example:

```text
Device:
    Interrupts = Unsupported
```

cannot be changed by CLI into Supported.

A project preference such as:

```text
HeapAllocation = Disabled
```

may be changed where the property is overridable and the platform supports the
new value.

## 27. Property mutability classes

Profile properties may carry metadata equivalent to:

```text
Overridable
FixedByTarget
FixedByDevice
Derived
```

Not every fact is CLI-overridable.

No override may weaken mandatory Sec safety semantics, enable an unsupported
capability, or enlarge a physical hard limit beyond canonical device/platform
facts.

## 28. Early profile validation

A profile configuration inconsistent with target facts is rejected during profile
resolution even when source code does not use the conflicting feature.

This differs from a valid profile against which the program later requires a
disabled capability.

Diagnostics distinguish these cases.

## 29. Program requirements against frozen profile

After resolution:

```text
ProgramRequirements
        ×
ResolvedTargetProfile
        ↓
valid / invalid / unproven
```

Program requirements never mutate the profile.

## 30. Canonical profile queries

Compiler subsystems consume typed profile queries conceptually equivalent to:

```text
profile.Capability(...)
profile.Policy(...)
profile.ContextPolicy(...)
profile.ResourceConstraint(...)
```

Subsystems must not reconstruct profile semantics from raw target/project config.

## 31. Sema and analysis consumption

Sema performs directly decidable profile legality checks against the frozen
profile.

Whole-program analyses consume the same immutable profile for transitive proofs,
including stack, ISR, resource, effect, and concurrency analyses.

The profile supplies constraints. The owning analysis supplies proof logic.

## 32. Initialization, lowering, linking, and LSP

Initialization, lowering, and linking consume already-resolved profile facts from
the `CompilationPlan`; they do not re-resolve project profile configuration.

The LSP uses the same canonical profile resolution as CLI builds. Analysis depth
may differ (`Interactive`, `Standard`, `Deep`), but profile semantics do not.

The LSP must not assume Hosted for convenience.

## 33. Profile identity and fingerprints

Every `ResolvedTargetProfile` has deterministic canonical identity and
fingerprint.

Implementations may use narrower sub-fingerprints such as:

```text
CapabilityFingerprint
ExecutionPolicyFingerprint
ResourceConstraintFingerprint
SafetyPolicyFingerprint
```

All profile facts that can affect compilation participate in appropriate
dependency fingerprints.

## 34. Incremental compatibility

Profile-dependent cache reuse is dependency-based, not controlled by one global
profile-compatibility boolean.

A change to `MaximumTasks` may invalidate concurrency/resource analyses without
invalidating ABI/layout when those artifacts provably do not depend on that fact.

## 35. Diagnostics

Profile diagnostics distinguish at least:

```text
capability unsupported
capability supported but disabled
profile configuration inconsistent
illegal override
resource bound exceeded
required static proof unavailable
context policy violation
panic forbidden in context
runtime-enforced resource unavailable
```

Diagnostics preserve profile-fact provenance and explain both the program
requirement and the origin of the constraint.

## 36. Determinism

For the same canonical profile inputs, resolution produces identical:

```text
ResolvedTargetProfile
capability states
policy states
resource constraints
context policies
provenance
profile fingerprints
diagnostics
```

independent of compiler host, filesystem order, map iteration, source traversal,
or worker scheduling.

## 37. Required test families

A conforming Sec 0.1 implementation covers at least:

- Hosted, RTOS, and BareMetal family baseline resolution;
- Required, EnabledIfSupported, Disabled, and TargetDefined dispositions;
- Supported/Unsupported/Unknown versus Enabled/Disabled;
- heap allocation defaults and legal explicit enablement;
- runtime-free and scheduler-enabled BareMetal;
- RTOS task/scheduler requirements without native-thread assumptions;
- detached-execution capability and shutdown/cancellation prerequisites;
- Hosted versus embedded interrupt/MMIO baselines;
- interrupt blocking/allocation/panic restrictions;
- dynamic safety check behavior without mandatory Sec runtime;
- inability of BuildProfile to weaken safety;
- defined panic strategy;
- KnownLimit, NoDeclaredLimit, and UnknownLimit behavior;
- UnknownLimit never yielding positive proof;
- typed resource domains;
- stack budgets by execution context;
- separate thread/task limits;
- StaticRequired, RuntimeEnforced, and Advisory enforcement;
- derived profile single lineage;
- target/device constraints and legal/illegal project/CLI overrides;
- identical profile semantics in Sema, analyses, initialization, linking, and LSP;
- deterministic profile fingerprints and selective invalidation;
- cross-compilation proving target facts, not host facts, define the profile.

## 38. Completion criteria

The Sec 0.1 target-profile implementation is complete when every concrete
`CompilationPlan` can deterministically:

1. select one canonical family or derived profile lineage;
2. apply family baselines;
3. merge target/platform defaults;
4. apply device/runtime hard constraints;
5. apply project Target/Variant policy;
6. apply only legal explicit overrides;
7. validate platform/profile consistency;
8. resolve every relevant capability activation state;
9. resolve semantically relevant context policies;
10. resolve every resource constraint to an explicit state;
11. attach enforcement policies where required;
12. preserve provenance;
13. produce one immutable `ResolvedTargetProfile`;
14. derive canonical profile fingerprints;
15. expose typed profile queries to downstream compiler consumers;
16. invalidate only relevant profile-dependent results;
17. emit diagnostics that distinguish unsupported, disabled, constrained, and
    unproven cases.

A frozen `CompilationPlan` contains no unresolved family disposition, ambiguous
profile default, implicit host-derived capability, or null/missing profile
property with semantic meaning.

## 39. Non-goals for Sec 0.1

This rulebook does not require:

- arbitrary user-defined string capability keys;
- multiple profile inheritance;
- one finite numeric resource budget for every target;
- one task-to-thread mapping;
- dynamic allocation in every profile;
- a heavy runtime for dynamic safety checks;
- a heavy unwind runtime for panic;
- Hosted as "all capabilities enabled";
- BareMetal as "no runtime";
- RTOS as a Hosted subtype;
- BuildProfile-based weakening of mandatory Sec safety;
- program requirements automatically enabling disabled capabilities;
- downstream target-specific ad hoc profile reconstruction.
