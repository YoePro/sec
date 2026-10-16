# P25 Decision Index — Existing MD Gates and New-Decision Policy

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec-mlir-dialect_package25_decision-index.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P25`

---

## § 1. Decision registry source

The public `https://raw.githubusercontent.com/YoePro/sec/main/missing-decisions.yaml` reads `updated: "2026-10-15"` as of this package. The user reports newer uncommitted information may exist. Use this only for duplicate checking, not as a complete assertion about the local decision database.

## § 2. MD-019: explicit layout syntax

Existing blocking decision about syntax of explicit packing/alignment/offset/endianness/representation. P25 supports only resolved layouts/foreign C defaults until syntax is accepted. No P25-D1 duplicate.

## § 3. MD-021: target C binding environment producer

Existing blocking decision about serialization and selection of target-provided `c::` binding declarations. P25 can classify compiler-known fundamental C family and already-resolved target binding metadata, but cannot invent `c::stddef` metadata file formats or silent defaults. No duplicate.

## § 4. MD-045: external callable/stack/reentry contract producers

Existing localized decision about declaration/metadata producer for imported stack, ABI effects, no-reentry and trust. P25 consumes verified facts where present and uses conservative unknown/rejection elsewhere. No new annotation invented.

## § 5. MD-052: privileged core bodies

Existing blocking syntax question for privileged low-level compiler/core bodies. It does not block the P25 target-neutral ABI model; do not use it as an invented FFI adapter syntax.

## § 6. Applied decisions are not open

MD-020 fixed lowercase `c::`; MD-023 covers foreign int/uint and wide scalar ABI position validation; MD-024 describes foreign unsafe/null distinctions. These applied decisions must be implemented, not asked anew. Preserve `extern "Sec"`/`"C"`/`"system"` and no general C-export for Sec 0.1.

## § 7. New P25-D# files

**None**. `rules/platform/abi.md` already makes the core physical ABI decisions, and `rules/platform/ffi.md` makes the source FFI contract choices. Unknown target-specific ABI classification is an unavailable ABIModel implementation/capability, not a license to introduce source language semantics. If a genuinely new fork emerges during actual implementation, create **one** `P25-D1.md`, then `P25-D2.md`, etc., rather than editing this index to hide it. User may map those into `missing-decisions.yaml`/MD-###.
