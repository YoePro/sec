# Correction: static storage and physical placement boundary

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-13
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Targets:** `rules/declarations/static.md`, `rules/foundations/attributes.md`

## Boundary rule

`static` defines storage lifetime and type/program association.

It does not by itself define:

- linker section placement;
- absolute address;
- MMIO binding;
- memory space;
- target-specific physical placement.

The attribute system in Sec 0.1 is a closed compiler-known set. Do not use conceptual examples such as `@section(...)` as if they were accepted language syntax unless `@section` is separately added to the canonical attribute rulebook with complete semantics.

Where physical placement is required, use only a placement mechanism defined by the canonical attribute, storage, ABI, or platform rules.

This correction removes the stale wording that described a "future attribute system"; the attribute system already exists canonically, while `@section` itself is not currently part of its closed Sec 0.1 set.
