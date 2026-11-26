# Project profile/allocation workspace

This service owns typed harness edits to `.sec/sec.toml` under MD-139. It uses
`project.ParseManifest`, `project.ResolvePlanSelection` and
`target.ResolveTargetProfile`; it introduces no manifest keys or providers.
The compiler CLI and optional MCP adapter call the same service.

An operation names an existing `target`, `variant`, or both, and any of
`profile`, `mode`, `provider`. Omitted fields preserve local values; `""` clears
a local key and restores normal inheritance. An unknown scope is rejected.
A Target-specific Variant override can be added for an existing Target and
reusable Variant. Modes and profile families keep their canonical meaning;
`[build].profile` and build-option profiles are never edited by this API.

`Inspect` validates the manifest and reports canonical profile/allocation facts
for reusable Variants and declared Target–Variant pairs. `Resolve` additionally
returns the selected C-binding environment, modules, source layers and fingerprint
through the same binding-plan resolver used by compiler/LSP analysis. Profile
and binding selection use one manifest snapshot. All filesystem access stays
inside the retained OS project-root handle, including selected module sources.
Scope provenance, target triple/ABI and the exact canonical resolved profile are
returned, without projecting an unknown provider to Supported.

A `PlanChange` contains the fixed file, typed edit, project identity, input/output
hashes, patch and resolved profile facts. The digest covers every review field.
Planning never grants authorization. `Apply` requires separate trusted approval
of that digest, regenerates the canonical proposal, rejects stale input and
commits through `configedit`'s shared rooted atomic replacement. No-op edits
preserve bytes; successful edits preserve unrelated comments and permissions.
Independent processes must cooperate with optimistic concurrency; an atomic
rename does not provide cross-process compare-and-swap.

`configedit` extracts the pre-existing C-binding TOML splice, diff and atomic
replacement algorithms. Schema validation remains with each owning service;
the low-level editor is not another TOML parser or source of configuration truth.
Unsupported layouts fail rather than reserializing unrelated content.

The current target provider catalogs are empty. Selecting `sec.system-heap.v1`
or another unqualified provider fails through the canonical resolver. Completing
this tooling does not complete MD-151 provider qualification or backend support.
