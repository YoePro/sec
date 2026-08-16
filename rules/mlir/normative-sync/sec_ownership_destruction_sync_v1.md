# Package 17 Normative Synchronization - Ownership, Destruction, Defer and Discard

## Status

Normative synchronization for:

```text
rules/memory/ownership.md
rules/memory/copy_move.md
rules/memory/destruction.txt
rules/control-flow/defer.md
rules/control-flow/discard.md
rules/compiler/semantic_ir.txt
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

This synchronization selects implementation choices already required or
recommended by the current rulebooks and makes their interaction explicit.

---

# 1. Unified cleanup registration order is canonical

The destruction rulebook presents the unified cleanup stack as the recommended
model.

P17 selects it as canonical.

Semantic cleanup registration events include:

```text
successful initialization of an owned non-trivial value
executed defer statement
reinitialization after a move
successful replacement creating a new owned value
```

Cleanup executes in reverse registration order, subject to scope/function-exit
eligibility.

---

# 2. Local destruction registration

A non-trivial local value registers automatic destruction immediately after
successful initialization.

If initialization never succeeds:

```text
no destruction registration exists
```

If ownership moves out:

```text
the existing registration is canceled/consumed
```

---

# 3. Replacement changes the owned incarnation

Replacing an initialized binding:

```text
destroys/terminates the old owned incarnation
creates a new owned incarnation at the replacement point
```

The new incarnation's cleanup registration occurs at the replacement point.

This gives deterministic interaction with values initialized or defers
registered between the original initialization and later replacement.

---

# 4. Reinitialization after move

Assigning to a moved mutable binding is initialization, not replacement.

Therefore:

```text
no old destruction occurs
new cleanup registration occurs at reinitialization
```

---

# 5. Defer registration

A defer entry is registered only when execution reaches the statement.

It is function-scoped.

It remains registered until normal function exit.

It is not executed on lexical-block exit or loop-iteration end.

---

# 6. Defer capture semantics

A defer does not copy the current value at registration.

The canonical compiler model is:

```text
capture resolved binding/place identity
read the binding/place when the defer executes
```

This preserves:

```sec
let mut value := 1

defer {
    Print(value)
}

value = 2
```

where the defer observes `2`.

---

# 7. Lifetime of defer-referenced bindings

A binding/place required by an active defer registration must remain valid until
the defer executes.

The compiler may extend physical storage lifetime to preserve this.

This is not a semantic copy.

---

# 8. Defer inside loops

Each executed defer statement creates one semantic registration.

If source semantics produce distinct binding instances for iterations, each
registration retains the identity of its own binding instance.

Do not collapse registrations by source variable spelling.

Physical storage strategy remains a lowering concern.

---

# 9. Function exit sequence

Normal return:

```text
1. evaluate return expression
2. establish return ownership transfer
3. run remaining cleanup in reverse registration order
4. return
```

Result error propagation:

```text
1. evaluate Result
2. identify Err
3. establish propagated error ownership
4. run remaining cleanup in reverse registration order
5. return Err
```

Returned/propagated ownership is excluded from callee destruction.

---

# 10. Lexical scope exit

Normal lexical scope exit, break and continue execute automatic cleanup for
exited scopes.

Function-scoped defer registrations do not execute merely because the lexical
scope containing the defer is exited.

---

# 11. Panic remains non-unwinding in P17

Current Sec rules do not define general panic unwinding.

P17 therefore does not route panic endpoints through normal pending cleanup.

No hidden exception unwinding runtime is introduced.

---

# 12. Discard uses deterministic destruction

`discard` is a terminal ownership action.

Explicit or implicit discard of a non-trivial value uses the same destruction
plan as normal lifetime end.

Discard is not:

```text
unused SSA
bit drop
implicit move with lost ownership
```

---

# 13. Discard of references/slices

Discarding:

```text
ref
ref mut
shared slice
mutable slice
```

ends the reference holder/borrow lifetime as appropriate.

It never destroys the referent/backing elements merely because the reference
value is discarded.

---

# 14. Infallible semantic copy

Only infallible semantic copy belongs to implicit/ordinary ownership transfer.

Fallible duplication remains an explicit function returning Result.

No assignment may hide copy failure.

---

# 15. Move source state

After move:

```text
source has no usable semantic value
source is not automatically defaulted
source is not destroyed
destination owns destruction responsibility
```

Reinitialization is explicit through later assignment to mutable storage.

---

# 16. Partial move scope

P17 initially supports direct field/payload partial move only where Sema can
track subobject state safely.

Do not generalize partial move to runtime-indexed fixed-array elements.

---

# 17. Custom free restriction

P17 may represent a resolved destruction plan with custom free.

It does not make user-defined `free` syntax accepted before the dedicated
parser/Sema restrictions are implemented.

---

# 18. Semantic IR cleanup requirement

Before high-level MLIR ownership verification completes:

```text
every non-trivial owned value has a known terminal ownership action
every normal exit has an explicit cleanup plan
every moved value is excluded from destruction
every partial aggregate has exact remaining initialized state
```

No backend inference is permitted.
