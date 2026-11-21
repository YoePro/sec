# MD-139 — Target Profile and Allocation Provider Selection

| Field | Value |
| --- | --- |
| Decision | MD-139 — `allocation-provider-selection-syntax` |
| Date | 2026-11-20 |
| Document revision | 1 |
| Sec version | 0.1 |
| Status | **Complete normative proposal; new manifest spelling is pending owner acceptance** |
| Repository reviewed | `YoePro/sec`, `main` at `d680ce820748224e3527a63699aefa4fd34250e0` |
| Primary owners | `rules/platform/target_profiles.md`, `rules/projects/projects.md`, `rules/memory/allocation.md` |
| Related existing decision | MD-034, accepted 2026-11-17 |
| Implementation governance | `platform.target-profiles`, `tooling.project-targets-and-configuration`, P18/P19 |

## § 1. Purpose and boundary

**§ 1(1)** This correction supplies the **missing source configuration, resolution and checked target facts** for a concrete Target–Variant `CompilationPlan`. It neither creates a new allocator API nor changes the ownership, lifetime, borrow, destruction or Arena semantics of Sec.

**§ 1(2)** The three *execution* target-profile families `Hosted`, `RTOS` and `BareMetal` (`target_profiles.md` § 4) are distinct from both the named *build* profile (`[profile.release]`, `[build].profile`; `projects.md` § 22) and the allocation policies informally named `hosted`, `embedded-arena`, `noalloc` (`allocation.md` § 22). A profile-family name alone cannot establish a concrete allocation-provider identity or availability.

**§ 1(3) [EXISTING DECISION, MD-034]** Freestanding does not mean that allocation is impossible. Freestanding and BareMetal begin with dynamic heap allocation disabled; a resolved profile may enable positively proven, compatible allocation facilities. `Unknown` is not `Unsupported`. There is no guessed default heap or host fallback.

**§ 1(4) [EXISTING DECISION]** Build profiles (optimization/debug/strip) do not modify the target-profile family or silently enable allocation. `@noAlloc` is a callable effect guarantee, not a synonym for `noalloc` selection. Allocation effects and context propagation continue to follow `allocation.md` and P19 §§ 51–63.

**§ 1(5)** No existing numbered rulebook paragraph is renumbered. This correction shall be merged by adding uniquely numbered subsections/paragraphs to each owning rulebook.

## § 2. Source syntax — target-profile family

**§ 2(1) [NEW PROPOSAL]** The existing reserved `profile` field of `[variant.<name>]` (`projects.md` § 19(7)) selects a **target profile** rather than a build profile. Sec 0.1 recognizes the canonical family selectors `hosted`, `rtos` and `baremetal`, or a registered derived target-profile identity whose unique base family is known:

```toml
[variant.linux-amd64]
os = "linux"
arch = "amd64"
abi = "gnu"
profile = "hosted"

[variant.board-m4]
os = "baremetal"
arch = "arm32"
profile = "baremetal"
```

**§ 2(2)** A Target may select a compatible target profile through `[target.<name>].profile`, and a Target-specific Variant override may select one through `[target.<name>.variant.<variant-name>].profile`. Their legality, name resolution and applicability are validated against the selected concrete target. An override cannot change the OS, CPU, ABI or hard device capabilities by reclassifying the target.

**§ 2(3)** If the field is omitted, the compiler uses the target registry's **explicit profile-family default for that concrete target**, never a heuristic from its OS/architecture name. Existing registry `Profile` strings such as `freestanding` are transitional *environment labels* and must be migrated or explicitly mapped **per registered target** to canonical profile identities; they are not free-standing allocation policies. A missing or contradictory mapping fails closed with a source- and target-aware diagnostic.

**§ 2(4)** A derived profile identity must resolve to exactly one lineage under `target_profiles.md` § 24. Undefined/ambiguous profiles and changes incompatible with a target's mandatory execution capabilities are configuration errors even if the program allocates nothing.

**§ 2(5)** The existing `[build].profile`, `[profile.<name>]` and `sec build --profile <name>` continue to select **build-option** profiles. They do not alias or override `[variant.<name>].profile` or its Target overrides. The resolved plan retains `BuildProfileIdentity` and `TargetProfileIdentity` as separate facts; a generic field named only `Profile` is insufficient where both can coexist.

## § 3. Source syntax — allocation policy and provider

**§ 3(1) [NEW PROPOSAL]** Allocation-policy selection is an independent, optional TOML table, never a new source-language attribute or an extension of build `[profile.*]`:

```toml
[variant.board-m4.allocation]
mode = "embedded-arena"
provider = "board-primary-arena"

[target.firmware.allocation]
mode = "noalloc"

[target.app.variant.linux-amd64.allocation]
mode = "hosted"
```

The provider spelling is **illustrative**: it is a key looked up in the verified target/platform provider catalog, not a globally built-in provider name. A manifest cannot create a provider merely by naming it.

**§ 3(2)** The canonical `mode` values are precisely:

| Mode | Meaning |
| --- | --- |
| `hosted` | Compiler-managed allocation context is permitted **only if** the selected target/profile supplies and validates a compatible provider and its runtime obligations. |
| `embedded-arena` | Explicit, supplied or preallocated Arena-context policy. Ambient allocation requires a positively established context source; a mere lexical `Arena` variable is not an ambient provider. |
| `noalloc` | Dynamic ambient allocation disabled. Explicit allocation-free representations and separately valid `Arena.FromBuffer` operations are not redefined or prohibited. |

**§ 3(3)** Omission of `mode` is **not** an unknown profile: it requests the resolved *family and platform baseline*. Hosted's `EnabledIfSupported` and RTOS/BareMetal's `Disabled` follow `target_profiles.md` §§ 5–7. When the family baseline enables allocation but no valid provider exists, availability and activation are reported independently; an allocating operation cannot be materialized from this state.

**§ 3(4)** `provider`, when supplied, names a target/platform-declared **canonical provider identity** with positive provenance, availability, compatible context/reclamation behavior and any necessary physical implementation. A project key supplies selection, not implementation or trust. The provider cannot be an unverified source string, an OS-global malloc assumption or a guessed runtime symbol.

**§ 3(5)** A selected `hosted` policy requires a verified compiler-context-capable provider when an ambient context is offered. A selected `embedded-arena` policy activates ambient allocation only where a verified context provider is resolved; explicit caller-propagation still follows P19 § 53 and cannot be invented from the presence of an Arena variable. Without such evidence, requested ambient allocation is unavailable. An explicit `Arena.FromBuffer` may still be legal independently because it uses caller-provided backing, subject to existing Arena rules.

**§ 3(6)** `provider` with `mode = "noalloc"` is rejected as contradictory configuration. If `provider` is present but `mode` is omitted, the family baseline still governs activation; the provider name does not silently enable a disabled capability. An explicit allocation mode that requires ambient provider capability which is unsupported or remains unknown is rejected during profile resolution, not deferred as an invented fallback. An explicit Arena-only operation remains governed by its own source contract.

**§ 3(7)** When `mode` requests a provider but `provider` is omitted, the target may use a **unique, explicitly declared, verified default provider** compatible with the requested context. Zero or multiple eligible defaults without a selected identity are unresolved/ambiguous and must be diagnosed. Provider choice is never based on catalog iteration order, registration order or compiler-host availability.

**§ 3(8)** The same `[allocation]` keys may appear on a reusable Variant, Target, or Target-specific Variant override. Within each scope the keys have exactly the same types and meaning. Unknown keys, duplicate keys, invalid mode names, empty/unknown provider IDs, unusable scope paths and incompatible overrides are configuration diagnostics.

## § 4. Resolution and precedence

**§ 4(1)** Resolve exactly one concrete Target–Variant plan in this order:

1. Resolve the target and its immutable platform/device facts.
2. Resolve the target-profile family and its selected derived lineage (§ 2).
3. Apply the canonical family baseline, then target/platform defaults and mandatory runtime/device constraints (`target_profiles.md` §§ 23–27).
4. Apply the reusable Variant's allocation properties, then the Target's properties, then the Target-specific Variant properties, honoring the ordinary scoped precedence of `projects.md` § 25. An explicit legal CLI override, **if defined**, has last precedence; this correction does not invent a new CLI flag.
5. Resolve the selected provider identity and its target-specific verified capability contract; validate provenance, availability, activation and compatibility.
6. Freeze an immutable `ResolvedTargetProfile` and `CompilationPlan`. A program requirement can reject that plan but cannot silently mutate it (`target_profiles.md` §§ 28–32).

**§ 4(2)** An explicit higher-priority policy replaces a lower-priority policy only when allowed by the target property mutability class. No override can turn `Unsupported` or `Unknown` availability into `Supported`, violate mandatory safety or invent a provider. A disabled policy is an intentional disablement, not proof the platform lacks the facility.

**§ 4(3)** For a multi-Variant build, resolve independent plan facts per Variant. Source identical across Variants can be checked against each plan, with per-Variant diagnostics. An LSP fallback must not assume Hosted or choose the compiler host.

## § 5. Typed `CompilationPlan` allocation facts

**§ 5(1)** The canonical resolved allocation facts must include at least the following **semantic fields** (names are contract labels, not mandatory Go field names):

```text
TargetProfileIdentity
BaseProfileFamily
AllocationPolicy
AllocationAvailability  // Supported | Unsupported | Unknown
AllocationActivation    // Enabled | Disabled
AllocationContextKind   // CompilerManaged | SuppliedArena | None
ProviderIdentity        // verified canonical ID, or Absent
ProviderProvenance      // target/platform and version/identity
ProviderCapabilities    // typed requirements per operation
ProviderReclamation     // individually reclaimable / bulk-only / neither, as proven
ProviderAlignment       // supported allocation alignment constraints
ProviderCapacity        // exact bound or explicitly unbounded/unknown
ProviderFailureContract // existing AllocationError / operation-specific mapping
ProviderContextLimits   // ordinary/task/thread/interrupt and related effects
ProviderLifetimeContract
ProviderImplementationBinding // verified target/runtime implementation
```

**§ 5(2)** `ProviderIdentity` and positive `ProviderCapabilities` do not follow merely from `Profile = "hosted"`. Missing evidence is represented as unresolved; it cannot produce a valid allocation plan. A noalloc plan may be fully resolved while carrying `ProviderIdentity = Absent`, with an explicitly disabled context.

**§ 5(3)** Provider capability facts are **operation-sensitive**. In particular, individually reclaimable dynamic-array backing (P18) cannot be obtained from an exclusively bulk-reclaimed Arena provider merely because both allocate bytes. `Arena.WithCapacity` requires compatible acquisition and release; `Arena.Growable` additionally requires an allowed nonrelocating growth strategy, as already specified by `arena.md` and P19. Explicit `Arena.FromBuffer` uses its own supplied backing contract.

**§ 5(4)** A provider may satisfy multiple modes/operations, but that must be established by explicit capability facts. The compile plan must retain selected provider identity through semantic analysis, IR and lowering; no late backend substitution may weaken reclamation, lifetime, address-stability, capacity, failure or cleanup semantics.

**§ 5(5)** Scope-specific context selection within functions remains governed by P19 § 53 (explicit Arena, propagated context, proven compiler-local context, target-provided context, otherwise rejection). This document does not alter that order, introduce an implicit lexical Arena or invent an allocator handle in every function signature.

**§ 5(6)** `AllocationCapabilities` and `ResolvedScalarPlan.AllocationCapabilities()` are currently incomplete projections based largely on a profile string (`internal/layout/allocation_capabilities.go`). They shall become *projections of the validated frozen plan*, never algorithms that infer availability or provider identity from a profile spelling.

## § 6. Precise diagnostics and failure handling

**§ 6(1)** Configuration errors before plan freeze are distinct from program errors against a valid plan. The former include unknown profile family, unsupported reclassification, provider ID not found, provider incompatible with selected target, conflicting policy/provider keys, unknown positive availability, and ambiguous provider default.

**§ 6(2)** An allocating operation requiring an unavailable context receives an actionable compile-time diagnostic naming Target, Variant, source operation, selected policy, capability/availability state and the remedy: explicit suitable Arena, compatible provider/profile selection, allocation-free form or proven nonallocating alternative, as applicable.

**§ 6(3)** Provider runtime exhaustion under an otherwise valid provider remains a *runtime* failure through the canonical `AllocationError` or owning operation's error type. Missing compiler capability evidence is not a runtime allocation failure and must not be silently converted into one.

**§ 6(4)** The compiler must distinguish a source-level `@noAlloc` violation, disabled allocation policy, physically unsupported allocation, unknown provider and wrong reclamation semantics. `unsafe` cannot manufacture missing hardware/provider capabilities.

## § 7. Integration tasks (no duplicate governance owners)

**§ 7(1)** `rules/projects/projects.md`: append canonical Variant/Target/Target+Variant allocation-table shapes and distinguish target-profile `profile` selection from `[build].profile` build-option selection; retain current key and scope definitions.

**§ 7(2)** `rules/platform/target_profiles.md`: add the exact profile-family/derived-profile selection, canonical overrides, provider provenance/availability, provider capability validation and frozen resolved facts without weakening §§ 3–7, 23–32.

**§ 7(3)** `rules/memory/allocation.md`: clarify allocation policy as independent from target-profile family, and that the provider selection is checked before materialization. Preserve existing `Arena.FromBuffer`, implicit context, failure, escape and destruction rules.

**§ 7(4)** `internal/platform/target/` and project manifest resolver must produce target profile and provider catalog facts. `internal/layout/allocation_capabilities.go` and the canonical `CompilationPlan` must consume these facts instead of inferring from `profile` alone. Compiler/LSP and P18/P19 share one frozen result; no independent backend allocator chooser.

**§ 7(5)** `missing-decisions.yaml` MD-139 changes from `open` to `decided-implementation-pending` **only after** the owner accepts the new manifest schema and this specification is merged into the owning rulebooks; preserve original MD ID and track concrete implementation closure in existing `platform.target-profiles`, project configuration and P18/P19 governance items. MD-034 remains independent and already decided.

## § 8. Conformance matrix

| Case | Expected outcome |
| --- | --- |
| Hosted target, supported verified default provider, mode omitted | Baseline may enable valid compiler-managed allocation. |
| Hosted target, provider unknown/unsupported, mode omitted | No implicit provider invented; allocating program requires capability and fails. |
| BareMetal/freestanding, no allocation override | Allocation disabled, `noalloc` behavior; no mandatory general runtime. |
| BareMetal with explicit `embedded-arena`, verified provider | Allowed where device/profile and provider contract permit. |
| BareMetal with explicit `embedded-arena`, provider unknown | Configuration error; no host fallback. |
| Explicit `noalloc` plus provider name | Configuration error. |
| Provider named under wrong target/ABI | Configuration error with target/variant context. |
| Two verified defaults, none selected | Ambiguity error; no selection by order. |
| `Arena.FromBuffer` with valid caller-owned backing in noalloc | Preserves existing explicit-buffer semantics; not disabled merely by no ambient heap. |
| Explicit Arena-selected allocation with verified backing | Uses P19 § 53 explicit context path. |
| P18 individually reclaimable `T[]`, bulk-only provider | Reject until independent reclaiming semantics are proven; no fake free. |
| `Arena.Growable`, fixed-only non-growing provider | Reject unsupported growth; do not relocate live storage. |
| Valid provider, runtime capacity exhausted | Owning operation's defined checked error, never invented compile-time support failure. |
| `@noAlloc` function on allocating call in hosted profile | Reject effect violation even though allocation is available. |
| Same Target with different Variants | Different frozen plans and cache identities; correct per-Variant diagnostics. |
| Build `[profile.release]` with target-profile `profile = "baremetal"` | Independent identities; `release` cannot enable heap. |
| LSP and CLI on identical plan | Identical profile/provider capability and diagnostic decisions. |

## § 9. Decision provenance and acceptance

**§ 9(1)** **Already decided:** the three target-profile families; family capabilities and precedence; `noalloc` and Arena semantics; MD-034 freestanding default; provider-positive-proof/no-host-fallback; P19 context-selection order; target-plan immutability; independent build profiles.

**§ 9(2)** **Genuinely new, proposed here:** use of `variant.profile` (and Target overrides) for target-profile identity; optional scoped `[allocation]` tables and the exact `mode`/`provider` spelling; provider ambiguity/selection diagnostics; resolved provider-capability contract. These are implementable design choices but **are not claimed to have been previously approved**.

**§ 9(3)** The specification is ready for owner approval and normative integration. No GitHub files were changed by writing this correction. No implementation or regression tests were run. Once the owner accepts the new manifest spelling, MD-139 need not generate additional unrelated language-design questions; only integration and verified target-provider data remain.

## § 10. Repository sources inspected

- `missing-decisions.yaml` MD-139 and MD-034.
- `rules/projects/projects.md` §§ 6, 17, 19, 21–22, 24–25.
- `rules/platform/target_profiles.md` §§ 3–7, 23–32.
- `rules/memory/allocation.md` §§ 1–10, 22, 26–28.
- `rules/memory/arena.md` §§ 10–18, 29–32.
- `rules/mlir/packages/sec-mlir-dialect_package19.md` §§ 53–63.
- `internal/layout/allocation_capabilities.go`, `internal/platform/target/target.go`.
