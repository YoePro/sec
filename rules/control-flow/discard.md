# Discard

- **Status:** Normative
- **Created:** 2026-07-31
- **Last updated:** 2026-08-26
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/control-flow/discard.md`
- **Repository baseline reviewed:** `b3315f6`

---

## 1. Purpose

`discard` explicitly consumes a value whose result or remaining lifetime the programmer intentionally does not need.

It is a general ownership and deterministic-destruction operation.

It is not specific to tasks, threads, error handling, or function results.

Sec also permits an ordinary non-`void` function or method result to be implicitly discarded when the call is used as a standalone statement.

Values with must-use semantics are never implicitly discarded.

---

## 2. Syntax

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

The operand expression is evaluated exactly once.

---

## 3. Core meaning

For:

```sec
discard value
```

the compiler must:

1. evaluate and resolve `value`;
2. validate that explicit discard is legal for the resolved value;
3. consume the resulting value or place;
4. execute deterministic destruction when required;
5. end any ownership or borrow responsibility carried by that consumed value;
6. mark consumed storage unavailable when applicable;
7. continue with the next statement.

For trivially destructible values, lowering may produce no machine instruction.

The semantic consumption still exists.

`discard` is not:

- an unused SSA result;
- an implicit copy followed by destruction of the copy;
- a move to another owner;
- a general exception or error-suppression mechanism;
- a replacement for lifecycle operations such as `await`, `join`, or `detach`.

---

## 4. Explicit and implicit discard

Sec distinguishes:

```text
explicit discard
    the programmer writes `discard expression`

implicit call-result discard
    an ordinary discardable non-void call result is unused because the call is a standalone statement
```

Both forms consume the produced value and run its required deterministic destruction.

Only explicit discard may acknowledge a must-use value.

Explicit:

```sec
discard Calculate()
```

Implicit ordinary call-result discard:

```sec
Calculate()
```

The second form is valid only when the result is not must-use and is discardable.

---

## 5. Discarding a binding

```sec
let resource := OpenResource()
discard resource
```

After the statement, the current value of `resource` is unavailable.

A later read, borrow, or move is invalid until a legal reinitialization occurs.
A second discard is a legal no-op because `discard` converges the place to
`Unavailable`; tooling may issue a low-priority redundancy advisory.

```sec
Use(resource)
```

must be rejected.

A diagnostic must identify the discard that made the value unavailable.

If the incoming place is `ConditionallyAvailable`, discard ends/destroys the
value only on paths that still own it and leaves every outgoing path
`Unavailable`. Discardability and incompatible-borrow checks still apply before
this convergence commits.

`discard` ends the lifetime of the binding's current value. It does not remove the binding name from scope.

---

## 6. Reinitialization after discard

Mutability and availability are separate.

A mutable binding may be reinitialized after its previous value has been discarded:

```sec
let mut resource := OpenResource()
discard resource

resource = OpenResource()
Use(resource)
```

The assignment after `discard` is reinitialization, not replacement.

Therefore:

- no old destination value is destroyed;
- the new value becomes available;
- a new destruction responsibility is established for the new value;
- cleanup registration occurs at the reinitialization point when required.

An immutable binding cannot be reinitialized after discard.

```sec
let resource := OpenResource()
discard resource
resource = OpenResource()
```

is invalid.

---

## 7. Copyable values

An ordinary read of a copyable value may copy it.

`discard` is different.

```sec
let number := 42
discard number
```

consumes the binding's current value and makes that binding unavailable.

The compiler must not implement explicit discard of a binding as:

```text
copy binding
destroy copy
leave original binding available
```

That would change the source semantics.

---

## 8. Move-only values

Discarding a move-only value consumes it exactly once.

```sec
let file := OpenFile()
discard file
```

The value's deterministic destruction executes according to its type.

The source becomes unavailable.

No explicit `move(...)` syntax is required for discard.

---

## 9. Explicitly discarding an expression result

A function or method result may be explicitly ignored when the complete result type is discardable:

```sec
discard Calculate()
```

The call executes normally.

Receiver, argument, ownership, borrow, error, and effect checks are unchanged by the surrounding `discard`.

For example:

```sec
discard self.Advance()
```

still requires whatever receiver authority `Advance` normally requires.

The produced temporary is consumed and destroyed before the next statement.

Explicit discard documents programmer intent and does not trigger the implicit-discard advisory for that result.

---

## 10. Implicitly discarding an ordinary call result

An ordinary function or method call returning a discardable non-`void` value may be used as a standalone statement:

```sec
Calculate()
lexer.NextToken()
buffer.WriteByte(value)
```

The call is evaluated normally.

Its returned temporary is then implicitly consumed and destroyed at the end of the statement.

Conceptually:

```text
1. evaluate receiver and arguments
2. perform the call
3. construct the returned temporary
4. validate implicit-discard legality
5. consume and destroy the temporary
6. continue with the next statement
```

No user-visible binding is created.

The backend must not treat a non-trivial returned value as merely an unused SSA result.

---

## 11. Expression-statement boundary

Implicit discard is defined for call-like standalone expression statements.

Initial call-like forms include:

- ordinary function calls;
- method calls;
- constructors and compiler-known call forms that follow ordinary call semantics.

Examples:

```sec
UpdateCache()
object.Refresh()
string.FromRuneArray(runes)
```

This rule does not make arbitrary value expressions meaningful statements.

Examples such as:

```sec
1 + 2
value
person.name
```

do not become valid merely because a discarded-result mechanism exists.

Their legality is defined by the ordinary expression-statement and unused-expression rules.

---

## 12. Advisory for implicit ordinary results

An implicitly discarded ordinary non-`void` call result is eligible for the configurable advisory:

```text
ownership.implicit-discarded-result
```

Default severity:

```text
info
```

Allowed policy levels:

```text
off
info
warning
error
```

The diagnostic ID is stable regardless of configured severity.

Promoting the advisory to `error` is a project diagnostic policy. It does not change Sec ownership semantics.

A project requiring explicit acknowledgement may therefore require:

```sec
discard Calculate()
```

instead of:

```sec
Calculate()
```

No implicit-discard advisory is emitted for a `void` call.

A must-use violation is a mandatory semantic error and is not replaced by this advisory.

---

## 13. `void` expressions

A `void` call is already a normal standalone statement:

```sec
LogMessage()
```

Explicit discard remains legal:

```sec
discard LogMessage()
```

The expression is evaluated and produces no payload to destroy.

The compiler may emit a configurable advisory that the explicit discard is unnecessary.

---

## 14. Must-use and discardability

Must-use and discardability are distinct semantic properties.

```text
must-use
    implicit discard is forbidden

discardable
    explicit discard is permitted
```

A value may therefore be:

- not must-use and discardable;
- must-use and discardable;
- must-use and non-discardable.

A must-use value cannot disappear through implicit discard.

Explicit discard acknowledges intentional non-use only when the complete value is discardable.

---

## 15. Initial must-use categories

Compiler-known must-use categories include at least:

```text
Result[T, E]
unresolved Task[T]
unresolved Thread[T]
unresolved Process, when Process is available
spawn results whose successful payload contains an unresolved lifecycle handle
other types explicitly defined as carrying a must-use obligation
```

A standalone expression producing a must-use value is a mandatory semantic error.

The diagnostic must explain:

- the exact result type;
- why the value is must-use;
- which obligation would otherwise be hidden;
- valid handling forms;
- whether explicit `discard` is legal.

Must-use errors cannot be disabled or demoted by advisory configuration.

---

## 16. `Result[T, E]`

`Result[T, E]` is always must-use.

This is invalid:

```sec
TryWriteLog()
```

because implicit discard could hide `Err(E)`.

Valid handling forms include:

- `try`;
- `match`;
- binding the result;
- returning it;
- passing it into another valid consuming context;
- explicit `discard` when the complete `Result` is discardable.

Explicit acknowledgement:

```sec
discard TryWriteLog()
```

means that both success and error variants are intentionally ignored.

The compiler must not infer this acknowledgement.

Discarding a `Result` destroys only the active initialized payload according to ordinary destruction rules.

Explicit discard is legal only when every possible active payload that may occur at that point is discardable, unless control-flow analysis has already proven a more specific active state.

Example:

```text
Result[int, IOError]
    must-use
    explicitly discardable

Result[Thread[int], SpawnError]
    must-use
    not discardable while Ok may contain an unresolved thread
```

---

## 17. `Option[T]`

`Option[T]` is not automatically must-use merely because it is an option.

A standalone call returning an ordinary discardable `Option[T]` may therefore be implicitly discarded and receive the normal advisory.

Explicit discard destroys the active `Some` payload according to ordinary destruction rules.

`None` has no payload to destroy.

Discardability is recursive.

For example:

```text
Option[int]
    discardable

Option[Thread[void]]
    not discardable while Some may contain an unresolved thread
```

---

## 18. Recursive discardability

Discardability is derived from semantic obligations, not merely from representation.

Conceptually:

```text
ordinary scalar
    discardable

aggregate
    discardable only when every still-initialized owned field that may be active is discardable

Option[T]
    discardable when T is discardable

Result[T, E]
    discardable when every possible active success/error payload is discardable

union
    discardable when every possible active owned payload is discardable,
    unless control-flow analysis proves a specific active discardable variant
```

A type-specific rule may define another non-discardable obligation.

---

## 19. References

Discarding a reference value ends that reference holder.

It does not destroy the referent.

```sec
let view := ref value
discard view
```

Afterward:

- `view` is unavailable;
- `value` remains owned by its original owner;
- the borrow held only by `view` may end when the compiler proves no other holder retains it.

Discarding an owned value while an incompatible live borrow exists is invalid.

---

## 20. Mutable references

Discarding `ref mut` ends that mutable reference holder.

It does not destroy the referent.

Because `ref mut` represents exclusive borrowed authority, ending the holder may release that exclusive borrow early when the compiler proves that no other retained value carries it.

---

## 21. Slices and other non-owning views

Discarding a non-owning reference-like view ends the view value and any borrow responsibility carried by that view.

It does not destroy the backing elements merely because the view is discarded.

The owning rulebook for the specific view type determines whether additional obligations exist.

---

## 22. Guards and subscriptions

Discard follows ordinary deterministic destruction.

Therefore:

```sec
discard guard
```

may release a guard early when the guard type's destruction releases the protected resource.

Likewise:

```sec
discard subscription
```

may close or unregister a subscription when its destruction semantics define that behavior.

An API whose returned value should almost never be destroyed immediately should mark that type must-use when such a semantic classification is available.

---

## 23. Execution handles

An unresolved lifecycle handle is not discardable.

Examples:

```sec
discard task
discard thread
```

are invalid while those values still carry unresolved execution obligations.

The owner must use a lifecycle operation permitted by the handle type, such as:

```text
await
join
detach
return or transfer ownership
```

where applicable.

Lifecycle-specific source forms such as:

```sec
detach worker discard
```

are not equivalent to:

```sec
discard worker
```

The former is governed by lifecycle rules and may explicitly define what happens to a future result.

---

## 24. Spawn results

Spawn is fallible.

A successful spawn produces an unresolved lifecycle handle.

Conceptually, a thread spawn may produce:

```text
Result[Thread[T], SpawnError]
```

Therefore both of these are invalid:

```sec
discard spawn thread Work()
spawn thread Work()
```

when the successful result would abandon an unresolved thread.

The caller must resolve both:

- creation failure;
- successful handle ownership.

A diagnostic for this case must not suggest explicit `discard`, because explicit discard is itself invalid.

---

## 25. Stored lifecycle results

After a lifecycle operation has resolved the execution obligation, the handle rulebook determines what stored result remains available and whether it is discardable.

If a stored result is explicitly discarded:

```sec
discard worker.value
```

that place is consumed.

A later access is invalid until a legal reinitialization exists under the containing type's rules.

Implicit call-result discard never implicitly consumes an existing stored member merely because that member is read as a standalone expression.

---

## 26. Aggregates and partial values

Discarding a complete aggregate destroys every still-initialized owned field according to the aggregate's destruction plan.

A direct field may be discarded only where the ownership and partial-state rules permit that field to be consumed independently.

```sec
discard object.payload
```

may make the containing aggregate partially initialized.

The compiler must then:

- mark the field unavailable;
- prevent later destruction of that already discarded field;
- reject complete-value use while the aggregate remains incomplete;
- permit reinitialization only where the general partial-reinitialization rules allow it.

Runtime-indexed arbitrary partial discard of collection elements is not implied by this rule.

Collection removal and extraction use the collection's explicit ownership API.

---

## 27. Constants and literals

Discarding a literal or compile-time constant is legal but normally has no useful semantic effect:

```sec
discard 42
```

The compiler may emit a configurable advisory such as:

```text
discard of literal int has no observable effect
```

The statement may lower to no machine code.

---

## 28. Unused variables

Unused local variables remain subject to the ordinary unused-local rules.

Explicit discard documents intentional non-use:

```sec
let result := Calculate()
discard result
```

A style advisory may suggest the direct form when equivalent:

```sec
discard Calculate()
```

A standalone ordinary call:

```sec
Calculate()
```

creates no local binding and therefore does not trigger the unused-local rule.

It may trigger the implicit-discard advisory.

---

## 29. Relationship to `_`

`discard` does not replace the context-specific meanings of `_`.

The underscore remains distinct for language contexts including:

- match ignore patterns;
- register reserved fields;
- loop discard binding positions;
- other explicitly defined pattern discard positions.

Bare `_` is not a general destruction expression.

This is invalid:

```sec
discard _
```

because `discard` requires an expression that identifies or produces a value.

---

## 30. Control-flow state

Discard is path-sensitive.

```sec
if condition {
    discard value
} else {
    Use(value)
}
```

After the merge, the compiler must reconcile the value's availability across all continuing paths.

If the value is unavailable on any continuing path and no rule proves one compatible available state, later complete use is invalid.

An implicitly discarded call-result temporary creates no binding state that survives the statement.

---

## 31. Interaction with `defer`

A registered `defer` may retain a future use of a binding or place.

If an active defer still requires the value, an earlier discard is invalid.

Conceptually:

```sec
defer {
    Close(resource)
}

discard resource
```

must be rejected when the deferred operation requires the same owned value.

A `Result` produced inside `defer` remains must-use.

Therefore:

```sec
defer {
    TryClose(resource)
}
```

is invalid when `TryClose` returns `Result` and the result is otherwise unhandled.

Explicit acknowledgement is possible only when the complete result is discardable:

```sec
defer {
    discard TryClose(resource)
}
```

---

## 32. Cleanup and destruction

Explicit and implicit discard use the same resolved destruction machinery as normal lifetime end.

For a non-trivial owned binding, explicit discard must conceptually:

1. execute the value's resolved destruction plan;
2. cancel the old pending cleanup responsibility for that consumed value;
3. mark the place unavailable;
4. prevent later automatic destruction of the consumed value.

A later legal reinitialization creates a new destruction responsibility.

This rule prevents double destruction.

For aggregates:

- structs destroy still-initialized owned fields according to their destruction order;
- arrays destroy still-initialized elements according to their destruction order;
- unions destroy only the active initialized unmoved payload;
- `Result` destroys only the active `Ok` or `Err` payload;
- `Option` destroys only an active `Some` payload;
- references and borrowed slices do not destroy their referents.

---

## 33. Destruction failure boundary

`discard` introduces no independent error-return channel.

If a type's destruction may panic, abort, fail, or require another policy, that behavior is defined by the destruction and panic rules for the type and target profile.

`discard` must not silently suppress a failure that ordinary deterministic destruction would expose.

No runtime `DiscardError` is introduced.

---

## 34. Parser and AST requirements

The parser recognizes:

```sec
discard expression
```

as an explicit discard statement.

Conceptual AST:

```text
DiscardStatement
    Token
    Value Expression
```

The AST does not decide whether the operand is must-use, discardable, borrowed, owned, partial, or lifecycle-constrained.

Those are semantic decisions.

Ordinary standalone calls remain ordinary expression statements in the source AST.

The source AST does not require a separate implicit-discard node.

---

## 35. Semantic analysis requirements

For explicit discard, Sema must determine at least:

- resolved expression type;
- whether the operand is a value, temporary, binding, field, or another place;
- ownership state;
- copy/move availability state;
- active borrow conflicts;
- must-use classification;
- discardability;
- destruction plan;
- partial initialization state;
- lifecycle obligations;
- control-flow state after consumption.

For standalone call expressions, Sema must determine at least:

- whether the expression is a permitted call-like statement;
- return type;
- whether the return is `void`;
- must-use classification;
- discardability;
- whether implicit discard is legal;
- applicable advisory policy;
- required destruction of the returned temporary;
- receiver and argument effects.

Required validation ordering is conceptually:

```text
1. resolve and validate the call/expression
2. validate ownership and borrow legality
3. determine must-use status
4. reject illegal implicit loss of must-use values
5. determine discardability
6. reject explicit or implicit discard of non-discardable values
7. emit configurable advisory when applicable
8. record terminal ownership and destruction effects
```

A mandatory semantic error must remain mandatory even when all related advisories are configured `off`.

---

## 36. Semantic IR requirements

Semantic IR must represent validated discard as an explicit terminal ownership action.

Canonical conceptual operation:

```text
DiscardValue
```

It records at least:

- consumed value or place;
- resolved type;
- destruction plan or trivial-destruction classification;
- source location;
- resulting availability state when a place is consumed;
- discard provenance.

Discard provenance distinguishes at least:

```text
Explicit
ImplicitCallResult
CompilerTemporary
```

`CompilerTemporary` describes compiler-created terminal temporary cleanup and is not a third source spelling of `discard`.

Lifecycle-specific operations remain separate when their semantics are not ordinary discard, including detach-with-result-discard forms.

The backend must not infer semantic discard merely from an unused SSA value.

---

## 37. Diagnostics

Primary discard diagnostics use stable registered IDs and symbolic names.

Important diagnostic categories include:

### Implicit ordinary result

```text
ownership.implicit-discarded-result
```

Configurable advisory, default `info`.

### Unhandled must-use result

Mandatory semantic error.

The diagnostic must explain the hidden obligation and valid handling forms.

### Non-discardable value

Mandatory semantic error.

The diagnostic must identify the obligation that makes explicit discard illegal.

### Use after discard

Mandatory semantic error with a related location pointing to the discard.

### Discard while borrowed

Mandatory semantic error identifying the conflicting borrow.

### Useless explicit discard

Configurable advisory.

### Redundant `void` discard

Configurable advisory.

Diagnostic IDs identify rules, not severity.

---

## 38. Diagnostic configuration

Advisory discard diagnostics follow the general project diagnostic configuration model.

Example:

```toml
[diagnostics.rules]
"ownership.implicit-discarded-result" = "warning"
```

A project may configure the advisory as:

```text
off
info
warning
error
```

Must-use, ownership-safety, borrow-safety, and lifecycle-obligation errors are mandatory and cannot be configured away.

---

## 39. Formatter requirements

Canonical explicit syntax is:

```sec
discard expression
```

The formatter must preserve explicit `discard` and format its operand using ordinary expression formatting.

Ordinary formatting must not insert or remove `discard` merely because a configurable advisory is enabled or disabled.

Changing an ordinary standalone call into explicit discard is a semantic/documentation code action, not unconditional canonical formatting.

---

## 40. Required language tests

The language test suite must cover at least:

- explicit discard of a trivially destructible binding;
- explicit discard of a move-only resource;
- use after discard;
- second discard of an already unavailable place as a legal no-op, with only an optional redundancy advisory;
- mutable reinitialization after discard;
- immutable reinitialization rejection after discard;
- implicit ordinary non-`void` call-result discard;
- implicit-discard advisory at `off`, `info`, `warning`, and `error` policy levels;
- explicit discard suppressing the implicit-discard advisory;
- `void` call as an ordinary statement;
- redundant explicit discard of `void`;
- standalone `Result[T, E]` rejection;
- explicit discard of discardable `Result[T, E]`;
- rejection of `Result` whose possible active payload is non-discardable;
- implicit discard of ordinary discardable `Option[T]`;
- rejection of non-discardable `Option[T]`;
- discard of `ref` ending the holder but not destroying the referent;
- discard of `ref mut` ending the exclusive holder;
- rejection of owned-value discard while incompatible borrow is live;
- lifecycle-handle discard rejection;
- spawn-result discard rejection;
- direct field discard where partial-state rules permit it;
- aggregate destruction skipping an already discarded field;
- branch-sensitive discard state;
- defer-retained value preventing early discard;
- useless literal discard advisory;
- `discard _` rejection;
- Semantic IR explicit-discard provenance;
- Semantic IR implicit-call-result provenance;
- no double destruction after discard;
- new cleanup registration after legal reinitialization.

---

## 41. Related rulebooks

Detailed adjacent semantics remain owned by their specialized rulebooks, including:

```text
rules/types/types.md
rules/declarations/functions.md
rules/memory/ownership.md
rules/memory/copy_move.md
rules/memory/borrowing.md
rules/memory/references.md
rules/memory/destruction.md
rules/errors/errorhandling.md
rules/compiler/semantic_ir.md
rules/tooling/diagnostics.txt
rules/tooling/formatter.md
```

Concurrency and lifecycle rulebooks define the exact obligations for tasks, threads, processes, spawn, await, join, and detach.

---

## 42. Design summary

`discard expression` is an explicit terminal ownership operation.

It consumes the value, performs deterministic destruction when required, and makes consumed storage unavailable.

A mutable whole binding may later be reinitialized; an immutable one may not.

An ordinary discardable non-`void` call result may be implicitly discarded when the call is a standalone statement. That implicit loss is advisory-policy-controlled but semantically still a real discard and destruction action.

Must-use values cannot be implicitly discarded.

Explicit discard acknowledges intentional non-use only when the complete value is itself discardable.

`Result[T, E]` is must-use, while `Option[T]` is not automatically must-use.

Unresolved lifecycle handles are non-discardable.

Discardability is recursive through aggregates and variant payloads.

References and borrowed views may be discarded without destroying their referents.

The frontend determines discard legality and ownership effects before lowering. Semantic IR records the terminal ownership action explicitly. No backend stage may infer discard from an unused machine-level value.
