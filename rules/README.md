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
| `compiler/` | Compiler architecture, pipeline, parser recovery, and canonical Semantic IR. |
| `mlir/` | Sec MLIR governance, dialect, lowering, version history, amendments, and implementation packages. |
| `platform/` | FFI, registers, ABI-adjacent, and target-facing rules. |
| `projects/` | Project and build organization. |
| `tooling/` | Diagnostics, formatter, and LSP. |
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
