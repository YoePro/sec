# Semantic IR Amendment - Allocation Context and Arena

## Status

Normative amendment for:

```text
rules/semantic_ir.txt
```

Package:

```text
SEC-MLIR-P19
```

Repository baseline:

```text
152c772
```

Local predecessors:

```text
SEC-MLIR-P13 through SEC-MLIR-P18
```

This amendment defines canonical target-independent Semantic IR for allocation
contexts and Arena.

---

# 1. Arena type

Semantic IR has one builtin Arena type corresponding to source:

```text
Arena
```

Backing/growth/profile are value facts.

---

# 2. Arena ownership

Arena is MoveOnly.

Move preserves domain/epoch/backing.

No copy operation is legal.

---

# 3. ArenaDomain

Each Arena creation establishes one ArenaDomain identity.

The domain is independent of physical backing address.

---

# 4. State version

Every mutating Arena operation has a semantic state-version transition.

State version is used for ordered analysis.

It is not reference validity epoch.

---

# 5. Backing policy

Arena semantic facts record:

```text
backing kind
growth kind
provider plan
capacity facts
storage classification
reclamation authority
memory space
```

No physical descriptor fields.

---

# 6. Borrowed Arena creation

Arena creation from mutable byte slice:

```text
fresh ArenaDomain
fixed policy
borrowed backing
retained exclusive backing dependency
initial epoch
infallible
```

---

# 7. Owned Arena creation

Owned fixed/growable Arena creation is explicit and fallible.

Failure publishes no live domain.

Success establishes the domain and ownership cleanup.

---

# 8. Typed allocation

Arena New/Alloc operations are explicit.

They consume writable Arena control authority transiently and return safe direct
reference/slice values.

The returned value retains ArenaDomain/epoch/bounds facts.

---

# 9. Allocation atomicity

Failed allocation leaves the prior Arena semantic state unchanged.

No partially initialized value is published.

---

# 10. Allocation epoch

Ordinary allocation preserves current Arena validity epoch.

Repeated allocations remain simultaneously valid.

---

# 11. Reset

Arena Reset:

```text
requires resolved dependency proof
advances epoch
preserves ArenaDomain
retains backing
resets allocation state
```

---

# 12. Release

Arena Release:

```text
consumes owner
ends domain
performs backing-kind/provider release action
```

No later ordinary Arena operation is valid.

---

# 13. Destruction

Arena destruction is a resolved P17 destruction plan implementing terminal
Release semantics.

---

# 14. Nested Arena

Borrowed child Arena retains parent storage/domain/epoch dependency.

Parent invalidation is prohibited while child remains live.

---

# 15. Allocation context

Semantic IR has a compiler-only allocation-context value.

It records one already-resolved allocation context.

It is not source-visible.

---

# 16. Context requirement

Callable metadata distinguishes:

```text
MayAllocate
RequiresAllocationContext
```

These facts are not aliases.

---

# 17. Context propagation

Synchronous direct calls explicitly propagate the context when required.

No lexical Arena discovery occurs in the builder.

---

# 18. Context origins

Canonical origins:

```text
explicit Arena
propagated ambient
compiler-local proven Arena
target-provided
```

No unresolved context reaches lowering.

---

# 19. Foreign boundary

Semantic IR marks functions/calls whose ABI is foreign.

Compiler-hidden allocation context may not cross that ABI unless a wrapper or
explicit contract exists.

---

# 20. Ordered Arena effects

Semantic IR Arena ops are ordered by ArenaDomain-aware effects.

Allocation is not pure.

Reset/Release are invalidation barriers.

---

# 21. Capacity demand

Callable/module analysis may retain Arena-demand expressions.

They are compile-time metadata only.

---

# 22. Profile validation

The active CompilationPlan decides:

```text
provider availability
growth availability
capacity-proof strictness
epoch representation
panic cleanup policy
ambient context availability
```

Source Arena semantics remain constant.

---

# 23. Storage transition reuse

Arena creation/reset/release reuse canonical storage transitions:

```text
EstablishDomain
AdvanceEpoch
EndDomain
Reclaim
```

No second Arena transition system.

---

# 24. Reference integration

Arena New/Alloc output uses existing P15/P16 reference and slice semantic types.

No Arena-specific pointer type.

---

# 25. Owning-container integration

P18 owning arrays may allocate backing through a P19 allocation context/Arena.

Container owns elements.

Arena controls backing domain according to the resolved plan.

---

# 26. Verification

Semantic IR verification must prove:

```text
Arena move-only behavior
fresh domain creation
allocation state/epoch distinction
atomic failure
safe T requirements
reset dependency legality
release consumption
nested dependency
context propagation
foreign context boundary
effect ordering
profile availability
```

---

# 27. Deterministic printer

Print high-level Arena semantics deterministically:

```text
ArenaDomain ID
state version
epoch dependency
backing kind
growth kind
provider plan ID
capacity summary
allocation-context origin
ordered effect
```

Do not print assumed physical pointers/cursors.
