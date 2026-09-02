# Error handling

- **Status:** Normative
- **Created:** 2026-07-21
- **Last updated:** 2026-08-24
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/errors/errorhandling.md`
- **Replaces:** `rules/errors/errorhandling.txt`
- **Repository baseline reviewed:** `b3315f6` (latest semantic source/rulebook parent: `45e5cd4`)

---

## 1. Purpose

Sec uses explicit, statically typed error handling without hidden exceptions.

The core error-handling model consists of:

```text
error
Result[T, E]
Ok(value)
Err(errorValue)
try expression
try assignment
match expression
```

`Option[T]` represents ordinary optionality rather than an error, but `try` may
also use its compiler-known `Some(T)` / `None` short-circuit semantics.

The design goals are:

- public failure contracts are explicit;
- success and failure remain statically typed;
- propagation is visible in source through `try`;
- local recovery is concise;
- the compiler never invents domain-specific error wrapping;
- `match` remains the general exhaustive pattern-matching construct;
- `try` is specialized sugar for the common implicit-success / short-circuit case;
- ownership, destruction, and cleanup remain deterministic on every path;
- diagnostics explain error and ownership mistakes in ordinary programmer language.

`try` is not an exception mechanism and does not catch panic.

---

## 2. The compiler-known `error` root type

`error` is a compiler-known fundamental Sec type.

It requires no import and is written in lowercase:

```sec
error
```

`error` is the common root of Sec error types.

It is not:

- a general object base class;
- ordinary inheritance;
- an exception base class;
- a mandatory heap object;
- a fixed `Id` / `Description` structure;
- permission for arbitrary runtime type conversion.

The source-language relation is an error-specific subtype/assignability rule.

A concrete error value may widen to `error` while preserving its concrete error
identity, active variant, payload, ownership state, and destruction behavior.

The physical representation of `error` is not defined by this rulebook.

### 2.1 Error assignability

For a concrete error type `MyError`:

```text
MyError -> MyError    valid subject to ordinary value rules
MyError -> error      valid implicit widening
error -> MyError      not an implicit conversion
ErrorA -> ErrorB      not an implicit conversion
```

An explicit future narrowing operation, if added, must be defined separately.

### 2.2 No general inheritance

Declaring an error type does not permit arbitrary inheritance among user types.

This rule:

```text
concrete error type -> error
```

is specific to the Sec error model.

---

## 3. Declaring concrete error types

Sec 0.1 supports simple closed enum errors and payload-bearing union errors.

### 3.1 Enum errors

A fieldless closed error family may be declared as an enum:

```sec
enum IOError error {
    OpenError
    ReadError
    WriteError
}
```

The `error` marker follows the ordinary enum type information.

When an explicit enum underlying type is present, `error` remains the final
error marker before the body:

```sec
enum ProtocolError uint16 error {
    InvalidFrame = 1
    UnsupportedVersion = 2
}
```

The enum remains a closed nominal enum according to the enum rulebook.
Its additional property is that values are assignable to `error`.

### 3.2 Union errors with payload data

An error family that needs per-variant data may use a tagged union:

```sec
type DetailedError union error {
    OpenError {
        Path: string
        Code: int
    }

    ReadError {
        Path: string
        Offset: uint64
        Code: int
    }
}
```

The declaration order follows the ordinary Sec pattern:

```text
type
name
kind
kind-specific qualifiers
```

The final `error` marker declares the union as an error type.

Payload fields are chosen by the concrete error type. `error` does not impose
universal `Message`, `Id`, `Code`, or `Description` storage.

### 3.3 Error values may own data

An error payload may contain ordinary Sec values, including owned and move-only
values when the containing declaration permits them.

Error values follow normal:

- ownership;
- copying and moving;
- borrowing;
- destruction;
- discardability;
- lifetime rules.

There is no special hidden clone merely because a value is an error.

---

## 4. `Result[T, E]`

`Result` is a predeclared nominal generic type with exactly two type arguments:

```sec
Result[ValueType, ErrorType]
```

For:

```sec
Result[T, E]
```

- `T` is the success type;
- `E` is the declared error-channel type;
- `E` must be `error` or a concrete type declared as an error type;
- a value is either `Ok(T)` or `Err(E)`;
- when `T` is `void`, success is written `Ok()`.

Examples:

```sec
Result[Data, IOError]
Result[Data, error]
Result[void, ParseError]
```

Invalid:

```sec
Result[int]
Result[int, IOError, string]
Result[int, string]
```

The last form is invalid because `string` is not an error type.

### 4.1 Precise versus open error channels

Prefer a concrete error type when an API can state its failures precisely:

```sec
fn Read() Result[Data, IOError]
```

Use the root type when the API intentionally accepts or forwards heterogeneous
Sec errors:

```sec
fn RunPipeline() Result[Data, error]
```

`Result[T, IOError]` is a closed, precise error channel.

`Result[T, error]` is an open error channel. Widening into it must preserve the
concrete error identity and payload.

---

## 5. `Ok` and `Err`

`Ok` and `Err` are language-level constructors for `Result` states.

Examples:

```sec
return Ok(value)
return Ok()
return Err(IOError.ReadError)
```

Rules:

- `Ok(expr)` requires `expr` assignable to the declared success type;
- `Ok()` is valid only for `Result[void, E]`;
- `Err(expr)` requires `expr` assignable to the declared error type;
- concrete errors may therefore be returned from `Result[T, error]` by normal
  error widening;
- a plain success value does not generally become `Ok(value)` implicitly.

Example:

```sec
fn Read() Result[Data, error] {
    return Err(IOError.ReadError)
}
```

is valid because `IOError` is assignable to `error`.

---

## 6. `Result` success/error projections

`Result[T, E]` has compiler-known symmetric projections for intentionally
keeping one side and forgetting the other.

### 6.1 Consuming projections

```sec
result.Ok()
result.Err()
```

have types:

```text
Result[T, E].Ok()  -> Option[T]
Result[T, E].Err() -> Option[E]
```

Semantics:

```text
Ok(value).Ok()   -> Some(value)
Err(error).Ok()  -> None

Err(error).Err() -> Some(error)
Ok(value).Err()  -> None
```

Both operations consume an owned `Result` receiver.

They do not clone move-only payloads.

When the retained active payload is move-only, it is moved into the returned
`Option`.

The non-retained active payload is destroyed according to ordinary deterministic
destruction and discardability rules.

If the non-retained active payload may carry a non-discardable obligation, the
projection is invalid unless control-flow analysis proves that state unreachable.

Example:

```sec
let result := Read()
let value := result.Ok()

Use(result)
```

The final use is invalid because `result.Ok()` consumed `result`.

### 6.2 Non-consuming borrowed projections

A caller that only wants to inspect one side without consuming the `Result` uses
compiler-known read-only properties:

```sec
result.OkRef
result.ErrRef
```

Their types are:

```text
Result[T, E].OkRef  -> Option[ref T]
Result[T, E].ErrRef -> Option[ref E]
```

Semantics:

```text
Ok(value).OkRef   -> Some(ref value)
Err(error).OkRef  -> None

Err(error).ErrRef -> Some(ref error)
Ok(value).ErrRef  -> None
```

These properties:

- do not consume the `Result`;
- do not copy or clone a payload;
- create an ordinary shared borrow when the requested payload is active;
- retain the receiver's normal lifetime and borrow restrictions.

A mutable borrowed projection is not defined by this revision.

### 6.3 Choosing between consuming and borrowed inspection

Use consuming projection when the complete `Result` is no longer needed:

```sec
if Clear().Err() is Some(error) {
    Handle(error)
}
```

Use borrowed projection or borrowed `match` when the original result must remain
available:

```sec
let result := Read()

if result.ErrRef is Some(error) {
    Inspect(error)
}

Use(result)
```

---

## 7. `Result` is must-use

`Result[T, E]` is always must-use.

A bare standalone `Result`-producing call cannot silently disappear.

Invalid:

```sec
TryWriteLog()
```

Valid handling includes:

```text
try
match
binding the Result
returning it
passing it to a valid consuming context
explicit discard when the complete Result is discardable
.Ok() or .Err() when intentionally selecting one side
```

Explicit discard means that both success and failure are intentionally ignored:

```sec
discard TryWriteLog()
```

Its legality is defined by `discard.md`, including recursive discardability of
both possible active payloads.

---

## 8. The `try` model

`try` is a language-level short-circuit expression form.

Conceptually it is specialized sugar over the resolved success/alternate states
of the protected expression, but the compiler need not literally lower it to a
source-level `match`.

`try` provides:

- an implicit success path;
- optional local handling of alternate/failure states;
- implicit propagation of unhandled alternate/failure states where the
  enclosing return channel is compatible.

`match`, by contrast, is explicit and exhaustive.

---

## 9. Operands accepted by `try`

`try` is not limited to expressions whose surface type is `Result[T, E]`.

The operand must be a language-defined fallible or short-circuit expression for
which Sema can resolve:

- a success type;
- one or more possible alternate/failure states;
- the control-flow action required for propagation.

Sec 0.1 includes at least:

1. `Result[T, E]` expressions;
2. `Option[T]` expressions;
3. language-defined fallible runtime checks;
4. fallible conversions and constrained constructions;
5. fallible property assignments;
6. fallible lifecycle construction through `new`.

Examples:

```sec
let data := try Read()
let item := try Find()
let sum := try left + right
let value := try values[index]
let percent := try Percent(raw)
let connection := try new Connection(address)
```

An ordinary infallible value is not a valid `try` operand merely because its type
could theoretically be wrapped in `Result` or `Option`.

---

## 10. Success and alternate states

### 10.1 `Result[T, E]`

For a `Result[T, E]` operand:

```text
Ok(value) -> success value T
Err(error) -> error alternate
```

### 10.2 `Option[T]`

For an `Option[T]` operand:

```text
Some(value) -> success value T
None        -> absence alternate
```

`None` is not an error and does not become one.

### 10.3 Language-defined fallible operation

A fallible operation has a compiler-known success type and one or more declared
failure sources as defined by its owning rulebook.

The compiler must not require materializing a temporary `Result` value when the
operation can lower directly to checked control flow.

---

## 11. Compiler-internal failure sets

One protected expression may contain multiple fallible points.

Example:

```sec
let value := try values[index] + amount
```

The index operation and arithmetic operation may have different concrete error
types.

The compiler may track an internal failure set such as:

```text
BoundsError
ArithmeticError
```

This is analysis information only.

It does not create:

- an anonymous source-language union;
- an inferred public error type;
- implicit wrapping into a user union;
- automatic variant selection.

Sequential failures are not aggregated. Evaluation stops at the first failure
according to ordinary source evaluation order.

### 11.1 Binding a heterogeneous failure set

`Err(_)` may handle a heterogeneous protected failure set because it introduces
no payload binding.

A catch-all `Err(errorValue)` binding requires one compiler-resolved binding type.
It is valid when the remaining failure paths already share one concrete or
explicitly declared common error channel, including an explicit `error` channel.

The compiler must not infer `error` merely to make a heterogeneous local binding
possible when no such common channel exists in the protected construct.

Programmers may instead use `Err(_)`, split the expression, or map failures
explicitly.

---

## 12. Naked `try` propagation

Syntax:

```text
try expression
```

### 12.1 Result/error propagation

For an unhandled error path, each propagated error must be assignable to the
enclosing function's declared `Result` error type.

Example:

```sec
fn ReadData() Result[Data, IOError] {
    let data := try ReadFile()
    return Ok(data)
}
```

Widening to `error` is valid:

```sec
fn Run() Result[Data, error] {
    let data := try ReadFile()
    return Ok(data)
}
```

No hidden wrapper is created.

### 12.2 Option propagation

For `Option[T]`, naked `try` propagates `None` only through an enclosing
compatible `Option[...]` return channel.

Example:

```sec
fn Resolve() Option[Device] {
    let device := try FindDevice()
    return Some(device)
}
```

`None` remains ordinary absence.

There is no implicit mapping:

```text
None -> Err(...)
Err(...) -> None
```

### 12.3 No arbitrary cross-channel conversion

The compiler must never invent a mapping between `Result` failure and `Option`
absence.

A programmer who wants such a conversion must state it explicitly.

---

## 13. `try` expression boundaries

`try` applies to the complete following expression until its natural grammatical
boundary.

Canonical:

```sec
let total := try left + right * quantity
```

Conceptual grouping:

```sec
let total := try (left + (right * quantity))
```

Parentheses may narrow the protected region:

```sec
let total := left + (try right * quantity)
```

Typical boundaries include:

```text
statement completion
comma separating arguments or elements
closing parenthesis
closing bracket
closing brace
match/select arrow
local try-handler block
```

Evaluation order is unchanged. The first encountered failure short-circuits the
protected expression.

---

## 14. `try` as a general expression

The success value of `try` may appear wherever that success type is legal.

Examples:

```sec
let value := try Read()
```

```sec
Process(try Read())
```

```sec
if try IsReady() {
    Proceed()
}
```

```sec
let item := Container {
    Value: try ReadValue()
}
```

```sec
let result := Calculate(try ReadLeft(), try ReadRight())
```

Assignment remains a statement in Sec 0.1. `try` does not make assignment a
value-producing expression.

Therefore this remains invalid:

```sec
if result = try Setup() {
    Proceed()
}
```

Write the assignment and condition separately.

---

## 15. Local `try` handlers

A `try` expression may add a local handler block.

For Result/error handling:

```sec
let config := try LoadConfig() {
    Err(IOError.NotFound) => DefaultConfig()
}
```

For Option absence:

```sec
let device := try FindDevice() {
    None => DefaultDevice()
}
```

The handler list describes only alternate/failure states.
The success path is implicit.

### 15.1 Success handlers are forbidden

A Result `try` handler block must not contain an explicit `Ok(...)` arm.

An Option `try` handler block must not contain an explicit `Some(...)` arm.

Use `match` when both success and alternate states should be written explicitly.

### 15.2 No nested `match` wrapper syntax

This obsolete form is invalid:

```sec
try Read() {
    match {
        Err(errorValue) => Handle(errorValue)
    }
}
```

Write:

```sec
try Read() {
    Err(errorValue) => Handle(errorValue)
}
```

---

## 16. Partial handlers and implicit propagation

A `try` handler block is intentionally partial.

Unmatched alternate/failure states continue through normal `try` propagation.

Example:

```sec
let value := try Read() {
    Err(IOError.NotFound) => DefaultValue()
}
```

If `Read()` may produce other errors, they propagate implicitly when compatible
with the enclosing error channel.

This is the key distinction:

```text
try    partial handling; unmatched alternate/failure states propagate
match  exhaustive handling of the resolved subject
```

An explicit catch-all handles all remaining Result/error failures locally:

```sec
let value := try Read() {
    Err(_) => DefaultValue()
}
```

For `Option[T]`, handling `None` covers its only alternate state:

```sec
let value := try Find() {
    None => DefaultValue()
}
```

---

## 17. `Err(_)` and explicit error acknowledgement

`Err(_)` is valid.

```sec
try Read() {
    Err(_) => Recover()
}
```

It explicitly acknowledges the `Err` state while intentionally ignoring its
payload.

This is different from a generic match catch-all `_` that could silently hide an
unhandled Result error.

Conceptually, when the payload is discardable:

```sec
Err(_) => Recover()
```

has the same payload-lifetime intent as:

```sec
Err(errorValue) => {
    discard errorValue
    Recover()
}
```

`Err(_)` is valid only when discarding the matched payload is legal under the
ordinary discardability and ownership rules.

---

## 18. Handler order and reachability

Handlers are tested from top to bottom.

The first matching handler wins.

Example:

```sec
try Read() {
    Err(IOError.NotFound) => UseDefault()
    Err(IOError.PermissionDenied) => RequestAccess()
    Err(_) => Fallback()
}
```

Source order is semantically significant.

When the compiler can prove that a later handler is unreachable because an
earlier unguarded handler already covers it, the later handler is a compile
error.

Invalid:

```sec
try Read() {
    Err(_) => Fallback()
    Err(IOError.NotFound) => UseDefault()
}
```

The diagnostic must identify both the unreachable handler and the earlier
handler that covers it.

---

## 19. Guards

A handler guard uses the same canonical spelling as `match`:

```text
Pattern where condition => body
```

Example:

```sec
try Read() {
    Err(errorValue) where errorValue.Code == 404 => UseDefault()
    Err(errorValue) => Handle(errorValue)
}
```

Rules:

- the pattern is tested before the guard;
- the guard is evaluated only after pattern success;
- the guard must have type `bool`;
- pattern bindings are visible in the guard;
- when the guard is false, matching continues to the next handler;
- a guarded handler is not assumed to cover its complete underlying pattern;
- first matching pattern with a true guard wins.

A guard or handler body is outside the protected operation set of the same
`try`.

If guard or recovery code is itself fallible, it must use its own explicit
`try` where required.

---

## 20. Handler ownership and guards

Handler payload bindings follow the same copy/move/borrow principles as the
corresponding `match` patterns.

For a prospective move-only by-value binding with a guard, ownership transfer is
not committed merely because the pattern matched.

The move is committed only after:

1. the pattern matches;
2. the guard succeeds when present;
3. the handler is selected.

The guard may inspect or borrow the prospective binding according to normal
rules but must not consume it before selection when a later handler or
propagation path may still need the original value.

---

## 21. Recovery values

When `try expression { ... }` is used in value position, a locally handled
alternate/failure path that continues execution must produce a value assignable
to the protected expression's success type.

Example:

```sec
let config := try LoadConfig() {
    Err(IOError.NotFound) => DefaultConfig()
}
```

If `LoadConfig()` has success type `Config`, `DefaultConfig()` must also produce
a value assignable to `Config`.

A handler may instead leave control flow:

```sec
let config := try LoadConfig() {
    Err(IOError.PermissionDenied) => {
        return Err(AppError.ConfigurationDenied)
    }
}
```

Such a path does not need a recovery value.

### 21.1 Block handler result position

A handler block in value position may produce its recovery value from its final
expression, using the same contextual result-position principle as expression
`match` arm blocks:

```sec
let config := try LoadConfig() {
    Err(IOError.NotFound) => {
        Log("Using default configuration")
        DefaultConfig()
    }
}
```

This does not make arbitrary Sec blocks general expressions.

### 21.2 `void` success

When the protected operation has `void` success, a locally handled failure may
complete normally without manufacturing a value:

```sec
try ClearCache() {
    Err(CacheError.AlreadyEmpty) => {
        Log("Cache was already empty")
    }
}
```

---

## 22. Handler failures are outside the protected set

A handler does not recursively catch failures produced by its own recovery code.

Example:

```sec
let config := try LoadConfig() {
    Err(ConfigError.NotFound) => try LoadDefaultConfig()
}
```

The outer `try` protects `LoadConfig()`.
The inner `try` handles or propagates failures from `LoadDefaultConfig()`.

The same rule applies to fallible guard expressions.

Handler code should preferably be infallible when practical, but this is a
programming recommendation rather than a language restriction.

---

## 23. Fallible assignment

A fallible assignment uses `try`.

Naked propagation is valid:

```sec
try vehicle.TopSpeed = requestedSpeed
```

Local handling is also valid:

```sec
try vehicle.TopSpeed = requestedSpeed {
    Err(SpeedError.TooHigh) => {
        Log("Requested speed was too high")
    }
}
```

Rules:

- the target must resolve to a fallible assignment path;
- success completes the assignment and produces no value;
- failure leaves the destination according to the owning assignment/setter
  transactional rule;
- naked failure propagates when assignable to the enclosing error channel;
- local handlers use the same partial, top-to-bottom, guard, wildcard, and
  propagation rules as ordinary `try`;
- no explicit `Ok(...)` handler is permitted;
- a fallible assignment without `try` is invalid.

Assignment success is a normal success control-flow edge. It is not specified as
an artificial source-level `Ok(())` value.

---

## 24. Fallible property setters

A fallible property setter declares its error type explicitly on the setter
line:

```sec
property TopSpeed: Speed {
    try set value SpeedError {
        if value < MinimumSpeed {
            return Err(SpeedError.TooLow)
        }

        self._speed = value
    }
}
```

The error type is part of the property assignment contract and must be `error` or
a concrete error type.

Setter success is implicit:

```text
normal body completion -> success
return                 -> early success
return Err(errorValue) -> failure
```

`return Ok()` is invalid in a `try set` body.

The setter is not modeled in source as an ordinary function returning
`Result[void, E]`.

### 24.1 Interfaces

An interface that requires a fallible setter declares the same error type:

```sec
property Position: Point {
    try set value PositionError
}
```

A conforming implementation must satisfy that declared error contract according
to ordinary interface and error assignability rules.

### 24.2 Getters are infallible in Sec 0.1

Property getters are infallible in Sec 0.1.

There is no canonical `try get ErrorType` form in this revision.

A property whose read operation inherently requires a typed failure should use
an ordinary fallible function/method API instead.

---

## 25. Fallible initializers

Lifecycle initializer syntax is owned by `impl.md`.

Error handling recognizes the existing form:

```sec
impl Connection {
    init(address: Address) ConnectError {
        if invalid {
            return Err(ConnectError.InvalidAddress)
        }

        self.address = address
    }
}
```

The trailing type is the construction error type, not a success return type.

Successful normal completion produces the completed instance.

A failed initializer produces no completed instance and follows the lifecycle
cleanup rules for partially initialized state.

Construction uses ordinary `try`:

```sec
let connection := try new Connection(address)
```

This rulebook does not redefine lifecycle initialization semantics.

---

## 26. `return try expression`

Sec permits direct forwarding from a fallible expression in Result-return
position:

```sec
fn Load() Result[Config, IOError] {
    return try ReadConfig()
}
```

For an enclosing `Result[T, E]` function:

```text
protected success value S -> return Ok(value), when S is assignable to T
protected failure F       -> propagate, when F is assignable to E
```

This is a special return-position forwarding rule.

It does not create a general implicit conversion from `T` to `Result[T, E]`.

Outside this form, ordinary success returns remain explicit:

```sec
return Ok(value)
```

This revision does not define an additional implicit `Some(...)` return wrapping
rule for `Option`.

---

## 27. Relationship to `match`

`match` is the general exhaustive pattern-matching construct.

Use `try` when success should continue implicitly and only selected alternate or
failure cases need local handling.

Use `match` when all subject states should be explicit in source.

Example `try`:

```sec
let value := try Read() {
    Err(IOError.NotFound) => DefaultValue()
}
```

Example `match`:

```sec
match Read() {
    Ok(value) => Use(value)
    Err(_) => Fallback()
}
```

The general pattern grammar, copy/move binding rules, guard semantics, and match
exhaustiveness rules are owned by `flowcontrol_match.md`.

### 27.1 Patterns are checked against the resolved subject type

The compiler must know the resolved return/subject type before validating
variant patterns.

For an `Option[Device]`, `Some`/`None` patterns are valid and `Ok`/`Err` are not.

For a `Result[Data, IOError]`, `Ok`/`Err` patterns are valid and `Some`/`None` are
not.

Diagnostics must explain the mismatch and suggest patterns from the correct
carrier family.

### 27.2 Matching `Result[T, error]`

A value widened to `error` retains its concrete error identity.

Therefore a `Result[T, error]` may use error-specific narrowing patterns inside
its explicit `Err(...)` branch:

```sec
match result {
    Ok(value) => Use(value)
    Err(IOError.NotFound) => UseDefault()
    Err(errorValue) => Handle(errorValue)
}
```

This is an error-specific narrowing rule.
It does not introduce general runtime type patterns for arbitrary Sec values.

Because `error` is an open domain, a match over `Result[T, error]` must retain an
exhaustive error fallback such as `Err(errorValue)` or `Err(_)` unless control-flow
facts prove a narrower closed state.

### 27.3 Generic `_` may not hide `Err`

A generic match catch-all must not silently absorb an otherwise unhandled Result
error.

Use explicit `Err(_)` to state that the error branch is intentionally handled
without its payload.

---

## 28. `if` tests for `Option`

The canonical one-branch presence/absence forms include:

```sec
if Clear().Err() is not None {
    HandleFailure()
}
```

and the narrow payload-binding convenience:

```sec
if Clear().Err() is Some(errorValue) {
    Handle(errorValue)
}
```

`Some(binding)` in positive `is` position is the Sec 0.1 exception that permits
one-branch Option payload binding without requiring a full `match`.

The binding exists only inside the true branch.

This is invalid:

```sec
if option is not Some(value) {
    Use(value)
}
```

because there is no `value` on the true path.

General `if` semantics remain owned by `flowcontrol_if.md`.

---

## 29. Ownership and affine value semantics

Error handling never suspends ordinary Sec affine ownership rules.

The compiler must track consumption caused by:

- by-value `match` and `try` bindings;
- consuming `Result.Ok()` / `Result.Err()` projections;
- explicit `<-` moves;
- explicit `discard`;
- returning or forwarding owned values;
- ownership transfer into error payloads.

A value consumed on one path becomes unavailable according to the normal
path-sensitive ownership merge rules.

Use-after-consume is always a compile error when the compiler can prove it.

The check is semantic correctness, not an optional optimization or lint.

---

## 30. Diagnostics must act as a mentor

Error-handling and ownership diagnostics must explain programmer-visible cause
and consequence rather than exposing only compiler-theory terminology.

For a consumed Result:

```sec
let result := Read()
let value := result.Ok()
Use(result)
```

an appropriate diagnostic shape is:

```text
error: `result` cannot be used here because `result.Ok()` consumed it

`result.Ok()` keeps the success side as an Option and consumes the original Result.
The later use therefore has no value left to read.

help: use `result` before calling `.Ok()`, borrow it with `OkRef`, or restructure
      the code so the original Result is not needed afterwards
```

Diagnostics should identify when relevant:

1. the value or expression involved;
2. the operation that consumed, moved, discarded, or propagated it;
3. the source location of that operation;
4. why the current operation is invalid;
5. a practical source-level correction when one is known.

Terms such as "affine", "ownership lattice", "projection", or "path merge" may
appear as secondary technical detail but must not be required to understand the
primary diagnostic.

The LSP must surface the same compiler facts rather than reconstructing a
separate ownership or error model.

---

## 31. `try` does not catch panic

`try` converts only language-defined fallible/short-circuit behavior belonging to
its protected expression.

It does not catch, unwind, or resume arbitrary panic from a called function.

Example:

```sec
let result := try Calculate() + value
```

If `Calculate()` may panic independently, that panic effect remains unless a
separate rule proves it absent.

Error handling and panic remain separate Sec mechanisms.

---

## 32. Cleanup and `defer`

Error propagation is a normal control-flow exit and must execute the same
required deterministic cleanup as any other return path.

A handler or propagation edge must preserve:

- lexical cleanup;
- active defer execution;
- destruction of initialized locals;
- partial initialization state;
- borrow termination;
- moved/discarded state;
- lifecycle obligations.

`try` does not bypass cleanup.

A `Result` produced inside `defer` remains must-use and must be handled according
to the defer, panic, and discard rules.

---

## 33. Parser and AST requirements

The parser must represent `try` as a first-class expression form rather than a
special case tied only to `let` declarations.

Conceptually:

```text
TryExpression
    Token
    ProtectedExpression
    Handlers []TryHandler

TryHandler
    Pattern
    Guard optional
    Body
```

Fallible assignment may retain a dedicated statement/AST representation when
that better preserves assignment semantics.

The AST must preserve:

- source handler order;
- exact pattern spelling;
- optional `where` guard;
- handler body form;
- protected expression boundary;
- source locations for diagnostics.

The parser must reject or recover with a focused diagnostic for:

- explicit `Ok(...)` in a Result `try` handler;
- explicit `Some(...)` in an Option `try` handler;
- obsolete nested `match { ... }` wrapper inside `try`;
- missing `=>`;
- malformed guards;
- malformed fallible setter error-type syntax.

---

## 34. Sema requirements

Sema must resolve before validating a `try`:

- protected success type;
- carrier/operation kind;
- concrete protected failure/alternate set;
- error assignability for each unhandled error path;
- Option absence propagation compatibility;
- handler pattern compatibility;
- handler order and proven reachability;
- guard type and guard control flow;
- handler-local binding type;
- handler binding ownership action;
- recovery value type;
- all terminating versus continuing paths;
- ownership state after each continuing path;
- cleanup obligations;
- whether panic effects remain outside converted fallible checks.

For `Result[T, error]`, Sema must preserve the concrete error identity needed for
error-specific narrowing patterns.

Sema must not infer anonymous public error unions.

---

## 35. Semantic IR and lowering requirements

Semantic IR must preserve enough information that lowering does not need to
rediscover source semantics from physical Result layout or textual variant names.

It must represent or unambiguously preserve:

- protected expression evaluation order;
- success type;
- Result/Option/language-check carrier kind;
- compiler-internal failure set where applicable;
- concrete error identity before and after widening to `error`;
- source-ordered handler dispatch;
- guards and guard-false continuation;
- partial-handler unmatched propagation;
- recovery-value merge;
- binding copy/move/borrow action;
- ownership commit point after guard success;
- consuming Result projection;
- borrowed Result projection;
- fallible assignment success/failure edges;
- `return try` forwarding;
- cleanup and destruction edges;
- remaining panic effects.

A backend may use optimized direct branches, tagged representations, erased
error carriers, or target helpers as long as the observable Sec semantics remain
identical.

No general exception runtime is required.

---

## 36. LSP requirements

The LSP must consume resolved compiler facts and expose at least:

- `Result[T, E]` success and error types;
- `Option[T]` success and absence semantics;
- success type produced by `try`;
- unhandled failures/absence that will propagate;
- concrete-to-`error` widening;
- local handler coverage and remaining propagation;
- handler binding type and ownership mode;
- whether `.Ok()` / `.Err()` consumes the receiver;
- whether `OkRef` / `ErrRef` borrows the receiver;
- source location of a move/consume that makes later use invalid;
- fallible setter declared error type;
- initializer construction error type;
- correct carrier-family patterns in completion and diagnostics.

For example, when a programmer writes an `Ok` pattern against `Option[T]`, the
LSP should explain that the expression returns `Option[T]` and offer `Some` and
`None`, rather than reporting only an unknown or incompatible pattern.

---

## 37. Required tests

The implementation must include focused parser, Sema, ownership, control-flow,
Semantic IR, lowering, diagnostics, LSP, formatter, and end-to-end tests.

### 37.1 Error root and Result

Test:

```text
error is compiler-known and import-free
enum ErrorType error declaration
enum explicit-underlying ErrorType ... error declaration
type DetailedError union error declaration
concrete error implicitly widens to error
error does not implicitly narrow to a concrete error type
one concrete error does not implicitly convert to another
Result[T, concrete-error] accepted
Result[T, error] accepted
Result[T, non-error] rejected
concrete identity survives widening to error
payload survives widening to error
```

### 37.2 Result projections

Test:

```text
Ok().Ok() produces Some(success)
Err().Ok() produces None
Err().Err() produces Some(error)
Ok().Err() produces None
.Ok() consumes an owned Result
.Err() consumes an owned Result
use after .Ok() rejected with source consume location
use after .Err() rejected with source consume location
move-only retained payload moves into Option
non-retained payload is destroyed exactly once
non-discardable alternate payload prevents unsafe consuming projection
OkRef returns Option[ref T] without consuming
ErrRef returns Option[ref E] without consuming
borrowed projection lifetime cannot outlive Result
Result remains usable after borrowed projection borrow ends
```

### 37.3 General `try` expression positions

Test explicitly:

```text
let value := try Read()
Process(try Read())
if try IsReady() { ... }
struct field initializer with try
multiple call arguments each containing try
parenthesized narrowed try boundary
fallible language-defined arithmetic expression
fallible indexing expression
fallible constrained conversion
fallible new expression
assignment remains non-expression inside if
```

### 37.4 Option `try`

Test:

```text
try Option Some unwraps to T
naked try Option propagates None through compatible Option return
None handler recovers with T
Some handler inside try rejected
Result Ok/Err patterns against Option rejected with mentor diagnostic
Option Some/None patterns against Result rejected with mentor diagnostic
no implicit Option-to-Result propagation
no implicit Result-to-Option propagation
```

### 37.5 Result/error handlers

Test:

```text
Err(_) accepted
Err(named) accepted
specific Err variant accepted
Ok handler inside try rejected
nested match wrapper inside try rejected
handlers run top-to-bottom
first matching handler wins
unreachable handler after Err(_) rejected
specific duplicate/unreachable handler rejected when provable
guarded handler uses where
guard false continues to later handler
guarded catch-all does not make later fallback unreachable
guard cannot prematurely consume prospective move-only binding
partial handler propagates unmatched compatible error
partial handler rejects unmatched error that cannot propagate
Err(_) handles heterogeneous failure set without binding
heterogeneous Err(named) rejected when no single declared binding type exists
```

### 37.6 Recovery values

Test:

```text
handler expression produces success-type fallback
handler block final expression produces success-type fallback
incompatible fallback type rejected
terminating handler needs no fallback value
void success handler may complete normally
handler-produced failure is outside outer try
inner try handles fallible recovery code
```

### 37.7 Fallible assignment and properties

Test:

```text
try assignment with local handler
naked try assignment propagation
fallible assignment without try rejected
try set value ErrorType parses
try set without ErrorType rejected
setter normal completion is success
bare return in try setter is early success
return Err(error) is setter failure
return Ok() in try setter rejected
fallible getter syntax rejected in Sec 0.1
interface fallible setter declares error type
implementation error contract checked against interface
```

### 37.8 `return try`

Test:

```text
return try Result success forwards as Ok
return try Result failure propagates
success type assignability checked
error type assignability checked
return try does not create general implicit T-to-Result conversion
```

### 37.9 Open `error` matching

Test:

```text
Result[T, error] matches concrete error variant
concrete error payload remains available after widening
Err(errorValue) fallback covers open error domain
Err(_) fallback covers open error domain
concrete-only arms without fallback are non-exhaustive for error
error-specific narrowing does not enable general runtime type patterns
```

### 37.10 Diagnostics and tooling

Test:

```text
use-after-Result-projection diagnostic names consuming operation
mentor diagnostic explains Result versus Option pattern mismatch
mentor diagnostic explains incompatible propagation
mentor diagnostic explains missing try on fallible setter
LSP hover shows try success type and propagated states
LSP hover shows concrete and widened error type
LSP hover distinguishes consuming and borrowed Result projections
LSP and compiler diagnostics share resolved ownership facts
```

---

## 38. Non-goals for Sec 0.1

This revision does not introduce:

- exceptions;
- catch/finally syntax;
- general inheritance;
- arbitrary runtime type patterns;
- inferred anonymous public error unions;
- automatic domain-error wrapping;
- implicit Option-to-Result conversion;
- implicit Result-to-Option conversion;
- fallible property getters;
- mutable `Result` projection properties;
- a mandatory error message/id storage layout;
- a mandatory general error runtime.

---

## 39. Related rulebooks

The detailed adjacent semantics remain owned by:

```text
rules/errors/runtime_checks.md
rules/control-flow/flowcontrol_match.md
rules/control-flow/flowcontrol_if.md
rules/control-flow/discard.md
rules/declarations/properties.md
rules/declarations/impl.md
rules/declarations/enums.md
rules/declarations/unions.md
rules/types/types.md
rules/memory/ownership.md
rules/memory/copy_move.md
rules/memory/borrowing.md
rules/memory/destruction.md
rules/foundations/grammar.md
rules/compiler/compiler_known_members.md
rules/compiler/semantic_ir.txt
rules/tooling/diagnostics.txt
rules/tooling/lsp.md
```

Where an older adjacent rulebook conflicts with this revision's error-handling
semantics, the accompanying correction documents must be applied.

---

## 40. Design summary

```text
errors are explicit values
concrete errors derive from compiler-known error
concrete error identity survives widening to error
Result[T, concrete-error] is precise
Result[T, error] is intentionally open
Result is must-use
.Ok() and .Err() consume Result and return Option
OkRef and ErrRef borrow without consuming
try has implicit success
try handlers are partial
match is exhaustive
unmatched try failures propagate when compatible
Option try unwraps Some and propagates/handles None
Err(_) explicitly handles an error without binding its payload
handlers are top-to-bottom and first-match-wins
guards use where
handler recovery code is outside the protected try
fallible setters declare their error type explicitly
property getters are infallible in Sec 0.1
return try forwards Result success/failure in return position
try never catches panic
ownership is checked on every error-handling path
diagnostics explain programmer-visible cause and remedy
```

## 41. Test propagation boundary

A top-level test invocation and each `testing.Run` subtest invocation are
compiler-known error-propagation boundaries. An otherwise unhandled
`Err(errorValue)` propagated by ordinary `try` to that boundary terminates only
the current invocation, performs ordinary `defer` and destruction cleanup,
records `Failed`, and reports the unexpected error.

This boundary does not give a `test` declaration a source-visible `Result`
return type and does not introduce a parallel testing error system. Expected
errors continue to use ordinary `try` handlers and `match`. Canonical invocation
outcome behavior is owned by `rules/tooling/testing.md`.
