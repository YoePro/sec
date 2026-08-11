# Sec MLIR Lowering

## Status

Normative lowering specification.

Current lowering specification version: `15`

Version 15 lowers canonical Semantic IR Arena/allocation-context semantics to
high-level schema-v15 Sec MLIR.

It stops before physical Arena/provider/LLVM lowering.

---

# 1. Arena type mapping

Semantic IR Arena maps to:

```text
!sec.arena
```

No backing-specific MLIR type split.

---

# 2. Allocation context mapping

Semantic IR allocation context maps to:

```text
!sec.alloc_context
```

for internal high-level Sec MLIR.

---

# 3. Borrowed Arena creation

Lower:

```text
ArenaCreateBorrowed
```

to:

```text
sec.arena.create_borrowed
```

using the existing mutable byte-slice value.

No allocation.

---

# 4. Owned fixed creation

Lower resolved owned fixed construction to:

```text
sec.arena.create_owned_fixed
```

Preserve:

```text
provider plan
capacity
fresh domain
AllocationError
atomic failure
```

---

# 5. Growable creation

Lower high-level growable construction to:

```text
sec.arena.create_growable
```

when source/plan availability permits.

The operation remains abstract over physical stable-segment strategy.

---

# 6. New[T]

Lower:

```text
ArenaNewOp
```

to:

```text
sec.arena.new
```

on a writable Arena Place.

Do not materialize raw backing pointer.

---

# 7. Alloc[T]

Lower:

```text
ArenaAllocOp
```

to:

```text
sec.arena.alloc
```

on a writable Arena Place and count.

Result is the existing high-level mutable slice.

---

# 8. Checked allocation CFG

Canonical create/New/Alloc flow:

```text
candidate, failed, error = arena operation
cf.cond_br failed, failure, success

failure:
    consume AllocationError through existing Result/try/local-handler flow

success:
    consume candidate
```

No partial candidate use on failure.

---

# 9. Proven infallible allocation

When Sema/plan proves the operation cannot fail:

```text
the failure branch may be eliminated
```

Source Result type metadata remains unchanged.

---

# 10. Arena state update

Successful New/Alloc changes Arena state version.

Failure leaves the existing Arena Place state unchanged.

High-level MLIR effect ordering preserves this even without physical cursor
fields.

---

# 11. Ordinary allocation and epoch

Do not emit:

```text
sec.storage.advance_epoch
```

for ordinary New/Alloc or stable growable segment acquisition.

Returned refs/slices use current epoch.

---

# 12. Reset lowering

Lower to:

```text
sec.arena.reset
```

with the resolved dependency proof metadata.

The op semantically performs:

```text
AdvanceEpoch
cursor/state reset
backing retention
```

No physical zeroing requirement.

---

# 13. Release lowering

Explicit source Release:

```text
move/consume Arena owner through P17
sec.arena.release
cancel P17 implicit Arena cleanup
```

No later source use.

---

# 14. Implicit destruction lowering

P17 `DestroyArena` lowers to:

```text
sec.arena.destroy
```

which performs terminal Release semantics.

No double release.

---

# 15. Borrowed Release

Borrowed Arena destruction/release:

```text
ends child ArenaDomain
ends retained backing borrow
does not deallocate backing owner
```

---

# 16. Owned Release

Owned fixed/growable release:

```text
ends ArenaDomain
returns owned segments to resolved provider
```

Physical provider call remains abstract.

---

# 17. Nested Arena

Preserve child-to-parent storage dependency metadata.

Do not erase it before parent Reset/Release verification.

---

# 18. Context-requiring function lowering

For a Sec-internal function summary:

```text
RequiresAllocationContext
```

insert/retain exactly one hidden:

```text
!sec.alloc_context
```

function argument.

Do not alter source overload/type identity.

---

# 19. Direct call lowering

Direct call to a requiring Sec function:

```text
passes current context
```

Direct call to a non-requiring function:

```text
passes none
```

No implicit global lookup at lowering time.

---

# 20. Explicit Arena operation context

When the source operation explicitly selects an Arena:

```text
sec.alloc_context.from_arena
```

may produce the compiler context consumed by a lower allocation-capable helper.

Do not make it ambient beyond the operation/call contract.

---

# 21. Compiler-local context

Lower only after proof.

Keep:

```text
sec.alloc_context.compiler_local
```

high-level.

Physical stack/static/target backing is deferred.

---

# 22. Target context

Entry-root context may lower to:

```text
sec.alloc_context.target
```

with CompilationPlan provider facts.

---

# 23. Foreign boundary

Do not add the hidden context to ordinary foreign call/entry ABI.

Generate/use a Sec wrapper only when already resolved by the ABI plan.

---

# 24. No-context failure

A valid Semantic IR module must never contain an implicit allocation operation
with unresolved/no context.

This is a verifier/frontend error, not a runtime null context.

---

# 25. Arena effects

Preserve source/effect order through:

```text
Create
Allocate
Reset
Release
```

and provider/storage effects.

---

# 26. Capacity demand metadata

Lower no runtime operation solely for Arena-demand summaries.

Carry them as compiler/module metadata as long as profile validation/tooling
needs them.

---

# 27. Strict profile

Before emitting target-dependent lower IR, reject a strict profile whose Arena
demand/provider requirements cannot be satisfied.

Do not silently select another provider.

---

# 28. Hosted profile

Unknown demand may remain runtime/provider-fallible.

Do not rewrite allocation failure into panic.

---

# 29. P15/P16 validity

Arena.New/Alloc outputs retain domain/epoch facts used by direct reference/slice
validity.

Reset/Release remain visible barriers until validity analysis is discharged.

---

# 30. P17 cleanup

Arena cleanup remains in the unified P17 cleanup order.

A deferred Arena-backed use must execute before any registered release that
would invalidate it.

---

# 31. P18 dynamic arrays

P18 allocation-capable dynamic-array operations consume P19 context/provider
facts.

Arena-backed dynamic array storage retains Arena-domain dependency.

---

# 32. P18 storage transition mapping

Arena creation/reset/release use the same canonical transition semantics:

```text
creation -> EstablishDomain
Reset    -> AdvanceEpoch
Release  -> EndDomain
```

Provider reclamation remains separate.

---

# 33. No physical cursor arithmetic

P19 high-level lowering does not emit:

```text
pointer bump
GEP
segment-list traversal
memcpy
malloc/free
```

Those belong to physical Arena/provider lowering.

---

# 34. No physical epoch representation

Do not select:

```text
i64 field
two i32 words
side table
no field
```

in lowering v15.

The CompilationPlan/reference lowering stage owns that choice.

---

# 35. No general concurrency lowering

Do not lower:

```text
spawn Arena context
await dependency completion
thread transfer
```

in P19.

Preserve summaries/facts for the later concurrency package.

---

# 36. Verification pipeline

As applicable:

```text
normal MLIR verification
P13-P18 verifiers
sec-verify-arenas
sec-verify-allocation-contexts
```

Run context verification before any pass erases hidden internal context
arguments.

---

# 37. Optimization independence

Correctness does not depend on:

```text
capacity check elimination
epoch elimination
Arena scalarization
stack lowering
CSE
LICM
DCE
inlining
```

These optimizations occur only after semantic verification.

---

# 38. Legal later optimization

After proof a later pass may:

```text
eliminate runtime capacity checks
eliminate runtime epoch metadata
stack-lower compiler-local Arena
fold fixed offsets
remove redundant Reset before terminal Release
remove unused zero-element allocation
eliminate Arena descriptor entirely
```

while preserving source effects/diagnostics as required.

---

# 39. Forbidden optimization

Never:

```text
CSE distinct allocations
move allocation across Reset
move dependent use across Release
merge different epochs
relocate live Arena allocation
remove required Release
invent hidden provider
```

---

# 40. Completion

Lowering version 15 is implemented when:

```text
Arena remains one high-level move-only value type
ArenaDomain/state/epoch facts survive
borrowed/owned/growable creation lowers
New/Alloc produce canonical ref/slice values
allocation failure remains Result-based and atomic
Reset advances epoch
Release ends domain
P17 implicit destruction performs Release
allocation context is explicit internally and propagated only when required
foreign ABI is protected
ordered Arena effects remain visible
P18 consumes the same context/storage model
no physical Arena/provider representation is selected
```
