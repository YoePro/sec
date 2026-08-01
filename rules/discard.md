# Discard

## Current implementation status

Implemented:

- lexer keyword support for `discard`;
- parser support for `discard expression`;
- AST representation through `DiscardStatement`;
- compatibility `Name` field for identifier discards;
- parser rejection of `discard` without an expression;
- semantic analysis of the discarded expression;
- explicit discard of temporary expression results such as function and method
  calls;
- normal receiver mutability requirements for methods called by a discarded
  expression, including implicit `self` mutability inference in `impl` methods;
- identifier discard consumes the named binding and records the reason as
  `discarded`;
- later reads of a discarded local identifier are rejected with a related source
  location;
- ordinary assignment to a discarded local binding is rejected through the
  existing unavailable/moved-state handling;
- discard of `ref` and `ref mut` bindings ends the reference binding;
- discard rejects unresolved `Task[T]` and `Thread[T]` handles;
- discard rejects a direct `spawn` expression because successful creation would
  abandon the returned lifecycle handle;
- standalone expression statements are already parsed and semantically
  evaluated;
- standalone `spawn` expressions are rejected;
- an unhandled `Result` expression inside `defer` is rejected;
- parser, AST, and compiler dump output preserve the discarded expression.

Partially implemented:

- ordinary non-`void` call expressions are accepted as standalone statements
  and semantically treated as implicit temporary discards;
- direct compiler-known must-use results (`Result[T, E]`, `Task[T]`, and
  `Thread[T]`) are rejected when used as standalone call statements;
- must-use and explicit discardability checks recurse through type arguments,
  array elements, struct fields, and union payloads, preventing wrappers and
  aggregates from silently hiding unresolved tasks or threads;
- the current frontend validates explicit discard ownership state for simple
  local identifiers, but not arbitrary places;
- task and thread lifecycle checks exist, but the complete recursive
  discardability model is not yet implemented;
- diagnostics use current English messages, but most discard diagnostics do not
  yet use stable registered IDs, structured notes, and help.

Not implemented yet:

- the configurable advisory diagnostic for implicitly discarded ordinary call
  results;
- path-sensitive wrapper and union-variant reasoning that proves a specific
  active payload is discardable;
- field-sensitive partial discard;
- path-sensitive discarded-state diagnostics distinct from moved-state after
  branch merges;
- destructor lowering;
- Semantic IR `DiscardValue`;
- explicit Semantic IR origin metadata distinguishing explicit and implicit
  discard;
- non-discardable `Process` checks before that type is fully compiler-known;
- complete `spawn Result[Handle, Error]` modelling for thread and process spawn;
- advisory diagnostics for useless literal and constant discards;
- configurable severity integration for implicit discard;
- complete synchronization with `functions.txt`, `ownership.md`,
  `destruction.txt`, and `semantic_ir.txt`.

The normative rules below supersede the older rule that every non-`void` call
statement always requires explicit `discard`.

---

## Purpose

`discard` explicitly consumes a value whose result or remaining lifetime the
programmer intentionally does not need.

It is a general ownership and deterministic-destruction operation.

It is not specific to tasks, threads, error handling, or function results.

Sec also permits an ordinary non-`void` function or method result to be
implicitly discarded when the call is used as a standalone statement.

The compiler reports that implicit discard through a configurable advisory
diagnostic.

Values with must-use semantics are never implicitly discarded.

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

## Explicit and implicit discard

Sec distinguishes:

```text
explicit discard
    the programmer writes `discard expression`

implicit discard
    an ordinary non-void call is used as a standalone statement
```

Both consume and destroy the returned temporary.

Only explicit discard may acknowledge a must-use result.

Example of explicit discard:

```sec
discard Calculate()
```

Example of implicit discard:

```sec
Calculate()
```

The second form is valid only when the returned value is ordinary and
discardable.

The compiler emits a configurable advisory diagnostic for the second form.

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

`discard` ends the binding's current value lifetime.

The binding is not reinitialized by ordinary assignment afterward under the
current binding-state rules.

A separate declaration may reuse the spelling only when permitted by the normal
scope and shadowing rules.

---

## Copyable values

An ordinary read of a copyable value may copy it.

`discard` is different.

```sec
let number := 42
discard number
```

This consumes the binding's owned value and ends its availability.

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

## Explicitly discarding an expression result

A function result may always be explicitly ignored when its type is
discardable:

```sec
discard Calculate()
```

The call executes normally.

This includes receiver effects. For example:

```sec
discard self.Advance()
```

requires a mutable implicit `self` when `Advance` mutates its receiver, just as
using the same call in another expression context would.

The returned temporary is consumed and destroyed before the next statement.

Explicit discard documents programmer intent and suppresses the implicit-discard
advisory diagnostic.

---

## Implicitly discarding an ordinary call result

An ordinary function or method call returning a discardable non-`void` value may
be used as a standalone statement:

```sec
Calculate()
lexer.NextToken()
buffer.WriteByte(value)
```

The call is evaluated normally.

Its returned temporary is then implicitly consumed and destroyed at the end of
the statement.

Conceptually:

```text
1. evaluate the receiver and arguments;
2. perform the call;
3. construct the returned temporary;
4. consume and destroy that temporary;
5. continue with the next statement.
```

No user-visible binding is created.

The backend must not treat implicit discard as merely an unused SSA result when
the return type requires destruction.

Implicit discard is intended to avoid boilerplate where the call's effects are
wanted but its ordinary return value is not.

It does not apply to must-use values.

---

## Expression-statement boundary

The implicit-discard rule is defined for call-like expression statements.

Initial call-like forms include:

```text
function call
method call
constructor or compiler-known call form that follows normal call semantics
```

Examples:

```sec
UpdateCache()
object.Refresh()
string.FromRuneArray(runes)
```

This rule does not make arbitrary value expressions meaningful statements.

Examples that remain invalid or receive their own diagnostics under expression
and operator rules include:

```sec
1 + 2
value
person.name
```

The parser may represent these as expression statements for recovery or later
semantic validation.

---

## Implicit-discard diagnostic

Every implicitly discarded ordinary non-`void` call result is eligible for the
advisory diagnostic:

```text
ownership.implicit-discarded-result
```

The diagnostic belongs to the configurable `A` family.

Its default severity is:

```text
info
```

It may be configured as:

```text
off
info
warning
error
```

A project that requires explicit handling of every non-`void` call result can
promote this single diagnostic to `error`.

Conceptual configuration:

```toml
[diagnostics.rules]
"ownership.implicit-discarded-result" = "error"
```

A strict diagnostics preset may also promote it.

No separate language-level `@option explicitDiscard` is required.

Changing this diagnostic's severity does not change ownership semantics.

It changes whether implicit syntax is accepted by the selected build policy.

When promoted to `error`, the programmer may satisfy the policy by:

```sec
discard Calculate()
```

or by using the returned value in another valid consuming context.

Suggested default diagnostic:

```text
info[A....]: return value of Calculate with type Measurement is implicitly discarded
```

Suggested help:

```text
use the value or write `discard Calculate()` to document the intent
```

The final numeric ID must be allocated in the central diagnostics registry and
must remain stable regardless of configured severity.

No implicit-discard advisory is emitted for a `void` call.

A must-use violation emits a mandatory semantic error instead of this advisory.

---

## void expressions

A `void` function call is already a normal standalone statement:

```sec
LogMessage()
```

Explicit discard is valid but unnecessary:

```sec
discard LogMessage()
```

The expression is evaluated and produces no payload to destroy.

The compiler should emit a configurable information diagnostic for redundant
explicit discard of `void`.

Suggested message:

```text
discard is unnecessary because LogMessage returns void
```

---

## Must-use values

A must-use value carries semantic information or an ownership obligation that
must not disappear implicitly.

Initial must-use categories include:

```text
Result[T, E]
unresolved Task[T]
unresolved Thread[T]
unresolved Process
spawn results whose successful payload contains an unresolved lifecycle handle
other types explicitly defined as carrying a must-use obligation
```

A standalone expression producing a must-use value is a mandatory semantic
error.

The diagnostic must explain:

- the exact result type;
- why it is must-use;
- what obligation would otherwise be hidden;
- which valid handling forms are available;
- whether explicit `discard` is permitted.

Must-use is not a configurable advisory rule.

It cannot be disabled, demoted, or suppressed.

---

## Result

`Result[T, E]` is always must-use.

Invalid:

```sec
TryWriteLog()
```

Reason:

> An implicit discard could hide the `Err(E)` variant.

The compiler must issue a mandatory error.

Suggested diagnostic:

```text
error[S....]: result of TryWriteLog has type Result[void, IOError] and must be handled explicitly
```

Suggested note:

```text
Result may contain IOError; implicit discard would hide the failure
```

Suggested help:

```text
handle it with try or match, bind it, return it, or write `discard TryWriteLog()` to explicitly ignore both variants
```

Explicit discard acknowledges that both success and error payloads are
intentionally ignored:

```sec
discard TryWriteLog()
```

This satisfies must-use analysis only because the programmer wrote `discard`
explicitly.

The compiler must not infer this acknowledgement.

Discarding a `Result` destroys the active variant payload according to ordinary
rules.

Explicit discard is valid only when every possible active payload is itself
discardable.

For example, a `Result[Thread[T], E]` is not discardable while the successful
variant would contain an unresolved thread handle.

---

## Option

`Option[T]` is not automatically must-use merely because it is an option.

A standalone call returning an ordinary discardable `Option[T]` may therefore
be implicitly discarded and receive the normal advisory diagnostic.

Explicit discard destroys the contained `T` when the value is `Some`.

`None` requires no payload destruction.

Discardability is recursive.

If `T` carries a non-discardable obligation, then `Option[T]` is also
non-discardable while that obligation may be active.

Example:

```text
Option[Thread[void]]
```

cannot be discarded while `Some` may contain an unresolved thread.

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

An implicitly discarded temporary guard or subscription is permitted only when
the returned type is not defined as must-use.

An API should mark such a type must-use when immediate destruction would almost
always defeat the operation's purpose.

The exact declaration mechanism for user-defined must-use types is deferred to
the attribute and type rules.

---

## Execution handles

Unresolved lifecycle handles are not discardable.

Invalid:

```sec
discard task
discard thread
discard process
```

Also invalid:

```sec
StartTask()
StartThread()
```

when those calls return unresolved handles.

Reason:

> Destruction or implicit discard must not silently abandon an unresolved
> execution entity.

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

It is not parsed as:

```sec
discard worker
```

The mandatory diagnostic must name the lifecycle obligation.

Suggested message:

```text
error[S....]: Thread[int] must be joined, detached, returned, or transferred; it cannot be discarded
```

---

## Spawn results

Spawn is fallible.

Conceptually:

```sec
spawn thread Work()
```

produces:

```text
Result[Thread[T], SpawnError]
```

Discarding the complete spawn result is invalid because the `Ok` variant would
contain an unresolved lifecycle handle.

Invalid:

```sec
discard spawn thread Work()
```

Implicit discard is also invalid:

```sec
spawn thread Work()
```

The caller must match, propagate with `try`, bind, return, or otherwise resolve
both:

- creation failure;
- successful handle ownership.

The diagnostic must not suggest `discard` because explicit discard is not valid
for this result type.

Suggested help:

```text
handle the Result and then await, join, detach, return, or transfer the created thread
```

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

Implicit discard applies only when a call returns the value as an unnamed
temporary.

It does not implicitly consume an existing stored member merely because the
programmer reads it as a standalone expression.

---

## Partial values

Discarding a complete aggregate destroys all still-initialized fields.

Discarding a field consumes and destroys that field.

The containing aggregate becomes partially initialized when partial move rules
permit it.

The compiler must not later destroy an already discarded field.

Field-sensitive tracking is not yet implemented.

Collection element removal or extraction follows the collection API and
ownership rules rather than arbitrary partial discard through runtime indexing.

---

## Constants and literals

Discarding a literal or compile-time constant is valid but normally useless:

```sec
discard 42
```

The compiler should emit a configurable advisory diagnostic because no owned
resource or effect is consumed.

Suggested default severity:

```text
info
```

Suggested message:

```text
discard of literal int has no observable effect
```

The statement may lower to no code.

---

## Unused variables

Unused local variables are compiler errors under Sec's diagnostics rules.

Explicit discard documents intentional non-use:

```sec
let result := Calculate()
discard result
```

A declaration immediately followed by discard may receive a configurable style
diagnostic when the expression could be discarded directly:

```sec
discard Calculate()
```

The semantic program remains valid.

This rule is distinct from implicit discard of an unnamed call result.

Example:

```sec
Calculate()
```

creates no local binding and therefore does not trigger the unused-local rule.

It may trigger the implicit-discard advisory.

---

## Relationship to underscore

`discard` does not replace the context-specific meanings of `_`.

The underscore remains distinct for:

- match ignore patterns;
- register reserved bits;
- loop or pattern discard positions defined by their rulebooks;
- visibility and scope naming rules;
- any separately defined declaration discard syntax.

Bare `_` must not be reused as a general destruction statement.

Invalid:

```sec
discard _
```

because `discard` requires an expression identifying or producing a value.

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

An implicitly discarded temporary does not create a binding state that survives
the statement.

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

A `Result` produced inside a deferred block remains must-use.

Invalid:

```sec
defer {
    TryClose(resource)
}
```

Valid when the programmer intentionally ignores both variants:

```sec
defer {
    discard TryClose(resource)
}
```

or when the result is otherwise handled.

---

## Function return values

A non-`void` function or method result must enter one of these contexts:

```text
binding
assignment
argument
return
try
match
explicit discard
implicit discard of an ordinary standalone call result
another type-specific consuming context
```

Ordinary result:

```sec
Calculate()
```

is valid and receives the configurable implicit-discard advisory.

Must-use result:

```sec
TryCalculate()
```

is invalid when `TryCalculate` returns `Result[T, E]`.

Explicit acknowledgement:

```sec
discard TryCalculate()
```

is valid only when the complete `Result` type is discardable.

A `void` call remains a normal statement without discard:

```sec
LogMessage()
```

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
Result[Handle, E] when Handle is an unresolved lifecycle handle
Option[Handle] when Handle is an unresolved lifecycle handle
an aggregate containing a non-discardable active field
```

A type-specific rule may add another obligation.

The compiler derives discardability recursively from type semantics.

Conceptually:

```text
discardable scalar
    yes

discardable aggregate
    only when every still-initialized owned field is discardable

Option[T]
    only when T is discardable

Result[T, E]
    only when both T and E are discardable

union
    only when every possible active owned payload is discardable, unless
    control-flow analysis proves a specific discardable active variant
```

Must-use and discardability are related but distinct:

```text
must-use
    implicit discard is forbidden

discardable
    explicit discard is permitted
```

Examples:

```text
Result[int, IOError]
    must-use
    explicitly discardable

Thread[int]
    must-use
    not discardable while unresolved

Result[Thread[int], SpawnError]
    must-use
    not discardable while Ok may contain an unresolved thread
```

---

## User-defined must-use types

The language should permit a core or user-defined nominal type to declare that
its values are must-use.

The exact declaration syntax is not defined by this rulebook.

It belongs to:

```text
attributes.md
types.txt
```

The semantic requirement is already fixed:

- a must-use value cannot be implicitly discarded;
- a mandatory diagnostic explains the obligation;
- explicit discard is accepted only if the type is also discardable.

Core types such as `Result[T, E]`, `Task[T]`, and `Thread[T]` may be
compiler-known must-use types before general user syntax exists.

---

## Panic and errors during destruction

`discard` uses the normal destruction model.

It does not introduce a separate error-return path.

If destruction may panic or fail, the owning type's destruction rule must define
that behavior.

`discard` must not silently suppress destruction failure.

The same rule applies to the destruction of an implicitly discarded temporary.

The future panic and destruction rulebooks must define:

- panic during explicit discard;
- panic during implicit temporary destruction;
- double panic;
- destruction failure across FFI;
- profile-specific abort or unwind behavior.

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

Ordinary standalone call expressions remain `ExpressionStatement`.

The parser does not decide whether the expression's result is:

- ordinary;
- must-use;
- discardable;
- non-discardable.

That is semantic analysis.

---

## AST

Conceptual explicit-discard AST:

```text
DiscardStatement
    Token
    Value Expression
```

The current AST also retains a compatibility identifier field:

```text
Name *Identifier
```

for simple identifier discards.

New semantic logic should use `Value` as the canonical operand.

Conceptual implicit-discard AST remains:

```text
ExpressionStatement
    Expression CallExpression
```

The AST does not need a separate source node for implicit discard.

Sema and Semantic IR attach the validated discard meaning.

---

## Semantic analysis

For explicit discard, Sema must determine:

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

For standalone call expressions, Sema must determine:

- whether the expression is call-like;
- return type;
- whether the return type is `void`;
- whether the result is must-use;
- whether the result is discardable;
- whether implicit discard is valid;
- which advisory diagnostic policy applies;
- required temporary destruction;
- receiver and argument effects.

When an explicit operand names owned storage, the original storage becomes
uninitialized or unavailable.

When an explicit or implicit operand is a temporary, the temporary is destroyed
before the next statement.

Mandatory ordering:

```text
1. infer and validate the call;
2. reject invalid call semantics;
3. determine must-use status;
4. reject implicit must-use loss;
5. determine discardability;
6. reject non-discardable explicit or implicit consumption;
7. emit configurable implicit-discard advisory when applicable;
8. record destruction and ownership effects.
```

A mandatory must-use error must be emitted even when the implicit-discard
advisory is configured `off`.

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
- whether the source was a temporary, binding, field, or another place;
- resulting initialization state;
- discard origin.

Discard origin is:

```text
Explicit
ImplicitCallStatement
```

Conceptual form:

```text
DiscardValue {
    value
    type
    destruction
    origin
    sourceLocation
}
```

A lifecycle-specific detach discard remains a distinct operation:

```text
TaskDetachDiscard
ThreadDetachDiscard
```

The backend must not infer semantic discard merely from an unused SSA value.

An ordinary trivially destructible implicit result may later optimize to no
machine instruction after its validated semantic discard is recorded.

---

## Diagnostics

All primary discard diagnostics require stable registered IDs.

Diagnostic IDs identify the rule, not the configured severity.

### Implicit ordinary result

Symbolic name:

```text
ownership.implicit-discarded-result
```

Classification:

```text
configurable advisory
default severity: info
allowed: off, info, warning, error
```

Example:

```text
info[A....]: return value of NextToken with type Token is implicitly discarded
help: use the value or write `discard NextToken()` to document the intent
```

### Unhandled must-use result

Classification:

```text
mandatory semantic error
```

Example:

```text
error[S....]: result of TryWriteLog has type Result[void, IOError] and must be handled explicitly
note: Result may contain IOError; implicit discard would hide the failure
help: use try, match, binding, return, or explicit discard
```

### Non-discardable lifecycle value

Classification:

```text
mandatory semantic error
```

Example:

```text
error[S....]: cannot discard unresolved Task[int]
note: ordinary destruction must not abandon an unresolved task
help: await, join, detach, return, or transfer it explicitly
```

### Non-discardable spawn result

Example:

```text
error[S....]: cannot discard Result[Thread[void], SpawnError]
note: the Ok variant would contain an unresolved Thread[void]
help: handle the Result and resolve the created thread lifecycle
```

The help must not recommend `discard` when explicit discard is invalid.

### Use after explicit discard

Example:

```text
error[S....]: value resource was discarded here and is no longer available
```

with a related location pointing to the discard statement.

### Borrow conflict

Example:

```text
error[S....]: cannot discard value while it is borrowed by view
```

### Partial value

Example:

```text
error[S....]: field payload has already been moved or discarded
```

### Useless explicit discard

Classification:

```text
configurable advisory
default severity: info
```

Example:

```text
info[A....]: discard of literal int has no observable effect
```

### Redundant void discard

Classification:

```text
configurable advisory
default severity: info
```

Example:

```text
info[A....]: discard is unnecessary because LogMessage returns void
```

---

## Diagnostic configuration

The general diagnostics configuration model governs advisory discard findings.

Example:

```toml
[diagnostics.rules]
"ownership.implicit-discarded-result" = "warning"
```

A project that wants the old explicit-everywhere behavior may use:

```toml
[diagnostics.rules]
"ownership.implicit-discarded-result" = "error"
```

A project that does not want the information may use:

```toml
[diagnostics.rules]
"ownership.implicit-discarded-result" = "off"
```

Must-use and ownership-safety errors are mandatory and cannot be configured
away.

The `strict` diagnostics preset may promote the advisory, but `strict` does not
change the Sec language rule.

This preserves one language semantics across builds while allowing different
review policies.

---

## Required tests

Required valid cases include:

```sec
fn Calculate() int {
    return 1
}

fn Run() void {
    Calculate()
    discard Calculate()
}
```

Required configurable-diagnostic cases include:

```sec
Calculate()
```

tested with:

```text
off
info
warning
error
```

The diagnostic ID must remain unchanged across severities.

Required must-use invalid case:

```sec
fn TryCalculate() Result[int, Error] {
    return Ok(1)
}

fn Run() void {
    TryCalculate()
}
```

Expected:

```text
mandatory error explaining Result and the hidden error path
```

Required explicit acknowledgement:

```sec
discard TryCalculate()
```

Required non-discardable wrapper case:

```sec
discard spawn thread Work()
```

Expected:

```text
mandatory error because Ok contains an unresolved thread
```

Existing tests for:

- identifier discard;
- use after discard;
- assignment after discard;
- reference discard;
- task discard;
- thread discard;
- spawn discard;
- receiver mutability;
- parser dump output;

must remain valid.

---

## Required synchronization

This replacement requires updates to:

```text
lexical_structure.md
types.txt
functions.txt
copy_move.md
ownership.md
borrowing.txt
references.txt
lifetime_analysis.txt
destruction.txt
defer.txt
errorhandling.txt
diagnostics.txt
semantic_ir.txt
compiler_pipeline.txt
formatter.md
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
language-rulebook-status.md
```

Required key changes include:

- `types.txt`: define must-use, discardability, and unavailable state;
- `functions.txt`: permit implicit discard of ordinary standalone call results
  and require handling of must-use results;
- `copy_move.md`: distinguish copy, move, explicit discard, and implicit
  temporary discard;
- `destruction.txt`: define destruction of explicit and implicit discarded
  values;
- `ownership.md`: add discard as ownership consumption and define unnamed
  returned temporaries;
- `borrowing.txt`: reject discard while incompatible borrows are live;
- `errorhandling.txt`: define mandatory handling and explicit discard of
  `Result[T, E]`;
- `diagnostics.txt`: register the advisory and mandatory discard-related rules;
- `semantic_ir.txt`: add `DiscardValue` with explicit/implicit origin;
- task, thread, and process rules: forbid ordinary discard of unresolved
  lifecycle handles;
- formatter: preserve canonical `discard expression` formatting;
- implementation tracker: reflect existing frontend support and remaining
  advisory, must-use, destruction, and IR work.

---

## Design summary

`discard expression` is an explicit ownership operation.

It consumes the value, performs deterministic destruction, and makes consumed
storage unavailable.

An ordinary non-`void` call may also be used as a standalone statement.

Its returned temporary is implicitly consumed and destroyed.

The compiler reports that implicit loss through a configurable information
diagnostic that may be disabled or promoted to warning or error.

`Result[T, E]`, unresolved execution handles, and other must-use values cannot be
implicitly discarded.

The compiler must explain both that explicit handling is required and why the
value carries that obligation.

Explicit `discard` acknowledges a must-use value only when the complete value is
itself discardable.

A result containing an unresolved lifecycle handle remains non-discardable even
when wrapped in `Result`, `Option`, a union, or another aggregate.
