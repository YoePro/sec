# Discard

## Current implementation status

Implemented:

- parser support for `discard expression`;
- AST representation of the discarded expression;
- compatibility name field for identifier discards;
- semantic analysis of discarded expressions;
- explicit discard of temporary expression results such as function calls;
- identifier discard consumes the named binding and makes later reads invalid;
- use-after-discard diagnostic for simple local identifiers;
- ordinary assignment to a discarded local binding is rejected;
- discard of `ref` and `ref mut` bindings ends the reference binding;
- discard rejects unresolved `Task[T]` handles;
- discard rejects direct `spawn` results because the successful branch would
  abandon the lifecycle handle;
- parser/AST/compiler dump output shows the discarded expression.

Not implemented yet:

- field-sensitive partial discard;
- path-sensitive discarded-state diagnostics distinct from moved-state after
  branch merges;
- destructor lowering or Semantic IR `DiscardValue`;
- non-discardable `Thread[T]` and `Process` checks before those types are fully
  compiler-known;
- spawn `Result[Handle, Error]` modelling for `spawn thread` and
  `spawn process`;
- warnings for useless literal/constant discards;
- enforcement that every non-void expression statement must be explicitly
  consumed or discarded;
- must-use analysis beyond existing defer `Result` handling;
- functions.txt synchronization for the future non-void expression-statement
  rule.

## Purpose

`discard` explicitly consumes a value whose result or remaining lifetime the
programmer intentionally does not need.

It is a general ownership and destruction operation.

It is not specific to tasks, threads or error handling.

---

## Syntax

```text
discard-statement:
    discard expression
```

Examples:

```sec
discard value
discard Calculate()
discard worker.value
```

`discard` is a statement.

It does not produce a value.

The expression is evaluated exactly once.

---

## Core meaning

```sec
discard value
```

means:

1. evaluate and resolve `value`;
2. consume the resulting owned value;
3. execute deterministic destruction when required;
4. mark the consumed binding or storage value unavailable;
5. continue with the next statement.

For trivially destructible values, lowering may produce no machine instruction.

The semantic consumption still exists.

---

## Discarding a binding

```sec
let resource := OpenResource()
discard resource
```

After the statement, `resource` is unavailable.

Later use is invalid:

```sec
Use(resource)
```

Expected diagnostic:

```text
value resource was discarded here and is no longer available
```

`discard` ends the binding's value lifetime.

The same binding is not reinitialized by ordinary assignment afterward.

A new declaration may shadow the name according to normal scope rules.

---

## Copyable values

An ordinary read of a copyable value may copy it.

`discard` is different.

```sec
let number := 42
discard number
```

consumes the binding's owned value and ends its availability.

The compiler must not implement this as an ordinary copy followed by destruction
of the copy.

---

## Move-only values

Discarding a move-only value consumes it exactly once.

```sec
let file := OpenFile()
discard file
```

This performs the type's deterministic destruction.

A later move or read is invalid.

No explicit `move(...)` syntax is required.

---

## Discarding an expression result

A function result may be explicitly ignored:

```sec
discard Calculate()
```

The call executes normally.

Its return value is then consumed and destroyed.

This is the explicit alternative to silently losing a non-void result.

---

## void expressions

Discarding a `void` expression is valid:

```sec
discard LogMessage()
```

It evaluates the call and produces no payload to destroy.

The compiler may warn that `discard` is unnecessary when the call is already a
valid standalone `void` statement.

---

## Result

Discarding a `Result[T, E]` explicitly acknowledges that both success and error
payloads are intentionally ignored:

```sec
discard TryWriteLog()
```

This satisfies must-use analysis only because the programmer wrote `discard`
explicitly.

The compiler must not infer the discard.

Discarding a `Result` destroys the active variant payload according to ordinary
rules.

---

## Option

Discarding `Option[T]` destroys the contained `T` when the value is `Some`.

`None` requires no payload destruction.

---

## References

Discarding a reference value ends that reference binding.

It does not destroy the referent.

```sec
let view := ref value
discard view
```

Afterward `view` is unavailable, while `value` remains owned by its original
owner.

Discarding an owned value while an incompatible live borrow exists is invalid.

---

## Mutable references

Discarding `ref mut` ends that mutable reference's lifetime.

This may release the exclusive borrow early when the compiler can prove that the
reference is not otherwise retained.

It does not destroy the referent.

---

## Guards and subscriptions

Discard follows ordinary deterministic destruction.

Therefore:

```sec
discard guard
```

may release a `MutexGuard[T]` early.

```sec
discard subscription
```

closes and unregisters a `Subscription` when its destructor defines that
behavior.

The rulebook for each type defines its destruction effect.

---

## Execution handles

Unresolved lifecycle handles are not generally discardable.

Invalid:

```sec
discard task
discard thread
discard process
```

Reason:

> Destruction must not silently abandon an unresolved execution entity.

The owner must use an explicit lifecycle operation:

```sec
await task
join thread
detach task
detach thread
```

After successful join, the stored result may be discarded:

```sec
join worker
discard worker.value
```

For a future result intentionally abandoned at detach:

```sec
detach worker discard
```

This is lifecycle syntax defined by task and thread rules.

It is not parsed as `discard worker`.

---

## Spawn results

Because spawn is fallible:

```sec
spawn thread Work()
```

produces a `Result`.

Discarding the complete spawn result is invalid when the `Ok` variant would
contain an unresolved lifecycle handle.

Invalid:

```sec
discard spawn thread Work()
```

The caller must match, propagate with `try`, or otherwise resolve both creation
failure and successful handle ownership.

---

## Joined copyable results

After join:

```sec
join worker
```

a copyable result may be read repeatedly until the stored result is consumed or
the joined handle is destroyed.

```sec
let first := worker.value
let second := worker.value
```

Explicit discard consumes the stored result:

```sec
discard worker.value
```

A later access is invalid even when `T` is copyable.

---

## Partial values

Discarding a complete aggregate destroys all still-initialized fields.

Discarding a field consumes and destroys that field.

The containing aggregate becomes partially initialized when partial move rules
permit it.

The compiler must not later destroy an already discarded field.

---

## Constants and literals

Discarding a literal or compile-time constant is valid but normally useless:

```sec
discard 42
```

The compiler should emit a configurable diagnostic because no owned resource or
effect is consumed.

The statement may lower to no code.

---

## Unused variables

Unused local variables are compiler errors under Sec's diagnostics rules.

Explicit discard documents intentional non-use:

```sec
let result := Calculate()
discard result
```

A declaration immediately followed by discard may still receive a style
diagnostic when the expression could be discarded directly:

```sec
discard Calculate()
```

The semantic program remains valid.

---

## Relationship to underscore

`discard` does not replace the context-specific meanings of `_`.

The underscore remains distinct for:

- match ignore patterns;
- register reserved bits;
- visibility and scope naming rules;
- any separately defined declaration discard syntax.

Bare `_` must not be reused as a general destruction statement.

---

## Control flow

A value discarded on one branch is unavailable only on paths where the discard
executed.

At merge points the compiler must reconcile ownership state.

Example:

```sec
if condition {
    discard value
} else {
    Use(value)
}
```

After the `if`, `value` is not definitely available and cannot be used unless
control-flow analysis proves a valid state.

---

## defer

A deferred operation that captures an owned value may conflict with earlier
discard.

Invalid:

```sec
defer Close(resource)
discard resource
```

when the deferred call requires the same owned value.

The compiler must diagnose the later deferred use.

A defer registered to run destruction may itself be unnecessary when `discard`
already performs deterministic destruction.

---

## Function return values

Functions returning non-void values may be called as statements only through
explicit discard or another consuming context.

Valid:

```sec
discard Calculate()
```

Invalid when the language requires result use:

```sec
Calculate()
```

A `void` function call may remain a normal statement.

`functions.txt` must be updated accordingly.

---

## Discardability

Most owned values are discardable through deterministic destruction.

A type is not discardable while it carries an unresolved semantic obligation
that ordinary destruction is forbidden to hide.

Initial non-discardable examples:

```text
unresolved Task[T]
unresolved Thread[T]
unresolved Process
a spawn Result whose Ok payload is an unresolved lifecycle handle
```

A type-specific rule may add another obligation.

The compiler derives discardability from type semantics.

---

## Panic and errors during destruction

`discard` uses the normal destruction model.

It does not introduce a separate error-return path.

If destruction may panic or fail, the owning type's destruction rule must define
that behavior.

`discard` must not silently suppress destruction failure.

---

## Core runtime errors

This rule introduces no runtime `DiscardError`.

Discard validity is determined statically.

All runtime errors produced by the discarded expression belong to that
expression's API and, when they are language-level core errors, must be declared
in:

```text
core/errors.sec
```

Compiler diagnostics are not runtime errors.

---

## Parser

The parser must recognize:

```sec
discard expression
```

as `DiscardStatement`.

The expression extends to the normal statement boundary.

`discard` is a keyword.

It must not be parsed as a function call.

---

## AST

Conceptual AST:

```text
DiscardStatement
    Token
    Value Expression
```

The AST does not decide whether the value is discardable.

---

## Semantic analysis

Sema must determine:

- expression type;
- ownership category;
- copy/move state;
- active borrows;
- discardability;
- destruction effect;
- partial initialization;
- lifecycle obligations;
- must-use acknowledgement;
- control-flow availability after discard.

When the operand names owned storage, the original storage becomes
uninitialized/unavailable.

When the operand is a temporary, the temporary is destroyed before the next
statement.

---

## Semantic IR

Semantic IR must represent:

```text
DiscardValue
```

The operation records:

- consumed value or storage;
- type;
- destruction operation;
- source location;
- whether the value is temporary, binding or field;
- resulting initialization state.

A lifecycle-specific detach discard remains a distinct operation:

```text
TaskDetachDiscard
ThreadDetachDiscard
```

The backend must not infer discard from an unused SSA value.

---

## Diagnostics

Examples:

```text
value resource was discarded here and is no longer available
```

```text
cannot discard value while it is borrowed by view
```

```text
cannot discard unresolved Task[int]; await, join or detach it explicitly
```

```text
cannot discard spawn result because successful creation would abandon Thread[void]
```

```text
field payload has already been moved or discarded
```

```text
discard of literal int has no observable effect
```

Diagnostics must have stable IDs.

---

## Required synchronization

The new rule requires updates to:

```text
lexical_structure.md
types.txt
functions.txt
copy_move.txt
ownership.txt
borrowing.txt
references.txt
lifetime_analysis.txt
destruction.txt
defer.txt
errorhandling.txt
diagnostics.txt
semantic_ir.txt
compiler_pipeline.txt
formatter.txt
tasks.txt
threads.md
processes.txt
spawn.md
await.md
channels.md
mutex.md
events.md
rules_implementations.txt
core-library.md
core/errors.sec
```

Required key changes include:

- `types.txt`: define discardability and unavailable state;
- `functions.txt`: non-void call results require a consuming context or
  `discard`;
- `copy_move.txt`: distinguish copy, move and discard consumption;
- `destruction.txt`: define explicit early destruction through discard;
- `ownership.txt`: add discard as ownership consumption;
- `borrowing.txt`: reject discard while incompatible borrows are live;
- `errorhandling.txt`: define explicit discard of `Result[T, E]`;
- `semantic_ir.txt`: add `DiscardValue`;
- task/thread/process rules: forbid ordinary discard of unresolved handles;
- formatter: canonical `discard expression` formatting.
