# P26 Decision Index — Existing MD Gates and New-Decision Policy

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-19
- **Last updated:** 2026-10-19
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/mlir/packages/sec-mlir-dialect_package26_decision-index.md` (installed 2026-10-20)
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P26`

---
## § 1. Live decision source

Consult current checkout `missing-decisions.yaml` plus locally uncommitted corrections. The publicly read file can lag; do not invent a committed decision status. No new decisions were identified as unavoidable for P26 core physical lowering.

## § 2. MD-070 already open

Existing decision on source-level `RawPtr` ↔ `ref` conversion spelling and proof facts. (Note 2026-10-22: MD-070 has since been decided and applied: `RawPtr[T].AsRef()` / `AsRefMut()`, `raw_pointers.md` § 18A. Its Semantic IR operation remains a capability boundary.) Do not create P26-D1 for it; block only exposed conversion API, not P25-proven internal foreign call adapters.

## § 3. MD-019 already open

Existing explicit layout spelling decision. P26 consumes already resolved P24 layout and cannot introduce new source packing or alignment attributes.

## § 4. MD-073 and MD-109 already open

These concern string/char materialization APIs, not generic reference validity. Do not block safe scalar reference lowering because of unrelated string source decisions.

## § 5. Applied decisions

MD-061 reborrow of referent through safe reference holder; recent corrected slice/string decisions such as MD-050, MD-071 and MD-072 stay applied. Do not reopen.

## § 6. New P26-D#

**None created.** If implementation reveals a genuinely new normative fork, create one standalone `P26-D1.md`, then sequential IDs, with authority, alternatives, recommendation, impact, and explicit no-implementation gate, for later user migration to MD-###.

## § 7. Additional open gate (installation note 2026-10-20)

MD-108 (borrowed interface view representation) is open and gates interface
and closure descriptors (package book § 27). New forks follow the project
practice: a simple choice is asked of the owner directly, a substantial one
becomes an `MD-###` entry in `missing-decisions.yaml`.
