# MD-034 — Freestanding Allocation Policy and Profile Resolution

- **Date:** 2026-11-17
- **Revision:** 1.0
- **Status:** Rulebooks synchronized 2026-11-17 (allocation.md § 22(7)–(9), target_profiles.md § 7); MD-034 decided-implementation-pending. The profile-selection syntax gap and the implementation are remaining items under `platform.target-profiles`
- **Sec version:** 0.1
- **Repository:** `YoePro/sec`, `main` at `78aa3f6a0844993dbe5d1e81e196fe43a2dc7a00`
- **Decision:** MD-034, `allocation-profile-of-freestanding-targets`
- **Owning rules:** `rules/platform/target_profiles.md`, `rules/platform/platform_model.md`, `rules/memory/allocation.md`
- **Implementation/governance:** `internal/layout/allocation_capabilities.go`; platform target/profile resolution; `governance/platform.yaml` and relevant allocation ownership
- **Authority:** Owner agreement on 2026-11-17. This file is a correction proposal for synchronization, **not** an implementation report or claim that the repository has been changed.
- **Revision policy:** Do not renumber existing normative paragraphs in owning rulebooks. Integrate as appended paragraphs or numbered additions consistent with each book's revision policy.

---

## § 1. Scope

**§ 1(1).** MD-034 concerns the relationship between a freestanding execution environment, the selected target profile, an allocation policy, and a positive allocation-context provider. It does not add an allocator implementation, source syntax, runtime library, or new ownership semantics.

**§ 1(2).** Existing authority is `rules/platform/target_profiles.md` §§ 3–7, 29–38, `rules/platform/platform_model.md` §§ 2–3, and `rules/memory/allocation.md` §§ 5–6, 21–22 and 26–27. Their existing requirements on borrowing, validity, cleanup, allocation effects, and deterministic `CompilationPlan` resolution remain in force.

## § 2. Approved normative principle

**§ 2(1).** **Freestanding is not synonymous with `noalloc`.** It describes the target's execution/runtime environment; it is not evidence that dynamic allocation is physically impossible, nor does it automatically provide an allocator.

**§ 2(2).** A freestanding/BareMetal configuration has dynamic heap allocation **disabled by default**, consistent with the BareMetal family baseline in `target_profiles.md` §§ 6–7. Its selected target/profile policy may explicitly enable a compatible allocation capability once the target/platform facts positively support that capability and the provider's obligations are satisfied.

**§ 2(3).** The chosen allocation policy must distinguish at least:

- **No dynamic allocation:** a disabled allocation capability and no active allocation context. This does not assert that the hardware is incapable of allocation.
- **A supplied/preallocated Arena or compatible explicit provider:** allocation operations are permitted only under the resolved capability, context, lifetime, capacity, and applicable interrupt restrictions.

**§ 2(4).** Availability, activation, and context provider are separate facts. `Unknown` availability never proves that an allocator exists; a disabled capability cannot be made available merely because an operation needs allocation. Selecting a profile does not authorize implicit host-runtime or global-heap fallback.

**§ 2(5).** The target/variant's selected profile, target/platform capability facts, and legal explicit overrides must resolve into **one canonical frozen `ResolvedTargetProfile` and `CompilationPlan`**. Sema, lowering, linking, and LSP must consume those same facts. A legacy target registry field spelled `freestanding` must not, by itself, be treated as a fully resolved allocation policy.

**§ 2(6).** Allocation-free code and values remain legal on no-allocation targets. An allocation-capable operation may be accepted without runtime allocation only where the existing canonical optimization/proof rules establish semantics-preserving allocation elimination (`allocation.md` §§ 26–27); no hidden escape promotion is permitted.

**§ 2(7).** Freestanding programs must remain capable of running without a mandatory general Sec runtime or heap allocator.

## § 3. Synchronization and implementation actions

**§ 3(1).** In `rules/platform/target_profiles.md`, append a normative clarification to §§ 6–7: the BareMetal baseline disables heap allocation, but a compatible, positively evidenced provider may be explicitly enabled by a resolved profile. Do not conflate `Availability.Unsupported`, `Availability.Unknown`, and `Activation.Disabled`.

**§ 3(2).** In `rules/memory/allocation.md` § 22, clarify that `hosted`, `embedded-arena`, and `noalloc` are allocation-policy configurations, not aliases for OS or execution-environment identity. Preserve the active-context and allocation-effect requirements in §§ 5–6.

**§ 3(3).** In the platform target/profile resolution path, remove the need to infer allocation capabilities from the raw `freestanding` string. Resolve the effective policy and provider through the target/variant profile model already established by `target_profiles.md` §§ 29–38. **Do not invent manifest keys, target names, or a new source-language allocator syntax in this correction.** If the existing project/target declaration grammar cannot express the selection, record that precise configuration gap before implementing it.

**§ 3(4).** Update `internal/layout/allocation_capabilities.go` and consumers only after the canonical profile resolution provides the necessary facts. The current `freestanding` → unresolved behavior is a conservative interim boundary, not a claim that freestanding can never allocate.

**§ 3(5).** Conformance must demonstrate: (a) freestanding with disabled dynamic allocation, (b) explicitly enabled and positively supported preallocated/supplied context, (c) unknown/unsupported provider rejected without host fallback, (d) illegal profile overrides rejected, (e) allocation-free freestanding source still accepted, and (f) CLI/LSP/IR agreement on the same frozen plan. Include ISR restrictions and profile fingerprints where affected.

## § 4. Completion and open boundary

**§ 4(1).** The **semantic principle is decided**. MD-034 must not be represented as an unresolved choice between “freestanding means noalloc” and “freestanding always has an allocator.” Both interpretations are incorrect.

**§ 4(2).** **Do not mark MD-034 fully closed merely by adding this document.** Closure requires normative synchronization, a resolved target/variant profile mapping (including any necessary configuration grammar), implementation, tests, and corresponding governance/status updates.

**§ 4(3).** This decision does not resolve unrelated platform/environment configurations such as MD-021 C binding selection or MD-075 WebAssembly target identities.
