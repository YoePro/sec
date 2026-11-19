# MD-048 side note — Numeric execution environment and isolation (deferred)

- **Document type:** Design side note; not a normative amendment to `rules/types/units.md`
- **Status:** Deferred — preserve confirmed decisions; cross-cutting isolation contract **not approved**
- **Date:** 2026-11-19
- **Revision:** 1
- **Suggested stage:** Sec 0.1.1 or later; exact owner/milestone not assigned
- **Origin:** MD-048 discussion of numerical strategies, FTZ/DAZ and target capabilities
- **Tracking:** `MD-144` in `missing-decisions.yaml`; the unit-conversion consequences are normative in `rules/types/units.md` §§ 22.5(8)–(10) (MD-048, integrated 2026-11-19). This note lives in the repository root because it is not a normative correction.
- **Priority:** Finish MD-048 affine and logarithmic **unit conversion** work first

## § 1. Boundary

**§ 1(1).** MD-048 specifies linear, affine and logarithmic unit conversions, `Reference`, conversion errors, result precision, and conversion-specific numeric strategy consequences. A language-wide model for processor floating-point state, function-call boundaries, threading, tasks, and FFI is **out of scope** and must not block MD-048.

**§ 1(2).** The user requested saving the existing conclusions as a side note for Sec 0.1.1 or later, without reopening them now or allowing the scope to expand indefinitely. This note is **not** approval of a particular implementation for floating-point-environment isolation.

## § 2. Approved principles to preserve

**§ 2(1).** `Strict` (default), `BoundedApproximation` (error bound on the delivered result), and opt-in `FastMath` (no general error bound) are distinct strategies. Selection exists at project, function, and lexical-block level with precedence **block > function > project > Strict**. A callee does not dynamically inherit a caller's strategy. Source spelling is not yet ratified.

**§ 2(2).** `FastMath` by itself does not assume NaN or infinity cannot occur. Such assumptions require separate explicit selection or compiler proof. Unit/Kind/type checks and required checked-conversion behavior are not implicitly discarded.

**§ 2(3).** FTZ (subnormal *results*) and DAZ (subnormal *operands*) are separately explicit opt-ins. Enabling one does not enable the other, and neither follows automatically from `FastMath` or `BoundedApproximation`. `Strict` retains its prescribed subnormal behavior. Under `BoundedApproximation`, the complete delivered-result error bound must still hold.

**§ 2(4).** Capability preferences support conceptual **Allow** (conforming alternative implementation may be used, with informative diagnostics when a specifically requested feature is unavailable) and **Demand** (mandatory requirement, compile-time error when unsatisfied). The normal path is automatic conforming implementation selection. Programmer-directed fine-grained requirements are optional.

**§ 2(5).** A named hardware Demand such as AVX2 fails compilation unless the *declared target* guarantees that capability. Requiring a processor feature does not force emission of a particular machine instruction. Exact instruction-level control belongs to explicitly selected target-gated intrinsics. Public syntax, precise intrinsic API and safety preconditions are pending; this note does not assign them to MD-048.

## § 3. Cross-cutting concerns noticed, but not decided

**§ 3(1).** Some targets expose FTZ/DAZ and related behavior as mutable execution state with a scope wider than a lexical conversion. Consequently compiler IR flags or a naive save/restore around one function do not necessarily establish isolation under optimization.

**§ 3(2).** A proposal to guarantee numerical-environment isolation across arbitrary function and block boundaries was discussed on 2026-11-19, but the user requested prior comparative research and then corrected the scope. **No such language-wide guarantee was ratified.**

**§ 3(3).** When this issue is resumed, review optimizer motion/inlining, actual hardware-state scope, explicit state transitions, calls with different effective numeric strategies, exceptional exits, tasks/threads, and foreign code **in their respective rulebooks**. FFI was explicitly excluded from the current MD-048 work; no FFI policy is created here.

## § 4. What remains in MD-048

**§ 4(1).** A unit conversion must satisfy its specified mathematical transform, domain and error checks, final result accuracy, and target compatibility. The compiler cannot silently claim an allowed result if its conversion-specific contract was not met.

**§ 4(2).** MD-048 need not prescribe register saving, state restoration, task scheduling, FFI conventions, or general numeric execution-state isolation. This side note exists to preserve the topic for the appropriate future workstream without delaying the unit-conversion specification.

## § 5. Resumption rule

**§ 5(1).** Resume only after a dedicated general numerical-execution/platform rulebook or Sec 0.1.x implementation stage owns this work. Carry the approved decisions from §2 forward; independently decide open questions from §3. The deferred note is not an MD-048 completion gate.
