# P27 Decision Index

- **Status:** Implementation specification; not implementation evidence
- **Created:** 2026-10-20
- **Last updated:** 2026-10-20
- **Revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/mlir/packages/sec-mlir-dialect_package27_decision-index.md` (installed 2026-10-20)
- **Replaces:** None; additive amendment. Preserve all existing paragraph references.
- **Package:** `SEC-MLIR-P27`

---

## § 1. New P27 decisions

**None identified in the reviewed public rulebooks.** P27 is a compiler-implementation boundary and must not reopen existing language/architecture choices. A new genuinely undecided fork found during implementation receives a separate `P27-D1.md` (then `P27-D2.md`, etc.) with explicit status, source evidence, alternatives, and affected TODO IDs.

## § 2. Existing MD-101 is not a new P27-D

The public `missing-decisions.yaml` records MD-101 for one-track LLVM behavior with provisional unresolved implementation details: trap/error reason encoding, hosted entry adapter, temporary internal scalar convention, capability gates and tool resolution. Treat it as existing pending work; it is not licensed as a universal permanent ABI/panic contract.

## § 3. Other existing gates

- `MD-070`: public raw-pointer/safe-reference conversions.
- `MD-019`: explicit layout syntax.
- `MD-021`: target C binding environment.
- `MD-045`: external callable and stack contracts.

These are dependencies only for their relevant feature rows; they do not block the S0 scalar/CFG bridge.

## § 4. Local status may be newer

Repository `missing-decisions.yaml` may lag the user's uncommitted migration of P#-D# items. Compare live local file before opening new decisions; never allocate MD-### IDs here.
