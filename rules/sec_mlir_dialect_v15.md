# Sec MLIR Dialect

## Status

Normative detailed representation specification.

Current Sec MLIR dialect schema version: `15`

Schema version 15 introduces high-level Arena and allocation-context semantics.

It does not define a universal Arena descriptor or allocator ABI.

---

# 1. Version history

```text
v1   dialect foundation
v2   Semantic IR bridge
v3   scalar/target coverage
v4   checked integer operations
v5   typed arithmetic failure and Result construction
v6   Result branching/local try handlers
v7   enum and union values
v8   verified match CFG
v9   struct semantic values
v10  fixed-array semantic values
v11  places and direct references
v12  slice semantic values
v13  ownership transfer and deterministic destruction
v14  owning dynamic arrays and storage transitions
v15  allocation context and Arena semantics
```

Compiler-generated v15 modules carry:

```mlir
sec.dialect_version = 15 : i32
```

---

# 2. `!sec.arena`

High-level Arena builtin type.

Properties:

```text
move-only
non-trivially destructible
owns ArenaDomain
backing policy is value metadata
```

No backing/growth/profile type parameters.

---

# 3. `!sec.alloc_context`

Compiler-internal allocation context.

Not source-visible.

Not ordinary user-storable data.

---

# 4. Arena metadata

Arena-producing/mutating ops carry/reference typed metadata equivalent to:

```text
sec.arena_domain
sec.arena_state_before
sec.arena_state_after
sec.arena_backing_kind
sec.arena_growth_kind
sec.provider_plan
sec.capacity_plan
sec.validity_epoch_dependency
sec.storage_identity
sec.memory_space
```

---

# 5. Backing kinds

Canonical:

```text
owned
borrowed
static
target-provided
```

---

# 6. Growth kinds

Canonical:

```text
fixed
growable
```

Growable semantics guarantee stable existing allocation addresses.

---

# 7. `sec.arena.create_borrowed`

Operand:

```text
!sec.slice_mut<byte>
```

Result:

```text
!sec.arena
```

Infallible.

Fresh ArenaDomain.

---

# 8. Borrowed creation verifier

Require:

```text
mutable byte slice
contiguous source
fresh ArenaDomain
fixed growth policy
borrowed backing facts
parent storage dependency
initial live epoch
```

---

# 9. `sec.arena.create_owned_fixed`

Operand:

```text
capacity: !sec.uint or plan-resolved uint
```

Results:

```text
!sec.arena
i1
!sec.core_error<"core::AllocationError">
```

Result order:

```text
arena
failed
error
```

---

# 10. Owned fixed create semantics

`failed == false`:

```text
arena valid
fresh domain established
error ignored
```

`failed == true`:

```text
arena not consumed as valid owner
no domain established
error consumed
```

---

# 11. `sec.arena.create_growable`

Checked result shape identical to owned fixed.

Growth policy must preserve existing allocation addresses.

---

# 12. `sec.arena.new`

Operand:

```text
!sec.place<!sec.arena,"rw">
```

Result:

```text
!sec.ref_mut<T>
i1 failed
AllocationError
```

T carried as result type/type parameter metadata.

---

# 13. `sec.arena.alloc`

Operands:

```text
!sec.place<!sec.arena,"rw">
count
```

Result:

```text
!sec.slice_mut<T>
i1 failed
AllocationError
```

---

# 14. New/Alloc verifier

Require:

```text
T sized
complete target-plan layout available
valid alignment
valid canonical default
trivially destructible
count uint for Alloc
same ArenaDomain state on failure
state version advances only on success
epoch preserved
```

---

# 15. `sec.arena.reset`

Operand:

```text
!sec.place<!sec.arena,"rw">
```

No result.

Required metadata:

```text
ArenaDomain
state before/after
epoch strategy
```

Semantically includes `AdvanceEpoch`.

---

# 16. `sec.arena.release`

Operand:

```text
!sec.arena
```

No result.

Consumes owner.

Semantically includes `EndDomain` and backing release/end-borrow behavior.

---

# 17. `sec.arena.destroy`

Operand:

```text
!sec.arena
```

No result.

Consumes owner under implicit P17 destruction.

Terminal Release semantics.

---

# 18. `sec.alloc_context.from_arena`

Input is resolved Arena mutable authority.

Result:

```text
!sec.alloc_context
```

Only for explicit Arena-selected allocation contract.

---

# 19. `sec.alloc_context.target`

Produces a target-provided context at an allowed Sec entry root.

Required CompilationPlan provider metadata.

---

# 20. `sec.alloc_context.compiler_local`

Produces a compiler-managed context after proof.

Required metadata includes:

```text
backing strategy
lifetime proof
non-escape proof
failure policy
```

---

# 21. Hidden function context argument

A Sec-internal `func.func` with ambient requirement carries one hidden argument:

```text
!sec.alloc_context
```

with:

```text
sec.hidden_allocation_context = true
```

This argument is compiler ABI only.

---

# 22. Source function identity

The hidden argument:

```text
does not participate in source overload resolution
does not appear in source function type
does not appear in user reflection/module source signature
```

Module compiler metadata separately records the requirement.

---

# 23. Direct calls

A direct Sec call to a context-requiring function passes exactly one compatible
context operand.

Calls to functions without the requirement do not invent one.

---

# 24. Foreign calls/exports

Ordinary foreign ABI operations/signatures reject hidden allocation-context
arguments unless the call/wrapper contract is Sec-aware.

---

# 25. Arena effects

Schema v15 Arena ops implement a Sec-specific ordered Arena effect interface.

Conceptual effects:

```text
Create(domain)
Allocate(domain)
Reset(domain)
Release(domain)
```

Provider effects remain additionally visible.

---

# 26. Arena effect resource

Conceptual custom resource:

```text
ArenaResource(ArenaDomain)
```

Different ArenaDomains are independent when storage/provider facts permit.

---

# 27. CSE

Arena allocation/create operations are not CSE-safe.

Zero-element allocation may be eliminated only after proving its result and all
observable Arena state/effect consequences are unused/equivalent.

---

# 28. Reset/Release barriers

Optimization may not move dependent Arena operations across:

```text
reset
release
```

unless proof establishes semantic equivalence.

---

# 29. P18 storage transitions

Schema v15 reuses:

```text
sec.storage.establish_domain
sec.storage.advance_epoch
sec.storage.end_domain
sec.storage.reclaim
```

Arena ops may either contain equivalent typed effects or be canonically expanded
to these operations in a later pass.

Do not duplicate domain state.

---

# 30. P17 destruction plan

Add/use:

```text
DestroyArena
```

as a valid destruction-plan kind.

It invokes `sec.arena.destroy`.

---

# 31. P15/P16 outputs

`sec.arena.new` returns normal:

```text
!sec.ref_mut<T>
```

`sec.arena.alloc` returns normal:

```text
!sec.slice_mut<T>
```

No Arena-specific reference type.

---

# 32. Domain/epoch propagation

Returned reference/slice metadata identifies:

```text
ArenaDomain
current epoch
allocation bounds
storage identity
```

Ordinary allocation does not change the epoch.

---

# 33. Arena verifier

Register:

```text
--sec-verify-arenas
```

---

# 34. Allocation-context verifier

Register:

```text
--sec-verify-allocation-contexts
```

---

# 35. Arena verifier checks

```text
move-only classification
fresh domain creation
backing/growth policy
safe T restrictions
state-version flow
atomic failure
ordinary allocation epoch preservation
reset epoch advance
release domain end
owner consumption
no double terminal action
nested dependency
result ref/slice facts
```

---

# 36. Allocation-context verifier checks

```text
one hidden context when required
none when not required
call propagation
origin selection
no lexical guessing
compiler-local proof metadata
target root validity
spawn/thread no inheritance fact
foreign boundary
```

---

# 37. Existing verifier integration

Extend:

```text
sec-verify-ownership
sec-verify-destruction-plans
sec-verify-cleanups
sec-verify-places
sec-verify-reference-guards
sec-verify-slice-guards
sec-verify-storage-transitions
```

for Arena facts where relevant.

---

# 38. No physical Arena type

Schema v15 does not define:

```text
base pointer
capacity field
cursor field
segment list
provider pointer
epoch storage field
```

---

# 39. No universal provider ABI

Schema v15 keeps provider plan abstract.

No direct `malloc`, OS allocator or runtime helper signature is canonical here.

---

# 40. Schema-v15 completion

Schema v15 is complete when:

```text
Arena/context types parse/print/verify
constructors and New/Alloc verify
Reset/Release verify
hidden context propagation verifies
Arena effects/resources verify
P15/P16 result types integrate
P17 cleanup integrates
P18 storage transitions integrate
schema-v14 regressions remain valid
no physical descriptor/provider ABI is selected
```
