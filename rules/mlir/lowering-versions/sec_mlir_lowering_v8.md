# Sec MLIR Lowering

## Status

Normative lowering specification.

Current lowering specification version: `8`

Version 8 defines lowering of resolved variant-oriented `match` into explicit
Sec MLIR CFG.

Physical enum/union/Result representation remains deferred.

---

# 1. Input requirement

Match lowering consumes:

```text
verified Semantic IR
Sema-resolved MatchPlan provenance
schema-v8-compatible enum/union/Result/Option types
```

It does not consume AST for semantic decisions.

---

# 2. Core lowering form

Lower match to:

```text
one subject SSA value
source-order pattern-test blocks
optional guard blocks
arm body blocks
expression merge or statement continuation
synthesized unreachable residual when required
```

Use standard `cf`.

---

# 3. Subject once

Materialize the Semantic IR subject exactly once.

Every emitted pattern operation uses the same lowered subject SSA value.

---

# 4. Enum pattern

Lower to:

```text
sec.enum.constant
sec.enum.cmp eq
cf.cond_br
```

Do not lower enum to its integer representation in this pass.

Alias behavior follows numeric enum comparison.

---

# 5. Union pattern

Lower to:

```text
sec.union.is_variant
cf.cond_br
```

On matching path, if copy-trivial binding exists:

```text
sec.union.unwrap_payload
```

Payload discard requires no projection.

---

# 6. Result pattern

Use:

```text
sec.result.is_err
cf.cond_br
sec.result.unwrap_ok / unwrap_err
```

Branch orientation depends on Ok versus Err pattern.

---

# 7. Option pattern

Use ordinary union variant test/projection for:

```text
Some
None
```

---

# 8. Guard

After pattern success:

```text
lower guard expression
require i1
cf.cond_br guard, body, next-pattern
```

Do not speculate guard evaluation.

---

# 9. Catch-all

Unguarded `_`:

```text
direct branch to body
```

Guarded `_`:

```text
evaluate guard
true -> body
false -> next pattern
```

---

# 10. Expression merge

Create one block argument of resolved match result type.

Each continuing value arm branches with exactly one value.

Terminating arms have no merge edge.

---

# 11. Statement continuation

Continuing statement arms branch to one continuation block.

If all exhaustive arms terminate, omit it.

---

# 12. Exhaustive residual

If a final semantic pattern test is emitted and its false edge is impossible
under the resolved exhaustive plan:

```text
false -> sec.unreachable
```

with:

```text
sec.synthesized = true
reason = "exhaustive-match-fallthrough"
```

---

# 13. Why not erase the final test

Do not skip a final union variant test merely because Sema says it is the only
remaining variant when payload projection requires a proven matching edge.

Keeping the test preserves the union guard invariant.

The impossible false edge is represented explicitly.

---

# 14. Match provenance emission

Attach transient:

```text
sec.match_id
sec.match_arm_index
sec.match_stage
sec.match_pattern_kind
```

to generated control-flow operations as specified by schema v8.

Use deterministic function-local IDs.

---

# 15. Verification

Run:

```text
normal MLIR verifier
sec-verify-result-guards when Result projections exist
sec-verify-union-guards when union projections exist
sec-verify-match-cfg
```

plus other package verifiers applicable to operations inside guards/bodies.

---

# 16. No physical representation lowering

Do not lower:

```text
!sec.enum
!sec.union
!sec.result
```

inside match lowering.

No tag extraction, integer enum erasure or Result tag layout.

---

# 17. No match switch optimization

Do not lower source match directly to:

```text
cf.switch
```

in P12.

Source-order guards and binding scopes are easier to verify in explicit CFG.

A future optimization may combine pure unguarded cases after proof.

---

# 18. Binding action gate

P12 lowering supports:

```text
none
discard
copy-trivial
```

Reject:

```text
borrow-shared
borrow-mutable
move
copy-semantic
conditional copy
```

until ownership/reference lowering exists.

---

# 19. Struct-like payload boundary

Do not synthesize a match-only aggregate representation.

Struct-like whole-payload binding remains unsupported until canonical struct
Semantic IR/MLIR exists.

---

# 20. Statement ignored-result boundary

Do not represent an owned/must-use arm result by leaving an SSA value unused.

Only void or semantically obligation-free immediate ignored values are supported
by the P12 statement path.

---

# 21. Optimization independence

Correctness does not depend on:

```text
CSE
canonicalize
SCCP
DCE
switch formation
```

Emit correct source-order CFG first.

---

# 22. Completion

Lowering version 8 is implemented when:

```text
subject-once invariant holds
enum/union/Result/Option patterns lower
guards preserve source order
first-match semantics hold
expression merge works
statement continuation works
exhaustive residual is explicit
payload projection remains guarded
ownership-sensitive bindings reject
no physical variant layout is chosen
```
