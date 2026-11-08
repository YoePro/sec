# Discard

- **Status:** Normative
- **Created:** 2026-07-31
- **Last updated:** 2026-11-08 (stable chapter, subsection, and paragraph numbering); 2026-08-26
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/control-flow/discard.md`
- **Repository baseline reviewed:** `50655d3bc5d2e2596935dc5609cb1db1387e0cec`

---

## § 1 Purpose

**§ 1(1)** `discard` explicitly consumes a value whose result or remaining lifetime the programmer intentionally does not need.

**§ 1(2)** It is a general ownership and deterministic-destruction operation.

**§ 1(3)** It is not specific to tasks, threads, error handling, or function results.

**§ 1(4)** Sec also permits an ordinary non-`void` function or method result to be implicitly discarded when the call is used as a standalone statement.

**§ 1(5)** Values with must-use semantics are never implicitly discarded.

---

## § 2 Syntax

```text
discard-statement:
    discard expression
```

**§ 2(1)** Examples:

```sec
discard value
discard Calculate()
discard worker.Value
```

**§ 2(2)** `discard` is a statement.

**§ 2(3)** It does not produce a value.

**§ 2(4)** The operand expression is evaluated exactly once.

---

## § 3 Core meaning

**§ 3(1)** For:

```sec
discard value
```

**§ 3(2)** the compiler must:

1. evaluate and resolve `value`;
2. validate that explicit discard is legal for the resolved value;
3. consume the resulting value or place;
4. execute deterministic destruction when required;
5. end any ownership or borrow responsibility carried by that consumed value;
6. mark consumed storage unavailable when applicable;
7. continue with the next statement.

**§ 3(3)** For trivially destructible values, lowering may produce no machine instruction.

**§ 3(4)** The semantic consumption still exists.

**§ 3(5)** `discard` is not:

- an unused SSA result;
- an implicit copy followed by destruction of the copy;
- a move to another owner;
- a general exception or error-suppression mechanism;
- a replacement for lifecycle operations such as `await`, `join`, or `detach`.

---

## § 4 Explicit and implicit discard

**§ 4(1)** Sec distinguishes:

```text
explicit discard
    the programmer writes `discard expression`

implicit call-result discard
    an ordinary discardable non-void call result is unused because the call is a standalone statement
```

**§ 4(2)** Both forms consume the produced value and run its required deterministic destruction.

**§ 4(3)** Only explicit discard may acknowledge a must-use value.

**§ 4(4)** Explicit:

```sec
discard Calculate()
```

**§ 4(5)** Implicit ordinary call-result discard:

```sec
Calculate()
```

**§ 4(6)** The second form is valid only when the result is not must-use and is discardable.

---

## § 5 Discarding a binding

```sec
let resource := OpenResource()
discard resource
```

**§ 5(1)** After the statement, the current value of `resource` is unavailable.

**§ 5(2)** A later read, borrow, or move is invalid until a legal reinitialization occurs.
A second discard is a legal no-op because `discard` converges the place to
`Unavailable`; tooling may issue a low-priority redundancy advisory.

```sec
Use(resource)
```

**§ 5(3)** must be rejected.

**§ 5(4)** A diagnostic must identify the discard that made the value unavailable.

**§ 5(5)** If the incoming place is `ConditionallyAvailable`, discard ends/destroys the
value only on paths that still own it and leaves every outgoing path
`Unavailable`. Discardability and incompatible-borrow checks still apply before
this convergence commits.

**§ 5(6)** `discard` ends the lifetime of the binding's current value. It does not remove the binding name from scope.

---

## § 6 Reinitialization after discard

**§ 6(1)** Mutability and availability are separate.

**§ 6(2)** A mutable binding may be reinitialized after its previous value has been discarded:

```sec
let mut resource := OpenResource()
discard resource

resource = OpenResource()
Use(resource)
```

**§ 6(3)** The assignment after `discard` is reinitialization, not replacement.

**§ 6(4)** Therefore:

- no old destination value is destroyed;
- the new value becomes available;
- a new destruction responsibility is established for the new value;
- cleanup registration occurs at the reinitialization point when required.

**§ 6(5)** An immutable binding cannot be reinitialized after discard.

```sec
let resource := OpenResource()
discard resource
resource = OpenResource()
```

**§ 6(6)** is invalid.

---

## § 7 Copyable values

**§ 7(1)** An ordinary read of a copyable value may copy it.

**§ 7(2)** `discard` is different.

```sec
let number := 42
discard number
```

**§ 7(3)** consumes the binding's current value and makes that binding unavailable.

**§ 7(4)** The compiler must not implement explicit discard of a binding as:

```text
copy binding
destroy copy
leave original binding available
```

**§ 7(5)** That would change the source semantics.

---

## § 8 Move-only values

**§ 8(1)** Discarding a move-only value consumes it exactly once.

```sec
let file := OpenFile()
discard file
```

**§ 8(2)** The value's deterministic destruction executes according to its type.

**§ 8(3)** The source becomes unavailable.

**§ 8(4)** No explicit `move(...)` syntax is required for discard.

---

## § 9 Explicitly discarding an expression result

**§ 9(1)** A function or method result may be explicitly ignored when the complete result type is discardable:

```sec
discard Calculate()
```

**§ 9(2)** The call executes normally.

**§ 9(3)** Receiver, argument, ownership, borrow, error, and effect checks are unchanged by the surrounding `discard`.

**§ 9(4)** For example:

```sec
discard self.Advance()
```

**§ 9(5)** still requires whatever receiver authority `Advance` normally requires.

**§ 9(6)** The produced temporary is consumed and destroyed before the next statement.

**§ 9(7)** Explicit discard documents programmer intent and does not trigger the implicit-discard advisory for that result.

---

## § 10 Implicitly discarding an ordinary call result

**§ 10(1)** An ordinary function or method call returning a discardable non-`void` value may be used as a standalone statement:

```sec
Calculate()
lexer.NextToken()
buffer.WriteByte(value)
```

**§ 10(2)** The call is evaluated normally.

**§ 10(3)** Its returned temporary is then implicitly consumed and destroyed at the end of the statement.

**§ 10(4)** Conceptually:

```text
1. evaluate receiver and arguments
2. perform the call
3. construct the returned temporary
4. validate implicit-discard legality
5. consume and destroy the temporary
6. continue with the next statement
```

**§ 10(5)** No user-visible binding is created.

**§ 10(6)** The backend must not treat a non-trivial returned value as merely an unused SSA result.

---

## § 11 Expression-statement boundary

**§ 11(1)** Implicit discard is defined for call-like standalone expression statements.

**§ 11(2)** Initial call-like forms include:

- ordinary function calls;
- method calls;
- constructors and compiler-known call forms that follow ordinary call semantics.

**§ 11(3)** Examples:

```sec
UpdateCache()
object.Refresh()
string.FromRuneArray(runes)
```

**§ 11(4)** This rule does not make arbitrary value expressions meaningful statements.

**§ 11(5)** Examples such as:

```sec
1 + 2
value
person.name
```

**§ 11(6)** do not become valid merely because a discarded-result mechanism exists.

**§ 11(7)** Their legality is defined by the ordinary expression-statement and unused-expression rules.

---

## § 12 Advisory for implicit ordinary results

**§ 12(1)** An implicitly discarded ordinary non-`void` call result is eligible for the configurable advisory:

```text
ownership.implicit-discarded-result
```

**§ 12(2)** Default severity:

```text
info
```

**§ 12(3)** Allowed policy levels:

```text
off
info
warning
error
```

**§ 12(4)** The diagnostic ID is stable regardless of configured severity.

**§ 12(5)** Promoting the advisory to `error` is a project diagnostic policy. It does not change Sec ownership semantics.

**§ 12(6)** A project requiring explicit acknowledgement may therefore require:

```sec
discard Calculate()
```

**§ 12(7)** instead of:

```sec
Calculate()
```

**§ 12(8)** No implicit-discard advisory is emitted for a `void` call.

**§ 12(9)** A must-use violation is a mandatory semantic error and is not replaced by this advisory.

---

## § 13 `void` expressions

**§ 13(1)** A `void` call is already a normal standalone statement:

```sec
LogMessage()
```

**§ 13(2)** Explicit discard remains legal:

```sec
discard LogMessage()
```

**§ 13(3)** The expression is evaluated and produces no payload to destroy.

**§ 13(4)** The compiler may emit a configurable advisory that the explicit discard is unnecessary.

---

## § 14 Must-use and discardability

**§ 14(1)** Must-use and discardability are distinct semantic properties.

```text
must-use
    implicit discard is forbidden

discardable
    explicit discard is permitted
```

**§ 14(2)** A value may therefore be:

- not must-use and discardable;
- must-use and discardable;
- must-use and non-discardable.

**§ 14(3)** A must-use value cannot disappear through implicit discard.

**§ 14(4)** Explicit discard acknowledges intentional non-use only when the complete value is discardable.

---

## § 15 Initial must-use categories

**§ 15(1)** Compiler-known must-use categories include at least:

```text
Result[T, E]
unresolved Task[T]
unresolved Thread[T]
unresolved Process, when Process is available
spawn results whose successful payload contains an unresolved lifecycle handle
other types explicitly defined as carrying a must-use obligation
```

**§ 15(2)** A standalone expression producing a must-use value is a mandatory semantic error.

**§ 15(3)** The diagnostic must explain:

- the exact result type;
- why the value is must-use;
- which obligation would otherwise be hidden;
- valid handling forms;
- whether explicit `discard` is legal.

**§ 15(4)** Must-use errors cannot be disabled or demoted by advisory configuration.

---

## § 16 `Result[T, E]`

**§ 16(1)** `Result[T, E]` is always must-use.

**§ 16(2)** This is invalid:

```sec
TryWriteLog()
```

**§ 16(3)** because implicit discard could hide `Err(E)`.

**§ 16(4)** Valid handling forms include:

- `try`;
- `match`;
- binding the result;
- returning it;
- passing it into another valid consuming context;
- explicit `discard` when the complete `Result` is discardable.

**§ 16(5)** Explicit acknowledgement:

```sec
discard TryWriteLog()
```

**§ 16(6)** means that both success and error variants are intentionally ignored.

**§ 16(7)** The compiler must not infer this acknowledgement.

**§ 16(8)** Discarding a `Result` destroys only the active initialized payload according to ordinary destruction rules.

**§ 16(9)** Explicit discard is legal only when every possible active payload that may occur at that point is discardable, unless control-flow analysis has already proven a more specific active state.

**§ 16(10)** Example:

```text
Result[int, IOError]
    must-use
    explicitly discardable

Result[Thread[int], SpawnError]
    must-use
    not discardable while Ok may contain an unresolved thread
```

---

## § 17 `Option[T]`

**§ 17(1)** `Option[T]` is not automatically must-use merely because it is an option.

**§ 17(2)** A standalone call returning an ordinary discardable `Option[T]` may therefore be implicitly discarded and receive the normal advisory.

**§ 17(3)** Explicit discard destroys the active `Some` payload according to ordinary destruction rules.

**§ 17(4)** `None` has no payload to destroy.

**§ 17(5)** Discardability is recursive.

**§ 17(6)** For example:

```text
Option[int]
    discardable

Option[Thread[void]]
    not discardable while Some may contain an unresolved thread
```

---

## § 18 Recursive discardability

**§ 18(1)** Discardability is derived from semantic obligations, not merely from representation.

**§ 18(2)** Conceptually:

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

**§ 18(3)** A type-specific rule may define another non-discardable obligation.

---

## § 19 References

**§ 19(1)** Discarding a reference value ends that reference holder.

**§ 19(2)** It does not destroy the referent.

```sec
let view := ref value
discard view
```

**§ 19(3)** Afterward:

- `view` is unavailable;
- `value` remains owned by its original owner;
- the borrow held only by `view` may end when the compiler proves no other holder retains it.

**§ 19(4)** Discarding an owned value while an incompatible live borrow exists is invalid.

---

## § 20 Mutable references

**§ 20(1)** Discarding `ref mut` ends that mutable reference holder.

**§ 20(2)** It does not destroy the referent.

**§ 20(3)** Because `ref mut` represents exclusive borrowed authority, ending the holder may release that exclusive borrow early when the compiler proves that no other retained value carries it.

---

## § 21 Slices and other non-owning views

**§ 21(1)** Discarding a non-owning reference-like view ends the view value and any borrow responsibility carried by that view.

**§ 21(2)** It does not destroy the backing elements merely because the view is discarded.

**§ 21(3)** The owning rulebook for the specific view type determines whether additional obligations exist.

---

## § 22 Guards and subscriptions

**§ 22(1)** Discard follows ordinary deterministic destruction.

**§ 22(2)** Therefore:

```sec
discard guard
```

**§ 22(3)** may release a guard early when the guard type's destruction releases the protected resource.

**§ 22(4)** Likewise:

```sec
discard subscription
```

**§ 22(5)** may close or unregister a subscription when its destruction semantics define that behavior.

**§ 22(6)** An API whose returned value should almost never be destroyed immediately should mark that type must-use when such a semantic classification is available.

---

## § 23 Execution handles

**§ 23(1)** An unresolved lifecycle handle is not discardable.

**§ 23(2)** Examples:

```sec
discard task
discard thread
```

**§ 23(3)** are invalid while those values still carry unresolved execution obligations.

**§ 23(4)** The owner must use a lifecycle operation permitted by the handle type, such as:

```text
await
join
detach
return or transfer ownership
```

**§ 23(5)** where applicable.

**§ 23(6)** Lifecycle-specific source forms such as:

```sec
detach worker discard
```

**§ 23(7)** are not equivalent to:

```sec
discard worker
```

**§ 23(8)** The former is governed by lifecycle rules and may explicitly define what happens to a future result.

---

## § 24 Spawn results

**§ 24(1)** Spawn is fallible.

**§ 24(2)** A successful spawn produces an unresolved lifecycle handle.

**§ 24(3)** Conceptually, a thread spawn may produce:

```text
Result[Thread[T], SpawnError]
```

**§ 24(4)** Task spawn analogously produces
`Result[Task[T], TaskSpawnError]`. Detach or detach-with-discard applies only to
the successful `Task[T]` after creation failure has been handled; it never
discards `TaskSpawnError`. `TaskOutcome[T]` receives no special implicit-discard
exemption.

**§ 24(5)** Therefore both of these are invalid:

```sec
discard spawn thread Work()
spawn thread Work()
```

**§ 24(6)** when the successful result would abandon an unresolved thread.

**§ 24(7)** The caller must resolve both:

- creation failure;
- successful handle ownership.

**§ 24(8)** A diagnostic for this case must not suggest explicit `discard`, because explicit discard is itself invalid.

---

## § 25 Stored lifecycle results

**§ 25(1)** After a lifecycle operation has resolved the execution obligation, the handle rulebook determines what stored result remains available and whether it is discardable.

**§ 25(2)** If a stored result is explicitly discarded:

```sec
discard worker.Value
```

**§ 25(3)** that place is consumed.

**§ 25(4)** A later access is invalid until a legal reinitialization exists under the containing type's rules.

**§ 25(5)** Implicit call-result discard never implicitly consumes an existing stored member merely because that member is read as a standalone expression.

---

## § 26 Aggregates and partial values

**§ 26(1)** Discarding a complete aggregate destroys every still-initialized owned field according to the aggregate's destruction plan.

**§ 26(2)** A direct field may be discarded only where the ownership and partial-state rules permit that field to be consumed independently.

```sec
discard object.payload
```

**§ 26(3)** may make the containing aggregate partially initialized.

**§ 26(4)** The compiler must then:

- mark the field unavailable;
- prevent later destruction of that already discarded field;
- reject complete-value use while the aggregate remains incomplete;
- permit reinitialization only where the general partial-reinitialization rules allow it.

**§ 26(5)** Runtime-indexed arbitrary partial discard of collection elements is not implied by this rule.

**§ 26(6)** Collection removal and extraction use the collection's explicit ownership API.

---

## § 27 Constants and literals

**§ 27(1)** Discarding a literal or compile-time constant is legal but normally has no useful semantic effect:

```sec
discard 42
```

**§ 27(2)** The compiler may emit a configurable advisory such as:

```text
discard of literal int has no observable effect
```

**§ 27(3)** The statement may lower to no machine code.

---

## § 28 Unused variables

**§ 28(1)** Unused local variables remain subject to the ordinary unused-local rules.

**§ 28(2)** Explicit discard documents intentional non-use:

```sec
let result := Calculate()
discard result
```

**§ 28(3)** A style advisory may suggest the direct form when equivalent:

```sec
discard Calculate()
```

**§ 28(4)** A standalone ordinary call:

```sec
Calculate()
```

**§ 28(5)** creates no local binding and therefore does not trigger the unused-local rule.

**§ 28(6)** It may trigger the implicit-discard advisory.

---

## § 29 Relationship to `_`

**§ 29(1)** `discard` does not replace the context-specific meanings of `_`.

**§ 29(2)** The underscore remains distinct for language contexts including:

- match ignore patterns;
- register reserved fields;
- loop discard binding positions;
- other explicitly defined pattern discard positions.

**§ 29(3)** Bare `_` is not a general destruction expression.

**§ 29(4)** This is invalid:

```sec
discard _
```

**§ 29(5)** because `discard` requires an expression that identifies or produces a value.

---

## § 30 Control-flow state

**§ 30(1)** Discard is path-sensitive.

```sec
if condition {
    discard value
} else {
    Use(value)
}
```

**§ 30(2)** After the merge, the compiler must reconcile the value's availability across all continuing paths.

**§ 30(3)** If the value is unavailable on any continuing path and no rule proves one compatible available state, later complete use is invalid.

**§ 30(4)** An implicitly discarded call-result temporary creates no binding state that survives the statement.

---

## § 31 Interaction with `defer`

**§ 31(1)** A registered `defer` may retain a future use of a binding or place.

**§ 31(2)** If an active defer still requires the value, an earlier discard is invalid.

**§ 31(3)** Conceptually:

```sec
defer {
    Close(resource)
}

discard resource
```

**§ 31(4)** must be rejected when the deferred operation requires the same owned value.

**§ 31(5)** A `Result` produced inside `defer` remains must-use.

**§ 31(6)** Therefore:

```sec
defer {
    TryClose(resource)
}
```

**§ 31(7)** is invalid when `TryClose` returns `Result` and the result is otherwise unhandled.

**§ 31(8)** Explicit acknowledgement is possible only when the complete result is discardable:

```sec
defer {
    discard TryClose(resource)
}
```

---

## § 32 Cleanup and destruction

**§ 32(1)** Explicit and implicit discard use the same resolved destruction machinery as normal lifetime end.

**§ 32(2)** For a non-trivial owned binding, explicit discard must conceptually:

1. execute the value's resolved destruction plan;
2. cancel the old pending cleanup responsibility for that consumed value;
3. mark the place unavailable;
4. prevent later automatic destruction of the consumed value.

**§ 32(3)** A later legal reinitialization creates a new destruction responsibility.

**§ 32(4)** This rule prevents double destruction.

**§ 32(5)** For aggregates:

- structs destroy still-initialized owned fields according to their destruction order;
- arrays destroy still-initialized elements according to their destruction order;
- unions destroy only the active initialized unmoved payload;
- `Result` destroys only the active `Ok` or `Err` payload;
- `Option` destroys only an active `Some` payload;
- references and borrowed slices do not destroy their referents.

---

## § 33 Destruction failure boundary

**§ 33(1)** `discard` introduces no independent error-return channel.

**§ 33(2)** If a type's destruction may panic, abort, fail, or require another policy, that behavior is defined by the destruction and panic rules for the type and target profile.

**§ 33(3)** `discard` must not silently suppress a failure that ordinary deterministic destruction would expose.

**§ 33(4)** No runtime `DiscardError` is introduced.

---

## § 34 Parser and AST requirements

**§ 34(1)** The parser recognizes:

```sec
discard expression
```

**§ 34(2)** as an explicit discard statement.

**§ 34(3)** Conceptual AST:

```text
DiscardStatement
    Token
    Value Expression
```

**§ 34(4)** The AST does not decide whether the operand is must-use, discardable, borrowed, owned, partial, or lifecycle-constrained.

**§ 34(5)** Those are semantic decisions.

**§ 34(6)** Ordinary standalone calls remain ordinary expression statements in the source AST.

**§ 34(7)** The source AST does not require a separate implicit-discard node.

---

## § 35 Semantic analysis requirements

**§ 35(1)** For explicit discard, Sema must determine at least:

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

**§ 35(2)** For standalone call expressions, Sema must determine at least:

- whether the expression is a permitted call-like statement;
- return type;
- whether the return is `void`;
- must-use classification;
- discardability;
- whether implicit discard is legal;
- applicable advisory policy;
- required destruction of the returned temporary;
- receiver and argument effects.

**§ 35(3)** Required validation ordering is conceptually:

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

**§ 35(4)** A mandatory semantic error must remain mandatory even when all related advisories are configured `off`.

---

## § 36 Semantic IR requirements

**§ 36(1)** Semantic IR must represent validated discard as an explicit terminal ownership action.

**§ 36(2)** Canonical conceptual operation:

```text
DiscardValue
```

**§ 36(3)** It records at least:

- consumed value or place;
- resolved type;
- destruction plan or trivial-destruction classification;
- source location;
- resulting availability state when a place is consumed;
- discard provenance.

**§ 36(4)** Discard provenance distinguishes at least:

```text
Explicit
ImplicitCallResult
CompilerTemporary
```

**§ 36(5)** `CompilerTemporary` describes compiler-created terminal temporary cleanup and is not a third source spelling of `discard`.

**§ 36(6)** Lifecycle-specific operations remain separate when their semantics are not ordinary discard, including detach-with-result-discard forms.

**§ 36(7)** The backend must not infer semantic discard merely from an unused SSA value.

---

## § 37 Diagnostics

**§ 37(1)** Primary discard diagnostics use stable registered IDs and symbolic names.

**§ 37(2)** Important diagnostic categories include:

### § 37.1 Implicit ordinary result

```text
ownership.implicit-discarded-result
```

**§ 37.1(1)** Configurable advisory, default `info`.

### § 37.2 Unhandled must-use result

**§ 37.2(1)** Mandatory semantic error.

**§ 37.2(2)** The diagnostic must explain the hidden obligation and valid handling forms.

### § 37.3 Non-discardable value

**§ 37.3(1)** Mandatory semantic error.

**§ 37.3(2)** The diagnostic must identify the obligation that makes explicit discard illegal.

### § 37.4 Use after discard

**§ 37.4(1)** Mandatory semantic error with a related location pointing to the discard.

### § 37.5 Discard while borrowed

**§ 37.5(1)** Mandatory semantic error identifying the conflicting borrow.

### § 37.6 Useless explicit discard

**§ 37.6(1)** Configurable advisory.

### § 37.7 Redundant `void` discard

**§ 37.7(1)** Configurable advisory.

**§ 37.7(2)** Diagnostic IDs identify rules, not severity.

---

## § 38 Diagnostic configuration

**§ 38(1)** Advisory discard diagnostics follow the general project diagnostic configuration model.

**§ 38(2)** Example:

```toml
[diagnostics.rules]
"ownership.implicit-discarded-result" = "warning"
```

**§ 38(3)** A project may configure the advisory as:

```text
off
info
warning
error
```

**§ 38(4)** Must-use, ownership-safety, borrow-safety, and lifecycle-obligation errors are mandatory and cannot be configured away.

---

## § 39 Formatter requirements

**§ 39(1)** Canonical explicit syntax is:

```sec
discard expression
```

**§ 39(2)** The formatter must preserve explicit `discard` and format its operand using ordinary expression formatting.

**§ 39(3)** Ordinary formatting must not insert or remove `discard` merely because a configurable advisory is enabled or disabled.

**§ 39(4)** Changing an ordinary standalone call into explicit discard is a semantic/documentation code action, not unconditional canonical formatting.

---

## § 40 Required language tests

**§ 40(1)** The language test suite must cover at least:

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

## § 41 Related rulebooks

**§ 41(1)** Detailed adjacent semantics remain owned by their specialized rulebooks, including:

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
rules/tooling/diagnostics.md
rules/tooling/formatter.md
```

**§ 41(2)** Concurrency and lifecycle rulebooks define the exact obligations for tasks, threads, processes, spawn, await, join, and detach.

---

## § 42 Design summary

**§ 42(1)** `discard expression` is an explicit terminal ownership operation.

**§ 42(2)** It consumes the value, performs deterministic destruction when required, and makes consumed storage unavailable.

**§ 42(3)** A mutable whole binding may later be reinitialized; an immutable one may not.

**§ 42(4)** An ordinary discardable non-`void` call result may be implicitly discarded when the call is a standalone statement. That implicit loss is advisory-policy-controlled but semantically still a real discard and destruction action.

**§ 42(5)** Must-use values cannot be implicitly discarded.

**§ 42(6)** Explicit discard acknowledges intentional non-use only when the complete value is itself discardable.

**§ 42(7)** `Result[T, E]` is must-use, while `Option[T]` is not automatically must-use.

**§ 42(8)** Unresolved lifecycle handles are non-discardable.

**§ 42(9)** Discardability is recursive through aggregates and variant payloads.

**§ 42(10)** References and borrowed views may be discarded without destroying their referents.

**§ 42(11)** The frontend determines discard legality and ownership effects before lowering. Semantic IR records the terminal ownership action explicitly. No backend stage may infer discard from an unused machine-level value.
