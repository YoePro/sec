# Sec MLIR Dialect

## Status

Normative detailed representation specification.

Current Sec MLIR dialect schema version: `13`

Schema version 13 introduces explicit high-level ownership transfer,
destruction, discard and cleanup operations.

It does not define physical destructor ABI or cleanup-stack representation.

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
```

Compiler-generated v13 modules carry:

```mlir
sec.dialect_version = 13 : i32
```

---

# 2. No ownership-token source type

Schema v13 does not add a source-visible ownership token type.

Ownership is represented through:

```text
SSA value identity
Place/storage identity
explicit ownership operations
explicit cleanup operations
typed ownership metadata
```

---

# 3. Ownership action enum

Canonical action values:

```text
construct-direct
copy-trivial
copy-semantic-infallible
move
borrow-shared
borrow-mutable
non-consuming
```

Use a typed/custom attribute where practical.

---

# 4. `sec.own.copy`

Operand/result:

```text
T -> T
```

Requires:

```text
copy-trivial classification
```

Source remains semantically valid.

---

# 5. `sec.own.semantic_copy`

Operand/result:

```text
T -> T
```

Requires a resolved infallible semantic-copy plan.

Source remains valid.

Result is independently owned where applicable.

---

# 6. `sec.own.move`

Operand/result:

```text
T -> T
```

Consumes source ownership on that path.

Result is the ownership continuation.

---

# 7. `sec.own.copy_from_place`

Operand:

```text
!sec.place<T,...>
```

Result:

```text
T
```

Allowed copy modes:

```text
trivial
semantic-infallible
```

Source place remains initialized.

---

# 8. `sec.own.move_from_place`

Operand:

```text
owned initialized !sec.place<T,...>
```

Result:

```text
T
```

Source place becomes moved/uninitialized.

Carries place/subobject ownership metadata.

---

# 9. `sec.own.initialize_place`

Operands:

```text
uninitialized writable Place<T>
owned T
```

No ordinary result.

Destination becomes initialized owner.

---

# 10. `sec.own.replace_place`

Operands:

```text
initialized writable Place<T>
owned replacement T
```

No ordinary result.

Required:

```text
old destruction plan
old cleanup action ID
new cleanup action ID
```

RHS preparation precedes the op.

MD-125 (2026-11-06, delegated spelling; destruction.md §§ 6.1(4)–(5), 13.2(5)–(8)):

- Equal old and new identities mean the replacement keeps the destination's
  live registration and its position. No `sec.cleanup.cancel` and no new
  `sec.cleanup.track_owned` follow it.
- With `if_owned` (a destination owned on only some paths), the identities must
  differ. The old one names the inherited registration, and the new one the
  registration that `sec.cleanup.track_owned ... unless_active` adds on the
  other paths (§ 15).
- Zero identities remain reserved for a replaced stored field, which stays
  under its owner's registration.

---

# 11. `sec.own.discard`

Consumes an owned/reference value.

Required:

```text
discard_origin
```

Non-trivial owned values also carry destruction plan.

---

# 12. `sec.own.destroy_value`

Consumes owned SSA value.

Required:

```text
destruction_plan
cause
```

No results.

---

# 13. `sec.own.destroy_place`

Consumes owned initialized Place value.

Destination place becomes uninitialized/destroyed.

No results.

---

# 14. Cleanup identity attributes

Canonical compile-time attrs:

```text
sec.cleanup_action_id
sec.cleanup_scope_id
sec.cleanup_registration_ordinal
sec.defer_id
sec.destruction_plan
```

They do not require runtime integer storage.

---

# 15. `sec.cleanup.track_owned`

Registers an automatic destruction responsibility.

Operand identifies value/place.

Required:

```text
cleanup action ID
scope ID
registration ordinal
destruction plan
```

Optional `unless_active = array<i32: ...>` (MD-125, 2026-11-06):

- The registration takes effect only on paths where none of the listed actions
  is live; on the others, the listed registration stays responsible in its
  position.
- The list may name the operation's own action. A loop iteration then keeps the
  instance an earlier iteration registered.
- Identities are non-zero. `--sec-verify-cleanups` models the alternative per
  path, and a whole `sec.own.initialize_place` after `sec.own.destroy_place`
  under a kept registration makes that registration responsible again.

---

# 16. `sec.cleanup.cancel`

Cancels one active automatic cleanup responsibility.

Required:

```text
cleanup action ID
reason
```

---

# 17. `sec.cleanup.defer_register`

Registers one defer instance at runtime semantic execution.

The operation contains one cleanup region.

Capture operands are P15 Places or equivalent compiler-internal binding-place
values.

It may execute repeatedly.

---

# 18. Defer region signature

Region entry arguments correspond exactly to capture operands.

The region terminates with:

```text
sec.cleanup.defer_yield
```

No returned source value.

No propagation to surrounding function.

---

# 19. `sec.cleanup.defer_yield`

Terminator for a defer body region.

No operands/results unless future bookkeeping requires them.

It returns control to cleanup execution, not to source CFG directly.

---

# 20. `sec.cleanup.run_scope`

Executes/removes eligible automatic cleanup for exited lexical scopes.

Required:

```text
scope IDs
exit kind
```

Does not execute function-scoped defer registrations.

---

# 21. `sec.cleanup.run_function`

Executes all remaining active cleanup entries in reverse registration order.

Required:

```text
exit kind
```

Valid only for normal language-controlled function exit.

---

# 22. Exit kinds

Canonical:

```text
fallthrough
return
error-propagation
break
continue
match-branch-exit
switch-branch-exit
```

`run_function` uses function-exit kinds only.

---

# 23. Destruction-plan attribute

A destruction plan attribute/reference identifies type-level semantic
destruction behavior.

It does not contain physical field offsets.

---

# 24. Constructor ownership metadata

Schema-v13 constructor ops accept per-operand ownership actions.

Includes:

```text
sec.struct.construct
sec.array.construct
sec.union.construct
sec.result.ok
sec.result.err
```

Verifier checks action legality against operand/result types.

---

# 25. Call ownership metadata

Direct/foreign call operations retain:

```text
argument ownership actions
result ownership action
```

Foreign consumption additionally requires existing FFI ownership contract
metadata.

P17 does not invent FFI ownership.

---

# 26. Branch ownership metadata

Compiler-generated `cf.br` / `cf.cond_br` carrying owned values use
dialect-scoped ownership action metadata.

The verifier interprets mutually exclusive branch edges path-sensitively.

---

# 27. Return ownership metadata

`func.return` carries the resolved return ownership action where the returned
value is ownership-sensitive.

The return transfer occurs before function cleanup.

---

# 28. P13 struct action extension

`sec.struct.construct` field actions now include:

```text
move
copy-semantic-infallible
```

in addition to previous actions.

---

# 29. P14 array action extension

Ordinary element segments may include:

```text
move
copy-semantic-infallible
```

when Sema resolves them.

Spread remains governed by spread copy rules.

---

# 30. P11 union action extension

Union payload construction supports resolved move/semantic-copy ownership
actions.

---

# 31. Reference/slice discard

`sec.own.discard` of a direct reference/slice must not invoke referent
destruction.

Borrow-lifetime consistency remains verified by the P15/P16 borrow verifier.

---

# 32. Cleanup verifier

Register:

```text
--sec-verify-cleanups
```

Checks registration/cancellation/execution and order.

---

# 33. Ownership verifier

Register:

```text
--sec-verify-ownership
```

Checks move/copy/use/merge/partial state.

---

# 34. Destruction-plan verifier

Register:

```text
--sec-verify-destruction-plans
```

Checks type-level destruction plan completeness.

---

# 35. No panic unwinding

Schema v13 cleanup operations are not implicitly inserted on paths ending in:

```text
sec.fail.*
sec.unreachable
raw non-returning process exit
```

unless a future rule explicitly defines cleanup.

---

# 36. No physical cleanup representation

Schema v13 does not define:

```text
LLVM landingpad
exception personality
runtime cleanup registry
physical destructor function ABI
cleanup stack memory layout
```

---

# 36A. Borrowed UTF-8 string views (MD-126)

Installed 2026-11-06 (delegated spelling; `collections.md` § 7.10,
`flowcontrol_for.md` § 19, `semantic_ir.md` § 17(9)–(10)). Admitted at schema 12
or later; compiler-generated modules use schema 13.

```text
!sec.string_view                    shared non-owning view, no element type
sec.string.rune_len(place)          !sec.place<!sec.string> -> !sec.uint
sec.string.view_borrow(place, s, n) reference properties as sec.slice.borrow_shared
sec.string_view.byte_len(view)      -> !sec.uint
sec.string_view.decode_next(view, cursor) -> (i1, !sec.rune, !sec.uint)
```

- **Holder operations.** Length (rune count), holder copy, nested views and
  holder ends reuse `sec.slice.len`, `sec.slice.copy_shared`,
  `sec.slice.reborrow_shared` and `sec.ref.end_borrow` over `!sec.string_view`.
- **Construction.** `view_borrow` takes the normalized tuple of one
  `sec.slice.range_normalize`/`range_check` whose source length is a constant
  rune count or `rune_len` of the same Place. `--sec-verify-slice-guards`
  requires that, and requires the guard to dominate publication.
- **Decoding.** `decode_next` yields a rune by value at a hidden view-relative
  cursor. It is an observation behind the view's validity guard.
- **Native lowering.** Lowering below the Sec dialect is P23.

---

# 37. Schema-v13 completion

Schema v13 is complete when:

```text
own.* operations parse/print/verify
cleanup.* operations parse/print/verify
defer regions verify
ownership metadata on constructors/calls/branches/returns verifies
partial struct/payload moves verify
cleanup order verifies
schema-v12 regressions remain valid
no physical ownership/destruction representation is selected
```
