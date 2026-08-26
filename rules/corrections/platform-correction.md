# Correction: `@address` always requires canonical region validation

- **Status:** Normative correction
- **Created:** 2026-08-26
- **Last updated:** 2026-08-26
- **Document revision:** 1
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `fc8d632`
- **Canonical path:** `rules/platform/correction.md`
- **Corrects:** `rules/platform/fixed-address-bindings.md`

## Correction

`@address` is always a compiler-validated fixed-storage binding.

A compile-time-known numeric address is not by itself sufficient to make an
`@address` declaration valid.

Every `@address` binding must resolve to an applicable canonical address-region
or storage contract supplied by the active `CompilationPlan`,
`MemoryEnvironment`, `DeviceModel`, or another canonical target-owned source.

The complete bound extent must satisfy all applicable region constraints,
including as relevant:

```text
address domain / address space
region extent
availability
read/write permissions
alignment
physical representation
allowed access widths
storage/device compatibility
```

If no applicable region can be resolved, the `@address` declaration is invalid.

`unsafe` does not waive this validation.

Therefore an address that is dynamic or otherwise cannot satisfy `@address`
region validation must use the applicable `RawPtr` or checked low-level
mechanism instead of an unverified `@address` declaration.

This rule applies to Hosted, RTOS, and BareMetal targets.

Hosted targets may explicitly define valid mapped hardware/device regions, for
example for drivers, GPIO, PCIe, FPGA, or similar access. The correction does
not prohibit `@address` on Hosted targets.

It prohibits treating arbitrary process virtual addresses as valid merely
because their numeric spelling is compile-time known.

## Superseding rule

Any wording in `fixed-address-bindings.md` that can be read as permitting
`unsafe` to make an otherwise unverifiable `@address` declaration valid is
superseded by this correction.

Dynamic/unverifiable access remains possible through the applicable explicit
low-level pointer or checked access mechanism.
