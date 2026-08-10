# Semantic IR Amendment - Ownership Transfer and Destruction

## Status

Normative amendment for:

```text
rules/semantic_ir.txt
```

Package:

```text
SEC-MLIR-P17
```

Repository baseline:

```text
152c772
```

Local predecessors:

```text
SEC-MLIR-P13
SEC-MLIR-P14
SEC-MLIR-P15
SEC-MLIR-P16
```

This amendment defines canonical Semantic IR ownership transfer, deterministic
destruction, discard and cleanup semantics.

---

# 1. Ownership is explicit

Every ownership-sensitive value use is classified before Semantic IR
construction.

Semantic IR does not infer ownership from SSA use count.

---

# 2. Transfer actions

Canonical actions:

```text
construct
copy-trivial
copy-semantic-infallible
move
borrow-shared
borrow-mutable
non-consuming
```

Fallible clone-like operations are ordinary calls, not copy actions.

---

# 3. Owned value state

Semantic IR analysis tracks:

```text
uninitialized
initialized
moved
destroyed
partial
```

per path.

Whole-value use requires compatible initialized state.

---

# 4. Place state

Addressable owned places retain initialization/ownership state.

Move from Place changes the place state without requiring physical byte
modification.

---

# 5. Partial subobjects

P17 supports explicit state for:

```text
struct fields
active union payload
Result payload
Option payload
```

Moved subobjects are excluded from later destruction.

---

# 6. Copy

Copy creates a new semantic value.

Source remains valid.

The result owns an independent destruction responsibility when non-trivial.

---

# 7. Semantic copy

Semantic copy is valid only with a resolved infallible copy plan.

A failing duplication algorithm is not a SemanticCopy operation.

---

# 8. Move

Move transfers ownership.

Source becomes unavailable.

Destination receives destruction responsibility.

No source destruction occurs.

---

# 9. Move from place

Moving from an owned initialized Place:

```text
produces an owned SSA value
marks the source place moved/uninitialized
updates partial aggregate state when applicable
```

---

# 10. Initialization

Initialization consumes an owned value into uninitialized storage.

The storage becomes the owner.

A new cleanup responsibility is registered when required.

---

# 11. Replacement

Replacement consumes a prepared new value into initialized storage.

The old destination ownership terminates exactly once through its destruction
plan.

The new owned incarnation is registered at the replacement point.

---

# 12. Transactional replacement

When replacement preparation is fallible, the old destination stays initialized
on the failure path.

Replacement occurs only on the successful path.

---

# 13. Destruction plans

Every non-trivially destructible concrete runtime type has one canonical
destruction plan.

Plans are semantic and layout-independent.

---

# 14. Derived struct destruction

Destroy initialized unmoved stored fields in reverse declaration order.

---

# 15. Derived fixed-array destruction

Destroy initialized elements in reverse index order.

Compact loop-oriented representation is allowed.

---

# 16. Variant destruction

Union/Result/Option destruction selects only the active initialized unmoved
payload.

---

# 17. Custom free

Semantic IR can identify a resolved custom-free implementation.

Source acceptance of custom free declarations remains controlled by the
dedicated frontend rules.

---

# 18. Discard

Discard is explicit semantic consumption.

It preserves explicit/implicit/compiler provenance.

Non-trivial discard invokes the same destruction plan as ordinary lifetime end.

---

# 19. Reference discard

Reference/slice discard consumes the reference value and may end the borrow.

It does not destroy referent ownership.

---

# 20. Cleanup registration

Semantic IR explicitly represents cleanup registration/cancellation.

Registration order is semantic because destruction/defer effects may be
observable.

---

# 21. Unified cleanup ordering

Automatic destruction and defer registration share one semantic registration
order.

Normal cleanup executes eligible active entries in reverse registration order.

---

# 22. Scope cleanup

Lexical scope exits run only automatic actions whose ownership scope ends.

Function-scoped defer remains pending.

---

# 23. Function cleanup

Normal function exit runs all remaining active cleanup entries after return/error
ownership is prepared.

---

# 24. Defer registration

Defer registration is dynamic semantic execution.

A loop may register the same source defer site multiple times.

---

# 25. Defer captures

Defer captures binding/place identity rather than copying the current value.

The captured place must remain valid until execution.

---

# 26. Defer body

A defer body is explicit Semantic IR cleanup code with capture-place parameters.

It cannot redirect the triggering function exit.

It cannot propagate errors.

---

# 27. Panic boundary

Pending normal cleanup is not assumed to run during panic.

No unwinding semantics are introduced.

---

# 28. Branch ownership

Owned branch arguments carry explicit transfer action.

At merge, the selected block argument becomes the single owner.

Incompatible ownership states are invalid.

---

# 29. Calls

Call arguments and return values carry explicit ownership actions.

A consuming argument removes caller destruction responsibility.

Borrowed arguments use reference values.

---

# 30. Return

Returning an owned value transfers it out before cleanup.

The callee must not destroy it afterward.

---

# 31. Verification

Semantic IR verification must prove:

```text
copy legality
move availability
no use after move
no double terminal action
complete normal-exit cleanup
partial-state correctness
destruction-plan completeness
cleanup order
defer order
return/error exclusion
```

---

# 32. No runtime ownership table

Ownership state, subobject state and cleanup IDs are compiler semantics.

They need not exist at runtime.

---

# 33. Deterministic printer

Print ownership-sensitive operations explicitly.

Printer output should expose:

```text
transfer mode
Place identity
destruction plan ID
cleanup action ID
scope ID
discard cause
defer ID
```

without printing physical layout assumptions.
