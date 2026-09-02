# Sec rulebooks

This directory contains the normative Sec language and compiler rulebooks.
`language-rulebook-status.md` in the repository root is the canonical inventory
of written, planned, living, and deferred documents. Implementation progress
belongs in `implementation-status.yaml`.

## Directory map

| Directory | Scope |
|---|---|
| `foundations/` | Language philosophy, lexical structure, grammar, names, operators, and attributes. |
| `types/` | Core type rules, contracts, defaults, and units. |
| `declarations/` | Declarations, functions, generics, interfaces, implementations, and properties. |
| `control-flow/` | Branching, loops, match, defer, and discard. |
| `collections/` | Collections and shaped values. |
| `memory/` | Ownership, borrowing, references, storage, layout, allocation, and unsafe code. |
| `errors/` | Error handling, panic, and runtime checks. |
| `concurrency/` | Tasks, threads, synchronization, scheduling, and the concurrency memory/runtime models. |
| `analysis/` | Compiler analyses and their shared semantic contracts. |
| `compiler/` | Compiler architecture, pipeline, parser recovery, canonical Semantic IR, linking, and final artifacts. |
| `mlir/` | Sec MLIR governance, dialect, lowering, version history, amendments, and implementation packages. |
| `platform/` | Target profiles, platform resolution, interrupts, FFI, fixed-address and volatile access, hardware-register access, and ABI rules. |
| `projects/` | Project and build organization. |
| `tooling/` | Diagnostics, formatter, source-level testing/toolchain integration, and LSP/editor integration. |
| `library/` | Core-library and standard-library contracts. |

## MLIR layout

The MLIR documentation is intentionally subdivided because it has several
different lifecycles:

| Path | Meaning |
|---|---|
| `mlir/sec_mlir.md` | Governance and the high-level Sec MLIR boundary. |
| `mlir/sec_mlir_dialect.md` | Current canonical dialect specification. |
| `mlir/sec_mlir_lowering.md` | Current canonical lowering specification. |
| `mlir/mlir.txt`, `mlir/mlir-optimize.txt` | General and living optimization notes. |
| `mlir/packages/` | Bounded implementation packages and their separate YAML status files. |
| `mlir/dialect-versions/` | Superseded dialect snapshots retained for history and compatibility. |
| `mlir/lowering-versions/` | Superseded lowering snapshots retained for history and compatibility. |
| `mlir/semantic-ir/` | Package-specific Semantic IR amendments. |
| `mlir/normative-sync/` | Package-specific amendments to non-MLIR rulebooks. |

New documents should be placed in the narrowest applicable directory and use
`.md` unless they extend an existing `.txt` rulebook.

## Implementation traceability

Compiler code that implements a normative language decision should cite the
canonical rulebook path in a nearby file- or function-level English comment.
When a correction document drives the change, cite that correction while it is
active; after archival, the canonical rulebook remains the durable authority.

Comments should identify the semantic reason for non-obvious parser, Sema,
ownership, control-flow, diagnostic, Semantic IR, lowering, ABI, target, or
formatter behavior. Generic infrastructure and self-explanatory mechanics do
not need artificial rule citations. Tests should carry the same traceability
where it clarifies the contract being protected. Rulebook moves require code
references to be updated in the same change.
