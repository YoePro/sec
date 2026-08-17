# Semantic IR correction — explicit discard operation

- **Status:** Applied normative correction
- **Applied:** 2026-08-17
- **Created:** 2026-08-16
- **Last updated:** 2026-08-16
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `56be75d`
- **Target rulebook:** `rules/compiler/semantic_ir.txt`

---

## Correction

Semantic IR must represent validated discard as an explicit terminal ownership operation.

Canonical conceptual operation:

```text
DiscardValue
```

The operation records at least:

```text
consumed value or Place
resolved type
destruction plan or trivial-destruction classification
source location
resulting availability state when a Place is consumed
discard provenance
```

Discard provenance distinguishes at least:

```text
Explicit
ImplicitCallResult
CompilerTemporary
```

`CompilerTemporary` is compiler provenance and does not add a new source-language discard spelling.

### Required semantics

`DiscardValue` means that the represented semantic value reaches a terminal ownership action.

For non-trivial values, the resolved destruction plan must be executed or linked to the corresponding cleanup/destruction operation.

When a tracked Place is consumed, later use is invalid until a legal reinitialization is represented.

Any previously registered cleanup for the consumed old value must be cancelled or otherwise proven not to execute twice.

### Backend boundary

No lower stage may infer source discard from:

```text
unused SSA result
unused load
missing store
dead machine value
```

Legal discard, must-use acknowledgement, discardability, borrow legality, and lifecycle obligations are frontend/Sema facts before lowering.

Lifecycle-specific operations whose semantics are not ordinary discard remain distinct IR operations.

## Cross-reference

Source semantics are defined by:

```text
rules/control-flow/discard.md
```
