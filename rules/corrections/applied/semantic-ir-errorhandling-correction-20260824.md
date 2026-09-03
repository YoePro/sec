# Semantic IR correction — errorhandling revision 2

- **Status:** Applied normative correction
- **Applied:** 2026-08-24
- **Created:** 2026-08-24
- **Last updated:** 2026-08-24
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/compiler/semantic_ir.md`

---

## Correction

Extend the semantic error-handling model so lowering does not depend on the old
exhaustive Result-only try representation.

Semantic IR must preserve or explicitly represent:

```text
compiler-known error root identity
concrete error identity after widening to error
Result precise/open error channel
Option try success/None short-circuit
language-defined fallible operation try
compiler-internal failure set
partial local handlers
unmatched propagation edge
source handler order
where guards and guard-false continuation
handler binding type and copy/move/borrow action
guard-success move commit
recovery-value merge
fallible assignment success/failure edge
return-try forwarding
Result consuming Ok/Err projection
Result borrowed OkRef/ErrRef projection
cleanup/destruction on every propagation/handler edge
remaining panic effects outside try conversion
```

### Existing Package 10 behavior

Existing Semantic IR support for source-ordered local Result handlers remains a
useful implementation base, but revision 2 removes the source-language explicit
Ok handler and exhaustive-handler requirements.

Do not preserve obsolete explicit-Ok handler semantics merely because the
current package schema contains that state.

### Error widening

A value lowered through the open `error` root must retain a stable semantic
concrete-type discriminator and all payload/destruction information needed by
error-specific narrowing.
The exact physical encoding is deferred to ABI/layout lowering.

### No hidden unions

The compiler-internal failure set is not a source Result error type and must not
be serialized or exposed as an inferred anonymous public error union.

## Cross-reference

```text
rules/errors/errorhandling.md
rules/errors/runtime_checks.md
rules/control-flow/flowcontrol_match.md
```
