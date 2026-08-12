# Semantic IR Amendment - Match CFG

## Status

Normative amendment for:

```text
rules/semantic_ir.txt
```

Package:

```text
SEC-MLIR-P12
```

Repository baseline:

```text
152c772
```

This amendment defines canonical Semantic IR treatment of resolved `match`.

---

# 1. Core representation rule

Semantic IR does not need a monolithic high-level MatchOp.

After successful Sema, match semantics may be represented as explicit verified
CFG using:

```text
subject SSA value
semantic pattern tests
conditional branches
payload projections
guard expressions
arm bodies
merge block arguments
ordinary terminators
synthesized unreachable
```

The source match meaning must remain recoverable through verification/provenance
metadata.

---

# 2. Subject-once invariant

The match subject is evaluated exactly once.

All pattern tests and payload projections refer to the same resulting Semantic
IR ValueID.

Re-evaluating the AST subject for later arms is invalid.

---

# 3. Resolved MatchPlan input

The builder consumes a read-only Sema-resolved MatchPlan containing:

```text
subject kind/type
value/statement context
resolved result type
source-order arms
resolved pattern kind
resolved enum numeric value
resolved union variant index
resolved binding type/action
guard presence
resolved arm flow
exhaustiveness
residual coverage proof
```

Semantic IR does not redo pattern/exhaustiveness analysis.

---

# 4. Match provenance metadata

A function may retain non-executable MatchRecord metadata:

```text
function-local MatchID
subject ValueID
subject/result TypeID
value-context flag
exhaustive flag
arm records
merge block
source location
```

It contains no AST nodes.

It exists for verifier/debugging support.

---

# 5. Pattern ordering

Pattern tests remain in source order.

A pattern false edge reaches the next live source arm.

No pattern/guard after the selected arm may execute.

---

# 6. Guard semantics

A guard is built only on the matched-pattern path.

Pattern bindings are available in the guard.

Guard false reaches the next arm.

Guard true reaches the arm body.

Bindings do not escape the arm scope.

---

# 7. Enum pattern

Enum pattern semantics use the enum's semantic numeric value.

The enum case declaration identity is provenance only.

Aliases with equal numeric values match the same subject value set.

---

# 8. Union pattern

Union patterns use canonical active-variant identity.

Payload projection is explicit and must be guarded.

P12 complete value support is limited to copy-trivial single payloads and
payload discard.

---

# 9. Result pattern

Result patterns use canonical high-level Result discrimination/projection.

Ok and Err projections are valid only on proven matching paths.

---

# 10. Option pattern

Concrete Option uses the canonical union representation.

Some payload projection follows ordinary union projection rules.

---

# 11. Match expression

A value-producing match uses one merge block argument of the resolved match
result type.

Continuing value arms branch to the merge with exactly one compatible value.

Returning/terminating arms do not reach the merge.

---

# 12. Match statement

Continuing arms branch to a continuation block.

Returning/terminating arms do not.

If every exhaustive arm terminates, no continuation is required.

---

# 13. Synthesized UnreachableOp

Semantic IR adds:

```text
UnreachableOp
```

as a no-successor terminator for compiler-proven impossible paths.

Required metadata:

```text
synthesized = true
reason
location
```

P12 canonical reason:

```text
exhaustive-match-fallthrough
```

This is not source panic or user-level undefined behavior.

---

# 14. Exhaustive residual

When the builder materializes a final pattern test whose false edge is
semantically impossible due to Sema-proven exhaustiveness, that false edge ends
in UnreachableOp.

This allows the final variant/payload projection to remain protected by an
actual matching condition.

---

# 15. Binding action

Semantic IR MatchPlan records the resolved binding action.

P12 executable support accepts:

```text
none
discard
copy-trivial
```

Other valid source actions remain explicit unsupported until ownership/reference
IR exists.

The builder must not reinterpret a binding read as an implicit move/copy.

---

# 16. Verification

Semantic IR verifier must check:

```text
subject once
same subject ValueID across tests
source-order arm chain
guard placement
guard bool type
binding/projection type
binding action support
body cannot continue to later arms
merge arity/type
return/terminate edge separation
exhaustive residual unreachable
```

---

# 17. Enum coverage verification

For enum MatchPlan:

```text
numeric coverage keys use arbitrary precision
unguarded duplicate numeric coverage is invalid
guarded duplicate value classes remain possible
exhaustive flag follows complete runtime-domain coverage
```

---

# 18. No hidden discard

An unused arm result that carries destruction/must-use semantics must not be
represented merely by an unused SSA value.

Until DiscardValue Semantic IR exists, such statement-arm cases remain
unsupported by the P12 new IR path.

---

# 19. No hidden ownership

Non-trivial payload extraction remains unrepresented.

Do not emit a normal SSA copy for:

```text
move
semantic copy
conditional copy
mutable borrow
shared borrow
```

until their canonical Semantic IR operations exist.

---

# 20. Printer

Semantic IR printer should make match CFG understandable through deterministic
block order and optional comments/metadata such as:

```text
match #1 arm 0 pattern
match #1 arm 0 guard
match #1 arm 0 body
match #1 merge
```

Such labels are presentation only.

They are not semantic identifiers.
