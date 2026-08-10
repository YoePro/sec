# Sec MLIR Lowering

## Status

Normative lowering specification.

Current lowering specification version: `13`

Version 13 lowers canonical Semantic IR ownership/destruction semantics to
high-level schema-v13 Sec MLIR.

It stops before physical destructor ABI, allocator lowering and LLVM.

---

# 1. Ownership lowering principle

Lower exactly the Sema/Semantic-IR-resolved ownership action.

Do not reinterpret value use based on MLIR type or use count.

---

# 2. Copy

Trivial copy:

```text
sec.own.copy
```

Infallible semantic copy:

```text
sec.own.semantic_copy
```

Fallible clone remains ordinary call/Result flow.

---

# 3. Move

Whole SSA move:

```text
sec.own.move
```

Move from storage/subobject:

```text
sec.own.move_from_place
```

No source destruction.

---

# 4. Initialization

Initialization/reinitialization:

```text
sec.own.initialize_place
```

No old destruction.

---

# 5. Replacement

After RHS is fully prepared:

```text
sec.own.replace_place
```

Old destruction is semantically part of the explicit ownership operation.

New cleanup responsibility starts at replacement.

---

# 6. Transactional fallible replacement

Keep old destination untouched on the RHS failure branch.

Emit replacement only on success.

---

# 7. Discard

Explicit/implicit discard lowers to:

```text
sec.own.discard
```

with provenance.

Non-trivial destruction is not represented as unused SSA.

---

# 8. Destruction

Semantic IR destroy lowers to:

```text
sec.own.destroy_value
sec.own.destroy_place
```

using resolved destruction-plan identity.

Do not choose physical destructor implementation yet.

---

# 9. Struct destruction plan

Preserve reverse declaration field order.

Skip moved/uninitialized fields.

No property destruction.

---

# 10. Array destruction plan

Preserve reverse element order.

Keep compact representation for large arrays.

A later physical lowering may generate a loop.

---

# 11. Variant destruction plan

Union/Result/Option destruction remains active-variant aware.

No inactive payload access.

---

# 12. Cleanup registration

Lower semantic cleanup registration/cancellation to:

```text
sec.cleanup.track_owned
sec.cleanup.cancel
```

Keep registration ordinal exact.

---

# 13. Defer registration

Lower executed defer statement to:

```text
sec.cleanup.defer_register
```

with a static body region and capture Places.

Do not copy captured values at registration.

---

# 14. Defer capture storage

If a defer requires a binding to outlive its ordinary physical lexical storage,
P17 high-level lowering preserves the semantic Place.

Physical lifetime extension is deferred.

Do not duplicate ownership merely to keep the binding alive.

---

# 15. Scope cleanup

Lower Sema cleanup-exit plan to:

```text
sec.cleanup.run_scope
```

before the control-flow edge that leaves the scopes.

---

# 16. Function cleanup

After return/error ownership transfer, emit:

```text
sec.cleanup.run_function
```

then the final return operation.

---

# 17. Return ordering

Canonical lowering:

```text
evaluate return expression
perform ownership transfer
cancel local cleanup responsibility
run function cleanup
func.return transferred value
```

---

# 18. Error propagation ordering

Canonical:

```text
prepare Err/result return value
transfer ownership out of local cleanup
run function cleanup
return Err
```

---

# 19. Break/continue

Insert:

```text
sec.cleanup.run_scope
```

for the lexical scopes exited.

Do not run function-scoped defer registrations.

---

# 20. Match/switch branch cleanup

Run branch-local automatic cleanup before exiting the branch when ownership was
not transferred to merge/return/outer storage.

---

# 21. Panic

Do not insert:

```text
sec.cleanup.run_function
```

merely because a `sec.fail.*` endpoint is reached.

No unwinding.

---

# 22. Constructor action lowering

P13/P14/P11 constructor operands preserve:

```text
construct-direct
copy-trivial
copy-semantic-infallible
move
```

No hidden ownership action.

---

# 23. P12 move-only match binding

On proven payload path:

```text
payload Place
sec.own.move_from_place
```

then bind result.

Container partial state remains verifier-visible.

---

# 24. P13 field assignment

For non-trivial owned stored field:

```text
evaluate replacement
resolve ownership
field Place
sec.own.replace_place
```

Do not whole-struct copy merely to replace one owned field.

---

# 25. P14 fixed-array whole ownership

Whole fixed-array move/destruction is supported.

Runtime-indexed element move-out remains unsupported.

---

# 26. Reference/slice lifetime end

Discard/destruction of reference/slice value coordinates with:

```text
sec.ref.end_borrow
```

as required by P15/P16 resolved lifetime facts.

No referent destruction.

---

# 27. Calls

Call-site ownership metadata controls:

```text
copy
semantic copy
move
borrow
```

Consuming arguments lose caller cleanup responsibility before control returns.

---

# 28. Branches

Owned values crossing block edges use explicit ownership metadata.

Block merge owns the selected incoming value.

---

# 29. Destruction-plan expansion boundary

P17 does not require `sec.own.destroy_*` to expand to field-by-field standard
MLIR immediately.

The high-level destruction plan remains until the dedicated physical
ownership/cleanup lowering stage.

---

# 30. Dynamic defer boundary

A dynamically repeated defer remains represented by high-level registration.

No global runtime is selected.

---

# 31. P5-P8 interaction

Earlier scalar/storage lowering may not erase storage identity, ownership action
or cleanup dependency required by P17.

Address-taken and non-trivial owned storage remains high-level.

---

# 32. Verification pipeline

As applicable:

```text
normal MLIR verifier
previous package verifiers
sec-verify-ownership
sec-verify-destruction-plans
sec-verify-cleanups
```

Run ownership verification before destructive cleanup normalization.

---

# 33. Optimization independence

Correctness does not depend on:

```text
copy elision
move elision
RVO
NRVO
DCE
SROA
mem2reg
inlining
cleanup merging
```

Optimization may apply only after semantic verification.

---

# 34. No physical ABI/destructor lowering

Do not lower P17 semantics directly to:

```text
LLVM lifetime intrinsics
LLVM memcpy/memmove
LLVM destructor ABI
exception landing pads
allocator free calls
runtime cleanup tables
```

unless a later package explicitly defines that stage.

---

# 35. Completion

Lowering version 13 is implemented when:

```text
all copy/move actions remain explicit
non-trivial owned values have destruction plans
replacement is transactional with respect to RHS failure
partial struct/payload state is preserved
discard consumes through destruction semantics
cleanup registration/cancellation is explicit
defer is dynamically registered with place captures
scope/function cleanup order is deterministic
return/error ownership is protected from cleanup
panic is not treated as unwinding
no physical ownership/runtime representation is selected
```
