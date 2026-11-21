# Sec Tooling Harness — Initial Design Notes

- **Document:** Sec Tooling Harness (working title)
- **Status:** Exploration / non-normative; no new Sec language decisions
- **Target milestone:** Sec 0.1.100 (tentative)
- **Created:** 2026-11-20
- **Last updated:** 2026-11-20
- **Document revision:** 1
- **Related decisions:** MD-021 (C binding environments), MD-034 (freestanding allocation policy), MD-139 (allocation profile/provider selection)
- **Primary sources:** `rules/projects/projects.md`, `rules/platform/target_profiles.md`, `rules/memory/allocation.md`, `missing-decisions.yaml`; MD-021 decisions established separately
- **Repository scope:** Proposal only. No source, manifest schema, or governance changes in this document.

---

## § 1. Purpose and boundaries

**§ 1(1)** Sec should support AI-assisted development workflows without placing an AI model, prompt runtime, probabilistic decision process, or agent dependency into the Sec language, compiler semantics, or ordinary build path.

**§ 1(2)** A *tooling harness* is a deterministic project-aware service for inspection, validation, configuration planning, transactional edits, and reporting. Its consumers may be humans, IDEs, scripts, or AI tools.

**§ 1(3)** MCP is a transport/tool-discovery integration, not the source of truth for project rules. The harness must also work without AI or MCP.

**§ 1(4)** Sec 0.1.100 is a planning milestone, not an approved release commitment. This note does not extend MD-139's scope or change MD-021 decisions.

## § 2. Executables and shared implementation

**§ 2(1)** **Proposed:** A separate `sec-mcp` executable exposes selected workspace operations through Model Context Protocol, initially over local stdio.

**§ 2(2)** The existing `sec` command retains its human-facing subcommands. Both surfaces call shared Go service APIs; `sec-mcp` must not maintain its own divergent TOML parser, semantic validator, or file-editing code.

**§ 2(3)** The service layer is reusable without starting `sec-mcp` and does not require an AI client. Naming of the eventual executable and Go package structure remains provisional.

**§ 2(4)** The harness must not silently shell out to `sec` merely to implement configuration mutation when the canonical code can be called directly. A subprocess adapter may exist only as a deliberately constrained compatibility layer.

## § 3. Configuration authorities

**§ 3(1)** Existing canonical configuration files retain their responsibility:

```text
.sec/sec.toml                 Project, Target, Variant and associated profiles
.sec/platform/c-bindings.toml  Selected C binding environments and shared modules
```

**§ 3(2)** The harness reads and mutates these files only through their respective owning schema/validation services. It cannot invent unrecognized keys.

**§ 3(3)** The already approved MD-139 manifest shape remains independent of this harness:

```toml
[variant.board-m4]
os = "baremetal"
arch = "arm32"
profile = "baremetal"

[variant.board-m4.allocation]
mode = "embedded-arena"
provider = "board-primary-arena"
```

Here the provider name is an example identifier, not proof that a target supplies one.

**§ 3(4)** An optional future harness-specific TOML section is **not yet named or specified**. If introduced, it will hold tool configuration and access policy, not duplicate compiler-semantic project facts.

## § 4. Preserve the MD-021 command surface

**§ 4(1)** The following approved human command family remains the canonical workflow for C binding configuration:

```text
sec platform c-bindings init
sec platform c-bindings show
sec platform c-bindings validate
sec platform c-bindings environment add
sec platform c-bindings environment set
sec platform c-bindings environment remove
sec platform c-bindings module add
sec platform c-bindings module remove
```

**§ 4(2)** Each operation has a typed request and deterministic structured response in the underlying service. The MCP adapter maps these operations to discoverable tools without changing their behavior.

**§ 4(3)** C-binding selection/inspection should expose a read-only resolved-view operation with target/variant, selected environments/modules, ABI, provenance, and fingerprint information. This is separate from edits.

**§ 4(4)** MD-021's existing requirements for JSON results, idempotent and comment-preserving atomic edits, file-hash concurrency checks, and diagnostics remain authoritative; the harness reuses them.

## § 5. Operations and safety model

**§ 5(1)** Distinguish read-only operations (`inspect`, `show`, `validate`, `resolve`, `diff`) from mutating operations (`init`, `add`, `set`, `remove`). Tool metadata must truthfully declare the distinction.

**§ 5(2)** **Proposed** two-phase mutation workflow:

```text
Inspect -> PlanChange -> ShowDiff -> Approve -> Apply -> Validate
```

**§ 5(3)** A change plan states target file, typed operation, expected input SHA-256, canonical validation consequences, and a preview of the patch. Applying fails rather than overwriting if the input hash changed.

**§ 5(4)** File writes are atomic to the extent supported by the host filesystem. Preserve unrelated comments and formatting whenever the owning config-edit contract requires it. Reapplying an already-applied idempotent operation must not generate unnecessary diffs.

**§ 5(5)** The harness itself cannot infer approval from model text. The user or explicitly trusted caller controls authorization to apply mutations. The MCP transport does not bypass filesystem permissions or repository write protections.

**§ 5(6)** Configuration edit tools operate only within an explicitly selected Sec project root and named files. The initial surface has no arbitrary shell, network, or unrestricted file-write tool.

**§ 5(7)** Validation or planning must not reclassify an unavailable target capability as supported; target-specific environment, ABI, allocation/provider and profile facts retain their existing authorities.

## § 6. First usable release slice

**§ 6(1)** Deliver in this order:

1. Shared typed workspace/configuration service with deterministic JSON and validated snapshots.
2. Local CLI functionality for the already approved MD-021 commands.
3. Transactional plan/diff/apply mutation infrastructure, using the owning TOML edit engine.
4. Separate `sec-mcp` adapter exposing read-only tools first, then authorized mutations.
5. Expand to MD-139 project-profile and allocation-provider configuration using the same service and safety rules.

**§ 6(2)** The first release does **not** include model hosting, automatic code generation, autonomous background agents, remote write access, generalized source rewriting, or prompt-driven changes to compiler truth.

**§ 6(3)** Remote HTTP transport, multi-user authorization, automated migration plans, and additional project domains can be evaluated after the local shared service has conformance tests.

## § 7. Validation and acceptance

**§ 7(1)** CLI and MCP must produce the same underlying semantic result for equivalent requests.

**§ 7(2)** Run negative tests for unknown project, path traversal, stale SHA-256, invalid TOML, unknown keys, duplicate names, invalid ABI/module selection, unsupported provider activation, no permission, interrupted write, and double application.

**§ 7(3)** Integration tests should verify comment preservation, deterministic JSON, unchanged files on validation failure, full target/variant provenance, and consistent CLI/LSP/CompilationPlan validation.

**§ 7(4)** The harness is optional. Installing or building Sec must never require an AI account, MCP client, or network access.

## § 8. Open scoping questions — not language-design blockers

**§ 8(1)** Decide the final executable/product name only when implementation starts. `sec-mcp` is a useful protocol-adapter name, but does not need to become the name of the entire workspace service.

**§ 8(2)** Whether a harness-specific TOML section belongs in `.sec/sec.toml`, a dedicated `.sec/tooling.toml`, or is unnecessary is postponed; no such syntax is introduced by this note.

**§ 8(3)** Whether a longer-running local server is needed beyond stdio is postponed until there is a concrete client workflow requiring it.

## § 9. Separation from MD-139 and MD-021

**§ 9(1)** MD-139 governs allocation/profile/provider semantics and manifest schema. Its approval and integration need not wait for the tooling harness.

**§ 9(2)** MD-021 governs C-binding environment semantics and its CLI/editing contract. The harness may expose those operations but must not reopen the owning decisions.

**§ 9(3)** This document can later be promoted into a standalone tooling specification, with stable paragraph identifiers, after the first real prototype and user review.
