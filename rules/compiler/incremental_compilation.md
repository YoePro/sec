# Incremental Compilation
- **Status:** Normative
- **Created:** 2026-09-27
- **Last updated:** 2026-09-27
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/compiler/incremental_compilation.md`
- **Replaces:** Planned `incremental_compilation.md` closure entry; no earlier canonical rulebook
- **Repository baseline reviewed:** `main-reviewed-2026-09-27`
- **Implementation governance:** `governance/compiler.yaml` (`compiler.incremental-compilation-v1`)
- **Cross-rulebook correction:** `rules/corrections/applied/incremental-compilation-cross-rulebook-correction-20260928.md`
- **Related rulebooks:** `rules/compiler/compiler_pipeline.md`, `rules/compiler/compiler_analysis.md`, `rules/compiler/semantic_ir.md`, `rules/compiler/monomorphization.md`, `rules/compiler/linking.md`, `rules/compiler/debug_information.md`, `rules/compiler/compiler_testing.md`, `rules/projects/modules.md`, `rules/tooling/lsp.md`, `rules/tooling/diagnostics.md`, `rules/platform/abi.md`, `rules/platform/target_profiles.md`

---
## § 1. Purpose and semantic authority

**Governance tags:** `compiler.incremental-compilation-v1`

§ 1(1) Incremental compilation is semantics-preserving compiler infrastructure for avoiding unnecessary compiler work. It is not a source-language mode and it introduces no alternate meaning for a Sec program.

§ 1(2) For identical canonical compilation inputs, a valid incremental or warm-cache compilation and a cold compilation must produce equivalent canonical Sec semantic results. Cache history, editor history, worker scheduling, and prior builds are not Sec program inputs.

§ 1(3) This rulebook owns reuse, invalidation, cache-validity, snapshot-consistency, history-independence, and incremental artifact rules. The rulebooks that define modules, types, generics, analyses, Semantic IR, ABI, debug information, linking, and other compiler facts remain authoritative for the meaning of those facts.

---
## § 2. Cold recomputation is the correctness fallback

**Governance tags:** `compiler.incremental-compilation-v1`

§ 2(1) Reuse is permitted only when compatibility is established. Missing, stale, incompatible, corrupt, undecodable, unsupported, or otherwise unusable cached state is unavailable reuse and is recomputed from canonical inputs when compilation can otherwise proceed.

§ 2(2) A cache miss or cache rejection is not a Sec source error. Clearing compiler-generated cache state may increase work, but it must not change source validity or program semantics.

§ 2(3) An implementation may conservatively abandon incremental work and continue through a cold path whenever reuse cannot be proven safe.

---
## § 3. Cache state is derived state, never canonical truth

**Governance tags:** `compiler.incremental-compilation-v1`

§ 3(1) Cached syntax, semantic facts, analysis summaries, generic specializations, Semantic IR, lowered representations, backend artifacts, debug data, and link state are derived results.

§ 3(2) The presence of a cached result does not prove that result is current. Current canonical source, project, compiler-contract, target, toolchain, and dependency state validate reuse.

§ 3(3) A higher physical artifact must never establish the validity of a lower semantic layer merely because that higher artifact exists.

---
## § 4. Logical compilation snapshots

**Governance tags:** `compiler.incremental-compilation-v1`

§ 4(1) Every published compilation result belongs to one logically consistent source/project/configuration snapshot and the applicable `CompilationPlan`.

§ 4(2) The compiler may snapshot inputs, restart work, supersede obsolete requests, or retain compatible old results. It must not combine mutually incompatible facts from different snapshots into one positive build result.

§ 4(3) Interactive tooling may temporarily retain explicitly stale information while newer analysis is pending, but stale information is not current positive semantic proof.

---
## § 5. Canonical identity is separate from cache validity

**Governance tags:** `compiler.incremental-compilation-v1`

§ 5(1) Canonical identities such as module, declaration, type, callable, `Place`, and generic-specialization identity answer which semantic entity a fact belongs to. Cache keys and fingerprints answer whether a previously derived result may still be reused.

§ 5(2) A semantic entity may retain its canonical identity while one or more previously derived artifacts become invalid.

§ 5(3) Cache and fingerprint identities are compiler infrastructure and do not replace ordinary Sec identities in diagnostics, LSP, debugger presentation, Semantic IR, or ABI/link identity.

---
## § 6. Layered reuse

**Governance tags:** `compiler.incremental-compilation-v1`

§ 6(1) Incremental implementations distinguish conceptually between layers whose validity dependencies differ. Relevant layers include source/syntax structure, declaration/name/type resolution, semantic analyses, generic template and specialization semantics, Semantic IR, `CompilationPlan`-specific realization/lowering, backend/object generation, debug artifacts, and linking/final-artifact state.

§ 6(2) The physical number and layout of caches are implementation-defined. Several conceptual layers may share storage, and one conceptual layer may use several stores.

§ 6(3) Combining storage layers must not erase the distinct dependency contracts required for correct invalidation.

---
## § 7. Fail-closed reuse

**Governance tags:** `compiler.incremental-compilation-v1`

§ 7(1) A cached result is reusable only when every dependency required by its cache contract is known compatible with the active logical snapshot and `CompilationPlan`.

§ 7(2) Unknown, incomplete, ambiguous, or insufficient compatibility causes invalidation or recomputation rather than speculative trust.

§ 7(3) Over-invalidation is a performance or implementation-quality defect. Under-invalidation that permits stale results is a compiler-correctness defect.

---
## § 8. Semantic and dependency-based invalidation

**Governance tags:** `compiler.incremental-compilation-v1`

§ 8(1) Incremental invalidation follows the facts and dependency domains that can affect the cached result. Raw source-byte change, file change, timestamp change, and semantic change are not identical concepts.

§ 8(2) Filesystem timestamps and similar metadata may be implementation fast paths, but they are not by themselves canonical proof that semantic dependencies are unchanged.

§ 8(3) A source/provenance change may invalidate diagnostics, LSP, documentation, or debug artifacts while leaving some semantic results reusable.

---
## § 9. Layer-, version-, and plan-sensitive compatibility

**Governance tags:** `compiler.incremental-compilation-v1`

§ 9(1) Compiler semantic-rule versions, serialization schemas, analysis contracts, `CompilationPlan` facts, optimization policy, debug policy, backend behavior, linker/toolchain state, and verifier contracts participate only in cache layers they can materially affect.

§ 9(2) A compiler may invalidate more broadly than necessary. The canonical model nevertheless permits independent compatibility domains so that a backend-only change need not redefine earlier semantic identity.

§ 9(3) Target-, ABI-, layout-, or plan-sensitive artifacts must never cross incompatible plans.

---
## § 10. Storage-independent validity model

**Governance tags:** `compiler.incremental-compilation-v1`

§ 10(1) The same validity rules apply to within-request memoization, in-memory daemon or LSP reuse, cross-invocation persistent caches, and distributed or shared build caches where those capabilities exist.

§ 10(2) Directory layout, database choice, serialization technology, persistence duration, and eviction policy are implementation-defined.

§ 10(3) Sec 0.1 introduces no source-visible cache, fingerprint, dependency, or incremental-compilation API.

---
## § 11. Dependency identities represent compiler facts

**Governance tags:** `compiler.incremental-compilation-v1`

§ 11(1) Incremental dependency tracking operates on canonical compiler facts, query results, or conservative invalidation domains appropriate to the reusable result, rather than being defined solely by files.

§ 11(2) A file or module may be the chosen safe granularity for an implementation, but file membership is not the canonical model when a finer semantic dependency is required for correctness.

§ 11(3) An implementation may begin conservatively and refine granularity without changing Sec semantics.

---
## § 12. Direct dependencies establish transitive validity

**Governance tags:** `compiler.incremental-compilation-v1`

§ 12(1) A reusable result records or otherwise commits to every direct dependency needed to justify that result.

§ 12(2) Transitive validity may be represented recursively through dependency fingerprints or generations; every entry need not flatten the entire transitive dependency closure.

§ 12(3) A transitive dependency must not become invisible merely because only direct edges are physically stored.

---
## § 13. Set-valued and negative dependencies

**Governance tags:** `compiler.incremental-compilation-v1`

§ 13(1) Queries whose result depends on a candidate, declaration, member, overload, import, conformance, or other open set must be invalidated when that set changes, including when a member is added or removed.

§ 13(2) An absence result such as `no matching declaration` or `no matching implementation` is dependency-bearing state. It must not remain valid merely because none of the entities that existed during the earlier query changed.

§ 13(3) Set-result identity or an equivalent sound invalidation domain therefore participates in reuse.

---
## § 14. Observed control inputs are dependencies

**Governance tags:** `compiler.incremental-compilation-v1`

§ 14(1) If a cached computation's result or executed path depends on a condition, plan fact, target capability, compile-time evaluation result, feature/build option, or other control input, that input participates in dependency validity.

§ 14(2) Dynamic dependency discovery is permitted. The implementation need only record facts relevant to the executed or result-producing path, provided the decisions selecting that path are themselves tracked.

§ 14(3) Undeclared ambient compiler-host state must not influence a cacheable canonical result.

---
## § 15. Domain-specific deterministic fingerprints

**Governance tags:** `compiler.incremental-compilation-v1`

§ 15(1) A fingerprint represents canonical state relevant to a particular reusable result. Semantic, provenance, layout, realization, debug, link, and other domains may use distinct fingerprints.

§ 15(2) Fingerprints exclude incidental implementation nondeterminism such as worker order, map iteration order, temporary node addresses, cache population order, and unrelated temporary paths.

§ 15(3) There is no requirement for one universal compiler-wide fingerprint.

---
## § 16. Fingerprint contract versioning

**Governance tags:** `compiler.incremental-compilation-v1`

§ 16(1) Persistent or cross-request fingerprints are interpreted only under a compatible fingerprint/canonicalization schema or domain version.

§ 16(2) Changing the meaning, canonicalization, or dependency contract of a fingerprint invalidates entries that cannot be proven compatible.

§ 16(3) The concrete hash algorithm, binary encoding, and storage representation are implementation-defined.

---
## § 17. Fingerprint integrity

**Governance tags:** `compiler.incremental-compilation-v1`

§ 17(1) The implementation's fingerprint/equality mechanism must be sufficiently robust for the correctness role in which it is used.

§ 17(2) Hash collision, truncation, cache corruption, index corruption, or serialization damage must not silently validate an incompatible artifact.

§ 17(3) Implementations may use strong fingerprints, secondary equality checks, checksums, verifier passes, or other mechanisms. This rulebook mandates the invariant, not a specific algorithm.

---
## § 18. Propagation may stop at unchanged canonical output

**Governance tags:** `compiler.incremental-compilation-v1`

§ 18(1) When an input dependency changes, affected nodes are invalidated or recomputed.

§ 18(2) If recomputation establishes that the canonical output for a particular dependency domain is unchanged, invalidation need not propagate through dependents of that domain.

§ 18(3) Other domains, including provenance or debug mapping, may still propagate independently. This optimization is legal only when no changed fact relevant to a dependent is concealed.

---
## § 19. Incremental dependency graph scope

**Governance tags:** `compiler.incremental-compilation-v1`

§ 19(1) The incremental dependency graph is broader than the module import/build graph. It may include semantic analyses, compile-time evaluation, generic specialization, target/plan facts, source provenance, lowering, ABI, debug information, linking, and final artifacts.

§ 19(2) The graph may contain cycles where owning analyses permit cyclic or fixed-point relationships.

§ 19(3) A change affecting a cyclic region must not allow stale fixed-point facts to escape as current proof.

---
## § 20. Reuse and invalidation are inspectable

**Governance tags:** `compiler.incremental-compilation-v1`

§ 20(1) Compiler inspection and test infrastructure must expose enough information to determine, for verification and debugging purposes, whether a result was reused or recomputed, which dependency/fingerprint domain justified reuse, which changed dependency caused invalidation, and which logical snapshot/plan owns the result.

§ 20(2) The exact CLI, file, protocol, or internal inspection format is tooling/implementation policy.

§ 20(3) Inspection metadata is not Sec program semantics.

---
## § 21. Snapshot-bound query results

**Governance tags:** `compiler.incremental-compilation-v1`

§ 21(1) Compiler computations that participate in incremental reuse are conceptually evaluated for a defined logical snapshot and the relevant `CompilationPlan`.

§ 21(2) Stored results may coexist across snapshots. Physical recency and completion order do not determine freshness; compatibility with the requesting snapshot does.

§ 21(3) A result must not become current merely because it completed last.

---
## § 22. Invalidation and eviction are separate

**Governance tags:** `compiler.incremental-compilation-v1`

§ 22(1) A stale result may remain physically stored for debugging, another compatible snapshot, or later eviction. Eager deletion is not required.

§ 22(2) The correctness requirement is that stale state is not returned or published as current without a valid compatibility proof.

§ 22(3) Implementations may use eager invalidation, lazy validation, generations, versioned stores, or hybrids.

---
## § 23. Atomic reusable-result publication

**Governance tags:** `compiler.incremental-compilation-v1`

§ 23(1) A result becomes reusable only after the computation has produced all metadata required by its cache contract, including dependency/compatibility data and required verification/provenance.

§ 23(2) Partially produced or partially recorded results may not masquerade as complete reusable facts.

§ 23(3) The implementation may satisfy this using immutable publication, generation commits, transactional storage, or another equivalent mechanism.

---
## § 24. Reusable negative semantic results

**Governance tags:** `compiler.incremental-compilation-v1`

§ 24(1) Canonical negative outcomes such as failed lookup, type or ownership errors, unsupported-target results, and other semantic failures may be cached where useful.

§ 24(2) Their reuse is subject to the same dependency and snapshot rules as positive results, including set-valued and negative dependencies.

§ 24(3) Compiler panic, internal invariant failure, incomplete computation, resource failure, or cancellation is not automatically a semantic negative result and must not be cached as though the source program itself were invalid.

---
## § 25. Cancellation and supersession

**Governance tags:** `compiler.incremental-compilation-v1`

§ 25(1) Cancelled, superseded, or otherwise incomplete work must not publish incomplete compiler facts as complete current results.

§ 25(2) An independently completed subcomputation may still be retained when it already satisfies its own complete dependency and verification contract.

§ 25(3) Cancellation of one consumer does not inherently invalidate a compatible shared computation still required by another consumer.

---
## § 26. Incremental fixed-point correctness

**Governance tags:** `compiler.incremental-compilation-v1`

§ 26(1) Cyclic dependency regions and fixed-point analyses remain governed by their owning analysis contracts. Previous results may be used as optimization seeds where sound, but they are not thereby current proof.

§ 26(2) An incremental fixed-point algorithm must correctly handle both newly added and removed facts. It must reset or recompute enough state to reach the canonical fixed point for the current snapshot.

§ 26(3) Externally reusable summaries and dependent diagnostics are published only after the required current convergence/verification boundary.

---
## § 27. Stale facts may not escape cyclic recomputation

**Governance tags:** `compiler.incremental-compilation-v1`

§ 27(1) When a change affects an SCC or fixed-point region, the implementation may recompute the entire region or use a sound finer-grained algorithm.

§ 27(2) Until the required current result is established, stale or intermediate region facts may not escape as positive proof to current dependents.

§ 27(3) The exact SCC representation, worklist, lattice algorithm, and convergence strategy are implementation-defined.

---
## § 28. Concurrent compatible work sharing

**Governance tags:** `compiler.incremental-compilation-v1`

§ 28(1) Concurrent builds, LSP requests, compiler workers, or plans may share immutable reusable results and deduplicate equivalent in-flight computations when their relevant query, snapshot, dependency, and plan contracts are compatible.

§ 28(2) Different request or snapshot identifiers do not by themselves prohibit reuse when dependency compatibility is proven.

§ 28(3) Apparent query-name equality does not permit sharing across incompatible dependencies.

---
## § 29. Completion order and speculation do not define current state

**Governance tags:** `compiler.incremental-compilation-v1`

§ 29(1) An older computation that finishes after a newer computation may not overwrite newer current state merely because it completed later.

§ 29(2) Speculative computation is permitted but must satisfy the same dependency, verification, and publication rules before reuse.

§ 29(3) Obsolete work may be cancelled, discarded, completed in isolation, or mined for independently compatible subresults. Generation/version protection must prevent stale late publication.

---
## § 30. Coherent final build snapshot

**Governance tags:** `compiler.incremental-compilation-v1`

§ 30(1) A final build may combine results produced at different physical times only when each reused result is compatible with the requested logical snapshot and `CompilationPlan`.

§ 30(2) The final published diagnostics and artifact set represent one coherent logical compilation state rather than an accidental mixture of incompatible generations.

§ 30(3) Internal query publication and final user-visible build publication are distinct correctness boundaries.

---
## § 31. Serialized artifacts cross a validation boundary

**Governance tags:** `compiler.incremental-compilation-v1`

§ 31(1) A persistent or shared cache artifact is not reusable merely because it can be located and decoded.

§ 31(2) Loading validates, as applicable, artifact-kind identity, schema compatibility, payload/metadata integrity, dependency/fingerprint compatibility, `CompilationPlan` compatibility, producer/toolchain compatibility, and required verifier compatibility.

§ 31(3) Failure normally yields cache rejection and recomputation rather than a Sec source-semantic error.

---
## § 32. Serialization and semantic compatibility are independent

**Governance tags:** `compiler.incremental-compilation-v1`

§ 32(1) A persistent artifact family has enough format identity to determine whether its representation can be decoded correctly.

§ 32(2) Semantic-rule compatibility, analysis/verifier compatibility, backend compatibility, and toolchain compatibility are independent dimensions.

§ 32(3) A representation may be decodable but semantically stale. Conversely, a semantic result may remain valid while an old physical encoding can no longer be read directly.

---
## § 33. Crash-safe persistent publication

**Governance tags:** `compiler.incremental-compilation-v1`

§ 33(1) A persistent cache entry becomes visible as reusable only as a complete committed artifact.

§ 33(2) Payload, artifact identity, and required dependency/compatibility metadata must be bound so that partial writes, crashes, mixed concurrent publications, or mismatched metadata/payload cannot silently form a valid entry.

§ 33(3) The concrete transaction, filesystem, database, checksum, or hashing mechanism is implementation-defined.

---
## § 34. Concurrent persistent-cache access

**Governance tags:** `compiler.incremental-compilation-v1`

§ 34(1) Concurrent writers may deduplicate or race to publish equivalent entries. Concurrent readers may observe an old valid entry, a new valid entry, or a miss according to storage policy.

§ 34(2) A reader must not observe a partial or mixed entry as valid.

§ 34(3) Eviction and cache garbage collection are distinct from semantic invalidation and may remove reusable entries without changing compilation correctness. Active reads and publications must remain safe under cleanup.

---
## § 35. Shared and remote caches

**Governance tags:** `compiler.incremental-compilation-v1`

§ 35(1) Cross-process, cross-machine, build-farm, or remote caches may be used where supported.

§ 35(2) A remote/shared hit has no weaker dependency, plan, schema, integrity, or verifier requirements than a local hit.

§ 35(3) Artifacts originating outside the configured trust boundary require validation or authentication sufficient for that deployment model before they may become trusted compiler facts. This rulebook does not mandate a transport or cryptographic system.

---
## § 36. Physical artifact cache identity

**Governance tags:** `compiler.incremental-compilation-v1`

§ 36(1) Backend, object, debug, and link cache identities include every fact capable of changing their required physical artifact contract.

§ 36(2) Relevant facts include, as applicable, `CompilationPlan`, target, ABI, layout, CPU/features, optimization/codegen policy, backend/toolchain identity, runtime policy, concrete generic realization, debug level/placement/format policy, `LinkPlan`, native dependencies, LTO, output kind, and reproducibility policy.

§ 36(3) Incremental compilation consumes the canonical identities supplied by the owning ABI, debug, monomorphization, backend, and linking rulebooks rather than redefining them.

---
## § 37. Semantic and physical layers invalidate independently

**Governance tags:** `compiler.incremental-compilation-v1`

§ 37(1) A change confined to debug, optimization, backend, linker, or another physical realization policy need not invalidate target-independent semantic results when their semantic contracts remain compatible.

§ 37(2) A semantic change invalidates every dependent physical artifact.

§ 37(3) A physical artifact from an incompatible semantic or plan state must not survive merely because its bytes remain available.

---
## § 38. Verifier-status compatibility

**Governance tags:** `compiler.incremental-compilation-v1`

§ 38(1) A serialized artifact treated as previously verified may retain that status only when the applicable verifier/invariant contract is compatible.

§ 38(2) If the verifier contract changed, the artifact is reverified or invalidated/recomputed as required by its layer.

§ 38(3) Successful decoding does not establish compliance with current Semantic IR, MLIR, backend, debug, link, or final-artifact invariants.

---
## § 39. Compiler cache is optional acceleration state

**Governance tags:** `compiler.incremental-compilation-v1`

§ 39(1) Evicting, clearing, or losing compiler-generated cache state may increase work but must not make an otherwise reproducible project compilation semantically impossible.

§ 39(2) Declared prebuilt libraries, SDK components, native dependencies, firmware blobs, or other project/toolchain inputs are not compiler-cache entries merely because they may be stored locally.

§ 39(3) Cache corruption or poisoning is isolated as cache/infrastructure failure where possible and does not redefine source validity.

---
## § 40. Persistent-cache provenance and decisions are inspectable

**Governance tags:** `compiler.incremental-compilation-v1`

§ 40(1) Compiler inspection/test infrastructure must expose enough persistent-cache metadata to verify and diagnose reuse, including artifact/cache layer, hit/miss/recompute outcome, local/shared origin where relevant, artifact and dependency identities, schema/domain compatibility, producer/toolchain compatibility, verification/revalidation outcome, and rejection/invalidation reason.

§ 40(2) Storage telemetry such as age, hit count, LRU state, or latency may guide eviction and performance policy but is not semantic validity evidence.

§ 40(3) An artifact does not become semantically stale merely because it is old.

---
## § 41. Module boundaries expose dependency-specific surfaces

**Governance tags:** `compiler.incremental-compilation-v1`

§ 41(1) Incremental compilation consumes the canonical `ModuleIdentity`, `ModuleInstance`, and `ModuleSurface` model defined by `rules/projects/modules.md`.

§ 41(2) A module's externally relevant state is the canonical semantic/compiler information required by its consumers rather than the raw bytes of all files in the module.

§ 41(3) Implementations may represent that state with one conservative surface fingerprint or multiple domain-specific surfaces.

---
## § 42. Stable importer-visible surfaces stop semantic propagation

**Governance tags:** `compiler.incremental-compilation-v1`

§ 42(1) Implementation-only changes are first recomputed within their owning dependency domains.

§ 42(2) If every importer-visible semantic fact consumed downstream remains canonically unchanged, semantic invalidation need not propagate beyond the module boundary.

§ 42(3) A private or internal implementation change that modifies an exported effect, conformance, generic, layout, or other downstream-required fact propagates the changed fact normally.

---
## § 43. Visibility bounds direct consumers but does not prove isolation

**Governance tags:** `compiler.incremental-compilation-v1`

§ 43(1) Source-file-private, module-internal, and public declarations have different possible direct consumer scopes according to the module/visibility rules.

§ 43(2) Incremental implementations may use those scopes to bound dependency discovery and invalidation.

§ 43(3) Visibility alone does not prove that downstream state is unaffected because a private implementation change may alter an importer-visible summary or a physical realization dependency.

---
## § 44. Module sets are invalidation-bearing state

**Governance tags:** `compiler.incremental-compilation-v1`

§ 44(1) Addition or removal of source files, declarations, overloads, public members, conformances, imports, or other set-valued module facts invalidates consumers of the affected set even when no previously existing declaration changed.

§ 44(2) Module discovery and public-surface tracking therefore obey the set-valued and negative-dependency rules of § 13.

§ 44(3) Filesystem discovery must account for file addition/removal and plan-dependent applicability, not only changes to already-known files.

---
## § 45. Module-facing dependency domains

**Governance tags:** `compiler.incremental-compilation-v1`

§ 45(1) A module/import relationship may expose several independently invalidatable dependency domains, including public semantic/API facts, compiler-derived analysis summaries, generic template semantics, layout/ABI facts, optimization/realization facts, and source/debug provenance.

§ 45(2) A change invalidates only the downstream domains that consume the changed fact where the implementation has sufficient precision.

§ 45(3) A conservative combined surface is permitted; under-invalidation is not.

---
## § 46. Generic templates carry downstream semantic dependencies

**Governance tags:** `compiler.incremental-compilation-v1`

§ 46(1) For exported generic declarations instantiated from template semantics, an unchanged public signature or constraint identity does not by itself make prior specializations reusable after a relevant template-body semantic change.

§ 46(2) Concrete instantiations depend on canonical template semantic fingerprints and the specialization dependencies defined by `rules/compiler/monomorphization.md`.

§ 46(3) Where dependency precision permits, only importers and specializations that consume the affected generic facts are invalidated.

---
## § 47. Ordinary bodies and cross-module physical optimization are separate

**Governance tags:** `compiler.incremental-compilation-v1`

§ 47(1) Changing the implementation body of a non-generic exported callable does not inherently invalidate importer name/type analysis when its importer-facing semantic contract remains unchanged.

§ 47(2) The owning implementation artifact is nevertheless stale.

§ 47(3) Inlining, LTO, constant propagation, or another cross-module optimization may create additional physical dependencies requiring importer/backend artifact regeneration. Those are realization dependencies, not new source-language API semantics.

---
## § 48. Module identity and surface compatibility are distinct

**Governance tags:** `compiler.incremental-compilation-v1`

§ 48(1) Import resolution depends on canonical `ModuleIdentity`, not merely import spelling, short module name, source path spelling, or local alias.

§ 48(2) Reuse requires compatible resolved module identity and compatible consumed surfaces.

§ 48(3) A similarly shaped module reached through an incompatible resolution must not be substituted merely because its exported declarations resemble the previous module.

---
## § 49. Production, test, and plan-selected module contexts

**Governance tags:** `compiler.incremental-compilation-v1`

§ 49(1) Production module instances, `sec test` views containing `*_test.sec`, and target/variant-selected module instances must not have incompatible member or surface state conflated in cache.

§ 49(2) Compatible production or target-independent subresults may be shared when reuse is proven safe.

§ 49(3) Test-only declarations and plan-selected source/member sets participate in the identities and dependencies of the contexts in which they exist.

---
## § 50. Validated and usage-driven module-surface publication

**Governance tags:** `compiler.incremental-compilation-v1`

§ 50(1) A changed module surface is published for downstream reuse only after applicable module identity, visibility/access, declaration-set, signature, and other canonical surface invariants have been validated.

§ 50(2) Downstream propagation is dependency- and usage-driven rather than inherently `rebuild every transitive importer`.

§ 50(3) Implementations may conservatively invalidate broader importer sets when finer dependency evidence is unavailable, but an unchanged importer-visible surface is a valid propagation barrier.

---
## § 51. Source snapshots and coordinates

**Governance tags:** `compiler.incremental-compilation-v1`

§ 51(1) Source content, file or editor-buffer generation, and source-coordinate mappings belong to the logical compilation snapshot.

§ 51(2) Line, column, byte offset, token range, and source range are meaningful only relative to the source snapshot that owns them.

§ 51(3) Reusing a semantic result does not automatically make its old source coordinates current.

---
## § 52. Syntax representation identity is not semantic identity

**Governance tags:** `compiler.incremental-compilation-v1`

§ 52(1) Tokens, CST/AST nodes, parser object addresses, and other syntax representation identities may be incrementally reused where valid but do not define canonical symbol, type, callable, `Place`, module, or other semantic identity.

§ 52(2) A reparse may produce new syntax objects for the same semantic entity, and syntax-object reuse does not prove that semantic identity or facts are unchanged.

§ 52(3) Canonical identity preservation follows the owning semantic identity rules.

---
## § 53. Incremental lexing/parsing requires compatible context

**Governance tags:** `compiler.incremental-compilation-v1`

§ 53(1) Unchanged source bytes or subtree hashes alone do not prove syntax-node reuse is valid.

§ 53(2) Lexical state, delimiter state, surrounding parser context, recovery state, declaration membership, and other inputs capable of changing the parse participate in syntax-layer validity.

§ 53(3) The exact incremental parser algorithm and invalidation window are implementation-defined and may conservatively reparse more source.

---
## § 54. Semantically irrelevant edits may still change provenance

**Governance tags:** `compiler.incremental-compilation-v1`

§ 54(1) Whitespace, comments, documentation, or other source material may be excluded from a semantic fingerprint only in dependency domains where their canonical owners declare them irrelevant.

§ 54(2) The same edit may still invalidate source ranges, documentation/hover data, formatter state, LSP locations, debug provenance, or source-content correlation.

§ 54(3) There is no compiler-wide rule that all trivia or comments are irrelevant to every consumer.

---
## § 55. Semantic identity remapping is fail-closed

**Governance tags:** `compiler.incremental-compilation-v1`

§ 55(1) Incremental compilation may match declarations and entities across source revisions to retain reusable facts, but canonical identity is not established from source proximity, AST pointer reuse, spelling similarity, or another heuristic alone.

§ 55(2) Renames, copy/paste, reorderings, scope changes, file moves, overload changes, and similar edits follow canonical module/symbol/type identity rules.

§ 55(3) When identity continuity cannot be proven sufficiently for reuse, identity-dependent state is recomputed rather than guessed.

---
## § 56. Canonical source identity

**Governance tags:** `compiler.incremental-compilation-v1`

§ 56(1) Incremental compilation reuses the canonical project/package/module/source identity model consumed by diagnostics, LSP, debug information, and the compiler pipeline. It does not invent an independent cache-only source identity.

§ 56(2) An absolute compiler-host path is not inherently canonical source identity.

§ 56(3) File changes or moves nevertheless participate in semantic dependencies where source-file identity affects language rules, including source-file-private declarations.

---
## § 57. Diagnostics and tooling must use current-snapshot occurrences

**Governance tags:** `compiler.incremental-compilation-v1`

§ 57(1) Cached semantic diagnostic or tooling facts may be reused independently of their old presentation positions when their semantic dependencies remain valid.

§ 57(2) Primary and related diagnostic spans, source excerpts, definition/reference locations, hover/navigation positions, and similar occurrences must correspond to the current source snapshot before being published as current.

§ 57(3) If provenance cannot be safely rebound or remapped, the occurrence is recomputed.

---
## § 58. Interactive tooling publication is versioned

**Governance tags:** `compiler.incremental-compilation-v1`

§ 58(1) LSP and editor results are associated with the source/buffer generation and semantic snapshot from which they were derived.

§ 58(2) An older request completing after a newer request must not overwrite or masquerade as the newer current result.

§ 58(3) Unsaved-buffer and on-disk source states must not be implicitly mixed as one current snapshot without compatibility proof. Explicit stale/pending presentation remains governed by the tooling contract.

---
## § 59. Semantic and provenance/debug invalidation are independent

**Governance tags:** `compiler.incremental-compilation-v1`

§ 59(1) Source relocation, comment/documentation changes, or other provenance-only edits may leave canonical semantic or Semantic IR results reusable while requiring refreshed diagnostics, LSP mappings, debug/source-content identities, or generated documentation.

§ 59(2) Conversely, semantic edits invalidate all dependent provenance-bearing physical artifacts as required.

§ 59(3) Incremental reuse preserves the source provenance required by the owning Semantic IR and debug-information rulebooks.

---
## § 60. Edit magnitude is not correctness

**Governance tags:** `compiler.incremental-compilation-v1`

§ 60(1) The textual size of an edit is a scheduling and performance heuristic, not an invalidation rule.

§ 60(2) A one-character edit may invalidate broad semantic state; a large semantically irrelevant edit may preserve it.

§ 60(3) Implementations may abandon fine-grained reuse and reparse or reanalyse broader regions whenever safe syntax context, semantic identity, or provenance reuse cannot be established.

---
## § 61. Semantic state and plan-specific realization

**Governance tags:** `compiler.incremental-compilation-v1`

§ 61(1) Verified frontend semantic state and Semantic IR may be reused independently of target-, ABI-, layout-, or backend-specific realization where their owning contracts permit.

§ 61(2) Sec MLIR, lower representations, LLVM/backend representations, objects, and other physical artifacts depend on the concrete `CompilationPlan` and transformation contracts they consume.

§ 61(3) One semantic entity or generic specialization may therefore have multiple plan-specific realizations without changing canonical semantic identity.

---
## § 62. Lowering-boundary compatibility

**Governance tags:** `compiler.incremental-compilation-v1`

§ 62(1) A cached lowering result is reusable only under compatible input representation identity, representation/schema contract, relevant `CompilationPlan` facts, lowering/pass-pipeline contract, optimization policy, and verifier contract, as applicable.

§ 62(2) Changing a later lowering or backend implementation need not invalidate earlier compatible semantic layers.

§ 62(3) An incompatible producing transformation invalidates its output and every lower artifact that depends on it.

---
## § 63. Physical compilation granularity is implementation-defined

**Governance tags:** `compiler.incremental-compilation-v1`

§ 63(1) Incremental code generation may operate per function, specialization, module, IR unit, object, LTO partition, or another safe physical unit.

§ 63(2) This rulebook does not require source-file/object correspondence or function-level backend caching.

§ 63(3) A backend may conservatively rebuild a coarser physical unit when it cannot safely reuse a finer one.

---
## § 64. ABI and layout are downstream physical dependencies

**Governance tags:** `compiler.incremental-compilation-v1`

§ 64(1) Call sites, by-value operations, field/layout operations, FFI boundaries, allocation, and other physical consumers depend on the concrete ABI/layout facts they materialize.

§ 64(2) A changed `ABISignature`, resolved layout, calling convention, address-space representation, or equivalent fact invalidates dependent physical realizations even when canonical declaration or type identity is unchanged.

§ 64(3) If the consumed ABI/layout fact remains canonically unchanged, propagation through that dependency domain may stop.

---
## § 65. Generic physical realization is recomputable

**Governance tags:** `compiler.incremental-compilation-v1`

§ 65(1) Canonical generic specialization identity follows `rules/compiler/monomorphization.md` and is not recreated merely because machine realization changes.

§ 65(2) Plan-, layout-, optimization-, and backend-specific generic artifacts invalidate independently.

§ 65(3) Physical implementation sharing or coalescing may be recomputed, added, or removed without merging semantic specialization identity or per-specialization semantic/static state.

---
## § 66. Cached artifacts do not create demand or reachability

**Governance tags:** `compiler.incremental-compilation-v1`

§ 66(1) The current compilation request, semantic demand graph, selected tests, exported/binary surfaces, platform/runtime requirements, and canonical `LinkPlan` determine which artifacts are required.

§ 66(2) The existence of cached specializations, helpers, objects, symbols, or debug records never by itself creates demand, reachability, or a link root.

§ 66(3) Removed demand must remove stale output participation, and newly introduced demand must be discovered even when old artifacts otherwise remain available.

---
## § 67. Linking and LTO reuse consume the current LinkPlan

**Governance tags:** `compiler.incremental-compilation-v1`

§ 67(1) Incremental linking, cached linker state, and LTO reuse are permitted when supported but remain subordinate to the canonical current `LinkPlan`, `LinkEnvironment`, root/surface model, input/native dependency identities, ABI contracts, LTO policy, toolchain identity, and output-artifact contract defined by `rules/compiler/linking.md`.

§ 67(2) A full relink is the correctness fallback when compatibility cannot be established.

§ 67(3) Incremental linker or LTO state does not define a competing program-reachability model.

---
## § 68. Mixed-generation physical inputs

**Governance tags:** `compiler.incremental-compilation-v1`

§ 68(1) A final build may combine physical artifacts produced at different times or in previous builds.

§ 68(2) Creation time is irrelevant. Every reused physical input must satisfy the current logical snapshot, plan, semantic/ABI dependency, verification, and link contracts required by the final artifact.

§ 68(3) Removed symbols, objects, specializations, debug records, and other stale state must not survive merely because an older reusable container still exists.

---
## § 69. Transactional final-artifact publication

**Governance tags:** `compiler.incremental-compilation-v1`

§ 69(1) A new object or final artifact becomes the current successful output only after required generation/linking and applicable artifact verification complete successfully.

§ 69(2) A failed, partial, or unverified incremental candidate must not replace an earlier known-good published artifact.

§ 69(3) Cached prior verification status is reusable only under compatible artifact, plan, and verifier contracts. Otherwise verification is repeated or the artifact is rejected.

---
## § 70. Lower-stage failure does not redefine higher-stage truth

**Governance tags:** `compiler.incremental-compilation-v1`

§ 70(1) Backend, lowering, LTO, linker, toolchain, or artifact-verification failures remain distinct from source-language semantic invalidity.

§ 70(2) Compatible higher-level semantic, analysis, and Semantic IR results may remain reusable after a lower-stage failure.

§ 70(3) An implementation-capability or physical-artifact failure must not be memoized as though the Sec source itself were semantically invalid.

---
## § 71. History-independent canonical results

**Governance tags:** `compiler.incremental-compilation-v1`

§ 71(1) For the same final canonical source/project inputs and `CompilationPlan`, the compiler must converge to equivalent canonical results regardless of the edit sequence, prior compilation sequence, cache population history, restart history, or physical reuse path that led to those inputs.

§ 71(2) History may affect compiler work and telemetry. It is not a program-semantic input.

§ 71(3) Incremental correctness therefore requires convergence to a function of current canonical inputs, not a function of everything the compiler happens to remember.

---
## § 72. Invalid intermediate snapshots do not poison later states

**Governance tags:** `compiler.incremental-compilation-v1`

§ 72(1) A sequence containing syntax errors, recovery state, negative lookup results, `Invalid` or `Unproven` analysis facts, or failed lower-stage work must not cause those facts to survive after their dependencies cease to hold.

§ 72(2) Transitions among valid, invalid, and unproven states work in every direction according to the owning contracts.

§ 72(3) A later valid snapshot must converge to the same canonical result as cold compilation of that snapshot.

---
## § 73. Revert and undo

**Governance tags:** `compiler.incremental-compilation-v1`

§ 73(1) When edits return canonical inputs to a previous compatible state, compatible old results may be reused if they remain available.

§ 73(2) Their presence is not required. Eviction or cache clearing may force recomputation without changing the restored program semantics.

§ 73(3) Undo and revert therefore change reuse opportunity, not the definition of the program.

---
## § 74. Cache population does not create canonical entities

**Governance tags:** `compiler.incremental-compilation-v1`

§ 74(1) Irrelevant stale entries, prior specializations, removed declarations/artifacts, and query discovery order must not affect current semantic identities, candidate/member sets, reachability, diagnostic identity/cause selection, canonical ordering, or artifact membership.

§ 74(2) Canonical entities and representative choices come from the current rulebook contracts, not cache insertion order or worker discovery order.

§ 74(3) A cache may accelerate discovery of a current entity but may not create that entity.

---
## § 75. Deterministic compatibility, flexible reuse strategy

**Governance tags:** `compiler.incremental-compilation-v1`

§ 75(1) For identical canonical inputs and compatibility domains, cache/dependency identities and the validity decision for the same candidate are deterministic.

§ 75(2) Eviction, resource pressure, speculative scheduling, concurrent production, or remote-cache availability may cause different valid hit/miss/recompute patterns.

§ 75(3) The implementation need not reproduce or maximize an exact hit pattern, provided every actual reuse is valid and final canonical results remain equivalent.

---
## § 76. Completed diagnostics and semantic representations converge

**Governance tags:** `compiler.incremental-compilation-v1`

§ 76(1) For a completed current snapshot, incremental compilation produces the same canonical diagnostic facts and equivalent canonical semantic/Semantic-IR results as cold compilation under the same configuration.

§ 76(2) Cache and recomputation order must not change canonical diagnostic ordering, representative cause selection, or current source attribution where the owning diagnostic contract specifies them.

§ 76(3) Explicitly stale or pending interactive presentation is separate and remains governed by the tooling model.

---
## § 77. Physical output equivalence follows the artifact contract

**Governance tags:** `compiler.incremental-compilation-v1`

§ 77(1) Where the active target/toolchain/artifact contract guarantees reproducible bytes, incremental and cold builds with identical canonical physical inputs satisfy that guarantee.

§ 77(2) Where byte reproducibility is not guaranteed, incremental output must still satisfy the same semantic, ABI, symbol, debug, runtime, and final-artifact invariants as the corresponding cold build.

§ 77(3) Incremental compilation neither strengthens nor weakens the owning reproducibility contract.

---
## § 78. Repeated unchanged compilation is semantically idempotent

**Governance tags:** `compiler.incremental-compilation-v1`

§ 78(1) Repeated compilation of an unchanged logical snapshot and plan may increase cache warmth or alter non-semantic telemetry, but must not progressively change canonical analysis facts, diagnostics, demand/reachability, semantic representations, or artifact contract.

§ 78(2) Long-lived compiler processes may retain obsolete snapshot state physically, but obsolete entities and dependency edges may not re-enter current semantic state merely through accumulation.

§ 78(3) Dependency-graph and cache garbage collection may be lazy as long as logical current membership remains correct.

---
## § 79. Concurrency and distributed-cache variation preserve correctness

**Governance tags:** `compiler.incremental-compilation-v1`

§ 79(1) Different valid worker schedules, duplicate computations, invalidation/computation races resolved under the publication rules, and local-versus-remote cache availability may change work performed but must converge to equivalent canonical results.

§ 79(2) Inspection and telemetry may truthfully report different reuse histories for different executions.

§ 79(3) Those histories are not themselves Sec semantics.

---
## § 80. Path-to-same-snapshot conformance

**Governance tags:** `compiler.incremental-compilation-v1`

§ 80(1) Verification must support comparing cold compilation of a final snapshot, repeated/warm compilation of that snapshot, and one or more incremental edit histories that reach the same final canonical inputs.

§ 80(2) All such paths must satisfy the same semantic and applicable artifact contracts.

§ 80(3) Testing only an immediate cold-versus-warm pair is insufficient evidence for stateful incremental correctness.

---
## § 81. Cold compilation is a differential reference path

**Governance tags:** `compiler.incremental-compilation-v1`

§ 81(1) Incremental conformance compares completed incremental results with cold compilation of the same final canonical inputs and `CompilationPlan`.

§ 81(2) Cold compilation is a differential reference path, not a replacement for rulebook-derived correctness oracles. The cold path must itself satisfy ordinary compiler conformance.

§ 81(3) A cold/incremental match proves equivalence between paths; feature rulebooks and compiler-testing evidence establish whether the shared result is correct Sec behavior.

---
## § 82. Correctness evidence and incrementality evidence are distinct

**Governance tags:** `compiler.incremental-compilation-v1`

§ 82(1) Incremental tests verify final canonical correctness/equivalence and, where applicable, the reuse/invalidation behavior promised by the implemented incremental capability or granularity.

§ 82(2) A compiler that cold-recomputes everything may remain semantically correct but does not thereby satisfy a claimed incremental reuse contract.

§ 82(3) Tests should assert stable dependency-domain behavior rather than incidental exact query/cache-hit counts unless such counts are explicitly part of an implementation contract.

---
## § 83. State-transition coverage

**Governance tags:** `compiler.incremental-compilation-v1`

§ 83(1) Incremental conformance includes representative sequences covering additions, removals, modifications, moves or renames, invalid-to-valid recovery, valid-to-invalid transitions, and revert/undo where those operations exercise distinct dependency contracts.

§ 83(2) Set-valued and negative dependencies receive explicit addition/removal coverage.

§ 83(3) The objective is sufficient evidence for stale-state removal and new-dependency discovery, not a fixed number of tests per feature.

---
## § 84. Plan and context isolation tests

**Governance tags:** `compiler.incremental-compilation-v1`

§ 84(1) Incremental tests include transitions and warm-cache scenarios across materially distinct targets, variants, ABIs, layouts, optimization/debug policies, production/test module views, and other context identities where reuse compatibility differs.

§ 84(2) Tests deliberately populate cache state under an incompatible context before requesting another context so that isolation is proven rather than inferred from separate clean runs.

§ 84(3) The matrix is contract-driven according to `rules/compiler/compiler_testing.md`; a full Cartesian product is not required.

---
## § 85. Malformed and incompatible cache state is first-class test input

**Governance tags:** `compiler.incremental-compilation-v1`

§ 85(1) Persistent and in-memory cache conformance includes deterministic injection of stale dependencies, wrong plan/domain entries, schema incompatibility, corruption or truncation, metadata/payload mismatch, obsolete verifier state, and partial/crashed publication where applicable.

§ 85(2) Such state must be rejected, isolated, or recomputed according to its contract without silently changing Sec semantics.

§ 85(3) Cache-rejection behavior is tested as infrastructure behavior, not represented as intentionally invalid Sec source.

---
## § 86. Race and publication testing

**Governance tags:** `compiler.incremental-compilation-v1`

§ 86(1) Tests exercise controlled interleavings involving duplicate computation/publication, reader/writer races, eviction/read races, invalidation versus computation, request cancellation, and late obsolete completion where implemented.

§ 86(2) Deterministic orchestrated tests establish required outcomes. Repetition, scheduler perturbation, stress, and fuzzing provide additional evidence rather than replacing the contract test.

§ 86(3) Concurrency tests use bounded completion and separate harness/infrastructure failures from compiler defects according to `compiler_testing.md`.

---
## § 87. Stateful edit-history fuzzing and property testing

**Governance tags:** `compiler.incremental-compilation-v1`

§ 87(1) Incremental verification includes generated or fuzzed sequences of source, project, plan, and cache operations rather than only isolated source inputs.

§ 87(2) For each relevant completed state, the harness may compare incremental output with cold compilation of the same current canonical inputs.

§ 87(3) A discovered failure retains the concrete initial state and operation sequence. Reduction may simplify both source complexity and history while preserving the failure predicate.

---
## § 88. Mutation testing of dependency and invalidation logic

**Governance tags:** `compiler.incremental-compilation-v1`

§ 88(1) Incremental dependency and invalidation logic is soundness-critical mutation-testing scope under `rules/compiler/compiler_testing.md`.

§ 88(2) Representative mutants include omitted dependency edges, ignored set additions/removals, stale negative lookup reuse, missing ABI/debug/plan key components, obsolete snapshot publication, verifier-version bypass, and stale link-root retention.

§ 88(3) An `always reuse if present` strategy must fail applicable invalidation suites. An `always cold recompute` strategy may preserve semantic correctness tests but must fail evidence for any incremental capability it claims but no longer implements.

---
## § 89. Reuse paths are tested according to implemented capabilities

**Governance tags:** `compiler.incremental-compilation-v1`

§ 89(1) Where supported, conformance distinguishes cold process execution, warm same-process reuse, process restart with persistent cache, and shared or remote producer-consumer reuse.

§ 89(2) Each path verifies equivalent canonical results and its own compatibility, integrity, publication, and trust obligations.

§ 89(3) A reuse mode not implemented or claimed in Sec 0.1 need not be fabricated merely to satisfy the test matrix.

---
## § 90. No unresolved supported-path cold/incremental divergence at closure

**Governance tags:** `compiler.incremental-compilation-v1`

§ 90(1) A reproducible difference in canonical semantic diagnostics, Semantic IR meaning, ABI/artifact contract, or Sec-observable program behavior between cold and incremental compilation of the same supported final inputs is a compiler-correctness defect.

§ 90(2) Such divergence is not an ordinary performance issue and is not made acceptable by instructing users to clear caches.

§ 90(3) Relevant conformance remains incomplete until the defect is corrected or the affected capability/path is accurately declared unsupported.

---
## § 91. Canonical ownership and path

**Governance tags:** `compiler.incremental-compilation-v1`

§ 91(1) `rules/compiler/incremental_compilation.md` is the canonical Sec 0.1 rulebook for compiler incremental-reuse semantics.

§ 91(2) It owns logical compilation snapshots, dependency validity, cache/fingerprint reuse, invalidation, stale-state prevention, persistent/shared cache correctness, incremental module/source/provenance behavior, incremental artifact reuse, and history independence.

§ 91(3) It introduces no competing source-language semantic model.

---
## § 92. Incremental compilation consumes identities owned elsewhere

**Governance tags:** `compiler.incremental-compilation-v1`

§ 92(1) Canonical module, declaration, type, generic-specialization, analysis, Semantic IR, ABI/layout, debug, `LinkPlan`, source, and artifact identities remain owned by their existing rulebooks.

§ 92(2) This rulebook defines whether previously derived results involving those identities remain reusable under the current snapshot and `CompilationPlan`.

§ 92(3) A cache identity never supersedes its owning canonical semantic identity.

---
## § 93. Governance ownership

**Governance tags:** `compiler.incremental-compilation-v1`

§ 93(1) Implementation status for this rulebook is tracked by `compiler.incremental-compilation-v1` in `governance/compiler.yaml`.

§ 93(2) No separate incremental governance fragment is introduced. The existing compiler governance fragment owns compiler-pipeline and artifact concerns and remains the semantic owner location for this integration.

§ 93(3) A future governance-wide reorganization may move the integration only through the normal governance migration process.

---
## § 94. Initial governance status is conservative

**Governance tags:** `compiler.incremental-compilation-v1`

§ 94(1) Writing this rulebook does not imply that canonical incremental compilation is implemented.

§ 94(2) Until a usable canonical incremental vertical slice is verified, `compiler.incremental-compilation-v1` remains `planned`.

§ 94(3) Existing `CompilationPlan`, analysis, `ModuleSurface`, monomorphization, linking, debug, LSP, and compiler-testing capabilities are foundations and dependencies; they are not mislabeled as implementation of the complete incremental contract.

---
## § 95. Incremental governance tracks incremental work only

**Governance tags:** `compiler.incremental-compilation-v1`

§ 95(1) The incremental governance integration tracks snapshot/generation management, dependency/query-result tracking, domain fingerprints, set/negative invalidation, reuse compatibility, incremental publication, persistent-cache validity, module/source/provenance reuse, lowering/artifact/link reuse integration, inspection metadata, history-independent convergence, and feature-specific incremental conformance.

§ 95(2) It does not duplicate implementation backlogs for the semantic facts, ABI, linker, debugger, module system, generics, analyses, or testing infrastructure it consumes.

§ 95(3) Cross-integration dependencies are referenced rather than copied.

---
## § 96. Compiler-testing infrastructure remains independently owned

**Governance tags:** `compiler.incremental-compilation-v1`

§ 96(1) `testing.compiler-verification-v1` continues to own generic compiler-test, differential, fault-injection, fuzzing, mutation, matrix, debugger-driving, regression, and corpus infrastructure.

§ 96(2) `compiler.incremental-compilation-v1` owns the incremental invariants and feature-specific evidence obligations defined by §§ 81-90.

§ 96(3) The generic cold-versus-incremental differential hook is activated against this rulebook rather than becoming a second definition of cache or invalidation semantics.

---
## § 97. Cross-rulebook harmonization

**Governance tags:** `compiler.incremental-compilation-v1`

§ 97(1) Integration of this rulebook requires existing references to the planned incremental book to use the canonical path `rules/compiler/incremental_compilation.md` and to treat this book as the current owner rather than a future placeholder.

§ 97(2) `language-rulebook-status.md` is updated from the planned root-level entry to `compiler/incremental_compilation.md` with status `Written`, and the planned-rulebook closure list is updated accordingly.

§ 97(3) These harmonizations correct references and ownership only; they do not silently renumber or redefine unrelated normative paragraphs.

---
## § 98. Design completion and implementation completion are separate

**Governance tags:** `compiler.incremental-compilation-v1`

§ 98(1) This rulebook is design-complete when the locked D1-D100 decisions are represented audibly in normative text, cross-rulebook conflicts are reconciled, and governance ownership is recorded.

§ 98(2) The implementation integration is complete only when its applicable mandatory capabilities are implemented and verified.

§ 98(3) No cache-hit percentage, build-time threshold, line-coverage percentage, mutation-score threshold, or arbitrary test count substitutes for correctness and conformance evidence.

---
## § 99. Sec 0.1 mandates a correctness model, not one architecture

**Governance tags:** `compiler.incremental-compilation-v1`

§ 99(1) Sec 0.1 does not require one specific query engine, persistent cache format/location, content-addressed storage, compiler daemon, remote/distributed cache, incremental linker, maximally fine-grained invalidation, or cross-machine reuse.

§ 99(2) Implementations may support any subset consistent with their declared capabilities.

§ 99(3) Every supported reuse mode remains subject to the snapshot, dependency, compatibility, integrity, publication, and cold-equivalence rules of this book.

---
## § 100. Closure of the planned incremental rulebook

**Governance tags:** `compiler.incremental-compilation-v1`

§ 100(1) After this rulebook is integrated and the required path/status/ownership corrections are applied, the planned `incremental_compilation.md` Sec 0.1 rulebook item is complete.

§ 100(2) Future implementation refinement, performance work, and additional cache capabilities proceed through governance without reopening the Sec 0.1 semantic contract unless implementation experience reveals a genuine specification defect.

§ 100(3) A discovered semantic conflict is corrected in the owning canonical rulebook or correction mechanism rather than resolved through implementation-specific behavior.

---
# Appendix A. Canonical incremental model

This appendix summarizes the normative model above without introducing additional rules.

```text
Current canonical inputs
        |
        v
logical source/project snapshot + CompilationPlan
        |
        v
query / fact / artifact identity
        |
        v
validate dependency domains and compatibility
        |
        +------------------------------+
        |                              |
        | reusable                     | not proven reusable
        v                              v
reuse validated result           recompute from canonical inputs
        |                              |
        +---------------+--------------+
                        |
                        v
             complete verified result
                        |
                        v
            logically atomic publication
                        |
                        v
             downstream dependency graph
```

A cache hit is a proof obligation, not an optimization guess.

Cold recomputation is the correctness fallback.

Canonical identity answers **what an entity is**. A fingerprint or cache key answers **whether a prior derived result may still be reused**.

---
# Appendix B. Dependency-domain summary

A conforming implementation may use coarser or finer physical caches, but the logical model distinguishes dependencies when their invalidation contracts differ.

```text
Source/syntax
    source content
    lexical/parser/recovery context
    source identity and ranges

Module and declaration semantics
    ModuleIdentity / ModuleInstance / ModuleSurface
    declaration/member/candidate sets
    visibility/access
    signatures and required type information

Semantic analysis
    ownership / borrowing / lifetime
    effects and call-target sets
    compile-time evaluation
    conformance / generic constraints
    fixed-point summaries

Generic specialization
    canonical specialization identity
    template semantic fingerprint
    specialization-relevant dependencies

Semantic IR
    canonical semantic operations
    current proof/verification contract
    source provenance

Plan-specific realization
    target / ABI / layout / address spaces
    optimization and backend pipeline
    runtime/platform policy

Physical artifacts
    object/debug metadata
    toolchain identity
    linker/LTO state
    LinkPlan and final-artifact contract
```

The exact physical representation is implementation-defined.

---
# Appendix C. Stateful correctness oracle

Incremental correctness is stronger than a single immediate warm build.

```text
Path 1:
    A -> B -> C -> Final

Path 2:
    A -> D -> Final

Path 3:
    cold(Final)

Required:
    canonical_result(Path 1)
      == canonical_result(Path 2)
      == canonical_result(Path 3)
```

Relevant comparisons include canonical diagnostics, semantic facts, Semantic IR meaning, ABI/artifact contracts, and Sec-observable execution behavior according to the owning rulebooks.

Physical cache hit counts, elapsed time, memory usage, and eviction history are not part of this equivalence.

---
# Appendix D. Required incremental test families

This appendix summarizes §§ 81-90 and does not replace `rules/compiler/compiler_testing.md`.

```text
cold versus warm equivalence
multi-step edit histories
add/remove set dependencies
negative-to-positive and positive-to-negative transitions
revert / undo
source move / provenance-only edits
CompilationPlan and Variant isolation
production versus test-view isolation
wrong-plan / stale / corrupt cache rejection
partial-publication recovery
concurrent publication and late-result races
persistent-cache restart paths
shared-cache producer/consumer paths where supported
stateful incremental fuzzing
history-aware regression reduction
mutation testing of dependency/invalidation logic
```

---
# Appendix E. Decision audit D1-D100

This appendix is a review aid. The normative requirements are the numbered sections above.

| Decision | Normative home |
| --- | --- |
| D1 | § 1 |
| D2 | § 2 |
| D3 | § 3 |
| D4 | § 4 |
| D5 | § 5 |
| D6 | § 6 |
| D7 | § 7 |
| D8 | § 8 |
| D9 | § 9 |
| D10 | § 10 |
| D11 | § 11 |
| D12 | § 12 |
| D13 | § 13 |
| D14 | § 14 |
| D15 | § 15 |
| D16 | § 16 |
| D17 | § 17 |
| D18 | § 18 |
| D19 | § 19 |
| D20 | § 20 |
| D21 | § 21 |
| D22 | § 22 |
| D23 | § 23 |
| D24 | § 24 |
| D25 | § 25 |
| D26 | § 26 |
| D27 | § 27 |
| D28 | § 28 |
| D29 | § 29 |
| D30 | § 30 |
| D31 | § 31 |
| D32 | § 32 |
| D33 | § 33 |
| D34 | § 34 |
| D35 | § 35 |
| D36 | § 36 |
| D37 | § 37 |
| D38 | § 38 |
| D39 | § 39 |
| D40 | § 40 |
| D41 | § 41 |
| D42 | § 42 |
| D43 | § 43 |
| D44 | § 44 |
| D45 | § 45 |
| D46 | § 46 |
| D47 | § 47 |
| D48 | § 48 |
| D49 | § 49 |
| D50 | § 50 |
| D51 | § 51 |
| D52 | § 52 |
| D53 | § 53 |
| D54 | § 54 |
| D55 | § 55 |
| D56 | § 56 |
| D57 | § 57 |
| D58 | § 58 |
| D59 | § 59 |
| D60 | § 60 |
| D61 | § 61 |
| D62 | § 62 |
| D63 | § 63 |
| D64 | § 64 |
| D65 | § 65 |
| D66 | § 66 |
| D67 | § 67 |
| D68 | § 68 |
| D69 | § 69 |
| D70 | § 70 |
| D71 | § 71 |
| D72 | § 72 |
| D73 | § 73 |
| D74 | § 74 |
| D75 | § 75 |
| D76 | § 76 |
| D77 | § 77 |
| D78 | § 78 |
| D79 | § 79 |
| D80 | § 80 |
| D81 | § 81 |
| D82 | § 82 |
| D83 | § 83 |
| D84 | § 84 |
| D85 | § 85 |
| D86 | § 86 |
| D87 | § 87 |
| D88 | § 88 |
| D89 | § 89 |
| D90 | § 90 |
| D91 | § 91 |
| D92 | § 92 |
| D93 | § 93 |
| D94 | § 94 |
| D95 | § 95 |
| D96 | § 96 |
| D97 | § 97 |
| D98 | § 98 |
| D99 | § 99 |
| D100 | § 100 |

---
# Appendix F. Sec 0.1 exclusions and capability boundaries

The following are not mandated merely by this rulebook:

- one specific query/memoization framework;
- one persistent cache directory or database format;
- content-addressed storage;
- a long-lived compiler daemon;
- remote or distributed cache infrastructure;
- cross-machine cache reuse;
- incremental linking;
- function-level codegen caching;
- maximal/minimal invalidation precision;
- a public Sec cache API;
- exact cache-hit counts or performance thresholds.

When any such capability is implemented and claimed, it remains subject to the canonical correctness rules in this book.

---
# Appendix G. Ownership summary

```text
rules/compiler/incremental_compilation.md
    owns:
        snapshots / generations
        dependency validity
        cache/fingerprint reuse
        invalidation
        publication
        history independence
        persistent/shared reuse correctness
        incremental artifact reuse rules

compiler_analysis.md
    owns:
        semantic analysis facts and fixed-point meaning

projects/modules.md
    owns:
        ModuleIdentity / ModuleInstance / ModuleSurface

monomorphization.md
    owns:
        canonical generic specialization identity
        generic semantic specialization contracts

semantic_ir.md
    owns:
        canonical Semantic IR meaning and verifier invariants

linking.md
    owns:
        LinkPlan / LinkEnvironment / link roots / final artifact contract

debug_information.md
    owns:
        debug semantic model and artifact/source correlation

compiler_testing.md
    owns:
        shared compiler verification infrastructure
```
