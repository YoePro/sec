# `match`

- **Status:** Normative
- **Created:** 2026-08-17
- **Last updated:** 2026-09-03
- **Document revision:** 2.1
- **Sec language version:** 0.1
- **Canonical path:** `rules/control-flow/flowcontrol_match.md`
- **Replaces:** `rules/control-flow/flowcontrol_match.txt`
- **Repository baseline reviewed:** `56be75d`

---

## 1. Purpose

`match` performs exhaustive structural and variant-based branching.

It is used for:

- union variants;
- `Result[T, E]`;
- `Option[T]`;
- ordinary enum members;
- open bit-backed enum values;
- payload binding;
- shallow named-field destructuring of struct-like union payloads;
- the compiler-known union initialization state `empty`;
- exhaustive control flow;
- value-producing branch selection.

`match` is not Sec's general value-comparison construct.

Use:

- `if` for a `bool` condition;
- `switch` for ordinary literal, range, or relational value selection;
- `match` for structural or variant selection.

Sec 0.1 does not define general runtime type matching or a recursive pattern language.

---

## 2. Core syntax

Statement form:

```sec
match result {
    Ok(value) => {
        Process(value)
    }

    Err(error) => {
        Handle(error)
    }
}
```

Expression form:

```sec
let message := match status {
    Status.Ready => "ready"
    Status.Waiting => "waiting"
    Status.Failed => "failed"
}
```

Guarded arm:

```sec
match option {
    Some(value) where value > 100 => Large(value)
    Some(value) => Normal(value)
    None => Missing()
}
```

A `match` must contain at least one arm.

---

## 3. Subject evaluation

The subject expression is evaluated exactly once.

```sec
match ReadResult() {
    Ok(value) => Process(value)
    Err(error) => Handle(error)
}
```

`ReadResult()` executes exactly once.

The resulting subject value or subject Place is the single semantic subject for all arm tests, guards, payload bindings, ownership actions, and final ownership-state merging.

The backend must not re-evaluate the source expression while lowering individual arms.

---

## 4. Arm ordering

Arms are considered in source order.

For each arm:

1. the pattern is tested;
2. when the pattern matches, candidate pattern bindings become visible to the guard;
3. when a guard exists, the guard is evaluated;
4. when the guard is false, the arm is not selected and matching continues with the next arm;
5. when the guard is true, the arm is selected;
6. ownership actions associated with the selected arm are committed according to this rulebook;
7. the arm body executes.

Exactly one arm executes.

No later pattern or guard is evaluated after an arm has been selected.

---

## 5. Exhaustiveness

`match` is exhaustive.

Every reachable semantic state of the subject must be covered by the arm set or by an unguarded catch-all pattern.

Coverage is based on the resolved subject type and proven control-flow state, not merely on source spelling.

A guarded arm does not by itself exhaust its underlying pattern because the guard may evaluate to `false`.

A finite set of guarded whole-payload arms may jointly cover a variant when
the payload is a closed enum and every reachable underlying enum value class is
selected by a direct equality guard on the payload binding. For example,
`Some(site) where site == SameSite.Strict` contributes the `Strict` value class.
The `Some` variant is covered only after the complete closed enum domain is
covered. This is a narrow finite-domain proof; arbitrary boolean guards do not
contribute exhaustiveness.

```sec
match option {
    Some(value) where value > 0 => Positive(value)
    Some(value) => Other(value)
    None => Missing()
}
```

This is exhaustive.

This is not:

```sec
match option {
    Some(value) where value > 0 => Positive(value)
    None => Missing()
}
```

because `Some` values for which the guard is false remain uncovered.

---

## 6. Catch-all pattern

The catch-all pattern is:

```text
_
```

It matches every remaining reachable state not handled by an earlier selected arm.

```sec
match direction {
    Direction.North => North()
    _ => Other()
}
```

An unguarded catch-all must be the final reachable arm.

An arm after an unguarded catch-all is unreachable and is invalid.

A guarded catch-all is not exhaustive by itself:

```sec
match value {
    _ where condition => Handle()
    _ => Fallback()
}
```

---

## 7. Result errors must not be hidden

A catch-all must not silently hide an unhandled `Err` variant.

This is invalid when `_` would absorb `Err`:

```sec
match result {
    Ok(value) => Process(value)
    _ => Ignore()
}
```

Handle the error explicitly:

```sec
match result {
    Ok(value) => Process(value)
    Err(error) => Handle(error)
}
```

An explicit `Err(_)` pattern handles the error variant and discards its payload
without introducing a binding. This is valid:

```sec
match result {
    Err(_) => Fallback()
    Ok(value) => Process(value)
}
```

This differs from the catch-all `_`, which may not hide an unhandled `Err`.
Name the payload only when the handler needs to use it:

```sec
match result {
    Ok(value) => Process(value)
    Err(error) => {
        discard error
        Fallback()
    }
}
```

---

## 8. Variant patterns

A payload-less variant is matched by its resolved variant name.

```sec
match direction {
    Direction.North => North()
    Direction.South => South()
    _ => Other()
}
```

Namespace qualification follows the normal name-resolution rules.

Variant spelling is resolved semantically. The backend must not infer variant identity from textual names.

---

## 9. Whole-payload binding

A variant carrying one payload may bind that payload as a whole value.

```sec
match result {
    Ok(value) => Process(value)
    Err(error) => Handle(error)
}
```

A struct-like union variant may also bind its complete payload object when that form is defined for the union:

```sec
match shape {
    Circle(circle) => Process(circle.Radius)
    Rectangle(rectangle) => Process(rectangle.Width, rectangle.Height)
}
```

The binding exists only within that arm's guard and body according to the guard rules below.

---

## 10. Shallow named-field destructuring

Struct-like union payloads support canonical shallow named-field destructuring.

```sec
match shape {
    Rectangle { width, height } => Use(width, height)
    Circle { radius } => UseRadius(radius)
}
```

Partial binding is valid and requires no `..` marker:

```sec
Rectangle { width } => Use(width)
```

Omitted fields:

- are not bound;
- do not constrain the matched value beyond variant selection;
- are not copied or moved merely because the pattern omits them.

Fields may be renamed:

```sec
Rectangle { width: w, height: h } => Use(w, h)
```

Fields may be borrowed:

```sec
Rectangle { width: ref w, height: ref h } => Inspect(w, h)
Rectangle { width: ref mut w } => Modify(w)
```

Destructuring is shallow.

Sec 0.1 does not define recursive nested destructuring such as:

```sec
Some(Ok(value)) => Use(value)
```

Use another `match` for the nested value.

---

## 11. The `empty` pattern

A mutable non-defaultable union binding may have the compiler-known initialization state `empty` according to the union rules.

When that state is reachable, `match` may inspect it:

```sec
match state {
    empty => Initialize()
    Idle => Wait()
    Running => Work()
}
```

`empty` is not:

- a union variant;
- a value of the union type;
- `None`;
- `Err`;
- `null`;
- a hidden numeric tag exposed to the program.

If control-flow analysis proves that the subject is initialized, `empty` is not part of exhaustiveness.

If `empty` is reachable, exhaustive coverage must include it or an unguarded catch-all.

`_` covers reachable `empty` state together with every other remaining reachable state.

A moved-from union is not `empty`.

---

## 12. Payload binding scope

Pattern bindings exist only within their arm.

They are visible:

- in that arm's guard;
- in that arm's body.

They are not visible:

- in another arm;
- after the `match`.

Normal Sec no-shadowing rules apply.

```sec
let value := 10

match result {
    Ok(value) => Process(value)
    Err(error) => Handle(error)
}
```

is invalid because the pattern binding would shadow the existing `value`.

---

## 13. Whole-payload ownership modes

Pattern bindings follow normal Sec ownership and borrowing rules.

For a whole-payload plain binding:

```sec
Some(value)
```

where the payload type is `T`:

- if `T` is implicitly copyable, the binding receives a copy;
- if `T` is move-only and the subject is an owned reusable Place from which ownership may legally be transferred, the binding receives the payload by move;
- if the subject is a fresh temporary, the payload may be forwarded directly into the binding without creating a reusable moved-from source binding;
- the compiler must never silently clone the payload;
- the compiler must never silently change a by-value binding into a borrow.

For explicit shared borrowing:

```sec
Some(ref value)
```

`value` has type `ref T`.

For explicit mutable borrowing:

```sec
Some(ref mut value)
```

`value` has type `ref mut T`, and the subject must provide mutable borrowing authority.

The borrow is branch-scoped and must satisfy the normal overlap and escape rules.

---

## 14. Borrowed subjects cannot transfer ownership

A subject accessed only through `ref UnionType` does not own the union value and therefore cannot transfer ownership of a move-only payload.

A plain by-value payload binding from a shared borrowed subject is valid only when the payload can be copied under the normal copy rules.

Move-only payload access requires an explicit borrow:

```sec
Some(ref value)
```

A `ref mut UnionType` subject may provide:

```sec
Some(ref value)
Some(ref mut value)
```

according to ordinary borrowing authority.

It still cannot move an owned payload out merely because mutable access exists. Mutable borrow authority is not ownership authority.

---

## 15. No hidden partial moves from shallow field destructuring

Shallow named-field destructuring must not create hidden partial moves from a union payload.

A plain field binding is therefore by-value copy only.

```sec
Packet { header, payload }
```

is valid for a field only when that field's type is implicitly copyable.

If a destructured field is move-only, plain by-value field binding is a compile-time error.

Use:

```sec
Packet { payload: ref p }
```

or:

```sec
Packet { payload: ref mut p }
```

when borrowing is intended.

When ownership of move-only content is required, bind the complete payload by value and perform explicit ownership operations on that complete value.

The compiler must not silently clone a field and must not leave the reusable union subject in a hidden partially moved state through shallow field destructuring.

---

## 16. Guards

A guard uses `where`:

```sec
Pattern where condition => body
```

The guard expression must have type `bool`.

The pattern is tested before the guard.

The guard is evaluated only when the pattern matches.

If the guard evaluates to `false`, matching continues with the next arm.

Bindings introduced by the pattern are visible in the guard.

---

## 17. Guards and move-only payloads

Ownership transfer for a move-only by-value payload binding must not be committed merely because the pattern matched.

The move is committed only after:

1. the pattern matches; and
2. the guard, when present, evaluates to `true`; and
3. the arm is selected.

Example:

```sec
match option {
    Some(value) where IsUsable(ref value) => Consume(value)
    Some(value) => HandleOther(value)
    None => Missing()
}
```

When `value` is move-only:

- the first pattern may expose a candidate binding to the guard;
- the guard may inspect or borrow the candidate under normal rules;
- the guard must not consume the prospective by-value binding before arm selection;
- if the guard is false, no move from the subject has occurred because of that arm;
- later arms must see the ownership state that exists after the guard's actual side effects but without a committed payload move from the rejected arm;
- if the guard is true, the move into `value` is committed before the arm body uses `value` as its owned binding.

A consuming use of a prospective move-only binding inside the guard is invalid.

Explicit `ref` and `ref mut` pattern bindings create the corresponding candidate borrow for guard evaluation. A borrow from an arm whose guard fails ends before the next arm is tested, subject to ordinary side effects already performed by the guard.

---

## 18. Ownership state after `match`

Ownership analysis is path-sensitive across match arms.

Every continuing arm contributes its resulting ownership state.

If a move-only payload is moved from a reusable union subject in an arm, the compiler must mark the affected subject or Place unavailable on that path according to the ownership model.

Example:

```sec
match value {
    Some(resource) => Consume(<-resource)
    None => Nothing()
}

Use(value)
```

If `resource` is move-only and the `Some` arm continues after consuming the payload, `value` is not definitely available after the `match`.

The post-match merge must conservatively combine the continuing arm states.

Conceptually:

```text
Available + Moved
    ConditionallyAvailable
```

A later whole-value use is invalid unless every continuing path establishes an available value through legal reinitialization.

A terminating arm does not contribute an ownership state to a later merge point because that path does not continue.

The compiler must enforce this rule. It is not an advisory analysis.

---

## 19. Statement `match`

A `match` may be used only for control flow and side effects.

```sec
match result {
    Ok(value) => Process(value)
    Err(error) => Handle(error)
}
```

An expression arm in statement context is evaluated in statement context.

A produced value must not disappear merely because the surrounding construct is `match`.

The normal discard rules apply:

- ordinary implicitly discardable call results may use the established implicit call-result discard behavior;
- must-use values may not be implicitly discarded;
- explicit discard must be written when required or intentionally used;
- ownership and destruction of discarded results follow `discard.md`.

When a statement arm needs explicit discard, use a block:

```sec
match option {
    Some(value) => {
        discard Calculate(value)
    }

    None => {}
}
```

`match` itself does not create a new blanket implicit-discard rule.

---

## 20. Expression `match`

A `match` may produce a value.

```sec
let message := match status {
    Status.Ready => "ready"
    Status.Waiting => "waiting"
    Status.Failed => "failed"
}
```

Every continuing arm must produce a value assignable to one common result type.

A branch that returns, propagates, or otherwise terminates does not need to produce the match result value.

```sec
let value := match result {
    Ok(value) => value
    Err(error) => return Err(error)
}
```

The match result's ownership classification follows its resolved result type and the ownership action that produced each continuing arm result.

---

## 21. Value-producing arm blocks

Sec 0.1 supports a contextual value-producing block specifically in the result position of an expression-match arm.

```sec
let price := match product {
    Book(book) => {
        LogBook(book)
        CalculateBookPrice(book)
    }

    Service(service) => service.Price
}
```

In such a block, the final expression on every continuing path is the arm result.

`CalculateBookPrice(book)` above is therefore the value produced by the `Book` arm.

This rule does not make arbitrary Sec blocks into expressions.

Outside this contextual match-arm result position, ordinary block semantics remain unchanged.

### Continuing paths

Every path that reaches the end of a value-producing arm block must reach a final value expression assignable to the match result type.

```sec
let result := match value {
    A(data) => {
        if Invalid(data) {
            return Err(Error.InvalidData)
        }

        Transform(data)
    }

    B => DefaultValue()
}
```

The `return` path terminates. The continuing path produces `Transform(data)`.

This is invalid when `Log(data)` returns `void`:

```sec
let result := match value {
    A(data) => {
        Log(data)
    }

    B => DefaultValue()
}
```

because the continuing `A` arm does not produce the required result value.

### Ownership of the final expression

If the final expression produces or transfers a move-only value, ownership of the match result must be established before cleanup destroys arm-local values required to produce that result.

Conceptually, arm-result ownership is committed before arm-scope cleanup runs.

The backend may optimize representation but must preserve this semantic ordering.

---

## 22. Result type compatibility

All continuing value-producing arms must produce compatible types under the normal Sec type system.

Named type identity remains significant.

No match-specific implicit conversion is introduced.

```sec
let value := match option {
    Some(number) => number
    None => "missing"
}
```

is invalid when `number` has type `int`.

Likewise, different named semantic types remain distinct even when their representation types are compatible.

---

## 23. Return and termination analysis

`match` participates in function return analysis.

```sec
fn Convert(result: Result[int, IOError]) int {
    match result {
        Ok(value) => {
            return value
        }

        Err(error) => {
            discard error
            return 0
        }
    }
}
```

An exhaustive match whose every arm terminates is itself non-continuing.

If at least one reachable arm continues, control may continue after the match.

---

## 24. Definite assignment

An exhaustive match can establish definite assignment when every continuing arm establishes the required assignment state.

```sec
fn Convert(option: Option[int]) int {
    let mut result: int

    match option {
        Some(value) => {
            result = value
        }

        None => {
            result = 0
        }
    }

    return result
}
```

Only continuing arms contribute to the state after the match.

The same path-sensitive rule applies to initialization state, union `empty` state, ownership state, borrow state, and other compiler analyses that merge across control flow.

---

## 25. Duplicate and unreachable arms

A duplicate unguarded pattern is invalid.

```sec
match option {
    None => A()
    None => B()
    Some(value) => C(value)
}
```

A pattern fully covered by an earlier unguarded arm is unreachable.

```sec
match option {
    Some(value) => A(value)
    Some(value) where value > 10 => B(value)
    None => C()
}
```

The guarded `Some` arm is unreachable because the earlier unguarded `Some` already covers every `Some` value.

A guarded arm may precede an unguarded fallback for the same pattern:

```sec
match option {
    Some(value) where value > 10 => Large(value)
    Some(value) => Other(value)
    None => Missing()
}
```

---

## 26. Ordinary enums

Ordinary enums are closed over their declared underlying value classes,
including integer and string values.

Exhaustiveness is based on distinct reachable declared values, not merely member spellings.

If multiple enum members are aliases for the same underlying value, they
represent one runtime value class for coverage and duplicate/unreachable
analysis.

A match need not contain separate arms for every alias spelling.

The compiler must normalize enum aliases before coverage analysis.

---

## 27. Open bit-backed enums

A `bit` or `bit[N]` hardware enum is open over the complete representable bit domain.

Declared members do not necessarily exhaust that domain.

Therefore:

```sec
match status {
    Status.Ready => Ready()
    Status.Error => Error()
}
```

is exhaustive only when the declared members provably cover the complete representable bit domain.

Otherwise an additional arm such as:

```sec
_ => UnknownEncoding()
```

is required.

The compiler must base exhaustiveness on the actual bit width and normalized numeric value classes.

---

## 28. `Result[T, E]`

`Result[T, E]` uses the same match machinery with compiler-known variants:

```sec
match result {
    Ok(value) => Success(value)
    Err(error) => Failure(error)
}
```

General `match` does not propagate `Err` implicitly.

Both success and error states must be covered explicitly according to the Result/error rules.

Use `try` when success should continue implicitly and errors should be propagated or handled through try-handler semantics.

Use `match` when both `Ok` and `Err` belong to explicit branch control flow.

---

## 29. `Option[T]`

`Option[T]` uses compiler-known variants:

```sec
match option {
    Some(value) => Use(value)
    None => Missing()
}
```

`None` is a real `Option[T]` value.

It is not the union initialization state `empty`.

Whole-payload binding of `Some` follows the ownership rules in this document.

### Open `error` narrowing

`Result[T, error]` preserves the concrete identity and payload of widened Sec
errors. Its `Err` branch may therefore use error-specific concrete patterns:

```sec
match result {
    Ok(value) => Use(value)
    Err(IOError.NotFound) => UseDefault()
    Err(errorValue) => Handle(errorValue)
}
```

The root `error` domain is open. Concrete error arms alone are not exhaustive;
an `Err(errorValue)` or `Err(_)` fallback is required unless control-flow facts
prove a narrower closed state. Generic `_` still may not hide an unhandled
`Err`. This is an error-specific narrowing rule, not general runtime type
matching.

---

## 30. Direct boolean matching is not part of Sec 0.1

A plain `bool` subject should use `if` / `else`.

This is not a Sec 0.1 match form:

```sec
match ready {
    true => Start()
    false => Wait()
}
```

Use:

```sec
if ready {
    Start()
} else {
    Wait()
}
```

A union or enum variant may of course contain or expose a `bool` payload. The structural variant may be matched, after which the boolean may be inspected with a guard or ordinary `if`.

`match` is justified by the structural or variant domain, not by the boolean value itself.

---

## 31. Literal and range patterns are not part of Sec 0.1

Ordinary literal value matching belongs to `switch` or `if`.

Do not write:

```sec
match value {
    1 => One()
    2 => Two()
    _ => Other()
}
```

Use `switch`.

Range patterns are likewise not part of Sec 0.1:

```sec
match value {
    0..<10 => Small()
    _ => Other()
}
```

Use the range facilities of `switch`.

Enum member patterns are not classified as ordinary literal patterns merely because enum members have numeric representations.

---

## 32. Unsupported pattern forms in Sec 0.1

Sec 0.1 does not define:

- pattern alternatives such as `A | B`;
- general literal patterns;
- range patterns;
- direct `true` / `false` match patterns;
- runtime type patterns;
- ordinary struct-subject destructuring;
- nested recursive patterns;
- arbitrary user-defined pattern protocols;
- regex patterns;
- guard syntax other than `where`.

Write separate arms, nested matches, `switch`, or `if` as appropriate.

---

## 33. No fallthrough

Match arms never fall through.

There is no `fallthrough` operation for `match`.

Each selected arm either:

- completes its own body and reaches the match merge;
- produces the match expression value;
- returns;
- propagates;
- terminates through another defined control-flow operation.

---

## 34. `break` and `continue`

`match` is not a `break` or `continue` target.

A `break` or `continue` written inside a match arm applies only when an enclosing loop or other construct legally provides that target.

Before such control transfer, ordinary cleanup rules apply to scopes exited by the edge.

The match construct itself does not create an implicit loop-like control target.

---

## 35. Empty matches

This is invalid in Sec 0.1:

```sec
match value {
}
```

Even if a future type system introduces an uninhabited type, zero-arm match syntax is not part of Sec 0.1.

The compiler should report one focused diagnostic.

---

## 36. Compiler analysis requirements

Before lowering, Sema and ownership/borrow analysis must resolve at least:

- subject type and evaluation identity;
- whether the subject is a reusable Place or temporary;
- subject mutability and ownership authority;
- subject initialization state, including reachable union `empty`;
- normalized pattern identity;
- variant identity;
- enum underlying value class when applicable;
- open versus closed enum domain;
- pattern-binding type;
- binding action for each bound payload or field;
- candidate guard binding state;
- guard type and source order;
- guard ownership restrictions;
- arm reachability;
- duplicate patterns;
- exhaustive residual coverage;
- arm flow classification;
- expression-result type;
- result ownership action;
- borrow creation and end;
- move commit point;
- post-arm ownership state;
- post-match ownership merge;
- definite-assignment merge;
- required cleanup and destruction edges;
- discard or must-use behavior for statement-arm results.

No backend may infer these facts from emitted low-level code.

---

## 37. LSP requirements

The LSP is an interactive view of compiler-resolved match semantics.

It must not implement a parallel match, ownership, borrow, or exhaustiveness analyzer.

Compiler-produced facts must be available to the LSP early enough for normal interactive editing.

### Payload binding hover

For a binding such as:

```sec
Some(value)
```

hover must be able to expose at least:

```text
Type: Resource
Binding mode: move
Source: option.Some payload
Commit: when this arm is selected
```

For a copyable payload:

```text
Binding mode: copy
```

For borrowed payloads:

```text
Binding mode: shared borrow
```

or:

```text
Binding mode: mutable borrow
```

### Guard-aware move display

For a guarded move-only binding, tooling must distinguish a prospective arm move from an already committed move.

Before a guarded arm is selected, the LSP must not display the subject as already moved merely because the pattern text contains a by-value move-only binding.

It may display:

```text
Ownership effect: moves payload if this arm is selected
```

### Post-match availability

When ownership merging produces a state such as `Moved`, `PartiallyAvailable`, or `ConditionallyAvailable`, hover and diagnostics must use the compiler's resolved state.

A mandatory use-after-move or conditionally-unavailable error must be published by the LSP as the same language-safety diagnostic produced by the compiler.

Tooling should identify:

- the arm that caused the move;
- the binding receiving ownership;
- the affected subject or payload Place;
- the merge that makes later use unavailable;
- valid borrow/reinitialization alternatives when provable.

Configurable ownership inlay hints may show non-obvious `copy`, `move`, `ref`, and `ref mut` effects.

The underlying ownership correctness is mandatory even when visual hints are disabled.

---

## 38. Semantic IR requirements

Resolved match semantics must be explicit before target lowering.

Semantic IR must preserve or encode:

- one evaluated subject;
- source arm order;
- pattern tests;
- resolved variant identities;
- enum underlying value classes;
- reachable `empty` state where applicable;
- guard control flow;
- candidate binding identity;
- binding ownership action;
- the guard-success ownership commit point;
- borrow begin/end behavior;
- arm flow classification;
- typed expression-arm results;
- value-producing match-arm block result;
- arm-result ownership transfer;
- statement-arm discard behavior where applicable;
- post-arm ownership state;
- merge ownership state;
- exhaustiveness proof and impossible residual edge.

Payload extraction or borrowing must occur only after the appropriate variant has been established.

A move from a reusable subject through a guarded by-value binding must not be represented as committed on the guard-false edge.

The backend must not perform match exhaustiveness analysis or reconstruct source ownership policy.

---

## 39. Diagnostics

Match diagnostics should be specific and source-related.

Required categories include:

- empty match;
- invalid subject type;
- unknown variant;
- invalid pattern for subject type;
- duplicate pattern;
- unreachable arm;
- non-exhaustive match;
- uncovered reachable `empty` state;
- uncovered open bit-enum encodings;
- non-`bool` guard;
- shadowing pattern binding;
- invalid `_` error discard;
- invalid `ref` or `ref mut` binding;
- attempted move from borrowed subject;
- move-only shallow field by-value binding;
- consuming use of a prospective move-only guard binding;
- post-match use of moved or conditionally unavailable subject;
- incompatible expression-arm result types;
- expression arm that fails to produce a value on a continuing path;
- must-use result implicitly discarded by statement match.

Ownership diagnostics should include related locations for the source Place, move-producing arm, and later invalid use when available.

---

## 40. Design summary

Sec 0.1 `match` follows these rules:

1. `match` is for structural and variant branching.
2. The subject is evaluated exactly once.
3. Arms and guards are evaluated in source order.
4. Match is exhaustive.
5. Guarded arms do not by themselves provide complete coverage.
6. `_` is the final unguarded catch-all and must not silently hide `Err`.
7. `empty` is compiler-known union initialization state, not a variant.
8. Whole-payload plain binding copies copyable payloads and moves move-only owned payloads.
9. `ref` and `ref mut` provide explicit payload borrowing.
10. Borrowed subjects cannot transfer ownership they do not possess.
11. Shallow field destructuring never performs a hidden move-only partial move.
12. Move-only ownership transfer from a guarded arm commits only after the guard succeeds.
13. Ownership state is merged path-sensitively and enforced after the match.
14. Statement match obeys the ordinary discard and must-use rules.
15. Expression match requires a common result type on every continuing arm.
16. A match-arm block may contextually produce its value from its final expression without creating general block-expression semantics.
17. Ordinary enums are closed over distinct declared underlying value classes.
18. Bit-backed enums are open over their complete representable domain.
19. Direct bool, literal, range, runtime-type, recursive nested, and ordinary-struct patterns are not part of Sec 0.1.
20. Compiler and LSP must expose the same resolved ownership and coverage facts.
