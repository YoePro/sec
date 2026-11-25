# Sec Target Profiles

- **Status:** Normative
- **Created:** 2026-08-24
- **Last updated:** 2026-11-25 (§ 13 Arena epoch exhaustion, MD-140); 2026-11-24 (§§ 19, 29–32 atomic context categories and capability proofs, MD-095); 2026-11-17 (§ 7 MD-034 freestanding allocation policy); 2026-09-24
- **Document revision:** 1
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `45e5cd4`
- **Canonical path:** `rules/platform/target_profiles.md`
- **Implementation governance:** `governance/platform.yaml`

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

MD-034 (owner decision 2026-11-17;
`rules/corrections/applied/MD-034-freestanding-allocation-principle-20261117-20261117.md`):
freestanding is not synonymous with `noalloc`. A freestanding/BareMetal
configuration has dynamic heap allocation disabled by default, as in the § 6
baseline. A resolved profile may explicitly enable a compatible allocation
capability only when the target/platform facts positively support it and the
provider's obligations are met (§ 3). `Availability.Unsupported`,
`Availability.Unknown` and `Activation.Disabled` remain distinct: an `Unknown`
provider never proves an allocator exists, and no host-runtime or global-heap
fallback is implied. The selected profile, platform facts and legal overrides
resolve into one frozen `ResolvedTargetProfile` (§§ 23–38); a target field
spelled `freestanding` is not by itself a resolved allocation policy.
Allocation-free code remains legal on every profile, and a freestanding program
must remain able to run without a mandatory general Sec runtime or heap
allocator.

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

A program using a select `after` branch requires a target/profile capability
for the canonical monotonic deadline semantics. A statically unsupported or
disabled timer capability is a compile-time profile diagnostic; it is not
lowered to a runtime `SelectError`.

## 11. Dynamic linking and deterministic static destruction

Hosted enables dynamic linking where supported by the `LinkEnvironment`.
RTOS and BareMetal disable it by default.

Hosted enables deterministic static destruction where supported.
RTOS leaves it target-defined.
BareMetal disables it by default, but may enable it when the target defines a
normal termination contract and required support.

The resolved `CompilationPlan` selects exactly one compatible platform C binding
environment for the target, normally provided from `sec/platform`. It supplies
target-specific C binding declarations such as `c::stddef::size_t` and
`c::time::time_t` under `c::` without a source import; the compiler owns the
fundamental C ABI types. Missing, incompatible, ambiguous, stale, or untrusted
binding metadata fails closed and never falls back to the host C ABI. The source
or metadata serialization and the selection mechanism of that environment are
not yet specified (MD-021, `rules/corrections/applied/md020-md021-md023-md024-ffi-c-bindings-null-correction-20261008.md` § 3; `rules/platform/ffi.md` § 6).

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

**MD-140 (2026-11-25; `rules/corrections/applied/MD-140-arena-epoch-exhaustion-correction-2026-11-24-20261125.md`).** A profile selects only the
verified physical realization of a panic, never a different language outcome.
In particular Arena Reset epoch exhaustion is the panic
`panic.arena-epoch-exhaustion` for Hosted, RTOS, BareMetal and every derived or
custom profile (`rules/memory/arena.md` § 39); a target trap is one possible
realization. A profile or reference policy that selects rekeying, domain
recreation, retirement or a fallible Reset for this API is invalid, and a
shorter epoch width is resolved together with a justification consistent with
that rule.

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

**MD-095 (2026-11-24; `rules/corrections/applied/MD-095-atomic-capability-requirements-correction-2026-11-24-20261124.md`).** These six names are the canonical
`ContextID` categories of capability requirements (`rules/concurrency/atomics.md`
§ 37A(8)). A task ID, thread ID, source function name or interrupt vector never
replaces a category, and a logical task is not `Thread` because it runs on a
physical worker. The category comes from the existing execution-context analysis
and the actual invocation contract; `NoBlocking` and `InterruptSafe` are
requirements of that contract, and `Interrupt` must prove both. Canonical target
facts such as vector, priority, masking domain, core or execution domain may
refine or restrict a category's proof when the target distinguishes them; they
create no context and enable no globally forbidden capability. A category-wide
capability discharges a concrete use only when its proof covers that use's
restrictions; otherwise the obligation stays open or a proven incompatible use is
rejected. A callable reachable from several contexts is checked under each, and
unresolved reachability is an open obligation, never an assumed `Ordinary`.

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

For a consuming task await, caller cancellation before await commit retains the
child lifecycle and may delay terminal caller cancellation until cooperative
child cleanup finishes. A target profile selects the physical suspension or
blocking mechanism but must not treat caller cancellation as implicit detach.

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

**MD-095.** For a capability requirement such as a concrete atomic operation,
`valid` needs a supported proof, `invalid` is a proven unsupported or disabled
capability, and `unproven` is an `Unknown` obligation that analysis may carry
but executable emission must discharge or reject (`atomics.md` §§ 37A(11)–(12)).

## 30. Canonical profile queries

Compiler subsystems consume typed profile queries conceptually equivalent to:

```text
profile.Capability(...)
profile.Policy(...)
profile.ContextPolicy(...)
profile.ResourceConstraint(...)
```

Subsystems must not reconstruct profile semantics from raw target/project config.

**MD-095.** Atomic capability is answered by one canonical plan capability
service keyed by carrier, representation, access extent, guaranteed alignment,
operation, orders, context category and additional requirements; matching,
normalization and context refinement belong to that service only
(`atomics.md` §§ 37A(9)–(10)).

## 31. Sema and analysis consumption

Sema performs directly decidable profile legality checks against the frozen
profile.

Whole-program analyses consume the same immutable profile for transitive proofs,
including stack, ISR, resource, effect, and concurrency analyses.

The profile supplies constraints. The owning analysis supplies proof logic.

**MD-095.** Sema may continue analysis of a semantically valid atomic operation
whose capability is `Unknown`; it keeps the obligation instead of reporting a
proven unsupported capability or treating it as verified.

## 32. Initialization, lowering, linking, and LSP

Initialization, lowering, and linking consume already-resolved profile facts from
the `CompilationPlan`; they do not re-resolve project profile configuration.

The LSP uses the same canonical profile resolution as CLI builds. Analysis depth
may differ (`Interactive`, `Standard`, `Deep`), but profile semantics do not.

The LSP must not assume Hosted for convenience.

**MD-095.** Lowering consumes the validated atomic capability evidence and fails
closed with a missing-evidence diagnostic when a required proof is still unknown,
including every ISR or nonblocking obligation. The LSP keeps such obligations
unproven and never presents them as executable validity.

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

Semantic `Atomic[T]` eligibility is target-independent language validation.
The frozen `CompilationPlan` separately records concrete `T`
layout/alignment, operation and requested-order capability, native/emulated
realization, blocking behavior, and interrupt safety. A semantically valid
`Atomic[uint64]` may therefore fail a selected target operation capability
without becoming an invalid Sec type. Compiler-host capabilities and target word
size never define language eligibility.

## 40. Native layout facts (MD-141, MD-142)

Integrated 2026-11-20 from `rules/corrections/applied/MD-141-MD-142-scalar-and-aggregate-layout-normative-correction-2026-11-20-20261121.md` § 11(3).

**§ 40(1)** A `CompilationPlan` that claims layout-sensitive lowering carries one immutable, normalized native-layout fact set: the target layout identity and its provenance (toolchain/provider and version, or the validated maintained description), the ABI identity, the required alignment of every scalar storage representation it supports (`IntegerStorage(8..256)`, `BinaryFloatStorage(32, 64)`; `rules/memory/layout.md` § 47.2), any verified native aggregate or array constraints and the address-space distinctions that affect them.

**§ 40(2)** The facts are keyed by representation class and width, are positive powers of two, and are validated against the representation the backend will emit. Absence of an explicit data-layout entry is not permission to guess: the producer applies the toolchain's documented default and verifies it, or leaves the fact unresolved.

**§ 40(3)** A plan whose facts are incomplete may still analyse and type-check, but it is analysis-only for the affected layouts: dependent layouts are incomplete with the missing representation named, an unsupported representation is unsupported, and contradictory facts are invalid. A CPU or ABI variant never inherits another variant's facts because the architecture spelling matches.

**§ 40(4)** The fact set and the compiler layout revision are part of the layout cache key; plans with different facts never share a completed layout.

## 41. Target profile and allocation-provider selection (MD-139)

Integrated 2026-11-20 from `rules/corrections/applied/MD-139-allocation-profile-provider-selection-proposed-correction-2026-11-20-20261121.md` (owner accepted the manifest spelling 2026-11-20). MD-034 stays decided and independent.

**§ 41(1)** The target-profile family (`hosted`, `rtos`, `baremetal`) is selected by the Variant/Target `profile` field (`rules/projects/projects.md` § 19(11)); without it the registry's explicit per-target default applies, never a heuristic from OS or architecture names. Transitional registry labels are mapped per target: `hosted` → Hosted, `rtos` → RTOS, `freestanding` → BareMetal. A missing or contradictory mapping fails closed.

**§ 41(2)** A derived target profile resolves to exactly one lineage (§ 24). An unknown or ambiguous profile, or a reclassification that changes the target's OS, CPU, ABI or mandatory device capabilities, is a configuration error even when the program allocates nothing.

**§ 41(3)** The allocation `mode` (`hosted`, `embedded-arena`, `noalloc`) is independent of the family. When `mode` is omitted the family baseline applies: Hosted enables allocation only if supported, RTOS and BareMetal start disabled (§§ 5–7). Availability and activation are reported separately.

**§ 41(4)** A `provider` names a canonical identity in the target/platform provider catalog with positive provenance, availability, context and reclamation behavior and an implementation binding. A provider not in the selected target's catalog, or one declared for another target or ABI, is a configuration error. `provider` with `noalloc` is contradictory. A provider without `mode` never enables an allocation the baseline disables.

**§ 41(5)** When a mode needs a provider and none is named, a unique explicitly declared verified default for the target is used; zero or several eligible defaults are unresolved or ambiguous and diagnosed. Selection never depends on catalog order, registration order or the compiler host.

**§ 41(6)** Resolution order for one Target–Variant plan: target facts; profile family and lineage; family baseline, then target/platform defaults and mandatory constraints; Variant, Target, then Target-specific Variant allocation properties (and a defined CLI override last); provider identity and capability validation; freeze. No override turns Unsupported or Unknown into Supported or invents a provider.

**§ 41(7)** The frozen plan carries at least: TargetProfileIdentity, BaseProfileFamily, AllocationPolicy, AllocationAvailability, AllocationActivation, AllocationContextKind (CompilerManaged, SuppliedArena, None), ProviderIdentity (or Absent), ProviderProvenance, operation-specific ProviderCapabilities, ProviderReclamation, ProviderAlignment, ProviderCapacity, ProviderFailureContract, ProviderContextLimits, ProviderLifetimeContract and ProviderImplementationBinding. Missing evidence stays unresolved and cannot yield a valid allocation plan; a `noalloc` plan is complete with ProviderIdentity Absent.

**§ 41(8)** Provider capabilities are operation-sensitive: individually reclaimable dynamic-array backing (P18) needs a reclaiming provider, and `Arena.Growable` needs a permitted nonrelocating growth strategy; `Arena.FromBuffer` keeps its own caller-supplied backing contract in every mode.

**§ 41(9)** Configuration errors before freeze (unknown family, provider not found, incompatible provider, contradictory keys, ambiguous default) are distinct from program errors against a valid plan. An allocating operation without an available context gets a compile-time diagnostic naming Target, Variant, operation, policy and capability state; provider exhaustion at run time stays an `AllocationError`. Build profiles, `@noAlloc` and `unsafe` never change these facts.

## 41A. Verified system-heap provider `sec.system-heap.v1` (MD-151)

Integrated 2026-11-24 from `rules/corrections/applied/MD-151-system-heap-provider-proposal-20261124-20261124.md` (owner accepted revision 1 as written). It decides the provider catalog and implementation binding left open by § 41(4)–(8). It defines no string descriptor ABI, source allocator API or manifest key.

**§ 41A(1) Identity.** `sec.system-heap.v1` is a compiler-managed provider backed by a qualified target C runtime's `malloc` and `free`. It identifies the provider contract, not one heap instance or one C ABI. Each catalog record binds it to exactly one target identity, ABI, C-runtime identity and version compatibility range and toolchain binding, and the frozen ProviderIdentity includes that record's immutable identity and digest. Records with different runtimes or incompatible bindings never share reclamation authority because the contract spelling agrees.

**§ 41A(2) Eligibility and catalog.** Eligible families are Hosted Linux, Darwin and Windows on amd64 and arm64. Eligibility is not support: a target/runtime combination gets a positive record only after qualification (§ 41A(9)); an absent or unqualified record is Unknown and an explicitly incompatible one Unsupported. No libc is chosen from an OS name. Records are maintained, versioned compiler/toolchain target metadata beside the target registry, as typed records with deterministic canonical serialization for fingerprints; no external catalog file format exists and project TOML cannot declare, create or override a record. A manifest `provider` (`rules/projects/projects.md` § 19(13)) references a catalog identity only.

**§ 41A(3) Defaults.** A qualified Hosted record explicitly nominates this provider as its default; zero defaults stay unresolved and several eligible defaults are an error (§ 41(5)). Family selection, allocation mode and activation precedence stay § 41; selecting a provider never enables a disabled baseline or overrides `noalloc`. RTOS, QNX, BareMetal and `embedded-arena` get no default from MD-151; QNX is not qualified by being POSIX-like, and no static-buffer provider, RTOS binding or capacity key is introduced. Caller-supplied Arena backing keeps its rules, and further provider contracts may be decided separately.

**§ 41A(4) Frozen facts.** Each positive record supplies, with provenance and a qualification-version digest, in addition to target and runtime compatibility:

| Fact | Value or obligation |
| --- | --- |
| Context kind | CompilerManaged |
| Capabilities | owned byte storage, individual reclamation, fresh nonrelocating Arena chunks |
| Reclamation | exact matching-provider free of a live allocation; no bulk-only substitution |
| Alignment | powers of two up to the explicit qualified maximum (§ 41A(5)) |
| Capacity | dynamic and unknown; no guaranteed free bytes or infallible reservation |
| Failure | canonical `AllocationError` (`rules/memory/allocation.md` § 10(7)) |
| Context limits | ordinary permitted execution contexts only (§ 41A(8)) |
| Lifetime | valid until matching release, subject to the owning language lifetime |
| Binding | exact target malloc/free call ABI, size representation, link requirements and adapter revision |

The binding records the target C `size_t`, pointer and address-space facts and the maximum native allocation extent; a source `uint` is never assumed to be `size_t`. The adapter and its allocation tokens belong to the compiler implementation and are not Sec FFI declarations or user-visible types. Allocation and deallocation use the same runtime heap instance and compatible CRT linkage, on Windows including CRT instance and linkage mode; a library cannot free through an unrelated CRT. Cross-module ownership needs a verified common binding or a retained call to the originating deallocator, otherwise the boundary is rejected.

**§ 41A(5) Size and alignment.** A request carries a byte extent and a required alignment from canonical layout facts. The alignment is a nonzero power of two within the record's qualified maximum, which is never inferred from malloc's fundamental alignment and must cover every admitted native layout; an unsupported statically known layout is diagnosed before emission. Stronger alignment is provided by an adapter that requests extra bytes and selects an aligned interior range, retaining the original pointer, provider identity and release metadata in an implementation-owned token that is retainable without a second fallible allocation. Extent conversions, metadata overhead, padding and offsets are checked against the target's size and object-extent limits before malloc, and the range and metadata stay inside that one allocation under the target's pointer and provenance rules; no universal integer-pointer trick is assumed.

**§ 41A(6) Release.** The original base is passed to the matching free exactly once and an interior pointer never. A public reference or view never acquires the token's release authority. The adapter never calls realloc on a live allocation and never relocates published storage. A zero-byte request calls neither malloc nor free, consumes no capacity and has an empty token with no releasable base; this relaxes no static context, ownership or failure rule, and nothing depends on a runtime's `malloc(0)`.

**§ 41A(7) Failure.** An invalid extent or an unrepresentable target or overhead size is `InvalidSize`, an invalid or unsupported runtime alignment `InvalidAlignment`, and a null result for a valid nonzero request `OutOfMemory`. Missing or incompatible provider evidence is a compile-time plan error, never a runtime `Unsupported`. A more specific API-defined failure earlier in the operation takes precedence. Failure has no language-visible side effects: provider storage acquired before the failure is released through the same binding, errno is not consulted, no Sec handler runs, nothing retries indefinitely and no other allocator is selected. A qualified runtime returns null on failure; failure-callback extensions are disabled or separately qualified for the binding.

**§ 41A(8) Execution contexts and lifetime.** The provider gives no bounded-time, nonblocking, lock-free, interrupt-safe or signal-safe guarantee and cannot satisfy ISR, `interruptSafe`, `@noAlloc` or other prohibited-context requirements; an ordinary task may use it only where its allocation and effect rules permit, and no scheduler, thread or event behavior is defined. Concurrent ordinary calls are permitted only by a record whose qualification establishes allocator safety in that environment; this grants no concurrent access to allocated Sec values or Arenas, and unverified concurrency never becomes Supported through the Hosted family. The provider lifetime encloses every live allocation and deallocator call, so a runtime or shared object holding the binding cannot unload while its tokens live; allocation identities stay distinct across address reuse, and the compiler never repairs an invalid lifetime by moving storage to this heap.

**§ 41A(9) Qualification and fingerprints.** A positive record is backed by maintained ABI and runtime evidence, a successful target link check and target-run qualification of alignment, release, zero extent, boundary sizes and controlled failure, on both architectures it claims; emulator execution is identified as such. Cross-compilation uses qualified target evidence, never a probe of the compiler host, and ordinary builds need not rerun qualification. Freeze and fingerprint include catalog identity, revision and digest, target/ABI/runtime compatibility, adapter revision, capabilities and limits, linkage mode and binding digest, and cache/import compatibility compares them. A missing binding fails before executable emission, and no unused positive heap capability is invented to make a plan freeze.

**§ 41A(10) Test bindings and migration.** Production failure tests use an explicit test-toolchain binding with deterministic allocation failure, the same adapter semantics and a distinct fingerprint; it is not a manifest option, an environment override or permission to claim verified provenance. They cover failing allocation N, every earlier successful allocation and every cleanup edge. When implemented, the provisional projection of the Hosted profile string to Supported is removed: qualified catalog defaults keep ordinary Hosted behavior, unqualified configurations become Unknown with a targeted diagnostic explaining the missing qualification, the compiler host's malloc is never selected silently, and allocation-free or `noalloc` plans need no provider. Formerly provisionally accepted allocating programs may be rejected.

## 42. C binding requirement (MD-021)

**§ 42(1)** Whether a plan needs a C binding environment follows from the selected target/profile and its actual C-binding/FFI requirements. A bare-metal or other target without C bindings is never forced to select a hosted C environment; a plan that needs bindings and has no resolvable environment fails closed (`rules/platform/platform_model.md` § 31).

## Debug-information target capabilities

Resolved target information represents debug support with granular semantic
capabilities rather than one `DebugSupported` boolean. The plan distinguishes
supported levels and placements, source mapping, full source value/type and
availability support, logical task inspection when active features require it,
artifact/source correlation, and requested physical stack walking.

A native physical format is a mechanism, not proof that all Sec `Full`
capabilities exist. A statically known unsupported combination is rejected while
validating the `CompilationPlan`; it is never silently downgraded. Canonical
debug semantics are owned by `rules/compiler/debug_information.md`.
